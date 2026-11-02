package claude

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/executiondemand"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/runtime"
	"github.com/agenticlab-ai/pyxis/internal/nativerouting"
)

type routingClock struct{ at time.Time }

func (c routingClock) Now() time.Time { return c.at }

type routingAuthorization struct{}

func (routingAuthorization) Authorize(proof nativerouting.AuthorizationProof) error {
	if proof.Repository != "agenticlab-ai/pyxis" || proof.OperatorID != "@mdarabi" {
		return errors.New("qualification is not repository-operator authorized")
	}
	return nil
}

var qualificationTime = time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)

func validatedClaudeCatalog(t *testing.T, a *Adapter, role nativerouting.Role) nativerouting.ValidatedCatalog {
	t.Helper()
	catalog, qualifications, err := a.Catalog(role)
	if err != nil {
		t.Fatal(err)
	}
	validated, diagnostics := nativerouting.ValidateCatalog(catalog, qualifications, a, routingClock{qualificationTime}, routingAuthorization{})
	if len(diagnostics) != 0 {
		t.Fatalf("%s catalog rejected: %v", role, diagnostics)
	}
	return validated
}

func TestCatalogsAreCompleteQualifiedAndRoleIndependent(t *testing.T) {
	a := mustAdapter(t)
	implementation := validatedClaudeCatalog(t, a, nativerouting.RoleImplementation)
	review := validatedClaudeCatalog(t, a, nativerouting.RoleReview)
	if implementation.Identity == review.Identity || implementation.Catalog.CatalogID == review.Catalog.CatalogID {
		t.Fatal("implementation and review catalogs share an identity")
	}
	if len(implementation.Entries()) != 4 || len(review.Entries()) != 4 {
		t.Fatalf("catalog sizes implementation=%d review=%d, want 4 each", len(implementation.Entries()), len(review.Entries()))
	}
	for _, role := range []nativerouting.Role{nativerouting.RoleImplementation, nativerouting.RoleReview} {
		catalog, qualifications, err := a.Catalog(role)
		if err != nil {
			t.Fatal(err)
		}
		if len(qualifications) != len(catalog.Entries) {
			t.Fatalf("%s qualifications=%d entries=%d", role, len(qualifications), len(catalog.Entries))
		}
		for _, q := range qualifications {
			if len(q.Claims) != 4 || !q.Validity.ValidFrom.Before(qualificationTime) || !qualificationTime.Before(q.Validity.ValidUntil) {
				t.Fatalf("qualification %q is incomplete or not current: %#v", q.QualificationID, q)
			}
			for _, claim := range q.Claims {
				if claim.Method != nativerouting.MethodDecodePinnedEval || claim.EvaluatorVersion != nativerouting.EvaluatorDecodePinnedV1 || !claim.Satisfied {
					t.Fatalf("qualification %q claim is not decode-pinned: %#v", q.QualificationID, claim)
				}
			}
		}
	}

	// Each call rebuilds policy data, so callers cannot mutate the released
	// roster or evidence observed by another resolution.
	catalog, _, _ := a.Catalog(nativerouting.RoleImplementation)
	catalog.Entries[0].NativeModel = "injected"
	fresh, _, _ := a.Catalog(nativerouting.RoleImplementation)
	if fresh.Entries[0].NativeModel == "injected" {
		t.Fatal("Catalog leaked mutable roster data")
	}
}

func derivableProfiles(t *testing.T) []executiondemand.ProfileMinimum {
	t.Helper()
	set := map[executiondemand.ProfileMinimum]bool{}
	evidence := executiondemand.EvidenceSet{Resolved: map[string]struct{}{"fixture#assessment": {}, "fixture#raise": {}}}
	for _, reasoning := range []executiondemand.Reasoning{executiondemand.ReasoningRoutine, executiondemand.ReasoningComplex, executiondemand.ReasoningDifficult} {
		for _, coupling := range []executiondemand.Coupling{executiondemand.CouplingContained, executiondemand.CouplingComposed, executiondemand.CouplingCrossBoundary} {
			for _, novelty := range []executiondemand.Novelty{executiondemand.NoveltyKnown, executiondemand.NoveltyAdapt, executiondemand.NoveltyFirstOfKind} {
				for _, verification := range []executiondemand.Verification{executiondemand.VerificationLocal, executiondemand.VerificationComposed, executiondemand.VerificationCrossBoundary} {
					for _, impact := range []executiondemand.ImpactScope{executiondemand.ImpactContained, executiondemand.ImpactMultiSurface, executiondemand.ImpactSystemWide} {
						for _, architecturalRole := range []executiondemand.ArchitecturalRole{executiondemand.RoleFeatureLocal, executiondemand.RoleSharedSubstrate, executiondemand.RoleSystemCore} {
							assessment := executiondemand.Assessment{Reasoning: reasoning, Coupling: coupling, Novelty: novelty, Verification: verification, ImpactScope: impact, ArchitecturalRole: architecturalRole, Citations: []executiondemand.Citation{{ID: "fixture#assessment"}}}
							for _, raise := range []executiondemand.Assurance{"", executiondemand.AssuranceElevated, executiondemand.AssuranceCritical} {
								var assuranceRaise executiondemand.AssuranceRaise
								if raise != "" {
									assuranceRaise = executiondemand.AssuranceRaise{Value: raise, Citations: []executiondemand.Citation{{ID: "fixture#raise"}}}
								}
								profile, err := executiondemand.Evaluate(assessment, evidence, assuranceRaise, executiondemand.DefaultPolicy)
								if err != nil { // a raise below the derived floor is not a derivable profile.
									continue
								}
								set[executiondemand.ProfileMinimum{Capability: profile.Capability, Deliberation: profile.Deliberation, Context: profile.Context, Assurance: profile.Assurance}] = true
							}
						}
					}
				}
			}
		}
	}
	out := make([]executiondemand.ProfileMinimum, 0, len(set))
	for profile := range set {
		out = append(out, profile)
	}
	return out
}

func expectedClaudeProfile(role nativerouting.Role, p executiondemand.ProfileMinimum) string {
	if role == nativerouting.RoleImplementation {
		switch {
		case p == (executiondemand.ProfileMinimum{Capability: "economy", Deliberation: "light", Context: "standard", Assurance: "ordinary"}):
			return "haiku-low"
		case p.Capability != "strong" && p.Capability != "frontier" && p.Deliberation != "deep" && p.Deliberation != "maximum" && p.Context != "extended" && p.Assurance == "ordinary":
			return "sonnet-medium"
		case p.Capability != "frontier" && p.Deliberation != "maximum" && p.Assurance != "critical":
			return "sonnet-high"
		default:
			return "opus-max"
		}
	}
	switch {
	case p.Capability == "economy" && p.Deliberation != "deep" && p.Deliberation != "maximum" && p.Context == "standard" && p.Assurance == "ordinary":
		return "haiku-medium-review"
	case p.Capability != "strong" && p.Capability != "frontier" && p.Deliberation != "maximum" && p.Context != "extended" && p.Assurance != "critical":
		return "sonnet-high-review"
	case p.Capability != "frontier" && p.Deliberation != "maximum" && p.Assurance != "critical":
		return "opus-high-review"
	default:
		return "opus-max-review"
	}
}

func TestNativeRoutingConformanceEveryDerivableCell(t *testing.T) {
	a := mustAdapter(t)
	profiles := derivableProfiles(t)
	cases := make([]runtime.NativeRoutingConformanceCase, 0, len(profiles)*2)
	for _, role := range []nativerouting.Role{nativerouting.RoleImplementation, nativerouting.RoleReview} {
		catalog := validatedClaudeCatalog(t, a, role)
		for _, requested := range profiles {
			wantID := expectedClaudeProfile(role, requested)
			wantClass := nativerouting.ResolutionPromoted
			for _, entry := range catalog.Entries() {
				if entry.NativeProfileID == wantID && entry.Provides == (nativerouting.Provides{Capability: requested.Capability, Deliberation: requested.Deliberation, Context: requested.Context, Assurance: requested.Assurance}) {
					wantClass = nativerouting.ResolutionExact
				}
			}
			cases = append(cases, runtime.NativeRoutingConformanceCase{Name: fmt.Sprintf("%s/%v", role, requested), Role: role, Requested: requested, WantClass: wantClass, WantNativeProfileID: wantID})
		}
	}
	err := runtime.CheckNativeRoutingConformance(runtime.NativeRoutingConformanceConfig{
		Harness: harnessID, Provider: a, Capability: a, Builder: a,
		Clock: routingClock{qualificationTime}, Authorization: routingAuthorization{}, Cases: cases,
		LaunchCapture: func(spec port.NativeLaunchSpec) (port.LaunchCapture, error) {
			fc := &fakeCommander{procStdout: []byte(resultJSON)}
			rt := mustAdapter(t, withCommander(fc))
			purpose := port.LaunchPurposeImplementation
			if spec.Role == nativerouting.RoleReview {
				purpose = port.LaunchPurposeReview
			}
			h, err := rt.Launch(context.Background(), port.LaunchSpec{Purpose: purpose, Brief: "decode-pinned fixture", Workdir: "/fixture", Model: spec.NativeModel, Effort: spec.NativeEffort})
			if err != nil {
				return port.LaunchCapture{}, err
			}
			return rt.LaunchCapture(h)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) == 0 {
		t.Fatal("derivable-domain generator produced no profiles")
	}
}

func selectedImplementationProfile(t *testing.T, a *Adapter) nativerouting.SelectedProfile {
	t.Helper()
	catalog := validatedClaudeCatalog(t, a, nativerouting.RoleImplementation)
	resolution := (nativerouting.Resolver{}).Resolve(nativerouting.Request{
		SchemaVersion: nativerouting.RequestSchema, AttemptID: "attempt-fixture", DemandID: "demand-fixture",
		Harness: harnessID, Role: nativerouting.RoleImplementation,
		RequestedProfile: executiondemand.ProfileMinimum{Capability: "standard", Deliberation: "standard", Context: "large", Assurance: "ordinary"},
		Mode:             nativerouting.ModeNormal,
	}, catalog)
	if resolution.NativeProfile == nil {
		t.Fatalf("fixture did not resolve: %#v", resolution)
	}
	return *resolution.NativeProfile
}

func TestBuildCaptureAndFidelityFailure(t *testing.T) {
	a := mustAdapter(t)
	selected := selectedImplementationProfile(t, a)
	built, err := a.Build(selected)
	if err != nil {
		t.Fatal(err)
	}
	if built.NativeProfileID != selected.NativeProfileID || built.NativeModel != selected.NativeModel || built.NativeEffort != selected.NativeEffort || len(built.Controls) != len(selected.Controls) {
		t.Fatalf("Build changed the selected profile: %#v", built)
	}

	launch := func(opts ...Option) *handle {
		fc := &fakeCommander{procStdout: []byte(resultJSON)}
		rt := mustAdapter(t, append([]Option{withCommander(fc)}, opts...)...)
		h, err := rt.Launch(context.Background(), port.LaunchSpec{Brief: "fixture", Workdir: "/fixture", Model: built.NativeModel, Effort: built.NativeEffort})
		if err != nil {
			t.Fatal(err)
		}
		return h.(*handle)
	}
	if err := a.VerifyLaunch(launch(), selected); err != nil {
		t.Fatalf("matching launch failed fidelity: %v", err)
	}

	mutations := map[string]func(*port.LaunchCapture){
		"model":    func(c *port.LaunchCapture) { c.NativeModel = "claude-opus-5" },
		"effort":   func(c *port.LaunchCapture) { c.NativeEffort = "low" },
		"controls": func(c *port.LaunchCapture) { c.Controls = c.Controls[:1] },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			h := launch()
			mutate(&h.capture)
			err := a.VerifyLaunch(h, selected)
			if !errors.Is(err, ErrLaunchFidelity) || !strings.Contains(err.Error(), selected.QualificationID) {
				t.Fatalf("mismatch err=%v, want fidelity failure naming qualification %q", err, selected.QualificationID)
			}
		})
	}
	// A process-only setting absent from the selected typed controls is surplus
	// actual behavior and therefore also invalidates fidelity.
	if err := a.VerifyLaunch(launch(WithMaxTurns(3)), selected); !errors.Is(err, ErrLaunchFidelity) {
		t.Fatalf("surplus max_turns did not fail fidelity: %v", err)
	}
}

func TestNativeBuilderAndCaptureRejectUnknownSettings(t *testing.T) {
	a := mustAdapter(t)
	base := selectedImplementationProfile(t, a)
	tests := map[string]func(*nativerouting.SelectedProfile){
		"model":  func(s *nativerouting.SelectedProfile) { s.NativeModel = "provider-latest" },
		"effort": func(s *nativerouting.SelectedProfile) { s.NativeEffort = "unbounded" },
		"control": func(s *nativerouting.SelectedProfile) {
			s.Controls = append(s.Controls, nativerouting.NativeControl{Name: "raw_arguments", Type: nativerouting.ControlString, StringValue: "--model hacked"})
		},
		"generation": func(s *nativerouting.SelectedProfile) { s.AdapterGeneration = "unsupported" },
		"role":       func(s *nativerouting.SelectedProfile) { s.Role = "worker" },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			selected := base
			selected.Controls = append([]nativerouting.NativeControl(nil), base.Controls...)
			mutate(&selected)
			if _, err := a.Build(selected); err == nil {
				t.Fatal("Build accepted an unknown native setting")
			}
		})
	}
	if _, err := a.decodeLaunchCapture(append(a.launchArgs(fixtureSpec()), "--raw-arguments", "--model hacked")); err == nil {
		t.Fatal("launch capture accepted a raw/unknown argument")
	}
	if _, _, err := a.Catalog("worker"); err == nil {
		t.Fatal("Catalog accepted an implicit role")
	}
}

func TestReviewCaptureRejectsConflictingSandboxSettings(t *testing.T) {
	a := mustAdapter(t)
	spec := fixtureSpec()
	spec.Purpose = port.LaunchPurposeReview
	for _, value := range []string{`{"sandbox":{"enabled":true}}`, `{"sandbox":{"enabled":false},"hooks":{}}`} {
		args := a.launchArgs(spec)
		for i, arg := range args {
			if arg == "--settings" {
				args[i+1] = value
			}
		}
		if _, err := a.decodeLaunchCapture(args); err == nil {
			t.Fatalf("capture accepted unqualified settings: %s", value)
		}
	}
	enabled := true
	for _, control := range []nativerouting.NativeControl{
		{Name: "sandbox_enabled", Type: nativerouting.ControlBoolean, BooleanValue: &enabled},
		{Name: "sandbox_enabled", Type: nativerouting.ControlBoolean},
		{Name: "sandbox_enabled", Type: nativerouting.ControlString, StringValue: "false"},
	} {
		selected := selectedImplementationProfile(t, a)
		selected.Controls = append(selected.Controls, control)
		if _, err := a.Build(selected); err == nil {
			t.Fatalf("Build accepted malformed sandbox control: %+v", control)
		}
	}
}
