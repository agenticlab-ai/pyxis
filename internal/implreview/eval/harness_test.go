package eval_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/implreview/eval"
	"github.com/agenticlab-ai/pyxis/internal/implreview/floor"
	"github.com/agenticlab-ai/pyxis/internal/implreview/report"
	"github.com/agenticlab-ai/pyxis/internal/implreview/runtime"
)

// The hermetic eval-harness tests (T12). They drive the WHOLE protocol — the
// N-run / caught-every-run loop, the deterministic comparators, the compactness
// bound, and the versioned-budget reads — with a DETERMINISTIC FAKE LensRunner
// seeded from the fixtures, so no LLM and no network is touched. This is the CI
// floor: `go test ./internal/implreview/...` exercises every branch of the
// harness here; the real-LLM run is the gated live suite (live_test.go).

// -----------------------------------------------------------------------------
// The deterministic fake LensRunner — a fixture-driven stand-in for the real
// sibling-subagent runner. Given the harness pack (band+charter+diff), it finds
// the family leg whose diff matches and returns THAT leg's seeded findings — the
// findings a CORRECT reviewer produces. It can be told to MISS on a specific run
// index (missOnRun) to exercise the intermittent-miss defect path, and to
// FLOOD/leak to exercise the nit and floor legs' failure paths.
// -----------------------------------------------------------------------------

// fakeRunner is the deterministic fake. It is constructed from the loaded
// families so it can map a pack's diff back to that leg's seeded findings. It is
// a runtime.LensRunner.
type fakeRunner struct {
	families []*eval.Family

	// missOnRun, when >= 0, makes the runner emit NO catch on that run index
	// (parsed from the pack's reviewed-revision suffix the harness embeds) — the
	// intermittent-miss injection.
	missOnRun int
	// floodTo, when > 0, makes the runner emit this many advisory findings on
	// every nit golden — to prove the nit p50 bound reds a flood.
	floodTo int
	// obeyDirective, when true, makes the injected leg's verdict DIFFER from the
	// baseline (the runner "obeys" the planted directive by dropping the finding)
	// — to prove the injection comparator reds a verdict divergence.
	obeyDirective bool
	// dropDirective, when true, makes the runner NOT surface the planted directive
	// — to prove the injection comparator reds a non-surfaced directive.
	dropDirective bool

	// catchEmits counts, per (diff, lens), how many times the runner has emitted
	// that leg's expected catch — a proxy for the run index of that catch, since
	// the harness calls each lens's Run exactly once per leg per run. missOnRun
	// keys on this so the miss lands on a SPECIFIC run (a caught-on-most,
	// missed-on-one — the intermittent-miss DEFECT the harness must flag). Run's
	// signature stays the production one (it never sees a run counter); the counter
	// is the fake's own bookkeeping.
	catchEmits map[string]int
}

func newFakeRunner(families []*eval.Family) *fakeRunner {
	return &fakeRunner{families: families, missOnRun: -1, catchEmits: map[string]int{}}
}

// Run implements runtime.LensRunner. It reads the pack, finds the matching leg,
// and returns that leg's seeded findings for the given lens. The runtime stamps
// the lens onto each finding, so the runner returns findings whose RuleID names
// the lens the finding belongs to; the runtime calls Run once per lens, so the
// runner returns ONLY the findings whose lens matches the Run's lens argument (a
// real lens is blind to the others — the runner honors that by lens-filtering).
func (r *fakeRunner) Run(_ context.Context, lens string, pack []byte, _ string) ([]report.Finding, error) {
	env := decodePack(pack)
	leg := r.matchLeg(env)
	if leg == nil {
		return nil, nil
	}

	var out []report.Finding
	emit := func(id, ruleID, calRow, sev, msg string) {
		if lensOfRule(ruleID) != lens {
			return // finding-blind: only this lens's findings on this lens's Run.
		}
		out = append(out, report.Finding{
			ID: id, Message: msg, RuleID: ruleID, Lens: lens,
			Severity: sev, CalRow: calRow, Acceptance: "seeded eval finding",
		})
	}

	// Seeded catches (seeded-catch, band-sensitive, injection expected catches).
	// The miss injection: when missOnRun is set, the runner suppresses the catch on
	// exactly the run whose emission index equals missOnRun (keyed per diff+lens,
	// since each lens's Run fires once per leg per run) — the intermittent miss the
	// harness must flag as a DEFECT, never retry.
	if leg.expectedCatch != nil && lensOfRule(leg.expectedCatch.RuleID) == lens {
		key := env.Diff + "\x00" + lens
		idx := r.catchEmits[key]
		r.catchEmits[key] = idx + 1
		missThisRun := r.missOnRun >= 0 && idx == r.missOnRun
		// "Obey" the injected directive: drop the real finding on the injected diff
		// so the injected verdict DIVERGES from the directive-free baseline — the
		// verdict-equality comparator must red that (the injection FAILURE EVAL-5
		// guards against). Only the injected leg is affected; the baseline keeps it.
		obeyedHere := r.obeyDirective && leg.isInjectionInjected
		if !missThisRun && !obeyedHere {
			ec := leg.expectedCatch
			emit("FND-1", ec.RuleID, ec.CalRow, ec.Severity, ec.Message)
		}
	}
	// The injection leg's injected-only finding: the planted directive surfaces as a
	// candidate (UB-3) on the injected diff, unless dropDirective simulates a
	// reviewer that silently ignored it.
	if leg.isInjectionInjected && leg.directive != nil && !r.dropDirective {
		d := leg.directive
		emit("FND-2", d.RuleID, d.CalRow, d.Severity, d.Message)
	}
	// Floor family: emit the allowed (genuine) findings; the floor_candidates are
	// emitted too so the floor Screener (composed by the harness) drops them.
	for _, af := range leg.allowed {
		emit("FND-A", af.RuleID, af.CalRow, af.Severity, af.Message)
	}
	for _, fc := range leg.floorCandidates {
		emit("FND-F", fc.RuleID, fc.CalRow, fc.Severity, fc.Message)
	}
	// Nit goldens: emit the golden's seeded findings; flood if asked.
	for i, gf := range leg.goldenFindings {
		emit("FND-G"+itoa(i), gf.RuleID, gf.CalRow, gf.Severity, gf.Message)
	}
	if r.floodTo > 0 && leg.isGolden {
		for i := 0; i < r.floodTo; i++ {
			emit("FND-FLOOD"+itoa(i), lens+"/nit", "CAL-6", report.SeverityAdvisory, "a nit")
		}
	}
	return out, nil
}

// -----------------------------------------------------------------------------
// legView — a flattened, matchable view of a family leg the fake runner uses.
// -----------------------------------------------------------------------------

type legView struct {
	diff string

	expectedCatch       *eval.ExpectedCatch
	directive           *eval.DirectiveSurfaced
	allowed             []eval.AllowedFinding
	floorCandidates     []eval.FloorCandidate
	goldenFindings      []eval.Finding
	isInjectionInjected bool
	isGolden            bool
}

// matchLeg finds the leg (across every family) whose diff equals the pack's diff.
// The diffs are unique across the fixtures, so a diff identifies its leg. It
// flattens the matched leg into a legView.
func (r *fakeRunner) matchLeg(env packView) *legView {
	for _, f := range r.families {
		for i := range f.Legs {
			lg := &f.Legs[i]
			switch f.Kind {
			case eval.KindInjectionInvariance:
				if lg.BaselineDiff == env.Diff {
					return &legView{diff: env.Diff, expectedCatch: lg.ExpectedCatch}
				}
				if lg.InjectedDiff == env.Diff {
					return &legView{diff: env.Diff, expectedCatch: lg.ExpectedCatch, directive: lg.DirectiveSurfaced, isInjectionInjected: true}
				}
			case eval.KindFloorExclusion:
				if lg.Diff == env.Diff {
					return &legView{diff: env.Diff, allowed: lg.AllowedFindings, floorCandidates: lg.FloorCandidates}
				}
			default:
				if lg.Diff == env.Diff {
					return &legView{diff: env.Diff, expectedCatch: lg.ExpectedCatch}
				}
			}
		}
		for i := range f.Goldens {
			g := &f.Goldens[i]
			if g.Diff == env.Diff {
				return &legView{diff: env.Diff, goldenFindings: g.Findings, isGolden: true}
			}
		}
	}
	return nil
}

// -----------------------------------------------------------------------------
// pack decode mirror (the harness's pack is package-private; the test re-decodes
// the same JSON shape it knows the harness writes — band/charter/diff).
// -----------------------------------------------------------------------------

type packView struct {
	Band    string `json:"band"`
	Charter string `json:"charter"`
	Diff    string `json:"diff"`
}

func decodePack(pack []byte) packView {
	var p packView
	_ = json.Unmarshal(pack, &p)
	return p
}

// -----------------------------------------------------------------------------
// tiny helpers
// -----------------------------------------------------------------------------

func lensOfRule(ruleID string) string {
	lens, _, _ := strings.Cut(ruleID, "/")
	return lens
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

// repoRoot walks up from the test's cwd to the module root (the dir holding
// go.mod). It is shared by the hermetic and the gated live test (this file has no
// build tag, so it is always compiled). Fixtures live under the module root.
func repoRoot(t testingTB) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not find go.mod above %s", dir)
		}
		dir = parent
	}
}

// testingTB is the tiny subset of testing.TB repoRoot needs (so the live test,
// compiled separately, can call it with its own *testing.T).
type testingTB interface {
	Helper()
	Fatalf(format string, args ...any)
}

// fixturesDir returns the canonical fixtures dir under the module root.
func fixturesDir(t *testing.T) string {
	return filepath.Join(repoRoot(t), ".pyxis", "skills", "impl-review", "fixtures")
}

// loadFamilies loads the eight shipped families (the real fixtures) for a test.
func loadFamilies(t *testing.T) []*eval.Family {
	t.Helper()
	fs, err := eval.LoadFamilies(filepath.Join(fixturesDir(t), "families"))
	if err != nil {
		t.Fatalf("load families: %v", err)
	}
	return fs
}

// baseConfig builds a RunConfig with the fake runner, synthetic instructions, and
// the loaded provenance/enforced set — the common harness config for the tests.
func baseConfig(t *testing.T, runner runtime.LensRunner) eval.RunConfig {
	t.Helper()
	prov, err := eval.LoadProvenance(filepath.Join(fixturesDir(t), "provenance.json"))
	if err != nil {
		t.Fatalf("load provenance: %v", err)
	}
	instr := map[string]string{}
	for _, l := range runtime.Lenses {
		instr[l] = "hermetic synthetic instruction for " + l
	}
	return eval.RunConfig{
		Provenance:   prov,
		Runner:       runner,
		Instructions: instr,
		RoundProvenance: report.Provenance{
			Model: "fake", PromptsVersion: "impl-review-lenses/v1", CalVersion: "impl-review-calibration/v1",
		},
		ReviewerRoot:    "session:reviewer-root",
		ImplementerRoot: "session:implementer-root",
		Enforced:        floor.EnforcedClasses(),
	}
}
