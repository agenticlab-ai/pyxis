package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/memory/reconcile"
)

type policyFileIdentity struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type policyEvidence struct {
	Schema    string               `json:"schema_version"`
	Generator string               `json:"generator"`
	Commit    string               `json:"commit,omitempty"`
	Files     []policyFileIdentity `json:"files"`
}

type policyRead struct {
	Policy   reconcile.PolicySet
	Evidence policyEvidence
	Files    map[string][]byte
}

// readPolicyAt discovers a closed root and reads all blobs in one batch. An
// absent marker is distinguished from a malformed or mixed adopted policy.
func readPolicyAt(ctx context.Context, m *memory, commit string) (policyRead, bool, error) {
	nodes, err := m.repo.TreeNodes(ctx, commit, ".github")
	if err != nil {
		return policyRead{}, false, fmt.Errorf("read reconciliation baseline tree at %s: %w", shortCommit(commit), err)
	}
	var paths []string
	rootPresent := false
	for _, node := range nodes {
		switch {
		case node.Path == ".github" || node.Path == reconcile.PolicyDirectory:
			if node.Type != "tree" || node.Mode != "040000" {
				return policyRead{}, false, fmt.Errorf("policy root %s is not a regular directory", node.Path)
			}
			if node.Path == reconcile.PolicyDirectory {
				rootPresent = true
			}
		case node.Path == reconcile.PolicyPath || strings.HasPrefix(node.Path, reconcile.PolicyDirectory+"/") || strings.HasPrefix(node.Path, reconcile.PolicyPath+"/"):
			if node.Path != reconcile.PolicyPath && !reconcile.PolicyMemberPath(node.Path) {
				return policyRead{}, false, fmt.Errorf("unexpected policy member %q", node.Path)
			}
			if !node.Regular() {
				return policyRead{}, false, fmt.Errorf("policy member %s is not a regular blob", node.Path)
			}
			paths = append(paths, node.Path)
		}
	}
	sort.Strings(paths)
	files, err := m.repo.Files(ctx, commit, paths)
	if err != nil {
		return policyRead{}, false, fmt.Errorf("read reconciliation baseline at %s: %w", shortCommit(commit), err)
	}
	for _, name := range paths {
		if _, present := files[name]; !present {
			return policyRead{}, false, fmt.Errorf("policy tree at %s lists unavailable blob %s; restore the retained input before checking", shortCommit(commit), name)
		}
	}
	marker, ok := files[reconcile.PolicyPath]
	if !ok {
		if len(paths) != 0 || rootPresent {
			return policyRead{}, false, fmt.Errorf("policy members exist without the required marker %s", reconcile.PolicyPath)
		}
		return policyRead{}, false, nil
	}
	members := make(map[string][]byte, len(files)-1)
	for name, body := range files {
		if name != reconcile.PolicyPath {
			members[name] = body
		}
	}
	p, err := reconcile.ParsePolicySet(marker, members)
	if err != nil {
		return policyRead{}, false, fmt.Errorf("reconciliation baseline at %s: %w", shortCommit(commit), err)
	}
	if p.Schema == reconcile.BaselineSchema && rootPresent {
		return policyRead{}, false, fmt.Errorf("mixed v1 baseline and v2 policy directory")
	}
	evidence := policyEvidence{Schema: p.Schema, Generator: p.Generator, Commit: commit, Files: []policyFileIdentity{}}
	for _, name := range paths {
		evidence.Files = append(evidence.Files, policyFileIdentity{Path: name, SHA256: policyDigest(files[name])})
	}
	return policyRead{Policy: p, Evidence: evidence, Files: files}, true, nil
}

func policyDigest(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

// Pinned reconciliation uses each tree's own configuration and memory. Dirty
// worktree content never supplies evidence about a committed candidate or base.
func pinnedReconciliation(ctx context.Context, m *memory, commit, generator string) (reconcile.Snapshot, reconcile.Report, error) {
	opts, set, err := reconciliationStateAt(ctx, m, commit)
	if err != nil {
		return reconcile.Snapshot{}, reconcile.Report{}, err
	}
	opts = baselineReproductionOptions(opts, generator)
	snapshot, err := m.snapshotAt(ctx, commit, opts)
	if err != nil {
		return reconcile.Snapshot{}, reconcile.Report{}, err
	}
	legacyNative, trustedFeatures, err := trustedNativeServes(ctx, m, snapshot.Commit)
	if err != nil {
		return reconcile.Snapshot{}, reconcile.Report{}, err
	}
	opts.LegacyNativeServes = opts.LegacyNativeServes || legacyNative
	snapshot.TrustedPRDFeatures = trustedFeatures
	if !opts.LegacyContractCoverage {
		snapshot, err = contractHistorySnapshot(ctx, m, snapshot)
		if err != nil {
			return reconcile.Snapshot{}, reconcile.Report{}, err
		}
	}
	load := func(closeout reconcile.ActiveCloseout) (reconcile.Snapshot, error) {
		return closeoutSnapshotAt(ctx, m, closeout, commit)
	}
	if !opts.LegacySourceRetirement {
		snapshot = sourceRetirementSnapshot(ctx, m, snapshot, set, m.reconciliationBase)
	}
	report := reconcile.VerifyOfflineWithHistory(snapshot, set, m.projection(set), opts, load)
	return snapshot, report, nil
}

// trustedNativeServes keeps candidate-selected policy and a removed PRD ledger
// from turning document-captured requirements into native requirements.
func trustedNativeServes(ctx context.Context, m *memory, commit string) (bool, map[string]bool, error) {
	if m.reconciliationBase == "" {
		return false, nil, nil
	}
	base, err := m.repo.ResolveCommit(ctx, m.reconciliationBase)
	if err != nil {
		// The ratchet reports an invalid base with its existing diagnostic.
		// Until then, native authorization remains closed.
		return true, nil, nil
	}
	if base == commit {
		return false, nil, nil
	}
	read, present, err := readPolicyAt(ctx, m, base)
	if err != nil {
		return false, nil, err
	}
	baseOpts, _, err := reconciliationStateAt(ctx, m, base)
	if err != nil {
		return false, nil, err
	}
	entries, err := m.repo.ListTree(ctx, base, ".")
	if err != nil {
		return false, nil, fmt.Errorf("list trusted PRD ledgers: %w", err)
	}
	features := map[string]bool{}
	for _, entry := range entries {
		if feature, ok := reconcile.LedgerFeature(entry.Path, baseOpts.PRDs); ok {
			features[feature] = true
		}
	}
	// Native requirements were adopted in v6. A later evaluator revision must
	// not withdraw that authority just because its current version changed.
	return !present || baselineReproductionOptions(reconcile.Options{}, read.Policy.Generator).LegacyNativeServes, features, nil
}

func migrationPathsOnly(ctx context.Context, m *memory, base, head string) error {
	paths, err := m.repo.ChangedPaths(ctx, base, head)
	if err != nil {
		return err
	}
	for _, name := range paths {
		if name != reconcile.PolicyPath && !reconcile.PolicyMemberPath(name) {
			return fmt.Errorf("v1 to v2 migration must change only policy representation; also changed %s", name)
		}
	}
	return nil
}

func authoritativePolicyRef(ctx context.Context, m *memory, ref string, env Env) (string, error) {
	if env.Getenv("PYXIS_CI_EVENT") == "" && env.Getenv("PYXIS_CI_BASE") == "" && env.Getenv("GITHUB_ACTIONS") != "true" {
		return ref, nil
	}
	trusted, inventory, err := memoryGateBase(ctx, m, "", env)
	if err != nil {
		return "", err
	}
	if inventory {
		if ref != "" {
			return "", fmt.Errorf("scheduled inventory cannot select a different policy base")
		}
		return "", nil
	}
	if ref != "" {
		requested, err := m.repo.ResolveCommit(ctx, ref)
		if err != nil {
			return "", err
		}
		if requested != trusted {
			return "", fmt.Errorf("CI supplies the trusted base; --baseline-ref cannot override it")
		}
	}
	return trusted, nil
}
