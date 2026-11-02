package contractgate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
)

// snapshot records a directory tree's file paths + contents so a test can prove
// the gate mutated NOTHING (design §3.11 / A-5, DINV-1) — the read-only
// guarantee is verified, not asserted.
func snapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		out[rel] = string(b)
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot walk: %v", err)
	}
	return out
}

func assertUnchanged(t *testing.T, before, after map[string]string) {
	t.Helper()
	if len(before) != len(after) {
		t.Fatalf("file count changed: %d → %d (the gate wrote or removed a file — DINV-1 violated)", len(before), len(after))
	}
	for p, b := range before {
		a, ok := after[p]
		if !ok {
			t.Fatalf("file %q disappeared (DINV-1 violated)", p)
		}
		if a != b {
			t.Fatalf("file %q content changed (DINV-1 violated)", p)
		}
	}
	for p := range after {
		if _, ok := before[p]; !ok {
			t.Fatalf("file %q was created (DINV-1 violated: nothing materializes)", p)
		}
	}
}

// runFixture runs the verb against a testdata fixture root with the default
// config shape (the machine faces are local-substitutable, design §3.11), and
// snapshots the tree before/after to enforce zero mutation.
func runFixture(t *testing.T, fixture, feature string) Verdict {
	t.Helper()
	root := filepath.Join("testdata", fixture)
	before := snapshot(t, root)
	v := RunWithConfig(root, feature, config.Defaults())
	after := snapshot(t, root)
	assertUnchanged(t, before, after)
	return v
}

func TestRun(t *testing.T) {
	tests := []struct {
		name      string
		fixture   string
		feature   string
		wantClass Class
		wantClean bool
		// wantInputs: expected count of evaluated input identities (0 = don't check).
		wantInputs int
	}{
		{
			name:       "well-formed contract is mechanically clean",
			fixture:    "wellformed",
			feature:    "demo",
			wantClass:  ClassClean,
			wantClean:  true,
			wantInputs: 2, // the source map + one serving design
		},
		{
			name:      "missing source map (unfolded ledger) is configuration-error",
			fixture:   "missing",
			feature:   "demo",
			wantClass: ClassConfigurationError,
			wantClean: false,
		},
		{
			name:      "nonexistent feature (no ledger dir) is configuration-error",
			fixture:   "wellformed",
			feature:   "no-such-feature",
			wantClass: ClassConfigurationError,
			wantClean: false,
		},
		{
			name:      "malformed machine face is error",
			fixture:   "malformed",
			feature:   "demo",
			wantClass: ClassError,
			wantClean: false,
		},
		{
			name:      "empty feature slug is configuration-error",
			fixture:   "wellformed",
			feature:   "",
			wantClass: ClassConfigurationError,
			wantClean: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			v := runFixture(t, tc.fixture, tc.feature)

			if v.Class != tc.wantClass {
				t.Errorf("class = %q, want %q (reason: %q)", v.Class, tc.wantClass, v.Reason)
			}
			if v.Clean() != tc.wantClean {
				t.Errorf("Clean() = %v, want %v", v.Clean(), tc.wantClean)
			}

			// The verdict validates closed on every path (design §3.11).
			if d := v.Validate(); len(d) != 0 {
				t.Errorf("verdict does not validate closed: %v", d)
			}

			// The reserved semantic slot is present and marked not-evaluated on
			// EVERY path (DINV-4) — a clean pass is never reported executable.
			if v.Semantic.Evaluated {
				t.Errorf("semantic slot reports evaluated — DINV-4 forbids it in v1")
			}
			if v.Semantic.Note == "" {
				t.Errorf("semantic slot carries no not-evaluated marker (DINV-4)")
			}
			if len(v.Semantic.Classes) != 0 || len(v.Semantic.Findings) != 0 {
				t.Errorf("semantic slot is populated in v1 — must stay empty (DINV-4)")
			}

			// The schema header is always present and pinned.
			if v.SchemaVersion != SchemaVersion || v.GeneratedBy != "docgate" || v.ToolVersion == "" {
				t.Errorf("verdict header wrong: version=%q by=%q tool=%q", v.SchemaVersion, v.GeneratedBy, v.ToolVersion)
			}

			// A fail-closed edge runs ZERO checks (design §3.11); its results
			// carry only the resolution defect, never a real check run.
			if !tc.wantClean && tc.wantClass != ClassClean {
				for _, r := range v.CheckResults {
					if r.Check != "resolve" {
						t.Errorf("fail-closed verdict ran a real check %q — zero checks must run on bad input", r.Check)
					}
				}
			}

			if tc.wantInputs > 0 && len(v.Evaluated) != tc.wantInputs {
				t.Errorf("evaluated inputs = %d, want %d (%+v)", len(v.Evaluated), tc.wantInputs, v.Evaluated)
			}
		})
	}
}

// The clean verdict records the exact content fingerprints it evaluated (design
// §3.7, AP-13): the source map's content_fingerprint and each serving design's.
func TestCleanVerdictRecordsFingerprints(t *testing.T) {
	v := runFixture(t, "wellformed", "demo")
	if !v.Clean() {
		t.Fatalf("expected clean, got %q: %s", v.Class, v.Reason)
	}

	var sawSourceMap, sawDesign bool
	for _, in := range v.Evaluated {
		switch in.Kind {
		case "prd-source-map":
			sawSourceMap = true
			if in.Fingerprint == "" {
				t.Errorf("source-map identity carries no fingerprint")
			}
			if in.Ref != "docs/prds/demo/effective.source.json" {
				t.Errorf("source-map ref = %q, want the source-map path", in.Ref)
			}
		case "serving-design":
			sawDesign = true
			if in.Ref != "demo-v1" {
				t.Errorf("serving-design ref = %q, want demo-v1", in.Ref)
			}
			if in.Fingerprint == "" {
				t.Errorf("serving-design %q carries no fingerprint", in.Ref)
			}
		default:
			t.Errorf("unexpected input kind %q", in.Kind)
		}
	}
	if !sawSourceMap {
		t.Errorf("verdict did not record the PRD source-map identity")
	}
	if !sawDesign {
		t.Errorf("verdict did not record the serving-design identity")
	}
}

// The contract-scope subset (T3) assembles the two self-registered checks; a
// well-formed fixture runs them all and stays clean, recording one passed
// CheckResult per check (design §3.9).
func TestContractScopeSubsetRunsOnClean(t *testing.T) {
	v := runFixture(t, "wellformed", "demo")
	if !v.Clean() {
		t.Fatalf("well-formed fixture is not clean: %q — %s", v.Class, v.Reason)
	}
	if len(v.CheckResults) != len(contractScopeChecks()) {
		t.Fatalf("clean verdict ran %d checks, want %d (the assembled subset)", len(v.CheckResults), len(contractScopeChecks()))
	}
	for _, r := range v.CheckResults {
		if !r.Passed {
			t.Errorf("check %q failed on the well-formed fixture: %+v", r.Check, r.Defects)
		}
		if r.Class != ClassClean {
			t.Errorf("passed check %q carries class %q, want clean", r.Check, r.Class)
		}
	}
}

// Determinism (DINV-5): repeated runs over identical machine faces yield a
// byte-identical verdict (the fingerprints and ordering never vary).
func TestDeterministic(t *testing.T) {
	root := filepath.Join("testdata", "wellformed")
	a := RunWithConfig(root, "demo", config.Defaults())
	b := RunWithConfig(root, "demo", config.Defaults())
	if a.Class != b.Class {
		t.Fatalf("class not stable: %q vs %q", a.Class, b.Class)
	}
	if len(a.Evaluated) != len(b.Evaluated) {
		t.Fatalf("evaluated count not stable: %d vs %d", len(a.Evaluated), len(b.Evaluated))
	}
	for i := range a.Evaluated {
		if a.Evaluated[i] != b.Evaluated[i] {
			t.Fatalf("evaluated[%d] not stable: %+v vs %+v", i, a.Evaluated[i], b.Evaluated[i])
		}
	}
}

// The Validate self-guard REJECTS a verdict that reports a reserved semantic
// class or an evaluated semantic slot (DINV-4) — proving the closed-shape guard
// actually bites, not just that well-formed verdicts pass it.
func TestValidateRejectsSemanticLeak(t *testing.T) {
	v := newVerdict("demo")
	v.Class = ClassClean
	if d := v.Validate(); len(d) != 0 {
		t.Fatalf("a well-formed verdict was rejected: %v", d)
	}

	leak := newVerdict("demo")
	leak.Class = ClassAmbiguous // a reserved semantic class must never be a v1 class
	if d := leak.Validate(); len(d) == 0 {
		t.Errorf("Validate accepted a reserved semantic class as the overall class (DINV-4)")
	}

	evald := newVerdict("demo")
	evald.Class = ClassClean
	evald.Semantic.Evaluated = true
	if d := evald.Validate(); len(d) == 0 {
		t.Errorf("Validate accepted an evaluated semantic slot in v1 (DINV-4)")
	}
}
