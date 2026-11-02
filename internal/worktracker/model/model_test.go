package model_test

import (
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/worktracker/model"
)

func TestClosedValueSets(t *testing.T) {
	if !model.ValidType(model.TypeTask) || model.ValidType(model.Type("saga")) {
		t.Fatal("type validity wrong")
	}
	if !model.ValidState(model.StateOpen) || model.ValidState(model.State("frozen")) {
		t.Fatal("state validity wrong")
	}
	// close_reason and priority are optional: empty is valid, out-of-set is not.
	if !model.ValidCloseReason(model.CloseReasonNone) || model.ValidCloseReason(model.CloseReason("nope")) {
		t.Fatal("close_reason validity wrong")
	}
	if !model.ValidPriority(model.PriorityUnset) || !model.ValidPriority(model.PriorityHigh) || model.ValidPriority(model.Priority("critical")) {
		t.Fatal("priority validity wrong")
	}
}

// View always presents the schema tag and never-null slices, so a consumer
// reads a stable shape.
func TestViewShape(t *testing.T) {
	w := model.WorkItem{ID: 3, Type: model.TypeTask, State: model.StateOpen, Title: "t",
		Created: time.Date(2026, 7, 15, 9, 0, 0, 0, time.UTC), Updated: time.Date(2026, 7, 15, 9, 0, 0, 0, time.UTC)}
	v := w.View()
	if v.Schema != model.SchemaVersion {
		t.Fatalf("schema tag = %q", v.Schema)
	}
	if v.Signals == nil || v.Assignees == nil || v.BlockedBy == nil || v.Log == nil {
		t.Fatal("slices must render as empty arrays, never null")
	}
	if v.Created != "2026-07-15T09:00:00Z" {
		t.Fatalf("timestamp not RFC 3339 UTC: %q", v.Created)
	}
}
