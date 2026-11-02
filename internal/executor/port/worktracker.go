package port

import "context"

// WorkTracker is the port onto the work-item tracker that projects the frozen
// plan (prd:implement-now/FR-1). The executor reads the ready frontier through
// it and posts progress back onto an item; every decision-critical read is exact
// and fresh, never a search index (AP-15). A production adapter binds a code
// host's issue tracker; the hermetic tests use an in-process fake.
type WorkTracker interface {
	// Frontier returns the ready tasks — open, unblocked and unclaimed over the
	// frozen plan's DAG. It is the Dispatcher's entry point; a single Worker
	// never calls it, since a Worker reads only its own handed Task
	// (prd:implement-now/FR-3).
	Frontier(ctx context.Context) ([]Task, error)

	// Comment posts a progress note onto a tracker item by issue number. It is a
	// notification, never the success artifact: callers treat its failure as
	// non-fatal — the proposal, not the comment, is what counts (DINV-3).
	Comment(ctx context.Context, issue int, body string) error
}
