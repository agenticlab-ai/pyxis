package checks

import (
	"context"
	"fmt"

	"github.com/agenticlab-ai/pyxis/internal/verify/ecosystem"
	"github.com/agenticlab-ai/pyxis/internal/verify/runner"
	"github.com/agenticlab-ai/pyxis/internal/verify/schema"
)

const (
	formatCheckID = "format"
	formatVerbID  = "format-check"
)

func init() {
	register(schema.Check{
		ID:                formatCheckID,
		Rung:              "0",
		VerbDeps:          []string{formatVerbID},
		AuthorIndependent: true,
		InputSet:          []string{"tree"},
	}, runFormat)
}

// runFormat interprets the format-check verb according to ecosystem data.
// Nonzero exits fail in either mode; successful stdout is either informational
// or a list of unformatted files, as declared by the stack's output mode.
func runFormat(ctx context.Context, rc RunContext) Outcome {
	if rc.Manifest == nil || rc.Ecosystem == nil {
		return Outcome{Result: schema.CheckResult{CheckID: formatCheckID, State: "error", Message: "format-check requires manifest and ecosystem defaults"}}
	}
	stack := rc.Ecosystem.ForStack(rc.Manifest.Stack)
	if stack == nil {
		return Outcome{Result: schema.CheckResult{CheckID: formatCheckID, State: "error", Message: fmt.Sprintf("no formatter defaults for stack %q", rc.Manifest.Stack)}}
	}
	switch stack.FormatOutputMode {
	case ecosystem.FormatOutputExitStatus, ecosystem.FormatOutputFileList:
	default:
		return Outcome{Result: schema.CheckResult{CheckID: formatCheckID, State: "error", Message: fmt.Sprintf("unsupported formatter output mode %q", stack.FormatOutputMode)}}
	}

	verb := rc.Verbs[formatVerbID]
	res := runner.Exec(ctx, rc.Workspace, verb)
	logRef := writeLog(rc, formatCheckID, res)

	if res.Class != "" {
		return runnerOutcome(formatCheckID, "0", res, logRef)
	}
	if !res.Passed {
		msg := firstLine(res.Stderr + res.Stdout)
		if msg == "" {
			msg = fmt.Sprintf("format-check exited %d", res.ExitCode)
		}
		return failOutcome(formatCheckID, msg, res, []schema.Finding{{
			CheckID: formatCheckID, Class: formatCheckID, Rung: "0", Message: msg, LogRef: logRef,
		}})
	}
	if stack.FormatOutputMode == ecosystem.FormatOutputExitStatus {
		return passOutcome(formatCheckID, res)
	}

	files := nonBlankLines(res.Stdout)
	if len(files) == 0 {
		return passOutcome(formatCheckID, res)
	}

	findings := make([]schema.Finding, 0, len(files))
	for _, f := range files {
		findings = append(findings, schema.Finding{
			CheckID: formatCheckID, Class: formatCheckID, Rung: "0",
			Path: f, Message: "file is not formatted", LogRef: logRef,
		})
	}
	return failOutcome(formatCheckID, fmt.Sprintf("%d file(s) not formatted", len(files)), res, findings)
}
