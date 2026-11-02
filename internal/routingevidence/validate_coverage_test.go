package routingevidence

import (
	"reflect"
	"strings"
	"testing"
)

type payloadMutation struct {
	name   string
	kind   EventKind
	mutate func(any)
}

func TestValidateEventBaseRejectsEveryEnvelopeBoundary(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Event)
	}{
		{"oversized field", func(e *Event) { e.Repository = strings.Repeat("x", maxStringBytes+1) }},
		{"schema", func(e *Event) { e.Schema = "other" }},
		{"version", func(e *Event) { e.Version = "other" }},
		{"repository", func(e *Event) { e.Repository = "" }},
		{"evidence ref", func(e *Event) { e.EvidenceRef = "" }},
		{"producer class", func(e *Event) { e.Producer.Class = "" }},
		{"producer generation", func(e *Event) { e.Producer.Generation = "" }},
		{"proof schema", func(e *Event) { e.ProducerProof.Schema = "other" }},
		{"proof key", func(e *Event) { e.ProducerProof.KeyID = "" }},
		{"proof signature", func(e *Event) { e.ProducerProof.Signature = "" }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := fixtureEvents(false)[0]
			tc.mutate(&e)
			if err := validateEventBase(e, true, true); err == nil {
				t.Fatal("mutated envelope was accepted")
			}
		})
	}
}

func TestSelectedAndNormalizedPayloadRejectUnionDefects(t *testing.T) {
	if _, err := selectedPayload(Event{}); err == nil {
		t.Fatal("empty union was accepted")
	}
	e := fixtureEvents(false)[0]
	e.Kind = EventKind("unknown")
	if _, err := selectedPayload(e); err == nil {
		t.Fatal("unknown event kind was accepted")
	}
	e = fixtureEvents(false)[0]
	e.Kind = RoutingResolved
	if _, err := selectedPayload(e); err == nil {
		t.Fatal("kind/member mismatch was accepted")
	}
	if !nilPayload("unsupported") {
		t.Fatal("unsupported payload type should be nil-equivalent")
	}

	e = fixtureEvents(false)[kindIndex(RoutingEvidenceInvalidated)]
	normalized, err := normalizedPayload(e)
	if err != nil {
		t.Fatal(err)
	}
	p := normalized.(*RoutingEvidenceInvalidatedPayload)
	p.InvalidationID = strings.Repeat("0", 64)
	e.Payload.RoutingEvidenceInvalidated = p
	if _, err := normalizedPayload(e); err == nil {
		t.Fatal("non-canonical invalidation identity was accepted")
	}
}

func TestValidatePayloadRejectsMalformedEventFamily(t *testing.T) {
	tests := []payloadMutation{
		// Execution demand.
		{"demand required", ExecutionDemandFrozen, func(v any) { v.(*ExecutionDemandFrozenPayload).DemandID = "" }},
		{"demand assessment", ExecutionDemandFrozen, func(v any) { v.(*ExecutionDemandFrozenPayload).Assessment.Reasoning = "unknown" }},
		{"demand evidence absent", ExecutionDemandFrozen, func(v any) { v.(*ExecutionDemandFrozenPayload).AssessmentEvidenceRefs = nil }},
		{"demand evidence empty", ExecutionDemandFrozen, func(v any) { v.(*ExecutionDemandFrozenPayload).AssessmentEvidenceRefs = []string{} }},
		{"demand evidence duplicate", ExecutionDemandFrozen, func(v any) { v.(*ExecutionDemandFrozenPayload).AssessmentEvidenceRefs = []string{"same", "same"} }},
		{"demand profile", ExecutionDemandFrozen, func(v any) { v.(*ExecutionDemandFrozenPayload).Profile.Capability = "unknown" }},

		// Legacy translation.

		// Successful and failed routing.
		{"resolved required", RoutingResolved, func(v any) { v.(*RoutingResolvedPayload).AttemptID = "" }},
		{"resolved enum", RoutingResolved, func(v any) { v.(*RoutingResolvedPayload).Role = "unknown" }},
		{"resolved generation", RoutingResolved, func(v any) { v.(*RoutingResolvedPayload).OwnerGeneration = 2 }},
		{"resolved proposal band", RoutingResolved, func(v any) { v.(*RoutingResolvedPayload).ProposalBand = "unknown" }},
		{"resolved components absent", RoutingResolved, func(v any) { v.(*RoutingResolvedPayload).PromotedComponents = nil }},
		{"resolved components duplicate", RoutingResolved, func(v any) { v.(*RoutingResolvedPayload).PromotedComponents = []string{"context", "context"} }},
		{"resolved exact promotion", RoutingResolved, func(v any) { v.(*RoutingResolvedPayload).Resolution = "exact" }},
		{"resolved profile", RoutingResolved, func(v any) { v.(*RoutingResolvedPayload).RequestedProfile.Context = "unknown" }},
		{"failed required", RoutingResolutionFailed, func(v any) { v.(*RoutingResolutionFailedPayload).AttemptID = "" }},
		{"failed role", RoutingResolutionFailed, func(v any) { v.(*RoutingResolutionFailedPayload).Role = "unknown" }},
		{"failed reason", RoutingResolutionFailed, func(v any) { v.(*RoutingResolutionFailedPayload).Reason = "unknown" }},
		{"failed proposal band", RoutingResolutionFailed, func(v any) { v.(*RoutingResolutionFailedPayload).ProposalBand = "unknown" }},
		{"failed components absent", RoutingResolutionFailed, func(v any) { v.(*RoutingResolutionFailedPayload).UnmetComponents = nil }},
		{"failed profile", RoutingResolutionFailed, func(v any) { v.(*RoutingResolutionFailedPayload).RequestedProfile.Deliberation = "unknown" }},

		// Attempt and task outcomes.
		{"attempt required", RoutedAttemptCompleted, func(v any) { v.(*RoutedAttemptCompletedPayload).Cause = "" }},
		{"attempt outcome", RoutedAttemptCompleted, func(v any) { v.(*RoutedAttemptCompletedPayload).Outcome = "unknown" }},
		{"attempt fidelity", RoutedAttemptCompleted, func(v any) { v.(*RoutedAttemptCompletedPayload).LaunchFidelity = "unknown" }},
		{"attempt generation", RoutedAttemptCompleted, func(v any) { v.(*RoutedAttemptCompletedPayload).OwnerGeneration = 0 }},
		{"task required", RoutedTaskOutcome, func(v any) { v.(*RoutedTaskOutcomePayload).DemandID = "" }},
		{"task enum", RoutedTaskOutcome, func(v any) { v.(*RoutedTaskOutcomePayload).Class = "unknown" }},
		{"task mapping", RoutedTaskOutcome, func(v any) { v.(*RoutedTaskOutcomePayload).Verification = "failed" }},
		{"task generation", RoutedTaskOutcome, func(v any) { v.(*RoutedTaskOutcomePayload).OwnerGeneration = 0 }},

		// Ownership and assurance.
		{"transfer required", AttemptOwnerTransferred, func(v any) { v.(*AttemptOwnerTransferredPayload).RuntimeFenceNonce = "" }},
		{"transfer generation", AttemptOwnerTransferred, func(v any) { v.(*AttemptOwnerTransferredPayload).NewOwnerGeneration = 3 }},
		{"transfer observation", AttemptOwnerTransferred, func(v any) { v.(*AttemptOwnerTransferredPayload).Observation = "running" }},
		{"pass required", AssurancePassCompleted, func(v any) { v.(*AssurancePassCompletedPayload).PassID = "" }},
		{"pass enum", AssurancePassCompleted, func(v any) { v.(*AssurancePassCompletedPayload).EffectiveDepth = "unknown" }},
		{"pass count", AssurancePassCompleted, func(v any) { v.(*AssurancePassCompletedPayload).PassIndex = 0 }},
		{"pass findings absent", AssurancePassCompleted, func(v any) { v.(*AssurancePassCompletedPayload).FindingRecords = nil }},
		{"pass findings count", AssurancePassCompleted, func(v any) { v.(*AssurancePassCompletedPayload).Findings = 1 }},
		{"pass invalid finding", AssurancePassCompleted, func(v any) { v.(*AssurancePassCompletedPayload).FindingRecords[0].Lens = "unknown" }},
		{"pass duplicate finding", AssurancePassCompleted, func(v any) {
			p := v.(*AssurancePassCompletedPayload)
			p.FindingRecords[1].FindingID = p.FindingRecords[0].FindingID
		}},
		{"pass provenance", AssurancePassCompleted, func(v any) { v.(*AssurancePassCompletedPayload).Provenance.Model = "" }},

		// Findings and dispositions.
		{"finding required", FindingPersisted, func(v any) { v.(*FindingPersistedPayload).FindingID = "" }},
		{"finding enum", FindingPersisted, func(v any) { v.(*FindingPersistedPayload).Severity = "unknown" }},
		{"finding optional bound", FindingPersisted, func(v any) { v.(*FindingPersistedPayload).Path = strings.Repeat("x", maxStringBytes+1) }},
		{"disposition required", DispositionRecorded, func(v any) { v.(*DispositionRecordedPayload).Actor = "" }},
		{"disposition enum", DispositionRecorded, func(v any) { v.(*DispositionRecordedPayload).Actor = "unknown" }},
		{"disposition mapping", DispositionRecorded, func(v any) { v.(*DispositionRecordedPayload).Verdict = "accepted" }},
		{"disposition declined reason", DispositionRecorded, func(v any) { v.(*DispositionRecordedPayload).Verdict = "declined" }},
		{"disposition optional bound", DispositionRecorded, func(v any) { v.(*DispositionRecordedPayload).Reason = strings.Repeat("x", maxStringBytes+1) }},

		// Review completion.
		{"review required", ReviewProcessCompleted, func(v any) { v.(*ReviewProcessCompletedPayload).DemandID = "" }},
		{"review count", ReviewProcessCompleted, func(v any) { v.(*ReviewProcessCompletedPayload).CompletedPasses = 3 }},
		{"review enum", ReviewProcessCompleted, func(v any) { v.(*ReviewProcessCompletedPayload).Result = "unknown" }},
		{"review attempts absent", ReviewProcessCompleted, func(v any) { v.(*ReviewProcessCompletedPayload).PassAttemptIDs = nil }},
		{"review attempt empty", ReviewProcessCompleted, func(v any) { v.(*ReviewProcessCompletedPayload).PassAttemptIDs[0] = "" }},
		{"review verdicts absent", ReviewProcessCompleted, func(v any) { v.(*ReviewProcessCompletedPayload).PassVerdicts = nil }},
		{"review verdict unknown", ReviewProcessCompleted, func(v any) { v.(*ReviewProcessCompletedPayload).PassVerdicts[0] = "unknown" }},
		{"review contexts absent", ReviewProcessCompleted, func(v any) { v.(*ReviewProcessCompletedPayload).TopLevelContextIDs = nil }},
		{"review arrays mismatch", ReviewProcessCompleted, func(v any) { v.(*ReviewProcessCompletedPayload).CompletedPasses = 0 }},

		// Invalidation and activation.
		{"invalidation required", RoutingEvidenceInvalidated, func(v any) { v.(*RoutingEvidenceInvalidatedPayload).AuthorizationRef = "" }},
		{"invalidation disposition", RoutingEvidenceInvalidated, func(v any) { v.(*RoutingEvidenceInvalidatedPayload).Disposition = "unknown" }},
		{"invalidation duplicate set", RoutingEvidenceInvalidated, func(v any) { v.(*RoutingEvidenceInvalidatedPayload).AffectedAttemptIDs = []string{"same", "same"} }},
		{"invalidation empty", RoutingEvidenceInvalidated, func(v any) {
			p := v.(*RoutingEvidenceInvalidatedPayload)
			p.AffectedArtifactRefs = []string{}
			p.AffectedAttemptIDs = []string{}
			p.AffectedEventIDs = []string{}
			p.AffectedProcessIDs = []string{}
		}},
		{"activation required", ActivationCheckpointEvent, func(v any) { v.(*ActivationCheckpointPayload).CoveragePct = "" }},
		{"activation digest", ActivationCheckpointEvent, func(v any) { v.(*ActivationCheckpointPayload).ProofManifestDigest = "bad" }},
		{"activation proofs absent", ActivationCheckpointEvent, func(v any) { v.(*ActivationCheckpointPayload).ProofIDs = nil }},
		{"activation decimal", ActivationCheckpointEvent, func(v any) { v.(*ActivationCheckpointPayload).CoveragePct = "01" }},
		{"activation precision", ActivationCheckpointEvent, func(v any) { v.(*ActivationCheckpointPayload).Precision = "1.0" }},
		{"activation count", ActivationCheckpointEvent, func(v any) { v.(*ActivationCheckpointPayload).Reviews = -1 }},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			payload := validNormalizedPayload(t, tc.kind)
			tc.mutate(payload)
			if err := validatePayload(payload); err == nil {
				t.Fatal("mutated payload was accepted")
			}
		})
	}

	if err := validatePayload(struct{}{}); err == nil {
		t.Fatal("unsupported payload type was accepted")
	}
}

func TestValidateOutcomeSemanticsRejectsEveryInvalidCombination(t *testing.T) {
	usage := Usage{InputTokens: 1}
	tests := []struct {
		name                     string
		outcome, cause, fidelity string
		usage                    Usage
		provenance               string
		turns                    int
	}{
		{"negative usage", "verified", "terminal_verified", "exact", Usage{InputTokens: -1}, "runtime_observed", 1},
		{"excess usage", "verified", "terminal_verified", "exact", Usage{OutputTokens: 1_000_000_001}, "runtime_observed", 1},
		{"negative turns", "verified", "terminal_verified", "exact", usage, "runtime_observed", -1},
		{"unknown provenance", "verified", "terminal_verified", "exact", usage, "unknown", 1},
		{"unobserved usage", "blocked", "crash_reaped", "unavailable", usage, "runtime_unavailable", 1},
		{"not launched fidelity", "failed", "preflight_failed", "exact", Usage{}, "not_launched", 0},
		{"verified mapping", "failed", "terminal_verified", "exact", usage, "runtime_observed", 1},
		{"worker failure mapping", "verified", "worker_failed", "exact", usage, "runtime_observed", 1},
		{"mismatch mapping", "failed", "launch_fidelity_mismatch", "exact", usage, "runtime_observed", 1},
		{"preflight mapping", "failed", "preflight_failed", "unavailable", Usage{}, "runtime_unavailable", 0},
		{"claim mapping", "failed", "claim_lost", "unavailable", Usage{}, "not_launched", 0},
		{"cancel mapping", "failed", "cancelled", "unavailable", Usage{}, "runtime_unavailable", 1},
		{"reaped mapping", "failed", "crash_reaped", "unavailable", Usage{}, "runtime_unavailable", 1},
		{"unknown cause", "blocked", "unknown", "unavailable", Usage{}, "runtime_unavailable", 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateOutcomeSemantics(tc.outcome, tc.cause, tc.fidelity, tc.usage, tc.provenance, tc.turns); err == nil {
				t.Fatal("invalid outcome semantics were accepted")
			}
		})
	}
	if got := mapTaskClass("other"); got != "blocked" {
		t.Fatalf("mapTaskClass(other) = %q", got)
	}
}

func TestValidationHelpersRejectBoundsAndNonCanonicalValues(t *testing.T) {
	huge := strings.Repeat("x", maxStringBytes+1)
	if err := required("field", huge); err == nil {
		t.Fatal("oversized required string was accepted")
	}
	if err := optionalStrings(huge); err == nil {
		t.Fatal("oversized optional string was accepted")
	}
	if err := optionalBand("unknown"); err == nil {
		t.Fatal("unknown optional band was accepted")
	}
	if err := orderedStrings("values", nil); err == nil {
		t.Fatal("nil ordered strings were accepted")
	}
	if err := orderedStrings("values", []string{""}); err == nil {
		t.Fatal("empty ordered string was accepted")
	}
	if got := sortedStrings(nil); got != nil {
		t.Fatalf("sortedStrings(nil) = %#v", got)
	}
	for _, value := range []string{"", "01", ".1", "1.", "1.0", "1.2.3", "-1", "1.a"} {
		if canonicalDecimal(value) {
			t.Errorf("canonicalDecimal(%q) = true", value)
		}
	}
	if err := validateBounds(reflectValueOfStrings(make([]string, maxCollectionElements+1))); err == nil {
		t.Fatal("oversized collection was accepted")
	}
}

func validNormalizedPayload(t *testing.T, kind EventKind) any {
	t.Helper()
	payload, err := normalizedPayload(fixtureEvents(false)[kindIndex(kind)])
	if err != nil {
		t.Fatalf("valid %s fixture: %v", kind, err)
	}
	return payload
}

// Kept behind a tiny helper so the test's table reads in domain values rather
// than reflection syntax.
func reflectValueOfStrings(values []string) reflect.Value { return reflect.ValueOf(values) }
