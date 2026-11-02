package gate_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/gate"
)

// FR-31: content cites version pins, never system-footprint paths.
func TestCheckContentPaths(t *testing.T) {
	root, run, write := fixtureRepo(t)
	cfg := config.Defaults()

	// A wiki entry with a footprint path → red naming file and line.
	write("docs/wiki/entry.md", "# E\n\nsee `.pyxis/templates/x.md` for the shape\n")
	wantDefect(t, gate.CheckContentPaths(root, "", "", cfg), "system-footprint path reference")
	// Version-pin citation → green.
	write("docs/wiki/entry.md", "# E\n\ninstantiated from `prd-template/v1` (surface in AGENTS.md)\n")
	if d := gate.CheckContentPaths(root, "", "", cfg); len(d) != 0 {
		t.Fatalf("pin citation flagged: %v", d)
	}
	// The wiki README (rules file, not an entry) is out of scope.
	write("docs/wiki/README.md", "# rules\n\nthe surface lives under `.pyxis/` per AGENTS.md\n")
	if d := gate.CheckContentPaths(root, "", "", cfg); len(d) != 0 {
		t.Fatalf("wiki README flagged: %v", d)
	}

	// A flat corpus file (README included) with a footprint path → red.
	write("docs/prds/README.md", "# index\n\ntemplates live at `.pyxis/templates/`\n")
	wantDefect(t, gate.CheckContentPaths(root, "", "", cfg), "system-footprint path reference")
	write("docs/prds/README.md", "# index\n\ninstantiate `prd-template/v1`\n")

	// Frozen history: a ledger entry present at the merge base → green even
	// with a footprint path (never retro-edited); the same entry ADDED since
	// the merge base → red.
	write("docs/prds/feat/000-base.md", "# F — PRD\n\n| Field | Value |\n|---|---|\n| **Template version** | prd-template/v1 |\n\nsee `.pyxis/bin/docgate`\n")
	run("add", ".")
	run("commit", "-qm", "base state")
	if d := gate.CheckContentPaths(root, "HEAD", "", cfg); len(d) != 0 {
		t.Fatalf("pre-existing entry retro-flagged: %v", d)
	}
	write("docs/prds/feat/001-x.md", "# Amendment 001 — x\n\nrun `.pyxis/scripts/foo.sh`\n")
	run("add", ".")
	run("commit", "-qm", "add amendment")
	wantDefect(t, gate.CheckContentPaths(root, "HEAD~1", "", cfg), "system-footprint path reference")

	// A design added since the merge base with a footprint path → red;
	// the generated index is out of scope.
	os.Remove(filepath.Join(root, "docs/prds/feat/001-x.md"))
	write("docs/tech-design/new-design.md", "# D\n\nwires `.pyxis/hooks/pre-push`\n")
	write("docs/tech-design/index.md", "# generated\n\n`.pyxis/` appears here legitimately\n")
	run("add", ".")
	run("commit", "-qm", "add design")
	d := gate.CheckContentPaths(root, "HEAD~1", "", cfg)
	foundDesign := false
	for _, def := range d {
		if def.File == "docs/tech-design/new-design.md" {
			foundDesign = true
		}
		if def.File == "docs/tech-design/index.md" {
			t.Fatalf("generated index flagged: %v", def)
		}
	}
	if !foundDesign {
		t.Fatalf("added design not flagged: %v", d)
	}

	// Rollups and the root manual are out of scope entirely.
	write("docs/techspec.md", "# spec\n\ndesk shims live in `.pyxis/hooks/`\n")
	write("AGENTS.md", "# manual\n\nthe surface is `.pyxis/skills/`\n")
	got := gate.CheckContentPaths(root, "", "", cfg)
	for _, def := range got {
		if def.File == "docs/techspec.md" || def.File == "AGENTS.md" {
			t.Fatalf("out-of-scope file flagged: %v", def)
		}
	}
}
