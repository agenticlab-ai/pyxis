package reviewgate

import (
	"context"
	"fmt"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
)

// TaskLandingGate rechecks complete policy and review evidence before the
// operator mutation and when recovering a landed task. The drive receives
// only its TaskLandingReader interface.
type TaskLandingGate struct {
	requests *TaskRequests
	receipts codehost.ReceiptReader
	landings codehost.TaskLandings
}

func NewTaskLandingGate(ctx context.Context, proposals codehost.ScopedProposals, artifacts codehost.ReviewArtifacts, reader Reader, receipts codehost.ReceiptReader, landings codehost.TaskLandings) (*TaskLandingGate, error) {
	if receipts == nil || landings == nil {
		return nil, fmt.Errorf("%w: missing task landing ports", ErrIncomplete)
	}
	requests, err := NewTaskRequests(ctx, proposals, artifacts, reader)
	if err != nil {
		return nil, err
	}
	return &TaskLandingGate{requests: requests, receipts: receipts, landings: landings}, nil
}

func (s *TaskLandingGate) LandTask(ctx context.Context, binding codehost.ReviewBinding) (codehost.TaskLandingResult, error) {
	if binding.Validate() != nil || binding.Operation != codehost.TaskMerge {
		return codehost.TaskLandingResult{}, codehost.ErrReviewInvalid
	}
	g, err := s.requests.proposals.Read(ctx, binding.ProposalID)
	if err != nil {
		return codehost.TaskLandingResult{}, err
	}
	if g.Proposal.State == "landed" {
		result, err := s.VerifyTaskLanding(ctx, binding.ProposalID)
		if err == nil && result.Landing.Review.Receipt.Binding != binding {
			err = codehost.ErrReviewInvalid
		}
		return result, err
	}
	packet, err := s.requests.Observe(ctx, binding.ProposalID)
	if err != nil {
		return codehost.TaskLandingResult{}, err
	}
	if packet.Binding != binding {
		return codehost.TaskLandingResult{}, codehost.ErrReviewInvalid
	}
	if _, err := s.receipts.Verify(ctx, binding); err != nil {
		return codehost.TaskLandingResult{}, err
	}
	result, err := s.landings.LandTask(ctx, binding)
	if err != nil {
		return codehost.TaskLandingResult{}, err
	}
	if result.Landing.Review.Receipt.Binding != binding {
		return codehost.TaskLandingResult{}, codehost.ErrReviewInvalid
	}
	// The adapter held this exact policy through final approval verification
	// and the atomic push. A later policy change cannot invalidate the return
	// from that completed transaction. Independent recovery still resolves
	// current policy below.
	return result, s.validate(ctx, result, &packet.Snapshot.Policy)
}

func (s *TaskLandingGate) VerifyTaskLanding(ctx context.Context, id string) (codehost.TaskLandingResult, error) {
	result, err := s.landings.VerifyTaskLanding(ctx, id)
	if err != nil {
		return codehost.TaskLandingResult{}, err
	}
	if result.Landing.Review.Receipt.Binding.ProposalID != id {
		return codehost.TaskLandingResult{}, codehost.ErrReviewInvalid
	}
	return result, s.validate(ctx, result, nil)
}

func (s *TaskLandingGate) validate(ctx context.Context, result codehost.TaskLandingResult, held *Policy) error {
	scope, err := s.requests.scope(ctx)
	if err != nil {
		return err
	}
	l, p := result.Landing, result.Prepared.Proposal
	if l.Validate() != nil || l.Feature != p.Feature || l.Task != p.Task || p.TargetRef != scope.TargetRef ||
		len(result.FeatureHead) != len(p.Head) || strings.Trim(result.FeatureHead, "0123456789abcdef") != "" {
		return codehost.ErrReviewInvalid
	}
	request, err := codehost.ParseTaskRequest(l.Request.Body)
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
	snapshot, err := Parse(l.Aggregate.Body)
	if err != nil {
		return err
	}
	var policy Policy
	if held != nil {
		policy = *held
	} else {
		policy, err = s.requests.reader.Policy(ctx, binding)
		if err != nil {
			return err
		}
	}
	return Validate(binding, snapshot, policy)
}
