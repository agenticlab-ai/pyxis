package github

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestGHClientRepositoryIdentityValidation(t *testing.T) {
	t.Parallel()
	transport := errors.New("identity transport failed")
	for _, tc := range []struct {
		name, response, want string
		err                  error
	}{
		{name: "native identity", response: " R_repo123\n", want: "R_repo123"},
		{name: "empty"},
		{name: "receipt separator", response: "R_repo:123"},
		{name: "multiple lines", response: "R_repo\n123"},
		{name: "carriage return", response: "R_repo\r123"},
		{name: "transport failure", err: transport},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := &ghClient{repo: "owner/repo", run: func(ctx context.Context, stdin []byte, args ...string) ([]byte, error) {
				if ctx != t.Context() || stdin != nil || !slices.Equal(args, []string{"api", "repos/owner/repo", "--jq", ".node_id"}) {
					t.Fatalf("identity request: context=%v stdin=%q args=%q", ctx, stdin, args)
				}
				return []byte(tc.response), tc.err
			}}
			got, err := c.repositoryNodeID(t.Context())
			if got != tc.want || (err != nil) != (tc.want == "") || (tc.err != nil && !errors.Is(err, tc.err)) {
				t.Fatalf("repositoryNodeID = %q, %v; want %q, transport=%v", got, err, tc.want, tc.err)
			}
		})
	}
}

func TestGHClientCommentCapabilityFailsClosed(t *testing.T) {
	t.Parallel()
	transport := errors.New("permission transport failed")
	for _, tc := range []struct {
		name, response string
		err            error
		want, wantErr  bool
	}{
		{name: "write permission", response: " true\n", want: true},
		{name: "read only", response: "false\n"},
		{name: "missing permissions", response: "null", wantErr: true},
		{name: "empty response", wantErr: true},
		{name: "transport failure", err: transport, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := &ghClient{repo: "owner/repo", run: func(ctx context.Context, stdin []byte, args ...string) ([]byte, error) {
				wantArgs := []string{"api", "repos/owner/repo", "--jq", ".permissions | (.admin or .maintain or .push or .triage)"}
				if ctx != t.Context() || stdin != nil || !slices.Equal(args, wantArgs) {
					t.Fatalf("permission request: context=%v stdin=%q args=%q", ctx, stdin, args)
				}
				return []byte(tc.response), tc.err
			}}
			got, err := c.canWriteProposalComments(t.Context())
			if got != tc.want || (err != nil) != tc.wantErr || (tc.err != nil && !errors.Is(err, tc.err)) {
				t.Fatalf("canWriteProposalComments = %t, %v; want %t, error=%t", got, err, tc.want, tc.wantErr)
			}
		})
	}
}

// Exercise the subprocess boundary using real local executables with known
// stream behavior. Only their temporary name is gh: no GitHub CLI, credentials,
// shell script, or network is involved in these transport tests.
func TestShellGHStreamsAndFailures(t *testing.T) {
	cat, err := exec.LookPath("cat")
	if err != nil {
		t.Fatal(err)
	}
	falseCommand, err := exec.LookPath("false")
	if err != nil {
		t.Fatal(err)
	}
	const body = "review body\n```json\n{\"finding\":\"δ\"}\n```\n"
	for _, tc := range []struct {
		name, executable string
		stdin            []byte
		stderr, canceled bool
		wantExit         int
	}{
		{name: "stdin to stdout", executable: cat, stdin: []byte(body)},
		{name: "stderr diagnostic", executable: cat, stderr: true, wantExit: 1},
		{name: "silent failure", executable: falseCommand, wantExit: 1},
		{name: "canceled before start", executable: cat, canceled: true},
		{name: "executable unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if tc.executable != "" {
				// A wrapper, not a symlink: multi-call coreutils refuse to run under another name.
				script := "#!/bin/sh\nexec " + tc.executable + " \"$@\"\n"
				if err := os.WriteFile(filepath.Join(dir, "gh"), []byte(script), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("PATH", dir)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if tc.canceled {
				cancel()
			}
			var args []string
			if tc.stderr {
				args = []string{filepath.Join(dir, "missing-comment.json")}
			}
			out, err := shellGH(ctx, tc.stdin, args...)
			switch {
			case tc.executable == "":
				if !errors.Is(err, exec.ErrNotFound) {
					t.Fatalf("missing executable error = %v", err)
				}
			case tc.canceled:
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("cancellation error = %v", err)
				}
			case tc.wantExit != 0:
				var exitErr *exec.ExitError
				if !errors.As(err, &exitErr) || exitErr.ExitCode() != tc.wantExit {
					t.Fatalf("subprocess failure = %v, want exit %d", err, tc.wantExit)
				}
				if tc.stderr && !strings.Contains(err.Error(), exitErr.Error()+":") {
					t.Fatalf("stderr diagnostic lost: %v", err)
				}
			default:
				if err != nil || string(out) != body {
					t.Fatalf("subprocess stdout = %q, %v", out, err)
				}
			}
			if err != nil && out != nil {
				t.Fatalf("failed subprocess returned usable output: %q", out)
			}
		})
	}
}
