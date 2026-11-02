package checks

import (
	"context"
	"fmt"

	"github.com/agenticlab-ai/pyxis/internal/verify/runner"
	"github.com/agenticlab-ai/pyxis/internal/verify/schema"
)

const (
	execProbeCheckID = "exec-probe"
	execProbeVerbID  = "exec-probe"
)

func init() {
	register(schema.Check{
		ID:                execProbeCheckID,
		Rung:              "4",
		VerbDeps:          []string{execProbeVerbID},
		AuthorIndependent: true,
		InputSet:          []string{"tree"},
	}, runExecProbe)
}

// runExecProbe is the execution-reality floor (FR-13): tests passing is not
// the artifact working. No ecosystem default binds this verb — it exists only
// where a manifest declares it (FR-6 skips the rung otherwise) — and it is a
// plain exit-code check: no manifest declares a universal probe output shape
// to parse.
func runExecProbe(ctx context.Context, rc RunContext) Outcome {
	verb := rc.Verbs[execProbeVerbID]
	res := runner.Exec(ctx, rc.Workspace, verb)
	logRef := writeLog(rc, execProbeCheckID, res)

	if res.Class != "" {
		return runnerOutcome(execProbeCheckID, "4", res, logRef)
	}
	if res.Passed {
		return passOutcome(execProbeCheckID, res)
	}

	message := firstLine(res.Stderr + res.Stdout)
	if message == "" {
		message = fmt.Sprintf("exec-probe exited %d", res.ExitCode)
	}
	return failOutcome(execProbeCheckID, message, res, []schema.Finding{{
		CheckID: execProbeCheckID, Class: execProbeCheckID, Rung: "4", Message: message, LogRef: logRef,
	}})
}
