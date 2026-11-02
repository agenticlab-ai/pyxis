package matrix

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/verify/invoke"
	"github.com/agenticlab-ai/pyxis/internal/verify/ladder"
	"github.com/agenticlab-ai/pyxis/internal/verify/schema"
)

// A scenario is one proposal a fixture is driven through. baseSetup, if set,
// mutates the fixture BEFORE the base commit (a file present in both base and
// head — e.g. a pre-existing test a silenced-test seed later edits); seed, if
// set, is the proposal itself (the base→head diff). Either may be nil.
type scenario struct {
	name      string
	baseSetup func(t *testing.T, dir string)
	seed      func(t *testing.T, dir string)
	// For a legitimate scenario: wantState is the expected verdict and wantClass
	// is empty. For a catch scenario: wantClass (and wantRung) name the finding
	// the harness must emit, and family/seedCase label its EVT-4 record.
	wantState string
	wantClass string
	wantRung  string
	family    string
	seedCase  string
}

func (s scenario) isCatch() bool { return s.wantClass != "" }

// stackFixture is one stack's fixture repo plus every scenario the matrix and
// catch suite drive it through. tool is the executable a scenario's verbs need
// (skip the whole stack when it is absent, so a bare machine never false-fails);
// the harness itself is stack-blind — every stack difference is data here.
type stackFixture struct {
	stack     string
	dir       string // fixture repo, relative to this package
	tool      string // exec.LookPath guard (go / python3 / node)
	scenarios []scenario
}

// requireTool skips the current test when the stack's toolchain is absent.
// Every CI runner the catch-suite workflow targets installs all three; this
// only ever fires on a developer machine missing one.
func requireTool(t *testing.T, name string) {
	t.Helper()
	if _, err := exec.LookPath(name); err != nil {
		t.Skipf("%s not on PATH — skipping this stack's fixtures (present in CI)", name)
	}
}

// buildWorkspace copies fixtureDir into a throwaway git repo, applies baseSetup
// then commits the base, applies seed then commits the head, and returns the
// workspace with bindings pinned to those two real commits. A nil seed leaves
// head == base (an empty proposal diff — the clean run).
func buildWorkspace(t *testing.T, fixtureDir string, baseSetup, seed func(t *testing.T, dir string)) (string, schema.Bindings) {
	t.Helper()
	ws := t.TempDir()
	if err := os.CopyFS(ws, os.DirFS(fixtureDir)); err != nil {
		t.Fatalf("copy fixture %s: %v", fixtureDir, err)
	}
	fixExecBits(t, ws)
	if baseSetup != nil {
		baseSetup(t, ws)
	}

	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_AUTHOR_NAME", "codegate-matrix")
	t.Setenv("GIT_AUTHOR_EMAIL", "matrix@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "codegate-matrix")
	t.Setenv("GIT_COMMITTER_EMAIL", "matrix@example.com")

	git(t, ws, "init", "-q", "-b", "main")
	git(t, ws, "add", "-A")
	git(t, ws, "commit", "-q", "-m", "base")
	base := gitOut(t, ws, "rev-parse", "HEAD")

	head := base
	if seed != nil {
		seed(t, ws)
		git(t, ws, "add", "-A")
		git(t, ws, "commit", "-q", "-m", "head")
		head = gitOut(t, ws, "rev-parse", "HEAD")
	}

	return ws, schema.Bindings{
		DiffBase:        base,
		HeadSHA:         head,
		ManifestHash:    "matrix-manifest",
		BaselineHash:    "matrix-baseline",
		CodegateVersion: "matrix-test",
		GitVersion:      "2.50.1",
	}
}

// fixExecBits restores the executable bit on the scripted verbs (os.CopyFS is
// not guaranteed to preserve mode across every platform/version).
func fixExecBits(t *testing.T, ws string) {
	t.Helper()
	verbs := filepath.Join(ws, "codegate-verbs")
	entries, err := os.ReadDir(verbs)
	if err != nil {
		return // a fixture with no scripted verbs (go) is fine
	}
	for _, e := range entries {
		if err := os.Chmod(filepath.Join(verbs, e.Name()), 0o755); err != nil {
			t.Fatalf("chmod verb %s: %v", e.Name(), err)
		}
	}
}

func loadManifest(t *testing.T, ws string) *schema.Manifest {
	t.Helper()
	m, _, cerr := invoke.LoadManifest(ws)
	if cerr != nil {
		t.Fatalf("load manifest: %v", cerr)
	}
	return m
}

func runLadder(t *testing.T, ws string, m *schema.Manifest, b schema.Bindings) *schema.Verdict {
	t.Helper()
	return ladder.Run(context.Background(), ws, m, b, "full")
}

// assertTrackedTreeClean is the DINV-7 guard (design A-3): after any run —
// including one whose rung-3 two-leg ran — the workspace's own TRACKED tree
// must be byte-identical, proving the falsification worktree never touched it.
// Untracked scratch (.codegate) is excluded via --untracked-files=no.
func assertTrackedTreeClean(t *testing.T, ws string) {
	t.Helper()
	if out := gitOut(t, ws, "status", "--porcelain", "--untracked-files=no"); out != "" {
		t.Errorf("workspace tracked tree was mutated by the run (DINV-7 breach):\n%s", out)
	}
}

func findingClasses(v *schema.Verdict) []string {
	classes := make([]string, 0, len(v.Findings))
	for _, f := range v.Findings {
		classes = append(classes, f.Class)
	}
	return classes
}

func findingWithClass(v *schema.Verdict, class string) *schema.Finding {
	for i := range v.Findings {
		if v.Findings[i].Class == class {
			return &v.Findings[i]
		}
	}
	return nil
}

// emitEVT4 writes the per-seed catch-suite record (design codegate-v1 §3.9,
// A-2) the permanent CI job surfaces: one JSON line per (stack, seed) naming
// whether the harness blocked it and the finding class it produced.
func emitEVT4(stack string, s scenario, blocked bool, gotClasses []string) {
	slog.New(slog.NewJSONHandler(os.Stdout, nil)).Info("EVT-4",
		"event", "codegate.catch_suite",
		"stack", stack,
		"seed_family", s.family,
		"seed_case", s.seedCase,
		"expected_class", s.wantClass,
		"blocked", blocked,
		"got_classes", gotClasses,
	)
}

// --- git + file mutation helpers -----------------------------------------

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	if out, err := runGit(dir, args...); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := runGit(dir, args...)
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return out
}

func runGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	return trimTrailingNewline(string(out)), err
}

func trimTrailingNewline(s string) string {
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}

func writeFixtureFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", rel, err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

func appendFixtureFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(rel))
	existing, err := os.ReadFile(full)
	if err != nil {
		t.Fatalf("read %s for append: %v", rel, err)
	}
	if err := os.WriteFile(full, append(existing, []byte(content)...), 0o644); err != nil {
		t.Fatalf("append %s: %v", rel, err)
	}
}

func removeFixtureFile(t *testing.T, dir, rel string) {
	t.Helper()
	if err := os.Remove(filepath.Join(dir, filepath.FromSlash(rel))); err != nil {
		t.Fatalf("remove %s: %v", rel, err)
	}
}
