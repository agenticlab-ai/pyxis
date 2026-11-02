package tasksession

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

func TestFeedbackNativeIDsSurviveReloadAndDisambiguateTimestamps(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions.json")
	store, err := NewFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	c, err := New(store, "forgejo:https://forgejo.test:owner/repo")
	if err != nil {
		t.Fatal(err)
	}
	task := port.Task{ID: "T1", Number: 1, SourceRef: "https://forgejo.test/owner/repo/issues/1"}
	at := time.Now().UTC()
	for _, id := range []string{"comment-1", "comment-2"} {
		if err := c.RecordFeedbackComments(t.Context(), task, at, []port.Comment{{NativeID: id, CreatedAt: at}}); err != nil {
			t.Fatal(err)
		}
	}
	store, err = NewFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	c, err = New(store, "forgejo:https://forgejo.test:owner/repo")
	if err != nil {
		t.Fatal(err)
	}
	seen, err := c.FeedbackSeen(t.Context(), task)
	if err != nil || len(seen) != 2 || !seen["comment-1"] || !seen["comment-2"] {
		t.Fatalf("seen=%v err=%v", seen, err)
	}
	if err := c.RecordFeedbackComments(t.Context(), task, at.Add(time.Second), []port.Comment{{NativeID: "comment-3", CreatedAt: at.Add(time.Second)}}); err != nil {
		t.Fatal(err)
	}
	seen, err = c.FeedbackSeen(t.Context(), task)
	if err != nil || len(seen) != 1 || !seen["comment-3"] {
		t.Fatalf("advanced=%v err=%v", seen, err)
	}
}
