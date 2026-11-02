package forgejotracker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/adapters/common/forgejo"
	"github.com/agenticlab-ai/pyxis/internal/plangate/tracker"
)

// Forgejo implements plan projection against one explicitly bound repository.
// Hierarchy is additive metadata on the parent; dependency edges are native.
type Forgejo struct {
	client *forgejo.Client
	repo   string
}

var _ tracker.Tracker = (*Forgejo)(nil)

func NewForgejo(client *forgejo.Client, repo string) *Forgejo {
	return &Forgejo{client: client, repo: repo}
}

func (f *Forgejo) path(suffix string) string { return forgejo.RepoPath(f.repo, suffix) }

// RepositoryKey is intentionally distinct for equal coordinates on two hosts.
func (f *Forgejo) RepositoryKey() string {
	return "forgejo:" + f.client.BaseURL() + ":" + f.repo
}

// SourceRef names the actual task host even when proposals use another host.
func (f *Forgejo) SourceRef(number int) string {
	return f.client.BaseURL() + "/" + f.repo + "/issues/" + strconv.Itoa(number)
}

func (f *Forgejo) identity(number int) string {
	return f.RepositoryKey() + "#" + strconv.Itoa(number)
}

type forgejoLabel struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Color       string `json:"color,omitempty"`
	Description string `json:"description,omitempty"`
}

type forgejoIssue struct {
	Number      int            `json:"number"`
	Title       string         `json:"title"`
	Body        string         `json:"body"`
	State       string         `json:"state"`
	Labels      []forgejoLabel `json:"labels"`
	PullRequest *struct{}      `json:"pull_request"`
	Repository  *struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
}

const forgejoTypeMarker = "<!-- pyxis:forgejo-issue/v1 "
const forgejoChildMarker = "<!-- pyxis:forgejo-child/v1 "

type forgejoIssueMetadata struct {
	SchemaVersion string `json:"schema_version"`
	Type          string `json:"type"`
}

// forgejoTypeSuffix is the issue-type record CreateIssue appends to a body.
// Reads remove it again, so the body is exactly what the projection wrote.
func forgejoTypeSuffix(issueType string) string {
	wire, _ := json.Marshal(forgejoIssueMetadata{SchemaVersion: "pyxis-forgejo-issue/v1", Type: issueType}) // a struct of two strings always encodes
	return "\n\nPyxis issue type: " + issueType + "\n\n" + forgejoTypeMarker + string(wire) + " -->\n"
}

type forgejoChildEdge struct {
	SchemaVersion string `json:"schema_version"`
	Parent        string `json:"parent"`
	Child         string `json:"child"`
}

func parseForgejoMetadata[T any](text, marker string) ([]T, error) {
	var records []T
	for {
		_, rest, found := strings.Cut(text, marker)
		if !found {
			return records, nil
		}
		payload, tail, closed := strings.Cut(rest, "-->")
		if !closed {
			return nil, fmt.Errorf("unterminated Forgejo metadata %q", marker)
		}
		var record T
		dec := json.NewDecoder(strings.NewReader(payload))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&record); err != nil {
			return nil, fmt.Errorf("parse Forgejo metadata: %w", err)
		}
		var extra any
		if err := dec.Decode(&extra); err != io.EOF {
			return nil, fmt.Errorf("Forgejo metadata has trailing content")
		}
		records = append(records, record)
		text = tail
	}
}

func (wire forgejoIssue) issue() (tracker.Issue, error) {
	if wire.Number <= 0 {
		return tracker.Issue{}, fmt.Errorf("Forgejo issue has invalid number %d", wire.Number)
	}
	metadata, err := parseForgejoMetadata[forgejoIssueMetadata](wire.Body, forgejoTypeMarker)
	if err != nil {
		return tracker.Issue{}, err
	}
	typeName := ""
	for _, record := range metadata {
		if record.SchemaVersion != "pyxis-forgejo-issue/v1" || record.Type == "" || typeName != "" {
			return tracker.Issue{}, fmt.Errorf("Forgejo issue #%d has invalid or duplicate issue-type metadata", wire.Number)
		}
		typeName = record.Type
	}
	body := wire.Body
	if typeName != "" {
		suffix := forgejoTypeSuffix(typeName)
		body = strings.TrimSuffix(strings.TrimSuffix(body, suffix), strings.TrimRight(suffix, "\n"))
	}
	labels := make([]string, 0, len(wire.Labels))
	for _, label := range wire.Labels {
		labels = append(labels, label.Name)
	}
	return tracker.Issue{Number: wire.Number, Title: wire.Title, Body: body, State: strings.ToLower(wire.State), Labels: labels, Type: typeName}, nil
}

func (f *Forgejo) labels(ctx context.Context) ([]forgejoLabel, error) {
	var labels []forgejoLabel
	if err := f.client.List(ctx, f.path("/labels"), &labels); err != nil {
		return nil, fmt.Errorf("list Forgejo labels: %w", err)
	}
	return labels, nil
}

func (f *Forgejo) EnsureLabel(name, color, description string) error {
	ctx := context.Background() // projection's existing port has no context parameter
	labels, err := f.labels(ctx)
	if err != nil {
		return err
	}
	for _, label := range labels {
		if strings.EqualFold(label.Name, name) {
			return nil // never mutate an existing label's appearance
		}
	}
	return f.client.Request(ctx, http.MethodPost, f.path("/labels"), struct {
		Name        string `json:"name"`
		Color       string `json:"color"`
		Description string `json:"description"`
	}{name, strings.TrimPrefix(color, "#"), description}, nil)
}

func (f *Forgejo) labelIDs(ctx context.Context, names []string) ([]int64, error) {
	if len(names) == 0 {
		return nil, nil
	}
	labels, err := f.labels(ctx)
	if err != nil {
		return nil, err
	}
	var ids []int64
	seen := make(map[int64]bool)
	for _, name := range names {
		var id int64
		for _, label := range labels {
			if strings.EqualFold(label.Name, name) {
				if id != 0 && id != label.ID {
					return nil, fmt.Errorf("ambiguous Forgejo label %q", name)
				}
				id = label.ID
			}
		}
		if id <= 0 {
			return nil, fmt.Errorf("Forgejo label %q does not exist", name)
		}
		if !seen[id] {
			ids = append(ids, id)
			seen[id] = true
		}
	}
	return ids, nil
}

func (f *Forgejo) CreateIssue(title, body string, labels []string, issueType string) (int, error) {
	ctx := context.Background() // projection's existing port has no context parameter
	if strings.Contains(body, forgejoTypeMarker) {
		return 0, fmt.Errorf("issue body already contains reserved Forgejo issue metadata")
	}
	ids, err := f.labelIDs(ctx, labels)
	if err != nil {
		return 0, err
	}
	if issueType != "" {
		body += forgejoTypeSuffix(issueType)
	}
	var created forgejoIssue
	if err := f.client.Request(ctx, http.MethodPost, f.path("/issues"), struct {
		Title  string  `json:"title"`
		Body   string  `json:"body"`
		Labels []int64 `json:"labels,omitempty"`
	}{title, body, ids}, &created); err != nil {
		return 0, fmt.Errorf("create Forgejo issue: %w", err)
	}
	if created.Number <= 0 {
		return 0, fmt.Errorf("create Forgejo issue returned no issue number")
	}
	return created.Number, nil
}

func (f *Forgejo) OpenIssues() ([]tracker.Issue, error) {
	return f.Issues(context.Background(), "open", "")
}

// Issues makes an exact paginated enumeration and independently verifies the
// requested state/label. Forgejo ignores unknown filter labels server-side.
func (f *Forgejo) Issues(ctx context.Context, state, label string) ([]tracker.Issue, error) {
	query := url.Values{"state": {state}, "type": {"issues"}, "sort": {"oldest"}}
	if label != "" {
		query.Set("labels", label)
	}
	var wires []forgejoIssue
	if err := f.client.List(ctx, f.path("/issues")+"?"+query.Encode(), &wires); err != nil {
		return nil, fmt.Errorf("list Forgejo issues: %w", err)
	}
	var issues []tracker.Issue
	for _, wire := range wires {
		if wire.PullRequest != nil || (state != "all" && !strings.EqualFold(wire.State, state)) {
			continue
		}
		if label != "" {
			matched := false
			for _, actual := range wire.Labels {
				matched = matched || strings.EqualFold(actual.Name, label)
			}
			if !matched {
				continue
			}
		}
		issue, err := wire.issue()
		if err != nil {
			return nil, err
		}
		issues = append(issues, issue)
	}
	return issues, nil
}

func (f *Forgejo) Issue(ctx context.Context, number int) (tracker.Issue, error) {
	var wire forgejoIssue
	if err := f.client.Request(ctx, http.MethodGet, f.path(fmt.Sprintf("/issues/%d", number)), nil, &wire); err != nil {
		return tracker.Issue{}, fmt.Errorf("read Forgejo issue #%d: %w", number, err)
	}
	if wire.Number != number || wire.PullRequest != nil {
		return tracker.Issue{}, fmt.Errorf("Forgejo issue #%d returned a different object", number)
	}
	return wire.issue()
}

func (f *Forgejo) SubIssues(parent int) ([]tracker.Issue, error) {
	ctx := context.Background() // projection's existing port has no context parameter
	comments, err := f.CommentsContext(ctx, parent)
	if err != nil {
		return nil, err
	}
	seen := make(map[int]bool)
	var numbers []int
	for _, comment := range comments {
		edges, err := parseForgejoMetadata[forgejoChildEdge](comment, forgejoChildMarker)
		if err != nil {
			return nil, err
		}
		for _, edge := range edges {
			prefix := f.RepositoryKey() + "#"
			if edge.SchemaVersion != "pyxis-forgejo-child/v1" || edge.Parent != f.identity(parent) || !strings.HasPrefix(edge.Child, prefix) {
				return nil, fmt.Errorf("Forgejo child edge has an invalid schema or repository/parent binding")
			}
			number, err := strconv.Atoi(strings.TrimPrefix(edge.Child, prefix))
			if err != nil || number <= 0 || number == parent || edge.Child != f.identity(number) {
				return nil, fmt.Errorf("Forgejo child edge has an invalid child identity")
			}
			if !seen[number] {
				seen[number] = true
				numbers = append(numbers, number)
			}
		}
	}
	sort.Ints(numbers)
	var children []tracker.Issue
	for _, number := range numbers {
		child, err := f.Issue(ctx, number)
		if err != nil {
			return nil, err
		}
		children = append(children, child)
	}
	return children, nil
}

func (f *Forgejo) LinkSubIssue(parent, child int) error {
	if parent <= 0 || child <= 0 || parent == child {
		return fmt.Errorf("invalid Forgejo parent/child numbers")
	}
	children, err := f.SubIssues(parent)
	if err != nil {
		return err
	}
	for _, existing := range children {
		if existing.Number == child {
			return nil
		}
	}
	if _, err := f.Issue(context.Background(), child); err != nil {
		return err
	}
	wire, err := json.Marshal(forgejoChildEdge{SchemaVersion: "pyxis-forgejo-child/v1", Parent: f.identity(parent), Child: f.identity(child)})
	if err != nil {
		return fmt.Errorf("encode Forgejo child edge: %w", err)
	}
	body := fmt.Sprintf("Pyxis child: [#%d](%s)\n\n%s%s -->", child, f.SourceRef(child), forgejoChildMarker, wire)
	// Concurrent distinct additions append independent records. A retry can
	// observe duplicates, which SubIssues folds without losing either child.
	if err := f.Comment(parent, body); err != nil {
		return err
	}
	children, err = f.SubIssues(parent)
	if err != nil {
		return err
	}
	for _, existing := range children {
		if existing.Number == child {
			return nil
		}
	}
	return fmt.Errorf("Forgejo child link was not visible after publication")
}

func (f *Forgejo) BlockedBy(issue int) ([]int, error) {
	return f.BlockedByContext(context.Background(), issue)
}

func (f *Forgejo) BlockedByContext(ctx context.Context, issue int) ([]int, error) {
	return f.blockedByContext(ctx, issue, false)
}

func (f *Forgejo) blockedByContext(ctx context.Context, issue int, exact bool) ([]int, error) {
	var wires []forgejoIssue
	list := f.client.List
	if exact {
		list = f.client.ListExact
	}
	if err := list(ctx, f.path(fmt.Sprintf("/issues/%d/dependencies", issue)), &wires); err != nil {
		return nil, fmt.Errorf("read Forgejo blockers for #%d: %w", issue, err)
	}
	seen := make(map[int]bool)
	var numbers []int
	for _, wire := range wires {
		if wire.Number <= 0 || (wire.Repository != nil && wire.Repository.FullName != "" && wire.Repository.FullName != f.repo) {
			return nil, fmt.Errorf("Forgejo blocker has an unsupported cross-repository or invalid identity")
		}
		if !seen[wire.Number] {
			seen[wire.Number] = true
			numbers = append(numbers, wire.Number)
		}
	}
	sort.Ints(numbers)
	return numbers, nil
}

func (f *Forgejo) AddBlockedBy(issue, blocker int) error {
	if issue <= 0 || blocker <= 0 || issue == blocker {
		return fmt.Errorf("invalid Forgejo dependency numbers")
	}
	existing, err := f.BlockedBy(issue)
	if err != nil {
		return err
	}
	if slices.Contains(existing, blocker) {
		return nil
	}
	owner, name, ok := strings.Cut(f.repo, "/")
	if !ok {
		return fmt.Errorf("invalid Forgejo repository coordinate")
	}
	if err := f.client.Request(context.Background(), http.MethodPost, f.path(fmt.Sprintf("/issues/%d/dependencies", issue)), struct {
		Index int    `json:"index"`
		Owner string `json:"owner"`
		Repo  string `json:"repo"`
	}{blocker, owner, name}, nil); err != nil {
		return fmt.Errorf("add Forgejo blocker: %w", err)
	}
	existing, err = f.BlockedBy(issue)
	if err != nil {
		return err
	}
	if slices.Contains(existing, blocker) {
		return nil
	}
	return fmt.Errorf("Forgejo dependency was not visible after publication")
}

// RemoveBlockedBy drops the edge issue ← blocker and confirms it is gone.
func (f *Forgejo) RemoveBlockedBy(issue, blocker int) error {
	owner, name, ok := strings.Cut(f.repo, "/")
	if !ok {
		return fmt.Errorf("invalid Forgejo repository coordinate")
	}
	if err := f.client.Request(context.Background(), http.MethodDelete, f.path(fmt.Sprintf("/issues/%d/dependencies", issue)), struct {
		Index int    `json:"index"`
		Owner string `json:"owner"`
		Repo  string `json:"repo"`
	}{blocker, owner, name}, nil); err != nil {
		return fmt.Errorf("remove Forgejo blocker: %w", err)
	}
	existing, err := f.BlockedBy(issue)
	if err != nil {
		return err
	}
	if slices.Contains(existing, blocker) {
		return fmt.Errorf("Forgejo dependency was still visible after removal")
	}
	return nil
}

func (f *Forgejo) AddLabels(issue int, labels []string) error {
	ctx := context.Background() // projection's existing port has no context parameter
	ids, err := f.labelIDs(ctx, labels)
	if err != nil || len(ids) == 0 {
		return err
	}
	return f.client.Request(ctx, http.MethodPost, f.path(fmt.Sprintf("/issues/%d/labels", issue)), struct {
		Labels []int64 `json:"labels"`
	}{ids}, nil)
}

// RemoveLabel withdraws one label from an issue.
func (f *Forgejo) RemoveLabel(issue int, label string) error {
	ctx := context.Background() // projection's existing port has no context parameter
	ids, err := f.labelIDs(ctx, []string{label})
	if err != nil {
		return err
	}
	return f.client.Request(ctx, http.MethodDelete, f.path(fmt.Sprintf("/issues/%d/labels/%d", issue, ids[0])), nil, nil)
}

func (f *Forgejo) Comments(issue int) ([]string, error) {
	return f.CommentsContext(context.Background(), issue)
}

func (f *Forgejo) CommentsContext(ctx context.Context, issue int) ([]string, error) {
	var comments []struct {
		ID   int64  `json:"id"`
		Body string `json:"body"`
	}
	if err := f.client.List(ctx, f.path(fmt.Sprintf("/issues/%d/comments", issue)), &comments); err != nil {
		return nil, fmt.Errorf("read Forgejo comments for #%d: %w", issue, err)
	}
	sort.SliceStable(comments, func(i, j int) bool { return comments[i].ID < comments[j].ID })
	bodies := make([]string, 0, len(comments))
	for _, comment := range comments {
		bodies = append(bodies, comment.Body)
	}
	return bodies, nil
}

func (f *Forgejo) Comment(issue int, body string) error {
	return f.CommentContext(context.Background(), issue, body)
}

func (f *Forgejo) CommentContext(ctx context.Context, issue int, body string) error {
	return f.client.Request(ctx, http.MethodPost, f.path(fmt.Sprintf("/issues/%d/comments", issue)), struct {
		Body string `json:"body"`
	}{body}, nil)
}

// EditIssue rewrites the title and body of an issue the projection created,
// keeping the issue-type record CreateIssue appended.
func (f *Forgejo) EditIssue(issue int, title, body string) error {
	ctx := context.Background() // projection's existing port has no context parameter
	if strings.Contains(body, forgejoTypeMarker) {
		return fmt.Errorf("issue body already contains reserved Forgejo issue metadata")
	}
	current, err := f.Issue(ctx, issue)
	if err != nil {
		return err
	}
	if current.Type != "" {
		body += forgejoTypeSuffix(current.Type)
	}
	return f.client.Request(ctx, http.MethodPatch, f.path(fmt.Sprintf("/issues/%d", issue)), struct {
		Title string `json:"title"`
		Body  string `json:"body"`
	}{title, body}, nil)
}

// ReplaceComment rewrites the single comment whose body is old.
func (f *Forgejo) ReplaceComment(issue int, old, body string) error {
	ctx := context.Background()
	var comments []struct {
		ID   int64  `json:"id"`
		Body string `json:"body"`
	}
	if err := f.client.List(ctx, f.path(fmt.Sprintf("/issues/%d/comments", issue)), &comments); err != nil {
		return fmt.Errorf("read Forgejo comments for #%d: %w", issue, err)
	}
	var ids []int64
	for _, comment := range comments {
		if comment.Body == old {
			ids = append(ids, comment.ID)
		}
	}
	if len(ids) != 1 {
		return fmt.Errorf("Forgejo issue #%d has %d comments matching the spec to replace, want exactly 1", issue, len(ids))
	}
	return f.client.Request(ctx, http.MethodPatch, f.path(fmt.Sprintf("/issues/comments/%d", ids[0])), struct {
		Body string `json:"body"`
	}{body}, nil)
}

// CloseContext is deliberately outside the projection interface: only the
// completion owner may close a task after verifying its proposal's ancestry.
func (f *Forgejo) CloseContext(ctx context.Context, issue int) error {
	current, err := f.Issue(ctx, issue)
	if err != nil {
		return err
	}
	if current.State == "closed" {
		return nil
	}
	if err := f.client.Request(ctx, http.MethodPatch, f.path(fmt.Sprintf("/issues/%d", issue)), struct {
		State string `json:"state"`
	}{"closed"}, nil); err != nil {
		return fmt.Errorf("close Forgejo issue #%d: %w", issue, err)
	}
	current, err = f.Issue(ctx, issue)
	if err != nil {
		return err
	}
	if current.State != "closed" {
		return fmt.Errorf("Forgejo issue #%d remains open after completion", issue)
	}
	return nil
}
