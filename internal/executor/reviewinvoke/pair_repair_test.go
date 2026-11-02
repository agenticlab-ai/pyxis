package reviewinvoke

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"path/filepath"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/reviewobligation"
	"github.com/agenticlab-ai/pyxis/internal/executor/worker"
	"github.com/agenticlab-ai/pyxis/internal/implreview"
	"github.com/agenticlab-ai/pyxis/internal/implreview/persist"
	"github.com/agenticlab-ai/pyxis/internal/implreview/publication"
	implreport "github.com/agenticlab-ai/pyxis/internal/implreview/report"
)

type pairPersistence struct {
	report *implreport.Report
	err    error
}

type pairReadOnlyPersistence struct{ pairPersistence }

func (p *pairReadOnlyPersistence) PreflightRead(context.Context, string) error { return p.err }

func (p *pairPersistence) Preflight(context.Context, string) error { return p.err }
func (p *pairPersistence) Complete(context.Context, implreview.VerifiedPublicationBinding, implreview.AggregateHandoff) (implreview.PublicationTerminal, error) {
	return implreview.PublicationTerminal{}, errors.New("pair repair must not publish")
}
func (p *pairPersistence) ExactRead(context.Context, string) (*implreport.Report, error) {
	return p.report, p.err
}

func pairRepairFixture(t *testing.T) (*PairRepairer, *reviewobligation.FileStore, reviewobligation.Obligation, reviewobligation.PublicationPair, reviewobligation.AttemptLease) {
	t.Helper()
	ctx := context.Background()
	store, err := reviewobligation.NewFileStore(filepath.Join(t.TempDir(), "review.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	o := reviewobligation.Obligation{SchemaVersion: reviewobligation.Schema, ID: "obligation-1", Repository: "o/r", Scope: "feature/001", TaskID: "T1", ProposalRef: "github:o/r#7", Revision: "abc123", Harness: "codex"}
	if err := store.Append(ctx, o); err != nil {
		t.Fatal(err)
	}
	if err := store.CompleteHandoff(ctx, o.ID); err != nil {
		t.Fatal(err)
	}
	attempt := reviewobligation.Attempt{SchemaVersion: reviewobligation.AttemptSchema, ObligationID: o.ID, AttemptID: "process-1", RunID: "run-1", AttemptProofRef: "proof-1"}
	lease, err := store.Begin(ctx, attempt)
	if err != nil {
		t.Fatal(err)
	}
	reviewReport := &implreport.Report{SchemaVersion: implreport.SchemaVersion, ProposalRef: o.ProposalRef, ReviewID: attempt.AttemptID,
		Band: implreport.BandStandard, Verdict: implreport.VerdictApprove,
		Rounds: []implreport.Round{{ReviewedRevision: o.Revision, ContextID: "ctx-1", Provenance: implreport.Provenance{Model: "review-model", PromptsVersion: "v1", CalVersion: "v1"}, Findings: []implreport.Finding{}}}}
	body, err := persist.ComposeArtifact(reviewReport)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte(body))
	artifactRef := "sha256:" + hex.EncodeToString(digest[:])
	terminal := implreview.PublicationTerminal{ReviewProcessID: attempt.AttemptID, ReviewObligationID: o.ID,
		PublicationAttemptID: "publication-1", ProposalID: o.ProposalRef, ReviewedRevision: o.Revision,
		Medium: "github", IdempotencyID: "idempotency-1", Status: implreview.PublicationVerified,
		ArtifactRef: artifactRef, NativeReceipt: "github-comment:R_test:7", AttemptProofRef: attempt.AttemptProofRef}
	terminal.EVT13Ref, err = publication.TerminalRef(terminal)
	if err != nil {
		t.Fatal(err)
	}
	host := worker.ReviewPublicationTerminal{ReviewAttemptID: attempt.AttemptID, ReviewObligationID: o.ID,
		ProposalRef: o.ProposalRef, Revision: o.Revision, Status: "verified", ArtifactRef: artifactRef,
		NativeReceipt: terminal.NativeReceipt, AttemptProofRef: attempt.AttemptProofRef}
	evt17Ref, err := hostedTerminalRef(host)
	if err != nil {
		t.Fatal(err)
	}
	pair := reviewobligation.PublicationPair{SchemaVersion: reviewobligation.PublicationPairSchema,
		ObligationID: o.ID, AttemptID: attempt.AttemptID, PublicationAttemptID: terminal.PublicationAttemptID,
		ProposalRef: o.ProposalRef, Revision: o.Revision, Medium: terminal.Medium, IdempotencyID: terminal.IdempotencyID,
		ArtifactRef: artifactRef, NativeReceipt: terminal.NativeReceipt, AttemptProofRef: attempt.AttemptProofRef,
		EVT13Ref: terminal.EVT13Ref, EVT17Ref: evt17Ref}
	if err := store.RecordPair(ctx, pair); err != nil {
		t.Fatal(err)
	}
	verifier, err := NewStoreTerminalVerifier(store, terminalRecordReader(terminal))
	if err != nil {
		t.Fatal(err)
	}
	repairer, err := NewPairRepairer(store, &pairPersistence{report: reviewReport}, verifier)
	if err != nil {
		t.Fatal(err)
	}
	return repairer, store, o, pair, lease
}

func TestPairRepairerReauthenticatesReceiptProofAndBothTerminalsBeforeClosing(t *testing.T) {
	repairer, store, obligation, pair, lease := pairRepairFixture(t)
	defer func() { _ = lease.Close() }() // release is best-effort; the operation result is already decided
	result, err := repairer.Repair(context.Background(), obligation, pair, "round-2")
	if err != nil {
		t.Fatal(err)
	}
	if result.Failed() || result.ImplReviewTerminal == nil || result.Publication == nil {
		t.Fatalf("repaired result = %+v", result)
	}
	state, ok, err := store.State(context.Background(), obligation.ID)
	if err != nil || !ok || state.Status != "verified" {
		t.Fatalf("repaired state = %+v, found=%v err=%v", state, ok, err)
	}
}

func TestPairRepairerRejectsCircularOrTamperedTerminalEvidence(t *testing.T) {
	repairer, store, obligation, pair, lease := pairRepairFixture(t)
	defer func() { _ = lease.Close() }() // release is best-effort; the operation result is already decided
	pair.EVT17Ref = "evidence:evt17:tampered"
	if _, err := repairer.Repair(context.Background(), obligation, pair, "round-2"); err == nil {
		t.Fatal("tampered EVT-17 evidence was accepted")
	}
	state, ok, err := store.State(context.Background(), obligation.ID)
	if err != nil || !ok || state.Status != "pair_verified" {
		t.Fatalf("tampered pair changed lifecycle state = %+v, found=%v err=%v", state, ok, err)
	}
}

func TestPairRepairerPreflightAndConstructorValidation(t *testing.T) {
	if _, err := NewPairRepairer(nil, nil, nil); err == nil {
		t.Fatal("NewPairRepairer accepted missing dependencies")
	}
	repairer, _, _, _, lease := pairRepairFixture(t)
	defer func() { _ = lease.Close() }() // release is best-effort; the operation result is already decided
	if err := repairer.Preflight(t.Context(), "o/r"); err != nil {
		t.Fatalf("Preflight: %v", err)
	}
	repairer.persistence = &pairPersistence{err: errors.New("persistence unavailable")}
	if err := repairer.Preflight(t.Context(), "o/r"); err == nil {
		t.Fatal("Preflight swallowed persistence error")
	}
	repairer.persistence = &pairReadOnlyPersistence{pairPersistence{err: errors.New("read unavailable")}}
	if err := repairer.Preflight(t.Context(), "o/r"); err == nil {
		t.Fatal("PreflightRead failure was swallowed")
	}
}

func TestPairRepairerRejectsMismatchedProofAndReceipt(t *testing.T) {
	t.Run("obligation mismatch", func(t *testing.T) {
		repairer, _, obligation, pair, lease := pairRepairFixture(t)
		defer func() { _ = lease.Close() }() // release is best-effort; the operation result is already decided
		pair.ObligationID = "other"
		if _, err := repairer.Repair(t.Context(), obligation, pair, "round"); err == nil {
			t.Fatal("mismatched obligation was accepted")
		}
	})
	t.Run("attempt proof mismatch", func(t *testing.T) {
		repairer, _, obligation, pair, lease := pairRepairFixture(t)
		defer func() { _ = lease.Close() }() // release is best-effort; the operation result is already decided
		pair.AttemptProofRef = "proof-tampered"
		if _, err := repairer.Repair(t.Context(), obligation, pair, "round"); err == nil {
			t.Fatal("mismatched attempt proof was accepted")
		}
	})
	t.Run("exact read error", func(t *testing.T) {
		repairer, _, obligation, pair, lease := pairRepairFixture(t)
		defer func() { _ = lease.Close() }() // release is best-effort; the operation result is already decided
		repairer.persistence = &pairPersistence{err: errors.New("receipt unavailable")}
		result, err := repairer.Repair(t.Context(), obligation, pair, "round")
		if err == nil || result.FailureClass != worker.ReviewFailureInfrastructure {
			t.Fatalf("exact-read result = %+v, %v", result, err)
		}
	})
	t.Run("receipt mismatch", func(t *testing.T) {
		repairer, _, obligation, pair, lease := pairRepairFixture(t)
		defer func() { _ = lease.Close() }() // release is best-effort; the operation result is already decided
		repairer.persistence = &pairPersistence{report: &implreport.Report{}}
		if _, err := repairer.Repair(t.Context(), obligation, pair, "round"); err == nil {
			t.Fatal("mismatched receipt was accepted")
		}
	})
}
