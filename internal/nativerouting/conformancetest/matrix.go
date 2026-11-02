package conformancetest

import (
	"errors"
	"fmt"
	"sort"

	"github.com/agenticlab-ai/pyxis/internal/entity"
	"github.com/agenticlab-ai/pyxis/internal/executiondemand"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/nativerouting"
)

type catalogOwner interface {
	port.AgentRuntime
	port.CatalogProvider
	port.LaunchBuilder
	port.NativeLaunchCapturer
	nativerouting.NativeCapability
}

type catalogUnit struct {
	catalog        nativerouting.ValidatedCatalog
	qualifications map[string]nativerouting.Qualification
}

// Check runs catalog, qualification, resolution, and typed-builder conformance
// over every generated cell. It returns only a complete passing matrix.
func Check(domain Domain, registry RuntimeRegistry, clock nativerouting.TrustedClock, authorization nativerouting.AuthorizationPolicy) (Matrix, QualificationReport, error) {
	if err := ValidateDomain(domain); err != nil {
		return Matrix{}, QualificationReport{}, err
	}
	if registry == nil || clock == nil || authorization == nil {
		return Matrix{}, QualificationReport{}, errors.New("native routing conformance: registry, trusted clock, and authorization are required")
	}
	liveHarnesses, err := canonicalStrings(harnessNames(registry.Harnesses()))
	if err != nil || !equalStrings(liveHarnesses, domain.Harnesses) {
		return Matrix{}, QualificationReport{}, errors.New("native routing conformance: domain does not equal the live runtime registry")
	}

	owners := make(map[string]catalogOwner, len(domain.Harnesses))
	units := make(map[string]catalogUnit, len(domain.Harnesses)*len(domain.Roles))
	for _, harness := range domain.Harnesses {
		runtimeOwner, resolveErr := registry.Resolve(entity.Harness(harness))
		if resolveErr != nil {
			return Matrix{}, QualificationReport{}, fmt.Errorf("native routing conformance: resolve registered harness %q: %w", harness, resolveErr)
		}
		owner, ok := runtimeOwner.(catalogOwner)
		if !ok {
			return Matrix{}, QualificationReport{}, fmt.Errorf("native routing conformance: registered harness %q has no complete catalog owner", harness)
		}
		owners[harness] = owner
		for _, role := range domain.Roles {
			catalog, qualifications, catalogErr := owner.Catalog(role)
			if catalogErr != nil {
				return Matrix{}, QualificationReport{}, fmt.Errorf("native routing conformance: %s/%s catalog omitted: %w", harness, role, catalogErr)
			}
			if catalog.Harness != entity.Harness(harness) || catalog.Role != role {
				return Matrix{}, QualificationReport{}, fmt.Errorf("native routing conformance: unregistered catalog owner %q for %s/%s", catalog.Harness, harness, role)
			}
			validated, diagnostics := nativerouting.ValidateCatalog(catalog, qualifications, owner, clock, authorization)
			if len(diagnostics) != 0 {
				return Matrix{}, QualificationReport{}, fmt.Errorf("native routing conformance: %s/%s catalog or qualification is stale or invalid: %v", harness, role, diagnostics)
			}
			byID := make(map[string]nativerouting.Qualification, len(qualifications))
			for _, qualification := range qualifications {
				byID[qualification.QualificationID] = qualification
			}
			units[catalogKey(harness, role)] = catalogUnit{catalog: validated, qualifications: byID}
		}
	}

	results := make([]CellResult, 0, len(domain.Cells))
	reportRows := make([]QualificationRow, 0, len(domain.Cells))
	reportQualifications := make([]QualificationEvidenceRecord, 0)
	reportedQualifications := map[string]bool{}
	for _, cell := range domain.Cells {
		unit, ok := units[catalogKey(cell.Harness.String(), cell.Role)]
		if !ok {
			return Matrix{}, QualificationReport{}, fmt.Errorf("native routing conformance: catalog omitted for cell %q", cell.CellID)
		}
		resolution := (nativerouting.Resolver{}).Resolve(nativerouting.Request{
			SchemaVersion: nativerouting.RequestSchema, AttemptID: "conformance/" + cell.CellID,
			DemandID: executiondemand.DemandID("conformance/" + cell.SemanticProfileID),
			Harness:  cell.Harness, Role: cell.Role, RequestedProfile: cell.RequestedProfile, Mode: nativerouting.ModeNormal,
		}, unit.catalog)
		result := baseCellResult(cell, resolution, unit.catalog.Catalog.AdapterGeneration)
		switch resolution.Class {
		case nativerouting.ResolutionExact, nativerouting.ResolutionPromoted:
			if resolution.NativeProfile == nil {
				return Matrix{}, QualificationReport{}, fmt.Errorf("native routing conformance: resolved cell %q omitted selected profile", cell.CellID)
			}
			selected := *resolution.NativeProfile
			qualification, qualified := unit.qualifications[selected.QualificationID]
			if !qualified || nativerouting.QualificationDigest(qualification) != selected.QualificationDigest {
				return Matrix{}, QualificationReport{}, fmt.Errorf("native routing conformance: resolved cell %q omitted its current qualification", cell.CellID)
			}
			nativeSpec, buildErr := owners[cell.Harness.String()].Build(selected)
			if buildErr != nil {
				return Matrix{}, QualificationReport{}, fmt.Errorf("native routing conformance: build cell %q: %w", cell.CellID, buildErr)
			}
			capture, captureErr := owners[cell.Harness.String()].CaptureNativeLaunch(nativeSpec)
			if captureErr != nil {
				return Matrix{}, QualificationReport{}, fmt.Errorf("native routing conformance: capture cell %q: %w", cell.CellID, captureErr)
			}
			if nativeSpec.NativeProfileID != selected.NativeProfileID || !capture.MatchesSelected(selected) {
				return Matrix{}, QualificationReport{}, fmt.Errorf("native routing conformance: adapter changed selected cell %q", cell.CellID)
			}
			result.Expected, result.Result = ExpectedResolved, ResultPass
			result.NativeProfileID, result.QualificationID = selected.NativeProfileID, selected.QualificationID
			result.QualificationDigest, result.AdapterGeneration = selected.QualificationDigest, selected.AdapterGeneration
			result.EvidenceReference = "qualification:" + selected.QualificationID + "@" + selected.QualificationDigest
			reportRows = append(reportRows, reportRow(cell, selected))
			qualificationKey := selected.QualificationID + "\x00" + selected.QualificationDigest
			if !reportedQualifications[qualificationKey] {
				reportQualifications = append(reportQualifications, qualificationEvidence(selected, qualification))
				reportedQualifications[qualificationKey] = true
			}
		case nativerouting.ResolutionUnsupported:
			if resolution.NativeProfile != nil || len(resolution.UnmetComponents) == 0 {
				return Matrix{}, QualificationReport{}, fmt.Errorf("native routing conformance: unsupported cell %q is not explicit", cell.CellID)
			}
			result.Expected, result.Result = ExpectedUnsupported, ResultPass
			result.EvidenceReference = "catalog:" + resolution.CatalogID + "@" + resolution.PolicyIdentity
		default:
			return Matrix{}, QualificationReport{}, fmt.Errorf("native routing conformance: cell %q ended %s", cell.CellID, resolution.Class)
		}
		results = append(results, result)
	}

	matrix, err := AssembleMatrix(domain, results)
	if err != nil {
		return Matrix{}, QualificationReport{}, err
	}
	report := QualificationReport{
		SchemaVersion: QualificationReportSchema, DomainIdentity: domain.DomainIdentity, MatrixIdentity: matrix.MatrixIdentity,
		Rows: reportRows, Qualifications: reportQualifications,
		Signoff: SignoffInput{Status: "pending", RequiredEvaluator: "@mdarabi", Rubric: "each component method, evidence, threshold, and result supports its declared level and exact adapter generation", MatrixIdentity: matrix.MatrixIdentity},
	}
	return matrix, report, nil
}

func baseCellResult(cell Cell, resolution nativerouting.Resolution, generation string) CellResult {
	return CellResult{
		SchemaVersion: CellResultSchema, CellID: cell.CellID, Harness: cell.Harness, Role: cell.Role,
		SemanticProfileID: cell.SemanticProfileID, RequestedProfile: cell.RequestedProfile,
		ResolutionClass: resolution.Class, CatalogID: resolution.CatalogID, PolicyIdentity: resolution.PolicyIdentity,
		AdapterGeneration: generation, UnmetComponents: append([]nativerouting.UnmetComponent(nil), resolution.UnmetComponents...),
	}
}

func catalogKey(harness string, role nativerouting.Role) string {
	return harness + "\x00" + string(role)
}

// AssembleMatrix canonicalizes independently completed fixture results and
// rejects omission, duplication, foreign cells, or a non-closed result.
func AssembleMatrix(domain Domain, results []CellResult) (Matrix, error) {
	if err := ValidateDomain(domain); err != nil {
		return Matrix{}, err
	}
	byCell := make(map[string]CellResult, len(results))
	for _, result := range results {
		if _, duplicate := byCell[result.CellID]; duplicate {
			return Matrix{}, fmt.Errorf("native routing conformance: duplicate result for cell %q", result.CellID)
		}
		byCell[result.CellID] = result
	}
	ordered := make([]CellResult, 0, len(domain.Cells))
	for _, cell := range domain.Cells {
		result, ok := byCell[cell.CellID]
		if !ok {
			return Matrix{}, fmt.Errorf("native routing conformance: result omitted for cell %q", cell.CellID)
		}
		delete(byCell, cell.CellID)
		if err := validateCellResult(cell, result); err != nil {
			return Matrix{}, err
		}
		ordered = append(ordered, result)
	}
	if len(byCell) != 0 {
		foreign := make([]string, 0, len(byCell))
		for id := range byCell {
			foreign = append(foreign, id)
		}
		sort.Strings(foreign)
		return Matrix{}, fmt.Errorf("native routing conformance: results name foreign cells %v", foreign)
	}
	matrix := Matrix{SchemaVersion: MatrixSchema, DomainIdentity: domain.DomainIdentity, Results: ordered}
	matrix.MatrixIdentity = contentIdentity(Matrix{SchemaVersion: matrix.SchemaVersion, DomainIdentity: matrix.DomainIdentity, Results: matrix.Results})
	return matrix, nil
}

func validateCellResult(cell Cell, result CellResult) error {
	if result.SchemaVersion != CellResultSchema || result.CellID != cell.CellID || result.Harness != cell.Harness || result.Role != cell.Role || result.SemanticProfileID != cell.SemanticProfileID || result.RequestedProfile != cell.RequestedProfile {
		return fmt.Errorf("native routing conformance: result identity does not equal cell %q", cell.CellID)
	}
	if result.Result != ResultPass || result.CatalogID == "" || result.PolicyIdentity == "" || result.AdapterGeneration == "" || result.EvidenceReference == "" {
		return fmt.Errorf("native routing conformance: cell %q has no closed passing result", cell.CellID)
	}
	switch result.Expected {
	case ExpectedResolved:
		if result.ResolutionClass != nativerouting.ResolutionExact && result.ResolutionClass != nativerouting.ResolutionPromoted {
			return fmt.Errorf("native routing conformance: resolved cell %q has class %q", cell.CellID, result.ResolutionClass)
		}
		if result.NativeProfileID == "" || result.QualificationID == "" || result.QualificationDigest == "" || len(result.UnmetComponents) != 0 {
			return fmt.Errorf("native routing conformance: resolved cell %q omitted qualification identity", cell.CellID)
		}
	case ExpectedUnsupported:
		if result.ResolutionClass != nativerouting.ResolutionUnsupported || result.NativeProfileID != "" || result.QualificationID != "" || result.QualificationDigest != "" || len(result.UnmetComponents) == 0 {
			return fmt.Errorf("native routing conformance: unsupported cell %q is not explicit", cell.CellID)
		}
	default:
		return fmt.Errorf("native routing conformance: cell %q has unknown expected result %q", cell.CellID, result.Expected)
	}
	return nil
}
