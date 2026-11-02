// Package trackeridentity adds explicit host/repository scope at a tracker
// composition boundary while leaving legacy adapters and native numbers intact.
package trackeridentity

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/integrations"
)

type Tracker struct {
	inner   port.WorkTracker
	binding integrations.ResolvedRepository
	guard   func(context.Context) error
}

// WrapWithGuard refuses ambiguous active legacy work before exposing scoped
// tasks to dispatch. The guard is evaluated on every fresh frontier read.
func WrapWithGuard(inner port.WorkTracker, binding integrations.ResolvedRepository, guard func(context.Context) error) *Tracker {
	return &Tracker{inner: inner, binding: binding, guard: guard}
}

func Wrap(inner port.WorkTracker, binding integrations.ResolvedRepository) *Tracker {
	return &Tracker{inner: inner, binding: binding}
}

func SourceRef(binding integrations.ResolvedRepository, number int) string {
	if binding.Provider == integrations.Carina {
		return binding.Key() + "#" + strconv.Itoa(number)
	}
	return strings.TrimRight(binding.BaseURL, "/") + "/" + binding.Repository + "/issues/" + strconv.Itoa(number)
}

func (t *Tracker) Frontier(ctx context.Context) ([]port.Task, error) {
	if t.guard != nil {
		if err := t.guard(ctx); err != nil {
			return nil, err
		}
	}
	tasks, err := t.inner.Frontier(ctx)
	if err != nil {
		return nil, err
	}
	for i := range tasks {
		if tasks[i].SourceKey != "" && tasks[i].SourceKey != t.binding.Key() {
			return nil, fmt.Errorf("tracker task source conflicts with selected repository")
		}
		tasks[i].SourceKey = t.binding.Key()
		tasks[i].SourceRef = SourceRef(t.binding, tasks[i].Number)
	}
	return tasks, nil
}

func (t *Tracker) Comment(ctx context.Context, issue int, body string) error {
	return t.inner.Comment(ctx, issue, body)
}

func (t *Tracker) Close(ctx context.Context, issue int) error {
	closer, ok := t.inner.(interface {
		Close(context.Context, int) error
	})
	if !ok {
		return fmt.Errorf("tracker %s has no completion capability", t.binding.Provider)
	}
	return closer.Close(ctx, issue)
}
