package feedback

import (
	"os"
	"path/filepath"
	"testing"
)

func validCoverageComment() CapturedComment {
	return CapturedComment{
		SchemaVersion: CapturedCommentVersion,
		Comment:       "review comment",
		Author:        "@reviewer",
		Timestamp:     "2026-09-01T00:00:00Z",
		SubjectAnchor: "subject#section",
		SourceID:      "conversation",
	}
}

func TestCoverage90WriterValidationAndSequenceScan(t *testing.T) {
	comment := validCoverageComment()
	comment.Comment = " "
	if _, err := RenderEntry("001", comment); err == nil {
		t.Fatal("empty comment was accepted")
	}
	comment = validCoverageComment()
	comment.Author = " "
	if _, err := RenderEntry("001", comment); err == nil {
		t.Fatal("empty author was accepted")
	}
	comment = validCoverageComment()
	comment.EditCommit = "abc123"
	if body, err := RenderEntry("001", comment); err != nil || len(body) == 0 {
		t.Fatalf("entry with edit commit = %q, %v", body, err)
	}

	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "ignored-dir"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "not-an-entry.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if got, err := nextSeq(dir); err != nil || got != "001" {
		t.Fatalf("nextSeq = %q, %v", got, err)
	}
}

func TestCoverage90DispositionOpenAndTableBoundaries(t *testing.T) {
	if _, err := Observe(nil, nil); err == nil {
		t.Fatal("Observe accepted a nil repo")
	}
	if disposed, err := ApplyTransitions(t.TempDir(), []Transition{{Flag: FlagPending}}); err != nil || len(disposed) != 0 {
		t.Fatalf("ApplyTransitions(open) = %v, %v", disposed, err)
	}
	if _, err := RenderDisposition(Disposition{Disposition: "applied", Outcome: "bad|outcome", ResolvedBy: "event"}); err == nil {
		t.Fatal("table-breaking disposition was accepted")
	}
}
