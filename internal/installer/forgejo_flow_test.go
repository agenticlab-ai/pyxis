package installer_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/adapters/common/forgejo"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/installer"
	"github.com/agenticlab-ai/pyxis/internal/integrations"
)

type initForgejoTransport func(*http.Request) (*http.Response, error)

func (f initForgejoTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func forgejoInitOptions(t *testing.T, target string) (installer.Options, *int) {
	t.Helper()
	r := newRelease(t)
	opts := r.carinaOptions(target)
	profile := integrations.Config{SchemaVersion: integrations.SchemaVersion, Connections: map[string]integrations.Connection{"local": {Provider: "forgejo", BaseURL: "http://127.0.0.1:3000", TokenEnv: "TEST_INIT_NATIVE_TOKEN"}}, CI: integrations.CI{Mode: "local"}}
	profile.Bindings.Tasks = integrations.Binding{Connection: "local", Repository: "owner/repository"}
	profile.Bindings.CodeHost = profile.Bindings.Tasks
	opts.Integrations = profile
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	opts.CredentialExecutable = executable // explicit executable seam, never installed outside this fixture
	writes := new(int)
	var issues []struct {
		Number int    `json:"number"`
		Body   string `json:"body"`
		URL    string `json:"html_url"`
	}
	opts.ForgejoClientFactory = func(bound integrations.ResolvedRepository) (*forgejo.Client, error) {
		return forgejo.NewWithHTTP(bound.BaseURL, "init-transport-test-token", &http.Client{Transport: initForgejoTransport(func(req *http.Request) (*http.Response, error) {
			status, body := 200, `{}`
			header := http.Header{}
			switch req.URL.Path {
			case "/api/v1/user":
				body = `{"id":1,"login":"assistant"}`
			case "/api/v1/repos/owner/repository":
				body = `{"id":2}`
			case "/api/v1/repos/owner/repository/labels":
				if req.Method == http.MethodGet {
					body = `[]`
					header.Set("X-Total-Count", "0")
				} else {
					*writes++
					status = 201
					body = `{"id":1}`
				}
			case "/api/v1/repos/owner/repository/issues":
				if req.Method == http.MethodGet {
					if issues == nil {
						body = `[]`
					} else {
						b, err := json.Marshal(issues)
						if err != nil {
							t.Fatal(err)
						}
						body = string(b)
					}
					header.Set("X-Total-Count", strconv.Itoa(len(issues)))
				} else {
					*writes++
					var created struct {
						Number int    `json:"number"`
						Body   string `json:"body"`
						URL    string `json:"html_url"`
					}
					if err := json.NewDecoder(req.Body).Decode(&created); err != nil {
						t.Fatal(err)
					}
					created.Number = len(issues) + 1
					created.URL = "http://127.0.0.1:3000/owner/repository/issues/" + strconv.Itoa(created.Number)
					issues = append(issues, created)
					b, err := json.Marshal(created)
					if err != nil {
						t.Fatal(err)
					}
					body = string(b)
					status = 201
				}
			default:
				t.Fatalf("unexpected Forgejo init request %s %s", req.Method, req.URL)
			}
			return &http.Response{StatusCode: status, Header: header, Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
		})})
	}
	return opts, writes
}

func TestForgejoInitNativeFlowAndCredentialRollback(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(strconv.FormatBool(fail), func(t *testing.T) {
			isolateGit(t)
			ambientIdentityConfig(t, "Ada", "ada@example.com")
			t.Setenv("TEST_INIT_NATIVE_TOKEN", "never-store-init-secret")
			target := t.TempDir()
			git(t, target, "init", "-b", "main")
			key := "credential.http://127.0.0.1:3000/owner/repository.git.helper"
			git(t, target, "config", "--add", key, "operator-helper")
			opts, writes := forgejoInitOptions(t, target)
			opts.Answers.RemoteURL = "" // the integration binding owns publication
			if fail {
				restore := installer.SetFailingStageForTest("green-proof")
				defer restore()
			}
			report, err := installer.Init(context.Background(), opts)
			if fail {
				if err == nil || report.Rollback == nil || !report.Rollback.Restored {
					t.Fatalf("rollback %#v err %v", report.Rollback, err)
				}
				if got := git(t, target, "config", "--get-all", key); strings.TrimSpace(got) != "operator-helper" {
					t.Fatalf("operator helper not restored: %q", got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if *writes == 0 {
				t.Fatal("native wiring was bypassed")
			}
			b, err := os.ReadFile(filepath.Join(target, ".pyxis", "config.yaml"))
			if err != nil {
				t.Fatal(err)
			}
			cfg, diagnostics := config.Parse(b)
			if len(diagnostics) != 0 || cfg.Knobs.Tracker != "forgejo" || cfg.Epoch.Commit == "" {
				t.Fatalf("installed config did not retain native tracker/epoch: %v", diagnostics)
			}
			if profile, err := integrations.Load(target); err != nil || profile.Bindings.CodeHost.Repository != "owner/repository" {
				t.Fatalf("installed binding: %#v %v", profile, err)
			}
			if got := git(t, target, "remote", "get-url", "origin"); strings.TrimSpace(got) != "http://127.0.0.1:3000/owner/repository.git" {
				t.Fatalf("remote %q", got)
			}
			gitConfig, err := os.ReadFile(filepath.Join(target, ".git", "config"))
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(gitConfig), "never-store-init-secret") || strings.Contains(string(b), "never-store-init-secret") {
				t.Fatal("native secret persisted")
			}
			if !strings.Contains(string(gitConfig), "credential --repo") {
				t.Fatal("native Git helper not registered")
			}
		})
	}
}
