package lifecycle

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	draftshell "github.com/agenticlab-ai/pyxis/internal/adapters/draft/shell"
)

func TestRollbackDraftStartRestoresPrecommitWorktree(t *testing.T) {
	root, _ := gitFixture(t)
	prior := gitTest(t, root, "rev-parse", "HEAD")
	gitTest(t, root, "switch", "-c", "draft/widget", "origin/main")
	path := filepath.Join(root, "drafts/widget/prd.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("partial draft"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := rollbackDraftStart(t.Context(), root, "draft/widget", "main", prior, []string{"drafts/widget/prd.md", "drafts/widget/missing.md"}, false, draftshell.Run); err != nil {
		t.Fatal(err)
	}
	if gitTest(t, root, "branch", "--show-current") != "main" || gitTest(t, root, "rev-parse", "HEAD") != prior || gitTest(t, root, "status", "--porcelain") != "" {
		t.Fatal("precommit rollback did not restore the caller worktree")
	}
	if branch := gitTest(t, root, "branch", "--list", "draft/widget"); branch != "" {
		t.Fatalf("incomplete branch remains: %s", branch)
	}
}

func TestRollbackDraftStartReportsRecoveryFailures(t *testing.T) {
	root := t.TempDir()
	blocked := filepath.Join(root, "blocked")
	if err := os.Mkdir(blocked, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(blocked, "child"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	run := func(context.Context, string, []string, string, ...string) ([]byte, error) {
		return nil, errors.New("simulated recovery failure")
	}
	err := rollbackDraftStart(t.Context(), root, "draft/widget", "main", "before", []string{"blocked"}, false, run)
	for _, named := range []string{"remove partial draft file", "reset failed draft branch", "restore prior branch", "remove incomplete draft branch"} {
		if err == nil || !strings.Contains(err.Error(), named) {
			t.Fatalf("rollback error %v omitted %q", err, named)
		}
	}
	err = rollbackDraftStart(t.Context(), root, "draft/widget", "", "before", nil, true, run)
	if err == nil || !strings.Contains(err.Error(), "restore prior head") {
		t.Fatalf("detached rollback error = %v", err)
	}
}
