package converge_test

// T6 — ci_mode wired for real (plan init-hardening/001 T6; prd:init-hardening/
// FR-11; design:init-hardening-v1#3.8). Two legs the T4 render-mechanics suite
// (workflow_test.go, always ci_mode ∈ {actions, both}) does not cover:
//
//   - the desired-set exclusion: Run/preview never write, repair, or count as
//     drift the hosted workflow when ci_mode excludes hosted CI ("local", or
//     the uncaptured zero value) — excluded means UNLISTED, so a fresh install
//     is born right and a second converge stays a noop;
//   - doctor's mismatch advisory: an already-installed repo carrying the
//     workflow under a since-excluding ci_mode gets a named, remedy-bearing
//     advisory (never a finding — the path is unlisted, not drifted) and
//     doctor performs zero mutations; a born-right (never-installed) tree
//     under the same excluding knob stays silent.

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"slices"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/converge"
	"github.com/agenticlab-ai/pyxis/internal/payload"
)

// ---- Run: local excludes, fresh install is born right ------------------------

func TestHostedWorkflowExcludedUnderLocalCIMode(t *testing.T) {
	r := newRelease(t, workflowFiles(t))
	root := t.TempDir()
	cfg := r.config(t, root)
	cfg.Repo.Knobs.CIMode = "local"
	// Only the repo-owned config exists (init's seeding turf) — a fresh install.
	writeFile(t, at(root, ".pyxis/config.yaml"), r.operandConfig(), 0o644)
	ctx := context.Background()

	rep, err := converge.Run(ctx, cfg, converge.Options{})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if slices.Contains(actionPaths(rep.Regenerated), payload.InstalledWorkflowTarget) {
		t.Errorf("Regenerated = %v, want the hosted workflow EXCLUDED under ci_mode=local", actionPaths(rep.Regenerated))
	}
	if slices.Contains(refusalPaths(rep.RefusedByName), payload.InstalledWorkflowTarget) {
		t.Errorf("RefusedByName = %v, want the excluded workflow unlisted, not refused", refusalPaths(rep.RefusedByName))
	}
	if _, err := os.Stat(at(root, payload.InstalledWorkflowTarget)); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the hosted workflow was materialized under ci_mode=local (excluded means unlisted, never written): %v", err)
	}
	// The exclusion is scoped to exactly the rendered TARGET: the workflow's own
	// engine-verbatim TEMPLATE source (a different, always-shipped path) and
	// other engine-verbatim content still land — no other mode's content is
	// touched by this knob (the plan's "hooks present in every mode" leg).
	if _, err := os.Stat(at(root, payload.InstalledWorkflow)); err != nil {
		t.Errorf("the engine-verbatim workflow template was not materialized under ci_mode=local: %v", err)
	}
	if _, err := os.Stat(at(root, ".pyxis/templates/prd-template.md")); err != nil {
		t.Errorf("other engine-verbatim content was not materialized under ci_mode=local: %v", err)
	}

	// A second converge run mutates nothing (idempotent desired set).
	before := snap(t, root)
	cfg.Fetcher = denyFetcher{t}
	rep2, err := converge.Run(ctx, cfg, converge.Options{})
	if err != nil {
		t.Fatalf("second Run: %v", err)
	}
	if rep2.Mode != converge.ModeNoop {
		t.Errorf("second run under ci_mode=local mode = %q, want noop", rep2.Mode)
	}
	sameSnap(t, before, snap(t, root), "second run under ci_mode=local")
}

// ---- Run: actions/both include, and stay included idempotently ---------------

func TestHostedWorkflowIncludedForActionsAndBoth(t *testing.T) {
	for _, mode := range []string{"actions", "both"} {
		t.Run(mode, func(t *testing.T) {
			r := newRelease(t, workflowFiles(t))
			root := t.TempDir()
			cfg := r.config(t, root)
			cfg.Repo.Knobs.CIMode = mode
			writeFile(t, at(root, ".pyxis/config.yaml"), r.operandConfig(), 0o644)
			ctx := context.Background()

			rep, err := converge.Run(ctx, cfg, converge.Options{})
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			if !slices.Contains(actionPaths(rep.Regenerated), payload.InstalledWorkflowTarget) {
				t.Fatalf("ci_mode=%s did not materialize the hosted workflow; Regenerated = %v", mode, actionPaths(rep.Regenerated))
			}
			want := renderWith(t, shippedTemplate(t), cfg, cfg.Repo.Branch, cfg.Repo.Remote)
			if got := mustRead(t, at(root, payload.InstalledWorkflowTarget)); !bytes.Equal(got, want) {
				t.Fatalf("ci_mode=%s workflow is not the config render\n--- got ---\n%s", mode, got)
			}

			// Idempotent: a second run under the same included knob is a noop.
			before := snap(t, root)
			cfg.Fetcher = denyFetcher{t}
			rep2, err := converge.Run(ctx, cfg, converge.Options{})
			if err != nil {
				t.Fatalf("second Run: %v", err)
			}
			if rep2.Mode != converge.ModeNoop {
				t.Errorf("ci_mode=%s second run mode = %q, want noop", mode, rep2.Mode)
			}
			sameSnap(t, before, snap(t, root), "second run under ci_mode="+mode)
		})
	}
}

// ---- doctor: the mismatch advisory, both remedies, zero mutations ------------

// A repo that captured the workflow under a formerly-including ci_mode and
// later flips to excluding it is left carrying a file converge will neither
// repair nor delete (no deletion machinery exists, #D-4). Doctor names the
// mismatch as an advisory — never a finding, since the path is unlisted, not
// drifted — with both remedies, and performs zero mutations.
func TestDoctorAdvisesOnExcludedWorkflowMismatch(t *testing.T) {
	root, cfg := operandWithWorkflow(t, "main", "origin") // pristine; helper sets ci_mode=both
	ctx := context.Background()
	gitInitRepo(t, root)
	gitHooksPathSet(t, root, ".githooks")

	// The operator flips the knob after install: the FILE remains (converge
	// never deletes an excluded path) but the config now excludes it.
	cfg.Repo.Knobs.CIMode = "local"

	before := snap(t, root)
	cfg.Fetcher = denyFetcher{t}
	h := mustDoctor(t, ctx, cfg)
	sameSnap(t, before, snap(t, root), "doctor mismatch advisory") // zero mutations

	wantRow(t, h, converge.CheckDrift, payload.InstalledWorkflowTarget, converge.SeverityAdvisory,
		"excludes hosted CI", `"actions"`, `"both"`, "delete", "FR-11")
	if !h.Healthy {
		t.Errorf("Healthy = false — the mismatch is advisory-only and must never red the run; findings = %+v", h.Findings)
	}
}

// A never-installed tree under an excluding ci_mode is born right: doctor
// stays silent (no advisory, no finding) since there is nothing to name.
func TestDoctorSilentWhenExcludedWorkflowNeverInstalled(t *testing.T) {
	r := newRelease(t, workflowFiles(t))
	root := t.TempDir()
	cfg := r.config(t, root)
	cfg.Repo.Knobs.CIMode = "local"
	writeFile(t, at(root, ".pyxis/config.yaml"), r.operandConfig(), 0o644)
	ctx := context.Background()
	if _, err := converge.Run(ctx, cfg, converge.Options{}); err != nil {
		t.Fatalf("warming Run: %v", err)
	}
	gitInitRepo(t, root)
	gitHooksPathSet(t, root, ".githooks")

	cfg.Fetcher = denyFetcher{t}
	h := mustDoctor(t, ctx, cfg)
	for _, f := range doctorRows(h, converge.CheckDrift) {
		if f.Name == payload.InstalledWorkflowTarget {
			t.Fatalf("doctor advised on the hosted workflow though it was never installed (born-right silence expected): %+v", f)
		}
	}
	if !h.Healthy {
		t.Errorf("Healthy = false on a born-right ci_mode=local install; findings = %+v", h.Findings)
	}
}
