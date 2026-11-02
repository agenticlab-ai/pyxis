package watch

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

type fakeReader struct {
	states  []port.ProposalState
	calls   atomic.Int64
	cadence time.Duration
}

func (r *fakeReader) States(context.Context, []port.ProposalRef, port.Cursor) ([]port.ProposalState, error) {
	r.calls.Add(1)
	return append([]port.ProposalState(nil), r.states...), nil
}
func (r *fakeReader) Cadence(context.Context) time.Duration { return r.cadence }

type fakeClock struct{ ticks chan time.Time }

func (c *fakeClock) Now() time.Time                       { return time.Time{} }
func (c *fakeClock) After(time.Duration) <-chan time.Time { return c.ticks }

func TestDeriveKindsAndCompletion(t *testing.T) {
	ref := port.ProposalRef{Repo: "r", Number: 1}
	r := &fakeReader{states: []port.ProposalState{{Landing: port.LandingOpen}}, cadence: time.Second}
	set, err := Derive(context.Background(), Plan{Tasks: []Task{
		{ID: "open", Proposal: &ref},
		{ID: "claimed", Claimed: true},
		{ID: "ready"},
		{ID: "blocked", BlockedBy: []string{"ready"}},
	}}, r)
	if err != nil {
		t.Fatal(err)
	}
	if len(set.Awaits) != 3 {
		t.Fatalf("awaits = %+v, want three kinds", set.Awaits)
	}
	if set.Awaits[0].Kind != AwaitClaimedNoProposal || set.Awaits[1].Kind != AwaitOpenProposal || set.Awaits[2].Kind != AwaitReadyUnclaimed {
		t.Fatalf("awaits = %+v, want sorted claimed/open/ready", set.Awaits)
	}
	complete, err := Derive(context.Background(), Plan{Tasks: []Task{{ID: "done", Closed: true}}}, r)
	if err != nil || !complete.Complete() {
		t.Fatalf("completion = %+v, err=%v", complete, err)
	}
}

// openReader answers one state per ref, so a plan whose proposal set changes
// between derivations always satisfies fetchStates' count check. Every ref
// reads as an open proposal — the only proposal state these loop tests need.
type openReader struct {
	calls   atomic.Int64
	cadence time.Duration
}

func (r *openReader) States(_ context.Context, refs []port.ProposalRef, _ port.Cursor) ([]port.ProposalState, error) {
	r.calls.Add(1)
	out := make([]port.ProposalState, len(refs))
	for i := range out {
		out[i] = port.ProposalState{Landing: port.LandingOpen}
	}
	return out, nil
}
func (r *openReader) Cadence(context.Context) time.Duration { return r.cadence }

// livePlan is a goroutine-safe, swappable Plan: the loop's PlanSource reads it
// every tick while the test rewrites it to stand for durable state changing
// underneath — a proposal opening, a blocker merging.
type livePlan struct{ p atomic.Pointer[Plan] }

func newLivePlan(plan Plan) *livePlan {
	lp := &livePlan{}
	lp.p.Store(&plan)
	return lp
}

func (l *livePlan) set(plan Plan) { l.p.Store(&plan) }

// loop drives RunLive over a swappable plan, counting sweeps and derivations.
// Derivations are the tick counter: a plan with no open proposals reads
// nothing, so the reader's own call count cannot serve as one.
type loop struct {
	sweeps      atomic.Int64
	derivations atomic.Int64
	clock       *fakeClock
	cancel      context.CancelFunc
	done        chan error
}

func startLoop(t *testing.T, plan *livePlan, reader Reader) *loop {
	t.Helper()
	l := &loop{clock: &fakeClock{ticks: make(chan time.Time, 8)}, done: make(chan error, 1)}
	ctx, cancel := context.WithCancel(context.Background())
	l.cancel = cancel
	t.Cleanup(cancel)
	source := func(ctx context.Context) (Plan, error) {
		l.derivations.Add(1)
		return *plan.p.Load(), ctx.Err()
	}
	go func() {
		_, err := RunLive(ctx, source, reader, func(context.Context) error { l.sweeps.Add(1); return nil }, l.clock, nil)
		l.done <- err
	}()
	return l
}

// settle waits until the loop is durably blocked on cadence, instead of
// guessing from a counter that has stayed unchanged for a few milliseconds.
func (l *loop) settle(t *testing.T) {
	t.Helper()
	synctest.Wait()
	if l.derivations.Load() == 0 {
		t.Fatal("loop parked before its first derivation")
	}
}

// tick releases one cadence tick and lets the loop finish reacting to it.
func (l *loop) tick(t *testing.T) {
	t.Helper()
	l.clock.ticks <- time.Now()
	l.settle(t)
}

func (l *loop) stop(t *testing.T) {
	t.Helper()
	l.cancel()
	if err := <-l.done; !errors.Is(err, context.Canceled) {
		t.Fatalf("RunLive error = %v, want cancellation", err)
	}
}

// TestRunSweepsReadyFrontierOnceThenCadencePaces is DINV-9's stuck-frontier
// case over the fixture the invariant actually names (A-12: "a permanently-
// failing READY frontier"): the sweep changes nothing, so the ready frontier
// is identical on every derivation. It sweeps once immediately, then only at
// cadence — never a hot loop.
func TestRunSweepsReadyFrontierOnceThenCadencePaces(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		l := startLoop(t, newLivePlan(Plan{Tasks: []Task{{ID: "ready"}}}), &openReader{cadence: time.Second})
		l.settle(t)
		if got := l.sweeps.Load(); got != 1 {
			t.Fatalf("sweeps after first derivation = %d, want 1", got)
		}
		l.tick(t)
		l.tick(t)
		l.stop(t)
		if got := l.sweeps.Load(); got != 3 {
			t.Fatalf("sweeps = %d, want one immediate plus two cadence-paced", got)
		}

	})
}

// TestRunPollsAndNeverSweepsWhileOnlyOpenProposalsAwait guards the
// repeat-dispatch defect. With the derived state UNCHANGED, an open proposal
// awaits review or landing and no number of cadence ticks may re-invoke the
// executor — elapsed time is not a trigger. The superseded version of this
// test asserted the opposite over exactly this fixture (one immediate plus two
// cadence-paced sweeps), which is how the defect reached production green.
func TestRunPollsAndNeverSweepsWhileOnlyOpenProposalsAwait(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ref := port.ProposalRef{Repo: "r", Number: 1}
		l := startLoop(t, newLivePlan(Plan{Tasks: []Task{{ID: "open", Proposal: &ref}}}), &openReader{cadence: time.Second})
		l.settle(t)
		// The first derived state is itself an edge and gets its one wake.
		before := l.sweeps.Load()
		if before != 1 {
			t.Fatalf("sweeps for the first derived state = %d, want 1", before)
		}
		for range 3 {
			l.tick(t)
		}
		l.stop(t)
		if got := l.sweeps.Load(); got != before {
			t.Fatalf("sweeps = %d, want %d — an unchanged open proposal is never re-swept", got, before)
		}
		if got := l.derivations.Load(); got < 4 {
			t.Fatalf("derivations = %d, want the loop to keep polling", got)
		}

	})
}

// TestRunPollsAndNeverSweepsWhileOnlyClaimedTasksAwait — a claimed task's
// worker is still running; re-invoking the executor cannot help it and races it.
func TestRunPollsAndNeverSweepsWhileOnlyClaimedTasksAwait(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		l := startLoop(t, newLivePlan(Plan{Tasks: []Task{{ID: "claimed", Claimed: true}}}), &openReader{cadence: time.Second})
		l.settle(t)
		before := l.sweeps.Load()
		l.tick(t)
		l.tick(t)
		l.stop(t)
		if got := l.sweeps.Load(); got != before {
			t.Fatalf("sweeps = %d, want %d — a claimed task's worker is already running", got, before)
		}

	})
}

// TestRunWakesOncePerTransitionThenStops reproduces the production incident: a
// ready task is swept, the sweep starts it, and its state becomes
// open-proposal — the sweep's own success.
//
// That transition IS a genuine wake edge, so the loop delivers exactly one
// sweep for it and lets the sweep re-derive and decide (§3.1 step 3, DINV-2/3;
// the executor's own preflight is what declines to restart the proposed task).
// The defect was everything after: a further sweep for every cadence tick,
// which is what turned one transition into four reruns.
func TestRunWakesOncePerTransitionThenStops(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ref := port.ProposalRef{Repo: "r", Number: 960}
		plan := newLivePlan(Plan{Tasks: []Task{{ID: "T3"}}})
		l := startLoop(t, plan, &openReader{cadence: time.Second})
		l.settle(t)
		if got := l.sweeps.Load(); got != 1 {
			t.Fatalf("sweeps for the ready task = %d, want 1", got)
		}
		plan.set(Plan{Tasks: []Task{{ID: "T3", Proposal: &ref}}}) // the sweep opened it
		l.tick(t)
		if got := l.sweeps.Load(); got != 2 {
			t.Fatalf("sweeps after the transition = %d, want 2 — one wake for the genuine edge", got)
		}
		for range 4 {
			l.tick(t)
		}
		l.stop(t)
		if got := l.sweeps.Load(); got != 2 {
			t.Fatalf("sweeps = %d, want 2 — every tick after the edge polls, never re-invokes", got)
		}

	})
}

// A landing is a genuine transition and must still wake the loop even though
// nothing becomes ready as a result. Suppressing non-ready edges would drop the
// design's wake contract (§3.1 step 3, DINV-2/3) — the overcorrection this
// guards against.
func TestRunWakesOnATransitionWithNothingReady(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ref := port.ProposalRef{Repo: "r", Number: 1}
		plan := newLivePlan(Plan{Tasks: []Task{
			{ID: "T1", Proposal: &ref},
			{ID: "T2", Claimed: true},
		}})
		l := startLoop(t, plan, &openReader{cadence: time.Second})
		l.settle(t)
		if got := l.sweeps.Load(); got != 1 {
			t.Fatalf("initial sweeps = %d, want 1 for the first derived state", got)
		}
		// T1 lands and closes; T2 is still claimed, so nothing becomes ready.
		plan.set(Plan{Tasks: []Task{
			{ID: "T1", Proposal: &ref, Closed: true},
			{ID: "T2", Claimed: true},
		}})
		l.tick(t)
		l.stop(t)
		if got := l.sweeps.Load(); got != 2 {
			t.Fatalf("sweeps = %d, want 2 — a real transition wakes even with nothing ready", got)
		}

	})
}

// TestRunSweepsOnceWhenAMergeMakesADownstreamTaskReady — the wake that matters:
// T1's proposal lands and closes it, unblocking T2. Exactly one sweep follows.
func TestRunSweepsOnceWhenAMergeMakesADownstreamTaskReady(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ref := port.ProposalRef{Repo: "r", Number: 1}
		plan := newLivePlan(Plan{Tasks: []Task{
			{ID: "T1", Proposal: &ref},
			{ID: "T2", BlockedBy: []string{"T1"}},
		}})
		l := startLoop(t, plan, &openReader{cadence: time.Second})
		l.settle(t)
		before := l.sweeps.Load()
		l.tick(t)
		if got := l.sweeps.Load(); got != before {
			t.Fatalf("sweeps while T1 is in flight, state unchanged = %d, want %d", got, before)
		}
		plan.set(Plan{Tasks: []Task{ // T1 merged: its issue closed, so T2 is ready
			{ID: "T1", Proposal: &ref, Closed: true},
			{ID: "T2"},
		}})
		l.tick(t)
		l.stop(t)
		if got := l.sweeps.Load(); got != before+1 {
			t.Fatalf("sweeps after the merge = %d, want exactly one more (%d)", got, before+1)
		}

	})
}

func TestRunIdleAndCompletionDoNotRead(t *testing.T) {
	r := &fakeReader{cadence: time.Second}
	var sweeps int
	sum, err := Run(context.Background(), Plan{Tasks: []Task{{ID: "done", Closed: true}}}, r, func(context.Context) error { sweeps++; return nil }, nil)
	if err != nil || !sum.Completed || r.calls.Load() != 0 || sweeps != 0 {
		t.Fatalf("sum=%+v err=%v reads=%d sweeps=%d", sum, err, r.calls.Load(), sweeps)
	}
}

func TestRunRejectsInvalidCadence(t *testing.T) {
	r := &fakeReader{cadence: 0}
	_, err := Run(context.Background(), Plan{Tasks: []Task{{ID: "ready"}}}, r, func(context.Context) error { return nil }, nil)
	if err == nil {
		t.Fatal("Run accepted non-positive cadence")
	}
}
