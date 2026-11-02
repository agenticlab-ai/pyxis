package watch

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

func TestCoverage90RecoveryValidationAndReductionBoundaries(t *testing.T) {
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	entry := func(scope, obligation, epoch string, action RecoveryAction) RecoveryEntry {
		row := RecoveryEntry{Scope: scope, ObligationID: obligation, RecoveryEpoch: epoch, Action: action, FirstObservedAtUTC: now, RecoveryBound: 1, RecordedAtUTC: now}
		switch action {
		case RecoveryAdmit:
			row.RecoveryCount, row.DriveRoundID = 1, "round"
		case RecoveryExhaust:
			row.RecoveryCount, row.DriveRoundID, row.Reason = 1, "round", "exhausted"
		case RecoveryRearm:
			row.Principal, row.Reason, row.PriorEvidenceRef = "operator", "repaired", "sha256:prior"
		}
		return mustSealRecovery(t, row)
	}

	init := entry("scope", "o-1", "epoch-1", RecoveryInitialize)
	for _, tc := range []struct {
		name string
		rows []RecoveryEntry
	}{
		{name: "foreign scope", rows: []RecoveryEntry{entry("other", "o-1", "epoch-1", RecoveryInitialize)}},
		{name: "conflicting replay", rows: func() []RecoveryEntry {
			conflict := init
			conflict.RecordedAtUTC = now.Add(time.Second)
			conflict = mustSealRecovery(t, conflict)
			return []RecoveryEntry{init, conflict}
		}()},
		{name: "duplicate initialization", rows: []RecoveryEntry{init, entry("scope", "o-1", "epoch-2", RecoveryInitialize)}},
		{name: "admit without initialization", rows: []RecoveryEntry{entry("scope", "o-1", "epoch-1", RecoveryAdmit)}},
		{name: "exhaust without initialization", rows: []RecoveryEntry{entry("scope", "o-1", "epoch-1", RecoveryExhaust)}},
		{name: "close without initialization", rows: []RecoveryEntry{entry("scope", "o-1", "epoch-1", RecoveryClose)}},
		{name: "rearm without initialization", rows: []RecoveryEntry{entry("scope", "o-1", "epoch-2", RecoveryRearm)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := reduceRecovery("scope", tc.rows); err == nil {
				t.Fatal("invalid recovery history was accepted")
			}
		})
	}
	if got, err := reduceRecovery("scope", []RecoveryEntry{init, init}); err != nil || got["o-1"].Epoch != "epoch-1" {
		t.Fatalf("idempotent recovery replay = %+v, %v", got, err)
	}

	base := RecoveryEntry{SchemaVersion: ReviewRecoverySchema, Scope: "scope", ObligationID: "o-1", RecoveryEpoch: "epoch", FirstObservedAtUTC: now, RecoveryBound: 1, RecordedAtUTC: now}
	invalid := []RecoveryEntry{
		func() RecoveryEntry { r := base; r.Action, r.RecoveryCount = RecoveryInitialize, 1; return r }(),
		func() RecoveryEntry { r := base; r.Action = RecoveryAdmit; return r }(),
		func() RecoveryEntry { r := base; r.Action = RecoveryExhaust; return r }(),
		func() RecoveryEntry { r := base; r.Action = RecoveryRearm; return r }(),
		func() RecoveryEntry { r := base; r.Action = "foreign"; return r }(),
		func() RecoveryEntry { r := base; r.Action, r.EvidenceRef = RecoveryClose, "sha256:wrong"; return r }(),
	}
	for i, row := range invalid {
		if err := validateRecoveryEntry(row); err == nil {
			t.Fatalf("invalid recovery row %d was accepted", i)
		}
	}
}

func TestCoverage90FileRecoveryAndEventBoundaries(t *testing.T) {
	root := t.TempDir()
	if _, err := (&FileRecoveryLedger{path: filepath.Join(root, "invalid-append.jsonl")}).Append(t.Context(), RecoveryEntry{}); err == nil {
		t.Fatal("invalid recovery append was accepted")
	}
	blocked := filepath.Join(root, "blocked")
	if err := os.WriteFile(blocked, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	valid := RecoveryEntry{Scope: "scope", ObligationID: "o-1", RecoveryEpoch: "epoch", Action: RecoveryInitialize, FirstObservedAtUTC: now, RecoveryBound: 1, RecordedAtUTC: now}
	if _, err := (&FileRecoveryLedger{path: filepath.Join(blocked, "ledger.jsonl")}).Append(t.Context(), valid); err == nil {
		t.Fatal("append ignored a recovery-lock failure")
	}
	loop := filepath.Join(root, "loop")
	if err := os.Symlink(loop, loop); err != nil {
		t.Fatal(err)
	}
	ledger := &FileRecoveryLedger{path: loop}
	if _, err := ledger.exactLocked("scope"); err == nil {
		t.Fatal("symlink-loop ledger was readable")
	}

	invalidPath := filepath.Join(root, "invalid.jsonl")
	if err := os.WriteFile(invalidPath, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := (&FileRecoveryLedger{path: invalidPath}).exactLocked("scope"); err == nil {
		t.Fatal("invalid recovery envelope was readable")
	}

	oversizedPath := filepath.Join(root, "oversized.jsonl")
	if err := os.WriteFile(oversizedPath, []byte(strings.Repeat("x", (1<<20)+1)), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := (&FileRecoveryLedger{path: oversizedPath}).exactLocked("scope"); err == nil {
		t.Fatal("oversized recovery row was readable")
	}

	var nilSink *Sink
	nilSink.Append(Event{Kind: EventCheckRan})
	if got := valueOrZero(nil); got != 0 {
		t.Fatalf("nil integer = %d", got)
	}
	sink := NewSink(filepath.Join(root, "events"), 1, nil)
	sink.warn = func(string, ...any) {}
	sink.fail("forced failure")
	if !sink.degraded {
		t.Fatal("sink failure did not mark degraded state")
	}
}

type coverage90Reader struct {
	states  []port.ProposalState
	err     error
	cadence time.Duration
}

func (r coverage90Reader) States(context.Context, []port.ProposalRef, port.Cursor) ([]port.ProposalState, error) {
	return r.states, r.err
}
func (r coverage90Reader) Cadence(context.Context) time.Duration { return r.cadence }

func TestCoverage90WatchLoopBoundaries(t *testing.T) {
	if _, err := Derive(t.Context(), Plan{}, nil); err == nil {
		t.Fatal("Derive accepted a nil reader")
	}
	ref := port.ProposalRef{Repo: "repo", Number: 1}
	plan := Plan{Tasks: []Task{{ID: "same", Claimed: true}, {ID: "same", Proposal: &ref}}}
	want := errors.New("state read failed")
	if _, err := Derive(t.Context(), plan, coverage90Reader{err: want}); !errors.Is(err, want) {
		t.Fatalf("state read error = %v", err)
	}
	if _, err := Derive(t.Context(), plan, coverage90Reader{}); err == nil {
		t.Fatal("short state read was accepted")
	}
	if _, err := Derive(t.Context(), plan, coverage90Reader{states: []port.ProposalState{{Landing: port.LandingOpen}}, cadence: time.Second}); err != nil {
		t.Fatal(err)
	}
	if !hasRed(port.ProposalState{Checks: []port.CheckContext{{State: port.CheckRed}}}) {
		t.Fatal("red check was not detected")
	}

	if _, err := RunLive(t.Context(), nil, nil, nil, nil, nil); err == nil {
		t.Fatal("RunLive accepted missing dependencies")
	}
	if _, err := RunLive(t.Context(), func(context.Context) (Plan, error) { return Plan{}, want }, coverage90Reader{cadence: time.Second}, func(context.Context) error { return nil }, nil, nil); !errors.Is(err, want) {
		t.Fatalf("plan source error = %v", err)
	}
	if _, err := RunLive(t.Context(), constantSource(Plan{Tasks: []Task{{ID: "ready"}}}), coverage90Reader{cadence: time.Second}, func(context.Context) error { return want }, nil, nil); !errors.Is(err, want) {
		t.Fatalf("sweep error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := RunLive(ctx, constantSource(Plan{}), coverage90Reader{cadence: time.Second}, func(context.Context) error { return nil }, nil, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled loop = %v", err)
	}
	_ = realClock{}.Now()
}

func TestCoverage90CombinedSortsAndRecordsMultipleExhaustedReviews(t *testing.T) {
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	ledger := &boundaryRecoveryLedger{}
	provider := &mutableReviewProvider{rows: []ReviewObligationState{pendingReview("o-2"), pendingReview("o-1")}}
	for _, id := range []string{"o-2", "o-1"} {
		init := mustSealRecovery(t, RecoveryEntry{Scope: "scope", ObligationID: id, RecoveryEpoch: "epoch-" + id, Action: RecoveryInitialize, FirstObservedAtUTC: now, RecoveryBound: 1, RecordedAtUTC: now})
		admit := mustSealRecovery(t, RecoveryEntry{Scope: "scope", ObligationID: id, RecoveryEpoch: "epoch-" + id, Action: RecoveryAdmit, FirstObservedAtUTC: now, RecoveryCount: 1, RecoveryBound: 1, RecordedAtUTC: now, DriveRoundID: "prior"})
		ledger.entries = append(ledger.entries, init, admit)
	}
	reviews := ReviewStateReader{Provider: provider, Recovery: ledger, Clock: &fixedRecoveryClock{now: now}}
	_, err := RunCombinedLive(t.Context(), constantSource(Plan{}), coverage90Reader{cadence: time.Second}, reviews, RecoveryPolicy{Bound: 1}, "scope", func(context.Context) error { return nil }, func(context.Context, ReviewRecoveryRequest) error { return nil }, func() (string, error) { return "decision", nil }, NewSink(filepath.Join(t.TempDir(), "events"), 0, func(string, ...any) {}), immediateClock{}, nil)
	if !errors.Is(err, ErrReviewExhausted) {
		t.Fatalf("combined exhausted result = %v", err)
	}
	if len(ledger.entries) != 6 {
		encoded, _ := json.Marshal(ledger.entries)
		t.Fatalf("exhaustion records = %s", encoded)
	}
}
