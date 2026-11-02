package admission

import (
	"context"
	"errors"

	"github.com/agenticlab-ai/pyxis/internal/maintenance/installation"
)

type LockedAuthority struct{ Store *installation.Store }

func (a LockedAuthority) Ready(ctx context.Context) (ready installation.ReadyBinding, err error) {
	if a.Store == nil {
		return ready, errors.New("maintenance installation store is required")
	}
	err = a.Store.WithTransaction(ctx, func(transaction *installation.Transaction) error {
		var readErr error
		ready, readErr = transaction.Ready()
		return readErr
	})
	return ready, err
}

func (a LockedAuthority) ValidateReady(ctx context.Context, ready installation.ReadyBinding) error {
	if a.Store == nil {
		return errors.New("maintenance installation store is required")
	}
	return a.Store.WithTransaction(ctx, func(transaction *installation.Transaction) error { return transaction.ValidateReady(ready) })
}

func (a LockedAuthority) GuardReady(ctx context.Context, ready installation.ReadyBinding, run func() error) error {
	if a.Store == nil {
		return errors.New("maintenance installation store is required")
	}
	if run == nil {
		return errors.New("maintenance ready guard callback is required")
	}
	return a.Store.WithTransaction(ctx, func(transaction *installation.Transaction) error {
		if err := transaction.ValidateReady(ready); err != nil {
			return err
		}
		return run()
	})
}
