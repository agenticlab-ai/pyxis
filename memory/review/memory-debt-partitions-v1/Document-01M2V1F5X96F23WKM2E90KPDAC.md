# Review: memory-debt-partitions-v1 — design-review report

| Field | Value |
|---|---|
| **In one line** | Approved after recording the bounded configuration-principle deviation for the existing policy location; technical direction remains unchanged. |
| **Relevant-when** | reviewing independent memory debt policy, its configuration boundary, and the evidence behind the design review |
| **Subject** | design:memory-debt-partitions-v1 · shape: present · supersedes: — |
| **Moment** | new-design |
| **Reviewer** | tech-design-review · repository guide inspected 2026-09-17 |
| **Date(s)** | 2026-09-17 |
| **Verdict** | approve |
| **Tracking** | Local introducing proposal; external PR not yet created |
| **Template version** | design-review-report/v1 |

## Round 1

Four lenses ran concurrently: independent contract-quality, coherence, and
upstream-alignment agents, with feasibility-completeness examined by the
coordinator against the existing baseline parser/comparator, replay, Git
reader, and CLI seams. Design lint passed. The subject's human-only §0 was
excluded. Findings concern the design/shape pair, not implementation proof.

### FND-1 — Record the policy-location deviation

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: design: §3.1 says “Discovery reads its fixed directory from the pinned Git tree”; shape: S-1 fixes the policy marker and member directory under `.github`; design: Principles originally said “Deviation: none.” AP-30 requires operand-owned paths to resolve through repository configuration, exempting only Pyxis-owned installed paths.
- **Acceptance condition**: Resolve the policy root through sanctioned configuration with trusted-lineage relocation checks, or explicitly justify a bounded AP-30 deviation for retaining the existing canonical root and its new members.
- **Disposition**: applied — Principles now records the bounded deviation, its authority-preservation rationale, affected paths, and the trigger for a separately verified relocation protocol. The approved layout and migration are unchanged.

No contract-quality, coherence, or feasibility-completeness findings survived
adversarial screening. The historical phase-advance exception and
scheduled-inventory mode qualify the corresponding general invariants.

## Round 2

The upstream lens rechecked FND-1 against only the changed Principles span
and its governing AP-30 slice. The acceptance condition is satisfied: the
deviation has an explicit reason, scope, and relocation trigger. No changed-span
regression was found. The new requirement joins were also checked against
their referenced rows in `prd:parallel-memory`; each has substantive coverage
in this design and its shape. No new findings.

The current authoring tree passes `docgate gate` and the feature's
`contract-doc-pr` contract check. Those checks establish mechanical validity,
not implementation completion or permission to skip the plan's merge gate.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-09-17 |
| 2 | approve | 2026-09-17 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 1 blocking · 0 advisory
- **Dispositions**: 1 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

none
