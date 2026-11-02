package main

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Exercise the actual composition root with native HTTP adapters and real Git,
// using separate code/task servers with identical repository names. No agent
// launch is needed for an empty frontier, and no gh process may be invoked.
func TestForgejoRunUsesIndependentBindingsWithoutGitHub(t *testing.T) {
	fixture := newImplementNowE2EFixture(t, e2eFixtureOptions{HarnessMode: e2eHarnessImplement})
	codeReads, taskReads := 0, 0
	server := func(token string, count *int, codeHost bool) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			*count++
			if r.Header.Get("Authorization") != "token "+token || r.Method != "GET" {
				t.Errorf("wrong binding authentication or write: %s %s", r.Method, r.URL)
				w.WriteHeader(403)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Path {
			case "/api/v1/user":
				if !codeHost {
					t.Error("task identity resolved against code actor")
				}
				if _, err := fmt.Fprint(w, `{"id":2,"login":"forgejo-worker"}`); err != nil {
					t.Error(err)
				}
			case "/api/v1/repos/owner/repo/pulls":
				if !codeHost {
					t.Error("proposals read from task host")
				}
				if _, err := fmt.Fprint(w, "[]"); err != nil {
					t.Error(err)
				}
			case "/api/v1/repos/owner/repo/issues":
				if codeHost {
					t.Error("tasks read from code host")
				}
				if _, err := fmt.Fprint(w, "[]"); err != nil {
					t.Error(err)
				}
			default:
				t.Errorf("unexpected API read %s", r.URL)
				w.WriteHeader(404)
			}
		}))
	}
	codeServer := server("code-token", &codeReads, true)
	defer codeServer.Close()
	taskServer := server("task-token", &taskReads, false)
	defer taskServer.Close()
	t.Setenv("FORGEJO_CODE_TEST_TOKEN", "code-token")
	t.Setenv("FORGEJO_TASK_TEST_TOKEN", "task-token")
	path := filepath.Join(fixture.repo, ".pyxis", "config.yaml")
	original, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	bindings := fmt.Sprintf("\nintegrations:\n  schema_version: pyxis-integrations/v1\n  connections:\n    code:\n      provider: forgejo\n      base_url: %s\n      token_env: FORGEJO_CODE_TEST_TOKEN\n    tasks:\n      provider: forgejo\n      base_url: %s\n      token_env: FORGEJO_TASK_TEST_TOKEN\n  bindings:\n    code_host:\n      connection: code\n      repository: owner/repo\n    tasks:\n      connection: tasks\n      repository: owner/repo\n  ci:\n    mode: both\n", codeServer.URL, taskServer.URL)
	if err := os.WriteFile(path, append(original, []byte(bindings)...), 0600); err != nil {
		t.Fatal(err)
	}
	for _, scope := range []string{"", "feature/001"} {
		var stdout, stderr bytes.Buffer
		args := fixture.args()
		if scope != "" {
			args = append(args, "--plan-ref", scope)
		}
		if exit := run(args, &stdout, &stderr); exit != 0 || !strings.Contains(stdout.String(), "nothing ready") {
			t.Fatalf("exit=%d stdout=%s stderr=%s", exit, &stdout, &stderr)
		}
	}
	if codeReads == 0 || taskReads == 0 {
		t.Fatalf("binding reads code=%d tasks=%d", codeReads, taskReads)
	}
	if _, err := os.Stat(filepath.Join(fixture.stateDir, "calls.jsonl")); !os.IsNotExist(err) {
		for _, call := range readE2ECalls(t, fixture.stateDir) {
			if call.Program == "gh" {
				t.Fatalf("Forgejo-only run invoked gh: %v", call.Args)
			}
		}
	}
}

func TestForgejoRunRefusesInvalidConfigurationBeforeDispatch(t *testing.T) {
	for _, body := range []string{"integrations:\n  schema_version: unknown\n", "integrations:\n  schema_version: pyxis-integrations/v1\n  connections:\n    local:\n      provider: forgejo\n      base_url: http://127.0.0.1:3000\n  bindings:\n    code_host:\n      connection: local\n      repository: owner/repo\n"} {
		t.Run(body, func(t *testing.T) {
			root := t.TempDir()
			if err := os.Mkdir(filepath.Join(root, ".pyxis"), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, ".pyxis", "config.yaml"), []byte(body), 0600); err != nil {
				t.Fatal(err)
			}
			var out, diagnostic bytes.Buffer
			if exit := run([]string{"--repo", root}, &out, &diagnostic); exit != 2 || !strings.Contains(diagnostic.String(), "integrations") {
				t.Fatalf("exit=%d err=%s", exit, &diagnostic)
			}
		})
	}
}

func TestRunSeparatesGitHubIdentitiesForExplicitBindings(t *testing.T) {
	fixture := newImplementNowE2EFixture(t, e2eFixtureOptions{})
	fixture.cfg.GHTokenRoles = map[string]string{"code-only-token": "code", "task-only-token": "tasks"}
	writeJSONFile(t, fixture.config, fixture.cfg)
	t.Setenv("PYXIS_TEST_CODE_PAT", "code-only-token")
	t.Setenv("PYXIS_TEST_TASK_PAT", "task-only-token")
	t.Setenv("GH_HOST", "wrong.example.invalid")
	writeIntegrationRunConfig(t, fixture.repo, `integrations:
  schema_version: pyxis-integrations/v1
  connections:
    code:
      provider: github
      token_env: PYXIS_TEST_CODE_PAT
    tasks:
      provider: github
      token_env: PYXIS_TEST_TASK_PAT
  bindings:
    code_host:
      connection: code
      repository: owner/code
    tasks:
      connection: tasks
      repository: owner/tasks
`)
	var out, diagnostic bytes.Buffer
	// Stop at legacy-claim admission against the missing Git remote. Both identities
	// must already have been resolved with their own token and public host.
	exit := run(append(fixture.args(), "--remote", "missing"), &out, &diagnostic)
	if exit != 2 || !strings.Contains(diagnostic.String(), "inspect legacy claims") {
		t.Fatalf("exit=%d diagnostic=%s", exit, &diagnostic)
	}
	roles := map[string]bool{}
	for _, call := range readE2ECalls(t, fixture.stateDir) {
		if call.Program == "gh" {
			roles[call.CredentialRole] = true
			if !equalArgs(call.Args, "api", "--hostname", "github.com", "user", "-q", ".login") {
				t.Fatalf("unexpected operation before workspace admission: %v", call.Args)
			}
		}
	}
	if len(roles) != 2 || !roles["code"] || !roles["tasks"] {
		t.Fatalf("resolved credential roles=%v", roles)
	}
}

func TestForgejoRunFailsClosedAtAuthenticationAndTrackerBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, codeToken, taskToken string
		denyActor                  bool
		want                       string
	}{
		{name: "invalid code token", codeToken: "invalid\nheader", taskToken: "tasks", want: "Forgejo: forgejo token"},
		{name: "actor denied", codeToken: "code", taskToken: "tasks", denyActor: true, want: "authenticate Forgejo actor"},
		{name: "invalid task token", codeToken: "code", taskToken: "invalid\nheader", want: "task tracker: forgejo token"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := newImplementNowE2EFixture(t, e2eFixtureOptions{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "GET" || r.URL.Path != "/api/v1/user" || r.Header.Get("Authorization") != "token code" {
					t.Errorf("unexpected request before admission: %s %s", r.Method, r.URL)
					w.WriteHeader(500)
					return
				}
				if tc.denyActor {
					w.WriteHeader(403)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				if _, err := fmt.Fprint(w, `{"id":2,"login":"worker"}`); err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()
			t.Setenv("PYXIS_TEST_CODE_PAT", tc.codeToken)
			t.Setenv("PYXIS_TEST_TASK_PAT", tc.taskToken)
			writeIntegrationRunConfig(t, fixture.repo, fmt.Sprintf(`integrations:
  schema_version: pyxis-integrations/v1
  connections:
    code:
      provider: forgejo
      base_url: %s
      token_env: PYXIS_TEST_CODE_PAT
    tasks:
      provider: forgejo
      base_url: %s
      token_env: PYXIS_TEST_TASK_PAT
  bindings:
    code_host:
      connection: code
      repository: owner/code
    tasks:
      connection: tasks
      repository: owner/tasks
`, server.URL, server.URL))
			var out, diagnostic bytes.Buffer
			if exit := run(fixture.args(), &out, &diagnostic); exit != 2 || !strings.Contains(diagnostic.String(), tc.want) {
				t.Fatalf("exit=%d diagnostic=%s", exit, &diagnostic)
			}
			if strings.Contains(diagnostic.String(), tc.codeToken) && tc.codeToken != "code" || strings.Contains(diagnostic.String(), tc.taskToken) && tc.taskToken != "tasks" {
				t.Fatal("diagnostic exposed malformed credentials")
			}
		})
	}
}

func writeIntegrationRunConfig(t *testing.T, root, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, ".pyxis", "config.yaml"), []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestRunRejectsInvalidLegacyRepositoryCoordinate(t *testing.T) {
	var out, diagnostic bytes.Buffer
	if exit := run([]string{"--repo", t.TempDir(), "--repo-slug", "owner/repo/extra"}, &out, &diagnostic); exit != 2 || !strings.Contains(diagnostic.String(), "code host:") {
		t.Fatalf("exit=%d diagnostic=%s", exit, &diagnostic)
	}
}

func TestRunUsesGitHubTaskIdentityWithForgejoCodeHost(t *testing.T) {
	fixture := newImplementNowE2EFixture(t, e2eFixtureOptions{})
	fixture.cfg.GHTokenRoles = map[string]string{"task-only-token": "tasks"}
	writeJSONFile(t, fixture.config, fixture.cfg)
	t.Setenv("PYXIS_TEST_TASK_PAT", "task-only-token")
	t.Setenv("PYXIS_TEST_CODE_PAT", "code-only-token")
	t.Setenv("GH_HOST", "wrong.example.invalid")
	actorReads := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/api/v1/user" || r.Header.Get("Authorization") != "token code-only-token" {
			t.Errorf("unexpected Forgejo operation: %s %s", r.Method, r.URL)
			w.WriteHeader(500)
			return
		}
		actorReads++
		w.Header().Set("Content-Type", "application/json")
		if _, err := fmt.Fprint(w, `{"id":2,"login":"code-worker"}`); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	writeIntegrationRunConfig(t, fixture.repo, fmt.Sprintf(`integrations:
  schema_version: pyxis-integrations/v1
  connections:
    code:
      provider: forgejo
      base_url: %s
      token_env: PYXIS_TEST_CODE_PAT
    tasks:
      provider: github
      token_env: PYXIS_TEST_TASK_PAT
  bindings:
    code_host:
      connection: code
      repository: owner/code
    tasks:
      connection: tasks
      repository: owner/tasks
`, server.URL))
	var out, diagnostic bytes.Buffer
	if exit := run(append(fixture.args(), "--remote", "missing"), &out, &diagnostic); exit != 2 || !strings.Contains(diagnostic.String(), "inspect legacy claims") {
		t.Fatalf("exit=%d diagnostic=%s", exit, &diagnostic)
	}
	calls := readE2ECalls(t, fixture.stateDir)
	if actorReads != 1 || len(calls) != 1 || calls[0].CredentialRole != "tasks" || !equalArgs(calls[0].Args, "api", "--hostname", "github.com", "user", "-q", ".login") {
		t.Fatalf("identity routing actor reads=%d calls=%+v", actorReads, calls)
	}
}
