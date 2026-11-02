package codehost

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"strings"
)

const (
	ReviewRequestSchema = "local-review-request/v1"
	ReviewBindingSchema = "local-review-binding/v1"
	ReceiptSchema       = "local-review-receipt/v1"
	DraftLock           = "draft-lock"
)

var (
	ErrReviewInvalid   = errors.New("invalid local review binding")
	ErrReceiptNotFound = errors.New("local review receipt is absent")
	ErrReceiptConflict = errors.New("local review receipt already records another declaration")
)

// DraftReviewRequest is embedded in a prepared proposal before review. It
// does not depend on that proposal's OID or on a later receipt.
type DraftReviewRequest struct {
	SchemaVersion string `json:"schema_version"`
	Operation     string `json:"operation"`
	Nonce         string `json:"nonce"`
	Digest        string `json:"digest"`
}

func NewDraftReviewRequest(digest string) (DraftReviewRequest, error) {
	return newDraftReviewRequest(digest, rand.Reader)
}

func newDraftReviewRequest(digest string, entropy io.Reader) (DraftReviewRequest, error) {
	r := DraftReviewRequest{SchemaVersion: ReviewRequestSchema, Operation: DraftLock, Digest: digest, Nonce: strings.Repeat("0", 32)}
	if err := r.Validate(); err != nil {
		return DraftReviewRequest{}, err
	}
	nonce := make([]byte, 16)
	if _, err := io.ReadFull(entropy, nonce); err != nil {
		return DraftReviewRequest{}, err
	}
	r.Nonce = hex.EncodeToString(nonce)
	return r, nil
}

func (r DraftReviewRequest) Validate() error {
	if r.SchemaVersion != ReviewRequestSchema || r.Operation != DraftLock || !lowerHex(r.Nonce, 32) || !strings.HasPrefix(r.Digest, "sha256:") || !lowerHex(strings.TrimPrefix(r.Digest, "sha256:"), 64) {
		return ErrReviewInvalid
	}
	return nil
}

func lowerHex(value string, size int) bool {
	if len(value) != size {
		return false
	}
	for _, c := range value {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// ReviewBinding is the exact request rendered from the current prepared
// generation. The operator must submit this binding, not just a head SHA.
type ReviewBinding struct {
	SchemaVersion          string         `json:"schema_version"`
	ProposalID             string         `json:"proposal_id"`
	Kind                   string         `json:"kind"`
	ProposalOID            string         `json:"proposal_oid"`
	Sequence               uint64         `json:"sequence"`
	Operation              string         `json:"operation"`
	Branch                 string         `json:"branch"`
	Head                   string         `json:"head"`
	TargetRef              string         `json:"target_ref"`
	TargetOID              string         `json:"target_oid"`
	Digest                 string         `json:"digest"`
	Nonce                  string         `json:"nonce"`
	Evidence               ArtifactRef    `json:"evidence,omitzero"`
	Request                ArtifactRef    `json:"request,omitzero"`
	Candidate              FinalCandidate `json:"candidate,omitzero"`
	ScenarioSetFingerprint string         `json:"scenario_set_fingerprint,omitempty"`
}

// Validate rejects an absent or unsupported saved-request discriminator
// before any origin operation. The adapter also compares every exact binding.
func (b ReviewBinding) Validate() error {
	if b.Kind == "implementation" {
		var err error
		if b.Operation == FeatureFinalMerge {
			err = b.validateCandidateIntent()
		} else {
			err = b.validateTaskIntent()
		}
		if err != nil {
			return err
		}
		if b.Request.Validate() != nil || b.Request.Ref != "refs/pyxis/review-requests/"+b.ProposalID+"/"+b.ProposalOID || len(b.Request.OID) != len(b.Head) {
			return ErrReviewInvalid
		}
		return nil
	}
	oid := func(s string) bool { return lowerHex(s, 40) || lowerHex(s, 64) }
	request := DraftReviewRequest{SchemaVersion: ReviewRequestSchema, Operation: b.Operation, Nonce: b.Nonce, Digest: b.Digest}
	if b.SchemaVersion != ReviewBindingSchema || b.ProposalID == "" || b.Kind != "draft" || b.Sequence == 0 ||
		!oid(b.ProposalOID) || !oid(b.Head) || !oid(b.TargetOID) || request.Validate() != nil ||
		!strings.HasPrefix(b.Branch, "refs/heads/") || !strings.HasPrefix(b.TargetRef, "refs/heads/") || b.Branch == b.TargetRef || b.Evidence != (ArtifactRef{}) || b.Request != (ArtifactRef{}) || b.Candidate != (FinalCandidate{}) || b.ScenarioSetFingerprint != "" {
		return ErrReviewInvalid
	}
	return nil
}

func (b ReviewBinding) validateTaskIntent() error {
	if b.SchemaVersion != ReviewBindingSchema || b.Kind != "implementation" || b.Operation != TaskMerge || b.ProposalID == "" || b.Sequence == 0 || !lowerHex(b.Nonce, 32) ||
		strings.ContainsAny(b.ProposalID+b.Branch+b.TargetRef, "\r\n\x00") || !strings.HasPrefix(b.Branch, "refs/heads/") || !strings.HasPrefix(b.TargetRef, "refs/heads/") || b.Branch == b.TargetRef ||
		b.Evidence.Validate() != nil || b.Digest != b.Evidence.Digest || b.Evidence.Ref != "refs/pyxis/review-aggregates/"+b.ProposalID+"/"+strings.TrimPrefix(b.Digest, "sha256:") || b.Candidate != (FinalCandidate{}) || b.ScenarioSetFingerprint != "" {
		return ErrReviewInvalid
	}
	for _, oid := range []string{b.ProposalOID, b.Head, b.TargetOID, b.Evidence.OID} {
		if len(oid) != len(b.Head) || (!lowerHex(oid, 40) && !lowerHex(oid, 64)) {
			return ErrReviewInvalid
		}
	}
	return nil
}

type ReviewReceipt struct {
	SchemaVersion string        `json:"schema_version"`
	Binding       ReviewBinding `json:"binding"`
	Reviewer      string        `json:"reviewer"`
	CaptureLine   string        `json:"capture_line,omitempty"`
}

type ReceiptRecord struct {
	OID     string        `json:"oid"`
	Digest  string        `json:"digest"`
	Receipt ReviewReceipt `json:"receipt"`
	Trust   Trust         `json:"trust"`
}

// ReceiptReader conveys consistency only. A drive can observe it without
// being given the operator's receipt-publication method.
type ReceiptReader interface {
	Binding(context.Context, string) (ReviewBinding, error)
	Verify(context.Context, ReviewBinding) (ReceiptRecord, error)
}

// ReviewValidator refreshes the complete current review policy and retained
// proof. Receipt consistency alone cannot satisfy review eligibility.
type ReviewValidator interface {
	ValidateReview(context.Context, ReviewBinding) error
}

// ReviewLease keeps the policy underlying a verified review unchanged through
// an operator mutation. Observation alone does not provide this guarantee.
type ReviewLease interface {
	WithReview(context.Context, ReviewBinding, func() error) error
}

type Receipts interface {
	ReceiptReader
	Record(context.Context, ReviewBinding, string, string) (ReceiptRecord, error)
}
