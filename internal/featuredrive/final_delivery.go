package featuredrive

import (
	"context"
	"errors"
	"fmt"
)

var (
	ErrFinalActivationRefused = errors.New("feature drive: final activation refused")
	ErrFinalStale             = errors.New("feature drive: final verdict is stale")
	ErrDeliveryUnverified     = errors.New("feature drive: delivery verification failed")
)

// DeliveryFailure carries the failed proof boundary and the one safe next
// step. It deliberately remains an error (and wraps ErrDeliveryUnverified) so
// callers cannot accidentally treat a diagnostic as a successful delivery.
type DeliveryFailure struct {
	Boundary string
	Remedy   string
}

func (e *DeliveryFailure) Error() string {
	return fmt.Sprintf("%s; remedy: %s", e.Boundary, e.Remedy)
}
func (e *DeliveryFailure) Unwrap() error { return ErrDeliveryUnverified }

func deliveryFailure(boundary, remedy string) error {
	return &DeliveryFailure{Boundary: boundary, Remedy: remedy}
}

// FinalProposal is the immutable identity advertised by the final proposal.
// ProposalID is retained on updates so a drive can never open a second final
// proposal for the same feature.
type FinalProposal struct {
	FeatureID              string            `json:"feature_id"`
	ProposalID             string            `json:"proposal_id"`
	Head                   string            `json:"head"`
	FeatureHead            string            `json:"feature_head"`
	MainHead               string            `json:"main_head"`
	Candidate              CandidateIdentity `json:"candidate"`
	ScenarioSetFingerprint string            `json:"scenario_set_fingerprint"`
	EvidenceID             string            `json:"evidence_id"`
}

type FinalProposalPublisher interface {
	OpenOrUpdateFinalProposal(context.Context, FinalProposal) (FinalProposal, error)
}

// FinalActivationCapability is deliberately stronger than a notification or
// a boolean "protected" badge. Both guarantees are required before action.
type FinalActivationCapability struct {
	RequiresCandidateVerdict bool
	EnforcesCurrentBase      bool
}

type CandidateVerdict struct {
	FeatureID              string `json:"feature_id"`
	ProposalID             string `json:"proposal_id"`
	ProposalHead           string `json:"proposal_head"`
	MainHead               string `json:"main_head"`
	CandidateID            string `json:"candidate_id"`
	EvidenceID             string `json:"evidence_id"`
	ScenarioSetFingerprint string `json:"scenario_set_fingerprint"`
	Accepted               bool   `json:"accepted"`
	Stale                  bool   `json:"stale"`
}

type CandidateVerdictPublisher interface {
	PublishCandidateVerdict(context.Context, CandidateVerdict) error
}

// ActivateFinalProposal publishes exactly one proposal and its verdict. It
// has no approval or merge port by construction.
func ActivateFinalProposal(ctx context.Context, proposal FinalProposal, evidence EvidenceVerdict, capability FinalActivationCapability, publisher FinalProposalPublisher, verdictPublisher CandidateVerdictPublisher) (FinalProposal, CandidateVerdict, error) {
	if proposal.FeatureID == "" || proposal.Head == "" || proposal.FeatureHead == "" || proposal.MainHead == "" || proposal.EvidenceID == "" || proposal.ScenarioSetFingerprint == "" || !validCandidateIdentity(proposal.Candidate) || proposal.Candidate.FeatureID != proposal.FeatureID || proposal.Candidate.FeatureHead != proposal.FeatureHead || proposal.Candidate.MainHead != proposal.MainHead || !evidence.Accepted || evidence.FeatureID != proposal.FeatureID || evidence.CandidateID != proposal.Candidate.Digest() || evidence.ScenarioSet != proposal.ScenarioSetFingerprint {
		return FinalProposal{}, CandidateVerdict{}, fmt.Errorf("%w: evidence and exact candidate heads are required", ErrFinalActivationRefused)
	}
	if !capability.RequiresCandidateVerdict || !capability.EnforcesCurrentBase {
		return FinalProposal{}, CandidateVerdict{}, fmt.Errorf("%w: medium cannot enforce required verdict and current base", ErrFinalActivationRefused)
	}
	if publisher == nil || verdictPublisher == nil {
		return FinalProposal{}, CandidateVerdict{}, fmt.Errorf("%w: proposal and verdict publishers are required", ErrFinalActivationRefused)
	}
	opened, err := publisher.OpenOrUpdateFinalProposal(ctx, proposal)
	if err != nil {
		return FinalProposal{}, CandidateVerdict{}, err
	}
	if opened.FeatureID != proposal.FeatureID || opened.Head != proposal.Head || opened.FeatureHead != proposal.FeatureHead || opened.MainHead != proposal.MainHead || opened.Candidate.Digest() != proposal.Candidate.Digest() || opened.ScenarioSetFingerprint != proposal.ScenarioSetFingerprint || opened.EvidenceID != proposal.EvidenceID || opened.ProposalID == "" || (proposal.ProposalID != "" && opened.ProposalID != proposal.ProposalID) {
		return FinalProposal{}, CandidateVerdict{}, fmt.Errorf("%w: publisher returned a different proposal identity", ErrFinalActivationRefused)
	}
	v := CandidateVerdict{FeatureID: opened.FeatureID, ProposalID: opened.ProposalID, ProposalHead: opened.Head, MainHead: opened.MainHead, CandidateID: opened.Candidate.Digest(), EvidenceID: opened.EvidenceID, ScenarioSetFingerprint: opened.ScenarioSetFingerprint, Accepted: true}
	if err := verdictPublisher.PublishCandidateVerdict(ctx, v); err != nil {
		return FinalProposal{}, CandidateVerdict{}, err
	}
	return opened, v, nil
}

// ValidateCandidateVerdict is called immediately before any final action and
// is intentionally independent of the platform's merged/review badge.
func ValidateCandidateVerdict(v CandidateVerdict, proposal FinalProposal, currentFeatureHead, currentMainHead string) error {
	return ValidateCandidateVerdictAt(v, proposal, proposal.Head, currentFeatureHead, currentMainHead)
}

// ValidateCandidateVerdictAt additionally compares the freshly observed
// proposal head, closing the review/head movement race.
func ValidateCandidateVerdictAt(v CandidateVerdict, proposal FinalProposal, currentProposalHead, currentFeatureHead, currentMainHead string) error {
	if !v.Accepted || v.Stale || proposal.FeatureID == "" || proposal.ProposalID == "" || !validCandidateIdentity(proposal.Candidate) || proposal.Head == "" || proposal.FeatureHead == "" || proposal.MainHead == "" || proposal.EvidenceID == "" || proposal.ScenarioSetFingerprint == "" || v.FeatureID == "" || v.ProposalID == "" || v.CandidateID == "" || v.EvidenceID == "" || v.ScenarioSetFingerprint == "" || currentProposalHead == "" || currentFeatureHead == "" || currentMainHead == "" || v.FeatureID != proposal.FeatureID || v.ProposalID != proposal.ProposalID || v.ProposalHead != currentProposalHead || v.MainHead != currentMainHead || proposal.FeatureHead != currentFeatureHead || proposal.MainHead != currentMainHead || v.CandidateID != proposal.Candidate.Digest() || v.EvidenceID != proposal.EvidenceID || v.ScenarioSetFingerprint != proposal.ScenarioSetFingerprint {
		return fmt.Errorf("%w: proposal or target heads moved", ErrFinalStale)
	}
	return nil
}

type AcceptedCandidate struct {
	Candidate CandidateIdentity
	Proposal  FinalProposal
}
type LandedRevision struct {
	Revision string
	Base     string
	Tree     string
}
type CurrentMain struct {
	Revision string
}
type DeliveryProof struct {
	AcceptedBase  bool
	CandidateTree bool
	MainAncestor  bool
}

type DeliveryVerifier interface {
	VerifyDelivery(context.Context, AcceptedCandidate, LandedRevision, CurrentMain) (DeliveryProof, error)
}

// VerifyDelivery is the sole delivery predicate. A merged badge is not an
// input: adapters must prove the named base, byte-identical tree, and ancestry.
func VerifyDelivery(ctx context.Context, accepted AcceptedCandidate, landed LandedRevision, current CurrentMain, verifier DeliveryVerifier) error {
	if verifier == nil || !validCandidateIdentity(accepted.Candidate) || accepted.Proposal.FeatureID == "" || accepted.Proposal.ProposalID == "" || accepted.Proposal.Head == "" || accepted.Proposal.FeatureHead == "" || accepted.Proposal.MainHead == "" || accepted.Proposal.EvidenceID == "" || accepted.Proposal.ScenarioSetFingerprint == "" || !validCandidateIdentity(accepted.Proposal.Candidate) || accepted.Proposal.Candidate.Digest() != accepted.Candidate.Digest() || accepted.Proposal.FeatureID != accepted.Candidate.FeatureID || accepted.Proposal.FeatureHead != accepted.Candidate.FeatureHead || accepted.Proposal.MainHead != accepted.Candidate.MainHead || landed.Revision == "" || landed.Base == "" || landed.Tree == "" || current.Revision == "" {
		return deliveryFailure("delivery evidence is incomplete", "retain ownership and re-observe the accepted base, candidate tree, landed revision, and current main")
	}
	if landed.Base != accepted.Proposal.MainHead || landed.Tree != accepted.Candidate.Tree {
		return deliveryFailure("landed base or candidate tree differs from the accepted proposal", "retain ownership and correct or dispose the proposal, then re-observe landing")
	}
	proof, err := verifier.VerifyDelivery(ctx, accepted, landed, current)
	if err != nil {
		return fmt.Errorf("%w: %v; remedy: retain ownership and correct the landing evidence, then re-observe", ErrDeliveryUnverified, err)
	}
	if !proof.AcceptedBase || !proof.CandidateTree || !proof.MainAncestor {
		return deliveryFailure("accepted base, candidate tree, and current-main ancestry proof is incomplete", "retain ownership and repair delivery until all three proofs succeed")
	}
	return nil
}

type CleanupIntent struct {
	FeatureID string
	Target    string
	Epoch     uint64
}
type RetentionPolicy interface {
	AuthorizeCleanup(context.Context, CleanupIntent) (bool, error)
}

// DeliveryTransition fixes the terminal ordering: proof, de-route, release,
// then policy-authorized cleanup. Cleanup is never attempted on failed proof.
func DeliveryTransition(ctx context.Context, accepted AcceptedCandidate, landed LandedRevision, current CurrentMain, verifier DeliveryVerifier, removeRouting func(context.Context) error, releaseOwnership func(context.Context) error, policy RetentionPolicy, cleanup func(context.Context, CleanupIntent) error, intent CleanupIntent) error {
	if err := VerifyDelivery(ctx, accepted, landed, current, verifier); err != nil {
		return err
	}
	if removeRouting == nil || releaseOwnership == nil {
		return fmt.Errorf("%w: terminal effects unavailable", ErrDeliveryUnverified)
	}
	if err := removeRouting(ctx); err != nil {
		return fmt.Errorf("remove routing: %w", err)
	}
	if err := releaseOwnership(ctx); err != nil {
		return fmt.Errorf("release ownership: %w", err)
	}
	if policy == nil || cleanup == nil {
		return nil
	}
	authorized, err := policy.AuthorizeCleanup(ctx, intent)
	if err != nil {
		return fmt.Errorf("authorize cleanup: %w", err)
	}
	if !authorized {
		return nil
	}
	return cleanup(ctx, intent)
}
