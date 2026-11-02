package tracker

import (
	"errors"
	"testing"
)

func TestParseGraphQLIssueList(t *testing.T) {
	// Two concatenated pages, as `gh api graphql --paginate` emits them.
	out := []byte(`{"data":{"repository":{"issues":{"pageInfo":{"hasNextPage":true,"endCursor":"c1"},"nodes":[{"number":7,"title":"seven","body":"plan-task-id: T7","state":"OPEN","labels":{"nodes":[{"name":"kind:feat"},{"name":"local-agent"}]}}]}}}}{"data":{"repository":{"issues":{"pageInfo":{"hasNextPage":false,"endCursor":null},"nodes":[{"number":9,"title":"nine","body":"","state":"OPEN","labels":{"nodes":[]}}]}}}}`)
	issues, err := parseGraphQLIssueList(out)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(issues) != 2 {
		t.Fatalf("want 2 issues across pages, got %d", len(issues))
	}
	if issues[0].Number != 7 || issues[0].Body != "plan-task-id: T7" || issues[0].State != "open" {
		t.Fatalf("issue 0 mismatch: %+v", issues[0])
	}
	if len(issues[0].Labels) != 2 || issues[0].Labels[0] != "kind:feat" {
		t.Fatalf("labels mismatch: %+v", issues[0].Labels)
	}
	if issues[1].Number != 9 || len(issues[1].Labels) != 0 {
		t.Fatalf("issue 1 mismatch: %+v", issues[1])
	}
}

func TestParseGraphQLIssueListMalformed(t *testing.T) {
	if _, err := parseGraphQLIssueList([]byte(`{"data":`)); err == nil {
		t.Fatal("want parse error on truncated payload")
	}
}

func TestIsServerErr(t *testing.T) {
	cases := []struct {
		err  error
		want bool
	}{
		{errors.New("gh api repos/o/r/issues?state=open: HTTP 503"), true},
		{errors.New("gh api …: HTTP 502 Bad Gateway"), true},
		{errors.New("gh api …: HTTP 404 Not Found"), false},
		{errors.New("gh api …: HTTP 401"), false},
		{nil, false},
	}
	for _, c := range cases {
		if got := isServerErr(c.err); got != c.want {
			t.Fatalf("isServerErr(%v) = %v, want %v", c.err, got, c.want)
		}
	}
}
