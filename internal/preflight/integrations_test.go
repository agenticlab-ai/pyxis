package preflight_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/adapters/common/forgejo"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/integrations"
	"github.com/agenticlab-ai/pyxis/internal/preflight"
	"github.com/agenticlab-ai/pyxis/internal/wire"
)

type nativeRoundTrip func(*http.Request) (*http.Response, error)

func (f nativeRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestIntegrationPreflightRequiresOnlySelectedHosts(t *testing.T) {
	profile := integrations.Config{SchemaVersion: integrations.SchemaVersion, Connections: map[string]integrations.Connection{"local": {Provider: "forgejo", BaseURL: "http://127.0.0.1:3000", TokenEnv: "TEST_TOKEN"}}}
	profile.Bindings.Tasks = integrations.Binding{Provider: "carina", Root: "work-items"}
	profile.Bindings.CodeHost = integrations.Binding{Connection: "local", Repository: "o/code"}
	runner := newHappyRunner(t)
	runner.failOnCall = true
	requests := 0
	factory := func(r integrations.ResolvedRepository) (*forgejo.Client, error) {
		return forgejo.NewWithHTTP(r.BaseURL, "test", &http.Client{Transport: nativeRoundTrip(func(req *http.Request) (*http.Response, error) {
			requests++
			if req.Method != "GET" {
				t.Fatal("preflight performed mutation")
			}
			body := `{"id":7}`
			if req.URL.Path == "/api/v1/user" {
				body = `{"id":1,"login":"agent"}`
			}
			return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
		})})
	}
	in := preflight.Input{Knobs: config.Knobs{Tracker: "carina", CIMode: "both", Wiring: config.Wiring{Repo: "use", Protection: "required-checks"}}, Integrations: profile, WireRunner: runner, ForgejoClientFactory: factory}
	rows, err := preflight.IntegrationChecks(context.Background(), in)
	if err != nil || len(rows) != 1 || rows[0].Status != preflight.StatusPass || requests != 2 {
		t.Fatalf("%#v %v requests %d", rows, err, requests)
	}
	tools, err := preflight.PrerequisitesForIntegrations(context.Background(), in.Knobs, runner, profile)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range tools {
		if tool.Name == "gh" {
			t.Fatal("Forgejo required gh")
		}
	}
}

func TestIntegrationPreflightGitHubFailuresUseDeclaredConnection(t *testing.T) {
	profile := integrations.Config{SchemaVersion: integrations.SchemaVersion, Connections: map[string]integrations.Connection{"host": {Provider: integrations.GitHub}}}
	profile.Bindings.Tasks = integrations.Binding{Connection: "host", Repository: "o/tasks"}
	profile.Bindings.CodeHost = profile.Bindings.Tasks
	for _, failure := range []string{"", "version", "auth", "repository", "default-runner"} {
		t.Run(failure, func(t *testing.T) {
			runner := newHappyRunner(t)
			switch failure {
			case "version":
				runner.version.exit = 1
			case "auth":
				runner.auth.exit = 1
			case "repository":
				runner.repoView = []scriptedResp{{exit: 1, stderr: "HTTP 404"}}
			}
			in := preflight.Input{Knobs: config.Knobs{Tracker: "github"}, Integrations: profile, WireRunner: runner}
			if failure == "default-runner" {
				t.Setenv("PATH", t.TempDir())
				in.WireRunner = wire.GHRunner{}
			}
			rows, err := preflight.IntegrationChecks(t.Context(), in)
			if err != nil || len(rows) != 1 || (rows[0].Status == preflight.StatusPass) != (failure == "") {
				t.Fatalf("declared Github preflight rows=%#v err=%v", rows, err)
			}
		})
	}
	rows, err := preflight.PrerequisitesForIntegrations(t.Context(), config.Knobs{Tracker: "github"}, newHappyRunner(t), profile)
	if err != nil || rows[len(rows)-1].Name != "gh" {
		t.Fatalf("selected GitHub requires gh: %#v %v", rows, err)
	}
	profile.Bindings.Tasks.Connection = "absent"
	if _, err := preflight.PrerequisitesForIntegrations(t.Context(), config.Knobs{}, nil, profile); err == nil {
		t.Fatal("invalid prerequisite connection ignored")
	}
}

func TestIntegrationPreflightNativeReadFailuresAndRegistryRefusal(t *testing.T) {
	profile := integrations.Config{SchemaVersion: integrations.SchemaVersion, Connections: map[string]integrations.Connection{"host": {Provider: integrations.Forgejo, BaseURL: "https://forge.example", TokenEnv: "TEST_NATIVE_PREFLIGHT"}}}
	profile.Bindings.Tasks = integrations.Binding{Connection: "host", Repository: "o/tasks"}
	profile.Bindings.CodeHost = profile.Bindings.Tasks
	t.Setenv("TEST_NATIVE_PREFLIGHT", "selected")
	prior := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = prior })
	http.DefaultTransport = nativeRoundTrip(func(req *http.Request) (*http.Response, error) {
		body := `{}`
		if req.URL.Path == "/api/v1/user" {
			body = `{"id":1,"login":"assistant"}`
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	in := preflight.Input{Target: t.TempDir(), Knobs: config.Knobs{Tracker: "forgejo", CIMode: "local", Wiring: config.Wiring{Protection: "none", Repo: "use"}}, Integrations: profile}
	rows, err := preflight.IntegrationChecks(t.Context(), in)
	if err != nil || len(rows) != 1 || rows[0].Status != preflight.StatusRefuse || !strings.Contains(rows[0].Reason, "incomplete") {
		t.Fatalf("incomplete repository admitted: %#v %v", rows, err)
	}
	rows, err = preflight.Run(t.Context(), in)
	if err == nil || len(rows) == 0 || rows[len(rows)-1].Status != preflight.StatusRefuse {
		t.Fatalf("registry lost native refusal: %#v %v", rows, err)
	}
	in.Integrations.Bindings.Tasks.Connection = "absent"
	if _, err := preflight.Run(t.Context(), in); err == nil || !strings.Contains(err.Error(), "integrations") {
		t.Fatalf("registry lost malformed binding: %v", err)
	}
	in.Integrations = profile
	in.Integrations.Bindings.Tasks.Connection = "host"
	in.ForgejoClientFactory = func(integrations.ResolvedRepository) (*forgejo.Client, error) {
		return nil, errors.New("factory unavailable")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := preflight.IntegrationChecks(ctx, in); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled native preflight %v", err)
	}
}

func TestIntegrationPreflightMissingTokenIsNamed(t *testing.T) {
	profile := integrations.Config{SchemaVersion: integrations.SchemaVersion, Connections: map[string]integrations.Connection{"local": {Provider: "forgejo", BaseURL: "http://127.0.0.1:3000", TokenEnv: "TEST_MISSING_FORGE_TOKEN"}}}
	profile.Bindings.Tasks = integrations.Binding{Connection: "local", Repository: "o/tasks"}
	profile.Bindings.CodeHost = profile.Bindings.Tasks
	t.Setenv("TEST_MISSING_FORGE_TOKEN", "")
	rows, err := preflight.IntegrationChecks(context.Background(), preflight.Input{Knobs: config.Knobs{Tracker: "forgejo"}, Integrations: profile})
	if err != nil || len(rows) != 1 || rows[0].Status != preflight.StatusRefuse || !strings.Contains(rows[0].Reason, "TEST_MISSING_FORGE_TOKEN") {
		t.Fatalf("%#v %v", rows, err)
	}
}
