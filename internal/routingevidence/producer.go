package routingevidence

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"sort"
)

const (
	ProducerPolicySchema = "routing-evidence-producer-policy/v1"
	producerProofDomain  = ProducerProofSchema + "\x00"
)

type ProducerClass string

const (
	ProducerPlanGate ProducerClass = "plan-gate"

	ProducerResolver            ProducerClass = "resolver-orchestrator"
	ProducerAttemptOwner        ProducerClass = "attempt-owner"
	ProducerAttemptReaper       ProducerClass = "attempt-reaper"
	ProducerRecoveryCoordinator ProducerClass = "recovery-coordinator"
	ProducerReviewOrchestrator  ProducerClass = "review-orchestrator"
	ProducerMediumAdapter       ProducerClass = "proposal-medium-adapter"
	ProducerEvidenceOwner       ProducerClass = "evidence-owner"
	ProducerReleaseGate         ProducerClass = "release-gate"
)

type AuthorizationKind string

const (
	AuthorizationPolicy             AuthorizationKind = "operator-policy"
	AuthorizationRuntimeFence       AuthorizationKind = "runtime-fence-proof"
	AuthorizationNativeAuthor       AuthorizationKind = "native-author-proof"
	AuthorizationInvalidation       AuthorizationKind = "invalidation-authorization"
	AuthorizationActivationManifest AuthorizationKind = "activation-manifest"
)

// KeyGeneration binds one public key to one producer class and implementation
// generation. PolicyActivationPoints makes rotation exact rather than ordered
// by a clock or a version string.
type KeyGeneration struct {
	Algorithm                string        `json:"algorithm"`
	ImplementationGeneration string        `json:"implementation_generation"`
	KeyID                    string        `json:"key_id"`
	PolicyActivationPoints   []string      `json:"policy_activation_points"`
	ProducerClass            ProducerClass `json:"producer_class"`
	PublicKey                string        `json:"public_key"`
	RevocationReference      string        `json:"revocation_reference,omitempty"`
}

type AuthorizationReference struct {
	ActorClass    string            `json:"actor_class,omitempty"`
	ActorIdentity string            `json:"actor_identity,omitempty"`
	ID            string            `json:"id"`
	Kind          AuthorizationKind `json:"kind"`
	PublicKey     string            `json:"public_key,omitempty"`
	Reference     string            `json:"reference,omitempty"`
}

type ProducerAuthorization struct {
	AuthorizationID          string        `json:"authorization_id"`
	EventKind                EventKind     `json:"event_kind"`
	ImplementationGeneration string        `json:"implementation_generation"`
	KeyID                    string        `json:"key_id"`
	ProducerClass            ProducerClass `json:"producer_class"`
	Role                     string        `json:"role,omitempty"`
}

type GenesisCheckpoint struct {
	AuthorizationReference string `json:"authorization_reference"`
	Commit                 string `json:"commit"`
	EvidenceRef            string `json:"evidence_ref"`
	Repository             string `json:"repository"`
	Tree                   string `json:"tree"`
}

type ProducerPolicy struct {
	ActivationPoint string                   `json:"activation_point"`
	Authorizations  []AuthorizationReference `json:"authorizations"`
	Digest          string                   `json:"digest"`
	EvidenceRef     string                   `json:"evidence_ref"`
	Genesis         GenesisCheckpoint        `json:"genesis"`
	Keys            []KeyGeneration          `json:"keys"`
	Matrix          []ProducerAuthorization  `json:"matrix"`
	Repository      string                   `json:"repository"`
	Schema          string                   `json:"schema"`
	Version         string                   `json:"version"`
}

// SigningCapability is deliberately narrower than a private key: the caller
// cannot use it outside its repository, ref, class, generation, kinds, or roles.
type SigningCapability struct {
	EvidenceRef              string
	EventKinds               []EventKind
	ImplementationGeneration string
	KeyID                    string
	PrivateKey               ed25519.PrivateKey
	ProducerClass            ProducerClass
	Repository               string
	Roles                    []string
}

var ErrProducerIntegrity = errors.New("routing evidence producer integrity failure")

type IntegrityCode string

const (
	IntegrityPolicy       IntegrityCode = "producer_policy_invalid"
	IntegrityUnauthorized IntegrityCode = "producer_unauthorized"
	IntegrityProof        IntegrityCode = "producer_proof_invalid"
	IntegrityContinuity   IntegrityCode = "genesis_continuity_invalid"
)

// IntegrityError is safe to surface to an operator and stable across append
// and fresh-read callers. Detail is diagnostic; Code is the decision surface.
type IntegrityError struct {
	Code   IntegrityCode
	Detail string
}

func (e *IntegrityError) Error() string { return fmt.Sprintf("%s: %s", e.Code, e.Detail) }
func (e *IntegrityError) Unwrap() error { return ErrProducerIntegrity }

func integrity(code IntegrityCode, format string, args ...any) error {
	return &IntegrityError{Code: code, Detail: fmt.Sprintf(format, args...)}
}

func Sign(unsigned UnsignedEnvelope, capability SigningCapability) (ProducerProof, error) {
	wire, err := validateUnsigned(unsigned)
	if err != nil {
		return ProducerProof{}, integrity(IntegrityProof, "unsigned envelope: %v", err)
	}
	role := unsignedRole(unsigned)
	if capability.Repository != unsigned.Repository || capability.EvidenceRef != unsigned.EvidenceRef ||
		string(capability.ProducerClass) != unsigned.Producer.Class || capability.ImplementationGeneration != unsigned.Producer.Generation ||
		!allowedProducer(unsigned.EventKind, capability.ProducerClass) || !validRole(unsigned.EventKind, role) ||
		!containsKind(capability.EventKinds, unsigned.EventKind) || !containsString(capability.Roles, role) {
		return ProducerProof{}, integrity(IntegrityUnauthorized, "signing capability does not admit the envelope boundary")
	}
	if capability.KeyID == "" || len(capability.PrivateKey) != ed25519.PrivateKeySize {
		return ProducerProof{}, integrity(IntegrityProof, "signing capability has no usable Ed25519 key")
	}
	signature := ed25519.Sign(capability.PrivateKey, proofMessage(wire))
	return ProducerProof{Algorithm: "ed25519", KeyID: capability.KeyID, Schema: ProducerProofSchema, Signature: hex.EncodeToString(signature)}, nil
}

// VerifyProducerCanonical is the fresh-reader entry point. It deliberately
// maps codec rejection into the same typed proof class used by appenders.
func VerifyProducerCanonical(canonical []byte, policy ProducerPolicy) error {
	event, err := DecodeCanonical(canonical)
	if err != nil {
		return integrity(IntegrityProof, "stored envelope: %v", err)
	}
	return VerifyProducer(event, policy)
}

func VerifyProducer(event Event, policy ProducerPolicy) error {
	if err := ValidateProducerPolicy(policy); err != nil {
		return integrity(IntegrityPolicy, "%v", err)
	}
	if event.Repository != policy.Repository || event.EvidenceRef != policy.EvidenceRef {
		return integrity(IntegrityUnauthorized, "event repository or ref is outside policy")
	}
	wire, err := EncodeUnsignedEnvelope(event)
	if err != nil {
		return integrity(IntegrityProof, "unsigned envelope: %v", err)
	}
	if event.ProducerProof.Schema != ProducerProofSchema || event.ProducerProof.Algorithm != "ed25519" {
		return integrity(IntegrityProof, "proof schema or algorithm is not admitted")
	}
	key, ok := policyKey(policy, event.ProducerProof.KeyID)
	if !ok || key.RevocationReference != "" || !containsString(key.PolicyActivationPoints, policy.ActivationPoint) {
		return integrity(IntegrityUnauthorized, "key is unknown, revoked, or inactive")
	}
	if string(key.ProducerClass) != event.Producer.Class || key.ImplementationGeneration != event.Producer.Generation {
		return integrity(IntegrityUnauthorized, "key class or generation does not match producer")
	}
	rule, ok := policyRule(policy, event, key)
	if !ok || !authorizationMatches(event, policy, rule.AuthorizationID) {
		return integrity(IntegrityUnauthorized, "event kind, role, or authorization reference is not admitted")
	}
	publicKey, err := decodeHexExact(key.PublicKey, ed25519.PublicKeySize)
	if err != nil {
		return integrity(IntegrityPolicy, "public key %q: %v", key.KeyID, err)
	}
	signature, err := decodeHexExact(event.ProducerProof.Signature, ed25519.SignatureSize)
	if err != nil || !ed25519.Verify(ed25519.PublicKey(publicKey), proofMessage(wire), signature) {
		return integrity(IntegrityProof, "Ed25519 signature does not verify")
	}
	return nil
}

func DigestProducerPolicy(policy ProducerPolicy) (string, error) {
	policy = normalizedPolicy(policy)
	policy.Digest = ""
	b, err := canonicalJSON(struct {
		Domain string         `json:"domain"`
		Policy ProducerPolicy `json:"policy"`
	}{ProducerPolicySchema, policy})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func EncodeProducerPolicy(policy ProducerPolicy) ([]byte, error) {
	if err := ValidateProducerPolicy(policy); err != nil {
		return nil, err
	}
	return canonicalJSON(normalizedPolicy(policy))
}

func DecodeProducerPolicy(b []byte) (ProducerPolicy, error) {
	canonical, err := canonicalizeInput(b)
	if err != nil || !bytes.Equal(canonical, b) {
		return ProducerPolicy{}, fmt.Errorf("decode producer policy: input is not canonical JSON")
	}
	var policy ProducerPolicy
	if err := decodeClosed(b, &policy); err != nil {
		return ProducerPolicy{}, fmt.Errorf("decode producer policy: %w", err)
	}
	if err := ValidateProducerPolicy(policy); err != nil {
		return ProducerPolicy{}, err
	}
	return normalizedPolicy(policy), nil
}

func ValidateProducerPolicy(policy ProducerPolicy) error {
	if policy.Schema != ProducerPolicySchema || policy.Version == "" || policy.Repository == "" || policy.EvidenceRef == "" || policy.ActivationPoint == "" {
		return fmt.Errorf("producer policy: missing or unsupported identity")
	}
	if policy.Genesis.Repository != policy.Repository || policy.Genesis.EvidenceRef != policy.EvidenceRef ||
		policy.Genesis.Commit == "" || policy.Genesis.Tree == "" || policy.Genesis.AuthorizationReference == "" {
		return fmt.Errorf("producer policy: genesis checkpoint does not close the repository and ref")
	}
	want, err := DigestProducerPolicy(policy)
	if err != nil || policy.Digest != want {
		return fmt.Errorf("producer policy: digest mismatch")
	}
	keys := make(map[string]KeyGeneration, len(policy.Keys))
	for _, key := range policy.Keys {
		if key.KeyID == "" || key.Algorithm != "ed25519" || key.ImplementationGeneration == "" || !knownProducerClass(key.ProducerClass) {
			return fmt.Errorf("producer policy: invalid key generation")
		}
		if _, exists := keys[key.KeyID]; exists {
			return fmt.Errorf("producer policy: duplicate key %q", key.KeyID)
		}
		if _, err := decodeHexExact(key.PublicKey, ed25519.PublicKeySize); err != nil {
			return fmt.Errorf("producer policy: key %q: %w", key.KeyID, err)
		}
		if len(key.PolicyActivationPoints) == 0 || !sortedUnique(key.PolicyActivationPoints) {
			return fmt.Errorf("producer policy: key %q has invalid activation points", key.KeyID)
		}
		keys[key.KeyID] = key
	}
	auths := make(map[string]AuthorizationReference, len(policy.Authorizations))
	for _, auth := range policy.Authorizations {
		if auth.ID == "" || !knownAuthorizationKind(auth.Kind) {
			return fmt.Errorf("producer policy: invalid authorization reference")
		}
		if (auth.Kind == AuthorizationPolicy && (auth.Reference != "" || auth.PublicKey != "" || auth.ActorClass != "" || auth.ActorIdentity != "")) ||
			(auth.Kind != AuthorizationPolicy && auth.Kind != AuthorizationActivationManifest && auth.Reference == "") ||
			(auth.Kind == AuthorizationNativeAuthor && (!oneOf(auth.ActorClass, "worker", "operator") || auth.ActorIdentity == "")) ||
			(auth.Kind != AuthorizationNativeAuthor && auth.Kind != AuthorizationActivationManifest && (auth.ActorClass != "" || auth.ActorIdentity != "")) ||
			(auth.Kind == AuthorizationActivationManifest && (auth.Reference != "" || auth.ActorIdentity == "" || auth.ActorClass != "operator")) ||
			(auth.Kind != AuthorizationActivationManifest && auth.PublicKey != "") {
			return fmt.Errorf("producer policy: malformed authorization %q", auth.ID)
		}
		if auth.Kind == AuthorizationActivationManifest {
			if _, err := decodeHexExact(auth.PublicKey, ed25519.PublicKeySize); err != nil {
				return fmt.Errorf("producer policy: activation authorization %q: %w", auth.ID, err)
			}
		}
		if _, exists := auths[auth.ID]; exists {
			return fmt.Errorf("producer policy: duplicate authorization %q", auth.ID)
		}
		auths[auth.ID] = auth
	}
	covered, rules := map[EventKind]bool{}, map[string]bool{}
	for _, rule := range policy.Matrix {
		key, keyOK := keys[rule.KeyID]
		auth, authOK := auths[rule.AuthorizationID]
		identity := fmt.Sprintf("%s\x00%s\x00%s\x00%s", rule.EventKind, rule.ProducerClass, rule.ImplementationGeneration, rule.Role)
		if rules[identity] || !keyOK || !authOK || key.RevocationReference != "" || !containsString(key.PolicyActivationPoints, policy.ActivationPoint) ||
			key.ProducerClass != rule.ProducerClass || key.ImplementationGeneration != rule.ImplementationGeneration ||
			!allowedProducer(rule.EventKind, rule.ProducerClass) || !validRole(rule.EventKind, rule.Role) || auth.Kind != requiredAuthorization(rule.EventKind) {
			return fmt.Errorf("producer policy: invalid or overlapping matrix row for %q", rule.EventKind)
		}
		rules[identity], covered[rule.EventKind] = true, true
	}
	for _, kind := range allEventKinds() {
		if !covered[kind] {
			return fmt.Errorf("producer policy: event kind %q is uncovered", kind)
		}
	}
	return nil
}

func validateUnsigned(unsigned UnsignedEnvelope) ([]byte, error) {
	event := Event{ContentDigest: unsigned.ContentDigest, EventID: unsigned.EventID, EvidenceRef: unsigned.EvidenceRef, Kind: unsigned.EventKind, Payload: unsigned.Payload, Producer: unsigned.Producer, Repository: unsigned.Repository, Schema: unsigned.Schema, Version: unsigned.EventVersion}
	want, err := EncodeUnsignedEnvelope(event)
	if err != nil {
		return nil, err
	}
	got, err := canonicalJSON(unsigned)
	if err != nil || !bytes.Equal(got, want) || unsigned.Domain != CanonicalDomain {
		return nil, fmt.Errorf("derived fields do not match canonical unsigned envelope")
	}
	return want, nil
}

func proofMessage(unsigned []byte) []byte { return append([]byte(producerProofDomain), unsigned...) }

func policyKey(policy ProducerPolicy, id string) (KeyGeneration, bool) {
	for _, key := range policy.Keys {
		if key.KeyID == id {
			return key, true
		}
	}
	return KeyGeneration{}, false
}

func policyRule(policy ProducerPolicy, event Event, key KeyGeneration) (ProducerAuthorization, bool) {
	role := eventRole(event)
	for _, rule := range policy.Matrix {
		if rule.EventKind == event.Kind && rule.ProducerClass == key.ProducerClass && rule.ImplementationGeneration == key.ImplementationGeneration && rule.KeyID == key.KeyID && rule.Role == role {
			return rule, true
		}
	}
	return ProducerAuthorization{}, false
}

func authorizationMatches(event Event, policy ProducerPolicy, id string) bool {
	var auth AuthorizationReference
	found := false
	for _, candidate := range policy.Authorizations {
		if candidate.ID == id {
			auth, found = candidate, true
			break
		}
	}
	if !found {
		return false
	}
	switch auth.Kind {
	case AuthorizationPolicy:
		return auth.Reference == ""
	case AuthorizationRuntimeFence:
		return event.Payload.AttemptOwnerTransferred != nil && event.Payload.AttemptOwnerTransferred.RuntimeFenceProofRef == auth.Reference
	case AuthorizationNativeAuthor:
		p := event.Payload.DispositionRecorded
		return p != nil && p.AttributionVerification == "verified" && p.NativeAuthorIdentityProof == auth.Reference && p.Actor == auth.ActorClass && p.ActorAttribution == auth.ActorIdentity
	case AuthorizationInvalidation:
		return event.Payload.RoutingEvidenceInvalidated != nil && event.Payload.RoutingEvidenceInvalidated.AuthorizationRef == auth.Reference
	case AuthorizationActivationManifest:
		return event.Payload.ActivationCheckpoint != nil && event.Payload.ActivationCheckpoint.OperatorAuthorizationID == auth.ID
	default:
		return false
	}
}

func eventRole(event Event) string {
	switch event.Kind {
	case RoutingResolved:
		return event.Payload.RoutingResolved.Role
	case RoutingResolutionFailed:
		return event.Payload.RoutingResolutionFailed.Role
	case DispositionRecorded:
		return event.Payload.DispositionRecorded.Actor
	case RoutingEvidenceInvalidated:
		return event.Payload.RoutingEvidenceInvalidated.Disposition
	default:
		return ""
	}
}

func unsignedRole(unsigned UnsignedEnvelope) string {
	return eventRole(Event{Kind: unsigned.EventKind, Payload: unsigned.Payload})
}

func requiredAuthorization(kind EventKind) AuthorizationKind {
	switch kind {
	case AttemptOwnerTransferred:
		return AuthorizationRuntimeFence
	case DispositionRecorded:
		return AuthorizationNativeAuthor
	case RoutingEvidenceInvalidated:
		return AuthorizationInvalidation
	case ActivationCheckpointEvent:
		return AuthorizationActivationManifest
	default:
		return AuthorizationPolicy
	}
}

func allowedProducer(kind EventKind, class ProducerClass) bool {
	switch kind {
	case ExecutionDemandFrozen:
		return class == ProducerPlanGate

	case RoutingResolved, RoutingResolutionFailed:
		return class == ProducerResolver
	case RoutedAttemptCompleted, RoutedTaskOutcome:
		return class == ProducerAttemptOwner || class == ProducerAttemptReaper
	case AttemptOwnerTransferred:
		return class == ProducerRecoveryCoordinator
	case AssurancePassCompleted, FindingPersisted, ReviewProcessCompleted:
		return class == ProducerReviewOrchestrator
	case DispositionRecorded:
		return class == ProducerMediumAdapter
	case RoutingEvidenceInvalidated:
		return class == ProducerEvidenceOwner
	case ActivationCheckpointEvent:
		return class == ProducerReleaseGate
	default:
		return false
	}
}

func validRole(kind EventKind, role string) bool {
	switch kind {
	case RoutingResolved, RoutingResolutionFailed:
		return oneOf(role, "implementation", "review")
	case DispositionRecorded:
		return oneOf(role, "worker", "operator")
	case RoutingEvidenceInvalidated:
		return oneOf(role, "rerun_required", "operator_remediation")
	default:
		return role == ""
	}
}

func knownProducerClass(class ProducerClass) bool {
	for _, kind := range allEventKinds() {
		if allowedProducer(kind, class) {
			return true
		}
	}
	return false
}

func knownAuthorizationKind(kind AuthorizationKind) bool {
	return kind == AuthorizationPolicy || kind == AuthorizationRuntimeFence || kind == AuthorizationNativeAuthor || kind == AuthorizationInvalidation || kind == AuthorizationActivationManifest
}

func allEventKinds() []EventKind {
	return []EventKind{ExecutionDemandFrozen, RoutingResolved, RoutingResolutionFailed, RoutedAttemptCompleted, RoutedTaskOutcome, AttemptOwnerTransferred, AssurancePassCompleted, FindingPersisted, DispositionRecorded, ReviewProcessCompleted, RoutingEvidenceInvalidated, ActivationCheckpointEvent}
}

func normalizedPolicy(policy ProducerPolicy) ProducerPolicy {
	policy.Keys = append([]KeyGeneration(nil), policy.Keys...)
	for i := range policy.Keys {
		policy.Keys[i].PolicyActivationPoints = append([]string(nil), policy.Keys[i].PolicyActivationPoints...)
		sort.Strings(policy.Keys[i].PolicyActivationPoints)
	}
	policy.Authorizations = append([]AuthorizationReference(nil), policy.Authorizations...)
	policy.Matrix = append([]ProducerAuthorization(nil), policy.Matrix...)
	sort.Slice(policy.Keys, func(i, j int) bool { return policy.Keys[i].KeyID < policy.Keys[j].KeyID })
	sort.Slice(policy.Authorizations, func(i, j int) bool { return policy.Authorizations[i].ID < policy.Authorizations[j].ID })
	sort.Slice(policy.Matrix, func(i, j int) bool {
		a, b := policy.Matrix[i], policy.Matrix[j]
		return fmt.Sprintf("%s\x00%s\x00%s\x00%s", a.EventKind, a.ProducerClass, a.ImplementationGeneration, a.Role) < fmt.Sprintf("%s\x00%s\x00%s\x00%s", b.EventKind, b.ProducerClass, b.ImplementationGeneration, b.Role)
	})
	return policy
}

func decodeHexExact(value string, size int) ([]byte, error) {
	b, err := hex.DecodeString(value)
	if err != nil || len(b) != size || hex.EncodeToString(b) != value {
		return nil, fmt.Errorf("expected %d-byte lowercase hex", size)
	}
	return b, nil
}

func containsKind(values []EventKind, want EventKind) bool {
	return slices.Contains(values, want)
}

func containsString(values []string, want string) bool {
	if want == "" && len(values) == 0 {
		return true
	}
	return slices.Contains(values, want)
}

func sortedUnique(values []string) bool {
	for i, value := range values {
		if value == "" || (i > 0 && values[i-1] >= value) {
			return false
		}
	}
	return true
}
