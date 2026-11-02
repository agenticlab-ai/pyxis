package report

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"
)

type lifecycleHandler struct {
	id       string
	kinds    []Kind
	after    []string
	failures int
	calls    []string
	shared   *[]string
}

func (h *lifecycleHandler) ID() string      { return h.id }
func (h *lifecycleHandler) Kinds() []Kind   { return h.kinds }
func (h *lifecycleHandler) After() []string { return h.after }
func (h *lifecycleHandler) Handle(_ context.Context, e Event) error {
	h.calls = append(h.calls, e.ID)
	if h.shared != nil {
		*h.shared = append(*h.shared, h.id)
	}
	if h.failures > 0 {
		h.failures--
		return errors.New("temporary outage")
	}
	return nil
}

func TestDurableHandlersRetryIndependentlyAcrossRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deliveries.json")
	event := TaskCompleted("task:T1:completed:abc", "T1", 42, "widgets/001", "abc")
	tracker := &lifecycleHandler{id: "tracker.close", kinds: []Kind{KindTaskCompleted}, failures: 1}
	cleanup := &lifecycleHandler{id: "workspace.cleanup", kinds: []Kind{KindTaskCompleted}}

	r, err := NewDurableReporter("run-1", path, nil, []Handler{tracker, cleanup})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Publish(t.Context(), event); err == nil {
		t.Fatal("Publish succeeded despite tracker outage")
	}
	if len(tracker.calls) != 1 || len(cleanup.calls) != 1 {
		t.Fatalf("first delivery calls tracker=%v cleanup=%v", tracker.calls, cleanup.calls)
	}

	// A fresh Reporter recovers the pending tracker handler. The already
	// successful cleanup is not repeated.
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	restarted, err := NewDurableReporter("run-2", path, nil, []Handler{tracker, cleanup})
	if err != nil {
		t.Fatal(err)
	}
	if err := restarted.Resume(t.Context()); err != nil {
		t.Fatal(err)
	}
	if len(tracker.calls) != 2 || len(cleanup.calls) != 1 {
		t.Fatalf("recovered calls tracker=%v cleanup=%v", tracker.calls, cleanup.calls)
	}
	want := []HandlerOutcome{
		{EventID: event.ID, EventKind: KindTaskCompleted, HandlerID: "tracker.close", Status: "completed", Attempts: 2},
		{EventID: event.ID, EventKind: KindTaskCompleted, HandlerID: "workspace.cleanup", Status: "completed", Attempts: 1},
	}
	got := restarted.HandlerOutcomes()
	for i := range got {
		got[i].UpdatedAt = want[i].UpdatedAt
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("outcomes = %#v, want %#v", got, want)
	}
}

func TestDurableHandlersHonorDependenciesAndDeduplicateEvents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deliveries.json")
	var order []string
	evidence := &lifecycleHandler{id: "evidence.record", kinds: []Kind{KindTaskCompleted}, failures: 1, shared: &order}
	cleanup := &lifecycleHandler{id: "workspace.cleanup", kinds: []Kind{KindTaskCompleted}, after: []string{"evidence.record"}, shared: &order}
	r, err := NewDurableReporter("run", path, nil, []Handler{cleanup, evidence})
	if err != nil {
		t.Fatal(err)
	}
	event := TaskCompleted("task:T2:completed:def", "T2", 43, "widgets/001", "def")
	if err := r.Publish(t.Context(), event); err == nil {
		t.Fatal("Publish succeeded despite evidence failure")
	}
	if !reflect.DeepEqual(order, []string{"evidence.record"}) {
		t.Fatalf("cleanup ran before evidence: %v", order)
	}
	if err := r.Resume(t.Context()); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(order, []string{"evidence.record", "evidence.record", "workspace.cleanup"}) {
		t.Fatalf("dependency order = %v", order)
	}
	if err := r.Publish(t.Context(), event); err != nil {
		t.Fatal(err)
	}
	if len(order) != 3 {
		t.Fatalf("duplicate event repeated completed handlers: %v", order)
	}
}

func TestDurableHandlerRegistrationFailsClosed(t *testing.T) {
	kind := []Kind{KindPlanCompleted}
	a := &lifecycleHandler{id: "a", kinds: kind, after: []string{"b"}}
	b := &lifecycleHandler{id: "b", kinds: kind, after: []string{"a"}}
	if _, err := NewDurableReporter("run", filepath.Join(t.TempDir(), "cycle.json"), nil, []Handler{a, b}); err == nil {
		t.Fatal("dependency cycle accepted")
	}
	unknown := &lifecycleHandler{id: "known", kinds: kind, after: []string{"missing"}}
	if _, err := NewDurableReporter("run", filepath.Join(t.TempDir(), "unknown.json"), nil, []Handler{unknown}); err == nil {
		t.Fatal("unknown dependency accepted")
	}
	duplicate := &lifecycleHandler{id: "same", kinds: kind}
	if _, err := NewDurableReporter("run", filepath.Join(t.TempDir(), "duplicate.json"), nil, []Handler{duplicate, duplicate}); err == nil {
		t.Fatal("duplicate handler identity accepted")
	}
}

func TestDurablePublicationRequiresStableEventIdentity(t *testing.T) {
	h := &lifecycleHandler{id: "tracker.close", kinds: []Kind{KindTaskCompleted}}
	r, err := NewDurableReporter("run", filepath.Join(t.TempDir(), "deliveries.json"), nil, []Handler{h})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Publish(t.Context(), TaskCompleted("", "T1", 1, "f/001", "abc")); err == nil {
		t.Fatal("durable publication accepted an event without a stable identity")
	}
	if len(h.calls) != 0 {
		t.Fatal("handler ran for an event that was not durably identified")
	}
}

func TestLifecycleEventConstructorsKeepCompletionMeaningsDistinct(t *testing.T) {
	implementation := TaskImplementationCompleted("i", "T1", 1, "f/001", "head", "#7")
	delivered := TaskCompleted("d", "T1", 1, "f/001", "landed")
	plan := PlanCompleted("p", "f/001", "landed")
	if implementation.Kind != KindTaskImplementationCompleted || delivered.Kind != KindTaskCompleted || plan.Kind != KindPlanCompleted {
		t.Fatalf("lifecycle kinds collapsed: %q %q %q", implementation.Kind, delivered.Kind, plan.Kind)
	}
	if implementation.ProposalRef != "#7" || delivered.Revision != "landed" || plan.TaskID != "" {
		t.Fatalf("lifecycle payloads = %#v %#v %#v", implementation, delivered, plan)
	}
}
