package main

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/contractgate"
	"github.com/agenticlab-ai/pyxis/internal/docgate/gate"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

// Same fail-closed contract as plangate: a flag-like positional refuses
// before any verb action runs (incident I3's class).
func TestFlagAfterPositionalRefuses(t *testing.T) {
	for _, args := range [][]string{
		{"fold", "docs/prds/x", "--defects-json", "-"},
		{"gate", "--merge-base"}, // valueless flag consumed the marker: also refused downstream
		{"lint-structure", "-x"},
	} {
		if got := run(args); got != 2 && got != 1 {
			t.Errorf("run(%v) = %d, want refusal (1 or 2), never a silent run", args, got)
		}
	}
}

func TestRunDispatchesEmptyRepositoryAndRefusals(t *testing.T) {
	root := docgateRoot(t)
	t.Chdir(root)
	for _, tc := range []struct {
		name string
		args []string
		want int
	}{
		{name: "no verb", want: 2},
		{name: "unknown", args: []string{"unknown"}, want: 2},
		{name: "fold empty", args: []string{"fold"}, want: 0},
		{name: "lint empty", args: []string{"lint-structure"}, want: 0},
		{name: "validate empty", args: []string{"validate-amendments"}, want: 0},
		{name: "verify generated empty", args: []string{"verify-generated"}, want: 0},
		{name: "verify rebase empty", args: []string{"verify-rebase"}, want: 0},
		{name: "diff empty", args: []string{"diff"}, want: 0},
		{name: "graduate flags", args: []string{"graduate"}, want: 2},
		{name: "context pack flags", args: []string{"context-pack"}, want: 2},
		{name: "bad contract trigger", args: []string{"contract-gate", "demo", "--trigger", "invalid"}, want: 2},
		{name: "sweep conflict", args: []string{"contract-gate", "demo", "--all"}, want: 2},
		{name: "contract positional", args: []string{"contract-gate"}, want: 2},
		{name: "bad flag", args: []string{"fold", "--unknown"}, want: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, _, _ := captureDocgateOutput(t, func() int { return run(tc.args) })
			if got != tc.want {
				t.Fatalf("run(%v) = %d, want %d", tc.args, got, tc.want)
			}
		})
	}

	for _, check := range gate.Checks() {
		if !check.NeedsMergeBase {
			continue
		}
		code, _, stderr := captureDocgateOutput(t, func() int { return run([]string{check.Verb}) })
		if code != 2 || !strings.Contains(stderr, "requires --merge-base") {
			t.Fatalf("history check %s = %d stderr=%q", check.Verb, code, stderr)
		}
		break
	}
}

func TestReportWritesBothFaces(t *testing.T) {
	defect := schema.Defect{Verb: "docgate", File: "doc.md", Op: "lint", Reason: "broken", Remedy: "repair"}
	code, stdout, stderr := captureDocgateOutput(t, func() int { return report([]schema.Defect{defect}, "-") })
	if code != 1 || !strings.Contains(stdout, `"reason":"broken"`) || !strings.Contains(stderr, "broken") {
		t.Fatalf("report = %d stdout=%q stderr=%q", code, stdout, stderr)
	}
	out := filepath.Join(t.TempDir(), "defects.jsonl")
	code, _, _ = captureDocgateOutput(t, func() int { return report([]schema.Defect{defect}, out) })
	wire, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if code != 1 || !strings.Contains(string(wire), `"file":"doc.md"`) {
		t.Fatalf("file report = %d wire=%q", code, wire)
	}
	code, _, stderr = captureDocgateOutput(t, func() int {
		return report(nil, filepath.Join(t.TempDir(), "missing", "defects.jsonl"))
	})
	if code != 2 || !strings.Contains(stderr, "no such file") {
		t.Fatalf("failed report = %d stderr=%q", code, stderr)
	}
	if code, _, _ := captureDocgateOutput(t, func() int { return report(nil, "") }); code != 0 {
		t.Fatalf("clean report = %d", code)
	}
}

func TestChangedFilesArgumentLiteralAndGitRange(t *testing.T) {
	if got := splitFileList(" a.md, b.md\n\r c.md ,, "); !reflect.DeepEqual(got, []string{"a.md", "b.md", "c.md"}) {
		t.Fatalf("splitFileList = %v", got)
	}
	if got, errText := changedFilesFromArg(t.TempDir(), "a.md,b.md\nc.md"); errText != "" || !reflect.DeepEqual(got, []string{"a.md", "b.md", "c.md"}) {
		t.Fatalf("literal changed files = %v, %q", got, errText)
	}
	if _, errText := changedFilesFromArg(t.TempDir(), "missing..HEAD"); errText == "" {
		t.Fatal("invalid git range succeeded")
	}

	root := t.TempDir()
	runGit := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com", "GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	runGit("init", "-q")
	if err := os.WriteFile(filepath.Join(root, "a.md"), []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit("add", "a.md")
	runGit("commit", "-qm", "first")
	base := runGit("rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(root, "a.md"), []byte("two\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "b.md"), []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit("add", "a.md", "b.md")
	runGit("commit", "-qm", "second")
	head := runGit("rev-parse", "HEAD")
	got, errText := changedFilesFromArg(root, base+".."+head)
	if errText != "" || !reflect.DeepEqual(got, []string{"a.md", "b.md"}) {
		t.Fatalf("range changed files = %v, %q", got, errText)
	}
}

func TestContractReportAndEmptyContinuousSurfaces(t *testing.T) {
	defect := schema.Defect{Verb: "contract-gate", File: "docs/prds/demo", Op: "coverage", Reason: "missing", Remedy: "add it"}
	verdict := contractgate.Verdict{SchemaVersion: contractgate.SchemaVersion, GeneratedBy: "docgate", ToolVersion: schema.ToolVersion,
		Feature: "demo", Trigger: contractgate.TriggerPlanningEntry, Class: contractgate.ClassIncomplete, Blocking: true,
		CheckResults: []contractgate.CheckResult{{Check: "coverage", Class: contractgate.ClassIncomplete, Passed: false, Severity: contractgate.SeverityBlocking, Defects: []schema.Defect{defect}}},
		Evaluated:    []contractgate.InputIdentity{}, Semantic: contractgate.Semantic{Evaluated: false, Note: "not evaluated"}, Reason: "missing requirement coverage"}
	dir := t.TempDir()
	_, _, stderr := captureDocgateOutput(t, func() int {
		if err := emitContractReport(dir, "", verdict); err != nil {
			t.Fatal(err)
		}
		return 0
	})
	if !strings.Contains(stderr, "wrote defect report") {
		t.Fatalf("emit report stderr=%q", stderr)
	}
	entries, err := os.ReadDir(filepath.Join(dir, filepath.FromSlash(contractgate.DefaultReportDir)))
	if err != nil || len(entries) != 2 {
		t.Fatalf("report entries = %v, %v", entries, err)
	}
	badSink := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(badSink, []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := emitContractReport(dir, badSink, verdict); err == nil {
		t.Fatal("file-shaped report sink succeeded")
	}

	root := docgateRoot(t)
	code, stdout, stderr := captureDocgateOutput(t, func() int { return contractGateContinuous(root, "README.md", "-") })
	if code != 0 && code != 1 {
		t.Fatalf("continuous = %d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if !strings.Contains(stdout, `"trigger": "contract-doc-pr"`) {
		t.Fatalf("continuous stdout=%q", stdout)
	}
	code, _, stderr = captureDocgateOutput(t, func() int { return contractGateContinuous(root, "bad..range", "-") })
	if code != 2 || !strings.Contains(stderr, "git diff") {
		t.Fatalf("bad continuous = %d stderr=%q", code, stderr)
	}
	code, _, _ = captureDocgateOutput(t, func() int { return contractGateSweep(root) })
	if code != 0 && code != 1 {
		t.Fatalf("sweep = %d", code)
	}
}

func TestRepoRootFindsConfiguredAncestor(t *testing.T) {
	root := docgateRoot(t)
	nested := filepath.Join(root, "one", "two")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(nested)
	if got, err := repoRoot(); err != nil || got != root {
		t.Fatalf("repoRoot = %q, %v", got, err)
	}
}

func TestRunLedgerHappyPaths(t *testing.T) {
	base, err := os.ReadFile(filepath.Join("..", "..", "internal", "docgate", "fold", "testdata", "fixture-zero", "000-base.md"))
	if err != nil {
		t.Fatal(err)
	}
	root := docgateRoot(t)
	ledger := filepath.Join(root, "docs", "prds", "demo")
	if err := os.MkdirAll(ledger, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ledger, "000-base.md"), base, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)

	code, stdout, stderr := captureDocgateOutput(t, func() int { return run([]string{"fold", "--dry-run", ledger}) })
	if code != 0 || !strings.Contains(stdout, "# docgate") || !strings.Contains(stderr, "nothing written") {
		t.Fatalf("dry-run fold = %d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if _, err := os.Stat(filepath.Join(ledger, "effective.md")); !os.IsNotExist(err) {
		t.Fatalf("dry-run wrote effective.md: %v", err)
	}

	for _, tc := range []struct {
		verb string
		want int
	}{
		{verb: "lint-structure", want: 0},
		{verb: "validate-amendments", want: 0},
		{verb: "fold", want: 0},
		{verb: "verify-generated", want: 0},
		{verb: "verify-rebase", want: 0},
		{verb: "diff", want: 0},
	} {
		t.Run(tc.verb, func(t *testing.T) {
			code, _, stderr := captureDocgateOutput(t, func() int { return run([]string{tc.verb, ledger}) })
			if code != tc.want {
				t.Fatalf("%s = %d, stderr=%q", tc.verb, code, stderr)
			}
		})
	}
	for _, name := range []string{"effective.md", "effective.source.json"} {
		if _, err := os.Stat(filepath.Join(ledger, name)); err != nil {
			t.Fatalf("fold did not write %s: %v", name, err)
		}
	}
}

func TestRunCoversAuxiliaryAndFailureDispatch(t *testing.T) {
	root := docgateRoot(t)
	if err := os.MkdirAll(filepath.Join(root, "docs", "tech-design"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	missing := filepath.Join(root, "docs", "prds", "missing")

	for _, verb := range []string{"fold", "lint-structure", "validate-amendments", "verify-generated", "verify-rebase", "diff"} {
		t.Run(verb+" missing ledger", func(t *testing.T) {
			code, _, stderr := captureDocgateOutput(t, func() int { return run([]string{verb, missing}) })
			if code != 1 || stderr == "" {
				t.Fatalf("%s missing = %d stderr=%q", verb, code, stderr)
			}
		})
	}

	code, _, stderr := captureDocgateOutput(t, func() int { return run([]string{"gen-index"}) })
	if code != 0 {
		t.Fatalf("gen-index = %d stderr=%q", code, stderr)
	}
	for _, name := range []string{"index.md", "index.json"} {
		if _, err := os.Stat(filepath.Join(root, "docs", "tech-design", name)); err != nil {
			t.Fatalf("gen-index did not write %s: %v", name, err)
		}
	}

	code, stdout, stderr := captureDocgateOutput(t, func() int { return run([]string{"contract-gate", "demo", "--report-out", "-"}) })
	if code != 1 || !strings.Contains(stdout, `"feature": "demo"`) || stderr == "" {
		t.Fatalf("contract-gate = %d stdout=%q stderr=%q", code, stdout, stderr)
	}
	code, stdout, stderr = captureDocgateOutput(t, func() int {
		return run([]string{"contract-gate", "--changed", "README.md", "--report-out", "-"})
	})
	if code != 0 || !strings.Contains(stdout, `"trigger": "contract-doc-pr"`) {
		t.Fatalf("continuous contract-gate = %d stdout=%q stderr=%q", code, stdout, stderr)
	}

	code, _, stderr = captureDocgateOutput(t, func() int {
		return run([]string{"context-pack", "--stage", "unknown-stage", "--subject", "demo"})
	})
	if code != 1 || stderr == "" {
		t.Fatalf("context-pack = %d stderr=%q", code, stderr)
	}
	code, _, stderr = captureDocgateOutput(t, func() int {
		return run([]string{"add-incident", "--date", "bad"})
	})
	if code != 1 || stderr == "" {
		t.Fatalf("add-incident = %d stderr=%q", code, stderr)
	}
	code, _, stderr = captureDocgateOutput(t, func() int {
		return run([]string{"graduate", "--id", "999", "--principle", "AP-999"})
	})
	if code != 1 || stderr == "" {
		t.Fatalf("graduate = %d stderr=%q", code, stderr)
	}
}

func TestRunPrePushDeletionOnlyIsClean(t *testing.T) {
	root := docgateRoot(t)
	t.Chdir(root)
	const zero = "0000000000000000000000000000000000000000"
	const prior = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(w, "(delete) "+zero+" refs/heads/old "+prior+"\n"); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	oldStdin := os.Stdin
	os.Stdin = r
	t.Cleanup(func() {
		os.Stdin = oldStdin
		if err := r.Close(); err != nil {
			t.Error(err)
		}
	})
	code, _, stderr := captureDocgateOutput(t, func() int { return run([]string{"pre-push"}) })
	if code != 0 || !strings.Contains(stderr, "deletion-only push") {
		t.Fatalf("pre-push deletion = %d stderr=%q", code, stderr)
	}
}

func TestRunFeedbackFoldCoverageAndEmptyContractSweep(t *testing.T) {
	wires := map[string][]byte{}
	for _, name := range []string{"001-conversation.md", "002-code-host.md"} {
		wire, err := os.ReadFile(filepath.Join("..", "..", "internal", "docgate", "feedback", "testdata", "subject-alpha", name))
		if err != nil {
			t.Fatal(err)
		}
		wires[name] = wire
	}
	root := docgateRoot(t)
	if err := os.MkdirAll(filepath.Join(root, "docs", "tech-design"), 0o755); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "docs", "feedback", "subject-alpha")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, wire := range wires {
		if err := os.WriteFile(filepath.Join(dir, name), wire, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(root)
	code, _, stderr := captureDocgateOutput(t, func() int { return run([]string{"fold", dir}) })
	if code != 0 {
		t.Fatalf("feedback fold = %d stderr=%q", code, stderr)
	}
	code, _, stderr = captureDocgateOutput(t, func() int { return run([]string{"verify-generated", dir}) })
	if code != 0 {
		t.Fatalf("feedback verify = %d stderr=%q", code, stderr)
	}
	for _, args := range [][]string{{"coverage"}, {"coverage", "--json"}, {"contract-gate", "--all"}} {
		code, stdout, stderr := captureDocgateOutput(t, func() int { return run(args) })
		if code != 0 || stdout == "" {
			t.Fatalf("run(%v) = %d stdout=%q stderr=%q", args, code, stdout, stderr)
		}
	}
}

func docgateRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, dir := range []string{filepath.Join(root, ".pyxis"), filepath.Join(root, "docs", "prds")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, ".pyxis", "config.yaml"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func captureDocgateOutput(t *testing.T, fn func() int) (int, string, string) {
	t.Helper()
	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	oldStdout, oldStderr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = stdoutW, stderrW
	t.Cleanup(func() { os.Stdout, os.Stderr = oldStdout, oldStderr })
	code := fn()
	if err := stdoutW.Close(); err != nil {
		t.Fatal(err)
	}
	if err := stderrW.Close(); err != nil {
		t.Fatal(err)
	}
	os.Stdout, os.Stderr = oldStdout, oldStderr
	stdout, err := io.ReadAll(stdoutR)
	if err != nil {
		t.Fatal(err)
	}
	stderr, err := io.ReadAll(stderrR)
	if err != nil {
		t.Fatal(err)
	}
	if err := stdoutR.Close(); err != nil {
		t.Fatal(err)
	}
	if err := stderrR.Close(); err != nil {
		t.Fatal(err)
	}
	return code, string(stdout), string(stderr)
}
