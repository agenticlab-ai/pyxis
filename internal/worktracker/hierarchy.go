package worktracker

import (
	"context"

	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/worktracker/model"
)

// Parenter sets and clears a work item's parent, maintaining the
// Feature→Milestone→Epic→Task relation (prd:local-work-tracker/FR-7). The child
// stores its parent; that stored half is authoritative and the reverse view is
// the ChildReader scan (DINV-1). A parent reference to a missing item is refused
// with a defect naming the target and nothing is written (UB-2, fail closed —
// AP-17); the returned item is the child at its new parentage.
type Parenter interface {
	SetParent(ctx context.Context, childID, parentID int) (model.WorkItem, []schema.Defect)
	ClearParent(ctx context.Context, childID int) (model.WorkItem, []schema.Defect)
}

// ReadMode selects how a hierarchy read treats the derived reverse-parent index
// (docs/tech-design/worktracker-hierarchy-index.md §3.7, DINV-2).
type ReadMode int

const (
	// Display trusts the index markers for speed — the mode for `list --parent`
	// and the `show` rollup (§3.1).
	Display ReadMode = iota
	// Decision re-reads each candidate child's authoritative `parent` and drops
	// any that no longer names the parent, so a lagging or hand-edited marker
	// degrades speed, never correctness — the mode for a read feeding an epic
	// close/reap or any dispatch decision (DINV-2, AP-15 / prd:local-work-tracker/
	// FR-14). The index accelerates *finding*, never *deciding*.
	Decision
)

// ChildFilter narrows a children query, composing with the parent selection just
// as the list filters do (prd:local-work-tracker/FR-19, §3.1). Every field is
// optional; an empty State means all states — the FR-19 default.
type ChildFilter struct {
	State  model.State
	Signal string
	Role   model.Role
}

// RollupView is an epic's children grouped by state — the all-states rollup that
// supersedes the open-only one, so a completed epic still surfaces its full task
// set (prd:local-work-tracker/FR-7, worktracker-hierarchy-index §3.1, A-7). Each
// group is creation-ordered; either may be empty.
type RollupView struct {
	Open   []model.WorkItem
	Closed []model.WorkItem
}

// ChildReader enumerates a work item's direct children through one implementation
// shared by the engine (port) and the CLI (DINV-7). It reads the derived
// reverse-parent index, so a lookup is O(k) in the child count and never O(N) in
// the whole store (prd:local-work-tracker/NFR-8, worktracker-hierarchy-index
// §3.6); the child's `parent` field stays the sole authoritative edge (DINV-1).
// filter narrows the set (empty = all states); mode selects whether the markers
// are trusted (Display) or re-verified against each child's authoritative
// `parent` (Decision, DINV-2). Children come back in creation (id) order. Rollup
// is the epic rollup over Children — the same children, grouped open/closed (FR-7,
// A-7) — one shared implementation so the two front doors never disagree (DINV-7).
type ChildReader interface {
	Children(ctx context.Context, parentID int, filter ChildFilter, mode ReadMode) ([]model.WorkItem, []schema.Defect)
	Rollup(ctx context.Context, parentID int, mode ReadMode) (RollupView, []schema.Defect)
}
