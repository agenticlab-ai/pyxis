package process

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/implreview"
)

func processGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@pyxis.invalid",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@pyxis.invalid")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}

func processGitFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	remote, repo := filepath.Join(root, "remote.git"), filepath.Join(root, "repo")
	if out, err := exec.Command("git", "init", "--bare", "-q", remote).CombinedOutput(); err != nil {
		t.Fatalf("init bare: %v: %s", err, out)
	}
	if err := os.Mkdir(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	processGit(t, repo, "init", "-q")
	if err := os.WriteFile(filepath.Join(repo, "README"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	processGit(t, repo, "add", "README")
	processGit(t, repo, "commit", "-qm", "base")
	processGit(t, repo, "remote", "add", "origin", remote)
	processGit(t, repo, "push", "-q", "origin", "HEAD:refs/heads/main")
	return repo
}

func TestGitStoreFencesLiveCoordinatorUntilExplicitTerminal(t *testing.T) {
	repo := processGitFixture(t)
	left, err := NewGitStore(repo, "origin", DefaultEvidenceRef)
	if err != nil {
		t.Fatal(err)
	}
	right, err := NewGitStore(repo, "origin", DefaultEvidenceRef)
	if err != nil {
		t.Fatal(err)
	}
	subject := implreview.ReviewSubject{ReviewObligationID: "obligation-1", ProposalID: "github:o/r#7", ReviewedRevision: "abc123"}
	first, err := left.Prepare(context.Background(), subject)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := right.Prepare(context.Background(), subject); !errors.Is(err, ErrProcessActive) {
		t.Fatalf("concurrent Prepare error = %v, want ErrProcessActive", err)
	}

	// Releasing a clone-local fence is not evidence that an owner in another
	// clone died. The row remains active until canonical recovery appends its
	// explicit terminal.
	left.release(first.ReviewProcessID)
	if _, err := right.Prepare(context.Background(), subject); !errors.Is(err, ErrProcessActive) {
		t.Fatalf("unterminalled admission after local release = %v, want ErrProcessActive", err)
	}
	if err := left.Finish(context.Background(), first, implreview.HandoffMissing, diagnosticDigest(implreview.HandoffMissing, "explicit recovery")); err != nil {
		t.Fatal(err)
	}
	second, err := right.Prepare(context.Background(), subject)
	if err != nil {
		t.Fatal(err)
	}
	if second.ReviewProcessID == first.ReviewProcessID {
		t.Fatal("abandoned process identity was reused")
	}
	if err := right.Finish(context.Background(), second, implreview.HandoffMissing, diagnosticDigest(implreview.HandoffMissing, "fixture")); err != nil {
		t.Fatal(err)
	}
	if err := right.Preflight(context.Background()); err != nil {
		t.Fatalf("reduced repaired process evidence: %v", err)
	}
}

func TestGitStoreCASFencesSeparateClones(t *testing.T) {
	repo := processGitFixture(t)
	remote := filepath.Join(filepath.Dir(repo), "remote.git")
	other := filepath.Join(t.TempDir(), "other")
	if out, err := exec.Command("git", "clone", "-q", remote, other).CombinedOutput(); err != nil {
		t.Fatalf("clone: %v: %s", err, out)
	}
	left, _ := NewGitStore(repo, "origin", DefaultEvidenceRef)
	right, _ := NewGitStore(other, "origin", DefaultEvidenceRef)
	subject := implreview.ReviewSubject{ReviewObligationID: "obligation-cross-clone", ProposalID: "github:o/r#8", ReviewedRevision: "def456"}
	if _, err := left.Prepare(context.Background(), subject); err != nil {
		t.Fatal(err)
	}
	if _, err := right.Prepare(context.Background(), subject); !errors.Is(err, ErrProcessActive) {
		t.Fatalf("separate clone admitted a second process: %v", err)
	}
}

func TestParseRowsRejectsMalformedState(t *testing.T) {
	validSubject := `{"review_obligation_id":"obligation-1","proposal_id":"github:o/r#1","reviewed_revision":"abc123"}`
	for _, data := range []string{
		`{`,
		`{}`,
		`{"schema_version":"impl-review-process-state/v1","kind":"admitted","review_process_id":"p-1","status":"completed","subject":` + validSubject + `}`,
		`{"schema_version":"impl-review-process-state/v1","kind":"terminal","review_process_id":"p-1","diagnostic_digest":"digest","subject":` + validSubject + `}`,
		`{"schema_version":"impl-review-process-state/v1","kind":"unknown","review_process_id":"p-1","subject":` + validSubject + `}`,
	} {
		if _, err := parseRows([]byte(data)); err == nil {
			t.Fatalf("parseRows accepted malformed state %s", data)
		}
	}
}

func TestGitStoreFinishRejectsUnknownAndConflictingAdmissions(t *testing.T) {
	repo := processGitFixture(t)
	store, err := NewGitStore(repo, "origin", DefaultEvidenceRef)
	if err != nil {
		t.Fatal(err)
	}
	subject := implreview.ReviewSubject{ReviewObligationID: "obligation-finish", ProposalID: "github:o/r#9", ReviewedRevision: "abc123"}
	unknown := implreview.AdmittedProcess{ReviewProcessID: "missing-process", Subject: subject}
	if err := store.Finish(t.Context(), unknown, implreview.HandoffMissing, "digest"); err == nil {
		t.Fatal("unknown process was terminalized")
	}
	admitted, err := store.Prepare(t.Context(), subject)
	if err != nil {
		t.Fatal(err)
	}
	conflict := admitted
	conflict.Subject.ProposalID = "github:o/r#different"
	if err := store.Finish(t.Context(), conflict, implreview.HandoffMissing, "digest"); err == nil {
		t.Fatal("conflicting admitted subject was terminalized")
	}
	if err := store.Finish(t.Context(), admitted, implreview.HandoffMissing, "digest"); err != nil {
		t.Fatal(err)
	}
	if err := store.Finish(t.Context(), admitted, implreview.HandoffMissing, "digest"); err != nil {
		t.Fatalf("idempotent terminal replay: %v", err)
	}
	if err := store.Finish(t.Context(), admitted, implreview.HandoffMissing, "different"); err == nil {
		t.Fatal("conflicting terminal replay was accepted")
	}
}
