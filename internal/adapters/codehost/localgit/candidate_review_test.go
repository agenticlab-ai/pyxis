package localgit

import (
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
	"github.com/agenticlab-ai/pyxis/internal/featuredrive"
	"github.com/agenticlab-ai/pyxis/internal/featuredrive/candidatereview"
	"github.com/agenticlab-ai/pyxis/internal/featuredrive/candidatereview/conformancetest"
)

type candidateReviewSource struct {
	plan          featuredrive.ScenarioPlan
	evidence      featuredrive.CandidateEvidence
	fault         string
	evidenceReads int
	policyOrigin  string
	policyRef     string
	policyOID     string
	beforeLease   func()
}

func (s *candidateReviewSource) WithEvidence(ctx context.Context, _ candidatereview.Binding, expected candidatereview.Snapshot, mutate func() error) error {
	if s.beforeLease != nil {
		s.beforeLease()
	}
	if !reflect.DeepEqual(s.plan, expected.Plan) || !reflect.DeepEqual(s.evidence, expected.Evidence) || s.policyOrigin == "" {
		return candidatereview.ErrIncomplete
	}
	// This fixture's version covers scenario policy, complete results and
	// attribution sources. Their real origin lease spans the operator write.
	return withUnchangedRefs(ctx, s.policyOrigin, []unchangedRef{{s.policyRef, s.policyOID}}, mutate)
}

func (s *candidateReviewSource) Plan(context.Context, candidatereview.Binding) (featuredrive.ScenarioPlan, error) {
	return s.plan, nil
}
func (s *candidateReviewSource) Evidence(context.Context, candidatereview.Binding) (featuredrive.CandidateEvidence, error) {
	s.evidenceReads++
	if s.fault == "evidence unavailable" {
		return featuredrive.CandidateEvidence{}, errors.New(s.fault)
	}
	return s.evidence, nil
}
func (s *candidateReviewSource) Authorize(context.Context, candidatereview.Binding, featuredrive.ScenarioPlan, featuredrive.CandidateEvidence) error {
	if s.fault == "attribution unavailable" {
		return errors.New(s.fault)
	}
	return nil
}

func candidateReviewFixture(t *testing.T) (Options, *ProposalStore, *ReviewArtifactStore, *candidateReviewSource, *candidatereview.Requests, codehost.ProposalGeneration) {
	t.Helper()
	o, d, drive := featureProposalFixture(t)
	base := fixtureGit(t, o.Root, "rev-parse", "HEAD")
	fixtureGit(t, o.Root, "commit", "--allow-empty", "-m", "feature task result")
	head := fixtureGit(t, o.Root, "rev-parse", "HEAD")
	fixtureGit(t, o.Root, "push", o.Remote, head+":"+d.IntegrationRef)
	opened, err := drive.RequestFinal(t.Context(), codehost.ProposalChange{ID: "final-widget", Feature: d.FeatureID, Branch: "refs/heads/final/widget", Head: head, State: "open", ExpectedTargetOID: base})
	if err != nil {
		t.Fatal(err)
	}
	candidate, err := featuredrive.NewGitCandidateBuilder(o.Root).Build(t.Context(), head, base, featuredrive.MergePolicy{Version: "merge-v1"}, "builder-v1")
	if err != nil {
		t.Fatal(err)
	}
	candidate.FeatureID = d.FeatureID
	plan := featuredrive.ScenarioPlan{Schema: featuredrive.ScenarioPlanSchema, FeatureID: d.FeatureID, Candidate: candidate, ScenarioSetFingerprint: "required-v1", Scenarios: []featuredrive.ScenarioSpec{
		{ID: "automatic", Route: featuredrive.RouteAutomatic}, {ID: "specialized", Route: featuredrive.RouteSpecialized, SpecializedRunner: "pdf-runner"}, {ID: "manual", Route: featuredrive.RouteManual},
	}}
	at := time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)
	evidence, err := featuredrive.NewCandidateEvidence(plan, []featuredrive.ScenarioResult{
		{ScenarioID: "automatic", CandidateID: candidate.Digest(), Route: featuredrive.RouteAutomatic, RunnerIdentity: "owner-runner", CompletedAt: at, Outcome: featuredrive.OutcomePassed, EvidenceRef: "retained-auto"},
		{ScenarioID: "specialized", CandidateID: candidate.Digest(), Route: featuredrive.RouteSpecialized, RunnerIdentity: "pdf-runner", CompletedAt: at, Outcome: featuredrive.OutcomePassed, EvidenceRef: "retained-specialized"},
		{ScenarioID: "manual", CandidateID: candidate.Digest(), Route: featuredrive.RouteManual, ActorIdentity: "registered-human", CompletedAt: at, Outcome: featuredrive.OutcomeManualAccepted, EvidenceRef: "retained-manual"},
	})
	if err != nil {
		t.Fatal(err)
	}
	reader := &candidateReviewSource{plan: plan, evidence: evidence}
	reader.policyOrigin, reader.policyRef, reader.policyOID = o.Origin, "refs/pyxis/test-candidate-policy", base
	fixtureGit(t, o.Origin, "update-ref", reader.policyRef, reader.policyOID)
	options := ProposalOptions{Git: o, Kind: "implementation", IntegrationRef: d.TargetMainRef, HeadRef: d.IntegrationRef}
	proposals, artifacts := NewProposals(options), NewReviewArtifacts(options)
	requests, err := candidatereview.NewRequests(t.Context(), proposals, artifacts, reader)
	if err != nil {
		t.Fatal(err)
	}
	return o, proposals, artifacts, reader, requests, opened.Generation
}

func TestCandidateReviewPortConformance(t *testing.T) {
	t.Parallel()
	conformancetest.Run(t, func(t *testing.T) conformancetest.Fixture {
		// The conformance cases do not share mutable repository state.
		t.Parallel()
		o, proposals, artifacts, reader, requests, opened := candidateReviewFixture(t)
		return conformancetest.Fixture{
			ID: opened.Proposal.ID, Candidate: codehost.FinalCandidate(reader.plan.Candidate), Scenarios: reader.plan.ScenarioSetFingerprint,
			Proposals: proposals, Artifacts: artifacts, Requests: requests, Receipts: NewCandidateReceipts(proposals.options, requests),
			ChangePolicy: func(*testing.T) { reader.plan.ScenarioSetFingerprint = "required-v2" },
			State:        func(t *testing.T) string { return fixtureGit(t, o.Origin, "show-ref") },
			Forget:       func(t *testing.T, ref codehost.ArtifactRef) { fixtureGit(t, o.Origin, "update-ref", "-d", ref.Ref) },
			RemoveCheckout: func(t *testing.T) {
				if err := os.RemoveAll(o.Root); err != nil {
					t.Fatal(err)
				}
			},
			Clone: func(t *testing.T) conformancetest.Fixture {
				options := proposals.options
				options.Git = proposalClone(t, o)
				p, a := NewProposals(options), NewReviewArtifacts(options)
				r, err := candidatereview.NewRequests(t.Context(), p, a, reader)
				if err != nil {
					t.Fatal(err)
				}
				return conformancetest.Fixture{Proposals: p, Artifacts: a, Requests: r, Receipts: NewCandidateReceipts(options, r)}
			},
		}
	})
}

func TestCandidateAcceptedRerunRotatesRequestAndRecoversFromClone(t *testing.T) {
	t.Parallel()
	o, proposals, artifacts, reader, requests, opened := candidateReviewFixture(t)
	candidate, scenarios := codehost.FinalCandidate(reader.plan.Candidate), reader.plan.ScenarioSetFingerprint
	old, err := requests.Prepare(t.Context(), opened.Proposal.ID, candidate, scenarios)
	if err != nil {
		t.Fatal(err)
	}
	receipts := NewCandidateReceipts(proposals.options, requests)
	if _, err := receipts.Record(t.Context(), old.Binding, "operator", ""); err != nil {
		t.Fatal(err)
	}
	result := reader.evidence.Results[0]
	result.CompletedAt = result.CompletedAt.Add(time.Second)
	result.EvidenceRef += "-accepted-rerun"
	reader.evidence.Results[0] = result
	packet, err := requests.Prepare(t.Context(), opened.Proposal.ID, candidate, scenarios)
	if err != nil || packet.Binding.ProposalOID == old.Binding.ProposalOID || packet.Binding.Nonce == old.Binding.Nonce || packet.Binding.Evidence == old.Binding.Evidence || packet.Binding.Sequence != old.Binding.Sequence+1 {
		t.Fatalf("accepted rerun did not publish fresh review intent: %+v %v", packet.Binding, err)
	}
	if _, err := receipts.Verify(t.Context(), old.Binding); err == nil {
		t.Fatal("old approval authorized the rerun")
	}
	if _, err := artifacts.ReadRetained(t.Context(), opened, codehost.CandidateRequest, old.Request.Identity); err == nil {
		t.Fatal("historical proof was accepted for an unrelated generation")
	}
	for _, identity := range []codehost.ArtifactRef{old.Request.Identity, old.Evidence.Identity} {
		if fixtureGit(t, o.Origin, "rev-parse", identity.Ref) != identity.OID {
			t.Fatal("rotation replaced immutable historical proof")
		}
	}
	options := proposals.options
	options.Git = proposalClone(t, o)
	p, a := NewProposals(options), NewReviewArtifacts(options)
	clone, err := candidatereview.NewRequests(t.Context(), p, a, reader)
	if err != nil {
		t.Fatal(err)
	}
	before := fixtureGit(t, o.Origin, "show-ref")
	resumed, err := clone.Prepare(t.Context(), opened.Proposal.ID, candidate, scenarios)
	if err != nil || !reflect.DeepEqual(resumed, packet) || fixtureGit(t, o.Origin, "show-ref") != before {
		t.Fatalf("clone did not resume the rerun's exact request: %v", err)
	}
	if _, err := NewCandidateReceipts(options, clone).Record(t.Context(), packet.Binding, "operator", ""); err != nil {
		t.Fatal(err)
	}
}

func TestCandidateReviewRefusesMovedInputsPolicyAndAttribution(t *testing.T) {
	t.Parallel()
	for _, fault := range []string{"feature moved", "main moved", "scenario changed", "attribution unavailable", "missing evidence", "missing request", "altered evidence", "altered request"} {
		t.Run(fault, func(t *testing.T) {
			t.Parallel()
			o, proposals, _, reader, requests, opened := candidateReviewFixture(t)
			packet := seedCandidateReview(t, o, reader, opened)
			receipts := NewCandidateReceipts(proposals.options, requests)
			seedReceipt(t, o, packet.Binding)
			switch fault {
			case "feature moved", "main moved":
				ref, parent := proposals.options.HeadRef, packet.Binding.Head
				if fault == "main moved" {
					ref, parent = packet.Binding.TargetRef, packet.Binding.TargetOID
				}
				next := fixtureGit(t, o.Origin, "-c", "user.name=Fixture Author", "-c", "user.email=fixture@example.test", "commit-tree", parent+"^{tree}", "-p", parent, "-m", "advanced workflow head")
				fixtureGit(t, o.Origin, "update-ref", ref, next)
			case "scenario changed":
				reader.plan.ScenarioSetFingerprint = "required-v2"
			case "attribution unavailable":
				reader.fault = fault
			case "missing evidence":
				fixtureGit(t, o.Origin, "update-ref", "-d", packet.Evidence.Identity.Ref)
			case "missing request":
				fixtureGit(t, o.Origin, "update-ref", "-d", packet.Request.Identity.Ref)
			case "altered evidence":
				fixtureGit(t, o.Origin, "update-ref", packet.Evidence.Identity.Ref, packet.Binding.Head)
			case "altered request":
				fixtureGit(t, o.Origin, "update-ref", packet.Request.Identity.Ref, packet.Binding.Head)
			}
			before := fixtureGit(t, o.Origin, "show-ref")
			if _, err := requests.Observe(t.Context(), opened.Proposal.ID); err == nil {
				t.Fatal("stale or unattributed proof presented")
			}
			if _, err := receipts.Verify(t.Context(), packet.Binding); err == nil {
				t.Fatal("approval survived stale policy, attribution, or origin proof")
			}
			if _, err := receipts.Record(t.Context(), packet.Binding, "declared operator", ""); err == nil {
				t.Fatal("stale final proof reused a declaration")
			}
			if fixtureGit(t, o.Origin, "show-ref") != before {
				t.Fatal("failed observation repaired shared state")
			}
		})
	}
}

func TestCandidateReviewInterruptedPreparationAndLostRequestAcknowledgement(t *testing.T) {
	t.Parallel()
	o, p, a, reader, requests, opened := candidateReviewFixture(t)
	candidate, scenarios := codehost.FinalCandidate(reader.plan.Candidate), reader.plan.ScenarioSetFingerprint
	reader.fault = "evidence unavailable"
	if _, err := requests.Prepare(t.Context(), opened.Proposal.ID, candidate, scenarios); err == nil {
		t.Fatal("missing scenario results presented")
	}
	first, err := p.Read(t.Context(), opened.Proposal.ID)
	if err != nil {
		t.Fatal(err)
	}
	reader.fault = ""
	a.commit = func(ctx context.Context, origin string, transaction []byte) error {
		_, err := proposalGit(ctx, origin, transaction, nil, "update-ref", "--stdin")
		if err == nil {
			return errors.New("lost acknowledgement")
		}
		return err
	}
	// The first retained proof loses its acknowledgement. Recovery starts a
	// new generation because no complete request was retained for that one.
	if _, err := requests.Prepare(t.Context(), opened.Proposal.ID, candidate, scenarios); err == nil {
		t.Fatal("lost acknowledgement was hidden")
	}
	a.commit = nil
	packet, err := requests.Prepare(t.Context(), opened.Proposal.ID, candidate, scenarios)
	if err != nil || packet.Binding.ProposalOID == first.OID || packet.Binding.Nonce == first.Proposal.CandidatePreparation.Nonce {
		t.Fatalf("interrupted generation was reused: %v", err)
	}
	before := fixtureGit(t, o.Origin, "show-ref")
	reader.fault = "evidence unavailable"
	if _, err := requests.Prepare(t.Context(), opened.Proposal.ID, candidate, scenarios); err == nil || fixtureGit(t, o.Origin, "show-ref") != before {
		t.Fatal("unavailable current evidence reused or replaced the request")
	}
	reader.fault = ""
	resumed, err := requests.Prepare(t.Context(), opened.Proposal.ID, candidate, scenarios)
	if err != nil || !reflect.DeepEqual(resumed, packet) || fixtureGit(t, o.Origin, "show-ref") != before {
		t.Fatalf("complete request was not resumed: %v", err)
	}
	// A lost acknowledgement after the request transaction is different:
	// its complete origin packet must be reused by a new coordinator.
	reader.fault = ""
	current, err := p.Read(t.Context(), opened.Proposal.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Publish(t.Context(), revision(current, current.Proposal.Head)); err != nil {
		t.Fatal(err)
	}
	a.commit = func(ctx context.Context, origin string, transaction []byte) error {
		_, err := proposalGit(ctx, origin, transaction, nil, "update-ref", "--stdin")
		if err == nil && strings.Contains(string(transaction), "create refs/pyxis/review-requests/") {
			return errors.New("lost request acknowledgement")
		}
		return err
	}
	if _, err := requests.Prepare(t.Context(), opened.Proposal.ID, candidate, scenarios); err == nil {
		t.Fatal("lost request acknowledgement was hidden")
	}
	a.commit = nil
	observed, err := requests.Observe(t.Context(), opened.Proposal.ID)
	if err != nil {
		t.Fatal(err)
	}
	before = fixtureGit(t, o.Origin, "show-ref")
	newProcess, err := candidatereview.NewRequests(t.Context(), p, a, reader)
	if err != nil {
		t.Fatal(err)
	}
	resumed, err = newProcess.Prepare(t.Context(), opened.Proposal.ID, candidate, scenarios)
	if err != nil || !reflect.DeepEqual(resumed, observed) || fixtureGit(t, o.Origin, "show-ref") != before {
		t.Fatalf("lost request acknowledgement minted another request: %v", err)
	}
}

func TestCandidateArtifactAndReceiptTransactionsLeaseBothWorkflowHeads(t *testing.T) {
	t.Parallel()
	for _, publication := range []string{"evidence", "request", "receipt"} {
		for _, changed := range []string{"feature", "main"} {
			t.Run(publication+"/"+changed, func(t *testing.T) {
				t.Parallel()
				o, proposals, artifacts, reader, requests, opened := candidateReviewFixture(t)
				packet := seedCandidateReview(t, o, reader, opened)
				artifact := packet.Evidence
				if publication == "request" {
					artifact = packet.Request
				}
				if publication != "receipt" {
					fixtureGit(t, o.Origin, "update-ref", "-d", artifact.Identity.Ref)
				}
				ref, parent := proposals.options.HeadRef, packet.Binding.Head
				if changed == "main" {
					ref, parent = packet.Binding.TargetRef, packet.Binding.TargetOID
				}
				next := fixtureGit(t, o.Root, "commit-tree", parent+"^{tree}", "-p", parent, "-m", "concurrent workflow advance")
				fixtureGit(t, o.Root, "push", o.Remote, next+":refs/pyxis/test/concurrent")
				wantRefs := ""
				commit := func(ctx context.Context, origin string, transaction []byte) error {
					fixtureGit(t, origin, "update-ref", ref, next)
					wantRefs = fixtureGit(t, origin, "show-ref")
					_, err := proposalGit(ctx, origin, transaction, nil, "update-ref", "--stdin")
					return err
				}
				var err error
				if publication == "receipt" {
					receipts := NewCandidateReceipts(proposals.options, requests)
					receipts.commit = commit
					_, err = receipts.Record(t.Context(), packet.Binding, "declared operator", "")
				} else {
					artifacts.commit = commit
					_, err = artifacts.Retain(t.Context(), packet.Snapshot.Binding.Generation, artifact.Kind, artifact.Body)
				}
				if err == nil || wantRefs == "" || fixtureGit(t, o.Origin, "show-ref") != wantRefs {
					t.Fatalf("moved workflow head allowed partial publication: %v", err)
				}
			})
		}
	}
}

func TestCandidatePortsRejectTaskPurposeWithoutOriginWrites(t *testing.T) {
	t.Parallel()
	o, proposals, artifacts, reader, requests, opened := candidateReviewFixture(t)
	packet, err := requests.Prepare(t.Context(), opened.Proposal.ID, codehost.FinalCandidate(reader.plan.Candidate), reader.plan.ScenarioSetFingerprint)
	if err != nil {
		t.Fatal(err)
	}
	before := fixtureGit(t, o.Origin, "show-ref")
	for _, kind := range []string{codehost.TaskAggregate, codehost.TaskRequest} {
		if _, err := artifacts.Retain(t.Context(), packet.Snapshot.Binding.Generation, kind, packet.Evidence.Body); err == nil {
			t.Fatal("candidate port accepted task proof")
		}
	}
	if _, err := NewTaskReceipts(proposals.options, nil).Binding(t.Context(), opened.Proposal.ID); err == nil {
		t.Fatal("task receipt route accepted final workflow")
	}
	if _, err := NewCandidateReceipts(proposals.options, nil).Record(t.Context(), packet.Binding, "operator", ""); err == nil {
		t.Fatal("final receipt accepted a missing current policy verifier")
	}
	if fixtureGit(t, o.Origin, "show-ref") != before {
		t.Fatal("purpose refusal changed origin state")
	}
}
