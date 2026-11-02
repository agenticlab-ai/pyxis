package price_test

import (
	"math"
	"path/filepath"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/phasemeter/price"
	"github.com/agenticlab-ai/pyxis/internal/phasemeter/record"
)

// fixture is a small, hand-authored stand-in for the vendored LiteLLM
// table's shape: one priced chat model with all four rate dimensions
// distinct and non-zero, one genuinely-free chat model (a real entry,
// priced at zero — must NOT read as unpriced), the sample_spec schema-doc
// entry (typed identically to a priced model — excluded by name, not
// shape), and a non-chat entry priced per-image, not per-token (excluded
// because it carries neither input_cost_per_token nor
// output_cost_per_token).
const fixture = `{
  "test-model-a": {
    "litellm_provider": "anthropic",
    "mode": "chat",
    "input_cost_per_token": 0.000003,
    "output_cost_per_token": 0.000015,
    "cache_creation_input_token_cost": 0.00000375,
    "cache_read_input_token_cost": 0.0000003
  },
  "test-model-free": {
    "litellm_provider": "local",
    "mode": "chat",
    "input_cost_per_token": 0,
    "output_cost_per_token": 0
  },
  "sample_spec": {
    "input_cost_per_token": 0.0,
    "output_cost_per_token": 0.0,
    "cache_creation_input_token_cost": 0.0,
    "cache_read_input_token_cost": 0.0,
    "litellm_provider": "one of https://docs.litellm.ai/docs/providers"
  },
  "test-image-model": {
    "litellm_provider": "openai",
    "mode": "image_generation",
    "output_cost_per_image": 0.02
  }
}`

const fixtureVariant = `{
  "test-model-a": {
    "litellm_provider": "anthropic",
    "mode": "chat",
    "input_cost_per_token": 0.000004,
    "output_cost_per_token": 0.00002
  }
}`

func mustParse(t *testing.T, raw string) *price.Table {
	t.Helper()
	tbl, err := price.Parse([]byte(raw))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return tbl
}

func approxEqual(a, b float64) bool {
	return math.Abs(a-b) < 1e-12
}

func TestParse_ExcludesSampleSpecAndNonTokenEntries(t *testing.T) {
	tbl := mustParse(t, fixture)

	if _, priced := tbl.Cost("sample_spec", record.TokenQuartet{Input: 1}); priced {
		t.Error("sample_spec should never resolve as a priced model")
	}
	if _, priced := tbl.Cost("test-image-model", record.TokenQuartet{Input: 1}); priced {
		t.Error("a non-token-priced entry (image/audio/…) should not resolve as priced")
	}
	if _, priced := tbl.Cost("never-heard-of-this-model", record.TokenQuartet{Input: 1}); priced {
		t.Error("a model absent from the table should not resolve as priced")
	}
}

func TestCost_ComputesAcrossAllFourRateDimensions(t *testing.T) {
	tbl := mustParse(t, fixture)

	got, priced := tbl.Cost("test-model-a", record.TokenQuartet{
		Input: 100, Output: 50, CacheCreation: 10, CacheRead: 5,
	})
	if !priced {
		t.Fatal("test-model-a should be priced")
	}
	want := 100*0.000003 + 50*0.000015 + 10*0.00000375 + 5*0.0000003
	if !approxEqual(got, want) {
		t.Errorf("cost = %v, want %v", got, want)
	}
}

// A model genuinely priced at $0 (e.g. a free local model) is priced —
// distinct from a model with no entry at all. Collapsing these would
// reintroduce the silent-zero bug UB-2 exists to prevent.
func TestCost_GenuinelyFreeModelIsStillPriced(t *testing.T) {
	tbl := mustParse(t, fixture)

	got, priced := tbl.Cost("test-model-free", record.TokenQuartet{Input: 1000, Output: 1000})
	if !priced {
		t.Fatal("a model with a real, zero-rate price entry should still be priced")
	}
	if got != 0 {
		t.Errorf("cost = %v, want 0", got)
	}
}

func TestTable_Version_IsDeterministicAndContentDerived(t *testing.T) {
	a := mustParse(t, fixture)
	b := mustParse(t, fixture)
	if a.Version != b.Version {
		t.Errorf("parsing identical content twice should yield the same version: %q vs %q", a.Version, b.Version)
	}
	if a.Version == "" {
		t.Error("Version should not be empty")
	}

	c := mustParse(t, fixtureVariant)
	if a.Version == c.Version {
		t.Errorf("parsing different content should yield a different version, both were %q", a.Version)
	}
}

func TestParse_MalformedJSON(t *testing.T) {
	if _, err := price.Parse([]byte("not json")); err == nil {
		t.Fatal("expected an error for malformed JSON")
	}
}

func TestLoad_MissingFile(t *testing.T) {
	if _, err := price.Load(filepath.Join(t.TempDir(), "does-not-exist.json")); err == nil {
		t.Fatal("expected an error for a missing file")
	}
}

func recWithSpend(spend map[string]record.ModelSpend) record.Record {
	return record.Record{
		SchemaVersion: record.SchemaVersion,
		RunID:         "run-1",
		FeatureID:     "pipeline-measurement",
		PhaseID:       "phase-1",
		Harness:       "claude-code",
		Spend:         spend,
	}
}

// Acceptance: "A record reproduces its dollar cost from the price-snapshot
// version it stamped."
func TestDerive_StampsVersionAndReproducesCost(t *testing.T) {
	tbl := mustParse(t, fixture)
	rec := recWithSpend(map[string]record.ModelSpend{
		"test-model-a": {TokenQuartet: record.TokenQuartet{Input: 100, Output: 50, CacheCreation: 10, CacheRead: 5}},
	})

	got := tbl.Derive(rec)
	if got.PriceSnapshotVersion != tbl.Version {
		t.Errorf("PriceSnapshotVersion = %q, want %q", got.PriceSnapshotVersion, tbl.Version)
	}
	ms := got.Spend["test-model-a"]
	if ms.Unpriced {
		t.Error("test-model-a has a price entry and should not be flagged Unpriced")
	}

	// Reproduce: an independent parse of the same snapshot content stamps
	// the same version, and deriving from it recomputes the identical cost
	// for the same tokens (DINV-6 — a historical cost reproduces exactly
	// from the stamped snapshot version).
	reproduced := mustParse(t, fixture)
	if reproduced.Version != got.PriceSnapshotVersion {
		t.Fatalf("re-parsing the same snapshot content should reproduce version %q, got %q", got.PriceSnapshotVersion, reproduced.Version)
	}
	rederived := reproduced.Derive(rec)
	if !approxEqual(rederived.Spend["test-model-a"].Cost, ms.Cost) {
		t.Errorf("cost did not reproduce from the stamped snapshot version: got %v, want %v", rederived.Spend["test-model-a"].Cost, ms.Cost)
	}
}

// Acceptance: "A model with no price entry is flagged, never given a $0
// cost."
func TestDerive_FlagsUnpricedModel(t *testing.T) {
	tbl := mustParse(t, fixture)
	rec := recWithSpend(map[string]record.ModelSpend{
		"never-heard-of-this-model": {TokenQuartet: record.TokenQuartet{Input: 1000, Output: 1000}},
	})

	got := tbl.Derive(rec)
	ms := got.Spend["never-heard-of-this-model"]
	if !ms.Unpriced {
		t.Error("a model with no price entry must be flagged Unpriced")
	}
	if ms.Cost != 0 {
		t.Errorf("an unpriced model's cost field should stay 0, got %v", ms.Cost)
	}
}

func TestDerive_EmptySpend_OnlyStampsVersion(t *testing.T) {
	tbl := mustParse(t, fixture)
	rec := recWithSpend(nil)

	got := tbl.Derive(rec)
	if got.PriceSnapshotVersion != tbl.Version {
		t.Errorf("PriceSnapshotVersion = %q, want %q", got.PriceSnapshotVersion, tbl.Version)
	}
	if got.Spend != nil {
		t.Errorf("Spend should stay nil, got %v", got.Spend)
	}
}

func TestDerive_DoesNotMutateSourceRecordsSpendMap(t *testing.T) {
	tbl := mustParse(t, fixture)
	spend := map[string]record.ModelSpend{
		"test-model-a": {TokenQuartet: record.TokenQuartet{Input: 100, Output: 50}},
	}
	rec := recWithSpend(spend)

	_ = tbl.Derive(rec)
	if spend["test-model-a"].Cost != 0 {
		t.Error("Derive must not mutate the caller's original Spend map")
	}
}

// Integration guard: the actual vendored snapshot (T10) must parse under
// this package's rules, and a currently-pinned, well-known model id must
// resolve as priced. Loose on the exact dollar figure — an upstream price
// bump must not need this test to change.
func TestLoad_RealVendoredSnapshot(t *testing.T) {
	path, err := filepath.Abs("../../../.pyxis/vendor/litellm-prices/model_prices_and_context_window.json")
	if err != nil {
		t.Fatalf("resolve path: %v", err)
	}
	tbl, err := price.Load(path)
	if err != nil {
		t.Fatalf("Load(real vendored snapshot): %v", err)
	}
	cost, priced := tbl.Cost("claude-sonnet-5", record.TokenQuartet{Input: 1000, Output: 1000, CacheCreation: 100, CacheRead: 100})
	if !priced {
		t.Fatal("claude-sonnet-5 should be priced in the vendored snapshot")
	}
	if cost <= 0 {
		t.Errorf("expected a positive cost, got %v", cost)
	}
}
