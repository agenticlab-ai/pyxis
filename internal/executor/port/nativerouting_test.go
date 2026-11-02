package port

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/nativerouting"
)

type catalogProviderFixture struct{}

func (catalogProviderFixture) Catalog(nativerouting.Role) (nativerouting.Catalog, nativerouting.QualificationSet, error) {
	return nativerouting.Catalog{}, nil, errors.New("fixture")
}

type launchBuilderFixture struct{}

func (launchBuilderFixture) Build(nativerouting.SelectedProfile) (NativeLaunchSpec, error) {
	return NativeLaunchSpec{}, nil
}

var _ CatalogProvider = catalogProviderFixture{}
var _ LaunchBuilder = launchBuilderFixture{}

func TestLaunchCaptureMatchesSelectedExactly(t *testing.T) {
	budget := int64(42)
	selected := nativerouting.SelectedProfile{
		NativeProfileID: "profile-1", NativeModel: "model-1", NativeEffort: "deep",
		Controls: []nativerouting.NativeControl{
			{Name: "sandbox", Type: nativerouting.ControlBoolean, BooleanValue: new(true)},
			{Name: "token_budget", Type: nativerouting.ControlInteger, IntegerValue: &budget},
		},
	}
	capture := LaunchCapture{NativeModel: "model-1", NativeEffort: "deep", Controls: selected.Controls}
	if !capture.MatchesSelected(selected) {
		t.Fatal("equal launch capture did not match selection")
	}
	for name, mutate := range map[string]func(*LaunchCapture){
		"model":   func(c *LaunchCapture) { c.NativeModel = "model-2" },
		"effort":  func(c *LaunchCapture) { c.NativeEffort = "light" },
		"control": func(c *LaunchCapture) { c.Controls = nil },
	} {
		t.Run(name, func(t *testing.T) {
			changed := capture
			mutate(&changed)
			if changed.MatchesSelected(selected) {
				t.Fatal("launch mismatch was accepted")
			}
		})
	}
	reordered := capture
	reordered.Controls = append([]nativerouting.NativeControl(nil), capture.Controls...)
	reordered.Controls[0], reordered.Controls[1] = reordered.Controls[1], reordered.Controls[0]
	if !reordered.MatchesSelected(selected) {
		t.Fatal("control collection order changed launch equality")
	}
}

func TestVerifyLaunchFidelityFailsAttemptAndProducesStableInvalidation(t *testing.T) {
	selected := nativerouting.SelectedProfile{QualificationID: "qualification-1", NativeModel: "model-1", NativeEffort: "deep", Controls: []nativerouting.NativeControl{}}
	captured := LaunchCapture{NativeModel: "model-2", NativeEffort: "deep", Controls: []nativerouting.NativeControl{}}
	at := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)

	first, err := VerifyLaunchFidelity(selected, captured, at)
	if !errors.Is(err, ErrLaunchFidelity) || first == nil || !strings.Contains(err.Error(), selected.QualificationID) {
		t.Fatalf("mismatch did not fail and name its qualification: invalidation=%#v err=%v", first, err)
	}
	second, err := VerifyLaunchFidelity(selected, captured, at)
	if err == nil || *first != *second {
		t.Fatalf("invalidation is not reproducible: first=%#v second=%#v err=%v", first, second, err)
	}
	if invalidation, err := VerifyLaunchFidelity(selected, LaunchCapture{NativeModel: selected.NativeModel, NativeEffort: selected.NativeEffort, Controls: []nativerouting.NativeControl{}}, at); invalidation != nil || err != nil {
		t.Fatalf("equal launch failed fidelity: invalidation=%#v err=%v", invalidation, err)
	}
}

type routedRuntime struct{ model, effort string }

func (routedRuntime) Launch(context.Context, LaunchSpec) (Handle, error) { return nil, nil }
func (routedRuntime) Await(context.Context, Handle) (RunResult, error) {
	return RunResult{}, nil
}
func (routedRuntime) Kill(context.Context, Handle) error { return nil }
func (routedRuntime) AuthCheck(context.Context) error    { return nil }
func (r routedRuntime) ModelRoute() ModelRoute {
	return ModelRoute{Model: r.model, Effort: r.effort}
}

type unroutedRuntime struct{}

func (unroutedRuntime) Launch(context.Context, LaunchSpec) (Handle, error) { return nil, nil }
func (unroutedRuntime) Await(context.Context, Handle) (RunResult, error) {
	return RunResult{}, nil
}
func (unroutedRuntime) Kill(context.Context, Handle) error { return nil }
func (unroutedRuntime) AuthCheck(context.Context) error    { return nil }

func TestRouteOf(t *testing.T) {
	if _, err := RouteOf(unroutedRuntime{}); err == nil {
		t.Fatal("runtime without a route was accepted")
	}
	for _, rt := range []routedRuntime{{effort: "low"}, {model: "m"}} {
		if _, err := RouteOf(rt); err == nil {
			t.Fatalf("empty route %+v was accepted", rt)
		}
	}
	got, err := RouteOf(routedRuntime{model: "m", effort: "low"})
	if err != nil || got.Model != "m" || got.Effort != "low" {
		t.Fatalf("RouteOf = %+v, %v", got, err)
	}
}
