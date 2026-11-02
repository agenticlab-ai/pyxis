# State reader v3 — batched proposal-state reads, the write budget, and the conformance kit — technical design

| Field | Value |
|---|---|
| **Status** | Approved *(operator approval via the introducing PR's review — the merge is the freezing event)* |
| **Owner** | @mdarabi |
| **Approver(s)** | @mdarabi (operator) — a platform-contract surface (every future medium adapter implements this port) |
| **Date** | 2026-08-03 |
| **Relevant-when** | designing or changing how the factory reads proposal state (landed/checks/comments) on any medium, the batched GraphQL read, the Carina store scan, the write-rate budget, or the reader conformance kit; implementing a new medium adapter against the reader port; triaging a wrong or empty state read, a rate-limit trip, a badge-trusted-as-landed defect, or a write that stalls with no worker running |
| **Scope** | The reader port (one batched state read), its GitHub and Carina adapters, the shared write budget on metered media — its home, its key, its recovery rules, and the deferral event that makes it observable — and the conformance kit that binds adapters. **Not**: who calls the reader or when (wake-watch-v2's loop and cadence), the status verb's rendering, wait-age thresholds, wake signals, or any executor-behavior change. |
| **Tracking** | — *(the introducing PR)* |
| **Builds-on** | docs/tech-design/implement-now-routed-execution.md (the executor port package and medium adapters this extends additively) · docs/tech-design/local-work-tracker-v2.md (the Carina store the local adapter reads) |
| **Supersedes** | state-reader-v2 |
| **Serves** | prd:self-drive-wake/FR-1 · prd:self-drive-wake/FR-2 · prd:self-drive-wake/FR-3 · prd:self-drive-wake/FR-8 · prd:self-drive-wake/FR-9 · prd:self-drive-wake/FR-13 · prd:self-drive-wake/UB-1 · prd:self-drive-wake/UB-2 · prd:self-drive-wake/UB-6 · prd:self-drive-wake/EVT-4 · prd:self-drive-wake/NR-2 · prd:self-drive-wake/NFR-1 · prd:self-drive-wake/NFR-5 |
| **Security/privacy impact** | none — read-only access over the media's existing authenticated clients; no new credential, listener, egress destination, or data class; the write budget shapes traffic that already flows, and its bucket holds timestamps only (§3.5) |
| **Template version** | tech-design-template/v1 |

## 0. Read to rule — the human reviewer's section (~4 min)

*The body is the contract; this section only routes attention — on any conflict the body governs; machines ignore it.*

**Judge these**

| # | The call | Could have gone | If you disagree |
|---|---|---|---|
| 1 | **A new reader interface as a new file; existing ports untouched; `Checks` delegates** | widening `CodeHost`/`WorkTracker` (design-brainstorm:self-drive-wake-tech-design#ALT-4) | tree-wide adapter/fake churn returns, and basis NR-1's safety argument weakens → §2, §3.1 |
| 2 | **Landing is the medium's claim, carried with the landed revision id — the reader never asserts ancestry** | trusting the platform's merged flag as goal-state (the incident-I2 class) | consumers lose the field goal-object verification needs, and a badge can advance a DAG → #DINV-3, §3.1 |
| 3 | **One GraphQL read; no REST fallback** | a dual-path read (design-brainstorm:self-drive-wake-tech-design#D-8's rejected variant) | a second semantics-identical path rots unexercised and runs untested in the first outage → §3.6 |
| 4 | **The required-check set reads from branch protection inside the same query; absent protection ⇒ every observed check must pass** | a config-declared required list beside the medium's own | two authorities for "required" drift apart (AP-11); the fallback's strictness is the safe direction → §3.6 |
| 5 | **The write budget is one flock-guarded bucket in the machine state dir, keyed to (medium host, authenticated identity)** | a per-checkout bucket (as v2 was built), or a per-repository one | a checkout's lifetime is unrelated to the metered window, and a finer key than the platform documents lets N repositories each spend a full window against one identity's quota → §3.8, #DINV-6 |
| 6 | **Recovery splits by what the state can hide: an absent bucket starts empty; a present-but-unreadable one starts spent** | fail closed on both (v2's rule, as built) | an absent bucket has no prior holder and can hide nothing, so charging for its absence costs a full idle window and buys no safety — and makes "safe to delete" false → §3.8, #DINV-6, #DINV-7, incident:I7 |
| 7 | **Unknown is a first-class read result, never an omission or a default** | erroring out, or presenting failures as empty results (the swallowed-evidence class) | a 429 mid-verification reads as "not landed" and drives wrong recovery → #DINV-1, #DINV-2 |

**Sanity-check the numbers** — one 50-proposal batch in ≤ 2 API round trips on GitHub (A-2; measured cost 1 point, basis §2.4); steady-state reads ≤ 10% of the medium's read budget at the proof cadence (A-9); burst drill: nine tasks' write sets concurrently, zero platform rate-limit responses, zero drops (A-6); a cold start grants its first write with no delay (A-6); Carina scan cost ~0 (local stat of dozens of files, §3.7).

**The author's open questions** — Q1: the Carina record-field mapping is fixed at implementation against local-work-tracker-v2 fixtures → §7.

**Skip the rest** — header/schema/structure → docgate lint; substance, reuse, and shape → the unmanned `tech-design-review`; implementation defects → codegate + the conformance kit; rationale → §4 (explanatory).

## 1. Problem

The factory can write to its media — open proposals, post comments — but cannot see them: no port answers "which of these proposals landed? which got comments? which failed which required check?" (basis §2.1; the port gap is `internal/executor/port/` read verbatim in design-brainstorm:self-drive-wake-tech-design#F-4). Every consumer that needs the answer today either cannot exist (the wake loop, the status verb) or would invent its own medium client (the drift path). Two constraints make *this* the cheapest moment: the read shape must be settled with both media in view before any consumer bends it around one (the basis's D-3 lesson — today's gap came from exactly that), and the read must be budget-safe by construction because it will run every few seconds for hours (basis NFR-1, FR-8).

Budget-safety has a second half this design's predecessor got wrong, and the correction is why v3 exists. A throttle that blocks writes is a *safety* mechanism only where the state it reads carries information. v2 contracted one recovery rule — "corrupt, unreadable, or missing ⇒ start conservative" — for three states, and the implementation followed it faithfully into a self-inflicted outage: every ephemeral driver checkout starts without a bucket, so every ephemeral driver charged itself a full hour-window before its first write, for spend that never happened (incident:I7). The bucket also lived inside the driver's own checkout — a location whose lifetime is unrelated to the window the platform meters — so it was never the machine-shared thing its own contract described.

## 2. Decision

**Decision:** One new **reader port** — a single batched state read — added as a new file beside the existing executor ports, implemented by the existing GitHub and Carina adapter packages, and bound by a conformance kit. Per proposal the read returns: **landing state as the medium's claim** (open · landed · closed-unlanded · unknown) carrying the medium-reported landed revision id; **per-context check states** compared against the medium's required set, never an aggregate rollup alone; and **comments newer than a caller-held cursor** across the medium's proposal-discussion surfaces. All read failures resolve to a typed **unknown** — never an error masquerading as absence, never a default. On metered media, every content-*write* passes a shared, flock-guarded budget bucket that blocks callers (context-aware) instead of ever letting a platform rate-limit response occur; that bucket lives in the **machine's** state directory, keyed to the entity the platform meters, and its recovery rules distinguish state that is *absent* from state that is *unreadable*. Existing port interfaces are untouched; the existing per-proposal `Checks` read delegates to the new implementation internally. (design-brainstorm:self-drive-wake-tech-design#D-2, #D-6, #D-8; ALT-5)

The reasoning, stated once: the port is a platform contract (AP-3) whose granularity must be set by the *metered* medium — per-item reads are free on Carina and an unfixable N+1 on GitHub (basis §2.4, measured), so the interface is batch-shaped with a declared round-trip bound the kit asserts. Landing is deliberately a *claim*: a medium's "merged" certifies the medium's object, never the mainline (AP-26; incident I2), so the reader carries what goal-object verification needs and asserts nothing it cannot know. And the budget's fail-closed posture is scoped to where failure hides something (AP-17): unreadable state may encode spend the reader cannot see and must be paid for; absent state had no prior holder, hides nothing, and must cost nothing.

*(v3 supersedes state-reader-v2 to carry prd:self-drive-wake amendment 001 into the design layer — the absent/unreadable recovery split, the bucket's move to the machine state dir under a platform-safe key, and the deferral event that makes a throttled wait observable. Every other binding statement carries forward verbatim; the whole-document form is the only legal one for a merged design (design:tech-design-conventions §2).)*

*(The one live dependent, wake-watch-v2, is **not** superseded alongside. The conventions' trigger for a paired supersession is contradiction — a successor must supersede a live design whose **binding** statement it falsifies — and none is falsified here: no `**Decision:**` callout, `DINV-n`, or `A-n` row of wake-watch-v2 rests on the budget's recovery rule or the bucket's home, and the reader port and event hook it does depend on carry forward verbatim. Its §3.4 mention of the hook is a descriptive citation, not a binding statement. What the regenerated index therefore shows on wake-watch-v2 — the `stale-builds-on` flag — is the convention's intended surfaced review prompt, recorded here so it reads as a deliberate verdict rather than an unnoticed break. This differs from the v1→v2 moment, where the pair moved together because implement-now's supersession had falsified a header row on **both** documents.)*

## 3. The design

### 3.1 The reader port

**Decision:** One interface, one method, in a new file in the executor's port package — consumer-side, per the hexagon (ports are declared by what the engine needs).

```go
// port/statereader.go (new file; nothing existing edited)
type ProposalStateReader interface {
    // States returns one ProposalState per requested ref — always all of
    // them, in order; a proposal the medium cannot answer for is returned
    // as StateUnknown with a typed reason, never omitted (DINV-1).
    States(ctx context.Context, refs []ProposalRef, since Cursor) ([]ProposalState, error)
}
```

- **`ProposalRef`** — the medium-neutral proposal identity (number + repo scope), the same identity `CodeHost.OpenProposal` returns today.
- **`ProposalState`** — closed struct: `Landing` (enum `open · landed · closed-unlanded · unknown` + `LandedRevision string` — the medium's claimed landed commit/record id, empty only when not landed) · `Checks []CheckContext` (`{Name, State}` per context, plus `RequiredMissing []string` — required contexts with no run observed) · `Comments []Comment` (author, created-at, surface kind, body ref) · `Unknown *UnknownReason` (typed enum: `rate-limited · offline · parse-defect · timeout`).
- **`Cursor`** — caller-held opaque per-surface positions (the reader is stateless, DINV-7); `since` filters comments only.
- **Comment surfaces** (the basis FR-1's enumerated classes, mapped per medium in §3.6/§3.7): discussion comments, inline review comments, review summaries.
- **The top-level `error`** is transport-fatal only: context cancelled, or a failure of the budget bucket's **substrate** rather than its contents — the state directory uncreatable, the lock unopenable, the replacement unwritable. Unreadable bucket *contents* are never fatal; §3.8 recovers them by starting conservative. Partial medium failures are per-proposal `Unknown` — the error return never smuggles state (DINV-2).
- **Delegation:** the existing `CodeHost.Checks(one)` implementation becomes a call through the same per-context derivation this port defines (adapter-internal; no interface change) — one implementation of "is this green?" (AP-11).

### 3.2 Boundaries and non-goals

- **Not solved here** — when and how often the reader is called (wake-watch-v2's cadence loop); what a consumer does with a landed claim (its own contract's goal-object verification); rendering (the status verb).
- **Owned by a future design** — wake-channel-v1's re-plumb of its per-goal probes onto this port (its own evolution — design-brainstorm:self-drive-wake-tech-design#O-1); further medium adapters (GitLab et al.) by following the kit (AP-3); any mechanism that *learns* a platform's real limit scope rather than assuming the coarser one (§3.8's key is fixed by what the platform documents, not by probing).
- **Implementation must not infer** — that `landed` implies mainline ancestry (it is the medium's claim — DINV-3); that an aggregate rollup may substitute for contexts (DINV-4); that a failed read may default to any definite state (DINV-2); that the reader may cache or persist medium content (DINV-7); that a missing budget bucket is evidence of anything (DINV-6).
- **Must remain unchanged** — every existing port interface and its adapters' external behavior (basis NR-2: existing write paths keep working under the budget; NR-1's executor semantics are untouched by construction — this design adds a port, changes no executor code path).

### 3.3 Invariants

- **DINV-1** — A batch read answers **every** requested ref exactly once; a proposal the medium cannot answer for is a typed `unknown`, never an omission, an empty struct, or a healthy-looking zero value (the swallowed-evidence class, research §3.2).
- **DINV-2** — No consumer-visible state transition derives from a failed read: `rate-limited/offline/parse-defect/timeout` all resolve to `unknown — retry later`; the top-level error carries no per-proposal state.
- **DINV-3** — Landing state is the **medium's claim**: every `landed` carries the medium-reported landed revision id, and the reader asserts nothing about mainline ancestry — goal-object verification is the consumer's act (AP-26; basis FR-13).
- **DINV-4** — Check state derives from the per-context set compared against the required set; an aggregate rollup alone never yields green, and a required context with no observed run reports as `RequiredMissing` ⇒ not-green (basis FR-3).
- **DINV-5** — On a metered medium, one batch completes within the adapter's declared round-trip bound, asserted by the conformance kit (basis FR-2).
- **DINV-6** — Every content-write on a metered medium acquires from the shared budget before sending; saturation blocks the caller context-aware — a platform rate-limit response reaching a caller is a defect, not weather. Recovery is decided by what the bucket's state can hide: **absent ⇒ start empty** (no prior holder existed, so no unobserved spend exists to protect against, and nothing is written to disk before the grant); **present but unreadable ⇒ start conservative** (both windows assumed spent, refilling on the clock — the bytes may encode spend the reader cannot see). The bucket is never partitioned finer than the platform documents its limit; an undocumented scope is assumed coarser, never narrower (prd:self-drive-wake §5.4; incident:I7).
- **DINV-7** — The reader is stateless: cursors are caller-held, no medium content is persisted, and the budget bucket is the only state — ephemeral and **genuinely** safe to delete: its absence costs no writes and no delay.
- **DINV-8** — Every deferral is observable at the moment it is decided: the throttle event carries the **projected** wait and the window's reset deadline, never elapsed time alone. A wait the operator cannot distinguish from a hang is a defect, not quiet (AP-5; incident:I7).

### 3.4 Observability

- **Events/logs** — the reader and budget accept an injected event hook (`func(Event)`, nil = silent): read completions (medium, batch size, cost points, duration, unknown count), unknown resolutions (typed reason), and **budget deferrals**. A deferral emits `throttle_deferred` with prd:self-drive-wake/EVT-4's typed properties — queue depth and the deferred duration — where the duration is the **projected** wait the bucket just computed, accompanied by the window's reset deadline (DINV-8). The event is emitted at each deferral decision, so a long wait re-announces itself on every re-evaluation rather than falling silent behind one timer. Consumers wire the hook to their sink — wake-watch-v2 wires it to the local wake-events file (its D-9 surface); sweeps may wire it to their run report. The reader itself owns no file.
- **EVT-4 is served here, and was served nowhere before.** No live design carried `prd:self-drive-wake/EVT-4` in its `Serves`, which is the mechanical reason the event was specified and never emitted: nothing bound anyone to emit it. This document takes it. Two consequences for the implementer, both verifiable rather than inferred: the sink's kind for this event is the requirement's name, `throttle_deferred` — a shipped constant spelling it otherwise is drift from the contract, not a competing authority (wake-watch-v2 §3.4's hyphenated list is descriptive prose, and the requirement's other four kinds already match the shipped constants verbatim); and the sink's closed event schema must gain EVT-4's typed properties as fields, since a closed schema (AP-6) cannot carry them any other way.
- **Metrics** — cost points and durations ride the events; the proof-run report (wake-watch-v2's committed artifact) distills them against basis NFR-1's ≤ 10% budget bound.
- **Alerts** — none of its own; staleness/liveness alarms are the consumer's (wake-watch-v2), fed by these events.
- **Debugging surface** — the status verb renders per-proposal `unknown` reasons verbatim; a `--json` style dump of one raw batch is an implementation affordance of the verb, not this port. A deferral's event is the operator's evidence that a stalled write is throttling and not a hang — the distinction §1 records as having cost an hour.
- **Trace/correlation** — events carry the batch id; the executor's existing trace fields apply where the caller supplies them (AP-5).

### 3.5 Security & privacy

none — matches the header row: reads ride the media's existing authenticated clients (no new credential or scope); nothing listens; no content leaves to any new destination; the budget only delays traffic that already flows. The injection posture is unaffected: the reader returns data; it carries no instructions and executes nothing it reads.

The bucket's move to the machine state directory adds no data class: its contents are write timestamps only — no repository name, proposal reference, or medium content — and its key derives from the medium host plus the already-locally-known authenticated identity (§3.8). It is machine-local and never transmitted. The one property the move does put at stake is write access, and §3.8 binds it rather than inheriting it: directory and file are owner-only, because the conservative-start rule turns a writable bucket into a local denial of writes — a hostile or merely careless local principal who can corrupt the file parks every pyxis content write for a full window. That is the whole of this design's security surface, and it is closed by construction rather than by convention.

### 3.6 The GitHub adapter

**Decision:** One GraphQL query per batch — no REST fallback (design-brainstorm:self-drive-wake-tech-design#D-8) — shaped as the session-measured read (basis §2.4, cost 1 point at 50 proposals):

- `pullRequests` by numbers: `number · state · merged · mergeCommit { oid }` (the claim + revision id, DINV-3),
- `commits(last:1) → statusCheckRollup → contexts(first:50)`: `CheckRun{name, conclusion}` + `StatusContext{context, state}` — per-context, never the rollup's own `state` (DINV-4),
- the three comment surfaces: issue comments, review threads' comments, review summaries — each with `createdAt` for cursor filtering,
- `baseRef → branchProtectionRule → requiredStatusCheckContexts` — the **required set**, read in the same query.

**Decision (required-set fallback):** where no branch protection declares required contexts, every *observed* context must conclude successfully for green — strict in the safe direction; a repo that wants looser green declares protection (the medium's own authority — AP-24: the governing input is the platform's configuration, not a parallel config of ours).

Errors: any GraphQL `errors` entry or transport failure marks **all affected proposals** unknown with the typed reason — the response is checked for `errors` *before* data extraction (a 200-with-errors carrying `null` fields is the exact shape that swallowed evidence in the predecessor, research §3.2). Rate limits: honor `retry-after` where present; the *next scheduled call* is the retry (no in-adapter retry loops — FR-9's semantics make the cadence the retry).

Pagination: `refs` beyond the page size split into successive calls — the kit's bound is per-batch-of-50 (≤ 2 calls), scaling linearly by page beyond it (basis FR-2's "constant within the medium's page size").

### 3.7 The Carina adapter

**Decision:** A stdlib mtime/size scan of the local store — no watch dependency (design-brainstorm:self-drive-wake-tech-design#D-7) — reading the same record files local-work-tracker-v2 owns: landing from the record's state fields (the store's accepted/closed markers are the medium's claim; the landed revision id is the record's recorded commit), check state from the store's recorded verdict fields, comments from record notes. The exact field mapping is fixed at implementation against local-work-tracker-v2's fixtures (Q1) — the kit's fixture corpus is the binding artifact, so a store-schema drift fails a test, not a user. Scan cost is ~0 (dozens of local files); the adapter's declared advice (consumed by wake-watch-v2) is the 1 s cadence the basis promises. No budget applies (no platform limits — basis FR-8 scopes the throttle to metered media).

### 3.8 The write budget

**Decision:** One token-bucket ledger per **(medium host, authenticated identity)**, homed in the **machine's** pyxis state directory, flock-guarded, shared by every pyxis process on the machine (design-brainstorm:self-drive-wake-tech-design#D-6; prd:self-drive-wake §5.4):

- **The home** — the machine's per-user state directory, never a repository checkout. The root is the convention this repository already ships for machine-level state (`${XDG_STATE_HOME:-$HOME/.local/state}/pyxis/…`, as `cmd/pyxis` homes its journal); the bucket takes a path beneath it rather than introducing a second state-dir authority (AP-11). A checkout's lifetime is unrelated to the window the platform meters: an ephemeral driver checkout is created and destroyed well inside a single hour-window, so a bucket kept there is not machine-shared however its documentation reads — it is per-workspace state wearing a machine-shared name. A bucket left inside a checkout by an earlier build is **not** read, migrated, or counted: it is superseded by the machine-level bucket and safe to delete at any time.
- **The key** — one bucket per (medium host, authenticated identity), covering every repository that identity writes to on that host. The platform publishes the content-write caps but not the entity they are scoped to, and reserves the right to change them without notice; the bucket is therefore never partitioned finer than the platform documents its limit, and an undocumented scope is assumed coarser rather than narrower. A per-repository key would let N repositories each spend a full window against what may be one identity's quota; the coarser key can only ever *over*-throttle — which UB-6 permits and a platform rate-limit trip (M3) does not. **When the identity cannot be resolved** — an unauthenticated path, a credential the medium will not resolve to a login, a transient lookup failure — the bucket keys by **medium host alone**, the coarser direction this rule already mandates. A process, checkout, or repository substitute is never admissible as a key component: reaching for one reinstates exactly the partition this design removes.
- **Ownership** — the bucket's directory and file are owner-only. This is load-bearing rather than hygienic: because unreadable contents start conservative, any principal able to write the path can park every pyxis content write for a full window by corrupting it, so write access to the bucket is exactly the set of processes entitled to spend the budget (§3.5).
- **Windows** — a per-minute and a per-hour window over content-writing requests; the limits are named constants in the adapter sourced from the platform's published secondary caps (the constants file cites the platform doc — the live authority; this document deliberately states no cap numbers, which the platform can change).
- **Acquire** — `Acquire(ctx, n)` before every content-write in the GitHub adapter; blocks until budget or ctx-cancel; never returns "denied" (backpressure, not errors — basis FR-8). Every duration the bucket computes — including the projected wait to the next refill — is derived from the **same injected clock** that stamps its entries; a wait read from the process clock while entries are stamped from an injected one cannot be tested at window scale and is a defect (§3.9).
- **Recovery** — decided by what the state can hide (DINV-6). **Absent ⇒ start empty**: no prior holder existed, so the windows are genuinely unspent, the first write is granted immediately, and **nothing is persisted before the grant** — fabricating a full ledger and writing it out extends one process's mistaken start to every process that later shares the bucket. **Present but unreadable ⇒ start conservative**: unparseable bytes, or a structurally invalid ledger (a missing window, a zero timestamp), may encode spend the reader cannot see, so both windows are assumed spent and refill on the clock. DINV-6's absent rule and DINV-7's "safe to delete" are one claim stated at two altitudes — the bucket is disposable *because* its absence is free — so a change to either that breaks the other is a defect in this document, not a local edit.
- **Batching stays the cheapest budget** — the review write (comments[] + verdict in one call) remains the adapters' shape for multi-comment writes (basis §2.4's evidence row; enforced by NR-2's suites which run with the budget active).
- **Deferral is announced, not silent** — see §3.4/DINV-8: the projected wait and reset deadline ride the event, at each deferral decision.
- Carina: no budget (§3.7).

### 3.9 Testing strategy

Seams binned per the template's menu: the **Carina adapter** is local-substitutable (a real temp-dir store from local-work-tracker-v2 fixtures — test directly); the **GitHub adapter** is remote-but-owned (the port's production adapter + recorded-response fixtures for every DINV case: rollup-green-context-missing, 200-with-errors, rate-limited, pagination; plus one live smoke inside the proof run — AP-14's deployment-context proof); the **kit** runs the same corpus against both adapters — a new adapter passes the kit or it does not exist (AP-3; basis NFR-5).

The **budget** is in-process with an injected clock **that every one of its computations reads** — entries, pruning, and the projected wait alike — so an hour-scale window is exercised in microseconds and a wait is asserted as a value rather than waited out. Four cases bind §3.8:

- **cold start** — no bucket on disk: the first write is granted with no delay, and no ledger is written before the grant (the defect incident:I7 records, in both its halves);
- **unreadable bucket** — corrupt bytes and a structurally invalid ledger each start conservative;
- **cross-checkout sharing** — two distinct repository checkouts under one identity draw from one bucket, and the second sees the first's spend;
- **burst drill** — nine tasks' write sets concurrently: zero platform rate-limit responses, zero drops, queue drains, and every deferral's event carries a projected wait and reset deadline (DINV-8).

## 4. Alternatives considered

| Alternative | Why rejected |
|---|---|
| Widen `CodeHost`/`WorkTracker` with the new reads (design-brainstorm:self-drive-wake-tech-design#ALT-4) | edits two shared interfaces every adapter and fake implements — tree-wide churn for zero capability gain; splits one logical read across two ports (D-2) |
| A standalone reader package with its own medium clients (#ALT-6) | duplicates auth/config/client plumbing per medium; two GitHub clients drift; bypasses the port layer (AP-1/AP-11) |
| GraphQL + REST fallback (#D-8's rejected variant) | a second semantics-identical implementation kept forever, exercised ~never, running untested in the first real outage (the I11 shape); FR-9 already defines error semantics — the cadence is the retry |
| A config-declared required-check list | a second authority for "required" beside the medium's branch protection — the AP-11 twin-predicate seam; the medium's own declaration governs (§3.6) |
| Per-process budget division / a single-writer daemon | division fails the aggregate fit whenever process count is guessed wrong; the daemon is a resident coordinator (basis NG-1's deferred ground) with a machine-wide single point of failure (D-6) |
| **Fail closed on absent *and* unreadable bucket state** (v2's rule, as built) | conflates two states carrying different information. An absent bucket had no prior holder and can hide nothing, so charging a full window for its absence buys no safety and costs an idle hour on every fresh checkout — and it makes DINV-7's "safe to delete" claim false on its face. incident:I7 |
| **Drop the conservative start altogether** | an unreadable bucket may encode spend the reader cannot see; granting a full window against it is exactly the rate-limit trip M3 forbids. Fail-closed is right where failure hides something (AP-17) |
| **A per-repository, or per-(identity, repository), bucket key** | partitions finer than the platform documents its limit: N repositories could each spend a full window against what may be one identity's quota. The platform publishes the caps but not their scope, and may change them without notice — so the safe direction is the coarser key |
| **A per-checkout bucket** (v2's home, as built) | a checkout's lifetime is unrelated to the metered window; ephemeral driver checkouts are born and destroyed inside one hour-window, so the bucket is never shared and every driver re-pays from zero |
| **Migrating an existing in-checkout bucket into the machine bucket** | it would import spend recorded under a different key against a different (possibly stale) window, for state the contract already declares safe to delete; ignoring it is both simpler and strictly safer |

## 5. Risks

| ID | Risk | Impact | Mitigation | Verification / evidence |
|---|---|---|---|---|
| R1 | Platform API drift makes a read silently wrong or empty | the drive "sees" nothing to do; false quiet | parse-or-defect (DINV-1/DINV-2): unparseable ⇒ typed unknown, never empty-healthy; `errors` checked before data | recorded-fixture suite incl. 200-with-errors; live smoke in the proof run (A-5, A-8) |
| R2 | The budget file becomes a contention point under many processes — now machine-wide, so processes in *different* repositories contend too | writes crawl | flock held only for the read-update-write of a tiny ledger (µs); windows amortize; measured in the burst drill, which the cross-checkout case extends across repositories | A-6's drill timings |
| R3 | The required-set fallback (all-observed-must-pass) is stricter than a repo intends | green withheld on optional checks | strict-in-the-safe-direction is deliberate (§3.6); the repo's remedy is declaring protection — the medium's own knob | A-3 fixture pair (protected/unprotected) |
| R4 | The Carina field mapping drifts from local-work-tracker-v2's schema | local reads silently wrong | the kit's fixture corpus is generated from lwt-v2 fixtures (Q1) — schema drift fails the kit | A-1/A-8 kit runs |
| R5 | Delegating `Checks(one)` through the new derivation changes a subtle existing behavior | executor drive-to-green misjudges | NR-2's suites run against the delegated path before merge | A-7 |
| R6 | The identity-wide key over-throttles when the platform's real limit scope is finer | concurrent drives across repositories share one window; writes slow under load that the platform would have allowed | deliberate and bounded — over-throttling is the backpressure UB-6 contracts for, while a rate-limit trip is what M3 forbids; the windows drain on the clock, and DINV-8's event makes the delay visible rather than silent, so the cost shows up as evidence if it ever bites | A-6's cross-checkout case + A-10's deferral events |
| R7 | The absent ⇒ empty rule is abused as a reset — a caller deletes the bucket to skip the queue | the machine spends past the platform's window and trips a rate limit | the rule is safe only because absence follows *no prior holder*; deletion by a live writer is out-of-contract operator action, not a code path. The bucket is machine-local state under an operator's own hand, and the platform's own limiter remains the outer bound | stated boundary (§3.8); no code path deletes the bucket |

## 6. Acceptance

| ID | Acceptance criterion | Verify | Required evidence | Traces to |
|---|---|---|---|---|
| A-1 | A mixed 50-proposal fixture on **both** adapters returns all three state kinds matching ground truth per enumerated surface; every unknown is typed; no ref omitted | Test | kit run (CI) | prd:self-drive-wake/FR-1 · #DINV-1 |
| A-2 | A 50-proposal batch on the GitHub adapter completes in ≤ 2 API round trips, kit-asserted | Test | kit round-trip assertion | prd:self-drive-wake/FR-2 · #DINV-5 |
| A-3 | Aggregate-green-with-required-context-missing reports not-green; the protected/unprotected fixture pair exercises the required-set fallback | Test | fixture results | prd:self-drive-wake/FR-3 · #DINV-4 |
| A-4 | Every landed report carries the medium's landed revision id; a consumer-side fixture performs an ancestry check from the reader's output alone | Test | consumer fixture | prd:self-drive-wake/FR-13 · #DINV-3 |
| A-5 | Injected 429/5xx/timeout/parse-defect fixtures yield typed unknown; zero consumer-visible transitions during the injection window; 200-with-`errors` never presents as empty-healthy | Test | failure-injection suite | prd:self-drive-wake/FR-9 · prd:self-drive-wake/UB-1 · prd:self-drive-wake/UB-2 · #DINV-2 |
| A-6 | Nine tasks' write sets replayed concurrently: zero platform rate-limit responses, zero dropped writes, queue drains. Plus the three recovery cases: an **absent** bucket grants the first write with zero delay and writes nothing to disk before granting; an **unreadable** bucket (corrupt bytes; structurally invalid ledger) starts conservative; two distinct repository checkouts under one identity draw from **one** bucket, the second observing the first's spend | Test | burst-drill report + the recovery cases | prd:self-drive-wake/FR-8 · prd:self-drive-wake/UB-6 · #DINV-6 · #DINV-7 |
| A-7 | Existing executor suites pass unchanged with the budget active and `Checks(one)` delegated | Test | existing suites (CI) | prd:self-drive-wake/NR-2 |
| A-8 | The conformance kit runs green on both shipped adapters and its corpus documents every asserted behavior; one live-API smoke passes inside the proof run | Test + Inspection | kit + smoke output | prd:self-drive-wake/NFR-5 · §3.9 |
| A-9 | Steady-state read consumption ≤ 10% of the medium's read budget at the proof run's cadence, computed from emitted events | Analysis | proof-run report extract | prd:self-drive-wake/NFR-1 |
| A-10 | Every deferral emits `throttle_deferred` carrying EVT-4's typed properties, where the deferred duration is the **projected** wait and the event names the window's reset deadline; asserted on a saturated-bucket fixture at both window scales, and no deferral reports a duration of zero | Test | event-stream extract from the burst drill | prd:self-drive-wake/EVT-4 · prd:self-drive-wake/UB-6 · #DINV-8 |
| A-11 | The budget's clock is injectable end-to-end: a test drives a full hour-window's saturation and drain without wall-clock waiting, and asserts the projected wait as a value | Test | budget suite runtime + asserted durations | §3.9 · #DINV-8 |

## 7. Open questions

| ID | Question | Owner | Blocks | Status | Resolution |
|---|---|---|---|---|---|
| Q1 | The exact Carina record-field mapping (landing/checks/comments ← lwt-v2 record schema) — fixed against lwt-v2 fixtures | @mdarabi | implementation (the kit corpus) | open | — |
| Q2 | Does the bucket's home eventually need a `paths`-config key of its own, or does the shipped per-user state-dir convention remain sufficient? | operator | nothing — the root is decided (§3.8); this asks only whether it later becomes configurable | open | the root is **not** open: §3.8 binds it to the convention `cmd/pyxis` already ships. Revisit only on a demonstrated need to relocate it |

## Principles

- **AP-1 — honored.** The read is a port; media vary behind it; the engine never learns which medium answered.
- **AP-3 — honored.** The conformance kit is the adapter-authoring contract; a new medium passes the kit or does not exist (A-8).
- **AP-5 — honored.** The budget's backpressure is observable by construction: a deferral announces its projected wait and reset deadline (DINV-8, A-10), so a throttled stall is distinguishable from a hang without reading code — the gap incident:I7 records.
- **AP-6 — honored.** Closed typed structs end-to-end; unknown is a typed value; no free-form state. The deferral event carries EVT-4's declared properties as typed fields, not an ad-hoc message.
- **AP-11 — honored.** One per-context check derivation (`Checks` delegates); the medium's branch protection is the single required-set authority (§3.6); one budget bucket per metered identity, not one per caller.
- **AP-15 — honored.** Decision-critical reads are exact and fresh — no conditional-read caching, no search indexes; the cadence, not a cache, provides cheapness.
- **AP-16 — honored.** Nothing loops or skips silently: a deferral counts and announces itself at *every* re-evaluation, carrying its projected wait and reset deadline (DINV-8, A-10). The failure this document repairs was the principle's exact inverse — one log line, then an hour of silence behind a single timer (incident:I7) — and a wait an operator cannot distinguish from a hang is the silent skip AP-16 forbids.
- **AP-17 — honored (sharpened).** Reads fail to *unknown*, and no destructive or dispatch action may derive from a failure (DINV-2) — positive evidence only. The budget applies the same rule where it belongs and only there: unreadable state may hide spend and fails closed; absent state can hide nothing and fails open. Conflating the two converted a safety rule into a self-inflicted outage (incident:I7), which is this principle's additive half being misread as its destructive one.
- **AP-19 — honored (enabling half).** The reader makes zero calls uninvoked; idle cost belongs to the caller's gate (wake-watch-v2).
- **AP-22 — honored.** Reads are idempotent; the budget's acquire is raceable across processes by construction (flock) — now across every repository under one identity, which is the sharing the contract always claimed.
- **AP-24 — honored.** The required-check set is read from the medium's own branch protection — the governing input stays the platform's human-set configuration; no parallel required-list of ours exists to drift from it (§3.6). The same deference sets the bucket key: the platform's documented limit scope governs, and where it documents none we assume coarser rather than invent finer.
- **AP-26 — honored.** Landing is the medium's claim, marked as such and carrying the revision id; ancestry is the consumer's observation (DINV-3).
- **AP-27 — honored.** The port is a new file; adapters extend internally; no shared surface is edited.
- **AP-28 — honored (scoped).** Raw read events are routine ticks (the rule's own carve-out); the teaching distillation is the committed proof-run report (wake-watch-v2's artifact).
- **RW-3 — honored.** No medium workaround crosses the port: GitHub's query mechanics and Carina's scan both die at the adapter boundary.
- **Deviation:** none.
