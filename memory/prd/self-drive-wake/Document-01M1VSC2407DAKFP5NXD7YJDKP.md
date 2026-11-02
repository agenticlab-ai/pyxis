# Amendment 004 — review-obligation recovery

| Field | Value |
|---|---|
| **Amends** | self-drive-wake *(chain through 003)* |
| **Status** | Approved |
| **Owner** | @mdarabi |
| **Date** | 2026-08-06 |
| **Learned-from** | review-thread:pyxis#1034 · dependency-change:prd-implement-now-005 |
| **Blast radius** | tier: contract-changing; touches: §0, §1, §2.1, §3.3, §4.1..§4.4, FR-7→FR-17, FR-15..FR-16, NR-3, UB-5→UB-10, UB-9, UB-11, §5.4..§5.5, EVT-7..EVT-8, DOD-1→DOD-12, DOD-2→DOD-15, DOD-3→DOD-13, DOD-6→DOD-14, DOD-11, §6.1..§6.4, §7.1..§7.2, §8; reason: configured implementation review is now a durable frontier independent of implementation readiness, so a driver that observes only proposal/task state can launder a failed or interrupted review to a clean empty run; provider attempts cannot meter driver spend, and exhausted/rearmed dispositions require immutable evidence |
| **Tracking** | issue:1029 · PR #1034 |
| **Template version** | amendment-template/v2 |

## Operations

### op: supersede §1 Summary

Reason: configured-review obligations add a second durable frontier and a fourth committed part; the old summary defines success and scope only in terms of proposal transitions.

- **Problem** — Proposal transitions and configured-review obligations can both leave a drive waiting, yet the factory neither observes them as one durable frontier nor reliably distinguishes “nothing remains” from “review failed and is still pending.” The operator becomes the clock, and a restart can launder advisory review failure into a clean empty run.
- **For whom** — Anyone running a pyxis feature drive on any installed repo (GitHub-backed or local Carina), and the feature/argus drivers contracted to continue from implementation and review completions.
- **Why now** — The proposal-state consumers exist, and prd:implement-now FR-40..FR-47 now make configured review a durable exact-revision obligation whose bounded recovery belongs to the driver rather than the one-shot executor.
- **Direction** — Four named parts behind one contract: a batched **proposal-state reader**; an exact **review-state reader** joining canonical obligations to consumer recovery state; a timer-driven, instruction-free **wake consumer** that derives one combined frontier and invokes one consumer-bounded review-only sweep when needed; and a read-only **status verb** showing implementation and review waits, ages, failure classes, provider attempts, recovery bounds, and liveness.
- **Appetite** — Everything-or-nothing: both media, all four parts, the combined-frontier/recovery failure matrix, and the proof run. Excludes an always-on background daemon, push adapters, hosted relays, multi-machine coordination, cadence knobs, and implementation/reviewer decision authority.
- **Success** — Zero manual re-invocations across a real 6-task feature on both backends; merge→wake p95 ≤ 15 s GitHub / ≤ 1 s Carina; every pending review is recovered to authenticated verification or remains visibly red/exhausted without implementation redispatch.
- **Guardrail** — Zero rate-limit trips; steady-state reads ≤ 10% of budget; zero clean/idle decision while review state is pending, unknown, or exhausted; zero implementation action during review-only recovery.
- **Top risks** — silently dead checking; API/schema drift; burst limits; review-state unknown interpreted as empty; repeated full sweeps redispatching succeeded work; and unbounded or laundered review retries.

### op: supersede §2.1 Problem statement

Reason: the durable review frontier is a second instance of the operator-as-clock problem, not merely a solution detail.

The factory needs to observe two durable frontiers. First, it can write proposals but cannot natively answer which landed, settled checks, or received new comments, so the operator re-invokes after each transition. Second, configured implementation review now persists an exact-revision obligation after a task is green; a failed or interrupted attempt remains pending even when no implementation task is ready. A driver that reads only proposal/task state reports the latter as an empty clean run, and retrying whole sweeps risks redispatching succeeded work. The operator is therefore both the clock and the memory of failed review. This feature supplies exact proposal and review-state reads, a combined readiness predicate, bounded review-only recovery, and status that makes pending/unknown/exhausted work explicit.

### op: supersede §3.3 Guardrails and decision rule

Reason: the old ship rule can pass on M1–M3 while the new blocking recovery proof fails.

| Guardrail metric | Threshold | Action if breached |
|---|---|---|
| Rate-limit trips attributable to this feature | any occurrence | halt the wake channel; fix throttle/cadence before re-enabling |
| Read-budget share while one drive runs | > 10% sustained | lengthen adapter-advised cadence; investigate query shape |
| Idle cost while the combined frontier is empty | > 0 medium/store calls attributable to an await-free channel | defect: FR-5 or the combined-empty predicate broke |
| Review-state laundering | any idle/clean decision while review state is pending, unknown, or exhausted | fail closed; block activation; retain the state/evidence and repair recovery |
| Review-only implementation activity | any task route, claim, worker, proposal write, or changed succeeded terminal | fail closed; block activation; preserve the task terminal and repair the boundary |
| Existing executor/adapter suites and NR-1..NR-3 | any regression | stop; the capability may not change one-sweep or implementation semantics |

**Ship rule (pre-registered):** M1 = 0, M2 within bounds, M3 clean, every guardrail above clean, and DOD-11..DOD-15 green on the same proof composition → ship. A fixture where M1–M3 pass but combined-frontier recovery, exhaustion visibility, event durability, or zero-implementation isolation fails must produce `do not ship`; no other section may weaken this predicate.

### op: supersede §4.1 Approach and alternatives considered

Reason: the chosen solution has four parts after configured review becomes a durable second frontier.

The chosen shape is four named parts behind one contract. The **proposal-state reader** returns batched landing/check/comment state on every medium. The **review-state reader** exact-reads canonical implement-now obligations and joins them to a consumer-owned durable recovery ledger. The **wake consumer** derives one combined implementation/review readiness predicate, coalesces duplicate hints into a durable driver round, and—when any obligation is eligible—starts one review-only executor sweep over the eligible in-scope frontier while the provider admits at most one attempt per obligation. The **status verb** renders proposal waits, review pending/unknown/exhausted states, ages, counters, failure classes, and checker liveness. Timer cadence remains the v1 wake mechanism; push slots stay hint-only.

| Option | Sketch | Why not chosen |
|---|---|---|
| A (chosen) | proposal reads + exact review-state/recovery reads + combined wake consumer + status | observes every durable frontier and keeps one-shot executor ownership intact |
| B | proposal-state wake only | launders failed/interrupted configured review when implementation is empty |
| C | invoke one executor sweep per obligation | N sweeps each rescan the same N-obligation frontier and race one another |
| D | retry on any non-zero process exit | conflates task and review failure and can redispatch succeeded work |
| E | push-first with slow polling backup | adds a silently-failable listener before measured need |
| F | hosted central relay | privileged middleman and custody surface; permanently refused |
| Do nothing | — | the operator remains the clock and review failure remains non-durable at the driver boundary |

### op: supersede §4.2 Scope perimeter

Reason: delivery cannot require recovery as an out-of-band “plus” while the authoritative scope lists only proposal reads, wake signals, and status.

- **In (plan of record)** — proposal-state port + GitHub/Carina adapters; canonical review-state consumer port; consumer-owned durable review-recovery ledger; combined readiness derivation; one deduplicated selected-frontier review-only sweep per driver round; provider-side at-most-one attempt per obligation; bounded recovery/exhaustion with immutable evidence; timer wake + reserved push slots; status for implementation and review waits; write budget guard; typed events; conformance/failure drills; 6-task proof on both backends.
- **Future (believed next, not committed)** — always-on background driver; push adapters; learned thresholds.
- **Out** — merging/approving; implementation or review decision authority; review artifact verification itself (owned by implement-now/impl-review); hardware isolation; hosted relay; multi-machine coordination; cadence knobs.

### op: supersede §0 Read to rule — the human reviewer's section (~5 min)

Reason: the routing section must expose the new second readiness frontier and the non-laundering decision to the signing human.

*The body is the contract; this section only routes attention — on any conflict the body governs; machines ignore it.*

**Judge these**

| # | The call | Could have gone | If you disagree |
|---|---|---|---|
| 1 | **Never a hosted relay** — pyxis will never run a central notify/webhook service for users | an agenticlab-hosted relay making setup one line shorter | you accept custody of every user's repo activity and a required middleman in every install → NG-3 |
| 2 | **Timer-only v1; push = reserved slots** — no listener, tunnel, or runner ships now | shipping the GitHub Actions listener channel in v1 | v1 gains a per-machine install step and a silently-failable moving part for ~10 s of latency → §4.1, FR-11, WR-1 |
| 3 | **Both backends day one** — the port shape is settled with Carina and GitHub in view | GitHub-first, Carina later | the read surface freezes around the metered medium → §2.4, FR-1/FR-2 |
| 4 | **Reads are batched, per-check, and marked as the medium's claim** | per-proposal reads; trusting aggregate badges | N+1 API cost and false advancement → FR-2, FR-3, FR-13 |
| 5 | **A failed read means unknown, never no** | treating a failed probe as absence | a transient failure drives a false empty frontier → FR-9, UB-2 |
| 6 | **Implementation and configured-review readiness form one wake predicate** — pending review is work even when no task is ready | stopping after an implementation-empty sweep | an interrupted or failed review is laundered to exit 0 on the next drive round → FR-15, FR-16, UB-9 |
| 7 | **Review recovery is one selected-frontier review-only sweep per round and consumer-bounded** — it never redispatches the succeeded task | one sweep per obligation or counting only provider attempts | green implementation history is rewritten, N sweeps race, or setup failures loop without consuming the driver guard → FR-15, FR-16, DOD-11 |
| 8 | **Everything-or-nothing appetite** — no pre-agreed cut-line | dropping comment reads or recovery evidence first | the finish line moves instead of scope being explicit → header Appetite, §6.3 R5 |

**Sanity-check the numbers** — merge→wake p95 ≤ 15 s (GitHub) and ≤ 1 s (Carina) at the proof run (M2, NFR-2); one 50-proposal state read in ≤ 2 API round trips (FR-2); zero manual re-invocations (M1); zero rate-limit trips (M3); one review-only sweep per driver round regardless of pending-obligation count; and every obligation ends authenticated-verified or remains visibly pending/red after its consumer recovery bound (FR-15, FR-16, DOD-11).

**The author's open questions** — Q1: what “too long” means per wait kind (§7.2); Q2: how argus binds to this capability (§7.2). Neither changes the closed review-recovery behavior.

**Skip the rest** — structure/IDs/fold → docgate; substance and coherence → `/prd-review`; query shapes, throttle storage, review-state reader placement, retry bound, and event persistence → the superseding technical designs.

### op: supersede §4.3 Experience: flows and states

Reason: the existing experience ends once proposal state reopens the implementation frontier and has no state for a durable review obligation after implementation is green.

**Flow F1 — a driven feature continues by itself (serves J1, J3)**

| Step | User does | System responds | States covered | Design |
|---|---|---|---|---|
| 1 | starts a drive on a frozen plan | the drive exact-reads proposal/task state and configured-review obligations, registers awaits for every non-empty frontier, and runs no checks only when both frontiers are empty | ideal · empty · review-only | superseding state-reader and wake designs |
| 2 | reviews + merges a task PR in the browser | within the cadence bound, the reader observes landing and the wake channel delivers `(await-id, transition)` | ideal · loading | superseding state-reader and wake designs |
| 3 | — | the drive re-derives the combined frontier through fresh reads and starts each newly ready implementation task at most once | ideal · partial | superseding wake design |
| 4 | — | when one or more review obligations are eligible, the drive invokes one bounded review-only sweep for that round over the eligible in-scope frontier; the provider exact-reads immutable obligations, admits at most one attempt per obligation, and performs zero implementation dispatch | review-only · partial · error | superseding wake design + prd:implement-now FR-40..FR-47 |
| 5 | closes the laptop / kills the process | on restart the combined frontier is re-derived from durable state; interrupted review remains pending and red until authenticated verification or explicit consumer recovery exhaustion | interruption · recovery · exhausted | superseding state-reader and wake designs |

**Flow F2 — “why is it quiet?” (serves J2)**

| Step | User does | System responds | States covered | Design |
|---|---|---|---|---|
| 1 | runs the status verb | one read-only pass renders per implementation await and review obligation: state, waiting-on, `age_s` since the consumer first observed the obligation, provider attempt count, consumer recovery epoch/count/bound, terminal failure class, a too-long flag, and checker last-check age | ideal · empty · review-only · partial · error | superseding state-reader design |

- **Empty** — no implementation awaits and no pending review obligations: say so explicitly; only this combined-empty state is idle.
- **Review-only** — no implementation task is ready but at least one review obligation is pending: show immutable identity, provider-attempt and consumer recovery counters/bound, age, and last failure; schedule at most one selected-frontier review-only sweep for the round.
- **Loading** — a check is in flight: render the last completed picture plus its age.
- **Partial** — unknown proposal or review state is typed and retained, never omitted or interpreted as empty.
- **Error** — medium/store unreachable: render the last observation as stale, keep the frontier non-empty, and exit non-clean; consumer recovery exhaustion is immutable escalation, never success.
- **Accessibility** — terminal status is never carried by color alone.
- **Localization** — none (single-locale CLI).

### op: supersede §4.4 Scenarios

Reason: the live scenario set must exercise the second frontier, selected-frontier cardinality, setup-failure accounting, and immutable exhaustion rather than only proposal wakes.

**Ordinary continuation.** The operator merges one task proposal. Within the medium bound, the drive exact-reads state and dispatches the newly ready task once.

**Review-only continuation.** Two configured-review obligations are pending and no implementation task is ready. One driver round initializes/reads their recovery entries, appends one admission for each eligible obligation, and invokes exactly one selected-frontier review-only sweep. The provider admits at most one attempt per selected obligation. Authenticated artifacts clear both; zero task route, claim, worker, or proposal operation occurs.

**Setup failure and exhaustion.** The review backend's auth preflight fails before a provider attempt. The consumer admission still counts for every selected obligation. At `recovery_count == recovery_bound`, no sweep launches; exhaustion is appended to immutable git evidence and status remains red until authenticated verification or an explicit evidenced rearm creates a new epoch.

**Unknown state.** The canonical obligation or recovery ledger is corrupt/unavailable. The combined frontier is unknown/non-empty, EVT-8 records bounded deferral, no implementation/review launches, and the round is non-clean until a successful exact read.

**Duplicate hints and restart.** Timer and push hints arrive together; one durable round coalesces them. A crash before admission consumes nothing; a crash after append-first admission consumes one recovery unit. Restart exact-replays both stores and never changes revision, bound, prior epoch, or succeeded implementation truth.

**Decision:** Solution gate — zero TBDs may pass.

| Reviewer | Role | Date | Verdict |
|---|---|---|---|
| @mdarabi | operator (via introducing PR review) | — | — |

### op: add FR

Reason: the state reader needs a typed, exact source for the new durable frontier; inferring it from the last run summary repeats the laundering defect.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-15 | When a consumer derives drive readiness, the state reader shall exact-read both the canonical configured-review obligation store and the consumer-owned durable recovery ledger and return a closed `review-wake-state/v1` containing each pending obligation's stable identity, task/proposal/revision binding, effective review backend, last terminal failure class, provider `review_attempt_count`, and consumer `first_observed_at_utc`, `recovery_epoch`, `recovery_count`, and frozen positive `recovery_bound`; it shall derive `age_s = floor(max(0, now_utc - first_observed_at_utc)/1s)` from an injected wall clock, so attempts/restarts never reset age and backward clock movement clamps rather than underflows. Missing recovery state for a newly observed obligation shall be initialized append-first with source-owned UTC observation time, count 0, and the current closed policy bound; later config/restarts shall not change that epoch's time or bound. Unreadable, structurally invalid, or unavailable whole-store state on either side shall instead be typed `unknown` and keep readiness non-empty, never become zero pending. | provider attempts do not count factory/preflight failures, and the provider contract exposes no creation time; the retry and age guards belong in the consumer that spends and observes | Must | critical | empty, one-pending, mixed-backend, verified-cleared, corrupt/unavailable provider, corrupt/unavailable recovery ledger, stale-summary, initial-state, config-change, new/retried/long-pending, restart, and backward-clock fixtures match canonical ground truth; injected-clock assertions prove append-first time and exact `age_s`; every unknown fixture returns non-empty and never idle | Test + Failure injection | G1, J3, F1, prd:implement-now FR-40, FR-42, UB-21, incident:I8 |

### op: add FR

Reason: a pending review must trigger work even when implementation has no frontier, but retry needs a bound and must never recreate implementation work.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-16 | For each durable driver round — one cycle identified by a fresh `drive_round_id` from exact combined-frontier read through persisted decisions and either one settled selected-frontier executor invocation or a persisted no-launch exhausted/unknown decision, with duplicate wakes coalesced into that cycle — the wake consumer shall select every in-scope pending obligation whose `recovery_count < recovery_bound`. If and only if that selected set is non-empty, it shall append-first increment each member's recovery count and issue exactly one review-only executor invocation carrying the round ID and selected IDs; a preflight/factory/auth failure still consumes admission even if the provider creates no attempt. `recovery_count == recovery_bound` is exhausted and excluded; when every pending obligation is exhausted, the round appends/replays EVT-7 evidence and launches nothing. The provider scans only the selected frontier and admits at most one attempt per member; the invocation performs zero implementation operations. Authenticated verification clears the await and closes recovery state; failure/interruption remains pending/red; exhaustion remains stale/escalated until explicit operator rearm creates a new epoch without erasing the old one. | one sweep per obligation races an N-item provider scan; provider attempts miss setup failures; an all-exhausted frontier must not manufacture an empty sweep | Must | critical | multi-obligation and empty-implementation fixtures produce at most one sweep per round, one decision/count increment per selected obligation, and at most one provider attempt each; truth tables at 0, bound−1, bound plus duplicate-wake, all-exhausted, mixed eligible/exhausted, preflight, crash edges, restart, rearm, forgery, and infrastructure prove exact arithmetic, immutable prior epochs, zero provider preflight/attempt/implementation on all-exhausted, and no clean pending/exhausted result | Test + Failure injection + Demo | G1, J1, J3, F1, FR-15, UB-3, UB-4, prd:implement-now FR-46, FR-47, UB-20, UB-21, incident:I8 |

### op: supersede FR-7

Reason: the status fit criterion must prove pending and exhausted review obligations, not merely let the experience prose imply that the old task/proposal table covers them.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-17 | When the operator runs the status verb, the system shall render, per task/proposal await and configured-review obligation in the active scope: its state, what it waits on, wait age, a too-long flag, and the checker's last-check age; review rows shall additionally render the immutable revision, FR-15 `age_s`, provider review-attempt count, consumer recovery epoch/count/bound, last failure class, and an explicit exhausted/escalated state — read-only. | “quiet” must distinguish implementation wait, pending review, exhausted review, and a dead reader without the browser | Must | high | seeded waiting-on-review, CI-running, blocked, stale-wait, dead-checker, review-pending in each failure class, review-unknown, and review-exhausted fixtures render every required field and flag exactly the seeded stale/exhausted rows; the verb performs zero writes | Test + Demo | G2, J2, F2, FR-15, FR-16, UB-9 |

### op: add NR

Reason: adding a second frontier must not turn the one-sweep executor into a scheduler or let recovery rewrite succeeded implementation history.

| ID | Existing behavior protected | Why protected | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|
| NR-3 | `implement-now` remains one-shot and task implementation remains terminal once succeeded; the drive owns cadence and review-only recovery invokes only the executor's selected review frontier | keeps scheduling out of the executor and preserves green task truth | critical | inspection finds no timer/loop in the executor; the FR-16 matrix records zero task dispatches and byte-identical succeeded task terminals across every retry | Test + Inspection | FR-16, prd:implement-now FR-42, FR-46, FR-47, NR-10 |

### op: add UB

Reason: restart laundering is the paid-for failure that the combined readiness predicate must forbid explicitly.

| ID | If… (trigger) | Then the system shall… | User sees | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|---|
| UB-9 | a review attempt is killed, fails, produces unauthentic evidence, or remains pending when the next drive round has no ready implementation task | exact-read and retain the same obligation, schedule only consumer-bounded review-only recovery, preserve its last closed failure class, and remain non-clean through recovery exhaustion; never redispatch implementation or derive the reviewed revision from the current proposal head | implementation stays succeeded; review stays visibly pending/red with provider-attempt and consumer recovery counters/remedy, or exhausted/escalated | Must | critical | kill/restart, absent/malformed/forged artifact, factory/auth/infrastructure, moving-head, empty-frontier, and exhausted-bound fixtures retain one exact obligation, count every admitted review-only sweep, launch zero implementation, and never report clean until authenticated verification | Test + Failure injection | FR-15, FR-16, EVT-7, prd:implement-now FR-40, FR-42, FR-46, FR-47, UB-20, UB-21 |

### op: supersede UB-5

Reason: the stale-state behavior cites retired FR-7 and omits pending/exhausted review waits.

| ID | If… (trigger) | Then the system shall… | User sees | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|---|
| UB-10 | an implementation await, configured-review obligation, or the checker exceeds its too-long threshold — including pending review reaching its consumer recovery bound | surface it as a named non-resting defect in status and events; append immutable evidence before reporting exhaustion; an exhausted review stays pending/escalated until authenticated verification or explicit evidenced rearm | a typed stale or exhausted row with age, provider attempts, recovery epoch/count/bound, last failure class, evidence ref, and remedy | Must | high | seeded stale implementation await, dead checker, pending review, exhausted review, and rearm fixtures flag at the exact threshold, retain/replay evidence, and never render idle/clean | Test + Demo + Inspection | FR-12, FR-15..FR-17, EVT-5, EVT-7 |

### op: add UB

Reason: a whole-store failure cannot identify an obligation and therefore needs a closed non-launching outcome separate from per-obligation recovery.

| ID | If… (trigger) | Then the system shall… | User sees | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|---|
| UB-11 | the canonical review-obligation store or consumer recovery ledger is unavailable, unreadable, or structurally invalid as a whole | treat combined readiness as unknown/non-empty, emit EVT-8 naming the failed source, wait the reader's bounded backoff before a fresh exact read of both stores, launch neither implementation nor review from that unknown snapshot, and end the current driver round non-cleanly if either remains unknown | explicit `review state unknown` with source, failure kind, retry delay, and remedy; never “nothing to do” | Must | critical | unavailable, permission-denied, corrupt-envelope, unknown-field, and recovery fixtures for both sources each emit EVT-8, perform zero launch, exit non-cleanly while unknown, and resume ordinary derivation only after both exact-read successfully | Test + Failure injection | FR-15, EVT-8, UB-2 |

### op: supersede §5.4 Data

Reason: provider review attempts are not a driver retry budget, and exhaustion cannot be durable on the existing rotated local event stream. The data contract must own the consumer counter and the immutable exhaustion/rearm evidence without weakening the live throttle rules.

| Entity / field | Meaning | Source of truth | Retention | Integrity rules | Privacy class |
|---|---|---|---|---|---|
| throttle ledger — identity | write-budget state for one authenticated identity on one medium host, shared by every pyxis process on the machine | machine pyxis state dir, never a checkout | ephemeral only when no unexpired spend exists | absent starts empty only after the host barrier; unreadable starts conservative; every resolved write charges identity and host umbrella | internal |
| throttle ledger — host umbrella | machine-wide bound for all identities/unresolved writers on one medium host | machine pyxis state dir beside identity ledgers | same safe-disposal rule as identity ledger | every resolved write charges it in addition to identity; unresolved charges it alone; absent/unreadable follow the barrier rule | internal |
| cutover marker | one record per `(machine, medium-host)` proving the first-initialization barrier completed and no non-umbrella writer has written since | machine pyxis state dir under the umbrella lock | durable; deletion deliberately re-arms the barrier | conservative ledger durable before marker; crash leaves conservative state or absent marker; unreadable marker means absent | internal |
| canonical review obligations / provider attempts | immutable exact-green obligations and append-only provider attempts owned by implement-now | implement-now's canonical configured-review store | per its governing contract | exact-read only; this feature cannot rewrite backend, proposal, revision, attempt, failure class, or verified state | internal |
| review-recovery ledger (`review-recovery/v1`) | consumer-owned append log keyed by `(scope, review_obligation_id, recovery_epoch)`: source-owned `first_observed_at_utc`, initial frozen positive bound, one admission increment per selected review-only sweep, exhaustion, explicit operator rearm, and closure on authenticated verification | configured drive evidence git ref, outside disposable checkout/workspace state | immutable git history; retained with the drive evidence | append-first before action; reconstruction is closed-schema exact replay; first observation creates time/epoch/count 0/bound; later config/restart cannot change time/bound; admission appends count+1 before executor launch; each `(scope, obligation, epoch, count, action)` is idempotent and conflicting replay invalid; exhausted/rearmed entries carry attributable principal, reason, prior evidence ref, and stable content-derived ref; rearm creates a new epoch and never edits/deletes the exhausted one | internal |
| `review-wake-state/v1` | derived exact join of canonical obligation/provider state and review-recovery ledger, including consumer `first_observed_at_utc`/`age_s`, provider attempt count, recovery epoch/count/bound, failure, and eligibility | derived at read time; never independently stored | none | any missing/corrupt/unavailable source makes the relevant side unknown/non-empty; verified provider obligation closes but does not erase recovery evidence | internal |
| wake/check/recovery telemetry | EVT-1..EVT-8 routine observations | repo event stream | standing rotated/local event retention | append-only within the stream; additive schema evolution only. EVT-7 invoke/defer telemetry may remain ephemeral, but `action=exhausted` or `rearmed` fires only after the corresponding immutable recovery-ledger append and must carry its evidence ref | internal |
| read cursors | caller-held comments-since positions | consumer durable state | consumer-owned | reader persists no medium content | internal |

The throttle partition remains no finer than the platform's documented limit: identity ledgers sit inside the host umbrella, never beside it. A throttle ledger is disposable only before first hold on an already-cut-over host or after the longest window with no writer active; mid-window deletion is outside contract.

Every transition between umbrella-aware and legacy writers remains a cutover with a drain. Upgrade: quiesce all legacy writers, then pay the conservative first-initialization barrier under the host lock. Downgrade: quiesce umbrella writers, durably invalidate the marker, then wait per host until the latest live expiry—or a full longest window from confirmed quiescence when state is missing/unreadable—before a legacy write. Parallel generations are forbidden. Same-generation rollback leaves ledgers/marker in place; legacy rollback follows the downgrade drain. Legacy files may be removed only after the longest window. A crash during initialization must leave either conservative ledger state or an absent marker.

- **Migration** — existing throttle cutover/drain applies unchanged. Review-recovery state is born append-first on first observation of an obligation; no summary/event migration is accepted as source of truth.
- **Parallel-run / cutover** — throttle generations follow the rule above. For review recovery, one drive owns a scope; duplicate wake processes may read but the git append/idempotence key admits at most one count transition for a given epoch/count.
- **Rollback** — disabling configured review stops new recovery invocations but never deletes obligations or recovery evidence. A build that cannot interpret a live pending/exhausted ledger must report unknown/non-clean, not empty.
- **Decommission** — no automatic deletion; closure remains in git history with the drive.
- **Privacy check** — internal IDs, revision hashes, failure classes, times, and principals only; no prompt, code payload, raw artifact, token, or attribution secret enters the ledger/events.
- **Egress** — none new: git evidence uses the configured repository/code-host path; medium reads use existing authenticated APIs; NG-3 still forbids a hosted relay.

### op: supersede §5.5 Interfaces and integrations

Reason: the combined wake predicate needs a typed review-state input and a review-only executor mode; neither is present in the existing interface table.

| Interface | Direction | Purpose | Contract (format, ranges, timing) | Failure modes | Owner |
|---|---|---|---|---|---|
| state-read port (consumers: drive, argus, status verb) | in | batched proposal-state reads | typed batch request → typed per-proposal states incl. unknown; round-trip bound per FR-2; landed reports carry the medium's revision id and are marked medium-claimed (FR-13) | unknown-retry-later (FR-9); parse defect (UB-2) | this feature |
| review-state port (consumers: drive, status verb) | in | exact durable configured-review readiness | closed `review-wake-state/v1`; exact join of provider obligation/attempt state with consumer `first_observed_at_utc`/`age_s` and recovery epoch/count/frozen-bound; verified is absent; whole-store unknown is readiness, not empty | unavailable/corrupt/invalid on either side → UB-11 typed unknown, bounded backoff, non-clean round, zero launch | implement-now owns obligation state; this feature owns recovery state and the joined port |
| wake delivery (consumer-registered) | out | schedule-now signals | closed schema: await id + transition kind, nothing else (FR-10); review transitions identify only the obligation await and transition | duplicate delivery harmless (UB-4); loss covered by next exact read | this feature |
| review-only executor invocation | out | reconcile eligible pending review without task implementation | exactly one one-shot invocation per durable driver round carrying `drive_round_id` plus a closed selected-obligation ID set; provider scans only that set and admits at most one attempt per member; zero implementation operations | setup/attempt failure remains pending and consumes consumer recovery admission; exhausted obligations excluded; unknown state launches nothing | implement-now provider + this feature consumer |
| GitHub API (GraphQL/REST) | out | the metered medium's reads + throttled writes | within published budgets; batched per FR-2; per-context checks per FR-3 | 429/5xx → FR-9/UB-1; drift → UB-2 | GitHub adapter |
| Carina store (local files) | out | the offline medium's reads | file reads + watch; no network | fs errors → FR-9's unknown | Carina adapter |
| push-slot (reserved, empty in v1) | in | future schedule-now hints | FR-11's hint-only contract | dead adapter = timer behavior | WR-1's feature |

### op: add EVT

Reason: review-only recovery needs a durable decision record distinguishable from implementation-triggered sweeps and ordinary proposal wakes.

| ID | Event name | Fires when | Properties (typed) | Source | Feeds |
|---|---|---|---|---|---|
| EVT-7 | `review_recovery_decision` | the driver evaluates one pending review obligation for a durable round | `drive_round_id:string`, `review_obligation_id:string`, `task_id:string`, `proposal_ref:string`, `revision:string`, `action:enum{invoke_review_only,defer,exhausted,rearmed}`, `provider_attempt_count:int`, `recovery_epoch:string`, `recovery_count:int`, `recovery_bound:int`, `last_failure_class:enum{artifact_unverified,artifact_untrusted,infrastructure}?`, `evidence_ref:string?`, `reason:string?`; `evidence_ref` is required for exhausted/rearmed | wake consumer / drive | M1, FR-15, FR-16, UB-9 |

### op: add EVT

Reason: whole-store unknown cannot truthfully populate EVT-7's obligation identity and needs its own terminal observation.

| ID | Event name | Fires when | Properties (typed) | Source | Feeds |
|---|---|---|---|---|---|
| EVT-8 | `review_state_unknown` | a combined-frontier derivation cannot exact-read a valid provider obligation store or consumer recovery ledger | `drive_round_id:string`, `scope:string`, `source:enum{obligation_store,recovery_ledger}`, `kind:enum{unavailable,permission_denied,invalid_envelope,unknown_field}`, `action:enum{defer}`, `retry_after_ms:int`, `reason:string` | review-state reader / drive | M1, FR-15, UB-11 |

### op: add DOD

Reason: the recovery and non-laundering path needs blocking evidence independent of ordinary proposal wake coverage.

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-11 | Pending configured-review obligations participate in the same readiness decision as implementation work and survive kill/restart, unavailable/corrupt reads, unauthentic artifacts, moving heads, duplicate wakes, setup failure, and an empty implementation frontier; one durable driver round issues at most one executor sweep over its non-empty selected frontier, consumer recovery accounting includes every admitted sweep, the provider admits at most one attempt per selected obligation, and zero implementation work occurs. All-exhausted and whole-store-unknown rounds persist evidence/events and launch nothing. Only authenticated verification clears an obligation; exhausted/rearmed epochs retain immutable evidence. | FR-15, FR-16, NR-3, UB-9, UB-11, EVT-7, EVT-8 | Test + Failure injection + Demo + Inspection | combined-frontier and 0/bound−1/bound truth tables; injected-clock first-observed age evidence; multi-obligation/duplicate-wake/mixed fixture proving one sweep; all-exhausted and unknown zero-preflight/zero-launch traces; setup-failure/crash accounting; kill/restart/forgery/moving-head/exhaustion/rearm; immutable replay; zero-implementation, EVT wire, status, and exit assertions | yes |

### op: supersede DOD-1

Reason: the all-Must adapter gate currently stops at FR-14 and would not execute the new critical readiness, recovery, and status fit criteria.

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-12 | Every live Must FR passes its fit criterion on every applicable adapter and consumer composition — the metered medium alone for FR-8/FR-14, both media for medium-neutral proposal reads, and the canonical review-state provider plus each supported review backend for FR-15..FR-17. | FR-1..FR-6, FR-8..FR-17 | Test + Failure injection + Demo | conformance and drill run indexed by every live Must row, including combined readiness, review-only recovery, and pending/unknown/exhausted status fixtures | yes |

### op: supersede DOD-3

Reason: the failure-drill gate omits both the live downgrade failure and the new review-laundering failure.

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-13 | The complete live unwanted-behavior matrix passes, including downgrade drain, durable review recovery/non-laundering, and whole-store unknown. | UB-1..UB-4, UB-7..UB-11 | Test + Failure injection + Inspection | one indexed result per live UB row; UB-8's drain oracle, UB-9's recovery traces, UB-10's stale/exhausted status, and UB-11's zero-launch unknown traces included | yes |

### op: supersede DOD-2

Reason: the non-regression gate must include the new review-only isolation row.

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-15 | No live non-regression row is broken; review recovery remains one-shot and preserves succeeded implementation terminals. | NR-1..NR-3 | Test + Inspection | existing executor suites plus the zero-implementation review-only trace and byte-identical succeeded terminals | yes |

### op: supersede DOD-6

Reason: the instrumentation gate must include the new recovery decision event.

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-14 | Every live §5.7 event is observed with its exact typed properties during conformance/proof, including EVT-4's ledger/depth/wait/deadline, EVT-7's round/obligation/provider-attempt/recovery/action/failure join and required immutable evidence refs for exhausted/rearmed, and EVT-8's whole-store unknown result. | EVT-1..EVT-8, §5.4 | Inspection + Test | event-log wire extracts, schema assertions for all eight events, git-retained exhaustion/rearm records, replay/idempotence/conflict fixtures, and evidence-ref joins | yes |

### op: supersede §6.1 Phasing and milestones

Reason: recovery evidence must block Build and the proof run must demonstrate a review-only transition.

| Phase | Purpose | Scope | Exit criterion |
|---|---|---|---|
| Build | the whole v1 (everything-or-nothing, D-8) | §4.2 “In” | DOD-7, DOD-10..DOD-15 green |
| Proof | validate the promise on reality | the 6-task dogfood feature, both backends, including one killed review recovered with no implementation work | DOD-4..5 and DOD-11..DOD-15 green — the D-6 numbers and non-laundering invariant hold |
| Ship | release via the normal channel | — | §8 walked; §6.6 launch gate satisfied; introducing-PR review signed |

### op: supersede §6.2 Dependencies

Reason: the canonical obligation state and review-only executor interface are new hard dependencies rather than optional consumers.

| Dependency | Type (team / system / vendor / approval) | Needed by | Status | Owner |
|---|---|---|---|---|
| executor ports surface (CodeHost/WorkTracker homes for proposal reads) | system (design decision) | Build | open — superseding state-reader design owns placement | @mdarabi |
| prd:implement-now FR-40..FR-47 and its canonical review-obligation store / selected-frontier review-only invocation | system contract + provider | Build | approved contract; implementation remains blocked on the sibling impl-review capability below, after which provider and consumer designs/implementations activate as one unit | @mdarabi |
| prd:impl-review host-side persistence-adapter authority plus exactly-one receipt-returning publication | sibling contract + design | before provider/consumer Build | blocked — the live sibling contract/design must explicitly authorize its host adapter to perform FR-6 for the reviewer program and expose exactly-one publication returning the native receipt; absence or ambiguity fails closed per prd:implement-now FR-41 | @mdarabi |
| superseding state-reader and wake-watch designs | system design | Build | required before configured review activates; stale predecessors may not ship against the new provider | @mdarabi |
| a frozen 6-task plan to drive | artifact | Proof | plentiful | @mdarabi |
| feature-drive and argus consumption | system consumers | after ship | consume the same combined readiness contract | @mdarabi |

### op: supersede §6.3 Risks, assumptions, and rabbit holes

Reason: the second frontier adds a silent-laundering risk and invalidates the old assumption that proposal-state awaits are the complete durable frontier.

| # | Risk / assumption / rabbit hole | Likelihood | Impact | Mitigation or test | Owner |
|---|---|---|---|---|---|
| R1 | the checker dies silently and self-drive quietly reverts to operator-clocked | M | H | FR-12 + UB-10; kill drill blocks | @mdarabi |
| R2 | platform API drift turns a read silently wrong or empty | M | H | UB-2 parse-or-defect; conformance fixtures | @mdarabi |
| R3 | write bursts trip platform limits despite the throttle | L | M | FR-8 burst fixture; M3 halts on any trip | @mdarabi |
| R4 | the port shape fits the shipped media but not a third | L | M | NFR-5 conformance kit | @mdarabi |
| R5 | comment-read edge cases balloon the build | M | M | schedule absorbs it; scope is not silently dropped | @mdarabi |
| R6 | assumption: consumers re-derive state on every wake, so duplicate/lost wakes are harmless | — | H if false | UB-3/UB-4 plus combined exact-read fixtures | @mdarabi |
| R7 | a failed review disappears when a later round sees no implementation task, or full-sweep retry recreates work | M | H | FR-15/FR-16 closed combined predicate, NR-3, UB-9, and blocking DOD-11 | @mdarabi |

### op: supersede §6.4 Release criteria

Reason: reliability and supportability must include durable review-only recovery and exhaustion, not only proposal-state failure modes.

| Lens | Criterion | Evaluated by |
|---|---|---|
| Functionality | every Must FR green on every applicable adapter and the combined implementation/review readiness truth table is complete | CI (conformance + suites) |
| Usability | DOD-5's quiet-state diagnosis plus pending/exhausted review rows are correctly classified from status alone | operator drill |
| Reliability | §5.2 drills, DOD-10 cutover suite, FR-14 mixed-writer fixture, and DOD-11 recovery/non-laundering matrix all pass | failure injection + run report |
| Performance | M2 latency bounds hold at the proof run | event-log analysis |
| Supportability | NFR-3 degraded-mode map includes review unknown, pending, three failure classes, and exhausted remedies | support review |

Release is permitted only by §3.3's single pre-registered ship predicate; this table supplies its lens evidence and does not define an alternate weaker decision rule.

### op: supersede §7.1 FAQ

Reason: the live FAQ cites retired UB-5 and claims the feature does not touch the executor, which is false once the wake consumer owns review-only invocation.

**Customer**

- **Q:** Does my machine need a server, tunnel, or new account? — **A:** No. v1 uses the same authenticated proposal/review stores and local timer; nothing listens on the network.
- **Q:** What happens when I close my laptop? — **A:** The drive pauses. On restart it exact-replays proposal, obligation, and recovery state; pending review remains pending and does not redispatch implementation.
- **Q:** Will this consume excessive API quota? — **A:** The measured proposal read remains one point per whole-feature check; review recovery is bounded in the consumer and uses the configured provider's existing interfaces.
- **Q:** Can it merge or approve things? — **A:** No. It observes and schedules one-shot work only.

**Internal**

- **Q:** Why is review recovery here rather than a loop in implement-now? — **A:** Implement-now remains one-shot (NR-1/NR-3); the drive owns cadence, durable recovery admission, and exhaustion, while the provider owns exact obligations and attempts.
- **Q:** Why not invoke once per obligation? — **A:** Each provider invocation scans its selected frontier; one invocation per obligation would create N competing scans. FR-16 makes one selected-frontier sweep per round and one provider admission per selected obligation.
- **Q:** Why are provider attempts and recovery counts separate? — **A:** Factory/auth/preflight failure can spend a driver invocation without creating a provider attempt. Incident I8's lesson puts the durable bound where the spend occurs.
- **Q:** What are the four primary failure classes? — **A:** dead/stale checking (FR-12/UB-10), parse or store unknown (UB-2/UB-11), platform budget breach (FR-8/FR-9), and pending review laundering/unbounded recovery (UB-9/FR-16).

### op: supersede §7.2 Open questions

Reason: Q1's dependency citation must follow the live status requirement rather than its retired predecessor.

| # | Question | Owner | Needed by | Status |
|---|---|---|---|---|
| Q1 | The ⚠ thresholds: fixed defaults per wait kind (implementation review, CI, claim, configured-review obligation) vs learned from repo history — and their values | @mdarabi | technical design (blocks FR-17's threshold table, not the recovery state machine) | open (brainstorm:self-drive-wake#G-1; WR-2 holds the learned path) |
| Q2 | Sequencing and citation shape between this capability's design and the argus wake design | @mdarabi | design-stage integration | open (brainstorm:self-drive-wake#G-2) |

### op: supersede §8 Contract readiness — the executability gate

Reason: the gate must count the new Must/NR/UB/event/DoD rows and make the provider/consumer supersession a launch blocker.

| Check | Pass? | Notes |
|---|---|---|
| Problem basis accepted — §1 problem accepted; §2 problem gate signed | ☐ | pending — introducing PR operator review |
| Zero solution-blocking TBDs or open questions (§7.2) | ✅ | Q1/Q2 do not change the closed recovery behavior |
| Every Must FR has a fit criterion and verification method | ✅ | FR-1..FR-6, FR-8..FR-17; FR-15..FR-17 have exact-read, recovery, and status matrices |
| Every requirement row carries a risk tier; high/critical rows have a safety plan | ✅ | FR-15, FR-16, NR-3, UB-9 are critical and DOD-11-backed |
| Non-goals drawn; non-regression rows named | ✅ | NG-1..6, WR-1..2; NR-1..NR-3 |
| Every primary job is covered | ✅ | J1/J3 include FR-15/FR-16; J2 includes review status through §4.3/F2 |
| Every key flow has functional, failure, and non-regression coverage | ✅ | F1→FR-15/FR-16/UB-9/NR-3; F2→FR-15/FR-17/UB-9/UB-10 |
| Every primary metric has instrumentation or an explicit plan | ✅ | M1 additionally reads EVT-7; M2..M4 remain EVT-1..EVT-6 + drills |
| Every blocking DoD row traces to a requirement, guardrail, release criterion, or rubric | ✅ | DOD-4..DOD-5, DOD-7..DOD-15; DOD-11..DOD-15 trace the live recovery, Must, UB, event, and NR contracts |
| Risks and assumptions have owners | ✅ | R1..R7 owned |
| Human-judgment criteria name an owner and rubric | ✅ | DOD-5 remains @mdarabi's scenario rubric |
| DoD complete: every blocking row sourced/evidenced; every n/a explained | ✅ | DOD-11..DOD-14 close recovery, all-Must, all-UB, and all-event evidence; DOD-8 remains explained n/a |
| Sibling impl-review contract/design supplies explicit host-adapter performer authority and exactly-one receipt-returning publication | ☐ | pending — §6.2 and prd:implement-now FR-41 block implementation and activation while the capability is absent or ambiguous |
| Provider and consumer designs supersede stale implement-now/state-reader/wake-watch contracts together before activation | ☐ | pending after the sibling capability above — required by §6.2 and DOD-11; no partial activation |
| Single ship predicate satisfied | ☐ | pending — §3.3 requires M1–M3, every guardrail, and DOD-11..DOD-15 on one proof composition; M1–M3 alone cannot ship |
| Launch gate satisfied — §6.4 green and every owned §6.6 row checked | ☐ | pending at ship |
