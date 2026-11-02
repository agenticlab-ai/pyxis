package main

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/draft/lifecycle"
	"github.com/agenticlab-ai/pyxis/internal/plangate/project"
)

func TestPrecheckPrintsBindingBasis(t *testing.T) {
	previous := planningPrecheck
	planningPrecheck = func(_ context.Context, _ string, feature string, run lifecycle.RunCommand) (string, []schema.Defect) {
		if feature != "widget" || run == nil {
			t.Fatalf("precheck feature=%q runner=%v", feature, run)
		}
		return "lock/widget/v2", nil
	}
	t.Cleanup(func() { planningPrecheck = previous })
	code, stdout, stderr := captureProcessOutput(t, func() int { return run([]string{"precheck", "widget"}) })
	if code != 0 || stdout != "lock/widget/v2\n" || stderr != "" {
		t.Fatalf("precheck code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestMainDelegatesItsExitCode(t *testing.T) {
	oldArgs, oldExit := os.Args, processExit
	t.Cleanup(func() {
		os.Args = oldArgs
		processExit = oldExit
	})
	os.Args = []string{"plangate"}
	got := -1
	processExit = func(code int) { got = code }
	main()
	if got != 2 {
		t.Fatalf("main exit = %d, want 2", got)
	}
}

func TestRunReachesRepositoryAndProjectBoundaryFailures(t *testing.T) {
	oldGetwd := currentWorkingDir
	currentWorkingDir = func() (string, error) { return "", errors.New("cwd unavailable") }
	t.Cleanup(func() { currentWorkingDir = oldGetwd })
	if code, _, stderr := captureProcessOutput(t, func() int { return run([]string{"fold"}) }); code != 2 || !strings.Contains(stderr, "cwd unavailable") {
		t.Fatalf("run with unavailable cwd = %d stderr=%q", code, stderr)
	}
	if _, err := repoRoot(); err == nil {
		t.Fatal("repoRoot hid a working-directory failure")
	}
	currentWorkingDir = oldGetwd

	root := t.TempDir()
	initPlanningRepository(t, root)
	if out, err := exec.Command("git", "-C", root, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	if out, err := exec.Command("git", "-C", root, "remote", "add", "origin", "https://github.com/owner/repo.git").CombinedOutput(); err != nil {
		t.Fatalf("git remote add: %v: %s", err, out)
	}
	binDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(binDir, "gh"), []byte("#!/bin/sh\nprintf 'owner/repo\\n'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Chdir(root)
	code, _, stderr := captureProcessOutput(t, func() int {
		return run([]string{"project", "docs/plans/missing"})
	})
	if code != 2 || !strings.Contains(stderr, "docs/plans/missing") {
		t.Fatalf("project missing plan = %d stderr=%q", code, stderr)
	}
}

// A flag after a positional must refuse before any action — the fail-open
// that ran a "dry-run" live (incident I3).
func TestFlagAfterPositionalRefuses(t *testing.T) {
	for _, args := range [][]string{
		{"project", "docs/plans/x", "--dry-run"},
		{"fold", "docs/plans/x", "--defects-json", "-"},
		{"verify", "-x"},
	} {
		if got := run(args); got != 2 {
			t.Errorf("run(%v) = %d, want 2 (usage refusal before any action)", args, got)
		}
	}
}

func TestUsageAndEmptyRepositoryVerbs(t *testing.T) {
	root := t.TempDir()
	initPlanningRepository(t, root)
	t.Chdir(root)

	for _, tc := range []struct {
		name string
		args []string
		want int
	}{
		{name: "no verb", want: 2},
		{name: "unknown", args: []string{"unknown"}, want: 2},
		{name: "fold empty", args: []string{"fold"}, want: 0},
		{name: "verify empty", args: []string{"verify"}, want: 0},
		{name: "coverage empty", args: []string{"coverage"}, want: 0},
		{name: "render needs one directory", args: []string{"render"}, want: 2},
		{name: "precheck needs a slug", args: []string{"precheck"}, want: 2},
		{name: "unknown tracker", args: []string{"project", "--tracker", "other", "docs/plans/missing"}, want: 2},
		{name: "bad flag", args: []string{"fold", "--not-a-flag"}, want: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, _, _ := captureProcessOutput(t, func() int { return run(tc.args) })
			if got != tc.want {
				t.Fatalf("run(%v) = %d, want %d", tc.args, got, tc.want)
			}
		})
	}
}

func TestEmitHumanAndMachineFaces(t *testing.T) {
	defect := schema.Defect{Verb: "plangate", File: "plan.md", Op: "test", Reason: "broken", Remedy: "fix it"}
	code, stdout, stderr := captureProcessOutput(t, func() int { return emit([]schema.Defect{defect}, "-") })
	if code != 1 || !strings.Contains(stdout, `"reason":"broken"`) || !strings.Contains(stderr, "broken") {
		t.Fatalf("emit = %d stdout=%q stderr=%q", code, stdout, stderr)
	}

	out := filepath.Join(t.TempDir(), "defects.jsonl")
	code, _, _ = captureProcessOutput(t, func() int { return emit([]schema.Defect{defect}, out) })
	wire, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if code != 1 || !strings.Contains(string(wire), `"file":"plan.md"`) {
		t.Fatalf("file emit = %d wire=%q", code, wire)
	}

	code, _, stderr = captureProcessOutput(t, func() int {
		return emit(nil, filepath.Join(t.TempDir(), "missing", "defects.jsonl"))
	})
	if code != 2 || !strings.Contains(stderr, "no such file") {
		t.Fatalf("failed emit = %d stderr=%q", code, stderr)
	}
	if code, _, _ := captureProcessOutput(t, func() int { return emit(nil, "") }); code != 0 {
		t.Fatalf("clean emit = %d", code)
	}
}

func TestPlanDirectoryHelpers(t *testing.T) {
	root := t.TempDir()
	cfg := config.Defaults()
	got, err := planDirs(root, cfg)
	if err != nil || got != nil {
		t.Fatalf("absent plan root = %v, %v", got, err)
	}
	for _, name := range []string{"zeta", "alpha"} {
		if err := os.MkdirAll(filepath.Join(root, "docs", "plans", name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "plans", "README.md"), []byte("not a plan"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err = planDirs(root, cfg)
	if err != nil || !reflect.DeepEqual(got, []string{"docs/plans/alpha", "docs/plans/zeta"}) {
		t.Fatalf("plan dirs = %v, %v", got, err)
	}

	dir := filepath.Join(root, "docs", "plans", "alpha")
	for _, name := range []string{"001-plan.md", "003-plan.md", "notes.md"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("plan"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if got, err := highestPlan(root, "docs/plans/alpha"); err != nil || got != "docs/plans/alpha/003-plan.md" {
		t.Fatalf("highest plan = %q, %v", got, err)
	}
	if got, err := highestPlan(root, "docs/plans/zeta"); err != nil || got != "" {
		t.Fatalf("empty highest plan = %q, %v", got, err)
	}
	if _, err := highestPlan(root, "docs/plans/missing"); err == nil {
		t.Fatal("missing plan directory did not fail")
	}
}

func TestRepoRootAndCarinaResolution(t *testing.T) {
	root := t.TempDir()
	initPlanningRepository(t, root)
	nested := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(nested)
	if got, err := repoRoot(); err != nil || got != root {
		t.Fatalf("repoRoot = %q, %v", got, err)
	}

	binDir := t.TempDir()
	bin := filepath.Join(binDir, "carina")
	if err := os.WriteFile(bin, nil, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir)
	if got, err := resolveCarinaBin(root); err != nil || got != bin {
		t.Fatalf("resolveCarinaBin = %q, %v", got, err)
	}
	t.Setenv("PATH", "")
	if _, err := resolveCarinaBin(t.TempDir()); err == nil {
		t.Fatal("missing carina and Go toolchain did not fail")
	}
}

func TestPrintReport(t *testing.T) {
	code, stdout, _ := captureProcessOutput(t, func() int {
		printReport(nil)
		printReport(&project.Report{Schema: project.ReportSchemaVersion, Tool: "plangate", Plan: "docs/plans/x/001-plan.md", Feature: "x", Version: "001", Dialect: "test", Routing: "withheld", Items: []project.Item{}})
		return 0
	})
	if code != 0 || !strings.Contains(stdout, `"schema_version": "plan-projection-report/v1"`) {
		t.Fatalf("printReport stdout=%q", stdout)
	}
}

func TestRunExercisesFrozenPlanAndExplicitFailurePaths(t *testing.T) {
	planWire, err := os.ReadFile(filepath.Join("..", "..", "docs", "plans", "ecosystem-help", "001-plan.md"))
	if err != nil {
		t.Fatal(err)
	}
	prdWire, err := os.ReadFile(filepath.Join("..", "..", "docs", "prds", "ecosystem-help", "effective.source.json"))
	if err != nil {
		t.Fatal(err)
	}
	prdMarkdown, err := os.ReadFile(filepath.Join("..", "..", "docs", "prds", "ecosystem-help", "effective.md"))
	if err != nil {
		t.Fatal(err)
	}
	designWire, err := os.ReadFile(filepath.Join("..", "..", "docs", "tech-design", "ecosystem-help-v1.md"))
	if err != nil {
		t.Fatal(err)
	}
	mermaidWire, err := os.ReadFile(filepath.Join("..", "..", ".pyxis", "vendor", "mermaid", "mermaid.min.js"))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	initPlanningRepository(t, root)
	dir := filepath.Join(root, "docs", "plans", "ecosystem-help")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "001-plan.md"), planWire, 0o644); err != nil {
		t.Fatal(err)
	}
	prdDir := filepath.Join(root, "docs", "prds", "ecosystem-help")
	if err := os.MkdirAll(prdDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(prdDir, "effective.source.json"), prdWire, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(prdDir, "effective.md"), prdMarkdown, 0o644); err != nil {
		t.Fatal(err)
	}
	designDir := filepath.Join(root, "docs", "tech-design")
	if err := os.MkdirAll(designDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(designDir, "ecosystem-help-v1.md"), designWire, 0o644); err != nil {
		t.Fatal(err)
	}
	mermaidDir := filepath.Join(root, ".pyxis", "vendor", "mermaid")
	if err := os.MkdirAll(mermaidDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mermaidDir, "mermaid.min.js"), mermaidWire, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	relDir := filepath.ToSlash(filepath.Join("docs", "plans", "ecosystem-help"))

	code, _, stderr := captureProcessOutput(t, func() int { return run([]string{"fold", relDir}) })
	if code != 0 {
		t.Fatalf("fold = %d stderr=%q", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(dir, "001-plan.json")); err != nil {
		t.Fatalf("fold output: %v", err)
	}
	code, _, stderr = captureProcessOutput(t, func() int { return run([]string{"verify", relDir}) })
	if code != 0 {
		t.Fatalf("verify = %d stderr=%q", code, stderr)
	}
	reviewPage := "plan-review.html"
	code, _, stderr = captureProcessOutput(t, func() int {
		return run([]string{"render", "--out", reviewPage, relDir})
	})
	if code != 0 {
		t.Fatalf("render = %d stderr=%q", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(root, reviewPage)); err != nil {
		t.Fatalf("render output: %v", err)
	}

	jsonPath := filepath.Join(dir, "001-plan.json")
	if err := os.Remove(jsonPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(jsonPath, 0o755); err != nil {
		t.Fatal(err)
	}
	code, _, stderr = captureProcessOutput(t, func() int { return run([]string{"fold", relDir}) })
	if code != 2 {
		t.Fatalf("fold over unwritable output = %d stderr=%q", code, stderr)
	}
	if err := os.Remove(jsonPath); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "002-plan.md"), []byte("not a plan"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, stderr = captureProcessOutput(t, func() int { return run([]string{"fold", relDir}) })
	if code != 1 {
		t.Fatalf("fold malformed plan = %d stderr=%q", code, stderr)
	}

	missing := filepath.ToSlash(filepath.Join("docs", "plans", "missing"))
	for _, tc := range []struct {
		args []string
		want int
	}{
		{args: []string{"fold", missing}, want: 2},
		{args: []string{"verify", missing}, want: 2},
		{args: []string{"coverage", missing}, want: 1},
		{args: []string{"render", missing}, want: 1},
		{args: []string{"precheck", "missing"}, want: 1},
		{args: []string{"project", relDir}, want: 2},
		{args: []string{"report", relDir}, want: 2},
	} {
		code, _, stderr := captureProcessOutput(t, func() int { return run(tc.args) })
		if code != tc.want {
			t.Errorf("run(%v) = %d stderr=%q, want %d", tc.args, code, stderr, tc.want)
		}
	}
}

func TestRunSurfacesRepositoryConfigurationAndPlanRootFailures(t *testing.T) {
	root := t.TempDir()
	initPlanningRepository(t, root)
	if err := os.MkdirAll(filepath.Join(root, ".pyxis"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".pyxis", "config.yaml"), []byte("unknown_key: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	if code, _, _ := captureProcessOutput(t, func() int { return run([]string{"fold"}) }); code != 1 {
		t.Fatalf("malformed configuration exit = %d", code)
	}

	if err := os.RemoveAll(filepath.Join(root, ".pyxis")); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "plans"), []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, _, _ := captureProcessOutput(t, func() int { return run([]string{"fold"}) }); code != 2 {
		t.Fatalf("unreadable plans root exit = %d", code)
	}
}

func TestRunCarinaSelectionSkipsAnEmptyPlanDirectory(t *testing.T) {
	root := t.TempDir()
	initPlanningRepository(t, root)
	emptyPlanDir := filepath.Join(root, "docs", "plans", "empty")
	if err := os.MkdirAll(emptyPlanDir, 0o755); err != nil {
		t.Fatal(err)
	}
	binDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(binDir, "carina"), nil, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir)
	t.Chdir(root)
	code, _, stderr := captureProcessOutput(t, func() int {
		return run([]string{"project", "--tracker", "carina", "docs/plans/empty"})
	})
	if code != 0 {
		t.Fatalf("empty carina projection = %d stderr=%q", code, stderr)
	}
}

func TestRepoAndPlanDirectoryBoundaryErrors(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	if _, err := repoRoot(); err == nil {
		t.Fatal("repoRoot found a repository above an isolated temporary directory")
	}

	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "plans"), []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := planDirs(root, config.Defaults()); err == nil {
		t.Fatal("planDirs accepted a non-directory plans root")
	}
}

func captureProcessOutput(t *testing.T, fn func() int) (int, string, string) {
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
	if err := stdoutW.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
		t.Fatal(err)
	}
	if err := stderrW.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
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
	if err := stdoutR.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
		t.Fatal(err)
	}
	if err := stderrR.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
		t.Fatal(err)
	}
	return code, string(stdout), string(stderr)
}

// Installed operands need neither engine source nor a product Go module.
func initPlanningRepository(t *testing.T, root string) {
	t.Helper()
	if out, err := exec.Command("git", "init", "-q", root).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	if err := os.MkdirAll(filepath.Join(root, ".pyxis"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".pyxis", "config.yaml"), []byte("schema_version: pyxis-config/v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestPlanningCommandsFindInstalledOperandWithoutGoModule(t *testing.T) {
	parent := t.TempDir()
	if err := os.WriteFile(filepath.Join(parent, "go.mod"), []byte("module unrelated.example/parent\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(parent, "operand")
	if err := os.MkdirAll(filepath.Join(root, ".pyxis"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".pyxis", "config.yaml"), []byte("schema_version: pyxis-config/v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(root, "src")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "go.mod"), []byte("module unrelated.example/nested\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{root, nested} {
		t.Chdir(dir)
		if got, err := repoRoot(); err != nil || got != root {
			t.Fatalf("repoRoot from %s = %q, %v; want operand %s", dir, got, err, root)
		}
		for _, verb := range []string{"fold", "verify", "coverage"} {
			code, _, stderr := captureProcessOutput(t, func() int { return run([]string{verb}) })
			if code != 0 {
				t.Fatalf("installed operand %s from %s = %d: %s", verb, dir, code, stderr)
			}
		}
	}
}

func TestInstalledOperandPrecheckRefusesUnwritableBasis(t *testing.T) {
	root := t.TempDir()
	initPlanningRepository(t, root)
	t.Chdir(root)
	previous := planningPrecheck
	planningPrecheck = func(_ context.Context, gotRoot, _ string, _ lifecycle.RunCommand) (string, []schema.Defect) {
		if gotRoot != root {
			t.Fatalf("precheck resolved %q, want installed operand %q", gotRoot, root)
		}
		return "lock/widget/v2", nil
	}
	t.Cleanup(func() { planningPrecheck = previous })
	code, stdout, stderr := captureProcessOutput(t, func() int {
		// A consumer cannot use a binding Basis that the command failed to emit.
		if err := os.Stdout.Close(); err != nil {
			t.Fatal(err)
		}
		return run([]string{"precheck", "widget"})
	})
	if code != 2 || stdout != "" || !strings.Contains(stderr, "print binding Basis") {
		t.Fatalf("unwritable precheck = %d stdout=%q stderr=%q", code, stdout, stderr)
	}
}
