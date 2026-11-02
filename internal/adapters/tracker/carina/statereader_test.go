package carinatracker

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/worktracker/localfs"
	"github.com/agenticlab-ai/pyxis/internal/worktracker/model"
)

// advancingClock is a deterministic clock stepping one second per call, so a
// fixture's create and its later transition land distinct timestamps
// (mirrors internal/worktracker/localfs's own test helper of the same name).
func advancingClock() func() time.Time {
	t := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	return func() time.Time {
		out := t
		t = t.Add(time.Second)
		return out
	}
}

// The conformance kit run: a real temp-dir store, built with the store's own
// Create/Transition calls (local-tracker fixtures), against the adapter's
// batch read (plan T2 acceptance criterion 1).
func TestStateReader_ConformanceKit(t *testing.T) {
	root := t.TempDir()
	store := localfs.New(root, advancingClock())
	ctx := context.Background()

	open, defects := store.Create(ctx, model.CreateSpec{Title: "still open"})
	mustNoDefects(t, defects)

	landed, defects := store.Create(ctx, model.CreateSpec{Title: "will land"})
	mustNoDefects(t, defects)
	landedItem, defects := store.Transition(ctx, landed.ID, model.StateClosed, model.CloseCompleted, "tester")
	mustNoDefects(t, defects)
	wantRevision := landedRevision(landedItem)
	if wantRevision == "" {
		t.Fatalf("fixture setup: landed item carries no revision id")
	}

	reader := NewStateReader(root)
	unknownRef := port.ProposalRef{Repo: root, Number: 9999} // no such item on disk

	cas := port.ReaderConformanceCase{
		Refs: []port.ProposalRef{
			{Repo: root, Number: open.ID},
			{Repo: root, Number: landed.ID},
			unknownRef,
		},
		Landed:         port.ProposalRef{Repo: root, Number: landed.ID},
		LandedRevision: wantRevision,
		Unknown:        unknownRef,
	}
	if err := port.CheckReaderConformance(ctx, reader, cas); err != nil {
		t.Fatal(err)
	}
}

// An open fixture reports its verdict-derived (status-signal-derived) check
// states, and comments strictly newer than the caller's cursor (acceptance
// criterion 2).
func TestStateReader_OpenItemChecksAndCommentsSinceCursor(t *testing.T) {
	root := t.TempDir()
	store := localfs.New(root, advancingClock())
	ctx := context.Background()

	w, defects := store.Create(ctx, model.CreateSpec{Title: "in flight"})
	mustNoDefects(t, defects)
	_, defects = store.AddSignal(ctx, w.ID, "verify-pending")
	mustNoDefects(t, defects)
	_, defects = store.Comment(ctx, w.ID, "author-a", "first note")
	mustNoDefects(t, defects)
	afterFirst := time.Now() // wall-clock cursor position between the two comments
	_, defects = store.Comment(ctx, w.ID, "author-b", "second note")
	mustNoDefects(t, defects)

	// Re-read to learn the first comment's actual (store-clock) timestamp, so
	// the cursor position is exact rather than approximate.
	got, defects := store.Get(ctx, w.ID)
	mustNoDefects(t, defects)
	var firstAt time.Time
	for _, e := range got.Log {
		if e.Kind == model.LogComment && e.Payload == "first note" {
			firstAt = e.At
		}
	}
	if firstAt.IsZero() {
		t.Fatalf("fixture setup: first comment not found in log: %+v", got.Log)
	}
	_ = afterFirst

	reader := NewStateReader(root)
	since := port.Cursor{Positions: map[port.CommentSurface]time.Time{port.SurfaceDiscussion: firstAt}}
	states, err := reader.States(ctx, []port.ProposalRef{{Repo: root, Number: w.ID}}, since)
	if err != nil {
		t.Fatalf("States: %v", err)
	}
	st := states[0]
	if st.Unknown != nil {
		t.Fatalf("open item reported Unknown: %+v", st)
	}
	if st.Landing != port.LandingOpen {
		t.Fatalf("Landing = %v, want open", st.Landing)
	}
	if len(st.Checks) != 1 || st.Checks[0].Name != "verify-pending" || st.Checks[0].State != port.CheckPending {
		t.Fatalf("Checks = %+v, want one pending verify-pending context", st.Checks)
	}
	if len(st.Comments) != 1 || st.Comments[0].Body != "second note" {
		t.Fatalf("Comments-since-cursor = %+v, want only the second note", st.Comments)
	}
}

// A corrupted record fixture yields a typed parse-defect unknown — never an
// empty or healthy-looking result (acceptance criterion 3).
func TestStateReader_CorruptedRecordYieldsParseDefect(t *testing.T) {
	root := t.TempDir()
	store := localfs.New(root, advancingClock())
	ctx := context.Background()
	w, defects := store.Create(ctx, model.CreateSpec{Title: "will be corrupted"})
	mustNoDefects(t, defects)

	if err := os.WriteFile(itemFilePath(root, w.ID), []byte("this is not a work item\n"), 0o644); err != nil {
		t.Fatalf("corrupt fixture: %v", err)
	}

	reader := NewStateReader(root)
	states, err := reader.States(ctx, []port.ProposalRef{{Repo: root, Number: w.ID}}, port.Cursor{})
	if err != nil {
		t.Fatalf("States: %v", err)
	}
	assertParseDefectUnknown(t, states[0])
}

// A ref naming an item that was never created also yields a typed
// parse-defect unknown, never an omission.
func TestStateReader_MissingRecordYieldsParseDefect(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "items"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	reader := NewStateReader(root)
	states, err := reader.States(context.Background(), []port.ProposalRef{{Repo: root, Number: 42}}, port.Cursor{})
	if err != nil {
		t.Fatalf("States: %v", err)
	}
	assertParseDefectUnknown(t, states[0])
}

func assertParseDefectUnknown(t *testing.T, st port.ProposalState) {
	t.Helper()
	if st.Unknown == nil || *st.Unknown != port.UnknownParseDefect {
		t.Fatalf("state = %+v, want a typed UnknownParseDefect", st)
	}
	if st.Landing != port.LandingUnknown || st.LandedRevision != "" || len(st.Checks) != 0 || len(st.Comments) != 0 {
		t.Fatalf("unknown state carries extra data: %+v", st)
	}
}

// A no-change scan performs no record parsing (acceptance criterion 4): the
// item-fetch seam must not be invoked a second time when the file has not
// changed since the last read.
func TestStateReader_NoChangeScanPerformsNoParsing(t *testing.T) {
	root := t.TempDir()
	store := localfs.New(root, advancingClock())
	ctx := context.Background()
	w, defects := store.Create(ctx, model.CreateSpec{Title: "steady"})
	mustNoDefects(t, defects)

	reader := NewStateReader(root)
	var calls int
	orig := reader.getItem
	reader.getItem = func(ctx context.Context, id int) (model.WorkItem, []schema.Defect) {
		calls++
		return orig(ctx, id)
	}

	refs := []port.ProposalRef{{Repo: root, Number: w.ID}}
	if _, err := reader.States(ctx, refs, port.Cursor{}); err != nil {
		t.Fatalf("first States: %v", err)
	}
	if calls != 1 {
		t.Fatalf("first read: getItem called %d times, want 1", calls)
	}

	if _, err := reader.States(ctx, refs, port.Cursor{}); err != nil {
		t.Fatalf("second States: %v", err)
	}
	if calls != 1 {
		t.Fatalf("no-change scan called getItem again: %d calls, want still 1", calls)
	}
}

// A changed file (a new comment appended) is re-parsed and its fresh state
// observed on the very next call.
func TestStateReader_ChangedFileIsReParsed(t *testing.T) {
	root := t.TempDir()
	store := localfs.New(root, advancingClock())
	ctx := context.Background()
	w, defects := store.Create(ctx, model.CreateSpec{Title: "will change"})
	mustNoDefects(t, defects)

	reader := NewStateReader(root)
	refs := []port.ProposalRef{{Repo: root, Number: w.ID}}
	states, err := reader.States(ctx, refs, port.Cursor{})
	if err != nil {
		t.Fatalf("first States: %v", err)
	}
	if len(states[0].Comments) != 0 {
		t.Fatalf("fixture setup: expected no comments yet, got %+v", states[0].Comments)
	}

	if _, defects := store.Comment(ctx, w.ID, "author", "a new note"); len(defects) > 0 {
		t.Fatalf("comment: %v", defects)
	}

	states, err = reader.States(ctx, refs, port.Cursor{})
	if err != nil {
		t.Fatalf("second States: %v", err)
	}
	if len(states[0].Comments) != 1 || states[0].Comments[0].Body != "a new note" {
		t.Fatalf("post-change read = %+v, want the new comment reflected", states[0].Comments)
	}
}

func TestStateReader_AdviceCadenceIsOneSecond(t *testing.T) {
	if AdviceCadence != time.Second {
		t.Fatalf("AdviceCadence = %v, want 1s", AdviceCadence)
	}
}

func TestStateReader_ContextCancelledReturnsTopLevelError(t *testing.T) {
	reader := NewStateReader(t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := reader.States(ctx, []port.ProposalRef{{Number: 1}}, port.Cursor{})
	if err == nil {
		t.Fatal("want a transport-fatal error on a cancelled context")
	}
}

// A per-item permission error (the file stats fine but cannot be read) is a
// filesystem error like any other and resolves to a typed unknown, never a
// crash or a healthy-looking result. Skipped when running as root, which
// ignores file-mode read permission.
func TestStateReader_UnreadableItemYieldsParseDefect(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX file-mode permissions only")
	}
	if os.Geteuid() == 0 {
		t.Skip("root ignores file-mode read permission")
	}
	root := t.TempDir()
	store := localfs.New(root, advancingClock())
	ctx := context.Background()
	w, defects := store.Create(ctx, model.CreateSpec{Title: "locked down"})
	mustNoDefects(t, defects)
	if err := os.Chmod(itemFilePath(root, w.ID), 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	defer os.Chmod(itemFilePath(root, w.ID), 0o644) // restore so TempDir cleanup can remove it

	reader := NewStateReader(root)
	states, err := reader.States(ctx, []port.ProposalRef{{Repo: root, Number: w.ID}}, port.Cursor{})
	if err != nil {
		t.Fatalf("States: %v", err)
	}
	assertParseDefectUnknown(t, states[0])
}

func mustNoDefects(t *testing.T, defects []schema.Defect) {
	t.Helper()
	if len(defects) > 0 {
		t.Fatalf("unexpected defects: %v", defects)
	}
}
