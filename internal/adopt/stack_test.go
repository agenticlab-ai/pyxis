package adopt_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/adopt"
)

// writeMarker creates an empty marker file at root/name.
func writeMarker(t *testing.T, root, name string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, name), []byte(""), 0o644); err != nil {
		t.Fatalf("write marker %s: %v", name, err)
	}
}

// TestDetectStackGo proves a Go fixture — a bare go.mod — is named "go"
// with the fixed gate-action set and no fabricated command (T4 acceptance
// #1; prd:existing-repo-onboarding/FR-7).
func TestDetectStackGo(t *testing.T) {
	root := t.TempDir()
	writeMarker(t, root, "go.mod")

	got := adopt.DetectStack(root)
	want := adopt.Stack{
		Detected:    []string{"go"},
		GateActions: []string{"executor.gate.build", "executor.gate.test", "executor.gate.lint"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("DetectStack(go fixture) = %+v, want %+v", got, want)
	}
}

// TestDetectStackNonGo proves a non-Go fixture — a bare package.json — is
// named "node" with the same fixed gate-action set, no command fabricated
// (T4 acceptance #1).
func TestDetectStackNonGo(t *testing.T) {
	root := t.TempDir()
	writeMarker(t, root, "package.json")

	got := adopt.DetectStack(root)
	want := adopt.Stack{
		Detected:    []string{"node"},
		GateActions: []string{"executor.gate.build", "executor.gate.test", "executor.gate.lint"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("DetectStack(node fixture) = %+v, want %+v", got, want)
	}
}

// TestDetectStackPython proves any of the python marker files — none of
// which is a go.mod or package.json — names the stack "python".
func TestDetectStackPython(t *testing.T) {
	for _, marker := range []string{"requirements.txt", "pyproject.toml", "setup.py"} {
		t.Run(marker, func(t *testing.T) {
			root := t.TempDir()
			writeMarker(t, root, marker)

			got := adopt.DetectStack(root)
			if !reflect.DeepEqual(got.Detected, []string{"python"}) {
				t.Errorf("Detected = %v, want [python]", got.Detected)
			}
		})
	}
}

// TestDetectStackUnknown proves a repository no marker matches degrades to
// the zero Stack — never an error — and that embedding it in a plan renders
// the single unknown-stack line (T4 acceptance #2;
// prd:existing-repo-onboarding/UB-3).
func TestDetectStackUnknown(t *testing.T) {
	root := t.TempDir()

	got := adopt.DetectStack(root)
	if len(got.Detected) != 0 || len(got.GateActions) != 0 {
		t.Fatalf("DetectStack(empty fixture) = %+v, want an empty Stack", got)
	}

	p := adopt.Plan{SchemaVersion: adopt.SchemaVersion, Status: adopt.StatusDraft, Stack: got}
	rendered := p.Render()
	const unknownLine = "- **Detected:** unknown — set your gate commands\n"
	if !strings.Contains(rendered, unknownLine) {
		t.Errorf("render missing the unknown-stack line; got:\n%s", rendered)
	}
}

// TestDetectStackPolyglot proves every matching stack is named, not just a
// dominant one (design-brainstorm:existing-repo-onboarding-tech-design#D-5).
func TestDetectStackPolyglot(t *testing.T) {
	root := t.TempDir()
	writeMarker(t, root, "go.mod")
	writeMarker(t, root, "package.json")

	got := adopt.DetectStack(root)
	want := []string{"go", "node"}
	if !reflect.DeepEqual(got.Detected, want) {
		t.Errorf("Detected = %v, want %v", got.Detected, want)
	}
}
