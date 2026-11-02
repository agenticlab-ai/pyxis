package reviewobligation

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func gitStoreFixture(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	remote, repo := filepath.Join(root, "remote.git"), filepath.Join(root, "repo")
	if out, err := exec.Command("git", "init", "--bare", "-q", remote).CombinedOutput(); err != nil {
		t.Fatalf("init bare: %v: %s", err, out)
	}
	if err := os.Mkdir(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	runStoreGit(t, repo, "init", "-q")
	if err := os.WriteFile(filepath.Join(repo, "README"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runStoreGit(t, repo, "add", "README")
	runStoreGit(t, repo, "commit", "-qm", "base")
	runStoreGit(t, repo, "remote", "add", "origin", remote)
	runStoreGit(t, repo, "push", "-q", "origin", "HEAD:refs/heads/main")
	return remote, repo
}

func TestGitStoreCASFencesAttemptsAcrossSeparateClones(t *testing.T) {
	remote, repo := gitStoreFixture(t)
	other := filepath.Join(t.TempDir(), "other")
	if out, err := exec.Command("git", "clone", "-q", remote, other).CombinedOutput(); err != nil {
		t.Fatalf("fresh clone: %v: %s", err, out)
	}
	left, _ := NewGitStore(repo, "origin", DefaultEvidenceRef)
	right, _ := NewGitStore(other, "origin", DefaultEvidenceRef)
	o := Obligation{SchemaVersion: Schema, ID: "o-cross-clone", Repository: "owner/repo", Scope: "feature/001", TaskID: "T1", ProposalRef: "github:owner/repo#1", Revision: "abc", Harness: "claude"}
	if err := left.Append(context.Background(), o); err != nil {
		t.Fatal(err)
	}
	if err := left.CompleteHandoff(context.Background(), o.ID); err != nil {
		t.Fatal(err)
	}
	lease, err := left.Begin(context.Background(), testAttempt(o.ID, "attempt-left", "run-left"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = lease.Close() }() // release is best-effort; the operation result is already decided
	if _, err := right.Begin(context.Background(), testAttempt(o.ID, "attempt-right", "run-right")); !errors.Is(err, ErrAttemptActive) {
		t.Fatalf("separate clone admitted a second attempt: %v", err)
	}
}

func TestGitStoreLifecycleReplaysFromFreshClone(t *testing.T) {
	remote, repo := gitStoreFixture(t)
	store, err := NewGitStore(repo, "origin", DefaultEvidenceRef)
	if err != nil {
		t.Fatal(err)
	}
	obligation := Obligation{SchemaVersion: Schema, ID: "o-1", Repository: "owner/repo", Scope: "feature/001", TaskID: "T1", ProposalRef: "github:owner/repo#1", Revision: "abc", Harness: "claude"}
	if err := store.Append(context.Background(), obligation); err != nil {
		t.Fatal(err)
	}
	states, err := store.Open(context.Background(), "feature/001")
	if err != nil || len(states) != 1 || states[0].Status != "handoff_pending" {
		t.Fatalf("pre-handoff state = %+v, %v", states, err)
	}
	if err := store.CompleteHandoff(context.Background(), obligation.ID); err != nil {
		t.Fatal(err)
	}
	lease, err := store.Begin(context.Background(), testAttempt(obligation.ID, "a-1", "run-1"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AppendTerminal(context.Background(), Terminal{SchemaVersion: TerminalSchema, ObligationID: obligation.ID, AttemptID: "a-1", Status: "failed", FailureClass: "artifact_untrusted"}); err != nil {
		t.Fatal(err)
	}
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}

	fresh := filepath.Join(t.TempDir(), "fresh")
	if out, err := exec.Command("git", "clone", "-q", remote, fresh).CombinedOutput(); err != nil {
		t.Fatalf("fresh clone: %v: %s", err, out)
	}
	replayed, err := NewGitStore(fresh, "origin", DefaultEvidenceRef)
	if err != nil {
		t.Fatal(err)
	}
	states, err = replayed.Open(context.Background(), "feature/001")
	if err != nil {
		t.Fatal(err)
	}
	if len(states) != 1 || states[0].Obligation != obligation || states[0].AttemptCount != 1 || states[0].LastFailureClass != "artifact_untrusted" || states[0].Status != "review_pending" {
		t.Fatalf("fresh state = %+v", states)
	}
}

func TestGitStoreRejectsDuplicateAttemptIdentityAcrossObligations(t *testing.T) {
	_, repo := gitStoreFixture(t)
	store, err := NewGitStore(repo, "origin", DefaultEvidenceRef)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	first := Obligation{SchemaVersion: Schema, ID: "o-first", Repository: "owner/repo", Scope: "feature/001", TaskID: "T1", ProposalRef: "github:owner/repo#1", Revision: "abc", Harness: "claude"}
	second := Obligation{SchemaVersion: Schema, ID: "o-second", Repository: "owner/repo", Scope: "feature/001", TaskID: "T2", ProposalRef: "github:owner/repo#2", Revision: "def", Harness: "claude"}
	for _, obligation := range []Obligation{first, second} {
		if err := store.Append(ctx, obligation); err != nil {
			t.Fatal(err)
		}
		if err := store.CompleteHandoff(ctx, obligation.ID); err != nil {
			t.Fatal(err)
		}
	}
	attempt := testAttempt(first.ID, "shared-attempt-id", "run-1")
	lease, err := store.Begin(ctx, attempt)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AppendTerminal(ctx, Terminal{SchemaVersion: TerminalSchema, ObligationID: first.ID, AttemptID: attempt.AttemptID, Status: "failed", FailureClass: "infrastructure"}); err != nil {
		t.Fatal(err)
	}
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
	duplicate := testAttempt(second.ID, attempt.AttemptID, "run-2")
	if _, err := store.Begin(ctx, duplicate); err == nil || !strings.Contains(err.Error(), "duplicate attempt identity") {
		t.Fatalf("cross-obligation duplicate attempt identity = %v", err)
	}
}

func TestGitStoreRecoveryLeaseAndFailedPairPermitFreshGeneration(t *testing.T) {
	_, repo := gitStoreFixture(t)
	store, err := NewGitStore(repo, "origin", DefaultEvidenceRef)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	obligation := Obligation{SchemaVersion: Schema, ID: "o-recovery", Repository: "owner/repo", Scope: "feature/001", TaskID: "T1", ProposalRef: "github:owner/repo#1", Revision: "abc", Harness: "claude"}
	if err := store.Append(ctx, obligation); err != nil {
		t.Fatal(err)
	}
	if err := store.CompleteHandoff(ctx, obligation.ID); err != nil {
		t.Fatal(err)
	}
	attempt := testAttempt(obligation.ID, "a-old", "run-old")
	owner, err := store.Begin(ctx, attempt)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Resume(ctx, obligation.ID, attempt.AttemptID); !errors.Is(err, ErrAttemptActive) {
		t.Fatalf("concurrent recovery lease = %v", err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	recovery, err := store.Resume(ctx, obligation.ID, attempt.AttemptID)
	if err != nil {
		t.Fatalf("recovery lease after owner exit: %v", err)
	}
	pair := testPublicationPair(obligation, attempt)
	if err := store.RecordPair(ctx, pair); err != nil {
		t.Fatal(err)
	}
	if err := store.AppendTerminal(ctx, Terminal{SchemaVersion: TerminalSchema, ObligationID: obligation.ID,
		AttemptID: attempt.AttemptID, Status: "failed", FailureClass: "infrastructure"}); err != nil {
		t.Fatal(err)
	}
	if err := recovery.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Resume(ctx, obligation.ID, attempt.AttemptID); !errors.Is(err, ErrAttemptSettled) {
		t.Fatalf("settled recovery lease = %v", err)
	}
	state, ok, err := store.State(ctx, obligation.ID)
	if err != nil || !ok || state.Status != "review_pending" || state.Pair != nil {
		t.Fatalf("failed historical pair state=%+v ok=%v err=%v", state, ok, err)
	}
	fresh, err := store.Begin(ctx, testAttempt(obligation.ID, "a-fresh", "run-fresh"))
	if err != nil {
		t.Fatalf("fresh generation remained blocked: %v", err)
	}
	defer func() { _ = fresh.Close() }() // release is best-effort; the operation result is already decided
}

func TestGitStoreProductionFencesFailClosed(t *testing.T) {
	_, repo := gitStoreFixture(t)
	store, err := NewGitStore(repo, "origin", DefaultEvidenceRef)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	o := Obligation{SchemaVersion: Schema, ID: "o-fences", Repository: "owner/repo", Scope: "feature/001", TaskID: "T1", ProposalRef: "github:owner/repo#1", Revision: "abc", Harness: "claude"}
	if err := store.Append(ctx, o); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Begin(ctx, testAttempt(o.ID, "a-before-handoff", "run-1")); !errors.Is(err, ErrHandoffPending) {
		t.Fatalf("attempt before implementation handoff = %v", err)
	}
	if err := store.CompleteHandoff(ctx, o.ID); err != nil {
		t.Fatal(err)
	}
	attempt1 := testAttempt(o.ID, "a-1", "run-1")
	lease1, err := store.Begin(ctx, attempt1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Begin(ctx, testAttempt(o.ID, "a-concurrent", "run-1")); !errors.Is(err, ErrAttemptActive) {
		t.Fatalf("concurrent attempt = %v", err)
	}
	if err := store.AppendTerminal(ctx, Terminal{SchemaVersion: TerminalSchema, ObligationID: o.ID, AttemptID: "unknown", Status: "failed", FailureClass: "infrastructure"}); err == nil {
		t.Fatal("terminal for an unadmitted attempt was accepted")
	}
	badPair := testPublicationPair(o, attempt1)
	badPair.AttemptProofRef = "wrong-proof"
	if err := store.RecordPair(ctx, badPair); err == nil {
		t.Fatal("publication pair with a mismatched proof was accepted")
	}
	failed := Terminal{SchemaVersion: TerminalSchema, ObligationID: o.ID, AttemptID: attempt1.AttemptID, Status: "failed", FailureClass: "infrastructure"}
	if err := store.AppendTerminal(ctx, failed); err != nil {
		t.Fatal(err)
	}
	if err := lease1.Close(); err != nil {
		t.Fatal(err)
	}
	attempt2 := testAttempt(o.ID, "a-2", "run-2")
	lease2, err := store.Begin(ctx, attempt2)
	if err != nil {
		t.Fatal(err)
	}
	pair := testPublicationPair(o, attempt2)
	if err := store.RecordPair(ctx, pair); err != nil {
		t.Fatal(err)
	}
	if err := lease2.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Begin(ctx, testAttempt(o.ID, "a-after-pair", "run-3")); !errors.Is(err, ErrPairVerified) {
		t.Fatalf("attempt after verified pair = %v", err)
	}
	verified := testVerifiedTerminal(pair)
	if err := store.AppendTerminal(ctx, verified); err != nil {
		t.Fatal(err)
	}
	if err := store.AppendTerminal(ctx, Terminal{SchemaVersion: TerminalSchema, ObligationID: o.ID, AttemptID: attempt2.AttemptID, Status: "failed", FailureClass: "artifact_untrusted"}); err == nil {
		t.Fatal("conflicting terminal replay was accepted")
	}
	if ok, err := store.Verified(ctx, o.ID); err != nil || !ok {
		t.Fatalf("verified state = %v, %v", ok, err)
	}
	if _, err := store.Begin(ctx, testAttempt(o.ID, "a-after-verified", "run-4")); !errors.Is(err, ErrAlreadyVerified) {
		t.Fatalf("attempt after verified terminal = %v, want ErrAlreadyVerified", err)
	}
}
