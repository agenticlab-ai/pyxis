# Review: pipeline-measurement — design-review report

| Field | Value |
|---|---|
| **In one line** | approve-with-advisories — a clean greenfield design faithful to its record and PRD; three advisories (store concurrent-append safety, incident-I3 CLI robustness, one DINV without a test), all applied. |
| **Relevant-when** | auditing or consuming the review of design:pipeline-measurement; asking why it was approved; tuning tech-design-review calibration against real findings. |
| **Subject** | design:pipeline-measurement · shape: none · supersedes: — |
| **Moment** | new-design |
| **Reviewer** | tech-design-review · 2026-07-21 |
| **Date(s)** | 2026-07-21 |
| **Verdict** | approve-with-advisories |
| **Tracking** | — *(the subject's introducing PR, once filed)* |
| **Template version** | design-review-report/v1 |

## Round 1

### FND-1 — the store's concurrent-append safety was unspecified

- **Severity**: advisory (CAL-4)
- **Lens**: feasibility-completeness
- **Evidence**: design:§3.6 (the append-only JSONL store) — vs — PRD §12 (parallel authorship: many sibling runs) + AP-23 (append-only, parallel-safe): §3.6 cited AP-23 but did not decide *how* concurrent parallel-run appends stay collision-free (a shared hot file would contend).
- **Acceptance condition**: §3.6 specifies the concurrent-write mechanism (one-file-per-run / atomic append) or parks it as a `Q-n`.
- **Disposition**: applied — §3.6 now writes **one file per run** (keyed by `run_id`), so parallel runs never contend and each append is collision-free.

### FND-2 — incident I3 (CLI fail-closed on misparse) was unaddressed for the recorder/reader

- **Severity**: advisory (CAL-4)
- **Lens**: feasibility-completeness
- **Evidence**: design:§3.11 (the recorder/reader is a standalone CLI tool that *writes* records) — vs — incident I3 (consequential CLIs fail closed on misparse; never truncate a consequential run's output): the design specified no CLI-input robustness, though the recorder writes to disk (a mild consequential path).
- **Acceptance condition**: a `DINV`/§3.x note on misparse-fail-closed (or an explicit not-consequential rationale).
- **Disposition**: applied — DINV-9 + a §3.11 sentence + acceptance A-11 + a Principles bullet added (fails closed on a flag-like positional before any write; no output truncation — per I3).

### FND-3 — DINV-7 carried no acceptance row

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: design:#DINV-7 (every record carries a resolvable run/feature id) — vs — the §6 acceptance table (A-1..A-9): DINV-7 was the one invariant without a runnable Verify, while every other DINV mapped to an A-row.
- **Acceptance condition**: add an A-row exercising DINV-7.
- **Disposition**: applied — A-10 added (run/feature id resolvable + a feature's phases aggregate to one cost).

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | approve-with-advisories | 2026-07-21 |

## Escalation brief

none

## Measurements

- **Rounds**: 1
- **Findings**: 0 blocking · 3 advisory
- **Dispositions**: 3 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

*Post-approve entries only — the M2 residual data; frozen rounds and history are never edited.*

none
