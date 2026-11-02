package checks

import (
	"os/exec"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/verify/schema"
)

func TestRunExecProbePassesCompletedCommand(t *testing.T) {
	truePath, err := exec.LookPath("true")
	if err != nil {
		t.Skip("true executable unavailable")
	}
	out := runExecProbe(t.Context(), RunContext{
		Workspace: t.TempDir(),
		Verbs: map[string]*schema.Verb{
			execProbeVerbID: {Command: truePath},
		},
	})
	if out.Result.State != "pass" || len(out.Findings) != 0 {
		t.Fatalf("exec probe outcome = %+v", out)
	}
}

func TestRunExecProbeClassifiesFailureAndInfrastructure(t *testing.T) {
	falsePath, err := exec.LookPath("false")
	if err != nil {
		t.Skip("false executable unavailable")
	}
	cases := []struct {
		name  string
		verb  string
		state string
	}{
		{name: "completed failure", verb: falsePath, state: "fail"},
		{name: "missing executable", verb: "/definitely/missing/coverage-probe", state: "error"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := runExecProbe(t.Context(), RunContext{
				Workspace: t.TempDir(),
				Verbs: map[string]*schema.Verb{
					execProbeVerbID: {Command: tc.verb},
				},
			})
			if out.Result.State != tc.state || len(out.Findings) != 1 {
				t.Fatalf("exec probe outcome = %+v", out)
			}
		})
	}
}
