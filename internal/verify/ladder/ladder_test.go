package ladder

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/verify/checks"
	"github.com/agenticlab-ai/pyxis/internal/verify/schema"
)

// fixtureGoDir is the checked-in Go-stack fixture (internal/verify/testdata):
// gofmt-clean, vet-clean, builds cleanly, and deliberately declares no verbs
// of its own — so ecosystem defaults resolve format-check/lint/build/test and
// exec-probe stays unbound (the probe-less scenario, #TestRun_ProbeLessFixture_GreenDegraded).
const fixtureGoDir = "../testdata/fixture-go"

// copyFixture copies fixtureGoDir into a fresh temp dir so each test mutates
// its own isolated copy and the checked-in fixture never changes.
func copyFixture(t *testing.T) string {
	t.Helper()
	dst := t.TempDir()
	if err := os.CopyFS(dst, os.DirFS(fixtureGoDir)); err != nil {
		t.Fatalf("copy fixture: %v", err)
	}
	return dst
}

func loadManifest(t *testing.T, workspace string) *schema.Manifest {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(workspace, ".pyxis", "codegate.json"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	m, err := schema.UnmarshalManifest(data)
	if err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	return m
}

// testBindings is a plausible six-field binding set. Run() only carries these
// through to the verdict — it never resolves or validates them (that is
// invoke's job, T2) — so a fixed literal is sufficient for every ladder test.
func testBindings() schema.Bindings {
	return schema.Bindings{
		HeadSHA:         "0123456789abcdef0123456789abcdef01234567",
		DiffBase:        "fedcba9876543210fedcba9876543210fedcba98",
		ManifestHash:    "manifest-hash",
		BaselineHash:    "baseline-hash",
		CodegateVersion: "1.0.0-test",
		GitVersion:      "2.40.0",
	}
}

// gitFixture copies the fixture and makes it a real git repo with a single
// commit, returning bindings whose DiffBase == HeadSHA — a valid, empty
// proposal diff. The rung-0 diff-based checks (secret/forbidden/machinery) and
// test-delta run diffset/git extraction against these bindings, so they need a
// resolvable base/head rather than the placeholder SHAs testBindings carries.
func gitFixture(t *testing.T) (string, schema.Bindings) {
	t.Helper()
	ws := copyFixture(t)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_AUTHOR_NAME", "codegate-test")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "codegate-test")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@example.com")
	gitCmd(t, ws, "init", "-q", "-b", "main")
	gitCmd(t, ws, "add", "-A")
	gitCmd(t, ws, "commit", "-q", "-m", "fixture base")
	sha := gitCmd(t, ws, "rev-parse", "HEAD")
	b := testBindings()
	b.DiffBase, b.HeadSHA = sha, sha
	return ws, b
}

func gitCmd(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// requireNotReached fails the test unless rung name is present and recorded
// skip:not-reached with zero check executions — the shape every rung past an
// early stop must have (DINV-4).
func requireNotReached(t *testing.T, v *schema.Verdict, name string) {
	t.Helper()
	r, ok := v.Rungs[name]
	if !ok {
		t.Fatalf("rung %s missing from verdict", name)
	}
	if !r.Skipped || r.SkipReason != "not-reached" {
		t.Errorf("rung %s = %+v, want skip:not-reached", name, r)
	}
	if len(r.Checks) != 0 {
		t.Errorf("rung %s recorded %d check execution(s), want zero (never reached)", name, len(r.Checks))
	}
}

// TestRun_ProbeLessFixture_GreenDegraded is T3's AC1 second half: the
// checked-in fixture binds no exec-probe verb, so rung 4's check records
// skip:verb-absent, the rung aggregates pass-with-skips, and the overall
// verdict is green-degraded — never a silent pass, never a red.
func TestRun_ProbeLessFixture_GreenDegraded(t *testing.T) {
	ws, bindings := gitFixture(t)
	manifest := loadManifest(t, ws)

	v := Run(context.Background(), ws, manifest, bindings, "full")

	if v.State != "green-degraded" {
		t.Fatalf("state = %q, want green-degraded (verdict: %+v)", v.State, v)
	}
	for _, name := range []string{"0", "1"} {
		r, ok := v.Rungs[name]
		if !ok || r.State != "pass" {
			t.Errorf("rung %s = %+v, want a clean pass", name, r)
		}
	}
	rung4, ok := v.Rungs["4"]
	if !ok {
		t.Fatal("rung 4 missing from verdict")
	}
	probe, ok := rung4.Checks["exec-probe"]
	if !ok {
		t.Fatal("exec-probe check missing from rung 4")
	}
	if probe.State != "skip" || probe.SkipReason != "verb-absent" {
		t.Errorf("exec-probe check = %+v, want skip:verb-absent", probe)
	}
	if rung4.State != "pass" {
		t.Errorf("rung 4 state = %q, want pass (a skip never raises a rung above pass)", rung4.State)
	}
}

func TestRun_UnknownStack_FailsClosed(t *testing.T) {
	// An unknown/unsupported stack binds no verbs, so every check skips. A run
	// that executed zero checks verified nothing and must fail closed (error),
	// never certify green-degraded and exit 0 (INV-6).
	manifest := &schema.Manifest{Version: "1.0.0", Stack: "python"}
	v := Run(context.Background(), t.TempDir(), manifest, testBindings(), "full")
	if v.State != "error" {
		t.Fatalf("state = %q, want error (a run that executed zero checks must fail closed): %+v", v.State, v)
	}
}

func TestSafeRun_ContainsPanic(t *testing.T) {
	// A check that panics must be contained as an error-class result, never
	// crash the process past the verdict writer.
	panicking := func(_ context.Context, _ checks.RunContext) checks.Outcome { panic("boom") }
	out := safeRun(context.Background(), panicking, checks.RunContext{}, "demo")
	if out.Result.State != "error" {
		t.Fatalf("panic outcome state = %q, want error", out.Result.State)
	}
}

func TestKnownState(t *testing.T) {
	for _, s := range []string{"pass", "skip", "fail", "error"} {
		if !knownState(s) {
			t.Errorf("knownState(%q) = false, want true", s)
		}
	}
	for _, s := range []string{"", "warn", "failed", "green"} {
		if knownState(s) {
			t.Errorf("knownState(%q) = true, want false — an unknown state must fail closed", s)
		}
	}
}

// TestRun_LintFailure_StopsLadderWithFileLine is T3's AC1 first half: a
// rung-0 lint failure produces a finding with file:line, and first-failure
// stop means rung 1 (build) records zero check executions.
func TestRun_LintFailure_StopsLadderWithFileLine(t *testing.T) {
	ws, bindings := gitFixture(t)
	injectVetFailure(t, ws)
	manifest := loadManifest(t, ws)

	v := Run(context.Background(), ws, manifest, bindings, "full")

	if v.State != "red" {
		t.Fatalf("state = %q, want red (verdict: %+v)", v.State, v)
	}
	rung0, ok := v.Rungs["0"]
	if !ok || rung0.State != "fail" {
		t.Fatalf("rung 0 = %+v, want fail", rung0)
	}

	var lintFinding *schema.Finding
	for i := range v.Findings {
		if v.Findings[i].CheckID == "lint" {
			lintFinding = &v.Findings[i]
			break
		}
	}
	if lintFinding == nil {
		t.Fatalf("no lint finding recorded among %d finding(s)", len(v.Findings))
	}
	if lintFinding.Path == "" || lintFinding.Line == 0 {
		t.Errorf("lint finding = %+v, want a non-empty file:line location", lintFinding)
	}

	requireNotReached(t, v, "1")
	requireNotReached(t, v, "4")
}

// TestRun_MissingVerbBinary_RungError is T3's AC3: a verb that cannot be
// spawned classifies its check as error/infra — never red, never green — and
// still stops the ladder before later rungs.
func TestRun_MissingVerbBinary_RungError(t *testing.T) {
	ws, bindings := gitFixture(t)
	manifest := loadManifest(t, ws)
	manifest.Verbs = map[string]*schema.Verb{
		"lint": {Command: "/nonexistent/definitely-not-a-real-binary-xyz123"},
	}

	v := Run(context.Background(), ws, manifest, bindings, "full")

	if v.State != "error" {
		t.Fatalf("state = %q, want error (verdict: %+v)", v.State, v)
	}
	rung0, ok := v.Rungs["0"]
	if !ok || rung0.State != "error" {
		t.Fatalf("rung 0 = %+v, want error", rung0)
	}
	lint, ok := rung0.Checks["lint"]
	if !ok {
		t.Fatal("lint check missing from rung 0")
	}
	if lint.State != "error" || lint.Class != "infra" {
		t.Errorf("lint check = %+v, want state=error class=infra", lint)
	}

	requireNotReached(t, v, "1")
}

// TestRun_AllVerbsBound_Green proves the fixture also drives a plain (never
// degraded) green: binding exec-probe removes the only skip source, so every
// rung passes clean.
func TestRun_AllVerbsBound_Green(t *testing.T) {
	ws, bindings := gitFixture(t)
	manifest := loadManifest(t, ws)
	manifest.Verbs = map[string]*schema.Verb{
		"exec-probe": {
			Command: "go run ./probe",
			Env: map[string]schema.EnvEntry{
				"PATH": {Mode: schema.EnvPass},
				"HOME": {Mode: schema.EnvPass},
			},
		},
	}

	v := Run(context.Background(), ws, manifest, bindings, "full")

	if v.State != "green" {
		t.Fatalf("state = %q, want green (verdict: %+v)", v.State, v)
	}
	for _, name := range []string{"0", "1", "4"} {
		r, ok := v.Rungs[name]
		if !ok || r.State != "pass" {
			t.Errorf("rung %s = %+v, want a clean pass", name, r)
		}
	}
}

// TestRun_VerdictBindingsCarryThrough proves Run() carries the caller's
// bindings and mode into the verdict verbatim — it resolves nothing itself
// (T2's job) — and that DINV-1 bindings-presence holds on a real run.
func TestRun_VerdictBindingsCarryThrough(t *testing.T) {
	ws, bindings := gitFixture(t)
	manifest := loadManifest(t, ws)

	v := Run(context.Background(), ws, manifest, bindings, "incremental")

	if v.Bindings != bindings {
		t.Errorf("Bindings = %+v, want the bindings passed in unchanged: %+v", v.Bindings, bindings)
	}
	if v.Mode != "incremental" {
		t.Errorf("Mode = %q, want %q", v.Mode, "incremental")
	}
	if err := v.Bindings.BindingsPresent(); err != nil {
		t.Errorf("BindingsPresent: %v", err)
	}
}

// injectVetFailure appends a function to the fixture's main.go that `go vet`
// flags via its printf analyzer — gofmt-clean (so format-check still passes;
// only lint reds) and reported as "path/to/main.go:LINE:COL: message", the
// file:line shape parseDiagnostics extracts a Finding from.
func injectVetFailure(t *testing.T, workspace string) {
	t.Helper()
	path := filepath.Join(workspace, "main.go")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	broken := string(data) + "\nfunc badVet() {\n\tfmt.Printf(\"%d\\n\", \"not-a-number\")\n}\n"
	if err := os.WriteFile(path, []byte(broken), 0o644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}
}
