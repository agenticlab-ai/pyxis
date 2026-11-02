package routingevidence

import (
	"context"
	"errors"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

type ownershipStore struct {
	event   Event
	subject *SubjectView
	readErr error
}

func (*ownershipStore) Append(context.Context, AppendRequest) (EvidenceProof, error) {
	return EvidenceProof{}, errors.New("unexpected append")
}

func (s *ownershipStore) ReadProof(context.Context, EvidenceProof) (ExactRead, error) {
	if s.readErr != nil {
		return ExactRead{}, s.readErr
	}
	return ExactRead{Events: []Event{s.event}}, nil
}

func (s *ownershipStore) ReadSubject(context.Context, SubjectID) (ExactRead, error) {
	return ExactRead{Subject: s.subject}, nil
}

func TestVerifyOwnershipTransferBindsExactCanonicalAndRuntimeProof(t *testing.T) {
	fence := port.RuntimeFenceProof{AttemptID: "attempt-7", Harness: "codex", OwnerGeneration: 1,
		Observation: port.RuntimeAbsentProven, Nonce: "nonce-7", ProofReference: "runtime-proof-7", Signature: "signed"}
	event := Event{Kind: AttemptOwnerTransferred, EventID: "transfer-7", ContentDigest: "digest-7",
		Payload: Payload{AttemptOwnerTransferred: &AttemptOwnerTransferredPayload{AttemptID: fence.AttemptID,
			PriorOwnerGeneration: 1, NewOwnerGeneration: 2, Observation: string(fence.Observation),
			RuntimeFenceNonce: fence.Nonce, RuntimeFenceProofRef: fence.ProofReference}}}
	store := &ownershipStore{event: event, subject: &SubjectView{Attempt: &AttemptView{AttemptID: fence.AttemptID,
		OwnerGeneration: 2, LaunchEligible: true, TransferEventIDs: []EventID{event.EventID}}}}
	proof := EvidenceProof{Entries: []EvidenceEntry{{EventID: event.EventID, ContentDigest: event.ContentDigest}}}

	got, err := VerifyOwnershipTransfer(t.Context(), store, proof, fence)
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != port.OwnershipProofTransfer || got.Reference != string(event.EventID) || got.Digest != event.ContentDigest ||
		got.AttemptID != fence.AttemptID || got.Harness != fence.Harness || got.OwnerGeneration != 2 || got.Fence != fence {
		t.Fatalf("ownership proof = %+v", got)
	}

	altered := fence
	altered.Nonce = "other"
	if _, err := VerifyOwnershipTransfer(t.Context(), store, proof, altered); err == nil {
		t.Fatal("altered runtime nonce matched canonical transfer")
	}
	store.subject.Attempt.OwnerGeneration = 3
	if _, err := VerifyOwnershipTransfer(t.Context(), store, proof, fence); err == nil {
		t.Fatal("non-current exact-read transfer admitted generation two")
	}
}

func TestVerifyOwnershipTransferRejectsMissingAndUnreadableProof(t *testing.T) {
	if _, err := VerifyOwnershipTransfer(t.Context(), nil, EvidenceProof{}, port.RuntimeFenceProof{}); err == nil {
		t.Fatal("missing transfer proof was accepted")
	}
	store := &ownershipStore{readErr: errors.New("read failed")}
	proof := EvidenceProof{Entries: []EvidenceEntry{{EventID: "event", ContentDigest: "digest"}}}
	if _, err := VerifyOwnershipTransfer(t.Context(), store, proof, port.RuntimeFenceProof{}); err == nil {
		t.Fatal("proof read failure was ignored")
	}
}
