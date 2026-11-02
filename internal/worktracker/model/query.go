package model

import "strconv"

// The query surface — the parameters a list takes and the schema-versioned
// machine projections both discovery verbs emit (docs/tech-design/
// local-work-tracker.md §3.6, prd:local-work-tracker/FR-10, FR-11, FR-15). The
// types are medium-neutral data: the localfs adapter scans the store and the
// engine consumes the same shapes through the port, so list and search can
// never disagree across the two front doors.

// Query is the list filter — state and signal, the discovery primitive under
// readiness and triage (FR-10). Each field is optional: an empty field imposes
// no constraint, and set fields combine with AND. Signal filters by a signal's
// *name*; Role filters by a signal's *role* (an item matches a role filter when
// it carries any signal of that role) — the routing/authorization selection is
// how the factory-implementable set is named (FR-6, §3.5). The result is always
// stably ordered by creation (id) so a scheduling pass rotates fairly (INV-12).
type Query struct {
	State  State  // "" = any state
	Signal string // "" = any signal name
	Role   Role   // "" = any role
	Parent *int   // nil = no parent filter; set = only the direct children of Parent (FR-19)
}

// ListSchemaVersion and SearchSchemaVersion tag the machine projections of the
// two discovery verbs (FR-15; AP-6 — cross-boundary data is version-pinned).
// Additive evolution only; the two verbs carry distinct tags so a consumer pins
// exactly the shape it reads.
const (
	ListSchemaVersion   = "worktracker.work-item-list/v1"
	SearchSchemaVersion = "worktracker.work-item-search/v1"
)

// ListView is the schema-versioned machine projection of a list result — the
// pinned schema tag, the match count, and the matched items as their full
// ItemViews in creation order (FR-15). Items is always present (never null) so
// a consumer reads a stable shape even for an empty result.
type ListView struct {
	Schema string     `json:"schema"`
	Query  QueryView  `json:"query"`
	Count  int        `json:"count"`
	Items  []ItemView `json:"items"`
}

// QueryView echoes the filter a list ran under, so a machine consumer can pin a
// result to its request. Empty fields render as empty strings.
type QueryView struct {
	State  string `json:"state"`
	Signal string `json:"signal"`
	Role   string `json:"role"`
	Parent string `json:"parent,omitempty"` // the parent filter, when set (FR-19); omitted otherwise
}

// SearchView is the schema-versioned machine projection of a search result —
// the pinned schema tag, the echoed query text, the match count, and the
// matched items as full ItemViews in creation order (FR-15).
type SearchView struct {
	Schema string     `json:"schema"`
	Query  string     `json:"query"`
	Count  int        `json:"count"`
	Items  []ItemView `json:"items"`
}

// ProjectList projects a filtered, creation-ordered item set and the query it
// ran under onto the schema-versioned list view.
func ProjectList(q Query, items []WorkItem) ListView {
	return ListView{
		Schema: ListSchemaVersion,
		Query:  QueryView{State: string(q.State), Signal: q.Signal, Role: string(q.Role), Parent: parentView(q.Parent)},
		Count:  len(items),
		Items:  itemViews(items),
	}
}

// parentView renders a parent filter for the machine echo — the id as a string,
// or "" when no parent filter was set (so QueryView.Parent stays omitted).
func parentView(parent *int) string {
	if parent == nil {
		return ""
	}
	return strconv.Itoa(*parent)
}

// ProjectSearch projects a matched, creation-ordered item set and the query
// text onto the schema-versioned search view.
func ProjectSearch(query string, items []WorkItem) SearchView {
	return SearchView{
		Schema: SearchSchemaVersion,
		Query:  query,
		Count:  len(items),
		Items:  itemViews(items),
	}
}

// itemViews projects a slice of items onto their machine views, always
// returning a non-nil slice so the collection renders as [] when empty.
func itemViews(items []WorkItem) []ItemView {
	out := make([]ItemView, 0, len(items))
	for _, w := range items {
		out = append(out, w.View())
	}
	return out
}
