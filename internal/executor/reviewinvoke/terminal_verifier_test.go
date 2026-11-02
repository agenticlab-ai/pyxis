package reviewinvoke

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/reviewobligation"
	"github.com/agenticlab-ai/pyxis/internal/executor/worker"
	"github.com/agenticlab-ai/pyxis/internal/implreview"
	publicationpkg "github.com/agenticlab-ai/pyxis/internal/implreview/publication"
)

type publicationRecordReader map[string]publicationpkg.Record

func (r publicationRecordReader) Record(_ context.Context, id string) (publicationpkg.Record, bool, error) {
	record, ok := r[id]
	return record, ok, nil
}

func terminalRecordReader(terminal implreview.PublicationTerminal) publicationRecordReader {
	return publicationRecordReader{terminal.PublicationAttemptID: {
		PublicationAttemptID: terminal.PublicationAttemptID,
		Stage:                publicationpkg.StageTerminal,
		Terminal:             &terminal,
	}}
}

type proofAliasStore struct{ reviewobligation.Store }

func (s proofAliasStore) Attempt(ctx context.Context, obligationID, attemptID string) (reviewobligation.Attempt, string, bool, error) {
	attempt, _, found, err := s.Store.Attempt(ctx, obligationID, attemptID)
	return attempt, attempt.AttemptProofRef, found, err
}

func TestNewStoreTerminalVerifierRejectsNilDependencies(t *testing.T) {
	if _, err := NewStoreTerminalVerifier(nil, publicationRecordReader{}); err == nil {
		t.Fatal("accepted nil review state")
	}
	store, err := reviewobligation.NewFileStore(filepath.Join(t.TempDir(), "review.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewStoreTerminalVerifier(store, nil); err == nil {
		t.Fatal("accepted nil publication state")
	}
}

func TestStoreTerminalVerifierAuthenticatesProtectedAttemptProof(t *testing.T) {
	store, err := reviewobligation.NewFileStore(filepath.Join(t.TempDir(), "review.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	o := reviewobligation.Obligation{SchemaVersion: reviewobligation.Schema, ID: "obligation-1", Repository: "o/r", TaskID: "T1", ProposalRef: "github:o/r#7", Revision: "abc123", Harness: "codex"}
	if err := store.Append(context.Background(), o); err != nil {
		t.Fatal(err)
	}
	if err := store.CompleteHandoff(context.Background(), o.ID); err != nil {
		t.Fatal(err)
	}
	lease, err := store.Begin(context.Background(), reviewobligation.Attempt{SchemaVersion: reviewobligation.AttemptSchema, ObligationID: o.ID, AttemptID: "process-1", RunID: "run-1", AttemptProofRef: "proof-1"})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = lease.Close() }() // release is best-effort; the operation result is already decided
	verifier, err := NewStoreTerminalVerifier(store, publicationRecordReader{})
	if err != nil {
		t.Fatal(err)
	}
	binding, err := verifier.VerifyAttemptProof(context.Background(), o, "process-1", "proof-1")
	if err != nil {
		t.Fatal(err)
	}
	if binding.ReviewObligationID != o.ID || binding.ReviewProcessID != "process-1" || binding.ProposalID != o.ProposalRef || binding.ReviewedRevision != o.Revision || binding.AttemptProofRef != "proof-1" || binding.ProofEvidenceRef == "" || binding.ProofEvidenceRef == binding.AttemptProofRef {
		t.Fatalf("verified binding = %+v", binding)
	}
	for _, mismatch := range []struct{ process, proof string }{{"other-process", "proof-1"}, {"process-1", "other-proof"}} {
		if _, err := verifier.VerifyAttemptProof(context.Background(), o, mismatch.process, mismatch.proof); err == nil {
			t.Fatalf("accepted mismatched process/proof %q/%q", mismatch.process, mismatch.proof)
		}
	}
	aliasVerifier, err := NewStoreTerminalVerifier(proofAliasStore{Store: store}, publicationRecordReader{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := aliasVerifier.VerifyAttemptProof(context.Background(), o, "process-1", "proof-1"); err == nil {
		t.Fatal("accepted a proof reference that aliases its protected evidence locator")
	}
}

func verifiedPublicationPair() (implreview.PublicationTerminal, worker.ReviewPublicationTerminal) {
	publication := implreview.PublicationTerminal{ReviewProcessID: "process-1", ReviewObligationID: "obligation-1",
		PublicationAttemptID: "publication-1", ProposalID: "github:o/r#7", ReviewedRevision: "abc123",
		Medium: "github", IdempotencyID: "idempotency-1", Status: implreview.PublicationVerified,
		ArtifactRef: "sha256:artifact", NativeReceipt: "github-comment:R_test:7", AttemptProofRef: "proof-1"}
	publication.EVT13Ref, _ = publicationpkg.TerminalRef(publication)
	host := worker.ReviewPublicationTerminal{ReviewAttemptID: "process-1", ReviewObligationID: "obligation-1",
		ProposalRef: "github:o/r#7", Revision: "abc123", Status: "verified", ArtifactRef: "sha256:artifact",
		NativeReceipt: "github-comment:R_test:7", AttemptProofRef: "proof-1"}
	return publication, host
}

func TestStoreTerminalVerifierAcceptsOnlyFieldIdenticalEVT13EVT17Pair(t *testing.T) {
	publication, host := verifiedPublicationPair()
	verifier := &StoreTerminalVerifier{publications: terminalRecordReader(publication)}
	if err := verifier.VerifyPair(context.Background(), publication, host); err != nil {
		t.Fatalf("valid pair: %v", err)
	}
	cases := []struct {
		name   string
		mutate func(*implreview.PublicationTerminal, *worker.ReviewPublicationTerminal)
	}{
		{name: "process", mutate: func(_ *implreview.PublicationTerminal, h *worker.ReviewPublicationTerminal) {
			h.ReviewAttemptID = "other"
		}},
		{name: "obligation", mutate: func(_ *implreview.PublicationTerminal, h *worker.ReviewPublicationTerminal) {
			h.ReviewObligationID = "other"
		}},
		{name: "proposal", mutate: func(_ *implreview.PublicationTerminal, h *worker.ReviewPublicationTerminal) {
			h.ProposalRef = "github:o/r#8"
		}},
		{name: "revision", mutate: func(_ *implreview.PublicationTerminal, h *worker.ReviewPublicationTerminal) {
			h.Revision = "moving-head"
		}},
		{name: "artifact", mutate: func(_ *implreview.PublicationTerminal, h *worker.ReviewPublicationTerminal) {
			h.ArtifactRef = "sha256:other"
		}},
		{name: "receipt", mutate: func(_ *implreview.PublicationTerminal, h *worker.ReviewPublicationTerminal) {
			h.NativeReceipt = "github-comment:R_test:8"
		}},
		{name: "proof", mutate: func(_ *implreview.PublicationTerminal, h *worker.ReviewPublicationTerminal) {
			h.AttemptProofRef = "proof-2"
		}},
		{name: "status", mutate: func(_ *implreview.PublicationTerminal, h *worker.ReviewPublicationTerminal) { h.Status = "failed" }},
		{name: "receipt proof alias", mutate: func(p *implreview.PublicationTerminal, h *worker.ReviewPublicationTerminal) {
			p.NativeReceipt, h.NativeReceipt = "proof-1", "proof-1"
		}},
		{name: "missing evt13 evidence", mutate: func(p *implreview.PublicationTerminal, _ *worker.ReviewPublicationTerminal) { p.EVT13Ref = "" }},
		{name: "verified escalated", mutate: func(_ *implreview.PublicationTerminal, h *worker.ReviewPublicationTerminal) { h.Escalated = true }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			publication, host := verifiedPublicationPair()
			verifier := &StoreTerminalVerifier{publications: terminalRecordReader(publication)}
			tc.mutate(&publication, &host)
			if err := verifier.VerifyPair(context.Background(), publication, host); err == nil {
				t.Fatalf("mismatched pair accepted: publication=%+v host=%+v", publication, host)
			}
		})
	}
}

func TestStoreTerminalVerifierRejectsMissingOrConflictingProtectedEVT13(t *testing.T) {
	publication, host := verifiedPublicationPair()
	for _, tc := range []struct {
		name   string
		reader publicationRecordReader
	}{
		{name: "missing", reader: publicationRecordReader{}},
		{name: "conflicting", reader: terminalRecordReader(func() implreview.PublicationTerminal {
			other := publication
			other.NativeReceipt = "github-comment:R_test:other"
			other.EVT13Ref, _ = publicationpkg.TerminalRef(other)
			return other
		}())},
	} {
		t.Run(tc.name, func(t *testing.T) {
			verifier := &StoreTerminalVerifier{publications: tc.reader}
			if err := verifier.VerifyPair(context.Background(), publication, host); err == nil {
				t.Fatal("unprotected or conflicting EVT-13 was accepted")
			}
		})
	}
	failingReader := publicationReaderFunc(func(context.Context, string) (publicationpkg.Record, bool, error) {
		return publicationpkg.Record{}, false, errors.New("evidence unavailable")
	})
	if err := (&StoreTerminalVerifier{publications: failingReader}).VerifyPair(context.Background(), publication, host); err == nil {
		t.Fatal("publication evidence read failure was accepted")
	}
}

type publicationReaderFunc func(context.Context, string) (publicationpkg.Record, bool, error)

func (f publicationReaderFunc) Record(ctx context.Context, id string) (publicationpkg.Record, bool, error) {
	return f(ctx, id)
}

func TestStoreTerminalVerifierRequiresIdenticalFailedClassReasonAndEscalation(t *testing.T) {
	publication, host := verifiedPublicationPair()
	publication.Status, publication.ArtifactRef, publication.NativeReceipt, publication.AttemptProofRef = implreview.PublicationFailed, "", "", ""
	publication.FailureClass, publication.Reason = implreview.FailureInfrastructure, "host write failed"
	publication.EVT13Ref, _ = publicationpkg.TerminalRef(publication)
	host.Status, host.ArtifactRef, host.NativeReceipt, host.AttemptProofRef = "failed", "", "", ""
	host.FailureClass, host.Reason, host.Escalated = "infrastructure", "host write failed", true
	verifier := &StoreTerminalVerifier{publications: terminalRecordReader(publication)}
	if err := verifier.VerifyPair(context.Background(), publication, host); err != nil {
		t.Fatalf("valid failed pair: %v", err)
	}
	for _, mutate := range []func(*worker.ReviewPublicationTerminal){
		func(h *worker.ReviewPublicationTerminal) { h.FailureClass = "artifact_unverified" },
		func(h *worker.ReviewPublicationTerminal) { h.Reason = "different" },
		func(h *worker.ReviewPublicationTerminal) { h.Escalated = false },
	} {
		bad := host
		mutate(&bad)
		if err := verifier.VerifyPair(context.Background(), publication, bad); err == nil {
			t.Fatalf("invalid failed pair accepted: %+v", bad)
		}
	}
}
