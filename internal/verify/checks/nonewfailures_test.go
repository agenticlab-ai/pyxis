package checks

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/verify/baseline"
	"github.com/agenticlab-ai/pyxis/internal/verify/ecosystem"
	"github.com/agenticlab-ai/pyxis/internal/verify/registry"
	"github.com/agenticlab-ai/pyxis/internal/verify/schema"
)

func TestNoNewFailures_RegisteredOnRung2(t *testing.T) {
	c := registry.GetCheck(noNewFailuresCheckID)
	if c == nil {
		t.Fatal("no-new-failures was not registered")
	}
	if c.Rung != "2" {
		t.Errorf("rung = %q, want 2", c.Rung)
	}
	if len(c.VerbDeps) != 1 || c.VerbDeps[0] != "test" {
		t.Errorf("VerbDeps = %v, want [test]", c.VerbDeps)
	}
}

// goTestWorkspace writes a minimal go module whose one test body is testBody
// (a `func TestX(...)` declaration). The check runs `go test -json ./...`
// against it for real — the failure identities it reds on come from the go
// toolchain, not a hand-built stream.
func goTestWorkspace(t *testing.T, testBody string) string {
	t.Helper()
	ws := t.TempDir()
	writeFile(t, ws, "go.mod", "module rung2fixture\n\ngo 1.21\n")
	writeFile(t, ws, "lib.go", "package rung2fixture\n\n// Lib keeps the package non-empty for `go build`.\nfunc Lib() int { return 1 }\n")
	writeFile(t, ws, "lib_test.go", "package rung2fixture\n\nimport \"testing\"\n\n"+testBody)
	return ws
}

// rung2Context builds the RunContext runNoNewFailures reads: the go verb set
// (optionally overriding "test"), the stack/baseline manifest, and a scratch
// log dir. Bindings are unused (rung 2 is tree-based).
func rung2Context(t *testing.T, ws, stack, baselinePath string, testVerbOverride *schema.Verb) RunContext {
	t.Helper()
	eco := ecosystem.New()
	overrides := map[string]*schema.Verb{}
	if testVerbOverride != nil {
		overrides["test"] = testVerbOverride
	}
	return RunContext{
		Workspace: ws,
		Verbs:     eco.ResolveVerbs(stack, overrides),
		Manifest:  &schema.Manifest{Version: "1", Stack: stack, BaselinePath: baselinePath},
		Ecosystem: eco,
		LogDir:    filepath.Join(ws, ".codegate", "runs", "test"),
	}
}

// writeBaseline captures ids into a baseline file at ws/rel and returns rel
// (the manifest-relative path the check loads).
func writeBaseline(t *testing.T, ws, rel string, ids ...string) string {
	t.Helper()
	b := baseline.Capture(ids, "test-version", "test", "output-hash", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	if err := b.WriteAtomicToFile(filepath.Join(ws, rel)); err != nil {
		t.Fatalf("write baseline: %v", err)
	}
	return rel
}

func runRung2(t *testing.T, rc RunContext) Outcome {
	t.Helper()
	return runNoNewFailures(context.Background(), rc)
}

// A stack with no failure-identity parser cannot compute identities: it skips
// (green-degraded), so python/node fixtures never red rung 2 on an unparseable
// runner — the property that keeps the T10 matrix passing.
func TestNoNewFailures_SkipWhenNoParser(t *testing.T) {
	rc := rung2Context(t, t.TempDir(), "python", "", nil)
	out := runRung2(t, rc)
	if out.Result.State != "skip" || out.Result.SkipReason != "parser-absent" {
		t.Errorf("state=%q skip=%q, want skip/parser-absent", out.Result.State, out.Result.SkipReason)
	}
}

// A baseline declared for a parser-less stack is a misconfiguration: fail
// closed rather than silently certify an evaluation the stack cannot perform.
func TestNoNewFailures_NoParserButBaselineBound_Errors(t *testing.T) {
	ws := t.TempDir()
	rel := writeBaseline(t, ws, "baseline.json")
	rc := rung2Context(t, ws, "python", rel, nil)
	out := runRung2(t, rc)
	if out.Result.State != "error" {
		t.Errorf("state = %q, want error (baseline declared for a parser-less stack)", out.Result.State)
	}
}

func TestNoNewFailures_NoBaseline_CleanPasses(t *testing.T) {
	ws := goTestWorkspace(t, "func TestX(t *testing.T) {}\n")
	out := runRung2(t, rung2Context(t, ws, "go", "", nil))
	if out.Result.State != "pass" {
		t.Errorf("state = %q, want pass (no failures, no baseline)", out.Result.State)
	}
}

func TestNoNewFailures_NoBaseline_FailingTestReds(t *testing.T) {
	ws := goTestWorkspace(t, "func TestX(t *testing.T) { t.Fatal(\"boom\") }\n")
	out := runRung2(t, rung2Context(t, ws, "go", "", nil))
	if out.Result.State != "fail" {
		t.Fatalf("state = %q, want fail (a failing test with no baseline is a new failure)", out.Result.State)
	}
	if f := findingOfClass(out.Findings, newFailureClass); f == nil {
		t.Errorf("no %q finding; got %v", newFailureClass, out.Findings)
	} else if f.Message == "" {
		t.Error("new-failure finding has no message identifying the failing test")
	}
}

// A failure already in the baseline is not new — the run passes, and nothing
// shrank.
func TestNoNewFailures_Baseline_KnownFailureAccepted(t *testing.T) {
	ws := goTestWorkspace(t, "func TestX(t *testing.T) { t.Fatal(\"boom\") }\n")
	rel := writeBaseline(t, ws, "baseline.json", "rung2fixture::TestX")
	out := runRung2(t, rung2Context(t, ws, "go", rel, nil))
	if out.Result.State != "pass" {
		t.Errorf("state = %q, want pass (the only failure is in the baseline)", out.Result.State)
	}
	if out.Shrinkage != 0 {
		t.Errorf("shrinkage = %d, want 0 (the baseline failure still reproduces)", out.Shrinkage)
	}
}

func TestNoNewFailures_Baseline_NewFailureReds(t *testing.T) {
	ws := goTestWorkspace(t, "func TestX(t *testing.T) { t.Fatal(\"x\") }\n\nfunc TestY(t *testing.T) { t.Fatal(\"y\") }\n")
	rel := writeBaseline(t, ws, "baseline.json", "rung2fixture::TestX") // only X is known
	out := runRung2(t, rung2Context(t, ws, "go", rel, nil))
	if out.Result.State != "fail" {
		t.Fatalf("state = %q, want fail (TestY is a new failure)", out.Result.State)
	}
	if len(out.Findings) != 1 || out.Findings[0].Message == "" {
		t.Fatalf("findings = %v, want exactly one for the new failure TestY", out.Findings)
	}
	// The known failure TestX must NOT be reported — only the new one.
	for _, f := range out.Findings {
		if strings.Contains(f.Message, "TestX") {
			t.Errorf("finding %q reports the baselined failure TestX, want only new failures", f.Message)
		}
	}
}

// A baseline entry the run no longer reproduces is recorded as shrinkage — the
// ratchet's only permitted movement — even though the run passes.
func TestNoNewFailures_Baseline_ShrinkRecorded(t *testing.T) {
	ws := goTestWorkspace(t, "func TestX(t *testing.T) {}\n") // passes: reproduces nothing
	rel := writeBaseline(t, ws, "baseline.json", "rung2fixture::TestOld")
	out := runRung2(t, rung2Context(t, ws, "go", rel, nil))
	if out.Result.State != "pass" {
		t.Errorf("state = %q, want pass (no new failures)", out.Result.State)
	}
	if out.Shrinkage != 1 {
		t.Errorf("shrinkage = %d, want 1 (TestOld no longer fails)", out.Shrinkage)
	}
}

// garbageTestVerb outputs non-JSON and exits nonzero — the runner completes,
// but the go parser cannot read per-test identities from it.
func garbageTestVerb() *schema.Verb {
	return &schema.Verb{
		Command: "sh -c 'printf garbage; exit 1'",
		Env:     map[string]schema.EnvEntry{"PATH": {Mode: schema.EnvPass}, "HOME": {Mode: schema.EnvPass}},
	}
}

// Garbage output with a baseline bound is a rung ERROR: an unparseable run can
// never be silently read as zero failures against a ratchet (UB-9).
func TestNoNewFailures_GarbageOutput_BaselineErrors(t *testing.T) {
	ws := goTestWorkspace(t, "func TestX(t *testing.T) {}\n")
	rel := writeBaseline(t, ws, "baseline.json", "rung2fixture::TestX")
	out := runRung2(t, rung2Context(t, ws, "go", rel, garbageTestVerb()))
	if out.Result.State != "error" {
		t.Errorf("state = %q, want error (unparseable output, baseline bound)", out.Result.State)
	}
}

// The same garbage output with NO baseline falls back to the verb's exit code.
func TestNoNewFailures_GarbageOutput_NoBaselineExitCodePath(t *testing.T) {
	ws := goTestWorkspace(t, "func TestX(t *testing.T) {}\n")
	out := runRung2(t, rung2Context(t, ws, "go", "", garbageTestVerb()))
	if out.Result.State != "fail" {
		t.Errorf("state = %q, want fail (nonzero exit, unparseable, no baseline → exit-code path)", out.Result.State)
	}
}

// --- small test-local helpers --------------------------------------------

func findingOfClass(findings []schema.Finding, class string) *schema.Finding {
	for i := range findings {
		if findings[i].Class == class {
			return &findings[i]
		}
	}
	return nil
}
