package watch

import (
	"context"
	"fmt"
)

// PlanSource supplies the plan's current durable facts for one derivation:
// per task, its proposal ref, claim state, and closure. Run's static Plan is
// the degenerate constant source; a live watch (the verb) re-reads tracker
// reality each tick so a sweep's effects — a claim planted, a proposal opened,
// a task closed — reach the next derivation (design:wake-watch-v2#3.1 step 1:
// derivation is from durable state only, held by no loop memory — DINV-5).
type PlanSource func(ctx context.Context) (Plan, error)

// RunLive is Run over a per-tick PlanSource. The sweep decision has two arms:
//
//   - a genuine transition (the derived state changed) delivers exactly one
//     sweep, as the design's wake contract requires (§3.1 step 3, DINV-2/3);
//   - an UNCHANGED state re-sweeps at the reader's cadence only while the
//     frontier still has ready-unclaimed work (DINV-9's stuck-frontier retry).
//     With nothing ready, a cadence tick polls and never sweeps, however long
//     the open proposals or working claims persist.
//
// Completion exits, cancellation returns promptly. hints is the
// reserved push slot's read side (design:wake-watch-v2#3.7, PushSlot): a
// receive ends the current wait at once, exactly as an early cadence tick
// would — the same reads run and the same sweep decision follows, nothing
// more. A nil hints channel (no adapter wired) or one that has gone silent
// (a dead adapter) simply never wins the select, so cadence-only behavior is
// unchanged either way (FR-11).
func RunLive(ctx context.Context, source PlanSource, reader Reader, sweep Sweep, clock Clock, hints <-chan struct{}) (Summary, error) {
	if clock == nil {
		clock = realClock{}
	}
	if source == nil || reader == nil || sweep == nil {
		return Summary{}, fmt.Errorf("watch: source, reader and sweep are required")
	}
	var sum Summary
	var previous string
	forceSweep := false
	for {
		if err := ctx.Err(); err != nil {
			return sum, err
		}
		plan, err := source(ctx)
		if err != nil {
			return sum, fmt.Errorf("watch: plan source: %w", err)
		}
		set, err := Derive(ctx, plan, reader)
		if err != nil {
			return sum, err
		}
		sum.Awaits = len(set.Awaits)
		if set.Complete() {
			sum.Completed = true
			return sum, nil
		}
		// Two distinct questions, and the defect was answering only one of them.
		//
		// key — did durable reality MOVE? A change is a genuine transition, and
		// the design delivers exactly one wake per distinct state (§3.1 step 3),
		// letting the sweep re-derive and decide (DINV-2/3). Suppressing these
		// would drop the loop's real wake edges.
		//
		// readyKey — is there anything a sweep could START? Only that justifies
		// re-invoking the executor over an UNCHANGED state, which is precisely
		// DINV-9's cadence-paced stuck-frontier retry. Forcing a sweep on an
		// unchanged state with nothing ready is what restarted one task four
		// times against its own open proposal: elapsed time is not a trigger.
		key := set.key()
		readyKey := set.readyKey()
		if key != previous || (forceSweep && readyKey != "") {
			if err := ctx.Err(); err != nil {
				return sum, err
			}
			if err := sweep(ctx); err != nil {
				return sum, fmt.Errorf("watch: sweep: %w", err)
			}
			sum.Sweeps++
			forceSweep = false
			previous = key
			continue
		}
		previous = key
		// The cadence tick is consumed whether or not it produced a sweep, so a
		// non-actionable frontier can never bank a force for a later one.
		forceSweep = false
		sum.Polls++
		cadence := reader.Cadence(ctx)
		if cadence <= 0 {
			return sum, fmt.Errorf("watch: reader returned non-positive cadence %s", cadence)
		}
		select {
		case <-ctx.Done():
			return sum, ctx.Err()
		case <-clock.After(cadence):
		case <-hints:
		}
		forceSweep = true
	}
}

// constantSource adapts a static Plan to the PlanSource shape.
func constantSource(plan Plan) PlanSource {
	return func(context.Context) (Plan, error) { return plan, nil }
}
