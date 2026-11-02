package release

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sourceWorkflowProfile = `schema_version: pyxis-config/v2
integrations:
  schema_version: pyxis-integrations/v1
  connections:
    local:
      provider: forgejo
      base_url: http://127.0.0.1:3000
  bindings:
    code_host:
      connection: local
      repository: o/source
    releases:
      source:
        connection: local
        repository: o/source
  ci:
    mode: both
`

func TestSourceWorkflowRefusesInvalidPlansBeforeWriting(t *testing.T) {
	for _, tc := range []struct {
		name, profile string
		setup         func(*testing.T, string)
	}{
		{"malformed profile", "integrations:\n  unknown: true\n", nil},
		{"github host", strings.ReplaceAll(strings.ReplaceAll(sourceWorkflowProfile, "provider: forgejo", "provider: github"), "http://127.0.0.1:3000", "https://github.com"), nil},
		{"local CI", strings.ReplaceAll(sourceWorkflowProfile, "mode: both", "mode: local"), nil},
		{"bad repo config", sourceWorkflowProfile + "unknown_top_level: value\n", nil},
		{"unsafe label", sourceWorkflowProfile + "    runner_label: unsafe;command\n", nil},
		{"token collides", strings.Replace(sourceWorkflowProfile, "      provider: forgejo", "      provider: forgejo\n      token_env: PYXIS_RELEASE_TAG", 1), nil},
		{"no templates", sourceWorkflowProfile, nil},
		{"empty templates", sourceWorkflowProfile, func(t *testing.T, r string) {
			if err := os.MkdirAll(filepath.Join(r, ".pyxis/dist/forgejo-source-workflows"), 0755); err != nil {
				t.Fatal(err)
			}
		}},
		{"malformed template", sourceWorkflowProfile, func(t *testing.T, r string) { testSourceTemplate(t, r, []byte("{{unknown}}")) }},
		{"directory template", sourceWorkflowProfile, func(t *testing.T, r string) {
			if err := os.MkdirAll(filepath.Join(r, ".pyxis/dist/forgejo-source-workflows/gate.yml"), 0755); err != nil {
				t.Fatal(err)
			}
		}},
		{"symlink output", sourceWorkflowProfile, func(t *testing.T, r string) {
			if err := os.Symlink(t.TempDir(), filepath.Join(r, ".forgejo")); err != nil {
				t.Fatal(err)
			}
		}},
		{"unreadable output", sourceWorkflowProfile, func(t *testing.T, r string) {
			testSourceTemplate(t, r, []byte("name: gate\n"))
			if err := os.MkdirAll(filepath.Join(r, ".forgejo/workflows/gate.yml"), 0755); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.Mkdir(filepath.Join(root, ".pyxis"), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, ".pyxis/config.yaml"), []byte(tc.profile), 0600); err != nil {
				t.Fatal(err)
			}
			if tc.setup != nil {
				tc.setup(t, root)
			}
			if _, err := MaterializeSourceWorkflows(root); err == nil {
				t.Fatal("invalid source workflow plan accepted")
			}
		})
	}
}

func testSourceTemplate(t *testing.T, root string, data []byte) {
	t.Helper()
	dir := filepath.Join(root, ".pyxis/dist/forgejo-source-workflows")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "gate.yml"), data, 0644); err != nil {
		t.Fatal(err)
	}
}

func TestSourceWorkflowDistributionCredentialBinding(t *testing.T) {
	for _, tc := range []struct {
		name, token, want string
		bad               bool
	}{
		{"separate", "DISTRIBUTION_TOKEN", "DISTRIBUTION_TOKEN", false},
		{"shared", "PYXIS_FORGEJO_TOKEN", "PYXIS_UNUSED_DISTRIBUTION_TOKEN", false},
		{"reserved", "PYXIS_RELEASE_TAG", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			profile := strings.Replace(sourceWorkflowProfile, "  bindings:", "    dist:\n      provider: forgejo\n      base_url: http://127.0.0.1:3000\n      token_env: "+tc.token+"\n  bindings:", 1)
			profile = strings.Replace(profile, "  ci:", "      distribution:\n        connection: dist\n        repository: o/dist\n  ci:", 1)
			testSourceTemplate(t, root, []byte("enabled: {{distribution_enabled}}\ncredential: {{distribution_token_env}}\n"))
			if err := os.WriteFile(filepath.Join(root, ".pyxis/config.yaml"), []byte(profile), 0600); err != nil {
				t.Fatal(err)
			}
			paths, err := MaterializeSourceWorkflows(root)
			if tc.bad {
				if err == nil {
					t.Fatal("reserved credential environment accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			b, err := os.ReadFile(paths[0])
			if err != nil {
				t.Fatal(err)
			}
			if string(b) != "enabled: true\ncredential: "+tc.want+"\n" {
				t.Fatalf("unexpected credential binding: %s", b)
			}
		})
	}
}

func TestSourceWorkflowInheritsLegacyCIMode(t *testing.T) {
	root := t.TempDir()
	testSourceTemplate(t, root, []byte("name: source\n"))
	profile := strings.Replace(sourceWorkflowProfile, "  ci:\n    mode: both\n", "", 1) + "knobs:\n  ci_mode: both\n"
	if err := os.WriteFile(filepath.Join(root, ".pyxis/config.yaml"), []byte(profile), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := MaterializeSourceWorkflows(root); err != nil {
		t.Fatal(err)
	}
}

func TestSourceWorkflowTemplatesStayOutOfInstalledPayload(t *testing.T) {
	if !isDevOnlyPayloadPath(".pyxis/dist/forgejo-source-workflows/docs-gate.yml") {
		t.Fatal("source toolchain workflow would ship in the installed payload")
	}
	if isDevOnlyPayloadPath(".pyxis/dist/forgejo-workflows/docs-gate.yml") {
		t.Fatal("binary gate template would be removed from the installed payload")
	}
}

func TestSourceWorkflowIndependentGitHubReleaseDefaults(t *testing.T) {
	root := t.TempDir()
	testSourceTemplate(t, root, []byte("source: {{release_token_env}}\ndistribution: {{distribution_token_env}}\n"))
	if err := os.WriteFile(filepath.Join(root, ".pyxis/dist/forgejo-source-workflows/README.md"), []byte("{{documentation_is_not_a_workflow}}"), 0600); err != nil {
		t.Fatal(err)
	}
	profile := strings.Replace(sourceWorkflowProfile, "  bindings:", "    public:\n      provider: github\n  bindings:", 1)
	profile = strings.Replace(profile, "      source:\n        connection: local", "      source:\n        connection: public", 1)
	profile = strings.Replace(profile, "  ci:", "      distribution:\n        connection: public\n        repository: o/dist\n  ci:", 1)
	if err := os.WriteFile(filepath.Join(root, ".pyxis/config.yaml"), []byte(profile), 0600); err != nil {
		t.Fatal(err)
	}
	paths, err := MaterializeSourceWorkflows(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 {
		t.Fatalf("published non-workflow documentation: %v", paths)
	}
	b, err := os.ReadFile(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "source: GH_TOKEN\ndistribution: PYXIS_UNUSED_DISTRIBUTION_TOKEN\n" {
		t.Fatalf("GitHub release defaults: %s", b)
	}
}
