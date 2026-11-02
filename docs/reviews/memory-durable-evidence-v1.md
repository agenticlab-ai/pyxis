# Review: memory-durable-evidence-v1 — design-review report

| Field | Value |
|---|---|
| **In one line** | Approve: both blocking findings are applied through durable failure-event retention and ownership-safe draft-repair exclusion. |
| **Relevant-when** | consuming the durable-memory-evidence design review; verifying closure of its retention and concurrent-write findings |
| **Subject** | design:memory-durable-evidence-v1 · shape: none · supersedes: — |
| **Moment** | new-design |
| **Reviewer** | tech-design-review · guide 2026-07-14 · tech-design-review-calibration/v1 |
| **Date(s)** | 2026-09-16 |
| **Verdict** | approve |
| **Tracking** | Subject's introducing PR, once filed |
| **Template version** | design-review-report/v1 |

## Round 1

Reviewed the approved, unmerged design after `lint-tech-designs --merge-base origin/main` passed. The governing corpus and subject were pinned by context-pack digest `sha256:43fc66c99a7aaaf277e2b3ff83d82e38e11e37b1acc3e5c45b12683d0db1dc29`; the human-only §0 was excluded. All four lenses ran, with delegation within available capacity. Contract quality and coherence produced no surviving findings. Squash support and deterministic, AI-independent evidence processing were treated as fixed operator rulings.

### FND-1 — Failure reports lack durable retention

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: design: §3.4 says checks “return a versioned, canonical JSON report” and places execution diagnostics in “existing ignored memory/gate log homes,” but assigns no retention owner or home to failure reports — vs — AP-28, which requires gate defect reports as committed, immutable event records. The existing reporting seam (`internal/memory/cli/reconcile.go`, `writeReconciliationOutput`) emits to stdout; that does not preserve the failure after logs expire or repair changes the inputs.
- **Acceptance condition**: Name the existing caller/writer responsible for retaining failure reports as immutable Git event records with their pinned inputs and findings. Verify that a retained failure survives removal of ignored diagnostics and a later successful repair without overwrite; clean passes create no event. Cite AP-28 in Principles.
- **Disposition**: applied — §3.4 assigns immutable events under `.pyxis/evidence/memory/<canonical-report-sha256>.json` to the shared gate/draft writer and commit responsibility to the proposal author or delivery caller; read-only checks expose explicit retention. A-7 requires survival after log deletion, repair, and a fresh clone; Principles cites AP-28.

### FND-2 — The existing lock cannot protect draft replacement from stale writers

- **Severity**: blocking (CAL-4; load-bearing for A-7)
- **Lens**: feasibility-completeness
- **Evidence**: design: §3.6 says “Use the existing memory writer's lock and atomic file replacement” and “A changed pin or file causes refusal, not overwriting concurrent work” — vs — AP-15/AP-22 and A-7. At source revision `3085177722469830eaa6fc48dd38607904b5ceb8`, `internal/memory/store/lock.go:13,26,31–32` expires a lock after one minute without proving its owner exited, and release unconditionally removes that path. A paused writer can resume after a second writer takes over and writes, overwrite its update, and remove its lock; atomic replacement alone does not prevent this.
- **Acceptance condition**: Require ownership-safe exclusion or equivalent conflict detection before draft replacement. Extend A-7 with a deterministic two-writer fixture covering a paused live holder, attempted takeover, and old-holder release, proving no stale overwrite and no removal of another holder's lock without a wall-clock completion assumption.
- **Disposition**: applied — §3.6 requires persistent kernel-held exclusion, no age-based takeover, owner-bound idempotent release, refusal of old directory locks, and expected-byte checks within the lock. A-7 requires the deterministic paused-holder/takeover/old-release fixture.

## Round 2

Scope-locked re-review covered only the changed retention paragraphs in §3.4, the lock paragraph in §3.6, A-7's evidence/traces, and the AP-28 Principles entry. Both finding acceptance conditions are satisfied by the revised design; no regression survived within those changed spans. The specified runtime fixtures remain implementation obligations.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-09-16 |
| 2 | approve | 2026-09-16 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 2 blocking · 0 advisory
- **Dispositions**: 2 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

none
