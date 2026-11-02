package forgejotracker

import (
	"strings"
	"testing"
)

func TestExactIssueReadsNativeBlockerWithoutFrontier(t *testing.T) {
	tr, r := newTestTracker(t, "https://forgejo.test", "irrelevant", "")
	r.responses["GET /api/v1/repos/o/r/issues/17"] = `{"number":17,"state":"open","title":"ordinary"}`
	r.responses["GET /api/v1/repos/o/r/issues/17/comments"] = `[{"id":3,"body":"criterion"}]`
	r.responses["GET /api/v1/repos/o/r/issues/17/dependencies"] = `[{"number":9}]`
	r.responses["GET /api/v1/repos/o/r/issues/9"] = `{"number":9,"state":"open"}`
	s, err := tr.ReadIssue(t.Context(), 17)
	if err != nil || !s.Executable || len(s.Comments) != 1 || len(s.Dependencies) != 1 || s.Dependencies[0].Closed {
		t.Fatalf("snapshot=%+v %v", s, err)
	}
	for _, call := range r.calls {
		if strings.HasSuffix(call, "/issues") {
			t.Fatal("ordinary frontier read")
		}
	}
	r.failures["GET /api/v1/repos/o/r/issues/9"] = 404
	if _, err := tr.ReadIssue(t.Context(), 17); err == nil {
		t.Fatal("missing blocker accepted")
	}
	delete(r.failures, "GET /api/v1/repos/o/r/issues/9")
	r.responses["GET /api/v1/repos/o/r/issues/17/dependencies"] = `[{"number":9,"repository":{"full_name":"other/repo"}}]`
	if _, err := tr.ReadIssue(t.Context(), 17); err == nil {
		t.Fatal("foreign blocker accepted")
	}
	r.responses["GET /api/v1/repos/o/r/issues/17"] = `{"number":17,"state":"open","pull_request":{}}`
	if _, err := tr.ReadIssue(t.Context(), 17); err == nil {
		t.Fatal("PR accepted as issue")
	}
}

func TestExactIssueRefusesUnreadableCommentDependencyAndStateFacts(t *testing.T) {
	for _, tc := range []struct {
		name, path, body string
		status           int
	}{
		{"comments denied", "issues/17/comments", "[]", 403},
		{"unidentified comment", "issues/17/comments", "[{}]", 0},
		{"null dependencies", "issues/17/dependencies", "null", 0},
		{"unknown source state", "issues/17", "{\"number\":17,\"state\":\"unknown\"}", 0},
		{"unknown blocker state", "issues/9", "{\"number\":9,\"state\":\"unknown\"}", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tr, r := newTestTracker(t, "https://forgejo.test", "", "")
			r.responses["GET /api/v1/repos/o/r/issues/17"] = "{\"number\":17,\"state\":\"open\"}"
			r.responses["GET /api/v1/repos/o/r/issues/17/comments"] = "[]"
			r.responses["GET /api/v1/repos/o/r/issues/17/dependencies"] = "[{\"number\":9}]"
			r.responses["GET /api/v1/repos/o/r/issues/9"] = "{\"number\":9,\"state\":\"closed\"}"
			key := "GET /api/v1/repos/o/r/" + tc.path
			r.responses[key], r.failures[key] = tc.body, tc.status
			if _, err := tr.ReadIssue(t.Context(), 17); err == nil {
				t.Fatal("unverified source/dependency admitted")
			}
		})
	}
	tr, r := newTestTracker(t, "https://forgejo.test", "", "")
	if _, err := tr.ReadIssue(t.Context(), 0); err == nil {
		t.Fatal("nonpositive issue read")
	}
	r.responses["GET /api/v1/repos/o/r/issues/17"] = "{\"number\":17,\"state\":\"closed\"}"
	if s, err := tr.ReadIssue(t.Context(), 17); err != nil || s.Open {
		t.Fatalf("closed snapshot %+v %v", s, err)
	}
}
