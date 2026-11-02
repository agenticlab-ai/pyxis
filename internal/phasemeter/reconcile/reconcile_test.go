package reconcile_test

import (
	"reflect"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/phasemeter/reconcile"
	"github.com/agenticlab-ai/pyxis/internal/phasemeter/record"
)

func quartet(input, output, cacheCreation, cacheRead int64) record.TokenQuartet {
	return record.TokenQuartet{Input: input, Output: output, CacheCreation: cacheCreation, CacheRead: cacheRead}
}

// Tier is the pure per-figure predicate every other function in this
// package builds on — table-driven over the DINV-4 tier boundary cases.
func TestTier(t *testing.T) {
	exact := quartet(100, 50, 10, 5)

	tests := []struct {
		name     string
		recorded record.TokenQuartet
		total    reconcile.Total
		found    bool
		wantTier record.ReconciliationTier
	}{
		{
			name:     "no total reported at all",
			recorded: exact,
			total:    reconcile.Total{},
			found:    false,
			wantTier: record.TierUnavailable,
		},
		{
			name:     "exact total, byte-for-byte match",
			recorded: exact,
			total:    reconcile.Total{Tokens: exact, Exact: true},
			found:    true,
			wantTier: record.TierExact,
		},
		{
			name:     "exact total, mismatch is non-reconcilable drift",
			recorded: exact,
			total:    reconcile.Total{Tokens: quartet(100, 50, 10, 6), Exact: true},
			found:    true,
			wantTier: record.TierUnavailable,
		},
		{
			name:     "coarse total, recorded within the harness's stated precision",
			recorded: exact,
			total:    reconcile.Total{Tokens: quartet(102, 48, 10, 5), Exact: false, Precision: 5},
			found:    true,
			wantTier: record.TierHarnessPrecision,
		},
		{
			name:     "coarse total, byte-exact match still caps at harness-precision — never invents exactness",
			recorded: exact,
			total:    reconcile.Total{Tokens: exact, Exact: false, Precision: 5},
			found:    true,
			wantTier: record.TierHarnessPrecision,
		},
		{
			name:     "coarse total, recorded outside the harness's stated precision",
			recorded: exact,
			total:    reconcile.Total{Tokens: quartet(999, 50, 10, 5), Exact: false, Precision: 5},
			found:    true,
			wantTier: record.TierUnavailable,
		},
		{
			name:     "coarse total with zero precision requires an exact numeric match to reconcile at all",
			recorded: exact,
			total:    reconcile.Total{Tokens: exact, Exact: false, Precision: 0},
			found:    true,
			wantTier: record.TierHarnessPrecision,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := reconcile.Tier(tt.recorded, tt.total, tt.found); got != tt.wantTier {
				t.Errorf("Tier(%+v, %+v, %v) = %q, want %q", tt.recorded, tt.total, tt.found, got, tt.wantTier)
			}
		})
	}
}

// Acceptance: "On a reference run every recorded figure carries a
// reconciliation tier." A reference run has phases with no reported total
// at all (today's reality for every adapter) and a phase whose model does
// have one — Reconcile must leave no entry at the unset zero-value tier
// either way.
func TestReconcile_EveryFigureCarriesATier(t *testing.T) {
	recorded := map[string]record.ModelSpend{
		"claude-sonnet-5": {TokenQuartet: quartet(100, 50, 10, 5)},
		"claude-opus-4-8": {TokenQuartet: quartet(7, 3, 0, 0)},
	}

	got := reconcile.Reconcile(recorded, nil) // no totals reported for this phase at all
	if len(got) != len(recorded) {
		t.Fatalf("want %d entries, got %d: %+v", len(recorded), len(got), got)
	}
	for model, ms := range got {
		if ms.ReconciliationTier == "" {
			t.Errorf("model %q carries the unset zero-value tier, want a recorded tier", model)
		}
		if ms.ReconciliationTier != record.TierUnavailable {
			t.Errorf("model %q: with no total reported, want %q, got %q", model, record.TierUnavailable, ms.ReconciliationTier)
		}
	}

	withTotal := reconcile.Reconcile(recorded, map[string]reconcile.Total{
		"claude-sonnet-5": {Tokens: quartet(100, 50, 10, 5), Exact: true},
	})
	for model, ms := range withTotal {
		if ms.ReconciliationTier == "" {
			t.Errorf("model %q carries the unset zero-value tier, want a recorded tier", model)
		}
	}
	if got := withTotal["claude-sonnet-5"].ReconciliationTier; got != record.TierExact {
		t.Errorf("claude-sonnet-5 tier = %q, want %q", got, record.TierExact)
	}
	if got := withTotal["claude-opus-4-8"].ReconciliationTier; got != record.TierUnavailable {
		t.Errorf("claude-opus-4-8 tier = %q, want %q", got, record.TierUnavailable)
	}
}

// Acceptance: "An authoritative-total fixture reconciles exact (recorded
// equals the harness figure)."
func TestReconcile_AuthoritativeTotalFixture_ReconcilesExact(t *testing.T) {
	q := quartet(100, 50, 10, 5)
	recorded := map[string]record.ModelSpend{"claude-sonnet-5": {TokenQuartet: q}}
	totals := map[string]reconcile.Total{"claude-sonnet-5": {Tokens: q, Exact: true}}

	got := reconcile.Reconcile(recorded, totals)

	ms, ok := got["claude-sonnet-5"]
	if !ok {
		t.Fatalf("want a claude-sonnet-5 entry, got %+v", got)
	}
	if ms.ReconciliationTier != record.TierExact {
		t.Errorf("tier = %q, want %q", ms.ReconciliationTier, record.TierExact)
	}
	if ms.TokenQuartet != q {
		t.Errorf("recorded tokens must be unchanged by reconciliation: got %+v, want %+v", ms.TokenQuartet, q)
	}
}

// Acceptance: "A coarser-total fixture records harness-precision and never
// reports more precision than the harness gave." Proven two ways: a
// recorded figure merely within the stated tolerance, and — the sharper
// case — a recorded figure that happens to match the coarse total
// byte-for-byte, which must still cap at harness-precision rather than be
// opportunistically upgraded to exact.
func TestReconcile_CoarserTotalFixture_RecordsHarnessPrecision(t *testing.T) {
	within := map[string]record.ModelSpend{"claude-sonnet-5": {TokenQuartet: quartet(1002, 498, 10, 5)}}
	coarse := map[string]reconcile.Total{"claude-sonnet-5": {Tokens: quartet(1000, 500, 10, 5), Exact: false, Precision: 10}}

	got := reconcile.Reconcile(within, coarse)
	if tier := got["claude-sonnet-5"].ReconciliationTier; tier != record.TierHarnessPrecision {
		t.Errorf("tier = %q, want %q", tier, record.TierHarnessPrecision)
	}

	exactMatch := quartet(1000, 500, 10, 5)
	recordedSameAsCoarse := map[string]record.ModelSpend{"claude-sonnet-5": {TokenQuartet: exactMatch}}
	coarseSameNumbers := map[string]reconcile.Total{"claude-sonnet-5": {Tokens: exactMatch, Exact: false, Precision: 10}}

	got = reconcile.Reconcile(recordedSameAsCoarse, coarseSameNumbers)
	if tier := got["claude-sonnet-5"].ReconciliationTier; tier != record.TierHarnessPrecision {
		t.Errorf("a byte-exact match against a coarse (non-Exact) total must still read harness-precision, not exact: got %q", tier)
	}
}

func TestReconcile_DoesNotMutateInputMap(t *testing.T) {
	recorded := map[string]record.ModelSpend{"claude-sonnet-5": {TokenQuartet: quartet(1, 1, 0, 0)}}
	_ = reconcile.Reconcile(recorded, map[string]reconcile.Total{
		"claude-sonnet-5": {Tokens: quartet(1, 1, 0, 0), Exact: true},
	})
	if recorded["claude-sonnet-5"].ReconciliationTier != "" {
		t.Error("Reconcile must not mutate the caller's original recorded map")
	}
}

func TestReconcile_EmptyAndNil(t *testing.T) {
	if got := reconcile.Reconcile(nil, nil); got != nil {
		t.Errorf("Reconcile(nil, nil) = %+v, want nil", got)
	}
	empty := map[string]record.ModelSpend{}
	if got := reconcile.Reconcile(empty, nil); len(got) != 0 {
		t.Errorf("Reconcile(empty, nil) = %+v, want empty", got)
	}
}

// Join joins on phase id then model — mirroring efficiency.Join's shape —
// and a phase absent from totals still gets every figure tiered rather
// than skipped.
func TestJoin_KeyedByPhaseThenModel(t *testing.T) {
	records := []record.Record{
		{
			SchemaVersion: record.SchemaVersion, RunID: "run-1", FeatureID: "pipeline-measurement",
			PhaseID: "prd-new", Harness: "claude-code",
			Spend: map[string]record.ModelSpend{"claude-sonnet-5": {TokenQuartet: quartet(100, 50, 10, 5)}},
		},
		{
			SchemaVersion: record.SchemaVersion, RunID: "run-1", FeatureID: "pipeline-measurement",
			PhaseID: "prd-review", Harness: "claude-code",
			Spend: map[string]record.ModelSpend{"claude-opus-4-8": {TokenQuartet: quartet(7, 3, 0, 0)}},
		},
	}
	totals := map[string]map[string]reconcile.Total{
		"prd-new": {"claude-sonnet-5": {Tokens: quartet(100, 50, 10, 5), Exact: true}},
		// "prd-review" has no entry at all — its own phase's authoritative
		// total simply was not reported.
	}

	got := reconcile.Join(records, totals)

	if len(got) != 2 {
		t.Fatalf("want 2 records, got %d", len(got))
	}
	if tier := got[0].Spend["claude-sonnet-5"].ReconciliationTier; tier != record.TierExact {
		t.Errorf("prd-new tier = %q, want %q", tier, record.TierExact)
	}
	if tier := got[1].Spend["claude-opus-4-8"].ReconciliationTier; tier != record.TierUnavailable {
		t.Errorf("prd-review tier = %q, want %q", tier, record.TierUnavailable)
	}
}

func TestJoin_DoesNotMutateRecordsOrTheirSpendMaps(t *testing.T) {
	original := map[string]record.ModelSpend{"claude-sonnet-5": {TokenQuartet: quartet(1, 1, 0, 0)}}
	records := []record.Record{{PhaseID: "prd-new", Spend: original}}
	totals := map[string]map[string]reconcile.Total{
		"prd-new": {"claude-sonnet-5": {Tokens: quartet(1, 1, 0, 0), Exact: true}},
	}

	got := reconcile.Join(records, totals)

	if original["claude-sonnet-5"].ReconciliationTier != "" {
		t.Error("Join must not mutate the original record's Spend map")
	}
	if got[0].Spend["claude-sonnet-5"].ReconciliationTier != record.TierExact {
		t.Error("Join's own returned record must carry the reconciled tier")
	}
	if !reflect.DeepEqual(records[0].Spend, original) {
		t.Error("Join must not mutate the caller's records slice contents")
	}
}

// A phase with no Spend at all (e.g. a DINV-3 fully-unavailable phase)
// must pass through untouched — nothing to reconcile, never an error.
func TestJoin_PhaseWithNoSpend_PassesThroughUnchanged(t *testing.T) {
	records := []record.Record{{
		PhaseID:     "tech-design-new",
		Unavailable: []record.Unavailable{{Harness: "claude-code", MissingFigure: "per-model spend"}},
	}}

	got := reconcile.Join(records, map[string]map[string]reconcile.Total{
		"tech-design-new": {"claude-sonnet-5": {Tokens: quartet(1, 1, 0, 0), Exact: true}},
	})

	if len(got[0].Spend) != 0 {
		t.Errorf("want no Spend entries, got %+v", got[0].Spend)
	}
	if len(got[0].Unavailable) != 1 {
		t.Errorf("Unavailable markers must pass through untouched, got %+v", got[0].Unavailable)
	}
}
