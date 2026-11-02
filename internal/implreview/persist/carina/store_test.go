package carina

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/implreview"
	"github.com/agenticlab-ai/pyxis/internal/implreview/persist"
	"github.com/agenticlab-ai/pyxis/internal/implreview/publication"
	implreport "github.com/agenticlab-ai/pyxis/internal/implreview/report"
)

func carinaRecord() ReviewRecord {
	return ReviewRecord{Repository: "o/r", Proposal: 7, IdempotencyID: "idempotency-1", PublicationAttemptID: "publication-1",
		ReviewObligationID: "obligation-1", ReviewProcessID: "process-1", ProposalID: "carina:o/r#7", ReviewedRevision: "abc123",
		ArtifactDigest: "sha256:artifact", ArtifactBody: "artifact body"}
}

func TestFileRecordStoreCreatesOnceExactReadsAndRejectsConflict(t *testing.T) {
	store, err := NewFileRecordStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	store.WithClock(func() time.Time { return time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC) })
	ctx := context.Background()
	if err := store.Preflight(ctx, "o/r"); err != nil {
		t.Fatal(err)
	}
	receipt, err := store.CreateIfAbsent(ctx, carinaRecord())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(receipt, "carina-review-record:") || !strings.HasSuffix(receipt, ":idempotency-1") {
		t.Fatalf("receipt = %q", receipt)
	}
	replayed, err := store.CreateIfAbsent(ctx, carinaRecord())
	if err != nil || replayed != receipt {
		t.Fatalf("replay receipt=%q err=%v", replayed, err)
	}
	read, err := store.ExactRead(ctx, receipt)
	if err != nil || read.CreatedAt.IsZero() || read.RecordDigest == "" || read.ArtifactBody != "artifact body" {
		t.Fatalf("exact read = %+v, %v", read, err)
	}
	conflict := carinaRecord()
	conflict.ReviewedRevision = "moving-head"
	if _, err := store.CreateIfAbsent(ctx, conflict); err == nil {
		t.Fatal("conflicting immutable record replay was accepted")
	}
}

func TestFileRecordStoreRejectsTamperAndForeignReceipt(t *testing.T) {
	store, err := NewFileRecordStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := store.CreateIfAbsent(context.Background(), carinaRecord())
	if err != nil {
		t.Fatal(err)
	}
	path, err := store.pathForReceipt(receipt)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data = []byte(strings.Replace(string(data), "artifact body", "forged body", 1))
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ExactRead(context.Background(), receipt); err == nil || !strings.Contains(err.Error(), "digest mismatch") {
		t.Fatalf("tampered exact read error = %v", err)
	}
	if _, err := store.ExactRead(context.Background(), "carina-review-record:..:7:id"); err == nil {
		t.Fatal("invalid encoded repository receipt was accepted")
	}
}

type carinaMemoryPublication struct {
	mu     sync.Mutex
	record publication.Record
}

type flakyReviewRecordStore struct {
	ReviewRecordStore
	failures int
	calls    int
	readErr  error
}

func (s *flakyReviewRecordStore) ExactRead(ctx context.Context, receipt string) (ReviewRecord, error) {
	s.calls++
	if s.readErr != nil {
		return ReviewRecord{}, s.readErr
	}
	if s.failures > 0 {
		s.failures--
		return ReviewRecord{}, errors.New("temporary replica lag")
	}
	return s.ReviewRecordStore.ExactRead(ctx, receipt)
}

func (*carinaMemoryPublication) Preflight(context.Context) error { return nil }
func (s *carinaMemoryPublication) Admit(_ context.Context, admission publication.Admission) (publication.Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.record.PublicationAttemptID != "" {
		return s.record, nil
	}
	s.record = publication.Record{Admission: admission, PublicationAttemptID: "publication-1", IdempotencyID: "idempotency-1", Stage: publication.StageAccepted}
	return s.record, nil
}
func (s *carinaMemoryPublication) Arm(_ context.Context, _ publication.Record) (publication.Record, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.record.Stage != publication.StageAccepted {
		return s.record, false, nil
	}
	s.record.Stage = publication.StageArmed
	return s.record, true, nil
}
func (s *carinaMemoryPublication) RecordReceipt(_ context.Context, _ publication.Record, receipt string) (publication.Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.record.Stage, s.record.NativeReceipt = publication.StageReceiptRecorded, receipt
	return s.record, nil
}
func (s *carinaMemoryPublication) Finish(_ context.Context, _ publication.Record, terminal implreview.PublicationTerminal) (publication.Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.record.Terminal != nil && *s.record.Terminal != terminal {
		return publication.Record{}, errors.New("terminal conflict")
	}
	s.record.Stage, s.record.Terminal = publication.StageTerminal, &terminal
	if terminal.NativeReceipt != "" {
		s.record.NativeReceipt = terminal.NativeReceipt
	}
	return s.record, nil
}

func carinaBinding() implreview.VerifiedPublicationBinding {
	return implreview.VerifiedPublicationBinding{PublicationBinding: implreview.PublicationBinding{
		ReviewObligationID: "obligation-1", ReviewProcessID: "process-1", ProposalID: "carina:o/r#7",
		ReviewedRevision: "abc123", AttemptProofRef: "proof-1",
	}, ProofEvidenceRef: "evidence:attempt:1"}
}

func carinaHandoff(t *testing.T) implreview.AggregateHandoff {
	t.Helper()
	binding := carinaBinding()
	report := &implreport.Report{SchemaVersion: implreport.SchemaVersion, ProposalRef: binding.ProposalID, ReviewID: binding.ReviewProcessID,
		Band: implreport.BandStandard, Verdict: implreport.VerdictApprove,
		Rounds: []implreport.Round{{ReviewedRevision: binding.ReviewedRevision, ContextID: "context-1", Findings: []implreport.Finding{}}}}
	return implreview.AggregateHandoff{Status: implreview.HandoffCompleted, DiagnosticDigest: "sha256:aggregate",
		Aggregate: &implreview.CompletedAggregate{ProcessID: binding.ReviewProcessID, ProposalID: binding.ProposalID,
			ReviewedRevision: binding.ReviewedRevision, Report: report}}
}

func TestCarinaHostPublishesAndExactReadsOneImmutableRecord(t *testing.T) {
	records, err := NewFileRecordStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	state := &carinaMemoryPublication{}
	host, err := NewHostAdapter("o/r", state, records)
	if err != nil {
		t.Fatal(err)
	}
	if err := host.Preflight(context.Background(), "o/r"); err != nil {
		t.Fatal(err)
	}
	terminal, err := host.Complete(context.Background(), carinaBinding(), carinaHandoff(t))
	if err != nil {
		t.Fatal(err)
	}
	if terminal.Status != implreview.PublicationVerified || terminal.Medium != string(persist.MediumCarina) || terminal.NativeReceipt == "" || terminal.ArtifactRef == "" || terminal.AttemptProofRef != "proof-1" {
		t.Fatalf("terminal = %+v", terminal)
	}
	read, err := host.ExactRead(context.Background(), terminal.NativeReceipt)
	if err != nil || read.ReviewID != "process-1" || read.ProposalRef != "carina:o/r#7" {
		t.Fatalf("exact read = %+v, %v", read, err)
	}
	replay, err := host.Complete(context.Background(), carinaBinding(), carinaHandoff(t))
	if err != nil || replay != terminal {
		t.Fatalf("terminal replay = %+v, %v", replay, err)
	}
}

func TestCarinaHostRetriesExactReceiptVisibility(t *testing.T) {
	records, err := NewFileRecordStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	flaky := &flakyReviewRecordStore{ReviewRecordStore: records, failures: 2}
	host, err := NewHostAdapter("o/r", &carinaMemoryPublication{}, flaky)
	if err != nil {
		t.Fatal(err)
	}
	host.readBackoff = 0
	host.sleep = func(context.Context, time.Duration) error { return nil }
	if err := host.Preflight(context.Background(), "o/r"); err != nil {
		t.Fatal(err)
	}
	terminal, err := host.Complete(context.Background(), carinaBinding(), carinaHandoff(t))
	if err != nil || terminal.Status != implreview.PublicationVerified {
		t.Fatalf("terminal=%+v err=%v", terminal, err)
	}
	if flaky.calls != 3 {
		t.Fatalf("exact-read calls=%d, want three bounded attempts", flaky.calls)
	}
}

func TestCarinaHostDoesNotRetryOrMisclassifyInvalidExactReceipt(t *testing.T) {
	records, err := NewFileRecordStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	invalid := &flakyReviewRecordStore{
		ReviewRecordStore: records,
		readErr:           invalidRecord(errors.New("carina review record: record digest mismatch")),
	}
	host, err := NewHostAdapter("o/r", &carinaMemoryPublication{}, invalid)
	if err != nil {
		t.Fatal(err)
	}
	host.readBackoff = 0
	host.sleep = func(context.Context, time.Duration) error { return nil }
	terminal, err := host.Complete(context.Background(), carinaBinding(), carinaHandoff(t))
	if err != nil {
		t.Fatal(err)
	}
	if terminal.Status != implreview.PublicationFailed || terminal.FailureClass != implreview.FailureArtifactUnverified {
		t.Fatalf("terminal=%+v, want one artifact-unverified failure", terminal)
	}
	if invalid.calls != 1 {
		t.Fatalf("exact-read calls=%d, want no retry for deterministic corruption", invalid.calls)
	}
}
