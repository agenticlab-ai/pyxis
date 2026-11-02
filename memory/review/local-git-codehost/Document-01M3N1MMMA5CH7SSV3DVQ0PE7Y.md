# Review: local-git-codehost tech spec — design-review report

| Field | Value |
|---|---|
| **In one line** | Approve durable candidate evidence and distinct final proposal refs. |
| **Relevant-when** | Auditing final-delivery recovery after the evidence worktree is lost. |
| **Subject** | design:local-git-codehost · shape: none · supersedes: — |
| **Moment** | draft |
| **Reviewer** | Codex using tech-design-review guide; same session as draft author, not an independent identity |
| **Date(s)** | 2026-09-28 |
| **Verdict** | approve |
| **Tracking** | https://github.com/agenticlab-ai/pyxis/pull/1312 |
| **Template version** | design-review-report/v1 |

**Review subject:** tech-spec
**Draft digest:** sha256:9995eb7d213277795c6236868e38a3b9a1817fee291a03a8c0cbeffda8d66a90

## Round 1

Reviewed committed head `16949439f31f7442179ba1ad2711870a055a6720` and the unchanged PRD basis through contract-quality, coherence, upstream-alignment, and feasibility-completeness lenses. Revised D-5 publishes the complete accepted `candidate-evidence/v1` record and required scenario results on an immutable, clone-visible ref before operator review, binds that ref/OID/digest and proposal generation to the receipt, embeds the envelope in the final proposal, and verifies it after a crash. A missing or altered ref refuses both landing and delivery verification. D-5 also makes the final proposal branch a distinct ref at `F`, allowing its leased deletion and the feature branch's `F→F'` update in one atomic push. D-6 covers the recovery and ref-collision cases. No substantive design finding survives. This review does not establish hosted CI or exact-head operator approval.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | approve | 2026-09-28 |

## Escalation brief

none

## Measurements

- **Rounds**: 1
- **Findings**: 0 blocking · 0 advisory
- **Dispositions**: 0 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

none
