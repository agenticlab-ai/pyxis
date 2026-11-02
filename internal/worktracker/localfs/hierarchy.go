package localfs

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"

	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/worktracker"
	"github.com/agenticlab-ai/pyxis/internal/worktracker/model"
)

// SetParent points childID at parentID, maintaining the parent/child relation
// (prd:local-work-tracker/FR-7). Both ends are checked before any write (fail
// closed, AP-17): a missing child surfaces its own resolve/parse defect (UB-1),
// a missing parent is a defect naming the target (UB-2, DINV-1), and an item
// cannot be its own parent. The child's stored parent is the authoritative half;
// the reverse view is the Children scan (DINV-1).
func (s *Store) SetParent(ctx context.Context, childID, parentID int) (model.WorkItem, []schema.Defect) {
	const verb = "parent"
	if childID == parentID {
		return model.WorkItem{}, []schema.Defect{def(verb, "", "validate", fmt.Sprintf("item #%d cannot be its own parent", childID), "reference a different item as the parent")}
	}
	if _, defects := s.Get(ctx, parentID); len(defects) > 0 {
		return model.WorkItem{}, []schema.Defect{def(verb, itemPath(s.itemsDir(), parentID), "resolve", fmt.Sprintf("parent #%d does not exist", parentID), "reference an existing item as the parent")}
	}
	return s.writeParent(ctx, verb, childID, strconv.Itoa(parentID))
}

// ClearParent removes childID's parent, unparenting it (prd:local-work-tracker/
// FR-7). A missing or malformed child surfaces its own defect (UB-1); no other
// item is touched.
func (s *Store) ClearParent(ctx context.Context, childID int) (model.WorkItem, []schema.Defect) {
	return s.writeParent(ctx, "parent", childID, "")
}

// writeParent loads the child under the per-item lock, sets its parent field
// (empty clears it), and writes it back atomically, bumping updated. The child's
// stored `parent` is the only authoritative record of the edge (DINV-1); the
// reverse-parent index is maintained as a derived side effect once that write
// has committed (field-first, DINV-5) — the old-parent marker removed and the
// new one created, one file per edge (DINV-4). Setting the parent to the value
// it already holds is a recorded no-op, so no marker churns.
func (s *Store) writeParent(_ context.Context, verb string, childID int, parent string) (model.WorkItem, []schema.Defect) {
	var oldParent string
	return s.mutateThen(verb, childID,
		func(w *model.WorkItem) (bool, []schema.Defect) {
			if w.Parent == parent {
				return false, nil
			}
			oldParent = w.Parent
			w.Parent = parent
			return true, nil
		},
		func(w model.WorkItem) { s.reindexEdge(w.ID, oldParent, parent) },
	)
}

// Children returns parentID's direct children through the derived reverse-parent
// index (worktracker-hierarchy-index §3.1/§3.6, prd:local-work-tracker/FR-19,
// NFR-8): it lists the parent's markers — O(k) in the child count, never a
// full-store scan — and reads each named child's authoritative file (DINV-1). A
// marker naming a since-deleted or malformed child is dropped, quarantined not
// fatal (§3.13), and swept by a later reindex. In Decision mode each candidate's
// own `parent` is re-verified and a stale or hand-edited marker is dropped, so
// the index never decides, only finds (DINV-2). filter narrows by state, signal,
// and role exactly as List does; an empty state returns all states (FR-19). The
// result is creation-ordered (INV-12). A directory-level IO error is a defect; an
// absent index is built first from the item files (DINV-6, A-4).
func (s *Store) Children(ctx context.Context, parentID int, filter worktracker.ChildFilter, mode worktracker.ReadMode) ([]model.WorkItem, []schema.Defect) {
	const verb = "children"
	if filter.State != "" && !model.ValidState(filter.State) {
		return nil, []schema.Defect{def(verb, "", "validate", fmt.Sprintf("state %q is out of set", filter.State), "state is one of "+join(model.States))}
	}
	if filter.Role != "" && !model.ValidRole(filter.Role) {
		return nil, []schema.Defect{def(verb, "", "validate", fmt.Sprintf("role %q is out of set", filter.Role), "role is one of "+join(model.Roles))}
	}

	// A decision read must be COMPLETE, not just correct about present markers: a
	// hand-edit that added an edge without a marker would otherwise be missed. So
	// on the decision path, a tripped dirty check heals the index first — the sole
	// recovery, a full rebuild (DINV-6). Display never pays this, so its path stays
	// O(k) (§3.7, A-2). Present-but-stale markers are still dropped by the
	// re-verify below (DINV-2); this closes the missing-marker gap.
	if mode == worktracker.Decision && s.dirty() {
		if _, _, err := s.buildIndex(); err != nil {
			return nil, []schema.Defect{def(verb, s.revDir(), "reindex", err.Error(), "ensure the store root is writable so the index can be rebuilt")}
		}
	}

	ids, defects := s.childMarkers(verb, parentID)
	if len(defects) > 0 {
		return nil, defects
	}

	// A role filter needs the name→role map from the governed vocabulary — the
	// same resolution List does, so the two front doors filter identically.
	var roleOf map[string]model.Role
	if filter.Role != "" {
		sigs, sdefs := s.Signals(ctx)
		if len(sdefs) > 0 {
			return nil, stampVerb(verb, sdefs)
		}
		roleOf = make(map[string]model.Role, len(sigs))
		for _, sig := range sigs {
			roleOf[sig.Name] = sig.Role
		}
	}

	want := strconv.Itoa(parentID)
	var children []model.WorkItem
	for _, id := range ids {
		w, ds := s.Get(ctx, id)
		if len(ds) > 0 {
			continue // a marker naming a gone/malformed child: drop (swept by reindex)
		}
		if mode == worktracker.Decision && w.Parent != want {
			continue // stale/hand-edited marker: the index never decides (DINV-2)
		}
		if filter.State != "" && w.State != filter.State {
			continue
		}
		if filter.Signal != "" && !contains(w.Signals, filter.Signal) {
			continue
		}
		if filter.Role != "" && !hasSignalOfRole(w, roleOf, filter.Role) {
			continue
		}
		children = append(children, w)
	}
	sort.Slice(children, func(i, j int) bool { return children[i].ID < children[j].ID })
	return children, nil
}

// Rollup is the epic rollup — parentID's direct children grouped by open and
// closed (prd:local-work-tracker/FR-7, worktracker-hierarchy-index §3.1, A-7). It
// is computed over the one shared Children enumerator (DINV-7), so it inherits
// the O(k) index read and the display/decision read modes; grouping is the
// medium-neutral model.GroupChildren so every adapter groups identically. A
// completed epic surfaces its full closed task set here, not the empty open-only
// view the base rollup returned.
func (s *Store) Rollup(ctx context.Context, parentID int, mode worktracker.ReadMode) (worktracker.RollupView, []schema.Defect) {
	kids, defects := s.Children(ctx, parentID, worktracker.ChildFilter{}, mode)
	if len(defects) > 0 {
		return worktracker.RollupView{}, defects
	}
	open, closed := model.GroupChildren(kids)
	return worktracker.RollupView{Open: open, Closed: closed}, nil
}

// childMarkers lists the child ids recorded under parentID in the reverse-parent
// index, building the index first if it is absent — a fresh clone or a store
// after removal (DINV-6, A-4). An absent parent directory means no children, not
// an error. This is the O(k) heart of the query: it reads one directory bounded
// by the child count, never the whole store.
func (s *Store) childMarkers(verb string, parentID int) ([]int, []schema.Defect) {
	if !s.revExists() {
		if _, _, err := s.buildIndex(); err != nil {
			return nil, []schema.Defect{def(verb, s.revDir(), "reindex", err.Error(), "ensure the store root is writable so the index can be built")}
		}
	}
	dir := s.parentDir(strconv.Itoa(parentID))
	entries, err := s.readDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, []schema.Defect{def(verb, dir, "scan", err.Error(), "ensure the index is readable, or rebuild it with carina reindex")}
	}
	var ids []int
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if id, err := strconv.Atoi(e.Name()); err == nil && id > 0 {
			ids = append(ids, id)
		}
	}
	return ids, nil
}
