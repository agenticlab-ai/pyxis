package routingevidence

import (
	"context"
	"errors"
	"fmt"
)

// Store is the consumer-owned canonical evidence boundary. Implementations
// must fetch one configured ref, validate its complete state, and make an
// event or declared composite reachable through one compare-and-swap.
type Store interface {
	Append(context.Context, AppendRequest) (EvidenceProof, error)
	ReadSubject(context.Context, SubjectID) (ExactRead, error)
	ReadProof(context.Context, EvidenceProof) (ExactRead, error)
}

// ObservedHead is a lease on one exact fetched evidence-ref generation. A
// stale lease may be replayed only when it remains an ancestor of the current
// authoritative head.
type ObservedHead struct {
	EvidenceRef string `json:"evidence_ref"`
	Commit      string `json:"commit"`
}

// AtomicComposite is a transaction, not an optional batching hint. Every
// event must become reachable in the same winning commit.
type AtomicComposite struct {
	Events []Event
}

// AppendRequest admits exactly one event or exactly one atomic composite.
type AppendRequest struct {
	ObservedHead ObservedHead
	Event        *Event
	Composite    *AtomicComposite
}

// EvidenceEntry exact-binds one immutable identity path to its event and
// content digests.
type EvidenceEntry struct {
	Path          string  `json:"path"`
	EventID       EventID `json:"event_id"`
	ContentDigest string  `json:"content_digest"`
}

// EvidenceProof is the durable exact-read receipt for one event set. Commit
// is the head at which the complete set was observed reachable.
type EvidenceProof struct {
	EvidenceRef string          `json:"evidence_ref"`
	Commit      string          `json:"commit"`
	Entries     []EvidenceEntry `json:"entries"`
}

// ExactRead returns bytes and decoded events from exactly Proof.Commit. A
// subject read additionally carries the reducer's exact subject view.
type ExactRead struct {
	ObservedHead ObservedHead      `json:"observed_head"`
	Proof        EvidenceProof     `json:"proof"`
	Events       []Event           `json:"-"`
	Bytes        map[string][]byte `json:"-"`
	Subject      *SubjectView      `json:"subject,omitempty"`
}

var ErrStoreIntegrity = errors.New("routing evidence store integrity failure")

type StoreErrorCode string

const (
	StoreIdentityConflict StoreErrorCode = "identity_conflict"
	StorePartialExisting  StoreErrorCode = "partial_existing_composite"
	StoreLeaseInvalid     StoreErrorCode = "observed_head_invalid"
	StoreProofInvalid     StoreErrorCode = "exact_read_proof_invalid"
	StoreRefUnavailable   StoreErrorCode = "evidence_ref_unavailable"
)

// StoreError is the stable append/read decision surface for defects owned by
// persistence rather than the producer or structural reducer.
type StoreError struct {
	Code   StoreErrorCode
	Detail string
}

func (e *StoreError) Error() string { return fmt.Sprintf("%s: %s", e.Code, e.Detail) }
func (e *StoreError) Unwrap() error { return ErrStoreIntegrity }

func NewStoreError(code StoreErrorCode, format string, args ...any) error {
	return &StoreError{Code: code, Detail: fmt.Sprintf(format, args...)}
}
