package routingevidence

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path"
)

func NewIdentityPreimage(e Event) (IdentityPreimage, error) {
	if err := validateEventBase(e, false, false); err != nil {
		return IdentityPreimage{}, err
	}
	payload, err := normalizedPayload(e)
	if err != nil {
		return IdentityPreimage{}, err
	}
	tuple, err := identityTuple(e.Kind, payload)
	if err != nil {
		return IdentityPreimage{}, err
	}
	return IdentityPreimage{
		Domain: EventIDDomain, EvidenceRef: e.EvidenceRef, EventKind: e.Kind,
		EventVersion: e.Version, Repository: e.Repository, Tuple: tuple,
	}, nil
}

func EncodeIdentityPreimage(e Event) ([]byte, error) {
	p, err := NewIdentityPreimage(e)
	if err != nil {
		return nil, err
	}
	return canonicalJSON(p)
}

func Identify(e Event) (EventID, error) {
	b, err := EncodeIdentityPreimage(e)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return EventID(hex.EncodeToString(sum[:])), nil
}

func NewContentPreimage(e Event) (ContentPreimage, error) {
	if err := validateEventBase(e, true, false); err != nil {
		return ContentPreimage{}, err
	}
	payload, err := normalizedPayload(e)
	if err != nil {
		return ContentPreimage{}, err
	}
	id, err := Identify(e)
	if err != nil {
		return ContentPreimage{}, err
	}
	if e.EventID != "" && e.EventID != id {
		return ContentPreimage{}, fmt.Errorf("event_id: supplied value does not match identity preimage")
	}
	union, err := unionForPayload(payload)
	if err != nil {
		return ContentPreimage{}, err
	}
	return ContentPreimage{
		Domain: CanonicalDomain, EventID: id, EvidenceRef: e.EvidenceRef,
		EventKind: e.Kind, EventVersion: e.Version, Payload: union,
		Producer: e.Producer, Repository: e.Repository, Schema: e.Schema,
	}, nil
}

func unionForPayload(payload any) (Payload, error) {
	var union Payload
	switch p := payload.(type) {
	case *ExecutionDemandFrozenPayload:
		union.ExecutionDemandFrozen = p

	case *RoutingResolvedPayload:
		union.RoutingResolved = p
	case *RoutingResolutionFailedPayload:
		union.RoutingResolutionFailed = p
	case *RoutedAttemptCompletedPayload:
		union.RoutedAttemptCompleted = p
	case *RoutedTaskOutcomePayload:
		union.RoutedTaskOutcome = p
	case *AttemptOwnerTransferredPayload:
		union.AttemptOwnerTransferred = p
	case *AssurancePassCompletedPayload:
		union.AssurancePassCompleted = p
	case *FindingPersistedPayload:
		union.FindingPersisted = p
	case *DispositionRecordedPayload:
		union.DispositionRecorded = p
	case *ReviewProcessCompletedPayload:
		union.ReviewProcessCompleted = p
	case *RoutingEvidenceInvalidatedPayload:
		union.RoutingEvidenceInvalidated = p
	case *ActivationCheckpointPayload:
		union.ActivationCheckpoint = p
	default:
		return Payload{}, fmt.Errorf("payload: unsupported member %T", payload)
	}
	return union, nil
}

func EncodeContentPreimage(e Event) ([]byte, error) {
	p, err := NewContentPreimage(e)
	if err != nil {
		return nil, err
	}
	return canonicalJSON(p)
}

func Digest(e Event) (string, error) {
	b, err := EncodeContentPreimage(e)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func NewUnsignedEnvelope(e Event) (UnsignedEnvelope, error) {
	content, err := NewContentPreimage(e)
	if err != nil {
		return UnsignedEnvelope{}, err
	}
	digest, err := Digest(e)
	if err != nil {
		return UnsignedEnvelope{}, err
	}
	if e.ContentDigest != "" && e.ContentDigest != digest {
		return UnsignedEnvelope{}, fmt.Errorf("content_digest: supplied value does not match content preimage")
	}
	return UnsignedEnvelope{
		ContentDigest: digest, Domain: content.Domain, EventID: content.EventID,
		EvidenceRef: content.EvidenceRef, EventKind: content.EventKind,
		EventVersion: content.EventVersion, Payload: content.Payload,
		Producer: content.Producer, Repository: content.Repository, Schema: content.Schema,
	}, nil
}

func EncodeUnsignedEnvelope(e Event) ([]byte, error) {
	u, err := NewUnsignedEnvelope(e)
	if err != nil {
		return nil, err
	}
	return canonicalJSON(u)
}

func NewEnvelope(e Event) (Envelope, error) {
	if err := validateEventBase(e, true, true); err != nil {
		return Envelope{}, err
	}
	u, err := NewUnsignedEnvelope(e)
	if err != nil {
		return Envelope{}, err
	}
	return Envelope{
		ContentDigest: u.ContentDigest, Domain: u.Domain, EventID: u.EventID,
		EvidenceRef: u.EvidenceRef, EventKind: u.EventKind,
		EventVersion: u.EventVersion, Payload: u.Payload, Producer: u.Producer,
		ProducerProof: e.ProducerProof, Repository: u.Repository, Schema: u.Schema,
	}, nil
}

// EncodeCanonical constructs all derived fields and emits the one canonical
// stored-file representation. Non-empty caller-supplied derived fields are
// assertions: a mismatch is rejected rather than silently repaired.
func EncodeCanonical(e Event) ([]byte, error) {
	envelope, err := NewEnvelope(e)
	if err != nil {
		return nil, err
	}
	return canonicalJSON(envelope)
}

type wireEnvelope struct {
	ContentDigest string          `json:"content_digest"`
	Domain        string          `json:"domain"`
	EventID       EventID         `json:"event_id"`
	EvidenceRef   string          `json:"evidence_ref"`
	EventKind     EventKind       `json:"event_kind"`
	EventVersion  string          `json:"event_version"`
	Payload       json.RawMessage `json:"payload"`
	Producer      Producer        `json:"producer"`
	ProducerProof ProducerProof   `json:"producer_proof"`
	Repository    string          `json:"repository"`
	Schema        string          `json:"schema"`
}

// DecodeCanonical accepts only bytes already in canonical form. Validation is
// all-or-nothing: callers receive the zero Event on every defect.
func DecodeCanonical(b []byte) (Event, error) {
	canonical, err := canonicalizeInput(b)
	if err != nil {
		return Event{}, err
	}
	if !bytes.Equal(canonical, b) {
		return Event{}, fmt.Errorf("canonical json: input is not the canonical representation")
	}
	var wire wireEnvelope
	if err := decodeClosed(b, &wire); err != nil {
		return Event{}, fmt.Errorf("decode envelope: %w", err)
	}
	if wire.Domain != CanonicalDomain {
		return Event{}, fmt.Errorf("domain: unsupported value %q", wire.Domain)
	}
	e := Event{
		ContentDigest: wire.ContentDigest, EventID: wire.EventID,
		EvidenceRef: wire.EvidenceRef, Kind: wire.EventKind, Producer: wire.Producer,
		ProducerProof: wire.ProducerProof, Repository: wire.Repository,
		Schema: wire.Schema, Version: wire.EventVersion,
	}
	if err := decodePayload(wire.EventKind, wire.Payload, &e.Payload); err != nil {
		return Event{}, err
	}
	reencoded, err := EncodeCanonical(e)
	if err != nil {
		return Event{}, err
	}
	if !bytes.Equal(reencoded, b) {
		return Event{}, fmt.Errorf("canonical json: decoded event does not reproduce input")
	}
	return e, nil
}

func decodePayload(kind EventKind, raw []byte, union *Payload) error {
	decode := func(dst any) error {
		if err := decodeClosed(raw, dst); err != nil {
			return fmt.Errorf("decode %s payload: %w", kind, err)
		}
		return nil
	}
	switch kind {
	case ExecutionDemandFrozen:
		var p ExecutionDemandFrozenPayload
		if err := decode(&p); err != nil {
			return err
		}
		union.ExecutionDemandFrozen = &p

	case RoutingResolved:
		var p RoutingResolvedPayload
		if err := decode(&p); err != nil {
			return err
		}
		union.RoutingResolved = &p
	case RoutingResolutionFailed:
		var p RoutingResolutionFailedPayload
		if err := decode(&p); err != nil {
			return err
		}
		union.RoutingResolutionFailed = &p
	case RoutedAttemptCompleted:
		var p RoutedAttemptCompletedPayload
		if err := decode(&p); err != nil {
			return err
		}
		union.RoutedAttemptCompleted = &p
	case RoutedTaskOutcome:
		var p RoutedTaskOutcomePayload
		if err := decode(&p); err != nil {
			return err
		}
		union.RoutedTaskOutcome = &p
	case AttemptOwnerTransferred:
		var p AttemptOwnerTransferredPayload
		if err := decode(&p); err != nil {
			return err
		}
		union.AttemptOwnerTransferred = &p
	case AssurancePassCompleted:
		var p AssurancePassCompletedPayload
		if err := decode(&p); err != nil {
			return err
		}
		union.AssurancePassCompleted = &p
	case FindingPersisted:
		var p FindingPersistedPayload
		if err := decode(&p); err != nil {
			return err
		}
		union.FindingPersisted = &p
	case DispositionRecorded:
		var p DispositionRecordedPayload
		if err := decode(&p); err != nil {
			return err
		}
		union.DispositionRecorded = &p
	case ReviewProcessCompleted:
		var p ReviewProcessCompletedPayload
		if err := decode(&p); err != nil {
			return err
		}
		union.ReviewProcessCompleted = &p
	case RoutingEvidenceInvalidated:
		var p RoutingEvidenceInvalidatedPayload
		if err := decode(&p); err != nil {
			return err
		}
		union.RoutingEvidenceInvalidated = &p
	case ActivationCheckpointEvent:
		var p ActivationCheckpointPayload
		if err := decode(&p); err != nil {
			return err
		}
		union.ActivationCheckpoint = &p
	default:
		return fmt.Errorf("event_kind: unknown kind %q", kind)
	}
	return nil
}

func identityTuple(kind EventKind, payload any) (IdentityTuple, error) {
	switch p := payload.(type) {
	case *ExecutionDemandFrozenPayload:
		return IdentityTuple{DemandID: p.DemandID}, nil

	case *RoutingResolvedPayload:
		return IdentityTuple{AttemptID: p.AttemptID}, nil
	case *RoutingResolutionFailedPayload:
		return IdentityTuple{AttemptID: p.AttemptID}, nil
	case *RoutedAttemptCompletedPayload:
		return IdentityTuple{AttemptID: p.AttemptID}, nil
	case *RoutedTaskOutcomePayload:
		return IdentityTuple{AttemptID: p.AttemptID}, nil
	case *AttemptOwnerTransferredPayload:
		return IdentityTuple{AttemptID: p.AttemptID, NewOwnerGeneration: p.NewOwnerGeneration}, nil
	case *AssurancePassCompletedPayload:
		return IdentityTuple{PassID: p.PassID, ProcessGeneration: p.ProcessGeneration, ReviewProcessID: p.ReviewProcessID}, nil
	case *FindingPersistedPayload:
		return IdentityTuple{FindingID: p.FindingID, PassID: p.PassID}, nil
	case *DispositionRecordedPayload:
		return IdentityTuple{FindingID: p.FindingID, NativeMediumActRef: p.NativeMediumActRef}, nil
	case *ReviewProcessCompletedPayload:
		return IdentityTuple{ProcessGeneration: p.ProcessGeneration, ReviewProcessID: p.ReviewProcessID}, nil
	case *RoutingEvidenceInvalidatedPayload:
		return IdentityTuple{InvalidationID: p.InvalidationID}, nil
	case *ActivationCheckpointPayload:
		return IdentityTuple{ActivationPolicyVersion: p.ActivationPolicyVersion, ExactMainlineObject: p.ExactMainlineObject, ProofManifestDigest: p.ProofManifestDigest}, nil
	default:
		return IdentityTuple{}, fmt.Errorf("event_kind: %s has no identity tuple for %T", kind, payload)
	}
}

// EventPath is a portable slash-separated immutable path derived solely from a
// lowercase SHA-256 event identity. Ref names, hosts, clocks, and git metadata
// cannot alter it.
func EventPath(id EventID) (string, error) {
	text := string(id)
	if len(text) != sha256.Size*2 {
		return "", fmt.Errorf("event_id: expected %d lowercase hex characters", sha256.Size*2)
	}
	decoded, err := hex.DecodeString(text)
	if err != nil || hex.EncodeToString(decoded) != text {
		return "", fmt.Errorf("event_id: expected lowercase SHA-256 hex")
	}
	return path.Join("events", text[:2], text[2:4], text+".json"), nil
}
