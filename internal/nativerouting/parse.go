package nativerouting

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

func closedDecode(data []byte, dst any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return fmt.Errorf("trailing JSON value")
		}
		return fmt.Errorf("trailing JSON: %w", err)
	}
	return nil
}

// ParseCatalog decodes exactly native-routing-catalog/v1. Unknown keys,
// including raw argument escape hatches, are typed parse defects.
func ParseCatalog(data []byte) (Catalog, []Diagnostic) {
	var context Catalog
	_ = json.Unmarshal(data, &context) // context only; closedDecode is authority.
	var c Catalog
	if err := closedDecode(data, &c); err != nil {
		defect := "malformed_catalog"
		if strings.Contains(err.Error(), "unknown field") {
			defect = "unknown_key"
		}
		if strings.Contains(err.Error(), "raw_arg") || strings.Contains(err.Error(), "arguments") {
			defect = "raw_arguments_forbidden"
		}
		diagnostic := Diagnostic{Harness: context.Harness, Role: context.Role, CatalogID: context.CatalogID, Defect: defect, Field: "catalog", Detail: err.Error()}
		if len(context.Entries) != 0 {
			diagnostic.ProfileID = context.Entries[0].NativeProfileID
			diagnostic.QualificationID = context.Entries[0].QualificationID
		}
		return Catalog{}, []Diagnostic{diagnostic}
	}
	return c, nil
}

// ParseQualification decodes one independently stored qualification artifact.
func ParseQualification(data []byte) (Qualification, []Diagnostic) {
	var context Qualification
	_ = json.Unmarshal(data, &context)
	var q Qualification
	if err := closedDecode(data, &q); err != nil {
		defect := "malformed_qualification"
		if strings.Contains(err.Error(), "unknown field") {
			defect = "unknown_key"
		}
		if strings.Contains(err.Error(), "raw_arg") || strings.Contains(err.Error(), "arguments") {
			defect = "raw_arguments_forbidden"
		}
		return Qualification{}, []Diagnostic{{Harness: context.Harness, Role: context.Role, ProfileID: context.NativeProfileID, QualificationID: context.QualificationID, Defect: defect, Field: "qualification", Detail: err.Error()}}
	}
	return q, nil
}

// ParseQualificationSet parses an array while preserving artifact-local
// diagnostics. Every element is closed-decoded independently.
func ParseQualificationSet(data []byte) (QualificationSet, []Diagnostic) {
	var raw []json.RawMessage
	if err := closedDecode(data, &raw); err != nil {
		return nil, []Diagnostic{{Defect: "malformed_qualification_set", Field: "qualifications", Detail: err.Error()}}
	}
	set := make(QualificationSet, 0, len(raw))
	var diagnostics []Diagnostic
	for _, item := range raw {
		q, ds := ParseQualification(item)
		diagnostics = append(diagnostics, ds...)
		if len(ds) == 0 {
			set = append(set, q)
		}
	}
	if len(diagnostics) != 0 {
		return nil, diagnostics
	}
	return set, nil
}

// ParseAndValidateCatalog is the closed one-unit boundary for a catalog and
// its separately retained qualification artifacts.
func ParseAndValidateCatalog(catalogJSON []byte, qualificationJSON [][]byte, capability NativeCapability, clock TrustedClock, authorization AuthorizationPolicy) (ValidatedCatalog, []Diagnostic) {
	c, diagnostics := ParseCatalog(catalogJSON)
	set := make(QualificationSet, 0, len(qualificationJSON))
	for _, raw := range qualificationJSON {
		q, ds := ParseQualification(raw)
		diagnostics = append(diagnostics, ds...)
		if len(ds) == 0 {
			set = append(set, q)
		}
	}
	if len(diagnostics) != 0 {
		for i := range diagnostics {
			if diagnostics[i].Harness == "" {
				diagnostics[i].Harness = c.Harness
			}
			if diagnostics[i].Role == "" {
				diagnostics[i].Role = c.Role
			}
			diagnostics[i].CatalogID = c.CatalogID
		}
		return ValidatedCatalog{}, diagnostics
	}
	return ValidateCatalog(c, set, capability, clock, authorization)
}
