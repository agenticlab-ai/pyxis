package routingevidence

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEventFamilyGoldenAndIndependentProducerConformance(t *testing.T) {
	producerA := fixtureEvents(false)
	producerB := fixtureEvents(true) // same sets, deliberately different input order
	if os.Getenv("UPDATE_ROUTING_EVIDENCE_GOLDEN") == "1" {
		goldens := make([]golden, len(producerA))
		for i, event := range producerA {
			goldens[i] = eventStages(t, event)
		}
		data, err := json.MarshalIndent(goldens, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		data = append(data, '\n')
		if err := os.WriteFile(filepath.Join("testdata", "event_family_golden.json"), data, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	goldenBytes, err := os.ReadFile(filepath.Join("testdata", "event_family_golden.json"))
	if err != nil {
		t.Fatal(err)
	}
	var goldens []golden
	if err := json.Unmarshal(goldenBytes, &goldens); err != nil {
		t.Fatal(err)
	}
	// Retain the historical wire fixture to prove that it cannot be admitted.
	current := goldens[:0]
	for _, g := range goldens {
		if g.Kind == "legacy_execution_demand_translated" {
			wire, err := hex.DecodeString(g.CanonicalHex)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := DecodeCanonical(wire); err == nil {
				t.Fatal("accepted retired demand event")
			}
			continue
		}
		current = append(current, g)
	}
	goldens = current
	if len(goldens) != len(producerA) {
		t.Fatalf("golden count = %d, want %d", len(goldens), len(producerA))
	}
	for i := range producerA {
		a, b := producerA[i], producerB[i]
		t.Run(string(a.Kind), func(t *testing.T) {
			if a.Kind != b.Kind || goldens[i].Kind != a.Kind {
				t.Fatalf("fixture order mismatch: %s/%s/%s", a.Kind, b.Kind, goldens[i].Kind)
			}
			stagesA := eventStages(t, a)
			stagesB := eventStages(t, b)
			if stagesA != stagesB {
				t.Fatalf("independent producers differed:\nA=%+v\nB=%+v", stagesA, stagesB)
			}
			if stagesA != goldens[i] {
				t.Fatalf("canonical golden drift:\n got %+v\nwant %+v", stagesA, goldens[i])
			}
			wire, err := hex.DecodeString(stagesA.CanonicalHex)
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := DecodeCanonical(wire)
			if err != nil {
				t.Fatalf("decode canonical: %v", err)
			}
			roundTrip, err := EncodeCanonical(decoded)
			if err != nil || !bytes.Equal(roundTrip, wire) {
				t.Fatalf("round trip: err=%v equal=%v", err, bytes.Equal(roundTrip, wire))
			}
		})
	}
}

func TestIdentityContentAndProofExclusions(t *testing.T) {
	for _, original := range fixtureEvents(false) {
		t.Run(string(original.Kind), func(t *testing.T) {
			id1, err := Identify(original)
			if err != nil {
				t.Fatal(err)
			}
			for i, mutate := range identityMutations(original.Kind) {
				identityMutation := fixtureEvents(false)[kindIndex(original.Kind)]
				mutate(&identityMutation)
				id2, err := Identify(identityMutation)
				if err != nil {
					t.Fatalf("identity mutation %d became invalid: %v", i, err)
				}
				if id1 == id2 {
					t.Fatalf("identity-bearing mutation %d did not change event_id", i)
				}
			}

			contentMutation := fixtureEvents(false)[kindIndex(original.Kind)]
			mutateNonIdentity(&contentMutation)
			id3, err := Identify(contentMutation)
			if err != nil || id3 != id1 {
				t.Fatalf("non-identity mutation changed event_id: id=%s err=%v", id3, err)
			}
			digest1, err := Digest(original)
			if err != nil {
				t.Fatal(err)
			}
			digest2, err := Digest(contentMutation)
			if err != nil || digest1 == digest2 {
				t.Fatalf("content mutation did not change digest: %s/%s err=%v", digest1, digest2, err)
			}

			proofMutation := fixtureEvents(false)[kindIndex(original.Kind)]
			proofMutation.ProducerProof.Signature += "ff"
			unsigned1, err := EncodeUnsignedEnvelope(original)
			if err != nil {
				t.Fatal(err)
			}
			unsigned2, err := EncodeUnsignedEnvelope(proofMutation)
			if err != nil || !bytes.Equal(unsigned1, unsigned2) {
				t.Fatalf("proof entered unsigned envelope: err=%v", err)
			}
			wire1, _ := EncodeCanonical(original)
			wire2, _ := EncodeCanonical(proofMutation)
			if bytes.Equal(wire1, wire2) {
				t.Fatal("proof mutation did not change stored envelope")
			}
		})
	}
}

func TestRetryHasNoAmbientTimeInput(t *testing.T) {
	first := fixtureEvents(false)
	second := fixtureEvents(false)
	for i := range first {
		a := eventStages(t, first[i])
		b := eventStages(t, second[i])
		if a != b {
			t.Fatalf("%s semantic retry changed with no source-owned input change", first[i].Kind)
		}
	}
	// The sole admitted time-shaped value is the disposition source's opaque
	// act value. It is content, not identity, and is never generated here.
	disposition := first[kindIndex(DispositionRecorded)]
	originalID, _ := Identify(disposition)
	originalDigest, _ := Digest(disposition)
	disposition.Payload.DispositionRecorded.SourceOwnedAt = "medium-sequence:43"
	changedID, _ := Identify(disposition)
	changedDigest, _ := Digest(disposition)
	if originalID != changedID || originalDigest == changedDigest {
		t.Fatal("source-owned opaque value did not obey identity/content exclusions")
	}
}

func TestCanonicalSurfacesHaveNoForbiddenPayloadSlots(t *testing.T) {
	for _, event := range fixtureEvents(false) {
		stages := [][]byte{}
		identity, err := EncodeIdentityPreimage(event)
		if err != nil {
			t.Fatal(err)
		}
		content, err := EncodeContentPreimage(event)
		if err != nil {
			t.Fatal(err)
		}
		unsigned, err := EncodeUnsignedEnvelope(event)
		if err != nil {
			t.Fatal(err)
		}
		stored, err := EncodeCanonical(event)
		if err != nil {
			t.Fatal(err)
		}
		stages = append(stages, identity, content, unsigned, stored)
		for _, stage := range stages {
			for _, key := range []string{"prompt", "raw_model_output", "source", "diff", "credential", "session_transcript", "ambient_clock", "git_metadata"} {
				if bytes.Contains(stage, []byte(`"`+key+`":`)) {
					t.Fatalf("%s admitted forbidden key %q in %s", event.Kind, key, stage)
				}
			}
		}
	}
}

func TestDecodeCanonicalRejectsForeignOrNonCanonicalInput(t *testing.T) {
	valid, err := EncodeCanonical(fixtureEvents(false)[kindIndex(RoutingResolved)])
	if err != nil {
		t.Fatal(err)
	}
	s := string(valid)
	digest, err := Digest(fixtureEvents(false)[kindIndex(RoutingResolved)])
	if err != nil {
		t.Fatal(err)
	}
	canonicalPrefix := `{"content_digest":"` + digest + `","domain":"` + CanonicalDomain + `"`
	reorderedPrefix := `{"domain":"` + CanonicalDomain + `","content_digest":"` + digest + `"`
	cases := map[string][]byte{
		"unknown key":       []byte(strings.Replace(s, "{", `{"ambient_clock":"forbidden",`, 1)),
		"unknown kind":      []byte(strings.Replace(s, `"event_kind":"routing_resolved"`, `"event_kind":"future_kind"`, 1)),
		"unknown version":   []byte(strings.Replace(s, `"event_version":"1"`, `"event_version":"2"`, 1)),
		"null":              []byte(strings.Replace(s, `"producer":{"class":"runtime-adapter","generation":"adapter/v7"}`, `"producer":null`, 1)),
		"float":             []byte(strings.Replace(s, `"owner_generation":1`, `"owner_generation":1.0`, 1)),
		"duplicate field":   []byte(strings.Replace(s, `{"content_digest":`, `{"content_digest":"duplicate","content_digest":`, 1)),
		"noncanonical keys": []byte(strings.Replace(s, canonicalPrefix, reorderedPrefix, 1)),
		"recursive digest":  []byte(strings.Replace(s, `"attempt_id":"attempt-001","demand_id":`, `"attempt_id":"attempt-001","content_digest":"recursive","demand_id":`, 1)),
		"recursive proof":   []byte(strings.Replace(s, `"policy_version":"routing-policy/v4","promoted_components":`, `"policy_version":"routing-policy/v4","producer_proof":{"schema":"recursive"},"promoted_components":`, 1)),
		"trailing value":    append(append([]byte(nil), valid...), []byte(`{}`)...),
		"invalid utf8":      append(append([]byte(nil), valid[:len(valid)-1]...), 0xff, '}'),
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := DecodeCanonical(input)
			if err == nil || got != (Event{}) {
				t.Fatalf("accepted invalid input: event=%#v err=%v", got, err)
			}
		})
	}

	ordered := fixtureEvents(false)[kindIndex(RoutingResolved)]
	wire, err := EncodeCanonical(ordered)
	if err != nil {
		t.Fatal(err)
	}
	badSetOrder := bytes.Replace(wire, []byte(`["capability","context"]`), []byte(`["context","capability"]`), 1)
	if bytes.Equal(badSetOrder, wire) {
		t.Fatal("fixture did not contain promoted-components set")
	}
	if _, err := DecodeCanonical(badSetOrder); err == nil {
		t.Fatal("accepted non-canonical set ordering")
	}
}

func TestDerivedFieldAndPathValidation(t *testing.T) {
	e := fixtureEvents(false)[0]
	e.EventID = EventID(strings.Repeat("0", 64))
	if _, err := EncodeCanonical(e); err == nil {
		t.Fatal("accepted recursive/mismatched event_id")
	}
	e = fixtureEvents(false)[0]
	e.ContentDigest = strings.Repeat("0", 64)
	if _, err := EncodeCanonical(e); err == nil {
		t.Fatal("accepted recursive/mismatched content_digest")
	}
	for _, bad := range []EventID{"", "ABC", EventID(strings.Repeat("A", 64)), EventID(strings.Repeat("z", 64))} {
		if _, err := EventPath(bad); err == nil {
			t.Fatalf("EventPath accepted %q", bad)
		}
	}
}

type golden struct {
	Kind           EventKind `json:"kind"`
	EventID        EventID   `json:"event_id"`
	ContentDigest  string    `json:"content_digest"`
	Path           string    `json:"path"`
	IdentitySHA256 string    `json:"identity_sha256"`
	ContentSHA256  string    `json:"content_sha256"`
	UnsignedSHA256 string    `json:"unsigned_sha256"`
	CanonicalHex   string    `json:"canonical_hex"`
}

func eventStages(t *testing.T, e Event) golden {
	t.Helper()
	identity, err := EncodeIdentityPreimage(e)
	if err != nil {
		t.Fatal(err)
	}
	content, err := EncodeContentPreimage(e)
	if err != nil {
		t.Fatal(err)
	}
	unsigned, err := EncodeUnsignedEnvelope(e)
	if err != nil {
		t.Fatal(err)
	}
	wire, err := EncodeCanonical(e)
	if err != nil {
		t.Fatal(err)
	}
	id, err := Identify(e)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := Digest(e)
	if err != nil {
		t.Fatal(err)
	}
	p, err := EventPath(id)
	if err != nil {
		t.Fatal(err)
	}
	return golden{Kind: e.Kind, EventID: id, ContentDigest: digest, Path: p,
		IdentitySHA256: shaHex(identity), ContentSHA256: shaHex(content),
		UnsignedSHA256: shaHex(unsigned), CanonicalHex: hex.EncodeToString(wire)}
}

func shaHex(b []byte) string { sum := sha256.Sum256(b); return hex.EncodeToString(sum[:]) }

func kindIndex(kind EventKind) int {
	for i, event := range fixtureEvents(false) {
		if event.Kind == kind {
			return i
		}
	}
	panic("missing fixture " + string(kind))
}

func identityMutations(kind EventKind) []func(*Event) {
	switch kind {
	case ExecutionDemandFrozen:
		return []func(*Event){func(e *Event) { e.Payload.ExecutionDemandFrozen.DemandID += "-other" }}

	case RoutingResolved:
		return []func(*Event){func(e *Event) { e.Payload.RoutingResolved.AttemptID += "-other" }}
	case RoutingResolutionFailed:
		return []func(*Event){func(e *Event) { e.Payload.RoutingResolutionFailed.AttemptID += "-other" }}
	case RoutedAttemptCompleted:
		return []func(*Event){func(e *Event) { e.Payload.RoutedAttemptCompleted.AttemptID += "-other" }}
	case RoutedTaskOutcome:
		return []func(*Event){func(e *Event) { e.Payload.RoutedTaskOutcome.AttemptID += "-other" }}
	case AttemptOwnerTransferred:
		return []func(*Event){
			func(e *Event) { e.Payload.AttemptOwnerTransferred.AttemptID += "-other" },
			func(e *Event) {
				e.Payload.AttemptOwnerTransferred.NewOwnerGeneration++
				e.Payload.AttemptOwnerTransferred.PriorOwnerGeneration++
			},
		}
	case AssurancePassCompleted:
		return []func(*Event){
			func(e *Event) { e.Payload.AssurancePassCompleted.ReviewProcessID += "-other" },
			func(e *Event) { e.Payload.AssurancePassCompleted.ProcessGeneration++ },
			func(e *Event) { e.Payload.AssurancePassCompleted.PassID += "-other" },
		}
	case FindingPersisted:
		return []func(*Event){
			func(e *Event) { e.Payload.FindingPersisted.PassID += "-other" },
			func(e *Event) { e.Payload.FindingPersisted.FindingID += "-other" },
		}
	case DispositionRecorded:
		return []func(*Event){
			func(e *Event) { e.Payload.DispositionRecorded.FindingID += "-other" },
			func(e *Event) { e.Payload.DispositionRecorded.NativeMediumActRef += "-other" },
		}
	case ReviewProcessCompleted:
		return []func(*Event){
			func(e *Event) { e.Payload.ReviewProcessCompleted.ReviewProcessID += "-other" },
			func(e *Event) { e.Payload.ReviewProcessCompleted.ProcessGeneration++ },
		}
	case RoutingEvidenceInvalidated:
		return []func(*Event){func(e *Event) {
			e.Payload.RoutingEvidenceInvalidated.AffectedEventIDs = append(e.Payload.RoutingEvidenceInvalidated.AffectedEventIDs, "evt-other")
		}}
	case ActivationCheckpointEvent:
		return []func(*Event){
			func(e *Event) { e.Payload.ActivationCheckpoint.ActivationPolicyVersion += "-other" },
			func(e *Event) { e.Payload.ActivationCheckpoint.ExactMainlineObject += "-other" },
			func(e *Event) {
				e.Payload.ActivationCheckpoint.ProofManifestDigest = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
			},
		}
	default:
		panic("missing identity mutations for " + string(kind))
	}
}

func mutateNonIdentity(e *Event) {
	switch e.Kind {
	case ExecutionDemandFrozen:
		e.Payload.ExecutionDemandFrozen.TaskID += "-other"

	case RoutingResolved:
		e.Payload.RoutingResolved.Harness += "-other"
	case RoutingResolutionFailed:
		e.Payload.RoutingResolutionFailed.Harness += "-other"
	case RoutedAttemptCompleted:
		e.Payload.RoutedAttemptCompleted.EvidenceRef += "-other"
	case RoutedTaskOutcome:
		e.Payload.RoutedTaskOutcome.EvidenceRef += "-other"
	case AttemptOwnerTransferred:
		e.Payload.AttemptOwnerTransferred.RuntimeFenceNonce += "-other"
	case AssurancePassCompleted:
		e.Payload.AssurancePassCompleted.WallMillis++
	case FindingPersisted:
		e.Payload.FindingPersisted.Message += " clarified"
	case DispositionRecorded:
		e.Payload.DispositionRecorded.Reason = "recorded rationale"
	case ReviewProcessCompleted:
		e.Payload.ReviewProcessCompleted.AggregateWallMillis++
	case RoutingEvidenceInvalidated:
		e.Producer.Generation += "-other"
	case ActivationCheckpointEvent:
		e.Payload.ActivationCheckpoint.Reviews++
	}
}

func fixtureEvents(reverseSets bool) []Event {
	set := func(values ...string) []string {
		if reverseSets {
			for i, j := 0, len(values)-1; i < j; i, j = i+1, j-1 {
				values[i], values[j] = values[j], values[i]
			}
		}
		return values
	}
	profile := Profile{Assurance: "critical", Capability: "frontier", Context: "extended", Deliberation: "maximum"}
	assessment := Assessment{ArchitecturalRole: "system_core", Coupling: "cross_boundary", ImpactScope: "system_wide", Novelty: "first_of_kind", Reasoning: "difficult", Verification: "cross_boundary"}
	usage := Usage{CacheCreationTokens: 11, CacheReadTokens: 22, InputTokens: 333, OutputTokens: 44}
	findingA := NormalizedFinding{Acceptance: "A-1", CalibrationRow: "CR-7", FindingID: "FND-1", Lens: "spec-fidelity", Line: 21, LogRef: "artifact:review-log", Message: "Canonical ordering is incomplete.", Path: "internal/example.go", RuleID: "SF-1", Severity: "blocking"}
	findingB := NormalizedFinding{Acceptance: "A-1", CalibrationRow: "CR-9", FindingID: "FND-2", Lens: "security-abuse", Message: "A foreign proof is admitted.", RuleID: "SA-2", Severity: "advisory"}
	base := func(kind EventKind, payload Payload) Event {
		return Event{EvidenceRef: "refs/pyxis/routing-evidence", Kind: kind, Payload: payload, Producer: Producer{Class: "runtime-adapter", Generation: "adapter/v7"}, ProducerProof: ProducerProof{Algorithm: "ed25519", KeyID: "key-2026-08", Schema: ProducerProofSchema, Signature: "4f5e6d"}, Repository: "agenticlab-ai/pyxis", Schema: Schema, Version: EventVersion}
	}
	return []Event{
		base(ExecutionDemandFrozen, Payload{ExecutionDemandFrozen: &ExecutionDemandFrozenPayload{Assessment: assessment, AssessmentEvidenceRefs: set("plan:T1#assessment", "plan:T1#routing"), AssuranceRaise: true, DemandID: "demand-001", PlanVersion: "003", PolicyVersion: "routing-policy/v4", Profile: profile, TaskID: "planner-driven-model-routing/003/T1"}}),
		base(RoutingResolved, Payload{RoutingResolved: &RoutingResolvedPayload{AssuranceJoinRaise: true, AttemptID: "attempt-001", DemandID: "demand-001", Harness: "codex", NativeProfileID: "codex/frontier/max", OwnerGeneration: 1, PolicyVersion: "routing-policy/v4", PromotedComponents: set("capability", "context"), ProposalBand: "deep", ProposalID: "proposal-42", RequestedProfile: profile, Resolution: "promoted", Role: "implementation", TaskID: "planner-driven-model-routing/003/T1"}}),
		base(RoutingResolutionFailed, Payload{RoutingResolutionFailed: &RoutingResolutionFailedPayload{AssuranceJoinRaise: false, AttemptID: "attempt-failed-001", DemandID: "demand-002", Harness: "cursor", PolicyVersion: "routing-policy/v4", Reason: "unsupported", RequestedProfile: profile, Role: "review", TaskID: "planner-driven-model-routing/003/T1", UnmetComponents: set("assurance", "context")}}),
		base(RoutedAttemptCompleted, Payload{RoutedAttemptCompleted: &RoutedAttemptCompletedPayload{AttemptID: "attempt-001", Cause: "terminal_verified", DemandID: "demand-001", EvidenceRef: "evidence:routing-resolution", LaunchCapabilityRef: "launch-capability-001", LaunchFidelity: "exact", Outcome: "verified", OwnerGeneration: 1, ProposalID: "proposal-42", TaskID: "planner-driven-model-routing/003/T1", Usage: usage, UsageProvenance: "runtime_observed"}}),
		base(RoutedTaskOutcome, Payload{RoutedTaskOutcome: &RoutedTaskOutcomePayload{AttemptID: "attempt-001", Cause: "terminal_verified", Class: "succeeded", DemandID: "demand-001", EvidenceRef: "evidence:routed-attempt", LaunchCapabilityRef: "launch-capability-001", LaunchFidelity: "exact", OwnerGeneration: 1, ProposalRef: "proposal-42", TaskID: "planner-driven-model-routing/003/T1", Turns: 7, Usage: usage, UsageProvenance: "runtime_observed", Verification: "verified"}}),
		base(AttemptOwnerTransferred, Payload{AttemptOwnerTransferred: &AttemptOwnerTransferredPayload{AttemptID: "attempt-003", NewOwnerGeneration: 2, Observation: "absent_proven", PriorOwnerGeneration: 1, RuntimeFenceNonce: "nonce-7", RuntimeFenceProofRef: "fence-proof-7"}}),
		base(AssurancePassCompleted, Payload{AssurancePassCompleted: &AssurancePassCompletedPayload{AttemptID: "review-attempt-1", Candidates: 3, DemandID: "review-demand-1", EffectiveDepth: "deep", FindingRecords: findings(reverseSets, findingA, findingB), Findings: 2, InternalRounds: 2, Killed: 1, PassID: "pass-1", PassIndex: 1, ProcessGeneration: 2, ProposalID: "proposal-42", Provenance: Provenance{CalibrationVersion: "cal/v3", Model: "gpt-5.6", PromptsVersion: "prompts/v9"}, RequiredPasses: 2, ReviewProcessID: "review-process-1", ReviewedRevision: "abc123", RoutedOutcomeRef: "event:outcome", RoutingResolutionRef: "event:resolution", TaskID: "planner-driven-model-routing/003/T1", Tokens: 1440, TopLevelContextID: "context-1", Verdict: "request-changes", WallMillis: 9090}}),
		base(FindingPersisted, Payload{FindingPersisted: &FindingPersistedPayload{NormalizedFinding: findingA, PassID: "pass-1", ProposalID: "proposal-42", ReviewProcessID: "review-process-1", ReviewedRevision: "abc123"}}),
		base(DispositionRecorded, Payload{DispositionRecorded: &DispositionRecordedPayload{Actor: "worker", ActorAttribution: "github:user-7", AttributionVerification: "verified", FindingID: "FND-1", NativeAuthorIdentityProof: "author-proof-7", NativeMediumActRef: "github:review-comment-91", SourceOwnedAt: "medium-sequence:42", Verdict: "applied"}}),
		base(ReviewProcessCompleted, Payload{ReviewProcessCompleted: &ReviewProcessCompletedPayload{AggregateVerdict: "request-changes", AggregateWallMillis: 12000, CompletedPasses: 1, DemandID: "review-demand-1", EffectiveDepth: "deep", EvidenceRef: "evidence:pass-set", PassAttemptIDs: []string{"review-attempt-1"}, PassVerdicts: []string{"request-changes"}, PlannedAssurance: "critical", ProcessGeneration: 2, ProposalBand: "deep", ProposalID: "proposal-42", RequiredPasses: 2, Result: "floor_violation", ReviewProcessID: "review-process-1", ReviewedRevision: "abc123", TaskID: "planner-driven-model-routing/003/T1", TopLevelContextIDs: []string{"context-1"}}}),
		base(RoutingEvidenceInvalidated, Payload{RoutingEvidenceInvalidated: &RoutingEvidenceInvalidatedPayload{AffectedArtifactRefs: set("artifact:report", "artifact:proposal"), AffectedAttemptIDs: set("attempt-1", "attempt-2"), AffectedEventIDs: set("evt-1", "evt-2"), AffectedProcessIDs: set("process-1", "process-2"), AuthorizationRef: "operator-decision-7", Disposition: "rerun_required", NativeProfileID: "codex/frontier/max", PolicyVersion: "routing-policy/v4"}}),
		base(ActivationCheckpointEvent, Payload{ActivationCheckpoint: &ActivationCheckpointPayload{Activated: false, ActivationPolicyVersion: "activation/v2", ActivationReady: false, AvailabilityDefects: 1, BlockingDisposed: 12, BurstFlag: false, CoveragePct: "98.5", DwellP50: "42.25", EvidenceHead: "evidence-head-1", ExactMainlineObject: "main:abc123", LaunchReadinessGreen: true, LiveBlockingDODGreen: true, MainlineRef: "refs/heads/main", MatrixCellsCovered: 19, MatrixCellsRequired: 20, NitP50: "0.5", OperatorAuthorizationID: "auth-activation", ParentSeamGreen: true, Precision: "0.975", ProcessViolations: 0, ProofIDs: set("proof:matrix", "proof:reviews"), ProofManifestDigest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Reviews: 10}}),
	}
}

func findings(reverse bool, values ...NormalizedFinding) []NormalizedFinding {
	if reverse {
		for i, j := 0, len(values)-1; i < j; i, j = i+1, j-1 {
			values[i], values[j] = values[j], values[i]
		}
	}
	return values
}
