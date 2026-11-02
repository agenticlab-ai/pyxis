package verdict

import "errors"

// Sentinel errors — part of the package contract, one per fail-closed condition,
// so a caller/test can branch with errors.Is.
var (
	// ErrNoRounds — Compute was called on a review with no rounds. A verdict is
	// derived from a round's findings, so a review must have at least one round
	// before it has a live verdict; Compute fails closed rather than invent one.
	ErrNoRounds = errors.New("verdict: cannot compute a verdict for a review with no rounds")

	// ErrIncompleteBrief — an escalate condition was signalled but its brief is
	// hollow (no conflict, no option, or no decision needed). An escalate must
	// carry an actionable FR-17 brief; Compute refuses an escalate verdict with
	// a brief the stage-9 human cannot act on.
	ErrIncompleteBrief = errors.New("verdict: escalate signalled with an incomplete decision brief (needs conflict, options, and decision-needed)")

	// ErrStaleVerdict — a helper that requires a current verdict was given a
	// stale one (the proposal head moved past the latest round's reviewed
	// revision). The live verdict is treated as absent (UB-6, DINV-7); a
	// post-adoption raise on a stale verdict is refused.
	ErrStaleVerdict = errors.New("verdict: the live verdict is stale (proposal head ≠ latest reviewed revision) — treated as absent")
)
