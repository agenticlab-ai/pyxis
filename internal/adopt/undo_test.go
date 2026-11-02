package adopt_test

// Hermetic tests for Undo's reversal (existing-repo-onboarding/001 T10;
// prd:existing-repo-onboarding/FR-9, UB-6; design §3.5, DINV-5, DINV-7, A-7):
// real adopt→apply→undo round trips over the archive-serving fixture, asserting
// the three T10 acceptance criteria — the tree returns byte-identical to its
// pre-adoption state and a post-adoption commit survives; an operator-edited
// artifact is preserved and named while the rest is removed; and the plan's
// collide-manual entries are named for the human to reverse — plus the
// no-applied-plan refusal. Reuses apply_test.go / inspect_test.go scaffolding.

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/adopt"
)

// applyFixture drafts and applies a plan over a fresh adopter repo, returning
// the repo root and the release — the common preamble every undo test shares.
func applyFixture(t *testing.T) (string, release) {
	t.Helper()
	r := newReleaseWithArchive(t, adoptFiles())
	root := nonEmptyAdopterRepo(t)
	draftPlan(t, root, r)
	if _, err := adopt.Apply(context.Background(), applyOpts(root, r)); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	return root, r
}

// ---- FR-9 / A-7: round trip restores pre-adoption state; commits survive ----

func TestUndoRoundTripRestoresPreAdoptionState(t *testing.T) {
	r := newReleaseWithArchive(t, adoptFiles())
	root := nonEmptyAdopterRepo(t)
	pre := snapshot(t, root)
	preHead := strings.TrimSpace(git(t, root, "rev-parse", "HEAD"))

	draftPlan(t, root, r)
	if _, err := adopt.Apply(context.Background(), applyOpts(root, r)); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if mapsEqual(pre, snapshot(t, root)) {
		t.Fatal("apply changed nothing — the fixture is wrong")
	}

	rep, err := adopt.Undo(context.Background(), applyOpts(root, r))
	if err != nil {
		t.Fatalf("Undo: %v", err)
	}

	// A-7: the whole tree is byte-identical to its pre-adoption state.
	if post := snapshot(t, root); !mapsEqual(pre, post) {
		t.Fatalf("undo did not restore pre-adoption state:\npre  %v\npost %v", pre, post)
	}
	// Undo creates no commit (HEAD unchanged) and unsets the hook activation.
	if h := strings.TrimSpace(git(t, root, "rev-parse", "HEAD")); h != preHead {
		t.Errorf("HEAD changed across undo: %s → %s", preHead, h)
	}
	if got, set := gitGet(t, root, "core.hooksPath"); set {
		t.Errorf("core.hooksPath = %q still set after undo; want unset", got)
	}
	if len(rep.Preserved) != 0 {
		t.Errorf("nothing was edited, want no preserved paths, got %v", rep.Preserved)
	}
	if !rep.Unchained {
		t.Error("Report.Unchained = false; want true (undo reversed the hook activation)")
	}
}

func TestUndoPreservesPostAdoptionCommit(t *testing.T) {
	root, r := applyFixture(t)

	// The operator commits their OWN post-adoption work — NOT the adoption's
	// uncommitted footprint.
	writeFile(t, filepath.Join(root, "feature.go"), []byte("package main\n\n// my work\n"))
	git(t, root, "add", "feature.go")
	git(t, root, "commit", "-q", "-m", "post-adoption work")
	workHead := strings.TrimSpace(git(t, root, "rev-parse", "HEAD"))

	if _, err := adopt.Undo(context.Background(), applyOpts(root, r)); err != nil {
		t.Fatalf("Undo: %v", err)
	}

	// The operator's commit and file survive undo untouched.
	if h := strings.TrimSpace(git(t, root, "rev-parse", "HEAD")); h != workHead {
		t.Errorf("undo destroyed the post-adoption commit: HEAD %s, want %s", h, workHead)
	}
	if _, err := os.Stat(filepath.Join(root, "feature.go")); err != nil {
		t.Errorf("undo removed the operator's post-adoption file: %v", err)
	}
	// Adoption's own footprint is gone.
	for _, gone := range []string{".pyxis/config.yaml", ".pyxis/templates/prd-template.md", adopt.PlanPath} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(gone))); !os.IsNotExist(err) {
			t.Errorf("undo did not remove adoption path %q (err=%v)", gone, err)
		}
	}
}

// ---- UB-6 / DINV-5: an operator-edited artifact is preserved and named ------

func TestUndoPreservesOperatorEditedArtifact(t *testing.T) {
	root, r := applyFixture(t)

	// The operator edits an engine-verbatim artifact after adoption (its hash
	// now diverges from the manifest SHA — the per-class "modified" signal).
	edited := ".pyxis/templates/prd-template.md"
	const myEdit = "# my own edits to a pyxis file\n"
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(edited)), []byte(myEdit), 0o644); err != nil {
		t.Fatal(err)
	}

	rep, err := adopt.Undo(context.Background(), applyOpts(root, r))
	if err != nil {
		t.Fatalf("Undo: %v", err)
	}

	// DINV-5: preserved and named, never deleted.
	if !slices.Contains(rep.Preserved, edited) {
		t.Errorf("edited artifact not named as preserved: %v", rep.Preserved)
	}
	if b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(edited))); err != nil || string(b) != myEdit {
		t.Errorf("edited artifact not preserved intact: got %q err=%v", b, err)
	}
	// The rest of the installation is still removed — a pristine generated path.
	if _, err := os.Stat(filepath.Join(root, "docs", "tech-design", "index.md")); !os.IsNotExist(err) {
		t.Error("undo did not remove the pristine generated index alongside the preserved edit")
	}
}

// ---- FR-9 / PRD §4.4: undo names collide-manual entries for reversal --------

func TestUndoNamesCollideManualEntries(t *testing.T) {
	root, r := applyFixture(t)

	// Inject a collide-manual entry into the applied plan — the AGENTS.md
	// collision T7 detects — simulating a plan that carried one (the baseline
	// inspect does not yet wire it in; T10 tests undo's handling of it).
	planAbs := filepath.Join(root, filepath.FromSlash(adopt.PlanPath))
	b, err := os.ReadFile(planAbs)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := adopt.Parse(b)
	if err != nil {
		t.Fatalf("parse applied plan: %v", err)
	}
	plan.Entries = append(plan.Entries, adopt.Entry{
		Path:   "AGENTS.md",
		Class:  adopt.ClassCollideManual,
		Detail: "- Pyxis: run `pyxis converge` before pushing.",
	})
	out, err := plan.Encode()
	if err != nil {
		t.Fatalf("re-encode plan with a collide-manual entry: %v", err)
	}
	writeFile(t, planAbs, out)

	rep, err := adopt.Undo(context.Background(), applyOpts(root, r))
	if err != nil {
		t.Fatalf("Undo: %v", err)
	}
	if len(rep.ManualEntries) != 1 || rep.ManualEntries[0].Path != "AGENTS.md" {
		t.Fatalf("collide-manual entry not named for reversal: %v", rep.ManualEntries)
	}
	if rep.ManualEntries[0].Detail == "" {
		t.Error("named collide-manual entry carries no detail for the human to act on")
	}
}

// ---- refusal: undo needs an applied plan ------------------------------------

func TestUndoRefusesWithoutAppliedPlan(t *testing.T) {
	r := newReleaseWithArchive(t, adoptFiles())

	// No plan at all.
	root := nonEmptyAdopterRepo(t)
	if _, err := adopt.Undo(context.Background(), applyOpts(root, r)); !errors.Is(err, adopt.ErrNoAppliedPlan) {
		t.Fatalf("undo with no plan: err = %v, want ErrNoAppliedPlan", err)
	}

	// A draft plan (inspected, never applied) is not undoable.
	draftPlan(t, root, r)
	if _, err := adopt.Undo(context.Background(), applyOpts(root, r)); !errors.Is(err, adopt.ErrNoAppliedPlan) {
		t.Fatalf("undo with a draft plan: err = %v, want ErrNoAppliedPlan", err)
	}
}
