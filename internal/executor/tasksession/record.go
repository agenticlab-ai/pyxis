package tasksession

import (
	"errors"
	"github.com/agenticlab-ai/pyxis/internal/entity"
	"strings"
	"time"
)

// Schema is the record's closed schema identity.
const Schema = "task-session/v1"

// Status values of a record.
const (
	StatusActive  = "active"
	StatusRetired = "retired"
)

// Record is one task's harness session and feedback cursor.
type Record struct {
	SchemaVersion string `json:"schema_version"`
	Repository    string `json:"repository"`
	TaskID        string `json:"task_id"`
	// Source is the tracker-scoped work-item identity behind the task (the
	// adapter's SourceRef, e.g. GitHub "#123" or "Carina work-item 2"; an
	// "item:<n>" fallback when only a number is known). Plan-task ids like
	// "T1" repeat across plans, so Source is part of the ledger key: two
	// plans' T1s never share a session or a feedback cursor.
	Source     string `json:"source,omitempty"`
	TaskNumber int    `json:"task_number,omitempty"`
	// Harness is the backend the session belongs to; a different harness
	// starts a fresh session.
	Harness entity.Harness `json:"harness"`
	// SessionID is the harness conversation identity (claude session id, codex
	// thread id, opencode session id). Pre-assigned before the first launch
	// when the harness allows it, else the id the first run reported.
	SessionID string `json:"session_id"`
	// Worktree is the absolute worktree the session was built in; a session is
	// resumed only into the same, still-existing worktree. WorktreeGen pins
	// the worktree INSTANCE (its .git entry's stat identity at session
	// creation): a worktree deleted and recreated at the same deterministic
	// path is a different workspace, and the old conversation is stale there.
	Worktree    string    `json:"worktree"`
	WorktreeGen string    `json:"worktree_gen,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	LastUsedAt  time.Time `json:"last_used_at"`
	// Launches counts the processes that ran under this session.
	Launches int `json:"launches"`
	// FeedbackRounds counts human-review feedback rounds serviced;
	// LastFeedbackAt is the newest proposal comment already serviced (the
	// cursor a later round must exceed).
	FeedbackRounds int       `json:"feedback_rounds"`
	LastFeedbackAt time.Time `json:"last_feedback_at"`
	FeedbackIDs    []string  `json:"feedback_ids,omitempty"`
	Status         string    `json:"status"`
	RetiredReason  string    `json:"retired_reason,omitempty"`
}

// Validate enforces the closed record shape.
func (r Record) Validate() error {
	switch {
	case r.SchemaVersion != Schema:
		return errors.New("tasksession: record schema is not " + Schema)
	case strings.TrimSpace(r.Repository) == "":
		return errors.New("tasksession: record has no repository")
	case strings.TrimSpace(r.TaskID) == "":
		return errors.New("tasksession: record has no task id")
	case strings.TrimSpace(r.Harness.String()) == "":
		return errors.New("tasksession: record has no harness")
	case r.Status != StatusActive && r.Status != StatusRetired:
		return errors.New("tasksession: record status must be active or retired")
	}
	return nil
}

// Key is the ledger key: one record per (repository, work item, plan task).
func (r Record) Key() string { return r.Repository + "\x00" + r.Source + "\x00" + r.TaskID }
