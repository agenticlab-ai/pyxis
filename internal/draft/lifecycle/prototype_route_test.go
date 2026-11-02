package lifecycle

import (
	"context"
	"errors"
	"strings"
	"testing"

	draftshell "github.com/agenticlab-ai/pyxis/internal/adapters/draft/shell"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/entity"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/harnessconfig"
	"github.com/agenticlab-ai/pyxis/internal/harnessconfig/harnessconfigtest"
	"github.com/agenticlab-ai/pyxis/internal/nativerouting"
)

type prototypeRouteRuntime struct {
	port.AgentRuntime // Any process operation during resolution is a test failure.
	auth              func(context.Context) error
}

func (r *prototypeRouteRuntime) AuthCheck(ctx context.Context) error { return r.auth(ctx) }

func TestResolvePrototypeRoute(t *testing.T) {
	// Put the cheaper shared-model profile later in catalog order so selecting
	// the first match cannot accidentally satisfy the lowest-cost case.
	source := strings.NewReplacer("cost_rank: 2\n", "cost_rank: 3\n", "cost_rank: 3\n", "cost_rank: 2\n").Replace(harnessconfigtest.YAML)
	catalog, err := harnessconfig.Parse([]byte(source))
	if err != nil {
		t.Fatal(err)
	}
	configured := &config.PrototypeSettings{Harness: entity.Codex, Profile: "codex-implementation-economy"}
	for _, tc := range []struct {
		name              string
		configured        *config.PrototypeSettings
		backend, model    string
		buildErr, authErr error
		nilRuntime        bool
		emptyCatalog      bool
		want, modelWant   string
		effortWant        string
		failure           string
	}{
		{name: "configured ignores ambient", configured: configured, backend: "invalid", model: "unknown", want: "configured codex codex-implementation-economy", modelWant: "gpt-5.6-luna", effortWant: "low"},
		{name: "ambient", backend: "claude", model: "claude-haiku-4-5", want: "ambient claude haiku-low", modelWant: "claude-haiku-4-5", effortWant: "low"},
		{name: "lowest cost", backend: "claude", model: "claude-sonnet-5", want: "ambient claude sonnet-high", modelWant: "claude-sonnet-5", effortWant: "high"},
		{name: "both flags absent", failure: "prototype config block"},
		{name: "model absent", backend: "codex", failure: "--ambient-model"},
		{name: "backend absent", model: "gpt-5.6-luna", failure: "--ambient-backend"},
		{name: "unknown backend", backend: "future", model: "gpt-5.6-luna", failure: "ambient backend"},
		{name: "model absent from backend", backend: "claude", model: "gpt-5.6-luna", failure: "prototype config block"},
		{name: "empty catalog", backend: "codex", model: "gpt-5.6-luna", emptyCatalog: true, failure: "no entry for model"},
		{name: "unknown configured profile", configured: &config.PrototypeSettings{Harness: entity.Codex, Profile: "missing"}, backend: "claude", model: "claude-haiku-4-5", failure: `does not hold profile "missing"`},
		{name: "review profile is not implementation", configured: &config.PrototypeSettings{Harness: entity.Claude, Profile: "haiku-medium-review"}, failure: "implementation catalog"},
		{name: "configured constructor failure never falls back", configured: configured, backend: "claude", model: "claude-haiku-4-5", buildErr: errors.New("runtime unavailable"), failure: "runtime unavailable"},
		{name: "nil runtime", configured: configured, nilRuntime: true, failure: "constructor returned no runtime"},
		{name: "configured auth failure never falls back", configured: configured, backend: "claude", model: "claude-haiku-4-5", authErr: errors.New("login missing"), failure: "login missing"},
		{name: "ambient auth failure", backend: "claude", model: "claude-haiku-4-5", authErr: errors.New("login missing"), failure: "login missing"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			builds, auths := 0, 0
			runtime := &prototypeRouteRuntime{auth: func(ctx context.Context) error {
				auths++
				if ctx != t.Context() {
					t.Fatal("authentication lost caller's context")
				}
				return tc.authErr
			}}
			build := func(backend entity.Harness) (port.AgentRuntime, error) {
				builds++
				if builds != 1 || (tc.configured != nil && backend != tc.configured.Harness) {
					t.Fatalf("unexpected fallback or second construction: %s", backend)
				}
				if tc.nilRuntime {
					return nil, nil
				}
				return runtime, tc.buildErr
			}
			hc := catalog
			if tc.emptyCatalog {
				hc = harnessconfig.Config{}
			}
			got, err := ResolvePrototypeRoute(t.Context(), tc.configured, hc, tc.backend, tc.model, build)
			if tc.failure != "" {
				if err == nil || !strings.Contains(err.Error(), tc.failure) || got.Runtime != nil {
					t.Fatalf("route=%+v err=%v, want refusal %q", got, err, tc.failure)
				}
				return
			}
			if err != nil || got.String() != tc.want || got.Profile.Model != tc.modelWant || got.Profile.Effort != tc.effortWant || got.Runtime != runtime || builds != 1 || auths != 1 {
				t.Fatalf("route=%+v err=%v builds=%d auths=%d", got, err, builds, auths)
			}
		})
	}
}

func TestPrototypeRouteIsCheckedAfterDraftAndBeforeFetch(t *testing.T) {
	for _, draftExists := range []bool{false, true} {
		t.Run(map[bool]string{false: "missing draft", true: "route refusal"}[draftExists], func(t *testing.T) {
			root, idea, cfg := prototypeFixture(t)
			if !draftExists {
				prototypeGit(t, root, "branch", "-D", "contracts/widget")
			}
			calls := 0
			cause := errors.New("route cannot authenticate")
			resolve := func(context.Context) (PrototypeRoute, error) {
				calls++
				return PrototypeRoute{}, cause
			}
			run := func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
				if args[0] == "fetch" || args[0] == "worktree" && args[1] == "add" {
					t.Fatalf("mutation before route resolved: %v", args)
				}
				return draftshell.Run(ctx, dir, env, name, args...)
			}
			_, err := CreatePrototype(t.Context(), root, cfg, "widget", idea, resolve, run)
			if draftExists {
				if calls != 1 || !errors.Is(err, cause) {
					t.Fatalf("route refusal lost: calls=%d err=%v", calls, err)
				}
			} else if calls != 0 || err == nil || !strings.Contains(err.Error(), "open the draft first") {
				t.Fatalf("route preceded draft check: calls=%d err=%v", calls, err)
			}
			assertNoPrototype(t, root)
		})
	}
}

func TestPrototypeResultRetainsRouteForBuild(t *testing.T) {
	root, idea, cfg := prototypeFixture(t)
	runtime := &prototypeRouteRuntime{auth: func(context.Context) error { return nil }}
	route := PrototypeRoute{
		Source: "configured", Harness: entity.Codex,
		Profile: nativerouting.ProfileSpec{ID: "selected", Model: "native-model", Effort: "high"},
		Runtime: runtime,
	}
	calls := 0
	resolve := func(context.Context) (PrototypeRoute, error) {
		calls++
		return route, nil
	}
	result, err := CreatePrototype(t.Context(), root, cfg, "widget", idea, resolve, draftshell.Run)
	if err != nil || calls != 1 || result.Route != route {
		t.Fatalf("result lost resolved runtime or profile: %+v, calls=%d, err=%v", result, calls, err)
	}
}
