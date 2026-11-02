package localgit

import (
	"context"
	"errors"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
)

type receiptReader struct {
	record codehost.ReceiptRecord
	err    error
}

func (r receiptReader) Binding(context.Context, string) (codehost.ReviewBinding, error) {
	return r.record.Receipt.Binding, r.err
}

func (r receiptReader) Verify(context.Context, codehost.ReviewBinding) (codehost.ReceiptRecord, error) {
	return r.record, r.err
}

func TestDeclaredApprovalPreservesLocalConsistencyBoundary(t *testing.T) {
	binding := codehost.ReviewBinding{Kind: "draft", Operation: codehost.DraftLock, Head: "exact head", ProposalOID: "exact generation"}
	record := codehost.ReceiptRecord{Receipt: codehost.ReviewReceipt{Binding: binding, Reviewer: "declared name", CaptureLine: "capture"}, Trust: codehost.Trust{SharedAccount: true}}
	got, err := (Reader{Receipts: receiptReader{record: record}}).DeclaredApproval(t.Context(), binding)
	if err != nil || got.Head != binding.Head || got.Operator != "declared name" || got.CaptureLine != "capture" || got.Record != record {
		t.Fatalf("declared review result: %+v, %v", got, err)
	}
	if _, err := (Reader{}).DeclaredApproval(t.Context(), binding); err == nil {
		t.Fatal("missing local reader accepted")
	}
	if _, err := (Reader{Receipts: receiptReader{err: codehost.ErrReceiptNotFound}}).DeclaredApproval(t.Context(), binding); !errors.Is(err, codehost.ErrReceiptNotFound) {
		t.Fatalf("missing receipt became approval: %v", err)
	}
	for _, trust := range []codehost.Trust{
		{},
		{SharedAccount: true, ApproverAuthenticated: true},
		{SharedAccount: true, MainlineProtected: true},
		{SharedAccount: true, IndependentOriginIdentity: true},
	} {
		wrong := record
		wrong.Trust = trust
		if _, err := (Reader{Receipts: receiptReader{record: wrong}}).DeclaredApproval(t.Context(), binding); err == nil {
			t.Fatalf("unsupported local trust claim accepted: %+v", trust)
		}
	}
	wrong := record
	wrong.Receipt.Binding.Head = "another head"
	if _, err := (Reader{Receipts: receiptReader{record: wrong}}).DeclaredApproval(t.Context(), binding); !errors.Is(err, codehost.ErrReviewInvalid) {
		t.Fatalf("mismatched verified record accepted: %v", err)
	}
	for _, wrongBinding := range []codehost.ReviewBinding{{Kind: "implementation", Operation: codehost.DraftLock}, {Kind: "draft", Operation: "feature-final-merge"}} {
		if _, err := (Reader{Receipts: receiptReader{record: record}}).DeclaredApproval(t.Context(), wrongBinding); !errors.Is(err, codehost.ErrReviewInvalid) {
			t.Fatalf("other operation crossed draft approval port: %v", err)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := (Reader{Receipts: receiptReader{record: record}}).DeclaredApproval(ctx, binding); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost: %v", err)
	}
}
