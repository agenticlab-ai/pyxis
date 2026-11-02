package protocol

import (
	"errors"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/implreview/report"
)

// TestDetectDriftFlagsOutOfScopeChange — a revision that changes spans OUTSIDE
// the named defects is flagged as scope drift, not silently re-reviewed (UB-4).
func TestDetectDriftFlagsOutOfScopeChange(t *testing.T) {
	// The named defect lives at internal/a/a.go:10 (tolerance 5 lines either side).
	named := []DefectSpan{{Path: "internal/a/a.go", Line: 10, Tolerance: 5}}

	t.Run("a change within the defect window → no drift", func(t *testing.T) {
		changed := SpanSet{{Path: "internal/a/a.go", StartLine: 9, EndLine: 12}}
		res := DetectDrift(named, changed)
		if res.HasDrift() {
			t.Errorf("a within-window fix must not drift: %+v", res.Drifted)
		}
		if len(res.Attributed) != 1 {
			t.Errorf("the fix span should be attributed to the defect")
		}
		if err := DriftError(res); err != nil {
			t.Errorf("DriftError = %v, want nil", err)
		}
	})

	t.Run("a change in an unrelated file → DRIFT", func(t *testing.T) {
		changed := SpanSet{
			{Path: "internal/a/a.go", StartLine: 9, EndLine: 12},         // the fix
			{Path: "internal/unrelated/u.go", StartLine: 1, EndLine: 40}, // drift
		}
		res := DetectDrift(named, changed)
		if !res.HasDrift() {
			t.Fatalf("an unrelated-file change must be flagged as drift")
		}
		if len(res.Drifted) != 1 || res.Drifted[0].Path != "internal/unrelated/u.go" {
			t.Errorf("drifted = %+v, want the unrelated file", res.Drifted)
		}
		if err := DriftError(res); !errors.Is(err, ErrScopeDrift) {
			t.Errorf("DriftError = %v, want ErrScopeDrift", err)
		}
	})

	t.Run("a change beyond the defect window in the same file → DRIFT", func(t *testing.T) {
		changed := SpanSet{{Path: "internal/a/a.go", StartLine: 100, EndLine: 140}}
		res := DetectDrift(named, changed)
		if !res.HasDrift() {
			t.Errorf("a same-file change far from the defect must drift")
		}
	})
}

// TestDefectSpansFromFindings — the drift baseline derives from round-1 blocking
// findings; a finding with no path anchors no span.
func TestDefectSpansFromFindings(t *testing.T) {
	findings := []report.Finding{
		blockingF("FND-1", "r1", "internal/a/a.go", 10),
		blockingF("FND-2", "r2", "", 0),                // no path — skipped
		blockingF("FND-3", "r3", "internal/b/b.go", 0), // file-level
	}
	spans := DefectSpansFromFindings(findings, 3)
	if len(spans) != 2 {
		t.Fatalf("spans = %d, want 2 (the path-less finding is skipped)", len(spans))
	}
	// The file-level defect (Line 0) licenses the whole file.
	res := DetectDrift(spans, SpanSet{{Path: "internal/b/b.go", StartLine: 500, EndLine: 999}})
	if res.HasDrift() {
		t.Errorf("a whole-file defect must license any change in the file: %+v", res.Drifted)
	}
}

// TestFileLevelDefectLicensesFile — a file-level named defect (Line 0) licenses
// any change in that file; a line-anchored one does not license a whole-file
// change.
func TestFileLevelDefectLicensesFile(t *testing.T) {
	t.Run("file-level defect licenses a whole-file change", func(t *testing.T) {
		named := []DefectSpan{{Path: "a.go", Line: 0}}
		res := DetectDrift(named, SpanSet{{Path: "a.go", StartLine: 0}})
		if res.HasDrift() {
			t.Errorf("a file-level defect must license a whole-file change")
		}
	})
	t.Run("line-anchored defect does NOT license a whole-file change", func(t *testing.T) {
		named := []DefectSpan{{Path: "a.go", Line: 10, Tolerance: 2}}
		res := DetectDrift(named, SpanSet{{Path: "a.go", StartLine: 0}})
		if !res.HasDrift() {
			t.Errorf("a whole-file change exceeds a line-anchored defect's window — must drift")
		}
	})
}
