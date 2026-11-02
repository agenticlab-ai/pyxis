package runtime

import (
	"errors"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/nativerouting"
)

type conformanceProvider func(nativerouting.Role) (nativerouting.Catalog, nativerouting.QualificationSet, error)

func (f conformanceProvider) Catalog(role nativerouting.Role) (nativerouting.Catalog, nativerouting.QualificationSet, error) {
	return f(role)
}

type conformanceCapability struct{}

func (conformanceCapability) ValidateProfile(nativerouting.CatalogEntry) error { return nil }

type conformanceBuilder struct{}

func (conformanceBuilder) Build(nativerouting.SelectedProfile) (port.NativeLaunchSpec, error) {
	return port.NativeLaunchSpec{}, nil
}

type conformanceClock struct{}

func (conformanceClock) Now() time.Time { return time.Unix(1, 0).UTC() }

type conformanceAuthorization struct{}

func (conformanceAuthorization) Authorize(nativerouting.AuthorizationProof) error { return nil }

func conformanceConfig(provider port.CatalogProvider) NativeRoutingConformanceConfig {
	return NativeRoutingConformanceConfig{
		Harness: "test", Provider: provider, Capability: conformanceCapability{}, Builder: conformanceBuilder{},
		Clock: conformanceClock{}, Authorization: conformanceAuthorization{},
		LaunchCapture: func(port.NativeLaunchSpec) (port.LaunchCapture, error) { return port.LaunchCapture{}, nil },
		Cases:         []NativeRoutingConformanceCase{{Name: "fixture", Role: nativerouting.RoleImplementation}},
	}
}

func TestNativeRoutingConformanceRejectsBadWiringAndCatalogs(t *testing.T) {
	if err := CheckNativeRoutingConformance(NativeRoutingConformanceConfig{}); err == nil {
		t.Fatal("accepted incomplete wiring")
	}
	cfg := conformanceConfig(conformanceProvider(func(nativerouting.Role) (nativerouting.Catalog, nativerouting.QualificationSet, error) {
		return nativerouting.Catalog{}, nil, nil
	}))
	cfg.Harness = ""
	if err := CheckNativeRoutingConformance(cfg); err == nil {
		t.Fatal("accepted empty harness")
	}

	want := errors.New("catalog unavailable")
	if err := CheckNativeRoutingConformance(conformanceConfig(conformanceProvider(func(nativerouting.Role) (nativerouting.Catalog, nativerouting.QualificationSet, error) {
		return nativerouting.Catalog{}, nil, want
	}))); !errors.Is(err, want) {
		t.Fatalf("catalog error = %v", err)
	}
	if err := CheckNativeRoutingConformance(conformanceConfig(conformanceProvider(func(role nativerouting.Role) (nativerouting.Catalog, nativerouting.QualificationSet, error) {
		return nativerouting.Catalog{Harness: "other", Role: role}, nil, nil
	}))); err == nil {
		t.Fatal("accepted a catalog for another harness")
	}
	if err := CheckNativeRoutingConformance(conformanceConfig(conformanceProvider(func(role nativerouting.Role) (nativerouting.Catalog, nativerouting.QualificationSet, error) {
		return nativerouting.Catalog{Harness: "test", Role: role}, nil, nil
	}))); err == nil {
		t.Fatal("accepted an invalid empty catalog")
	}
}
