package main

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/executiondemand"
	"github.com/agenticlab-ai/pyxis/internal/executor/config"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/reviewinvoke"
	"github.com/agenticlab-ai/pyxis/internal/executor/runtime"
	"github.com/agenticlab-ai/pyxis/internal/harnessconfig"
)

// This opt-in smoke test uses installed, authenticated CLIs and makes model
// calls. It exercises local configuration, production route resolution and
// adapter launches without publishing anything or touching a real task. It is
// distinct from the hermetic command E2E, whose external CLIs are Go helpers.
func TestLive_MachineLocalHarnesses(t *testing.T) {
	repo := os.Getenv("PYXIS_LIVE_HARNESS_REPO")
	if repo == "" {
		t.Skip("set PYXIS_LIVE_HARNESS_REPO to a repository with machine-local routing; this test makes model calls")
	}
	cfg, err := config.Load(repo)
	if err != nil {
		t.Fatal(err)
	}
	hc, err := harnessconfig.Load(repo)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DefaultHarness() != "opencode" || cfg.ReviewHarness() != "cursor" {
		t.Fatalf("machine routes = implementation %q, review %q; want opencode and cursor", cfg.DefaultHarness(), cfg.ReviewHarness())
	}
	task := routedTask("local-harness-smoke", executiondemand.ProfileMinimum{Capability: executiondemand.CapabilityEconomy, Deliberation: executiondemand.DeliberationLight, Context: executiondemand.ContextStandard, Assurance: executiondemand.AssuranceOrdinary})

	t.Run("implementation", func(t *testing.T) {
		for _, profile := range []struct {
			name   string
			demand executiondemand.ProfileMinimum
		}{
			{"economy", executiondemand.ProfileMinimum{Capability: executiondemand.CapabilityEconomy, Deliberation: executiondemand.DeliberationLight, Context: executiondemand.ContextStandard, Assurance: executiondemand.AssuranceOrdinary}},
			{"frontier", executiondemand.ProfileMinimum{Capability: executiondemand.CapabilityFrontier, Deliberation: executiondemand.DeliberationMaximum, Context: executiondemand.ContextExtended, Assurance: executiondemand.AssuranceCritical}},
		} {
			t.Run(profile.name, func(t *testing.T) {
				task := task
				task.DemandProfile = profile.demand
				ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
				defer cancel()
				backend, rt, err := newRuntimeResolver(ctx, cfg, "codex", hc, nil)(task)
				if err != nil {
					t.Fatal(err)
				}
				if backend != "opencode" {
					t.Fatalf("local implementation override lost to ambient Codex: %q", backend)
				}
				if err := rt.AuthCheck(ctx); err != nil {
					t.Fatal(err)
				}
				workdir := liveHarnessWorkdir(t)
				before := liveHarnessGit(t, workdir, "rev-parse", "HEAD")
				stateDir, _, err := runtime.PrepareTaskState(ctx, workdir, backend)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := os.RemoveAll(runtime.TaskStateRoot(workdir)); err != nil {
						t.Error(err)
					}
				})
				route, err := port.RouteOf(rt)
				if err != nil {
					t.Fatal(err)
				}
				model, effort := route.Model, route.Effort
				t.Logf("implementation: backend=%s model=%s effort=%s", backend, model, effort)
				handle, err := rt.Launch(ctx, port.LaunchSpec{
					Purpose: port.LaunchPurposeImplementation, Workdir: workdir, HeadBranch: "main", RuntimeStateDir: stateDir,
					Model: model, Effort: effort,
					Brief: "Task T1 of plan local-harness-smoke: create smoke.txt containing exactly PYXIS_IMPLEMENTATION_OK followed by a newline. The supervisor will commit your edits after you finish. No other changes are needed.",
				})
				if err != nil {
					t.Fatal(err)
				}
				result, err := rt.Await(ctx, handle)
				if err != nil {
					t.Fatal(err)
				}
				if result.Incomplete {
					t.Fatalf("implementation incomplete: %s", result.Reason)
				}
				if after := liveHarnessGit(t, workdir, "rev-parse", "HEAD"); after != before {
					t.Fatal("implementation changed Git history before the supervisor checkpoint")
				}
				artifact, err := os.ReadFile(filepath.Join(workdir, "smoke.txt"))
				if err != nil || string(artifact) != "PYXIS_IMPLEMENTATION_OK\n" {
					t.Fatalf("implementation artifact = %q, error = %v", artifact, err)
				}
				liveHarnessGit(t, workdir, "add", "smoke.txt")
				liveHarnessGit(t, workdir, "commit", "-m", "chore: checkpoint local harness smoke")
				if got := liveHarnessGit(t, workdir, "show", "HEAD:smoke.txt"); got != "PYXIS_IMPLEMENTATION_OK" {
					t.Fatalf("committed implementation artifact = %q", got)
				}
			})
		}
	})

	t.Run("review", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
		defer cancel()
		factory := newReviewFactory(cfg, hc, repo, "fixture/repo", "origin", "main", "local-harness-smoke/001", nil, slog.New(slog.DiscardHandler), nil)
		if factory == nil {
			t.Fatal("local review configuration did not enable review")
		}
		service, err := factory(cfg.ReviewHarness())
		if err != nil {
			t.Fatal(err)
		}
		route, err := service.(*reviewinvoke.Invoker).ResolveRoute(ctx, task, "github:fixture/repo#1", strings.Repeat("a", 40))
		if err != nil {
			t.Fatal(err)
		}
		rt, err := buildReviewHarness(cfg.ReviewHarness(), cfg, hc)
		if err != nil {
			t.Fatal(err)
		}
		if err := rt.AuthCheck(ctx); err != nil {
			t.Fatal(err)
		}
		workdir := liveHarnessWorkdir(t)
		stateDir := t.TempDir()
		if err := runtime.PrepareReviewAuthentication(ctx, stateDir, cfg.ReviewHarness()); err != nil {
			t.Fatal(err)
		}
		t.Logf("review: backend=%s model=%s effort=%s", cfg.ReviewHarness(), route.Model, route.Effort)
		handle, err := rt.Launch(ctx, port.LaunchSpec{
			Purpose: port.LaunchPurposeReview, Workdir: workdir, RuntimeStateDir: stateDir,
			Model: route.Model, Effort: route.Effort,
			Brief: "Read review-input.txt in the current directory. Reply with its exact contents. Do not edit files or run any write commands.",
		})
		if err != nil {
			t.Fatal(err)
		}
		result, err := rt.Await(ctx, handle)
		if err != nil {
			t.Fatal(err)
		}
		if result.Incomplete || !strings.Contains(result.Output, "PYXIS_REVIEW_OK") {
			t.Fatalf("review did not read the fixture: incomplete=%v reason=%s output=%q", result.Incomplete, result.Reason, result.Output)
		}
		if got := liveHarnessGit(t, workdir, "status", "--porcelain"); got != "" {
			t.Fatalf("review modified its input: %s", got)
		}
	})
}

func liveHarnessWorkdir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	liveHarnessGit(t, dir, "init", "--initial-branch=main")
	liveHarnessGit(t, dir, "config", "user.name", "Pyxis local smoke")
	liveHarnessGit(t, dir, "config", "user.email", "pyxis-smoke@example.invalid")
	if err := os.WriteFile(filepath.Join(dir, "review-input.txt"), []byte("PYXIS_REVIEW_OK\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	liveHarnessGit(t, dir, "add", "review-input.txt")
	liveHarnessGit(t, dir, "commit", "-m", "chore: seed local harness smoke")
	return dir
}

func liveHarnessGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("fixture git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}
