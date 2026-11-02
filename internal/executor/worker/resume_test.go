package worker

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

// boundRuntime drives the bounded-execution tests. When blockUntilKill is set,
// Await parks until Kill is called — a backend with no native turn cap that would
// otherwise run forever, so the Worker's wall-clock budget is the only bound.
// Otherwise Await returns `result` promptly — a backend whose native cap
// self-terminated the process within the budget window. It counts launches and
// kills so a test can assert the bound fired (or did not).
type boundRuntime struct {
	blockUntilKill bool
	result         port.RunResult
	stopErr        error

	kill     chan struct{}
	killOnce sync.Once
	killed   atomic.Int32
	launched atomic.Int32
	awaiting chan struct{}
}

func newBoundRuntime(blockUntilKill bool) *boundRuntime {
	return &boundRuntime{blockUntilKill: blockUntilKill, kill: make(chan struct{}), awaiting: make(chan struct{})}
}

func (r *boundRuntime) AuthCheck(context.Context) error { return nil }

func (r *boundRuntime) ModelRoute() port.ModelRoute {
	return port.ModelRoute{Model: "fake-model", Effort: "fake-effort"}
}

func (r *boundRuntime) Launch(_ context.Context, spec port.LaunchSpec) (port.Handle, error) {
	r.launched.Add(1)
	r.result = describedResult(r.result, spec)
	return fakeHandle{}, nil
}

func (r *boundRuntime) Await(ctx context.Context, _ port.Handle) (port.RunResult, error) {
	close(r.awaiting)
	if !r.blockUntilKill {
		return r.result, nil
	}
	// Park until the bound kills us (or the context is cancelled). This is the
	// no-native-cap backend: it never finishes on its own within the window.
	select {
	case <-r.kill:
		return r.result, r.stopErr
	case <-ctx.Done():
		return r.result, ctx.Err()
	}
}

func (r *boundRuntime) Kill(context.Context, port.Handle) error {
	r.killed.Add(1)
	r.killOnce.Do(func() { close(r.kill) })
	return nil
}

var (
	_ port.AgentRuntime = (*boundRuntime)(nil)
	_ port.Handle       = fakeHandle{}
)

// boundDeps wires the shared inline fakes around a given runtime, so a resume
// test can swap only the runtime.
func boundDeps(t *testing.T, rt port.AgentRuntime) (Deps, *fakeLease, *fakeHost) {
	t.Helper()
	lease := &fakeLease{}
	host := &fakeHost{proposal: port.Proposal{Number: 1}, checkState: port.CheckGreen}
	deps := Deps{
		Tracker: &fakeTracker{},
		Lease:   lease,
		Space:   &fakeWorkspace{dir: t.TempDir()},
		Runtime: rt,
		Host:    host,
	}
	return deps, lease, host
}

// --- A-7 (worker half): the wall-clock bound kills a non-converging run --------

// A backend with no native turn cap that never finishes is bounded by the
// Worker's wall-clock budget: on expiry the process is Killed and the task exits
// non-converged and resumable — the claim released (fire-or-release), no proposal
// opened (the Worker exits at the bound, before verify), the WIP branch left for
// the respawn to resume (DINV-9, FR-13, §3.7).
func TestRun_WallClockBudgetKillsNonConverging(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		rt := newBoundRuntime(true) // never finishes on its own
		deps, lease, host := boundDeps(t, rt)
		const budget = 30 * time.Minute
		w := mustWorker(t, deps, WithBudget(budget))
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		var outcome Outcome
		var err error
		done := make(chan struct{})
		go func() {
			outcome, err = w.Run(ctx, newTestTask())
			close(done)
		}()
		<-rt.awaiting
		synctest.Wait() // the budget timer is armed and Await is blocked
		time.Sleep(budget - time.Nanosecond)
		synctest.Wait()
		if got := rt.killed.Load(); got != 0 {
			t.Fatalf("Kill called %d times before the deadline", got)
		}
		time.Sleep(time.Nanosecond)
		<-done
		if err != nil {
			t.Fatalf("Run: a budget-exhausted run is a non-error terminal, got err: %v", err)
		}
		if outcome != OutcomeUnconverged {
			t.Fatalf("outcome = %v, want unconverged (the bound tripped)", outcome)
		}
		if got := rt.killed.Load(); got != 1 {
			t.Errorf("Kill called %d times, want 1 (the bound kills the process)", got)
		}
		if got := rt.launched.Load(); got != 1 {
			t.Errorf("Launch called %d times, want 1", got)
		}
		// Resumable: the claim was released, and no proposal was opened this attempt.
		if lease.claim == nil || lease.claim.releases != 1 {
			t.Fatalf("claim not released exactly once (fire-or-release): %+v", lease.claim)
		}
		if len(host.openedTasks) != 0 {
			t.Errorf("OpenProposal called %d times on a bound-exhausted run, want 0", len(host.openedTasks))
		}

	})
}

func TestRun_BudgetCannotHideFailedProcessTeardown(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		rt := newBoundRuntime(true)
		rt.stopErr = port.ErrNotQuiescent
		deps, _, host := boundDeps(t, rt)
		w := mustWorker(t, deps, WithBudget(30*time.Minute))
		outcome, err := w.Run(t.Context(), newTestTask())
		space := deps.Space.(*fakeWorkspace)
		if outcome != OutcomeFailed || !errors.Is(err, port.ErrNotQuiescent) {
			t.Fatalf("failed teardown became resumable: outcome=%v err=%v", outcome, err)
		}
		if space.last.checkpoints != 0 || space.last.pushes != 0 || len(host.openedTasks) != 0 {
			t.Fatal("supervisor checkpointed or published a possibly live process")
		}

	})
}

type unacknowledgedRuntime struct {
	*boundRuntime
	release chan struct{}
	done    chan struct{}
	killErr error
}

func (r *unacknowledgedRuntime) Await(context.Context, port.Handle) (port.RunResult, error) {
	defer close(r.done)
	<-r.release
	return port.RunResult{}, nil
}

func (r *unacknowledgedRuntime) Kill(context.Context, port.Handle) error { return r.killErr }

func TestRun_UnacknowledgedKillNeverCheckpoints(t *testing.T) {
	for _, killErr := range []error{errors.New("process group cannot be killed"), nil} {
		t.Run(fmt.Sprint(killErr), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				rt := &unacknowledgedRuntime{boundRuntime: newBoundRuntime(true), release: make(chan struct{}), done: make(chan struct{}), killErr: killErr}
				t.Cleanup(func() {
					close(rt.release)
					if rt.launched.Load() != 0 {
						<-rt.done
					}
				})
				deps, _, host := boundDeps(t, rt)
				w := mustWorker(t, deps, WithBudget(30*time.Minute))
				outcome, err := w.Run(t.Context(), newTestTask())
				if outcome != OutcomeFailed || !errors.Is(err, port.ErrNotQuiescent) || (killErr != nil && !errors.Is(err, killErr)) {
					t.Fatalf("unacknowledged stop: outcome=%v err=%v", outcome, err)
				}
				space := deps.Space.(*fakeWorkspace)
				if space.last.checkpoints != 0 || space.last.pushes != 0 || len(host.openedTasks) != 0 {
					t.Fatal("unacknowledged kill checkpointed or published a possible live writer")
				}
			})
		})
	}
}

// --- A-7 (worker half): the bound holds uniformly across backends -------------

// A backend whose native turn cap self-terminated the process within the budget
// converges normally — the uniform wall-clock bound does NOT fire and the process
// is never wall-clock-killed. Together with the test above this is the uniformity
// property (plan T11 criterion 3): the turn bound holds for a native-cap backend
// (by native completion) and a no-native-cap backend (by the wall-clock kill)
// alike — neither runs unbounded (§3.7, R4).
func TestRun_NativeCapBackendConvergesWithinBudget(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		rt := newBoundRuntime(false)                      // returns promptly
		rt.result = port.RunResult{Tokens: 500, Turns: 8} // bounded by the native cap
		deps, _, _ := boundDeps(t, rt)
		w := mustWorker(t, deps, WithBudget(time.Hour)) // generous budget; native cap wins

		outcome, err := w.Run(context.Background(), newTestTask())
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		if outcome != OutcomeSucceeded {
			t.Fatalf("outcome = %v, want succeeded (converged within the uniform budget)", outcome)
		}
		if got := rt.killed.Load(); got != 0 {
			t.Errorf("Kill called %d times for a converged run, want 0 (no wall-clock kill)", got)
		}

	})
}

// A context cancelled mid-run under a budget kills the in-flight process and
// surfaces the cancellation as an operational failure (not a respawn-eligible
// non-convergence) — the Worker honors cancellation promptly, and the claim is
// still released.
func TestRun_CancelledUnderBudgetKillsAndReleases(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		rt := newBoundRuntime(true) // parks in Await until cancelled
		deps, lease, _ := boundDeps(t, rt)
		w := mustWorker(t, deps, WithBudget(time.Hour)) // budget won't fire; cancel wins

		ctx, cancel := context.WithCancel(context.Background())
		type ret struct {
			outcome Outcome
			err     error
		}
		got := make(chan ret, 1)
		go func() {
			o, e := w.Run(ctx, newTestTask())
			got <- ret{o, e}
		}()
		// Cancel only after Await has begun, with no scheduler deadline.
		defer cancel()
		<-rt.awaiting
		cancel()

		r := <-got
		if r.err == nil {
			t.Fatal("Run: want an error for a cancelled run")
		}
		if r.outcome != OutcomeFailed {
			t.Fatalf("outcome = %v, want failed (cancellation is operational, not non-convergence)", r.outcome)
		}
		if got := rt.killed.Load(); got != 1 {
			t.Errorf("Kill called %d times on cancellation, want 1", got)
		}
		if lease.claim == nil || lease.claim.releases != 1 {
			t.Fatalf("claim not released exactly once on cancellation: %+v", lease.claim)
		}

	})
}

// --- option + enum hygiene ----------------------------------------------------

// A non-positive budget is ignored (the base path is never silently disabled),
// and a positive one is installed — mirroring WithGreenDriver's nil guard.
func TestWithBudget_NonPositiveIgnored(t *testing.T) {
	deps, _, _ := boundDeps(t, newBoundRuntime(false))
	for _, d := range []time.Duration{0, -time.Second} {
		w := mustWorker(t, deps, WithBudget(d))
		if w.budget != 0 {
			t.Errorf("WithBudget(%v) set budget = %v, want 0 (non-positive ignored)", d, w.budget)
		}
	}
	w := mustWorker(t, deps, WithBudget(30*time.Second))
	if w.budget != 30*time.Second {
		t.Errorf("budget = %v, want 30s", w.budget)
	}
}

func TestOutcomeUnconvergedString(t *testing.T) {
	if got := OutcomeUnconverged.String(); got != "unconverged" {
		t.Errorf("OutcomeUnconverged.String() = %q, want %q", got, "unconverged")
	}
}
