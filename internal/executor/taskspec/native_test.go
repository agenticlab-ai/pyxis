package taskspec

import (
	"github.com/agenticlab-ai/pyxis/internal/taskrequirements"
	"reflect"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/taskrequirements/requirementstest"
)

func nativeFixture(t *testing.T) Spec {
	t.Helper()
	r, err := taskrequirements.Parse([]byte(requirementstest.Input))
	if err != nil {
		t.Fatal(err)
	}
	r, err = r.Compile(taskrequirements.Sources(nil))
	if err != nil {
		t.Fatal(err)
	}
	return Spec{Goal: "Test @claude with --> and <markup>", Acceptance: []string{"criterion"}, Scope: "behavior", Type: "feat", Deps: []int{4}, DoD: "verified", Design: "docs/tech-design/example.md", Plan: "docs/plans/example/001-plan.md", PlanTaskID: "example/001/T1", PlanFingerprint: "sha256:source", Requirements: &r}
}

func TestNativeCarrierRoundTrip(t *testing.T) {
	want := nativeFixture(t)
	body, err := RenderNative(want)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(body, "@claude") || strings.Count(body, "-->") != 1 || strings.Contains(body, `"rung"`) {
		t.Fatal(body)
	}
	got, err := Parse([]string{body, body})
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, err %v", got, err)
	}

}

func TestNativeCarrierRejectsCorruption(t *testing.T) {
	body, err := RenderNative(nativeFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{
		body + "<!-- fs-spec:v3 {} -->",
		strings.Replace(body, `"schema_version":"task-spec/v2"`, `"schema_version":"task-spec/v3"`, 1),
		strings.Replace(body, `"goal":`, `"rung":2,"goal":`, 1),
		strings.Replace(body, `"goal":`, `"goal":"other","goal":`, 1),
		strings.Replace(body, `"plan_task_id":"example/001/T1"`, `"plan_task_id":""`, 1),
		strings.Replace(body, `"reasoning":"complex"`, `"reasoning":null`, 1),
		strings.TrimSuffix(body, " -->"),
	} {
		if _, err := Parse([]string{bad}); err == nil {
			t.Fatalf("accepted %s", bad)
		}
	}
	changed := strings.Replace(body, "complex", "different", 1)
	if _, err := Parse([]string{body, changed}); err == nil {
		t.Fatal("accepted conflicting native specs")
	}
	legacy := `<!-- fs-spec:v1 {"goal":"goal","acceptance":["done"]} -->`
	if _, err := Parse([]string{body, legacy}); err == nil {
		t.Fatal("accepted mixed native/legacy specs")
	}
}

func TestNativeCarrierRejectsInvalidExpectedSpec(t *testing.T) {
	if _, err := MatchNative([]string{"<!-- fs-spec:v2 {} -->"}, "invalid"); err == nil {
		t.Fatal("accepted invalid expected spec")
	}
}

func TestMatchNative(t *testing.T) {
	body, err := RenderNative(nativeFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name         string
		comments     []string
		present, bad bool
	}{
		{"absent", []string{"ordinary comment"}, false, false},
		{"exact", []string{body}, true, false},
		{"equal retry", []string{body, body}, true, false},
		{"future", []string{body, "<!-- fs-spec:v3 {} -->"}, false, true},
		{"tampered", []string{strings.Replace(body, "complex", "routine", 1)}, false, true},
		{"other task", []string{strings.Replace(body, "example/001/T1", "example/001/T2", 1)}, false, true},
		{"other source", []string{strings.Replace(body, "sha256:source", "sha256:other", 1)}, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			present, err := MatchNative(tc.comments, body)
			if present != tc.present || (err != nil) != tc.bad {
				t.Fatalf("present=%v error=%v", present, err)
			}
		})
	}
}

func TestNativeRoutingIdentity(t *testing.T) {
	s := nativeFixture(t)
	identity := DemandIdentity{Repository: "o/r", Task: "T1"}
	r, err := s.RoutingData(identity)
	if err != nil {
		t.Fatal(err)
	}
	if r.ID == "" || r.Requirements == nil {
		t.Fatalf("native was translated: %+v", r)
	}
	s.Requirements.Implementation.Rationale = "Changed assessment evidence"
	changed, err := s.RoutingData(identity)
	if err != nil || changed.ID == r.ID {
		t.Fatalf("native edit did not change identity: %+v %v", changed, err)
	}
	if _, err := s.RoutingData(DemandIdentity{Repository: "o/r", Task: "T2"}); err == nil {
		t.Fatal("accepted copied task identity")
	}
}

func TestRequirementsIdentityBindsCharterAndBothRoles(t *testing.T) {
	identity := DemandIdentity{Repository: "o/r", Task: "T1"}
	original, err := nativeFixture(t).RoutingData(identity)
	if err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*Spec){
		func(s *Spec) { s.Goal += " changed" },
		func(s *Spec) { s.Acceptance[0] += " changed" },
		func(s *Spec) { s.Scope += " changed" },
		func(s *Spec) { s.PlanFingerprint += " changed" },
		func(s *Spec) { s.Requirements.Implementation.Assessment.Novelty = "known" },
		func(s *Spec) { s.Requirements.Review.Assessment.Reasoning = "routine" },
		func(s *Spec) { s.Requirements.Review.Rationale += " changed" },
	} {
		s := nativeFixture(t)
		mutate(&s)
		s.Requirements.Implementation.Profile, s.Requirements.Review.Profile = nil, nil
		compiled, err := s.Requirements.Compile(taskrequirements.Sources(s.Traces))
		if err != nil {
			t.Fatal(err)
		}
		s.Requirements = &compiled
		got, err := s.RoutingData(identity)
		if err != nil || got.ID == original.ID {
			t.Fatalf("unbound task edit: %s %v", got.ID, err)
		}
	}
}
