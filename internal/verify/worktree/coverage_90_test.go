package worktree

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCoverage90CreateAndProvisionFilesystemBoundaries(t *testing.T) {
	t.Run("scratch parent is file", func(t *testing.T) {
		workspace := t.TempDir()
		if err := os.WriteFile(filepath.Join(workspace, ".codegate"), []byte("occupied"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := Create(t.Context(), workspace, "HEAD", "run"); err == nil {
			t.Fatal("Create accepted a file as scratch parent")
		}
	})

	t.Run("existing destination is preserved", func(t *testing.T) {
		root := t.TempDir()
		source := filepath.Join(root, "source")
		target := filepath.Join(root, "target")
		for _, path := range []string{filepath.Join(source, "state"), filepath.Join(target, "state")} {
			if err := os.MkdirAll(path, 0o755); err != nil {
				t.Fatal(err)
			}
		}
		w := &EphemeralWorktree{workspace: source, Path: target}
		if err := w.Provision(t.Context(), []string{"state"}); err != nil {
			t.Fatalf("Provision existing destination: %v", err)
		}
	})
}

func TestCoverage90RunGitRetriesTransientCollision(t *testing.T) {
	prior := runGitOnceFunc
	t.Cleanup(func() { runGitOnceFunc = prior })
	calls := 0
	runGitOnceFunc = func(_ context.Context, _ string, _ ...string) error {
		calls++
		if calls == 1 {
			return errors.New("unable to create lock: file exists")
		}
		return nil
	}
	if err := runGit(t.Context(), t.TempDir(), "worktree", "prune"); err != nil {
		t.Fatalf("runGit transient retry: %v", err)
	}
	if calls != 2 {
		t.Fatalf("runGit calls = %d, want 2", calls)
	}
}
