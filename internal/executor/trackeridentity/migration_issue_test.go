package trackeridentity

import (
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/integrations"
)

func TestIssueMigrationLimitsCompatibilityToMatchingHostedBinding(t *testing.T) {
	old := migrationOutput
	t.Cleanup(func() { migrationOutput = old })
	t.Setenv("MIGRATION_TEST_TOKEN", "test-token")
	binding := integrations.ResolvedRepository{Provider: integrations.GitHub, BaseURL: "https://github.com", Repository: "o/r", TokenEnv: "MIGRATION_TEST_TOKEN"}
	offline := errors.New("offline")
	for _, tc := range []struct {
		name, claims, proposals string
		tasks                   integrations.ResolvedRepository
		number                  int
		gitErr, hostErr         error
		wantErr                 bool
	}{
		{name: "unrelated claims and proposals", tasks: binding, number: 1492, claims: "sha\trefs/claim/1582\n", proposals: `[{"number":9,"head":{"ref":"feat/prototype-1582"}}]`},
		{name: "selected claim", tasks: binding, number: 1492, claims: "sha\trefs/claim/1492\n", wantErr: true},
		{name: "selected proposal", tasks: binding, number: 1492, proposals: `[{"number":9,"head":{"ref":"feat/previous-1492"}}]`, wantErr: true},
		{name: "scoped proposal", tasks: binding, number: 1492, proposals: `[{"number":9,"head":{"ref":"feat/source/previous-1492"}}]`},
		{name: "different repository", tasks: integrations.ResolvedRepository{Provider: binding.Provider, BaseURL: binding.BaseURL, Repository: "o/other"}, number: 1492, claims: "sha\trefs/claim/1582\n", wantErr: true},
		{name: "different host", tasks: integrations.ResolvedRepository{Provider: binding.Provider, BaseURL: "https://elsewhere.example", Repository: binding.Repository}, number: 1492, claims: "sha\trefs/claim/1582\n", wantErr: true},
		{name: "different provider", tasks: integrations.ResolvedRepository{Provider: integrations.Forgejo, BaseURL: binding.BaseURL, Repository: binding.Repository}, number: 1492, claims: "sha\trefs/claim/1582\n", wantErr: true},
		{name: "foreign binding without legacy work", tasks: integrations.ResolvedRepository{Provider: integrations.Carina, Root: "/repo"}, number: 1492},
		{name: "foreign proposal", tasks: integrations.ResolvedRepository{Provider: integrations.Carina, Root: "/repo"}, number: 1492, proposals: `[{"number":9,"head":{"ref":"feat/prototype-1582"}}]`, wantErr: true},
		{name: "claim read failure", tasks: binding, number: 1492, gitErr: offline, wantErr: true},
		{name: "proposal read failure", tasks: binding, number: 1492, hostErr: offline, wantErr: true},
		{name: "malformed proposals", tasks: binding, number: 1492, proposals: "{", wantErr: true},
		{name: "invalid number", tasks: binding, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			migrationOutput = func(cmd *exec.Cmd) ([]byte, error) {
				if cmd.Args[0] == "git" {
					if !strings.Contains(strings.Join(cmd.Args, " "), "ls-remote origin refs/claim/*") {
						t.Fatalf("unexpected claim operation: %v", cmd.Args)
					}
					return []byte(tc.claims), tc.gitErr
				}
				proposals := tc.proposals
				if proposals == "" {
					proposals = "[]"
				}
				return []byte(proposals), tc.hostErr
			}
			key, err := CheckIssueMigration(t.Context(), "repo", "origin", tc.tasks, binding, tc.number)
			if (err != nil) != tc.wantErr {
				t.Fatalf("migration admission = %q, %v", key, err)
			}
			wantKey := ""
			if !tc.wantErr && tc.tasks.Key() == binding.Key() {
				wantKey = binding.Key()
			}
			if key != wantKey {
				t.Fatalf("compatibility source = %q, want %q", key, wantKey)
			}
			if (tc.gitErr != nil || tc.hostErr != nil) && !errors.Is(err, offline) {
				t.Fatalf("read failure lost: %v", err)
			}
		})
	}
}
