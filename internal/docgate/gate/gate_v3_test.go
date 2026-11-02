package gate_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/gate"
)

// fixtureRepo returns a temp git repo plus run/write helpers (the house
// temp-repo pattern; unit-hermetic tests elsewhere deliberately skip git).
func fixtureRepo(t *testing.T) (string, func(...string), func(string, string)) {
	t.Helper()
	root := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	write := func(rel, content string) {
		t.Helper()
		p := filepath.Join(root, rel)
		os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run("init", "-q", "-b", "main")
	return root, run, write
}

func TestLintGoverning(t *testing.T) {
	root, _, write := fixtureRepo(t)
	// Relative link in the vision → red (FR-18 purity).
	write("docs/product-vision.md", "# V\n\nsee [the prds](prds/README.md) for detail\n")
	wantDefect(t, gate.LintGoverning(root, config.Defaults()), "relative link")
	// Requirement-ID citation → red.
	write("docs/product-vision.md", "# V\n\nas VNEXT-3 demands\n")
	wantDefect(t, gate.LintGoverning(root, config.Defaults()), "requirement-ID citation")
	// External links, fragments, fenced blocks, code spans → green.
	write("docs/product-vision.md", "# V\n\n[site](https://example.com) and [top](#north-star)\n\n```\n[x](docs/foo.md) GATE-1\n```\n\nand `GATE-1 in a span` is fine\n")
	write("docs/architecture-principles.md", "# P\n\n**AP-1 — x.** The registry starts at I1.\n")
	if d := gate.LintGoverning(root, config.Defaults()); len(d) != 0 {
		t.Fatalf("clean governing docs flagged: %v", d)
	}
}

func TestVisionAppendOnly(t *testing.T) {
	root, run, write := fixtureRepo(t)
	write("docs/product-vision.md", "# V\n\n## 1. North star\n\nbody\n")
	run("add", ".")
	run("commit", "-qm", "base")
	// Trailered APPEND to the vision → green (FR-18).
	write("docs/product-vision.md", "# V\n\n## 1. North star\n\nbody\n\n## 2. More\n\nappended\n")
	run("add", ".")
	run("commit", "-qm", "append\n\nGoverning-Change: operator")
	if d := gate.CheckGoverning(root, "HEAD~1", "", config.Defaults()); len(d) != 0 {
		t.Fatalf("trailered vision append flagged: %v", d)
	}
	// Trailered LINE-EDIT of the vision → red even with the trailer.
	write("docs/product-vision.md", "# V\n\n## 1. North star REWRITTEN\n\nbody\n\n## 2. More\n\nappended\n")
	run("add", ".")
	run("commit", "-qm", "edit\n\nGoverning-Change: operator")
	wantDefect(t, gate.CheckGoverning(root, "HEAD~1", "", config.Defaults()), "append-only even with the trailer")
}

// TestGoverningPathsAreConfigDriven proves the check-governing trust root
// resolves from the configured governing-doc paths, not baked-in literals
// (config-driven-portability-v1 §3.2, FR-1/FR-3; DINV-1). Under a NON-DEFAULT
// config the purity lint and the append-only history check both fire on the
// RELOCATED vision/principles and IGNORE the stale default locations; the
// default arms elsewhere in this file prove byte-identity (DINV-2).
func TestGoverningPathsAreConfigDriven(t *testing.T) {
	cfg := config.Defaults()
	cfg.ProductVision = "governance/vision.md"
	cfg.ArchitecturePrinciples = "governance/principles.md"

	t.Run("purity lint follows the configured location", func(t *testing.T) {
		root, _, write := fixtureRepo(t)
		// A stale-location vision with an impurity is NOT checked — the engine
		// looks only where config points.
		write("docs/product-vision.md", "# V\n\nas VNEXT-3 demands\n")
		if d := gate.LintGoverning(root, cfg); len(d) != 0 {
			t.Fatalf("lint checked the stale default location: %v", d)
		}
		// The SAME impurity at the configured location IS caught.
		write("governance/vision.md", "# V\n\nas VNEXT-3 demands\n")
		wantDefect(t, gate.LintGoverning(root, cfg), "requirement-ID citation")
	})

	t.Run("history check follows the configured location", func(t *testing.T) {
		root, run, write := fixtureRepo(t)
		write("governance/vision.md", "# V\n\n## 1. North star\n\nbody\n")
		write("governance/principles.md", "**AP-1 — x.** body\n")
		run("add", ".")
		run("commit", "-qm", "base")
		// An untrailered LINE-EDIT of the relocated principles → red.
		write("governance/principles.md", "**AP-1 — REWRITTEN.** body\n")
		run("add", ".")
		run("commit", "-qm", "sneaky edit")
		wantDefect(t, gate.CheckGoverning(root, "HEAD~1", "", cfg), "without the `Governing-Change: operator` trailer")
	})

	t.Run("stale default location is not the trust root under a relocated config", func(t *testing.T) {
		root, run, write := fixtureRepo(t)
		write("governance/vision.md", "# V\n\nbody\n")
		write("governance/principles.md", "**AP-1 — x.** body\n")
		run("add", ".")
		run("commit", "-qm", "base")
		// Editing a file at the OLD default path is untracked by the check now —
		// it is not a governing document under this config.
		write("docs/product-vision.md", "# stale\n\nedited without a trailer\n")
		run("add", ".")
		run("commit", "-qm", "edit the stale-location file")
		if d := gate.CheckGoverning(root, "HEAD~1", "", cfg); len(d) != 0 {
			t.Fatalf("the stale default location was treated as the trust root: %v", d)
		}
	})
}

const baseConfig = `system:
  version: "0.1.0"
docgate:
  version: "source"
grandfathered_designs:
  - old-one
precedence_grandfathered:
  - docgate-v2
capability_paths:
  - cmd/
`

func TestCheckConfigPolicy(t *testing.T) {
	root, run, write := fixtureRepo(t)
	write(".pyxis/config.yaml", baseConfig)
	run("add", ".")
	run("commit", "-qm", "base")
	// Untrailered self-exemption → red (FR-23; the self-unlock fix).
	write(".pyxis/config.yaml", baseConfig+"  - internal/\n")
	run("add", ".")
	run("commit", "-qm", "widen capability paths")
	wantDefect(t, gate.CheckConfigPolicy(root, "HEAD~1"), "capability_paths")
	run("reset", "-q", "--hard", "HEAD~1")
	sneaky := baseConfig[:len(baseConfig)-len("capability_paths:\n  - cmd/\n")] + "capability_paths:\n  - cmd/\n"
	_ = sneaky
	write(".pyxis/config.yaml", "system:\n  version: \"0.1.0\"\ndocgate:\n  version: \"source\"\ngrandfathered_designs:\n  - old-one\n  - my-new-design\nprecedence_grandfathered:\n  - docgate-v2\ncapability_paths:\n  - cmd/\n")
	run("add", ".")
	run("commit", "-qm", "exempt myself")
	wantDefect(t, gate.CheckConfigPolicy(root, "HEAD~1"), "grandfathered_designs")
	// The same change, trailered → green.
	run("commit", "-q", "--amend", "-m", "exempt with ruling\n\nGoverning-Change: operator")
	if d := gate.CheckConfigPolicy(root, "HEAD~1"); len(d) != 0 {
		t.Fatalf("trailered policy change flagged: %v", d)
	}
	// Non-policy keys change untrailered → green.
	write(".pyxis/config.yaml", "system:\n  version: \"0.2.0\"\ndocgate:\n  version: \"source\"\ngrandfathered_designs:\n  - old-one\n  - my-new-design\nprecedence_grandfathered:\n  - docgate-v2\ncapability_paths:\n  - cmd/\n")
	run("add", ".")
	run("commit", "-qm", "bump version")
	if d := gate.CheckConfigPolicy(root, "HEAD~1"); len(d) != 0 {
		t.Fatalf("non-policy key change flagged: %v", d)
	}
}

func TestCheckMaps(t *testing.T) {
	root, run, write := fixtureRepo(t)
	cfg := config.Defaults()
	mapDoc := "# DM\n\n| # | Invariant | Lands as |\n|---|---|---|\n| INV-1 | one | A |\n| INV-2 | two | E |\n\n| # | Decision | Leaning | Blocks |\n|---|---|---|---|\n| D1 | engine state | x | y |\n"
	write("docs/domain-model.md", mapDoc)
	run("add", ".")
	run("commit", "-qm", "base")
	// Removing a registry row → red (FR-22).
	write("docs/domain-model.md", "# DM\n\n| # | Invariant | Lands as |\n|---|---|---|\n| INV-1 | one | A |\n\n| # | Decision | Leaning | Blocks |\n|---|---|---|---|\n| D1 | engine state | x | y |\n")
	wantDefect(t, gate.CheckMaps(root, "HEAD", cfg), "INV-2")
	// Renumbering (INV-2 → INV-3) → red: the old anchor is gone.
	write("docs/domain-model.md", "# DM\n\n| # | Invariant | Lands as |\n|---|---|---|\n| INV-1 | one | A |\n| INV-3 | two | E |\n\n| # | Decision | Leaning | Blocks |\n|---|---|---|---|\n| D1 | engine state | x | y |\n")
	wantDefect(t, gate.CheckMaps(root, "HEAD", cfg), "INV-2")
	// Appending a fresh row + editing prose/content → green.
	write("docs/domain-model.md", "# DM\n\nnew prose\n\n| # | Invariant | Lands as |\n|---|---|---|\n| INV-1 | one, reworded | A |\n| INV-2 | two | E |\n| INV-3 | three | D |\n\n| # | Decision | Leaning | Blocks |\n|---|---|---|---|\n| D1 | engine state | x | y |\n| D2 | config | x | y |\n")
	if d := gate.CheckMaps(root, "HEAD", cfg); len(d) != 0 {
		t.Fatalf("append+edit flagged: %v", d)
	}
	// A map absent at the base (system-design.md not merged yet) → skipped.
	if d := gate.CheckMaps(root, "HEAD", cfg); len(d) != 0 {
		t.Fatalf("absent-map case flagged: %v", d)
	}
	// Deleting the whole map → every anchor red (maximal removal).
	os.Remove(filepath.Join(root, "docs/domain-model.md"))
	wantDefect(t, gate.CheckMaps(root, "HEAD", cfg), "INV-1")
}

func TestCheckLinks(t *testing.T) {
	root, _, write := fixtureRepo(t)
	cfg := config.Defaults()
	write("docs/real.md", "# R\n")
	// Dangling relative link → red naming file, line, target (FR-21).
	write("docs/broken.md", "# B\n\nsee [gone](missing/file.md)\n")
	wantDefect(t, gate.CheckLinks(root, cfg, gate.ScanScope{}), "missing/file.md")
	// Resolving, external, fragment, fenced, code-span → green.
	write("docs/broken.md", "# B\n\nsee [ok](real.md#part) and [ext](https://x.y) and [top](#here)\n\n```\n[fenced](nope.md)\n```\n\nand `[span](nope.md)`\n")
	if d := gate.CheckLinks(root, cfg, gate.ScanScope{}); len(d) != 0 {
		t.Fatalf("clean links flagged: %v", d)
	}
	// Reference-style definition with a dangling target → red.
	write("docs/broken.md", "# B\n\n[ref]: nowhere.md\n")
	wantDefect(t, gate.CheckLinks(root, cfg, gate.ScanScope{}), "nowhere.md")
}
