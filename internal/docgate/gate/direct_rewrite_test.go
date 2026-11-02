package gate

import (
	"errors"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/rewriteauth"
)

func TestFilterDirectRewriteAuthorization(t *testing.T) {
	input := []schema.Defect{
		{Verb: "check-immutable", Reason: "history"},
		{Verb: "check-ledger-history", Reason: "history"},
		{Verb: "check-standalone-docs", Reason: "scope"},
		{Verb: "solution-shape", Reason: "lifecycle"},
		{Verb: "check-citations", Reason: "must remain"},
	}
	if got := filterDirectRewriteAuthorization(rewriteauth.Result{}, append([]schema.Defect(nil), input...)); len(got) != len(input) {
		t.Fatalf("inactive authorization filtered defects: %v", got)
	}
	invalid := rewriteauth.Result{Active: true, Err: errors.New("digest mismatch")}
	if got := filterDirectRewriteAuthorization(invalid, append([]schema.Defect(nil), input...)); len(got) != len(input)+1 || got[len(got)-1].Verb != "direct-rewrite-authorization" || got[len(got)-1].File != rewriteauth.MarkerPath {
		t.Fatalf("invalid authorization did not fail closed: %v", got)
	}
	invalidRepair := rewriteauth.Result{Active: true, Marker: rewriteauth.RepairMarkerPath, Err: errors.New("digest mismatch")}
	if got := filterDirectRewriteAuthorization(invalidRepair, append([]schema.Defect(nil), input...)); len(got) != len(input)+1 || got[len(got)-1].File != rewriteauth.RepairMarkerPath {
		t.Fatalf("invalid repair authorization did not name its marker: %v", got)
	}
	valid := rewriteauth.Result{Active: true, Authorized: true}
	got := filterDirectRewriteAuthorization(valid, append([]schema.Defect(nil), input...))
	if len(got) != 1 || got[0].Verb != "check-citations" {
		t.Fatalf("authorization scope = %v", got)
	}
}
