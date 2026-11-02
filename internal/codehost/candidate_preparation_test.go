package codehost

import (
	"bytes"
	"strings"
	"testing"
)

func TestCandidatePreparationBindsFullIdentityAndFreshNonce(t *testing.T) {
	for _, size := range []int{40, 64} {
		candidate := FinalCandidate{FeatureID: "widget", FeatureHead: strings.Repeat("a", size), MainHead: strings.Repeat("b", size), Commit: strings.Repeat("c", size), Tree: strings.Repeat("d", size), BuilderVersion: "builder-v1", MergePolicyVersion: "merge-v1"}
		p, err := newCandidatePreparation(candidate, "required-v1", bytes.NewReader(bytes.Repeat([]byte{3}, 16)))
		if err != nil || p.Validate() != nil || p.Candidate != candidate || p.Nonce != strings.Repeat("03", 16) {
			t.Fatalf("candidate identity or nonce changed: %+v %v", p, err)
		}
		for _, mutate := range []func(*CandidatePreparation){
			func(p *CandidatePreparation) { p.SchemaVersion = "unknown" },
			func(p *CandidatePreparation) { p.Operation = TaskMerge },
			func(p *CandidatePreparation) { p.Nonce = "short" },
			func(p *CandidatePreparation) { p.Candidate.FeatureID = "" },
			func(p *CandidatePreparation) { p.Candidate.BuilderVersion = "" },
			func(p *CandidatePreparation) { p.ScenarioSetFingerprint = "" },
			func(p *CandidatePreparation) { p.Candidate.MergePolicyVersion += "\n" },
			func(p *CandidatePreparation) { p.Candidate.Commit = "short" },
			func(p *CandidatePreparation) { p.Candidate.Tree = strings.Repeat("A", size) },
		} {
			changed := p
			mutate(&changed)
			if changed.Validate() == nil {
				t.Fatal("invalid candidate preparation accepted")
			}
		}
		if _, err := newCandidatePreparation(candidate, "required-v1", bytes.NewReader(nil)); err == nil {
			t.Fatal("unavailable nonce entropy accepted")
		}
	}
	if _, err := NewCandidatePreparation(FinalCandidate{}, "required-v1"); err == nil {
		t.Fatal("invalid candidate consumed an approval preparation")
	}
}
