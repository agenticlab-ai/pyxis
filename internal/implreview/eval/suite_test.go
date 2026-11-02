package eval_test

import (
	"context"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/implreview/eval"
	"github.com/agenticlab-ai/pyxis/internal/implreview/floor"
	"github.com/agenticlab-ai/pyxis/internal/implreview/report"
)

// TestAllFamiliesCaughtOnEveryRun is the CORE T12 assertion (§3.5, DOD): every
// shipped family, driven by a CORRECT reviewer (the fixture-seeded fake), is
// caught/scored-pass on EVERY one of its N runs. This exercises the whole
// caught-every-run protocol over the real eight fixtures, hermetically.
func TestAllFamiliesCaughtOnEveryRun(t *testing.T) {
	t.Parallel()
	families := loadFamilies(t)
	if len(families) != 8 {
		t.Fatalf("expected the eight EVAL-1..8 families, got %d", len(families))
	}
	cfg := baseConfig(t, newFakeRunner(families))

	for _, f := range families {
		f := f
		t.Run(f.ID, func(t *testing.T) {
			res, err := eval.RunFamily(context.Background(), f, cfg)
			if err != nil {
				t.Fatalf("run family %s: %v", f.ID, err)
			}
			if res.Runs < eval.MinRunsPerFamily {
				t.Fatalf("family %s ran %d times, want >= %d (N >= 5, §3.5)", f.ID, res.Runs, eval.MinRunsPerFamily)
			}
			if !res.Passed {
				t.Fatalf("family %s NOT caught on every run (%d runs): %s", f.ID, res.Runs, res.Detail)
			}
		})
	}
}

// TestIntermittentMissIsADefect is the §3.5 non-negotiable: a family caught on
// SOME runs but missed on ONE is a DEFECT the harness reports (with the miss run
// index), NOT retried-to-green. The fake is told to miss on run 3 of N; the
// harness must return Passed=false with IntermittentMiss==3.
func TestIntermittentMissIsADefect(t *testing.T) {
	t.Parallel()
	families := loadFamilies(t)
	// EVAL-1 is a single-leg seeded catch — the cleanest family to inject a miss on.
	f := familyByID(t, families, "EVAL-1")

	runner := newFakeRunner(families)
	runner.missOnRun = 3 // caught on runs 0,1,2, then MISSED on run 3.
	cfg := baseConfig(t, runner)

	res, err := eval.RunFamily(context.Background(), f, cfg)
	if err != nil {
		t.Fatalf("run family: %v", err)
	}
	if res.Passed {
		t.Fatal("a family that missed on one of N runs must FAIL — an intermittent miss is a defect, never retried-to-green (§3.5)")
	}
	if res.IntermittentMiss != 3 {
		t.Fatalf("expected the intermittent miss to be flagged on run 3, got IntermittentMiss=%d", res.IntermittentMiss)
	}
	if !strings.Contains(res.Detail, "MISSED") {
		t.Fatalf("expected the detail to name the missed catch, got %q", res.Detail)
	}
}

// TestMissOnEveryRunIsNotIntermittent — a family that is NEVER caught (misses on
// run 0, the first run) fails, but is NOT flagged as an INTERMITTENT miss (it
// never caught once). The harness distinguishes "never worked" from "flaky".
func TestMissOnEveryRunIsNotIntermittent(t *testing.T) {
	t.Parallel()
	families := loadFamilies(t)
	f := familyByID(t, families, "EVAL-1")

	runner := newFakeRunner(families)
	runner.missOnRun = 0 // missed on the very first run — never caught.
	cfg := baseConfig(t, runner)

	res, err := eval.RunFamily(context.Background(), f, cfg)
	if err != nil {
		t.Fatalf("run family: %v", err)
	}
	if res.Passed {
		t.Fatal("a family missed on run 0 must fail")
	}
	if res.IntermittentMiss != -1 {
		t.Fatalf("a never-caught family is not an intermittent miss; want IntermittentMiss=-1, got %d", res.IntermittentMiss)
	}
}

// TestFloorExclusionScoredByComparator — EVAL-4 is scored by the deterministic
// defect-class-membership comparator, NOT a judge. The floor family passes when
// the floor screener drops the seeded restatements (secret, test-deleted) and the
// comparator sees zero enforced-class survivors, while the genuine allowed finding
// survives.
func TestFloorExclusionScoredByComparator(t *testing.T) {
	t.Parallel()
	families := loadFamilies(t)
	f := familyByID(t, families, "EVAL-4")
	if f.Comparator != eval.ComparatorDefectClassMembership {
		t.Fatalf("EVAL-4 must declare the defect-class-membership comparator, has %q", f.Comparator)
	}
	cfg := baseConfig(t, newFakeRunner(families))

	res, err := eval.RunFamily(context.Background(), f, cfg)
	if err != nil {
		t.Fatalf("run EVAL-4: %v", err)
	}
	if !res.Passed {
		t.Fatalf("EVAL-4 floor exclusion should pass (restatements dropped, allowed survives): %s", res.Detail)
	}
}

// TestFloorComparatorRedsALeak — the defect-class-membership comparator itself
// reds a floor LEAK: a surviving finding whose class-token IS a codegate-enforced
// class fails the comparator (scored directly, bypassing the screener — the point
// is the comparator's own verdict, the independent backstop). This proves the
// comparator is a real check, not vacuous.
func TestFloorComparatorRedsALeak(t *testing.T) {
	t.Parallel()
	enforced := floor.EnforcedClasses()

	// A genuine judgment finding (class NOT enforced) → the comparator passes.
	clean := []report.Finding{{RuleID: "spec-fidelity/charter-ac-contradicted"}}
	if got := eval.ScoreFloorExclusion(clean, enforced); !got.Passed {
		t.Fatalf("a non-enforced-class finding must pass the floor comparator, got %+v", got)
	}

	// A finding restating an enforced class ("secret") → the comparator reds it.
	leak := []report.Finding{{RuleID: "security-abuse/secret"}}
	got := eval.ScoreFloorExclusion(leak, enforced)
	if got.Passed {
		t.Fatal("a survivor restating the enforced class 'secret' must FAIL the floor comparator")
	}
	if len(got.Restated) != 1 || got.Restated[0] != "secret" {
		t.Fatalf("expected the leaked class 'secret' named, got %v", got.Restated)
	}
}

// TestInjectionInvarianceScoredByComparator — EVAL-5 is scored by the
// deterministic verdict-equality comparator, NOT a judge: the injected diff's
// verdict must equal the directive-free baseline's, and the directive must surface.
// With a correct reviewer (the fake ignores the directive and surfaces it), the
// family passes.
func TestInjectionInvarianceScoredByComparator(t *testing.T) {
	t.Parallel()
	families := loadFamilies(t)
	f := familyByID(t, families, "EVAL-5")
	if f.Comparator != eval.ComparatorVerdictEquality {
		t.Fatalf("EVAL-5 must declare the verdict-equality comparator, has %q", f.Comparator)
	}
	cfg := baseConfig(t, newFakeRunner(families))

	res, err := eval.RunFamily(context.Background(), f, cfg)
	if err != nil {
		t.Fatalf("run EVAL-5: %v", err)
	}
	if !res.Passed {
		t.Fatalf("EVAL-5 injection invariance should pass (equal verdicts + directive surfaced): %s", res.Detail)
	}
}

// TestInjectionComparatorRedsADivergedVerdict — if the reviewer OBEYS the planted
// directive (dropping the real finding on the injected diff so its verdict
// diverges from the baseline), the verdict-equality comparator reds it. This is
// the injection FAILURE the family guards against.
func TestInjectionComparatorRedsADivergedVerdict(t *testing.T) {
	t.Parallel()
	families := loadFamilies(t)
	f := familyByID(t, families, "EVAL-5")

	runner := newFakeRunner(families)
	runner.obeyDirective = true // the reviewer "obeys" the injected directive.
	cfg := baseConfig(t, runner)

	res, err := eval.RunFamily(context.Background(), f, cfg)
	if err != nil {
		t.Fatalf("run EVAL-5: %v", err)
	}
	if res.Passed {
		t.Fatal("an injected directive that ALTERED the verdict must FAIL the verdict-equality comparator (EVAL-5)")
	}
}

// TestInjectionComparatorRedsANonSurfacedDirective — even if the verdict is
// unchanged, a directive the reviewer did NOT surface as a candidate fails the
// comparator (UB-3 requires the directive be surfaced, not silently ignored).
func TestInjectionComparatorRedsANonSurfacedDirective(t *testing.T) {
	t.Parallel()
	families := loadFamilies(t)
	f := familyByID(t, families, "EVAL-5")

	runner := newFakeRunner(families)
	runner.dropDirective = true // verdict unchanged, but the directive is not surfaced.
	cfg := baseConfig(t, runner)

	res, err := eval.RunFamily(context.Background(), f, cfg)
	if err != nil {
		t.Fatalf("run EVAL-5: %v", err)
	}
	if res.Passed {
		t.Fatal("a directive that was not surfaced as a candidate must FAIL the comparator (UB-3)")
	}
}

// TestBandSensitiveFamilyDeepLegBlocks — EVAL-3 and EVAL-7's deep legs must catch
// the defect and (EVAL-7) ship it BLOCKING (FR-10). The standard leg of EVAL-3
// records the same defect as uncaught (not required) — both legs run, only the
// deep leg's catch is required.
func TestBandSensitiveFamilyDeepLegBlocks(t *testing.T) {
	t.Parallel()
	families := loadFamilies(t)
	cfg := baseConfig(t, newFakeRunner(families))

	for _, id := range []string{"EVAL-3", "EVAL-7"} {
		f := familyByID(t, families, id)
		res, err := eval.RunFamily(context.Background(), f, cfg)
		if err != nil {
			t.Fatalf("run %s: %v", id, err)
		}
		if !res.Passed {
			t.Fatalf("%s band-sensitive family should pass (deep leg catches, ships blocking): %s", id, res.Detail)
		}
	}
}

// TestTruncationResistance — EVAL-8: an oversize diff (exceeding the standard
// band's read budget) records a size-escalation and draws NO approve verdict
// (UB-7), scored by the no-approve-over-oversize comparator. The runner is not
// consulted (the guard is at the band boundary).
func TestTruncationResistance(t *testing.T) {
	t.Parallel()
	families := loadFamilies(t)
	f := familyByID(t, families, "EVAL-8")
	if f.Comparator != eval.ComparatorNoApproveOverOversize {
		t.Fatalf("EVAL-8 must declare the no-approve-over-oversize comparator, has %q", f.Comparator)
	}
	cfg := baseConfig(t, newFakeRunner(families))

	res, err := eval.RunFamily(context.Background(), f, cfg)
	if err != nil {
		t.Fatalf("run EVAL-8: %v", err)
	}
	if !res.Passed {
		t.Fatalf("EVAL-8 truncation resistance should pass (escalation + no approve over partial read): %s", res.Detail)
	}
}

// TestOversizeComparatorRedsAnApprove — the no-approve-over-oversize comparator
// itself reds an approve over an oversize diff: a recorded escalation but an
// approve verdict is the forbidden case (UB-7).
func TestOversizeComparatorRedsAnApprove(t *testing.T) {
	t.Parallel()
	forbidden := []string{report.VerdictApprove, report.VerdictApproveWithAdvisories}

	// Escalation recorded + a non-approve verdict → pass.
	if got := eval.ScoreOversize(true, report.VerdictRequestChanges, forbidden); !got.Passed {
		t.Fatalf("escalation + request-changes must pass, got %+v", got)
	}
	// Escalation recorded + an approve verdict → the forbidden case, fail.
	got := eval.ScoreOversize(true, report.VerdictApprove, forbidden)
	if got.Passed {
		t.Fatal("an approve over an oversize (escalated) diff must FAIL (UB-7)")
	}
	if got.Forbidden != report.VerdictApprove {
		t.Fatalf("expected the forbidden approve named, got %q", got.Forbidden)
	}
	// No escalation recorded at all → an unread diff that was not caught, fail.
	if eval.ScoreOversize(false, report.VerdictRequestChanges, forbidden).Passed {
		t.Fatal("no size-escalation recorded must FAIL (the oversize was not caught)")
	}
}

// TestNitDisciplineFlood — EVAL-6: a disciplined reviewer draws <= max_findings_p50
// per golden at p50. When the reviewer FLOODS (emits many findings per golden), the
// nit p50 bound reds it. The clean run (no flood) passes.
func TestNitDisciplineFlood(t *testing.T) {
	t.Parallel()
	families := loadFamilies(t)
	f := familyByID(t, families, "EVAL-6")

	// Clean: passes.
	cleanRes, err := eval.RunFamily(context.Background(), f, baseConfig(t, newFakeRunner(families)))
	if err != nil {
		t.Fatalf("run EVAL-6 clean: %v", err)
	}
	if !cleanRes.Passed {
		t.Fatalf("EVAL-6 clean goldens should pass: %s", cleanRes.Detail)
	}

	// Flood: the runner emits 5 nits per golden — p50 = 5 > max_findings_p50 (3).
	flood := newFakeRunner(families)
	flood.floodTo = 5
	floodRes, err := eval.RunFamily(context.Background(), f, baseConfig(t, flood))
	if err != nil {
		t.Fatalf("run EVAL-6 flood: %v", err)
	}
	if floodRes.Passed {
		t.Fatal("a nit flood (p50 findings/golden over the bound) must FAIL EVAL-6")
	}
}

// TestCompactnessBoundRedsOverLargePayload — the compactness leg bounds the
// composed artifact size over the goldens (§3.5). A compact golden report passes;
// a report with a giant finding message (a verbose lens) reds the bound. The bound
// is on the REAL persisted artifact (persist.ComposeArtifact), not an estimate.
func TestCompactnessBoundRedsOverLargePayload(t *testing.T) {
	t.Parallel()
	families := loadFamilies(t)
	f := familyByID(t, families, "EVAL-6")

	// Every shipped golden's artifact is within the bound.
	for i := range f.Goldens {
		g := &f.Goldens[i]
		r := eval.GoldenReport(f.ID, i, g)
		res, err := eval.ScoreCompactness(r)
		if err != nil {
			t.Fatalf("compactness golden %d: %v", i, err)
		}
		if !res.Passed {
			t.Fatalf("shipped golden %d artifact is %d bytes, over the compactness bound %d — verbosity defect", i, res.Bytes, res.Bound)
		}
	}

	// A verbose payload (a giant finding message) reds the bound.
	big := &eval.Golden{
		Band:    report.BandStandard,
		Charter: "x",
		Diff:    "x",
		Findings: []eval.Finding{{
			RuleID:      "spec-fidelity/verbose",
			CalRow:      "CAL-2",
			Severity:    report.SeverityAdvisory,
			DefectClass: "verbose",
			Message:     strings.Repeat("an essay, not a claim + evidence anchor. ", 1000),
		}},
	}
	res, err := eval.ScoreCompactness(eval.GoldenReport("EVAL-6", 99, big))
	if err != nil {
		t.Fatalf("compactness big: %v", err)
	}
	if res.Passed {
		t.Fatalf("a %d-byte artifact must FAIL the compactness bound %d (a verbosity defect fixed at the prompt, never by splitting — §3.5)", res.Bytes, res.Bound)
	}
}

// familyByID finds a family by id or fails the test.
func familyByID(t *testing.T, families []*eval.Family, id string) *eval.Family {
	t.Helper()
	for _, f := range families {
		if f.ID == id {
			return f
		}
	}
	t.Fatalf("family %q not found among the loaded fixtures", id)
	return nil
}
