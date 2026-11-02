package eval

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/implreview/report"
	implruntime "github.com/agenticlab-ai/pyxis/internal/implreview/runtime"
)

func writeCoverage90Fixture(t *testing.T, name, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCoverage90BudgetAndProvenanceDataFailures(t *testing.T) {
	budgets := writeCoverage90Fixture(t, "budgets.json", `{
		"schema":"impl-review-eval-budgets/v1",
		"bands":{"skip":{"max_tokens":0,"max_latency_ms":0}}
	}`)
	if _, err := LoadBudgets(budgets); err == nil {
		t.Fatal("LoadBudgets accepted a file missing standard and deep bands")
	}

	provenance := writeCoverage90Fixture(t, "provenance.json", `{
		"schema":"impl-review-eval-provenance/v0",
		"runs_per_family":5,
		"caught_every_run":true
	}`)
	if _, err := LoadProvenance(provenance); err == nil {
		t.Fatal("LoadProvenance accepted an unsupported schema")
	}
}

func TestCoverage90GoldenReportDerivesBlockingVerdict(t *testing.T) {
	golden := &Golden{
		Band: report.BandDeep,
		Findings: []Finding{{
			RuleID: "correctness-critic/defect", CalRow: "CAL-1",
			Severity: report.SeverityBlocking, Message: "blocking defect",
		}},
	}
	got := GoldenReport("EVAL-COVERAGE", 1, golden)
	if got.Verdict != report.VerdictRequestChanges {
		t.Fatalf("GoldenReport verdict = %q, want request changes", got.Verdict)
	}
}

type coverage90ErrorRunner struct{}

func (coverage90ErrorRunner) Run(context.Context, string, []byte, string) ([]report.Finding, error) {
	return nil, errors.New("lens failed")
}

func coverage90RunConfig() RunConfig {
	instructions := make(map[string]string, len(implruntime.Lenses))
	for _, lens := range implruntime.Lenses {
		instructions[lens] = "review the supplied pack"
	}
	return RunConfig{
		Runner:          coverage90ErrorRunner{},
		Instructions:    instructions,
		RoundProvenance: report.Provenance{Model: "test", PromptsVersion: "v1", CalVersion: "v1"},
		ReviewerRoot:    "reviewer-root",
		ImplementerRoot: "implementer-root",
	}
}

func TestCoverage90HarnessConfigurationAndDispatchFailures(t *testing.T) {
	family := &Family{ID: "EVAL-COVERAGE"}
	if _, err := RunFamily(context.Background(), family, RunConfig{}); err == nil {
		t.Fatal("RunFamily accepted nil provenance")
	}
	if _, err := RunFamily(context.Background(), family, RunConfig{Provenance: &Provenance{}}); err == nil {
		t.Fatal("RunFamily accepted nil runner")
	}
	zeroRuns := coverage90RunConfig()
	zeroRuns.Provenance = &Provenance{}
	if got, err := RunFamily(context.Background(), family, zeroRuns); err != nil || !got.Passed {
		t.Fatalf("RunFamily zero-run boundary = %+v, %v", got, err)
	}
	if _, _, err := runOnce(context.Background(), family, zeroRuns, nil, 0); err == nil {
		t.Fatal("runOnce accepted an unknown family kind")
	}
}

func TestCoverage90HarnessLegsPropagateRuntimeErrors(t *testing.T) {
	cfg := coverage90RunConfig()
	leg := Leg{Band: report.BandStandard, Charter: "charter", Diff: "diff", BaselineDiff: "base", InjectedDiff: "injected"}
	tests := []struct {
		name string
		run  func() error
	}{
		{"seeded", func() error {
			_, _, err := runSeededCatch(context.Background(), &Family{Legs: []Leg{leg}}, cfg, 0)
			return err
		}},
		{"floor", func() error {
			_, _, err := runFloorExclusion(context.Background(), &Family{Legs: []Leg{leg}}, cfg, map[string]bool{}, 0)
			return err
		}},
		{"injection", func() error {
			_, _, err := runInjection(context.Background(), &Family{Legs: []Leg{leg}}, cfg, 0)
			return err
		}},
		{"nit", func() error {
			_, _, err := runNitGolden(context.Background(), &Family{Goldens: []Golden{{Band: report.BandStandard, Charter: "charter", Diff: "diff"}}}, cfg, 0)
			return err
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.run(); err == nil {
				t.Fatal("harness leg swallowed the runtime error")
			}
		})
	}
}
