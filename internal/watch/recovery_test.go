package watch

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

type fixedRecoveryClock struct{ now time.Time }

func (c *fixedRecoveryClock) Now() time.Time { return c.now }

type mutableReviewProvider struct {
	mu    sync.Mutex
	rows  []ReviewObligationState
	err   error
	calls int
}

func (p *mutableReviewProvider) Open(context.Context, string) ([]ReviewObligationState, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls++
	return append([]ReviewObligationState(nil), p.rows...), p.err
}

func (p *mutableReviewProvider) verify(ids ...string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	set := map[string]bool{}
	for _, id := range ids {
		set[id] = true
	}
	for i := range p.rows {
		if set[p.rows[i].ID] {
			p.rows[i].ProviderState = ReviewVerified
		}
	}
}

func pendingReview(id string) ReviewObligationState {
	return ReviewObligationState{ID: id, TaskID: "T-" + id, ProposalRef: "github:o/r#1", Revision: "abc", Harness: "claude", ProviderState: ReviewPending}
}

func TestReviewStateReaderInitializesOnceAndPreservesAgeAcrossRestart(t *testing.T) {
	ledger, err := NewFileRecoveryLedger(filepath.Join(t.TempDir(), "recovery.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	provider := &mutableReviewProvider{rows: []ReviewObligationState{pendingReview("o-1")}}
	clock := &fixedRecoveryClock{now: time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)}
	reader := ReviewStateReader{Provider: provider, Recovery: ledger, Clock: clock, Epoch: func() (string, error) { return "epoch-1", nil }}

	first, err := reader.DeriveReadiness(context.Background(), "feature/001", RecoveryPolicy{Bound: 3})
	if err != nil || first.Status != ReviewStateKnown || len(first.Rows) != 1 {
		t.Fatalf("first derivation = %+v, err=%v", first, err)
	}
	if first.Rows[0].RecoveryCount != 0 || first.Rows[0].RecoveryBound != 3 || first.Rows[0].AgeS != 0 {
		t.Fatalf("initialized row = %+v", first.Rows[0])
	}

	clock.now = clock.now.Add(91 * time.Second)
	restarted := ReviewStateReader{Provider: provider, Recovery: ledger, Clock: clock, Epoch: func() (string, error) { return "must-not-run", nil }}
	second, err := restarted.DeriveReadiness(context.Background(), "feature/001", RecoveryPolicy{Bound: 99})
	if err != nil {
		t.Fatal(err)
	}
	if second.Rows[0].RecoveryEpoch != "epoch-1" || second.Rows[0].RecoveryBound != 3 || second.Rows[0].AgeS != 91 {
		t.Fatalf("restart changed frozen recovery state: %+v", second.Rows[0])
	}

	clock.now = first.Rows[0].FirstObservedAtUTC.Add(-time.Minute)
	backward, err := restarted.DeriveReadiness(context.Background(), "feature/001", RecoveryPolicy{Bound: 3})
	if err != nil || backward.Rows[0].AgeS != 0 {
		t.Fatalf("backward clock age = %d, err=%v", backward.Rows[0].AgeS, err)
	}
}

func TestReviewStateReaderExplicitRearmStartsNewEpochAndRetainsExhaustion(t *testing.T) {
	ledger, err := NewFileRecoveryLedger(filepath.Join(t.TempDir(), "recovery.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	provider := &mutableReviewProvider{rows: []ReviewObligationState{pendingReview("o-1")}}
	clock := &fixedRecoveryClock{now: time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)}
	epoch := 0
	reader := ReviewStateReader{Provider: provider, Recovery: ledger, Clock: clock, Epoch: func() (string, error) {
		epoch++
		return "epoch-" + strconv.Itoa(epoch), nil
	}}
	state, err := reader.DeriveReadiness(context.Background(), "feature/001", RecoveryPolicy{Bound: 1})
	if err != nil {
		t.Fatal(err)
	}
	row, err := reader.Admit(context.Background(), "feature/001", "round-1", state.Rows[0])
	if err != nil {
		t.Fatal(err)
	}
	prior, err := reader.RecordExhaustion(context.Background(), "feature/001", "round-2", row)
	if err != nil {
		t.Fatal(err)
	}
	clock.now = clock.now.Add(time.Minute)
	rearmed, err := reader.Rearm(context.Background(), "feature/001", "o-1", "operator-1", "review service repaired", RecoveryPolicy{Bound: 5})
	if err != nil || rearmed == "" || rearmed == prior {
		t.Fatalf("Rearm ref=%q prior=%q err=%v", rearmed, prior, err)
	}
	next, err := reader.DeriveReadiness(context.Background(), "feature/001", RecoveryPolicy{Bound: 99})
	if err != nil || len(next.Rows) != 1 {
		t.Fatalf("next = %+v, err=%v", next, err)
	}
	if next.Rows[0].RecoveryEpoch != "epoch-2" || next.Rows[0].RecoveryCount != 0 || next.Rows[0].RecoveryBound != 5 || next.Rows[0].Eligibility != ReviewEligible {
		t.Fatalf("rearmed row = %+v", next.Rows[0])
	}
	entries, err := ledger.Exact(context.Background(), "feature/001")
	if err != nil || len(entries) != 4 || entries[2].Action != RecoveryExhaust || entries[3].Action != RecoveryRearm || entries[3].PriorEvidenceRef != prior {
		t.Fatalf("retained ledger = %+v, err=%v", entries, err)
	}
}

func TestSnapshotStatusIsReadOnlyAndMarksUnobserved(t *testing.T) {
	ledger, _ := NewFileRecoveryLedger(filepath.Join(t.TempDir(), "recovery.jsonl"))
	provider := &mutableReviewProvider{rows: []ReviewObligationState{pendingReview("o-1")}}
	reader := ReviewStateReader{Provider: provider, Recovery: ledger, Clock: &fixedRecoveryClock{now: time.Now()}}
	state, err := reader.SnapshotStatus(context.Background(), "feature/001")
	if err != nil || len(state.Rows) != 1 || state.Rows[0].Eligibility != ReviewUnobserved {
		t.Fatalf("snapshot = %+v, err=%v", state, err)
	}
	rows, err := ledger.Exact(context.Background(), "feature/001")
	if err != nil || len(rows) != 0 {
		t.Fatalf("status wrote recovery state: rows=%+v err=%v", rows, err)
	}
}

func TestFileRecoveryLedgerRejectsConflictingOrOutOfSequenceReplay(t *testing.T) {
	ledger, _ := NewFileRecoveryLedger(filepath.Join(t.TempDir(), "recovery.jsonl"))
	now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	init := RecoveryEntry{Scope: "feature/001", ObligationID: "o-1", RecoveryEpoch: "e-1", Action: RecoveryInitialize, FirstObservedAtUTC: now, RecoveryCount: 0, RecoveryBound: 3, RecordedAtUTC: now}
	ref, err := ledger.Append(context.Background(), init)
	if err != nil || ref == "" {
		t.Fatalf("initialize: ref=%q err=%v", ref, err)
	}
	if replay, err := ledger.Append(context.Background(), init); err != nil || replay != ref {
		t.Fatalf("idempotent replay: ref=%q err=%v", replay, err)
	}
	bad := init
	bad.RecordedAtUTC = now.Add(time.Second)
	if _, err := ledger.Append(context.Background(), bad); err == nil {
		t.Fatal("conflicting idempotence key was accepted")
	}
	skip := RecoveryEntry{Scope: "feature/001", ObligationID: "o-1", RecoveryEpoch: "e-1", Action: RecoveryAdmit, FirstObservedAtUTC: now, RecoveryCount: 2, RecoveryBound: 3, RecordedAtUTC: now, DriveRoundID: "round-1"}
	if _, err := ledger.Append(context.Background(), skip); err == nil {
		t.Fatal("out-of-sequence admission was accepted")
	}
}

func runRecoveryGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@invalid", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@invalid")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}

func TestGitRecoveryLedgerReplaysFromFreshClone(t *testing.T) {
	root := t.TempDir()
	remote, repo := filepath.Join(root, "remote.git"), filepath.Join(root, "repo")
	if out, err := exec.Command("git", "init", "--bare", "-q", remote).CombinedOutput(); err != nil {
		t.Fatalf("init bare: %v: %s", err, out)
	}
	if err := os.Mkdir(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	runRecoveryGit(t, repo, "init", "-q")
	if err := os.WriteFile(filepath.Join(repo, "README"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runRecoveryGit(t, repo, "add", "README")
	runRecoveryGit(t, repo, "commit", "-qm", "base")
	runRecoveryGit(t, repo, "remote", "add", "origin", remote)
	runRecoveryGit(t, repo, "push", "-q", "origin", "HEAD:refs/heads/main")

	ledger, err := NewGitRecoveryLedger(repo, "origin", DefaultRecoveryEvidenceRef)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	entry := RecoveryEntry{Scope: "feature/001", ObligationID: "o-1", RecoveryEpoch: "epoch-1", Action: RecoveryInitialize, FirstObservedAtUTC: now, RecoveryCount: 0, RecoveryBound: 3, RecordedAtUTC: now}
	ref, err := ledger.Append(context.Background(), entry)
	if err != nil {
		t.Fatal(err)
	}
	if replay, err := ledger.Append(context.Background(), entry); err != nil || replay != ref {
		t.Fatalf("Git recovery replay = %q, %v; want %q", replay, err, ref)
	}
	conflict := entry
	conflict.RecordedAtUTC = now.Add(time.Second)
	if _, err := ledger.Append(context.Background(), conflict); err == nil {
		t.Fatal("Git recovery ledger accepted a conflicting replay")
	}

	fresh := filepath.Join(root, "fresh")
	if out, err := exec.Command("git", "clone", "-q", remote, fresh).CombinedOutput(); err != nil {
		t.Fatalf("clone fresh: %v: %s", err, out)
	}
	replayed, err := NewGitRecoveryLedger(fresh, "origin", DefaultRecoveryEvidenceRef)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := replayed.Exact(context.Background(), "feature/001")
	if err != nil || len(rows) != 1 || rows[0].ObligationID != "o-1" || rows[0].RecoveryBound != 3 {
		t.Fatalf("fresh recovery rows=%+v err=%v", rows, err)
	}
}

type immediateClock struct{ now time.Time }

func (c immediateClock) Now() time.Time { return c.now }
func (c immediateClock) After(time.Duration) <-chan time.Time {
	ch := make(chan time.Time, 1)
	ch <- c.now
	return ch
}

func combinedFixture(t *testing.T, provider *mutableReviewProvider) (ReviewStateReader, *Sink) {
	t.Helper()
	ledger, err := NewFileRecoveryLedger(filepath.Join(t.TempDir(), "recovery.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	sequence := 0
	reader := ReviewStateReader{Provider: provider, Recovery: ledger, Clock: &fixedRecoveryClock{now: now}, Epoch: func() (string, error) {
		sequence++
		return "epoch-" + strconv.Itoa(sequence), nil
	}}
	return reader, NewSink(filepath.Join(t.TempDir(), "events"), 0, nil)
}

func TestRunCombinedLiveReviewOnlyUsesOneSelectedFrontierAndZeroImplementation(t *testing.T) {
	provider := &mutableReviewProvider{rows: []ReviewObligationState{pendingReview("o-2"), pendingReview("o-1")}}
	reviews, sink := combinedFixture(t, provider)
	reader := &fakeReader{cadence: time.Second}
	implementationCalls := 0
	var requests []ReviewRecoveryRequest
	sum, err := RunCombinedLive(context.Background(), constantSource(Plan{Tasks: []Task{{ID: "done", Closed: true}}}), reader, reviews, RecoveryPolicy{Bound: 3}, "feature/001",
		func(context.Context) error { implementationCalls++; return nil },
		func(_ context.Context, request ReviewRecoveryRequest) error {
			requests = append(requests, request)
			provider.verify(request.ObligationIDs...)
			return nil
		},
		func() (string, error) { return "round-1", nil }, sink, immediateClock{}, nil)
	if err != nil || !sum.Completed {
		t.Fatalf("RunCombinedLive = %+v, err=%v", sum, err)
	}
	if implementationCalls != 0 {
		t.Fatalf("review-only recovery launched %d implementation sweeps", implementationCalls)
	}
	if len(requests) != 1 || !reflect.DeepEqual(requests[0].ObligationIDs, []string{"o-1", "o-2"}) {
		t.Fatalf("selected requests = %+v", requests)
	}
	if sum.ReviewSweeps != 1 {
		t.Fatalf("review sweeps = %d, want 1", sum.ReviewSweeps)
	}
	events, err := os.ReadFile(sink.current())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"kind":"review_recovery_decision"`, `"drive_round_id":"round-1"`, `"provider_attempt_count":0`, `"recovery_count":1`, `"recovery_bound":3`, `"action":"invoke_review_only"`} {
		if !strings.Contains(string(events), want) {
			t.Fatalf("recovery event missing %s: %s", want, events)
		}
	}
}

func TestRunCombinedLiveFailureConsumesBoundThenStopsWithoutImplementation(t *testing.T) {
	provider := &mutableReviewProvider{rows: []ReviewObligationState{pendingReview("o-1")}}
	reviews, sink := combinedFixture(t, provider)
	reader := &fakeReader{cadence: time.Second}
	implementationCalls, reviewCalls, round := 0, 0, 0
	sum, err := RunCombinedLive(context.Background(), constantSource(Plan{Tasks: []Task{{ID: "done", Closed: true}}}), reader, reviews, RecoveryPolicy{Bound: 2}, "feature/001",
		func(context.Context) error { implementationCalls++; return nil },
		func(context.Context, ReviewRecoveryRequest) error {
			reviewCalls++
			return errors.New("preflight failed")
		},
		func() (string, error) { round++; return "round-" + strconv.Itoa(round), nil }, sink, immediateClock{}, nil)
	if !errors.Is(err, ErrReviewExhausted) {
		t.Fatalf("error = %v, want ErrReviewExhausted", err)
	}
	if implementationCalls != 0 || reviewCalls != 2 || sum.ReviewSweeps != 2 {
		t.Fatalf("implementation=%d review=%d summary=%+v", implementationCalls, reviewCalls, sum)
	}
	events, readErr := os.ReadFile(sink.current())
	if readErr != nil || !strings.Contains(string(events), `"action":"exhausted"`) || !strings.Contains(string(events), `"evidence_ref":"sha256:`) || !strings.Contains(string(events), `"drive_round_id":"round-3"`) {
		t.Fatalf("exhaustion event=%s err=%v", events, readErr)
	}
}

func TestRunCombinedLiveUnknownRetriesOnceAndLaunchesNothing(t *testing.T) {
	provider := &mutableReviewProvider{err: errors.New("permission denied")}
	reviews, sink := combinedFixture(t, provider)
	reader := &fakeReader{cadence: time.Second}
	implementationCalls, reviewCalls, round := 0, 0, 0
	_, err := RunCombinedLive(context.Background(), constantSource(Plan{Tasks: []Task{{ID: "ready"}}}), reader, reviews, RecoveryPolicy{Bound: 3}, "feature/001",
		func(context.Context) error { implementationCalls++; return nil },
		func(context.Context, ReviewRecoveryRequest) error { reviewCalls++; return nil },
		func() (string, error) { round++; return "round-" + strconv.Itoa(round), nil }, sink, immediateClock{}, nil)
	if !errors.Is(err, ErrReviewStateUnknown) {
		t.Fatalf("error = %v, want ErrReviewStateUnknown", err)
	}
	if implementationCalls != 0 || reviewCalls != 0 || provider.calls != 2 {
		t.Fatalf("implementation=%d review=%d provider reads=%d", implementationCalls, reviewCalls, provider.calls)
	}
	events, readErr := os.ReadFile(sink.current())
	if readErr != nil {
		t.Fatal(readErr)
	}
	wire := string(events)
	for _, want := range []string{`"event":"review_state_unknown"`, `"scope":"feature/001"`, `"source":"obligation_store"`, `"kind":"permission_denied"`, `"action":"defer"`, `"retry_after_ms":1000`} {
		if !strings.Contains(wire, want) {
			t.Fatalf("unknown event missing %s: %s", want, wire)
		}
	}
	if strings.Contains(wire, `"failure_kind"`) || strings.Contains(wire, `"task_id":"feature/001"`) {
		t.Fatalf("unknown event carries divergent legacy fields: %s", wire)
	}
}
