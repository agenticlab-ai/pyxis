package routingevidence

import (
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/nativerouting"
)

func signingCapability(seed string, event Event, class ProducerClass, generation string, kinds []EventKind, roles []string) SigningCapability {
	digest := sha256.Sum256([]byte(seed))
	return SigningCapability{
		Repository: event.Repository, EvidenceRef: event.EvidenceRef,
		ProducerClass: class, ImplementationGeneration: generation,
		KeyID: seed + "-key", PrivateKey: ed25519.NewKeyFromSeed(digest[:]),
		EventKinds: kinds, Roles: roles,
	}
}

func TestSignedReviewWriterEnforcesShapeAndCapabilityBoundaries(t *testing.T) {
	_, fx := reducerFixture(t)
	review := fx.review(true)
	pass, finding, terminal := review[3], review[4], review[len(review)-1]
	disposition := cloneEvent(t, fixtureEvents(false)[kindIndex(DispositionRecorded)])

	orchestrator := signingCapability("review-writer", pass, ProducerReviewOrchestrator, "review-writer/v1",
		[]EventKind{AssurancePassCompleted, FindingPersisted, ReviewProcessCompleted}, []string{""})
	medium := signingCapability("medium-writer", disposition, ProducerMediumAdapter, "medium-writer/v1",
		[]EventKind{DispositionRecorded}, []string{"worker", "operator"})

	if _, err := (SignedReviewEventWriter{}).WriteReviewEvents(t.Context(), ObservedHead{}, []Event{pass}); err == nil {
		t.Fatal("writer without a store was accepted")
	}
	store := &appendCaptureStore{}
	writer := SignedReviewEventWriter{Store: store, OrchestratorCapability: orchestrator, MediumCapability: medium}
	for name, events := range map[string][]Event{
		"empty":               nil,
		"single finding":      {finding},
		"composite no parent": {finding, finding},
		"two parents":         {pass, pass},
		"mixed composite":     {pass, terminal},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := writer.WriteReviewEvents(t.Context(), ObservedHead{}, events); err == nil {
				t.Fatal("invalid review append shape was accepted")
			}
		})
	}

	head := ObservedHead{EvidenceRef: pass.EvidenceRef, Commit: "head-before-review"}
	if _, err := writer.WriteReviewEvents(t.Context(), head, []Event{pass, finding}); err != nil {
		t.Fatalf("composite append: %v", err)
	}
	if store.request.Composite == nil || store.request.Event != nil || len(store.request.Composite.Events) != 2 {
		t.Fatalf("review composite = %#v", store.request)
	}
	for _, event := range store.request.Composite.Events {
		if event.Producer.Class != string(ProducerReviewOrchestrator) || event.ProducerProof.Signature == "" {
			t.Fatalf("orchestrator signature missing: %#v", event)
		}
	}

	if _, err := writer.WriteReviewEvents(t.Context(), head, []Event{disposition}); err != nil {
		t.Fatalf("disposition append: %v", err)
	}
	if store.request.Event == nil || store.request.Composite != nil || store.request.Event.Producer.Class != string(ProducerMediumAdapter) {
		t.Fatalf("disposition was not a medium-signed single append: %#v", store.request)
	}
	if _, err := (SignedReviewEventWriter{Store: store}).WriteReviewEvents(t.Context(), head, []Event{terminal}); err == nil {
		t.Fatal("writer without a signing key was accepted")
	}
}

func TestSignedInvalidationAndResolutionWritersSignOneCanonicalEvent(t *testing.T) {
	store := &appendCaptureStore{}
	invalidation := cloneEvent(t, fixtureEvents(false)[kindIndex(RoutingEvidenceInvalidated)])
	invalidator := SignedInvalidationWriter{Store: store, Capability: signingCapability(
		"invalidation-writer", invalidation, ProducerEvidenceOwner, "invalidation-writer/v1",
		[]EventKind{RoutingEvidenceInvalidated}, []string{"rerun_required", "operator_remediation"},
	)}
	if _, err := invalidator.WriteInvalidation(t.Context(), ObservedHead{EvidenceRef: invalidation.EvidenceRef}, invalidation); err != nil {
		t.Fatalf("invalidation append: %v", err)
	}
	if store.request.Event == nil || store.request.Event.Producer.Class != string(ProducerEvidenceOwner) || store.request.Event.ProducerProof.Signature == "" {
		t.Fatalf("invalidation was not signed: %#v", store.request)
	}
	if _, err := (SignedInvalidationWriter{}).WriteInvalidation(t.Context(), ObservedHead{}, invalidation); err == nil {
		t.Fatal("invalidation writer without a store was accepted")
	}
	if _, err := (SignedInvalidationWriter{Store: store}).WriteInvalidation(t.Context(), ObservedHead{}, invalidation); err == nil {
		t.Fatal("invalidation writer without a capability was accepted")
	}

	_, admissionStore, intent := testAdmission(t)
	intent.Role = nativerouting.RoleImplementation
	resolution := (nativerouting.Resolver{}).Resolve(nativerouting.Request{
		SchemaVersion:    nativerouting.RequestSchema,
		AttemptID:        intent.AttemptID,
		DemandID:         intent.DemandID,
		Harness:          intent.Harness,
		Role:             nativerouting.RoleImplementation,
		RequestedProfile: intent.RequestedProfile,
		Mode:             intent.Mode,
	}, intent.Catalog)
	terminal := terminalFor(intent, "routing-policy/v1", resolution)
	terminal.Repository, terminal.EvidenceRef = admissionStore.origin.Repository, admissionStore.origin.EvidenceRef
	resolver := SignedResolutionTerminalWriter{Store: store, Capability: signingCapability(
		"resolution-writer", terminal, ProducerResolver, "resolution-writer/v1",
		[]EventKind{RoutingResolved, RoutingResolutionFailed}, []string{"implementation", "review"},
	)}
	if _, err := resolver.WriteTerminal(t.Context(), ObservedHead{EvidenceRef: terminal.EvidenceRef}, terminal); err != nil {
		t.Fatalf("resolution append: %v", err)
	}
	if store.request.Event == nil || store.request.Event.Producer.Class != string(ProducerResolver) || store.request.Event.ProducerProof.Signature == "" {
		t.Fatalf("resolution was not signed: %#v", store.request)
	}
	if _, err := (SignedResolutionTerminalWriter{}).WriteTerminal(t.Context(), ObservedHead{}, terminal); err == nil {
		t.Fatal("resolution writer without a store was accepted")
	}
	if _, err := (SignedResolutionTerminalWriter{Store: store}).WriteTerminal(t.Context(), ObservedHead{}, terminal); err == nil {
		t.Fatal("resolution writer without a capability was accepted")
	}
}

func TestAdmissionPermitProjectsOnlyVerifiedBindings(t *testing.T) {
	var zero AdmissionPermit
	if err := zero.Authorize(OperationClaim, BoundaryBinding{}); err == nil {
		t.Fatal("zero permit authorized a claim")
	}
	if _, err := zero.NativeLaunch(nativerouting.RoleImplementation); err == nil {
		t.Fatal("zero permit returned a native launch")
	}
	if _, err := zero.RuntimeOwnershipProof(); err == nil {
		t.Fatal("zero permit returned an ownership proof")
	}

	verifier, _, intent := testAdmission(t)
	permit, err := verifier.AdmitImplementation(t.Context(), intent)
	if err != nil {
		t.Fatal(err)
	}
	if permit.AttemptID() != intent.AttemptID || permit.NativeProfileID() != "codex-strong" || permit.PolicyVersion() != "routing-policy/v1" || permit.OwnerGeneration() != 1 {
		t.Fatalf("permit projections lost their verified bindings: %#v", permit)
	}
	if proof := permit.EvidenceProof(); len(proof.Entries) != 1 {
		t.Fatalf("evidence proof = %#v", proof)
	}
	launch, err := permit.NativeLaunch(nativerouting.RoleImplementation)
	if err != nil || launch.NativeModel != "gpt-5.6" || launch.NativeProfileID != "codex-strong" {
		t.Fatalf("native launch = %#v, %v", launch, err)
	}
	if _, err := permit.NativeLaunch(nativerouting.RoleReview); err == nil {
		t.Fatal("permit crossed its verified role")
	}
	ownership, err := permit.RuntimeOwnershipProof()
	if err != nil || ownership.AttemptID != intent.AttemptID || ownership.Reference == "" || ownership.Digest == "" {
		t.Fatalf("ownership proof = %#v, %v", ownership, err)
	}
	if err := permit.Authorize(OperationClaim, BoundaryBinding{Role: nativerouting.RoleImplementation}); err == nil {
		t.Fatal("permit authorized a mismatched boundary")
	}
}

func TestStoreAndTypedRoutingErrorsPreserveTheirSentinels(t *testing.T) {
	storeErr := NewStoreError(StoreLeaseInvalid, "head %s moved", "old")
	if !errors.Is(storeErr, ErrStoreIntegrity) || storeErr.Error() == "" {
		t.Fatalf("store error contract = %v", storeErr)
	}
	activation := activationReject(ActivationEvidenceStale, "head moved")
	if activation.Error() == "" || !errors.Is(activation, ErrActivationRejected) {
		t.Fatalf("activation error contract = %v", activation)
	}
	reducer := reducerError(ReducerConflict, "event", "event-1", "duplicate")
	if reducer.Error() == "" || !errors.Is(reducer, ErrReducerIntegrity) {
		t.Fatalf("reducer error contract = %v", reducer)
	}
}
