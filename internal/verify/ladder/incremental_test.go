package ladder

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/verify/invoke"
	"github.com/agenticlab-ai/pyxis/internal/verify/registry"
	"github.com/agenticlab-ai/pyxis/internal/verify/schema"
)

func qualifyingBindings() schema.Bindings {
	return schema.Bindings{
		HeadSHA:         "new-head-sha",
		DiffBase:        "diff-base-sha",
		ManifestHash:    "manifest-hash",
		BaselineHash:    "baseline-hash",
		CodegateVersion: "1.0.0-test",
		GitVersion:      "2.40.0",
	}
}

func priorVerdictFor(bindings schema.Bindings, priorHeadSHA string) *schema.Verdict {
	pb := bindings
	pb.HeadSHA = priorHeadSHA
	return &schema.Verdict{
		RunID: "run-prior-1",
		State: "green",
		Rungs: map[string]*schema.Rung{
			"0": {Name: "0", State: "pass", Checks: map[string]*schema.CheckResult{"format": {CheckID: "format", State: "pass"}}},
		},
		Bindings: pb,
	}
}

// --- QualifiesForIncremental ------------------------------------------------

func TestQualifiesForIncremental_NilPrior(t *testing.T) {
	if QualifiesForIncremental(nil, qualifyingBindings()) {
		t.Error("a nil prior must never qualify")
	}
}

func TestQualifiesForIncremental_AllBindingsMatchExceptHead(t *testing.T) {
	bindings := qualifyingBindings()
	prior := priorVerdictFor(bindings, "old-head-sha")
	if !QualifiesForIncremental(prior, bindings) {
		t.Error("a prior verdict differing only in head SHA must qualify")
	}
}

// The task's Scope explicitly calls out these four fixtures: changing any
// single one of manifest hash, baseline hash, diff base, or codegate version
// must force a full run (never qualify). Git version is DINV-1's sixth
// binding and gets the identical treatment for the identical reason.
func TestQualifiesForIncremental_AnyOtherBindingMismatchForcesFullRun(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*schema.Bindings)
	}{
		{"diff base changed", func(b *schema.Bindings) { b.DiffBase = "different-diff-base" }},
		{"manifest hash changed", func(b *schema.Bindings) { b.ManifestHash = "different-manifest-hash" }},
		{"baseline hash changed", func(b *schema.Bindings) { b.BaselineHash = "different-baseline-hash" }},
		{"codegate version changed", func(b *schema.Bindings) { b.CodegateVersion = "2.0.0" }},
		{"git version changed", func(b *schema.Bindings) { b.GitVersion = "2.41.0" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bindings := qualifyingBindings()
			prior := priorVerdictFor(bindings, "old-head-sha")
			tt.mutate(&prior.Bindings) // mutate only the PRIOR verdict's recorded binding
			if QualifiesForIncremental(prior, bindings) {
				t.Errorf("%s: must force a full run (not qualify)", tt.name)
			}
		})
	}
}

func TestQualifiesForIncremental_IncompleteBindingsNeverQualifies(t *testing.T) {
	bindings := qualifyingBindings()
	prior := priorVerdictFor(bindings, "old-head-sha")
	prior.Bindings.GitVersion = ""
	if QualifiesForIncremental(prior, bindings) {
		t.Error("a prior verdict with incomplete bindings must never qualify")
	}
}

func TestQualifiesForIncremental_EmptyRunIDNeverQualifies(t *testing.T) {
	bindings := qualifyingBindings()
	prior := priorVerdictFor(bindings, "old-head-sha")
	prior.RunID = ""
	if QualifiesForIncremental(prior, bindings) {
		t.Error("a prior verdict with no run id must never qualify (nothing to carry-from)")
	}
}

func TestQualifiesForIncremental_InvalidPriorStructureNeverQualifies(t *testing.T) {
	bindings := qualifyingBindings()
	prior := priorVerdictFor(bindings, "old-head-sha")
	// A pass-class verdict containing a skip:not-reached rung violates DINV-3
	// — a corrupted or hand-edited artifact must never be trusted as state.
	prior.Rungs["1"] = &schema.Rung{Name: "1", Skipped: true, SkipReason: "not-reached"}
	if QualifiesForIncremental(prior, bindings) {
		t.Error("a structurally invalid prior verdict must never qualify")
	}
}

// --- PlanIncremental ---------------------------------------------------------

func treeRung(name string) *registry.Rung {
	return &registry.Rung{Name: name, Checks: []*schema.Check{{ID: name + "-check", Rung: name, InputSet: []string{"tree"}}}}
}

func diffRung(name string) *registry.Rung {
	return &registry.Rung{Name: name, Checks: []*schema.Check{{ID: name + "-check", Rung: name, InputSet: []string{"diff"}}}}
}

// mixedRung has one "tree"-tagged and one "diff"-tagged check — rung 0's real
// shape (format/lint are tree; secret-scan et al. are diff) — proving the
// rung-level tag is the UNION of its checks: any "tree" check makes the
// whole rung whole-tree-sensitive.
func mixedRung(name string) *registry.Rung {
	return &registry.Rung{Name: name, Checks: []*schema.Check{
		{ID: name + "-tree-check", Rung: name, InputSet: []string{"tree"}},
		{ID: name + "-diff-check", Rung: name, InputSet: []string{"diff"}},
	}}
}

func TestPlanIncremental_PreviouslyFailedRungAlwaysReruns(t *testing.T) {
	prior := &schema.Verdict{RunID: "run-1", Rungs: map[string]*schema.Rung{
		"0": {Name: "0", State: "fail"},
	}}
	plan := PlanIncremental(prior, []*registry.Rung{treeRung("0")}, nil, nil)
	if !plan.Rerun["0"] {
		t.Error("a previously-failed rung must always rerun")
	}
	if _, carried := plan.Carry["0"]; carried {
		t.Error("a previously-failed rung must never carry")
	}
}

func TestPlanIncremental_NotReachedRungAlwaysReruns(t *testing.T) {
	prior := &schema.Verdict{RunID: "run-1", Rungs: map[string]*schema.Rung{
		"3": {Name: "3", Skipped: true, SkipReason: "not-reached"},
	}}
	plan := PlanIncremental(prior, []*registry.Rung{diffRung("3")}, nil, nil)
	if !plan.Rerun["3"] {
		t.Error("a not-reached rung must always rerun — a pass-class verdict can never contain a never-run rung")
	}
}

func TestPlanIncremental_ErroredRungAlwaysReruns(t *testing.T) {
	prior := &schema.Verdict{RunID: "run-1", Rungs: map[string]*schema.Rung{
		"1": {Name: "1", State: "error"},
	}}
	plan := PlanIncremental(prior, []*registry.Rung{treeRung("1")}, nil, nil)
	if !plan.Rerun["1"] {
		t.Error("an errored rung must always rerun")
	}
}

func TestPlanIncremental_RungAbsentFromPriorAlwaysReruns(t *testing.T) {
	prior := &schema.Verdict{RunID: "run-1", Rungs: map[string]*schema.Rung{}}
	plan := PlanIncremental(prior, []*registry.Rung{treeRung("4")}, nil, nil)
	if !plan.Rerun["4"] {
		t.Error("a rung the prior verdict never recorded must rerun")
	}
}

func TestPlanIncremental_TreeTaggedRung_AnyChangeInvalidates(t *testing.T) {
	prior := &schema.Verdict{RunID: "run-1", Rungs: map[string]*schema.Rung{
		"0": {Name: "0", State: "pass", Checks: map[string]*schema.CheckResult{"0-check": {State: "pass"}}},
	}}
	plan := PlanIncremental(prior, []*registry.Rung{treeRung("0")}, map[string]bool{"unrelated.go": true}, nil)
	if !plan.Rerun["0"] {
		t.Error("a tree-tagged rung must rerun whenever the fix touched ANY file")
	}
	if _, carried := plan.Carry["0"]; carried {
		t.Error("a tree-tagged rung must not carry when the fix delta is non-empty")
	}
}

func TestPlanIncremental_TreeTaggedRung_NoChangeCarries(t *testing.T) {
	prior := &schema.Verdict{RunID: "run-1", Rungs: map[string]*schema.Rung{
		"0": {Name: "0", State: "pass", Checks: map[string]*schema.CheckResult{"0-check": {State: "pass"}}},
	}}
	plan := PlanIncremental(prior, []*registry.Rung{treeRung("0")}, map[string]bool{}, nil)
	carried, ok := plan.Carry["0"]
	if !ok {
		t.Fatal("a tree-tagged rung with an empty fix delta must carry")
	}
	if carried.Carried == nil || carried.Carried.FromRunID != "run-1" {
		t.Errorf("Carried = %+v, want FromRunID = run-1", carried.Carried)
	}
	if carried.Duration != 0 {
		t.Errorf("Duration = %v, want 0 (nothing executed this run)", carried.Duration)
	}
	if carried.State != "pass" {
		t.Errorf("State = %q, want the carried pass state preserved", carried.State)
	}
}

// TestPlanIncremental_CarryChain_PreservesTrueOriginProvenance proves
// carried-from tracks back to the run that ACTUALLY executed the rung, not
// merely the immediately preceding verdict: prior is itself already a
// carried pass (Carried.FromRunID = "run-1", stamped two hops back), so a
// third consecutive carry must still report "run-1" — never "run-2", the
// immediate prior's own id, which itself never executed this rung either
// (design §3.4's scratch retention keeps only the current run's scratch plus
// ONE prior verdict, so a hop-by-hop reference would go stale after exactly
// this scenario).
func TestPlanIncremental_CarryChain_PreservesTrueOriginProvenance(t *testing.T) {
	prior := &schema.Verdict{RunID: "run-2", Rungs: map[string]*schema.Rung{
		"0": {Name: "0", State: "pass", Checks: map[string]*schema.CheckResult{"0-check": {State: "pass"}}, Carried: &schema.CarriedPass{FromRunID: "run-1"}},
	}}
	plan := PlanIncremental(prior, []*registry.Rung{treeRung("0")}, map[string]bool{}, nil)
	carried, ok := plan.Carry["0"]
	if !ok {
		t.Fatal("an already-carried pass with an empty fix delta must carry again")
	}
	if carried.Carried == nil || carried.Carried.FromRunID != "run-1" {
		t.Errorf("Carried = %+v, want FromRunID = run-1 (the run that actually executed it), not run-2 (the intermediate carry)", carried.Carried)
	}
}

func TestPlanIncremental_DiffTaggedRung_NewPathInvalidates(t *testing.T) {
	prior := &schema.Verdict{RunID: "run-1", Rungs: map[string]*schema.Rung{
		"3": {Name: "3", State: "pass"},
	}}
	fixDelta := map[string]bool{"new_test.go": true}
	newDiffPaths := map[string]bool{"new_test.go": true} // a path never before part of the diff
	plan := PlanIncremental(prior, []*registry.Rung{diffRung("3")}, fixDelta, newDiffPaths)
	if !plan.Rerun["3"] {
		t.Error("a diff-tagged rung must rerun when the fix introduces a path new to the diff's composition")
	}
}

func TestPlanIncremental_DiffTaggedRung_EditWithinExistingDiffCarries(t *testing.T) {
	prior := &schema.Verdict{RunID: "run-1", Rungs: map[string]*schema.Rung{
		"3": {Name: "3", State: "pass", Checks: map[string]*schema.CheckResult{"3-check": {State: "pass"}}},
	}}
	// The fix touched a file, but that file was ALREADY part of the diff as of
	// the prior run (newDiffPaths is empty) — the diff's composition (which
	// files, test-or-not) didn't move.
	fixDelta := map[string]bool{"probe/main.go": true}
	newDiffPaths := map[string]bool{}
	plan := PlanIncremental(prior, []*registry.Rung{diffRung("3")}, fixDelta, newDiffPaths)
	carried, ok := plan.Carry["3"]
	if !ok {
		t.Fatal("a diff-tagged rung whose composition is unchanged must carry")
	}
	if carried.Carried.FromRunID != "run-1" {
		t.Errorf("FromRunID = %q, want run-1", carried.Carried.FromRunID)
	}
}

func TestPlanIncremental_MixedRungBehavesAsTree(t *testing.T) {
	prior := &schema.Verdict{RunID: "run-1", Rungs: map[string]*schema.Rung{
		"0": {Name: "0", State: "pass", Checks: map[string]*schema.CheckResult{"0-tree-check": {State: "pass"}, "0-diff-check": {State: "pass"}}},
	}}
	// newDiffPaths is empty (the "diff" half would carry alone), but the
	// mixed rung also carries a "tree"-tagged check (format/lint's real
	// shape for rung 0), which must dominate.
	fixDelta := map[string]bool{"probe/main.go": true}
	plan := PlanIncremental(prior, []*registry.Rung{mixedRung("0")}, fixDelta, map[string]bool{})
	if !plan.Rerun["0"] {
		t.Error("a rung mixing a tree-tagged check must rerun on any fix delta, even if its diff-tagged check's composition is unchanged")
	}
}

func TestPlanIncremental_EmptyFixDeltaNeverInvalidates(t *testing.T) {
	// A degenerate edge case (head differs with no path-level delta, e.g. an
	// empty commit): with nothing changed, neither tag has anything to
	// invalidate it on.
	prior := &schema.Verdict{RunID: "run-1", Rungs: map[string]*schema.Rung{
		"0": {Name: "0", State: "pass", Checks: map[string]*schema.CheckResult{"0-check": {State: "pass"}}},
		"3": {Name: "3", State: "pass", Checks: map[string]*schema.CheckResult{"3-check": {State: "pass"}}},
	}}
	plan := PlanIncremental(prior, []*registry.Rung{treeRung("0"), diffRung("3")}, map[string]bool{}, map[string]bool{})
	if _, ok := plan.Carry["0"]; !ok {
		t.Error("tree rung should carry when the fix delta is empty")
	}
	if _, ok := plan.Carry["3"]; !ok {
		t.Error("diff rung should carry when the fix delta is empty")
	}
}

func TestPlanIncremental_RerunAndCarryPartitionTheLadder(t *testing.T) {
	prior := &schema.Verdict{RunID: "run-1", Rungs: map[string]*schema.Rung{
		"0": {Name: "0", State: "pass", Checks: map[string]*schema.CheckResult{"0-check": {State: "pass"}}},
		"1": {Name: "1", State: "fail"},
		"3": {Name: "3", Skipped: true, SkipReason: "not-reached"},
	}}
	rungs := []*registry.Rung{treeRung("0"), treeRung("1"), diffRung("3")}
	plan := PlanIncremental(prior, rungs, map[string]bool{}, map[string]bool{})

	if len(plan.Rerun)+len(plan.Carry) != len(rungs) {
		t.Fatalf("Rerun(%d) + Carry(%d) != len(rungs)(%d): every rung must land in exactly one set", len(plan.Rerun), len(plan.Carry), len(rungs))
	}
	if !plan.Rerun["1"] || !plan.Rerun["3"] {
		t.Errorf("Rerun = %v, want it to include the failed rung 1 and the not-reached rung 3", plan.Rerun)
	}
	if _, ok := plan.Carry["0"]; !ok {
		t.Errorf("Carry = %v, want it to include the previously-passed, untouched rung 0", plan.Carry)
	}
}

// --- RunIncremental end-to-end -----------------------------------------------

// initGitRepo creates a hermetic git repo at a fresh temp dir with an
// isolated identity, so the host environment can never leak in (mirrors
// checks/testhelpers_test.go's helper of the same shape).
//
// It also excludes the scratch directory via .git/info/exclude, exactly the
// local-metadata mechanism design §3.6 gives codegate to keep its own
// scratch out of the tree it verifies. These fixtures call Run/RunIncremental
// directly (never through cmd/codegate, which is where that exclusion would
// normally be established at first invocation), then commit again to
// simulate a fix cycle — without this, the first run's own scratch log files
// would be untracked, `git add -A` would sweep them into the "fix" commit,
// and the fix's own diff would falsely appear to touch paths it never did.
func initGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_AUTHOR_NAME", "codegate-test")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "codegate-test")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@example.com")
	gitCmd(t, dir, "init", "-q", "-b", "main")
	excludePath := filepath.Join(dir, ".git", "info", "exclude")
	if err := os.WriteFile(excludePath, []byte(invoke.ScratchDirRelPath+"/\n/.pyxis/logs/\n"), 0o644); err != nil {
		t.Fatalf("write .git/info/exclude: %v", err)
	}
	return dir
}

func writeTestFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	full := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func commitAll(t *testing.T, dir, msg string) string {
	t.Helper()
	gitCmd(t, dir, "add", "-A")
	gitCmd(t, dir, "commit", "-q", "-m", msg)
	return gitCmd(t, dir, "rev-parse", "HEAD")
}

// execProbeVerb is the exec-probe verb binding the incremental fixture below
// uses (mirrors TestRun_AllVerbsBound_Green's pattern).
func execProbeVerb() *schema.Verb {
	return &schema.Verb{
		Command: "go run ./probe",
		Env: map[string]schema.EnvEntry{
			"PATH": {Mode: schema.EnvPass},
			"HOME": {Mode: schema.EnvPass},
		},
	}
}

// executedCheckCount counts check results belonging to rungs that actually
// ran this invocation — i.e. NOT carried forward and NOT skip:not-reached —
// the proxy this test uses for "verb runs" (design codegate-v1 §3.8, the
// plan's A-5 acceptance bar): a carried rung invokes no verb at all, so
// fewer executed checks directly implies fewer verb invocations.
func executedCheckCount(v *schema.Verdict) int {
	n := 0
	for _, r := range v.Rungs {
		if r.Carried != nil || r.Skipped {
			continue
		}
		n += len(r.Checks)
	}
	return n
}

// TestRunIncremental_CarriesUnaffectedRung_FewerChecksExecuted is T8's core
// fixture fix-cycle (design codegate-v1 §3.8, plan A-5): a full run finds
// rung 4 (exec-probe) broken while rung 3 (falsification + test-delta) has
// genuinely passed on a real, non-tautological test — the two-verb-run
// falsification case (mirrors checks.TestFalsification_MixedGenuineTestPasses's
// sanity+revert legs). The fix touches ONLY the already-diffed probe file, so
// rung 3's diff composition never moves: the incremental run reruns the
// tree-tagged rungs (0, 1, and the previously-failed 4) but carries rung 3
// forward untouched, with carried-from provenance, executing strictly fewer
// checks than the full run did.
func TestRunIncremental_CarriesUnaffectedRung_FewerChecksExecuted(t *testing.T) {
	dir := initGitRepo(t)
	writeTestFile(t, dir, "go.mod", "module fixture\n\ngo 1.21\n")
	writeTestFile(t, dir, "main.go", "package main\n\nfunc main() {}\n")
	diffBase := commitAll(t, dir, "base")

	// The proposal: a genuine (non-tautological) Square function + test —
	// falsification's applicable, cleanly-passing path — alongside a probe
	// that regresses.
	writeTestFile(t, dir, "main.go", "package main\n\nfunc Square(x int) int {\n\treturn x * x\n}\n\nfunc main() {}\n")
	writeTestFile(t, dir, "main_test.go", "package main\n\nimport \"testing\"\n\nfunc TestSquare(t *testing.T) {\n\tif Square(3) != 9 {\n\t\tt.Fatal(\"bad square\")\n\t}\n}\n")
	writeTestFile(t, dir, "probe/main.go", "package main\n\nimport (\n\t\"fmt\"\n\t\"os\"\n)\n\nfunc main() {\n\tfmt.Fprintln(os.Stderr, \"probe: broken\")\n\tos.Exit(1)\n}\n")
	head1 := commitAll(t, dir, "add Square with a genuine test; probe regresses")

	manifest := &schema.Manifest{Version: "1.0.0", Stack: "go", Verbs: map[string]*schema.Verb{"exec-probe": execProbeVerb()}}
	bindings1 := schema.Bindings{
		HeadSHA: head1, DiffBase: diffBase,
		ManifestHash: "manifest-hash", BaselineHash: "baseline-hash",
		CodegateVersion: "1.0.0-test", GitVersion: "2.40.0",
	}

	verdict1 := Run(context.Background(), dir, manifest, bindings1, "full")

	if verdict1.State != "red" {
		t.Fatalf("verdict1.State = %q, want red (exec-probe is broken): %+v", verdict1.State, verdict1)
	}
	for _, name := range []string{"0", "1", "3"} {
		r, ok := verdict1.Rungs[name]
		if !ok || r.State != "pass" {
			t.Fatalf("verdict1 rung %s = %+v, want a clean pass", name, r)
		}
	}
	if r4, ok := verdict1.Rungs["4"]; !ok || r4.State != "fail" {
		t.Fatalf("verdict1 rung 4 = %+v, want fail (broken probe)", verdict1.Rungs["4"])
	}

	// The fix: touch ONLY probe/main.go — a path already part of the diff —
	// to repair exec-probe. The diff's file composition never moves.
	writeTestFile(t, dir, "probe/main.go", "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"probe: fixed\")\n}\n")
	head2 := commitAll(t, dir, "fix the probe")

	bindings2 := bindings1
	bindings2.HeadSHA = head2

	verdict2 := RunIncremental(context.Background(), dir, manifest, bindings2, verdict1)

	if verdict2.Mode != "incremental" {
		t.Fatalf("verdict2.Mode = %q, want incremental (the prior verdict must qualify): %+v", verdict2.Mode, verdict2)
	}
	if verdict2.CarriedFrom != verdict1.RunID {
		t.Errorf("CarriedFrom = %q, want %q", verdict2.CarriedFrom, verdict1.RunID)
	}
	if verdict2.State != "green" {
		t.Fatalf("verdict2.State = %q, want green (verdict: %+v)", verdict2.State, verdict2)
	}

	rung3, ok := verdict2.Rungs["3"]
	if !ok {
		t.Fatal("verdict2 rung 3 missing")
	}
	if rung3.Carried == nil || rung3.Carried.FromRunID != verdict1.RunID {
		t.Errorf("rung 3 Carried = %+v, want FromRunID = %s", rung3.Carried, verdict1.RunID)
	}
	for _, name := range []string{"0", "1", "4"} {
		r, ok := verdict2.Rungs[name]
		if !ok {
			t.Fatalf("verdict2 rung %s missing", name)
		}
		if r.Carried != nil {
			t.Errorf("rung %s Carried = %+v, want nil — it must have actually re-run (tree-tagged, or the previously-failed rung)", name, r.Carried)
		}
		if r.State != "pass" {
			t.Errorf("rung %s State = %q, want pass", name, r.State)
		}
	}

	executed1, executed2 := executedCheckCount(verdict1), executedCheckCount(verdict2)
	if executed2 >= executed1 {
		t.Errorf("executed checks: full run = %d, incremental run = %d, want strictly fewer (rung 3's carrying should skip both falsification and test-delta)", executed1, executed2)
	}
}

// TestRunIncremental_NonQualifyingPrior_FallsBackToFull proves --incremental
// never launders a stale result: a manifest-hash mismatch (one of the four
// invalidation fixtures the task's Scope names) makes the same two-commit fix
// cycle run the identical full ladder Run would — no carrying, no error,
// mode stamped "full".
func TestRunIncremental_NonQualifyingPrior_FallsBackToFull(t *testing.T) {
	dir := initGitRepo(t)
	writeTestFile(t, dir, "go.mod", "module fixture\n\ngo 1.21\n")
	writeTestFile(t, dir, "main.go", "package main\n\nfunc main() {}\n")
	diffBase := commitAll(t, dir, "base")

	writeTestFile(t, dir, "main.go", "package main\n\nfunc Square(x int) int {\n\treturn x * x\n}\n\nfunc main() {}\n")
	writeTestFile(t, dir, "main_test.go", "package main\n\nimport \"testing\"\n\nfunc TestSquare(t *testing.T) {\n\tif Square(3) != 9 {\n\t\tt.Fatal(\"bad square\")\n\t}\n}\n")
	writeTestFile(t, dir, "probe/main.go", "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"probe: ok\")\n}\n")
	head1 := commitAll(t, dir, "add Square with a genuine test")

	manifest := &schema.Manifest{Version: "1.0.0", Stack: "go", Verbs: map[string]*schema.Verb{"exec-probe": execProbeVerb()}}
	bindings1 := schema.Bindings{
		HeadSHA: head1, DiffBase: diffBase,
		ManifestHash: "manifest-hash-v1", BaselineHash: "baseline-hash",
		CodegateVersion: "1.0.0-test", GitVersion: "2.40.0",
	}
	verdict1 := Run(context.Background(), dir, manifest, bindings1, "full")
	if verdict1.State != "green" {
		t.Fatalf("verdict1.State = %q, want green: %+v", verdict1.State, verdict1)
	}

	writeTestFile(t, dir, "README.md", "unrelated follow-up\n")
	head2 := commitAll(t, dir, "unrelated follow-up")

	bindings2 := bindings1
	bindings2.HeadSHA = head2
	bindings2.ManifestHash = "manifest-hash-v2" // the manifest changed between runs

	verdict2 := RunIncremental(context.Background(), dir, manifest, bindings2, verdict1)

	if verdict2.Mode != "full" {
		t.Errorf("verdict2.Mode = %q, want full (a manifest hash mismatch must never qualify)", verdict2.Mode)
	}
	if verdict2.CarriedFrom != "" {
		t.Errorf("CarriedFrom = %q, want empty on a full-fallback run", verdict2.CarriedFrom)
	}
	for name, r := range verdict2.Rungs {
		if r.Carried != nil {
			t.Errorf("rung %s Carried = %+v, want nil — nothing may carry on a non-qualifying prior", name, r.Carried)
		}
	}
}

// TestRunIncremental_MissingPriorFallsBackToFull is the plan's "missing or
// unparseable prior verdict silently degrades to a full run" acceptance
// criterion, exercised at RunIncremental's own boundary (nil prior, exactly
// what schema.LoadVerdict returns for a missing file).
func TestRunIncremental_MissingPriorFallsBackToFull(t *testing.T) {
	dir := initGitRepo(t)
	writeTestFile(t, dir, "go.mod", "module fixture\n\ngo 1.21\n")
	writeTestFile(t, dir, "main.go", "package main\n\nfunc main() {}\n")
	writeTestFile(t, dir, "probe/main.go", "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"probe: ok\")\n}\n")
	head := commitAll(t, dir, "base")

	manifest := &schema.Manifest{Version: "1.0.0", Stack: "go", Verbs: map[string]*schema.Verb{"exec-probe": execProbeVerb()}}
	bindings := schema.Bindings{
		HeadSHA: head, DiffBase: head,
		ManifestHash: "manifest-hash", BaselineHash: "baseline-hash",
		CodegateVersion: "1.0.0-test", GitVersion: "2.40.0",
	}

	verdict := RunIncremental(context.Background(), dir, manifest, bindings, nil)

	if verdict.Mode != "full" {
		t.Errorf("Mode = %q, want full", verdict.Mode)
	}
	if verdict.State != "green" {
		t.Errorf("State = %q, want green: %+v", verdict.State, verdict)
	}
}
