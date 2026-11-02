// Flags region — region 5 of the page (plan-viewer-v1 §3.6, plan T3). The one
// place every recorded acceptance aggregates: each size-cap or overlap
// acceptance a task carries surfaces here with its task (linked to the card)
// and reason, so a reviewer sees the plan's whole waiver surface at a glance
// (PRD FR-5, A-5). The panel lists exactly the machine face's acceptances —
// no more (an empty panel means none were recorded), no fewer. Values are
// projected from the folded model and rendered as template text (auto-escaped).
package render

import "github.com/agenticlab-ai/pyxis/internal/plangate/plan"

// flagView is one recorded acceptance: which task carries it, whether it waives
// the size band or a task overlap, and the recorded reason.
type flagView struct {
	Task   string // the task ID that carries the acceptance (linked to its card)
	Kind   string // "size" or "overlap"
	Reason string // the recorded justification
}

// flags aggregates every recorded size/overlap acceptance across the folded
// model in task order — size before overlap within a task. A task with neither
// contributes nothing, so the panel's length is exactly the count of recorded
// acceptances (A-5).
func flags(doc *plan.Doc) []flagView {
	var out []flagView
	for i := range doc.Tasks {
		t := &doc.Tasks[i]
		if t.SizeAccepted != "" {
			out = append(out, flagView{Task: t.ID, Kind: "size", Reason: t.SizeAccepted})
		}
		if t.OverlapAccepted != "" {
			out = append(out, flagView{Task: t.ID, Kind: "overlap", Reason: t.OverlapAccepted})
		}
	}
	return out
}
