package executiondemand

import (
	"strings"
	"testing"
)

func TestCapsuleValidationHelpersRejectUnsafeShapes(t *testing.T) {
	if safeDiagnosticIdentity("") || safeDiagnosticIdentity(strings.Repeat("a", 257)) || safeDiagnosticIdentity("unsafe value") {
		t.Fatal("safeDiagnosticIdentity accepted an empty, oversized, or unsafe identity")
	}
	if !safeDiagnosticIdentity("repo/name.task_1-ok") {
		t.Fatal("safeDiagnosticIdentity rejected a bounded identifier")
	}
	if validDigest("short") || validDigest(strings.Repeat("A", 64)) || !validDigest(strings.Repeat("a", 64)) {
		t.Fatal("validDigest did not enforce lowercase sha256 shape")
	}
}

func TestCapsuleValidateRejectsEachRequiredOuterField(t *testing.T) {
	valid := mustSeal(t, baselineOriginV2())
	tests := map[string]func(*Capsule){
		"codec":         func(c *Capsule) { c.Codec = "unknown" },
		"demand id":     func(c *Capsule) { c.DemandID = "" },
		"repository":    func(c *Capsule) { c.Repository = "" },
		"plan":          func(c *Capsule) { c.Plan = PlanIdentity{} },
		"task":          func(c *Capsule) { c.Task = "" },
		"source digest": func(c *Capsule) { c.Source = "" },
		"origin":        func(c *Capsule) { c.Origin = Origin{} },
		"digest":        func(c *Capsule) { c.Digest = "" },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			got := valid
			mutate(&got)
			if err := got.validate(); err == nil {
				t.Fatal("validate accepted a malformed capsule")
			}
		})
	}
}
