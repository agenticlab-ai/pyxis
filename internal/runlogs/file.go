package runlogs

import (
	"context"
	"fmt"
	"os"
)

// Create opens a private per-run diagnostic file. The caller owns its lifetime.
func Create(ctx context.Context, root, category, pattern string) (*os.File, error) {
	dir, err := Resolve(ctx, root, category, "")
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create log directory: %w", err)
	}
	f, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return nil, fmt.Errorf("create execution log: %w", err)
	}
	return f, nil
}
