package ghcodehost

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

type staticGraphQL struct{ body string }

func (s staticGraphQL) Query(context.Context, string, map[string]any) ([]byte, error) {
	return []byte(s.body), nil
}

// The reader carries the review state of review-summary comments and the
// proposal's aggregate review decision — the signals a human-feedback round
// reads.
func TestStateReader_CarriesReviewStateAndDecision(t *testing.T) {
	body := `{"data":{"repository":{"pullRequests":{"nodes":[{"number":7,"state":"OPEN","reviewDecision":"CHANGES_REQUESTED","baseRefName":"main",
	  "commits":{"nodes":[{"commit":{"checkSuites":{"nodes":[]},"statusCheckRollup":null}}]},
	  "comments":{"nodes":[]},"reviewThreads":{"nodes":[]},
	  "reviews":{"nodes":[{"author":{"login":"mdarabi"},"submittedAt":"2026-09-01T10:00:00Z","body":"needs work","state":"CHANGES_REQUESTED"}]}}]}}}}`
	r := &StateReader{client: staticGraphQL{body: body}, repo: "o/r"}
	states, err := r.States(context.Background(), []port.ProposalRef{{Repo: "o/r", Number: 7}}, port.Cursor{})
	if err != nil {
		t.Fatal(err)
	}
	if len(states) != 1 || states[0].Unknown != nil {
		t.Fatalf("states = %+v", states)
	}
	st := states[0]
	if st.ReviewDecision != "CHANGES_REQUESTED" {
		t.Fatalf("review decision = %q", st.ReviewDecision)
	}
	if len(st.Comments) != 1 || st.Comments[0].Surface != port.SurfaceReviewSummary || st.Comments[0].ReviewState != "CHANGES_REQUESTED" || st.Comments[0].Author != "mdarabi" {
		t.Fatalf("comments = %+v", st.Comments)
	}
}

// The query document matches GitHub's CURRENT schema (the dogfood found the
// old requiredStatusChecks{contexts} selection rejected live), and the current
// wire shape for required contexts decodes.
func TestStateReader_QueryMatchesCurrentSchemaShape(t *testing.T) {
	if strings.Contains(GitHubStateQuery, "requiredStatusChecks {") {
		t.Fatal("query selects requiredStatusChecks{...}, which GitHub's schema rejects; use requiredStatusCheckContexts")
	}
	if !strings.Contains(GitHubStateQuery, "requiredStatusCheckContexts") || !strings.Contains(GitHubStateQuery, "reviewDecision") {
		t.Fatalf("query lost a required selection:\n%s", GitHubStateQuery)
	}
	// GitHub rejects queries whose potential node count exceeds 500k; the
	// dominant walks are PRs × suites × runs and PRs × threads × comments
	// (found live as MAX_NODE_LIMIT_EXCEEDED at 100×100×100).
	firsts := map[string]int{}
	for _, m := range regexp.MustCompile(`(checkSuites|checkRuns|reviewThreads|comments|reviews|contexts)\(first:(\d+)\)`).FindAllStringSubmatch(GitHubStateQuery, -1) {
		n, _ := strconv.Atoi(m[2])
		if prev, ok := firsts[m[1]]; !ok || n > prev {
			firsts[m[1]] = n
		}
	}
	prs := githubStatePageSize
	runsWalk := prs * firsts["checkSuites"] * firsts["checkRuns"]
	threadWalk := prs * firsts["reviewThreads"] * firsts["comments"]
	if total := runsWalk + threadWalk + prs*firsts["contexts"] + prs*firsts["comments"] + prs*firsts["reviews"]; total > 450000 {
		t.Fatalf("query's worst-case node walk %d is over GitHub's budget (runs %d, thread comments %d)", total, runsWalk, threadWalk)
	}
	body := `{"data":{"repository":{"pullRequests":{"nodes":[{"number":9,"state":"OPEN","baseRefName":"main",
	  "baseRef":{"branchProtectionRule":{"requiredStatusCheckContexts":["docs-gate"]}},
	  "commits":{"nodes":[{"commit":{"checkSuites":{"nodes":[]},"statusCheckRollup":null}}]},
	  "comments":{"nodes":[]},"reviewThreads":{"nodes":[]},"reviews":{"nodes":[]}}]}}}}`
	r := &StateReader{client: staticGraphQL{body: body}, repo: "o/r"}
	states, err := r.States(context.Background(), []port.ProposalRef{{Repo: "o/r", Number: 9}}, port.Cursor{})
	if err != nil || len(states) != 1 || states[0].Unknown != nil {
		t.Fatalf("states = %+v, %v", states, err)
	}
	if len(states[0].RequiredMissing) != 1 || states[0].RequiredMissing[0] != "docs-gate" {
		t.Fatalf("required contexts not decoded from the current shape: %+v", states[0])
	}
}

// The query selects authorAssociation on every comment surface — the trust
// gate for feedback rounds reads it — and the reader carries it through.
func TestStateReader_CarriesAuthorAssociation(t *testing.T) {
	if got := strings.Count(GitHubStateQuery, "authorAssociation"); got != 3 {
		t.Fatalf("authorAssociation selected %d times in the query, want 3 (discussion, inline, review)", got)
	}
	body := `{"data":{"repository":{"pullRequests":{"nodes":[{"number":7,"state":"OPEN","baseRefName":"main",
	  "commits":{"nodes":[{"commit":{"checkSuites":{"nodes":[]},"statusCheckRollup":null}}]},
	  "comments":{"nodes":[{"author":{"login":"visitor"},"authorAssociation":"NONE","createdAt":"2026-09-01T09:00:00Z","body":"hi"}]},
	  "reviewThreads":{"nodes":[{"comments":{"nodes":[{"author":{"login":"m"},"authorAssociation":"MEMBER","createdAt":"2026-09-01T09:01:00Z","body":"inline"}]}}]},
	  "reviews":{"nodes":[{"author":{"login":"o"},"authorAssociation":"OWNER","submittedAt":"2026-09-01T09:02:00Z","body":"summary","state":"COMMENTED"}]}}]}}}}`
	r := &StateReader{client: staticGraphQL{body: body}, repo: "o/r"}
	states, err := r.States(context.Background(), []port.ProposalRef{{Repo: "o/r", Number: 7}}, port.Cursor{})
	if err != nil || len(states) != 1 || states[0].Unknown != nil {
		t.Fatalf("states = %+v, %v", states, err)
	}
	got := map[string]string{}
	for _, c := range states[0].Comments {
		got[c.Author] = c.AuthorAssociation
	}
	want := map[string]string{"visitor": "NONE", "m": "MEMBER", "o": "OWNER"}
	for author, assoc := range want {
		if got[author] != assoc {
			t.Fatalf("associations = %v, want %v", got, want)
		}
	}
}

func TestStateReaderAuthenticatedBotAliasesAcrossSurfaces(t *testing.T) {
	if strings.Count(GitHubStateQuery, "author { __typename login }") != 3 {
		t.Fatal("the live query must select actor type on all three comment surfaces")
	}
	for _, tc := range []struct {
		name, author, alias string
	}{
		{name: "GraphQL bot", author: `{"__typename":"Bot","login":"review-app"}`, alias: "review-app[bot]"},
		{name: "REST bot spelling", author: `{"__typename":"Bot","login":"review-app[bot]"}`, alias: "review-app"},
		{name: "human", author: `{"__typename":"User","login":"review-app"}`},
		{name: "unreported type", author: `{"login":"review-app"}`},
		{name: "other actor type", author: `{"__typename":"Organization","login":"review-app"}`},
		{name: "unreported login", author: `{"__typename":"Bot"}`},
		{name: "empty bot name", author: `{"__typename":"Bot","login":"[bot]"}`},
		{name: "deleted author", author: `null`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := fmt.Sprintf(`{"data":{"repository":{"pullRequests":{"nodes":[{"number":7,"state":"OPEN",
			  "comments":{"nodes":[{"author":%s,"authorAssociation":"NONE","body":"discussion"}]},
			  "reviewThreads":{"nodes":[{"comments":{"nodes":[{"author":%s,"authorAssociation":"NONE","body":"inline"}]}}]},
			  "reviews":{"nodes":[{"author":%s,"authorAssociation":"NONE","body":"summary","state":"COMMENTED"}]}
			}]}}}}`, tc.author, tc.author, tc.author)
			states, err := newStateReader(staticGraphQL{body: body}, "o/r").States(t.Context(), []port.ProposalRef{{Repo: "o/r", Number: 7}}, port.Cursor{})
			if err != nil || states[0].Unknown != nil || len(states[0].Comments) != 3 {
				t.Fatalf("comment surfaces=%+v err=%v", states, err)
			}
			for _, c := range states[0].Comments {
				if c.AuthorAlias != tc.alias {
					t.Errorf("%s alias=%q want=%q", c.Surface, c.AuthorAlias, tc.alias)
				}
			}
		})
	}
}
