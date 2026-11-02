package matrix

import (
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/verify/schema"
)

// runScenario stands a fixture up for one scenario, runs the real ladder, and
// enforces DINV-7 (the workspace's own tracked tree is byte-identical after
// the run, incl. any rung-3 two-leg). It returns the verdict for the caller to
// assert on. The body is identical for every stack — the driver is stack-blind.
func runScenario(t *testing.T, sf stackFixture, sc scenario) *schema.Verdict {
	t.Helper()
	ws, bindings := buildWorkspace(t, sf.dir, sc.baseSetup, sc.seed)
	manifest := loadManifest(t, ws)
	v := runLadder(t, ws, manifest, bindings)
	assertTrackedTreeClean(t, ws)
	return v
}

// TestFixtureMatrix_LadderAdjudicatesEachStack is A-1: the identical harness
// binds each stack's manifest and runs the full registered ladder to the
// expected verdict on the clean run and every legitimate diff (test-only,
// refactor-only, mixed) — with zero per-stack harness code (asserted
// separately by the inspection test). A mixed diff's genuine new test survives
// rung 3 (falsification records not-a-tautology), never a wrong-block.
func TestFixtureMatrix_LadderAdjudicatesEachStack(t *testing.T) {
	for _, sf := range stackFixtures() {
		sf := sf
		t.Run(sf.stack, func(t *testing.T) {
			requireTool(t, sf.tool)
			for _, sc := range sf.scenarios {
				if sc.isCatch() {
					continue
				}
				sc := sc
				t.Run(sc.name, func(t *testing.T) {
					v := runScenario(t, sf, sc)
					if v.State != sc.wantState {
						t.Errorf("verdict state = %q, want %q (findings: %v)", v.State, sc.wantState, findingClasses(v))
					}
				})
			}
		})
	}
}

// TestCatchSuite_BlocksEverySeedFamily is A-2: every adversarial seed — a
// tautological test (rung 3), a silenced test via deletion / skip-marker /
// assertion-removal (rung 3), and a config-pin downgrade (rung 0) — is blocked
// (verdict red) with the expected finding class, on every stack it applies to.
// Each case emits its EVT-4 record for the permanent CI job. Paired with the
// legitimate diffs in the matrix test above, this is the wrong-block guardrail:
// the seeds red, their honest counterparts do not.
func TestCatchSuite_BlocksEverySeedFamily(t *testing.T) {
	for _, sf := range stackFixtures() {
		sf := sf
		t.Run(sf.stack, func(t *testing.T) {
			requireTool(t, sf.tool)
			for _, sc := range sf.scenarios {
				if !sc.isCatch() {
					continue
				}
				sc := sc
				t.Run(sc.name, func(t *testing.T) {
					v := runScenario(t, sf, sc)
					got := findingClasses(v)
					f := findingWithClass(v, sc.wantClass)
					emitEVT4(sf.stack, sc, f != nil && v.State == "red", got)

					if v.State != "red" {
						t.Errorf("seed not blocked: verdict state = %q, want red (findings: %v)", v.State, got)
					}
					if f == nil {
						t.Fatalf("no %q finding emitted; got %v", sc.wantClass, got)
					}
					if sc.wantRung != "" && f.Rung != sc.wantRung {
						t.Errorf("%q finding on rung %q, want %q", sc.wantClass, f.Rung, sc.wantRung)
					}
				})
			}
		})
	}
}
