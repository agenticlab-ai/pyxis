package nativerouting

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/executiondemand"
)

type validation struct {
	c Catalog
	d []Diagnostic
}

func (v *validation) add(entry CatalogEntry, defect, field, detail string) {
	v.d = append(v.d, Diagnostic{Harness: v.c.Harness, Role: v.c.Role, CatalogID: v.c.CatalogID, ProfileID: entry.NativeProfileID, QualificationID: entry.QualificationID, Defect: defect, Field: field, Detail: detail})
}

func oneOf[T ~string](value T, values ...T) bool {
	return slices.Contains(values, value)
}

// ReleasedCatalogAuthorization admits only the repository-operator marker
// embedded in released adapter qualifications. Those qualifications belong to
// the adapter policy and are reusable by invocation repositories.
type ReleasedCatalogAuthorization struct{}

func (ReleasedCatalogAuthorization) Authorize(proof AuthorizationProof) error {
	if proof.Kind != AuthorizationRepositoryOperator || strings.TrimSpace(proof.Repository) == "" || strings.TrimSpace(proof.OperatorID) == "" {
		return errors.New("catalog qualification lacks repository-operator authorization")
	}
	return nil
}

func digestLike(value string) bool {
	value = strings.TrimPrefix(value, "sha256:")
	if len(value) != 64 {
		return false
	}
	for _, r := range value {
		if !strings.ContainsRune("0123456789abcdef", r) {
			return false
		}
	}
	return true
}

func validateControl(c NativeControl) error {
	if c.Name == "" {
		return fmt.Errorf("control name is required")
	}
	switch c.Type {
	case ControlString:
		if c.IntegerValue != nil || c.BooleanValue != nil {
			return fmt.Errorf("string control %q carries another value type", c.Name)
		}
	case ControlInteger:
		if c.IntegerValue == nil || c.StringValue != "" || c.BooleanValue != nil {
			return fmt.Errorf("integer control %q must carry only integer_value", c.Name)
		}
	case ControlBoolean:
		if c.BooleanValue == nil || c.StringValue != "" || c.IntegerValue != nil {
			return fmt.Errorf("boolean control %q must carry only boolean_value", c.Name)
		}
	default:
		return fmt.Errorf("control %q has unknown type %q", c.Name, c.Type)
	}
	return nil
}

func validateControls(controls []NativeControl) error {
	seen := map[string]bool{}
	for _, control := range controls {
		if seen[control.Name] {
			return fmt.Errorf("duplicate control %q", control.Name)
		}
		seen[control.Name] = true
		if err := validateControl(control); err != nil {
			return err
		}
	}
	return nil
}

func validProvides(p Provides) bool {
	return oneOf(p.Capability, executiondemand.CapabilityEconomy, executiondemand.CapabilityStandard, executiondemand.CapabilityStrong, executiondemand.CapabilityFrontier) &&
		oneOf(p.Deliberation, executiondemand.DeliberationLight, executiondemand.DeliberationStandard, executiondemand.DeliberationDeep, executiondemand.DeliberationMaximum) &&
		oneOf(p.Context, executiondemand.ContextStandard, executiondemand.ContextLarge, executiondemand.ContextExtended) &&
		oneOf(p.Assurance, executiondemand.AssuranceOrdinary, executiondemand.AssuranceElevated, executiondemand.AssuranceCritical)
}

func levelIndex(component Component, level string) int {
	var values []string
	switch component {
	case ComponentCapability:
		values = []string{"economy", "standard", "strong", "frontier"}
	case ComponentDeliberation:
		values = []string{"light", "standard", "deep", "maximum"}
	case ComponentContext:
		values = []string{"standard", "large", "extended"}
	case ComponentAssurance:
		values = []string{"ordinary", "elevated", "critical"}
	default:
		return -1
	}
	for i, value := range values {
		if level == value {
			return i
		}
	}
	return -1
}

func providedLevel(p Provides, component Component) string {
	switch component {
	case ComponentCapability:
		return string(p.Capability)
	case ComponentDeliberation:
		return string(p.Deliberation)
	case ComponentContext:
		return string(p.Context)
	case ComponentAssurance:
		return string(p.Assurance)
	default:
		return ""
	}
}

func dominates(a, b Provides) bool {
	for _, component := range []Component{ComponentCapability, ComponentDeliberation, ComponentContext, ComponentAssurance} {
		if levelIndex(component, providedLevel(a, component)) < levelIndex(component, providedLevel(b, component)) {
			return false
		}
	}
	return true
}

func expectedEvaluator(method QualificationMethod) EvaluatorVersion {
	switch method {
	case MethodDeterministicConformance:
		return EvaluatorDeterministicV1
	case MethodDecodePinnedEval:
		return EvaluatorDecodePinnedV1
	case MethodOperatorInspection:
		return EvaluatorInspectionV1
	default:
		return ""
	}
}

func (v *validation) validateClaim(entry CatalogEntry, claim QualificationClaim) {
	field := "qualification.claims." + string(claim.Component)
	if levelIndex(claim.Component, claim.AssertedLevel) < 0 {
		v.add(entry, "unknown_component_level", field+".asserted_level", fmt.Sprintf("unknown %s level %q", claim.Component, claim.AssertedLevel))
	} else if levelIndex(claim.Component, claim.AssertedLevel) < levelIndex(claim.Component, providedLevel(entry.Provides, claim.Component)) || !claim.Satisfied {
		v.add(entry, "component_insufficient", field, "claim does not meet the catalog provision")
	}
	wantEvaluator := expectedEvaluator(claim.Method)
	if wantEvaluator == "" {
		v.add(entry, "unknown_qualification_method", field+".method", fmt.Sprintf("unknown method %q", claim.Method))
	} else if claim.EvaluatorVersion != wantEvaluator {
		v.add(entry, "unknown_evaluator_version", field+".evaluator_version", fmt.Sprintf("method %q requires %q", claim.Method, wantEvaluator))
	}
	if !digestLike(claim.FixtureDigest) || !digestLike(claim.ResultDigest) {
		v.add(entry, "malformed_evidence_digest", field, "fixture_digest and result_digest must be lowercase SHA-256 digests")
	}
	if claim.Threshold == "" || claim.Result == "" || claim.ResponsibleEvaluator == "" {
		v.add(entry, "incomplete_claim", field, "threshold, result, and responsible_evaluator are required")
	}
	if claim.EvidenceOrigin == EvidenceWorker || claim.EvidenceOrigin == EvidenceProvider {
		v.add(entry, "self_declared_evidence", field+".evidence_origin", "worker and provider declarations are not qualification evidence")
	} else if !oneOf(claim.EvidenceOrigin, EvidenceIndependentEvaluator, EvidenceOperatorRepository) {
		v.add(entry, "unknown_evidence_origin", field+".evidence_origin", fmt.Sprintf("unknown origin %q", claim.EvidenceOrigin))
	}
	if !strings.HasPrefix(claim.EvidenceReference, "repo:") && !strings.HasPrefix(claim.EvidenceReference, "event:") {
		v.add(entry, "unbound_evidence", field+".evidence_reference", "evidence must be a repository or event reference, not free text or a URL")
	}
	if claim.Method == MethodOperatorInspection && claim.RubricResult == "" {
		v.add(entry, "missing_rubric_result", field+".rubric_result", "operator inspection requires a named rubric result")
	}
}

func (v *validation) validateQualification(entry CatalogEntry, q Qualification, now time.Time, authorization AuthorizationPolicy) {
	if q.SchemaVersion != QualificationSchema || q.CatalogSchema != CatalogSchema {
		v.add(entry, "unknown_schema_version", "qualification.schema_version", "qualification and catalog schema versions must be v1")
	}
	if q.QualificationID != entry.QualificationID || q.Harness != v.c.Harness || q.Role != v.c.Role || q.NativeProfileID != entry.NativeProfileID {
		v.add(entry, "qualification_identity_mismatch", "qualification", "qualification does not bind the exact harness, role, profile, and identity")
	}
	if q.NativeModel != entry.NativeModel || q.NativeEffort != entry.NativeEffort || !reflect.DeepEqual(canonicalControls(q.Controls), canonicalControls(entry.Controls)) {
		v.add(entry, "qualification_native_mismatch", "qualification.native_profile", "qualified native model, effort, or controls differ from the catalog entry")
	}
	if q.AdapterGeneration == "" || q.AdapterGeneration != v.c.AdapterGeneration || q.AdapterGeneration != entry.AdapterGeneration {
		v.add(entry, "wrong_adapter_generation", "qualification.adapter_generation", "qualification must bind the catalog's exact adapter generation")
	}
	if !digestLike(q.LaunchCaptureDigest) {
		v.add(entry, "missing_launch_capture", "qualification.launch_capture_digest", "a lowercase SHA-256 launch capture digest is required")
	}
	if entry.QualificationDigest != QualificationDigest(q) {
		v.add(entry, "qualification_digest_mismatch", "entry.qualification_digest", "entry does not bind the canonical qualification artifact")
	}
	claims := map[Component]QualificationClaim{}
	for _, claim := range q.Claims {
		if _, duplicate := claims[claim.Component]; duplicate {
			v.add(entry, "duplicate_component_claim", "qualification.claims", fmt.Sprintf("duplicate %q claim", claim.Component))
			continue
		}
		claims[claim.Component] = claim
		v.validateClaim(entry, claim)
	}
	for _, component := range []Component{ComponentCapability, ComponentDeliberation, ComponentContext, ComponentAssurance} {
		if _, ok := claims[component]; !ok {
			v.add(entry, "missing_component_claim", "qualification.claims", fmt.Sprintf("missing %q claim", component))
		}
	}
	if q.Validity.ValidFrom.IsZero() || q.Validity.ValidUntil.IsZero() || !q.Validity.ValidFrom.Before(q.Validity.ValidUntil) {
		v.add(entry, "invalid_validity_window", "qualification.validity", "valid_from must precede required valid_until")
	} else if now.Before(q.Validity.ValidFrom) {
		v.add(entry, "qualification_not_yet_valid", "qualification.validity.valid_from", "qualification is not current at trusted time")
	} else if !now.Before(q.Validity.ValidUntil) {
		v.add(entry, "qualification_expired", "qualification.validity.valid_until", "qualification has expired at trusted time")
	}
	if len(q.Validity.Invalidations) != 0 {
		v.add(entry, "qualification_invalidated", "qualification.validity.invalidations", "an additive invalidation event is present")
	}
	if !oneOf(q.Authorization.Kind, AuthorizationRepositoryOperator, AuthorizationContributor, AuthorizationWorker) {
		v.add(entry, "unknown_authorization_kind", "qualification.authorization.kind", fmt.Sprintf("unknown authorization kind %q", q.Authorization.Kind))
	} else if q.Authorization.Kind != AuthorizationRepositoryOperator {
		v.add(entry, "self_declared_authorization", "qualification.authorization.kind", "only repository-operator authorization is effective")
	} else if q.Authorization.Repository == "" || q.Authorization.OperatorID == "" || !digestLike(q.Authorization.EvidenceDigest) || q.Authorization.AuthorizedAt.IsZero() || q.Authorization.AuthorizedAt.After(now) {
		v.add(entry, "malformed_authorization", "qualification.authorization", "operator, repository, evidence digest, and non-future authorization time are required")
	} else if authorization == nil {
		v.add(entry, "authorization_policy_unavailable", "qualification.authorization", "repository authorization policy is required")
	} else if err := authorization.Authorize(q.Authorization); err != nil {
		v.add(entry, "unauthorized", "qualification.authorization", err.Error())
	}
}

// ValidateCatalog admits the complete harness-role policy or returns only
// diagnostics. Any defect discards every candidate.
func ValidateCatalog(c Catalog, qualifications QualificationSet, capability NativeCapability, clock TrustedClock, authorization AuthorizationPolicy) (ValidatedCatalog, []Diagnostic) {
	c = canonicalCatalog(c)
	v := validation{c: c}
	if c.SchemaVersion != CatalogSchema {
		v.add(CatalogEntry{}, "unknown_schema_version", "catalog.schema_version", fmt.Sprintf("got %q, want %q", c.SchemaVersion, CatalogSchema))
	}
	if c.CatalogID == "" || c.Harness == "" || c.AdapterGeneration == "" || !oneOf(c.Role, RoleImplementation, RoleReview) {
		v.add(CatalogEntry{}, "incomplete_catalog_identity", "catalog", "catalog ID, harness, closed role, and adapter generation are required")
	}
	if len(c.Entries) == 0 {
		v.add(CatalogEntry{}, "empty_catalog", "catalog.entries", "at least one native profile is required")
	}
	if capability == nil {
		v.add(CatalogEntry{}, "native_capability_unavailable", "catalog.entries", "adapter native capability is required")
	}
	var now time.Time
	if clock == nil {
		v.add(CatalogEntry{}, "trusted_clock_unavailable", "qualification.validity", "trusted clock is required")
	} else {
		now = clock.Now()
	}

	sortedQualifications := append(QualificationSet(nil), qualifications...)
	sort.Slice(sortedQualifications, func(i, j int) bool {
		if sortedQualifications[i].QualificationID != sortedQualifications[j].QualificationID {
			return sortedQualifications[i].QualificationID < sortedQualifications[j].QualificationID
		}
		return QualificationDigest(sortedQualifications[i]) < QualificationDigest(sortedQualifications[j])
	})
	byQualification := map[string]Qualification{}
	for _, q := range sortedQualifications {
		q = canonicalQualification(q)
		if _, duplicate := byQualification[q.QualificationID]; duplicate || q.QualificationID == "" {
			entry := CatalogEntry{NativeProfileID: q.NativeProfileID, QualificationID: q.QualificationID}
			v.add(entry, "duplicate_qualification_identity", "qualification.qualification_id", "qualification identity is empty or duplicated")
			continue
		}
		byQualification[q.QualificationID] = q
	}
	referencedQualifications := make(map[string]bool, len(c.Entries))
	for _, entry := range c.Entries {
		referencedQualifications[entry.QualificationID] = true
	}
	for _, q := range sortedQualifications {
		if q.QualificationID != "" && !referencedQualifications[q.QualificationID] {
			v.add(CatalogEntry{NativeProfileID: q.NativeProfileID, QualificationID: q.QualificationID}, "injected_qualification", "qualifications", "qualification is not referenced by any catalog entry")
		}
	}

	profileIDs, ranks := map[string]bool{}, map[int]bool{}
	for _, entry := range c.Entries {
		if entry.NativeProfileID == "" || profileIDs[entry.NativeProfileID] {
			v.add(entry, "duplicate_profile_identity", "entry.native_profile_id", "profile identity is empty or duplicated")
		}
		profileIDs[entry.NativeProfileID] = true
		if entry.CostRank <= 0 || ranks[entry.CostRank] {
			v.add(entry, "duplicate_cost_rank", "entry.cost_rank", "cost rank must be positive and unique")
		}
		ranks[entry.CostRank] = true
		if entry.NativeModel == "" || entry.NativeEffort == "" || entry.QualificationID == "" || !digestLike(entry.QualificationDigest) {
			v.add(entry, "incomplete_entry", "entry", "native model, effort, qualification identity, and digest are required")
		}
		if entry.AdapterGeneration != c.AdapterGeneration {
			v.add(entry, "wrong_adapter_generation", "entry.adapter_generation", "entry must bind the catalog adapter generation")
		}
		if !validProvides(entry.Provides) {
			v.add(entry, "unknown_semantic_value", "entry.provides", "all four provisions must use closed execution-demand values")
		}
		if err := validateControls(entry.Controls); err != nil {
			v.add(entry, "invalid_typed_control", "entry.controls", err.Error())
		}
		if capability != nil {
			if err := capability.ValidateProfile(entry); err != nil {
				var nativeDefect NativeProfileDefect
				if errors.As(err, &nativeDefect) {
					v.add(entry, nativeDefect.Defect, nativeDefect.Field, nativeDefect.Detail)
				} else {
					v.add(entry, "unknown_native_profile", "entry.native_profile", err.Error())
				}
			}
		}
		q, ok := byQualification[entry.QualificationID]
		if !ok {
			v.add(entry, "missing_qualification", "entry.qualification_id", "referenced qualification is absent")
		} else if clock != nil {
			v.validateQualification(entry, q, now, authorization)
		}
	}

	marked, ambiguousEntry := -1, CatalogEntry{}
	for i, entry := range c.Entries {
		if entry.Strongest {
			if marked != -1 {
				ambiguousEntry = entry
				marked = -2
				break
			}
			marked = i
		}
	}
	if marked < 0 {
		if ambiguousEntry.NativeProfileID == "" && len(c.Entries) != 0 {
			ambiguousEntry = c.Entries[0]
		}
		v.add(ambiguousEntry, "ambiguous_strongest_profile", "catalog.entries.strongest", "exactly one strongest profile must be marked")
	} else {
		for i, entry := range c.Entries {
			if i != marked && (!dominates(c.Entries[marked].Provides, entry.Provides) || c.Entries[marked].Provides == entry.Provides) {
				v.add(c.Entries[marked], "ambiguous_strongest_profile", "entry.strongest", "marked profile is not the unique componentwise greatest provision")
				break
			}
		}
	}

	if len(v.d) != 0 {
		return ValidatedCatalog{}, v.d
	}
	return ValidatedCatalog{Catalog: c, Identity: PolicyIdentity(c, qualifications)}, nil
}
