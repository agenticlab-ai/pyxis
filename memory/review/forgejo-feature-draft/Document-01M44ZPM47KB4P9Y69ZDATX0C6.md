# Review: prd:forgejo-feature-draft/draft — review report

| Field | Value |
|---|---|
| **In one line** | request-changes: the blocking Definition of Done does not verify the Forgejo abandonment path’s required policy check and review. |
| **Relevant-when** | Auditing report 004 or revising the Forgejo feature-draft PRD |
| **Subject** | prd:forgejo-feature-draft/draft |
| **Moment** | draft |
| **Reviewer** | prd-review · guide dated 2026-09-25 |
| **Date(s)** | 2026-10-02 |
| **Verdict** | request-changes |
| **Tracking** | https://github.com/agenticlab-ai/pyxis/pull/1365 |
| **Template version** | review-report/v1 |

**Review subject:** prd  
**Draft digest:** sha256:f843506ca76232942082f628d67849343ad106d9785d18f506648cac130085ce

## Round 1

I read the PRD at commit `1922f691ad771e94a32ae075191b3a937573785e` with section 0 removed before inspection. I read the tech spec as context, the exact draft tree’s `brainstorm/003.md` and empty `problems.md`, and the product vision, architecture principles, incident registry, review guide, template, and calibration. I inspected the existing abandonment path to check the current precondition. I accepted the supplied gate receipts as ground and did not rerun gates.

| Lens | Inspection |
|---|---|
| contract-quality | Checked the Must rows’ fit criteria, verification methods, risk tiers, failure cases, and §5.9. FR-4’s abandonment criterion does not test its inherited precondition. |
| coherence | Traced goals and metrics through §5.7, jobs through requirements, and Must rows through blocking DoD rows; checked §8 against those rows. The abandonment path is absent from the stated DoD tests. |
| upstream-alignment | Checked the current draft brainstorm’s D-1/F-10, relevant vision and principles, and incident lessons. Treated attributed operator and Argus rulings as recorded, without rejudging them. |
| completeness-critic | Checked identity, origin and credentials, protection, exact-head review, mainline proof, HTTP and pagination failures, and partial-effect recovery against the PRD and contextual tech spec. The abandonment acceptance gap survives. |

### FND-1 — Abandonment is not verified by a blocking DoD criterion

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: PRD FR-4 requires “retaining round and abandonment semantics,” but its fit criterion checks only that “abandonment leaves required reason record”; DOD-1 names opening, iteration, and preparation, while DOD-5 names failure fixtures without an abandonment policy or review case. In the exact draft’s `brainstorm/003.md` D-1 and F-10, abandonment requires an affirmative automatic head-branch-deletion policy **before any mutation**. The current tech spec D-3 specifies positive and refusal tests, but those tests are not bound by a blocking PRD DoD criterion. A Forgejo path could therefore mutate on a false policy observation and still satisfy the stated DoD evidence.
- **Acceptance condition**: Make the inherited abandonment precondition and review transition explicit in FR-4’s fit criterion or a linked unwanted-behavior row. Add blocking DoD evidence that exercises a successful Forgejo abandonment with its required reason and human review, and proves that false, missing, malformed, or inaccessible policy observations cause no reason-record, branch, or PR mutation.
- **Disposition**: applied — the operator approved the two exact proposed criteria under corrected requirement problem P-8; the FR-4 fit criterion and blocking DOD-1 evidence now cover affirmative policy/reason/human-review/transition and zero writes for false/missing/malformed/inaccessible policy observations. A scope-locked re-review on the new committed digest is required; this report retains its original digest and request-changes verdict.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-10-02 |

## Escalation brief

none

## Measurements

- **Rounds**: 1
- **Findings**: 1 blocking · 0 advisory
- **Dispositions**: 1 applied · 0 declined · 0 withheld-as-violation · 0 pending author disposition

## Overlay corrections

none
