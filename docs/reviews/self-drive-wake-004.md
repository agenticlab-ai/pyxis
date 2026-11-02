# Review: self-drive-wake amendment 004 — review report

| Field | Value |
|---|---|
| **In one line** | Approve — configured review is now a typed second frontier with driver-owned durable recovery, one selected-frontier sweep per round, immutable exhaustion, and no implementation redispatch. |
| **Relevant-when** | auditing self-drive-wake amendment 004; implementing or reviewing PR #1034's review-only recovery and non-laundering path |
| **Subject** | `prd:self-drive-wake/004` |
| **Moment** | amendment |
| **Reviewer** | prd-review · 2026-07-10 guide |
| **Date(s)** | 2026-08-06 (rounds 1–2) |
| **Verdict** | approve |
| **Tracking** | issue:1029 · PR #1034 |
| **Template version** | review-report/v1 |

## Round 1

### FND-1 — Recovery state and arithmetic were not executable

- **Severity**: blocking (CAL-3)
- **Lens**: contract-quality
- **Evidence**: initial FR-15 exposed an undefined `age`, while FR-16 left the retry counter, equality boundary, driver-round lifetime, interruption accounting, and whole-store-unknown outcome unspecified. EVT-7 required an obligation identity that a failed whole-store read cannot supply.
- **Acceptance condition**: define clock/source/units/restart semantics; bind exact counter ownership and `count < bound` arithmetic at every crash edge; define one durable round; and give whole-store unknown a closed no-launch event and exit path.
- **Disposition**: applied — FR-15 derives `age_s` from consumer-owned append-first `first_observed_at_utc`; FR-16 defines durable rounds, append-first consumer recovery admission, exact eligibility/exhaustion, and crash semantics; UB-11/EVT-8 close whole-store unknown with bounded deferral, zero launch, and non-clean outcome.

### FND-2 — Scope, cardinality, and release gates disagreed

- **Severity**: blocking (CAL-1)
- **Lens**: coherence + completeness critic
- **Evidence**: initial FR-16 implied one executor invocation per obligation even though the provider reconciles an in-scope frontier; §1/§4.1/§4.2 still named only the original three proposal-wake parts; and §3.3 could ship on M1–M3 while the new blocking recovery proof failed. The old all-Must, all-NR, all-UB, and all-event DoD rows also stopped before the new live IDs.
- **Acceptance condition**: choose one enforceable sweep cardinality; make recovery a committed scope element; define a single ship predicate; and supersede every aggregate evidence row so deletion/inversion of the recovery path fails the gate.
- **Disposition**: applied — FR-16 and §5.5 require one deduplicated selected-frontier review-only sweep per round with at most one provider attempt per selected obligation; §1/§2.1/§4.1..§4.4 own the fourth part; §3.3 is the sole ship predicate; DOD-12..DOD-15 cover every live Must/NR/UB/event row.

### FND-3 — The retry guard and exhausted disposition lived on the wrong durability boundary

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: provider attempt counts do not advance for factory/auth/preflight failures, so using them as the driver retry budget permits an unbounded loop, contrary to incident I8/AP-16. EVT-7 was called durable while the existing wake event stream is rotated/local, contrary to AP-28 for an exhaustion decision. The declared blast radius also omitted every row superseded while applying the first draft.
- **Acceptance condition**: meter every budget-consuming driver invocation in consumer-owned durable state; keep provider attempts separate; make exhaustion/rearm immutable and attributable with replay/idempotence rules; and declare the full computed blast radius.
- **Disposition**: applied — `review-recovery/v1` is an append-first git-retained consumer ledger with frozen bounds, epochs, counts, evidence refs, and conflict detection; setup failures consume recovery admission; routine telemetry remains ephemeral but exhausted/rearmed EVT-7 follows immutable evidence; the header enumerates all supersessions and sections.

## Round 2

### FND-4 — The all-exhausted frontier still required an empty executor invocation

- **Severity**: blocking (CAL-3)
- **Lens**: contract-quality
- **Evidence**: the first disposition defined a durable round as ending in one settled invocation and said to issue exactly one after selecting eligible obligations, while exhausted obligations were excluded. With all obligations exhausted, the selected set was empty but the row still required a sweep.
- **Acceptance condition**: make invocation conditional on a non-empty selected set; let an all-exhausted round end with persisted no-launch evidence; prove zero provider preflight, review attempt, and implementation activity.
- **Disposition**: applied — FR-16 now says “if and only if” the selected set is non-empty; all-exhausted replays/appends EVT-7 and launches nothing; DOD-11 carries the zero-preflight/zero-launch fixture.

### FND-5 — Review age depended on an undeclared provider timestamp

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: the first disposition required provider `created_at_utc`, but implement-now's obligation contract exposes no timestamp and the routing evidence contract rejects locally invented canonical time.
- **Acceptance condition**: either amend the provider schema or source age from a consumer-owned observation timestamp; no decision may depend on git metadata or an undeclared field.
- **Disposition**: applied — the recovery ledger append-first records source-owned `first_observed_at_utc`; FR-15 derives `age_s` only from it with injected-clock and backward-clock behavior; provider data is unchanged.

Scope-locked disposition verification found no remaining contradiction in the changed spans. The folded projection contains one conditional selected-frontier invocation rule, separate provider/recovery counters, a closed unknown path, immutable exhaustion/rearm evidence, complete aggregate gates, and no live citation to the superseded FR-7/UB-5/DOD rows.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-08-06 |
| 2 | approve | 2026-08-06 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 5 blocking · 0 advisory
- **Dispositions**: 5 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

- **2026-08-19 · factory (upstream reconciliation)** — After `prd:implement-now/005` merged, a scope-locked check found that the amendment still stopped its provider citations at FR-45 and did not carry the new sibling `impl-review` capability blocker. The source now traces selected-frontier recovery and zero-implementation isolation to live FR-46/FR-47/NR-10, updates the provider range to FR-40..FR-47, and makes explicit that provider/consumer implementation and activation remain blocked until the sibling contract/design authorizes host-adapter FR-6 performance plus exactly-one receipt-returning publication. The recovery state machine, prior findings, dispositions, and round-2 approval are unchanged; this is not a third review round.
- **2026-08-19 · @mdarabi (human overlay)** — Corrected the two remaining binding all-pending-frontier phrases in §4.2 and §0 judge-these #7 to `selected-frontier`, matching FR-16, NR-3, UB-9, §5.5, and §6.2. The same stale wording was corrected in §4.4's amendment reason and this report's scope-locked verification sentence so the evidence package no longer repeats the contradicted vocabulary. No recovery behavior or prior verdict changed.
