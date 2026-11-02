package localfs

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/worktracker/model"
)

// fixedClock stamps every write with one deterministic instant so tests are
// hermetic (no wall clock; AGENTS.md test rules).
func fixedClock() func() time.Time {
	at := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	return func() time.Time { return at }
}

// advancingClock returns a deterministic clock that steps forward one second
// per call — hermetic, but with enough resolution that a later mutation's
// updated stamp is observably past an earlier one (the lifecycle rows need to
// see the stamp move). Single-goroutine callers only.
func advancingClock() func() time.Time {
	at := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	return func() time.Time {
		t := at
		at = at.Add(time.Second)
		return t
	}
}

func TestCreateDefaultsAndRoundTrip(t *testing.T) {
	s := New(t.TempDir(), fixedClock())
	ctx := context.Background()
	w, defects := s.Create(ctx, model.CreateSpec{Title: "no type given"})
	if len(defects) > 0 {
		t.Fatalf("create: %v", defects)
	}
	if w.ID != 1 || w.Type != model.TypeTask || w.State != model.StateOpen {
		t.Fatalf("create defaults wrong: %+v", w)
	}
	// The rendered file is human-readable and re-parses to the same item.
	raw, err := os.ReadFile(itemPath(s.itemsDir(), 1))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(raw), "title: no type given") || !strings.Contains(string(raw), "## Log") {
		t.Fatalf("on-disk form not legible:\n%s", raw)
	}
	got, defects := Parse("show", "x", raw)
	if len(defects) > 0 || got.Title != w.Title || got.Type != w.Type {
		t.Fatalf("round-trip: %v %+v", defects, got)
	}
}

func TestCreateRejectsEmptyTitle(t *testing.T) {
	s := New(t.TempDir(), fixedClock())
	_, defects := s.Create(context.Background(), model.CreateSpec{Title: "   "})
	if len(defects) == 0 || !strings.Contains(defects[0].Reason, "title is empty") {
		t.Fatalf("empty title not rejected: %v", defects)
	}
	// No file was written (fail closed).
	if _, err := os.Stat(itemPath(s.itemsDir(), 1)); !os.IsNotExist(err) {
		t.Fatalf("a file was written despite the rejected create")
	}
}

func TestGetMissing(t *testing.T) {
	s := New(t.TempDir(), fixedClock())
	_, defects := s.Get(context.Background(), 7)
	if len(defects) != 1 || defects[0].Op != "resolve" || !strings.Contains(defects[0].Reason, "no work item") {
		t.Fatalf("missing item defect wrong: %v", defects)
	}
	if defects[0].Verb != "show" {
		t.Fatalf("defect verb not stamped: %+v", defects[0])
	}
}

// A crashed create leaves a zero-byte reserved file: the id is a permanent gap
// (Get reports it absent) and is never reused by the next create (§3.3).
func TestCrashedCreateLeavesPermanentGap(t *testing.T) {
	s := New(t.TempDir(), fixedClock())
	ctx := context.Background()
	if _, defects := s.Create(ctx, model.CreateSpec{Title: "one"}); len(defects) > 0 {
		t.Fatalf("create 1: %v", defects)
	}
	// Simulate a create that reserved id 2 then crashed before filling it.
	id, err := s.reserveID(2)
	if err != nil || id != 2 {
		t.Fatalf("reserve: id=%d err=%v", id, err)
	}
	if _, defects := s.Get(ctx, 2); len(defects) != 1 || !strings.Contains(defects[0].Reason, "empty placeholder") {
		t.Fatalf("empty reserved file not reported absent: %v", defects)
	}
	// The next create must skip the gap, never reuse id 2.
	w, defects := s.Create(ctx, model.CreateSpec{Title: "three"})
	if len(defects) > 0 || w.ID != 3 {
		t.Fatalf("gap reused or skipped wrong: id=%d defects=%v", w.ID, defects)
	}
}

func TestParseRejectsOutOfSetEnum(t *testing.T) {
	good := Render(model.WorkItem{
		ID: 1, Type: model.TypeTask, State: model.StateOpen, Title: "t",
		Created: time.Now().UTC(), Updated: time.Now().UTC(),
	})
	bad := strings.Replace(string(good), "type: task", "type: saga", 1)
	_, defects := Parse("show", "items/1.md", []byte(bad))
	if len(defects) == 0 {
		t.Fatalf("out-of-set type accepted")
	}
	if !strings.Contains(defects[0].Reason, "out of set") || defects[0].File != "items/1.md" {
		t.Fatalf("enum defect wrong: %+v", defects[0])
	}
}

func TestParseRejectsUnknownKey(t *testing.T) {
	good := Render(model.WorkItem{
		ID: 1, Type: model.TypeTask, State: model.StateOpen, Title: "t",
		Created: time.Now().UTC(), Updated: time.Now().UTC(),
	})
	bad := strings.Replace(string(good), "title: t\n", "title: t\nmood: cheerful\n", 1)
	_, defects := Parse("show", "items/1.md", []byte(bad))
	found := false
	for _, d := range defects {
		if strings.Contains(d.Reason, `unknown header key "mood"`) {
			found = true
		}
	}
	if !found {
		t.Fatalf("unknown key not rejected: %v", defects)
	}
}
