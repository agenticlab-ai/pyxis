package reviewobligation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func runStoreGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@pyxis.invalid",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@pyxis.invalid")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}

func testAttempt(obligationID, attemptID, runID string) Attempt {
	return Attempt{SchemaVersion: AttemptSchema, ObligationID: obligationID, AttemptID: attemptID, RunID: runID, AttemptProofRef: "proof-" + attemptID}
}

func testPublicationPair(obligation Obligation, attempt Attempt) PublicationPair {
	return PublicationPair{SchemaVersion: PublicationPairSchema, ObligationID: obligation.ID, AttemptID: attempt.AttemptID,
		PublicationAttemptID: "publication-" + attempt.AttemptID,
		ProposalRef:          obligation.ProposalRef, Revision: obligation.Revision, Medium: "github", IdempotencyID: "idempotency-" + obligation.ID,
		ArtifactRef:   "sha256:artifact-" + attempt.AttemptID,
		NativeReceipt: "github-comment:R_test:7", AttemptProofRef: attempt.AttemptProofRef,
		EVT13Ref: "evidence:evt13:" + attempt.AttemptID, EVT17Ref: "evidence:evt17:" + attempt.AttemptID}
}

func testVerifiedTerminal(pair PublicationPair) Terminal {
	return Terminal{SchemaVersion: TerminalSchema, ObligationID: pair.ObligationID, AttemptID: pair.AttemptID, Status: "verified",
		ArtifactRef: pair.ArtifactRef, NativeReceipt: pair.NativeReceipt, AttemptProofRef: pair.AttemptProofRef,
		EVT13Ref: pair.EVT13Ref, EVT17Ref: pair.EVT17Ref}
}

func TestNewRepositoryStoreUsesCommonDirectoryForLinkedWorktree(t *testing.T) {
	root := t.TempDir()
	repo, linked := filepath.Join(root, "repo"), filepath.Join(root, "linked")
	if err := os.Mkdir(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	runStoreGit(t, repo, "init", "-q")
	if err := os.WriteFile(filepath.Join(repo, "README"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runStoreGit(t, repo, "add", "README")
	runStoreGit(t, repo, "commit", "-qm", "base")
	runStoreGit(t, repo, "worktree", "add", "-q", "-b", "linked", linked)

	store, err := NewRepositoryStore(context.Background(), linked)
	if err != nil {
		t.Fatal(err)
	}
	o := Obligation{SchemaVersion: Schema, ID: "o-1", Repository: "o/r", TaskID: "T1", ProposalRef: "github:o/r#1", Revision: "abc", Harness: "claude"}
	if err := store.Append(context.Background(), o); err != nil {
		t.Fatalf("Append through linked worktree: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, ".git", "pyxis", "review-obligations.jsonl")); err != nil {
		t.Fatalf("common-directory state: %v", err)
	}
}

func TestFileStoreIdempotentAppendAndConflict(t *testing.T) {
	store, err := NewFileStore(t.TempDir() + "/review.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	o := Obligation{SchemaVersion: Schema, ID: "o-1", Repository: "owner/repo", TaskID: "T1", ProposalRef: "github:owner/repo#1", Revision: "abc", Harness: "claude"}
	if err := store.Append(context.Background(), o); err != nil {
		t.Fatal(err)
	}
	if err := store.Append(context.Background(), o); err != nil {
		t.Fatalf("idempotent replay: %v", err)
	}
	got, ok, err := store.Get(context.Background(), o.ID)
	if err != nil || !ok || got != o {
		t.Fatalf("Get = %+v, %v, %v", got, ok, err)
	}
	conflict := o
	conflict.Revision = "def"
	if err := store.Append(context.Background(), conflict); err == nil || !strings.Contains(err.Error(), "conflicting replay") {
		t.Fatalf("conflicting append = %v", err)
	}
}

func TestFileStoreTerminalIsIdempotentAndMarksVerified(t *testing.T) {
	store, _ := NewFileStore(t.TempDir() + "/review.jsonl")
	o := Obligation{SchemaVersion: Schema, ID: "o-1", Repository: "owner/repo", TaskID: "T1", ProposalRef: "github:owner/repo#1", Revision: "abc", Harness: "claude"}
	if err := store.Append(context.Background(), o); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Begin(context.Background(), testAttempt(o.ID, "a-before-handoff", "run-0")); !errors.Is(err, ErrHandoffPending) {
		t.Fatalf("pre-handoff Begin error = %v", err)
	}
	if err := store.CompleteHandoff(context.Background(), o.ID); err != nil {
		t.Fatal(err)
	}
	attempt := testAttempt(o.ID, "a-1", "run-1")
	lease, err := store.Begin(context.Background(), attempt)
	if err != nil {
		t.Fatal(err)
	}
	pair := testPublicationPair(o, attempt)
	if err := store.RecordPair(context.Background(), pair); err != nil {
		t.Fatal(err)
	}
	terminal := testVerifiedTerminal(pair)
	if err := store.AppendTerminal(context.Background(), terminal); err != nil {
		t.Fatal(err)
	}
	if err := store.AppendTerminal(context.Background(), terminal); err != nil {
		t.Fatal(err)
	}
	ok, err := store.Verified(context.Background(), o.ID)
	if err != nil || !ok {
		t.Fatalf("Verified = %v, %v", ok, err)
	}
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Begin(context.Background(), testAttempt(o.ID, "a-after-verified", "run-2")); !errors.Is(err, ErrAlreadyVerified) {
		t.Fatalf("attempt after verified terminal = %v, want ErrAlreadyVerified", err)
	}
}

func TestFileStoreExposesPairVerifiedUntilLifecycleTerminalReplays(t *testing.T) {
	store, _ := NewFileStore(t.TempDir() + "/review.jsonl")
	obligation := Obligation{SchemaVersion: Schema, ID: "o-pair", Repository: "owner/repo", Scope: "feature/001", TaskID: "T1", ProposalRef: "github:owner/repo#1", Revision: "abc", Harness: "claude"}
	if err := store.Append(context.Background(), obligation); err != nil {
		t.Fatal(err)
	}
	if err := store.CompleteHandoff(context.Background(), obligation.ID); err != nil {
		t.Fatal(err)
	}
	attempt := testAttempt(obligation.ID, "a-pair", "run-1")
	lease, err := store.Begin(context.Background(), attempt)
	if err != nil {
		t.Fatal(err)
	}
	pair := testPublicationPair(obligation, attempt)
	if err := store.RecordPair(context.Background(), pair); err != nil {
		t.Fatal(err)
	}
	state, ok, err := store.State(context.Background(), obligation.ID)
	if err != nil || !ok || state.Status != "pair_verified" || state.Pair == nil || *state.Pair != pair {
		t.Fatalf("pair-only state = %+v ok=%v err=%v", state, ok, err)
	}
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Begin(context.Background(), testAttempt(obligation.ID, "a-forbidden", "run-2")); !errors.Is(err, ErrPairVerified) {
		t.Fatalf("new attempt after verified pair error = %v", err)
	}
	if err := store.AppendTerminal(context.Background(), testVerifiedTerminal(pair)); err != nil {
		t.Fatal(err)
	}
	state, ok, err = store.State(context.Background(), obligation.ID)
	if err != nil || !ok || state.Status != "verified" {
		t.Fatalf("replayed lifecycle state = %+v ok=%v err=%v", state, ok, err)
	}
}

func TestFileStoreFailedTerminalWinsOverHistoricalPairAndAllowsFreshAttempt(t *testing.T) {
	store, _ := NewFileStore(t.TempDir() + "/review.jsonl")
	obligation := Obligation{SchemaVersion: Schema, ID: "o-pair-failed", Repository: "owner/repo", Scope: "feature/001", TaskID: "T1", ProposalRef: "github:owner/repo#1", Revision: "abc", Harness: "claude"}
	if err := store.Append(context.Background(), obligation); err != nil {
		t.Fatal(err)
	}
	if err := store.CompleteHandoff(context.Background(), obligation.ID); err != nil {
		t.Fatal(err)
	}
	attempt := testAttempt(obligation.ID, "a-old", "run-old")
	lease, err := store.Begin(context.Background(), attempt)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.RecordPair(context.Background(), testPublicationPair(obligation, attempt)); err != nil {
		t.Fatal(err)
	}
	if err := store.AppendTerminal(context.Background(), Terminal{SchemaVersion: TerminalSchema, ObligationID: obligation.ID,
		AttemptID: attempt.AttemptID, Status: "failed", FailureClass: "infrastructure"}); err != nil {
		t.Fatal(err)
	}
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
	state, ok, err := store.State(context.Background(), obligation.ID)
	if err != nil || !ok || state.Status != "review_pending" || state.Pair != nil {
		t.Fatalf("historical pair+failed state=%+v ok=%v err=%v", state, ok, err)
	}
	fresh, err := store.Begin(context.Background(), testAttempt(obligation.ID, "a-fresh", "run-fresh"))
	if err != nil {
		t.Fatalf("fresh attempt remained blocked: %v", err)
	}
	defer func() { _ = fresh.Close() }() // release is best-effort; the operation result is already decided
}

func TestFileStoreOpenReducesScopedPendingAndVerifiedRows(t *testing.T) {
	store, _ := NewFileStore(t.TempDir() + "/review.jsonl")
	pending := Obligation{SchemaVersion: Schema, ID: "o-pending", Repository: "owner/repo", Scope: "feature/001", TaskID: "T1", ProposalRef: "github:owner/repo#1", Revision: "abc", Harness: "claude"}
	verified := Obligation{SchemaVersion: Schema, ID: "o-verified", Repository: "owner/repo", Scope: "feature/001", TaskID: "T2", ProposalRef: "github:owner/repo#2", Revision: "def", Harness: "codex"}
	foreign := Obligation{SchemaVersion: Schema, ID: "o-foreign", Repository: "owner/repo", Scope: "other/001", TaskID: "T3", ProposalRef: "github:owner/repo#3", Revision: "ghi", Harness: "claude"}
	for _, obligation := range []Obligation{pending, verified, foreign} {
		if err := store.Append(context.Background(), obligation); err != nil {
			t.Fatal(err)
		}
	}
	for _, obligation := range []Obligation{pending, verified} {
		if err := store.CompleteHandoff(context.Background(), obligation.ID); err != nil {
			t.Fatal(err)
		}
	}
	for _, attempt := range []Attempt{
		testAttempt(pending.ID, "a-pending", "run-1"),
		testAttempt(verified.ID, "a-verified", "run-1"),
	} {
		lease, err := store.Begin(context.Background(), attempt)
		if err != nil {
			t.Fatal(err)
		}
		terminal := Terminal{SchemaVersion: TerminalSchema, ObligationID: attempt.ObligationID, AttemptID: attempt.AttemptID, Status: "failed", FailureClass: "artifact_untrusted"}
		if attempt.ObligationID == verified.ID {
			pair := testPublicationPair(verified, attempt)
			if err := store.RecordPair(context.Background(), pair); err != nil {
				t.Fatal(err)
			}
			terminal = testVerifiedTerminal(pair)
		}
		if err := store.AppendTerminal(context.Background(), terminal); err != nil {
			t.Fatal(err)
		}
		if err := lease.Close(); err != nil {
			t.Fatal(err)
		}
	}
	states, err := store.Open(context.Background(), "feature/001")
	if err != nil {
		t.Fatal(err)
	}
	if len(states) != 2 || states[0].Obligation.ID != pending.ID || states[0].Status != "review_pending" || states[0].AttemptCount != 1 || states[0].LastFailureClass != "artifact_untrusted" || states[1].Status != "verified" {
		t.Fatalf("Open = %+v", states)
	}
}

func TestFileStoreRefusesArtifactFreeVerifiedTerminal(t *testing.T) {
	store, _ := NewFileStore(t.TempDir() + "/review.jsonl")
	err := store.AppendTerminal(context.Background(), Terminal{
		SchemaVersion: TerminalSchema, ObligationID: "o-1", AttemptID: "a-1", Status: "verified",
	})
	if err == nil || !strings.Contains(err.Error(), "requires the exact artifact") {
		t.Fatalf("artifact-free verified terminal error = %v", err)
	}
}

func TestFileStoreFencesConcurrentAttemptAcrossInstances(t *testing.T) {
	path := t.TempDir() + "/review.jsonl"
	left, _ := NewFileStore(path)
	right, _ := NewFileStore(path)
	o := Obligation{SchemaVersion: Schema, ID: "o-1", Repository: "owner/repo", TaskID: "T1", ProposalRef: "github:owner/repo#1", Revision: "abc", Harness: "claude"}
	if err := left.Append(context.Background(), o); err != nil {
		t.Fatal(err)
	}
	if err := left.CompleteHandoff(context.Background(), o.ID); err != nil {
		t.Fatal(err)
	}

	start := make(chan struct{})
	type outcome struct {
		lease AttemptLease
		err   error
	}
	results := make(chan outcome, 2)
	var wg sync.WaitGroup
	for n, store := range []*FileStore{left, right} {
		wg.Add(1)
		go func(n int, store *FileStore) {
			defer wg.Done()
			<-start
			lease, err := store.Begin(context.Background(), testAttempt(o.ID, "a-"+string(rune('1'+n)), "run-concurrent"))
			results <- outcome{lease: lease, err: err}
		}(n, store)
	}
	close(start)
	wg.Wait()
	close(results)
	var admitted, fenced int
	var hold AttemptLease
	held := false
	for result := range results {
		switch {
		case result.err == nil:
			admitted++
			hold = result.lease
			held = true
		case errors.Is(result.err, ErrAttemptActive):
			fenced++
		default:
			t.Fatalf("Begin error = %v", result.err)
		}
	}
	if held {
		defer func() { _ = hold.Close() }() // release is best-effort; the operation result is already decided
	}
	if admitted != 1 || fenced != 1 {
		t.Fatalf("admitted=%d fenced=%d, want 1/1", admitted, fenced)
	}
}

func TestFileStoreRecoverySettlesAbandonedAttemptBeforeRetry(t *testing.T) {
	store, _ := NewFileStore(t.TempDir() + "/review.jsonl")
	o := Obligation{SchemaVersion: Schema, ID: "o-1", Repository: "owner/repo", TaskID: "T1", ProposalRef: "github:owner/repo#1", Revision: "abc", Harness: "claude"}
	if err := store.Append(context.Background(), o); err != nil {
		t.Fatal(err)
	}
	if err := store.CompleteHandoff(context.Background(), o.ID); err != nil {
		t.Fatal(err)
	}
	first, err := store.Begin(context.Background(), testAttempt(o.ID, "a-1", "run-1"))
	if err != nil {
		t.Fatal(err)
	}
	// Simulate process death: its OS fence disappears without a terminal.
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	second, err := store.Begin(context.Background(), testAttempt(o.ID, "a-2", "run-2"))
	if err != nil {
		t.Fatalf("recovery Begin: %v", err)
	}
	defer func() { _ = second.Close() }() // release is best-effort; the operation result is already decided
	rows, err := store.terminalsLocked()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].AttemptID != "a-1" || rows[0].FailureClass != "infrastructure" {
		t.Fatalf("abandoned terminal rows = %+v", rows)
	}
}

func TestFileStoreIgnoresAndRepairsOnlyAnUncommittedFinalRow(t *testing.T) {
	path := filepath.Join(t.TempDir(), "review.jsonl")
	store, err := NewFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	first := Obligation{SchemaVersion: Schema, ID: "o-1", Repository: "owner/repo", TaskID: "T1", ProposalRef: "github:owner/repo#1", Revision: "abc", Harness: "codex"}
	if err := store.Append(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(`{"schema_version":"review-obligation/v1","review_obligation_id":"torn`); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if got, ok, err := store.Get(context.Background(), first.ID); err != nil || !ok || got != first {
		t.Fatalf("committed row behind torn tail = %+v, found=%v err=%v", got, ok, err)
	}
	second := Obligation{SchemaVersion: Schema, ID: "o-2", Repository: "owner/repo", TaskID: "T2", ProposalRef: "github:owner/repo#2", Revision: "def", Harness: "codex"}
	if err := store.Append(context.Background(), second); err != nil {
		t.Fatal(err)
	}
	rows, err := store.obligationsLocked()
	if err != nil || len(rows) != 2 {
		t.Fatalf("rows after torn-tail repair = %+v, err=%v", rows, err)
	}
}

func TestFileStoreFailsClosedWhenEvidenceHasNoCommittedRow(t *testing.T) {
	path := filepath.Join(t.TempDir(), "review.jsonl")
	if err := os.WriteFile(path, []byte(`{"schema_version":"review-obligation/v1","review_obligation_id":"torn`), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := NewFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if states, err := store.Open(context.Background(), ""); err == nil {
		t.Fatalf("truncated evidence read as clean frontier: %+v", states)
	}
}

func TestFileStoreFirstAppendIsNewlineCommitted(t *testing.T) {
	path := filepath.Join(t.TempDir(), "review.jsonl")
	store, err := NewFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	o := Obligation{SchemaVersion: Schema, ID: "o-1", Repository: "owner/repo", TaskID: "T1", ProposalRef: "github:owner/repo#1", Revision: "abc", Harness: "codex"}
	if err := store.Append(context.Background(), o); err != nil {
		t.Fatal(err)
	}
	got, ok, err := store.Get(context.Background(), o.ID)
	if err != nil || !ok || got != o {
		t.Fatalf("obligation = %+v, found=%v err=%v", got, ok, err)
	}
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 || data[len(data)-1] != '\n' {
		t.Fatalf("first-row bytes are not newline committed: %q, %v", data, err)
	}
}

func TestFileStoreWillNotReplaceTornFirstRow(t *testing.T) {
	path := filepath.Join(t.TempDir(), "review.jsonl")
	original := []byte(`{"schema_version":"some-other-record"`)
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	store, _ := NewFileStore(path)
	o := Obligation{SchemaVersion: Schema, ID: "o-1", Repository: "owner/repo", TaskID: "T1", ProposalRef: "github:owner/repo#1", Revision: "abc", Harness: "codex"}
	if err := store.Append(context.Background(), o); err == nil || !strings.Contains(err.Error(), "no newline-committed row") {
		t.Fatalf("torn row append = %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != string(original) {
		t.Fatalf("unrelated corruption was replaced: %q, %v", got, err)
	}
}

func TestFileStoreBeginRefusalDoesNotCommitOrReplaceTornAttempt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "review.jsonl")
	store, _ := NewFileStore(path)
	o := Obligation{SchemaVersion: Schema, ID: "o-1", Repository: "owner/repo", TaskID: "T1", ProposalRef: "github:owner/repo#1", Revision: "abc", Harness: "codex"}
	if err := store.Append(context.Background(), o); err != nil {
		t.Fatal(err)
	}
	oldAttempt, _ := json.Marshal(testAttempt(o.ID, "attempt-crashed", "run-old"))
	torn := oldAttempt[:len(oldAttempt)/2]
	if err := os.WriteFile(path+".attempts", torn, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Begin(context.Background(), testAttempt(o.ID, "attempt-retry", "run-new")); !errors.Is(err, ErrHandoffPending) {
		t.Fatalf("Begin error = %v, want ErrHandoffPending before attempt-store mutation", err)
	}
	got, err := os.ReadFile(path + ".attempts")
	if err != nil || !bytes.Equal(got, torn) {
		t.Fatalf("refused Begin mutated torn attempt: %q, %v", got, err)
	}
}

func TestFileStoreReadsAdditiveFieldsWithoutWeakeningRequiredValidation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "review.jsonl")
	store, err := NewFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	row := `{"schema_version":"review-obligation/v1","review_obligation_id":"o-1","repository":"owner/repo","task_id":"T1","proposal_ref":"github:owner/repo#1","revision":"abc","backend":"codex","future_additive_field":true}` + "\n"
	if err := os.WriteFile(path, []byte(row), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, ok, err := store.Get(context.Background(), "o-1"); err != nil || !ok || got.ID != "o-1" {
		t.Fatalf("additive row = %+v, found=%v err=%v", got, ok, err)
	}
	if err := os.WriteFile(path, []byte(`{"future_additive_field":true}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.Get(context.Background(), "o-1"); err == nil {
		t.Fatal("row missing required schema and identities was accepted")
	}
}
