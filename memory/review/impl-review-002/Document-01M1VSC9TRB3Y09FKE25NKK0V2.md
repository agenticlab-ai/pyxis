# Review: impl-review amendment 002 — review report

| Field | Value |
|---|---|
| **In one line** | Approved after the scope-locked convergence round closed all eleven blocking contract/evidence findings with no changed-span regressions. |
| **Relevant-when** | auditing or consuming review of `prd:impl-review/002`; asking why its review-demand, evidence, eval, seam, or activation contracts changed |
| **Subject** | prd:impl-review/002 |
| **Moment** | amendment |
| **Reviewer** | prd-review · guide current 2026-07-27 |
| **Date(s)** | 2026-07-27 · 2026-07-27 |
| **Verdict** | approve |
| **Tracking** | — *(the introducing PR, once filed)* |
| **Template version** | review-report/v1 |

## Round 1

### FND-1 — Folded §5.9 erased every executable eval definition

- **Severity**: blocking (CAL-3 — Must FR-3/FR-4/FR-22 and blocking DoD depended on missing eval rows)
- **Lens**: contract-quality
- **Evidence**: round-1 folded §5.9 named EVAL-1..EVAL-11 but contained no eval table, while FR-3 cited EVAL-1/2/3/7 and FR-4 cited EVAL-4's comparator — vs — product vision §11, evals as a verification primitive
- **Acceptance condition**: folded §5.9 contains complete executable EVAL-1..EVAL-11 rows, including deterministic comparator definitions, with every citation resolvable.
- **Disposition**: applied — restored the complete table and comparator/run protocol in the §5.9 replacement.

### FND-2 — Eval telemetry could not prove the required run protocol

- **Severity**: blocking (CAL-3 — DOD-2 and eval-verified Must rows were unverifiable)
- **Lens**: contract-quality
- **Evidence**: round-1 §5.9 required `N≥5`, every run green, and native route provenance, but live EVT-4 carried only summary runs/model/prompt/calibration fields — vs — AP-5 traceability and AP-6 closed contractual data
- **Acceptance condition**: eval evidence records required/executed counts, per-run outcomes, harness/role/native profile, routing policy, decode config, prompts, and calibration; pass is impossible unless all required runs execute and pass.
- **Disposition**: applied — EVT-12 supersedes EVT-4 and DOD-17 supersedes DOD-2 with the full proof contract.

### FND-3 — Two fit criteria positively allowed zero assurance passes

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: round-1 FR-18 said zero fixtures build zero passes and FR-24 said zero cells produce zero passes — vs — G3, FR-21, UB-16, and `prd:planner-driven-model-routing/NR-9`'s universal model-backed review floor
- **Acceptance condition**: all nine assurance×band cells yield one or two passes; zero-pass exists only as a negative UB-16 fixture.
- **Disposition**: applied — corrected FR-18, FR-24, and EVAL-9.

### FND-4 — Token spend was incorrectly an acceptance verdict

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: round-1 §3.2 called cost an operating measurement while NFR-6 made an overrun non-accepting — vs — AP-29 and incident I5, measurements record and never gate
- **Acceptance condition**: completed passes remain semantically valid despite measured overspend; a mechanical cap that prevents completion is an absent-pass failure, not a measurement verdict.
- **Disposition**: applied — NFR-6 now records descriptive spend and routes incomplete capped work through UB-9.

### FND-5 — Live timing/spend instrumentation used retired IDs and wrong semantics

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: round-1 EVT-7 fed human M4 from model wall time; NFR-7 lacked aggregate timing; NFR-4/NFR-5/EVT-5 cited superseded FR-13/EVT-1 — vs — G4/M4 and AP-5
- **Acceptance condition**: M4 has an explicit human capture source; aggregate process timing is unambiguous; audit/spend/checkpoint rows cite live events and requirements only.
- **Disposition**: applied — corrected M4 and EVT-7/9, superseded NFR-4/5 with NFR-10/11, and superseded EVT-5 with EVT-11.

### FND-6 — Production activation had multiple disagreeing predicates

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: round-1 §3.3, Phase C, §6.5, and the launch gate each named a different subset; live blocking DoD rows were not universally required — vs — product vision §1's machine-decidable definition of done
- **Acceptance condition**: one activation predicate requires every live blocking DoD, launch readiness, both downstream seams, and the parent full seam gate; every release section cites it identically.
- **Disposition**: applied — §3.3 defines the single predicate, EVT-11 records it, and §§6.1/6.4/6.5/6.6 reference it.

### FND-7 — HUMAN-lane forcing was claimed while its consumer remained owed

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: J4/FR-7 claimed operational HUMAN forcing while NR-5 said the merge-lane `Downgrader` adapter was unimplemented and §6.2 omitted it — vs — AP-11 and the live NR-5 boundary
- **Acceptance condition**: the consuming adapter is an explicit pre-activation dependency with end-to-end HUMAN-forcing evidence, or the job/release claim is narrowed to signal emission.
- **Disposition**: applied — §6.2 marks the adapter unimplemented, DOD-14 and §6.6 require end-to-end proof, and FR-31 binds post-activation forcing to it.

### FND-8 — PR #754 was falsely described as delivering the review invocation seam

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: round-1 §6.2 marked the VerificationGate review carrier/invocation seam merged in #754 — vs — merged `prd:implement-now` §4.2, which excludes review-role resolution/pass orchestration and adds only implementation-demand consumption
- **Acceptance condition**: mark the seam unimplemented and require a typed implement-now amendment carrying planned assurance + selected review harness and returning the aggregate before activation.
- **Disposition**: applied — corrected §6.2 and made the seam blocking in DOD-14/§6.6.

### FND-9 — Proposal-only retention violated the parent git-evidence contract

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: round-1 FR-27/§5.4 made GitHub/Carina proposal payloads the retained source — vs — `prd:planner-driven-model-routing/FR-21` and AP-28 requiring immutable append-only git event records
- **Acceptance condition**: every route/failure/outcome/pass/aggregate/invalidation is retained append-only in git; proposal payloads are linked lossless projections; reruns never overwrite.
- **Disposition**: applied — revised FR-27/FR-30, NFR-10, DOD-13, and §§5.4–5.5.

### FND-10 — Review events were incompatible with the canonical parent evidence chain

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: round-1 EVT-9 omitted canonical task/proposal/pass-attempt fields and widened its closed result enum; EVT-7 collapsed resolution and pass completion with no parent EVT-2/3→EVT-5 outcome chain — vs — `prd:planner-driven-model-routing/FR-20`, `FR-24`, and EVT-2/3/5/9
- **Acceptance condition**: child evidence is a schema-compatible additive projection or a lossless tested adapter to the canonical chain, including task identity, exactly one outcome only for resolved routes, and actual-launch agreement with the route.
- **Disposition**: applied — aligned EVT-8/9/10, linked EVT-7 to canonical route/outcome refs, strengthened FR-25/FR-27, and added real-runtime EVAL-11 launch fidelity.

### FND-11 — Declared blast radius omitted a new critical failure row

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: round-1 amendment added UB-17 but declared only UB-9..UB-16 — vs — amendment blast-radius honesty and the parent post-defect invalidation contract
- **Acceptance condition**: the declaration includes UB-17 and every other operation/folded replacement introduced while closing review findings.
- **Disposition**: applied — expanded the declaration through all live new/superseded IDs and sections.

## Round 2

No findings. The scope-locked convergence review closed FND-1 through FND-11 and found no regression in any changed span. The integrity lens revalidated the executable evaluation corpus and proof telemetry; the coherence lens revalidated the universal review floor, descriptive-only measurements, single activation predicate, and explicitly owed HUMAN-lane consumer; the upstream-alignment lens revalidated the unimplemented implement-now carrier, immutable git evidence chain, canonical parent event compatibility, and complete blast declaration.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-07-27 |
| 2 | approve | 2026-07-27 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 11 blocking · 0 advisory
- **Dispositions**: 11 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

none
