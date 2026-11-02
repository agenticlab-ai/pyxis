package checks

import (
	"context"

	"github.com/agenticlab-ai/pyxis/internal/verify/runner"
	"github.com/agenticlab-ai/pyxis/internal/verify/schema"
)

const (
	lintCheckID = "lint"
	lintVerbID  = "lint"
)

func init() {
	register(schema.Check{
		ID:                lintCheckID,
		Rung:              "0",
		VerbDeps:          []string{lintVerbID},
		AuthorIndependent: true,
		InputSet:          []string{"tree"},
	}, runLint)
}

// runLint interprets the lint verb (ecosystem default: `go vet ./...`), whose
// exit code is meaningful (nonzero on any diagnostic) — unlike format-check,
// no output-on-success quirk to work around.
func runLint(ctx context.Context, rc RunContext) Outcome {
	verb := rc.Verbs[lintVerbID]
	res := runner.Exec(ctx, rc.Workspace, verb)
	logRef := writeLog(rc, lintCheckID, res)

	if res.Class != "" {
		return runnerOutcome(lintCheckID, "0", res, logRef)
	}
	if res.Passed {
		return passOutcome(lintCheckID, res)
	}

	findings := parseDiagnostics(lintCheckID, "0", res.Stdout+"\n"+res.Stderr, logRef)
	return failOutcome(lintCheckID, summarizeFindings(findings), res, findings)
}
