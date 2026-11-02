package adopt

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/payload"
)

func TestWorkflowCollisionRejectsInvalidIntegrationProfile(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".pyxis"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".pyxis/config.yaml"), []byte("integrations:\n  unknown: true\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveWorkflowEntries(root, []Entry{{Path: payload.ForgejoWorkflowTarget, Class: ClassGenerated}}); err == nil {
		t.Fatal("adoption selected a workflow despite invalid integration config")
	}
}

func TestUndoUnreadableGeneratedWorkflowPreservesThePath(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "workflow.yml")
	if err := os.Symlink("workflow.yml", path); err != nil {
		t.Fatal(err)
	}
	var report UndoReport
	if err := report.removeIfPristine(root, "missing.yml", []byte("desired")); err != nil {
		t.Fatal("already-absent workflow was not idempotent", err)
	}
	if err := report.removeIfPristine(root, "workflow.yml", []byte("desired")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(path); err != nil {
		t.Fatal("unreadable workflow was removed")
	}
	if len(report.Removed) != 0 {
		t.Fatalf("incorrect removal report: %+v", report)
	}
	if len(report.Preserved) != 1 || report.Preserved[0] != "workflow.yml" {
		t.Fatalf("unreadable workflow not preserved: %+v", report)
	}
}
