package converge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/payload"
)

func harnessBoundaryConfig(surfaces ...string) config.Config {
	return config.Config{Knobs: config.Knobs{CLISurfaces: surfaces}}
}

func TestHarnessSurfacesReportUnreadableInputs(t *testing.T) {
	cfg := harnessBoundaryConfig("codex")

	t.Run("template is a directory", func(t *testing.T) {
		root := t.TempDir()
		if err := os.MkdirAll(treePath(root, payload.HarnessImplementNowSkill), 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := ReconcileHarnessSurfaces(root, cfg); err == nil {
			t.Fatal("ReconcileHarnessSurfaces accepted an unreadable template")
		}
		if defects := HarnessSurfaceDefects(root, cfg); len(defects) == 0 || !strings.Contains(defects[0], "unreadable") {
			t.Fatalf("template defects = %v", defects)
		}
	})

	t.Run("canonical skill tree is absent", func(t *testing.T) {
		root := t.TempDir()
		template := treePath(root, payload.HarnessImplementNowSkill)
		if err := os.MkdirAll(filepath.Dir(template), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(template, []byte("{{harness_name}} {{backend_id}}"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := ReconcileHarnessSurfaces(root, cfg); err == nil {
			t.Fatal("ReconcileHarnessSurfaces accepted a missing canonical tree")
		}
		if defects := HarnessSurfaceDefects(root, cfg); len(defects) == 0 || !strings.Contains(defects[0], "canonical skill tree") {
			t.Fatalf("canonical-tree defects = %v", defects)
		}
	})
}

func TestHarnessSymlinkAndStaleEntrySafetyBoundaries(t *testing.T) {
	root := t.TempDir()
	regular := filepath.Join(root, "regular")
	if err := os.WriteFile(regular, []byte("operand"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := reconcileSymlink(regular, "target"); err == nil {
		t.Fatal("reconcileSymlink overwrote a regular file")
	}
	if _, err := reconcileSymlink(filepath.Join(regular, "child"), "target"); err == nil {
		t.Fatal("reconcileSymlink accepted a file as its parent directory")
	}
	if err := removeStaleHarnessEntries(filepath.Join(root, "missing"), "missing", map[string]bool{}, new([]Action)); err == nil {
		t.Fatal("removeStaleHarnessEntries accepted a missing directory")
	}

	dir := filepath.Join(root, "view")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "operand"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	var actions []Action
	if err := removeStaleHarnessEntries(dir, "view", map[string]bool{}, &actions); err == nil {
		t.Fatal("removeStaleHarnessEntries removed operand-owned content")
	}
}

func TestRemoveStaleHarnessEntriesRemovesOwnedSymlink(t *testing.T) {
	root := t.TempDir()
	view := filepath.Join(root, "view")
	if err := os.Mkdir(view, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../canonical", filepath.Join(view, "stale")); err != nil {
		t.Fatal(err)
	}
	var actions []Action
	if err := removeStaleHarnessEntries(view, "view", map[string]bool{}, &actions); err != nil {
		t.Fatal(err)
	}
	if len(actions) != 1 || actions[0].Path != "view/stale" {
		t.Fatalf("actions = %+v", actions)
	}
}

func TestHarnessSurfaceDefectEmptyAndNeutralCases(t *testing.T) {
	if defects := HarnessSurfaceDefects(t.TempDir(), config.Config{}); defects != nil {
		t.Fatalf("empty surface selection defects = %v", defects)
	}

	root := t.TempDir()
	template := treePath(root, payload.HarnessImplementNowSkill)
	if err := os.MkdirAll(filepath.Dir(template), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(template, []byte("{{harness_name}} {{backend_id}}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(treePath(root, ".pyxis/skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	neutralImpl := treePath(root, filepath.ToSlash(filepath.Join(harnessViewSkillsPath("neutral"), "implement-now")))
	if err := os.MkdirAll(neutralImpl, 0o755); err != nil {
		t.Fatal(err)
	}
	defects := HarnessSurfaceDefects(root, harnessBoundaryConfig("unknown"))
	if len(defects) != 1 || !strings.Contains(defects[0], "neutral views") {
		t.Fatalf("neutral defects = %v", defects)
	}
	if got := joinHarnessDefects([]string{"one", "two"}); got != "one; two" {
		t.Fatalf("joinHarnessDefects = %q", got)
	}
}
