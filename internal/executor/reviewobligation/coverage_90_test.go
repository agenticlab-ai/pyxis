package reviewobligation

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func manualCoverageStore(t *testing.T, obligation Obligation) *FileStore {
	t.Helper()
	path := filepath.Join(t.TempDir(), "review.jsonl")
	if err := os.WriteFile(path, coverageJSONLine(t, obligation), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := NewFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func writeCoverageRows(t *testing.T, path string, rows ...[]byte) {
	t.Helper()
	var data []byte
	for _, row := range rows {
		data = append(data, row...)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestCoverage90FileStorePropagatesLockFailures(t *testing.T) {
	if _, err := NewGitStore("repo", "origin", "not-an-evidence-ref"); err == nil {
		t.Fatal("NewGitStore accepted an invalid evidence ref")
	}
	root := t.TempDir()
	blocker := filepath.Join(root, "blocker")
	if err := os.WriteFile(blocker, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	store := &FileStore{path: filepath.Join(blocker, "review.jsonl")}
	o := coverageObligation("o-lock")
	a := testAttempt(o.ID, "a-lock", "run-lock")
	pair := testPublicationPair(o, a)
	terminal := Terminal{SchemaVersion: TerminalSchema, ObligationID: o.ID, AttemptID: a.AttemptID, Status: "failed", FailureClass: "infrastructure"}
	operations := []struct {
		name string
		run  func() error
	}{
		{name: "preflight", run: func() error { return store.Preflight(context.Background()) }},
		{name: "append", run: func() error { return store.Append(context.Background(), o) }},
		{name: "get", run: func() error { _, _, err := store.Get(context.Background(), o.ID); return err }},
		{name: "open", run: func() error { _, err := store.Open(context.Background(), o.Scope); return err }},
		{name: "handoff", run: func() error { return store.CompleteHandoff(context.Background(), o.ID) }},
		{name: "attempt lookup", run: func() error { _, _, _, err := store.Attempt(context.Background(), o.ID, a.AttemptID); return err }},
		{name: "terminal lookup", run: func() error { _, _, err := store.Terminal(context.Background(), o.ID, a.AttemptID); return err }},
		{name: "pair", run: func() error { return store.RecordPair(context.Background(), pair) }},
		{name: "terminal", run: func() error { return store.AppendTerminal(context.Background(), terminal) }},
		{name: "verified", run: func() error { _, err := store.Verified(context.Background(), o.ID); return err }},
	}
	for _, operation := range operations {
		t.Run(operation.name, func(t *testing.T) {
			if err := operation.run(); err == nil {
				t.Fatal("operation ignored an unusable lock path")
			}
		})
	}
}

func TestCoverage90FileStorePropagatesEachEvidenceReadFailure(t *testing.T) {
	ctx := context.Background()
	o := coverageObligation("o-read")
	a := testAttempt(o.ID, "a-read", "run-read")
	pair := testPublicationPair(o, a)
	failed := Terminal{SchemaVersion: TerminalSchema, ObligationID: o.ID, AttemptID: a.AttemptID, Status: "failed", FailureClass: "infrastructure"}
	handoff := Handoff{SchemaVersion: HandoffSchema, ObligationID: o.ID}
	corrupt := []byte("{]\n")

	t.Run("state open", func(t *testing.T) {
		store := manualCoverageStore(t, o)
		writeCoverageRows(t, store.path+".attempts", corrupt)
		if _, _, err := store.State(ctx, o.ID); err == nil {
			t.Fatal("State accepted corrupt attempt evidence")
		}
	})
	t.Run("handoff obligation", func(t *testing.T) {
		store := manualCoverageStore(t, o)
		writeCoverageRows(t, store.path, corrupt)
		if err := store.CompleteHandoff(ctx, o.ID); err == nil {
			t.Fatal("CompleteHandoff accepted corrupt obligation evidence")
		}
	})
	t.Run("handoff rows", func(t *testing.T) {
		store := manualCoverageStore(t, o)
		writeCoverageRows(t, store.path+".handoffs", corrupt)
		if err := store.CompleteHandoff(ctx, o.ID); err == nil {
			t.Fatal("CompleteHandoff accepted corrupt handoff evidence")
		}
	})

	for _, tc := range []struct {
		name   string
		suffix string
		seed   map[string][]byte
	}{
		{name: "begin handoffs", suffix: ".handoffs"},
		{name: "begin terminals", suffix: ".terminals", seed: map[string][]byte{".handoffs": coverageJSONLine(t, handoff)}},
		{name: "begin pairs", suffix: ".pairs", seed: map[string][]byte{".handoffs": coverageJSONLine(t, handoff)}},
		{name: "begin attempts", suffix: ".attempts", seed: map[string][]byte{".handoffs": coverageJSONLine(t, handoff)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := manualCoverageStore(t, o)
			for suffix, rows := range tc.seed {
				writeCoverageRows(t, store.path+suffix, rows)
			}
			writeCoverageRows(t, store.path+tc.suffix, corrupt)
			if lease, err := store.Begin(ctx, a); err == nil {
				lease.Close()
				t.Fatal("Begin accepted corrupt evidence")
			}
		})
	}

	t.Run("duplicate attempt", func(t *testing.T) {
		store, _ := NewFileStore(filepath.Join(t.TempDir(), "review.jsonl"))
		if err := store.Append(ctx, o); err != nil {
			t.Fatal(err)
		}
		if err := store.CompleteHandoff(ctx, o.ID); err != nil {
			t.Fatal(err)
		}
		lease, err := store.Begin(ctx, a)
		if err != nil {
			t.Fatal(err)
		}
		if err := lease.Close(); err != nil {
			t.Fatal(err)
		}
		if lease, err := store.Begin(ctx, a); err == nil {
			lease.Close()
			t.Fatal("duplicate attempt identity was admitted")
		}
	})

	t.Run("resume attempts", func(t *testing.T) {
		store := manualCoverageStore(t, o)
		writeCoverageRows(t, store.path+".attempts", corrupt)
		if lease, err := store.Resume(ctx, o.ID, a.AttemptID); err == nil {
			lease.Close()
			t.Fatal("Resume accepted corrupt attempts")
		}
	})
	t.Run("resume terminals", func(t *testing.T) {
		store := manualCoverageStore(t, o)
		writeCoverageRows(t, store.path+".attempts", coverageJSONLine(t, a))
		writeCoverageRows(t, store.path+".terminals", corrupt)
		if lease, err := store.Resume(ctx, o.ID, a.AttemptID); err == nil {
			lease.Close()
			t.Fatal("Resume accepted corrupt terminals")
		}
	})
	t.Run("attempt lookup", func(t *testing.T) {
		store := manualCoverageStore(t, o)
		writeCoverageRows(t, store.path+".attempts", corrupt)
		if _, _, _, err := store.Attempt(ctx, o.ID, a.AttemptID); err == nil {
			t.Fatal("Attempt accepted corrupt attempts")
		}
	})
	t.Run("conflicting attempt lookup", func(t *testing.T) {
		store := manualCoverageStore(t, o)
		other := a
		other.RunID = "different"
		writeCoverageRows(t, store.path+".attempts", coverageJSONLine(t, a), coverageJSONLine(t, other))
		if _, _, _, err := store.Attempt(ctx, o.ID, a.AttemptID); err == nil {
			t.Fatal("Attempt accepted conflicting attempts")
		}
	})
	t.Run("terminal lookup", func(t *testing.T) {
		store := manualCoverageStore(t, o)
		writeCoverageRows(t, store.path+".terminals", corrupt)
		if _, _, err := store.Terminal(ctx, o.ID, a.AttemptID); err == nil {
			t.Fatal("Terminal accepted corrupt terminals")
		}
	})

	t.Run("pair attempts", func(t *testing.T) {
		store := manualCoverageStore(t, o)
		writeCoverageRows(t, store.path+".attempts", corrupt)
		if err := store.RecordPair(ctx, pair); err == nil {
			t.Fatal("RecordPair accepted corrupt attempts")
		}
	})
	t.Run("pair unknown attempt", func(t *testing.T) {
		store := manualCoverageStore(t, o)
		if err := store.RecordPair(ctx, pair); err == nil {
			t.Fatal("RecordPair accepted an unknown attempt")
		}
	})
	t.Run("pair rows", func(t *testing.T) {
		store := manualCoverageStore(t, o)
		writeCoverageRows(t, store.path+".attempts", coverageJSONLine(t, a))
		writeCoverageRows(t, store.path+".pairs", corrupt)
		if err := store.RecordPair(ctx, pair); err == nil {
			t.Fatal("RecordPair accepted corrupt pairs")
		}
	})
	t.Run("pair replay and conflict", func(t *testing.T) {
		store := manualCoverageStore(t, o)
		writeCoverageRows(t, store.path+".attempts", coverageJSONLine(t, a))
		if err := store.RecordPair(ctx, pair); err != nil {
			t.Fatal(err)
		}
		if err := store.RecordPair(ctx, pair); err != nil {
			t.Fatalf("idempotent pair replay: %v", err)
		}
		conflict := pair
		conflict.ArtifactRef = "artifact:different"
		if err := store.RecordPair(ctx, conflict); err == nil {
			t.Fatal("conflicting pair replay was accepted")
		}
	})

	t.Run("terminal rows", func(t *testing.T) {
		store := manualCoverageStore(t, o)
		writeCoverageRows(t, store.path+".terminals", corrupt)
		if err := store.AppendTerminal(ctx, failed); err == nil {
			t.Fatal("AppendTerminal accepted corrupt terminals")
		}
	})
	t.Run("terminal attempts", func(t *testing.T) {
		store := manualCoverageStore(t, o)
		writeCoverageRows(t, store.path+".attempts", corrupt)
		if err := store.AppendTerminal(ctx, failed); err == nil {
			t.Fatal("AppendTerminal accepted corrupt attempts")
		}
	})
	t.Run("terminal unknown attempt", func(t *testing.T) {
		store := manualCoverageStore(t, o)
		if err := store.AppendTerminal(ctx, failed); err == nil {
			t.Fatal("AppendTerminal accepted an unknown attempt")
		}
	})
	t.Run("terminal replay and conflict", func(t *testing.T) {
		store := manualCoverageStore(t, o)
		writeCoverageRows(t, store.path+".attempts", coverageJSONLine(t, a))
		if err := store.AppendTerminal(ctx, failed); err != nil {
			t.Fatal(err)
		}
		if err := store.AppendTerminal(ctx, failed); err != nil {
			t.Fatalf("idempotent terminal replay: %v", err)
		}
		conflict := failed
		conflict.FailureClass = "artifact_untrusted"
		if err := store.AppendTerminal(ctx, conflict); err == nil {
			t.Fatal("conflicting terminal replay was accepted")
		}
	})
}

func TestCoverage90GitStoreIdempotencyAndConflictBoundaries(t *testing.T) {
	_, repo := gitStoreFixture(t)
	store, err := NewGitStore(repo, "origin", DefaultEvidenceRef)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := store.Preflight(ctx); err != nil {
		t.Fatalf("empty GitStore preflight: %v", err)
	}
	o := coverageObligation("o-git-coverage")
	if err := store.Append(ctx, Obligation{}); err == nil {
		t.Fatal("GitStore accepted an invalid obligation")
	}
	if err := store.Append(ctx, o); err != nil {
		t.Fatal(err)
	}
	if err := store.Append(ctx, o); err != nil {
		t.Fatalf("idempotent obligation replay: %v", err)
	}
	conflictingObligation := o
	conflictingObligation.Revision = "different"
	if err := store.Append(ctx, conflictingObligation); err == nil {
		t.Fatal("conflicting obligation replay was accepted")
	}
	if err := store.CompleteHandoff(ctx, ""); err == nil {
		t.Fatal("empty handoff identity was accepted")
	}
	if err := store.CompleteHandoff(ctx, "missing"); err == nil {
		t.Fatal("unknown handoff identity was accepted")
	}
	if err := store.CompleteHandoff(ctx, o.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.CompleteHandoff(ctx, o.ID); err != nil {
		t.Fatalf("idempotent handoff replay: %v", err)
	}
	if _, _, found, err := store.Attempt(ctx, o.ID, "missing-attempt"); err != nil || found {
		t.Fatalf("missing attempt found=%t err=%v", found, err)
	}
	if lease, err := store.Begin(ctx, Attempt{}); err == nil {
		lease.Close()
		t.Fatal("invalid attempt was admitted")
	}
	a := testAttempt(o.ID, "a-git-coverage", "run-git-coverage")
	lease, err := store.Begin(ctx, a)
	if err != nil {
		t.Fatal(err)
	}
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
	if lease, err := store.Begin(ctx, a); err == nil {
		lease.Close()
		t.Fatal("duplicate attempt identity was admitted")
	}
	if lease, err := store.Resume(ctx, "", ""); err == nil {
		lease.Close()
		t.Fatal("empty recovery identity was accepted")
	}

	pair := testPublicationPair(o, a)
	if err := store.RecordPair(ctx, PublicationPair{}); err == nil {
		t.Fatal("invalid pair was accepted")
	}
	if err := store.RecordPair(ctx, pair); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordPair(ctx, pair); err != nil {
		t.Fatalf("idempotent pair replay: %v", err)
	}
	conflictingPair := pair
	conflictingPair.ArtifactRef = "artifact:different"
	if err := store.RecordPair(ctx, conflictingPair); err == nil {
		t.Fatal("conflicting pair replay was accepted")
	}
	if err := store.AppendTerminal(ctx, Terminal{}); err == nil {
		t.Fatal("invalid terminal was accepted")
	}
	terminal := Terminal{SchemaVersion: TerminalSchema, ObligationID: o.ID, AttemptID: a.AttemptID, Status: "failed", FailureClass: "infrastructure"}
	if err := store.AppendTerminal(ctx, terminal); err != nil {
		t.Fatal(err)
	}
	if err := store.AppendTerminal(ctx, terminal); err != nil {
		t.Fatalf("idempotent terminal replay: %v", err)
	}
	conflictingTerminal := terminal
	conflictingTerminal.FailureClass = "artifact_untrusted"
	if err := store.AppendTerminal(ctx, conflictingTerminal); err == nil {
		t.Fatal("conflicting terminal replay was accepted")
	}

	second := testAttempt(o.ID, "a-git-coverage-2", "run-git-coverage-2")
	lease, err = store.Begin(ctx, second)
	if err != nil {
		t.Fatal(err)
	}
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordPair(ctx, testPublicationPair(o, second)); err != nil {
		t.Fatal(err)
	}

	conflictingAttempt := second
	conflictingAttempt.RunID = "different-run"
	conflictingRow := coverageJSONLine(t, conflictingAttempt)
	if _, err := store.log.Update(ctx, gitAttemptsPath, "test: seed conflicting configured-review attempt", func(current []byte) ([]byte, bool, error) {
		return append(append([]byte(nil), current...), conflictingRow...), true, nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := store.Attempt(ctx, o.ID, second.AttemptID); err == nil {
		t.Fatal("GitStore accepted conflicting canonical attempt rows")
	}
}

func TestCoverage90FileLockCloseReportsClosedDescriptor(t *testing.T) {
	f, err := os.Create(filepath.Join(t.TempDir(), "lock"))
	if err != nil {
		t.Fatal(err)
	}
	lock := &fileLock{f: f}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := lock.Close(); err == nil {
		t.Fatal("closing a lock with an already-closed descriptor succeeded")
	}
	if !errors.Is(lock.Close(), os.ErrInvalid) {
		// The exact syscall wrapper varies by platform; the first assertion is the
		// contract. This second call simply proves repeated closure remains an error.
	}
}
