package floor_test

import (
	"context"
	"slices"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/implreview/floor"
	"github.com/agenticlab-ai/pyxis/internal/implreview/report"
	"github.com/agenticlab-ai/pyxis/internal/implreview/runtime"
	"github.com/agenticlab-ai/pyxis/internal/verify/schema"
)

// TestEnforcedClassesReadFromCodegate — the enforced set is READ from codegate's
// live registry (not guessed): it contains the registered check IDs (format, lint,
// build, exec-probe, …) and the finer finding classes those checks emit (secret,
// forbidden-binary, test-deleted, …). This is the EVAL-4 basis: the set the floor
// screens against is codegate's own, read read-only.
func TestEnforcedClassesReadFromCodegate(t *testing.T) {
	t.Parallel()
	set := floor.EnforcedClasses()
	if len(set) == 0 {
		t.Fatal("enforced class set is empty — codegate checks did not register (missing import side effect?)")
	}

	// Check IDs (read from registry.AllChecks via the import side effect).
	for _, id := range []string{"format", "lint", "build", "exec-probe", "secret-scan", "forbidden-artifacts", "machinery-protection", "test-delta"} {
		if !set[id] {
			t.Errorf("enforced set missing registered check id %q", id)
		}
	}
	// Finer finding classes (the mirrored, source-guarded set).
	for _, cl := range []string{"secret", "forbidden-binary", "forbidden-size", "forbidden-path", "machinery", "test-deleted", "test-renamed-out", "skip-marker-added", "assertion-count-drop"} {
		if !set[cl] {
			t.Errorf("enforced set missing finding class %q", cl)
		}
	}
}

func TestFlaggedClassesNilVerdictIsEmpty(t *testing.T) {
	t.Parallel()
	if got := floor.FlaggedClasses(nil); got != nil {
		t.Fatalf("FlaggedClasses(nil) = %v", got)
	}
	if got := floor.EnforcedClassList(); len(got) == 0 || !slices.IsSorted(got) {
		t.Fatalf("EnforcedClassList() = %v", got)
	}
}

// judgmentFinding is a genuine impl-review finding whose class-token does NOT
// collide with any codegate class — the kind the lenses SHOULD keep.
func judgmentFinding(id, lens, ruleClass, calRow, severity string) report.Finding {
	return report.Finding{
		ID:         id,
		Message:    "a genuine judgment defect: " + id,
		RuleID:     lens + "/" + ruleClass,
		Lens:       lens,
		Severity:   severity,
		CalRow:     calRow,
		Acceptance: "A-1",
	}
}

// TestFloorDropsRestatementKeepsJudgment — the core FR-4 behavior: a candidate
// whose class-token IS a codegate-enforced class is dropped; a genuine judgment
// finding (whose class-token collides with none) is kept.
func TestFloorDropsRestatementKeepsJudgment(t *testing.T) {
	t.Parallel()
	s := floor.NewScreener()

	cases := []struct {
		name     string
		finding  report.Finding
		survives bool
	}{
		{
			name:     "restates a check-id class (format)",
			finding:  judgmentFinding("F1", report.LensDesignConformance, "format", "CAL-6", report.SeverityAdvisory),
			survives: false,
		},
		{
			name:     "restates a check-id class (build)",
			finding:  judgmentFinding("F2", report.LensCorrectnessCritic, "build", "CAL-11", report.SeverityBlocking),
			survives: false,
		},
		{
			name:     "restates a finding class (secret)",
			finding:  judgmentFinding("F3", report.LensSecurityAbuse, "secret", "CAL-8", report.SeverityBlocking),
			survives: false,
		},
		{
			name:     "restates a finding class (test-deleted)",
			finding:  judgmentFinding("F4", report.LensCorrectnessCritic, "test-deleted", "CAL-11", report.SeverityBlocking),
			survives: false,
		},
		{
			name:     "genuine spec-fidelity judgment (revision-presence)",
			finding:  judgmentFinding("F5", report.LensSpecFidelity, "revision-presence", "CAL-1", report.SeverityBlocking),
			survives: true,
		},
		{
			name:     "genuine correctness judgment (nil-deref)",
			finding:  judgmentFinding("F6", report.LensCorrectnessCritic, "nil-deref-on-empty", "CAL-11", report.SeverityBlocking),
			survives: true,
		},
		{
			name:     "genuine security judgment whose token merely CONTAINS a class word",
			finding:  judgmentFinding("F7", report.LensSecurityAbuse, "secret-derived-from-env", "CAL-8", report.SeverityBlocking),
			survives: true, // class-token is "secret-derived-from-env", NOT "secret" — not a restatement
		},
		{
			name: "malformed bare enforced-class RuleID is still floored",
			// A RuleID of just "secret" (no <lens>/ prefix — malformed) must not slip
			// past the floor: the whole bare string is treated as the class token.
			finding:  report.Finding{ID: "F8", Message: "m", RuleID: "secret", Lens: report.LensSecurityAbuse, Severity: report.SeverityBlocking, CalRow: "CAL-8", Acceptance: "A-1"},
			survives: false,
		},
		{
			name:     "restates a parametric runner class (timeout)",
			finding:  judgmentFinding("F9", report.LensCorrectnessCritic, "timeout", "CAL-11", report.SeverityBlocking),
			survives: false, // "timeout" is a codegate execution-fault class (runner enum) — a restatement
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := s.Survives(tc.finding.Lens, tc.finding)
			if got != tc.survives {
				t.Fatalf("Survives(%q) = %v, want %v (restated class %q)",
					tc.finding.RuleID, got, tc.survives, floor.RestatedClass(tc.finding))
			}
		})
	}
}

// TestRestatedClass — the deterministic class-token extraction (the EVAL-4
// comparator's key): the token after the last "/", or "" when there is none.
func TestRestatedClass(t *testing.T) {
	t.Parallel()
	cases := []struct {
		ruleID string
		want   string
	}{
		{"security-abuse/secret", "secret"},
		{"design-conformance/format", "format"},
		{"spec-fidelity/revision-presence", "revision-presence"},
		{"correctness-critic/closed-parse", "closed-parse"},
		{"no-slash-here", "no-slash-here"}, // malformed identity: the whole bare string is the class token
		{"secret", "secret"},               // a bare enforced-class name must still be catchable
		{"trailing-slash/", ""},            // empty class token
		{"", ""},
		{"a/b/format", "format"}, // last segment governs
	}
	for _, tc := range cases {
		got := floor.RestatedClass(report.Finding{RuleID: tc.ruleID})
		if got != tc.want {
			t.Errorf("RestatedClass(%q) = %q, want %q", tc.ruleID, got, tc.want)
		}
	}
}

// TestFloorScreenerIsRuntimeScreener — the floor Screener plugs into T4's runtime
// as a runtime.Screener with NO edit to the runtime: it composes with the runtime's
// DefaultScreener and the runtime's own per-candidate loop drives both, dropping a
// floor restatement before it reaches a round.
func TestFloorScreenerIsRuntimeScreener(t *testing.T) {
	t.Parallel()

	// The floor Screener satisfies runtime.Screener structurally.
	var _ runtime.Screener = floor.NewScreener()

	// Compose the floor with the runtime's structural default; the composite is a
	// runtime.Screener the runtime drives unchanged.
	composed := floor.Compose(floor.NewScreener(), runtime.DefaultScreener{})

	restatement := judgmentFinding("F1", report.LensDesignConformance, "lint", "CAL-6", report.SeverityAdvisory)
	genuine := judgmentFinding("F2", report.LensSpecFidelity, "charter-must-unmet", "CAL-1", report.SeverityBlocking)
	// A candidate the DefaultScreener kills (no message) — proves BOTH screens run.
	malformed := report.Finding{ID: "F3", Lens: report.LensCorrectnessCritic, RuleID: "correctness-critic/x", CalRow: "CAL-11", Severity: report.SeverityBlocking}

	if composed.Survives(restatement.Lens, restatement) {
		t.Error("composed screen let a floor restatement survive")
	}
	if !composed.Survives(genuine.Lens, genuine) {
		t.Error("composed screen killed a genuine judgment finding")
	}
	if composed.Survives(malformed.Lens, malformed) {
		t.Error("composed screen let a message-less candidate survive (DefaultScreener leg)")
	}
}

// TestComposeNilPanics — a nil screen would silently pass everything, defeating the
// floor; Compose refuses it.
func TestComposeNilPanics(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("Compose(nil, …) did not panic")
		}
	}()
	floor.Compose(nil, floor.NewScreener())
}

// TestFloorExclusionYieldsZeroFindings is the EVAL-4 basis: a review whose lenses
// emit ONLY candidates restating codegate-catchable defect classes yields ZERO
// findings in the floor set. It drives the real runtime.Review with a fake
// LensRunner seeded with floor restatements, screened by the floor Screener — the
// assembled round has an empty finding set, and every restatement is counted as
// killed. (The full eval suite is T12; this is the mechanism's unit basis.)
func TestFloorExclusionYieldsZeroFindings(t *testing.T) {
	t.Parallel()

	// Each lens "finds" only floor restatements — the defects codegate already
	// catches, which impl-review must NOT re-report.
	seeded := map[string][]report.Finding{
		report.LensSpecFidelity:      {judgmentFinding("A", report.LensSpecFidelity, "format", "CAL-1", report.SeverityBlocking)},
		report.LensDesignConformance: {judgmentFinding("B", report.LensDesignConformance, "lint", "CAL-4", report.SeverityBlocking)},
		report.LensSecurityAbuse:     {judgmentFinding("C", report.LensSecurityAbuse, "secret", "CAL-8", report.SeverityBlocking)},
		report.LensCorrectnessCritic: {judgmentFinding("D", report.LensCorrectnessCritic, "build", "CAL-11", report.SeverityBlocking)},
	}

	req := runtime.ReviewRequest{
		ReviewerRoot:     "reviewer-root-eval4",
		ImplementerRoot:  "implementer-root-eval4",
		Pack:             []byte("shared review context pack"),
		ReviewedRevision: "cafebabe",
		Instructions: map[string]string{
			report.LensSpecFidelity:      "spec instruction",
			report.LensDesignConformance: "design instruction",
			report.LensSecurityAbuse:     "security instruction",
			report.LensCorrectnessCritic: "critic instruction",
		},
		Provenance: report.Provenance{Model: "m", PromptsVersion: "p", CalVersion: "impl-review-calibration/v1"},
		Runner:     &seededRunner{seeded: seeded},
		// The floor screen composed with the runtime's structural default.
		Screener: floor.Compose(floor.NewScreener(), runtime.DefaultScreener{}),
	}

	res, err := runtime.Review(context.Background(), req)
	if err != nil {
		t.Fatalf("Review: %v", err)
	}
	if got := len(res.Round.Findings); got != 0 {
		t.Fatalf("floor set should be EMPTY (every candidate restates a ladder class); got %d findings: %+v",
			got, res.Round.Findings)
	}
	if res.Round.FindingsCount != 0 {
		t.Fatalf("findings_count = %d, want 0", res.Round.FindingsCount)
	}
	if res.Round.Candidates != 4 {
		t.Fatalf("candidates = %d, want 4 (one restatement per lens)", res.Round.Candidates)
	}
	if res.Round.Killed != 4 {
		t.Fatalf("killed = %d, want 4 (every restatement dropped by the floor)", res.Round.Killed)
	}
}

// seededRunner is a fake LensRunner returning a fixed candidate set per lens — no
// network, no real subagent (the runtime's own test harness pattern).
type seededRunner struct {
	seeded map[string][]report.Finding
}

func (r *seededRunner) Run(_ context.Context, lens string, _ []byte, _ string) ([]report.Finding, error) {
	return r.seeded[lens], nil
}

// --- the verdict-aware floor: screen against what codegate ACTUALLY flagged ---

// TestFlaggedClassesFromVerdict — FlaggedClasses extracts codegate's actually-
// flagged classes from a verdict: every finding's class plus every failed/errored
// check's class, and NOT a passed check's (empty) class.
func TestFlaggedClassesFromVerdict(t *testing.T) {
	t.Parallel()
	v := &schema.Verdict{
		Findings: []schema.Finding{
			{CheckID: "lint", Class: "lint", Message: "x"},
			{CheckID: "forbidden-artifacts", Class: "forbidden-binary", Message: "y"},
		},
		Rungs: map[string]*schema.Rung{
			"0": {Name: "0", Checks: map[string]*schema.CheckResult{
				"format":      {CheckID: "format", Class: "format", State: "fail"},
				"secret-scan": {CheckID: "secret-scan", State: "pass"}, // pass carries no class
			}},
			"2": {Name: "2", Checks: map[string]*schema.CheckResult{
				"tests": {CheckID: "tests", Class: "timeout", State: "error"},
			}},
		},
	}
	got := floor.FlaggedClasses(v)
	want := map[string]bool{"lint": true, "forbidden-binary": true, "format": true, "timeout": true}
	if len(got) != len(want) {
		t.Fatalf("flagged classes = %v, want keys %v", got, want)
	}
	for _, c := range got {
		if !want[c] {
			t.Errorf("unexpected flagged class %q", c)
		}
	}
	// A nil verdict yields nothing.
	if fc := floor.FlaggedClasses(nil); fc != nil {
		t.Errorf("FlaggedClasses(nil) = %v, want nil", fc)
	}
}

// TestVerdictAwareFloorDropsActualDuplicate — a screener built for a specific
// verdict drops a candidate restating a class codegate ACTUALLY flagged on this
// diff (the true FR-4 duplicate), while still keeping a genuine judgment finding and
// still flooring the static catalog.
func TestVerdictAwareFloorDropsActualDuplicate(t *testing.T) {
	t.Parallel()
	v := &schema.Verdict{
		Findings: []schema.Finding{{CheckID: "lint", Class: "lint", Message: "an actual lint failure on this diff"}},
	}
	s := floor.NewScreenerForVerdict(floor.FlaggedClasses(v))

	// A candidate whose declared class is "lint" — a class codegate flagged here.
	dup := judgmentFinding("D1", report.LensDesignConformance, "lint", "CAL-5", report.SeverityBlocking)
	if s.Survives(dup.Lens, dup) {
		t.Error("verdict-aware floor kept a candidate restating a class codegate actually flagged")
	}
	// A genuine judgment finding survives.
	genuine := judgmentFinding("D2", report.LensSpecFidelity, "charter-must-unmet", "CAL-1", report.SeverityBlocking)
	if !s.Survives(genuine.Lens, genuine) {
		t.Error("verdict-aware floor dropped a genuine judgment finding")
	}
	// The static catalog is still in the set: a "build" restatement is floored even
	// though this verdict did not flag build.
	staticFloor := judgmentFinding("D3", report.LensCorrectnessCritic, "build", "CAL-11", report.SeverityBlocking)
	if s.Survives(staticFloor.Lens, staticFloor) {
		t.Error("verdict-aware floor lost the static catalog (a 'build' restatement survived)")
	}
}
