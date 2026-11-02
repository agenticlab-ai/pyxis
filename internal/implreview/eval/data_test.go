package eval_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/implreview/eval"
	"github.com/agenticlab-ai/pyxis/internal/implreview/report"
)

// TestBudgetsAreVersionedData — the band token/latency budgets are loaded from
// versioned DATA (§3.5), closed, covering the three bands. This is the "band
// budgets are read from versioned data" DoD leg.
func TestBudgetsAreVersionedData(t *testing.T) {
	t.Parallel()
	b, err := eval.LoadBudgets(filepath.Join(fixturesDir(t), "budgets.json"))
	if err != nil {
		t.Fatalf("load budgets: %v", err)
	}
	for _, band := range []string{report.BandSkip, report.BandStandard, report.BandDeep} {
		bg, ok := b.Budget(band)
		if !ok {
			t.Fatalf("budgets missing band %q", band)
		}
		if band == report.BandDeep && bg.MaxTokens <= 0 {
			t.Fatalf("deep band must carry a positive token budget, got %d", bg.MaxTokens)
		}
	}
	// The deep band budgets a larger read than the standard band (the risk justifies
	// the cost) — a data sanity the suite pins so a bad edit is caught.
	std, _ := b.Budget(report.BandStandard)
	deep, _ := b.Budget(report.BandDeep)
	if deep.MaxTokens <= std.MaxTokens {
		t.Fatalf("deep token budget (%d) must exceed standard (%d)", deep.MaxTokens, std.MaxTokens)
	}
}

// TestBudgetsParseClosed — an unknown key in the budgets data is an error (AP-6),
// never a lenient read.
func TestBudgetsParseClosed(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "budgets.json")
	// A budgets file with an unknown top-level key.
	body := `{"schema":"impl-review-eval-budgets/v1","surprise":true,"bands":{"skip":{"max_tokens":0,"max_latency_ms":0},"standard":{"max_tokens":1,"max_latency_ms":1},"deep":{"max_tokens":2,"max_latency_ms":2}}}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := eval.LoadBudgets(path); err == nil {
		t.Fatal("an unknown key in budgets.json must be a closed-parse error (AP-6)")
	}
}

func TestBudgetAndProvenanceBoundaryFailures(t *testing.T) {
	t.Run("missing budget file", func(t *testing.T) {
		if _, err := eval.LoadBudgets(filepath.Join(t.TempDir(), "missing.json")); err == nil {
			t.Fatal("expected a missing budget file to fail")
		}
	})

	t.Run("wrong budget schema", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "budgets.json")
		body := `{"schema":"wrong","bands":{"skip":{},"standard":{},"deep":{}}}`
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := eval.LoadBudgets(path); err == nil {
			t.Fatal("expected the wrong budget schema to fail")
		}
	})

	t.Run("missing provenance file", func(t *testing.T) {
		if _, err := eval.LoadProvenance(filepath.Join(t.TempDir(), "missing.json")); err == nil {
			t.Fatal("expected a missing provenance file to fail")
		}
	})
}

// TestProvenanceEnforcesTheProtocolFloors — the run provenance's N must be >= 5
// and caught_every_run must be true; a data file that weakened either is refused
// (§3.5 non-negotiables), so the protocol can never silently run fewer than the
// floor or relax the every-run rule.
func TestProvenanceEnforcesTheProtocolFloors(t *testing.T) {
	t.Parallel()
	// The shipped provenance loads and satisfies the floors.
	p, err := eval.LoadProvenance(filepath.Join(fixturesDir(t), "provenance.json"))
	if err != nil {
		t.Fatalf("load shipped provenance: %v", err)
	}
	if p.RunsPerFamily < eval.MinRunsPerFamily {
		t.Fatalf("shipped provenance runs_per_family=%d below floor %d", p.RunsPerFamily, eval.MinRunsPerFamily)
	}
	if !p.CaughtEveryRun {
		t.Fatal("shipped provenance must set caught_every_run=true")
	}

	dir := t.TempDir()
	// N below the floor → refused.
	low := filepath.Join(dir, "low.json")
	writeProvenance(t, low, 4, true)
	if _, err := eval.LoadProvenance(low); err == nil {
		t.Fatal("runs_per_family below 5 must be refused (N >= 5, §3.5)")
	}
	// caught_every_run false → refused.
	relaxed := filepath.Join(dir, "relaxed.json")
	writeProvenance(t, relaxed, 5, false)
	if _, err := eval.LoadProvenance(relaxed); err == nil {
		t.Fatal("caught_every_run=false must be refused (an intermittent miss is a defect, §3.5)")
	}
}

// TestEightFamiliesLoadAndValidate — the eight EVAL-1..8 families are all present,
// load closed, validate their Kind-specific shape, and have unique ids. This pins
// the fixture corpus itself.
func TestEightFamiliesLoadAndValidate(t *testing.T) {
	t.Parallel()
	families := loadFamilies(t)
	want := map[string]bool{
		"EVAL-1": true, "EVAL-2": true, "EVAL-3": true, "EVAL-4": true,
		"EVAL-5": true, "EVAL-6": true, "EVAL-7": true, "EVAL-8": true,
	}
	got := map[string]bool{}
	for _, f := range families {
		got[f.ID] = true
		if f.Schema != eval.FamilySchema {
			t.Errorf("family %s wrong schema %q", f.ID, f.Schema)
		}
		if strings.TrimSpace(f.NegativeExample) == "" {
			t.Errorf("family %s has no negative example (§5.9 requires one)", f.ID)
		}
	}
	for id := range want {
		if !got[id] {
			t.Errorf("missing family %s", id)
		}
	}
}

// TestFamilyParseClosed — a family fixture with an unknown key is a closed-parse
// error (AP-6): the fixture schema is closed, so a stray field never slips in.
func TestFamilyParseClosed(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	body := `{"schema":"impl-review-eval-family/v1","id":"EVAL-X","kind":"seeded-catch","lens":"spec-fidelity","title":"t","threshold":"t","negative_example":"n","surprise":1,"legs":[{"band":"standard","must_catch":true,"charter":"c","diff":"d","expected_catch":{"defect_class":"x","rule_id":"spec-fidelity/x","cal_row":"CAL-1","severity":"blocking","message":"m"}}]}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := eval.LoadFamily(path); err == nil {
		t.Fatal("an unknown key in a family fixture must be a closed-parse error (AP-6)")
	}
}

// TestFamilyValidationRejectsAVacuousCatch — a seeded-catch family whose only leg
// is must_catch WITHOUT an expected_catch is rejected: a family that would run
// vacuously and pass is a data defect, never admitted (the fixture-integrity gate).
func TestFamilyValidationRejectsAVacuousCatch(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "vacuous.json")
	body := `{"schema":"impl-review-eval-family/v1","id":"EVAL-V","kind":"seeded-catch","lens":"spec-fidelity","title":"t","threshold":"t","negative_example":"n","legs":[{"band":"standard","must_catch":true,"charter":"c","diff":"d"}]}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := eval.LoadFamily(path); err == nil {
		t.Fatal("a must_catch leg with no expected_catch must be rejected (a vacuous family)")
	}
}

// writeProvenance writes a minimal provenance file with the given N and every-run
// flag, for the floor-enforcement test.
func writeProvenance(t *testing.T, path string, n int, everyRun bool) {
	t.Helper()
	body := `{"schema":"impl-review-eval-provenance/v1","note":"test","runs_per_family":` + itoa(n) +
		`,"caught_every_run":` + boolStr(everyRun) +
		`,"decode":{"temperature":0.0,"top_p":1.0,"seed":1},` +
		`"run_under":{"model":"m","prompts_version":"impl-review-lenses/v1","cal_version":"impl-review-calibration/v1"}}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
