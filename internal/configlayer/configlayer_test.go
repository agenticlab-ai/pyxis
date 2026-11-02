package configlayer_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/configlayer"
)

func TestLoadOverlaysLeavesAndPreservesGlobalSiblings(t *testing.T) {
	root := t.TempDir()
	write(t, root, configlayer.GlobalPath, `schema_version: pyxis-config/v2
executor:
  routing:
    default: claude
  caps:
    worker_turns: 20
    parallel_width: 4
capability_paths:
  - cmd/
  - internal/
`)
	write(t, root, configlayer.LocalPath, `executor:
  routing:
    default: codex
  caps:
    parallel_width: 2
capability_paths:
  - local/
`)

	got, err := configlayer.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	text := string(got)
	for _, want := range []string{
		"schema_version: pyxis-config/v2",
		"default: codex",
		"worker_turns: 20",
		"parallel_width: 2",
		"  - local/",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("merged config missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "- cmd/") || strings.Contains(text, "- internal/") {
		t.Errorf("local sequence did not replace global sequence:\n%s", text)
	}
}

func TestLoadSupportsEitherFileBeingAbsent(t *testing.T) {
	root := t.TempDir()
	got, err := configlayer.Load(root)
	if err != nil || len(got) != 0 {
		t.Fatalf("no files: got %q, err %v", got, err)
	}

	write(t, root, configlayer.LocalPath, "executor:\n  routing:\n    default: codex\n")
	got, err = configlayer.Load(root)
	if err != nil || !strings.Contains(string(got), "default: codex") {
		t.Fatalf("local only: got %q, err %v", got, err)
	}
}

func TestLoadRejectsMalformedOrDuplicateLocalConfig(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want string
	}{
		{"odd indentation", "executor:\n   routing:\n", "indentation"},
		{"duplicate", "executor:\n  routing:\n  routing:\n", "duplicate mapping key"},
		{"orphan sequence", "- executor\n", "sequence has no mapping key"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			write(t, root, configlayer.LocalPath, tc.body)
			_, err := configlayer.Load(root)
			if err == nil || !strings.Contains(err.Error(), tc.want) || !strings.Contains(err.Error(), configlayer.LocalPath) {
				t.Fatalf("error = %v, want local path and %q", err, tc.want)
			}
		})
	}
}

func write(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
