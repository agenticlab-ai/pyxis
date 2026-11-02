package worktracker

import (
	"context"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

// ReindexReport is the outcome of a reverse-parent index rebuild — its trigger,
// the item and edge-marker counts it derived, and how long it took (docs/tech-
// design/worktracker-hierarchy-index.md §3.4, DINV-6). It is schema-versioned so
// an operator or a script can pin it (AP-6).
type ReindexReport struct {
	Schema  string        `json:"schema"`
	Trigger string        `json:"trigger"` // explicit · first-use · dirty · mismatch
	Items   int           `json:"items"`   // readable items scanned
	Markers int           `json:"markers"` // edge markers written
	Dur     time.Duration `json:"dur_ns"`  // rebuild duration in nanoseconds
}

// ReindexSchemaVersion pins the report's machine projection (AP-6; additive
// evolution only).
const ReindexSchemaVersion = "worktracker.reindex-report/v1"

// Reindexer rebuilds the derived reverse-parent index wholly from the item
// `parent` fields — the SOLE drift recovery (worktracker-hierarchy-index §3.7,
// DINV-6). It is a pure O(N) derivation kept off the hot path: writes stay
// incremental (the write choke point maintains one marker per edge) and this is
// the occasional heal, run on an absent index (first use), a detected mismatch, a
// tripped dirty check, or an explicit request. It never reads or trusts the old
// index — the item files are its sole input, so a crash-desynced, hand-edited, or
// missing index converges to the truth.
type Reindexer interface {
	Reindex(ctx context.Context) (ReindexReport, []schema.Defect)
}
