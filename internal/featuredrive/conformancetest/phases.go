package conformancetest

import (
	"context"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/featuredrive"
)

type phaseGoalFunc func(context.Context, featuredrive.FeatureDrive, featuredrive.EffectIntent) (featuredrive.EvidenceRef, error)

func (f phaseGoalFunc) VerifyPhaseGoal(ctx context.Context, d featuredrive.FeatureDrive, e featuredrive.EffectIntent) (featuredrive.EvidenceRef, error) {
	return f(ctx, d, e)
}

// runPhaseJourney persists every boundary from the canonical creation state.
// Fixture ports supply goal objects; the real core predicates authorize each
// goal. This proves store behavior, not an installed runner or live model.
func runPhaseJourney(t *testing.T, f StoreFixture) {
	t.Helper()
	execution := f.ExecutionStore
	if execution == nil {
		execution = f.Store
	}
	load := func() featuredrive.FeatureDrive {
		d, err := f.Reopen(t).Load(t.Context(), f.Initial.FeatureID)
		expect(t, err == nil, "reopen phase aggregate: %v", err)
		return d
	}
	if err := f.Store.Create(t.Context(), f.Initial); err != nil {
		t.Fatal(err)
	}
	d := load()
	next := d
	next.Version++
	next.Ownership = featuredrive.Ownership{State: featuredrive.Active, Domain: f.Owner.Domain, OwnerID: f.Owner.ID, Epoch: 1}
	next.Last = featuredrive.Transition{Kind: "ownership-assigned", PriorVersion: d.Version, NewVersion: next.Version, NewEpoch: 1}
	expect(t, f.Store.CAS(t.Context(), d.FeatureID, d.Version, next) == nil, "assign journey owner")
	owner := featuredrive.OwnerEpoch{Owner: f.Owner, Epoch: 1}
	advance := func(kind, goalKind, goalID string, destination featuredrive.Phase, produce, verify func() error) {
		t.Helper()
		d := load()
		if f.ReopenExecution != nil {
			execution = f.ReopenExecution(t)
		}
		effect := featuredrive.EffectIntent{Kind: kind, Subject: goalID, InputDigest: goalID, RequiredEpoch: owner.Epoch, Status: featuredrive.Pending}
		effect.ID = featuredrive.EffectID(d.Repository, d.FeatureID, owner.Epoch, kind, goalID, effect.InputDigest)
		intent := d
		intent.Version++
		intent.Last = featuredrive.Transition{Kind: "effect-intent", PriorVersion: d.Version, NewVersion: intent.Version, PriorEpoch: owner.Epoch, NewEpoch: owner.Epoch}
		intent.Effects = append(slices.Clone(d.Effects), effect)
		expect(t, execution.CAS(t.Context(), d.FeatureID, d.Version, intent) == nil, "persist %s intent", kind)
		d = load()
		expect(t, produce() == nil, "produce %s goal", kind)
		ref := featuredrive.EvidenceRef{Kind: goalKind, ID: goalID, Digest: goalID}
		observed := d
		observed.Version++
		observed.Last = featuredrive.Transition{Kind: "effect-observed", PriorVersion: d.Version, NewVersion: observed.Version, PriorEpoch: owner.Epoch, NewEpoch: owner.Epoch}
		observed.Effects = slices.Clone(d.Effects)
		observed.Effects[len(observed.Effects)-1].Status = featuredrive.Observed
		observed.Effects[len(observed.Effects)-1].Observation = &ref
		expect(t, execution.CAS(t.Context(), d.FeatureID, d.Version, observed) == nil, "persist %s observation", kind)
		before := load()
		if f.ReopenExecution != nil {
			execution = f.ReopenExecution(t)
		}
		reads := 0
		lifecycle := featuredrive.PhaseLifecycle{Store: execution, Goals: phaseGoalFunc(func(_ context.Context, current featuredrive.FeatureDrive, observed featuredrive.EffectIntent) (featuredrive.EvidenceRef, error) {
			reads++
			expect(t, current.Version == before.Version && observed.ID == effect.ID, "phase did not verify its current goal")
			return ref, verify()
		})}
		_, err := lifecycle.Advance(t.Context(), d.FeatureID, owner, effect.ID)
		expect(t, err == nil, "advance %s to %s: %v", kind, destination, err)
		after := load()
		expect(t, reads == 1 && after.Phase == destination && after.Version == before.Version+1, "phase was not persisted: %+v", after)
		expect(t, reflect.DeepEqual(after.Created, before.Created) && reflect.DeepEqual(after.Effects, before.Effects) && after.PlanFingerprint == before.PlanFingerprint, "phase rewrote lineage or goal history")
		if destination == featuredrive.Delivered {
			expect(t, after.Ownership == (featuredrive.Ownership{State: featuredrive.Released, Epoch: 2}), "delivery did not release the owner")
		} else {
			expect(t, after.Ownership == before.Ownership, "ordinary phase changed ownership")
		}
	}

	ports := &phasePorts{landings: make(map[string]featuredrive.LandingProof), candidates: make(map[string]featuredrive.CandidateIdentity)}
	landTask := func(task, base, head string) {
		proposal := featuredrive.ProposalObservation{TaskID: task, ProposalID: task + "-proposal", Head: task + "-head", Base: base, Landed: head, ChecksGreen: true}
		review := featuredrive.ReviewObservation{ProposalID: proposal.ProposalID, Head: proposal.Head, State: "approved"}
		err := featuredrive.RecordLanding(t.Context(), ports, ports, proposal, review, head)
		expect(t, err == nil, "record proven %s landing: %v", task, err)
	}
	landTask("T1", "F0", "F1")
	taskIDs := []string{"T1"}
	tasksReady := func() error {
		tasks := make([]featuredrive.CandidateTask, 0, len(taskIDs))
		for _, id := range taskIDs {
			proof, ok := ports.landings[id]
			tasks = append(tasks, featuredrive.CandidateTask{ID: id, Landed: ok && proof.LandedRevision != ""})
		}
		return featuredrive.CandidateReady(tasks)
	}
	advance("tasks-integrated", "task-landings", "landed-T1", featuredrive.Candidate, tasksReady, tasksReady)
	candidate := featuredrive.CandidateIdentity{FeatureID: d.FeatureID, FeatureHead: "F1", MainHead: "M", Commit: "C1", Tree: "tree1", MergePolicyVersion: "merge-v1", BuilderVersion: "builder-v1"}
	publish := func() error {
		ports.candidate = candidate
		_, err := featuredrive.BuildCandidate(t.Context(), candidate.FeatureID, candidate.FeatureHead, candidate.MainHead, featuredrive.MergePolicy{Version: candidate.MergePolicyVersion}, candidate.BuilderVersion, ports, ports)
		return err
	}
	verifyCandidate := func() error {
		if ports.candidates[candidate.Ref()] != candidate {
			return featuredrive.ErrCandidateStale
		}
		return nil
	}
	advance("candidate-published", "candidate", candidate.Ref(), featuredrive.Verifying, publish, verifyCandidate)
	plan := featuredrive.ScenarioPlan{Schema: featuredrive.ScenarioPlanSchema, FeatureID: d.FeatureID, Candidate: candidate, ScenarioSetFingerprint: "all-required", Scenarios: []featuredrive.ScenarioSpec{{ID: "automated", Route: featuredrive.RouteAutomatic}, {ID: "human", Route: featuredrive.RouteManual}}}
	failed := featuredrive.ScenarioResult{ScenarioID: "automated", CandidateID: candidate.Digest(), Route: featuredrive.RouteAutomatic, RunnerIdentity: "fixture-runner", CompletedAt: time.Unix(10, 0), Outcome: featuredrive.OutcomeFailed, EvidenceRef: "failed-result"}
	advance("verification-repair-created", "verification-repair", "repair-1", featuredrive.Repairing, func() error {
		_, err := featuredrive.CreateVerificationRepair(t.Context(), ports, plan, failed)
		return err
	}, func() error {
		if ports.repair.CandidateID != candidate.Digest() || ports.repair.ScenarioID != failed.ScenarioID || ports.repair.EvidenceRef != failed.EvidenceRef {
			return featuredrive.ErrRepairEvidence
		}
		return nil
	})
	landTask("repair-1", "F1", "F2")
	taskIDs = append(taskIDs, "repair-1")
	advance("tasks-integrated", "task-landings", "landed-T1-and-repair", featuredrive.Candidate, tasksReady, tasksReady)
	candidate.FeatureHead, candidate.Commit, candidate.Tree = "F2", "C2", "tree2"
	advance("candidate-published", "candidate", candidate.Ref(), featuredrive.Verifying, publish, verifyCandidate)
	plan.Candidate = candidate
	passed := failed
	passed.CandidateID, passed.Outcome, passed.EvidenceRef = candidate.Digest(), featuredrive.OutcomePassed, "passed-result"
	manual, err := featuredrive.ResultFromManualAttestation(plan, featuredrive.ManualAttestation{ScenarioID: "human", CandidateID: candidate.Digest(), ActorIdentity: "fixture-human", Human: true, Accepted: true, EvidenceRef: "manual-result", CompletedAt: time.Unix(11, 0)})
	expect(t, err == nil, "bind manual result to rebuilt candidate: %v", err)
	evidence, err := featuredrive.NewCandidateEvidence(plan, []featuredrive.ScenarioResult{passed, manual})
	expect(t, err == nil, "complete rebuilt scenario set: %v", err)
	proposal := featuredrive.FinalProposal{FeatureID: candidate.FeatureID, ProposalID: "final-1", Head: "final-head", FeatureHead: candidate.FeatureHead, MainHead: candidate.MainHead, Candidate: candidate, ScenarioSetFingerprint: plan.ScenarioSetFingerprint, EvidenceID: "complete-evidence"}
	advance("final-proposal-activated", "final-proposal", proposal.ProposalID, featuredrive.FinalReview, func() error {
		verdict, err := featuredrive.EvaluateCandidateEvidence(plan, evidence)
		if err != nil {
			return err
		}
		_, _, err = featuredrive.ActivateFinalProposal(t.Context(), proposal, verdict, featuredrive.FinalActivationCapability{RequiresCandidateVerdict: true, EnforcesCurrentBase: true}, ports, ports)
		return err
	}, func() error {
		verdict, err := featuredrive.EvaluateCandidateEvidence(plan, evidence)
		if err != nil || !verdict.Accepted {
			return featuredrive.ErrFinalActivationRefused
		}
		return featuredrive.ValidateCandidateVerdictAt(ports.verdict, ports.proposal, proposal.Head, candidate.FeatureHead, candidate.MainHead)
	})
	accepted := featuredrive.AcceptedCandidate{Candidate: candidate, Proposal: proposal}
	landed := featuredrive.LandedRevision{Revision: "L", Base: candidate.MainHead, Tree: candidate.Tree}
	current := featuredrive.CurrentMain{Revision: "L"}
	verifyDelivery := func() error { return featuredrive.VerifyDelivery(t.Context(), accepted, landed, current, ports) }
	advance("final-landing-verified", "final-landing", "retained-L", featuredrive.DeliveryVerifying, verifyDelivery, verifyDelivery)
	advance("delivery-routing-released", "delivery", "verified-L-and-routing-release", featuredrive.Delivered, func() error {
		if err := verifyDelivery(); err != nil {
			return err
		}
		ports.routingReleased = true
		return nil
	}, func() error {
		if !ports.routingReleased {
			return featuredrive.ErrDeliveryUnverified
		}
		return verifyDelivery()
	})
}

// These ports stand in for medium-owned object publication/verification. The
// shared contract deliberately runs the production core predicates above.
type phasePorts struct {
	candidate       featuredrive.CandidateIdentity
	candidates      map[string]featuredrive.CandidateIdentity
	landings        map[string]featuredrive.LandingProof
	repair          featuredrive.VerificationRepair
	proposal        featuredrive.FinalProposal
	verdict         featuredrive.CandidateVerdict
	routingReleased bool
}

func (p *phasePorts) Build(context.Context, string, string, featuredrive.MergePolicy, string) (featuredrive.CandidateIdentity, error) {
	return p.candidate, nil
}
func (p *phasePorts) CreateOnly(_ context.Context, ref string, candidate featuredrive.CandidateIdentity) error {
	if prior, ok := p.candidates[ref]; ok && prior != candidate {
		return featuredrive.ErrCandidateExists
	}
	p.candidates[ref] = candidate
	return nil
}
func (p *phasePorts) CreateVerificationRepair(_ context.Context, repair featuredrive.VerificationRepair) (string, error) {
	p.repair = repair
	return repair.ID, nil
}
func (p *phasePorts) OpenOrUpdateFinalProposal(_ context.Context, proposal featuredrive.FinalProposal) (featuredrive.FinalProposal, error) {
	p.proposal = proposal
	return proposal, nil
}
func (p *phasePorts) PublishCandidateVerdict(_ context.Context, verdict featuredrive.CandidateVerdict) error {
	p.verdict = verdict
	return nil
}
func (*phasePorts) VerifyDelivery(context.Context, featuredrive.AcceptedCandidate, featuredrive.LandedRevision, featuredrive.CurrentMain) (featuredrive.DeliveryProof, error) {
	return featuredrive.DeliveryProof{AcceptedBase: true, CandidateTree: true, MainAncestor: true}, nil
}

func (*phasePorts) VerifyLanding(_ context.Context, proposal featuredrive.ProposalObservation, _ featuredrive.ReviewObservation, featureHead string) (featuredrive.LandingProof, error) {
	return featuredrive.LandingProof{ReviewedHead: proposal.Head, ReviewedBase: proposal.Base, LandedRevision: proposal.Landed, FeatureHead: featureHead, FeatureBase: proposal.Base}, nil
}
func (p *phasePorts) RecordLanding(_ context.Context, task, _ string, proof featuredrive.LandingProof) error {
	p.landings[task] = proof
	return nil
}
