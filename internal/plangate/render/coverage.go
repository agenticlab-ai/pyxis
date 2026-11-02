// Coverage region — region 4 of the page (plan-viewer-v1 §3.6, plan T3). The
// plan's coverage matrix rendered so the reviewer can walk from a governing
// requirement to the task cards that serve it: every covered row lists its
// task references as in-page links to their `id="T<n>"` cards (A-4), and every
// deferred row — a non-Must requirement deliberately not planned — renders with
// a distinct marking and its reason, never silently dropped (PRD FR-4/FR-15).
// Values are projected from the folded model and rendered as template text
// (auto-escaped); nothing re-parses the source (AP-6).
package render

import "github.com/agenticlab-ai/pyxis/internal/plangate/plan"

// covRow is one row of the coverage region — either a covered requirement (its
// serving task IDs, each linked to a card) or a deferred requirement (its
// reason, distinctly marked). The two shapes share a row so the region reads as
// one matrix; Deferred selects the rendering.
type covRow struct {
	Requirement string
	Tasks       []string // covered rows: serving task IDs, each an in-page card link
	Deferred    bool     // true → deferred row (Reason, distinct marking)
	Reason      string   // deferred rows: why it is not planned here
}

// coverage projects the folded model's coverage matrix onto rows in reading
// order: every covered row first (plan order), then every deferred row. Each
// covered row's task references resolve to the card anchors the task-card
// region emits (id="T<n>"), so the matrix is navigable in-page (A-4).
func coverage(doc *plan.Doc) []covRow {
	out := make([]covRow, 0, len(doc.Coverage)+len(doc.Deferred))
	for _, r := range doc.Coverage {
		out = append(out, covRow{Requirement: r.Requirement, Tasks: r.Tasks})
	}
	for _, r := range doc.Deferred {
		out = append(out, covRow{Requirement: r.Requirement, Deferred: true, Reason: r.Reason})
	}
	return out
}
