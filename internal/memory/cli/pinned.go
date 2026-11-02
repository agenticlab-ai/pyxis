package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/memory/gitsrc"
	"github.com/agenticlab-ai/pyxis/internal/memory/reconcile"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
	"github.com/agenticlab-ai/pyxis/internal/memory/resolve"
	"github.com/agenticlab-ai/pyxis/internal/memory/store"
)

// pinnedState is what one invocation has already derived from pinned commits:
// each commit's memory tree and its projection, its reconciliation snapshot,
// and its evidence snapshot. A commit id fixes every input of these values, so
// each is derived once and then shared by the evidence, packet and
// reconciliation steps of that invocation, which treat them as read-only.
//
// Nothing here is keyed by a moving ref or read from the working tree, a
// failure is never kept, and nothing outlives the invocation.
type pinnedState struct {
	repo      *gitsrc.Repo
	sets      map[string]*pinnedSet                            // commit → memory tree
	snapshots map[string]reconcile.Snapshot                    // commit and options → sources
	evidence  map[string]map[string]reconcile.EvidenceSnapshot // declared paths → commit → sources
}

// pinnedSet is one commit's memory tree and, once asked for, its projection.
type pinnedSet struct {
	set        *store.Set
	projection *resolve.Projection
}

func newPinnedState(repo *gitsrc.Repo) *pinnedState {
	repo.KeepPinnedReads()
	return &pinnedState{
		repo: repo, sets: map[string]*pinnedSet{}, snapshots: map[string]reconcile.Snapshot{},
		evidence: map[string]map[string]reconcile.EvidenceSnapshot{},
	}
}

// holds reports whether a value derived from commit may be reused under ctx:
// only for a full commit id, and never once the context has ended, so a
// cancelled read still fails at the read itself.
func (p *pinnedState) holds(ctx context.Context, commit string) bool {
	return p != nil && ctx.Err() == nil && record.ValidCommitID(commit)
}

// evidenceSnapshots returns the evidence snapshots already built for exactly
// this declared path set, so sessions verifying the same records share them.
func (p *pinnedState) evidenceSnapshots(declared []string) map[string]reconcile.EvidenceSnapshot {
	if p == nil {
		return map[string]reconcile.EvidenceSnapshot{}
	}
	key := strings.Join(declared, "\x00")
	if p.evidence[key] == nil {
		p.evidence[key] = map[string]reconcile.EvidenceSnapshot{}
	}
	return p.evidence[key]
}

// baseSet loads the memory tree at base, once per pinned commit.
func (m *memory) baseSet(ctx context.Context, base string) (*store.Set, error) {
	if !m.pinned.holds(ctx, base) {
		return store.Load(ctx, gitsrc.TreeSource{Repo: m.repo, Ref: base, Prefix: store.Dir})
	}
	if held, ok := m.pinned.sets[base]; ok {
		return held.set, nil
	}
	set, err := store.Load(ctx, gitsrc.TreeSource{Repo: m.repo, Ref: base, Prefix: store.Dir})
	if err != nil {
		return nil, err
	}
	m.pinned.sets[base] = &pinnedSet{set: set}
	return set, nil
}

// projection is resolve.New(set). The working tree this memory holds and each
// pinned commit's tree are projected once; any other set is projected afresh.
func (m *memory) projection(set *store.Set) *resolve.Projection {
	if m.proj != nil && m.proj.Set == set {
		return m.proj
	}
	if m.pinned != nil {
		for _, held := range m.pinned.sets {
			if held.set != set {
				continue
			}
			if held.projection == nil {
				held.projection = resolve.New(set)
			}
			return held.projection
		}
	}
	return resolve.New(set)
}

// snapshotAt is reconcile.BuildSnapshot, built once per pinned commit and
// option set.
func (m *memory) snapshotAt(ctx context.Context, commit string, opts reconcile.Options) (reconcile.Snapshot, error) {
	if !m.pinned.holds(ctx, commit) {
		return reconcile.BuildSnapshot(ctx, m.repo, commit, opts)
	}
	key := fmt.Sprintf("%s\x00%#v", commit, opts)
	if snapshot, ok := m.pinned.snapshots[key]; ok {
		return snapshot, nil
	}
	snapshot, err := reconcile.BuildSnapshot(ctx, m.repo, commit, opts)
	if err != nil {
		return snapshot, err
	}
	m.pinned.snapshots[key] = snapshot
	return snapshot, nil
}
