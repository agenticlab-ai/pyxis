package port

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/nativerouting"
)

// CatalogProvider is the consumer-owned boundary every runtime adapter uses to
// publish separate implementation and review policy data. Catalog performs no
// probing or network access; the composition validates the returned unit.
type CatalogProvider interface {
	Catalog(role nativerouting.Role) (nativerouting.Catalog, nativerouting.QualificationSet, error)
}

// NativeLaunchSpec is the typed, argument-free native portion of a launch.
// The Worker later composes it with process metadata such as brief/workdir.
type NativeLaunchSpec struct {
	NativeProfileID string                        `json:"native_profile_id"`
	Role            nativerouting.Role            `json:"role"`
	NativeModel     string                        `json:"native_model"`
	NativeEffort    string                        `json:"native_effort"`
	Controls        []nativerouting.NativeControl `json:"controls"`
}

// ModelRoute is the complete local output of model routing. It deliberately
// contains only the two values an AgentRuntime needs to launch a task.
type ModelRoute struct {
	Model  string
	Effort string
}

// ModelRouteProvider is implemented by a task-scoped runtime after its model
// and effort have been resolved. Launches and fixers read this route; the
// AgentRuntime itself does not select a model.
type ModelRouteProvider interface {
	ModelRoute() ModelRoute
}

// RouteOf reads the resolved model route. A runtime that has not been wrapped
// with one, or that resolved an empty model or effort, fails closed.
func RouteOf(rt AgentRuntime) (ModelRoute, error) {
	provider, ok := rt.(ModelRouteProvider)
	if !ok {
		return ModelRoute{}, errors.New("agent runtime has no resolved model route")
	}
	route := provider.ModelRoute()
	if route.Model == "" || route.Effort == "" {
		return ModelRoute{}, errors.New("agent runtime resolved an empty model route")
	}
	return route, nil
}

// LaunchBuilder translates only a resolver-selected profile. It cannot see
// semantic demand and therefore cannot reinterpret or weaken it.
type LaunchBuilder interface {
	Build(nativerouting.SelectedProfile) (NativeLaunchSpec, error)
}

// NativeLaunchCapturer translates a typed spec through the adapter's real
// argv builder and closed decoder without starting a process. Conformance uses
// it to inspect the exact native settings that would cross the process seam.
type NativeLaunchCapturer interface {
	CaptureNativeLaunch(NativeLaunchSpec) (LaunchCapture, error)
}

// LaunchCapture is the adapter-observed native launch identity. It is retained
// independently from the requested semantic profile for exact fidelity checks.
type LaunchCapture struct {
	NativeModel  string                        `json:"native_model"`
	NativeEffort string                        `json:"native_effort"`
	Controls     []nativerouting.NativeControl `json:"controls"`
}

var ErrLaunchFidelity = errors.New("native launch differs from selected profile")

// LaunchFidelity is the single selected-versus-captured equality predicate.
// Collection order is non-semantic, while missing or surplus controls fail.
func LaunchFidelity(selected nativerouting.SelectedProfile, captured LaunchCapture) bool {
	return captured.NativeModel == selected.NativeModel &&
		captured.NativeEffort == selected.NativeEffort &&
		reflect.DeepEqual(canonicalLaunchControls(captured.Controls), canonicalLaunchControls(selected.Controls))
}

// VerifyLaunchFidelity turns a mismatch into both a failed attempt and the
// additive invalidation required before the qualification can be reused.
func VerifyLaunchFidelity(selected nativerouting.SelectedProfile, captured LaunchCapture, at time.Time) (*nativerouting.Invalidation, error) {
	if LaunchFidelity(selected, captured) {
		return nil, nil
	}
	payload, err := json.Marshal(struct {
		QualificationID string                        `json:"qualification_id"`
		SelectedModel   string                        `json:"selected_model"`
		SelectedEffort  string                        `json:"selected_effort"`
		SelectedControl []nativerouting.NativeControl `json:"selected_controls"`
		CapturedModel   string                        `json:"captured_model"`
		CapturedEffort  string                        `json:"captured_effort"`
		CapturedControl []nativerouting.NativeControl `json:"captured_controls"`
		At              time.Time                     `json:"at"`
	}{
		QualificationID: selected.QualificationID,
		SelectedModel:   selected.NativeModel, SelectedEffort: selected.NativeEffort, SelectedControl: canonicalLaunchControls(selected.Controls),
		CapturedModel: captured.NativeModel, CapturedEffort: captured.NativeEffort, CapturedControl: canonicalLaunchControls(captured.Controls),
		At: at.UTC(),
	})
	if err != nil {
		return nil, fmt.Errorf("encode launch fidelity evidence: %w", err)
	}
	digest := sha256.Sum256(payload)
	invalidation := &nativerouting.Invalidation{
		EventID: "native-launch-mismatch-" + hex.EncodeToString(digest[:8]), At: at.UTC(),
		Reason: "captured native model, effort, or typed controls differ from selected profile",
	}
	return invalidation, fmt.Errorf("%w: qualification %q", ErrLaunchFidelity, selected.QualificationID)
}

// MatchesSelected is strict: omissions and surplus controls are mismatches.
// Control collection order is not launch behavior, so both sides are compared
// in canonical name order.
func (c LaunchCapture) MatchesSelected(selected nativerouting.SelectedProfile) bool {
	return LaunchFidelity(selected, c)
}

func canonicalLaunchControls(controls []nativerouting.NativeControl) []nativerouting.NativeControl {
	out := append([]nativerouting.NativeControl(nil), controls...)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
