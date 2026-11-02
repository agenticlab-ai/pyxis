package report

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"
)

// Handler is one consequential, durably tracked reaction to lifecycle events.
// ID must remain stable across restarts. Handle must be idempotent for Event.ID.
type Handler interface {
	ID() string
	Kinds() []Kind
	After() []string
	Handle(context.Context, Event) error
}

// HandlerOutcome is the cold-readable delivery state for one event/handler.
type HandlerOutcome struct {
	EventID   string    `json:"event_id"`
	EventKind Kind      `json:"event_kind"`
	HandlerID string    `json:"handler_id"`
	Status    string    `json:"status"`
	Attempts  int       `json:"attempts"`
	LastError string    `json:"last_error,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

type deliveryRecord struct {
	Sequence uint64                    `json:"sequence"`
	Event    Event                     `json:"event"`
	Handlers map[string]HandlerOutcome `json:"handlers"`
}

type deliveryLedger struct {
	Schema  string                    `json:"schema"`
	Records map[string]deliveryRecord `json:"records"`
}

const deliverySchema = "executor-lifecycle-delivery/v1"

// ErrDeliveryLedgerInUse means another reporter owns this ledger. Use a
// different path for concurrent runs or close the owner before resuming it.
var ErrDeliveryLedgerInUse = errors.New("report: durable delivery ledger is already in use")

// durableHandlers is deliberately part of Reporter rather than a second event
// bus. Reporter remains the single fan-out point; this ledger adds recovery for
// effects whose loss would be consequential.
type durableHandlers struct {
	path     string
	handlers map[string]Handler
	order    []string
	ledger   deliveryLedger
	now      func() time.Time
	dirty    bool
	save     func() error
	lock     io.Closer
	sequence uint64
}

func newDurableHandlers(path string, handlers []Handler, now func() time.Time) (*durableHandlers, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("report: empty durable delivery path")
	}
	d := &durableHandlers{path: path, handlers: make(map[string]Handler), now: now,
		ledger: deliveryLedger{Schema: deliverySchema, Records: make(map[string]deliveryRecord)}}
	d.save = d.persist
	for _, h := range handlers {
		if h == nil || (reflect.ValueOf(h).Kind() == reflect.Pointer && reflect.ValueOf(h).IsNil()) || strings.TrimSpace(h.ID()) == "" {
			return nil, errors.New("report: lifecycle handler has no stable identity")
		}
		if len(h.Kinds()) == 0 {
			return nil, fmt.Errorf("report: lifecycle handler %q has no event kinds", h.ID())
		}
		for _, kind := range h.Kinds() {
			if strings.TrimSpace(string(kind)) == "" {
				return nil, fmt.Errorf("report: lifecycle handler %q has an empty event kind", h.ID())
			}
		}
		if _, exists := d.handlers[h.ID()]; exists {
			return nil, fmt.Errorf("report: duplicate lifecycle handler %q", h.ID())
		}
		d.handlers[h.ID()] = h
	}
	if err := d.resolveOrder(); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("report: create durable delivery directory: %w", err)
	}
	// Resolve aliases before selecting the sidecar lock, including an existing
	// ledger symlink. All reporters addressing that ledger must share one lock.
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		d.path = resolved
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("report: resolve durable delivery path: %w", err)
	}
	lock, err := os.OpenFile(d.path+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("report: open durable delivery lock: %w", err)
	}
	if err := lockDeliveryFile(lock); err != nil {
		return nil, errors.Join(fmt.Errorf("%w: %s: %w", ErrDeliveryLedgerInUse, d.path, err), lock.Close())
	}
	d.lock = lock
	if err := d.load(); err != nil {
		return nil, errors.Join(err, lock.Close())
	}
	return d, nil
}

func (d *durableHandlers) resolveOrder() error {
	state := make(map[string]uint8)
	var visit func(string) error
	visit = func(id string) error {
		switch state[id] {
		case 1:
			return fmt.Errorf("report: lifecycle handler dependency cycle at %q", id)
		case 2:
			return nil
		}
		state[id] = 1
		for _, dep := range d.handlers[id].After() {
			if _, ok := d.handlers[dep]; !ok {
				return fmt.Errorf("report: lifecycle handler %q depends on unknown handler %q", id, dep)
			}
			if err := visit(dep); err != nil {
				return err
			}
		}
		state[id] = 2
		d.order = append(d.order, id)
		return nil
	}
	ids := make([]string, 0, len(d.handlers))
	for id := range d.handlers {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if err := visit(id); err != nil {
			return err
		}
	}
	return nil
}

func (d *durableHandlers) load() error {
	b, err := os.ReadFile(d.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("report: read durable delivery ledger: %w", err)
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&d.ledger); err != nil {
		return fmt.Errorf("report: decode durable delivery ledger: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return errors.New("report: durable delivery ledger has trailing content")
	}
	if d.ledger.Schema != deliverySchema || d.ledger.Records == nil {
		return errors.New("report: unsupported durable delivery ledger")
	}
	sequences := make(map[uint64]bool)
	for eventID, rec := range d.ledger.Records {
		if strings.TrimSpace(eventID) == "" || rec.Event.ID != eventID || rec.Event.Kind == "" || rec.Handlers == nil || rec.Sequence == 0 || sequences[rec.Sequence] {
			return fmt.Errorf("report: malformed durable delivery record %q", eventID)
		}
		sequences[rec.Sequence] = true
		if rec.Sequence > d.sequence {
			d.sequence = rec.Sequence
		}
		for handlerID, outcome := range rec.Handlers {
			if outcome.EventID != eventID || outcome.EventKind != rec.Event.Kind || outcome.HandlerID != handlerID ||
				outcome.Attempts < 0 || (outcome.Status != "pending" && outcome.Status != "completed") ||
				(outcome.Status == "completed" && (outcome.Attempts == 0 || outcome.LastError != "")) {
				return fmt.Errorf("report: malformed outcome for event %q handler %q", eventID, handlerID)
			}
			if _, ok := d.handlers[handlerID]; !ok && outcome.Status != "completed" {
				return fmt.Errorf("report: pending delivery references unregistered handler %q", handlerID)
			}
			if outcome.Status == "completed" {
				continue
			}
			h := d.handlers[handlerID]
			if !handles(h, rec.Event.Kind) {
				return fmt.Errorf("report: pending handler %q no longer handles %q", handlerID, rec.Event.Kind)
			}
			for _, dep := range h.After() {
				if _, recorded := rec.Handlers[dep]; !recorded && handles(d.handlers[dep], rec.Event.Kind) {
					return fmt.Errorf("report: pending handler %q has unrecorded dependency %q", handlerID, dep)
				}
			}
		}
	}
	return nil
}

func (d *durableHandlers) persist() error {
	b, err := json.MarshalIndent(d.ledger, "", "  ")
	if err != nil {
		return fmt.Errorf("report: encode durable delivery ledger: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(d.path), 0o755); err != nil {
		return fmt.Errorf("report: create durable delivery directory: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(d.path), ".lifecycle-delivery-*")
	if err != nil {
		return fmt.Errorf("report: create durable delivery temporary file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }() // Best-effort cleanup; rename consumes the file on success.
	// CreateTemp already creates a private 0600 file. Keep the write error in
	// the outer scope so a failed write can never publish a truncated ledger.
	if err := writeDeliveryFile(tmp, append(b, '\n')); err != nil {
		return fmt.Errorf("report: write durable delivery ledger: %w", err)
	}
	if err := os.Rename(tmpName, d.path); err != nil {
		return fmt.Errorf("report: replace durable delivery ledger: %w", err)
	}
	dir, err := os.Open(filepath.Dir(d.path))
	if err != nil {
		return fmt.Errorf("report: open durable delivery directory: %w", err)
	}
	if err := dir.Sync(); err != nil {
		_ = dir.Close() // Preserve the directory sync failure.
		return fmt.Errorf("report: sync durable delivery directory: %w", err)
	}
	if err := dir.Close(); err != nil {
		return fmt.Errorf("report: close durable delivery directory: %w", err)
	}
	return nil
}

type deliveryFile interface {
	io.WriteCloser
	Sync() error
}

func writeDeliveryFile(f deliveryFile, body []byte) error {
	n, err := f.Write(body)
	if err == nil && n != len(body) {
		err = io.ErrShortWrite
	}
	if err == nil {
		err = f.Sync()
	}
	return errors.Join(err, f.Close())
}

// flush retries a failed save before any further handler can run. In-memory
// successes remain pending persistence, so a live retry does not repeat their
// effects or run dependents ahead of durable evidence. After a crash handlers
// still need to be idempotent: an effect and its receipt cannot commit together.
func (d *durableHandlers) flush() error {
	if !d.dirty {
		return nil
	}
	if err := d.save(); err != nil {
		return err
	}
	d.dirty = false
	return nil
}

func copyEvent(e Event) Event {
	if e.Counts != nil {
		counts := *e.Counts
		e.Counts = &counts
	}
	return e
}

func handles(h Handler, kind Kind) bool {
	for _, candidate := range h.Kinds() {
		if candidate == kind {
			return true
		}
	}
	return false
}

func (d *durableHandlers) handlesKind(kind Kind) bool {
	for _, h := range d.handlers {
		if handles(h, kind) {
			return true
		}
	}
	return false
}

func (d *durableHandlers) record(e Event) error {
	if strings.TrimSpace(e.ID) == "" {
		return errors.New("report: lifecycle event has no stable identity")
	}
	if existing, ok := d.ledger.Records[e.ID]; ok {
		prior := existing.Event
		// Only transport correlation may change when a producer retries a
		// transition in another run. Every handler-visible payload field binds
		// the stable identity, including fields added to Event in the future.
		prior.Time, prior.RunID = e.Time, e.RunID
		if !reflect.DeepEqual(prior, e) {
			return fmt.Errorf("report: lifecycle event identity %q reused for a different transition", e.ID)
		}
		return d.flush()
	}
	if _, err := json.Marshal(e); err != nil {
		return fmt.Errorf("report: encode lifecycle event: %w", err)
	}
	if d.sequence == ^uint64(0) {
		return errors.New("report: durable delivery sequence exhausted")
	}
	d.sequence++
	rec := deliveryRecord{Sequence: d.sequence, Event: copyEvent(e), Handlers: make(map[string]HandlerOutcome)}
	for _, id := range d.order {
		if handles(d.handlers[id], e.Kind) {
			rec.Handlers[id] = HandlerOutcome{EventID: e.ID, EventKind: e.Kind, HandlerID: id, Status: "pending", UpdatedAt: d.now()}
		}
	}
	d.ledger.Records[e.ID] = rec
	d.dirty = true
	return d.flush()
}

func (d *durableHandlers) drain(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := d.flush(); err != nil {
		return err
	}
	ids := make([]string, 0, len(d.ledger.Records))
	for id := range d.ledger.Records {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return d.ledger.Records[ids[i]].Sequence < d.ledger.Records[ids[j]].Sequence })
	var joined error
	blocked := make(map[string]bool)
	for _, eventID := range ids {
		rec := d.ledger.Records[eventID]
		for _, handlerID := range d.order {
			if err := ctx.Err(); err != nil {
				return errors.Join(joined, err)
			}
			outcome, selected := rec.Handlers[handlerID]
			if !selected || outcome.Status == "completed" || blocked[handlerID] {
				continue
			}
			ready := true
			for _, dep := range d.handlers[handlerID].After() {
				if depOutcome, relevant := rec.Handlers[dep]; relevant && depOutcome.Status != "completed" {
					ready = false
					break
				}
			}
			if !ready {
				blocked[handlerID] = true
				continue
			}
			outcome.Attempts++
			outcome.UpdatedAt = d.now()
			if err := d.handlers[handlerID].Handle(ctx, copyEvent(rec.Event)); err != nil {
				blocked[handlerID] = true
				outcome.LastError = err.Error()
				joined = errors.Join(joined, fmt.Errorf("handler %s for event %s: %w", handlerID, eventID, err))
			} else {
				outcome.Status = "completed"
				outcome.LastError = ""
			}
			rec.Handlers[handlerID] = outcome
			d.ledger.Records[eventID] = rec
			d.dirty = true
			if err := d.flush(); err != nil {
				return errors.Join(joined, err)
			}
		}
	}
	return joined
}

func (d *durableHandlers) outcomes() []HandlerOutcome {
	var out []HandlerOutcome
	for _, rec := range d.ledger.Records {
		for _, outcome := range rec.Handlers {
			out = append(out, outcome)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].EventID == out[j].EventID {
			return out[i].HandlerID < out[j].HandlerID
		}
		return out[i].EventID < out[j].EventID
	})
	return out
}
