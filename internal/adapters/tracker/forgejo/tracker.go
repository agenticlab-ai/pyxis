// Package forgejotracker binds execution to Forgejo issues without a GitHub
// CLI or GitHub credentials. Projection and execution share one API mapping.
package forgejotracker

import (
	"context"
	"fmt"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/adapters/common/forgejo"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/taskspec"
	"github.com/agenticlab-ai/pyxis/internal/integrations"
	"github.com/agenticlab-ai/pyxis/internal/plangate/planref"
)

const DefaultRoutingLabel = "local-agent"

type Tracker struct {
	projection  *Forgejo
	label       string
	planFeature string
	planVersion string
}

var _ port.WorkTracker = (*Tracker)(nil)

// New restricts the frontier to planRef when supplied. An empty planRef keeps
// the existing repository-wide routing-label mode.
func New(client *forgejo.Client, repo, routingLabel, planRef string) (*Tracker, error) {
	if client == nil {
		return nil, fmt.Errorf("forgejotracker: client is required")
	}
	if err := integrations.ValidateRepository(repo); err != nil {
		return nil, fmt.Errorf("forgejotracker: %w", err)
	}
	if routingLabel == "" {
		routingLabel = DefaultRoutingLabel
	}
	t := &Tracker{projection: NewForgejo(client, repo), label: routingLabel}
	if planRef != "" {
		feature, version, err := planref.ParsePlanRef(planRef)
		if err != nil {
			return nil, fmt.Errorf("forgejotracker: %w", err)
		}
		t.planFeature, t.planVersion = feature, version
	}
	return t, nil
}

func (t *Tracker) Frontier(ctx context.Context) ([]port.Task, error) {
	issues, err := t.projection.Issues(ctx, "open", t.label)
	if err != nil {
		return nil, fmt.Errorf("forgejotracker: read frontier: %w", err)
	}
	tasks := make([]port.Task, 0, len(issues))
	for _, issue := range issues {
		if t.planFeature != "" && !planref.InPlan(issue.Body, t.planFeature, t.planVersion) {
			continue
		}
		comments, err := t.projection.CommentsContext(ctx, issue.Number)
		if err != nil {
			return nil, fmt.Errorf("forgejotracker: read task #%d capsule: %w", issue.Number, err)
		}
		spec, err := taskspec.ParseTask(issue.Body, comments)
		if err != nil {
			return nil, fmt.Errorf("forgejotracker: validate task #%d capsule: %w", issue.Number, err)
		}
		id, title := splitTaskTitle(issue.Title)
		id = spec.TaskIdentity(id)
		demandTaskID := id
		if demandTaskID == "" {
			demandTaskID = fmt.Sprintf("issue-%d", issue.Number)
		}
		routing, err := spec.RoutingData(taskspec.DemandIdentity{Repository: t.projection.RepositoryKey(), Task: demandTaskID})
		if err != nil {
			return nil, fmt.Errorf("forgejotracker: task #%d routing: %w", issue.Number, err)
		}
		blockers, err := t.projection.BlockedByContext(ctx, issue.Number)
		if err != nil {
			return nil, fmt.Errorf("forgejotracker: task #%d blockers: %w", issue.Number, err)
		}
		tasks = append(tasks, port.Task{
			ID: id, Number: issue.Number, SourceRef: t.projection.SourceRef(issue.Number), SourceKey: t.projection.RepositoryKey(),
			Title: title, Brief: issue.Body, BlockedBy: blockers,
			DemandID: routing.ID, Requirements: routing.Requirements,
		})
	}
	return tasks, nil
}

func (t *Tracker) Comment(ctx context.Context, issue int, body string) error {
	if err := t.projection.CommentContext(ctx, issue, body); err != nil {
		return fmt.Errorf("forgejotracker: comment on #%d: %w", issue, err)
	}
	return nil
}

// Close is an optional completion capability, separate from WorkTracker's
// read/progress port. Its caller owns the verified-merge/ancestry precondition.
func (t *Tracker) Close(ctx context.Context, issue int) error {
	return t.projection.CloseContext(ctx, issue)
}

func splitTaskTitle(raw string) (string, string) {
	s := strings.TrimSpace(raw)
	if strings.HasPrefix(s, "[") {
		if i := strings.Index(s, "]"); i > 1 {
			return strings.TrimSpace(s[1:i]), strings.TrimSpace(s[i+1:])
		}
	}
	return "", s
}
