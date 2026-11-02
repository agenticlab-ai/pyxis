package dialect

import (
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/plangate/plan"
)

func TestCoverage90MechanicalTaskAndDependencyRendering(t *testing.T) {
	m := Harbormaster()
	doc := &plan.Doc{Title: "Plan", Feature: "feature", Version: "001", Design: "design.md"}
	task := plan.Task{
		ID:         "T1",
		Title:      "Generate",
		Goal:       "Generate the artifact",
		Mechanical: true,
		Provenance: []string{"source-a", "source-b"},
		Acceptance: []string{"artifact matches"},
		Traces:     []string{"FR-1"},
		PlanTaskID: "feature/001/T1",
		Foundation: plan.Foundation{Level: plan.FoundationRoot, Reason: plan.ReasonRoot},
	}
	body := m.TaskBody(doc, task, []int{12})
	for _, want := range []string{"Provenance to verify", "source-a", "Blocked by #12", "band=0"} {
		if !strings.Contains(body, want) {
			t.Errorf("TaskBody missing %q: %s", want, body)
		}
	}
	if labels := m.TaskCreateLabels(task); len(labels) != 2 || labels[1] != m.MechLabel {
		t.Fatalf("TaskCreateLabels = %v", labels)
	}
	if got := (Manifest{Triggers: []string{""}}).Neutralize("unchanged"); got != "unchanged" {
		t.Fatalf("Neutralize short trigger = %q", got)
	}
}
