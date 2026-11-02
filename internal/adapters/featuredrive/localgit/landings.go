package localgit

import (
	"context"
	"fmt"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
	"github.com/agenticlab-ai/pyxis/internal/featuredrive"
)

// The injected reader is the reviewgate's complete policy/proof verifier.
// This drive adapter has no operator landing or receipt-publication method.
type TaskLandingVerifier struct {
	reader codehost.TaskLandingReader
}

func NewTaskLandingVerifier(reader codehost.TaskLandingReader) *TaskLandingVerifier {
	return &TaskLandingVerifier{reader: reader}
}

var _ featuredrive.LandingVerifier = (*TaskLandingVerifier)(nil)

func (s *TaskLandingVerifier) VerifyLanding(ctx context.Context, proposal featuredrive.ProposalObservation, review featuredrive.ReviewObservation, featureHead string) (featuredrive.LandingProof, error) {
	if s.reader == nil || review.State != "approved" || review.ProposalID != proposal.ProposalID || review.Head != proposal.Head || !proposal.ChecksGreen {
		return featuredrive.LandingProof{}, featuredrive.ErrLandingNotProven
	}
	result, err := s.reader.VerifyTaskLanding(ctx, proposal.ProposalID)
	if err != nil {
		return featuredrive.LandingProof{}, fmt.Errorf("%w: %w", featuredrive.ErrLandingNotProven, err)
	}
	b := result.Landing.Review.Receipt.Binding
	if result.Landing.Task != proposal.TaskID || b.ProposalID != proposal.ProposalID || b.Head != proposal.Head || b.TargetRef != proposal.Base || result.FeatureHead != featureHead || (proposal.Landed != "" && proposal.Landed != result.Landing.MergeCommit) {
		return featuredrive.LandingProof{}, featuredrive.ErrLandingNotProven
	}
	return featuredrive.LandingProof{ReviewedHead: b.Head, ReviewedBase: b.TargetRef, LandedRevision: result.Landing.MergeCommit, FeatureHead: result.FeatureHead, FeatureBase: b.TargetRef}, nil
}
