package port

import (
	"context"
	"errors"
)

// ErrClaimLost is returned by Lease.Acquire when the task is already claimed by
// another worker — a local sibling or the external fleet. It is a skip signal,
// not a failure: the losing racer no-ops rather than erroring hard (DINV-5,
// AP-22). Callers match it with errors.Is.
var ErrClaimLost = errors.New("port: task claim already held")

// Lease is the port onto the at-most-one-worker claim, keyed by (repo, task).
// Production adapters implement it as the git-ref compare-and-swap the external
// fleet shares, plus a local in-process guard so two local Workers never race a
// claim before the CAS (prd:implement-now/FR-2, NR-2). Every acquired claim is
// released on every terminal path (fire-or-release, DINV-5).
type Lease interface {
	// Acquire attempts an at-most-one claim on the task. On success it returns a
	// Claim the caller must Release. When the task is already held it returns
	// ErrClaimLost — a skip, distinct from a hard acquire error.
	Acquire(ctx context.Context, task Task) (Claim, error)
}

// Claim is a held at-most-one-worker claim on a task.
type Claim interface {
	// Release relinquishes the claim. The Worker calls it via defer on every
	// terminal path, so a claim is never leaked.
	Release(ctx context.Context) error
}
