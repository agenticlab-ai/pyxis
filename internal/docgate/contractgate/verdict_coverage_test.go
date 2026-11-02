package contractgate

import (
	"strings"
	"testing"
)

func TestVerdictSummaryAndClosedValidationBranches(t *testing.T) {
	advisory := CheckResult{Check: "coverage", Class: ClassIncomplete, Severity: SeverityAdvisory}
	if advisory.Blocking() || !advisory.Advisory() {
		t.Fatalf("advisory disposition = blocking %v, advisory %v", advisory.Blocking(), advisory.Advisory())
	}
	blocking := CheckResult{Check: "shape", Class: ClassMalformed, Severity: SeverityBlocking}
	if !blocking.Blocking() || blocking.Advisory() {
		t.Fatalf("blocking disposition = blocking %v, advisory %v", blocking.Blocking(), blocking.Advisory())
	}
	v := newVerdict("widget")
	v.Class = ClassIncomplete
	v.Trigger = TriggerContractDocPR
	v.CheckResults = []CheckResult{advisory, {Check: "resolve", Passed: false}}
	v.Evaluated = []InputIdentity{{Kind: "prd", Ref: "widget"}}
	v.Reason = "one requirement is uncovered"
	if v.Clean() || v.Blocked() || !v.hasAdvisory() {
		t.Fatalf("verdict predicates = clean %v blocked %v advisory %v", v.Clean(), v.Blocked(), v.hasAdvisory())
	}
	summary := v.Summary()
	for _, want := range []string{"advisory only", "1 check(s) run", "1 advisory defect(s)", v.Reason} {
		if !strings.Contains(summary, want) {
			t.Fatalf("summary %q omits %q", summary, want)
		}
	}
	clean := newVerdict("clean-widget")
	clean.Class = ClassClean
	if !clean.Clean() || !strings.Contains(clean.Summary(), "mechanically clean") {
		t.Fatalf("clean verdict = %+v / %q", clean, clean.Summary())
	}

	invalid := Verdict{SchemaVersion: "old", Trigger: "unknown", Class: ClassAmbiguous, Semantic: Semantic{Evaluated: true}}
	if defects := invalid.Validate(); len(defects) != 5 {
		t.Fatalf("invalid verdict defects = %d: %v", len(defects), defects)
	}
}
