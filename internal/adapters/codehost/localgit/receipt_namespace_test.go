package localgit

import (
	"errors"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
)

func TestReceiptRefsSeparateProposalsUsingTheSameNonce(t *testing.T) {
	t.Parallel()
	o, proposals, receipts, first := preparedReceiptFixture(t)
	open, err := proposals.Publish(t.Context(), codehost.ProposalChange{ID: "draft-gadget", Feature: "gadget", Branch: "refs/heads/draft-gadget", Head: first.Head, State: "open", ExpectedTargetOID: first.TargetOID})
	if err != nil {
		t.Fatal(err)
	}
	request := codehost.DraftReviewRequest{SchemaVersion: codehost.ReviewRequestSchema, Operation: first.Operation, Nonce: first.Nonce, Digest: first.Digest}
	prepared := revision(open, first.Head)
	prepared.State, prepared.ReviewRequest = "prepared", &request
	if _, err := proposals.Publish(t.Context(), prepared); err != nil {
		t.Fatal(err)
	}
	second, err := receipts.Binding(t.Context(), open.Proposal.ID)
	if err != nil || second.Nonce != first.Nonce {
		t.Fatalf("second prepared binding: %+v %v", second, err)
	}
	clone := NewReceipts(proposalClone(t, o))
	for _, binding := range []codehost.ReviewBinding{first, second} {
		record, err := receipts.Record(t.Context(), binding, "operator", binding.ProposalID)
		if err != nil {
			t.Fatal(err)
		}
		ref := "refs/pyxis/receipts/" + binding.ProposalID + "/" + binding.Nonce
		if fixtureGit(t, o.Origin, "rev-parse", ref) != record.OID {
			t.Fatal("receipt is not stored in the contract's proposal namespace")
		}
		if verified, err := clone.Verify(t.Context(), binding); err != nil || verified != record {
			t.Fatalf("clone verification: %+v %v", verified, err)
		}
		if _, err := receipts.Record(t.Context(), binding, "another operator", ""); !errors.Is(err, codehost.ErrReceiptConflict) {
			t.Fatalf("once-only receipt was replaced: %v", err)
		}
	}
	if got := fixtureGit(t, o.Origin, "for-each-ref", "--format=%(refname)", "refs/pyxis/receipts/"+first.Nonce); got != "" {
		t.Fatalf("receipt used the global nonce namespace: %s", got)
	}
}
