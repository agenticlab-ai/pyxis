package ghcodehost

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

type recordedGraphQL struct {
	body  []byte
	err   error
	calls int
	query string
}

func (f *recordedGraphQL) Query(_ context.Context, query string, _ map[string]any) ([]byte, error) {
	f.calls++
	f.query = query
	return f.body, f.err
}

func TestStateReader_OneQueryAndClaimsRequiredMissing(t *testing.T) {
	f := &recordedGraphQL{body: []byte(`{"data":{"repository":{"pullRequests":{"nodes":[{"number":7,"state":"OPEN","branchProtectionRule":{"requiredStatusChecks":{"contexts":["build","deploy"]}},"commits":{"nodes":[{"commit":{"statusCheckRollup":{"contexts":{"nodes":[{"context":"build","state":"SUCCESS"}]}}}}]},"comments":{"nodes":[]},"reviewThreads":{"nodes":[]},"reviews":{"nodes":[]}}]}}}}`)}
	r := newStateReader(f, "acme/widget")
	got, err := r.States(context.Background(), []port.ProposalRef{{Repo: "acme/widget", Number: 7}}, port.Cursor{})
	if err != nil || len(got) != 1 {
		t.Fatalf("States = %+v, %v", got, err)
	}
	if f.calls != 1 || len(got[0].RequiredMissing) != 1 || got[0].RequiredMissing[0] != "deploy" {
		t.Fatalf("calls=%d state=%+v; want one query and missing deploy", f.calls, got[0])
	}
	if f.query == "" || !contains(f.query, "reviewThreads") || !contains(f.query, "statusCheckRollup") {
		t.Fatal("query does not cover all required read surfaces")
	}
}

func TestStateReader_ErrorsBeforeData(t *testing.T) {
	f := &recordedGraphQL{body: []byte(`{"errors":[{"message":"rate limit exceeded"}],"data":{"repository":{"pullRequests":{"nodes":[{"number":1}]}}}}`)}
	r := newStateReader(f, "acme/widget")
	got, err := r.States(context.Background(), []port.ProposalRef{{Repo: "acme/widget", Number: 1}}, port.Cursor{})
	if err != nil || got[0].Unknown == nil || *got[0].Unknown != port.UnknownRateLimited {
		t.Fatalf("got=%+v err=%v, want rate-limited unknown before data extraction", got, err)
	}
	if got[0].Landing != port.LandingUnknown || len(got[0].Checks) != 0 {
		t.Fatalf("errored response leaked healthy data: %+v", got[0])
	}
}

func TestStateReader_ConformanceKit50ProposalsWithinTwoRoundTrips(t *testing.T) {
	var nodes strings.Builder
	for number := 1; number <= 49; number++ {
		if number > 1 {
			nodes.WriteByte(',')
		}
		merge := "null"
		mergedAt := "null"
		if number == 49 {
			merge = `{"oid":"abc123"}`
			mergedAt = `"2026-07-30T00:00:00Z"`
		}
		fmt.Fprintf(&nodes, `{"number":%d,"state":"OPEN","mergedAt":%s,"mergeCommit":%s,"comments":{"nodes":[]},"reviewThreads":{"nodes":[]},"reviews":{"nodes":[]}}`, number, mergedAt, merge)
	}
	f := &recordedGraphQL{body: []byte(fmt.Sprintf(`{"data":{"repository":{"pullRequests":{"nodes":[%s]}}}}`, nodes.String()))}
	refs := make([]port.ProposalRef, 50)
	for i := range refs {
		refs[i] = port.ProposalRef{Repo: "acme/widget", Number: i + 1}
	}
	r := newStateReader(f, "acme/widget")
	err := port.CheckReaderConformance(context.Background(), r, port.ReaderConformanceCase{
		Refs: refs, Landed: refs[48], LandedRevision: "abc123", Unknown: refs[49], MaxRoundTrips: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if f.calls != 1 {
		t.Fatalf("calls = %d, want one GraphQL round trip", f.calls)
	}
}

func TestStateReader_ClassifiesTransportFailures(t *testing.T) {
	for name, err := range map[string]error{"offline": errors.New("connection refused"), "rate": errors.New("HTTP 429 rate limit")} {
		t.Run(name, func(t *testing.T) {
			f := &recordedGraphQL{err: err}
			got, callErr := newStateReader(f, "acme/widget").States(context.Background(), []port.ProposalRef{{Number: 1}}, port.Cursor{})
			if callErr != nil || got[0].Unknown == nil {
				t.Fatalf("got=%+v err=%v", got, callErr)
			}
			want := port.UnknownOffline
			if name == "rate" {
				want = port.UnknownRateLimited
			}
			if *got[0].Unknown != want {
				t.Fatalf("reason=%v want %v", *got[0].Unknown, want)
			}
		})
	}
}

func TestStateReader_DecodesCheckRunsAndStatusContextsOnce(t *testing.T) {
	f := &recordedGraphQL{body: []byte(`{
  "data": {"repository": {"pullRequests": {"nodes": [{
    "number": 8,
    "state": "OPEN",
    "branchProtectionRule": null,
    "commits": {"nodes": [{"commit": {
      "checkSuites": {"nodes": [{"checkRuns": {"nodes": [
        {"name": "build", "status": "COMPLETED", "conclusion": "SUCCESS"}
      ]}}]},
      "statusCheckRollup": {"contexts": {"nodes": [
        {"__typename": "StatusContext", "context": "build", "state": "SUCCESS"},
        {"__typename": "StatusContext", "context": "lint", "state": "PENDING"}
      ]}}
    }}]},
    "comments": {"nodes": []},
    "reviewThreads": {"nodes": []},
    "reviews": {"nodes": []}
  }]}}}
}`)}
	got, err := newStateReader(f, "acme/widget").States(context.Background(), []port.ProposalRef{{Number: 8}}, port.Cursor{})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("state = %+v", got[0])
	if len(got[0].Checks) != 2 {
		t.Fatalf("checks = %+v, want one entry per observed context", got[0].Checks)
	}
	if got[0].Checks[0].Name != "build" || got[0].Checks[0].State != port.CheckGreen {
		t.Fatalf("build = %+v, want green", got[0].Checks[0])
	}
	if got[0].Checks[1].Name != "lint" || got[0].Checks[1].State != port.CheckPending {
		t.Fatalf("lint = %+v, want pending", got[0].Checks[1])
	}
	if len(got[0].RequiredMissing) != 0 {
		t.Fatalf("RequiredMissing = %v, want empty without protection", got[0].RequiredMissing)
	}
}

func TestStateReaderClosedParsingAndCursorBoundaries(t *testing.T) {
	r := newStateReader(&recordedGraphQL{}, "acme/widget")
	if got, err := r.States(t.Context(), nil, port.Cursor{}); err != nil || len(got) != 0 || r.RoundTrips() != 0 {
		t.Fatalf("empty states = %+v trips=%d err=%v", got, r.RoundTrips(), err)
	}
	if _, err := newStateReader(&recordedGraphQL{}, "invalid").States(t.Context(), []port.ProposalRef{{Number: 1}}, port.Cursor{}); err == nil {
		t.Fatal("invalid repository was accepted")
	}

	for _, body := range [][]byte{
		[]byte("not json"),
		[]byte(`{"data":{"repository":{"pullRequests":{"nodes":[]}}}}`),
	} {
		got, err := newStateReader(&recordedGraphQL{body: body}, "acme/widget").States(t.Context(), []port.ProposalRef{{Number: 9}}, port.Cursor{})
		if err != nil || len(got) != 1 || got[0].Unknown == nil || *got[0].Unknown != port.UnknownParseDefect {
			t.Fatalf("parse/missing state = %+v, %v", got, err)
		}
	}

	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	proposal := githubProposal{Number: 9, State: "CLOSED"}
	proposal.BranchProtectionRule = &struct {
		RequiredStatusCheckContexts []string `json:"requiredStatusCheckContexts"`
		RequiredStatusChecks        *struct {
			Contexts []string `json:"contexts"`
		} `json:"requiredStatusChecks"`
	}{RequiredStatusChecks: &struct {
		Contexts []string `json:"contexts"`
	}{Contexts: []string{"build", "build", ""}}}
	comment := githubComment{CreatedAt: now, Body: "discussion"}
	comment.Author.Login = "alice"
	proposal.Comments.Nodes = []githubComment{comment}
	thread := struct {
		Comments githubComments `json:"comments"`
	}{Comments: githubComments{Nodes: []githubComment{comment}}}
	proposal.ReviewThreads.Nodes = append(proposal.ReviewThreads.Nodes, thread)
	review := githubReview{SubmittedAt: now, Body: "summary"}
	review.Author.Login = "bob"
	proposal.Reviews.Nodes = []githubReview{review}

	state := decodeProposal(proposal, port.Cursor{Positions: map[port.CommentSurface]time.Time{port.SurfaceDiscussion: now}})
	if state.Landing != port.LandingClosedUnlanded || len(state.RequiredMissing) != 1 || len(state.Comments) != 2 {
		t.Fatalf("decoded closed proposal = %+v", state)
	}
}

func TestStateReaderErrorClassifiersCoverTimeoutAndGraphQLFallbacks(t *testing.T) {
	ctx, cancel := context.WithDeadline(t.Context(), time.Now().Add(-time.Second))
	defer cancel()
	if got := classifyReadError(ctx, errors.New("transport")); got != port.UnknownTimeout {
		t.Fatalf("deadline classification = %v", got)
	}
	if got := classifyGraphQLErrors([]graphqlError{{Message: "upstream timeout"}}); got != port.UnknownTimeout {
		t.Fatalf("GraphQL timeout classification = %v", got)
	}
	if got := classifyGraphQLErrors([]graphqlError{{Message: "other defect"}}); got != port.UnknownParseDefect {
		t.Fatalf("GraphQL fallback classification = %v", got)
	}
	all := unknownAll([]port.ProposalRef{{Number: 1}, {Number: 2}}, port.UnknownOffline)
	if len(all) != 2 || all[1].Unknown == nil || *all[1].Unknown != port.UnknownOffline {
		t.Fatalf("unknownAll = %+v", all)
	}
}

func contains(s, want string) bool {
	for i := 0; i+len(want) <= len(s); i++ {
		if s[i:i+len(want)] == want {
			return true
		}
	}
	return false
}
