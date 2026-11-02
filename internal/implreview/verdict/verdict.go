// Package verdict is the impl-review reviewer's live verdict — the
// downgrade-only value a review carries, the escalation brief an escalate
// produces, the reviewed-revision staleness binding, and the well-formed
// downgrade signal a merge lane could read (docs/tech-design/impl-review-v1.md
// §3.1, §3.6, §3.7 DINV-7; plan T7 — FR-7, FR-17, UB-6, NR-4/emit).
//
// The whole point of this package is a single, load-bearing guarantee:
//
//	The verdict is DOWNGRADE-ONLY. It can force the HUMAN lane or bounce the
//	proposal back to the worker, and it can do NOTHING else — no value, no
//	field, and no method grants the AUTO lane or writes a stage-9 sign-off.
//
// That guarantee is made STRUCTURAL, not documented: a verdict's downgrade
// authority is the closed [Authority] enum, whose only non-empty members are
// [AuthorityReturnToWorker] and [AuthorityForceHuman]. There is no
// `AuthorityGrantAuto` to hold, no signature field to set, and no verb that
// emits either — so "the verdict grants AUTO" is UNREPRESENTABLE by
// construction (FR-7). Forcing HUMAN and returning-to-worker are the only two
// effects the type can express; a test asserts both work and that no
// AUTO-granting value exists (verdict_test.go).
//
// What lives HERE (the testable Go core, T7):
//   - [Compute]: derive the live verdict from a review's rounds — the latest
//     round's findings + dispositions + the caller's escalate signal — mapping
//     blocking-and-unresolved → request-changes (or escalate on an escalate
//     condition), advisory-only → approve-with-advisories, none → approve
//     (FR-7). It returns an [Outcome] carrying the report verdict string, its
//     downgrade [Authority], and the escalation brief present iff escalate.
//   - [SetEnvelope]: stamp the computed verdict + brief onto a report.Report,
//     the envelope T4's runtime deliberately left unset. It leaves a
//     report.Report that satisfies report.Validate's brief-iff-escalate rule.
//   - the staleness predicate (staleness.go): the live verdict is stale iff the
//     proposal head ≠ the LATEST round's reviewed_revision, and a stale verdict
//     is treated as ABSENT — never current (UB-6, DINV-7).
//   - the emitted downgrade signal (signal.go): [DowngradeSignal], a
//     self-contained record a merge lane could read to route HUMAN. This
//     package emits it; CONSUMING it into the lane's Downgrader seam is the
//     owed lane amendment (design §3.6 Q4), out of scope here — so this package
//     imports NO internal/lane code (a test proves shape-compatibility with a
//     LOCAL adapter).
//
// What is NOT here: the merge lane's consumption of the signal (owed amendment,
// design Q4); the two-round dispute protocol and disposition RECORDING (T8 —
// this package READS a round's dispositions to compute, it does not record
// them); resume / the in-flight predicate / review-absence on failure (T9).
//
// Stdlib only, per the repo Go rules.
package verdict

import (
	"fmt"

	"github.com/agenticlab-ai/pyxis/internal/implreview/report"
)

// MostRestrictive applies impl-review's existing aggregate order to a complete
// pass verdict set. The routing-evidence layer calls this owner function only
// to validate a persisted aggregate; aggregate policy remains here.
func MostRestrictive(verdicts []string) (string, error) {
	if len(verdicts) == 0 {
		return "", fmt.Errorf("impl-review verdict: empty pass set")
	}
	rank := map[string]int{
		report.VerdictApprove:               0,
		report.VerdictApproveWithAdvisories: 1,
		report.VerdictRequestChanges:        2,
		report.VerdictEscalate:              3,
	}
	result, highest := "", -1
	for _, candidate := range verdicts {
		value, ok := rank[candidate]
		if !ok {
			return "", fmt.Errorf("impl-review verdict: unknown pass verdict %q", candidate)
		}
		if value > highest {
			result, highest = candidate, value
		}
	}
	return result, nil
}

// Authority is a verdict's DOWNGRADE-ONLY effect on the merge lane — the closed
// set of things a verdict may do. This enum is where FR-7's guarantee lives:
// its only non-empty members force HUMAN or return-to-worker, and there is
// deliberately NO member that grants AUTO or signs stage 9. A verdict cannot
// hold an AUTO-granting authority because the type has none to hold — the
// "downgrade-only, forever" ruling (PRD §0 #2) made unrepresentable rather than
// merely asserted.
//
// The mapping from a report verdict to its authority is fixed (see
// [authorityOf]): approve / approve-with-advisories carry [AuthorityNone] (they
// downgrade nothing — but note an approve is NOT a grant either: it withholds a
// downgrade, and the lane's own floor+classes still decide AUTO eligibility;
// this package never widens it). request-changes carries
// [AuthorityReturnToWorker]; escalate carries [AuthorityForceHuman].
type Authority string

const (
	// AuthorityNone is the absence of a downgrade — the authority an approve or
	// approve-with-advisories carries. It is NOT an AUTO grant: it withholds a
	// downgrade signal, leaving the lane's own floor+class decision untouched.
	// This package never emits a signal that widens lane eligibility (FR-7).
	AuthorityNone Authority = ""
	// AuthorityReturnToWorker bounces the proposal back to the worker — the
	// authority a request-changes carries. It is a downgrade (blocks a raise /
	// holds the verdict), never a grant.
	AuthorityReturnToWorker Authority = "return-to-worker"
	// AuthorityForceHuman routes the proposal to the HUMAN lane — the authority
	// an escalate carries (paired with the escalation brief). It is the
	// strongest downgrade; it can never be inverted into an AUTO grant.
	AuthorityForceHuman Authority = "force-human"
)

// Downgrades reports whether this authority downgrades the proposal (forces
// HUMAN or returns to the worker) versus withholding any downgrade. It is the
// one predicate the emitted signal (signal.go) and any reader use to tell "this
// verdict downgrades" from "this verdict does not" — never "this verdict grants
// AUTO", which no authority expresses.
func (a Authority) Downgrades() bool {
	return a == AuthorityReturnToWorker || a == AuthorityForceHuman
}

// authorityOf maps a report verdict string to its fixed downgrade authority.
// This is the ONLY place the verdict→authority relation is defined, and it is
// total over the four downgrade-only verdicts. An unknown verdict maps to
// AuthorityNone defensively, but Compute never produces one — it emits only the
// four report.Verdict* values.
func authorityOf(reportVerdict string) Authority {
	switch reportVerdict {
	case report.VerdictRequestChanges:
		return AuthorityReturnToWorker
	case report.VerdictEscalate:
		return AuthorityForceHuman
	case report.VerdictApprove, report.VerdictApproveWithAdvisories:
		return AuthorityNone
	default:
		// Unreachable from Compute; fail safe to no-downgrade rather than
		// fabricate an authority for a verdict this package did not emit.
		return AuthorityNone
	}
}

// Escalation is a caller-supplied escalate condition and the decision brief it
// carries (FR-17). An escalate verdict is NOT derivable from finding severity
// alone: it is the specific "the fix is a spec decision, not a code fix" case —
// a declined blocking finding (FR-8), a materially-defective charter (FR-14),
// or a calibration row whose escalation condition fires (e.g. CAL-3, an
// ambiguous/self-contradictory charter). The lens layer (T5), the two-round
// protocol (T8), and invocation validation (T10) each detect their own
// escalate condition and hand it here as a Signals.Escalation; this package
// does not re-derive it, it acts on it — producing the brief on the proposal.
//
// Present (non-nil) in [Signals] iff the caller determined the review escalates;
// its Brief is the FR-17 decision brief (the single conflict, the options with
// their risks, the decision needed, addressed to the stage-9 human).
type Escalation struct {
	// Brief is the FR-17 decision brief carried on an escalate verdict. Its
	// three fields are all required to be non-empty (Compute returns
	// ErrIncompleteBrief otherwise): a brief that names no conflict, offers no
	// option, or states no decision is not an actionable escalation.
	Brief report.EscalationBrief
}

// Signals is the escalate condition input to [Compute] — the ONE thing the
// verdict computation cannot read off the findings themselves. Everything else
// Compute derives from the rounds' findings and dispositions; the escalate
// condition (FR-17) is a caller judgment (T5/T8/T10), so it is passed here.
//
// The zero value (Escalation == nil) means "no escalate condition" — the common
// case, where the verdict is approve / approve-with-advisories / request-changes
// derived purely from the findings. A non-nil Escalation forces escalate and
// carries the brief.
type Signals struct {
	// Escalation, when non-nil, forces an escalate verdict carrying its Brief
	// (FR-17). Nil means no escalate condition fired.
	Escalation *Escalation
}

// Outcome is the computed live verdict of a review: the report verdict string
// ([report.VerdictApprove]…[report.VerdictEscalate]), its downgrade [Authority],
// and the escalation brief present iff the verdict is escalate. It is the
// value [SetEnvelope] stamps onto a report.Report, and the value the emitted
// [DowngradeSignal] is built from.
type Outcome struct {
	// Verdict is one of the four downgrade-only report.Verdict* values. It is
	// NEVER any other string — Compute emits only the four.
	Verdict string
	// Authority is the verdict's downgrade-only effect (force-human /
	// return-to-worker / none). It is derived from Verdict by authorityOf, so it
	// can never disagree with the verdict and can never grant AUTO.
	Authority Authority
	// Brief is the FR-17 escalation brief, present iff Verdict is
	// report.VerdictEscalate (nil otherwise) — mirroring report.Report's
	// brief-iff-escalate invariant (report.Validate enforces it on the payload).
	Brief *report.EscalationBrief
}

// Downgrades reports whether the outcome downgrades the proposal — a convenience
// over Authority.Downgrades() for callers holding an Outcome.
func (o Outcome) Downgrades() bool { return o.Authority.Downgrades() }

// Compute derives a review's live verdict from its rounds and the caller's
// escalate signal (FR-7):
//
//   - If the caller supplied an escalate condition (sig.Escalation != nil), the
//     verdict is escalate, carrying the FR-17 brief (force-human). The brief
//     must be complete (all three fields) or Compute returns ErrIncompleteBrief.
//   - Otherwise the verdict is derived from the LATEST round's findings and
//     their dispositions: a blocking finding that is not resolved-away (see
//     resolvedAway) → request-changes (return-to-worker); no unresolved
//     blocking finding but at least one advisory → approve-with-advisories;
//     no findings that matter → approve.
//
// It reads the LATEST round because that is the current state of the review
// (each round re-evaluates; an earlier round's blocker the worker fixed is
// re-judged in the next round — §3.1). An empty rounds slice is a programmer
// error (a review always has at least one round before a verdict) and returns
// ErrNoRounds.
//
// The result is ALWAYS one of the four downgrade-only verdicts with a matching
// Authority; it can never produce an AUTO grant, because no code path here can
// name one — the Authority enum has no such member.
func Compute(rounds []report.Round, sig Signals) (Outcome, error) {
	if len(rounds) == 0 {
		return Outcome{}, fmt.Errorf("%w", ErrNoRounds)
	}

	// An escalate condition dominates: it is the "spec decision needed" case,
	// stronger than any severity-derived verdict (FR-17). Its brief must be
	// complete to be an actionable escalation.
	if sig.Escalation != nil {
		brief := sig.Escalation.Brief
		if !briefComplete(brief) {
			return Outcome{}, fmt.Errorf("%w", ErrIncompleteBrief)
		}
		b := brief // copy so the Outcome does not alias the caller's Signals
		return Outcome{
			Verdict:   report.VerdictEscalate,
			Authority: AuthorityForceHuman,
			Brief:     &b,
		}, nil
	}

	// No escalate condition: derive from the latest round's findings.
	latest := &rounds[len(rounds)-1]

	hasUnresolvedBlocking := false
	hasAdvisory := false
	for i := range latest.Findings {
		f := &latest.Findings[i]
		switch f.Severity {
		case report.SeverityBlocking:
			if !resolvedAway(f) {
				hasUnresolvedBlocking = true
			}
		case report.SeverityAdvisory:
			hasAdvisory = true
		}
	}

	switch {
	case hasUnresolvedBlocking:
		return Outcome{
			Verdict:   report.VerdictRequestChanges,
			Authority: AuthorityReturnToWorker,
		}, nil
	case hasAdvisory:
		return Outcome{
			Verdict:   report.VerdictApproveWithAdvisories,
			Authority: AuthorityNone,
		}, nil
	default:
		return Outcome{
			Verdict:   report.VerdictApprove,
			Authority: AuthorityNone,
		}, nil
	}
}

// resolvedAway reports whether a blocking finding no longer holds the verdict —
// i.e. an OPERATOR accepted it away (a rejected-by-operator finding the operator
// judged a nit, or an accepted disposition the operator signed off). A worker's
// own disposition NEVER resolves a blocker: a worker `applied` is an unverified
// claim that holds the verdict until a re-review confirms it (T8's applied-but-
// unverified rule), and a worker `declined` is the escalate condition (T8/FR-8),
// surfaced to the caller as an escalate Signal — not a resolution.
//
// Concretely, a blocker is resolved-away only when its disposition is an
// operator verdict that closes it: VerdictAccepted (the operator accepted the
// fix/finding) — a downgrade the OPERATOR made, whose attribution the harvest
// (T11) verifies. VerdictRejected by the operator means the operator judged the
// finding itself unwarranted (a nit), which also stops it holding the verdict.
// Any worker disposition, or no disposition, leaves the blocker holding.
func resolvedAway(f *report.Finding) bool {
	d := f.Disposition
	if d == nil || d.Actor != report.ActorOperator {
		return false
	}
	switch d.Verdict {
	case report.VerdictAccepted, report.VerdictRejected:
		return true
	default:
		return false
	}
}

// briefComplete reports whether an escalation brief is actionable: it names a
// conflict, offers at least one option, and states the decision needed (FR-17).
// A brief missing any of these is not a usable escalation, so Compute refuses
// it rather than emit an escalate verdict with a hollow brief.
func briefComplete(b report.EscalationBrief) bool {
	if b.Conflict == "" || b.DecisionNeeded == "" {
		return false
	}
	if len(b.Options) == 0 {
		return false
	}
	for _, opt := range b.Options {
		if opt == "" {
			return false
		}
	}
	return true
}

// SetEnvelope stamps the computed Outcome onto a report.Report — the envelope
// verdict + escalation brief that T4's runtime deliberately left unset (it
// produces the round; T7 sets the verdict). After SetEnvelope, the report's
// Verdict is the Outcome's verdict and its EscalationBrief is present iff the
// verdict is escalate, so report.Validate's brief-iff-escalate rule holds.
//
// It mutates r in place and returns it for chaining. A nil report is a
// programmer error (there is no envelope to stamp) and returns an error.
func SetEnvelope(r *report.Report, o Outcome) (*report.Report, error) {
	if r == nil {
		return nil, fmt.Errorf("verdict: nil report — no envelope to set")
	}
	r.Verdict = o.Verdict
	r.EscalationBrief = o.Brief
	return r, nil
}
