# Wake watch v3 — combined implementation and review-obligation drive — technical design

| Field | Value |
|---|---|
| **Status** | Draft → Approved *(merge requires Approved; merge freezes this document)* |
| **Owner** | @mdarabi |
| **Approver(s)** | @mdarabi (operator) — user-facing scheduler, recovery budget, and immutable escalation surface |
| **Date** | 2026-08-06 |
| **Relevant-when** | driving a frozen plan; deciding whether implementation or configured review is ready; recovering a pending review after restart/setup failure; changing review retry bounds, durable rounds, status, exhaustion/rearm evidence, or the watch/status verbs |
| **Scope** | Exact-incorporates wake-watch-v2's proposal wake loop, cadence, status, staleness, event-file, and push-slot behavior; adds the configured-review frontier, durable driver rounds, consumer-bounded selected recovery, immutable exhaustion/rearm evidence, and combined status. Not proposal/review reads (state-reader-v5), review attempt execution or verification (implement-now successor), or feature-lifecycle ownership. |
| **Tracking** | issue:1029 · PR #1034 |
| **Builds-on** | docs/tech-design/state-reader-v5.md · docs/tech-design/wake-channel-v1.md · docs/tech-design/implement-now-authenticated-review-obligations.md |
| **Supersedes** | wake-watch-v2 |
| **Serves** | prd:self-drive-wake/FR-4 · prd:self-drive-wake/FR-5 · prd:self-drive-wake/FR-6 · prd:self-drive-wake/FR-10 · prd:self-drive-wake/FR-11 · prd:self-drive-wake/FR-12 · prd:self-drive-wake/FR-15 · prd:self-drive-wake/FR-16 · prd:self-drive-wake/FR-17 · prd:self-drive-wake/UB-1 · prd:self-drive-wake/UB-3 · prd:self-drive-wake/UB-4 · prd:self-drive-wake/UB-9 · prd:self-drive-wake/UB-10 · prd:self-drive-wake/UB-11 · prd:self-drive-wake/NR-1 · prd:self-drive-wake/NR-3 · prd:self-drive-wake/NFR-2 · prd:self-drive-wake/NFR-3 · prd:self-drive-wake/NFR-4 · prd:self-drive-wake/EVT-1 · prd:self-drive-wake/EVT-2 · prd:self-drive-wake/EVT-5 · prd:self-drive-wake/EVT-6 · prd:self-drive-wake/EVT-7 |
| **Security/privacy impact** | **yes** — appends attributable internal recovery decisions to the configured evidence ref and invokes a privileged local executor entry; the wake input remains instruction-free and no new network listener, credential, prompt, code, or artifact body is admitted · reviewer: @mdarabi |
| **Template version** | tech-design-template/v1 |

## 0. Read to rule — the human reviewer's section (~4 min)

*The body is the contract; this section only routes attention; machines ignore it.*

| # | The call | Could have gone | If you disagree |
|---|---|---|---|
| 1 | **One combined readiness predicate** — implementation and pending review both prevent idle/clean | read only ready tasks | a later empty implementation frontier launders review failure → §3.1, #DINV-11 |
| 2 | **The drive, not the provider, spends recovery** — default bound 3, frozen per epoch | use provider attempt count | factory/auth failures create no provider attempt and could loop forever → §3.3, incident:I8 |
| 3 | **One durable round invokes one selected frontier** | one sweep per obligation or a provider-wide scan | duplicate scans race, or unselected/exhausted work is spent → §3.2, #DINV-13 |
| 4 | **Admission is append-first** | increment after launch | a crash after launch but before accounting gets free retries → §3.3, #DINV-14 |
| 5 | **All-exhausted and unknown rounds launch nothing and remain red** | issue an empty sweep | exit 0 launders durable pending work → §3.4, #DINV-15 |
| 6 | **Only exhaustion/rearm crosses into immutable git evidence** | persist all ticks, or only local JSONL | avoid both noisy history and disposable escalation → §3.5, AP-28 |

**Sanity-check the proof** — duplicate wakes coalesce to one round; N eligible obligations produce one selected invocation and N admission records; setup failure consumes N admissions and produces zero provider attempts; count = bound excludes the obligation; an all-exhausted or unknown round performs zero preflight/launch; review-only traces contain zero implementation operations.

**The author's open questions** — none.

## 1. Problem

Wake-watch-v2 derives only proposal/task awaits. Configured review begins after a task's implementation result is terminal, so a failed/interrupted review can remain durable while the implementation frontier is empty. Re-running a full sweep risks redispatch or a provider-wide scan; trusting the last summary lets the next empty run erase the failure. The consumer must join both frontiers, bound its own invocations, and preserve terminal escalation without turning the one-shot executor into a scheduler.

## 2. Decision and carried composition

**Decision:** Wake-watch-v2 is incorporated as a frozen conformance component at Git object `52d724929450e5bd9db20b36d20c4543af129010`. Its in-process context-owned loop, proposal/task await derivation, adapter-advised cadence, instruction-free wake grammar, state-edge coalescing, idle-zero behavior for its original frontier, status/staleness semantics, local rotated JSONL events, reserved push slot, `DINV-1..DINV-9`, risks, and `A-1..A-13` remain binding and executable. A successor manifest maps every carried Serves/DINV/A anchor to the same named check/digest; omission, mutation, or favorable subset fails A-1.

V3 replaces only “proposal/task frontier empty means idle” with the combined predicate below and extends status/recovery. The executor remains one-shot and owns no timer.

## 3. Combined drive

### 3.1 Exact combined readiness

At each derivation boundary the loop exact-reads the existing plan/task/proposal frontier and state-reader-v5's `review-wake-state/v1`. `CombinedReadiness` is known-empty only when both are valid and empty. Ready implementation or `handoff_repair` schedules the ordinary one-shot entry; that entry exact-repairs handoff before admitting its newly repaired review and never touches pre-existing review-pending rows. Eligible review-pending schedules the selected review-only entry. Both may run within one durable round, but they are distinct entries and review never occupies implementation claim/width. Unknown on either source is non-empty/non-clean and authorizes neither action from that snapshot.

The loop re-derives after every settled entry. No wake payload, prior summary, in-memory queue, proposal current head, or task success event is an authority for a review selection.

### 3.2 Durable driver round and selected request

A fresh cryptographically random `drive_round_id` from an injected `RoundIDSource` identifies one exact-read → persisted-decision → settled-entry/no-launch cycle. Production uses OS entropy; tests use a deterministic source, and empty/duplicate source output fails before persistence or launch. Timer/push/filesystem hints arriving before settlement coalesce into it. The drive selects every in-scope `ProviderState=review_pending` obligation with `recovery_count < recovery_bound`, ordered by stable obligation ID. `handoff_pending` is excluded and routed only through §3.1's normal handoff repair. If the selected set is non-empty, the drive append-first admits every member and calls exactly one review-only executor entry with:

```go
type ReviewRecoveryRequest struct {
	DriveRoundID string
	ObligationIDs []string
}
```

The request is closed, non-empty, duplicate-free, and scope-bound. The provider exact-reads only these identities, admits at most one attempt per selected obligation, and performs no implementation route, claim, worker, proposal, or terminal rewrite. Any invalid selection or provider mismatch makes the round non-clean; it never widens the set.

### 3.3 Consumer recovery policy

`wake.review_recovery_bound` is a closed integer in `[1,100]`, default `3`. State-reader-v5 freezes the value in each new recovery epoch. Each selected admission appends `recovery_count+1` under an idempotence/CAS key before invocation; a crash before append consumes nothing, while a crash after append consumes one even if the provider creates no attempt. Factory/auth/preflight/setup failure therefore counts exactly like an admitted attempt without falsifying `provider_attempt_count`.

Only authenticated provider verification closes an obligation. A failure or interruption remains pending. At `count == bound` it is exhausted and excluded. Explicit operator rearm requires a principal and reason, appends a linked new epoch under the then-current bound, and never edits the old epoch/evidence.

### 3.4 No-launch terminals

If pending obligations exist but all are exhausted, the drive append/exact-reads immutable exhaustion evidence (idempotently), emits/replays EVT-7 `action=exhausted`, launches no executor or preflight, renders non-clean status, and exits non-zero for the round. If either joined store is unknown, state-reader-v5 emits EVT-8, the drive applies bounded backoff and one fresh exact read; if still unknown it launches nothing and exits non-zero. Neither condition can produce the v2 completed/idle terminal.

### 3.5 Observability and evidence

Routine EVT-1/2/5/6 checks/wakes/sweeps and EVT-7 invoke/defer remain in the existing rotated local JSONL stream. EVT-7 `exhausted|rearmed` is emitted only after the consumer recovery ledger's immutable append and carries its stable `evidence_ref`. One event encoder owns the exact live wire:

- EVT-7 fields and action/failure enums are byte-golden against the PRD;
- every selected `invoke_review_only` row carries round, obligation/task/proposal/revision, provider attempts, epoch/count/bound, and prior failure;
- exhaustion/rearm carries attributable reason and evidence reference;
- no prompt, output, code, artifact body, credential, or native control is logged.

EVT-6 marks the selected invocation `initiator=drive-wake` and its scope; the provider's EVT-15 terminals carry the same `drive_round_id`.

### 3.6 Security & privacy

The v2 wake input remains a closed schedule hint and accepts no network instruction. Immutable recovery entries are written only through the configured evidence adapter under the caller's existing authority; attributable principal and reason are data, never commands. The drive passes selected stable IDs to the local executor API and mints no reviewer or code-host credential. Closed request parsing, scope checks, append CAS, log redaction, and filesystem/ref permissions are mandatory; untrusted prompt, code, comment, artifact body, native controls, and credentials never enter recovery evidence.

### 3.7 Status

The read-only status verb calls state-reader-v5's `SnapshotStatus`, never `DeriveReadiness`, and extends v2's task/proposal rows with one row per review obligation: immutable proposal/revision/backend, `age_s`, provider attempt count, recovery epoch/count/bound, last failure class, and `eligible|pending|exhausted|unknown|unobserved`. Exhausted is an escalation with evidence ref and rearm remedy; unknown names the failed source/remedy. A healthy provider row not yet observed by a drive renders `unobserved` and literal `unknown` for the unavailable recovery fields; status never invents an epoch/bound. Accessibility remains non-color-only. Rendering performs no append, initialization, admission, or rearm.

### 3.8 Invariants

- **DINV-10** — V2 behavior is exact-object incorporated and cannot be shadowed by a v3-only implementation.
- **DINV-11** — Idle/clean is authorized only by one valid exact read proving both frontiers empty.
- **DINV-12** — A wake only schedules exact re-derivation; it never supplies review identity, revision, or authority.
- **DINV-13** — One durable round submits at most one closed selected review-only request; the provider touches no unselected obligation.
- **DINV-14** — Every selected member's consumer count is durably incremented before invocation; provider attempt count never substitutes for it.
- **DINV-15** — Exhausted/unknown stays non-clean and authorizes zero executor setup or launch.
- **DINV-16** — Review-only execution performs zero implementation operations and cannot change succeeded implementation truth.
- **DINV-17** — Exhaustion/rearm becomes observable only after immutable attributable evidence exists.
- **DINV-18** — Status is a pure snapshot; first-observation initialization belongs only to drive readiness derivation.
- **DINV-19** — Handoff repair is normal executor work before review eligibility; it never consumes a recovery admission or enters a review-only request.

### 3.9 Concurrency and cancellation

The existing goroutine-owned loop remains the sole round owner. Duplicate hints are coalesced while a round is active. Cancellation stops waiting and the active executor entry through context; append-first admissions remain spent and provider obligations remain pending. Two watch processes may derive the same selection: recovery-ledger CAS admits each `(epoch,count)` once; a loser re-derives rather than invoking. No goroutine outlives the verb.

## 4. Alternatives considered

| Alternative | Why rejected |
|---|---|
| Retry every pending obligation in a normal full sweep | permits implementation work and provider-wide scans; violates NR-3 |
| One invocation per obligation | N competing scans and races instead of one selected frontier |
| Increment after provider attempt | setup failure and crash-before-attempt become free infinite retries |
| Treat exhausted as empty | launders a required but unfinished review into success |
| Put the loop inside implement-now | violates the one-shot executor boundary |

## 5. Risks

| ID | Risk | Impact | Mitigation | Verification |
|---|---|---|---|---|
| R9 | two watchers select the same count | duplicate spend/attempt | append CAS; loser re-derives | concurrent-watch fixture |
| R10 | selected obligation verifies between read and invocation | stale launch | provider exact-read rejects verified/mismatched member before setup; drive re-derives | race fixture |
| R11 | evidence ref append fails | exhaustion disappears | no exhausted event/status; round non-clean | failure injection |
| R12 | normal and review-only work starve each other | delayed progress | separate entries/capacity; round trace and bounded queues | saturation fixture |
| R13 | config shrinks bound mid-epoch | retroactive exhaustion | bound frozen at epoch creation | config-change fixture |

## 6. Acceptance

| ID | Acceptance criterion | Verify | Required evidence | Traces to |
|---|---|---|---|---|
| A-1 | The closed `internal/wake/testdata/wake-watch-v3-carry-forward.json` manifest pins wake-watch-v2 object `52d724929450e5bd9db20b36d20c4543af129010` and maps every v2 Serves/DINV/A anchor to `{package, check_name, fixture_digest}`. A Go conformance test parses closed, rejects duplicate/missing/unknown/favorable-subset entries, verifies the frozen blob and registered checks, then runs the carried suites unchanged. | Test + Inspection | carry-forward manifest + negative corpus | #DINV-10 |
| A-2 | Exact combined-frontier truth tables cover empty, implementation-only, handoff-repair, review-only, both, partial/unknown, pending, and exhausted; only valid double-empty is idle/clean, and handoff repair consumes no recovery admission before exact EVT-10 repair. | Test | closed readiness table | prd:self-drive-wake/FR-15 · prd:self-drive-wake/UB-11 · #DINV-11 · #DINV-19 |
| A-3 | N eligible obligations plus duplicate hints produce one durable round, N append-first admissions, one selected review-only invocation, at most one provider attempt each, and zero implementation work; mixed exhausted members are not touched. | Test + Failure injection | selection/cardinality/access trace | prd:self-drive-wake/FR-16 · prd:self-drive-wake/NR-3 · #DINV-13..#DINV-16 |
| A-4 | Injected crashes before/after admission, factory/auth/preflight failure, interruption, restart, and two concurrent watchers produce exact consumer/provider arithmetic and never a free retry or duplicate invocation. | Test + Failure injection | crash/concurrency truth table | prd:self-drive-wake/FR-16 · prd:self-drive-wake/UB-9 · #DINV-14 |
| A-5 | Count 0, bound−1, bound, all-exhausted, config-change, and explicit-rearm fixtures preserve frozen epochs; exhausted/rearmed evidence replays from a fresh clone; exhausted/unknown performs zero executor setup/launch and exits non-zero. | Test + Inspection | boundary/evidence/zero-launch traces | prd:self-drive-wake/UB-10 · prd:self-drive-wake/UB-11 · prd:self-drive-wake/EVT-7 · #DINV-15 · #DINV-17 |
| A-6 | Status fixtures render every FR-17 review field and distinguish all three provider failure classes, unknown, unobserved, exhausted, and dead checker; an append spy proves the command performs no writes. | Test + Demo | status goldens/write trace | prd:self-drive-wake/FR-17 · #DINV-18 |
| A-7 | EVT-7/EVT-8 and joined provider EVT-15 wire goldens carry exact enums/fields, round and evidence joins, and no undeclared data; every blocking DOD-11..DOD-15 row has indexed evidence from the same composition. | Test + Inspection | event schema corpus + DoD index | prd:self-drive-wake/EVT-7 · prd:self-drive-wake/EVT-8 |

## 7. Open questions

None. Changing recovery ownership, durable-round identity, the default/bounds, selected request grammar, evidence durability, or combined-idle predicate requires a successor design (and a PRD amendment when behavior changes).

## Principles

- **AP-1/AP-3 — honored.** Consumer-defined ports preserve the executor/reader/wake adapter boundaries.
- **AP-2/AP-15 — honored.** Wake hints carry no instruction; only exact fresh reads authorize actions.
- **AP-5/AP-6 — honored.** Closed schemas and stable identities join every decision and terminal.
- **AP-10/AP-11 — honored.** One combined predicate and one consumer counter authority.
- **AP-16/AP-17 — honored.** Bounded recovery and fail-closed unknown/exhausted behavior prevent hidden spend or false completion (incident:I8).
- **AP-28 — honored.** Durable evidence is reserved for improvement-bearing exhaustion/rearm; routine telemetry remains ephemeral.
- **RW-5 — honored.** The successor changes the review-recovery composition while exact-incorporating v2.
- **Deviation:** none.
