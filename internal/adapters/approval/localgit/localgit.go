// Package localgit observes declared local review consistency for draft locks.
package localgit

import (
	"context"
	"fmt"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
	"github.com/agenticlab-ai/pyxis/internal/draft/approval"
)

type Reader struct{ Receipts codehost.ReceiptReader }

var _ approval.DeclaredPort = Reader{}

func (r Reader) DeclaredApproval(ctx context.Context, binding codehost.ReviewBinding) (approval.DeclaredApproval, error) {
	if err := ctx.Err(); err != nil {
		return approval.DeclaredApproval{}, err
	}
	if binding.Kind != "draft" || binding.Operation != codehost.DraftLock {
		return approval.DeclaredApproval{}, codehost.ErrReviewInvalid
	}
	if r.Receipts == nil {
		return approval.DeclaredApproval{}, fmt.Errorf("declared approval requires a configured local receipt reader")
	}
	record, err := r.Receipts.Verify(ctx, binding)
	if err != nil {
		return approval.DeclaredApproval{}, err
	}
	if !record.Trust.SharedAccount || record.Trust.ApproverAuthenticated || record.Trust.MainlineProtected || record.Trust.IndependentOriginIdentity {
		return approval.DeclaredApproval{}, fmt.Errorf("declared approval returned an unsupported trust claim")
	}
	if record.Receipt.Binding != binding {
		return approval.DeclaredApproval{}, codehost.ErrReviewInvalid
	}
	return approval.DeclaredApproval{Operator: record.Receipt.Reviewer, Head: record.Receipt.Binding.Head, CaptureLine: record.Receipt.CaptureLine, Record: record}, nil
}
