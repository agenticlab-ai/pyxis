package converge_test

// T1 (existing-repo-onboarding/001; design existing-repo-onboarding-v2 §3.1,
// DINV-2): the engine-verbatim and generated writes are invocable for a
// caller-chosen path set from a classification (converge.Drift) plus the
// verified release archive (resolve.EnsurePayload) — RepairEngineVerbatim and
// WriteGenerated, called here directly, NEVER through converge.Run — the
// shape adopt will drive plan/apply from.

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/converge"
	"github.com/agenticlab-ai/pyxis/internal/relmanifest"
	"github.com/agenticlab-ai/pyxis/internal/resolve"
)

// verifiedArchive acquires the verified payload archive exactly as a caller
// independent of Run would: EnsureManifest then EnsurePayload — the same
// two calls Run itself makes, never Run.
func verifiedArchive(t *testing.T, ctx context.Context, cfg resolve.Config) string {
	t.Helper()
	m, err := resolve.EnsureManifest(ctx, cfg)
	if err != nil {
		t.Fatalf("EnsureManifest: %v", err)
	}
	archive, err := resolve.EnsurePayload(ctx, cfg, m)
	if err != nil {
		t.Fatalf("EnsurePayload: %v", err)
	}
	return archive
}

// findingAt returns the classification's finding for path, failing the test
// if the path carries no finding (i.e. it is not drifted).
func findingAt(t *testing.T, cls converge.Classification, path string) converge.Finding {
	t.Helper()
	for _, f := range cls.Findings {
		if f.Path == path {
			return f
		}
	}
	t.Fatalf("no finding for %s in %+v", path, cls.Findings)
	return converge.Finding{}
}

// ---- RepairEngineVerbatim: a chosen subset, independent of Run ---------------

func TestRepairEngineVerbatimActsOnlyOnChosenPathSet(t *testing.T) {
	r := newRelease(t, stdFiles())
	root, cfg := r.newOperand(t)
	ctx := context.Background()

	// Drift TWO engine-verbatim files; only one will be chosen.
	writeFile(t, at(root, ".pyxis/templates/prd-template.md"), []byte("tampered\n"), 0o644)
	if err := os.Remove(at(root, ".pyxis/scripts/hook.sh")); err != nil {
		t.Fatal(err)
	}

	archive := verifiedArchive(t, ctx, cfg)
	cls, err := converge.Drift(ctx, cfg)
	if err != nil {
		t.Fatalf("Drift: %v", err)
	}

	chosen := []converge.Finding{findingAt(t, cls, ".pyxis/templates/prd-template.md")}
	actions, refusals, err := converge.RepairEngineVerbatim(ctx, root, chosen, archive)
	if err != nil {
		t.Fatalf("RepairEngineVerbatim: %v", err)
	}
	if len(refusals) != 0 {
		t.Errorf("refusals = %v, want none", refusals)
	}
	want := []string{".pyxis/templates/prd-template.md"}
	if !slices.Equal(actionPaths(actions), want) {
		t.Fatalf("actions = %v, want %v", actionPaths(actions), want)
	}

	// The chosen path is repaired byte-exact — no Run call anywhere above.
	if got := mustRead(t, at(root, ".pyxis/templates/prd-template.md")); !bytes.Equal(got, r.find(t, ".pyxis/templates/prd-template.md").b) {
		t.Error("chosen path not repaired byte-exact")
	}
	// The UN-chosen drifted path (still missing) is untouched: the primitive
	// acts only on the given path set, not on every drifted verbatim file.
	if _, err := os.Stat(at(root, ".pyxis/scripts/hook.sh")); !errors.Is(err, fs.ErrNotExist) {
		t.Error("an un-chosen path was written — RepairEngineVerbatim must act only on the given findings")
	}
}

// A WantSHA256 that does not match the archive's actual bytes (a defective
// pinned release) is a named refusal, and nothing is written — fail closed,
// proven directly against the primitive (no Run, no Drift needed: the
// finding is hand-built to isolate the hash-mismatch leg).
func TestRepairEngineVerbatimRefusesOnHashMismatch(t *testing.T) {
	r := newRelease(t, stdFiles())
	root, cfg := r.newOperand(t)
	ctx := context.Background()
	if err := os.Remove(at(root, ".pyxis/scripts/hook.sh")); err != nil {
		t.Fatal(err)
	}
	archive := verifiedArchive(t, ctx, cfg)

	bogus := converge.Finding{
		Path: ".pyxis/scripts/hook.sh", Class: relmanifest.EngineVerbatim,
		State: converge.StateMissing, WantSHA256: strings.Repeat("de", 32),
	}
	actions, refusals, err := converge.RepairEngineVerbatim(ctx, root, []converge.Finding{bogus}, archive)
	if err != nil {
		t.Fatalf("RepairEngineVerbatim: %v", err)
	}
	if len(actions) != 0 {
		t.Errorf("actions = %v, want none (fail closed on a hash mismatch)", actions)
	}
	if len(refusals) != 1 || refusals[0].Path != ".pyxis/scripts/hook.sh" {
		t.Fatalf("refusals = %v, want exactly the mismatched path", refusals)
	}
	if !strings.Contains(refusals[0].Reason, "defective") {
		t.Errorf("refusal reason = %q, want the named defective-pin reason", refusals[0].Reason)
	}
	if _, err := os.Stat(at(root, ".pyxis/scripts/hook.sh")); !errors.Is(err, fs.ErrNotExist) {
		t.Error("a hash-mismatched file was written — fail closed violated")
	}
}

// A finding the archive carries no entry for at all is a named refusal.
func TestRepairEngineVerbatimRefusesUnknownArchivePath(t *testing.T) {
	r := newRelease(t, stdFiles())
	root, cfg := r.newOperand(t)
	ctx := context.Background()
	archive := verifiedArchive(t, ctx, cfg)

	phantom := converge.Finding{
		Path: "no/such/path.txt", Class: relmanifest.EngineVerbatim,
		State: converge.StateMissing, WantSHA256: strings.Repeat("00", 32),
	}
	actions, refusals, err := converge.RepairEngineVerbatim(ctx, root, []converge.Finding{phantom}, archive)
	if err != nil {
		t.Fatalf("RepairEngineVerbatim: %v", err)
	}
	if len(actions) != 0 {
		t.Errorf("actions = %v, want none", actions)
	}
	if len(refusals) != 1 || !strings.Contains(refusals[0].Reason, "carries no such file") {
		t.Fatalf("refusals = %+v, want the named carries-no-such-file reason", refusals)
	}
}

// Findings of another class are silently ignored — a caller may pass the
// full Classification.Findings and only the engine-verbatim subset acts.
func TestRepairEngineVerbatimIgnoresOtherClasses(t *testing.T) {
	r := newRelease(t, stdFiles())
	root, cfg := r.newOperand(t)
	ctx := context.Background()

	writeFile(t, at(root, ".pyxis/templates/prd-template.md"), []byte("tampered\n"), 0o644)
	if err := os.Remove(at(root, "docs/tech-design/index.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(at(root, ".pyxis/stamp/origin.json")); err != nil {
		t.Fatal(err)
	}
	archive := verifiedArchive(t, ctx, cfg)
	cls, err := converge.Drift(ctx, cfg)
	if err != nil {
		t.Fatalf("Drift: %v", err)
	}
	if len(cls.Findings) < 3 {
		t.Fatalf("fixture drift too thin: %+v", cls.Findings)
	}

	actions, refusals, err := converge.RepairEngineVerbatim(ctx, root, cls.Findings, archive)
	if err != nil {
		t.Fatalf("RepairEngineVerbatim: %v", err)
	}
	if len(refusals) != 0 {
		t.Errorf("refusals = %v, want none", refusals)
	}
	want := []string{".pyxis/templates/prd-template.md"}
	if !slices.Equal(actionPaths(actions), want) {
		t.Errorf("actions = %v, want exactly the engine-verbatim subset %v (generated/repo-owned findings must be ignored)", actionPaths(actions), want)
	}
}

// ---- WriteGenerated: a chosen subset, independent of Run ---------------------

func TestWriteGeneratedActsOnlyOnChosenPathSet(t *testing.T) {
	r := newRelease(t, stdFiles())
	root, cfg := r.newOperand(t)
	ctx := context.Background()

	// Drift BOTH generated files; only one will be chosen.
	writeFile(t, at(root, "docs/tech-design/index.json"), []byte("{\"stale\":true}\n"), 0o644)
	if err := os.Remove(at(root, "docs/tech-design/index.md")); err != nil {
		t.Fatal(err)
	}

	cls, err := converge.Drift(ctx, cfg)
	if err != nil {
		t.Fatalf("Drift: %v", err)
	}
	chosen := []converge.Finding{findingAt(t, cls, "docs/tech-design/index.json")}

	actions, refusals, err := converge.WriteGenerated(ctx, root, chosen, cls.Generated)
	if err != nil {
		t.Fatalf("WriteGenerated: %v", err)
	}
	if len(refusals) != 0 {
		t.Errorf("refusals = %v, want none", refusals)
	}
	want := []string{"docs/tech-design/index.json"}
	if !slices.Equal(actionPaths(actions), want) {
		t.Fatalf("actions = %v, want %v", actionPaths(actions), want)
	}
	if got := mustRead(t, at(root, "docs/tech-design/index.json")); !bytes.Equal(got, cls.Generated["docs/tech-design/index.json"]) {
		t.Error("chosen generated path not written to the classification's desired bytes")
	}
	// The un-chosen drifted path (still missing) is untouched.
	if _, err := os.Stat(at(root, "docs/tech-design/index.md")); !errors.Is(err, fs.ErrNotExist) {
		t.Error("an un-chosen path was written — WriteGenerated must act only on the given findings")
	}
}

// A StateUngeneratable finding is a named refusal, never a write — the
// generator's own refusal reason carried straight through (mirrors
// TestGeneratedRefusalsUngeneratableAndUnregistered, but drives the write
// primitive directly rather than through Run).
func TestWriteGeneratedRefusesUngeneratable(t *testing.T) {
	r := newRelease(t, stdFiles())
	root, cfg := r.newOperand(t)
	ctx := context.Background()

	// A broken design makes the index corpus ungeneratable.
	writeFile(t, at(root, "docs/tech-design/broken.md"), []byte("not a design\n"), 0o644)
	if err := os.Remove(at(root, "docs/tech-design/index.md")); err != nil {
		t.Fatal(err)
	}

	cls, err := converge.Drift(ctx, cfg)
	if err != nil {
		t.Fatalf("Drift: %v", err)
	}
	var ungeneratable []converge.Finding
	for _, f := range cls.Findings {
		if f.State == converge.StateUngeneratable {
			ungeneratable = append(ungeneratable, f)
		}
	}
	if len(ungeneratable) == 0 {
		t.Fatalf("fixture did not produce an ungeneratable finding: %+v", cls.Findings)
	}

	actions, refusals, err := converge.WriteGenerated(ctx, root, ungeneratable, cls.Generated)
	if err != nil {
		t.Fatalf("WriteGenerated: %v", err)
	}
	if len(actions) != 0 {
		t.Errorf("actions = %v, want none", actions)
	}
	if len(refusals) != len(ungeneratable) {
		t.Fatalf("refusals = %v, want one per ungeneratable finding", refusals)
	}
	for _, ref := range refusals {
		if !strings.Contains(ref.Reason, "generator refuses the design corpus") {
			t.Errorf("refusal reason = %q, want the named generator-refusal reason", ref.Reason)
		}
	}
	if _, err := os.Stat(at(root, "docs/tech-design/index.md")); !errors.Is(err, fs.ErrNotExist) {
		t.Error("an ungeneratable file was written")
	}
}

// Findings of another class are silently ignored — mirrors
// TestRepairEngineVerbatimIgnoresOtherClasses for the generated primitive.
func TestWriteGeneratedIgnoresOtherClasses(t *testing.T) {
	r := newRelease(t, stdFiles())
	root, cfg := r.newOperand(t)
	ctx := context.Background()

	writeFile(t, at(root, ".pyxis/templates/prd-template.md"), []byte("tampered\n"), 0o644)
	if err := os.Remove(at(root, "docs/tech-design/index.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(at(root, ".pyxis/stamp/origin.json")); err != nil {
		t.Fatal(err)
	}
	cls, err := converge.Drift(ctx, cfg)
	if err != nil {
		t.Fatalf("Drift: %v", err)
	}

	actions, refusals, err := converge.WriteGenerated(ctx, root, cls.Findings, cls.Generated)
	if err != nil {
		t.Fatalf("WriteGenerated: %v", err)
	}
	if len(refusals) != 0 {
		t.Errorf("refusals = %v, want none", refusals)
	}
	want := []string{"docs/tech-design/index.md"}
	if !slices.Equal(actionPaths(actions), want) {
		t.Errorf("actions = %v, want exactly the generated subset %v (engine-verbatim/repo-owned findings must be ignored)", actionPaths(actions), want)
	}
}
