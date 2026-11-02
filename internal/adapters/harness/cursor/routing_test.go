package cursor

import (
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/executiondemand"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/nativerouting"
)

type routingClock struct{ at time.Time }

func (c routingClock) Now() time.Time { return c.at }

type routingAuthorization struct{}

func (routingAuthorization) Authorize(proof nativerouting.AuthorizationProof) error {
	if proof.Repository != "agenticlab-ai/pyxis" || proof.OperatorID != "native-routing-operator" {
		return errors.New("not authorized by the repository operator")
	}
	return nil
}

func validateCursorCatalog(t *testing.T, a *Adapter, role nativerouting.Role) (nativerouting.ValidatedCatalog, nativerouting.QualificationSet) {
	t.Helper()
	catalog, qualifications, err := a.Catalog(role)
	if err != nil {
		t.Fatalf("Catalog(%s): %v", role, err)
	}
	validated, diagnostics := nativerouting.ValidateCatalog(
		catalog, qualifications, a,
		routingClock{time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)},
		routingAuthorization{},
	)
	if len(diagnostics) != 0 {
		t.Fatalf("Catalog(%s) did not validate: %v", role, diagnostics)
	}
	return validated, qualifications
}

func TestRoleCatalogsAreSeparateCompleteAndQualified(t *testing.T) {
	a := mustAdapter(t)
	implementation, _ := validateCursorCatalog(t, a, nativerouting.RoleImplementation)
	review, _ := validateCursorCatalog(t, a, nativerouting.RoleReview)
	if implementation.Catalog.CatalogID == review.Catalog.CatalogID || implementation.Identity == review.Identity {
		t.Fatal("implementation and review must have independent catalog and policy identities")
	}
	for _, left := range implementation.Entries() {
		for _, right := range review.Entries() {
			if left.NativeProfileID == right.NativeProfileID || left.QualificationID == right.QualificationID {
				t.Fatalf("roles reused profile or qualification identity: %q / %q", left.NativeProfileID, left.QualificationID)
			}
		}
	}
	if _, _, err := a.Catalog("implementation-review"); err == nil {
		t.Fatal("an unknown role must not inherit either role catalog")
	}
}

func TestEverySemanticDomainCellHasAnExplicitCursorResult(t *testing.T) {
	a := mustAdapter(t)
	capabilities := []executiondemand.Capability{executiondemand.CapabilityEconomy, executiondemand.CapabilityStandard, executiondemand.CapabilityStrong, executiondemand.CapabilityFrontier}
	deliberations := []executiondemand.Deliberation{executiondemand.DeliberationLight, executiondemand.DeliberationStandard, executiondemand.DeliberationDeep, executiondemand.DeliberationMaximum}
	contexts := []executiondemand.Context{executiondemand.ContextStandard, executiondemand.ContextLarge, executiondemand.ContextExtended}
	assurances := []executiondemand.Assurance{executiondemand.AssuranceOrdinary, executiondemand.AssuranceElevated, executiondemand.AssuranceCritical}

	for _, role := range []nativerouting.Role{nativerouting.RoleImplementation, nativerouting.RoleReview} {
		catalog, _ := validateCursorCatalog(t, a, role)
		cell := 0
		for _, capability := range capabilities {
			for _, deliberation := range deliberations {
				for _, contextLevel := range contexts {
					for _, assurance := range assurances {
						cell++
						result := (nativerouting.Resolver{}).Resolve(nativerouting.Request{
							SchemaVersion: nativerouting.RequestSchema, AttemptID: fmt.Sprintf("%s-%03d", role, cell),
							DemandID: executiondemand.DemandID(fmt.Sprintf("demand-%03d", cell)), Harness: harnessID, Role: role,
							RequestedProfile: executiondemand.ProfileMinimum{Capability: capability, Deliberation: deliberation, Context: contextLevel, Assurance: assurance},
							Mode:             nativerouting.ModeNormal,
						}, catalog)
						if result.Class == nativerouting.ResolutionPolicyInvalid || (result.Class == nativerouting.ResolutionUnsupported) != (result.NativeProfile == nil) {
							t.Fatalf("%s cell %d did not return a closed resolved/unsupported result: %#v", role, cell, result)
						}
					}
				}
			}
		}
		if cell != 144 {
			t.Fatalf("enumerated %d semantic cells, want 144", cell)
		}
	}
}

func TestSelectedProfilesBuildAndCaptureExactlyWithoutEffortFlag(t *testing.T) {
	for _, role := range []nativerouting.Role{nativerouting.RoleImplementation, nativerouting.RoleReview} {
		t.Run(string(role), func(t *testing.T) {
			fc := &fakeCommander{procStdout: []byte(resultJSON)}
			a := mustAdapter(t, withCommander(fc))
			catalog, _ := validateCursorCatalog(t, a, role)
			for i, entry := range catalog.Entries() {
				result := (nativerouting.Resolver{}).Resolve(nativerouting.Request{
					SchemaVersion: nativerouting.RequestSchema, AttemptID: fmt.Sprintf("attempt-%d", i),
					DemandID: executiondemand.DemandID(fmt.Sprintf("demand-%d", i)), Harness: harnessID, Role: role,
					RequestedProfile: executiondemand.ProfileMinimum{
						Capability: entry.Provides.Capability, Deliberation: entry.Provides.Deliberation,
						Context: entry.Provides.Context, Assurance: entry.Provides.Assurance,
					}, Mode: nativerouting.ModeNormal,
				}, catalog)
				if result.NativeProfile == nil || result.NativeProfile.NativeProfileID != entry.NativeProfileID {
					t.Fatalf("entry %q did not resolve to itself: %#v", entry.NativeProfileID, result)
				}
				native, err := a.Build(*result.NativeProfile)
				if err != nil {
					t.Fatalf("Build(%s): %v", entry.NativeProfileID, err)
				}
				purpose := port.LaunchPurposeImplementation
				if role == nativerouting.RoleReview {
					purpose = port.LaunchPurposeReview
				}
				h, err := a.Launch(t.Context(), port.LaunchSpec{Purpose: purpose, Brief: "decode-pinned fixture", Workdir: t.TempDir(), Model: native.NativeModel, Effort: native.NativeEffort})
				if err != nil {
					t.Fatalf("Launch(%s): %v", entry.NativeProfileID, err)
				}
				capture, err := a.LaunchCapture(h)
				if err != nil {
					t.Fatalf("LaunchCapture(%s): %v", entry.NativeProfileID, err)
				}
				if !capture.MatchesSelected(*result.NativeProfile) {
					t.Fatalf("capture %#v differs from selection %#v", capture, result.NativeProfile)
				}
				if slices.Contains(fc.starts[len(fc.starts)-1].Args, "--effort") {
					t.Fatal("Cursor launch fabricated an unsupported --effort flag")
				}
			}
		})
	}
}

func TestBuildAndLaunchRejectSelectionDrift(t *testing.T) {
	a := mustAdapter(t, withCommander(&fakeCommander{procStdout: []byte(resultJSON)}))
	catalog, _ := validateCursorCatalog(t, a, nativerouting.RoleImplementation)
	entry := catalog.Entries()[0]
	selected := nativerouting.SelectedProfile{
		CatalogID: catalog.Catalog.CatalogID, PolicyIdentity: catalog.Identity, Harness: harnessID, Role: catalog.Catalog.Role,
		NativeProfileID: entry.NativeProfileID, NativeModel: entry.NativeModel, NativeEffort: entry.NativeEffort,
		Controls: entry.Controls, Provides: entry.Provides, CostRank: entry.CostRank,
		QualificationID: entry.QualificationID, QualificationDigest: entry.QualificationDigest, AdapterGeneration: entry.AdapterGeneration,
	}
	for name, mutate := range map[string]func(*nativerouting.SelectedProfile){
		"policy":  func(s *nativerouting.SelectedProfile) { s.PolicyIdentity = "stale-policy" },
		"role":    func(s *nativerouting.SelectedProfile) { s.Role = nativerouting.RoleReview },
		"model":   func(s *nativerouting.SelectedProfile) { s.NativeModel = "cursor-grok-4.5-high" },
		"effort":  func(s *nativerouting.SelectedProfile) { s.NativeEffort = "high" },
		"control": func(s *nativerouting.SelectedProfile) { s.Controls = s.Controls[1:] },
	} {
		t.Run(name, func(t *testing.T) {
			changed := selected
			mutate(&changed)
			if _, err := a.Build(changed); err == nil {
				t.Fatal("Build accepted a selection that differs from qualified catalog data")
			}
		})
	}
	if _, err := a.Launch(t.Context(), port.LaunchSpec{Brief: "fixture", Workdir: t.TempDir(), Model: "cursor-grok-4.5-high", Effort: "medium"}); err == nil {
		t.Fatal("Launch accepted effort that differs from the selected model parameter")
	}
}

func TestLaunchDifferencesFailAndInvalidateQualification(t *testing.T) {
	a := mustAdapter(t)
	validated, qualifications := validateCursorCatalog(t, a, nativerouting.RoleImplementation)
	entry := validated.Entries()[0]
	selected := nativerouting.SelectedProfile{
		CatalogID: validated.Catalog.CatalogID, PolicyIdentity: validated.Identity, Harness: harnessID, Role: validated.Catalog.Role,
		NativeProfileID: entry.NativeProfileID, NativeModel: entry.NativeModel, NativeEffort: entry.NativeEffort,
		Controls: entry.Controls, Provides: entry.Provides, CostRank: entry.CostRank,
		QualificationID: entry.QualificationID, QualificationDigest: entry.QualificationDigest, AdapterGeneration: entry.AdapterGeneration,
	}
	base := port.LaunchCapture{NativeModel: selected.NativeModel, NativeEffort: selected.NativeEffort, Controls: selected.Controls}
	for name, mutate := range map[string]func(*port.LaunchCapture){
		"model_parameter": func(c *port.LaunchCapture) { c.NativeModel = "cursor-grok-4.5-high" },
		"decoded_effort":  func(c *port.LaunchCapture) { c.NativeEffort = "high" },
		"typed_control":   func(c *port.LaunchCapture) { c.Controls = c.Controls[1:] },
	} {
		t.Run(name, func(t *testing.T) {
			capture := base
			mutate(&capture)
			if capture.MatchesSelected(selected) {
				t.Fatal("seeded launch difference matched the selection")
			}
			invalidated := append(nativerouting.QualificationSet(nil), qualifications...)
			for i := range invalidated {
				if invalidated[i].QualificationID == selected.QualificationID {
					invalidated[i].Validity.Invalidations = []nativerouting.Invalidation{{EventID: "launch-mismatch-" + name, At: time.Date(2026, 8, 5, 13, 0, 0, 0, time.UTC), Reason: name}}
				}
			}
			got, diagnostics := nativerouting.ValidateCatalog(validated.Catalog, invalidated, a, routingClock{time.Date(2026, 8, 5, 14, 0, 0, 0, time.UTC)}, routingAuthorization{})
			if len(got.Entries()) != 0 || !hasRoutingDefect(diagnostics, "qualification_invalidated", selected.QualificationID) {
				t.Fatalf("mismatch did not invalidate only the affected qualification: got=%#v diagnostics=%v", got, diagnostics)
			}
		})
	}
}

func hasRoutingDefect(diagnostics []nativerouting.Diagnostic, defect, qualificationID string) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Defect == defect && diagnostic.QualificationID == qualificationID {
			return true
		}
	}
	return false
}
