// This file defines the closed execution-demand/v2 origin envelope: a
// versioned plan/v2 shape that carries a
// frozen task's execution demand across repository, plan, tracker, and
// review boundaries (design: execution-demand-contract-v1 §3.1).
package executiondemand

import "fmt"

// OriginSchema is the closed schema/version tag every accepted Origin must
// carry. It has no successor within this design; a semantic change to the
// origin shape requires a superseding design and a new schema value.
const OriginSchema = "execution-demand/v2"

// OriginKind is the closed tag selecting which origin member is populated.
type OriginKind string

const (
	// OriginPlanV2 tags a frozen model-backed assessment and its
	// mechanically derived profile.
	OriginPlanV2 OriginKind = "plan_v2"
)

// RepositoryIdentity names the repository an origin was frozen in.
type RepositoryIdentity string

// TaskIdentity is the plan-task idempotency key, e.g. "<feature>/<NNN>/<Tn>".
type TaskIdentity string

// SourceDigest is the frozen task's source-content digest.
type SourceDigest string

// DemandID is the lowercase hex digest identifying one execution demand
// (design §3.3). It is a pure function of an Origin's identity-bearing
// fields — see Identify.
type DemandID string

// PlanIdentity names the exact frozen plan document an origin was read
// from: which document (Feature, Version) and that it is exactly this
// frozen content (Fingerprint).
type PlanIdentity struct {
	Feature     string
	Version     string
	Fingerprint string
}

func (p PlanIdentity) validate() error {
	if p.Feature == "" || p.Version == "" || p.Fingerprint == "" {
		return Diagnostic{"plan", "missing_identity", "plan identity requires feature, version, and fingerprint"}
	}
	return nil
}

// PlanV2Origin is the plan/v2 member: a frozen model-backed assessment and
// its mechanically derived profile (design §3.1). Every field here is
// identity-bearing — see Identify.
type PlanV2Origin struct {
	Repository RepositoryIdentity
	Plan       PlanIdentity
	Task       TaskIdentity
	Source     SourceDigest
	Assessment Assessment
	// Raise is the optional planner assurance raise. Nil means no raise was
	// made; a raise may only increase assurance (enforced by Evaluate/T1).
	Raise   *AssuranceRaise
	Profile Profile
}

func (p PlanV2Origin) validate() error {
	if p.Repository == "" {
		return Diagnostic{"repository", "missing_identity", "repository identity is required"}
	}
	if err := p.Plan.validate(); err != nil {
		return err
	}
	if p.Task == "" {
		return Diagnostic{"task", "missing_identity", "task identity is required"}
	}
	if p.Source == "" {
		return Diagnostic{"source_digest", "missing_identity", "task source digest is required"}
	}
	if err := p.Assessment.validate(); err != nil {
		return err
	}
	if err := validateCitationShape(p.Assessment.Citations, "assessment.citations"); err != nil {
		return err
	}
	if err := validateProfileComplete(profileMinimumOf(p.Profile), "profile"); err != nil {
		return err
	}
	if p.Profile.PolicyVersion == "" || p.Profile.PolicyDigest == "" {
		return Diagnostic{"profile", "missing_policy_identity", "policy version and digest are required"}
	}
	if p.Raise != nil {
		if p.Raise.Value == "" || !valid(p.Raise.Value, []Assurance{AssuranceOrdinary, AssuranceElevated, AssuranceCritical}) {
			return Diagnostic{"assurance_raise.value", "unknown_value", "unknown assurance"}
		}
		if err := validateCitationShape(p.Raise.Citations, "assurance_raise.citations"); err != nil {
			return err
		}
	}
	return nil
}

// Origin is the closed execution-demand/v2 envelope for a plan/v2 assessment.
type Origin struct {
	Schema string
	Kind   OriginKind
	PlanV2 *PlanV2Origin
}

func (o Origin) validate() error {
	if o.Schema != OriginSchema {
		return Diagnostic{"schema", "unknown_schema", fmt.Sprintf("unsupported origin schema %q", o.Schema)}
	}
	switch o.Kind {
	case OriginPlanV2:
		if o.PlanV2 == nil {
			return Diagnostic{"kind", "inconsistent_union", "plan_v2 kind requires exactly the plan_v2 member"}
		}
		return o.PlanV2.validate()

	default:
		return Diagnostic{"kind", "unknown_kind", fmt.Sprintf("unknown origin kind %q", o.Kind)}
	}
}

// coreIdentity returns o's repository, frozen-plan, task, and source
// identity for Identify and Seal (capsule.go).
func (o Origin) coreIdentity() (RepositoryIdentity, PlanIdentity, TaskIdentity, SourceDigest) {
	switch o.Kind {
	case OriginPlanV2:
		p := o.PlanV2
		return p.Repository, p.Plan, p.Task, p.Source

	default:
		return "", PlanIdentity{}, "", ""
	}
}

// profileMinimumOf projects a full Profile down to its four minima.
func profileMinimumOf(p Profile) ProfileMinimum {
	return ProfileMinimum{Capability: p.Capability, Deliberation: p.Deliberation, Context: p.Context, Assurance: p.Assurance}
}

// validateProfileComplete requires all four minima to be present and known
// — unlike T1's validateMinimum (which permits a partial floor spec), a
// frozen final profile must fully resolve every axis (DINV-1).
func validateProfileComplete(m ProfileMinimum, field string) error {
	if m.Capability == "" || !valid(m.Capability, []Capability{CapabilityEconomy, CapabilityStandard, CapabilityStrong, CapabilityFrontier}) {
		return Diagnostic{field + ".capability", "unknown_value", "unknown or missing capability"}
	}
	if m.Deliberation == "" || !valid(m.Deliberation, []Deliberation{DeliberationLight, DeliberationStandard, DeliberationDeep, DeliberationMaximum}) {
		return Diagnostic{field + ".deliberation", "unknown_value", "unknown or missing deliberation"}
	}
	if m.Context == "" || !valid(m.Context, []Context{ContextStandard, ContextLarge, ContextExtended}) {
		return Diagnostic{field + ".context", "unknown_value", "unknown or missing context"}
	}
	if m.Assurance == "" || !valid(m.Assurance, []Assurance{AssuranceOrdinary, AssuranceElevated, AssuranceCritical}) {
		return Diagnostic{field + ".assurance", "unknown_value", "unknown or missing assurance"}
	}
	return nil
}

// validateCitationShape checks structural well-formedness (non-empty, no
// duplicates) independent of any EvidenceSet — an already-frozen origin
// carries no evidence resolver of its own.
func validateCitationShape(cs []Citation, field string) error {
	if len(cs) == 0 {
		return Diagnostic{field, "missing_citation", "at least one citation is required"}
	}
	seen := make(map[string]bool, len(cs))
	for _, c := range cs {
		if c.ID == "" {
			return Diagnostic{field, "invalid_citation", "citation id must not be empty"}
		}
		if seen[c.ID] {
			return Diagnostic{field, "duplicate_citation", fmt.Sprintf("duplicate citation %q", c.ID)}
		}
		seen[c.ID] = true
	}
	return nil
}
