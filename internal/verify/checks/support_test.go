package checks

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/verify/runner"
	"github.com/agenticlab-ai/pyxis/internal/verify/schema"
)

func TestParseDiagnostics_MatchedLines(t *testing.T) {
	combined := "# fixture-go\n./main.go:15:2: Printf format %d has arg \"x\" of wrong type string\n./other.go:3: something else wrong\n"

	findings := parseDiagnostics("lint", "0", combined, "lint.log")

	if len(findings) != 2 {
		t.Fatalf("got %d findings, want 2: %+v", len(findings), findings)
	}
	if findings[0].Path != "./main.go" || findings[0].Line != 15 {
		t.Errorf("finding[0] = %+v, want path=./main.go line=15", findings[0])
	}
	if findings[0].Message != `Printf format %d has arg "x" of wrong type string` {
		t.Errorf("finding[0].Message = %q", findings[0].Message)
	}
	if findings[1].Path != "./other.go" || findings[1].Line != 3 {
		t.Errorf("finding[1] = %+v, want path=./other.go line=3", findings[1])
	}
	for _, f := range findings {
		if f.CheckID != "lint" || f.Class != "lint" || f.Rung != "0" || f.LogRef != "lint.log" {
			t.Errorf("finding metadata = %+v, want CheckID/Class=lint Rung=0 LogRef=lint.log", f)
		}
	}
}

func TestParseDiagnostics_NoMatch_FallsBackToOneFinding(t *testing.T) {
	combined := "cannot find package \"nonexistent\" in any of:\n\t/usr/local/go/src/nonexistent\n"

	findings := parseDiagnostics("build", "1", combined, "build.log")

	if len(findings) != 1 {
		t.Fatalf("got %d findings, want exactly 1 fallback finding: %+v", len(findings), findings)
	}
	if findings[0].Path != "" || findings[0].Line != 0 {
		t.Errorf("fallback finding = %+v, want no path/line", findings[0])
	}
	if findings[0].Message != `cannot find package "nonexistent" in any of:` {
		t.Errorf("fallback Message = %q", findings[0].Message)
	}
}

func TestParseDiagnostics_EmptyOutput_NeverDropsTheFailure(t *testing.T) {
	findings := parseDiagnostics("build", "1", "", "build.log")

	if len(findings) != 1 {
		t.Fatalf("got %d findings, want exactly 1: %+v", len(findings), findings)
	}
	if findings[0].Message != "build failed with no diagnostic output" {
		t.Errorf("Message = %q", findings[0].Message)
	}
}

func TestSummarizeFindings(t *testing.T) {
	one := []schema.Finding{{Message: "only one"}}
	if got := summarizeFindings(one); got != "only one" {
		t.Errorf("summarizeFindings(1) = %q, want the single message verbatim", got)
	}

	many := []schema.Finding{{Message: "a"}, {Message: "b"}, {Message: "c"}}
	if got := summarizeFindings(many); got != "3 findings" {
		t.Errorf("summarizeFindings(3) = %q, want %q", got, "3 findings")
	}
}

func TestFirstLine(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"single line", "hello", "hello"},
		{"leading blank lines skipped", "\n\n  \nhello\nworld", "hello"},
		{"whitespace trimmed", "  hello  \nworld", "hello"},
		{"all blank", "\n \n\t\n", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := firstLine(tt.in); got != tt.want {
				t.Errorf("firstLine(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestNonBlankLines(t *testing.T) {
	got := nonBlankLines("a.go\n\n  \nb.go\n  c.go  \n")
	want := []string{"a.go", "b.go", "c.go"}
	if len(got) != len(want) {
		t.Fatalf("nonBlankLines = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("nonBlankLines[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestWriteLog_WritesFileAndReturnsWorkspaceRelativePath(t *testing.T) {
	workspace := t.TempDir()
	logDir := filepath.Join(workspace, ".codegate", "runs", "run-1")
	rc := RunContext{Workspace: workspace, LogDir: logDir}
	res := runner.Result{Stdout: "out-content", Stderr: "err-content"}

	ref := writeLog(rc, "lint", res)

	if ref != filepath.Join(".codegate", "runs", "run-1", "lint.log") {
		t.Errorf("ref = %q, want a workspace-relative path", ref)
	}
	data, err := os.ReadFile(filepath.Join(logDir, "lint.log"))
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "out-content") || !strings.Contains(content, "err-content") {
		t.Errorf("log content = %q, want both stdout and stderr present", content)
	}
}

func TestWriteLog_EmptyLogDir_ReturnsEmptyRef(t *testing.T) {
	rc := RunContext{Workspace: t.TempDir(), LogDir: ""}
	if ref := writeLog(rc, "lint", runner.Result{}); ref != "" {
		t.Errorf("ref = %q, want empty when LogDir is unset", ref)
	}
}

func TestRunnerOutcome_ClassifiesAsErrorNeverFailOrPass(t *testing.T) {
	res := runner.Result{Class: runner.ClassInfra, Err: errors.New("spawn failed"), Duration: 5 * time.Millisecond}

	out := runnerOutcome("lint", "0", res, "lint.log")

	if out.Result.State != "error" {
		t.Errorf("State = %q, want error", out.Result.State)
	}
	if out.Result.Class != "infra" {
		t.Errorf("Class = %q, want infra", out.Result.Class)
	}
	if out.Result.Message != "spawn failed" {
		t.Errorf("Message = %q, want the underlying error text", out.Result.Message)
	}
	if len(out.Findings) != 1 || out.Findings[0].Class != "infra" || out.Findings[0].Rung != "0" {
		t.Errorf("Findings = %+v, want one infra-classed finding on rung 0", out.Findings)
	}
}

func TestPassOutcome_NoFindings(t *testing.T) {
	out := passOutcome("build", runner.Result{Duration: time.Second})
	if out.Result.State != "pass" {
		t.Errorf("State = %q, want pass", out.Result.State)
	}
	if out.Result.Class != "" {
		t.Errorf("Class = %q, want empty on a clean pass", out.Result.Class)
	}
	if len(out.Findings) != 0 {
		t.Errorf("Findings = %+v, want none on a clean pass", out.Findings)
	}
}

func TestFailOutcome_CarriesFindingsThrough(t *testing.T) {
	findings := []schema.Finding{{Message: "bad"}}
	out := failOutcome("build", "1 findings", runner.Result{Duration: time.Second}, findings)

	if out.Result.State != "fail" {
		t.Errorf("State = %q, want fail", out.Result.State)
	}
	if out.Result.Class != "build" {
		t.Errorf("Class = %q, want the check ID", out.Result.Class)
	}
	if len(out.Findings) != 1 {
		t.Errorf("Findings = %+v, want the findings passed in", out.Findings)
	}
}
