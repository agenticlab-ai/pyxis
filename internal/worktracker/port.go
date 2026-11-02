// Package worktracker declares the WorkTracker port — the consumer-side Go
// interface the engine depends on and the localfs adapter implements
// (docs/tech-design/local-work-tracker.md §3.7). The engine core imports this
// package, never an adapter package (the hexagon boundary, AGENTS.md Go rules).
// The interface carries no file-isms — no paths, no formats — so the future
// extraction into a cross-adapter contract stays clean (§3.2, Q4).
//
// The port grows by per-concern sub-interfaces composed by embedding, so later
// behaviors add a new interface and a new file rather than editing a shared
// method list (the plan's overlap discipline, AP-27). The first task defines
// the Creator and Reader concerns; WorkTracker aggregates them.
package worktracker

import (
	"context"

	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/worktracker/model"
)

// Creator allocates and persists a new work item. The returned item carries
// the assigned id, state, and timestamps; a non-empty defect slice means no
// item was created (fail closed — AP-17).
type Creator interface {
	Create(ctx context.Context, spec model.CreateSpec) (model.WorkItem, []schema.Defect)
}

// Reader resolves a work item by its identifier. A malformed or missing item
// yields a defect naming file, operation, reason, and remedy — never a partial
// or guessed item (DINV-1, prd:local-work-tracker/UB-1).
type Reader interface {
	Get(ctx context.Context, id int) (model.WorkItem, []schema.Defect)
}

// WorkTracker is the aggregate port. Later tasks embed further concern
// interfaces as they land. T2 added the lifecycle concerns Editor and
// Transitioner (lifecycle.go); T3 added the append-log concerns Commenter and
// Recorder (log.go); T4 added Signaler (signal.go); T5 added Parenter and
// ChildReader (hierarchy.go); T6 added Depender and BlockingReader
// (dependency.go); T7 added the query concerns Lister and Searcher (query.go);
// T8 added Predicate (predicate.go).
type WorkTracker interface {
	Creator
	Reader
	Editor
	Transitioner
	Commenter
	Recorder
	Signaler
	Parenter
	ChildReader
	Reindexer
	Depender
	BlockingReader
	Lister
	Searcher
	Predicate
}
