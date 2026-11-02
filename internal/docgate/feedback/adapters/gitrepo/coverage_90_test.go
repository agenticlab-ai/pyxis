package gitrepo

import (
	"path/filepath"
	"testing"
)

func TestCoverage90EditLandingEmptyAndInvalidCommit(t *testing.T) {
	r := Repo{Worktree: filepath.Join(t.TempDir(), "missing"), Ref: "HEAD"}
	if got, err := r.EditLanded(""); err != nil || got.Landed {
		t.Fatalf("EditLanded(empty) = %+v, %v", got, err)
	}
	if _, err := r.EditLanded("deadbeef"); err == nil {
		t.Fatal("EditLanded accepted an invalid workspace and commit")
	}
}

func TestCoverage90ChangedLineCountSkipsNonTextRows(t *testing.T) {
	if got := changedLineCount([]byte(" \n\t")); got != 0 {
		t.Fatalf("blank numstat = %d", got)
	}
	if got := changedLineCount([]byte("malformed\n-\t-\tbinary\n2\t3\ttext")); got != 5 {
		t.Fatalf("mixed numstat = %d, want 5", got)
	}
}
