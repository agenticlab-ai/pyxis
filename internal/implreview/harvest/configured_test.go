package harvest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfiguredHarvestSelectsGitHub(t *testing.T) {
	for _, tc := range []struct {
		name, config string
		wantErr      bool
	}{
		{"legacy", "{}", false},
		{"github token", "integrations:\n  schema_version: pyxis-integrations/v1\n  connections:\n    public:\n      provider: github\n      token_env: HARVEST_TOKEN\n  bindings:\n    code_host:\n      connection: public\n      repository: o/r\n", false},
		{"bad", "integrations: {", true},
		{"bad repository", "{}", true},
		{"local-git", "integrations:\n  schema_version: pyxis-integrations/v1\n  bindings:\n    code_host:\n      provider: local-git\n      root: /configured/origin.git\n", true},
		{"forgejo", "integrations:\n  schema_version: pyxis-integrations/v1\n  connections:\n    local:\n      provider: forgejo\n      base_url: https://forgejo.test/base\n      token_env: HARVEST_TOKEN\n  bindings:\n    code_host:\n      connection: local\n      repository: o/r\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.MkdirAll(filepath.Join(root, ".pyxis"), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, ".pyxis", "config.yaml"), []byte(tc.config), 0600); err != nil {
				t.Fatal(err)
			}
			repo := "o/r"
			if tc.name == "bad repository" {
				repo = ""
			}
			a, err := NewConfiguredAdapter(root, repo)
			if (err != nil) != tc.wantErr {
				t.Fatalf("adapter=%T err=%v", a, err)
			}
			if err == nil && tc.name == "github token" {
				t.Setenv("HARVEST_TOKEN", "")
				if _, err := a.Enumerate(t.Context()); err == nil || !strings.Contains(err.Error(), "HARVEST_TOKEN") {
					t.Fatalf("missing configured token=%v", err)
				}
			}
		})
	}
}
