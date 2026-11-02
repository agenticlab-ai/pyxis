package executiondemand

// baselinePlanV2 is a structurally valid plan/v2 origin shared across this
// package's tests.
func baselinePlanV2() PlanV2Origin {
	return PlanV2Origin{
		Repository: "github.com/agenticlab-ai/pyxis",
		Plan:       PlanIdentity{Feature: "planner-driven-model-routing", Version: "001", Fingerprint: "planfingerprint1"},
		Task:       "planner-driven-model-routing/001/T2",
		Source:     "sha256:taskdigest1",
		Assessment: Assessment{
			Reasoning:         ReasoningComplex,
			Coupling:          CouplingComposed,
			Novelty:           NoveltyAdapt,
			Verification:      VerificationComposed,
			ImpactScope:       ImpactContained,
			ArchitecturalRole: RoleFeatureLocal,
			Citations:         []Citation{{ID: "spec#1"}, {ID: "spec#2"}},
		},
		Raise: &AssuranceRaise{Value: AssuranceElevated, Citations: []Citation{{ID: "spec#3"}}},
		Profile: Profile{
			Capability:    CapabilityStandard,
			Deliberation:  DeliberationDeep,
			Context:       ContextLarge,
			Assurance:     AssuranceElevated,
			Applied:       []AppliedRule{{ID: "compose-adapt", Kind: "promotion"}, {ID: "coupling-composed", Kind: "floor"}},
			PolicyVersion: "1.0.0",
			PolicyDigest:  "policydigesthash1",
		},
	}
}

func baselineOriginV2() Origin {
	p := baselinePlanV2()
	return Origin{Schema: OriginSchema, Kind: OriginPlanV2, PlanV2: &p}
}
