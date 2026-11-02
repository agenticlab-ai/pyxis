package port_test

import (
	"github.com/agenticlab-ai/pyxis/internal/executiondemand"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/taskspec/taskspectest"
	"testing"
)

func TestTaskReadsEachFrozenRoleWithoutFallback(t *testing.T) {
	s, err := taskspectest.Spec("T1")
	if err != nil {
		t.Fatal(err)
	}
	task := port.Task{ID: "T1", DemandID: "frozen-task", Requirements: s.Requirements}
	implementation, err := task.ImplementationDemand()
	if err != nil {
		t.Fatal(err)
	}
	review, err := task.ReviewDemand()
	if err != nil {
		t.Fatal(err)
	}
	if implementation.Capability != s.Requirements.Implementation.Profile.Capability || review.Capability != s.Requirements.Review.Profile.Capability || implementation == review {
		t.Fatalf("role profiles lost: %+v / %+v", implementation, review)
	}
	task.DemandProfile = implementation
	task.Requirements.Review.Profile = nil
	if _, err := task.ReviewDemand(); err == nil {
		t.Fatal("incomplete requirements fell back to implementation profile")
	}
	task.Requirements = nil
	if got, err := task.ReviewDemand(); err != nil || got != implementation {
		t.Fatalf("persisted obligation profile changed: %+v %v", got, err)
	}
	task.DemandProfile = executiondemand.ProfileMinimum{}
	if _, err := task.ReviewDemand(); err == nil {
		t.Fatal("missing profile acquired a default")
	}
}
