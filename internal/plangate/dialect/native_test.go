package dialect

import (
	"github.com/agenticlab-ai/pyxis/internal/executor/taskspec"
	"github.com/agenticlab-ai/pyxis/internal/plangate/plan"
	"github.com/agenticlab-ai/pyxis/internal/taskrequirements"
	"github.com/agenticlab-ai/pyxis/internal/taskrequirements/requirementstest"
	"reflect"
	"strings"
	"testing"
)

func TestNativeProjectionPreservesRequirements(t *testing.T) {
	d, task := sampleDoc(t)
	d.Schema = plan.NativeSchemaVersion
	r, err := taskrequirements.Parse([]byte(requirementstest.Input))
	if err != nil {
		t.Fatal(err)
	}
	r, err = r.Compile(taskrequirements.Sources(nil))
	if err != nil {
		t.Fatal(err)
	}
	task.Requirements = &r
	body, err := Harbormaster().SpecComment(d, task, []int{12})
	if err != nil {
		t.Fatal(err)
	}
	s, err := taskspec.Parse([]string{body})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(s.Requirements, task.Requirements) || s.Goal != task.Goal || s.PlanTaskID != task.PlanTaskID || s.PlanFingerprint != d.Fingerprint {
		t.Fatalf("lost frozen data: %+v", s)
	}
	if strings.Contains(body, `"rung"`) {
		t.Fatal(body)
	}
	if !Harbormaster().HasSpec([]string{body}) {
		t.Fatal("native spec not recognized on retry")
	}
	face := Harbormaster().TaskBody(d, task, nil)
	for _, want := range []string{"reasoning=complex", "context=extended", "Independent <review>", "task:acceptance"} {
		if !strings.Contains(face, want) {
			t.Fatalf("human face missing %q", want)
		}
	}
}
