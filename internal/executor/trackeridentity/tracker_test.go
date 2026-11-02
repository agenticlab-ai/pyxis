package trackeridentity

import (
	"context"
	"errors"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/integrations"
)

type fakeTracker struct {
	task             port.Task
	err              error
	comments, closed int
}

func (f *fakeTracker) Frontier(context.Context) ([]port.Task, error) {
	return []port.Task{f.task}, f.err
}
func (f *fakeTracker) Comment(context.Context, int, string) error { f.comments++; return f.err }
func (f *fakeTracker) Close(context.Context, int) error           { f.closed++; return f.err }

type withoutClose struct{ port.WorkTracker }

func TestWrapQualifiesOnlySourceAndPreservesCapabilities(t *testing.T) {
	ctx := context.Background()
	inner := &fakeTracker{task: port.Task{ID: "T1", Number: 3, SourceRef: "#3", Title: "title"}}
	binding := integrations.ResolvedRepository{Provider: "github", BaseURL: "https://github.com", Repository: "o/r"}
	wrapped := Wrap(inner, binding)
	tasks, err := wrapped.Frontier(ctx)
	if err != nil || tasks[0].Number != 3 || tasks[0].ID != "T1" || tasks[0].SourceRef != "https://github.com/o/r/issues/3" || tasks[0].SourceKey != binding.Key() {
		t.Fatalf("tasks=%+v err=%v", tasks, err)
	}
	if err := wrapped.Comment(ctx, 3, "note"); err != nil || inner.comments != 1 {
		t.Fatal("comment not delegated")
	}
	if err := wrapped.Close(ctx, 3); err != nil || inner.closed != 1 {
		t.Fatal("close not delegated")
	}
	if err := Wrap(withoutClose{inner}, binding).Close(ctx, 3); err == nil {
		t.Fatal("unsupported close accepted")
	}
	inner.task.SourceKey = "another-host"
	if _, err := wrapped.Frontier(ctx); err == nil {
		t.Fatal("cross-host task accepted")
	}
	inner.err = errors.New("offline")
	if _, err := wrapped.Frontier(ctx); !errors.Is(err, inner.err) {
		t.Fatal("frontier error swallowed")
	}
	carina := integrations.ResolvedRepository{Provider: "carina", Root: "/repo"}
	if got := SourceRef(carina, 3); got != "carina:/repo#3" {
		t.Fatalf("Carina ref=%q", got)
	}
}
