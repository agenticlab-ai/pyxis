package checks

import (
	"context"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/verify/diffset"
)

func TestCoverage90FalsificationInputAndModeOnlyBoundaries(t *testing.T) {
	if got := falsificationCheck(context.Background(), RunContext{}); got.Result.State != "error" {
		t.Fatalf("falsificationCheck without diff state = %q, want error", got.Result.State)
	}
	if got := falsificationCheck(context.Background(), RunContext{Diff: &diffset.DiffSet{}}); got.Result.State != "error" {
		t.Fatalf("falsificationCheck without manifest state = %q, want error", got.Result.State)
	}

	paths, changed := classifyFalsificationDiff(&diffset.DiffSet{Files: []diffset.FileChange{{
		Status: diffset.StatusModified,
		Path:   "main.go",
	}}}, nil)
	if len(paths) != 0 || changed {
		t.Fatalf("mode-only change classified as paths=%v changed=%v, want neither", paths, changed)
	}
}

func TestCoverage90ParseNameStatusTruncation(t *testing.T) {
	for _, raw := range []string{"R100\x00old", "C100\x00old", "A", "D", "M"} {
		t.Run(raw[:1], func(t *testing.T) {
			if _, err := parseNameStatusZ([]byte(raw)); err == nil {
				t.Fatalf("parseNameStatusZ(%q) succeeded, want truncated-record error", raw)
			}
		})
	}
}
