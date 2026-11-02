package ghcodehost

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"slices"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/adapters/common/github/githubauth"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

// GitHubStateQuery is deliberately one document: pull requests, protection,
// checks, statuses, and all three comment surfaces are selected together.
const GitHubStateQuery = `query($owner:String!, $repo:String!, $first:Int!) {
  repository(owner:$owner, name:$repo) { pullRequests(first:$first, states:[OPEN,CLOSED], orderBy:{field:UPDATED_AT,direction:DESC}) {
    nodes { number state mergedAt mergeCommit { oid } baseRefName reviewDecision
      baseRef { branchProtectionRule { requiredStatusCheckContexts } }
      commits(last:1) { nodes { commit { checkSuites(first:10) { nodes { checkRuns(first:50) { nodes { name status conclusion } } } } statusCheckRollup { contexts(first:100) { nodes { __typename ... on StatusContext { context state } ... on CheckRun { name status conclusion } } } } } } }
      comments(first:100) { nodes { author { __typename login } authorAssociation createdAt body } }
      reviewThreads(first:50) { nodes { comments(first:50) { nodes { author { __typename login } authorAssociation createdAt body } } } }
      reviews(first:50) { nodes { author { __typename login } authorAssociation submittedAt body state } }
    }
  } }
}`

// githubStatePageSize is the pull-request page. Its product with the nested
// page sizes above must stay under GitHub's 500,000-potential-node GraphQL
// budget (found live: 100 PRs × 100 suites × 100 runs was rejected with
// MAX_NODE_LIMIT_EXCEEDED). At 50 PRs the worst-case walk is ~165k nodes:
// 50×10×50 check runs + 50×50×50 thread comments dominate.
const githubStatePageSize = 50

// graphqlClient is the only medium-specific seam used by the reader.
type graphqlClient interface {
	Query(context.Context, string, map[string]any) ([]byte, error)
}

type graphqlShell struct{ tokenEnv, host string }

func (s graphqlShell) Query(ctx context.Context, query string, vars map[string]any) ([]byte, error) {
	// gh api graphql is the authenticated transport already used by this
	// adapter. gh binds ONE GraphQL variable per -f/-F flag; a single
	// `-f variables=<json>` would define a variable literally named
	// "variables" and leave $owner/$repo/$first null (found live by the
	// dogfood fixture: every batched read failed as "invalid value"). -F
	// preserves integer typing for Int! variables; strings ride -f.
	args := []string{"api", "graphql", "-f", "query=" + query}
	keys := make([]string, 0, len(vars))
	for k := range vars {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		switch v := vars[k].(type) {
		case string:
			args = append(args, "-f", k+"="+v)
		default:
			args = append(args, "-F", fmt.Sprintf("%s=%v", k, v))
		}
	}
	if s.tokenEnv != "" || s.host != "" {
		return githubauth.RunForHost(ctx, s.tokenEnv, s.host, nil, args...)
	}
	return shellGH(ctx, args...)
}

// StateReader is GitHub's batched implementation of ProposalStateReader.
type StateReader struct {
	client     graphqlClient
	repo       string
	cadence    time.Duration
	roundTrips atomic.Int64
}

var _ port.ProposalStateReader = (*StateReader)(nil)

// NewStateReader binds a production reader. cadence is advisory and exposed
// for callers that schedule the next read; the reader never retries in-band.
func NewStateReader(repo string) *StateReader {
	return &StateReader{client: graphqlShell{}, repo: repo, cadence: 10 * time.Second}
}

func newStateReader(c graphqlClient, repo string) *StateReader {
	return &StateReader{client: c, repo: repo, cadence: 10 * time.Second}
}

func (r *StateReader) Cadence() time.Duration { return r.cadence }

// RoundTrips reports transport calls made by this reader. It is intentionally
// observational: scheduling and retry decisions remain outside the adapter.
func (r *StateReader) RoundTrips() int { return int(r.roundTrips.Load()) }

func (r *StateReader) States(ctx context.Context, refs []port.ProposalRef, since port.Cursor) ([]port.ProposalState, error) {
	results := make([]port.ProposalState, len(refs))
	if len(refs) == 0 {
		return results, nil
	}
	owner, repo, ok := strings.Cut(r.repo, "/")
	if !ok || owner == "" || repo == "" {
		return nil, fmt.Errorf("ghcodehost: invalid repository %q", r.repo)
	}
	vars := map[string]any{"owner": owner, "repo": repo, "first": githubStatePageSize}
	r.roundTrips.Add(1)
	body, err := r.client.Query(ctx, GitHubStateQuery, vars)
	if err != nil {
		return unknownAll(refs, classifyReadError(ctx, err)), nil
	}
	var wire githubStateResponse
	if err := json.Unmarshal(body, &wire); err != nil {
		return unknownAll(refs, port.UnknownParseDefect), nil
	}
	if len(wire.Errors) > 0 {
		return unknownAll(refs, classifyGraphQLErrors(wire.Errors)), nil
	}
	byNumber := make(map[int]githubProposal, len(wire.Data.Repository.PullRequests.Nodes))
	for _, n := range wire.Data.Repository.PullRequests.Nodes {
		byNumber[n.Number] = n
	}
	for i, ref := range refs {
		n, found := byNumber[ref.Number]
		if !found {
			results[i] = unknownState(port.UnknownParseDefect)
			continue
		}
		results[i] = decodeProposal(n, since)
	}
	return results, nil
}

func unknownState(reason port.UnknownReason) port.ProposalState {
	return port.ProposalState{Unknown: &reason}
}
func unknownAll(refs []port.ProposalRef, reason port.UnknownReason) []port.ProposalState {
	out := make([]port.ProposalState, len(refs))
	for i := range out {
		out[i] = unknownState(reason)
	}
	return out
}

func classifyReadError(ctx context.Context, err error) port.UnknownReason {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
		return port.UnknownTimeout
	}
	if strings.Contains(strings.ToLower(err.Error()), "rate") || strings.Contains(strings.ToLower(err.Error()), "429") {
		return port.UnknownRateLimited
	}
	// A schema/validation refusal is a QUERY defect, not connectivity: gh
	// surfaces GraphQL errors on stderr with these signatures. Mapping them to
	// offline (the old behavior) sent operators hunting for a network problem
	// that was a broken query the whole time.
	msg := strings.ToLower(err.Error())
	for _, sig := range []string{"doesn't exist on type", "max_node_limit", "exceeds the maximum limit", "was provided invalid value", "parse error", "undefinedfield"} {
		if strings.Contains(msg, sig) {
			return port.UnknownParseDefect
		}
	}
	var ne net.Error
	if errors.As(err, &ne) {
		return port.UnknownOffline
	}
	return port.UnknownOffline
}

type graphqlError struct {
	Message string `json:"message"`
}

func classifyGraphQLErrors(es []graphqlError) port.UnknownReason {
	for _, e := range es {
		s := strings.ToLower(e.Message)
		if strings.Contains(s, "rate") || strings.Contains(s, "limit") {
			return port.UnknownRateLimited
		}
		if strings.Contains(s, "timeout") {
			return port.UnknownTimeout
		}
	}
	return port.UnknownParseDefect
}

type githubStateResponse struct {
	Errors []graphqlError `json:"errors"`
	Data   struct {
		Repository struct {
			PullRequests struct {
				Nodes []githubProposal `json:"nodes"`
			} `json:"pullRequests"`
		} `json:"repository"`
	} `json:"data"`
}
type githubProposal struct {
	ReviewDecision string     `json:"reviewDecision"`
	Number         int        `json:"number"`
	State          string     `json:"state"`
	MergedAt       *time.Time `json:"mergedAt"`
	MergeCommit    *struct {
		OID string `json:"oid"`
	} `json:"mergeCommit"`
	BaseRef *struct {
		BranchProtectionRule *struct {
			// RequiredStatusCheckContexts is the field GitHub's current schema
			// serves; the nested requiredStatusChecks{contexts} shape below is
			// retained only to decode recorded responses.
			RequiredStatusCheckContexts []string `json:"requiredStatusCheckContexts"`
			RequiredStatusChecks        *struct {
				Contexts []string `json:"contexts"`
			} `json:"requiredStatusChecks"`
		} `json:"branchProtectionRule"`
	} `json:"baseRef"`
	// BranchProtectionRule is retained for recorded responses made by the
	// predecessor query; live GraphQL reads use BaseRef above.
	BranchProtectionRule *struct {
		RequiredStatusCheckContexts []string `json:"requiredStatusCheckContexts"`
		RequiredStatusChecks        *struct {
			Contexts []string `json:"contexts"`
		} `json:"requiredStatusChecks"`
	} `json:"branchProtectionRule"`
	Commits struct {
		Nodes []struct {
			Commit githubCommit `json:"commit"`
		} `json:"nodes"`
	} `json:"commits"`
	Comments      githubComments `json:"comments"`
	ReviewThreads struct {
		Nodes []struct {
			Comments githubComments `json:"comments"`
		} `json:"nodes"`
	} `json:"reviewThreads"`
	Reviews struct {
		Nodes []githubReview `json:"nodes"`
	} `json:"reviews"`
}
type githubCommit struct {
	CheckSuites struct {
		Nodes []struct {
			CheckRuns githubRuns `json:"checkRuns"`
		} `json:"nodes"`
	} `json:"checkSuites"`
	StatusCheckRollup struct {
		Contexts struct {
			Nodes []githubContext `json:"nodes"`
		} `json:"contexts"`
	} `json:"statusCheckRollup"`
}
type githubRuns struct {
	Nodes []struct{ Name, Status, Conclusion string } `json:"nodes"`
}
type githubContext struct {
	Type                                     string `json:"__typename"`
	Context, State, Name, Status, Conclusion string
}
type githubComments struct {
	Nodes []githubComment `json:"nodes"`
}
type githubAuthor struct {
	Type  string `json:"__typename"`
	Login string `json:"login"`
}

// GitHub's authenticated Bot actor metadata establishes the REST/GraphQL
// spelling pair. A User or an author whose type is unavailable gains no alias.
func (a githubAuthor) alias() string {
	login := strings.TrimSpace(a.Login)
	if a.Type != "Bot" || login == "" {
		return ""
	}
	if plain, ok := strings.CutSuffix(login, "[bot]"); ok {
		return plain
	}
	return login + "[bot]"
}

type githubComment struct {
	Author            githubAuthor `json:"author"`
	AuthorAssociation string       `json:"authorAssociation"`
	CreatedAt         time.Time    `json:"createdAt"`
	Body              string       `json:"body"`
}
type githubReview struct {
	State             string       `json:"state"`
	Author            githubAuthor `json:"author"`
	AuthorAssociation string       `json:"authorAssociation"`
	SubmittedAt       time.Time    `json:"submittedAt"`
	Body              string       `json:"body"`
}

func decodeProposal(n githubProposal, since port.Cursor) port.ProposalState {
	st := port.ProposalState{Landing: port.LandingOpen}
	if n.MergedAt != nil {
		st.Landing = port.LandingLanded
		if n.MergeCommit != nil {
			st.LandedRevision = n.MergeCommit.OID
		}
	} else if strings.EqualFold(n.State, "CLOSED") {
		st.Landing = port.LandingClosedUnlanded
	}
	var required []string
	switch {
	case n.BaseRef != nil && n.BaseRef.BranchProtectionRule != nil && len(n.BaseRef.BranchProtectionRule.RequiredStatusCheckContexts) > 0:
		required = n.BaseRef.BranchProtectionRule.RequiredStatusCheckContexts
	case n.BaseRef != nil && n.BaseRef.BranchProtectionRule != nil && n.BaseRef.BranchProtectionRule.RequiredStatusChecks != nil:
		required = n.BaseRef.BranchProtectionRule.RequiredStatusChecks.Contexts
	case n.BranchProtectionRule != nil && len(n.BranchProtectionRule.RequiredStatusCheckContexts) > 0:
		required = n.BranchProtectionRule.RequiredStatusCheckContexts
	case n.BranchProtectionRule != nil && n.BranchProtectionRule.RequiredStatusChecks != nil:
		required = n.BranchProtectionRule.RequiredStatusChecks.Contexts
	}
	seen := map[string]bool{}
	// GitHub exposes the same logical check through both checkRuns and the
	// status rollup. Decode both surfaces, but emit one context per name. The
	// rollup is useful for legacy StatusContext entries; checkRuns is required
	// for checks that are not represented there.
	addCheck := func(name, status, conclusion, state string) {
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		st.Checks = append(st.Checks, port.CheckContext{Name: name, State: checkState(status, conclusion, state)})
	}
	for _, c := range n.Commits.Nodes {
		for _, suite := range c.Commit.CheckSuites.Nodes {
			for _, run := range suite.CheckRuns.Nodes {
				addCheck(run.Name, run.Status, run.Conclusion, "")
			}
		}
		for _, s := range c.Commit.StatusCheckRollup.Contexts.Nodes {
			name := s.Context
			if name == "" {
				name = s.Name
			}
			addCheck(name, s.Status, s.Conclusion, s.State)
		}
	}
	// A protection rule is authoritative when present. With no declared
	// required set, RequiredMissing is empty by design: consumers must reduce
	// every observed context (the strict fallback) and may not use an aggregate
	// rollup as a substitute for those contexts.
	if len(required) > 0 {
		for _, name := range required {
			if name != "" && !seen[name] && !containsString(st.RequiredMissing, name) {
				st.RequiredMissing = append(st.RequiredMissing, name)
			}
		}
	}
	addComments := func(xs []port.Comment) {
		for _, c := range xs {
			if t, ok := since.Positions[c.Surface]; !ok || c.CreatedAt.After(t) {
				st.Comments = append(st.Comments, c)
			}
		}
	}
	for _, c := range n.Comments.Nodes {
		addComments([]port.Comment{{Author: c.Author.Login, AuthorAlias: c.Author.alias(), AuthorAssociation: c.AuthorAssociation, CreatedAt: c.CreatedAt, Surface: port.SurfaceDiscussion, Body: c.Body}})
	}
	for _, th := range n.ReviewThreads.Nodes {
		for _, c := range th.Comments.Nodes {
			addComments([]port.Comment{{Author: c.Author.Login, AuthorAlias: c.Author.alias(), AuthorAssociation: c.AuthorAssociation, CreatedAt: c.CreatedAt, Surface: port.SurfaceInlineReview, Body: c.Body}})
		}
	}
	for _, v := range n.Reviews.Nodes {
		addComments([]port.Comment{{Author: v.Author.Login, AuthorAlias: v.Author.alias(), AuthorAssociation: v.AuthorAssociation, CreatedAt: v.SubmittedAt, Surface: port.SurfaceReviewSummary, Body: v.Body, ReviewState: v.State}})
	}
	st.ReviewDecision = n.ReviewDecision
	return st
}

func containsString(xs []string, want string) bool {
	return slices.Contains(xs, want)
}
func checkState(status, conclusion, state string) port.CheckState {
	if strings.EqualFold(state, "SUCCESS") || strings.EqualFold(conclusion, "SUCCESS") || strings.EqualFold(conclusion, "NEUTRAL") || strings.EqualFold(conclusion, "SKIPPED") {
		return port.CheckGreen
	}
	if strings.EqualFold(status, "COMPLETED") || (state != "" && !strings.EqualFold(state, "PENDING")) {
		return port.CheckRed
	}
	return port.CheckPending
}
