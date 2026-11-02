package protocol

import (
	"fmt"

	"github.com/agenticlab-ai/pyxis/internal/implreview/report"
)

// RoundScope is the scope a SECOND round is locked to (FR-8): the named defects
// (the round-1 blocking findings the fix was supposed to close) plus the spans
// the revision actually changed. Round 2 re-reviews ONLY these — a round-2
// finding outside this scope is WITHHELD as a violation (ErrWithheldOnUnchanged),
// never shipped. This is what makes the two-round model a bounded dispute rather
// than an open-ended re-review (§3.1, "re-reviews only the named defects + the
// changed spans").
//
// A finding is IN SCOPE for round 2 iff EITHER it re-raises a NAMED defect (its
// rule_id matches a named defect's — the same defect the round is disputing) OR
// its location falls within a CHANGED span (the fix touched that content, so a
// new finding there is legitimately in scope). Anything else is on content the
// revision did not change and the round did not name — a scope violation.
type RoundScope struct {
	// NamedDefects is the set of round-1 blocking findings this round re-reviews —
	// the defects the fix named. A round-2 finding whose RuleID matches one of
	// these is in scope (it is a re-judgment of that named defect). Matched by
	// RuleID, the lens+rule identity that names the defect class.
	NamedDefects []report.Finding
	// ChangedSpans is the set of spans the revision changed. A round-2 finding
	// whose location falls in a changed span is in scope (the fix touched that
	// content). Anything outside both NamedDefects and ChangedSpans is withheld.
	ChangedSpans SpanSet
}

// namesDefect reports whether a finding re-raises one of the round's named
// defects — matched by RuleID (the defect identity the round is disputing). A
// finding with the same rule_id as a named defect is a re-judgment of that
// defect and is in scope regardless of its span.
func (rs RoundScope) namesDefect(f report.Finding) bool {
	for i := range rs.NamedDefects {
		if rs.NamedDefects[i].RuleID != "" && rs.NamedDefects[i].RuleID == f.RuleID {
			return true
		}
		// A named defect matched by its FND anchor is also in scope — the round-2
		// finding may re-raise the same defect by its id.
		if rs.NamedDefects[i].ID != "" && rs.NamedDefects[i].ID == f.ID {
			return true
		}
	}
	return false
}

// InScope reports whether a round-2 finding is within the round's scope: it
// either re-raises a named defect (by rule_id or FND id) or lands in a changed
// span. A finding neither naming a defect nor in a changed span is out of scope
// (withheld).
func (rs RoundScope) InScope(f report.Finding) bool {
	if rs.namesDefect(f) {
		return true
	}
	return rs.ChangedSpans.CoversLocation(f.Path, f.Line)
}

// ScopeResult is the outcome of applying the round-2 scope-lock to a set of
// candidate round-2 findings (FR-8): the findings that survive the scope-lock
// (in scope — a named defect or a changed span) and the ones WITHHELD as
// violations (on unchanged content the round did not name). The withheld set is
// surfaced (not silently dropped) so the observability surface can show WHY a
// round-2 finding did not ship (§3.8).
type ScopeResult struct {
	// InScope are the candidate round-2 findings within the round's scope — these
	// ship. In candidate order.
	InScope []report.Finding
	// Withheld are the candidate round-2 findings on unchanged content outside the
	// named defects — WITHHELD as scope violations (ErrWithheldOnUnchanged), never
	// shipped. Surfaced for observability.
	Withheld []report.Finding
}

// ApplyScopeLock partitions a round's candidate findings into the ones IN SCOPE
// (which ship) and the ones WITHHELD as violations (on unchanged content — FR-8).
// This is the round-2 scope-lock: round 2 re-reviews only the named defects plus
// the changed spans, so a candidate finding outside that scope is a violation
// that is withheld, not shipped.
//
// roundIndex is 1-based. The scope-lock applies to round 2 (and would apply to
// any later round, but MaxRounds forbids one). Round 1 is the FRESH review — it
// has no scope-lock (every finding is in scope), so ApplyScopeLock returns all
// candidates in scope for round 1. A roundIndex past MaxRounds is refused
// (ErrThirdRoundRefused) — a third round is never scoped, it is refused
// outright.
//
// It fails closed on a third-round attempt; otherwise it never errors — a
// candidate is either in scope or withheld, both recorded in the ScopeResult.
func ApplyScopeLock(roundIndex int, candidates []report.Finding, scope RoundScope) (*ScopeResult, error) {
	if roundIndex > MaxRounds {
		return nil, fmt.Errorf("%w: attempted round %d", ErrThirdRoundRefused, roundIndex)
	}

	res := &ScopeResult{}
	if roundIndex <= 1 {
		// Round 1 is the fresh review: no scope-lock, every candidate is in scope.
		res.InScope = append(res.InScope, candidates...)
		return res, nil
	}

	// Round 2: partition by scope. A candidate that names a defect or lands in a
	// changed span ships; anything else is withheld as a violation.
	for _, c := range candidates {
		if scope.InScope(c) {
			res.InScope = append(res.InScope, c)
		} else {
			res.Withheld = append(res.Withheld, c)
		}
	}
	return res, nil
}

// WithheldError builds the ErrWithheldOnUnchanged sentinel for a specific
// withheld finding — the named error a caller/test uses to assert a particular
// round-2 finding was withheld as a scope violation (rather than shipped). It is
// a convenience over inspecting ScopeResult.Withheld; the partition in
// ApplyScopeLock is the source of truth.
func WithheldError(f report.Finding) error {
	loc := f.Path
	if loc == "" {
		loc = "(no path)"
	}
	return fmt.Errorf("%w: finding %q (rule %q) at %s:%d", ErrWithheldOnUnchanged, f.ID, f.RuleID, loc, f.Line)
}

// RefuseThirdRound reports the ErrThirdRoundRefused error when a caller is about
// to open a round past the two-round cap, and nil when the next round is within
// the cap (§3.1). nextRoundIndex is the 1-based index of the round the caller
// wants to open: 1 and 2 are allowed, 3+ is refused.
//
// This is the explicit round-cap gate a caller calls BEFORE building a new
// round — the structural enforcement of "a third round is refused". It is the
// same refusal ApplyScopeLock enforces, exposed as a standalone predicate for a
// caller that gates round creation before it has candidates.
func RefuseThirdRound(nextRoundIndex int) error {
	if nextRoundIndex > MaxRounds {
		return fmt.Errorf("%w: attempted to open round %d", ErrThirdRoundRefused, nextRoundIndex)
	}
	return nil
}
