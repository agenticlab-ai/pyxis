# Review: plan-new amendment 004 — review report

| Field | Value |
|---|---|
| **In one line** | Approve after making semantic-evidence judgment accountable, bounding eval repetition, separating semantic and mechanical validation, aligning milestone/cutover gates, replacing wall-clock release gates with measurements, and repairing the risk ledger. |
| **Relevant-when** | auditing or consuming plan-new amendment 004; asking how plan/v2 demand is validated and approved; tuning prd-review calibration against planner-routing findings |
| **Subject** | `prd:plan-new/004` |
| **Moment** | amendment |
| **Reviewer** | prd-review · 2026-07-27 |
| **Date(s)** | 2026-07-27 (rounds 1–2) |
| **Verdict** | approve |
| **Tracking** | — *(introducing PR not filed)* |
| **Template version** | review-report/v1 |

## Round 1

### FND-1 — the checkpoint had no acceptance rubric for assessment evidence

- **Severity**: blocking (CAL-3)
- **Lens**: contract-quality
- **Evidence**: the first-fold FR-29 said the checkpoint judged substantive support, while §5.9 said deterministic validation could not substitute for that judgment — vs — §8 named rubrics only for DOD-6 and FR-4/FR-9 and said merely that assessment was visible at the checkpoint.
- **Acceptance condition**: Add a Must checkpoint rule naming the accountable human, requiring cited anchors to support every assessment axis and assurance raise, rejecting irrelevant/contradictory/partial support before freeze, and persisting a completed task-by-task rubric with the decision; bind the semantic UB class to it.
- **Disposition**: applied

### FND-2 — EVAL-2's repetition count was an unbound placeholder

- **Severity**: blocking (CAL-3)
- **Lens**: contract-quality
- **Evidence**: the first-fold EVAL-2 required decomposition to run “K times per release” with no value or governed lower bound — vs — DOD-12's blocking claim that a green EVAL-2 supplied reproducible release evidence for FR-28/FR-29.
- **Acceptance condition**: Replace `K` with a positive lower bound or governed bounded datum; record required/executed counts, and require every run of every family to meet the threshold without rerun-until-green.
- **Disposition**: applied

### FND-3 — semantic citation defects were assigned to incompatible stages

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: the first-fold §4.4 said an irrelevant citation refused the plan before checkpoint presentation — vs — FR-29 and §5.9, which assigned irrelevant/contradictory/partial-support judgment to the operator checkpoint and limited the deterministic pre-checkpoint gate to structure, resolution, and derivation.
- **Acceptance condition**: Make pre-checkpoint validation refuse only structural, citation-resolution, derivation, and portability defects; make resolvable-but-semantically-bad evidence reach the rubric and prevent approval/freeze; leave no live span assigning semantic support to both stages.
- **Disposition**: applied

### FND-4 — milestone exits omitted the human gates their scopes claimed to complete

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: the first-fold §6.1 Authoring scope included checkpoint + freeze but its exit omitted critical FR-17, while Dogfood omitted DOD-6 even though §6.4 required that operator judgment.
- **Acceptance condition**: Make Authoring exit require FR-17 and the persisted evidence rubric; make Dogfood exit require DOD-6.
- **Disposition**: applied

### FND-5 — the immutable plan/v2 cutover had no coherent activation point

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: the first-fold NR-4 and §5.4 began plan/v2 only “after activation,” while §6.5 Stage 1 froze the dogfood plan before Stage 3 “full” and §3.3 required that dogfood artifact to prove plan/v2 conformance.
- **Acceptance condition**: Name one plan/v2 schema cutover before the first Stage-1 freeze; require every plan frozen from that point to be plan/v2; distinguish the pre-release capability modes from the parent's later one-step production routing activation; align NR-4, §5.4, §6.5, and §3.3.
- **Disposition**: applied

### FND-6 — wall-clock latency was a blocking release criterion

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: the first-fold §6.4 made NFR-2's developer-hardware and tracker-inclusive elapsed thresholds part of a timed release suite — vs — AP-29 and incident I5, which require performance to remain measurement rather than correctness because identical work varies with hardware, instrumentation, and external latency.
- **Acceptance condition**: Remove elapsed-time thresholds from release and DoD verdicts; retain local/tracker timing as non-blocking measurement, and prove injected delay cannot change an otherwise identical semantic verdict.
- **Disposition**: applied

### FND-7 — readiness claimed risks that the live risk ledger did not carry

- **Severity**: blocking (CAL-4)
- **Lens**: completeness-critic
- **Evidence**: the first-fold §8 claimed assessment/derivation risks were covered and all risks owned — vs — §6.3 R1 still cited superseded FR-15 and discussed decomposition only, while no row owned routing-policy/version drift across authoring, validation, and projection.
- **Acceptance condition**: Supersede §6.3 so R1 covers decomposition and semantic-assessment quality using only live controls, and add an owned derivation-policy/version-drift risk with reproducibility, refusal, and round-trip mitigations; make §8 cite the resulting live rows.
- **Disposition**: applied

## Round 2

No further findings survived the scope-locked re-review. FR-30 and UB-7 now make the operator rubric accountable and durable; EVAL-2 runs every family at least five times on a recorded decode-pinned configuration; F1 cleanly separates mechanical pre-checkpoint refusal from semantic checkpoint judgment; Authoring and Dogfood exits include their human gates; the Stage-0 schema cutover precedes every real freeze and remains distinct from direct production routing activation; NFR-8 makes timing non-blocking; and R1/R7 plus §8 form a live, owned risk chain with no stale FR-15 citation.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-07-27 |
| 2 | approve | 2026-07-27 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 7 blocking · 0 advisory
- **Dispositions**: 7 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

none
