# Review: feature-draft-reopen — design-review report

| Field | Value |
|---|---|
| **In one line** | Approve after round 2. Round 1's blocking finding: the lock trusted that a reopened draft started from the binding version's text, reading the starting version from the branch name, so a hand-seeded draft could lock changes its approval request never lists. The lock now checks the first commit's text by content. Three advisories were applied: every row ID is covered by the no-reuse rule, a no-change lock is refused (the operator's ruling), and the history clause has evidence. |
| **Relevant-when** | auditing or consuming the review of feature-draft-reopen; asking why it was approved or bounced; tuning tech-design-review calibration |
| **Subject** | design:feature-draft-reopen · shape: none · supersedes: — |
| **Moment** | new-design |
| **Reviewer** | tech-design-review · `.pyxis/authoring/tech-design-review.md` as of 2026-09-24, run in the authoring session |
| **Date(s)** | 2026-09-24 (round 1) · 2026-09-24 (round 2) |
| **Verdict** | approve |
| **Tracking** | — *(the introducing PR, once filed)* |
| **Template version** | design-review-report/v1 |

## Round 1

### FND-1 — The lock trusts the seed

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: design: "The records a lock writes are exactly the change its approval request lists" (§2) and "The starting version is the one below the version in the draft's branch name" (§3.6) — vs — `prd:feature-draft/UB-1` (every requirement change made during a draft is listed in the approval request) and AP-18 (observed, not declared). No binding element checks that the draft's first commit carries the binding version's text. The change list is computed from the branch's history since its first commit (`design:feature-draft-v1` §3.6). So a draft seeded by hand, or from an older version, can revert the binding version's rows without any listed change. Under the differences-only rule, the lock then writes those reverts.
- **Acceptance condition**: lock preparation verifies, by content, that the draft branch's first commit carries exactly the binding version's stored PRD and tech-spec text, and refuses otherwise. #DINV-6 is restated on that observation, and the branch-name derivation is dropped. An A-row covers a draft that started from an older version and one seeded by hand.
- **Disposition**: applied — §3.6 checks the first commit's text by content and names the version it matches; #DINV-6, A-7, R4, and §3.9 agree, and the branch-name derivation is gone

### FND-2 — ID reuse covers only requirement and decision rows

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: design: "the ID of a requirement or tech-spec decision row, is never reused across versions" (#DINV-4) — vs — solution shape S-3 of `design:feature-draft-v1`: question and risk rows compile to per-feature families too (`prd/<feature> · Q<n>`, `· R<n>`). A question or risk ID retired at one version and reused later would land in the retired record's family.
- **Acceptance condition**: #DINV-4, §3.7, and A-4 cover every PRD and tech-spec row ID.
- **Disposition**: applied — #DINV-4, §3.7, and A-4 cover every PRD and tech-spec row ID

### FND-3 — A reopened draft with no change

- **Severity**: advisory (CAL-4)
- **Lens**: feasibility-completeness
- **Evidence**: design: "an unchanged unit writes nothing" (#DINV-2) — vs — `prd:feature-draft/FR-19` (a lock makes the PRD and tech spec the feature's next locked version). The design does not say what lock preparation does when the PRD and tech-spec text equal the binding version's: refuse, or lock an identical version N+1 that writes nothing.
- **Acceptance condition**: the design states the zero-change behavior, with an acceptance row.
- **Disposition**: applied — the operator ruled to refuse (`design-brainstorm:feature-draft-tech-design-2#D-3`); §3.6, #DINV-7, A-8, §3.9, and §4 state it

### FND-4 — The history clause has no evidence

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: design: "a later version never supersedes or revokes a problem, brainstorm record, review report, or event" (#DINV-3) — vs — A-2, whose fixture does not require the binding version to carry any history. A comparison that revokes every record absent from the draft would pass A-2 whenever the fixture holds no history.
- **Acceptance condition**: A-2's binding version carries a problem, a brainstorm record, a review report, and an event, and A-2 asserts that none is superseded or revoked.
- **Disposition**: applied — A-2's binding version carries a problem, a brainstorm record, a review report, and an event, and A-2 asserts none is superseded or revoked

## Round 2

Scope-locked to the spans changed for FND-1 to FND-4.

- FND-1 — closed: §3.6 observes the draft's starting point from its first commit's content. #DINV-6, A-7, R4, and §3.9 agree, and §2's claim that the lock writes exactly the listed change now holds by #DINV-6.
- FND-2 — closed: #DINV-4, §3.7, and A-4 name every PRD and tech-spec row ID.
- FND-3 — closed: §3.6 refuses a no-change lock with the abandon remedy per the operator's ruling; #DINV-7, A-8, §3.9, §4, and §0 agree.
- FND-4 — closed: A-2 requires history in the binding version and asserts that none of it is superseded or revoked.

No new findings: the changed spans agree with §2, the boundaries, and each other, and a first version's lock is untouched.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-09-24 |
| 2 | approve | 2026-09-24 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 1 blocking · 3 advisory
- **Dispositions**: 4 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

none
