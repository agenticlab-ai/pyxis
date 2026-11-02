package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/codegatemirror"
	"github.com/agenticlab-ai/pyxis/internal/executor/config"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/report"
	"github.com/agenticlab-ai/pyxis/internal/integrations"
)

func TestGreenFactoryRetainsIssueObserverDuringTruncatedGate(t *testing.T) {
	repo := implementReviewGitFixture(t)
	if err := os.MkdirAll(filepath.Join(repo, ".pyxis"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".pyxis", "config.yaml"), []byte("code-gate:\n  required:\n    command: false\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runRoutingGit(t, repo, "add", ".pyxis/config.yaml")
	runRoutingGit(t, repo, "commit", "-qm", "test: truncated required local gate")
	emitter := &capturingEmitter{}
	driver, err := newGreenFactory(coverageGreenHost{}, config.Defaults(), emitter, "origin")(coverageAgentRuntime{})
	if err != nil {
		t.Fatal(err)
	}
	task := port.Task{ID: "owned-issue", Number: 17}
	ctx := codegatemirror.WithProgressObserver(t.Context(), func(progress codegatemirror.Progress) {
		emitter.Emit(report.Event{Kind: report.KindGateCheckProgress, TaskID: task.ID, Issue: task.Number, Phase: progress.Name, Status: progress.Status})
	})
	green, err := driver.GateGreen(ctx, &healWorkdir{dir: repo, branch: "task"})
	if err != nil || green {
		t.Fatalf("truncated failing gate changed its verdict: green=%v err=%v", green, err)
	}
	var statuses []string
	for _, event := range emitter.events {
		if event.Kind != report.KindGateCheckProgress {
			continue
		}
		if event.TaskID != task.ID || event.Issue != task.Number || event.Phase != "required" {
			t.Fatalf("truncated gate replaced the owning issue observer: %+v", event)
		}
		statuses = append(statuses, event.Status)
	}
	if strings.Join(statuses, ",") != "started,failed" {
		t.Fatalf("truncated gate lost or duplicated progress: %v", statuses)
	}
}

func TestDirectIssueScratchOutagePreventsResumedNativeExecution(t *testing.T) {
	f := directIssueFixture(t)
	binding := integrations.ResolvedRepository{Provider: integrations.GitHub, BaseURL: "https://github.com", Repository: f.cfg.Repository}
	snapshot := port.IssueSnapshot{Number: f.cfg.IssueNumber, Open: true, Executable: true, Title: f.cfg.TaskTitle, Body: f.cfg.TaskBody}
	input := filepath.Join(f.root, "issue-input.json")
	writeJSONFile(t, input, issueCharter(t, binding, snapshot))
	now := func() time.Time { return time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC) }
	if _, _, err := admitIssue(t.Context(), &issueReadFixture{snapshot: snapshot}, binding, snapshot.Number, input, f.repo, "origin", now(), now); err != nil {
		t.Fatal(err)
	}

	t.Setenv("TMPDIR", filepath.Join(f.root, "missing-scratch"))
	var out, diagnostic bytes.Buffer
	args := append(f.args(), "--issue", strconv.Itoa(snapshot.Number), "--issue-input", input)
	if code := run(args, &out, &diagnostic); code != 2 || !strings.Contains(diagnostic.String(), "issue verification boundary") {
		t.Fatalf("scratch refusal: exit %d, %s %s", code, &out, &diagnostic)
	}
	calls := readE2ECalls(t, f.stateDir)
	if len(callsFor(calls, "codex", "exec")) != 0 || len(githubCalls(calls, "POST", "repos/"+f.cfg.Repository+"/pulls")) != 0 {
		t.Fatal("scratch outage launched inference or published a proposal")
	}
	if refs := f.gitBare("for-each-ref", "--format=%(refname)", "refs/claim/"); refs != "" {
		t.Fatalf("scratch outage acquired a claim: %s", refs)
	}
}

func TestGreenFactoryBindsProgressAndStopsUnchangedGitFixer(t *testing.T) {
	repo := implementReviewGitFixture(t)
	if err := os.MkdirAll(filepath.Join(repo, ".pyxis"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".pyxis", "config.yaml"), []byte("code-gate:\n  required:\n    command: false\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runRoutingGit(t, repo, "add", ".pyxis/config.yaml")
	runRoutingGit(t, repo, "commit", "-qm", "test: required local gate")
	emitter := &capturingEmitter{}
	driver, err := newGreenFactory(coverageGreenHost{}, config.Defaults(), emitter, "origin")(coverageAgentRuntime{})
	if err != nil {
		t.Fatal(err)
	}
	task := port.Task{ID: "owned-task", Number: 17}
	if _, err := driver.DriveLocal(t.Context(), task, &healWorkdir{dir: repo, branch: "task"}); err == nil || !strings.Contains(err.Error(), "made no progress") {
		t.Fatalf("unchanged fixer replayed verification: %v", err)
	}
	var statuses []string
	for _, event := range emitter.events {
		if event.Kind == report.KindGateCheckProgress {
			if event.TaskID != task.ID || event.Issue != task.Number || event.Phase != "required" {
				t.Fatalf("progress lost its owning task/check: %+v", event)
			}
			statuses = append(statuses, event.Status)
		}
	}
	if strings.Join(statuses, ",") != "started,failed" {
		t.Fatalf("unchanged fixer replayed or lost gate progress: %v", statuses)
	}
}
