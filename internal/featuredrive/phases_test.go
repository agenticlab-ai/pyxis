package featuredrive

import (
	"context"
	"errors"
	"math"
	"reflect"
	"testing"
)

type phaseVerifierFunc func(context.Context, FeatureDrive, EffectIntent) (EvidenceRef, error)

func (f phaseVerifierFunc) VerifyPhaseGoal(ctx context.Context, d FeatureDrive, e EffectIntent) (EvidenceRef, error) {
	return f(ctx, d, e)
}

func phasePrior(phase Phase, kind, goalKind string) FeatureDrive {
	created := Transition{Kind: "create", NewVersion: 1}
	return FeatureDrive{Schema: Schema, Repository: "repo", FeatureID: "feature", PlanFingerprint: "frozen-plan", TargetMainRef: "refs/heads/main", IntegrationRef: "refs/heads/feature", Version: 4, Created: created,
		Last:      Transition{Kind: "effect-observed", PriorVersion: 3, NewVersion: 4, PriorEpoch: 1, NewEpoch: 1},
		Ownership: Ownership{State: Active, Domain: "local", OwnerID: "owner", Epoch: 1}, Phase: phase,
		Wait:     &Wait{Kind: "goal", Goal: EvidenceRef{Kind: goalKind, ID: "goal-object"}},
		Evidence: []EvidenceRef{{Kind: "plan", ID: "frozen-plan"}},
		Effects:  []EffectIntent{{ID: "goal", Kind: kind, RequiredEpoch: 1, Status: Observed, Observation: &EvidenceRef{Kind: goalKind, ID: "goal-object", Digest: "sha256:goal"}}}}
}

func TestPhaseLifecycleUsesClosedGoalsAndPreservesHistory(t *testing.T) {
	for _, tc := range []struct {
		from, to   Phase
		kind, goal string
	}{
		{Integrating, Candidate, "tasks-integrated", "task-landings"},
		{Repairing, Candidate, "tasks-integrated", "task-landings"},
		{Candidate, Verifying, "candidate-published", "candidate"},
		{Candidate, Repairing, "candidate-conflict-repair-created", "conflict-repair"},
		{Verifying, Repairing, "verification-repair-created", "verification-repair"},
		{Verifying, FinalReview, "final-proposal-activated", "final-proposal"},
		{Verifying, Candidate, "candidate-invalidated", "candidate-stale"},
		{FinalReview, Repairing, "final-feedback-repair-created", "review-repair"},
		{FinalReview, Candidate, "candidate-invalidated", "candidate-stale"},
		{FinalReview, DeliveryVerifying, "final-landing-verified", "final-landing"},
		{DeliveryVerifying, Delivered, "delivery-routing-released", "delivery"},
	} {
		t.Run(string(tc.from)+"/"+tc.kind, func(t *testing.T) {
			prior := phasePrior(tc.from, tc.kind, tc.goal)
			store := NewMemoryStore()
			if err := store.Create(t.Context(), prior); err != nil {
				t.Fatal(err)
			}
			reads := 0
			lifecycle := PhaseLifecycle{Store: store, Goals: phaseVerifierFunc(func(_ context.Context, d FeatureDrive, e EffectIntent) (EvidenceRef, error) {
				reads++
				if !reflect.DeepEqual(d, prior) {
					t.Fatal("verified stale aggregate")
				}
				return *e.Observation, nil
			})}
			next, err := lifecycle.Advance(t.Context(), prior.FeatureID, OwnerEpoch{Owner: Owner{Domain: "local", ID: "owner"}, Epoch: 1}, "goal")
			if err != nil || next.Phase != tc.to || next.Wait != nil || reads != 1 || ValidatePhaseTransition(prior, next) != nil {
				t.Fatalf("phase failed: %+v %v", next, err)
			}
			if !reflect.DeepEqual(next.Effects, prior.Effects) || next.Evidence[0] != prior.Evidence[0] || len(prior.Evidence) != 1 {
				t.Fatal("phase rewrote shared history")
			}
			if tc.to == Delivered && next.Ownership != (Ownership{State: Released, Epoch: 2}) {
				t.Fatal("delivery did not release exactly once")
			}
		})
	}
}

func TestPhaseLifecycleRefusesMissingStaleAndUnverifiedGoals(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*FeatureDrive)
	}{
		{"invalid schema", func(d *FeatureDrive) { d.Schema = "unknown" }},
		{"blocked", func(d *FeatureDrive) { d.Blocker = &EvidenceRef{Kind: "blocker", ID: "blocked"} }},
		{"failed", func(d *FeatureDrive) { d.Failure = &FailureDisposition{Trigger: "failure"} }},
		{"missing goal", func(d *FeatureDrive) { d.Effects = nil }},
		{"pending", func(d *FeatureDrive) { d.Effects[0].Status = Pending }},
		{"prior epoch", func(d *FeatureDrive) { d.Ownership.Epoch = 2 }},
		{"wrong effect", func(d *FeatureDrive) { d.Effects[0].Kind = "dispatch" }},
		{"wrong observation", func(d *FeatureDrive) { d.Effects[0].Observation.Kind = "proposal" }},
		{"replacement wait kind", func(d *FeatureDrive) { d.Wait.Goal.Kind = "candidate" }},
		{"replacement wait object", func(d *FeatureDrive) { d.Wait.Goal.ID = "replacement-goal" }},
		{"replacement wait digest", func(d *FeatureDrive) { d.Wait.Goal.Digest = "sha256:replacement" }},
		{"consumed", func(d *FeatureDrive) { d.Evidence = append(d.Evidence, phaseEvidence(d.Effects[0])) }},
		{"terminal", func(d *FeatureDrive) { d.Phase = Delivered }},
		{"version overflow", func(d *FeatureDrive) {
			d.Version = math.MaxUint64
			d.Last.NewVersion = d.Version
			d.Last.PriorVersion = d.Version - 1
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			prior := phasePrior(Integrating, "tasks-integrated", "task-landings")
			tc.edit(&prior)
			if _, _, err := phaseStep(prior, "goal"); err == nil {
				t.Fatal("invalid goal advanced the phase")
			}
		})
	}
	prior := phasePrior(DeliveryVerifying, "delivery-routing-released", "delivery")
	prior.Ownership.Epoch, prior.Effects[0].RequiredEpoch = math.MaxUint64, math.MaxUint64
	if _, _, err := phaseStep(prior, "goal"); err == nil {
		t.Fatal("epoch overflow released ownership")
	}
	if _, _, err := phaseStep(phasePrior(Integrating, "tasks-integrated", "task-landings"), ""); err == nil {
		t.Fatal("empty goal advanced phase")
	}
}

func TestPhaseLifecycleFencesFreshObservation(t *testing.T) {
	for _, failure := range []string{"verification", "different goal", "empty goal", "cancelled", "concurrent writer", "stale owner", "stale domain", "stale epoch", "missing store", "missing verifier", "missing drive"} {
		t.Run(failure, func(t *testing.T) {
			prior := phasePrior(Integrating, "tasks-integrated", "task-landings")
			store := NewMemoryStore()
			if err := store.Create(t.Context(), prior); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			cause := errors.New("goal unavailable")
			lifecycle := PhaseLifecycle{Store: store, Goals: phaseVerifierFunc(func(_ context.Context, _ FeatureDrive, e EffectIntent) (EvidenceRef, error) {
				ref := *e.Observation
				switch failure {
				case "verification":
					return ref, cause
				case "different goal":
					ref.ID = "moved"
				case "empty goal":
					ref = EvidenceRef{}
				case "cancelled":
					cancel()
				case "concurrent writer":
					winner := prior
					winner.Version++
					winner.Frontier = []string{"new-frontier"}
					winner.Last = Transition{Kind: "frontier-observed", PriorVersion: prior.Version, NewVersion: winner.Version, PriorEpoch: 1, NewEpoch: 1}
					if err := store.CAS(t.Context(), prior.FeatureID, prior.Version, winner); err != nil {
						t.Fatal(err)
					}
				}
				return ref, nil
			})}
			owner := OwnerEpoch{Owner: Owner{Domain: "local", ID: "owner"}, Epoch: 1}
			id := prior.FeatureID
			switch failure {
			case "stale owner":
				owner.Owner.ID = "other"
			case "stale domain":
				owner.Owner.Domain = "remote"
			case "stale epoch":
				owner.Epoch++
			case "missing store":
				lifecycle.Store = nil
			case "missing verifier":
				lifecycle.Goals = nil
			case "missing drive":
				id = "missing"
			}
			if _, err := lifecycle.Advance(ctx, id, owner, "goal"); err == nil {
				t.Fatal("refused goal advanced phase")
			}
			after, err := store.Load(t.Context(), prior.FeatureID)
			if err != nil || after.Phase != prior.Phase || !reflect.DeepEqual(after.Evidence, prior.Evidence) {
				t.Fatal("refused phase changed authority")
			}
		})
	}
}

func TestPhaseTransitionRefusesUnrelatedDeltasInMemory(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*FeatureDrive)
	}{
		{"unknown kind", func(d *FeatureDrive) { d.Last.Kind = "invented" }},
		{"skipped phase", func(d *FeatureDrive) { d.Phase = FinalReview }},
		{"removed proof", func(d *FeatureDrive) { d.Evidence = d.Evidence[:1] }},
		{"altered proof", func(d *FeatureDrive) { d.Evidence[1].Digest = "invented" }},
		{"altered history", func(d *FeatureDrive) { d.Evidence[0].ID = "invented" }},
		{"owner release", func(d *FeatureDrive) { d.Ownership = Ownership{State: Released, Epoch: 2} }},
		{"plan", func(d *FeatureDrive) { d.PlanFingerprint = "other-plan" }},
		{"frontier", func(d *FeatureDrive) { d.Frontier = []string{"invented"} }},
		{"effects", func(d *FeatureDrive) { d.Effects = nil }},
		{"epoch", func(d *FeatureDrive) { d.Last.NewEpoch++ }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			prior := phasePrior(Integrating, "tasks-integrated", "task-landings")
			next, _, err := phaseStep(prior, "goal")
			if err != nil {
				t.Fatal(err)
			}
			tc.edit(&next)
			if ValidatePhaseTransition(prior, next) == nil {
				t.Fatal("phase changed an unrelated fact")
			}
			if next.Last.Kind == PhaseObserved {
				store := NewMemoryStore()
				if err := store.Create(t.Context(), prior); err != nil {
					t.Fatal(err)
				}
				if err := store.CAS(t.Context(), prior.FeatureID, prior.Version, next); err == nil {
					t.Fatal("memory accepted malformed phase")
				}
			}
		})
	}
}
