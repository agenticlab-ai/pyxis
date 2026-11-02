package localgit

import (
	"context"
	"errors"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
	"github.com/agenticlab-ai/pyxis/internal/featuredrive"
)

type landingReader struct {
	result codehost.TaskLandingResult
	err    error
	calls  int
}

func (r *landingReader) VerifyTaskLanding(context.Context, string) (codehost.TaskLandingResult, error) {
	r.calls++
	return r.result, r.err
}

func TestTaskLandingVerifierRequiresCompleteCurrentMatchingProof(t *testing.T) {
	for _, fault := range []string{"", "unknown observed landing", "missing reader", "pending review", "foreign review", "old reviewed head", "red checks", "missing proof", "foreign task", "foreign proposal", "old head", "wrong target", "old feature head", "different landing"} {
		t.Run(fault, func(t *testing.T) {
			proposal := featuredrive.ProposalObservation{TaskID: "T1", ProposalID: "task-widget-t1", Head: "reviewed", Base: "refs/heads/feature/widget", Landed: "landed", ChecksGreen: true}
			review := featuredrive.ReviewObservation{ProposalID: proposal.ProposalID, Head: proposal.Head, State: "approved"}
			binding := codehost.ReviewBinding{ProposalID: proposal.ProposalID, Head: proposal.Head, TargetRef: proposal.Base}
			reader := &landingReader{result: codehost.TaskLandingResult{FeatureHead: "feature", Landing: codehost.TaskLanding{Task: proposal.TaskID, MergeCommit: proposal.Landed, Review: codehost.ReceiptRecord{Receipt: codehost.ReviewReceipt{Binding: binding}}}}}
			verifier := NewTaskLandingVerifier(reader)
			switch fault {
			case "unknown observed landing":
				proposal.Landed = ""
			case "missing reader":
				verifier = NewTaskLandingVerifier(nil)
			case "pending review":
				review.State = "pending"
			case "foreign review":
				review.ProposalID += "-other"
			case "old reviewed head":
				review.Head += "-old"
			case "red checks":
				proposal.ChecksGreen = false
			case "missing proof":
				reader.err = errors.New("retained aggregate absent")
			case "foreign task":
				reader.result.Landing.Task += "-other"
			case "foreign proposal":
				reader.result.Landing.Review.Receipt.Binding.ProposalID += "-other"
			case "old head":
				reader.result.Landing.Review.Receipt.Binding.Head += "-old"
			case "wrong target":
				reader.result.Landing.Review.Receipt.Binding.TargetRef += "-other"
			case "old feature head":
				reader.result.FeatureHead += "-old"
			case "different landing":
				proposal.Landed += "-other"
			}
			proof, err := verifier.VerifyLanding(t.Context(), proposal, review, "feature")
			if fault == "" || fault == "unknown observed landing" {
				if err != nil || proof.LandedRevision != "landed" || proof.ReviewedHead != proposal.Head || reader.calls != 1 {
					t.Fatalf("verified task refused: %+v %v", proof, err)
				}
			} else if !errors.Is(err, featuredrive.ErrLandingNotProven) || proof != (featuredrive.LandingProof{}) {
				t.Fatalf("unproven task allowed progression: %+v %v", proof, err)
			}
		})
	}
}
