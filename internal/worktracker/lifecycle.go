package worktracker

import (
	"context"

	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/worktracker/model"
)

// Editor changes a work item's free fields — the ones a human hand-edits. It
// touches exactly the fields the patch names and advances the updated stamp
// when a value actually changes, leaving every other field byte-identical
// (docs/tech-design/local-work-tracker.md §3.9; T2 acceptance). An out-of-set
// value or a missing item yields a defect and no mutation (fail closed, AP-17).
type Editor interface {
	Edit(ctx context.Context, id int, patch model.EditPatch) (model.WorkItem, []schema.Defect)
}

// Transitioner moves a work item between open and closed idempotently (§3.10,
// AP-22). A transition to the state the item already holds is a recorded no-op:
// it succeeds, changes nothing, and appends no audit entry — so the transition
// log carries exactly one entry per *applied* change (DINV-11 keeps state and
// close_reason in their closed sets; an out-of-set value is refused, naming it).
// Closing records why (a close reason); reopening clears it.
type Transitioner interface {
	Transition(ctx context.Context, id int, to model.State, reason model.CloseReason, author string) (model.WorkItem, []schema.Defect)
}
