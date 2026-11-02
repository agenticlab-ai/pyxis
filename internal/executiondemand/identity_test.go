package executiondemand

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

func TestIdentifyIsStableAcrossRepeatedCalls(t *testing.T) {
	o := baselineOriginV2()
	id1, err := Identify(o)
	if err != nil {
		t.Fatal(err)
	}
	for range 5 {
		id2, err := Identify(o)
		if err != nil {
			t.Fatal(err)
		}
		if id1 != id2 {
			t.Fatalf("demand id unstable across repeated calls (retry/role simulation): %q != %q", id1, id2)
		}
	}
}

func TestIdentifyIsOrderIndependentForCitationsAndAppliedRules(t *testing.T) {
	base := baselinePlanV2()
	reordered := baselinePlanV2()
	reordered.Assessment.Citations = []Citation{{ID: "spec#2"}, {ID: "spec#1"}}
	reordered.Profile.Applied = []AppliedRule{{ID: "coupling-composed", Kind: "floor"}, {ID: "compose-adapt", Kind: "promotion"}}

	id1, err := Identify(Origin{Schema: OriginSchema, Kind: OriginPlanV2, PlanV2: &base})
	if err != nil {
		t.Fatal(err)
	}
	id2, err := Identify(Origin{Schema: OriginSchema, Kind: OriginPlanV2, PlanV2: &reordered})
	if err != nil {
		t.Fatal(err)
	}
	if id1 != id2 {
		t.Fatal("reordering citations/applied rules changed the demand id")
	}
}

func TestIdentifyIsLowercaseHex(t *testing.T) {
	id, err := Identify(baselineOriginV2())
	if err != nil {
		t.Fatal(err)
	}
	if len(id) != 64 {
		t.Fatalf("demand id %q has unexpected length %d, want 64 (sha256 hex)", id, len(id))
	}
	for _, r := range string(id) {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			t.Fatalf("demand id %q is not lowercase hex", id)
		}
	}
}

func TestIdentifyRejectsInvalidOrigin(t *testing.T) {
	p := baselinePlanV2()
	p.Assessment.Citations = nil
	if _, err := Identify(Origin{Schema: OriginSchema, Kind: OriginPlanV2, PlanV2: &p}); err == nil {
		t.Fatal("invalid origin produced a demand id")
	}
}

func TestIdentifyChangesWithEveryIdentityBearingMutationPlanV2(t *testing.T) {
	baseline, err := Identify(baselineOriginV2())
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		mutate func(*PlanV2Origin)
	}{
		{"repository", func(p *PlanV2Origin) { p.Repository = "other/repo" }},
		{"plan_feature", func(p *PlanV2Origin) { p.Plan.Feature = "other-feature" }},
		{"plan_version", func(p *PlanV2Origin) { p.Plan.Version = "002" }},
		{"plan_fingerprint", func(p *PlanV2Origin) { p.Plan.Fingerprint = "other-fingerprint" }},
		{"task", func(p *PlanV2Origin) { p.Task = "other-feature/001/T9" }},
		{"source_digest", func(p *PlanV2Origin) { p.Source = "sha256:other" }},
		{"assessment_reasoning", func(p *PlanV2Origin) { p.Assessment.Reasoning = ReasoningDifficult }},
		{"assessment_coupling", func(p *PlanV2Origin) { p.Assessment.Coupling = CouplingCrossBoundary }},
		{"assessment_novelty", func(p *PlanV2Origin) { p.Assessment.Novelty = NoveltyFirstOfKind }},
		{"assessment_verification", func(p *PlanV2Origin) { p.Assessment.Verification = VerificationCrossBoundary }},
		{"assessment_impact_scope", func(p *PlanV2Origin) { p.Assessment.ImpactScope = ImpactSystemWide }},
		{"assessment_architectural_role", func(p *PlanV2Origin) { p.Assessment.ArchitecturalRole = RoleSystemCore }},
		{"citation_added", func(p *PlanV2Origin) {
			p.Assessment.Citations = append(p.Assessment.Citations, Citation{ID: "spec#99"})
		}},
		{"applied_rule_added", func(p *PlanV2Origin) {
			p.Profile.Applied = append(p.Profile.Applied, AppliedRule{ID: "extra-rule", Kind: "floor"})
		}},
		{"raise_removed", func(p *PlanV2Origin) { p.Raise = nil }},
		{"raise_value_changed", func(p *PlanV2Origin) {
			p.Raise = &AssuranceRaise{Value: AssuranceCritical, Citations: []Citation{{ID: "spec#3"}}}
		}},
		{"raise_citation_changed", func(p *PlanV2Origin) {
			p.Raise = &AssuranceRaise{Value: AssuranceElevated, Citations: []Citation{{ID: "spec#other"}}}
		}},
		{"policy_version", func(p *PlanV2Origin) { p.Profile.PolicyVersion = "2.0.0" }},
		{"policy_digest", func(p *PlanV2Origin) { p.Profile.PolicyDigest = "different-digest" }},
		{"final_profile_capability", func(p *PlanV2Origin) { p.Profile.Capability = CapabilityFrontier }},
		{"final_profile_deliberation", func(p *PlanV2Origin) { p.Profile.Deliberation = DeliberationMaximum }},
		{"final_profile_context", func(p *PlanV2Origin) { p.Profile.Context = ContextExtended }},
		{"final_profile_assurance", func(p *PlanV2Origin) { p.Profile.Assurance = AssuranceCritical }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := baselinePlanV2()
			tc.mutate(&p)
			id, err := Identify(Origin{Schema: OriginSchema, Kind: OriginPlanV2, PlanV2: &p})
			if err != nil {
				t.Fatalf("mutated origin rejected: %v", err)
			}
			if id == baseline {
				t.Fatalf("mutating %s did not change the demand id", tc.name)
			}
		})
	}
}

// goldenFixture pins EncodeCanonical/Identify output for a fixed input so an
// unintentional codec or identity-derivation drift is caught (execution-
// demand codec fixtures, plan T2 Touches).
type goldenFixture struct {
	CanonicalHex string `json:"canonical_hex"`
	DemandID     string `json:"demand_id"`
}

func TestGoldenPlanV2(t *testing.T) {
	testGolden(t, "testdata/golden_plan_v2.json", baselineOriginV2())
}

func testGolden(t *testing.T, path string, o Origin) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var want goldenFixture
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}
	b, err := EncodeCanonical(o)
	if err != nil {
		t.Fatal(err)
	}
	if got := hex.EncodeToString(b); got != want.CanonicalHex {
		t.Fatalf("canonical encoding drifted from golden fixture %s:\ngot:  %s\nwant: %s", path, got, want.CanonicalHex)
	}
	id, err := Identify(o)
	if err != nil {
		t.Fatal(err)
	}
	if string(id) != want.DemandID {
		t.Fatalf("demand id drifted from golden fixture %s: got %s want %s", path, id, want.DemandID)
	}
}
