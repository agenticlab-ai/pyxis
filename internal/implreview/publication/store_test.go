package publication

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/implreview"
)

func publicationGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@pyxis.invalid",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@pyxis.invalid")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}

func TestReduceMigratesLegacyArmedRowsWithoutRewritingEvidence(t *testing.T) {
	accepted := Record{Admission: testAdmission(), PublicationAttemptID: "publication-legacy", IdempotencyID: "idempotency-legacy", Stage: StageAccepted}
	armed := accepted
	armed.Stage = StageArmed // v1 had no CreateArmed field.
	var data []byte
	for _, record := range []Record{accepted, armed} {
		row, err := json.Marshal(stateRow{SchemaVersion: rowSchemaV1, Record: record})
		if err != nil {
			t.Fatal(err)
		}
		data = append(data, row...)
		data = append(data, '\n')
	}
	states, err := reduce(data)
	if err != nil {
		t.Fatalf("legacy state failed migration: %v", err)
	}
	got := states[armed.PublicationAttemptID]
	if got.Stage != StageArmed || !got.CreateArmed {
		t.Fatalf("legacy armed row was not conservatively fenced: %+v", got)
	}
}

func TestReduceKeepsLegacyPreArmFailureRetryable(t *testing.T) {
	accepted := Record{Admission: testAdmission(), PublicationAttemptID: "publication-legacy-failed", IdempotencyID: "idempotency-legacy", Stage: StageAccepted}
	failedTerminal := implreview.PublicationTerminal{
		ReviewProcessID: accepted.Binding.ReviewProcessID, ReviewObligationID: accepted.Binding.ReviewObligationID,
		PublicationAttemptID: accepted.PublicationAttemptID, ProposalID: accepted.Binding.ProposalID,
		ReviewedRevision: accepted.Binding.ReviewedRevision, Medium: accepted.Medium,
		IdempotencyID: accepted.IdempotencyID, Status: implreview.PublicationFailed,
		FailureClass: implreview.FailureArtifactUnverified, Reason: "malformed handoff", EVT13Ref: "evidence:evt13:failed",
	}
	failed := accepted
	failed.Stage, failed.Terminal = StageTerminal, &failedTerminal
	var data []byte
	for _, record := range []Record{accepted, failed} {
		row, err := json.Marshal(stateRow{SchemaVersion: rowSchemaV1, Record: record})
		if err != nil {
			t.Fatal(err)
		}
		data = append(data, row...)
		data = append(data, '\n')
	}
	states, err := reduce(data)
	if err != nil {
		t.Fatal(err)
	}
	if got := states[accepted.PublicationAttemptID]; got.CreateArmed {
		t.Fatalf("legacy pre-arm failure was migrated as a publication owner: %+v", got)
	}
}

func TestGitStoreArmCASHasExactlyOneOwnerAcrossClones(t *testing.T) {
	repo := publicationGitFixture(t)
	remote := filepath.Join(filepath.Dir(repo), "remote.git")
	other := filepath.Join(t.TempDir(), "other")
	if out, err := exec.Command("git", "clone", "-q", remote, other).CombinedOutput(); err != nil {
		t.Fatalf("clone: %v: %s", err, out)
	}
	left, _ := NewGitStore(repo, "origin", DefaultEvidenceRef)
	right, _ := NewGitStore(other, "origin", DefaultEvidenceRef)
	record, err := left.Admit(context.Background(), testAdmission())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := right.Admit(context.Background(), testAdmission()); err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	type outcome struct {
		owner bool
		err   error
	}
	results := make(chan outcome, 2)
	var wg sync.WaitGroup
	for _, store := range []*GitStore{left, right} {
		wg.Add(1)
		go func(store *GitStore) {
			defer wg.Done()
			<-start
			_, owner, err := store.Arm(context.Background(), record)
			results <- outcome{owner: owner, err: err}
		}(store)
	}
	close(start)
	wg.Wait()
	close(results)
	owners := 0
	for result := range results {
		if result.err != nil {
			t.Fatal(result.err)
		}
		if result.owner {
			owners++
		}
	}
	if owners != 1 {
		t.Fatalf("publication arm owners=%d, want exactly one", owners)
	}
}

func publicationGitFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	remote, repo := filepath.Join(root, "remote.git"), filepath.Join(root, "repo")
	if out, err := exec.Command("git", "init", "--bare", "-q", remote).CombinedOutput(); err != nil {
		t.Fatalf("init bare: %v: %s", err, out)
	}
	if err := os.Mkdir(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	publicationGit(t, repo, "init", "-q")
	if err := os.WriteFile(filepath.Join(repo, "README"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	publicationGit(t, repo, "add", "README")
	publicationGit(t, repo, "commit", "-qm", "base")
	publicationGit(t, repo, "remote", "add", "origin", remote)
	publicationGit(t, repo, "push", "-q", "origin", "HEAD:refs/heads/main")
	return repo
}

func testAdmission() Admission {
	return Admission{Binding: implreview.VerifiedPublicationBinding{PublicationBinding: implreview.PublicationBinding{
		ReviewObligationID: "obligation-1", ReviewProcessID: "process-1", ProposalID: "github:o/r#1",
		ReviewedRevision: "abc123", AttemptProofRef: "proof-1",
	}, ProofEvidenceRef: "evidence:attempt:1"}, HandoffDigest: "sha256:handoff", ArtifactBodyDigest: "sha256:artifact", Medium: "github"}
}

func TestGitStorePublicationLifecycleIsCASOwnedAndReplayable(t *testing.T) {
	repo := publicationGitFixture(t)
	store, err := NewGitStore(repo, "origin", DefaultEvidenceRef)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	record, err := store.Admit(ctx, testAdmission())
	if err != nil || record.Stage != StageAccepted {
		t.Fatalf("admit = %+v, %v", record, err)
	}
	armed, owner, err := store.Arm(ctx, record)
	if err != nil || !owner || armed.Stage != StageArmed {
		t.Fatalf("arm = %+v owner=%v err=%v", armed, owner, err)
	}
	if _, owner, err := store.Arm(ctx, record); err != nil || owner {
		t.Fatalf("stale concurrent arm owner=%v err=%v, want non-owner replay", owner, err)
	}
	receipted, err := store.RecordReceipt(ctx, armed, "github-comment:R_test:7")
	if err != nil || receipted.Stage != StageReceiptRecorded {
		t.Fatalf("record receipt = %+v, %v", receipted, err)
	}
	terminal := implreview.PublicationTerminal{ReviewProcessID: "process-1", ReviewObligationID: "obligation-1",
		PublicationAttemptID: record.PublicationAttemptID, ProposalID: "github:o/r#1", ReviewedRevision: "abc123",
		Medium: "github", IdempotencyID: record.IdempotencyID, Status: implreview.PublicationVerified,
		ArtifactRef: "sha256:artifact", NativeReceipt: "github-comment:R_test:7", AttemptProofRef: "proof-1",
		EVT13Ref: "evidence:evt13:1"}
	finished, err := store.Finish(ctx, receipted, terminal)
	if err != nil || finished.Stage != StageTerminal || finished.Terminal == nil || *finished.Terminal != terminal {
		t.Fatalf("finish = %+v, %v", finished, err)
	}
	stored, found, err := store.Record(ctx, record.PublicationAttemptID)
	if err != nil || !found || !recordsEqual(stored, finished) {
		t.Fatalf("protected exact read = %+v found=%v err=%v; want %+v", stored, found, err, finished)
	}

	replayed, err := store.Admit(ctx, testAdmission())
	if err != nil || !recordsEqual(replayed, finished) {
		t.Fatalf("terminal replay = %+v, %v; want %+v", replayed, err, finished)
	}
	if _, err := store.Finish(ctx, replayed, terminal); err != nil {
		t.Fatalf("idempotent terminal replay: %v", err)
	}
	if err := store.Preflight(ctx); err != nil {
		t.Fatalf("preflight reduced durable lifecycle: %v", err)
	}
}

func TestGitStoreRejectsConflictingHandoffForOneProcess(t *testing.T) {
	store, err := NewGitStore(publicationGitFixture(t), "origin", DefaultEvidenceRef)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := store.Admit(ctx, testAdmission()); err != nil {
		t.Fatal(err)
	}
	conflict := testAdmission()
	conflict.HandoffDigest = "sha256:other"
	if _, err := store.Admit(ctx, conflict); err == nil {
		t.Fatal("conflicting process handoff was accepted")
	}
}

func TestGitStoreFreshProcessDoesNotAdoptOlderArmedPublication(t *testing.T) {
	store, err := NewGitStore(publicationGitFixture(t), "origin", DefaultEvidenceRef)
	if err != nil {
		t.Fatal(err)
	}
	record, err := store.Admit(context.Background(), testAdmission())
	if err != nil {
		t.Fatal(err)
	}
	if _, owner, err := store.Arm(context.Background(), record); err != nil || !owner {
		t.Fatalf("arm owner=%v err=%v", owner, err)
	}
	recovery := testAdmission()
	recovery.Binding.ReviewProcessID = "process-recovery"
	recovery.Binding.AttemptProofRef = "proof-recovery"
	recovery.Binding.ProofEvidenceRef = "evidence:attempt:recovery"
	replayed, err := store.Admit(context.Background(), recovery)
	if err != nil {
		t.Fatal(err)
	}
	if replayed.PublicationAttemptID == record.PublicationAttemptID || replayed.Stage != StageAccepted || replayed.CreateArmed || replayed.Binding.ReviewProcessID != "process-recovery" {
		t.Fatalf("fresh process adopted older armed fence: old=%+v fresh=%+v", record, replayed)
	}
}

func TestGitStoreRecoverReturnsOriginalArmedProcessIdentity(t *testing.T) {
	store, err := NewGitStore(publicationGitFixture(t), "origin", DefaultEvidenceRef)
	if err != nil {
		t.Fatal(err)
	}
	record, err := store.Admit(context.Background(), testAdmission())
	if err != nil {
		t.Fatal(err)
	}
	armed, owner, err := store.Arm(context.Background(), record)
	if err != nil || !owner {
		t.Fatalf("arm owner=%v err=%v", owner, err)
	}
	got, found, err := store.Recover(context.Background(), implreview.ReviewSubject{
		ReviewObligationID: record.Binding.ReviewObligationID,
		ProposalID:         record.Binding.ProposalID,
		ReviewedRevision:   record.Binding.ReviewedRevision,
	})
	if err != nil || !found || got.PublicationAttemptID != armed.PublicationAttemptID || got.Binding.ReviewProcessID != "process-1" {
		t.Fatalf("recovered publication = %+v found=%v err=%v", got, found, err)
	}
}

func TestGitStoreFreshProcessGetsDistinctIdempotencyIdentity(t *testing.T) {
	store, err := NewGitStore(publicationGitFixture(t), "origin", DefaultEvidenceRef)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	first, err := store.Admit(ctx, testAdmission())
	if err != nil {
		t.Fatal(err)
	}
	failed := implreview.PublicationTerminal{ReviewProcessID: first.Binding.ReviewProcessID,
		ReviewObligationID: first.Binding.ReviewObligationID, PublicationAttemptID: first.PublicationAttemptID,
		ProposalID: first.Binding.ProposalID, ReviewedRevision: first.Binding.ReviewedRevision,
		Medium: first.Medium, IdempotencyID: first.IdempotencyID, Status: implreview.PublicationFailed,
		FailureClass: implreview.FailureArtifactUnverified, Reason: "handoff missing", EVT13Ref: "evidence:evt13:failed"}
	if _, err := store.Finish(ctx, first, failed); err != nil {
		t.Fatal(err)
	}
	recovery := testAdmission()
	recovery.Binding.ReviewProcessID = "process-recovery"
	recovery.Binding.AttemptProofRef = "proof-recovery"
	recovery.Binding.ProofEvidenceRef = "evidence:attempt:recovery"
	recovery.HandoffDigest = "sha256:recovered-handoff"
	second, err := store.Admit(ctx, recovery)
	if err != nil {
		t.Fatal(err)
	}
	if second.IdempotencyID == first.IdempotencyID || second.PublicationAttemptID == first.PublicationAttemptID {
		t.Fatalf("recovery identities first=%+v second=%+v", first, second)
	}
	if err := store.Preflight(ctx); err != nil {
		t.Fatalf("fresh process generation did not reduce: %v", err)
	}
}

func TestGitStoreMultipleHistoricalArmedRowsReconcileOneAtATime(t *testing.T) {
	store, err := NewGitStore(publicationGitFixture(t), "origin", DefaultEvidenceRef)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	first, err := store.Admit(ctx, testAdmission())
	if err != nil {
		t.Fatal(err)
	}
	first, _, err = store.Arm(ctx, first)
	if err != nil {
		t.Fatal(err)
	}
	otherAdmission := testAdmission()
	otherAdmission.Binding.ReviewProcessID = "process-2"
	otherAdmission.Binding.AttemptProofRef = "proof-2"
	otherAdmission.Binding.ProofEvidenceRef = "evidence:attempt:2"
	otherAdmission.HandoffDigest = "sha256:handoff-2"
	second, err := store.Admit(ctx, otherAdmission)
	if err != nil {
		t.Fatal(err)
	}
	second, _, err = store.Arm(ctx, second)
	if err != nil {
		t.Fatal(err)
	}
	subject := implreview.ReviewSubject{ReviewObligationID: first.Binding.ReviewObligationID, ProposalID: first.Binding.ProposalID, ReviewedRevision: first.Binding.ReviewedRevision}
	selected, found, err := store.Recover(ctx, subject)
	if err != nil || !found {
		t.Fatalf("first recovery = %+v found=%v err=%v", selected, found, err)
	}
	terminal := implreview.PublicationTerminal{ReviewProcessID: selected.Binding.ReviewProcessID, ReviewObligationID: selected.Binding.ReviewObligationID,
		PublicationAttemptID: selected.PublicationAttemptID, ProposalID: selected.Binding.ProposalID, ReviewedRevision: selected.Binding.ReviewedRevision,
		Medium: selected.Medium, IdempotencyID: selected.IdempotencyID, Status: implreview.PublicationFailed,
		FailureClass: implreview.FailureInfrastructure, Reason: "historical acknowledgement ambiguous"}
	terminal.EVT13Ref, err = TerminalRef(terminal)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Finish(ctx, selected, terminal); err != nil {
		t.Fatal(err)
	}
	next, found, err := store.Recover(ctx, subject)
	if err != nil || !found || next.PublicationAttemptID == selected.PublicationAttemptID {
		t.Fatalf("second recovery = %+v found=%v err=%v", next, found, err)
	}
	if next.PublicationAttemptID != first.PublicationAttemptID && next.PublicationAttemptID != second.PublicationAttemptID {
		t.Fatalf("recovery selected unknown publication: %+v", next)
	}
}
