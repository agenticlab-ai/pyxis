package carina

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/implreview"
	"github.com/agenticlab-ai/pyxis/internal/implreview/persist"
	"github.com/agenticlab-ai/pyxis/internal/implreview/publication"
	implreport "github.com/agenticlab-ai/pyxis/internal/implreview/report"
)

type memoryPublicationStore struct {
	record publication.Record
}

func (*memoryPublicationStore) Preflight(context.Context) error { return nil }

func (s *memoryPublicationStore) Admit(_ context.Context, admission publication.Admission) (publication.Record, error) {
	if s.record.PublicationAttemptID != "" {
		if s.record.Admission != admission {
			return publication.Record{}, errors.New("conflicting admission")
		}
		return s.record, nil
	}
	s.record = publication.Record{Admission: admission, PublicationAttemptID: "publication-1", IdempotencyID: "idempotency-1", Stage: publication.StageAccepted}
	return s.record, nil
}

func (s *memoryPublicationStore) Arm(_ context.Context, prior publication.Record) (publication.Record, bool, error) {
	if s.record != prior {
		return publication.Record{}, false, errors.New("stale arm")
	}
	if s.record.Stage != publication.StageAccepted {
		return s.record, false, nil
	}
	s.record.Stage, s.record.CreateArmed = publication.StageArmed, true
	return s.record, true, nil
}

func (s *memoryPublicationStore) RecordReceipt(_ context.Context, prior publication.Record, receipt string) (publication.Record, error) {
	if s.record != prior || s.record.Stage != publication.StageArmed {
		return publication.Record{}, errors.New("stale receipt")
	}
	s.record.Stage, s.record.NativeReceipt = publication.StageReceiptRecorded, receipt
	return s.record, nil
}

func (s *memoryPublicationStore) Finish(_ context.Context, prior publication.Record, terminal implreview.PublicationTerminal) (publication.Record, error) {
	if s.record.Stage == publication.StageTerminal && s.record.Terminal != nil {
		if *s.record.Terminal != terminal {
			return publication.Record{}, errors.New("conflicting terminal")
		}
		return s.record, nil
	}
	if s.record != prior {
		return publication.Record{}, errors.New("stale terminal")
	}
	s.record.Stage, s.record.Terminal = publication.StageTerminal, &terminal
	if terminal.NativeReceipt != "" {
		s.record.NativeReceipt = terminal.NativeReceipt
	}
	return s.record, nil
}

func (s *memoryPublicationStore) Recover(_ context.Context, subject implreview.ReviewSubject) (publication.Record, bool, error) {
	record := s.record
	found := record.CreateArmed && record.Binding.ReviewObligationID == subject.ReviewObligationID &&
		record.Binding.ProposalID == subject.ProposalID && record.Binding.ReviewedRevision == subject.ReviewedRevision &&
		(record.Stage != publication.StageTerminal || record.Terminal == nil || record.Terminal.Status != implreview.PublicationFailed)
	return record, found, nil
}

type memoryRecordStore struct {
	records map[string]ReviewRecord
	creates int
}

type publicationStoreOnly struct{ publication.Store }

type publicationPreflightStore struct {
	publication.Store
	err error
}

func (s publicationPreflightStore) Preflight(context.Context) error { return s.err }

type recordStoreStub struct {
	preflightErr error
	exact        func(string) (ReviewRecord, error)
}

func (s *recordStoreStub) Preflight(context.Context, string) error { return s.preflightErr }
func (*recordStoreStub) CreateIfAbsent(context.Context, ReviewRecord) (string, error) {
	return "", errors.New("create unavailable")
}
func (s *recordStoreStub) ExactRead(_ context.Context, receipt string) (ReviewRecord, error) {
	if s.exact != nil {
		return s.exact(receipt)
	}
	return ReviewRecord{}, errors.New("record absent")
}

func (*memoryRecordStore) Preflight(context.Context, string) error { return nil }

func (s *memoryRecordStore) CreateIfAbsent(_ context.Context, record ReviewRecord) (string, error) {
	receipt := receiptFor(record.Repository, record.Proposal, record.IdempotencyID)
	if existing, ok := s.records[receipt]; ok {
		if !sameInput(existing, record) {
			return "", errors.New("conflicting create")
		}
		return receipt, nil
	}
	s.creates++
	s.records[receipt] = record
	return receipt, nil
}

func (s *memoryRecordStore) ExactRead(_ context.Context, receipt string) (ReviewRecord, error) {
	record, ok := s.records[receipt]
	if !ok {
		return ReviewRecord{}, errors.New("record absent")
	}
	return record, nil
}

func carinaTestBinding() implreview.VerifiedPublicationBinding {
	return implreview.VerifiedPublicationBinding{PublicationBinding: implreview.PublicationBinding{
		ReviewObligationID: "obligation-1", ReviewProcessID: "process-1", ProposalID: "carina:o/r#7",
		ReviewedRevision: "abc123", AttemptProofRef: "proof-1",
	}, ProofEvidenceRef: "evidence:attempt:1"}
}

func carinaTestHandoff(t *testing.T) implreview.AggregateHandoff {
	t.Helper()
	binding := carinaTestBinding()
	report := &implreport.Report{SchemaVersion: implreport.SchemaVersion, ProposalRef: binding.ProposalID, ReviewID: binding.ReviewProcessID,
		Band: implreport.BandStandard, Verdict: implreport.VerdictApprove,
		Rounds: []implreport.Round{{ReviewedRevision: binding.ReviewedRevision, ContextID: "context-1", Findings: []implreport.Finding{}}}}
	return implreview.AggregateHandoff{Status: implreview.HandoffCompleted, DiagnosticDigest: "sha256:aggregate",
		Aggregate: &implreview.CompletedAggregate{ProcessID: binding.ReviewProcessID, ProposalID: binding.ProposalID,
			ReviewedRevision: binding.ReviewedRevision, Report: report}}
}

func newCarinaTestHost(t *testing.T, state *memoryPublicationStore, records *memoryRecordStore) *HostAdapter {
	t.Helper()
	adapter, err := NewHostAdapter("o/r", state, records)
	if err != nil {
		t.Fatal(err)
	}
	adapter.readAttempts = 1
	adapter.readBackoff = 0
	adapter.sleep = func(context.Context, time.Duration) error { return nil }
	return adapter
}

func TestHostAdapterPublishesAndReplaysOneExactCarinaRecord(t *testing.T) {
	state := &memoryPublicationStore{}
	records := &memoryRecordStore{records: map[string]ReviewRecord{}}
	adapter := newCarinaTestHost(t, state, records)

	terminal, err := adapter.Complete(context.Background(), carinaTestBinding(), carinaTestHandoff(t))
	if err != nil {
		t.Fatal(err)
	}
	if terminal.Status != implreview.PublicationVerified || terminal.Medium != string(persist.MediumCarina) || terminal.NativeReceipt == "" || records.creates != 1 {
		t.Fatalf("terminal=%+v creates=%d", terminal, records.creates)
	}
	replayed, err := adapter.Complete(context.Background(), carinaTestBinding(), carinaTestHandoff(t))
	if err != nil || replayed != terminal || records.creates != 1 {
		t.Fatalf("replay=%+v err=%v creates=%d", replayed, err, records.creates)
	}
}

func TestHostAdapterRecoversArmedCarinaCreateByDeterministicReceipt(t *testing.T) {
	state := &memoryPublicationStore{}
	records := &memoryRecordStore{records: map[string]ReviewRecord{}}
	adapter := newCarinaTestHost(t, state, records)
	binding, handoff := carinaTestBinding(), carinaTestHandoff(t)
	body, err := persist.ComposeArtifact(handoff.Aggregate.Report)
	if err != nil {
		t.Fatal(err)
	}
	admission := publication.Admission{Binding: binding, HandoffDigest: handoff.DiagnosticDigest,
		ArtifactBodyDigest: digestBytes([]byte(body)), Medium: string(persist.MediumCarina)}
	record, err := state.Admit(context.Background(), admission)
	if err != nil {
		t.Fatal(err)
	}
	record, _, err = state.Arm(context.Background(), record)
	if err != nil {
		t.Fatal(err)
	}
	proposal, _ := proposalNumber(binding.ProposalID, "o/r")
	receipt, err := records.CreateIfAbsent(context.Background(), ReviewRecord{
		Repository: "o/r", Proposal: proposal, IdempotencyID: record.IdempotencyID,
		PublicationAttemptID: record.PublicationAttemptID, ReviewObligationID: binding.ReviewObligationID,
		ReviewProcessID: binding.ReviewProcessID, ProposalID: binding.ProposalID, ReviewedRevision: binding.ReviewedRevision,
		ArtifactDigest: admission.ArtifactBodyDigest, ArtifactBody: body,
	})
	if err != nil {
		t.Fatal(err)
	}

	recovered, found, err := adapter.RecoverPublication(context.Background(), implreview.ReviewSubject{
		ReviewObligationID: binding.ReviewObligationID, ProposalID: binding.ProposalID, ReviewedRevision: binding.ReviewedRevision,
	})
	if err != nil || !found || recovered.Terminal.Status != implreview.PublicationVerified || recovered.Terminal.NativeReceipt != receipt {
		t.Fatalf("recovered=%+v found=%v err=%v", recovered, found, err)
	}
	if records.creates != 1 {
		t.Fatalf("recovery issued another create: %d", records.creates)
	}
}

func TestHostAdapterTerminalizesArmedCarinaStateWhenReceiptIsAbsent(t *testing.T) {
	state := &memoryPublicationStore{}
	records := &memoryRecordStore{records: map[string]ReviewRecord{}}
	adapter := newCarinaTestHost(t, state, records)
	binding, handoff := carinaTestBinding(), carinaTestHandoff(t)
	body, _ := persist.ComposeArtifact(handoff.Aggregate.Report)
	record, err := state.Admit(context.Background(), publication.Admission{Binding: binding, HandoffDigest: handoff.DiagnosticDigest,
		ArtifactBodyDigest: digestBytes([]byte(body)), Medium: string(persist.MediumCarina)})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := state.Arm(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	recovered, found, err := adapter.RecoverPublication(context.Background(), implreview.ReviewSubject{
		ReviewObligationID: binding.ReviewObligationID, ProposalID: binding.ProposalID, ReviewedRevision: binding.ReviewedRevision,
	})
	if err != nil || !found || recovered.Terminal.Status != implreview.PublicationFailed || recovered.Terminal.FailureClass != implreview.FailureInfrastructure {
		t.Fatalf("recovered=%+v found=%v err=%v", recovered, found, err)
	}
	if _, found, err := adapter.InspectPublication(context.Background(), implreview.ReviewSubject{
		ReviewObligationID: binding.ReviewObligationID, ProposalID: binding.ProposalID, ReviewedRevision: binding.ReviewedRevision,
	}); err != nil || found {
		t.Fatalf("failed recovery remained blocking: found=%v err=%v", found, err)
	}
}

func TestHostAdapterConstructionAndPreflightFailures(t *testing.T) {
	state := &memoryPublicationStore{}
	records := &memoryRecordStore{records: map[string]ReviewRecord{}}
	for _, tc := range []struct {
		repository string
		state      publication.Store
		records    ReviewRecordStore
	}{
		{state: state, records: records},
		{repository: "o/r", records: records},
		{repository: "o/r", state: state},
	} {
		if _, err := NewHostAdapter(tc.repository, tc.state, tc.records); err == nil {
			t.Fatalf("NewHostAdapter(%q, %T, %T) succeeded", tc.repository, tc.state, tc.records)
		}
	}
	adapter := newCarinaTestHost(t, state, records)
	if err := adapter.Preflight(context.Background(), "other/repo"); err == nil {
		t.Fatal("Preflight accepted another repository")
	}
	badRecords := &recordStoreStub{preflightErr: errors.New("records unavailable")}
	adapter, _ = NewHostAdapter("o/r", state, badRecords)
	if err := adapter.Preflight(context.Background(), "o/r"); err == nil {
		t.Fatal("Preflight ignored record-store failure")
	}
	badState := publicationPreflightStore{Store: state, err: errors.New("state unavailable")}
	adapter, _ = NewHostAdapter("o/r", badState, records)
	if err := adapter.Preflight(context.Background(), "o/r"); err == nil {
		t.Fatal("Preflight ignored publication-state failure")
	}
	adapter = newCarinaTestHost(t, state, records)
	if err := adapter.Preflight(context.Background(), "o/r"); err != nil {
		t.Fatalf("valid Preflight: %v", err)
	}
}

func TestHostAdapterInspectionMapsEveryRecoveryState(t *testing.T) {
	binding := carinaTestBinding()
	subject := implreview.ReviewSubject{ReviewObligationID: binding.ReviewObligationID, ProposalID: binding.ProposalID, ReviewedRevision: binding.ReviewedRevision}
	records := &memoryRecordStore{records: map[string]ReviewRecord{}}
	plainState := publicationStoreOnly{Store: &memoryPublicationStore{}}
	plain, err := NewHostAdapter("o/r", plainState, records)
	if err != nil {
		t.Fatal(err)
	}
	if _, found, err := plain.InspectPublication(context.Background(), subject); err != nil || found {
		t.Fatalf("non-recovering InspectPublication = found %v, %v", found, err)
	}
	if _, found, err := plain.RecoverPublication(context.Background(), subject); err != nil || found {
		t.Fatalf("non-recovering RecoverPublication = found %v, %v", found, err)
	}

	for _, tc := range []struct {
		stage publication.Stage
		want  implreview.PublicationRecoveryState
	}{
		{stage: publication.StageArmed, want: implreview.PublicationRecoveryArmed},
		{stage: publication.StageReceiptRecorded, want: implreview.PublicationRecoveryReceiptRecorded},
		{stage: publication.StageTerminal, want: implreview.PublicationRecoveryTerminal},
	} {
		state := &memoryPublicationStore{record: publication.Record{Admission: publication.Admission{Binding: binding}, CreateArmed: true, Stage: tc.stage}}
		if tc.stage == publication.StageTerminal {
			state.record.Terminal = &implreview.PublicationTerminal{Status: implreview.PublicationVerified}
		}
		adapter := newCarinaTestHost(t, state, records)
		got, found, err := adapter.InspectPublication(context.Background(), subject)
		if err != nil || !found || got.State != tc.want {
			t.Fatalf("stage %q = %+v, found %v, %v", tc.stage, got, found, err)
		}
	}
	state := &memoryPublicationStore{record: publication.Record{Admission: publication.Admission{Binding: binding}, CreateArmed: true, Stage: publication.StageAccepted}}
	adapter := newCarinaTestHost(t, state, records)
	if _, _, err := adapter.InspectPublication(context.Background(), subject); err == nil {
		t.Fatal("InspectPublication accepted an unexpected state")
	}
}

func TestHostAdapterRecoveryAndReadErrorBranches(t *testing.T) {
	binding := carinaTestBinding()
	records := &recordStoreStub{}
	state := &memoryPublicationStore{}
	adapter, err := NewHostAdapter("o/r", state, records)
	if err != nil {
		t.Fatal(err)
	}
	adapter.readAttempts = 1
	if _, err := adapter.Read(context.Background(), persist.Ref{}); err == nil {
		t.Fatal("Read accepted an empty reference")
	}
	if _, err := adapter.Read(context.Background(), persist.Ref{Medium: persist.MediumCarina, Locator: "missing"}); err == nil {
		t.Fatal("Read accepted a missing Carina receipt")
	}
	if _, err := adapter.ExactRead(context.Background(), "missing"); err == nil {
		t.Fatal("ExactRead accepted a missing receipt")
	}
	records.exact = func(string) (ReviewRecord, error) { return ReviewRecord{ArtifactBody: "not a review"}, nil }
	if _, err := adapter.ExactRead(context.Background(), "malformed"); err == nil {
		t.Fatal("ExactRead accepted a malformed artifact")
	}

	terminalRecord := publication.Record{Admission: publication.Admission{Binding: binding}, Stage: publication.StageTerminal}
	if _, err := adapter.recoverRecord(context.Background(), terminalRecord); err == nil {
		t.Fatal("recovered an empty terminal")
	}
	failed := implreview.PublicationTerminal{Status: implreview.PublicationFailed}
	terminalRecord.Terminal = &failed
	if got, err := adapter.recoverRecord(context.Background(), terminalRecord); err != nil || got != failed {
		t.Fatalf("failed terminal recovery = %+v, %v", got, err)
	}
	if _, err := adapter.recoverRecord(context.Background(), publication.Record{Stage: publication.StageAccepted}); err == nil {
		t.Fatal("recovered an unexpected stage")
	}

	badProposal := publication.Record{Admission: publication.Admission{Binding: binding}, Stage: publication.StageArmed, PublicationAttemptID: "p1", IdempotencyID: "i1"}
	badProposal.Binding.ProposalID = "github:o/r#7"
	state.record = badProposal
	if got, err := adapter.recoverRecord(context.Background(), badProposal); err != nil || got.Status != implreview.PublicationFailed {
		t.Fatalf("bad proposal recovery = %+v, %v", got, err)
	}
}

func TestCompleteRejectsBindingAndTerminalizesIncompleteHandoff(t *testing.T) {
	state := &memoryPublicationStore{}
	records := &memoryRecordStore{records: map[string]ReviewRecord{}}
	adapter := newCarinaTestHost(t, state, records)
	if _, err := adapter.Complete(context.Background(), implreview.VerifiedPublicationBinding{}, implreview.AggregateHandoff{}); err == nil {
		t.Fatal("Complete accepted an empty verified binding")
	}
	terminal, err := adapter.Complete(context.Background(), carinaTestBinding(), implreview.AggregateHandoff{})
	if err != nil {
		t.Fatal(err)
	}
	if terminal.Status != implreview.PublicationFailed || terminal.FailureClass != implreview.FailureArtifactUnverified {
		t.Fatalf("incomplete handoff terminal = %+v", terminal)
	}
}

func TestExactReadRetryHelpersAndValueHelpers(t *testing.T) {
	state := &memoryPublicationStore{}
	records := &recordStoreStub{exact: func(string) (ReviewRecord, error) { return ReviewRecord{}, errors.New("transient") }}
	adapter, err := NewHostAdapter("o/r", state, records)
	if err != nil {
		t.Fatal(err)
	}
	adapter.readAttempts = 2
	adapter.readBackoff = 0
	adapter.sleep = func(context.Context, time.Duration) error { return context.Canceled }
	_, err = adapter.exactReadRecord(context.Background(), "receipt-1")
	var transport *exactReadTransportError
	if !errors.As(err, &transport) || !errors.Is(err, context.Canceled) || transport.Error() == "" {
		t.Fatalf("transport error = %T %v", err, err)
	}
	records.exact = func(string) (ReviewRecord, error) { return ReviewRecord{}, invalidRecord(errors.New("invalid")) }
	if _, err := adapter.exactReadRecord(context.Background(), "receipt-2"); err == nil {
		t.Fatal("invalid record was accepted")
	}

	if err := sleepContext(context.Background(), 0); err != nil {
		t.Fatalf("zero sleep: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if !errors.Is(sleepContext(ctx, time.Hour), context.Canceled) {
		t.Fatal("sleepContext ignored cancellation")
	}
	if err := validateBinding(implreview.VerifiedPublicationBinding{}); err == nil {
		t.Fatal("validateBinding accepted an empty binding")
	}
	for _, proposal := range []string{"github:o/r#7", "carina:o/r#0", "carina:o/r#bad"} {
		if _, err := proposalNumber(proposal, "o/r"); err == nil {
			t.Fatalf("proposalNumber(%q) succeeded", proposal)
		}
	}
	if got := digestStrings("a", "b"); got == "" || got != digestStrings("a", "b") {
		t.Fatalf("digestStrings = %q", got)
	}
}
