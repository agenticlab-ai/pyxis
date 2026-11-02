package report_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/implreview/report"
	codegate "github.com/agenticlab-ai/pyxis/internal/verify/schema"
)

// TestCodegateGrammarCompatibility is the load-bearing NR-1/DINV-6 pin: it
// asserts that impl-review's Finding carries the SAME json tags codegate's
// Finding does for the four overlapping wire keys — `path`, `line`, `message`,
// `log_ref` — so the host's fixer parses codegate and impl-review findings as
// ONE grammar family. It compares tags by REFLECTION over both structs; it does
// NOT couple the layouts (impl-review's production code imports no codegate
// struct — see the package doc). This test is the whole coupling: if codegate
// drifts a tag on any of these keys, this fails, forcing the shared grammar to
// be reconciled deliberately rather than silently.
//
// Only this test file imports codegate. The production package stays free of it
// (a compatibility contract on the wire, not a struct import — §3.1, DINV-6).
func TestCodegateGrammarCompatibility(t *testing.T) {
	t.Parallel()

	// The overlapping wire keys and the Go field each side exposes them through.
	// Both fields are looked up by NAME on their own struct, and their full json
	// struct tags must be byte-identical.
	overlap := []struct {
		wireKey            string
		implField, cgField string
	}{
		{"path", "Path", "Path"},
		{"line", "Line", "Line"},
		{"message", "Message", "Message"},
		{"log_ref", "LogRef", "LogRef"},
	}

	implT := reflect.TypeOf(report.Finding{})
	cgT := reflect.TypeOf(codegate.Finding{})

	for _, o := range overlap {
		implSF, ok := implT.FieldByName(o.implField)
		if !ok {
			t.Fatalf("impl-review Finding has no field %q for wire key %q", o.implField, o.wireKey)
		}
		cgSF, ok := cgT.FieldByName(o.cgField)
		if !ok {
			t.Fatalf("codegate Finding has no field %q for wire key %q", o.cgField, o.wireKey)
		}

		implTag := implSF.Tag.Get("json")
		cgTag := cgSF.Tag.Get("json")
		if implTag != cgTag {
			t.Fatalf("grammar drift on wire key %q: impl-review json tag %q != codegate json tag %q "+
				"(the shared finding grammar broke — reconcile NR-1/DINV-6)",
				o.wireKey, implTag, cgTag)
		}

		// The tag's key name must actually be the overlapping wire key — guards
		// against both sides drifting the same way and still passing.
		if gotKey := jsonKey(implTag); gotKey != o.wireKey {
			t.Fatalf("wire key mismatch: field %q json tag %q names key %q, want %q",
				o.implField, implTag, gotKey, o.wireKey)
		}
	}

	// The Go field TYPES of the overlapping keys must also match — a matching tag
	// over a mismatched type (e.g. line as string vs int) would still break the
	// shared parse.
	for _, o := range overlap {
		implSF, _ := implT.FieldByName(o.implField)
		cgSF, _ := cgT.FieldByName(o.cgField)
		if implSF.Type != cgSF.Type {
			t.Fatalf("type drift on wire key %q: impl-review %s != codegate %s",
				o.wireKey, implSF.Type, cgSF.Type)
		}
	}

	// Guard the OTHER direction of DINV-6: impl-review must NOT carry codegate's
	// rung-only keys (check_id/class/rung) — it has its own rule_id/lens identity
	// and no rungs. A stray one would silently re-couple to codegate's layout.
	for _, forbidden := range []string{"check_id", "class", "rung"} {
		for i := 0; i < implT.NumField(); i++ {
			if jsonKey(implT.Field(i).Tag.Get("json")) == forbidden {
				t.Fatalf("impl-review Finding carries codegate rung-only key %q (field %q) — "+
					"DINV-6 forbids it; impl-review has no rungs and its own rule_id/lens identity",
					forbidden, implT.Field(i).Name)
			}
		}
	}
}

// jsonKey extracts the key name from a json struct tag value ("path,omitempty"
// → "path"; "" → "").
func jsonKey(tag string) string {
	key, _, _ := strings.Cut(tag, ",")
	return key
}
