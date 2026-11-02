package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const prDescriptionBodyText = `## Human Reviewer data
- Lines of code: 2 (+2/-0)
- Complexity: low
- Blast radius: low
- Decision type: two-way
- Review notes: Nothing unusual.

## What
- Add A.

## Why
A is needed.

## How
- Declare it.

## Testing
- [x] go test: passed.
`

func prDescriptionGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// prDescriptionRepo builds a repository whose branch adds two lines of Go on
// top of origin/main, and makes it the working directory.
func prDescriptionRepo(t *testing.T) (dir, base, head string) {
	t.Helper()
	dir = t.TempDir()
	for k, v := range map[string]string{"GIT_CONFIG_NOSYSTEM": "1", "GIT_CONFIG_GLOBAL": os.DevNull,
		"GIT_AUTHOR_NAME": "t", "GIT_AUTHOR_EMAIL": "t@example.com", "GIT_COMMITTER_NAME": "t", "GIT_COMMITTER_EMAIL": "t@example.com"} {
		t.Setenv(k, v)
	}
	prDescriptionGit(t, dir, "init", "-q", "-b", "main")
	prDescriptionGit(t, dir, "commit", "-q", "--allow-empty", "-m", "root")
	base = prDescriptionGit(t, dir, "rev-parse", "HEAD")
	prDescriptionGit(t, dir, "update-ref", "refs/remotes/origin/main", base)
	prDescriptionGit(t, dir, "switch", "-q", "-c", "feat/x")
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a\n\nvar A = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	prDescriptionGit(t, dir, "add", "-A")
	prDescriptionGit(t, dir, "commit", "-q", "-m", "feature")
	t.Chdir(dir)
	return dir, base, prDescriptionGit(t, dir, "rev-parse", "HEAD")
}

func runPRDescription(args ...string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	code := prDescriptionVerb(context.Background(), args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func TestPRDescriptionRefusesBadInvocations(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{nil, "a subcommand is required"},
		{[]string{"bogus"}, `unknown subcommand "bogus"`},
		{[]string{"lines", "extra"}, "takes no positional arguments"},
		{[]string{"lines", "--nope"}, "flag provided but not defined: -nope"},
		{[]string{"check"}, "--body is required"},
		{[]string{"gate", "--base", "x"}, "flag provided but not defined: -base"},
		{[]string{"lines", "-h"}, "usage: pyxis pr-description"},
	}
	for _, c := range cases {
		code, _, stderr := runPRDescription(c.args...)
		if code != 2 || !strings.Contains(stderr, c.want) {
			t.Errorf("%v: code %d, stderr %q; want 2 and %q", c.args, code, stderr, c.want)
		}
	}
}

func TestPRDescriptionLinesAndCheck(t *testing.T) {
	dir, base, _ := prDescriptionRepo(t)

	code, stdout, stderr := runPRDescription("lines", "--files")
	if code != 0 || stdout != "2 (+2/-0)\n  code  a.go  +2/-0\n" {
		t.Fatalf("lines against the configured mainline: %d %q %q", code, stdout, stderr)
	}
	if code, stdout, _ := runPRDescription("lines", "--base", "HEAD"); code != 0 || stdout != "n/a (no code or prompt changes)\n" {
		t.Fatalf("lines with nothing changed: %d %q", code, stdout)
	}
	if code, _, stderr := runPRDescription("lines", "--base", "no-such-ref"); code != 1 || !strings.Contains(stderr, "list changed files") {
		t.Fatalf("lines with an unknown base: %d %q", code, stderr)
	}

	good := filepath.Join(dir, "good.md")
	if err := os.WriteFile(good, []byte(prDescriptionBodyText), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, stdout, stderr := runPRDescription("check", "--body", good, "--base", base); code != 0 || !strings.Contains(stdout, "follows the template (lines of code: 2 (+2/-0))") {
		t.Fatalf("check of a good description: %d %q %q", code, stdout, stderr)
	}
	prDescriptionStdin = strings.NewReader(strings.Replace(prDescriptionBodyText, "- Complexity: low\n", "", 1))
	t.Cleanup(func() { prDescriptionStdin = os.Stdin })
	if code, _, stderr := runPRDescription("check", "--body", "-"); code != 1 || !strings.Contains(stderr, "pr-description: Complexity: missing") {
		t.Fatalf("check of a description from stdin: %d %q", code, stderr)
	}
	if code, _, stderr := runPRDescription("check", "--body", filepath.Join(dir, "absent.md")); code != 1 || !strings.Contains(stderr, "read the description") {
		t.Fatalf("check of a missing file: %d %q", code, stderr)
	}
}

func TestPRDescriptionGate(t *testing.T) {
	_, base, head := prDescriptionRepo(t)
	if code, stdout, _ := runPRDescription("gate"); code != 0 || !strings.Contains(stdout, "nothing to check") {
		t.Fatalf("gate outside its workflow: %d %q", code, stdout)
	}
	t.Setenv("PYXIS_PR_DESCRIPTION_RUN", "1")
	t.Setenv("GITHUB_EVENT_PATH", "")
	if code, _, stderr := runPRDescription("gate"); code != 1 || !strings.Contains(stderr, "GITHUB_EVENT_PATH is unset") {
		t.Fatalf("gate without an event: %d %q", code, stderr)
	}

	event := func(body, headRef, headSHA string) {
		raw, err := json.Marshal(map[string]any{"pull_request": map[string]any{"number": 9, "body": body,
			"base": map[string]any{"ref": "main", "sha": base}, "head": map[string]any{"ref": headRef, "sha": headSHA}}})
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(t.TempDir(), "event.json")
		if err := os.WriteFile(path, raw, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Setenv("GITHUB_EVENT_PATH", path)
	}
	event(prDescriptionBodyText, "feat/x", head)
	if code, stdout, stderr := runPRDescription("gate"); code != 0 || !strings.Contains(stdout, "follows the template") {
		t.Fatalf("gate on a good description: %d %q %q", code, stdout, stderr)
	}
	event(strings.Replace(prDescriptionBodyText, "2 (+2/-0)", "5", 1), "feat/x", head)
	if code, _, stderr := runPRDescription("gate"); code != 1 || !strings.Contains(stderr, "measures 2 (+2/-0)") || !strings.Contains(stderr, "edit the description of #9") {
		t.Fatalf("gate on a stale count: %d %q", code, stderr)
	}
	event("anything", "draft/feature", head)
	if code, stdout, _ := runPRDescription("gate"); code != 0 || !strings.Contains(stdout, "feature-draft branch draft/feature") {
		t.Fatalf("gate on a draft branch: %d %q", code, stdout)
	}
	event(prDescriptionBodyText, "feat/x", "no-such-commit")
	if code, _, stderr := runPRDescription("gate"); code != 1 || !strings.Contains(stderr, "measure pull request #9") {
		t.Fatalf("gate on an unknown head: %d %q", code, stderr)
	}
}

func TestPRDescriptionNeedsARepositoryAndValidConfig(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(mustGetwd(t)))
	if code, _, stderr := runPRDescription("lines"); code != 1 || !strings.Contains(stderr, "no repo root found") {
		t.Fatalf("outside a repository: %d %q", code, stderr)
	}
	dir, _, _ := prDescriptionRepo(t)
	if err := os.MkdirAll(filepath.Join(dir, ".pyxis"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".pyxis", "config.yaml"), []byte("no_such_key: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, _, stderr := runPRDescription("lines"); code != 1 || stderr == "" {
		t.Fatalf("with a malformed config: %d %q", code, stderr)
	}
}

func mustGetwd(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return wd
}
