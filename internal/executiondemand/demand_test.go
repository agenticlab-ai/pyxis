package executiondemand

import "testing"

var testAssessment = Assessment{Reasoning: ReasoningComplex, Coupling: CouplingComposed, Novelty: NoveltyAdapt, Verification: VerificationComposed, ImpactScope: ImpactContained, ArchitecturalRole: RoleFeatureLocal, Citations: []Citation{{ID: "spec#1"}}}
var testEvidence = EvidenceSet{Resolved: map[string]struct{}{"spec#1": {}}}

func TestEvaluateAppliesFloorsPromotionAndRaise(t *testing.T) {
	p, err := Evaluate(testAssessment, testEvidence, AssuranceRaise{Value: AssuranceElevated, Citations: []Citation{{ID: "spec#1"}}}, DefaultPolicy)
	if err != nil {
		t.Fatal(err)
	}
	if p.Capability != CapabilityStandard || p.Deliberation != DeliberationDeep || p.Context != ContextLarge || p.Assurance != AssuranceElevated {
		t.Fatalf("unexpected profile: %+v", p)
	}
	if p.PolicyDigest == "" || len(p.Applied) != 6 {
		t.Fatalf("missing provenance: %+v", p)
	}
}

func TestEvaluateRejectsIncompleteAndLoweredInputs(t *testing.T) {
	bad := testAssessment
	bad.Novelty = ""
	if _, err := Evaluate(bad, testEvidence, AssuranceRaise{}, DefaultPolicy); err == nil {
		t.Fatal("missing axis accepted")
	}
	if _, err := Evaluate(testAssessment, EvidenceSet{}, AssuranceRaise{}, DefaultPolicy); err == nil {
		t.Fatal("missing evidence accepted")
	}
	high := testAssessment
	high.ImpactScope = ImpactMultiSurface
	if _, err := Evaluate(high, testEvidence, AssuranceRaise{Value: AssuranceOrdinary, Citations: []Citation{{ID: "spec#1"}}}, DefaultPolicy); err == nil {
		t.Fatal("assurance reduction accepted")
	}
}

func TestPolicyDigestIgnoresPolicyOrdering(t *testing.T) {
	p := DefaultPolicy
	p.Floors = append([]Floor(nil), p.Floors...)
	p.Promotions = append([]Promotion(nil), p.Promotions...)
	p.Floors[0], p.Floors[len(p.Floors)-1] = p.Floors[len(p.Floors)-1], p.Floors[0]
	a, err := Evaluate(testAssessment, testEvidence, AssuranceRaise{}, DefaultPolicy)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Evaluate(testAssessment, testEvidence, AssuranceRaise{}, p)
	if err != nil {
		t.Fatal(err)
	}
	if a.PolicyDigest != b.PolicyDigest {
		t.Fatal("policy digest depends on input order")
	}
}

func TestVerifyProfileRejectsLoweredProfile(t *testing.T) {
	p, err := Evaluate(testAssessment, testEvidence, AssuranceRaise{}, DefaultPolicy)
	if err != nil {
		t.Fatal(err)
	}
	p.Capability = CapabilityEconomy
	if err := VerifyProfile(testAssessment, testEvidence, AssuranceRaise{}, DefaultPolicy, p); err == nil {
		t.Fatal("lowered profile accepted")
	}
}

func TestPolicyRejectsDuplicateRuleIDs(t *testing.T) {
	p := DefaultPolicy
	p.Floors = append([]Floor(nil), p.Floors...)
	p.Floors = append(p.Floors, p.Floors[0])
	if _, err := Evaluate(testAssessment, testEvidence, AssuranceRaise{}, p); err == nil {
		t.Fatal("duplicate policy rule accepted")
	} else if d, ok := err.(Diagnostic); !ok || d.Code != "duplicate_policy_id" || d.Field != "floors[12].id" {
		t.Fatalf("unexpected diagnostic: %v", err)
	}
}
