package conformancetest

import (
	"bytes"
	"encoding/json"
	"errors"
	"slices"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/entity"
	"github.com/agenticlab-ai/pyxis/internal/executiondemand"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/runtime"
	"github.com/agenticlab-ai/pyxis/internal/nativerouting"
)

type resolverSideEffectRecorder struct {
	inference int
	network   int
	terminals [][]byte
}

func (r *resolverSideEffectRecorder) resolve(request nativerouting.Request, catalog nativerouting.ValidatedCatalog) nativerouting.Resolution {
	result := (nativerouting.Resolver{}).Resolve(request, catalog)
	terminal, err := nativerouting.CanonicalResolutionJSON(result)
	if err != nil {
		panic(err)
	}
	r.terminals = append(r.terminals, terminal)
	return result
}

func liveOwners(t *testing.T) map[string]catalogOwner {
	t.Helper()
	registry := runtime.LiveRegistry()
	owners := make(map[string]catalogOwner, len(registry.Harnesses()))
	for _, harness := range registry.Harnesses() {
		owner, err := registry.Resolve(harness)
		if err != nil {
			t.Fatal(err)
		}
		owners[harness.String()] = owner.(catalogOwner)
	}
	return owners
}

func liveCatalog(t *testing.T, owner catalogOwner, role nativerouting.Role) (nativerouting.Catalog, nativerouting.QualificationSet, nativerouting.ValidatedCatalog) {
	t.Helper()
	catalog, qualifications, err := owner.Catalog(role)
	if err != nil {
		t.Fatal(err)
	}
	validated, diagnostics := nativerouting.ValidateCatalog(catalog, qualifications, owner, fakeClock{matrixTime}, liveAuthorization{})
	if len(diagnostics) != 0 {
		t.Fatalf("live catalog is invalid: %v", diagnostics)
	}
	return catalog, qualifications, validated
}

func reseal(catalog *nativerouting.Catalog, qualifications nativerouting.QualificationSet) {
	for i := range catalog.Entries {
		for _, qualification := range qualifications {
			if catalog.Entries[i].QualificationID == qualification.QualificationID {
				catalog.Entries[i].QualificationDigest = nativerouting.QualificationDigest(qualification)
			}
		}
	}
}

func TestLiveAdaptersRejectCompleteAbuseCorpusWithoutLaunchAuthority(t *testing.T) {
	type abuseCase struct {
		defect string
		mutate func(*nativerouting.Catalog, *nativerouting.QualificationSet)
	}
	cases := map[string]abuseCase{
		"malformed": {"unknown_schema_version", func(c *nativerouting.Catalog, q *nativerouting.QualificationSet) {
			(*q)[0].SchemaVersion = "native-profile-qualification/v0"
			reseal(c, *q)
		}},
		"missing": {"missing_qualification", func(_ *nativerouting.Catalog, q *nativerouting.QualificationSet) { *q = (*q)[1:] }},
		"stale": {"qualification_expired", func(c *nativerouting.Catalog, q *nativerouting.QualificationSet) {
			(*q)[0].Validity.ValidUntil = matrixTime
			reseal(c, *q)
		}},
		"unqualified": {"self_declared_evidence", func(c *nativerouting.Catalog, q *nativerouting.QualificationSet) {
			(*q)[0].Claims[0].EvidenceOrigin = nativerouting.EvidenceWorker
			reseal(c, *q)
		}},
		"insufficient": {"component_insufficient", func(c *nativerouting.Catalog, q *nativerouting.QualificationSet) {
			(*q)[0].Claims[0].Satisfied = false
			reseal(c, *q)
		}},
		"wrong generation": {"wrong_adapter_generation", func(c *nativerouting.Catalog, q *nativerouting.QualificationSet) {
			(*q)[0].AdapterGeneration += "-stale"
			reseal(c, *q)
		}},
		"ambiguous rank": {"duplicate_cost_rank", func(c *nativerouting.Catalog, _ *nativerouting.QualificationSet) {
			c.Entries[1].CostRank = c.Entries[0].CostRank
		}},
		"injected": {"injected_qualification", func(_ *nativerouting.Catalog, q *nativerouting.QualificationSet) {
			injected := (*q)[0]
			injected.QualificationID += "-injected"
			*q = append(*q, injected)
		}},
		"unauthorized": {"unauthorized", func(c *nativerouting.Catalog, q *nativerouting.QualificationSet) {
			(*q)[0].Authorization.Repository = "untrusted/fork"
			reseal(c, *q)
		}},
	}

	for harness, owner := range liveOwners(t) {
		for _, role := range []nativerouting.Role{nativerouting.RoleImplementation, nativerouting.RoleReview} {
			for name, test := range cases {
				t.Run(harness+"/"+string(role)+"/"+name, func(t *testing.T) {
					catalog, qualifications, _ := liveCatalog(t, owner, role)
					test.mutate(&catalog, &qualifications)
					validated, diagnostics := nativerouting.ValidateCatalog(catalog, qualifications, owner, fakeClock{matrixTime}, liveAuthorization{})
					if len(validated.Entries()) != 0 {
						t.Fatal("invalid policy returned launch authority")
					}
					firstDiagnostics, _ := json.Marshal(diagnostics)
					slices.Reverse(catalog.Entries)
					slices.Reverse(qualifications)
					_, reorderedDiagnostics := nativerouting.ValidateCatalog(catalog, qualifications, owner, fakeClock{matrixTime}, liveAuthorization{})
					secondDiagnostics, _ := json.Marshal(reorderedDiagnostics)
					if !bytes.Equal(firstDiagnostics, secondDiagnostics) {
						t.Fatalf("input order changed diagnostics:\n%s\n%s", firstDiagnostics, secondDiagnostics)
					}
					for _, diagnostic := range diagnostics {
						if diagnostic.Defect == test.defect && diagnostic.Harness == entity.Harness(harness) && diagnostic.Role == role && diagnostic.CatalogID == catalog.CatalogID && diagnostic.QualificationID != "" {
							return
						}
					}
					t.Fatalf("defect %q lost harness/role/catalog/qualification context: %v", test.defect, diagnostics)
				})
			}
		}
	}
}

func TestLiveAdaptersReplaceOnlyCompleteAuthorizedNonWeakeningPolicy(t *testing.T) {
	for harness, owner := range liveOwners(t) {
		for _, role := range []nativerouting.Role{nativerouting.RoleImplementation, nativerouting.RoleReview} {
			catalog, qualifications, _ := liveCatalog(t, owner, role)
			base := nativerouting.DefaultCatalog{
				Catalog: catalog, Qualifications: qualifications, Capability: owner,
				Clock: fakeClock{matrixTime}, ResolverCompatibility: nativerouting.ResolverCompatibility,
			}
			override := nativerouting.CatalogOverride{
				SchemaVersion: nativerouting.CatalogOverrideSchema, ResolverCompatibility: nativerouting.ResolverCompatibility,
				Harness: entity.Harness(harness), Role: role, CatalogReference: "policy/" + harness + "/catalog.json",
				QualificationReference: "policy/" + harness + "/qualifications.json", Authorization: qualifications[0].Authorization,
				Catalog: catalog, Qualifications: qualifications,
			}
			override.Catalog.CatalogID += "-replacement"
			effective, diagnostics := nativerouting.SelectEffectiveCatalog(base, override, liveAuthorization{})
			if len(diagnostics) != 0 || len(effective.Entries()) == 0 || effective.Provenance.OverrideIdentity == "" {
				t.Fatalf("%s/%s complete authorized replacement failed: effective=%#v diagnostics=%v", harness, role, effective, diagnostics)
			}

			weakening := override
			weakening.Catalog.Entries = slices.Clone(override.Catalog.Entries)
			strongest := slices.IndexFunc(weakening.Catalog.Entries, func(entry nativerouting.CatalogEntry) bool { return entry.Strongest })
			removedQualification := weakening.Catalog.Entries[strongest].QualificationID
			weakening.Catalog.Entries = slices.Delete(weakening.Catalog.Entries, strongest, strongest+1)
			for i := range weakening.Catalog.Entries {
				weakening.Catalog.Entries[i].Strongest = i == len(weakening.Catalog.Entries)-1
			}
			weakening.Qualifications = slices.DeleteFunc(slices.Clone(qualifications), func(q nativerouting.Qualification) bool { return q.QualificationID == removedQualification })
			failed, weakeningDiagnostics := nativerouting.SelectEffectiveCatalog(base, weakening, liveAuthorization{})
			if len(failed.Entries()) != 0 || !hasDiagnostic(weakeningDiagnostics, "weakening_override", weakening.Catalog.Entries[len(weakening.Catalog.Entries)-1].QualificationID) {
				t.Fatalf("%s/%s weakening replacement retained launch authority: %v", harness, role, weakeningDiagnostics)
			}

			unauthorized := override
			unauthorized.Authorization.Kind = nativerouting.AuthorizationWorker
			failed, unauthorizedDiagnostics := nativerouting.SelectEffectiveCatalog(base, unauthorized, liveAuthorization{})
			if len(failed.Entries()) != 0 || !hasDefect(unauthorizedDiagnostics, "unauthorized_replacement") {
				t.Fatalf("%s/%s unauthorized replacement retained launch authority: %v", harness, role, unauthorizedDiagnostics)
			}
		}
	}
}

func requestFor(harness string, role nativerouting.Role, profile executiondemand.ProfileMinimum, mode nativerouting.Mode) nativerouting.Request {
	return nativerouting.Request{
		SchemaVersion: nativerouting.RequestSchema, AttemptID: "acceptance-attempt", DemandID: "acceptance-demand",
		Harness: entity.Harness(harness), Role: role, RequestedProfile: profile, Mode: mode,
	}
}

func minimum(provides nativerouting.Provides) executiondemand.ProfileMinimum {
	return executiondemand.ProfileMinimum{Capability: provides.Capability, Deliberation: provides.Deliberation, Context: provides.Context, Assurance: provides.Assurance}
}

func allProfiles() []executiondemand.ProfileMinimum {
	var profiles []executiondemand.ProfileMinimum
	for _, capability := range []executiondemand.Capability{"economy", "standard", "strong", "frontier"} {
		for _, deliberation := range []executiondemand.Deliberation{"light", "standard", "deep", "maximum"} {
			for _, context := range []executiondemand.Context{"standard", "large", "extended"} {
				for _, assurance := range []executiondemand.Assurance{"ordinary", "elevated", "critical"} {
					profiles = append(profiles, executiondemand.ProfileMinimum{Capability: capability, Deliberation: deliberation, Context: context, Assurance: assurance})
				}
			}
		}
	}
	return profiles
}

func findClass(t *testing.T, harness string, role nativerouting.Role, catalog nativerouting.ValidatedCatalog, class nativerouting.ResolutionClass) nativerouting.Request {
	t.Helper()
	for _, profile := range allProfiles() {
		request := requestFor(harness, role, profile, nativerouting.ModeNormal)
		if (nativerouting.Resolver{}).Resolve(request, catalog).Class == class {
			return request
		}
	}
	t.Fatalf("%s/%s has no %s fixture", harness, role, class)
	return nativerouting.Request{}
}

func boundedCatalog(t *testing.T, owner catalogOwner, role nativerouting.Role) nativerouting.ValidatedCatalog {
	t.Helper()
	catalog, qualifications, _ := liveCatalog(t, owner, role)
	strongest := slices.IndexFunc(catalog.Entries, func(entry nativerouting.CatalogEntry) bool { return entry.Strongest })
	removedID := catalog.Entries[strongest].QualificationID
	catalog.Entries = slices.Delete(catalog.Entries, strongest, strongest+1)
	for i := range catalog.Entries {
		catalog.Entries[i].Strongest = i == len(catalog.Entries)-1
	}
	qualifications = slices.DeleteFunc(qualifications, func(q nativerouting.Qualification) bool { return q.QualificationID == removedID })
	validated, diagnostics := nativerouting.ValidateCatalog(catalog, qualifications, owner, fakeClock{matrixTime}, liveAuthorization{})
	if len(diagnostics) != 0 {
		t.Fatalf("bounded policy is invalid: %v", diagnostics)
	}
	return validated
}

func TestLiveResolutionIsDeterministicOfflineAndStrongestPreservesCallerProcess(t *testing.T) {
	type reviewProcess struct {
		Depth     string `json:"review_depth"`
		PassCount int    `json:"pass_count"`
		Blindness string `json:"blindness"`
		Aggregate string `json:"aggregate"`
	}
	process := reviewProcess{Depth: "critical", PassCount: 4, Blindness: "independent", Aggregate: "unanimous"}
	processBytes, _ := json.Marshal(process)

	for harness, owner := range liveOwners(t) {
		selectedByRole := map[nativerouting.Role]string{}
		for _, role := range []nativerouting.Role{nativerouting.RoleImplementation, nativerouting.RoleReview} {
			_, _, catalog := liveCatalog(t, owner, role)
			bounded := boundedCatalog(t, owner, role)
			unsupported := findClass(t, harness, role, bounded, nativerouting.ResolutionUnsupported)
			strongestUnsupported := unsupported
			strongestUnsupported.Mode = nativerouting.ModeStrongestSatisfying
			requests := []nativerouting.Request{
				findClass(t, harness, role, catalog, nativerouting.ResolutionExact),
				findClass(t, harness, role, catalog, nativerouting.ResolutionPromoted),
				unsupported,
				requestFor(harness, role, allProfiles()[0], nativerouting.ModeStrongestSatisfying),
				strongestUnsupported,
			}
			catalogs := []nativerouting.ValidatedCatalog{catalog, catalog, bounded, catalog, bounded}
			for i, request := range requests {
				recorder := &resolverSideEffectRecorder{}
				first := recorder.resolve(request, catalogs[i])
				reordered := catalogs[i]
				reordered.Catalog.Entries = slices.Clone(reordered.Catalog.Entries)
				for entryIndex := range reordered.Catalog.Entries {
					reordered.Catalog.Entries[entryIndex].Controls = slices.Clone(reordered.Catalog.Entries[entryIndex].Controls)
					slices.Reverse(reordered.Catalog.Entries[entryIndex].Controls)
				}
				slices.Reverse(reordered.Catalog.Entries)
				second := recorder.resolve(request, reordered)
				third := recorder.resolve(request, catalogs[i])
				firstJSON, _ := nativerouting.CanonicalResolutionJSON(first)
				secondJSON, _ := nativerouting.CanonicalResolutionJSON(second)
				thirdJSON, _ := nativerouting.CanonicalResolutionJSON(third)
				if !bytes.Equal(firstJSON, secondJSON) || !bytes.Equal(firstJSON, thirdJSON) || recorder.inference != 0 || recorder.network != 0 {
					t.Fatalf("case %d changed under repeat/reorder or used side effects", i)
				}
				if first.NativeProfile != nil && !dominatesRequest(first.NativeProfile.Provides, request.RequestedProfile) {
					t.Fatalf("case %d selected a profile below one of four minima: %#v", i, first)
				}
				if first.Class == nativerouting.ResolutionUnsupported && first.NativeProfile != nil {
					t.Fatal("unsupported result carried launch authority")
				}
				if request.Mode == nativerouting.ModeStrongestSatisfying {
					if first.Harness != entity.Harness(harness) || first.Role != role || first.RequestedProfile != request.RequestedProfile {
						t.Fatalf("strongest fallback changed demand, harness, or role: %#v", first)
					}
					after, _ := json.Marshal(process)
					if !bytes.Equal(processBytes, after) {
						t.Fatal("strongest fallback changed caller review-process metadata")
					}
					if first.Class == nativerouting.ResolutionUnsupported {
						if first.NativeProfile != nil || len(first.UnmetComponents) == 0 {
							t.Fatalf("insufficient strongest profile did not fail unsupported: %#v", first)
						}
					} else {
						selectedByRole[role] = first.NativeProfile.NativeProfileID
					}
				}
			}
		}
		if selectedByRole[nativerouting.RoleImplementation] == selectedByRole[nativerouting.RoleReview] {
			t.Fatalf("%s reused one native profile identity across roles", harness)
		}
	}
}

func TestLiveLaunchCaptureMismatchFailsAndInvalidatesQualification(t *testing.T) {
	for harness, owner := range liveOwners(t) {
		for _, role := range []nativerouting.Role{nativerouting.RoleImplementation, nativerouting.RoleReview} {
			catalog, qualifications, validated := liveCatalog(t, owner, role)
			request := requestFor(harness, role, minimum(catalog.Entries[0].Provides), nativerouting.ModeNormal)
			resolution := (nativerouting.Resolver{}).Resolve(request, validated)
			selected := *resolution.NativeProfile
			spec, err := owner.Build(selected)
			if err != nil {
				t.Fatal(err)
			}
			capture, err := owner.CaptureNativeLaunch(spec)
			if err != nil || !port.LaunchFidelity(selected, capture) {
				t.Fatalf("%s/%s baseline launch mismatch: capture=%#v err=%v", harness, role, capture, err)
			}
			for name, mutate := range map[string]func(*port.LaunchCapture){
				"model":  func(c *port.LaunchCapture) { c.NativeModel += "-mismatch" },
				"effort": func(c *port.LaunchCapture) { c.NativeEffort += "-mismatch" },
				"controls": func(c *port.LaunchCapture) {
					c.Controls = append(c.Controls, nativerouting.NativeControl{Name: "injected", Type: nativerouting.ControlString, StringValue: "true"})
				},
			} {
				t.Run(harness+"/"+string(role)+"/"+name, func(t *testing.T) {
					changed := capture
					changed.Controls = slices.Clone(capture.Controls)
					mutate(&changed)
					invalidation, fidelityErr := port.VerifyLaunchFidelity(selected, changed, matrixTime)
					if !errors.Is(fidelityErr, port.ErrLaunchFidelity) || invalidation == nil {
						t.Fatalf("mismatch retained launch authority: invalidation=%#v err=%v", invalidation, fidelityErr)
					}
					invalidated := slices.Clone(qualifications)
					for i := range invalidated {
						if invalidated[i].QualificationID == selected.QualificationID {
							invalidated[i].Validity.Invalidations = append(invalidated[i].Validity.Invalidations, *invalidation)
						}
					}
					invalidCatalog := catalog
					invalidCatalog.Entries = slices.Clone(catalog.Entries)
					reseal(&invalidCatalog, invalidated)
					usable, diagnostics := nativerouting.ValidateCatalog(invalidCatalog, invalidated, owner, fakeClock{matrixTime}, liveAuthorization{})
					if len(usable.Entries()) != 0 || !hasDiagnostic(diagnostics, "qualification_invalidated", selected.QualificationID) {
						t.Fatalf("mismatch did not invalidate qualification %q: %v", selected.QualificationID, diagnostics)
					}
				})
			}
		}
	}
}

func hasDiagnostic(diagnostics []nativerouting.Diagnostic, defect, qualificationID string) bool {
	return slices.ContainsFunc(diagnostics, func(d nativerouting.Diagnostic) bool {
		return d.Defect == defect && d.QualificationID == qualificationID
	})
}

func hasDefect(diagnostics []nativerouting.Diagnostic, defect string) bool {
	return slices.ContainsFunc(diagnostics, func(d nativerouting.Diagnostic) bool { return d.Defect == defect })
}

func dominatesRequest(provides nativerouting.Provides, requested executiondemand.ProfileMinimum) bool {
	capability := map[executiondemand.Capability]int{"economy": 0, "standard": 1, "strong": 2, "frontier": 3}
	deliberation := map[executiondemand.Deliberation]int{"light": 0, "standard": 1, "deep": 2, "maximum": 3}
	context := map[executiondemand.Context]int{"standard": 0, "large": 1, "extended": 2}
	assurance := map[executiondemand.Assurance]int{"ordinary": 0, "elevated": 1, "critical": 2}
	return capability[provides.Capability] >= capability[requested.Capability] &&
		deliberation[provides.Deliberation] >= deliberation[requested.Deliberation] &&
		context[provides.Context] >= context[requested.Context] &&
		assurance[provides.Assurance] >= assurance[requested.Assurance]
}
