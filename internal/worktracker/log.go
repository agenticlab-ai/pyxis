package worktracker

import (
	"context"

	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/worktracker/model"
)

// Commenter appends dialogue to a work item's append-only log
// (docs/tech-design/local-work-tracker.md §3.1, prd:local-work-tracker/FR-5). A
// comment is never rewritten and never a no-op: every call lands one entry in
// time order, so an interleaved burst from concurrent writers loses nothing
// (DINV-5). This is its own file so the port grows by embedding a new
// sub-interface, never by editing a shared method list (the plan's
// extension-by-addition discipline, AP-27).
type Commenter interface {
	// Comment appends author's body to the item as a comment entry and returns
	// the updated item. An empty body is refused naming the offense; a missing or
	// malformed item surfaces its own defect (fail closed, AP-17).
	Comment(ctx context.Context, id int, author, body string) (model.WorkItem, []schema.Defect)
}

// Recorder freezes typed judgments onto a work item as namespaced decision-
// records — the structured replacement for a hosted tracker's marker-comment
// grammar (RW-3, prd:local-work-tracker/FR-9, §3.14). Records resolve
// latest-wins per namespace (DINV-5): the last record appended under a namespace
// is authoritative, an identical repeat is a recorded no-op (AP-22), and an
// out-of-set namespace is refused naming the value (DINV-11).
type Recorder interface {
	// Record appends rec as a decision-record and returns the updated item.
	// Appending a record whose payload equals the current latest record in the
	// same namespace writes nothing (a recorded no-op, AP-22). An out-of-set
	// namespace is a defect naming it; a missing or malformed item surfaces its
	// own defect.
	Record(ctx context.Context, id int, rec model.DecisionRecord) (model.WorkItem, []schema.Defect)
	// LatestRecord returns the authoritative decision-record for ns and whether
	// one exists — the latest-wins read (DINV-5, DINV-6). An out-of-set namespace
	// is a defect; a missing item surfaces its own.
	LatestRecord(ctx context.Context, id int, ns model.Namespace) (model.DecisionRecord, bool, []schema.Defect)
}
