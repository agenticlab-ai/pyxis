package conformance

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/worktracker/model"
)

// The dependency rows extend the port contract with blocked_by ordering edges
// (prd:local-work-tracker/FR-8, UB-2, UB-3). They run against any adapter
// through the port alone, so the same behaviors prove localfs today and any
// future medium unchanged. Later behaviors append rows without editing these.

// dependencyBidirectional: after AddBlockedBy the source names its blocker and
// the blocker names the source through Blocking; after RemoveBlockedBy both
// directions drop the edge (FR-8 — the edge is maintained both directions).
func dependencyBidirectional(t *testing.T, h Harness) {
	store, _ := h.NewStore(t)
	ctx := context.Background()
	feature, defects := store.Create(ctx, model.CreateSpec{Title: "the feature"})
	mustClean(t, "create-feature", defects)
	blocker, defects := store.Create(ctx, model.CreateSpec{Title: "the blocker"})
	mustClean(t, "create-blocker", defects)

	added, defects := store.AddBlockedBy(ctx, feature.ID, strconv.Itoa(blocker.ID))
	mustClean(t, "add-blocked-by", defects)
	// Forward: the source names its blocker.
	mustCondition(t, hasRef(added.BlockedBy, blocker.ID), "AddBlockedBy returned blocked_by %v, want #%d", added.BlockedBy, blocker.ID)
	got, defects := store.Get(ctx, feature.ID)
	mustClean(t, "get-source", defects)
	mustCondition(t, hasRef(got.BlockedBy, blocker.ID), "source blocked_by = %v, want #%d", got.BlockedBy, blocker.ID)
	// Reverse: the blocker names the source through Blocking.
	blocking, defects := store.Blocking(ctx, blocker.ID)
	mustClean(t, "blocking", defects)
	mustCondition(t, len(blocking) == 1 && blocking[0].ID == feature.ID, "Blocking(%d) = %v, want just #%d", blocker.ID, ids(blocking), feature.ID)

	// Remove drops the edge both directions.
	if _, defects := store.RemoveBlockedBy(ctx, feature.ID, strconv.Itoa(blocker.ID)); len(defects) > 0 {
		t.Fatalf("remove-blocked-by: %v", defects)
	}
	got, defects = store.Get(ctx, feature.ID)
	mustClean(t, "get-removed", defects)
	if hasRef(got.BlockedBy, blocker.ID) {
		t.Fatalf("source blocked_by after remove = %v, want none", got.BlockedBy)
	}
	blocking, defects = store.Blocking(ctx, blocker.ID)
	mustClean(t, "blocking-after-remove", defects)
	if len(blocking) != 0 {
		t.Fatalf("Blocking(%d) after remove = %v, want none", blocker.ID, ids(blocking))
	}
}

// dependencyCycleRefused: an edge that would close a cycle is refused with the
// full cycle path named, and the edge set is left unchanged (UB-3/DINV-10). The
// fixture is #a blocked_by #b blocked_by #c; adding #c blocked_by #a closes the
// loop.
func dependencyCycleRefused(t *testing.T, h Harness) {
	store, _ := h.NewStore(t)
	ctx := context.Background()
	a, defects := store.Create(ctx, model.CreateSpec{Title: "a"})
	mustClean(t, "create-a", defects)
	b, defects := store.Create(ctx, model.CreateSpec{Title: "b"})
	mustClean(t, "create-b", defects)
	c, defects := store.Create(ctx, model.CreateSpec{Title: "c"})
	mustClean(t, "create-c", defects)

	if _, defects := store.AddBlockedBy(ctx, a.ID, strconv.Itoa(b.ID)); len(defects) > 0 {
		t.Fatalf("add a→b: %v", defects)
	}
	if _, defects := store.AddBlockedBy(ctx, b.ID, strconv.Itoa(c.ID)); len(defects) > 0 {
		t.Fatalf("add b→c: %v", defects)
	}

	// Adding c blocked_by a would close a→b→c→a.
	_, defects = store.AddBlockedBy(ctx, c.ID, strconv.Itoa(a.ID))
	if len(defects) == 0 {
		t.Fatalf("cycle-closing edge #%d blocked_by #%d was accepted", c.ID, a.ID)
	}
	named := false
	for _, d := range defects {
		if d.File == "" || d.Op == "" || d.Reason == "" || d.Remedy == "" {
			t.Fatalf("cycle defect does not name the full quadruple: %+v", d)
		}
		// The full path names every node on the cycle.
		if namesAll(d.Reason, a.ID, b.ID, c.ID) {
			named = true
		}
	}
	if !named {
		t.Fatalf("cycle refusal does not name the full path #%d→#%d→#%d: %v", a.ID, b.ID, c.ID, defects)
	}

	// The edge set is unchanged: c gained no blocker.
	got, defects := store.Get(ctx, c.ID)
	mustClean(t, "get-c", defects)
	if len(got.BlockedBy) != 0 {
		t.Fatalf("refused cycle still mutated #%d: blocked_by=%v", c.ID, got.BlockedBy)
	}
}

// dependencyDanglingRefused: a blocked_by target that names no item is refused
// with a defect naming the missing target, and the source gains no edge
// (UB-2 — no dangling edge persists).
func dependencyDanglingRefused(t *testing.T, h Harness) {
	store, _ := h.NewStore(t)
	ctx := context.Background()
	src, defects := store.Create(ctx, model.CreateSpec{Title: "src"})
	mustClean(t, "create-src", defects)

	const missing = 9999
	_, defects = store.AddBlockedBy(ctx, src.ID, strconv.Itoa(missing))
	mustNameOffense(t, "dangling blocked_by target", defects, strconv.Itoa(missing))

	got, defects := store.Get(ctx, src.ID)
	mustClean(t, "get-src", defects)
	if len(got.BlockedBy) != 0 {
		t.Fatalf("dangling edge persisted: blocked_by=%v", got.BlockedBy)
	}
}

// dependencyMalformedRefused: a reference that is neither a local id nor the
// cross-repo owner/repo#id form is refused with a defect naming it, and the
// source gains no edge (UB-2). The cross-repo form itself is accepted and stored
// opaquely, proving the malformed refusal is not merely rejecting non-integers.
func dependencyMalformedRefused(t *testing.T, h Harness) {
	store, _ := h.NewStore(t)
	ctx := context.Background()
	src, defects := store.Create(ctx, model.CreateSpec{Title: "src"})
	mustClean(t, "create-src", defects)

	const malformed = "not a ref"
	_, defects = store.AddBlockedBy(ctx, src.ID, malformed)
	mustNameOffense(t, "malformed blocked_by reference", defects, malformed)

	got, defects := store.Get(ctx, src.ID)
	mustClean(t, "get-src", defects)
	if len(got.BlockedBy) != 0 {
		t.Fatalf("malformed reference persisted: blocked_by=%v", got.BlockedBy)
	}

	// The cross-repo form is stored opaquely, without resolution.
	const crossRepo = "octo/hello#42"
	added, defects := store.AddBlockedBy(ctx, src.ID, crossRepo)
	mustClean(t, "add-cross-repo", defects)
	if countSignal(added.BlockedBy, crossRepo) != 1 {
		t.Fatalf("cross-repo reference not stored: blocked_by=%v", added.BlockedBy)
	}
	got, defects = store.Get(ctx, src.ID)
	mustClean(t, "get-cross-repo", defects)
	if countSignal(got.BlockedBy, crossRepo) != 1 {
		t.Fatalf("cross-repo reference did not round-trip: blocked_by=%v", got.BlockedBy)
	}
}

// hasRef reports whether refs carries the local id as a blocked_by edge.
func hasRef(refs []string, id int) bool {
	for _, r := range refs {
		if kind, n := model.ClassifyRef(r); kind == model.RefLocal && n == id {
			return true
		}
	}
	return false
}

// namesAll reports whether reason names every given id as `#n`.
func namesAll(reason string, ids ...int) bool {
	for _, id := range ids {
		if !strings.Contains(reason, "#"+strconv.Itoa(id)) {
			return false
		}
	}
	return true
}
