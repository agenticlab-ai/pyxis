package reviewinvoke

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/adapters/harness/claude"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/harnessconfig"
	"github.com/agenticlab-ai/pyxis/internal/harnessconfig/harnessconfigtest"
	"github.com/agenticlab-ai/pyxis/internal/taskrouting"
)

func fixtureHarnessConfig(t *testing.T) harnessconfig.Config {
	t.Helper()
	cfg, err := harnessconfigtest.Config()
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestNativeTaskPinsIndependentReviewPair(t *testing.T) {
	rt, err := claude.New(claude.WithProfiles(fixtureHarnessConfig(t).Profiles("claude")))
	if err != nil {
		t.Fatal(err)
	}
	i := &Invoker{runtime: rt, harness: "claude", now: time.Now}
	task := port.Task{ID: "T1", DemandID: "native-identity", NativeRouting: &taskrouting.Spec{Schema: taskrouting.Schema,
		Implementation: taskrouting.Route{Harness: "codex", Model: "gpt-5.6-sol", Effort: "high"},
		Review:         taskrouting.Route{Harness: "claude", Model: "claude-opus-5", Effort: "high"}, Rationale: "Independent review"}}
	route, err := i.resolveNativeRoute(task, "github:o/r#12", "revision")
	if err != nil {
		t.Fatal(err)
	}
	if route.Model != "claude-opus-5" || route.Effort != "high" {
		t.Fatalf("changed review: %+v", route)
	}
	if _, err := requestedReviewProfile(task); err != nil {
		t.Fatal(err)
	}
	task.NativeRouting.Review.Harness = "codex"
	if _, err := i.resolveNativeRoute(task, "github:o/r#12", "revision"); err == nil {
		t.Fatal("accepted wrong review harness")
	}
}

func TestNativeReviewRejectsInvalidSourceAndLaunch(t *testing.T) {
	adapter, err := claude.New(claude.WithProfiles(fixtureHarnessConfig(t).Profiles("claude")))
	if err != nil {
		t.Fatal(err)
	}
	for _, failure := range []string{"schema", "pair", "build", "capture", "mismatch", "identity"} {
		t.Run(failure, func(t *testing.T) {
			rt := &coverageNativeFailureRuntime{Adapter: adapter}
			i := &Invoker{runtime: rt, harness: "claude", now: time.Now}
			task := port.Task{ID: "T1", DemandID: "native", NativeRouting: &taskrouting.Spec{Schema: taskrouting.Schema, Implementation: taskrouting.Route{Harness: "codex", Model: "gpt-5.6-sol", Effort: "high"}, Review: taskrouting.Route{Harness: "claude", Model: "claude-opus-5", Effort: "high"}, Rationale: "Independent review"}}
			switch failure {
			case "schema":
				task.NativeRouting.Schema = "invalid"
			case "pair":
				task.NativeRouting.Review.Model = "unknown"
			case "build":
				rt.buildErr = errors.New("build refused")
			case "capture":
				rt.captureErr = errors.New("capture refused")
			case "mismatch":
				rt.mismatch = true
			case "identity":
				task.DemandID = ""
				if _, err := requestedReviewProfile(task); err == nil {
					t.Fatal("accepted missing identity")
				}
				return
			}
			if _, err := i.resolveNativeRoute(task, "o/r#1", "revision"); err == nil {
				t.Fatal("accepted invalid native review")
			}
		})
	}
}

func TestNativeReviewRecoveryPreservesFrozenRoute(t *testing.T) {
	rt, err := claude.New(claude.WithProfiles(fixtureHarnessConfig(t).Profiles("claude")))
	if err != nil {
		t.Fatal(err)
	}
	i := lifecycleInvoker(t, rt, &fakeVerifier{}, time.Minute)
	i.WithProposalInspector(fakeProposalInspector{})
	i.route = nil
	task := port.Task{ID: "T-native", Number: 17, DemandID: "native-source-digest", NativeRouting: &taskrouting.Spec{Schema: taskrouting.Schema,
		Implementation: taskrouting.Route{Harness: "codex", Model: "gpt-5.6-sol", Effort: "high"},
		Review:         taskrouting.Route{Harness: "claude", Model: "claude-opus-5", Effort: "high"}, Rationale: "Review separately"}}
	proposal := port.Proposal{Number: 77}
	capture := i.Capture(t.Context(), task, proposal)
	if capture.Err != nil {
		t.Fatal(capture.Err)
	}
	if err := i.CompleteHandoff(t.Context(), capture); err != nil {
		t.Fatal(err)
	}
	recovered, _, err := i.PrepareSelected(t.Context(), capture.ObligationID, "native-restart")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(recovered.NativeRouting, task.NativeRouting) || recovered.DemandID != task.DemandID || recovered.DemandProfile.Capability != "" {
		t.Fatalf("recovery changed native contract: %+v", recovered)
	}
	route, err := i.resolveNativeRoute(recovered, capture.ProposalRef, capture.Revision)
	if err != nil || route.Model != "claude-opus-5" || route.Effort != "high" {
		t.Fatalf("recovered route %+v %v", route, err)
	}
}
