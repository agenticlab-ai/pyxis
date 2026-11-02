package candidatereview

import (
	"context"
	"reflect"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
)

// LandingGate is the operator composition's complete current policy and
// attribution predicate. Drives receive only its read-only landing interface.
type LandingGate struct {
	requests *Requests
	receipts codehost.ReceiptReader
	landings codehost.CandidateLandings
}

func NewLandingGate(ctx context.Context, proposals codehost.ScopedProposals, artifacts codehost.ReviewArtifacts, reader Reader, receipts codehost.ReceiptReader, landings codehost.CandidateLandings) (*LandingGate, error) {
	if receipts == nil || landings == nil {
		return nil, ErrIncomplete
	}
	requests, err := NewRequests(ctx, proposals, artifacts, reader)
	if err != nil {
		return nil, err
	}
	return &LandingGate{requests: requests, receipts: receipts, landings: landings}, nil
}

func (s *LandingGate) LandCandidate(ctx context.Context, binding codehost.ReviewBinding) (codehost.CandidateLandingResult, error) {
	if binding.Validate() != nil || binding.Operation != codehost.FeatureFinalMerge {
		return codehost.CandidateLandingResult{}, codehost.ErrReviewInvalid
	}
	g, err := s.requests.proposals.Read(ctx, binding.ProposalID)
	if err != nil {
		return codehost.CandidateLandingResult{}, err
	}
	if g.Proposal.State == "landed" {
		result, err := s.VerifyCandidateLanding(ctx, binding.ProposalID)
		if err == nil && result.Landing.Review.Receipt.Binding != binding {
			err = codehost.ErrReviewInvalid
		}
		return result, err
	}
	packet, err := s.requests.Observe(ctx, binding.ProposalID)
	if err != nil {
		return codehost.CandidateLandingResult{}, err
	}
	if packet.Binding != binding {
		return codehost.CandidateLandingResult{}, codehost.ErrReviewInvalid
	}
	if _, err := s.receipts.Verify(ctx, binding); err != nil {
		return codehost.CandidateLandingResult{}, err
	}
	result, err := s.landings.LandCandidate(ctx, binding)
	if err != nil {
		return codehost.CandidateLandingResult{}, err
	}
	if result.Landing.Review.Receipt.Binding != binding {
		return codehost.CandidateLandingResult{}, codehost.ErrReviewInvalid
	}
	// The host leased this complete policy/evidence/attribution snapshot
	// through the committed push. Later source changes do not retroactively
	// invalidate this transaction's return; independent recovery stays fresh.
	return result, s.validate(ctx, result, &packet.Snapshot)
}

func (s *LandingGate) VerifyCandidateLanding(ctx context.Context, id string) (codehost.CandidateLandingResult, error) {
	result, err := s.landings.VerifyCandidateLanding(ctx, id)
	if err != nil {
		return codehost.CandidateLandingResult{}, err
	}
	if result.Landing.Review.Receipt.Binding.ProposalID != id {
		return codehost.CandidateLandingResult{}, codehost.ErrReviewInvalid
	}
	return result, s.validate(ctx, result, nil)
}

func (s *LandingGate) validate(ctx context.Context, result codehost.CandidateLandingResult, held *Snapshot) error {
	scope, err := s.requests.scope(ctx)
	if err != nil {
		return err
	}
	l, p := result.Landing, result.Prepared.Proposal
	if l.Validate() != nil || l.Candidate.FeatureID != p.Feature || p.TargetRef != scope.TargetRef || p.SourceRef != scope.HeadRef {
		return codehost.ErrReviewInvalid
	}
	for _, oid := range []string{result.MainHead, result.FeatureHead} {
		if len(oid) != len(p.Head) || strings.Trim(oid, "0123456789abcdef") != "" {
			return codehost.ErrReviewInvalid
		}
	}
	request, err := codehost.ParseCandidateRequest(l.Request.Body)
	if err != nil {
		return err
	}
	if err := request.Matches(result.Prepared); err != nil {
		return err
	}
	binding, err := Bind(scope.Origin, result.Prepared)
	if err != nil {
		return err
	}
	snapshot, err := Parse(l.Evidence.Body)
	if err != nil {
		return err
	}
	if held != nil {
		if !reflect.DeepEqual(snapshot, *held) {
			return ErrIncomplete
		}
		return Validate(binding, snapshot, held.Plan)
	}
	return Verify(ctx, binding, snapshot, s.requests.reader)
}
