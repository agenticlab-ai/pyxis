package plan

import (
	"encoding/json"
	"github.com/agenticlab-ai/pyxis/internal/taskrequirements/requirementstest"
	"strings"
	"testing"
)

const nativeSpec = requirementstest.Input

func nativePlan() string {
	s := strings.ReplaceAll(goldenPlan, "plan-template/v1", NativeTemplateVersion)
	s = strings.ReplaceAll(s, DialectHarbormaster, DialectNative)
	for _, val := range []string{"rung=2", "rung=3", "rung=1", "—"} {
		s = strings.ReplaceAll(s, "| **Routing** | "+val+" |", "| **Requirements** | "+nativeSpec+" |")
	}
	return s
}

func TestNativePlanRoundTrip(t *testing.T) {
	d, defects := Parse("docs/plans/widget/001-plan.md", []byte(nativePlan()))
	if len(defects) != 0 {
		t.Fatal(defects)
	}
	if d.Schema != NativeSchemaVersion {
		t.Fatal(d.Schema)
	}
	b, err := d.JSON()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), `"rung"`) {
		t.Fatal("native plan emitted rung")
	}
	var decoded Doc
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatal(err)
	}
	for _, task := range decoded.Tasks {
		if task.Requirements == nil || task.Requirements.Implementation.Assessment.Reasoning != "complex" || task.Requirements.Review.Profile.Assurance != "critical" {
			t.Fatalf("lost routing: %+v", task)
		}
	}
}

func TestNativePlanRejectsIncompleteOrMixedRouting(t *testing.T) {
	for _, source := range []string{
		strings.Replace(nativePlan(), "| **Requirements** | "+nativeSpec+" |", "", 1),
		strings.Replace(nativePlan(), "| **Requirements** | "+nativeSpec+" |", "| **Routing** | rung=2 |", 1),
		strings.Replace(nativePlan(), `"reasoning":"complex"`, `"reasoning":""`, 1),
		strings.Replace(nativePlan(), "| **Requirements** | "+nativeSpec+" |", "| **Requirements** | "+nativeSpec+" |\n| **Requirements** | "+nativeSpec+" |", 1),
		strings.Replace(nativePlan(), NativeTemplateVersion, PlanTemplateVersion, 1),
	} {
		if _, defects := Parse("docs/plans/widget/001-plan.md", []byte(source)); len(defects) == 0 {
			t.Fatal("accepted invalid native source")
		}
	}
}
