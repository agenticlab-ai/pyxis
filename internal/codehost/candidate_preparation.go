package codehost

import (
	"crypto/rand"
	"io"
	"strings"
)

const (
	CandidatePreparationSchema = "local-candidate-preparation/v1"
	FeatureFinalMerge          = "feature-final-merge"
)

// FinalCandidate carries the complete accepted identity before its
// generation-bound evidence exists. Fields match the consumer's candidate
// identity, without importing that consumer through the code-host port.
type FinalCandidate struct {
	FeatureID          string `json:"feature_id"`
	FeatureHead        string `json:"feature_head"`
	MainHead           string `json:"main_head"`
	Commit             string `json:"commit"`
	Tree               string `json:"tree"`
	MergePolicyVersion string `json:"merge_policy_version"`
	BuilderVersion     string `json:"builder_version"`
}

type CandidatePreparation struct {
	SchemaVersion          string         `json:"schema_version"`
	Operation              string         `json:"operation"`
	Nonce                  string         `json:"nonce"`
	Candidate              FinalCandidate `json:"candidate"`
	ScenarioSetFingerprint string         `json:"scenario_set_fingerprint"`
}

func NewCandidatePreparation(candidate FinalCandidate, scenarios string) (CandidatePreparation, error) {
	return newCandidatePreparation(candidate, scenarios, rand.Reader)
}

func newCandidatePreparation(candidate FinalCandidate, scenarios string, entropy io.Reader) (CandidatePreparation, error) {
	p := CandidatePreparation{CandidatePreparationSchema, FeatureFinalMerge, strings.Repeat("0", 32), candidate, scenarios}
	if err := p.Validate(); err != nil {
		return CandidatePreparation{}, err
	}
	nonce, err := newTaskPreparation(entropy)
	if err != nil {
		return CandidatePreparation{}, err
	}
	p.Nonce = nonce.Nonce
	return p, nil
}

func (p CandidatePreparation) Validate() error {
	c := p.Candidate
	if p.SchemaVersion != CandidatePreparationSchema || p.Operation != FeatureFinalMerge || !lowerHex(p.Nonce, 32) ||
		c.FeatureID == "" || c.MergePolicyVersion == "" || c.BuilderVersion == "" || p.ScenarioSetFingerprint == "" ||
		strings.ContainsAny(c.FeatureID+c.MergePolicyVersion+c.BuilderVersion+p.ScenarioSetFingerprint, "\r\n\x00") {
		return ErrReviewInvalid
	}
	for _, oid := range []string{c.FeatureHead, c.MainHead, c.Commit, c.Tree} {
		if len(oid) != len(c.FeatureHead) || (!lowerHex(oid, 40) && !lowerHex(oid, 64)) {
			return ErrReviewInvalid
		}
	}
	return nil
}
