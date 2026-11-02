package gate

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
)

func TestCoverage90CorpusAndEntryReadBoundaries(t *testing.T) {
	cfg := config.Defaults()
	if !inCorpus(".pyxis/templates/example.md", cfg, false) {
		t.Fatal("template markdown was excluded from the corpus")
	}
	if inCorpus("docs/reviews/example.md", cfg, true) {
		t.Fatal("review markdown was included when reviews are skipped")
	}

	dir := t.TempDir()
	if err := os.Symlink("missing", filepath.Join(dir, "000-base.md")); err != nil {
		t.Fatal(err)
	}
	entries, defects := ReadEntries(dir)
	if entries != nil || len(defects) == 0 {
		t.Fatalf("ReadEntries = entries %+v defects %+v", entries, defects)
	}
}

func TestCoverage90ConfigPolicyTreatsDeletionAsDefaults(t *testing.T) {
	root := coverageGitRepo(t)
	writeCoverageFile(t, root, ".pyxis/config.yaml", "system:\n  version: \"0.1.0\"\ndocgate:\n  version: \"source\"\ngrandfathered_designs:\n  - old-one\n")
	coverageGit(t, root, "add", ".pyxis/config.yaml")
	coverageGit(t, root, "commit", "-qm", "base config")
	if err := os.Remove(filepath.Join(root, ".pyxis", "config.yaml")); err != nil {
		t.Fatal(err)
	}
	if defects := CheckConfigPolicy(root, "HEAD"); len(defects) == 0 {
		t.Fatal("deleting a policy-bearing config produced no defect")
	}
}

func TestCoverage90LedgerHistoryReportsInvalidMergeBase(t *testing.T) {
	if defects := CheckLedgerHistory(t.TempDir(), "missing-ref", config.Defaults()); len(defects) == 0 {
		t.Fatal("invalid merge base produced no ledger-history defect")
	}
}

func TestCoverage90ShapeWithoutPairedDesignSkipsBindingCheck(t *testing.T) {
	root := t.TempDir()
	cfg := config.Defaults()
	doc := "# standalone shape\n\n| Field | Value |\n|---|---|\n| Design | |\n| Status | authoritative |\n\n```sql\nCREATE TABLE t (id INT);\n```\n"
	writeCoverageFile(t, root, filepath.ToSlash(filepath.Join(cfg.SolutionShapes, "standalone.md")), doc)
	if defects := CheckShapeLifecycle(root, "missing-ref", cfg); len(defects) != 0 {
		t.Fatalf("standalone shape defects = %+v", defects)
	}
}

func TestCoverage90ReviewFeedbackSkipsUnreadableAndMalformedEntries(t *testing.T) {
	root := coverageGitRepo(t)
	dir := filepath.Join(root, "docs", "feedback", "demo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("missing", filepath.Join(dir, "001-review.md")); err != nil {
		t.Fatal(err)
	}
	writeCoverageFile(t, root, "docs/feedback/demo/002-review.md", "not a feedback entry\n")
	coverageGit(t, root, "add", "docs/feedback/demo")
	_ = CheckReviewFeedbackCrosslinks(root, config.Defaults())
	if reviewReportHasAnchor([]byte("#\n"), "FND-1") {
		t.Fatal("empty heading resolved as a finding anchor")
	}
}

func coverageGitRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	coverageGit(t, root, "init", "-q", "-b", "main")
	return root
}

func coverageGit(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=coverage", "GIT_AUTHOR_EMAIL=coverage@example.invalid", "GIT_COMMITTER_NAME=coverage", "GIT_COMMITTER_EMAIL=coverage@example.invalid")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func writeCoverageFile(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
