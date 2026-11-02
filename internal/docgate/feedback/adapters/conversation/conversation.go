// Package conversation is the operator-conversation CaptureSource adapter
// (docs/tech-design/review-feedback-capture.md §3.3). It persists a confirmed
// human review comment as a review-feedback ledger entry. It is a real adapter
// package, separate from
// the writer core, which imports it — never the reverse (AP-1/AP-3/RW-3,
// DINV-2). The medium specifics (the capture invocation, the fail-open
// retry queue) live here and die at the CaptureSource boundary (RW-3); the
// writer core knows only the medium-neutral CapturedComment.
//
// Capture is off the review critical path (PRD NFR-1) and fails open
// (PRD UB-1/UB-2): a failed ledger write never blocks the review and never
// loses the comment — it is spooled to the retry queue, idempotent on comment
// identity (UB-4), for a later drain. This is the T3 slice; disposition
// determination (§3.4) and the queue drain are later tasks.
package conversation

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/feedback"
)

// SourceID is the conversation adapter's stable source stem — it becomes the
// entry's Source field and the filename's source stem (kebab-case).
const SourceID = "conversation"

// ReviewComment is one confirmed operator comment at capture time. The
// adapter maps it to the
// medium-neutral CapturedComment the writer core consumes; the writer never
// sees this type (RW-3).
type ReviewComment struct {
	Comment       string // the review comment, verbatim (§3.8)
	Author        string // the reviewer's handle
	Timestamp     string // the confirm event's ISO-8601 timestamp
	SubjectAnchor string // <subject-stem>#<section> the comment targets
	// EditCommit is the commit associated with the resulting edit, recorded at
	// capture so the disposition observer can later ask whether it landed (§3.4,
	// T4). Empty when the confirmed comment produced no edit — a comment that
	// yields no change (a candidate no-action, UB-3).
	EditCommit string
}

// Source implements feedback.CaptureSource, yielding the single comment a
// confirmed comment carries. Each invocation captures one comment, so the
// batch is a one-element slice; SourceID is always "conversation".
type Source struct{ Comment ReviewComment }

// Comments implements feedback.CaptureSource.
func (s Source) Comments(context.Context) ([]feedback.CapturedComment, error) {
	return []feedback.CapturedComment{{
		SchemaVersion: feedback.CapturedCommentVersion,
		Comment:       s.Comment.Comment,
		Author:        s.Comment.Author,
		Timestamp:     s.Comment.Timestamp,
		SubjectAnchor: s.Comment.SubjectAnchor,
		SourceID:      SourceID,
		EditCommit:    s.Comment.EditCommit,
	}}, nil
}

// compile-time proof the adapter satisfies the port.
var _ feedback.CaptureSource = Source{}

// Outcome is the fate of one captured comment: written to the ledger, or —
// on a write failure — spooled to the retry queue. Err is set only when the
// comment could be neither written nor queued (the sole comment-at-risk case).
type Outcome struct {
	Comment feedback.CapturedComment
	Written string // the ledger entry path, if the write succeeded
	Queued  string // the queue spool path, if the write failed and it was queued
	Err     error  // non-nil only if the comment was neither written nor queued
}

// Result is a Capture call's per-comment outcomes.
type Result struct {
	Outcomes []Outcome
}

// Capture drains a conversation source and persists each comment through the
// coded writer, off the review critical path and failing open (UB-1/UB-2). A
// per-comment ledger-write failure never propagates as a blocking error: the
// comment is spooled to queueDir for a later retry, so it is never lost. The
// only returned error is a source-drain error (the adapter could produce no
// comment) — itself non-blocking to the caller, which logs and exits cleanly.
func Capture(ctx context.Context, w *feedback.Writer, queueDir string, src feedback.CaptureSource) (Result, error) {
	var res Result
	comments, err := src.Comments(ctx)
	if err != nil {
		return res, fmt.Errorf("conversation source: %w", err)
	}
	for _, cc := range comments {
		o := Outcome{Comment: cc}
		path, werr := w.WriteEntry(cc)
		if werr == nil {
			o.Written = path
			res.Outcomes = append(res.Outcomes, o)
			continue
		}
		// Fail open (UB-1/UB-2): the ledger write failed; spool the comment for
		// a later retry rather than propagate the failure onto the review path.
		qp, qerr := spool(queueDir, cc)
		if qerr != nil {
			o.Err = fmt.Errorf("ledger write failed (%v) and queueing failed: %w", werr, qerr)
		} else {
			o.Queued = qp
		}
		res.Outcomes = append(res.Outcomes, o)
	}
	return res, nil
}

// spool persists one captured comment to the retry queue as its schema-versioned
// JSON, under a content-addressed filename so the same comment queued twice
// collides to one file (idempotent on comment identity, UB-4).
func spool(dir string, cc feedback.CapturedComment) (string, error) {
	if strings.TrimSpace(dir) == "" {
		return "", fmt.Errorf("no retry queue directory configured")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create queue dir %s: %w", dir, err)
	}
	b, err := json.MarshalIndent(cc, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal queued comment: %w", err)
	}
	path := filepath.Join(dir, queueName(cc))
	if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
		return "", fmt.Errorf("write queued comment %s: %w", path, err)
	}
	return path, nil
}

// queueName is a content-addressed filename over a comment's full identity
// (source · anchor · author · timestamp · verbatim comment), giving the queue
// idempotency on comment identity (UB-4).
func queueName(cc feedback.CapturedComment) string {
	h := sha256.Sum256([]byte(strings.Join([]string{
		cc.SourceID, cc.SubjectAnchor, cc.Author, cc.Timestamp, cc.Comment,
	}, "\x00")))
	return fmt.Sprintf("%x.json", h[:])
}
