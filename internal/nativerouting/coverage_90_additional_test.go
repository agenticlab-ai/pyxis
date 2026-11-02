package nativerouting

import (
	"strings"
	"testing"
)

func TestCoverage90ClosedCodecRejectsInvalidDocuments(t *testing.T) {
	if _, err := ParseRequest([]byte(`{}`)); err == nil {
		t.Fatal("ParseRequest must reject a structurally valid but incomplete request")
	}
	if _, err := ParseResolution([]byte(`{`)); err == nil {
		t.Fatal("ParseResolution must reject malformed JSON")
	}
	if _, err := ParseResolution([]byte(`{}`)); err == nil {
		t.Fatal("ParseResolution must reject an incomplete resolution")
	}
}

func TestCoverage90RequestDefectsNameEveryMissingEnvelopeField(t *testing.T) {
	defects := requestDefects(Request{})
	if len(defects) < 6 {
		t.Fatalf("requestDefects(empty) = %#v, want all missing envelope fields", defects)
	}
}

func TestCoverage90TypedControlFailures(t *testing.T) {
	integer := int64(1)
	boolean := true
	tests := []NativeControl{
		{Type: ControlString},
		{Name: "string", Type: ControlString, IntegerValue: &integer},
		{Name: "integer", Type: ControlInteger},
		{Name: "boolean", Type: ControlBoolean},
		{Name: "unknown", Type: ControlType("object"), BooleanValue: &boolean},
	}
	for _, control := range tests {
		if err := validateControl(control); err == nil {
			t.Fatalf("validateControl(%+v) must fail closed", control)
		}
	}
	if digestLike(strings.Repeat("z", 64)) {
		t.Fatal("digestLike accepted a non-hex digest")
	}
	if err := validateControls([]NativeControl{{Name: "same", Type: ControlString}, {Name: "same", Type: ControlString}}); err == nil {
		t.Fatal("validateControls accepted a duplicate name")
	}
	if got := levelIndex(Component("unknown"), "value"); got != -1 {
		t.Fatalf("levelIndex(unknown) = %d, want -1", got)
	}
}

func TestCoverage90ErrorValues(t *testing.T) {
	if got := (NativeProfileDefect{Detail: "native detail"}).Error(); got != "native detail" {
		t.Fatalf("NativeProfileDefect.Error() = %q", got)
	}
	if got := (Diagnostic{Defect: "bad", Field: "entry.model", Detail: "unsupported"}).Error(); got == "" {
		t.Fatal("Diagnostic.Error() returned an empty message")
	}
}
