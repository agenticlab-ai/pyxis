package workspace

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

func TestIntegrationRefusesInvalidWorkspaceWithoutChangingCheckpoint(t *testing.T) {
	plain := &Workdir{base: "pinned"}
	if base, err := plain.RefreshMainline(t.Context()); err != nil || base != "pinned" {
		t.Fatalf("direct workdir base: %q %v", base, err)
	}
	if err := plain.ValidateIntegration(t.Context()); err != nil {
		t.Fatal(err)
	}
	h := newHarness(t)
	ws := h.newWorkspace(t, t.Context())
	work, err := ws.Prepare(t.Context(), port.Task{ID: "T1", HeadBranch: "feat/invalid-integration"})
	if err != nil {
		t.Fatal(err)
	}
	d := work.(*Workdir)
	head := gitOK(t, d.Dir(), "rev-parse", "HEAD")
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := d.RefreshMainline(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled refresh: %v", err)
	}
	d.branch = "feat/another-owner"
	if _, err := d.RefreshMainline(t.Context()); err == nil || !strings.Contains(err.Error(), "expected") {
		t.Fatalf("foreign branch accepted: %v", err)
	}
	d.branch = gitOK(t, d.Dir(), "symbolic-ref", "--short", "HEAD")
	gitOK(t, d.Dir(), "checkout", "--detach", "-q", head)
	if _, err := d.RefreshMainline(t.Context()); err == nil {
		t.Fatal("detached integration accepted")
	}
	gitOK(t, d.Dir(), "checkout", "-q", d.branch)
	ws.remote = "missing-origin"
	if _, err := d.RefreshMainline(t.Context()); err == nil {
		t.Fatal("failed base fetch certified integration")
	}
	if err := d.ValidateIntegration(t.Context()); err == nil {
		t.Fatal("failed publication base read certified evidence")
	}
	if gitOK(t, d.Dir(), "rev-parse", "HEAD") != head || gitOK(t, d.Dir(), "status", "--porcelain") != "" {
		t.Fatal("refused integration changed checkpoint")
	}
}

func TestIntegrationAdvancesBasePreservesCheckpointAndRefusesStalePublication(t *testing.T) {
	h := newHarness(t)
	ws := h.newWorkspace(t, t.Context())
	work, err := ws.Prepare(t.Context(), port.Task{ID: "T1", HeadBranch: "feat/integration"})
	if err != nil {
		t.Fatal(err)
	}
	d := work.(*Workdir)
	writeFile(t, filepath.Join(d.Dir(), "task"), "checkpoint\n")
	if _, err := d.Checkpoint(t.Context()); err != nil {
		t.Fatal(err)
	}
	checkpoint := gitOK(t, d.Dir(), "rev-parse", "HEAD")
	advance := func(name string) string {
		writeFile(t, filepath.Join(h.origin, name), "mainline\n")
		gitOK(t, h.origin, "add", name)
		gitOK(t, h.origin, "commit", "-qm", name)
		return gitOK(t, h.origin, "rev-parse", "HEAD")
	}
	base := advance("first")
	if got, err := d.RefreshMainline(t.Context()); err != nil || got != base {
		t.Fatalf("refresh = %q, %v", got, err)
	}
	gitOK(t, d.Dir(), "merge-base", "--is-ancestor", checkpoint, "HEAD")
	gitOK(t, d.Dir(), "merge-base", "--is-ancestor", base, "HEAD")
	if content, err := os.ReadFile(filepath.Join(d.Dir(), "task")); err != nil || string(content) != "checkpoint\n" {
		t.Fatalf("checkpoint lost: %q, %v", content, err)
	}
	head := gitOK(t, d.Dir(), "rev-parse", "HEAD")
	if got, err := d.RefreshMainline(t.Context()); err != nil || got != base || gitOK(t, d.Dir(), "rev-parse", "HEAD") != head {
		t.Fatal("unchanged mainline created another checkpoint")
	}
	newBase := advance("second")
	if published, err := d.Push(t.Context()); err == nil || published || !strings.Contains(err.Error(), base) || !strings.Contains(err.Error(), newBase) {
		t.Fatalf("published stale evidence: %v, %v", published, err)
	}
	if got := gitOK(t, d.Dir(), "rev-parse", "HEAD"); got != head {
		t.Fatal("publication changed the reviewed head")
	}
	if got, err := d.RefreshMainline(t.Context()); err != nil || got != newBase {
		t.Fatalf("second refresh = %q, %v", got, err)
	}
	if err := d.ValidateIntegration(t.Context()); err != nil {
		t.Fatal(err)
	}
	if published, err := d.Push(t.Context()); err != nil || !published {
		t.Fatalf("fresh verified publication = %v, %v", published, err)
	}
	if got := gitOK(t, h.parent, "rev-parse", "HEAD"); got != h.baseA {
		t.Fatal("integration mutated parent checkout")
	}
}

func TestIntegrationConflictRestoresCleanCheckpointAndReportsPins(t *testing.T) {
	h := newHarness(t)
	ws := h.newWorkspace(t, t.Context())
	work, err := ws.Prepare(t.Context(), port.Task{ID: "T1", HeadBranch: "feat/conflict"})
	if err != nil {
		t.Fatal(err)
	}
	d := work.(*Workdir)
	writeFile(t, filepath.Join(d.Dir(), "f"), "task\n")
	if _, err := d.Checkpoint(t.Context()); err != nil {
		t.Fatal(err)
	}
	head := gitOK(t, d.Dir(), "rev-parse", "HEAD")
	writeFile(t, filepath.Join(h.origin, "f"), "mainline\n")
	gitOK(t, h.origin, "add", "f")
	gitOK(t, h.origin, "commit", "-qm", "conflicting mainline")
	base := gitOK(t, h.origin, "rev-parse", "HEAD")
	if _, err := d.RefreshMainline(t.Context()); err == nil || !strings.Contains(err.Error(), head) || !strings.Contains(err.Error(), base) {
		t.Fatalf("integration conflict lost pins: %v", err)
	}
	if gitOK(t, d.Dir(), "rev-parse", "HEAD") != head || gitOK(t, d.Dir(), "status", "--porcelain") != "" {
		t.Fatal("failed integration damaged the clean checkpoint")
	}
	if _, err := gitTry(t, d.Dir(), "rev-parse", "--verify", "MERGE_HEAD"); err == nil {
		t.Fatal("integration left an unfinished merge")
	}
	writeFile(t, filepath.Join(d.Dir(), "dirty"), "uncheckpointed")
	if _, err := d.RefreshMainline(t.Context()); err == nil || !strings.Contains(err.Error(), "uncheckpointed") {
		t.Fatalf("dirty integration admitted: %v", err)
	}
}
