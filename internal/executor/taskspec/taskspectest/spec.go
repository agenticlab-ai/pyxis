// Package taskspectest supplies current task carriers for adapter and execution tests.
package taskspectest

import (
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/taskspec"
	"github.com/agenticlab-ai/pyxis/internal/taskrequirements"
	"github.com/agenticlab-ai/pyxis/internal/taskrequirements/requirementstest"
)

// Spec builds a complete persisted contract with independent role requirements.
func Spec(id string) (taskspec.Spec, error) {
	return spec(id, requirementstest.Input)
}

func spec(id, requirements string) (taskspec.Spec, error) {
	r, err := taskrequirements.Parse([]byte(requirements))
	if err != nil {
		return taskspec.Spec{}, err
	}
	r, err = r.Compile(taskrequirements.Sources(nil))
	if err != nil {
		return taskspec.Spec{}, err
	}
	return taskspec.Spec{Goal: "build it", Acceptance: []string{"it is green"}, PlanTaskID: "fixture/001/" + id, PlanFingerprint: "sha256:fixture", Requirements: &r}, nil
}

func Marker(t testing.TB, id string) string {
	t.Helper()
	return marker(t, id, requirementstest.Input)
}

func marker(t testing.TB, id, requirements string) string {
	t.Helper()
	s, err := spec(id, requirements)
	if err != nil {
		t.Fatal(err)
	}
	body, err := taskspec.RenderNative(s)
	if err != nil {
		t.Fatal(err)
	}
	return body
}
