package runtime_test

import (
	"errors"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/adapters/harness/claude"
	"github.com/agenticlab-ai/pyxis/internal/executiondemand"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/runtime"
	"github.com/agenticlab-ai/pyxis/internal/nativerouting"
)

type conformanceClock struct{}

func (conformanceClock) Now() time.Time {
	return time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC)
}

type conformanceAuthorization struct{}

func (conformanceAuthorization) Authorize(nativerouting.AuthorizationProof) error { return nil }

type failingBuilder struct{}

func (failingBuilder) Build(nativerouting.SelectedProfile) (port.NativeLaunchSpec, error) {
	return port.NativeLaunchSpec{}, errors.New("cannot build")
}

type weakestOnlyProvider struct{ base port.CatalogProvider }

func (p weakestOnlyProvider) Catalog(role nativerouting.Role) (nativerouting.Catalog, nativerouting.QualificationSet, error) {
	catalog, qualifications, err := p.base.Catalog(role)
	if err != nil {
		return nativerouting.Catalog{}, nil, err
	}
	entry := catalog.Entries[0]
	entry.Strongest = true
	catalog.Entries = []nativerouting.CatalogEntry{entry}
	for _, qualification := range qualifications {
		if qualification.QualificationID == entry.QualificationID {
			return catalog, nativerouting.QualificationSet{qualification}, nil
		}
	}
	return nativerouting.Catalog{}, nil, errors.New("weakest qualification missing")
}

func TestNativeRoutingConformanceThroughRealAdapter(t *testing.T) {
	adapter, err := claude.New()
	if err != nil {
		t.Fatal(err)
	}
	var cases []runtime.NativeRoutingConformanceCase
	for _, role := range []nativerouting.Role{nativerouting.RoleImplementation, nativerouting.RoleReview} {
		catalog, _, err := adapter.Catalog(role)
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range catalog.Entries {
			cases = append(cases, runtime.NativeRoutingConformanceCase{
				Name: entry.NativeProfileID, Role: role,
				Requested: executiondemand.ProfileMinimum{
					Capability: entry.Provides.Capability, Deliberation: entry.Provides.Deliberation,
					Context: entry.Provides.Context, Assurance: entry.Provides.Assurance,
				},
				WantClass: nativerouting.ResolutionExact, WantNativeProfileID: entry.NativeProfileID,
			})
		}
	}
	if err := runtime.CheckNativeRoutingConformance(runtime.NativeRoutingConformanceConfig{
		Harness: "claude", Provider: adapter, Capability: adapter, Builder: adapter,
		Clock: conformanceClock{}, Authorization: conformanceAuthorization{},
		LaunchCapture: adapter.CaptureNativeLaunch, Cases: cases,
	}); err != nil {
		t.Fatal(err)
	}
}

func TestNativeRoutingConformanceRejectsUnknownRoleAndWrongExpectation(t *testing.T) {
	adapter, err := claude.New()
	if err != nil {
		t.Fatal(err)
	}
	base := func(cases []runtime.NativeRoutingConformanceCase) runtime.NativeRoutingConformanceConfig {
		return runtime.NativeRoutingConformanceConfig{
			Harness: "claude", Provider: adapter, Capability: adapter, Builder: adapter,
			Clock: conformanceClock{}, Authorization: conformanceAuthorization{},
			LaunchCapture: adapter.CaptureNativeLaunch, Cases: cases,
		}
	}
	if err := runtime.CheckNativeRoutingConformance(base([]runtime.NativeRoutingConformanceCase{{Name: "unknown", Role: "unknown"}})); err == nil {
		t.Fatal("accepted a case for an unknown role")
	}
	catalog, _, err := adapter.Catalog(nativerouting.RoleImplementation)
	if err != nil {
		t.Fatal(err)
	}
	entry := catalog.Entries[0]
	requested := executiondemand.ProfileMinimum{
		Capability: entry.Provides.Capability, Deliberation: entry.Provides.Deliberation,
		Context: entry.Provides.Context, Assurance: entry.Provides.Assurance,
	}
	if err := runtime.CheckNativeRoutingConformance(base([]runtime.NativeRoutingConformanceCase{{
		Name: "wrong class", Role: nativerouting.RoleImplementation, Requested: requested,
		WantClass: nativerouting.ResolutionPromoted, WantNativeProfileID: entry.NativeProfileID,
	}})); err == nil {
		t.Fatal("accepted an incorrect expected resolution class")
	}
	if err := runtime.CheckNativeRoutingConformance(base([]runtime.NativeRoutingConformanceCase{{
		Name: "wrong profile", Role: nativerouting.RoleImplementation, Requested: requested,
		WantClass: nativerouting.ResolutionExact, WantNativeProfileID: "not-the-selected-profile",
	}})); err == nil {
		t.Fatal("accepted an incorrect expected native profile")
	}
	buildFailure := base([]runtime.NativeRoutingConformanceCase{{
		Name: "build failure", Role: nativerouting.RoleImplementation, Requested: requested,
		WantClass: nativerouting.ResolutionExact, WantNativeProfileID: entry.NativeProfileID,
	}})
	buildFailure.Builder = failingBuilder{}
	if err := runtime.CheckNativeRoutingConformance(buildFailure); err == nil {
		t.Fatal("ignored a native launch build failure")
	}
	captureFailure := base(buildFailure.Cases)
	captureFailure.LaunchCapture = func(port.NativeLaunchSpec) (port.LaunchCapture, error) {
		return port.LaunchCapture{}, errors.New("cannot capture")
	}
	if err := runtime.CheckNativeRoutingConformance(captureFailure); err == nil {
		t.Fatal("ignored a native launch capture failure")
	}
	unsupported := requested
	unsupported.Capability = executiondemand.Capability("beyond-frontier")
	if err := runtime.CheckNativeRoutingConformance(base([]runtime.NativeRoutingConformanceCase{{
		Name: "unsupported profile leak", Role: nativerouting.RoleImplementation, Requested: unsupported,
		WantClass: nativerouting.ResolutionUnsupported, WantNativeProfileID: "must-not-exist",
	}})); err == nil {
		t.Fatal("accepted an expected profile for an unsupported resolution")
	}
}

func TestNativeRoutingConformanceAcceptsUnsupportedCellsWithoutProfiles(t *testing.T) {
	adapter, err := claude.New()
	if err != nil {
		t.Fatal(err)
	}
	provider := weakestOnlyProvider{base: adapter}
	review, _, err := provider.Catalog(nativerouting.RoleReview)
	if err != nil {
		t.Fatal(err)
	}
	unsupported := executiondemand.ProfileMinimum{
		Capability: executiondemand.CapabilityFrontier, Deliberation: executiondemand.DeliberationMaximum,
		Context: executiondemand.ContextExtended, Assurance: executiondemand.AssuranceCritical,
	}
	reviewEntry := review.Entries[0]
	reviewRequest := executiondemand.ProfileMinimum{
		Capability: reviewEntry.Provides.Capability, Deliberation: reviewEntry.Provides.Deliberation,
		Context: reviewEntry.Provides.Context, Assurance: reviewEntry.Provides.Assurance,
	}
	if err := runtime.CheckNativeRoutingConformance(runtime.NativeRoutingConformanceConfig{
		Harness: "claude", Provider: provider, Capability: adapter, Builder: adapter,
		Clock: conformanceClock{}, Authorization: conformanceAuthorization{}, LaunchCapture: adapter.CaptureNativeLaunch,
		Cases: []runtime.NativeRoutingConformanceCase{
			{Name: "unsupported implementation", Role: nativerouting.RoleImplementation, Requested: unsupported, WantClass: nativerouting.ResolutionUnsupported},
			{Name: "supported review", Role: nativerouting.RoleReview, Requested: reviewRequest, WantClass: nativerouting.ResolutionExact, WantNativeProfileID: reviewEntry.NativeProfileID},
		},
	}); err != nil {
		t.Fatalf("unsupported conformance cell: %v", err)
	}
}
