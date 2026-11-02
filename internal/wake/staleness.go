package wake

import (
	"fmt"
	"time"
)

// The wake channel's one staleness predicate (self-drive-wake/001 T6; design-
// brainstorm:self-drive-wake-tech-design#D-1; wake-watch-v2 DINV-7: "Staleness
// is computed by exactly one predicate ... no second implementation exists").
// argusledger.WakeStaleness is the argus backstop's own ledger-aware surfacing
// (since-date parsing, defect formatting over an Await) and stays exactly as
// it is — this file adds the primitive comparison underneath "too old?" as a
// bare, parametrized function so a second wake stack (its own bound, its own
// clock reads) shares the identical age-vs-bound math instead of hand-rolling
// a twin (the twin-predicate drift class).

// Stale reports whether age exceeds bound — the wake-staleness predicate
// every consumer shares. The bound is exclusive: an age exactly at the bound
// is not yet stale, so an alarm can never fire on an await the very next tick
// at that cadence would legally serve (wake-channel-v1 §3.2). This matches
// argusledger.WakeStaleness's own bound-exclusive reading (`age > bound`) —
// the argus backstop's existing verdicts are this function's verdicts under
// its own bound, byte for byte.
func Stale(age, bound time.Duration) bool {
	return age > bound
}

// ValidateBound enforces the bound-strictly-exceeds-cadence relation every
// staleness consumer shares (wake-channel-v1 §3.2's pinned relation, restated
// at wake-watch-v2 §3.8: "the primitive relation inherited from wake-channel-
// v1 holds"): a staleness bound that does not strictly exceed the cadence
// serving it could fire on an await the very next tick would legally
// resolve. Fail closed — refuse the inverted or equal pair at construction
// rather than ship a flapping alarm.
func ValidateBound(bound, cadence time.Duration) error {
	if bound <= cadence {
		return fmt.Errorf("wake: staleness bound %s does not strictly exceed cadence %s", bound, cadence)
	}
	return nil
}
