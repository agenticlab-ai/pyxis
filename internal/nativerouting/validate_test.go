package nativerouting

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/executiondemand"
)

var fixtureNow = time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)

type fixedClock struct{ at time.Time }

func (c fixedClock) Now() time.Time { return c.at }

type fixtureAuthorization struct{}

func (fixtureAuthorization) Authorize(proof AuthorizationProof) error {
	if proof.Repository != "agenticlab-ai/pyxis" || proof.OperatorID != "operator-1" {
		return errors.New("proof is not authorized for this repository")
	}
	return nil
}

type fixtureCapability struct{}

func (fixtureCapability) ValidateProfile(entry CatalogEntry) error {
	if !slices.Contains([]string{"native-economy", "native-frontier"}, entry.NativeModel) {
		return NativeProfileDefect{Defect: "unknown_native_value", Field: "entry.native_model", Detail: "model is not in the adapter generation"}
	}
	if !slices.Contains([]string{"low", "maximum"}, entry.NativeEffort) {
		return NativeProfileDefect{Defect: "unknown_native_value", Field: "entry.native_effort", Detail: "effort is not in the adapter generation"}
	}
	for _, control := range entry.Controls {
		if !slices.Contains([]string{"sandbox", "token_budget"}, control.Name) {
			return NativeProfileDefect{Defect: "unknown_native_control", Field: "entry.controls", Detail: "control is not recognized by the adapter"}
		}
	}
	return nil
}

func fixtureDigest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func fixtureClaims(p Provides) []QualificationClaim {
	levels := map[Component]string{
		ComponentCapability: string(p.Capability), ComponentDeliberation: string(p.Deliberation),
		ComponentContext: string(p.Context), ComponentAssurance: string(p.Assurance),
	}
	out := make([]QualificationClaim, 0, 4)
	for _, component := range []Component{ComponentCapability, ComponentDeliberation, ComponentContext, ComponentAssurance} {
		out = append(out, QualificationClaim{
			Component: component, AssertedLevel: levels[component], Method: MethodDecodePinnedEval,
			FixtureDigest: fixtureDigest("fixture-" + string(component)), ResultDigest: fixtureDigest("result-" + string(component)),
			Threshold: "score>=0.80", Result: "score=0.95", Satisfied: true,
			EvidenceReference: "repo:docs/evals/" + string(component), EvaluatorVersion: EvaluatorDecodePinnedV1,
			ResponsibleEvaluator: "evaluator-1", EvidenceOrigin: EvidenceIndependentEvaluator,
		})
	}
	return out
}

func fixturePolicy(role Role) (Catalog, QualificationSet) {
	boolTrue := true
	controls := []NativeControl{{Name: "sandbox", Type: ControlBoolean, BooleanValue: &boolTrue}}
	entries := []CatalogEntry{
		{NativeProfileID: "economy", NativeModel: "native-economy", NativeEffort: "low", Controls: controls,
			Provides: Provides{Capability: executiondemand.CapabilityEconomy, Deliberation: executiondemand.DeliberationLight, Context: executiondemand.ContextStandard, Assurance: executiondemand.AssuranceOrdinary},
			CostRank: 1, QualificationID: "qual-economy", AdapterGeneration: "adapter-gen-7"},
		{NativeProfileID: "frontier", NativeModel: "native-frontier", NativeEffort: "maximum", Controls: controls,
			Provides: Provides{Capability: executiondemand.CapabilityFrontier, Deliberation: executiondemand.DeliberationMaximum, Context: executiondemand.ContextExtended, Assurance: executiondemand.AssuranceCritical},
			CostRank: 2, QualificationID: "qual-frontier", AdapterGeneration: "adapter-gen-7", Strongest: true},
	}
	catalog := Catalog{SchemaVersion: CatalogSchema, CatalogID: "codex-" + string(role) + "-v1", Harness: "codex", Role: role, AdapterGeneration: "adapter-gen-7", Entries: entries}
	set := make(QualificationSet, 0, len(entries))
	for _, entry := range entries {
		q := Qualification{
			SchemaVersion: QualificationSchema, QualificationID: entry.QualificationID, Harness: catalog.Harness, Role: role,
			NativeProfileID: entry.NativeProfileID, NativeModel: entry.NativeModel, NativeEffort: entry.NativeEffort,
			Controls: entry.Controls, AdapterGeneration: catalog.AdapterGeneration, CatalogSchema: CatalogSchema,
			LaunchCaptureDigest: fixtureDigest("launch-" + entry.NativeProfileID), Claims: fixtureClaims(entry.Provides),
			Validity:      Validity{ValidFrom: fixtureNow.Add(-time.Hour), ValidUntil: fixtureNow.Add(time.Hour), Invalidations: []Invalidation{}},
			Authorization: AuthorizationProof{Kind: AuthorizationRepositoryOperator, Repository: "agenticlab-ai/pyxis", OperatorID: "operator-1", EvidenceDigest: fixtureDigest("authorization"), AuthorizedAt: fixtureNow.Add(-2 * time.Hour)},
		}
		set = append(set, q)
	}
	sealQualifications(&catalog, set)
	return catalog, set
}

func sealQualifications(catalog *Catalog, set QualificationSet) {
	for i := range catalog.Entries {
		for _, q := range set {
			if q.QualificationID == catalog.Entries[i].QualificationID {
				catalog.Entries[i].QualificationDigest = QualificationDigest(q)
			}
		}
	}
}

func validateFixture(c Catalog, q QualificationSet) (ValidatedCatalog, []Diagnostic) {
	return ValidateCatalog(c, q, fixtureCapability{}, fixedClock{fixtureNow}, fixtureAuthorization{})
}

func diagnosticWith(defect string, diagnostics []Diagnostic) (Diagnostic, bool) {
	for _, diagnostic := range diagnostics {
		if diagnostic.Defect == defect {
			return diagnostic, true
		}
	}
	return Diagnostic{}, false
}

func requireDefect(t *testing.T, defect string, diagnostics []Diagnostic) Diagnostic {
	t.Helper()
	diagnostic, ok := diagnosticWith(defect, diagnostics)
	if !ok {
		t.Fatalf("missing defect %q in %#v", defect, diagnostics)
	}
	return diagnostic
}

func TestCompleteQualifiedCatalogValidatesForEachRole(t *testing.T) {
	for _, role := range []Role{RoleImplementation, RoleReview} {
		t.Run(string(role), func(t *testing.T) {
			catalog, qualifications := fixturePolicy(role)
			got, diagnostics := validateFixture(catalog, qualifications)
			if len(diagnostics) != 0 {
				t.Fatalf("valid catalog rejected: %v", diagnostics)
			}
			if got.Identity == "" || len(got.Entries()) != len(catalog.Entries) {
				t.Fatalf("validated catalog incomplete: %#v", got)
			}
		})
	}
}

func TestCatalogValidationNegativeCorpusReturnsNoCandidates(t *testing.T) {
	tests := []struct {
		name   string
		defect string
		mutate func(*Catalog, *QualificationSet)
	}{
		{"missing qualification", "missing_qualification", func(_ *Catalog, q *QualificationSet) { *q = (*q)[1:] }},
		{"expired", "qualification_expired", func(c *Catalog, q *QualificationSet) {
			(*q)[0].Validity.ValidUntil = fixtureNow
			sealQualifications(c, *q)
		}},
		{"not yet valid", "qualification_not_yet_valid", func(c *Catalog, q *QualificationSet) {
			(*q)[0].Validity.ValidFrom = fixtureNow.Add(time.Minute)
			(*q)[0].Validity.ValidUntil = fixtureNow.Add(time.Hour)
			sealQualifications(c, *q)
		}},
		{"invalidated", "qualification_invalidated", func(c *Catalog, q *QualificationSet) {
			(*q)[0].Validity.Invalidations = []Invalidation{{EventID: "invalidate-1", At: fixtureNow.Add(-time.Minute), Reason: "launch drift"}}
			sealQualifications(c, *q)
		}},
		{"wrong generation", "wrong_adapter_generation", func(c *Catalog, q *QualificationSet) {
			(*q)[0].AdapterGeneration = "adapter-gen-6"
			sealQualifications(c, *q)
		}},
		{"component insufficient", "component_insufficient", func(c *Catalog, q *QualificationSet) {
			(*q)[1].Claims[0].AssertedLevel = "economy"
			sealQualifications(c, *q)
		}},
		{"missing component", "missing_component_claim", func(c *Catalog, q *QualificationSet) { (*q)[0].Claims = (*q)[0].Claims[1:]; sealQualifications(c, *q) }},
		{"self declared evidence", "self_declared_evidence", func(c *Catalog, q *QualificationSet) {
			(*q)[0].Claims[0].EvidenceOrigin = EvidenceWorker
			sealQualifications(c, *q)
		}},
		{"unbound evidence", "unbound_evidence", func(c *Catalog, q *QualificationSet) {
			(*q)[0].Claims[0].EvidenceReference = "https://provider.example/self-description"
			sealQualifications(c, *q)
		}},
		{"self authorized", "self_declared_authorization", func(c *Catalog, q *QualificationSet) {
			(*q)[0].Authorization.Kind = AuthorizationWorker
			sealQualifications(c, *q)
		}},
		{"repository unauthorized", "unauthorized", func(c *Catalog, q *QualificationSet) {
			(*q)[0].Authorization.Repository = "fork/example"
			sealQualifications(c, *q)
		}},
		{"duplicate qualification identity", "duplicate_qualification_identity", func(c *Catalog, q *QualificationSet) {
			(*q)[1].QualificationID = (*q)[0].QualificationID
			c.Entries[1].QualificationID = c.Entries[0].QualificationID
			sealQualifications(c, *q)
		}},
		{"injected qualification", "injected_qualification", func(_ *Catalog, q *QualificationSet) {
			injected := (*q)[0]
			injected.QualificationID = "qual-injected"
			*q = append(*q, injected)
		}},
		{"duplicate rank", "duplicate_cost_rank", func(c *Catalog, _ *QualificationSet) { c.Entries[1].CostRank = c.Entries[0].CostRank }},
		{"duplicate profile identity", "duplicate_profile_identity", func(c *Catalog, _ *QualificationSet) { c.Entries[1].NativeProfileID = c.Entries[0].NativeProfileID }},
		{"ambiguous strongest", "ambiguous_strongest_profile", func(c *Catalog, _ *QualificationSet) { c.Entries[0].Strongest = true }},
		{"missing launch capture", "missing_launch_capture", func(c *Catalog, q *QualificationSet) { (*q)[0].LaunchCaptureDigest = ""; sealQualifications(c, *q) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			catalog, qualifications := fixturePolicy(RoleImplementation)
			test.mutate(&catalog, &qualifications)
			got, diagnostics := validateFixture(catalog, qualifications)
			if len(got.Entries()) != 0 {
				t.Fatalf("invalid catalog exposed candidates: %#v", got.Entries())
			}
			diagnostic := requireDefect(t, test.defect, diagnostics)
			if diagnostic.Harness != "codex" || diagnostic.Role != RoleImplementation || diagnostic.CatalogID != catalog.CatalogID || diagnostic.Defect == "" {
				t.Fatalf("diagnostic lost catalog context: %#v", diagnostic)
			}
		})
	}
}

func TestClosedAndUnknownValueCorpusNamesExactEntry(t *testing.T) {
	tests := []struct {
		name   string
		defect string
		mutate func(*Catalog, *QualificationSet)
	}{
		{"unknown semantic", "unknown_semantic_value", func(c *Catalog, _ *QualificationSet) { c.Entries[0].Provides.Capability = "marketing-tier" }},
		{"unknown native value", "unknown_native_value", func(c *Catalog, q *QualificationSet) {
			c.Entries[0].NativeModel = "provider-latest"
			(*q)[0].NativeModel = "provider-latest"
			sealQualifications(c, *q)
		}},
		{"unknown native control", "unknown_native_control", func(c *Catalog, q *QualificationSet) {
			control := NativeControl{Name: "raw_switch", Type: ControlString, StringValue: "--upload"}
			c.Entries[0].Controls = append(c.Entries[0].Controls, control)
			(*q)[0].Controls = append((*q)[0].Controls, control)
			sealQualifications(c, *q)
		}},
		{"unknown typed control", "invalid_typed_control", func(c *Catalog, _ *QualificationSet) { c.Entries[0].Controls[0].Type = "object" }},
		{"unknown method", "unknown_qualification_method", func(c *Catalog, q *QualificationSet) {
			(*q)[0].Claims[0].Method = "model_self_report"
			sealQualifications(c, *q)
		}},
		{"unknown evaluator", "unknown_evaluator_version", func(c *Catalog, q *QualificationSet) {
			(*q)[0].Claims[0].EvaluatorVersion = "decode-pinned-eval/v99"
			sealQualifications(c, *q)
		}},
		{"unknown evidence origin", "unknown_evidence_origin", func(c *Catalog, q *QualificationSet) {
			(*q)[0].Claims[0].EvidenceOrigin = "model_card"
			sealQualifications(c, *q)
		}},
		{"unknown authorization kind", "unknown_authorization_kind", func(c *Catalog, q *QualificationSet) {
			(*q)[0].Authorization.Kind = "self_signed"
			sealQualifications(c, *q)
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			catalog, qualifications := fixturePolicy(RoleReview)
			test.mutate(&catalog, &qualifications)
			got, diagnostics := validateFixture(catalog, qualifications)
			if len(got.Entries()) != 0 {
				t.Fatal("invalid policy returned a partial candidate set")
			}
			diagnostic := requireDefect(t, test.defect, diagnostics)
			if diagnostic.ProfileID != "economy" || diagnostic.QualificationID != "qual-economy" || diagnostic.Harness != "codex" || diagnostic.Role != RoleReview {
				t.Fatalf("diagnostic lost exact policy coordinates: %#v", diagnostic)
			}
		})
	}
}

func TestClosedParsingRejectsUnknownKeysAndRawArguments(t *testing.T) {
	catalog, qualifications := fixturePolicy(RoleImplementation)
	catalogJSON, err := json.Marshal(catalog)
	if err != nil {
		t.Fatal(err)
	}
	unknownCatalog := strings.Replace(string(catalogJSON), `"native_model":"native-economy"`, `"native_model":"native-economy","surprise":true`, 1)
	_, diagnostics := ParseCatalog([]byte(unknownCatalog))
	diagnostic := requireDefect(t, "unknown_key", diagnostics)
	if diagnostic.ProfileID != "economy" || diagnostic.QualificationID != "qual-economy" {
		t.Fatalf("catalog parse defect lost entry context: %#v", diagnostic)
	}

	qualificationJSON := make([][]byte, len(qualifications))
	for i, qualification := range qualifications {
		qualificationJSON[i], err = json.Marshal(qualification)
		if err != nil {
			t.Fatal(err)
		}
	}
	qualificationJSON[0] = []byte(strings.Replace(string(qualificationJSON[0]), `"native_model":"native-economy"`, `"native_model":"native-economy","raw_arguments":["--dangerously-upload"]`, 1))
	got, diagnostics := ParseAndValidateCatalog(catalogJSON, qualificationJSON, fixtureCapability{}, fixedClock{fixtureNow}, fixtureAuthorization{})
	if len(got.Entries()) != 0 {
		t.Fatal("raw arguments exposed candidates")
	}
	diagnostic = requireDefect(t, "raw_arguments_forbidden", diagnostics)
	if diagnostic.CatalogID != catalog.CatalogID || diagnostic.Harness != catalog.Harness || diagnostic.Role != catalog.Role || diagnostic.ProfileID != "economy" || diagnostic.QualificationID != "qual-economy" {
		t.Fatalf("qualification parse defect lost unit context: %#v", diagnostic)
	}
}

func TestReorderingProducesSamePolicyIdentityAndValidationResult(t *testing.T) {
	catalog, qualifications := fixturePolicy(RoleReview)
	first, diagnostics := validateFixture(catalog, qualifications)
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}

	reorderedCatalog := catalog
	reorderedCatalog.Entries = slices.Clone(catalog.Entries)
	slices.Reverse(reorderedCatalog.Entries)
	reorderedQualifications := slices.Clone(qualifications)
	slices.Reverse(reorderedQualifications)
	for i := range reorderedQualifications {
		slices.Reverse(reorderedQualifications[i].Claims)
		slices.Reverse(reorderedQualifications[i].Controls)
	}
	second, diagnostics := validateFixture(reorderedCatalog, reorderedQualifications)
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	if first.Identity != second.Identity || !slices.EqualFunc(first.Entries(), second.Entries(), func(a, b CatalogEntry) bool { return a.NativeProfileID == b.NativeProfileID }) {
		t.Fatalf("reordering changed validated policy: %q != %q", first.Identity, second.Identity)
	}

	entriesJSON, _ := json.Marshal(catalog.Entries)
	reorderedFields := `{"entries":` + string(entriesJSON) + `,"adapter_generation":"adapter-gen-7","role":"review","harness":"codex","catalog_id":"codex-review-v1","schema_version":"native-routing-catalog/v1"}`
	parsed, parseDiagnostics := ParseCatalog([]byte(reorderedFields))
	if len(parseDiagnostics) != 0 || PolicyIdentity(parsed, qualifications) != first.Identity {
		t.Fatalf("field reordering changed policy identity: diagnostics=%v identity=%q", parseDiagnostics, PolicyIdentity(parsed, qualifications))
	}
}
