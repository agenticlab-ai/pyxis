package nativerouting

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executiondemand"
)

func resolverRequest(profile executiondemand.ProfileMinimum) Request {
	return Request{
		SchemaVersion: RequestSchema, AttemptID: "attempt-1", DemandID: "demand-1",
		Harness: "codex", Role: RoleImplementation, RequestedProfile: profile, Mode: ModeNormal,
	}
}

func resolverEntry(id string, rank int, provides Provides, strongest bool, controls ...NativeControl) CatalogEntry {
	return CatalogEntry{
		NativeProfileID: id, NativeModel: "model-" + id, NativeEffort: "effort-" + id,
		Controls: controls, Provides: provides, CostRank: rank, QualificationID: "qualification-" + id,
		QualificationDigest: "qualified-" + id, AdapterGeneration: "adapter-gen-7", Strongest: strongest,
	}
}

func resolverCatalog(role Role, entries ...CatalogEntry) ValidatedCatalog {
	return ValidatedCatalog{
		Catalog:  Catalog{SchemaVersion: CatalogSchema, CatalogID: "codex-" + string(role) + "-v1", Harness: "codex", Role: role, AdapterGeneration: "adapter-gen-7", Entries: entries},
		Identity: "policy-identity-1",
	}
}

func minimum(provides Provides) executiondemand.ProfileMinimum {
	return executiondemand.ProfileMinimum{
		Capability: provides.Capability, Deliberation: provides.Deliberation,
		Context: provides.Context, Assurance: provides.Assurance,
	}
}

func exhaustiveCatalog() ValidatedCatalog {
	capabilities := []executiondemand.Capability{executiondemand.CapabilityEconomy, executiondemand.CapabilityStandard, executiondemand.CapabilityStrong, executiondemand.CapabilityFrontier}
	deliberations := []executiondemand.Deliberation{executiondemand.DeliberationLight, executiondemand.DeliberationStandard, executiondemand.DeliberationDeep, executiondemand.DeliberationMaximum}
	contexts := []executiondemand.Context{executiondemand.ContextStandard, executiondemand.ContextLarge, executiondemand.ContextExtended}
	assurances := []executiondemand.Assurance{executiondemand.AssuranceOrdinary, executiondemand.AssuranceElevated, executiondemand.AssuranceCritical}
	entries := make([]CatalogEntry, 0, len(capabilities)*len(deliberations)*len(contexts)*len(assurances))
	rank := 0
	for ci, capability := range capabilities {
		for di, deliberation := range deliberations {
			for xi, context := range contexts {
				for ai, assurance := range assurances {
					rank++
					provides := Provides{Capability: capability, Deliberation: deliberation, Context: context, Assurance: assurance}
					id := fmt.Sprintf("c%d-d%d-x%d-a%d", ci, di, xi, ai)
					entries = append(entries, resolverEntry(id, rank, provides, ci == 3 && di == 3 && xi == 2 && ai == 2))
				}
			}
		}
	}
	return resolverCatalog(RoleImplementation, entries...)
}

func TestResolverExhaustiveProfilesSelectUniqueLowestCostDominator(t *testing.T) {
	catalog := exhaustiveCatalog()
	for _, requested := range catalog.Entries() {
		request := resolverRequest(minimum(requested.Provides))
		got := (Resolver{}).Resolve(request, catalog)
		if got.Class != ResolutionExact || got.NativeProfile == nil {
			t.Fatalf("request %#v resolved as %#v", request.RequestedProfile, got)
		}
		if got.NativeProfile.NativeProfileID != requested.NativeProfileID || got.NativeProfile.CostRank != requested.CostRank {
			t.Fatalf("request %#v selected %s/rank %d, want %s/rank %d", request.RequestedProfile, got.NativeProfile.NativeProfileID, got.NativeProfile.CostRank, requested.NativeProfileID, requested.CostRank)
		}
	}
}

func TestResolverConsumesACompletelyValidatedCatalog(t *testing.T) {
	rawCatalog, qualifications := fixturePolicy(RoleImplementation)
	catalog, diagnostics := validateFixture(rawCatalog, qualifications)
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	request := resolverRequest(executiondemand.ProfileMinimum{
		Capability: executiondemand.CapabilityEconomy, Deliberation: executiondemand.DeliberationLight,
		Context: executiondemand.ContextStandard, Assurance: executiondemand.AssuranceOrdinary,
	})
	got := (Resolver{}).Resolve(request, catalog)
	if got.Class != ResolutionExact || got.NativeProfile == nil || got.NativeProfile.NativeProfileID != "economy" || got.PolicyIdentity != catalog.Identity {
		t.Fatalf("validated catalog did not resolve exactly: %#v", got)
	}
}

func promotionCatalog() ValidatedCatalog {
	boolTrue := true
	stringControl := NativeControl{Name: "sandbox", Type: ControlBoolean, BooleanValue: &boolTrue}
	integer := int64(32000)
	integerControl := NativeControl{Name: "token_budget", Type: ControlInteger, IntegerValue: &integer}
	return resolverCatalog(RoleImplementation,
		resolverEntry("cheap-promoted", 1, Provides{
			Capability: executiondemand.CapabilityStandard, Deliberation: executiondemand.DeliberationStandard,
			Context: executiondemand.ContextLarge, Assurance: executiondemand.AssuranceElevated,
		}, false, integerControl, stringControl),
		resolverEntry("costly-exact", 2, Provides{
			Capability: executiondemand.CapabilityEconomy, Deliberation: executiondemand.DeliberationLight,
			Context: executiondemand.ContextStandard, Assurance: executiondemand.AssuranceOrdinary,
		}, false),
		resolverEntry("strongest", 3, Provides{
			Capability: executiondemand.CapabilityFrontier, Deliberation: executiondemand.DeliberationMaximum,
			Context: executiondemand.ContextExtended, Assurance: executiondemand.AssuranceCritical,
		}, true),
	)
}

func TestResolverReportsEveryPromotionOnCheapestCandidate(t *testing.T) {
	request := resolverRequest(executiondemand.ProfileMinimum{
		Capability: executiondemand.CapabilityEconomy, Deliberation: executiondemand.DeliberationLight,
		Context: executiondemand.ContextStandard, Assurance: executiondemand.AssuranceOrdinary,
	})
	got := (Resolver{}).Resolve(request, promotionCatalog())
	if got.Class != ResolutionPromoted || got.NativeProfile == nil || got.NativeProfile.NativeProfileID != "cheap-promoted" {
		t.Fatalf("got %#v", got)
	}
	want := []Component{ComponentCapability, ComponentDeliberation, ComponentContext, ComponentAssurance}
	if !slices.Equal(got.PromotedComponents, want) {
		t.Fatalf("promotions=%v, want %v", got.PromotedComponents, want)
	}
	if got.NativeProfile.Controls[0].Name != "sandbox" || got.NativeProfile.Controls[1].Name != "token_budget" {
		t.Fatalf("controls are not canonical: %#v", got.NativeProfile.Controls)
	}
}

func TestResolverDoesNotCompensateAcrossComponents(t *testing.T) {
	catalog := resolverCatalog(RoleImplementation,
		resolverEntry("capability-surplus", 1, Provides{
			Capability: executiondemand.CapabilityFrontier, Deliberation: executiondemand.DeliberationLight,
			Context: executiondemand.ContextStandard, Assurance: executiondemand.AssuranceOrdinary,
		}, false),
		resolverEntry("deliberation-surplus", 2, Provides{
			Capability: executiondemand.CapabilityEconomy, Deliberation: executiondemand.DeliberationMaximum,
			Context: executiondemand.ContextStandard, Assurance: executiondemand.AssuranceOrdinary,
		}, false),
		resolverEntry("strongest", 3, Provides{
			Capability: executiondemand.CapabilityFrontier, Deliberation: executiondemand.DeliberationMaximum,
			Context: executiondemand.ContextExtended, Assurance: executiondemand.AssuranceCritical,
		}, true),
	)
	request := resolverRequest(executiondemand.ProfileMinimum{
		Capability: executiondemand.CapabilityFrontier, Deliberation: executiondemand.DeliberationMaximum,
		Context: executiondemand.ContextStandard, Assurance: executiondemand.AssuranceOrdinary,
	})
	got := (Resolver{}).Resolve(request, catalog)
	if got.NativeProfile == nil || got.NativeProfile.NativeProfileID != "strongest" {
		t.Fatalf("a surplus compensated for a deficit: %#v", got)
	}
}

func TestResolverUnsupportedNamesEveryUnmetComponentAndNoProfile(t *testing.T) {
	available := Provides{
		Capability: executiondemand.CapabilityEconomy, Deliberation: executiondemand.DeliberationLight,
		Context: executiondemand.ContextStandard, Assurance: executiondemand.AssuranceOrdinary,
	}
	catalog := resolverCatalog(RoleReview, resolverEntry("available", 1, available, true))
	request := resolverRequest(executiondemand.ProfileMinimum{
		Capability: executiondemand.CapabilityFrontier, Deliberation: executiondemand.DeliberationMaximum,
		Context: executiondemand.ContextExtended, Assurance: executiondemand.AssuranceCritical,
	})
	request.Role = RoleReview
	got := (Resolver{}).Resolve(request, catalog)
	if got.Class != ResolutionUnsupported || got.Reason != ReasonUnsupported || got.NativeProfile != nil || len(got.UnmetComponents) != 4 {
		t.Fatalf("got %#v", got)
	}
	for i, component := range componentOrder {
		if got.UnmetComponents[i].Component != component || got.UnmetComponents[i].Requested == "" || got.UnmetComponents[i].Available == "" {
			t.Fatalf("unmet components are incomplete or unordered: %#v", got.UnmetComponents)
		}
	}
}

func TestStrongestModeChangesOnlyCandidatePreference(t *testing.T) {
	request := resolverRequest(executiondemand.ProfileMinimum{
		Capability: executiondemand.CapabilityEconomy, Deliberation: executiondemand.DeliberationLight,
		Context: executiondemand.ContextStandard, Assurance: executiondemand.AssuranceOrdinary,
	})
	normal := (Resolver{}).Resolve(request, promotionCatalog())
	request.Mode = ModeStrongestSatisfying
	strongest := (Resolver{}).Resolve(request, promotionCatalog())
	if strongest.NativeProfile == nil || strongest.NativeProfile.NativeProfileID != "strongest" {
		t.Fatalf("strongest mode got %#v", strongest)
	}
	if strongest.AttemptID != normal.AttemptID || strongest.DemandID != normal.DemandID || strongest.Harness != normal.Harness || strongest.Role != normal.Role || strongest.RequestedProfile != normal.RequestedProfile || strongest.PolicyIdentity != normal.PolicyIdentity {
		t.Fatalf("strongest mode changed caller demand or process identity: normal=%#v strongest=%#v", normal, strongest)
	}

	underpowered := resolverCatalog(RoleImplementation, resolverEntry("available", 1, Provides{
		Capability: executiondemand.CapabilityEconomy, Deliberation: executiondemand.DeliberationLight,
		Context: executiondemand.ContextStandard, Assurance: executiondemand.AssuranceOrdinary,
	}, true))
	request.RequestedProfile.Capability = executiondemand.CapabilityFrontier
	got := (Resolver{}).Resolve(request, underpowered)
	if got.Class != ResolutionUnsupported || got.NativeProfile != nil {
		t.Fatalf("underpowered strongest profile did not fail closed: %#v", got)
	}
	if _, err := CanonicalResolutionJSON(got); err != nil {
		t.Fatalf("single-component unsupported result is not canonical: %v", err)
	}
}

func TestResolverPreservesHarnessAndSeparatesRoles(t *testing.T) {
	request := resolverRequest(executiondemand.ProfileMinimum{
		Capability: executiondemand.CapabilityEconomy, Deliberation: executiondemand.DeliberationLight,
		Context: executiondemand.ContextStandard, Assurance: executiondemand.AssuranceOrdinary,
	})
	for name, mutate := range map[string]func(*Request){
		"harness": func(r *Request) { r.Harness = "cursor" },
		"role":    func(r *Request) { r.Role = RoleReview },
	} {
		t.Run(name, func(t *testing.T) {
			changed := request
			mutate(&changed)
			got := (Resolver{}).Resolve(changed, promotionCatalog())
			if got.Class != ResolutionPolicyInvalid || got.NativeProfile != nil || got.Harness != changed.Harness || got.Role != changed.Role {
				t.Fatalf("resolver substituted catalog identity: %#v", got)
			}
		})
	}
}

func TestResolverIsCanonicalAcrossCandidateControlAndFieldOrder(t *testing.T) {
	catalog := promotionCatalog()
	request := resolverRequest(executiondemand.ProfileMinimum{
		Capability: executiondemand.CapabilityEconomy, Deliberation: executiondemand.DeliberationLight,
		Context: executiondemand.ContextStandard, Assurance: executiondemand.AssuranceOrdinary,
	})
	before := catalog
	before.Catalog.Entries = slices.Clone(catalog.Catalog.Entries)
	first := (Resolver{}).Resolve(request, catalog)
	firstJSON, err := CanonicalResolutionJSON(first)
	if err != nil {
		t.Fatal(err)
	}

	reordered := catalog
	reordered.Catalog.Entries = slices.Clone(catalog.Catalog.Entries)
	slices.Reverse(reordered.Catalog.Entries)
	for i := range reordered.Catalog.Entries {
		slices.Reverse(reordered.Catalog.Entries[i].Controls)
	}
	secondJSON, err := CanonicalResolutionJSON((Resolver{}).Resolve(request, reordered))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstJSON, secondJSON) {
		t.Fatalf("input reordering changed result:\n%s\n%s", firstJSON, secondJSON)
	}
	if !reflect.DeepEqual(catalog.Catalog.Entries, before.Catalog.Entries) {
		t.Fatal("resolver mutated its catalog input")
	}

	profileJSON, _ := json.Marshal(request.RequestedProfile)
	reorderedFields := fmt.Sprintf(`{"mode":"normal","requested_profile":%s,"role":"implementation","harness":"codex","demand_id":"demand-1","attempt_id":"attempt-1","schema_version":"native-route-request/v1"}`, profileJSON)
	parsed, err := ParseRequest([]byte(reorderedFields))
	if err != nil {
		t.Fatal(err)
	}
	thirdJSON, err := CanonicalResolutionJSON((Resolver{}).Resolve(parsed, catalog))
	if err != nil || !bytes.Equal(firstJSON, thirdJSON) {
		t.Fatalf("request field order changed result: err=%v\n%s\n%s", err, firstJSON, thirdJSON)
	}
}

func TestResolverRejectsAmbiguousPolicyWithoutPartialSelection(t *testing.T) {
	catalog := promotionCatalog()
	catalog.Catalog.Entries[1].CostRank = catalog.Catalog.Entries[0].CostRank
	request := resolverRequest(executiondemand.ProfileMinimum{
		Capability: executiondemand.CapabilityEconomy, Deliberation: executiondemand.DeliberationLight,
		Context: executiondemand.ContextStandard, Assurance: executiondemand.AssuranceOrdinary,
	})
	got := (Resolver{}).Resolve(request, catalog)
	if got.Class != ResolutionPolicyInvalid || got.NativeProfile != nil || got.Reason != ReasonPolicyInvalid || len(got.PolicyDefects) == 0 {
		t.Fatalf("ambiguous policy partially resolved: %#v", got)
	}
	found := false
	for _, defect := range got.PolicyDefects {
		found = found || defect.Code == "duplicate_cost_rank"
	}
	if !found {
		t.Fatalf("duplicate rank not named: %#v", got.PolicyDefects)
	}
	if _, err := CanonicalResolutionJSON(got); err != nil {
		t.Fatalf("policy-invalid result is not canonical: %v", err)
	}
}

func TestMissingValidatedPolicyStillReturnsCanonicalPolicyInvalid(t *testing.T) {
	request := resolverRequest(executiondemand.ProfileMinimum{
		Capability: executiondemand.CapabilityEconomy, Deliberation: executiondemand.DeliberationLight,
		Context: executiondemand.ContextStandard, Assurance: executiondemand.AssuranceOrdinary,
	})
	got := (Resolver{}).Resolve(request, ValidatedCatalog{})
	if got.Class != ResolutionPolicyInvalid || got.NativeProfile != nil || got.CatalogID != "" || got.PolicyIdentity != "" {
		t.Fatalf("missing policy did not fail closed: %#v", got)
	}
	if _, err := CanonicalResolutionJSON(got); err != nil {
		t.Fatalf("missing-policy terminal is not canonical: %v", err)
	}
}

func TestClosedRequestAndResolutionGrammar(t *testing.T) {
	request := resolverRequest(executiondemand.ProfileMinimum{
		Capability: executiondemand.CapabilityEconomy, Deliberation: executiondemand.DeliberationLight,
		Context: executiondemand.ContextStandard, Assurance: executiondemand.AssuranceOrdinary,
	})
	requestJSON, _ := json.Marshal(request)
	requestJSON = bytes.Replace(requestJSON, []byte(`"mode":"normal"`), []byte(`"mode":"normal","raw_arguments":["--model=other"]`), 1)
	if _, err := ParseRequest(requestJSON); err == nil {
		t.Fatal("request parser admitted an unknown raw argument")
	}

	resolved := (Resolver{}).Resolve(request, promotionCatalog())
	wire, err := CanonicalResolutionJSON(resolved)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseResolution(wire)
	if err != nil || !reflect.DeepEqual(parsed, resolved) {
		t.Fatalf("resolution did not round trip: err=%v got=%#v want=%#v", err, parsed, resolved)
	}

	broken := resolved
	broken.NativeProfile = nil
	if _, err := CanonicalResolutionJSON(broken); err == nil {
		t.Fatal("promoted resolution admitted no native profile")
	}
	broken = resolved
	broken.UnmetComponents = []UnmetComponent{{Component: ComponentCapability, Requested: "frontier", Available: "economy"}}
	if _, err := CanonicalResolutionJSON(broken); err == nil {
		t.Fatal("resolved terminal admitted unmet components")
	}
}
