package tasksession

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

func testContinuity(t *testing.T) (*Continuity, *FileStore, *time.Time, string) {
	t.Helper()
	store, err := NewFileStore(filepath.Join(t.TempDir(), "pyxis", "task-sessions.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	ids := 0
	c, err := New(store, "o/r", WithClock(func() time.Time { return now }), WithIDMinter(func() (string, error) {
		ids++
		return "sess-" + strings.Repeat("0", 3) + string(rune('0'+ids)), nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	worktree := t.TempDir()
	return c, store, &now, worktree
}

// A first launch mints and persists a session BEFORE the process exists; a
// later launch of the same task, harness and worktree resumes it; the
// finished launch's reported id supersedes the pre-assigned one.
func TestContinuity_StartThenResumeAndRecordReportedID(t *testing.T) {
	c, store, _, wt := testContinuity(t)
	ctx := context.Background()
	task := port.Task{ID: "T1", Number: 11}

	spec, err := c.Session(ctx, task, "claude", wt)
	if err != nil || spec.Resume || spec.ID != "sess-0001" {
		t.Fatalf("first session = %+v, %v", spec, err)
	}
	rec, ok, err := store.Lookup(ctx, "o/r", "item:11", "T1")
	if err != nil || !ok || rec.SessionID != "sess-0001" || rec.Launches != 0 || rec.Status != StatusActive || rec.Harness != "claude" {
		t.Fatalf("provisional record = %+v %v %v (must exist before the launch)", rec, ok, err)
	}
	if err := c.Record(ctx, task, "claude", wt, spec, port.RunResult{SessionID: "sess-0001", Turns: 3}); err != nil {
		t.Fatal(err)
	}
	again, err := c.Session(ctx, task, "claude", wt)
	if err != nil || !again.Resume || again.ID != "sess-0001" {
		t.Fatalf("second session = %+v, %v, want a resume", again, err)
	}
	// A harness that mints its own id (codex) reports it; the record adopts it.
	if err := c.Record(ctx, task, "claude", wt, again, port.RunResult{SessionID: "thread-9"}); err != nil {
		t.Fatal(err)
	}
	rec, _, _ = store.Lookup(ctx, "o/r", "item:11", "T1")
	if rec.SessionID != "thread-9" || rec.Launches != 2 {
		t.Fatalf("record after reported id = %+v", rec)
	}
	third, _ := c.Session(ctx, task, "claude", wt)
	if !third.Resume || third.ID != "thread-9" {
		t.Fatalf("third session = %+v", third)
	}
}

// A pre-assigned identity whose launch never ran is minted again, not resumed:
// the harness never created that conversation.
func TestContinuity_NeverResumesASessionNoLaunchRanUnder(t *testing.T) {
	c, _, _, wt := testContinuity(t)
	ctx := context.Background()
	task := port.Task{ID: "T3"}
	first, _ := c.Session(ctx, task, "claude", wt)
	// No Record: the process failed to start.
	second, err := c.Session(ctx, task, "claude", wt)
	if err != nil || second.Resume || second.ID == first.ID {
		t.Fatalf("second session = %+v (first %+v), want a fresh identity", second, first)
	}
	_ = c.Record(ctx, task, "claude", wt, second, port.RunResult{SessionID: second.ID})
	if third, _ := c.Session(ctx, task, "claude", wt); !third.Resume || third.ID != second.ID {
		t.Fatalf("after a real launch the session must resume: %+v", third)
	}
}

func TestContinuity_ResumableHarness(t *testing.T) {
	c, _, _, wt := testContinuity(t)
	ctx := context.Background()
	task := port.Task{ID: "T-routing", Number: 17}

	if harness, ok, err := c.ResumableHarness(ctx, task); err != nil || ok || harness != "" {
		t.Fatalf("missing session hint = %q, %v, %v", harness, ok, err)
	}
	spec, err := c.Session(ctx, task, "claude", wt)
	if err != nil {
		t.Fatal(err)
	}
	if harness, ok, err := c.ResumableHarness(ctx, task); err != nil || ok || harness != "" {
		t.Fatalf("provisional session hint = %q, %v, %v", harness, ok, err)
	}
	if err := c.Record(ctx, task, "claude", wt, spec, port.RunResult{SessionID: spec.ID}); err != nil {
		t.Fatal(err)
	}
	if harness, ok, err := c.ResumableHarness(ctx, task); err != nil || !ok || harness != "claude" {
		t.Fatalf("resumable session hint = %q, %v, %v", harness, ok, err)
	}
	if err := os.RemoveAll(wt); err != nil {
		t.Fatal(err)
	}
	if harness, ok, err := c.ResumableHarness(ctx, task); err != nil || ok || harness != "" {
		t.Fatalf("stale session hint = %q, %v, %v", harness, ok, err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, _, err := c.ResumableHarness(cancelled, task); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled lookup error = %v, want context.Canceled", err)
	}
}

// A session is never resumed across a harness change, into another worktree,
// into a worktree that no longer exists, or after retirement; the feedback
// cursor survives the fresh start.
func TestContinuity_FreshOnHarnessWorktreeOrRetirementChange(t *testing.T) {
	c, store, _, wt := testContinuity(t)
	ctx := context.Background()
	task := port.Task{ID: "T1"}
	first, _ := c.Session(ctx, task, "claude", wt)
	_ = c.Record(ctx, task, "claude", wt, first, port.RunResult{SessionID: first.ID})
	_ = c.RecordFeedback(ctx, task, time.Date(2026, 9, 1, 13, 0, 0, 0, time.UTC))

	if s, _ := c.Session(ctx, task, "codex", wt); s.Resume {
		t.Fatalf("a different harness must start fresh: %+v", s)
	}
	other := t.TempDir()
	if s, _ := c.Session(ctx, task, "codex", other); s.Resume {
		t.Fatalf("a different worktree must start fresh: %+v", s)
	}
	last, rounds, _ := c.LastFeedback(ctx, task)
	if rounds != 1 || last.IsZero() {
		t.Fatalf("feedback cursor lost across a fresh session: %v %d", last, rounds)
	}
	// Back on the original harness+worktree the record is now the fresh codex
	// one, so claude starts fresh as well (one session per task at a time).
	s, _ := c.Session(ctx, task, "claude", wt)
	if s.Resume {
		t.Fatalf("after another harness took over, claude must start fresh: %+v", s)
	}
	_ = c.Record(ctx, task, "claude", wt, s, port.RunResult{SessionID: s.ID})
	if r, _ := c.Session(ctx, task, "claude", wt); !r.Resume {
		t.Fatal("same harness+worktree must resume")
	}
	if err := os.RemoveAll(wt); err != nil {
		t.Fatal(err)
	}
	if r, _ := c.Session(ctx, task, "claude", wt); r.Resume {
		t.Fatal("a vanished worktree must not be resumed into")
	}
	wt2 := t.TempDir()
	s2, _ := c.Session(ctx, task, "claude", wt2)
	_ = c.Record(ctx, task, "claude", wt2, s2, port.RunResult{SessionID: s2.ID})
	if err := c.Retire(ctx, task, "proposal landed"); err != nil {
		t.Fatal(err)
	}
	rec, _, _ := store.Lookup(ctx, "o/r", "", "T1")
	if rec.Status != StatusRetired || rec.RetiredReason != "proposal landed" {
		t.Fatalf("retired record = %+v", rec)
	}
	if r, _ := c.Session(ctx, task, "claude", wt2); r.Resume {
		t.Fatal("a retired session must not be resumed")
	}
}

// The feedback cursor advances to the newest serviced comment at admission and
// counts rounds; an older 'latest' never moves it backwards.
func TestContinuity_FeedbackCursor(t *testing.T) {
	c, _, _, _ := testContinuity(t)
	ctx := context.Background()
	task := port.Task{ID: "T2", Number: 2}
	last, rounds, err := c.LastFeedback(ctx, task)
	if err != nil || !last.IsZero() || rounds != 0 {
		t.Fatalf("empty cursor = %v %d %v", last, rounds, err)
	}
	t1 := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	if err := c.RecordFeedback(ctx, task, t1); err != nil {
		t.Fatal(err)
	}
	if err := c.RecordFeedback(ctx, task, t1.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	last, rounds, _ = c.LastFeedback(ctx, task)
	if !last.Equal(t1) || rounds != 2 {
		t.Fatalf("cursor = %v %d, want %v 2", last, rounds, t1)
	}
}

// The ledger is append-only and latest-wins; a malformed line fails closed.
func TestFileStore_LatestWinsAndMalformedLineFailsClosed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ledger.jsonl")
	store, _ := NewFileStore(path)
	ctx := context.Background()
	base := Record{SchemaVersion: Schema, Repository: "o/r", TaskID: "T1", Harness: "claude", SessionID: "a", Worktree: "/w", Status: StatusActive}
	if err := store.Put(ctx, base); err != nil {
		t.Fatal(err)
	}
	next := base
	next.SessionID, next.Launches = "b", 2
	if err := store.Put(ctx, next); err != nil {
		t.Fatal(err)
	}
	other := base
	other.TaskID = "T2"
	if err := store.Put(ctx, other); err != nil {
		t.Fatal(err)
	}
	got, ok, err := store.Lookup(ctx, "o/r", "", "T1")
	if err != nil || !ok || got.SessionID != "b" || got.Launches != 2 {
		t.Fatalf("lookup = %+v %v %v", got, ok, err)
	}
	if _, ok, _ := store.Lookup(ctx, "o/r", "", "T9"); ok {
		t.Fatal("unknown task must not resolve")
	}
	if err := store.Put(ctx, Record{SchemaVersion: "task-session/v0", Repository: "o/r", TaskID: "x", Harness: "h", Status: StatusActive}); err == nil {
		t.Fatal("wrong schema must be refused")
	}
	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	_, _ = f.WriteString("{not json\n")
	_ = f.Close()
	if _, _, err := store.Lookup(ctx, "o/r", "", "T1"); err == nil || !strings.Contains(err.Error(), "malformed") {
		t.Fatalf("malformed line must fail closed: %v", err)
	}
}

func TestDefaultPathLivesUnderTheGitCommonDir(t *testing.T) {
	repo := t.TempDir()
	if out, err := runGit(repo, "init", "-q"); err != nil {
		t.Skipf("git unavailable: %v %s", err, out)
	}
	path, err := DefaultPath(context.Background(), repo)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(path, filepath.Join("pyxis", "task-sessions.jsonl")) || !strings.Contains(path, ".git") {
		t.Fatalf("path = %s", path)
	}
	if _, err := DefaultPath(context.Background(), t.TempDir()); err == nil {
		t.Fatal("a non-repository must be refused")
	}
	if _, err := New(nil, "o/r"); err == nil {
		t.Fatal("nil store must be refused")
	}
	if _, err := mintUUID(); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(context.Canceled, context.Canceled) {
		t.Fatal("sanity")
	}
}

// The closed record shape and the store's fail-closed edges.
func TestRecordValidateAndStoreEdges(t *testing.T) {
	base := Record{SchemaVersion: Schema, Repository: "o/r", TaskID: "T1", Harness: "claude", Status: StatusActive}
	for name, mutate := range map[string]func(Record) Record{
		"wrong schema":  func(r Record) Record { r.SchemaVersion = "x"; return r },
		"no repository": func(r Record) Record { r.Repository = " "; return r },
		"no task":       func(r Record) Record { r.TaskID = ""; return r },
		"no harness":    func(r Record) Record { r.Harness = ""; return r },
		"bad status":    func(r Record) Record { r.Status = "paused"; return r },
	} {
		if err := mutate(base).Validate(); err == nil {
			t.Errorf("%s must be refused", name)
		}
	}
	if err := base.Validate(); err != nil {
		t.Fatal(err)
	}
	retired := base
	retired.Status = StatusRetired
	if err := retired.Validate(); err != nil {
		t.Fatal(err)
	}
	if base.Key() != "o/r\x00\x00T1" {
		t.Fatalf("key = %q", base.Key())
	}

	if _, err := NewFileStore("  "); err == nil {
		t.Fatal("empty path must be refused")
	}
	store, _ := NewFileStore(filepath.Join(t.TempDir(), "l.jsonl"))
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := store.Put(cancelled, base); err == nil {
		t.Fatal("cancelled Put must fail")
	}
	if _, _, err := store.Lookup(cancelled, "o/r", "", "T1"); err == nil {
		t.Fatal("cancelled Lookup must fail")
	}
	if store.Path() == "" {
		t.Fatal("path accessor")
	}
	var lock *ledgerLock
	if err := lock.Close(); err != nil {
		t.Fatalf("nil lock close: %v", err)
	}
}

// Continuity's own refusals and fresh-record branches.
func TestContinuityEdges(t *testing.T) {
	c, store, _, wt := testContinuity(t)
	ctx := context.Background()
	task := port.Task{ID: "T9", Number: 9}
	if _, err := c.Session(ctx, task, " ", wt); err == nil {
		t.Fatal("empty harness must be refused")
	}
	if _, err := c.Session(ctx, task, "claude", " "); err == nil {
		t.Fatal("empty worktree must be refused")
	}
	// Retire with no record is a no-op, not an error.
	if err := c.Retire(ctx, task, "abandoned"); err != nil {
		t.Fatal(err)
	}
	// Record with no prior record creates one carrying the reported id.
	if err := c.Record(ctx, task, "codex", wt, port.SessionSpec{ID: "pre"}, port.RunResult{SessionID: "thread-1"}); err != nil {
		t.Fatal(err)
	}
	rec, ok, _ := store.Lookup(ctx, "o/r", "item:9", "T9")
	if !ok || rec.SessionID != "thread-1" || rec.Launches != 1 || rec.Harness != "codex" || rec.TaskNumber != 9 {
		t.Fatalf("record = %+v", rec)
	}
	// Record after a harness change starts a fresh record but keeps the cursor.
	_ = c.RecordFeedback(ctx, task, time.Date(2026, 9, 1, 9, 0, 0, 0, time.UTC))
	if err := c.Record(ctx, task, "claude", wt, port.SessionSpec{ID: "s2"}, port.RunResult{}); err != nil {
		t.Fatal(err)
	}
	rec, _, _ = store.Lookup(ctx, "o/r", "item:9", "T9")
	// The harness-change launch reported no session id, so it is unconfirmed:
	// the fresh record carries the cursor but arms no resume (Launches 0).
	if rec.Harness != "claude" || rec.SessionID != "s2" || rec.Launches != 0 || rec.FeedbackRounds != 1 {
		t.Fatalf("fresh record after harness change = %+v", rec)
	}
	// New(...) with a bad minter surfaces the mint failure.
	bad, _ := New(store, "o/r", WithIDMinter(func() (string, error) { return "", errors.New("entropy gone") }))
	if _, err := bad.Session(ctx, port.Task{ID: "T10"}, "claude", wt); err == nil {
		t.Fatal("mint failure must surface")
	}
}

// The ledger's operational error branches fail closed: an uncreatable lock or
// ledger directory, an unreadable ledger, and a line past the scanner bound
// are named errors, never silent adoption.
func TestStoreOperationalFailuresFailClosed(t *testing.T) {
	ctx := context.Background()
	base := Record{SchemaVersion: Schema, Repository: "o/r", TaskID: "T1", Harness: "claude", Status: StatusActive}

	// The lock (and ledger) directory cannot be created: its parent is a file.
	parent := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(parent, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	blocked, _ := NewFileStore(filepath.Join(parent, "deeper", "ledger.jsonl"))
	if err := blocked.Put(ctx, base); err == nil {
		t.Fatal("uncreatable lock directory must fail Put")
	}
	if _, _, err := blocked.Lookup(ctx, "o/r", "", "T1"); err == nil {
		t.Fatal("uncreatable lock directory must fail Lookup")
	}

	// The ledger path is a directory: the replay read fails, named.
	dirLedger := filepath.Join(t.TempDir(), "ledger.jsonl")
	if err := os.MkdirAll(dirLedger, 0o700); err != nil {
		t.Fatal(err)
	}
	asDir, _ := NewFileStore(dirLedger)
	if _, _, err := asDir.Lookup(ctx, "o/r", "", "T1"); err == nil {
		t.Fatal("a directory ledger must fail the replay")
	}
	if err := asDir.Put(ctx, base); err == nil {
		t.Fatal("a directory ledger must fail the append")
	}

	// A line past the scanner bound is a scan error, not a truncated adoption.
	huge := filepath.Join(t.TempDir(), "ledger.jsonl")
	store, _ := NewFileStore(huge)
	if err := store.Put(ctx, base); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(huge, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write(append(bytes.Repeat([]byte("x"), 5*1024*1024), '\n')); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	if _, _, err := store.Lookup(ctx, "o/r", "", "T1"); err == nil {
		t.Fatal("an over-long ledger line must fail the scan")
	}
}

func TestStoreInjectedEncodingAndDirectoryFailuresFailClosed(t *testing.T) {
	ctx := context.Background()
	record := Record{SchemaVersion: Schema, Repository: "o/r", TaskID: "T1", Harness: "claude", Status: StatusActive}
	store, err := NewFileStore(filepath.Join(t.TempDir(), "ledger.jsonl"))
	if err != nil {
		t.Fatal(err)
	}

	originalMarshal := marshalRecord
	originalMkdir := makeLedgerDirectory
	t.Cleanup(func() {
		marshalRecord = originalMarshal
		makeLedgerDirectory = originalMkdir
	})

	wantMarshal := errors.New("encoder unavailable")
	marshalRecord = func(any) ([]byte, error) { return nil, wantMarshal }
	if err := store.Put(ctx, record); !errors.Is(err, wantMarshal) {
		t.Fatalf("Put marshal error = %v, want %v", err, wantMarshal)
	}

	marshalRecord = originalMarshal
	wantMkdir := errors.New("directory unavailable")
	makeLedgerDirectory = func(string, os.FileMode) error { return wantMkdir }
	if err := store.Put(ctx, record); !errors.Is(err, wantMkdir) {
		t.Fatalf("Put directory error = %v, want %v", err, wantMkdir)
	}
}

// The two remaining operational branches: a lock path that exists as a
// directory (open fails) and a ledger file the process cannot read.
func TestStoreLockOpenAndUnreadableLedgerFailClosed(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission bits do not bind for root")
	}
	ctx := context.Background()
	base := Record{SchemaVersion: Schema, Repository: "o/r", TaskID: "T1", Harness: "claude", Status: StatusActive}

	dir := t.TempDir()
	ledger := filepath.Join(dir, "ledger.jsonl")
	if err := os.MkdirAll(ledger+".lock", 0o700); err != nil {
		t.Fatal(err)
	}
	store, _ := NewFileStore(ledger)
	if err := store.Put(ctx, base); err == nil {
		t.Fatal("a directory lock path must fail the open")
	}

	dir2 := t.TempDir()
	ledger2 := filepath.Join(dir2, "ledger.jsonl")
	store2, _ := NewFileStore(ledger2)
	if err := store2.Put(ctx, base); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(ledger2, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(ledger2, 0o600) })
	if _, _, err := store2.Lookup(ctx, "o/r", "", "T1"); err == nil {
		t.Fatal("an unreadable ledger must fail the replay, not read as empty")
	}
}

// An append the filesystem refuses (read-only ledger) is a named error.
func TestStorePutOnReadOnlyLedgerFailsClosed(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission bits do not bind for root")
	}
	ctx := context.Background()
	base := Record{SchemaVersion: Schema, Repository: "o/r", TaskID: "T1", Harness: "claude", Status: StatusActive}
	ledger := filepath.Join(t.TempDir(), "ledger.jsonl")
	store, _ := NewFileStore(ledger)
	if err := store.Put(ctx, base); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(ledger, 0o400); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(ledger, 0o600) })
	if err := store.Put(ctx, base); err == nil {
		t.Fatal("a read-only ledger must fail the append")
	}
}

// Replay tolerates blank lines, rejects a decoded record that fails validation,
// and DefaultPath refuses an empty repository path.
func TestStoreReplayEdgesAndDefaultPathEmptyRepo(t *testing.T) {
	ctx := context.Background()
	if _, err := DefaultPath(ctx, "   "); err == nil {
		t.Fatal("DefaultPath must refuse an empty repository path")
	}

	ledger := filepath.Join(t.TempDir(), "ledger.jsonl")
	store, _ := NewFileStore(ledger)
	base := Record{SchemaVersion: Schema, Repository: "o/r", TaskID: "T1", Harness: "claude", Status: StatusActive}
	if err := store.Put(ctx, base); err != nil {
		t.Fatal(err)
	}
	// A blank line between records is skipped, not an error.
	if f, err := os.OpenFile(ledger, os.O_APPEND|os.O_WRONLY, 0o600); err != nil {
		t.Fatal(err)
	} else {
		if _, err := f.WriteString("\n"); err != nil {
			t.Fatal(err)
		}
		_ = f.Close()
	}
	if _, ok, err := store.Lookup(ctx, "o/r", "", "T1"); err != nil || !ok {
		t.Fatalf("blank line must be tolerated: ok=%v err=%v", ok, err)
	}
	// A structurally valid JSON line that fails record validation is a named error.
	if f, err := os.OpenFile(ledger, os.O_APPEND|os.O_WRONLY, 0o600); err != nil {
		t.Fatal(err)
	} else {
		if _, err := f.WriteString(`{"schema":"task-session/v1","repository":"o/r","task_id":"T2"}` + "\n"); err != nil {
			t.Fatal(err)
		}
		_ = f.Close()
	}
	if _, _, err := store.Lookup(ctx, "o/r", "", "T1"); err == nil {
		t.Fatal("an invalid decoded record must fail the replay")
	}
}

// Two plans' identical task ids never share a session or a cursor: the ledger
// key carries the tracker-scoped work-item identity.
func TestContinuity_PlanScopedTaskIdentity(t *testing.T) {
	c, store, _, wt := testContinuity(t)
	ctx := context.Background()
	a := port.Task{ID: "T1", SourceRef: "#101"}
	b := port.Task{ID: "T1", SourceRef: "#202"}
	sa, err := c.Session(ctx, a, "claude", wt)
	if err != nil {
		t.Fatal(err)
	}
	sb, err := c.Session(ctx, b, "claude", wt)
	if err != nil || sa.ID == sb.ID {
		t.Fatalf("same-id tasks from different work items shared a session: %v %v %v", sa, sb, err)
	}
	if err := c.Record(ctx, a, "claude", wt, sa, port.RunResult{SessionID: sa.ID}); err != nil {
		t.Fatal(err)
	}
	if r, _ := c.Session(ctx, a, "claude", wt); !r.Resume || r.ID != sa.ID {
		t.Fatalf("a's confirmed session must resume: %+v", r)
	}
	if r, _ := c.Session(ctx, b, "claude", wt); r.Resume {
		t.Fatalf("b never launched and must not resume: %+v", r)
	}
	if err := c.RecordFeedback(ctx, a, time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	if _, rounds, _ := c.LastFeedback(ctx, b); rounds != 0 {
		t.Fatalf("b's cursor moved with a's feedback: rounds=%d", rounds)
	}
	if rec, ok, _ := store.Lookup(ctx, "o/r", "#101", "T1"); !ok || rec.Source != "#101" {
		t.Fatalf("a's record = %+v ok=%v", rec, ok)
	}
}

// A worktree deleted and recreated at the same deterministic path is a
// different workspace: the pinned generation stops matching and the stale
// conversation is not resumed into it.
func TestContinuity_WorktreeGenerationPinsTheInstance(t *testing.T) {
	c, _, _, wt := testContinuity(t)
	gen := "gen-1"
	c.genOf = func(string) string { return gen }
	ctx := context.Background()
	task := port.Task{ID: "T1", Number: 11}
	spec, err := c.Session(ctx, task, "claude", wt)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Record(ctx, task, "claude", wt, spec, port.RunResult{SessionID: spec.ID}); err != nil {
		t.Fatal(err)
	}
	if r, _ := c.Session(ctx, task, "claude", wt); !r.Resume {
		t.Fatalf("same generation must resume: %+v", r)
	}
	gen = "gen-2" // the worktree was deleted and recreated at the same path
	if r, _ := c.Session(ctx, task, "claude", wt); r.Resume {
		t.Fatalf("a recreated worktree must not adopt the stale session: %+v", r)
	}
}

// An interrupted or failed launch that never reported its conversation id is
// unconfirmed: it arms no resume; one confirmed launch does.
func TestContinuity_UnconfirmedLaunchNeverArmsResume(t *testing.T) {
	c, _, _, wt := testContinuity(t)
	ctx := context.Background()
	task := port.Task{ID: "T1", Number: 11}
	spec, err := c.Session(ctx, task, "claude", wt)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Record(ctx, task, "claude", wt, spec, port.RunResult{}); err != nil {
		t.Fatal(err)
	}
	if r, _ := c.Session(ctx, task, "claude", wt); r.Resume {
		t.Fatalf("an unconfirmed launch armed resume: %+v", r)
	}
	spec2, _ := c.Session(ctx, task, "claude", wt)
	if err := c.Record(ctx, task, "claude", wt, spec2, port.RunResult{SessionID: spec2.ID}); err != nil {
		t.Fatal(err)
	}
	if r, _ := c.Session(ctx, task, "claude", wt); !r.Resume || r.ID != spec2.ID {
		t.Fatalf("a confirmed launch must arm resume: %+v", r)
	}
}

// worktreeGeneration observes the worktree's .git entry: absent means no
// generation; a rewritten entry moves it.
func TestWorktreeGeneration(t *testing.T) {
	dir := t.TempDir()
	if g := worktreeGeneration(dir); g != "" {
		t.Fatalf("no .git entry must yield no generation, got %q", g)
	}
	gitFile := filepath.Join(dir, ".git")
	if err := os.WriteFile(gitFile, []byte("gitdir: /elsewhere"), 0o600); err != nil {
		t.Fatal(err)
	}
	g1 := worktreeGeneration(dir)
	if g1 == "" {
		t.Fatal("a .git entry must yield a generation")
	}
	if err := os.Chtimes(gitFile, time.Now().Add(2*time.Second), time.Now().Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	if g2 := worktreeGeneration(dir); g2 == g1 {
		t.Fatalf("a rewritten .git entry must move the generation: %q", g2)
	}
}
