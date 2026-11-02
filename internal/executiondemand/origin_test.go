package executiondemand

import (
	"reflect"
	"testing"
)

func TestOriginValidateAcceptsBaselines(t *testing.T) {
	if err := baselineOriginV2().validate(); err != nil {
		t.Fatalf("valid plan_v2 origin rejected: %v", err)
	}
}

func TestOriginValidateRejectsUnknownSchema(t *testing.T) {
	o := baselineOriginV2()
	o.Schema = "execution-demand/v3"
	if err := o.validate(); err == nil {
		t.Fatal("unknown schema accepted")
	}
}

func TestOriginValidateRejectsUnknownKind(t *testing.T) {
	o := baselineOriginV2()
	o.Kind = "plan_v3"
	if err := o.validate(); err == nil {
		t.Fatal("unknown kind accepted")
	}
}

func TestOriginValidateRejectsInconsistentUnion(t *testing.T) {
	missing := baselineOriginV2()
	missing.PlanV2 = nil
	if err := missing.validate(); err == nil {
		t.Fatal("origin missing its declared member accepted")
	}
}

func TestPlanV2OriginValidateRejectsMissingIdentity(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*PlanV2Origin)
	}{
		{"repository", func(p *PlanV2Origin) { p.Repository = "" }},
		{"plan_feature", func(p *PlanV2Origin) { p.Plan.Feature = "" }},
		{"plan_version", func(p *PlanV2Origin) { p.Plan.Version = "" }},
		{"plan_fingerprint", func(p *PlanV2Origin) { p.Plan.Fingerprint = "" }},
		{"task", func(p *PlanV2Origin) { p.Task = "" }},
		{"source_digest", func(p *PlanV2Origin) { p.Source = "" }},
		{"policy_version", func(p *PlanV2Origin) { p.Profile.PolicyVersion = "" }},
		{"policy_digest", func(p *PlanV2Origin) { p.Profile.PolicyDigest = "" }},
		{"profile_capability", func(p *PlanV2Origin) { p.Profile.Capability = "" }},
		{"citations", func(p *PlanV2Origin) { p.Assessment.Citations = nil }},
		{"unknown_reasoning", func(p *PlanV2Origin) { p.Assessment.Reasoning = "bogus" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := baselinePlanV2()
			tc.mutate(&p)
			if err := p.validate(); err == nil {
				t.Fatalf("missing/invalid %s accepted", tc.name)
			}
		})
	}
}

func TestPlanV2OriginValidateRejectsDuplicateAndEmptyCitations(t *testing.T) {
	p := baselinePlanV2()
	p.Assessment.Citations = []Citation{{ID: "spec#1"}, {ID: "spec#1"}}
	if err := p.validate(); err == nil {
		t.Fatal("duplicate citation ids accepted")
	}

	p2 := baselinePlanV2()
	p2.Assessment.Citations = []Citation{{ID: ""}}
	if err := p2.validate(); err == nil {
		t.Fatal("empty citation id accepted")
	}
}

func TestPlanV2OriginValidateRejectsBadRaise(t *testing.T) {
	p := baselinePlanV2()
	p.Raise = &AssuranceRaise{Value: "bogus", Citations: []Citation{{ID: "spec#1"}}}
	if err := p.validate(); err == nil {
		t.Fatal("unknown raise value accepted")
	}

	p2 := baselinePlanV2()
	p2.Raise = &AssuranceRaise{Value: AssuranceElevated}
	if err := p2.validate(); err == nil {
		t.Fatal("raise without citations accepted")
	}
}

// TestOriginShapeHasNoDisallowedFields pins the exact allowed field set on
// each origin type. Design §3.6/A-7 forbid tracker text, proposal text,
// timestamps, runtime attempts, current aliases, harnesses, and native
// launch settings from ever entering the origin; since none of those has a
// field here today, this test forces a future field addition to update this
// allow-list deliberately rather than silently widening the schema.
func TestOriginShapeHasNoDisallowedFields(t *testing.T) {
	assertFields(t, PlanV2Origin{}, "Repository", "Plan", "Task", "Source", "Assessment", "Raise", "Profile")
	assertFields(t, Origin{}, "Schema", "Kind", "PlanV2")
}

func assertFields(t *testing.T, v any, allowed ...string) {
	t.Helper()
	want := map[string]bool{}
	for _, f := range allowed {
		want[f] = true
	}
	typ := reflect.TypeOf(v)
	if typ.NumField() != len(allowed) {
		t.Fatalf("%s: got %d fields, want %d %v", typ.Name(), typ.NumField(), len(allowed), allowed)
	}
	for field := range typ.Fields() {
		name := field.Name
		if !want[name] {
			t.Fatalf("%s: unexpected field %q", typ.Name(), name)
		}
	}
}
