package candidatereview

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
	"github.com/agenticlab-ai/pyxis/internal/featuredrive"
)

func fixture(t *testing.T) (Binding, Snapshot, *source) {
	t.Helper()
	c := featuredrive.CandidateIdentity{FeatureID: "widget", FeatureHead: strings.Repeat("a", 40), MainHead: strings.Repeat("b", 40), Commit: strings.Repeat("c", 40), Tree: strings.Repeat("d", 40), MergePolicyVersion: "merge-v1", BuilderVersion: "builder-v1"}
	p := codehost.Proposal{SchemaVersion: codehost.ProposalSchema, ID: "final-widget", Feature: "widget", Kind: "implementation", Branch: "refs/heads/final/widget", Head: c.FeatureHead,
		TargetRef: "refs/heads/trunk", TargetOID: c.MainHead, SourceRef: "refs/heads/feature/widget", Sequence: 2, PreviousOID: strings.Repeat("f", 40), State: "prepared",
		CandidatePreparation: &codehost.CandidatePreparation{SchemaVersion: codehost.CandidatePreparationSchema, Operation: codehost.FeatureFinalMerge, Nonce: strings.Repeat("1", 32), Candidate: codehost.FinalCandidate(c), ScenarioSetFingerprint: "required-v1"}}
	b, err := Bind("/fixture/origin.git", codehost.ProposalGeneration{OID: strings.Repeat("e", 40), Proposal: p})
	if err != nil {
		t.Fatal(err)
	}
	plan := featuredrive.ScenarioPlan{Schema: featuredrive.ScenarioPlanSchema, FeatureID: c.FeatureID, Candidate: c, ScenarioSetFingerprint: "required-v1", Scenarios: []featuredrive.ScenarioSpec{
		{ID: "automatic", Route: featuredrive.RouteAutomatic}, {ID: "specialized", Route: featuredrive.RouteSpecialized, SpecializedRunner: "pdf-runner"}, {ID: "manual", Route: featuredrive.RouteManual},
	}}
	at := time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)
	results := []featuredrive.ScenarioResult{
		{ScenarioID: "automatic", CandidateID: c.Digest(), Route: featuredrive.RouteAutomatic, RunnerIdentity: "owner-runner", CompletedAt: at, Outcome: featuredrive.OutcomePassed, EvidenceRef: "retained-auto"},
		{ScenarioID: "specialized", CandidateID: c.Digest(), Route: featuredrive.RouteSpecialized, RunnerIdentity: "pdf-runner", CompletedAt: at, Outcome: featuredrive.OutcomePassed, EvidenceRef: "retained-specialized"},
		{ScenarioID: "manual", CandidateID: c.Digest(), Route: featuredrive.RouteManual, ActorIdentity: "registered-human", CompletedAt: at, Outcome: featuredrive.OutcomeManualAccepted, EvidenceRef: "retained-human"},
	}
	evidence, err := featuredrive.NewCandidateEvidence(plan, results)
	if err != nil {
		t.Fatal(err)
	}
	s := Snapshot{SnapshotSchema, b, plan, evidence}
	return b, s, &source{plan: plan, evidence: evidence}
}

type source struct {
	plan           featuredrive.ScenarioPlan
	evidence       featuredrive.CandidateEvidence
	fault          string
	authorizations int
	evidenceReads  int
}

func (s *source) Plan(context.Context, Binding) (featuredrive.ScenarioPlan, error) {
	if s.fault == "plan" {
		return featuredrive.ScenarioPlan{}, errors.New("current scenario plan unavailable")
	}
	return s.plan, nil
}
func (s *source) Evidence(context.Context, Binding) (featuredrive.CandidateEvidence, error) {
	s.evidenceReads++
	if s.fault == "evidence" {
		return featuredrive.CandidateEvidence{}, errors.New("current candidate results unavailable")
	}
	return s.evidence, nil
}
func (s *source) Authorize(context.Context, Binding, featuredrive.ScenarioPlan, featuredrive.CandidateEvidence) error {
	s.authorizations++
	if s.fault == "attribution" {
		return errors.New("manual attestation or runner authority unavailable")
	}
	return nil
}

func TestCandidateSnapshotRoundTripAndAuthorizedRecovery(t *testing.T) {
	b, expected, reader := fixture(t)
	s, err := Capture(t.Context(), b, reader)
	if err != nil || !reflect.DeepEqual(s, expected) || reader.authorizations != 1 {
		t.Fatalf("capture: %+v %v", s, err)
	}
	body, err := s.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := Parse(body)
	if err != nil || !reflect.DeepEqual(parsed, s) {
		t.Fatalf("canonical round trip: %v", err)
	}
	if err := Verify(t.Context(), b, parsed, reader); err != nil || reader.evidenceReads != 3 || reader.authorizations != 2 {
		t.Fatalf("retained verification skipped current results or attribution: %v", err)
	}
}

func TestCandidateRecoveryComparesSerializedTimesWithCurrentSource(t *testing.T) {
	b, _, reader := fixture(t)
	reader.evidence.Results[0].CompletedAt = time.Date(2026, 10, 3, 0, 0, 0, 0, time.FixedZone("runner-clock", 3600))
	snapshot, err := Capture(t.Context(), b, reader)
	if err != nil {
		t.Fatal(err)
	}
	body, err := snapshot.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	retained, err := Parse(body)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(t.Context(), b, retained, reader); err != nil {
		t.Fatalf("identical serialized results rejected the runner's clock location: %v", err)
	}
}

func TestCandidateSnapshotRejectsIncompleteStaleOrUnattributedResults(t *testing.T) {
	for _, fault := range []string{"missing", "failed", "skipped", "duplicate", "different candidate", "old scenario set", "new required scenario", "wrong specialized runner", "machine manual", "wrong binding", "plan", "evidence", "attribution", "cancelled", "missing reader"} {
		t.Run(fault, func(t *testing.T) {
			b, s, reader := fixture(t)
			ctx := t.Context()
			switch fault {
			case "missing":
				s.Evidence.Results = s.Evidence.Results[:2]
			case "failed":
				s.Evidence.Results[0].Outcome = featuredrive.OutcomeFailed
			case "skipped":
				s.Evidence.Results[0].Outcome = featuredrive.OutcomeSkipped
			case "duplicate":
				s.Evidence.Results = append(s.Evidence.Results, s.Evidence.Results[0])
			case "different candidate":
				s.Evidence.Results[0].CandidateID = "another-candidate"
			case "old scenario set":
				reader.plan.ScenarioSetFingerprint = "required-v2"
			case "new required scenario":
				reader.plan.Scenarios = append(reader.plan.Scenarios, featuredrive.ScenarioSpec{ID: "new", Route: featuredrive.RouteAutomatic})
			case "wrong specialized runner":
				s.Evidence.Results[1].RunnerIdentity = "owner-runner"
			case "machine manual":
				s.Evidence.Results[2].RunnerIdentity = "machine-runner"
			case "wrong binding":
				b.Generation.OID = strings.Repeat("9", 40)
			case "cancelled":
				cancelled, cancel := context.WithCancel(ctx)
				cancel()
				ctx = cancelled
			default:
				reader.fault = fault
			}
			if fault == "missing reader" {
				if Verify(ctx, b, s, nil) == nil || func() bool { _, e := Capture(ctx, b, nil); return e == nil }() {
					t.Fatal("missing authority port accepted")
				}
				return
			}
			if fault == "evidence" {
				if _, err := Capture(ctx, b, reader); err == nil {
					t.Fatal("unavailable evidence accepted")
				}
			} else if err := Verify(ctx, b, s, reader); err == nil {
				t.Fatal("incomplete final evidence accepted")
			}
		})
	}
}

func TestCandidateSnapshotClosedCanonicalBoundedAndPurposeBound(t *testing.T) {
	b, s, _ := fixture(t)
	body, err := s.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	for _, changed := range [][]byte{nil, []byte("{"), append(append([]byte(nil), body...), body...), []byte(" " + string(body)), []byte(strings.Replace(string(body), `"schema_version":`, `"unknown":1,"schema_version":`, 1)), []byte(strings.Replace(string(body), `"schema_version":`, `"schema_version":"ignored","schema_version":`, 1)), []byte(strings.Repeat("x", codehost.MaxReviewArtifactBytes+1))} {
		if _, err := Parse(changed); err == nil {
			t.Fatal("noncanonical, open, or oversized envelope accepted")
		}
	}
	s.Evidence.Results[0].EvidenceRef = strings.Repeat("x", codehost.MaxReviewArtifactBytes)
	if _, err := s.Canonical(); err == nil {
		t.Fatal("oversized whole result set accepted")
	}
	for _, fault := range []string{"origin", "task", "state", "nonce", "feature", "head", "target", "source alias", "short oid"} {
		binding, _, _ := fixture(t)
		p := &binding.Generation.Proposal
		switch fault {
		case "origin":
			binding.Origin = ""
		case "task":
			p.Task = "T1"
		case "state":
			p.State = "open"
		case "nonce":
			p.CandidatePreparation.Nonce = "invalid"
		case "feature":
			p.Feature = "other"
		case "head":
			p.Head = strings.Repeat("9", 40)
		case "target":
			p.TargetOID = strings.Repeat("9", 40)
		case "source alias":
			p.SourceRef = p.Branch
		case "short oid":
			binding.Generation.OID = "short"
		}
		if _, err := Capture(t.Context(), binding, &source{}); err == nil {
			t.Fatalf("wrong candidate binding accepted: %s", fault)
		}
	}
	if b.Validate() != nil {
		t.Fatal("valid binding changed")
	}
}
