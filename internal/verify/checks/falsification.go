// Package checks: falsification.go implements rung 3's anti-tautology floor
// (design codegate-v1 §3.9, basis FR-11/FR-18): a proposal diff's new or
// changed tests must be demonstrated to fail once the non-test half that
// makes them pass is reverted — never merely asserted. Applicable only when
// the diff's non-test half is non-empty AND it adds or modifies test files;
// otherwise the check records not-applicable and emits nothing (FR-18). When
// applicable, codegate creates an ephemeral worktree at head, provisions it,
// and runs a two-leg protocol under DINV-5's runner: a sanity leg (unreverted;
// the targeted tests must cleanly pass, else rung error — never a
// falsification verdict) and a revert leg (the non-test half's reverse
// applied; a targeted test that still passes is tautological). A revert that
// does not apply cleanly is a rung error (UB-4). The worktree is removed at
// run end; the workspace under verification is never touched (DINV-7).
package checks

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/verify/diffset"
	"github.com/agenticlab-ai/pyxis/internal/verify/ecosystem"
	"github.com/agenticlab-ai/pyxis/internal/verify/runner"
	"github.com/agenticlab-ai/pyxis/internal/verify/schema"
	"github.com/agenticlab-ai/pyxis/internal/verify/worktree"
)

// FalsificationCheckID is this check's registry and finding CheckID.
const FalsificationCheckID = "falsification"

// falsificationTestVerbID is the "test" verb rung 3 shares with rung 2.
// Declaring it as a VerbDep means an unbound test verb (an unsupported or
// misconfigured stack) skips this check the same generic way every verb check
// skips (FR-6) — the ladder's own concern, never this check's.
const falsificationTestVerbID = "test"

func init() {
	register(schema.Check{
		ID:                FalsificationCheckID,
		Rung:              "3",
		VerbDeps:          []string{falsificationTestVerbID},
		AuthorIndependent: true,
		InputSet:          []string{"diff"},
	}, falsificationCheck)
}

// falsificationCheck is the ladder-dispatched executor. It resolves
// applicability (FR-18) cheaply from the shared diff set alone; only once
// applicable does it touch git/the filesystem further (target extraction, the
// revert patch, the ephemeral worktree).
func falsificationCheck(ctx context.Context, rc RunContext) Outcome {
	if rc.Diff == nil {
		return falsificationErrorf("no diff set provided")
	}
	if rc.Manifest == nil {
		return falsificationErrorf("no manifest available")
	}

	// TablesForStack's stack-support gate is shared with test-delta (T7):
	// falsification reads only its resolved TestGlobs, but a stack absent from
	// the ecosystem's skip/assertion tables fails closed here exactly as it
	// does there — growing stack support (T10) is a data change, not a rewrite
	// of this check.
	tables, err := TablesForStack(rc.Ecosystem, rc.Manifest.Stack, rc.Manifest.TestGlobs)
	if err != nil {
		return falsificationErrorf("%v", err)
	}
	// A stack with a registered focused-test strategy (go: `go test -json`,
	// which yields machine-parseable per-test outcomes) runs the two-leg at
	// function-level granularity; a stack with none runs each changed test
	// file's whole suite, adjudicated by the bound test verb's exit code alone.
	// The dispatch is ecosystem-table data (FR-16 / A-1) — no stack name appears
	// in this check; extending stack support is a data addition, not a rewrite.
	strat, focused := rc.Ecosystem.FocusedTestStrategyFor(rc.Manifest.Stack)
	globs := compileGlobs(tables.TestGlobs)

	nonTestPaths, testChanged := classifyFalsificationDiff(rc.Diff, globs)
	if len(nonTestPaths) == 0 || !testChanged {
		return Outcome{Result: schema.CheckResult{
			CheckID:    FalsificationCheckID,
			State:      "pass",
			SkipReason: "not-applicable",
			Message:    "the proposal diff has no non-test change together with an added or modified test (FR-18)",
		}}
	}

	verb, ok := rc.Verbs[falsificationTestVerbID]
	if !ok || verb == nil {
		// evaluateRung's VerbDeps gate should already have turned an unbound
		// test verb into skip:verb-absent before this function ever runs;
		// reaching here with none bound is a caller wiring bug, not a runtime
		// condition to degrade quietly.
		return falsificationErrorf("the %q verb is unbound despite being a declared dependency", falsificationTestVerbID)
	}

	targets, err := extractFalsificationTargets(ctx, rc.Workspace, rc.Bindings.HeadSHA, rc.Diff, globs, strat, focused)
	if err != nil {
		return falsificationErrorf("extract targeted tests: %v", err)
	}

	patchArgs := append([]string{"diff", "--no-color", "-M", rc.Bindings.DiffBase, rc.Bindings.HeadSHA, "--"}, nonTestPaths...)
	patch, err := runGitBytes(ctx, rc.Workspace, patchArgs...)
	if err != nil {
		return falsificationErrorf("build the non-test-half patch: %v", err)
	}

	runID := filepath.Base(rc.LogDir)
	if runID == "" || runID == "." || runID == string(filepath.Separator) {
		return falsificationErrorf("no run id available to create the ephemeral worktree")
	}

	wt, err := worktree.Create(ctx, rc.Workspace, rc.Bindings.HeadSHA, runID)
	if err != nil {
		return falsificationErrorf("create ephemeral worktree: %v", err)
	}
	defer func() { _ = wt.Remove(ctx) }() // best-effort teardown; a leak is reaped by the janitor (T9)

	_, stateDirs := rc.Ecosystem.ResolveManifestDefaults(rc.Manifest.Stack, rc.Manifest.TestGlobs, rc.Manifest.StateDirs)
	if err := wt.Provision(ctx, stateDirs); err != nil {
		return falsificationErrorf("provision ephemeral worktree: %v", err)
	}

	return runFalsificationLegs(ctx, wt, verb, targets, patch, strat, focused)
}

// falsificationErrorf builds an error-class Outcome tagged with this check's
// ID — a rung error per §3.6's exit-3 class, never a falsification verdict.
func falsificationErrorf(format string, a ...any) Outcome {
	return Outcome{Result: schema.CheckResult{CheckID: FalsificationCheckID, State: "error", Message: fmt.Sprintf(format, a...)}}
}

// classifyFalsificationDiff splits diff's files into the proposal's non-test
// half (every path outside testGlobs — added, modified, deleted, or renamed;
// what the revert leg reverses) and whether any test-glob path had content
// introduced (added, or genuinely modified — never a pure rename or a
// deletion). These are FR-18's two applicability halves. A renamed non-test
// file contributes BOTH its old and new path: `git diff -M` only pairs a
// rename when both sides are in the pathspec it is filtered to — restricting
// to the new path alone makes git render it as an unpaired new file, whose
// reverse-apply would delete the new path without ever restoring the old one
// (a corrupted worktree, not a clean revert of an ordinary rename+modify).
func classifyFalsificationDiff(diff *diffset.DiffSet, testGlobs []*regexp.Regexp) (nonTestPaths []string, testChanged bool) {
	for _, f := range diff.Files {
		if anyGlobMatch(testGlobs, f.Path) {
			if f.ContentIntroduced() {
				testChanged = true
			}
			continue
		}
		// A behaviour-neutral modify (a mode-only chmod: status M, no hunks, no
		// binary marker) introduces no content, so its reverse-apply leaves the
		// tree byte-identical — the revert leg would then falsely brand every
		// target tautological. It is not part of the implementation half. (A
		// deletion or rename IS revert-relevant and stays in.)
		if f.Status == diffset.StatusModified && !f.ContentIntroduced() {
			continue
		}
		nonTestPaths = append(nonTestPaths, f.Path)
		if f.Status == diffset.StatusRenamed && f.OldPath != "" {
			nonTestPaths = append(nonTestPaths, f.OldPath)
		}
	}
	return nonTestPaths, testChanged
}

// falsificationTarget is one test the two-leg protocol targets: a Go test
// function (Name set) discovered as new or changed by this diff, or — when no
// function-level identity is available (a non-.go test-glob match, or a
// change this scan could not attribute to one function) — a whole
// file/package fallback (Name empty).
type falsificationTarget struct {
	Path string // the test file's path at head
	Dir  string // its containing package, as a `go test` argument (e.g. "./" or "./sub/dir")
	Name string // the target Go test function name; empty means "whole package"
}

// extractFalsificationTargets resolves the new-or-changed test functions
// FR-11 targets: for an added test-glob file, every top-level Test function it
// declares; for a modified one, only the functions whose span contains an
// added line — so an unrelated, untouched sibling test in the same file is
// never swept in as a false target (which would risk a wrong-block on a
// legitimate mixed diff). A qualifying test-glob file that yields no
// function-level identity still contributes a whole-file/package target, so
// the leg covers it at coarser granularity rather than silently dropping it.
func extractFalsificationTargets(ctx context.Context, workspace, head string, diff *diffset.DiffSet, testGlobs []*regexp.Regexp, strat ecosystem.FocusedTestStrategy, focused bool) ([]falsificationTarget, error) {
	var targets []falsificationTarget
	for _, f := range diff.Files {
		if !anyGlobMatch(testGlobs, f.Path) || !f.ContentIntroduced() {
			continue
		}
		dir := packageDirOf(f.Path)

		// A stack with no focused strategy — or a file the strategy cannot
		// resolve to function-level targets (e.g. a testdata match) — is covered
		// whole-suite: one unnamed target, adjudicated by the leg's exit code.
		if !focused || !strat.TargetablePath(f.Path) {
			targets = append(targets, falsificationTarget{Path: f.Path, Dir: dir})
			continue
		}

		content, err := runGitBytes(ctx, workspace, "show", head+":"+f.Path)
		if err != nil {
			return nil, fmt.Errorf("read %s at %s: %w", f.Path, head, err)
		}

		names := strat.ExtractTargetNames(content, addedLineNums(f.AddedLines), f.Status == diffset.StatusAdded)
		if len(names) == 0 {
			targets = append(targets, falsificationTarget{Path: f.Path, Dir: dir})
			continue
		}
		for _, n := range names {
			targets = append(targets, falsificationTarget{Path: f.Path, Dir: dir, Name: n})
		}
	}
	return targets, nil
}

// addedLineNums projects a diff's AddedLine records to their line numbers — the
// shape the ecosystem focused-test strategy consumes, so ecosystem carries no
// diffset dependency.
func addedLineNums(added []diffset.AddedLine) []int {
	nums := make([]int, len(added))
	for i, a := range added {
		nums[i] = a.Line
	}
	return nums
}

// packageDirOf returns p's containing directory as a `go test` package
// argument: "./" for a root-level file, "./sub/dir" otherwise. A path inside
// one or more "testdata" directories is attributed to the nearest ancestor
// package that owns that testdata tree — by Go's own convention "testdata" is
// never itself a package (the go tool refuses to build it: "no Go files in
// ..."), which the ecosystem table's test glob ("**/testdata/**") can still
// match on its own, so a diff touching only a testdata fixture must still
// resolve to something `go test` can actually run. Diff paths are always
// repo-relative with forward slashes (git's own convention), so this uses
// "path", never "path/filepath" (which would split on the OS separator and
// corrupt the argument on Windows).
func packageDirOf(p string) string {
	parts := strings.Split(path.Dir(p), "/")
	for i, part := range parts {
		if part == "testdata" {
			parts = parts[:i]
			break
		}
	}
	dir := strings.Join(parts, "/")
	if dir == "" || dir == "." {
		return "./"
	}
	return "./" + dir
}

// runFalsificationLegs implements the two-leg protocol (design §3.9) against
// an already-provisioned worktree: the sanity leg (unreverted) must cleanly
// demonstrate every target passing, or the check errors — never a
// falsification verdict; the revert leg then applies patch and re-runs the
// same targets, naming every one that still passes as tautological.
func runFalsificationLegs(ctx context.Context, wt *worktree.EphemeralWorktree, verb *schema.Verb, targets []falsificationTarget, patch []byte, strat ecosystem.FocusedTestStrategy, focused bool) Outcome {
	legs := groupTargetsByPackage(targets)

	// Sanity leg, per package; narrow each package's checked names to those that
	// actually passed (a skipped target is dropped as not-applicable).
	for i := range legs {
		leg := runTargetedLeg(ctx, wt.Path, verb, legs[i].dir, legs[i].names, strat, focused)
		passed, reason := sanityResult(leg, legs[i])
		if reason != "" {
			return falsificationErrorf("sanity leg (%s): %s", legs[i].dir, reason)
		}
		legs[i].names = passed
	}

	if err := applyReversePatch(ctx, wt.Path, patch); err != nil {
		return falsificationErrorf("revert does not apply cleanly: %v", err) // UB-4
	}

	// Revert leg, per package: name every target that STILL passes.
	var findings []schema.Finding
	for i := range legs {
		leg := runTargetedLeg(ctx, wt.Path, verb, legs[i].dir, legs[i].names, strat, focused)
		if leg.Result.Class != "" {
			return falsificationErrorf("revert leg (%s): failed to execute: %v", legs[i].dir, leg.Result.Err)
		}
		findings = append(findings, tautologyFindings(leg, legs[i])...)
	}
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Path != findings[j].Path {
			return findings[i].Path < findings[j].Path
		}
		return findings[i].Message < findings[j].Message
	})

	result := schema.CheckResult{CheckID: FalsificationCheckID, State: "pass"}
	if len(findings) > 0 {
		result.State = "fail"
		result.Message = fmt.Sprintf("%d tautological test(s) pass with the implementation reverted", len(findings))
	}
	return Outcome{Result: result, Findings: findings}
}

// packageLeg is one target package's plan for both legs: its `go test` package
// argument, its targets, and — when EVERY target in it carries a function name —
// the -run names to focus on. A single unnamed target (a non-.go testdata match
// or an unattributable change) makes THIS ONE package whole-suite (names nil),
// never the others, so one testdata file can no longer collapse the whole
// invocation and let a genuine tautology slip (and per-package runs mean a
// same-named test in two packages never collides in one outcomes map).
type packageLeg struct {
	dir     string
	targets []falsificationTarget
	names   []string // -run names; nil => whole-suite for this package
	whole   bool
}

func groupTargetsByPackage(targets []falsificationTarget) []packageLeg {
	byDir := map[string]*packageLeg{}
	var order []string
	for _, t := range targets {
		pl, ok := byDir[t.Dir]
		if !ok {
			pl = &packageLeg{dir: t.Dir}
			byDir[t.Dir] = pl
			order = append(order, t.Dir)
		}
		pl.targets = append(pl.targets, t)
		if t.Name == "" {
			pl.whole = true
		}
	}
	legs := make([]packageLeg, 0, len(order))
	for _, d := range order {
		pl := byDir[d]
		if !pl.whole {
			nameSet := map[string]bool{}
			for _, t := range pl.targets {
				nameSet[t.Name] = true
			}
			pl.names = sortedKeys(nameSet)
		}
		legs = append(legs, *pl)
	}
	return legs
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// goTestLeg is one leg's raw execution plus its per-test outcomes (from `go
// test -json`, populated only when function-level names were available to
// filter on).
type goTestLeg struct {
	Result   runner.Result
	Outcomes map[string]string // test name -> terminal action: "pass" | "fail" | "skip"
}

// runTargetedLeg runs one package's leg. For a stack with a focused strategy
// (go), it runs the strategy's focused command (`go test -json`, filtered to
// names via -run) and parses its per-test outcomes, so a same-package sibling
// test can never pollute either leg. For a stack without one, it runs the
// bound test verb exactly as declared (whole-suite), whose only signal is its
// exit code — groupTargetsByPackage forces pl.whole for such stacks, so sanity
// and tautology both key off Result.Passed. In both cases verb supplies the
// Env/bounds DINV-5 requires.
func runTargetedLeg(ctx context.Context, worktreePath string, verb *schema.Verb, dir string, names []string, strat ecosystem.FocusedTestStrategy, focused bool) goTestLeg {
	if !focused {
		res := runner.Exec(ctx, worktreePath, verb)
		return goTestLeg{Result: res}
	}
	leg := *verb
	leg.Command = strat.BuildCommand(dir, names)
	res := runner.Exec(ctx, worktreePath, &leg)
	return goTestLeg{Result: res, Outcomes: strat.ParseOutcomes(res.Stdout)}
}

// sanityResult checks one package's sanity leg and returns the names that
// cleanly PASSED — the set the revert leg re-checks. A skipped target is
// not-applicable: dropped from that set, never a sanity failure (a test that
// skips can't be demonstrated to bite). A non-empty reason is a rung error
// (never a falsification verdict): an infra fault, a named target that failed or
// never reached a terminal outcome, or — in whole-package mode — a nonzero exit.
// This is what stops an unrunnable worktree from masquerading as "the tests
// bite" (design §3.9).
func sanityResult(leg goTestLeg, pl packageLeg) (passed []string, reason string) {
	if leg.Result.Class != "" {
		return nil, fmt.Sprintf("failed to execute: %v", leg.Result.Err)
	}
	if pl.whole {
		if !leg.Result.Passed {
			return nil, "the targeted test(s) did not pass before reverting the implementation"
		}
		return nil, "" // whole-package: the revert leg keys off the package exit, not names
	}
	var notPassed []string
	for _, n := range pl.names {
		switch leg.Outcomes[n] {
		case "pass":
			passed = append(passed, n)
		case "skip":
			// not-applicable: dropped from the checked set, never a sanity failure.
		default: // "fail", or absent (built but never reached a terminal outcome)
			notPassed = append(notPassed, n)
		}
	}
	if len(notPassed) > 0 {
		sort.Strings(notPassed)
		return nil, fmt.Sprintf("targeted test(s) did not pass before reverting the implementation: %s", strings.Join(notPassed, ", "))
	}
	return passed, ""
}

// tautologyFindings names every target that still passed in the revert leg —
// a test that actually discriminates the implementation's presence would have
// failed once it was gone.
func tautologyFindings(revert goTestLeg, pl packageLeg) []schema.Finding {
	var findings []schema.Finding

	if pl.whole {
		// Whole-package fallback: no per-function attribution, so a clean pass
		// with the implementation reverted names every targeted FILE in THIS
		// package (the coarsest identity this mode offers), deduplicated.
		if revert.Result.Passed {
			seen := map[string]bool{}
			for _, t := range pl.targets {
				if seen[t.Path] {
					continue
				}
				seen[t.Path] = true
				findings = append(findings, schema.Finding{
					CheckID: FalsificationCheckID, Class: "tautological-test", Rung: "3", Path: t.Path,
					Message: "tests in this file still pass with the implementation reverted",
				})
			}
		}
		return findings
	}

	byName := map[string]falsificationTarget{}
	for _, t := range pl.targets {
		if t.Name != "" {
			byName[t.Name] = t
		}
	}
	for _, n := range pl.names { // holds only the names that passed sanity
		if revert.Outcomes[n] != "pass" {
			continue
		}
		findings = append(findings, schema.Finding{
			CheckID: FalsificationCheckID, Class: "tautological-test", Rung: "3", Path: byName[n].Path,
			Message: fmt.Sprintf("test %s still passes with the implementation reverted", n),
		})
	}
	return findings
}

// applyReversePatch reverse-applies patch (a forward diffBase→head patch
// restricted to the non-test half) inside the worktree at worktreePath. A
// patch that does not apply cleanly — conflicting hunks, or binary content
// (a plain `git diff` never carries actual binary patch data, only a
// placeholder git apply refuses) — surfaces as this function's error, which
// the caller maps to UB-4's rung error.
func applyReversePatch(ctx context.Context, worktreePath string, patch []byte) error {
	cmd := exec.CommandContext(ctx, "git", "-C", worktreePath, "apply", "--reverse")
	cmd.Stdin = bytes.NewReader(patch)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git apply --reverse: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}
