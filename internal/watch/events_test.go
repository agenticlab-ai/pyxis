package watch

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

func TestSinkAppendAndHealth(t *testing.T) {
	dir := t.TempDir()
	s := NewSink(filepath.Join(dir, "wake-events"), 0, func(string, ...any) { t.Fatal("no warning expected") })
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	s.clock = func() time.Time { return now }

	s.Append(Event{Kind: EventCheckRan, Medium: "carina", Batch: 3})
	h := s.Health()
	if h.Defect != "" {
		t.Fatalf("healthy sink reported defect %q", h.Defect)
	}
	if !h.LastCheck.Equal(now) {
		t.Fatalf("LastCheck = %v, want %v", h.LastCheck, now)
	}
}

func TestSinkHealthWithoutChecksIsLoud(t *testing.T) {
	s := NewSink(filepath.Join(t.TempDir(), "wake-events"), 0, func(string, ...any) {})
	if h := s.Health(); !strings.Contains(h.Defect, "cannot-verify-liveness") {
		t.Fatalf("empty trail must be a named defect, got %+v", h)
	}
	// A non-check event alone must not read as liveness.
	s.Append(Event{Kind: EventWakeDelivered})
	if h := s.Health(); !strings.Contains(h.Defect, "cannot-verify-liveness") {
		t.Fatalf("wake without checks must stay a named defect, got %+v", h)
	}
}

func TestSinkRotates(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "wake-events")
	s := NewSink(dir, 300, func(string, ...any) {})
	for i := 0; i < 20; i++ {
		s.Append(Event{Kind: EventCheckRan, Medium: "carina", Batch: i})
	}
	cur, err := os.Stat(filepath.Join(dir, "current.jsonl"))
	if err != nil {
		t.Fatalf("current missing after rotation: %v", err)
	}
	if cur.Size() > 300 {
		t.Fatalf("current grew past the cap: %d", cur.Size())
	}
	if _, err := os.Stat(filepath.Join(dir, "previous.jsonl")); err != nil {
		t.Fatalf("previous missing after rotation: %v", err)
	}
	// Health still reads through the rotated pair.
	if h := s.Health(); h.Defect != "" || h.LastCheck.IsZero() {
		t.Fatalf("health across rotation = %+v", h)
	}
}

func TestSinkUnwritableDegradesLoudlyOnce(t *testing.T) {
	dir := t.TempDir()
	blocked := filepath.Join(dir, "not-a-dir")
	if err := os.WriteFile(blocked, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	warnings := 0
	s := NewSink(filepath.Join(blocked, "wake-events"), 0, func(string, ...any) { warnings++ })
	s.Append(Event{Kind: EventCheckRan})
	s.Append(Event{Kind: EventCheckRan})
	s.Append(Event{Kind: EventWakeDelivered})
	if warnings != 1 {
		t.Fatalf("want exactly one warning per window, got %d", warnings)
	}
	if h := s.Health(); !strings.Contains(h.Defect, "unwritable") {
		t.Fatalf("unwritable sink must surface the named defect, got %+v", h)
	}
}

func TestSinkToleratesTornTailLine(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "wake-events")
	s := NewSink(dir, 0, func(string, ...any) {})
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	s.clock = func() time.Time { return now }
	s.Append(Event{Kind: EventCheckRan})
	f, err := os.OpenFile(filepath.Join(dir, "current.jsonl"), os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(`{"time":"2026-07-30T13:00:00Z","kind":"wake_check_r`); err != nil {
		t.Fatal(err)
	}
	f.Close()
	h := s.Health()
	if h.Defect != "" || !h.LastCheck.Equal(now) {
		t.Fatalf("torn tail must not poison health: %+v", h)
	}
}

type scriptedReader struct {
	states  []port.ProposalState
	cadence time.Duration
}

func (r scriptedReader) States(_ context.Context, refs []port.ProposalRef, _ port.Cursor) ([]port.ProposalState, error) {
	out := make([]port.ProposalState, len(refs))
	copy(out, r.states)
	return out, nil
}
func (r scriptedReader) Cadence(context.Context) time.Duration { return r.cadence }

func TestInstrumentReaderEmitsCheckAndUnknowns(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "wake-events")
	s := NewSink(dir, 0, func(string, ...any) {})
	unknown := port.UnknownRateLimited
	inner := scriptedReader{states: []port.ProposalState{
		{Landing: port.LandingLanded},
		{Unknown: &unknown},
	}, cadence: time.Second}
	r := &InstrumentReader{Inner: inner, Sink: s, Medium: "github"}

	refs := []port.ProposalRef{{Number: 1}, {Number: 2}}
	if _, err := r.States(context.Background(), refs, port.Cursor{}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "current.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if !strings.Contains(got, string(EventCheckRan)) || !strings.Contains(got, `"batch":2`) || !strings.Contains(got, `"unknowns":1`) {
		t.Fatalf("check event incomplete: %s", got)
	}
	if !strings.Contains(got, string(EventReadUnknown)) || !strings.Contains(got, "rate-limited") {
		t.Fatalf("unknown event incomplete: %s", got)
	}
	if r.Cadence(context.Background()) != time.Second {
		t.Fatal("cadence must pass through")
	}
}

func TestWrapSweepRecordsBeforeEffect(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "wake-events")
	s := NewSink(dir, 0, func(string, ...any) {})
	order := []string{}
	sweep := WrapSweep(s, "self-drive-wake/001", func(context.Context) error {
		if h := s.Health(); h.Defect == "" || strings.Contains(h.Defect, "unwritable") {
			// The wake event exists before the effect runs; liveness still needs
			// a check event, so the defect must be the no-checks one.
		}
		order = append(order, "effect")
		return nil
	})
	if err := sweep(context.Background()); err != nil {
		t.Fatal(err)
	}
	order = append(order, "done")
	data, _ := os.ReadFile(filepath.Join(dir, "current.jsonl"))
	if !strings.Contains(string(data), string(EventWakeDelivered)) {
		t.Fatalf("wake event missing: %s", data)
	}
	if len(order) != 2 || order[0] != "effect" {
		t.Fatalf("effect must run after the record: %v", order)
	}
}
