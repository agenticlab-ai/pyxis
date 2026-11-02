package conformancetest

import (
	"errors"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/nativerouting"
)

type coverageCatalogErrorOwner struct{ *fakeOwner }

func (*coverageCatalogErrorOwner) Catalog(nativerouting.Role) (nativerouting.Catalog, nativerouting.QualificationSet, error) {
	return nativerouting.Catalog{}, nil, errors.New("catalog unavailable")
}

func TestCoverage90DomainEqualityBoundaries(t *testing.T) {
	if equalStrings([]string{"one"}, nil) {
		t.Fatal("equalStrings accepted different lengths")
	}
	if equalStrings([]string{"one"}, []string{"two"}) {
		t.Fatal("equalStrings accepted different values")
	}
	if equalRoles([]nativerouting.Role{nativerouting.RoleImplementation}, nil) {
		t.Fatal("equalRoles accepted different lengths")
	}
	if equalRoles(
		[]nativerouting.Role{nativerouting.RoleImplementation},
		[]nativerouting.Role{nativerouting.RoleReview},
	) {
		t.Fatal("equalRoles accepted different values")
	}
}

func TestCoverage90DomainRejectsAnEmptyProfileSet(t *testing.T) {
	registry := fakeRegistry{backends: []string{"fixture"}}
	domain, err := GenerateDomain(emptyPolicy(), registry, Roles{
		nativerouting.RoleImplementation,
		nativerouting.RoleReview,
	})
	if err != nil {
		t.Fatal(err)
	}
	domain.Profiles = nil
	domain.Cells = nil
	domain.DomainIdentity = contentIdentity(domainWithoutIdentity(domain))
	if err := ValidateDomain(domain); err == nil {
		t.Fatal("ValidateDomain accepted an empty semantic-profile set")
	}
}

func TestCoverage90CellResultTerminalBoundaries(t *testing.T) {
	cell := Cell{
		CellID: "harness/implementation/profile", Harness: "harness",
		Role: nativerouting.RoleImplementation, SemanticProfileID: "profile",
	}
	base := CellResult{
		SchemaVersion: CellResultSchema, CellID: cell.CellID, Harness: cell.Harness,
		Role: cell.Role, SemanticProfileID: cell.SemanticProfileID,
		RequestedProfile: cell.RequestedProfile, Result: ResultPass,
		CatalogID: "catalog", PolicyIdentity: "policy", AdapterGeneration: "generation",
		EvidenceReference: "catalog:catalog@policy",
	}

	unknown := base
	unknown.Expected = Expected("unknown")
	if err := validateCellResult(cell, unknown); err == nil {
		t.Fatal("validateCellResult accepted an unknown expected result")
	}

	omitted := base
	omitted.Expected = ExpectedResolved
	omitted.ResolutionClass = nativerouting.ResolutionExact
	if err := validateCellResult(cell, omitted); err == nil {
		t.Fatal("validateCellResult accepted a resolved result without qualification identity")
	}
}

func TestCoverage90CheckReportsCatalogFailure(t *testing.T) {
	domainRegistry := fakeRegistry{backends: []string{"fixture"}}
	domain, err := GenerateDomain(emptyPolicy(), domainRegistry, Roles{
		nativerouting.RoleImplementation,
		nativerouting.RoleReview,
	})
	if err != nil {
		t.Fatal(err)
	}
	owner := &coverageCatalogErrorOwner{fakeOwner: &fakeOwner{harness: "fixture"}}
	registry := fakeRegistry{
		backends: []string{"fixture"},
		owners:   map[string]port.AgentRuntime{"fixture": owner},
	}
	if _, _, err := Check(domain, registry, fakeClock{matrixTime}, fakeAuthorization{}); err == nil {
		t.Fatal("Check accepted a registered owner whose catalog failed")
	}
}
