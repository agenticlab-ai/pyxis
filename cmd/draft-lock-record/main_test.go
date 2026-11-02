package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/draft/approval"
	"github.com/agenticlab-ai/pyxis/internal/draft/lifecycle"
	"github.com/agenticlab-ai/pyxis/internal/draft/lock"
)

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("output failed") }

func TestMainReportsFailureAndSuccess(t *testing.T) {
	oldRun, oldExit, oldError := processRun, processExit, processError
	t.Cleanup(func() { processRun, processExit, processError = oldRun, oldExit, oldError })
	var stderr bytes.Buffer
	processError = &stderr
	code := 0
	processExit = func(value int) { code = value }
	processRun = func(context.Context) error { return errors.New("failed lock") }
	main()
	if code != 1 || !strings.Contains(stderr.String(), "failed lock") {
		t.Fatalf("exit=%d stderr=%q", code, stderr.String())
	}
	code, stderr = 0, bytes.Buffer{}
	processRun = func(context.Context) error { return nil }
	main()
	if code != 0 || stderr.Len() != 0 {
		t.Fatalf("success exit=%d stderr=%q", code, stderr.String())
	}
}

func TestRunRecordsPushAndReportsErrors(t *testing.T) {
	oldGetwd, oldGetenv, oldRange, oldOutput := processGetwd, processGetenv, processRecordRange, processOutput
	t.Cleanup(func() {
		processGetwd, processGetenv, processRecordRange, processOutput = oldGetwd, oldGetenv, oldRange, oldOutput
	})
	root := t.TempDir()
	processGetwd = func() (string, error) { return root, nil }
	env := map[string]string{"GITHUB_REPOSITORY": "example/repo", "PYXIS_PUSH_BEFORE": strings.Repeat("a", 40), "GITHUB_SHA": strings.Repeat("b", 40), "GH_TOKEN": "test-token", "PYXIS_LOCK_RECORD_RUN": "1"}
	processGetenv = func(key string) string { return env[key] }
	var stdout bytes.Buffer
	processOutput = &stdout
	called := 0
	processRecordRange = func(_ context.Context, gotRoot string, cfg config.Config, repo, before, after string, reader approval.Port, runner lifecycle.RunCommand) ([]lock.LockRecord, error) {
		called++
		if gotRoot != root || cfg.Remote != "origin" || repo != env["GITHUB_REPOSITORY"] || before != env["PYXIS_PUSH_BEFORE"] || after != env["GITHUB_SHA"] || reader == nil || runner == nil {
			t.Fatalf("bad range input root=%q repo=%q before=%q after=%q", gotRoot, repo, before, after)
		}
		return []lock.LockRecord{{Feature: "widget", Version: 2, MergeCommit: after, PullRequest: 17}}, nil
	}
	if err := run(t.Context()); err != nil || called != 1 || !strings.Contains(stdout.String(), "lock/widget/v2") {
		t.Fatalf("run error=%v calls=%d output=%q", err, called, stdout.String())
	}
	processOutput = failingWriter{}
	if err := run(t.Context()); err == nil || !strings.Contains(err.Error(), "report lock record") {
		t.Fatalf("output error=%v", err)
	}
	processOutput = &stdout
	delete(env, "PYXIS_LOCK_RECORD_RUN")
	if err := run(t.Context()); err != nil || called != 2 {
		t.Fatalf("ordinary gate error=%v calls=%d", err, called)
	}
	env["PYXIS_LOCK_RECORD_RUN"] = "1"
	delete(env, "GH_TOKEN")
	if err := run(t.Context()); err == nil || !strings.Contains(err.Error(), "GH_TOKEN") || called != 2 {
		t.Fatalf("missing token error=%v calls=%d", err, called)
	}
	env["GH_TOKEN"] = "test-token"
	processRecordRange = func(context.Context, string, config.Config, string, string, string, approval.Port, lifecycle.RunCommand) ([]lock.LockRecord, error) {
		return nil, errors.New("cannot read mainline")
	}
	if err := run(t.Context()); err == nil || !strings.Contains(err.Error(), "cannot read mainline") {
		t.Fatalf("range error=%v", err)
	}
	processGetwd = func() (string, error) { return "", errors.New("no checkout") }
	if err := run(t.Context()); err == nil || !strings.Contains(err.Error(), "locate checkout") {
		t.Fatalf("working directory error=%v", err)
	}
	processGetwd = func() (string, error) { return root, nil }
	if err := os.MkdirAll(filepath.Join(root, ".pyxis"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".pyxis", "config.yaml"), []byte("invalid config\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := run(t.Context()); err == nil || !strings.Contains(err.Error(), "invalid pyxis config") {
		t.Fatalf("config error=%v", err)
	}
}
