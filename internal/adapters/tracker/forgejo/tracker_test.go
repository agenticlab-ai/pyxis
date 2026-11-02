package forgejotracker

import (
	"context"
	"fmt"
	"github.com/agenticlab-ai/pyxis/internal/executor/taskspec/taskspectest"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/adapters/common/forgejo"
)

type trackerTransport struct {
	responses map[string]string
	failures  map[string]int
	calls     []string
}

func (r *trackerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	key := req.Method + " " + req.URL.Path
	r.calls = append(r.calls, key)
	status := 200
	if failed := r.failures[key]; failed != 0 {
		status = failed
	}
	body := r.responses[key]
	if req.URL.Query().Get("page") != "" && req.URL.Query().Get("page") != "1" {
		body = "[]"
	}
	if body == "" {
		body = "[]"
	}
	return &http.Response{StatusCode: status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
}

func newTestTracker(t *testing.T, base, label, plan string) (*Tracker, *trackerTransport) {
	t.Helper()
	r := &trackerTransport{responses: map[string]string{}, failures: map[string]int{}}
	client, err := forgejo.NewWithHTTP(base, "", &http.Client{Transport: r})
	if err != nil {
		t.Fatal(err)
	}
	tr, err := New(client, "o/r", label, plan)
	if err != nil {
		t.Fatal(err)
	}
	return tr, r
}

func seedFrontier(t *testing.T, r *trackerTransport) {
	r.responses["GET /api/v1/repos/o/r/issues"] = `[
	{"number":1,"title":"[T1] target","body":"<!-- pyxis:plan-task feature/001/T1 -->","state":"open","labels":[{"name":"local-agent"}]},
	{"number":2,"title":"[T2] another plan","body":"<!-- pyxis:plan-task other/001/T2 -->","state":"open","labels":[{"name":"local-agent"}]},
	{"number":3,"title":"closed","state":"closed","labels":[{"name":"local-agent"}]},
	{"number":4,"title":"untagged","state":"open"},
	{"number":5,"title":"proposal","state":"open","labels":[{"name":"local-agent"}],"pull_request":{}}
	]`
	r.responses["GET /api/v1/repos/o/r/issues/1/comments"] = `[{"id":1,"body":` + strconv.Quote(taskspectest.Marker(t, "T1")) + `}]`
	r.responses["GET /api/v1/repos/o/r/issues/2/comments"] = `[{"id":1,"body":` + strconv.Quote(taskspectest.Marker(t, "T1")) + `}]`
	r.responses["GET /api/v1/repos/o/r/issues/1/dependencies"] = `[{"number":9}]`
}

func TestFrontierScopeAndHostIdentity(t *testing.T) {
	tr, r := newTestTracker(t, "http://127.0.0.1:3000", "", "feature/001")
	seedFrontier(t, r)
	tasks, err := tr.Frontier(context.Background())
	if err != nil || len(tasks) != 1 {
		t.Fatalf("Frontier=%+v %v", tasks, err)
	}
	task := tasks[0]
	if task.ID != "T1" || task.Title != "target" || task.SourceRef != "http://127.0.0.1:3000/o/r/issues/1" || task.SourceKey != "forgejo:http://127.0.0.1:3000:o/r" || !reflect.DeepEqual(task.BlockedBy, []int{9}) || task.DemandID == "" {
		t.Fatalf("bad task mapping: %+v", task)
	}
	for _, call := range r.calls {
		if strings.Contains(call, "/issues/2/") {
			t.Fatalf("out-of-plan capsule read: %s", call)
		}
	}
	other, transport := newTestTracker(t, "http://127.0.0.1:3001", "local-agent", "feature/001")
	seedFrontier(t, transport)
	otherTasks, err := other.Frontier(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if otherTasks[0].DemandID == task.DemandID || otherTasks[0].ScopeToken() == task.ScopeToken() {
		t.Fatal("same numbers on different hosts shared identity")
	}
}

func TestUnscopedFrontierAndInvalidInputs(t *testing.T) {
	tr, r := newTestTracker(t, "http://127.0.0.1:3000", "local-agent", "")
	seedFrontier(t, r)
	r.responses["GET /api/v1/repos/o/r/issues"] = `[{"number":1,"title":"No prefix","state":"open","labels":[{"name":"local-agent"}]}]`
	tasks, err := tr.Frontier(context.Background())
	if err != nil || len(tasks) != 1 || tasks[0].ID != "T1" || tasks[0].Title != "No prefix" {
		t.Fatalf("unscoped = %+v %v", tasks, err)
	}
	if _, err := New(nil, "o/r", "", ""); err == nil {
		t.Fatal("nil client accepted")
	}
	client, err := forgejo.New("http://127.0.0.1:3000", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, args := range [][2]string{{"bad", ""}, {"o/r", "feature/01"}, {"o/r", "feature/001/T1"}} {
		if _, err := New(client, args[0], "", args[1]); err == nil {
			t.Fatalf("invalid input accepted: %v", args)
		}
	}
	for _, title := range []string{"[T2] Hello", "Plain", "[] Empty", "[invalid"} {
		id, trimmed := splitTaskTitle(title)
		if trimmed == "" {
			t.Fatalf("empty parsed title %q (%q)", title, id)
		}
	}
}

func TestFrontierAndCommentFailuresRemainErrors(t *testing.T) {
	for _, path := range []string{"/issues", "/issues/1/comments", "/issues/1/dependencies"} {
		t.Run(path, func(t *testing.T) {
			tr, r := newTestTracker(t, "http://127.0.0.1:3000", "", "feature/001")
			seedFrontier(t, r)
			r.failures["GET /api/v1/repos/o/r"+path] = 403
			if _, err := tr.Frontier(context.Background()); err == nil {
				t.Fatal("read failure accepted")
			}
		})
	}
	tr, r := newTestTracker(t, "http://127.0.0.1:3000", "", "feature/001")
	seedFrontier(t, r)
	r.responses["GET /api/v1/repos/o/r/issues/1/comments"] = `[{"body":"<!-- fs-spec:v1 {\"goal\":\"build\"} -->"}]`
	if _, err := tr.Frontier(context.Background()); err == nil {
		t.Fatal("malformed task capsule accepted")
	}
	if err := tr.Comment(context.Background(), 1, "progress"); err != nil {
		t.Fatal(err)
	}
	r.failures["POST /api/v1/repos/o/r/issues/1/comments"] = 403
	if err := tr.Comment(context.Background(), 1, "progress"); err == nil {
		t.Fatal("comment failure swallowed")
	}
	r.responses["GET /api/v1/repos/o/r/issues/1"] = `{"number":1,"state":"closed"}`
	if err := tr.Close(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	r.failures["GET /api/v1/repos/o/r/issues/1"] = 404
	if err := tr.Close(context.Background(), 1); err == nil {
		t.Fatal("unknown task closed")
	}
	if fmt.Sprint(r.calls) == "" {
		t.Fatal("transport unused")
	}
}
