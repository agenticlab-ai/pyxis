package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/githooks"
)

func installTestHook(t *testing.T, path, body string) {
	t.Helper()
	writeFile(t, path, "#!/bin/sh\nset -e\n"+body)
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

// Exercise Git's installed hook path and an actual push to a local origin. This
// catches the original failure even when all checked-in hook files look correct.
func TestHooksSetupPreservesRepositoryGatesAcrossWorktrees(t *testing.T) {
	h := newHarness(t)
	prePush, err := os.ReadFile("../../../.pyxis/hooks/pre-push")
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(h.parent, ".githooks/pre-push"), string(prePush))
	if err := os.Chmod(filepath.Join(h.parent, ".githooks/pre-push"), 0o755); err != nil {
		t.Fatal(err)
	}
	installTestHook(t, filepath.Join(h.parent, ".githooks/pre-commit"), "printf 'commit\\n' >> hook-events\n")
	installTestHook(t, filepath.Join(h.parent, ".githooks/prepare-commit-msg"), "printf '\\nHook-Stamp: present\\n' >> \"$1\"\n")
	installTestHook(t, filepath.Join(h.parent, ".pyxis/scripts/docgate-resolve.sh"), "printf 'doc\\n' >> hook-events\ncat > hook-refs\n")
	installTestHook(t, filepath.Join(h.parent, ".pyxis/scripts/code-gate-resolve.sh"), "printf 'code\\n' >> hook-events\nexit 27\n")
	gitOK(t, h.parent, "add", ".githooks", ".pyxis")
	gitOK(t, h.parent, "commit", "-qm", "install repository hooks")
	gitOK(t, h.parent, "config", "core.hooksPath", ".githooks")
	// Create the worktree before guarding; both old and new worktrees must inherit
	// the same single dispatcher and resolve relative hook sources in their own tree.
	linked := filepath.Join(h.root, "interactive")
	gitOK(t, h.parent, "worktree", "add", "-qb", "feat/hooks", linked)
	if err := githooks.Install(t.Context(), h.parent); err != nil {
		t.Fatal(err)
	}
	active := gitOK(t, h.parent, "config", "core.hooksPath")
	if got := gitOK(t, linked, "config", "core.hooksPath"); got != active {
		t.Fatalf("worktree hook setup = %q, parent = %q", got, active)
	}
	if got := gitOK(t, linked, "config", "pyxis.repositoryHooksPath"); got != ".githooks" {
		t.Fatalf("repository hooks displaced: %q", got)
	}
	// Rearming through the executor uses the same setup, never chains to itself.
	ws := h.newWorkspace(t, t.Context())
	if _, err := ws.Prepare(t.Context(), port.Task{ID: "T-hooks", HeadBranch: "feat/executor-hooks"}); err != nil {
		t.Fatal(err)
	}
	if got := gitOK(t, linked, "config", "pyxis.repositoryHooksPath"); got != ".githooks" {
		t.Fatalf("rearming changed the hook source: %q", got)
	}
	gitOK(t, linked, "commit", "--allow-empty", "-qm", "interactive change")
	if msg := gitOK(t, linked, "log", "-1", "--format=%B"); !strings.Contains(msg, "Hook-Stamp: present") {
		t.Fatalf("prepare-commit-msg was lost: %s", msg)
	}
	if out, err := gitTry(t, linked, "push", "origin", "HEAD:refs/heads/hook-rejected"); err == nil {
		t.Fatalf("red code gate allowed a push: %s", out)
	}
	if out, err := gitTry(t, h.origin, "show-ref", "--verify", "refs/heads/hook-rejected"); err == nil {
		t.Fatalf("rejected push changed origin: %s", out)
	}
	events, err := os.ReadFile(filepath.Join(linked, "hook-events"))
	if err != nil || string(events) != "commit\ndoc\ncode\n" {
		t.Fatalf("hook execution = %q, %v", events, err)
	}
	refs, err := os.ReadFile(filepath.Join(linked, "hook-refs"))
	if err != nil || !strings.Contains(string(refs), "refs/heads/hook-rejected") {
		t.Fatalf("pre-push stdin lost: %q, %v", refs, err)
	}
	installTestHook(t, filepath.Join(linked, ".pyxis/scripts/code-gate-resolve.sh"), "printf 'code\\n' >> hook-events\n")
	gitOK(t, linked, "push", "origin", "HEAD:refs/heads/hook-accepted")
	if got, want := gitOK(t, h.origin, "rev-parse", "refs/heads/hook-accepted"), gitOK(t, linked, "rev-parse", "HEAD"); got != want {
		t.Fatalf("green gate published %s, want %s", got, want)
	}
	if out, err := gitTry(t, h.parent, "commit", "--allow-empty", "-qm", "blocked parent"); err == nil || !strings.Contains(out, "refusing to mutate") {
		t.Fatalf("parent guard = %v: %s", err, out)
	}
}

func TestHooksSetupMigratesOldGuardAndPreservesCustomHooks(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		t.Run(map[bool]string{false: "custom source", true: "legacy guard"}[legacy], func(t *testing.T) {
			h := newHarness(t)
			source := "custom hooks 'quoted'"
			if legacy {
				source = ".githooks"
			}
			installTestHook(t, filepath.Join(h.parent, source, "pre-push"), "printf '%s\\n' \"$@\" > hook-args\ncat > hook-input\nexit 31\n")
			if legacy {
				old := filepath.Join(h.parent, ".git", guardDirName)
				writeFile(t, filepath.Join(old, "pre-commit"), "#!/bin/sh\nexit 0\n")
				gitOK(t, h.parent, "config", "core.hooksPath", old)
			} else {
				gitOK(t, h.parent, "config", "core.hooksPath", source)
			}
			for range 2 {
				if err := githooks.Install(t.Context(), h.parent); err != nil {
					t.Fatal(err)
				}
			}
			if got := gitOK(t, h.parent, "config", "pyxis.repositoryHooksPath"); got != source {
				t.Fatalf("source = %q, want %q", got, source)
			}
			if out, err := gitTry(t, h.parent, "hook", "run", "pre-push", "--", "origin", "remote with spaces"); err == nil {
				t.Fatalf("custom hook's red result lost: %s", out)
			}
			args, err := os.ReadFile(filepath.Join(h.parent, "hook-args"))
			if err != nil || string(args) != "origin\nremote with spaces\n" {
				t.Fatalf("hook args = %q, %v", args, err)
			}
		})
	}
}

func TestHookSetupRejectsRecursiveSourceAndInvalidRepository(t *testing.T) {
	if err := githooks.Install(t.Context(), t.TempDir()); err == nil {
		t.Fatal("setup outside Git succeeded")
	}
	h := newHarness(t)
	if err := githooks.Install(t.Context(), h.parent); err != nil {
		t.Fatal(err)
	}
	active := gitOK(t, h.parent, "config", "core.hooksPath")
	gitOK(t, h.parent, "config", "pyxis.repositoryHooksPath", active)
	if err := githooks.Install(t.Context(), h.parent); err == nil || !strings.Contains(err.Error(), "back to the dispatcher") {
		t.Fatalf("recursive source = %v", err)
	}
}
