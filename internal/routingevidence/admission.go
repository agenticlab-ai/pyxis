package routingevidence

import (
	"context"
	"errors"
	"fmt"
	"github.com/agenticlab-ai/pyxis/internal/entity"
	"reflect"
	"sort"
	"sync"

	"github.com/agenticlab-ai/pyxis/internal/executiondemand"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/nativerouting"
)

type BoundaryOperation string

const (
	OperationDemandExactRead BoundaryOperation = "demand_exact_read"
	OperationResolution      BoundaryOperation = "offline_resolution"
	OperationCanonicalAppend BoundaryOperation = "canonical_append"
	OperationProofExactRead  BoundaryOperation = "proof_exact_read"
	OperationClaim           BoundaryOperation = "implementation_claim"
	OperationReviewLaunch    BoundaryOperation = "review_launch"
)

type BoundaryObservation struct {
	Operation BoundaryOperation `json:"operation"`
	Network   bool              `json:"network"`
	Inference bool              `json:"inference"`
}

// BoundaryTrace is the caller-owned audit of the consequential interval. The
// admission service records its own operations; instrumented callers may add
// observations so an unpermitted outbound operation fails the final boundary.
type BoundaryTrace struct {
	mu           sync.Mutex
	observations []BoundaryObservation
}

func (t *BoundaryTrace) Observe(operation BoundaryOperation, network, inference bool) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.observations = append(t.observations, BoundaryObservation{Operation: operation, Network: network, Inference: inference})
}

func (t *BoundaryTrace) Observations() []BoundaryObservation {
	if t == nil {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]BoundaryObservation(nil), t.observations...)
}

func (t *BoundaryTrace) authorize(boundary BoundaryOperation) error {
	if t == nil {
		return errors.New("routing admission: boundary trace is absent")
	}
	t.Observe(boundary, false, false)
	observations := t.Observations()
	resolution := -1
	appendSeen, readSeen := false, false
	for i, observation := range observations {
		if observation.Inference {
			return fmt.Errorf("routing admission: inference observed at %s", observation.Operation)
		}
		if observation.Operation == OperationResolution {
			if observation.Network || resolution >= 0 {
				return errors.New("routing admission: resolution was not one offline operation")
			}
			resolution = i
		}
		if resolution >= 0 && i > resolution && i < len(observations)-1 && observation.Network {
			switch observation.Operation {
			case OperationCanonicalAppend:
				appendSeen = true
			case OperationProofExactRead:
				readSeen = true
			default:
				return fmt.Errorf("routing admission: network operation %s occurred inside the protected interval", observation.Operation)
			}
		}
	}
	if len(observations) == 0 || observations[len(observations)-1].Operation != boundary {
		return errors.New("routing admission: consequential boundary is not the final observation")
	}
	if resolution >= 0 && (!appendSeen || !readSeen) {
		return errors.New("routing admission: resolution was not followed by canonical append and exact-read")
	}
	if resolution < 0 && !containsOperation(observations, OperationProofExactRead) {
		return errors.New("routing admission: recovered permit lacks an exact-read")
	}
	return nil
}

func containsOperation(observations []BoundaryObservation, want BoundaryOperation) bool {
	for _, observation := range observations {
		if observation.Operation == want {
			return true
		}
	}
	return false
}

type BoundaryBinding struct {
	DemandID   string
	TaskID     string
	Harness    entity.Harness
	Role       nativerouting.Role
	ProposalID string
}

// BoundaryPermit is the narrow consumer port shared by the implementation
// claim and review-launch seams. AdmissionPermit is its only production
// implementation; focused boundary tests may provide a fail-closed double.
type BoundaryPermit interface {
	Authorize(BoundaryOperation, BoundaryBinding) error
}

type permitGate struct {
	mu       sync.Mutex
	consumed bool
}

// AdmissionPermit is opaque and one-use. Its bindings come only from an
// exact-read routing_resolved event and cannot be populated by callers.
type AdmissionPermit struct {
	proof            EvidenceProof
	attemptID        string
	demandID         string
	taskID           string
	harness          entity.Harness
	role             nativerouting.Role
	proposalID       string
	proposalBand     string
	requestedProfile Profile
	nativeProfileID  string
	nativeModel      string
	nativeEffort     string
	nativeControls   []nativerouting.NativeControl
	policyVersion    string
	ownerGeneration  int
	trace            *BoundaryTrace
	gate             *permitGate
}

func (p AdmissionPermit) Authorize(boundary BoundaryOperation, binding BoundaryBinding) error {
	if p.gate == nil || p.trace == nil {
		return errors.New("routing admission: permit is unverified")
	}
	wantBoundary := OperationClaim
	if p.role == nativerouting.RoleReview {
		wantBoundary = OperationReviewLaunch
	}
	if boundary != wantBoundary || binding.Role != p.role || binding.DemandID != p.demandID ||
		binding.TaskID != p.taskID || binding.Harness != p.harness || binding.ProposalID != p.proposalID {
		return errors.New("routing admission: permit does not bind the consequential boundary")
	}
	p.gate.mu.Lock()
	defer p.gate.mu.Unlock()
	if p.gate.consumed {
		return errors.New("routing admission: permit was already consumed")
	}
	if err := p.trace.authorize(boundary); err != nil {
		return err
	}
	p.gate.consumed = true
	return nil
}

func (p AdmissionPermit) EvidenceProof() EvidenceProof { return cloneEvidenceProof(p.proof) }
func (p AdmissionPermit) AttemptID() string            { return p.attemptID }
func (p AdmissionPermit) NativeProfileID() string      { return p.nativeProfileID }
func (p AdmissionPermit) PolicyVersion() string        { return p.policyVersion }
func (p AdmissionPermit) OwnerGeneration() int         { return p.ownerGeneration }

// NativeLaunch returns only the resolver-selected native identity. The Worker
// launches that identity and does not select another model.
func (p AdmissionPermit) NativeLaunch(role nativerouting.Role) (port.NativeLaunchSpec, error) {
	if p.role != role || p.nativeProfileID == "" || p.nativeModel == "" || p.nativeEffort == "" {
		return port.NativeLaunchSpec{}, errors.New("routing admission: permit has no complete native launch")
	}
	return port.NativeLaunchSpec{NativeProfileID: p.nativeProfileID, Role: p.role, NativeModel: p.nativeModel, NativeEffort: p.nativeEffort,
		Controls: append([]nativerouting.NativeControl(nil), p.nativeControls...)}, nil
}

// RuntimeOwnershipProof projects the already exact-read routing_resolved proof
// into the consumer runtime's activation contract. No tracker or proposal fact
// participates in this authority.
func (p AdmissionPermit) RuntimeOwnershipProof() (port.OwnershipProof, error) {
	if p.attemptID == "" || p.harness == "" || p.ownerGeneration != 1 || len(p.proof.Entries) != 1 {
		return port.OwnershipProof{}, errors.New("routing admission: permit has no runtime ownership proof")
	}
	entry := p.proof.Entries[0]
	if entry.EventID == "" || entry.ContentDigest == "" {
		return port.OwnershipProof{}, errors.New("routing admission: runtime ownership proof is incomplete")
	}
	return port.OwnershipProof{Kind: port.OwnershipProofResolution, Reference: string(entry.EventID), Digest: entry.ContentDigest,
		AttemptID: p.attemptID, Harness: p.harness, OwnerGeneration: port.OwnerGeneration(p.ownerGeneration)}, nil
}

type RouteIntent struct {
	AttemptID          string
	DemandID           executiondemand.DemandID
	TaskID             string
	Harness            entity.Harness
	Role               nativerouting.Role
	RequestedProfile   executiondemand.ProfileMinimum
	Catalog            nativerouting.ValidatedCatalog
	Mode               nativerouting.Mode
	ProposalID         string
	ProposalBand       string
	AssuranceJoinRaise bool
	ReviewAdmissionKey string
	Trace              *BoundaryTrace
}

// ResolutionTerminalWriter owns signing plus the canonical append. It is kept
// narrower than Store so admission cannot perform any unrelated network work.
type ResolutionTerminalWriter interface {
	WriteTerminal(context.Context, ObservedHead, Event) (EvidenceProof, error)
}

type SignedResolutionTerminalWriter struct {
	Store      Store
	Capability SigningCapability
}

func (w SignedResolutionTerminalWriter) WriteTerminal(ctx context.Context, head ObservedHead, event Event) (EvidenceProof, error) {
	if w.Store == nil {
		return EvidenceProof{}, errors.New("routing admission: terminal store is unavailable")
	}
	event.Producer = Producer{Class: string(w.Capability.ProducerClass), Generation: w.Capability.ImplementationGeneration}
	unsigned, err := NewUnsignedEnvelope(event)
	if err != nil {
		return EvidenceProof{}, fmt.Errorf("routing admission: construct terminal envelope: %w", err)
	}
	event.ProducerProof, err = Sign(unsigned, w.Capability)
	if err != nil {
		return EvidenceProof{}, fmt.Errorf("routing admission: sign terminal: %w", err)
	}
	return w.Store.Append(ctx, AppendRequest{ObservedHead: head, Event: &event})
}

var ErrAdmissionDenied = errors.New("routing admission denied")

type AdmissionVerifier struct {
	store  Store
	writer ResolutionTerminalWriter
	mu     sync.Mutex
	review map[string]string
}

func NewAdmissionVerifier(store Store, writer ResolutionTerminalWriter) (*AdmissionVerifier, error) {
	if store == nil || writer == nil {
		return nil, errors.New("routing admission: store and terminal writer are required")
	}
	return &AdmissionVerifier{store: store, writer: writer, review: make(map[string]string)}, nil
}

func (v *AdmissionVerifier) AdmitImplementation(ctx context.Context, intent RouteIntent) (AdmissionPermit, error) {
	intent.Role = nativerouting.RoleImplementation
	return v.admit(ctx, intent)
}

func (v *AdmissionVerifier) AdmitReview(ctx context.Context, intent RouteIntent) (AdmissionPermit, error) {
	intent.Role = nativerouting.RoleReview
	return v.admit(ctx, intent)
}

func (v *AdmissionVerifier) admit(ctx context.Context, intent RouteIntent) (AdmissionPermit, error) {
	trace := intent.Trace
	if trace == nil {
		trace = &BoundaryTrace{}
	}
	if intent.AttemptID == "" || intent.DemandID == "" || intent.TaskID == "" || intent.Harness == "" {
		return AdmissionPermit{}, fmt.Errorf("%w: route intent identity is incomplete", ErrAdmissionDenied)
	}
	if intent.Role == nativerouting.RoleReview && intent.ReviewAdmissionKey == "" {
		return AdmissionPermit{}, fmt.Errorf("%w: review admission key is required", ErrAdmissionDenied)
	}
	trace.Observe(OperationDemandExactRead, true, false)
	originRead, err := v.store.ReadSubject(ctx, SubjectID{Kind: SubjectDemand, ID: string(intent.DemandID)})
	if err != nil {
		return AdmissionPermit{}, fmt.Errorf("%w: exact-read demand origin: %v", ErrAdmissionDenied, err)
	}
	origin, policyVersion, err := exactDemand(originRead, intent)
	if err != nil {
		return AdmissionPermit{}, fmt.Errorf("%w: %v", ErrAdmissionDenied, err)
	}
	request := nativerouting.Request{SchemaVersion: nativerouting.RequestSchema, AttemptID: intent.AttemptID,
		DemandID: intent.DemandID, Harness: intent.Harness, Role: intent.Role,
		RequestedProfile: intent.RequestedProfile, Mode: intent.Mode}
	resolution := (nativerouting.Resolver{}).Resolve(request, intent.Catalog)
	trace.Observe(OperationResolution, false, false)
	terminal := terminalFor(intent, policyVersion, resolution)
	duplicate := false
	if terminal.Kind == RoutingResolved && intent.Role == nativerouting.RoleReview {
		v.mu.Lock()
		if _, duplicate = v.review[intent.ReviewAdmissionKey]; !duplicate {
			v.review[intent.ReviewAdmissionKey] = intent.AttemptID
		}
		v.mu.Unlock()
		if duplicate {
			terminal = failedTerminal(intent, policyVersion, "duplicate_admission", nil)
		}
	}
	terminal.Repository, terminal.EvidenceRef = origin.Repository, origin.EvidenceRef
	trace.Observe(OperationCanonicalAppend, true, false)
	proof, err := v.writer.WriteTerminal(ctx, originRead.ObservedHead, terminal)
	if err != nil {
		return AdmissionPermit{}, fmt.Errorf("%w: append resolution terminal: %v", ErrAdmissionDenied, err)
	}
	if terminal.Kind != RoutingResolved {
		return AdmissionPermit{}, fmt.Errorf("%w: terminal=%s", ErrAdmissionDenied, terminal.Kind)
	}
	permit, err := v.verifyPermit(ctx, proof, trace)
	if err != nil {
		return AdmissionPermit{}, err
	}
	if resolution.NativeProfile != nil {
		permit.nativeModel = resolution.NativeProfile.NativeModel
		permit.nativeEffort = resolution.NativeProfile.NativeEffort
		permit.nativeControls = append([]nativerouting.NativeControl(nil), resolution.NativeProfile.Controls...)
	}
	if permit.attemptID != intent.AttemptID || permit.demandID != string(intent.DemandID) || permit.taskID != intent.TaskID ||
		permit.harness != intent.Harness || permit.role != intent.Role || permit.proposalID != intent.ProposalID ||
		permit.proposalBand != intent.ProposalBand || permit.policyVersion != policyVersion ||
		permit.requestedProfile != evidenceProfile(intent.RequestedProfile) || permit.nativeProfileID != resolution.NativeProfile.NativeProfileID {
		return AdmissionPermit{}, fmt.Errorf("%w: exact proof differs from route intent", ErrAdmissionDenied)
	}
	return permit, nil
}

func (v *AdmissionVerifier) VerifyPermit(ctx context.Context, proof EvidenceProof) (AdmissionPermit, error) {
	trace := &BoundaryTrace{}
	return v.verifyPermit(ctx, proof, trace)
}

func (v *AdmissionVerifier) verifyPermit(ctx context.Context, proof EvidenceProof, trace *BoundaryTrace) (AdmissionPermit, error) {
	trace.Observe(OperationProofExactRead, true, false)
	exact, err := v.store.ReadProof(ctx, proof)
	if err != nil {
		return AdmissionPermit{}, fmt.Errorf("%w: exact-read resolution proof: %v", ErrAdmissionDenied, err)
	}
	if len(exact.Events) != 1 || len(proof.Entries) != 1 || exact.Events[0].Kind != RoutingResolved {
		return AdmissionPermit{}, fmt.Errorf("%w: proof is not one routing_resolved event", ErrAdmissionDenied)
	}
	event := exact.Events[0]
	p := event.Payload.RoutingResolved
	if p == nil || p.AttemptID == "" || p.DemandID == "" || p.TaskID == "" || p.Harness == "" || p.NativeProfileID == "" ||
		p.OwnerGeneration != 1 || (p.Role != string(nativerouting.RoleImplementation) && p.Role != string(nativerouting.RoleReview)) ||
		(p.Resolution != string(nativerouting.ResolutionExact) && p.Resolution != string(nativerouting.ResolutionPromoted)) {
		return AdmissionPermit{}, fmt.Errorf("%w: resolved proof has no payload", ErrAdmissionDenied)
	}
	trace.Observe(OperationProofExactRead, true, false)
	current, err := v.store.ReadSubject(ctx, SubjectID{Kind: SubjectAttempt, ID: p.AttemptID})
	if err != nil || current.Subject == nil || current.Subject.Attempt == nil || !current.Subject.Attempt.LaunchEligible ||
		current.Subject.Attempt.ResolutionEventID != event.EventID || current.Subject.Attempt.AttemptID != p.AttemptID {
		return AdmissionPermit{}, fmt.Errorf("%w: proof is stale, invalidated, or not reducer-eligible", ErrAdmissionDenied)
	}
	entry := proof.Entries[0]
	if entry.EventID != event.EventID || entry.ContentDigest != event.ContentDigest || proof.EvidenceRef != event.EvidenceRef {
		return AdmissionPermit{}, fmt.Errorf("%w: proof entry differs from exact event", ErrAdmissionDenied)
	}
	return AdmissionPermit{proof: cloneEvidenceProof(proof), attemptID: p.AttemptID, demandID: p.DemandID, taskID: p.TaskID, harness: p.Harness,
		role: nativerouting.Role(p.Role), proposalID: p.ProposalID, proposalBand: p.ProposalBand, requestedProfile: p.RequestedProfile,
		nativeProfileID: p.NativeProfileID, policyVersion: p.PolicyVersion, ownerGeneration: p.OwnerGeneration, trace: trace, gate: &permitGate{}}, nil
}

func exactDemand(read ExactRead, intent RouteIntent) (Event, string, error) {
	if read.Subject == nil || read.Subject.Demand == nil || read.Subject.Demand.DemandID != string(intent.DemandID) {
		return Event{}, "", errors.New("demand subject is absent or different")
	}
	for _, event := range read.Events {
		if event.EventID != read.Subject.Demand.OriginEventID {
			continue
		}
		var taskID, policy string
		var profile Profile
		switch event.Kind {
		case ExecutionDemandFrozen:
			p := event.Payload.ExecutionDemandFrozen
			taskID, policy, profile = p.TaskID, p.PolicyVersion, p.Profile

		default:
			return Event{}, "", errors.New("demand origin has an unsupported kind")
		}
		if taskID != intent.TaskID || !reflect.DeepEqual(profile, evidenceProfile(intent.RequestedProfile)) {
			return Event{}, "", errors.New("route intent differs from exact demand origin")
		}
		return event, policy, nil
	}
	return Event{}, "", errors.New("exact demand origin event is absent")
}

func terminalFor(intent RouteIntent, policy string, resolution nativerouting.Resolution) Event {
	if resolution.Class != nativerouting.ResolutionExact && resolution.Class != nativerouting.ResolutionPromoted || resolution.NativeProfile == nil ||
		resolution.AttemptID != intent.AttemptID || resolution.DemandID != intent.DemandID || resolution.Harness != intent.Harness ||
		resolution.Role != intent.Role || !reflect.DeepEqual(resolution.RequestedProfile, intent.RequestedProfile) {
		reason := "integrity_error"
		switch resolution.Class {
		case nativerouting.ResolutionUnsupported:
			reason = "unsupported"
		case nativerouting.ResolutionPolicyInvalid:
			reason = "policy_invalid"
		}
		return failedTerminal(intent, policy, reason, unmetComponents(resolution))
	}
	promoted := make([]string, len(resolution.PromotedComponents))
	for i, component := range resolution.PromotedComponents {
		promoted[i] = string(component)
	}
	sort.Strings(promoted)
	return Event{Schema: Schema, Version: EventVersion, Kind: RoutingResolved, Payload: Payload{RoutingResolved: &RoutingResolvedPayload{
		AssuranceJoinRaise: intent.AssuranceJoinRaise, AttemptID: intent.AttemptID, DemandID: string(intent.DemandID), Harness: intent.Harness,
		NativeProfileID: resolution.NativeProfile.NativeProfileID, OwnerGeneration: 1, PolicyVersion: policy,
		PromotedComponents: promoted, ProposalBand: intent.ProposalBand, ProposalID: intent.ProposalID,
		RequestedProfile: evidenceProfile(intent.RequestedProfile), Resolution: string(resolution.Class), Role: string(intent.Role), TaskID: intent.TaskID,
	}}}
}

func failedTerminal(intent RouteIntent, policy, reason string, unmet []string) Event {
	if unmet == nil {
		unmet = []string{}
	}
	return Event{Schema: Schema, Version: EventVersion, Kind: RoutingResolutionFailed, Payload: Payload{RoutingResolutionFailed: &RoutingResolutionFailedPayload{
		AssuranceJoinRaise: intent.AssuranceJoinRaise, AttemptID: intent.AttemptID, DemandID: string(intent.DemandID), Harness: intent.Harness,
		PolicyVersion: policy, ProposalBand: intent.ProposalBand, ProposalID: intent.ProposalID, Reason: reason,
		RequestedProfile: evidenceProfile(intent.RequestedProfile), Role: string(intent.Role), TaskID: intent.TaskID, UnmetComponents: unmet,
	}}}
}

func unmetComponents(resolution nativerouting.Resolution) []string {
	out := make([]string, len(resolution.UnmetComponents))
	for i, unmet := range resolution.UnmetComponents {
		out[i] = string(unmet.Component)
	}
	sort.Strings(out)
	return out
}

func evidenceProfile(profile executiondemand.ProfileMinimum) Profile {
	return Profile{Capability: string(profile.Capability), Deliberation: string(profile.Deliberation), Context: string(profile.Context), Assurance: string(profile.Assurance)}
}

func cloneEvidenceProof(proof EvidenceProof) EvidenceProof {
	proof.Entries = append([]EvidenceEntry(nil), proof.Entries...)
	return proof
}
