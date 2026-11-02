package converge_test

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/converge"
	"github.com/agenticlab-ai/pyxis/internal/payload"
	"github.com/agenticlab-ai/pyxis/internal/relmanifest"
)

func shippedHarnessTemplate(t *testing.T) []byte {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(payload.HarnessImplementNowSkill)))
	if err != nil {
		t.Fatalf("read harness template: %v", err)
	}
	return b
}

func harnessFiles(t *testing.T) []pf {
	return append(stdFiles(),
		pf{path: payload.HarnessImplementNowSkill, b: shippedHarnessTemplate(t), class: relmanifest.EngineVerbatim},
		pf{path: ".pyxis/skills/help/SKILL.md", b: []byte("---\nname: help\n---\n"), class: relmanifest.EngineVerbatim},
		pf{path: ".pyxis/skills/implement-now/SKILL.md", b: []byte("---\nname: implement-now\n---\n"), class: relmanifest.EngineVerbatim},
	)
}

func TestConvergeMigratesAndRepairsHarnessViews(t *testing.T) {
	isolateGit(t)
	r := newRelease(t, harnessFiles(t))
	root, cfg := r.newOperand(t)
	cfg.Repo.Knobs.CLISurfaces = []string{"codex", "opencode"}

	// Simulate a pre-feature operand: both discovery paths expose the entire
	// canonical tree and no generated harness views exist.
	for _, rel := range []string{".agents/skills", ".codex/skills"} {
		path := at(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("../.pyxis/skills", path); err != nil {
			t.Fatal(err)
		}
	}

	rep, err := converge.Run(context.Background(), cfg, converge.Options{})
	if err != nil {
		t.Fatalf("upgrade converge: %v", err)
	}
	if got, err := os.Readlink(at(root, ".agents/skills")); err != nil || got != "../.pyxis/generated/harnesses/neutral/skills" {
		t.Fatalf("migrated .agents/skills = %q, %v", got, err)
	}
	if got, err := os.Readlink(at(root, ".codex/skills")); err != nil || got != "../.pyxis/generated/harnesses/codex/skills" {
		t.Fatalf("migrated .codex/skills = %q, %v", got, err)
	}
	if !slices.Contains(actionPaths(rep.Regenerated), ".pyxis/generated/harnesses/codex/skills/implement-now/SKILL.md") {
		t.Fatalf("upgrade did not report generated Codex skill: %v", actionPaths(rep.Regenerated))
	}

	impl := at(root, ".pyxis/generated/harnesses/codex/skills/implement-now/SKILL.md")
	writeFile(t, impl, []byte("stale\n"), 0o644)
	if defects := converge.HarnessSurfaceDefects(root, cfg.Repo); len(defects) == 0 || !strings.Contains(strings.Join(defects, "\n"), "missing or stale") {
		t.Fatalf("generated-view drift was not detected: %v", defects)
	}
	health := mustDoctor(t, context.Background(), cfg)
	wantRow(t, health, converge.CheckSurfaces, "generated-skills", converge.SeverityFinding, "missing or stale")

	if _, err := converge.Run(context.Background(), cfg, converge.Options{}); err != nil {
		t.Fatalf("repair converge: %v", err)
	}
	if defects := converge.HarnessSurfaceDefects(root, cfg.Repo); len(defects) != 0 {
		t.Fatalf("repair left harness defects: %v", defects)
	}
}
