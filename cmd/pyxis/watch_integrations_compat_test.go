package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	repoconfig "github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/integrations"
	"github.com/agenticlab-ai/pyxis/internal/watch"
	"github.com/agenticlab-ai/pyxis/internal/worktracker/localfs"
	"github.com/agenticlab-ai/pyxis/internal/worktracker/model"
)

func TestIntegrationGitHubExactReadsAndCompletion(t *testing.T) {
	old := integrationCommandOutput
	t.Cleanup(func() { integrationCommandOutput = old })
	t.Setenv("WATCH_SELECTED_GH_TOKEN", "selected-token")
	t.Setenv("GH_TOKEN", "wrong-ambient")
	t.Setenv("GITHUB_TOKEN", "wrong-ambient")
	binding := integrations.ResolvedRepository{Provider: integrations.GitHub, BaseURL: "https://github.com", Repository: "o/r", TokenEnv: "WATCH_SELECTED_GH_TOKEN"}
	state := "open"
	list, detail := `[{"number":7,"state":"closed"},{"number":8,"state":"open"}][]`, `{"number":7,"state":"closed","merged":true,"merge_commit_sha":"merged"}`
	var fail error
	mutate := true
	badIssue := false
	integrationCommandOutput = func(cmd *exec.Cmd) ([]byte, error) {
		if cmd.Dir != "root" || !strings.Contains(strings.Join(cmd.Args, " "), "api --hostname github.com") {
			t.Fatalf("wrong exact-read command %v", cmd.Args)
		}
		env := strings.Join(cmd.Env, "\n")
		if !strings.Contains(env, "GH_TOKEN=selected-token") || strings.Contains(env, "wrong-ambient") {
			t.Fatal("selected task credential leaked to ambient GitHub account")
		}
		if fail != nil {
			return nil, fail
		}
		args := strings.Join(cmd.Args, " ")
		switch {
		case strings.Contains(args, "issues/1"):
			if strings.Contains(args, "PATCH") && mutate {
				state = "closed"
			}
			if badIssue {
				return []byte(`{"number":2,"state":"closed"}`), nil
			}
			return []byte(`{"number":1,"state":"` + state + `"}`), nil
		case strings.Contains(args, "pulls?state=all"):
			return []byte(list), nil
		case strings.Contains(args, "pulls/7"):
			return []byte(detail), nil
		default:
			t.Fatalf("unexpected API command %v", cmd.Args)
			return nil, nil
		}
	}
	mapping := planMap{issueByTask: map[string]int{"T1": 1}}
	states, err := readIntegrationTasks(t.Context(), "root", binding, mapping)
	if err != nil || states[1] {
		t.Fatalf("open task=%v %v", states, err)
	}
	if err := closeIntegrationTask(t.Context(), "root", binding, 1); err != nil || state != "closed" {
		t.Fatalf("completion=%s %v", state, err)
	}
	state, mutate = "open", false
	if err := closeIntegrationTask(t.Context(), "root", binding, 1); err == nil {
		t.Fatal("unconfirmed close accepted")
	}
	badIssue = true
	if _, err := readIntegrationTasks(t.Context(), "root", binding, mapping); err == nil {
		t.Fatal("wrong exact task accepted")
	}
	if err := closeIntegrationTask(t.Context(), "root", binding, 1); err == nil {
		t.Fatal("wrong close readback accepted")
	}
	badIssue = false
	pulls, err := readIntegrationProposals(t.Context(), "root", binding)
	if err != nil || len(pulls) != 2 || !pulls[0].Merged || pulls[1].Number != 8 {
		t.Fatalf("definitive merge details=%+v %v", pulls, err)
	}
	detail = `{"number":9}`
	if _, err := readIntegrationProposals(t.Context(), "root", binding); err == nil {
		t.Fatal("wrong exact proposal accepted")
	}
	list = "{"
	if _, err := readIntegrationProposals(t.Context(), "root", binding); err == nil {
		t.Fatal("malformed proposal list accepted")
	}
	fail = errors.New("offline")
	if _, err := readIntegrationTasks(t.Context(), "root", binding, mapping); !errors.Is(err, fail) {
		t.Fatal("task read failure hidden")
	}
	if _, err := readIntegrationProposals(t.Context(), "root", binding); !errors.Is(err, fail) {
		t.Fatal("proposal read failure hidden")
	}
	if err := closeIntegrationTask(t.Context(), "root", binding, 1); !errors.Is(err, fail) {
		t.Fatal("task write failure hidden")
	}
	t.Setenv("WATCH_SELECTED_GH_TOKEN", "")
	if _, err := integrationGitHub(t.Context(), "root", binding, "repos/o/r"); err == nil {
		t.Fatal("missing selected token fell back to ambient auth")
	}
	if _, err := integrationGitHub(t.Context(), "root", integrations.ResolvedRepository{BaseURL: "invalid"}); err == nil {
		t.Fatal("invalid host accepted")
	}
}

func TestIntegrationCarinaCompletionAndUnknownStates(t *testing.T) {
	root := t.TempDir()
	store := localfs.New(root, time.Now)
	item, defects := store.Create(t.Context(), model.CreateSpec{Title: "[T1] task"})
	if len(defects) > 0 {
		t.Fatal(defects)
	}
	binding := integrations.ResolvedRepository{Provider: integrations.Carina, Root: root}
	mapping := planMap{issueByTask: map[string]int{"T1": item.ID}}
	states, err := readIntegrationTasks(t.Context(), "repo", binding, mapping)
	if err != nil || states[item.ID] {
		t.Fatalf("open Carina=%v %v", states, err)
	}
	if err := closeIntegrationTask(t.Context(), "repo", binding, item.ID); err != nil {
		t.Fatal(err)
	}
	closed, defects := store.Get(t.Context(), item.ID)
	if len(defects) > 0 || closed.State != model.StateClosed {
		t.Fatalf("real Carina closure=%+v %v", closed, defects)
	}
	if err := closeIntegrationTask(t.Context(), "repo", binding, 999); err == nil {
		t.Fatal("nonexistent Carina close accepted")
	}
	if _, err := readIntegrationTasks(t.Context(), "repo", binding, planMap{issueByTask: map[string]int{"missing": 999}}); err == nil {
		t.Fatal("unknown Carina task became closed")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := readIntegrationTasks(ctx, "repo", binding, mapping); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled task read ignored")
	}
	if _, err := readIntegrationTasks(t.Context(), "repo", integrations.ResolvedRepository{Provider: "other"}, mapping); err == nil {
		t.Fatal("invalid task provider accepted")
	}
	if err := closeIntegrationTask(t.Context(), "repo", integrations.ResolvedRepository{Provider: "other"}, 1); err == nil {
		t.Fatal("invalid close provider accepted")
	}
}

func TestConfiguredWatchSelectionErrors(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".pyxis"), 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, ".pyxis/config.yaml")
	write := func(body string) {
		t.Helper()
		body = strings.Replace(body, "integrations:\n", "integrations:\n  schema_version: pyxis-integrations/v1\n", 1)
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("integrations:\n  unexpected: true\n")
	if _, _, _, _, err := configuredWatch(root, "o/r", "gh", root, repoconfig.Defaults(), planMap{}, false); err == nil {
		t.Fatal("invalid config accepted")
	}
	write("integrations:\n  bindings:\n    code_host:\n      provider: github\n      repository: o/r\n")
	if _, _, _, _, err := configuredWatch(root, "o/r", "invalid", root, repoconfig.Defaults(), planMap{}, false); err == nil {
		t.Fatal("invalid legacy task binding accepted")
	}
	reader, source, medium, enabled, err := configuredWatch(root, "o/r", "gh", root, repoconfig.Defaults(), planMap{}, false)
	if err != nil || !enabled || source == nil || medium != integrations.GitHub || reader.Cadence(t.Context()) <= 0 {
		t.Fatalf("GH reader=%+v %s %v %v", reader, medium, enabled, err)
	}
	write("integrations:\n  bindings:\n    tasks:\n      provider: carina\n")
	if _, _, _, _, err := configuredWatch(root, "invalid-repo", "gh", root, repoconfig.Defaults(), planMap{}, false); err == nil {
		t.Fatal("invalid legacy code binding accepted")
	}
}

func TestWatchConfiguredCodeHostNeedsNoLegacyRepoSlug(t *testing.T) {
	repo := reviewRearmRepo(t, "integrations:\n  schema_version: pyxis-integrations/v1\n  bindings:\n    code_host:\n      provider: github\n      repository: configured/repo\n")
	writePlanJSON(t, repo)
	report := fakeReport(t, repo, `{"task_id":"T1","issue":1}`)
	oldRun := watchRunLive
	t.Cleanup(func() { watchRunLive = oldRun })
	watchRunLive = func(context.Context, watch.PlanSource, watch.Reader, watch.Sweep, watch.Clock, <-chan struct{}) (watch.Summary, error) {
		return watch.Summary{Completed: true}, nil
	}
	var stdout, stderr bytes.Buffer
	code := watchVerb([]string{"--repo", repo, "--plan", "docs/plans/self-drive-wake", "--tracker", "carina", "--carina-root", repo, "--report-cmd", report}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("configured watch required legacy repo: %d %s", code, stderr.String())
	}
}
