package checks

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/verify/diffset"
	"github.com/agenticlab-ai/pyxis/internal/verify/ecosystem"
	"github.com/agenticlab-ai/pyxis/internal/verify/registry"
	"github.com/agenticlab-ai/pyxis/internal/verify/schema"
)

// --- registration --------------------------------------------------------

func TestFalsification_RegisteredOnRung3(t *testing.T) {
	c := registry.GetCheck(FalsificationCheckID)
	if c == nil {
		t.Fatal("falsification was not registered")
	}
	if c.Rung != "3" {
		t.Errorf("rung = %q, want 3", c.Rung)
	}
	if !c.AuthorIndependent {
		t.Error("falsification must be author-independent")
	}
	if len(c.VerbDeps) != 1 || c.VerbDeps[0] != "test" {
		t.Errorf("VerbDeps = %v, want [test]", c.VerbDeps)
	}
}

// --- pure logic: applicability, spans, targets ---------------------------

func testGlobRE(t *testing.T) []*regexp.Regexp {
	t.Helper()
	tables, err := TablesForStack(ecosystem.New(), "go", nil)
	if err != nil {
		t.Fatalf("TablesForStack: %v", err)
	}
	return compileGlobs(tables.TestGlobs)
}

func TestClassifyFalsificationDiff_NonTestAndTestHalves(t *testing.T) {
	globs := testGlobRE(t)
	diff := &diffset.DiffSet{Files: []diffset.FileChange{
		{Status: diffset.StatusModified, Path: "main.go", AddedLines: []diffset.AddedLine{{Line: 1, Content: "x"}}},
		{Status: diffset.StatusAdded, Path: "main_test.go", AddedLines: []diffset.AddedLine{{Line: 1, Content: "y"}}},
	}}
	nonTest, testChanged := classifyFalsificationDiff(diff, globs)
	if len(nonTest) != 1 || nonTest[0] != "main.go" {
		t.Errorf("nonTestPaths = %v, want [main.go]", nonTest)
	}
	if !testChanged {
		t.Error("testChanged = false, want true (a test file was added)")
	}
}

func TestClassifyFalsificationDiff_PureRenameOfTestNotCountedAsChange(t *testing.T) {
	globs := testGlobRE(t)
	diff := &diffset.DiffSet{Files: []diffset.FileChange{
		{Status: diffset.StatusModified, Path: "main.go", AddedLines: []diffset.AddedLine{{Line: 1, Content: "x"}}},
		{Status: diffset.StatusRenamed, Path: "pkg/main_test.go", OldPath: "main_test.go"}, // no added lines: a pure rename
	}}
	_, testChanged := classifyFalsificationDiff(diff, globs)
	if testChanged {
		t.Error("testChanged = true, want false: a pure rename introduces no test content")
	}
}

func TestClassifyFalsificationDiff_TestOnlyDiffHasEmptyNonTestHalf(t *testing.T) {
	globs := testGlobRE(t)
	diff := &diffset.DiffSet{Files: []diffset.FileChange{
		{Status: diffset.StatusModified, Path: "main_test.go", AddedLines: []diffset.AddedLine{{Line: 1, Content: "y"}}},
	}}
	nonTest, testChanged := classifyFalsificationDiff(diff, globs)
	if len(nonTest) != 0 {
		t.Errorf("nonTestPaths = %v, want empty", nonTest)
	}
	if !testChanged {
		t.Error("testChanged = false, want true")
	}
}

func TestPackageDirOf(t *testing.T) {
	cases := map[string]string{
		"main_test.go":                       "./",
		"pkg/sub/foo_test.go":                "./pkg/sub",
		"a/b/c/bar_test.go":                  "./a/b/c",
		"testdata/note.txt":                  "./",
		"pkg/testdata/note.txt":              "./pkg",
		"pkg/sub/testdata/nested/thing.json": "./pkg/sub",
	}
	for in, want := range cases {
		if got := packageDirOf(in); got != want {
			t.Errorf("packageDirOf(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestGroupTargetsByPackage_UnnamedTargetMakesOnlyItsOwnPackageWhole proves the
// per-package isolation that replaced the old all-or-nothing split: an unnamed
// target (a testdata match or an unattributable change) collapses ONLY its own
// package to whole-suite; a sibling package whose targets all carry names keeps
// its -run filter. This is what stops one testdata file from dropping the -run
// focus across every package and letting a genuine tautology slip.
func TestGroupTargetsByPackage_UnnamedTargetMakesOnlyItsOwnPackageWhole(t *testing.T) {
	targets := []falsificationTarget{
		{Path: "a_test.go", Dir: "./", Name: "TestA"},
		{Path: "a_test.go", Dir: "./", Name: "TestB"},
		{Path: "sub/testdata/x.json", Dir: "./sub"}, // no name: whole-package fallback for ./sub only
	}
	legs := groupTargetsByPackage(targets)
	if len(legs) != 2 {
		t.Fatalf("got %d legs, want 2 (one per package dir): %+v", len(legs), legs)
	}
	byDir := map[string]packageLeg{}
	for _, l := range legs {
		byDir[l.dir] = l
	}

	root, ok := byDir["./"]
	if !ok {
		t.Fatal(`missing leg for "./"`)
	}
	if root.whole {
		t.Error(`"./" leg is whole, want named (both its targets carry names)`)
	}
	if len(root.names) != 2 || root.names[0] != "TestA" || root.names[1] != "TestB" {
		t.Errorf(`"./" names = %v, want [TestA TestB]`, root.names)
	}

	sub, ok := byDir["./sub"]
	if !ok {
		t.Fatal(`missing leg for "./sub"`)
	}
	if !sub.whole {
		t.Error(`"./sub" leg is not whole, want whole (its only target is unnamed)`)
	}
	if sub.names != nil {
		t.Errorf(`"./sub" names = %v, want nil (whole-suite)`, sub.names)
	}
}

// TestGroupTargetsByPackage_AllNamedKeepsRunFilter proves a package whose
// targets all carry function names runs -run-focused (never whole-suite), so a
// same-package sibling test can never pollute the leg.
func TestGroupTargetsByPackage_AllNamedKeepsRunFilter(t *testing.T) {
	targets := []falsificationTarget{
		{Path: "a_test.go", Dir: "./", Name: "TestA"},
		{Path: "a_test.go", Dir: "./", Name: "TestB"},
	}
	legs := groupTargetsByPackage(targets)
	if len(legs) != 1 {
		t.Fatalf("got %d legs, want 1: %+v", len(legs), legs)
	}
	if legs[0].dir != "./" {
		t.Errorf("dir = %q, want ./", legs[0].dir)
	}
	if legs[0].whole {
		t.Error("leg is whole, want named")
	}
	if len(legs[0].names) != 2 || legs[0].names[0] != "TestA" || legs[0].names[1] != "TestB" {
		t.Errorf("names = %v, want [TestA TestB]", legs[0].names)
	}
}

// --- end-to-end fixtures --------------------------------------------------

// falsificationManifest is the manifest used across the end-to-end fixtures;
// stateDirs lets the withheld/provisioned state-dir tests vary just that field.
func falsificationManifest(stateDirs []string) *schema.Manifest {
	return &schema.Manifest{Version: "1.0.0", Stack: "go", StateDirs: stateDirs}
}

// buildFalsificationRC assembles the RunContext falsificationCheck reads,
// extracting the real proposal diff between base and head.
func buildFalsificationRC(t *testing.T, dir, base, head string, manifest *schema.Manifest) RunContext {
	t.Helper()
	ds, err := diffset.Extract(context.Background(), dir, base, head)
	if err != nil {
		t.Fatalf("diffset.Extract: %v", err)
	}
	return RunContext{
		Workspace: dir,
		Verbs: map[string]*schema.Verb{
			"test": {Command: "go test ./...", Env: map[string]schema.EnvEntry{
				"PATH": {Mode: schema.EnvPass},
				"HOME": {Mode: schema.EnvPass},
			}},
		},
		LogDir:    filepath.Join(dir, ".codegate", "runs", "run-falsification-test"),
		Bindings:  schema.Bindings{HeadSHA: head, DiffBase: base},
		Manifest:  manifest,
		Ecosystem: ecosystem.New(),
		Diff:      ds,
	}
}

const goModHeader = "module fixture\n\ngo 1.21\n"

// TestFalsification_TautologicalTestReds is the plan's catch-seed-1
// acceptance criterion: a new function plus a test that asserts a tautology
// (never calling the new function) reds rung 3 with a finding naming it.
func TestFalsification_TautologicalTestReds(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "go.mod", goModHeader)
	writeFile(t, dir, "main.go", "package main\n\nfunc main() {}\n")
	base := commit(t, dir, "base")

	writeFile(t, dir, "main.go", "package main\n\nfunc DoubleIt(x int) int {\n\treturn x * 2\n}\n\nfunc main() {}\n")
	writeFile(t, dir, "main_test.go", "package main\n\nimport \"testing\"\n\nfunc TestDoubleItTautological(t *testing.T) {\n\tx := 4\n\tif x != 4 {\n\t\tt.Fatal(\"impossible\")\n\t}\n}\n")
	head := commit(t, dir, "add DoubleIt with a tautological test")

	before := hashWorkspaceTree(t, dir)
	out := falsificationCheck(context.Background(), buildFalsificationRC(t, dir, base, head, falsificationManifest(nil)))
	after := hashWorkspaceTree(t, dir)

	if out.Result.State != "fail" {
		t.Fatalf("State = %q, want fail; result=%+v findings=%+v", out.Result.State, out.Result, out.Findings)
	}
	if len(out.Findings) != 1 {
		t.Fatalf("got %d finding(s), want exactly 1: %+v", len(out.Findings), out.Findings)
	}
	f := out.Findings[0]
	if f.Class != "tautological-test" || f.Rung != "3" {
		t.Errorf("finding = %+v, want class=tautological-test rung=3", f)
	}
	if !strings.Contains(f.Message, "TestDoubleItTautological") {
		t.Errorf("finding message = %q, want it to name TestDoubleItTautological", f.Message)
	}
	if before != after {
		t.Error("workspace tree hash changed across a red run (DINV-7 violated)")
	}
}

// TestFalsification_MixedGenuineTestPasses is the plan's "mixed change with
// genuine tests" fixture: the new test actually exercises the new function
// (reverting breaks its build), so no finding fires.
func TestFalsification_MixedGenuineTestPasses(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "go.mod", goModHeader)
	writeFile(t, dir, "main.go", "package main\n\nfunc main() {}\n")
	base := commit(t, dir, "base")

	writeFile(t, dir, "main.go", "package main\n\nfunc Square(x int) int {\n\treturn x * x\n}\n\nfunc main() {}\n")
	writeFile(t, dir, "main_test.go", "package main\n\nimport \"testing\"\n\nfunc TestSquare(t *testing.T) {\n\tif Square(3) != 9 {\n\t\tt.Fatal(\"bad square\")\n\t}\n}\n")
	head := commit(t, dir, "add Square with a genuine test")

	before := hashWorkspaceTree(t, dir)
	out := falsificationCheck(context.Background(), buildFalsificationRC(t, dir, base, head, falsificationManifest(nil)))
	after := hashWorkspaceTree(t, dir)

	if out.Result.State != "pass" {
		t.Fatalf("State = %q, want pass; result=%+v findings=%+v", out.Result.State, out.Result, out.Findings)
	}
	if len(out.Findings) != 0 {
		t.Errorf("findings = %+v, want none", out.Findings)
	}
	if before != after {
		t.Error("workspace tree hash changed across a passing run (DINV-7 violated)")
	}
}

// TestFalsification_TestdataOnlyTestHalf_TargetsContainingPackage proves a
// test-glob match that ISN'T a _test.go file (the ecosystem table also globs
// "**/testdata/**" — a fixture change, not a Go source file) targets the
// package that actually OWNS that testdata directory, not the testdata
// directory itself: "go test" can never run a directory holding only data
// files ("no Go files in ..."), so naively using the matched path's own
// directory would make every such ordinary diff error out on the sanity leg
// even though nothing is wrong with it.
func TestFalsification_TestdataOnlyTestHalf_TargetsContainingPackage(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "go.mod", goModHeader)
	writeFile(t, dir, "pkg/main.go", "package pkg\n\nfunc Greet() string {\n\treturn \"hello\"\n}\n")
	writeFile(t, dir, "pkg/main_test.go", "package pkg\n\nimport \"testing\"\n\nfunc TestGreet(t *testing.T) {\n\tif Greet() != \"hi\" {\n\t\tt.Fatal(\"bad\")\n\t}\n}\n")
	writeFile(t, dir, "pkg/testdata/note.txt", "v1\n")
	base := commit(t, dir, "base")

	// main_test.go is untouched between base and head: the ONLY test-glob
	// match in this diff is the testdata fixture, so the check's applicability
	// and its target extraction both run through the testdata-fallback path.
	writeFile(t, dir, "pkg/main.go", "package pkg\n\nfunc Greet() string {\n\treturn \"hi\"\n}\n")
	writeFile(t, dir, "pkg/testdata/note.txt", "v2\n")
	head := commit(t, dir, "change Greet's behavior and update its testdata fixture")

	before := hashWorkspaceTree(t, dir)
	out := falsificationCheck(context.Background(), buildFalsificationRC(t, dir, base, head, falsificationManifest(nil)))
	after := hashWorkspaceTree(t, dir)

	// TestGreet is unchanged but genuinely exercises Greet(): reverting main.go
	// makes it fail, so this is a correctly-falsified pass with no findings —
	// the point of this test is State != "error": go test must run against
	// "./pkg" (the package the testdata dir belongs to), never "./pkg/testdata"
	// (which holds no Go files and cannot build, let alone run).
	if out.Result.State == "error" {
		t.Fatalf("State = error, want pass (a testdata-only test-half change must target its owning package, not the testdata dir itself): %+v", out.Result)
	}
	if out.Result.State != "pass" {
		t.Fatalf("State = %q, want pass", out.Result.State)
	}
	if len(out.Findings) != 0 {
		t.Errorf("findings = %+v, want none", out.Findings)
	}
	if before != after {
		t.Error("workspace tree hash changed across a run (DINV-7 violated)")
	}
}

// TestFalsification_RefactorOnlyNotApplicable is the plan's "refactor-only"
// fixture (FR-18): the non-test half changes but no test file does, so the
// check records not-applicable — critically as a PASS, not a skip, so the
// overall verdict can still reach green rather than green-degraded.
func TestFalsification_RefactorOnlyNotApplicable(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "go.mod", goModHeader)
	writeFile(t, dir, "main.go", "package main\n\nfunc Greet() string {\n\treturn \"hi\"\n}\n\nfunc main() {}\n")
	writeFile(t, dir, "main_test.go", "package main\n\nimport \"testing\"\n\nfunc TestGreet(t *testing.T) {\n\tif Greet() != \"hi\" {\n\t\tt.Fatal(\"bad\")\n\t}\n}\n")
	base := commit(t, dir, "base")

	writeFile(t, dir, "main.go", "package main\n\n// refactored, same behavior\nfunc Greet() string {\n\treturn \"hi\"\n}\n\nfunc main() {}\n")
	head := commit(t, dir, "refactor only, no test change")

	out := falsificationCheck(context.Background(), buildFalsificationRC(t, dir, base, head, falsificationManifest(nil)))

	if out.Result.State != "pass" {
		t.Fatalf("State = %q, want pass (not-applicable must never demote the check below pass)", out.Result.State)
	}
	if out.Result.SkipReason != "not-applicable" {
		t.Errorf("SkipReason = %q, want not-applicable", out.Result.SkipReason)
	}
	if len(out.Findings) != 0 {
		t.Errorf("findings = %+v, want none", out.Findings)
	}
}

// TestFalsification_TestOnlyNotApplicable_TestDeltaStillEvaluates is the
// plan's "test-only" fixture: the non-test half is empty (only main_test.go
// changes), so falsification records not-applicable — while test-delta (its
// independent rung-3 sibling, T7) still evaluates the SAME diff on its own
// trigger, proving neither check gates the other.
func TestFalsification_TestOnlyNotApplicable_TestDeltaStillEvaluates(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "go.mod", goModHeader)
	writeFile(t, dir, "main.go", "package main\n\nfunc Greet() string {\n\treturn \"hi\"\n}\n\nfunc main() {}\n")
	writeFile(t, dir, "main_test.go", "package main\n\nimport \"testing\"\n\nfunc TestGreet(t *testing.T) {\n\tif Greet() != \"hi\" {\n\t\tt.Fatal(\"bad\")\n\t}\n}\n")
	base := commit(t, dir, "base")

	writeFile(t, dir, "main_test.go", "package main\n\nimport \"testing\"\n\nfunc TestGreet(t *testing.T) {\n\tif Greet() != \"hi\" {\n\t\tt.Fatal(\"bad\")\n\t}\n}\n\nfunc TestGreetAgain(t *testing.T) {\n\tif Greet() != \"hi\" {\n\t\tt.Fatal(\"bad\")\n\t}\n}\n")
	head := commit(t, dir, "test-only change")

	rc := buildFalsificationRC(t, dir, base, head, falsificationManifest(nil))

	falsOut := falsificationCheck(context.Background(), rc)
	if falsOut.Result.State != "pass" || falsOut.Result.SkipReason != "not-applicable" {
		t.Fatalf("falsification result = %+v, want pass/not-applicable", falsOut.Result)
	}

	deltaOut := testDeltaCheck(context.Background(), rc)
	if deltaOut.Result.State != "pass" {
		t.Errorf("test-delta result = %+v, want it to still evaluate (pass, since nothing was silenced here)", deltaOut.Result)
	}
}

// TestFalsification_WithheldStateDir_SanityLegErrors is the plan's "state-dir
// deliberately withheld" acceptance criterion: the test needs a directory the
// manifest never declares as a state dir, so it is never provisioned into the
// ephemeral worktree; the sanity leg (unreverted) then fails to even pass,
// which must surface as a rung error — never a falsification verdict.
func TestFalsification_WithheldStateDir_SanityLegErrors(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "go.mod", goModHeader)
	writeFile(t, dir, "main.go", "package main\n\nfunc main() {}\n")
	base := commit(t, dir, "base")

	writeFile(t, dir, "main.go", `package main

import "os"

func ReadGreeting() (string, error) {
	b, err := os.ReadFile("state/data.txt")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func main() {}
`)
	writeFile(t, dir, "main_test.go", `package main

import "testing"

func TestReadGreeting(t *testing.T) {
	got, err := ReadGreeting()
	if err != nil {
		t.Fatal(err)
	}
	if got != "hello-state" {
		t.Fatalf("got %q", got)
	}
}
`)
	head := commit(t, dir, "add ReadGreeting depending on an unprovisioned state dir")

	// state/data.txt exists on disk but is NEVER committed and NEVER declared
	// as a manifest state dir — the "deliberately withheld" fixture.
	writeFile(t, dir, "state/data.txt", "hello-state")

	before := hashWorkspaceTree(t, dir)
	out := falsificationCheck(context.Background(), buildFalsificationRC(t, dir, base, head, falsificationManifest(nil)))
	after := hashWorkspaceTree(t, dir)

	if out.Result.State != "error" {
		t.Fatalf("State = %q, want error (never a falsification verdict): result=%+v", out.Result.State, out.Result)
	}
	if len(out.Findings) != 0 {
		t.Errorf("findings = %+v, want none on a rung error", out.Findings)
	}
	if before != after {
		t.Error("workspace tree hash changed across an errored run (DINV-7 violated)")
	}
}

// TestFalsification_ProvisionedStateDir_SanityLegSucceeds is the withheld
// test's positive counterpart: declaring the SAME directory as a manifest
// state dir provisions it into the worktree, so the identical fixture's
// sanity leg passes and the check reaches a real verdict (not an error).
func TestFalsification_ProvisionedStateDir_SanityLegSucceeds(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "go.mod", goModHeader)
	writeFile(t, dir, "main.go", "package main\n\nfunc main() {}\n")
	base := commit(t, dir, "base")

	writeFile(t, dir, "main.go", `package main

import "os"

func ReadGreeting() (string, error) {
	b, err := os.ReadFile("state/data.txt")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func main() {}
`)
	writeFile(t, dir, "main_test.go", `package main

import "testing"

func TestReadGreeting(t *testing.T) {
	got, err := ReadGreeting()
	if err != nil {
		t.Fatal(err)
	}
	if got != "hello-state" {
		t.Fatalf("got %q", got)
	}
}
`)
	head := commit(t, dir, "add ReadGreeting depending on a provisioned state dir")
	writeFile(t, dir, "state/data.txt", "hello-state")

	out := falsificationCheck(context.Background(), buildFalsificationRC(t, dir, base, head, falsificationManifest([]string{"state"})))

	// Reverting ReadGreeting breaks main_test.go's build (undefined:
	// ReadGreeting), so this is a correctly-falsified pass with no findings —
	// the point of this test is State != "error": the sanity leg cleanly
	// passed because the state dir WAS provisioned.
	if out.Result.State == "error" {
		t.Fatalf("State = error even with the state dir declared: %+v", out.Result)
	}
	if out.Result.State != "pass" {
		t.Fatalf("State = %q, want pass", out.Result.State)
	}
}

// TestFalsification_RenamedNonTestFile_RevertStillRestoresOldContent proves the
// non-test-half patch survives an ordinary rename+modify: filtering `git diff`
// to only a renamed file's NEW path defeats -M's rename pairing (git sees an
// unpaired "new file" with no old side in scope), so reverse-applying it would
// merely delete the new path while never restoring the old one — corrupting
// the worktree into a state where the package fails to build for a reason
// having nothing to do with whether the target test discriminates the
// implementation. That masks the true tautology check: a build failure reads
// as "did not pass" exactly like a genuine revert would, so a tautological
// test sitting behind an unrelated rename would silently escape detection.
// pkg/other.go (untouched by the diff) calls Impl() so the package can only
// build post-revert if Impl() is genuinely restored under its old name.
func TestFalsification_RenamedNonTestFile_RevertStillRestoresOldContent(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "go.mod", goModHeader)
	writeFile(t, dir, "pkg/impl.go", "package pkg\n\nfunc Impl() int {\n\treturn 1\n}\n")
	writeFile(t, dir, "pkg/other.go", "package pkg\n\nfunc UseImpl() int {\n\treturn Impl() + 100\n}\n")
	base := commit(t, dir, "base")

	if err := os.Remove(filepath.Join(dir, "pkg/impl.go")); err != nil {
		t.Fatalf("remove pkg/impl.go: %v", err)
	}
	writeFile(t, dir, "pkg/impl2.go", "package pkg\n\nfunc Impl() int {\n\treturn 2\n}\n")
	// The new test is genuinely tautological: it never calls anything from pkg,
	// so it is unaffected by whether Impl() is reverted — the correct verdict
	// is a falsification finding naming it.
	writeFile(t, dir, "pkg/main_test.go", "package pkg\n\nimport \"testing\"\n\nfunc TestTautological(t *testing.T) {\n\tif 1+1 != 2 {\n\t\tt.Fatal(\"impossible\")\n\t}\n}\n")
	head := commit(t, dir, "rename+modify impl.go alongside a tautological test")

	before := hashWorkspaceTree(t, dir)
	out := falsificationCheck(context.Background(), buildFalsificationRC(t, dir, base, head, falsificationManifest(nil)))
	after := hashWorkspaceTree(t, dir)

	if out.Result.State != "fail" {
		t.Fatalf("State = %q, want fail (the rename must not corrupt the revert into masking a real tautology): result=%+v findings=%+v", out.Result.State, out.Result, out.Findings)
	}
	if len(out.Findings) != 1 || !strings.Contains(out.Findings[0].Message, "TestTautological") {
		t.Fatalf("findings = %+v, want exactly one naming TestTautological", out.Findings)
	}
	if before != after {
		t.Error("workspace tree hash changed across a run (DINV-7 violated)")
	}
}

// TestFalsification_BinaryNonTestFile_RevertInapplicableErrors is UB-4: a
// binary file in the non-test half produces a patch git itself cannot
// reverse-apply (a plain diff never carries real binary patch data), which
// must surface as a rung error, never a false pass and never a silent skip.
func TestFalsification_BinaryNonTestFile_RevertInapplicableErrors(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "go.mod", goModHeader)
	writeFile(t, dir, "main.go", "package main\n\nfunc main() {}\n")
	base := commit(t, dir, "base")

	if err := os.WriteFile(filepath.Join(dir, "asset.bin"), []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x01, 0x02, 0x03}, 0o644); err != nil {
		t.Fatalf("write binary asset: %v", err)
	}
	writeFile(t, dir, "main_test.go", "package main\n\nimport \"testing\"\n\nfunc TestTrivial(t *testing.T) {\n\tif 1 != 1 {\n\t\tt.Fatal(\"impossible\")\n\t}\n}\n")
	head := commit(t, dir, "add a binary asset alongside a test")

	before := hashWorkspaceTree(t, dir)
	out := falsificationCheck(context.Background(), buildFalsificationRC(t, dir, base, head, falsificationManifest(nil)))
	after := hashWorkspaceTree(t, dir)

	if out.Result.State != "error" {
		t.Fatalf("State = %q, want error (UB-4: an inapplicable revert must never verdict): result=%+v", out.Result.State, out.Result)
	}
	if len(out.Findings) != 0 {
		t.Errorf("findings = %+v, want none on a rung error", out.Findings)
	}
	if before != after {
		t.Error("workspace tree hash changed across an errored (UB-4) run (DINV-7 violated)")
	}
}

// --- test helpers ----------------------------------------------------------

// hashWorkspaceTree hashes every path+content under dir, excluding .git
// (git's own internal bookkeeping) and .codegate (codegate's own scratch,
// explicitly out of scope for the tree-identity guarantee DINV-7 makes).
func hashWorkspaceTree(t *testing.T, dir string) string {
	t.Helper()
	h := sha256.New()
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(dir, p)
		if relErr != nil {
			return relErr
		}
		if rel == "." {
			return nil
		}
		if rel == ".git" || rel == ".codegate" {
			return filepath.SkipDir
		}
		h.Write([]byte(rel))
		if d.IsDir() {
			return nil
		}
		data, rerr := os.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		h.Write(data)
		return nil
	})
	if err != nil {
		t.Fatalf("hash workspace tree %s: %v", dir, err)
	}
	return hex.EncodeToString(h.Sum(nil))
}
