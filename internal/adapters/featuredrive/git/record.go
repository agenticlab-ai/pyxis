package gitdrive

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"

	"github.com/agenticlab-ai/pyxis/internal/featuredrive"
)

const RecordSchema = "feature-drive-transition-record/v1"

// The envelope signs the complete aggregate and exact predecessor object, in
// addition to the core transition signature. Copying a transition signature
// onto altered state cannot authenticate the resulting record.
type Record struct {
	SchemaVersion   string                    `json:"schema_version"`
	Ref             string                    `json:"ref"`
	Authorization   string                    `json:"authorization"`
	Parent          string                    `json:"parent"`
	PreviousDigest  string                    `json:"previous_digest,omitempty"`
	AggregateDigest string                    `json:"aggregate_digest"`
	Actor           string                    `json:"actor"`
	Drive           featuredrive.FeatureDrive `json:"drive"`
	Signature       string                    `json:"signature"`
}

func (r Record) canonical() ([]byte, error) { return json.Marshal(r) }

func (r Record) signingBytes() ([]byte, error) {
	r.Signature = ""
	body, err := r.canonical()
	return append([]byte(RecordSchema+"\x00"), body...), err
}

func decodeRecord(body []byte) (Record, error) {
	var record Record
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&record); err != nil {
		return record, fmt.Errorf("%w: record: %v", featuredrive.ErrMalformed, err)
	}
	canonical, err := record.canonical()
	if err != nil || !bytes.Equal(body, canonical) || record.SchemaVersion != RecordSchema || record.Drive.Validate() != nil {
		return record, featuredrive.ErrMalformed
	}
	return record, nil
}

func aggregateDigest(drive featuredrive.FeatureDrive) (string, error) {
	body, err := json.Marshal(drive)
	return contentDigest(body), err
}

func contentDigest(body []byte) string { return fmt.Sprintf("sha256:%x", sha256.Sum256(body)) }

func objectID(oid string) bool {
	body, err := hex.DecodeString(oid)
	return err == nil && (len(body) == 20 || len(body) == 32) && hex.EncodeToString(body) == oid
}

func verifySignature(key ed25519.PublicKey, body []byte, encoded string) bool {
	signature, err := hex.DecodeString(encoded)
	return err == nil && len(signature) == ed25519.SignatureSize && hex.EncodeToString(signature) == encoded && ed25519.Verify(key, body, signature)
}

func (s *Store) sign(ctx context.Context, body []byte) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	signature, err := s.signer.Sign(ctx, bytes.Clone(body))
	if err != nil {
		return "", fmt.Errorf("%w: sign: %w", ErrUnauthorized, err)
	}
	encoded := hex.EncodeToString(signature)
	if !verifySignature(s.principals[s.signer.ID].PublicKey, body, encoded) {
		return "", ErrUnauthorized
	}
	return encoded, ctx.Err()
}

func (s *Store) signTransition(ctx context.Context, transition featuredrive.Transition) (featuredrive.Transition, error) {
	if transition.Signature != "" {
		if transition.Actor != s.signer.ID || !featuredrive.VerifyTransition(transition, s.principals[s.signer.ID].PublicKey) {
			return transition, ErrUnauthorized
		}
		return transition, nil
	}
	// Unsigned core transitions receive the actual registered signer's
	// attribution. Payload-supplied actor text never authenticates a write.
	transition.Actor = s.signer.ID
	body, err := json.Marshal(transition)
	if err != nil {
		return transition, err
	}
	transition.Signature, err = s.sign(ctx, body)
	return transition, err
}

func (s *Store) authorize(principal Principal, prior *featuredrive.FeatureDrive, next featuredrive.FeatureDrive) error {
	if next.Validate() != nil || next.FeatureID != s.bound.FeatureID || next.Repository != s.bound.Repository {
		return featuredrive.ErrMalformed
	}
	if (next.Ownership.State == featuredrive.Active || next.Ownership.State == featuredrive.Quiescing) &&
		!s.owners[featuredrive.Owner{Domain: next.Ownership.Domain, ID: next.Ownership.OwnerID}] {
		return ErrUnauthorized
	}
	if prior == nil {
		if !principal.Operator {
			return ErrUnauthorized
		}
		if next.Version != 1 || next.Last.Kind != "create" || next.Last.PriorVersion != 0 || next.Last.PriorEpoch != 0 || next.Last.NewEpoch != 0 || next.Ownership != (featuredrive.Ownership{State: featuredrive.Unowned}) || next.Phase != featuredrive.Integrating ||
			len(next.Effects) != 0 || len(next.Frontier) != 0 || next.Wait != nil || next.Blocker != nil || len(next.Evidence) != 0 || next.Failure != nil || len(next.SecondaryFailures) != 0 {
			return featuredrive.ErrMalformed
		}
		return nil
	}
	if next.Version != prior.Version+1 || next.Version == 0 || next.Last.PriorVersion != prior.Version || next.Last.PriorEpoch != prior.Ownership.Epoch || next.Last.NewEpoch != next.Ownership.Epoch || !reflect.DeepEqual(next.Created, prior.Created) || next.PlanFingerprint != prior.PlanFingerprint || next.TargetMainRef != prior.TargetMainRef || next.IntegrationRef != prior.IntegrationRef || next.Ownership.Epoch < prior.Ownership.Epoch {
		return featuredrive.ErrMalformed
	}
	if err := validateOwnershipStep(*prior, next); err != nil {
		return err
	}
	if principal.Operator {
		// Human ownership decisions do not confer execution authority. Task,
		// proposal and frontier transitions require the current owner's key.
		switch next.Last.Kind {
		case "ownership-assigned", "handoff-quiescing", "handoff-blocked", "cancel-pending", "cancel-blocked", "ownership-changed":
			return nil
		default:
			return ErrUnauthorized
		}
	}
	if (prior.Ownership.State != featuredrive.Active && prior.Ownership.State != featuredrive.Quiescing) || prior.Ownership.Domain != principal.Owner.Domain || prior.Ownership.OwnerID != principal.Owner.ID {
		return ErrUnauthorized
	}
	// Ownership assignment and human requests remain operator-authenticated.
	// The current owner may only record effects or a blocked quiescence result.
	switch next.Last.Kind {
	case "effect-intent", "effect-observed", "frontier-observed", "handoff-blocked", "cancel-blocked", featuredrive.PhaseObserved:
	default:
		return ErrUnauthorized
	}
	if next.Ownership != prior.Ownership && !deliveredRelease(*prior, next) && next.Last.Kind != featuredrive.PhaseObserved {
		return ErrUnauthorized
	}
	return nil
}

func deliveredRelease(prior, next featuredrive.FeatureDrive) bool {
	if prior.Phase != featuredrive.DeliveryVerifying || next.Phase != featuredrive.Delivered || next.Last.Kind != "effect-observed" ||
		prior.Ownership.State != featuredrive.Active || next.Ownership != (featuredrive.Ownership{State: featuredrive.Released, Epoch: prior.Ownership.Epoch + 1}) || next.Ownership.Epoch == 0 || len(prior.Effects) != len(next.Effects) {
		return false
	}
	for i, effect := range prior.Effects {
		observed := next.Effects[i]
		if effect.Status == featuredrive.Pending && observed.Status == featuredrive.Observed && effect.Kind == "delivery-routing-released" &&
			effect.RequiredEpoch == prior.Ownership.Epoch && observed.Observation != nil && observed.Observation.Kind == "delivery" && observed.Observation.Validate() == nil {
			return true
		}
	}
	return false
}

func validateOwnershipStep(prior, next featuredrive.FeatureDrive) error {
	oneEpoch := next.Ownership.Epoch == prior.Ownership.Epoch+1 && next.Ownership.Epoch != 0
	sameOwner := next.Ownership.Domain == prior.Ownership.Domain && next.Ownership.OwnerID == prior.Ownership.OwnerID && next.Ownership.Epoch == prior.Ownership.Epoch
	retained := next
	retained.Version, retained.Last = prior.Version, prior.Last
	valid := false
	switch next.Last.Kind {
	case featuredrive.PhaseObserved:
		return featuredrive.ValidatePhaseTransition(prior, next)
	case "ownership-assigned":
		retained.Ownership = prior.Ownership
		valid = (prior.Ownership.State == featuredrive.Unowned || prior.Ownership.State == featuredrive.Released) && next.Ownership.State == featuredrive.Active && oneEpoch && prior.Phase == featuredrive.Integrating && reflect.DeepEqual(retained, prior)
	case "handoff-quiescing":
		retained.Ownership = prior.Ownership
		valid = prior.Ownership.State == featuredrive.Active && next.Ownership.State == featuredrive.Quiescing && sameOwner && reflect.DeepEqual(retained, prior)
	case "handoff-blocked":
		retained.Blocker = prior.Blocker
		valid = prior.Ownership.State == featuredrive.Quiescing && reflect.DeepEqual(retained, prior)
	case "cancel-pending":
		retained.Phase = prior.Phase
		valid = prior.Ownership.State == featuredrive.Active && next.Phase == featuredrive.CancelPending && reflect.DeepEqual(retained, prior)
	case "cancel-blocked":
		retained.Blocker = prior.Blocker
		valid = prior.Phase == featuredrive.CancelPending && reflect.DeepEqual(retained, prior)
	case "ownership-changed":
		switch {
		case prior.Ownership.State == featuredrive.Quiescing:
			retained.Ownership, retained.Evidence, retained.Blocker = prior.Ownership, prior.Evidence, prior.Blocker
			valid = next.Ownership.State == featuredrive.Active && oneEpoch && next.Blocker == nil && appendsEvidence(prior, next) && reflect.DeepEqual(retained, prior)
		case prior.Phase == featuredrive.CancelPending:
			retained.Phase, retained.Ownership, retained.Evidence, retained.Blocker = prior.Phase, prior.Ownership, prior.Evidence, prior.Blocker
			valid = next.Phase == featuredrive.Cancelled && next.Blocker == nil && next.Ownership == (featuredrive.Ownership{State: featuredrive.Released, Epoch: prior.Ownership.Epoch + 1}) && oneEpoch && appendsEvidence(prior, next) && reflect.DeepEqual(retained, prior)
		case prior.Phase == featuredrive.Cancelled:
			retained.Phase = prior.Phase
			valid = (next.Phase == featuredrive.Integrating || next.Phase == featuredrive.Abandoned) && reflect.DeepEqual(retained, prior)
		}
	case "effect-intent":
		valid = executable(prior) && appendsPendingEffects(prior, next)
	case "effect-observed":
		valid = completesPendingEffects(prior, next)
		if deliveredRelease(prior, next) {
			// Delivery may release ownership, but still requires an existing
			// pending observation and cannot rewrite any other aggregate fact.
			retained := next
			retained.Phase, retained.Ownership = prior.Phase, prior.Ownership
			valid = completesPendingEffects(prior, retained)
		}
	case "frontier-observed":
		retained := next
		retained.Version, retained.Last, retained.Frontier = prior.Version, prior.Last, prior.Frontier
		valid = executable(prior) && reflect.DeepEqual(retained, prior)
	}
	if !valid {
		return featuredrive.ErrMalformed
	}
	return nil
}

func appendsEvidence(prior, next featuredrive.FeatureDrive) bool {
	return len(next.Evidence) == len(prior.Evidence)+1 && slices.Equal(prior.Evidence, next.Evidence[:len(prior.Evidence)]) &&
		next.Evidence[len(prior.Evidence)].Validate() == nil
}

// Planning may append new pending work, while retaining every previously
// recorded intent, completion and lifecycle fact byte-for-byte.
func appendsPendingEffects(prior, next featuredrive.FeatureDrive) bool {
	if len(next.Effects) <= len(prior.Effects) {
		return false
	}
	retained := next
	retained.Version, retained.Last, retained.Effects = prior.Version, prior.Last, prior.Effects
	if !reflect.DeepEqual(retained, prior) {
		return false
	}
	for i, effect := range prior.Effects {
		if !reflect.DeepEqual(effect, next.Effects[i]) {
			return false
		}
	}
	for _, effect := range next.Effects[len(prior.Effects):] {
		if effect.Status != featuredrive.Pending || effect.Observation != nil {
			return false
		}
	}
	return true
}

func executable(drive featuredrive.FeatureDrive) bool {
	if drive.Ownership.State != featuredrive.Active {
		return false
	}
	switch drive.Phase {
	case featuredrive.Integrating, featuredrive.Candidate, featuredrive.Verifying, featuredrive.Repairing, featuredrive.FinalReview, featuredrive.DeliveryVerifying:
		return true
	default:
		return false
	}
}

// An effect observation completes existing pending work. Both active and
// revoked routing forbid fresh effects, changed inputs, or unrelated state.
func completesPendingEffects(prior, next featuredrive.FeatureDrive) bool {
	if len(prior.Effects) != len(next.Effects) {
		return false
	}
	retained := next
	retained.Version, retained.Last, retained.Effects = prior.Version, prior.Last, prior.Effects
	if !reflect.DeepEqual(retained, prior) {
		return false
	}
	completed := false
	for i, effect := range prior.Effects {
		observed := next.Effects[i]
		if reflect.DeepEqual(effect, observed) {
			continue
		}
		if effect.Status != featuredrive.Pending || observed.Status != featuredrive.Observed || observed.Observation == nil || observed.Observation.Validate() != nil {
			return false
		}
		observed.Status, observed.Observation = effect.Status, effect.Observation
		if !reflect.DeepEqual(effect, observed) {
			return false
		}
		completed = true
	}
	return completed
}
