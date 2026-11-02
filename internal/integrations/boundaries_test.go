package integrations

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRejectsMalformedNestedIntegrationContracts(t *testing.T) {
	for _, body := range []string{
		"  schema_version:\n    nested: no\n",
		"  connections: wrong\n",
		"  connections:\n    1bad:\n      provider: github\n",
		"  connections:\n    valid:\n      provider:\n        nested: github\n",
		"  bindings:\n    code_host:\n      provider: carina\n",
		"  bindings:\n    tasks:\n      unknown: value\n",
		"  bindings:\n    tasks:\n      provider:\n        nested: carina\n",
		"  bindings:\n    tasks:\n",
		"  bindings:\n    releases:\n      other:\n        provider: github\n",
		"  bindings:\n    releases:\n      source:\n        unknown: value\n",
		"  bindings:\n    releases:\n      source:\n        connection: absent\n        repository: o/r\n",
		"  bindings:\n    releases:\n      distribution:\n        provider: carina\n",
		"  ci:\n    mode:\n      nested: local\n",
		"  ci:\n    runner_label:\n      nested: runner\n",
		"  ci:\n    runner_label: \"line\\nbreak\"\n",
	} {
		text := "integrations:\n"
		if body != "  schema_version:\n    nested: no\n" {
			text += "  schema_version: " + SchemaVersion + "\n"
		}
		if _, err := Parse([]byte(text + body)); !errors.Is(err, ErrInvalid) {
			t.Errorf("nested malformed contract %q: %v", body, err)
		}
	}
}

func TestDirectBindingsRejectWrongCapabilityAndConflicts(t *testing.T) {
	c := Config{Connections: map[string]Connection{"host": {Provider: GitHub}}}
	for _, b := range []Binding{
		{Provider: Carina, Connection: "host"},
		{Provider: GitHub, Root: "invalid"},
		{Provider: Forgejo, Connection: "host", Repository: "o/r"},
		{Repository: "o/r"},
	} {
		if _, err := c.Resolve(b); !errors.Is(err, ErrInvalid) {
			t.Fatalf("invalid binding %#v: %v", b, err)
		}
	}
	c.Bindings = Bindings{CodeHost: Binding{Provider: Carina}, Artifacts: Binding{Provider: Carina}, Releases: ReleaseBindings{Source: Binding{Provider: Carina}}}
	if _, err := c.CodeHost("o/r"); !errors.Is(err, ErrInvalid) {
		t.Fatal("Carina became a code host")
	}
	if _, _, err := c.Artifacts(); !errors.Is(err, ErrInvalid) {
		t.Fatal("Carina became an artifact host")
	}
	if _, err := c.Release("source", "o/r"); !errors.Is(err, ErrInvalid) {
		t.Fatal("Carina became a release host")
	}
	if _, err := c.Release("other", "o/r"); !errors.Is(err, ErrInvalid) {
		t.Fatal("unknown release target accepted")
	}
	if _, err := c.Tasks("o/r", "other", ""); !errors.Is(err, ErrInvalid) {
		t.Fatal("unknown legacy tracker accepted")
	}
	if key := (ResolvedRepository{Provider: Carina, Root: "local/../tasks"}).Key(); key != "carina:tasks" {
		t.Fatalf("Carina identity %q", key)
	}
	if _, err := normalizeConnection(Connection{Provider: Forgejo, BaseURL: "https://host/base%2Fpath"}); err == nil {
		t.Fatal("encoded separator in server root accepted")
	}
	if _, err := normalizeConnection(Connection{Provider: Forgejo, BaseURL: "https://host", DraftPrefix: "bad\nprefix"}); err == nil {
		t.Fatal("multiline draft policy accepted")
	}
}

func TestRenderAndLoadRefuseInvalidProfiles(t *testing.T) {
	for _, c := range []Config{
		{SchemaVersion: SchemaVersion, Connections: map[string]Connection{"bad:name": {Provider: GitHub}}},
		{SchemaVersion: "unsupported"},
	} {
		if _, err := c.Render(); !errors.Is(err, ErrInvalid) {
			t.Fatalf("render emitted invalid contract: %v", err)
		}
	}
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".pyxis", "config.yaml"), 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(root); err == nil {
		t.Fatal("unreadable configuration was ignored")
	}
}

func TestRenderRoundTripsEveryCapabilityWithoutCredentials(t *testing.T) {
	c := Config{SchemaVersion: SchemaVersion, Connections: map[string]Connection{
		"source": {Provider: GitHub, TokenEnv: "TEST_RENDER_SOURCE"},
		"local":  {Provider: Forgejo, BaseURL: "https://forge.example/base", TokenEnv: "TEST_RENDER_LOCAL", DraftPrefix: "Draft:"},
	}, CI: CI{Mode: "both", RunnerLabel: "pyxis-linux"}}
	c.Bindings.Tasks = Binding{Provider: Carina, Root: "."}
	c.Bindings.CodeHost = Binding{Connection: "local", Repository: "owner/code"}
	c.Bindings.Releases.Source = Binding{Connection: "source", Repository: "owner/release-source"}
	c.Bindings.Releases.Distribution = Binding{Connection: "local", Repository: "owner/distribution"}
	c.Bindings.Artifacts = Binding{Connection: "local", Repository: "owner/pyxis-releases"}
	t.Setenv("TEST_RENDER_LOCAL", "token-canary-never-rendered")
	b, err := c.Render()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "token-canary-never-rendered") {
		t.Fatal("render serialized live credentials")
	}
	parsed, err := Parse(b)
	if err != nil || parsed.Bindings != c.Bindings || parsed.CI != c.CI {
		t.Fatalf("capability bindings failed round trip: %#v %v", parsed.Bindings, err)
	}
	again, err := c.Render()
	if err != nil || string(again) != string(b) {
		t.Fatal("integration rendering is nondeterministic")
	}
	if b, err := (Config{}).Render(); err != nil || len(b) != 0 {
		t.Fatal("legacy rendering emitted an integration block")
	}
}
