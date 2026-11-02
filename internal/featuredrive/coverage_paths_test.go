package featuredrive

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

type feedbackRepairFunc func(context.Context, string, string) error

func (f feedbackRepairFunc) RouteFeedbackRepair(ctx context.Context, proposal, comment string) error {
	return f(ctx, proposal, comment)
}

type linkedWorkFunc func(context.Context, LinkedWork) (string, error)

func (f linkedWorkFunc) CreateLinkedWork(ctx context.Context, work LinkedWork) (string, error) {
	return f(ctx, work)
}

func TestHandleReviewRoutesEachStableCommentExactlyOnce(t *testing.T) {
	var repaired []string
	var linked []LinkedWork
	err := HandleReview(t.Context(), feedbackRepairFunc(func(_ context.Context, proposal, comment string) error {
		repaired = append(repaired, proposal+":"+comment)
		return nil
	}), linkedWorkFunc(func(_ context.Context, work LinkedWork) (string, error) {
		linked = append(linked, work)
		return "linked-" + work.ID, nil
	}), ReviewObservation{ProposalID: "P1", Comments: []ReviewComment{
		{ID: "C1", Body: "in scope", InCharter: true},
		{ID: "C1", Body: "duplicate", InCharter: true},
		{ID: "C2", Body: "follow-up", InCharter: false},
	}})
	if err != nil || !reflect.DeepEqual(repaired, []string{"P1:C1"}) || len(linked) != 1 || linked[0].BlockedBy != "P1" || linked[0].Reason != "follow-up" {
		t.Fatalf("repaired=%v linked=%#v err=%v", repaired, linked, err)
	}

	for name, review := range map[string]ReviewObservation{
		"missing proposal": {},
		"missing comment":  {ProposalID: "P1", Comments: []ReviewComment{{}}},
	} {
		t.Run(name, func(t *testing.T) {
			if err := HandleReview(t.Context(), nil, nil, review); !errors.Is(err, ErrFeedbackScope) {
				t.Fatalf("error=%v", err)
			}
		})
	}
	if err := HandleReview(t.Context(), nil, linkedWorkFunc(func(context.Context, LinkedWork) (string, error) { return "", nil }), ReviewObservation{ProposalID: "P1", Comments: []ReviewComment{{ID: "C1", InCharter: true}}}); !errors.Is(err, ErrFeedbackScope) {
		t.Fatalf("missing repair error=%v", err)
	}
	if err := HandleReview(t.Context(), feedbackRepairFunc(func(context.Context, string, string) error { return nil }), nil, ReviewObservation{ProposalID: "P1", Comments: []ReviewComment{{ID: "C1"}}}); !errors.Is(err, ErrFeedbackScope) {
		t.Fatalf("missing linked-work error=%v", err)
	}
	want := errors.New("route failed")
	if err := HandleReview(t.Context(), feedbackRepairFunc(func(context.Context, string, string) error { return want }), nil, ReviewObservation{ProposalID: "P1", Comments: []ReviewComment{{ID: "C1", InCharter: true}}}); !errors.Is(err, want) {
		t.Fatalf("repair error=%v", err)
	}
	if err := HandleReview(t.Context(), nil, linkedWorkFunc(func(context.Context, LinkedWork) (string, error) { return "", want }), ReviewObservation{ProposalID: "P1", Comments: []ReviewComment{{ID: "C1"}}}); !errors.Is(err, want) {
		t.Fatalf("linked-work error=%v", err)
	}
}

type retentionPolicyFunc func(context.Context, CleanupIntent) (bool, error)

func (f retentionPolicyFunc) AuthorizeCleanup(ctx context.Context, intent CleanupIntent) (bool, error) {
	return f(ctx, intent)
}

func TestCandidateVerdictAndDeliveryTransitionBindEveryTerminalEffect(t *testing.T) {
	accepted, landed, current := deliveryFixture()
	v := CandidateVerdict{
		FeatureID: accepted.Proposal.FeatureID, ProposalID: accepted.Proposal.ProposalID,
		ProposalHead: accepted.Proposal.Head, MainHead: accepted.Proposal.MainHead,
		CandidateID: accepted.Candidate.Digest(), EvidenceID: accepted.Proposal.EvidenceID,
		ScenarioSetFingerprint: accepted.Proposal.ScenarioSetFingerprint, Accepted: true,
	}
	if err := ValidateCandidateVerdict(v, accepted.Proposal, accepted.Candidate.FeatureHead, accepted.Candidate.MainHead); err != nil {
		t.Fatalf("valid verdict: %v", err)
	}
	v.Stale = true
	if err := ValidateCandidateVerdict(v, accepted.Proposal, accepted.Candidate.FeatureHead, accepted.Candidate.MainHead); !errors.Is(err, ErrFinalStale) {
		t.Fatalf("stale verdict error=%v", err)
	}

	verifier := deliveryVerifierFunc(func(context.Context, AcceptedCandidate, LandedRevision, CurrentMain) (DeliveryProof, error) {
		return DeliveryProof{AcceptedBase: true, CandidateTree: true, MainAncestor: true}, nil
	})
	var order []string
	effect := func(name string) func(context.Context) error {
		return func(context.Context) error { order = append(order, name); return nil }
	}
	intent := CleanupIntent{FeatureID: "feature", Target: "branch", Epoch: 1}
	err := DeliveryTransition(t.Context(), accepted, landed, current, verifier, effect("routing"), effect("ownership"), retentionPolicyFunc(func(context.Context, CleanupIntent) (bool, error) {
		order = append(order, "policy")
		return true, nil
	}), func(context.Context, CleanupIntent) error { order = append(order, "cleanup"); return nil }, intent)
	if err != nil || !reflect.DeepEqual(order, []string{"routing", "ownership", "policy", "cleanup"}) {
		t.Fatalf("order=%v err=%v", order, err)
	}

	if err := DeliveryTransition(t.Context(), accepted, landed, current, verifier, nil, effect("ownership"), nil, nil, intent); !errors.Is(err, ErrDeliveryUnverified) {
		t.Fatalf("missing effect error=%v", err)
	}
	stop := errors.New("effect stopped")
	if err := DeliveryTransition(t.Context(), accepted, landed, current, verifier, func(context.Context) error { return stop }, effect("ownership"), nil, nil, intent); !errors.Is(err, stop) || !strings.Contains(err.Error(), "remove routing") {
		t.Fatalf("routing error=%v", err)
	}
	if err := DeliveryTransition(t.Context(), accepted, landed, current, verifier, effect("routing"), func(context.Context) error { return stop }, nil, nil, intent); !errors.Is(err, stop) || !strings.Contains(err.Error(), "release ownership") {
		t.Fatalf("ownership error=%v", err)
	}
	if err := DeliveryTransition(t.Context(), accepted, landed, current, verifier, effect("routing"), effect("ownership"), retentionPolicyFunc(func(context.Context, CleanupIntent) (bool, error) { return false, stop }), func(context.Context, CleanupIntent) error { return nil }, intent); !errors.Is(err, stop) {
		t.Fatalf("policy error=%v", err)
	}
	if err := DeliveryTransition(t.Context(), accepted, landed, current, verifier, effect("routing"), effect("ownership"), retentionPolicyFunc(func(context.Context, CleanupIntent) (bool, error) { return false, nil }), func(context.Context, CleanupIntent) error { return stop }, intent); err != nil {
		t.Fatalf("unauthorized cleanup ran: %v", err)
	}
}

func TestTransitionSignaturesFailureVocabularyAndStatusString(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	transition := Transition{Kind: "advance", PriorVersion: 1, NewVersion: 2, PriorEpoch: 1, NewEpoch: 1, Actor: "worker"}
	signed := SignTransition(transition, private)
	if signed.Signature == "" || !VerifyTransition(signed, public) {
		t.Fatal("valid transition signature was rejected")
	}
	signed.Actor = "other"
	if VerifyTransition(signed, public) {
		t.Fatal("mutated transition signature was accepted")
	}
	signed.Signature = "not-hex"
	if VerifyTransition(signed, public) {
		t.Fatal("malformed transition signature was accepted")
	}

	evidence := []EvidenceRef{{Kind: "proof", ID: "e-1", Digest: "digest"}}
	for _, trigger := range []string{"landing-unverified", "candidate-conflict", "verification-unaccepted", "candidate-stale", "delivery-unverified", "owner-unavailable", "ownership-invalid", "cancel-active", "handoff-not-quiescent", "invalid-feature"} {
		got, err := ClassifyFailure(trigger, evidence)
		if err != nil || got.Trigger != trigger || got.Remedy == "" || got.ClearCondition == "" || got.BlockedEffect == "" {
			t.Fatalf("ClassifyFailure(%q) = %#v, %v", trigger, got, err)
		}
	}
	if _, err := ClassifyFailure("unknown", evidence); err == nil {
		t.Fatal("unknown failure was classified")
	}
	if _, err := ClassifyFailure("candidate-stale", nil); err == nil {
		t.Fatal("failure without evidence was classified")
	}
	status := FeatureDriveStatus{FeatureID: "f-1", Phase: Blocked, Owner: Ownership{Domain: "local", OwnerID: "machine", Epoch: 2}, PendingEffects: []EffectIntent{{ID: "e"}}}
	if got := status.String(); !strings.Contains(got, "f-1") || !strings.Contains(got, "pending=1") {
		t.Fatalf("status string=%q", got)
	}
}

func TestOwnerBackstopRunRoutesDueTouchesAndIgnoresStaleOnes(t *testing.T) {
	store := NewMemoryStore()
	drive := fixture()
	drive.Wait = &Wait{Kind: "review", Goal: EvidenceRef{Kind: "review", ID: "r1"}}
	if err := store.Create(t.Context(), drive); err != nil {
		t.Fatal(err)
	}
	var calls int
	backstop := OwnerBackstop{
		Store: store, Domain: drive.Ownership.Domain, OwnerID: drive.Ownership.OwnerID,
		Now: func() time.Time { return time.Unix(20, 0) },
	}
	backstop.Router = TouchRouter{Store: store, Reconcile: func(context.Context, string, string) error { calls++; return nil }}
	if err := backstop.Run(t.Context()); err != nil || calls != 1 {
		t.Fatalf("Run calls=%d err=%v", calls, err)
	}
	backstop.Router.Reconcile = func(context.Context, string, string) error { return errors.New("reconcile failed") }
	if err := backstop.Run(t.Context()); err == nil {
		t.Fatal("backstop swallowed a reconciliation failure")
	}
	backstop.Store = nil
	if err := backstop.Run(t.Context()); err == nil {
		t.Fatal("backstop accepted an incomplete scanner")
	}
}
