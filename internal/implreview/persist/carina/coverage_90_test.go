package carina

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/implreview"
	"github.com/agenticlab-ai/pyxis/internal/implreview/publication"
)

type coverage90PublicationStore struct {
	admit         func(publication.Admission) (publication.Record, error)
	arm           func(publication.Record) (publication.Record, bool, error)
	recordReceipt func(publication.Record, string) (publication.Record, error)
}

func (*coverage90PublicationStore) Preflight(context.Context) error { return nil }

func (s *coverage90PublicationStore) Admit(_ context.Context, in publication.Admission) (publication.Record, error) {
	if s.admit != nil {
		return s.admit(in)
	}
	return publication.Record{Admission: in, PublicationAttemptID: "publication-1", IdempotencyID: "idempotency-1", Stage: publication.StageAccepted}, nil
}

func (s *coverage90PublicationStore) Arm(_ context.Context, in publication.Record) (publication.Record, bool, error) {
	if s.arm != nil {
		return s.arm(in)
	}
	in.Stage = publication.StageArmed
	return in, true, nil
}

func (s *coverage90PublicationStore) RecordReceipt(_ context.Context, in publication.Record, receipt string) (publication.Record, error) {
	if s.recordReceipt != nil {
		return s.recordReceipt(in, receipt)
	}
	in.Stage, in.NativeReceipt = publication.StageReceiptRecorded, receipt
	return in, nil
}

func (*coverage90PublicationStore) Finish(_ context.Context, in publication.Record, terminal implreview.PublicationTerminal) (publication.Record, error) {
	in.Stage, in.Terminal = publication.StageTerminal, &terminal
	return in, nil
}

func TestCoverage90CompleteStateBoundaries(t *testing.T) {
	binding := carinaTestBinding()
	handoff := carinaTestHandoff(t)
	records := &memoryRecordStore{records: map[string]ReviewRecord{}}

	adapter, err := NewHostAdapter("o/r", &coverage90PublicationStore{}, records)
	if err != nil {
		t.Fatal(err)
	}

	want := errors.New("admit failed")
	state := &coverage90PublicationStore{admit: func(publication.Admission) (publication.Record, error) {
		return publication.Record{}, want
	}}
	adapter, _ = NewHostAdapter("o/r", state, records)
	if _, err := adapter.Complete(t.Context(), binding, handoff); !errors.Is(err, want) {
		t.Fatalf("admit error = %v", err)
	}

	failed := implreview.PublicationTerminal{Status: implreview.PublicationFailed}
	state = &coverage90PublicationStore{admit: func(in publication.Admission) (publication.Record, error) {
		other := in.Binding
		other.ReviewProcessID = "other-process"
		return publication.Record{Admission: publication.Admission{Binding: other}, Stage: publication.StageTerminal, Terminal: &failed}, nil
	}}
	adapter, _ = NewHostAdapter("o/r", state, records)
	if terminal, err := adapter.Complete(t.Context(), binding, handoff); err != nil || terminal.Status != implreview.PublicationFailed {
		t.Fatalf("mismatched replay = %+v, %v", terminal, err)
	}

	state = &coverage90PublicationStore{admit: func(in publication.Admission) (publication.Record, error) {
		return publication.Record{Admission: in, Stage: publication.StageTerminal, Terminal: &failed}, nil
	}}
	adapter, _ = NewHostAdapter("o/r", state, records)
	if terminal, err := adapter.Complete(t.Context(), binding, handoff); err != nil || terminal.Status != implreview.PublicationFailed {
		t.Fatalf("terminal replay = %+v, %v", terminal, err)
	}

	armErr := errors.New("arm failed")
	state = &coverage90PublicationStore{arm: func(in publication.Record) (publication.Record, bool, error) {
		return in, false, armErr
	}}
	adapter, _ = NewHostAdapter("o/r", state, records)
	if _, err := adapter.Complete(t.Context(), binding, handoff); !errors.Is(err, armErr) {
		t.Fatalf("arm error = %v", err)
	}

	state = &coverage90PublicationStore{arm: func(in publication.Record) (publication.Record, bool, error) {
		return in, false, nil
	}}
	adapter, _ = NewHostAdapter("o/r", state, records)
	if _, err := adapter.Complete(t.Context(), binding, handoff); err == nil {
		t.Fatal("non-owner publication fence was accepted")
	}

	state = &coverage90PublicationStore{admit: func(in publication.Admission) (publication.Record, error) {
		return publication.Record{Admission: in, Stage: publication.Stage("unknown")}, nil
	}}
	adapter, _ = NewHostAdapter("o/r", state, records)
	if _, err := adapter.Complete(t.Context(), binding, handoff); err == nil {
		t.Fatal("unknown publication stage was accepted")
	}

	adapter, _ = NewHostAdapter("o/r", &coverage90PublicationStore{}, &recordStoreStub{})
	if terminal, err := adapter.Complete(t.Context(), binding, handoff); err != nil || terminal.FailureClass != implreview.FailureInfrastructure {
		t.Fatalf("record create failure = %+v, %v", terminal, err)
	}

	receiptErr := errors.New("receipt failed")
	state = &coverage90PublicationStore{recordReceipt: func(in publication.Record, _ string) (publication.Record, error) {
		return in, receiptErr
	}}
	adapter, _ = NewHostAdapter("o/r", state, records)
	if _, err := adapter.Complete(t.Context(), binding, handoff); !errors.Is(err, receiptErr) {
		t.Fatalf("record receipt error = %v", err)
	}
}

func TestCoverage90FileRecordExistingReadAndSchemaBoundaries(t *testing.T) {
	root := t.TempDir()
	store, err := NewFileRecordStore(root)
	if err != nil {
		t.Fatal(err)
	}
	record := carinaRecord()
	receipt := receiptFor(record.Repository, record.Proposal, record.IdempotencyID)
	path, err := store.pathForReceipt(receipt)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(path, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateIfAbsent(t.Context(), record); err == nil {
		t.Fatal("unreadable existing record path was overwritten")
	}

	record = carinaRecord()
	record.SchemaVersion = "carina-impl-review-record/v99"
	if err := validateInput(record); err == nil {
		t.Fatal("foreign record schema was accepted")
	}
}

func TestCoverage90ExactReadUsesAtLeastOneAttempt(t *testing.T) {
	records := &recordStoreStub{exact: func(string) (ReviewRecord, error) {
		return ReviewRecord{Repository: "o/r"}, nil
	}}
	adapter, err := NewHostAdapter("o/r", &coverage90PublicationStore{}, records)
	if err != nil {
		t.Fatal(err)
	}
	adapter.readAttempts = 0
	if _, err := adapter.exactReadRecord(t.Context(), "receipt"); err != nil {
		t.Fatal(err)
	}
}
