# Review: init-hardening-v1 — design-review report

| Field | Value |
|---|---|
| **In one line** | Approve after one revision round — four blocking findings (a coherence check that denied its own first refusing pair, an unimplemented Serves anchor, two mis-traced acceptance rows, a silently omitted principle) all applied. |
| **Relevant-when** | auditing or consuming the review of init-hardening-v1; asking why it was approved; tuning tech-design-review calibration against real findings |
| **Subject** | design:init-hardening-v1 · shape: none · supersedes: — |
| **Moment** | new-design |
| **Reviewer** | tech-design-review · `.pyxis/authoring/tech-design-review.md` (run inline by the authoring session, bootstrap-era) |
| **Date(s)** | 2026-07-28 |
| **Verdict** | approve |
| **Tracking** | — *(the subject's introducing PR, once filed)* |
| **Template version** | design-review-report/v1 |

## Round 1

### FND-1 — The coherence check denies the wedge FR-11 itself creates

- **Severity**: blocking (CAL-4 — load-bearing for acceptance: A-2's defect coverage and the design's FR-11 truth claim)
- **Lens**: feasibility-completeness
- **Evidence**: design: "this check gates combinations, and currently holds no refusing pairs" (§3.1 check 6) — vs — `prd:init-hardening/FR-11` composed with the wire table's protection semantics: with the workflow excluded under `local`, `wiring.protection=required-checks` still demands the hosted check context — a repository born with every PR blocked on a check that never reports. This is the audit's original trap, masked until this very design makes the knob real.
- **Acceptance condition**: check 6 carries the pair as a live refusal with both remedies; A-2's seeded-defect set includes it.
- **Disposition**: applied — §3.1 check 6 rewritten; A-2 criterion and traces extended (FR-11).

### FND-2 — Serves cites NFR-1 with no implementing content

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment (Serves substance)
- **Evidence**: design: header `Serves … prd:init-hardening/NFR-1` — vs — `prd:init-hardening/NFR-1` (the check pass fits the installer contract's install budget): no §3 or §6 content addressed the bound; the lint proves the anchor resolves, not that it is true.
- **Acceptance condition**: a performance treatment plus an Analysis acceptance row tracing NFR-1 — or the anchor dropped.
- **Disposition**: applied — §3.10 gains the Performance paragraph; A-13 added (Analysis, measurement record, traces NFR-1).

### FND-3 — Two acceptance rows mis-trace FR-15

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: design: A-5 traced "FR-11 · FR-15 · §3.8" and A-9 traced "FR-15 · #DINV-2" — vs — `prd:init-hardening/FR-15` is the code-without-manifest doctor advisory, served by A-7; A-5's advisory is the `ci_mode` mismatch (design-bound via §3.8), and A-9 is prerequisite delegation (DINV-2's property).
- **Acceptance condition**: both rows trace their real anchors.
- **Disposition**: applied — A-5 → "FR-11 · §3.8"; A-9 → "#DINV-2 · §3.1".

### FND-4 — AP-24 silently omitted where §3.8 instantiates it

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment (principles honesty)
- **Evidence**: design: §3.8's mechanism — an authored class-policy entry driving the derived desired set — vs — AP-24 ("Governance is never rebuilt from the governed"): the exact pattern that principle names, absent from the Principles section.
- **Acceptance condition**: the AP-24 line lands with its §3.8 tie.
- **Disposition**: applied.

## Round 2

Scope-locked re-examination of the four amended spans (§3.1 check 6 + A-2 · §3.10 Performance + A-13 · the A-5/A-9 trace cells · the Principles AP-24 line) against their acceptance conditions, plus regression check within the changed spans; the design-class floor (gen-index · check-citations · check-design-shape · gate) re-ran green on the revised draft. All four conditions met; no new findings within scope; unchanged content not re-examined (protocol).

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-07-28 |
| 2 | approve | 2026-07-28 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 4 blocking · 0 advisory
- **Dispositions**: 4 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

none
