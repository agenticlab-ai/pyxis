package worktracker

import (
	"context"

	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/worktracker/model"
)

// Signaler carries typed control state on work items through the store's
// governed signal vocabulary (docs/tech-design/local-work-tracker.md §3.5,
// §3.14). Signal mutation is idempotent — a present-add or an absent-remove is a
// recorded no-op (AP-22) — and the vocabulary is closed: an undefined name is
// refused (prd:local-work-tracker/UB-2) and an engine-recognized signal cannot
// be redefined (DINV-11). This is its own file so the port grows by embedding a
// new sub-interface, never by editing a shared method list (the plan's
// extension-by-addition discipline, AP-27).
type Signaler interface {
	// AddSignal puts a defined signal on the item and returns the updated item.
	// Adding an already-present signal is a recorded no-op; adding an undefined
	// signal is a defect naming it (prd:local-work-tracker/UB-2).
	AddSignal(ctx context.Context, id int, name string) (model.WorkItem, []schema.Defect)
	// RemoveSignal takes a signal off the item and returns the updated item.
	// Removing an absent signal is a recorded no-op.
	RemoveSignal(ctx context.Context, id int, name string) (model.WorkItem, []schema.Defect)
	// DefineSignal extends the vocabulary along the classification axis. Defining
	// a non-classification signal, or redefining an engine-recognized one, is a
	// defect naming the offense (DINV-11).
	DefineSignal(ctx context.Context, sig model.Signal) []schema.Defect
	// Signals returns the store's full signal vocabulary — the governed seed
	// merged with the operand's classification extensions, each reporting its
	// role.
	Signals(ctx context.Context) ([]model.Signal, []schema.Defect)
}
