// Package protocol is the impl-review two-round dispute protocol — the rules
// that govern how a worker disposes a blocking finding, how a second round is
// scope-locked to the named defects and their changed spans, how every blocking
// finding is disposed before a raise, and how a schema-invalid round is voided
// (docs/tech-design/impl-review-v1.md §3.1, §3.7; plan T8 — FR-8, FR-15, UB-2,
// UB-4).
//
// The two-round cap is STRUCTURAL, not documented: the model has exactly two
// evaluations of a proposal (§3.1, "the two-round cap structural"). Round 1 is
// the fresh review; round 2 re-reviews ONLY the named defects plus the spans the
// revision changed; a third round is REFUSED. Everything here is a pure function
// over report types — this package DETECTS a protocol condition (a reason-less
// dispute, a declined blocker, a scope violation, an undisposed raise, a
// schema-invalid round) and the caller (the /impl-review skill, the host) ACTS
// on it, exactly as band detects oversize and verdict acts on a caller-signalled
// escalate. In particular:
//
//   - a DECLINED blocking finding is detected here and surfaced as a
//     verdict.Escalation the caller feeds to verdict.Compute (FR-8 → FR-17); this
//     package never computes the verdict itself (T7 owns that);
//   - an APPLIED blocking finding renders VERIFIED only after a round-2
//     re-review confirms the named defect closed; otherwise it is an UNVERIFIED
//     claim that HOLDS the verdict at request-changes (FR-15) — this package
//     produces the round-2 finding set so verdict.Compute sees the blocker still
//     holding on an unverified claim (it never edits verdict.resolvedAway, which
//     already refuses to let a worker `applied` resolve a blocker);
//   - a schema-invalid round is VOIDED, retried once, then recorded ABSENT
//     (UB-2) — this package reuses report.Validate to detect invalidity and
//     returns ErrReviewAbsent, the terminal a caller maps to the T2/T9
//     review-absence outcome (band's ErrOversize precedent: the package names
//     the terminal, the host records the absence).
//
// This package imports report (T1) and verdict (T7) — it depends on them, it
// edits neither. Stdlib only otherwise, per the repo Go rules.
package protocol

import (
	"errors"
	"strings"
)

// MaxRounds is the two-round cap (§3.1). A proposal is evaluated at most twice:
// round 1 (the fresh review) and round 2 (the scope-locked re-review). A third
// round is refused (RefuseThirdRound / ErrThirdRoundRefused). The cap is a
// constant, not a config knob — it is the structural shape of the dispute
// protocol, not a tunable.
const MaxRounds = 2

// Sentinel errors — part of the package contract, one per fail-closed protocol
// condition, so a caller/test can branch with errors.Is (the band/verdict/report
// precedent).
var (
	// ErrDisputeNoReason — a worker disputed a blocking finding with no reason.
	// A reason-less dispute is not a valid disposition; it VOIDS its round (UB-2,
	// FR-8): RecordDisposition refuses it and the round it would have belonged to
	// is void (VoidRound), never counted.
	ErrDisputeNoReason = errors.New("impl-review protocol: a dispute must carry a reason — a reason-less dispute voids its round")

	// ErrThirdRoundRefused — an attempt to open a third round (a round index past
	// MaxRounds). The two-round cap is structural (§3.1); a third round is
	// refused, never opened.
	ErrThirdRoundRefused = errors.New("impl-review protocol: a third round is refused — the two-round cap is structural")

	// ErrWithheldOnUnchanged — a round-2 finding lands on content the revision did
	// NOT change (outside the named defects and their changed spans). Round 2 is
	// scope-locked to the named defects + changed spans (FR-8); a finding outside
	// that scope is WITHHELD as a violation, never shipped.
	ErrWithheldOnUnchanged = errors.New("impl-review protocol: a round-2 finding on unchanged content is withheld as a scope violation")

	// ErrScopeDrift — a revision changed spans OUTSIDE the named defects (UB-4).
	// The out-of-scope change is FLAGGED as scope drift, never silently
	// re-reviewed. DetectDrift returns this wrapping the drifted spans.
	ErrScopeDrift = errors.New("impl-review protocol: the revision changed spans outside the named defects (scope drift)")

	// ErrUndisposedBlocking — a raise carries a blocking finding with no recorded
	// disposition (FR-15). Before a raise, EVERY blocking finding must carry a
	// disposition; a raise with an undisposed blocking finding is a protocol
	// defect, refused.
	ErrUndisposedBlocking = errors.New("impl-review protocol: a raise carries a blocking finding with no recorded disposition (disposition-before-raise)")

	// ErrReviewAbsent — a round was schema-invalid, voided, retried once, and the
	// retry was ALSO schema-invalid (UB-2). The terminal outcome is
	// review-absence; a caller maps this to the T2/T9 review-absence recording
	// (band's ErrOversize precedent — this package names the terminal, the host
	// records the absence). It is NEVER a silent pass.
	ErrReviewAbsent = errors.New("impl-review protocol: a schema-invalid round voided, retried once, and failed again — review-absent")
)

// ChangedSpan is a contiguous region a revision changed — a repo-relative path
// and an inclusive 1-based line range within it. The two-round scope-lock is
// evaluated against a set of ChangedSpans plus the named defects: round 2
// re-reviews the named defects and the spans the fix touched, and NOTHING else
// (FR-8). A whole-file change is a span with StartLine 0 (file-level), which
// Contains treats as covering every line of the path.
//
// A ChangedSpan is a value type with no clock/IO — the whole protocol is a pure
// function over (findings, dispositions, spans, named defects).
type ChangedSpan struct {
	// Path is the repo-relative file the span is in.
	Path string
	// StartLine is the inclusive 1-based first changed line, or 0 for a
	// file-level change (the whole file is in scope).
	StartLine int
	// EndLine is the inclusive last changed line; ignored when StartLine is 0.
	EndLine int
}

// Contains reports whether a finding at (path, line) falls within this changed
// span. A file-level span (StartLine 0) contains every line of its path; a
// line-ranged span contains a finding whose line is within [StartLine, EndLine].
// A finding with line 0 (file-level) matches only a file-level span on the same
// path — a file-level finding is "in scope" only when the whole file changed.
func (s ChangedSpan) Contains(path string, line int) bool {
	if !strings.EqualFold(s.Path, path) {
		return false
	}
	if s.StartLine == 0 {
		// A whole-file change covers every line of the path (including a
		// file-level finding, line 0).
		return true
	}
	if line == 0 {
		// A file-level finding is in scope only for a whole-file change, handled
		// above; a line-ranged span does not cover a file-level finding.
		return false
	}
	return line >= s.StartLine && line <= s.EndLine
}

// SpanSet is the set of spans a revision changed, evaluated together against a
// finding's location. It is the scope half of the round-2 scope-lock (the named
// defects are the other half).
type SpanSet []ChangedSpan

// CoversLocation reports whether any span in the set contains (path, line) — the
// "is this location a changed span" predicate the scope-lock uses. An empty set
// covers nothing (a round-2 finding with no changed span behind it is out of
// scope unless it names an in-scope defect).
func (ss SpanSet) CoversLocation(path string, line int) bool {
	for _, s := range ss {
		if s.Contains(path, line) {
			return true
		}
	}
	return false
}
