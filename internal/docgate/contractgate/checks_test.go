package contractgate

import (
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

// TestChecks table-drives the two T3 contract-scope checks over per-defect-class
// fixtures (design contract-gate-v1 §3.9 / plan T3): each structural-completeness
// seed reds `incomplete` naming the requirement, each compilability seed reds
// `malformed` naming the join, and a well-formed contract is `clean`. The
// mutation guard (runFixture) also proves every run is read-only (DINV-1).
func TestChecks(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		feature string
		want    Class
		// wantCheck is the CheckResult.Check expected to fail (empty on clean).
		wantCheck string
		// wantOffender is a substring the failing check's defects must name.
		wantOffender string
	}{
		{
			name:         "(a) a Must FR lacking a fit criterion reds incomplete naming it",
			fixture:      "struct-nofit",
			feature:      "demo",
			want:         ClassIncomplete,
			wantCheck:    "structural-completeness",
			wantOffender: "FR-1",
		},
		{
			name:         "(b) a requirement lacking a verify method reds incomplete naming it",
			fixture:      "struct-noverify",
			feature:      "demo",
			want:         ClassIncomplete,
			wantCheck:    "structural-completeness",
			wantOffender: "FR-1",
		},
		{
			name:         "(c) a missing DoD reds incomplete",
			fixture:      "struct-nodod",
			feature:      "demo",
			want:         ClassIncomplete,
			wantCheck:    "structural-completeness",
			wantOffender: "definition of done",
		},
		{
			name:         "(d) an orphan Serves reds malformed naming the join",
			fixture:      "compil-orphan-serves",
			feature:      "demo",
			want:         ClassMalformed,
			wantCheck:    "compilability",
			wantOffender: "FR-9",
		},
		{
			name:         "(e) a dangling supersession reds malformed naming the unit",
			fixture:      "compil-dangling-supersession",
			feature:      "demo",
			want:         ClassMalformed,
			wantCheck:    "compilability",
			wantOffender: "FR-99",
		},
		{
			name:         "(f) a design build-on cycle reds malformed naming the cycle",
			fixture:      "compil-buildon-cycle",
			feature:      "demo",
			want:         ClassMalformed,
			wantCheck:    "compilability",
			wantOffender: "build-on cycle",
		},
		{
			name:    "a fully well-formed contract is clean",
			fixture: "wellformed",
			feature: "demo",
			want:    ClassClean,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			v := runFixture(t, tc.fixture, tc.feature)

			if v.Class != tc.want {
				t.Fatalf("class = %q, want %q (reason: %q; results: %+v)", v.Class, tc.want, v.Reason, v.CheckResults)
			}
			// The verdict validates closed on every path (design §3.11).
			if d := v.Validate(); len(d) != 0 {
				t.Errorf("verdict does not validate closed: %v", d)
			}

			if tc.want == ClassClean {
				for _, r := range v.CheckResults {
					if !r.Passed {
						t.Errorf("clean verdict has a failing check %q: %+v", r.Check, r.Defects)
					}
				}
				return
			}

			// The named check failed, carries its declared class, and its defects
			// name the offender (FR-8: every defect names check-id + offender).
			var found *CheckResult
			for i := range v.CheckResults {
				if v.CheckResults[i].Check == tc.wantCheck {
					found = &v.CheckResults[i]
				}
			}
			if found == nil {
				t.Fatalf("no CheckResult for %q (results: %+v)", tc.wantCheck, v.CheckResults)
			}
			if found.Passed {
				t.Fatalf("check %q passed, expected it to red the fixture", tc.wantCheck)
			}
			if found.Class != tc.want {
				t.Errorf("check %q class = %q, want %q", tc.wantCheck, found.Class, tc.want)
			}
			if len(found.Defects) == 0 {
				t.Fatalf("failing check %q named zero defects", tc.wantCheck)
			}
			named := false
			for _, d := range found.Defects {
				if d.Verb != "contract-gate" {
					t.Errorf("defect verb = %q, want contract-gate", d.Verb)
				}
				if d.Op != tc.wantCheck {
					t.Errorf("defect op = %q, want %q", d.Op, tc.wantCheck)
				}
				if d.Remedy == "" {
					t.Errorf("defect names no unblock (remedy): %+v", d)
				}
				if strings.Contains(d.Reason, tc.wantOffender) {
					named = true
				}
			}
			if !named {
				t.Errorf("no defect names offender %q; defects: %+v", tc.wantOffender, found.Defects)
			}
		})
	}
}

// TestReusedChecks table-drives the three T4 reused checks — currency,
// inverse-coverage, link — over per-defect-class fixtures (design contract-gate-v1
// §3.9 / plan T4). Each verifies the class, the named failing check, and that the
// offender (design + anchor / Must / dangling citation + location) is named. It
// also asserts isolation: a fixture seeded for one reused check reds ONLY that
// check, so a scoped check does not leak into another (and the T3 checks stay
// clean too). UB-5 (a PRD with no live serving design) reds `incomplete`.
func TestReusedChecks(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		feature string
		want    Class
		// wantCheck is the CheckResult.Check expected to fail.
		wantCheck string
		// wantOffenders are substrings the failing check's defects must name (all).
		wantOffenders []string
	}{
		{
			name:          "(a) a live serving design serving a superseded requirement reds stale naming design + anchor (FR-2)",
			fixture:       "stale-serve",
			feature:       "demo",
			want:          ClassStale,
			wantCheck:     "currency",
			wantOffenders: []string{"demo-v1", "prd:demo/FR-2"},
		},
		{
			name:          "(b) a live Must served by no live design reds incomplete naming the Must (FR-3)",
			fixture:       "uncovered-must",
			feature:       "demo",
			want:          ClassIncomplete,
			wantCheck:     "inverse-coverage",
			wantOffenders: []string{"FR-1"},
		},
		{
			name:      "(c) a dangling body citation reds unresolved naming the offender + location (FR-4)",
			fixture:   "dangling-citation",
			feature:   "demo",
			want:      ClassUnresolved,
			wantCheck: "link",
			// the offender token, its source file, and a line reference.
			wantOffenders: []string{"design:ghost-design", "000-base.md", "line "},
		},
		{
			name:          "(d) UB-5: a PRD with no live serving design reds incomplete (every Must unserved)",
			fixture:       "no-designs",
			feature:       "demo",
			want:          ClassIncomplete,
			wantCheck:     "inverse-coverage",
			wantOffenders: []string{"FR-1"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			v := runFixture(t, tc.fixture, tc.feature)

			if v.Class != tc.want {
				t.Fatalf("class = %q, want %q (reason: %q; results: %+v)", v.Class, tc.want, v.Reason, v.CheckResults)
			}
			if d := v.Validate(); len(d) != 0 {
				t.Errorf("verdict does not validate closed: %v", d)
			}

			// Exactly the target check red; every other check passed (isolation —
			// a scoped reused check must not leak, NR-2/NR-3).
			var target *CheckResult
			for i := range v.CheckResults {
				r := &v.CheckResults[i]
				if r.Check == tc.wantCheck {
					target = r
					continue
				}
				if !r.Passed {
					t.Errorf("unrelated check %q also red (class %q): %+v", r.Check, r.Class, r.Defects)
				}
			}
			if target == nil {
				t.Fatalf("no CheckResult for %q (results: %+v)", tc.wantCheck, v.CheckResults)
			}
			if target.Passed {
				t.Fatalf("check %q passed, expected it to red the fixture", tc.wantCheck)
			}
			if target.Class != tc.want {
				t.Errorf("check %q class = %q, want %q", tc.wantCheck, target.Class, tc.want)
			}
			if len(target.Defects) == 0 {
				t.Fatalf("failing check %q named zero defects", tc.wantCheck)
			}

			// Every defect is a well-formed contract-scope defect (FR-8 shape).
			for _, d := range target.Defects {
				if d.Verb != "contract-gate" {
					t.Errorf("defect verb = %q, want contract-gate", d.Verb)
				}
				if d.Op != tc.wantCheck {
					t.Errorf("defect op = %q, want %q", d.Op, tc.wantCheck)
				}
				if d.Remedy == "" {
					t.Errorf("defect names no unblock (remedy): %+v", d)
				}
			}
			// Every required offender substring appears across the defect set.
			joined := ""
			for _, d := range target.Defects {
				joined += d.File + " " + d.Reason + "\n"
			}
			for _, off := range tc.wantOffenders {
				if !strings.Contains(joined, off) {
					t.Errorf("offender %q not named; defects:\n%s", off, joined)
				}
			}
		})
	}
}

// TestReusedChecksCleanOnWellformed proves the three T4 reused checks pass on the
// well-formed fixture (their `clean` case): a fully-current, fully-covered,
// fully-resolvable contract reds none of currency, inverse-coverage, or link.
func TestReusedChecksCleanOnWellformed(t *testing.T) {
	v := runFixture(t, "wellformed", "demo")
	if !v.Clean() {
		t.Fatalf("well-formed fixture is not clean: %q — %s", v.Class, v.Reason)
	}
	reused := map[string]bool{"currency": true, "inverse-coverage": true, "link": true}
	seen := map[string]bool{}
	for _, r := range v.CheckResults {
		if reused[r.Check] {
			seen[r.Check] = true
			if !r.Passed {
				t.Errorf("reused check %q red on the well-formed fixture: %+v", r.Check, r.Defects)
			}
		}
	}
	for name := range reused {
		if !seen[name] {
			t.Errorf("reused check %q did not run on the well-formed fixture", name)
		}
	}
}

// TestAPTwentySevenSelfRegistration proves the DCR contract (design §3.9 / A-6,
// AP-27): the five contract-scope checks are self-registered, each contributed
// from its OWN file (adding a check touches no shared enumeration), and the
// registry assembles ALL of them deterministically by name. The per-file
// assertion reads each check's Run source file via runtime reflection — the
// machine-checkable form of "one new file, no shared-line edit". The two T3
// checks (compilability, structural-completeness) plus the three T4 reused
// checks (currency, inverse-coverage, link) are all present and distinct-filed.
func TestAPTwentySevenSelfRegistration(t *testing.T) {
	checks := contractScopeChecks()

	// Deterministic assembly: sorted by name (DCR DINV-1), never init order.
	wantOrder := []string{"compilability", "currency", "inverse-coverage", "link", "structural-completeness"}
	if got := names(checks); !reflect.DeepEqual(got, wantOrder) {
		t.Fatalf("assembly order = %v, want %v (sorted by name)", got, wantOrder)
	}

	// Each check's class maps to the right verdict class.
	byName := map[string]Class{}
	for _, c := range checks {
		byName[c.Name] = c.Class
	}
	wantClass := map[string]Class{
		"structural-completeness": ClassIncomplete,
		"compilability":           ClassMalformed,
		"currency":                ClassStale,
		"inverse-coverage":        ClassIncomplete,
		"link":                    ClassUnresolved,
	}
	for name, want := range wantClass {
		if byName[name] != want {
			t.Errorf("%s class = %q, want %q", name, byName[name], want)
		}
	}

	// Each check lands in its own reg_check_*.go file, and all five files are
	// distinct — the AP-27 "one new file, touching no shared enumeration"
	// property, verified per check.
	files := map[string]string{}
	seen := map[string]string{}
	for _, c := range checks {
		file := runFuncFile(c.Run)
		base := filepath.Base(file)
		if !strings.HasPrefix(base, "reg_check_") || !strings.HasSuffix(base, ".go") {
			t.Errorf("check %q lives in %q, not a reg_check_*.go self-registration file", c.Name, base)
		}
		if other, dup := seen[base]; dup {
			t.Errorf("checks %q and %q share the file %q — each must be one self-contained file (AP-27)", c.Name, other, base)
		}
		seen[base] = c.Name
		files[c.Name] = base
	}
}

// TestRegisterRejectsDuplicate proves a duplicate check name is a hard failure,
// never a silent shadow (DCR DINV-4): Register panics on a name already present.
func TestRegisterRejectsDuplicate(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Register accepted a duplicate name — DINV-4 requires a hard failure, not a silent overwrite")
		}
	}()
	// "compilability" is already registered by its init(); re-registering panics.
	Register(Check{Name: "compilability", Class: ClassMalformed, Run: func(*contract) Defects { return nil }})
}

func names(cs []Check) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.Name
	}
	return out
}

// runFuncFile returns the source file a func value was defined in.
func runFuncFile(fn func(c *contract) Defects) string {
	pc := reflect.ValueOf(fn).Pointer()
	f := runtime.FuncForPC(pc)
	if f == nil {
		return ""
	}
	file, _ := f.FileLine(pc)
	return file
}
