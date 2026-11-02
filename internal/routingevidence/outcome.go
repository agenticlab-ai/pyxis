package routingevidence

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

type OutcomeClass string

const (
	OutcomeVerified OutcomeClass = "verified"
	OutcomeFailed   OutcomeClass = "failed"
	OutcomeBlocked  OutcomeClass = "blocked"
)

type OutcomeCause string

const (
	CauseTerminalVerified       OutcomeCause = "terminal_verified"
	CauseWorkerFailed           OutcomeCause = "worker_failed"
	CauseLaunchFidelityMismatch OutcomeCause = "launch_fidelity_mismatch"
	CausePreflightFailed        OutcomeCause = "preflight_failed"
	CauseClaimLost              OutcomeCause = "claim_lost"
	CauseCancelled              OutcomeCause = "cancelled"
	CauseCrashReaped            OutcomeCause = "crash_reaped"
)

type LaunchFidelity string

const (
	FidelityExact       LaunchFidelity = "exact"
	FidelityMismatch    LaunchFidelity = "mismatch"
	FidelityUnavailable LaunchFidelity = "unavailable"
)

type UsageProvenance string

const (
	UsageRuntimeObserved    UsageProvenance = "runtime_observed"
	UsageRuntimeUnavailable UsageProvenance = "runtime_unavailable"
	UsageNotLaunched        UsageProvenance = "not_launched"
)

// AttemptTerminal is the bounded caller-owned part of an attempt terminal.
// Demand, task, proposal, role, and resolution references are always recovered
// from the exact canonical attempt; callers cannot restate those joins.
type AttemptTerminal struct {
	AttemptID           string
	Outcome             OutcomeClass
	Cause               OutcomeCause
	OwnerGeneration     int
	LaunchCapability    port.LaunchCapability
	LaunchCapabilityRef string
	LaunchFidelity      LaunchFidelity
	Usage               Usage
	UsageProvenance     UsageProvenance
	Turns               int
}

// OutcomeComposite is the only shape an outcome writer accepts. Review is one
// parent event; implementation is a parent and its exact task projection.
type OutcomeComposite struct {
	Attempt     Event
	TaskOutcome *Event
}

func (c OutcomeComposite) events() ([]Event, error) {
	if c.Attempt.Kind != RoutedAttemptCompleted || c.Attempt.Payload.RoutedAttemptCompleted == nil {
		return nil, errors.New("routing outcome: composite parent is not routed_attempt_completed")
	}
	if c.TaskOutcome == nil {
		return []Event{c.Attempt}, nil
	}
	if c.TaskOutcome.Kind != RoutedTaskOutcome || c.TaskOutcome.Payload.RoutedTaskOutcome == nil {
		return nil, errors.New("routing outcome: composite child is not routed_task_outcome")
	}
	return []Event{c.Attempt, *c.TaskOutcome}, nil
}

// OutcomeWriter signs and atomically appends one already-joined outcome.
type OutcomeWriter interface {
	WriteOutcome(context.Context, ObservedHead, OutcomeComposite) (EvidenceProof, error)
}

type SignedOutcomeWriter struct {
	Store      Store
	Capability SigningCapability
}

func (w SignedOutcomeWriter) WriteOutcome(ctx context.Context, head ObservedHead, composite OutcomeComposite) (EvidenceProof, error) {
	if w.Store == nil {
		return EvidenceProof{}, errors.New("routing outcome: store is unavailable")
	}
	events, err := composite.events()
	if err != nil {
		return EvidenceProof{}, err
	}
	for i := range events {
		events[i].Producer = Producer{Class: string(w.Capability.ProducerClass), Generation: w.Capability.ImplementationGeneration}
		unsigned, signErr := NewUnsignedEnvelope(events[i])
		if signErr != nil {
			return EvidenceProof{}, fmt.Errorf("routing outcome: construct %s: %w", events[i].Kind, signErr)
		}
		events[i].ProducerProof, signErr = Sign(unsigned, w.Capability)
		if signErr != nil {
			return EvidenceProof{}, fmt.Errorf("routing outcome: sign %s: %w", events[i].Kind, signErr)
		}
	}
	if len(events) == 1 {
		return w.Store.Append(ctx, AppendRequest{ObservedHead: head, Event: &events[0]})
	}
	return w.Store.Append(ctx, AppendRequest{ObservedHead: head, Composite: &AtomicComposite{Events: events}})
}

// OutcomeAuthorizer is the fencing half of terminal creation. Production owner
// implementations call FencedRuntime.AuthorizeTerminal; a reaper supplies its
// separately authorized implementation and may admit only crash_reaped.
type OutcomeAuthorizer interface {
	AuthorizeOutcome(context.Context, AttemptTerminal, AttemptView) error
}

type FencedOutcomeAuthorizer struct {
	Runtime interface {
		AuthorizeTerminal(port.LaunchCapability) error
	}
}

func (a FencedOutcomeAuthorizer) AuthorizeOutcome(_ context.Context, terminal AttemptTerminal, attempt AttemptView) error {
	if a.Runtime == nil {
		return errors.New("routing outcome: fenced runtime is unavailable")
	}
	capability := terminal.LaunchCapability
	if terminal.OwnerGeneration != attempt.OwnerGeneration || capability.AttemptID != terminal.AttemptID || int(capability.OwnerGeneration) != attempt.OwnerGeneration ||
		capability.CapabilityRef == "" || terminal.LaunchCapabilityRef != capability.CapabilityRef {
		return errors.New("routing outcome: launch capability differs from current attempt generation")
	}
	return a.Runtime.AuthorizeTerminal(capability)
}

type OutcomeService struct {
	store      Store
	writer     OutcomeWriter
	authorizer OutcomeAuthorizer
}

func NewOutcomeService(store Store, writer OutcomeWriter, authorizer OutcomeAuthorizer) (*OutcomeService, error) {
	if store == nil || writer == nil || authorizer == nil {
		return nil, errors.New("routing outcome: store, writer, and authorizer are required")
	}
	return &OutcomeService{store: store, writer: writer, authorizer: authorizer}, nil
}

var ErrOutcomeRejected = errors.New("routing outcome rejected")

// CompleteAttempt closes only the current canonically resolved generation. A
// byte-identical terminal retry returns the existing exact proof; any differing
// repeat is rejected before a writer can append another fact.
func (s *OutcomeService) CompleteAttempt(ctx context.Context, terminal AttemptTerminal) (EvidenceProof, error) {
	if s == nil || s.store == nil || terminal.AttemptID == "" {
		return EvidenceProof{}, fmt.Errorf("%w: completion service or attempt identity is absent", ErrOutcomeRejected)
	}
	if terminal.LaunchCapabilityRef == "" {
		terminal.LaunchCapabilityRef = terminal.LaunchCapability.CapabilityRef
	}
	read, err := s.store.ReadSubject(ctx, SubjectID{Kind: SubjectAttempt, ID: terminal.AttemptID})
	if err != nil {
		return EvidenceProof{}, fmt.Errorf("%w: exact-read attempt: %v", ErrOutcomeRejected, err)
	}
	if read.Subject == nil || read.Subject.Attempt == nil || read.Subject.Attempt.AttemptID != terminal.AttemptID {
		return EvidenceProof{}, fmt.Errorf("%w: canonical attempt is absent", ErrOutcomeRejected)
	}
	attempt := *read.Subject.Attempt
	resolution, ok := resolvedEvent(read.Events, terminal.AttemptID)
	if !ok {
		return EvidenceProof{}, fmt.Errorf("%w: attempt has no routing_resolved parent", ErrOutcomeRejected)
	}
	if terminal.OwnerGeneration != attempt.OwnerGeneration {
		return EvidenceProof{}, fmt.Errorf("%w: owner generation %d is not current generation %d", ErrOutcomeRejected, terminal.OwnerGeneration, attempt.OwnerGeneration)
	}
	composite, err := outcomeFor(resolution, terminal)
	if err != nil {
		return EvidenceProof{}, fmt.Errorf("%w: %v", ErrOutcomeRejected, err)
	}
	if attempt.OutcomeEventID != "" {
		proof, matchErr := existingOutcomeProof(read, composite)
		if matchErr != nil {
			return EvidenceProof{}, fmt.Errorf("%w: conflicting terminal retry: %v", ErrOutcomeRejected, matchErr)
		}
		return proof, nil
	}
	if attempt.State != StateResolvedPendingOutcome {
		return EvidenceProof{}, fmt.Errorf("%w: attempt state %s cannot accept an outcome", ErrOutcomeRejected, attempt.State)
	}
	if err := s.authorizer.AuthorizeOutcome(ctx, terminal, attempt); err != nil {
		return EvidenceProof{}, fmt.Errorf("%w: terminal authority: %v", ErrOutcomeRejected, err)
	}
	proof, err := s.writer.WriteOutcome(ctx, read.ObservedHead, composite)
	if err != nil {
		return EvidenceProof{}, fmt.Errorf("%w: append outcome: %v", ErrOutcomeRejected, err)
	}
	if err := s.verifyWritten(ctx, proof, composite); err != nil {
		return EvidenceProof{}, fmt.Errorf("%w: exact-read appended outcome: %v", ErrOutcomeRejected, err)
	}
	return proof, nil
}

func resolvedEvent(events []Event, attemptID string) (Event, bool) {
	for _, event := range events {
		if event.Kind == RoutingResolved && event.Payload.RoutingResolved != nil && event.Payload.RoutingResolved.AttemptID == attemptID {
			return event, true
		}
	}
	return Event{}, false
}

func outcomeFor(resolution Event, terminal AttemptTerminal) (OutcomeComposite, error) {
	p := resolution.Payload.RoutingResolved
	if p == nil || p.AttemptID != terminal.AttemptID {
		return OutcomeComposite{}, errors.New("resolution binding differs")
	}
	if terminal.OwnerGeneration < 1 || terminal.LaunchCapabilityRef == "" {
		return OutcomeComposite{}, errors.New("owner generation and launch capability reference are required")
	}
	parentPayload := &RoutedAttemptCompletedPayload{AttemptID: p.AttemptID, Cause: string(terminal.Cause), DemandID: p.DemandID,
		EvidenceRef: string(resolution.EventID), LaunchCapabilityRef: terminal.LaunchCapabilityRef, LaunchFidelity: string(terminal.LaunchFidelity),
		Outcome: string(terminal.Outcome), OwnerGeneration: terminal.OwnerGeneration, ProposalID: p.ProposalID, TaskID: p.TaskID,
		Usage: terminal.Usage, UsageProvenance: string(terminal.UsageProvenance)}
	parent := Event{EvidenceRef: resolution.EvidenceRef, Kind: RoutedAttemptCompleted, Repository: resolution.Repository,
		Schema: Schema, Version: EventVersion, Payload: Payload{RoutedAttemptCompleted: parentPayload}}
	if err := validatePayload(parentPayload); err != nil {
		return OutcomeComposite{}, err
	}
	if p.Role == "review" {
		if terminal.Turns != 0 {
			return OutcomeComposite{}, errors.New("review outcome cannot carry implementation turns")
		}
		return OutcomeComposite{Attempt: parent}, nil
	}
	if p.Role != "implementation" {
		return OutcomeComposite{}, fmt.Errorf("unknown resolved role %q", p.Role)
	}
	parentID, err := Identify(parent)
	if err != nil {
		return OutcomeComposite{}, err
	}
	class, verification := mapOutcome(string(terminal.Outcome))
	childPayload := &RoutedTaskOutcomePayload{AttemptID: p.AttemptID, Cause: string(terminal.Cause), Class: class, DemandID: p.DemandID,
		EvidenceRef: string(parentID), LaunchCapabilityRef: terminal.LaunchCapabilityRef, LaunchFidelity: string(terminal.LaunchFidelity),
		OwnerGeneration: terminal.OwnerGeneration, ProposalRef: p.ProposalID, TaskID: p.TaskID, Turns: terminal.Turns,
		Usage: terminal.Usage, UsageProvenance: string(terminal.UsageProvenance), Verification: verification}
	if err := validatePayload(childPayload); err != nil {
		return OutcomeComposite{}, err
	}
	child := Event{EvidenceRef: resolution.EvidenceRef, Kind: RoutedTaskOutcome, Repository: resolution.Repository,
		Schema: Schema, Version: EventVersion, Payload: Payload{RoutedTaskOutcome: childPayload}}
	return OutcomeComposite{Attempt: parent, TaskOutcome: &child}, nil
}

func (s *OutcomeService) verifyWritten(ctx context.Context, proof EvidenceProof, want OutcomeComposite) error {
	exact, err := s.store.ReadProof(ctx, proof)
	if err != nil {
		return err
	}
	return compareOutcomeEvents(exact.Events, want)
}

func existingOutcomeProof(read ExactRead, want OutcomeComposite) (EvidenceProof, error) {
	wantEvents, _ := want.events()
	found := make([]Event, 0, len(wantEvents))
	for _, event := range read.Events {
		if event.Kind == RoutedAttemptCompleted && event.Payload.RoutedAttemptCompleted != nil && event.Payload.RoutedAttemptCompleted.AttemptID == want.Attempt.Payload.RoutedAttemptCompleted.AttemptID {
			found = append(found, event)
		}
		if want.TaskOutcome != nil && event.Kind == RoutedTaskOutcome && event.Payload.RoutedTaskOutcome != nil && event.Payload.RoutedTaskOutcome.AttemptID == want.Attempt.Payload.RoutedAttemptCompleted.AttemptID {
			found = append(found, event)
		}
	}
	if err := compareOutcomeEvents(found, want); err != nil {
		return EvidenceProof{}, err
	}
	entries := make([]EvidenceEntry, 0, len(found))
	for _, event := range found {
		path, err := EventPath(event.EventID)
		if err != nil {
			return EvidenceProof{}, err
		}
		entries = append(entries, EvidenceEntry{Path: path, EventID: event.EventID, ContentDigest: event.ContentDigest})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	return EvidenceProof{EvidenceRef: read.ObservedHead.EvidenceRef, Commit: read.ObservedHead.Commit, Entries: entries}, nil
}

func compareOutcomeEvents(got []Event, want OutcomeComposite) error {
	wantEvents, err := want.events()
	if err != nil || len(got) != len(wantEvents) {
		return fmt.Errorf("outcome event count is %d, want %d", len(got), len(wantEvents))
	}
	byKind := make(map[EventKind]Event, len(got))
	for _, event := range got {
		byKind[event.Kind] = event
	}
	for _, expected := range wantEvents {
		actual, ok := byKind[expected.Kind]
		if !ok || !reflect.DeepEqual(actual.Payload, expected.Payload) || actual.Repository != expected.Repository || actual.EvidenceRef != expected.EvidenceRef {
			return fmt.Errorf("%s differs from requested terminal", expected.Kind)
		}
	}
	return nil
}
