// Package routingevidence defines the closed routing-evidence/v1 event family.
// It is deliberately a leaf package: producers supply already-decided bounded
// facts, while this package owns only their schema, canonical bytes, and
// semantic identities.
package routingevidence

import (
	"encoding/json"
	"fmt"
	"github.com/agenticlab-ai/pyxis/internal/entity"
)

const (
	Schema               = "routing-evidence/v1"
	CanonicalDomain      = "routing-evidence-canonical/v1"
	EventIDDomain        = "routing-evidence-event-id/v1"
	InvalidationIDDomain = "routing-evidence-invalidation-id/v1"
	ProofManifestDomain  = "routing-evidence-proof-manifest/v1"
	// v2 is the all-path implementation+review activation contract carried by
	// implement-now-authenticated-review-obligations §3.16. v1 proved only the
	// routing-evidence substrate and must never authorize production cutover.
	ActivationManifestSchema = "routing-activation-proof/v2"
	MainlineProofSchema      = "routing-mainline-proof/v1"
	ProducerProofSchema      = "routing-evidence-producer-proof/v1"
	EventVersion             = "1"
	maxStringBytes           = 8192
	maxCollectionElements    = 256
)

type EventID string
type EventKind string

const (
	ExecutionDemandFrozen EventKind = "execution_demand_frozen"

	RoutingResolved            EventKind = "routing_resolved"
	RoutingResolutionFailed    EventKind = "routing_resolution_failed"
	RoutedAttemptCompleted     EventKind = "routed_attempt_completed"
	RoutedTaskOutcome          EventKind = "routed_task_outcome"
	AttemptOwnerTransferred    EventKind = "attempt_owner_transferred"
	AssurancePassCompleted     EventKind = "assurance_pass_completed"
	FindingPersisted           EventKind = "finding_persisted"
	DispositionRecorded        EventKind = "disposition_recorded"
	ReviewProcessCompleted     EventKind = "review_process_completed"
	RoutingEvidenceInvalidated EventKind = "routing_evidence_invalidated"
	ActivationCheckpointEvent  EventKind = "activation_checkpoint"
)

// Producer is content-bearing attribution. Authorization of the class and
// generation belongs to the producer-policy layer, not this codec.
type Producer struct {
	Class      string `json:"class"`
	Generation string `json:"generation"`
}

// ProducerProof is the closed proof wire shape. This task preserves and
// excludes it correctly; cryptographic authorization is implemented by T2.
type ProducerProof struct {
	Algorithm string `json:"algorithm"`
	KeyID     string `json:"key_id"`
	Schema    string `json:"schema"`
	Signature string `json:"signature"`
}

type Assessment struct {
	ArchitecturalRole string `json:"architectural_role"`
	Coupling          string `json:"coupling"`
	ImpactScope       string `json:"impact_scope"`
	Novelty           string `json:"novelty"`
	Reasoning         string `json:"reasoning"`
	Verification      string `json:"verification"`
}

type Profile struct {
	Assurance    string `json:"assurance"`
	Capability   string `json:"capability"`
	Context      string `json:"context"`
	Deliberation string `json:"deliberation"`
}

// Usage contains summable, provider-neutral counters only. A producer that
// cannot observe a counter records zero; prompts and transcripts have no slot.
type Usage struct {
	CacheCreationTokens int `json:"cache_creation_tokens"`
	CacheReadTokens     int `json:"cache_read_tokens"`
	InputTokens         int `json:"input_tokens"`
	OutputTokens        int `json:"output_tokens"`
}

type Provenance struct {
	CalibrationVersion string `json:"calibration_version"`
	Model              string `json:"model"`
	PromptsVersion     string `json:"prompts_version"`
}

type NormalizedFinding struct {
	Acceptance     string `json:"acceptance"`
	CalibrationRow string `json:"cal_row"`
	FindingID      string `json:"finding_id"`
	Lens           string `json:"lens"`
	Line           int    `json:"line,omitempty"`
	LogRef         string `json:"log_ref,omitempty"`
	Message        string `json:"message"`
	Path           string `json:"path,omitempty"`
	RuleID         string `json:"rule_id"`
	Severity       string `json:"severity"`
}

type ExecutionDemandFrozenPayload struct {
	Assessment             Assessment `json:"assessment"`
	AssessmentEvidenceRefs []string   `json:"assessment_evidence_refs"`
	AssuranceRaise         bool       `json:"assurance_raise"`
	DemandID               string     `json:"demand_id"`
	PlanVersion            string     `json:"plan_version"`
	PolicyVersion          string     `json:"policy_version"`
	Profile                Profile    `json:"profile"`
	TaskID                 string     `json:"task_id"`
}

type RoutingResolvedPayload struct {
	AssuranceJoinRaise bool           `json:"assurance_join_raise"`
	AttemptID          string         `json:"attempt_id"`
	DemandID           string         `json:"demand_id"`
	Harness            entity.Harness `json:"harness"`
	NativeProfileID    string         `json:"native_profile_id"`
	OwnerGeneration    int            `json:"owner_generation"`
	PolicyVersion      string         `json:"policy_version"`
	PromotedComponents []string       `json:"promoted_components"`
	ProposalBand       string         `json:"proposal_band,omitempty"`
	ProposalID         string         `json:"proposal_id,omitempty"`
	RequestedProfile   Profile        `json:"requested_profile"`
	Resolution         string         `json:"resolution"`
	Role               string         `json:"role"`
	TaskID             string         `json:"task_id"`
}

type RoutingResolutionFailedPayload struct {
	AssuranceJoinRaise bool           `json:"assurance_join_raise"`
	AttemptID          string         `json:"attempt_id"`
	DemandID           string         `json:"demand_id"`
	Harness            entity.Harness `json:"harness"`
	PolicyVersion      string         `json:"policy_version"`
	ProposalBand       string         `json:"proposal_band,omitempty"`
	ProposalID         string         `json:"proposal_id,omitempty"`
	Reason             string         `json:"reason"`
	RequestedProfile   Profile        `json:"requested_profile"`
	Role               string         `json:"role"`
	TaskID             string         `json:"task_id"`
	UnmetComponents    []string       `json:"unmet_components"`
}

type RoutedAttemptCompletedPayload struct {
	AttemptID           string `json:"attempt_id"`
	Cause               string `json:"cause"`
	DemandID            string `json:"demand_id"`
	EvidenceRef         string `json:"evidence_ref"`
	LaunchCapabilityRef string `json:"launch_capability_ref"`
	LaunchFidelity      string `json:"launch_fidelity"`
	Outcome             string `json:"outcome"`
	OwnerGeneration     int    `json:"owner_generation"`
	ProposalID          string `json:"proposal_id,omitempty"`
	TaskID              string `json:"task_id"`
	Usage               Usage  `json:"usage"`
	UsageProvenance     string `json:"usage_provenance"`
}

type RoutedTaskOutcomePayload struct {
	AttemptID           string `json:"attempt_id"`
	Cause               string `json:"cause"`
	Class               string `json:"class"`
	DemandID            string `json:"demand_id"`
	EvidenceRef         string `json:"evidence_ref"`
	LaunchCapabilityRef string `json:"launch_capability_ref"`
	LaunchFidelity      string `json:"launch_fidelity"`
	OwnerGeneration     int    `json:"owner_generation"`
	ProposalRef         string `json:"proposal_ref,omitempty"`
	TaskID              string `json:"task_id"`
	Turns               int    `json:"turns"`
	Usage               Usage  `json:"usage"`
	UsageProvenance     string `json:"usage_provenance"`
	Verification        string `json:"verification"`
}

type AttemptOwnerTransferredPayload struct {
	AttemptID            string `json:"attempt_id"`
	NewOwnerGeneration   int    `json:"new_owner_generation"`
	Observation          string `json:"observation"`
	PriorOwnerGeneration int    `json:"prior_owner_generation"`
	RuntimeFenceNonce    string `json:"runtime_fence_nonce"`
	RuntimeFenceProofRef string `json:"runtime_fence_proof_ref"`
}

type AssurancePassCompletedPayload struct {
	AttemptID            string              `json:"attempt_id"`
	Candidates           int                 `json:"candidates"`
	DemandID             string              `json:"demand_id"`
	EffectiveDepth       string              `json:"effective_depth"`
	FindingRecords       []NormalizedFinding `json:"finding_records"`
	Findings             int                 `json:"findings"`
	InternalRounds       int                 `json:"internal_rounds"`
	Killed               int                 `json:"killed"`
	PassID               string              `json:"pass_id"`
	PassIndex            int                 `json:"pass_index"`
	ProposalID           string              `json:"proposal_id"`
	Provenance           Provenance          `json:"provenance"`
	ProcessGeneration    int                 `json:"process_generation"`
	RequiredPasses       int                 `json:"required_passes"`
	ReviewProcessID      string              `json:"review_process_id"`
	ReviewedRevision     string              `json:"reviewed_revision"`
	RoutedOutcomeRef     string              `json:"routed_outcome_ref"`
	RoutingResolutionRef string              `json:"routing_resolution_ref"`
	TaskID               string              `json:"task_id"`
	Tokens               int                 `json:"tokens"`
	TopLevelContextID    string              `json:"top_level_context_id"`
	Verdict              string              `json:"verdict"`
	WallMillis           int                 `json:"wall_ms"`
}

type FindingPersistedPayload struct {
	NormalizedFinding
	PassID           string `json:"pass_id"`
	ProposalID       string `json:"proposal_id"`
	ReviewProcessID  string `json:"review_process_id"`
	ReviewedRevision string `json:"reviewed_revision"`
}

type DispositionRecordedPayload struct {
	Actor                     string `json:"actor"`
	ActorAttribution          string `json:"actor_attribution"`
	AttributionVerification   string `json:"attribution_verification"`
	FindingID                 string `json:"finding_id"`
	NativeAuthorIdentityProof string `json:"native_author_identity_proof"`
	NativeMediumActRef        string `json:"native_medium_act_ref"`
	Reason                    string `json:"reason,omitempty"`
	SourceOwnedAt             string `json:"at,omitempty"`
	Verdict                   string `json:"verdict"`
}

type ReviewProcessCompletedPayload struct {
	AggregateVerdict    string   `json:"aggregate_verdict"`
	AggregateWallMillis int      `json:"aggregate_wall_ms"`
	CompletedPasses     int      `json:"completed_passes"`
	DemandID            string   `json:"demand_id"`
	EffectiveDepth      string   `json:"effective_depth"`
	EvidenceRef         string   `json:"evidence_ref"`
	PassAttemptIDs      []string `json:"pass_attempt_ids"`
	PassVerdicts        []string `json:"pass_verdicts"`
	PlannedAssurance    string   `json:"planned_assurance"`
	ProcessGeneration   int      `json:"process_generation"`
	ProposalBand        string   `json:"proposal_band"`
	ProposalID          string   `json:"proposal_id"`
	RequiredPasses      int      `json:"required_passes"`
	Result              string   `json:"result"`
	ReviewProcessID     string   `json:"review_process_id"`
	ReviewedRevision    string   `json:"reviewed_revision"`
	TaskID              string   `json:"task_id"`
	TopLevelContextIDs  []string `json:"top_level_context_ids"`
}

type RoutingEvidenceInvalidatedPayload struct {
	AffectedArtifactRefs []string `json:"artifact_refs"`
	AffectedAttemptIDs   []string `json:"attempt_ids"`
	AffectedEventIDs     []string `json:"event_ids"`
	AffectedProcessIDs   []string `json:"review_process_ids"`
	AuthorizationRef     string   `json:"authorization_ref"`
	Disposition          string   `json:"disposition"`
	InvalidationID       string   `json:"invalidation_id"`
	NativeProfileID      string   `json:"native_profile_id"`
	PolicyVersion        string   `json:"policy_version"`
}

// Percentile and percentage values stay decimal strings. This preserves the
// upstream value without admitting JSON floats into the canonical grammar.
type ActivationCheckpointPayload struct {
	Activated               bool     `json:"activated"`
	ActivationPolicyVersion string   `json:"activation_policy_version"`
	ActivationReady         bool     `json:"activation_ready"`
	AvailabilityDefects     int      `json:"availability_defects"`
	BlockingDisposed        int      `json:"blocking_disposed"`
	BurstFlag               bool     `json:"burst_flag"`
	CoveragePct             string   `json:"coverage_pct"`
	DwellP50                string   `json:"dwell_p50"`
	EvidenceHead            string   `json:"evidence_head"`
	ExactMainlineObject     string   `json:"exact_mainline_object"`
	LaunchReadinessGreen    bool     `json:"launch_readiness_green"`
	LiveBlockingDODGreen    bool     `json:"live_blocking_dod_green"`
	MatrixCellsCovered      int      `json:"matrix_cells_covered"`
	MatrixCellsRequired     int      `json:"matrix_cells_required"`
	MainlineRef             string   `json:"mainline_ref"`
	NitP50                  string   `json:"nit_p50"`
	ParentSeamGreen         bool     `json:"parent_seam_green"`
	OperatorAuthorizationID string   `json:"operator_authorization_id"`
	Precision               string   `json:"precision,omitempty"`
	ProcessViolations       int      `json:"process_violations"`
	ProofIDs                []string `json:"proof_ids"`
	ProofManifestDigest     string   `json:"proof_manifest_digest"`
	Reviews                 int      `json:"reviews"`
}

// Payload is a closed tagged union. Exactly one member, selected by Event.Kind,
// is admitted. On the wire the selected member's fields appear directly below
// "payload"; these Go wrapper names never enter canonical bytes.
type Payload struct {
	ExecutionDemandFrozen      *ExecutionDemandFrozenPayload
	RoutingResolved            *RoutingResolvedPayload
	RoutingResolutionFailed    *RoutingResolutionFailedPayload
	RoutedAttemptCompleted     *RoutedAttemptCompletedPayload
	RoutedTaskOutcome          *RoutedTaskOutcomePayload
	AttemptOwnerTransferred    *AttemptOwnerTransferredPayload
	AssurancePassCompleted     *AssurancePassCompletedPayload
	FindingPersisted           *FindingPersistedPayload
	DispositionRecorded        *DispositionRecordedPayload
	ReviewProcessCompleted     *ReviewProcessCompletedPayload
	RoutingEvidenceInvalidated *RoutingEvidenceInvalidatedPayload
	ActivationCheckpoint       *ActivationCheckpointPayload
}

// MarshalJSON projects the selected union member without adding a second kind
// wrapper. Kind/member agreement is checked by Event before this method runs.
func (p Payload) MarshalJSON() ([]byte, error) {
	members := []any{
		p.ExecutionDemandFrozen,
		p.RoutingResolved, p.RoutingResolutionFailed, p.RoutedAttemptCompleted,
		p.RoutedTaskOutcome, p.AttemptOwnerTransferred, p.AssurancePassCompleted,
		p.FindingPersisted, p.DispositionRecorded, p.ReviewProcessCompleted,
		p.RoutingEvidenceInvalidated, p.ActivationCheckpoint,
	}
	var selected any
	for _, member := range members {
		if !nilPayload(member) {
			if selected != nil {
				return nil, fmt.Errorf("payload: multiple union members")
			}
			selected = member
		}
	}
	if selected == nil {
		return nil, fmt.Errorf("payload: no union member")
	}
	return json.Marshal(selected)
}

// Event is both the producer input and decoded stored value. EventID and
// ContentDigest may be left empty on producer input; all construction functions
// compute them and reject a non-empty mismatch.
type Event struct {
	ContentDigest string
	EventID       EventID
	EvidenceRef   string
	Kind          EventKind
	Payload       Payload
	Producer      Producer
	ProducerProof ProducerProof
	Repository    string
	Schema        string
	Version       string
}

type IdentityTuple struct {
	ActivationPolicyVersion string `json:"activation_policy_version,omitempty"`
	AttemptID               string `json:"attempt_id,omitempty"`
	DemandID                string `json:"demand_id,omitempty"`
	ExactMainlineObject     string `json:"exact_mainline_object,omitempty"`
	FindingID               string `json:"finding_id,omitempty"`
	InvalidationID          string `json:"invalidation_id,omitempty"`
	NativeMediumActRef      string `json:"native_medium_act_ref,omitempty"`
	NewOwnerGeneration      int    `json:"new_owner_generation,omitempty"`
	PassID                  string `json:"pass_id,omitempty"`
	ProcessGeneration       int    `json:"process_generation,omitempty"`
	ProofManifestDigest     string `json:"proof_manifest_digest,omitempty"`
	ReviewProcessID         string `json:"review_process_id,omitempty"`
}

type IdentityPreimage struct {
	Domain       string        `json:"domain"`
	EvidenceRef  string        `json:"evidence_ref"`
	EventKind    EventKind     `json:"event_kind"`
	EventVersion string        `json:"event_version"`
	Repository   string        `json:"repository"`
	Tuple        IdentityTuple `json:"tuple"`
}

type ContentPreimage struct {
	Domain       string    `json:"domain"`
	EventID      EventID   `json:"event_id"`
	EvidenceRef  string    `json:"evidence_ref"`
	EventKind    EventKind `json:"event_kind"`
	EventVersion string    `json:"event_version"`
	Payload      Payload   `json:"payload"`
	Producer     Producer  `json:"producer"`
	Repository   string    `json:"repository"`
	Schema       string    `json:"schema"`
}

type UnsignedEnvelope struct {
	ContentDigest string    `json:"content_digest"`
	Domain        string    `json:"domain"`
	EventID       EventID   `json:"event_id"`
	EvidenceRef   string    `json:"evidence_ref"`
	EventKind     EventKind `json:"event_kind"`
	EventVersion  string    `json:"event_version"`
	Payload       Payload   `json:"payload"`
	Producer      Producer  `json:"producer"`
	Repository    string    `json:"repository"`
	Schema        string    `json:"schema"`
}

type Envelope struct {
	ContentDigest string        `json:"content_digest"`
	Domain        string        `json:"domain"`
	EventID       EventID       `json:"event_id"`
	EvidenceRef   string        `json:"evidence_ref"`
	EventKind     EventKind     `json:"event_kind"`
	EventVersion  string        `json:"event_version"`
	Payload       Payload       `json:"payload"`
	Producer      Producer      `json:"producer"`
	ProducerProof ProducerProof `json:"producer_proof"`
	Repository    string        `json:"repository"`
	Schema        string        `json:"schema"`
}
