package verdict

import (
	"fmt"

	"github.com/agenticlab-ai/pyxis/internal/implreview/report"
)

// DowngradeSignal is the WELL-FORMED, SELF-CONTAINED signal a review emits for a
// merge lane to read (NR-4/emit, design §3.6). It is the honest realization of
// "the persisted verdict is the source; a future lane Downgrader adapter reads
// it": everything a downgrade-only lane stage needs to route a proposal — does
// this verdict downgrade, to which lane, and why — is on this record, derived
// from the persisted report alone.
//
// The design is deliberate about the boundary (design §3.6, plan T7 out-of-scope
// note): this package EMITS the signal; CONSUMING it into the lane's
// classifier.Downgrader seam is the owed merge-lane amendment (design Q4), NOT
// built here. So this package imports NO internal/lane code. The signal is a
// plain data record whose method [DowngradeSignal.DowngradesToHuman] has the
// SAME shape the lane's Downgrader.Downgrade returns ((downgrade bool, reason
// string) — plus the ready error path a future adapter surfaces), so a
// one-line lane adapter can wrap it later. A test (signal_test.go) demonstrates
// that compatibility with a LOCAL adapter type, proving the shape without this
// production package depending on the lane.
//
// Staleness is folded in at construction (UB-6/DINV-7): [SignalFor] takes the
// current head and, if the verdict is stale, emits an ABSENT signal (Present ==
// false) that downgrades nothing — a stale verdict is treated as absent, so it
// can never route a lane off a tree it no longer certifies.
type DowngradeSignal struct {
	// ProposalRef identifies the proposal this signal is about (the report's
	// proposal_ref) — the key a lane adapter matches its diff against.
	ProposalRef string
	// Present is whether a current (non-stale) verdict exists. A stale verdict,
	// a report with no rounds, or an unset envelope verdict all emit an ABSENT
	// signal (Present == false) that downgrades nothing (UB-6/DINV-7).
	Present bool
	// Verdict is the live report verdict string this signal reflects, when
	// Present. Empty when absent.
	Verdict string
	// Authority is the verdict's downgrade-only effect (force-human /
	// return-to-worker / none). It is the closed Authority enum, which has no
	// AUTO-granting member — so this signal can express "downgrade to HUMAN",
	// "return to worker", or "no downgrade", and NOTHING that widens lane
	// eligibility (FR-7). Absent when Present is false.
	Authority Authority
	// ReviewedRevision is the tree the verdict certifies (the latest round's
	// reviewed_revision), for a reader that records which tree it routed off.
	ReviewedRevision string
	// Reason is a human-readable statement of why the signal downgrades (or does
	// not) — the string a lane adapter forwards as its downgrade reason.
	Reason string
}

// SignalFor builds the emitted downgrade signal from a persisted report at the
// proposal's current head (UB-6/DINV-7 folded in). If the report's verdict is
// stale (head ≠ latest reviewed revision) or otherwise absent, the signal is
// ABSENT and downgrades nothing. Otherwise it reflects the current verdict and
// its downgrade authority.
//
// It is a pure function of (report, head): the SAME persisted report at the
// SAME head always emits the SAME signal, so a lane consuming it is
// deterministic (mirroring the classifier's own byte-reproducibility, DINV-4).
// A nil report emits an absent signal (there is nothing to route).
func SignalFor(r *report.Report, head string) DowngradeSignal {
	if r == nil {
		return DowngradeSignal{Present: false, Reason: "no review payload — absent signal, no downgrade"}
	}

	live, present := Current(r, head)
	if !present {
		// Stale, no rounds, or an unset envelope verdict: absent, downgrades
		// nothing (UB-6/DINV-7). Name WHY so a reader can log the absence.
		reason := "no current verdict — absent signal, no downgrade"
		if IsStale(r, head) {
			if rev, ok := LatestReviewedRevision(r); ok {
				reason = fmt.Sprintf("verdict is stale (head %q ≠ latest reviewed revision %q) — treated as absent, no downgrade", head, rev)
			} else {
				reason = "verdict is stale (no reviewed round) — treated as absent, no downgrade"
			}
		}
		return DowngradeSignal{
			ProposalRef: r.ProposalRef,
			Present:     false,
			Reason:      reason,
		}
	}

	auth := authorityOf(live)
	rev, _ := LatestReviewedRevision(r)
	return DowngradeSignal{
		ProposalRef:      r.ProposalRef,
		Present:          true,
		Verdict:          live,
		Authority:        auth,
		ReviewedRevision: rev,
		Reason:           signalReason(live, auth),
	}
}

// DowngradesToHuman reports whether this signal downgrades the proposal to the
// HUMAN lane, with the reason naming why. This is the ONE question a merge lane's
// downgrade-only stage asks — and its return shape ((downgrade bool, reason
// string)) is exactly the value the lane's classifier.Downgrader.Downgrade
// returns, so a future lane adapter (design Q4) wraps this in one line WITHOUT
// this package importing the lane.
//
// A signal downgrades to HUMAN iff it is present AND its authority forces HUMAN
// (an escalate). A request-changes returns the proposal to the WORKER — a
// downgrade, but NOT a lane-HUMAN routing (the worker fixes and re-review runs);
// so for the LANE's HUMAN-routing question it reports false, with the reason
// naming the return-to-worker downgrade. An absent (stale/missing) signal
// downgrades nothing. In NO case can this report a value that grants AUTO — the
// method returns only "downgrade to HUMAN: true/false", never a grant.
func (s DowngradeSignal) DowngradesToHuman() (downgrade bool, reason string) {
	if !s.Present {
		return false, s.Reason
	}
	if s.Authority == AuthorityForceHuman {
		return true, s.Reason
	}
	return false, s.Reason
}

// Downgrades reports whether this signal expresses ANY downgrade (force-HUMAN or
// return-to-worker) versus withholding one — the broader predicate over the
// signal's authority, distinct from the lane-specific [DowngradesToHuman]. A
// present request-changes Downgrades() == true (it downgrades to the worker)
// but DowngradesToHuman() == false (it does not route the lane to HUMAN).
func (s DowngradeSignal) Downgrades() bool {
	return s.Present && s.Authority.Downgrades()
}

// signalReason states why a present signal downgrades (or does not), per its
// verdict — the human-readable string a lane adapter forwards.
func signalReason(reportVerdict string, auth Authority) string {
	switch auth {
	case AuthorityForceHuman:
		return "impl-review escalated — force HUMAN lane (" + reportVerdict + ")"
	case AuthorityReturnToWorker:
		return "impl-review requests changes — return to worker (" + reportVerdict + "); the lane is not routed HUMAN by this verdict"
	default:
		return "impl-review " + reportVerdict + " — no downgrade (the lane's own floor+classes decide eligibility; this verdict never grants AUTO)"
	}
}
