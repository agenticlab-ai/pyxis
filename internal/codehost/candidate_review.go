package codehost

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

const (
	CandidateRequestSchema = "local-candidate-review-request/v1"
	CandidateEvidence      = "candidate-evidence"
	CandidateRequest       = "candidate-review-request"
)

func (b ReviewBinding) validateCandidateIntent() error {
	preparation := CandidatePreparation{CandidatePreparationSchema, b.Operation, b.Nonce, b.Candidate, b.ScenarioSetFingerprint}
	if b.SchemaVersion != ReviewBindingSchema || b.Kind != "implementation" || b.ProposalID == "" || b.Sequence == 0 || preparation.Validate() != nil ||
		strings.ContainsAny(b.ProposalID+b.Branch+b.TargetRef, "\r\n\x00") || !strings.HasPrefix(b.Branch, "refs/heads/") || !strings.HasPrefix(b.TargetRef, "refs/heads/") || b.Branch == b.TargetRef ||
		b.Head != b.Candidate.FeatureHead || b.TargetOID != b.Candidate.MainHead || b.Evidence.Validate() != nil || b.Digest != b.Evidence.Digest ||
		b.Evidence.Ref != "refs/pyxis/candidate-evidence/"+b.ProposalID+"/"+strings.TrimPrefix(b.Digest, "sha256:") {
		return ErrReviewInvalid
	}
	for _, oid := range []string{b.ProposalOID, b.Evidence.OID} {
		if len(oid) != len(b.Head) || (!lowerHex(oid, 40) && !lowerHex(oid, 64)) {
			return ErrReviewInvalid
		}
	}
	return nil
}

// CandidateReviewRequest precedes approval and pins the whole retained
// candidate proof. Its own identity is added only to the presented binding.
type CandidateReviewRequest struct {
	SchemaVersion string        `json:"schema_version"`
	Binding       ReviewBinding `json:"binding"`
}

func (r CandidateReviewRequest) Validate() error {
	if r.SchemaVersion != CandidateRequestSchema || r.Binding.Request != (ArtifactRef{}) {
		return ErrReviewInvalid
	}
	return r.Binding.validateCandidateIntent()
}

func (r CandidateReviewRequest) Matches(g ProposalGeneration) error {
	if err := r.Validate(); err != nil {
		return err
	}
	p, b := g.Proposal, r.Binding
	if p.SchemaVersion != ProposalSchema || p.Kind != "implementation" || p.State != "prepared" || p.Task != "" || p.SourceRef == "" || p.ReviewRequest != nil || p.TaskPreparation != nil || p.CandidatePreparation == nil || p.CandidatePreparation.Validate() != nil ||
		b.ProposalID != p.ID || b.ProposalOID != g.OID || b.Sequence != p.Sequence || b.Kind != p.Kind || b.Branch != p.Branch || b.Head != p.Head || b.TargetRef != p.TargetRef || b.TargetOID != p.TargetOID ||
		b.Nonce != p.CandidatePreparation.Nonce || b.Candidate != p.CandidatePreparation.Candidate || b.Candidate.FeatureID != p.Feature || b.ScenarioSetFingerprint != p.CandidatePreparation.ScenarioSetFingerprint {
		return ErrReviewInvalid
	}
	return nil
}

func (r CandidateReviewRequest) Canonical() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, err
	}
	body, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}
	return append(body, '\n'), nil
}

func ParseCandidateRequest(body []byte) (CandidateReviewRequest, error) {
	var r CandidateReviewRequest
	if len(body) > MaxReviewArtifactBytes {
		return r, ErrReviewInvalid
	}
	d := json.NewDecoder(bytes.NewReader(body))
	d.DisallowUnknownFields()
	if err := d.Decode(&r); err != nil {
		return CandidateReviewRequest{}, fmt.Errorf("%w: candidate request: %w", ErrReviewInvalid, err)
	}
	if err := d.Decode(new(any)); !errors.Is(err, io.EOF) {
		return CandidateReviewRequest{}, ErrReviewInvalid
	}
	canonical, err := r.Canonical()
	if err != nil || !bytes.Equal(body, canonical) {
		return CandidateReviewRequest{}, ErrReviewInvalid
	}
	return r, nil
}
