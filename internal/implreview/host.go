// Package implreview declares the trusted host boundary around an isolated
// impl-review process. Reviewer execution can return only an untrusted process
// handoff; the coordinator turns that into a canonical aggregate and the
// persistence adapter is the sole proposal-medium publisher.
package implreview

import (
	"context"

	implreport "github.com/agenticlab-ai/pyxis/internal/implreview/report"
)

// HandoffStatus is the closed result of the outer reviewer process handoff.
type HandoffStatus string

const (
	HandoffCompleted  HandoffStatus = "completed"
	HandoffMissing    HandoffStatus = "missing"
	HandoffSurplus    HandoffStatus = "surplus"
	HandoffMalformed  HandoffStatus = "malformed"
	HandoffMismatched HandoffStatus = "mismatched"
)

// PublicationStatus is the closed terminal status of one publication attempt.
type PublicationStatus string

const (
	PublicationVerified PublicationStatus = "verified"
	PublicationFailed   PublicationStatus = "failed"
)

// FailureClass is shared byte-for-byte by impl-review EVT-13 and the hosting
// executor's reciprocal publication terminal.
type FailureClass string

const (
	FailureArtifactUnverified FailureClass = "artifact_unverified"
	FailureArtifactUntrusted  FailureClass = "artifact_untrusted"
	FailureInfrastructure     FailureClass = "infrastructure"
)

// ReviewSubject is the exact host-owned binding admitted before reviewer
// launch. Process identity is deliberately absent: the ProcessCoordinator is
// its sole allocator.
type ReviewSubject struct {
	ReviewObligationID string `json:"review_obligation_id"`
	ProposalID         string `json:"proposal_id"`
	ReviewedRevision   string `json:"reviewed_revision"`
}

// AdmittedProcess is the process identity and exact subject returned by
// ProcessCoordinator.Prepare. A hosting executor must adopt ReviewProcessID as
// its review_attempt_id rather than minting a parallel identity.
type AdmittedProcess struct {
	ReviewProcessID string        `json:"review_process_id"`
	Subject         ReviewSubject `json:"subject"`
}

// ProcessHandoff is the only value allowed to leave isolated reviewer
// execution. ArtifactBody is untrusted terminal output and is interpreted only
// by ProcessCoordinator.Complete.
type ProcessHandoff struct {
	Status           HandoffStatus `json:"status"`
	ArtifactBody     string        `json:"artifact_body,omitempty"`
	DiagnosticDigest string        `json:"diagnostic_digest,omitempty"`
}

// CompletedAggregate is the canonical, validated process aggregate. The
// persistence adapter renders and publishes Report; callers do not parse it.
type CompletedAggregate struct {
	ProcessID        string             `json:"review_process_id"`
	ProposalID       string             `json:"proposal_id"`
	ReviewedRevision string             `json:"reviewed_revision"`
	Report           *implreport.Report `json:"report"`
}

// AggregateHandoff is the trusted coordinator's closed output.
type AggregateHandoff struct {
	Status           HandoffStatus       `json:"status"`
	Aggregate        *CompletedAggregate `json:"aggregate,omitempty"`
	DiagnosticDigest string              `json:"diagnostic_digest,omitempty"`
}

const NativeLensEvidenceSchema = "impl-review-native-lens-evidence/v1"

// NativeLensEvidence is minted by the host after checking the native children.
// It is not part of the isolated reviewer's handoff. Its aggregate digest binds
// recovery to the same completed report rather than a parent's new claim.
type NativeLensEvidence struct {
	SchemaVersion   string `json:"schema_version"`
	PackDigest      string `json:"pack_digest"`
	EvidenceDigest  string `json:"evidence_digest"`
	AggregateDigest string `json:"aggregate_digest"`
}

type NativeLensEvidenceCoordinator interface {
	RecordNativeLensEvidence(context.Context, AdmittedProcess, NativeLensEvidence) error
	VerifyNativeLensEvidence(context.Context, AdmittedProcess) error
}

// PublicationBinding carries the exact hosting identities into impl-review's
// persistence boundary. AttemptProofRef is a non-secret reference; its raw
// source remains outside every child process and workspace.
type PublicationBinding struct {
	ReviewObligationID string `json:"review_obligation_id"`
	ReviewProcessID    string `json:"review_process_id"`
	ProposalID         string `json:"proposal_id"`
	ReviewedRevision   string `json:"reviewed_revision"`
	AttemptProofRef    string `json:"attempt_proof_ref"`
}

// VerifiedPublicationBinding can be constructed only after the host verifies
// AttemptProofRef against protected obligation/attempt evidence.
type VerifiedPublicationBinding struct {
	PublicationBinding
	ProofEvidenceRef string `json:"proof_evidence_ref"`
}

// PublicationTerminal is the complete impl-review EVT-13 projection returned
// to a hosting executor. PublicationAttemptID is impl-review-owned and has no
// executor alias. EVT13Ref identifies the immutable canonical terminal.
type PublicationTerminal struct {
	ReviewProcessID      string            `json:"review_process_id"`
	ReviewObligationID   string            `json:"review_obligation_id"`
	PublicationAttemptID string            `json:"publication_attempt_id"`
	ProposalID           string            `json:"proposal_id"`
	ReviewedRevision     string            `json:"reviewed_revision"`
	Medium               string            `json:"medium"`
	IdempotencyID        string            `json:"idempotency_id"`
	Status               PublicationStatus `json:"status"`
	ArtifactRef          string            `json:"artifact_ref,omitempty"`
	NativeReceipt        string            `json:"native_receipt,omitempty"`
	AttemptProofRef      string            `json:"attempt_proof_ref,omitempty"`
	FailureClass         FailureClass      `json:"failure_class,omitempty"`
	Reason               string            `json:"reason,omitempty"`
	EVT13Ref             string            `json:"evt13_ref"`
}

// ProcessCoordinator owns review-process identity and converts one bounded,
// untrusted outer-process handoff into canonical aggregate evidence.
type ProcessCoordinator interface {
	Prepare(ctx context.Context, subject ReviewSubject) (AdmittedProcess, error)
	Complete(ctx context.Context, process AdmittedProcess, handoff ProcessHandoff) (AggregateHandoff, error)
}

// HostPersistenceAdapter is impl-review's sole proposal publisher. Preflight is
// side-effect-free; Complete durably fences one publication, returns its exact
// native receipt, exact-reads that receipt, and records EVT-13.
type HostPersistenceAdapter interface {
	Preflight(ctx context.Context, repository string) error
	Complete(ctx context.Context, binding VerifiedPublicationBinding, handoff AggregateHandoff) (PublicationTerminal, error)
	ExactRead(ctx context.Context, nativeReceipt string) (*implreport.Report, error)
}

// HostPublicationRecovery is the optional durable repair side implemented by
// production host adapters. It adopts an already-armed publication using its
// original process identity, exact-reads any receipt, and never republishes.
// The executor calls it before allocating a replacement review process.
type HostPublicationRecovery interface {
	InspectPublication(ctx context.Context, subject ReviewSubject) (PublicationRecovery, bool, error)
	RecoverPublication(ctx context.Context, subject ReviewSubject) (RecoveredPublication, bool, error)
}

type PublicationRecoveryState string

const (
	PublicationRecoveryArmed           PublicationRecoveryState = "armed"
	PublicationRecoveryReceiptRecorded PublicationRecoveryState = "receipt_recorded"
	PublicationRecoveryTerminal        PublicationRecoveryState = "terminal"
)

// PublicationRecovery is the read-only recovery candidate inspected before
// the executor acquires the original attempt lease. The adapter may mutate its
// durable publication state only through RecoverPublication after that lease.
type PublicationRecovery struct {
	Binding VerifiedPublicationBinding
	State   PublicationRecoveryState
}

// RecoveredPublication keeps the protected publication binding beside its
// terminal. Failed EVT-13 rows may legitimately omit attempt_proof_ref, so the
// executor authenticates the original attempt from Binding rather than
// inventing a proof from the terminal's optional field.
type RecoveredPublication struct {
	Binding  VerifiedPublicationBinding
	Terminal PublicationTerminal
}
