package integrations_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/integrations"
)

const configured = `paths:
  prds: docs/prds
integrations:
  schema_version: pyxis-integrations/v1
  connections:
    forge:
      provider: forgejo
      base_url: http://127.0.0.1:3000/forge/
      token_env: TEST_FORGEJO_TOKEN
      draft_prefix: 'WIP:'
    hub:
      provider: github
  bindings:
    tasks:
      connection: forge
      repository: owner/tasks
    code_host:
      connection: hub
      repository: owner/code
    releases:
      source:
        connection: forge
        repository: owner/releases
      distribution:
        connection: hub
        repository: owner/dist
    artifacts:
      connection: forge
      repository: owner/tools
  ci:
    mode: both
    runner_label: pyxis-linux
executor:
  unrelated: ignored
`

func TestIndependentBindingsAndIdentity(t *testing.T) {
	c, err := integrations.Parse([]byte(configured))
	if err != nil {
		t.Fatal(err)
	}
	tasks, err := c.Tasks("legacy/repo", "gh", "")
	if err != nil {
		t.Fatal(err)
	}
	host, err := c.CodeHost("legacy/repo")
	if err != nil {
		t.Fatal(err)
	}
	source, err := c.Release("source", "legacy/repo")
	if err != nil {
		t.Fatal(err)
	}
	dist, err := c.Release("distribution", "legacy/repo")
	if err != nil {
		t.Fatal(err)
	}
	artifact, ok, err := c.Artifacts()
	if err != nil || !ok {
		t.Fatalf("artifacts: %v %v", ok, err)
	}
	if tasks.Provider != integrations.Forgejo || host.Provider != integrations.GitHub || source.Repository != "owner/releases" || dist.Repository != "owner/dist" || artifact.Repository != "owner/tools" {
		t.Fatalf("bindings mixed: %#v %#v %#v %#v %#v", tasks, host, source, dist, artifact)
	}
	if tasks.BaseURL != "http://127.0.0.1:3000/forge" || tasks.DraftPrefix != "WIP:" {
		t.Fatalf("normalized connection: %#v", tasks)
	}
	t.Setenv("TEST_FORGEJO_TOKEN", "secret-value")
	if tasks.Token() != "secret-value" || strings.Contains(tasks.Key(), "secret-value") {
		t.Fatal("credential reference or key failed")
	}
	other := tasks
	other.BaseURL = "http://127.0.0.1:3001/forge"
	if tasks.Key() == other.Key() {
		t.Fatal("different server ports share identity")
	}
	other.BaseURL = "http://127.0.0.1:3000/other"
	if tasks.Key() == other.Key() {
		t.Fatal("different server paths share identity")
	}
}

func TestMissingBindingsPreserveLegacy(t *testing.T) {
	c, err := integrations.Parse([]byte("executor:\n  unknown: ignored\n"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tracker := range []string{"", "gh", "github", "carina"} {
		t.Run(tracker, func(t *testing.T) {
			r, err := c.Tasks("o/r", tracker, "/tmp/carina")
			if err != nil {
				t.Fatal(err)
			}
			if tracker == "carina" {
				if r.Provider != "carina" || r.Root != "/tmp/carina" {
					t.Fatalf("%#v", r)
				}
				return
			}
			if r.Provider != "github" || r.Repository != "o/r" || r.BaseURL != "https://github.com" {
				t.Fatalf("%#v", r)
			}
		})
	}
	if _, present, err := c.Artifacts(); present || err != nil {
		t.Fatalf("unexpected artifact override: %v %v", present, err)
	}
	if _, err := c.Release("invalid", "o/r"); !errors.Is(err, integrations.ErrInvalid) {
		t.Fatalf("%v", err)
	}
	if _, err := c.Tasks("o/r", "bad", ""); !errors.Is(err, integrations.ErrInvalid) {
		t.Fatalf("%v", err)
	}
}

func TestProviderCombinationMatrix(t *testing.T) {
	c := integrations.Config{Connections: map[string]integrations.Connection{
		"forge": {Provider: "forgejo", BaseURL: "http://127.0.0.1:3000"},
		"hub":   {Provider: "github"},
	}}
	for _, task := range []string{"forge", "hub", "carina"} {
		for _, host := range []string{"forge", "hub"} {
			t.Run(task+"-"+host, func(t *testing.T) {
				cfg := c
				cfg.Bindings.Tasks = integrations.Binding{Connection: task, Repository: "o/tasks"}
				if task == "carina" {
					cfg.Bindings.Tasks = integrations.Binding{Provider: "carina", Root: ".pyxis/work-items"}
				}
				cfg.Bindings.CodeHost = integrations.Binding{Connection: host, Repository: "o/code"}
				if _, err := cfg.Tasks("", "", ""); err != nil {
					t.Fatal(err)
				}
				if _, err := cfg.CodeHost(""); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestClosedSchemaAndUnsafeInputs(t *testing.T) {
	cases := map[string]string{
		"unknown block key":      strings.Replace(configured, "  ci:\n", "  mystery:\n", 1),
		"unknown connection key": strings.Replace(configured, "      token_env:", "      token:", 1),
		"unknown binding":        strings.Replace(configured, "    tasks:", "    taskz:", 1),
		"unknown release":        strings.Replace(configured, "      distribution:", "      other:", 1),
		"unknown ci":             strings.Replace(configured, "    runner_label:", "    runner_secret:", 1),
		"missing schema":         strings.Replace(configured, "  schema_version: pyxis-integrations/v1\n", "", 1),
		"bad schema":             strings.Replace(configured, "pyxis-integrations/v1", "pyxis-integrations/v2", 1),
		"unknown connection":     strings.Replace(configured, "connection: forge", "connection: missing", 1),
		"unknown provider":       strings.Replace(configured, "provider: forgejo", "provider: gitlab", 1),
		"credential URL":         strings.Replace(configured, "http://127.0.0.1:3000/forge/", "https://user:secret@example.test/", 1),
		"remote HTTP":            strings.Replace(configured, "http://127.0.0.1:3000/forge/", "http://example.test/", 1),
		"query URL":              strings.Replace(configured, "http://127.0.0.1:3000/forge/", "https://example.test/?token=secret", 1),
		"path traversal":         strings.Replace(configured, "http://127.0.0.1:3000/forge/", "https://example.test/forge/../", 1),
		"bad env":                strings.Replace(configured, "TEST_FORGEJO_TOKEN", "SECRET=value", 1),
		"bad repo":               strings.Replace(configured, "owner/tasks", "../tasks", 1),
		"duplicate":              strings.Replace(configured, "  schema_version:", "  ci:\n    mode: local\n  schema_version:", 1),
		"scalar child":           strings.Replace(configured, "  connections:", "  connections: literal", 1),
		"bad indent":             strings.Replace(configured, "      provider: forgejo", "       provider: forgejo", 1),
		"tab indent":             strings.Replace(configured, "      provider: forgejo", "\tprovider: forgejo", 1),
		"flow map":               strings.Replace(configured, "  connections:", "  connections: {}", 1),
		"bad mode":               strings.Replace(configured, "mode: both", "mode: remote", 1),
		"duplicate block":        configured + "integrations:\n  schema_version: pyxis-integrations/v1\n",
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := integrations.Parse([]byte(input)); !errors.Is(err, integrations.ErrInvalid) {
				t.Fatalf("expected closed-parse error, got %v", err)
			}
		})
	}
}

func TestLocalOverlayMergesOneConnectionLeaf(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".pyxis")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(configured), 0600); err != nil {
		t.Fatal(err)
	}
	local := "integrations:\n  connections:\n    forge:\n      base_url: http://127.0.0.1:4000\n"
	if err := os.WriteFile(filepath.Join(dir, "config.local.yaml"), []byte(local), 0600); err != nil {
		t.Fatal(err)
	}
	c, err := integrations.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	r, err := c.Tasks("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if r.BaseURL != "http://127.0.0.1:4000" || r.Repository != "owner/tasks" || r.TokenEnv != "TEST_FORGEJO_TOKEN" {
		t.Fatalf("overlay lost siblings: %#v", r)
	}
}

func TestRelativeCarinaOperandResolvesFromConfiguredRepository(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".pyxis"), 0755); err != nil {
		t.Fatal(err)
	}
	input := "integrations:\n  schema_version: pyxis-integrations/v1\n  bindings:\n    tasks:\n      provider: carina\n      root: ../tasks-operand\n"
	if err := os.WriteFile(filepath.Join(root, ".pyxis", "config.yaml"), []byte(input), 0600); err != nil {
		t.Fatal(err)
	}
	c, err := integrations.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	tasks, err := c.Tasks("", "", "")
	if err != nil || tasks.Root != filepath.Join(filepath.Dir(root), "tasks-operand") {
		t.Fatalf("relative Carina operand %#v %v", tasks, err)
	}
	serialized, err := c.Render()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(serialized), filepath.Dir(root)) {
		t.Fatal("portable Carina binding acquired a machine-specific root")
	}
}
