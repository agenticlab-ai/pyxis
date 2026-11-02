package e2e

import (
	"context"
	"sync"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/dispatch"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/worker"
	"github.com/agenticlab-ai/pyxis/internal/executor/workspace"
)

// The reusability witness (A-9, DINV-10, NFR-5): the Worker core and the five
// ports are reused VERBATIM behind a second, remote-shaped dispatcher. These
// compile-time assertions are the load-bearing half — if the reusable surface had
// to change shape for a different runtime, THIS FILE WOULD NOT COMPILE:
//
//   - worker.New is the same constructor both dispatchers call.
//   - worker.Deps is the same five-port bundle.
//   - (*worker.Worker).Run has the exact same signature the local Dispatcher
//     invokes — a method expression pins it structurally.
var (
	_                                                                          = worker.New
	_                                                                          = worker.Deps{}
	_ func(*worker.Worker, context.Context, port.Task) (worker.Outcome, error) = (*worker.Worker).Run
)

// altDispatch is a SECOND dispatch runtime over the SAME worker.Worker and the
// SAME five ports — deliberately NOT dispatch.Dispatcher. Where the local
// Dispatcher walks the frozen plan's frontier, serializes claims behind a guard,
// bounds concurrency with a width semaphore, and fans a normalized event stream
// to two sinks, altDispatch does none of that: it models a REMOTE work-queue shape
// — a fixed pool of "lanes" pulling already-dispatched tasks off a channel, each
// lane running a fresh Worker as a remote node would pick up a job. The only thing
// shared below the seam is the Worker and its ports, used with zero change. This
// is the local↔remote reusability bet made concrete: a future remote/Docker
// Dispatcher is a second driver like this one, never a Worker rewrite (§2, DINV-10).
type altDispatch struct {
	deps  worker.Deps
	lanes int
}

// altResult is one task's terminal outcome under altDispatch.
type altResult struct {
	task    port.Task
	outcome worker.Outcome
	err     error
}

// run dispatches the given ready tasks across the lane pool and collects their
// outcomes. Each lane builds a fresh worker.New(a.deps) per task (a fresh unit per
// job — the Worker carries no cross-task state, DINV-11) and runs it over the
// shared ports. The goroutines have a definite owner and stop: the feeder closes
// the queue after the last task, every lane returns when the queue drains, and the
// collector closes results after the pool's WaitGroup settles — no leak.
func (a *altDispatch) run(ctx context.Context, tasks []port.Task) []altResult {
	lanes := max(a.lanes, 1)
	queue := make(chan port.Task)
	results := make(chan altResult)

	var wg sync.WaitGroup
	for range lanes {
		wg.Go(func() {
			for task := range queue {
				// The reusable unit, VERBATIM: the same constructor and Run the local
				// Dispatcher uses, over the same ports (DINV-10).
				w, err := worker.New(a.deps)
				if err != nil {
					results <- altResult{task: task, err: err}
					continue
				}
				outcome, runErr := w.Run(ctx, task)
				results <- altResult{task: task, outcome: outcome, err: runErr}
			}
		})
	}

	go func() {
		for _, task := range tasks {
			queue <- task
		}
		close(queue)
	}()
	go func() {
		wg.Wait()
		close(results)
	}()

	var out []altResult
	for r := range results {
		out = append(out, r)
	}
	return out
}

// freshPorts builds one fresh set of the five ports over its own git world — the
// real Workspace and Lease adapters, the two GitHub stubs and the agent-runtime
// stub — so a witness can run a task with no shared state from another test.
func freshPorts(t *testing.T) (*memTracker, *countingLease, *workspace.Workspace, *scriptRuntime, *gitHost) {
	t.Helper()
	world := newGitWorld(t)
	return &memTracker{}, world.newLease(t), world.newWorkspace(t), &scriptRuntime{}, newGitHost(world.origin)
}

// TestReusability_WorkerRunsUnchangedBehindSecondDispatcher runs real Workers
// behind the remote-shaped altDispatch — with NO change to worker.Worker or the
// ports — and asserts they reach the same succeeded terminal the local Dispatcher
// produces. The Worker core is proven runtime-agnostic (A-9, DINV-10).
func TestReusability_WorkerRunsUnchangedBehindSecondDispatcher(t *testing.T) {
	tracker, lease, space, runtime, host := freshPorts(t)
	deps := worker.Deps{Tracker: tracker, Lease: lease, Space: space, Runtime: runtime, Host: host}

	tasks := []port.Task{
		{ID: "R1", Number: 11, HeadBranch: "feat/R1", Brief: "impl R1 [[e2e:R1:green]]"},
		{ID: "R2", Number: 12, HeadBranch: "feat/R2", Brief: "impl R2 [[e2e:R2:green]]"},
		{ID: "R3", Number: 13, HeadBranch: "feat/R3", Brief: "impl R3 [[e2e:R3:green]]"},
	}

	alt := &altDispatch{deps: deps, lanes: 2}
	results := alt.run(context.Background(), tasks)

	if len(results) != len(tasks) {
		t.Fatalf("altDispatch returned %d results, want %d", len(results), len(tasks))
	}
	for _, r := range results {
		if r.err != nil {
			t.Errorf("task %s under altDispatch errored: %v", r.task.ID, r.err)
		}
		if r.outcome != worker.OutcomeSucceeded {
			t.Errorf("task %s outcome under altDispatch = %v, want succeeded", r.task.ID, r.outcome)
		}
	}
	// The same Workers really ran (a fresh process per task) and opened a proposal
	// each through the shared ports — the ports were exercised, not bypassed.
	if got := runtime.launchCount(); got != len(tasks) {
		t.Errorf("fresh processes launched = %d, want %d (one per task)", got, len(tasks))
	}
	if got := host.distinctProposals(); got != len(tasks) {
		t.Errorf("proposals opened = %d, want %d", got, len(tasks))
	}
	// Fire-or-release held under the second driver too (DINV-5): every claim freed.
	if got := lease.releaseCount(); got != len(tasks) {
		t.Errorf("claims released = %d, want %d", got, len(tasks))
	}
}

// TestReusability_SameTaskSameOutcomeUnderBothDispatchers runs the SAME task shape
// through the real dispatch.Dispatcher (its base path) and through the
// remote-shaped altDispatch, and asserts both reach a succeeded terminal. The
// Worker is dispatcher-agnostic: swapping the driver above it changes nothing
// about the outcome — the witness the remote fleet is "a second Dispatcher, not a
// rewrite" (§2).
func TestReusability_SameTaskSameOutcomeUnderBothDispatchers(t *testing.T) {
	newTask := func() port.Task {
		return port.Task{ID: "X1", Number: 21, HeadBranch: "feat/X1", Brief: "impl X1 [[e2e:X1:green]]"}
	}

	// Local Dispatcher (base path: single runtime, no resolver/factory/reporter).
	localTracker, localLease, localSpace, localRuntime, localHost := freshPorts(t)
	d, err := dispatch.New(
		dispatch.Deps{Tracker: localTracker, Lease: localLease, Space: localSpace, Runtime: localRuntime, Host: localHost},
		dispatch.Config{Width: 1},
	)
	if err != nil {
		t.Fatalf("build local dispatcher: %v", err)
	}
	localTracker.setFrontier(newTask())
	localRep, err := d.Sweep(context.Background())
	if err != nil {
		t.Fatalf("local sweep: %v", err)
	}
	if localRep.Succeeded != 1 {
		t.Fatalf("local Dispatcher: succeeded = %d, want 1", localRep.Succeeded)
	}

	// Remote-shaped altDispatch over an equivalent fresh port set.
	altTracker, altLease, altSpace, altRuntime, altHost := freshPorts(t)
	alt := &altDispatch{
		deps:  worker.Deps{Tracker: altTracker, Lease: altLease, Space: altSpace, Runtime: altRuntime, Host: altHost},
		lanes: 1,
	}
	altResults := alt.run(context.Background(), []port.Task{newTask()})
	if len(altResults) != 1 || altResults[0].outcome != worker.OutcomeSucceeded {
		t.Fatalf("altDispatch results = %+v, want one succeeded", altResults)
	}

	// Same task, same succeeded terminal, two different dispatch runtimes over one
	// unchanged Worker core.
	if localRep.Results[0].Class != dispatch.ClassSucceeded {
		t.Errorf("local class = %v, want succeeded", localRep.Results[0].Class)
	}
}
