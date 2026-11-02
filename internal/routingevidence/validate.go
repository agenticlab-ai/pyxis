package routingevidence

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"reflect"
	"slices"
	"sort"
	"strings"
)

func selectedPayload(e Event) (any, error) {
	members := []any{
		e.Payload.ExecutionDemandFrozen,
		e.Payload.RoutingResolved, e.Payload.RoutingResolutionFailed,
		e.Payload.RoutedAttemptCompleted, e.Payload.RoutedTaskOutcome,
		e.Payload.AttemptOwnerTransferred, e.Payload.AssurancePassCompleted,
		e.Payload.FindingPersisted, e.Payload.DispositionRecorded,
		e.Payload.ReviewProcessCompleted, e.Payload.RoutingEvidenceInvalidated,
		e.Payload.ActivationCheckpoint,
	}
	count := 0
	for _, member := range members {
		if !nilPayload(member) {
			count++
		}
	}
	if count != 1 {
		return nil, fmt.Errorf("payload: event requires exactly one union member, got %d", count)
	}
	var selected any
	switch e.Kind {
	case ExecutionDemandFrozen:
		selected = e.Payload.ExecutionDemandFrozen

	case RoutingResolved:
		selected = e.Payload.RoutingResolved
	case RoutingResolutionFailed:
		selected = e.Payload.RoutingResolutionFailed
	case RoutedAttemptCompleted:
		selected = e.Payload.RoutedAttemptCompleted
	case RoutedTaskOutcome:
		selected = e.Payload.RoutedTaskOutcome
	case AttemptOwnerTransferred:
		selected = e.Payload.AttemptOwnerTransferred
	case AssurancePassCompleted:
		selected = e.Payload.AssurancePassCompleted
	case FindingPersisted:
		selected = e.Payload.FindingPersisted
	case DispositionRecorded:
		selected = e.Payload.DispositionRecorded
	case ReviewProcessCompleted:
		selected = e.Payload.ReviewProcessCompleted
	case RoutingEvidenceInvalidated:
		selected = e.Payload.RoutingEvidenceInvalidated
	case ActivationCheckpointEvent:
		selected = e.Payload.ActivationCheckpoint
	default:
		return nil, fmt.Errorf("event_kind: unknown kind %q", e.Kind)
	}
	if nilPayload(selected) {
		return nil, fmt.Errorf("payload: %s kind does not select its populated union member", e.Kind)
	}
	return selected, nil
}

func nilPayload(v any) bool {
	switch x := v.(type) {
	case *ExecutionDemandFrozenPayload:
		return x == nil

	case *RoutingResolvedPayload:
		return x == nil
	case *RoutingResolutionFailedPayload:
		return x == nil
	case *RoutedAttemptCompletedPayload:
		return x == nil
	case *RoutedTaskOutcomePayload:
		return x == nil
	case *AttemptOwnerTransferredPayload:
		return x == nil
	case *AssurancePassCompletedPayload:
		return x == nil
	case *FindingPersistedPayload:
		return x == nil
	case *DispositionRecordedPayload:
		return x == nil
	case *ReviewProcessCompletedPayload:
		return x == nil
	case *RoutingEvidenceInvalidatedPayload:
		return x == nil
	case *ActivationCheckpointPayload:
		return x == nil
	default:
		return true
	}
}

func normalizedPayload(e Event) (any, error) {
	payload, err := selectedPayload(e)
	if err != nil {
		return nil, err
	}
	switch p := payload.(type) {
	case *ExecutionDemandFrozenPayload:
		out := *p
		out.AssessmentEvidenceRefs = sortedStrings(p.AssessmentEvidenceRefs)
		payload = &out
	case *RoutingResolvedPayload:
		out := *p
		out.PromotedComponents = sortedStrings(p.PromotedComponents)
		payload = &out
	case *RoutingResolutionFailedPayload:
		out := *p
		out.UnmetComponents = sortedStrings(p.UnmetComponents)
		payload = &out
	case *AssurancePassCompletedPayload:
		out := *p
		out.FindingRecords = append([]NormalizedFinding(nil), p.FindingRecords...)
		sort.Slice(out.FindingRecords, func(i, j int) bool {
			ib, _ := canonicalJSON(out.FindingRecords[i])
			jb, _ := canonicalJSON(out.FindingRecords[j])
			return bytes.Compare(ib, jb) < 0
		})
		payload = &out
	case *RoutingEvidenceInvalidatedPayload:
		out := *p
		out.AffectedArtifactRefs = sortedStrings(p.AffectedArtifactRefs)
		out.AffectedAttemptIDs = sortedStrings(p.AffectedAttemptIDs)
		out.AffectedEventIDs = sortedStrings(p.AffectedEventIDs)
		out.AffectedProcessIDs = sortedStrings(p.AffectedProcessIDs)
		want, err := invalidationID(out)
		if err != nil {
			return nil, err
		}
		if out.InvalidationID != "" && out.InvalidationID != want {
			return nil, fmt.Errorf("invalidation_id: does not match its canonical preimage")
		}
		out.InvalidationID = want
		payload = &out
	case *ActivationCheckpointPayload:
		out := *p
		out.ProofIDs = sortedStrings(p.ProofIDs)
		payload = &out
	}
	if err := validatePayload(payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func validateEventBase(e Event, producer, proof bool) error {
	if err := validateBounds(reflect.ValueOf(e)); err != nil {
		return err
	}
	if e.Schema != Schema {
		return fmt.Errorf("schema: unsupported value %q", e.Schema)
	}
	if e.Version != EventVersion {
		return fmt.Errorf("event_version: unsupported value %q", e.Version)
	}
	if err := required("repository", e.Repository); err != nil {
		return err
	}
	if err := required("evidence_ref", e.EvidenceRef); err != nil {
		return err
	}
	if producer {
		if err := required("producer.class", e.Producer.Class); err != nil {
			return err
		}
		if err := required("producer.generation", e.Producer.Generation); err != nil {
			return err
		}
	}
	if proof {
		if e.ProducerProof.Schema != ProducerProofSchema || e.ProducerProof.Algorithm != "ed25519" {
			return fmt.Errorf("producer_proof: unsupported schema or algorithm")
		}
		if err := required("producer_proof.key_id", e.ProducerProof.KeyID); err != nil {
			return err
		}
		if err := required("producer_proof.signature", e.ProducerProof.Signature); err != nil {
			return err
		}
	}
	return nil
}

func validatePayload(payload any) error {
	if err := validateBounds(reflect.ValueOf(payload)); err != nil {
		return err
	}
	switch p := payload.(type) {
	case *ExecutionDemandFrozenPayload:
		if err := requiredMany(p.DemandID, p.TaskID, p.PlanVersion, p.PolicyVersion); err != nil {
			return err
		}
		if err := validateAssessment(p.Assessment); err != nil {
			return err
		}
		if err := stringSet("assessment_evidence_refs", p.AssessmentEvidenceRefs, true); err != nil {
			return err
		}
		return validateProfile(p.Profile)

	case *RoutingResolvedPayload:
		if err := requiredMany(p.AttemptID, p.DemandID, p.Harness.String(), p.NativeProfileID, p.PolicyVersion, string(p.Role), p.TaskID); err != nil {
			return err
		}
		if !oneOf(p.Role, "implementation", "review") || !oneOf(p.Resolution, "exact", "promoted") {
			return fmt.Errorf("routing_resolved: unknown role or resolution")
		}
		if p.OwnerGeneration != 1 {
			return fmt.Errorf("owner_generation: initial resolution requires generation 1")
		}
		if err := optionalBand(p.ProposalBand); err != nil {
			return err
		}
		if err := stringSet("promoted_components", p.PromotedComponents, false); err != nil {
			return err
		}
		if p.Resolution == "exact" && len(p.PromotedComponents) != 0 {
			return fmt.Errorf("promoted_components: exact resolution cannot promote")
		}
		return validateProfile(p.RequestedProfile)
	case *RoutingResolutionFailedPayload:
		if err := requiredMany(p.AttemptID, p.DemandID, p.Harness.String(), p.PolicyVersion, string(p.Role), p.TaskID); err != nil {
			return err
		}
		if !oneOf(p.Role, "implementation", "review") {
			return fmt.Errorf("role: unknown value %q", p.Role)
		}
		if !oneOf(p.Reason, "policy_invalid", "unsupported", "integrity_error", "legacy_incompatible", "duplicate_admission") {
			return fmt.Errorf("reason: unknown value %q", p.Reason)
		}
		if err := optionalBand(p.ProposalBand); err != nil {
			return err
		}
		if err := stringSet("unmet_components", p.UnmetComponents, false); err != nil {
			return err
		}
		return validateProfile(p.RequestedProfile)
	case *RoutedAttemptCompletedPayload:
		if err := requiredMany(p.AttemptID, p.Cause, p.DemandID, p.EvidenceRef, p.LaunchCapabilityRef, p.LaunchFidelity, p.TaskID, p.UsageProvenance); err != nil {
			return err
		}
		if !oneOf(p.Outcome, "verified", "failed", "blocked") {
			return fmt.Errorf("outcome: unknown value %q", p.Outcome)
		}
		if !oneOf(p.LaunchFidelity, "exact", "mismatch", "unavailable") {
			return fmt.Errorf("launch_fidelity: unknown value %q", p.LaunchFidelity)
		}
		if p.OwnerGeneration < 1 {
			return fmt.Errorf("owner_generation: must be positive")
		}
		return validateOutcomeSemantics(p.Outcome, p.Cause, p.LaunchFidelity, p.Usage, p.UsageProvenance, 0)
	case *RoutedTaskOutcomePayload:
		if err := requiredMany(p.AttemptID, p.Cause, p.DemandID, p.EvidenceRef, p.LaunchCapabilityRef, p.LaunchFidelity, p.TaskID, p.UsageProvenance); err != nil {
			return err
		}
		if !oneOf(p.Class, "succeeded", "failed", "blocked") || !oneOf(p.Verification, "verified", "failed", "not_reached") {
			return fmt.Errorf("routed_task_outcome: unknown class or verification")
		}
		if (p.Class == "succeeded" && p.Verification != "verified") || (p.Class == "failed" && p.Verification != "failed") || (p.Class == "blocked" && p.Verification != "not_reached") {
			return fmt.Errorf("routed_task_outcome: invalid class/verification mapping")
		}
		if p.OwnerGeneration < 1 {
			return fmt.Errorf("owner_generation: must be positive")
		}
		outcome := mapTaskClass(p.Class)
		return validateOutcomeSemantics(outcome, p.Cause, p.LaunchFidelity, p.Usage, p.UsageProvenance, p.Turns)
	case *AttemptOwnerTransferredPayload:
		if err := requiredMany(p.AttemptID, p.RuntimeFenceNonce, p.RuntimeFenceProofRef); err != nil {
			return err
		}
		if p.PriorOwnerGeneration < 1 || p.NewOwnerGeneration != p.PriorOwnerGeneration+1 {
			return fmt.Errorf("new_owner_generation: must be exactly one greater than prior")
		}
		if !oneOf(p.Observation, "not_started", "terminal", "absent_proven") {
			return fmt.Errorf("observation: transfer cannot follow %q", p.Observation)
		}
		return nil
	case *AssurancePassCompletedPayload:
		if err := requiredMany(p.AttemptID, p.DemandID, p.PassID, p.ProposalID, p.ReviewProcessID, p.ReviewedRevision, p.RoutedOutcomeRef, p.RoutingResolutionRef, p.TaskID, p.TopLevelContextID, p.Verdict); err != nil {
			return err
		}
		if !oneOf(p.EffectiveDepth, "standard", "deep") || !oneOf(p.Verdict, "approve", "approve-with-advisories", "request-changes", "escalate") {
			return fmt.Errorf("assurance_pass_completed: unknown depth or verdict")
		}
		if p.PassIndex < 1 || p.ProcessGeneration < 1 || p.RequiredPasses < 1 || p.PassIndex > p.RequiredPasses || p.InternalRounds < 1 || p.Candidates < 0 || p.Killed < 0 || p.Tokens < 0 || p.WallMillis < 0 {
			return fmt.Errorf("assurance_pass_completed: invalid count")
		}
		if p.FindingRecords == nil || len(p.FindingRecords) > maxCollectionElements {
			return fmt.Errorf("findings: absent or too large")
		}
		if p.Findings != len(p.FindingRecords) {
			return fmt.Errorf("findings: count does not match finding_records")
		}
		seen := map[string]bool{}
		for _, finding := range p.FindingRecords {
			if err := validateFinding(finding); err != nil {
				return err
			}
			if seen[finding.FindingID] {
				return fmt.Errorf("findings: duplicate %q", finding.FindingID)
			}
			seen[finding.FindingID] = true
		}
		return validateProvenance(p.Provenance)
	case *FindingPersistedPayload:
		if err := requiredMany(p.PassID, p.ProposalID, p.ReviewProcessID, p.ReviewedRevision); err != nil {
			return err
		}
		return validateFinding(p.NormalizedFinding)
	case *DispositionRecordedPayload:
		if err := requiredMany(p.Actor, p.ActorAttribution, p.AttributionVerification, p.FindingID, p.NativeAuthorIdentityProof, p.NativeMediumActRef, p.Verdict); err != nil {
			return err
		}
		if !oneOf(p.Actor, "worker", "operator") || !oneOf(p.AttributionVerification, "verified", "failed") || !oneOf(p.Verdict, "applied", "declined", "accepted", "rejected") {
			return fmt.Errorf("disposition_recorded: unknown enum")
		}
		if (p.Actor == "worker" && !oneOf(p.Verdict, "applied", "declined")) || (p.Actor == "operator" && !oneOf(p.Verdict, "accepted", "rejected")) {
			return fmt.Errorf("disposition_recorded: actor/verdict mapping is invalid")
		}
		if p.Verdict == "declined" && p.Reason == "" {
			return fmt.Errorf("reason: declined disposition requires a reason")
		}
		return optionalStrings(p.Reason, p.SourceOwnedAt)
	case *ReviewProcessCompletedPayload:
		if err := requiredMany(p.AggregateVerdict, p.DemandID, p.EffectiveDepth, p.EvidenceRef, p.PlannedAssurance, p.ProposalBand, p.ProposalID, p.Result, p.ReviewProcessID, p.ReviewedRevision, p.TaskID); err != nil {
			return err
		}
		if p.ProcessGeneration < 1 || p.RequiredPasses < 1 || p.CompletedPasses < 0 || p.CompletedPasses > p.RequiredPasses || p.AggregateWallMillis < 0 {
			return fmt.Errorf("review_process_completed: invalid count")
		}
		if !oneOf(p.PlannedAssurance, "ordinary", "elevated", "critical") || !oneOf(p.EffectiveDepth, "standard", "deep") || !oneOf(p.ProposalBand, "skip", "standard", "deep") || !oneOf(p.Result, "satisfied", "floor_violation", "conflict_nonaccepting") || !reviewVerdict(p.AggregateVerdict) {
			return fmt.Errorf("review_process_completed: unknown enum")
		}
		if err := orderedStrings("pass_attempt_ids", p.PassAttemptIDs); err != nil {
			return err
		}
		if err := orderedStrings("pass_verdicts", p.PassVerdicts); err != nil {
			return err
		}
		for _, verdict := range p.PassVerdicts {
			if !reviewVerdict(verdict) {
				return fmt.Errorf("pass_verdicts: unknown verdict %q", verdict)
			}
		}
		if err := orderedStrings("top_level_context_ids", p.TopLevelContextIDs); err != nil {
			return err
		}
		if len(p.PassAttemptIDs) != len(p.PassVerdicts) || len(p.PassAttemptIDs) != len(p.TopLevelContextIDs) || len(p.PassAttemptIDs) != p.CompletedPasses {
			return fmt.Errorf("review_process_completed: pass arrays must match completed_passes")
		}
		return nil
	case *RoutingEvidenceInvalidatedPayload:
		if err := requiredMany(p.AuthorizationRef, p.Disposition, p.InvalidationID, p.NativeProfileID, p.PolicyVersion); err != nil {
			return err
		}
		if !oneOf(p.Disposition, "rerun_required", "operator_remediation") {
			return fmt.Errorf("disposition: unknown value %q", p.Disposition)
		}
		for name, values := range map[string][]string{"artifact_refs": p.AffectedArtifactRefs, "attempt_ids": p.AffectedAttemptIDs, "event_ids": p.AffectedEventIDs, "review_process_ids": p.AffectedProcessIDs} {
			if err := stringSet(name, values, false); err != nil {
				return err
			}
		}
		if len(p.AffectedArtifactRefs)+len(p.AffectedAttemptIDs)+len(p.AffectedEventIDs)+len(p.AffectedProcessIDs) == 0 {
			return fmt.Errorf("invalidation: at least one affected identity is required")
		}
		return nil
	case *ActivationCheckpointPayload:
		if err := requiredMany(p.ActivationPolicyVersion, p.CoveragePct, p.DwellP50, p.EvidenceHead, p.ExactMainlineObject, p.MainlineRef, p.NitP50, p.OperatorAuthorizationID, p.ProofManifestDigest); err != nil {
			return err
		}
		if _, err := decodeHexExact(p.ProofManifestDigest, sha256.Size); err != nil {
			return fmt.Errorf("proof_manifest_digest: %w", err)
		}
		if err := stringSet("proof_ids", p.ProofIDs, true); err != nil {
			return err
		}
		for name, value := range map[string]string{"coverage_pct": p.CoveragePct, "dwell_p50": p.DwellP50, "nit_p50": p.NitP50} {
			if !canonicalDecimal(value) {
				return fmt.Errorf("%s: non-canonical decimal", name)
			}
		}
		if p.Precision != "" && !canonicalDecimal(p.Precision) {
			return fmt.Errorf("precision: non-canonical decimal")
		}
		if p.AvailabilityDefects < 0 || p.BlockingDisposed < 0 || p.MatrixCellsCovered < 0 || p.MatrixCellsRequired < 0 || p.ProcessViolations < 0 || p.Reviews < 0 {
			return fmt.Errorf("activation_checkpoint: negative count")
		}
		return nil
	default:
		return fmt.Errorf("payload: unsupported member %T", payload)
	}
}

func reviewVerdict(value string) bool {
	return oneOf(value, "approve", "approve-with-advisories", "request-changes", "escalate")
}

func validateAssessment(a Assessment) error {
	if !oneOf(a.Reasoning, "routine", "complex", "difficult") || !oneOf(a.Coupling, "contained", "composed", "cross_boundary") || !oneOf(a.Novelty, "known", "adapt", "first_of_kind") || !oneOf(a.Verification, "local", "composed", "cross_boundary") || !oneOf(a.ImpactScope, "contained", "multi_surface", "system_wide") || !oneOf(a.ArchitecturalRole, "feature_local", "shared_substrate", "system_core") {
		return fmt.Errorf("assessment: unknown or missing axis")
	}
	return nil
}

func validateProfile(p Profile) error {
	if !oneOf(p.Capability, "economy", "standard", "strong", "frontier") || !oneOf(p.Deliberation, "light", "standard", "deep", "maximum") || !oneOf(p.Context, "standard", "large", "extended") || !oneOf(p.Assurance, "ordinary", "elevated", "critical") {
		return fmt.Errorf("profile: unknown or missing component")
	}
	return nil
}

func validateUsage(u Usage) error {
	if u.CacheCreationTokens < 0 || u.CacheReadTokens < 0 || u.InputTokens < 0 || u.OutputTokens < 0 {
		return fmt.Errorf("usage: counters must be non-negative")
	}
	const maxUsageCounter = 1_000_000_000
	if u.CacheCreationTokens > maxUsageCounter || u.CacheReadTokens > maxUsageCounter || u.InputTokens > maxUsageCounter || u.OutputTokens > maxUsageCounter {
		return fmt.Errorf("usage: counter exceeds %d", maxUsageCounter)
	}
	return nil
}

func validateOutcomeSemantics(outcome, cause, fidelity string, usage Usage, provenance string, turns int) error {
	if err := validateUsage(usage); err != nil {
		return err
	}
	if turns < 0 || turns > 1_000_000 {
		return fmt.Errorf("turns: must be between 0 and 1000000")
	}
	if !oneOf(provenance, "runtime_observed", "runtime_unavailable", "not_launched") {
		return fmt.Errorf("usage_provenance: unknown value %q", provenance)
	}
	if provenance != "runtime_observed" && usage != (Usage{}) {
		return fmt.Errorf("usage: unavailable or unlaunched usage must contain zero counters")
	}
	if provenance == "not_launched" && (fidelity != "unavailable" || turns != 0) {
		return fmt.Errorf("usage_provenance: not_launched requires unavailable fidelity and zero turns")
	}
	switch cause {
	case "terminal_verified":
		if outcome != "verified" || fidelity != "exact" || provenance != "runtime_observed" {
			return fmt.Errorf("terminal cause: verified requires verified/exact/runtime_observed")
		}
	case "worker_failed":
		if outcome != "failed" || fidelity != "exact" || provenance != "runtime_observed" {
			return fmt.Errorf("terminal cause: worker_failed requires failed/exact/runtime_observed")
		}
	case "launch_fidelity_mismatch":
		if outcome != "failed" || fidelity != "mismatch" || provenance != "runtime_observed" {
			return fmt.Errorf("terminal cause: launch_fidelity_mismatch requires failed/mismatch/runtime_observed")
		}
	case "preflight_failed":
		if outcome != "failed" || fidelity != "unavailable" || provenance != "not_launched" {
			return fmt.Errorf("terminal cause: preflight_failed requires failed/unavailable/not_launched")
		}
	case "claim_lost":
		if outcome != "blocked" || fidelity != "unavailable" || provenance != "not_launched" {
			return fmt.Errorf("terminal cause: claim_lost requires blocked/unavailable/not_launched")
		}
	case "cancelled":
		if outcome != "blocked" || fidelity == "mismatch" {
			return fmt.Errorf("terminal cause: cancelled requires blocked and non-mismatch fidelity")
		}
	case "crash_reaped":
		if outcome != "blocked" || provenance != "runtime_unavailable" || fidelity == "mismatch" {
			return fmt.Errorf("terminal cause: crash_reaped requires blocked/runtime_unavailable and non-mismatch fidelity")
		}
	default:
		return fmt.Errorf("terminal cause: unknown value %q", cause)
	}
	return nil
}

func mapTaskClass(class string) string {
	switch class {
	case "succeeded":
		return "verified"
	case "failed":
		return "failed"
	default:
		return "blocked"
	}
}

func validateProvenance(p Provenance) error {
	return requiredMany(p.CalibrationVersion, p.Model, p.PromptsVersion)
}

func validateFinding(f NormalizedFinding) error {
	if err := requiredMany(f.Acceptance, f.CalibrationRow, f.FindingID, f.Lens, f.Message, f.RuleID, f.Severity); err != nil {
		return err
	}
	if f.Line < 0 || !oneOf(f.Lens, "spec-fidelity", "design-conformance", "security-abuse", "correctness-critic") || !oneOf(f.Severity, "blocking", "advisory") {
		return fmt.Errorf("finding: invalid line, lens, or severity")
	}
	return optionalStrings(f.LogRef, f.Path)
}

func requiredMany(values ...string) error {
	for _, value := range values {
		if err := required("field", value); err != nil {
			return err
		}
	}
	return nil
}
func required(name, value string) error {
	if value == "" {
		return fmt.Errorf("%s: required", name)
	}
	if len(value) > maxStringBytes {
		return fmt.Errorf("%s: exceeds %d bytes", name, maxStringBytes)
	}
	return nil
}
func optionalStrings(values ...string) error {
	for _, value := range values {
		if len(value) > maxStringBytes {
			return fmt.Errorf("string: exceeds %d bytes", maxStringBytes)
		}
	}
	return nil
}
func oneOf(value string, values ...string) bool {
	return slices.Contains(values, value)
}
func optionalBand(value string) error {
	if value != "" && !oneOf(value, "skip", "standard", "deep") {
		return fmt.Errorf("proposal_band: unknown value %q", value)
	}
	return nil
}

func orderedStrings(name string, values []string) error {
	if values == nil || len(values) > maxCollectionElements {
		return fmt.Errorf("%s: absent or too large", name)
	}
	for _, value := range values {
		if err := required(name, value); err != nil {
			return err
		}
	}
	return nil
}

func stringSet(name string, values []string, nonempty bool) error {
	if err := orderedStrings(name, values); err != nil {
		return err
	}
	if nonempty && len(values) == 0 {
		return fmt.Errorf("%s: must not be empty", name)
	}
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		if seen[value] {
			return fmt.Errorf("%s: duplicate %q", name, value)
		}
		seen[value] = true
	}
	return nil
}

func sortedStrings(values []string) []string {
	if values == nil {
		return nil
	}
	// Preserve a declared empty set as [] rather than collapsing it to null;
	// null is outside the canonical grammar while an empty set is admitted by
	// several closed payloads.
	out := make([]string, len(values))
	copy(out, values)
	sort.Strings(out)
	return out
}

func canonicalDecimal(value string) bool {
	if value == "0" {
		return true
	}
	parts := strings.Split(value, ".")
	if len(parts) > 2 || parts[0] == "" || (len(parts[0]) > 1 && parts[0][0] == '0') {
		return false
	}
	for _, r := range parts[0] {
		if r < '0' || r > '9' {
			return false
		}
	}
	if len(parts) == 2 {
		if parts[1] == "" || parts[1][len(parts[1])-1] == '0' {
			return false
		}
		for _, r := range parts[1] {
			if r < '0' || r > '9' {
				return false
			}
		}
	}
	return true
}

type invalidationPreimage struct {
	ArtifactRefs     []string `json:"artifact_refs"`
	AttemptIDs       []string `json:"attempt_ids"`
	AuthorizationRef string   `json:"authorization_ref"`
	Disposition      string   `json:"disposition"`
	Domain           string   `json:"domain"`
	EventIDs         []string `json:"event_ids"`
	NativeProfileID  string   `json:"native_profile_id"`
	PolicyVersion    string   `json:"policy_version"`
	ReviewProcessIDs []string `json:"review_process_ids"`
}

func invalidationID(p RoutingEvidenceInvalidatedPayload) (string, error) {
	return canonicalDigest(invalidationPreimage{ArtifactRefs: p.AffectedArtifactRefs, AttemptIDs: p.AffectedAttemptIDs, AuthorizationRef: p.AuthorizationRef, Disposition: p.Disposition, Domain: InvalidationIDDomain, EventIDs: p.AffectedEventIDs, NativeProfileID: p.NativeProfileID, PolicyVersion: p.PolicyVersion, ReviewProcessIDs: p.AffectedProcessIDs})
}

func canonicalDigest(v any) (string, error) {
	b, err := canonicalJSON(v)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func validateBounds(v reflect.Value) error {
	if !v.IsValid() {
		return nil
	}
	if v.Kind() == reflect.Interface || v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return nil
		}
		return validateBounds(v.Elem())
	}
	switch v.Kind() {
	case reflect.String:
		if len(v.String()) > maxStringBytes {
			return fmt.Errorf("payload: string exceeds %d bytes", maxStringBytes)
		}
	case reflect.Slice, reflect.Array:
		if v.Len() > maxCollectionElements {
			return fmt.Errorf("payload: collection exceeds %d elements", maxCollectionElements)
		}
		for i := 0; i < v.Len(); i++ {
			if err := validateBounds(v.Index(i)); err != nil {
				return err
			}
		}
	case reflect.Struct:
		for _, field := range v.Fields() {
			if err := validateBounds(field); err != nil {
				return err
			}
		}
	}
	return nil
}
