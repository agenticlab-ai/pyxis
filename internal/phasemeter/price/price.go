// Package price derives a phase record's per-model dollar cost from the
// vendored, pinned LiteLLM per-model price snapshot
// (.pyxis/vendor/litellm-prices, T10) — vendored *data*, parsed with
// stdlib encoding/json, never a code dependency
// (docs/tech-design/pipeline-measurement-v2.md §3.12).
//
// A parsed Table stamps a content-derived Version onto a record — the
// pricing-snapshot version DINV-6 requires every record to carry, so a
// historical cost reproduces exactly from that stamp — and flags a model
// absent from the table as Unpriced rather than deriving a silent zero
// cost (prd:pipeline-measurement/UB-2).
package price

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"

	"github.com/agenticlab-ai/pyxis/internal/phasemeter/record"
)

// DefaultRelPath is the vendored price snapshot's default location
// relative to the repo root (internal/reporoot) — the file T10's
// .pyxis/vendor/litellm-prices/refresh.sh vendors and pins.
const DefaultRelPath = ".pyxis/vendor/litellm-prices/model_prices_and_context_window.json"

// sampleSpecKey is the vendored table's one documented non-model entry — a
// schema-doc sample, not a priced model (litellm's own convention; the
// vendor's refresh.sh shape check keys on its presence too). It is
// excluded by name: its fields happen to type-check as a priced model's
// (see modelRates), so shape alone would not exclude it.
const sampleSpecKey = "sample_spec"

// modelRates is one model's per-token USD rates, as the vendored table
// represents them — a field-for-field match to record.TokenQuartet's four
// dimensions (design §3.12: cost = tokens × the pinned per-class price).
type modelRates struct {
	InputCostPerToken           float64 `json:"input_cost_per_token"`
	OutputCostPerToken          float64 `json:"output_cost_per_token"`
	CacheCreationInputTokenCost float64 `json:"cache_creation_input_token_cost"`
	CacheReadInputTokenCost     float64 `json:"cache_read_input_token_cost"`
}

// cost returns q's dollar cost at r's rates.
func (r modelRates) cost(q record.TokenQuartet) float64 {
	return float64(q.Input)*r.InputCostPerToken +
		float64(q.Output)*r.OutputCostPerToken +
		float64(q.CacheCreation)*r.CacheCreationInputTokenCost +
		float64(q.CacheRead)*r.CacheReadInputTokenCost
}

// Table is a parsed, immutable snapshot of the vendored per-model price
// map, plus the content-derived Version DINV-6 requires every record to
// stamp. Version is computed from the parsed bytes themselves, so it can
// never drift from the data it names: the same snapshot content always
// yields the same Version, and any changed byte yields a different one —
// a stronger reproducibility guarantee than trusting a hand-bumped label
// (the vendor README documents why a litellm release tag alone isn't
// cryptographically pinned). A Table is read-only after Parse/Load return,
// so concurrent use needs no synchronization.
type Table struct {
	Version string

	rates map[string]modelRates
}

// Load reads and parses the price snapshot at path (see DefaultRelPath).
func Load(path string) (*Table, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("phasemeter/price: load %s: %w", path, err)
	}
	t, err := Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("load %s: %w", path, err)
	}
	return t, nil
}

// Parse parses raw price-snapshot JSON (the vendored table's shape: a
// top-level object keyed by model id) into a Table. An entry with neither
// an input nor an output per-token rate — a non-chat pricing dimension
// (image/audio/… priced per-image or per-second, not per-token) or a
// non-model meta key such as sample_spec — is not a priced model for this
// table's purpose and is excluded, never zero-priced.
func Parse(raw []byte) (*Table, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		return nil, fmt.Errorf("phasemeter/price: parse: %w", err)
	}

	rates := make(map[string]modelRates, len(top))
	for id, entry := range top {
		if id == sampleSpecKey {
			continue
		}
		var presence map[string]json.RawMessage
		if err := json.Unmarshal(entry, &presence); err != nil {
			continue // not an object; not a priced model
		}
		_, hasInput := presence["input_cost_per_token"]
		_, hasOutput := presence["output_cost_per_token"]
		if !hasInput && !hasOutput {
			continue
		}
		var r modelRates
		if err := json.Unmarshal(entry, &r); err != nil {
			return nil, fmt.Errorf("phasemeter/price: parse: model %q: %w", id, err)
		}
		rates[id] = r
	}

	sum := sha256.Sum256(raw)
	return &Table{
		Version: fmt.Sprintf("litellm-%x", sum),
		rates:   rates,
	}, nil
}

// Cost returns model's dollar cost for tokens at t's rates, and whether
// model has a price entry at all. An unpriced model's cost is always 0 —
// callers must check priced before trusting it: the zero is not a
// measurement, it is the absence of one (UB-2).
func (t *Table) Cost(model string, tokens record.TokenQuartet) (cost float64, priced bool) {
	r, ok := t.rates[model]
	if !ok {
		return 0, false
	}
	return r.cost(tokens), true
}

// Derive returns rec with every Spend entry's Cost and Unpriced flag set
// from t, and rec.PriceSnapshotVersion stamped to t.Version (DINV-6). A
// model with no entry in t is flagged Unpriced rather than given a silent
// zero cost (UB-2); its recorded tokens are left exactly as they were. The
// original rec (and its Spend map) is never mutated.
func (t *Table) Derive(rec record.Record) record.Record {
	rec.PriceSnapshotVersion = t.Version
	if len(rec.Spend) == 0 {
		return rec
	}
	spend := make(map[string]record.ModelSpend, len(rec.Spend))
	for model, ms := range rec.Spend {
		cost, priced := t.Cost(model, ms.TokenQuartet)
		ms.Cost = cost
		ms.Unpriced = !priced
		spend[model] = ms
	}
	rec.Spend = spend
	return rec
}
