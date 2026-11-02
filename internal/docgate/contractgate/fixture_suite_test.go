package contractgate

import (
	"path/filepath"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
)

// The consolidated fixture suite (plan T8; design contract-gate-v1 §3.11: "table-
// driven tests over fixture contracts — clean and one-per-defect-class"). The
// per-check tests (checks_test.go) drive the check logic; THIS test proves the
// corpus is COMPLETE — a clean fixture plus at least one fixture per defect class
// (structural/compilability/currency/coverage/link), each producing exactly its
// class as a permanent, checked-in test. It is the guardrail that every defect
// class stays exercised: if a class loses its fixture, this fails.

// classFixture pins one representative fixture per outcome class and the failed
// check it must exercise. Each is a permanent fixture under testdata/; the suite
// asserts the fixture still produces its class and reds its check.
type classFixture struct {
	// class is the human name of the defect class this fixture anchors.
	class string
	// fixture + feature locate the fixture contract.
	fixture string
	feature string
	// wantClass is the verdict class the fixture must produce.
	wantClass Class
	// wantCheck is the check the fixture must red (empty for the clean fixture).
	wantCheck string
	// severityClass is the check's declared severity class (empty for clean) — the
	// EVT-2 taxonomy dimension, asserted so every severity class has a fixture too.
	severityClass SeverityClass
}

// fixtureSuite is the canonical corpus: one clean fixture and one per defect
// class. Kept in one place so the coverage of the defect-class space is a single,
// reviewable list (adding a check adds a row here).
var fixtureSuite = []classFixture{
	{class: "clean", fixture: "wellformed", feature: "demo", wantClass: ClassClean},
	{class: "structural", fixture: "struct-nofit", feature: "demo", wantClass: ClassIncomplete, wantCheck: "structural-completeness", severityClass: SeverityClassStructural},
	{class: "compilability", fixture: "compil-orphan-serves", feature: "demo", wantClass: ClassMalformed, wantCheck: "compilability", severityClass: SeverityClassCompilability},
	{class: "currency", fixture: "stale-serve", feature: "demo", wantClass: ClassStale, wantCheck: "currency", severityClass: SeverityClassCurrency},
	{class: "coverage", fixture: "uncovered-must", feature: "demo", wantClass: ClassIncomplete, wantCheck: "inverse-coverage", severityClass: SeverityClassCoverage},
	{class: "link", fixture: "dangling-citation", feature: "demo", wantClass: ClassUnresolved, wantCheck: "link", severityClass: SeverityClassLink},
}

// TestFixtureSuiteCoversEveryDefectClass proves the fixture corpus exercises a
// clean pass and each defect class exactly (plan T8 scope: "the fixture suite
// (clean plus one per defect class) as permanent tests"). Each fixture produces
// its declared verdict class, the clean one passes every check, and each defect
// fixture reds the expected check with named defects. The runFixture guard also
// re-proves every run is read-only (DINV-1).
func TestFixtureSuiteCoversEveryDefectClass(t *testing.T) {
	for _, tc := range fixtureSuite {
		t.Run(tc.class, func(t *testing.T) {
			v := runFixture(t, tc.fixture, tc.feature)

			if v.Class != tc.wantClass {
				t.Fatalf("%s fixture %q: class = %q, want %q (reason: %q)", tc.class, tc.fixture, v.Class, tc.wantClass, v.Reason)
			}
			if d := v.Validate(); len(d) != 0 {
				t.Errorf("%s fixture verdict does not validate closed: %v", tc.class, d)
			}

			if tc.wantClass == ClassClean {
				if !v.Clean() {
					t.Fatalf("clean fixture %q is not clean: %q", tc.fixture, v.Class)
				}
				for _, r := range v.CheckResults {
					if !r.Passed {
						t.Errorf("clean fixture has a failing check %q: %+v", r.Check, r.Defects)
					}
				}
				return
			}

			// A defect fixture reds its check with named defects.
			var target *CheckResult
			for i := range v.CheckResults {
				if v.CheckResults[i].Check == tc.wantCheck {
					target = &v.CheckResults[i]
				}
			}
			if target == nil {
				t.Fatalf("%s fixture: no CheckResult for %q (results: %+v)", tc.class, tc.wantCheck, v.CheckResults)
			}
			if target.Passed {
				t.Fatalf("%s fixture: check %q passed, expected it to red", tc.class, tc.wantCheck)
			}
			if len(target.Defects) == 0 {
				t.Fatalf("%s fixture: check %q red with zero named defects", tc.class, tc.wantCheck)
			}

			// The failed check's declared severity class matches — so every severity
			// class in the EVT-2 taxonomy is anchored by a fixture.
			sc, ok := checkSeverityClass(tc.wantCheck)
			if !ok {
				t.Fatalf("check %q is not registered", tc.wantCheck)
			}
			if sc != tc.severityClass {
				t.Errorf("%s fixture: check %q severity class = %q, want %q", tc.class, tc.wantCheck, sc, tc.severityClass)
			}
		})
	}
}

// TestEveryDefectClassHasAFixture proves the corpus is COMPLETE against the
// registry — every registered check's severity class has at least one fixture in
// the suite that reds it, and the clean class is covered. If a new check is
// registered without a fixture, this fails, forcing the fixture to be added.
func TestEveryDefectClassHasAFixture(t *testing.T) {
	// The severity classes the fixture suite exercises (the defect fixtures).
	covered := map[SeverityClass]bool{}
	sawClean := false
	for _, tc := range fixtureSuite {
		if tc.wantClass == ClassClean {
			sawClean = true
			continue
		}
		covered[tc.severityClass] = true
	}
	if !sawClean {
		t.Errorf("the fixture suite has no clean fixture")
	}

	// Every registered check's severity class must be covered by a fixture.
	for _, chk := range contractScopeChecks() {
		if !covered[chk.SeverityClass] {
			t.Errorf("registered check %q (severity class %q) has no fixture in the suite that reds it — add one", chk.Name, chk.SeverityClass)
		}
	}

	// The full EVT-2 taxonomy is covered (structural·compilability·currency·
	// coverage·link) — a belt-and-suspenders check against the closed class set.
	for _, sc := range []SeverityClass{
		SeverityClassStructural, SeverityClassCompilability,
		SeverityClassCurrency, SeverityClassCoverage, SeverityClassLink,
	} {
		if !covered[sc] {
			t.Errorf("severity class %q has no fixture in the suite", sc)
		}
	}
}

// TestFixtureSuiteReadOnly re-proves the whole suite is read-only in one place:
// gating every fixture mutates NOTHING (DINV-1). runFixture already snapshots per
// fixture; this asserts the corpus-level invariant explicitly, plus that the
// report builder never materializes on any fixture (a clean verdict emits none,
// a bounce's report goes only to an explicit sink — never the fixture tree).
func TestFixtureSuiteReadOnly(t *testing.T) {
	for _, tc := range fixtureSuite {
		t.Run(tc.class, func(t *testing.T) {
			root := filepath.Join("testdata", tc.fixture)
			before := snapshot(t, root)
			v := RunWithConfig(root, tc.feature, config.Defaults())
			// Building the report (without emitting) touches no file — a pure
			// transform over the verdict; assert it here to cover the read path.
			_, _ = ReportForVerdict(v)
			after := snapshot(t, root)
			assertUnchanged(t, before, after)
		})
	}
}
