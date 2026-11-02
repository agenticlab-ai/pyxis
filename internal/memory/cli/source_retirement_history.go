package cli

import (
	"context"
	"fmt"

	"github.com/agenticlab-ai/pyxis/internal/memory/reconcile"
	"github.com/agenticlab-ai/pyxis/internal/memory/store"
)

func sourceRetirementSnapshot(ctx context.Context, m *memory, snapshot reconcile.Snapshot, set *store.Set, trustedRef string) reconcile.Snapshot {
	active, err := reconcile.ActiveSourceRetirements(set, m.projection(set))
	if err != nil || len(active) == 0 {
		return snapshot
	} // The shared analyzer reports malformed seals.
	snapshot.SourceRetirements = map[string]reconcile.SourceRetirementProof{}
	for _, a := range active {
		evidence, err := sourceRetirementEvidence(ctx, m, snapshot, a.Receipt, trustedRef)
		proof := reconcile.SourceRetirementProof{Evidence: evidence}
		if err != nil {
			proof.Error = err.Error()
		}
		snapshot.SourceRetirements[a.RecordID] = proof
	}
	return snapshot
}

func retirementStateAt(ctx context.Context, m *memory, commit string) (reconcile.Snapshot, *store.Set, error) {
	opts, set, err := reconciliationStateAt(ctx, m, commit)
	if err != nil {
		return reconcile.Snapshot{}, nil, err
	}
	snapshot, err := m.snapshotAt(ctx, commit, opts)
	return snapshot, set, err
}

func sourceRetirementEvidence(ctx context.Context, m *memory, current reconcile.Snapshot, receipt reconcile.SourceRetirementReceipt, trustedRef string) (reconcile.SourceRetirementEvidence, error) {
	var out reconcile.SourceRetirementEvidence
	if trustedRef == "" {
		trustedRef = m.repo.DefaultBase(ctx, m.remote, m.branch)
	}
	if trustedRef == "" {
		return out, fmt.Errorf("source retirement requires locally retained configured mainline")
	}
	trusted, err := m.repo.ResolveCommit(ctx, trustedRef)
	if err != nil {
		return out, err
	}
	trusted, err = m.repo.MergeBase(ctx, trusted, current.Commit)
	if err != nil {
		return out, err
	}
	// This check also rejects a receipt whose own topic-only source commit is
	// merely reachable through the candidate's side of a merge.
	if err := m.repo.FirstParentAncestor(ctx, receipt.SourceCommit, trusted); err != nil {
		return out, err
	}
	out.Issued, out.IssuedSet, err = retirementStateAt(ctx, m, receipt.SourceCommit)
	if err != nil {
		return out, err
	}
	out.Introduction, err = m.repo.SourceIntroduction(ctx, receipt.SourceCommit, receipt.Path)
	if err != nil {
		return out, err
	}
	out.CurrentIntroduction, err = m.repo.SourceIntroduction(ctx, current.Commit, receipt.Path)
	if err != nil {
		return out, err
	}
	out.Trusted, out.TrustedSet, err = retirementStateAt(ctx, m, trusted)
	if err != nil {
		return out, err
	}
	if _, present := out.Trusted.Files[receipt.Path]; !present {
		before, err := m.repo.BeforeSourceDeletion(ctx, trusted, receipt.Path)
		if err != nil {
			return out, err
		}
		out.Trusted, out.TrustedSet, err = retirementStateAt(ctx, m, before)
		if err != nil {
			return out, err
		}
	}
	if err := m.repo.FirstParentAncestor(ctx, receipt.SourceCommit, out.Trusted.Commit); err != nil {
		return out, err
	}
	out.TrustedIntroduction, err = m.repo.SourceIntroduction(ctx, out.Trusted.Commit, receipt.Path)
	if err != nil {
		return out, err
	}
	if _, present := current.Files[receipt.Path]; !present {
		before, err := m.repo.BeforeSourceDeletion(ctx, current.Commit, receipt.Path)
		if err != nil {
			return out, err
		}
		snapshot, _, err := retirementStateAt(ctx, m, before)
		if err != nil {
			return out, err
		}
		if f, ok := snapshot.Files[receipt.Path]; ok {
			out.BeforeDeletion = &f
		}
	}
	// Every seal issued at one commit shares that commit's memory tree, so its
	// projection is computed once here rather than once per seal by the verifier.
	out.IssuedProjection, out.TrustedProjection = m.projection(out.IssuedSet), m.projection(out.TrustedSet)
	return out, nil
}
