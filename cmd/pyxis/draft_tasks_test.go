package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/adapters/common/forgejo"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/worktracker/model"
)

func taskBindingRoot(t *testing.T, integrationsBlock string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".pyxis"), 0o755); err != nil {
		t.Fatal(err)
	}
	if integrationsBlock != "" {
		if err := os.WriteFile(filepath.Join(root, ".pyxis/config.yaml"), []byte(integrationsBlock), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestConfiguredTaskReaderFollowsTheTaskBinding(t *testing.T) {
	if tasks, err := configuredTaskReader(taskBindingRoot(t, ""), config.Defaults()); err != nil || tasks != nil {
		t.Fatalf("default GitHub tasks: reader=%v err=%v", tasks, err)
	}
	carina := config.Defaults()
	carina.Knobs.Tracker = "carina"
	if tasks, err := configuredTaskReader(taskBindingRoot(t, ""), carina); err != nil {
		t.Fatal(err)
	} else if c, ok := tasks.(carinaTasks); !ok || !c.AppendOnly() {
		t.Fatalf("Carina tasks: reader=%T", tasks)
	}
	unsupported := config.Defaults()
	unsupported.Knobs.Tracker = "jira"
	if _, err := configuredTaskReader(taskBindingRoot(t, ""), unsupported); err == nil {
		t.Fatal("an unsupported tracker was accepted")
	}
	if _, err := configuredTaskReader(taskBindingRoot(t, "integrations: [\n"), config.Defaults()); err == nil {
		t.Fatal("an unreadable integrations block was accepted")
	}
	broken := taskBindingRoot(t, "")
	if err := os.WriteFile(filepath.Join(broken, ".pyxis/config.yaml"), []byte("work_items: [\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := configuredTaskReader(broken, carina); err == nil {
		t.Fatal("an unreadable Carina operand was accepted")
	}
}

const forgejoTaskBinding = `integrations:
  schema_version: pyxis-integrations/v1
  connections:
    local:
      provider: forgejo
      base_url: http://127.0.0.1:3000/base
  bindings:
    tasks:
      connection: local
      repository: o/r
`

func TestConfiguredTaskReaderListsEveryForgejoIssue(t *testing.T) {
	root := taskBindingRoot(t, forgejoTaskBinding)
	old := integrationForgejoClient
	t.Cleanup(func() { integrationForgejoClient = old })
	fail := false
	integrationForgejoClient = func(base, token string) (*forgejo.Client, error) {
		return forgejo.NewWithHTTP(base, token, &http.Client{Transport: watchRoundTrip(func(req *http.Request) (*http.Response, error) {
			if req.URL.Path != "/base/api/v1/repos/o/r/issues" || req.URL.Query().Get("state") != "all" {
				t.Fatalf("unexpected request %s", req.URL)
			}
			body := "[]"
			if fail {
				return &http.Response{StatusCode: 500, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("{}")), Request: req}, nil
			}
			if req.URL.Query().Get("page") == "1" {
				body = `[{"number":4,"state":"open","body":"<!-- pyxis:plan-task widget/001/T1 -->"},{"number":5,"state":"closed","body":"<!-- pyxis:plan-task widget/001/T2 -->"}]`
			}
			return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
		})})
	}
	tasks, err := configuredTaskReader(root, config.Defaults())
	if err != nil {
		t.Fatal(err)
	}
	items, defects := tasks.List(context.Background(), model.Query{})
	if len(defects) != 0 || len(items) != 2 || items[0].ID != 4 || items[0].State != model.StateOpen || items[1].State != model.StateClosed || !strings.Contains(items[1].Body, "widget/001/T2") {
		t.Fatalf("items=%+v defects=%v", items, defects)
	}
	fail = true
	if _, defects := tasks.List(context.Background(), model.Query{}); len(defects) == 0 {
		t.Fatal("a failed listing reported no defect")
	}
	integrationForgejoClient = func(string, string) (*forgejo.Client, error) { return nil, errors.New("bad client") }
	if _, err := configuredTaskReader(root, config.Defaults()); err == nil {
		t.Fatal("a bad Forgejo client was accepted")
	}
}

func TestDraftVerbsStopWhenTheTaskTrackerCannotBeRead(t *testing.T) {
	root := taskBindingRoot(t, forgejoTaskBinding)
	oldRoot, oldClient := draftRepoRoot, integrationForgejoClient
	t.Cleanup(func() { draftRepoRoot, integrationForgejoClient = oldRoot, oldClient })
	draftRepoRoot = func() (string, error) { return root, nil }
	integrationForgejoClient = func(string, string) (*forgejo.Client, error) { return nil, errors.New("bad client") }
	for _, args := range [][]string{{"prepare-lock", "widget"}, {"reopen", "widget"}} {
		var out, errout bytes.Buffer
		if code := draftVerb(args, &out, &errout); code != 1 || !strings.Contains(errout.String(), "task tracker: bad client") {
			t.Fatalf("%v: exit %d stderr %q", args, code, errout.String())
		}
	}
}

func TestConfiguredTaskReaderReadsAnExplicitGitHubBinding(t *testing.T) {
	root := taskBindingRoot(t, "integrations:\n  schema_version: pyxis-integrations/v1\n  bindings:\n    tasks:\n      provider: github\n      repository: o/tasks\n")
	old := integrationCommandOutput
	t.Cleanup(func() { integrationCommandOutput = old })
	out, fail := `[{"number":3,"state":"closed","body":"<!-- pyxis:plan-task widget/001/T1 -->"},{"number":4,"state":"open","pull_request":{}}][{"number":5,"state":"open","body":"b"}]`, error(nil)
	integrationCommandOutput = func(cmd *exec.Cmd) ([]byte, error) {
		if args := strings.Join(cmd.Args, " "); !strings.Contains(args, "repos/o/tasks/issues?state=all") || cmd.Dir != root {
			t.Fatalf("wrong repository read: %s in %s", args, cmd.Dir)
		}
		return []byte(out), fail
	}
	tasks, err := configuredTaskReader(root, config.Defaults())
	if err != nil {
		t.Fatal(err)
	}
	items, defects := tasks.List(context.Background(), model.Query{})
	if len(defects) != 0 || len(items) != 2 || items[0].ID != 3 || items[0].State != model.StateClosed || items[1].ID != 5 || items[1].State != model.StateOpen {
		t.Fatalf("items=%+v defects=%v", items, defects)
	}
	out = "[{"
	if _, defects := tasks.List(context.Background(), model.Query{}); len(defects) == 0 {
		t.Fatal("an unreadable listing reported no defect")
	}
	fail = errors.New("denied")
	if _, defects := tasks.List(context.Background(), model.Query{}); len(defects) == 0 {
		t.Fatal("a failed listing reported no defect")
	}
}
