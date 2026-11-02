package main

import (
	"errors"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/config"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/nativerouting"
	"github.com/agenticlab-ai/pyxis/internal/taskrouting"
)

func TestNativeTaskPinsImplementationPair(t *testing.T) {
	task := port.Task{ID: "T1", NativeRouting: &taskrouting.Spec{Schema: taskrouting.Schema,
		Implementation: taskrouting.Route{Harness: "codex", Model: "gpt-5.6-sol", Effort: "high"},
		Review:         taskrouting.Route{Harness: "claude", Model: "claude-opus-5", Effort: "max"}, Rationale: "Cross-package change"}}
	// A conflicting ambient harness and empty semantic demand cannot change
	// the task's frozen native route.
	resolve := newRuntimeResolver(t.Context(), config.Defaults(), "claude", fixtureHarnessConfig(t), nil)
	h, rt, err := resolve(task)
	if err != nil {
		t.Fatal(err)
	}
	route, err := port.RouteOf(rt)
	if err != nil {
		t.Fatal(err)
	}
	if h != "codex" || route.Model != "gpt-5.6-sol" || route.Effort != "high" {
		t.Fatalf("changed native choice: %s %+v", h, route)
	}
	task.NativeRouting.Implementation.Effort = "low"
	if _, _, err := resolve(task); err == nil {
		t.Fatal("unsupported pair fell back to a different native model")
	}
}

func TestNativeImplementationRejectsInvalidSourceAndLaunch(t *testing.T) {
	catalog, qualifications := coverageActiveCatalogFixture(nativerouting.RoleImplementation)
	rt := &coverageNativeRuntime{catalog: catalog, qualifications: qualifications}
	task := port.Task{ID: "T1", NativeRouting: &taskrouting.Spec{Schema: taskrouting.Schema, Implementation: taskrouting.Route{Harness: "codex", Model: "native-economy", Effort: "low"}, Review: taskrouting.Route{Harness: "claude", Model: "claude-opus-5", Effort: "high"}, Rationale: "Explicit native selection"}}
	if _, err := resolveImplementationModel(task, "claude", rt); err == nil {
		t.Fatal("accepted different harness")
	}
	if _, err := resolveImplementationModel(task, "codex", coverageAgentRuntime{}); err == nil {
		t.Fatal("accepted runtime without native capability")
	}
	rt.buildErr = errors.New("launch refused")
	if _, err := resolveImplementationModel(task, "codex", rt); err == nil {
		t.Fatal("accepted failed native launch build")
	}
	task.NativeRouting.Schema = "invalid"
	if _, err := resolveImplementationModel(task, "codex", rt); err == nil {
		t.Fatal("accepted invalid native contract")
	}
	resolve := newRuntimeResolver(t.Context(), config.Defaults(), "codex", fixtureHarnessConfig(t), nil)
	if _, _, err := resolve(task); err == nil {
		t.Fatal("resolver accepted invalid native contract")
	}
}
