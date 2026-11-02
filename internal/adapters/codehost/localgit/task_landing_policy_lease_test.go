package localgit

import (
	"context"
	"errors"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
	"github.com/agenticlab-ai/pyxis/internal/executor/reviewgate"
)

type policyChangedAfterTaskLanding struct {
	codehost.TaskLandings
	after func()
}

type refusedTaskReviewLease struct {
	codehost.ReceiptReader
	cause error
}

func (s refusedTaskReviewLease) WithReview(context.Context, codehost.ReviewBinding, func() error) error {
	return s.cause
}

func TestTaskLandingPreservesTypedPolicyLeaseRefusalsWithoutMutation(t *testing.T) {
	t.Parallel()
	for _, cause := range []error{reviewgate.ErrPolicyStale, codehost.ErrReviewInvalid, context.Canceled} {
		t.Run(cause.Error(), func(t *testing.T) {
			t.Parallel()
			o, store, packet := readyTaskAdapter(t)
			store.receipts = refusedTaskReviewLease{store.receipts, cause}
			before := fixtureGit(t, o.Origin, "show-ref")
			result, err := store.LandTask(t.Context(), packet.Binding)
			if result.Created || !errors.Is(err, cause) || !errors.Is(err, codehost.ErrProposalConflict) || fixtureGit(t, o.Origin, "show-ref") != before {
				t.Fatalf("typed no-mutation refusal lost: %+v %v", result, err)
			}
		})
	}
}

func (s policyChangedAfterTaskLanding) LandTask(ctx context.Context, b codehost.ReviewBinding) (codehost.TaskLandingResult, error) {
	result, err := s.TaskLandings.LandTask(ctx, b)
	if err == nil {
		s.after()
	}
	return result, err
}

func TestTaskLandingPostcheckUsesPolicyHeldThroughCommittedPush(t *testing.T) {
	t.Parallel()
	o, store, reader, _, packet := readyTaskLanding(t)
	proposals, artifacts := NewProposals(store.options), NewReviewArtifacts(store.options)
	changed := false
	landings := policyChangedAfterTaskLanding{TaskLandings: store, after: func() {
		changed = true
		fixtureGit(t, o.Origin, "update-ref", reader.policyRef, packet.Binding.Head, reader.policyOID)
		reader.policyOID = packet.Binding.Head
		reader.policy.Prompts += "-next-policy"
	}}
	gate, err := reviewgate.NewTaskLandingGate(t.Context(), proposals, artifacts, reader, store.receipts, landings)
	if err != nil {
		t.Fatal(err)
	}
	result, err := gate.LandTask(t.Context(), packet.Binding)
	if err != nil || !result.Created || !changed {
		t.Fatalf("committed landing rejected its held policy: %+v %v", result, err)
	}
	if fixtureGit(t, o.Origin, "rev-parse", result.Prepared.Proposal.TargetRef) != result.Landing.MergeCommit {
		t.Fatal("successful return does not identify the committed feature landing")
	}
	before := fixtureGit(t, o.Origin, "show-ref")
	if _, err := gate.VerifyTaskLanding(t.Context(), packet.Binding.ProposalID); err == nil || fixtureGit(t, o.Origin, "show-ref") != before {
		t.Fatal("later recovery ignored current policy or changed origin")
	}
}

func TestTaskLandingHoldsPolicySourceThroughAtomicPush(t *testing.T) {
	t.Parallel()
	o, store, reader, _, packet := readyTaskLanding(t)
	attempted := false
	store.push = func(ctx context.Context, root string, args ...string) error {
		attempted = true
		if _, err := proposalGit(ctx, o.Origin, nil, nil, "update-ref", reader.policyRef, packet.Binding.Head, reader.policyOID); err == nil {
			t.Fatal("policy source changed after final verification and before landing")
		}
		_, err := proposalGit(ctx, root, nil, nil, args...)
		return err
	}
	result, err := store.LandTask(t.Context(), packet.Binding)
	if err != nil || !result.Created || !attempted {
		t.Fatalf("leased task landing failed: %+v %v", result, err)
	}
	if fixtureGit(t, o.Origin, "rev-parse", reader.policyRef) != reader.policyOID {
		t.Fatal("landing changed the policy version")
	}
	// The guard is released after the mutation, so a subsequent legitimate
	// policy revision is possible.
	fixtureGit(t, o.Origin, "update-ref", reader.policyRef, packet.Binding.Head, reader.policyOID)
}

func TestTaskLandingRefusesPolicyMovementBeforeLeaseWithoutMutation(t *testing.T) {
	t.Parallel()
	o, store, reader, _, packet := readyTaskLanding(t)
	var before string
	reader.beforeLease = func() {
		fixtureGit(t, o.Origin, "update-ref", reader.policyRef, packet.Binding.Head, reader.policyOID)
		before = fixtureGit(t, o.Origin, "show-ref")
	}
	if result, err := store.LandTask(t.Context(), packet.Binding); err == nil || result.Created || before == "" {
		t.Fatalf("obsolete policy source reached landing: %+v %v", result, err)
	}
	if fixtureGit(t, o.Origin, "show-ref") != before {
		t.Fatal("refused lease changed landing refs")
	}
}

type unleasedTaskReader struct{ reviewgate.Reader }
type unleasedReceiptReader struct{ codehost.ReceiptReader }

func TestTaskLandingRefusesReadersWithoutPolicyLease(t *testing.T) {
	t.Parallel()
	o, store, reader, _, packet := readyTaskLanding(t)
	proposals, artifacts := NewProposals(store.options), NewReviewArtifacts(store.options)
	requests, err := reviewgate.NewTaskRequests(t.Context(), proposals, artifacts, unleasedTaskReader{reader})
	if err != nil {
		t.Fatal(err)
	}
	before := fixtureGit(t, o.Origin, "show-ref")
	for _, receipts := range []codehost.ReceiptReader{
		unleasedReceiptReader{store.receipts}, NewTaskReceipts(store.options, requests),
	} {
		unleased := NewTaskLandings(store.options, receipts)
		if _, err := unleased.LandTask(t.Context(), packet.Binding); err == nil || fixtureGit(t, o.Origin, "show-ref") != before {
			t.Fatal("unleased policy reader reached atomic landing")
		}
	}
	for _, receipts := range []*ReceiptStore{
		NewReceipts(o), NewTaskReceipts(store.options, nil), store.receipts.(*ReceiptStore),
	} {
		if err := receipts.WithReview(t.Context(), packet.Binding, nil); err == nil {
			t.Fatal("missing policy lease or callback accepted")
		}
	}
	if fixtureGit(t, o.Origin, "show-ref") != before {
		t.Fatal("missing lease or callback changed origin refs")
	}
}
