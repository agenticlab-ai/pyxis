package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/watch"
)

func TestWatchRefusesUnavailableExecutionLog(t *testing.T) {
	repo := reviewRearmRepo(t, "executor:\n  logging:\n    directory: logs\n")
	writePlanJSON(t, repo)
	report := fakeReport(t, repo, `{"task_id":"T1","issue":834,"status":"existing"}`)
	if err := os.WriteFile(filepath.Join(repo, "logs"), []byte("existing file\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := watchVerb([]string{"--repo", repo, "--repo-slug", "owner/repo", "--plan", "docs/plans/self-drive-wake", "--report-cmd", report}, &stdout, &stderr)
	if code != 1 || !strings.Contains(stderr.String(), "open execution log: create implement-now log directory") {
		t.Fatalf("watch = %d, stdout=%q, stderr=%q", code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("failed log open announced a watch: %q", stdout.String())
	}
	if data, err := os.ReadFile(filepath.Join(repo, "logs")); err != nil || string(data) != "existing file\n" {
		t.Fatalf("existing file was changed: %q, %v", data, err)
	}
}

func TestWatchExecutionLogRecordsFailureAndResumedCompletion(t *testing.T) {
	repo := reviewRearmRepo(t, "executor:\n  logging:\n    directory: logs[1]\n")
	writePlanJSON(t, repo)
	report := fakeReport(t, repo, `{"task_id":"T1","issue":834,"status":"existing"}`)
	oldRun := watchRunLive
	t.Cleanup(func() { watchRunLive = oldRun })
	var logPath string
	var firstRun []byte
	for _, fail := range []bool{true, false} {
		watchRunLive = func(context.Context, watch.PlanSource, watch.Reader, watch.Sweep, watch.Clock, <-chan struct{}) (watch.Summary, error) {
			if fail {
				return watch.Summary{}, errors.New("tracker unavailable")
			}
			return watch.Summary{Completed: true}, nil
		}
		var stdout, stderr bytes.Buffer
		code := watchVerb([]string{"--repo", repo, "--repo-slug", "owner/repo", "--plan", "docs/plans/self-drive-wake", "--report-cmd", report}, &stdout, &stderr)
		wantCode := 0
		if fail {
			wantCode = 1
		}
		if code != wantCode {
			t.Fatalf("watch = %d, stdout=%q, stderr=%q", code, stdout.String(), stderr.String())
		}
		entries, err := os.ReadDir(filepath.Join(repo, "logs[1]"))
		if err != nil || len(entries) != 1 {
			t.Fatalf("execution logs = %v, %v", entries, err)
		}
		path := filepath.Join(repo, "logs[1]", entries[0].Name())
		if !strings.Contains(stdout.String(), "watch: execution log "+path) {
			t.Fatalf("watch did not announce log %q: %s", path, stdout.String())
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if fail {
			logPath, firstRun = path, data
			for _, want := range []string{"plan self-drive-wake/001 watch started with 1 task(s)", "plan self-drive-wake/001 watch failed: tracker unavailable"} {
				if !strings.Contains(string(data), want) {
					t.Fatalf("execution log missing %q: %s", want, data)
				}
			}
		} else if path != logPath || !bytes.HasPrefix(data, firstRun) || !strings.Contains(string(data), "plan self-drive-wake/001 complete:") || strings.Count(string(data), "watch started") != 2 {
			t.Fatalf("resumed watch did not append completion to %q: path=%q log=%s", logPath, path, data)
		}
	}
}
