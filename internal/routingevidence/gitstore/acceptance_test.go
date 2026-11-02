package gitstore

import (
	"context"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/routingevidence"
)

// This is the fresh-clone spine for T12. It deliberately crosses the real Git
// adapter for every durable event and discards the producing adapter before
// checking the complete route, pass/finding composite, disposition, and review
// process from a new client with no cache or proposal state.
func TestAcceptanceFreshCloneReconstructsCompleteRouteAndReview(t *testing.T) {
	ctx := context.Background()
	fx := newFixture(t, true)
	writer := fx.adapter(t, fx.first)
	head := fx.genesisHead()

	demand := fx.demand("demand-review", "task-review")
	head = appendOne(t, ctx, writer, head, demand)
	resolution := fx.reviewResolution(demand)
	head = appendOne(t, ctx, writer, head, resolution)
	outcome := fx.reviewOutcome(resolution)
	head = appendOne(t, ctx, writer, head, outcome)
	pass, finding := fx.reviewPass(resolution, outcome)
	proof, err := writer.Append(ctx, routingevidence.AppendRequest{ObservedHead: head, Composite: &routingevidence.AtomicComposite{Events: []routingevidence.Event{pass, finding}}})
	if err != nil || len(proof.Entries) != 2 {
		t.Fatalf("atomic pass/finding = %+v, %v", proof, err)
	}
	head = routingevidence.ObservedHead{EvidenceRef: proof.EvidenceRef, Commit: proof.Commit}
	disposition := fx.disposition(finding)
	head = appendOne(t, ctx, writer, head, disposition)
	process := fx.reviewProcess(pass)
	_ = appendOne(t, ctx, writer, head, process)

	fresh := fx.adapter(t, fx.newClient(t, "fresh-acceptance"))
	reader, err := routingevidence.NewReader(fresh, routingevidence.NewMemoryViewCache())
	if err != nil {
		t.Fatal(err)
	}
	verifier, err := routingevidence.NewFreshCloneVerifier(reader)
	if err != nil {
		t.Fatal(err)
	}
	report, err := verifier.Verify(ctx, routingevidence.SubjectID{Kind: routingevidence.SubjectProcess, ID: "process-review"})
	if err != nil {
		t.Fatal(err)
	}
	if !report.Accepting || report.State != string(routingevidence.StateProcessTerminal) || report.ProofEntries != 7 || report.ExactGitObject == "" {
		t.Fatalf("fresh-clone report = %+v", report)
	}
	if wire, err := routingevidence.EncodeOperatorDiagnostic(report); err != nil || len(wire) == 0 {
		t.Fatalf("reviewable fresh-clone report = %q, %v", wire, err)
	}
}

func appendOne(t *testing.T, ctx context.Context, adapter *Adapter, head routingevidence.ObservedHead, event routingevidence.Event) routingevidence.ObservedHead {
	t.Helper()
	proof, err := adapter.Append(ctx, routingevidence.AppendRequest{ObservedHead: head, Event: &event})
	if err != nil {
		t.Fatal(err)
	}
	return routingevidence.ObservedHead{EvidenceRef: proof.EvidenceRef, Commit: proof.Commit}
}

func (f *fixture) reviewResolution(demand routingevidence.Event) routingevidence.Event {
	p := demand.Payload.ExecutionDemandFrozen
	return f.sign(routingevidence.Event{
		EvidenceRef: f.policy.EvidenceRef, Kind: routingevidence.RoutingResolved, Repository: f.policy.Repository, Schema: routingevidence.Schema, Version: routingevidence.EventVersion,
		Payload: routingevidence.Payload{RoutingResolved: &routingevidence.RoutingResolvedPayload{
			AttemptID: "attempt-review", DemandID: p.DemandID, Harness: "codex", NativeProfileID: "codex/strong", OwnerGeneration: 1,
			PolicyVersion: p.PolicyVersion, PromotedComponents: []string{}, ProposalBand: "deep", ProposalID: "proposal-review",
			RequestedProfile: p.Profile, Resolution: "exact", Role: "review", TaskID: p.TaskID,
		}},
	}, routingevidence.ProducerResolver)
}

func (f *fixture) reviewOutcome(resolution routingevidence.Event) routingevidence.Event {
	p := resolution.Payload.RoutingResolved
	return f.sign(routingevidence.Event{
		EvidenceRef: f.policy.EvidenceRef, Kind: routingevidence.RoutedAttemptCompleted, Repository: f.policy.Repository, Schema: routingevidence.Schema, Version: routingevidence.EventVersion,
		Payload: routingevidence.Payload{RoutedAttemptCompleted: &routingevidence.RoutedAttemptCompletedPayload{
			AttemptID: p.AttemptID, Cause: "terminal_verified", DemandID: p.DemandID, EvidenceRef: string(mustID(f.t, resolution)),
			LaunchCapabilityRef: "launch-capability:" + p.AttemptID, LaunchFidelity: "exact", Outcome: "verified", OwnerGeneration: 1,
			ProposalID: p.ProposalID, TaskID: p.TaskID, Usage: routingevidence.Usage{InputTokens: 10, OutputTokens: 2}, UsageProvenance: "runtime_observed",
		}},
	}, routingevidence.ProducerAttemptOwner)
}

func (f *fixture) reviewPass(resolution, outcome routingevidence.Event) (routingevidence.Event, routingevidence.Event) {
	r := resolution.Payload.RoutingResolved
	findingRecord := routingevidence.NormalizedFinding{Acceptance: "A-6", CalibrationRow: "blocking/wrong", FindingID: "finding-review", Lens: "correctness-critic", Message: "fixture finding", RuleID: "DINV-9", Severity: "advisory"}
	pass := f.sign(routingevidence.Event{
		EvidenceRef: f.policy.EvidenceRef, Kind: routingevidence.AssurancePassCompleted, Repository: f.policy.Repository, Schema: routingevidence.Schema, Version: routingevidence.EventVersion,
		Payload: routingevidence.Payload{AssurancePassCompleted: &routingevidence.AssurancePassCompletedPayload{
			AttemptID: r.AttemptID, Candidates: 1, DemandID: r.DemandID, EffectiveDepth: "deep", FindingRecords: []routingevidence.NormalizedFinding{findingRecord}, Findings: 1,
			InternalRounds: 1, Killed: 0, PassID: "pass-review", PassIndex: 1, ProposalID: r.ProposalID,
			Provenance:        routingevidence.Provenance{CalibrationVersion: "cal/v1", Model: "fixture", PromptsVersion: "prompt/v1"},
			ProcessGeneration: 1, RequiredPasses: 1, ReviewProcessID: "process-review", ReviewedRevision: "revision-review",
			RoutedOutcomeRef: string(mustID(f.t, outcome)), RoutingResolutionRef: string(mustID(f.t, resolution)), TaskID: r.TaskID,
			TopLevelContextID: "context-review", Verdict: "approve-with-advisories", WallMillis: 1,
		}},
	}, routingevidence.ProducerReviewOrchestrator)
	finding := f.sign(routingevidence.Event{
		EvidenceRef: f.policy.EvidenceRef, Kind: routingevidence.FindingPersisted, Repository: f.policy.Repository, Schema: routingevidence.Schema, Version: routingevidence.EventVersion,
		Payload: routingevidence.Payload{FindingPersisted: &routingevidence.FindingPersistedPayload{NormalizedFinding: findingRecord, PassID: "pass-review", ProposalID: r.ProposalID, ReviewProcessID: "process-review", ReviewedRevision: "revision-review"}},
	}, routingevidence.ProducerReviewOrchestrator)
	return pass, finding
}

func (f *fixture) disposition(finding routingevidence.Event) routingevidence.Event {
	return f.sign(routingevidence.Event{
		EvidenceRef: f.policy.EvidenceRef, Kind: routingevidence.DispositionRecorded, Repository: f.policy.Repository, Schema: routingevidence.Schema, Version: routingevidence.EventVersion,
		Payload: routingevidence.Payload{DispositionRecorded: &routingevidence.DispositionRecordedPayload{
			Actor: "worker", ActorAttribution: "github:worker", AttributionVerification: "verified", FindingID: finding.Payload.FindingPersisted.FindingID,
			NativeAuthorIdentityProof: "native-proof", NativeMediumActRef: "github:comment-1", Verdict: "applied",
		}},
	}, routingevidence.ProducerMediumAdapter)
}

func (f *fixture) reviewProcess(pass routingevidence.Event) routingevidence.Event {
	p := pass.Payload.AssurancePassCompleted
	return f.sign(routingevidence.Event{
		EvidenceRef: f.policy.EvidenceRef, Kind: routingevidence.ReviewProcessCompleted, Repository: f.policy.Repository, Schema: routingevidence.Schema, Version: routingevidence.EventVersion,
		Payload: routingevidence.Payload{ReviewProcessCompleted: &routingevidence.ReviewProcessCompletedPayload{
			AggregateVerdict: p.Verdict, AggregateWallMillis: p.WallMillis, CompletedPasses: 1, DemandID: p.DemandID, EffectiveDepth: p.EffectiveDepth,
			EvidenceRef: string(mustID(f.t, pass)), PassAttemptIDs: []string{p.AttemptID}, PassVerdicts: []string{p.Verdict}, PlannedAssurance: f.profile.Assurance,
			ProcessGeneration: 1, ProposalBand: "deep", ProposalID: p.ProposalID, RequiredPasses: 1, Result: "satisfied", ReviewProcessID: p.ReviewProcessID,
			ReviewedRevision: p.ReviewedRevision, TaskID: p.TaskID, TopLevelContextIDs: []string{p.TopLevelContextID},
		}},
	}, routingevidence.ProducerReviewOrchestrator)
}
