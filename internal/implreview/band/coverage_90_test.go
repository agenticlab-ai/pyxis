package band

import (
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/lane/classifier"
)

func TestCoverage90EmptyClassFallbacks(t *testing.T) {
	if reason := standardReason(classifier.Verdict{Lane: classifier.Human}); !strings.Contains(reason, "without a floor hit") {
		t.Fatalf("standardReason = %q", reason)
	}
	if got := autoClassLabel(""); got != "class unnamed" {
		t.Fatalf("autoClassLabel empty = %q", got)
	}
	if isSkipClass("") {
		t.Fatal("empty class was treated as skippable")
	}
}
