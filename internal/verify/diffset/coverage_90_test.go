package diffset

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestCoverage90GitFailureBoundaries(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	if _, err := batchSizes(t.Context(), missing, "HEAD", []string{"file.go"}); err == nil {
		t.Fatal("batchSizes accepted a missing workspace")
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := runGit(ctx, t.TempDir(), "status"); err == nil || !strings.Contains(err.Error(), "git status") {
		t.Fatalf("cancelled runGit error = %v", err)
	}
}
