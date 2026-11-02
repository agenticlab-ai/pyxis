package routingevidence

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestEvidenceAuditClosesRequirementsInvariantsFaultsAndReports(t *testing.T) {
	audit := completeEvidenceAudit()
	wire, err := EncodeEvidenceAudit(audit)
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(wire) || !strings.Contains(string(wire), `"schema":"routing-evidence-audit/v1"`) {
		t.Fatalf("audit wire is not reviewable JSON: %s", wire)
	}

	for i := range audit.Matrix.Cells {
		mutated := audit
		mutated.Matrix.Cells = append([]AcceptanceCell(nil), audit.Matrix.Cells...)
		mutated.Matrix.Cells = append(mutated.Matrix.Cells[:i], mutated.Matrix.Cells[i+1:]...)
		if err := mutated.Validate(); err == nil {
			t.Fatalf("omitted matrix cell %d was accepted", i)
		}
	}
	for i := range audit.Crashes {
		mutated := audit
		mutated.Crashes = append([]CrashCase(nil), audit.Crashes...)
		mutated.Crashes[i].SilentSuccess = true
		if err := mutated.Validate(); err == nil {
			t.Fatalf("silent success at %s was accepted", audit.Crashes[i].FaultPoint)
		}
	}
	for i := range audit.Mutations {
		mutated := audit
		mutated.Mutations = append([]MutationCase(nil), audit.Mutations...)
		mutated.Mutations[i].DesignatedCaseFailed = false
		if err := mutated.Validate(); err == nil {
			t.Fatalf("reverted %s guard survived its designated case", audit.Mutations[i].Guard)
		}
	}
}

func TestEvidenceAuditRejectsFavorableHostedAndProjectionDrift(t *testing.T) {
	audit := completeEvidenceAudit()
	audit.Hosted.Failures[0].OperatorVisible = false
	if err := audit.Validate(); err == nil {
		t.Fatal("operator-invisible hosted outage was accepted")
	}

	audit = completeEvidenceAudit()
	audit.Matrix.Cells[0].EvidenceReference = ""
	if err := audit.Validate(); err == nil {
		t.Fatal("matrix cell without deterministic evidence was accepted")
	}

	audit = completeEvidenceAudit()
	audit.Reports = audit.Reports[:len(audit.Reports)-1]
	if err := audit.Validate(); err == nil {
		t.Fatal("audit without fresh-clone report was accepted")
	}
}

func completeEvidenceAudit() EvidenceAudit {
	coverage := RequiredAcceptanceCoverage()
	dimensions := []CoverageDimension{CoverageRequirement, CoverageScenario, CoverageEventKind, CoverageState, CoverageProducer, CoverageInvariant, CoverageAcceptance}
	cells := make([]AcceptanceCell, 0)
	for _, dimension := range dimensions {
		for _, value := range coverage[dimension] {
			cells = append(cells, AcceptanceCell{Dimension: dimension, Value: value, EvidenceReference: evidenceFor(dimension, value)})
		}
	}
	crashes := make([]CrashCase, 0, len(RequiredFaultPoints()))
	for _, point := range RequiredFaultPoints() {
		crashes = append(crashes, CrashCase{FaultPoint: point, EvidenceReference: faultEvidence(point)})
	}
	guards := []MutationGuard{GuardLaunchProof, GuardOwnerFence, GuardAtomicity, GuardProducerAuthority, GuardProjection, GuardInvalidation, GuardActivation}
	guardEvidence := map[MutationGuard]string{
		GuardLaunchProof:       "go-test:internal/routingevidence/TestAdmissionFailedAndUnverifiableRoutesAuthorizeNothing",
		GuardOwnerFence:        "go-test:internal/routingevidence/TestVerifyOwnershipTransferBindsExactCanonicalAndRuntimeProof",
		GuardAtomicity:         "go-test:internal/routingevidence/TestCompletePassRejectsMissingExtraChangedAndPartialFindingSets",
		GuardProducerAuthority: "go-test:internal/routingevidence/TestProducerAbuseFailsTheSameAtAppendAndFreshRead",
		GuardProjection:        "go-test:internal/routingevidence/TestVerifyProjectionRejectsEveryMismatchAndNonAcceptingFace",
		GuardInvalidation:      "go-test:internal/routingevidence/TestRerunInvalidationExcludesEveryCurrentAcceptanceAndReplacementPasses",
		GuardActivation:        "go-test:internal/routingevidence/TestVerifyActivationRejectsMissingStaleForgedUnmergedMovedAndCrossBoundaryInputs",
	}
	mutations := make([]MutationCase, 0, len(guards))
	for _, guard := range guards {
		mutations = append(mutations, MutationCase{Guard: guard, DesignatedCaseFailed: true, EvidenceReference: guardEvidence[guard]})
	}
	return EvidenceAudit{
		Schema:    EvidenceAuditSchema,
		Matrix:    AcceptanceMatrix{Schema: AcceptanceMatrixSchema, Cells: cells},
		Crashes:   crashes,
		Mutations: mutations,
		Hosted: HostedRefRehearsal{
			Schema: HostedRehearsalSchema, EvidenceReference: "rehearsal:hosted-routing-evidence/v1",
			NonRewritingContinuity: true, FreshReadReconstruction: true, BoundedRetries: true, PayloadExcluded: true,
			Failures: []HostedFailure{{Mode: HostedOutage, Rejected: true, OperatorVisible: true}, {Mode: HostedForce, Rejected: true, OperatorVisible: true}, {Mode: HostedDelete, Rejected: true, OperatorVisible: true}, {Mode: HostedRewrite, Rejected: true, OperatorVisible: true}},
		},
		Reports: []EvidenceReport{
			{Class: ReportConformance, EvidenceReference: "go-test:internal/routingevidence/event-family"},
			{Class: ReportCrash, EvidenceReference: "go-test:internal/routingevidence/boundary-faults"},
			{Class: ReportSecurity, EvidenceReference: "go-test:internal/routingevidence/producer-abuse"},
			{Class: ReportPayload, EvidenceReference: "go-test:internal/routingevidence/payload-audit"},
			{Class: ReportHostedRef, EvidenceReference: "rehearsal:hosted-routing-evidence/v1"},
			{Class: ReportFreshClone, EvidenceReference: "go-test:internal/routingevidence/gitstore/fresh-clone-route-review"},
		},
	}
}

func evidenceFor(dimension CoverageDimension, value string) string {
	switch dimension {
	case CoverageEventKind:
		return "golden:internal/routingevidence/testdata/event_family_golden.json#" + value
	case CoverageProducer:
		return "go-test:internal/routingevidence/TestProducerPolicySignsAndVerifiesEveryEventKind#" + value
	case CoverageState:
		return "go-test:internal/routingevidence/TestReducerRecognizesEveryDeclaredReachableState#" + value
	case CoverageInvariant:
		return invariantEvidence(value)
	case CoverageAcceptance:
		return acceptanceEvidence(value)
	case CoverageScenario:
		return scenarioEvidence(value)
	default:
		return "plan:planner-driven-model-routing/003/T12#" + value
	}
}

func scenarioEvidence(value string) string {
	refs := map[string]string{
		"exact":          "TestAdmissionImplementationExactProofGatesClaim",
		"promoted":       "TestEventFamilyGoldenAndIndependentProducerConformance/routing_resolved",
		"unsupported":    "TestAdmissionFailedAndUnverifiableRoutesAuthorizeNothing",
		"implementation": "TestCompleteAttemptTerminalCauseAndMappingMatrix",
		"review":         "gitstore/TestAcceptanceFreshCloneReconstructsCompleteRouteAndReview",
		"recovery":       "TestAttemptReaperRoutesEveryRuntimeObservationWithoutLaunching",
		"invalidation":   "TestRerunInvalidationExcludesEveryCurrentAcceptanceAndReplacementPasses",
		"activation":     "TestVerifyActivationBindsSignedManifestExactMainlineAndCurrentEvidence",
	}
	return "go-test:internal/routingevidence/" + refs[value]
}

func faultEvidence(point FaultPoint) string {
	switch point {
	case FaultBeforeAppend, FaultAfterAppend, FaultBeforeCAS, FaultAfterCAS:
		return "go-test:internal/gitrefstore/TestApplyConcurrentNonConflictingWritersReplayFromWinningHead#" + string(point)
	case FaultBeforeExactRead, FaultAfterExactRead, FaultBeforeClaim, FaultAfterClaim:
		return "go-test:internal/routingevidence/TestAdmissionImplementationExactProofGatesClaim#" + string(point)
	case FaultBeforeSpawn, FaultAfterSpawn:
		return "go-test:internal/routingevidence/TestFencedAndReaperOutcomeAuthorities#" + string(point)
	case FaultBeforeComposite, FaultAfterComposite:
		return "go-test:internal/gitrefstore/TestUnpushedMultiPathCommitIsNotReachable#" + string(point)
	case FaultBeforeTransfer, FaultAfterTransfer:
		return "go-test:internal/routingevidence/TestAttemptReaperRoutesEveryRuntimeObservationWithoutLaunching#" + string(point)
	case FaultBeforeProjection, FaultAfterProjection:
		return "go-test:internal/routingevidence/TestVerifyProjectionRejectsEveryMismatchAndNonAcceptingFace#" + string(point)
	case FaultBeforeInvalidation, FaultAfterInvalidation:
		return "go-test:internal/routingevidence/TestInvalidationServiceRepeatAndConflictsFailClosed#" + string(point)
	default:
		return "go-test:internal/routingevidence/TestCheckpointIdentityReplayAndChangeSemantics#" + string(point)
	}
}

func invariantEvidence(value string) string {
	refs := map[string]string{
		"DINV-1": "TestEventFamilyGoldenAndIndependentProducerConformance", "DINV-2": "TestCanonicalSurfacesHaveNoForbiddenPayloadSlots",
		"DINV-3": "TestReducerRejectsSameIdentityWithDifferentCanonicalBytes", "DINV-4": "TestReducerRejectsEveryBrokenJoinClass",
		"DINV-5": "TestAdmissionImplementationExactProofGatesClaim", "DINV-6": "TestAdmissionFailedAndUnverifiableRoutesAuthorizeNothing",
		"DINV-7": "TestCompleteAttemptTerminalCauseAndMappingMatrix", "DINV-8": "TestAttemptReaperRoutesEveryRuntimeObservationWithoutLaunching",
		"DINV-9": "TestVerifyProjectionRejectsEveryMismatchAndNonAcceptingFace", "DINV-10": "TestProducerAbuseFailsTheSameAtAppendAndFreshRead",
		"DINV-11": "TestRetryHasNoAmbientTimeInput", "DINV-12": "TestReducerRecognizesEveryDeclaredReachableState",
		"DINV-13": "TestCompletePassRejectsMissingExtraChangedAndPartialFindingSets", "DINV-14": "TestVerifyOwnershipTransferBindsExactCanonicalAndRuntimeProof",
		"DINV-15": "TestAdmissionTraceRejectsHiddenNetworkOperation",
	}
	return "go-test:internal/routingevidence/" + refs[value]
}

func acceptanceEvidence(value string) string {
	refs := map[string]string{
		"A-1": "TestEventFamilyGoldenAndIndependentProducerConformance", "A-2": "TestAdapterConcurrentWritersConvergeAndConflictPreservesWinner",
		"A-3": "TestAdmissionTraceRejectsHiddenNetworkOperation", "A-4": "TestCompleteAttemptTerminalCauseAndMappingMatrix",
		"A-5": "TestFencedAndReaperOutcomeAuthorities", "A-6": "TestEveryMediumProjectionRoundTripsOneCanonicalIdentitySet",
		"A-7": "TestRerunInvalidationExcludesEveryCurrentAcceptanceAndReplacementPasses", "A-8": "TestCanonicalSurfacesHaveNoForbiddenPayloadSlots",
		"A-9": "TestProtectedRemoteDeniesForceDeleteAndSecondInitialization",
	}
	return "go-test:internal/routingevidence/" + refs[value]
}
