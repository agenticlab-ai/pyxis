package dispatch

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/implreview/lifecycle"
	"github.com/agenticlab-ai/pyxis/internal/implreview/report"
)

// spyReinvoker records each orphaned-review re-invoke and returns a scripted
// error keyed by proposal number, so the reap→re-invoke wiring is tested without
// the real reviewer runtime.
type spyReinvoker struct {
	seen   []int         // proposal numbers re-invoked, in order
	errFor map[int]error // scripted error per proposal number
}

func (s *spyReinvoker) Reinvoke(_ context.Context, p port.Proposal) error {
	s.seen = append(s.seen, p.Number)
	if s.errFor != nil {
		return s.errFor[p.Number]
	}
	return nil
}

var _ ReviewReinvoker = (*spyReinvoker)(nil)

// baseTime is a fixed instant the reap tests measure from — the clock is
// injected via `now`, mirroring lifecycle.InFlight's hermetic posture (no
// wall-clock read anywhere in the reap).
var baseReapTime = time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)

// startedReview is a persisted review with n rounds and the given live verdict —
// an empty verdict is a started-round-no-verdict (in-flight-eligible) review; a
// stamped verdict is a completed one. Mirrors the lifecycle package's own seed.
func startedReview(verdict string, nRounds int) *report.Report {
	rounds := make([]report.Round, 0, nRounds)
	for i := 0; i < nRounds; i++ {
		rounds = append(rounds, report.Round{ReviewedRevision: "sha-" + string(rune('a'+i))})
	}
	return &report.Report{
		SchemaVersion: report.SchemaVersion,
		ProposalRef:   "github:agenticlab-ai/pyxis#0",
		ReviewID:      "rv-1",
		Verdict:       verdict,
		Rounds:        rounds,
	}
}

// reapDispatcher builds a Dispatcher over the package's port fakes — the reaper
// touches none of them (it only needs a constructed Dispatcher), so their zero
// values suffice. It reuses the existing test fakes (fakeTracker, …) rather than
// inventing new ones.
func reapDispatcher(t *testing.T) *Dispatcher {
	t.Helper()
	return mustDispatcher(t, 1, &fakeTracker{}, &fakeLease{}, &fakeWorkspace{}, &fakeRuntime{}, &fakeHost{})
}

// The host janitor reaps a SEEDED orphaned in-flight review through the
// lifecycle predicate and re-invokes it, while leaving a live review (within its
// timeout) and a completed review (terminal verdict) untouched
// (prd:impl-review/UB-8, design §3.4/§3.10, AP-12 deviation).
func TestReapOrphanedReviews_ReapsOrphanReinvokesIt(t *testing.T) {
	const timeout = 30 * time.Minute
	cfg := lifecycle.Config{AbsentTimeout: timeout, MaxAttempts: 3}
	now := baseReapTime.Add(timeout + time.Minute) // past the timeout for a round started at baseReapTime

	candidates := []OrphanCandidate{
		{
			// ORPHAN: started round, no verdict, started PAST the timeout → reaped.
			Proposal: port.Proposal{Number: 412},
			State:    lifecycle.InFlightState{Report: startedReview("", 1), RoundStartedAt: baseReapTime},
		},
		{
			// LIVE: started round, no verdict, but started only a minute before now
			// (well within the timeout) → a slow deep-band review, left alone.
			Proposal: port.Proposal{Number: 500},
			State:    lifecycle.InFlightState{Report: startedReview("", 1), RoundStartedAt: now.Add(-time.Minute)},
		},
		{
			// COMPLETE: a terminal verdict, long past the timeout → nothing to reap.
			Proposal: port.Proposal{Number: 600},
			State:    lifecycle.InFlightState{Report: startedReview(report.VerdictApprove, 1), RoundStartedAt: baseReapTime},
		},
	}
	inv := &spyReinvoker{}

	rep, err := reapDispatcher(t).ReapOrphanedReviews(context.Background(), candidates, inv, cfg, now)
	if err != nil {
		t.Fatalf("ReapOrphanedReviews: %v", err)
	}

	// Only the orphan (#412) was re-invoked.
	if len(inv.seen) != 1 || inv.seen[0] != 412 {
		t.Fatalf("re-invoked %v, want exactly [412] (only the orphan)", inv.seen)
	}
	if rep.Reaped != 1 {
		t.Errorf("Reaped = %d, want 1", rep.Reaped)
	}
	if rep.Live != 1 {
		t.Errorf("Live = %d, want 1 (the within-timeout review)", rep.Live)
	}
	if len(rep.ReinvokeErrs) != 0 {
		t.Errorf("ReinvokeErrs = %v, want none", rep.ReinvokeErrs)
	}
}

// A re-invoke that ERRORS is recorded in the report but never strands the other
// orphans: a second orphan after a failing one is still reaped (fail-open across
// siblings, the janitor posture).
func TestReapOrphanedReviews_ReinvokeErrorDoesNotStrandSiblings(t *testing.T) {
	const timeout = 10 * time.Minute
	cfg := lifecycle.Config{AbsentTimeout: timeout, MaxAttempts: 3}
	now := baseReapTime.Add(timeout + time.Minute)

	reinvokeErr := errors.New("reviewer backend unavailable")
	candidates := []OrphanCandidate{
		{Proposal: port.Proposal{Number: 1}, State: lifecycle.InFlightState{Report: startedReview("", 1), RoundStartedAt: baseReapTime}},
		{Proposal: port.Proposal{Number: 2}, State: lifecycle.InFlightState{Report: startedReview("", 1), RoundStartedAt: baseReapTime}},
	}
	inv := &spyReinvoker{errFor: map[int]error{1: reinvokeErr}}

	rep, err := reapDispatcher(t).ReapOrphanedReviews(context.Background(), candidates, inv, cfg, now)
	if err != nil {
		t.Fatalf("ReapOrphanedReviews: %v", err)
	}

	// BOTH orphans were reaped despite the first re-invoke failing.
	if len(inv.seen) != 2 || inv.seen[0] != 1 || inv.seen[1] != 2 {
		t.Fatalf("re-invoked %v, want [1 2] (a failing re-invoke must not strand the sibling)", inv.seen)
	}
	if rep.Reaped != 2 {
		t.Errorf("Reaped = %d, want 2", rep.Reaped)
	}
	// The failing re-invoke's error is recorded, and it wraps the underlying error.
	if got, ok := rep.ReinvokeErrs[1]; !ok || !errors.Is(got, reinvokeErr) {
		t.Fatalf("ReinvokeErrs[1] = %v (ok=%v), want it to wrap the re-invoke error", got, ok)
	}
	if _, ok := rep.ReinvokeErrs[2]; ok {
		t.Errorf("ReinvokeErrs has an entry for #2, but its re-invoke succeeded")
	}
}

// A nil candidate set (nothing in flight) reaps nothing and is a clean no-op —
// the additive posture: no review in flight ⇒ existing behavior unchanged.
func TestReapOrphanedReviews_EmptyIsNoOp(t *testing.T) {
	cfg := lifecycle.Config{AbsentTimeout: 5 * time.Minute, MaxAttempts: 3}
	inv := &spyReinvoker{}

	rep, err := reapDispatcher(t).ReapOrphanedReviews(context.Background(), nil, inv, cfg, baseReapTime)
	if err != nil {
		t.Fatalf("ReapOrphanedReviews(nil): %v", err)
	}
	if len(inv.seen) != 0 {
		t.Fatalf("re-invoked %v for an empty candidate set, want none", inv.seen)
	}
	if rep.Reaped != 0 || rep.Live != 0 {
		t.Errorf("report = %+v, want a zero report", rep)
	}
}

// A nil reinvoker is a fail-closed wiring error (reaping needs a reviewer to
// re-invoke) — never a silent no-op.
func TestReapOrphanedReviews_NilReinvokerFailsClosed(t *testing.T) {
	cfg := lifecycle.Config{AbsentTimeout: 5 * time.Minute, MaxAttempts: 3}
	_, err := reapDispatcher(t).ReapOrphanedReviews(context.Background(), nil, nil, cfg, baseReapTime)
	if err == nil {
		t.Fatal("want an error for a nil ReviewReinvoker")
	}
}

// A mis-set lifecycle config (zero absent-timeout) is a fail-closed error rather
// than a silent "never reap" — the closed-parse posture applied to the reap's
// own data (a garbage timeout would make InFlight never trip).
func TestReapOrphanedReviews_InvalidConfigFailsClosed(t *testing.T) {
	inv := &spyReinvoker{}
	candidates := []OrphanCandidate{
		{Proposal: port.Proposal{Number: 1}, State: lifecycle.InFlightState{Report: startedReview("", 1), RoundStartedAt: baseReapTime}},
	}
	_, err := reapDispatcher(t).ReapOrphanedReviews(context.Background(), candidates, inv, lifecycle.Config{ /* zero: invalid */ }, baseReapTime)
	if !errors.Is(err, lifecycle.ErrInvalidConfig) {
		t.Fatalf("err = %v, want it to wrap lifecycle.ErrInvalidConfig", err)
	}
	if len(inv.seen) != 0 {
		t.Errorf("re-invoked %v despite an invalid config, want none (fail closed, no reap)", inv.seen)
	}
}

// Cancellation is honored promptly: a cancelled context stops the reap and
// returns the context error, leaving un-reached candidates for a later pass.
func TestReapOrphanedReviews_HonorsCancellation(t *testing.T) {
	cfg := lifecycle.Config{AbsentTimeout: 5 * time.Minute, MaxAttempts: 3}
	now := baseReapTime.Add(time.Hour) // everything is past the timeout
	candidates := []OrphanCandidate{
		{Proposal: port.Proposal{Number: 1}, State: lifecycle.InFlightState{Report: startedReview("", 1), RoundStartedAt: baseReapTime}},
	}
	inv := &spyReinvoker{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancelled before the reap runs

	_, err := reapDispatcher(t).ReapOrphanedReviews(ctx, candidates, inv, cfg, now)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if len(inv.seen) != 0 {
		t.Errorf("re-invoked %v under a cancelled context, want none", inv.seen)
	}
}
