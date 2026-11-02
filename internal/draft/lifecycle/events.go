package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/draft/compile"
	"github.com/agenticlab-ai/pyxis/internal/draft/events"
	"github.com/agenticlab-ai/pyxis/internal/memory/gitsrc"
)

type pendingEvents interface {
	Add(context.Context, events.Event) error
	Read(context.Context) ([]events.Event, error)
}

var newPendingEvents = func(root, remote, feature string, version int) (pendingEvents, error) {
	return events.OpenPending(root, remote, feature, version)
}

func scaffoldEvents(ctx context.Context, files []scaffoldFile, queue pendingEvents, opened events.Event) ([]scaffoldFile, error) {
	pending, err := queue.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("read pending draft events: %w", err)
	}
	for i := range files {
		if files[i].Path == "" || filepath.Base(files[i].Path) != "events.md" {
			continue
		}
		body, _, err := events.Append(files[i].Body, append(pending, opened)...)
		if err != nil {
			return nil, fmt.Errorf("append draft opening events: %w", err)
		}
		files[i].Body = body
		return files, nil
	}
	return nil, fmt.Errorf("draft scaffold has no events.md")
}

func recordStepFailure(ctx context.Context, root string, cfg config.Config, feature string, version int, step string, cause error) error {
	queue, err := newPendingEvents(root, cfg.Remote, feature, version)
	if err != nil {
		return errors.Join(cause, fmt.Errorf("queue %s failure event: %w", step, err))
	}
	event, err := events.New(events.DraftStepFailed, feature, "", time.Now())
	if err != nil {
		return errors.Join(cause, err)
	}
	event.Step, event.ErrorClass = step, "step-error"
	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancel()
	if err := queue.Add(writeCtx, event); err != nil {
		return errors.Join(cause, fmt.Errorf("queue %s failure event: %w", step, err))
	}
	return cause
}

// abandonmentEvents reads the pinned draft and the cross-clone pending ref.
// A legacy incomplete draft can still be abandoned; a complete one must pass
// the change listing so its ruled requirement edits are retained.
func abandonmentEvents(ctx context.Context, root string, cfg config.Config, feature string, version int, head string) ([]byte, error) {
	repo, err := gitsrc.Open(ctx, root)
	if err != nil {
		return nil, fmt.Errorf("open abandoned draft: %w", err)
	}
	base, err := repo.MergeBase(ctx, cfg.MainlineRef(), head)
	if err != nil {
		return nil, fmt.Errorf("find draft history base: %w", err)
	}
	prefix := filepath.ToSlash(filepath.Join(cfg.Drafts, feature)) + "/"
	parts, err := repo.Files(ctx, head, []string{prefix + "events.md", prefix + "tech-spec.md"})
	if err != nil {
		return nil, fmt.Errorf("read abandoned draft events: %w", err)
	}
	body := parts[prefix+"events.md"]
	queue, err := newPendingEvents(root, cfg.Remote, feature, version)
	if err != nil {
		return nil, fmt.Errorf("open pending abandonment events: %w", err)
	}
	pending, err := queue.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("read pending abandonment events: %w", err)
	}
	additions := append([]events.Event(nil), pending...)
	if _, complete := parts[prefix+"tech-spec.md"]; complete {
		result, err := compile.CompileBranch(ctx, repo, base, head, cfg.Drafts, feature)
		if err != nil {
			return nil, fmt.Errorf("list abandoned requirement changes: %w", err)
		}
		changes, err := compile.ChangeEvents(result, time.Now())
		if err != nil {
			return nil, fmt.Errorf("emit abandoned requirement changes: %w", err)
		}
		additions = append(additions, changes...)
	}
	body, _, err = events.Append(body, additions...)
	if err != nil {
		return nil, fmt.Errorf("retain abandoned draft events: %w", err)
	}
	parsed, err := events.Parse(body)
	if err != nil {
		return nil, err
	}
	for _, event := range parsed {
		if event.Feature != feature {
			return nil, fmt.Errorf("abandoned event %s belongs to %s, not %s", event.ID, event.Feature, feature)
		}
	}
	if len(parsed) == 0 {
		return nil, nil
	}
	return []byte(strings.TrimSpace(string(body))), nil
}
