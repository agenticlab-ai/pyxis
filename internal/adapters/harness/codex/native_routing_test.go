package codex

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/executiondemand"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/nativerouting"
)

type codexRoutingClock struct{ at time.Time }

func (c codexRoutingClock) Now() time.Time { return c.at }

type codexRoutingAuthorization struct{}

func (codexRoutingAuthorization) Authorize(proof nativerouting.AuthorizationProof) error {
	if proof.Repository != "agenticlab-ai/pyxis" || proof.OperatorID != "pyxis-maintainers" {
		return errors.New("not authorized Codex routing evidence")
	}
	return nil
}

func validateCodexCatalog(t *testing.T, adapter *Adapter, role nativerouting.Role) (nativerouting.ValidatedCatalog, nativerouting.QualificationSet) {
	t.Helper()
	catalog, qualifications, err := adapter.Catalog(role)
	if err != nil {
		t.Fatalf("Catalog(%s): %v", role, err)
	}
	validated, diagnostics := nativerouting.ValidateCatalog(
		catalog,
		qualifications,
		adapter,
		codexRoutingClock{at: time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)},
		codexRoutingAuthorization{},
	)
	if len(diagnostics) != 0 {
		t.Fatalf("Catalog(%s) did not validate: %v", role, diagnostics)
	}
	return validated, qualifications
}

func allSemanticProfiles() []executiondemand.ProfileMinimum {
	capabilities := []executiondemand.Capability{
		executiondemand.CapabilityEconomy, executiondemand.CapabilityStandard,
		executiondemand.CapabilityStrong, executiondemand.CapabilityFrontier,
	}
	deliberations := []executiondemand.Deliberation{
		executiondemand.DeliberationLight, executiondemand.DeliberationStandard,
		executiondemand.DeliberationDeep, executiondemand.DeliberationMaximum,
	}
	contexts := []executiondemand.Context{
		executiondemand.ContextStandard, executiondemand.ContextLarge, executiondemand.ContextExtended,
	}
	assurances := []executiondemand.Assurance{
		executiondemand.AssuranceOrdinary, executiondemand.AssuranceElevated, executiondemand.AssuranceCritical,
	}
	profiles := make([]executiondemand.ProfileMinimum, 0, len(capabilities)*len(deliberations)*len(contexts)*len(assurances))
	for _, capability := range capabilities {
		for _, deliberation := range deliberations {
			for _, context := range contexts {
				for _, assurance := range assurances {
					profiles = append(profiles, executiondemand.ProfileMinimum{
						Capability: capability, Deliberation: deliberation, Context: context, Assurance: assurance,
					})
				}
			}
		}
	}
	return profiles
}

func resolveCodex(t *testing.T, catalog nativerouting.ValidatedCatalog, role nativerouting.Role, profile executiondemand.ProfileMinimum) nativerouting.SelectedProfile {
	t.Helper()
	resolution := (nativerouting.Resolver{}).Resolve(nativerouting.Request{
		SchemaVersion:    nativerouting.RequestSchema,
		AttemptID:        "codex-conformance-attempt",
		DemandID:         executiondemand.DemandID("codex-conformance-demand"),
		Harness:          harnessID,
		Role:             role,
		RequestedProfile: profile,
		Mode:             nativerouting.ModeNormal,
	}, catalog)
	if resolution.Class != nativerouting.ResolutionExact && resolution.Class != nativerouting.ResolutionPromoted {
		t.Fatalf("%s profile %#v resolved as %#v", role, profile, resolution)
	}
	if resolution.NativeProfile == nil {
		t.Fatalf("%s profile %#v returned no selected profile", role, profile)
	}
	return *resolution.NativeProfile
}

func TestCodexRoleCatalogsResolveAndLaunchCompleteSemanticDomain(t *testing.T) {
	adapter := mustAdapter(t)
	selectedByRole := map[nativerouting.Role]nativerouting.SelectedProfile{}
	for _, role := range []nativerouting.Role{nativerouting.RoleImplementation, nativerouting.RoleReview} {
		t.Run(string(role), func(t *testing.T) {
			catalog, _ := validateCodexCatalog(t, adapter, role)
			if catalog.Catalog.Role != role {
				t.Fatalf("catalog role = %s, want %s", catalog.Catalog.Role, role)
			}
			for _, profile := range allSemanticProfiles() {
				selected := resolveCodex(t, catalog, role, profile)
				built, err := adapter.Build(selected)
				if err != nil {
					t.Fatalf("Build(%s): %v", selected.NativeProfileID, err)
				}
				if built.NativeProfileID != selected.NativeProfileID || built.NativeModel != selected.NativeModel || built.NativeEffort != selected.NativeEffort || len(built.Controls) != 0 {
					t.Fatalf("builder changed selected profile: selected=%#v built=%#v", selected, built)
				}
				args := adapter.launchArgs(port.LaunchSpec{Model: built.NativeModel, Effort: built.NativeEffort})
				capture, err := captureNativeLaunch(args)
				if err != nil {
					t.Fatalf("capture actual launch for %s: %v", selected.NativeProfileID, err)
				}
				if !capture.MatchesSelected(selected) {
					t.Fatalf("captured launch differs: selected=%#v capture=%#v", selected, capture)
				}
			}
			selectedByRole[role] = resolveCodex(t, catalog, role, executiondemand.ProfileMinimum{
				Capability:   executiondemand.CapabilityEconomy,
				Deliberation: executiondemand.DeliberationLight,
				Context:      executiondemand.ContextStandard,
				Assurance:    executiondemand.AssuranceOrdinary,
			})
		})
	}
	implementation := selectedByRole[nativerouting.RoleImplementation]
	review := selectedByRole[nativerouting.RoleReview]
	if implementation.Role == review.Role || implementation.NativeProfileID == review.NativeProfileID {
		t.Fatalf("review implicitly reused implementation selection: implementation=%#v review=%#v", implementation, review)
	}
}

func TestCodexLaunchCaptureRejectsEverySelectedDifference(t *testing.T) {
	adapter := mustAdapter(t)
	catalog, _ := validateCodexCatalog(t, adapter, nativerouting.RoleImplementation)
	selected := resolveCodex(t, catalog, nativerouting.RoleImplementation, executiondemand.ProfileMinimum{
		Capability:   executiondemand.CapabilityStrong,
		Deliberation: executiondemand.DeliberationDeep,
		Context:      executiondemand.ContextExtended,
		Assurance:    executiondemand.AssuranceElevated,
	})
	built, err := adapter.Build(selected)
	if err != nil {
		t.Fatal(err)
	}
	capture, err := captureNativeLaunch(adapter.launchArgs(port.LaunchSpec{Model: built.NativeModel, Effort: built.NativeEffort}))
	if err != nil || !capture.MatchesSelected(selected) {
		t.Fatalf("baseline launch did not match: capture=%#v err=%v", capture, err)
	}
	falseValue := false
	for name, mutate := range map[string]func(*port.LaunchCapture){
		"model":  func(c *port.LaunchCapture) { c.NativeModel += "-different" },
		"effort": func(c *port.LaunchCapture) { c.NativeEffort = "low" },
		"surplus control": func(c *port.LaunchCapture) {
			c.Controls = []nativerouting.NativeControl{{Name: "unselected", Type: nativerouting.ControlBoolean, BooleanValue: &falseValue}}
		},
	} {
		t.Run(name, func(t *testing.T) {
			changed := capture
			mutate(&changed)
			if changed.MatchesSelected(selected) {
				t.Fatal("seeded actual-launch difference was accepted")
			}
		})
	}

	args := adapter.launchArgs(port.LaunchSpec{Model: built.NativeModel, Effort: built.NativeEffort})
	args = append(args, "--raw-injected-setting")
	if _, err := captureNativeLaunch(args); err == nil {
		t.Fatal("actual-launch decoder accepted an unknown raw argument")
	}
}

func TestCodexLaunchMismatchInvalidatesAffectedQualification(t *testing.T) {
	adapter := mustAdapter(t)
	catalog, qualifications, err := adapter.Catalog(nativerouting.RoleReview)
	if err != nil {
		t.Fatal(err)
	}
	selectedCatalog, _ := validateCodexCatalog(t, adapter, nativerouting.RoleReview)
	selected := resolveCodex(t, selectedCatalog, nativerouting.RoleReview, executiondemand.ProfileMinimum{
		Capability:   executiondemand.CapabilityEconomy,
		Deliberation: executiondemand.DeliberationLight,
		Context:      executiondemand.ContextStandard,
		Assurance:    executiondemand.AssuranceOrdinary,
	})
	built, err := adapter.Build(selected)
	if err != nil {
		t.Fatal(err)
	}
	mismatchedArgs := adapter.launchArgs(port.LaunchSpec{Model: built.NativeModel, Effort: built.NativeEffort})
	mismatchedArgs[5] = `model_reasoning_effort="high"`
	_, invalidation, fidelityErr := verifyNativeLaunch(selected, mismatchedArgs, time.Date(2026, 8, 5, 12, 1, 0, 0, time.UTC))
	if fidelityErr == nil || invalidation == nil {
		t.Fatalf("launch mismatch did not fail and produce an invalidation: err=%v invalidation=%#v", fidelityErr, invalidation)
	}
	for i := range qualifications {
		if qualifications[i].QualificationID != selected.QualificationID {
			continue
		}
		qualifications[i].Validity.Invalidations = append(qualifications[i].Validity.Invalidations, *invalidation)
		for j := range catalog.Entries {
			if catalog.Entries[j].QualificationID == selected.QualificationID {
				catalog.Entries[j].QualificationDigest = nativerouting.QualificationDigest(qualifications[i])
			}
		}
	}
	validated, diagnostics := nativerouting.ValidateCatalog(catalog, qualifications, adapter, codexRoutingClock{at: time.Date(2026, 8, 5, 12, 2, 0, 0, time.UTC)}, codexRoutingAuthorization{})
	if len(validated.Entries()) != 0 {
		t.Fatal("launch-invalidated qualification remained selectable")
	}
	found := false
	for _, diagnostic := range diagnostics {
		found = found || (diagnostic.Defect == "qualification_invalidated" && diagnostic.QualificationID == selected.QualificationID)
	}
	if !found {
		t.Fatalf("affected qualification invalidation was not diagnosed: %v", diagnostics)
	}
}

func TestCodexNativeCapabilityAndBuilderFailClosed(t *testing.T) {
	adapter := mustAdapter(t)
	trueValue := true
	for name, entry := range map[string]nativerouting.CatalogEntry{
		"model":   {NativeModel: "provider-latest", NativeEffort: "high", Controls: []nativerouting.NativeControl{}},
		"effort":  {NativeModel: "gpt-5.6-sol", NativeEffort: "ultra", Controls: []nativerouting.NativeControl{}},
		"control": {NativeModel: "gpt-5.6-sol", NativeEffort: "high", Controls: []nativerouting.NativeControl{{Name: "raw_arguments", Type: nativerouting.ControlBoolean, BooleanValue: &trueValue}}},
	} {
		t.Run(name, func(t *testing.T) {
			err := adapter.ValidateProfile(entry)
			var defect nativerouting.NativeProfileDefect
			if !errors.As(err, &defect) || !strings.HasPrefix(defect.Defect, "unknown_native_") {
				t.Fatalf("unknown native setting did not return a typed defect: %v", err)
			}
		})
	}

	validCatalog, _ := validateCodexCatalog(t, adapter, nativerouting.RoleImplementation)
	selected := resolveCodex(t, validCatalog, nativerouting.RoleImplementation, allSemanticProfiles()[0])
	for name, mutate := range map[string]func(*nativerouting.SelectedProfile){
		"wrong harness":         func(s *nativerouting.SelectedProfile) { s.Harness = "claude" },
		"wrong generation":      func(s *nativerouting.SelectedProfile) { s.AdapterGeneration = "codex-native-routing/v0" },
		"unknown model":         func(s *nativerouting.SelectedProfile) { s.NativeModel = "gpt-latest" },
		"missing qualification": func(s *nativerouting.SelectedProfile) { s.QualificationID = "" },
	} {
		t.Run(name, func(t *testing.T) {
			changed := selected
			mutate(&changed)
			if _, err := adapter.Build(changed); err == nil {
				t.Fatal("builder accepted a non-qualified selection")
			}
		})
	}
}

func TestCodexCatalogReturnsFreshRoleOwnedData(t *testing.T) {
	adapter := mustAdapter(t)
	implementation, implementationQualifications, err := adapter.Catalog(nativerouting.RoleImplementation)
	if err != nil {
		t.Fatal(err)
	}
	implementation.Entries[0].NativeModel = "mutated"
	implementationQualifications[0].Claims[0].Result = "mutated"
	fresh, freshQualifications, err := adapter.Catalog(nativerouting.RoleImplementation)
	if err != nil {
		t.Fatal(err)
	}
	if fresh.Entries[0].NativeModel == "mutated" || freshQualifications[0].Claims[0].Result == "mutated" {
		t.Fatal("catalog provider leaked mutable policy data across calls")
	}
	if _, _, err := adapter.Catalog("implement-or-review"); err == nil {
		t.Fatal("catalog provider accepted an unknown role")
	}
	review, _, err := adapter.Catalog(nativerouting.RoleReview)
	if err != nil {
		t.Fatal(err)
	}
	if implementation.CatalogID == review.CatalogID || implementation.Role == review.Role {
		t.Fatal("implementation and review did not receive separate catalog identities")
	}
}
