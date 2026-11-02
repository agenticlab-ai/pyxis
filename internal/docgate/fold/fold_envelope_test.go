package fold_test

import (
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/fold"
	"github.com/agenticlab-ai/pyxis/internal/docgate/ledger"
)

// withTracking splices a Tracking row into an amendment envelope.
func withTracking(e ledger.Entry) ledger.Entry {
	return ledger.Entry{Name: e.Name, Bytes: []byte(strings.Replace(string(e.Bytes),
		"| **Owner** | @o |", "| **Owner** | @o |\n| **Tracking** | issue #54 |", 1))}
}

// FR-24: envelope validation keys off the DECLARED template version.
func TestEnvelopeVersions(t *testing.T) {
	// v2 with Tracking → green.
	v2 := withTracking(amend(goodSupersede, "Template version", "amendment-template/v2"))
	if res := fold.Fold([]ledger.Entry{base(""), v2}); len(res.Defects) != 0 {
		t.Fatalf("v2+Tracking flagged: %v", res.Defects)
	}
	// v2 without Tracking → green (the row is optional).
	if res := fold.Fold([]ledger.Entry{base(""), amend(goodSupersede, "Template version", "amendment-template/v2")}); len(res.Defects) != 0 {
		t.Fatalf("bare v2 flagged: %v", res.Defects)
	}
	// v1 WITH Tracking → red: unknown to the declared version (grandfathering
	// is by declaration, never retroactive).
	red(t, "v1-with-tracking", "unknown to the declared template version",
		base(""), withTracking(amend(goodSupersede)))
	// Declared v3 → red: outside the supported set.
	red(t, "unsupported-envelope-version", "not in the supported set",
		base(""), amend(goodSupersede, "Template version", "amendment-template/v3"))
}
