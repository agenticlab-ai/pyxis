package contractgate

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
)

// runTrig runs the verb against a testdata fixture under an explicit trigger and
// severity map, snapshotting the tree before/after to enforce zero mutation
// (DINV-1) — the in-process split seam (design §3.10/§3.11).
func runTrig(t *testing.T, fixture, feature string, trig Trigger, sev SeverityMap) Verdict {
	t.Helper()
	root := filepath.Join("testdata", fixture)
	before := snapshot(t, root)
	v := RunWithConfigTrigger(root, feature, trig, config.Defaults(), sev)
	after := snapshot(t, root)
	assertUnchanged(t, before, after)
	return v
}

// TestTriggerSeveritySplit is the heart of T6 (plan T6 acceptance #1; FR-9): the
// SAME check subset runs at both triggers, but the per-trigger severity map
// decides whether a failed check BLOCKS (exit 1) or is merely surfaced as
// advisory (exit 0). It table-drives every DoD case: a currency/link/structural
// defect blocks on contract-doc-pr; an uncovered-Must-only bounce is advisory
// (non-blocking) on contract-doc-pr but BLOCKS on planning-entry; a clean
// contract blocks nowhere.
func TestTriggerSeveritySplit(t *testing.T) {
	def := defaultSeverityMap()
	tests := []struct {
		name         string
		fixture      string
		trigger      Trigger
		wantClass    Class // the most-severe FAILED class (names what is wrong)
		wantClean    bool
		wantBlocked  bool     // the exit-code outcome
		wantFailed   string   // the check expected to red (empty on clean)
		wantSeverity Severity // that check's disposition at this trigger
	}{
		{
			name:         "a structural defect BLOCKS on contract-doc-pr",
			fixture:      "struct-nofit",
			trigger:      TriggerContractDocPR,
			wantClass:    ClassIncomplete,
			wantBlocked:  true,
			wantFailed:   "structural-completeness",
			wantSeverity: SeverityBlocking,
		},
		{
			name:         "a currency defect BLOCKS on contract-doc-pr",
			fixture:      "stale-serve",
			trigger:      TriggerContractDocPR,
			wantClass:    ClassStale,
			wantBlocked:  true,
			wantFailed:   "currency",
			wantSeverity: SeverityBlocking,
		},
		{
			name:         "a link defect BLOCKS on contract-doc-pr",
			fixture:      "dangling-citation",
			trigger:      TriggerContractDocPR,
			wantClass:    ClassUnresolved,
			wantBlocked:  true,
			wantFailed:   "link",
			wantSeverity: SeverityBlocking,
		},
		{
			name:         "an uncovered-Must-only bounce is ADVISORY (non-blocking) on contract-doc-pr",
			fixture:      "uncovered-must",
			trigger:      TriggerContractDocPR,
			wantClass:    ClassIncomplete, // the class still names the gap…
			wantBlocked:  false,           // …but it does NOT stop the run (FR-9)
			wantFailed:   "inverse-coverage",
			wantSeverity: SeverityAdvisory,
		},
		{
			name:         "the SAME uncovered Must BLOCKS on planning-entry",
			fixture:      "uncovered-must",
			trigger:      TriggerPlanningEntry,
			wantClass:    ClassIncomplete,
			wantBlocked:  true, // every class is blocking at planning entry
			wantFailed:   "inverse-coverage",
			wantSeverity: SeverityBlocking,
		},
		{
			name:         "UB-5 no-designs is advisory on contract-doc-pr (coverage-only)",
			fixture:      "no-designs",
			trigger:      TriggerContractDocPR,
			wantClass:    ClassIncomplete,
			wantBlocked:  false,
			wantFailed:   "inverse-coverage",
			wantSeverity: SeverityAdvisory,
		},
		{
			name:        "a clean contract blocks nowhere (planning-entry)",
			fixture:     "wellformed",
			trigger:     TriggerPlanningEntry,
			wantClass:   ClassClean,
			wantClean:   true,
			wantBlocked: false,
		},
		{
			name:        "a clean contract blocks nowhere (contract-doc-pr)",
			fixture:     "wellformed",
			trigger:     TriggerContractDocPR,
			wantClass:   ClassClean,
			wantClean:   true,
			wantBlocked: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			v := runTrig(t, tc.fixture, "demo", tc.trigger, def)

			if v.Trigger != tc.trigger {
				t.Errorf("verdict trigger = %q, want %q", v.Trigger, tc.trigger)
			}
			if v.Class != tc.wantClass {
				t.Errorf("class = %q, want %q (reason: %q)", v.Class, tc.wantClass, v.Reason)
			}
			if v.Clean() != tc.wantClean {
				t.Errorf("Clean() = %v, want %v", v.Clean(), tc.wantClean)
			}
			if v.Blocked() != tc.wantBlocked {
				t.Errorf("Blocked() = %v, want %v (reason: %q)", v.Blocked(), tc.wantBlocked, v.Reason)
			}
			if v.Blocking != tc.wantBlocked {
				t.Errorf("verdict.Blocking field = %v, want %v", v.Blocking, tc.wantBlocked)
			}
			// The verdict validates closed on every trigger (design §3.11).
			if d := v.Validate(); len(d) != 0 {
				t.Errorf("verdict does not validate closed: %v", d)
			}
			if tc.wantClean {
				return
			}

			// The failed check carries the expected per-trigger severity, and its
			// Blocking()/Advisory() dispositions agree.
			var r *CheckResult
			for i := range v.CheckResults {
				if v.CheckResults[i].Check == tc.wantFailed {
					r = &v.CheckResults[i]
				}
			}
			if r == nil {
				t.Fatalf("no CheckResult for %q (results: %+v)", tc.wantFailed, v.CheckResults)
			}
			if r.Passed {
				t.Fatalf("check %q passed, expected it to red the fixture", tc.wantFailed)
			}
			if r.Severity != tc.wantSeverity {
				t.Errorf("check %q severity = %q, want %q", tc.wantFailed, r.Severity, tc.wantSeverity)
			}
			if r.Blocking() != (tc.wantSeverity == SeverityBlocking) {
				t.Errorf("check %q Blocking() = %v, want %v", tc.wantFailed, r.Blocking(), tc.wantSeverity == SeverityBlocking)
			}
			if r.Advisory() != (tc.wantSeverity == SeverityAdvisory) {
				t.Errorf("check %q Advisory() = %v, want %v", tc.wantFailed, r.Advisory(), tc.wantSeverity == SeverityAdvisory)
			}
			// An advisory-only bounce is NOT clean (it carries a defect) yet is
			// non-blocking — the exact FR-9 property.
			if tc.wantSeverity == SeverityAdvisory && v.Clean() {
				t.Errorf("advisory-only bounce reported clean — the defect must still be surfaced")
			}
		})
	}
}

// TestPlanningEntryUnchangedFromDefault proves the DEFAULT trigger (planning-
// entry) preserves T5's behavior exactly: every defect class blocks, so a
// per-defect-class fixture BLOCKS just as it did before the split existed (the
// verdict class is unchanged and the run fails). This is the "planning-entry
// behavior stays" guarantee.
func TestPlanningEntryUnchangedFromDefault(t *testing.T) {
	def := defaultSeverityMap()
	cases := []struct {
		fixture string
		class   Class
	}{
		{"struct-nofit", ClassIncomplete},
		{"stale-serve", ClassStale},
		{"dangling-citation", ClassUnresolved},
		{"uncovered-must", ClassIncomplete},
		{"compil-orphan-serves", ClassMalformed},
	}
	for _, c := range cases {
		t.Run(c.fixture, func(t *testing.T) {
			v := runTrig(t, c.fixture, "demo", TriggerPlanningEntry, def)
			if v.Class != c.class {
				t.Errorf("class = %q, want %q", v.Class, c.class)
			}
			if !v.Blocked() {
				t.Errorf("planning-entry did not block a %q defect — every class must block at planning entry", c.class)
			}
			// Cross-check against the T5 back-compat entry (RunWithConfig, default
			// trigger + default map): identical class, and non-clean.
			legacy := runFixture(t, c.fixture, "demo")
			if legacy.Class != v.Class {
				t.Errorf("RunWithConfig class %q != RunWithConfigTrigger planning-entry class %q (behavior drifted)", legacy.Class, v.Class)
			}
			if legacy.Clean() {
				t.Errorf("RunWithConfig reported a %q fixture clean", c.fixture)
			}
		})
	}
}

// TestSeverityMapFlipsOutcomeNoCode is the inspection-style DoD test (plan T6
// acceptance #3; FR-13): editing ONLY the severity-map DATA flips a fixture's
// blocking/advisory outcome with ZERO code change. The uncovered-Must fixture is
// advisory (non-blocking) under the default contract-doc-pr split; overlaying a
// policy that promotes coverage to blocking makes the SAME contract BLOCK — no
// code path differs between the two runs, only the SeverityMap value.
func TestSeverityMapFlipsOutcomeNoCode(t *testing.T) {
	// The default split: coverage is advisory on contract-doc-pr → non-blocking.
	advisoryMap := defaultSeverityMap()
	// A data-only edit: promote coverage to blocking on contract-doc-pr. Parsed
	// from policy TEXT so the test exercises the real closed parser, and proves a
	// severity is a DATA value — not a constant baked into a code branch.
	blockingMap, defects := ParseSeverityMap([]byte("schema_version: \"1\"\ntriggers:\n  contract-doc-pr:\n    coverage: blocking\n"))
	if len(defects) != 0 {
		t.Fatalf("the flip policy did not parse: %v", defects)
	}

	// SAME fixture, SAME trigger, SAME code — only the map differs.
	adv := runTrig(t, "uncovered-must", "demo", TriggerContractDocPR, advisoryMap)
	blk := runTrig(t, "uncovered-must", "demo", TriggerContractDocPR, blockingMap)

	if adv.Blocked() {
		t.Errorf("under the default split the uncovered-Must bounce blocked — expected advisory (non-blocking)")
	}
	if !blk.Blocked() {
		t.Errorf("after promoting coverage to blocking the SAME bounce did not block — the data edit had no effect")
	}
	// The class is identical across both runs (the defect is the same); ONLY the
	// blocking disposition flipped — the FR-13 property, proven by data alone.
	if adv.Class != blk.Class {
		t.Errorf("class differed across the flip (%q vs %q) — only the severity should change", adv.Class, blk.Class)
	}
	// And the per-check severity is exactly what the two policies say.
	if s := severityOf(adv, "inverse-coverage"); s != SeverityAdvisory {
		t.Errorf("default coverage severity on contract-doc-pr = %q, want advisory", s)
	}
	if s := severityOf(blk, "inverse-coverage"); s != SeverityBlocking {
		t.Errorf("promoted coverage severity on contract-doc-pr = %q, want blocking", s)
	}
}

// TestSeverityMapFileDrivesFlip proves the same flip through the FILE-loading
// path (LoadSeverityMap): a fixture carrying a .pyxis/severity-map.yaml that
// promotes coverage to blocking makes the uncovered-Must contract BLOCK on
// contract-doc-pr via the production Run path, while the same fixture WITHOUT the
// file (uncovered-must) stays advisory. The demotion/promotion lever is a repo
// file, not a code change.
func TestSeverityMapFileDrivesFlip(t *testing.T) {
	// No policy file → default split → coverage advisory → non-blocking.
	absent := RunTrigger(filepath.Join("testdata", "uncovered-must"), "demo", TriggerContractDocPR)
	if absent.Blocked() {
		t.Errorf("uncovered-must with no policy file blocked on contract-doc-pr — the default split makes coverage advisory")
	}
	// A committed policy file promoting coverage to blocking → the SAME contract
	// now blocks. The only difference is the presence of the data file.
	present := RunTrigger(filepath.Join("testdata", "sevmap-coverage-blocking"), "demo", TriggerContractDocPR)
	if !present.Blocked() {
		t.Errorf("sevmap-coverage-blocking did not block on contract-doc-pr — the committed policy promoting coverage had no effect")
	}
	if absent.Class != present.Class {
		t.Errorf("class differed across the file-driven flip (%q vs %q)", absent.Class, present.Class)
	}
}

// TestMalformedSeverityMapIsConfigurationError proves UB-6's fail-closed edge: a
// malformed severity-map (an unknown check class here) yields a configuration-
// error verdict naming the policy defect, with ZERO checks run — the gate never
// proceeds on a broken policy. This runs through the production Run path
// (LoadSeverityMap reads the fixture's .pyxis/severity-map.yaml).
func TestMalformedSeverityMapIsConfigurationError(t *testing.T) {
	root := filepath.Join("testdata", "sevmap-malformed")
	before := snapshot(t, root)
	v := RunTrigger(root, "demo", TriggerContractDocPR)
	after := snapshot(t, root)
	assertUnchanged(t, before, after) // read-only even on the fail-closed edge

	if v.Class != ClassConfigurationError {
		t.Fatalf("class = %q, want configuration-error (UB-6)", v.Class)
	}
	if !v.Blocked() {
		t.Errorf("a configuration-error verdict must block the run")
	}
	if v.Clean() {
		t.Errorf("a malformed policy reported clean — UB-6 forbids it")
	}
	// Zero real checks run on the fail-closed edge (only the resolve pseudo-result).
	for _, r := range v.CheckResults {
		if r.Check != "resolve" {
			t.Errorf("a real check %q ran despite a malformed policy — zero checks must run (UB-6)", r.Check)
		}
	}
	// The verdict names the policy defect (the reason points at the severity map).
	if !strings.Contains(v.Reason, "severity-map") {
		t.Errorf("configuration-error reason %q does not name the severity-map defect", v.Reason)
	}
	if d := v.Validate(); len(d) != 0 {
		t.Errorf("configuration-error verdict does not validate closed: %v", d)
	}
}

// TestAbsentSeverityMapUsesDefault proves UB-6's other half: a MISSING policy
// file never fails — it runs on the built-in default split. Every existing
// fixture already has no policy file, so LoadSeverityMap returns the default with
// no defects; here we assert that explicitly and that the default split is the
// #D-4 seed (planning-entry all-blocking; contract-doc-pr coverage advisory).
func TestAbsentSeverityMapUsesDefault(t *testing.T) {
	m, defects := LoadSeverityMap(t.TempDir()) // an empty dir has no policy file
	if len(defects) != 0 {
		t.Fatalf("a missing policy file produced defects (must fall back silently): %v", defects)
	}
	// The default is the #D-4 split.
	assertDefaultSplit(t, m)
	// And a well-formed contract runs clean under it through the production path.
	v := RunTrigger(filepath.Join("testdata", "wellformed"), "demo", TriggerContractDocPR)
	if !v.Clean() {
		t.Errorf("wellformed on the default split is not clean: %q — %s", v.Class, v.Reason)
	}
}

// TestAdvisoryBounceStillEmitsReport proves an advisory-only bounce is RECORDED
// (§3.10/FR-9: advisory defects are reported in the verdict AND the report,
// they just do not block): the uncovered-Must fixture under contract-doc-pr is
// non-blocking yet emits both dual-face report files, and the report carries the
// trigger, blocking=false, and the per-check advisory severity. The read-only
// audit (the fixture is unchanged; the report lands only in the sink) holds.
func TestAdvisoryBounceStillEmitsReport(t *testing.T) {
	root := filepath.Join("testdata", "uncovered-must")
	sink := t.TempDir()

	before := snapshot(t, root)
	v := RunWithConfigTrigger(root, "demo", TriggerContractDocPR, config.Defaults(), defaultSeverityMap())
	paths, err := EmitReport(sink, v)
	after := snapshot(t, root)
	assertUnchanged(t, before, after) // gate wrote nothing under the contract inputs

	if err != nil {
		t.Fatalf("EmitReport: %v", err)
	}
	if v.Clean() || v.Blocked() {
		t.Fatalf("precondition: want an advisory-only (non-clean, non-blocking) bounce; got clean=%v blocked=%v", v.Clean(), v.Blocked())
	}
	if len(paths) != 2 {
		t.Fatalf("an advisory-only bounce emitted %d report faces, want 2 (its defects must be recorded)", len(paths))
	}

	// The report struct records the trigger, the non-blocking disposition, and the
	// failed check's advisory severity — the surfaced-but-not-blocking evidence.
	r, ok := ReportForVerdict(v)
	if !ok {
		t.Fatal("ReportForVerdict returned no report for an advisory bounce")
	}
	if r.Trigger != TriggerContractDocPR {
		t.Errorf("report trigger = %q, want contract-doc-pr", r.Trigger)
	}
	if r.Blocking {
		t.Errorf("report blocking = true for an advisory-only bounce")
	}
	sawAdvisory := false
	for _, c := range r.Checks {
		if c.Check == "inverse-coverage" {
			if c.Severity != SeverityAdvisory {
				t.Errorf("inverse-coverage report severity = %q, want advisory", c.Severity)
			}
			sawAdvisory = true
		}
	}
	if !sawAdvisory {
		t.Errorf("the advisory inverse-coverage defect is not enumerated in the report")
	}
}

// severityOf returns a check's recorded severity in a verdict (empty if absent).
func severityOf(v Verdict, check string) Severity {
	for _, r := range v.CheckResults {
		if r.Check == check {
			return r.Severity
		}
	}
	return ""
}

// assertDefaultSplit asserts a map encodes the #D-4 default: every class blocking
// at planning-entry; on contract-doc-pr all blocking EXCEPT inverse-coverage,
// which is advisory (a PRD merges before its designs exist).
func assertDefaultSplit(t *testing.T, m SeverityMap) {
	t.Helper()
	for _, class := range severityMapClasses() {
		if !m.Blocking(TriggerPlanningEntry, class) {
			t.Errorf("default: %q is not blocking at planning-entry (every class must block there)", class)
		}
	}
	for _, class := range severityMapClasses() {
		want := SeverityBlocking
		if class == SeverityClassCoverage {
			want = SeverityAdvisory
		}
		if got := m.Severity(TriggerContractDocPR, class); got != want {
			t.Errorf("default: %q on contract-doc-pr = %q, want %q (#D-4 split)", class, got, want)
		}
	}
}
