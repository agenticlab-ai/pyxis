package routingevidence

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"reflect"
	"sort"
	"strings"
)

const (
	activationManifestDomain = ActivationManifestSchema + "\x00"
	mainlineProofDomain      = MainlineProofSchema + "\x00"
)

type ActivationProofKind string

const (
	ProofRuntimeGeneration ActivationProofKind = "runtime_generation"
	ProofCatalog           ActivationProofKind = "catalog"
	ProofQualification     ActivationProofKind = "qualification"
	ProofEvidenceEvent     ActivationProofKind = "evidence_event"
	ProofConformance       ActivationProofKind = "conformance"
	ProofEvaluation        ActivationProofKind = "evaluation"
	ProofDogfood           ActivationProofKind = "dogfood"
	ProofDefinitionOfDone  ActivationProofKind = "definition_of_done"
	ProofSeam              ActivationProofKind = "seam"
	ProofFallback          ActivationProofKind = "fallback"
	ProofInvalidation      ActivationProofKind = "invalidation"
	ProofReachability      ActivationProofKind = "reachability"
)

var requiredActivationProofKinds = []ActivationProofKind{
	ProofRuntimeGeneration, ProofCatalog, ProofQualification, ProofEvidenceEvent,
	ProofConformance, ProofEvaluation, ProofDogfood, ProofDefinitionOfDone,
	ProofSeam, ProofFallback, ProofInvalidation, ProofReachability,
}

// ActivationProof exact-identifies one manifest input. Evidence-event IDs are
// canonical routing event IDs; every other ID names an object at the signed
// candidate mainline object. Digest is always the object's lowercase SHA-256.
type ActivationProof struct {
	Digest string              `json:"digest"`
	ID     string              `json:"id"`
	Kind   ActivationProofKind `json:"kind"`
}

// ActivationArtifactReader exact-reads one repository proof artifact at the
// manifest's named mainline object. Non-evidence proofs use IDs of the form
// `repo:<clean-relative-path>`; event proofs remain exact-read through State.
type ActivationArtifactReader interface {
	ReadActivationArtifact(context.Context, string, string) ([]byte, error)
}

type ActivationPredicate struct {
	ActivationReady      bool   `json:"activation_ready"`
	AvailabilityDefects  int    `json:"availability_defects"`
	BlockingDisposed     int    `json:"blocking_disposed"`
	BurstFlag            bool   `json:"burst_flag"`
	CoveragePct          string `json:"coverage_pct"`
	DwellP50             string `json:"dwell_p50"`
	LaunchReadinessGreen bool   `json:"launch_readiness_green"`
	LiveBlockingDODGreen bool   `json:"live_blocking_dod_green"`
	MatrixCellsCovered   int    `json:"matrix_cells_covered"`
	MatrixCellsRequired  int    `json:"matrix_cells_required"`
	NitP50               string `json:"nit_p50"`
	ParentSeamGreen      bool   `json:"parent_seam_green"`
	Precision            string `json:"precision"`
	ProcessViolations    int    `json:"process_violations"`
	Reviews              int    `json:"reviews"`
}

type ActivationSignature struct {
	Algorithm string `json:"algorithm"`
	KeyID     string `json:"key_id"`
	Signature string `json:"signature"`
}

type ActivationManifest struct {
	ActivationPolicyVersion string              `json:"activation_policy_version"`
	EvidenceHead            string              `json:"evidence_head"`
	EvidenceRef             string              `json:"evidence_ref"`
	ExactMainlineObject     string              `json:"exact_mainline_object"`
	MainlineRef             string              `json:"mainline_ref"`
	OperatorAuthorizationID string              `json:"operator_authorization_id"`
	OperatorSignature       ActivationSignature `json:"operator_signature"`
	Predicate               ActivationPredicate `json:"predicate"`
	Proofs                  []ActivationProof   `json:"proofs"`
	Repository              string              `json:"repository"`
	Schema                  string              `json:"schema"`
}

// MainlineProof is a signed exact-read statement, not a branch or proposal
// status. CurrentObject must still equal ExactMainlineObject when evaluated.
type MainlineProof struct {
	CurrentObject           string              `json:"current_object"`
	ExactMainlineObject     string              `json:"exact_mainline_object"`
	HumanMergeActor         string              `json:"human_merge_actor"`
	HumanMerged             bool                `json:"human_merged"`
	MainlineRef             string              `json:"mainline_ref"`
	ManifestDigest          string              `json:"manifest_digest"`
	OperatorAuthorizationID string              `json:"operator_authorization_id"`
	Proofs                  []ActivationProof   `json:"proofs"`
	Repository              string              `json:"repository"`
	Schema                  string              `json:"schema"`
	Signature               ActivationSignature `json:"signature"`
}

// ActivationCheckpoint is the least-privilege writer input. Its observed head
// is part of the CAS lease but not part of the canonical event payload.
type ActivationCheckpoint struct {
	ObservedHead ObservedHead
	Payload      ActivationCheckpointPayload
	Repository   string
	EvidenceRef  string
}

var ErrActivationRejected = errors.New("routing activation rejected")

type ActivationErrorCode string

const (
	ActivationManifestInvalid  ActivationErrorCode = "manifest_invalid"
	ActivationProofSetInvalid  ActivationErrorCode = "proof_set_invalid"
	ActivationSignatureInvalid ActivationErrorCode = "operator_signature_invalid"
	ActivationMainlineInvalid  ActivationErrorCode = "mainline_proof_invalid"
	ActivationEvidenceStale    ActivationErrorCode = "evidence_stale"
	ActivationProofInvalidated ActivationErrorCode = "proof_invalidated"
	ActivationPredicateFalse   ActivationErrorCode = "predicate_false"
)

type ActivationError struct {
	Code   ActivationErrorCode
	Detail string
}

func (e *ActivationError) Error() string { return fmt.Sprintf("%s: %s", e.Code, e.Detail) }
func (e *ActivationError) Unwrap() error { return ErrActivationRejected }
func activationReject(code ActivationErrorCode, format string, args ...any) error {
	return &ActivationError{Code: code, Detail: fmt.Sprintf(format, args...)}
}

func SignActivationManifest(manifest ActivationManifest, keyID string, private ed25519.PrivateKey) (ActivationManifest, error) {
	manifest.Schema = ActivationManifestSchema
	manifest.OperatorSignature = ActivationSignature{}
	wire, normalized, err := activationManifestUnsigned(manifest)
	if err != nil {
		return ActivationManifest{}, err
	}
	if keyID == "" || len(private) != ed25519.PrivateKeySize {
		return ActivationManifest{}, activationReject(ActivationSignatureInvalid, "operator signing key is invalid")
	}
	normalized.OperatorSignature = ActivationSignature{Algorithm: "ed25519", KeyID: keyID, Signature: hex.EncodeToString(ed25519.Sign(private, append([]byte(activationManifestDomain), wire...)))}
	return normalized, nil
}

func DigestActivationManifest(manifest ActivationManifest) (string, error) {
	_, normalized, err := activationManifestUnsigned(manifest)
	if err != nil {
		return "", err
	}
	normalized.OperatorSignature = manifest.OperatorSignature
	return canonicalDigest(struct {
		Domain   string             `json:"domain"`
		Manifest ActivationManifest `json:"manifest"`
	}{ProofManifestDomain, normalized})
}

func EncodeActivationManifest(manifest ActivationManifest) ([]byte, error) {
	_, normalized, err := activationManifestUnsigned(manifest)
	if err != nil {
		return nil, err
	}
	normalized.OperatorSignature = manifest.OperatorSignature
	if normalized.OperatorSignature.Algorithm != "ed25519" || normalized.OperatorSignature.KeyID == "" {
		return nil, activationReject(ActivationSignatureInvalid, "manifest is unsigned")
	}
	return canonicalJSON(normalized)
}

func DecodeActivationManifest(wire []byte) (ActivationManifest, error) {
	canonical, err := canonicalizeInput(wire)
	if err != nil || !bytes.Equal(canonical, wire) {
		return ActivationManifest{}, activationReject(ActivationManifestInvalid, "manifest is not canonical JSON")
	}
	var manifest ActivationManifest
	if err := decodeClosed(wire, &manifest); err != nil {
		return ActivationManifest{}, activationReject(ActivationManifestInvalid, "%v", err)
	}
	_, normalized, err := activationManifestUnsigned(manifest)
	if err != nil {
		return ActivationManifest{}, activationReject(ActivationManifestInvalid, "%v", err)
	}
	normalized.OperatorSignature = manifest.OperatorSignature
	return normalized, nil
}

func SignMainlineProof(proof MainlineProof, keyID string, private ed25519.PrivateKey) (MainlineProof, error) {
	proof.Schema, proof.Signature = MainlineProofSchema, ActivationSignature{}
	wire, normalized, err := mainlineUnsigned(proof)
	if err != nil {
		return MainlineProof{}, err
	}
	if keyID == "" || len(private) != ed25519.PrivateKeySize {
		return MainlineProof{}, activationReject(ActivationSignatureInvalid, "mainline signing key is invalid")
	}
	normalized.Signature = ActivationSignature{Algorithm: "ed25519", KeyID: keyID, Signature: hex.EncodeToString(ed25519.Sign(private, append([]byte(mainlineProofDomain), wire...)))}
	return normalized, nil
}

func EncodeMainlineProof(proof MainlineProof) ([]byte, error) {
	_, normalized, err := mainlineUnsigned(proof)
	if err != nil {
		return nil, err
	}
	normalized.Signature = proof.Signature
	if normalized.Signature.Algorithm != "ed25519" || normalized.Signature.KeyID == "" {
		return nil, activationReject(ActivationSignatureInvalid, "mainline proof is unsigned")
	}
	return canonicalJSON(normalized)
}

func DecodeMainlineProof(wire []byte) (MainlineProof, error) {
	canonical, err := canonicalizeInput(wire)
	if err != nil || !bytes.Equal(canonical, wire) {
		return MainlineProof{}, activationReject(ActivationMainlineInvalid, "proof is not canonical JSON")
	}
	var proof MainlineProof
	if err := decodeClosed(wire, &proof); err != nil {
		return MainlineProof{}, activationReject(ActivationMainlineInvalid, "%v", err)
	}
	_, normalized, err := mainlineUnsigned(proof)
	if err != nil {
		return MainlineProof{}, activationReject(ActivationMainlineInvalid, "%v", err)
	}
	normalized.Signature = proof.Signature
	return normalized, nil
}

// VerifyActivation is the canonical pure reducer over already exact-read
// inputs. ActivationService first exact-reads every non-event proof artifact;
// callers that invoke this lower-level function own that prerequisite.
func VerifyActivation(state State, manifest ActivationManifest, mainline MainlineProof) (ActivationCheckpoint, error) {
	unsigned, normalized, err := activationManifestUnsigned(manifest)
	if err != nil {
		return ActivationCheckpoint{}, activationReject(ActivationManifestInvalid, "%v", err)
	}
	auth, publicKey, err := activationAuthority(state.producerPolicy, normalized.OperatorAuthorizationID)
	if err != nil {
		return ActivationCheckpoint{}, err
	}
	if err := verifyActivationSignature(manifest.OperatorSignature, auth.ActorIdentity, publicKey, append([]byte(activationManifestDomain), unsigned...)); err != nil {
		return ActivationCheckpoint{}, err
	}
	digest, err := DigestActivationManifest(manifest)
	if err != nil {
		return ActivationCheckpoint{}, activationReject(ActivationManifestInvalid, "%v", err)
	}
	if state.Repository != normalized.Repository || state.EvidenceRef != normalized.EvidenceRef || state.EvidenceHead == "" || state.EvidenceHead != normalized.EvidenceHead {
		return ActivationCheckpoint{}, activationReject(ActivationEvidenceStale, "manifest does not bind the current canonical evidence head")
	}
	mainUnsigned, normalizedMainline, err := mainlineUnsigned(mainline)
	if err != nil {
		return ActivationCheckpoint{}, activationReject(ActivationMainlineInvalid, "%v", err)
	}
	if normalizedMainline.Repository != normalized.Repository || normalizedMainline.MainlineRef != normalized.MainlineRef ||
		normalizedMainline.ExactMainlineObject != normalized.ExactMainlineObject || normalizedMainline.CurrentObject != normalized.ExactMainlineObject ||
		normalizedMainline.ManifestDigest != digest || normalizedMainline.OperatorAuthorizationID != normalized.OperatorAuthorizationID ||
		!normalizedMainline.HumanMerged || normalizedMainline.HumanMergeActor == "" || !reflect.DeepEqual(normalizedMainline.Proofs, normalized.Proofs) {
		return ActivationCheckpoint{}, activationReject(ActivationMainlineInvalid, "mainline proof is unmerged, moved, cross-boundary, or proof-different")
	}
	if err := verifyActivationSignature(mainline.Signature, auth.ActorIdentity, publicKey, append([]byte(mainlineProofDomain), mainUnsigned...)); err != nil {
		return ActivationCheckpoint{}, err
	}
	if err := verifyCurrentEvidenceProofs(state, normalized.Proofs); err != nil {
		return ActivationCheckpoint{}, err
	}
	if !predicateResult(normalized.Predicate) || !normalized.Predicate.ActivationReady {
		return ActivationCheckpoint{}, activationReject(ActivationPredicateFalse, "declared result does not match the complete union predicate")
	}
	proofIDs := make([]string, len(normalized.Proofs))
	for i := range normalized.Proofs {
		proofIDs[i] = normalized.Proofs[i].ID
	}
	p := normalized.Predicate
	payload := ActivationCheckpointPayload{Activated: false, ActivationPolicyVersion: normalized.ActivationPolicyVersion,
		ActivationReady: true, AvailabilityDefects: p.AvailabilityDefects, BlockingDisposed: p.BlockingDisposed, BurstFlag: p.BurstFlag,
		CoveragePct: p.CoveragePct, DwellP50: p.DwellP50, EvidenceHead: normalized.EvidenceHead, ExactMainlineObject: normalized.ExactMainlineObject,
		LaunchReadinessGreen: p.LaunchReadinessGreen, LiveBlockingDODGreen: p.LiveBlockingDODGreen, MatrixCellsCovered: p.MatrixCellsCovered,
		MatrixCellsRequired: p.MatrixCellsRequired, MainlineRef: normalized.MainlineRef, NitP50: p.NitP50,
		OperatorAuthorizationID: normalized.OperatorAuthorizationID, ParentSeamGreen: p.ParentSeamGreen, Precision: p.Precision,
		ProcessViolations: p.ProcessViolations, ProofIDs: proofIDs, ProofManifestDigest: digest, Reviews: p.Reviews}
	return ActivationCheckpoint{ObservedHead: ObservedHead{EvidenceRef: state.EvidenceRef, Commit: state.EvidenceHead}, Payload: payload, Repository: state.Repository, EvidenceRef: state.EvidenceRef}, nil
}

func activationManifestUnsigned(manifest ActivationManifest) ([]byte, ActivationManifest, error) {
	if manifest.Schema != ActivationManifestSchema || manifest.Repository == "" || manifest.EvidenceRef == "" || manifest.EvidenceHead == "" ||
		manifest.MainlineRef == "" || manifest.ExactMainlineObject == "" || manifest.ActivationPolicyVersion == "" || manifest.OperatorAuthorizationID == "" {
		return nil, ActivationManifest{}, errors.New("manifest identity is incomplete")
	}
	proofs, err := normalizeActivationProofs(manifest.Proofs)
	if err != nil {
		return nil, ActivationManifest{}, err
	}
	manifest.Proofs = proofs
	signature := manifest.OperatorSignature
	manifest.OperatorSignature = ActivationSignature{}
	wire, err := canonicalJSON(manifest)
	manifest.OperatorSignature = signature
	return wire, manifest, err
}

func mainlineUnsigned(proof MainlineProof) ([]byte, MainlineProof, error) {
	if proof.Schema != MainlineProofSchema || proof.Repository == "" || proof.MainlineRef == "" || proof.ExactMainlineObject == "" || proof.CurrentObject == "" || proof.ManifestDigest == "" || proof.OperatorAuthorizationID == "" {
		return nil, MainlineProof{}, errors.New("mainline proof identity is incomplete")
	}
	proofs, err := normalizeActivationProofs(proof.Proofs)
	if err != nil {
		return nil, MainlineProof{}, err
	}
	proof.Proofs = proofs
	signature := proof.Signature
	proof.Signature = ActivationSignature{}
	wire, err := canonicalJSON(proof)
	proof.Signature = signature
	return wire, proof, err
}

func normalizeActivationProofs(proofs []ActivationProof) ([]ActivationProof, error) {
	out := append([]ActivationProof(nil), proofs...)
	sort.Slice(out, func(i, j int) bool { return proofSortKey(out[i]) < proofSortKey(out[j]) })
	counts := map[ActivationProofKind]int{}
	seenIDs := map[string]bool{}
	for _, proof := range out {
		if proof.ID == "" || !containsProofKind(requiredActivationProofKinds, proof.Kind) {
			return nil, errors.New("proof has an unknown kind or empty identity")
		}
		if _, err := decodeHexExact(proof.Digest, sha256.Size); err != nil {
			return nil, fmt.Errorf("proof %q digest: %w", proof.ID, err)
		}
		if seenIDs[proof.ID] {
			return nil, fmt.Errorf("proof %q is duplicated", proof.ID)
		}
		seenIDs[proof.ID], counts[proof.Kind] = true, counts[proof.Kind]+1
	}
	for _, kind := range requiredActivationProofKinds {
		if counts[kind] == 0 {
			return nil, fmt.Errorf("proof kind %q is missing", kind)
		}
	}
	if counts[ProofRuntimeGeneration] != 6 {
		return nil, fmt.Errorf("runtime_generation proof count is %d, want 6", counts[ProofRuntimeGeneration])
	}
	return out, nil
}

func proofSortKey(p ActivationProof) string {
	return string(p.Kind) + "\x00" + p.ID + "\x00" + p.Digest
}
func containsProofKind(values []ActivationProofKind, want ActivationProofKind) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}

func activationAuthority(policy ProducerPolicy, id string) (AuthorizationReference, ed25519.PublicKey, error) {
	if err := ValidateProducerPolicy(policy); err != nil {
		return AuthorizationReference{}, nil, activationReject(ActivationSignatureInvalid, "producer policy: %v", err)
	}
	for _, auth := range policy.Authorizations {
		if auth.ID == id && auth.Kind == AuthorizationActivationManifest {
			key, err := decodeHexExact(auth.PublicKey, ed25519.PublicKeySize)
			if err != nil {
				break
			}
			return auth, ed25519.PublicKey(key), nil
		}
	}
	return AuthorizationReference{}, nil, activationReject(ActivationSignatureInvalid, "operator authorization is not trusted")
}

func verifyActivationSignature(signature ActivationSignature, keyID string, publicKey ed25519.PublicKey, message []byte) error {
	if signature.Algorithm != "ed25519" || signature.KeyID != keyID {
		return activationReject(ActivationSignatureInvalid, "signature key or algorithm is not authorized")
	}
	sig, err := decodeHexExact(signature.Signature, ed25519.SignatureSize)
	if err != nil || !ed25519.Verify(publicKey, message, sig) {
		return activationReject(ActivationSignatureInvalid, "Ed25519 signature does not verify")
	}
	return nil
}

func verifyCurrentEvidenceProofs(state State, proofs []ActivationProof) error {
	byID := map[string]Event{}
	invalidated := map[string]bool{}
	for _, event := range state.events {
		byID[string(event.EventID)] = event
	}
	for _, event := range state.events {
		if event.Kind == RoutingEvidenceInvalidated {
			for _, id := range event.Payload.RoutingEvidenceInvalidated.AffectedEventIDs {
				invalidated[id] = true
			}
		}
	}
	for _, proof := range proofs {
		if proof.Kind != ProofEvidenceEvent {
			continue
		}
		event, ok := byID[proof.ID]
		if !ok || event.ContentDigest != proof.Digest {
			return activationReject(ActivationProofSetInvalid, "evidence proof %q is missing or stale", proof.ID)
		}
		if invalidated[proof.ID] {
			return activationReject(ActivationProofInvalidated, "evidence proof %q is invalidated", proof.ID)
		}
	}
	return nil
}

// VerifyActivationArtifacts proves that every non-event proof digest names
// bytes present at the exact candidate mainline object. A digest in a signed
// manifest is not evidence by itself; the corresponding object must exist and
// exact-read before the union predicate can authorize a checkpoint.
func VerifyActivationArtifacts(ctx context.Context, reader ActivationArtifactReader, manifest ActivationManifest) error {
	if reader == nil {
		return activationReject(ActivationProofSetInvalid, "activation artifact reader is unavailable")
	}
	_, normalized, err := activationManifestUnsigned(manifest)
	if err != nil {
		return activationReject(ActivationManifestInvalid, "%v", err)
	}
	for _, proof := range normalized.Proofs {
		if proof.Kind == ProofEvidenceEvent {
			continue
		}
		if !strings.HasPrefix(proof.ID, "repo:") {
			return activationReject(ActivationProofSetInvalid, "proof %q is not an exact repository artifact", proof.ID)
		}
		artifact, readErr := reader.ReadActivationArtifact(ctx, normalized.ExactMainlineObject, strings.TrimPrefix(proof.ID, "repo:"))
		if readErr != nil {
			return activationReject(ActivationProofSetInvalid, "proof %q exact-read: %v", proof.ID, readErr)
		}
		digest := sha256.Sum256(artifact)
		if hex.EncodeToString(digest[:]) != proof.Digest {
			return activationReject(ActivationProofSetInvalid, "proof %q digest differs from exact mainline bytes", proof.ID)
		}
	}
	return nil
}

func predicateResult(p ActivationPredicate) bool {
	return p.Reviews >= 10 && p.MatrixCellsRequired > 0 && p.MatrixCellsCovered == p.MatrixCellsRequired && p.BlockingDisposed >= 5 &&
		decimalEqual(p.CoveragePct, "100") && decimalAtLeast(p.Precision, "0.7") && decimalAtMost(p.Precision, "1") && decimalAtMost(p.NitP50, "3") &&
		canonicalDecimal(p.DwellP50) && p.ProcessViolations == 0 && p.AvailabilityDefects == 0 && p.LiveBlockingDODGreen && p.LaunchReadinessGreen && p.ParentSeamGreen
}
func decimalEqual(value, want string) bool {
	a, ok := new(big.Rat).SetString(value)
	if !ok {
		return false
	}
	b, _ := new(big.Rat).SetString(want)
	return a.Cmp(b) == 0
}

func decimalAtLeast(value, floor string) bool {
	a, ok := new(big.Rat).SetString(value)
	if !ok {
		return false
	}
	b, _ := new(big.Rat).SetString(floor)
	return a.Cmp(b) >= 0
}
func decimalAtMost(value, ceiling string) bool {
	a, ok := new(big.Rat).SetString(value)
	if !ok {
		return false
	}
	b, _ := new(big.Rat).SetString(ceiling)
	return a.Cmp(b) <= 0
}

type CheckpointAppender interface {
	AppendCheckpoint(context.Context, ActivationCheckpoint) (EvidenceProof, error)
}

type SignedCheckpointAppender struct {
	Store      Store
	Capability SigningCapability
}

func (w SignedCheckpointAppender) AppendCheckpoint(ctx context.Context, checkpoint ActivationCheckpoint) (EvidenceProof, error) {
	if w.Store == nil || w.Capability.ProducerClass != ProducerReleaseGate || !containsKind(w.Capability.EventKinds, ActivationCheckpointEvent) {
		return EvidenceProof{}, activationReject(ActivationSignatureInvalid, "writer lacks release-gate checkpoint authority")
	}
	event := Event{EvidenceRef: checkpoint.EvidenceRef, Kind: ActivationCheckpointEvent, Payload: Payload{ActivationCheckpoint: &checkpoint.Payload},
		Producer: Producer{Class: string(w.Capability.ProducerClass), Generation: w.Capability.ImplementationGeneration}, Repository: checkpoint.Repository, Schema: Schema, Version: EventVersion}
	unsigned, err := NewUnsignedEnvelope(event)
	if err != nil {
		return EvidenceProof{}, fmt.Errorf("activation checkpoint: construct envelope: %w", err)
	}
	event.ProducerProof, err = Sign(unsigned, w.Capability)
	if err != nil {
		return EvidenceProof{}, fmt.Errorf("activation checkpoint: sign: %w", err)
	}
	return w.Store.Append(ctx, AppendRequest{ObservedHead: checkpoint.ObservedHead, Event: &event})
}

type ActivationStateReader interface {
	ReadHead(context.Context) (ExactRead, error)
}

type ActivationService struct {
	Reader    ActivationStateReader
	Reducer   Reducer
	Appender  CheckpointAppender
	Store     Store
	Artifacts ActivationArtifactReader
}

func (s ActivationService) Activate(ctx context.Context, manifest ActivationManifest, mainline MainlineProof) (EvidenceProof, error) {
	if s.Reader == nil || s.Appender == nil || s.Store == nil || s.Artifacts == nil {
		return EvidenceProof{}, activationReject(ActivationEvidenceStale, "release gate dependencies are unavailable")
	}
	read, err := s.Reader.ReadHead(ctx)
	if err != nil {
		return EvidenceProof{}, activationReject(ActivationEvidenceStale, "exact-read evidence: %v", err)
	}
	state, err := s.Reducer.Reduce(read.Events)
	if err != nil {
		return EvidenceProof{}, activationReject(ActivationEvidenceStale, "reduce evidence: %v", err)
	}
	state, err = BindEvidenceHead(state, read.ObservedHead)
	if err != nil {
		return EvidenceProof{}, err
	}
	if err := VerifyActivationArtifacts(ctx, s.Artifacts, manifest); err != nil {
		return EvidenceProof{}, err
	}
	checkpoint, err := VerifyActivation(state, manifest, mainline)
	if err != nil {
		return EvidenceProof{}, err
	}
	proof, err := s.Appender.AppendCheckpoint(ctx, checkpoint)
	if err != nil {
		return EvidenceProof{}, fmt.Errorf("activation checkpoint: append: %w", err)
	}
	exact, err := s.Store.ReadProof(ctx, proof)
	if err != nil || len(exact.Events) != 1 || exact.Events[0].Kind != ActivationCheckpointEvent || exact.Events[0].Payload.ActivationCheckpoint.ProofManifestDigest != checkpoint.Payload.ProofManifestDigest {
		return EvidenceProof{}, activationReject(ActivationEvidenceStale, "appended checkpoint did not exact-read")
	}
	return proof, nil
}
