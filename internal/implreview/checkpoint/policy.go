// Package checkpoint is the impl-review advisory ramp and the ten-review
// adoption checkpoint (docs/tech-design/impl-review-v1.md §3.3 the harvest feeds
// the checkpoint, §2; docs/prds/impl-review/effective.md §3.2 M1/M2 + thresholds,
// §3.3 the guardrails + the pre-registered ship rule, §5.7 EVT-5; plan T13 —
// FR-11, DOD-4).
//
// Two things live here, together because they are the same decision (the
// advisory→adopted RAMP and the CHECKPOINT that governs a promotion), and one
// load-bearing guarantee ties them:
//
//		Until an operator PROMOTES it, the reviewer's verdict GATES NOTHING. Findings
//		deliver and persist, but PR-raise and lane routing PROCEED REGARDLESS of the
//		verdict. Promotion is a recorded operator act — default off, per repo — and
//		ANY guardrail breach HOLDS the ramp at advisory (FR-11, §3.3).
//
//	  - The advisory gate ([Gate], gate.go) — the state a caller consults before
//	    letting a verdict block a raise or route a lane. Default [StateAdvisory]:
//	    [Gate.RaiseBlocked] and [Gate.LaneRoutingBlocked] return false for ANY
//	    verdict. Only after a recorded promotion ([Gate.Promote], per repo,
//	    default off) does a downgrade-only verdict block a raise / force the human
//	    lane. The gate NEVER auto-promotes.
//
//	  - The ten-review checkpoint ([Evaluate], checkpoint.go) — computes coverage,
//	    precision evaluated ONLY over ≥ the disposition-floor count of
//	    operator-disposed blocking findings, and the nit rate from the HARVESTED
//	    corpus ([harvest.Metrics] — the checkpoint CONSUMES the harvest, it never
//	    recomputes a metric), and emits the EVT-5 [Record]. A below-floor window
//	    is INCONCLUSIVE → the window EXTENDS, never a vacuous pass; a zero-blocking
//	    window HOLDS, likewise never a vacuous pass. It surfaces a
//	    disposition-engagement signal (dwell/burst) as an ADVISORY FLAG, never an
//	    auto-fail.
//
//	  - The ship rule ([Policy], this file; applied in [Evaluate]) — M1 met AND M3
//	    green AND the nit guardrail clean AND M2 ≥ threshold over the floor →
//	    verdicts MAY gate PR-raise ([Record.ShipRuleMet] is true, meaning the
//	    operator may now record the promotion). This package computes ELIGIBILITY;
//	    the promotion ACT is the operator's recorded [Gate.Promote]. Thresholds are
//	    the §3.2/§3.3 STARTING VALUES carried as data ([DefaultPolicy]).
//
// This package reads a derived corpus and computes a verdict about a ramp; it
// writes nothing and mutates no proposal. It imports the harvest (the corpus it
// reads) and the report package (the verdict enum a gated verdict is judged by).
// Stdlib only, per the repo Go rules.
package checkpoint

import "fmt"

// Policy is the ship rule's thresholds carried as VERSIONED DATA — the §3.2/§3.3
// starting values, amendment-tuned only (never a silent retune; §3.3's "changes
// only by reviewed data PR"). It is a closed struct: a config surface parses it
// closed (AP-6), and a checkpoint reads these values rather than hard-coding the
// numbers in logic, so tuning a threshold is a data change, not a code change.
//
// The four fields are the four dials of the pre-registered ship rule (§3.3):
//   - WindowSize — the "ten-review checkpoint" cadence (M1/M3/nit/M2 evaluated
//     over a window of this many reviews).
//   - DispositionFloor — M2 is evaluable ONLY over ≥ this many operator-disposed
//     blocking findings (§3.2's "≥ 5"); below it the window is inconclusive and
//     extends, never a vacuous pass.
//   - PrecisionThreshold — the M2 bar (§3.2's ≥ 0.70) precision must meet, over
//     the floor, for the ship rule to be met.
//   - NitP50Max — the nit guardrail (§3.3): operator `rejected` per review at
//     p50 must not exceed this, or the ramp holds at advisory.
type Policy struct {
	// WindowSize is the checkpoint cadence — the review count a checkpoint
	// evaluates over (the "ten-review checkpoint"). A checkpoint whose window
	// carries fewer than this many reviews has not yet reached the cadence.
	WindowSize int `json:"window_size"`
	// DispositionFloor is the minimum operator-disposed blocking-finding count M2
	// is evaluable over (§3.2's ≥ 5). Below it the precision window is inconclusive
	// (extend, never a vacuous pass).
	DispositionFloor int `json:"disposition_floor"`
	// PrecisionThreshold is the M2 bar precision must meet over the floor for the
	// ship rule to be met (§3.2's ≥ 0.70). A fraction in [0,1].
	PrecisionThreshold float64 `json:"precision_threshold"`
	// NitP50Max is the nit guardrail: operator `rejected` dispositions per review
	// at p50 must be ≤ this (§3.3), or the ramp holds at advisory.
	NitP50Max float64 `json:"nit_p50_max"`
}

// DefaultPolicy is the §3.2/§3.3 STARTING VALUES — the pre-registered ship rule's
// data as of the base PRD: a ten-review window, a ≥ 5 operator-disposed
// blocking-finding floor, a ≥ 70% precision bar over that floor, and a ≤ 3-per-
// review nit p50 guardrail. These are amendment-tuned only (§3.3); a repo may
// carry its own Policy, but the code hard-codes none of the numbers — they live
// here as data.
func DefaultPolicy() Policy {
	return Policy{
		WindowSize:         10,
		DispositionFloor:   5,
		PrecisionThreshold: 0.70,
		NitP50Max:          3,
	}
}

// Validate reports whether a Policy is well-formed — a config surface calls it
// after a closed parse so a malformed data file fails loudly rather than
// silently skewing the ship rule. A non-positive window or floor, a precision
// threshold outside [0,1], or a negative nit bound is a defect.
func (p Policy) Validate() error {
	if p.WindowSize <= 0 {
		return fmt.Errorf("checkpoint policy: window_size must be > 0, got %d", p.WindowSize)
	}
	if p.DispositionFloor <= 0 {
		return fmt.Errorf("checkpoint policy: disposition_floor must be > 0, got %d", p.DispositionFloor)
	}
	if p.PrecisionThreshold < 0 || p.PrecisionThreshold > 1 {
		return fmt.Errorf("checkpoint policy: precision_threshold must be in [0,1], got %v", p.PrecisionThreshold)
	}
	if p.NitP50Max < 0 {
		return fmt.Errorf("checkpoint policy: nit_p50_max must be >= 0, got %v", p.NitP50Max)
	}
	return nil
}
