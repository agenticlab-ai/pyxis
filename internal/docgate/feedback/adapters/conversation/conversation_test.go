package conversation_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/feedback"
	"github.com/agenticlab-ai/pyxis/internal/docgate/feedback/adapters/conversation"
)

type failingSource struct{}

func (failingSource) Comments(context.Context) ([]feedback.CapturedComment, error) {
	return nil, errors.New("source unavailable")
}

func ann() conversation.ReviewComment {
	return conversation.ReviewComment{
		Comment:       "Tighten §3 to name the concrete artifact.",
		Author:        "@reviewer-one",
		Timestamp:     "2026-07-13T09:30:00Z",
		SubjectAnchor: "review-feedback-capture#3.1",
	}
}

// The adapter maps a confirmed operator comment to exactly one medium-neutral
// CapturedComment carrying every field, stamped with the pinned schema version
// and the conversation source stem (§3.3).
func TestSourceYieldsOneCapturedComment(t *testing.T) {
	got, err := conversation.Source{Comment: ann()}.Comments(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("yielded %d comments, want 1", len(got))
	}
	c := got[0]
	if c.SchemaVersion != feedback.CapturedCommentVersion {
		t.Errorf("schema = %q, want %q", c.SchemaVersion, feedback.CapturedCommentVersion)
	}
	if c.SourceID != conversation.SourceID {
		t.Errorf("source = %q, want %q", c.SourceID, conversation.SourceID)
	}
	if c.Comment != ann().Comment || c.Author != ann().Author ||
		c.Timestamp != ann().Timestamp || c.SubjectAnchor != ann().SubjectAnchor {
		t.Errorf("field drift: %+v", c)
	}
}

func TestCaptureReportsSourceAndQueueFailuresWithoutLosingOutcome(t *testing.T) {
	if _, err := conversation.Capture(t.Context(), feedback.NewWriter(t.TempDir()), t.TempDir(), failingSource{}); err == nil {
		t.Fatal("source failure was accepted")
	}
	blockedRoot := filepath.Join(t.TempDir(), "ledger-file")
	if err := os.WriteFile(blockedRoot, []byte("blocked"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := conversation.Capture(t.Context(), feedback.NewWriter(blockedRoot), "", conversation.Source{Comment: ann()})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Outcomes) != 1 || res.Outcomes[0].Err == nil || res.Outcomes[0].Written != "" || res.Outcomes[0].Queued != "" {
		t.Fatalf("double-failure outcome = %+v", res.Outcomes)
	}
}

// A-2 happy path: a confirmed comment produces exactly one ledger entry
// under its subject directory, folding cleanly with all fields (FR-1).
func TestCaptureWritesOneEntryWithAllFields(t *testing.T) {
	root := t.TempDir()
	queue := filepath.Join(t.TempDir(), "queue")

	res, err := conversation.Capture(context.Background(), feedback.NewWriter(root), queue, conversation.Source{Comment: ann()})
	if err != nil {
		t.Fatalf("capture: %v", err)
	}
	if len(res.Outcomes) != 1 {
		t.Fatalf("%d outcomes, want 1", len(res.Outcomes))
	}
	o := res.Outcomes[0]
	if o.Written == "" || o.Queued != "" || o.Err != nil {
		t.Fatalf("expected a written entry, got %+v", o)
	}
	// Nothing was queued on the happy path.
	if entries, _ := os.ReadDir(queue); len(entries) != 0 {
		t.Fatalf("happy path queued %d comments", len(entries))
	}

	// Exactly one entry file under the subject dir, and it folds with all fields.
	dir := filepath.Join(root, "review-feedback-capture")
	names := mdNames(t, dir)
	if strings.Join(names, ",") != "001-conversation.md" {
		t.Fatalf("entry files = %v, want [001-conversation.md]", names)
	}
	fr := feedback.Fold("review-feedback-capture", readSubject(t, dir))
	if len(fr.Defects) != 0 {
		t.Fatalf("captured entry has fold defects: %v", fr.Defects)
	}
	if len(fr.Entries) != 1 {
		t.Fatalf("folded %d entries, want 1", len(fr.Entries))
	}
	e := fr.Entries[0]
	if e.Author != ann().Author || e.Timestamp != ann().Timestamp ||
		e.SubjectAnchor != ann().SubjectAnchor || e.Source != conversation.SourceID ||
		e.Comment != ann().Comment {
		t.Fatalf("captured entry missing/incorrect fields: %+v", e)
	}
	if e.Disposition != nil {
		t.Fatalf("capture must write no disposition: %+v", e.Disposition)
	}
}

// A-2 / UB-1: a simulated ledger-write failure loses no comment and blocks no
// review. Rooting the ledger at a regular file forces the writer's MkdirAll to
// fail; Capture must then spool the comment to the queue and return no blocking
// error — the comment survives for a later retry.
func TestCaptureFailsOpenAndQueuesOnWriteFailure(t *testing.T) {
	tmp := t.TempDir()
	// A regular file where the ledger root must be a directory → every write fails.
	fileRoot := filepath.Join(tmp, "not-a-dir")
	if err := os.WriteFile(fileRoot, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	queue := filepath.Join(tmp, "queue")

	res, err := conversation.Capture(context.Background(), feedback.NewWriter(fileRoot), queue, conversation.Source{Comment: ann()})
	if err != nil {
		t.Fatalf("capture must not surface the write failure as a blocking error: %v", err)
	}
	if len(res.Outcomes) != 1 {
		t.Fatalf("%d outcomes, want 1", len(res.Outcomes))
	}
	o := res.Outcomes[0]
	if o.Written != "" {
		t.Fatalf("nothing should have been written: %q", o.Written)
	}
	if o.Err != nil {
		t.Fatalf("the comment was neither written nor queued (lost): %v", o.Err)
	}
	if o.Queued == "" {
		t.Fatal("the comment was not queued — a comment was lost (UB-1 violated)")
	}

	// The queued file exists and round-trips the verbatim comment + all fields.
	b, rerr := os.ReadFile(o.Queued)
	if rerr != nil {
		t.Fatalf("read queued comment: %v", rerr)
	}
	for _, want := range []string{ann().Comment, ann().Author, ann().Timestamp, ann().SubjectAnchor, conversation.SourceID, feedback.CapturedCommentVersion} {
		if !strings.Contains(string(b), want) {
			t.Fatalf("queued comment is missing %q:\n%s", want, b)
		}
	}
}

// Queueing is idempotent on comment identity (UB-4): the same comment spooled
// twice collides to one content-addressed file, never a duplicate.
func TestQueueIsIdempotentOnIdentity(t *testing.T) {
	tmp := t.TempDir()
	fileRoot := filepath.Join(tmp, "not-a-dir")
	if err := os.WriteFile(fileRoot, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	queue := filepath.Join(tmp, "queue")
	w := feedback.NewWriter(fileRoot)

	for i := range 2 {
		if _, err := conversation.Capture(context.Background(), w, queue, conversation.Source{Comment: ann()}); err != nil {
			t.Fatalf("capture %d: %v", i, err)
		}
	}
	if entries, _ := os.ReadDir(queue); len(entries) != 1 {
		t.Fatalf("the same comment queued twice produced %d files, want 1 (UB-4)", len(entries))
	}
}

func mdNames(t *testing.T, dir string) []string {
	t.Helper()
	items, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	var names []string
	for _, it := range items {
		if !it.IsDir() && strings.HasSuffix(it.Name(), ".md") {
			names = append(names, it.Name())
		}
	}
	return names
}

func readSubject(t *testing.T, dir string) []feedback.Entry {
	t.Helper()
	var entries []feedback.Entry
	for _, name := range mdNames(t, dir) {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		entries = append(entries, feedback.Entry{Name: name, Bytes: b})
	}
	return entries
}
