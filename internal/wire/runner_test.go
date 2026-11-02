package wire

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestGHRunnerAbsentWrapsErrNotFound: with no gh on PATH the production
// runner returns an error wrapping exec.ErrNotFound — the exact signal the
// preflight turns into the named GHAbsentError refusal.
func TestGHRunnerAbsentWrapsErrNotFound(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // an empty dir: nothing resolvable
	_, _, _, err := GHRunner{}.RunGH(context.Background(), "--version")
	if err == nil {
		t.Fatalf("want a lookup error with gh absent")
	}
	if !errors.Is(err, exec.ErrNotFound) {
		t.Fatalf("error %v does not wrap exec.ErrNotFound", err)
	}
}

func TestGHRunnerExplicitCredentialIsChildScoped(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the sh stub fixture is POSIX-only")
	}
	dir := t.TempDir()
	stub := "#!/bin/sh\nprintf '%s|%s|%s' \"$GH_TOKEN\" \"${GITHUB_TOKEN-unset}\" \"$GH_HOST\"\n"
	if err := os.WriteFile(filepath.Join(dir, "gh"), []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("TEST_SELECTED_GITHUB", "selected-token")
	t.Setenv("GH_TOKEN", "ambient-gh")
	t.Setenv("GITHUB_TOKEN", "ambient-github")
	t.Setenv("GH_HOST", "other-host.example")
	stdout, _, exit, err := (GHRunner{TokenEnv: "TEST_SELECTED_GITHUB"}).RunGH(context.Background(), "api", "user")
	if err != nil || exit != 0 || stdout != "selected-token|unset|github.com" {
		t.Fatalf("stdout %q exit %d err %v", stdout, exit, err)
	}
	if os.Getenv("GH_TOKEN") != "ambient-gh" || os.Getenv("GITHUB_TOKEN") != "ambient-github" || os.Getenv("GH_HOST") != "other-host.example" {
		t.Fatal("runner changed parent credentials")
	}
	t.Setenv("TEST_SELECTED_GITHUB", "")
	_, _, _, err = (GHRunner{TokenEnv: "TEST_SELECTED_GITHUB"}).RunGH(context.Background(), "api", "user")
	if err == nil || !strings.Contains(err.Error(), "TEST_SELECTED_GITHUB") {
		t.Fatalf("absent bound token used ambient auth: %v", err)
	}
}

// TestGHRunnerCapturesStreamsAndExit runs the runner against a minimal
// stub executable named gh (a test fixture emulating the third-party
// binary — the one integration point a Go test cannot fake in-process):
// stdout, stderr, and the exit code must come back verbatim with a nil
// error, because a non-zero gh exit is an OUTCOME for classification,
// never a transport failure.
func TestGHRunnerCapturesStreamsAndExit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the sh stub fixture is POSIX-only")
	}
	dir := t.TempDir()
	stub := "#!/bin/sh\necho \"out $1\"\necho \"err $2\" >&2\nexit 3\n"
	if err := os.WriteFile(filepath.Join(dir, "gh"), []byte(stub), 0o755); err != nil {
		t.Fatalf("write stub: %v", err)
	}
	t.Setenv("PATH", dir)
	stdout, stderr, exit, err := GHRunner{}.RunGH(context.Background(), "alpha", "beta")
	if err != nil {
		t.Fatalf("a completed non-zero exit is not a transport error: %v", err)
	}
	if stdout != "out alpha\n" {
		t.Errorf("stdout %q", stdout)
	}
	if stderr != "err beta\n" {
		t.Errorf("stderr %q", stderr)
	}
	if exit != 3 {
		t.Errorf("exit %d, want 3", exit)
	}
}

// TestIsTransient proves the exported wrapper agrees with classify's own
// transient verdicts (TestClassify in execute_test.go exercises classify's
// full taxonomy) — the preflight GitHub-mode existence probe (plan
// init-hardening/001 T3) retries through exactly this pattern vocabulary.
func TestIsTransient(t *testing.T) {
	cases := []struct {
		stderr string
		want   bool
	}{
		{"gh: API rate limit exceeded for installation ID 123. (HTTP 403)", true},
		{"gh: Internal Server Error (HTTP 500)", true},
		{"read tcp 10.0.0.1:443: connection reset by peer", true},
		{"gh: Not Found (HTTP 404)", false},
		{"gh: Forbidden (HTTP 403)", false},
		{"something entirely unexpected", false},
	}
	for _, tc := range cases {
		if got := IsTransient(tc.stderr); got != tc.want {
			t.Errorf("IsTransient(%q) = %v, want %v", tc.stderr, got, tc.want)
		}
	}
}

// TestGHRunnerZeroExit: the happy path returns exit 0 and nil error.
func TestGHRunnerZeroExit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the sh stub fixture is POSIX-only")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "gh"), []byte("#!/bin/sh\necho ok\n"), 0o755); err != nil {
		t.Fatalf("write stub: %v", err)
	}
	t.Setenv("PATH", dir)
	stdout, stderr, exit, err := GHRunner{}.RunGH(context.Background(), "anything")
	if err != nil || exit != 0 || stdout != "ok\n" || stderr != "" {
		t.Fatalf("got stdout=%q stderr=%q exit=%d err=%v", stdout, stderr, exit, err)
	}
}

func TestFailureClassLabelsAndFirstLine(t *testing.T) {
	labels := map[failureClass]string{
		classOther:      "unclassified",
		classTransient:  "transient",
		classPermission: "permission",
		classNotFound:   "not-found",
	}
	for class, want := range labels {
		if got := class.String(); got != want {
			t.Errorf("failureClass(%d).String() = %q, want %q", class, got, want)
		}
	}
	if got := firstLine(" \n  first diagnostic  \nsecond"); got != "first diagnostic" {
		t.Errorf("firstLine = %q, want first diagnostic", got)
	}
	if got := firstLine(" \n\t"); got != "" {
		t.Errorf("firstLine(blank) = %q, want empty", got)
	}
}
