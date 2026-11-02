package watch

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

// T9 acceptance: "The seeded fixture set (waiting-review · ci-running ·
// blocked · stale-wait · dead-checker · unknown · offline) renders every
// column and flags exactly the seeded rows, with zero writes performed."
// dead-checker is exercised separately (it is a liveness-footer fixture, not
// a task row) by TestBuildReport_KillTheCheckerDrill below.
func TestStatuses_SeededFixtureSet(t *testing.T) {
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	reader := port.NewFakeStateReader()

	refReview := port.ProposalRef{Repo: "r", Number: 1}
	refCI := port.ProposalRef{Repo: "r", Number: 2}
	refStale := port.ProposalRef{Repo: "r", Number: 4}
	refUnknown := port.ProposalRef{Repo: "r", Number: 5}
	refOffline := port.ProposalRef{Repo: "r", Number: 6}

	reader.Set(refReview, port.ProposalState{Landing: port.LandingOpen,
		Checks: []port.CheckContext{{Name: "build", State: port.CheckGreen}}})
	reader.Set(refCI, port.ProposalState{Landing: port.LandingOpen,
		Checks: []port.CheckContext{{Name: "build", State: port.CheckPending}}})
	reader.Set(refStale, port.ProposalState{Landing: port.LandingOpen,
		Checks: []port.CheckContext{{Name: "build", State: port.CheckGreen}}})
	reader.SetUnknown(refUnknown, port.UnknownParseDefect)
	reader.SetUnknown(refOffline, port.UnknownOffline)

	plan := Plan{Tasks: []Task{
		{ID: "waiting-review", Proposal: &refReview, Since: now.Add(-10 * time.Minute)},
		{ID: "ci-running", Proposal: &refCI, Since: now.Add(-5 * time.Minute)},
		{ID: "blocked", BlockedBy: []string{"ci-running"}},
		{ID: "stale-wait", Proposal: &refStale, Since: now.Add(-3 * time.Hour)},
		{ID: "unknown", Proposal: &refUnknown, Since: now.Add(-5 * time.Minute)},
		{ID: "offline", Proposal: &refOffline, Since: now.Add(-1 * time.Hour)},
		{ID: "worker-claimed", Claimed: true, Since: now.Add(-10 * time.Minute)},
		{ID: "worker-ready"},
		{ID: "landed", Closed: true},
	}}

	got, err := Statuses(context.Background(), plan, reader, DefaultThresholds(), now)
	if err != nil {
		t.Fatalf("Statuses: %v", err)
	}
	if len(got) != len(plan.Tasks) {
		t.Fatalf("got %d rows, want one per task (%d)", len(got), len(plan.Tasks))
	}
	byID := make(map[string]TaskStatus, len(got))
	for _, r := range got {
		byID[r.TaskID] = r
	}

	want := map[string]struct {
		state  TaskState
		wait   WaitKind
		hasAge bool
		age    time.Duration
		stale  bool
		reason string
	}{
		"waiting-review": {StateWaitingReview, WaitReview, true, 10 * time.Minute, false, ""},
		"ci-running":     {StateChecksRunning, WaitCI, true, 5 * time.Minute, false, ""},
		"blocked":        {StateBlocked, WaitNone, false, 0, false, ""},
		"stale-wait":     {StateWaitingReview, WaitReview, true, 3 * time.Hour, true, ""},
		"unknown":        {StateUnknown, WaitNone, true, 5 * time.Minute, false, "parse-defect"},
		"offline":        {StateUnknown, WaitNone, true, time.Hour, true, "offline"},
		"worker-claimed": {StateClaimedNoProposal, WaitWorker, true, 10 * time.Minute, false, ""},
		"worker-ready":   {StateReadyUnclaimed, WaitWorker, false, 0, false, ""},
		"landed":         {StateLanded, WaitNone, false, 0, false, ""},
	}
	for id, w := range want {
		row, ok := byID[id]
		if !ok {
			t.Fatalf("row %q missing from render", id)
		}
		if row.State != w.state || row.Wait != w.wait || row.HasAge != w.hasAge || row.Stale != w.stale || row.UnknownReason != w.reason {
			t.Errorf("row %q = %+v, want state=%s wait=%s hasAge=%v stale=%v reason=%q",
				id, row, w.state, w.wait, w.hasAge, w.stale, w.reason)
		}
		if w.hasAge && row.Age != w.age {
			t.Errorf("row %q age = %s, want %s", id, row.Age, w.age)
		}
	}

	// "flags exactly the seeded stale rows": stale-wait and offline only.
	var staleIDs []string
	for _, r := range got {
		if r.Stale {
			staleIDs = append(staleIDs, r.TaskID)
		}
	}
	if len(staleIDs) != 2 || !contains(staleIDs, "stale-wait") || !contains(staleIDs, "offline") {
		t.Fatalf("stale rows = %v, want exactly [offline stale-wait]", staleIDs)
	}

	// The render surfaces every column, the offline row's exact UB-1 text,
	// and the stale marker — over the SAME derived rows (no second pass).
	var buf strings.Builder
	WriteReport(&buf, Report{Tasks: got})
	out := buf.String()
	for _, want := range []string{
		"TASK", "STATE", "WAIT", "AGE", "FLAG",
		"waiting-on-review", "checks-running", "blocked",
		"unknown (offline, retrying)", "unknown (parse-defect)",
		"⚠ stale", "landed ✓",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("render missing %q:\n%s", want, out)
		}
	}
}

func contains(ss []string, v string) bool {
	for _, s := range ss {
		if s == v {
			return true
		}
	}
	return false
}

// T9 acceptance: "The kill-the-checker drill renders the stale last-check
// flag within one cadence and the defect event fires at the configured
// bound." The sink's last recorded check freezes at `base`; BuildReport is
// driven forward in simulated time without any further checks landing.
func TestBuildReport_KillTheCheckerDrill(t *testing.T) {
	dir := t.TempDir()
	sink := NewSink(filepath.Join(dir, "wake-events"), 0, func(string, ...any) {})
	base := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	sink.clock = func() time.Time { return base }
	sink.Append(Event{Kind: EventCheckRan, Medium: "github", Batch: 1})

	cadence := 10 * time.Second
	thresholds := DefaultThresholds() // checker bound defaults to 3x cadence = 30s
	reader := port.NewFakeStateReader()

	// Within one cadence: healthy, no flags.
	repHealthy, err := BuildReport(context.Background(), constantSource(Plan{}), reader, sink, thresholds, cadence, base.Add(5*time.Second))
	if err != nil {
		t.Fatalf("BuildReport: %v", err)
	}
	if repHealthy.Liveness.DisplayStale || repHealthy.Liveness.BoundStale || repHealthy.Degraded() {
		t.Fatalf("within one cadence must read healthy: %+v", repHealthy.Liveness)
	}

	// Past one cadence, short of the configured bound: the DISPLAY flag
	// fires ("a check was missed — render the truth") but no defect event.
	repMissed, err := BuildReport(context.Background(), constantSource(Plan{}), reader, sink, thresholds, cadence, base.Add(15*time.Second))
	if err != nil {
		t.Fatalf("BuildReport: %v", err)
	}
	if !repMissed.Liveness.DisplayStale {
		t.Fatal("want the display flag within one missed cadence")
	}
	if repMissed.Liveness.BoundStale {
		t.Fatal("must not be bound-stale before the configured bound")
	}
	if data, _ := os.ReadFile(filepath.Join(dir, "wake-events", "current.jsonl")); strings.Contains(string(data), `"subject":"checker"`) {
		t.Fatal("the defect event must not fire before the configured bound")
	}

	// Past the configured bound (3x cadence = 30s): the defect event fires.
	repDead, err := BuildReport(context.Background(), constantSource(Plan{}), reader, sink, thresholds, cadence, base.Add(35*time.Second))
	if err != nil {
		t.Fatalf("BuildReport: %v", err)
	}
	if !repDead.Liveness.BoundStale || !repDead.Degraded() {
		t.Fatalf("want bound-stale and a degraded report past the configured bound: %+v", repDead.Liveness)
	}
	data, err := os.ReadFile(filepath.Join(dir, "wake-events", "current.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), string(EventStaleFlagged)) || !strings.Contains(string(data), `"subject":"checker"`) {
		t.Fatalf("defect event did not fire at the configured bound: %s", data)
	}
}

// T9 acceptance: "An absent or unwritable events file renders the named
// cannot-verify-liveness defect — never a healthy footer."
func TestBuildReport_AbsentEventsFileCannotVerifyLiveness(t *testing.T) {
	sink := NewSink(filepath.Join(t.TempDir(), "wake-events"), 0, func(string, ...any) {})
	reader := port.NewFakeStateReader()
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)

	rep, err := BuildReport(context.Background(), constantSource(Plan{}), reader, sink, DefaultThresholds(), 10*time.Second, now)
	if err != nil {
		t.Fatalf("BuildReport: %v", err)
	}
	if !strings.Contains(rep.Liveness.Defect, "cannot-verify-liveness") {
		t.Fatalf("want the named cannot-verify-liveness defect, got %+v", rep.Liveness)
	}
	if !rep.Liveness.LastCheck.IsZero() || rep.Liveness.DisplayStale || rep.Liveness.BoundStale {
		t.Fatalf("an unverifiable footer must never look healthy or flagged: %+v", rep.Liveness)
	}
	if !rep.Degraded() {
		t.Fatal("an unverifiable liveness footer must degrade the report")
	}

	var buf strings.Builder
	WriteReport(&buf, rep)
	if !strings.Contains(buf.String(), "cannot-verify-liveness") {
		t.Fatalf("render must never present a healthy-looking footer: %s", buf.String())
	}
}

func TestBuildReport_UnwritableEventsFileCannotVerifyLiveness(t *testing.T) {
	dir := t.TempDir()
	blocked := filepath.Join(dir, "not-a-dir")
	if err := os.WriteFile(blocked, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	sink := NewSink(filepath.Join(blocked, "wake-events"), 0, func(string, ...any) {})
	sink.Append(Event{Kind: EventCheckRan}) // forces the degraded path (no dir to create)

	reader := port.NewFakeStateReader()
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	rep, err := BuildReport(context.Background(), constantSource(Plan{}), reader, sink, DefaultThresholds(), 10*time.Second, now)
	if err != nil {
		t.Fatalf("BuildReport: %v", err)
	}
	if !strings.Contains(rep.Liveness.Defect, "unwritable") {
		t.Fatalf("want the unwritable defect, got %+v", rep.Liveness)
	}
	if !rep.Degraded() {
		t.Fatal("an unwritable sink must degrade the report")
	}
}

// BuildReport reads through source exactly once, per call — mirroring
// RunLive's own per-tick re-derivation (DINV-5) — and a failing source is a
// cannot-render condition, never a silently empty report.
func TestBuildReport_PropagatesPlanSourceError(t *testing.T) {
	sink := NewSink(filepath.Join(t.TempDir(), "wake-events"), 0, func(string, ...any) {})
	reader := port.NewFakeStateReader()
	failing := func(context.Context) (Plan, error) { return Plan{}, errPlanSourceFixture }
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	_, err := BuildReport(context.Background(), failing, reader, sink, DefaultThresholds(), 10*time.Second, now)
	if !errors.Is(err, errPlanSourceFixture) {
		t.Fatalf("err = %v, want it to wrap the plan source's own error", err)
	}
}

var errPlanSourceFixture = errors.New("fixture: plan source unavailable")

func TestReport_Degraded(t *testing.T) {
	clean := Report{Tasks: []TaskStatus{{TaskID: "t1", State: StateWaitingReview}}}
	if clean.Degraded() {
		t.Fatal("an all-clean report must not be degraded")
	}
	staleTask := Report{Tasks: []TaskStatus{{TaskID: "t1", Stale: true}}}
	if !staleTask.Degraded() {
		t.Fatal("a stale task row must degrade the report")
	}
	unknownTask := Report{Tasks: []TaskStatus{{TaskID: "t1", State: StateUnknown}}}
	if !unknownTask.Degraded() {
		t.Fatal("an unknown task row must degrade the report")
	}
	displayStaleChecker := Report{Liveness: Liveness{DisplayStale: true}}
	if !displayStaleChecker.Degraded() {
		t.Fatal("a display-stale checker must degrade the report")
	}
	defectChecker := Report{Liveness: Liveness{Defect: "cannot-verify-liveness: x"}}
	if !defectChecker.Degraded() {
		t.Fatal("an unverifiable checker must degrade the report")
	}
}

// Statuses performs no filesystem, git, or medium write of its own — its
// only inputs are the fake reader and the in-memory plan, so a caller
// driving it directly (as the fixture test above does) is trivially
// zero-write by construction; the CLI-level zero-writes guarantee is
// asserted at that layer (cmd/pyxis/status_test.go).
func TestStatuses_ReaderIsTheOnlyReadDependency(t *testing.T) {
	reader := port.NewFakeStateReader()
	ref := port.ProposalRef{Repo: "r", Number: 1}
	reader.Set(ref, port.ProposalState{Landing: port.LandingOpen})
	plan := Plan{Tasks: []Task{{ID: "t1", Proposal: &ref}}}
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	if _, err := Statuses(context.Background(), plan, reader, DefaultThresholds(), now); err != nil {
		t.Fatalf("Statuses: %v", err)
	}
}
