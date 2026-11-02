package conformancetest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/entity"
	"github.com/agenticlab-ai/pyxis/internal/executiondemand"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/nativerouting"
)

var matrixTime = time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)

type fakeRegistry struct {
	backends []string
	owners   map[string]port.AgentRuntime
}

func (r fakeRegistry) Harnesses() []entity.Harness {
	out := make([]entity.Harness, len(r.backends))
	for i, name := range r.backends {
		out[i] = entity.Harness(name)
	}
	return out
}
func (r fakeRegistry) Resolve(name entity.Harness) (port.AgentRuntime, error) {
	owner, ok := r.owners[name.String()]
	if !ok {
		return nil, errors.New("not registered")
	}
	return owner, nil
}

type fakeClock struct{ at time.Time }

func (c fakeClock) Now() time.Time { return c.at }

type fakeAuthorization struct{}

func (fakeAuthorization) Authorize(proof nativerouting.AuthorizationProof) error {
	if proof.Kind != nativerouting.AuthorizationRepositoryOperator || proof.Repository != "agenticlab-ai/pyxis" || proof.OperatorID == "" {
		return errors.New("not operator-authorized")
	}
	return nil
}

type fakeOwner struct {
	harness entity.Harness
	expired bool
}

type fakeHandle struct{}

func (fakeHandle) RuntimeHandle() {}

func (*fakeOwner) Launch(context.Context, port.LaunchSpec) (port.Handle, error) {
	return fakeHandle{}, nil
}
func (*fakeOwner) Await(context.Context, port.Handle) (port.RunResult, error) {
	return port.RunResult{}, nil
}
func (*fakeOwner) Kill(context.Context, port.Handle) error { return nil }
func (*fakeOwner) AuthCheck(context.Context) error         { return nil }

func (o *fakeOwner) Catalog(role nativerouting.Role) (nativerouting.Catalog, nativerouting.QualificationSet, error) {
	if role != nativerouting.RoleImplementation && role != nativerouting.RoleReview {
		return nativerouting.Catalog{}, nil, errors.New("unknown role")
	}
	entry := nativerouting.CatalogEntry{
		NativeProfileID: "base-" + string(role), NativeModel: "fixture-model", NativeEffort: "fixture-effort",
		Controls: []nativerouting.NativeControl{}, Provides: nativerouting.Provides{
			Capability: executiondemand.CapabilityEconomy, Deliberation: executiondemand.DeliberationLight,
			Context: executiondemand.ContextStandard, Assurance: executiondemand.AssuranceOrdinary,
		},
		CostRank: 1, QualificationID: "qualification-" + string(role), AdapterGeneration: "fixture-adapter/v1", Strongest: true,
	}
	qualification := fakeQualification(o.harness, role, entry, o.expired)
	entry.QualificationDigest = nativerouting.QualificationDigest(qualification)
	return nativerouting.Catalog{
		SchemaVersion: nativerouting.CatalogSchema, CatalogID: "fixture-" + string(role), Harness: o.harness,
		Role: role, AdapterGeneration: "fixture-adapter/v1", Entries: []nativerouting.CatalogEntry{entry},
	}, nativerouting.QualificationSet{qualification}, nil
}

func fakeQualification(harness entity.Harness, role nativerouting.Role, entry nativerouting.CatalogEntry, expired bool) nativerouting.Qualification {
	levels := map[nativerouting.Component]string{
		nativerouting.ComponentCapability: string(entry.Provides.Capability), nativerouting.ComponentDeliberation: string(entry.Provides.Deliberation),
		nativerouting.ComponentContext: string(entry.Provides.Context), nativerouting.ComponentAssurance: string(entry.Provides.Assurance),
	}
	claims := make([]nativerouting.QualificationClaim, 0, 4)
	for _, component := range []nativerouting.Component{nativerouting.ComponentAssurance, nativerouting.ComponentContext, nativerouting.ComponentDeliberation, nativerouting.ComponentCapability} {
		claims = append(claims, nativerouting.QualificationClaim{
			Component: component, AssertedLevel: levels[component], Method: nativerouting.MethodDecodePinnedEval,
			FixtureDigest: testDigest("fixture/" + string(component)), ResultDigest: testDigest("result/" + string(component)),
			Threshold: "all cases pass", Result: "all cases passed", Satisfied: true,
			EvidenceReference: "repo:fixture/evidence#" + string(component), EvaluatorVersion: nativerouting.EvaluatorDecodePinnedV1,
			ResponsibleEvaluator: "fixture-independent-evaluator", EvidenceOrigin: nativerouting.EvidenceIndependentEvaluator,
		})
	}
	validUntil := matrixTime.Add(time.Hour)
	if expired {
		validUntil = matrixTime.Add(-time.Hour)
	}
	return nativerouting.Qualification{
		SchemaVersion: nativerouting.QualificationSchema, QualificationID: entry.QualificationID,
		Harness: harness, Role: role, NativeProfileID: entry.NativeProfileID, NativeModel: entry.NativeModel,
		NativeEffort: entry.NativeEffort, Controls: []nativerouting.NativeControl{}, AdapterGeneration: "fixture-adapter/v1",
		CatalogSchema: nativerouting.CatalogSchema, LaunchCaptureDigest: testDigest("launch/" + string(role)), Claims: claims,
		Validity: nativerouting.Validity{ValidFrom: matrixTime.Add(-time.Hour), ValidUntil: validUntil, Invalidations: []nativerouting.Invalidation{}},
		Authorization: nativerouting.AuthorizationProof{
			Kind: nativerouting.AuthorizationRepositoryOperator, Repository: "agenticlab-ai/pyxis", OperatorID: "fixture-operator",
			EvidenceDigest: testDigest("authorization"), AuthorizedAt: matrixTime.Add(-time.Hour),
		},
	}
}

func testDigest(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

func (o *fakeOwner) ValidateProfile(entry nativerouting.CatalogEntry) error {
	if entry.NativeModel != "fixture-model" || entry.NativeEffort != "fixture-effort" || len(entry.Controls) != 0 {
		return errors.New("unknown native profile")
	}
	return nil
}

func (o *fakeOwner) Build(selected nativerouting.SelectedProfile) (port.NativeLaunchSpec, error) {
	if selected.Harness != o.harness || selected.AdapterGeneration != "fixture-adapter/v1" {
		return port.NativeLaunchSpec{}, errors.New("foreign selection")
	}
	return port.NativeLaunchSpec{NativeProfileID: selected.NativeProfileID, NativeModel: selected.NativeModel, NativeEffort: selected.NativeEffort, Controls: slices.Clone(selected.Controls)}, nil
}

func (*fakeOwner) CaptureNativeLaunch(spec port.NativeLaunchSpec) (port.LaunchCapture, error) {
	return port.LaunchCapture{NativeModel: spec.NativeModel, NativeEffort: spec.NativeEffort, Controls: slices.Clone(spec.Controls)}, nil
}

func emptyPolicy() executiondemand.DerivationPolicy {
	return executiondemand.DerivationPolicy{Schema: "execution-demand-policy/v1", Version: "fixture-v1", Floors: []executiondemand.Floor{}, Promotions: []executiondemand.Promotion{}}
}

func TestGenerateDomainCrossesLiveRegistryRolesAndPolicyDeterministically(t *testing.T) {
	forward := fakeRegistry{backends: []string{"zeta", "alpha"}}
	reverse := fakeRegistry{backends: []string{"alpha", "zeta"}}
	roles := Roles{nativerouting.RoleReview, nativerouting.RoleImplementation}
	first, err := GenerateDomain(executiondemand.DefaultPolicy, forward, roles)
	if err != nil {
		t.Fatal(err)
	}
	second, err := GenerateDomain(executiondemand.DefaultPolicy, reverse, slices.Clone(roles))
	if err != nil {
		t.Fatal(err)
	}
	firstJSON, err := first.JSON()
	if err != nil {
		t.Fatal(err)
	}
	secondJSON, err := second.JSON()
	if err != nil {
		t.Fatal(err)
	}
	if string(firstJSON) != string(secondJSON) {
		t.Fatalf("registry order changed domain bytes\n%s\n%s", firstJSON, secondJSON)
	}
	if len(first.Cells) != len(first.Harnesses)*2*len(first.Profiles) {
		t.Fatalf("cell count = %d, want %d", len(first.Cells), len(first.Harnesses)*2*len(first.Profiles))
	}
	seen := map[string]bool{}
	for _, cell := range first.Cells {
		if seen[cell.CellID] {
			t.Fatalf("duplicate generated cell %q", cell.CellID)
		}
		seen[cell.CellID] = true
	}
	if _, err := GenerateDomain(emptyPolicy(), forward, Roles{nativerouting.RoleImplementation}); err == nil {
		t.Fatal("domain accepted an omitted review role")
	}
}

func TestDomainBoundaryFailures(t *testing.T) {
	roles := Roles{nativerouting.RoleImplementation, nativerouting.RoleReview}
	if _, err := GenerateDomain(emptyPolicy(), nil, roles); err == nil {
		t.Fatal("GenerateDomain accepted a nil registry")
	}
	if _, err := GenerateDomain(emptyPolicy(), fakeRegistry{}, roles); err == nil {
		t.Fatal("GenerateDomain accepted an empty registry")
	}
	if _, err := canonicalStrings([]string{""}); err == nil {
		t.Fatal("canonicalStrings accepted an empty identity")
	}
	if _, err := canonicalStrings([]string{"same", "same"}); err == nil {
		t.Fatal("canonicalStrings accepted a duplicate identity")
	}

	registry := fakeRegistry{backends: []string{"fixture"}}
	valid, err := GenerateDomain(emptyPolicy(), registry, roles)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateDomain(Domain{}); err == nil {
		t.Fatal("ValidateDomain accepted an incomplete domain")
	}
	changed := valid
	changed.DomainIdentity = "sha256:wrong"
	if err := ValidateDomain(changed); err == nil {
		t.Fatal("ValidateDomain accepted an identity that does not bind content")
	}
	changed = valid
	changed.Harnesses = []string{"fixture", "fixture"}
	changed.DomainIdentity = contentIdentity(domainWithoutIdentity(changed))
	if err := ValidateDomain(changed); err == nil {
		t.Fatal("ValidateDomain accepted a non-unique harness domain")
	}
	changed = valid
	changed.Roles = Roles{nativerouting.RoleImplementation, nativerouting.RoleImplementation}
	changed.DomainIdentity = contentIdentity(domainWithoutIdentity(changed))
	if err := ValidateDomain(changed); err == nil {
		t.Fatal("ValidateDomain accepted an incomplete role domain")
	}
}

func TestCheckProducesClosedResolvedAndUnsupportedCellsAndReviewReport(t *testing.T) {
	owner := &fakeOwner{harness: "fixture"}
	registry := fakeRegistry{backends: []string{"fixture"}, owners: map[string]port.AgentRuntime{"fixture": owner}}
	domain, err := GenerateDomain(emptyPolicy(), registry, Roles{nativerouting.RoleImplementation, nativerouting.RoleReview})
	if err != nil {
		t.Fatal(err)
	}
	matrix, report, err := Check(domain, registry, fakeClock{matrixTime}, fakeAuthorization{})
	if err != nil {
		t.Fatal(err)
	}
	resolved, unsupported := 0, 0
	for _, result := range matrix.Results {
		if result.Event() != (RoutingConformanceChecked{Harness: result.Harness, Role: result.Role, SemanticProfileID: result.SemanticProfileID, Expected: result.Expected, Result: result.Result, EvidenceReference: result.EvidenceReference}) {
			t.Fatalf("event payload did not close over result: %#v", result)
		}
		switch result.Expected {
		case ExpectedResolved:
			resolved++
			if result.QualificationID == "" || result.QualificationDigest == "" || result.AdapterGeneration != "fixture-adapter/v1" {
				t.Fatalf("resolved result omitted qualification evidence: %#v", result)
			}
		case ExpectedUnsupported:
			unsupported++
			if len(result.UnmetComponents) != 1 || result.UnmetComponents[0].Component != nativerouting.ComponentAssurance {
				t.Fatalf("unsupported result did not name unmet assurance: %#v", result)
			}
		}
	}
	if resolved != 2 || unsupported != 4 || len(report.Rows) != resolved {
		t.Fatalf("resolved=%d unsupported=%d report rows=%d", resolved, unsupported, len(report.Rows))
	}
	qualificationIDs := map[string]bool{}
	for _, qualification := range report.Qualifications {
		qualificationIDs[qualification.QualificationID+"\x00"+qualification.QualificationDigest] = true
		if len(qualification.Claims) != 4 || qualification.Validity.ValidUntil.IsZero() || qualification.AdapterGeneration == "" {
			t.Fatalf("report qualification is incomplete: %#v", qualification)
		}
		for _, claim := range qualification.Claims {
			if claim.Method == "" || claim.FixtureDigest == "" || claim.ResultDigest == "" || claim.Threshold == "" || claim.Result == "" || claim.ResponsibleEvaluator == "" {
				t.Fatalf("report claim is incomplete: %#v", claim)
			}
		}
	}
	for _, row := range report.Rows {
		if row.AdapterGeneration == "" || !qualificationIDs[row.QualificationID+"\x00"+row.QualificationDigest] {
			t.Fatalf("report row is incomplete: %#v", row)
		}
	}
	reportJSON, err := report.JSON()
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"prompt", "credential", "raw_launch", "launch_argument", "source_code"} {
		if strings.Contains(string(reportJSON), forbidden) {
			t.Fatalf("review report exposed forbidden field %q: %s", forbidden, reportJSON)
		}
	}
}

func TestAssembleMatrixRefusesOmissionsDuplicatesAndForeignResults(t *testing.T) {
	owner := &fakeOwner{harness: "fixture"}
	registry := fakeRegistry{backends: []string{"fixture"}, owners: map[string]port.AgentRuntime{"fixture": owner}}
	domain, _ := GenerateDomain(emptyPolicy(), registry, Roles{nativerouting.RoleReview, nativerouting.RoleImplementation})
	matrix, _, err := Check(domain, registry, fakeClock{matrixTime}, fakeAuthorization{})
	if err != nil {
		t.Fatal(err)
	}
	reordered := slices.Clone(matrix.Results)
	slices.Reverse(reordered)
	reassembled, err := AssembleMatrix(domain, reordered)
	if err != nil {
		t.Fatal(err)
	}
	firstJSON, err := matrix.JSON()
	if err != nil {
		t.Fatal(err)
	}
	secondJSON, err := reassembled.JSON()
	if err != nil {
		t.Fatal(err)
	}
	if string(firstJSON) != string(secondJSON) {
		t.Fatal("fixture result order changed matrix bytes")
	}
	if _, err := AssembleMatrix(domain, matrix.Results[:len(matrix.Results)-1]); err == nil {
		t.Fatal("matrix accepted an omitted result")
	}
	duplicated := append(slices.Clone(matrix.Results), matrix.Results[0])
	if _, err := AssembleMatrix(domain, duplicated); err == nil {
		t.Fatal("matrix accepted a duplicate result")
	}
	foreign := slices.Clone(matrix.Results)
	changed := foreign[0]
	changed.CellID = "foreign"
	foreign[0] = changed
	if _, err := AssembleMatrix(domain, foreign); err == nil {
		t.Fatal("matrix accepted a foreign result")
	}
}

func TestCheckRefusesStaleQualificationAndUnregisteredCatalogOwner(t *testing.T) {
	for name, owner := range map[string]*fakeOwner{
		"stale qualification":        {harness: "fixture", expired: true},
		"unregistered catalog owner": {harness: "foreign"},
	} {
		t.Run(name, func(t *testing.T) {
			registry := fakeRegistry{backends: []string{"fixture"}, owners: map[string]port.AgentRuntime{"fixture": owner}}
			domain, err := GenerateDomain(emptyPolicy(), registry, Roles{nativerouting.RoleImplementation, nativerouting.RoleReview})
			if err != nil {
				t.Fatal(err)
			}
			if _, _, err := Check(domain, registry, fakeClock{matrixTime}, fakeAuthorization{}); err == nil {
				t.Fatal("invalid live qualification/catalog owner was accepted")
			}
		})
	}
}

func TestCheckAndCellResultBoundaryFailures(t *testing.T) {
	owner := &fakeOwner{harness: "fixture"}
	registry := fakeRegistry{backends: []string{"fixture"}, owners: map[string]port.AgentRuntime{"fixture": owner}}
	domain, err := GenerateDomain(emptyPolicy(), registry, Roles{nativerouting.RoleImplementation, nativerouting.RoleReview})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := Check(Domain{}, registry, fakeClock{matrixTime}, fakeAuthorization{}); err == nil {
		t.Fatal("Check accepted an invalid domain")
	}
	if _, _, err := Check(domain, nil, fakeClock{matrixTime}, fakeAuthorization{}); err == nil {
		t.Fatal("Check accepted a nil registry")
	}
	if _, _, err := Check(domain, fakeRegistry{backends: []string{"other"}}, fakeClock{matrixTime}, fakeAuthorization{}); err == nil {
		t.Fatal("Check accepted a registry outside the frozen domain")
	}
	if _, _, err := Check(domain, fakeRegistry{backends: []string{"fixture"}}, fakeClock{matrixTime}, fakeAuthorization{}); err == nil {
		t.Fatal("Check hid a registered harness resolution failure")
	}
	if _, err := AssembleMatrix(Domain{}, nil); err == nil {
		t.Fatal("AssembleMatrix accepted an invalid domain")
	}

	matrix, _, err := Check(domain, registry, fakeClock{matrixTime}, fakeAuthorization{})
	if err != nil {
		t.Fatal(err)
	}
	foreign := append(slices.Clone(matrix.Results), matrix.Results[0])
	foreign[len(foreign)-1].CellID = "foreign"
	if _, err := AssembleMatrix(domain, foreign); err == nil {
		t.Fatal("AssembleMatrix accepted a foreign result in addition to the complete domain")
	}
	cell := domain.Cells[0]
	baseline := matrix.Results[0]
	mutations := []func(*CellResult){
		func(r *CellResult) { r.CellID = "foreign" },
		func(r *CellResult) { r.Result = "" },
		func(r *CellResult) {
			r.Expected, r.ResolutionClass = ExpectedResolved, nativerouting.ResolutionUnsupported
		},
		func(r *CellResult) { r.Expected, r.QualificationID = ExpectedResolved, "" },
		func(r *CellResult) {
			r.Expected, r.ResolutionClass = ExpectedUnsupported, nativerouting.ResolutionExact
		},
		func(r *CellResult) { r.Expected = Expected("unknown") },
	}
	for i, mutate := range mutations {
		result := baseline
		mutate(&result)
		if err := validateCellResult(cell, result); err == nil {
			t.Fatalf("validateCellResult accepted invalid mutation %d: %+v", i, result)
		}
	}
}

func TestReportAndMatrixTypesContainNoOpenPayloadMaps(t *testing.T) {
	for _, typ := range []reflect.Type{reflect.TypeFor[CellResult](), reflect.TypeFor[RoutingConformanceChecked](), reflect.TypeFor[QualificationReport](), reflect.TypeFor[QualificationRow](), reflect.TypeFor[QualificationEvidenceRecord](), reflect.TypeFor[QualificationEvidence]()} {
		for field := range typ.Fields() {
			if field.Type.Kind() == reflect.Map || field.Type.Kind() == reflect.Interface {
				t.Fatalf("%s.%s is an open payload field", typ.Name(), field.Name)
			}
		}
	}
}
