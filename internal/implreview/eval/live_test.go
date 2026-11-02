//go:build eval

package eval_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/implreview/eval"
	"github.com/agenticlab-ai/pyxis/internal/implreview/floor"
	"github.com/agenticlab-ai/pyxis/internal/implreview/lens"
	"github.com/agenticlab-ai/pyxis/internal/implreview/report"
	"github.com/agenticlab-ai/pyxis/internal/implreview/runtime"
)

// TestLiveEvalSuite is the GATED live-LLM eval (build tag `eval`, env
// IMPL_REVIEW_EVAL_LIVE=1). It is excluded from the hermetic `go test ./...`
// by the build tag and SKIPS when the env gate is unset or no live runner is
// wired — so a `-tags eval` build without a backend never produces a false
// failure. When enabled AND a runner is wired, it drives the SAME RunFamily
// protocol the hermetic harness proves, against the real lenses.
//
// Run it per release:
//
//	IMPL_REVIEW_EVAL_LIVE=1 go test -tags eval ./internal/implreview/eval/
func TestLiveEvalSuite(t *testing.T) {
	if !eval.LiveEnabled() {
		t.Skipf("live eval opt-out: set %s=1 to run the real-LLM eval (the hermetic harness runs in `go test ./...`)", eval.LiveEnvVar)
	}
	runner, err := eval.LiveRunner()
	if err != nil {
		t.Skipf("live eval: no runner wired: %v", err)
	}

	root := repoRoot(t)
	fixtures := filepath.Join(root, ".pyxis", "skills", "impl-review", "fixtures")

	families, err := eval.LoadFamilies(filepath.Join(fixtures, "families"))
	if err != nil {
		t.Fatalf("load families: %v", err)
	}
	prov, err := eval.LoadProvenance(filepath.Join(fixtures, "provenance.json"))
	if err != nil {
		t.Fatalf("load provenance: %v", err)
	}
	if _, err := eval.LoadBudgets(filepath.Join(fixtures, "budgets.json")); err != nil {
		t.Fatalf("load budgets: %v", err)
	}
	instr, err := lens.LoadInstructions(filepath.Join(root, ".pyxis", "skills", "impl-review", "lenses"))
	if err != nil {
		t.Fatalf("load lens instructions: %v", err)
	}

	cfg := eval.RunConfig{
		Provenance:   prov,
		Runner:       runner,
		Instructions: instr,
		RoundProvenance: report.Provenance{
			Model:          prov.RunUnder.Model,
			PromptsVersion: prov.RunUnder.PromptsVersion,
			CalVersion:     prov.RunUnder.CalVersion,
		},
		ReviewerRoot:    "session:live-reviewer-root",
		ImplementerRoot: "session:live-implementer-root",
		Enforced:        floor.EnforcedClasses(),
	}
	var _ runtime.LensRunner = cfg.Runner // the live seam is the T4 runner.

	for _, f := range families {
		f := f
		t.Run(f.ID, func(t *testing.T) {
			res, err := eval.RunFamily(context.Background(), f, cfg)
			if err != nil {
				t.Fatalf("run family %s: %v", f.ID, err)
			}
			if !res.Passed {
				if res.IntermittentMiss >= 0 {
					t.Fatalf("family %s INTERMITTENT MISS on run %d of %d — a defect, never retried: %s",
						f.ID, res.IntermittentMiss, res.Runs, res.Detail)
				}
				t.Fatalf("family %s not caught on every run (%d runs): %s", f.ID, res.Runs, res.Detail)
			}
		})
	}
}
