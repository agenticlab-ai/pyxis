package carinatracker

import (
	"context"
	"errors"
	"github.com/agenticlab-ai/pyxis/internal/executor/taskspec/taskspectest"
	"strconv"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

// fakeRun records the argv it was handed and replays a configured result, so
// the adapter runs with no real `carina` binary.
type fakeRun struct {
	calls [][]string
	out   []byte
	err   error
}

func (f *fakeRun) run(_ context.Context, args ...string) ([]byte, error) {
	f.calls = append(f.calls, args)
	return f.out, f.err
}

// readyJSON mirrors a real `carina ready --json` payload: one plain task and one
// task whose blocked_by carries both a local id and a cross-repo ref.
func readyJSON(t *testing.T) string {
	return `{"schema":"worktracker.predicate-set/v1","predicate":"ready","items":[
 {"schema":"worktracker.work-item/v1","id":2,"type":"task","state":"open",
  "title":"[T1] Define the closed schemas, the check registry",
  "body":"## Goal\nthe data substrate","blocked_by":[],
  "log":[{"kind":"comment","payload":` + strconv.Quote(taskspectest.Marker(t, "T1")) + `}]},
 {"schema":"worktracker.work-item/v1","id":5,"type":"task","state":"open",
  "title":"[T4] Verbless rung-0 checks","body":"body4","blocked_by":["2","owner/repo#9"],
  "log":[{"kind":"comment","payload":` + strconv.Quote(taskspectest.Marker(t, "T4")) + `}]}
]}`
}

func TestFrontier_MapsReadyItemsToTasks(t *testing.T) {
	fr := &fakeRun{out: []byte(readyJSON(t))}
	tr := New("/store", withRunner(fr.run))

	tasks, err := tr.Frontier(context.Background())
	if err != nil {
		t.Fatalf("Frontier: %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("got %d tasks, want 2", len(tasks))
	}
	if got := strings.Join(fr.calls[0], " "); got != "ready --root /store --json" {
		t.Errorf("argv = %q, want 'ready --root /store --json'", got)
	}

	t1 := tasks[0]
	if t1.ID != "T1" || t1.Number != 2 || !strings.Contains(t1.Title, "Define the closed schemas") {
		t.Errorf("T1 mapping = %+v", t1)
	}
	// Provenance is the trustworthy Carina reference (no bare "#N" that a code
	// host would mis-link to its own issue).
	if t1.SourceRef != "Carina work-item 2" {
		t.Errorf("T1 SourceRef = %q, want 'Carina work-item 2'", t1.SourceRef)
	}
	if !strings.Contains(t1.Brief, "## Goal") {
		t.Errorf("T1 Brief dropped the body charter: %q", t1.Brief)
	}
	if len(t1.BlockedBy) != 0 {
		t.Errorf("T1 BlockedBy = %v, want none", t1.BlockedBy)
	}

	// T4: the numeric local blocker is kept; the cross-repo ref is dropped.
	t4 := tasks[1]
	if t4.ID != "T4" || t4.Number != 5 {
		t.Errorf("T4 mapping = %+v", t4)
	}
	if len(t4.BlockedBy) != 1 || t4.BlockedBy[0] != 2 {
		t.Errorf("T4 BlockedBy = %v, want [2] (cross-repo ref dropped)", t4.BlockedBy)
	}

}

func TestOptionsAndMissingBinaryBoundary(t *testing.T) {
	tr := New("/store", WithBinary("custom-carina"), WithAuthor("operator"), WithRepository("owner/repo"))
	if tr.run == nil || tr.author != "operator" || tr.repository != "owner/repo" {
		t.Fatalf("configured tracker = %+v", tr)
	}
	run := shellCarina("/definitely/missing/carina")
	if _, err := run(t.Context(), "ready"); err == nil || !strings.Contains(err.Error(), "carina ready") {
		t.Fatalf("missing binary error = %v", err)
	}
}

func TestFrontier_SkipsRollupAndClosed(t *testing.T) {
	js := `{"predicate":"ready","items":[
	  {"id":1,"type":"epic","state":"open","title":"[Epic] x","body":"b"},
	  {"id":9,"type":"task","state":"closed","title":"[T9] y","body":"b"},
	  {"id":2,"type":"task","state":"open","title":"[T1] keep","body":"b",
	   "log":[{"kind":"comment","payload":` + strconv.Quote(taskspectest.Marker(t, "T1")) + `}]}]}`
	fr := &fakeRun{out: []byte(js)}
	tr := New("/s", withRunner(fr.run))

	tasks, err := tr.Frontier(context.Background())
	if err != nil {
		t.Fatalf("Frontier: %v", err)
	}
	if len(tasks) != 1 || tasks[0].ID != "T1" {
		t.Fatalf("want only the open leaf T1, got %+v", tasks)
	}
}

func TestFrontier_MissingCommentSpecFailsClosed(t *testing.T) {
	js := `{"predicate":"ready","items":[
	  {"id":2,"type":"task","state":"open","title":"[T1] no spec","body":"b","log":[]}]}`
	fr := &fakeRun{out: []byte(js)}
	_, err := New("/s", withRunner(fr.run)).Frontier(context.Background())
	if err == nil || !strings.Contains(err.Error(), "no fs-spec:v2 capsule") {
		t.Fatalf("Frontier error = %v, want missing-capsule refusal", err)
	}
}

func TestFrontier_DecisionRecordCannotMasqueradeAsSpec(t *testing.T) {
	js := `{"predicate":"ready","items":[
	  {"id":2,"type":"task","state":"open","title":"[T1] no comment spec","body":"b",
	   "log":[{"kind":"decision-record","payload":` + strconv.Quote(taskspectest.Marker(t, "T1")) + `}]}]}`
	fr := &fakeRun{out: []byte(js)}
	_, err := New("/s", withRunner(fr.run)).Frontier(context.Background())
	if err == nil || !strings.Contains(err.Error(), "no fs-spec:v2 capsule") {
		t.Fatalf("Frontier error = %v, want non-comment capsule refusal", err)
	}
}

func TestFrontier_RunError_FailsClosed(t *testing.T) {
	fr := &fakeRun{err: errors.New("carina ready: store locked")}
	tr := New("/s", withRunner(fr.run))
	if _, err := tr.Frontier(context.Background()); err == nil {
		t.Fatal("want a fail-closed error when the run seam fails")
	}
}

func TestFrontier_MalformedOutput(t *testing.T) {
	fr := &fakeRun{out: []byte("<html>unicorn</html>")}
	tr := New("/s", withRunner(fr.run))
	if _, err := tr.Frontier(context.Background()); err == nil {
		t.Fatal("want a decode error on non-JSON output")
	}
}

func TestComment_Argv(t *testing.T) {
	fr := &fakeRun{out: []byte("{}")}
	tr := New("/store", withRunner(fr.run), WithAuthor("argus"))
	if err := tr.Comment(context.Background(), 2, "drove to green"); err != nil {
		t.Fatalf("Comment: %v", err)
	}
	got := strings.Join(fr.calls[0], " ")
	want := "comment 2 --root /store --body drove to green --author argus"
	if got != want {
		t.Errorf("argv = %q, want %q", got, want)
	}
}

func TestComment_ErrorPropagates(t *testing.T) {
	fr := &fakeRun{err: errors.New("no such item")}
	tr := New("/s", withRunner(fr.run))
	if err := tr.Comment(context.Background(), 99, "x"); err == nil {
		t.Fatal("want error when the comment fails")
	}
}

func TestSplitTaskTitle(t *testing.T) {
	for _, c := range []struct{ in, id, title string }{
		{"[T1] Define X", "T1", "Define X"},
		{"[T10] Multi-stack", "T10", "Multi-stack"},
		{"no tag here", "", "no tag here"},
		{"  [T2]  trimmed  ", "T2", "trimmed"},
	} {
		id, title := splitTaskTitle(c.in)
		if id != c.id || title != c.title {
			t.Errorf("split(%q) = (%q,%q), want (%q,%q)", c.in, id, title, c.id, c.title)
		}
	}
}

func TestParseRefs(t *testing.T) {
	got := parseRefs([]string{"2", "owner/repo#9", " 3 ", "not-a-number"})
	if len(got) != 2 || got[0] != 2 || got[1] != 3 {
		t.Errorf("parseRefs = %v, want [2 3]", got)
	}
	if parseRefs(nil) != nil {
		t.Error("parseRefs(nil) should be nil")
	}
}

var _ port.WorkTracker = (*Tracker)(nil)

// scopedReadyJSON is a store-wide ready set spanning TWO plans (matching the
// harbormaster-dialect plan-task marker the projection stamps into each body):
// self-drive-wake/001 and other-feature/001. A plan-scoped frontier admits
// only the first plan's tasks — the exact leak the shared routing label
// otherwise allows, closed here identically to the GitHub adapter (#851).
func scopedReadyJSON(t *testing.T) string {
	return `{"schema":"worktracker.predicate-set/v1","predicate":"ready","items":[
 {"schema":"worktracker.work-item/v1","id":10,"type":"task","state":"open",
  "title":"[T1] Mine","body":"charter\n<!-- pyxis:plan-task self-drive-wake/001/T1 -->","blocked_by":[],
  "log":[{"kind":"comment","payload":` + strconv.Quote(taskspectest.Marker(t, "T1")) + `}]},
 {"schema":"worktracker.work-item/v1","id":11,"type":"task","state":"open",
  "title":"[T2] Also mine","body":"c\n<!-- pyxis:plan-task self-drive-wake/001/T2 -->","blocked_by":["10"],
  "log":[{"kind":"comment","payload":` + strconv.Quote(taskspectest.Marker(t, "T2")) + `}]},
 {"schema":"worktracker.work-item/v1","id":20,"type":"task","state":"open",
  "title":"[T1] Not mine","body":"c\n<!-- pyxis:plan-task other-feature/001/T1 -->","blocked_by":[],
  "log":[{"kind":"comment","payload":` + strconv.Quote(taskspectest.Marker(t, "T1")) + `}]},
 {"schema":"worktracker.work-item/v1","id":30,"type":"task","state":"open",
  "title":"[T9] Unmarked","body":"a task with no plan-task marker at all","blocked_by":[],
  "log":[{"kind":"comment","payload":` + strconv.Quote(taskspectest.Marker(t, "T9")) + `}]}
]}`
}

func TestNewForPlan_ScopesFrontierToOnePlan(t *testing.T) {
	fr := &fakeRun{out: []byte(scopedReadyJSON(t))}
	tr, err := NewForPlan("/store", "self-drive-wake/001", withRunner(fr.run))
	if err != nil {
		t.Fatalf("NewForPlan: %v", err)
	}
	tasks, err := tr.Frontier(context.Background())
	if err != nil {
		t.Fatalf("Frontier: %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("scoped frontier = %d tasks, want 2 (only self-drive-wake/001)", len(tasks))
	}
	for _, task := range tasks {
		if task.Number != 10 && task.Number != 11 {
			t.Errorf("frontier admitted a foreign item: %+v", task)
		}
	}

}

func TestNewForPlan_UnscopedSeesAll(t *testing.T) {
	fr := &fakeRun{out: []byte(scopedReadyJSON(t))}
	tr := New("/store", withRunner(fr.run)) // unscoped
	tasks, err := tr.Frontier(context.Background())
	if err != nil {
		t.Fatalf("Frontier: %v", err)
	}
	if len(tasks) != 4 {
		t.Fatalf("unscoped frontier = %d, want 4 (whole store, incl. the unmarked task)", len(tasks))
	}
}

func TestNewForPlan_RejectsBadPlanRef(t *testing.T) {
	if _, err := NewForPlan("/store", "not-a-plan-ref"); err == nil {
		t.Fatal("NewForPlan accepted an invalid plan reference")
	}
	if _, err := NewForPlan("/store", "feature/1"); err == nil {
		t.Fatal("NewForPlan accepted a non-dense version")
	}
}
