package draftshell

import (
	"context"
	"strings"
	"testing"
)

func TestRunReturnsOutputAndNamesProcessErrors(t *testing.T) {
	if out, err := Run(context.Background(), t.TempDir(), nil, "git", "--version"); err != nil || !strings.Contains(string(out), "git version") {
		t.Fatalf("git version: %q %v", out, err)
	}
	if _, err := Run(context.Background(), t.TempDir(), nil, "nonexistent-pyxis-fixture-command"); err == nil || !strings.Contains(err.Error(), "nonexistent-pyxis-fixture-command") {
		t.Fatalf("missing process: %v", err)
	}
}
