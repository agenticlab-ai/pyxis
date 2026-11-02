package featuredrive

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
)

const PhaseObserved = "phase-observed"

// PhaseGoalVerifier re-reads the goal object of an already observed effect.
// Implementations use the task-landing, candidate, scenario, final-review and
// delivery predicates; a cached observation or notification is insufficient.
type PhaseGoalVerifier interface {
	VerifyPhaseGoal(context.Context, FeatureDrive, EffectIntent) (EvidenceRef, error)
}

type PhaseLifecycle struct {
	Store FeatureDriveStore
	Goals PhaseGoalVerifier
}

// Advance derives the destination from a completed lifecycle effect, rather
// than accepting a caller-selected phase. The CAS fences both observation and
// persistence against owner, epoch and aggregate movement.
func (l PhaseLifecycle) Advance(ctx context.Context, featureID string, owner OwnerEpoch, effectID string) (FeatureDrive, error) {
	if l.Store == nil || l.Goals == nil {
		return FeatureDrive{}, ErrMalformed
	}
	d, err := l.Store.Load(ctx, featureID)
	if err != nil {
		return d, err
	}
	if d.Ownership.State != Active || d.Ownership.Domain != owner.Owner.Domain || d.Ownership.OwnerID != owner.Owner.ID || d.Ownership.Epoch != owner.Epoch {
		return d, ErrStaleEpoch
	}
	next, effect, err := phaseStep(d, effectID)
	if err != nil {
		return d, err
	}
	verified, err := l.Goals.VerifyPhaseGoal(ctx, d, effect)
	if err != nil {
		return d, fmt.Errorf("verify phase goal %s: %w", effectID, err)
	}
	if verified.Validate() != nil || verified != *effect.Observation {
		return d, fmt.Errorf("%w: phase goal changed", ErrMalformed)
	}
	if err := ctx.Err(); err != nil {
		return d, err
	}
	if err := l.Store.CAS(ctx, featureID, d.Version, next); err != nil {
		return d, err
	}
	return next, nil
}

// ValidatePhaseTransition is shared by in-memory persistence and signed Git
// replay. Only phase, its resolved wait, one evidence append, and the terminal
// delivery release may change. Actor/signature verification belongs to the
// authenticated store, and cannot change the canonical state delta.
func ValidatePhaseTransition(prior, next FeatureDrive) error {
	if next.Last.Kind != PhaseObserved || len(next.Evidence) != len(prior.Evidence)+1 {
		return ErrMalformed
	}
	goal := next.Evidence[len(prior.Evidence)]
	expected, _, err := phaseStep(prior, goal.ID)
	if err != nil {
		return err
	}
	expected.Last.Actor, expected.Last.Signature = next.Last.Actor, next.Last.Signature
	if !reflect.DeepEqual(expected, next) {
		return ErrMalformed
	}
	return nil
}

func phaseStep(prior FeatureDrive, effectID string) (FeatureDrive, EffectIntent, error) {
	if prior.Validate() != nil || prior.Ownership.State != Active || prior.Blocker != nil || prior.Failure != nil || effectID == "" {
		return prior, EffectIntent{}, ErrMalformed
	}
	var goal EffectIntent
	for _, effect := range prior.Effects {
		if effect.Status == Pending {
			return prior, goal, fmt.Errorf("%w: phase has pending effects", ErrIneligible)
		}
		if effect.ID == effectID {
			goal = effect
		}
	}
	if goal.Status != Observed || goal.RequiredEpoch != prior.Ownership.Epoch || goal.Observation == nil || goal.Observation.Validate() != nil {
		return prior, goal, ErrMalformed
	}
	for _, evidence := range prior.Evidence {
		if evidence.Kind == "phase-effect" && evidence.ID == effectID {
			return prior, goal, fmt.Errorf("%w: phase goal was already consumed", ErrIneligible)
		}
	}
	phase, kind := phaseTarget(prior.Phase, goal.Kind)
	if phase == "" || goal.Observation.Kind != kind {
		return prior, goal, ErrMalformed
	}
	if prior.Wait != nil && (prior.Wait.Goal.Kind != goal.Observation.Kind || prior.Wait.Goal.ID != goal.Observation.ID ||
		(prior.Wait.Goal.Digest != "" && prior.Wait.Goal.Digest != goal.Observation.Digest)) {
		return prior, goal, fmt.Errorf("%w: phase goal differs from the current wait", ErrMalformed)
	}
	next := prior
	next.Phase, next.Wait = phase, nil
	next.Evidence = append(slices.Clone(prior.Evidence), phaseEvidence(goal))
	if phase == Delivered {
		next.Ownership = Ownership{State: Released, Epoch: prior.Ownership.Epoch + 1}
	}
	next.Version++
	next.Last = Transition{Kind: PhaseObserved, PriorVersion: prior.Version, NewVersion: next.Version, PriorEpoch: prior.Ownership.Epoch, NewEpoch: next.Ownership.Epoch}
	if err := next.Validate(); err != nil || next.Version == 0 || next.Ownership.Epoch == 0 {
		return prior, goal, ErrMalformed
	}
	return next, goal, nil
}

func phaseEvidence(effect EffectIntent) EvidenceRef {
	body, _ := json.Marshal(effect.Observation)
	return EvidenceRef{Kind: "phase-effect", ID: effect.ID, Digest: fmt.Sprintf("sha256:%x", sha256.Sum256(body))}
}

func phaseTarget(phase Phase, effect string) (Phase, string) {
	switch phase {
	case Integrating, Repairing:
		if effect == "tasks-integrated" {
			return Candidate, "task-landings"
		}
	case Candidate:
		switch effect {
		case "candidate-published":
			return Verifying, "candidate"
		case "candidate-conflict-repair-created":
			return Repairing, "conflict-repair"
		}
	case Verifying:
		switch effect {
		case "verification-repair-created":
			return Repairing, "verification-repair"
		case "final-proposal-activated":
			return FinalReview, "final-proposal"
		case "candidate-invalidated":
			return Candidate, "candidate-stale"
		}
	case FinalReview:
		switch effect {
		case "final-feedback-repair-created":
			return Repairing, "review-repair"
		case "candidate-invalidated":
			return Candidate, "candidate-stale"
		case "final-landing-verified":
			return DeliveryVerifying, "final-landing"
		}
	case DeliveryVerifying:
		if effect == "delivery-routing-released" {
			return Delivered, "delivery"
		}
	}
	return "", ""
}
