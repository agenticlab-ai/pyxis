package ghcodehost

import (
	"context"
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestFailedLogsRejectOversizedRawDiagnostics(t *testing.T) {
	for _, count := range []int{1, 2} {
		head := strings.Repeat(strconv.Itoa(count), 40)
		c := &ghClient{repo: "o/r", run: func(_ context.Context, args ...string) ([]byte, error) {
			if args[0] == "api" {
				if count == 1 {
					return []byte(strings.ReplaceAll(`{"workflow_runs":[{"id":1,"workflow_id":1,"event":"pull_request","head_sha":"head","status":"completed","conclusion":"failure"}]}`, `"head"`, strconv.Quote(head))), nil
				}
				return []byte(strings.ReplaceAll(`{"workflow_runs":[{"id":1,"workflow_id":1,"event":"pull_request","head_sha":"head","status":"completed","conclusion":"failure"},{"id":2,"workflow_id":2,"event":"pull_request","head_sha":"head","status":"completed","conclusion":"failure"}]}`, `"head"`, strconv.Quote(head))), nil
			}
			return []byte(strings.Repeat("x", (8<<20)/count+1)), nil
		}}
		logs, err := c.failedLogs(t.Context(), head)
		if err == nil || logs != "" {
			t.Fatalf("%d runs: oversized diagnostics accepted (%d bytes), %v", count, len(logs), err)
		}
	}
}

func TestBoundedLogCommandOwnsOverflowAndCancellation(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("PYXIS_TEST_LOG_CHILD", "1")
	for _, tc := range []struct {
		name    string
		mode    string
		size    int
		limit   int
		want    string
		wantErr error
	}{
		{name: "exact bound", mode: "stdout", size: 10, limit: 10, want: strings.Repeat("x", 10)},
		{name: "stdout overflow", mode: "stdout", size: 100000, limit: 10, wantErr: errGHLogLimit},
		{name: "stderr overflow", mode: "stderr", size: 100000, limit: 10, wantErr: errGHLogLimit},
		{name: "empty budget", mode: "stdout", size: 1, limit: 0, wantErr: errGHLogLimit},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := boundedLogCommand(t.Context(), tc.limit, exe, "-test.run=^TestGHBoundedLogProcess$", "--", tc.mode, strconv.Itoa(tc.size))
			if string(out) != tc.want || !errors.Is(err, tc.wantErr) {
				t.Fatalf("bounded log = %d bytes, %v", len(out), err)
			}
		})
	}
	t.Run("command failure retains bounded stderr", func(t *testing.T) {
		out, err := boundedLogCommand(t.Context(), 100, exe, "-test.run=^TestGHBoundedLogProcess$", "--", "fail", "10")
		if len(out) != 0 || err == nil || !strings.Contains(err.Error(), "xxxxxxxxxx") {
			t.Fatalf("command error = %q, %v", out, err)
		}
	})
	t.Run("canceled owner", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		out, err := boundedLogCommand(ctx, 100, exe)
		if len(out) != 0 || !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled log = %q, %v", out, err)
		}
	})
	t.Run("missing command", func(t *testing.T) {
		if out, err := boundedLogCommand(t.Context(), 100, t.TempDir()+"/absent"); err == nil || len(out) != 0 {
			t.Fatalf("missing command = %q, %v", out, err)
		}
	})
	t.Run("selected token stays out of command errors", func(t *testing.T) {
		env := append(os.Environ(), "GH_TOKEN=xxxxxxxxxx", "GITHUB_TOKEN=xxxxxxxxxx")
		out, err := boundedLogCommandEnv(t.Context(), 100, env, exe, "-test.run=^TestGHBoundedLogProcess$", "--", "fail", "10")
		if len(out) != 0 || err == nil || strings.Contains(err.Error(), "xxxxxxxxxx") || !strings.Contains(err.Error(), "[redacted]") {
			t.Fatalf("selected token was not scrubbed: %v", err)
		}
	})
}

func TestGHBoundedLogProcess(t *testing.T) {
	if os.Getenv("PYXIS_TEST_LOG_CHILD") == "" {
		return
	}
	mode := os.Args[len(os.Args)-2]
	size, err := strconv.Atoi(os.Args[len(os.Args)-1])
	if err != nil {
		t.Fatal(err)
	}
	dest := os.Stdout
	if mode != "stdout" {
		dest = os.Stderr
	}
	if _, err := dest.WriteString(strings.Repeat("x", size)); err != nil {
		os.Exit(2) // The owning command may close the pipe after its limit.
	}
	if mode == "fail" {
		os.Exit(7)
	}
	os.Exit(0)
}

func TestFailedLogsBoundedRunnerAndExhaustedAggregate(t *testing.T) {
	for _, fail := range []bool{false, true} {
		calls := 0
		c := &ghClient{repo: "o/r", run: func(context.Context, ...string) ([]byte, error) {
			return []byte(`{"workflow_runs":[{"id":1,"workflow_id":1,"event":"pull_request","head_sha":"head","status":"completed","conclusion":"failure"},{"id":2,"workflow_id":2,"event":"pull_request","head_sha":"head","status":"completed","conclusion":"failure"}]}`), nil
		}, runLog: func(_ context.Context, limit int, args ...string) ([]byte, error) {
			calls++
			if fail {
				return nil, errGHLogLimit
			}
			return []byte(strings.Repeat("x", limit)), nil
		}}
		out, err := c.failedLogs(t.Context(), "head")
		if out != "" || !errors.Is(err, errGHLogLimit) || calls != 1 {
			t.Fatalf("aggregate overflow = %d bytes, %v, %d commands", len(out), err, calls)
		}
	}
}
