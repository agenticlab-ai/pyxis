package runtime

import (
	"errors"
	"fmt"
	"github.com/agenticlab-ai/pyxis/internal/entity"

	"github.com/agenticlab-ai/pyxis/internal/executiondemand"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/nativerouting"
)

// NativeRoutingConformanceCase is one required harness-role semantic cell.
// Callers enumerate their complete derivable domain rather than letting the
// shared suite infer or silently omit support.
type NativeRoutingConformanceCase struct {
	Name                string
	Role                nativerouting.Role
	Requested           executiondemand.ProfileMinimum
	WantClass           nativerouting.ResolutionClass
	WantNativeProfileID string
}

// NativeRoutingConformanceConfig wires one adapter to the shared role-routing
// and launch-fidelity suite. LaunchCapture must exercise the adapter's real
// typed builder/process-argument decoder with a hermetic process boundary.
type NativeRoutingConformanceConfig struct {
	Harness       entity.Harness
	Provider      port.CatalogProvider
	Capability    nativerouting.NativeCapability
	Builder       port.LaunchBuilder
	Clock         nativerouting.TrustedClock
	Authorization nativerouting.AuthorizationPolicy
	LaunchCapture func(port.NativeLaunchSpec) (port.LaunchCapture, error)
	Cases         []NativeRoutingConformanceCase
}

// CheckNativeRoutingConformance validates independent role catalogs, resolves
// every declared semantic cell, and proves selected-versus-observed native
// launch equality. It performs no model call or network access.
func CheckNativeRoutingConformance(c NativeRoutingConformanceConfig) error {
	if c.Provider == nil || c.Capability == nil || c.Builder == nil || c.Clock == nil || c.Authorization == nil || c.LaunchCapture == nil {
		return errors.New("runtime/native-conformance: incomplete adapter wiring")
	}
	if c.Harness == "" || len(c.Cases) == 0 {
		return errors.New("runtime/native-conformance: harness and complete case domain are required")
	}

	catalogs := map[nativerouting.Role]nativerouting.ValidatedCatalog{}
	for _, role := range []nativerouting.Role{nativerouting.RoleImplementation, nativerouting.RoleReview} {
		catalog, qualifications, err := c.Provider.Catalog(role)
		if err != nil {
			return fmt.Errorf("runtime/native-conformance: publish %s catalog: %w", role, err)
		}
		if catalog.Harness != c.Harness || catalog.Role != role {
			return fmt.Errorf("runtime/native-conformance: %s catalog identifies %s/%s", role, catalog.Harness, catalog.Role)
		}
		validated, diagnostics := nativerouting.ValidateCatalog(catalog, qualifications, c.Capability, c.Clock, c.Authorization)
		if len(diagnostics) != 0 {
			return fmt.Errorf("runtime/native-conformance: validate %s catalog: %v", role, diagnostics)
		}
		catalogs[role] = validated
	}
	if err := requireIndependentCatalogs(catalogs[nativerouting.RoleImplementation], catalogs[nativerouting.RoleReview]); err != nil {
		return err
	}

	seenRoles := map[nativerouting.Role]bool{}
	for i, test := range c.Cases {
		catalog, ok := catalogs[test.Role]
		if !ok {
			return fmt.Errorf("runtime/native-conformance: case %d %q has unknown role %q", i, test.Name, test.Role)
		}
		seenRoles[test.Role] = true
		resolution := (nativerouting.Resolver{}).Resolve(nativerouting.Request{
			SchemaVersion: nativerouting.RequestSchema, AttemptID: fmt.Sprintf("native-conformance-%d", i),
			DemandID: executiondemand.DemandID(fmt.Sprintf("native-conformance-demand-%d", i)),
			Harness:  c.Harness, Role: test.Role, RequestedProfile: test.Requested, Mode: nativerouting.ModeNormal,
		}, catalog)
		if resolution.Class != test.WantClass {
			return fmt.Errorf("runtime/native-conformance: case %q class=%s, want %s", test.Name, resolution.Class, test.WantClass)
		}
		if resolution.Class == nativerouting.ResolutionUnsupported {
			if resolution.NativeProfile != nil || test.WantNativeProfileID != "" {
				return fmt.Errorf("runtime/native-conformance: unsupported case %q exposed or expected a native profile", test.Name)
			}
			continue
		}
		if resolution.NativeProfile == nil || resolution.NativeProfile.NativeProfileID != test.WantNativeProfileID {
			return fmt.Errorf("runtime/native-conformance: case %q selected %#v, want %q", test.Name, resolution.NativeProfile, test.WantNativeProfileID)
		}
		nativeSpec, err := c.Builder.Build(*resolution.NativeProfile)
		if err != nil {
			return fmt.Errorf("runtime/native-conformance: build case %q: %w", test.Name, err)
		}
		capture, err := c.LaunchCapture(nativeSpec)
		if err != nil {
			return fmt.Errorf("runtime/native-conformance: capture case %q: %w", test.Name, err)
		}
		if !capture.MatchesSelected(*resolution.NativeProfile) {
			return fmt.Errorf("runtime/native-conformance: case %q selected model=%q effort=%q controls=%v but captured model=%q effort=%q controls=%v",
				test.Name, resolution.NativeProfile.NativeModel, resolution.NativeProfile.NativeEffort, resolution.NativeProfile.Controls,
				capture.NativeModel, capture.NativeEffort, capture.Controls)
		}
	}
	if !seenRoles[nativerouting.RoleImplementation] || !seenRoles[nativerouting.RoleReview] {
		return errors.New("runtime/native-conformance: case domain omitted implementation or review")
	}
	return nil
}

func requireIndependentCatalogs(implementation, review nativerouting.ValidatedCatalog) error {
	if implementation.Identity == review.Identity {
		return errors.New("runtime/native-conformance: implementation and review policy identities are not independent")
	}
	return nil
}
