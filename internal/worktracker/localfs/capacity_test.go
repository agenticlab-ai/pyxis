package localfs

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/worktracker/model"
)

// The T10 capacity fixture verifies query results at NFR-2's 10,000 items.
// Latencies are recorded as advisory samples in every build mode (AP-29).
// A sample, including the minimum of repeated runs, does not establish p95.
//
// The fixture is written straight to disk rather than through Create: the suite
// measures query latency at capacity, not create latency, and the store's
// per-create directory rescan is O(n²) — building 10k items through the port
// takes ~30s and would swamp the test with setup unrelated to what it proves.
// Direct Render+write produces the exact bytes Create would (Render is the
// store's own serializer), so the queries read a genuine corpus.

const (
	capacityN = 10000 // the NFR-2 capacity target

)

func TestCapacityQueries(t *testing.T) {
	s := seedCapacityFixture(t, capacityN)
	ctx := context.Background()
	queries := []struct {
		name string
		run  func() ([]model.WorkItem, []schema.Defect)
		want int
	}{
		{"list", func() ([]model.WorkItem, []schema.Defect) { return s.List(ctx, model.Query{}) }, capacityN},
		{"search", func() ([]model.WorkItem, []schema.Defect) { return s.Search(ctx, capNeedle) }, 1},
		{"ready", func() ([]model.WorkItem, []schema.Defect) { return s.ReadySet(ctx) }, capReadyCount},
		{"triage", func() ([]model.WorkItem, []schema.Defect) { return s.TriageSet(ctx) }, capTriageCount},
	}
	for _, q := range queries {
		t.Run(q.name, func(t *testing.T) {
			start := time.Now()
			items, defects := q.run()
			t.Logf("%s over %d items: %v; advisory single sample (AP-29)", q.name, capacityN, time.Since(start))
			if len(defects) != 0 || len(items) != q.want {
				t.Fatalf("got %d items (defects %v), want %d", len(items), defects, q.want)
			}
		})
	}
	start := time.Now()
	w, defects := s.Get(ctx, capacityN/2)
	t.Logf("single-item get over %d items: %v; advisory single sample (AP-29)", capacityN, time.Since(start))
	if len(defects) != 0 || w.ID != capacityN/2 {
		t.Fatalf("get: item=%+v defects=%v", w, defects)
	}
}

// The fixture's planted counts and the unique search needle, so the correctness
// anchors read by name. One in fifty items is a ready leaf (open task carrying
// the agent authorization signal); the needle sits in exactly one body.
const (
	capReadyEvery  = 50
	capReadyCount  = capacityN / capReadyEvery
	capTriageCount = 9320 // all open leaves: 10,000 minus 265 epics and 415 closed tasks
	capNeedle      = "xylophone-marker-zzq"
)

// seedCapacityFixture writes n item files straight to a fresh store's items
// directory and returns the opened store. The mix is realistic: mostly plain
// open tasks, one in fifty an agent-authorized ready leaf, a scattering of
// epics (structural rollups the predicates must exclude) and closed items — so
// ready/triage do real filtering work over the whole corpus, not a trivial scan.
func seedCapacityFixture(t *testing.T, n int) *Store {
	t.Helper()
	root := t.TempDir()
	s := New(root, func() time.Time { return time.Unix(0, 0).UTC() })
	dir := s.itemsDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir items: %v", err)
	}
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for id := 1; id <= n; id++ {
		w := model.WorkItem{
			ID:      id,
			Type:    model.TypeTask,
			State:   model.StateOpen,
			Title:   fmt.Sprintf("capacity item %d", id),
			Body:    fmt.Sprintf("body for item %d", id),
			Created: base.Add(time.Duration(id) * time.Second),
			Updated: base.Add(time.Duration(id) * time.Second),
		}
		switch {
		case id%capReadyEvery == 0:
			w.Signals = []string{"agent"} // open, authorized, unblocked leaf → ready + triage
		case id%37 == 0:
			w.Type = model.TypeEpic // a structural rollup both predicates exclude
		case id%23 == 0:
			w.State = model.StateClosed
			w.CloseReason = model.CloseCompleted
		}
		if id == n/2 {
			w.Body = "the " + capNeedle + " lives here alone"
		}
		if err := os.WriteFile(itemPath(dir, id), Render(w), 0o644); err != nil {
			t.Fatalf("write item %d: %v", id, err)
		}
	}
	return s
}
