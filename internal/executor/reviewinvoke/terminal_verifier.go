package reviewinvoke

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/agenticlab-ai/pyxis/internal/executor/reviewobligation"
	"github.com/agenticlab-ai/pyxis/internal/executor/worker"
	"github.com/agenticlab-ai/pyxis/internal/implreview"
	publicationpkg "github.com/agenticlab-ai/pyxis/internal/implreview/publication"
)

type ReviewTerminalVerifier interface {
	VerifyAttemptProof(context.Context, reviewobligation.Obligation, string, string) (implreview.VerifiedPublicationBinding, error)
	VerifyPair(context.Context, implreview.PublicationTerminal, worker.ReviewPublicationTerminal) error
}

type StoreTerminalVerifier struct {
	store        reviewobligation.Store
	publications publicationpkg.Reader
}

func NewStoreTerminalVerifier(store reviewobligation.Store, publications publicationpkg.Reader) (*StoreTerminalVerifier, error) {
	if store == nil {
		return nil, errors.New("reviewinvoke: nil protected review state")
	}
	if publications == nil {
		return nil, errors.New("reviewinvoke: nil protected publication state")
	}
	return &StoreTerminalVerifier{store: store, publications: publications}, nil
}

func (v *StoreTerminalVerifier) VerifyAttemptProof(ctx context.Context, obligation reviewobligation.Obligation, processID, proofRef string) (implreview.VerifiedPublicationBinding, error) {
	attempt, evidenceRef, found, err := v.store.Attempt(ctx, obligation.ID, processID)
	if err != nil {
		return implreview.VerifiedPublicationBinding{}, err
	}
	if !found || attempt.AttemptID != processID || attempt.AttemptProofRef == "" || attempt.AttemptProofRef != proofRef || evidenceRef == "" || evidenceRef == proofRef {
		return implreview.VerifiedPublicationBinding{}, errors.New("reviewinvoke: protected attempt proof does not match admitted process")
	}
	return implreview.VerifiedPublicationBinding{PublicationBinding: implreview.PublicationBinding{
		ReviewObligationID: obligation.ID, ReviewProcessID: processID,
		ProposalID: obligation.ProposalRef, ReviewedRevision: obligation.Revision,
		AttemptProofRef: proofRef,
	}, ProofEvidenceRef: evidenceRef}, nil
}

func (v *StoreTerminalVerifier) VerifyPair(ctx context.Context, publication implreview.PublicationTerminal, host worker.ReviewPublicationTerminal) error {
	record, found, err := v.publications.Record(ctx, publication.PublicationAttemptID)
	if err != nil {
		return fmt.Errorf("reviewinvoke: exact-read protected EVT-13: %w", err)
	}
	if !found || record.Stage != publicationpkg.StageTerminal || record.Terminal == nil || *record.Terminal != publication {
		return errors.New("reviewinvoke: EVT-13 does not match protected publication evidence")
	}
	wantRef, err := publicationpkg.TerminalRef(publication)
	if err != nil || publication.EVT13Ref != wantRef {
		return errors.New("reviewinvoke: EVT-13 immutable evidence reference mismatch")
	}
	if publication.ReviewProcessID == "" || publication.ReviewObligationID == "" || publication.ProposalID == "" || publication.ReviewedRevision == "" || publication.EVT13Ref == "" ||
		publication.ReviewProcessID != host.ReviewAttemptID || publication.ReviewObligationID != host.ReviewObligationID || publication.ProposalID != host.ProposalRef || publication.ReviewedRevision != host.Revision ||
		string(publication.Status) != host.Status || publication.ArtifactRef != host.ArtifactRef || publication.NativeReceipt != host.NativeReceipt || publication.AttemptProofRef != host.AttemptProofRef || string(publication.FailureClass) != host.FailureClass {
		return errors.New("reviewinvoke: EVT-13 and EVT-17 are not field-identical")
	}
	switch publication.Status {
	case implreview.PublicationVerified:
		if host.ArtifactRef == "" || host.NativeReceipt == "" || host.AttemptProofRef == "" || host.NativeReceipt == host.AttemptProofRef || host.FailureClass != "" || host.Reason != "" || host.Escalated {
			return errors.New("reviewinvoke: verified publication pair is incomplete")
		}
	case implreview.PublicationFailed:
		if host.FailureClass == "" || host.Reason == "" || !host.Escalated || publication.Reason != host.Reason {
			return errors.New("reviewinvoke: failed publication pair is incomplete")
		}
	default:
		return fmt.Errorf("reviewinvoke: unknown publication status %q", publication.Status)
	}
	return nil
}

func hostedTerminalRef(terminal worker.ReviewPublicationTerminal) (string, error) {
	b, err := json.Marshal(terminal)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return "evidence:evt17:sha256:" + hex.EncodeToString(sum[:]), nil
}
