# Work-item hierarchy query & index — design-brainstorm record

| Field | Value |
|---|---|
| **In one line** | Design direction for the local work tracker's parent→children query (FR-19) and its O(k) hierarchy lookup (NFR-8) — the index mechanism, its freshness/decision-path discipline, the query surface, and where the design lands. |
| **Relevant-when** | designing or changing the work-item hierarchy query, the children/rollup index, or Carina's parent→children surface; checking prior rulings on the index mechanism or refused shapes (denormalized `children:` header, directory-per-parent). |
| **Basis** | prd:local-work-tracker *(merged, effective As of 001 — FR-19, NFR-8)* |
| **Authority** | @mdarabi |
| **Date(s)** | 2026-07-15 |
| **Status** | gated: ready-for-design *(2026-07-15; after gating the record changes only through the introducing PR's review loop, and freezes at merge)* |
| **Tracking** | — |
| **Feeds** | docs/tech-design/worktracker-hierarchy-index.md |
| **Prior records** | none |
| **Template version** | design-brainstorm-record/v1 |

## Orientation

| Source | Why loaded |
|---|---|
| docs/tech-design/local-work-tracker.md | the live design this extends/supersedes — §3.5 hierarchy (child stores `parent`, children by scan), §3.6 queries + Q3 deferred index, DINV-1/DINV-4 |
| docs/prds/local-work-tracker/effective.md | the basis — FR-19, NFR-8 (+ FR-7 rollup, FR-14 strong read, FR-16 one-store, NFR-1/NFR-3) |
| docs/architecture-principles.md | AP-27 (aggregate derived, never authored), AP-15/INV-7 (exact fresh decision reads), AP-11/INV-30 (one predicate, one place) |
| docs/observability/incidents.md | checked for stale-index / hierarchy lessons — none matched (registry as of 2026-07-15) |
| docs/wiki/glossary.md | "index" already denotes the *design* index — term-collision risk for a work-item index |

**Overlaps surfaced:** The live `local-work-tracker` design already anticipates this — Q3 defers a store index ("added on measured need; decision-path reads bypass it; never authoritative", DINV-4) and R1 names it. This session *resolves* that deferred question for hierarchy; it does not contradict a live ruling, so the verdict is not `covered`.

## Design questions

- **DQ-1**: How is the parent→children query exposed — a dedicated `children <id>` verb, a `--parent <id>` filter on the existing `list`, or both — and should `show <epic>` additionally surface **closed** children (grouped by state) rather than the open-only rollup it prints today?
- **DQ-2**: What mechanism makes children + rollup lookup **O(k), not O(N)** — a derived rebuildable index, an authored `children:` header on the parent, a directory-per-parent layout, or an in-memory index rebuilt per process? *(opens a seam — the index artifact + its maintenance)*
- **DQ-3**: For the chosen mechanism — when is it built/updated, how is staleness detected and crash-safety preserved (the index vs. the atomic item write), and which reads **bypass it to a direct scan** on the decision path (DINV-4)?
- **DQ-4**: Does this land as a design that **supersedes** the frozen `local-work-tracker` design, or an additive **builds-on** design (both live) that resolves its Q3 for hierarchy only?

## Panel log

*One line lands per DQ as it is grilled (DINV-7).*

- **DQ-1**: extends — skipped — the query reuses the existing `Children` port method and the `list`/`show` CLI seams; no new port/interface seam opens (authority did not force).
- **DQ-2**: opens — fired — the index mechanism opens a new artifact + maintenance seam; three panelists spawned under orthogonal constraints (integrity · simplicity · performance). → chosen **ALT-1** (marker directory, D-3); ALT-2 rejected (hardlink unsound under rename-replace, F-9), ALT-3 rejected (AP-27 shared-append), ALT-4/5/6 rejected.
- **DQ-3**: extends — skipped — the maintenance/consistency discipline extends the DQ-2 index seam; no new seam opens (benchmark-informed, D-5).
- **DQ-4**: extends — skipped — a design-corpus placement call (supersede vs builds-on), not a code/mechanism seam; governed by the tech-design conventions, no panel.

## Decisions

### D-1 — Query surface: `list --parent` filter + all-states `show`

- **Question**: Expose parent→children as a `children` verb, a `list --parent` filter, or both — and should `show <epic>` list closed children too?
- **Answer**: A `--parent <id>` filter on the existing `list` (composing with `--state`/`--signal`/`--role`), plus `show <epic>` listing children grouped by open+closed; no separate `children` verb.
- **Reasoning**: reuses the existing `Children` port method and the `list`/`show` seams (no port-method proliferation); one filterable path for every "children of X where…" query (AP-11, one place); all-states `show` closes the completed-epic gap directly.
- **Anchor**: 2026-07-15 · DQ-1 ruling
- **Status**: live

### D-2 — Usage scope: single machine, single user, multiple processes

- **Question**: Must Carina handle concurrent writers across multiple *clones* (distributed/offline, reconciled by git merge), or is its scope narrower?
- **Answer**: One machine, one user, multiple processes — that is the whole target. Multi-user / multi-clone collaboration is **out of scope**; a team that needs it uses a hosted medium (GitHub Issues / Linear / Jira) through the WorkTracker port's hosted adapter. A git-merge conflict from two writers means it is being used outside its design.
- **Reasoning**: the operator's product boundary, aligning with the PRD anti-segment + NG-4 (single-tenant; the OS filesystem is the permission boundary). The remote is a **backup of one writer's truth**, not a collaboration point; the only real concurrency is multi-process on one clone, already handled by per-item `flock` + atomic writes.
- **Anchor**: 2026-07-15 · scope clarification
- **Status**: live

### D-3 — Index mechanism: reverse-parent marker directory (ALT-1)

- **Question**: Which O(k) mechanism — the reverse marker directory (ALT-1), the per-parent sidecar file (ALT-3), or the authored header (ALT-4)?
- **Answer**: ALT-1 — a reverse-parent directory of empty per-edge marker files (`rev/<parentID>/<childID>`).
- **Reasoning**: three lenses converged — the panel (simplicity), git-merge behavior (per-edge files merge clean; scope-retired by D-2 but still the cleaner shape), and the concurrency architecture (decompose so writers touch disjoint files — no coordinator/daemon, AP-27). Empty markers add no parser and no whole-file rewrite; membership-only + a fresh `Get` sidesteps every stale-value class.
- **Anchor**: 2026-07-15 · DQ-2 ruling
- **Status**: live

### D-4 — Placement: the index is gitignored, rebuildable, local

- **Question**: Is `rev/` committed to git (self-contained store) or gitignored (git carries only the truth)?
- **Answer**: gitignored, rebuildable, local — git versions only the authoritative item files; `rev/` is a derived accelerator, never pushed.
- **Reasoning**: the index is derived state; committing it duplicates the truth, adds diff noise + repo bloat, and (absent D-2's single-writer scope) would risk post-merge staleness. Single-user makes it obvious — the remote backs up one writer's truth; a rebuildable cache has no business there. Rebuild is one O(N) `reindex` per clone.
- **Anchor**: 2026-07-15 · placement ruling
- **Status**: live

### D-5 — Consistency discipline: incremental writes + rebuild-as-heal

- **Question**: How does the index stay honest — incremental maintenance vs rebuild-on-write, and how are drift + decision paths handled?
- **Answer**: Incremental at the choke point (create/unlink one marker per parent-write, field-first under the child's lock); a dumb full `reindex` is the *sole* recovery — run on first use, on a detected mismatch, and on a cheap dirty-check (hand-edit). Decision-path reads verify each candidate's `parent:` field; display reads trust the markers.
- **Reasoning**: measured on-hardware — the incremental write (~40 µs) is ~160,000× cheaper than a full rebuild (~6.5 s at 100k), so rebuild-on-write is off the table (blows NFR-1, goes quadratic on bulk); but a full rebuild is cheap enough *occasionally* (~ms at typical scale) to be the ONLY drift recovery, which keeps the maintenance logic dumb-simple. Markers can only lag, never hold a wrong value (ALT-1).
- **Anchor**: 2026-07-15 · DQ-3 ruling (benchmark-informed)
- **Status**: live

### D-6 — Design placement: a builds-on design resolving Q3

- **Question**: Supersede the frozen local-work-tracker design, or a lighter builds-on resolving its deferred Q3?
- **Answer**: A builds-on additive design (working name `worktracker-hierarchy-index`) — `Serves` FR-19 / NFR-8 / FR-7, `Builds-on` local-work-tracker, resolves its deferred Q3 for hierarchy; both designs stay live.
- **Reasoning**: no contradiction (§3.6 deferred the index via Q3; DINV-1's "no *authoritative* reverse index" holds — ours is non-authoritative; the scan stays the rebuild/verify path), so supersession is not required; proportional (a focused diff vs a whole-document rewrite); precedent — `plan-viewer-v1` and `plan-new-v1` already `Builds-on` local-work-tracker.
- **Anchor**: 2026-07-15 · DQ-4 ruling
- **Status**: live

## Alternatives

### ALT-1 — Reverse-parent marker directory (empty per-edge files)

- **Shape**: one zero-byte file per edge at `<root>/rev/<parentID>/<childID>` (parent dir = the shard). `Children(P)` = `ReadDir(rev/P)` → k ids → a fresh `Get` of each (content always current). Rollup counts `open` from those Gets. Maintained at the edge choke point (F-10) under the child's lock: create/unlink the marker. Non-authoritative, rebuildable.
- **Trade-offs**: read **O(k)** (one k-entry `ReadDir` + k header Gets); write **O(1)** (±1 marker syscall, unfsynced); rebuild **O(N)** on demand; +1 inode/edge; introduces the store's first non-atomic two-write (item + marker) — contained because the marker is non-authoritative and rebuildable. `ls rev/42/` literally lists epic 42's children.
- **Principles**: AP-27 **honored** — each edge self-registers as its *own file*; concurrent creates/re-parents never touch a shared aggregate (file-not-line). DINV-1/DINV-4 **honored** — marker is membership-only; decision reads re-`Get` the child's `parent`, never trusting the marker's existence. AP-15/INV-7 **honored** — content is always fresh via `Get`. AP-23 **honored**. Strains DINV-2 — the item+marker pair is not one atomic act (degrades to staleness, never corruption).
- **Incidents**: no index-drift lesson in the registry (incidents.md, 2026-07-15); AP-15's own scar (a stale index-read hid a shipped item) is the governing constraint — discharged by keeping every decision read off `rev/`.
- **Verdict**: **chosen (recommended)** — the AP-27-cleanest and simplest *sound* shape that hits O(k); membership-only + fresh `Get` sidesteps every stale-**value** class of drift, leaving only set-membership drift (cheap to detect + rebuild).
- **From**: DQ-2 · panel (simplicity)

### ALT-2 — Reverse-parent hardlink directory

- **Shape**: as ALT-1, but each entry is a **hardlink** to `items/<child>.md`, intended so the query reads the child's bytes *through* the link (no separate `Get`).
- **Trade-offs**: same O(k) read; aimed at avoiding the per-child `Get`.
- **Principles**: AP-27 / DINV-1 as ALT-1; maps the drift risk to I2/AP-26 (never trust a derived surface — confirm against the authoritative object).
- **Incidents**: I2 (2026-07-09) — the same derived-vs-truth divergence shape, by analogy.
- **Verdict**: **rejected** — unsound under F-9: item writes are rename-**replace**, which swaps the child's inode, so a link set at parent-time points at the old, orphaned inode and shows **stale content** after any later child edit. Reading via the link would demand re-linking on *every* child write, not just parent changes. Stripped of read-via-link it degenerates to ALT-1 (name-only + fresh `Get`), which dominates. Its verify-on-decision-path + mtime dirty-flag are **hybridized into the chosen direction**.
- **From**: DQ-2 · panel (integrity)

### ALT-3 — Per-parent child-id sidecar file

- **Shape**: one file per parent at `index/children/<parentID>.children` = a sorted, newline-delimited child-id list. Read = 1 file read + k `Get`s; write = rewrite the old/new parent's sidecar (atomic) under a per-sidecar lock at the choke point; `reindex` rebuilds via one O(N) scan.
- **Trade-offs**: read **O(k)** (1 + k); write **O(k_parent)** whole-file rewrite; rebuild **O(N)**; decision paths run "verify mode" (re-prove each candidate's `parent`).
- **Principles**: DINV-1/DINV-4 **honored** (non-authoritative; decision paths re-prove). **Strains AP-27** — the sidecar is a *shared per-parent append point*: N children of one epic all rewrite the **same file** (the conflict magnet AP-27 names a defect), serialized by lock but not eliminated.
- **Incidents**: none on index-drift; AP-15 governing (verify-mode discharges it).
- **Verdict**: **rejected in favor of ALT-1** — identical O(k) and discipline, but re-introduces the shared-append magnet AP-27 forbids; ALT-1's per-edge file avoids it entirely. Its **typed hint-vs-decision read modes** and the **`reindex` verb** are adopted into the direction.
- **From**: DQ-2 · panel (performance)

### ALT-4 — Authored `children:` header on the parent (denormalized)

- **Shape**: store the child-id list *in the parent's item-file header*; read = parse the parent (O(k)); write = rewrite the parent on every child edge (the operator's original instinct).
- **Trade-offs**: O(k) read from one file, but write-amplifies the parent on every child create/reparent, and the list is a **second copy of the edge in an authoritative file**.
- **Principles**: **violates AP-27** (every child edits the parent's one line — a shared append point) and **strains DINV-1** (two authoritative copies that can disagree; a hand-edit of either desyncs). AP-15 risk: the header list invites being trusted as truth.
- **Incidents**: I2/AP-26 as a standing warning (derived-vs-truth divergence).
- **Verdict**: **rejected** — puts a driftable second copy in an authoritative file and makes the parent a hot, shared-append write; the reverse-directory keeps one truth + a disposable cache.
- **From**: DQ-2 · hand-authored (the baseline the panel improves on)

### ALT-5 — Directory-per-parent as primary layout

- **Shape**: store children physically under the parent — `items/<parentID>/<childID>.md`; `Children` = `ReadDir` (O(k)) natively, no sidecar.
- **Trade-offs**: O(k) with no derived state, but restructures the whole store.
- **Principles**: breaks the flat `items/<id>.md` model, `O_EXCL` flat-id allocation, and makes reparent a file **move**; the frozen design (§4) already **rejected** directory-per-item for inode/atomic-write reasons.
- **Incidents**: none checked (incidents.md, 2026-07-15).
- **Verdict**: **rejected** — contradicts a settled §4 alternative; far too invasive for an additive query.
- **From**: DQ-2 · hand-authored

### ALT-6 — In-memory index rebuilt per process

- **Shape**: scan the store once at process start, hold parent→children in memory for the session.
- **Trade-offs**: O(k) reads after warmup, but the CLI is short-lived and daemon-free (AP-19), so it pays **O(N) rebuild on every invocation**.
- **Principles**: strains AP-19 (rebuild per CLI run); helps only a long-lived engine, which does not exist yet.
- **Incidents**: none checked (incidents.md, 2026-07-15).
- **Verdict**: **rejected** — O(N) per CLI invocation is worse than today's scan for the actual usage model.
- **From**: DQ-2 · hand-authored

## Seam map

- **S-1**: the `list` CLI verb — extends — behind it: a `--parent <id>` filter composing with the existing state/signal/role filters — feeds: design Scope (query surface) · Serves prd:local-work-tracker/FR-19. (from D-1)
- **S-2**: the `show <epic>` rollup — extends — behind it: children listed grouped by open+closed, superseding the open-only rollup — feeds: Scope (rollup surface) · Serves FR-19, FR-7. (from D-1)
- **S-3**: the `rev/` reverse-parent index — **opens** — behind it: a gitignored, non-authoritative, rebuildable directory of empty per-edge markers (`rev/<parent>/<child>`), maintained incrementally at the edge choke point (F-10) and healed by a `reindex` verb — feeds: design Scope (the index + `reindex`) · Serves NFR-8. (from D-3/D-4/D-5)

## Facts

- **F-1**: FR-19 requires a parent→children query returning every direct child across all states by default (optional state filter), through one implementation shared by the port and the CLI. — source: prd:local-work-tracker/FR-19
- **F-2**: NFR-8 requires children + rollup lookup bounded by the child count k, not the corpus size N — no full-corpus scan on the query path — while the child's `parent` field stays the sole authoritative record of the edge. — source: prd:local-work-tracker/NFR-8
- **F-3**: The frozen design stores the edge once, on the child (`parent:`); children are enumerated by a direct store scan; no parent→children index is authoritative (DINV-1). — source: design:local-work-tracker#3.5
- **F-4**: The frozen design defers a store index to measured need, with decision-path reads always bypassing it and it never becoming authoritative (Q3, R1, DINV-4). — source: design:local-work-tracker#3.6
- **F-5**: AP-27 — a genuinely needed central aggregate is derived, never authored (self-registration, scan, or build); no author's diff touches the aggregate. — source: architecture-principles#AP-27
- **F-6**: AP-15/INV-7 — reads feeding destructive or dispatch decisions must be exact and fresh, never an index-derived value that can lag. — source: architecture-principles#AP-15
- **F-7**: The shipped `Children(parentID)` reads every item via a full `Get` and filters on `parent` — O(N) full parses to find one parent's k children. — source: internal/worktracker/localfs/hierarchy.go
- **F-8**: Epics are excluded from the ready-set as structural rollups, so the children rollup is off the *readiness* decision path today (but FR-14 strong-read still governs any decision-feeding read of the set). — source: design:local-work-tracker#3.6
- **F-9**: Every item mutation is a whole-file atomic replace — temp + fsync + `rename()` **over the target** — which swaps the item's inode on every write. (Decisive against a hardlink-based reverse edge: a link set at parent-time points at the old, orphaned inode after any later child edit.) — source: design:local-work-tracker#3.4
- **F-10**: Parent mutation funnels through one choke point — `Create` (born with a parent), `SetParent`, `ClearParent` → `writeParent` → `mutate`, already under the child's per-item `flock` — the single place a reverse edge would be maintained. — source: internal/worktracker/localfs/hierarchy.go
- **F-11**: The work-item store is bucket-1 — git-versioned and **pushed to a remote for offline/multi-writer collaboration** (§3.8). So any reverse structure is evaluated on **git behavior** too: if committed it must be **merge-clean** and must not go stale against the truth after a merge; a derived structure that can't guarantee both should be **gitignored** (bucket-2, rebuildable). *Merge behavior:* a per-edge **file** (ALT-1) merges cleanly — two branches adding children to one epic add *different* files (no conflict); a shared per-parent **file/line** (ALT-3 sidecar, ALT-4 header) is an **add/add merge-conflict magnet** on every concurrent child. Git independently condemns the shared-append shapes — the same verdict AP-27 reaches. — source: design:local-work-tracker#3.8 · operator constraint 2026-07-15

## Fog

none — every framed design question (DQ-1..4) resolved to a live ruling; no blocking or non-blocking fog remains.

## Out of scope

- **O-1**: multi-user, multi-clone, and distributed/offline concurrent writing — served by the hosted WorkTracker adapters (GitHub/Linear/Jira), never by Carina; a git-merge conflict is the signal of out-of-scope usage. (from D-2)

## Candidate glossary terms

- **T-1**: **reverse-parent directory** (`rev/`) — the derived, non-authoritative, gitignored accelerator mapping a parent to its child markers; the child's `parent:` field stays the sole truth. Avoid bare "index" (collides with the *design* index) and "children index". The authored design should adopt this term in its glossary/§3.

## Parked product gaps

- **P-1**: the item id scheme (`O_EXCL` sequential ints, §3.3) is collision-free across concurrent processes on **one** clone but not across concurrent **offline clones** (two clones each mint `#50` → an add/add `items/50.md` conflict on push) — anchors: design:local-work-tracker#3.3 · prd:local-work-tracker (Q2) — disposition: **proceed-anyway per D-2** (cross-clone concurrency is out of scope; the single-clone `O_EXCL` guarantee suffices for the intended usage; the concern belongs to a hosted adapter, not Carina).

## Gate

| Field | Value |
|---|---|
| **Verdict** | ready-for-design |
| **Date** | 2026-07-15 |
| **Authority** | @mdarabi |
| **Covering design** | — |
| **Coverage check** | pass — core four covered (§2 Decision, §4 Alternatives, Serves+Scope, §3.2 boundaries), zero blocking fog |
| **Amend route** | — |
