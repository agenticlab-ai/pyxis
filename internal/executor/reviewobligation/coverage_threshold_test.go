package reviewobligation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func coverageObligation(id string) Obligation {
	return Obligation{
		SchemaVersion: Schema,
		ID:            id,
		Repository:    "owner/repo",
		Scope:         "feature/coverage",
		TaskID:        "T-coverage",
		ProposalRef:   "github:owner/repo#1",
		Revision:      "abc123",
		Harness:       "codex",
	}
}

func coverageJSONLine(t *testing.T, value any) []byte {
	t.Helper()
	b, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return append(b, '\n')
}

func TestReviewRecordValidationBoundaries(t *testing.T) {
	obligation := coverageObligation("o-validation")
	attempt := testAttempt(obligation.ID, "a-validation", "run-validation")
	pair := testPublicationPair(obligation, attempt)
	verified := testVerifiedTerminal(pair)

	checks := []struct {
		name string
		err  error
	}{
		{name: "obligation", err: validate(Obligation{})},
		{name: "attempt", err: validateAttempt(Attempt{})},
		{name: "terminal identity", err: validateTerminal(Terminal{})},
		{name: "terminal verified evidence", err: validateTerminal(Terminal{SchemaVersion: TerminalSchema, ObligationID: obligation.ID, AttemptID: attempt.AttemptID, Status: "verified"})},
		{name: "terminal failed class", err: validateTerminal(Terminal{SchemaVersion: TerminalSchema, ObligationID: obligation.ID, AttemptID: attempt.AttemptID, Status: "failed", FailureClass: "open-ended"})},
		{name: "terminal status", err: validateTerminal(Terminal{SchemaVersion: TerminalSchema, ObligationID: obligation.ID, AttemptID: attempt.AttemptID, Status: "pending"})},
		{name: "pair", err: validatePair(PublicationPair{})},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			if check.err == nil {
				t.Fatal("invalid row was accepted")
			}
		})
	}
	for _, failureClass := range []string{"artifact_unverified", "artifact_untrusted", "infrastructure"} {
		if err := validateTerminal(Terminal{SchemaVersion: TerminalSchema, ObligationID: obligation.ID, AttemptID: attempt.AttemptID, Status: "failed", FailureClass: failureClass}); err != nil {
			t.Fatalf("valid failure class %q: %v", failureClass, err)
		}
	}
	if err := validate(obligation); err != nil {
		t.Fatal(err)
	}
	if err := validateAttempt(attempt); err != nil {
		t.Fatal(err)
	}
	if err := validatePair(pair); err != nil {
		t.Fatal(err)
	}
	if err := validateTerminal(verified); err != nil {
		t.Fatal(err)
	}
}

func TestReviewJSONAndParserBoundaries(t *testing.T) {
	var dst map[string]any
	for _, input := range [][]byte{[]byte("{"), []byte("{} {}"), []byte("{} {")} {
		if err := decodeRecordJSON(input, &dst); err == nil {
			t.Fatalf("decodeRecordJSON(%q) succeeded", input)
		}
	}

	parsers := []struct {
		name  string
		parse func([]byte) error
	}{
		{name: "obligations", parse: func(data []byte) error { _, err := parseObligations(data); return err }},
		{name: "attempts", parse: func(data []byte) error { _, err := parseAttempts(data); return err }},
		{name: "terminals", parse: func(data []byte) error { _, err := parseTerminals(data); return err }},
		{name: "pairs", parse: func(data []byte) error { _, err := parsePairs(data); return err }},
		{name: "handoffs", parse: func(data []byte) error { _, err := parseHandoffs(data); return err }},
	}
	for _, parser := range parsers {
		t.Run(parser.name, func(t *testing.T) {
			for _, input := range [][]byte{[]byte("{\n"), []byte("{}\n")} {
				if err := parser.parse(input); err == nil {
					t.Fatalf("parser accepted %q", input)
				}
			}
		})
	}

	wantParse := errors.New("parse sentinel")
	if err := parseLines([]byte("row\n"), func([]byte) error { return wantParse }); !errors.Is(err, wantParse) {
		t.Fatalf("parse callback error = %v", err)
	}
	tooLong := append(bytes.Repeat([]byte("x"), (1<<20)+1), '\n')
	if err := parseLines(tooLong, func([]byte) error { return nil }); err == nil {
		t.Fatal("oversized JSONL row was accepted")
	}
	if _, _, err := appendJSONLine(nil, make(chan int)); err == nil {
		t.Fatal("unencodable JSON row was accepted")
	}
}

func TestReduceStatesRejectsInconsistentEvidence(t *testing.T) {
	o := coverageObligation("o-reduce")
	a := testAttempt(o.ID, "a-reduce", "run-1")
	h := Handoff{SchemaVersion: HandoffSchema, ObligationID: o.ID}
	failed := Terminal{SchemaVersion: TerminalSchema, ObligationID: o.ID, AttemptID: a.AttemptID, Status: "failed", FailureClass: "infrastructure"}
	pair := testPublicationPair(o, a)
	verified := testVerifiedTerminal(pair)

	conflictingO := o
	conflictingO.Revision = "different"
	conflictingA := a
	conflictingA.RunID = "different"
	conflictingTerminal := failed
	conflictingTerminal.FailureClass = "artifact_untrusted"
	conflictingPair := pair
	conflictingPair.ArtifactRef = "artifact:different"

	cases := []struct {
		name        string
		obligations []Obligation
		attempts    []Attempt
		pairs       []PublicationPair
		terminals   []Terminal
		handoffs    []Handoff
	}{
		{name: "conflicting obligations", obligations: []Obligation{o, conflictingO}},
		{name: "attempt without obligation", attempts: []Attempt{a}},
		{name: "conflicting attempts", obligations: []Obligation{o}, attempts: []Attempt{a, conflictingA}},
		{name: "terminal without attempt", obligations: []Obligation{o}, terminals: []Terminal{failed}},
		{name: "conflicting terminals", obligations: []Obligation{o}, attempts: []Attempt{a}, terminals: []Terminal{failed, conflictingTerminal}},
		{name: "pair without attempt", obligations: []Obligation{o}, pairs: []PublicationPair{pair}},
		{name: "pair proof mismatch", obligations: []Obligation{o}, attempts: []Attempt{a}, pairs: []PublicationPair{func() PublicationPair { p := pair; p.AttemptProofRef = "wrong"; return p }()}},
		{name: "pair proposal mismatch", obligations: []Obligation{o}, attempts: []Attempt{a}, pairs: []PublicationPair{func() PublicationPair { p := pair; p.ProposalRef = "github:owner/repo#2"; return p }()}},
		{name: "conflicting pairs", obligations: []Obligation{o}, attempts: []Attempt{a}, pairs: []PublicationPair{pair, conflictingPair}},
		{name: "handoff without obligation", handoffs: []Handoff{h}},
		{name: "attempt before handoff", obligations: []Obligation{o}, attempts: []Attempt{a}},
		{name: "verified terminal without pair", obligations: []Obligation{o}, attempts: []Attempt{a}, terminals: []Terminal{verified}, handoffs: []Handoff{h}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := reduceStates(o.Scope, tc.obligations, tc.attempts, tc.pairs, tc.terminals, tc.handoffs); err == nil {
				t.Fatal("inconsistent evidence was accepted")
			}
		})
	}

	states, err := reduceStates(o.Scope, []Obligation{o}, []Attempt{a}, []PublicationPair{pair}, []Terminal{failed}, []Handoff{h})
	if err != nil {
		t.Fatal(err)
	}
	if len(states) != 1 || states[0].Pair != nil || states[0].Status != "review_pending" || states[0].LastFailureClass != "infrastructure" {
		t.Fatalf("failed paired state = %+v", states)
	}
}

func TestFileStoreInvalidAndCancelledOperations(t *testing.T) {
	if _, err := NewFileStore(" \t"); err == nil {
		t.Fatal("empty path was accepted")
	}
	if _, err := NewRepositoryStore(context.Background(), ""); err == nil {
		t.Fatal("empty repository was accepted")
	}
	if _, err := NewRepositoryStore(context.Background(), t.TempDir()); err == nil {
		t.Fatal("non-repository was accepted")
	}

	store, err := NewFileStore(filepath.Join(t.TempDir(), "review.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	o := coverageObligation("o-cancelled")
	a := testAttempt(o.ID, "a-cancelled", "run-cancelled")
	p := testPublicationPair(o, a)
	terminal := Terminal{SchemaVersion: TerminalSchema, ObligationID: o.ID, AttemptID: a.AttemptID, Status: "failed", FailureClass: "infrastructure"}
	operations := []struct {
		name string
		run  func() error
	}{
		{name: "append", run: func() error { return store.Append(cancelled, o) }},
		{name: "get", run: func() error { _, _, err := store.Get(cancelled, o.ID); return err }},
		{name: "open", run: func() error { _, err := store.Open(cancelled, o.Scope); return err }},
		{name: "handoff", run: func() error { return store.CompleteHandoff(cancelled, o.ID) }},
		{name: "begin", run: func() error { _, err := store.Begin(cancelled, a); return err }},
		{name: "resume", run: func() error { _, err := store.Resume(cancelled, o.ID, a.AttemptID); return err }},
		{name: "attempt", run: func() error { _, _, _, err := store.Attempt(cancelled, o.ID, a.AttemptID); return err }},
		{name: "terminal", run: func() error { _, _, err := store.Terminal(cancelled, o.ID, a.AttemptID); return err }},
		{name: "pair", run: func() error { return store.RecordPair(cancelled, p) }},
		{name: "append terminal", run: func() error { return store.AppendTerminal(cancelled, terminal) }},
		{name: "verified", run: func() error { _, err := store.Verified(cancelled, o.ID); return err }},
	}
	for _, operation := range operations {
		t.Run(operation.name, func(t *testing.T) {
			if err := operation.run(); !errors.Is(err, context.Canceled) {
				t.Fatalf("error = %v, want context.Canceled", err)
			}
		})
	}

	if err := store.Append(context.Background(), Obligation{}); err == nil {
		t.Fatal("invalid obligation was accepted")
	}
	if err := store.CompleteHandoff(context.Background(), ""); err == nil {
		t.Fatal("empty handoff was accepted")
	}
	if err := store.CompleteHandoff(context.Background(), "missing"); err == nil {
		t.Fatal("unknown handoff was accepted")
	}
	if _, err := store.Begin(context.Background(), Attempt{}); err == nil {
		t.Fatal("invalid attempt was accepted")
	}
	if _, err := store.Resume(context.Background(), "", ""); err == nil {
		t.Fatal("empty recovery identities were accepted")
	}
	if _, err := store.Resume(context.Background(), o.ID, a.AttemptID); err == nil {
		t.Fatal("unadmitted recovery was accepted")
	}
	if err := store.RecordPair(context.Background(), PublicationPair{}); err == nil {
		t.Fatal("invalid publication pair was accepted")
	}
	if err := store.AppendTerminal(context.Background(), Terminal{}); err == nil {
		t.Fatal("invalid terminal was accepted")
	}
	if _, _, found, err := store.Attempt(context.Background(), o.ID, a.AttemptID); err != nil || found {
		t.Fatalf("missing attempt found=%v err=%v", found, err)
	}
	if _, found, err := store.Terminal(context.Background(), o.ID, a.AttemptID); err != nil || found {
		t.Fatalf("missing terminal found=%v err=%v", found, err)
	}
}

func TestFileStoreCorruptEvidenceAndFilesystemBoundaries(t *testing.T) {
	o := coverageObligation("o-corrupt")
	validObligation := coverageJSONLine(t, o)

	for _, suffix := range []string{"", ".attempts", ".terminals", ".pairs", ".handoffs"} {
		t.Run("corrupt"+strings.ReplaceAll(suffix, ".", "-"), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "review.jsonl")
			if err := os.WriteFile(path, validObligation, 0o600); err != nil {
				t.Fatal(err)
			}
			if suffix == "" {
				if err := os.WriteFile(path, []byte("{]\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			} else if err := os.WriteFile(path+suffix, []byte("{]\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			store, err := NewFileStore(path)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := store.Open(context.Background(), o.Scope); err == nil {
				t.Fatal("corrupt evidence was accepted")
			}
		})
	}

	t.Run("oversized row", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "review.jsonl")
		if err := os.WriteFile(path, append(bytes.Repeat([]byte("x"), 70*1024), '\n'), 0o600); err != nil {
			t.Fatal(err)
		}
		store, _ := NewFileStore(path)
		if _, err := store.obligationsLocked(); err == nil {
			t.Fatal("oversized row was accepted")
		}
	})

	t.Run("complete rows", func(t *testing.T) {
		dir := t.TempDir()
		missing := filepath.Join(dir, "missing")
		if got, err := completeRows(missing); err != nil || got != nil {
			t.Fatalf("missing rows = %q, %v", got, err)
		}
		empty := filepath.Join(dir, "empty")
		if err := os.WriteFile(empty, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		if got, err := completeRows(empty); err != nil || len(got) != 0 {
			t.Fatalf("empty rows = %q, %v", got, err)
		}
		torn := filepath.Join(dir, "torn")
		if err := os.WriteFile(torn, []byte("one\ntwo"), 0o600); err != nil {
			t.Fatal(err)
		}
		if got, err := completeRows(torn); err != nil || string(got) != "one\n" {
			t.Fatalf("committed rows = %q, %v", got, err)
		}
		if err := os.WriteFile(torn, []byte("only-torn"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := completeRows(torn); err == nil {
			t.Fatal("sole torn row was accepted")
		}
		if _, err := completeRows(dir); err == nil {
			t.Fatal("directory was read as evidence")
		}
	})

	t.Run("append failures", func(t *testing.T) {
		root := t.TempDir()
		blocker := filepath.Join(root, "blocker")
		if err := os.WriteFile(blocker, []byte("not a directory"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := appendSynced(filepath.Join(blocker, "review.jsonl"), []byte("{}")); err == nil {
			t.Fatal("append through a file parent succeeded")
		}
		targetDir := filepath.Join(root, "target-dir")
		if err := os.Mkdir(targetDir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := appendSynced(targetDir, []byte("{}")); err == nil {
			t.Fatal("append to a directory succeeded")
		}
		if err := createFirstRowAtomically(targetDir, []byte("{}")); err == nil {
			t.Fatal("atomic row replaced a directory")
		}
	})
}

func TestFileStoreRejectsInvalidAndConflictingCommittedRows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "review.jsonl")
	store, err := NewFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.obligationsLocked(); err == nil {
		t.Fatal("semantically incomplete obligation was accepted")
	}

	first := coverageObligation("o-conflict")
	second := first
	second.Revision = "different"
	rows := append(coverageJSONLine(t, first), coverageJSONLine(t, second)...)
	if err := os.WriteFile(path, rows, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.obligationsLocked(); err == nil {
		t.Fatal("conflicting obligation rows were accepted")
	}
	if _, _, err := store.Get(context.Background(), first.ID); err == nil {
		t.Fatal("conflicting obligation lookup succeeded")
	}
}

func TestGitStoreTerminalLookupBoundary(t *testing.T) {
	_, repo := gitStoreFixture(t)
	store, err := NewGitStore(repo, "origin", DefaultEvidenceRef)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	o := coverageObligation("o-terminal-lookup")
	if err := store.Append(ctx, o); err != nil {
		t.Fatal(err)
	}
	if err := store.CompleteHandoff(ctx, o.ID); err != nil {
		t.Fatal(err)
	}
	a := testAttempt(o.ID, "a-terminal-lookup", "run-terminal-lookup")
	lease, err := store.Begin(ctx, a)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = lease.Close() }() // release is best-effort; the operation result is already decided
	if _, found, err := store.Terminal(ctx, o.ID, a.AttemptID); err != nil || found {
		t.Fatalf("terminal before append found=%v err=%v", found, err)
	}
	want := Terminal{SchemaVersion: TerminalSchema, ObligationID: o.ID, AttemptID: a.AttemptID, Status: "failed", FailureClass: "infrastructure"}
	if err := store.AppendTerminal(ctx, want); err != nil {
		t.Fatal(err)
	}
	got, found, err := store.Terminal(ctx, o.ID, a.AttemptID)
	if err != nil || !found || got != want {
		t.Fatalf("terminal after append = %+v, found=%v err=%v", got, found, err)
	}
}
