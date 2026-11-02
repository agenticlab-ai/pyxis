# State reader v5 — proposal state plus durable review-readiness joins — technical design

| Field | Value |
|---|---|
| **Status** | Draft → Approved *(merge requires Approved; merge freezes this document)* |
| **Owner** | @mdarabi |
| **Approver(s)** | @mdarabi (operator) — platform-contract and durable-read surface |
| **Date** | 2026-08-06 |
| **Relevant-when** | reading proposal/check/comment state; enforcing the shared write budget; deriving configured-review readiness, age, failure, and recovery counters; rendering review pending/unknown/exhausted; triaging a drive that became clean while review remained pending |
| **Scope** | Incorporates state-reader-v4's proposal-state port, GitHub/Carina adapters, write-budget ledger pair/cutover, and conformance kit unchanged; adds the exact join between implement-now's canonical review obligations and the drive-owned recovery ledger. Not the cadence or decision to invoke (wake-watch-v3), review attempts (implement-now successor), or review artifact contents. |
| **Tracking** | issue:1029 · PR #1034 |
| **Builds-on** | docs/tech-design/implement-now-authenticated-review-obligations.md · docs/tech-design/local-work-tracker-v2.md |
| **Supersedes** | state-reader-v4 |
| **Serves** | prd:self-drive-wake/FR-1 · prd:self-drive-wake/FR-2 · prd:self-drive-wake/FR-3 · prd:self-drive-wake/FR-8 · prd:self-drive-wake/FR-9 · prd:self-drive-wake/FR-13 · prd:self-drive-wake/FR-14 · prd:self-drive-wake/FR-15 · prd:self-drive-wake/FR-17 · prd:self-drive-wake/UB-1 · prd:self-drive-wake/UB-2 · prd:self-drive-wake/UB-7 · prd:self-drive-wake/UB-11 · prd:self-drive-wake/NR-2 · prd:self-drive-wake/NFR-1 · prd:self-drive-wake/NFR-5 · prd:self-drive-wake/EVT-4 · prd:self-drive-wake/EVT-8 |
| **Security/privacy impact** | **yes** — carries forward the identity/host write-budget state and adds immutable recovery evidence containing internal proposal/revision/principal identifiers; no prompt, code payload, artifact body, credential, or attribution secret is admitted · reviewer: @mdarabi |
| **Template version** | tech-design-template/v1 |

## 0. Read to rule — the human reviewer's section (~4 min)

*The body is the contract; this section only routes attention; machines ignore it.*

| # | The call | Could have gone | If you disagree |
|---|---|---|---|
| 1 | **State-reader-v4 is incorporated exactly, not paraphrased away** | re-open the proven proposal/budget design | the successor creates accidental regressions unrelated to review recovery → §2, A-1 |
| 2 | **Review readiness exact-joins two authorities** — provider obligations/attempts plus consumer recovery evidence | read EVT-16 or a mutable summary | a later empty summary launders durable pending work → §3.2, #DINV-15 |
| 3 | **Provider attempts and consumer recovery counts remain distinct** | use attempt count as retry budget | setup failure spends a drive round without an attempt and loops unbounded → §3.3, incident:I8 |
| 4 | **Age begins at append-first consumer observation** | invent provider creation time or use git commit time | the consumer silently extends another schema and thresholds depend on incidental metadata → §3.3, #DINV-17 |
| 5 | **Either store unknown means readiness unknown/non-empty** | unknown ⇒ zero pending | corruption becomes “nothing to do” → §3.4, #DINV-18 |
| 6 | **Exhaustion/rearm is immutable git evidence; routine telemetry stays local** | persist every tick or persist only rotated JSONL | either noise floods git or the disposition disappears → §3.3, AP-28 |

**Sanity-check the proof** — v4's 50-proposal/≤2-round-trip and nine-writer budget proofs remain byte-identical; every joined row has one obligation, one recovery epoch/count/bound, exact `age_s`; provider/recovery corruption emits EVT-8 and zero launch; exhausted/rearmed evidence replays identically from a fresh clone.

**The author's open questions** — none.

## 1. Problem

State-reader-v4 supplies exact proposal state and the metered write guard, but configured review creates a second durable frontier after implementation is already green. Implement-now owns exact obligations and provider attempts; the drive owns how many review-only invocations it spends and whether that budget is exhausted. Neither store alone can answer whether the drive is ready, unknown, eligible, or exhausted. Reading a run summary is insufficient because a later empty sweep can overwrite the observation while the obligation remains pending.

## 2. Decision and carried composition

**Decision:** State-reader-v4 is incorporated as a frozen conformance component at Git object `9a75b181b550d148b65fb7fd67d1d21558266e7f`. Its reader/budget decisions, port shapes, adapter behavior, storage/cutover protocol, all `DINV-1..DINV-13`, risks `R1..R8`, and acceptance `A-1..A-21` remain binding and executable without semantic change. The generated successor manifest pins that exact object and maps each carried anchor to the same named check/digest; a missing or changed mapping fails A-1 below. This successor changes only the Builds-on edge and adds §3's review-readiness composition.

The existing `ProposalStateReader.States`, GitHub GraphQL shape, Carina scan, identity+host-umbrella budget, cutover marker, upgrade barrier, downgrade drain support, EVT-4, and adapter kit are therefore not reimplemented or forked. No code path may claim “v5” while bypassing the pinned v4 conformance component.

## 3. Review-readiness composition

### 3.1 Consumer ports

```go
type ReviewObligationStateReader interface {
	Open(ctx context.Context, scope string) ([]ReviewObligationState, error)
}

type ReviewRecoveryLedger interface {
	Exact(ctx context.Context, scope string) ([]RecoveryEntry, error)
	Append(ctx context.Context, entry RecoveryEntry) (EvidenceRef, error)
}

type ReviewStateReader interface {
	DeriveReadiness(ctx context.Context, scope string, policy RecoveryPolicy) (ReviewWakeState, error)
	SnapshotStatus(ctx context.Context, scope string) (ReviewStatusSnapshot, error)
}
```

The implement-now adapter supplies immutable obligation identity, task/proposal/revision/backend, provider attempt count, last terminal failure class, and verified/pending state. It supplies no creation timestamp and owns no retry bound. The recovery adapter reads/writes `review-recovery/v1` on the configured drive-evidence git ref, outside disposable workspaces. `DeriveReadiness` is the drive's deterministic join and may persist only first-observation initialization through the recovery adapter. `SnapshotStatus` exact-reads the same sources but can never call `Append`; the separate methods make the read-only status contract mechanically testable.

### 3.2 Closed joined state

`ReviewWakeState` is a closed `review-wake-state/v1` envelope with `Status = known|unknown`. Known rows contain obligation/task/proposal/revision/backend; `ProviderState = handoff_pending|review_pending`; provider attempt count/failure; consumer `first_observed_at_utc`, derived `age_s`, recovery epoch/count/frozen-bound; and `Eligibility = handoff_repair|eligible|exhausted`. Verified obligations are absent from open rows but their recovery history remains immutable. A `handoff_pending` row always has `handoff_repair` and no consumer recovery epoch/count; it authorizes only a normal executor handoff-repair derivation, never consumer admission or a review-only request.

Join keys are repository scope + stable obligation identity. During `DeriveReadiness`, a `review_pending` provider row without recovery state initializes one append-first record with injected-clock UTC, epoch identity, count 0, and the current closed positive policy bound. A `handoff_pending` row is never initialized or selected: the normal executor must first exact-repair EVT-10 and return it as `review_pending`. During `SnapshotStatus`, a healthy `review_pending` row not yet observed renders `recovery=unobserved`, with immutable provider fields and literal `unknown` placeholders for age/epoch/count/bound; it does not append or invent canonical values. A recovery row without its provider obligation is an integrity-unknown, never stale cleanup. Duplicate/conflicting identities are invalid.

### 3.3 Recovery ledger and age

`review-recovery/v1` is an append-only closed log. Admission appends `count+1` before the review-only invocation. `(scope, obligation, epoch, count, action)` is the idempotence key; identical replay is a no-op, conflict fails closed. Config/restart cannot change an epoch's time/bound. Explicit operator rearm appends an attributable new epoch linked to the exhausted evidence; it never edits history. Verified closure appends but does not delete.

`age_s = floor(max(0, now_utc - first_observed_at_utc)/1s)` from an injected wall clock. Attempts/restarts/rearm do not change the original first-observed value; backward movement clamps at zero. Routine invoke/defer EVT-7 may remain in the rotated local stream. Exhausted/rearmed EVT-7 fires only after immutable append and carries its stable evidence ref.

### 3.4 Unknown

Provider or recovery transport failure, permission denial, invalid envelope, unknown field, unmatched recovery entry, or conflicting replay returns `Status=unknown`; no partial known subset is offered for action. The reader emits EVT-8 with source/kind/backoff, performs no implementation/review launch, and the round is non-clean. The next exact read of both sources is the only recovery.

### 3.5 Invariants

- **DINV-14** — V4 proposal/budget behavior is exact-object incorporated and cannot be shadowed by a v5-specific implementation.
- **DINV-15** — Review readiness derives only from exact provider obligations plus exact consumer recovery evidence; EVT-16/report projections never authorize idle or launch.
- **DINV-16** — Provider attempt count is observational; only consumer recovery count/bound controls driver eligibility.
- **DINV-17** — Age uses consumer `first_observed_at_utc` and injected time; no provider/git timestamp is inferred.
- **DINV-18** — Unknown on either source is non-empty/non-clean and authorizes zero work.
- **DINV-19** — Exhaustion and rearm become observable only after immutable attributable evidence append.
- **DINV-20** — Status uses `SnapshotStatus`; no status path can initialize, admit, close, exhaust, or rearm recovery state.
- **DINV-21** — `handoff_pending` is distinct from review-pending eligibility: it consumes no recovery admission and can reach review only after normal executor exact-repairs EVT-10.

### 3.6 Observability

Routine state reads log source, row count, join result, and duration without content. EVT-8 carries round/scope/source/kind/defer/backoff/reason. Recovery evidence carries internal IDs/times/counts/principal/reason/evidence refs only. No prompt, code, comment body, artifact payload, credential, raw attempt entropy, or model output enters it. Fresh-clone replay is the debugging authority.

### 3.7 Security & privacy

Recovery evidence contains internal proposal/revision/principal identifiers and is access-controlled with the configured drive-evidence ref. The closed reader and append ports reject unknown fields and conflicting identities. Logs and immutable evidence exclude prompt, code, comment body, artifact payload, credentials, raw attempt entropy, and model output. The state reader mints no credential and introduces no network listener or executable content.

## 4. Alternatives considered

| Alternative | Why rejected |
|---|---|
| Read EVT-16/report summaries | projections can be stale or overwritten; not obligation truth |
| Put recovery bound/count in implement-now | provider attempts do not count preflight failure; guard belongs where invocation spend occurs |
| Use provider creation or git commit time | no provider field; git metadata is not domain time |
| Treat recovery-store failure as empty | exact laundering defect this change closes |
| Commit all wake telemetry | routine ticks are not improvement-bearing evidence; only exhaustion/rearm crosses AP-28's boundary |

## 5. Risks

| ID | Risk | Impact | Mitigation | Verification |
|---|---|---|---|---|
| R9 | two watchers initialize one missing recovery row | duplicate epochs/bounds | append CAS + identity idempotence; conflict invalid | concurrent first-observation fixture |
| R10 | provider and recovery snapshots are from different logical moments | stale eligibility | exact-read both, then revalidate provider identity/state before consumer admission; verified/missing becomes non-actioning rederive | race fixtures |
| R11 | recovery evidence ref unavailable | exhaustion can disappear | append-first and fail round non-clean; never emit exhausted first | outage/replay fixture |

## 6. Acceptance

| ID | Acceptance criterion | Verify | Required evidence | Traces to |
|---|---|---|---|---|
| A-1 | The closed `internal/executor/port/testdata/state-reader-v5-carry-forward.json` manifest pins state-reader-v4 object `9a75b181b550d148b65fb7fd67d1d21558266e7f` and maps every v4 DINV/A/Serves anchor to `{package, check_name, fixture_digest}`. A Go conformance test parses closed, rejects duplicate/missing/unknown/favorable-subset entries, verifies the frozen blob and registered checks, then runs the carried suites unchanged. | Test + Inspection | carry-forward manifest + negative corpus | #DINV-14 |
| A-2 | Known join fixtures cover empty, handoff-pending, one/many review-pending, mixed backend, verified-cleared, provider attempts/failures, initial recovery state, config change, and restart exactly; handoff-pending never initializes or spends recovery. | Test | table-driven join suite | prd:self-drive-wake/FR-15 · #DINV-15 · #DINV-16 · #DINV-21 |
| A-3 | Injected-clock new/retried/long-pending/restart/backward-time fixtures produce exact append-first `first_observed_at_utc` and `age_s`. | Test | clock fixture output | prd:self-drive-wake/FR-15 · #DINV-17 |
| A-4 | Provider/recovery unavailable, permission denied, invalid envelope, unknown field, unmatched/conflict cases emit exact EVT-8, return unknown/non-empty, and produce zero launch. | Test + Failure injection | unknown matrix + event wire | prd:self-drive-wake/UB-11 · prd:self-drive-wake/EVT-8 · #DINV-18 |
| A-5 | Recovery append replay is idempotent; conflict fails; exhaustion/rearm/closure survive fresh clone with stable evidence refs and no history rewrite. | Test + Inspection | git replay/CAS/security matrix | prd:self-drive-wake/FR-15 · #DINV-19 |
| A-6 | `SnapshotStatus` renders initialized rows with immutable revision, age, provider attempts, recovery epoch/count/bound, failure, and exhausted/unknown state; a not-yet-observed provider row renders `recovery=unobserved` plus literal unknown recovery fields. Every fixture proves zero recovery-adapter writes. | Test + Demo | status-state fixture + append spy | prd:self-drive-wake/FR-17 · #DINV-20 |

## 7. Open questions

None. Recovery policy bound values are closed configuration data owned by wake-watch-v3; changing store ownership, time source, join keys, or evidence durability requires a successor.

## Principles

- **AP-1/AP-3 — honored.** Consumer ports and conformance kits keep provider/recovery adapters outside the core.
- **AP-5/AP-6 — honored.** Closed versioned identities join both stores; no map-shaped or projection authority.
- **AP-10/AP-11 — honored.** One deterministic join and one recovery counter authority.
- **AP-15/AP-17 — honored.** Exact reads precede action; unknown fails in the safe no-launch direction.
- **AP-16 — honored.** Recovery spend is bounded in the consumer that spends it (incident:I8).
- **AP-28 — honored.** Improvement-bearing exhaustion/rearm persists; routine ticks remain ephemeral.
- **RW-5 — honored.** This successor changes one independently revisable concern while exact-incorporating v4.
- **Deviation:** none.
