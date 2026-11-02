package tasksession

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/entity"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

// Continuity decides, per launch, whether a task continues its recorded
// harness session or starts one, and keeps the record and the feedback cursor
// current. It is the one implementation behind worker.SessionContinuity and
// dispatch.FeedbackLedger.
type Continuity struct {
	store      Store
	repository string
	now        func() time.Time
	newID      func() (string, error)
	dirExists  func(string) bool
	genOf      func(string) string
}

// Option configures a Continuity.
type Option func(*Continuity)

// WithClock injects the clock (tests).
func WithClock(now func() time.Time) Option { return func(c *Continuity) { c.now = now } }

// WithIDMinter injects the fresh-session identity minter (tests).
func WithIDMinter(mint func() (string, error)) Option {
	return func(c *Continuity) { c.newID = mint }
}

// New builds a Continuity over a store for one repository slug.
func New(store Store, repository string, opts ...Option) (*Continuity, error) {
	if store == nil {
		return nil, errors.New("tasksession: nil store")
	}
	if strings.TrimSpace(repository) == "" {
		return nil, errors.New("tasksession: empty repository")
	}
	c := &Continuity{store: store, repository: repository, now: func() time.Time { return time.Now().UTC() }, newID: mintUUID, dirExists: dirExists, genOf: worktreeGeneration}
	for _, opt := range opts {
		if opt != nil {
			opt(c)
		}
	}
	return c, nil
}

// Session returns the spec for the task's next implementation launch on
// harness in worktree: a resume of the recorded session when one is active
// for the same harness and the same, still-existing worktree; otherwise a
// fresh session whose identity is minted and persisted BEFORE the launch, so
// a harness that pre-assigns identities (claude) starts under an id the
// ledger already knows even if the process dies before reporting.
func (c *Continuity) Session(ctx context.Context, task port.Task, harness entity.Harness, worktree string) (port.SessionSpec, error) {
	harness = entity.Harness(strings.TrimSpace(harness.String()))
	worktree = filepath.Clean(strings.TrimSpace(worktree))
	if harness == "" || worktree == "." || worktree == "" {
		return port.SessionSpec{}, errors.New("tasksession: session needs a harness and a worktree")
	}
	rec, ok, err := c.lookup(ctx, task)
	if err != nil {
		return port.SessionSpec{}, err
	}
	if ok && c.resumable(rec, harness, worktree) {
		return port.SessionSpec{ID: rec.SessionID, Resume: true}, nil
	}
	id, err := c.newID()
	if err != nil {
		return port.SessionSpec{}, fmt.Errorf("tasksession: mint session id: %w", err)
	}
	now := c.now()
	fresh := Record{SchemaVersion: Schema, Repository: c.repository, TaskID: task.ID, Source: sourceOf(task), TaskNumber: task.Number,
		Harness: harness, SessionID: id, Worktree: worktree, WorktreeGen: c.genOf(worktree), CreatedAt: now, LastUsedAt: now, Status: StatusActive}
	if ok {
		// Carry the human-feedback cursor across a fresh session: the comments
		// already serviced stay serviced even when the conversation restarts.
		fresh.FeedbackRounds, fresh.LastFeedbackAt = rec.FeedbackRounds, rec.LastFeedbackAt
		fresh.FeedbackIDs = rec.FeedbackIDs
	}
	if err := c.store.Put(ctx, fresh); err != nil {
		return port.SessionSpec{}, err
	}
	return port.SessionSpec{ID: id, Resume: false}, nil
}

// ResumableHarness returns the backend of the task's currently resumable
// conversation. Callers can use it as an automatic-routing hint so a later
// process invocation does not abandon an existing model session merely because
// another provider became available. It never returns stale or provisional
// records.
func (c *Continuity) ResumableHarness(ctx context.Context, task port.Task) (entity.Harness, bool, error) {
	rec, ok, err := c.lookup(ctx, task)
	if err != nil || !ok {
		return "", false, err
	}
	worktree := strings.TrimSpace(rec.Worktree)
	if worktree == "" || !c.resumable(rec, rec.Harness, worktree) {
		return "", false, nil
	}
	return rec.Harness, true, nil
}

// resumable is the one predicate deciding continuation: active, same harness,
// same worktree path, the same worktree INSTANCE (generation pinned at
// creation — a deleted-and-recreated worktree at the deterministic path is a
// different workspace holding none of the session's files), that worktree
// still on disk, and at least one CONFIRMED launch — a launch counts only
// when the harness reported the conversation's identity (Record), so a
// pre-assigned or synthetic id no process ever established is minted again
// rather than resumed.
func (c *Continuity) resumable(rec Record, harness entity.Harness, worktree string) bool {
	if rec.Status != StatusActive || rec.Harness != harness || strings.TrimSpace(rec.SessionID) == "" ||
		rec.Launches <= 0 || filepath.Clean(rec.Worktree) != worktree || !c.dirExists(worktree) {
		return false
	}
	// A recorded generation must still match; a record without one (pre-gen
	// ledger, or a worktree without a .git entry) keeps the path-only check.
	return rec.WorktreeGen == "" || rec.WorktreeGen == c.genOf(worktree)
}

// lookup reads the task's record under the tracker-scoped key.
func (c *Continuity) lookup(ctx context.Context, task port.Task) (Record, bool, error) {
	return c.store.Lookup(ctx, c.repository, sourceOf(task), task.ID)
}

// sourceOf is the tracker-scoped work-item identity a task is keyed by:
// the adapter's native SourceRef, an "item:<n>" fallback when only the
// tracker number is known, empty when the task carries neither.
func sourceOf(task port.Task) string {
	if src := strings.TrimSpace(task.SourceRef); src != "" {
		return src
	}
	if task.Number != 0 {
		return fmt.Sprintf("item:%d", task.Number)
	}
	return ""
}

// Record persists what a finished launch actually ran under: the harness's
// reported session id when it reports one (codex and opencode mint their own;
// claude echoes the pre-assigned id), the launch count, and the last use.
func (c *Continuity) Record(ctx context.Context, task port.Task, harness entity.Harness, worktree string, spec port.SessionSpec, res port.RunResult) error {
	harness = entity.Harness(strings.TrimSpace(harness.String()))
	worktree = filepath.Clean(strings.TrimSpace(worktree))
	rec, ok, err := c.lookup(ctx, task)
	if err != nil {
		return err
	}
	now := c.now()
	if !ok || rec.Harness != harness || rec.Status != StatusActive || filepath.Clean(rec.Worktree) != worktree {
		prior := rec
		rec = Record{SchemaVersion: Schema, Repository: c.repository, TaskID: task.ID, Source: sourceOf(task), TaskNumber: task.Number,
			Harness: harness, Worktree: worktree, WorktreeGen: c.genOf(worktree), CreatedAt: now, Status: StatusActive}
		if ok {
			rec.FeedbackRounds, rec.LastFeedbackAt = prior.FeedbackRounds, prior.LastFeedbackAt
			rec.FeedbackIDs = prior.FeedbackIDs
		}
	}
	if id := strings.TrimSpace(res.SessionID); id != "" {
		rec.SessionID = id
	} else if strings.TrimSpace(rec.SessionID) == "" {
		rec.SessionID = strings.TrimSpace(spec.ID)
	}
	if task.Number != 0 {
		rec.TaskNumber = task.Number
	}
	// Only a CONFIRMED launch — one whose harness reported the conversation's
	// identity — counts toward resumability. An interrupted or failed run that
	// never reported (timeout, cancellation, crash before the id event) proves
	// no conversation exists under a synthetic id, so it must not arm resume.
	if strings.TrimSpace(res.SessionID) != "" {
		rec.Launches++
	}
	rec.LastUsedAt = now
	return c.store.Put(ctx, rec)
}

// Retire ends a task's session (its proposal landed or closed, the task was
// abandoned): later launches start fresh.
func (c *Continuity) Retire(ctx context.Context, task port.Task, reason string) error {
	rec, ok, err := c.lookup(ctx, task)
	if err != nil || !ok {
		return err
	}
	if rec.Status == StatusRetired {
		return nil
	}
	rec.Status, rec.RetiredReason, rec.LastUsedAt = StatusRetired, strings.TrimSpace(reason), c.now()
	return c.store.Put(ctx, rec)
}

// LastFeedback is the human-feedback cursor: the newest proposal comment a
// round already serviced (zero when none) and how many rounds ran.
func (c *Continuity) LastFeedback(ctx context.Context, task port.Task) (time.Time, int, error) {
	rec, ok, err := c.lookup(ctx, task)
	if err != nil || !ok {
		return time.Time{}, 0, err
	}
	return rec.LastFeedbackAt, rec.FeedbackRounds, nil
}

// RecordFeedback advances the cursor to latest and counts the round. It is
// called at ADMISSION of a feedback round, so the same comments never wake a
// second round even when the round fails: a failed round is a visible failed
// terminal, and a human's next comment is the retry.
func (c *Continuity) RecordFeedback(ctx context.Context, task port.Task, latest time.Time) error {
	return c.RecordFeedbackComments(ctx, task, latest, nil)
}

// FeedbackSeen returns the native identities at the current timestamp. This
// disambiguates distinct comments created within the API's timestamp precision.
func (c *Continuity) FeedbackSeen(ctx context.Context, task port.Task) (map[string]bool, error) {
	rec, _, err := c.lookup(ctx, task)
	if err != nil {
		return nil, err
	}
	ids := map[string]bool{}
	for _, id := range rec.FeedbackIDs {
		ids[id] = true
	}
	return ids, nil
}

func (c *Continuity) RecordFeedbackComments(ctx context.Context, task port.Task, latest time.Time, comments []port.Comment) error {
	rec, ok, err := c.lookup(ctx, task)
	if err != nil {
		return err
	}
	now := c.now()
	if !ok {
		rec = Record{SchemaVersion: Schema, Repository: c.repository, TaskID: task.ID, Source: sourceOf(task), TaskNumber: task.Number,
			Harness: "pending", CreatedAt: now, Status: StatusActive}
	}
	if latest.After(rec.LastFeedbackAt) {
		rec.LastFeedbackAt = latest
		rec.FeedbackIDs = nil
	}
	seen := map[string]bool{}
	for _, id := range rec.FeedbackIDs {
		seen[id] = true
	}
	for _, comment := range comments {
		if comment.CreatedAt.Equal(rec.LastFeedbackAt) && comment.NativeID != "" && !seen[comment.NativeID] {
			rec.FeedbackIDs = append(rec.FeedbackIDs, comment.NativeID)
			seen[comment.NativeID] = true
		}
	}
	rec.FeedbackRounds++
	rec.LastUsedAt = now
	return c.store.Put(ctx, rec)
}

// mintUUID returns a random RFC 4122 v4 UUID — the identity claude accepts for
// --session-id and a fine opaque key for every other harness.
func mintUUID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	h := hex.EncodeToString(b[:])
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32], nil
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// worktreeGeneration is the worktree instance identity: the stat identity of
// its .git entry (written once when git creates the worktree, file or dir).
// A recreated worktree gets a fresh .git entry, so the generation moves and a
// stale session stops matching. Empty when the entry cannot be observed.
func worktreeGeneration(worktree string) string {
	info, err := os.Stat(filepath.Join(worktree, ".git"))
	if err != nil {
		return ""
	}
	return fmt.Sprintf("git:%d:%d", info.ModTime().UnixNano(), info.Size())
}
