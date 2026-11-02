package featuredrive

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type finalProposalPublisherFunc func(context.Context, FinalProposal) (FinalProposal, error)

func (f finalProposalPublisherFunc) OpenOrUpdateFinalProposal(ctx context.Context, p FinalProposal) (FinalProposal, error) {
	return f(ctx, p)
}

type verdictPublisherFunc func(context.Context, CandidateVerdict) error

func (f verdictPublisherFunc) PublishCandidateVerdict(ctx context.Context, v CandidateVerdict) error {
	return f(ctx, v)
}

func TestActivateFinalProposalBindsPublishedVerdictToEvidence(t *testing.T) {
	a, _, _ := deliveryFixture()
	a.Proposal.EvidenceID = "evidence"
	a.Proposal.ScenarioSetFingerprint = "scenarios"
	evidence := EvidenceVerdict{FeatureID: a.Proposal.FeatureID, CandidateID: a.Candidate.Digest(), ScenarioSet: a.Proposal.ScenarioSetFingerprint, Accepted: true}
	var published CandidateVerdict
	_, published, err := ActivateFinalProposal(context.Background(), a.Proposal, evidence, FinalActivationCapability{RequiresCandidateVerdict: true, EnforcesCurrentBase: true}, finalProposalPublisherFunc(func(context.Context, FinalProposal) (FinalProposal, error) {
		return a.Proposal, nil
	}), verdictPublisherFunc(func(_ context.Context, v CandidateVerdict) error {
		published = v
		return nil
	}))
	if err != nil {
		t.Fatalf("ActivateFinalProposal() error = %v", err)
	}
	if published.EvidenceID != a.Proposal.EvidenceID || published.ScenarioSetFingerprint != a.Proposal.ScenarioSetFingerprint {
		t.Fatalf("verdict = %+v, want evidence-bound verdict", published)
	}
}

func TestActivateFinalProposalRejectsPublisherEvidenceMutation(t *testing.T) {
	a, _, _ := deliveryFixture()
	a.Proposal.EvidenceID = "evidence"
	a.Proposal.ScenarioSetFingerprint = "scenarios"
	evidence := EvidenceVerdict{FeatureID: a.Proposal.FeatureID, CandidateID: a.Candidate.Digest(), ScenarioSet: a.Proposal.ScenarioSetFingerprint, Accepted: true}
	_, _, err := ActivateFinalProposal(context.Background(), a.Proposal, evidence, FinalActivationCapability{RequiresCandidateVerdict: true, EnforcesCurrentBase: true}, finalProposalPublisherFunc(func(_ context.Context, p FinalProposal) (FinalProposal, error) {
		p.EvidenceID = "different"
		return p, nil
	}), verdictPublisherFunc(func(context.Context, CandidateVerdict) error { return nil }))
	if !errors.Is(err, ErrFinalActivationRefused) {
		t.Fatalf("error = %v, want ErrFinalActivationRefused", err)
	}
}

func TestActivateFinalProposalRejectsPublisherProposalReplacement(t *testing.T) {
	a, _, _ := deliveryFixture()
	a.Proposal.EvidenceID = "evidence"
	a.Proposal.ScenarioSetFingerprint = "scenarios"
	evidence := EvidenceVerdict{FeatureID: a.Proposal.FeatureID, CandidateID: a.Candidate.Digest(), ScenarioSet: a.Proposal.ScenarioSetFingerprint, Accepted: true}
	_, _, err := ActivateFinalProposal(context.Background(), a.Proposal, evidence, FinalActivationCapability{RequiresCandidateVerdict: true, EnforcesCurrentBase: true}, finalProposalPublisherFunc(func(_ context.Context, p FinalProposal) (FinalProposal, error) {
		p.ProposalID = "different-proposal"
		return p, nil
	}), verdictPublisherFunc(func(context.Context, CandidateVerdict) error { return nil }))
	if !errors.Is(err, ErrFinalActivationRefused) {
		t.Fatalf("error = %v, want ErrFinalActivationRefused", err)
	}
}

func TestActivateFinalProposalRejectsIncompleteEvidence(t *testing.T) {
	if _, _, err := ActivateFinalProposal(context.Background(), FinalProposal{}, EvidenceVerdict{}, FinalActivationCapability{}, nil, nil); !errors.Is(err, ErrFinalActivationRefused) {
		t.Fatalf("error = %v, want ErrFinalActivationRefused", err)
	}
}

type deliveryVerifierFunc func(context.Context, AcceptedCandidate, LandedRevision, CurrentMain) (DeliveryProof, error)

func (f deliveryVerifierFunc) VerifyDelivery(ctx context.Context, a AcceptedCandidate, l LandedRevision, m CurrentMain) (DeliveryProof, error) {
	return f(ctx, a, l, m)
}

func deliveryFixture() (AcceptedCandidate, LandedRevision, CurrentMain) {
	c := CandidateIdentity{FeatureID: "feature", FeatureHead: "feature-head", MainHead: "main-head", Commit: "candidate-commit", Tree: "tree", MergePolicyVersion: "policy", BuilderVersion: "builder"}
	p := FinalProposal{FeatureID: "feature", ProposalID: "proposal", Head: "proposal-head", FeatureHead: c.FeatureHead, MainHead: c.MainHead, Candidate: c, EvidenceID: "evidence", ScenarioSetFingerprint: "scenarios"}
	return AcceptedCandidate{Candidate: c, Proposal: p}, LandedRevision{Revision: "landed", Base: c.MainHead, Tree: c.Tree}, CurrentMain{Revision: "main"}
}

func TestVerifyDeliveryFailureNamesRemedyAndRetainsSentinel(t *testing.T) {
	a, l, m := deliveryFixture()
	l.Tree = "wrong-tree"
	err := VerifyDelivery(context.Background(), a, l, m, deliveryVerifierFunc(func(context.Context, AcceptedCandidate, LandedRevision, CurrentMain) (DeliveryProof, error) {
		t.Fatal("verifier must not run when the landed tree is already wrong")
		return DeliveryProof{}, nil
	}))
	if !errors.Is(err, ErrDeliveryUnverified) {
		t.Fatalf("error = %v, want ErrDeliveryUnverified", err)
	}
	var failure *DeliveryFailure
	if !errors.As(err, &failure) {
		t.Fatalf("error = %v, want DeliveryFailure", err)
	}
	if failure.Remedy == "" || !strings.Contains(err.Error(), "remedy:") {
		t.Fatalf("error = %v, want an actionable remedy", err)
	}
}

func TestVerifyDeliveryRejectsPartialProofWithRemedy(t *testing.T) {
	a, l, m := deliveryFixture()
	err := VerifyDelivery(context.Background(), a, l, m, deliveryVerifierFunc(func(context.Context, AcceptedCandidate, LandedRevision, CurrentMain) (DeliveryProof, error) {
		return DeliveryProof{AcceptedBase: true, CandidateTree: true}, nil
	}))
	if !errors.Is(err, ErrDeliveryUnverified) || !strings.Contains(err.Error(), "all three proofs") {
		t.Fatalf("error = %v, want partial-proof refusal with remedy", err)
	}
}

func TestVerifyDeliveryRejectsProposalCandidateIdentityMismatch(t *testing.T) {
	a, l, m := deliveryFixture()
	a.Proposal.FeatureHead = "different-feature-head"
	err := VerifyDelivery(context.Background(), a, l, m, deliveryVerifierFunc(func(context.Context, AcceptedCandidate, LandedRevision, CurrentMain) (DeliveryProof, error) {
		t.Fatal("verifier must not run for inconsistent accepted evidence")
		return DeliveryProof{}, nil
	}))
	if !errors.Is(err, ErrDeliveryUnverified) || !strings.Contains(err.Error(), "delivery evidence is incomplete") {
		t.Fatalf("error = %v, want exact-evidence refusal with remedy", err)
	}
}

func TestVerifyDeliveryRejectsIncompleteFinalProposalIdentity(t *testing.T) {
	a, l, m := deliveryFixture()
	a.Proposal.ProposalID = ""
	err := VerifyDelivery(context.Background(), a, l, m, deliveryVerifierFunc(func(context.Context, AcceptedCandidate, LandedRevision, CurrentMain) (DeliveryProof, error) {
		t.Fatal("verifier must not run for an incomplete final proposal")
		return DeliveryProof{}, nil
	}))
	if !errors.Is(err, ErrDeliveryUnverified) || !strings.Contains(err.Error(), "delivery evidence is incomplete") {
		t.Fatalf("error = %v, want incomplete-proposal refusal with remedy", err)
	}
}

func TestValidateCandidateVerdictRejectsIncompleteIdentity(t *testing.T) {
	a, _, _ := deliveryFixture()
	v := CandidateVerdict{FeatureID: a.Proposal.FeatureID, ProposalID: a.Proposal.ProposalID, ProposalHead: a.Proposal.Head, MainHead: a.Proposal.MainHead, CandidateID: a.Candidate.Digest(), EvidenceID: "evidence", ScenarioSetFingerprint: "scenarios", Accepted: true}
	a.Proposal.EvidenceID = "evidence"
	a.Proposal.ScenarioSetFingerprint = "scenarios"
	a.Proposal.Candidate.BuilderVersion = ""
	if !errors.Is(ValidateCandidateVerdictAt(v, a.Proposal, a.Proposal.Head, a.Candidate.FeatureHead, a.Candidate.MainHead), ErrFinalStale) {
		t.Fatal("incomplete candidate identity must invalidate the verdict")
	}
}
