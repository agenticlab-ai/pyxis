package checks

import (
	"context"

	"github.com/agenticlab-ai/pyxis/internal/verify/runner"
	"github.com/agenticlab-ai/pyxis/internal/verify/schema"
)

const (
	buildCheckID = "build"
	buildVerbID  = "build"
)

func init() {
	register(schema.Check{
		ID:                buildCheckID,
		Rung:              "1",
		VerbDeps:          []string{buildVerbID},
		AuthorIndependent: true,
		InputSet:          []string{"tree"},
	}, runBuild)
}

// runBuild interprets the build verb (ecosystem default: `go build ./...`):
// nothing downstream is meaningful on a broken build (FR-9).
func runBuild(ctx context.Context, rc RunContext) Outcome {
	verb := rc.Verbs[buildVerbID]
	res := runner.Exec(ctx, rc.Workspace, verb)
	logRef := writeLog(rc, buildCheckID, res)

	if res.Class != "" {
		return runnerOutcome(buildCheckID, "1", res, logRef)
	}
	if res.Passed {
		return passOutcome(buildCheckID, res)
	}

	findings := parseDiagnostics(buildCheckID, "1", res.Stdout+"\n"+res.Stderr, logRef)
	return failOutcome(buildCheckID, summarizeFindings(findings), res, findings)
}
