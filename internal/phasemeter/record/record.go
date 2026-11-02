// Package record is the pipeline-measurement run-record schema: one
// phase's measurement, as one schema-versioned JSON line in the store
// (docs/tech-design/pipeline-measurement-v2.md §3.6). Additive-schema
// evolution only (AP-6, DINV-6): a later schema version adds fields, never
// repurposes or removes an existing one, so an older reader still parses a
// newer record (ignoring fields it doesn't know) and a newer reader
// defaults any field an older record omits.
package record

import "fmt"

// SchemaVersion is the run-record schema this package writes and parses.
// Every record stamps it (DINV-6).
const SchemaVersion = "run-record/v1"

// ReconciliationTier says how tightly a recorded per-model figure was
// checked against the harness's own accounting (design §3.9, DINV-4).
// Populated by the reconciler (a later task); the zero value "" means
// not yet reconciled.
type ReconciliationTier string

// The three reconciliation tiers (design §3.9, DINV-4). Never invent a
// fourth: a figure that cannot be reconciled at all is TierUnavailable,
// not an omitted tier.
const (
	TierExact            ReconciliationTier = "exact"
	TierHarnessPrecision ReconciliationTier = "harness-precision"
	TierUnavailable      ReconciliationTier = "unavailable"
)

// TokenQuartet is one model's token accounting for a phase bracket, as a
// harness's own accounting reports it (design §3.6).
type TokenQuartet struct {
	Input         int64 `json:"input"`
	Output        int64 `json:"output"`
	CacheCreation int64 `json:"cache_creation"`
	CacheRead     int64 `json:"cache_read"`
}

// ModelSpend is one model's spend within a phase: its token quartet, the
// derived dollar cost, whether that cost could be derived at all, and how
// tightly the figures reconcile to the harness's own accounting (DINV-4).
// A later task's spend-source adapter, reconciler, and price deriver
// populate this; T1 only carries the slot.
type ModelSpend struct {
	TokenQuartet
	Cost               float64            `json:"cost,omitempty"`
	Unpriced           bool               `json:"unpriced,omitempty"` // true: no price entry — Cost is not a measurement (UB-2)
	ReconciliationTier ReconciliationTier `json:"reconciliation_tier,omitempty"`
}

// Efficiency is a phase's deterministic artifact+git structural signals
// (design §3.10, D-5): counts of the corpus's own typed anchors, plus git
// latency. A later task's efficiency reader populates this; T1 only
// carries the slots.
type Efficiency struct {
	Questions      int     `json:"questions,omitempty"`       // D-n anchors opened
	Fog            int     `json:"fog,omitempty"`             // G-n anchors (parked fog)
	Rework         int     `json:"rework,omitempty"`          // superseded D-n + amendment count
	Defects        int     `json:"defects,omitempty"`         // review FND-n anchors
	CoverageRatio  float64 `json:"coverage_ratio,omitempty"`  // from the plan's coverage matrix
	LatencySeconds int64   `json:"latency_seconds,omitempty"` // from git timestamps
}

// Unavailable declares one figure a harness could not expose for a phase —
// never a silent zero or estimate (design DINV-3; prd:pipeline-measurement
// UB-1; EVT-2 spend_unavailable).
type Unavailable struct {
	Harness       string `json:"harness"`
	MissingFigure string `json:"missing_figure"`
}

// Record is one phase's measurement: one JSON line in a run's store file
// (design §3.6). SchemaVersion, RunID, FeatureID, PhaseID, and Harness are
// the record's identity and are required (DINV-6, DINV-7; Validate checks
// them). Spend, Efficiency, PriceSnapshotVersion, and Unavailable are
// slots a later task populates — capturing spend/efficiency and deriving
// cost is out of scope for T1, so a T1-written record leaves them at
// their zero value, and a reader defaults any of them a record omits
// (additive-schema evolution, AP-6).
type Record struct {
	SchemaVersion string `json:"schema_version"`
	RunID         string `json:"run_id"`
	FeatureID     string `json:"feature_id"`
	PhaseID       string `json:"phase_id"`
	Harness       string `json:"harness"`

	Spend                map[string]ModelSpend `json:"spend,omitempty"`
	Efficiency           Efficiency            `json:"efficiency,omitzero"`
	PriceSnapshotVersion string                `json:"price_snapshot_version,omitempty"`
	Unavailable          []Unavailable         `json:"unavailable,omitempty"`
}

// Validate reports whether r carries the identity fields the store's
// append contract requires (DINV-6, DINV-7). It does not validate
// Spend/Efficiency content — capturing those values is a later task's
// concern; T1 is the schema and the store.
func (r Record) Validate() error {
	switch {
	case r.SchemaVersion == "":
		return fmt.Errorf("schema_version is required")
	case r.RunID == "":
		return fmt.Errorf("run_id is required")
	case r.FeatureID == "":
		return fmt.Errorf("feature_id is required")
	case r.PhaseID == "":
		return fmt.Errorf("phase_id is required")
	case r.Harness == "":
		return fmt.Errorf("harness is required")
	}
	return nil
}
