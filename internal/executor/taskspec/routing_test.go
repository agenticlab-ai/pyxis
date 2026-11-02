package taskspec

import "testing"

func TestRoutingDataRequiresValidSourceAndRepository(t *testing.T) {
	s := nativeFixture(t)
	if _, err := s.RoutingData(DemandIdentity{}); err == nil {
		t.Fatal("accepted no repository")
	}
	s.Requirements.Schema = "invalid"
	if _, err := s.RoutingData(DemandIdentity{Repository: "o/r"}); err == nil {
		t.Fatal("accepted invalid route")
	}
	legacy := Spec{Goal: "behavior", Acceptance: []string{"verified"}, Plan: "docs/plans/example/001-plan.md"}
	if _, err := legacy.RoutingData(DemandIdentity{Repository: "o/r", Task: "T1"}); err == nil {
		t.Fatal("accepted task without requirements")
	}
	if _, err := legacy.RoutingData(DemandIdentity{}); err == nil {
		t.Fatal("accepted no legacy identity")
	}
}
