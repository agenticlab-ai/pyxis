package converge

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/payload"
	"github.com/agenticlab-ai/pyxis/internal/relmanifest"
)

type harnessSkillBinding struct {
	name    string
	backend string
}

var harnessSkillBindings = map[string]harnessSkillBinding{
	"claude":   {name: "Claude Code", backend: "claude"},
	"codex":    {name: "Codex", backend: "codex"},
	"cursor":   {name: "Cursor CLI", backend: "cursor"},
	"opencode": {name: "OpenCode", backend: "opencode"},
}

// ReconcileHarnessSurfaces materializes the selected harness skill views and
// their vendor pointers. It is shared by init and converge so release upgrades
// re-render implement-now from the current template and migrate old pointers.
func ReconcileHarnessSurfaces(root string, repo config.Config) ([]Action, error) {
	if len(repo.Knobs.CLISurfaces) == 0 {
		return nil, nil
	}
	tmpl, err := os.ReadFile(treePath(root, payload.HarnessImplementNowSkill))
	if err != nil {
		if os.IsNotExist(err) {
			// Releases predating harness-specific views carry no template. Their
			// converge remains valid; pinning a newer release materializes the
			// template first and activates migration in this same pass.
			return nil, nil
		}
		return nil, fmt.Errorf("surfaces: read harness skill template: %w", err)
	}
	entries, err := os.ReadDir(treePath(root, ".pyxis/skills"))
	if err != nil {
		return nil, fmt.Errorf("surfaces: read canonical skills: %w", err)
	}

	views := selectedHarnessViews(repo.Knobs.CLISurfaces)
	var actions []Action
	for _, view := range views {
		viewRel := harnessViewSkillsPath(view)
		viewDir := treePath(root, viewRel)
		if err := os.MkdirAll(viewDir, 0o755); err != nil {
			return actions, fmt.Errorf("surfaces: create %s: %w", viewRel, err)
		}
		desired := map[string]bool{}
		for _, entry := range entries {
			if !entry.IsDir() || entry.Name() == "implement-now" {
				continue
			}
			desired[entry.Name()] = true
			linkRel := filepath.ToSlash(filepath.Join(viewRel, entry.Name()))
			target := filepath.ToSlash(filepath.Join("..", "..", "..", "..", "skills", entry.Name()))
			changed, err := reconcileSymlink(treePath(root, linkRel), target)
			if err != nil {
				return actions, fmt.Errorf("surfaces: neutral skill %s: %w", linkRel, err)
			}
			if changed {
				actions = append(actions, Action{Path: linkRel, Class: relmanifest.Generated})
			}
		}
		if view != "neutral" {
			desired["implement-now"] = true
			binding := harnessSkillBindings[view]
			rendered, err := payload.Substitute(tmpl, map[string]string{"harness_name": binding.name, "backend_id": binding.backend})
			if err != nil {
				return actions, fmt.Errorf("surfaces: render %s implement-now: %w", view, err)
			}
			skillRel := filepath.ToSlash(filepath.Join(viewRel, "implement-now", "SKILL.md"))
			current, readErr := os.ReadFile(treePath(root, skillRel))
			if readErr != nil || !bytes.Equal(current, rendered) {
				if err := writeTreeAtomic(treePath(root, skillRel), rendered, 0o644); err != nil {
					return actions, fmt.Errorf("surfaces: write %s: %w", skillRel, err)
				}
				actions = append(actions, Action{Path: skillRel, Class: relmanifest.Generated})
			}
		}
		if err := removeStaleHarnessEntries(viewDir, viewRel, desired, &actions); err != nil {
			return actions, err
		}
	}

	seen := map[string]bool{}
	for _, surface := range repo.Knobs.CLISurfaces {
		for _, pointer := range SurfacePointers(repo)[surface] {
			if seen[pointer.Link] {
				continue
			}
			seen[pointer.Link] = true
			changed, err := reconcileSymlink(treePath(root, pointer.Link), pointer.Target)
			if err != nil {
				return actions, fmt.Errorf("surfaces: wire %s → %s: %w", pointer.Link, pointer.Target, err)
			}
			if changed {
				actions = append(actions, Action{Path: pointer.Link, Class: relmanifest.Generated})
			}
		}
	}
	return actions, nil
}

func selectedHarnessViews(surfaces []string) []string {
	views := []string{"neutral"}
	seen := map[string]bool{"neutral": true}
	for _, surface := range surfaces {
		if _, known := harnessSkillBindings[surface]; known && !seen[surface] {
			views = append(views, surface)
			seen[surface] = true
		}
	}
	return views
}

func harnessViewSkillsPath(view string) string {
	return filepath.ToSlash(filepath.Join(".pyxis", "generated", "harnesses", view, "skills"))
}

func reconcileSymlink(link, target string) (bool, error) {
	if current, err := os.Readlink(link); err == nil {
		if filepath.ToSlash(current) == target {
			return false, nil
		}
		if err := os.Remove(link); err != nil {
			return false, err
		}
	} else if _, statErr := os.Lstat(link); statErr == nil {
		return false, fmt.Errorf("exists and is not a symlink — refusing to overwrite operand content")
	} else if !os.IsNotExist(statErr) {
		return false, statErr
	}
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		return false, err
	}
	return true, os.Symlink(target, link)
}

func removeStaleHarnessEntries(dir, rel string, desired map[string]bool, actions *[]Action) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("surfaces: inspect %s: %w", rel, err)
	}
	for _, entry := range entries {
		if desired[entry.Name()] {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if info.Mode()&fs.ModeSymlink == 0 {
			return fmt.Errorf("surfaces: stale %s exists and is not a symlink — refusing to remove operand content", filepath.ToSlash(filepath.Join(rel, entry.Name())))
		}
		if err := os.Remove(path); err != nil {
			return err
		}
		*actions = append(*actions, Action{Path: filepath.ToSlash(filepath.Join(rel, entry.Name())), Class: relmanifest.Generated})
	}
	return nil
}

// HarnessSurfaceDefects verifies the generated view contents, not merely the
// outer vendor pointer. Returned strings are deterministic and user-facing.
func HarnessSurfaceDefects(root string, repo config.Config) []string {
	if len(repo.Knobs.CLISurfaces) == 0 {
		return nil
	}
	tmpl, err := os.ReadFile(treePath(root, payload.HarnessImplementNowSkill))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return []string{fmt.Sprintf("harness skill template %s is unreadable: %v", payload.HarnessImplementNowSkill, err)}
	}
	canonical, err := os.ReadDir(treePath(root, ".pyxis/skills"))
	if err != nil {
		return []string{fmt.Sprintf("canonical skill tree .pyxis/skills is unreadable: %v", err)}
	}
	var defects []string
	for _, view := range selectedHarnessViews(repo.Knobs.CLISurfaces) {
		viewRel := harnessViewSkillsPath(view)
		for _, entry := range canonical {
			if !entry.IsDir() || entry.Name() == "implement-now" {
				continue
			}
			link := treePath(root, filepath.ToSlash(filepath.Join(viewRel, entry.Name())))
			want := filepath.ToSlash(filepath.Join("..", "..", "..", "..", "skills", entry.Name()))
			got, err := os.Readlink(link)
			if err != nil || filepath.ToSlash(got) != want {
				defects = append(defects, fmt.Sprintf("%s/%s is not the canonical skill symlink %q", viewRel, entry.Name(), want))
			}
		}
		impl := treePath(root, filepath.ToSlash(filepath.Join(viewRel, "implement-now", "SKILL.md")))
		if view == "neutral" {
			if _, err := os.Lstat(filepath.Dir(impl)); !os.IsNotExist(err) {
				defects = append(defects, fmt.Sprintf("%s exposes implement-now but neutral views must not", viewRel))
			}
			continue
		}
		binding := harnessSkillBindings[view]
		want, subErr := payload.Substitute(tmpl, map[string]string{"harness_name": binding.name, "backend_id": binding.backend})
		got, readErr := os.ReadFile(impl)
		if subErr != nil || readErr != nil || !bytes.Equal(got, want) {
			defects = append(defects, fmt.Sprintf("%s/implement-now/SKILL.md is missing or stale for backend %q", viewRel, binding.backend))
		}
	}
	slices.Sort(defects)
	return defects
}

func joinHarnessDefects(defects []string) string { return strings.Join(defects, "; ") }
