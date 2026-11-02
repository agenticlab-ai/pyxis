package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	forgejotracker "github.com/agenticlab-ai/pyxis/internal/adapters/tracker/forgejo"
)

func TestProjectSelectsConfiguredForgejoWithoutGitHubCLI(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".pyxis"), 0o755); err != nil {
		t.Fatal(err)
	}
	config := `integrations:
  schema_version: pyxis-integrations/v1
  connections:
    local:
      provider: forgejo
      base_url: http://127.0.0.1:3000
    alternate:
      provider: forgejo
      base_url: http://127.0.0.1:3001/base
  bindings:
    tasks:
      connection: local
      repository: o/r
`
	if err := os.WriteFile(filepath.Join(root, ".pyxis/config.yaml"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", t.TempDir())
	adapter, err := projectTracker(root, "gh", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	f, ok := adapter.(*forgejotracker.Forgejo)
	if !ok {
		t.Fatalf("adapter = %T", adapter)
	}
	if f.SourceRef(1) != "http://127.0.0.1:3000/o/r/issues/1" {
		t.Fatal("configured repository not used")
	}
	adapter, err = projectTracker(root, "gh", "", "alternate", "other/repo")
	if err != nil {
		t.Fatal(err)
	}
	if got := adapter.(*forgejotracker.Forgejo).SourceRef(1); got != "http://127.0.0.1:3001/base/other/repo/issues/1" {
		t.Fatalf("override=%q", got)
	}
	if _, err := projectTracker(root, "gh", "", "missing", ""); err == nil {
		t.Fatal("missing connection accepted")
	}
	if _, err := projectTracker(root, "mystery", "", "", ""); err == nil {
		t.Fatal("unknown tracker accepted")
	}
	if _, err := projectTracker(root, "gh", "", "", "not-a-coordinate"); err == nil {
		t.Fatal("invalid repository accepted")
	}
	if err := os.WriteFile(filepath.Join(root, ".pyxis/config.yaml"), []byte(strings.Replace(config, "schema_version:", "unknown_key:", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := projectTracker(root, "gh", "", "", ""); err == nil {
		t.Fatal("invalid configuration accepted")
	}
}

func TestProjectRefusesUnavailableConfiguredTaskProvider(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.test/unavailable-task-provider\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".pyxis"), 0o755); err != nil {
		t.Fatal(err)
	}
	config := `integrations:
  schema_version: pyxis-integrations/v1
  bindings:
    tasks:
      provider: carina
`
	if err := os.WriteFile(filepath.Join(root, ".pyxis/config.yaml"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", t.TempDir())
	t.Chdir(root)
	code, stdout, stderr := captureProcessOutput(t, func() int {
		return run([]string{"project", "docs/plans/feature"})
	})
	if code != 2 || stdout != "" || !strings.Contains(stderr, "carina not on PATH") || !strings.Contains(stderr, "install carina") {
		t.Fatalf("unavailable configured task provider: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}
