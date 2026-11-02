package routingevidence

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"reflect"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

type outcomeTestStore struct {
	resolution Event
	attempt    AttemptView
	outcomes   []Event
	readErr    error
}

func (*outcomeTestStore) Append(context.Context, AppendRequest) (EvidenceProof, error) {
	return EvidenceProof{}, errors.New("unexpected direct append")
}

func (s *outcomeTestStore) ReadSubject(context.Context, SubjectID) (ExactRead, error) {
	if s.readErr != nil {
		return ExactRead{}, s.readErr
	}
	events := append([]Event{s.resolution}, s.outcomes...)
	return ExactRead{ObservedHead: ObservedHead{EvidenceRef: s.resolution.EvidenceRef, Commit: "head-current"}, Events: events,
		Subject: &SubjectView{Attempt: &s.attempt}}, nil
}

func (s *outcomeTestStore) ReadProof(_ context.Context, proof EvidenceProof) (ExactRead, error) {
	return ExactRead{Proof: proof, Events: append([]Event(nil), s.outcomes...)}, nil
}

type outcomeTestWriter struct {
	store   *outcomeTestStore
	writes  int
	last    OutcomeComposite
	partial bool
}

func (w *outcomeTestWriter) WriteOutcome(_ context.Context, _ ObservedHead, composite OutcomeComposite) (EvidenceProof, error) {
	w.writes++
	w.last = composite
	events, err := composite.events()
	if err != nil {
		return EvidenceProof{}, err
	}
	if w.partial {
		events = events[:1]
	}
	entries := make([]EvidenceEntry, 0, len(events))
	for i := range events {
		events[i].EventID, _ = Identify(events[i])
		events[i].ContentDigest = "digest-" + string(events[i].Kind)
		path, _ := EventPath(events[i].EventID)
		entries = append(entries, EvidenceEntry{Path: path, EventID: events[i].EventID, ContentDigest: events[i].ContentDigest})
	}
	w.store.outcomes = events
	w.store.attempt.State = StateResolvedTerminal
	w.store.attempt.OutcomeEventID = events[0].EventID
	if len(events) == 2 {
		w.store.attempt.TaskOutcomeEventID = events[1].EventID
	}
	return EvidenceProof{EvidenceRef: composite.Attempt.EvidenceRef, Commit: "head-outcome", Entries: entries}, nil
}

type outcomeTestAuthorizer struct {
	calls int
	err   error
}

func (a *outcomeTestAuthorizer) AuthorizeOutcome(_ context.Context, _ AttemptTerminal, _ AttemptView) error {
	a.calls++
	return a.err
}

func outcomeServiceFixture(t *testing.T, role string) (*OutcomeService, *outcomeTestStore, *outcomeTestWriter, *outcomeTestAuthorizer) {
	t.Helper()
	resolution := Event{EventID: "resolution-event", ContentDigest: "resolution-digest", EvidenceRef: "refs/pyxis/routing-evidence",
		Kind: RoutingResolved, Repository: "owner/repo", Schema: Schema, Version: EventVersion,
		Payload: Payload{RoutingResolved: &RoutingResolvedPayload{AttemptID: "attempt-7", DemandID: "demand-7", Harness: "codex",
			NativeProfileID: "codex/strong", OwnerGeneration: 1, PolicyVersion: "policy/v1", PromotedComponents: []string{},
			ProposalID: "proposal-7", RequestedProfile: Profile{Capability: "strong", Deliberation: "deep", Context: "large", Assurance: "elevated"},
			Resolution: "exact", Role: role, TaskID: "feature/003/T7"}}}
	store := &outcomeTestStore{resolution: resolution, attempt: AttemptView{AttemptID: "attempt-7", DemandID: "demand-7",
		TaskID: "feature/003/T7", Role: role, State: StateResolvedPendingOutcome, ResolutionEventID: resolution.EventID,
		OwnerGeneration: 1, LaunchEligible: true}}
	writer := &outcomeTestWriter{store: store}
	authorizer := &outcomeTestAuthorizer{}
	service, err := NewOutcomeService(store, writer, authorizer)
	if err != nil {
		t.Fatal(err)
	}
	return service, store, writer, authorizer
}

func terminalForTest(outcome OutcomeClass, cause OutcomeCause, fidelity LaunchFidelity, provenance UsageProvenance) AttemptTerminal {
	return AttemptTerminal{AttemptID: "attempt-7", Outcome: outcome, Cause: cause, OwnerGeneration: 1,
		LaunchCapabilityRef: "capability-7", LaunchFidelity: fidelity, UsageProvenance: provenance}
}

func TestCompleteAttemptTerminalCauseAndMappingMatrix(t *testing.T) {
	tests := []struct {
		name       string
		outcome    OutcomeClass
		cause      OutcomeCause
		fidelity   LaunchFidelity
		provenance UsageProvenance
		class      string
		verify     string
	}{
		{"verified", OutcomeVerified, CauseTerminalVerified, FidelityExact, UsageRuntimeObserved, "succeeded", "verified"},
		{"worker failed", OutcomeFailed, CauseWorkerFailed, FidelityExact, UsageRuntimeObserved, "failed", "failed"},
		{"fidelity mismatch", OutcomeFailed, CauseLaunchFidelityMismatch, FidelityMismatch, UsageRuntimeObserved, "failed", "failed"},
		{"preflight failed", OutcomeFailed, CausePreflightFailed, FidelityUnavailable, UsageNotLaunched, "failed", "failed"},
		{"claim lost", OutcomeBlocked, CauseClaimLost, FidelityUnavailable, UsageNotLaunched, "blocked", "not_reached"},
		{"cancelled", OutcomeBlocked, CauseCancelled, FidelityExact, UsageRuntimeObserved, "blocked", "not_reached"},
		{"crash reaped", OutcomeBlocked, CauseCrashReaped, FidelityUnavailable, UsageRuntimeUnavailable, "blocked", "not_reached"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			service, _, writer, authorizer := outcomeServiceFixture(t, "implementation")
			terminal := terminalForTest(tc.outcome, tc.cause, tc.fidelity, tc.provenance)
			if tc.provenance == UsageRuntimeObserved {
				terminal.Usage, terminal.Turns = Usage{InputTokens: 20, OutputTokens: 5}, 2
			}
			proof, err := service.CompleteAttempt(t.Context(), terminal)
			if err != nil {
				t.Fatal(err)
			}
			if len(proof.Entries) != 2 || writer.writes != 1 || authorizer.calls != 1 || writer.last.TaskOutcome == nil {
				t.Fatalf("proof/writes = %d/%d, authorizations=%d", len(proof.Entries), writer.writes, authorizer.calls)
			}
			parent, child := writer.last.Attempt.Payload.RoutedAttemptCompleted, writer.last.TaskOutcome.Payload.RoutedTaskOutcome
			if child.Class != tc.class || child.Verification != tc.verify || child.Cause != parent.Cause ||
				child.AttemptID != parent.AttemptID || child.DemandID != parent.DemandID || child.TaskID != parent.TaskID ||
				child.ProposalRef != parent.ProposalID || child.LaunchCapabilityRef != parent.LaunchCapabilityRef ||
				child.LaunchFidelity != parent.LaunchFidelity || child.OwnerGeneration != parent.OwnerGeneration ||
				child.Usage != parent.Usage || child.UsageProvenance != parent.UsageProvenance {
				t.Fatalf("implementation outcome lost exact mapping:\nparent=%+v\nchild=%+v", parent, child)
			}
		})
	}
}

func TestOutcomeConstructionRejectsIncompleteBoundaries(t *testing.T) {
	if _, err := (OutcomeComposite{}).events(); err == nil {
		t.Fatal("composite without an attempt parent was accepted")
	}
	parent := Event{
		Kind: RoutedAttemptCompleted,
		Payload: Payload{RoutedAttemptCompleted: &RoutedAttemptCompletedPayload{
			AttemptID: "attempt-1",
		}},
	}
	if _, err := (OutcomeComposite{Attempt: parent, TaskOutcome: &Event{}}).events(); err == nil {
		t.Fatal("composite with an invalid task child was accepted")
	}
	if _, err := (SignedOutcomeWriter{}).WriteOutcome(t.Context(), ObservedHead{}, OutcomeComposite{Attempt: parent}); err == nil {
		t.Fatal("signed writer without a store was accepted")
	}
	if _, err := (SignedOutcomeWriter{Store: &outcomeTestStore{}}).WriteOutcome(t.Context(), ObservedHead{}, OutcomeComposite{}); err == nil {
		t.Fatal("signed writer accepted an invalid composite")
	}
	if _, err := NewOutcomeService(nil, nil, nil); err == nil {
		t.Fatal("outcome service without dependencies was accepted")
	}
}

func TestCompleteAttemptReviewWritesOnlyParentAndRetryIsIdempotent(t *testing.T) {
	service, _, writer, authorizer := outcomeServiceFixture(t, "review")
	terminal := terminalForTest(OutcomeVerified, CauseTerminalVerified, FidelityExact, UsageRuntimeObserved)
	terminal.Usage = Usage{InputTokens: 12, OutputTokens: 3}
	first, err := service.CompleteAttempt(t.Context(), terminal)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.CompleteAttempt(t.Context(), terminal)
	if err != nil {
		t.Fatal(err)
	}
	if writer.writes != 1 || authorizer.calls != 1 || writer.last.TaskOutcome != nil || len(first.Entries) != 1 || len(second.Entries) != 1 {
		t.Fatalf("review retry wrote again or emitted a child: writes=%d auth=%d first=%+v second=%+v", writer.writes, authorizer.calls, first, second)
	}
}

func TestCompleteAttemptRejectsNonCanonicalTerminalsBeforeAppend(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*AttemptTerminal, *outcomeTestStore, *outcomeTestWriter, *outcomeTestAuthorizer)
	}{
		{"stale generation", func(v *AttemptTerminal, _ *outcomeTestStore, _ *outcomeTestWriter, _ *outcomeTestAuthorizer) {
			v.OwnerGeneration = 2
		}},
		{"missing usage provenance", func(v *AttemptTerminal, _ *outcomeTestStore, _ *outcomeTestWriter, _ *outcomeTestAuthorizer) {
			v.UsageProvenance = ""
		}},
		{"invalid mapping", func(v *AttemptTerminal, _ *outcomeTestStore, _ *outcomeTestWriter, _ *outcomeTestAuthorizer) {
			v.Outcome = OutcomeFailed
		}},
		{"unbounded usage", func(v *AttemptTerminal, _ *outcomeTestStore, _ *outcomeTestWriter, _ *outcomeTestAuthorizer) {
			v.Usage.InputTokens = 1_000_000_001
		}},
		{"authority denied", func(_ *AttemptTerminal, _ *outcomeTestStore, _ *outcomeTestWriter, a *outcomeTestAuthorizer) {
			a.err = errors.New("stale capability")
		}},
		{"partial writer", func(_ *AttemptTerminal, _ *outcomeTestStore, w *outcomeTestWriter, _ *outcomeTestAuthorizer) {
			w.partial = true
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			service, store, writer, authorizer := outcomeServiceFixture(t, "implementation")
			terminal := terminalForTest(OutcomeVerified, CauseTerminalVerified, FidelityExact, UsageRuntimeObserved)
			terminal.Usage = Usage{InputTokens: 1}
			tc.mutate(&terminal, store, writer, authorizer)
			if _, err := service.CompleteAttempt(t.Context(), terminal); !errors.Is(err, ErrOutcomeRejected) {
				t.Fatalf("CompleteAttempt error = %v, want rejected", err)
			}
			if tc.name != "partial writer" && writer.writes != 0 {
				t.Fatalf("invalid terminal reached writer %d times", writer.writes)
			}
		})
	}
}

func TestCompleteAttemptFailedResolutionEmitsNothingAndConflictDoesNotRewrite(t *testing.T) {
	service, store, writer, _ := outcomeServiceFixture(t, "implementation")
	store.resolution = Event{Kind: RoutingResolutionFailed, Payload: Payload{RoutingResolutionFailed: &RoutingResolutionFailedPayload{AttemptID: "attempt-7"}}}
	store.attempt.State = StateResolutionFailed
	terminal := terminalForTest(OutcomeBlocked, CauseClaimLost, FidelityUnavailable, UsageNotLaunched)
	if _, err := service.CompleteAttempt(t.Context(), terminal); !errors.Is(err, ErrOutcomeRejected) || writer.writes != 0 {
		t.Fatalf("failed resolution completion = %v, writes=%d", err, writer.writes)
	}

	service, _, writer, _ = outcomeServiceFixture(t, "implementation")
	first := terminalForTest(OutcomeVerified, CauseTerminalVerified, FidelityExact, UsageRuntimeObserved)
	first.Usage = Usage{InputTokens: 3}
	if _, err := service.CompleteAttempt(t.Context(), first); err != nil {
		t.Fatal(err)
	}
	conflict := terminalForTest(OutcomeFailed, CauseWorkerFailed, FidelityExact, UsageRuntimeObserved)
	conflict.Usage = Usage{InputTokens: 3}
	if _, err := service.CompleteAttempt(t.Context(), conflict); !errors.Is(err, ErrOutcomeRejected) || writer.writes != 1 {
		t.Fatalf("conflicting retry = %v, writes=%d", err, writer.writes)
	}
}

type fakeTerminalRuntime struct{ err error }

func (r fakeTerminalRuntime) AuthorizeTerminal(port.LaunchCapability) error { return r.err }

func TestFencedAndReaperOutcomeAuthorities(t *testing.T) {
	attempt := AttemptView{AttemptID: "attempt-7", OwnerGeneration: 2}
	capability := port.LaunchCapability{AttemptID: "attempt-7", OwnerGeneration: 2, CapabilityRef: "capability-7"}
	terminal := terminalForTest(OutcomeVerified, CauseTerminalVerified, FidelityExact, UsageRuntimeObserved)
	terminal.OwnerGeneration, terminal.LaunchCapability, terminal.LaunchCapabilityRef = 2, capability, capability.CapabilityRef
	if err := (FencedOutcomeAuthorizer{Runtime: fakeTerminalRuntime{}}).AuthorizeOutcome(t.Context(), terminal, attempt); err != nil {
		t.Fatal(err)
	}
	terminal.OwnerGeneration = 1
	if err := (FencedOutcomeAuthorizer{Runtime: fakeTerminalRuntime{}}).AuthorizeOutcome(t.Context(), terminal, attempt); err == nil {
		t.Fatal("stale capability authorized")
	}
	reaped := terminalForTest(OutcomeBlocked, CauseCrashReaped, FidelityUnavailable, UsageRuntimeUnavailable)
	reaped.OwnerGeneration = 2
	if err := (ReaperOutcomeAuthorizer{AuthorizationReference: "operator:reap-7"}).AuthorizeOutcome(t.Context(), reaped, attempt); err != nil {
		t.Fatal(err)
	}
	reaped.Cause = CauseCancelled
	if err := (ReaperOutcomeAuthorizer{AuthorizationReference: "operator:reap-7"}).AuthorizeOutcome(t.Context(), reaped, attempt); err == nil {
		t.Fatal("reaper authorized a non-crash terminal")
	}
}

func TestOutcomeForImplementationAndReviewDoNotTrustCallerJoins(t *testing.T) {
	_, store, _, _ := outcomeServiceFixture(t, "implementation")
	terminal := terminalForTest(OutcomeVerified, CauseTerminalVerified, FidelityExact, UsageRuntimeObserved)
	got, err := outcomeFor(store.resolution, terminal)
	if err != nil {
		t.Fatal(err)
	}
	p := got.Attempt.Payload.RoutedAttemptCompleted
	if !reflect.DeepEqual([]string{p.DemandID, p.TaskID, p.ProposalID, p.EvidenceRef}, []string{"demand-7", "feature/003/T7", "proposal-7", "resolution-event"}) {
		t.Fatalf("canonical joins = %+v", p)
	}
}

type appendCaptureStore struct{ request AppendRequest }

func (s *appendCaptureStore) Append(_ context.Context, request AppendRequest) (EvidenceProof, error) {
	s.request = request
	return EvidenceProof{EvidenceRef: request.ObservedHead.EvidenceRef, Commit: "outcome-head"}, nil
}
func (*appendCaptureStore) ReadSubject(context.Context, SubjectID) (ExactRead, error) {
	return ExactRead{}, errors.New("unexpected read")
}
func (*appendCaptureStore) ReadProof(context.Context, EvidenceProof) (ExactRead, error) {
	return ExactRead{}, errors.New("unexpected read")
}

func TestSignedOutcomeWriterSignsAtomicImplementationAndSingleReview(t *testing.T) {
	seed := sha256.Sum256([]byte("outcome-writer-test"))
	private := ed25519.NewKeyFromSeed(seed[:])
	capability := SigningCapability{Repository: "owner/repo", EvidenceRef: "refs/pyxis/routing-evidence",
		ProducerClass: ProducerAttemptOwner, ImplementationGeneration: "owner/v1", KeyID: "owner-key", PrivateKey: private,
		EventKinds: []EventKind{RoutedAttemptCompleted, RoutedTaskOutcome}}

	_, fixtureStore, _, _ := outcomeServiceFixture(t, "implementation")
	terminal := terminalForTest(OutcomeVerified, CauseTerminalVerified, FidelityExact, UsageRuntimeObserved)
	composite, err := outcomeFor(fixtureStore.resolution, terminal)
	if err != nil {
		t.Fatal(err)
	}
	capture := &appendCaptureStore{}
	writer := SignedOutcomeWriter{Store: capture, Capability: capability}
	if _, err := writer.WriteOutcome(t.Context(), ObservedHead{EvidenceRef: capability.EvidenceRef, Commit: "resolution-head"}, composite); err != nil {
		t.Fatal(err)
	}
	if capture.request.Composite == nil || capture.request.Event != nil || len(capture.request.Composite.Events) != 2 {
		t.Fatalf("implementation append was not one composite: %+v", capture.request)
	}
	for _, event := range capture.request.Composite.Events {
		if event.Producer.Class != string(ProducerAttemptOwner) || event.ProducerProof.Signature == "" {
			t.Fatalf("event was not owner-signed: %+v", event)
		}
	}

	_, fixtureStore, _, _ = outcomeServiceFixture(t, "review")
	composite, err = outcomeFor(fixtureStore.resolution, terminal)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writer.WriteOutcome(t.Context(), ObservedHead{EvidenceRef: capability.EvidenceRef, Commit: "resolution-head"}, composite); err != nil {
		t.Fatal(err)
	}
	if capture.request.Event == nil || capture.request.Composite != nil || capture.request.Event.Kind != RoutedAttemptCompleted {
		t.Fatalf("review append was not one parent event: %+v", capture.request)
	}
}
