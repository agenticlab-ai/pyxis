package protocol

import (
	"errors"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/implreview/report"
)

// TestRound1HasNoScopeLock — round 1 is the fresh review: every candidate is in
// scope, nothing withheld.
func TestRound1HasNoScopeLock(t *testing.T) {
	candidates := []report.Finding{
		blockingF("FND-1", "r1", "a.go", 5),
		advisoryF("FND-2", "r2"),
	}
	res, err := ApplyScopeLock(1, candidates, RoundScope{})
	if err != nil {
		t.Fatalf("ApplyScopeLock(round 1): %v", err)
	}
	if len(res.InScope) != 2 {
		t.Errorf("round 1 in-scope = %d, want 2 (no scope-lock on the fresh review)", len(res.InScope))
	}
	if len(res.Withheld) != 0 {
		t.Errorf("round 1 withheld = %d, want 0", len(res.Withheld))
	}
}

// TestRound2ScopeLock — round 2 reviews ONLY named defects + changed spans; a
// round-2 finding on unchanged content is WITHHELD as a violation (FR-8).
func TestRound2ScopeLock(t *testing.T) {
	named := blockingF("FND-1", "spec-fidelity/R1", "internal/a/a.go", 10)
	scope := RoundScope{
		NamedDefects: []report.Finding{named},
		ChangedSpans: SpanSet{
			{Path: "internal/a/a.go", StartLine: 8, EndLine: 14}, // the fix's span
		},
	}

	cases := []struct {
		name        string
		candidate   report.Finding
		wantInScope bool
	}{
		{
			name:        "re-raises the named defect by rule_id → in scope",
			candidate:   blockingF("FND-1b", "spec-fidelity/R1", "internal/a/a.go", 10),
			wantInScope: true,
		},
		{
			name:        "new finding inside the changed span → in scope",
			candidate:   blockingF("FND-3", "security-abuse/R7", "internal/a/a.go", 12),
			wantInScope: true,
		},
		{
			name:        "finding on unchanged content, unnamed defect → WITHHELD",
			candidate:   blockingF("FND-4", "design-conformance/R2", "internal/b/b.go", 99),
			wantInScope: false,
		},
		{
			name:        "finding in the same file but OUTSIDE the changed span → WITHHELD",
			candidate:   blockingF("FND-5", "correctness-critic/R4", "internal/a/a.go", 40),
			wantInScope: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := ApplyScopeLock(2, []report.Finding{tc.candidate}, scope)
			if err != nil {
				t.Fatalf("ApplyScopeLock: %v", err)
			}
			if tc.wantInScope {
				if len(res.InScope) != 1 || len(res.Withheld) != 0 {
					t.Errorf("want in-scope; got InScope=%d Withheld=%d", len(res.InScope), len(res.Withheld))
				}
			} else {
				if len(res.Withheld) != 1 || len(res.InScope) != 0 {
					t.Errorf("want withheld; got InScope=%d Withheld=%d", len(res.InScope), len(res.Withheld))
				}
			}
		})
	}
}

// TestScopeLockMixedPartition — a mixed candidate set partitions correctly, and
// WithheldError names a withheld finding.
func TestScopeLockMixedPartition(t *testing.T) {
	named := blockingF("FND-1", "spec-fidelity/R1", "internal/a/a.go", 10)
	scope := RoundScope{
		NamedDefects: []report.Finding{named},
		ChangedSpans: SpanSet{{Path: "internal/a/a.go", StartLine: 8, EndLine: 14}},
	}
	candidates := []report.Finding{
		blockingF("FND-1b", "spec-fidelity/R1", "internal/a/a.go", 10),    // named → in scope
		blockingF("FND-3", "security-abuse/R7", "internal/a/a.go", 12),    // changed span → in scope
		blockingF("FND-9", "design-conformance/R2", "internal/z/z.go", 3), // unchanged → withheld
	}
	res, err := ApplyScopeLock(2, candidates, scope)
	if err != nil {
		t.Fatalf("ApplyScopeLock: %v", err)
	}
	if len(res.InScope) != 2 {
		t.Errorf("in-scope = %d, want 2", len(res.InScope))
	}
	if len(res.Withheld) != 1 {
		t.Fatalf("withheld = %d, want 1", len(res.Withheld))
	}
	if res.Withheld[0].ID != "FND-9" {
		t.Errorf("withheld the wrong finding: %q", res.Withheld[0].ID)
	}
	if err := WithheldError(res.Withheld[0]); !errors.Is(err, ErrWithheldOnUnchanged) {
		t.Errorf("WithheldError = %v, want ErrWithheldOnUnchanged", err)
	}
}

// TestFileLevelChangedSpan — a whole-file change (StartLine 0) puts every line of
// the file in scope, including a file-level finding.
func TestFileLevelChangedSpan(t *testing.T) {
	scope := RoundScope{
		ChangedSpans: SpanSet{{Path: "internal/a/a.go", StartLine: 0}}, // whole file changed
	}
	inFile := blockingF("FND-1", "r1", "internal/a/a.go", 500)
	fileLevel := report.Finding{ID: "FND-2", RuleID: "r2", Path: "internal/a/a.go", Line: 0, Message: "m", CalRow: "CAL-1", Lens: report.LensSpecFidelity, Severity: report.SeverityBlocking}
	otherFile := blockingF("FND-3", "r3", "internal/b/b.go", 1)

	res, err := ApplyScopeLock(2, []report.Finding{inFile, fileLevel, otherFile}, scope)
	if err != nil {
		t.Fatalf("ApplyScopeLock: %v", err)
	}
	if len(res.InScope) != 2 {
		t.Errorf("in-scope = %d, want 2 (both findings in the whole-file change)", len(res.InScope))
	}
	if len(res.Withheld) != 1 || res.Withheld[0].ID != "FND-3" {
		t.Errorf("want FND-3 (other file) withheld; got %+v", res.Withheld)
	}
}
