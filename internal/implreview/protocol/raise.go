package protocol

import (
	"fmt"

	"github.com/agenticlab-ai/pyxis/internal/implreview/report"
)

// CheckRaise enforces disposition-before-raise (FR-15): before a proposal is
// raised, EVERY blocking finding must carry a recorded disposition. A raise
// carrying a blocking finding with NO disposition is a protocol defect
// (ErrUndisposedBlocking), refused.
//
// findings is the finding set being raised (the latest round's findings). It
// checks each BLOCKING finding: an advisory finding needs no disposition (it
// holds nothing and never blocks a raise), but a blocking finding with a nil
// Disposition is the defect. The FIRST undisposed blocking finding is reported
// (named), so the caller/test knows which finding is the protocol defect; a
// caller that wants them all can iterate UndisposedBlocking.
//
// It returns nil when every blocking finding is disposed — a well-formed raise.
// It does NOT judge the disposition's outcome (applied vs declined) — that is
// the verdict's job (verdict.Compute) and the applied-verified transition
// (ApplyVerification below); CheckRaise only enforces PRESENCE.
func CheckRaise(findings []report.Finding) error {
	for i := range findings {
		f := &findings[i]
		if f.Severity != report.SeverityBlocking {
			continue
		}
		if f.Disposition == nil {
			return fmt.Errorf("%w: finding %q (rule %q)", ErrUndisposedBlocking, f.ID, f.RuleID)
		}
	}
	return nil
}

// UndisposedBlocking returns every blocking finding with no recorded disposition
// — the full set of disposition-before-raise defects (FR-15), for a caller that
// wants all of them rather than the first (CheckRaise reports the first). An
// empty return means the raise satisfies disposition-before-raise.
func UndisposedBlocking(findings []report.Finding) []report.Finding {
	var out []report.Finding
	for i := range findings {
		f := &findings[i]
		if f.Severity == report.SeverityBlocking && f.Disposition == nil {
			out = append(out, *f)
		}
	}
	return out
}

// VerificationResult is the outcome of the round-2 applied-verified transition
// for one applied blocking finding (FR-15). An APPLIED blocking finding is an
// UNVERIFIED claim until a round-2 re-review confirms the named defect closed:
//
//   - VERIFIED (the round-2 re-review did NOT re-raise the named defect — the
//     defect is closed): the finding is resolved; it no longer holds the
//     verdict. In the round-2 finding set it is CLOSED (dropped) so
//     verdict.Compute sees no blocker for it.
//   - UNVERIFIED (the round-2 re-review DID re-raise the named defect, OR there
//     was no round-2 re-review to confirm it): the applied claim is an
//     UNVERIFIED claim that HOLDS the verdict at request-changes. In the round-2
//     finding set it stays a BLOCKING finding, so verdict.Compute returns
//     request-changes (and verdict.resolvedAway already refuses to let a worker
//     `applied` resolve a blocker — this package never contradicts it).
type VerificationResult struct {
	// FindingID is the applied blocking finding this result is about.
	FindingID string
	// Verified is true iff the round-2 re-review confirmed the named defect
	// closed (it was NOT re-raised in round 2). false = an unverified claim that
	// holds the verdict.
	Verified bool
}

// ApplyVerification computes the round-2 applied-verified transition (FR-15) and
// returns BOTH the per-finding results AND the round-2 finding set to record —
// the set verdict.Compute then reads. It is the mechanism that makes an
// APPLIED-but-UNVERIFIED blocking finding HOLD the verdict at request-changes,
// while a VERIFIED one stops holding it.
//
// round1Blocking is the set of round-1 BLOCKING findings, each carrying its
// worker disposition (an applied claim carries a worker/applied disposition; a
// declined one carries a worker/declined disposition — declined blockers
// escalate via RecordDisposition and are NOT resolved here). round2Findings is
// the scope-locked round-2 finding set (the survivors of ApplyScopeLock) — the
// defects the round-2 re-review actually re-raised, by rule_id / FND id.
//
// The transition, per round-1 blocking finding whose disposition is
// worker/applied:
//
//   - if round 2 did NOT re-raise the named defect (no round-2 finding matches
//     its rule_id or id) → VERIFIED: the defect is closed; the finding is
//     dropped from the recorded round-2 set (it no longer holds the verdict).
//   - if round 2 DID re-raise it → UNVERIFIED: the applied claim did not close
//     the defect; the RE-RAISED round-2 finding (blocking) stays in the set and
//     HOLDS request-changes.
//
// A round-1 blocking finding that is NOT an applied claim (e.g. a declined one)
// is left to its own path (declined → escalate; no disposition → CheckRaise
// refuses the raise) and is not part of the applied-verified transition, so it
// is not added by this function. Round-2 findings that are FRESH (do not match
// any round-1 named defect) are carried through unchanged — a legitimately
// in-scope new finding on a changed span still ships.
//
// The returned Round2Findings is what the caller records as round 2's finding
// set; verdict.Compute over it yields request-changes iff any unverified
// (re-raised) applied blocker — or any other live blocker — remains.
func ApplyVerification(round1Blocking, round2Findings []report.Finding) (results []VerificationResult, round2Set []report.Finding) {
	// Index the round-2 findings by the identities a re-raise matches on
	// (rule_id and FND id), so we can tell whether a named defect was re-raised.
	reRaisedByRule := make(map[string]*report.Finding)
	reRaisedByID := make(map[string]*report.Finding)
	for i := range round2Findings {
		f := &round2Findings[i]
		if f.RuleID != "" {
			reRaisedByRule[f.RuleID] = f
		}
		if f.ID != "" {
			reRaisedByID[f.ID] = f
		}
	}

	// Track which round-2 findings are "consumed" as the re-raise of a named
	// applied defect, so they are added exactly once and the remaining round-2
	// findings (fresh, in-scope) are carried through afterward.
	consumed := make(map[string]bool)

	for i := range round1Blocking {
		f := &round1Blocking[i]
		d := f.Disposition
		// Only APPLIED worker claims take the verified/unverified transition. A
		// declined blocker escalates (RecordDisposition); an undisposed blocker is
		// refused at raise (CheckRaise); an operator-resolved one is handled by
		// verdict.resolvedAway. None of those are verified here.
		if d == nil || d.Actor != report.ActorWorker || d.Verdict != report.VerdictApplied {
			continue
		}

		reRaised := matchReRaise(f, reRaisedByRule, reRaisedByID)
		if reRaised == nil {
			// VERIFIED: round 2 did not re-raise the named defect — it is closed.
			// The finding is dropped from the recorded round-2 set (stops holding).
			results = append(results, VerificationResult{FindingID: f.ID, Verified: true})
			continue
		}

		// UNVERIFIED: round 2 re-raised the named defect — the applied claim did
		// not close it. The re-raised round-2 finding (blocking) HOLDS the verdict.
		results = append(results, VerificationResult{FindingID: f.ID, Verified: false})
		round2Set = append(round2Set, *reRaised)
		markConsumed(reRaised, consumed)
	}

	// Carry through every round-2 finding not already consumed as a re-raise —
	// the fresh, in-scope round-2 findings (a legitimately new finding on a
	// changed span). They ship as-is.
	for i := range round2Findings {
		f := &round2Findings[i]
		if isConsumed(f, consumed) {
			continue
		}
		round2Set = append(round2Set, *f)
	}

	return results, round2Set
}

// matchReRaise returns the round-2 finding that re-raises a round-1 named defect
// (by rule_id first, then FND id), or nil when round 2 did not re-raise it.
func matchReRaise(named *report.Finding, byRule, byID map[string]*report.Finding) *report.Finding {
	if named.RuleID != "" {
		if f, ok := byRule[named.RuleID]; ok {
			return f
		}
	}
	if named.ID != "" {
		if f, ok := byID[named.ID]; ok {
			return f
		}
	}
	return nil
}

// markConsumed records that a round-2 finding was added as the re-raise of a
// named applied defect, keyed by its identities, so the carry-through pass does
// not add it a second time.
func markConsumed(f *report.Finding, consumed map[string]bool) {
	if f.ID != "" {
		consumed["id:"+f.ID] = true
	}
	if f.RuleID != "" {
		consumed["rule:"+f.RuleID] = true
	}
}

// isConsumed reports whether a round-2 finding was already added as a re-raise.
func isConsumed(f *report.Finding, consumed map[string]bool) bool {
	if f.ID != "" && consumed["id:"+f.ID] {
		return true
	}
	if f.RuleID != "" && consumed["rule:"+f.RuleID] {
		return true
	}
	return false
}
