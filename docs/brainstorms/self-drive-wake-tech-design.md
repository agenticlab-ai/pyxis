# Self-drive wake — design-brainstorm record

| Field | Value |
|---|---|
| **In one line** | Solution-space session for the merged wake-capability contract (prd:self-drive-wake): how the state reader (batched proposal reads on both media), the in-drive wake loop, the status verb, and the budget guards are actually built — and how the new surface relates to the live argus wake stack (wake-channel-v1 + internal/wake), the executor ports it must extend, and the not-yet-merged feature-drive lifecycle that will consume it. |
| **Relevant-when** | designing or changing the state-reader port and its GitHub/Carina adapters, the drive-side wake loop, the status verb, wait-age thresholds, or the write throttle; checking prior design rulings or refused shapes about wake/read machinery — the wake-channel-v1 relationship ruling, the port-placement ruling; resolving prd:self-drive-wake Q1/Q2 |
| **Basis** | `prd:self-drive-wake` (merged ledger — squash `00cda744`, fold `0d2d047e…`, as of 000) |
| **Authority** | @mdarabi (operator) |
| **Date(s)** | 2026-07-30 |
| **Status** | gated: ready-for-design *(after gating, the record changes only through the introducing PR's review loop — rulings added or superseded with review anchors — and freezes entirely at merge)* |
| **Tracking** | — |
| **Feeds** | `docs/tech-design/state-reader-v1.md` · `docs/tech-design/wake-watch-v1.md` |
| **Prior records** | `brainstorm:self-drive-wake` · `design-brainstorm:argus-unattended-drive-tech-design` |
| **Template version** | design-brainstorm-record/v1 |

## Orientation

| Source | Why loaded |
|---|---|
| `docs/prds/self-drive-wake/effective.md` | the basis — FR-1..FR-13, UB-1..6, NR-1..2, NFR-1..5, Q1/Q2, the everything-or-nothing appetite |
| `docs/tech-design/index.md` | routing — which live designs' Serves touch this ground; surfaced `wake-channel-v1` (live, serves prd:argus-unattended-drive FR-13..17/UB-5/7/9/NR-5/NFR-2/4/ABUSE-3), `implement-now` (live, owns the executor and today's ports), `fleet-bridge-v1` + `maintenance-sweep-v1` (both Builds-on wake-channel-v1), `local-work-tracker-v2` (Carina) |
| `docs/tech-design/wake-channel-v1.md` | the adjacent live wake: ledger-registered awaits, push-preferred delivery, the scheduled non-resident backstop tick (probe goal objects exactly, invoke the CLI per due touch, exit), staleness-bound > tick-interval relation, config-surface knobs, additive posture over the frozen argus pair |
| `docs/brainstorms/argus-unattended-drive-tech-design.md` | prior design rulings not to re-litigate: D-4 (that channel's shape), D-5 (additive, no supersession; awaits ride the journal), the rejected ALTs (push-only hosted sessions; resident host listener) |
| `internal/wake/` *(code read)* | the argus channel is partially implemented: `tick.go`, `push.go`, `probe.go`, signal + tests |
| `internal/executor/port/{codehost,worktracker}.go` | today's port surface the reader must extend: `Checks(one proposal)`, write-only `Comment`, no landing/comment reads (basis §2.4) |
| `docs/prds/feature-implementation-drive/effective.md` *(draft, working tree)* | the intended primary consumer — **unmerged**: the drive process the basis's F1 narrates has no contract yet; implement-now stays one-sweep (basis NR-1) |
| `docs/architecture-principles.md` + `docs/observability/incidents.md` + `docs/wiki/glossary.md` | via this session's loaded corpus: AP-2/6/11/13/15/19/22/25/26/27, I2/I4/I5, the ruled terms (wake signal, feature integration drive, self-drive avoid-entry) |
| `docs/research/harbormaster-self-drive-failure-taxonomy.md` | the predecessor evidence the mechanism choices must answer (silent clocks, swallowed evidence, claim-layer defects) |

**Overlaps surfaced:** `wake-channel-v1` is live and partially built — it owns the *argus-drive* wake (ledger awaits, goal-object probes, CLI-invoking tick) and its brainstorm already **rejected** resident-listener and hosted-session shapes for that consumer; it does not decide the batched task-state reads, the in-drive loop, the status verb, or budgets — but shares the await/staleness/signal concepts, so the relationship must be ruled (DQ-1) before anything else. `implement-now` (live) owns the executor ports the reader extends and is protected by basis NR-1. `plan-new-v1`/`local-work-tracker-v2` own projection and the Carina store — consumed, not re-decided. No live design serves prd:self-drive-wake — the ground is open.

## Design questions

- **DQ-1**: What is the design relationship between this capability and the live `wake-channel-v1` stack (design + `internal/wake`) — beneath it, superseding it, or building on its primitives — and which of the shared concepts (await, staleness predicate, signal grammar) is owned where?
- **DQ-2**: How many design documents carry FR-1..FR-13, and what partitions them?
- **DQ-3**: Where do the state reads live at the port layer — extending `CodeHost`/`WorkTracker`, or a new consumer-side reader port — and what does the conformance kit assert (FR-2's round-trip bound, FR-13's claim marking)?
- **DQ-4**: What hosts the v1 wake loop, given the feature drive (FID) is an unmerged draft and implement-now must stay one-sweep — and what exactly does a delivered wake invoke?
- **DQ-5**: The write throttle's mechanism — process-local vs state-dir-shared budget, its backpressure surface, and its home inside the GitHub adapter (FR-8, basis P-3).
- **DQ-6**: The Carina adapter's watch mechanism — a file-watch dependency vs a ~1 s poll (stdlib-first policy) — and how adapter-advised cadence is declared (FR-6).
- **DQ-7**: The GitHub adapter's read mechanism — the batched GraphQL shape, per-context check derivation, comment-surface mapping, cursor semantics (FR-1/FR-2/FR-3, basis P-1/P-2).
- **DQ-8**: Q1's thresholds — the too-long defaults per wait kind: values, and their home (constants vs closed-parse config data).
- **DQ-9**: Where EVT-1..EVT-6 land on the existing events surface, and what the proof run's report reads from them.

## Panel log

- **DQ-1**: opens (the relationship seam between two wake surfaces sharing concepts) — fired — three structurally distinct relations: substrate-beneath, generalize-and-supersede, build-on-primitives
- **DQ-3**: opens (a new read surface at the port layer) — fired — three placements: widen existing interfaces, one new reader port, standalone reader package
- **DQ-4**: opens (a new runtime surface — the loop's host verb/process) — fired — three hosts: a thin shipped watch verb, library-plus-proof-harness, the argus tick as host
- **DQ-2**: extends (the conventions' one-revisable-question rule; the options are cardinalities, not structural shapes) — skipped — weighed inline in D-4
- **DQ-8**: extends (the shared staleness predicate's parameters + the existing config surface) — skipped — the options are data-home choices, not structural shapes; weighed inline in D-5
- **DQ-5**: extends (the GitHub adapter's internal write path; the state-dir surface already exists) — skipped — FR-8's cross-process aggregate fit admits one non-daemon shape; the rejected variants are recorded inline in D-6
- **DQ-6**: extends (the Carina adapter's internals; a dependency-policy application, not a seam) — skipped — one stdlib shape meets the frozen ≤1 s bound; fsnotify weighed inline in D-7; the 10 s uniform-cadence variant surfaced a basis conflict and was withdrawn (D-7)
- **DQ-7**: extends (the GitHub adapter's internal read path behind the D-2 port) — skipped — mechanism inside an existing adapter; the REST-fallback variant weighed inline in D-8
- **DQ-9**: extends (the observability surface's physical layout; the sink class is settled ground) — skipped — data-home choice weighed inline in D-9; the committed-everything variant fails AP-28's own routine-ticks carve-out

## Decisions

### D-1 — Wake kinship: build on wake-channel-v1's primitives; one staleness brain, two bodies

- **Question**: DQ-1 — the relationship to the live argus wake stack: fully separate, generalize-and-supersede, or new designs building on its shared primitives?
- **Answer**: **ALT-3** — new sibling design(s) own the reader, loop, verb, and budgets; a `Builds-on: wake-channel-v1` edge scoped to the shared primitives (the wake-signal grammar and the staleness predicate), which stay owned there; `internal/wake`'s signal + staleness code becomes the single implementation both stacks import. The argus stack is untouched; its future re-plumb onto the batched reader is its own evolution, a non-goal here.
- **Reasoning**: Adopted the recommendation — one staleness brain by construction answers the predecessor's twin-predicate class (research §3.4) without re-approving a frozen design's ~12 anchors (ALT-2's cost) or accepting copy-drift (ALT-1's cost); the two consumer runtimes (no-process tick vs live loop) are genuinely different and stay separately designed. Partially resolves basis Q2 (the binding half; sequencing lands with DQ-2).
- **Anchor**: 2026-07-30 · wake-kinship question (AskUserQuestion #1)
- **Status**: live

### D-2 — Reads' home: a new consumer-side reader port, pure addition; check-state single-sourced by delegation

- **Question**: DQ-3 — where the state reads live: widen `CodeHost`/`WorkTracker`, a new reader port as a new file, or a standalone reader package with its own clients?
- **Answer**: **ALT-5** — one new interface (batch call → per proposal: medium-claimed landing + landed revision id, per-context check states, comments-since-cursor) in the executor's port package as a new file; both existing adapter packages implement it; existing interfaces untouched; `Checks(one)` internally delegates to the new read so per-context check derivation exists once; the conformance kit (round-trip bound, claim marking) homes on the new port.
- **Reasoning**: Adopted the recommendation — pure file-level addition (AP-27) keeps basis NR-1 trivially safe and avoids the tree-wide fake/adapter churn of widening shared interfaces (ALT-4); a standalone package (ALT-6) would duplicate medium plumbing and bypass the hexagon (AP-1/AP-11).
- **Anchor**: 2026-07-30 · reads-home question (AskUserQuestion #2)
- **Status**: live

### D-3 — Loop host: a thin shipped watch verb; the executor invoked, never modified; FID inherits or replaces

- **Question**: DQ-4 — what hosts the v1 loop with FID unmerged: a thin shipped watch verb, library-plus-proof-harness, or the argus tick?
- **Answer**: **ALT-7** — a new thin CLI verb (working name `pyxis watch <plan>`): derive the awaited set from the frozen plan + tracker/proposal state via the reader port, run the in-process cadence loop, invoke the executor's existing one-shot sweep entry per wake, and feed the status verb's data. No ownership, feature branches, or e2e semantics — that ground stays FID's; when FID merges it grows from or replaces the verb, by its own contract.
- **Reasoning**: Adopted the recommendation — the promise ships and the proof run exercises the real path (AP-14; the harness option repeats the I11/hermetic-fakes class); the argus-tick option reopens D-1 and ties the 15 s promise to a parked-problem clock. Completes basis Q2's sequencing half: watch now, FID later, argus untouched.
- **Anchor**: 2026-07-30 · loop-host question (AskUserQuestion #3)
- **Status**: live

### D-4 — Partition: two documents — state-reader-v1 (the medium boundary) and wake-watch-v1 (the consumer runtime)

- **Question**: DQ-2 — how many design documents carry FR-1..FR-13: one, two, or three?
- **Answer**: **Two** — `state-reader-v1` (the reader port, both adapters, the write throttle, the conformance kit; Serves ≈ FR-1..3/FR-8/FR-9/FR-13/NR-2/NFR-1/4/5) and `wake-watch-v1` (the watch verb, the in-process loop, the status verb, Q1 thresholds, events; Serves ≈ FR-4..7/FR-10..12/NR-1/NFR-2/3; Builds-on state-reader-v1 + wake-channel-v1 scoped to the primitives per D-1).
- **Reasoning**: Adopted the recommendation — the two known future revisions cut on exactly this line (a new medium adapter moves only the reader; FID's arrival moves only the watch host), so each document holds one independently-revisable question; one doc re-approves unmoved ground on every change; three splits a pair (loop + verb) that revises together. Mirrors the argus precedent's partition logic.
- **Anchor**: 2026-07-30 · doc-split question (AskUserQuestion #4)
- **Status**: live

### D-5 — Q1 resolved: thresholds as closed-parse config keys with shipped defaults

- **Question**: DQ-8 / basis Q1 — the too-long thresholds: fixed constants, config keys with defaults, or learned from history?
- **Answer**: **Config keys with defaults** — the four thresholds live as closed-parse keys on the repo config surface (unknown keys are errors), shipped with the recommended defaults: review-wait 2 h · CI-wait 30 m · worker/claim-wait 2 h · checker self-liveness 3× the cadence; tunable per repo without a code change. The learned-from-history version stays parked (basis WR-2).
- **Reasoning**: Operator's selection against the fixed-constants recommendation — per-repo tunability is worth four documented keys; no ruling conflicts (basis NG-6 refused a *cadence* knob specifically, and Q1 explicitly carried the config-value option). The keys ride the existing config surface's closed-parse discipline (AP-8/AP-6); the staleness *math* stays in the one shared predicate (D-1) — config supplies only the bounds.
- **Anchor**: 2026-07-30 · thresholds question (AskUserQuestion #5)
- **Status**: live

### D-6 — Throttle: a flock-guarded state-dir token bucket inside the GitHub adapter's write path

- **Question**: DQ-5 — the write throttle: shared state-dir bucket, per-process division, or a single-writer daemon?
- **Answer**: **Shared bucket** — one budget file under the pyxis state dir (per repo), flock-guarded; every content-write in the GitHub adapter acquires first; callers block context-aware (backpressure, never a platform 429); corrupt/missing file starts conservative (window assumed spent); Carina unthrottled.
- **Reasoning**: Adopted the recommendation — the only shape meeting FR-8's aggregate-across-processes fit without a resident coordinator (NG-1's ground); per-process division fails the fit whenever N is guessed wrong; matches basis §5.4's data rules verbatim.
- **Anchor**: 2026-07-30 · throttle question (AskUserQuestion #6)
- **Status**: live

### D-7 — Carina watch: stdlib 1 s mtime poll; the contract's ≤1 s bound stands

- **Question**: DQ-6 — Carina change detection: stdlib 1 s mtime poll vs an fsnotify dependency; then, after the authority's initial "stdlib 10 s" answer, the surfaced conflict: 10 s cannot meet the frozen basis's Carina bound (M2/NFR-2 p95 ≤ 1 s) — which stands?
- **Answer**: **The contract stands** — stdlib mtime scan at 1 s cadence while awaits exist (walk the store's dirs, compare mtime/size, wake on delta; idle → no scanning); no fsnotify dependency; GitHub keeps its own 10 s advice — two cadences, each adapter-declared per FR-6. The 10 s-everywhere variant was withdrawn once its cost (a typed amendment to a day-old frozen metric, for symmetry saving ~nothing) was on the table.
- **Reasoning**: The 1 s scan of a local store costs ~zero, meets the merged promise exactly, adds no supply-chain entry, and avoids fsnotify's platform edge cases (atomic renames, watch re-registration, event coalescing) whose standard mitigation is a backup poll anyway. The conflict was surfaced per the contradiction rule rather than designed around (DINV-8).
- **Anchor**: 2026-07-30 · carina-watch question (AskUserQuestion #7, resolved via the 1s-vs-10s follow-up)
- **Status**: live

### D-8 — GitHub read: one batched GraphQL implementation; per-context, claim-marked, unknown-on-failure; no REST fallback

- **Question**: DQ-7 — the GitHub adapter's read mechanics: the batched GraphQL query alone, or GraphQL with a per-PR REST fallback?
- **Answer**: **One implementation** — the session-verified query per check: 50 PRs × (merged flag + merge-commit oid (FR-13) · statusCheckRollup **contexts** compared against the required set client-side (FR-3) · the three enumerated comment surfaces since the caller's cursor). Kit pins ≤2 calls/50-PR batch (FR-2); any error/429 → affected PRs unknown-retry (FR-9); the next cadence check is the retry. No REST fallback.
- **Reasoning**: Adopted the recommendation — measured 1-point cost (basis §2.4); a REST fallback is a second semantics-identical read path that rots unexercised and runs untested during the first real outage (the I11 shape); FR-9 already defines error semantics, making a fallback redundant by contract.
- **Anchor**: 2026-07-30 · github-read question (AskUserQuestion #8)
- **Status**: live

### D-9 — Events: a separate local-only, gitignored wake-events file for debugging; the committed proof-run report stays

- **Question**: DQ-9 — where EVT-1..EVT-6 land: the executor's existing stream, a dedicated wake-events file, or git-committed records?
- **Answer**: **A separate wake-events file, local-only and gitignored** — EVT-1..EVT-5 append there as typed JSONL for local debugging; it is never committed. Two survivals from the frozen basis: EVT-6's initiator/scope properties still ride the executor's existing sweep-start event (§5.7's source cell; M1 reads it), and the proof-run report — M1/M2/M3 distilled from the streams — is still generated and committed as DOD-4/DOD-6 evidence (AP-28's teaching artifact; the raw ticks are not fuel).
- **Reasoning**: The authority's own shape (via Other). Read against basis §5.4: its "the repo's event stream (existing events surface)" cell is read as naming the event-sink *class* (typed, append-only, additive — all preserved), with physical file layout design-owned; if design review reads that cell stricter, the remedy is a one-row amendment, flagged openly here rather than silently assumed. Accepted cost: the proof-report generator merges two files (wake events + executor events) by timestamp.
- **Anchor**: 2026-07-30 · events-home question (AskUserQuestion #9)
- **Status**: live

## Alternatives

### ALT-1 — Substrate-beneath: new sibling designs; wake-channel-v1 untouched

- **Shape**: This feature ships as new design(s) owning the reader port, the in-drive loop, the verb, and budgets. `wake-channel-v1` keeps its own probes today; a future re-plumb of its probe layer onto the reader port is *its* evolution, recorded here only as a non-goal pointer. Shared concepts stay where they are (glossary + wake-channel-v1); this design cites, never redefines.
- **Trade-offs**: wins — zero coupling to the argus surface, ships independently, no re-approval of 12 served anchors; costs — two staleness computations and two probe implementations coexist in the tree until the re-plumb (divergence risk on the shared predicate).
- **Principles**: AP-25/AP-27 honored (additive sibling); AP-11 strained at the *staleness predicate* (two evaluators of "too old?" unless code is shared without a design relation).
- **Incidents**: hm-class twin-predicate loops (research §3.4) — the strain above; none in pyxis I1..I5 matched (checked registry 2026-07-30).
- **Verdict**: hybridized into ALT-3 — the independence is right, the unshared predicate is not.
- **From**: DQ-1 · panel

### ALT-2 — Generalize-and-supersede: one wake capability for both contracts

- **Shape**: A new `wake-v2` supersedes `wake-channel-v1`, serving both prd:argus-unattended-drive FR-13..17 and prd:self-drive-wake — one await model, one staleness brain, one delivery layer with two consumer bindings (CLI-invoking tick for parked argus problems; in-process loop for live drives).
- **Trade-offs**: wins — one stack, no concept duplication ever; costs — re-approves a frozen design's ~12 served anchors through supersession for a feature that doesn't need to move them; couples the argus behavior surface to this feature's schedule; forces one shape over two genuinely different runtime models (non-resident tick vs in-process loop) — the generalization is speculative.
- **Principles**: strains RW-5 (one decision per document — this bundles two); strains the conventions' churn rule the argus session already cited (its D-1); AP-11 honored maximally.
- **Incidents**: none matched (checked registry 2026-07-30); the argus record's D-5 shows the house preference for additive over supersession when no binding statement moves.
- **Verdict**: rejected — supersession without a falsified statement is ceremony; the two consumer shapes are real, not incidental.
- **From**: DQ-1 · panel

### ALT-3 — Build-on-primitives: new designs, `Builds-on wake-channel-v1` for the shared concepts, single-sourced code

- **Shape**: New sibling design(s) own everything the basis contracts; they declare `Builds-on: wake-channel-v1` scoped to the shared primitives — the wake-signal grammar (slug/id + reason, instruction-free) and the staleness predicate ("age vs bound, bound > cadence") — which stay owned by wake-channel-v1 and are consumed, not redefined; `internal/wake`'s signal/staleness code becomes the single implementation both stacks import (AP-11 at the code layer). The reader port, loop, verb, throttle are new ground; wake-channel-v1's future probe re-plumb onto the reader port is named as its evolution, out of scope here.
- **Trade-offs**: wins — one staleness brain and one signal grammar by construction; independence preserved (Builds-on narrows nothing that moves); the argus stack untouched. costs — a design-graph edge to maintain; the primitives' home is a design that never anticipated a second consumer (its prose stays argus-flavored until its own next revision).
- **Principles**: AP-11 (one predicate, one place — by import), AP-25/AP-27 (additive growth), AP-2/AP-6 (signal grammar reuse), RW-5 held (each doc one question).
- **Incidents**: research §3.4 (twin predicates) answered structurally; pyxis I1..I5 — none matched (checked registry 2026-07-30).
- **Verdict**: chosen (D-1)
- **From**: DQ-1 · panel

### ALT-4 — Widen the existing interfaces

- **Shape**: Add the batch state read to `CodeHost` and a comments read to `WorkTracker` — the reads live where the writes live today.
- **Trade-offs**: wins — no new port, consumers already hold the handles; costs — edits two shared interfaces every adapter and every test fake implements (a hot-surface edit across the tree), and splits one conceptual read (a proposal's full state) across two ports.
- **Principles**: strains AP-27 (extension by addition — this is modification of shared surfaces); AP-1 held.
- **Incidents**: none matched (checked registry 2026-07-30); research §3.5's claim-layer churn argues against widening load-bearing surfaces casually.
- **Verdict**: rejected — hot-surface edit across the tree; splits one read across two ports (D-2)
- **From**: DQ-3 · panel

### ALT-5 — One new consumer-side reader port, pure addition

- **Shape**: A new interface in the executor's port package, its own file — one batch call returning, per proposal: landing (medium-claimed + landed revision id), per-context checks, comments-since-cursor. Implemented by the existing GitHub/Carina adapter packages; existing interfaces untouched. The per-context check derivation becomes the single implementation; the existing `Checks(one)` delegates to it internally (adapter-internal change, no interface change).
- **Trade-offs**: wins — pure addition (new file), NR-1 trivially safe, one home for the conformance kit (round-trip bound, claim marking), AP-11 satisfied for check state by delegation; costs — a second read entry point beside `Checks` until the delegation lands.
- **Principles**: AP-27 (file-level addition), AP-11 (delegation single-sources per-context checks), AP-1/AP-3 (port + kit), AP-6 (typed batch contract).
- **Incidents**: I5's single-sourcing lesson honored via delegation; none else matched (checked registry 2026-07-30).
- **Verdict**: chosen (D-2)
- **From**: DQ-3 · panel

### ALT-6 — Standalone reader package with its own medium adapters

- **Shape**: `internal/reader` owns its own GitHub/Carina clients, independent of the executor's adapters and wiring.
- **Trade-offs**: wins — zero contact with executor surfaces; costs — duplicates auth/config/client plumbing per medium (two GitHub adapters in one tree), and the status verb/loop then bypass the port layer the engine's hexagon mandates.
- **Principles**: strains AP-1 (a second, parallel medium boundary) and AP-11 (duplicate client/config predicates).
- **Incidents**: none matched (checked registry 2026-07-30).
- **Verdict**: rejected — duplicate medium plumbing; bypasses the port layer (D-2)
- **From**: DQ-3 · panel

### ALT-7 — A thin shipped watch verb hosts the loop

- **Shape**: A new CLI verb (working name `pyxis watch <plan>`): derive the awaited set from the frozen plan + tracker/proposal state through the reader port, run the in-process cadence loop, and on each wake invoke the executor's existing public sweep entry (one-shot, unchanged); render/serve the same data `pyxis status` prints. The future feature drive (FID), when it merges, either grows from or replaces this verb — its contract's call.
- **Trade-offs**: wins — the promise ships (M1's zero re-invocations is real for users now); the proof run exercises the real integration, not a harness; smallest honest host. costs — a v1 surface FID must later reconcile with (naming + semantics); the verb must stay thin or it becomes a shadow FID.
- **Principles**: AP-19 (loop idles free behind await registration), AP-2 (wake schedules the same sweep), AP-27 (new verb, new file); NR-1 held (the sweep is invoked, never modified).
- **Incidents**: research §3.1's remedy shape (a self-sustaining loop owned by the process, not an external scheduler); none in I1..I5 matched (checked registry 2026-07-30).
- **Verdict**: chosen (D-3)
- **From**: DQ-4 · panel

### ALT-8 — Library only; the proof run uses a test harness; FID hosts later

- **Shape**: Ship the reader port, loop library, verb, and budgets — but no shipped loop host; DOD-4's proof run drives a harness binary in the test tree; the first real host arrives when FID merges.
- **Trade-offs**: wins — nothing to reconcile with FID later; costs — the shipped feature cannot deliver M1 for a user until FID lands (the contract's own ship rule reads "M1 = 0 … → ship", measured on a harness, not a shipped surface), and a harness-driven proof is exactly the hermetic-integration gap the house has paid for before.
- **Principles**: strains AP-14 (verify from the deployment context, not a bespoke harness) and the basis's G1 (the operator's day changes only after FID).
- **Incidents**: I11-class (green double over a never-shipped path) via the research record §3.7; the operator's own hermetic-fakes lesson.
- **Verdict**: rejected — proves the promise on an unshipped path; M1 stays fictional for users until FID (D-3)
- **From**: DQ-4 · panel

### ALT-9 — The argus backstop tick hosts the loop

- **Shape**: Extend wake-channel-v1's scheduled tick to also read plan/task awaits and invoke sweeps — one host for both wake stacks.
- **Trade-offs**: wins — no new verb; costs — inverts D-1 (the argus stack was to stay untouched), binds task-watching latency to a tick sized for parked problems (its interval = argus's latency bound, not this basis's 15 s promise), and puts a second consumer inside a design that just rejected growing (its F-2 rejected shapes).
- **Principles**: violates D-1's ruling scope; strains AP-19's per-consumer cost statement and the tick's stated ~0-idle identity.
- **Incidents**: none matched (checked registry 2026-07-30).
- **Verdict**: rejected — reopens D-1, wrong clock for the 15 s promise, grows a design that just closed (D-3)
- **From**: DQ-4 · panel

## Seam map

- **S-1**: the wake-primitives seam (`internal/wake` signal grammar + staleness predicate) — **extends** — behind it: one shared implementation imported by the argus tick and the new drive loop/status verb — feeds: the design's `Builds-on: wake-channel-v1` line, scoped to the primitives (D-1)
- **S-2**: the reader port (new interface, new file in the executor port package) — **opens** — behind it: both medium adapters' batched state reads + the conformance kit; `Checks(one)` delegates internally — feeds: the design's Scope (port + adapters + kit) and its Serves rows FR-1/FR-2/FR-3/FR-13 (D-2)
- **S-3**: the watch verb (new CLI surface + in-process loop) — **opens** — behind it: await derivation from plan+tracker, the cadence loop, sweep invocation, the status data source — feeds: the design's Scope (loop + verb) and Serves rows FR-4/FR-5/FR-6/FR-7/FR-12; FID's future host decision named as its evolution (D-3)
- **S-4**: the wake-events file (local-only, gitignored JSONL beside the existing stream) — **opens** — behind it: EVT-1..EVT-5's sink + rotation; the proof-report generator merges it with the executor stream — feeds: wake-watch-v1's Scope/observability section (D-9)

## Facts

- **F-1**: `wake-channel-v1` is live: registry-driven (awaits in the delivery ledger's journal), push-preferred per-adapter delivery, and a **scheduled, non-resident backstop tick** — read awaits, probe each goal object exactly, invoke the agentic CLI per due touch, exit; tick interval = the latency bound; staleness bound strictly > interval; knobs are closed-parse config keys. — source: `docs/tech-design/wake-channel-v1.md` §2/§3.2
- **F-2**: Its brainstorm already rejected, for the argus consumer, push-only hosted sessions and a resident host listener; and ruled the whole stack additive over the frozen argus pair (no supersession). — source: `docs/brainstorms/argus-unattended-drive-tech-design.md` D-4/D-5, ALT-8/ALT-10
- **F-3**: `internal/wake/` exists with `tick.go`, `push.go`, `probe.go`, signal + tests — the argus channel is at least partially implemented; extent unverified this session. — source: code listing 2026-07-30
- **F-4**: Today's executor ports cannot express the basis's reads: `CodeHost` = `OpenProposal` + `Checks(one)`, `WorkTracker.Comment` write-only; `implement-now` (live design) owns this surface and basis NR-1 freezes the executor's one-sweep semantics. — source: `internal/executor/port/*.go`; `docs/tech-design/index.md`
- **F-5**: `prd:feature-implementation-drive` is an unmerged draft — the "drive process" of the basis's F1 has no contract; something else must host the v1 loop for the proof run (DQ-4). — source: its effective.md header (Draft — problem review pending)
- **F-6**: `fleet-bridge-v1` and `maintenance-sweep-v1` both `Builds-on` wake-channel-v1 — its concepts already have three consumers; moving them is expensive. — source: `docs/tech-design/index.md`

## Fog

- **G-1** (non-blocking — blocks: wake-watch-v1's observability wording, not the direction): whether basis §5.4's "the repo's event stream (existing events surface)" cell admits a second physical file behind the same sink class (D-9's reading) — if design review reads it stricter, the remedy is a one-row `/prd-amend`; rides the authored design as its own Q-n.

## Out of scope

- **O-1**: Re-plumbing wake-channel-v1's probes onto the new reader port — that stack's own future evolution, never this design's act. (from D-1)
- **O-2**: Feature-lifecycle semantics — ownership, feature branches, candidate-merge e2e — the feature drive's future ground; the watch verb stays thin. (from D-3)
- **O-3**: Modifying the executor's one-sweep semantics — the watch verb invokes the existing entry, never changes it (basis NR-1). (from D-3)
- **O-4**: A REST fallback read path beside the GraphQL read — one implementation only. (from D-8)
- **O-5**: Any file-watch dependency (fsnotify or kin) — the stdlib mtime scan meets the bound. (from D-7)

## Candidate glossary terms

none

## Parked product gaps

none

## Gate

| Field | Value |
|---|---|
| **Verdict** | ready-for-design |
| **Date** | 2026-07-30 |
| **Authority** | @mdarabi |
| **Covering design** | — |
| **Coverage check** | pass — core four covered (§2 Decision: D-1..D-4 panel-logged · §4 Alternatives: ALT-1..9 all verdicted · Serves/Scope: S-1..S-4 + D-4's FR mapping · §3.2 Boundaries: O-1..O-5), zero blocking fog (G-1 non-blocking) |
| **Amend route** | — |
