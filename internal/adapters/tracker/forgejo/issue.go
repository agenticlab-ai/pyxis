package forgejotracker

import (
	"context"
	"fmt"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"strconv"
)

func (t *Tracker) ReadIssue(ctx context.Context, number int) (port.IssueSnapshot, error) {
	if number <= 0 {
		return port.IssueSnapshot{}, fmt.Errorf("issue number must be positive")
	}
	issue, err := t.projection.Issue(ctx, number)
	if err != nil {
		return port.IssueSnapshot{}, err
	}
	out := port.IssueSnapshot{Number: number, Title: issue.Title, Body: issue.Body, Open: issue.State == "open", Executable: issue.Type == "" || issue.Type == "task"}
	if issue.State != "open" && issue.State != "closed" {
		return out, fmt.Errorf("unknown issue state")
	}
	if !out.Open || !out.Executable {
		return out, nil
	}
	var comments []struct {
		ID   int64  `json:"id"`
		Body string `json:"body"`
	}
	if err := t.projection.client.ListExact(ctx, t.projection.path(fmt.Sprintf("issues/%d/comments", number)), &comments); err != nil {
		return out, fmt.Errorf("read source comments: %w", err)
	}
	for _, comment := range comments {
		if comment.ID <= 0 {
			return out, fmt.Errorf("source comment has no native identity")
		}
		out.Comments = append(out.Comments, port.IssueComment{ID: strconv.FormatInt(comment.ID, 10), Body: comment.Body})
	}
	blockers, err := t.projection.blockedByContext(ctx, number, true)
	if err != nil {
		return out, err
	}
	for _, number := range blockers {
		blocker, err := t.projection.Issue(ctx, number)
		if err != nil {
			return out, fmt.Errorf("read blocker #%d: %w", number, err)
		}
		if blocker.State != "open" && blocker.State != "closed" {
			return out, fmt.Errorf("blocker has unknown state")
		}
		out.Dependencies = append(out.Dependencies, port.IssueDependency{Number: number, Closed: blocker.State == "closed"})
	}
	return out, nil
}
