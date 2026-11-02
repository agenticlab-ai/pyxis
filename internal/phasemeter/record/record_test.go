package record_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/phasemeter/record"
)

func full() record.Record {
	return record.Record{
		SchemaVersion: record.SchemaVersion,
		RunID:         "run-1",
		FeatureID:     "pipeline-measurement",
		PhaseID:       "phase-1",
		Harness:       "claude-code",
		Spend: map[string]record.ModelSpend{
			"claude-sonnet-5": {
				TokenQuartet: record.TokenQuartet{
					Input: 100, Output: 50, CacheCreation: 10, CacheRead: 5,
				},
				Cost:               0.42,
				ReconciliationTier: record.TierExact,
			},
		},
		Efficiency: record.Efficiency{
			Questions: 3, Fog: 1, Rework: 2, Defects: 0,
			CoverageRatio: 0.8, LatencySeconds: 120,
		},
		PriceSnapshotVersion: "litellm-2026-07-01",
		Unavailable: []record.Unavailable{
			{Harness: "cursor", MissingFigure: "input_tokens"},
		},
	}
}

func TestRoundTrip(t *testing.T) {
	want := full()
	b, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got record.Record
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("round trip mismatch:\nwant %+v\ngot  %+v", want, got)
	}
}

// The design's schema (§3.6) lists a model's spend as one flat object —
// {input, output, cache_creation, cache_read, cost, reconciliation_tier}
// — not a token quartet nested under its own key. ModelSpend embeds
// TokenQuartet anonymously so encoding/json flattens it; this test pins
// that shape.
func TestModelSpendFieldsAreFlat(t *testing.T) {
	rec := record.Record{
		SchemaVersion: record.SchemaVersion, RunID: "r", FeatureID: "f", PhaseID: "p", Harness: "h",
		Spend: map[string]record.ModelSpend{
			"m": {TokenQuartet: record.TokenQuartet{Input: 1, Output: 2, CacheCreation: 3, CacheRead: 4}},
		},
	}
	b, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("unmarshal to raw: %v", err)
	}
	spend, ok := raw["spend"].(map[string]any)
	if !ok {
		t.Fatalf("spend not an object: %v", raw["spend"])
	}
	m, ok := spend["m"].(map[string]any)
	if !ok {
		t.Fatalf("spend[m] not an object: %v", spend["m"])
	}
	for _, key := range []string{"input", "output", "cache_creation", "cache_read"} {
		if _, ok := m[key]; !ok {
			t.Errorf("spend[m] missing flattened field %q: %v", key, m)
		}
	}
	if _, nested := m["tokens"]; nested {
		t.Errorf("spend[m] should flatten the token quartet, not nest it under \"tokens\": %v", m)
	}
}

// The un-priced flag (T11; prd:pipeline-measurement/UB-2: "a model with no
// price entry is flagged, never given a $0 cost") is a plain flattened
// field on ModelSpend, like cost and reconciliation_tier — and, unlike
// them, it is only ever meaningful when true, so a priced model's JSON
// stays clean (omitempty drops it) while an unpriced model's carries an
// explicit, unambiguous marker distinct from a silently-omitted zero cost.
func TestModelSpendUnpricedFlattensAndOmitsWhenFalse(t *testing.T) {
	rec := record.Record{
		SchemaVersion: record.SchemaVersion, RunID: "r", FeatureID: "f", PhaseID: "p", Harness: "h",
		Spend: map[string]record.ModelSpend{
			"priced":   {TokenQuartet: record.TokenQuartet{Input: 1}, Cost: 0.01},
			"unpriced": {TokenQuartet: record.TokenQuartet{Input: 1}, Unpriced: true},
		},
	}
	b, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("unmarshal to raw: %v", err)
	}
	spend, ok := raw["spend"].(map[string]any)
	if !ok {
		t.Fatalf("spend not an object: %v", raw["spend"])
	}

	priced, ok := spend["priced"].(map[string]any)
	if !ok {
		t.Fatalf("spend[priced] not an object: %v", spend["priced"])
	}
	if _, present := priced["unpriced"]; present {
		t.Errorf("a priced model should omit the unpriced key entirely, got %v", priced)
	}

	unpriced, ok := spend["unpriced"].(map[string]any)
	if !ok {
		t.Fatalf("spend[unpriced] not an object: %v", spend["unpriced"])
	}
	if v, present := unpriced["unpriced"]; !present || v != true {
		t.Errorf("an unpriced model should carry unpriced:true, got %v", unpriced)
	}

	var got record.Record
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !reflect.DeepEqual(rec, got) {
		t.Fatalf("round trip mismatch:\nwant %+v\ngot  %+v", rec, got)
	}
}

func TestValidateRequiresIdentityFields(t *testing.T) {
	base := record.Record{
		SchemaVersion: record.SchemaVersion, RunID: "r", FeatureID: "f", PhaseID: "p", Harness: "h",
	}
	if err := base.Validate(); err != nil {
		t.Fatalf("fully-identified record should validate, got %v", err)
	}

	tests := []struct {
		name string
		zero func(*record.Record)
	}{
		{"schema_version", func(r *record.Record) { r.SchemaVersion = "" }},
		{"run_id", func(r *record.Record) { r.RunID = "" }},
		{"feature_id", func(r *record.Record) { r.FeatureID = "" }},
		{"phase_id", func(r *record.Record) { r.PhaseID = "" }},
		{"harness", func(r *record.Record) { r.Harness = "" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := base
			tt.zero(&rec)
			err := rec.Validate()
			if err == nil {
				t.Fatalf("expected an error with %s empty, got nil", tt.name)
			}
			if !strings.Contains(err.Error(), tt.name) {
				t.Errorf("error %q should name the missing field %q", err.Error(), tt.name)
			}
		})
	}
}

// Acceptance: "A record missing an optional field loads with that field
// defaulted, never an error."
func TestMissingOptionalFieldsDefaultOnUnmarshal(t *testing.T) {
	const minimal = `{"schema_version":"run-record/v1","run_id":"r","feature_id":"f","phase_id":"p","harness":"h"}`
	var got record.Record
	if err := json.Unmarshal([]byte(minimal), &got); err != nil {
		t.Fatalf("a record with only the identity fields must load without error, got %v", err)
	}
	if got.Spend != nil {
		t.Errorf("Spend should default to nil, got %v", got.Spend)
	}
	if got.Efficiency != (record.Efficiency{}) {
		t.Errorf("Efficiency should default to its zero value, got %+v", got.Efficiency)
	}
	if got.PriceSnapshotVersion != "" {
		t.Errorf("PriceSnapshotVersion should default to \"\", got %q", got.PriceSnapshotVersion)
	}
	if got.Unavailable != nil {
		t.Errorf("Unavailable should default to nil, got %v", got.Unavailable)
	}
}

// Additive-schema evolution (AP-6) cuts both ways: a record carrying a
// field this build doesn't know yet (a future schema version) must still
// load, the unknown field simply dropped.
func TestUnknownFieldsAreIgnored(t *testing.T) {
	const fromTheFuture = `{
		"schema_version":"run-record/v1","run_id":"r","feature_id":"f","phase_id":"p","harness":"h",
		"a_field_v1_has_never_heard_of": {"nested": true}
	}`
	var got record.Record
	if err := json.Unmarshal([]byte(fromTheFuture), &got); err != nil {
		t.Fatalf("a record carrying an unknown field must still load (additive-schema evolution), got %v", err)
	}
	if got.RunID != "r" {
		t.Errorf("known fields should still populate, got RunID=%q", got.RunID)
	}
}
