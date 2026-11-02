package report

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"
)

// Sink consumes one normalized event. Implementations render or persist it (the
// operator log line, the argus JSONL journal). A Sink need not be safe for
// concurrent use on its own: the Reporter serializes every Deliver, so a sink's
// writes never interleave even while many Workers emit concurrently. Deliver
// returns an error only for a genuine sink failure (a broken journal writer); the
// Reporter treats delivery best-effort so one failing sink never silences another.
type Sink interface {
	Deliver(Event) error
}

// Emitter is the emit hook a producer calls — the Dispatcher for the run/task
// lifecycle, and the drive-to-green loop's OnAttempt for ci_fix_attempt. Keeping
// it an interface (satisfied by *Reporter) lets a producer depend on the narrow
// emit capability, and lets a test capture events with a trivial fake.
type Emitter interface {
	Emit(Event)
}

// compile-time proof the Reporter is a usable Emitter.
var _ Emitter = (*Reporter)(nil)

// Reporter fans one event out to every sink, serialized under a mutex so
// concurrent producers (the parallel Workers of a sweep, plus each Worker's
// self-heal loop) never interleave a sink's writes. It stamps two correlation
// fields a producer need not thread itself: the run id and the event time (§3.4,
// AP-5). One Reporter serves one run.
type Reporter struct {
	mu    sync.Mutex
	runID string
	now   func() time.Time
	sinks []Sink
	// onDeliverErr, when set, observes a sink delivery failure. Delivery stays
	// best-effort regardless: a failing sink is surfaced here, never allowed to
	// abort the run or silence the other sinks.
	onDeliverErr func(Sink, Event, error)
	onPublishErr func(Event, error)
	durable      *durableHandlers
	closed       bool
}

var ErrReporterClosed = errors.New("report: reporter is closed")

// NewDurableReporter builds the existing Reporter with durable registered
// lifecycle handlers. The ledger is loaded immediately; call Resume at startup
// to retry work left pending by an earlier process. An OS lock enforces exclusive
// ownership until Close or process exit; concurrent runs must use different
// paths. Call Close when finished with the reporter. Handlers and sink
// callbacks must not call back into this Reporter while delivery holds its lock.
func NewDurableReporter(runID, ledgerPath string, sinks []Sink, handlers []Handler, opts ...Option) (*Reporter, error) {
	r := newReporter(runID, sinks, opts)
	durable, err := newDurableHandlers(ledgerPath, handlers, r.now)
	if err != nil {
		return nil, err
	}
	r.durable = durable
	return r, nil
}

// Option configures a Reporter at construction.
type Option func(*Reporter)

// WithClock injects the clock the Reporter stamps events with (hermetic tests
// pin it; production leaves the default time.Now). A nil clock is ignored.
func WithClock(now func() time.Time) Option {
	return func(r *Reporter) {
		if now != nil {
			r.now = now
		}
	}
}

// WithDeliveryErrorHandler installs an observer for a sink's delivery failure.
// Without it a delivery error is swallowed (best-effort); with it the handler
// sees which sink failed on which event, so a driver can log or degrade loudly.
func WithDeliveryErrorHandler(h func(Sink, Event, error)) Option {
	return func(r *Reporter) { r.onDeliverErr = h }
}

// WithPublicationErrorHandler observes durable delivery failures from the
// fire-and-forget Emitter interface. Without an observer Emit logs the failure.
// Consequential producers should use Publish and handle its error directly.
func WithPublicationErrorHandler(h func(Event, error)) Option {
	return func(r *Reporter) { r.onPublishErr = h }
}

// NewReporter builds a Reporter for one run over the given sinks. runID is the
// correlation key stamped onto every event whose producer left it empty; sinks
// are delivered to in order.
func NewReporter(runID string, sinks ...Sink) *Reporter {
	return newReporter(runID, sinks, nil)
}

// NewReporterWith builds a Reporter with options (the clock, a delivery-error
// handler). It is the injectable-clock form the hermetic tests use.
func NewReporterWith(runID string, sinks []Sink, opts ...Option) *Reporter {
	return newReporter(runID, sinks, opts)
}

func newReporter(runID string, sinks []Sink, opts []Option) *Reporter {
	r := &Reporter{runID: runID, now: time.Now, sinks: sinks}
	for _, opt := range opts {
		if opt != nil {
			opt(r)
		}
	}
	return r
}

// Emit stamps the event's time and run id (when the producer left them empty) and
// delivers it to every sink under the reporter lock — so a concurrent producer's
// event is never interleaved with this one across the sinks. Delivery is
// best-effort: a sink error is routed to the delivery-error handler (if any) and
// never stops the remaining sinks or the run.
func (r *Reporter) Emit(e Event) {
	if err := r.Publish(context.Background(), e); err != nil {
		if r.onPublishErr != nil {
			r.onPublishErr(e, err)
		} else {
			slog.Error("durable event delivery failed", "event_id", e.ID, "kind", e.Kind, "error", err)
		}
	}
}

// Publish persists a lifecycle event before running its registered handlers.
// Handler failures leave only that handler pending and are returned without
// changing the event's verified lifecycle meaning or repeating successful
// siblings. Ordinary sinks remain the existing best-effort reporting surface.
func (r *Reporter) Publish(ctx context.Context, e Event) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return ErrReporterClosed
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if e.Time.IsZero() {
		e.Time = r.now()
	}
	if e.RunID == "" {
		e.RunID = r.runID
	}
	durable := r.durable != nil && r.durable.handlesKind(e.Kind)
	if durable {
		if err := r.durable.record(e); err != nil {
			return err
		}
	}
	for _, s := range r.sinks {
		if err := s.Deliver(copyEvent(e)); err != nil && r.onDeliverErr != nil {
			r.onDeliverErr(s, e, err)
		}
	}
	if durable {
		return r.durable.drain(ctx)
	}
	return nil
}

// Resume retries every pending handler loaded from the durable ledger.
func (r *Reporter) Resume(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return ErrReporterClosed
	}
	if r.durable == nil {
		return nil
	}
	return r.durable.drain(ctx)
}

// Close flushes outstanding receipts and releases the ledger's ownership lock.
// A failed flush retains ownership so the caller can repair storage and retry.
// It does not run pending handlers or close caller-owned sinks.
func (r *Reporter) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return nil
	}
	if r.durable != nil {
		if err := r.durable.flush(); err != nil {
			return err
		}
		r.closed = true
		return r.durable.lock.Close()
	}
	r.closed = true
	return nil
}

// HandlerOutcomes exposes pending and completed effects without recasting a
// successfully delivered task as an implementation failure.
func (r *Reporter) HandlerOutcomes() []HandlerOutcome {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.durable == nil {
		return nil
	}
	return r.durable.outcomes()
}
