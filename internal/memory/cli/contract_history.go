package cli

import (
	"context"
	"fmt"

	"github.com/agenticlab-ai/pyxis/internal/memory/reconcile"
)

// Only the pinned predecessor can establish that a design or native Decision
// was already present. A new record cannot gain historical status by choosing
// an old timestamp or by declaring an old source commit in its own payload.
func contractHistorySnapshot(ctx context.Context, m *memory, snapshot reconcile.Snapshot) (reconcile.Snapshot, error) {
	if !reconcile.NeedsContractHistory(snapshot) {
		return snapshot, nil
	}
	ref := m.reconciliationBase
	if ref == "" {
		ref = m.repo.DefaultBase(ctx, m.remote, m.branch)
	}
	if ref == "" {
		return snapshot, nil
	} // absence of proof leaves integrity findings blocking
	commit, err := m.repo.ResolveCommit(ctx, ref)
	if err != nil {
		return snapshot, fmt.Errorf("resolve contract history: %w", err)
	}
	commit, err = m.repo.MergeBase(ctx, commit, snapshot.Commit)
	if err != nil {
		return snapshot, fmt.Errorf("pin retained contract history: %w", err)
	}
	opts, set, err := reconciliationStateAt(ctx, m, commit)
	if err != nil {
		return snapshot, err
	}
	before, err := m.snapshotAt(ctx, commit, opts)
	if err != nil {
		return snapshot, err
	}
	snapshot.ContractHistory = &reconcile.ContractHistory{Snapshot: before, Set: set, Projection: m.projection(set)}
	return snapshot, nil
}
