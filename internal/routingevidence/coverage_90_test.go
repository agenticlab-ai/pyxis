package routingevidence

import (
	"context"

	"errors"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

func TestCoverage90ActivationRejectsMalformedContracts(t *testing.T) {
	_, manifest, mainline, operatorKey := activationFixture(t, false)

	badManifest := manifest
	badManifest.Proofs = append(append([]ActivationProof(nil), manifest.Proofs...), manifest.Proofs[0])
	if _, err := SignActivationManifest(badManifest, "operator-key-1", operatorKey); err == nil {
		t.Fatal("signed a manifest with duplicate proofs")
	}
	if _, err := DigestActivationManifest(badManifest); err == nil {
		t.Fatal("digested a manifest with duplicate proofs")
	}
	if _, err := EncodeActivationManifest(badManifest); err == nil {
		t.Fatal("encoded a manifest with duplicate proofs")
	}
	unsignedManifest := manifest
	unsignedManifest.OperatorSignature = ActivationSignature{}
	if _, err := EncodeActivationManifest(unsignedManifest); err == nil {
		t.Fatal("encoded an unsigned manifest")
	}
	if _, err := SignActivationManifest(manifest, "", operatorKey); err == nil {
		t.Fatal("signed a manifest without a key identity")
	}
	if _, err := DecodeActivationManifest([]byte("{")); err == nil {
		t.Fatal("decoded malformed manifest JSON")
	}

	badMainline := mainline
	badMainline.Proofs = append(append([]ActivationProof(nil), mainline.Proofs...), mainline.Proofs[0])
	if _, err := SignMainlineProof(badMainline, "operator-key-1", operatorKey); err == nil {
		t.Fatal("signed a mainline proof with duplicate proofs")
	}
	if _, err := EncodeMainlineProof(badMainline); err == nil {
		t.Fatal("encoded a mainline proof with duplicate proofs")
	}
	unsignedMainline := mainline
	unsignedMainline.Signature = ActivationSignature{}
	if _, err := EncodeMainlineProof(unsignedMainline); err == nil {
		t.Fatal("encoded an unsigned mainline proof")
	}
	if _, err := SignMainlineProof(mainline, "", operatorKey); err == nil {
		t.Fatal("signed a mainline proof without a key identity")
	}
	if _, err := DecodeMainlineProof([]byte("{")); err == nil {
		t.Fatal("decoded malformed mainline JSON")
	}
}

func TestCoverage90AdmissionRejectsInvalidBoundaryHistoriesAndIntents(t *testing.T) {
	var nilTrace *BoundaryTrace
	nilTrace.Observe(OperationClaim, false, false)
	if got := nilTrace.Observations(); got != nil {
		t.Fatalf("nil trace observations = %#v", got)
	}
	if err := nilTrace.authorize(OperationClaim); err == nil {
		t.Fatal("nil trace authorized a claim")
	}

	tests := []struct {
		name  string
		trace func() *BoundaryTrace
	}{
		{"inference", func() *BoundaryTrace {
			t := &BoundaryTrace{}
			t.Observe("model_call", false, true)
			return t
		}},
		{"duplicate resolution", func() *BoundaryTrace {
			t := &BoundaryTrace{}
			t.Observe(OperationResolution, false, false)
			t.Observe(OperationResolution, false, false)
			return t
		}},
		{"foreign network operation", func() *BoundaryTrace {
			t := &BoundaryTrace{}
			t.Observe(OperationResolution, false, false)
			t.Observe("tracker_read", true, false)
			return t
		}},
		{"missing append and proof read", func() *BoundaryTrace {
			t := &BoundaryTrace{}
			t.Observe(OperationResolution, false, false)
			return t
		}},
		{"recovered permit without proof read", func() *BoundaryTrace { return &BoundaryTrace{} }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.trace().authorize(OperationClaim); err == nil {
				t.Fatal("invalid boundary history was authorized")
			}
		})
	}

	verifier, _, intent := testAdmission(t)
	intent.Trace = nil
	intent.AttemptID = ""
	if _, err := verifier.AdmitImplementation(context.Background(), intent); err == nil {
		t.Fatal("incomplete implementation intent was admitted")
	}
	_, _, intent = testAdmission(t)
	intent.ReviewAdmissionKey = ""
	if _, err := verifier.AdmitReview(context.Background(), intent); err == nil {
		t.Fatal("review intent without an admission key was admitted")
	}
}

func TestCoverage90ContinuityDemandAndEventErrorPaths(t *testing.T) {
	checkpoint := GenesisCheckpoint{AuthorizationReference: "operator:init", Commit: "genesis", EvidenceRef: "refs/pyxis/evidence", Repository: "owner/repo", Tree: "tree-genesis"}
	view := RefView{Repository: checkpoint.Repository, EvidenceRef: checkpoint.EvidenceRef, Exists: true, Head: "head", InitializationAuthorization: checkpoint.AuthorizationReference,
		Commits: []RefCommit{{Object: "genesis", Tree: "tree-genesis"}, {Object: "genesis", Tree: "tree-genesis"}, {Object: "head", Tree: "tree-head", Parents: []string{"genesis"}}}}
	if err := VerifyContinuity(view, checkpoint); err == nil {
		t.Fatal("duplicate commit identity was accepted")
	}

	if _, err := NewIdentityPreimage(Event{}); err == nil {
		t.Fatal("empty event produced an identity preimage")
	}
	if _, err := unionForPayload(struct{}{}); err == nil {
		t.Fatal("unsupported payload entered the event union")
	}
	for _, kind := range []EventKind{
		ExecutionDemandFrozen,
		RoutingResolved,
		RoutingResolutionFailed,
		RoutedAttemptCompleted,
		RoutedTaskOutcome,
		AttemptOwnerTransferred,
		AssurancePassCompleted,
		FindingPersisted,
		DispositionRecorded,
	} {
		t.Run("malformed-"+string(kind), func(t *testing.T) {
			if err := decodePayload(kind, []byte("{"), &Payload{}); err == nil {
				t.Fatal("malformed payload JSON was decoded")
			}
		})
	}
}

func TestCoverage90ProducerRejectsInvalidSigningAndPolicyShapes(t *testing.T) {
	if got := (&IntegrityError{Code: IntegrityUnauthorized, Detail: "denied"}).Error(); !strings.Contains(got, "denied") {
		t.Fatalf("integrity error = %q", got)
	}
	if _, err := Sign(UnsignedEnvelope{}, SigningCapability{}); err == nil {
		t.Fatal("signed a malformed unsigned envelope")
	}

	policy, privateKeys := producerPolicyFixture(t)
	event := fixtureEvents(false)[kindIndex(RoutingResolved)]
	unsigned, err := NewUnsignedEnvelope(event)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Sign(unsigned, SigningCapability{}); err == nil {
		t.Fatal("signed without a complete capability")
	}
	if err := VerifyProducer(event, ProducerPolicy{}); err == nil {
		t.Fatal("verified against an empty producer policy")
	}
	if _, err := DecodeProducerPolicy([]byte(`{"unknown":true}`)); err == nil {
		t.Fatal("decoded a policy with an unknown field")
	}

	mutations := []struct {
		name   string
		mutate func(*ProducerPolicy)
	}{
		{"duplicate key", func(p *ProducerPolicy) { p.Keys = append(p.Keys, p.Keys[0]) }},
		{"invalid public key", func(p *ProducerPolicy) { p.Keys[0].PublicKey = "not-hex" }},
		{"unordered activation points", func(p *ProducerPolicy) { p.Keys[0].PolicyActivationPoints = []string{"z", "a"} }},
		{"invalid authorization kind", func(p *ProducerPolicy) { p.Authorizations[0].Kind = "foreign" }},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			candidate := clonePolicy(t, policy)
			tc.mutate(&candidate)
			setPolicyDigest(t, &candidate)
			if err := ValidateProducerPolicy(candidate); err == nil {
				t.Fatal("invalid producer policy was accepted")
			}
		})
	}

	authenticated := authenticateFixture(t, event, policy, privateKeys)
	if err := VerifyProducer(authenticated, policy); err != nil {
		t.Fatalf("valid control producer failed: %v", err)
	}
}

func TestCoverage90ProjectionAndReducerBranches(t *testing.T) {
	all := fixtureEvents(false)
	if got := eventViews(all); len(got) != len(all) {
		t.Fatalf("event views = %d, want %d", len(got), len(all))
	}
	if got := measurementViews(all); len(got) == 0 {
		t.Fatal("event family produced no measurements")
	}

	_, fx := reducerFixture(t)
	subject := SubjectID{Kind: SubjectProcess, ID: "process-1"}
	exact := exactProjectionFixture(t, fx.review(true), subject, strings.Repeat("a", 40))

	badEvent := exact
	badEvent.Events = append([]Event(nil), exact.Events...)
	badEvent.Events[0].Schema = "foreign"
	if err := validateExactHead(badEvent); err == nil {
		t.Fatal("exact head accepted an event that cannot encode canonically")
	}

	badBytes := exact
	badBytes.Bytes = make(map[string][]byte, len(exact.Bytes))
	for path, wire := range exact.Bytes {
		badBytes.Bytes[path] = append([]byte(nil), wire...)
	}
	badBytes.Bytes[badBytes.Proof.Entries[0].Path] = []byte("tampered")
	if err := validateExactHead(badBytes); err == nil {
		t.Fatal("exact head accepted bytes that differ from the event")
	}

	badProof := exact
	badProof.Proof.Entries = append([]EvidenceEntry(nil), exact.Proof.Entries...)
	badProof.Proof.Entries[0].ContentDigest = strings.Repeat("0", 64)
	if err := validateExactHead(badProof); err == nil {
		t.Fatal("exact head accepted a disconnected proof entry")
	}

	if _, err := viewsFromExact(exact, SubjectID{Kind: SubjectProcess, ID: "absent"}); err == nil {
		t.Fatal("views projected an absent process")
	}
	incoherent := exactProjectionFixture(t, fixtureEvents(false), SubjectID{Kind: SubjectDemand, ID: "absent"}, strings.Repeat("b", 40))
	if _, err := viewsFromExact(incoherent, SubjectID{Kind: SubjectDemand, ID: "absent"}); err == nil {
		t.Fatal("views reduced an incoherent event family")
	}
	store := &projectionStore{exact: incoherent}
	reader, err := NewReader(store, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reader.ReadViews(t.Context(), SubjectID{Kind: SubjectDemand, ID: "absent"}); err == nil {
		t.Fatal("reader accepted an incoherent exact event family")
	}

	_, _, views := readProjectionFixture(t, fx.review(true), subject)
	projection, err := Project(views, MediumProposal)
	if err != nil {
		t.Fatal(err)
	}
	projection.Schema = "foreign"
	if err := VerifyProjection(projection, views); err == nil {
		t.Fatal("verified a projection with a foreign identity")
	}
	notCurrent := views
	notCurrent.currentExactRead = false
	if err := validateViews(notCurrent, true); err == nil {
		t.Fatal("validated a non-current view for publication")
	}
	badDigest := views
	badDigest.Digest = ""
	if err := validateViews(badDigest, true); err == nil {
		t.Fatal("validated a view with a changed digest")
	}

	if _, err := (Reducer{}).ValidateTransition(State{}, EventOrComposite{}); err == nil {
		t.Fatal("empty transition was accepted")
	}
	if _, err := (Reducer{}).admit([]Event{{}}); err == nil {
		t.Fatal("invalid producer event was admitted")
	}
	frozen := fixtureEvents(false)[kindIndex(ExecutionDemandFrozen)]
	if demand, task, _ := demandFields(frozen); demand == "" || task == "" {
		t.Fatalf("frozen demand fields = %q/%q", demand, task)
	}
	for _, outcome := range []string{"verified", "failed", "blocked", "foreign"} {
		class, verification := mapOutcome(outcome)
		if class == "" || verification == "" {
			t.Fatalf("mapOutcome(%q) = %q/%q", outcome, class, verification)
		}
	}
	if hasProcessPass(&reductionIndex{}, "missing") {
		t.Fatal("found an absent review pass")
	}
}

func TestCoverage90ReducerRejectsDisconnectedAtomicAndJoinShapes(t *testing.T) {
	if err := validateJoins(&reductionIndex{
		transfers:   map[string][]Event{"attempt": {{Kind: AttemptOwnerTransferred, Payload: Payload{AttemptOwnerTransferred: &AttemptOwnerTransferredPayload{AttemptID: "attempt"}}}}},
		resolutions: map[string]Event{},
	}); err == nil {
		t.Fatal("owner transfer without a resolved attempt was accepted")
	}
	if err := validatePasses(&reductionIndex{
		passes:       map[string]Event{},
		findings:     map[string]Event{"finding": {Payload: Payload{FindingPersisted: &FindingPersistedPayload{NormalizedFinding: NormalizedFinding{FindingID: "finding"}, PassID: "missing"}}}},
		dispositions: map[string][]Event{},
	}); err == nil {
		t.Fatal("orphan finding was accepted")
	}
	if err := validateProcesses(&reductionIndex{
		passes: map[string]Event{},
		processes: map[string]Event{"process": {Payload: Payload{ReviewProcessCompleted: &ReviewProcessCompletedPayload{
			ReviewProcessID: "process", CompletedPasses: 1,
		}}}},
	}); err == nil {
		t.Fatal("process terminal with an incomplete pass set was accepted")
	}
	if invalidatesAttempt(&RoutingEvidenceInvalidatedPayload{}, "missing", &reductionIndex{resolutions: map[string]Event{}}) {
		t.Fatal("invalidation reached an absent attempt")
	}
	if !invalidatesProcess(&RoutingEvidenceInvalidatedPayload{AffectedEventIDs: []string{"terminal"}}, "process", &reductionIndex{
		processes: map[string]Event{"process": {EventID: "terminal"}},
	}) {
		t.Fatal("process terminal identity did not invalidate its process")
	}

	child := Event{Kind: RoutedTaskOutcome, Payload: Payload{RoutedTaskOutcome: &RoutedTaskOutcomePayload{AttemptID: "attempt"}}}
	if err := validateAtomicBoundary(nil, []Event{child}); err == nil {
		t.Fatal("task outcome without its atomic parent was accepted")
	}
	pass := Event{Kind: AssurancePassCompleted, Payload: Payload{AssurancePassCompleted: &AssurancePassCompletedPayload{
		PassID: "pass", FindingRecords: []NormalizedFinding{{FindingID: "finding"}},
	}}}
	if err := validateAtomicBoundary(nil, []Event{pass}); err == nil {
		t.Fatal("pass without its declared finding was accepted")
	}
	finding := Event{Kind: FindingPersisted, Payload: Payload{FindingPersisted: &FindingPersistedPayload{
		NormalizedFinding: NormalizedFinding{FindingID: "finding"}, PassID: "pass",
	}}}
	if err := validateAtomicBoundary(nil, []Event{finding}); err == nil {
		t.Fatal("finding without its pass was accepted")
	}
	if err := validateAtomicBoundary(nil, []Event{pass, finding}); err != nil {
		t.Fatalf("complete pass composite was rejected: %v", err)
	}
}

func TestCoverage90ReviewHelpersRejectMalformedEvidence(t *testing.T) {
	_, _, _, _, composite := reviewServiceFixture(t, false)
	if _, err := eventBase(nil); err == nil {
		t.Fatal("empty event boundary was accepted")
	}
	terminal := Event{Kind: ReviewProcessCompleted, Payload: Payload{ReviewProcessCompleted: &ReviewProcessCompletedPayload{ReviewProcessID: composite.Pass.ReviewProcessID}}}
	if !hasProcessTerminal([]Event{terminal}, composite.Pass.ReviewProcessID) {
		t.Fatal("review process terminal was not detected")
	}
	if err := compareReviewEvents(
		[]Event{{Kind: ReviewProcessCompleted, Repository: "actual"}},
		[]Event{{Kind: ReviewProcessCompleted, Repository: "expected"}},
	); err == nil {
		t.Fatal("different review evidence compared equal")
	}
	if _, err := proofForReview(ExactRead{}, []Event{{EventID: "bad/id"}}); err == nil {
		t.Fatal("invalid review event identity produced a proof")
	}
	if err := validateDispositionBinding(DispositionEvent{}); err == nil {
		t.Fatal("disposition without exact bindings was accepted")
	}
	if err := validateDispositionBinding(DispositionEvent{
		PassID: "pass", ReviewProcessID: "process", ProposalID: "proposal", ReviewedRevision: "revision", TopLevelContextID: "context",
	}); err == nil {
		t.Fatal("disposition without a valid payload was accepted")
	}
	if err := bindDispositionFinding(nil, DispositionEvent{}); err == nil {
		t.Fatal("disposition without an exact pass was accepted")
	}

	badChild := composite
	badChild.Findings = append([]FindingEvent(nil), composite.Findings...)
	badChild.Findings[0].Payload.FindingID = ""
	if err := validatePassComposite(badChild); err == nil {
		t.Fatal("pass with an invalid finding child was accepted")
	}
	duplicateChild := composite
	duplicateChild.Findings = append([]FindingEvent(nil), composite.Findings...)
	duplicateChild.Findings[1] = duplicateChild.Findings[0]
	if err := validatePassComposite(duplicateChild); err == nil {
		t.Fatal("pass with a duplicate finding child was accepted")
	}
	if err := validatePassAttempt(ExactRead{Subject: &SubjectView{Attempt: &AttemptView{ProcessEligible: true}}}, composite.Pass); err == nil {
		t.Fatal("pass without a routing resolution was accepted")
	}
	if _, err := passEvents(nil, composite); err == nil {
		t.Fatal("pass events were constructed without a canonical boundary")
	}

	passEvent := Event{Kind: AssurancePassCompleted, Payload: Payload{AssurancePassCompleted: &composite.Pass}}
	findingID := composite.Pass.FindingRecords[0].FindingID
	disposition := Event{Kind: DispositionRecorded, Payload: Payload{DispositionRecorded: &DispositionRecordedPayload{FindingID: findingID}}}
	selected := reviewEvents([]Event{passEvent, disposition}, composite.Pass.ReviewProcessID, composite.Pass.PassID, findingID)
	if len(selected) != 2 {
		t.Fatalf("review event selection = %d, want 2", len(selected))
	}

	capability := signingCapability("coverage-90-review", passEvent, ProducerReviewOrchestrator, "review/v1", []EventKind{AssurancePassCompleted}, []string{""})
	capability.PrivateKey = nil
	passEvent.Repository, passEvent.EvidenceRef, passEvent.Schema, passEvent.Version = "owner/repo", "refs/pyxis/evidence", Schema, EventVersion
	capability.Repository, capability.EvidenceRef = passEvent.Repository, passEvent.EvidenceRef
	if _, err := (SignedReviewEventWriter{Store: &appendCaptureStore{}, OrchestratorCapability: capability}).WriteReviewEvents(t.Context(), ObservedHead{}, []Event{passEvent}); err == nil {
		t.Fatal("review event without a signing key was written")
	}
	if _, err := reduceProcessTerminal(nil, ReviewProcessCompletedPayload{PlannedAssurance: "foreign", ProposalBand: "docs-only"}); err == nil {
		t.Fatal("process terminal with an invalid assurance policy was reduced")
	}
}

func TestCoverage90InvalidationIncludesDispositionsAndActivationProofs(t *testing.T) {
	state := State{events: []Event{
		{EventID: "resolution", Kind: RoutingResolved, Payload: Payload{RoutingResolved: &RoutingResolvedPayload{
			AttemptID: "attempt", NativeProfileID: "profile", PolicyVersion: "policy", ProposalID: "proposal",
		}}},
		{EventID: "pass-event", Kind: AssurancePassCompleted, Payload: Payload{AssurancePassCompleted: &AssurancePassCompletedPayload{
			AttemptID: "attempt", PassID: "pass", ProposalID: "proposal", ReviewProcessID: "process", ReviewedRevision: "revision",
			FindingRecords: []NormalizedFinding{{FindingID: "finding"}},
		}}},
		{EventID: "finding-event", Kind: FindingPersisted, Payload: Payload{FindingPersisted: &FindingPersistedPayload{
			NormalizedFinding: NormalizedFinding{FindingID: "finding"},
		}}},
		{EventID: "disposition-event", Kind: DispositionRecorded, Payload: Payload{DispositionRecorded: &DispositionRecordedPayload{FindingID: "finding"}}},
		{EventID: "process-event", Kind: ReviewProcessCompleted, Payload: Payload{ReviewProcessCompleted: &ReviewProcessCompletedPayload{ReviewProcessID: "process"}}},
		{EventID: "activation-event", Kind: ActivationCheckpointEvent, Payload: Payload{ActivationCheckpoint: &ActivationCheckpointPayload{
			ProofIDs: []string{"pass-event"}, ExactMainlineObject: "mainline",
		}}},
	}}
	affected, err := DiscoverAffected(state, QualificationDefect{PolicyVersion: "policy", NativeProfileID: "profile"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []EventID{"activation-event", "disposition-event", "finding-event", "pass-event", "process-event", "resolution"} {
		if !containsEventID(affected.EventIDs, want) {
			t.Errorf("affected event set omitted %q: %v", want, affected.EventIDs)
		}
	}
	if !containsString(affected.ArtifactRefs, "mainline") {
		t.Fatalf("affected artifacts omitted activation mainline: %v", affected.ArtifactRefs)
	}

	invalidationEvent := cloneEvent(t, fixtureEvents(false)[kindIndex(RoutingEvidenceInvalidated)])
	capability := signingCapability("coverage-90-invalidation", invalidationEvent, ProducerEvidenceOwner, "invalidator/v1", []EventKind{RoutingEvidenceInvalidated}, []string{"rerun_required"})
	capability.PrivateKey = nil
	if _, err := (SignedInvalidationWriter{Store: &appendCaptureStore{}, Capability: capability}).WriteInvalidation(t.Context(), ObservedHead{}, invalidationEvent); err == nil {
		t.Fatal("invalidation without a signing key was written")
	}

	acceptance := CurrentAcceptance(SubjectView{
		Activation: &ActivationView{ActivationEligible: true, InvalidationIDs: []string{"invalidation"}},
		Invalidations: []InvalidationView{
			{Disposition: string(DispositionRerunRequired)},
			{Disposition: string(DispositionOperatorRemediation)},
		},
	})
	if acceptance.ActivationEligible || acceptance.Disposition != "" || !acceptance.ReplacementRequired || !acceptance.RemediationRequired {
		t.Fatalf("mixed invalidation acceptance = %+v", acceptance)
	}
}

func TestCoverage90ReviewProjectionRejectsInvalidAndPasslessEvidence(t *testing.T) {
	malformed := ExactRead{ObservedHead: ObservedHead{EvidenceRef: "refs/pyxis/evidence", Commit: "head"}}
	if _, err := ProjectReview(malformed, "process"); err == nil {
		t.Fatal("malformed exact evidence produced a review projection")
	}
	_, fx := reducerFixture(t)
	events := fx.review(true)
	validExact := exactProjectionFixture(t, events, SubjectID{Kind: SubjectProcess, ID: "process-1"}, strings.Repeat("d", 40))
	invalidProjection, err := ProjectReview(validExact, "process-1")
	if err != nil {
		t.Fatal(err)
	}
	invalidProjection.Schema = "foreign"
	wire, err := canonicalJSON(invalidProjection)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeReviewProjection(wire); err == nil {
		t.Fatal("incomplete canonical projection was decoded")
	}

	process := cloneEvent(t, events[len(events)-1])
	p := process.Payload.ReviewProcessCompleted
	p.CompletedPasses = 0
	p.PassAttemptIDs, p.PassVerdicts, p.TopLevelContextIDs = []string{}, []string{}, []string{}
	p.Result = "floor_violation"
	process = fx.sign(process)
	exact := exactProjectionFixture(t, []Event{events[0], process}, SubjectID{Kind: SubjectProcess, ID: p.ReviewProcessID}, strings.Repeat("c", 40))
	if _, err := ProjectReview(exact, p.ReviewProcessID); err == nil {
		t.Fatal("passless review process produced a projection")
	}
}

func TestCoverage90OutcomeOwnershipAndReaperFailures(t *testing.T) {
	terminal := terminalForTest(OutcomeVerified, CauseTerminalVerified, FidelityExact, UsageRuntimeObserved)
	if err := (FencedOutcomeAuthorizer{}).AuthorizeOutcome(t.Context(), terminal, AttemptView{}); err == nil {
		t.Fatal("nil fenced runtime authorized an outcome")
	}

	service, _, _, _ := outcomeServiceFixture(t, "implementation")
	terminal.LaunchCapabilityRef = ""
	if _, err := service.CompleteAttempt(t.Context(), terminal); err == nil {
		t.Fatal("terminal without a launch reference was accepted")
	}
	service, store, _, _ := outcomeServiceFixture(t, "implementation")
	store.readErr = errors.New("read failed")
	if _, err := service.CompleteAttempt(t.Context(), terminalForTest(OutcomeVerified, CauseTerminalVerified, FidelityExact, UsageRuntimeObserved)); err == nil {
		t.Fatal("attempt read error was ignored")
	}
	service, store, _, _ = outcomeServiceFixture(t, "implementation")
	store.attempt.AttemptID = "other"
	if _, err := service.CompleteAttempt(t.Context(), terminalForTest(OutcomeVerified, CauseTerminalVerified, FidelityExact, UsageRuntimeObserved)); err == nil {
		t.Fatal("different canonical attempt was accepted")
	}
	service, store, _, _ = outcomeServiceFixture(t, "implementation")
	store.attempt.State = StateResolutionFailed
	if _, err := service.CompleteAttempt(t.Context(), terminalForTest(OutcomeVerified, CauseTerminalVerified, FidelityExact, UsageRuntimeObserved)); err == nil {
		t.Fatal("non-pending attempt accepted an outcome")
	}

	proof := EvidenceProof{Entries: []EvidenceEntry{{EventID: "event", ContentDigest: "digest"}}}
	if _, err := VerifyOwnershipTransfer(t.Context(), &ownershipStore{}, proof, port.RuntimeFenceProof{}); err == nil {
		t.Fatal("non-transfer event produced ownership proof")
	}

	reaper, _, _, _ := reaperFixture(t, port.RuntimeUnknown)
	reaperStore := reaper.Store.(*outcomeTestStore)
	reaperStore.readErr = errors.New("read failed")
	if _, err := reaper.ReapResolved(t.Context(), "attempt-7"); err == nil {
		t.Fatal("reaper ignored exact-read failure")
	}
	reaper, _, _, _ = reaperFixture(t, port.RuntimeUnknown)
	reaperStore = reaper.Store.(*outcomeTestStore)
	reaperStore.attempt.AttemptID = "other"
	if _, err := reaper.ReapResolved(t.Context(), "attempt-7"); err == nil {
		t.Fatal("reaper accepted a different canonical attempt")
	}
	reaper, observer, _, _ := reaperFixture(t, port.RuntimeUnknown)
	reaperStore = reaper.Store.(*outcomeTestStore)
	reaperStore.attempt.OutcomeEventID = "outcome-event"
	result, err := reaper.ReapResolved(t.Context(), "attempt-7")
	if err != nil || result.State != ReapAlreadyTerminal || observer.calls != 0 {
		t.Fatalf("already-terminal reap = %+v, err=%v, observer calls=%d", result, err, observer.calls)
	}
}
