package workspace

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

// Recovery reopens only the exact checkpoint an operator named. It never
// creates a worktree or branch, and it refuses a checkout that has moved or
// carries uncommitted work.
func TestExisting_ReopensOnlyTheExactCommittedCheckpoint(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t)
	ws := h.newWorkspace(t, ctx)
	task := port.Task{ID: "T1", Number: 1, HeadBranch: "feat/recover-1"}

	if _, err := ws.Existing(ctx, task, h.baseA); err == nil || !strings.Contains(err.Error(), "no existing worktree") {
		t.Fatalf("absent worktree: %v", err)
	}
	if _, err := os.Stat(filepath.Join(ws.root, slug(task.HeadBranch))); !os.IsNotExist(err) {
		t.Fatalf("recovery created a worktree: %v", err)
	}
	if out, err := gitTry(t, h.parent, "rev-parse", "--verify", "--quiet", "refs/heads/"+task.HeadBranch); err == nil {
		t.Fatalf("recovery created the task branch: %s", out)
	}
	if _, err := ws.Existing(ctx, port.Task{ID: "T1"}, h.baseA); err == nil {
		t.Fatal("task without a head branch reopened")
	}

	work, err := ws.Prepare(ctx, task)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(work.Dir(), "done.txt"), "done\n")
	if _, err := work.Checkpoint(ctx); err != nil {
		t.Fatal(err)
	}
	checkpoint := gitOK(t, work.Dir(), "rev-parse", "HEAD")

	// Mainline moves on while the checkpoint waits; the reopened worktree is
	// pinned to the fresh tip, exactly as Prepare pins it.
	writeFile(t, filepath.Join(h.origin, "f"), "B\n")
	gitOK(t, h.origin, "commit", "-q", "-am", "B")
	tip := gitOK(t, h.origin, "rev-parse", "HEAD")

	reopened, err := ws.Existing(ctx, task, checkpoint)
	if err != nil {
		t.Fatalf("exact checkpoint refused: %v", err)
	}
	if reopened.Dir() != work.Dir() || reopened.Branch() != task.HeadBranch || reopened.(*Workdir).base != tip {
		t.Fatalf("reopened %s on %s at base %s, want %s on %s at %s", reopened.Dir(), reopened.Branch(), reopened.(*Workdir).base, work.Dir(), task.HeadBranch, tip)
	}
	if head := gitOK(t, work.Dir(), "rev-parse", "HEAD"); head != checkpoint {
		t.Fatalf("reopening moved the checkpoint to %s", head)
	}

	if _, err := ws.Existing(ctx, task, h.baseA); err == nil || !strings.Contains(err.Error(), "not the expected checkpoint") {
		t.Fatalf("moved checkpoint accepted: %v", err)
	}
	writeFile(t, filepath.Join(work.Dir(), "untracked.txt"), "wip\n")
	if _, err := ws.Existing(ctx, task, checkpoint); err == nil || !strings.Contains(err.Error(), "uncommitted") {
		t.Fatalf("dirty checkout accepted: %v", err)
	}
	if err := os.Remove(filepath.Join(work.Dir(), "untracked.txt")); err != nil {
		t.Fatal(err)
	}
	gitOK(t, work.Dir(), "switch", "-q", "-c", "feat/elsewhere")
	if _, err := ws.Existing(ctx, task, checkpoint); err == nil || !strings.Contains(err.Error(), "no existing worktree") {
		t.Fatalf("worktree on another branch accepted: %v", err)
	}
}

func TestExistingDescendantPreservesOnlyCleanTaskCheckpoints(t *testing.T) {
	ctx := t.Context()
	h := newHarness(t)
	ws := h.newWorkspace(t, ctx)
	task := port.Task{ID: "T1", Number: 1, HeadBranch: "feat/resume-ci-1"}
	if _, err := ws.ExistingDescendant(ctx, task, h.baseA); err == nil {
		t.Fatal("continuation created a missing checkout")
	}
	if _, err := ws.ExistingDescendant(ctx, port.Task{ID: "T1"}, h.baseA); err == nil {
		t.Fatal("continuation accepted an empty branch")
	}
	work, err := ws.Prepare(ctx, task)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(work.Dir(), "done.txt"), "published implementation\n")
	if _, err := work.Checkpoint(ctx); err != nil {
		t.Fatal(err)
	}
	published := gitOK(t, work.Dir(), "rev-parse", "HEAD")
	writeFile(t, filepath.Join(work.Dir(), "repair.txt"), "unpublished repair\n")
	if _, err := work.Checkpoint(ctx); err != nil {
		t.Fatal(err)
	}
	checkpoint := gitOK(t, work.Dir(), "rev-parse", "HEAD")
	for _, anchor := range []string{published, checkpoint} {
		reopened, err := ws.ExistingDescendant(ctx, task, anchor)
		if err != nil || reopened.Dir() != work.Dir() || reopened.Branch() != task.HeadBranch {
			t.Fatalf("valid continuation from %s: %v", anchor, err)
		}
		if got := gitOK(t, reopened.Dir(), "rev-parse", "HEAD"); got != checkpoint {
			t.Fatalf("continuation changed the checkpoint to %s", got)
		}
	}
	if _, err := ws.Existing(ctx, task, published); err == nil {
		t.Fatal("continuation relaxed exact checkpoint recovery")
	}
	writeFile(t, filepath.Join(h.origin, "f"), "foreign branch progress\n")
	gitOK(t, h.origin, "commit", "-q", "-am", "foreign progress")
	foreign := gitOK(t, h.origin, "rev-parse", "HEAD")
	gitOK(t, work.Dir(), "fetch", "origin", "main")
	for _, anchor := range []string{foreign, strings.Repeat("f", 40), "--all"} {
		if _, err := ws.ExistingDescendant(ctx, task, anchor); err == nil || !strings.Contains(err.Error(), "does not contain published head") {
			t.Fatalf("unrelated/invalid anchor %q accepted: %v", anchor, err)
		}
	}
	writeFile(t, filepath.Join(work.Dir(), "untracked.txt"), "uncommitted repair\n")
	if _, err := ws.ExistingDescendant(ctx, task, published); err == nil || !strings.Contains(err.Error(), "uncommitted") {
		t.Fatalf("dirty continuation accepted: %v", err)
	}
	if err := os.Remove(filepath.Join(work.Dir(), "untracked.txt")); err != nil {
		t.Fatal(err)
	}
	gitOK(t, work.Dir(), "switch", "-q", "-c", "feat/foreign-ci")
	if _, err := ws.ExistingDescendant(ctx, task, published); err == nil || !strings.Contains(err.Error(), "no existing worktree") {
		t.Fatalf("foreign task branch accepted: %v", err)
	}
}
