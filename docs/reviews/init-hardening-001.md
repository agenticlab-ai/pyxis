# Review: prd:init-hardening/001 — review report

| Field | Value |
|---|---|
| **In one line** | Approve, round 1 — the permission-hand-off amendment resolves a machine-surfaced contradiction between two frozen contracts by the operator's explicit ruling, with honest blast radius and an additions-plus-supersession chain intact. |
| **Relevant-when** | auditing or consuming the review of init-hardening amendment 001 (the permission hand-off); asking why it was approved; tuning prd-review calibration against real findings |
| **Subject** | `prd:init-hardening/001` |
| **Moment** | amendment |
| **Reviewer** | prd-review · `.pyxis/authoring/prd-review.md` (run inline by the authoring session, bootstrap-era) |
| **Date(s)** | 2026-07-29 |
| **Verdict** | approve |
| **Tracking** | — *(the amendment's introducing PR, once filed)* |
| **Template version** | review-report/v1 |

## Round 1

No findings survived adversarial scrutiny. The dimensions examined, for the record:

- **Blast-radius honesty** — declared `tier: contract-changing; touches: §0, UB-1` equals the computed radius (contract-changing; §0 · UB-1).
- **Learned-from resolution** — the impl-review payload persists on PR #812 (comment 5125178984, FND-1's escalation brief); the operator's ruling is this session's recorded decision; both classes are within the closed set.
- **Chain integrity** — one supersession (UB-1 → UB-4, fresh ID beyond the base's UB-3 maximum), both entries retained by the fold; the §0 supersede is wholesale and byte-faithful except judge row 3, whose rebind states exactly the ops' consequence.
- **Contradiction genuineness** — verified against the corpus: `prd:installer` UB-4's hand-off contract ("complete-the-rest-and-hand-off" for permission-classified failures) and the as-built wire semantics predate and contradict the base's UB-1 rollback demand; the composed matrix (T11) surfaced it; the amendment aligns the newer contract to the deliberate older design rather than silently breaking the non-admin path.
- **Consumer note** — `design:init-hardening-v1`'s A-3 row cites the superseded anchor; the regenerated index flags the design `stale-serves` — the designed staleness signal, resolved by a later design refresh, never by editing the frozen document.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | approve | 2026-07-29 |

## Escalation brief

none

## Measurements

- **Rounds**: 1
- **Findings**: 0 blocking · 0 advisory
- **Dispositions**: 0 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

none
