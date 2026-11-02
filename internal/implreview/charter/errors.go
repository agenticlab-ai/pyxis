package charter

import "errors"

// Sentinel errors — the NAMED invocation defects (FR-14). Each is part of the
// package contract, one per fail-closed refusal condition, so a caller/test can
// branch with errors.Is and a refusal always names WHICH defect refused the
// invocation (never an anonymous "invalid input"). A refusal carries one of
// these; it is the "named defect" FR-14 requires and the guarantee that no
// invocation path degrades to a silent pass.
var (
	// ErrNoCharter — the invocation carried NO task charter (FR-14). A review has
	// nothing to review AGAINST without the charter that states what the change
	// must satisfy, so the invocation is REFUSED and NO review is produced (no
	// findings, no verdict) — never a silent pass over a spec-less proposal. This
	// is the charter-presence defect: distinct from a charter that is present but
	// broken (that ESCALATES — see [OutcomeEscalate] — rather than refuses).
	ErrNoCharter = errors.New("impl-review charter: invocation carries no task charter — refused, no review produced (FR-14)")

	// ErrDefectiveCharter — the charter is PRESENT but MATERIALLY DEFECTIVE for
	// review (ambiguous, self-contradictory, or contradicting the governing
	// design — FR-14). It is an UPSTREAM defect: the spec, not the code, is
	// broken, so the reviewer neither refuses outright (a charter exists) nor
	// reviews the code against a broken spec. It ESCALATES the upstream-defect
	// class (FR-17) and WITHHOLDS findings against the defective span. This
	// sentinel names the escalate class; [Validate] returns it alongside the
	// escalate [Outcome] so a caller can branch with errors.Is while still
	// carrying the FR-17 brief.
	ErrDefectiveCharter = errors.New("impl-review charter: charter is materially defective for review — escalate upstream, withhold findings against the defective span (FR-14/FR-17)")
)
