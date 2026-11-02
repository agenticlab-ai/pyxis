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

func TestDispatchReportsRepositoryResolutionFailures(t *testing.T) {
	old := resolveRepoRoot
	resolveRepoRoot = func() (string, error) { return "", errors.New("root unavailable") }
	t.Cleanup(func() { resolveRepoRoot = old })
	for _, args := range [][]string{{"run"}, {"--no-cache"}, {"list"}} {
		var stdout, stderr bytes.Buffer
		if code := dispatch(args, &stdout, &stderr); code != exitConfigErr || !strings.Contains(stderr.String(), "root unavailable") {
			t.Fatalf("dispatch(%v)=%d stderr=%q", args, code, stderr.String())
		}
	}
}

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".pyxis"), 0o755); err != nil {
		t.Fatal(err)
	}
	if body != "" {
		if err := os.WriteFile(filepath.Join(root, ".pyxis", "config.yaml"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestRunGate_ExitCodes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		body string
		want int
	}{
		{
			name: "passing required check → 0",
			body: "code-gate:\n  ok:\n    command: true\n",
			want: exitPass,
		},
		{
			name: "failing required check → 1",
			body: "code-gate:\n  bad:\n    command: false\n",
			want: exitRed,
		},
		{
			name: "advisory red still passes → 0",
			body: "code-gate:\n  nit:\n    command: false\n    class: advisory\n",
			want: exitPass,
		},
		{
			name: "malformed config fails closed → 2",
			body: "code-gate:\n  bad:\n    command: true\n    bogus: x\n",
			want: exitConfigErr,
		},
		{
			name: "no config file → empty gate, vacuous pass → 0",
			body: "",
			want: exitPass,
		},
		{
			name: "no code-gate block → empty gate → 0",
			body: "executor:\n  gate:\n    build: go build ./...\n",
			want: exitPass,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := writeConfig(t, tt.body)
			var stdout, stderr bytes.Buffer
			if got := runGate(root, true, &stdout, &stderr); got != tt.want {
				t.Fatalf("runGate = %d, want %d (stderr: %s)", got, tt.want, stderr.String())
			}
		})
	}
}

func TestDispatch_InformationalVerbs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		args []string
		want int
	}{
		{[]string{"version"}, exitPass},
		{[]string{"--version"}, exitPass},
		{[]string{"help"}, exitPass},
		{[]string{"bogus-verb"}, exitConfigErr},
	}
	for _, tt := range tests {
		t.Run(tt.args[0], func(t *testing.T) {
			t.Parallel()
			var stdout, stderr bytes.Buffer
			if got := dispatch(tt.args, &stdout, &stderr); got != tt.want {
				t.Fatalf("dispatch(%v) = %d, want %d", tt.args, got, tt.want)
			}
		})
	}
}

func TestListGate(t *testing.T) {
	t.Parallel()
	root := writeConfig(t, "code-gate:\n  gofmt:\n    command: gofmt -l .\n    fail-on: output\n")
	var stdout, stderr bytes.Buffer
	if got := listGate(root, &stdout, &stderr); got != exitPass {
		t.Fatalf("listGate = %d, want %d", got, exitPass)
	}
	if !bytes.Contains(stdout.Bytes(), []byte("gofmt")) || !bytes.Contains(stdout.Bytes(), []byte("fail-on:output")) {
		t.Fatalf("list output missing check detail: %s", stdout.String())
	}
}

func TestDispatchRunsAndListsFromResolvedRepositoryRoot(t *testing.T) {
	root := writeConfig(t, "code-gate:\n  unit:\n    command: true\n")
	cmd := exec.Command("git", "-C", root, "init", "-q")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	nested := filepath.Join(root, "nested")
	if err := os.Mkdir(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(nested)
	got, err := repoRoot()
	wantRoot, wantErr := filepath.EvalSymlinks(root)
	gotRoot, gotErr := filepath.EvalSymlinks(got)
	if err != nil || wantErr != nil || gotErr != nil || gotRoot != wantRoot {
		t.Fatalf("repoRoot=%q err=%v", got, err)
	}

	for _, args := range [][]string{nil, {"run", "--no-cache"}, {"--no-cache"}} {
		var stdout, stderr bytes.Buffer
		if code := dispatch(args, &stdout, &stderr); code != exitPass || !strings.Contains(stderr.String(), "PASS") {
			t.Fatalf("dispatch(%v)=%d stderr=%q", args, code, stderr.String())
		}
	}
	for _, args := range [][]string{{"list"}, {"--list"}} {
		var stdout, stderr bytes.Buffer
		if code := dispatch(args, &stdout, &stderr); code != exitPass || !strings.Contains(stdout.String(), "unit") {
			t.Fatalf("dispatch(%v)=%d stdout=%q stderr=%q", args, code, stdout.String(), stderr.String())
		}
	}
	for _, args := range [][]string{{"-v"}, {"--help"}, {"-h"}} {
		var stdout, stderr bytes.Buffer
		if code := dispatch(args, &stdout, &stderr); code != exitPass || stdout.Len() == 0 {
			t.Fatalf("dispatch(%v)=%d stdout=%q", args, code, stdout.String())
		}
	}
	if !hasFlag([]string{"x", "--no-cache"}, "--no-cache") || hasFlag([]string{"x"}, "--no-cache") {
		t.Fatal("hasFlag did not require an exact match")
	}
}

func TestListGateFailsClosedOnMalformedConfiguration(t *testing.T) {
	root := writeConfig(t, "code-gate:\n  unit:\n    command: true\n    unknown: value\n")
	var stdout, stderr bytes.Buffer
	if code := listGate(root, &stdout, &stderr); code != exitConfigErr || stderr.Len() == 0 {
		t.Fatalf("listGate=%d stderr=%q", code, stderr.String())
	}
}
