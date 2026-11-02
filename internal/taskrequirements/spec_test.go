package taskrequirements_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executiondemand"
	"github.com/agenticlab-ai/pyxis/internal/taskrequirements"
	"github.com/agenticlab-ai/pyxis/internal/taskrequirements/requirementstest"
)

func requirementsFixture(t *testing.T) *taskrequirements.Spec {
	t.Helper()
	s, err := taskrequirements.Parse([]byte(requirementstest.Input))
	if err != nil {
		t.Fatal(err)
	}
	s, err = s.Compile(taskrequirements.Sources(nil))
	if err != nil {
		t.Fatal(err)
	}
	return &s
}

func TestCompleteRoleProfilesAndProvenance(t *testing.T) {
	s := requirementsFixture(t)
	i, r := s.Implementation.Profile, s.Review.Profile
	if i.Capability != executiondemand.CapabilityStrong || i.Deliberation != executiondemand.DeliberationDeep || i.Context != executiondemand.ContextLarge || i.Assurance != executiondemand.AssuranceElevated {
		t.Fatalf("implementation: %+v", i)
	}
	if r.Capability != executiondemand.CapabilityFrontier || r.Deliberation != executiondemand.DeliberationMaximum || r.Context != executiondemand.ContextExtended || r.Assurance != executiondemand.AssuranceCritical {
		t.Fatalf("review: %+v", r)
	}
	for _, role := range []taskrequirements.Role{s.Implementation, s.Review} {
		if role.Profile.PolicyDigest == "" || role.Profile.PolicyVersion == "" || len(role.Profile.Applied) == 0 {
			t.Fatal("missing derivation provenance")
		}
		if !strings.Contains(role.Summary(), role.Rationale) || !strings.Contains(role.Summary(), "evidence=") {
			t.Fatal("incomplete human summary")
		}
	}
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	got, err := taskrequirements.Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	if err := got.Validate(taskrequirements.Sources(nil)); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(&got, s) {
		t.Fatal("round trip lost dimensions")
	}
}

func TestEveryAssessmentEnumCombinationDerivesAndRoundTrips(t *testing.T) {
	v := taskrequirements.Values()
	// Exercise the six-axis Cartesian product, not just example values.
	for _, r := range v.Reasoning {
		for _, c := range v.Coupling {
			for _, n := range v.Novelty {
				for _, ver := range v.Verification {
					for _, i := range v.ImpactScope {
						for _, a := range v.ArchitecturalRole {
							s := *requirementsFixture(t)
							s.Implementation.Assessment = executiondemand.Assessment{Reasoning: r, Coupling: c, Novelty: n, Verification: ver, ImpactScope: i, ArchitecturalRole: a, Citations: []executiondemand.Citation{{ID: "task:goal"}}}
							s.Implementation.Profile = nil
							s.Review = s.Implementation
							compiled, err := s.Compile(taskrequirements.Sources(nil))
							if err != nil {
								t.Fatal(err)
							}
							b, err := json.Marshal(compiled)
							if err != nil {
								t.Fatal(err)
							}
							got, err := taskrequirements.Parse(b)
							if err != nil {
								t.Fatal(err)
							}
							if err := got.Validate(taskrequirements.Sources(nil)); err != nil {
								t.Fatal(err)
							}
							if !reflect.DeepEqual(got, compiled) {
								t.Fatal("lost enum combination")
							}
						}
					}
				}
			}
		}
	}
}

func TestRejectMissingUnknownAndForgedRequirements(t *testing.T) {
	for _, axis := range []struct{ key, value string }{
		{"reasoning", "complex"}, {"coupling", "composed"}, {"novelty", "adapt"}, {"verification", "composed"}, {"impact_scope", "multi_surface"}, {"architectural_role", "shared_substrate"},
	} {
		for _, value := range []string{`"invalid"`, `""`, `null`, `42`} {
			input := strings.Replace(requirementstest.Input, `"`+axis.key+`":"`+axis.value+`"`, `"`+axis.key+`":`+value, 1)
			s, err := taskrequirements.Parse([]byte(input))
			if err == nil {
				_, err = s.Compile(taskrequirements.Sources(nil))
			}
			if err == nil {
				t.Fatalf("accepted %s=%s", axis.key, value)
			}
		}
		input := strings.Replace(requirementstest.Input, `"`+axis.key+`":"`+axis.value+`",`, "", 1)
		s, err := taskrequirements.Parse([]byte(input))
		if err == nil {
			_, err = s.Compile(taskrequirements.Sources(nil))
		}
		if err == nil {
			t.Fatalf("accepted missing %s", axis.key)
		}
	}
	for _, mutate := range []func(*taskrequirements.Spec){
		func(s *taskrequirements.Spec) { s.Schema = "unknown" },
		func(s *taskrequirements.Spec) { s.Review.Rationale = " " },
		func(s *taskrequirements.Spec) { s.Review.Assessment.Citations = nil },
		func(s *taskrequirements.Spec) { s.Review.Assessment.Citations[0].ID = "missing:source" },
		func(s *taskrequirements.Spec) { s.Review.Profile = nil },
		func(s *taskrequirements.Spec) { s.Implementation.Profile.Capability = "invalid" },
		func(s *taskrequirements.Spec) { s.Implementation.Profile.Deliberation = "invalid" },
		func(s *taskrequirements.Spec) { s.Implementation.Profile.Context = "" },
		func(s *taskrequirements.Spec) { s.Implementation.Profile.Assurance = executiondemand.AssuranceCritical },
		func(s *taskrequirements.Spec) { s.Implementation.Profile.PolicyDigest = "forged" },
		func(s *taskrequirements.Spec) { s.Implementation.Profile.Applied = nil },
	} {
		s := requirementsFixture(t)
		mutate(s)
		if err := s.Validate(taskrequirements.Sources(nil)); err == nil {
			t.Fatalf("accepted invalid requirements: %+v", s)
		}
	}
	for _, input := range []string{"{", requirementstest.Input + " {}", strings.Replace(requirementstest.Input, `"reasoning":`, `"model":"x","reasoning":`, 1), strings.Replace(requirementstest.Input, `"reasoning":`, `"reasoning":"routine","reasoning":`, 1)} {
		if _, err := taskrequirements.Parse([]byte(input)); err == nil {
			t.Fatal("accepted open/duplicate contract")
		}
	}
}

func TestTraceCitationsAndInputImmutability(t *testing.T) {
	s, err := taskrequirements.Parse([]byte(strings.ReplaceAll(requirementstest.Input, "task:acceptance", "prd:widget/FR-1")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Compile(taskrequirements.Sources(nil)); err == nil {
		t.Fatal("accepted undeclared trace")
	}
	compiled, err := s.Compile(taskrequirements.Sources([]string{"prd:widget/FR-1"}))
	if err != nil {
		t.Fatal(err)
	}
	if s.Implementation.Profile != nil || s.Review.Profile != nil {
		t.Fatal("compile changed the author's input")
	}
	if !strings.Contains(s.Implementation.Summary(), "reasoning=complex") {
		t.Fatal("missing uncompiled assessment")
	}
	if err := compiled.Validate(taskrequirements.Sources([]string{"prd:widget/FR-1"})); err != nil {
		t.Fatal(err)
	}
}
