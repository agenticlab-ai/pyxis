// The gh-CLI GitHub adapter — the seam's sole v1 implementation. It shells
// the authenticated `gh` binary (the repo's standing operating assumption:
// hooks and CI already require it) so no token handling lives here; every
// listing is exact REST with explicit pagination, never a search index
// (DINV-4; the predecessor's I4/I12 lesson).
package tracker

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"os/exec"
	"strings"
)

// GH is the GitHub-backed Tracker for one repository.
type GH struct {
	Repo string // owner/name
	// run is the hermetic adapter-boundary seam used by conformance tests.
	// Production leaves it nil and executes the authenticated gh CLI.
	run func(args ...string) ([]byte, error)
}

// ResolveRepo reads the repository slug for a checkout root.
func ResolveRepo(root string) (string, error) {
	out, err := ghIn(root, "repo", "view", "--json", "nameWithOwner", "--jq", ".nameWithOwner")
	if err != nil {
		return "", fmt.Errorf("resolve repo slug: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func ghIn(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command("gh", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
			return nil, fmt.Errorf("gh %s: %s", strings.Join(args, " "), strings.TrimSpace(string(ee.Stderr)))
		}
		return nil, fmt.Errorf("gh %s: %w", strings.Join(args, " "), err)
	}
	return out, nil
}

func (g *GH) api(args ...string) ([]byte, error) {
	if g.run != nil {
		return g.run(args...)
	}
	return ghIn("", append([]string{"api"}, args...)...)
}

type ghIssue struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	Body   string `json:"body"`
	State  string `json:"state"`
	Labels []struct {
		Name string `json:"name"`
	} `json:"labels"`
	Type *struct {
		Name string `json:"name"`
	} `json:"type"`
	PullRequest *struct{} `json:"pull_request"`
	ID          int64     `json:"id"`
}

func (gi ghIssue) issue() Issue {
	labels := make([]string, 0, len(gi.Labels))
	for _, l := range gi.Labels {
		labels = append(labels, l.Name)
	}
	typeName := ""
	if gi.Type != nil {
		typeName = gi.Type.Name
	}
	return Issue{Number: gi.Number, Title: gi.Title, Body: gi.Body, Labels: labels, State: strings.ToLower(gi.State), Type: typeName}
}

func (g *GH) EnsureLabel(name, color, description string) error {
	if _, err := g.api(fmt.Sprintf("repos/%s/labels/%s", g.Repo, name)); err == nil {
		return nil // exists — never recolor (NR-2)
	}
	_, err := g.api("--method", "POST", fmt.Sprintf("repos/%s/labels", g.Repo),
		"-f", "name="+name, "-f", "color="+color, "-f", "description="+description)
	return err
}

func (g *GH) CreateIssue(title, body string, labels []string, issueType string) (int, error) {
	args := []string{"--method", "POST", fmt.Sprintf("repos/%s/issues", g.Repo),
		"-f", "title=" + title, "-f", "body=" + body}
	for _, l := range labels {
		args = append(args, "-f", "labels[]="+l)
	}
	if issueType != "" {
		args = append(args, "-f", "type="+issueType)
	}
	out, err := g.api(args...)
	if err != nil && issueType != "" {
		// A tracker without this type still gets the issue — the type is
		// legibility, not contract (fail open on the additive nicety).
		return g.CreateIssue(title, body, labels, "")
	}
	if err != nil {
		return 0, err
	}
	var gi ghIssue
	if err := json.Unmarshal(out, &gi); err != nil {
		return 0, fmt.Errorf("parse created issue: %w", err)
	}
	return gi.Number, nil
}

func (g *GH) OpenIssues() ([]Issue, error) {
	out, err := g.api("--paginate", fmt.Sprintf("repos/%s/issues?state=open&per_page=100", g.Repo))
	if err == nil {
		return parseIssueList(out)
	}
	if !isServerErr(err) {
		return nil, err
	}
	// REST 5xx (the 2026-07-16 route outage class): fall back to the exact
	// GraphQL repository.issues connection — the same listing, still never
	// a search index (the I4/I12 constraint holds; GraphQL connections are
	// exact reads, not index queries).
	issues, gqlErr := g.openIssuesGraphQL()
	if gqlErr != nil {
		return nil, fmt.Errorf("REST: %v; GraphQL fallback: %w", err, gqlErr)
	}
	return issues, nil
}

// isServerErr recognizes gh's 5xx surface ("gh api …: HTTP 503") — the only
// class where a transport fallback is sound; 4xx stays a real answer.
func isServerErr(err error) bool {
	return err != nil && strings.Contains(err.Error(), "HTTP 50")
}

func (g *GH) openIssuesGraphQL() ([]Issue, error) {
	owner, name, ok := strings.Cut(g.Repo, "/")
	if !ok {
		return nil, fmt.Errorf("repo slug %q: want owner/name", g.Repo)
	}
	const q = `query($owner:String!,$name:String!,$endCursor:String){repository(owner:$owner,name:$name){issues(states:OPEN,first:100,after:$endCursor){pageInfo{hasNextPage endCursor} nodes{number title body state labels(first:100){nodes{name}}}}}}`
	out, err := g.api("graphql", "--paginate", "-f", "query="+q, "-F", "owner="+owner, "-F", "name="+name)
	if err != nil {
		return nil, err
	}
	return parseGraphQLIssueList(out)
}

// parseGraphQLIssueList tolerates gh --paginate's concatenated JSON objects
// (one per page). The issues connection carries no pull requests by
// construction; the type nicety is absent in this path (Issue.Type = "").
func parseGraphQLIssueList(out []byte) ([]Issue, error) {
	var issues []Issue
	dec := json.NewDecoder(bytes.NewReader(out))
	for dec.More() {
		var page struct {
			Data struct {
				Repository struct {
					Issues struct {
						Nodes []struct {
							Number int    `json:"number"`
							Title  string `json:"title"`
							Body   string `json:"body"`
							State  string `json:"state"`
							Labels struct {
								Nodes []struct {
									Name string `json:"name"`
								} `json:"nodes"`
							} `json:"labels"`
						} `json:"nodes"`
					} `json:"issues"`
				} `json:"repository"`
			} `json:"data"`
		}
		if err := dec.Decode(&page); err != nil {
			return nil, fmt.Errorf("parse graphql issue list: %w", err)
		}
		for _, n := range page.Data.Repository.Issues.Nodes {
			labels := make([]string, 0, len(n.Labels.Nodes))
			for _, l := range n.Labels.Nodes {
				labels = append(labels, l.Name)
			}
			issues = append(issues, Issue{Number: n.Number, Title: n.Title, Body: n.Body, Labels: labels, State: strings.ToLower(n.State)})
		}
	}
	return issues, nil
}

func (g *GH) SubIssues(parent int) ([]Issue, error) {
	out, err := g.api("--paginate", "-H", "Accept: application/vnd.github+json",
		fmt.Sprintf("repos/%s/issues/%d/sub_issues?per_page=100", g.Repo, parent))
	if err != nil {
		// A tracker without the sub-issue surface reads as "no children";
		// linking will surface the real capability defect.
		if strings.Contains(err.Error(), "404") {
			return nil, nil
		}
		return nil, err
	}
	return parseIssueList(out)
}

// parseIssueList tolerates gh --paginate's concatenated JSON arrays and
// filters pull requests out of the issues listing.
func parseIssueList(out []byte) ([]Issue, error) {
	var issues []Issue
	dec := json.NewDecoder(strings.NewReader(string(out)))
	for dec.More() {
		var page []ghIssue
		if err := dec.Decode(&page); err != nil {
			return nil, fmt.Errorf("parse issue list: %w", err)
		}
		for _, gi := range page {
			if gi.PullRequest != nil {
				continue
			}
			issues = append(issues, gi.issue())
		}
	}
	return issues, nil
}

func (g *GH) internalID(issue int) (int64, error) {
	out, err := g.api(fmt.Sprintf("repos/%s/issues/%d", g.Repo, issue), "--jq", ".id")
	if err != nil {
		return 0, err
	}
	var id int64
	if _, err := fmt.Sscanf(strings.TrimSpace(string(out)), "%d", &id); err != nil {
		return 0, fmt.Errorf("parse issue id: %w", err)
	}
	return id, nil
}

func (g *GH) LinkSubIssue(parent, child int) error {
	id, err := g.internalID(child)
	if err != nil {
		return err
	}
	// -F sends the id typed (integer) — the string-encoded -f form 422s.
	_, err = g.api("--method", "POST", "-H", "Accept: application/vnd.github+json",
		fmt.Sprintf("repos/%s/issues/%d/sub_issues", g.Repo, parent),
		"-F", fmt.Sprintf("sub_issue_id=%d", id))
	return err
}

func (g *GH) BlockedBy(issue int) ([]int, error) {
	out, err := g.api("--paginate", "-H", "Accept: application/vnd.github+json",
		fmt.Sprintf("repos/%s/issues/%d/dependencies/blocked_by?per_page=100", g.Repo, issue))
	if err != nil {
		if strings.Contains(err.Error(), "404") {
			return nil, nil
		}
		return nil, err
	}
	var nums []int
	dec := json.NewDecoder(strings.NewReader(string(out)))
	for dec.More() {
		var page []struct {
			Number int `json:"number"`
		}
		if err := dec.Decode(&page); err != nil {
			return nil, fmt.Errorf("parse blocked-by list: %w", err)
		}
		for _, e := range page {
			nums = append(nums, e.Number)
		}
	}
	return nums, nil
}

func (g *GH) AddBlockedBy(issue, blocker int) error {
	id, err := g.internalID(blocker)
	if err != nil {
		return err
	}
	_, err = g.api("--method", "POST", "-H", "Accept: application/vnd.github+json",
		fmt.Sprintf("repos/%s/issues/%d/dependencies/blocked_by", g.Repo, issue),
		"-F", fmt.Sprintf("issue_id=%d", id))
	return err
}

// RemoveBlockedBy drops the edge issue ← blocker.
func (g *GH) RemoveBlockedBy(issue, blocker int) error {
	id, err := g.internalID(blocker)
	if err != nil {
		return err
	}
	_, err = g.api("--method", "DELETE", "-H", "Accept: application/vnd.github+json",
		fmt.Sprintf("repos/%s/issues/%d/dependencies/blocked_by/%d", g.Repo, issue, id))
	return err
}

func (g *GH) AddLabels(issue int, labels []string) error {
	args := []string{"--method", "POST", fmt.Sprintf("repos/%s/issues/%d/labels", g.Repo, issue)}
	for _, l := range labels {
		args = append(args, "-f", "labels[]="+l)
	}
	_, err := g.api(args...)
	return err
}

// RemoveLabel withdraws one label from an issue.
func (g *GH) RemoveLabel(issue int, label string) error {
	_, err := g.api("--method", "DELETE", fmt.Sprintf("repos/%s/issues/%d/labels/%s", g.Repo, issue, url.PathEscape(label)))
	return err
}

func (g *GH) Comments(issue int) ([]string, error) {
	out, err := g.api("--paginate", fmt.Sprintf("repos/%s/issues/%d/comments?per_page=100", g.Repo, issue))
	if err != nil {
		return nil, err
	}
	var bodies []string
	dec := json.NewDecoder(strings.NewReader(string(out)))
	for dec.More() {
		var page []struct {
			Body string `json:"body"`
		}
		if err := dec.Decode(&page); err != nil {
			return nil, fmt.Errorf("parse comments: %w", err)
		}
		for _, c := range page {
			bodies = append(bodies, c.Body)
		}
	}
	return bodies, nil
}

func (g *GH) Comment(issue int, body string) error {
	_, err := g.api("--method", "POST", fmt.Sprintf("repos/%s/issues/%d/comments", g.Repo, issue),
		"-f", "body="+body)
	return err
}

// EditIssue rewrites the title and body of an issue the projection created.
func (g *GH) EditIssue(issue int, title, body string) error {
	_, err := g.api("--method", "PATCH", fmt.Sprintf("repos/%s/issues/%d", g.Repo, issue), "-f", "title="+title, "-f", "body="+body)
	return err
}

// ReplaceComment rewrites the single comment whose body is old. None or
// several such comments is a refusal: the projection never guesses.
func (g *GH) ReplaceComment(issue int, old, body string) error {
	out, err := g.api("--paginate", fmt.Sprintf("repos/%s/issues/%d/comments?per_page=100", g.Repo, issue))
	if err != nil {
		return err
	}
	var ids []int64
	dec := json.NewDecoder(strings.NewReader(string(out)))
	for dec.More() {
		var page []struct {
			ID   int64  `json:"id"`
			Body string `json:"body"`
		}
		if err := dec.Decode(&page); err != nil {
			return fmt.Errorf("parse comments: %w", err)
		}
		for _, c := range page {
			if c.Body == old {
				ids = append(ids, c.ID)
			}
		}
	}
	if len(ids) != 1 {
		return fmt.Errorf("issue #%d has %d comments matching the spec to replace, want exactly 1", issue, len(ids))
	}
	_, err = g.api("--method", "PATCH", fmt.Sprintf("repos/%s/issues/comments/%d", g.Repo, ids[0]), "-f", "body="+body)
	return err
}
