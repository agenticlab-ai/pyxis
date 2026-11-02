package report

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"
)

// ArgusSink is the unattended sink: it appends the normalized event stream to a
// delivery journal as one self-describing JSON object per line (JSONL,
// files-as-truth), and fail-closed-escalates a stuck/failed task the moment it
// reaches the journal (§3.9). It reuses argus's journal-as-truth shape rather
// than inventing a new surface: the events land on the journal, and a stuck task
// surfaces through the injected escalation handler — argus escalates to the human
// per its authority contract; the executor is only the hands (DINV-7).
//
// The write is guarded by its own mutex so the sink is correct even used outside
// a Reporter; under a Reporter the two locks are simply redundant, never a
// deadlock (they are never held across each other).
type ArgusSink struct {
	mu       sync.Mutex
	w        io.Writer
	escalate func(Event)
	// escalations counts fail-closed escalations surfaced this run — an observable
	// the driver reads to decide whether the run needs human attention.
	escalations int
}

var _ Sink = (*ArgusSink)(nil)

// ArgusOption configures an ArgusSink.
type ArgusOption func(*ArgusSink)

// WithEscalationHandler installs the fail-closed escalation surface: the handler
// is invoked for every escalating event (a stuck/failed task, IsEscalation),
// after that event is journaled. In production it trips argus's
// escalation-brief / wake-channel; without it the escalation is still recorded in
// the journal (escalated=true on the task_outcome line) and counted, never lost.
func WithEscalationHandler(h func(Event)) ArgusOption {
	return func(s *ArgusSink) { s.escalate = h }
}

// NewArgusSink builds an argus journal sink writing JSONL to journal (a file in
// production, a buffer in tests). A nil writer is a wiring error surfaced at
// Deliver time — the sink fails closed rather than silently dropping the journal.
func NewArgusSink(journal io.Writer, opts ...ArgusOption) *ArgusSink {
	s := &ArgusSink{w: journal}
	for _, opt := range opts {
		if opt != nil {
			opt(s)
		}
	}
	return s
}

// Deliver appends the event to the journal as one JSON line, then fail-closed-
// escalates when the event is a stuck/failed terminal (IsEscalation). A journal
// write failure is returned (the Reporter routes it to its delivery-error
// handler); the escalation handler runs only after a successful journal write, so
// files-as-truth holds — nothing escalates that was not first recorded.
func (s *ArgusSink) Deliver(e Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.w == nil {
		return fmt.Errorf("report: argus sink has no journal writer")
	}
	line, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("report: marshal event %s: %w", e.Kind, err)
	}
	line = append(line, '\n')
	if _, err := s.w.Write(line); err != nil {
		return fmt.Errorf("report: append to argus journal: %w", err)
	}

	if IsEscalation(e) {
		s.escalations++
		if s.escalate != nil {
			s.escalate(e)
		}
	}
	return nil
}

// Escalations reports how many fail-closed escalations this sink has surfaced —
// an observable the driver reads after a run to decide whether it needs human
// attention.
func (s *ArgusSink) Escalations() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.escalations
}
