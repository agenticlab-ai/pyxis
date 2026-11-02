package checks

import (
	"context"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/verify/schema"
)

func TestRunBuildPassFailureAndInfrastructure(t *testing.T) {
	workspace, coverDir := t.TempDir(), t.TempDir()
	for _, tc := range []struct {
		name string
		verb *schema.Verb
		want string
	}{
		{name: "pass", verb: formatHelperVerb("clean", coverDir), want: "pass"},
		{name: "failure", verb: formatHelperVerb("fail", coverDir), want: "fail"},
		{name: "infra", verb: &schema.Verb{Command: "definitely-not-a-pyxis-command"}, want: "error"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := runBuild(context.Background(), RunContext{Workspace: workspace, LogDir: t.TempDir(), Verbs: map[string]*schema.Verb{buildVerbID: tc.verb}})
			if got.Result.State != tc.want {
				t.Fatalf("runBuild = %+v", got)
			}
		})
	}
}
