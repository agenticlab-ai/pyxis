package routingevidence

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

type reviewTestStore struct {
	events      []Event
	lastWritten []Event
	writes      int
	partial     bool
	subjectErr  error
	proofErr    error
	ineligible  bool
}

func (*reviewTestStore) Append(context.Context, AppendRequest) (EvidenceProof, error) {
	return EvidenceProof{}, errors.New("unexpected direct append")
}

func (s *reviewTestStore) ReadSubject(_ context.Context, subject SubjectID) (ExactRead, error) {
	if s.subjectErr != nil {
		return ExactRead{}, s.subjectErr
	}
	read := ExactRead{ObservedHead: ObservedHead{EvidenceRef: "refs/pyxis/routing-evidence", Commit: "head-current"}, Events: append([]Event(nil), s.events...)}
	switch subject.Kind {
	case SubjectAttempt:
		read.Subject = &SubjectView{Attempt: &AttemptView{AttemptID: subject.ID, Role: "review", State: StateResolvedTerminal, ProcessEligible: !s.ineligible}}
	case SubjectProcess:
		read.Subject = &SubjectView{Process: &ProcessView{ReviewProcessID: subject.ID, State: StateProcessInFlight, ProcessEligible: true}}
	case SubjectDemand:
		read.Subject = &SubjectView{Demand: &DemandView{DemandID: subject.ID}}
	default:
		return ExactRead{}, errors.New("unsupported test subject")
	}
	return read, nil
}

func (s *reviewTestStore) ReadProof(_ context.Context, proof EvidenceProof) (ExactRead, error) {
	if s.proofErr != nil {
		return ExactRead{}, s.proofErr
	}
	return ExactRead{ObservedHead: ObservedHead{EvidenceRef: proof.EvidenceRef, Commit: proof.Commit}, Proof: proof, Events: append([]Event(nil), s.lastWritten...)}, nil
}

type reviewTestWriter struct {
	store *reviewTestStore
	err   error
}

func (w *reviewTestWriter) WriteReviewEvents(_ context.Context, _ ObservedHead, events []Event) (EvidenceProof, error) {
	if w.err != nil {
		return EvidenceProof{}, w.err
	}
	w.store.writes++
	stored := make([]Event, 0, len(events))
	for _, event := range events {
		class := ProducerReviewOrchestrator
		if event.Kind == DispositionRecorded {
			class = ProducerMediumAdapter
		}
		event.Producer = Producer{Class: string(class), Generation: "test/v1"}
		event.ProducerProof = ProducerProof{Algorithm: "ed25519", KeyID: "test-key", Schema: ProducerProofSchema, Signature: "00"}
		wire, err := EncodeCanonical(event)
		if err != nil {
			return EvidenceProof{}, err
		}
		decoded, err := DecodeCanonical(wire)
		if err != nil {
			return EvidenceProof{}, err
		}
		stored = append(stored, decoded)
	}
	if w.store.partial && len(stored) > 1 {
		stored = stored[:1]
	}
	w.store.lastWritten = stored
	w.store.events = append(w.store.events, stored...)
	read := ExactRead{ObservedHead: ObservedHead{EvidenceRef: "refs/pyxis/routing-evidence", Commit: "head-written"}}
	return proofForReview(read, stored)
}

type reviewTestVerifier struct {
	act   NativeAct
	err   error
	calls int
}

func (v *reviewTestVerifier) VerifyNativeAct(_ context.Context, _ string) (NativeAct, error) {
	v.calls++
	return v.act, v.err
}

func reviewServiceFixture(t *testing.T, withPass bool) (*ReviewService, *reviewTestStore, *reviewTestWriter, *reviewTestVerifier, PassComposite) {
	t.Helper()
	_, fx := reducerFixture(t)
	all := canonicalEvents(t, fx.review(false))
	base := append([]Event(nil), all[:3]...)
	pass := *all[3].Payload.AssurancePassCompleted
	children := make([]FindingEvent, 0, len(pass.FindingRecords))
	for _, event := range all[4:] {
		children = append(children, FindingEvent{Payload: *event.Payload.FindingPersisted})
	}
	if withPass {
		base = all
	}
	store := &reviewTestStore{events: base}
	writer := &reviewTestWriter{store: store}
	verifier := &reviewTestVerifier{}
	service, err := NewReviewService(store, writer, verifier)
	if err != nil {
		t.Fatal(err)
	}
	return service, store, writer, verifier, PassComposite{Pass: pass, Findings: children}
}

func canonicalEvents(t *testing.T, events []Event) []Event {
	t.Helper()
	out := make([]Event, len(events))
	for i, event := range events {
		wire, err := EncodeCanonical(event)
		if err != nil {
			t.Fatal(err)
		}
		out[i], err = DecodeCanonical(wire)
		if err != nil {
			t.Fatal(err)
		}
	}
	return out
}

func TestReviewWriteShapeAndServiceRejectInvalidWiring(t *testing.T) {
	if _, err := (SignedReviewEventWriter{}).WriteReviewEvents(t.Context(), ObservedHead{}, nil); err == nil {
		t.Fatal("empty review write was accepted")
	}
	for _, events := range [][]Event{
		{{Kind: FindingPersisted}},
		{{Kind: FindingPersisted}, {Kind: DispositionRecorded}},
		{{Kind: FindingPersisted}, {Kind: FindingPersisted}},
	} {
		if err := validateReviewWriteShape(events); err == nil {
			t.Fatalf("invalid review write shape was accepted: %+v", events)
		}
	}
	if _, err := NewReviewService(nil, nil, nil); err == nil {
		t.Fatal("review service accepted missing dependencies")
	}
}

func TestCompletePassReportsReadBindingWriteAndExactReadFailures(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*reviewTestStore, *reviewTestWriter)
	}{
		{"subject read", func(s *reviewTestStore, _ *reviewTestWriter) { s.subjectErr = errors.New("read failed") }},
		{"attempt binding", func(s *reviewTestStore, _ *reviewTestWriter) { s.ineligible = true }},
		{"writer", func(_ *reviewTestStore, w *reviewTestWriter) { w.err = errors.New("write failed") }},
		{"proof read", func(s *reviewTestStore, _ *reviewTestWriter) { s.proofErr = errors.New("proof failed") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service, store, writer, _, composite := reviewServiceFixture(t, false)
			tc.mutate(store, writer)
			if _, err := service.CompletePass(t.Context(), composite); !errors.Is(err, ErrReviewEvidenceRejected) {
				t.Fatalf("CompletePass error = %v", err)
			}
		})
	}
}

func TestCompletePassAppendsOneAtomicBijectionAndExactReadsIt(t *testing.T) {
	service, store, _, _, composite := reviewServiceFixture(t, false)
	proof, err := service.CompletePass(t.Context(), composite)
	if err != nil {
		t.Fatal(err)
	}
	if store.writes != 1 || len(store.lastWritten) != 1+len(composite.Findings) || len(proof.Entries) != len(store.lastWritten) {
		t.Fatalf("writes/events/proof = %d/%d/%d", store.writes, len(store.lastWritten), len(proof.Entries))
	}
	if store.lastWritten[0].Kind != AssurancePassCompleted {
		t.Fatalf("first event = %s", store.lastWritten[0].Kind)
	}
	for _, child := range store.lastWritten[1:] {
		p := child.Payload.FindingPersisted
		if child.Kind != FindingPersisted || p.PassID != composite.Pass.PassID || p.ReviewProcessID != composite.Pass.ReviewProcessID ||
			p.ProposalID != composite.Pass.ProposalID || p.ReviewedRevision != composite.Pass.ReviewedRevision {
			t.Fatalf("finding lost exact parent binding: %+v", child)
		}
	}
	second, err := service.CompletePass(t.Context(), composite)
	if err != nil || store.writes != 1 || len(second.Entries) != len(proof.Entries) {
		t.Fatalf("retry = %+v, %v; writes=%d", second, err, store.writes)
	}
}

func TestCompletePassRejectsMissingExtraChangedAndPartialFindingSets(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*PassComposite, *reviewTestStore)
	}{
		{"missing child", func(c *PassComposite, _ *reviewTestStore) { c.Findings = c.Findings[:1] }},
		{"extra child", func(c *PassComposite, _ *reviewTestStore) {
			c.Findings = append(c.Findings, c.Findings[0])
			c.Pass.Findings++
		}},
		{"changed bytes", func(c *PassComposite, _ *reviewTestStore) { c.Findings[0].Payload.Message += " fabricated" }},
		{"cross revision", func(c *PassComposite, _ *reviewTestStore) { c.Findings[0].Payload.ReviewedRevision = "other" }},
		{"partial writer", func(_ *PassComposite, s *reviewTestStore) { s.partial = true }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			service, store, _, _, composite := reviewServiceFixture(t, false)
			tc.mutate(&composite, store)
			_, err := service.CompletePass(t.Context(), composite)
			if !errors.Is(err, ErrReviewEvidenceRejected) {
				t.Fatalf("error = %v", err)
			}
			if tc.name != "partial writer" && store.writes != 0 {
				t.Fatalf("invalid composite reached writer %d times", store.writes)
			}
		})
	}
}

func TestRecordDispositionRequiresExactNativeActAndAuthorProof(t *testing.T) {
	service, store, _, verifier, composite := reviewServiceFixture(t, true)
	payload := DispositionRecordedPayload{Actor: "operator", ActorAttribution: "github:user-7", AttributionVerification: "verified",
		FindingID: composite.Findings[0].Payload.FindingID, NativeAuthorIdentityProof: "proof:user-7",
		NativeMediumActRef: "github:review:91", SourceOwnedAt: "medium-sequence:42", Verdict: "accepted"}
	event := DispositionEvent{PassID: composite.Pass.PassID, ReviewProcessID: composite.Pass.ReviewProcessID,
		ProposalID: composite.Pass.ProposalID, ReviewedRevision: composite.Pass.ReviewedRevision,
		TopLevelContextID: composite.Pass.TopLevelContextID, Payload: payload}
	verifier.act = nativeAct(payload)
	proof, err := service.RecordDisposition(t.Context(), event)
	if err != nil {
		t.Fatal(err)
	}
	if verifier.calls != 1 || store.writes != 1 || len(proof.Entries) != 1 || store.lastWritten[0].Kind != DispositionRecorded {
		t.Fatalf("verifier/writer/proof = %d/%d/%+v", verifier.calls, store.writes, proof)
	}

	for name, mutate := range map[string]func(*DispositionEvent, *reviewTestVerifier){
		"cross finding":      func(e *DispositionEvent, _ *reviewTestVerifier) { e.Payload.FindingID = "FND-absent" },
		"quoted act":         func(_ *DispositionEvent, v *reviewTestVerifier) { v.act.NativeMediumActRef = "github:quoted:91" },
		"fabricated actor":   func(_ *DispositionEvent, v *reviewTestVerifier) { v.act.ActorAttribution = "github:bot" },
		"unattributed":       func(e *DispositionEvent, _ *reviewTestVerifier) { e.Payload.AttributionVerification = "failed" },
		"wrong author proof": func(_ *DispositionEvent, v *reviewTestVerifier) { v.act.NativeAuthorIdentityProof = "proof:other" },
	} {
		t.Run(name, func(t *testing.T) {
			service, store, _, verifier, composite := reviewServiceFixture(t, true)
			e := event
			e.PassID, e.ReviewProcessID = composite.Pass.PassID, composite.Pass.ReviewProcessID
			verifier.act = nativeAct(e.Payload)
			mutate(&e, verifier)
			if _, err := service.RecordDisposition(t.Context(), e); !errors.Is(err, ErrReviewEvidenceRejected) || store.writes != 0 {
				t.Fatalf("error=%v writes=%d", err, store.writes)
			}
		})
	}
}

func TestCompleteProcessReducesSatisfiedMissingConflictingAndLeakedSets(t *testing.T) {
	base := func(t *testing.T) (*ReviewService, *reviewTestStore, ReviewProcessCompletedPayload) {
		service, store, _, _, composite := reviewServiceFixture(t, true)
		p := ReviewProcessCompletedPayload{AggregateVerdict: composite.Pass.Verdict, AggregateWallMillis: 100,
			CompletedPasses: 1, DemandID: composite.Pass.DemandID, EffectiveDepth: composite.Pass.EffectiveDepth,
			EvidenceRef: "evidence:pass-set", PassAttemptIDs: []string{composite.Pass.AttemptID}, PassVerdicts: []string{composite.Pass.Verdict},
			PlannedAssurance: "ordinary", ProcessGeneration: composite.Pass.ProcessGeneration, ProposalBand: "deep",
			ProposalID: composite.Pass.ProposalID, RequiredPasses: 1, Result: "satisfied", ReviewProcessID: composite.Pass.ReviewProcessID,
			ReviewedRevision: composite.Pass.ReviewedRevision, TaskID: composite.Pass.TaskID, TopLevelContextIDs: []string{composite.Pass.TopLevelContextID}}
		return service, store, p
	}

	t.Run("satisfied", func(t *testing.T) {
		service, store, terminal := base(t)
		if _, err := service.CompleteProcess(t.Context(), ProcessTerminal{Payload: terminal}); err != nil {
			t.Fatal(err)
		}
		got := store.lastWritten[0].Payload.ReviewProcessCompleted
		if got.Result != "satisfied" || got.AggregateVerdict != "approve" || got.CompletedPasses != 1 {
			t.Fatalf("terminal = %+v", got)
		}
	})

	t.Run("missing", func(t *testing.T) {
		service, store, terminal := base(t)
		pass := store.events[3].Payload.AssurancePassCompleted
		pass.RequiredPasses = 2
		terminal.PlannedAssurance, terminal.RequiredPasses = "critical", 2
		terminal.CompletedPasses = 1
		if _, err := service.CompleteProcess(t.Context(), ProcessTerminal{Payload: terminal}); err != nil {
			t.Fatal(err)
		}
		if got := store.lastWritten[0].Payload.ReviewProcessCompleted.Result; got != "floor_violation" {
			t.Fatalf("result = %q", got)
		}
	})

	for _, tc := range []struct {
		name   string
		mutate func(*ReviewProcessCompletedPayload, *reviewTestStore)
	}{
		{"non-accepting", func(_ *ReviewProcessCompletedPayload, s *reviewTestStore) {
			s.events[3].Payload.AssurancePassCompleted.Verdict = "request-changes"
		}},
		{"stale", func(p *ReviewProcessCompletedPayload, _ *reviewTestStore) { p.ReviewedRevision = "stale-revision" }},
		{"leaked context", func(_ *ReviewProcessCompletedPayload, s *reviewTestStore) {
			second := cloneReviewPass(s.events[3], "pass-2", "attempt-2", 2)
			second.Payload.AssurancePassCompleted.TopLevelContextID = s.events[3].Payload.AssurancePassCompleted.TopLevelContextID
			s.events[3].Payload.AssurancePassCompleted.RequiredPasses = 2
			s.events = append(s.events, second)
		}},
		{"favorable subset", func(p *ReviewProcessCompletedPayload, s *reviewTestStore) {
			s.events[3].Payload.AssurancePassCompleted.RequiredPasses = 2
			s.events = append(s.events, cloneReviewPass(s.events[3], "pass-2", "attempt-2", 2))
			p.RequiredPasses = 2
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service, store, terminal := base(t)
			tc.mutate(&terminal, store)
			if len(store.events) > 6 {
				terminal.RequiredPasses = 2
				terminal.PlannedAssurance = "critical"
				if tc.name != "favorable subset" {
					terminal.CompletedPasses = 2
					second := store.events[len(store.events)-1].Payload.AssurancePassCompleted
					terminal.PassAttemptIDs = append(terminal.PassAttemptIDs, second.AttemptID)
					terminal.PassVerdicts = append(terminal.PassVerdicts, second.Verdict)
					terminal.TopLevelContextIDs = append(terminal.TopLevelContextIDs, second.TopLevelContextID)
				}
			}
			if _, err := service.CompleteProcess(t.Context(), ProcessTerminal{Payload: terminal}); err != nil {
				t.Fatal(err)
			}
			if got := store.lastWritten[0].Payload.ReviewProcessCompleted.Result; got == "satisfied" {
				t.Fatalf("invalid set produced satisfied")
			}
		})
	}
}

func cloneReviewPass(event Event, passID, attemptID string, index int) Event {
	copy := event
	p := *event.Payload.AssurancePassCompleted
	p.PassID, p.AttemptID, p.PassIndex = passID, attemptID, index
	copy.Payload = Payload{AssurancePassCompleted: &p}
	copy.EventID = EventID(strings.Repeat(string(rune('a'+index)), 64))
	copy.ContentDigest = strings.Repeat(string(rune('0'+index)), 64)
	return copy
}

func TestReviewProjectionRoundTripsExactIdentitiesAndRejectsDrift(t *testing.T) {
	service, store, _, verifier, composite := reviewServiceFixture(t, true)
	payload := DispositionRecordedPayload{Actor: "worker", ActorAttribution: "github:worker", AttributionVerification: "verified",
		FindingID: composite.Findings[0].Payload.FindingID, NativeAuthorIdentityProof: "proof:worker", NativeMediumActRef: "github:act:1", Verdict: "applied"}
	verifier.act = nativeAct(payload)
	if _, err := service.RecordDisposition(t.Context(), DispositionEvent{PassID: composite.Pass.PassID, ReviewProcessID: composite.Pass.ReviewProcessID,
		ProposalID: composite.Pass.ProposalID, ReviewedRevision: composite.Pass.ReviewedRevision, TopLevelContextID: composite.Pass.TopLevelContextID, Payload: payload}); err != nil {
		t.Fatal(err)
	}
	_, fx := reducerFixture(t)
	terminal := canonicalEvents(t, fx.review(true))
	store.events = append(store.events, terminal[len(terminal)-1])
	exact := exactProjectionFixture(t, store.events, SubjectID{Kind: SubjectProcess, ID: composite.Pass.ReviewProcessID}, "head-projection")
	projection, err := ProjectReview(exact, composite.Pass.ReviewProcessID)
	if err != nil {
		t.Fatal(err)
	}
	wire, err := EncodeReviewProjection(projection)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeReviewProjection(wire)
	if err != nil || !reflect.DeepEqual(decoded, projection) || bytes.Contains(wire, []byte("candidates")) || bytes.Contains(wire, []byte("model")) {
		t.Fatalf("projection round trip=%v equal=%v wire=%s", err, reflect.DeepEqual(decoded, projection), wire)
	}
	if err := VerifyReviewProjection(decoded, exact); err != nil {
		t.Fatal(err)
	}
	decoded.FindingEventIDs = decoded.FindingEventIDs[:1]
	if err := VerifyReviewProjection(decoded, exact); err == nil {
		t.Fatal("changed favorable projection was accepted")
	}
}

func TestReviewProjectionRejectsMalformedBoundaries(t *testing.T) {
	if _, err := ProjectReview(ExactRead{}, ""); err == nil {
		t.Fatal("ProjectReview accepted an empty exact read")
	}
	if err := VerifyReviewProjection(ReviewProjection{}, ExactRead{}); err == nil {
		t.Fatal("VerifyReviewProjection accepted an unsupported schema")
	}
	if err := VerifyReviewProjection(ReviewProjection{Schema: ReviewProjectionSchema}, ExactRead{}); err == nil {
		t.Fatal("VerifyReviewProjection accepted absent canonical evidence")
	}
	if _, err := EncodeReviewProjection(ReviewProjection{}); err == nil {
		t.Fatal("EncodeReviewProjection accepted an incomplete identity set")
	}
	if _, err := DecodeReviewProjection([]byte("{")); err == nil {
		t.Fatal("DecodeReviewProjection accepted malformed JSON")
	}
	if _, err := DecodeReviewProjection([]byte(`{"schema":"wrong"} `)); err == nil {
		t.Fatal("DecodeReviewProjection accepted non-canonical JSON")
	}

	shape := ReviewProjection{
		Schema: ReviewProjectionSchema, Canonical: Projection{Schema: ProjectionSchema},
		ReviewProcessID: "process-1", ReviewedRevision: "revision-1",
		EvidenceRef: "refs/pyxis/evidence", EvidenceCommit: strings.Repeat("a", 40),
		PassEventIDs: []EventID{"bad"}, FindingEventIDs: []EventID{}, DispositionEventIDs: []EventID{},
	}
	if err := validateReviewProjectionShape(shape); err == nil {
		t.Fatal("projection shape accepted an invalid pass event identity")
	}
	shape.PassEventIDs = []EventID{EventID(strings.Repeat("a", 64))}
	shape.ProcessEventID = "bad"
	if err := validateReviewProjectionShape(shape); err == nil {
		t.Fatal("projection shape accepted an invalid process event identity")
	}
}
