package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRun_AbsolutePathFailureIsUsageFault(t *testing.T) {
	original := resolveAbs
	resolveAbs = func(string) (string, error) { return "", errors.New("synthetic abs failure") }
	t.Cleanup(func() { resolveAbs = original })

	var stdout, stderr bytes.Buffer
	if code := run([]string{"--repo", "repo"}, &stdout, &stderr); code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
	if stdout.Len() != 0 || !strings.Contains(stderr.String(), "synthetic abs failure") {
		t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestMainDelegatesTheRunExit(t *testing.T) {
	oldArgs, oldExit := os.Args, processExit
	t.Cleanup(func() {
		os.Args = oldArgs
		processExit = oldExit
	})
	os.Args = []string{"maintenance-sweep", "unexpected"}
	got := -1
	processExit = func(code int) { got = code }
	main()
	if got != 2 {
		t.Fatalf("main exit = %d, want 2", got)
	}
}

// The invocation is fail-closed (DINV-3, incident I3): an unknown flag or a
// flag-like positional refuses with usage and the fault exit code before any
// detector runs — and emits no machine face, so nothing on stdout is mistakable
// for a clean sweep.
func TestRun_MisparseFailsClosed(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"unknown flag", []string{"--bogus"}},
		{"unknown short flag", []string{"-x"}},
		{"positional argument", []string{"sweep"}},
		{"flag-like positional after --", []string{"--", "--weird"}},
		{"positional after valid flag", []string{"--repo", ".", "extra"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(c.args, &stdout, &stderr)
			if code != 2 {
				t.Fatalf("misparse %v: exit=%d want 2", c.args, code)
			}
			if stdout.Len() != 0 {
				t.Fatalf("misparse %v emitted a machine face:\n%s", c.args, stdout.String())
			}
			if stderr.Len() == 0 {
				t.Fatalf("misparse %v produced no usage/defect on stderr", c.args)
			}
		})
	}
}

// A well-formed invocation over the surrounding checkout runs the pass and emits
// exactly one machine-face line on stdout plus the human face on stderr. The
// exit code is 0, 1, or 2 — never a crash — regardless of what this repo's
// detectors find.
func TestRun_WellFormedEmitsBothFaces(t *testing.T) {
	root := repoRoot(t)
	var stdout, stderr bytes.Buffer
	code := run([]string{"--repo", root}, &stdout, &stderr)

	if code < 0 || code > 2 {
		t.Fatalf("exit code out of range: %d", code)
	}
	out := strings.TrimRight(stdout.String(), "\n")
	if !strings.HasPrefix(out, `{"schema_version":"maintenance-report/v1"`) {
		t.Fatalf("stdout is not one maintenance-report/v1 object:\n%s", out)
	}
	if strings.Count(out, "\n") != 0 {
		t.Fatalf("machine face is not a single line:\n%s", out)
	}
	if !strings.Contains(stderr.String(), "maintenance sweep — evaluated commit") {
		t.Fatalf("stderr missing the human face:\n%s", stderr.String())
	}
}

// repoRoot returns the repository root the test runs inside, so the well-formed
// pass has a real git checkout to evaluate.
func repoRoot(t *testing.T) string {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		wd, _ := os.Getwd()
		t.Skipf("not inside a git checkout (%v); cwd=%s", err, wd)
	}
	return filepath.Clean(strings.TrimSpace(string(out)))
}
