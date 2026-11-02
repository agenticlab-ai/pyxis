package routingevidence

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"sort"

	implverdict "github.com/agenticlab-ai/pyxis/internal/implreview/verdict"
)

// StructuralStateName is the closed set of consequence-bearing states in
// routing-evidence-v1 §3.2. Demand origins and activation checkpoints have
// their own views; they do not invent an additional routing-chain state.
type StructuralStateName string

const (
	StateResolutionFailed       StructuralStateName = "resolution_failed"
	StateResolvedPendingOutcome StructuralStateName = "resolved_pending_outcome"
	StateResolvedTerminal       StructuralStateName = "resolved_terminal"
	StateProcessInFlight        StructuralStateName = "process_in_flight"
	StateProcessTerminal        StructuralStateName = "process_terminal"
	StateInvalidated            StructuralStateName = "invalidated"
)

type SubjectKind string

const (
	SubjectDemand       SubjectKind = "demand"
	SubjectAttempt      SubjectKind = "attempt"
	SubjectProcess      SubjectKind = "process"
	SubjectInvalidation SubjectKind = "invalidation"
	SubjectActivation   SubjectKind = "activation"
)

type SubjectID struct {
	Kind SubjectKind `json:"kind"`
	ID   string      `json:"id"`
}

type DemandView struct {
	DemandID        string   `json:"demand_id"`
	OriginEventID   EventID  `json:"origin_event_id"`
	AttemptIDs      []string `json:"attempt_ids"`
	InvalidationIDs []string `json:"invalidation_ids"`
}

type AttemptView struct {
	AttemptID          string              `json:"attempt_id"`
	DemandID           string              `json:"demand_id"`
	TaskID             string              `json:"task_id"`
	Role               string              `json:"role"`
	State              StructuralStateName `json:"state"`
	ResolutionEventID  EventID             `json:"resolution_event_id"`
	OutcomeEventID     EventID             `json:"outcome_event_id,omitempty"`
	TaskOutcomeEventID EventID             `json:"task_outcome_event_id,omitempty"`
	OwnerGeneration    int                 `json:"owner_generation,omitempty"`
	TransferEventIDs   []EventID           `json:"transfer_event_ids"`
	PassIDs            []string            `json:"pass_ids"`
	InvalidationIDs    []string            `json:"invalidation_ids"`
	LaunchEligible     bool                `json:"launch_eligible"`
	ProcessEligible    bool                `json:"process_eligible"`
}

type PassView struct {
	PassID              string    `json:"pass_id"`
	AttemptID           string    `json:"attempt_id"`
	PassIndex           int       `json:"pass_index"`
	TerminalEventID     EventID   `json:"terminal_event_id"`
	FindingEventIDs     []EventID `json:"finding_event_ids"`
	DispositionEventIDs []EventID `json:"disposition_event_ids"`
}

type ProcessView struct {
	ReviewProcessID    string              `json:"review_process_id"`
	ProcessGeneration  int                 `json:"process_generation"`
	State              StructuralStateName `json:"state"`
	Passes             []PassView          `json:"passes"`
	TerminalEventID    EventID             `json:"terminal_event_id,omitempty"`
	AggregateVerdict   string              `json:"aggregate_verdict,omitempty"`
	Result             string              `json:"result,omitempty"`
	InvalidationIDs    []string            `json:"invalidation_ids"`
	ProcessEligible    bool                `json:"process_eligible"`
	ProjectionEligible bool                `json:"projection_eligible"`
	Satisfied          bool                `json:"satisfied"`
}

type InvalidationView struct {
	InvalidationID string              `json:"invalidation_id"`
	EventID        EventID             `json:"event_id"`
	State          StructuralStateName `json:"state"`
	Disposition    string              `json:"disposition"`
}

type ActivationView struct {
	ActivationPolicyVersion string   `json:"activation_policy_version"`
	ExactMainlineObject     string   `json:"exact_mainline_object"`
	ProofManifestDigest     string   `json:"proof_manifest_digest"`
	EventID                 EventID  `json:"event_id"`
	InvalidationIDs         []string `json:"invalidation_ids"`
	ActivationEligible      bool     `json:"activation_eligible"`
}

type SubjectView struct {
	Subject           SubjectID          `json:"subject"`
	Demand            *DemandView        `json:"demand,omitempty"`
	Attempt           *AttemptView       `json:"attempt,omitempty"`
	Process           *ProcessView       `json:"process,omitempty"`
	Invalidation      *InvalidationView  `json:"invalidation,omitempty"`
	Activation        *ActivationView    `json:"activation,omitempty"`
	Invalidations     []InvalidationView `json:"invalidations"`
	RemediationBriefs []RemediationBrief `json:"remediation_briefs"`
}

// StructuralState is the deterministic read model for one exact repository
// and evidence ref. Slices are sorted by semantic identity, never input order.
type StructuralState struct {
	Repository        string             `json:"repository"`
	EvidenceRef       string             `json:"evidence_ref"`
	EvidenceHead      string             `json:"evidence_head,omitempty"`
	Demands           []DemandView       `json:"demands"`
	Attempts          []AttemptView      `json:"attempts"`
	Processes         []ProcessView      `json:"processes"`
	Invalidations     []InvalidationView `json:"invalidations"`
	RemediationBriefs []RemediationBrief `json:"remediation_briefs"`
	Activations       []ActivationView   `json:"activations"`
	events            []Event
	producerPolicy    ProducerPolicy
}

// State is the transition API's short name from the design.
type State = StructuralState

type EventOrComposite struct {
	Events []Event
}

var ErrReducerIntegrity = errors.New("routing evidence reducer integrity failure")

type ReducerErrorCode string

const (
	ReducerCanonical        ReducerErrorCode = "canonical_invalid"
	ReducerConflict         ReducerErrorCode = "identity_conflict"
	ReducerDuplicate        ReducerErrorCode = "duplicate_terminal"
	ReducerMissingParent    ReducerErrorCode = "missing_parent"
	ReducerCrossBoundary    ReducerErrorCode = "cross_boundary_join"
	ReducerInvalidJoin      ReducerErrorCode = "invalid_join"
	ReducerCardinality      ReducerErrorCode = "cardinality_invalid"
	ReducerPartialComposite ReducerErrorCode = "partial_composite"
)

// ReducerError names the exact failed edge so callers never need to parse
// generic prose to distinguish an orphan, cross-subject join, or partial
// atomic composite.
type ReducerError struct {
	Code    ReducerErrorCode
	Edge    string
	Subject string
	Detail  string
}

func (e *ReducerError) Error() string {
	return fmt.Sprintf("%s edge=%s subject=%s: %s", e.Code, e.Edge, e.Subject, e.Detail)
}
func (e *ReducerError) Unwrap() error { return ErrReducerIntegrity }

func reducerError(code ReducerErrorCode, edge, subject, format string, args ...any) error {
	return &ReducerError{Code: code, Edge: edge, Subject: subject, Detail: fmt.Sprintf(format, args...)}
}

type Reducer struct {
	ProducerPolicy ProducerPolicy
}

func NewReducer(policy ProducerPolicy) Reducer { return Reducer{ProducerPolicy: policy} }

// Reduce authenticates and joins a complete reachable event set. It is a set
// reducer: byte-identical repeats are idempotent and every output order is
// derived from semantic identities.
func (r Reducer) Reduce(events []Event) (StructuralState, error) {
	canonical, err := r.admit(events)
	if err != nil {
		return StructuralState{}, err
	}
	state, err := reduceCanonical(canonical)
	state.producerPolicy = r.ProducerPolicy
	return state, err
}

// BindEvidenceHead attaches the exact fetched head to an already reduced
// state. Reducers intentionally cannot invent this git object themselves.
func BindEvidenceHead(state State, head ObservedHead) (State, error) {
	if head.EvidenceRef == "" || head.Commit == "" || head.EvidenceRef != state.EvidenceRef {
		return State{}, reducerError(ReducerCrossBoundary, "state->evidence_head", head.Commit, "head does not identify the reduced evidence ref")
	}
	state.EvidenceHead = head.Commit
	return state, nil
}

// ValidateTransition validates the transaction boundary as well as the
// resulting state. Composite children must arrive with their parent even when
// a malformed prior state would otherwise appear to supply it.
func (r Reducer) ValidateTransition(state State, next EventOrComposite) (State, error) {
	if len(next.Events) == 0 {
		return State{}, reducerError(ReducerCardinality, "transition.events", "transition", "event or composite is empty")
	}
	if err := validateAtomicBoundary(state.events, next.Events); err != nil {
		return State{}, err
	}
	combined := append(append([]Event(nil), state.events...), next.Events...)
	return r.Reduce(combined)
}

func (r Reducer) admit(events []Event) ([]Event, error) {
	byID := make(map[EventID][]byte, len(events))
	accepted := make(map[EventID]Event, len(events))
	for _, event := range events {
		wire, err := EncodeCanonical(event)
		if err != nil {
			return nil, reducerError(ReducerCanonical, "event.canonical", string(event.EventID), "%v", err)
		}
		decoded, err := DecodeCanonical(wire)
		if err != nil {
			return nil, reducerError(ReducerCanonical, "event.canonical", string(event.EventID), "%v", err)
		}
		if err := VerifyProducer(decoded, r.ProducerPolicy); err != nil {
			return nil, err
		}
		if prior, ok := byID[decoded.EventID]; ok {
			if !bytes.Equal(prior, wire) {
				return nil, reducerError(ReducerConflict, "event_id->canonical_bytes", string(decoded.EventID), "same identity has different canonical bytes")
			}
			continue
		}
		byID[decoded.EventID], accepted[decoded.EventID] = wire, decoded
	}
	ids := make([]string, 0, len(accepted))
	for id := range accepted {
		ids = append(ids, string(id))
	}
	sort.Strings(ids)
	out := make([]Event, 0, len(ids))
	for _, id := range ids {
		out = append(out, accepted[EventID(id)])
	}
	return out, nil
}

func (s StructuralState) SubjectView(subject SubjectID) (SubjectView, error) {
	view := SubjectView{Subject: subject, Invalidations: []InvalidationView{}, RemediationBriefs: []RemediationBrief{}}
	switch subject.Kind {
	case SubjectDemand:
		for i := range s.Demands {
			if s.Demands[i].DemandID == subject.ID {
				copy := s.Demands[i]
				view.Demand = &copy
				return s.withInvalidationDetails(view, copy.InvalidationIDs), nil
			}
		}
	case SubjectAttempt:
		for i := range s.Attempts {
			if s.Attempts[i].AttemptID == subject.ID {
				copy := s.Attempts[i]
				view.Attempt = &copy
				return s.withInvalidationDetails(view, copy.InvalidationIDs), nil
			}
		}
	case SubjectProcess:
		for i := range s.Processes {
			if s.Processes[i].ReviewProcessID == subject.ID {
				copy := s.Processes[i]
				view.Process = &copy
				return s.withInvalidationDetails(view, copy.InvalidationIDs), nil
			}
		}
	case SubjectInvalidation:
		for i := range s.Invalidations {
			if s.Invalidations[i].InvalidationID == subject.ID {
				copy := s.Invalidations[i]
				view.Invalidation = &copy
				return s.withInvalidationDetails(view, []string{copy.InvalidationID}), nil
			}
		}
	case SubjectActivation:
		for i := range s.Activations {
			if activationSubject(s.Activations[i]) == subject.ID {
				copy := s.Activations[i]
				view.Activation = &copy
				return s.withInvalidationDetails(view, copy.InvalidationIDs), nil
			}
		}
	default:
		return SubjectView{}, reducerError(ReducerInvalidJoin, "subject.kind", subject.ID, "unknown subject kind %q", subject.Kind)
	}
	return SubjectView{}, reducerError(ReducerMissingParent, "subject.id->view", subject.ID, "subject is not reachable")
}

func (s StructuralState) withInvalidationDetails(view SubjectView, ids []string) SubjectView {
	for _, id := range ids {
		for _, invalidation := range s.Invalidations {
			if invalidation.InvalidationID == id {
				view.Invalidations = append(view.Invalidations, invalidation)
			}
		}
		for _, brief := range s.RemediationBriefs {
			if brief.InvalidationID == id {
				view.RemediationBriefs = append(view.RemediationBriefs, brief)
			}
		}
	}
	return view
}

// EncodeCanonicalView makes the order-independence assertion available to
// downstream stores without exposing reducer internals.
func EncodeCanonicalView(value any) ([]byte, error) { return canonicalJSON(value) }

func activationSubject(v ActivationView) string {
	return v.ActivationPolicyVersion + "\x00" + v.ExactMainlineObject + "\x00" + v.ProofManifestDigest
}

type reductionIndex struct {
	events        map[EventID]Event
	demands       map[string]Event
	resolutions   map[string]Event
	outcomes      map[string]Event
	taskOutcomes  map[string]Event
	transfers     map[string][]Event
	passes        map[string]Event
	findings      map[string]Event
	dispositions  map[string][]Event
	processes     map[string]Event
	invalidations []Event
	activations   []Event
}

func reduceCanonical(events []Event) (StructuralState, error) {
	idx := reductionIndex{
		events: make(map[EventID]Event), demands: make(map[string]Event), resolutions: make(map[string]Event),
		outcomes: make(map[string]Event), taskOutcomes: make(map[string]Event), transfers: make(map[string][]Event),
		passes: make(map[string]Event), findings: make(map[string]Event), dispositions: make(map[string][]Event),
		processes: make(map[string]Event),
	}
	state := StructuralState{
		Demands: []DemandView{}, Attempts: []AttemptView{}, Processes: []ProcessView{},
		Invalidations: []InvalidationView{}, RemediationBriefs: []RemediationBrief{}, Activations: []ActivationView{},
		events: append([]Event(nil), events...),
	}
	for _, event := range events {
		idx.events[event.EventID] = event
		if state.Repository == "" {
			state.Repository, state.EvidenceRef = event.Repository, event.EvidenceRef
		} else if event.Repository != state.Repository || event.EvidenceRef != state.EvidenceRef {
			return StructuralState{}, reducerError(ReducerCrossBoundary, "event->repository/ref", string(event.EventID), "got %s %s, want %s %s", event.Repository, event.EvidenceRef, state.Repository, state.EvidenceRef)
		}
		if err := indexEvent(&idx, event); err != nil {
			return StructuralState{}, err
		}
	}
	if err := validateJoins(&idx); err != nil {
		return StructuralState{}, err
	}
	buildViews(&state, &idx)
	return state, nil
}

func indexEvent(idx *reductionIndex, event Event) error {
	put := func(m map[string]Event, key, edge string, duplicateCode ReducerErrorCode) error {
		if prior, ok := m[key]; ok {
			return reducerError(duplicateCode, edge, key, "events %s and %s occupy the same identity", prior.EventID, event.EventID)
		}
		m[key] = event
		return nil
	}
	switch event.Kind {
	case ExecutionDemandFrozen:
		return put(idx.demands, event.Payload.ExecutionDemandFrozen.DemandID, "demand_id->origin", ReducerCardinality)

	case RoutingResolved:
		return put(idx.resolutions, event.Payload.RoutingResolved.AttemptID, "attempt_id->resolution_terminal", ReducerDuplicate)
	case RoutingResolutionFailed:
		return put(idx.resolutions, event.Payload.RoutingResolutionFailed.AttemptID, "attempt_id->resolution_terminal", ReducerDuplicate)
	case RoutedAttemptCompleted:
		return put(idx.outcomes, event.Payload.RoutedAttemptCompleted.AttemptID, "attempt_id->routed_attempt_completed", ReducerDuplicate)
	case RoutedTaskOutcome:
		return put(idx.taskOutcomes, event.Payload.RoutedTaskOutcome.AttemptID, "attempt_id->routed_task_outcome", ReducerDuplicate)
	case AttemptOwnerTransferred:
		p := event.Payload.AttemptOwnerTransferred
		idx.transfers[p.AttemptID] = append(idx.transfers[p.AttemptID], event)
	case AssurancePassCompleted:
		return put(idx.passes, event.Payload.AssurancePassCompleted.PassID, "pass_id->assurance_pass_completed", ReducerDuplicate)
	case FindingPersisted:
		return put(idx.findings, event.Payload.FindingPersisted.FindingID, "finding_id->finding_persisted", ReducerDuplicate)
	case DispositionRecorded:
		p := event.Payload.DispositionRecorded
		idx.dispositions[p.FindingID] = append(idx.dispositions[p.FindingID], event)
	case ReviewProcessCompleted:
		return put(idx.processes, event.Payload.ReviewProcessCompleted.ReviewProcessID, "review_process_id->terminal", ReducerDuplicate)
	case RoutingEvidenceInvalidated:
		idx.invalidations = append(idx.invalidations, event)
	case ActivationCheckpointEvent:
		idx.activations = append(idx.activations, event)
	}
	return nil
}

func validateJoins(idx *reductionIndex) error {
	for attemptID, terminal := range idx.resolutions {
		var demandID, taskID, policy string
		var profile Profile
		switch terminal.Kind {
		case RoutingResolved:
			p := terminal.Payload.RoutingResolved
			demandID, taskID, policy, profile = p.DemandID, p.TaskID, p.PolicyVersion, p.RequestedProfile
		case RoutingResolutionFailed:
			p := terminal.Payload.RoutingResolutionFailed
			demandID, taskID, policy, profile = p.DemandID, p.TaskID, p.PolicyVersion, p.RequestedProfile
		}
		origin, ok := idx.demands[demandID]
		if !ok {
			return reducerError(ReducerMissingParent, "resolution.demand_id->demand_origin", attemptID, "demand %q is absent", demandID)
		}
		originTask, originPolicy, originProfile := demandFields(origin)
		if taskID != originTask || policy != originPolicy || !reflect.DeepEqual(profile, originProfile) {
			return reducerError(ReducerCrossBoundary, "resolution->demand_origin", attemptID, "task, policy, or requested profile differs from demand %q", demandID)
		}
	}
	for attemptID, transfers := range idx.transfers {
		terminal, ok := idx.resolutions[attemptID]
		if !ok || terminal.Kind != RoutingResolved {
			return reducerError(ReducerMissingParent, "owner_transfer.attempt_id->routing_resolved", attemptID, "resolved attempt is absent")
		}
		sort.Slice(transfers, func(i, j int) bool {
			return transfers[i].Payload.AttemptOwnerTransferred.NewOwnerGeneration < transfers[j].Payload.AttemptOwnerTransferred.NewOwnerGeneration
		})
		generation := terminal.Payload.RoutingResolved.OwnerGeneration
		for _, transfer := range transfers {
			p := transfer.Payload.AttemptOwnerTransferred
			if p.PriorOwnerGeneration != generation || p.NewOwnerGeneration != generation+1 {
				return reducerError(ReducerCardinality, "owner_generation->next_generation", attemptID, "transfer %s does not continue generation %d", transfer.EventID, generation)
			}
			generation = p.NewOwnerGeneration
		}
		idx.transfers[attemptID] = transfers
	}
	for attemptID, outcome := range idx.outcomes {
		resolution, ok := idx.resolutions[attemptID]
		if !ok {
			return reducerError(ReducerMissingParent, "outcome.attempt_id->routing_resolved", attemptID, "resolution terminal is absent")
		}
		if resolution.Kind != RoutingResolved {
			return reducerError(ReducerInvalidJoin, "outcome->resolution_failed", attemptID, "failed resolutions cannot have outcomes")
		}
		rp, op := resolution.Payload.RoutingResolved, outcome.Payload.RoutedAttemptCompleted
		generation := rp.OwnerGeneration + len(idx.transfers[attemptID])
		if op.DemandID != rp.DemandID || op.TaskID != rp.TaskID || op.ProposalID != rp.ProposalID || op.OwnerGeneration != generation || op.EvidenceRef != string(resolution.EventID) {
			return reducerError(ReducerCrossBoundary, "routed_attempt_completed->routing_resolved", attemptID, "demand, task, proposal, owner generation, or evidence reference differs")
		}
		child, hasTaskOutcome := idx.taskOutcomes[attemptID]
		if rp.Role == "implementation" {
			if !hasTaskOutcome {
				return reducerError(ReducerPartialComposite, "routed_attempt_completed->routed_task_outcome", attemptID, "implementation outcome composite is incomplete")
			}
			if err := validateTaskOutcome(outcome, child); err != nil {
				return err
			}
		} else if hasTaskOutcome {
			return reducerError(ReducerInvalidJoin, "routed_task_outcome->review_attempt", attemptID, "review attempts emit only routed_attempt_completed")
		}
	}
	for attemptID := range idx.taskOutcomes {
		if _, ok := idx.outcomes[attemptID]; !ok {
			return reducerError(ReducerMissingParent, "routed_task_outcome->routed_attempt_completed", attemptID, "parent outcome is absent")
		}
	}
	if err := validatePasses(idx); err != nil {
		return err
	}
	if err := validateProcesses(idx); err != nil {
		return err
	}
	return validateInvalidations(idx)
}

func demandFields(event Event) (string, string, Profile) {
	p := event.Payload.ExecutionDemandFrozen
	return p.TaskID, p.PolicyVersion, p.Profile
}

func validateTaskOutcome(parent, child Event) error {
	p, c := parent.Payload.RoutedAttemptCompleted, child.Payload.RoutedTaskOutcome
	wantClass, wantVerification := mapOutcome(p.Outcome)
	proposal := p.ProposalID
	if c.AttemptID != p.AttemptID || c.Cause != p.Cause || c.DemandID != p.DemandID || c.TaskID != p.TaskID || c.ProposalRef != proposal || c.EvidenceRef != string(parent.EventID) || c.LaunchCapabilityRef != p.LaunchCapabilityRef || c.LaunchFidelity != p.LaunchFidelity || c.OwnerGeneration != p.OwnerGeneration || c.Class != wantClass || c.Verification != wantVerification || c.Usage != p.Usage || c.UsageProvenance != p.UsageProvenance {
		return reducerError(ReducerCrossBoundary, "routed_task_outcome->routed_attempt_completed", p.AttemptID, "shared fields or terminal mapping differ")
	}
	return nil
}

func mapOutcome(outcome string) (string, string) {
	switch outcome {
	case "verified":
		return "succeeded", "verified"
	case "failed":
		return "failed", "failed"
	default:
		return "blocked", "not_reached"
	}
}

func validatePasses(idx *reductionIndex) error {
	claimedFindings := make(map[string]string)
	for passID, event := range idx.passes {
		p := event.Payload.AssurancePassCompleted
		outcome, ok := idx.outcomes[p.AttemptID]
		if !ok {
			return reducerError(ReducerMissingParent, "pass.attempt_id->review_outcome", passID, "attempt %q has no outcome", p.AttemptID)
		}
		resolution := idx.resolutions[p.AttemptID]
		if resolution.Kind != RoutingResolved || resolution.Payload.RoutingResolved.Role != "review" || outcome.Payload.RoutedAttemptCompleted.Outcome != "verified" {
			return reducerError(ReducerInvalidJoin, "assurance_pass_completed->verified_review_attempt", passID, "attempt is not a verified review route")
		}
		rp := resolution.Payload.RoutingResolved
		if p.DemandID != rp.DemandID || p.TaskID != rp.TaskID || p.ProposalID != rp.ProposalID || p.RoutingResolutionRef != string(resolution.EventID) || p.RoutedOutcomeRef != string(outcome.EventID) {
			return reducerError(ReducerCrossBoundary, "assurance_pass_completed->review_attempt", passID, "demand, task, proposal, or evidence reference differs")
		}
		for _, record := range p.FindingRecords {
			child, ok := idx.findings[record.FindingID]
			if !ok {
				return reducerError(ReducerPartialComposite, "pass.finding_records->finding_persisted", passID, "finding %q is absent", record.FindingID)
			}
			cp := child.Payload.FindingPersisted
			if cp.PassID != passID || cp.ProposalID != p.ProposalID || cp.ReviewProcessID != p.ReviewProcessID || cp.ReviewedRevision != p.ReviewedRevision || !reflect.DeepEqual(cp.NormalizedFinding, record) {
				return reducerError(ReducerCrossBoundary, "finding_persisted->assurance_pass_completed", record.FindingID, "parent identity, revision, or normalized bytes differ")
			}
			if prior := claimedFindings[record.FindingID]; prior != "" && prior != passID {
				return reducerError(ReducerCardinality, "finding_id->pass_id", record.FindingID, "claimed by passes %q and %q", prior, passID)
			}
			claimedFindings[record.FindingID] = passID
		}
	}
	for findingID, child := range idx.findings {
		if claimedFindings[findingID] == "" {
			return reducerError(ReducerMissingParent, "finding_persisted.pass_id->pass_manifest", findingID, "pass %q does not declare this finding", child.Payload.FindingPersisted.PassID)
		}
	}
	for findingID, dispositions := range idx.dispositions {
		if _, ok := idx.findings[findingID]; !ok {
			return reducerError(ReducerMissingParent, "disposition.finding_id->finding_persisted", findingID, "finding is absent")
		}
		seenAct := map[string]bool{}
		for _, disposition := range dispositions {
			act := disposition.Payload.DispositionRecorded.NativeMediumActRef
			if seenAct[act] {
				return reducerError(ReducerCardinality, "finding/native_medium_act_ref->disposition", findingID, "native act %q is duplicated", act)
			}
			seenAct[act] = true
		}
	}
	return nil
}

func validateProcesses(idx *reductionIndex) error {
	passesByProcess := make(map[string][]Event)
	for _, pass := range idx.passes {
		p := pass.Payload.AssurancePassCompleted
		passesByProcess[p.ReviewProcessID] = append(passesByProcess[p.ReviewProcessID], pass)
	}
	for processID, passes := range passesByProcess {
		sort.Slice(passes, func(i, j int) bool {
			return passes[i].Payload.AssurancePassCompleted.PassIndex < passes[j].Payload.AssurancePassCompleted.PassIndex
		})
		base := passes[0].Payload.AssurancePassCompleted
		seenIndex := map[int]bool{}
		for _, event := range passes {
			p := event.Payload.AssurancePassCompleted
			if seenIndex[p.PassIndex] {
				return reducerError(ReducerCardinality, "process/pass_index->pass", processID, "pass index %d is duplicated", p.PassIndex)
			}
			seenIndex[p.PassIndex] = true
			if p.ProcessGeneration != base.ProcessGeneration || p.DemandID != base.DemandID || p.TaskID != base.TaskID || p.ProposalID != base.ProposalID || p.ReviewedRevision != base.ReviewedRevision || p.EffectiveDepth != base.EffectiveDepth || p.RequiredPasses != base.RequiredPasses {
				return reducerError(ReducerCrossBoundary, "pass->review_process", p.PassID, "process generation, subject, revision, depth, or required count differs")
			}
		}
		passesByProcess[processID] = passes
	}
	for processID, terminal := range idx.processes {
		p := terminal.Payload.ReviewProcessCompleted
		passes := passesByProcess[processID]
		if len(passes) != p.CompletedPasses {
			return reducerError(ReducerCardinality, "process.completed_passes->pass_composites", processID, "declares %d, reachable set has %d", p.CompletedPasses, len(passes))
		}
		if p.Result == "satisfied" && p.CompletedPasses != p.RequiredPasses {
			return reducerError(ReducerPartialComposite, "process.satisfied->required_passes", processID, "satisfied terminal is missing required passes")
		}
		contexts, attempts := map[string]bool{}, map[string]bool{}
		conflict := false
		for i, event := range passes {
			pass := event.Payload.AssurancePassCompleted
			if pass.ProcessGeneration != p.ProcessGeneration || pass.DemandID != p.DemandID || pass.TaskID != p.TaskID || pass.ProposalID != p.ProposalID || pass.ReviewedRevision != p.ReviewedRevision || pass.EffectiveDepth != p.EffectiveDepth || pass.RequiredPasses != p.RequiredPasses || p.PassAttemptIDs[i] != pass.AttemptID || p.PassVerdicts[i] != pass.Verdict || p.TopLevelContextIDs[i] != pass.TopLevelContextID {
				return reducerError(ReducerCrossBoundary, "review_process_completed->pass_set", processID, "pass %q differs from terminal manifest", pass.PassID)
			}
			if pass.PassIndex != i+1 || contexts[pass.TopLevelContextID] || attempts[pass.AttemptID] || (i > 0 && pass.Verdict != passes[0].Payload.AssurancePassCompleted.Verdict) || !oneOf(pass.Verdict, "approve", "approve-with-advisories") {
				conflict = true
			}
			contexts[pass.TopLevelContextID], attempts[pass.AttemptID] = true, true
		}
		origin, ok := idx.demands[p.DemandID]
		if !ok {
			return reducerError(ReducerMissingParent, "process.demand_id->demand_origin", processID, "demand %q is absent", p.DemandID)
		}
		task, _, profile := demandFields(origin)
		if task != p.TaskID || profile.Assurance != p.PlannedAssurance {
			return reducerError(ReducerCrossBoundary, "review_process_completed->demand_origin", processID, "task or planned assurance differs")
		}
		floorViolation := p.CompletedPasses != p.RequiredPasses
		if len(p.PassVerdicts) > 0 {
			aggregate, err := implverdict.MostRestrictive(p.PassVerdicts)
			if err != nil || p.AggregateVerdict != aggregate {
				return reducerError(ReducerInvalidJoin, "review_process_completed.aggregate->pass_verdicts", processID, "aggregate verdict is not the deterministic impl-review aggregate")
			}
		}
		switch p.Result {
		case "satisfied":
			if floorViolation {
				return reducerError(ReducerPartialComposite, "process.satisfied->required_passes", processID, "satisfied terminal violates the required pass floor")
			}
			if conflict {
				return reducerError(ReducerInvalidJoin, "process.satisfied->accepting_pass_set", processID, "satisfied terminal contains conflicting, non-accepting, or non-blind passes")
			}
		case "floor_violation":
			if !floorViolation {
				return reducerError(ReducerInvalidJoin, "process.floor_violation->pass_floor", processID, "terminal names no floor violation")
			}
		case "conflict_nonaccepting":
			if floorViolation {
				return reducerError(ReducerInvalidJoin, "process.conflict_nonaccepting->pass_floor", processID, "floor violation must use its typed result")
			}
		}
	}
	return nil
}

func validateInvalidations(idx *reductionIndex) error {
	byGeneration := map[string]Event{}
	allEvents := make([]Event, 0, len(idx.events))
	for _, event := range idx.events {
		allEvents = append(allEvents, event)
	}
	for _, event := range idx.invalidations {
		p := event.Payload.RoutingEvidenceInvalidated
		generation := p.PolicyVersion + "\x00" + p.NativeProfileID
		if prior, exists := byGeneration[generation]; exists {
			return reducerError(ReducerConflict, "defective_generation->invalidation", generation, "events %s and %s conflict", prior.EventID, event.EventID)
		}
		byGeneration[generation] = event
		for _, id := range p.AffectedEventIDs {
			if _, ok := idx.events[EventID(id)]; !ok {
				return reducerError(ReducerMissingParent, "invalidation.event_ids->event", p.InvalidationID, "event %q is absent", id)
			}
		}
		for _, id := range p.AffectedAttemptIDs {
			if _, ok := idx.resolutions[id]; !ok {
				return reducerError(ReducerMissingParent, "invalidation.attempt_ids->attempt", p.InvalidationID, "attempt %q is absent", id)
			}
		}
		for _, id := range p.AffectedProcessIDs {
			if !hasProcessPass(idx, id) {
				if _, terminalOK := idx.processes[id]; !terminalOK {
					return reducerError(ReducerMissingParent, "invalidation.review_process_ids->process", p.InvalidationID, "process %q is absent", id)
				}
			}
		}
		want, err := DiscoverAffected(State{events: allEvents}, QualificationDefect{PolicyVersion: p.PolicyVersion, NativeProfileID: p.NativeProfileID})
		if err != nil || !equalInvalidationPayloadSet(p, want) {
			return reducerError(ReducerInvalidJoin, "invalidation.affected_set->discovery", p.InvalidationID, "affected identities do not equal exact discovery")
		}
	}
	return nil
}

func equalInvalidationPayloadSet(payload *RoutingEvidenceInvalidatedPayload, want AffectedSet) bool {
	eventIDs := make([]string, len(want.EventIDs))
	for i, id := range want.EventIDs {
		eventIDs[i] = string(id)
	}
	return reflect.DeepEqual(payload.AffectedEventIDs, eventIDs) && reflect.DeepEqual(payload.AffectedAttemptIDs, want.AttemptIDs) &&
		reflect.DeepEqual(payload.AffectedProcessIDs, want.ReviewProcessIDs) && reflect.DeepEqual(payload.AffectedArtifactRefs, want.ArtifactRefs)
}

func hasProcessPass(idx *reductionIndex, processID string) bool {
	for _, pass := range idx.passes {
		if pass.Payload.AssurancePassCompleted.ReviewProcessID == processID {
			return true
		}
	}
	return false
}

func buildViews(state *StructuralState, idx *reductionIndex) {
	for demandID, origin := range idx.demands {
		view := DemandView{DemandID: demandID, OriginEventID: origin.EventID, AttemptIDs: []string{}, InvalidationIDs: []string{}}
		for attemptID, terminal := range idx.resolutions {
			if resolutionDemandID(terminal) == demandID {
				view.AttemptIDs = append(view.AttemptIDs, attemptID)
			}
		}
		view.InvalidationIDs = invalidationsFor(idx.invalidations, func(p *RoutingEvidenceInvalidatedPayload) bool {
			return containsString(p.AffectedEventIDs, string(origin.EventID))
		})
		sort.Strings(view.AttemptIDs)
		state.Demands = append(state.Demands, view)
	}
	for attemptID, terminal := range idx.resolutions {
		view := AttemptView{AttemptID: attemptID, ResolutionEventID: terminal.EventID, TransferEventIDs: []EventID{}, PassIDs: []string{}, InvalidationIDs: []string{}}
		if terminal.Kind == RoutingResolutionFailed {
			p := terminal.Payload.RoutingResolutionFailed
			view.DemandID, view.TaskID, view.Role, view.State = p.DemandID, p.TaskID, p.Role, StateResolutionFailed
		} else {
			p := terminal.Payload.RoutingResolved
			view.DemandID, view.TaskID, view.Role, view.OwnerGeneration = p.DemandID, p.TaskID, p.Role, p.OwnerGeneration+len(idx.transfers[attemptID])
			view.State, view.LaunchEligible = StateResolvedPendingOutcome, true
			if outcome, ok := idx.outcomes[attemptID]; ok {
				view.State, view.LaunchEligible, view.OutcomeEventID = StateResolvedTerminal, false, outcome.EventID
				if child, ok := idx.taskOutcomes[attemptID]; ok {
					view.TaskOutcomeEventID = child.EventID
				}
				view.ProcessEligible = p.Role == "review" && outcome.Payload.RoutedAttemptCompleted.Outcome == "verified"
			}
		}
		for _, transfer := range idx.transfers[attemptID] {
			view.TransferEventIDs = append(view.TransferEventIDs, transfer.EventID)
		}
		for passID, pass := range idx.passes {
			if pass.Payload.AssurancePassCompleted.AttemptID == attemptID {
				view.PassIDs = append(view.PassIDs, passID)
			}
		}
		sort.Strings(view.PassIDs)
		view.InvalidationIDs = invalidationsFor(idx.invalidations, func(p *RoutingEvidenceInvalidatedPayload) bool {
			return invalidatesAttempt(p, attemptID, idx)
		})
		if len(view.InvalidationIDs) > 0 {
			view.State, view.LaunchEligible, view.ProcessEligible = StateInvalidated, false, false
		}
		state.Attempts = append(state.Attempts, view)
	}
	processIDs := map[string]bool{}
	for _, pass := range idx.passes {
		processIDs[pass.Payload.AssurancePassCompleted.ReviewProcessID] = true
	}
	for id := range idx.processes {
		processIDs[id] = true
	}
	for processID := range processIDs {
		view := ProcessView{ReviewProcessID: processID, State: StateProcessInFlight, Passes: []PassView{}, InvalidationIDs: []string{}, ProcessEligible: true}
		for passID, passEvent := range idx.passes {
			p := passEvent.Payload.AssurancePassCompleted
			if p.ReviewProcessID != processID {
				continue
			}
			view.ProcessGeneration = p.ProcessGeneration
			pv := PassView{PassID: passID, AttemptID: p.AttemptID, PassIndex: p.PassIndex, TerminalEventID: passEvent.EventID, FindingEventIDs: []EventID{}, DispositionEventIDs: []EventID{}}
			for _, finding := range p.FindingRecords {
				child := idx.findings[finding.FindingID]
				pv.FindingEventIDs = append(pv.FindingEventIDs, child.EventID)
				for _, disposition := range idx.dispositions[finding.FindingID] {
					pv.DispositionEventIDs = append(pv.DispositionEventIDs, disposition.EventID)
				}
			}
			slices.Sort(pv.FindingEventIDs)
			slices.Sort(pv.DispositionEventIDs)
			view.Passes = append(view.Passes, pv)
		}
		sort.Slice(view.Passes, func(i, j int) bool { return view.Passes[i].PassIndex < view.Passes[j].PassIndex })
		if terminal, ok := idx.processes[processID]; ok {
			view.State, view.ProcessEligible, view.ProjectionEligible, view.TerminalEventID = StateProcessTerminal, false, true, terminal.EventID
			view.ProcessGeneration = terminal.Payload.ReviewProcessCompleted.ProcessGeneration
			view.AggregateVerdict = terminal.Payload.ReviewProcessCompleted.AggregateVerdict
			view.Result = terminal.Payload.ReviewProcessCompleted.Result
			view.Satisfied = view.Result == "satisfied"
		}
		view.InvalidationIDs = invalidationsFor(idx.invalidations, func(p *RoutingEvidenceInvalidatedPayload) bool {
			return invalidatesProcess(p, processID, idx)
		})
		if len(view.InvalidationIDs) > 0 {
			view.State, view.ProcessEligible, view.ProjectionEligible, view.Satisfied = StateInvalidated, false, false, false
		}
		state.Processes = append(state.Processes, view)
	}
	for _, event := range idx.invalidations {
		p := event.Payload.RoutingEvidenceInvalidated
		state.Invalidations = append(state.Invalidations, InvalidationView{InvalidationID: p.InvalidationID, EventID: event.EventID, State: StateInvalidated, Disposition: p.Disposition})
		if brief := remediationBrief(event, *state); brief != nil {
			state.RemediationBriefs = append(state.RemediationBriefs, *brief)
		}
	}
	for _, event := range idx.activations {
		p := event.Payload.ActivationCheckpoint
		ids := invalidationsFor(idx.invalidations, func(ip *RoutingEvidenceInvalidatedPayload) bool {
			return containsString(ip.AffectedEventIDs, string(event.EventID))
		})
		state.Activations = append(state.Activations, ActivationView{ActivationPolicyVersion: p.ActivationPolicyVersion, ExactMainlineObject: p.ExactMainlineObject, ProofManifestDigest: p.ProofManifestDigest, EventID: event.EventID, InvalidationIDs: ids, ActivationEligible: p.ActivationReady && len(ids) == 0})
	}
	sort.Slice(state.Demands, func(i, j int) bool { return state.Demands[i].DemandID < state.Demands[j].DemandID })
	sort.Slice(state.Attempts, func(i, j int) bool { return state.Attempts[i].AttemptID < state.Attempts[j].AttemptID })
	sort.Slice(state.Processes, func(i, j int) bool { return state.Processes[i].ReviewProcessID < state.Processes[j].ReviewProcessID })
	sort.Slice(state.Invalidations, func(i, j int) bool {
		return state.Invalidations[i].InvalidationID < state.Invalidations[j].InvalidationID
	})
	sort.Slice(state.RemediationBriefs, func(i, j int) bool {
		return state.RemediationBriefs[i].InvalidationID < state.RemediationBriefs[j].InvalidationID
	})
	sort.Slice(state.Activations, func(i, j int) bool {
		return activationSubject(state.Activations[i]) < activationSubject(state.Activations[j])
	})
}

func resolutionDemandID(event Event) string {
	if event.Kind == RoutingResolved {
		return event.Payload.RoutingResolved.DemandID
	}
	return event.Payload.RoutingResolutionFailed.DemandID
}

func invalidationsFor(events []Event, match func(*RoutingEvidenceInvalidatedPayload) bool) []string {
	ids := []string{}
	for _, event := range events {
		p := event.Payload.RoutingEvidenceInvalidated
		if match(p) {
			ids = append(ids, p.InvalidationID)
		}
	}
	sort.Strings(ids)
	return ids
}

func invalidatesAttempt(invalidation *RoutingEvidenceInvalidatedPayload, attemptID string, idx *reductionIndex) bool {
	if containsString(invalidation.AffectedAttemptIDs, attemptID) {
		return true
	}
	affected := func(event Event) bool {
		return event.EventID != "" && containsString(invalidation.AffectedEventIDs, string(event.EventID))
	}
	resolution, ok := idx.resolutions[attemptID]
	if !ok {
		return false
	}
	if affected(resolution) || affected(idx.demands[resolutionDemandID(resolution)]) || affected(idx.outcomes[attemptID]) || affected(idx.taskOutcomes[attemptID]) {
		return true
	}
	return slices.ContainsFunc(idx.transfers[attemptID], affected)
}

func invalidatesProcess(invalidation *RoutingEvidenceInvalidatedPayload, processID string, idx *reductionIndex) bool {
	if containsString(invalidation.AffectedProcessIDs, processID) {
		return true
	}
	affected := func(event Event) bool {
		return event.EventID != "" && containsString(invalidation.AffectedEventIDs, string(event.EventID))
	}
	if affected(idx.processes[processID]) {
		return true
	}
	for _, pass := range idx.passes {
		p := pass.Payload.AssurancePassCompleted
		if p.ReviewProcessID != processID {
			continue
		}
		if invalidatesAttempt(invalidation, p.AttemptID, idx) || affected(pass) {
			return true
		}
		for _, finding := range p.FindingRecords {
			if affected(idx.findings[finding.FindingID]) {
				return true
			}
			if slices.ContainsFunc(idx.dispositions[finding.FindingID], affected) {
				return true
			}
		}
	}
	return false
}

func validateAtomicBoundary(prior, next []Event) error {
	_ = prior
	parents, taskChildren, passParents, findingChildren := map[string]bool{}, map[string]bool{}, map[string]Event{}, map[string]bool{}
	for _, event := range next {
		switch event.Kind {
		case RoutedAttemptCompleted:
			parents[event.Payload.RoutedAttemptCompleted.AttemptID] = true
		case RoutedTaskOutcome:
			taskChildren[event.Payload.RoutedTaskOutcome.AttemptID] = true
		case AssurancePassCompleted:
			passParents[event.Payload.AssurancePassCompleted.PassID] = event
		case FindingPersisted:
			findingChildren[event.Payload.FindingPersisted.FindingID] = true
		}
	}
	for attemptID := range taskChildren {
		if !parents[attemptID] {
			return reducerError(ReducerPartialComposite, "transition.routed_task_outcome->parent", attemptID, "child and parent must be appended atomically")
		}
	}
	for passID, event := range passParents {
		for _, finding := range event.Payload.AssurancePassCompleted.FindingRecords {
			if !findingChildren[finding.FindingID] {
				return reducerError(ReducerPartialComposite, "transition.pass_manifest->finding", passID, "finding %q is outside the composite", finding.FindingID)
			}
		}
	}
	for findingID := range findingChildren {
		found := false
		for _, event := range passParents {
			for _, finding := range event.Payload.AssurancePassCompleted.FindingRecords {
				if finding.FindingID == findingID {
					found = true
				}
			}
		}
		if !found {
			return reducerError(ReducerPartialComposite, "transition.finding->pass_manifest", findingID, "finding and pass must be appended atomically")
		}
	}
	return nil
}

// MarshalJSON deliberately excludes the retained transition events. They are
// reducer implementation state, not part of any typed subject view.
func (s StructuralState) MarshalJSON() ([]byte, error) {
	type wire struct {
		Repository        string             `json:"repository"`
		EvidenceRef       string             `json:"evidence_ref"`
		Demands           []DemandView       `json:"demands"`
		Attempts          []AttemptView      `json:"attempts"`
		Processes         []ProcessView      `json:"processes"`
		Invalidations     []InvalidationView `json:"invalidations"`
		RemediationBriefs []RemediationBrief `json:"remediation_briefs"`
		Activations       []ActivationView   `json:"activations"`
	}
	return json.Marshal(wire{s.Repository, s.EvidenceRef, s.Demands, s.Attempts, s.Processes, s.Invalidations, s.RemediationBriefs, s.Activations})
}
