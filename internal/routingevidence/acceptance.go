package routingevidence

import (
	"encoding/json"
	"fmt"
	"sort"
)

const (
	AcceptanceMatrixSchema = "routing-evidence-acceptance-matrix/v1"
	EvidenceAuditSchema    = "routing-evidence-audit/v1"
	HostedRehearsalSchema  = "routing-evidence-hosted-ref-rehearsal/v1"
)

// FaultPoint is one side of a durable or side-effect boundary. A crash corpus
// is complete only when it contains every value in RequiredFaultPoints.
type FaultPoint string

const (
	FaultBeforeAppend       FaultPoint = "before_append"
	FaultAfterAppend        FaultPoint = "after_append"
	FaultBeforeCAS          FaultPoint = "before_cas"
	FaultAfterCAS           FaultPoint = "after_cas"
	FaultBeforeExactRead    FaultPoint = "before_exact_read"
	FaultAfterExactRead     FaultPoint = "after_exact_read"
	FaultBeforeClaim        FaultPoint = "before_claim"
	FaultAfterClaim         FaultPoint = "after_claim"
	FaultBeforeSpawn        FaultPoint = "before_spawn"
	FaultAfterSpawn         FaultPoint = "after_spawn"
	FaultBeforeComposite    FaultPoint = "before_composite"
	FaultAfterComposite     FaultPoint = "after_composite"
	FaultBeforeTransfer     FaultPoint = "before_transfer"
	FaultAfterTransfer      FaultPoint = "after_transfer"
	FaultBeforeProjection   FaultPoint = "before_projection"
	FaultAfterProjection    FaultPoint = "after_projection"
	FaultBeforeInvalidation FaultPoint = "before_invalidation"
	FaultAfterInvalidation  FaultPoint = "after_invalidation"
	FaultBeforeCheckpoint   FaultPoint = "before_checkpoint"
	FaultAfterCheckpoint    FaultPoint = "after_checkpoint"
)

type CoverageDimension string

const (
	CoverageRequirement CoverageDimension = "requirement"
	CoverageScenario    CoverageDimension = "scenario"
	CoverageEventKind   CoverageDimension = "event_kind"
	CoverageState       CoverageDimension = "structural_state"
	CoverageProducer    CoverageDimension = "producer_class"
	CoverageInvariant   CoverageDimension = "invariant"
	CoverageAcceptance  CoverageDimension = "acceptance"
)

// AcceptanceCell is deliberately one-dimensional. This makes an omission
// visible as one missing row instead of hiding it inside a many-valued cell.
type AcceptanceCell struct {
	Dimension         CoverageDimension `json:"dimension"`
	Value             string            `json:"value"`
	EvidenceReference string            `json:"evidence_ref"`
}

type AcceptanceMatrix struct {
	Schema string           `json:"schema"`
	Cells  []AcceptanceCell `json:"cells"`
}

type CrashCase struct {
	FaultPoint        FaultPoint `json:"fault_point"`
	DuplicateLaunch   bool       `json:"duplicate_launch"`
	PartialAuthority  bool       `json:"partial_authority"`
	SilentSuccess     bool       `json:"silent_success"`
	EvidenceReference string     `json:"evidence_ref"`
}

type MutationGuard string

const (
	GuardLaunchProof       MutationGuard = "launch_proof"
	GuardOwnerFence        MutationGuard = "owner_fence"
	GuardAtomicity         MutationGuard = "atomicity"
	GuardProducerAuthority MutationGuard = "producer_authority"
	GuardProjection        MutationGuard = "projection"
	GuardInvalidation      MutationGuard = "invalidation"
	GuardActivation        MutationGuard = "activation"
)

type MutationCase struct {
	Guard                MutationGuard `json:"guard"`
	DesignatedCaseFailed bool          `json:"designated_case_failed"`
	EvidenceReference    string        `json:"evidence_ref"`
}

type HostedFailureMode string

const (
	HostedOutage  HostedFailureMode = "outage"
	HostedForce   HostedFailureMode = "force"
	HostedDelete  HostedFailureMode = "delete"
	HostedRewrite HostedFailureMode = "rewrite"
)

type HostedFailure struct {
	Mode            HostedFailureMode `json:"mode"`
	Rejected        bool              `json:"rejected"`
	OperatorVisible bool              `json:"operator_visible"`
}

// HostedRefRehearsal is provider-neutral recorded evidence. The hosted
// provider remains outside core logic; this contract records only the same
// protection predicate exercised by the local bare-git adapter.
type HostedRefRehearsal struct {
	Schema                  string          `json:"schema"`
	EvidenceReference       string          `json:"evidence_ref"`
	NonRewritingContinuity  bool            `json:"non_rewriting_continuity"`
	FreshReadReconstruction bool            `json:"fresh_read_reconstruction"`
	BoundedRetries          bool            `json:"bounded_retries"`
	PayloadExcluded         bool            `json:"payload_excluded"`
	Failures                []HostedFailure `json:"failures"`
}

type EvidenceReportClass string

const (
	ReportConformance EvidenceReportClass = "conformance"
	ReportCrash       EvidenceReportClass = "crash"
	ReportSecurity    EvidenceReportClass = "security"
	ReportPayload     EvidenceReportClass = "payload"
	ReportHostedRef   EvidenceReportClass = "hosted_ref"
	ReportFreshClone  EvidenceReportClass = "fresh_clone"
)

type EvidenceReport struct {
	Class             EvidenceReportClass `json:"class"`
	EvidenceReference string              `json:"evidence_ref"`
}

// EvidenceAudit is the reviewable join across the bounded reports required by
// routing-evidence-v1 A-1 through A-9. Validate fails closed on every omitted,
// duplicated, foreign, or favorable-but-incomplete row.
type EvidenceAudit struct {
	Schema    string             `json:"schema"`
	Matrix    AcceptanceMatrix   `json:"acceptance_matrix"`
	Crashes   []CrashCase        `json:"crash_cases"`
	Mutations []MutationCase     `json:"mutation_cases"`
	Hosted    HostedRefRehearsal `json:"hosted_ref_rehearsal"`
	Reports   []EvidenceReport   `json:"reports"`
}

func RequiredFaultPoints() []FaultPoint {
	return []FaultPoint{
		FaultBeforeAppend, FaultAfterAppend, FaultBeforeCAS, FaultAfterCAS,
		FaultBeforeExactRead, FaultAfterExactRead, FaultBeforeClaim, FaultAfterClaim,
		FaultBeforeSpawn, FaultAfterSpawn, FaultBeforeComposite, FaultAfterComposite,
		FaultBeforeTransfer, FaultAfterTransfer, FaultBeforeProjection, FaultAfterProjection,
		FaultBeforeInvalidation, FaultAfterInvalidation, FaultBeforeCheckpoint, FaultAfterCheckpoint,
	}
}

func RequiredAcceptanceCoverage() map[CoverageDimension][]string {
	events := []string{
		string(ExecutionDemandFrozen), string(RoutingResolved),
		string(RoutingResolutionFailed), string(RoutedAttemptCompleted), string(RoutedTaskOutcome),
		string(AttemptOwnerTransferred), string(AssurancePassCompleted), string(FindingPersisted),
		string(DispositionRecorded), string(ReviewProcessCompleted), string(RoutingEvidenceInvalidated),
		string(ActivationCheckpointEvent),
	}
	producers := []string{
		string(ProducerPlanGate), string(ProducerResolver),
		string(ProducerAttemptOwner), string(ProducerAttemptReaper), string(ProducerRecoveryCoordinator),
		string(ProducerReviewOrchestrator), string(ProducerMediumAdapter), string(ProducerEvidenceOwner),
		string(ProducerReleaseGate),
	}
	invariants, acceptance := make([]string, 15), make([]string, 9)
	for i := range invariants {
		invariants[i] = fmt.Sprintf("DINV-%d", i+1)
	}
	for i := range acceptance {
		acceptance[i] = fmt.Sprintf("A-%d", i+1)
	}
	return map[CoverageDimension][]string{
		CoverageRequirement: {"FR-20", "FR-21", "FR-24", "UB-7", "UB-10", "UB-14", "NFR-4", "NFR-8"},
		CoverageScenario:    {"exact", "promoted", "unsupported", "implementation", "review", "recovery", "invalidation", "activation"},
		CoverageEventKind:   events,
		CoverageState:       {string(StateResolutionFailed), string(StateResolvedPendingOutcome), string(StateResolvedTerminal), string(StateProcessInFlight), string(StateProcessTerminal), string(StateInvalidated)},
		CoverageProducer:    producers, CoverageInvariant: invariants, CoverageAcceptance: acceptance,
	}
}

func (m AcceptanceMatrix) Validate() error {
	if m.Schema != AcceptanceMatrixSchema {
		return fmt.Errorf("routing evidence acceptance matrix: unsupported schema %q", m.Schema)
	}
	required := RequiredAcceptanceCoverage()
	want := make(map[string]bool)
	for dimension, values := range required {
		for _, value := range values {
			want[coverageKey(dimension, value)] = true
		}
	}
	seen := make(map[string]bool, len(m.Cells))
	for _, cell := range m.Cells {
		key := coverageKey(cell.Dimension, cell.Value)
		if !want[key] || seen[key] || cell.EvidenceReference == "" {
			return fmt.Errorf("routing evidence acceptance matrix: invalid cell %s", key)
		}
		seen[key] = true
	}
	missing := make([]string, 0)
	for key := range want {
		if !seen[key] {
			missing = append(missing, key)
		}
	}
	if len(missing) != 0 {
		sort.Strings(missing)
		return fmt.Errorf("routing evidence acceptance matrix: omitted cells %v", missing)
	}
	return nil
}

func (r HostedRefRehearsal) Validate() error {
	if r.Schema != HostedRehearsalSchema || r.EvidenceReference == "" || !r.NonRewritingContinuity || !r.FreshReadReconstruction || !r.BoundedRetries || !r.PayloadExcluded {
		return fmt.Errorf("routing evidence hosted-ref rehearsal: protection predicate is incomplete")
	}
	want := map[HostedFailureMode]bool{HostedOutage: true, HostedForce: true, HostedDelete: true, HostedRewrite: true}
	seen := map[HostedFailureMode]bool{}
	for _, failure := range r.Failures {
		if !want[failure.Mode] || seen[failure.Mode] || !failure.Rejected || !failure.OperatorVisible {
			return fmt.Errorf("routing evidence hosted-ref rehearsal: invalid failure row %q", failure.Mode)
		}
		seen[failure.Mode] = true
	}
	if len(seen) != len(want) {
		return fmt.Errorf("routing evidence hosted-ref rehearsal: failure mode omitted")
	}
	return nil
}

func (a EvidenceAudit) Validate() error {
	if a.Schema != EvidenceAuditSchema {
		return fmt.Errorf("routing evidence audit: unsupported schema %q", a.Schema)
	}
	if err := a.Matrix.Validate(); err != nil {
		return err
	}
	if err := validateClosedRows(a.Crashes, RequiredFaultPoints(), func(v CrashCase) FaultPoint { return v.FaultPoint }, func(v CrashCase) bool {
		return !v.DuplicateLaunch && !v.PartialAuthority && !v.SilentSuccess && v.EvidenceReference != ""
	}); err != nil {
		return fmt.Errorf("routing evidence audit: crash corpus: %w", err)
	}
	guards := []MutationGuard{GuardLaunchProof, GuardOwnerFence, GuardAtomicity, GuardProducerAuthority, GuardProjection, GuardInvalidation, GuardActivation}
	if err := validateClosedRows(a.Mutations, guards, func(v MutationCase) MutationGuard { return v.Guard }, func(v MutationCase) bool {
		return v.DesignatedCaseFailed && v.EvidenceReference != ""
	}); err != nil {
		return fmt.Errorf("routing evidence audit: mutation corpus: %w", err)
	}
	if err := a.Hosted.Validate(); err != nil {
		return err
	}
	reports := []EvidenceReportClass{ReportConformance, ReportCrash, ReportSecurity, ReportPayload, ReportHostedRef, ReportFreshClone}
	if err := validateClosedRows(a.Reports, reports, func(v EvidenceReport) EvidenceReportClass { return v.Class }, func(v EvidenceReport) bool { return v.EvidenceReference != "" }); err != nil {
		return fmt.Errorf("routing evidence audit: reports: %w", err)
	}
	return nil
}

func EncodeEvidenceAudit(a EvidenceAudit) ([]byte, error) {
	if err := a.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(a)
}

func coverageKey(dimension CoverageDimension, value string) string {
	return string(dimension) + "/" + value
}

func validateClosedRows[T any, K comparable](rows []T, required []K, key func(T) K, valid func(T) bool) error {
	want, seen := make(map[K]bool, len(required)), make(map[K]bool, len(rows))
	for _, value := range required {
		want[value] = true
	}
	for _, row := range rows {
		value := key(row)
		if !want[value] || seen[value] || !valid(row) {
			return fmt.Errorf("invalid, duplicate, or foreign row %v", value)
		}
		seen[value] = true
	}
	if len(seen) != len(want) {
		return fmt.Errorf("required row omitted")
	}
	return nil
}
