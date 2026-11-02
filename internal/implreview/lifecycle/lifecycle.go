// Package lifecycle is the impl-review review lifecycle — the testable state
// machine, predicates, and config the reviewer's RUNTIME (T4) and the host
// JANITOR (T14) consume to make a review idempotently resumable, orphan-reapable
// by contract, and never a silent pass or an indefinite stall
// (docs/tech-design/impl-review-v1.md §3.4 lifecycle, §3.7 DINV-1, §3.10; plan
// T9 — UB-1, UB-8).
//
// The Go/skill split is deliberate. The actual reviewer execution (spawning the
// four sibling subagents) lives in the runtime (T4); the real reaper that acts
// on an orphaned review executes host-side (implement-now's janitor, T14 — the
// AP-12 reaper-location deviation). This package owns NONE of that. It owns the
// three CONTRACTS those consumers share, modeled as pure functions and types
// with the clock and attempt count INJECTED (hermetic — no real clock, no
// network, no goroutine):
//
//  1. Idempotent resume (UB-8) — [Decide] reads a persisted report's round
//     state as AUTHORITATIVE and returns whether to START, RESUME at round N, or
//     treat the review as COMPLETE. A re-invocation resumes where the persisted
//     state left off; a double-invocation is a NO-OP that never starts a fresh
//     round 1. The decision is deterministic from (rounds, terminal verdict).
//
//  2. The in-flight predicate (UB-8) — [InFlight] flags a review with a STARTED
//     round and NO terminal verdict once the CONFIGURED absent-timeout has
//     elapsed since the round started: an orphan the janitor reaps. WITHIN the
//     timeout the same review is a (possibly slow deep-band) LIVE review, not an
//     orphan; a review with a terminal verdict is never in-flight. The
//     absent-timeout is config DATA ([Config.AbsentTimeout]), never hard-coded —
//     the reap contract T14 consumes.
//
//  3. Recorded review-absence on bounded failure (UB-1) — [RecordAbsence] runs a
//     reviewer attempt under a BOUNDED retry and, when it errors or exceeds its
//     bound after that retry, records a review-absence on the proposal rather
//     than a silent pass or an indefinite stall. It REUSES T2's absence outcome
//     (persist.AbsenceRecordedError / persist.ErrRecordedAbsence) so the absence
//     is the SAME distinguishable, harvestable signal a persist failure records.
//     Advisory: the host proceeds flagged; post-adoption: the absence routes
//     HUMAN — both are the caller's step over the recorded outcome, not this
//     package's.
//
// This package imports report (T1) and persist (T2); it edits neither, and
// neither imports it (no cycle). Stdlib only otherwise, per the repo Go rules.
package lifecycle

import (
	"github.com/agenticlab-ai/pyxis/internal/implreview/report"
)

// isTerminalVerdict reports whether a report's ENVELOPE verdict is a terminal
// (review-complete) verdict — the "no terminal verdict" half of the in-flight
// predicate and the "resume vs complete" half of the resume decision.
//
// The split is exactly the T4/T7 boundary the runtime already draws: T4's Review
// produces a round and DELIBERATELY leaves the envelope verdict UNSET (an empty
// string); T7's SetEnvelope stamps one of the four downgrade-only verdicts once
// the review is finalized. So an EMPTY envelope verdict is a review that started
// (its round exists) but has NOT reached a terminal verdict — in-flight; any of
// the four downgrade-only verdicts is TERMINAL — the review completed its
// evaluation and persisted a verdict.
//
// Note "terminal" here means "the review reached a persisted verdict", NOT "the
// proposal is done": a request-changes bounces to the worker and an escalate
// routes a human, but in both the REVIEW itself completed and stamped a verdict,
// so neither is an orphan to reap. A between-rounds fix re-invokes the reviewer,
// which resumes (Decide) rather than being flagged in-flight — a completed round
// with a stamped verdict is never an orphan.
//
// An unrecognized non-empty verdict string is treated as terminal (a verdict was
// stamped, even if this build does not recognize its exact value — it is not the
// unset in-flight state); Validate (T1) is the authority that rejects a
// malformed verdict, not this predicate.
func isTerminalVerdict(v string) bool {
	switch v {
	case "":
		// The unset envelope verdict T4 leaves — the review has not reached a
		// terminal verdict yet (in-flight).
		return false
	default:
		// Any stamped verdict (the four downgrade-only values, or any non-empty
		// string) is a terminal verdict — the review persisted a verdict.
		return true
	}
}

// HasStartedRound reports whether a persisted report has at least one recorded
// round — the "started round" half of both the in-flight predicate and the
// resume decision. A review with zero rounds has not started (nothing to resume,
// nothing to reap); a review with ≥ 1 round has started evaluating.
//
// A nil report has not started. This mirrors report.Validate's "at least one
// round is required" rule (a valid persisted report always has ≥ 1 round), but
// this package fails SAFE on a zero-round or nil report (treat as not-started)
// rather than assuming validity — the lifecycle reasons about the persisted
// state as it FINDS it, including a partially-written one.
func HasStartedRound(r *report.Report) bool {
	return r != nil && len(r.Rounds) > 0
}

// TerminalVerdict reports whether a persisted report carries a terminal
// (review-complete) verdict — exported for a caller (the janitor, the runtime)
// that wants to ask directly rather than infer from a [Decision]. A nil report
// has no verdict, so it is not terminal.
func TerminalVerdict(r *report.Report) bool {
	if r == nil {
		return false
	}
	return isTerminalVerdict(r.Verdict)
}
