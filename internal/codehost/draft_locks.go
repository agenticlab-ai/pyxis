package codehost

import (
	"context"
	"strings"
)

const DraftLockSchema = "local-draft-lock/v1"

// LocalDraftLock is a declared lock, with no hosted PR or authenticated review
// identity. Its retained receipt names the exact prepared generation and base.
type LocalDraftLock struct {
	SchemaVersion        string        `json:"schema_version"`
	Feature              string        `json:"feature"`
	Version              int           `json:"version"`
	MergeCommit          string        `json:"merge_commit"`
	Review               ReceiptRecord `json:"review"`
	ProductApprover      string        `json:"product_approver"`
	EngineeringApprover  string        `json:"engineering_approver"`
	Lineless             bool          `json:"lineless"`
	ProposalsSinceOpened int           `json:"proposals_since_opened"`
}

func (r LocalDraftLock) Validate() error {
	b, receipt := r.Review.Receipt.Binding, r.Review.Receipt
	request := DraftReviewRequest{SchemaVersion: ReviewRequestSchema, Operation: b.Operation, Nonce: b.Nonce, Digest: b.Digest}
	oid := func(s string) bool { return lowerHex(s, 40) || lowerHex(s, 64) }
	if r.SchemaVersion != DraftLockSchema || r.Feature == "" || r.Version < 1 || !oid(r.MergeCommit) ||
		r.ProposalsSinceOpened < 1 || b.Validate() != nil || request.Validate() != nil || receipt.SchemaVersion != ReceiptSchema ||
		b.Kind != "draft" || b.ProposalID == "" || b.Sequence == 0 || !oid(b.ProposalOID) || !oid(b.Head) ||
		!oid(b.TargetOID) || !strings.HasPrefix(b.Branch, "refs/heads/") || !strings.HasPrefix(b.TargetRef, "refs/heads/") || b.Branch == b.TargetRef ||
		!oid(r.Review.OID) || !strings.HasPrefix(r.Review.Digest, "sha256:") || !lowerHex(strings.TrimPrefix(r.Review.Digest, "sha256:"), 64) ||
		strings.TrimSpace(receipt.Reviewer) == "" || len(receipt.Reviewer) > 128 || strings.ContainsAny(receipt.Reviewer, "\r\n\x00") ||
		len(receipt.CaptureLine) > 8192 || strings.ContainsRune(receipt.CaptureLine, '\x00') ||
		r.ProductApprover != receipt.Reviewer || r.EngineeringApprover != receipt.Reviewer || r.Lineless != (receipt.CaptureLine == "") ||
		r.Review.Trust != (Trust{SharedAccount: true}) {
		return ErrReviewInvalid
	}
	return nil
}

type DraftLandingRequest struct {
	Binding              ReviewBinding
	Version              int
	ProposalsSinceOpened int
}

type DraftLandingResult struct {
	Lock    LocalDraftLock `json:"lock"`
	Created bool           `json:"created"`
}

// DraftLockReader exposes observation without the operator's landing method.
type DraftLockReader interface {
	VerifyDraftLock(context.Context, string, int) (LocalDraftLock, error)
}

// DraftLandings is composed only for the operator command after the workflow
// has rechecked readiness and memory. A receipt alone never proves readiness.
type DraftLandings interface {
	DraftLockReader
	LandDraft(context.Context, DraftLandingRequest) (DraftLandingResult, error)
}
