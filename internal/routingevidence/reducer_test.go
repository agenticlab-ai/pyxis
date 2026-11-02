package routingevidence

import (
	"bytes"
	"crypto/ed25519"
	"errors"
	"strings"
	"testing"
)

func TestReducerRecognizesEveryDeclaredReachableState(t *testing.T) {
	r, fx := reducerFixture(t)
	cases := []struct {
		name    string
		events  []Event
		subject SubjectID
		want    StructuralStateName
	}{
		{name: "resolution failed", events: fx.failed(), subject: SubjectID{Kind: SubjectAttempt, ID: "attempt-failed"}, want: StateResolutionFailed},
		{name: "resolved pending outcome", events: fx.implementation(false), subject: SubjectID{Kind: SubjectAttempt, ID: "attempt-impl"}, want: StateResolvedPendingOutcome},
		{name: "resolved terminal", events: fx.implementation(true), subject: SubjectID{Kind: SubjectAttempt, ID: "attempt-impl"}, want: StateResolvedTerminal},
		{name: "process in flight", events: fx.review(false), subject: SubjectID{Kind: SubjectProcess, ID: "process-1"}, want: StateProcessInFlight},
		{name: "process terminal", events: fx.review(true), subject: SubjectID{Kind: SubjectProcess, ID: "process-1"}, want: StateProcessTerminal},
		{name: "invalidated", events: fx.invalidated(), subject: SubjectID{Kind: SubjectAttempt, ID: "attempt-impl"}, want: StateInvalidated},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			state, err := r.Reduce(tc.events)
			if err != nil {
				t.Fatal(err)
			}
			view, err := state.SubjectView(tc.subject)
			if err != nil {
				t.Fatal(err)
			}
			var got StructuralStateName
			switch {
			case view.Attempt != nil:
				got = view.Attempt.State
			case view.Process != nil:
				got = view.Process.State
			}
			if got != tc.want {
				t.Fatalf("state = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestReducerEligibilityOnlyComesFromCompleteStates(t *testing.T) {
	r, fx := reducerFixture(t)
	pending, err := r.Reduce(fx.implementation(false))
	if err != nil {
		t.Fatal(err)
	}
	if !pending.Attempts[0].LaunchEligible || pending.Attempts[0].ProcessEligible {
		t.Fatalf("pending eligibility = %+v", pending.Attempts[0])
	}
	terminal, err := r.Reduce(fx.implementation(true))
	if err != nil {
		t.Fatal(err)
	}
	if terminal.Attempts[0].LaunchEligible || terminal.Attempts[0].ProcessEligible {
		t.Fatalf("implementation terminal eligibility = %+v", terminal.Attempts[0])
	}
	inFlight, err := r.Reduce(fx.review(false))
	if err != nil {
		t.Fatal(err)
	}
	if !inFlight.Processes[0].ProcessEligible || inFlight.Processes[0].ProjectionEligible {
		t.Fatalf("in-flight process eligibility = %+v", inFlight.Processes[0])
	}
	complete, err := r.Reduce(fx.review(true))
	if err != nil {
		t.Fatal(err)
	}
	if complete.Processes[0].ProcessEligible || !complete.Processes[0].ProjectionEligible {
		t.Fatalf("terminal process eligibility = %+v", complete.Processes[0])
	}
	invalid, err := r.Reduce(fx.invalidated())
	if err != nil {
		t.Fatal(err)
	}
	if invalid.Attempts[0].LaunchEligible || invalid.Attempts[0].ProcessEligible {
		t.Fatalf("invalidated eligibility = %+v", invalid.Attempts[0])
	}
}

func TestReducerIsOrderIndependentAndIdempotent(t *testing.T) {
	r, fx := reducerFixture(t)
	events := fx.review(true)
	wantState, err := r.Reduce(events)
	if err != nil {
		t.Fatal(err)
	}
	want, err := EncodeCanonicalView(wantState)
	if err != nil {
		t.Fatal(err)
	}
	orders := [][]Event{
		reverseEvents(events),
		rotateEvents(events, 3),
		append(append([]Event(nil), events...), events[2], events[2], events[len(events)-1]),
	}
	for i, order := range orders {
		state, err := r.Reduce(order)
		if err != nil {
			t.Fatalf("order %d: %v", i, err)
		}
		got, err := EncodeCanonicalView(state)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("order %d changed view\nwant %s\n got %s", i, want, got)
		}
	}
}

func TestReducerRejectsSameIdentityWithDifferentCanonicalBytes(t *testing.T) {
	r, fx := reducerFixture(t)
	demand := fx.demand()
	conflict := cloneEvent(t, demand)
	conflict.Payload.ExecutionDemandFrozen.AssuranceRaise = !conflict.Payload.ExecutionDemandFrozen.AssuranceRaise
	conflict = fx.sign(conflict)
	_, err := r.Reduce([]Event{demand, conflict})
	assertReducerError(t, err, ReducerConflict, "event_id->canonical_bytes")
}

func TestReducerRejectsEveryBrokenJoinClass(t *testing.T) {
	r, fx := reducerFixture(t)
	validImpl := fx.implementation(true)
	validReview := fx.review(true)

	tests := []struct {
		name   string
		mutate func() []Event
		code   ReducerErrorCode
		edge   string
	}{
		{name: "missing demand parent", mutate: func() []Event { return []Event{fx.implementation(false)[1]} }, code: ReducerMissingParent, edge: "resolution.demand_id->demand_origin"},
		{name: "duplicate resolution terminal", mutate: func() []Event {
			events := fx.implementation(false)
			failed := fx.failed()[1]
			failed.Payload.RoutingResolutionFailed.AttemptID = "attempt-impl"
			failed.Payload.RoutingResolutionFailed.DemandID = "demand-1"
			failed.Payload.RoutingResolutionFailed.TaskID = "task-1"
			failed.Payload.RoutingResolutionFailed.PolicyVersion = "routing-policy/v4"
			failed.Payload.RoutingResolutionFailed.RequestedProfile = fx.profile
			failed.Payload.RoutingResolutionFailed.Role = "implementation"
			return append(events, fx.sign(failed))
		}, code: ReducerDuplicate, edge: "attempt_id->resolution_terminal"},
		{name: "cross task", mutate: func() []Event {
			events := fx.implementation(false)
			events[1].Payload.RoutingResolved.TaskID = "other-task"
			events[1] = fx.sign(events[1])
			return events
		}, code: ReducerCrossBoundary, edge: "resolution->demand_origin"},
		{name: "outcome orphan", mutate: func() []Event { return []Event{validImpl[2], validImpl[3]} }, code: ReducerMissingParent, edge: "outcome.attempt_id->routing_resolved"},
		{name: "partial implementation composite", mutate: func() []Event { return append([]Event(nil), validImpl[:3]...) }, code: ReducerPartialComposite, edge: "routed_attempt_completed->routed_task_outcome"},
		{name: "cross owner generation", mutate: func() []Event {
			events := cloneEvents(t, validImpl)
			events[2].Payload.RoutedAttemptCompleted.OwnerGeneration = 2
			events[2] = fx.sign(events[2])
			events[3].Payload.RoutedTaskOutcome.EvidenceRef = string(mustID(t, events[2]))
			events[3] = fx.sign(events[3])
			return events
		}, code: ReducerCrossBoundary, edge: "routed_attempt_completed->routing_resolved"},
		{name: "task outcome terminal mismatch", mutate: func() []Event {
			events := cloneEvents(t, validImpl)
			events[3].Payload.RoutedTaskOutcome.Cause = "worker_failed"
			events[3].Payload.RoutedTaskOutcome.Class = "failed"
			events[3].Payload.RoutedTaskOutcome.Verification = "failed"
			events[3] = fx.sign(events[3])
			return events
		}, code: ReducerCrossBoundary, edge: "routed_task_outcome->routed_attempt_completed"},
		{name: "pass on implementation role", mutate: func() []Event {
			events := cloneEvents(t, validReview)
			events[1].Payload.RoutingResolved.Role = "implementation"
			events[1] = fx.sign(events[1])
			taskOutcome := cloneEvent(t, fixtureEvents(false)[kindIndex(RoutedTaskOutcome)])
			tp := taskOutcome.Payload.RoutedTaskOutcome
			op := events[2].Payload.RoutedAttemptCompleted
			tp.AttemptID, tp.DemandID, tp.TaskID, tp.ProposalRef, tp.EvidenceRef = op.AttemptID, op.DemandID, op.TaskID, op.ProposalID, string(mustID(t, events[2]))
			tp.Usage = op.Usage
			events = append(events, fx.sign(taskOutcome))
			return events
		}, code: ReducerInvalidJoin, edge: "assurance_pass_completed->verified_review_attempt"},
		{name: "partial pass composite", mutate: func() []Event {
			events := cloneEvents(t, validReview)
			return append(events[:4], events[5:]...)
		}, code: ReducerPartialComposite, edge: "pass.finding_records->finding_persisted"},
		{name: "cross revision finding", mutate: func() []Event {
			events := cloneEvents(t, validReview)
			events[4].Payload.FindingPersisted.ReviewedRevision = "other-revision"
			events[4] = fx.sign(events[4])
			return events
		}, code: ReducerCrossBoundary, edge: "finding_persisted->assurance_pass_completed"},
		{name: "satisfied process missing pass", mutate: func() []Event {
			events := cloneEvents(t, validReview)
			terminal := events[len(events)-1]
			terminal.Payload.ReviewProcessCompleted.RequiredPasses = 2
			terminal = fx.sign(terminal)
			events[len(events)-1] = terminal
			return events
		}, code: ReducerPartialComposite, edge: "process.satisfied->required_passes"},
		{name: "orphan disposition", mutate: func() []Event {
			d := cloneEvent(t, fixtureEvents(false)[kindIndex(DispositionRecorded)])
			d.Payload.DispositionRecorded.FindingID = "absent-finding"
			return []Event{fx.sign(d)}
		}, code: ReducerMissingParent, edge: "disposition.finding_id->finding_persisted"},
		{name: "orphan invalidation target", mutate: func() []Event {
			events := fx.implementation(false)
			inv := cloneEvent(t, fixtureEvents(false)[kindIndex(RoutingEvidenceInvalidated)])
			p := inv.Payload.RoutingEvidenceInvalidated
			p.AffectedArtifactRefs, p.AffectedEventIDs, p.AffectedProcessIDs = []string{}, []string{}, []string{}
			p.AffectedAttemptIDs, p.InvalidationID = []string{"absent-attempt"}, ""
			return append(events, fx.sign(inv))
		}, code: ReducerMissingParent, edge: "invalidation.attempt_ids->attempt"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := r.Reduce(tc.mutate())
			assertReducerError(t, err, tc.code, tc.edge)
		})
	}
}

func TestReducerRejectsInvalidSchemaProofAndRepositoryBeforeJoining(t *testing.T) {
	r, fx := reducerFixture(t)
	tests := []struct {
		name  string
		event Event
	}{
		{name: "schema", event: func() Event { e := fx.demand(); e.Schema = "routing-evidence/v2"; return e }()},
		{name: "proof", event: func() Event {
			e := fx.demand()
			e.ProducerProof.Signature = strings.Repeat("0", len(e.ProducerProof.Signature))
			return e
		}()},
		{name: "repository", event: func() Event { e := fx.demand(); e.Repository = "other/repository"; return e }()},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := r.Reduce([]Event{tc.event})
			if err == nil {
				t.Fatal("expected fail-closed error")
			}
		})
	}
}

func TestReducerOwnerGenerationChainAndAtomicTransition(t *testing.T) {
	r, fx := reducerFixture(t)
	pendingEvents := fx.implementation(false)
	transfer := cloneEvent(t, fixtureEvents(false)[kindIndex(AttemptOwnerTransferred)])
	transfer.Payload.AttemptOwnerTransferred.AttemptID = "attempt-impl"
	transfer.Payload.AttemptOwnerTransferred.PriorOwnerGeneration = 1
	transfer.Payload.AttemptOwnerTransferred.NewOwnerGeneration = 2
	transfer = fx.sign(transfer)
	pendingEvents = append(pendingEvents, transfer)
	state, err := r.Reduce(pendingEvents)
	if err != nil {
		t.Fatal(err)
	}
	if state.Attempts[0].OwnerGeneration != 2 {
		t.Fatalf("owner generation = %d", state.Attempts[0].OwnerGeneration)
	}

	parent := fx.implementation(true)[2]
	parent.Payload.RoutedAttemptCompleted.OwnerGeneration = 2
	parent = fx.sign(parent)
	child := fx.implementation(true)[3]
	child.Payload.RoutedTaskOutcome.EvidenceRef = string(mustID(t, parent))
	child.Payload.RoutedTaskOutcome.OwnerGeneration = 2
	child = fx.sign(child)
	if _, err := r.ValidateTransition(state, EventOrComposite{Events: []Event{parent}}); err == nil {
		t.Fatal("partial outcome transition accepted")
	}
	terminal, err := r.ValidateTransition(state, EventOrComposite{Events: []Event{child, parent}})
	if err != nil {
		t.Fatal(err)
	}
	if terminal.Attempts[0].State != StateResolvedTerminal {
		t.Fatalf("state = %q", terminal.Attempts[0].State)
	}

	stale := cloneEvent(t, transfer)
	stale.Payload.AttemptOwnerTransferred.RuntimeFenceNonce = "different"
	stale = fx.sign(stale)
	_, err = r.Reduce(append(pendingEvents, stale))
	assertReducerError(t, err, ReducerConflict, "event_id->canonical_bytes")
}

func TestReducerActivationAndExactSubjectViews(t *testing.T) {
	r, fx := reducerFixture(t)
	events := fx.review(true)
	activation := cloneEvent(t, fixtureEvents(false)[kindIndex(ActivationCheckpointEvent)])
	activation.Payload.ActivationCheckpoint.ActivationReady = true
	activation = fx.sign(activation)
	events = append(events, activation)
	state, err := r.Reduce(events)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Activations) != 1 || !state.Activations[0].ActivationEligible {
		t.Fatalf("activation = %+v", state.Activations)
	}
	subject := activationSubject(state.Activations[0])
	view, err := state.SubjectView(SubjectID{Kind: SubjectActivation, ID: subject})
	if err != nil || view.Activation == nil || !view.Activation.ActivationEligible {
		t.Fatalf("subject view = %+v, %v", view, err)
	}
	if _, err := state.SubjectView(SubjectID{Kind: SubjectAttempt, ID: "absent"}); err == nil {
		t.Fatal("missing subject was inferred")
	}
}

func TestReducerInvalidationPropagatesOnlyAlongExactEdges(t *testing.T) {
	r, fx := reducerFixture(t)
	events := fx.review(true)
	targeted := cloneEvent(t, fixtureEvents(false)[kindIndex(RoutingEvidenceInvalidated)])
	tp := targeted.Payload.RoutingEvidenceInvalidated
	state, err := r.Reduce(events)
	if err != nil {
		t.Fatal(err)
	}
	affected, err := DiscoverAffected(state, QualificationDefect{PolicyVersion: tp.PolicyVersion, NativeProfileID: tp.NativeProfileID})
	if err != nil {
		t.Fatal(err)
	}
	*tp = invalidationPayload(Invalidation{Affected: affected, Disposition: DispositionRerunRequired, AuthorizationRef: tp.AuthorizationRef})
	targeted = fx.sign(targeted)

	state, err = r.Reduce(append(events, targeted))
	if err != nil {
		t.Fatal(err)
	}
	process := state.Processes[0]
	if process.State != StateInvalidated || len(process.InvalidationIDs) != 1 {
		t.Fatalf("process invalidations = %+v", process)
	}
	if process.InvalidationIDs[0] != targeted.Payload.RoutingEvidenceInvalidated.InvalidationID {
		// The producer input leaves the derived ID blank. Compare through the
		// canonical invalidation view instead.
		var targetedID string
		for _, view := range state.Invalidations {
			if view.EventID == mustID(t, targeted) {
				targetedID = view.InvalidationID
			}
		}
		if process.InvalidationIDs[0] != targetedID {
			t.Fatalf("process did not inherit exact invalidation: %+v", process.InvalidationIDs)
		}
	}
}

type reducerFixtures struct {
	t           *testing.T
	policy      ProducerPolicy
	privateKeys map[ProducerClass]ed25519.PrivateKey
	profile     Profile
}

func reducerFixture(t *testing.T) (Reducer, reducerFixtures) {
	t.Helper()
	policy, keys := producerPolicyFixture(t)
	// The family fixture covers one role per routing-terminal kind. The reducer
	// corpus needs both roles without widening either producer class.
	policy.Matrix = append(policy.Matrix,
		ProducerAuthorization{AuthorizationID: "auth-policy", EventKind: RoutingResolved, ImplementationGeneration: generationFor(ProducerResolver), KeyID: keyIDFor(ProducerResolver), ProducerClass: ProducerResolver, Role: "review"},
		ProducerAuthorization{AuthorizationID: "auth-policy", EventKind: RoutingResolutionFailed, ImplementationGeneration: generationFor(ProducerResolver), KeyID: keyIDFor(ProducerResolver), ProducerClass: ProducerResolver, Role: "implementation"},
		ProducerAuthorization{AuthorizationID: "auth-invalidation", EventKind: RoutingEvidenceInvalidated, ImplementationGeneration: generationFor(ProducerEvidenceOwner), KeyID: keyIDFor(ProducerEvidenceOwner), ProducerClass: ProducerEvidenceOwner, Role: "operator_remediation"},
	)
	setPolicyDigest(t, &policy)
	if err := ValidateProducerPolicy(policy); err != nil {
		t.Fatal(err)
	}
	profile := fixtureEvents(false)[kindIndex(ExecutionDemandFrozen)].Payload.ExecutionDemandFrozen.Profile
	fx := reducerFixtures{t: t, policy: policy, privateKeys: keys, profile: profile}
	return NewReducer(policy), fx
}

func (f reducerFixtures) sign(event Event) Event {
	f.t.Helper()
	class := classFor(event.Kind)
	event.Producer = Producer{Class: string(class), Generation: generationFor(class)}
	event.EventID, event.ContentDigest, event.ProducerProof = "", "", ProducerProof{}
	unsigned, err := NewUnsignedEnvelope(event)
	if err != nil {
		f.t.Fatal(err)
	}
	capability := capabilityForRaw(event, f.policy, f.privateKeys)
	proof, err := Sign(unsigned, capability)
	if err != nil {
		f.t.Fatal(err)
	}
	event.ProducerProof = proof
	return event
}

func capabilityForRaw(event Event, policy ProducerPolicy, keys map[ProducerClass]ed25519.PrivateKey) SigningCapability {
	class := ProducerClass(event.Producer.Class)
	roles := []string{}
	if role := eventRole(event); role != "" {
		roles = append(roles, role)
	}
	keyID := keyIDFor(class)
	for _, row := range policy.Matrix {
		if row.EventKind == event.Kind && row.ProducerClass == class && row.ImplementationGeneration == event.Producer.Generation && row.Role == eventRole(event) {
			keyID = row.KeyID
		}
	}
	return SigningCapability{Repository: event.Repository, EvidenceRef: event.EvidenceRef, ProducerClass: class, ImplementationGeneration: event.Producer.Generation, KeyID: keyID, PrivateKey: keys[class], EventKinds: []EventKind{event.Kind}, Roles: roles}
}

func (f reducerFixtures) demand() Event {
	e := cloneEvent(f.t, fixtureEvents(false)[kindIndex(ExecutionDemandFrozen)])
	p := e.Payload.ExecutionDemandFrozen
	p.DemandID, p.TaskID, p.PolicyVersion = "demand-1", "task-1", "routing-policy/v4"
	return f.sign(e)
}

func (f reducerFixtures) failed() []Event {
	demand := f.demand()
	terminal := cloneEvent(f.t, fixtureEvents(false)[kindIndex(RoutingResolutionFailed)])
	p := terminal.Payload.RoutingResolutionFailed
	p.AttemptID, p.DemandID, p.TaskID, p.PolicyVersion, p.Role = "attempt-failed", "demand-1", "task-1", "routing-policy/v4", "review"
	p.RequestedProfile = f.profile
	return []Event{demand, f.sign(terminal)}
}

func (f reducerFixtures) implementation(terminal bool) []Event {
	demand := f.demand()
	resolution := cloneEvent(f.t, fixtureEvents(false)[kindIndex(RoutingResolved)])
	rp := resolution.Payload.RoutingResolved
	rp.AttemptID, rp.DemandID, rp.TaskID, rp.PolicyVersion, rp.Role = "attempt-impl", "demand-1", "task-1", "routing-policy/v4", "implementation"
	rp.RequestedProfile, rp.ProposalID = f.profile, "proposal-1"
	resolution = f.sign(resolution)
	events := []Event{demand, resolution}
	if !terminal {
		return events
	}
	outcome := cloneEvent(f.t, fixtureEvents(false)[kindIndex(RoutedAttemptCompleted)])
	op := outcome.Payload.RoutedAttemptCompleted
	op.AttemptID, op.DemandID, op.TaskID, op.ProposalID, op.EvidenceRef, op.OwnerGeneration = "attempt-impl", "demand-1", "task-1", "proposal-1", string(mustID(f.t, resolution)), 1
	outcome = f.sign(outcome)
	taskOutcome := cloneEvent(f.t, fixtureEvents(false)[kindIndex(RoutedTaskOutcome)])
	tp := taskOutcome.Payload.RoutedTaskOutcome
	tp.AttemptID, tp.DemandID, tp.TaskID, tp.ProposalRef, tp.EvidenceRef = "attempt-impl", "demand-1", "task-1", "proposal-1", string(mustID(f.t, outcome))
	tp.Usage = op.Usage
	return append(events, outcome, f.sign(taskOutcome))
}

func (f reducerFixtures) review(withProcessTerminal bool) []Event {
	demand := f.demand()
	resolution := cloneEvent(f.t, fixtureEvents(false)[kindIndex(RoutingResolved)])
	rp := resolution.Payload.RoutingResolved
	rp.AttemptID, rp.DemandID, rp.TaskID, rp.PolicyVersion, rp.Role = "attempt-review", "demand-1", "task-1", "routing-policy/v4", "review"
	rp.RequestedProfile, rp.ProposalID, rp.ProposalBand = f.profile, "proposal-1", "deep"
	resolution = f.sign(resolution)
	outcome := cloneEvent(f.t, fixtureEvents(false)[kindIndex(RoutedAttemptCompleted)])
	op := outcome.Payload.RoutedAttemptCompleted
	op.AttemptID, op.DemandID, op.TaskID, op.ProposalID, op.EvidenceRef, op.OwnerGeneration = "attempt-review", "demand-1", "task-1", "proposal-1", string(mustID(f.t, resolution)), 1
	outcome = f.sign(outcome)
	pass := cloneEvent(f.t, fixtureEvents(false)[kindIndex(AssurancePassCompleted)])
	pp := pass.Payload.AssurancePassCompleted
	pp.AttemptID, pp.DemandID, pp.TaskID, pp.ProposalID = "attempt-review", "demand-1", "task-1", "proposal-1"
	pp.PassID, pp.ReviewProcessID, pp.ReviewedRevision = "pass-1", "process-1", "revision-1"
	pp.ProcessGeneration, pp.RequiredPasses, pp.PassIndex = 1, 1, 1
	pp.Verdict = "approve"
	pp.RoutingResolutionRef, pp.RoutedOutcomeRef = string(mustID(f.t, resolution)), string(mustID(f.t, outcome))
	pass = f.sign(pass)
	events := []Event{demand, resolution, outcome, pass}
	for _, record := range pp.FindingRecords {
		finding := cloneEvent(f.t, fixtureEvents(false)[kindIndex(FindingPersisted)])
		fp := finding.Payload.FindingPersisted
		fp.NormalizedFinding, fp.PassID, fp.ProposalID, fp.ReviewProcessID, fp.ReviewedRevision = record, "pass-1", "proposal-1", "process-1", "revision-1"
		events = append(events, f.sign(finding))
	}
	if !withProcessTerminal {
		return events
	}
	process := cloneEvent(f.t, fixtureEvents(false)[kindIndex(ReviewProcessCompleted)])
	pc := process.Payload.ReviewProcessCompleted
	pc.DemandID, pc.TaskID, pc.ProposalID, pc.ReviewProcessID, pc.ReviewedRevision = "demand-1", "task-1", "proposal-1", "process-1", "revision-1"
	pc.PlannedAssurance, pc.ProposalBand, pc.EffectiveDepth = f.profile.Assurance, "deep", pp.EffectiveDepth
	pc.ProcessGeneration, pc.RequiredPasses, pc.CompletedPasses = 1, 1, 1
	pc.PassAttemptIDs, pc.PassVerdicts, pc.TopLevelContextIDs = []string{"attempt-review"}, []string{pp.Verdict}, []string{pp.TopLevelContextID}
	pc.AggregateVerdict = pp.Verdict
	pc.Result = "satisfied"
	return append(events, f.sign(process))
}

func (f reducerFixtures) invalidated() []Event {
	events := f.implementation(false)
	inv := cloneEvent(f.t, fixtureEvents(false)[kindIndex(RoutingEvidenceInvalidated)])
	p := inv.Payload.RoutingEvidenceInvalidated
	p.AffectedArtifactRefs, p.AffectedProcessIDs = []string{"proposal-1"}, []string{}
	p.AffectedAttemptIDs, p.AffectedEventIDs = []string{"attempt-impl"}, []string{string(mustID(f.t, events[1]))}
	p.InvalidationID = ""
	return append(events, f.sign(inv))
}

func mustID(t *testing.T, event Event) EventID {
	t.Helper()
	id, err := Identify(event)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func reverseEvents(events []Event) []Event {
	out := append([]Event(nil), events...)
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

func rotateEvents(events []Event, n int) []Event {
	n %= len(events)
	return append(append([]Event(nil), events[n:]...), events[:n]...)
}

func cloneEvents(t *testing.T, events []Event) []Event {
	t.Helper()
	out := make([]Event, len(events))
	for i, event := range events {
		out[i] = cloneEvent(t, event)
	}
	return out
}

func assertReducerError(t *testing.T, err error, code ReducerErrorCode, edge string) {
	t.Helper()
	if err == nil {
		t.Fatal("expected reducer error")
	}
	var typed *ReducerError
	if !errors.As(err, &typed) {
		t.Fatalf("error is not ReducerError: %v", err)
	}
	if typed.Code != code || typed.Edge != edge {
		t.Fatalf("error = code %q edge %q (%v), want code %q edge %q", typed.Code, typed.Edge, err, code, edge)
	}
}
