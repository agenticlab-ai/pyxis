package report

import (
	"errors"
	"fmt"
	"strings"
)

// Sentinel errors — part of the package contract, one per integrity rule, so a
// caller/test can tell WHICH rule failed with errors.Is. Every Validate return
// wraps exactly one of these with the offending finding/round named (DINV-1/2/3/7).
var (
	// ErrSchemaVersion — the payload's schema_version is not SchemaVersion.
	ErrSchemaVersion = errors.New("impl-review-report: wrong schema version")
	// ErrNoRounds — a report with no rounds; a round is the append unit and at
	// least one is required.
	ErrNoRounds = errors.New("impl-review-report: no rounds")
	// ErrMissingReviewedRevision — a round without its reviewed_revision (DINV-7):
	// a round must record the head sha it evaluated.
	ErrMissingReviewedRevision = errors.New("impl-review-report: round missing reviewed_revision")
	// ErrMissingCalRow — a finding without a calibration-row citation (DINV-3):
	// every finding must cite the row it takes its severity from.
	ErrMissingCalRow = errors.New("impl-review-report: finding missing cal_row")
	// ErrUnknownCalRow — a finding cites a cal_row the calibration table does not
	// know (DINV-3): the citation must resolve.
	ErrUnknownCalRow = errors.New("impl-review-report: finding cites unknown cal_row")
	// ErrSeverityMismatch — a finding's severity contradicts the severity its
	// cited cal_row maps to (DINV-3).
	ErrSeverityMismatch = errors.New("impl-review-report: finding severity contradicts its cited cal_row")
	// ErrDeclineNoReason — a declined disposition with no reason; a decline must
	// carry a reason.
	ErrDeclineNoReason = errors.New("impl-review-report: declined disposition missing reason")
	// ErrDuplicateReport — two reports share the same (proposal_ref, review_id):
	// one payload per proposal per review (DINV-1).
	ErrDuplicateReport = errors.New("impl-review-report: duplicate (proposal_ref, review_id)")
	// ErrRenderFidelity — a rendered region diverges from Render(r) (DINV-2): the
	// human face is a pure rendering of the payload, which governs.
	ErrRenderFidelity = errors.New("impl-review-report: rendered region diverges from the payload")
	// ErrEscalationBrief — the escalation brief presence contradicts the verdict:
	// a brief is present iff the verdict is escalate.
	ErrEscalationBrief = errors.New("impl-review-report: escalation brief presence contradicts the verdict")
	// ErrRoundMutated — an append that rewrote a prior round's reviewed_revision
	// (plan T1 acceptance #4): the round record is append-only.
	ErrRoundMutated = errors.New("impl-review-report: prior round's reviewed_revision mutated")
)

// CalLookup resolves a calibration-row citation to the severity that row maps
// to. Validate takes it as a parameter so this package does NOT depend on the
// T5 calibration table — a test passes a stub, and the real calibration data
// implements it. Severity returns (severity, true) for a known row, and
// (_, false) for a row the table does not carry.
type CalLookup interface {
	Severity(calRow string) (string, bool)
}

// CalLookupFunc adapts a plain function to CalLookup — the ergonomic form a
// caller or test uses when a whole type would be ceremony.
type CalLookupFunc func(calRow string) (string, bool)

// Severity implements CalLookup.
func (f CalLookupFunc) Severity(calRow string) (string, bool) { return f(calRow) }

// Validate enforces the payload-internal integrity rules — DINV-1's per-payload
// half (schema version), DINV-3 (every finding carries a resolvable cal_row
// whose mapped severity equals the finding's), DINV-7 (every round records its
// reviewed_revision), the decline-reason rule, the escalation-brief presence
// rule, and the at-least-one-round rule (a round is the append unit). It returns
// a NAMED sentinel per rule (wrapped with the offending element) so a caller can
// branch with errors.Is.
//
// The cross-report DINV-1 uniqueness rule (two payloads for one proposal+review)
// is ValidateUnique, since it is a property of a SET of reports, not one. The
// DINV-2 render-fidelity rule is ValidateRenderFidelity, since it needs the
// rendered face to compare against.
//
// cal resolves each finding's cal_row to its mapped severity (DINV-3). It must
// be non-nil: a nil CalLookup cannot certify DINV-3, so Validate fails closed
// rather than skip the rule.
func Validate(r *Report, cal CalLookup) error {
	if r == nil {
		return fmt.Errorf("validate: nil report")
	}
	if cal == nil {
		return fmt.Errorf("validate: nil CalLookup — DINV-3 cannot be certified without the calibration table")
	}

	if r.SchemaVersion != SchemaVersion {
		return fmt.Errorf("%w: %q (want %q)", ErrSchemaVersion, r.SchemaVersion, SchemaVersion)
	}

	// The escalation brief is present iff the verdict is escalate.
	hasBrief := r.EscalationBrief != nil
	wantBrief := r.Verdict == VerdictEscalate
	if hasBrief != wantBrief {
		return fmt.Errorf("%w: verdict=%q brief-present=%t", ErrEscalationBrief, r.Verdict, hasBrief)
	}

	// A round is the append unit: at least one is required.
	if len(r.Rounds) == 0 {
		return fmt.Errorf("%w", ErrNoRounds)
	}

	for ri := range r.Rounds {
		round := &r.Rounds[ri]

		// DINV-7: every round records the head sha it evaluated.
		if strings.TrimSpace(round.ReviewedRevision) == "" {
			return fmt.Errorf("%w: round %d", ErrMissingReviewedRevision, ri)
		}

		for fi := range round.Findings {
			f := &round.Findings[fi]

			// DINV-3: every finding carries a cal_row citation.
			if strings.TrimSpace(f.CalRow) == "" {
				return fmt.Errorf("%w: round %d finding %q", ErrMissingCalRow, ri, f.ID)
			}
			// DINV-3: the citation must resolve, and the finding's severity must
			// equal the severity the cited row maps to.
			mapped, ok := cal.Severity(f.CalRow)
			if !ok {
				return fmt.Errorf("%w: round %d finding %q cites %q", ErrUnknownCalRow, ri, f.ID, f.CalRow)
			}
			if f.Severity != mapped {
				return fmt.Errorf("%w: round %d finding %q is %q but cal_row %q maps to %q",
					ErrSeverityMismatch, ri, f.ID, f.Severity, f.CalRow, mapped)
			}

			// A declined disposition must carry a reason.
			if d := f.Disposition; d != nil && d.Verdict == VerdictDeclined && strings.TrimSpace(d.Reason) == "" {
				return fmt.Errorf("%w: round %d finding %q", ErrDeclineNoReason, ri, f.ID)
			}
		}
	}

	return nil
}

// ValidateUnique enforces DINV-1's cross-report half: no two reports share the
// same (proposal_ref, review_id). One payload per proposal per review is the
// uniqueness the whole design rests on (rounds append INSIDE a payload; a second
// payload for the same key is invalid). It returns ErrDuplicateReport naming the
// colliding key.
func ValidateUnique(rs []*Report) error {
	type key struct{ proposal, review string }
	seen := make(map[key]bool, len(rs))
	for _, r := range rs {
		if r == nil {
			continue
		}
		k := key{r.ProposalRef, r.ReviewID}
		if seen[k] {
			return fmt.Errorf("%w: proposal_ref=%q review_id=%q", ErrDuplicateReport, r.ProposalRef, r.ReviewID)
		}
		seen[k] = true
	}
	return nil
}

// ValidateAppend enforces the append-only round contract (plan T1 acceptance
// #4): a next state may only ADD rounds after prior's rounds — it may never
// rewrite a prior round's recorded reviewed_revision, and never drop a prior
// round. prior is the persisted state; next is the write being validated. A
// mutation of any prior round's reviewed_revision (or a shortened round list) is
// ErrRoundMutated. The two must be the same proposal+review — a caller pairs
// them by key before appending.
func ValidateAppend(prior, next *Report) error {
	if prior == nil || next == nil {
		return fmt.Errorf("validate append: nil report")
	}
	if len(next.Rounds) < len(prior.Rounds) {
		return fmt.Errorf("%w: next has %d rounds, prior had %d (a round was dropped)",
			ErrRoundMutated, len(next.Rounds), len(prior.Rounds))
	}
	for i := range prior.Rounds {
		if prior.Rounds[i].ReviewedRevision != next.Rounds[i].ReviewedRevision {
			return fmt.Errorf("%w: round %d was %q, now %q",
				ErrRoundMutated, i, prior.Rounds[i].ReviewedRevision, next.Rounds[i].ReviewedRevision)
		}
	}
	return nil
}

// Render is the PURE, deterministic human rendering of the payload — the same
// input always yields the same output, with no clock/random/map-iteration
// nondeterminism (DINV-2's basis; the disposition rendering sorts its keys, and
// rounds/findings render in their payload order). It is a compact report text: a
// header line, the envelope, then each round with its revision, provenance,
// counts, and findings; findings render their anchor, lens/severity, cal_row,
// message, location, acceptance, and disposition. The human face is generated
// from this function and NEVER hand-authored (DINV-2); a divergence is caught by
// ValidateRenderFidelity.
//
// Render is intentionally exhaustive over the payload so the rendered face is a
// faithful projection: a reviewer reads this text, not the raw JSON, and the
// harvest still reads the payload (never this prose — DINV-8).
func Render(r *Report) string {
	if r == nil {
		return ""
	}
	var b strings.Builder

	fmt.Fprintf(&b, "impl-review report (%s)\n", r.SchemaVersion)
	fmt.Fprintf(&b, "proposal: %s\n", r.ProposalRef)
	fmt.Fprintf(&b, "review: %s\n", r.ReviewID)
	fmt.Fprintf(&b, "band: %s\n", r.Band)
	fmt.Fprintf(&b, "verdict: %s\n", r.Verdict)

	if r.EscalationBrief != nil {
		eb := r.EscalationBrief
		b.WriteString("escalation:\n")
		fmt.Fprintf(&b, "  conflict: %s\n", eb.Conflict)
		for _, opt := range eb.Options {
			fmt.Fprintf(&b, "  option: %s\n", opt)
		}
		fmt.Fprintf(&b, "  decision-needed: %s\n", eb.DecisionNeeded)
	}

	for ri := range r.Rounds {
		round := &r.Rounds[ri]
		fmt.Fprintf(&b, "round %d:\n", ri+1)
		fmt.Fprintf(&b, "  reviewed-revision: %s\n", round.ReviewedRevision)
		fmt.Fprintf(&b, "  context: %s\n", round.ContextID)
		fmt.Fprintf(&b, "  provenance: model=%s prompts=%s cal=%s\n",
			round.Provenance.Model, round.Provenance.PromptsVersion, round.Provenance.CalVersion)
		fmt.Fprintf(&b, "  counts: findings=%d candidates=%d killed=%d\n",
			round.FindingsCount, round.Candidates, round.Killed)
		for fi := range round.Findings {
			renderFinding(&b, &round.Findings[fi])
		}
	}

	return b.String()
}

// renderFinding writes one finding's compact rendering. Location and disposition
// are rendered deterministically; the disposition line is omitted when absent.
func renderFinding(b *strings.Builder, f *Finding) {
	fmt.Fprintf(b, "  #%s [%s/%s] %s\n", f.ID, f.Lens, f.Severity, f.CalRow)
	fmt.Fprintf(b, "    message: %s\n", f.Message)
	if f.Path != "" {
		if f.Line > 0 {
			fmt.Fprintf(b, "    at: %s:%d\n", f.Path, f.Line)
		} else {
			fmt.Fprintf(b, "    at: %s\n", f.Path)
		}
	}
	if f.LogRef != "" {
		fmt.Fprintf(b, "    log: %s\n", f.LogRef)
	}
	fmt.Fprintf(b, "    acceptance: %s\n", f.Acceptance)
	if d := f.Disposition; d != nil {
		fmt.Fprintf(b, "    disposition: %s %s\n", d.Actor, d.Verdict)
		if d.Reason != "" {
			fmt.Fprintf(b, "      reason: %s\n", d.Reason)
		}
		if d.Attribution != "" {
			fmt.Fprintf(b, "      attribution: %s\n", d.Attribution)
		}
	}
}

// ValidateRenderFidelity enforces DINV-2: the persisted rendered region MUST
// equal Render(r) exactly. A hand-authored region that diverged from the payload
// is invalid — the payload governs, the face is its pure rendering. It returns
// ErrRenderFidelity on any divergence.
func ValidateRenderFidelity(r *Report, rendered string) error {
	want := Render(r)
	if rendered != want {
		return fmt.Errorf("%w", ErrRenderFidelity)
	}
	return nil
}
