package localfs

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/worktracker"
	"github.com/agenticlab-ai/pyxis/internal/worktracker/model"
)

// The discovery verbs — list/filter and full-text search — are direct scans of
// the item files, with no index on any path (docs/tech-design/
// local-work-tracker.md §3.6, DINV-4). Both read the authoritative files
// through the shared read path, so a query reflects every prior write; both
// return their matches in creation (id) order for a stable, fairly-rotating set
// (FR-10, INV-12). A hand-corrupted item is quarantined — skipped, not fatal —
// so one bad file cannot blind a query to its well-formed siblings (§3.13).

// List returns the items matching q — state and signal (by name or role) —
// bounded and stably sorted by creation order (prd:local-work-tracker/FR-10).
// An out-of-set state or role is refused before any scan (fail closed, AP-17,
// DINV-11). A role filter resolves each item's signal names against the store
// vocabulary; an empty query returns the whole backlog.
func (s *Store) List(ctx context.Context, q model.Query) ([]model.WorkItem, []schema.Defect) {
	const verb = "list"
	if q.State != "" && !model.ValidState(q.State) {
		return nil, []schema.Defect{def(verb, "", "validate", fmt.Sprintf("state %q is out of set", q.State), "state is one of "+join(model.States))}
	}
	if q.Role != "" && !model.ValidRole(q.Role) {
		return nil, []schema.Defect{def(verb, "", "validate", fmt.Sprintf("role %q is out of set", q.Role), "role is one of "+join(model.Roles))}
	}

	// A parent-scoped list is the reverse-parent query: route it through the one
	// shared O(k) children enumerator over the index, never a full-store scan
	// (worktracker-hierarchy-index §3.1, prd:local-work-tracker/FR-19, NFR-8). It
	// composes with the same state/signal/role filters below.
	if q.Parent != nil {
		return s.Children(ctx, *q.Parent, worktracker.ChildFilter{State: q.State, Signal: q.Signal, Role: q.Role}, worktracker.Display)
	}

	// A role filter needs the name→role map from the governed vocabulary; only
	// then does a scanned item's signal names resolve to roles.
	var roleOf map[string]model.Role
	if q.Role != "" {
		sigs, defects := s.Signals(ctx)
		if len(defects) > 0 {
			return nil, stampVerb(verb, defects)
		}
		roleOf = make(map[string]model.Role, len(sigs))
		for _, sig := range sigs {
			roleOf[sig.Name] = sig.Role
		}
	}

	items, defects := s.scanItems(verb)
	if len(defects) > 0 {
		return nil, defects
	}
	var out []model.WorkItem
	for _, w := range items {
		if q.State != "" && w.State != q.State {
			continue
		}
		if q.Signal != "" && !contains(w.Signals, q.Signal) {
			continue
		}
		if q.Role != "" && !hasSignalOfRole(w, roleOf, q.Role) {
			continue
		}
		out = append(out, w)
	}
	return out, nil
}

// Search returns every item — and only the items — whose title, body, or
// comments contain the query text, matched exactly and never fuzzily
// (prd:local-work-tracker/FR-11). Matching is case-insensitive literal
// containment: no stemming, ranking, or edit distance, so a near-miss token
// (a spelling variant a fuzzy matcher would catch) is not a substring and does
// not match — zero false positives. An empty query is refused (fail closed).
func (s *Store) Search(ctx context.Context, text string) ([]model.WorkItem, []schema.Defect) {
	const verb = "search"
	needle := strings.ToLower(strings.TrimSpace(text))
	if needle == "" {
		return nil, []schema.Defect{def(verb, "", "validate", "search text is empty", "supply text to search for, e.g. carina search substrate")}
	}
	items, defects := s.scanItems(verb)
	if len(defects) > 0 {
		return nil, defects
	}
	var out []model.WorkItem
	for _, w := range items {
		if matchesText(w, needle) {
			out = append(out, w)
		}
	}
	return out, nil
}

// matchesText reports whether needle (already lower-cased) is an exact
// substring of the item's title, body, or any comment payload — the searchable
// surface FR-11 names. Decision-record and transition log entries are audit
// state, not human text, so they are not searched.
func matchesText(w model.WorkItem, needle string) bool {
	if strings.Contains(strings.ToLower(w.Title), needle) || strings.Contains(strings.ToLower(w.Body), needle) {
		return true
	}
	for _, e := range w.Log {
		if e.Kind == model.LogComment && strings.Contains(strings.ToLower(e.Payload), needle) {
			return true
		}
	}
	return false
}

// hasSignalOfRole reports whether the item carries any signal whose role (per
// the store vocabulary) is want. An item's signal name absent from the
// vocabulary contributes no role — it cannot have been added (AddSignal refuses
// undefined names), so this only guards a hand-edited file.
func hasSignalOfRole(w model.WorkItem, roleOf map[string]model.Role, want model.Role) bool {
	for _, name := range w.Signals {
		if roleOf[name] == want {
			return true
		}
	}
	return false
}

// scanItems reads every well-formed item in the store directly (no index,
// DINV-4) and returns them in creation (id) order. A malformed item is
// quarantined (skipped) so one bad hand-edit cannot break a whole query
// (§3.13); only a directory-level IO error is a defect. An absent store is an
// empty backlog, not an error.
func (s *Store) scanItems(verb string) ([]model.WorkItem, []schema.Defect) {
	entries, err := s.readDir(s.itemsDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, []schema.Defect{def(verb, s.itemsDir(), "scan", err.Error(), "ensure the store directory is readable")}
	}
	var items []model.WorkItem
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		id, ok := idFromName(e.Name())
		if !ok {
			continue
		}
		w, defects := s.read(verb, id)
		if len(defects) > 0 {
			continue // a malformed item is quarantined, not fatal to the query
		}
		items = append(items, w)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return items, nil
}
