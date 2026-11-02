package checks

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/verify/ecosystem"
	"github.com/agenticlab-ai/pyxis/internal/verify/schema"
)

func TestFormatRunnerHelper(t *testing.T) {
	if os.Getenv("PYXIS_FORMAT_HELPER") == "" {
		return
	}
	mode := os.Args[len(os.Args)-1]
	write := func(line string) {
		if _, err := fmt.Fprintln(os.Stdout, line); err != nil {
			os.Exit(2)
		}
	}
	switch mode {
	case "files":
		write("a.go\n\nb.go")
	case "summary":
		write("6 files already formatted")
	case "fail":
		os.Exit(3)
	}
	os.Exit(0)
}

func TestRunFormatUsesExitStatusForPython(t *testing.T) {
	workspace := t.TempDir()
	coverDir := t.TempDir()
	manifest := &schema.Manifest{Version: "1.0.0", Stack: "python"}
	for _, tc := range []struct {
		name, mode, state string
	}{
		{"successful summary", "summary", "pass"},
		{"formatter failure", "fail", "fail"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := runFormat(context.Background(), RunContext{
				Workspace: workspace, LogDir: t.TempDir(), Manifest: manifest, Ecosystem: ecosystem.New(),
				Verbs: map[string]*schema.Verb{formatVerbID: formatHelperVerb(tc.mode, coverDir)},
			})
			if got.Result.State != tc.state || (tc.state == "pass" && len(got.Findings) != 0) {
				t.Fatalf("python format outcome = %+v", got)
			}
		})
	}
}

func formatHelperVerb(mode, coverDir string) *schema.Verb {
	return &schema.Verb{
		Command: strconv.Quote(os.Args[0]) + " -test.run=^TestFormatRunnerHelper$ -- " + mode,
		Env: map[string]schema.EnvEntry{
			"PYXIS_FORMAT_HELPER": {Mode: schema.EnvPin, Value: "1"},
			"GOCOVERDIR":          {Mode: schema.EnvPin, Value: coverDir},
		},
	}
}

func TestRunFormatUsesEcosystemDefaultsForFourthStack(t *testing.T) {
	for _, tc := range []struct {
		mode  ecosystem.FormatOutputMode
		state string
	}{
		{ecosystem.FormatOutputFileList, "fail"},
		{ecosystem.FormatOutputExitStatus, "pass"},
	} {
		t.Run(string(tc.mode), func(t *testing.T) {
			eco := ecosystem.New()
			eco.Stacks["fourth"] = &ecosystem.StackDefaults{Name: "fourth", FormatOutputMode: tc.mode}
			got := runFormat(context.Background(), RunContext{
				Workspace: t.TempDir(), LogDir: t.TempDir(), Ecosystem: eco,
				Manifest: &schema.Manifest{Version: "1.0.0", Stack: "fourth"},
				Verbs:    map[string]*schema.Verb{formatVerbID: formatHelperVerb("files", t.TempDir())},
			})
			if got.Result.State != tc.state {
				t.Fatalf("format outcome = %+v", got)
			}
			if tc.state == "fail" {
				if len(got.Findings) != 2 || got.Findings[0].Path != "a.go" || got.Findings[1].Path != "b.go" {
					t.Fatalf("file-list findings = %+v", got.Findings)
				}
			} else if len(got.Findings) != 0 {
				t.Fatalf("exit-status findings = %+v", got.Findings)
			}
		})
	}
}

func TestRunFormatRejectsMissingOrUnknownOutputMode(t *testing.T) {
	for _, tc := range []struct {
		name     string
		manifest *schema.Manifest
		eco      *ecosystem.Defaults
	}{
		{name: "missing manifest", eco: ecosystem.New()},
		{name: "missing ecosystem", manifest: &schema.Manifest{Stack: "go"}},
		{name: "unknown stack", manifest: &schema.Manifest{Stack: "fourth"}, eco: ecosystem.New()},
		{name: "missing mode", manifest: &schema.Manifest{Stack: "fourth"}, eco: &ecosystem.Defaults{Stacks: map[string]*ecosystem.StackDefaults{"fourth": {}}}},
		{name: "unknown mode", manifest: &schema.Manifest{Stack: "fourth"}, eco: &ecosystem.Defaults{Stacks: map[string]*ecosystem.StackDefaults{"fourth": {FormatOutputMode: "unknown"}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := runFormat(context.Background(), RunContext{Manifest: tc.manifest, Ecosystem: tc.eco})
			if got.Result.State != "error" || got.Result.Message == "" {
				t.Fatalf("unsupported formatter outcome = %+v", got)
			}
		})
	}
}

func TestRunFormatClassifiesExecutionAndOutput(t *testing.T) {
	workspace := t.TempDir()
	coverDir := t.TempDir()
	cases := []struct {
		name         string
		verb         *schema.Verb
		wantState    string
		wantFindings int
		wantMessage  string
	}{
		{name: "infra", verb: &schema.Verb{Command: "definitely-not-a-pyxis-command"}, wantState: "error", wantFindings: 1},
		{name: "nonzero without diagnostic", verb: formatHelperVerb("fail", coverDir), wantState: "fail", wantFindings: 1, wantMessage: "format-check exited 3"},
		{name: "clean", verb: formatHelperVerb("clean", coverDir), wantState: "pass"},
		{name: "files", verb: formatHelperVerb("files", coverDir), wantState: "fail", wantFindings: 2, wantMessage: "2 file(s) not formatted"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var log bytes.Buffer
			_ = log
			got := runFormat(context.Background(), RunContext{
				Workspace: workspace, LogDir: t.TempDir(), Verbs: map[string]*schema.Verb{formatVerbID: tc.verb},
				Manifest: &schema.Manifest{Stack: "go"}, Ecosystem: ecosystem.New(),
			})
			if got.Result.State != tc.wantState || len(got.Findings) != tc.wantFindings {
				t.Fatalf("outcome = %+v", got)
			}
			if tc.wantMessage != "" && !strings.Contains(got.Result.Message, tc.wantMessage) {
				t.Fatalf("message = %q, want %q", got.Result.Message, tc.wantMessage)
			}
		})
	}
}
