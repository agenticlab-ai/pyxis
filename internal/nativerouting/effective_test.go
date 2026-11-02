package nativerouting

import (
	"slices"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/executiondemand"
)

func effectiveFixture() (DefaultCatalog, CatalogOverride) {
	defaultCatalog, defaultQualifications := fixturePolicy(RoleImplementation)
	overrideCatalog, overrideQualifications := fixturePolicy(RoleImplementation)
	overrideCatalog.CatalogID = "codex-implementation-repository-v2"
	base := DefaultCatalog{
		Catalog: defaultCatalog, Qualifications: defaultQualifications,
		Capability: fixtureCapability{}, Clock: fixedClock{fixtureNow}, ResolverCompatibility: ResolverCompatibility,
	}
	override := CatalogOverride{
		SchemaVersion: CatalogOverrideSchema, ResolverCompatibility: ResolverCompatibility,
		Harness: "codex", Role: RoleImplementation,
		CatalogReference:       ".pyxis/native-routing/codex-implementation.json",
		QualificationReference: ".pyxis/native-routing/codex-implementation-qualifications.json",
		Authorization:          overrideQualifications[0].Authorization,
		Catalog:                overrideCatalog, Qualifications: overrideQualifications,
	}
	return base, override
}

func TestAuthorizedCompleteReplacementIsSoleEffectivePolicy(t *testing.T) {
	base, override := effectiveFixture()
	effective, diagnostics := SelectEffectiveCatalog(base, override, fixtureAuthorization{})
	if len(diagnostics) != 0 {
		t.Fatalf("SelectEffectiveCatalog: %v", diagnostics)
	}
	if effective.Catalog.Catalog.CatalogID != override.Catalog.CatalogID || len(effective.Entries()) != len(override.Catalog.Entries) {
		t.Fatalf("effective catalog is not the complete override: %#v", effective)
	}
	p := effective.Provenance
	if p.DefaultIdentity == "" || p.OverrideIdentity == "" || p.AuthorizationIdentity == "" || p.EffectiveDigest == "" {
		t.Fatalf("replacement provenance is incomplete: %#v", p)
	}
	if p.DefaultIdentity == p.OverrideIdentity || p.OverrideIdentity == p.EffectiveIdentity || effective.Catalog.Identity != p.EffectiveIdentity {
		t.Fatalf("replacement provenance identities disagree: %#v", p)
	}

	resolution := (Resolver{}).Resolve(Request{
		SchemaVersion: RequestSchema, AttemptID: "attempt-1", DemandID: "demand-1", Harness: "codex", Role: RoleImplementation,
		RequestedProfile: executiondemand.ProfileMinimum{Capability: executiondemand.CapabilityEconomy, Deliberation: executiondemand.DeliberationLight, Context: executiondemand.ContextStandard, Assurance: executiondemand.AssuranceOrdinary},
		Mode:             ModeNormal,
	}, effective.Catalog)
	if resolution.Class != ResolutionExact || resolution.PolicyIdentity != p.EffectiveIdentity || resolution.CatalogID != override.Catalog.CatalogID {
		t.Fatalf("resolver did not consume only the replacement: %#v", resolution)
	}
}

func TestAbsentOverrideSelectsReleasedDefaultWithProvenance(t *testing.T) {
	base, _ := effectiveFixture()
	effective, diagnostics := SelectEffectiveCatalog(base, CatalogOverride{}, fixtureAuthorization{})
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	p := effective.Provenance
	if p.DefaultIdentity == "" || p.EffectiveIdentity != p.DefaultIdentity || p.OverrideIdentity != "" || p.AuthorizationIdentity != "" || p.EffectiveDigest == "" {
		t.Fatalf("default provenance = %#v", p)
	}
}

func TestEveryPartialTypedOverrideRefusesDefaultFallback(t *testing.T) {
	base, _ := effectiveFixture()
	partials := map[string]CatalogOverride{
		"authorization repository only": {Authorization: AuthorizationProof{Repository: "agenticlab-ai/pyxis"}},
		"authorization operator only":   {Authorization: AuthorizationProof{OperatorID: "operator-1"}},
		"authorization evidence only":   {Authorization: AuthorizationProof{EvidenceDigest: digestJSON("evidence")}},
		"authorization time only":       {Authorization: AuthorizationProof{AuthorizedAt: fixtureNow}},
		"catalog schema only":           {Catalog: Catalog{SchemaVersion: CatalogSchema}},
		"catalog harness only":          {Catalog: Catalog{Harness: "codex"}},
		"catalog role only":             {Catalog: Catalog{Role: RoleImplementation}},
		"catalog generation only":       {Catalog: Catalog{AdapterGeneration: "adapter-gen-7"}},
		"empty catalog entries":         {Catalog: Catalog{Entries: []CatalogEntry{}}},
		"catalog entries only":          {Catalog: Catalog{Entries: []CatalogEntry{{NativeProfileID: "partial"}}}},
		"empty qualifications":          {Qualifications: QualificationSet{}},
	}
	for name, override := range partials {
		t.Run(name, func(t *testing.T) {
			effective, diagnostics := SelectEffectiveCatalog(base, override, fixtureAuthorization{})
			if len(diagnostics) == 0 {
				t.Fatal("partial override was treated as absent")
			}
			if len(effective.Entries()) != 0 || effective.Catalog.Identity != "" || effective.Provenance.EffectiveIdentity != "" {
				t.Fatalf("partial override fell back or exposed candidates: %#v", effective)
			}
		})
	}
}

func TestInvalidOverrideNeverFallsBackAndExposesZeroCandidates(t *testing.T) {
	tests := []struct {
		name   string
		defect string
		mutate func(*CatalogOverride)
	}{
		{"partial reference", "partial_override", func(o *CatalogOverride) { o.QualificationReference = "" }},
		{"raw argument reference", "invalid_policy_reference", func(o *CatalogOverride) { o.CatalogReference += " --model=hacked" }},
		{"incompatible resolver", "incompatible_resolver", func(o *CatalogOverride) { o.ResolverCompatibility = "native-routing-resolver/v2" }},
		{"incompatible generation", "incompatible_adapter_generation", func(o *CatalogOverride) {
			o.Catalog.AdapterGeneration = "adapter-gen-8"
			for i := range o.Catalog.Entries {
				o.Catalog.Entries[i].AdapterGeneration = "adapter-gen-8"
			}
			for i := range o.Qualifications {
				o.Qualifications[i].AdapterGeneration = "adapter-gen-8"
			}
			sealQualifications(&o.Catalog, o.Qualifications)
		}},
		{"unqualified", "missing_qualification", func(o *CatalogOverride) { o.Qualifications = o.Qualifications[1:] }},
		{"weakening", "weakening_override", func(o *CatalogOverride) {
			o.Catalog.Entries[1].Provides.Capability = executiondemand.CapabilityStandard
			o.Qualifications[1].Claims[0].AssertedLevel = string(executiondemand.CapabilityStandard)
			sealQualifications(&o.Catalog, o.Qualifications)
		}},
		{"contributor authorization", "unauthorized_replacement", func(o *CatalogOverride) { o.Authorization.Kind = AuthorizationContributor }},
		{"worker authorization", "unauthorized_replacement", func(o *CatalogOverride) { o.Authorization.Kind = AuthorizationWorker }},
		{"cross-role target", "override_target_mismatch", func(o *CatalogOverride) { o.Role = RoleReview }},
		{"injected qualification", "injected_qualification", func(o *CatalogOverride) {
			extra := o.Qualifications[0]
			extra.QualificationID = "qual-injected"
			o.Qualifications = append(o.Qualifications, extra)
		}},
		{"field-merge shaped catalog", "empty_catalog", func(o *CatalogOverride) {
			o.Catalog = Catalog{SchemaVersion: CatalogSchema, CatalogID: o.Catalog.CatalogID, Harness: o.Harness, Role: o.Role, AdapterGeneration: "adapter-gen-7"}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			base, override := effectiveFixture()
			test.mutate(&override)
			effective, diagnostics := SelectEffectiveCatalog(base, override, fixtureAuthorization{})
			if len(effective.Entries()) != 0 || effective.Catalog.Identity != "" || effective.Provenance.EffectiveIdentity != "" {
				t.Fatalf("invalid replacement fell back or exposed candidates: %#v", effective)
			}
			if _, ok := diagnosticWith(test.defect, diagnostics); !ok {
				t.Fatalf("diagnostics %v do not contain %q", diagnostics, test.defect)
			}
		})
	}
}

func TestReplacementOrderingDoesNotChangeProvenance(t *testing.T) {
	base, override := effectiveFixture()
	first, diagnostics := SelectEffectiveCatalog(base, override, fixtureAuthorization{})
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	slices.Reverse(override.Catalog.Entries)
	slices.Reverse(override.Qualifications)
	second, diagnostics := SelectEffectiveCatalog(base, override, fixtureAuthorization{})
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	if first.Provenance != second.Provenance {
		t.Fatalf("ordering changed provenance:\nfirst  %#v\nsecond %#v", first.Provenance, second.Provenance)
	}
}

func TestReplacementProvenanceBindsDeclarationAndAuthorization(t *testing.T) {
	base, override := effectiveFixture()
	first, diagnostics := SelectEffectiveCatalog(base, override, fixtureAuthorization{})
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}

	withOtherReference := override
	withOtherReference.CatalogReference = ".pyxis/native-routing/alternate-codex-implementation.json"
	second, diagnostics := SelectEffectiveCatalog(base, withOtherReference, fixtureAuthorization{})
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	if first.Provenance.EffectiveIdentity != second.Provenance.EffectiveIdentity || first.Provenance.OverrideIdentity == second.Provenance.OverrideIdentity || first.Provenance.EffectiveDigest == second.Provenance.EffectiveDigest {
		t.Fatalf("replacement reference is not independently bound:\nfirst  %#v\nsecond %#v", first.Provenance, second.Provenance)
	}

	withOtherAuthorization := override
	withOtherAuthorization.Authorization.EvidenceDigest = fixtureDigest("alternate-authorization")
	third, diagnostics := SelectEffectiveCatalog(base, withOtherAuthorization, fixtureAuthorization{})
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	if first.Provenance.OverrideIdentity != third.Provenance.OverrideIdentity || first.Provenance.AuthorizationIdentity == third.Provenance.AuthorizationIdentity || first.Provenance.EffectiveDigest == third.Provenance.EffectiveDigest {
		t.Fatalf("authorization is not independently bound:\nfirst %#v\nthird %#v", first.Provenance, third.Provenance)
	}
}

func TestReplacementUsesOneTrustedInstant(t *testing.T) {
	base, override := effectiveFixture()
	clock := &advancingClock{at: fixtureNow}
	base.Clock = clock

	effective, diagnostics := SelectEffectiveCatalog(base, override, fixtureAuthorization{})
	if len(diagnostics) != 0 || len(effective.Entries()) == 0 {
		t.Fatalf("one selection used different validation instants: effective=%#v diagnostics=%v", effective, diagnostics)
	}
	if clock.calls != 1 {
		t.Fatalf("trusted clock sampled %d times, want exactly once", clock.calls)
	}
}

type advancingClock struct {
	at    time.Time
	calls int
}

func (c *advancingClock) Now() time.Time {
	at := c.at.Add(time.Duration(c.calls) * 2 * time.Hour)
	c.calls++
	return at
}

func TestInvalidReplacementDiagnosticsAreOrderIndependent(t *testing.T) {
	base, override := effectiveFixture()
	for _, id := range []string{"qual-injected-z", "qual-injected-a"} {
		extra := override.Qualifications[0]
		extra.QualificationID = id
		override.Qualifications = append(override.Qualifications, extra)
	}
	_, first := SelectEffectiveCatalog(base, override, fixtureAuthorization{})
	slices.Reverse(override.Qualifications)
	_, second := SelectEffectiveCatalog(base, override, fixtureAuthorization{})
	if !slices.Equal(first, second) {
		t.Fatalf("qualification ordering changed diagnostics:\nfirst  %#v\nsecond %#v", first, second)
	}
}
