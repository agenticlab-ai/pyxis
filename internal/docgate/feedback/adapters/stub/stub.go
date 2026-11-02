// Package stub is a CaptureSource adapter for the conformance suite
// (docs/tech-design/review-feedback-capture.md §3.3): a real adapter package,
// separate from the writer core, that replays a fixed list of captured
// comments. The conformance test drives the Writer through it, and the
// import-boundary lint (feedback/writer_test.go) proves the writer core never
// imports this package — the adapter depends on the core, never the reverse
// (AP-1/AP-3/RW-3, DINV-2). The live conversation and code-host adapters (T3+)
// implement the same port; this one carries no medium mechanism.
package stub

import (
	"context"

	"github.com/agenticlab-ai/pyxis/internal/docgate/feedback"
)

// Source is a CaptureSource that yields a fixed, pre-supplied comment list.
type Source struct {
	Captured []feedback.CapturedComment
	// Err, if set, is returned instead of the comments — for exercising the
	// Writer's source-error path.
	Err error
}

// Comments implements feedback.CaptureSource.
func (s Source) Comments(context.Context) ([]feedback.CapturedComment, error) {
	if s.Err != nil {
		return nil, s.Err
	}
	return s.Captured, nil
}

// compile-time proof the stub satisfies the port.
var _ feedback.CaptureSource = Source{}
