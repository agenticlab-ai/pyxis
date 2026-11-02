package localfs

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/worktracker/model"
)

// Dependencies are blocked_by edges (docs/tech-design/local-work-tracker.md
// §3.5, FR-8). The source item stores its blocked_by list; the reverse blocking
// view is a scan (DINV-1), so no forward→reverse index can drift. Adding a local
// edge runs a pre-write cycle check over the transitive blocked_by closure and
// refuses a cycle naming the full path (DINV-10), so readiness never deadlocks.
// A local target must exist (a dangling target is refused, UB-2); a malformed
// reference is refused naming it; the cross-repo owner/repo#id form is stored
// opaquely — never resolved, never cycle-checked (Q5). Mutations are idempotent
// — a present-add or an absent-remove is a recorded no-op that writes nothing
// (AP-22). Every refusal is fail-closed: nothing is written (AP-17).

// AddBlockedBy adds a blocked_by edge from item id to ref. A missing or
// malformed source surfaces its own defect (UB-1). The reference is classified:
// a malformed one is refused naming it; a local one must resolve to an existing
// item (a dangling target is refused, UB-2) and must not close a cycle (the
// cycle path is named, DINV-10); the cross-repo form is stored opaquely (Q5). An
// already-present edge is a recorded no-op returning the item unchanged (AP-22).
func (s *Store) AddBlockedBy(ctx context.Context, id int, ref string) (model.WorkItem, []schema.Defect) {
	const verb = "dep"
	w, defects := s.read(verb, id)
	if len(defects) > 0 {
		return model.WorkItem{}, defects
	}

	canon, kind, targetID := model.CanonicalRef(ref)
	switch kind {
	case model.RefMalformed:
		return model.WorkItem{}, []schema.Defect{def(verb, itemPath(s.itemsDir(), id), "validate",
			fmt.Sprintf("blocked_by reference %q is malformed", strings.TrimSpace(ref)),
			"a blocker is a positive item id (e.g. 12) or the cross-repo form owner/repo#id")}
	case model.RefLocal:
		if _, d := s.Get(ctx, targetID); len(d) > 0 {
			return model.WorkItem{}, []schema.Defect{def(verb, itemPath(s.itemsDir(), targetID), "resolve",
				fmt.Sprintf("blocker #%d does not exist", targetID),
				"reference an existing item as the blocker")}
		}
		if path := s.cyclePath(ctx, id, targetID); path != "" {
			return model.WorkItem{}, []schema.Defect{def(verb, itemPath(s.itemsDir(), id), "validate",
				fmt.Sprintf("adding #%d blocked_by #%d closes a cycle: %s", id, targetID, path),
				"redirect the edge so the blocked_by graph stays acyclic")}
		}
	case model.RefCrossRepo:
		// stored opaquely; no resolution, no cycle check (Q5).
	}

	if contains(w.BlockedBy, canon) {
		return w, nil // recorded no-op (AP-22)
	}
	w.BlockedBy = append(w.BlockedBy, canon)
	return s.rewriteItem(verb, w)
}

// RemoveBlockedBy removes a blocked_by edge from item id. A missing or malformed
// source surfaces its own defect (UB-1). Removal never resolves the target — a
// dangling or already-absent edge is removable — so an absent edge is a recorded
// no-op returning the item unchanged (AP-22).
func (s *Store) RemoveBlockedBy(ctx context.Context, id int, ref string) (model.WorkItem, []schema.Defect) {
	const verb = "dep"
	w, defects := s.read(verb, id)
	if len(defects) > 0 {
		return model.WorkItem{}, defects
	}
	canon, kind, _ := model.CanonicalRef(ref)
	if kind == model.RefMalformed {
		canon = strings.TrimSpace(ref) // match whatever bytes a hand-edit may hold
	}
	if !contains(w.BlockedBy, canon) {
		return w, nil // recorded no-op (AP-22)
	}
	w.BlockedBy = removeStr(w.BlockedBy, canon)
	return s.rewriteItem(verb, w)
}

// Blocking returns every item whose blocked_by names id, in creation (id) order,
// by scanning the store directly — no index or cache (DINV-1, DINV-4). It is the
// reverse of the stored blocked_by half. A file that fails to parse is
// quarantined (skipped) so one bad hand-edit cannot break the enumeration of its
// well-formed siblings (§3.13). Only a directory-level IO error is a defect.
func (s *Store) Blocking(ctx context.Context, id int) ([]model.WorkItem, []schema.Defect) {
	const verb = "blocking"
	entries, err := s.readDir(s.itemsDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, []schema.Defect{def(verb, s.itemsDir(), "scan", err.Error(), "ensure the store directory is readable")}
	}
	var blocking []model.WorkItem
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		otherID, ok := idFromName(e.Name())
		if !ok {
			continue
		}
		w, defects := s.Get(ctx, otherID)
		if len(defects) > 0 {
			continue // a malformed sibling is quarantined, not a blocking item
		}
		if blockedByLocal(w.BlockedBy, id) {
			blocking = append(blocking, w)
		}
	}
	sort.Slice(blocking, func(i, j int) bool { return blocking[i].ID < blocking[j].ID })
	return blocking, nil
}

// cyclePath reports whether adding source blocked_by target would close a cycle,
// returning the full cycle path rendered as `#a → #b → … → #a`, or "" if the
// edge is acyclic. It walks the existing blocked_by closure from target: if that
// walk can reach source, the new edge source→target completes the loop. Only
// local ids are traversed — a cross-repo or malformed edge is a leaf in the
// local graph (Q5). A self edge (target == source) is the shortest cycle and is
// named `#source → #source`.
func (s *Store) cyclePath(ctx context.Context, source, target int) string {
	back := s.reaches(ctx, target, source, map[int]bool{})
	if back == nil {
		return ""
	}
	// The cycle is source → target → … → source: the new edge, then the
	// existing path back that reaches source is what closes it.
	full := append([]int{source}, back...)
	parts := make([]string, len(full))
	for i, n := range full {
		parts[i] = "#" + strconv.Itoa(n)
	}
	return strings.Join(parts, " → ")
}

// reaches walks the local blocked_by edges from `from`, returning the path
// `[from … goal]` (inclusive) if goal is reachable, else nil. visited guards
// against looping on any pre-existing cycle in hand-edited data — the walk is
// still bounded even though the invariant it enforces forbids one. A node that
// fails to resolve stops that branch (its edges are not traversable).
func (s *Store) reaches(ctx context.Context, from, goal int, visited map[int]bool) []int {
	if from == goal {
		return []int{from}
	}
	if visited[from] {
		return nil
	}
	visited[from] = true
	w, defects := s.Get(ctx, from)
	if len(defects) > 0 {
		return nil
	}
	for _, ref := range w.BlockedBy {
		kind, next := model.ClassifyRef(ref)
		if kind != model.RefLocal {
			continue // cross-repo/malformed edges are leaves in the local graph (Q5)
		}
		if path := s.reaches(ctx, next, goal, visited); path != nil {
			return append([]int{from}, path...)
		}
	}
	return nil
}

// blockedByLocal reports whether refs contains the local id as a blocked_by
// edge — a cross-repo or malformed entry never matches a local id.
func blockedByLocal(refs []string, id int) bool {
	for _, ref := range refs {
		if kind, n := model.ClassifyRef(ref); kind == model.RefLocal && n == id {
			return true
		}
	}
	return false
}
