package executiondemand

import (
	"encoding/json"
	"slices"
	"testing"
)

func TestDerivableProfilesArePolicyDerivedAndOrderIndependent(t *testing.T) {
	baseline, err := DerivableProfiles(DefaultPolicy)
	if err != nil {
		t.Fatal(err)
	}
	reordered := DefaultPolicy
	reordered.Floors = slices.Clone(reordered.Floors)
	reordered.Promotions = slices.Clone(reordered.Promotions)
	slices.Reverse(reordered.Floors)
	slices.Reverse(reordered.Promotions)
	got, err := DerivableProfiles(reordered)
	if err != nil {
		t.Fatal(err)
	}
	wantJSON, err := json.Marshal(baseline)
	if err != nil {
		t.Fatal(err)
	}
	gotJSON, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if string(gotJSON) != string(wantJSON) || len(got) == 0 {
		t.Fatalf("reordered policy changed derivable profiles\nwant %s\n got %s", wantJSON, gotJSON)
	}
	wantDigest, _ := DerivationPolicyDigest(DefaultPolicy)
	gotDigest, _ := DerivationPolicyDigest(reordered)
	if gotDigest != wantDigest {
		t.Fatalf("reordered policy digest = %q, want %q", gotDigest, wantDigest)
	}
}

func TestDerivableProfilesIncludesAssuranceRaiseDomain(t *testing.T) {
	policy := DerivationPolicy{Schema: "execution-demand-policy/v1", Version: "empty-v1", Floors: []Floor{}, Promotions: []Promotion{}}
	profiles, err := DerivableProfiles(policy)
	if err != nil {
		t.Fatal(err)
	}
	want := []ProfileMinimum{
		{Capability: CapabilityEconomy, Deliberation: DeliberationLight, Context: ContextStandard, Assurance: AssuranceCritical},
		{Capability: CapabilityEconomy, Deliberation: DeliberationLight, Context: ContextStandard, Assurance: AssuranceElevated},
		{Capability: CapabilityEconomy, Deliberation: DeliberationLight, Context: ContextStandard, Assurance: AssuranceOrdinary},
	}
	if !slices.Equal(profiles, want) {
		t.Fatalf("profiles = %#v, want %#v", profiles, want)
	}
}

func TestDerivableProfilesRejectsInvalidLivePolicy(t *testing.T) {
	invalid := DefaultPolicy
	invalid.Floors = append(slices.Clone(invalid.Floors), invalid.Floors[0])
	if _, err := DerivableProfiles(invalid); err == nil {
		t.Fatal("duplicate live policy rule was accepted")
	}
}
