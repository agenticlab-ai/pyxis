package reviewinvoke

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/agenticlab-ai/pyxis/internal/executor/report"
	"github.com/agenticlab-ai/pyxis/internal/executor/reviewobligation"
	"github.com/agenticlab-ai/pyxis/internal/executor/worker"
	"github.com/agenticlab-ai/pyxis/internal/implreview"
	"github.com/agenticlab-ai/pyxis/internal/implreview/persist"
	"github.com/agenticlab-ai/pyxis/internal/implreview/verdict"
)

// PairRepairer authenticates an already-durable EVT-13↔EVT-17 pair and its
// exact host artifact before appending the missing lifecycle terminal. It owns
// no runtime and therefore cannot launch or republish during crash recovery.
type PairRepairer struct {
	store       reviewobligation.Store
	persistence implreview.HostPersistenceAdapter
	verifier    ReviewTerminalVerifier
}

func NewPairRepairer(store reviewobligation.Store, persistence implreview.HostPersistenceAdapter, verifier ReviewTerminalVerifier) (*PairRepairer, error) {
	if store == nil || persistence == nil || verifier == nil {
		return nil, errors.New("reviewinvoke: pair repair requires store, persistence adapter, and terminal verifier")
	}
	return &PairRepairer{store: store, persistence: persistence, verifier: verifier}, nil
}

func (r *PairRepairer) Preflight(ctx context.Context, repository string) error {
	if err := r.store.Preflight(ctx); err != nil {
		return fmt.Errorf("reviewinvoke: pair repair obligation store: %w", err)
	}
	var err error
	if readOnly, ok := r.persistence.(interface {
		PreflightRead(context.Context, string) error
	}); ok {
		err = readOnly.PreflightRead(ctx, repository)
	} else {
		err = r.persistence.Preflight(ctx, repository)
	}
	if err != nil {
		return fmt.Errorf("reviewinvoke: pair repair persistence: %w", err)
	}
	return nil
}

func (r *PairRepairer) Repair(ctx context.Context, obligation reviewobligation.Obligation, pair reviewobligation.PublicationPair, driveRoundID string) (worker.ReviewResult, error) {
	result := worker.ReviewResult{DriveRoundID: driveRoundID, AttemptID: pair.AttemptID,
		ObligationID: pair.ObligationID, Harness: obligation.Harness,
		ProposalRef: pair.ProposalRef, Revision: pair.Revision, Phase: worker.ReviewPhaseAttribution,
		Status: report.ReviewStatusFailed, ObligationStatus: report.ReviewObligationPending,
		FailureClass: worker.ReviewFailureArtifactUntrusted}
	if pair.ObligationID != obligation.ID || pair.ProposalRef != obligation.ProposalRef || pair.Revision != obligation.Revision {
		return result, errors.New("reviewinvoke: publication pair does not match its protected obligation")
	}
	binding, err := r.verifier.VerifyAttemptProof(ctx, obligation, pair.AttemptID, pair.AttemptProofRef)
	if err != nil {
		return result, fmt.Errorf("reviewinvoke: reverify protected attempt proof: %w", err)
	}
	if binding.ReviewProcessID != pair.AttemptID || binding.ReviewObligationID != pair.ObligationID || binding.ProposalID != pair.ProposalRef || binding.ReviewedRevision != pair.Revision || binding.AttemptProofRef != pair.AttemptProofRef {
		return result, errors.New("reviewinvoke: recovered publication binding is not field-identical")
	}
	publication := implreview.PublicationTerminal{
		ReviewProcessID: pair.AttemptID, ReviewObligationID: pair.ObligationID,
		PublicationAttemptID: pair.PublicationAttemptID, ProposalID: pair.ProposalRef,
		ReviewedRevision: pair.Revision, Medium: pair.Medium, IdempotencyID: pair.IdempotencyID,
		Status: implreview.PublicationVerified, ArtifactRef: pair.ArtifactRef,
		NativeReceipt: pair.NativeReceipt, AttemptProofRef: pair.AttemptProofRef, EVT13Ref: pair.EVT13Ref,
	}
	host := worker.ReviewPublicationTerminal{ReviewAttemptID: pair.AttemptID,
		ReviewObligationID: pair.ObligationID, ProposalRef: pair.ProposalRef, Revision: pair.Revision,
		Status: report.ReviewStatusCompleted, ArtifactRef: pair.ArtifactRef,
		NativeReceipt: pair.NativeReceipt, AttemptProofRef: pair.AttemptProofRef}
	if err := r.verifier.VerifyPair(ctx, publication, host); err != nil {
		return result, fmt.Errorf("reviewinvoke: reverify reciprocal publication pair: %w", err)
	}
	evt17Ref, err := hostedTerminalRef(host)
	if err != nil || evt17Ref != pair.EVT17Ref {
		return result, errors.New("reviewinvoke: EVT-17 immutable evidence reference mismatch")
	}
	reviewReport, err := r.persistence.ExactRead(ctx, pair.NativeReceipt)
	if err != nil {
		result.FailureClass = worker.ReviewFailureInfrastructure
		return result, fmt.Errorf("reviewinvoke: exact-read recovered native receipt: %w", err)
	}
	revision, revisionOK := verdict.LatestReviewedRevision(reviewReport)
	body, renderErr := persist.ComposeArtifact(reviewReport)
	digest := sha256.Sum256([]byte(body))
	artifactRef := "sha256:" + hex.EncodeToString(digest[:])
	if !revisionOK || renderErr != nil || reviewReport.ProposalRef != pair.ProposalRef || reviewReport.ReviewID != pair.AttemptID || revision != pair.Revision || artifactRef != pair.ArtifactRef {
		return result, errors.New("reviewinvoke: recovered receipt does not contain the paired exact artifact")
	}
	if err := r.store.AppendTerminal(ctx, reviewobligation.Terminal{SchemaVersion: reviewobligation.TerminalSchema,
		ObligationID: pair.ObligationID, AttemptID: pair.AttemptID, Status: report.ReviewStatusCompleted,
		ArtifactRef: pair.ArtifactRef, NativeReceipt: pair.NativeReceipt,
		AttemptProofRef: pair.AttemptProofRef, EVT13Ref: pair.EVT13Ref, EVT17Ref: pair.EVT17Ref}); err != nil {
		result.FailureClass = worker.ReviewFailureInfrastructure
		return result, fmt.Errorf("reviewinvoke: append recovered lifecycle terminal: %w", err)
	}
	result.Phase, result.Status, result.ObligationStatus, result.FailureClass = worker.ReviewPhaseArtifact, report.ReviewStatusCompleted, report.ReviewObligationVerified, ""
	result.ArtifactRef, result.ArtifactProofRef, result.NativeReceipt = pair.ArtifactRef, pair.AttemptProofRef, pair.NativeReceipt
	result.EVT13Ref, result.EVT17Ref = pair.EVT13Ref, pair.EVT17Ref
	result.ImplReviewTerminal, result.Publication = &publication, &host
	return result, nil
}
