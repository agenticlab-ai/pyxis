package payload

import (
	"reflect"
	"strings"
	"testing"
)

func TestSubstitutionHelpersCoverDuplicatesAndRemnants(t *testing.T) {
	if got := VarsIn([]byte("{{z}} {{a}} {{z}}")); !reflect.DeepEqual(got, []string{"a", "z"}) {
		t.Fatalf("VarsIn = %v", got)
	}
	if _, err := Substitute([]byte("{{missing}} {{missing}}"), nil); err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("missing substitution error = %v", err)
	}
	if _, err := Substitute([]byte("first\n{{INVALID}}"), nil); err == nil || !strings.Contains(err.Error(), "line 2") {
		t.Fatalf("remnant error = %v", err)
	}
	if got, err := RenderInstalledWorkflow([]byte("{{branch}}/{{remote}}"), "main", "origin", WorkflowPins{}); err != nil || string(got) != "main/origin" {
		t.Fatalf("RenderInstalledWorkflow = %q, %v", got, err)
	}
	if got := snippet([]byte("one line\nignored")); got != "one line" {
		t.Fatalf("newline snippet = %q", got)
	}
	if got := snippet([]byte(strings.Repeat("x", 30))); len(got) <= 24 || !strings.HasSuffix(got, "…") {
		t.Fatalf("long snippet = %q", got)
	}
}
