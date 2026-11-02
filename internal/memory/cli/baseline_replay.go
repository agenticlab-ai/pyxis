package cli

import (
	"context"
	"fmt"
	"sort"

	"github.com/agenticlab-ai/pyxis/internal/memory/reconcile"
	"github.com/agenticlab-ai/pyxis/internal/memory/store"
)

// replayReconciliationBaseline accepts a snapshot only when its baseline bytes
// and all four input identities equal the baseline already trusted at the base.
// The snapshot can come from a squashed branch: its content hashes establish
// identity, while the actual baseline ref still establishes authority.
func replayReconciliationBaseline(ctx context.Context, m *memory, name, ref string, prior reconcile.Baseline, digest string) (reconcile.Snapshot, *store.Set, error) {
	if ref == "" {
		return reconcile.Snapshot{}, nil, fmt.Errorf("supply --baseline-replay-ref only when the original exact snapshot is available")
	}
	commit, err := m.repo.ResolveCommit(ctx, ref)
	if err != nil {
		return reconcile.Snapshot{}, nil, fmt.Errorf("resolve baseline replay ref %q: %w", ref, err)
	}
	read, present, err := readPolicyAt(ctx, m, commit)
	if err != nil {
		return reconcile.Snapshot{}, nil, err
	}
	if !present || read.Policy.Legacy == nil || policyDigest(read.Files[name]) != digest {
		return reconcile.Snapshot{}, nil, fmt.Errorf("baseline replay %s does not contain the unchanged trusted baseline", shortCommit(commit))
	}
	opts, set, err := reconciliationStateAt(ctx, m, commit)
	if err != nil {
		return reconcile.Snapshot{}, nil, err
	}
	opts = baselineReproductionOptions(opts, prior.Generator)
	snapshot, err := m.snapshotAt(ctx, commit, opts)
	if err != nil {
		return reconcile.Snapshot{}, nil, err
	}
	if !baselineSnapshotMatches(prior, snapshot, set) {
		return reconcile.Snapshot{}, nil, fmt.Errorf("baseline replay %s does not reproduce all four trusted input identities", shortCommit(commit))
	}
	return snapshot, set, nil
}

// baselineReproductionOptions retains each known evaluator's classification.
// v1 included session logs; both v1 and v2 used the pre-zsh code inventory.
// v1/v2/v3 retain the original contract-coverage classification. Unknown and
// current generators always retain the current compiler policy.
func baselineReproductionOptions(opts reconcile.Options, generator string) reconcile.Options {
	opts.LegacySessionLogs = generator == "memory-reconcile/v1"
	opts.LegacyCodeInventory = generator == "memory-reconcile/v1" || generator == "memory-reconcile/v2"
	opts.LegacyContractCoverage = generator == "memory-reconcile/v1" || generator == "memory-reconcile/v2" || generator == "memory-reconcile/v3"
	opts.LegacySourceRetirement = opts.LegacyContractCoverage || generator == "memory-reconcile/v4"
	opts.LegacyNativeServes = generator == "memory-reconcile/v1" || generator == "memory-reconcile/v2" || generator == "memory-reconcile/v3" || generator == "memory-reconcile/v4" || generator == "memory-reconcile/v5"
	opts.LegacyNativeEvaluator = generator == "memory-reconcile/v5"
	opts.LegacyCodeCoverage = opts.LegacyNativeServes || generator == "memory-reconcile/v6"
	return opts
}

func baselineSnapshotMatches(baseline reconcile.Baseline, snapshot reconcile.Snapshot, set *store.Set) bool {
	return snapshot.SourceClosureSHA256 == baseline.SourceClosureSHA256 &&
		snapshot.CodeInventorySHA256 == baseline.CodeInventorySHA256 &&
		snapshot.DesignIndexSHA256 == baseline.DesignIndexSHA256 &&
		reconcile.MemoryClosureSHA256(set) == baseline.MemoryClosureSHA256
}

func unionBaselineSources(left, right []string) []string {
	seen := make(map[string]bool, len(left)+len(right))
	for _, source := range left {
		seen[source] = true
	}
	for _, source := range right {
		seen[source] = true
	}
	out := make([]string, 0, len(seen))
	for source := range seen {
		out = append(out, source)
	}
	sort.Strings(out)
	return out
}
