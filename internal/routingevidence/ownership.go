package routingevidence

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

// VerifyOwnershipTransfer exact-reads both the transfer event and its current
// attempt projection before producing a runtime activation proof. The runtime
// independently verifies the signed nonce-bound fence proof during Activate;
// this half proves the canonical event adopted those exact fields.
func VerifyOwnershipTransfer(ctx context.Context, store Store, proof EvidenceProof, fence port.RuntimeFenceProof) (port.OwnershipProof, error) {
	if store == nil || len(proof.Entries) != 1 {
		return port.OwnershipProof{}, errors.New("routing ownership: one exact transfer proof is required")
	}
	exact, err := store.ReadProof(ctx, proof)
	if err != nil {
		return port.OwnershipProof{}, fmt.Errorf("routing ownership: exact-read transfer: %w", err)
	}
	if len(exact.Events) != 1 || exact.Events[0].Kind != AttemptOwnerTransferred || exact.Events[0].Payload.AttemptOwnerTransferred == nil {
		return port.OwnershipProof{}, errors.New("routing ownership: proof is not one attempt_owner_transferred event")
	}
	event, entry := exact.Events[0], proof.Entries[0]
	payload := event.Payload.AttemptOwnerTransferred
	if entry.EventID != event.EventID || entry.ContentDigest != event.ContentDigest || payload.AttemptID != fence.AttemptID ||
		payload.PriorOwnerGeneration != int(fence.OwnerGeneration) || payload.NewOwnerGeneration != int(fence.OwnerGeneration)+1 ||
		payload.Observation != string(fence.Observation) || payload.RuntimeFenceNonce != fence.Nonce ||
		payload.RuntimeFenceProofRef != fence.ProofReference {
		return port.OwnershipProof{}, errors.New("routing ownership: canonical transfer differs from runtime fence proof")
	}
	current, err := store.ReadSubject(ctx, SubjectID{Kind: SubjectAttempt, ID: payload.AttemptID})
	if err != nil || current.Subject == nil || current.Subject.Attempt == nil {
		return port.OwnershipProof{}, errors.New("routing ownership: exact current attempt is unavailable")
	}
	attempt := current.Subject.Attempt
	if attempt.AttemptID != payload.AttemptID || attempt.OwnerGeneration != payload.NewOwnerGeneration ||
		!attempt.LaunchEligible || !slices.Contains(attempt.TransferEventIDs, event.EventID) {
		return port.OwnershipProof{}, errors.New("routing ownership: transfer is not the current launch-eligible generation")
	}
	return port.OwnershipProof{Kind: port.OwnershipProofTransfer, Reference: string(event.EventID), Digest: event.ContentDigest,
		AttemptID: payload.AttemptID, Harness: fence.Harness, OwnerGeneration: port.OwnerGeneration(payload.NewOwnerGeneration), Fence: fence}, nil
}
