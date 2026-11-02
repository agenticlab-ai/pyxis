package featuredrive

import (
	"context"
	"errors"
	"fmt"
)

// ProposalObservation is a fresh read of a task proposal. A proposal is not
// completion evidence; it is only the object to which landing proof binds.
type ProposalObservation struct {
	TaskID      string
	ProposalID  string
	Head        string
	Base        string
	Landed      string
	ChecksGreen bool
	Open        bool
}

type ReviewObservation struct {
	ProposalID string
	Head       string
	State      string // open, changes-requested, approved
	Comments   []ReviewComment
}

type ReviewComment struct {
	ID        string
	Body      string
	InCharter bool
}

type LandingProof struct {
	ReviewedHead   string
	ReviewedBase   string
	LandedRevision string
	FeatureHead    string
	FeatureBase    string
}

var (
	ErrLandingNotProven = errors.New("feature drive: landing proof failed")
	ErrFeedbackScope    = errors.New("feature drive: feedback scope is not classified")
)

// LandingVerifier owns all medium-specific reads needed to prove a landing.
// Implementations must perform fresh reads; badges and cached observations are
// deliberately not part of this interface.
type LandingVerifier interface {
	VerifyLanding(context.Context, ProposalObservation, ReviewObservation, string) (LandingProof, error)
}

type LandingRecorder interface {
	RecordLanding(context.Context, string, string, LandingProof) error
}

type LinkedWork struct {
	ID        string
	Title     string
	BlockedBy string
	Proposal  string
	Reason    string
}

type CreateLinkedWork interface {
	CreateLinkedWork(context.Context, LinkedWork) (string, error)
}

type RouteFeedbackRepair interface {
	RouteFeedbackRepair(context.Context, string, string) error
}

// RecordLanding is idempotent at the port boundary: a recorder must treat the
// same task/proposal/revision tuple as a no-op. Nothing calls it until proof is
// complete, so invalid proposals can unlock no dependents.
func RecordLanding(ctx context.Context, verifier LandingVerifier, recorder LandingRecorder, proposal ProposalObservation, review ReviewObservation, featureHead string) error {
	if verifier == nil || recorder == nil || proposal.TaskID == "" || proposal.ProposalID == "" {
		return fmt.Errorf("%w: incomplete landing inputs", ErrLandingNotProven)
	}
	// Review is a necessary precondition, not a hint from the code host. Keep
	// every non-approved state incomplete and reject observations belonging to a
	// different proposal or revision before consulting the landing adapter.
	if (!proposal.Open && proposal.Landed == "") || !proposal.ChecksGreen || review.ProposalID != proposal.ProposalID || review.Head != proposal.Head || review.State != "approved" {
		return fmt.Errorf("%w: proposal is not approved, green, and reviewed at its current head", ErrLandingNotProven)
	}
	if proposal.Head == "" || proposal.Base == "" || featureHead == "" {
		return fmt.Errorf("%w: missing proposal head, feature base, or feature head", ErrLandingNotProven)
	}
	proof, err := verifier.VerifyLanding(ctx, proposal, review, featureHead)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrLandingNotProven, err)
	}
	// When the proposal observation includes a landed revision, bind the proof
	// to that exact object as well. An adapter may omit it when the code host
	// exposes the landing only through its fresh verification read, but it must
	// never be allowed to substitute a different observed revision.
	if proof.ReviewedHead == "" || proof.ReviewedBase == "" || proof.LandedRevision == "" || proof.FeatureHead == "" || proof.FeatureBase == "" || proof.ReviewedHead != proposal.Head || proof.ReviewedBase != proposal.Base || proof.FeatureHead != featureHead || proof.FeatureBase != proposal.Base || (proposal.Landed != "" && proof.LandedRevision != proposal.Landed) {
		return fmt.Errorf("%w: reviewed head, landed revision, or feature head mismatch", ErrLandingNotProven)
	}
	return recorder.RecordLanding(ctx, proposal.TaskID, proposal.ProposalID, proof)
}

// HandleReview routes only in-charter comments to the existing proposal. An
// out-of-charter comment creates one linked, blocked work item per comment id.
func HandleReview(ctx context.Context, repair RouteFeedbackRepair, linked CreateLinkedWork, review ReviewObservation) error {
	if review.ProposalID == "" {
		return fmt.Errorf("%w: missing proposal", ErrFeedbackScope)
	}
	seen := make(map[string]bool, len(review.Comments))
	for _, c := range review.Comments {
		if c.ID == "" {
			return fmt.Errorf("%w: comment has no stable id", ErrFeedbackScope)
		}
		if seen[c.ID] {
			continue
		}
		seen[c.ID] = true
		if c.InCharter {
			if repair == nil {
				return fmt.Errorf("%w: repair route unavailable", ErrFeedbackScope)
			}
			if err := repair.RouteFeedbackRepair(ctx, review.ProposalID, c.ID); err != nil {
				return err
			}
			continue
		}
		if linked == nil {
			return fmt.Errorf("%w: linked-work route unavailable", ErrFeedbackScope)
		}
		if _, err := linked.CreateLinkedWork(ctx, LinkedWork{ID: c.ID, Title: "Review feedback: " + c.ID, BlockedBy: review.ProposalID, Proposal: review.ProposalID, Reason: c.Body}); err != nil {
			return err
		}
	}
	return nil
}

type FrontierTask struct {
	ID                    string
	Open, Claimed, Landed bool
	BlockedBy             []string
}

// ReadyFrontier applies the one completion predicate used by routing. Stable
// ordering makes duplicate touches and frontier projections deterministic.
func ReadyFrontier(tasks []FrontierTask) []string {
	landed := map[string]bool{}
	for _, t := range tasks {
		if t.Landed {
			landed[t.ID] = true
		}
	}
	var ready []string
	for _, t := range tasks {
		if !t.Open || t.Claimed || t.Landed {
			continue
		}
		ok := true
		for _, b := range t.BlockedBy {
			if !landed[b] {
				ok = false
				break
			}
		}
		if ok {
			ready = append(ready, t.ID)
		}
	}
	return ready
}
