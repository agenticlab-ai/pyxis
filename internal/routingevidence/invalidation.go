package routingevidence

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"
)

type InvalidationDisposition string

const (
	DispositionRerunRequired       InvalidationDisposition = "rerun_required"
	DispositionOperatorRemediation InvalidationDisposition = "operator_remediation"
)

// QualificationDefect exact-identifies one defective routing generation. The
// authorization and disposition are decisions owned outside the evidence
// reducer; discovery owns only the blast radius of that exact generation.
type QualificationDefect struct {
	PolicyVersion   string `json:"policy_version"`
	NativeProfileID string `json:"native_profile_id"`
}

// AffectedSet is the deterministic transitive closure of routes selected by a
// defective policy/profile generation. PassIDs are a readable projection of
// affected pass events; the canonical invalidation identity binds them through
// EventIDs, whose referenced payloads carry the pass identities.
type AffectedSet struct {
	PolicyVersion    string    `json:"policy_version"`
	NativeProfileID  string    `json:"native_profile_id"`
	EventIDs         []EventID `json:"event_ids"`
	AttemptIDs       []string  `json:"attempt_ids"`
	ReviewProcessIDs []string  `json:"review_process_ids"`
	PassIDs          []string  `json:"pass_ids"`
	ArtifactRefs     []string  `json:"artifact_refs"`
}

// Invalidation is the complete caller input. It contains no timestamp or
// mutable status; identical current evidence and authorization reconstruct the
// same canonical event bytes.
type Invalidation struct {
	Affected         AffectedSet             `json:"affected"`
	Disposition      InvalidationDisposition `json:"disposition"`
	AuthorizationRef string                  `json:"authorization_ref"`
}

// RemediationBrief is derived from canonical evidence only. It asks the
// operator for a decision but deliberately has no field capable of mutating or
// reverting an accepted artifact.
type RemediationBrief struct {
	InvalidationID   string    `json:"invalidation_id"`
	PolicyVersion    string    `json:"policy_version"`
	NativeProfileID  string    `json:"native_profile_id"`
	EventIDs         []EventID `json:"event_ids"`
	AttemptIDs       []string  `json:"attempt_ids"`
	ReviewProcessIDs []string  `json:"review_process_ids"`
	PassIDs          []string  `json:"pass_ids"`
	ArtifactRefs     []string  `json:"artifact_refs"`
	AuthorizationRef string    `json:"authorization_ref"`
	DecisionRequired string    `json:"decision_required"`
}

// DiscoverAffected computes every and only event chain whose resolved route
// used the exact defective policy/profile generation. An affected review pass
// invalidates its process terminal, but sibling passes remain unaffected.
func DiscoverAffected(state State, defect QualificationDefect) (AffectedSet, error) {
	if defect.PolicyVersion == "" || defect.NativeProfileID == "" {
		return AffectedSet{}, errors.New("routing invalidation: policy version and native profile are required")
	}
	set := AffectedSet{PolicyVersion: defect.PolicyVersion, NativeProfileID: defect.NativeProfileID,
		EventIDs: []EventID{}, AttemptIDs: []string{}, ReviewProcessIDs: []string{}, PassIDs: []string{}, ArtifactRefs: []string{}}
	affectedAttempts := map[string]bool{}
	eventIDs, attempts, processes, passes, artifacts := map[EventID]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, event := range state.events {
		if event.Kind != RoutingResolved {
			continue
		}
		p := event.Payload.RoutingResolved
		if p.PolicyVersion == defect.PolicyVersion && p.NativeProfileID == defect.NativeProfileID {
			affectedAttempts[p.AttemptID], attempts[p.AttemptID], eventIDs[event.EventID] = true, true, true
			if p.ProposalID != "" {
				artifacts[p.ProposalID] = true
			}
		}
	}
	if len(affectedAttempts) == 0 {
		return AffectedSet{}, errors.New("routing invalidation: defective generation has no reachable route")
	}
	for _, event := range state.events {
		attemptID := eventAttemptID(event)
		if attemptID != "" && affectedAttempts[attemptID] {
			eventIDs[event.EventID] = true
		}
		if event.Kind != AssurancePassCompleted || !affectedAttempts[event.Payload.AssurancePassCompleted.AttemptID] {
			continue
		}
		p := event.Payload.AssurancePassCompleted
		processes[p.ReviewProcessID], passes[p.PassID] = true, true
		artifacts[p.ProposalID], artifacts[p.ReviewedRevision] = true, true
		for _, finding := range p.FindingRecords {
			for _, candidate := range state.events {
				switch candidate.Kind {
				case FindingPersisted:
					if candidate.Payload.FindingPersisted.FindingID == finding.FindingID {
						eventIDs[candidate.EventID] = true
					}
				case DispositionRecorded:
					if candidate.Payload.DispositionRecorded.FindingID == finding.FindingID {
						eventIDs[candidate.EventID] = true
					}
				}
			}
		}
	}
	for _, event := range state.events {
		if event.Kind == ReviewProcessCompleted && processes[event.Payload.ReviewProcessCompleted.ReviewProcessID] {
			eventIDs[event.EventID] = true
		}
	}
	for _, event := range state.events {
		if event.Kind != ActivationCheckpointEvent {
			continue
		}
		for _, proofID := range event.Payload.ActivationCheckpoint.ProofIDs {
			if eventIDs[EventID(proofID)] {
				eventIDs[event.EventID] = true
				artifacts[event.Payload.ActivationCheckpoint.ExactMainlineObject] = true
				break
			}
		}
	}
	set.EventIDs = sortedEventIDSet(eventIDs)
	set.AttemptIDs = sortedStringSet(attempts)
	set.ReviewProcessIDs = sortedStringSet(processes)
	set.PassIDs = sortedStringSet(passes)
	set.ArtifactRefs = sortedStringSet(artifacts)
	return set, nil
}

func eventAttemptID(event Event) string {
	switch event.Kind {
	case RoutingResolved:
		return event.Payload.RoutingResolved.AttemptID
	case RoutedAttemptCompleted:
		return event.Payload.RoutedAttemptCompleted.AttemptID
	case RoutedTaskOutcome:
		return event.Payload.RoutedTaskOutcome.AttemptID
	case AttemptOwnerTransferred:
		return event.Payload.AttemptOwnerTransferred.AttemptID
	case AssurancePassCompleted:
		return event.Payload.AssurancePassCompleted.AttemptID
	default:
		return ""
	}
}

func sortedEventIDSet(set map[EventID]bool) []EventID {
	out := make([]EventID, 0, len(set))
	for value := range set {
		if value != "" {
			out = append(out, value)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func sortedStringSet(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for value := range set {
		if value != "" {
			out = append(out, value)
		}
	}
	sort.Strings(out)
	return out
}

// InvalidationAuthorizer independently validates that the reference is a
// current qualification-defect or operator-remediation authorization. Event
// signing and producer-policy verification remain a separate mandatory gate.
type InvalidationAuthorizer interface {
	AuthorizeInvalidation(context.Context, Invalidation) error
}

type InvalidationWriter interface {
	WriteInvalidation(context.Context, ObservedHead, Event) (EvidenceProof, error)
}

type SignedInvalidationWriter struct {
	Store      Store
	Capability SigningCapability
}

func (w SignedInvalidationWriter) WriteInvalidation(ctx context.Context, head ObservedHead, event Event) (EvidenceProof, error) {
	if w.Store == nil {
		return EvidenceProof{}, errors.New("routing invalidation: store is unavailable")
	}
	event.Producer = Producer{Class: string(w.Capability.ProducerClass), Generation: w.Capability.ImplementationGeneration}
	unsigned, err := NewUnsignedEnvelope(event)
	if err != nil {
		return EvidenceProof{}, fmt.Errorf("routing invalidation: construct envelope: %w", err)
	}
	event.ProducerProof, err = Sign(unsigned, w.Capability)
	if err != nil {
		return EvidenceProof{}, fmt.Errorf("routing invalidation: sign event: %w", err)
	}
	return w.Store.Append(ctx, AppendRequest{ObservedHead: head, Event: &event})
}

type InvalidationResult struct {
	Proof EvidenceProof     `json:"proof"`
	Brief *RemediationBrief `json:"brief,omitempty"`
}

type InvalidationService struct {
	store      Store
	writer     InvalidationWriter
	authorizer InvalidationAuthorizer
}

func NewInvalidationService(store Store, writer InvalidationWriter, authorizer InvalidationAuthorizer) (*InvalidationService, error) {
	if store == nil || writer == nil || authorizer == nil {
		return nil, errors.New("routing invalidation: store, writer, and authorizer are required")
	}
	return &InvalidationService{store: store, writer: writer, authorizer: authorizer}, nil
}

var ErrInvalidationRejected = errors.New("routing evidence invalidation rejected")

// Invalidate re-discovers the blast radius from a fresh exact read. Thus a
// stale, narrowed, or widened caller set never reaches authorization or append.
// An identical retry returns the existing proof; any second disposition for
// the same defective generation is an integrity conflict.
func (s *InvalidationService) Invalidate(ctx context.Context, invalidation Invalidation) (InvalidationResult, error) {
	if s == nil || len(invalidation.Affected.AttemptIDs) == 0 {
		return InvalidationResult{}, invalidationReject("affected set has no attempt")
	}
	if invalidation.AuthorizationRef == "" || !oneOf(string(invalidation.Disposition), string(DispositionRerunRequired), string(DispositionOperatorRemediation)) {
		return InvalidationResult{}, invalidationReject("authorization or disposition is invalid")
	}
	read, err := s.store.ReadSubject(ctx, SubjectID{Kind: SubjectAttempt, ID: invalidation.Affected.AttemptIDs[0]})
	if err != nil {
		return InvalidationResult{}, invalidationReject("exact-read affected chain: %v", err)
	}
	state, err := reduceCanonical(read.Events)
	if err != nil {
		return InvalidationResult{}, invalidationReject("reduce exact evidence: %v", err)
	}
	want, err := DiscoverAffected(state, QualificationDefect{PolicyVersion: invalidation.Affected.PolicyVersion, NativeProfileID: invalidation.Affected.NativeProfileID})
	if err != nil || !reflect.DeepEqual(want, invalidation.Affected) {
		return InvalidationResult{}, invalidationReject("affected set differs from exact discovery")
	}
	payload := invalidationPayload(invalidation)
	base, err := eventBase(read.Events)
	if err != nil {
		return InvalidationResult{}, invalidationReject("event boundary: %v", err)
	}
	event := base
	event.Kind, event.Payload = RoutingEvidenceInvalidated, Payload{RoutingEvidenceInvalidated: &payload}
	normalized, err := normalizedPayload(event)
	if err != nil {
		return InvalidationResult{}, invalidationReject("payload: %v", err)
	}
	event.Payload.RoutingEvidenceInvalidated = normalized.(*RoutingEvidenceInvalidatedPayload)
	for _, prior := range read.Events {
		if prior.Kind != RoutingEvidenceInvalidated {
			continue
		}
		p := prior.Payload.RoutingEvidenceInvalidated
		if p.PolicyVersion != payload.PolicyVersion || p.NativeProfileID != payload.NativeProfileID {
			continue
		}
		if !reflect.DeepEqual(p, event.Payload.RoutingEvidenceInvalidated) {
			return InvalidationResult{}, invalidationReject("defective generation already has a different invalidation")
		}
		proof, proofErr := proofForInvalidation(read, prior)
		if proofErr != nil {
			return InvalidationResult{}, invalidationReject("existing proof: %v", proofErr)
		}
		return InvalidationResult{Proof: proof, Brief: remediationBrief(prior, state)}, nil
	}
	if err := s.authorizer.AuthorizeInvalidation(ctx, invalidation); err != nil {
		return InvalidationResult{}, invalidationReject("authorization: %v", err)
	}
	proof, err := s.writer.WriteInvalidation(ctx, read.ObservedHead, event)
	if err != nil {
		return InvalidationResult{}, invalidationReject("append: %v", err)
	}
	exact, err := s.store.ReadProof(ctx, proof)
	if err != nil {
		return InvalidationResult{}, invalidationReject("exact-read appended invalidation: %v", err)
	}
	if len(exact.Events) != 1 || exact.Events[0].Kind != RoutingEvidenceInvalidated || !reflect.DeepEqual(exact.Events[0].Payload.RoutingEvidenceInvalidated, event.Payload.RoutingEvidenceInvalidated) {
		return InvalidationResult{}, invalidationReject("exact-read appended invalidation failed")
	}
	return InvalidationResult{Proof: proof, Brief: remediationBrief(exact.Events[0], state)}, nil
}

func invalidationPayload(in Invalidation) RoutingEvidenceInvalidatedPayload {
	eventIDs := make([]string, len(in.Affected.EventIDs))
	for i, id := range in.Affected.EventIDs {
		eventIDs[i] = string(id)
	}
	return RoutingEvidenceInvalidatedPayload{AffectedArtifactRefs: append([]string(nil), in.Affected.ArtifactRefs...),
		AffectedAttemptIDs: append([]string(nil), in.Affected.AttemptIDs...), AffectedEventIDs: eventIDs,
		AffectedProcessIDs: append([]string(nil), in.Affected.ReviewProcessIDs...), AuthorizationRef: in.AuthorizationRef,
		Disposition: string(in.Disposition), NativeProfileID: in.Affected.NativeProfileID, PolicyVersion: in.Affected.PolicyVersion}
}

func proofForInvalidation(read ExactRead, event Event) (EvidenceProof, error) {
	for _, entry := range read.Proof.Entries {
		if entry.EventID == event.EventID {
			return EvidenceProof{EvidenceRef: read.ObservedHead.EvidenceRef, Commit: read.ObservedHead.Commit, Entries: []EvidenceEntry{entry}}, nil
		}
	}
	path, err := EventPath(event.EventID)
	if err != nil {
		return EvidenceProof{}, err
	}
	return EvidenceProof{EvidenceRef: read.ObservedHead.EvidenceRef, Commit: read.ObservedHead.Commit,
		Entries: []EvidenceEntry{{Path: path, EventID: event.EventID, ContentDigest: event.ContentDigest}}}, nil
}

func remediationBrief(event Event, state State) *RemediationBrief {
	p := event.Payload.RoutingEvidenceInvalidated
	if p == nil || p.Disposition != string(DispositionOperatorRemediation) {
		return nil
	}
	passIDs := []string{}
	for _, id := range p.AffectedEventIDs {
		for _, candidate := range state.events {
			if string(candidate.EventID) == id && candidate.Kind == AssurancePassCompleted {
				passIDs = append(passIDs, candidate.Payload.AssurancePassCompleted.PassID)
			}
		}
	}
	sort.Strings(passIDs)
	eventIDs := make([]EventID, len(p.AffectedEventIDs))
	for i, id := range p.AffectedEventIDs {
		eventIDs[i] = EventID(id)
	}
	return &RemediationBrief{InvalidationID: p.InvalidationID, PolicyVersion: p.PolicyVersion, NativeProfileID: p.NativeProfileID,
		EventIDs: eventIDs, AttemptIDs: append([]string(nil), p.AffectedAttemptIDs...), ReviewProcessIDs: append([]string(nil), p.AffectedProcessIDs...),
		PassIDs: passIDs, ArtifactRefs: append([]string(nil), p.AffectedArtifactRefs...), AuthorizationRef: p.AuthorizationRef,
		DecisionRequired: "decide incident and remediation; accepted artifacts remain unchanged"}
}

func invalidationReject(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidationRejected, fmt.Sprintf(format, args...))
}

// Acceptance is the closed consequence surface shared by claim, review,
// projection, and activation callers.
type Acceptance struct {
	ClaimEligible       bool                    `json:"claim_eligible"`
	ReviewEligible      bool                    `json:"review_eligible"`
	ReviewSatisfied     bool                    `json:"review_satisfied"`
	ProjectionEligible  bool                    `json:"projection_eligible"`
	ActivationEligible  bool                    `json:"activation_eligible"`
	ReplacementRequired bool                    `json:"replacement_required"`
	RemediationRequired bool                    `json:"remediation_required"`
	Disposition         InvalidationDisposition `json:"disposition,omitempty"`
}

// CurrentAcceptance never infers authority from history alone: it returns only
// the reducer's current consequence bits and clears all of them on invalidation.
func CurrentAcceptance(view SubjectView) Acceptance {
	out := Acceptance{}
	invalidated := false
	if view.Attempt != nil {
		out.ClaimEligible, out.ReviewEligible = view.Attempt.LaunchEligible, view.Attempt.ProcessEligible
		invalidated = view.Attempt.State == StateInvalidated
	}
	if view.Process != nil {
		out.ReviewSatisfied, out.ProjectionEligible = view.Process.Satisfied, view.Process.ProjectionEligible
		invalidated = view.Process.State == StateInvalidated
	}
	if view.Activation != nil {
		out.ActivationEligible = view.Activation.ActivationEligible
		invalidated = len(view.Activation.InvalidationIDs) > 0
	}
	sawRerun, sawRemediation := false, false
	for _, invalidation := range view.Invalidations {
		disposition := InvalidationDisposition(invalidation.Disposition)
		if out.Disposition == "" {
			out.Disposition = disposition
		} else if out.Disposition != disposition {
			out.Disposition = ""
		}
		switch disposition {
		case DispositionRerunRequired:
			sawRerun = true
		case DispositionOperatorRemediation:
			sawRemediation = true
		}
	}
	if len(view.Invalidations) > 0 {
		out.ReplacementRequired, out.RemediationRequired = sawRerun, sawRemediation
	} else {
		out.ReplacementRequired = invalidated
	}
	if out.ReplacementRequired || out.RemediationRequired {
		out.ClaimEligible, out.ReviewEligible, out.ReviewSatisfied, out.ProjectionEligible, out.ActivationEligible = false, false, false, false, false
	}
	return out
}
