// Package charter is impl-review's INVOCATION VALIDATION — the gate the reviewer
// passes before any lens runs, ruling on the task charter the invocation carries
// (docs/tech-design/impl-review-v1.md §3.4, the review context pack's charter
// input; plan T10 — FR-14, FR-17).
//
// The whole point of this package is a single, load-bearing guarantee:
//
//	No invocation path degrades to a SILENT PASS. Every invocation resolves to
//	exactly one of three outcomes — PROCEED, REFUSE (a named defect, no review),
//	or ESCALATE (an upstream-defect brief, findings withheld against the broken
//	span) — and there is no fourth, "review anyway against a broken spec," path.
//
// A review reviews the change against the charter that states what the change
// must satisfy. Two things can be wrong with that charter BEFORE a single line
// of the diff is judged, and each has its own outcome (FR-14):
//
//   - The charter is ABSENT. There is nothing to review against, so the
//     invocation is REFUSED with a named defect ([ErrNoCharter]) and produces NO
//     review — no findings, no verdict. Refusing is honest: a review against a
//     nonexistent spec would be theater, and a silent pass would be the exact
//     failure UB-1 forbids at the invocation boundary.
//   - The charter is PRESENT but MATERIALLY DEFECTIVE for review — ambiguous,
//     self-contradictory, or contradicting the governing design. The defect is
//     UPSTREAM (the spec, not the code), so the reviewer does NOT review the code
//     against a broken spec and does NOT silently pass it: it ESCALATES the
//     upstream-defect class (FR-17), producing the decision brief the verdict
//     layer (T7) turns into an escalate verdict, and WITHHOLDS findings against
//     the defective span (reviewing a span its own spec is broken about would
//     manufacture noise, not signal).
//
// The DETECTION of a material defect is a CALLER-SUPPLIED signal — exactly as
// T8's escalate condition and T6's oversize are caller-signalled, and as the
// design frames the escalate cases (verdict.Escalation's doc: "invocation
// validation (T10) detect their own escalate condition and hand it here"). An
// LLM lens or the reviewer's own reading judges "this charter is
// self-contradictory"; THIS package owns the CONTRACT — the three outcomes, the
// named defects, and the finding-withholding — not that judgment. A caller
// reports a material defect via [MaterialDefect] on the [Invocation]; the
// package rules on it deterministically and hermetically.
//
// This package produces the FR-17 escalation brief in the shape the verdict
// layer (T7) consumes it — a [report.EscalationBrief], handed on as a
// [verdict.Escalation] via [Outcome.Signals] so the verdict computation escalates
// without re-deriving the condition. It imports report (the brief type) and
// verdict (the Signals shape it feeds); verdict imports neither, so there is no
// cycle. Stdlib only, per the repo Go rules.
package charter

import (
	"fmt"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/implreview/report"
	"github.com/agenticlab-ai/pyxis/internal/implreview/verdict"
)

// Kind is the tri-state VERDICT of invocation validation — the closed set of
// what the gate can rule (FR-14). Its three members are the only outcomes an
// invocation can reach; there is deliberately NO "review anyway" member, so
// "degrade to a silent pass" is UNREPRESENTABLE by construction (mirroring the
// verdict package's downgrade-only Authority: the forbidden state has no name to
// hold). Every [Validate] result carries exactly one Kind.
type Kind int

const (
	// KindProceed — the charter is present and not reported materially defective;
	// the review PROCEEDS to the lenses (the normal path). This is the ONLY
	// outcome that lets a review run; the other two stop it before any lens.
	KindProceed Kind = iota
	// KindRefuse — the invocation carried no charter; it is REFUSED with a named
	// defect ([ErrNoCharter]) and produces NO review (no findings, no verdict).
	KindRefuse
	// KindEscalate — the charter is present but materially defective; the review
	// ESCALATES the upstream-defect class (FR-17), carrying the decision brief and
	// WITHHOLDING findings against the defective span. It is NOT a refusal (a
	// charter exists) and NOT a pass (the spec is broken).
	KindEscalate
)

// String renders a Kind for logs/messages — never a bare integer in a
// human-facing string.
func (k Kind) String() string {
	switch k {
	case KindProceed:
		return "proceed"
	case KindRefuse:
		return "refuse"
	case KindEscalate:
		return "escalate"
	default:
		return fmt.Sprintf("charter.Kind(%d)", int(k))
	}
}

// DefectClass is the NAMED class of a material charter defect (FR-14) — the
// closed vocabulary of what "materially defective for review" means, so an
// escalation names WHICH kind of upstream defect it escalates rather than an
// opaque "the charter is bad". A caller reporting a material defect
// ([MaterialDefect]) picks the class; the escalation brief and the withheld-span
// record carry it.
type DefectClass string

const (
	// DefectAmbiguous — the charter is ambiguous on the reviewed behavior: it does
	// not determine what the change must satisfy on the load-bearing path, so a
	// finding "the code does X" cannot be judged right or wrong against it.
	DefectAmbiguous DefectClass = "ambiguous"
	// DefectSelfContradictory — the charter contradicts ITSELF (it requires both A
	// and not-A on the same span), so no implementation can satisfy it and the
	// span cannot be reviewed against it.
	DefectSelfContradictory DefectClass = "self-contradictory"
	// DefectContradictsDesign — the charter contradicts the GOVERNING DESIGN (its
	// own upstream): the spec the reviewer would review against is itself off-spec,
	// so the defect is upstream of the code entirely.
	DefectContradictsDesign DefectClass = "contradicts-governing-design"
)

// Valid reports whether a DefectClass is one of the three named classes — a
// caller-supplied class outside the set is a malformed report, refused by
// [MaterialDefect.validate] rather than escalated under an unnamed class.
func (c DefectClass) Valid() bool {
	switch c {
	case DefectAmbiguous, DefectSelfContradictory, DefectContradictsDesign:
		return true
	default:
		return false
	}
}

// Span identifies the DEFECTIVE region of the charter — the span the escalation
// is about and the span findings are WITHHELD against (FR-14). It is an opaque
// reference the caller supplies (a charter section anchor, a requirement id, a
// line range); this package carries it, records it as the withheld span, and
// threads it into the brief, without interpreting its form.
//
// The zero value ("" reference) means "the whole charter" — a defect the caller
// could not localize withholds findings against the ENTIRE charter's subject,
// the fail-closed default (better to withhold broadly than review part of a
// charter whose defect the caller could not bound).
type Span struct {
	// Ref is the opaque span reference (a section anchor / requirement id / range).
	// Empty means the whole charter (the fail-closed, un-localized default).
	Ref string
}

// Whole reports whether this span is the un-localized "whole charter" span
// (an empty Ref) — the fail-closed default a caller that could not bound the
// defect produces.
func (s Span) Whole() bool { return s.Ref == "" }

// String renders a span for messages, naming the whole-charter case explicitly.
func (s Span) String() string {
	if s.Whole() {
		return "the whole charter"
	}
	return s.Ref
}

// MaterialDefect is a CALLER-SUPPLIED report that the charter is materially
// defective for review (FR-14) — the one judgment this package does NOT make
// itself (it owns the contract, not the "is this charter self-contradictory?"
// determination; that is an LLM/reader judgment, handed here like T8's escalate
// and T6's oversize). Its presence on an [Invocation] is what turns a present
// charter from PROCEED into ESCALATE.
//
// It carries the defect CLASS (one of the three named classes), the SPAN it is
// against (which findings are withheld from), and the decision BRIEF the
// escalation forces (FR-17) — the single conflict, the options with their risks,
// and the decision needed, addressed to the human who resolves the upstream
// defect. The brief must be complete (verdict.Compute's ErrIncompleteBrief rule
// applies) because an escalation the human cannot act on is not an escalation.
type MaterialDefect struct {
	// Class is the named defect class (ambiguous / self-contradictory /
	// contradicts-governing-design). Required and must be Valid().
	Class DefectClass
	// Span is the defective region findings are withheld against; the zero value
	// withholds against the whole charter (the fail-closed default).
	Span Span
	// Brief is the FR-17 decision brief the escalation carries — the single
	// conflict, the options with their risks, the decision needed. It must be
	// complete (all three fields), the same bar verdict.Compute enforces, or
	// [Validate] refuses it (ErrIncompleteBrief) rather than escalate a hollow
	// brief.
	Brief report.EscalationBrief
}

// Invocation is the input to [Validate] — the charter the invocation carries and
// the optional caller-supplied material-defect report. It is the SUBJECT of
// invocation validation: everything the gate rules on is here, so Validate is a
// pure function of it (hermetic, deterministic).
//
// The DISTINCTION the two fields draw is the whole contract:
//   - Charter EMPTY ⇒ REFUSE (no charter — [ErrNoCharter]), regardless of Defect.
//     A charter-less invocation cannot ALSO be "materially defective" (there is
//     no material to be defective); absence dominates.
//   - Charter NON-EMPTY, Defect nil ⇒ PROCEED (a present, not-reported-defective
//     charter).
//   - Charter NON-EMPTY, Defect non-nil ⇒ ESCALATE (a present-but-broken charter).
type Invocation struct {
	// Charter is the task charter text the invocation carries. Empty (after
	// trimming — a whitespace-only charter is no charter) ⇒ the charter-presence
	// defect (refuse). Non-empty ⇒ a charter exists; whether it is USABLE is the
	// Defect field's business.
	Charter string
	// Defect, when non-nil, is the caller's report that the present charter is
	// materially defective for review (escalate). Nil ⇒ the charter is not
	// reported defective (proceed). It is IGNORED when Charter is empty — absence
	// dominates (you cannot report a defect in a charter that is not there).
	Defect *MaterialDefect
}

// Outcome is the RESULT of invocation validation — the tri-state ruling and its
// payload (FR-14). Exactly one [Kind] is set; the other fields are populated per
// the kind:
//
//   - KindProceed: WithheldSpan is nil, Signals is nil — the review proceeds
//     with no restriction.
//   - KindRefuse: WithheldSpan is nil (no review at all — nothing to withhold
//     from), Signals is nil (no escalate). The refusal reason is the error
//     [Validate] returns ([ErrNoCharter]).
//   - KindEscalate: WithheldSpan is the defective span findings are withheld
//     against (FR-14), and Signals carries the FR-17 escalate the verdict layer
//     consumes.
//
// The type makes "a review that both proceeds AND withholds" impossible: a
// non-nil WithheldSpan only ever accompanies KindEscalate, so the withholding is
// bound to the escalate outcome, never a silent partial review under proceed.
type Outcome struct {
	// Kind is the tri-state ruling — exactly one of proceed/refuse/escalate.
	Kind Kind
	// Class is the named defect class on an escalate (ErrDefectiveCharter's
	// class), and "" otherwise — so an escalate names WHICH upstream defect it is.
	Class DefectClass
	// WithheldSpan is the span findings are WITHHELD against on an escalate
	// (FR-14), and nil on proceed/refuse. A caller MUST NOT emit a finding against
	// this span — [Outcome.Withholds] is the predicate a finding-emitter checks.
	WithheldSpan *Span
	// Signals is the verdict-layer escalate input (FR-17) on an escalate, and nil
	// otherwise. It is exactly the verdict.Signals a caller passes to
	// verdict.Compute so the review escalates carrying the brief — the T7 handoff.
	Signals *verdict.Signals
}

// Proceeds reports whether the outcome lets the review run (KindProceed) — the
// one outcome that reaches the lenses.
func (o Outcome) Proceeds() bool { return o.Kind == KindProceed }

// Refused reports whether the outcome REFUSED the invocation (KindRefuse) — no
// review produced.
func (o Outcome) Refused() bool { return o.Kind == KindRefuse }

// Escalated reports whether the outcome ESCALATED an upstream charter defect
// (KindEscalate) — findings withheld against the defective span, a brief carried.
func (o Outcome) Escalated() bool { return o.Kind == KindEscalate }

// ProducesReview reports whether the outcome yields ANY review artifact
// (findings and/or a verdict). It is FALSE for a refusal (KindRefuse produces no
// review — the FR-14 "produces no review" guarantee made a queryable predicate)
// and TRUE otherwise (a proceed produces the normal review; an escalate produces
// the escalate verdict + its withheld-span record). It is the predicate a caller
// uses to decide whether to run the reviewer at all after validation.
func (o Outcome) ProducesReview() bool { return o.Kind != KindRefuse }

// Withholds reports whether a finding against `span` must be WITHHELD under this
// outcome (FR-14). It is true only on an escalate whose withheld span COVERS the
// candidate finding's span:
//
//   - a whole-charter withheld span (an un-localized defect) withholds EVERY
//     finding (the fail-closed broad withholding), and
//   - a localized withheld span withholds a finding whose span matches it.
//
// A proceed or refuse withholds nothing (a refuse produces no findings at all; a
// proceed reviews normally). This is the finding-emitter's gate: a lens/screen
// consults it before shipping a candidate, so a finding against a span the
// charter is broken about is never emitted — the "withholds findings against the
// defective span rather than reviewing against a broken spec" rule (FR-14) made
// a checkable predicate rather than a comment.
func (o Outcome) Withholds(span Span) bool {
	if o.Kind != KindEscalate || o.WithheldSpan == nil {
		return false
	}
	if o.WithheldSpan.Whole() {
		// An un-localized defect withholds every finding — fail closed broadly.
		return true
	}
	return span.Ref == o.WithheldSpan.Ref
}

// Validate rules on an invocation's charter (FR-14) — the pure, hermetic gate
// that resolves an [Invocation] to one of the three [Kind] outcomes and NEVER a
// silent pass. It returns:
//
//   - (KindRefuse, [ErrNoCharter]) when the charter is empty (or whitespace-only)
//     — the invocation is refused with the named defect and the returned Outcome
//     produces no review (ProducesReview()==false). The error IS the named
//     defect, so a caller both branches (errors.Is) and reports the refusal.
//   - (KindEscalate, [ErrDefectiveCharter]) when the charter is present but the
//     caller reported a material defect — the Outcome carries the withheld span
//     and the verdict.Signals escalate (the FR-17 brief). The error names the
//     escalate class; a caller consuming the Outcome escalates and withholds,
//     while a caller that wants the sentinel has it. If the reported defect is
//     malformed (an unnamed class or an incomplete brief) Validate returns
//     (KindRefuse, a wrapping error) rather than escalate a defect it cannot
//     express — a malformed report is a programmer error, and the REFUSE outcome
//     fails closed even if the caller ignores the error, never a silent escalate
//     or proceed.
//   - (KindProceed, nil) when the charter is present and no material defect was
//     reported — the review proceeds.
//
// The order is deliberate: PRESENCE is checked FIRST (absence dominates a
// defect report — you cannot have a materially-defective absent charter), then
// the defect report. There is no code path that returns KindProceed for an
// empty charter or omits an error on a refuse/escalate — the "no silent pass"
// guarantee is the function's postcondition, asserted by the tests.
func Validate(inv Invocation) (Outcome, error) {
	// PRESENCE FIRST: a charter-less invocation is refused, named, no review.
	// A whitespace-only charter is no charter — trim before the emptiness test so
	// a blank-but-present string does not sneak past as a "present" charter.
	if isBlank(inv.Charter) {
		return Outcome{Kind: KindRefuse}, fmt.Errorf("%w", ErrNoCharter)
	}

	// A present charter with NO reported material defect proceeds.
	if inv.Defect == nil {
		return Outcome{Kind: KindProceed}, nil
	}

	// A present charter WITH a reported material defect escalates upstream (FR-17)
	// and withholds findings against the defective span (FR-14). First validate
	// the caller's report is well-formed — an escalate under an unnamed class or a
	// hollow brief is not an actionable escalation. A malformed report returns a
	// REFUSE outcome (not the zero Outcome, whose Kind would read as proceed):
	// even a caller that ignores the error fails CLOSED, never a silent proceed.
	if err := inv.Defect.validate(); err != nil {
		return Outcome{Kind: KindRefuse}, err
	}

	span := inv.Defect.Span
	// Copy the brief so the Outcome's Signals never alias the caller's Invocation
	// (the same non-aliasing care verdict.Compute takes with its Signals).
	brief := inv.Defect.Brief
	sig := &verdict.Signals{Escalation: &verdict.Escalation{Brief: brief}}

	spanCopy := span
	return Outcome{
		Kind:         KindEscalate,
		Class:        inv.Defect.Class,
		WithheldSpan: &spanCopy,
		Signals:      sig,
	}, fmt.Errorf("%w: class=%s, span=%s", ErrDefectiveCharter, inv.Defect.Class, span)
}

// validate checks a caller-supplied material-defect report is well-formed enough
// to escalate: a NAMED defect class and a COMPLETE brief (the same completeness
// bar verdict.Compute enforces, so an escalate this package hands to the verdict
// layer never fails there for a hollow brief). A malformed report is refused by
// Validate rather than escalated under an unnamed class or with an unactionable
// brief — a programmer error surfaced, never a silent degrade.
func (d *MaterialDefect) validate() error {
	if !d.Class.Valid() {
		return fmt.Errorf("charter: material-defect report has an unnamed defect class %q (want one of ambiguous/self-contradictory/contradicts-governing-design)", d.Class)
	}
	if !briefComplete(d.Brief) {
		return fmt.Errorf("charter: material-defect report carries an incomplete escalation brief (needs conflict, options, and decision-needed) — cannot escalate a hollow brief")
	}
	return nil
}

// briefComplete reports whether an escalation brief is actionable — a conflict,
// at least one non-empty option, and a decision needed (FR-17). It mirrors the
// verdict package's own completeness rule so this package refuses a hollow brief
// at the invocation gate rather than hand verdict.Compute a brief it would
// reject; the two agree by construction (a brief that passes here passes there).
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

// isBlank reports whether a charter string is empty or whitespace-only — a
// whitespace-only charter is NO charter (it states nothing to review against), so
// it triggers the charter-presence refusal exactly as an empty string does.
func isBlank(s string) bool {
	return strings.TrimSpace(s) == ""
}
