package sweep_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/sweep"
)

// hasReason reports whether any defect's reason contains the substring.
func hasReason(defects []schema.Defect, want string) bool {
	for _, d := range defects {
		if strings.Contains(d.Reason, want) {
			return true
		}
	}
	return false
}

const wellFormed = `schema_version: maintenance-policy/v1
staleness_bound: 720h
waivers:
  packages:
    - package: "internal/worktracker/conformance"
      reason: "test-harness library: consumed by adapter suites, deliberately no binary"
  plans:
    - plan: "installer/001"
      reason: "frozen 2026-07-19, fleet executing — surfaces legitimately pending"
`

// A well-formed policy exposes the bound, the package waivers, and the plan
// waivers with their reasons to a reader (acceptance 2).
func TestWellFormedPolicyExposesEverything(t *testing.T) {
	p, defects := sweep.Parse([]byte(wellFormed))
	if len(defects) != 0 {
		t.Fatalf("well-formed policy flagged: %v", defects)
	}
	if p.StalenessBound != 720*time.Hour {
		t.Fatalf("bound not exposed: %v", p.StalenessBound)
	}
	if len(p.PackageWaivers) != 1 || p.PackageWaivers[0].Package != "internal/worktracker/conformance" {
		t.Fatalf("package waiver not exposed: %+v", p.PackageWaivers)
	}
	if !strings.Contains(p.PackageWaivers[0].Reason, "test-harness") {
		t.Fatalf("package waiver reason not exposed: %q", p.PackageWaivers[0].Reason)
	}
	if len(p.PlanWaivers) != 1 || p.PlanWaivers[0].Plan != "installer/001" {
		t.Fatalf("plan waiver not exposed: %+v", p.PlanWaivers)
	}
	if !strings.Contains(p.PlanWaivers[0].Reason, "fleet executing") {
		t.Fatalf("plan waiver reason not exposed: %q", p.PlanWaivers[0].Reason)
	}
}

// The staleness_bound defaults to fourteen days when the policy omits it
// (acceptance: the default bound a well-formed policy exposes).
func TestStalenessBoundDefaultsWhenAbsent(t *testing.T) {
	p, defects := sweep.Parse([]byte("schema_version: maintenance-policy/v1\n"))
	if len(defects) != 0 {
		t.Fatalf("policy without bound flagged: %v", defects)
	}
	if p.StalenessBound != sweep.DefaultStalenessBound {
		t.Fatalf("default bound wrong: %v", p.StalenessBound)
	}
	if sweep.DefaultStalenessBound != 14*24*time.Hour {
		t.Fatalf("default bound is not fourteen days: %v", sweep.DefaultStalenessBound)
	}
}

// A malformed policy, an unknown key, and a reasonless waiver each refuse to
// load with a named defect (acceptance 1). Each row asserts a defect whose
// reason names the fault.
func TestNamedDefectRefusals(t *testing.T) {
	cases := []struct {
		name, body, wantReason string
	}{
		{"malformed structure", "schema_version: maintenance-policy/v1\n        garbage here\n", "unrecognized structure"},
		{"wrong version", "schema_version: maintenance-policy/v2\n", "unknown schema version"},
		{"absent version", "staleness_bound: 336h\n", "\"schema_version\" is absent"},
		{"unknown top-level key", "schema_version: maintenance-policy/v1\nnonsense: 1\n", "unknown top-level key"},
		{"unknown waivers key", "schema_version: maintenance-policy/v1\nwaivers:\n  bogus:\n", "unknown waivers key"},
		{"unknown package field", "schema_version: maintenance-policy/v1\nwaivers:\n  packages:\n    - package: \"x\"\n      bogus: \"y\"\n      reason: \"r\"\n", "unknown package-waiver field"},
		{"reasonless package waiver", "schema_version: maintenance-policy/v1\nwaivers:\n  packages:\n    - package: \"x\"\n", "package waiver for \"x\" has no reason"},
		{"reasonless plan waiver", "schema_version: maintenance-policy/v1\nwaivers:\n  plans:\n    - plan: \"p/001\"\n", "plan waiver for \"p/001\" has no reason"},
		{"bad bound", "schema_version: maintenance-policy/v1\nstaleness_bound: soon\n", "not a positive duration"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, defects := sweep.Parse([]byte(c.body))
			if !hasReason(defects, c.wantReason) {
				t.Fatalf("expected a defect naming %q, got %v", c.wantReason, defects)
			}
		})
	}
}

// An absent file loads as the empty policy with the default bound (acceptance 1,
// last clause): no defect, the legal advisory-tool state.
func TestAbsentFileIsEmptyPolicy(t *testing.T) {
	root := t.TempDir() // no docs/policy/maintenance-sweep.yaml exists
	p, defects := sweep.LoadPolicy(root)
	if len(defects) != 0 {
		t.Fatalf("absent policy flagged: %v", defects)
	}
	if p.SchemaVersion != sweep.PolicySchemaVersion {
		t.Fatalf("empty policy schema wrong: %q", p.SchemaVersion)
	}
	if p.StalenessBound != sweep.DefaultStalenessBound {
		t.Fatalf("empty policy bound wrong: %v", p.StalenessBound)
	}
	if len(p.PackageWaivers) != 0 || len(p.PlanWaivers) != 0 {
		t.Fatalf("empty policy carries waivers: %+v %+v", p.PackageWaivers, p.PlanWaivers)
	}
}

// A present but malformed file refuses to load through LoadPolicyFile: the empty
// policy is returned WITH a named defect stamped with the resolved path (so a
// caller can never mistake it for a clean load).
func TestLoadFileRefusesMalformed(t *testing.T) {
	root := t.TempDir()
	rel := "policy.yaml"
	if err := os.WriteFile(filepath.Join(root, rel), []byte("schema_version: maintenance-policy/v1\n bad: 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, defects := sweep.LoadPolicyFile(root, rel)
	if len(defects) == 0 {
		t.Fatal("malformed file loaded clean")
	}
	if defects[0].File != rel {
		t.Fatalf("defect not stamped with the resolved path: %q", defects[0].File)
	}
}
