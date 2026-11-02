package codegatemirror

import (
	"context"
	"time"
)

// Progress is an execution observation, not a reusable verification verdict.
// Observers may be called concurrently by independent check groups.
type Progress struct {
	Name     string
	Phase    int
	Group    string
	Status   string
	Duration time.Duration
	Detail   string
	LogPath  string
}

type progressKey struct{}

func WithProgressObserver(ctx context.Context, observe func(Progress)) context.Context {
	return context.WithValue(ctx, progressKey{}, observe)
}

func emitProgress(ctx context.Context, check Check, status string, out Outcome) {
	if observe, ok := ctx.Value(progressKey{}).(func(Progress)); ok && observe != nil {
		observe(Progress{Name: check.Name, Phase: check.ExecutionPhase(), Group: check.ExecutionGroup(), Status: status, Duration: out.Duration, Detail: out.Detail, LogPath: out.LogPath})
	}
}
