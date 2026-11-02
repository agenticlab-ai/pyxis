package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

func TestCheckpointLiteralPathsDeletionAndProvenance(t *testing.T) {
	h := newHarness(t)
	w, err := h.newWorkspace(t, t.Context()).Prepare(t.Context(), port.Task{ID: "T7", SourceRef: "plan alpha/001, issue #7", HeadBranch: "feat/literal-paths"})
	if err != nil {
		t.Fatal(err)
	}
	files := []string{" leading space.txt", ":(glob)*", "line\nbreak.txt", "-option.txt"}
	for _, name := range files {
		writeFile(t, filepath.Join(w.Dir(), name), "implementation\n")
	}
	if err := os.Remove(filepath.Join(w.Dir(), "f")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(w.Dir(), ".gitignore"), "ignored-output\n")
	writeFile(t, filepath.Join(w.Dir(), "ignored-output"), "generated\n")
	if hasWork, err := w.Checkpoint(t.Context()); err != nil || !hasWork {
		t.Fatalf("checkpoint = %v, %v", hasWork, err)
	}
	for _, name := range files {
		if got := gitOK(t, w.Dir(), "show", "HEAD:"+name); strings.TrimSpace(got) != "implementation" {
			t.Fatalf("literal filename %q was not committed: %q", name, got)
		}
	}
	for _, name := range []string{"f", "ignored-output"} {
		if _, err := gitTry(t, w.Dir(), "cat-file", "-e", "HEAD:"+name); err == nil {
			t.Fatalf("unexpected committed file %q", name)
		}
	}
	message := gitOK(t, w.Dir(), "show", "-s", "--format=%B", "HEAD")
	for _, want := range []string{"Task: T7", "Source: plan alpha/001, issue #7", "Branch: feat/literal-paths"} {
		if !strings.Contains(message, want) {
			t.Fatalf("commit lacks %q: %s", want, message)
		}
	}
}

func TestCheckpointRefusesUnexpectedBranchWithoutCommitting(t *testing.T) {
	h := newHarness(t)
	w, err := h.newWorkspace(t, t.Context()).Prepare(t.Context(), port.Task{ID: "T8", HeadBranch: "feat/expected"})
	if err != nil {
		t.Fatal(err)
	}
	gitOK(t, w.Dir(), "switch", "-c", "unexpected")
	writeFile(t, filepath.Join(w.Dir(), "uncommitted"), "preserve\n")
	before := gitOK(t, w.Dir(), "rev-parse", "HEAD")
	if _, err := w.Checkpoint(t.Context()); err == nil {
		t.Fatal("checkpoint accepted another branch")
	}
	if after := gitOK(t, w.Dir(), "rev-parse", "HEAD"); after != before {
		t.Fatal("checkpoint committed on the wrong branch")
	}
}

func TestCheckpointLockedIndexPreservesWorkForRetry(t *testing.T) {
	h := newHarness(t)
	w, err := h.newWorkspace(t, t.Context()).Prepare(t.Context(), port.Task{ID: "T9", HeadBranch: "feat/locked-index"})
	if err != nil {
		t.Fatal(err)
	}
	before := gitOK(t, w.Dir(), "rev-parse", "HEAD")
	writeFile(t, filepath.Join(w.Dir(), "pending"), "preserved\n")
	lock := gitOK(t, w.Dir(), "rev-parse", "--path-format=absolute", "--git-path", "index.lock")
	writeFile(t, lock, "another Git operation owns the index\n")
	if _, err := w.Checkpoint(t.Context()); err == nil {
		t.Fatal("checkpoint ignored the index lock")
	}
	if after := gitOK(t, w.Dir(), "rev-parse", "HEAD"); after != before {
		t.Fatal("failed checkpoint advanced HEAD")
	}
	if err := os.Remove(lock); err != nil {
		t.Fatal(err)
	}
	if hasWork, err := w.Checkpoint(t.Context()); err != nil || !hasWork {
		t.Fatalf("checkpoint retry: hasWork=%v err=%v", hasWork, err)
	}
	if got := gitOK(t, w.Dir(), "show", "HEAD:pending"); got != "preserved" {
		t.Fatalf("retry lost pending edits: %q", got)
	}
}
