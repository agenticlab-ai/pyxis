# Work-item hierarchy query and the reverse-parent index — plan 002

| Field | Value |
|---|---|
| **Feature** | local-work-tracker |
| **PRD** | docs/prds/local-work-tracker |
| **Design** | docs/tech-design/worktracker-hierarchy-index.md |
| **Plan version** | 002 |
| **Dialect** | harbormaster/fs-spec:v1 |
| **Epic title** | Carina — work-item hierarchy query and the reverse-parent index |
| **Status** | Draft — freezes at merge |
| **Template version** | plan-template/v1 |

## Tasks

### T1 — Stand up the reverse-parent marker index and maintain it at the choke point

| Field | Value |
|---|---|
| **Blocked by** | — |
| **Size band** | 150–210 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | rung=2 |
| **Pattern** | — |
| **Touches** | internal/worktracker/localfs, internal/worktracker/conformance |

**Goal:** The store keeps a derived reverse-parent index — one zero-byte marker per parent→child edge, sharded under a gitignored directory — current at the single parent-mutation choke point and rebuildable in full from the item files alone.

**Current behavior:** The child's stored `parent` is the only record of an edge; the reverse direction is a full-store scan and no reverse index exists on disk.

**Desired behavior:** Creating a child with a parent, reparenting, and clearing a parent each write or remove exactly one marker, and only after the authoritative item write has committed (field-first), under the child's existing per-item lock. Markers are empty — membership is presence — so the index can only lag, never hold a wrong value. The index directory is gitignored and never committed. A rebuild reconstructs the whole index purely from the item `parent` fields, and a read finding the index absent triggers that build. Concurrent parent-writes to different children under one parent touch disjoint marker files and lose none.

**Scope:**
- the sharded per-edge marker layout and the gitignore entry that keeps it out of version control
- incremental one-marker maintenance inside the set-parent / clear-parent / create choke point, ordered strictly after the item write
- the pure rebuild-from-item-fields primitive and the lazy build invoked when the index is absent
- the disjoint-file property under concurrent parent-writes

**Out of scope:**
- the parent→children query surface and read modes (T2)
- the operator-facing recovery verb, the dirty-check trigger, and crash recovery (T3)

**Acceptance criteria:**
- [ ] after a create-with-parent, a reparent, and a clear, the reverse index lists exactly each parent's current children and the vacated edge's marker is gone
- [ ] the index directory never appears in git status, and a rebuild from an empty index reconstructs every edge from the item files alone
- [ ] a marker operation is observed only after the item write has committed, verified by a stop injected between the two writes
- [ ] concurrent parent-writes to different children under one parent complete with no lost marker and touch disjoint files

**Key interfaces:**
- the marker path and shard helpers, and the gitignored index root beside `items/`
- the edge-maintenance hook invoked from within the existing parent mutation, after the atomic item write
- the rebuild-from-truth primitive that derives the whole index from the item `parent` fields

**Traces:** prd:local-work-tracker/NFR-8 · design:worktracker-hierarchy-index#3.6 · design:worktracker-hierarchy-index#3.7 · design:worktracker-hierarchy-index#DINV-3 · design:worktracker-hierarchy-index#DINV-4 · design:worktracker-hierarchy-index#DINV-5 · design:worktracker-hierarchy-index#DINV-6

### T2 — Parent→children query: the shared O(k) enumerator and `list --parent`

| Field | Value |
|---|---|
| **Blocked by** | T1 |
| **Size band** | 150–220 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/worktracker, internal/worktracker/localfs, internal/worktracker/conformance, cmd/carina |

**Goal:** A work item's direct children are queryable in time bounded by the child count through one implementation shared by the port and the `carina list --parent` CLI, with a display mode that trusts the index and a decision mode that re-verifies each candidate's authoritative parent.

**Current behavior:** Children are found by scanning and filtering every item (bounded by the whole store); `Children` takes only a parent id, there is no parent filter on the list surface, and there is no display-versus-decision distinction.

**Desired behavior:** A parent-scoped query returns exactly the given item's direct children, all states by default, narrowable by the existing state, signal, and role filters, and byte-identical from the CLI and the port; a parent with no children returns empty, not an error. Lookup reads the parent's markers and then each named child's authoritative file, so latency stays flat as the total item count grows with a fixed child count, with no full-corpus scan on the query path. A decision-mode read re-reads each candidate's `parent` field and drops any that no longer names this parent; a display-mode read trusts the markers. A read that finds the index absent builds it first.

**Scope:**
- the shared child enumerator over the marker index, carrying the display and decision read modes
- the parent filter on the list query, composing with the state, signal, and role filters
- the `carina list --parent` surface with machine output, byte-identical to the port
- the read-time lazy build when the index is absent

**Out of scope:**
- the grouped epic rollup and the `show` surface (T4)
- the operator-facing recovery verb and the dirty-check trigger (T3)

**Acceptance criteria:**
- [ ] a parent-scoped list returns exactly the item's direct children in every state, byte-identical from the CLI and the port; a state filter narrows to the expected members; a childless parent returns empty
- [ ] children-query latency stays flat as total item count grows across the capacity range at fixed child count, with no full-corpus scan on the path
- [ ] a decision-mode read drops a stale or hand-edited marker by re-verifying each candidate's parent, while a display-mode read over the same store still lists it
- [ ] a query run against an absent index builds the index and returns the correct children

**Key interfaces:**
- `Children` — the enumerator gains a child filter and a read-mode argument, one implementation over the index
- the read-mode selector distinguishing a display read from a decision read
- the list query's parent field composing with state, signal, and role
- the `carina list --parent` verb and its machine output

**Traces:** prd:local-work-tracker/FR-19 · prd:local-work-tracker/NFR-8 · prd:local-work-tracker/FR-14 · design:worktracker-hierarchy-index#3.1 · design:worktracker-hierarchy-index#3.7 · design:worktracker-hierarchy-index#DINV-2 · design:worktracker-hierarchy-index#DINV-7

### T3 — `reindex`: the sole drift recovery, dirty-check, and crash restoration

| Field | Value |
|---|---|
| **Blocked by** | T1 |
| **Size band** | 110–170 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | parallel branch over the store, kit, and CLI packages; the recovery verb, its conformance rows, and its fault fixtures each land in their own file — extension is by new file, not shared-line append (AP-27) — so the lone shared registration line is the only contended edit and falls to the recovery loop |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/worktracker/localfs, internal/worktracker/conformance, cmd/carina |

**Goal:** A single reindex operation rebuilds the reverse index wholly from the item files — the only drift recovery — exposed as `carina reindex`, healing a torn item-then-marker pair or a hand-edit, and gated by a cheap dirty check before an exact read.

**Current behavior:** The rebuild-from-truth primitive exists (T1) but is not reachable as a verb, has no trigger beyond first-use absence, and no crash or hand-edit recovery is demonstrated.

**Desired behavior:** `carina reindex` rebuilds the index from the item `parent` fields and reports its trigger, its item and marker counts, and its duration. A crash injected between the item write and the marker operation leaves the item correct and the index stale by one edge, which reindex restores. A hand-edited `parent` that bypassed the CLI trips a cheap dirty check that rebuilds the index before the next exact read. Recovery is always a full rebuild — no incremental self-correction path exists.

**Scope:**
- the `carina reindex` verb over the rebuild primitive, reporting trigger, item and marker counts, and duration
- crash-injection recovery across the item-then-marker cut point
- the dirty-check signal that gates a rebuild before an exact read, healing a hand-edit
- the reindex and fault conformance rows

**Out of scope:**
- any incremental self-correction of the index — recovery is a full rebuild by design
- the enumerator, list, and rollup surfaces (T2, T4)

**Acceptance criteria:**
- [ ] `carina reindex` reconstructs the index to match the item files and reports its trigger, its item and marker counts, and its duration
- [ ] a crash injected between the item write and the marker operation leaves the item correct, and reindex restores the missing marker
- [ ] a hand-edited parent trips the dirty check and a rebuild heals the index before the next exact read
- [ ] the only recovery path is a full rebuild — no incremental self-correction exists (inspection)

**Key interfaces:**
- the reindex operation reporting its trigger, counts, and duration
- the `carina reindex` verb and its machine output
- the dirty-check signal comparing index freshness against the item tree

**Traces:** prd:local-work-tracker/NFR-8 · prd:local-work-tracker/FR-14 · design:worktracker-hierarchy-index#3.7 · design:worktracker-hierarchy-index#3.8 · design:worktracker-hierarchy-index#DINV-5 · design:worktracker-hierarchy-index#DINV-6

### T4 — The epic rollup grouped by open and closed (`show`)

| Field | Value |
|---|---|
| **Blocked by** | T2 |
| **Size band** | 90–150 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/worktracker, internal/worktracker/localfs, internal/worktracker/conformance, cmd/carina |

**Goal:** An epic's `show` reports its children grouped into open and closed sets — the all-states rollup — computed over the shared child enumerator, replacing today's open-only rollup so a completed epic still surfaces its full task set.

**Current behavior:** The rollup derives only the open children from a `Children` scan, so `show <epic>` omits closed children and a completed epic's task set is reachable only by grepping the item files.

**Desired behavior:** `show <epic>` reports its children grouped into open and closed sets — the count and identity of each — computed over the one shared child enumerator, so the port's rollup and the CLI agree. The grouping matches a hand-computed expectation over a fixture corpus, and a completed epic surfaces its full closed task set through the rollup.

**Scope:**
- the rollup over the shared enumerator returning open and closed child groups
- the `carina show` grouped presentation replacing the open-only rollup
- the rollup conformance row against a fixture corpus

**Out of scope:**
- any richer traversal — transitive descendants, ancestor chains, cross-repo hierarchy — which a future design owns
- any auto-close sweep of an epic from its children's states

**Acceptance criteria:**
- [ ] `show <epic>` reports its children grouped into open and closed sets, count and identity each, matching a hand-computed expectation over a fixture corpus
- [ ] the rollup is computed over the one shared child enumerator, and the port's rollup and the CLI return the same grouped result
- [ ] a completed epic surfaces its full closed task set through the rollup rather than an empty open-only view

**Key interfaces:**
- the rollup returning the open and closed child groups over the shared enumerator
- the `carina show` grouped output replacing the open-only rollup

**Traces:** prd:local-work-tracker/FR-7 · design:worktracker-hierarchy-index#3.1 · design:worktracker-hierarchy-index#DINV-7 · design:worktracker-hierarchy-index#A-7

## Coverage

| Requirement | Tasks |
|---|---|
| prd:local-work-tracker/FR-7 | T4 |
| prd:local-work-tracker/FR-19 | T2 |
| prd:local-work-tracker/NFR-8 | T1, T2, T3 |
