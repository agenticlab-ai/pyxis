package forgejotracker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/adapters/common/forgejo"
	"github.com/agenticlab-ai/pyxis/internal/executiondemand/conformancetest"
)

type forgejoTestComment struct {
	ID   int64  `json:"id"`
	Body string `json:"body"`
}

type forgejoTestServer struct {
	mu       sync.Mutex
	labels   []forgejoLabel
	issues   map[int]forgejoIssue
	comments map[int][]forgejoTestComment
	blockers map[int][]forgejoIssue
	failures map[string]int
	bodies   map[string]string
	calls    []string
	posts    []string
}

func newForgejoTest(t *testing.T) (*Forgejo, *forgejoTestServer) {
	t.Helper()
	s := &forgejoTestServer{issues: map[int]forgejoIssue{}, comments: map[int][]forgejoTestComment{}, blockers: map[int][]forgejoIssue{}, failures: map[string]int{}, bodies: map[string]string{}}
	client, err := forgejo.NewWithHTTP("http://127.0.0.1:3000/forge", "test-token", &http.Client{Transport: s})
	if err != nil {
		t.Fatal(err)
	}
	return NewForgejo(client, "o/r"), s
}

func (s *forgejoTestServer) RoundTrip(r *http.Request) (*http.Response, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := r.Context().Err(); err != nil {
		return nil, err
	}
	if r.Header.Get("Authorization") != "token test-token" {
		return nil, fmt.Errorf("missing selected-provider credential")
	}
	if !strings.HasPrefix(r.URL.Path, "/forge/api/v1/repos/o/r/") {
		return nil, fmt.Errorf("wrong configured host path %s", r.URL.Path)
	}
	path := strings.TrimPrefix(r.URL.Path, "/forge/api/v1/repos/o/r")
	key := r.Method + " " + path
	s.calls = append(s.calls, key)
	if r.Method != http.MethodGet {
		s.posts = append(s.posts, key)
	}
	respond := func(status int, value any) (*http.Response, error) {
		body, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		return &http.Response{StatusCode: status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(string(body))), Request: r}, nil
	}
	if status := s.failures[key]; status != 0 {
		return respond(status, struct{}{})
	}
	if body, ok := s.bodies[key]; ok {
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	}
	if r.Method == http.MethodGet && r.URL.Query().Get("page") != "" && r.URL.Query().Get("page") != "1" {
		return respond(200, []struct{}{})
	}
	decode := func(out any) error { return json.NewDecoder(r.Body).Decode(out) }
	if path == "/labels" {
		if r.Method == http.MethodGet {
			return respond(200, s.labels)
		}
		var label forgejoLabel
		if err := decode(&label); err != nil {
			return nil, err
		}
		label.ID = int64(len(s.labels) + 1)
		s.labels = append(s.labels, label)
		return respond(201, label)
	}
	if path == "/issues" {
		if r.Method == http.MethodGet {
			var issues []forgejoIssue
			for _, issue := range s.issues {
				issues = append(issues, issue) // deliberately over-return filters
			}
			sort.Slice(issues, func(i, j int) bool { return issues[i].Number < issues[j].Number })
			return respond(200, issues)
		}
		var body struct {
			Title  string  `json:"title"`
			Body   string  `json:"body"`
			Labels []int64 `json:"labels"`
		}
		if err := decode(&body); err != nil {
			return nil, err
		}
		number := len(s.issues) + 1
		issue := forgejoIssue{Number: number, Title: body.Title, Body: body.Body, State: "open"}
		for _, id := range body.Labels {
			for _, label := range s.labels {
				if label.ID == id {
					issue.Labels = append(issue.Labels, label)
				}
			}
		}
		s.issues[number] = issue
		return respond(201, issue)
	}
	parts := strings.Split(strings.TrimPrefix(path, "/issues/"), "/")
	number, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, fmt.Errorf("unexpected endpoint %s", key)
	}
	issue, exists := s.issues[number]
	if !exists {
		return respond(404, struct{}{})
	}
	if len(parts) == 1 {
		if r.Method == http.MethodPatch {
			var body struct {
				State *string `json:"state"`
				Title *string `json:"title"`
				Body  *string `json:"body"`
			}
			if err := decode(&body); err != nil {
				return nil, err
			}
			if body.State != nil {
				issue.State = *body.State
			}
			if body.Title != nil {
				issue.Title = *body.Title
			}
			if body.Body != nil {
				issue.Body = *body.Body
			}
			s.issues[number] = issue
		}
		return respond(200, issue)
	}
	switch parts[1] {
	case "comments":
		if r.Method == http.MethodGet {
			return respond(200, s.comments[number])
		}
		var comment forgejoTestComment
		if err := decode(&comment); err != nil {
			return nil, err
		}
		comment.ID = int64(len(s.comments[number]) + 1)
		s.comments[number] = append(s.comments[number], comment)
		return respond(201, comment)
	case "dependencies":
		if r.Method == http.MethodGet {
			return respond(200, s.blockers[number])
		}
		var body struct {
			Index int    `json:"index"`
			Owner string `json:"owner"`
			Repo  string `json:"repo"`
		}
		if err := decode(&body); err != nil {
			return nil, err
		}
		if body.Owner != "o" || body.Repo != "r" || body.Index == 0 {
			return nil, fmt.Errorf("dependency requires blocker index and repository coordinate: %+v", body)
		}
		if r.Method == http.MethodDelete {
			s.blockers[number] = slices.DeleteFunc(s.blockers[number], func(b forgejoIssue) bool { return b.Number == body.Index })
			return respond(200, s.issues[body.Index])
		}
		s.blockers[number] = append(s.blockers[number], s.issues[body.Index])
		return respond(201, s.issues[body.Index])
	case "labels":
		if r.Method == http.MethodDelete && len(parts) == 3 {
			issue.Labels = slices.DeleteFunc(issue.Labels, func(l forgejoLabel) bool { return strconv.FormatInt(l.ID, 10) == parts[2] })
			s.issues[number] = issue
			return respond(204, struct{}{})
		}
		var body struct {
			Labels []int64 `json:"labels"`
		}
		if err := decode(&body); err != nil {
			return nil, err
		}
		for _, id := range body.Labels {
			for _, label := range s.labels {
				if label.ID == id {
					issue.Labels = append(issue.Labels, label)
				}
			}
		}
		s.issues[number] = issue
		return respond(200, issue.Labels)
	default:
		return nil, fmt.Errorf("unexpected endpoint %s", key)
	}
}

func TestForgejoProjectionRelationshipsAndState(t *testing.T) {
	f, server := newForgejoTest(t)
	if err := f.EnsureLabel("local-agent", "#aabbcc", "routing"); err != nil {
		t.Fatal(err)
	}
	if err := f.EnsureLabel("LOCAL-AGENT", "000000", "new description"); err != nil {
		t.Fatal(err)
	}
	if len(server.labels) != 1 || server.labels[0].Color != "aabbcc" || server.labels[0].Description != "routing" {
		t.Fatalf("existing label changed: %+v", server.labels)
	}
	parent, err := f.CreateIssue("Parent", "original body", nil, "Epic")
	if err != nil {
		t.Fatal(err)
	}
	child, err := f.CreateIssue("[T1] Work", "charter", []string{"local-agent", "LOCAL-AGENT"}, "Task")
	if err != nil {
		t.Fatal(err)
	}
	other, err := f.CreateIssue("[T2] Dependent", "charter two", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := f.AddLabels(other, []string{"local-agent"}); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := f.LinkSubIssue(parent, child); err != nil {
			t.Fatal(err)
		}
		if err := f.LinkSubIssue(parent, other); err != nil {
			t.Fatal(err)
		}
		if err := f.AddBlockedBy(other, child); err != nil {
			t.Fatal(err)
		}
	}
	// Duplicate edge records do not inflate the child set.
	server.comments[parent] = append(server.comments[parent], server.comments[parent][0])
	children, err := f.SubIssues(parent)
	if err != nil || len(children) != 2 || children[0].Type != "Task" {
		t.Fatalf("children = %+v, %v", children, err)
	}
	blocked, err := f.BlockedBy(other)
	if err != nil || !reflect.DeepEqual(blocked, []int{child}) {
		t.Fatalf("dependency direction = %v, %v", blocked, err)
	}
	if len(server.blockers[other]) != 1 || len(server.comments[parent]) != 3 {
		t.Fatal("idempotent relationship operation duplicated a write")
	}
	if err := f.Comment(child, "progress"); err != nil {
		t.Fatal(err)
	}
	if err := f.CloseContext(context.Background(), child); err != nil {
		t.Fatal(err)
	}
	posts := len(server.posts)
	if err := f.CloseContext(context.Background(), child); err != nil || len(server.posts) != posts {
		t.Fatalf("repeated close mutated state: %v", err)
	}
	open, err := f.OpenIssues()
	if err != nil || len(open) != 2 {
		t.Fatalf("open = %+v, %v", open, err)
	}
	if f.SourceRef(child) != "http://127.0.0.1:3000/forge/o/r/issues/2" || f.RepositoryKey() != "forgejo:http://127.0.0.1:3000/forge:o/r" {
		t.Fatal("task identity lost the host port/base path")
	}
}

func TestForgejoIssueEnumerationVerifiesServerFilters(t *testing.T) {
	f, server := newForgejoTest(t)
	server.issues = map[int]forgejoIssue{
		1: {Number: 1, State: "open", Labels: []forgejoLabel{{Name: "local-agent"}}},
		2: {Number: 2, State: "closed", Labels: []forgejoLabel{{Name: "local-agent"}}},
		3: {Number: 3, State: "open"},
		4: {Number: 4, State: "open", Labels: []forgejoLabel{{Name: "local-agent"}}, PullRequest: &struct{}{}},
	}
	issues, err := f.Issues(context.Background(), "open", "local-agent")
	if err != nil || len(issues) != 1 || issues[0].Number != 1 {
		t.Fatalf("filtered issues = %+v, %v", issues, err)
	}
	issues, err = f.Issues(context.Background(), "all", "")
	if err != nil || len(issues) != 3 {
		t.Fatalf("all issues = %+v, %v", issues, err)
	}
	server.comments[1] = []forgejoTestComment{{ID: 3, Body: "third"}, {ID: 1, Body: "first"}, {ID: 2, Body: "second"}}
	comments, err := f.Comments(1)
	if err != nil || !reflect.DeepEqual(comments, []string{"first", "second", "third"}) {
		t.Fatalf("comment ordering = %v, %v", comments, err)
	}
}

func TestForgejoMetadataRejectsCorruptionAndOtherHosts(t *testing.T) {
	for _, payload := range []string{
		forgejoTypeMarker + `{}` + " -->",
		forgejoTypeMarker + `{"schema_version":"pyxis-forgejo-issue/v1","type":"Task","surprise":1}` + " -->",
		forgejoTypeMarker + `{"schema_version":"pyxis-forgejo-issue/v1","type":"Task"} {}` + " -->",
		forgejoTypeMarker + `{` + " -->",
		forgejoTypeMarker + `{}`,
		strings.Repeat(forgejoTypeMarker+`{"schema_version":"pyxis-forgejo-issue/v1","type":"Task"}`+" -->", 2),
	} {
		t.Run(payload, func(t *testing.T) {
			if _, err := (forgejoIssue{Number: 1, Body: payload}).issue(); err == nil {
				t.Fatal("corrupt metadata was accepted")
			}
		})
	}
	f, server := newForgejoTest(t)
	server.issues[1] = forgejoIssue{Number: 1}
	for _, edge := range []forgejoChildEdge{
		{SchemaVersion: "v0", Parent: f.identity(1), Child: f.identity(2)},
		{SchemaVersion: "pyxis-forgejo-child/v1", Parent: f.identity(3), Child: f.identity(2)},
		{SchemaVersion: "pyxis-forgejo-child/v1", Parent: f.identity(1), Child: "forgejo:http://127.0.0.1:3001/forge:o/r#2"},
		{SchemaVersion: "pyxis-forgejo-child/v1", Parent: f.identity(1), Child: f.identity(1)},
		{SchemaVersion: "pyxis-forgejo-child/v1", Parent: f.identity(1), Child: f.RepositoryKey() + "#02"},
	} {
		wire, err := json.Marshal(edge)
		if err != nil {
			t.Fatal(err)
		}
		server.comments[1] = []forgejoTestComment{{Body: forgejoChildMarker + string(wire) + " -->"}}
		if _, err := f.SubIssues(1); err == nil {
			t.Fatalf("invalid edge accepted: %+v", edge)
		}
	}
	server.blockers[1] = []forgejoIssue{{Number: 2, Repository: &struct {
		FullName string `json:"full_name"`
	}{FullName: "other/repo"}}}
	if _, err := f.BlockedBy(1); err == nil {
		t.Fatal("cross-repository numeric blocker accepted")
	}
}

func TestForgejoCapabilitiesFailClosed(t *testing.T) {
	for _, tc := range []struct {
		name string
		key  string
		run  func(*Forgejo) error
	}{
		{"labels", "GET /labels", func(f *Forgejo) error { return f.EnsureLabel("name", "123456", "") }},
		{"create", "POST /issues", func(f *Forgejo) error { _, err := f.CreateIssue("title", "body", nil, ""); return err }},
		{"list", "GET /issues", func(f *Forgejo) error { _, err := f.OpenIssues(); return err }},
		{"issue", "GET /issues/1", func(f *Forgejo) error { _, err := f.Issue(context.Background(), 1); return err }},
		{"children", "GET /issues/1/comments", func(f *Forgejo) error { _, err := f.SubIssues(1); return err }},
		{"dependency", "GET /issues/1/dependencies", func(f *Forgejo) error { _, err := f.BlockedBy(1); return err }},
		{"add dependency", "POST /issues/1/dependencies", func(f *Forgejo) error { return f.AddBlockedBy(1, 2) }},
		{"comment", "POST /issues/1/comments", func(f *Forgejo) error { return f.Comment(1, "body") }},
		{"close", "PATCH /issues/1", func(f *Forgejo) error { return f.CloseContext(context.Background(), 1) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, server := newForgejoTest(t)
			server.issues[1] = forgejoIssue{Number: 1, State: "open"}
			server.issues[2] = forgejoIssue{Number: 2, State: "open"}
			server.failures[tc.key] = 404
			if err := tc.run(f); err == nil {
				t.Fatal("missing capability became success")
			}
		})
	}
	f, _ := newForgejoTest(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := f.Issues(ctx, "open", ""); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled context = %v", err)
	}
}

func TestForgejoAdapterRunsSharedCapsuleConformance(t *testing.T) {
	conformancetest.Run(t, "forgejo", func(t *testing.T, body string) string {
		f, _ := newForgejoTest(t)
		if _, err := f.CreateIssue("[T1] capsule conformance", body, nil, ""); err != nil {
			t.Fatal(err)
		}
		issues, err := f.OpenIssues()
		if err != nil || len(issues) != 1 {
			t.Fatalf("round trip = %+v, %v", issues, err)
		}
		return issues[0].Body
	})
}

func TestForgejoRejectsInvalidLabelsAndObjectIdentities(t *testing.T) {
	f, server := newForgejoTest(t)
	if _, err := f.CreateIssue("title", "body", []string{"missing"}, ""); err == nil {
		t.Fatal("issue created without its required routing label")
	}
	server.labels = []forgejoLabel{{ID: 1, Name: "route"}, {ID: 2, Name: "ROUTE"}}
	if err := f.AddLabels(1, []string{"route"}); err == nil {
		t.Fatal("ambiguous label name accepted")
	}
	if err := f.AddLabels(1, nil); err != nil {
		t.Fatalf("empty additive label set: %v", err)
	}
	if _, err := f.CreateIssue("title", forgejoTypeMarker+"{} -->", nil, ""); err == nil {
		t.Fatal("caller supplied reserved issue metadata")
	}
	server.bodies["POST /issues"] = `{"number":0}`
	if _, err := f.CreateIssue("title", "body", nil, ""); err == nil {
		t.Fatal("creation without native identity accepted")
	}
	server.bodies["GET /issues/1"] = `{"number":2}`
	if _, err := f.Issue(context.Background(), 1); err == nil {
		t.Fatal("exact issue read returned another number")
	}
	if _, err := (forgejoIssue{}).issue(); err == nil {
		t.Fatal("missing native issue identity accepted")
	}
	for _, pair := range [][2]int{{0, 1}, {1, 0}, {1, 1}} {
		if err := f.LinkSubIssue(pair[0], pair[1]); err == nil {
			t.Fatalf("invalid hierarchy pair accepted: %v", pair)
		}
		if err := f.AddBlockedBy(pair[0], pair[1]); err == nil {
			t.Fatalf("invalid dependency pair accepted: %v", pair)
		}
	}
	if len(server.posts) != 1 {
		t.Fatalf("invalid inputs performed unexpected writes: %v", server.posts)
	}
}

func TestForgejoConcurrentChildAdditionsPreserveBothEdges(t *testing.T) {
	f, server := newForgejoTest(t)
	for number := 1; number <= 3; number++ {
		server.issues[number] = forgejoIssue{Number: number, State: "open"}
	}
	var group sync.WaitGroup
	errs := make(chan error, 2)
	for _, child := range []int{2, 3} {
		group.Go(func() { errs <- f.LinkSubIssue(1, child) })
	}
	group.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	children, err := f.SubIssues(1)
	if err != nil || len(children) != 2 || children[0].Number != 2 || children[1].Number != 3 {
		t.Fatalf("concurrent child addition lost an edge: %+v %v", children, err)
	}
}

func TestForgejoRewritesOnlyTheNamedSpecComment(t *testing.T) {
	f, s := newForgejoTest(t)
	s.issues[1] = forgejoIssue{Number: 1, Title: "task", Body: "old body", State: "open"}
	s.issues[2] = forgejoIssue{Number: 2, Title: "task", Body: "old body", State: "open"}
	s.comments[1] = []forgejoTestComment{{ID: 1, Body: "note"}, {ID: 2, Body: "old spec"}}
	s.comments[2] = []forgejoTestComment{{ID: 3, Body: "old spec"}, {ID: 4, Body: "old spec"}}
	s.bodies["PATCH /issues/comments/2"] = "{}"
	s.bodies["PATCH /issues/1"] = "{}"
	if err := f.ReplaceComment(1, "old spec", "new spec"); err != nil {
		t.Fatal(err)
	}
	if err := f.EditIssue(1, "[T1] new title", "new body"); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(s.posts, []string{"PATCH /issues/comments/2", "PATCH /issues/1"}) {
		t.Fatalf("writes = %v", s.posts)
	}
	if err := f.ReplaceComment(2, "old spec", "new spec"); err == nil || !strings.Contains(err.Error(), "2 comments") {
		t.Fatalf("ambiguous spec comments accepted: %v", err)
	}
}

func TestForgejoWithdrawsALabelAndAnEdge(t *testing.T) {
	f, s := newForgejoTest(t)
	for number := 1; number <= 3; number++ {
		s.issues[number] = forgejoIssue{Number: number, State: "open"}
	}
	if err := f.EnsureLabel("local-agent", "aabbcc", "routing"); err != nil {
		t.Fatal(err)
	}
	if err := f.AddLabels(1, []string{"local-agent"}); err != nil {
		t.Fatal(err)
	}
	if err := f.AddBlockedBy(1, 2); err != nil {
		t.Fatal(err)
	}
	if err := f.RemoveLabel(1, "local-agent"); err != nil {
		t.Fatal(err)
	}
	if err := f.RemoveBlockedBy(1, 2); err != nil {
		t.Fatal(err)
	}
	if len(s.issues[1].Labels) != 0 || len(s.blockers[1]) != 0 {
		t.Fatalf("labels=%v blockers=%v", s.issues[1].Labels, s.blockers[1])
	}
	if err := f.RemoveLabel(1, "missing"); err == nil {
		t.Fatal("an unknown label was withdrawn")
	}
	if err := f.AddBlockedBy(3, 2); err != nil {
		t.Fatal(err)
	}
	s.bodies["DELETE /issues/3/dependencies"] = "{}"
	if err := f.RemoveBlockedBy(3, 2); err == nil || !strings.Contains(err.Error(), "still visible") {
		t.Fatalf("an ignored removal passed: %v", err)
	}
	delete(s.bodies, "DELETE /issues/3/dependencies")
	s.failures["DELETE /issues/3/dependencies"] = 500
	if err := f.RemoveBlockedBy(3, 2); err == nil || !strings.Contains(err.Error(), "remove Forgejo blocker") {
		t.Fatalf("a failed removal passed: %v", err)
	}
	delete(s.failures, "DELETE /issues/3/dependencies")
	s.failures["GET /issues/3/dependencies"] = 500
	if err := f.RemoveBlockedBy(3, 2); err == nil {
		t.Fatal("an unconfirmed removal passed")
	}
	if err := NewForgejo(f.client, "invalid").RemoveBlockedBy(3, 2); err == nil {
		t.Fatal("an invalid repository coordinate was accepted")
	}
}

func TestForgejoEditKeepsTheIssueTypeOutOfTheProjectedBody(t *testing.T) {
	f, s := newForgejoTest(t)
	n, err := f.CreateIssue("[T1] task", "charter", nil, "Task")
	if err != nil {
		t.Fatal(err)
	}
	read, err := f.Issue(t.Context(), n)
	if err != nil || read.Body != "charter" || read.Type != "Task" {
		t.Fatalf("read = %+v, %v", read, err)
	}
	if err := f.EditIssue(n, "[T1] renamed", "new charter"); err != nil {
		t.Fatal(err)
	}
	if raw := s.issues[n].Body; !strings.HasPrefix(raw, "new charter") || !strings.Contains(raw, forgejoTypeMarker) {
		t.Fatalf("edit dropped the type record: %q", raw)
	}
	read, err = f.Issue(t.Context(), n)
	if err != nil || read.Body != "new charter" || read.Type != "Task" || read.Title != "[T1] renamed" {
		t.Fatalf("read after edit = %+v, %v", read, err)
	}
	issue := s.issues[n]
	issue.Body = strings.TrimRight(issue.Body, "\n") // a host that trims the final newline
	s.issues[n] = issue
	if read, err = f.Issue(t.Context(), n); err != nil || read.Body != "new charter" {
		t.Fatalf("trimmed read = %+v, %v", read, err)
	}
	if err := f.EditIssue(n, "[T1] renamed", "spoof "+forgejoTypeMarker); err == nil {
		t.Fatal("a body carrying the reserved record was written")
	}
	if err := f.EditIssue(99, "[T9] missing", "body"); err == nil {
		t.Fatal("a missing issue was edited")
	}
}
