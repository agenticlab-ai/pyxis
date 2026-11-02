package routingevidence

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"sort"
	"strings"
	"testing"
)

type projectionStore struct {
	exact ExactRead
	reads int
	err   error
}

type failingReader struct{ err error }

func (r failingReader) ReadViews(context.Context, SubjectID) (SubjectViews, error) {
	return SubjectViews{}, r.err
}

func (*projectionStore) Append(context.Context, AppendRequest) (EvidenceProof, error) {
	return EvidenceProof{}, errors.New("unexpected append")
}

func (s *projectionStore) ReadSubject(_ context.Context, subject SubjectID) (ExactRead, error) {
	s.reads++
	if s.err != nil {
		return ExactRead{}, s.err
	}
	copy := s.exact
	copy.Subject = &SubjectView{Subject: subject}
	return copy, nil
}

func (*projectionStore) ReadProof(context.Context, EvidenceProof) (ExactRead, error) {
	return ExactRead{}, errors.New("unexpected proof read")
}

func exactProjectionFixture(t *testing.T, events []Event, subject SubjectID, head string) ExactRead {
	t.Helper()
	events = canonicalizeEvents(t, events)
	entries := make([]EvidenceEntry, 0, len(events))
	files := make(map[string][]byte, len(events))
	for _, event := range events {
		path, err := EventPath(event.EventID)
		if err != nil {
			t.Fatal(err)
		}
		wire, err := EncodeCanonical(event)
		if err != nil {
			t.Fatal(err)
		}
		files[path] = wire
		entries = append(entries, EvidenceEntry{Path: path, EventID: event.EventID, ContentDigest: event.ContentDigest})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	proof := EvidenceProof{EvidenceRef: events[0].EvidenceRef, Commit: head, Entries: entries}
	return ExactRead{ObservedHead: ObservedHead{EvidenceRef: proof.EvidenceRef, Commit: head}, Proof: proof, Events: events, Bytes: files, Subject: &SubjectView{Subject: subject}}
}

func readProjectionFixture(t *testing.T, events []Event, subject SubjectID) (*projectionStore, Reader, SubjectViews) {
	t.Helper()
	store := &projectionStore{exact: exactProjectionFixture(t, events, subject, strings.Repeat("a", 40))}
	reader, err := NewReader(store, NewMemoryViewCache())
	if err != nil {
		t.Fatal(err)
	}
	views, err := reader.ReadViews(t.Context(), subject)
	if err != nil {
		t.Fatal(err)
	}
	return store, reader, views
}

func TestReadViewsAlwaysExactReadsAndCachesOnlyTheExactHead(t *testing.T) {
	_, fx := reducerFixture(t)
	subject := SubjectID{Kind: SubjectProcess, ID: "process-1"}
	store, reader, first := readProjectionFixture(t, fx.review(true), subject)
	second, err := reader.ReadViews(t.Context(), subject)
	if err != nil {
		t.Fatal(err)
	}
	firstWire, _ := EncodeCanonicalView(first)
	secondWire, _ := EncodeCanonicalView(second)
	if store.reads != 2 || !bytes.Equal(firstWire, secondWire) {
		t.Fatalf("reads=%d byte-identical=%v", store.reads, bytes.Equal(firstWire, secondWire))
	}

	store.exact.ObservedHead.Commit = strings.Repeat("b", 40)
	store.exact.Proof.Commit = store.exact.ObservedHead.Commit
	third, err := reader.ReadViews(t.Context(), subject)
	if err != nil {
		t.Fatal(err)
	}
	if store.reads != 3 || third.EvidenceHead.Commit == first.EvidenceHead.Commit || third.Digest == first.Digest {
		t.Fatalf("new head reused cache: reads=%d first=%s third=%s", store.reads, first.Digest, third.Digest)
	}
	if len(third.Measurements) != 1 || third.Measurements[0].Verified != 1 || third.Measurements[0].Usage.InputTokens == 0 {
		t.Fatalf("bounded measurement = %+v", third.Measurements)
	}
}

func TestEveryMediumProjectionRoundTripsOneCanonicalIdentitySet(t *testing.T) {
	_, fx := reducerFixture(t)
	_, _, views := readProjectionFixture(t, fx.review(true), SubjectID{Kind: SubjectProcess, ID: "process-1"})
	media := []Medium{MediumProposal, MediumReport, MediumTaskTracker, MediumProposalTracker, MediumGitHubTracker, MediumCarinaTracker, MediumAudit, MediumMeasurement}
	var eventIDs []EventID
	for _, medium := range media {
		projection, err := Project(views, medium)
		if err != nil {
			t.Fatal(err)
		}
		wire, err := EncodeProjection(projection)
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := DecodeProjection(wire)
		if err != nil || !reflect.DeepEqual(decoded, projection) {
			t.Fatalf("%s round trip: %v", medium, err)
		}
		if err := VerifyProjection(decoded, views); err != nil {
			t.Fatalf("%s verify: %v", medium, err)
		}
		if eventIDs == nil {
			eventIDs = projection.EventIDs
		} else if !reflect.DeepEqual(eventIDs, projection.EventIDs) {
			t.Fatalf("%s projected a different canonical set", medium)
		}
		if bytes.Contains(wire, []byte("message")) || bytes.Contains(wire, []byte("signature")) || bytes.Contains(wire, []byte("model")) {
			t.Fatalf("%s projection leaked payload material: %s", medium, wire)
		}
	}
}

func TestVerifyProjectionRejectsEveryMismatchAndNonAcceptingFace(t *testing.T) {
	_, fx := reducerFixture(t)
	_, _, views := readProjectionFixture(t, fx.review(true), SubjectID{Kind: SubjectProcess, ID: "process-1"})
	base, err := Project(views, MediumProposal)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		code ProjectionErrorCode
		edit func(*Projection)
	}{
		{"missing favorable subset", ProjectionMissing, func(p *Projection) { p.EventIDs = p.EventIDs[1:] }},
		{"extra", ProjectionExtra, func(p *Projection) { p.EventIDs = append(p.EventIDs, EventID(strings.Repeat("f", 64))) }},
		{"stale", ProjectionStale, func(p *Projection) { p.EvidenceHead = strings.Repeat("c", 40) }},
		{"changed", ProjectionChanged, func(p *Projection) { p.ViewDigest = strings.Repeat("0", 64) }},
		{"disconnected", ProjectionDisconnected, func(p *Projection) { p.Proof.Entries[0].ContentDigest = strings.Repeat("0", 64) }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := base
			got.EventIDs = append([]EventID(nil), base.EventIDs...)
			got.Proof.Entries = append([]EvidenceEntry(nil), base.Proof.Entries...)
			tc.edit(&got)
			var projectionErr *ProjectionError
			if err := VerifyProjection(got, views); !errors.As(err, &projectionErr) || projectionErr.Code != tc.code {
				t.Fatalf("error=%v code=%v", err, projectionErr)
			}
		})
	}

	_, _, inFlight := readProjectionFixture(t, fx.review(false), SubjectID{Kind: SubjectProcess, ID: "process-1"})
	nonAccepting, _ := Project(inFlight, MediumReport)
	assertProjectionCode(t, VerifyProjection(nonAccepting, inFlight), ProjectionNonAccepting)

	_, _, invalidated := readProjectionFixture(t, fx.invalidated(), SubjectID{Kind: SubjectAttempt, ID: "attempt-impl"})
	invalidatedFace, _ := Project(invalidated, MediumTaskTracker)
	assertProjectionCode(t, VerifyProjection(invalidatedFace, invalidated), ProjectionInvalidated)
}

func assertProjectionCode(t *testing.T, err error, want ProjectionErrorCode) {
	t.Helper()
	var got *ProjectionError
	if !errors.As(err, &got) || got.Code != want {
		t.Fatalf("error=%v want=%s", err, want)
	}
}

func TestFreshCloneVerifierExplainsSafeChainStateProofAndDiff(t *testing.T) {
	_, fx := reducerFixture(t)
	_, reader, views := readProjectionFixture(t, fx.review(true), SubjectID{Kind: SubjectProcess, ID: "process-1"})
	projection, _ := Project(views, MediumProposal)
	verifier, err := NewFreshCloneVerifier(reader)
	if err != nil {
		t.Fatal(err)
	}
	report, err := verifier.Verify(t.Context(), views.Subject.Subject, projection)
	if err != nil {
		t.Fatal(err)
	}
	if report.TaskID != "task-1" || report.ProposalID != "proposal-1" || report.Harness == "" || report.Role != "review" || report.PolicyVersion != "routing-policy/v4" || report.ReviewProcessID != "process-1" || report.State != string(StateProcessTerminal) || report.CurrentOwnerGeneration != 1 || report.ExactGitObject == "" || report.ProofDigest == "" || len(report.SemanticComponents) < 4 || len(report.Chain) != len(views.Events) {
		t.Fatalf("diagnostic omitted required identity: %+v", report)
	}
	wire, err := EncodeOperatorDiagnostic(report)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"message", "signature", "reason", "native_author_identity_proof", "prompts_version"} {
		if bytes.Contains(wire, []byte(forbidden)) {
			t.Fatalf("diagnostic leaked %q: %s", forbidden, wire)
		}
	}

	projection.EventIDs = projection.EventIDs[1:]
	report, err = verifier.Verify(t.Context(), views.Subject.Subject, projection)
	if err == nil || report.ProjectionDiff == nil || report.ProjectionDiff.Code != ProjectionMissing {
		t.Fatalf("diff report=%+v error=%v", report.ProjectionDiff, err)
	}

	broken, _ := NewFreshCloneVerifier(failingReader{err: reducerError(ReducerMissingParent, "outcome.attempt_id->routing_terminal", "attempt-1", "absent")})
	report, err = broken.Verify(t.Context(), SubjectID{Kind: SubjectAttempt, ID: "attempt-1"})
	if err == nil || report.MissingEdge != "outcome.attempt_id->routing_terminal" || report.State != string(ReducerMissingParent) {
		t.Fatalf("missing-edge report=%+v error=%v", report, err)
	}
}

func TestProjectionEntryPointsRejectMalformedBoundaries(t *testing.T) {
	if _, err := NewReader(nil, nil); err == nil {
		t.Fatal("NewReader accepted a nil store")
	}
	wantErr := errors.New("exact read failed")
	reader, err := NewReader(&projectionStore{err: wantErr}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reader.ReadViews(t.Context(), SubjectID{Kind: SubjectDemand, ID: "demand-1"}); !errors.Is(err, wantErr) {
		t.Fatalf("ReadViews store error = %v", err)
	}

	reader, err = NewReader(&projectionStore{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reader.ReadViews(t.Context(), SubjectID{Kind: SubjectDemand, ID: "demand-1"}); err == nil {
		t.Fatal("ReadViews accepted an invalid exact head")
	}
	if _, err := viewsFromExact(ExactRead{}, SubjectID{Kind: SubjectDemand, ID: "demand-1"}); err == nil {
		t.Fatal("viewsFromExact accepted an invalid exact head")
	}
	exact := ExactRead{
		ObservedHead: ObservedHead{EvidenceRef: "refs/pyxis/evidence", Commit: "head"},
		Proof:        EvidenceProof{EvidenceRef: "refs/pyxis/evidence", Commit: "head"},
	}
	if err := validateExactHead(exact); err == nil {
		t.Fatal("validateExactHead accepted empty event/proof sets")
	}

	if _, err := Project(SubjectViews{}, Medium("foreign")); err == nil {
		t.Fatal("Project accepted an unsupported medium")
	}
	if _, err := Project(SubjectViews{}, MediumProposal); err == nil {
		t.Fatal("Project accepted an incomplete view")
	}
	projectionErr := &ProjectionError{Code: ProjectionMissing, Field: "event_ids"}
	if projectionErr.Error() == "" || !errors.Is(projectionErr, ErrProjectionRejected) {
		t.Fatal("ProjectionError lost its sentinel contract")
	}
	if err := VerifyProjection(Projection{}, SubjectViews{}); err == nil {
		t.Fatal("VerifyProjection accepted an incomplete current view")
	}
	if _, err := EncodeProjection(Projection{}); err == nil {
		t.Fatal("EncodeProjection accepted an incomplete projection")
	}
	if _, err := DecodeProjection([]byte("{")); err == nil {
		t.Fatal("DecodeProjection accepted malformed JSON")
	}
	if _, err := DecodeProjection([]byte(`{"schema":"routing-evidence-projection/v1"} `)); err == nil {
		t.Fatal("DecodeProjection accepted non-canonical JSON")
	}
}
