package port

import (
	"context"

	"github.com/agenticlab-ai/pyxis/internal/entity"
)

// InvocationObserver receives host-observed argv capture and terminal usage.
// It receives no prompt, path, raw output, credential, or runtime capability.
type InvocationObserver interface {
	Launched(entity.Harness, LaunchPurpose, LaunchCapture) func(RunResult, error)
}

type invocationObserverKey struct{}

func WithInvocationObserver(ctx context.Context, observer InvocationObserver) context.Context {
	return context.WithValue(ctx, invocationObserverKey{}, observer)
}

func ObserveInvocation(ctx context.Context, harness entity.Harness, purpose LaunchPurpose, capture LaunchCapture) func(RunResult, error) {
	if observer, ok := ctx.Value(invocationObserverKey{}).(InvocationObserver); ok {
		return observer.Launched(harness, purpose, capture)
	}
	return nil
}
