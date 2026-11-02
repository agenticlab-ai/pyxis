package routingevidence

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executiondemand"
	"github.com/agenticlab-ai/pyxis/internal/nativerouting"
)

type admissionStore struct {
	origin         Event
	terminals      []Event
	readProofErr   error
	launchEligible bool
	alterHarness   bool
}

func (s *admissionStore) Append(context.Context, AppendRequest) (EvidenceProof, error) {
	return EvidenceProof{}, errors.New("unexpected direct append")
}

func (s *admissionStore) ReadSubject(_ context.Context, subject SubjectID) (ExactRead, error) {
	if subject.Kind == SubjectAttempt {
		for _, terminal := range s.terminals {
			attemptID, demand, task, role := terminalAttempt(terminal)
			if attemptID != subject.ID {
				continue
			}
			return ExactRead{Subject: &SubjectView{Subject: subject, Attempt: &AttemptView{
				AttemptID: attemptID, DemandID: demand, TaskID: task, Role: role,
				State: StateResolvedPendingOutcome, ResolutionEventID: terminal.EventID,
				OwnerGeneration: 1, LaunchEligible: s.launchEligible,
			}}}, nil
		}
		return ExactRead{}, errors.New("unknown attempt")
	}
	if subject.Kind != SubjectDemand || subject.ID != demandID(s.origin) {
		return ExactRead{}, errors.New("unknown subject")
	}
	return ExactRead{
		ObservedHead: ObservedHead{EvidenceRef: s.origin.EvidenceRef, Commit: "origin-head"},
		Events:       []Event{s.origin},
		Subject: &SubjectView{Subject: subject, Demand: &DemandView{
			DemandID: subject.ID, OriginEventID: s.origin.EventID,
		}},
	}, nil
}

func (s *admissionStore) ReadProof(_ context.Context, proof EvidenceProof) (ExactRead, error) {
	if s.readProofErr != nil {
		return ExactRead{}, s.readProofErr
	}
	for _, terminal := range s.terminals {
		if len(proof.Entries) != 1 || terminal.EventID != proof.Entries[0].EventID {
			continue
		}
		exact := terminal
		if s.alterHarness && exact.Kind == RoutingResolved {
			payload := *exact.Payload.RoutingResolved
			payload.Harness = "altered"
			exact.Payload.RoutingResolved = &payload
		}
		return ExactRead{Proof: proof, Events: []Event{exact}}, nil
	}
	return ExactRead{}, errors.New("proof event is absent")
}

type admissionWriter struct{ store *admissionStore }

func (w admissionWriter) WriteTerminal(_ context.Context, _ ObservedHead, event Event) (EvidenceProof, error) {
	event.EventID = EventID("event-" + terminalAttemptID(event))
	event.ContentDigest = "digest-" + terminalAttemptID(event)
	w.store.terminals = append(w.store.terminals, event)
	path, _ := EventPath(event.EventID)
	return EvidenceProof{EvidenceRef: event.EvidenceRef, Commit: "terminal-head-" + terminalAttemptID(event), Entries: []EvidenceEntry{{
		Path: path, EventID: event.EventID, ContentDigest: event.ContentDigest,
	}}}, nil
}

func terminalAttemptID(event Event) string {
	if event.Kind == RoutingResolved {
		return event.Payload.RoutingResolved.AttemptID
	}
	return event.Payload.RoutingResolutionFailed.AttemptID
}

func terminalAttempt(event Event) (attempt, demand, task, role string) {
	if event.Kind == RoutingResolved {
		p := event.Payload.RoutingResolved
		return p.AttemptID, p.DemandID, p.TaskID, p.Role
	}
	p := event.Payload.RoutingResolutionFailed
	return p.AttemptID, p.DemandID, p.TaskID, p.Role
}

func demandID(event Event) string {
	return event.Payload.ExecutionDemandFrozen.DemandID
}

func testAdmission(t *testing.T) (*AdmissionVerifier, *admissionStore, RouteIntent) {
	t.Helper()
	profile := executiondemand.ProfileMinimum{
		Capability: executiondemand.CapabilityStrong, Deliberation: executiondemand.DeliberationDeep,
		Context: executiondemand.ContextLarge, Assurance: executiondemand.AssuranceElevated,
	}
	origin := Event{
		EventID: "origin", ContentDigest: "origin-digest", Repository: "owner/repo", EvidenceRef: "refs/pyxis/routing-evidence",
		Schema: Schema, Version: EventVersion, Kind: ExecutionDemandFrozen,
		Payload: Payload{ExecutionDemandFrozen: &ExecutionDemandFrozenPayload{
			DemandID: "demand-1", PlanVersion: "003", PolicyVersion: "routing-policy/v1",
			TaskID: "T5", Profile: evidenceProfile(profile),
		}},
	}
	store := &admissionStore{origin: origin, launchEligible: true}
	verifier, err := NewAdmissionVerifier(store, admissionWriter{store: store})
	if err != nil {
		t.Fatal(err)
	}
	intent := RouteIntent{
		AttemptID: "attempt-1", DemandID: "demand-1", TaskID: "T5", Harness: "codex",
		RequestedProfile: profile, Catalog: admissionCatalog(profile), Mode: nativerouting.ModeNormal,
		Trace: &BoundaryTrace{},
	}
	return verifier, store, intent
}

func admissionCatalog(profile executiondemand.ProfileMinimum) nativerouting.ValidatedCatalog {
	return nativerouting.ValidatedCatalog{Identity: "native-policy-v1", Catalog: nativerouting.Catalog{
		SchemaVersion: nativerouting.CatalogSchema, CatalogID: "catalog-v1", Harness: "codex",
		Role: nativerouting.RoleImplementation, AdapterGeneration: "adapter-v1",
		Entries: []nativerouting.CatalogEntry{{
			NativeProfileID: "codex-strong", NativeModel: "gpt-5.6", NativeEffort: "high", Controls: []nativerouting.NativeControl{},
			Provides: nativerouting.Provides{Capability: profile.Capability, Deliberation: profile.Deliberation, Context: profile.Context, Assurance: profile.Assurance},
			CostRank: 1, QualificationID: "qualification-1", QualificationDigest: "qualification-digest", AdapterGeneration: "adapter-v1", Strongest: true,
		}},
	}}
}

func TestAdmissionImplementationExactProofGatesClaim(t *testing.T) {
	verifier, store, intent := testAdmission(t)
	permit, err := verifier.AdmitImplementation(context.Background(), intent)
	if err != nil {
		t.Fatalf("AdmitImplementation: %v", err)
	}
	if len(store.terminals) != 1 || store.terminals[0].Kind != RoutingResolved {
		t.Fatalf("terminals = %#v, want one routing_resolved", store.terminals)
	}
	payload := store.terminals[0].Payload.RoutingResolved
	if payload.DemandID != string(intent.DemandID) || payload.TaskID != intent.TaskID || payload.Harness != intent.Harness ||
		payload.Role != string(nativerouting.RoleImplementation) || payload.PolicyVersion != "routing-policy/v1" ||
		payload.NativeProfileID != "codex-strong" || payload.RequestedProfile != evidenceProfile(intent.RequestedProfile) {
		t.Fatalf("resolved payload lost an exact binding: %#v", payload)
	}
	if err := permit.Authorize(OperationClaim, BoundaryBinding{DemandID: string(intent.DemandID), TaskID: intent.TaskID, Harness: intent.Harness, Role: nativerouting.RoleImplementation}); err != nil {
		t.Fatalf("Authorize claim: %v", err)
	}
	want := []BoundaryObservation{
		{OperationDemandExactRead, true, false}, {OperationResolution, false, false},
		{OperationCanonicalAppend, true, false}, {OperationProofExactRead, true, false},
		{OperationProofExactRead, true, false}, {OperationClaim, false, false},
	}
	if got := intent.Trace.Observations(); !reflect.DeepEqual(got, want) {
		t.Fatalf("trace = %#v, want %#v", got, want)
	}
	if err := permit.Authorize(OperationClaim, BoundaryBinding{DemandID: string(intent.DemandID), TaskID: intent.TaskID, Harness: intent.Harness, Role: nativerouting.RoleImplementation}); err == nil {
		t.Fatal("one permit authorized a second claim")
	}
}

func TestAdmissionFailedAndUnverifiableRoutesAuthorizeNothing(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*admissionStore, *RouteIntent)
	}{
		{name: "unsupported", mutate: func(_ *admissionStore, i *RouteIntent) {
			i.Catalog.Catalog.Entries[0].Provides.Capability = executiondemand.CapabilityEconomy
		}},
		{name: "policy invalid", mutate: func(_ *admissionStore, i *RouteIntent) { i.Catalog.Identity = "" }},
		{name: "proof unavailable", mutate: func(s *admissionStore, _ *RouteIntent) { s.readProofErr = errors.New("evidence ref unavailable") }},
		{name: "stale proof", mutate: func(s *admissionStore, _ *RouteIntent) { s.launchEligible = false }},
		{name: "altered proof", mutate: func(s *admissionStore, _ *RouteIntent) { s.alterHarness = true }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			verifier, store, intent := testAdmission(t)
			tc.mutate(store, &intent)
			if _, err := verifier.AdmitImplementation(context.Background(), intent); !errors.Is(err, ErrAdmissionDenied) {
				t.Fatalf("AdmitImplementation error = %v, want admission denied", err)
			}
			for _, observation := range intent.Trace.Observations() {
				if observation.Operation == OperationClaim || observation.Operation == OperationReviewLaunch {
					t.Fatalf("failed route reached consequential boundary: %#v", intent.Trace.Observations())
				}
			}
		})
	}
}

func TestAdmissionReviewDuplicateAppendsLosingFailure(t *testing.T) {
	verifier, store, intent := testAdmission(t)
	intent.ProposalID = "github:owner/repo#42"
	intent.ReviewAdmissionKey = "process-1/generation-1/pass-1"
	intent.Catalog.Catalog.Role = nativerouting.RoleReview
	first, err := verifier.AdmitReview(context.Background(), intent)
	if err != nil {
		t.Fatalf("first AdmitReview: %v", err)
	}
	if err := first.Authorize(OperationReviewLaunch, BoundaryBinding{DemandID: string(intent.DemandID), TaskID: intent.TaskID, Harness: intent.Harness, Role: nativerouting.RoleReview, ProposalID: intent.ProposalID}); err != nil {
		t.Fatalf("first review launch: %v", err)
	}
	intent.AttemptID, intent.Trace = "attempt-2", &BoundaryTrace{}
	if _, err := verifier.AdmitReview(context.Background(), intent); !errors.Is(err, ErrAdmissionDenied) {
		t.Fatalf("duplicate AdmitReview error = %v, want denied", err)
	}
	if len(store.terminals) != 2 || store.terminals[1].Kind != RoutingResolutionFailed || store.terminals[1].Payload.RoutingResolutionFailed.Reason != "duplicate_admission" {
		t.Fatalf("duplicate terminal = %#v", store.terminals)
	}
	if containsOperation(intent.Trace.Observations(), OperationReviewLaunch) {
		t.Fatal("duplicate review admission reached launch")
	}
}

func TestAdmissionTraceRejectsHiddenNetworkOperation(t *testing.T) {
	verifier, _, intent := testAdmission(t)
	permit, err := verifier.AdmitImplementation(context.Background(), intent)
	if err != nil {
		t.Fatal(err)
	}
	intent.Trace.Observe("harness_probe", true, false)
	if err := permit.Authorize(OperationClaim, BoundaryBinding{DemandID: string(intent.DemandID), TaskID: intent.TaskID, Harness: intent.Harness, Role: nativerouting.RoleImplementation}); err == nil {
		t.Fatal("hidden network operation did not close the claim boundary")
	}
}

func TestVerifyPermitRejectsResolutionFailureProof(t *testing.T) {
	verifier, store, intent := testAdmission(t)
	failed := failedTerminal(intent, "routing-policy/v1", "unsupported", []string{"capability"})
	failed.Repository, failed.EvidenceRef = store.origin.Repository, store.origin.EvidenceRef
	proof, err := (admissionWriter{store: store}).WriteTerminal(context.Background(), ObservedHead{}, failed)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := verifier.VerifyPermit(context.Background(), proof); !errors.Is(err, ErrAdmissionDenied) {
		t.Fatalf("VerifyPermit error = %v, want denied", err)
	}
}
