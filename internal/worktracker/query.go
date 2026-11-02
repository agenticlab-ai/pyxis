package worktracker

import (
	"context"

	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/worktracker/model"
)

// Lister returns the work items matching a state/signal filter, bounded and
// stably sorted in creation (id) order — the discovery primitive under
// readiness and triage (prd:local-work-tracker/FR-10, §3.6). The query fields
// combine with AND and an empty query returns the whole backlog; a role filter
// selects every item carrying a signal of that role, which is how the
// factory-implementable (authorization-role) set is named (§3.5, DINV-4 — the
// scan reads authoritative files, never an index). An out-of-set state or role
// is refused with a named defect (DINV-11). This is its own file so the port
// grows by embedding a new sub-interface, never by editing a shared method list
// (AP-27).
type Lister interface {
	List(ctx context.Context, q model.Query) ([]model.WorkItem, []schema.Defect)
}

// Searcher returns every item — and only the items — whose title, body, or
// comments contain the query text as an exact match, never a fuzzy one
// (prd:local-work-tracker/FR-11, §3.6). Matching is literal containment (no
// stemming, ranking, or edit distance), so a near-miss yields zero false
// positives. Results come back in creation (id) order. An empty query is
// refused with a named defect.
type Searcher interface {
	Search(ctx context.Context, text string) ([]model.WorkItem, []schema.Defect)
}
