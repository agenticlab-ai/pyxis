package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Missing required flags fail fast with a usage exit, never a partial pass.
func TestRunRequiresFlags(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run([]string{"--subject-dir", "x"}, &out, &errb); code != 2 {
		t.Fatalf("want exit 2 on missing --subject-path, got %d", code)
	}
}

func TestMainDelegatesUsageExit(t *testing.T) {
	oldArgs, oldExit := os.Args, processExit
	t.Cleanup(func() {
		os.Args = oldArgs
		processExit = oldExit
	})
	os.Args = []string{"feedback-dispose", "--bad"}
	got := -1
	processExit = func(code int) { got = code }
	main()
	if got != 2 {
		t.Fatalf("main exit = %d, want 2", got)
	}
}

func TestReadSubjectAndRunFailureBranches(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	if _, err := readSubject(missing); err == nil {
		t.Fatal("readSubject accepted a missing directory")
	}
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "effective.source.json"), []byte("projection"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "001.md"), []byte("entry"), 0o600); err != nil {
		t.Fatal(err)
	}
	entries, err := readSubject(root)
	if err != nil || len(entries) != 1 || entries[0].Name != "001.md" {
		t.Fatalf("readSubject = %+v, %v", entries, err)
	}
	broken := filepath.Join(t.TempDir(), "broken")
	if err := os.Mkdir(broken, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(broken, "absent"), filepath.Join(broken, "001.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := readSubject(broken); err == nil {
		t.Fatal("readSubject ignored an unreadable entry")
	}

	var out, errb bytes.Buffer
	if code := run([]string{"--subject-dir", missing, "--subject-path", "doc.md"}, &out, &errb); code != 1 {
		t.Fatalf("missing subject dir exit = %d", code)
	}
	errDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(errDir, "001.md"), []byte("malformed"), 0o600); err != nil {
		t.Fatal(err)
	}
	if code := run([]string{"--subject-dir", errDir, "--subject-path", "doc.md"}, &out, &errb); code != 1 {
		t.Fatalf("malformed subject exit = %d", code)
	}
}

// End-to-end: a subject with a landed edit-commit is disposed applied, and the
// entry file on disk gains the disposition block.
func TestRunAppliesLandedEdit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	env := append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
		"GIT_AUTHOR_DATE=2026-07-13T00:00:00Z", "GIT_COMMITTER_DATE=2026-07-13T00:00:00Z")
	git := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = env
		if o, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, o)
		}
	}
	git("init", "-q", "-b", "main")

	// Seed the reviewed subject, then the annotation edit.
	docRel := "docs/tech-design/some-doc.md"
	if err := os.MkdirAll(filepath.Join(dir, "docs/tech-design"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, docRel), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", docRel)
	git("commit", "-q", "-m", "seed")
	if err := os.WriteFile(filepath.Join(dir, docRel), []byte("base\nedited\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", docRel)
	git("commit", "-q", "-m", "edit")
	editSha := strings.TrimSpace(gitOut(t, dir, env, []string{"rev-parse", "HEAD"}))

	// The captured ledger entry recording that edit-commit.
	subjectDir := filepath.Join(dir, "docs/feedback/some-doc")
	if err := os.MkdirAll(subjectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	entry := "# Review-feedback entry 001\n\n" +
		"| Field | Value |\n|---|---|\n" +
		"| **Entry** | 001 |\n| **Source** | conversation |\n| **Author** | @r |\n" +
		"| **Timestamp** | 2026-07-13T09:30:00Z |\n| **Subject-anchor** | some-doc#1 |\n" +
		"| **Edit-commit** | " + editSha + " |\n| **Template version** | review-feedback-entry/v1 |\n\n" +
		"## Comment\n\nedit the doc\n"
	entryPath := filepath.Join(subjectDir, "001-conversation.md")
	if err := os.WriteFile(entryPath, []byte(entry), 0o644); err != nil {
		t.Fatal(err)
	}
	pending := "# Review-feedback entry 002\n\n" +
		"| Field | Value |\n|---|---|\n" +
		"| **Entry** | 002 |\n| **Source** | conversation |\n| **Author** | @r |\n" +
		"| **Timestamp** | 2026-07-13T09:31:00Z |\n| **Subject-anchor** | some-doc#2 |\n" +
		"| **Edit-commit** |  |\n| **Template version** | review-feedback-entry/v1 |\n\n" +
		"## Comment\n\nleave this open\n"
	if err := os.WriteFile(filepath.Join(subjectDir, "002-conversation.md"), []byte(pending), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errb bytes.Buffer
	code := run([]string{"--subject-dir", subjectDir, "--subject-path", docRel, "--worktree", dir, "--ref", "HEAD"}, &out, &errb)
	if code != 0 {
		t.Fatalf("run exit %d\nstderr: %s", code, errb.String())
	}
	if !strings.Contains(out.String(), "applied") {
		t.Fatalf("want an applied report, got %q (stderr %q)", out.String(), errb.String())
	}
	if !strings.Contains(out.String(), "open — pending") {
		t.Fatalf("want a pending report, got %q", out.String())
	}
	raw, _ := os.ReadFile(entryPath)
	if !strings.Contains(string(raw), "## Disposition") || !strings.Contains(string(raw), "| **Disposition** | applied |") {
		t.Fatalf("entry did not gain an applied disposition:\n%s", raw)
	}
}

func gitOut(t *testing.T, dir string, env, args []string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = env
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return string(out)
}
