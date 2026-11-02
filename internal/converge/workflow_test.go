package converge_test

// T4 — the installed CI workflow as a GENERATED surface converge renders from
// the repo config (config-driven-portability-v1 §3.1, FR-4/FR-2/UB-3/NFR-2;
// #DINV-4, #A-5, #A-6). The four acceptance legs:
//
//   AC-1 — a non-default config renders a workflow carrying no branch/remote
//          literal absent from config (the render substitutes them).
//   AC-2 — a config change re-renders the workflow on the next converge.
//   AC-3 — a hand-edited generated workflow is named as drift by doctor and
//          repaired by converge.
//   AC-4 — a default config renders the CURRENT workflow byte-identical (the
//          A-5 render golden; DINV-2 non-regression).
//
// The fixture classes .github/workflows/docs-gate.yml GENERATED and carries the
// REAL shipped template at .pyxis/dist/github-workflows/docs-gate.yml
// (engine-verbatim, the single source — DINV-4), so the render is proven
// against the actual deliverable, not a stand-in.

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/converge"
	"github.com/agenticlab-ai/pyxis/internal/payload"
	"github.com/agenticlab-ai/pyxis/internal/relmanifest"
	"github.com/agenticlab-ai/pyxis/internal/resolve"
)

// shippedTemplate reads the REAL installed-workflow template from the source
// repo (this package's deliverable is the render machinery; the template is a
// sibling deliverable, so its absence is a loud failure, never a skip).
func shippedTemplate(t *testing.T) []byte {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(payload.InstalledWorkflow)))
	if err != nil {
		t.Fatalf("read shipped workflow template %s: %v", payload.InstalledWorkflow, err)
	}
	return b
}

// renderWith renders the shipped template for newRelease's fixture manifest and
// cfg's artifact source — the test's independent oracle for the expected render.
func renderWith(t *testing.T, tmpl []byte, cfg resolve.Config, branch, remote string) []byte {
	t.Helper()
	sum := hexOf([]byte("#!/bin/sh\necho docgate fixture\n"))
	out, err := payload.Substitute(tmpl, map[string]string{
		"branch": branch, "remote": remote,
		"version": "v0.4.0", "linux_arm_checksum": sum, "linux_amd_checksum": sum,
		"artifact_source": cfg.Repo.System.ArtifactSource,
	})
	if err != nil {
		t.Fatalf("render workflow (branch=%s remote=%s): %v", branch, remote, err)
	}
	return out
}

// workflowFiles is the payload set for the T4 legs: the standard shapes with
// the installed workflow reclassed GENERATED, plus the engine-verbatim overlay
// SOURCE (the template converge renders from) present in the operand tree.
func workflowFiles(t *testing.T) []pf {
	files := []pf{
		{path: ".pyxis/config.yaml", b: []byte("# packed source config; never extracted\n"), class: relmanifest.RepoOwned},
		{path: ".pyxis/templates/prd-template.md", b: []byte("# PRD template\nbody\n"), class: relmanifest.EngineVerbatim},
		{path: payload.InstalledWorkflow, b: shippedTemplate(t), class: relmanifest.EngineVerbatim},
		{path: payload.InstalledWorkflowTarget, b: nil, class: relmanifest.Generated},
		{path: "docs/tech-design/index.md", b: []byte("stale packed index\n"), class: relmanifest.Generated},
		{path: "docs/tech-design/index.json", b: []byte("{}\n"), class: relmanifest.Generated},
	}
	return files
}

// operandWithWorkflow builds a PRISTINE converged operand from workflowFiles,
// then materializes the generated installed workflow at branch/remote — the
// render newOperand skips (it writes only the index generator's output). The
// returned config carries the given branch/remote so the operand config and the
// tree agree, and a ci_mode that INCLUDES hosted CI (§3.8) — this file's own
// concern is the render/drift/repair mechanics (T4), not the knob-conditional
// desired set (T6, cimode_test.go's concern), so every leg here exercises the
// included path unchanged.
func operandWithWorkflow(t *testing.T, branch, remote string) (string, resolve.Config) {
	t.Helper()
	r := newRelease(t, workflowFiles(t))
	root, cfg := r.newOperand(t)
	// The operand's config sets branch/remote; converge reads them from here.
	cfg.Repo.Branch, cfg.Repo.Remote = branch, remote
	cfg.Repo.Knobs.CIMode = "both"
	// Materialize the pristine generated workflow to the render for this config.
	writeFile(t, at(root, payload.InstalledWorkflowTarget),
		renderWith(t, shippedTemplate(t), cfg, branch, remote), 0o644)
	return root, cfg
}

// ---- AC-4: default config renders the current workflow byte-identical --------

// The A-5 render golden: under the default config (branch main, remote origin),
// the render equals the pre-config-driven static workflow byte-for-byte
// (DINV-2). The golden is the shipped template rendered with the defaults — the
// SAME bytes the former engine-verbatim file carried, proven here to survive a
// clean converge unchanged.
func TestWorkflowDefaultRenderByteIdentical(t *testing.T) {
	root, cfg := operandWithWorkflow(t, "main", "origin")
	ctx := context.Background()

	// The pristine tree is already converged: a run is a noop, the workflow
	// untouched and byte-identical to the default render.
	rep, err := converge.Run(ctx, cfg, converge.Options{})
	if err != nil {
		t.Fatalf("Run on the pristine tree: %v", err)
	}
	if rep.Mode != converge.ModeNoop {
		t.Errorf("mode = %q, want noop (a pristine default-rendered tree converges to nothing)", rep.Mode)
	}
	if len(rep.Regenerated) != 0 {
		t.Errorf("Regenerated = %v, want none — the default render is already in the tree", actionPaths(rep.Regenerated))
	}

	golden := renderWith(t, shippedTemplate(t), cfg, "main", "origin")
	got := mustRead(t, at(root, payload.InstalledWorkflowTarget))
	if !bytes.Equal(got, golden) {
		t.Fatalf("default-rendered workflow is not byte-identical to the A-5 golden\n--- got ---\n%s\n--- want ---\n%s", got, golden)
	}
	// The default render carries the original functional literals (the former
	// static file's contract) and no substitution remnant.
	for _, want := range []string{"branches: [main]", "git fetch origin main", "--merge-base origin/main --defects-json -"} {
		if !strings.Contains(string(got), want) {
			t.Errorf("default render missing the original literal %q", want)
		}
	}
	if strings.Contains(string(got), "{{") {
		t.Errorf("default render retains a substitution token")
	}
}

// ---- AC-1: a non-default config renders with no stale branch/remote literal --

// The rendered workflow carries the CONFIGURED branch/remote everywhere the
// former literals sat, and NO `main`/`origin` literal that is not the
// substituted value survives (config-driven-portability-v1 #DINV-1, A-5).
func TestWorkflowNonDefaultRenderCarriesNoStaleLiteral(t *testing.T) {
	const branch, remote = "trunk", "upstream"
	root, cfg := operandWithWorkflow(t, branch, remote)
	ctx := context.Background()

	// A converged non-default tree is a noop; the workflow is the non-default
	// render.
	if _, err := converge.Run(ctx, cfg, converge.Options{}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	got := string(mustRead(t, at(root, payload.InstalledWorkflowTarget)))

	// Every former-literal site now carries the configured value.
	for _, want := range []string{
		"branches: [" + branch + "]",
		"git fetch " + remote + " " + branch,
		"--merge-base " + remote + "/" + branch + " --defects-json -",
		"against " + remote + "/" + branch, // the desk-parity comment too
	} {
		if !strings.Contains(got, want) {
			t.Errorf("non-default render missing %q", want)
		}
	}
	// No stale default literal remains (neither the branch nor the remote): a
	// missed literal would coincidentally match the default and hide (§0 R-context).
	for _, stale := range []string{"main", "origin"} {
		if strings.Contains(got, stale) {
			t.Errorf("non-default render still carries the stale default literal %q — a target-facing literal was missed", stale)
		}
	}
	if strings.Contains(got, "{{") {
		t.Errorf("non-default render retains a substitution token")
	}
}

// ---- AC-2: a config change re-renders on the next converge -------------------

// A repo whose config branch/remote change gets its installed workflow
// re-rendered on the next converge: the stale (old-config) render is StateStale
// drift, and converge rewrites it to the new-config render (FR-2, DINV-4).
func TestWorkflowConfigChangeRerenders(t *testing.T) {
	// Start converged at the default config.
	root, cfg := operandWithWorkflow(t, "main", "origin")
	ctx := context.Background()
	if _, err := converge.Run(ctx, cfg, converge.Options{}); err != nil {
		t.Fatalf("warming Run: %v", err)
	}

	// The operator changes the config's branch/remote. The tree's workflow now
	// renders stale against the NEW config — converge must re-render it.
	cfg.Repo.Branch, cfg.Repo.Remote = "release", "fork"

	rep, err := converge.Run(ctx, cfg, converge.Options{})
	if err != nil {
		t.Fatalf("re-render Run: %v", err)
	}
	if want := []string{payload.InstalledWorkflowTarget}; !slices.Equal(actionPaths(rep.Regenerated), want) {
		t.Fatalf("Regenerated = %v, want the re-rendered workflow %v", actionPaths(rep.Regenerated), want)
	}
	if !slices.Contains(rep.DriftClasses, "generated") {
		t.Errorf("DriftClasses = %v, want it to include generated", rep.DriftClasses)
	}

	// The workflow now matches the NEW-config render exactly.
	want := renderWith(t, shippedTemplate(t), cfg, "release", "fork")
	if got := mustRead(t, at(root, payload.InstalledWorkflowTarget)); !bytes.Equal(got, want) {
		t.Fatalf("workflow not re-rendered to the new config\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}

	// A second converge under the same (now-current) config is a noop — the
	// re-render is idempotent.
	rep2, err := converge.Run(ctx, cfg, converge.Options{})
	if err != nil {
		t.Fatalf("idempotency Run: %v", err)
	}
	if rep2.Mode != converge.ModeNoop {
		t.Errorf("second run mode = %q, want noop (re-render is idempotent)", rep2.Mode)
	}
}

// ---- AC-3: a hand-edited generated workflow is drift, and converge repairs ---

// A hand-edit of the generated workflow is StateStale, named by doctor's drift
// check (config-driven-portability-v1 UB-3, A-6) and repaired by converge back
// to the config render.
func TestWorkflowHandEditIsDriftAndRepaired(t *testing.T) {
	root, cfg := operandWithWorkflow(t, "main", "origin")
	ctx := context.Background()

	// Warm the manifest cache so the read-only faces are pure reads.
	if _, err := resolve.EnsureManifest(ctx, cfg); err != nil {
		t.Fatalf("EnsureManifest: %v", err)
	}

	// A human hand-edits the generated workflow (e.g. flips the branch by hand
	// instead of via config — exactly the drift the class change prevents).
	handEdited := []byte("name: docs-gate\non:\n  push:\n    branches: [hand-edited]\n")
	writeFile(t, at(root, payload.InstalledWorkflowTarget), handEdited, 0o644)

	// (a) Drift names it StateStale (the shared predicate doctor consumes).
	cls, err := converge.Drift(ctx, cfg)
	if err != nil {
		t.Fatalf("Drift: %v", err)
	}
	findings := cls.Findings
	var found *converge.Finding
	for i := range findings {
		if findings[i].Path == payload.InstalledWorkflowTarget {
			found = &findings[i]
		}
	}
	if found == nil {
		t.Fatalf("Drift did not name the hand-edited workflow; findings = %+v", findings)
	}
	if found.State != converge.StateStale || found.Class != relmanifest.Generated {
		t.Errorf("workflow finding = {class %q, state %q}, want {generated, stale}", found.Class, found.State)
	}

	// (b) doctor surfaces the same as a CheckDrift finding that reds the run.
	h, err := converge.Doctor(ctx, cfg)
	if err != nil {
		t.Fatalf("Doctor: %v", err)
	}
	if h.Healthy {
		t.Error("Healthy = true with a hand-edited generated workflow")
	}
	var named bool
	for _, f := range h.Findings {
		if f.Class == converge.CheckDrift && f.Name == payload.InstalledWorkflowTarget {
			named = true
			if f.Severity != converge.SeverityFinding || !strings.Contains(f.Detail, "generated stale") {
				t.Errorf("doctor drift row = {sev %q, detail %q}, want a finding naming generated stale", f.Severity, f.Detail)
			}
		}
	}
	if !named {
		t.Errorf("doctor did not name the hand-edited workflow as drift; findings = %+v", h.Findings)
	}

	// (c) converge REPAIRS it back to the config render.
	rep, err := converge.Run(ctx, cfg, converge.Options{Repair: true})
	if err != nil {
		t.Fatalf("repair Run: %v", err)
	}
	if want := []string{payload.InstalledWorkflowTarget}; !slices.Equal(actionPaths(rep.Regenerated), want) {
		t.Fatalf("Regenerated = %v, want the repaired workflow %v", actionPaths(rep.Regenerated), want)
	}
	repaired := renderWith(t, shippedTemplate(t), cfg, "main", "origin")
	if got := mustRead(t, at(root, payload.InstalledWorkflowTarget)); !bytes.Equal(got, repaired) {
		t.Fatalf("workflow not repaired to the config render\n--- got ---\n%s", got)
	}
}

// ---- fresh install: the workflow materializes from the archive in one pass --

// A fresh install runs the generated scan BEFORE it materializes the
// engine-verbatim payload, so the installed-workflow template is absent from
// the tree on the first converge. A MUTATING run sources the template from the
// verified archive (the same authoritative bytes engine-verbatim repair uses),
// so the workflow is materialized in ONE pass — regressing this is a broken
// `pyxis init` on every fresh install (init calls converge.Run first, then
// fails closed on any refusal).
func TestWorkflowFreshInstallMaterializesInOnePass(t *testing.T) {
	r := newRelease(t, workflowFiles(t))
	root := t.TempDir()
	cfg := r.config(t, root)
	cfg.Repo.Knobs.CIMode = "both" // this leg is T4's materialization mechanics, not T6's knob gate
	// Only the repo-owned config exists (init's seeding turf) — no .pyxis
	// payload, no docs/, no workflow template in the tree yet.
	writeFile(t, at(root, ".pyxis/config.yaml"), r.operandConfig(), 0o644)
	ctx := context.Background()

	rep, err := converge.Run(ctx, cfg, converge.Options{})
	if err != nil {
		t.Fatalf("fresh-install Run refused (regressed: init fails closed on this): %v", err)
	}
	if rep.Mode != converge.ModeInstall {
		t.Errorf("mode = %q, want install", rep.Mode)
	}
	if !slices.Contains(actionPaths(rep.Regenerated), payload.InstalledWorkflowTarget) {
		t.Fatalf("the workflow was not materialized on the fresh install; Regenerated = %v, refusals = %v",
			actionPaths(rep.Regenerated), refusalPaths(rep.RefusedByName))
	}
	// It is the default render (config is default here), byte-for-byte.
	want := renderWith(t, shippedTemplate(t), cfg, cfg.Repo.Branch, cfg.Repo.Remote)
	if got := mustRead(t, at(root, payload.InstalledWorkflowTarget)); !bytes.Equal(got, want) {
		t.Fatalf("fresh-install workflow is not the config render\n--- got ---\n%s", got)
	}
	// And the engine-verbatim template source landed too (same pass).
	if _, err := os.Stat(at(root, payload.InstalledWorkflow)); err != nil {
		t.Errorf("the engine-verbatim workflow template was not materialized: %v", err)
	}
}

// ---- a hand-modified tree template cannot corrupt the render (mutating run) --

// The workflow template is an engine-verbatim file converge itself owns. On a
// MUTATING run the render sources the AUTHORITATIVE archive bytes, so a
// hand-modified (but still substitutable) tree template does NOT leak into the
// rendered workflow — converge renders the correct workflow AND repairs the
// tampered template, in one pass (no stale render left for a second converge).
func TestWorkflowTamperedTemplateDoesNotCorruptRender(t *testing.T) {
	root, cfg := operandWithWorkflow(t, "main", "origin")
	ctx := context.Background()

	// Tamper the template so a naive tree-read would render a WRONG workflow
	// (an injected extra line the archive's template does not carry), and drift
	// the target so a regenerate is needed.
	tampered := append(shippedTemplate(t), []byte("\n# INJECTED BY A MALICIOUS PR\n")...)
	writeFile(t, at(root, payload.InstalledWorkflow), tampered, 0o644)
	writeFile(t, at(root, payload.InstalledWorkflowTarget), []byte("drifted\n"), 0o644)

	rep, err := converge.Run(ctx, cfg, converge.Options{Repair: true})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// The workflow is regenerated from the ARCHIVE render — no injected line.
	got := mustRead(t, at(root, payload.InstalledWorkflowTarget))
	if bytes.Contains(got, []byte("INJECTED")) {
		t.Fatalf("the tampered tree template corrupted the rendered workflow (must render from the authoritative archive)\n%s", got)
	}
	want := renderWith(t, shippedTemplate(t), cfg, "main", "origin")
	if !bytes.Equal(got, want) {
		t.Fatalf("workflow not rendered from the authoritative template\n--- got ---\n%s", got)
	}
	// The tampered engine-verbatim template was itself repaired byte-exact.
	if got := mustRead(t, at(root, payload.InstalledWorkflow)); !bytes.Equal(got, shippedTemplate(t)) {
		t.Error("the tampered template was not repaired byte-exact")
	}
	// A second converge is a noop — one pass converged everything.
	rep2, err := converge.Run(ctx, cfg, converge.Options{})
	if err != nil {
		t.Fatalf("second Run: %v", err)
	}
	if rep2.Mode != converge.ModeNoop {
		t.Errorf("second run mode = %q, want noop (one pass fully converged)", rep2.Mode)
	}
	_ = rep
}

// ---- read-only face: an absent tree template is named as ungeneratable drift -

// Doctor/Drift never fetch or extract (FR-14/FR-19): they audit the tree AS IS.
// An absent installed-workflow template makes the generated workflow
// ungeneratable — a named drift finding, never a silent pass and never a fetch.
func TestWorkflowReadOnlyFaceNamesMissingTemplate(t *testing.T) {
	root, cfg := operandWithWorkflow(t, "main", "origin")
	ctx := context.Background()
	if _, err := resolve.EnsureManifest(ctx, cfg); err != nil {
		t.Fatalf("EnsureManifest: %v", err)
	}

	// Remove the tree template; the read-only faces have no archive to fall back on.
	if err := os.Remove(at(root, payload.InstalledWorkflow)); err != nil {
		t.Fatal(err)
	}

	cfg.Fetcher = denyFetcher{t} // the read-only faces must not fetch
	cls, err := converge.Drift(ctx, cfg)
	if err != nil {
		t.Fatalf("Drift: %v", err)
	}
	findings := cls.Findings
	var wf *converge.Finding
	for i := range findings {
		if findings[i].Path == payload.InstalledWorkflowTarget {
			wf = &findings[i]
		}
	}
	if wf == nil {
		t.Fatalf("Drift did not name the workflow; findings = %+v", findings)
	}
	if wf.State != converge.StateUngeneratable {
		t.Errorf("workflow state = %q, want ungeneratable (the tree template is absent)", wf.State)
	}
	if !strings.Contains(wf.Detail, "template") || !strings.Contains(wf.Detail, "cannot be read") {
		t.Errorf("ungeneratable detail = %q, want the named missing-template reason", wf.Detail)
	}
}
