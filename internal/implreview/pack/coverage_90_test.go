package pack

import (
	"strings"
	"testing"
)

func TestCoverage90ContextPackRejectsWhitespaceSubject(t *testing.T) {
	in := goldenInputs()
	in.Subject = " "
	if _, err := Build(in); err == nil || !strings.Contains(err.Error(), "empty subject") {
		t.Fatalf("Build whitespace subject error = %v", err)
	}
}
