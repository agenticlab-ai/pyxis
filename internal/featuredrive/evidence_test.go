package featuredrive

import (
	"context"
	"errors"
	"testing"
	"time"
)

var evidenceNow = time.Date(2026, 7, 26, 22, 0, 0, 0, time.UTC)

func evidencePlan(candidate CandidateIdentity) ScenarioPlan {
	return ScenarioPlan{
		Schema:                 ScenarioPlanSchema,
		FeatureID:              candidate.FeatureID,
		Candidate:              candidate,
		ScenarioSetFingerprint: "suite-v1",
		Scenarios: []ScenarioSpec{
			{ID: "owner", Route: RouteAutomatic},
			{ID: "special", Route: RouteSpecialized, SpecializedRunner: "gpu"},
			{ID: "manual", Route: RouteManual},
		},
	}
}

func evidenceCandidate(feature, commit string) CandidateIdentity {
	return CandidateIdentity{FeatureID: feature, FeatureHead: "feature-" + commit, MainHead: "main-" + commit, Commit: commit, Tree: "tree-" + commit, MergePolicyVersion: "merge-v1", BuilderVersion: "builder-v1"}
}

type fakeScenarioRunner struct {
	id     string
	result ScenarioResult
	seen   ScenarioExecution
	err    error
}

func (r *fakeScenarioRunner) Identity() string { return r.id }
func (r *fakeScenarioRunner) Run(_ context.Context, e ScenarioExecution) (ScenarioResult, error) {
	r.seen = e
	return r.result, r.err
}

func TestExecuteScenario_RoutesSpecializedBeforeOwnerAndBindsCandidate(t *testing.T) {
	plan := evidencePlan(evidenceCandidate("feature", "candidate"))
	owner := &fakeScenarioRunner{id: "local", result: passingResultFor(plan, "owner", RouteAutomatic, "local")}
	gpu := &fakeScenarioRunner{id: "gpu", result: passingResultFor(plan, "special", RouteSpecialized, "gpu")}

	got, err := ExecuteScenario(context.Background(), plan, "special", owner, map[string]ScenarioRunner{"gpu": gpu})
	if err != nil {
		t.Fatal(err)
	}
	if gpu.seen.CandidateID != plan.Candidate.Digest() || gpu.seen.Candidate.Commit != plan.Candidate.Commit {
		t.Fatalf("specialized runner saw wrong candidate: %+v", gpu.seen)
	}
	if owner.seen.CandidateID != "" {
		t.Fatal("owner runner executed a specialized scenario")
	}
	if got.RunnerIdentity != "gpu" || got.Route != RouteSpecialized || got.CandidateID != plan.Candidate.Digest() {
		t.Fatalf("result not rebound to selected route/candidate: %+v", got)
	}

	got, err = ExecuteScenario(context.Background(), plan, "owner", owner, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.RunnerIdentity != "local" || owner.seen.CandidateID != plan.Candidate.Digest() {
		t.Fatalf("owner-default route = %+v, execution = %+v", got, owner.seen)
	}
	if _, err = ExecuteScenario(context.Background(), plan, "manual", owner, nil); !errors.Is(err, ErrManualRequired) {
		t.Fatalf("manual route error = %v, want ErrManualRequired", err)
	}
	owner.result.CandidateID = "wrong-candidate"
	if _, err = ExecuteScenario(context.Background(), plan, "owner", owner, nil); !errors.Is(err, ErrEvidenceMalformed) {
		t.Fatalf("wrong-candidate runner error = %v", err)
	}
}

func TestManualAttestation_RequiresHumanExactCandidateAndSettlesOnlyNamedScenario(t *testing.T) {
	plan := evidencePlan(evidenceCandidate("feature", "candidate"))
	valid := ManualAttestation{ScenarioID: "manual", CandidateID: plan.Candidate.Digest(), ActorIdentity: "human:operator", Human: true, Accepted: true, EvidenceRef: "attestation:1", CompletedAt: evidenceNow}
	got, err := ResultFromManualAttestation(plan, valid)
	if err != nil {
		t.Fatal(err)
	}
	if got.Outcome != OutcomeManualAccepted || got.ScenarioID != "manual" || got.ActorIdentity != "human:operator" {
		t.Fatalf("manual result = %+v", got)
	}

	bad := []ManualAttestation{
		{ScenarioID: "manual", CandidateID: "wrong", ActorIdentity: "human:operator", Human: true, Accepted: true, EvidenceRef: "a", CompletedAt: evidenceNow},
		{ScenarioID: "manual", CandidateID: plan.Candidate.Digest(), ActorIdentity: "machine:runner", Human: false, Accepted: true, EvidenceRef: "a", CompletedAt: evidenceNow},
		{ScenarioID: "owner", CandidateID: plan.Candidate.Digest(), ActorIdentity: "human:operator", Human: true, Accepted: true, EvidenceRef: "a", CompletedAt: evidenceNow},
		{ScenarioID: "manual", CandidateID: plan.Candidate.Digest(), ActorIdentity: "", Human: true, Accepted: true, EvidenceRef: "a", CompletedAt: evidenceNow},
	}
	for i, att := range bad {
		if _, err := ResultFromManualAttestation(plan, att); !errors.Is(err, ErrEvidenceMalformed) {
			t.Errorf("bad attestation %d error = %v", i, err)
		}
	}
}

func TestEvaluateCandidateEvidence_ClosedOutcomeMatrixAndCompleteness(t *testing.T) {
	plan := evidencePlan(evidenceCandidate("feature", "candidate"))
	acceptedManual, err := ResultFromManualAttestation(plan, ManualAttestation{ScenarioID: "manual", CandidateID: plan.Candidate.Digest(), ActorIdentity: "human:operator", Human: true, Accepted: true, EvidenceRef: "manual:1", CompletedAt: evidenceNow})
	if err != nil {
		t.Fatal(err)
	}
	base := []ScenarioResult{
		passingResultFor(plan, "owner", RouteAutomatic, "local"),
		passingResultFor(plan, "special", RouteSpecialized, "gpu"),
		acceptedManual,
	}
	evidence, err := NewCandidateEvidence(plan, base)
	if err != nil {
		t.Fatal(err)
	}
	verdict, err := EvaluateCandidateEvidence(plan, evidence)
	if err != nil || !verdict.Accepted || len(verdict.Blocks) != 0 {
		t.Fatalf("accepted verdict = %+v, err=%v", verdict, err)
	}

	for _, outcome := range []ScenarioOutcome{OutcomeFailed, OutcomeSkipped, OutcomeUnavailable, OutcomeInconclusive, OutcomePending} {
		t.Run(string(outcome), func(t *testing.T) {
			results := append([]ScenarioResult(nil), base...)
			results[0].Outcome = outcome
			e, err := NewCandidateEvidence(plan, results)
			if err != nil {
				t.Fatal(err)
			}
			v, err := EvaluateCandidateEvidence(plan, e)
			if err != nil || v.Accepted || len(v.Blocks) != 1 || v.Blocks[0].Outcome != outcome {
				t.Fatalf("verdict = %+v, err=%v", v, err)
			}
		})
	}

	partial, err := NewCandidateEvidence(plan, base[:2])
	if err != nil {
		t.Fatal(err)
	}
	verdict, err = EvaluateCandidateEvidence(plan, partial)
	if err != nil || verdict.Accepted || len(verdict.Blocks) != 1 || verdict.Blocks[0].ScenarioID != "manual" {
		t.Fatalf("partial verdict = %+v, err=%v", verdict, err)
	}

	conflicting := append(append([]ScenarioResult(nil), base...), base[0])
	evidence, err = NewCandidateEvidence(plan, conflicting)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = EvaluateCandidateEvidence(plan, evidence); !errors.Is(err, ErrEvidenceConflict) {
		t.Fatalf("conflicting evidence error = %v", err)
	}
}

func TestEvaluateCandidateEvidence_RejectsWrongCandidateIdentityAndFingerprint(t *testing.T) {
	plan := evidencePlan(evidenceCandidate("feature", "candidate"))
	result := passingResultFor(plan, "owner", RouteAutomatic, "local")
	evidence := CandidateEvidence{Schema: CandidateEvidenceSchema, FeatureID: plan.FeatureID, Candidate: plan.Candidate, ScenarioSetFingerprint: plan.ScenarioSetFingerprint, Results: []ScenarioResult{result}}

	wrongCandidate := plan
	wrongCandidate.Candidate = evidenceCandidate("feature", "new-candidate")
	if _, err := EvaluateCandidateEvidence(wrongCandidate, evidence); !errors.Is(err, ErrEvidenceMalformed) {
		t.Fatalf("wrong-candidate error = %v", err)
	}
	wrongSuite := plan
	wrongSuite.ScenarioSetFingerprint = "suite-v2"
	if _, err := EvaluateCandidateEvidence(wrongSuite, evidence); !errors.Is(err, ErrEvidenceMalformed) {
		t.Fatalf("wrong-suite error = %v", err)
	}
	result.CandidateID = "other"
	evidence.Results = []ScenarioResult{result}
	if _, err := EvaluateCandidateEvidence(plan, evidence); !errors.Is(err, ErrEvidenceMalformed) {
		t.Fatalf("wrong-result-candidate error = %v", err)
	}
}

type recordingRepairCreator struct {
	repairs []VerificationRepair
	ids     map[string]string
}

func (r *recordingRepairCreator) CreateVerificationRepair(_ context.Context, repair VerificationRepair) (string, error) {
	if id := r.ids[repair.ID]; id != "" {
		return id, nil
	}
	id := "task:" + repair.ID
	if r.ids == nil {
		r.ids = map[string]string{}
	}
	r.ids[repair.ID] = id
	r.repairs = append(r.repairs, repair)
	return id, nil
}

func TestCreateVerificationRepair_IdempotentAndFullRerunRequired(t *testing.T) {
	oldPlan := evidencePlan(evidenceCandidate("feature", "candidate-old"))
	failed := passingResultFor(oldPlan, "owner", RouteAutomatic, "local")
	failed.Outcome = OutcomeFailed
	failed.EvidenceRef = "log:failed"
	failed.ImplicatedRefs = []string{"proposal:8", "task:3"}
	creator := &recordingRepairCreator{}

	first, err := CreateVerificationRepair(context.Background(), creator, oldPlan, failed)
	if err != nil {
		t.Fatal(err)
	}
	second, err := CreateVerificationRepair(context.Background(), creator, oldPlan, failed)
	if err != nil {
		t.Fatal(err)
	}
	if first != second || len(creator.repairs) != 1 {
		t.Fatalf("repair ids %q/%q, writes=%d", first, second, len(creator.repairs))
	}
	got := creator.repairs[0]
	if got.CandidateID != oldPlan.Candidate.Digest() || got.ScenarioID != "owner" || got.EvidenceRef != "log:failed" || len(got.ImplicatedRefs) != 2 || got.ImplicatedRefs[0] != "proposal:8" {
		t.Fatalf("repair = %+v", got)
	}

	// A landed repair changes the feature head and therefore the candidate.
	// Old evidence is stale, and a partial result on the new candidate cannot
	// restore eligibility; only the complete accepted suite can.
	oldEvidence, err := NewCandidateEvidence(oldPlan, []ScenarioResult{failed})
	if err != nil {
		t.Fatal(err)
	}
	newPlan := evidencePlan(evidenceCandidate("feature", "candidate-after-repair"))
	if _, err := EvaluateCandidateEvidence(newPlan, oldEvidence); !errors.Is(err, ErrEvidenceMalformed) {
		t.Fatalf("old evidence on new candidate error = %v", err)
	}
	partial, err := NewCandidateEvidence(newPlan, []ScenarioResult{passingResultFor(newPlan, "owner", RouteAutomatic, "local")})
	if err != nil {
		t.Fatal(err)
	}
	verdict, err := EvaluateCandidateEvidence(newPlan, partial)
	if err != nil || verdict.Accepted || len(verdict.Blocks) != 2 {
		t.Fatalf("partial rerun verdict = %+v, err=%v", verdict, err)
	}
	manual, err := ResultFromManualAttestation(newPlan, ManualAttestation{ScenarioID: "manual", CandidateID: newPlan.Candidate.Digest(), ActorIdentity: "human:operator", Human: true, Accepted: true, EvidenceRef: "manual:new", CompletedAt: evidenceNow})
	if err != nil {
		t.Fatal(err)
	}
	complete, err := NewCandidateEvidence(newPlan, []ScenarioResult{
		passingResultFor(newPlan, "owner", RouteAutomatic, "local"),
		passingResultFor(newPlan, "special", RouteSpecialized, "gpu"),
		manual,
	})
	if err != nil {
		t.Fatal(err)
	}
	verdict, err = EvaluateCandidateEvidence(newPlan, complete)
	if err != nil || !verdict.Accepted {
		t.Fatalf("complete rerun verdict = %+v, err=%v", verdict, err)
	}
}

func TestScenarioPlanAndExecutionBoundaryFailures(t *testing.T) {
	valid := evidencePlan(evidenceCandidate("feature", "candidate"))
	cases := []ScenarioPlan{
		{},
		func() ScenarioPlan {
			p := valid
			p.Scenarios = append([]ScenarioSpec(nil), valid.Scenarios...)
			p.Scenarios[1].ID = p.Scenarios[0].ID
			return p
		}(),
		func() ScenarioPlan {
			p := valid
			p.Scenarios = append([]ScenarioSpec(nil), valid.Scenarios...)
			p.Scenarios[0].SpecializedRunner = "unexpected"
			return p
		}(),
		func() ScenarioPlan {
			p := valid
			p.Scenarios = append([]ScenarioSpec(nil), valid.Scenarios...)
			p.Scenarios[1].SpecializedRunner = ""
			return p
		}(),
		func() ScenarioPlan {
			p := valid
			p.Scenarios = append([]ScenarioSpec(nil), valid.Scenarios...)
			p.Scenarios[0].Route = ScenarioRoute("unknown")
			return p
		}(),
	}
	for i, plan := range cases {
		if err := plan.Validate(); !errors.Is(err, ErrEvidenceMalformed) {
			t.Fatalf("invalid plan %d error = %v", i, err)
		}
	}
	if _, err := ExecuteScenario(t.Context(), ScenarioPlan{}, "scenario", nil, nil); !errors.Is(err, ErrEvidenceMalformed) {
		t.Fatalf("invalid-plan execution error = %v", err)
	}
	if _, err := ExecuteScenario(t.Context(), valid, "missing", nil, nil); !errors.Is(err, ErrEvidenceMalformed) {
		t.Fatalf("missing-scenario execution error = %v", err)
	}
}

func passingResult(id string, route ScenarioRoute, runner string) ScenarioResult {
	return ScenarioResult{ScenarioID: id, Route: route, RunnerIdentity: runner, StartedAt: evidenceNow.Add(-time.Minute), CompletedAt: evidenceNow, Outcome: OutcomePassed, EvidenceRef: "run:" + id}
}

func passingResultFor(plan ScenarioPlan, id string, route ScenarioRoute, runner string) ScenarioResult {
	r := passingResult(id, route, runner)
	r.CandidateID = plan.Candidate.Digest()
	return r
}
