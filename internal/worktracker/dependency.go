package worktracker

import (
	"context"

	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/worktracker/model"
)

// Depender adds and removes blocked_by dependency edges — the ordering
// constraints of prd:local-work-tracker/FR-8. The source item stores its
// blocked_by list (the authoritative half); the reverse blocking view is the
// BlockingReader scan (DINV-1). Both are maintained together: an add or remove
// updates the stored half and the reverse view follows. An edge that would
// close a cycle is refused with the full cycle path named and nothing written
// (UB-3/DINV-10, fail closed — AP-17); a dangling local target or a malformed
// reference is likewise refused naming it (UB-2). The returned item is the
// source at its new edge set. The cross-repo owner/repo#id reference form is
// stored and displayed opaquely — never resolved in v1 (§3.5, Q5).
type Depender interface {
	AddBlockedBy(ctx context.Context, id int, ref string) (model.WorkItem, []schema.Defect)
	RemoveBlockedBy(ctx context.Context, id int, ref string) (model.WorkItem, []schema.Defect)
}

// BlockingReader enumerates the items an item blocks — the reverse of
// blocked_by — by scanning for the id in other items' blocked_by lists; no
// forward→reverse index is authoritative (DINV-1), and the scan is strong
// (§3.5). Results come back in creation (id) order. Cross-repo references are
// never resolved, so blocking is a local-only reverse view (Q5).
type BlockingReader interface {
	Blocking(ctx context.Context, id int) ([]model.WorkItem, []schema.Defect)
}
