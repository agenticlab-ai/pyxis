package band

import (
	"fmt"

	"github.com/agenticlab-ai/pyxis/internal/implreview/report"
)

// Oversize escalation (UB-7).
//
// A diff exceeding the band's honest full-read budget cannot be reviewed whole —
// so it is NEVER truncated-and-reviewed-partial. Instead it records a
// SIZE-ESCALATION outcome routing the proposal HUMAN, attesting the un-read
// extent, and the review NEVER emits an approve over a partially-read diff
// (UB-7, ABUSE-1's oversize-to-force-truncation evasion, EVAL-8).
//
// The budget is per-band DATA (a bigger diff is honestly readable in the deep
// band's larger context than in a skip): BandBudgets below is the versioned
// default, sourced as config in a later phase (the same data-as-config posture as
// the calibration table). This file owns the size gate and the escalation
// outcome; the budget VALUES are data.

// BandBudget is one band's honest read budget — the maximum diff size (in bytes)
// the band can read WHOLE. A diff over it is a size-escalation, not a partial
// review.
type BandBudget struct {
	// MaxDiffBytes is the largest diff (bytes) this band reads whole. Zero means
	// "no review reads a diff in this band" (the skip band reads none), so ANY
	// non-empty diff in a zero-budget review band is over budget — but skip runs
	// no lenses, so the gate is only consulted for standard/deep.
	MaxDiffBytes int
}

// BandBudgets is the versioned default per-band read budget (UB-7). The deep band
// carries a larger budget than the standard band — the deep band pays for a
// bigger honest read where the risk lives; the standard band's budget bounds the
// common case. These are STARTING VALUES tuned only by a reviewed data change
// (like the calibration thresholds), never a code edit, and sourced from the
// pyxis-config surface in a later phase.
//
// A band absent from this map has NO budget entry, which SizeCheck treats as
// fail-closed (any non-empty diff over-budget) rather than an unbounded read — an
// unknown band never silently reads an arbitrarily large diff.
var BandBudgets = map[Band]BandBudget{
	// The standard band's honest full-read budget — the common-case bound.
	report.BandStandard: {MaxDiffBytes: 256 * 1024},
	// The deep band reads a larger diff whole (the risk justifies the cost).
	report.BandDeep: {MaxDiffBytes: 1024 * 1024},
	// The skip band runs no lenses and reads no diff; its budget is zero (any
	// diff would be over budget, but the gate is not consulted for a skip).
	report.BandSkip: {MaxDiffBytes: 0},
}

// SizeEscalation is the recorded UB-7 outcome: the diff was too large for the
// band's honest read, so the proposal routes HUMAN and the un-read extent is
// attested. It is NOT an approve — a size-escalation is the opposite of an
// approve over a partial read.
type SizeEscalation struct {
	// Band is the band whose budget was exceeded.
	Band Band
	// DiffBytes is the actual diff size.
	DiffBytes int
	// BudgetBytes is the band's honest read budget the diff exceeded.
	BudgetBytes int
	// UnreadBytes is the extent (bytes) beyond the budget that could NOT be read
	// honestly — the "un-reviewed extent" UB-7 requires the report to attest.
	UnreadBytes int
	// RouteHuman is always true — a size-escalation routes the proposal HUMAN
	// (UB-7); the field makes the routing explicit in the recorded outcome.
	RouteHuman bool
	// Reason is the recorded, human-readable basis.
	Reason string
}

// Marker is the recorded size-escalation marker naming the un-read extent — the
// harvestable outcome UB-7 requires ("a size-escalation marker naming the un-read
// extent"). Cited/read like the skip marker.
func (s SizeEscalation) Marker() string {
	return fmt.Sprintf("size-escalation:band=%s,diff=%d,budget=%d,unread=%d",
		s.Band, s.DiffBytes, s.BudgetBytes, s.UnreadBytes)
}

// SizeCheck gates a diff against the band's honest read budget (UB-7). It returns
// a non-nil *SizeEscalation (and ErrOversize) when the diff exceeds the band's
// budget — the caller records the escalation, routes the proposal HUMAN, and does
// NOT emit an approve. It returns (nil, nil) when the diff fits the budget (the
// review proceeds normally).
//
// A band with NO budget entry (an unknown band) is fail-closed: any non-empty
// diff is treated as over budget (a zero budget), so an unrecognized band never
// silently reads an arbitrarily large diff. An empty diff is never over budget.
func SizeCheck(band Band, diff []byte) (*SizeEscalation, error) {
	n := len(diff)
	if n == 0 {
		return nil, nil // an empty diff reads whole trivially.
	}
	budget, ok := BandBudgets[band]
	max := 0
	if ok {
		max = budget.MaxDiffBytes
	}
	if n <= max {
		return nil, nil // fits the honest read budget.
	}
	esc := &SizeEscalation{
		Band:        band,
		DiffBytes:   n,
		BudgetBytes: max,
		UnreadBytes: n - max,
		RouteHuman:  true,
		Reason: fmt.Sprintf("the diff (%d bytes) exceeds the %s band's honest read budget (%d bytes); "+
			"routing HUMAN, attesting %d un-read bytes — never an approve over a partial read (UB-7)",
			n, band, max, n-max),
	}
	return esc, fmt.Errorf("%w: %s", ErrOversize, esc.Marker())
}

// GuardApprove is the UB-7 belt-and-braces: a size-escalated review must NEVER
// carry an approve verdict. Given a proposed verdict and whether a size-escalation
// was recorded, it returns the verdict UNCHANGED when no escalation occurred, and
// FORCES the verdict away from an approve when one did — an escalated review's
// verdict is request-changes at most (an approve or approve-with-advisories over
// an un-read diff is exactly the UB-7 forbidden case). It returns the safe
// verdict and whether it overrode the proposal.
//
// This makes "never emits an approve over a partially-read diff" a structural
// guarantee at the verdict boundary, not merely a convention: even a caller that
// mistakenly proposed an approve after an escalation is corrected here.
func GuardApprove(proposed string, escalated bool) (verdict string, overridden bool) {
	if !escalated {
		return proposed, false
	}
	switch proposed {
	case report.VerdictApprove, report.VerdictApproveWithAdvisories:
		// An approve over a size-escalated (partially-read) diff is forbidden —
		// force it to request-changes (the proposal routes HUMAN regardless).
		return report.VerdictRequestChanges, true
	default:
		// request-changes / escalate are already non-approving — keep them.
		return proposed, false
	}
}
