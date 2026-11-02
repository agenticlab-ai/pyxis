package verdict

import (
	"fmt"

	"github.com/agenticlab-ai/pyxis/internal/implreview/report"
)

// LiveVerdict is a live verdict paired with its staleness against the proposal's
// current head — the value a reader (the emitted signal, a lane, stage 9) must
// consult through [IsStale] / [Current] rather than trusting the report's
// Verdict field blind. A verdict certifies the exact tree it evaluated (AP-13);
// once the head moves past that tree, the verdict is STALE and MUST be treated
// as ABSENT, never current (UB-6, DINV-7).

// IsStale reports whether the report's live verdict is stale: it is stale iff
// the proposal's current head differs from the LATEST round's reviewed_revision
// (DINV-7). Each round binds its own reviewed_revision (a fix between rounds is
// a new sha — §3.1), and the LIVE verdict certifies the latest round's tree; so
// staleness is judged against the latest round alone.
//
// A report with no rounds has no reviewed tree to certify, so its verdict is
// treated as stale (there is nothing current to be stale against — fail closed
// to "not current"). An empty head is likewise stale: a reader that cannot name
// the current head cannot certify the verdict matches it.
//
// This is a PURE predicate over (report, head): no clock, no IO. The head is
// the proposal's current head sha, supplied by the caller (the host reads it
// from the code medium); this package does not fetch it.
func IsStale(r *report.Report, head string) bool {
	if r == nil || len(r.Rounds) == 0 {
		return true
	}
	if head == "" {
		return true
	}
	latest := r.Rounds[len(r.Rounds)-1]
	return head != latest.ReviewedRevision
}

// Current returns the report's live verdict ONLY when it is not stale, and the
// empty string when it is stale — the "treat a stale verdict as absent"
// realization (UB-6, DINV-7). A reader deciding whether a verdict routes
// anything MUST go through Current (or IsStale): a stale verdict is absent, so
// it routes nothing and certifies nothing.
//
// The bool return is false when the verdict is absent (stale, no rounds, or an
// empty report verdict), true when a current verdict is present. This makes the
// absence explicit at the call site rather than hiding behind an empty string a
// caller might misread as "approve".
func Current(r *report.Report, head string) (liveVerdict string, present bool) {
	if IsStale(r, head) {
		return "", false
	}
	if r.Verdict == "" {
		// A report whose envelope verdict was never set has no live verdict to
		// be current — absent, not a silent approve.
		return "", false
	}
	return r.Verdict, true
}

// LatestReviewedRevision returns the head sha the live verdict certifies — the
// latest round's reviewed_revision — and whether one exists. It is the value a
// caller compares the current head against; exposed so a caller can log or
// record WHICH tree a stale verdict certified versus the head that moved.
func LatestReviewedRevision(r *report.Report) (revision string, ok bool) {
	if r == nil || len(r.Rounds) == 0 {
		return "", false
	}
	rev := r.Rounds[len(r.Rounds)-1].ReviewedRevision
	if rev == "" {
		return "", false
	}
	return rev, true
}

// StaleError is returned by helpers that require a current verdict but were
// given a stale one — a named sentinel so a caller (a post-adoption raise that
// must refuse a stale verdict, UB-6) can branch with errors.Is. The core
// predicates above return bools; this is for the callers that want an error.
func StaleError(r *report.Report, head string) error {
	if !IsStale(r, head) {
		return nil
	}
	rev, ok := LatestReviewedRevision(r)
	if !ok {
		return fmt.Errorf("%w: no reviewed round (head %q)", ErrStaleVerdict, head)
	}
	return fmt.Errorf("%w: head %q ≠ latest reviewed revision %q", ErrStaleVerdict, head, rev)
}
