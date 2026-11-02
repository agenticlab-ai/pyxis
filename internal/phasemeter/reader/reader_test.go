package reader_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/phasemeter/price"
	"github.com/agenticlab-ai/pyxis/internal/phasemeter/reader"
	"github.com/agenticlab-ai/pyxis/internal/phasemeter/record"
	"github.com/agenticlab-ai/pyxis/internal/phasemeter/store"
)

// newGitRepo and commitAt mirror internal/phasemeter/efficiency's own test
// fixture (a throwaway repo with a fixed identity and injected commit
// dates, never the wall clock — the hermetic-test discipline) — duplicated
// here rather than imported since Go test helpers do not cross package
// boundaries.
func newGitRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	runGit(t, dir, "2026-07-01T00:00:00Z", "init", "-q", "-b", "main")
	return dir
}

func runGit(t *testing.T, dir, date string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
		"GIT_AUTHOR_DATE="+date, "GIT_COMMITTER_DATE="+date)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func commitAt(t *testing.T, dir, path, content, date string) {
	t.Helper()
	writeFile(t, filepath.Join(dir, path), content)
	runGit(t, dir, date, "add", path)
	runGit(t, dir, date, "commit", "-q", "-m", "commit "+path+" at "+date)
}

const brainstormFixture = `# Foo brainstorm

### D-1 — pick approach A
- **Status**: live

### D-2 — old approach, revised
- **Status**: superseded by D-1 — revised after feedback

- **G-1** — need to confirm X later
`

const priceFixture = `{
  "model-a": {"litellm_provider": "anthropic", "mode": "chat", "input_cost_per_token": 0.000003, "output_cost_per_token": 0.000015},
  "model-b": {"litellm_provider": "anthropic", "mode": "chat", "input_cost_per_token": 0.000001, "output_cost_per_token": 0.000002}
}`

func loadPriceFixture(t *testing.T) *price.Table {
	t.Helper()
	tbl, err := price.Parse([]byte(priceFixture))
	if err != nil {
		t.Fatalf("parse price fixture: %v", err)
	}
	return tbl
}

// buildReferenceRun stands up a two-phase reference run: a
// feature-brainstorm phase (a brainstorm doc revised twice, carrying two
// decisions — one superseded — and one fog entry) and a prd-new phase (a
// base + one amendment, and a spend figure the harness could not fully
// capture for a second model). It returns the repo root and the store
// holding "run-1"'s two records.
func buildReferenceRun(t *testing.T) (repoRoot string, st *store.Store) {
	t.Helper()
	dir := newGitRepo(t)
	commitAt(t, dir, "docs/brainstorms/foo.md", "draft\n", "2026-07-01T00:00:00Z")
	commitAt(t, dir, "docs/brainstorms/foo.md", brainstormFixture, "2026-07-01T03:00:00Z")
	commitAt(t, dir, "docs/prds/foo/000-base.md", "base\n", "2026-07-02T00:00:00Z")
	commitAt(t, dir, "docs/prds/foo/001-amend.md", "amendment\n", "2026-07-02T05:00:00Z")

	storeDir := t.TempDir()
	st = store.New(storeDir)
	records := []record.Record{
		{
			SchemaVersion: record.SchemaVersion, RunID: "run-1", FeatureID: "foo",
			PhaseID: "feature-brainstorm", Harness: "claude-code",
			Spend: map[string]record.ModelSpend{
				"model-a": {TokenQuartet: record.TokenQuartet{Input: 1000, Output: 200}, ReconciliationTier: record.TierExact},
			},
		},
		{
			SchemaVersion: record.SchemaVersion, RunID: "run-1", FeatureID: "foo",
			PhaseID: "prd-new", Harness: "claude-code",
			Spend: map[string]record.ModelSpend{
				"model-b": {TokenQuartet: record.TokenQuartet{Input: 500, Output: 100}, ReconciliationTier: record.TierHarnessPrecision},
			},
			Unavailable: []record.Unavailable{{Harness: "claude-code", MissingFigure: "per-model spend (model-c)"}},
		},
	}
	for _, r := range records {
		if err := st.Append(r); err != nil {
			t.Fatalf("append fixture record: %v", err)
		}
	}
	return dir, st
}

// Acceptance (plan pipeline-measurement/001 T12): "For a reference run, the
// read lists every phase with its per-model tokens, cost, and efficiency
// signals, or the declared unavailable markers."
func TestRead_JoinsReconciledSpendCostAndEfficiency(t *testing.T) {
	repoRoot, st := buildReferenceRun(t)
	prices := loadPriceFixture(t)

	views, err := reader.Read(st, prices, repoRoot, "run-1")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(views) != 2 {
		t.Fatalf("want 2 phase views, got %d: %+v", len(views), views)
	}

	brainstorm := views[0]
	if brainstorm.PhaseID != "feature-brainstorm" || brainstorm.FeatureID != "foo" || brainstorm.Harness != "claude-code" {
		t.Errorf("feature-brainstorm identity: %+v", brainstorm)
	}
	if len(brainstorm.Models) != 1 {
		t.Fatalf("want 1 model on feature-brainstorm, got %+v", brainstorm.Models)
	}
	m := brainstorm.Models[0]
	if m.Model != "model-a" || m.Tokens != (record.TokenQuartet{Input: 1000, Output: 200}) {
		t.Errorf("feature-brainstorm model-a tokens: %+v", m)
	}
	if m.Unpriced {
		t.Error("model-a has a price entry and must not be flagged Unpriced")
	}
	wantCost := 1000*0.000003 + 200*0.000015
	if diff := m.Cost - wantCost; diff > 1e-12 || diff < -1e-12 {
		t.Errorf("feature-brainstorm cost = %v, want %v", m.Cost, wantCost)
	}
	if m.ReconciliationTier != record.TierExact {
		t.Errorf("reconciliation tier = %q, want %q", m.ReconciliationTier, record.TierExact)
	}
	wantEff := record.Efficiency{Questions: 2, Fog: 1, Rework: 1, LatencySeconds: 3 * 60 * 60}
	if brainstorm.Efficiency != wantEff {
		t.Errorf("feature-brainstorm efficiency = %+v, want %+v", brainstorm.Efficiency, wantEff)
	}

	prdNew := views[1]
	if prdNew.PhaseID != "prd-new" {
		t.Errorf("want prd-new second (recorded order), got %+v", prdNew)
	}
	if len(prdNew.Models) != 1 || prdNew.Models[0].Model != "model-b" {
		t.Fatalf("want 1 model-b on prd-new, got %+v", prdNew.Models)
	}
	wantCostB := 500*0.000001 + 100*0.000002
	if diff := prdNew.Models[0].Cost - wantCostB; diff > 1e-12 || diff < -1e-12 {
		t.Errorf("prd-new cost = %v, want %v", prdNew.Models[0].Cost, wantCostB)
	}
	if len(prdNew.Unavailable) != 1 || prdNew.Unavailable[0].MissingFigure != "per-model spend (model-c)" {
		t.Errorf("prd-new unavailable markers: %+v", prdNew.Unavailable)
	}
	wantEffPrd := record.Efficiency{Rework: 1, LatencySeconds: 5 * 60 * 60} // one amendment, no anchors
	if prdNew.Efficiency != wantEffPrd {
		t.Errorf("prd-new efficiency = %+v, want %+v", prdNew.Efficiency, wantEffPrd)
	}
}

func TestRead_UnknownRun_ReturnsEmptyNotError(t *testing.T) {
	repoRoot := t.TempDir()
	st := store.New(t.TempDir())

	views, err := reader.Read(st, loadPriceFixture(t), repoRoot, "no-such-run")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(views) != 0 {
		t.Errorf("want no views for an unknown run, got %+v", views)
	}
}

// A model absent from the price table reads Unpriced, never a silent $0
// (UB-2), exercised through the reader's own join rather than price.Table
// directly.
func TestRead_UnpricedModel_NeverASilentZero(t *testing.T) {
	repoRoot := t.TempDir()
	storeDir := t.TempDir()
	st := store.New(storeDir)
	if err := st.Append(record.Record{
		SchemaVersion: record.SchemaVersion, RunID: "run-1", FeatureID: "f", PhaseID: "unknown-phase", Harness: "claude-code",
		Spend: map[string]record.ModelSpend{"never-priced-model": {TokenQuartet: record.TokenQuartet{Input: 10}}},
	}); err != nil {
		t.Fatal(err)
	}

	views, err := reader.Read(st, loadPriceFixture(t), repoRoot, "run-1")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(views) != 1 || len(views[0].Models) != 1 {
		t.Fatalf("want 1 view with 1 model, got %+v", views)
	}
	m := views[0].Models[0]
	if !m.Unpriced {
		t.Error("a model with no price entry must be flagged Unpriced")
	}
	if m.Cost != 0 {
		t.Errorf("an unpriced model's cost must stay 0, got %v", m.Cost)
	}
}

// A nil price table (no pricing available at all) must still flag every
// model Unpriced rather than let a record's own zero-valued Cost field
// read as a genuine $0 measurement.
func TestRead_NilPrices_FlagsEveryModelUnpriced(t *testing.T) {
	repoRoot := t.TempDir()
	st := store.New(t.TempDir())
	if err := st.Append(record.Record{
		SchemaVersion: record.SchemaVersion, RunID: "run-1", FeatureID: "f", PhaseID: "unknown-phase", Harness: "claude-code",
		Spend: map[string]record.ModelSpend{"model-a": {TokenQuartet: record.TokenQuartet{Input: 10}}},
	}); err != nil {
		t.Fatal(err)
	}

	views, err := reader.Read(st, nil, repoRoot, "run-1")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if !views[0].Models[0].Unpriced {
		t.Error("with no price table, every model must read Unpriced")
	}
}
