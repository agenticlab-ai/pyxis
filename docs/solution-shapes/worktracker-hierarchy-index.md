# Work-item hierarchy query & the reverse-parent index — solution shape

| Field | Value |
|---|---|
| **Design** | worktracker-hierarchy-index |
| **Status** | authoritative |

*Concrete forms for the design's decisions — authoritative as the implementation reference until the code exists, then `superseded-by-code`. Names/paths shown are the intended shape; final spellings fix at implementation. Each element is captioned with the `DINV-n`/`A-n` it realizes in the paired design.*

## On-disk layout

The child's `parent:` stays the sole truth while `rev/` holds one zero-byte marker per edge — derived, sharded, and gitignored (**DINV-1**, **DINV-3**, **DINV-4**):

```
<work_items_root>/
  items/<id>.md                    # the truth — child stores `parent:` (unchanged)
  rev/                             # DERIVED, gitignored, rebuildable (DINV-3)
    <shard>/<parentID>/<childID>   # one ZERO-BYTE marker per edge (DINV-1 non-authoritative, DINV-4 per-edge)
  .gitignore                       # contains: rev/
```

Membership = the marker's *presence*; it carries no bytes, so it can only lag, never hold a wrong value. `ls rev/<shard>/42/` lists epic 42's children. Sharding threshold is design Q2.

`rev/` is never committed — git versions only the authoritative item files (**DINV-3**):

```gitignore
# <work_items_root>/.gitignore
rev/
```

## Port / core interfaces

One `Children`/`Rollup` implementation shared by port and CLI (**DINV-7**), carrying the two read modes (**DINV-2**) and the `Reindex` recovery (**DINV-6**); reads are O(k) (**A-2**) and return exact direct children (**A-1**), the rollup grouped by state (**A-7**):

```go
// package worktracker (consumer-side port — additive; existing methods unchanged)

type ReadMode int
const (
    Display  ReadMode = iota // trust the markers; O(k); for list --parent / show
    Decision                 // verify mode: re-read each candidate's parent (DINV-2)
)

// Children returns a work item's direct children, optionally narrowed. mode=Decision
// re-verifies each candidate's authoritative `parent` and drops any that no longer
// names parentID (DINV-2). O(k) in the child count, never O(N) (A-2, A-1).
Children(ctx context.Context, parentID int, filter ChildFilter, mode ReadMode) ([]WorkItem, []schema.Defect)

// Rollup is the FR-7 rollup over Children (count + identity), grouped by state (A-7).
Rollup(ctx context.Context, parentID int, mode ReadMode) (RollupView, []schema.Defect)

// Reindex rebuilds rev/ wholly from the item `parent` fields — the sole drift
// recovery, pure O(N), off the hot path (DINV-6).
Reindex(ctx context.Context) (ReindexReport, []schema.Defect)

type ChildFilter struct { States []State; Signal string; Role SignalRole } // empty States = all (FR-19)
type RollupView   struct { Open []WorkItem; Closed []WorkItem }
type ReindexReport struct { Trigger string; Items, Markers int; Dur time.Duration }
```

The `list --parent` filter composes with the existing list filters (**A-1**, design §3.1):

```go
type ListQuery struct {
    State  string
    Signal string
    Role   string
    Parent *int // NEW — nil = no parent filter; set = direct children of Parent
}
```

## Incremental maintenance at the choke point

The reverse edge is maintained inside the existing `mutate`, **after** the atomic item write commits (field-first), under the child's per-item lock — one marker op per parent change, no shared write; a torn pair heals by `Reindex` (**DINV-5**, **DINV-4**, **DINV-6**):

```go
// localfs — inside writeParent/create, AFTER the item write commits (DINV-5):
func (s *Store) reindexEdge(childID int, oldParent, newParent string) {
    if oldParent != "" {
        _ = os.Remove(markerPath(s.rev, oldParent, childID)) // idempotent; absent = no-op
    }
    if newParent != "" {
        d := parentDir(s.rev, newParent)
        _ = os.MkdirAll(d, 0o755)
        _ = os.WriteFile(markerPath(s.rev, newParent, childID), nil, 0o644) // ~O(1); empty (DINV-4)
    }
    // No fsync, no lock beyond the child's: a lost/torn marker degrades to staleness,
    // healed by Reindex (DINV-6). The item write already committed (DINV-5).
}
```

## The two read modes

A decision read re-reads each candidate's authoritative `parent` and drops any stale/hand-edited marker — the marker set is never trusted on a decision path (**DINV-2**); display reads trust the markers for speed, and a childless parent returns empty (**A-1**):

```go
func (s *Store) Children(ctx, parentID int, f ChildFilter, mode ReadMode) ([]WorkItem, []schema.Defect) {
    ids := s.readMarkers(parentID)          // ReadDir(rev/.../parentID) → k ids; build lazily if rev/ absent (DINV-6, A-4)
    out := make([]WorkItem, 0, len(ids))
    for _, id := range ids {
        w, defs := s.Get(ctx, id)           // ALWAYS the authoritative file — content fresh
        if len(defs) > 0 { continue }       // stale marker → child gone; drop (swept by Reindex)
        if mode == Decision && w.Parent != itoa(parentID) {
            continue                        // hand-edited/lagged marker → drop (DINV-2)
        }
        if f.matches(w) { out = append(out, w) }
    }
    sortByCreation(out)                      // stable order (INV-12)
    return out, nil
}
```

## CLI surface

The parent→children surface reuses the existing `list` flags (no new verb) plus the grouped `show` rollup and the one new `reindex` recovery verb (**A-1**, **A-7**, design §3.1; rebuild is **DINV-6**):

```
carina list --parent <id> [--state open|closed] [--signal <name>] [--role <role>] [--json]
                                  # direct children of <id>, all states by default (FR-19 / A-1)
carina show <epic-id>             # rollup grouped:  open children: N [..]
                                  #                  closed children: M [..]   (FR-7 / A-7)
carina reindex [--json]           # rebuild rev/ from the item files (DINV-6); prints ReindexReport
```
