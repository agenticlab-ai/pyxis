package converge_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/converge"
	"github.com/agenticlab-ai/pyxis/internal/githooks"
)

func TestDoctorRecognizesComposedHooksAndFindsMissingPrePush(t *testing.T) {
	isolateGit(t)
	r := newRelease(t, stdFiles())
	root, cfg := r.newOperand(t)
	cfg.Repo.System.Mode = "source"
	gitInitRepo(t, root)
	if err := os.MkdirAll(filepath.Join(root, ".githooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := githooks.Install(t.Context(), root); err != nil {
		t.Fatal(err)
	}
	h := mustDoctor(t, t.Context(), cfg)
	if rows := doctorRows(h, converge.CheckHooks); len(rows) != 0 {
		t.Fatalf("valid composed hooks reported inactive: %+v", rows)
	}
	out, err := exec.Command("git", "-C", root, "config", "core.hooksPath").Output()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(strings.TrimSpace(string(out)), "pre-push")); err != nil {
		t.Fatal(err)
	}
	h = mustDoctor(t, t.Context(), cfg)
	wantRow(t, h, converge.CheckHooks, "core.hooksPath", converge.SeverityFinding, "pre-push", "hooks-setup")
}
