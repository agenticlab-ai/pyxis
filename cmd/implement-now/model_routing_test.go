package main

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/adapters/harness/claude"
	"github.com/agenticlab-ai/pyxis/internal/adapters/harness/codex"
	"github.com/agenticlab-ai/pyxis/internal/entity"
	"github.com/agenticlab-ai/pyxis/internal/executiondemand"
	"github.com/agenticlab-ai/pyxis/internal/executor/config"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/harnessconfig"
)

func modelRoutingTask() port.Task {
	return port.Task{
		ID:            "T-routing",
		DemandID:      executiondemand.DemandID(strings.Repeat("a", 64)),
		DemandProfile: executiondemand.ProfileMinimum{Capability: executiondemand.CapabilityEconomy, Deliberation: executiondemand.DeliberationLight, Context: executiondemand.ContextStandard, Assurance: executiondemand.AssuranceOrdinary},
	}
}

func TestImplementationLocalModelsBindAcrossProfiles(t *testing.T) {
	hc, err := harnessconfig.Parse([]byte(opencodeCatalogYAML("opencode-go/gpt-5.6-luna", "opencode-go/kimi-k3", "opencode-go/kimi-k3")))
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Parse([]byte("executor:\n  routing:\n    default: opencode\n  review:\n    backend: cursor\n"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name    string
		profile executiondemand.ProfileMinimum
		model   string
	}{
		{"economy", executiondemand.ProfileMinimum{Capability: executiondemand.CapabilityEconomy, Deliberation: executiondemand.DeliberationLight, Context: executiondemand.ContextStandard, Assurance: executiondemand.AssuranceOrdinary}, "opencode-go/gpt-5.6-luna"},
		{"strong", executiondemand.ProfileMinimum{Capability: executiondemand.CapabilityStandard, Deliberation: executiondemand.DeliberationStandard, Context: executiondemand.ContextLarge, Assurance: executiondemand.AssuranceOrdinary}, "opencode-go/kimi-k3"},
		{"frontier", executiondemand.ProfileMinimum{Capability: executiondemand.CapabilityFrontier, Deliberation: executiondemand.DeliberationMaximum, Context: executiondemand.ContextExtended, Assurance: executiondemand.AssuranceCritical}, "opencode-go/kimi-k3"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			task := modelRoutingTask()
			task.DemandProfile = tc.profile
			backend, rt, err := newRuntimeResolver(t.Context(), cfg, "codex", hc, nil)(task)
			if err != nil {
				t.Fatal(err)
			}
			if backend != "opencode" {
				t.Fatalf("local backend = %q", backend)
			}
			if route := rt.(port.ModelRouteProvider).ModelRoute(); route.Model != tc.model || route.Effort != "none" {
				t.Fatalf("persisted model route = %+v", route)
			}
		})
	}
	if _, err := buildReviewHarness("cursor", cfg, hc); err != nil {
		t.Fatal(err)
	}
}

func TestImplementationCatalogRefusesDemandTheConfiguredProfilesCannotMeet(t *testing.T) {
	hc, err := harnessconfig.Parse([]byte(opencodeCatalogYAML("opencode-go/gpt-5.6-luna", "", "")))
	if err != nil {
		t.Fatal(err)
	}
	task := modelRoutingTask()
	task.DemandProfile = executiondemand.ProfileMinimum{Capability: executiondemand.CapabilityFrontier, Deliberation: executiondemand.DeliberationMaximum, Context: executiondemand.ContextExtended, Assurance: executiondemand.AssuranceCritical}
	if _, _, err := newRuntimeResolver(t.Context(), config.Defaults(), "opencode", hc, nil)(task); err == nil || !strings.Contains(err.Error(), "semantic route did not resolve") {
		t.Fatalf("unsatisfied demand routing = %v", err)
	}
	cfg, err := config.Parse(coverageActivationConfigWire(strings.Repeat("a", 64)))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := buildBackend("opencode", cfg, hc); err != nil {
		t.Fatalf("configured catalog should build under evidence activation: %v", err)
	}
	if _, err := buildReviewHarness("opencode", cfg, hc); err != nil {
		t.Fatalf("review adapter should use the same configured catalog: %v", err)
	}
}

func opencodeCatalogYAML(economy, strong, frontier string) string {
	body := "harness-config:\n  opencode:\n    implementation:\n      implementation-economy:\n        model: " + economy + "\n        effort: none\n        capability: economy\n        deliberation: light\n        context: standard\n        assurance: ordinary\n        cost_rank: 1\n"
	if strong == "" {
		return body + "        strongest: true\n    review:\n      review-standard:\n        model: anthropic/claude-sonnet-5\n        effort: none\n        capability: standard\n        deliberation: standard\n        context: large\n        assurance: elevated\n        cost_rank: 1\n        strongest: true\n"
	}
	return body + "      implementation-strong:\n        model: " + strong + "\n        effort: none\n        capability: strong\n        deliberation: deep\n        context: extended\n        assurance: elevated\n        cost_rank: 2\n      implementation-frontier:\n        model: " + frontier + "\n        effort: none\n        capability: frontier\n        deliberation: maximum\n        context: extended\n        assurance: critical\n        cost_rank: 3\n        strongest: true\n    review:\n      review-standard:\n        model: anthropic/claude-sonnet-5\n        effort: none\n        capability: standard\n        deliberation: standard\n        context: large\n        assurance: elevated\n        cost_rank: 1\n        strongest: true\n"
}

func TestImplementationRoutingRejectsInvalidDemandBeforeLaunch(t *testing.T) {
	task := modelRoutingTask()
	task.DemandProfile.Capability = executiondemand.Capability("beyond-frontier")
	_, _, err := newRuntimeResolver(context.Background(), config.Defaults(), "codex", fixtureHarnessConfig(t), nil)(task)
	if err == nil || !strings.Contains(err.Error(), "semantic route did not resolve") {
		t.Fatalf("invalid demand error = %v, want a routing refusal", err)
	}
}

type discoveryRuntime struct {
	name        string
	authErr     error
	authChecks  *[]string
	launchCalls *int
}

func (r *discoveryRuntime) Launch(context.Context, port.LaunchSpec) (port.Handle, error) {
	(*r.launchCalls)++
	return nil, nil
}

func (*discoveryRuntime) Await(context.Context, port.Handle) (port.RunResult, error) {
	return port.RunResult{}, nil
}

func (*discoveryRuntime) Kill(context.Context, port.Handle) error { return nil }

func (r *discoveryRuntime) AuthCheck(context.Context) error {
	*r.authChecks = append(*r.authChecks, r.name)
	return r.authErr
}

type authenticatedCodex struct {
	*codex.Adapter
	authChecks *int
	authErr    error
}

func (r *authenticatedCodex) AuthCheck(context.Context) error {
	(*r.authChecks)++
	return r.authErr
}

type authenticatedClaude struct {
	*claude.Adapter
	authChecks *int
	authErr    error
}

func (r *authenticatedClaude) AuthCheck(context.Context) error {
	(*r.authChecks)++
	return r.authErr
}

func TestDiscoverBackendUsesStablePreferenceWithoutLaunching(t *testing.T) {
	buildFailure := errors.New("not installed")
	authFailure := errors.New("not authenticated")
	var builds, authChecks []string
	launchCalls := 0
	source := backendSource{
		backends: func() []entity.Harness {
			return []entity.Harness{"zeta", "cursor", "opencode", "claude", "codex", "cursor"}
		},
		build: func(backend entity.Harness) (port.AgentRuntime, error) {
			builds = append(builds, backend.String())
			switch backend {
			case "codex":
				return nil, buildFailure
			case "claude":
				return &discoveryRuntime{name: backend.String(), authErr: authFailure, authChecks: &authChecks, launchCalls: &launchCalls}, nil
			default:
				return &discoveryRuntime{name: backend.String(), authChecks: &authChecks, launchCalls: &launchCalls}, nil
			}
		},
	}

	backend, err := discoverBackend(context.Background(), source, nil)
	if err != nil {
		t.Fatalf("discover backend: %v", err)
	}
	if backend != "cursor" {
		t.Fatalf("backend = %q, want cursor", backend)
	}
	if want := []string{"codex", "claude", "cursor"}; !reflect.DeepEqual(builds, want) {
		t.Fatalf("build order = %v, want %v", builds, want)
	}
	if want := []string{"claude", "cursor"}; !reflect.DeepEqual(authChecks, want) {
		t.Fatalf("auth-check order = %v, want %v", authChecks, want)
	}
	if launchCalls != 0 {
		t.Fatalf("discovery invoked Launch %d times, want zero", launchCalls)
	}
}

func TestExplicitBackendFailureDoesNotFallBackToDiscovery(t *testing.T) {
	explicitFailure := errors.New("explicit backend build failed")
	registryCalls := 0
	source := backendSource{
		backends: func() []entity.Harness {
			registryCalls++
			return []entity.Harness{"codex"}
		},
		build: func(backend entity.Harness) (port.AgentRuntime, error) {
			if backend != "claude" {
				t.Fatalf("build backend = %q, want explicit claude", backend)
			}
			return nil, explicitFailure
		},
	}

	_, _, err := newRuntimeResolverFrom(context.Background(), config.Defaults(), "claude", source, nil)(modelRoutingTask())
	if !errors.Is(err, explicitFailure) {
		t.Fatalf("resolve error = %v, want explicit build failure", err)
	}
	if registryCalls != 0 {
		t.Fatalf("explicit failure consulted automatic registry %d times, want zero", registryCalls)
	}
}

func TestAutomaticDiscoveryFailsClosedAndPropagatesCancellation(t *testing.T) {
	t.Run("no usable backend", func(t *testing.T) {
		source := backendSource{
			backends: func() []entity.Harness { return []entity.Harness{"claude", "codex"} },
			build: func(backend entity.Harness) (port.AgentRuntime, error) {
				return nil, errors.New(backend.String() + " unavailable")
			},
		}
		_, _, err := newRuntimeResolverFrom(context.Background(), config.Defaults(), "", source, nil)(modelRoutingTask())
		if !errors.Is(err, ErrNoUsableBackend) {
			t.Fatalf("resolve error = %v, want ErrNoUsableBackend", err)
		}
	})

	t.Run("nil runtime", func(t *testing.T) {
		source := backendSource{
			backends: func() []entity.Harness { return []entity.Harness{"codex"} },
			build:    func(entity.Harness) (port.AgentRuntime, error) { return nil, nil },
		}
		_, _, err := newRuntimeResolverFrom(context.Background(), config.Defaults(), "", source, nil)(modelRoutingTask())
		if !errors.Is(err, ErrNoUsableBackend) || !strings.Contains(err.Error(), "runtime builder returned nil") {
			t.Fatalf("resolve error = %v, want nil-runtime discovery failure", err)
		}
	})

	t.Run("cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		buildCalls := 0
		source := backendSource{
			backends: func() []entity.Harness { return []entity.Harness{"codex"} },
			build: func(entity.Harness) (port.AgentRuntime, error) {
				buildCalls++
				return nil, errors.New("must not build")
			},
		}
		_, _, err := newRuntimeResolverFrom(ctx, config.Defaults(), "", source, nil)(modelRoutingTask())
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("resolve error = %v, want context.Canceled", err)
		}
		if buildCalls != 0 {
			t.Fatalf("cancelled discovery built %d backends, want zero", buildCalls)
		}
	})
}

func TestAutomaticBackendSelectionIsReusedForRepeatedTaskResolution(t *testing.T) {
	authChecks, builds := 0, 0
	source := backendSource{
		backends: func() []entity.Harness { return []entity.Harness{"codex"} },
		build: func(entity.Harness) (port.AgentRuntime, error) {
			builds++
			adapter, err := codex.New(codex.WithProfiles(fixtureHarnessConfig(t).Profiles("codex")))
			if err != nil {
				return nil, err
			}
			return &authenticatedCodex{Adapter: adapter, authChecks: &authChecks}, nil
		},
	}
	resolve := newRuntimeResolverFrom(context.Background(), config.Defaults(), "", source, nil)
	for i := range 2 {
		if backend, _, err := resolve(modelRoutingTask()); err != nil || backend != "codex" {
			t.Fatalf("resolve task %d = backend %q, error %v", i+1, backend, err)
		}
	}
	if authChecks != 1 {
		t.Fatalf("automatic backend auth checks = %d, want one", authChecks)
	}
	if builds != 3 {
		t.Fatalf("runtime builds = %d, want discovery plus one task-scoped runtime per task", builds)
	}
}

func TestAutomaticDiscoverySkipsAuthenticatedBackendThatCannotRouteTask(t *testing.T) {
	var builds, authChecks []string
	launchCalls := 0
	source := backendSource{
		backends: func() []entity.Harness { return []entity.Harness{"codex", "claude"} },
		build: func(backend entity.Harness) (port.AgentRuntime, error) {
			builds = append(builds, backend.String())
			if backend == "codex" {
				return &discoveryRuntime{name: backend.String(), authChecks: &authChecks, launchCalls: &launchCalls}, nil
			}
			adapter, err := claude.New(claude.WithProfiles(fixtureHarnessConfig(t).Profiles("claude")))
			if err != nil {
				return nil, err
			}
			checks := 0
			return &authenticatedClaude{Adapter: adapter, authChecks: &checks}, nil
		},
	}
	backend, _, err := newRuntimeResolverFrom(context.Background(), config.Defaults(), "", source, nil)(modelRoutingTask())
	if err != nil || backend != "claude" {
		t.Fatalf("automatic route = %q, %v; want usable claude fallback", backend, err)
	}
	if want := []string{"codex", "claude", "claude"}; !reflect.DeepEqual(builds, want) {
		t.Fatalf("build order = %v, want %v", builds, want)
	}
	if launchCalls != 0 {
		t.Fatalf("routing probe invoked Launch %d times, want zero", launchCalls)
	}
}

func TestResumableBackendWinsAutomaticDiscoveryAcrossInvocations(t *testing.T) {
	authChecks, discoveryCalls := 0, 0
	source := backendSource{
		backends: func() []entity.Harness {
			discoveryCalls++
			return []entity.Harness{"codex", "claude"}
		},
		build: func(backend entity.Harness) (port.AgentRuntime, error) {
			switch backend {
			case "claude":
				adapter, err := claude.New(claude.WithProfiles(fixtureHarnessConfig(t).Profiles("claude")))
				if err != nil {
					return nil, err
				}
				return &authenticatedClaude{Adapter: adapter, authChecks: &authChecks}, nil
			case "codex":
				adapter, err := codex.New(codex.WithProfiles(fixtureHarnessConfig(t).Profiles("codex")))
				if err != nil {
					return nil, err
				}
				return &authenticatedCodex{Adapter: adapter, authChecks: &authChecks}, nil
			default:
				return nil, errors.New("unknown backend " + backend.String())
			}
		},
	}
	hint := func(context.Context, port.Task) (entity.Harness, bool, error) { return "claude", true, nil }
	for invocation := 1; invocation <= 2; invocation++ {
		resolve := newRuntimeResolverFrom(context.Background(), config.Defaults(), "", source, hint)
		backend, _, err := resolve(modelRoutingTask())
		if err != nil || backend != "claude" {
			t.Fatalf("invocation %d route = %q, %v; want resumable claude", invocation, backend, err)
		}
	}
	if discoveryCalls != 0 {
		t.Fatalf("resumable route invoked automatic discovery %d times", discoveryCalls)
	}
	if authChecks != 2 {
		t.Fatalf("resumable backend auth checks = %d, want one per invocation", authChecks)
	}
}

func TestExplicitBackendOverridesResumeHint(t *testing.T) {
	hintCalls, authChecks := 0, 0
	source := backendSource{
		backends: func() []entity.Harness { t.Fatal("explicit route invoked discovery"); return nil },
		build: func(backend entity.Harness) (port.AgentRuntime, error) {
			if backend != "codex" {
				t.Fatalf("built backend %q, want codex", backend)
			}
			adapter, err := codex.New(codex.WithProfiles(fixtureHarnessConfig(t).Profiles("codex")))
			if err != nil {
				return nil, err
			}
			return &authenticatedCodex{Adapter: adapter, authChecks: &authChecks}, nil
		},
	}
	hint := func(context.Context, port.Task) (entity.Harness, bool, error) {
		hintCalls++
		return "claude", true, nil
	}
	backend, _, err := newRuntimeResolverFrom(context.Background(), config.Defaults(), "codex", source, hint)(modelRoutingTask())
	if err != nil || backend != "codex" {
		t.Fatalf("explicit route = %q, %v; want codex", backend, err)
	}
	if hintCalls != 0 {
		t.Fatalf("explicit route consulted resume hint %d times", hintCalls)
	}
}

func TestResumableBackendsRemainTaskScopedAndFailClosed(t *testing.T) {
	buildFailure := errors.New("recorded backend unavailable")
	hints := map[string]string{"T-claude": "claude", "T-codex": "codex", "T-broken": "broken"}
	hintCalls := map[string]int{}
	source := backendSource{
		backends: func() []entity.Harness { t.Fatal("resumable route invoked discovery"); return nil },
		build: func(backend entity.Harness) (port.AgentRuntime, error) {
			if backend == "broken" {
				return nil, buildFailure
			}
			if backend == "claude" {
				adapter, err := claude.New(claude.WithProfiles(fixtureHarnessConfig(t).Profiles("claude")))
				if err != nil {
					return nil, err
				}
				checks := 0
				return &authenticatedClaude{Adapter: adapter, authChecks: &checks}, nil
			}
			adapter, err := codex.New(codex.WithProfiles(fixtureHarnessConfig(t).Profiles("codex")))
			if err != nil {
				return nil, err
			}
			checks := 0
			return &authenticatedCodex{Adapter: adapter, authChecks: &checks}, nil
		},
	}
	hint := func(_ context.Context, task port.Task) (entity.Harness, bool, error) {
		hintCalls[task.ID]++
		return entity.Harness(hints[task.ID]), true, nil
	}
	resolve := newRuntimeResolverFrom(context.Background(), config.Defaults(), "", source, hint)
	for taskID, want := range map[string]string{"T-claude": "claude", "T-codex": "codex"} {
		task := modelRoutingTask()
		task.ID = taskID
		backend, _, err := resolve(task)
		if err != nil || backend != entity.Harness(want) {
			t.Fatalf("task %s route = %q, %v; want %s", taskID, backend, err, want)
		}
	}
	hints["T-claude"] = "codex"
	task := modelRoutingTask()
	task.ID = "T-claude"
	if backend, _, err := resolve(task); err != nil || backend != "claude" {
		t.Fatalf("repeated task route = %q, %v; want pinned claude", backend, err)
	}
	if hintCalls["T-claude"] != 1 {
		t.Fatalf("same task consulted changing resume hint %d times, want once", hintCalls["T-claude"])
	}
	task = modelRoutingTask()
	task.ID = "T-broken"
	if _, _, err := resolve(task); !errors.Is(err, buildFailure) || !strings.Contains(err.Error(), "recorded backend") {
		t.Fatalf("broken resumable backend error = %v", err)
	}

	badHint := func(context.Context, port.Task) (entity.Harness, bool, error) { return "", true, nil }
	if _, _, err := newRuntimeResolverFrom(context.Background(), config.Defaults(), "", source, badHint)(modelRoutingTask()); err == nil || !strings.Contains(err.Error(), "empty backend") {
		t.Fatalf("empty resumable backend error = %v", err)
	}
}
