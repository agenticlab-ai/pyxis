package protocol

import (
	"fmt"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/implreview/report"
)

// DefectSpan is where a NAMED defect lives — the location a round-1 blocking
// finding pointed at, which a legitimate fix is expected to change. Scope drift
// (UB-4) is measured against these: a revision that changes spans OUTSIDE the
// named defects' locations has drifted beyond the dispute, and that drift is
// FLAGGED, never silently re-reviewed.
//
// A DefectSpan is derived from a named defect's finding location (its Path and,
// when line-anchored, a tolerance window around its Line). A file-level named
// defect (Line 0) licenses changes anywhere in that file; a line-anchored one
// licenses changes within a window around the anchor (Tolerance lines either
// side), because a real fix touches lines near the defect, not only the exact
// line.
type DefectSpan struct {
	// Path is the file the named defect is in — changes to this file (within the
	// window) are in-scope for the fix.
	Path string
	// Line is the named defect's anchor line (0 for a file-level defect, which
	// licenses the whole file).
	Line int
	// Tolerance is how many lines either side of Line a change may fall and still
	// be attributed to fixing this defect. Ignored for a file-level defect. A
	// zero tolerance means only the exact anchor line is licensed.
	Tolerance int
}

// licenses reports whether a changed span is attributable to fixing this named
// defect — same path, and (for a line-anchored defect) overlapping the anchor's
// tolerance window. A file-level defect licenses any span in its path.
func (d DefectSpan) licenses(s ChangedSpan) bool {
	if !strings.EqualFold(d.Path, s.Path) {
		return false
	}
	if d.Line == 0 {
		// A file-level named defect licenses any change in the file.
		return true
	}
	if s.StartLine == 0 {
		// A whole-file change cannot be attributed to a single line-anchored
		// defect — it is broader than the defect's window.
		return false
	}
	lo := d.Line - d.Tolerance
	if lo < 1 {
		lo = 1
	}
	hi := d.Line + d.Tolerance
	// The changed span [StartLine, EndLine] must lie within the defect's window
	// [lo, hi] to be attributed to this defect; a change extending beyond the
	// window is drift.
	return s.StartLine >= lo && s.EndLine <= hi
}

// DriftResult is the outcome of the scope-drift check (UB-4): the changed spans
// attributed to fixing a named defect (in-scope) and the DRIFTED spans that fall
// outside every named defect's window. A non-empty Drifted set is scope drift —
// FLAGGED (DriftError), never silently re-reviewed.
type DriftResult struct {
	// Attributed are the changed spans a named defect licenses (a legitimate fix).
	// In input order.
	Attributed []ChangedSpan
	// Drifted are the changed spans no named defect licenses — the revision
	// changed content outside the dispute (scope drift, UB-4). In input order.
	Drifted []ChangedSpan
	// Drifted being non-empty is the flag condition; HasDrift reports it.
}

// HasDrift reports whether the revision drifted — changed at least one span
// outside the named defects (UB-4).
func (r *DriftResult) HasDrift() bool { return len(r.Drifted) > 0 }

// DetectDrift flags scope drift (UB-4): a revision that changed spans OUTSIDE the
// named defects is drift, FLAGGED and surfaced, never silently re-reviewed. It
// partitions the revision's changed spans into the ones ATTRIBUTED to a named
// defect (a legitimate fix, within a defect's window) and the DRIFTED ones (no
// named defect licenses them).
//
// namedDefects are the round-1 blocking findings the round is disputing —
// changes near their locations are the fix; changes elsewhere are drift.
// changedSpans are the spans the revision actually changed. It returns a
// DriftResult partitioning them; HasDrift reports whether any drifted.
//
// This is DISTINCT from the round-2 scope-lock (ApplyScopeLock): the scope-lock
// partitions the reviewer's round-2 FINDINGS by scope, whereas DetectDrift
// examines the REVISION's changed spans themselves — did the worker change
// things beyond the named defects? A drifted revision is flagged (the caller
// routes it, e.g. a size/scope escalation to a human), not silently re-reviewed
// as if the extra changes were in scope.
func DetectDrift(namedDefects []DefectSpan, changedSpans SpanSet) *DriftResult {
	res := &DriftResult{}
	for _, s := range changedSpans {
		attributed := false
		for _, d := range namedDefects {
			if d.licenses(s) {
				attributed = true
				break
			}
		}
		if attributed {
			res.Attributed = append(res.Attributed, s)
		} else {
			res.Drifted = append(res.Drifted, s)
		}
	}
	return res
}

// DriftError builds the ErrScopeDrift sentinel naming the drifted spans — the
// named error a caller/test uses to assert (and log) that a revision drifted
// outside the named defects (UB-4). It returns nil when there is no drift, so a
// caller can `if err := DriftError(res); err != nil { … }` after DetectDrift.
func DriftError(res *DriftResult) error {
	if res == nil || !res.HasDrift() {
		return nil
	}
	locs := make([]string, 0, len(res.Drifted))
	for _, s := range res.Drifted {
		locs = append(locs, fmt.Sprintf("%s:%d-%d", s.Path, s.StartLine, s.EndLine))
	}
	return fmt.Errorf("%w: drifted spans %s", ErrScopeDrift, strings.Join(locs, ", "))
}

// DefectSpansFromFindings derives the DefectSpans a round-2 drift check measures
// against from a set of named-defect findings — a convenience so a caller that
// holds round-1 blocking findings (report.Finding) can build the drift baseline
// without hand-mapping. Each finding's Path and Line become a DefectSpan with
// the given tolerance; a finding with no Path is skipped (it anchors no span to
// license a change against).
func DefectSpansFromFindings(findings []report.Finding, tolerance int) []DefectSpan {
	var spans []DefectSpan
	for i := range findings {
		f := &findings[i]
		if f.Path == "" {
			continue
		}
		spans = append(spans, DefectSpan{Path: f.Path, Line: f.Line, Tolerance: tolerance})
	}
	return spans
}
