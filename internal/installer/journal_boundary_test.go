package installer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestJournalWriteRefusesFilesystemShapeConflicts(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "parent-file"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	j := newJournal(root)
	if err := j.writeFile("parent-file/child", []byte("x"), 0o644); err == nil {
		t.Fatal("journal write accepted a file as its parent directory")
	}

	targetDir := filepath.Join(root, "target-dir")
	if err := os.Mkdir(targetDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := j.writeFile("target-dir", []byte("replacement"), 0o644); err == nil {
		t.Fatal("journal write replaced a directory with a file")
	}
}

func TestJournalUnwindReportsRemovalAndGitRestorationFailures(t *testing.T) {
	root := t.TempDir()
	nonEmpty := filepath.Join(root, "not-a-file")
	if err := os.Mkdir(nonEmpty, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nonEmpty, "child"), []byte("foreign"), 0o644); err != nil {
		t.Fatal(err)
	}
	j := newJournal(root)
	j.entries = append(j.entries,
		journalEntry{kind: journalFile, path: nonEmpty},
		journalEntry{kind: journalConfig, key: "user.name"},
		journalEntry{kind: journalRemote, key: "origin"},
	)
	run := &initRun{j: j, rep: &Report{}, branch: "main"}
	rb := run.unwind(t.Context(), "boundary")
	if rb.Restored || len(rb.Errors) < 3 {
		t.Fatalf("unwind = %+v, want removal and git restoration errors", rb)
	}
	if len(rb.ForeignPreserved) == 0 {
		t.Fatalf("unwind did not name preserved foreign content: %+v", rb)
	}
}
