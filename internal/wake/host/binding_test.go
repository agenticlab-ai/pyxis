package host

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/integrations"
)

func TestFullURLBindings(t *testing.T) {
	cfg := integrations.Config{Connections: map[string]integrations.Connection{
		"first":  {Provider: integrations.Forgejo, BaseURL: "https://forgejo.example/base", TokenEnv: "FIRST_TOKEN"},
		"second": {Provider: integrations.Forgejo, BaseURL: "https://forgejo.example/other", TokenEnv: "SECOND_TOKEN"},
		"github": {Provider: integrations.GitHub, BaseURL: "https://github.com"},
	}}
	for _, tc := range []struct {
		ref, provider, token string
		task                 bool
	}{
		{"https://forgejo.example/base/o/r/pulls/7", integrations.Forgejo, "FIRST_TOKEN", false},
		{"https://forgejo.example/other/o/r/issues/7", integrations.Forgejo, "SECOND_TOKEN", true},
		{"https://github.com/o/r/pull/7", integrations.GitHub, "", false},
		{"https://github.com/o/r/issues/7", integrations.GitHub, "", true},
	} {
		repo, n, got, err := resolveURLBinding(cfg, integrations.Binding{}, tc.ref, tc.task)
		if err != nil || repo != "o/r" || n != "7" || got.Provider != tc.provider || got.TokenEnv != tc.token {
			t.Fatalf("%s => %s#%s %+v %v", tc.ref, repo, n, got, err)
		}
	}
	for _, ref := range []string{
		"https://unknown.example/base/o/r/pulls/7", "https://forgejo.example/base2/o/r/pulls/7",
		"https://forgejo.example/o/r/pulls/7", "https://forgejo.example/base/o/r/issues/7",
		"http://forgejo.example/base/o/r/pulls/7", "https://forgejo.example:443/base/o/r/pulls/7",
		"https://user:secret@forgejo.example/base/o/r/pulls/7", "https://forgejo.example/base/o/r/pulls/7?access_token=secret",
		"https://forgejo.example/base/o/r/pulls/7#x", "https://forgejo.example/base/o%2Fr/pulls/7",
		"https://forgejo.example/base/o/r/pulls/0", "https://forgejo.example/base/o/r/pulls/07",
		"https://forgejo.example/base/../r/pulls/7", "https://forgejo.example/base/o/r/pulls/nope",
		"https://forgejo.example/base/o/r/pulls/7/", "http://%",
	} {
		if _, _, _, err := resolveURLBinding(cfg, integrations.Binding{}, ref, false); err == nil {
			t.Errorf("unsafe/mismatched URL accepted: %s", ref)
		}
	}
	cfg.Connections["duplicate"] = cfg.Connections["first"]
	ref := "https://forgejo.example/base/o/r/pulls/7"
	if _, _, _, err := resolveURLBinding(cfg, integrations.Binding{}, ref, false); err == nil {
		t.Fatal("ambiguous connection picked nondeterministically")
	}
	if _, _, b, err := resolveURLBinding(cfg, integrations.Binding{Connection: "first"}, ref, false); err != nil || b.TokenEnv != "FIRST_TOKEN" {
		t.Fatalf("selected duplicate=%+v %v", b, err)
	}
	if _, _, b, err := resolveURLBinding(integrations.Config{}, integrations.Binding{Provider: integrations.GitHub}, "https://github.com/o/r/pull/7", false); err != nil || b.Provider != integrations.GitHub {
		t.Fatalf("provider-only binding=%+v %v", b, err)
	}
	cfg.Connections["invalid"] = integrations.Connection{Provider: "bad"}
	if _, _, _, err := resolveURLBinding(cfg, integrations.Binding{}, ref, false); err == nil {
		t.Fatal("invalid connection accepted")
	}
}

func writeWakeConfig(t *testing.T, root, base string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, ".pyxis"), 0o755); err != nil {
		t.Fatal(err)
	}
	content := fmt.Sprintf(`integrations:
  schema_version: pyxis-integrations/v1
  connections:
    code:
      provider: forgejo
      base_url: %s/code
      token_env: WAKE_CODE_TOKEN
    tasks:
      provider: forgejo
      base_url: %s/tasks
      token_env: WAKE_TASK_TOKEN
  bindings:
    code_host:
      connection: code
      repository: o/r
    tasks:
      connection: tasks
      repository: o/r
`, base, base)
	if err := os.WriteFile(filepath.Join(root, ".pyxis/config.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestNativeWakeUsesDurableHostAndExactObjects(t *testing.T) {
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Method != "GET" {
			t.Errorf("wake wrote %s", r.Method)
			w.WriteHeader(405)
			return
		}
		wantToken := "code-token"
		if strings.HasPrefix(r.URL.Path, "/tasks/") {
			wantToken = "task-token"
		}
		if r.Header.Get("Authorization") != "token "+wantToken {
			t.Error("credential crossed host binding")
			w.WriteHeader(403)
			return
		}
		var result any
		switch r.URL.Path {
		case "/tasks/api/v1/repos/o/r/issues/7":
			result = struct {
				Number int `json:"number"`
			}{7}
		case "/tasks/api/v1/repos/o/r/issues/8":
			result = struct {
				Number int `json:"number"`
			}{9}
		case "/tasks/api/v1/repos/o/r/issues/9":
			w.WriteHeader(404)
			return
		case "/tasks/api/v1/repos/o/r/issues/10":
			w.WriteHeader(403)
			return
		case "/code/api/v1/repos/o/r/pulls/7":
			result = struct {
				Merged bool   `json:"merged"`
				SHA    string `json:"merge_commit_sha"`
			}{true, "merge-sha"}
		case "/code/api/v1/repos/o/r/pulls/8":
			result = struct {
				Merged bool `json:"merged"`
			}{false}
		default:
			t.Errorf("wrong host/repo/object path %s", r.URL.Path)
			w.WriteHeader(400)
			return
		}
		if err := json.NewEncoder(w).Encode(result); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	t.Setenv("WAKE_CODE_TOKEN", "code-token")
	t.Setenv("WAKE_TASK_TOKEN", "task-token")
	root := t.TempDir()
	writeWakeConfig(t, root, server.URL)
	var gitCalls []string
	p := Prober{Root: root, GHRepo: "legacy/repo", Remote: "upstream", Branch: "trunk", Run: func(_ context.Context, name string, args ...string) ([]byte, error) {
		if name != "git" {
			t.Fatalf("Forgejo read invoked %s", name)
		}
		gitCalls = append(gitCalls, strings.Join(args, " "))
		return nil, nil
	}}
	if ok, err := p.RecordExists(t.Context(), server.URL+"/tasks/o/r/issues/7"); err != nil || !ok {
		t.Fatalf("exact task=%v %v", ok, err)
	}
	if ok, err := p.AncestorOfMainline(t.Context(), server.URL+"/code/o/r/pulls/7"); err != nil || !ok {
		t.Fatalf("merged proposal=%v %v", ok, err)
	}
	if len(gitCalls) != 2 || !strings.Contains(gitCalls[0], "upstream trunk") || !strings.Contains(gitCalls[1], "merge-sha upstream/trunk") {
		t.Fatalf("wrong ancestry proof: %v", gitCalls)
	}
	if ok, err := p.AncestorOfMainline(t.Context(), server.URL+"/code/o/r/pulls/8"); err != nil || ok {
		t.Fatalf("unmerged=%v %v", ok, err)
	}
	if ok, err := p.RecordExists(t.Context(), server.URL+"/tasks/o/r/issues/8"); err == nil || ok {
		t.Fatal("wrong issue identity accepted")
	}
	if ok, err := p.RecordExists(t.Context(), server.URL+"/tasks/o/r/issues/9"); err != nil || ok {
		t.Fatalf("absent=%v %v", ok, err)
	}
	if ok, err := p.RecordExists(t.Context(), server.URL+"/tasks/o/r/issues/10"); err == nil || ok {
		t.Fatal("permission failure hidden")
	}
	before := calls.Load()
	for _, ref := range []string{"7", "#7", "o/r#7", server.URL + "/wrong/o/r/issues/7", "https://foreign.invalid/o/r/issues/7"} {
		if _, err := p.RecordExists(t.Context(), ref); err == nil {
			t.Errorf("unscoped/unbound reference %s accepted", ref)
		}
	}
	if _, err := p.AncestorOfMainline(t.Context(), "7"); err == nil {
		t.Fatal("unscoped proposal accepted")
	}
	if calls.Load() != before {
		t.Fatal("binding mismatch transmitted credentials")
	}
	p.Run = func(context.Context, string, ...string) ([]byte, error) { return nil, errors.New("fetch failed") }
	if _, err := p.AncestorOfMainline(t.Context(), server.URL+"/code/o/r/pulls/7"); err == nil {
		t.Fatal("fetch failure hidden")
	}
	if err := os.WriteFile(filepath.Join(root, ".pyxis/config.yaml"), []byte("integrations:\n  unexpected: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := p.resolveBinding("7", true); err == nil {
		t.Fatal("broken config silently fell back")
	}
}
