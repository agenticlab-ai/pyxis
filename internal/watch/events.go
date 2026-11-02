package watch

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/runlogs"
)

// The wake-events sink: a local, gitignored, size-rotated JSONL file holding
// the loop's typed operational ticks (design:wake-watch-v2#3.4, D-9). Raw
// ticks never reach git — the committed teaching artifact is the proof-run
// report distilled from this stream. Effects never block on the sink; an
// unwritable sink degrades LOUDLY: one warning per rotation window plus a
// readable defect on the liveness surface (DINV-8) — because the last-check
// age FR-12's dead-checker detection reads comes from this file, a silently
// dead sink would blind exactly the alarm it feeds.

// EventKind enumerates the sink's closed set of typed kinds (basis §5.7).
type EventKind string

const (
	EventCheckRan      EventKind = "wake_check_ran"
	EventWakeDelivered EventKind = "wake_delivered"
	EventReadUnknown   EventKind = "read_unknown"
	// EventThrottleDeferred carries the requirement's own name. An earlier
	// build spelled this constant "budget_deferred" and emitted it nowhere,
	// so no consumer ever saw the divergent spelling — the other four kinds
	// already matched the basis verbatim (basis EVT-4; incident I7).
	EventThrottleDeferred       EventKind = "throttle_deferred"
	EventStaleFlagged           EventKind = "stale_flagged"
	EventReviewRecoveryDecision EventKind = "review_recovery_decision"
	EventReviewStateUnknown     EventKind = "review_state_unknown"
)

// Event is one typed line. Fields are explicit and optional-by-emptiness —
// the schema is closed (AP-6): consumers reject unknown keys, so additions
// are new fields here, never ad-hoc maps.
type Event struct {
	Time                 time.Time `json:"time"`
	Kind                 EventKind `json:"kind"`
	Medium               string    `json:"medium,omitempty"`
	Batch                int       `json:"batch,omitempty"`
	Unknowns             int       `json:"unknowns,omitempty"`
	TaskID               string    `json:"task_id,omitempty"`
	Scope                string    `json:"scope,omitempty"`
	Ref                  string    `json:"ref,omitempty"`
	Reason               string    `json:"reason,omitempty"`
	AgeS                 int       `json:"age_s,omitempty"`
	BoundS               int       `json:"bound_s,omitempty"`
	Subject              string    `json:"subject,omitempty"`
	DriveRoundID         string    `json:"drive_round_id,omitempty"`
	ReviewObligationID   string    `json:"review_obligation_id,omitempty"`
	ProposalRef          string    `json:"proposal_ref,omitempty"`
	Revision             string    `json:"revision,omitempty"`
	Action               string    `json:"action,omitempty"`
	ProviderAttemptCount *int      `json:"provider_attempt_count,omitempty"`
	RecoveryEpoch        string    `json:"recovery_epoch,omitempty"`
	RecoveryCount        *int      `json:"recovery_count,omitempty"`
	RecoveryBound        *int      `json:"recovery_bound,omitempty"`
	LastFailureClass     string    `json:"last_failure_class,omitempty"`
	EvidenceRef          string    `json:"evidence_ref,omitempty"`
	Source               string    `json:"source,omitempty"`
	FailureKind          string    `json:"failure_kind,omitempty"`
	RetryAfterMS         *int      `json:"retry_after_ms,omitempty"`
	// EVT-4's typed properties. Ledger names which window blocked;
	// QueueDepth is the waiters on that ledger in the emitting process;
	// DeferredMS is the PROJECTED wait, never elapsed; ResetAt is when the
	// blocking window refills (design:state-reader-v4#DINV-8, #DINV-10).
	Ledger     string `json:"ledger,omitempty"`
	QueueDepth int    `json:"queue_depth,omitempty"`
	DeferredMS int    `json:"deferred_ms,omitempty"`
	// omitzero, not omitempty: omitempty does nothing for a struct, so every
	// unrelated event would carry "reset_at":"0001-01-01T00:00:00Z".
	ResetAt time.Time `json:"reset_at,omitzero"`
}

// MarshalJSON resolves EVT-8's governing wire vocabulary: its payload owns the
// field name "kind" for the read-failure class, so this one event spells the
// envelope discriminator "event". Other established wake-event bytes retain
// their historical kind discriminator.
func (e Event) MarshalJSON() ([]byte, error) {
	if e.Kind != EventReviewStateUnknown {
		type wire Event
		return json.Marshal(wire(e))
	}
	return json.Marshal(struct {
		Time         time.Time `json:"time"`
		Event        EventKind `json:"event"`
		DriveRoundID string    `json:"drive_round_id"`
		Scope        string    `json:"scope"`
		Source       string    `json:"source"`
		Kind         string    `json:"kind"`
		Action       string    `json:"action"`
		RetryAfterMS int       `json:"retry_after_ms"`
		Reason       string    `json:"reason"`
	}{Time: e.Time, Event: e.Kind, DriveRoundID: e.DriveRoundID, Scope: e.Scope, Source: e.Source, Kind: e.FailureKind, Action: e.Action, RetryAfterMS: valueOrZero(e.RetryAfterMS), Reason: e.Reason})
}

func valueOrZero(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}

// ProjectedWaitMillis preserves EVT-4's distinction between no wait and a
// positive wait. Truncation would turn a real sub-millisecond deferral into
// zero, and omitempty would then erase it from the JSON record.
func ProjectedWaitMillis(d time.Duration) int {
	if d <= 0 {
		return 0
	}
	ms := d / time.Millisecond
	if d%time.Millisecond != 0 {
		ms++
	}
	return int(ms)
}

// DefaultEventsDir is the sink's home relative to the repo root — inside the
// repo's pyxis dir so one repo's watches share one trail, and gitignored so
// it never lands in history (D-9).
const DefaultEventsDir = runlogs.Watch

// Sink appends typed events to <dir>/current.jsonl, rotating to
// <dir>/previous.jsonl at maxBytes. Append never returns an error to the
// caller — effects proceed regardless — but the first failure per rotation
// window emits one warning line to warn (stderr in production) and marks the
// sink degraded, which Health surfaces as a named defect.
type Sink struct {
	dir      string
	maxBytes int64
	warn     func(format string, args ...any)
	clock    func() time.Time

	mu       sync.Mutex
	size     int64
	degraded bool
	warned   bool
}

// NewSink binds the sink. maxBytes <= 0 selects the default (4 MiB). warn
// nil selects stderr.
func NewSink(dir string, maxBytes int64, warn func(string, ...any)) *Sink {
	if maxBytes <= 0 {
		maxBytes = 4 << 20
	}
	if warn == nil {
		warn = func(format string, args ...any) { fmt.Fprintf(os.Stderr, "watch: "+format+"\n", args...) }
	}
	return &Sink{dir: dir, maxBytes: maxBytes, warn: warn, clock: time.Now}
}

func (s *Sink) current() string  { return filepath.Join(s.dir, "current.jsonl") }
func (s *Sink) previous() string { return filepath.Join(s.dir, "previous.jsonl") }

// Append writes one event. It swallows every failure into the degraded state
// (one warning per rotation window) — the caller's effect must never block on
// diagnostics (DINV-8).
func (s *Sink) Append(ev Event) {
	if s == nil {
		return
	}
	if ev.Time.IsZero() {
		ev.Time = s.clock()
	}
	line, err := json.Marshal(ev)
	if err != nil {
		s.fail("encode wake event: %v", err)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		s.failLocked("create wake-events dir: %v", err)
		return
	}
	if s.size == 0 {
		if st, err := os.Stat(s.current()); err == nil {
			s.size = st.Size()
		}
	}
	if s.size+int64(len(line))+1 > s.maxBytes {
		// Rotate: current becomes previous (best effort), a fresh current opens.
		_ = os.Rename(s.current(), s.previous())
		s.size = 0
		s.warned = false // a fresh window may warn once again
	}
	f, err := os.OpenFile(s.current(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		s.failLocked("open wake-events file: %v", err)
		return
	}
	defer f.Close()
	if _, err := f.Write(append(line, '\n')); err != nil {
		s.failLocked("append wake event: %v", err)
		return
	}
	s.size += int64(len(line)) + 1
	s.degraded = false
}

func (s *Sink) fail(format string, args ...any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failLocked(format, args...)
}

func (s *Sink) failLocked(format string, args ...any) {
	s.degraded = true
	if !s.warned {
		s.warned = true
		s.warn(format, args...)
	}
}

// Health reports the sink's liveness surface: the newest check event's age,
// or a named defect when liveness cannot be verified — an absent, stale, or
// unwritable trail is loud, never a healthy-looking footer (DINV-8; the
// status verb renders this as cannot-verify-liveness).
type Health struct {
	LastCheck time.Time
	Defect    string // empty = verifiable
}

// Health scans the current (then previous) file tail-first for the newest
// EventCheckRan. It is a read-only pass usable by any process.
func (s *Sink) Health() Health {
	s.mu.Lock()
	degraded := s.degraded
	s.mu.Unlock()
	if degraded {
		return Health{Defect: "cannot-verify-liveness: wake-events sink unwritable"}
	}
	for _, p := range []string{s.current(), s.previous()} {
		if t, ok := newestCheck(p); ok {
			return Health{LastCheck: t}
		}
	}
	return Health{Defect: "cannot-verify-liveness: no check events recorded"}
}

func newestCheck(path string) (time.Time, bool) {
	f, err := os.Open(path)
	if err != nil {
		return time.Time{}, false
	}
	defer f.Close()
	var newest time.Time
	found := false
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		var ev Event
		if err := json.Unmarshal(sc.Bytes(), &ev); err != nil {
			continue // a torn tail line is not a defect; the next append heals it
		}
		if ev.Kind == EventCheckRan && ev.Time.After(newest) {
			newest, found = ev.Time, true
		}
	}
	return newest, found
}

// InstrumentReader decorates a reader so every batch read lands one
// check-ran event and every typed unknown lands a read-unknown event —
// wrapping is how the sink integrates without editing the landed port
// (AP-27; the port ships no hook).
type InstrumentReader struct {
	Inner  Reader
	Sink   *Sink
	Medium string
}

var _ Reader = (*InstrumentReader)(nil)

func (r *InstrumentReader) States(ctx context.Context, refs []port.ProposalRef, since port.Cursor) ([]port.ProposalState, error) {
	states, err := r.Inner.States(ctx, refs, since)
	unknowns := 0
	for i := range states {
		if states[i].Unknown != nil {
			unknowns++
			r.Sink.Append(Event{Kind: EventReadUnknown, Medium: r.Medium, Ref: refs[i].String(), Reason: states[i].Unknown.String()})
		}
	}
	r.Sink.Append(Event{Kind: EventCheckRan, Medium: r.Medium, Batch: len(refs), Unknowns: unknowns})
	return states, err
}

func (r *InstrumentReader) Cadence(ctx context.Context) time.Duration { return r.Inner.Cadence(ctx) }

// WrapSweep records the wake before the sweep's effect proceeds — the wake
// event is the delivery record (DINV-8's attempt-then-effect ordering).
func WrapSweep(sink *Sink, scope string, inner Sweep) Sweep {
	return func(ctx context.Context) error {
		sink.Append(Event{Kind: EventWakeDelivered, TaskID: scope})
		return inner(ctx)
	}
}
