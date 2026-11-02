package reviewinvoke

import (
	"context"
	"errors"
	"time"
)

const (
	settlementPhaseBudget = 30 * time.Second
	// Six authenticated phases may each use their own 30-second IO budget.
	// The parent deadline also bounds work between phases.
	settlementBudget = 6 * settlementPhaseBudget
)

// settleReviewPhase gives one operation a fresh deadline without extending the
// overall settlement bound. Even an adapter returning success after its deadline
// cannot authorize the next phase.
func settleReviewPhase[T any](ctx context.Context, operation func(context.Context) (T, error)) (T, error) {
	phaseCtx, cancel := context.WithTimeout(ctx, settlementPhaseBudget)
	defer cancel()
	if err := phaseCtx.Err(); err != nil {
		var zero T
		return zero, err
	}
	value, err := operation(phaseCtx)
	return value, errors.Join(err, phaseCtx.Err())
}

func settleReviewAction(ctx context.Context, operation func(context.Context) error) error {
	_, err := settleReviewPhase(ctx, func(phaseCtx context.Context) (struct{}, error) {
		return struct{}{}, operation(phaseCtx)
	})
	return err
}
