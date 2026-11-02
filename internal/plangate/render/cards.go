// Task-card region — region 3 of the page (plan-viewer-v1 §3.6, plan T2). One
// anchored card per task carrying its complete §4.5 layer-2 charter — the
// contract the worker executes — so the reviewer reads it whole. Each card
// renders the charter prose and list fields, the task's traces, its identity
// (plan-task-id), and a badge row (mechanical/behavioral, size band, foundation,
// blocked-by, routing, size/overlap acceptances, expand–contract pattern).
// Every value is projected from the folded model and rendered as template text
// (auto-escaped): charter prose is never live markup (A-6/DINV-3). Touches is
// plan-internal overlap sequencing (§4.5) and never surfaces in a card.
package render

import (
	"fmt"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/plangate/plan"
)

// badge is one labeled chip on a card. Optional badges are appended only when
// their field is populated, so an empty optional field shows no chip.
type badge struct {
	Label string
	Value string
}

// cardView is one task's projection onto the card template — every charter
// field and computed badge, precomputed from the folded model so the template
// stays declarative and nothing re-parses the source (AP-6).
type cardView struct {
	ID            string
	Title         string
	Goal          string
	Current       string
	Desired       string
	Scope         []string
	OutOfScope    []string
	Acceptance    []string
	KeyInterfaces []string
	Provenance    []string
	Traces        []string
	Badges        []badge
	PlanTaskID    string
}

// cards projects every task onto a cardView in plan order.
func cards(doc *plan.Doc) []cardView {
	out := make([]cardView, 0, len(doc.Tasks))
	for i := range doc.Tasks {
		out = append(out, cardOf(&doc.Tasks[i]))
	}
	return out
}

// cardOf builds one task's card. The three always-present badges (kind, size
// band, foundation) render for every task; the rest are appended only when
// their charter field carries a value.
func cardOf(t *plan.Task) cardView {
	kind := "behavioral"
	if t.Mechanical {
		kind = "mechanical"
	}
	badges := []badge{
		{"kind", kind},
		{"size band", fmt.Sprintf("%d–%d", t.Band.Lo, t.Band.Hi)},
		{"foundation", t.Foundation.Level + " — " + t.Foundation.Reason},
	}
	if len(t.BlockedBy) > 0 {
		badges = append(badges, badge{"blocked by", strings.Join(t.BlockedBy, ", ")})
	}
	if t.Requirements != nil {
		for _, role := range []struct {
			name  string
			value string
		}{
			{"implementation", t.Requirements.Implementation.Summary()},
			{"review", t.Requirements.Review.Summary()},
		} {
			badges = append(badges, badge{role.name, role.value})
		}
	}
	if t.SizeAccepted != "" {
		badges = append(badges, badge{"size accepted", t.SizeAccepted})
	}
	if t.OverlapAccepted != "" {
		badges = append(badges, badge{"overlap accepted", t.OverlapAccepted})
	}
	if t.Pattern != "" {
		badges = append(badges, badge{"pattern", t.Pattern})
	}
	return cardView{
		ID:            t.ID,
		Title:         t.Title,
		Goal:          t.Goal,
		Current:       t.Current,
		Desired:       t.Desired,
		Scope:         t.Scope,
		OutOfScope:    t.OutOfScope,
		Acceptance:    t.Acceptance,
		KeyInterfaces: t.KeyInterfaces,
		Provenance:    t.Provenance,
		Traces:        t.Traces,
		Badges:        badges,
		PlanTaskID:    t.PlanTaskID,
	}
}
