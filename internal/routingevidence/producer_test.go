package routingevidence

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
)

func TestProducerPolicySignsAndVerifiesEveryEventKind(t *testing.T) {
	policy, privateKeys := producerPolicyFixture(t)
	encoded, err := EncodeProducerPolicy(policy)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeProducerPolicy(encoded)
	if err != nil || decoded.Digest != policy.Digest {
		t.Fatalf("policy round trip: digest=%q err=%v", decoded.Digest, err)
	}
	unknown := append(encoded[:len(encoded)-1], []byte(`,"unknown":true}`)...)
	if _, err := DecodeProducerPolicy(unknown); err == nil {
		t.Fatal("closed policy decoder admitted an unknown key")
	}

	for _, event := range fixtureEvents(false) {
		event := authenticateFixture(t, event, policy, privateKeys)
		t.Run(string(event.Kind), func(t *testing.T) {
			if err := VerifyProducer(event, policy); err != nil {
				t.Fatalf("append verification: %v", err)
			}
			wire, err := EncodeCanonical(event)
			if err != nil {
				t.Fatal(err)
			}
			if err := VerifyProducerCanonical(wire, policy); err != nil {
				t.Fatalf("fresh verification: %v", err)
			}
			unsigned, _ := NewUnsignedEnvelope(event)
			proof2, err := Sign(unsigned, capabilityFor(event, policy, privateKeys))
			if err != nil || proof2 != event.ProducerProof {
				t.Fatalf("Ed25519 signing is not deterministic: proof=%+v err=%v", proof2, err)
			}
		})
	}
}

func TestProducerAbuseFailsTheSameAtAppendAndFreshRead(t *testing.T) {
	policy, privateKeys := producerPolicyFixture(t)
	original := authenticateFixture(t, fixtureEvents(false)[kindIndex(RoutingResolved)], policy, privateKeys)
	cases := map[string]Event{
		"unsigned":         cloneEvent(t, original),
		"forged":           cloneEvent(t, original),
		"altered":          cloneEvent(t, original),
		"stale generation": cloneEvent(t, original),
		"cross repository": cloneEvent(t, original),
		"cross ref":        cloneEvent(t, original),
		"generic writer":   cloneEvent(t, original),
		"cross kind":       authenticateFixture(t, fixtureEvents(false)[kindIndex(RoutingResolutionFailed)], policy, privateKeys),
	}
	unsigned := cases["unsigned"]
	unsigned.ProducerProof = ProducerProof{}
	cases["unsigned"] = unsigned
	forged := cases["forged"]
	forged.ProducerProof.Signature = strings.Repeat("00", ed25519.SignatureSize)
	cases["forged"] = forged
	altered := cases["altered"]
	altered.Payload.RoutingResolved.Harness = "opencode"
	cases["altered"] = altered
	stale := cases["stale generation"]
	stale.Producer.Generation = "resolver/v0"
	cases["stale generation"] = stale
	crossRepository := cases["cross repository"]
	crossRepository.Repository = "other/repository"
	cases["cross repository"] = crossRepository
	crossRef := cases["cross ref"]
	crossRef.EvidenceRef = "refs/heads/main"
	cases["cross ref"] = crossRef
	generic := cases["generic writer"]
	generic.Producer.Class = "generic-ref-writer"
	cases["generic writer"] = generic
	crossKind := cases["cross kind"]
	crossKind.ProducerProof = original.ProducerProof
	cases["cross kind"] = crossKind

	for name, event := range cases {
		t.Run(name, func(t *testing.T) {
			appendErr := VerifyProducer(event, policy)
			freshErr := VerifyProducerCanonical(unverifiedWire(t, event), policy)
			if appendErr == nil || freshErr == nil || !errors.Is(appendErr, ErrProducerIntegrity) || !errors.Is(freshErr, ErrProducerIntegrity) {
				t.Fatalf("append=%v fresh=%v", appendErr, freshErr)
			}
			if integrityErrorCode(t, appendErr) != integrityErrorCode(t, freshErr) {
				t.Fatalf("failure class differs: append=%v fresh=%v", appendErr, freshErr)
			}
		})
	}
}

func TestAuthorizationReferencesAndLeastPrivilege(t *testing.T) {
	policy, privateKeys := producerPolicyFixture(t)
	for _, kind := range []EventKind{AttemptOwnerTransferred, DispositionRecorded, RoutingEvidenceInvalidated, ActivationCheckpointEvent} {
		event := fixtureEvents(false)[kindIndex(kind)]
		event = authenticateFixture(t, event, policy, privateKeys)
		switch kind {
		case AttemptOwnerTransferred:
			event.Payload.AttemptOwnerTransferred.RuntimeFenceProofRef = "fence:foreign"
		case DispositionRecorded:
			event.Payload.DispositionRecorded.NativeAuthorIdentityProof = "author:forged"
		case RoutingEvidenceInvalidated:
			event.Payload.RoutingEvidenceInvalidated.AuthorizationRef = "invalidation:foreign"
		case ActivationCheckpointEvent:
			event.Payload.ActivationCheckpoint.OperatorAuthorizationID = "auth-foreign"
		}
		event = resignFixture(t, event, policy, privateKeys)
		if code := integrityErrorCode(t, VerifyProducer(event, policy)); code != IntegrityUnauthorized {
			t.Errorf("%s authorization failure code = %s", kind, code)
		}
	}

	// An ordinary outcome key cannot authorize invalidation even with a valid
	// signature over the exact invalidation envelope.
	overprivileged := fixtureEvents(false)[kindIndex(RoutingEvidenceInvalidated)]
	overprivileged.Producer = Producer{Class: string(ProducerAttemptOwner), Generation: generationFor(ProducerAttemptOwner)}
	unsigned, err := NewUnsignedEnvelope(overprivileged)
	if err != nil {
		t.Fatal(err)
	}
	ownerKey := privateKeys[ProducerAttemptOwner]
	sig := ed25519.Sign(ownerKey, proofMessage(mustCanonicalUnsigned(t, unsigned)))
	overprivileged.ProducerProof = ProducerProof{Algorithm: "ed25519", KeyID: keyIDFor(ProducerAttemptOwner), Schema: ProducerProofSchema, Signature: hex.EncodeToString(sig)}
	if code := integrityErrorCode(t, VerifyProducer(overprivileged, policy)); code != IntegrityUnauthorized {
		t.Fatalf("ordinary outcome key invalidation code = %s", code)
	}
	if _, err := Sign(unsigned, SigningCapability{Repository: overprivileged.Repository, EvidenceRef: overprivileged.EvidenceRef, ProducerClass: ProducerAttemptOwner, ImplementationGeneration: overprivileged.Producer.Generation, KeyID: keyIDFor(ProducerAttemptOwner), PrivateKey: ownerKey, EventKinds: []EventKind{RoutingEvidenceInvalidated}, Roles: []string{"rerun_required"}}); integrityErrorCode(t, err) != IntegrityUnauthorized {
		t.Fatalf("overprivileged signing capability was admitted: %v", err)
	}
}

func TestOperatorDispositionRequiresConfiguredIdentity(t *testing.T) {
	policy, privateKeys := producerPolicyFixture(t)
	policy.Authorizations = append(policy.Authorizations, AuthorizationReference{ID: "auth-operator", Kind: AuthorizationNativeAuthor, Reference: "operator-proof-1", ActorClass: "operator", ActorIdentity: "operator:alice"})
	policy.Matrix = append(policy.Matrix, ProducerAuthorization{AuthorizationID: "auth-operator", EventKind: DispositionRecorded, ImplementationGeneration: generationFor(ProducerMediumAdapter), KeyID: keyIDFor(ProducerMediumAdapter), ProducerClass: ProducerMediumAdapter, Role: "operator"})
	setPolicyDigest(t, &policy)

	event := fixtureEvents(false)[kindIndex(DispositionRecorded)]
	event.Payload.DispositionRecorded.Actor = "operator"
	event.Payload.DispositionRecorded.ActorAttribution = "operator:alice"
	event.Payload.DispositionRecorded.NativeAuthorIdentityProof = "operator-proof-1"
	event.Payload.DispositionRecorded.Verdict = "accepted"
	event = authenticateFixture(t, event, policy, privateKeys)
	if err := VerifyProducer(event, policy); err != nil {
		t.Fatal(err)
	}
	event.Payload.DispositionRecorded.ActorAttribution = "operator:mallory"
	event = resignFixture(t, event, policy, privateKeys)
	if code := integrityErrorCode(t, VerifyProducer(event, policy)); code != IntegrityUnauthorized {
		t.Fatalf("forged operator code = %s", code)
	}
}

func TestConfiguredReaperAuthorityIsLimitedToOutcomes(t *testing.T) {
	policy, privateKeys := producerPolicyFixture(t)
	seed := sha256.Sum256([]byte("producer-key:" + ProducerAttemptReaper))
	reaperKey := ed25519.NewKeyFromSeed(seed[:])
	privateKeys[ProducerAttemptReaper] = reaperKey
	policy.Keys = append(policy.Keys, KeyGeneration{Algorithm: "ed25519", ImplementationGeneration: generationFor(ProducerAttemptReaper), KeyID: keyIDFor(ProducerAttemptReaper), PolicyActivationPoints: []string{policy.ActivationPoint}, ProducerClass: ProducerAttemptReaper, PublicKey: hex.EncodeToString(reaperKey.Public().(ed25519.PublicKey))})
	for _, kind := range []EventKind{RoutedAttemptCompleted, RoutedTaskOutcome} {
		policy.Matrix = append(policy.Matrix, ProducerAuthorization{AuthorizationID: "auth-policy", EventKind: kind, ImplementationGeneration: generationFor(ProducerAttemptReaper), KeyID: keyIDFor(ProducerAttemptReaper), ProducerClass: ProducerAttemptReaper})
	}
	setPolicyDigest(t, &policy)
	event := fixtureEvents(false)[kindIndex(RoutedAttemptCompleted)]
	event.Producer = Producer{Class: string(ProducerAttemptReaper), Generation: generationFor(ProducerAttemptReaper)}
	event = resignFixture(t, event, policy, privateKeys)
	if err := VerifyProducer(event, policy); err != nil {
		t.Fatalf("configured reaper outcome: %v", err)
	}

	route := fixtureEvents(false)[kindIndex(RoutingResolved)]
	route.Producer = event.Producer
	unsigned, _ := NewUnsignedEnvelope(route)
	_, err := Sign(unsigned, SigningCapability{Repository: route.Repository, EvidenceRef: route.EvidenceRef, ProducerClass: ProducerAttemptReaper, ImplementationGeneration: route.Producer.Generation, KeyID: keyIDFor(ProducerAttemptReaper), PrivateKey: reaperKey, EventKinds: []EventKind{RoutingResolved}, Roles: []string{"implementation"}})
	if code := integrityErrorCode(t, err); code != IntegrityUnauthorized {
		t.Fatalf("reaper routing authority code = %s", code)
	}
}

func TestKeyRotationIsExactAndDoesNotExpandOldAuthority(t *testing.T) {
	oldPolicy, privateKeys := producerPolicyFixture(t)
	oldEvent := authenticateFixture(t, fixtureEvents(false)[kindIndex(RoutingResolved)], oldPolicy, privateKeys)
	newPolicy := clonePolicy(t, oldPolicy)
	newPolicy.ActivationPoint = "policy-point/2"
	for i := range newPolicy.Keys {
		if newPolicy.Keys[i].ProducerClass == ProducerResolver {
			newPolicy.Keys[i].RevocationReference = "operator-revocation/2"
		} else {
			newPolicy.Keys[i].PolicyActivationPoints = append(newPolicy.Keys[i].PolicyActivationPoints, "policy-point/2")
		}
	}
	seed := sha256.Sum256([]byte("resolver-key-v2"))
	newPrivate := ed25519.NewKeyFromSeed(seed[:])
	newPolicy.Keys = append(newPolicy.Keys, KeyGeneration{Algorithm: "ed25519", ImplementationGeneration: "resolver/v2", KeyID: "key-resolver-v2", PolicyActivationPoints: []string{"policy-point/2"}, ProducerClass: ProducerResolver, PublicKey: hex.EncodeToString(newPrivate.Public().(ed25519.PublicKey))})
	for i := range newPolicy.Matrix {
		if newPolicy.Matrix[i].ProducerClass == ProducerResolver {
			newPolicy.Matrix[i].ImplementationGeneration = "resolver/v2"
			newPolicy.Matrix[i].KeyID = "key-resolver-v2"
		}
	}
	setPolicyDigest(t, &newPolicy)

	newEvent := fixtureEvents(false)[kindIndex(RoutingResolved)]
	newEvent.Producer = Producer{Class: string(ProducerResolver), Generation: "resolver/v2"}
	unsigned, _ := NewUnsignedEnvelope(newEvent)
	proof, err := Sign(unsigned, SigningCapability{Repository: newEvent.Repository, EvidenceRef: newEvent.EvidenceRef, ProducerClass: ProducerResolver, ImplementationGeneration: "resolver/v2", KeyID: "key-resolver-v2", PrivateKey: newPrivate, EventKinds: []EventKind{RoutingResolved}, Roles: []string{"implementation"}})
	if err != nil {
		t.Fatal(err)
	}
	newEvent.ProducerProof = proof
	if err := VerifyProducer(oldEvent, oldPolicy); err != nil {
		t.Fatalf("old event before activation: %v", err)
	}
	if err := VerifyProducer(newEvent, newPolicy); err != nil {
		t.Fatalf("new event at activation: %v", err)
	}
	if VerifyProducer(newEvent, oldPolicy) == nil || VerifyProducer(oldEvent, newPolicy) == nil {
		t.Fatal("rotation crossed its exact activation point")
	}
}

func TestGenesisContinuityRejectsEveryDestructiveShape(t *testing.T) {
	checkpoint := GenesisCheckpoint{AuthorizationReference: "operator:init-1", Commit: "commit-genesis", EvidenceRef: "refs/pyxis/routing-evidence", Repository: "agenticlab-ai/pyxis", Tree: "tree-genesis"}
	valid := RefView{Repository: checkpoint.Repository, EvidenceRef: checkpoint.EvidenceRef, Exists: true, Head: "commit-two", InitializationAuthorization: checkpoint.AuthorizationReference, Commits: []RefCommit{{Object: "commit-genesis", Tree: "tree-genesis", Parents: []string{}}, {Object: "commit-one", Tree: "tree-one", Parents: []string{"commit-genesis"}}, {Object: "commit-two", Tree: "tree-two", Parents: []string{"commit-one"}}}}
	if err := VerifyContinuity(valid, checkpoint); err != nil {
		t.Fatal(err)
	}
	if err := VerifyContinuity(valid, GenesisCheckpoint{}); err == nil {
		t.Fatal("incomplete checkpoint was accepted")
	}
	wrongRepository := cloneRefView(valid)
	wrongRepository.Repository = "other/repo"
	if err := VerifyContinuity(wrongRepository, checkpoint); err == nil {
		t.Fatal("continuity crossed repositories")
	}
	cases := map[string]RefView{
		"missing":      cloneRefView(valid),
		"deleted":      cloneRefView(valid),
		"changed":      cloneRefView(valid),
		"rewritten":    cloneRefView(valid),
		"force update": cloneRefView(valid),
		"unauthorized": cloneRefView(valid),
	}
	missing := cases["missing"]
	missing.Exists = false
	cases["missing"] = missing
	deleted := cases["deleted"]
	deleted.DeletionObserved = true
	cases["deleted"] = deleted
	changed := cases["changed"]
	changed.Commits[0].Tree = "tree-changed"
	cases["changed"] = changed
	rewritten := cases["rewritten"]
	rewritten.Head = "commit-rewrite"
	rewritten.Commits = append(rewritten.Commits, RefCommit{Object: "commit-rewrite", Tree: "tree-rewrite", Parents: []string{}})
	cases["rewritten"] = rewritten
	forced := cases["force update"]
	forced.NonFastForwardObserved = true
	cases["force update"] = forced
	unauthorized := cases["unauthorized"]
	unauthorized.InitializationAuthorization = "ordinary-writer"
	cases["unauthorized"] = unauthorized
	for name, view := range cases {
		t.Run(name, func(t *testing.T) {
			err := VerifyContinuity(view, checkpoint)
			if !errors.Is(err, ErrProducerIntegrity) || integrityErrorCode(t, err) != IntegrityContinuity {
				t.Fatalf("continuity error = %v", err)
			}
		})
	}
}

func producerPolicyFixture(t *testing.T) (ProducerPolicy, map[ProducerClass]ed25519.PrivateKey) {
	t.Helper()
	operatorSeed := sha256.Sum256([]byte("activation-operator"))
	operatorKey := ed25519.NewKeyFromSeed(operatorSeed[:])
	policy := ProducerPolicy{ActivationPoint: "policy-point/1", EvidenceRef: "refs/pyxis/routing-evidence", Repository: "agenticlab-ai/pyxis", Schema: ProducerPolicySchema, Version: "policy/v1"}
	policy.Genesis = GenesisCheckpoint{AuthorizationReference: "operator:init-1", Commit: "commit-genesis", EvidenceRef: policy.EvidenceRef, Repository: policy.Repository, Tree: "tree-genesis"}
	policy.Authorizations = []AuthorizationReference{
		{ID: "auth-policy", Kind: AuthorizationPolicy},
		{ID: "auth-fence", Kind: AuthorizationRuntimeFence, Reference: "fence-proof-7"},
		{ID: "auth-native", Kind: AuthorizationNativeAuthor, Reference: "author-proof-7", ActorClass: "worker", ActorIdentity: "github:user-7"},
		{ID: "auth-invalidation", Kind: AuthorizationInvalidation, Reference: "operator-decision-7"},
		{ID: "auth-activation", Kind: AuthorizationActivationManifest, ActorClass: "operator", ActorIdentity: "operator-key-1", PublicKey: hex.EncodeToString(operatorKey.Public().(ed25519.PublicKey))},
	}
	privateKeys := map[ProducerClass]ed25519.PrivateKey{}
	seen := map[ProducerClass]bool{}
	for _, event := range fixtureEvents(false) {
		class := classFor(event.Kind)
		if !seen[class] {
			seed := sha256.Sum256([]byte("producer-key:" + class))
			private := ed25519.NewKeyFromSeed(seed[:])
			privateKeys[class] = private
			policy.Keys = append(policy.Keys, KeyGeneration{Algorithm: "ed25519", ImplementationGeneration: generationFor(class), KeyID: keyIDFor(class), PolicyActivationPoints: []string{policy.ActivationPoint}, ProducerClass: class, PublicKey: hex.EncodeToString(private.Public().(ed25519.PublicKey))})
			seen[class] = true
		}
		policy.Matrix = append(policy.Matrix, ProducerAuthorization{AuthorizationID: authorizationID(event.Kind), EventKind: event.Kind, ImplementationGeneration: generationFor(class), KeyID: keyIDFor(class), ProducerClass: class, Role: eventRole(event)})
	}
	setPolicyDigest(t, &policy)
	if err := ValidateProducerPolicy(policy); err != nil {
		t.Fatal(err)
	}
	return policy, privateKeys
}

func classFor(kind EventKind) ProducerClass {
	switch kind {
	case ExecutionDemandFrozen:
		return ProducerPlanGate

	case RoutingResolved, RoutingResolutionFailed:
		return ProducerResolver
	case RoutedAttemptCompleted, RoutedTaskOutcome:
		return ProducerAttemptOwner
	case AttemptOwnerTransferred:
		return ProducerRecoveryCoordinator
	case AssurancePassCompleted, FindingPersisted, ReviewProcessCompleted:
		return ProducerReviewOrchestrator
	case DispositionRecorded:
		return ProducerMediumAdapter
	case RoutingEvidenceInvalidated:
		return ProducerEvidenceOwner
	case ActivationCheckpointEvent:
		return ProducerReleaseGate
	default:
		panic(kind)
	}
}

func authorizationID(kind EventKind) string {
	switch requiredAuthorization(kind) {
	case AuthorizationRuntimeFence:
		return "auth-fence"
	case AuthorizationNativeAuthor:
		return "auth-native"
	case AuthorizationInvalidation:
		return "auth-invalidation"
	case AuthorizationActivationManifest:
		return "auth-activation"
	default:
		return "auth-policy"
	}
}

func generationFor(class ProducerClass) string { return string(class) + "/v1" }
func keyIDFor(class ProducerClass) string      { return "key-" + string(class) }

func authenticateFixture(t *testing.T, event Event, policy ProducerPolicy, privateKeys map[ProducerClass]ed25519.PrivateKey) Event {
	t.Helper()
	class := classFor(event.Kind)
	event.Producer = Producer{Class: string(class), Generation: generationFor(class)}
	return resignFixture(t, event, policy, privateKeys)
}

func resignFixture(t *testing.T, event Event, policy ProducerPolicy, privateKeys map[ProducerClass]ed25519.PrivateKey) Event {
	t.Helper()
	event.EventID, event.ContentDigest, event.ProducerProof = "", "", ProducerProof{}
	unsigned, err := NewUnsignedEnvelope(event)
	if err != nil {
		t.Fatal(err)
	}
	proof, err := Sign(unsigned, capabilityFor(event, policy, privateKeys))
	if err != nil {
		t.Fatal(err)
	}
	event.ProducerProof = proof
	return event
}

func capabilityFor(event Event, policy ProducerPolicy, privateKeys map[ProducerClass]ed25519.PrivateKey) SigningCapability {
	class := ProducerClass(event.Producer.Class)
	roles := []string(nil)
	if role := eventRole(event); role != "" {
		roles = []string{role}
	}
	keyID := keyIDFor(class)
	for _, row := range policy.Matrix {
		if row.EventKind == event.Kind && row.ProducerClass == class && row.ImplementationGeneration == event.Producer.Generation && row.Role == eventRole(event) {
			keyID = row.KeyID
		}
	}
	return SigningCapability{Repository: event.Repository, EvidenceRef: event.EvidenceRef, ProducerClass: class, ImplementationGeneration: event.Producer.Generation, KeyID: keyID, PrivateKey: privateKeys[class], EventKinds: []EventKind{event.Kind}, Roles: roles}
}

func unverifiedWire(t *testing.T, event Event) []byte {
	t.Helper()
	u, err := NewUnsignedEnvelope(event)
	if err != nil {
		t.Fatal(err)
	}
	b, err := canonicalJSON(Envelope{ContentDigest: u.ContentDigest, Domain: u.Domain, EventID: u.EventID, EvidenceRef: u.EvidenceRef, EventKind: u.EventKind, EventVersion: u.EventVersion, Payload: u.Payload, Producer: u.Producer, ProducerProof: event.ProducerProof, Repository: u.Repository, Schema: u.Schema})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func mustCanonicalUnsigned(t *testing.T, unsigned UnsignedEnvelope) []byte {
	t.Helper()
	b, err := validateUnsigned(unsigned)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func setPolicyDigest(t *testing.T, policy *ProducerPolicy) {
	t.Helper()
	policy.Digest = ""
	digest, err := DigestProducerPolicy(*policy)
	if err != nil {
		t.Fatal(err)
	}
	policy.Digest = digest
}

func clonePolicy(t *testing.T, policy ProducerPolicy) ProducerPolicy {
	t.Helper()
	b, err := EncodeProducerPolicy(policy)
	if err != nil {
		t.Fatal(err)
	}
	clone, err := DecodeProducerPolicy(b)
	if err != nil {
		t.Fatal(err)
	}
	return clone
}

func cloneEvent(t *testing.T, event Event) Event {
	t.Helper()
	b, err := EncodeCanonical(event)
	if err != nil {
		t.Fatal(err)
	}
	clone, err := DecodeCanonical(b)
	if err != nil {
		t.Fatal(err)
	}
	clone.EventID, clone.ContentDigest = "", ""
	return clone
}

func cloneRefView(view RefView) RefView {
	clone := view
	clone.Commits = append([]RefCommit(nil), view.Commits...)
	for i := range clone.Commits {
		clone.Commits[i].Parents = append([]string(nil), view.Commits[i].Parents...)
	}
	return clone
}

func integrityErrorCode(t *testing.T, err error) IntegrityCode {
	t.Helper()
	var typed *IntegrityError
	if !errors.As(err, &typed) {
		t.Fatalf("error is not an IntegrityError: %v", err)
	}
	return typed.Code
}
