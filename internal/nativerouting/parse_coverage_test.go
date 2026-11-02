package nativerouting

import (
	"strings"
	"testing"
)

func TestClosedParsersRejectTrailingAndRawArgumentInputs(t *testing.T) {
	var dst map[string]any
	if err := closedDecode([]byte(`{} {}`), &dst); err == nil || !strings.Contains(err.Error(), "trailing JSON value") {
		t.Fatalf("trailing value error = %v", err)
	}
	if err := closedDecode([]byte(`{} x`), &dst); err == nil || !strings.Contains(err.Error(), "trailing JSON") {
		t.Fatalf("trailing malformed error = %v", err)
	}
	catalog := `{"harness":"h","role":"implementation","catalog_id":"c","entries":[{"native_profile_id":"p","qualification_id":"q"}],"raw_arguments":[]}`
	if _, diagnostics := ParseCatalog([]byte(catalog)); len(diagnostics) != 1 || diagnostics[0].Defect != "raw_arguments_forbidden" || diagnostics[0].ProfileID != "p" || diagnostics[0].QualificationID != "q" {
		t.Fatalf("catalog diagnostics = %+v", diagnostics)
	}
	qualification := `{"harness":"h","role":"review","native_profile_id":"p","qualification_id":"q","raw_arg":"--unsafe"}`
	if _, diagnostics := ParseQualification([]byte(qualification)); len(diagnostics) != 1 || diagnostics[0].Defect != "raw_arguments_forbidden" {
		t.Fatalf("qualification diagnostics = %+v", diagnostics)
	}
}

func TestParseQualificationSetPreservesElementDiagnostics(t *testing.T) {
	if _, diagnostics := ParseQualificationSet([]byte(`not-json`)); len(diagnostics) != 1 || diagnostics[0].Defect != "malformed_qualification_set" {
		t.Fatalf("outer diagnostics = %+v", diagnostics)
	}
	if set, diagnostics := ParseQualificationSet([]byte(`[{}, {"unknown":true}]`)); set != nil || len(diagnostics) != 1 {
		t.Fatalf("mixed set = %+v, diagnostics = %+v", set, diagnostics)
	}
	if set, diagnostics := ParseQualificationSet([]byte(`[{}, {}]`)); len(diagnostics) != 0 || len(set) != 2 {
		t.Fatalf("valid set = %+v, diagnostics = %+v", set, diagnostics)
	}
}
