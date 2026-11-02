package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"time"

	forgejotracker "github.com/agenticlab-ai/pyxis/internal/adapters/tracker/forgejo"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/integrations"
	"github.com/agenticlab-ai/pyxis/internal/worktracker"
	"github.com/agenticlab-ai/pyxis/internal/worktracker/localfs"
	"github.com/agenticlab-ai/pyxis/internal/worktracker/model"
)

// configuredTaskReader reads plan tasks from the configured task binding.
// The implicit GitHub default returns nil: a hosted draft reads the code
// host's issues, and a local draft cannot read task state, so its live plan
// counts as in use. An explicit binding is read from its own repository.
func configuredTaskReader(root string, cfg config.Config) (worktracker.Lister, error) {
	profile, err := integrations.Load(root)
	if err != nil {
		return nil, err
	}
	if profile.Bindings.Tasks == (integrations.Binding{}) {
		switch cfg.Knobs.Tracker {
		case "", "gh", integrations.GitHub:
			return nil, nil // the default: tasks are the code host's GitHub issues
		}
	}
	binding, err := profile.Tasks("", cfg.Knobs.Tracker, root)
	if err != nil {
		return nil, err
	}
	switch binding.Provider {
	case integrations.Carina:
		trackerConfig, defects := config.Load(binding.Root)
		if len(defects) != 0 {
			return nil, fmt.Errorf("carina operand configuration: %v", defects)
		}
		return carinaTasks{localfs.New(filepath.Join(binding.Root, trackerConfig.WorkItems), time.Now)}, nil
	case integrations.Forgejo:
		client, err := integrationForgejoClient(binding.BaseURL, binding.Token())
		if err != nil {
			return nil, err
		}
		return forgejoTasks{forgejotracker.NewForgejo(client, binding.Repository)}, nil
	default:
		return githubTasks{root: root, binding: binding}, nil
	}
}

// carinaTasks reads Carina work items. Carina's item log is append-only, so a
// plan changed in place cannot rewrite its tasks' specs there.
type carinaTasks struct{ *localfs.Store }

func (carinaTasks) AppendOnly() bool { return true }

// githubTasks lists every issue of an explicitly bound GitHub repository,
// with that binding's server and credential.
type githubTasks struct {
	root    string
	binding integrations.ResolvedRepository
}

func (g githubTasks) List(ctx context.Context, _ model.Query) ([]model.WorkItem, []schema.Defect) {
	data, err := integrationGitHub(ctx, g.root, g.binding, "--paginate", "--method", "GET", "repos/"+g.binding.Repository+"/issues?state=all&per_page=100")
	if err != nil {
		return nil, []schema.Defect{{Op: "list", Reason: err.Error()}}
	}
	var items []model.WorkItem
	dec := json.NewDecoder(bytes.NewReader(data))
	for {
		var page []struct {
			Number      int       `json:"number"`
			Title       string    `json:"title"`
			Body        string    `json:"body"`
			State       string    `json:"state"`
			PullRequest *struct{} `json:"pull_request"`
		}
		if err := dec.Decode(&page); errors.Is(err, io.EOF) {
			return items, nil
		} else if err != nil {
			return nil, []schema.Defect{{Op: "list", Reason: fmt.Sprintf("parse GitHub tasks: %v", err)}}
		}
		for _, issue := range page {
			if issue.PullRequest != nil {
				continue
			}
			state := model.StateOpen
			if issue.State == "closed" {
				state = model.StateClosed
			}
			items = append(items, model.WorkItem{ID: issue.Number, State: state, Title: issue.Title, Body: issue.Body})
		}
	}
}

// forgejoTasks lists every Forgejo issue, open or closed, as a work item, so
// a plan's task state is read from the Forgejo tracker that holds its tasks.
type forgejoTasks struct{ tracker *forgejotracker.Forgejo }

func (f forgejoTasks) List(ctx context.Context, _ model.Query) ([]model.WorkItem, []schema.Defect) {
	issues, err := f.tracker.Issues(ctx, "all", "")
	if err != nil {
		return nil, []schema.Defect{{Op: "list", Reason: err.Error()}}
	}
	items := make([]model.WorkItem, 0, len(issues))
	for _, issue := range issues {
		state := model.StateOpen
		if issue.State == "closed" {
			state = model.StateClosed
		}
		items = append(items, model.WorkItem{ID: issue.Number, State: state, Title: issue.Title, Body: issue.Body})
	}
	return items, nil
}
