# Self-drive wake — plan 001

| Field | Value |
|---|---|
| **Feature** | self-drive-wake |
| **PRD** | docs/prds/self-drive-wake |
| **Design** | docs/tech-design/state-reader-v2.md |
| **Plan version** | 001 |
| **Dialect** | harbormaster/fs-spec:v1 |
| **Epic title** | Build the wake capability: batched state reads, the watch loop, and the status verb |
| **Status** | Draft — freezes at merge |
| **Template version** | plan-template/v1 |

## Tasks

### T1 — Define the reader port, its typed state model, and the conformance kit skeleton

| Field | Value |
|---|---|
| **Blocked by** | — |
| **Size band** | 160–240 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/executor/port (new statereader file + kit file), internal/executor/port/port_test.go-adjacent kit fixtures |

**Goal:** A consumer-side reader interface exists with its complete typed result model, an in-process fake, and the conformance kit skeleton every adapter must pass.

**Current behavior:** No port can answer proposal state: the code-host port opens proposals and reads one proposal's checks; the tracker port's comment surface is write-only; landing and comment reads do not exist.

**Desired behavior:** One batch call returns, per requested proposal ref — always all of them, in order: landing state (open · landed · closed-unlanded · unknown) carrying the medium-reported landed revision id; per-context check states with required-but-missing contexts named; comments newer than a caller-held cursor, surface-tagged; and a typed unknown reason (rate-limited · offline · parse-defect · timeout) where the medium cannot answer. The kit skeleton asserts the all-refs-answered and typed-unknown properties against any implementation, and the fake passes it.

**Scope:**
- The reader interface (one batch method) and its closed result types, including the claim-marked landing with revision id
- The typed unknown model: per-proposal unknowns never omitted, never zero-valued; the top-level error reserved for transport-fatal cases
- An in-process fake implementing the port for consumer tests
- The conformance kit skeleton (all-refs-answered, typed-unknown, claim-marking) with authoring notes for future adapters

**Out of scope:**
- Any real medium adapter — the GitHub and Carina implementations are their own tasks
- Any consumer (loop, verb) — this task delivers the seam and its kit only

**Acceptance criteria:**
- [ ] The kit run against the in-process fake passes: a mixed fixture batch returns every ref exactly once with all three state kinds populated or typed-unknown
- [ ] A landed fixture result carries a non-empty revision id, and the kit asserts landing is presented as the medium's claim (the ancestry-check fixture consumes it without further reads)
- [ ] A fixture where the medium cannot answer yields a typed unknown reason — the kit fails any implementation that omits a ref or returns an untyped zero value
- [ ] Existing executor port tests pass unchanged — no existing interface is edited

**Key interfaces:**
- `ProposalStateReader` (the one batch method), `ProposalRef`, `ProposalState`, `CheckContext`, `Comment`, `Cursor`, `UnknownReason`

**Traces:** prd:self-drive-wake/FR-1 · prd:self-drive-wake/FR-9 · prd:self-drive-wake/FR-13 · prd:self-drive-wake/NFR-5 · design:state-reader-v2#3.1 · design:state-reader-v2#3.3

### T2 — Implement the Carina adapter's reads over the local store

| Field | Value |
|---|---|
| **Blocked by** | T1 |
| **Size band** | 160–240 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/executor/carinatracker, kit fixtures derived from the local-work-tracker fixtures |

**Goal:** The local tracker's adapter answers the reader port from store records, with the 1-second mtime-scan cadence advice declared.

**Current behavior:** The Carina adapter opens proposals and writes comments; nothing reads a proposal's landing, verdict, or notes back through a port.

**Desired behavior:** A batch read maps store records to the typed model — the store's accepted/closed markers as the medium's landing claim with the recorded revision id, recorded verdict fields as check states, record notes as comments — and declares cadence advice of one second realized by an mtime/size scan of the store directories, with zero scanning while unconsulted. Store filesystem errors resolve to typed unknowns, and an unparseable record surfaces a named parse-defect, never an empty-healthy result.

**Scope:**
- The record-to-state field mapping, fixed against the local tracker's fixture corpus (the frozen design's open mapping question resolves here)
- The mtime/size delta scan and the declared 1-second advice
- Typed-unknown handling for filesystem errors and unparseable records
- The conformance kit run green over the adapter with store-derived fixtures

**Out of scope:**
- Any change to the store's record schema or the tracker's write paths
- Any watch dependency — the scan is standard-library only

**Acceptance criteria:**
- [ ] The conformance kit passes over the adapter against a real temp-dir store built from local-tracker fixtures
- [ ] An accepted fixture record reports landed with its recorded revision id; an open one reports its verdict-derived check states and notes-since-cursor
- [ ] A corrupted record fixture yields a typed parse-defect unknown — never an empty or healthy-looking result
- [ ] The adapter declares 1 s cadence advice, and a no-change scan performs no record parsing

**Key interfaces:**
- `ProposalStateReader` (Carina implementation), the store record schema (read-only), cadence advice declaration

**Traces:** prd:self-drive-wake/FR-1 · prd:self-drive-wake/FR-6 · prd:self-drive-wake/FR-9 · prd:self-drive-wake/UB-1 · prd:self-drive-wake/UB-2 · prd:self-drive-wake/NFR-5 · design:state-reader-v2#3.7

### T3 — Implement the GitHub adapter's batched read

| Field | Value |
|---|---|
| **Blocked by** | T1 |
| **Size band** | 220–300 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/executor/ghcodehost, recorded-response fixtures |

**Goal:** One GraphQL query per batch answers the reader port for GitHub, per-context and claim-marked, with errors resolving to typed unknowns and the round-trip bound kit-asserted.

**Current behavior:** The GitHub adapter reads one proposal's aggregate check state; landing, comments, required contexts, and batching do not exist; a rate-limited or failed read has no typed shape.

**Desired behavior:** A single query covers up to a page of proposals: the merged flag with the merge-commit id (the claim), per-context check runs and status contexts compared client-side against the branch-protection-declared required set (absent protection ⇒ every observed context must pass), and the three comment surfaces filtered by the caller's cursor. The response's error list is checked before any data extraction — a partial or errored response marks all affected proposals typed-unknown; a rate-limit honors any retry-after by deferring to the next scheduled call, never an in-adapter retry loop. The kit asserts a 50-proposal batch completes in at most two API round trips. No REST fallback path exists.

**Scope:**
- The batched query shape and its closed decoding into the typed model
- The required-set derivation with the strict absent-protection fallback
- Errors-before-data handling: typed unknowns for rate-limited, offline, timeout, and parse-defect classes
- Recorded-response fixtures covering every unknown class plus the rollup-green-with-required-missing case; the kit green including the round-trip assertion

**Out of scope:**
- Write-path changes — the budget is its own task
- Any second read implementation — a REST fallback is a refused alternative

**Acceptance criteria:**
- [ ] The conformance kit passes over recorded fixtures: a mixed 50-proposal batch in ≤ 2 round trips with all three state kinds matching the recorded ground truth per surface
- [ ] The aggregate-green-with-required-context-missing fixture reports not-green with the missing context named
- [ ] A 200-with-errors fixture marks every affected proposal typed-unknown before any data extraction — no empty-healthy decode
- [ ] Every landed report carries the merge-commit id, and the ancestry-check consumer fixture verifies from the reader's output alone

**Key interfaces:**
- `ProposalStateReader` (GitHub implementation), the batched query document, required-set derivation, cadence advice declaration (≈10 s)

**Traces:** prd:self-drive-wake/FR-1 · prd:self-drive-wake/FR-2 · prd:self-drive-wake/FR-3 · prd:self-drive-wake/FR-6 · prd:self-drive-wake/FR-9 · prd:self-drive-wake/FR-13 · prd:self-drive-wake/UB-1 · prd:self-drive-wake/UB-2 · design:state-reader-v2#3.6

### T4 — Single-source check derivation: delegate the existing per-proposal read

| Field | Value |
|---|---|
| **Blocked by** | T2, T3 |
| **Size band** | 60–140 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/executor/ghcodehost, internal/executor/carinatracker (Checks internals only) |

**Goal:** The pre-existing one-proposal check read is answered by the new per-context derivation internally, so exactly one implementation of check state exists per adapter.

**Current behavior:** The existing per-proposal check read computes its own answer independently of the new batched read — two derivations of the same predicate.

**Desired behavior:** The existing read's contract and signature are unchanged, but its answer comes from the same per-context derivation the batched read uses; the executor's drive-to-green behavior is byte-equivalent on green and red fixtures.

**Scope:**
- Internal delegation in both adapters, no interface change
- Equivalence fixtures proving the delegated answer matches the prior behavior on green, red, and pending cases

**Out of scope:**
- Any signature or semantics change to the existing port method
- The batched read itself — already landed

**Acceptance criteria:**
- [ ] Existing executor suites pass unchanged with delegation active
- [ ] A fixture pair (green · red · pending) yields identical existing-read answers before and after delegation
- [ ] Exactly one per-context derivation function remains per adapter — the old computation is removed, not shadowed

**Key interfaces:**
- The existing one-proposal checks method (unchanged contract), the shared per-context derivation

**Traces:** prd:self-drive-wake/FR-3 · prd:self-drive-wake/NR-2 · design:state-reader-v2#3.1

### T5 — Implement the shared write budget on the metered adapter

| Field | Value |
|---|---|
| **Blocked by** | T1 |
| **Size band** | 180–260 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/executor/ghcodehost (write path), a new write-budget package under internal/executor, the pyxis state dir layout |

**Goal:** Every content-write on the GitHub adapter passes one machine-shared token budget that blocks callers instead of ever surfacing a platform rate-limit response.

**Current behavior:** Writes go straight to the platform; concurrent processes can collectively exceed the published per-minute and per-hour content caps, surfacing rate-limit errors to whichever caller loses.

**Desired behavior:** A per-repo, flock-guarded budget ledger in the state directory is acquired before every content-write by every pyxis process; saturation blocks context-aware (cancellation still works); a corrupt, unreadable, or missing ledger starts conservative with both windows assumed spent; the caps are named constants citing the platform's published limits; an injected event hook reports deferrals. The local tracker is unthrottled.

**Scope:**
- The budget ledger (windows, flock, conservative recovery) and its acquire call
- Wiring into every content-write of the metered adapter
- The injected event hook for deferral reporting (nil-safe)
- The burst drill: nine tasks' write sets replayed concurrently with zero simulated-platform rate-limit responses and zero drops

**Out of scope:**
- Any resident coordinator process — refused ground
- Read-path budgeting — reads are already batch-cheap by design

**Acceptance criteria:**
- [ ] The burst fixture (nine concurrent write sets) completes with zero platform rate-limit responses, zero dropped writes, and a drained queue
- [ ] A corrupted-ledger start behaves conservatively (window assumed spent) and recovers on the clock
- [ ] Existing executor write-path suites pass unchanged with the budget active
- [ ] Cancellation during a blocked acquire returns promptly with no write sent

**Key interfaces:**
- `Acquire(ctx, n)` on the budget, the ledger file schema, the deferral event hook

**Traces:** prd:self-drive-wake/FR-8 · prd:self-drive-wake/UB-6 · prd:self-drive-wake/NR-2 · design:state-reader-v2#3.8

### T6 — Single-source the wake primitives: staleness predicate and signal grammar

| Field | Value |
|---|---|
| **Blocked by** | — |
| **Size band** | 80–160 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/wake (staleness + signal, parametrization only) |

**Goal:** The existing wake package's staleness predicate and wake-signal grammar are importable, parametrized primitives both wake stacks share — the argus stack's behavior unchanged.

**Current behavior:** The wake package's staleness and signal handling serve the argus tick with its own bounds; a second consumer would have to copy the logic — the twin-predicate drift class.

**Desired behavior:** One staleness predicate (age vs bound, with the bound-strictly-exceeds-cadence relation) and one closed-schema signal grammar (identifier + reason kind, nothing else; unknown fields are parse defects) accept caller-supplied parameters; the argus tick consumes them exactly as before with its existing bounds, and every argus-side test stays green untouched.

**Scope:**
- Parametrizing the predicate and grammar for a second consumer, additively
- A property test: the bound>cadence relation refuses inverted parameters
- Poisoned-signal fixtures: extra fields rejected at parse

**Out of scope:**
- Any behavior change to the argus tick, its awaits, or its ledgers
- The new loop's parameters — they arrive with their consumer tasks

**Acceptance criteria:**
- [ ] Every existing wake-package and argus-side test passes unchanged
- [ ] Two consumers with different parameters get consistent verdicts from one predicate implementation — proven by a shared-fixture test
- [ ] A signal fixture with any extra field fails closed at parse with a named defect

**Key interfaces:**
- The staleness predicate (parametrized), the wake-signal schema, the bound/cadence relation check

**Traces:** prd:self-drive-wake/FR-10 · prd:self-drive-wake/FR-12 · design:wake-watch-v2#3.8 · design:wake-watch-v2#3.3

### T7 — Build the watch loop: await derivation, edge-triggered wakes, completion exit

| Field | Value |
|---|---|
| **Blocked by** | T1, T6 |
| **Size band** | 220–300 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | a new watch package (loop core), consumer tests against the port fake |

**Goal:** The in-process loop derives what a frozen plan awaits, checks on adapter advice, emits instruction-free wakes whose only effect is scheduling a sweep, idles free, never hot-loops, and exits on plan completion.

**Current behavior:** Nothing runs the reads: no process derives awaits from a plan, notices transitions, or re-invokes anything — the operator is the clock.

**Desired behavior:** Derivation from durable state alone yields three await kinds — open proposals, claimed tasks awaiting their proposal, ready-unclaimed tasks (immediate trigger); an empty set is provably a completed plan and exits with a summary. While awaits exist, the loop ticks on the adapter's advice, coalesces a burst into one wake, and invokes the injected sweep entry edge-triggered: an unchanged derived state after a sweep drops re-invocation to cadence pace with the worker-wait age accruing — a stuck frontier surfaces as staleness, never a spin. Duplicate or lost wakes converge (level-triggered); restart re-derives with no duplicate effects; cancellation stops cleanly.

**Scope:**
- Await derivation with the three kinds and the completion argument
- The cadence tick, burst coalescing, and edge-triggered sweep invocation with the stuck-frontier downgrade
- Idle-zero and completion-exit behavior
- Restart, duplicate-wake, and two-loop convergence drills against the port fake and an injected fake sweep

**Out of scope:**
- The CLI verb, the events file, and the executor wiring — the next task
- Any claim, merge, or tracker write from the loop — refused by invariant

**Acceptance criteria:**
- [ ] Drill: a transition on the fake reader yields exactly one sweep invocation per burst; identical consecutive derived states never re-invoke immediately
- [ ] Drill: a permanently-failing ready frontier settles to cadence-paced re-invocation with accruing worker-wait age — call counts prove no hot loop
- [ ] Drill: kill/restart mid-await re-derives and continues with zero duplicate invocations; a fixture plan driven to completion exits with the summary
- [ ] One idle hour on an await-free derivation performs zero reader calls

**Key interfaces:**
- The loop entry (plan ref + reader + sweep func + clock), the await-set derivation, the edge-trigger state

**Traces:** prd:self-drive-wake/FR-4 · prd:self-drive-wake/FR-5 · prd:self-drive-wake/FR-6 · prd:self-drive-wake/UB-3 · prd:self-drive-wake/UB-4 · design:wake-watch-v2#3.1 · design:wake-watch-v2#3.3

### T8 — Ship the watch verb: executor wiring, wake events, sweep attribution

| Field | Value |
|---|---|
| **Blocked by** | T7 |
| **Size band** | 180–260 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | the pyxis CLI (new watch verb), the watch package (events sink), the executor dispatch event emission (additive properties), the gitignore |

**Goal:** A user runs the watch verb on a frozen plan and the loop drives the real executor, with every check and wake appended to a local gitignored events file and sweep invocations attributed by initiator and scope.

**Current behavior:** The loop exists as a library; the executor's sweep-start event carries no initiator or scope; no wake-events surface exists.

**Desired behavior:** The verb resolves the plan, wires the real reader and the executor's public one-shot sweep entry, and runs the loop under terminal cancellation. Every check and wake attempts a typed append to a size-rotated, gitignored events file before its effect — effects never block on the sink, and an unwritable sink degrades loudly via a warning and the liveness surface. The executor's existing sweep-start event gains initiator and scope properties additively, populated by every invoker. The executor's one-sweep semantics and suites are untouched.

**Scope:**
- The verb (argument handling, wiring, cancellation) invoking the executor entry unchanged
- The wake-events sink: typed kinds for checks, wakes, unknowns, budget deferrals; rotation; the gitignore entry
- The additive initiator/scope properties on the sweep-start event, populated for operator and wake invocations
- The events-unwritable degradation path (warn once per rotation window, keep effects flowing)

**Out of scope:**
- Feature-lifecycle semantics — ownership, branches, e2e stay future ground
- The status verb — its own task

**Acceptance criteria:**
- [ ] A fixture plan driven through the verb lands real sweeps: transitions produce claims and proposals via the executor with zero executor-code changes beyond the additive event properties
- [ ] Every check and wake appears as a typed line in the events file; rotation caps the file; the file is gitignored
- [ ] With the sink unwritable, effects continue, a warning is emitted, and the liveness surface reports the sink defect
- [ ] Existing executor suites pass unchanged; sweep-start events from the verb carry wake initiator and the plan scope, and operator-invoked sweeps carry theirs

**Key interfaces:**
- The watch verb contract, the events sink and its kinds, the sweep-start initiator/scope properties

**Traces:** prd:self-drive-wake/FR-4 · prd:self-drive-wake/FR-5 · prd:self-drive-wake/FR-12 · prd:self-drive-wake/NR-1 · prd:self-drive-wake/NFR-4 · design:wake-watch-v2#3.4 · design:wake-watch-v2#3.9

### T9 — Ship the status verb: wait ages, thresholds, and loud liveness

| Field | Value |
|---|---|
| **Blocked by** | T6, T8 |
| **Size band** | 220–300 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | the pyxis CLI (new status verb), the watch package (render), the repo config surface (threshold keys) |

**Goal:** One read-only command shows, per task, what the factory is waiting on and for how long, flags waits past their configured thresholds, and makes a dead or unverifiable checker loud.

**Current behavior:** Diagnosing a quiet drive means hand-correlating browser tabs; no threshold data exists; a dead checker is indistinguishable from a quiet repo.

**Desired behavior:** The verb renders the loop's own derivation — per task: state, waiting-on kind, wait age, a flag when the age exceeds the wait-kind's threshold — plus the checker's last-check age from the events file, flagged when it exceeds one cadence, with the defect event firing at the configured bound. Thresholds are closed-parse config keys (review, ci, worker, checker) with the ruled defaults; unknown keys are named errors; an absent section means defaults. Unknowns render with reasons, offline renders the last observation marked stale, an absent or unwritable events file renders a named cannot-verify-liveness defect, and exit status distinguishes rendered-stale from cannot-render. The verb performs zero writes.

**Scope:**
- The rendering pass over the shared derivation with per-kind waiting-on classification and age starts fixed against fixtures
- The threshold config keys with closed parse, defaults, and the shared-predicate consumption
- The liveness footer: last-check age, the one-cadence display flag, the configured-bound defect event, the cannot-verify-liveness path
- Degraded renders: unknown-with-reason, stale-marked offline, distinguishing exit codes

**Out of scope:**
- Any mutation — the verb is read-only by invariant
- Learned thresholds — parked ground

**Acceptance criteria:**
- [ ] The seeded fixture set (waiting-review · ci-running · blocked · stale-wait · dead-checker · unknown · offline) renders every column and flags exactly the seeded rows, with zero writes performed
- [ ] Threshold keys parse closed with a named error on an unknown key; absent config yields the ruled defaults; the shared predicate consumes them
- [ ] The kill-the-checker drill renders the stale last-check flag within one cadence and the defect event fires at the configured bound
- [ ] An absent or unwritable events file renders the named cannot-verify-liveness defect — never a healthy footer

**Key interfaces:**
- The status verb contract, the wait-kind classification, the threshold config schema, the liveness read

**Traces:** prd:self-drive-wake/FR-7 · prd:self-drive-wake/FR-12 · prd:self-drive-wake/UB-1 · prd:self-drive-wake/UB-5 · design:wake-watch-v2#3.6 · design:wake-watch-v2#3.8

### T10 — Reserve the push slot: the hint-only contract and its stub drill

| Field | Value |
|---|---|
| **Blocked by** | T8 |
| **Size band** | 60–140 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | the watch package (slot interface + stub) |

**Goal:** The push-channel slot exists as a narrow hint-only contract with a stub adapter proving that push can only ever accelerate, never carry.

**Current behavior:** No push surface exists; nothing defines what a future push adapter may or may not do.

**Desired behavior:** A deliver-hint interface whose only effect is advancing the loop's next check to now; no payload beyond the hint reaches the loop; an absent or dead adapter changes nothing but latency. The stub is the only v1 implementation, and the kill-stub drill is the conformance bar recorded for future adapters.

**Scope:**
- The slot interface and the loop's hint handling
- The stub adapter and the kill-stub drill
- The conformance note future adapters are held to

**Out of scope:**
- Any real push transport — waiting-room ground

**Acceptance criteria:**
- [ ] With the stub wired, a delivered hint advances the next check to now and nothing else — state, awaits, and sweep behavior are unchanged
- [ ] Killing the stub mid-drill leaves behavior byte-identical to timer cadence — proven by comparing event traces
- [ ] A hint carrying extra payload is rejected at parse with a named defect

**Key interfaces:**
- The deliver-hint slot interface, the stub adapter

**Traces:** prd:self-drive-wake/FR-10 · prd:self-drive-wake/FR-11 · design:wake-watch-v2#3.7

### T11 — Run the proof: a real feature end to end on both backends, with the committed report

| Field | Value |
|---|---|
| **Blocked by** | T4, T5, T9, T10 |
| **Size band** | 140–220 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | the watch package (report generator), a proof-run report under the feature's docs, drill fixtures for the diagnosis rubric |

**Goal:** The shipped capability is proven on reality: a real multi-task feature driven end to end on each backend with the contract's numbers distilled from the event streams into a committed report, plus the operator's quiet-state diagnosis drill.

**Current behavior:** Every acceptance so far is fixture-level; no end-to-end run exists, no report generator, and the contract's ship-rule numbers are unmeasured.

**Desired behavior:** A report generator merges the wake-events file with the executor's stream by timestamp and computes: operator-initiated sweep invocations within the driven scope (target zero), transition-to-wake latency percentiles per backend against the promised bounds with the clock-offset note, and rate-limit responses attributable to the run (target zero) with read-budget share. The proof run drives a real feature of at least six tasks on the GitHub backend and a Carina-tracked repo, commits the generated report as the evidence artifact, and stages the five-scenario quiet-state drill the operator judges against the contract's rubric.

**Scope:**
- The report generator over the two event streams with the three metric families and the budget share
- The proof-run execution on both backends and the committed report
- The five quiet-state drill scenarios staged for the operator's judgment

**Out of scope:**
- Relaxing any bound to fit an observed number — a miss is a defect to fix, never a documented limitation

**Acceptance criteria:**
- [ ] The committed report shows zero operator-initiated sweeps in scope, latency percentiles within the promised bounds per backend, and zero attributable rate-limit responses, each computed from the recorded streams
- [ ] Read-budget share over the run is within the contract's ceiling, computed from check events
- [ ] The five diagnosis scenarios are staged and the operator's verdict is recorded against the rubric
- [ ] The report generator is deterministic over a fixture stream pair — same streams, same report

**Key interfaces:**
- The report generator (two streams in, the metric families out), the drill scenario set

**Traces:** prd:self-drive-wake/NFR-1 · prd:self-drive-wake/NFR-2 · prd:self-drive-wake/NFR-3 · design:wake-watch-v2#3.4 · design:state-reader-v2#3.9

## Coverage

| Requirement | Tasks |
|---|---|
| prd:self-drive-wake/FR-1 | T1, T2, T3 |
| prd:self-drive-wake/FR-2 | T3 |
| prd:self-drive-wake/FR-3 | T3, T4 |
| prd:self-drive-wake/FR-4 | T7, T8 |
| prd:self-drive-wake/FR-5 | T7, T8 |
| prd:self-drive-wake/FR-6 | T2, T3, T7 |
| prd:self-drive-wake/FR-7 | T9 |
| prd:self-drive-wake/FR-8 | T5 |
| prd:self-drive-wake/FR-9 | T1, T2, T3 |
| prd:self-drive-wake/FR-10 | T6, T10 |
| prd:self-drive-wake/FR-11 | T10 |
| prd:self-drive-wake/FR-12 | T6, T8, T9 |
| prd:self-drive-wake/FR-13 | T1, T3 |
| prd:self-drive-wake/UB-1 | T2, T3, T9 |
| prd:self-drive-wake/UB-2 | T2, T3 |
| prd:self-drive-wake/UB-3 | T7 |
| prd:self-drive-wake/UB-4 | T7 |
| prd:self-drive-wake/UB-5 | T9 |
| prd:self-drive-wake/UB-6 | T5 |
| prd:self-drive-wake/NR-1 | T8 |
| prd:self-drive-wake/NR-2 | T4, T5 |
| prd:self-drive-wake/NFR-1 | T11 |
| prd:self-drive-wake/NFR-2 | T11 |
| prd:self-drive-wake/NFR-3 | T9, T11 |
| prd:self-drive-wake/NFR-4 | T8 |
| prd:self-drive-wake/NFR-5 | T1, T2, T3 |
