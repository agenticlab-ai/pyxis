package forgejotracker

import (
	"encoding/json"
	"github.com/agenticlab-ai/pyxis/internal/executor/taskspec"
	"github.com/agenticlab-ai/pyxis/internal/taskrequirements"
	"github.com/agenticlab-ai/pyxis/internal/taskrequirements/requirementstest"
	"reflect"
	"testing"
)

func TestNativeFrontierPreservesExactSpec(t *testing.T) {
	r, err := taskrequirements.Parse([]byte(requirementstest.Input))
	if err != nil {
		t.Fatal(err)
	}
	r, err = r.Compile(taskrequirements.Sources(nil))
	if err != nil {
		t.Fatal(err)
	}
	spec := taskspec.Spec{Goal: "native task", Acceptance: []string{"done"}, Plan: "docs/plans/feature/001-plan.md", PlanTaskID: "feature/001/T1", PlanFingerprint: "source", Requirements: &r}
	body, err := taskspec.RenderNative(spec)
	if err != nil {
		t.Fatal(err)
	}

	tr, transport := newTestTracker(t, "http://127.0.0.1:3000", "local-agent", "feature/001")
	seedFrontier(t, transport)
	payload, err := json.Marshal([]map[string]any{{"id": 1, "body": body}})
	if err != nil {
		t.Fatal(err)
	}
	transport.responses["GET /api/v1/repos/o/r/issues/1/comments"] = string(payload)

	tasks, err := tr.Frontier(t.Context())
	if err != nil || len(tasks) != 1 {
		t.Fatalf("tasks=%+v err=%v", tasks, err)
	}
	task := tasks[0]
	if !reflect.DeepEqual(task.Requirements, spec.Requirements) || task.NativeRouting != nil || task.DemandID == "" || task.DemandProfile.Capability != "" {
		t.Fatalf("native task lost or translated: %+v", task)
	}
}
