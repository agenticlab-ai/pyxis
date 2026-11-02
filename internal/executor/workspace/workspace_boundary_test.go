package workspace

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

func TestNewHonorsExplicitWorktreeRoot(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t)
	wantRoot := filepath.Join(h.root, "custom-worktrees")

	w, err := New(ctx, Config{
		Parent:   h.parent,
		Remote:   "origin",
		Mainline: "main",
		Root:     wantRoot,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if w.root != wantRoot {
		t.Fatalf("root = %q, want %q", w.root, wantRoot)
	}
}

func TestPrepareReportsProvisioningBoundaries(t *testing.T) {
	t.Run("guard cannot be armed", func(t *testing.T) {
		w := &Workspace{
			parent:   t.TempDir(),
			remote:   "origin",
			mainline: "main",
			root:     filepath.Join(t.TempDir(), "worktrees"),
		}
		_, err := w.Prepare(t.Context(), port.Task{ID: "T-guard", HeadBranch: "feat/guard"})
		if err == nil || !strings.Contains(err.Error(), "re-arming the guard") {
			t.Fatalf("Prepare error = %v, want guard failure", err)
		}
	})

	t.Run("worktree root is occupied by a file", func(t *testing.T) {
		h := newHarness(t)
		w := h.newWorkspace(t, t.Context())
		blocked := filepath.Join(h.root, "blocked-root")
		if err := os.WriteFile(blocked, []byte("occupied"), 0o644); err != nil {
			t.Fatal(err)
		}
		w.root = blocked
		_, err := w.Prepare(t.Context(), port.Task{ID: "T-root", HeadBranch: "feat/root"})
		if err == nil || !strings.Contains(err.Error(), "creating the worktree root") {
			t.Fatalf("Prepare error = %v, want root creation failure", err)
		}
	})

	t.Run("deterministic path belongs to another branch", func(t *testing.T) {
		h := newHarness(t)
		w := h.newWorkspace(t, t.Context())
		branch := "feat/requested"
		dir := filepath.Join(w.root, slug(branch))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		gitOK(t, dir, "init", "-q", "-b", "other")
		writeFile(t, filepath.Join(dir, "seed"), "seed\n")
		gitOK(t, dir, "add", "seed")
		gitOK(t, dir, "commit", "-q", "-m", "seed")

		_, err := w.Prepare(t.Context(), port.Task{ID: "T-foreign", HeadBranch: branch})
		if err == nil || !strings.Contains(err.Error(), "not the task branch") {
			t.Fatalf("Prepare error = %v, want foreign-worktree failure", err)
		}
	})

	t.Run("nonempty destination rejects worktree add", func(t *testing.T) {
		h := newHarness(t)
		w := h.newWorkspace(t, t.Context())
		branch := "feat/occupied"
		dir := filepath.Join(w.root, slug(branch))
		writeFile(t, filepath.Join(dir, "occupied"), "occupied\n")

		_, err := w.Prepare(t.Context(), port.Task{ID: "T-add", HeadBranch: branch})
		if err == nil || !strings.Contains(err.Error(), "adding worktree") {
			t.Fatalf("Prepare error = %v, want worktree-add failure", err)
		}
	})
}

func TestWorkspaceGitFailureBoundaries(t *testing.T) {
	h := newHarness(t)

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	w := &Workspace{parent: h.parent, remote: "missing", mainline: "main"}
	if err := w.fetchMainline(canceled); err == nil {
		t.Fatal("fetchMainline with canceled context: want error")
	}

	invalid := &Workspace{parent: filepath.Join(t.TempDir(), "missing")}
	if _, err := invalid.branchExists(t.Context(), "feat/missing"); err == nil {
		t.Fatal("branchExists outside a repository: want error")
	}
	if branch := invalid.worktreeBranch(t.Context(), t.TempDir()); branch != "" {
		t.Fatalf("worktreeBranch on a non-worktree = %q; want empty", branch)
	}
}

func TestWorkdirReportsInvalidStateAndCleanupFailures(t *testing.T) {
	repo := t.TempDir()
	gitOK(t, repo, "init", "-q", "-b", "main")

	withoutBase := &Workdir{dir: repo, branch: "feat/no-base"}
	if _, err := withoutBase.Push(t.Context()); err == nil {
		t.Fatal("Push without a base: want error")
	}
	if _, err := withoutBase.Checkpoint(t.Context()); err == nil {
		t.Fatal("Checkpoint without a base: want error")
	}

	invalid := &Workdir{dir: filepath.Join(t.TempDir(), "missing"), branch: "feat/invalid"}
	if err := invalid.commitPending(t.Context()); err == nil {
		t.Fatal("commitPending outside a repository: want error")
	}

	parent := t.TempDir()
	liveDir := t.TempDir()
	if err := (&Workdir{parent: parent, dir: liveDir, branch: "feat/live"}).Cleanup(t.Context()); err == nil {
		t.Fatal("Cleanup with a live directory outside a repository: want error")
	}
	missingDir := filepath.Join(t.TempDir(), "already-gone")
	if err := (&Workdir{parent: parent, dir: missingDir, branch: "feat/gone"}).Cleanup(t.Context()); err == nil {
		t.Fatal("Cleanup outside a repository: want prune error")
	}
	if _, err := (&Workdir{parent: parent, branch: "feat/check"}).branchExists(t.Context()); err == nil {
		t.Fatal("Workdir.branchExists outside a repository: want error")
	}
}

func TestPushAndCheckpointWrapCommitAndRevisionFailures(t *testing.T) {
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)

	missing := filepath.Join(t.TempDir(), "missing")
	invalid := &Workdir{dir: missing, branch: "feat/invalid", base: "base"}
	if _, err := invalid.Push(t.Context()); err == nil || !strings.Contains(err.Error(), "inspect branch") {
		t.Fatalf("Push error = %v", err)
	}
	if _, err := invalid.Checkpoint(t.Context()); err == nil || !strings.Contains(err.Error(), "resolve checkpoint branch") {
		t.Fatalf("Checkpoint error = %v", err)
	}

	repo := t.TempDir()
	gitOK(t, repo, "init", "-q", "-b", "main")
	gitOK(t, repo, "config", "user.name", "pyxis-test")
	gitOK(t, repo, "config", "user.email", "test@pyxis.local")
	writeFile(t, filepath.Join(repo, "seed"), "seed\n")
	gitOK(t, repo, "add", "seed")
	gitOK(t, repo, "commit", "-q", "-m", "seed")
	badBase := &Workdir{dir: repo, branch: "main", base: "missing-revision"}
	if _, err := badBase.Push(t.Context()); err == nil || !strings.Contains(err.Error(), "counting commits") {
		t.Fatalf("Push bad-base error = %v", err)
	}
	if _, err := badBase.Checkpoint(t.Context()); err == nil || !strings.Contains(err.Error(), "counting commits") {
		t.Fatalf("Checkpoint bad-base error = %v", err)
	}
}
