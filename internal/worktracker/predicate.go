package worktracker

import (
	"context"

	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/worktracker/model"
)

// Predicate answers the engine's two dispatch questions — the ready set and the
// triage (undecided) set — each as one shared implementation over the model,
// returned identically to the engine (through this port) and to a human (through
// the CLI) so the two can never diverge (DINV-6 — AP-11/INV-30;
// prd:local-work-tracker/FR-12, FR-13, FR-16). Both reads are strong: the
// adapter scans the authoritative item files directly, with no index or cache on
// the decision path (DINV-4), so a membership read reflects every prior write.
// Results come back in creation (id) order for fair rotation (INV-12). A
// directory-level IO error is a defect; a single malformed item is quarantined,
// never failing the whole set (§3.13).
type Predicate interface {
	ReadySet(ctx context.Context) ([]model.WorkItem, []schema.Defect)
	TriageSet(ctx context.Context) ([]model.WorkItem, []schema.Defect)
}
