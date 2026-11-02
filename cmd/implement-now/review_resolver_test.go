package main

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/entity"
	"github.com/agenticlab-ai/pyxis/internal/executiondemand"
	"github.com/agenticlab-ai/pyxis/internal/executor/config"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/reviewinvoke"
)

func routedTask(id string, profile executiondemand.ProfileMinimum) port.Task {
	return port.Task{ID: id, DemandID: executiondemand.DemandID(strings.Repeat("a", 64)), DemandProfile: profile}
}

func reviewFactoryRepoDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestReviewFactoryAbsentIsUninstalled(t *testing.T) {
	factory := newReviewFactory(
		config.Defaults(), fixtureHarnessConfig(t), "/repo", "agenticlab-ai/pyxis", "origin", "main", "feature/001", nil, slog.New(slog.DiscardHandler), nil,
	)
	if factory != nil {
		t.Fatal("an absent executor.review block installed a reviewer")
	}
}

func TestReviewFactoryConfiguredWithoutBackendUsesTypedInheritance(t *testing.T) {
	cfg, err := config.Parse([]byte("executor:\n  review:\n    budget_minutes: 9\n"))
	if err != nil {
		t.Fatal(err)
	}
	factory := newReviewFactory(cfg, fixtureHarnessConfig(t), reviewFactoryRepoDir(t), "owner/repo", "origin", "main", "feature/001", nil, slog.New(slog.DiscardHandler), nil)
	if factory == nil {
		t.Fatal("configured inherited review block was treated as absent")
	}
	inv, err := factory("codex")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := inv.(*reviewinvoke.Invoker); !ok {
		t.Fatalf("factory returned %T", inv)
	}
}

// This exercises the real buildBackend composition for all published adapters.
// Review resolves against the independently qualified role=review catalog.
func TestConfiguredReviewFactoryUsesRoleQualifiedCatalog(t *testing.T) {
	tests := []struct {
		backend string
		profile string
		model   string
		effort  string
		minimum string
	}{
		{backend: "claude", profile: "opus-max-review", model: "claude-opus-5", effort: "max", minimum: "haiku-medium-review"},
		{backend: "codex", profile: "codex-review-maximum", model: "gpt-5.6-sol", effort: "xhigh", minimum: "codex-review-economy"},
		{backend: "cursor", profile: "review-grok-4-5-high", model: "cursor-grok-4.5-high", effort: "high", minimum: "review-composer-2-5"},
		{backend: "opencode", profile: "review-frontier", model: "anthropic/claude-opus-4-8", effort: "none", minimum: "review-standard"},
	}
	for _, tc := range tests {
		t.Run(tc.backend, func(t *testing.T) {
			cfg, err := config.Parse(fmt.Appendf(nil, "executor:\n  review:\n    backend: %s\n  caps:\n    worker_turns: 42\n", tc.backend))
			if err != nil {
				t.Fatalf("parse executor config: %v", err)
			}
			factory := newReviewFactory(cfg, fixtureHarnessConfig(t), reviewFactoryRepoDir(t), "other/repository", "origin", "main", "feature/001", nil, slog.New(slog.DiscardHandler), nil)
			if factory == nil {
				t.Fatal("explicit review backend did not install a factory")
			}
			configured, err := factory(entity.Harness(tc.backend))
			if err != nil {
				t.Fatalf("build configured reviewer: %v", err)
			}
			inv, ok := configured.(*reviewinvoke.Invoker)
			if !ok {
				t.Fatalf("factory returned %T, want production *reviewinvoke.Invoker", configured)
			}
			inv.WithClock(func() time.Time {
				return time.Date(2026, time.August, 5, 12, 0, 0, 0, time.UTC)
			})
			route, err := inv.ResolveRoute(
				context.Background(), routedTask("T4", executiondemand.ProfileMinimum{Capability: executiondemand.CapabilityFrontier, Deliberation: executiondemand.DeliberationMaximum, Context: executiondemand.ContextExtended, Assurance: executiondemand.AssuranceCritical}), "github:other/repository#1034", "abc123",
			)
			if err != nil {
				t.Fatalf("resolve production review route: %v", err)
			}
			if route.NativeProfileID != tc.profile || route.Model != tc.model || route.Effort != tc.effort {
				t.Fatalf("route = %+v, want profile/model/effort %s/%s/%s", route, tc.profile, tc.model, tc.effort)
			}
			if route.CatalogID == "" || route.PolicyIdentity == "" {
				t.Fatalf("route omitted catalog audit identity: %+v", route)
			}
			inv.WithClock(func() time.Time {
				return time.Date(2030, time.August, 5, 12, 0, 0, 0, time.UTC)
			})
			minimum, err := inv.ResolveRoute(
				context.Background(), routedTask("T1", executiondemand.ProfileMinimum{Capability: executiondemand.CapabilityEconomy, Deliberation: executiondemand.DeliberationLight, Context: executiondemand.ContextStandard, Assurance: executiondemand.AssuranceOrdinary}), "github:other/repository#1034", "abc123",
			)
			if err != nil {
				t.Fatalf("resolve minimum review route: %v", err)
			}
			if minimum.NativeProfileID != tc.minimum {
				t.Fatalf("minimum review profile = %q, want %q", minimum.NativeProfileID, tc.minimum)
			}
		})
	}
}
