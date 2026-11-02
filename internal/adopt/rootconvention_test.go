package adopt_test

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/adopt"
)

func hashFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// TestResolveRootConventionExistingFileNeverWritten proves T7 acceptance
// #1: an existing AGENTS.md is byte-identical after ResolveRootConvention
// runs — the resolution IS the whole write-side story for this class, since
// no later step ever touches a collide-manual path (design §3.3) — and the
// returned entry carries the exact lines to add.
func TestResolveRootConventionExistingFileNeverWritten(t *testing.T) {
	root := t.TempDir()
	agentsPath := filepath.Join(root, "AGENTS.md")
	if err := os.WriteFile(agentsPath, []byte("# My own conventions\n\nDo the thing.\n"), 0o644); err != nil {
		t.Fatalf("write fixture AGENTS.md: %v", err)
	}
	before := hashFile(t, agentsPath)

	entry, ok, err := adopt.ResolveRootConvention(root)
	if err != nil {
		t.Fatalf("ResolveRootConvention: %v", err)
	}
	if !ok {
		t.Fatal("ok = false, want true for an existing AGENTS.md")
	}
	if entry.Path != "AGENTS.md" {
		t.Errorf("Path = %q, want AGENTS.md", entry.Path)
	}
	if entry.Class != adopt.ClassCollideManual {
		t.Errorf("Class = %s, want %s", entry.Class, adopt.ClassCollideManual)
	}
	if entry.Detail == "" {
		t.Fatal("Detail is empty, want the exact lines to add")
	}
	if !strings.Contains(entry.Detail, "pyxis doctor") {
		t.Errorf("Detail = %q, want it to name `pyxis doctor`", entry.Detail)
	}

	if after := hashFile(t, agentsPath); after != before {
		t.Error("AGENTS.md changed — the resolution must never write to an existing root convention file")
	}
}

// TestResolveRootConventionAbsentFileNoEntry proves a tree with no prior
// AGENTS.md yields no collision entry: this resolution's whole concern is
// FR-6's collision case (existing-repo-onboarding/001 T7's scope), never a
// fresh-authoring path.
func TestResolveRootConventionAbsentFileNoEntry(t *testing.T) {
	root := t.TempDir()

	entry, ok, err := adopt.ResolveRootConvention(root)
	if err != nil {
		t.Fatalf("ResolveRootConvention: %v", err)
	}
	if ok {
		t.Fatalf("ok = true, want false for a tree with no AGENTS.md (got %+v)", entry)
	}
	if entry != (adopt.Entry{}) {
		t.Errorf("Entry = %+v, want the zero value when ok is false", entry)
	}
}

// TestResolveRootConventionRefusesOnStatError proves a stat failure other
// than "absent" is a named refusal, never silently treated as "no
// collision" — routing root through a path segment that is a regular file
// (never a directory) forces ENOTDIR rather than ENOENT.
func TestResolveRootConventionRefusesOnStatError(t *testing.T) {
	dir := t.TempDir()
	notADir := filepath.Join(dir, "not-a-directory")
	if err := os.WriteFile(notADir, []byte("x"), 0o644); err != nil {
		t.Fatalf("write fixture file: %v", err)
	}

	_, ok, err := adopt.ResolveRootConvention(notADir)
	if err == nil {
		t.Fatal("err = nil, want a named refusal when root cannot be probed")
	}
	if ok {
		t.Error("ok = true, want false on a refusal")
	}
}

// TestResolveRootConventionEntryRendersAsFencedBlock proves the
// collide-manual entry's multi-line Detail renders through Plan.Render as
// the indented fenced block render.go documents for this shape (design
// §3.3), so a reviewer sees the exact lines verbatim.
func TestResolveRootConventionEntryRendersAsFencedBlock(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("# existing\n"), 0o644); err != nil {
		t.Fatalf("write fixture AGENTS.md: %v", err)
	}
	entry, ok, err := adopt.ResolveRootConvention(root)
	if err != nil || !ok {
		t.Fatalf("ResolveRootConvention: ok=%v err=%v", ok, err)
	}

	p := adopt.Plan{SchemaVersion: adopt.SchemaVersion, Status: adopt.StatusDraft, Entries: []adopt.Entry{entry}}
	rendered := p.Render()
	if !strings.Contains(rendered, "`AGENTS.md` — collide-manual") {
		t.Errorf("render missing the entry line; got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "```") {
		t.Errorf("render missing the fenced block for a multi-line Detail; got:\n%s", rendered)
	}
	for _, line := range strings.Split(entry.Detail, "\n") {
		if !strings.Contains(rendered, line) {
			t.Errorf("render missing detail line %q; got:\n%s", line, rendered)
		}
	}
}

// TestPlanManualInstructionsFiltersCollideManualOnly proves undo's naming
// seam returns exactly the collide-manual entries, in the plan's own order,
// ignoring every other class (T7 acceptance #2; design §3.5,
// prd:existing-repo-onboarding/FR-9).
func TestPlanManualInstructionsFiltersCollideManualOnly(t *testing.T) {
	p := adopt.Plan{
		SchemaVersion: adopt.SchemaVersion,
		Status:        adopt.StatusDraft,
		Entries: []adopt.Entry{
			{Path: ".pyxis/config.yaml", Class: adopt.ClassAdd},
			{Path: "AGENTS.md", Class: adopt.ClassCollideManual, Detail: "add this"},
			{Path: "docs/CONTRIBUTING.md", Class: adopt.ClassCollideManual, Detail: "add that"},
			{Path: "src/**", Class: adopt.ClassNeverTouch},
		},
	}

	got := p.ManualInstructions()
	if len(got) != 2 {
		t.Fatalf("ManualInstructions() = %+v, want exactly the 2 collide-manual entries", got)
	}
	if got[0].Path != "AGENTS.md" || got[1].Path != "docs/CONTRIBUTING.md" {
		t.Errorf("ManualInstructions() = %+v, want AGENTS.md then docs/CONTRIBUTING.md (plan order)", got)
	}
}

// TestPlanManualInstructionsEmptyWhenNone proves a plan with no
// collide-manual entries yields none, never an error.
func TestPlanManualInstructionsEmptyWhenNone(t *testing.T) {
	p := adopt.Plan{
		SchemaVersion: adopt.SchemaVersion,
		Status:        adopt.StatusDraft,
		Entries:       []adopt.Entry{{Path: "src/**", Class: adopt.ClassNeverTouch}},
	}
	if got := p.ManualInstructions(); len(got) != 0 {
		t.Errorf("ManualInstructions() = %+v, want none", got)
	}
}
