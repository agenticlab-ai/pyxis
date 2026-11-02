package opencode

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/executiondemand"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/nativerouting"
)

var routingNow = time.Date(2026, time.August, 5, 12, 0, 0, 0, time.UTC)

type routingClock struct{}

func (routingClock) Now() time.Time { return routingNow }

type routingAuthorization struct{}

func (routingAuthorization) Authorize(proof nativerouting.AuthorizationProof) error {
	if proof.Kind != nativerouting.AuthorizationRepositoryOperator || proof.Repository != "agenticlab-ai/pyxis" || proof.OperatorID != "pyxis-routing-operator" {
		return errors.New("qualification lacks OpenCode routing-operator authority")
	}
	return nil
}

func validatedRoleCatalog(t *testing.T, adapter *Adapter, role nativerouting.Role) (nativerouting.ValidatedCatalog, nativerouting.Catalog, nativerouting.QualificationSet) {
	t.Helper()
	catalog, qualifications, err := adapter.Catalog(role)
	if err != nil {
		t.Fatalf("Catalog(%s): %v", role, err)
	}
	validated, diagnostics := nativerouting.ValidateCatalog(catalog, qualifications, adapter, routingClock{}, routingAuthorization{})
	if len(diagnostics) != 0 {
		t.Fatalf("Catalog(%s) rejected: %v", role, diagnostics)
	}
	return validated, catalog, qualifications
}

func resolveProfile(t *testing.T, adapter *Adapter, role nativerouting.Role, requested executiondemand.ProfileMinimum) nativerouting.SelectedProfile {
	t.Helper()
	validated, _, _ := validatedRoleCatalog(t, adapter, role)
	resolution := (nativerouting.Resolver{}).Resolve(nativerouting.Request{
		SchemaVersion: nativerouting.RequestSchema, AttemptID: "attempt-opencode-conformance",
		DemandID: executiondemand.DemandID("demand-opencode-conformance"), Harness: harnessID,
		Role: role, RequestedProfile: requested, Mode: nativerouting.ModeNormal,
	}, validated)
	if resolution.Class != nativerouting.ResolutionExact && resolution.Class != nativerouting.ResolutionPromoted {
		t.Fatalf("Resolve(%s, %+v) = %s: %+v", role, requested, resolution.Class, resolution)
	}
	if resolution.NativeProfile == nil {
		t.Fatal("resolved result omitted its selected profile")
	}
	return *resolution.NativeProfile
}

func TestRoleCatalogsAreCurrentCompleteAndIndependent(t *testing.T) {
	adapter := mustAdapter(t)
	implementation, implementationCatalog, _ := validatedRoleCatalog(t, adapter, nativerouting.RoleImplementation)
	review, reviewCatalog, _ := validatedRoleCatalog(t, adapter, nativerouting.RoleReview)
	if implementation.Identity == review.Identity || implementationCatalog.CatalogID == reviewCatalog.CatalogID {
		t.Fatal("implementation and review reused one catalog identity")
	}
	for _, entry := range implementationCatalog.Entries {
		if strings.HasPrefix(entry.NativeProfileID, "review-") {
			t.Fatalf("implementation catalog contains review profile %q", entry.NativeProfileID)
		}
	}
	for _, entry := range reviewCatalog.Entries {
		if strings.HasPrefix(entry.NativeProfileID, "implementation-") {
			t.Fatalf("review catalog contains implementation profile %q", entry.NativeProfileID)
		}
	}

	minimum := executiondemand.ProfileMinimum{
		Capability: executiondemand.CapabilityEconomy, Deliberation: executiondemand.DeliberationLight,
		Context: executiondemand.ContextStandard, Assurance: executiondemand.AssuranceOrdinary,
	}
	implementationSelection := resolveProfile(t, adapter, nativerouting.RoleImplementation, minimum)
	reviewSelection := resolveProfile(t, adapter, nativerouting.RoleReview, minimum)
	if implementationSelection.NativeProfileID != "implementation-economy" || reviewSelection.NativeProfileID != "review-standard" {
		t.Fatalf("role selection reused implicitly: implementation=%q review=%q", implementationSelection.NativeProfileID, reviewSelection.NativeProfileID)
	}
}

func TestEverySemanticDomainCellResolvesThroughEachRole(t *testing.T) {
	adapter := mustAdapter(t)
	capabilities := []executiondemand.Capability{executiondemand.CapabilityEconomy, executiondemand.CapabilityStandard, executiondemand.CapabilityStrong, executiondemand.CapabilityFrontier}
	deliberations := []executiondemand.Deliberation{executiondemand.DeliberationLight, executiondemand.DeliberationStandard, executiondemand.DeliberationDeep, executiondemand.DeliberationMaximum}
	contexts := []executiondemand.Context{executiondemand.ContextStandard, executiondemand.ContextLarge, executiondemand.ContextExtended}
	assurances := []executiondemand.Assurance{executiondemand.AssuranceOrdinary, executiondemand.AssuranceElevated, executiondemand.AssuranceCritical}

	for _, role := range []nativerouting.Role{nativerouting.RoleImplementation, nativerouting.RoleReview} {
		t.Run(string(role), func(t *testing.T) {
			validated, catalog, _ := validatedRoleCatalog(t, adapter, role)
			selected := map[string]bool{}
			cells := 0
			for _, capability := range capabilities {
				for _, deliberation := range deliberations {
					for _, contextLevel := range contexts {
						for _, assurance := range assurances {
							cells++
							resolution := (nativerouting.Resolver{}).Resolve(nativerouting.Request{
								SchemaVersion: nativerouting.RequestSchema, AttemptID: "attempt-domain",
								DemandID: executiondemand.DemandID("demand-domain"), Harness: harnessID, Role: role,
								RequestedProfile: executiondemand.ProfileMinimum{Capability: capability, Deliberation: deliberation, Context: contextLevel, Assurance: assurance},
								Mode:             nativerouting.ModeNormal,
							}, validated)
							if resolution.NativeProfile == nil || (resolution.Class != nativerouting.ResolutionExact && resolution.Class != nativerouting.ResolutionPromoted) {
								t.Fatalf("domain cell %+v returned %s", resolution.RequestedProfile, resolution.Class)
							}
							if resolution.NativeProfile.Role != role {
								t.Fatalf("domain cell crossed roles: got %s, want %s", resolution.NativeProfile.Role, role)
							}
							selected[resolution.NativeProfile.NativeProfileID] = true
						}
					}
				}
			}
			if cells != 144 {
				t.Fatalf("enumerated %d domain cells, want 144", cells)
			}
			for _, entry := range catalog.Entries {
				if !selected[entry.NativeProfileID] {
					t.Fatalf("catalog profile %q is unreachable from the semantic domain", entry.NativeProfileID)
				}
			}
		})
	}
}

func TestFixtureEvidenceBindsEveryQualification(t *testing.T) {
	data, err := os.ReadFile("testdata/role-routing-v1.json")
	if err != nil {
		t.Fatalf("read role-routing fixture: %v", err)
	}
	sum := sha256.Sum256(data)
	if got := hex.EncodeToString(sum[:]); got != roleFixtureDigest {
		t.Fatalf("fixture digest = %s, want %s", got, roleFixtureDigest)
	}
	resultData, err := os.ReadFile("testdata/role-routing-results-v1.json")
	if err != nil {
		t.Fatalf("read role-routing results: %v", err)
	}
	resultSum := sha256.Sum256(resultData)
	if got := hex.EncodeToString(resultSum[:]); got != roleResultDigest {
		t.Fatalf("result digest = %s, want %s", got, roleResultDigest)
	}
	var fixture struct {
		SchemaVersion     string `json:"schema_version"`
		AdapterGeneration string `json:"adapter_generation"`
		DecodePins        struct {
			NativeEffort string `json:"native_effort"`
			OutputFormat string `json:"output_format"`
			Session      string `json:"session"`
		} `json:"decode_pins"`
		Cases []struct {
			Role            nativerouting.Role `json:"role"`
			NativeProfileID string             `json:"native_profile_id"`
			ProviderModel   string             `json:"provider_model"`
			Provides        []string           `json:"provides"`
		} `json:"cases"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&fixture); err != nil {
		t.Fatalf("decode closed role-routing fixture: %v", err)
	}
	if fixture.SchemaVersion != "opencode-role-routing-conformance/v1" || fixture.AdapterGeneration != adapterGeneration || fixture.DecodePins.NativeEffort != nativeEffortNone || fixture.DecodePins.OutputFormat != "json" || fixture.DecodePins.Session != "fresh" {
		t.Fatalf("fixture decode pins drifted: %+v", fixture)
	}
	var results struct {
		SchemaVersion     string `json:"schema_version"`
		FixtureDigest     string `json:"fixture_digest"`
		AdapterGeneration string `json:"adapter_generation"`
		Results           []struct {
			Role                nativerouting.Role `json:"role"`
			NativeProfileID     string             `json:"native_profile_id"`
			ComponentResults    []string           `json:"component_results"`
			LaunchCaptureDigest string             `json:"launch_capture_digest"`
		} `json:"results"`
	}
	resultDecoder := json.NewDecoder(bytes.NewReader(resultData))
	resultDecoder.DisallowUnknownFields()
	if err := resultDecoder.Decode(&results); err != nil {
		t.Fatalf("decode closed role-routing results: %v", err)
	}
	if results.SchemaVersion != "opencode-role-routing-results/v1" || results.FixtureDigest != roleFixtureDigest || results.AdapterGeneration != adapterGeneration {
		t.Fatalf("result evidence binding drifted: %+v", results)
	}

	adapter := mustAdapter(t)
	wantCases := map[string]bool{}
	wantResults := map[string]string{}
	for _, role := range []nativerouting.Role{nativerouting.RoleImplementation, nativerouting.RoleReview} {
		_, catalog, qualifications := validatedRoleCatalog(t, adapter, role)
		for _, entry := range catalog.Entries {
			provides := []string{string(entry.Provides.Capability), string(entry.Provides.Deliberation), string(entry.Provides.Context), string(entry.Provides.Assurance)}
			wantCases[string(role)+"/"+entry.NativeProfileID+"/"+entry.NativeModel+"/"+strings.Join(provides, ",")] = true
		}
		for _, qualification := range qualifications {
			wantResults[string(role)+"/"+qualification.NativeProfileID] = qualification.LaunchCaptureDigest
			for _, claim := range qualification.Claims {
				if claim.Method != nativerouting.MethodDecodePinnedEval || claim.FixtureDigest != roleFixtureDigest || claim.ResultDigest != roleResultDigest || claim.EvaluatorVersion != nativerouting.EvaluatorDecodePinnedV1 {
					t.Fatalf("qualification %q is not bound to the decode-pinned fixture", qualification.QualificationID)
				}
			}
		}
	}
	if len(fixture.Cases) != len(wantCases) {
		t.Fatalf("fixture cases = %d, catalog profiles = %d", len(fixture.Cases), len(wantCases))
	}
	for _, testCase := range fixture.Cases {
		key := string(testCase.Role) + "/" + testCase.NativeProfileID + "/" + testCase.ProviderModel + "/" + strings.Join(testCase.Provides, ",")
		if !wantCases[key] {
			t.Fatalf("fixture contains omitted or foreign profile %q", key)
		}
		delete(wantCases, key)
	}
	if len(wantCases) != 0 {
		t.Fatalf("fixture omitted profiles: %v", wantCases)
	}
	if len(results.Results) != len(wantResults) {
		t.Fatalf("result cases = %d, qualifications = %d", len(results.Results), len(wantResults))
	}
	wantComponentResults := []string{"capability:pass", "deliberation:pass", "context:pass", "assurance:pass"}
	for _, result := range results.Results {
		key := string(result.Role) + "/" + result.NativeProfileID
		if wantResults[key] != result.LaunchCaptureDigest || !slices.Equal(result.ComponentResults, wantComponentResults) {
			t.Fatalf("result evidence differs from qualification %q: %+v", key, result)
		}
		delete(wantResults, key)
	}
	if len(wantResults) != 0 {
		t.Fatalf("result evidence omitted qualifications: %v", wantResults)
	}
}

func TestTypedLaunchBuildAndCaptureFidelity(t *testing.T) {
	adapter := mustAdapter(t)
	selected := resolveProfile(t, adapter, nativerouting.RoleImplementation, executiondemand.ProfileMinimum{
		Capability: executiondemand.CapabilityStrong, Deliberation: executiondemand.DeliberationDeep,
		Context: executiondemand.ContextExtended, Assurance: executiondemand.AssuranceElevated,
	})
	native, err := adapter.Build(selected)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	args := nativeLaunchArgs(native, "implement the selected route", "")
	if slices.Contains(args, "--effort") || strings.Contains(strings.Join(args, " "), selected.NativeEffort) {
		t.Fatalf("OpenCode launch fabricated an effort argument: %v", args)
	}
	capture, err := captureLaunch(args)
	if err != nil {
		t.Fatalf("captureLaunch: %v", err)
	}
	if !capture.MatchesSelected(selected) {
		t.Fatalf("selected and captured native launch differ: selected=%+v capture=%+v", selected, capture)
	}
	if err := adapter.VerifyLaunch(selected, capture); err != nil {
		t.Fatalf("VerifyLaunch(equal): %v", err)
	}

	commander := &fakeCommander{procStdout: []byte(resultJSON)}
	liveAdapter := mustAdapter(t, withCommander(commander))
	handle, err := liveAdapter.Launch(context.Background(), port.LaunchSpec{
		Brief: "implement the selected route", Workdir: t.TempDir(), Model: selected.NativeModel, Effort: "descriptive-only",
	})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	actual, err := liveAdapter.LaunchCapture(handle)
	if err != nil {
		t.Fatalf("LaunchCapture: %v", err)
	}
	if !actual.MatchesSelected(selected) {
		t.Fatalf("actual launch capture differs from selection: selected=%+v actual=%+v", selected, actual)
	}
}

func TestLaunchDifferencesFailAndInvalidateQualification(t *testing.T) {
	adapter := mustAdapter(t)
	selected := resolveProfile(t, adapter, nativerouting.RoleReview, executiondemand.ProfileMinimum{
		Capability: executiondemand.CapabilityStandard, Deliberation: executiondemand.DeliberationStandard,
		Context: executiondemand.ContextLarge, Assurance: executiondemand.AssuranceElevated,
	})
	native, err := adapter.Build(selected)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	base := nativeLaunchArgs(native, "review selected route", "")
	for name, mutate := range map[string]func([]string) []string{
		"provider/model": func(args []string) []string { args[4] = "anthropic/claude-opus-4-8"; return args },
		"typed control":  func(args []string) []string { args[2] = "yaml"; return args },
		"raw argument": func(args []string) []string {
			return slices.Insert(args, len(args)-1, "--effort", "high")
		},
	} {
		t.Run(name, func(t *testing.T) {
			changed := mutate(append([]string(nil), base...))
			capture, captureErr := captureLaunch(changed)
			if captureErr == nil {
				var fidelityError LaunchFidelityError
				if err := adapter.VerifyLaunch(selected, capture); !errors.As(err, &fidelityError) || fidelityError.QualificationID != selected.QualificationID {
					t.Fatalf("changed native launch did not fail its qualification: %v", err)
				}
			}
			invalidateSelectedQualification(t, adapter, selected)
		})
	}
}

func invalidateSelectedQualification(t *testing.T, adapter *Adapter, selected nativerouting.SelectedProfile) {
	t.Helper()
	catalog, qualifications, err := adapter.Catalog(selected.Role)
	if err != nil {
		t.Fatalf("Catalog(%s): %v", selected.Role, err)
	}
	for i := range qualifications {
		if qualifications[i].QualificationID != selected.QualificationID {
			continue
		}
		qualifications[i].Validity.Invalidations = append(qualifications[i].Validity.Invalidations, nativerouting.Invalidation{
			EventID: "launch-mismatch", At: routingNow, Reason: "selected-versus-actual native launch mismatch",
		})
		for j := range catalog.Entries {
			if catalog.Entries[j].QualificationID == selected.QualificationID {
				catalog.Entries[j].QualificationDigest = nativerouting.QualificationDigest(qualifications[i])
			}
		}
	}
	validated, diagnostics := nativerouting.ValidateCatalog(catalog, qualifications, adapter, routingClock{}, routingAuthorization{})
	if len(validated.Entries()) != 0 {
		t.Fatal("launch-invalidated qualification left usable candidates")
	}
	for _, diagnostic := range diagnostics {
		if diagnostic.Defect == "qualification_invalidated" && diagnostic.QualificationID == selected.QualificationID {
			return
		}
	}
	t.Fatalf("launch mismatch did not invalidate qualification %q: %v", selected.QualificationID, diagnostics)
}

func TestLaunchBuilderRejectsUnrecognizedNativeSettings(t *testing.T) {
	adapter := mustAdapter(t)
	selected := resolveProfile(t, adapter, nativerouting.RoleImplementation, executiondemand.ProfileMinimum{
		Capability: executiondemand.CapabilityEconomy, Deliberation: executiondemand.DeliberationLight,
		Context: executiondemand.ContextStandard, Assurance: executiondemand.AssuranceOrdinary,
	})
	for name, mutate := range map[string]func(*nativerouting.SelectedProfile){
		"provider/model": func(profile *nativerouting.SelectedProfile) { profile.NativeModel = "provider/latest" },
		"effort":         func(profile *nativerouting.SelectedProfile) { profile.NativeEffort = "high" },
		"control": func(profile *nativerouting.SelectedProfile) {
			profile.Controls = append(profile.Controls, nativerouting.NativeControl{Name: "raw_arguments", Type: nativerouting.ControlString, StringValue: "--dangerously-skip-permissions"})
		},
		"generation": func(profile *nativerouting.SelectedProfile) { profile.AdapterGeneration = "opencode-native-routing/0" },
	} {
		t.Run(name, func(t *testing.T) {
			changed := selected
			changed.Controls = append([]nativerouting.NativeControl(nil), selected.Controls...)
			mutate(&changed)
			if _, err := adapter.Build(changed); err == nil {
				t.Fatal("Build accepted an unrecognized native selection")
			}
		})
	}
	if _, _, err := adapter.Catalog("worker"); err == nil {
		t.Fatal("Catalog accepted an unknown role")
	}
}
