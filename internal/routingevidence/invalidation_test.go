package routingevidence

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestDiscoverAffectedExactBlastRadiusAndUnaffectedContinuity(t *testing.T) {
	r, fx := reducerFixture(t)
	affected := fx.review(true)
	unaffected := alternateImplementation(fx, "demand-other", "attempt-other", "task-other", "proposal-other", "routing-policy/v4", "claude/other/deep")
	state, err := r.Reduce(append(affected, unaffected...))
	if err != nil {
		t.Fatal(err)
	}
	set, err := DiscoverAffected(state, QualificationDefect{PolicyVersion: "routing-policy/v4", NativeProfileID: "codex/frontier/max"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(set.AttemptIDs, []string{"attempt-review"}) || !reflect.DeepEqual(set.ReviewProcessIDs, []string{"process-1"}) ||
		!reflect.DeepEqual(set.PassIDs, []string{"pass-1"}) || !reflect.DeepEqual(set.ArtifactRefs, []string{"proposal-1", "revision-1"}) {
		t.Fatalf("affected set = %+v", set)
	}
	for _, event := range unaffected {
		if containsEventID(set.EventIDs, mustID(t, event)) {
			t.Fatalf("unaffected event %s entered blast radius", mustID(t, event))
		}
	}
	view, err := state.SubjectView(SubjectID{Kind: SubjectAttempt, ID: "attempt-other"})
	if err != nil || CurrentAcceptance(view).ReplacementRequired {
		t.Fatalf("unaffected route continuity = %+v, %v", view, err)
	}
}

func TestInvalidationBoundariesFailClosedBeforeMutation(t *testing.T) {
	if _, err := DiscoverAffected(State{}, QualificationDefect{}); err == nil {
		t.Fatal("empty qualification defect was accepted")
	}
	if _, err := DiscoverAffected(State{}, QualificationDefect{PolicyVersion: "policy/v1", NativeProfileID: "profile"}); err == nil {
		t.Fatal("unreachable qualification defect was accepted")
	}
	transferred := Event{Kind: AttemptOwnerTransferred, Payload: Payload{AttemptOwnerTransferred: &AttemptOwnerTransferredPayload{AttemptID: "attempt"}}}
	if got := eventAttemptID(transferred); got != "attempt" {
		t.Fatalf("transferred attempt ID = %q", got)
	}
	if _, err := (SignedInvalidationWriter{}).WriteInvalidation(t.Context(), ObservedHead{}, Event{}); err == nil {
		t.Fatal("signed invalidation writer without a store was accepted")
	}
	if _, err := NewInvalidationService(nil, nil, nil); err == nil {
		t.Fatal("invalidation service without dependencies was accepted")
	}
	var service *InvalidationService
	if _, err := service.Invalidate(t.Context(), Invalidation{}); !errors.Is(err, ErrInvalidationRejected) {
		t.Fatalf("nil service error = %v", err)
	}
	service = &InvalidationService{}
	invalid := Invalidation{Affected: AffectedSet{AttemptIDs: []string{"attempt"}}}
	if _, err := service.Invalidate(t.Context(), invalid); !errors.Is(err, ErrInvalidationRejected) {
		t.Fatalf("invalid authorization/disposition error = %v", err)
	}
}

func TestRerunInvalidationExcludesEveryCurrentAcceptanceAndReplacementPasses(t *testing.T) {
	r, fx := reducerFixture(t)
	original := fx.review(true)
	state, err := r.Reduce(original)
	if err != nil {
		t.Fatal(err)
	}
	set, err := DiscoverAffected(state, QualificationDefect{PolicyVersion: "routing-policy/v4", NativeProfileID: "codex/frontier/max"})
	if err != nil {
		t.Fatal(err)
	}
	invalidation := invalidationEvent(fx, state, set, DispositionRerunRequired)
	invalidated, err := r.Reduce(append(original, invalidation))
	if err != nil {
		t.Fatal(err)
	}
	process, err := invalidated.SubjectView(SubjectID{Kind: SubjectProcess, ID: "process-1"})
	if err != nil {
		t.Fatal(err)
	}
	acceptance := CurrentAcceptance(process)
	if acceptance.ReviewSatisfied || acceptance.ProjectionEligible || !acceptance.ReplacementRequired || acceptance.RemediationRequired {
		t.Fatalf("invalidated acceptance = %+v", acceptance)
	}
	attempt, err := invalidated.SubjectView(SubjectID{Kind: SubjectAttempt, ID: "attempt-review"})
	if err != nil || CurrentAcceptance(attempt).ReviewEligible {
		t.Fatalf("invalidated attempt remained eligible: %+v, %v", attempt, err)
	}

	replacement := alternateReview(fx, "demand-replacement", "attempt-replacement", "process-replacement", "pass-replacement", "routing-policy/v5", "claude/qualified/deep")
	replaced, err := r.Reduce(append(append(original, invalidation), replacement...))
	if err != nil {
		t.Fatal(err)
	}
	replacementView, err := replaced.SubjectView(SubjectID{Kind: SubjectProcess, ID: "process-replacement"})
	if err != nil {
		t.Fatal(err)
	}
	replacementAcceptance := CurrentAcceptance(replacementView)
	if !replacementAcceptance.ReviewSatisfied || !replacementAcceptance.ProjectionEligible || replacementAcceptance.ReplacementRequired {
		t.Fatalf("replacement acceptance = %+v", replacementAcceptance)
	}
	stillInvalid, _ := replaced.SubjectView(SubjectID{Kind: SubjectProcess, ID: "process-1"})
	if CurrentAcceptance(stillInvalid).ReviewSatisfied {
		t.Fatal("replacement rewrote the invalidated original process")
	}
}

func TestLateOperatorRemediationEmitsOneCompleteBriefWithoutMutation(t *testing.T) {
	r, fx := reducerFixture(t)
	events := fx.review(true)
	before := canonicalEventBytes(t, events)
	state, err := r.Reduce(events)
	if err != nil {
		t.Fatal(err)
	}
	set, err := DiscoverAffected(state, QualificationDefect{PolicyVersion: "routing-policy/v4", NativeProfileID: "codex/frontier/max"})
	if err != nil {
		t.Fatal(err)
	}
	invalidation := invalidationEvent(fx, state, set, DispositionOperatorRemediation)
	late, err := r.Reduce(append(events, invalidation))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, canonicalEventBytes(t, events)) || len(late.RemediationBriefs) != 1 {
		t.Fatalf("accepted history changed or brief count = %d", len(late.RemediationBriefs))
	}
	brief := late.RemediationBriefs[0]
	if brief.PolicyVersion != set.PolicyVersion || brief.NativeProfileID != set.NativeProfileID || !reflect.DeepEqual(brief.EventIDs, set.EventIDs) ||
		!reflect.DeepEqual(brief.AttemptIDs, set.AttemptIDs) || !reflect.DeepEqual(brief.ReviewProcessIDs, set.ReviewProcessIDs) ||
		!reflect.DeepEqual(brief.PassIDs, set.PassIDs) || !reflect.DeepEqual(brief.ArtifactRefs, set.ArtifactRefs) || brief.DecisionRequired == "" {
		t.Fatalf("remediation brief = %+v", brief)
	}
	view, err := late.SubjectView(SubjectID{Kind: SubjectProcess, ID: "process-1"})
	if err != nil {
		t.Fatal(err)
	}
	acceptance := CurrentAcceptance(view)
	if !acceptance.RemediationRequired || acceptance.ReplacementRequired || acceptance.ReviewSatisfied || len(view.RemediationBriefs) != 1 {
		t.Fatalf("late acceptance = %+v, view briefs = %+v", acceptance, view.RemediationBriefs)
	}
}

func TestInvalidationServiceRepeatAndConflictsFailClosed(t *testing.T) {
	r, fx := reducerFixture(t)
	store := &invalidationTestStore{t: t, reducer: r, events: canonicalizeEvents(t, fx.review(true))}
	writer := &invalidationTestWriter{store: store, sign: fx.sign}
	authorizer := &invalidationTestAuthorizer{}
	service, err := NewInvalidationService(store, writer, authorizer)
	if err != nil {
		t.Fatal(err)
	}
	state, err := r.Reduce(store.events)
	if err != nil {
		t.Fatal(err)
	}
	set, err := DiscoverAffected(state, QualificationDefect{PolicyVersion: "routing-policy/v4", NativeProfileID: "codex/frontier/max"})
	if err != nil {
		t.Fatal(err)
	}
	in := Invalidation{Affected: set, Disposition: DispositionRerunRequired, AuthorizationRef: "operator-decision-7"}
	first, err := service.Invalidate(t.Context(), in)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.Invalidate(t.Context(), in)
	if err != nil {
		t.Fatal(err)
	}
	firstWire, _ := EncodeCanonical(store.events[len(store.events)-1])
	secondWire, _ := EncodeCanonical(store.events[len(store.events)-1])
	if !reflect.DeepEqual(first.Proof.Entries, second.Proof.Entries) || !bytes.Equal(firstWire, secondWire) || writer.writes != 1 || authorizer.calls != 1 {
		t.Fatalf("repeat: first=%+v second=%+v writes=%d auth=%d", first, second, writer.writes, authorizer.calls)
	}

	changed := in
	changed.Affected = cloneAffected(set)
	changed.Affected.ArtifactRefs = append(changed.Affected.ArtifactRefs, "proposal-injected")
	if _, err := service.Invalidate(t.Context(), changed); !errors.Is(err, ErrInvalidationRejected) {
		t.Fatalf("altered blast radius error = %v", err)
	}
	changed = in
	changed.Disposition = DispositionOperatorRemediation
	if _, err := service.Invalidate(t.Context(), changed); !errors.Is(err, ErrInvalidationRejected) {
		t.Fatalf("altered disposition error = %v", err)
	}
	changed = in
	changed.AuthorizationRef = "different-authorization"
	if _, err := service.Invalidate(t.Context(), changed); !errors.Is(err, ErrInvalidationRejected) {
		t.Fatalf("altered authorization error = %v", err)
	}
	if writer.writes != 1 || len(store.events) != len(fx.review(true))+1 {
		t.Fatalf("conflicts weakened history: writes=%d events=%d", writer.writes, len(store.events))
	}
}

type invalidationTestAuthorizer struct{ calls int }

func (a *invalidationTestAuthorizer) AuthorizeInvalidation(_ context.Context, _ Invalidation) error {
	a.calls++
	return nil
}

type invalidationTestStore struct {
	t       *testing.T
	reducer Reducer
	events  []Event
	last    Event
}

func (*invalidationTestStore) Append(context.Context, AppendRequest) (EvidenceProof, error) {
	return EvidenceProof{}, errors.New("unexpected direct append")
}

func (s *invalidationTestStore) ReadSubject(_ context.Context, subject SubjectID) (ExactRead, error) {
	state, err := s.reducer.Reduce(s.events)
	if err != nil {
		return ExactRead{}, err
	}
	view, err := state.SubjectView(subject)
	if err != nil {
		return ExactRead{}, err
	}
	return ExactRead{ObservedHead: ObservedHead{EvidenceRef: "refs/pyxis/routing-evidence", Commit: "head-current"},
		Proof: proofForEvents(s.t, s.events), Events: append([]Event(nil), s.events...), Subject: &view}, nil
}

func (s *invalidationTestStore) ReadProof(_ context.Context, proof EvidenceProof) (ExactRead, error) {
	return ExactRead{ObservedHead: ObservedHead{EvidenceRef: proof.EvidenceRef, Commit: proof.Commit}, Proof: proof, Events: []Event{s.last}}, nil
}

type invalidationTestWriter struct {
	store  *invalidationTestStore
	sign   func(Event) Event
	writes int
}

func (w *invalidationTestWriter) WriteInvalidation(_ context.Context, _ ObservedHead, event Event) (EvidenceProof, error) {
	w.writes++
	stored := canonicalEvent(w.store.t, w.sign(event))
	w.store.events = append(w.store.events, stored)
	w.store.last = stored
	proof := proofForEvents(w.store.t, []Event{stored})
	proof.Commit = "head-written"
	return proof, nil
}

func proofForEvents(t *testing.T, events []Event) EvidenceProof {
	t.Helper()
	proof := EvidenceProof{EvidenceRef: "refs/pyxis/routing-evidence", Commit: "head-current", Entries: []EvidenceEntry{}}
	for _, event := range events {
		stored := canonicalEvent(t, event)
		path, err := EventPath(stored.EventID)
		if err != nil {
			t.Fatal(err)
		}
		proof.Entries = append(proof.Entries, EvidenceEntry{Path: path, EventID: stored.EventID, ContentDigest: stored.ContentDigest})
	}
	return proof
}

func invalidationEvent(fx reducerFixtures, state State, set AffectedSet, disposition InvalidationDisposition) Event {
	base, err := eventBase(state.events)
	if err != nil {
		fx.t.Fatal(err)
	}
	payload := invalidationPayload(Invalidation{Affected: set, Disposition: disposition, AuthorizationRef: "operator-decision-7"})
	base.Kind, base.Payload = RoutingEvidenceInvalidated, Payload{RoutingEvidenceInvalidated: &payload}
	return fx.sign(base)
}

func alternateImplementation(fx reducerFixtures, demandID, attemptID, taskID, proposalID, policyVersion, nativeProfile string) []Event {
	events := cloneEvents(fx.t, fx.implementation(true))
	demand := events[0].Payload.ExecutionDemandFrozen
	demand.DemandID, demand.TaskID, demand.PolicyVersion = demandID, taskID, policyVersion
	events[0] = fx.sign(events[0])
	resolution := events[1].Payload.RoutingResolved
	resolution.AttemptID, resolution.DemandID, resolution.TaskID, resolution.ProposalID = attemptID, demandID, taskID, proposalID
	resolution.PolicyVersion, resolution.NativeProfileID = policyVersion, nativeProfile
	events[1] = fx.sign(events[1])
	outcome := events[2].Payload.RoutedAttemptCompleted
	outcome.AttemptID, outcome.DemandID, outcome.TaskID, outcome.ProposalID, outcome.EvidenceRef = attemptID, demandID, taskID, proposalID, string(mustID(fx.t, events[1]))
	events[2] = fx.sign(events[2])
	taskOutcome := events[3].Payload.RoutedTaskOutcome
	taskOutcome.AttemptID, taskOutcome.DemandID, taskOutcome.TaskID, taskOutcome.ProposalRef, taskOutcome.EvidenceRef = attemptID, demandID, taskID, proposalID, string(mustID(fx.t, events[2]))
	events[3] = fx.sign(events[3])
	return events
}

func alternateReview(fx reducerFixtures, demandID, attemptID, processID, passID, policyVersion, nativeProfile string) []Event {
	events := cloneEvents(fx.t, fx.review(true))
	proposalID, revision, taskID := "proposal-1", "revision-1", "task-1"
	demand := events[0].Payload.ExecutionDemandFrozen
	demand.DemandID, demand.TaskID, demand.PolicyVersion = demandID, taskID, policyVersion
	events[0] = fx.sign(events[0])
	resolution := events[1].Payload.RoutingResolved
	resolution.AttemptID, resolution.DemandID, resolution.PolicyVersion, resolution.NativeProfileID = attemptID, demandID, policyVersion, nativeProfile
	events[1] = fx.sign(events[1])
	outcome := events[2].Payload.RoutedAttemptCompleted
	outcome.AttemptID, outcome.DemandID, outcome.EvidenceRef = attemptID, demandID, string(mustID(fx.t, events[1]))
	events[2] = fx.sign(events[2])
	pass := events[3].Payload.AssurancePassCompleted
	pass.AttemptID, pass.DemandID, pass.ReviewProcessID, pass.PassID = attemptID, demandID, processID, passID
	pass.ProposalID, pass.ReviewedRevision = proposalID, revision
	pass.RoutingResolutionRef, pass.RoutedOutcomeRef = string(mustID(fx.t, events[1])), string(mustID(fx.t, events[2]))
	for i := range pass.FindingRecords {
		pass.FindingRecords[i].FindingID += "-replacement"
	}
	events[3] = fx.sign(events[3])
	for i := 4; i < len(events)-1; i++ {
		finding := events[i].Payload.FindingPersisted
		finding.FindingID += "-replacement"
		finding.PassID, finding.ReviewProcessID, finding.ProposalID, finding.ReviewedRevision = passID, processID, proposalID, revision
		events[i] = fx.sign(events[i])
	}
	terminal := events[len(events)-1].Payload.ReviewProcessCompleted
	terminal.DemandID, terminal.ReviewProcessID = demandID, processID
	terminal.PassAttemptIDs = []string{attemptID}
	events[len(events)-1] = fx.sign(events[len(events)-1])
	return events
}

func containsEventID(values []EventID, want EventID) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func canonicalEvent(t *testing.T, event Event) Event {
	t.Helper()
	wire, err := EncodeCanonical(event)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeCanonical(wire)
	if err != nil {
		t.Fatal(err)
	}
	return decoded
}

func canonicalizeEvents(t *testing.T, events []Event) []Event {
	t.Helper()
	out := make([]Event, len(events))
	for i, event := range events {
		out[i] = canonicalEvent(t, event)
	}
	return out
}

func canonicalEventBytes(t *testing.T, events []Event) []byte {
	t.Helper()
	var out []byte
	for _, event := range events {
		wire, err := EncodeCanonical(event)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, wire...)
	}
	return out
}

func cloneAffected(in AffectedSet) AffectedSet {
	in.EventIDs = append([]EventID(nil), in.EventIDs...)
	in.AttemptIDs = append([]string(nil), in.AttemptIDs...)
	in.ReviewProcessIDs = append([]string(nil), in.ReviewProcessIDs...)
	in.PassIDs = append([]string(nil), in.PassIDs...)
	in.ArtifactRefs = append([]string(nil), in.ArtifactRefs...)
	return in
}
