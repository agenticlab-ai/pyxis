package prdescription

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// gateRepo builds a repository whose feature branch adds two lines of Go,
// with origin/main recorded the way a CI checkout fetches it.
func gateRepo(t *testing.T) (dir, base, head string) {
	t.Helper()
	dir = t.TempDir()
	for k, v := range map[string]string{"GIT_CONFIG_NOSYSTEM": "1", "GIT_CONFIG_GLOBAL": os.DevNull,
		"GIT_AUTHOR_NAME": "t", "GIT_AUTHOR_EMAIL": "t@example.com", "GIT_COMMITTER_NAME": "t", "GIT_COMMITTER_EMAIL": "t@example.com"} {
		t.Setenv(k, v)
	}
	gitIn(t, dir, "init", "-q", "-b", "main")
	gitIn(t, dir, "commit", "-q", "--allow-empty", "-m", "root")
	base = gitIn(t, dir, "rev-parse", "HEAD")
	gitIn(t, dir, "update-ref", "refs/remotes/origin/main", base)
	gitIn(t, dir, "switch", "-q", "-c", "feat/x")
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a\n\nvar A = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitIn(t, dir, "add", "-A")
	gitIn(t, dir, "commit", "-q", "-m", "feature")
	return dir, base, gitIn(t, dir, "rev-parse", "HEAD")
}

func writeEvent(t *testing.T, payload any) string {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "event.json")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestReadEvent(t *testing.T) {
	body := "text"
	full := map[string]any{"pull_request": map[string]any{"number": 7, "body": body,
		"base": map[string]any{"ref": "main", "sha": "b"}, "head": map[string]any{"ref": "feat/x", "sha": "h"}, "title": "ignored"}}
	e, err := ReadEvent(writeEvent(t, full))
	if err != nil || e != (Event{Number: 7, Body: "text", BaseRef: "main", BaseSHA: "b", HeadRef: "feat/x", HeadSHA: "h"}) {
		t.Fatalf("ReadEvent = %+v, %v", e, err)
	}
	nullBody := map[string]any{"pull_request": map[string]any{"number": 7, "body": nil,
		"base": map[string]any{"ref": "main", "sha": "b"}, "head": map[string]any{"ref": "feat/x", "sha": "h"}}}
	if e, err := ReadEvent(writeEvent(t, nullBody)); err != nil || e.Body != "" {
		t.Fatalf("a null body reads as empty: %+v, %v", e, err)
	}
	notJSON := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(notJSON, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		"":                              "GITHUB_EVENT_PATH is unset",
		filepath.Join(t.TempDir(), "x"): "read event payload",
		notJSON:                         "decode event payload",
		writeEvent(t, map[string]any{"push": true}):                                "not a pull request event",
		writeEvent(t, map[string]any{"pull_request": map[string]any{"number": 1}}): "lacks its base or head",
	}
	for path, want := range cases {
		if _, err := ReadEvent(path); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("ReadEvent(%q) = %v, want %q", path, err, want)
		}
	}
}

func TestGate(t *testing.T) {
	dir, base, head := gateRepo(t)
	ctx := context.Background()
	good := strings.Replace(valid1, "12 (+10/-2)", "2 (+2/-0)", 1)
	event := Event{Number: 3, Body: good, BaseRef: "main", BaseSHA: "0000000000000000000000000000000000000000", HeadRef: "feat/x", HeadSHA: head}

	result, err := Gate(ctx, dir, event, "origin", "draft/")
	if err != nil || result.Exempt || len(result.Problems) != 0 || result.Size.String() != "2 (+2/-0)" {
		t.Fatalf("Gate via the tracking ref = %+v, %v", result, err)
	}

	event.BaseRef, event.BaseSHA = "gone", base
	if result, err := Gate(ctx, dir, event, "origin", "draft/"); err != nil || len(result.Problems) != 0 {
		t.Fatalf("Gate via the event's base commit = %+v, %v", result, err)
	}

	event.Body = valid1
	if result, err := Gate(ctx, dir, event, "origin", "draft/"); err != nil || len(result.Problems) != 1 || result.Problems[0].Field != FieldLines {
		t.Fatalf("a stale count = %+v, %v", result, err)
	}

	event.HeadRef = "draft/feature"
	if result, err := Gate(ctx, dir, event, "origin", "draft/"); err != nil || !result.Exempt {
		t.Fatalf("a draft branch is exempt: %+v, %v", result, err)
	}

	event.HeadRef, event.HeadSHA = "feat/x", "no-such-commit"
	if _, err := Gate(ctx, dir, event, "origin", "draft/"); err == nil || !strings.Contains(err.Error(), "measure pull request #3") {
		t.Fatalf("an unknown head: %v", err)
	}
}
