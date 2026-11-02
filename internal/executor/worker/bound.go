package worker

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

// errBudgetExhausted is the internal signal that a Worker's wall-clock budget
// tripped: the agent process was killed mid-run and the task did not converge.
// Run maps it to OutcomeUnconverged — the resumable, respawn-eligible terminal
// (DINV-9). It is unexported on purpose: the Outcome, not this error, is the
// contract the Dispatcher keys on.
var errBudgetExhausted = errors.New("worker: wall-clock budget exhausted")
var errNotQuiescent = port.ErrNotQuiescent

// WithBudget sets a uniform wall-clock budget the Worker enforces around the
// agent's run: if Await does not return within the budget, the Worker Kills the
// process and treats the task as non-converged (OutcomeUnconverged). This is the
// mechanical turn bound of DINV-9/FR-13 made uniform across backends — it holds
// for CLIs that carry no native turn cap (codex, opencode) exactly as a native
// cap does for those that do (claude's --max-turns, cursor), so no backend can
// run unbounded (§3.7, R4). A non-positive budget is ignored (no wall-clock
// bound; a backend's native cap, where present, still applies), so the option
// never silently disables the base path — mirroring WithGreenDriver's nil guard.
func WithBudget(d time.Duration) Option {
	return func(w *Worker) {
		if d > 0 {
			w.budget = d
		}
	}
}

// awaitPhase blocks for the agent process to finish, bounded by the Worker's
// wall-clock budget when one is set (WithBudget). With no budget it is a plain
// Await — the backend's native turn cap, where present, is the only bound. With a
// budget it races Await against a timer:
//
//   - Await returns first  → the process finished on its own; its result/error is
//     returned unchanged (a native cap that self-terminated lands here).
//   - the timer fires first → the budget is exhausted: the process is Killed and
//     errBudgetExhausted is returned, which Run maps to the resumable
//     OutcomeUnconverged (the checkpoint-exit, DINV-9/AP-16).
//   - the context is cancelled → the process is Killed and the cancellation is
//     returned as an operational error (honor cancellation promptly).
//
// The awaiting goroutine has a definite owner and stop: it always returns once
// Await returns, and Await is unblocked either by the process finishing on its
// own or by the Kill below. The channel is buffered (capacity 1) so the goroutine
// can send its result and exit even after this function has already returned on
// the timeout/cancel path — no goroutine leaks (AGENTS.md concurrency rule).
func (w *Worker) awaitPhase(ctx context.Context, handle port.Handle, phase string) (result port.RunResult, awaitErr error) {
	started := time.Now()
	w.activity(Activity{Phase: phase, Status: "started"})
	defer func() {
		status := "finished"
		if awaitErr != nil {
			status = "failed"
		}
		if errors.Is(awaitErr, errBudgetExhausted) {
			status = "budget_exhausted"
		}
		w.activity(Activity{Phase: phase, Status: status, ElapsedMS: time.Since(started).Milliseconds(), Usage: &result, Err: awaitErr})
	}()

	type awaitReturn struct {
		res port.RunResult
		err error
	}
	done := make(chan awaitReturn, 1)
	go func() {
		res, err := w.runtime.Await(ctx, handle)
		done <- awaitReturn{res: res, err: err}
	}()

	var budget <-chan time.Time
	if w.budget > 0 {
		timer := time.NewTimer(w.budget)
		defer timer.Stop()
		budget = timer.C
	}
	heartbeat := time.NewTicker(30 * time.Second)
	defer heartbeat.Stop()

await:
	for {
		select {
		case <-heartbeat.C:
			elapsed := time.Since(started)
			activity := Activity{Phase: phase, Status: "running", ElapsedMS: elapsed.Milliseconds()}
			if w.budget > 0 {
				remaining := max(int64(0), (w.budget - elapsed).Milliseconds())
				activity.RemainingMS = &remaining
			}
			w.activity(activity)
		case r := <-done:
			if ctx.Err() != nil {
				if err := w.runtime.Kill(context.WithoutCancel(ctx), handle); err != nil {
					return r.res, fmt.Errorf("%w: %w", errNotQuiescent, err)
				}
			}
			return r.res, r.err
		case <-budget:
			if err := w.runtime.Kill(context.WithoutCancel(ctx), handle); err != nil {
				return port.RunResult{}, fmt.Errorf("%w: %w", errNotQuiescent, err)
			}
			break await
		case <-ctx.Done():
			if err := w.runtime.Kill(context.WithoutCancel(ctx), handle); err != nil {
				return port.RunResult{}, errors.Join(ctx.Err(), fmt.Errorf("%w: %w", errNotQuiescent, err))
			}
			break await
		}
	}
	// A kill signal is not an exit acknowledgement. Drain Await before the
	// supervisor touches the index, preserving usage from the interrupted run.
	grace := time.NewTimer(5 * time.Second)
	defer grace.Stop()
	select {
	case r := <-done:
		if errors.Is(r.err, port.ErrNotQuiescent) {
			return r.res, errors.Join(r.err, ctx.Err())
		}
		if ctx.Err() != nil {
			return r.res, ctx.Err()
		}
		return r.res, errBudgetExhausted
	case <-grace.C:
		return port.RunResult{}, errNotQuiescent
	}
}
