# Review: plan-new-routed-demand — design-review report

| Field | Value |
|---|---|
| **In one line** | Approve: source/generated authority, checkpoint judgment, planning admission, retained failures, and every carried plan-new binding now converge. |
| **Relevant-when** | auditing or consuming the review of plan-new-routed-demand; checking why plan/v2 supersedes plan-new-v1; tuning tech-design-review calibration |
| **Subject** | design:plan-new-routed-demand · shape: none · supersedes: plan-new-v1 |
| **Moment** | supersession |
| **Reviewer** | tech-design-review · 2026-07-29 guide |
| **Date(s)** | 2026-07-29 |
| **Verdict** | approve |
| **Tracking** | — *(the subject's introducing PR, once filed)* |
| **Template version** | design-review-report/v1 |

## Round 1

### FND-1 — the flow put the evaluator on both sides of the authored-source boundary

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: design: §3 diagram ordered judgment → evaluator → plan/v2 source → plangate recomputation — vs — design: §2, §3.2, §3.3, and #DINV-11 make plangate fold the sole evaluator and generated-profile author.
- **Acceptance condition**: redraw the flow so judgment authors raw source, plangate calls the evaluator once inside fold, and the generated face/capsule emerges afterward.
- **Disposition**: applied

### FND-2 — checkpoint source, human authority, and mechanical verification were conflated

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: design: §3.3 defined a human-completed rubric while §3.7 required recomputation of an undefined checkpoint projection — vs — `prd:plan-new/FR-30`, ABUSE-3, DOD-9/DOD-11, and design #DINV-12 require attributable human judgment bound to exact plan bytes.
- **Acceptance condition**: define the operator-completed checkpoint source, existing PR approval/merge authority, exact digest/edit invalidation, and the mechanical structure/binding checks; prohibit plangate from recomputing or declaring semantic judgment.
- **Disposition**: applied

### FND-3 — the supersession dropped the human decomposition rubric

- **Severity**: blocking (CAL-3)
- **Lens**: contract-quality
- **Evidence**: design: §3.3 and A-3 covered only demand evidence while A-5 claimed mechanically tested `FR-4..FR-13` — vs — `prd:plan-new/FR-4`, FR-9, DOD-6, and target `plan-new-v1#§3.1` require human judgment of vertical slices and compound refactor-plus-behavior scope.
- **Acceptance condition**: persist the per-task FR-4/FR-9 judgment in the one checkpoint, give it a human-judgment A-row, and narrow the mechanical A-row to enumerable checks.
- **Disposition**: applied

### FND-4 — FR-27's planning-entry gate had neither its live dependency nor a pre-draft action

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: design: the header built only on docgate-v1 and §3.7 first ran the precondition during fold — vs — `prd:plan-new/FR-27` and `contract-gate-v1#§2` require exact `docgate contract-gate <feature>` clean admission before drafting, freezing, and projection.
- **Acceptance condition**: declare the live contract-gate dependency, make its exact clean verdict plan-new's first action with zero output on bounce, surface its retained report reference, and revalidate later boundaries.
- **Disposition**: applied

### FND-5 — duplicate feature-plan admission had no extend-or-supersede path

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: design: §3.1 said only "overlap handling" and no A-row probed existing plans/epics — vs — `prd:plan-new/FR-3`, which requires plausible overlap to be surfaced before drafting with an explicit extend-or-supersede choice.
- **Acceptance condition**: bind bounded exact discovery, persist new/extend/supersede and matched identities, fail on ambiguous matches, and add the full admission fixture matrix.
- **Disposition**: applied

### FND-6 — the claimed AP-28 alignment discarded planning failure signal

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: design: §3.8 exposed failures through logs/debug output and Q-3 retained comment-only projection reports while Principles claimed AP-28 — vs — AP-28 requires failure and judgment fuel as immutable append-only git event records and exempts only clean/routine derivable output.
- **Acceptance condition**: retain fog, structural/demand, checkpoint-refusal, and failed/partial-projection events append-only in git; keep clean/regenerable output unmaterialized; prove append/no-overwrite/no-clean-event behavior.
- **Disposition**: applied

### FND-7 — the superseding CLI contract omitted incident I3's fail-closed parse guard

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: design: §3.7 rebound every plangate verb but refused only unknown schema versions/keys — vs — `docs/observability/incidents.md#I3` and target `plan-new-v1#§3.6`, where a misplaced `--dry-run` became a directory and performed forbidden writes.
- **Acceptance condition**: refuse unknown, misplaced, and flag-like positional tokens before side effects for every verb; make consequential output complete; add the incident corpus and multi-defect golden.
- **Disposition**: applied

### FND-8 — carried operational promises lacked exact bindings and acceptance

- **Severity**: advisory (CAL-4)
- **Lens**: feasibility-completeness
- **Evidence**: design: §2/§3.1 said sizing, dialect, and operations were carried while A-7/A-9 did not cover zero-at-rest/per-use cost, defect quadruples, Go hygiene, or the macOS/Linux matrix — vs — `prd:plan-new/NFR-3..NFR-6` and target `plan-new-v1#§3.9`, `plan-new-v1#§3.10`, `plan-new-v1#A-9`.
- **Acceptance condition**: restate the configured plan path, exact sizing-policy home/closed keys, dialect manifest and projection identities, and add acceptance for cost, operability, Go hygiene/race, and both supported operating systems.
- **Disposition**: applied

## Round 2 — scope-locked

All eight Round-1 findings close on the current subject. The flow now has one evaluator call inside fold; the operator rubric is the sole human checkpoint source while existing exact-head PR approval/merge remains authority; decomposition judgment and demand support coexist in that record. Contract cleanliness and overlapping-plan disposition precede drafting. Teaching failures append under the plan event home, the CLI refuses incident-I3 token shapes, and the carried sizing/dialect/operational contracts have exact mechanisms and evidence rows. No new finding was admitted. The separately tracked stale-dependent currency protocol is outside this routing subject and remains in GitHub issue #806.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-07-29 |
| 2 | approve | 2026-07-29 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 7 blocking · 1 advisory
- **Dispositions**: 8 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

none
