# Review: prd:implement-now/006 — review report

| Field | Value |
|---|---|
| **In one line** | Approve, round 2 — the projection repair now renders the exact live requirement set, carries rollout proof through one activation predicate, resolves its provenance, and makes the frozen design's successor-lineage repair explicit. |
| **Relevant-when** | auditing or consuming the review of implement-now amendment 006; asking why FR-27 was replaced by FR-48; tuning prd-review calibration against projection-repair findings |
| **Subject** | `prd:implement-now/006` |
| **Moment** | amendment |
| **Reviewer** | prd-review · `.pyxis/authoring/prd-review.md` (run inline by the authoring session, bootstrap-era) |
| **Date(s)** | 2026-08-20 |
| **Verdict** | approve |
| **Tracking** | — *(the amendment's introducing PR, once filed)* |
| **Template version** | review-report/v1 |

## Round 1

### FND-1 — The repaired functional-requirement projection still disagreed with the live ledger

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: the first repair draft's §5.1 retained source-retired FR-12, omitted source-live FR-20, and made DOD-35 and §8 call that set every live Must requirement — vs — `docs/prds/implement-now/effective.source.json`, where FR-12 is superseded through FR-39→FR-47 and FR-20 remains live.
- **Acceptance condition**: render FR-20 exactly once and FR-12/FR-27 zero times in live §5.1; make DOD-35 and §8 enumerate the exact live Must set; replace retired FR-34/FR-39 job citations with live replacements; prove every rendered live ID and source-map live ID agree.
- **Disposition**: applied — §5.1 restores source-owned FR-20, removes retired FR-12, and contains one FR-48; DOD-35 and §8 now use the exact live set and live job mappings.

### FND-2 — Activation still had competing predicates and a contradictory rollback rehearsal

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: the first repair draft's §3.3 required M1–M7, every blocking DoD, and every guardrail, while §6.5 authorized through a narrower DoD/dependency subset and the inherited §6.6 rehearsed a don't-invoke shortcut — vs — §3.3's pre-registered single ship rule and §8's fail-closed readiness gate.
- **Acceptance condition**: define one non-recursive activation predicate containing M1–M7, the exact live blocking-DoD set, every guardrail, all required §6.2 dependencies, concrete launch readiness, and the real rollback rehearsal; make §6.5 and §6.6 refer only to it.
- **Disposition**: applied — §3.3 is explicitly sole, names concrete support/docs/platform completion and rollback rehearsal, and §6.5/§6.6 no longer authorize through alternate subsets.

### FND-3 — Retiring FR-27 left frozen downstream design lineage stale and undeclared

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: `design:implement-now-routed-execution` still serves `prd:implement-now/FR-27` in its frozen header and A-8/A-9 while the first repair draft neither declared that consumer nor constrained its successor — vs — incident I6, which permits only whole-document supersession for frozen-design lineage repair.
- **Acceptance condition**: declare the frozen design in the blast radius and require its already-owed whole-document successor to replace local FR-27 Serves/acceptance lineage with FR-48 before planning or code, without editing the frozen design.
- **Disposition**: applied — the blast radius names the design and §6.2 makes the successor-lineage replacement a pre-planning/pre-code dependency. The §7.3 FR-27 citation remains unchanged because it is explicitly qualified to the sibling planner-driven-model-routing PRD, where FR-27 is live.

### FND-4 — Learned-from did not resolve to the evidence that caused the repair

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: `review-thread:implement-now-006-projection-congruence` named no durable artifact — vs — the recorded discovery and operator choice in the session decision log and the amendment model's resolvable-provenance requirement.
- **Acceptance condition**: replace the label with a durable reference to the recorded projection defect and operator decision.
- **Disposition**: applied — Learned-from now resolves to `operator-decision:logs/2026-08-20-1153-implement-now-amendment-006.md#P-1`.

## Round 2

No new findings. The scope-locked re-review confirmed:

- folded §5.1 contains each source-live FR exactly once after applying FR-27→FR-48 and contains no source-retired FR;
- DOD-35, §3.3, and §8 enumerate the exact live Must/blocking sets and use only live local job citations;
- §3.3 is one non-recursive activation predicate and §6.5/§6.6 exclusively consume it while rehearsing the actual evidence-preserving fallback and review-only recovery;
- the provenance anchor resolves, and §6.2 carries the incident-I6-compliant successor-design repair before planning or code; and
- the unchanged §7.3 FR-27 belongs to the immediately qualified parent routing PRD, so it is not stale local lineage.

No contract-quality or completeness-critic candidate survived adversarial scrutiny in either round. §0 was excluded as human-only.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-08-20 |
| 2 | approve | 2026-08-20 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 4 blocking · 0 advisory
- **Dispositions**: 4 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

| Correction | Found by | Date |
|---|---|---|
| The approved draft's §5.1 restatement restored FR-20 using the base row's longer rationale, silently changing the shorter live rationale without a typed FR-20 supersession. Amendment 006 now restores FR-20 byte-faithfully as `the operational binding lives on the map`. Pending PR #1091 compares complete row content when the same ID exists on both sides of a section supersede, but its grandfathered-debt path cannot authenticate the content used to restore an already-missing live row; that narrower follow-up belongs on #1091 rather than in this repair. | human review, PR #1094 | 2026-08-20 |
