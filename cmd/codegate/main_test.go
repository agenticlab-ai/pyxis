package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/verify/invoke"
	"github.com/agenticlab-ai/pyxis/internal/verify/schema"
)

// fixtureGoDir is internal/verify/testdata's checked-in Go-stack fixture,
// reached from cmd/codegate.
const fixtureGoDir = "../../internal/verify/testdata/fixture-go"

// gitFixtureWorkspace copies the fixture-go tree into a fresh temp dir,
// applies mutate to the WORKING TREE before the commit (so the committed HEAD
// already carries any injected defect and the tree is clean at invocation
// time — the invocation contract's precondition, §3.6), then commits it as a
// one-commit git repo and returns its path.
func gitFixtureWorkspace(t *testing.T, mutate func(dir string)) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.CopyFS(dir, os.DirFS(fixtureGoDir)); err != nil {
		t.Fatalf("copy fixture: %v", err)
	}
	if mutate != nil {
		mutate(dir)
	}

	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_AUTHOR_NAME", "codegate-test")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "codegate-test")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@example.com")

	gitOK(t, dir, "init", "-q", "-b", "main")
	gitOK(t, dir, "add", "-A")
	gitOK(t, dir, "commit", "-q", "-m", "fixture commit")

	return dir
}

func gitOK(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func readVerdict(t *testing.T, workspace string) *schema.Verdict {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(workspace, ".codegate", "verdict.json"))
	if err != nil {
		t.Fatalf("read verdict artifact: %v", err)
	}
	var v schema.Verdict
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatalf("unmarshal verdict artifact: %v", err)
	}
	return &v
}

// injectVetFailure appends a gofmt-clean function that `go vet` flags via its
// printf analyzer, so only the lint check reds (format-check still passes).
func injectVetFailure(t *testing.T, dir string) {
	t.Helper()
	path := filepath.Join(dir, "main.go")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	broken := string(data) + "\nfunc badVet() {\n\tfmt.Printf(\"%d\\n\", \"not-a-number\")\n}\n"
	if err := os.WriteFile(path, []byte(broken), 0o644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}
}

// TestRunCmd_EndToEnd_GreenDegraded is T3's Goal statement made literal: the
// ladder runs end to end on the fixture repo through the real `codegate run`
// entry point, exits 0 (pass-class), and leaves a verdict artifact behind.
func TestRunCmd_EndToEnd_GreenDegraded(t *testing.T) {
	ws := gitFixtureWorkspace(t, nil)

	code := runCmd([]string{"--workspace", ws, "--diff-base", "HEAD"})

	if code != invoke.ExitPass {
		t.Fatalf("exit code = %d, want %d (pass-class)", code, invoke.ExitPass)
	}
	v := readVerdict(t, ws)
	if v.State != "green-degraded" {
		t.Errorf("verdict state = %q, want green-degraded", v.State)
	}
}

// TestRunCmd_EndToEnd_Red proves a genuine check failure exits 1 with a red
// verdict artifact on disk — never a false pass.
func TestRunCmd_EndToEnd_Red(t *testing.T) {
	ws := gitFixtureWorkspace(t, func(dir string) { injectVetFailure(t, dir) })

	code := runCmd([]string{"--workspace", ws, "--diff-base", "HEAD"})

	if code != invoke.ExitRed {
		t.Fatalf("exit code = %d, want %d (red)", code, invoke.ExitRed)
	}
	v := readVerdict(t, ws)
	if v.State != "red" {
		t.Errorf("verdict state = %q, want red", v.State)
	}
}

// TestRunCmd_EndToEnd_RungError proves a missing verb binary exits 3 (never
// 0, never 1) with an error-class verdict artifact on disk — T3's AC3 at the
// real process boundary.
func TestRunCmd_EndToEnd_RungError(t *testing.T) {
	ws := gitFixtureWorkspace(t, func(dir string) {
		manifest := `{"version":"1.0.0","stack":"go","verbs":{"lint":{"command":"/nonexistent/definitely-not-a-real-binary-xyz123"}}}`
		manifestPath := filepath.Join(dir, ".pyxis", "codegate.json")
		if err := os.WriteFile(manifestPath, []byte(manifest), 0o644); err != nil {
			t.Fatalf("write manifest: %v", err)
		}
	})

	code := runCmd([]string{"--workspace", ws, "--diff-base", "HEAD"})

	if code != invoke.ExitRungErr {
		t.Fatalf("exit code = %d, want %d (rung error)", code, invoke.ExitRungErr)
	}
	v := readVerdict(t, ws)
	if v.State != "error" {
		t.Errorf("verdict state = %q, want error", v.State)
	}
}

func TestExitCodeForState(t *testing.T) {
	tests := []struct {
		state string
		want  int
	}{
		{"green", invoke.ExitPass},
		{"green-degraded", invoke.ExitPass},
		{"red", invoke.ExitRed},
		{"error", invoke.ExitRungErr},
		{"unknown-future-state", invoke.ExitRungErr},
	}
	for _, tt := range tests {
		if got := exitCodeForState(tt.state); got != tt.want {
			t.Errorf("exitCodeForState(%q) = %d, want %d", tt.state, got, tt.want)
		}
	}
}

// seedWorkspace builds a minimal seedable workspace: a marker file plus a
// .git directory (ValidateWorkspacePath checks presence only, so no real
// repository is needed at this layer).
func seedWorkspace(t *testing.T, markers ...string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, m := range markers {
		if err := os.WriteFile(filepath.Join(dir, m), []byte("marker\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// The seed dispatch end to end: a first run writes the manifest, a repeat
// refuses (UB-10) without touching it.
func TestSeedCmdSeedsThenRefusesARepeat(t *testing.T) {
	dir := seedWorkspace(t, "go.mod")

	if code := seedCmd([]string{"--workspace", dir}); code != invoke.ExitPass {
		t.Fatalf("first seed exit = %d, want %d", code, invoke.ExitPass)
	}
	data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(invoke.ManifestRelPath)))
	if err != nil {
		t.Fatalf("manifest not written: %v", err)
	}
	if _, err := schema.UnmarshalManifest(data); err != nil {
		t.Fatalf("seeded manifest does not closed-parse: %v", err)
	}

	if code := seedCmd([]string{"--workspace", dir}); code != invoke.ExitConfigErr {
		t.Fatalf("repeat seed exit = %d, want the UB-10 refusal %d", code, invoke.ExitConfigErr)
	}
}

func TestSeedCmdWritesPlaceholdersForAStackWithoutDefaults(t *testing.T) {
	dir := seedWorkspace(t, "requirements.txt")
	if code := seedCmd([]string{"--workspace", dir}); code != invoke.ExitPass {
		t.Fatalf("python seed exit = %d", code)
	}
}

// The closed flag surface, matching the sibling verbs' discipline: unknown
// flags, a valueless/empty --workspace, and a repeated --workspace all
// refuse rather than silently resolving.
func TestSeedCmdRefusesMalformedArguments(t *testing.T) {
	cases := [][]string{
		{"--stack", "go"},
		{"--workspace"},
		{"--workspace", ""},
		{"--workspace", "a", "--workspace", "b"},
	}
	for _, args := range cases {
		if code := seedCmd(args); code != invoke.ExitConfigErr {
			t.Errorf("seedCmd(%q) exit = %d, want %d", args, code, invoke.ExitConfigErr)
		}
	}
}

// A workspace that is not a git working tree refuses before any detection.
func TestSeedCmdRefusesANonRepository(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := seedCmd([]string{"--workspace", dir}); code != invoke.ExitConfigErr {
		t.Fatalf("exit = %d, want %d", code, invoke.ExitConfigErr)
	}
}

func TestCaptureBaselineRunsTheBoundTestAndWritesTheDeclaredArtifact(t *testing.T) {
	ws := gitFixtureWorkspace(t, func(dir string) {
		manifest := `{"version":"1.0.0","stack":"go","verbs":{"test":{"command":"go test -json ./..."}},"baseline_path":".codegate/baseline.json"}`
		if err := os.WriteFile(filepath.Join(dir, ".pyxis", "codegate.json"), []byte(manifest), 0o644); err != nil {
			t.Fatal(err)
		}
	})
	if code := captureBaselineCmd([]string{"--workspace", ws}); code != invoke.ExitPass {
		t.Fatalf("capture exit=%d", code)
	}
	if _, err := os.Stat(filepath.Join(ws, ".codegate", "baseline.json")); err != nil {
		t.Fatalf("baseline was not written: %v", err)
	}
}

func TestCaptureBaselineRefusesClosedArgumentsAndMissingContract(t *testing.T) {
	for _, args := range [][]string{
		nil,
		{"--workspace"},
		{"--workspace", "one", "--workspace", "two"},
		{"--unknown"},
		{"--workspace", t.TempDir()},
	} {
		if code := captureBaselineCmd(args); code != invoke.ExitConfigErr {
			t.Fatalf("captureBaselineCmd(%v)=%d", args, code)
		}
	}
	ws := gitFixtureWorkspace(t, nil)
	if code := captureBaselineCmd([]string{"--workspace", ws}); code != invoke.ExitConfigErr {
		t.Fatalf("capture without baseline path=%d", code)
	}
}

func TestRunCmdIncrementalFallbackAndSummaryBoundaries(t *testing.T) {
	ws := gitFixtureWorkspace(t, nil)
	if code := runCmd([]string{"--workspace", ws, "--diff-base", "HEAD", "--incremental"}); code != invoke.ExitPass {
		t.Fatalf("incremental fallback exit=%d", code)
	}
	for _, args := range [][]string{nil, {"--workspace"}, {"--workspace", t.TempDir(), "--diff-base", "HEAD"}} {
		if code := runCmd(args); code != invoke.ExitConfigErr {
			t.Fatalf("runCmd(%v)=%d", args, code)
		}
	}

	findings := make([]schema.Finding, maxSummaryFindings+2)
	for i := range findings {
		findings[i] = schema.Finding{Rung: "2", CheckID: "test", Path: "x.go", Line: i + 1, Message: "failure"}
	}
	printSummary(&schema.Verdict{State: "red", Workspace: ws, Findings: findings})
	printSummary(&schema.Verdict{State: "green", Workspace: ws})

	blocked := t.TempDir()
	if err := os.WriteFile(filepath.Join(blocked, ".codegate"), []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeVerdict(blocked, &schema.Verdict{}); err == nil {
		t.Fatal("writeVerdict accepted a blocked scratch path")
	}
	usage()
}

func TestRunCmdNamesLockBindingAndManifestFailures(t *testing.T) {
	t.Run("held workspace lock", func(t *testing.T) {
		ws := gitFixtureWorkspace(t, nil)
		lock, cerr := invoke.AcquireWorkspaceLock(ws)
		if cerr != nil {
			t.Fatalf("acquire fixture lock: %v", cerr)
		}
		defer lock.Release()
		if code := runCmd([]string{"--workspace", ws, "--diff-base", "HEAD"}); code != invoke.ExitRungErr {
			t.Fatalf("locked run exit = %d", code)
		}
	})

	t.Run("unresolvable diff base", func(t *testing.T) {
		ws := gitFixtureWorkspace(t, nil)
		if code := runCmd([]string{"--workspace", ws, "--diff-base", "refs/heads/does-not-exist"}); code != invoke.ExitConfigErr {
			t.Fatalf("bad diff-base exit = %d", code)
		}
	})

	t.Run("missing manifest", func(t *testing.T) {
		ws := gitFixtureWorkspace(t, func(dir string) {
			if err := os.Remove(filepath.Join(dir, filepath.FromSlash(invoke.ManifestRelPath))); err != nil {
				t.Fatal(err)
			}
		})
		if code := runCmd([]string{"--workspace", ws, "--diff-base", "HEAD"}); code != invoke.ExitConfigErr {
			t.Fatalf("missing manifest exit = %d", code)
		}
	})
}

func TestCaptureBaselineNamesExecutionAndArtifactFailures(t *testing.T) {
	writeManifest := func(t *testing.T, ws, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(ws, filepath.FromSlash(invoke.ManifestRelPath)), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("manifest cannot load", func(t *testing.T) {
		ws := gitFixtureWorkspace(t, nil)
		if err := os.Remove(filepath.Join(ws, filepath.FromSlash(invoke.ManifestRelPath))); err != nil {
			t.Fatal(err)
		}
		if code := captureBaselineCmd([]string{"--workspace", ws}); code != invoke.ExitConfigErr {
			t.Fatalf("missing manifest exit = %d", code)
		}
	})

	t.Run("stack has no identity parser", func(t *testing.T) {
		ws := gitFixtureWorkspace(t, nil)
		writeManifest(t, ws, `{"version":"1.0.0","stack":"python","verbs":{"test":{"command":"/bin/echo ok"}},"baseline_path":".codegate/baseline.json"}`)
		if code := captureBaselineCmd([]string{"--workspace", ws}); code != invoke.ExitConfigErr {
			t.Fatalf("unparseable stack exit = %d", code)
		}
	})

	t.Run("test command cannot start", func(t *testing.T) {
		ws := gitFixtureWorkspace(t, nil)
		writeManifest(t, ws, `{"version":"1.0.0","stack":"go","verbs":{"test":{"command":"/definitely/missing/codegate-test"}},"baseline_path":".codegate/baseline.json"}`)
		if code := captureBaselineCmd([]string{"--workspace", ws}); code != invoke.ExitRungErr {
			t.Fatalf("unrunnable test exit = %d", code)
		}
	})

	t.Run("test output cannot parse", func(t *testing.T) {
		ws := gitFixtureWorkspace(t, nil)
		writeManifest(t, ws, `{"version":"1.0.0","stack":"go","verbs":{"test":{"command":"/bin/echo not-json"}},"baseline_path":".codegate/baseline.json"}`)
		if code := captureBaselineCmd([]string{"--workspace", ws}); code != invoke.ExitRungErr {
			t.Fatalf("unparseable output exit = %d", code)
		}
	})

	t.Run("baseline destination is blocked", func(t *testing.T) {
		ws := gitFixtureWorkspace(t, nil)
		writeManifest(t, ws, `{"version":"1.0.0","stack":"go","verbs":{"test":{"command":"/bin/echo '{\"Action\":\"pass\",\"Package\":\"example.test\"}'"}},"baseline_path":"blocked/baseline.json"}`)
		if err := os.WriteFile(filepath.Join(ws, "blocked"), []byte("not a directory"), 0o644); err != nil {
			t.Fatal(err)
		}
		if code := captureBaselineCmd([]string{"--workspace", ws}); code != invoke.ExitRungErr {
			t.Fatalf("blocked baseline exit = %d", code)
		}
	})
}
