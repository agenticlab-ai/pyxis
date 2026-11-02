# State reader v2 — batched proposal-state reads, the write budget, and the conformance kit — technical design

| Field | Value |
|---|---|
| **Status** | Approved *(operator approval via the introducing PR's review — the merge is the freezing event)* |
| **Owner** | @mdarabi |
| **Approver(s)** | @mdarabi (operator) — a platform-contract surface (every future medium adapter implements this port) |
| **Date** | 2026-07-30 |
| **Relevant-when** | designing or changing how the factory reads proposal state (landed/checks/comments) on any medium, the batched GraphQL read, the Carina store scan, the write-rate budget, or the reader conformance kit; implementing a new medium adapter against the reader port; triaging a wrong or empty state read, a rate-limit trip, or a badge-trusted-as-landed defect |
| **Scope** | The reader port (one batched state read), its GitHub and Carina adapters, the shared write budget on metered media, and the conformance kit that binds adapters. **Not**: who calls the reader or when (wake-watch-v2's loop and cadence), the status verb's rendering, wait-age thresholds, wake signals, or any executor-behavior change. |
| **Tracking** | — *(the introducing PR)* |
| **Builds-on** | docs/tech-design/implement-now-routed-execution.md (the executor port package and medium adapters this extends additively) · docs/tech-design/local-work-tracker-v2.md (the Carina store the local adapter reads) |
| **Supersedes** | state-reader-v1 |
| **Serves** | prd:self-drive-wake/FR-1 · prd:self-drive-wake/FR-2 · prd:self-drive-wake/FR-3 · prd:self-drive-wake/FR-8 · prd:self-drive-wake/FR-9 · prd:self-drive-wake/FR-13 · prd:self-drive-wake/UB-1 · prd:self-drive-wake/UB-2 · prd:self-drive-wake/UB-6 · prd:self-drive-wake/NR-2 · prd:self-drive-wake/NFR-1 · prd:self-drive-wake/NFR-5 |
| **Security/privacy impact** | none — read-only access over the media's existing authenticated clients; no new credential, listener, egress destination, or data class; the write budget shapes traffic that already flows |
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
| 5 | **The write budget is one flock-guarded state-dir bucket; corrupt/missing starts conservative** | per-process division (fails the aggregate fit) or a writer daemon (basis NG-1) | N processes can collectively trip the platform caps mid-burst → §3.8, #DINV-6 |
| 6 | **Unknown is a first-class read result, never an omission or a default** | erroring out, or presenting failures as empty results (the swallowed-evidence class) | a 429 mid-verification reads as "not landed" and drives wrong recovery → #DINV-1, #DINV-2 |

**Sanity-check the numbers** — one 50-proposal batch in ≤ 2 API round trips on GitHub (A-2; measured cost 1 point, basis §2.4); steady-state reads ≤ 10% of the medium's read budget at the proof cadence (A-9); burst drill: nine tasks' write sets concurrently, zero platform rate-limit responses, zero drops (A-6); Carina scan cost ~0 (local stat of dozens of files, §3.7).

**The author's open questions** — Q1: the Carina record-field mapping is fixed at implementation against local-work-tracker-v2 fixtures → §7.

**Skip the rest** — header/schema/structure → docgate lint; substance, reuse, and shape → the unmanned `tech-design-review`; implementation defects → codegate + the conformance kit; rationale → §4 (explanatory).

## 1. Problem

The factory can write to its media — open proposals, post comments — but cannot see them: no port answers "which of these proposals landed? which got comments? which failed which required check?" (basis §2.1; the port gap is `internal/executor/port/` read verbatim in design-brainstorm:self-drive-wake-tech-design#F-4). Every consumer that needs the answer today either cannot exist (the wake loop, the status verb) or would invent its own medium client (the drift path). Two constraints make *this* the cheapest moment: the read shape must be settled with both media in view before any consumer bends it around one (the basis's D-3 lesson — today's gap came from exactly that), and the read must be budget-safe by construction because it will run every few seconds for hours (basis NFR-1, FR-8).

## 2. Decision

**Decision:** One new **reader port** — a single batched state read — added as a new file beside the existing executor ports, implemented by the existing GitHub and Carina adapter packages, and bound by a conformance kit. Per proposal the read returns: **landing state as the medium's claim** (open · landed · closed-unlanded · unknown) carrying the medium-reported landed revision id; **per-context check states** compared against the medium's required set, never an aggregate rollup alone; and **comments newer than a caller-held cursor** across the medium's proposal-discussion surfaces. All read failures resolve to a typed **unknown** — never an error masquerading as absence, never a default. On metered media, every content-*write* passes a shared, flock-guarded state-dir budget that blocks callers (context-aware) instead of ever letting a platform rate-limit response occur. Existing port interfaces are untouched; the existing per-proposal `Checks` read delegates to the new implementation internally. (design-brainstorm:self-drive-wake-tech-design#D-2, #D-6, #D-8; ALT-5)

The reasoning, stated once: the port is a platform contract (AP-3) whose granularity must be set by the *metered* medium — per-item reads are free on Carina and an unfixable N+1 on GitHub (basis §2.4, measured), so the interface is batch-shaped with a declared round-trip bound the kit asserts. Landing is deliberately a *claim*: a medium's "merged" certifies the medium's object, never the mainline (AP-26; incident I2), so the reader carries what goal-object verification needs and asserts nothing it cannot know.

*(v2 supersedes state-reader-v1 solely to repair the `Builds-on` lineage after implement-now's supersession by implement-now-routed-execution (#811); every binding statement carries forward verbatim — the collision that forced supersession over a row edit is incident I6.)*

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
- **The top-level `error`** is transport-fatal only (context cancelled, budget file unreadable); partial medium failures are per-proposal `Unknown` — the error return never smuggles state (DINV-2).
- **Delegation:** the existing `CodeHost.Checks(one)` implementation becomes a call through the same per-context derivation this port defines (adapter-internal; no interface change) — one implementation of "is this green?" (AP-11).

### 3.2 Boundaries and non-goals

- **Not solved here** — when and how often the reader is called (wake-watch-v2's cadence loop); what a consumer does with a landed claim (its own contract's goal-object verification); rendering (the status verb).
- **Owned by a future design** — wake-channel-v1's re-plumb of its per-goal probes onto this port (its own evolution — design-brainstorm:self-drive-wake-tech-design#O-1); further medium adapters (GitLab et al.) by following the kit (AP-3).
- **Implementation must not infer** — that `landed` implies mainline ancestry (it is the medium's claim — DINV-3); that an aggregate rollup may substitute for contexts (DINV-4); that a failed read may default to any definite state (DINV-2); that the reader may cache or persist medium content (DINV-7).
- **Must remain unchanged** — every existing port interface and its adapters' external behavior (basis NR-2: existing write paths keep working under the budget; NR-1's executor semantics are untouched by construction — this design adds a port, changes no executor code path).

### 3.3 Invariants

- **DINV-1** — A batch read answers **every** requested ref exactly once; a proposal the medium cannot answer for is a typed `unknown`, never an omission, an empty struct, or a healthy-looking zero value (the swallowed-evidence class, research §3.2).
- **DINV-2** — No consumer-visible state transition derives from a failed read: `rate-limited/offline/parse-defect/timeout` all resolve to `unknown — retry later`; the top-level error carries no per-proposal state.
- **DINV-3** — Landing state is the **medium's claim**: every `landed` carries the medium-reported landed revision id, and the reader asserts nothing about mainline ancestry — goal-object verification is the consumer's act (AP-26; basis FR-13).
- **DINV-4** — Check state derives from the per-context set compared against the required set; an aggregate rollup alone never yields green, and a required context with no observed run reports as `RequiredMissing` ⇒ not-green (basis FR-3).
- **DINV-5** — On a metered medium, one batch completes within the adapter's declared round-trip bound, asserted by the conformance kit (basis FR-2).
- **DINV-6** — Every content-write on a metered medium acquires from the shared budget before sending; saturation blocks the caller context-aware — a platform rate-limit response reaching a caller is a defect, not weather; corrupt or missing budget state starts conservative (window assumed spent).
- **DINV-7** — The reader is stateless: cursors are caller-held, no medium content is persisted, and the budget ledger is the only state — ephemeral and safe to delete.

### 3.4 Observability

- **Events/logs** — the reader and budget accept an injected event hook (`func(Event)`, nil = silent): read completions (medium, batch size, cost points, duration, unknown count), unknown resolutions (typed reason), budget deferrals (queue depth, deferred ms). Consumers wire the hook to their sink — wake-watch-v2 wires it to the local wake-events file (its D-9 surface); sweeps may wire it to their run report. The reader itself owns no file.
- **Metrics** — cost points and durations ride the events; the proof-run report (wake-watch-v2's committed artifact) distills them against basis NFR-1's ≤ 10% budget bound.
- **Alerts** — none of its own; staleness/liveness alarms are the consumer's (wake-watch-v2), fed by these events.
- **Debugging surface** — the status verb renders per-proposal `unknown` reasons verbatim; a `--json` style dump of one raw batch is an implementation affordance of the verb, not this port.
- **Trace/correlation** — events carry the batch id; the executor's existing trace fields apply where the caller supplies them (AP-5).

### 3.5 Security & privacy

none — matches the header row: reads ride the media's existing authenticated clients (no new credential or scope); nothing listens; no content leaves to any new destination; the budget only delays traffic that already flows. The injection posture is unaffected: the reader returns data; it carries no instructions and executes nothing it reads.

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

**Decision:** One token-bucket ledger per repo under the pyxis state dir, flock-guarded, shared by every pyxis process on the machine (design-brainstorm:self-drive-wake-tech-design#D-6):

- **Windows** — a per-minute and a per-hour window over content-writing requests; the limits are named constants in the adapter sourced from the platform's published secondary caps (the constants file cites the platform doc — the live authority; this document deliberately states no cap numbers, which the platform can change).
- **Acquire** — `Acquire(ctx, n)` before every content-write in the GitHub adapter; blocks until budget or ctx-cancel; never returns "denied" (backpressure, not errors — basis FR-8).
- **Recovery** — corrupt, unreadable, or missing ledger ⇒ start conservative: both windows assumed spent, refilling on the clock (DINV-6; basis §5.4's data rule).
- **Batching stays the cheapest budget** — the review write (comments[] + verdict in one call) remains the adapters' shape for multi-comment writes (basis §2.4's evidence row; enforced by NR-2's suites which run with the budget active).
- Carina: no budget (§3.7).

### 3.9 Testing strategy

Seams binned per the template's menu: the **Carina adapter** is local-substitutable (a real temp-dir store from local-work-tracker-v2 fixtures — test directly); the **GitHub adapter** is remote-but-owned (the port's production adapter + recorded-response fixtures for every DINV case: rollup-green-context-missing, 200-with-errors, rate-limited, pagination; plus one live smoke inside the proof run — AP-14's deployment-context proof); the **budget** is in-process with an injected clock (burst drill: nine write-sets concurrently, assert zero platform responses and full drain); the **kit** runs the same corpus against both adapters — a new adapter passes the kit or it does not exist (AP-3; basis NFR-5).

## 4. Alternatives considered

| Alternative | Why rejected |
|---|---|
| Widen `CodeHost`/`WorkTracker` with the new reads (design-brainstorm:self-drive-wake-tech-design#ALT-4) | edits two shared interfaces every adapter and fake implements — tree-wide churn for zero capability gain; splits one logical read across two ports (D-2) |
| A standalone reader package with its own medium clients (#ALT-6) | duplicates auth/config/client plumbing per medium; two GitHub clients drift; bypasses the port layer (AP-1/AP-11) |
| GraphQL + REST fallback (#D-8's rejected variant) | a second semantics-identical implementation kept forever, exercised ~never, running untested in the first real outage (the I11 shape); FR-9 already defines error semantics — the cadence is the retry |
| A config-declared required-check list | a second authority for "required" beside the medium's branch protection — the AP-11 twin-predicate seam; the medium's own declaration governs (§3.6) |
| Per-process budget division / a single-writer daemon | division fails the aggregate fit whenever process count is guessed wrong; the daemon is a resident coordinator (basis NG-1's deferred ground) with a machine-wide single point of failure (D-6) |

## 5. Risks

| ID | Risk | Impact | Mitigation | Verification / evidence |
|---|---|---|---|---|
| R1 | Platform API drift makes a read silently wrong or empty | the drive "sees" nothing to do; false quiet | parse-or-defect (DINV-1/DINV-2): unparseable ⇒ typed unknown, never empty-healthy; `errors` checked before data | recorded-fixture suite incl. 200-with-errors; live smoke in the proof run (A-5, A-8) |
| R2 | The budget file becomes a contention point under many processes | writes crawl | flock held only for the read-update-write of a tiny ledger (µs); windows amortize; measured in the burst drill | A-6's drill timings |
| R3 | The required-set fallback (all-observed-must-pass) is stricter than a repo intends | green withheld on optional checks | strict-in-the-safe-direction is deliberate (§3.6); the repo's remedy is declaring protection — the medium's own knob | A-3 fixture pair (protected/unprotected) |
| R4 | The Carina field mapping drifts from local-work-tracker-v2's schema | local reads silently wrong | the kit's fixture corpus is generated from lwt-v2 fixtures (Q1) — schema drift fails the kit | A-1/A-8 kit runs |
| R5 | Delegating `Checks(one)` through the new derivation changes a subtle existing behavior | executor drive-to-green misjudges | NR-2's suites run against the delegated path before merge | A-7 |

## 6. Acceptance

| ID | Acceptance criterion | Verify | Required evidence | Traces to |
|---|---|---|---|---|
| A-1 | A mixed 50-proposal fixture on **both** adapters returns all three state kinds matching ground truth per enumerated surface; every unknown is typed; no ref omitted | Test | kit run (CI) | prd:self-drive-wake/FR-1 · #DINV-1 |
| A-2 | A 50-proposal batch on the GitHub adapter completes in ≤ 2 API round trips, kit-asserted | Test | kit round-trip assertion | prd:self-drive-wake/FR-2 · #DINV-5 |
| A-3 | Aggregate-green-with-required-context-missing reports not-green; the protected/unprotected fixture pair exercises the required-set fallback | Test | fixture results | prd:self-drive-wake/FR-3 · #DINV-4 |
| A-4 | Every landed report carries the medium's landed revision id; a consumer-side fixture performs an ancestry check from the reader's output alone | Test | consumer fixture | prd:self-drive-wake/FR-13 · #DINV-3 |
| A-5 | Injected 429/5xx/timeout/parse-defect fixtures yield typed unknown; zero consumer-visible transitions during the injection window; 200-with-`errors` never presents as empty-healthy | Test | failure-injection suite | prd:self-drive-wake/FR-9 · prd:self-drive-wake/UB-1 · prd:self-drive-wake/UB-2 · #DINV-2 |
| A-6 | Nine tasks' write sets replayed concurrently: zero platform rate-limit responses, zero dropped writes, queue drains; corrupt-ledger start is conservative | Test | burst-drill report | prd:self-drive-wake/FR-8 · prd:self-drive-wake/UB-6 · #DINV-6 |
| A-7 | Existing executor suites pass unchanged with the budget active and `Checks(one)` delegated | Test | existing suites (CI) | prd:self-drive-wake/NR-2 |
| A-8 | The conformance kit runs green on both shipped adapters and its corpus documents every asserted behavior; one live-API smoke passes inside the proof run | Test + Inspection | kit + smoke output | prd:self-drive-wake/NFR-5 · §3.9 |
| A-9 | Steady-state read consumption ≤ 10% of the medium's read budget at the proof run's cadence, computed from emitted events | Analysis | proof-run report extract | prd:self-drive-wake/NFR-1 |

## 7. Open questions

| ID | Question | Owner | Blocks | Status | Resolution |
|---|---|---|---|---|---|
| Q1 | The exact Carina record-field mapping (landing/checks/comments ← lwt-v2 record schema) — fixed against lwt-v2 fixtures | @mdarabi | implementation (the kit corpus) | open | — |

## Principles

- **AP-1 — honored.** The read is a port; media vary behind it; the engine never learns which medium answered.
- **AP-3 — honored.** The conformance kit is the adapter-authoring contract; a new medium passes the kit or does not exist (A-8).
- **AP-6 — honored.** Closed typed structs end-to-end; unknown is a typed value; no free-form state.
- **AP-11 — honored.** One per-context check derivation (`Checks` delegates); the medium's branch protection is the single required-set authority (§3.6).
- **AP-15 — honored.** Decision-critical reads are exact and fresh — no conditional-read caching, no search indexes; the cadence, not a cache, provides cheapness.
- **AP-17 — honored.** Reads fail to *unknown*, and no destructive or dispatch action may derive from a failure (DINV-2) — positive evidence only.
- **AP-19 — honored (enabling half).** The reader makes zero calls uninvoked; idle cost belongs to the caller's gate (wake-watch-v2).
- **AP-22 — honored.** Reads are idempotent; the budget's acquire is raceable across processes by construction (flock).
- **AP-24 — honored.** The required-check set is read from the medium's own branch protection — the governing input stays the platform's human-set configuration; no parallel required-list of ours exists to drift from it (§3.6).
- **AP-26 — honored.** Landing is the medium's claim, marked as such and carrying the revision id; ancestry is the consumer's observation (DINV-3).
- **AP-27 — honored.** The port is a new file; adapters extend internally; no shared surface is edited.
- **AP-28 — honored (scoped).** Raw read events are routine ticks (the rule's own carve-out); the teaching distillation is the committed proof-run report (wake-watch-v2's artifact).
- **RW-3 — honored.** No medium workaround crosses the port: GitHub's query mechanics and Carina's scan both die at the adapter boundary.
- **Deviation:** none.
