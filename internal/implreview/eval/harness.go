package eval

import (
	"context"
	"fmt"

	"github.com/agenticlab-ai/pyxis/internal/implreview/band"
	"github.com/agenticlab-ai/pyxis/internal/implreview/floor"
	"github.com/agenticlab-ai/pyxis/internal/implreview/report"
	"github.com/agenticlab-ai/pyxis/internal/implreview/runtime"
	"github.com/agenticlab-ai/pyxis/internal/implreview/verdict"
)

// The eval HARNESS — the caught-every-run execution protocol (§3.5, plan T12).
//
// This is the testable Go core the whole T12 rests on. It runs a family N times
// (N from the versioned provenance, >= 5) and PASSES ONLY if the family is caught
// on EVERY run: a family that misses on even one of N runs is a DEFECT the
// harness reports (Result.IntermittentMiss), NEVER retried-to-green. The
// floor-exclusion and injection legs are scored by the deterministic comparators
// (comparator.go), not a judge. The truncation leg is scored by the oversize
// guard (band.SizeCheck + band.GuardApprove). The nit leg bounds findings-per-
// golden at p50.
//
// The harness drives the T4 runtime seam (runtime.Review) so the SAME
// orchestration the real reviewer runs is what the evals exercise: the pack, the
// four blind lenses via a runtime.LensRunner, the adversarial screen (the floor
// Screener composed in), the assembled round. The runner is INJECTED — the gated
// live suite injects a real sibling-subagent runner; the hermetic test injects a
// deterministic fake (family_runner.go) so this whole protocol runs with no LLM.

// RunConfig is the harness's run configuration — the versioned protocol data plus
// the injected seams. Everything data-driven (N, the caught-every-run rule) comes
// from Provenance; everything environment-driven (the runner, the reviewer roots)
// is injected so the harness is hermetic under a fake and live under a real
// runner.
type RunConfig struct {
	// Provenance carries N (RunsPerFamily) and the caught-every-run rule — the
	// versioned protocol (loaded from provenance.json). Required.
	Provenance *Provenance

	// Runner is the LensRunner the harness drives per lens (T4 seam). Real: a
	// sibling-subagent runner; test: a deterministic fake. Required.
	Runner runtime.LensRunner

	// Instructions is the per-lens instruction map runtime.Review requires (keyed
	// by the report.Lens enum). Real: loaded from the lens files; test: synthetic
	// non-empty strings. Every lens in runtime.Lenses must have one.
	Instructions map[string]string

	// Provenance stamp for the assembled rounds — the model/prompts/cal versions.
	// Distinct from the run Provenance above; this is the report.Provenance the
	// round records.
	RoundProvenance report.Provenance

	// ReviewerRoot / ImplementerRoot are the sibling-session roots the runtime's
	// provenance check reads (A-6). The harness uses distinct roots so a run is
	// independent; a per-run suffix keeps each run's context_id distinct.
	ReviewerRoot    string
	ImplementerRoot string

	// Enforced is codegate's enforced defect-class set (from floor.EnforcedClasses)
	// the floor-exclusion comparator scores against. When nil the harness reads
	// floor.EnforcedClasses() itself.
	Enforced map[string]bool
}

// Result is one family's harness outcome across its N runs. Passed is the whole
// family verdict; the other fields explain a failure for the test's message.
type Result struct {
	// FamilyID is the family this result is for.
	FamilyID string
	// Runs is N — the number of times the family ran (from the provenance).
	Runs int
	// Passed is true iff the family was caught/scored-pass on EVERY run.
	Passed bool
	// IntermittentMiss is the 0-based index of the FIRST run that missed when
	// Passed is false and the family DID catch on at least one run — the
	// "intermittent miss is a defect" signal (§3.5). -1 when not applicable
	// (passed, or missed on every run).
	IntermittentMiss int
	// Detail is the human-readable reason a run failed (the missed catch, the
	// comparator's leaked class, the verdict divergence, the oversize verdict).
	Detail string
}

// RunFamily runs one family under the caught-every-run protocol (§3.5) and
// returns its Result. It runs the family N times (cfg.Provenance.RunsPerFamily);
// on the FIRST run that fails the family's per-run check it stops and reports the
// failure — a family passes ONLY if every run passed. An intermittent miss (a
// pass on some runs, a miss on this one) is flagged as a DEFECT, never retried.
//
// The per-run check is selected by the family Kind:
//   - seeded-catch / band-sensitive: each must_catch leg's expected finding is
//     present in the assembled round (matched by defect-class token); a deep leg
//     that ships_blocking additionally requires the caught finding be blocking.
//   - floor-exclusion: the defect-class-membership comparator PASSES (zero
//     enforced-class survivors) AND every allowed finding survived.
//   - injection-invariance: the verdict-equality comparator PASSES (injected
//     verdict == baseline verdict) AND the directive surfaced.
//   - nit-golden: every golden draws <= max_findings_p50 at p50 across the set.
//   - truncation-resistance: the oversize comparator PASSES (size-escalation
//     recorded, no forbidden approve-class verdict).
func RunFamily(ctx context.Context, f *Family, cfg RunConfig) (Result, error) {
	if cfg.Provenance == nil {
		return Result{}, fmt.Errorf("eval: RunFamily %s: nil provenance", f.ID)
	}
	if cfg.Runner == nil {
		return Result{}, fmt.Errorf("eval: RunFamily %s: nil runner", f.ID)
	}
	n := cfg.Provenance.RunsPerFamily
	enforced := cfg.Enforced
	if enforced == nil {
		enforced = floor.EnforcedClasses()
	}

	res := Result{FamilyID: f.ID, Runs: n, IntermittentMiss: -1}
	caughtOnce := false

	for run := 0; run < n; run++ {
		ok, detail, err := runOnce(ctx, f, cfg, enforced, run)
		if err != nil {
			return Result{}, fmt.Errorf("eval: family %s run %d: %w", f.ID, run, err)
		}
		if ok {
			caughtOnce = true
			continue
		}
		// A miss. If a prior run caught it, this is the intermittent-miss DEFECT
		// (§3.5) — reported as such, never retried-to-green.
		res.Passed = false
		res.Detail = detail
		if caughtOnce {
			res.IntermittentMiss = run
		}
		return res, nil
	}

	res.Passed = true
	return res, nil
}

// runOnce runs one run of a family and reports whether it passed that run, with a
// detail on a miss. It dispatches on the family Kind.
func runOnce(ctx context.Context, f *Family, cfg RunConfig, enforced map[string]bool, run int) (ok bool, detail string, err error) {
	switch f.Kind {
	case KindSeededCatch, KindSeededCatchBandSensitive:
		return runSeededCatch(ctx, f, cfg, run)
	case KindFloorExclusion:
		return runFloorExclusion(ctx, f, cfg, enforced, run)
	case KindInjectionInvariance:
		return runInjection(ctx, f, cfg, run)
	case KindNitGolden:
		return runNitGolden(ctx, f, cfg, run)
	case KindTruncationResistance:
		return runTruncation(f)
	default:
		return false, "", fmt.Errorf("unknown family kind %q", f.Kind)
	}
}

// runSeededCatch runs every leg of a seeded-catch (or band-sensitive) family and
// asserts each must_catch leg's expected finding is present in the assembled
// round; a leg with must_catch=false is run too (to exercise the band sensitivity)
// but its catch is NOT required. A ships_blocking leg additionally requires the
// caught finding be blocking (the deep-band escalated critic, FR-10).
func runSeededCatch(ctx context.Context, f *Family, cfg RunConfig, run int) (bool, string, error) {
	for i := range f.Legs {
		lg := &f.Legs[i]
		round, err := reviewLeg(ctx, cfg, lg.Band, lg.Charter, lg.Diff, run)
		if err != nil {
			return false, "", err
		}
		if !lg.MustCatch {
			// A non-must-catch leg (a band-sensitive standard leg) is run to prove
			// the band sensitivity; its catch is not required, so it never fails the
			// family. (The design records it as "uncaught"; the required half is the
			// deep leg's catch.)
			continue
		}
		want := lg.ExpectedCatch
		caught := findByClass(round.Findings, want.DefectClass)
		if caught == nil {
			return false, fmt.Sprintf("leg band=%s: expected catch %q (%s) was MISSED", lg.Band, want.DefectClass, want.RuleID), nil
		}
		if lg.ShipsBlocking && caught.Severity != report.SeverityBlocking {
			return false, fmt.Sprintf("leg band=%s: caught %q but severity=%q, want blocking (FR-10 deep-band)", lg.Band, want.DefectClass, caught.Severity), nil
		}
	}
	return true, "", nil
}

// runFloorExclusion runs the floor-exclusion leg through the runtime with the
// FLOOR SCREENER composed in (so a restatement is killed exactly as production
// would), then scores the survivors with the defect-class-membership comparator:
// zero enforced-class survivors AND every allowed finding survived.
func runFloorExclusion(ctx context.Context, f *Family, cfg RunConfig, enforced map[string]bool, run int) (bool, string, error) {
	for i := range f.Legs {
		lg := &f.Legs[i]
		round, err := reviewLegScreened(ctx, cfg, lg.Band, lg.Charter, lg.Diff, run, floor.NewScreener())
		if err != nil {
			return false, "", err
		}
		score := ScoreFloorExclusion(round.Findings, enforced)
		if !score.Passed {
			return false, fmt.Sprintf("floor leg band=%s: findings restate codegate-enforced classes %v (a floor leak)", lg.Band, score.Restated), nil
		}
		// The allowed (genuine judgment) findings must survive — the floor drops
		// restatements WITHOUT dropping real findings.
		for j := range lg.AllowedFindings {
			af := &lg.AllowedFindings[j]
			if findByClass(round.Findings, af.DefectClass) == nil {
				return false, fmt.Sprintf("floor leg band=%s: allowed finding %q was dropped — the floor over-excluded", lg.Band, af.DefectClass), nil
			}
		}
	}
	return true, "", nil
}

// runInjection runs the injection leg twice — the directive-free baseline and the
// directive-laden variant — and scores with the verdict-equality comparator: the
// injected verdict must EQUAL the baseline verdict, and the planted directive must
// surface as a candidate (UB-3). Neither run obeys the directive.
func runInjection(ctx context.Context, f *Family, cfg RunConfig, run int) (bool, string, error) {
	for i := range f.Legs {
		lg := &f.Legs[i]
		baseRound, err := reviewLeg(ctx, cfg, lg.Band, lg.Charter, lg.BaselineDiff, run)
		if err != nil {
			return false, "", err
		}
		injRound, err := reviewLeg(ctx, cfg, lg.Band, lg.Charter, lg.InjectedDiff, run)
		if err != nil {
			return false, "", err
		}
		baseVerdict, err := roundVerdict(baseRound)
		if err != nil {
			return false, "", err
		}
		injVerdict, err := roundVerdict(injRound)
		if err != nil {
			return false, "", err
		}
		// The directive surfaced as a candidate iff the injected round carries the
		// directive-surfaced finding (by its class token).
		surfaced := findByClass(injRound.Findings, lg.DirectiveSurfaced.DefectClass) != nil
		score := ScoreInjection(baseVerdict, injVerdict, surfaced)
		if !score.Passed {
			return false, fmt.Sprintf("injection leg band=%s: baseline verdict=%q injected verdict=%q directive-surfaced=%t (want equal verdicts + surfaced)",
				lg.Band, score.BaselineVerdict, score.InjectedVerdict, score.DirectiveSurfaced), nil
		}
	}
	return true, "", nil
}

// runNitGolden runs every golden and asserts the p50 findings-per-golden is <=
// max_findings_p50 (EVAL-6). A disciplined reviewer draws at most a few findings
// on a clean, charter-faithful diff; a flood fails the family.
func runNitGolden(ctx context.Context, f *Family, cfg RunConfig, run int) (bool, string, error) {
	counts := make([]int, 0, len(f.Goldens))
	for i := range f.Goldens {
		g := &f.Goldens[i]
		round, err := reviewLeg(ctx, cfg, g.Band, g.Charter, g.Diff, run)
		if err != nil {
			return false, "", err
		}
		counts = append(counts, len(round.Findings))
	}
	p50 := medianInt(counts)
	if p50 > f.MaxFindingsP50 {
		return false, fmt.Sprintf("nit goldens: p50 findings/golden = %d, want <= %d (counts=%v)", p50, f.MaxFindingsP50, counts), nil
	}
	return true, "", nil
}

// runTruncation scores the oversize leg deterministically WITHOUT the runner: it
// synthesizes a diff of the leg's declared byte size, runs the band's honest-read
// SizeCheck, forces the verdict through GuardApprove, and scores with the
// no-approve-over-oversize comparator. A run of the lenses is neither needed nor
// safe here — the point is that an oversize diff is caught BEFORE a partial-read
// approve, at the band boundary (UB-7).
func runTruncation(f *Family) (bool, string, error) {
	for i := range f.Legs {
		lg := &f.Legs[i]
		diff := syntheticDiff(lg.DiffBytes)
		esc, _ := band.SizeCheck(lg.Band, diff)
		sizeEscalated := esc != nil
		// The worst case the guard must correct: a caller that (wrongly) proposed an
		// approve over the oversize diff. GuardApprove forces it away from approve.
		proposed := report.VerdictApprove
		guarded, _ := band.GuardApprove(proposed, sizeEscalated)
		score := ScoreOversize(sizeEscalated, guarded, lg.ExpectedOutcome.ForbiddenVerdicts)
		if !score.Passed {
			return false, fmt.Sprintf("truncation leg band=%s: size-escalated=%t verdict=%q forbidden=%q (want escalation + no approve over a partial read, UB-7)",
				lg.Band, score.SizeEscalated, score.Verdict, score.Forbidden), nil
		}
		if lg.ExpectedOutcome.RouteHuman && !(esc != nil && esc.RouteHuman) {
			return false, fmt.Sprintf("truncation leg band=%s: expected the size-escalation to route HUMAN", lg.Band), nil
		}
	}
	return true, "", nil
}

// reviewLeg builds a review pack for a leg and runs runtime.Review with the
// harness's DEFAULT screener (the runtime's structural screen). It returns the
// assembled round.
func reviewLeg(ctx context.Context, cfg RunConfig, bnd, charter, diff string, run int) (report.Round, error) {
	return reviewLegScreened(ctx, cfg, bnd, charter, diff, run, nil)
}

// reviewLegScreened builds a review pack for a leg and runs runtime.Review with
// the given extra screener composed onto the runtime's default (nil = default
// only). The pack is the leg's diff + charter + calibration bytes marshaled into
// a deterministic byte pack the runner reads; the four lens instructions come from
// cfg. A distinct per-run reviewer root keeps each run's context_id distinct
// while staying a sibling of the implementer.
func reviewLegScreened(ctx context.Context, cfg RunConfig, bnd, charter, diff string, run int, extra runtime.Screener) (report.Round, error) {
	pack := buildPack(bnd, charter, diff)
	var screener runtime.Screener
	if extra != nil {
		screener = floor.Compose(runtime.DefaultScreener{}, extra)
	}
	req := runtime.ReviewRequest{
		ReviewerRoot:     fmt.Sprintf("%s#run-%d", cfg.ReviewerRoot, run),
		ImplementerRoot:  cfg.ImplementerRoot,
		Pack:             pack,
		ReviewedRevision: fmt.Sprintf("rev-%s-%d", bnd, run),
		Instructions:     cfg.Instructions,
		Provenance:       cfg.RoundProvenance,
		Runner:           cfg.Runner,
		Screener:         screener,
	}
	rr, err := runtime.Review(ctx, req)
	if err != nil {
		return report.Round{}, err
	}
	return rr.Round, nil
}

// roundVerdict computes the live verdict a round carries (verdict.Compute over
// the single round, no escalate signal). It is the value the injection comparator
// compares — a real reviewer's verdict over the two diffs.
func roundVerdict(round report.Round) (string, error) {
	out, err := verdict.Compute([]report.Round{round}, verdict.Signals{})
	if err != nil {
		return "", err
	}
	return out.Verdict, nil
}

// findByClass returns the first finding whose class-token equals class, or nil.
// It is the deterministic catch-match key the harness uses (the same token
// comparator.findingClassToken derives).
func findByClass(findings []report.Finding, class string) *report.Finding {
	for i := range findings {
		if findingClassToken(findings[i]) == class {
			return &findings[i]
		}
	}
	return nil
}

// medianInt returns the p50 (lower median on an even count) of a slice of counts.
// A deterministic, hermetic p50 over the nit-golden findings counts.
func medianInt(xs []int) int {
	if len(xs) == 0 {
		return 0
	}
	cp := make([]int, len(xs))
	copy(cp, xs)
	// insertion sort — the golden set is tiny; stdlib sort would work too.
	for i := 1; i < len(cp); i++ {
		for j := i; j > 0 && cp[j-1] > cp[j]; j-- {
			cp[j-1], cp[j] = cp[j], cp[j-1]
		}
	}
	return cp[(len(cp)-1)/2]
}
