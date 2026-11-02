package checks

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/verify/runner"
	"github.com/agenticlab-ai/pyxis/internal/verify/schema"
)

// writeLog persists a verb's full combined output to rc.LogDir/<checkID>.log
// and returns a workspace-relative reference for Finding.LogRef — "full
// output attached by reference, never inline" (FR-2). A logging fault
// degrades to an empty ref rather than losing the check's own result.
func writeLog(rc RunContext, checkID string, res runner.Result) string {
	if rc.LogDir == "" {
		return ""
	}
	if err := os.MkdirAll(rc.LogDir, 0o755); err != nil {
		return ""
	}
	logPath := filepath.Join(rc.LogDir, checkID+".log")
	content := "--- stdout ---\n" + res.Stdout + "\n--- stderr ---\n" + res.Stderr + "\n"
	if err := os.WriteFile(logPath, []byte(content), 0o644); err != nil {
		return ""
	}
	if rel, err := filepath.Rel(rc.Workspace, logPath); err == nil {
		return rel
	}
	return logPath
}

// runnerOutcome turns a runner.Result that did NOT complete cleanly (infra or
// timeout — UB-1/UB-2) into the check's Outcome. It is the one place that
// classification becomes a CheckResult, shared by every repo-verb check so
// the mapping stays singular rather than reimplemented per check.
func runnerOutcome(checkID, rung string, res runner.Result, logRef string) Outcome {
	message := fmt.Sprintf("%s: unclassified execution fault", checkID)
	if res.Err != nil {
		message = res.Err.Error()
	}
	return Outcome{
		Result: schema.CheckResult{
			CheckID: checkID, Class: string(res.Class), State: "error",
			Duration: schema.DurationMS(res.Duration), Message: message,
		},
		Findings: []schema.Finding{{
			CheckID: checkID, Class: string(res.Class), Rung: rung, Message: message, LogRef: logRef,
		}},
	}
}

// passOutcome is a clean pass: no findings, no class (Class describes a
// problem's kind — a pass has none).
func passOutcome(checkID string, res runner.Result) Outcome {
	return Outcome{Result: schema.CheckResult{CheckID: checkID, State: "pass", Duration: schema.DurationMS(res.Duration)}}
}

// failOutcome is a red check result: the verb ran to completion but its
// content is a genuine defect (as opposed to runnerOutcome's execution
// fault).
func failOutcome(checkID, message string, res runner.Result, findings []schema.Finding) Outcome {
	return Outcome{
		Result: schema.CheckResult{
			CheckID: checkID, Class: checkID, State: "fail",
			Duration: schema.DurationMS(res.Duration), Message: message,
		},
		Findings: findings,
	}
}

// summarizeFindings compresses a finding slice into ONE line for
// CheckResult.Message — the full list still lives in Findings.
func summarizeFindings(findings []schema.Finding) string {
	if len(findings) == 1 {
		return findings[0].Message
	}
	return fmt.Sprintf("%d findings", len(findings))
}

// diagnosticLineRE matches a compiler/vet-style diagnostic line:
// "path/to/file.go:LINE:COL: message" (column optional). Shared by lint and
// build, whose underlying tools (go vet, go build) both use this convention.
var diagnosticLineRE = regexp.MustCompile(`(?m)^(\S+\.go):(\d+)(?::\d+)?:\s?(.+)$`)

// parseDiagnostics extracts one finding per matched diagnostic line from a
// verb's combined output. When nothing matches (an error shape neither tool
// uses, e.g. "cannot find package"), it falls back to ONE finding carrying
// the trimmed raw output — a real failure is never dropped silently.
func parseDiagnostics(checkID, rung, combined, logRef string) []schema.Finding {
	matches := diagnosticLineRE.FindAllStringSubmatch(combined, -1)
	if len(matches) == 0 {
		msg := firstLine(combined)
		if msg == "" {
			msg = checkID + " failed with no diagnostic output"
		}
		return []schema.Finding{{CheckID: checkID, Class: checkID, Rung: rung, Message: msg, LogRef: logRef}}
	}
	findings := make([]schema.Finding, 0, len(matches))
	for _, m := range matches {
		// m[2] is the regex's (\d+) line-number capture — all digits, so Atoi
		// cannot fail; a 0 line on the impossible error path is a safe default.
		line, _ := strconv.Atoi(m[2])
		findings = append(findings, schema.Finding{
			CheckID: checkID, Class: checkID, Rung: rung,
			Path: m[1], Line: line, Message: strings.TrimSpace(m[3]), LogRef: logRef,
		})
	}
	return findings
}

// nonBlankLines splits s into its non-blank, trimmed lines.
func nonBlankLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if t := strings.TrimSpace(line); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// firstLine returns s's first non-empty trimmed line — a one-line message
// (FR-2) carved out of what may be multi-line raw diagnostic output.
func firstLine(s string) string {
	lines := nonBlankLines(s)
	if len(lines) == 0 {
		return ""
	}
	return lines[0]
}
