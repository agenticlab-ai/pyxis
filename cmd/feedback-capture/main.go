// feedback-capture — the conversation capture entry
// (docs/tech-design/review-feedback-capture.md §3.3, PRD FR-1/UB-1). It turns a
// confirmed operator comment into a review-feedback ledger entry via the
// coded writer, off the review critical path and failing open: the verbatim
// comment is read on stdin, its metadata from flags, and the conversation
// CaptureSource adapter is driven through the feedback.Writer.
//
// This binary ALWAYS exits 0 on the capture path: a ledger-write failure spools
// the comment to the retry queue and never blocks the review (UB-1/UB-2). The
// bash resolver shim (.pyxis/scripts/feedback-capture-resolve.sh) only
// locates and invokes this binary — all capture logic lives here, never in the
// shim. Capture requires no editor integration or companion process.
//
// Usage:
//
//	feedback-capture --author=<h> --subject-anchor=<stem#sec> [--timestamp=<iso>] < comment
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/docgate/feedback"
	"github.com/agenticlab-ai/pyxis/internal/docgate/feedback/adapters/conversation"
)

var processExit = os.Exit

func main() { processExit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr, time.Now)) }

// run is the testable entry: flags + stdin in, exit code out. now is injected
// so the default-timestamp path is hermetic under test (no ambient clock).
func run(args []string, stdin io.Reader, stdout, stderr io.Writer, now func() time.Time) int {
	fs := flag.NewFlagSet("feedback-capture", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var author, timestamp, anchor, editCommit, root, queue string
	fs.StringVar(&author, "author", "", "the review comment's author handle")
	fs.StringVar(&timestamp, "timestamp", "", "ISO-8601 capture timestamp (default: now, UTC)")
	fs.StringVar(&anchor, "subject-anchor", "", "<subject-stem>#<section> the comment targets")
	fs.StringVar(&editCommit, "edit-commit", "", "the commit associated with the resulting edit (empty if none), recorded for later disposition")
	fs.StringVar(&root, "root", "docs/feedback", "the review-feedback ledger root")
	fs.StringVar(&queue, "queue", ".pyxis/feedback-queue", "the fail-open retry queue directory")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	comment, err := io.ReadAll(stdin)
	if err != nil {
		// Fail open: even a stdin read error must not block the review.
		_, _ = fmt.Fprintf(stderr, "feedback-capture: read comment: %v (review not blocked)\n", err) // Best-effort diagnostics must not block review.
		return 0
	}
	if timestamp == "" {
		timestamp = now().UTC().Format(time.RFC3339)
	}

	ann := conversation.ReviewComment{
		Comment:       string(comment),
		Author:        author,
		Timestamp:     timestamp,
		SubjectAnchor: anchor,
		EditCommit:    editCommit,
	}
	res, err := conversation.Capture(context.Background(), feedback.NewWriter(root), queue, conversation.Source{Comment: ann})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "feedback-capture: %v (review not blocked)\n", err) // Best-effort diagnostics must not block review.
		return 0
	}
	for _, o := range res.Outcomes {
		switch {
		case o.Written != "":
			_, _ = fmt.Fprintf(stdout, "captured: %s\n", o.Written) // Best-effort diagnostics must not block review.
		case o.Err != nil:
			_, _ = fmt.Fprintf(stderr, "feedback-capture: %v (review not blocked)\n", o.Err) // Best-effort diagnostics must not block review.
		default:
			_, _ = fmt.Fprintf(stderr, "feedback-capture: ledger write failed; queued for retry at %s (review not blocked)\n", o.Queued) // Best-effort diagnostics must not block review.
		}
	}
	return 0
}
