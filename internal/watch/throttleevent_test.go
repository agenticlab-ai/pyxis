package watch

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// EVT-4's kind is the requirement's own name. An earlier build spelled the
// constant "budget_deferred" and emitted it nowhere (incident I7).
func TestThrottleDeferredUsesTheContractedKindName(t *testing.T) {
	if EventThrottleDeferred != "throttle_deferred" {
		t.Errorf("kind = %q, want throttle_deferred (basis EVT-4)", EventThrottleDeferred)
	}
}

func TestProjectedWaitMillisNeverErasesAPositiveDeferral(t *testing.T) {
	for _, tc := range []struct {
		in   time.Duration
		want int
	}{
		{0, 0},
		{time.Nanosecond, 1},
		{999 * time.Microsecond, 1},
		{time.Millisecond, 1},
		{time.Millisecond + time.Nanosecond, 2},
	} {
		if got := ProjectedWaitMillis(tc.in); got != tc.want {
			t.Errorf("ProjectedWaitMillis(%s) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestSubMillisecondDeferralSerializesAsNonZero(t *testing.T) {
	dir := t.TempDir()
	sink := NewSink(dir, 0, nil)
	sink.Append(Event{
		Kind:       EventThrottleDeferred,
		Ledger:     "host-umbrella",
		QueueDepth: 1,
		DeferredMS: ProjectedWaitMillis(time.Nanosecond),
		ResetAt:    time.Now().Add(time.Nanosecond),
	})
	data, err := os.ReadFile(filepath.Join(dir, "current.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"deferred_ms":1`) {
		t.Fatalf("positive sub-millisecond deferral lost from event: %s", data)
	}
}

// The sink's closed schema carries EVT-4's typed properties: which ledger
// blocked, the queue depth, the PROJECTED wait, and the reset deadline —
// together the operator's only evidence that a stalled write is throttling
// rather than hung (design:state-reader-v4#DINV-8, #DINV-10).
func TestThrottleDeferredCarriesLedgerDepthWaitAndReset(t *testing.T) {
	dir := t.TempDir()
	sink := NewSink(dir, 0, nil)
	reset := time.Date(2026, 8, 5, 13, 0, 0, 0, time.UTC)

	sink.Append(Event{
		Kind:       EventThrottleDeferred,
		Medium:     "github",
		Ledger:     "host-umbrella",
		QueueDepth: 9,
		DeferredMS: int((59*time.Minute + 59*time.Second) / time.Millisecond),
		ResetAt:    reset,
	})

	data, err := os.ReadFile(filepath.Join(dir, "current.jsonl"))
	if err != nil {
		t.Fatalf("read sink: %v", err)
	}
	line := strings.TrimSpace(string(data))

	var got struct {
		Kind       string    `json:"kind"`
		Ledger     string    `json:"ledger"`
		QueueDepth int       `json:"queue_depth"`
		DeferredMS int       `json:"deferred_ms"`
		ResetAt    time.Time `json:"reset_at"`
	}
	if err := json.Unmarshal([]byte(line), &got); err != nil {
		t.Fatalf("decode event: %v\n%s", err, line)
	}
	if got.Kind != "throttle_deferred" || got.Ledger != "host-umbrella" {
		t.Errorf("kind/ledger = %q/%q", got.Kind, got.Ledger)
	}
	if got.QueueDepth != 9 {
		t.Errorf("queue_depth = %d, want 9", got.QueueDepth)
	}
	if got.DeferredMS != 3599000 {
		t.Errorf("deferred_ms = %d, want 3599000 — the projected wait, not elapsed", got.DeferredMS)
	}
	if !got.ResetAt.Equal(reset) {
		t.Errorf("reset_at = %s, want %s", got.ResetAt, reset)
	}
}

// Every other kind stays byte-identical to what it emitted before EVT-4's
// fields existed. `omitempty` does nothing for a struct, so a plain time.Time
// would stamp a zero timestamp onto every check, wake and stale event.
func TestUnrelatedEventsCarryNoThrottleFields(t *testing.T) {
	dir := t.TempDir()
	sink := NewSink(dir, 0, nil)

	sink.Append(Event{Kind: EventCheckRan, Medium: "github", Batch: 12})
	sink.Append(Event{Kind: EventWakeDelivered, Ref: "pyxis#968"})
	sink.Append(Event{Kind: EventStaleFlagged, Subject: "await", AgeS: 90, BoundS: 30})

	data, err := os.ReadFile(filepath.Join(dir, "current.jsonl"))
	if err != nil {
		t.Fatalf("read sink: %v", err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		for _, absent := range []string{"reset_at", "queue_depth", "deferred_ms", "ledger", "0001-01-01"} {
			if strings.Contains(line, absent) {
				t.Errorf("an unrelated event carries %q:\n%s", absent, line)
			}
		}
	}
}
