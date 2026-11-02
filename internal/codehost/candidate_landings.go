package codehost

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
)

const CandidateLandingSchema = "local-candidate-landing/v1"

// CandidateLanding retains delivered L and feature marker F' alongside the
// complete accepted proof and the exact operator request and declaration.
type CandidateLanding struct {
	SchemaVersion string         `json:"schema_version"`
	Candidate     FinalCandidate `json:"candidate"`
	MergeCommit   string         `json:"merge_commit"`
	FeatureMarker string         `json:"feature_marker"`
	Evidence      ReviewArtifact `json:"evidence"`
	Request       ReviewArtifact `json:"request"`
	Review        ReceiptRecord  `json:"review"`
}

func (l CandidateLanding) Validate() error {
	b := l.Review.Receipt.Binding
	if l.SchemaVersion != CandidateLandingSchema || b.Validate() != nil || b.Operation != FeatureFinalMerge || l.Candidate != b.Candidate ||
		!lowerHex(l.MergeCommit, len(b.Head)) || !lowerHex(l.FeatureMarker, len(b.Head)) || l.MergeCommit == l.FeatureMarker ||
		l.Review.Receipt.SchemaVersion != ReceiptSchema || !lowerHex(l.Review.OID, len(b.Head)) ||
		l.Evidence.Kind != CandidateEvidence || l.Evidence.Identity != b.Evidence || l.Request.Kind != CandidateRequest || l.Request.Identity != b.Request {
		return ErrReviewInvalid
	}
	for _, artifact := range []ReviewArtifact{l.Evidence, l.Request} {
		if len(artifact.Body) == 0 || len(artifact.Body) > MaxReviewArtifactBytes || artifact.Identity.Digest != fmt.Sprintf("sha256:%x", sha256.Sum256(artifact.Body)) {
			return ErrReviewInvalid
		}
	}
	request, err := ParseCandidateRequest(l.Request.Body)
	if err != nil {
		return err
	}
	request.Binding.Request = l.Request.Identity
	if request.Binding != b {
		return ErrReviewInvalid
	}
	body, err := json.Marshal(l.Review.Receipt)
	if err != nil {
		return err
	}
	if l.Review.Digest != fmt.Sprintf("sha256:%x", sha256.Sum256(append(body, '\n'))) {
		return ErrReviewInvalid
	}
	return nil
}

type CandidateLandingResult struct {
	Landing     CandidateLanding   `json:"landing"`
	Prepared    ProposalGeneration `json:"prepared"`
	MainHead    string             `json:"main_head"`
	FeatureHead string             `json:"feature_head"`
	Created     bool               `json:"created"`
}

// A drive receives only the reader. Publishing an approved final landing is
// an explicit operator action after current scenario and attribution checks.
type CandidateLandingReader interface {
	VerifyCandidateLanding(context.Context, string) (CandidateLandingResult, error)
}

type CandidateLandings interface {
	CandidateLandingReader
	LandCandidate(context.Context, ReviewBinding) (CandidateLandingResult, error)
}
