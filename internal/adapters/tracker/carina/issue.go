package carinatracker

import (
	"context"
	"fmt"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/taskrouting"
	"github.com/agenticlab-ai/pyxis/internal/worktracker/model"
	"strconv"
)

func (t *Tracker) readItem(ctx context.Context, number int) (model.WorkItem, error) {
	var item model.WorkItem
	if number <= 0 {
		return item, fmt.Errorf("item number must be positive")
	}
	raw, err := t.run(ctx, "show", strconv.Itoa(number), "--root", t.root, "--json")
	if err != nil {
		return item, fmt.Errorf("read Carina item %d: %w", number, err)
	}
	var view model.ItemView
	if err := taskrouting.Decode(raw, &view); err != nil {
		return item, fmt.Errorf("decode Carina item: %w", err)
	}
	if view.Schema != model.SchemaVersion {
		return item, fmt.Errorf("unsupported Carina item schema")
	}
	item = model.WorkItem{ID: view.ID, Type: model.Type(view.Type), State: model.State(view.State), Title: view.Title, Body: view.Body, BlockedBy: view.BlockedBy, Signals: view.Signals}
	for _, entry := range view.Log {
		item.Log = append(item.Log, model.LogEntry{Kind: model.LogKind(entry.Kind), Namespace: entry.Namespace, Payload: entry.Payload})
	}
	if item.ID != number || (item.State != model.StateOpen && item.State != model.StateClosed) {
		return item, fmt.Errorf("carina item returned unknown identity/state")
	}
	return item, nil
}
func (t *Tracker) ReadIssue(ctx context.Context, number int) (port.IssueSnapshot, error) {
	item, err := t.readItem(ctx, number)
	if err != nil {
		return port.IssueSnapshot{}, err
	}
	out := port.IssueSnapshot{Number: number, Title: item.Title, Body: item.Body, Open: item.State == model.StateOpen, Executable: item.Type == model.TypeTask}
	if !out.Open || !out.Executable {
		return out, nil
	}
	for index, entry := range item.Log {
		if entry.Kind == model.LogComment {
			out.Comments = append(out.Comments, port.IssueComment{ID: strconv.Itoa(index + 1), Body: entry.Payload})
		}
	}
	for _, ref := range item.BlockedBy {
		kind, dep := model.ClassifyRef(ref)
		if kind != model.RefLocal {
			return out, fmt.Errorf("unsupported native Carina dependency %q", ref)
		}
		blocker, err := t.readItem(ctx, dep)
		if err != nil {
			return out, err
		}
		out.Dependencies = append(out.Dependencies, port.IssueDependency{Number: dep, Closed: blocker.State == model.StateClosed})
	}
	// Explicit selection supplies authorization without writing a signal. All
	// other native ready semantics still come from the shared predicate.
	selected := item
	selected.Signals = append([]string{}, item.Signals...)
	for _, signal := range model.SeedSignals() {
		if signal.Role == model.RoleAuthorization {
			selected.Signals = append(selected.Signals, signal.Name)
		}
	}
	index := map[int]model.WorkItem{}
	for _, dep := range out.Dependencies {
		state := model.StateOpen
		if dep.Closed {
			state = model.StateClosed
		}
		index[dep.Number] = model.WorkItem{ID: dep.Number, State: state}
	}
	out.Executable = out.Executable && model.IsReady(selected, index)
	return out, nil
}
