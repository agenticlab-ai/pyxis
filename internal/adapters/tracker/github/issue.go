package ghtracker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"io"
	"strconv"
)

type exactIssueClient interface {
	snapshot(context.Context, int) (port.IssueSnapshot, error)
}

func (t *Tracker) ReadIssue(ctx context.Context, number int) (port.IssueSnapshot, error) {
	c, ok := t.gh.(exactIssueClient)
	if !ok {
		return port.IssueSnapshot{}, fmt.Errorf("ghtracker: exact issue reads unavailable")
	}
	return c.snapshot(ctx, number)
}

func (c *ghClient) issue(ctx context.Context, number int) (ghIssue, error) {
	var item ghIssue
	if number <= 0 {
		return item, fmt.Errorf("issue number must be positive")
	}
	out, err := c.runRead(ctx, "api", "--method", "GET", "-H", noCacheHeader, fmt.Sprintf("repos/%s/issues/%d", c.repo, number))
	if err != nil {
		return item, fmt.Errorf("read issue #%d: %w", number, err)
	}
	if err := json.Unmarshal(out, &item); err != nil {
		return item, fmt.Errorf("decode issue #%d: %w", number, err)
	}
	if item.Number != number || (item.State != "open" && item.State != "closed") {
		return item, fmt.Errorf("issue #%d returned unknown identity/state", number)
	}
	return item, nil
}
func (c *ghClient) snapshot(ctx context.Context, number int) (port.IssueSnapshot, error) {
	item, err := c.issue(ctx, number)
	if err != nil {
		return port.IssueSnapshot{}, err
	}
	out := port.IssueSnapshot{Number: number, Title: item.Title, Body: item.Body, Open: item.State == "open", Executable: item.PullRequest == nil}
	if !out.Open || !out.Executable {
		return out, nil
	}
	raw, err := c.runRead(ctx, "api", "--method", "GET", "--paginate", "-H", noCacheHeader, fmt.Sprintf("repos/%s/issues/%d/comments?per_page=100", c.repo, number))
	if err != nil {
		return out, fmt.Errorf("read source comments: %w", err)
	}
	comments, err := issuePages[struct {
		ID   int64  `json:"id"`
		Body string `json:"body"`
	}](raw)
	if err != nil {
		return out, fmt.Errorf("decode source comments: %w", err)
	}
	for _, comment := range comments {
		if comment.ID <= 0 {
			return out, fmt.Errorf("source comment has no native identity")
		}
		out.Comments = append(out.Comments, port.IssueComment{ID: strconv.FormatInt(comment.ID, 10), Body: comment.Body})
	}
	// Unlike the legacy frontier, an unavailable dependency API is never "none".
	raw, err = c.runRead(ctx, "api", "--method", "GET", "--paginate", "-H", "Accept: application/vnd.github+json", "-H", noCacheHeader, fmt.Sprintf("repos/%s/issues/%d/dependencies/blocked_by?per_page=100", c.repo, number))
	if err != nil {
		return out, fmt.Errorf("read native dependencies: %w", err)
	}
	seen := make(map[int]bool)
	dependencies, err := issuePages[struct {
		Number        int    `json:"number"`
		RepositoryURL string `json:"repository_url"`
	}](raw)
	if err != nil {
		return out, fmt.Errorf("decode dependencies: %w", err)
	}
	for _, dep := range dependencies {
		if dep.Number <= 0 || dep.RepositoryURL != "https://api.github.com/repos/"+c.repo {
			return out, fmt.Errorf("unsupported dependency identity")
		}
		if seen[dep.Number] {
			continue
		}
		seen[dep.Number] = true
		blocker, err := c.issue(ctx, dep.Number)
		if err != nil {
			return out, fmt.Errorf("read blocker #%d: %w", dep.Number, err)
		}
		out.Dependencies = append(out.Dependencies, port.IssueDependency{Number: dep.Number, Closed: blocker.State == "closed"})
	}
	return out, nil
}

func issuePages[T any](raw []byte) ([]T, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	out := []T{}
	pages := 0
	for {
		var wire json.RawMessage
		if err := dec.Decode(&wire); err != nil {
			if err == io.EOF && pages > 0 {
				return out, nil
			}
			return nil, fmt.Errorf("unreadable exact page: %w", err)
		}
		if len(wire) == 0 || wire[0] != '[' {
			return nil, fmt.Errorf("exact page is not an array")
		}
		var page []T
		if err := json.Unmarshal(wire, &page); err != nil {
			return nil, err
		}
		out = append(out, page...)
		pages++
	}
}
