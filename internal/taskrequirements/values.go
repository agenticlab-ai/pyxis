package taskrequirements

import "github.com/agenticlab-ai/pyxis/internal/executiondemand"

// DimensionValues exposes typed authoring choices without loading a harness or
// probing credentials. These are the enums accepted by the derivation policy.
type DimensionValues struct {
	Schema            SchemaVersion                       `json:"schema_version"`
	Reasoning         []executiondemand.Reasoning         `json:"reasoning"`
	Coupling          []executiondemand.Coupling          `json:"coupling"`
	Novelty           []executiondemand.Novelty           `json:"novelty"`
	Verification      []executiondemand.Verification      `json:"verification"`
	ImpactScope       []executiondemand.ImpactScope       `json:"impact_scope"`
	ArchitecturalRole []executiondemand.ArchitecturalRole `json:"architectural_role"`
	Capability        []executiondemand.Capability        `json:"capability"`
	Deliberation      []executiondemand.Deliberation      `json:"deliberation"`
	Context           []executiondemand.Context           `json:"context"`
	Assurance         []executiondemand.Assurance         `json:"assurance"`
	Policy            executiondemand.DerivationPolicy    `json:"derivation_policy"`
}

func Values() DimensionValues {
	return DimensionValues{
		Schema:            Schema,
		Reasoning:         []executiondemand.Reasoning{executiondemand.ReasoningRoutine, executiondemand.ReasoningComplex, executiondemand.ReasoningDifficult},
		Coupling:          []executiondemand.Coupling{executiondemand.CouplingContained, executiondemand.CouplingComposed, executiondemand.CouplingCrossBoundary},
		Novelty:           []executiondemand.Novelty{executiondemand.NoveltyKnown, executiondemand.NoveltyAdapt, executiondemand.NoveltyFirstOfKind},
		Verification:      []executiondemand.Verification{executiondemand.VerificationLocal, executiondemand.VerificationComposed, executiondemand.VerificationCrossBoundary},
		ImpactScope:       []executiondemand.ImpactScope{executiondemand.ImpactContained, executiondemand.ImpactMultiSurface, executiondemand.ImpactSystemWide},
		ArchitecturalRole: []executiondemand.ArchitecturalRole{executiondemand.RoleFeatureLocal, executiondemand.RoleSharedSubstrate, executiondemand.RoleSystemCore},
		Capability:        []executiondemand.Capability{executiondemand.CapabilityEconomy, executiondemand.CapabilityStandard, executiondemand.CapabilityStrong, executiondemand.CapabilityFrontier},
		Deliberation:      []executiondemand.Deliberation{executiondemand.DeliberationLight, executiondemand.DeliberationStandard, executiondemand.DeliberationDeep, executiondemand.DeliberationMaximum},
		Context:           []executiondemand.Context{executiondemand.ContextStandard, executiondemand.ContextLarge, executiondemand.ContextExtended},
		Assurance:         []executiondemand.Assurance{executiondemand.AssuranceOrdinary, executiondemand.AssuranceElevated, executiondemand.AssuranceCritical},
		Policy:            executiondemand.DefaultPolicy,
	}
}
