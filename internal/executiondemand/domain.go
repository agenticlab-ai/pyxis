package executiondemand

import (
	"errors"
	"sort"
)

// DerivationPolicyDigest returns the canonical identity used by Evaluate.
// Policy collection order is not semantic, so reordered rules have one digest.
func DerivationPolicyDigest(policy DerivationPolicy) (string, error) {
	if err := policy.validate(); err != nil {
		return "", err
	}
	return policyDigest(policy), nil
}

// DerivableProfiles enumerates every distinct semantic minimum that policy can
// produce. The assessment domain is generated from the policy's live floor and
// promotion predicates plus one valid baseline value per axis: values absent
// from policy are behaviorally equivalent to that baseline. Assurance raises
// are crossed in because they are an input to the same derivation function.
func DerivableProfiles(policy DerivationPolicy) ([]ProfileMinimum, error) {
	if err := policy.validate(); err != nil {
		return nil, err
	}

	reasoning := map[Reasoning]struct{}{ReasoningRoutine: {}}
	coupling := map[Coupling]struct{}{CouplingContained: {}}
	novelty := map[Novelty]struct{}{NoveltyKnown: {}}
	verification := map[Verification]struct{}{VerificationLocal: {}}
	impact := map[ImpactScope]struct{}{ImpactContained: {}}
	architecture := map[ArchitecturalRole]struct{}{RoleFeatureLocal: {}}
	for _, floor := range policy.Floors {
		switch floor.Axis {
		case "reasoning":
			reasoning[Reasoning(floor.Value)] = struct{}{}
		case "coupling":
			coupling[Coupling(floor.Value)] = struct{}{}
		case "novelty":
			novelty[Novelty(floor.Value)] = struct{}{}
		case "verification":
			verification[Verification(floor.Value)] = struct{}{}
		case "impact_scope":
			impact[ImpactScope(floor.Value)] = struct{}{}
		case "architectural_role":
			architecture[ArchitecturalRole(floor.Value)] = struct{}{}
		}
	}
	for _, promotion := range policy.Promotions {
		if promotion.When.Coupling != "" {
			coupling[promotion.When.Coupling] = struct{}{}
		}
		if promotion.When.Novelty != "" {
			novelty[promotion.When.Novelty] = struct{}{}
		}
		if promotion.When.ImpactScope != "" {
			impact[promotion.When.ImpactScope] = struct{}{}
		}
		if promotion.When.ArchitecturalRole != "" {
			architecture[promotion.When.ArchitecturalRole] = struct{}{}
		}
	}

	profiles := map[ProfileMinimum]struct{}{}
	evidence := EvidenceSet{Resolved: map[string]struct{}{"domain": {}, "raise": {}}}
	for r := range reasoning {
		for c := range coupling {
			for n := range novelty {
				for v := range verification {
					for i := range impact {
						for a := range architecture {
							assessment := Assessment{Reasoning: r, Coupling: c, Novelty: n, Verification: v, ImpactScope: i, ArchitecturalRole: a, Citations: []Citation{{ID: "domain"}}}
							for _, raised := range []Assurance{"", AssuranceOrdinary, AssuranceElevated, AssuranceCritical} {
								raise := AssuranceRaise{Value: raised}
								if raised != "" {
									raise.Citations = []Citation{{ID: "raise"}}
								}
								profile, err := Evaluate(assessment, evidence, raise, policy)
								if err != nil {
									var diagnostic Diagnostic
									if errors.As(err, &diagnostic) && diagnostic.Code == "reduction" {
										continue
									}
									return nil, err
								}
								profiles[ProfileMinimum{Capability: profile.Capability, Deliberation: profile.Deliberation, Context: profile.Context, Assurance: profile.Assurance}] = struct{}{}
							}
						}
					}
				}
			}
		}
	}

	out := make([]ProfileMinimum, 0, len(profiles))
	for profile := range profiles {
		out = append(out, profile)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Capability != out[j].Capability {
			return out[i].Capability < out[j].Capability
		}
		if out[i].Deliberation != out[j].Deliberation {
			return out[i].Deliberation < out[j].Deliberation
		}
		if out[i].Context != out[j].Context {
			return out[i].Context < out[j].Context
		}
		return out[i].Assurance < out[j].Assurance
	})
	return out, nil
}
