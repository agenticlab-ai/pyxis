package routingevidence

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"fmt"
	"reflect"
	"testing"
)

func TestVerifyActivationBindsSignedManifestExactMainlineAndCurrentEvidence(t *testing.T) {
	state, manifest, mainline, _ := activationFixture(t, false)
	checkpoint, err := VerifyActivation(state, manifest, mainline)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := DigestActivationManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if !checkpoint.Payload.ActivationReady || checkpoint.Payload.Activated || checkpoint.Payload.ProofManifestDigest != digest ||
		checkpoint.Payload.EvidenceHead != state.EvidenceHead || checkpoint.Payload.ExactMainlineObject != manifest.ExactMainlineObject ||
		checkpoint.Payload.OperatorAuthorizationID != manifest.OperatorAuthorizationID || checkpoint.ObservedHead.Commit != state.EvidenceHead {
		t.Fatalf("checkpoint = %+v", checkpoint)
	}
	wantIDs := make([]string, len(manifest.Proofs))
	for i := range manifest.Proofs {
		wantIDs[i] = manifest.Proofs[i].ID
	}
	if !reflect.DeepEqual(checkpoint.Payload.ProofIDs, wantIDs) {
		t.Fatalf("proof ids = %v, want %v", checkpoint.Payload.ProofIDs, wantIDs)
	}
}

func TestActivationManifestAndMainlineProofAreClosedCanonicalContracts(t *testing.T) {
	_, manifest, mainline, _ := activationFixture(t, false)
	manifestWire, err := EncodeActivationManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	decodedManifest, err := DecodeActivationManifest(manifestWire)
	if err != nil || !reflect.DeepEqual(decodedManifest, manifest) {
		t.Fatalf("manifest round trip = %+v, %v", decodedManifest, err)
	}
	mainlineWire, err := EncodeMainlineProof(mainline)
	if err != nil {
		t.Fatal(err)
	}
	decodedMainline, err := DecodeMainlineProof(mainlineWire)
	if err != nil || !reflect.DeepEqual(decodedMainline, mainline) {
		t.Fatalf("mainline round trip = %+v, %v", decodedMainline, err)
	}
	unknown := append([]byte(nil), manifestWire[:len(manifestWire)-1]...)
	unknown = append(unknown, []byte(`,"rollout_cohort":"canary"}`)...)
	if _, err := DecodeActivationManifest(unknown); err == nil {
		t.Fatal("unknown staged-rollout field was admitted")
	}
}

func TestVerifyActivationArtifactsExactReadsEveryNonEventProof(t *testing.T) {
	_, manifest, _, _ := activationFixture(t, false)
	reader := activationArtifactFixture{}
	if err := VerifyActivationArtifacts(t.Context(), reader, manifest); err != nil {
		t.Fatal(err)
	}
	broken := reader
	broken.tamper = true
	err := VerifyActivationArtifacts(t.Context(), broken, manifest)
	assertActivationCode(t, err, ActivationProofSetInvalid)

	missing := manifest
	for i := range missing.Proofs {
		if missing.Proofs[i].Kind != ProofEvidenceEvent {
			missing.Proofs[i].ID = "opaque-proof-id"
			break
		}
	}
	err = VerifyActivationArtifacts(t.Context(), reader, missing)
	assertActivationCode(t, err, ActivationProofSetInvalid)
}

func TestVerifyActivationRejectsMissingStaleForgedUnmergedMovedAndCrossBoundaryInputs(t *testing.T) {
	state, manifest, mainline, operatorKey := activationFixture(t, false)
	tests := []struct {
		name   string
		code   ActivationErrorCode
		mutate func(*State, *ActivationManifest, *MainlineProof)
		resign bool
	}{
		{name: "missing proof kind", code: ActivationManifestInvalid, mutate: func(_ *State, m *ActivationManifest, _ *MainlineProof) { m.Proofs = m.Proofs[1:] }},
		{name: "extra proof", code: ActivationManifestInvalid, mutate: func(_ *State, m *ActivationManifest, _ *MainlineProof) {
			m.Proofs = append(m.Proofs, ActivationProof{Kind: "foreign", ID: "extra", Digest: digestText("extra")})
		}},
		{name: "stale evidence head", code: ActivationEvidenceStale, resign: true, mutate: func(_ *State, m *ActivationManifest, _ *MainlineProof) { m.EvidenceHead = "older-head" }},
		{name: "unsigned", code: ActivationSignatureInvalid, mutate: func(_ *State, m *ActivationManifest, _ *MainlineProof) { m.OperatorSignature = ActivationSignature{} }},
		{name: "forged", code: ActivationSignatureInvalid, mutate: func(_ *State, m *ActivationManifest, _ *MainlineProof) {
			m.OperatorSignature.Signature = fmt.Sprintf("%0128d", 0)
		}},
		{name: "unmerged", code: ActivationMainlineInvalid, resign: true, mutate: func(_ *State, _ *ActivationManifest, p *MainlineProof) { p.HumanMerged = false }},
		{name: "moved mainline", code: ActivationMainlineInvalid, resign: true, mutate: func(_ *State, _ *ActivationManifest, p *MainlineProof) { p.CurrentObject = "main-object-new" }},
		{name: "cross repository", code: ActivationEvidenceStale, resign: true, mutate: func(_ *State, m *ActivationManifest, _ *MainlineProof) { m.Repository = "other/repository" }},
		{name: "cross ref", code: ActivationMainlineInvalid, resign: true, mutate: func(_ *State, _ *ActivationManifest, p *MainlineProof) { p.MainlineRef = "refs/heads/release" }},
		{name: "changed evidence proof", code: ActivationProofSetInvalid, resign: true, mutate: func(_ *State, m *ActivationManifest, p *MainlineProof) {
			for i := range m.Proofs {
				if m.Proofs[i].Kind == ProofEvidenceEvent {
					m.Proofs[i].Digest = digestText("changed")
				}
			}
			p.Proofs = append([]ActivationProof(nil), m.Proofs...)
		}},
		{name: "changed predicate result", code: ActivationPredicateFalse, resign: true, mutate: func(_ *State, m *ActivationManifest, _ *MainlineProof) { m.Predicate.AvailabilityDefects = 1 }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, m, p := state, manifest, mainline
			m.Proofs, p.Proofs = append([]ActivationProof(nil), manifest.Proofs...), append([]ActivationProof(nil), mainline.Proofs...)
			tc.mutate(&s, &m, &p)
			if tc.resign {
				var err error
				m, err = SignActivationManifest(m, "operator-key-1", operatorKey)
				if err != nil {
					t.Fatal(err)
				}
				digest, err := DigestActivationManifest(m)
				if err != nil {
					t.Fatal(err)
				}
				p.ManifestDigest, p.OperatorAuthorizationID = digest, m.OperatorAuthorizationID
				p, err = SignMainlineProof(p, "operator-key-1", operatorKey)
				if err != nil {
					t.Fatal(err)
				}
			}
			_, err := VerifyActivation(s, m, p)
			assertActivationCode(t, err, tc.code)
		})
	}
}

func TestVerifyActivationRejectsCurrentInvalidatedEvidence(t *testing.T) {
	state, manifest, mainline, _ := activationFixture(t, true)
	_, err := VerifyActivation(state, manifest, mainline)
	assertActivationCode(t, err, ActivationProofInvalidated)
}

func TestCheckpointIdentityReplayAndChangeSemantics(t *testing.T) {
	state, manifest, mainline, _ := activationFixture(t, false)
	first, err := VerifyActivation(state, manifest, mainline)
	if err != nil {
		t.Fatal(err)
	}
	second, err := VerifyActivation(state, manifest, mainline)
	if err != nil {
		t.Fatal(err)
	}
	firstEvent := checkpointEvent(first, Producer{Class: string(ProducerReleaseGate), Generation: "release/v1"})
	secondEvent := checkpointEvent(second, firstEvent.Producer)
	if mustID(t, firstEvent) != mustID(t, secondEvent) {
		t.Fatal("byte-identical replay changed identity")
	}

	changed := first
	changed.Payload.ActivationPolicyVersion = "activation/v3"
	if mustID(t, checkpointEvent(changed, firstEvent.Producer)) == mustID(t, firstEvent) {
		t.Fatal("policy change retained checkpoint identity")
	}
	changed = first
	changed.Payload.ExactMainlineObject = "main-object-new"
	if mustID(t, checkpointEvent(changed, firstEvent.Producer)) == mustID(t, firstEvent) {
		t.Fatal("mainline change retained checkpoint identity")
	}
	changed = first
	changed.Payload.ProofManifestDigest = digestText("other-manifest")
	if mustID(t, checkpointEvent(changed, firstEvent.Producer)) == mustID(t, firstEvent) {
		t.Fatal("manifest change retained checkpoint identity")
	}

	changed = first
	changed.Payload.Reviews++
	conflictA := checkpointEvent(first, firstEvent.Producer)
	conflictB := checkpointEvent(changed, firstEvent.Producer)
	if mustID(t, conflictA) != mustID(t, conflictB) {
		t.Fatal("non-identity result change did not preserve conflict identity")
	}
	if bytesA, _ := EncodeUnsignedEnvelope(conflictA); reflect.DeepEqual(bytesA, mustUnsigned(t, conflictB)) {
		t.Fatal("changed result did not change signed content")
	}
	changedProducer := checkpointEvent(first, Producer{Class: string(ProducerReleaseGate), Generation: "release/v2"})
	if mustID(t, changedProducer) != mustID(t, firstEvent) {
		t.Fatal("producer generation did not preserve semantic identity")
	}
	if reflect.DeepEqual(mustUnsigned(t, changedProducer), mustUnsigned(t, firstEvent)) {
		t.Fatal("producer generation did not create typed-conflict content")
	}
}

func TestSignedCheckpointAppenderRejectsGenericWriterAndUsesReleaseGate(t *testing.T) {
	state, manifest, mainline, _ := activationFixture(t, false)
	checkpoint, err := VerifyActivation(state, manifest, mainline)
	if err != nil {
		t.Fatal(err)
	}
	generic := SignedCheckpointAppender{Store: &activationCaptureStore{}, Capability: SigningCapability{ProducerClass: ProducerAttemptOwner, EventKinds: []EventKind{ActivationCheckpointEvent}}}
	if _, err := generic.AppendCheckpoint(t.Context(), checkpoint); !errors.Is(err, ErrActivationRejected) {
		t.Fatalf("generic writer error = %v", err)
	}

	_, fx := reducerFixture(t)
	capture := &activationCaptureStore{}
	key := fx.privateKeys[ProducerReleaseGate]
	writer := SignedCheckpointAppender{Store: capture, Capability: SigningCapability{Repository: state.Repository, EvidenceRef: state.EvidenceRef,
		ProducerClass: ProducerReleaseGate, ImplementationGeneration: generationFor(ProducerReleaseGate), KeyID: keyIDFor(ProducerReleaseGate),
		PrivateKey: key, EventKinds: []EventKind{ActivationCheckpointEvent}, Roles: []string{""}}}
	if _, err := writer.AppendCheckpoint(t.Context(), checkpoint); err != nil {
		t.Fatal(err)
	}
	if capture.event == nil || capture.event.Kind != ActivationCheckpointEvent || capture.event.Producer.Class != string(ProducerReleaseGate) {
		t.Fatalf("captured event = %+v", capture.event)
	}
}

func activationFixture(t *testing.T, invalidated bool) (State, ActivationManifest, MainlineProof, ed25519.PrivateKey) {
	t.Helper()
	r, fx := reducerFixture(t)
	events := fx.implementation(true)
	if invalidated {
		events = fx.invalidated()
	}
	state, err := r.Reduce(events)
	if err != nil {
		t.Fatal(err)
	}
	state, err = BindEvidenceHead(state, ObservedHead{EvidenceRef: state.EvidenceRef, Commit: "evidence-head-current"})
	if err != nil {
		t.Fatal(err)
	}

	var evidence Event
	if invalidated {
		for _, event := range state.events {
			if event.Kind == RoutingResolved {
				evidence = event
				break
			}
		}
	} else {
		evidence = state.events[0]
	}
	proofs := activationProofFixture(evidence)
	manifest := ActivationManifest{Schema: ActivationManifestSchema, Repository: state.Repository, EvidenceRef: state.EvidenceRef,
		EvidenceHead: state.EvidenceHead, MainlineRef: "refs/heads/main", ExactMainlineObject: "main-object-abc",
		ActivationPolicyVersion: "activation/v2", OperatorAuthorizationID: "auth-activation", Proofs: proofs,
		Predicate: ActivationPredicate{ActivationReady: true, Reviews: 10, CoveragePct: "100", MatrixCellsRequired: 12, MatrixCellsCovered: 12,
			BlockingDisposed: 5, Precision: "0.7", NitP50: "3", DwellP50: "42.25", LiveBlockingDODGreen: true, LaunchReadinessGreen: true, ParentSeamGreen: true}}
	seed := sha256.Sum256([]byte("activation-operator"))
	operatorKey := ed25519.NewKeyFromSeed(seed[:])
	manifest, err = SignActivationManifest(manifest, "operator-key-1", operatorKey)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := DigestActivationManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	mainline := MainlineProof{Schema: MainlineProofSchema, Repository: state.Repository, MainlineRef: manifest.MainlineRef,
		ExactMainlineObject: manifest.ExactMainlineObject, CurrentObject: manifest.ExactMainlineObject, HumanMerged: true, HumanMergeActor: "human:operator",
		ManifestDigest: digest, OperatorAuthorizationID: manifest.OperatorAuthorizationID, Proofs: append([]ActivationProof(nil), manifest.Proofs...)}
	mainline, err = SignMainlineProof(mainline, "operator-key-1", operatorKey)
	if err != nil {
		t.Fatal(err)
	}
	return state, manifest, mainline, operatorKey
}

func activationProofFixture(event Event) []ActivationProof {
	proofs := []ActivationProof{}
	for i := 1; i <= 6; i++ {
		id := fmt.Sprintf("repo:.pyxis/routing-evidence/proofs/runtime-generation-%d.json", i)
		proofs = append(proofs, ActivationProof{Kind: ProofRuntimeGeneration, ID: id, Digest: digestText(id)})
	}
	for _, kind := range requiredActivationProofKinds[1:] {
		id := "repo:.pyxis/routing-evidence/proofs/" + string(kind) + ".json"
		proofs = append(proofs, ActivationProof{Kind: kind, ID: id, Digest: digestText(id)})
	}
	for i := range proofs {
		if proofs[i].Kind == ProofEvidenceEvent {
			proofs[i].ID, proofs[i].Digest = string(event.EventID), event.ContentDigest
		}
	}
	normalized, err := normalizeActivationProofs(proofs)
	if err != nil {
		panic(err)
	}
	return normalized
}

type activationArtifactFixture struct{ tamper bool }

func (r activationArtifactFixture) ReadActivationArtifact(_ context.Context, _ string, path string) ([]byte, error) {
	if r.tamper {
		return []byte("tampered"), nil
	}
	return []byte("repo:" + path), nil
}

func digestText(value string) string {
	sum := sha256.Sum256([]byte(value))
	return fmt.Sprintf("%x", sum[:])
}

func checkpointEvent(checkpoint ActivationCheckpoint, producer Producer) Event {
	return Event{EvidenceRef: checkpoint.EvidenceRef, Kind: ActivationCheckpointEvent, Payload: Payload{ActivationCheckpoint: &checkpoint.Payload}, Producer: producer, Repository: checkpoint.Repository, Schema: Schema, Version: EventVersion}
}

func mustUnsigned(t *testing.T, event Event) []byte {
	t.Helper()
	b, err := EncodeUnsignedEnvelope(event)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func assertActivationCode(t *testing.T, err error, want ActivationErrorCode) {
	t.Helper()
	var got *ActivationError
	if !errors.As(err, &got) || got.Code != want {
		t.Fatalf("activation error = %v, want %s", err, want)
	}
}

type activationCaptureStore struct{ event *Event }

func (s *activationCaptureStore) Append(_ context.Context, request AppendRequest) (EvidenceProof, error) {
	s.event = request.Event
	return EvidenceProof{EvidenceRef: request.ObservedHead.EvidenceRef, Commit: "checkpoint-head", Entries: []EvidenceEntry{{Path: "event", EventID: "id", ContentDigest: "digest"}}}, nil
}
func (*activationCaptureStore) ReadSubject(context.Context, SubjectID) (ExactRead, error) {
	return ExactRead{}, errors.New("unused")
}
func (*activationCaptureStore) ReadProof(context.Context, EvidenceProof) (ExactRead, error) {
	return ExactRead{}, errors.New("unused")
}
