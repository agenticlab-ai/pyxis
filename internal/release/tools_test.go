package release_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/release"
)

// The shipped-set rule, against a fixture command tree: every cmd/* dir
// ships EXCEPT the build-only release tool, toolchain-ignored dirs, and
// non-dirs (AP-27 — the scan is the list).
func TestShippedToolsScan(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{"toolb", "toola", release.BuildOnlyTool, "testdata", ".hidden", "_ignored"} {
		if err := os.MkdirAll(filepath.Join(root, "cmd", d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "cmd", "README.md"), []byte("not a tool\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tools, err := release.ShippedTools(root)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if want := []string{"toola", "toolb"}; !slices.Equal(tools, want) {
		t.Fatalf("shipped set = %v, want %v", tools, want)
	}
}

// Zero shipped tools is a defect, not an empty release.
func TestShippedToolsRefusesEmptyScan(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "cmd", release.BuildOnlyTool), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := release.ShippedTools(root); err == nil || !strings.Contains(err.Error(), "zero shipped tools") {
		t.Fatalf("scan of a build-tool-only tree = %v, want the zero-tools refusal", err)
	}
}

// The REAL command tree scans to the current engine set — and never to the
// build tool. This is the workflow's matrix input; it grows by mkdir, never
// by editing a list.
func TestShippedToolsRealTree(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	tools, err := release.ShippedTools(root)
	if err != nil {
		t.Fatalf("scan real tree: %v", err)
	}
	if slices.Contains(tools, release.BuildOnlyTool) {
		t.Fatalf("the build-only tool is in the shipped set: %v", tools)
	}
	for _, must := range []string{"docgate", "codegate", "plangate"} {
		if !slices.Contains(tools, must) {
			t.Errorf("real scan is missing %s: %v", must, tools)
		}
	}
	if !slices.IsSorted(tools) {
		t.Errorf("shipped set is not sorted: %v", tools)
	}
}
