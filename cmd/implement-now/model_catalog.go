package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/nativerouting"
)

// nativeModelRuntime is the small adapter surface needed to turn semantic
// demand into a launchable model and effort pair. It is shared by the normal
// local router and the optional evidence-backed admission path.
type nativeModelRuntime interface {
	port.AgentRuntime
	port.CatalogProvider
	port.LaunchBuilder
	port.NativeLaunchCapturer
	nativerouting.NativeCapability
}

type modelRoutingClock struct{}

func (modelRoutingClock) Now() time.Time { return time.Now().UTC() }

func validateModelCatalog(rt port.AgentRuntime, role nativerouting.Role) (nativeModelRuntime, nativerouting.ValidatedCatalog, error) {
	native, ok := rt.(nativeModelRuntime)
	if !ok {
		return nil, nativerouting.ValidatedCatalog{}, errors.New("runtime lacks catalog, capability, launch builder, or launch capture")
	}
	catalog, qualifications, err := native.Catalog(role)
	if err != nil {
		return nil, nativerouting.ValidatedCatalog{}, err
	}
	validated, diagnostics := nativerouting.ValidateCatalog(catalog, qualifications, native, modelRoutingClock{}, nativerouting.ReleasedCatalogAuthorization{})
	if len(diagnostics) != 0 {
		return nil, nativerouting.ValidatedCatalog{}, fmt.Errorf("validate %s/%s native catalog: %v", catalog.Harness, role, diagnostics)
	}
	return native, validated, nil
}

func verifyModelSelection(native nativeModelRuntime, resolution nativerouting.Resolution) error {
	if resolution.NativeProfile == nil {
		return fmt.Errorf("semantic route did not resolve: class=%s reason=%s defects=%v unmet=%v", resolution.Class, resolution.Reason, resolution.PolicyDefects, resolution.UnmetComponents)
	}
	spec, err := native.Build(*resolution.NativeProfile)
	if err != nil {
		return fmt.Errorf("build selected native profile: %w", err)
	}
	capture, err := native.CaptureNativeLaunch(spec)
	if err != nil {
		return fmt.Errorf("capture selected native launch: %w", err)
	}
	if !capture.MatchesSelected(*resolution.NativeProfile) {
		return errors.New("selected native profile differs from actual launch capture")
	}
	return nil
}

func verifyRequestedProfile(task port.Task) error {
	if task.NativeRouting != nil {
		if task.DemandID == "" {
			return fmt.Errorf("task %s lacks frozen native identity", task.ID)
		}
		return task.NativeRouting.Validate()
	}
	_, err := task.ImplementationDemand()
	return err
}
