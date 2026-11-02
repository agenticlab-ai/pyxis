// Package dialect renders frozen charters into an executor's native
// grammar (plan-new-v1 §3.9): one versioned rendering module + a data
// manifest + a contract test per executor. v1 ships exactly one — the
// predecessor fleet's. Grammar bytes live HERE and in the golden fixtures,
// never in frozen documents; a dialect only down-renders the charter
// schema, it never demands fields the schema lacks (DINV-9's shape).
package dialect

import (
	"fmt"
	"github.com/agenticlab-ai/pyxis/internal/executor/taskspec"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/plangate/plan"
	"github.com/agenticlab-ai/pyxis/internal/plangate/tracker"
)

// Label is one canonical signal the dialect emits: presentation is used at
// creation only — an existing label is never restyled.
type Label struct{ Name, Color, Description string }

// Manifest is the harbormaster dialect's data half.
type Manifest struct {
	Name          string   // plan.DialectHarbormaster
	SpecPrefix    string   // the versioned marker key readers probe for
	RoutingLabel  string   // makes a task dispatchable — applied last
	OperatorLabel string   // routes a task to a human, never a worker
	EpicLabels    []string // structural vocabulary on the epic issue
	TaskLabels    []string // structural vocabulary on every task
	MechLabel     string   // the mechanical-quarantine signal
	Triggers      []string // literal strings that fire tracker automations
	// OperatorSurfaces route a task to the operator when touched: the
	// fleet's App identity cannot push these (the predecessor's
	// workflow-permission dead-end incident).
	OperatorSurfaces []string
	// EpicType / TaskType are native issue-type names for list legibility
	// ("" = untyped). Data: pointing EpicType at a dedicated org "Epic"
	// type is a manifest edit, not code.
	EpicType string
	TaskType string
}

// EpicTitle prefixes the plan's epic title so the hierarchy reads in any
// list view, not only where native types render.
func (m Manifest) EpicTitle(d *plan.Doc) string {
	return "[Epic] " + d.EpicTitle
}

// TaskTitle carries the task's plan ID so ordering and membership read at
// a glance: "[T3] The task cards: …".
func (m Manifest) TaskTitle(t plan.Task) string {
	return "[" + t.ID + "] " + t.Title
}

// Harbormaster is the v1 manifest. Colors follow the predecessor's
// canonical presentation where known; they apply only at label creation.
func Harbormaster() Manifest {
	return Manifest{
		Name:             plan.DialectHarbormaster,
		SpecPrefix:       "fs-spec:v2",
		RoutingLabel:     "local-agent",
		OperatorLabel:    "operator",
		EpicLabels:       []string{"kind:feat", "epic"},
		TaskLabels:       []string{"kind:feat"},
		MechLabel:        "mechanical",
		Triggers:         []string{"@claude"},
		OperatorSurfaces: []string{".github/workflows/"},
		EpicType:         "Feature",
		TaskType:         "Task",
	}
}

// Labels enumerates every label this dialect may emit, with presentation.
func (m Manifest) Labels() []Label {
	return []Label{
		{m.RoutingLabel, "1D76DB", "Factory: eligible for autonomous agent pickup"},
		{m.OperatorLabel, "5319E7", "Factory: routed to the operator, never a worker"},
		{"kind:feat", "0E8A16", "Factory: feature work"},
		{"epic", "3E4B9E", "Factory: structural parent issue"},
		{m.MechLabel, "BFBFBF", "Factory: provenance-reviewed mechanical change"},
	}
}

// Routing returns the routing label for a task — the operator label when
// any plan-internal touched surface is one the fleet cannot push.
func (m Manifest) Routing(t plan.Task) string {
	for _, touch := range t.Touches {
		for _, s := range m.OperatorSurfaces {
			if strings.HasPrefix(strings.TrimPrefix(touch, "./"), s) {
				return m.OperatorLabel
			}
		}
	}
	return m.RoutingLabel
}

// StaleRouting returns the routing labels an issue carries that its task no
// longer routes to: a plan changed in place may move a task between the
// worker and the operator.
func (m Manifest) StaleRouting(t plan.Task, labels []string) []string {
	want := m.Routing(t)
	var stale []string
	for _, l := range []string{m.RoutingLabel, m.OperatorLabel} {
		if l != want && tracker.HasLabel(labels, l) {
			stale = append(stale, l)
		}
	}
	return stale
}

// Classification returns the labels to add to and remove from an issue so it
// carries the mechanical-quarantine signal exactly when its task is
// mechanical: a plan changed in place may reclassify a task.
func (m Manifest) Classification(t plan.Task, labels []string) (add, remove []string) {
	has := tracker.HasLabel(labels, m.MechLabel)
	switch {
	case t.Mechanical && !has:
		return []string{m.MechLabel}, nil
	case !t.Mechanical && has:
		return nil, []string{m.MechLabel}
	}
	return nil, nil
}

// TaskCreateLabels are the labels applied at task creation — structural
// vocabulary only; the routing signal is applied last (DINV-5).
func (m Manifest) TaskCreateLabels(t plan.Task) []string {
	labels := append([]string{}, m.TaskLabels...)
	if t.Mechanical {
		labels = append(labels, m.MechLabel)
	}
	return labels
}

// Neutralize renders trigger strings inert: a zero-width joiner after the
// sigil preserves the visual text while breaking the literal match.
func (m Manifest) Neutralize(s string) string {
	for _, trig := range m.Triggers {
		if len(trig) < 2 {
			continue
		}
		s = strings.ReplaceAll(s, trig, trig[:1]+"\u200d"+trig[1:])
	}
	return s
}

// PlanTaskMarker is the idempotency key's machine form in an issue body.
func PlanTaskMarker(planTaskID string) string {
	return fmt.Sprintf("<!-- pyxis:plan-task %s -->", planTaskID)
}

// PlanEpicMarker keys the plan's epic issue.
func PlanEpicMarker(name, version string) string {
	return fmt.Sprintf("<!-- pyxis:plan-epic %s/%s -->", name, version)
}

// TaskBody renders a task's human-readable issue body: the charter as
// prose plus the machine stamps — every byte derived from the frozen plan
// (DINV-2). Plan-internal metadata (Touches) is deliberately absent. The
// header line carries the plan's path beside the design's: a worker
// resolves both governing documents directly, no repo hunt (fleet
// transcripts showed workers grepping docs/plans/ for the bare reference).
func (m Manifest) TaskBody(d *plan.Doc, t plan.Task, blockers []int) string {
	var b strings.Builder
	if d.Intake == "" {
		fmt.Fprintf(&b, "**Plan:** %s (%s/%s · `%s`) · **Design:** %s\n\n", d.Title, d.Feature, d.Version, d.SourceFile(), d.Design)
	} else {
		fmt.Fprintf(&b, "**Plan:** %s (%s/%s · `%s`)\n\n", d.Title, d.Intake, d.Version, d.SourceFile())
	}
	if t.Requirements != nil {
		r := t.Requirements
		fmt.Fprintf(&b, "## Task requirements\n\n- Implementation: %s\n- Review: %s\n\n", r.Implementation.Summary(), r.Review.Summary())
	}
	fmt.Fprintf(&b, "## Goal\n\n%s\n\n", t.Goal)
	if t.Mechanical {
		b.WriteString("## Provenance to verify\n\n")
		for _, p := range t.Provenance {
			fmt.Fprintf(&b, "- %s\n", p)
		}
		b.WriteString("\n")
	} else {
		fmt.Fprintf(&b, "## Current behavior\n\n%s\n\n## Desired behavior\n\n%s\n\n", t.Current, t.Desired)
		b.WriteString("## Scope\n\n")
		for _, s := range t.Scope {
			fmt.Fprintf(&b, "- %s\n", s)
		}
		b.WriteString("\n## Out of scope\n\n")
		for _, s := range t.OutOfScope {
			fmt.Fprintf(&b, "- %s\n", s)
		}
		b.WriteString("\n## Key interfaces\n\n")
		for _, s := range t.KeyInterfaces {
			fmt.Fprintf(&b, "- %s\n", s)
		}
		b.WriteString("\n")
	}
	b.WriteString("## Acceptance criteria\n\n")
	for _, a := range t.Acceptance {
		fmt.Fprintf(&b, "- [ ] %s\n", a)
	}
	b.WriteString("\n## Traces\n\n")
	fmt.Fprintf(&b, "%s\n", strings.Join(t.Traces, " · "))
	if len(blockers) > 0 {
		b.WriteString("\n## Dependencies\n\n")
		for _, n := range blockers {
			fmt.Fprintf(&b, "- Blocked by #%d\n", n)
		}
	}
	fmt.Fprintf(&b, "\n%s\n", PlanTaskMarker(t.PlanTaskID))
	fmt.Fprintf(&b, "<!-- rig:foundation level=%s source=plan reason=%s -->\n", t.Foundation.Level, t.Foundation.Reason)
	band := "0"
	if !t.Mechanical {
		band = fmt.Sprintf("%d–%d", t.Band.Lo, t.Band.Hi)
	}
	fmt.Fprintf(&b, "<!-- rig:est_logic_lines band=%s source=plan -->\n", band)
	return m.Neutralize(b.String())
}

// EpicBody renders the plan's tracker face: the consumption contract (the
// frontier rule, verbatim per PRD FR-20), the frozen plan's citation, and
// the member-task list.
func (m Manifest) EpicBody(d *plan.Doc) string {
	var b strings.Builder
	if d.Intake == "" {
		fmt.Fprintf(&b, "**Plan:** `%s` (frozen at merge) · **Design:** %s · **PRD:** %s\n\n", d.SourceFile(), d.Design, d.PRD)
	} else {
		fmt.Fprintf(&b, "**Plan:** `%s` (frozen at merge) · **Intake:** %s\n\n", d.SourceFile(), d.Intake)
	}
	b.WriteString("Work the **frontier**: any task that is open, unblocked, and unclaimed.\n\n## Tasks\n\n")
	for _, t := range d.Tasks {
		fmt.Fprintf(&b, "- %s — %s\n", t.ID, t.Title)
	}
	fmt.Fprintf(&b, "\n%s\n", PlanEpicMarker(d.Name(), d.Version))
	return m.Neutralize(b.String())
}

// SpecComment renders the fs-spec charter comment for a task: a
// human-readable block and the one machine marker line the executor's
// spec-validity gate parses.
func (m Manifest) SpecComment(d *plan.Doc, t plan.Task, blockers []int) (string, error) {
	kind := "feat"
	if t.Mechanical {
		kind = "mechanical"
	}
	scope := strings.Join(t.Scope, "; ")
	if t.Mechanical {
		scope = strings.Join(t.Provenance, "; ")
	}
	if d.Schema == plan.NativeSchemaVersion {
		return taskspec.RenderNative(taskspec.Spec{Goal: t.Goal, Acceptance: t.Acceptance,
			Scope: scope, Type: kind, Deps: blockers, DoD: "every acceptance criterion verified",
			Design: d.Design, Plan: d.SourceFile(), PlanTaskID: t.PlanTaskID, PlanFingerprint: d.Fingerprint, Requirements: t.Requirements, Traces: t.Traces})
	}
	return "", fmt.Errorf("historical plans require replanning before projection")
}

// ParseSpecMarker extracts and decodes a spec marker from a comment body —
// the contract test's replica of the executor's spec-validity gate.
func (m Manifest) ParseSpecMarker(body string) (goal string, acceptance []string, ok bool) {
	s, err := taskspec.Parse([]string{body})
	return s.Goal, s.Acceptance, err == nil
}

// HasSpec reports whether any comment carries a structurally valid spec —
// the re-run probe that heals a crash between create and spec-comment.
func (m Manifest) HasSpec(comments []string) bool {
	for _, c := range comments {
		if goal, acc, ok := m.ParseSpecMarker(c); ok && goal != "" && len(acc) > 0 {
			return true
		}
	}
	return false
}
