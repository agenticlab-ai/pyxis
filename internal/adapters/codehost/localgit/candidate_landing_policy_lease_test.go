package localgit

import (
	"context"
	"errors"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
	"github.com/agenticlab-ai/pyxis/internal/featuredrive/candidatereview"
)

type policyChangedAfterCandidateLanding struct {
	codehost.CandidateLandings
	after func()
}

type refusedCandidateReviewLease struct {
	codehost.ReceiptReader
	cause error
}

func (s refusedCandidateReviewLease) WithReview(context.Context, codehost.ReviewBinding, func() error) error {
	return s.cause
}

func TestCandidateLandingPreservesTypedEvidenceLeaseRefusalsWithoutMutation(t *testing.T) {
	t.Parallel()
	for _, cause := range []error{candidatereview.ErrIncomplete, codehost.ErrReviewInvalid, context.Canceled} {
		t.Run(cause.Error(), func(t *testing.T) {
			t.Parallel()
			o, store, packet := readyCandidateAdapter(t)
			store.receipts = refusedCandidateReviewLease{store.receipts, cause}
			before := fixtureGit(t, o.Origin, "show-ref")
			result, err := store.LandCandidate(t.Context(), packet.Binding)
			if result.Created || !errors.Is(err, cause) || !errors.Is(err, codehost.ErrProposalConflict) || fixtureGit(t, o.Origin, "show-ref") != before {
				t.Fatalf("typed no-mutation refusal lost: %+v %v", result, err)
			}
		})
	}
}

func (s policyChangedAfterCandidateLanding) LandCandidate(ctx context.Context, b codehost.ReviewBinding) (codehost.CandidateLandingResult, error) {
	result, err := s.CandidateLandings.LandCandidate(ctx, b)
	if err == nil {
		s.after()
	}
	return result, err
}

func TestCandidateLandingPostcheckUsesEvidenceHeldThroughCommittedPush(t *testing.T) {
	t.Parallel()
	o, store, reader, _, packet := readyCandidateLanding(t)
	proposals, artifacts := NewProposals(store.options), NewReviewArtifacts(store.options)
	changed := false
	landings := policyChangedAfterCandidateLanding{CandidateLandings: store, after: func() {
		changed = true
		fixtureGit(t, o.Origin, "update-ref", reader.policyRef, packet.Binding.Head, reader.policyOID)
		reader.policyOID = packet.Binding.Head
		reader.plan.ScenarioSetFingerprint += "-next-policy"
		reader.fault = "attribution unavailable"
	}}
	gate, err := candidatereview.NewLandingGate(t.Context(), proposals, artifacts, reader, store.receipts, landings)
	if err != nil {
		t.Fatal(err)
	}
	result, err := gate.LandCandidate(t.Context(), packet.Binding)
	if err != nil || !result.Created || !changed {
		t.Fatalf("committed candidate rejected its held evidence: %+v %v", result, err)
	}
	if fixtureGit(t, o.Origin, "rev-parse", result.Prepared.Proposal.TargetRef) != result.Landing.MergeCommit {
		t.Fatal("successful return does not identify the committed mainline landing")
	}
	before := fixtureGit(t, o.Origin, "show-ref")
	if _, err := gate.VerifyCandidateLanding(t.Context(), packet.Binding.ProposalID); err == nil || fixtureGit(t, o.Origin, "show-ref") != before {
		t.Fatal("later recovery ignored current authority or changed origin")
	}
}

func TestCandidateLandingHoldsPolicySourceThroughAtomicPush(t *testing.T) {
	t.Parallel()
	o, store, reader, _, packet := readyCandidateLanding(t)
	attempted := false
	store.push = func(ctx context.Context, root string, args ...string) error {
		attempted = true
		if _, err := proposalGit(ctx, o.Origin, nil, nil, "update-ref", reader.policyRef, packet.Binding.Head, reader.policyOID); err == nil {
			t.Fatal("policy source changed after final verification and before landing")
		}
		_, err := proposalGit(ctx, root, nil, nil, args...)
		return err
	}
	result, err := store.LandCandidate(t.Context(), packet.Binding)
	if err != nil || !result.Created || !attempted {
		t.Fatalf("leased candidate landing failed: %+v %v", result, err)
	}
	if fixtureGit(t, o.Origin, "rev-parse", reader.policyRef) != reader.policyOID {
		t.Fatal("landing changed the policy version")
	}
	// The guard is released after the mutation, so a subsequent legitimate
	// policy revision is possible.
	fixtureGit(t, o.Origin, "update-ref", reader.policyRef, packet.Binding.Head, reader.policyOID)
}

func TestCandidateLandingRefusesPolicyMovementBeforeLeaseWithoutMutation(t *testing.T) {
	t.Parallel()
	o, store, reader, _, packet := readyCandidateLanding(t)
	var before string
	reader.beforeLease = func() {
		fixtureGit(t, o.Origin, "update-ref", reader.policyRef, packet.Binding.Head, reader.policyOID)
		before = fixtureGit(t, o.Origin, "show-ref")
	}
	if result, err := store.LandCandidate(t.Context(), packet.Binding); err == nil || result.Created || before == "" {
		t.Fatalf("obsolete policy source reached landing: %+v %v", result, err)
	}
	if fixtureGit(t, o.Origin, "show-ref") != before {
		t.Fatal("refused lease changed landing refs")
	}
}

type unleasedCandidateReader struct{ candidatereview.Reader }

func TestCandidateLandingRefusesReadersWithoutPolicyLease(t *testing.T) {
	t.Parallel()
	o, store, reader, _, packet := readyCandidateLanding(t)
	proposals, artifacts := NewProposals(store.options), NewReviewArtifacts(store.options)
	requests, err := candidatereview.NewRequests(t.Context(), proposals, artifacts, unleasedCandidateReader{reader})
	if err != nil {
		t.Fatal(err)
	}
	before := fixtureGit(t, o.Origin, "show-ref")
	for _, receipts := range []codehost.ReceiptReader{
		unleasedReceiptReader{store.receipts}, NewCandidateReceipts(store.options, requests),
	} {
		unleased := NewCandidateLandings(store.options, receipts)
		if _, err := unleased.LandCandidate(t.Context(), packet.Binding); err == nil || fixtureGit(t, o.Origin, "show-ref") != before {
			t.Fatal("unleased policy reader reached atomic landing")
		}
	}
}
