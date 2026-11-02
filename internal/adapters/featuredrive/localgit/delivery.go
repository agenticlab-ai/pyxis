package localgit

import (
	"context"
	"fmt"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
	"github.com/agenticlab-ai/pyxis/internal/featuredrive"
	"github.com/agenticlab-ai/pyxis/internal/featuredrive/candidatereview"
)

// CandidateDeliveryVerifier consumes the complete landing gate's reader.
// It has no receipt-publication or operator landing method.
type CandidateDeliveryVerifier struct {
	reader codehost.CandidateLandingReader
}

func NewCandidateDeliveryVerifier(reader codehost.CandidateLandingReader) *CandidateDeliveryVerifier {
	return &CandidateDeliveryVerifier{reader: reader}
}

var _ featuredrive.DeliveryVerifier = (*CandidateDeliveryVerifier)(nil)

func (s *CandidateDeliveryVerifier) VerifyDelivery(ctx context.Context, accepted featuredrive.AcceptedCandidate, landed featuredrive.LandedRevision, current featuredrive.CurrentMain) (featuredrive.DeliveryProof, error) {
	if s.reader == nil {
		return featuredrive.DeliveryProof{}, featuredrive.ErrDeliveryUnverified
	}
	result, err := s.reader.VerifyCandidateLanding(ctx, accepted.Proposal.ProposalID)
	if err != nil {
		return featuredrive.DeliveryProof{}, fmt.Errorf("%w: %w", featuredrive.ErrDeliveryUnverified, err)
	}
	l, b := result.Landing, result.Landing.Review.Receipt.Binding
	snapshot, err := candidatereview.Parse(l.Evidence.Body)
	if err != nil {
		return featuredrive.DeliveryProof{}, fmt.Errorf("%w: %w", featuredrive.ErrDeliveryUnverified, err)
	}
	p := accepted.Proposal
	if codehost.FinalCandidate(accepted.Candidate) != l.Candidate || p.Candidate != accepted.Candidate || p.ProposalID != b.ProposalID || p.FeatureID != l.Candidate.FeatureID || p.Head != b.Head ||
		p.FeatureHead != l.Candidate.FeatureHead || p.MainHead != l.Candidate.MainHead || p.ScenarioSetFingerprint != b.ScenarioSetFingerprint || p.EvidenceID != b.Evidence.Digest || snapshot.Binding.Generation.OID != b.ProposalOID ||
		landed.Revision != l.MergeCommit || landed.Base != l.Candidate.MainHead || landed.Tree != l.Candidate.Tree || current.Revision != result.MainHead {
		return featuredrive.DeliveryProof{}, featuredrive.ErrDeliveryUnverified
	}
	return featuredrive.DeliveryProof{AcceptedBase: true, CandidateTree: true, MainAncestor: true}, nil
}
