# Contract-gate dogfood sweep — disposition record

| Field | Value |
|---|---|
| **Feature** | contract-gate |
| **Task** | T8 — fixtures, determinism, performance, and the dogfood sweep |
| **Command** | `go run ./cmd/docgate contract-gate --all` |
| **Trigger** | `planning-entry` (the hard bar — every defect class blocking) |
| **Date** | 2026-07-17 |
| **Result** | 22 live contracts — 11 clean, 0 advisory, 11 blocking |
| **Verdict** | every bounce is a REAL contract defect; ZERO false bounces |

This record is the M2 honesty artifact (`prd:contract-gate/M2`, tech design §3.11):
the mechanical gate run over **every live contract in the repo's own corpus**,
with each bounce classified as a genuine catch or a false bounce. It is a
descriptive snapshot of the corpus at the date above — not authoritative, and it
staleness-decays as the corpus's designs catch up to their PRDs; re-run the
command to regenerate. (All `prd:` / `design:` tokens below are held in code
spans so this record adds no typed-citation resolution burden.)

## Summary

The sweep gates each feature's assembled contract read-only under the
planning-entry severity split. Under that split every defect class is blocking (a
contract entering planning must be whole), so a feature bounces if it has any
stale serving design or any live `Must` requirement no live design serves.

- **11 clean**: `argus`, `argus-boundary-enforcement`, `argus-ledger-lint`,
  `argus-unattended-drive`, `context-pack`, `contract-gate`,
  `design-solution-shape`, `implementation-verification`, `plan-viewer`,
  `read-to-rule`, `workbench-ui`.
- **11 blocking**: enumerated below, each with its verified real defect.

The known drift instance the plan predicted — `plan-new`'s new `FR-27` (the
contract-gate precondition Must introduced by T7's own amendment, with no serving
design yet) — is present and caught. The sweep additionally surfaces ten more
genuine catches: the repo is in active product-definition phase, so many merged
PRDs carry `Must` requirements whose technical designs do not yet declare a
`Serves` for every one, and several designs still serve or build on units that
were later superseded/retired. These are true catches worth the operator seeing —
exactly the "surface the drift" purpose of the gate — not gate bugs.

## Per-feature disposition (the 11 bounces)

Each row was verified against the folded source map (the unit's `status` +
`priority`) and every live serving design's `Serves` / `Builds-on` list. The
"real?" column is a manual confirmation that the flagged offender genuinely lacks
a serving design (coverage) or genuinely serves/builds-on a superseded unit
(currency).

| Feature | Check(s) | Representative offender | Real? |
|---|---|---|---|
| `docgate` | inverse-coverage ×8 | live `Must` `FR-12`, `FR-13`, `FR-17`, `FR-25`, `FR-27`, `FR-31`, `UB-1`, `UB-2` served by no live design (its 10 live designs serve a proper subset) | ✅ real |
| `feature-brainstorm` | currency ×1, inverse-coverage ×13 | design `brainstorm-record-model` still serves `prd:feature-brainstorm/FR-6`, which is **superseded** by `FR-17`; 13 live Musts unserved | ✅ real |
| `impl-review` | inverse-coverage ×9 | design `impl-review-v1` serves `UB-2/5/6/8` but not `UB-1`, `UB-3`, `UB-4`, `UB-7` (and `FR-4/8/11/14/15`) | ✅ real |
| `implement-now` | inverse-coverage ×8 | design `implement-now` serves every `FR` but no `UB-1..8` (all live Musts) | ✅ real |
| `incident-machinery` | inverse-coverage ×1 | design `incident-machinery` serves `UB-1` but not `UB-2` (a live Must — the duplicate-`I<n>` boundary) | ✅ real |
| `local-work-tracker` | currency ×1, inverse-coverage ×22 | live designs `worktracker-hierarchy-index` and `plan-new-v1` **build on** `local-work-tracker`, which is **superseded** by `local-work-tracker-v2`; 22 live Musts unserved (the base design is superseded) | ✅ real |
| `plan-new` | currency ×2, inverse-coverage ×1 | **the known drift** — `FR-27` (T7's precondition Must) served by no live design; plus `plan-new-v1` serves superseded `FR-15` and builds on superseded `local-work-tracker` | ✅ real |
| `prd-review` | inverse-coverage ×17 | design `prd-review-artifacts` serves `FR-5/6/7/11/14` only; 17 other live Musts unserved | ✅ real |
| `review-feedback-capture` | currency ×2, inverse-coverage ×4 | design `review-feedback-capture` serves `FR-2` and `FR-6`, both **retired**; 4 live `UB` Musts unserved | ✅ real |
| `tech-design-brainstorm` | inverse-coverage ×14 | design `design-brainstorm-record-model` serves `FR-2/7/8/11/12/13/14` only; 14 other live Musts unserved | ✅ real |
| `tech-design-review` | inverse-coverage ×16 | design `tech-design-review-artifacts` serves `FR-5/6/7/11/14` only; 16 other live Musts unserved | ✅ real |

## Conclusion

- **Zero false bounces.** Every one of the 11 bounces was traced to a concrete,
  verifiable contract defect in the committed machine faces — a live `Must` no
  live design serves, or a live design serving/building-on a superseded unit. No
  bounce was a gate bug; no gate code was changed to make a bounce disappear.
- **The known drift is caught** (`plan-new` / `FR-27`), as the plan required, and
  the sweep surfaces the wider real drift besides.
- **Determinism**: the sweep result is byte-identical across runs
  (`TestSweepByteIdenticalAcrossRuns`), so this record is reproducible.
- **These bounces are not resolved here.** Clearing them is upstream authoring
  work (add the missing `Serves`, or supersede/re-point the stale designs) owned
  by each feature — not T8's scope. T8's job is to prove the gate is honest and
  the corpus's real state is visible, which it is.
