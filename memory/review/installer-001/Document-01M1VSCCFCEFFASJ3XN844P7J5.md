# Review: prd:installer/001 — review report

| Field | Value |
|---|---|
| **In one line** | Approve, round 1 — the init-hardening alignment amendment's blast radius is honest (declared = computed contract-changing), its Learned-from references resolve, the folded chain is intact, and every replacement row is consistent with the merged `prd:init-hardening`. |
| **Relevant-when** | auditing or consuming the review of installer amendment 001 (init-hardening alignment); asking why it was approved; tuning prd-review calibration against real findings |
| **Subject** | `prd:installer/001` |
| **Moment** | amendment |
| **Reviewer** | prd-review · `.pyxis/authoring/prd-review.md` (run inline by the authoring session, bootstrap-era) |
| **Date(s)** | 2026-07-28 |
| **Verdict** | approve |
| **Tracking** | — *(the amendment's introducing PR, once filed)* |
| **Template version** | review-report/v1 |

## Round 1

No findings survived adversarial scrutiny. The dimensions examined, for the record:

- **Blast-radius honesty** — declared `tier: contract-changing; touches: FR-3, FR-19, UB-1, §0` equals the computed radius (`docgate diff`: contract-changing; affected §0 · FR-3 · FR-19 · UB-1). No understatement.
- **Learned-from resolution** — `operator-decision:brainstorm:init-hardening#D-7`, `#D-8`, and `review-thread:review:init-hardening-000#FND-1` all resolve to merged artifacts; the classes are within the closed set.
- **Chain integrity** — the fold marks FR-3 → FR-23, FR-19 → FR-24, UB-1 → UB-11 with both entries retained; replacement IDs are fresh in the global namespace (prior maxima FR-22, UB-10); the §0 supersede is wholesale and byte-faithful to the base except the one routed numbers claim, re-anchored FR-3 → FR-23.
- **Alignment fidelity** — FR-23's question boundary matches `prd:init-hardening` FR-6/FR-7 (conditional exchange, promptless parity untouched); FR-24 extends preview's report surface without re-contracting the checks it cites; UB-11's content-based trigger and adopt routing match `prd:init-hardening` FR-1/FR-2 and compose with `prd:existing-repo-onboarding` UB-1's reverse edge. Rows tracing the superseded FR-3 (NR-3, UB-6) remain valid provenance per supersession mechanics.
- **Scope completeness** — the remaining installer rows were swept for init-hardening contradictions: FR-4 (promptless), FR-20 (actions-mode first-push green), UB-7 (prerequisite refusals), FR-15 (welcome item), UB-10 (identity) all remain true under the new contract; in particular no installer row contracts an unconditional workflow install, so `ci_mode` semantics need no supersession here — `prd:init-hardening` FR-11 owns them.
- **Generated-surface consequence** — the fold legitimately re-derives the tech-design index: `installer-v1` now carries `stale-serves` (its Serves cite the superseded anchors), the designed staleness signal; the regenerated index rides the amendment PR.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | approve | 2026-07-28 |

## Escalation brief

none

## Measurements

- **Rounds**: 1
- **Findings**: 0 blocking · 0 advisory
- **Dispositions**: 0 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

none
