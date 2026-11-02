package integrations_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/integrations"
)

const localProfile = `integrations:
  schema_version: pyxis-integrations/v1
  bindings:
    code_host:
      provider: local-git
      root: .origins/code.git
    tasks:
      provider: carina
  ci:
    mode: local
`

func TestLocalGitResolvesConfiguredOriginWithoutHostedIdentity(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".pyxis"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".pyxis/config.yaml"), []byte(localProfile), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := integrations.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	host, err := cfg.CodeHost("ignored/hosted")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, ".origins/code.git")
	if host.Provider != integrations.LocalGit || host.Root != want || host.Repository != "" || host.BaseURL != "" || host.TokenEnv != "" || host.Key() != integrations.LocalGit+":"+want {
		t.Fatalf("local binding mixed origin and hosted identity: %+v", host)
	}
	tasks, err := cfg.Tasks("", "", root)
	if err != nil || tasks.Provider != integrations.Carina {
		t.Fatalf("independent Carina binding: %+v, %v", tasks, err)
	}
}

func TestLocalGitClosedBindingAndCapabilityScope(t *testing.T) {
	for name, profile := range map[string]string{
		"missing origin": strings.Replace(localProfile, "      root: .origins/code.git\n", "", 1),
		"connection":     strings.Replace(localProfile, "      root:", "      connection: remote\n      root:", 1),
		"repository":     strings.Replace(localProfile, "      root:", "      repository: o/r\n      root:", 1),
		"URL origin":     strings.Replace(localProfile, ".origins/code.git", "https://example.test/code.git", 1),
		"unknown key":    strings.Replace(localProfile, "      root:", "      origin_id: claimed\n      root:", 1),
		"tracker":        strings.Replace(localProfile, "    tasks:\n      provider: carina", "    tasks:\n      provider: local-git\n      root: .origins/code.git", 1),
		"artifacts":      strings.Replace(localProfile, "    tasks:\n      provider: carina", "    artifacts:\n      provider: local-git\n      root: .origins/code.git", 1),
		"release":        strings.Replace(localProfile, "    tasks:\n      provider: carina", "    releases:\n      source:\n        provider: local-git\n        root: .origins/code.git", 1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := integrations.Parse([]byte(profile)); !errors.Is(err, integrations.ErrInvalid) {
				t.Fatalf("invalid local binding accepted: %v", err)
			}
		})
	}
	for _, capability := range []string{"tasks", "artifacts", "release"} {
		t.Run("direct "+capability, func(t *testing.T) {
			cfg := integrations.Config{}
			binding := integrations.Binding{Provider: integrations.LocalGit, Root: ".origins/code.git"}
			var err error
			switch capability {
			case "tasks":
				cfg.Bindings.Tasks = binding
				_, err = cfg.Tasks("", "", "")
			case "artifacts":
				cfg.Bindings.Artifacts = binding
				_, _, err = cfg.Artifacts()
			case "release":
				cfg.Bindings.Releases.Source = binding
				_, err = cfg.Release("source", "")
			}
			if !errors.Is(err, integrations.ErrInvalid) {
				t.Fatalf("local Git admitted for %s: %v", capability, err)
			}
		})
	}
}
