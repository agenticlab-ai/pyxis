package sweep_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/sweep"
)

// The pass integration fixtures are hermetic temp repos that are BOTH a git repo
// (the pass pins HEAD + dirty through git) and a go module (the unwired detector
// runs the real toolchain), exercised through the production runner or a
// fault-injecting wrapper of it (maintenance-sweep-v1 §3.9). No test reads the
// wall clock — the machine face carries no clock field, so determinism is
// structural (DINV-2).

const passModule = "github.com/agenticlab-ai/pyxis"

// gitFixture materializes files under a temp dir, makes it a committed git repo,
// and returns its root — a clean tree (dirty=false) pinned at one HEAD.
func gitFixture(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, content := range files {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	gitCmd(t, root, "init", "-q")
	gitCmd(t, root, "config", "user.email", "sweep@test")
	gitCmd(t, root, "config", "user.name", "sweep")
	gitCmd(t, root, "add", "-A")
	gitCmd(t, root, "commit", "-q", "-m", "fixture")
	return root
}

func gitCmd(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_DATE=2026-01-01T00:00:00 +0000", "GIT_COMMITTER_DATE=2026-01-01T00:00:00 +0000")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

// cleanModule is a fixture every detector reads as clean: one main package (no
// unwired non-main packages), no plans (touches empty), and no living rollup
// (staleness skips). The pass over it is outcome=clean, exit 0.
func cleanModule() map[string]string {
	return map[string]string{
		"go.mod":           "module " + passModule + "\n\ngo 1.26\n",
		"cmd/tool/main.go": "package main\n\nfunc main() {}\n",
	}
}

// orphanModule adds one unwired non-main package to the clean module — one
// unwired-packages finding, so the pass is outcome=findings, exit 1.
func orphanModule() map[string]string {
	m := cleanModule()
	m["internal/orphan/orphan.go"] = "package orphan\n\nvar V int\n"
	return m
}

// faultRunner wraps the production runner and fails one git subcommand (matched
// by its first arg), so a single detector's toolchain read faults while the
// state pin (rev-parse/status) and the other detectors' reads succeed — the
// fault-injection seam for DINV-4.
type faultRunner struct {
	inner   sweep.Runner
	failGit string // the git subcommand (args[0]) to fail; "" fails none
}

func (f faultRunner) Run(ctx context.Context, dir, name string, args ...string) ([]string, error) {
	if name == "git" && len(args) > 0 && args[0] == f.failGit {
		return nil, fmt.Errorf("injected fault: git %s unavailable", f.failGit)
	}
	return f.inner.Run(ctx, dir, name, args...)
}

func runPass(t *testing.T, root string, runner sweep.Runner) sweep.Report {
	t.Helper()
	return sweep.Run(context.Background(), root, runner)
}

// findingSubjects returns every finding subject across all detectors, for parity
// and presence assertions.
func findingSubjects(r sweep.Report) []string {
	var out []string
	for _, d := range r.Detectors {
		for _, f := range d.Findings {
			out = append(out, f.Subject)
		}
	}
	return out
}

func detector(r sweep.Report, name string) (sweep.DetectorResult, bool) {
	for _, d := range r.Detectors {
		if d.Name == name {
			return d, true
		}
	}
	return sweep.DetectorResult{}, false
}

// A clean fixture: every detector ran, zero findings, zero faults → outcome
// clean, exit 0; the machine face runs all three detectors and the human face
// prints each detector's explicit "0 findings" line (acceptance: clean zero;
// prd NFR-3).
func TestPass_CleanIsZero(t *testing.T) {
	root := gitFixture(t, cleanModule())
	r := runPass(t, root, sweep.ExecRunner{})

	if r.Outcome != sweep.OutcomeClean || r.ExitCode() != 0 {
		t.Fatalf("clean fixture: outcome=%q exit=%d\n%s", r.Outcome, r.ExitCode(), r.HumanFace())
	}
	if len(r.Detectors) != 3 {
		t.Fatalf("expected all three detectors present, got %d", len(r.Detectors))
	}
	if len(findingSubjects(r)) != 0 {
		t.Fatalf("clean fixture reported findings: %v", findingSubjects(r))
	}
	human := r.HumanFace()
	for _, name := range []string{sweep.DetectorUnwiredPackages, sweep.DetectorTouchesUnderdelivery, sweep.DetectorRollupStaleness} {
		want := "detector " + name + ": 0 findings"
		if !strings.Contains(human, want) {
			t.Fatalf("human face missing explicit zero line %q:\n%s", want, human)
		}
	}
}

// A findings fixture: one unwired package → outcome findings, exit 1; both faces
// carry the same finding (face parity, acceptance: findings one; NFR-3).
func TestPass_FindingsIsOne(t *testing.T) {
	root := gitFixture(t, orphanModule())
	r := runPass(t, root, sweep.ExecRunner{})

	if r.Outcome != sweep.OutcomeFindings || r.ExitCode() != 1 {
		t.Fatalf("findings fixture: outcome=%q exit=%d\n%s", r.Outcome, r.ExitCode(), r.HumanFace())
	}
	subs := findingSubjects(r)
	if len(subs) != 1 || subs[0] != "internal/orphan" {
		t.Fatalf("expected one finding internal/orphan, got %v", subs)
	}
	if !strings.Contains(r.HumanFace(), "internal/orphan") {
		t.Fatalf("human face missing the finding:\n%s", r.HumanFace())
	}
}

// A fault-injected detector: staleness's git-log read fails while unwired still
// finds the orphan → the remaining detector's findings are present, the fault is
// named, and the pass exits 1 (findings win over the coexisting fault — DINV-4,
// acceptance: fault-injected leaves findings present exiting one).
func TestPass_FaultWithFindingsIsOne(t *testing.T) {
	root := gitFixture(t, orphanModule())
	r := runPass(t, root, faultRunner{inner: sweep.ExecRunner{}, failGit: "log"})

	if r.Outcome != sweep.OutcomeFindings || r.ExitCode() != 1 {
		t.Fatalf("fault+findings: outcome=%q exit=%d\n%s", r.Outcome, r.ExitCode(), r.HumanFace())
	}
	st, ok := detector(r, sweep.DetectorRollupStaleness)
	if !ok || st.Fault == "" {
		t.Fatalf("staleness fault not named: %+v", st)
	}
	if subs := findingSubjects(r); len(subs) != 1 || subs[0] != "internal/orphan" {
		t.Fatalf("the other detector's findings were lost: %v", subs)
	}
}

// The pinned variant: the same injected fault with zero findings (fully wired) →
// outcome fault, exit 2, the fault named in-band (DINV-4; acceptance: two with
// zero findings).
func TestPass_FaultWithoutFindingsIsTwo(t *testing.T) {
	root := gitFixture(t, cleanModule())
	r := runPass(t, root, faultRunner{inner: sweep.ExecRunner{}, failGit: "log"})

	if r.Outcome != sweep.OutcomeFault || r.ExitCode() != 2 {
		t.Fatalf("fault-only: outcome=%q exit=%d\n%s", r.Outcome, r.ExitCode(), r.HumanFace())
	}
	if len(findingSubjects(r)) != 0 {
		t.Fatalf("fault-only reported findings: %v", findingSubjects(r))
	}
	st, ok := detector(r, sweep.DetectorRollupStaleness)
	if !ok || st.Fault == "" {
		t.Fatalf("staleness fault not named: %+v", st)
	}
	// The remaining detectors still ran clean — the fault did not masquerade.
	if uw, _ := detector(r, sweep.DetectorUnwiredPackages); uw.Fault != "" {
		t.Fatalf("unwired should not have faulted: %q", uw.Fault)
	}
}

// An unreadable repository state (root is not a git repo) → a pass-level fault,
// exit 2, zero finding output, empty commit (DINV-3; acceptance: unreadable repo
// with zero finding output).
func TestPass_UnreadableRepoIsTwo(t *testing.T) {
	root := t.TempDir() // no git init
	r := runPass(t, root, sweep.ExecRunner{})

	if r.Outcome != sweep.OutcomeFault || r.ExitCode() != 2 {
		t.Fatalf("unreadable: outcome=%q exit=%d", r.Outcome, r.ExitCode())
	}
	if len(r.Faults) == 0 || !strings.Contains(r.Faults[0], "unreadable repository state") {
		t.Fatalf("unreadable-state fault not named: %v", r.Faults)
	}
	if len(r.Detectors) != 0 || len(findingSubjects(r)) != 0 {
		t.Fatalf("unreadable pass ran detectors or emitted findings: %+v", r.Detectors)
	}
	if r.EvaluatedState.Commit != "" {
		t.Fatalf("unreadable pass pinned a commit: %q", r.EvaluatedState.Commit)
	}
}

// A malformed policy (unknown key) → a pass-level fault, exit 2, zero finding
// output, no detector run (prd UB-2, DINV-3; acceptance: malformed policy).
func TestPass_MalformedPolicyIsTwo(t *testing.T) {
	files := cleanModule()
	files["docs/policy/maintenance-sweep.yaml"] = "schema_version: maintenance-policy/v1\nbogus_key: 3\n"
	root := gitFixture(t, files)
	r := runPass(t, root, sweep.ExecRunner{})

	if r.Outcome != sweep.OutcomeFault || r.ExitCode() != 2 {
		t.Fatalf("malformed policy: outcome=%q exit=%d\n%s", r.Outcome, r.ExitCode(), r.HumanFace())
	}
	if len(r.Detectors) != 0 || len(findingSubjects(r)) != 0 {
		t.Fatalf("malformed policy ran detectors: %+v", r.Detectors)
	}
	if len(r.Faults) == 0 {
		t.Fatalf("malformed policy produced no named fault")
	}
	// The commit is still pinned — the repo was readable, only the policy was not.
	if r.EvaluatedState.Commit == "" {
		t.Fatalf("malformed policy failed to pin the readable repo's commit")
	}
}

// A reasonless waiver → a pass-level fault, exit 2, zero finding output — every
// suppression is attributable or it refuses to load (prd FR-10, DINV-3/5).
func TestPass_ReasonlessWaiverIsTwo(t *testing.T) {
	files := cleanModule()
	files["docs/policy/maintenance-sweep.yaml"] = "schema_version: maintenance-policy/v1\nwaivers:\n  packages:\n    - package: internal/orphan\n"
	root := gitFixture(t, files)
	r := runPass(t, root, sweep.ExecRunner{})

	if r.Outcome != sweep.OutcomeFault || r.ExitCode() != 2 {
		t.Fatalf("reasonless waiver: outcome=%q exit=%d\n%s", r.Outcome, r.ExitCode(), r.HumanFace())
	}
	if len(findingSubjects(r)) != 0 || len(r.Detectors) != 0 {
		t.Fatalf("reasonless waiver ran detectors or emitted findings")
	}
	joined := strings.Join(r.Faults, " ")
	if !strings.Contains(joined, "reason") {
		t.Fatalf("reasonless-waiver fault does not name the missing reason: %v", r.Faults)
	}
}

// Two consecutive runs on a pinned fixture produce byte-identical machine faces,
// and both faces carry the same findings (acceptance: determinism + parity;
// DINV-2, NFR-3).
func TestPass_DeterministicAndParity(t *testing.T) {
	root := gitFixture(t, orphanModule())
	r1 := runPass(t, root, sweep.ExecRunner{})
	r2 := runPass(t, root, sweep.ExecRunner{})

	b1, err := r1.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	b2, err := r2.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	if string(b1) != string(b2) {
		t.Fatalf("machine faces not byte-identical:\n%s\n%s", b1, b2)
	}
	// Every machine-face finding is named on the human face — face parity.
	human := r1.HumanFace()
	for _, sub := range findingSubjects(r1) {
		if !strings.Contains(human, sub) {
			t.Fatalf("finding %q on the machine face is absent from the human face:\n%s", sub, human)
		}
	}
}

// A dirty working tree sets the machine-face dirty annotation and the human-face
// warning, and the commit still names the evaluated base (acceptance: dirty tree;
// §3.3).
func TestPass_DirtyTreeAnnotated(t *testing.T) {
	root := gitFixture(t, cleanModule())
	head := strings.TrimSpace(gitOut(t, root, "rev-parse", "HEAD"))

	// Introduce an uncommitted change — an untracked file makes the tree dirty.
	if err := os.WriteFile(filepath.Join(root, "scratch.txt"), []byte("wip"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := runPass(t, root, sweep.ExecRunner{})

	if !r.EvaluatedState.Dirty {
		t.Fatalf("dirty tree not annotated on the machine face")
	}
	if r.EvaluatedState.Commit != head {
		t.Fatalf("dirty pass pinned %q, want the committed base %q", r.EvaluatedState.Commit, head)
	}
	human := r.HumanFace()
	if !strings.Contains(human, "dirty") || !strings.Contains(human, head) {
		t.Fatalf("human face missing the dirty warning naming the base:\n%s", human)
	}
}

func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %s: %v", strings.Join(args, " "), err)
	}
	return string(out)
}

// ExitCode maps the outcome by the pinned ranking, and the report's own ExitCode
// mirrors it — the outcome field and the exit code are one ranking read twice.
func TestExitCodeRanking(t *testing.T) {
	cases := []struct {
		outcome string
		code    int
	}{
		{sweep.OutcomeClean, 0},
		{sweep.OutcomeFindings, 1},
		{sweep.OutcomeFault, 2},
	}
	for _, c := range cases {
		if got := sweep.ExitCode(c.outcome); got != c.code {
			t.Fatalf("ExitCode(%q)=%d want %d", c.outcome, got, c.code)
		}
	}
}
