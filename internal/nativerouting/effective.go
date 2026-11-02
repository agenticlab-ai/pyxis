package nativerouting

import (
	"fmt"
	"github.com/agenticlab-ai/pyxis/internal/entity"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	CatalogOverrideSchema       = "native-routing-catalog-override/v1"
	ResolverCompatibility       = "native-routing-resolver/v1"
	ReplacementProvenanceSchema = "native-routing-replacement-provenance/v1"
)

// DefaultCatalog is the released policy and the runtime-owned dependencies
// used to validate either policy source. Keeping capability and clock here
// makes SelectEffectiveCatalog's authorization input the only repository-owned
// authority at the replacement boundary.
type DefaultCatalog struct {
	Catalog               Catalog
	Qualifications        QualificationSet
	Capability            NativeCapability
	Clock                 TrustedClock
	ResolverCompatibility string
}

// CatalogOverride is one self-contained repository replacement. References
// are retained provenance, while Catalog and Qualifications are the complete
// artifacts loaded from those references. Nothing is inherited from Default.
type CatalogOverride struct {
	SchemaVersion          string             `json:"schema_version"`
	ResolverCompatibility  string             `json:"resolver_compatibility"`
	Harness                entity.Harness     `json:"harness"`
	Role                   Role               `json:"role"`
	CatalogReference       string             `json:"catalog_ref"`
	QualificationReference string             `json:"qualifications_ref"`
	Authorization          AuthorizationProof `json:"authorization"`
	Catalog                Catalog            `json:"-"`
	Qualifications         QualificationSet   `json:"-"`
}

func (o CatalogOverride) Present() bool {
	return o.SchemaVersion != "" || o.ResolverCompatibility != "" || o.Harness != "" || o.Role != "" ||
		o.CatalogReference != "" || o.QualificationReference != "" || o.Authorization.Kind != "" ||
		o.Authorization.Repository != "" || o.Authorization.OperatorID != "" || o.Authorization.EvidenceDigest != "" ||
		!o.Authorization.AuthorizedAt.IsZero() || o.Catalog.SchemaVersion != "" || o.Catalog.CatalogID != "" ||
		o.Catalog.Harness != "" || o.Catalog.Role != "" || o.Catalog.AdapterGeneration != "" ||
		o.Catalog.Entries != nil || o.Qualifications != nil
}

// ReplacementProvenance binds the released policy, repository replacement,
// authorization proof, and selected result into one deterministic chain.
type ReplacementProvenance struct {
	SchemaVersion         string `json:"schema_version"`
	DefaultIdentity       string `json:"default_identity"`
	OverrideIdentity      string `json:"override_identity,omitempty"`
	AuthorizationIdentity string `json:"authorization_identity,omitempty"`
	EffectiveIdentity     string `json:"effective_identity"`
	EffectiveDigest       string `json:"effective_digest"`
}

type EffectiveCatalog struct {
	Catalog    ValidatedCatalog      `json:"catalog"`
	Provenance ReplacementProvenance `json:"provenance"`
}

func (e EffectiveCatalog) Entries() []CatalogEntry { return e.Catalog.Entries() }

func selectionDiagnostic(c Catalog, defect, field, detail string) Diagnostic {
	return Diagnostic{Harness: c.Harness, Role: c.Role, CatalogID: c.CatalogID, Defect: defect, Field: field, Detail: detail}
}

// ValidPolicyReference reports whether reference is a non-argument path under
// the repository root. Both config parsing and effective selection use this
// rule so a caller cannot bypass the closed config boundary with a typed value.
func ValidPolicyReference(reference string) bool {
	if reference == "" || filepath.IsAbs(reference) || strings.HasPrefix(reference, "-") || strings.ContainsAny(reference, " \t\r\n\x00") {
		return false
	}
	clean := filepath.Clean(reference)
	return clean != "." && clean != ".." && !strings.HasPrefix(clean, ".."+string(filepath.Separator))
}

func replacementDefects(base DefaultCatalog, override CatalogOverride, authorization AuthorizationPolicy) []Diagnostic {
	c := override.Catalog
	var defects []Diagnostic
	add := func(defect, field, detail string) {
		defects = append(defects, selectionDiagnostic(c, defect, field, detail))
	}
	if override.SchemaVersion != CatalogOverrideSchema {
		add("incompatible_override_schema", "override.schema_version", fmt.Sprintf("got %q, want %q", override.SchemaVersion, CatalogOverrideSchema))
	}
	if override.ResolverCompatibility != ResolverCompatibility || base.ResolverCompatibility != ResolverCompatibility {
		add("incompatible_resolver", "override.resolver_compatibility", fmt.Sprintf("default %q and override %q must both match %q", base.ResolverCompatibility, override.ResolverCompatibility, ResolverCompatibility))
	}
	if override.CatalogReference == "" || override.QualificationReference == "" || len(override.Qualifications) == 0 {
		add("partial_override", "override", "catalog and qualification references plus a complete qualification set are required")
	}
	if (override.CatalogReference != "" && !ValidPolicyReference(override.CatalogReference)) ||
		(override.QualificationReference != "" && !ValidPolicyReference(override.QualificationReference)) {
		add("invalid_policy_reference", "override.references", "catalog and qualification references must be non-argument repository-relative paths")
	}
	if override.Harness == "" || override.Role == "" || c.Harness != override.Harness || c.Role != override.Role ||
		override.Harness != base.Catalog.Harness || override.Role != base.Catalog.Role {
		add("override_target_mismatch", "override.harness_role", "override, replacement catalog, and released catalog must identify the same harness and role")
	}
	if c.AdapterGeneration != base.Catalog.AdapterGeneration {
		add("incompatible_adapter_generation", "override.catalog.adapter_generation", "replacement must target the released adapter generation")
	}
	if override.Authorization.Kind != AuthorizationRepositoryOperator {
		add("unauthorized_replacement", "override.authorization.kind", "only repository-operator authorization may replace native routing policy")
	} else if override.Authorization.Repository == "" || override.Authorization.OperatorID == "" ||
		!digestLike(override.Authorization.EvidenceDigest) || override.Authorization.AuthorizedAt.IsZero() ||
		base.Clock == nil || override.Authorization.AuthorizedAt.After(base.Clock.Now()) {
		add("malformed_authorization", "override.authorization", "operator, repository, evidence digest, and non-future authorization time are required")
	} else if authorization == nil {
		add("authorization_policy_unavailable", "override.authorization", "repository authorization policy is required")
	} else if err := authorization.Authorize(override.Authorization); err != nil {
		add("unauthorized_replacement", "override.authorization", err.Error())
	}

	referenced := make(map[string]bool, len(c.Entries))
	for _, entry := range c.Entries {
		referenced[entry.QualificationID] = true
	}
	for _, qualification := range override.Qualifications {
		if !referenced[qualification.QualificationID] {
			add("injected_qualification", "override.qualifications", fmt.Sprintf("qualification %q is not referenced by the replacement catalog", qualification.QualificationID))
		}
	}
	return defects
}

func strongest(c Catalog) (CatalogEntry, bool) {
	for _, entry := range c.Entries {
		if entry.Strongest {
			return entry, true
		}
	}
	return CatalogEntry{}, false
}

func catalogOverrideIdentity(override CatalogOverride, policyIdentity string) string {
	return digestJSON(struct {
		SchemaVersion          string         `json:"schema_version"`
		ResolverCompatibility  string         `json:"resolver_compatibility"`
		Harness                entity.Harness `json:"harness"`
		Role                   Role           `json:"role"`
		CatalogReference       string         `json:"catalog_ref"`
		QualificationReference string         `json:"qualifications_ref"`
		PolicyIdentity         string         `json:"policy_identity"`
	}{
		SchemaVersion:          override.SchemaVersion,
		ResolverCompatibility:  override.ResolverCompatibility,
		Harness:                override.Harness,
		Role:                   override.Role,
		CatalogReference:       override.CatalogReference,
		QualificationReference: override.QualificationReference,
		PolicyIdentity:         policyIdentity,
	})
}

func canonicalDiagnostics(diagnostics []Diagnostic) []Diagnostic {
	sort.Slice(diagnostics, func(i, j int) bool {
		left, right := diagnostics[i], diagnostics[j]
		for _, pair := range [][2]string{
			{left.Field, right.Field},
			{left.Defect, right.Defect},
			{left.Harness.String(), right.Harness.String()},
			{string(left.Role), string(right.Role)},
			{left.CatalogID, right.CatalogID},
			{left.ProfileID, right.ProfileID},
			{left.QualificationID, right.QualificationID},
			{left.Detail, right.Detail},
		} {
			if pair[0] != pair[1] {
				return pair[0] < pair[1]
			}
		}
		return false
	})
	return diagnostics
}

type selectionClock struct{ at time.Time }

func (c selectionClock) Now() time.Time { return c.at }

// SelectEffectiveCatalog validates one released default or one complete
// replacement. A present but invalid override never returns Default: every
// failure returns the zero EffectiveCatalog, whose candidate set is empty.
func SelectEffectiveCatalog(base DefaultCatalog, override CatalogOverride, authorization AuthorizationPolicy) (EffectiveCatalog, []Diagnostic) {
	// Default and replacement form one policy decision, so both must be checked
	// against the same trusted instant. A live clock is sampled exactly once to
	// prevent an expiry boundary from producing a partially time-skewed choice.
	if base.Clock != nil {
		base.Clock = selectionClock{at: base.Clock.Now()}
	}
	validatedDefault, diagnostics := ValidateCatalog(base.Catalog, base.Qualifications, base.Capability, base.Clock, authorization)
	if len(diagnostics) != 0 {
		return EffectiveCatalog{}, diagnostics
	}
	if base.ResolverCompatibility != ResolverCompatibility {
		return EffectiveCatalog{}, []Diagnostic{selectionDiagnostic(base.Catalog, "incompatible_resolver", "default.resolver_compatibility", fmt.Sprintf("got %q, want %q", base.ResolverCompatibility, ResolverCompatibility))}
	}
	if !override.Present() {
		provenance := ReplacementProvenance{SchemaVersion: ReplacementProvenanceSchema, DefaultIdentity: validatedDefault.Identity, EffectiveIdentity: validatedDefault.Identity}
		provenance.EffectiveDigest = digestJSON(provenance)
		return EffectiveCatalog{Catalog: validatedDefault, Provenance: provenance}, nil
	}

	diagnostics = replacementDefects(base, override, authorization)
	validatedOverride, validationDiagnostics := ValidateCatalog(override.Catalog, override.Qualifications, base.Capability, base.Clock, authorization)
	diagnostics = append(diagnostics, validationDiagnostics...)
	if len(validationDiagnostics) == 0 {
		baseStrongest, baseOK := strongest(validatedDefault.Catalog)
		overrideStrongest, overrideOK := strongest(validatedOverride.Catalog)
		if !baseOK || !overrideOK || !dominates(overrideStrongest.Provides, baseStrongest.Provides) {
			diagnostic := selectionDiagnostic(override.Catalog, "weakening_override", "override.catalog.entries.strongest", "replacement strongest profile must dominate the released strongest profile")
			diagnostic.ProfileID, diagnostic.QualificationID = overrideStrongest.NativeProfileID, overrideStrongest.QualificationID
			diagnostics = append(diagnostics, diagnostic)
		}
	}
	if len(diagnostics) != 0 {
		return EffectiveCatalog{}, canonicalDiagnostics(diagnostics)
	}

	provenance := ReplacementProvenance{
		SchemaVersion: ReplacementProvenanceSchema, DefaultIdentity: validatedDefault.Identity,
		OverrideIdentity: catalogOverrideIdentity(override, validatedOverride.Identity), AuthorizationIdentity: digestJSON(override.Authorization),
		EffectiveIdentity: validatedOverride.Identity,
	}
	provenance.EffectiveDigest = digestJSON(provenance)
	return EffectiveCatalog{Catalog: validatedOverride, Provenance: provenance}, nil
}
