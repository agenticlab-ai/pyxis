# Review: e2e-verification-artifacts — design-review report

| Field | Value |
|---|---|
| **In one line** | request-changes (round 1) on two blocking findings — the e2e review report cannot reuse prd-review's PRD-typed `review-report/v1` verbatim (it needs its own instance-schema in the review-report family), and the plan reuse mis-homes/under-specifies the e2e plan against plan-new's `docs/plans/` + freeze-at-merge pipeline — plus five advisories (a DINV-2 overclaim, a v1-untestable freeze clause, a leaked idempotency test, a DINV-6 citation, and an NR-1 `Serves` omission). |
| **Relevant-when** | auditing or consuming the review of the `e2e-verification-artifacts` design; asking why it bounced; tuning tech-design-review calibration |
| **Subject** | design:e2e-verification-artifacts · shape: present · supersedes: — |
| **Moment** | new-design |
| **Reviewer** | tech-design-review · 2026-07-24 |
| **Date(s)** | 2026-07-24 |
| **Verdict** | approve-with-advisories *(round 2; history below is authoritative)* |
| **Tracking** | — *(the subject's introducing PR)* |
| **Template version** | design-review-report/v1 |

## Round 1

*Generated through four blind fresh-context lenses over the two paired designs and their shared solution-shape (`design-brainstorm:e2e-verification-tech-design`). The `lint-tech-designs` floor is excluded.*

### FND-1 — the e2e review report cannot be a verbatim `review-report/v1` instance; that schema is prd-review's, PRD-typed

- **Severity**: blocking (CAL-4 — feasibility gap load-bearing for A-3 and the §2/DINV-5 "no new schema" claim)
- **Lens**: feasibility-completeness
- **Evidence**: `design:` "Review report — a `review-report/v1` instance … it adopts the existing shape verbatim" (§3.2) · "neither introduces a new schema" (#DINV-5) · A-3 "the e2e review report validates against `review-report/v1`" — vs — `review-report/v1` is prd-review's schema with a **closed** header typed to PRDs (`Subject: prd:<feature>/000|NNN`, `Moment: new-base|amendment`, `Reviewer: prd-review`); the reviewer program gives each reviewer class its **own** report schema (`design-review-report/v1`, `impl-review-report/v1`), and `design:tech-design-review-artifacts` §4 explicitly rejects reusing the sibling's schema with a subject switch. The e2e review's subject is an e2e **design doc** with coverage/anti-gaming lenses — which that closed PRD header cannot admit.
- **Acceptance condition**: mint an `e2e-review-report/v1` in the review-report family (subject `e2e-design:<feature>`, its own coverage/anti-gaming calibration), mirroring the record-class pattern; drop the "no new schema" / "verbatim" claim for the report (keep it for the plan only — see FND-2); update §2, §3.2, #DINV-5, R3, A-3. The record's D-2 (which said "review-report/v1 instance") gains a review-anchored refinement to "an instance in the review-report family."

### FND-2 — the plan reuse mis-homes and under-specifies the e2e plan against plan-new's actual pipeline

- **Severity**: blocking (CAL-4 — feasibility gap load-bearing for A-5 and the FR-15 Must, and for the NR-1 "reuses, never edits" claim)
- **Lens**: feasibility-completeness
- **Evidence**: `design:` "the plan reuses plan-new's frozen-plan + idempotent tracker-projection dialect … The plan freezes at **operator approval** … re-projection creates no duplicate tracker items" (§3.2) · "`docs/e2e/<feature>/` … `plan.md` (the frozen plan artifact)" (§3.3) · §7 Q2 leaves "whether the frozen `plan.md` artifact is a full plan/v1 instance or a thinner manifest" **undecided** — vs — `design:plan-new-v1` §3.1/#DINV-1: plans live at `docs/plans/<feature>/NNN-plan.md`, **freeze = merge** ("`plangate project` refuses any plan not reachable as merged content"), and idempotent projection is `plangate project` over a folded `plan/v1`. As written, the e2e plan's home (`docs/e2e/`), freeze event (approval vs merge), and undecided plan/v1-ness leave the FR-15 idempotency mechanism unbound, and any divergence from plan-new's home/freeze would *edit* the reuse (violating NR-1).
- **Acceptance condition**: resolve Q2 in-design — the e2e plan **is** a plan-new `plan/v1`, authored by e2e-plan from the approved e2e design doc, homed at `docs/plans/<feature>/`, frozen at its plan-PR merge (= the operator's plan acceptance, exactly plan-new's lifecycle), and projected by `plangate project` verbatim; the e2e feature dir `docs/e2e/<feature>/` then holds `design.md` + `review.md` only. Update §2, §3.2, §3.3, #DINV-4/5, R2, A-3/A-4/A-5, and align `design:e2e-verification-skills` §3.3.

### FND-3 — DINV-2 asserts coverage *completeness* the artifact class cannot self-enforce, and cites FR-4

- **Severity**: advisory (CAL-3 — a binding invariant overclaims its enforcement surface; substance holds, so not blocking)
- **Lens**: contract-quality
- **Evidence**: `design:` "**DINV-2** — every required behavior appears in the coverage map … a behavior absent from **both** the coverage map and the routes is invalid — the no-silent-drop property (`prd:e2e-verification/FR-4`)" (§3.5) and R1 crediting "the template's structure checklist (`#DINV-1/2/3`)" with catching it — vs — a structure checklist sees only what is *in* the doc; detecting a required behavior wholly *absent* needs the independently-derived required-behavior set, which is the coverage **lens's** judgment (`design:e2e-verification-skills` §3.2 · EVAL-1). And §3.1 assigns FR-2 to the coverage map, FR-4 to the route — so DINV-2's FR-4 citation is crossed.
- **Acceptance condition**: reword DINV-2 to the well-formedness property the checklist can enforce (every coverage-map entry present, carrying ≥ 1 scenario or a routed disposition), re-cite it to FR-2, add FR-4 to DINV-3's route clause, and reword R1/§3.3 so completeness-against-the-required-set is credited to the coverage lens, not the authoring checklist.

### FND-4 — DINV-4 / A-4's "successor, never in place" freeze clause is un-exercisable in v1

- **Severity**: advisory (CAL-3 — one A-row clause is untestable in v1; the row's other clauses are testable, so not wholly blocking)
- **Lens**: contract-quality
- **Evidence**: `design:` "#DINV-4 — thereafter it changes only by a successor doc, never in place" and A-4 "a post-approval change is a successor doc, not an in-place edit — Verify: Inspection, Test — evidence: … freeze fixture" (§3.5/§6) — vs — the successor-authoring path is deferred out of contract (`shape:` `#O-2`) and immutability enforcement is deferred docgate governance (`design:` §3.3, §7 Q1) — so there is no v1 mechanism to exercise "never in place." (The FR-15 citation on DINV-4 supports only its plan-byte-stability clause.)
- **Acceptance condition**: split A-4 so its v1-enforceable clauses (absent from the generated index; plan artifact byte-stable via the reused dialect) are what "Test" covers, and restate the design-doc "successor, never in place" as a forward mutability posture whose enforcement (§7 Q1) and successor path (`#O-2`) are explicitly deferred; correct the FR-15 citation to the plan clause only.

### FND-5 — A-5's "re-projection creates zero duplicates" is a runtime behavior test the schema design scoped out

- **Severity**: advisory (CAL-3 — partition honesty; a smell, not a correctness hole)
- **Lens**: contract-quality
- **Evidence**: `design:` A-5 "a re-projection of the plan creates zero duplicate tracker items — Verify: Test" (§6) while Scope declares "**Not**: the three skills' behavior that produces/consumes these artifacts" — vs — that test necessarily runs e2e-plan's projection (behavior), which `design:e2e-verification-skills` A-5 already owns; RW-5 / `shape:` `#D-11` place behavior in the skills design.
- **Acceptance condition**: in the artifacts design assert only the static reuse binding (the plan conforms to plan-new's `plan/v1` + projection dialect, shape-verifiable), and leave the runtime idempotency acceptance to `design:e2e-verification-skills` (which already carries it). Folds together with FND-2's resolution.

### FND-6 — §3.4 cites `#DINV-6` for "not docgate-governed," but DINV-6 asserts a different property

- **Severity**: advisory (CAL-3 — a binding-element cross-reference is wrong; the claim is true and supported elsewhere)
- **Lens**: contract-quality
- **Evidence**: `design:` "Implementation must not infer — that the class is docgate-governed (it is not, in v1 — `#DINV-6`)" (§3.4) — vs — "#DINV-6 — the e2e design doc is not a technical design: it is absent from the generated design index and carries no `Serves`/`Supersedes` semantics" (§3.5): DINV-6 says nothing about docgate governance (the ungoverned-in-v1 property lives in §3.3 / §2 / §7 Q1).
- **Acceptance condition**: point that boundary item at §3.3 / §7 Q1 (which do state ungoverned-in-v1), or add an invariant asserting the class is docgate-ungoverned in v1 and cite it.

### FND-7 — §3.4 commits the NR-1 non-regression in substance, but NR-1 is absent from the `Serves` header

- **Severity**: advisory (CAL-2 — downgraded from the default: NR-1 is served by `design:e2e-verification-skills`, so there is no inverse-coverage hole; per incident I4 an isolated `Serves`-completeness gap is advisory)
- **Lens**: upstream-alignment
- **Evidence**: `design:` §3.4 "Must remain unchanged — … `review-report/v1`; plan-new's projection dialect and the task/charter schema — this design **reuses**, never edits, all three (`prd:e2e-verification/NR-1`)" — vs — the `Serves` header "FR-1 · FR-2 · FR-4 · FR-5 · FR-11 · FR-14 · FR-15" omits NR-1, the exact I4 "one-line Serves omission" that is trivial pre-merge and a supersession post-freeze.
- **Acceptance condition**: add `prd:e2e-verification/NR-1` to the artifacts `Serves` row (it genuinely protects the reused `review-report`-family + task/charter schemas from edits, per §3.4).

## Round 2

*Terminal re-review, scope-locked to the round-1 findings and regressions in their changed spans. The `e2e-verification-artifacts` design was rewritten to apply all seven.*

- **Blocking — both closed.** FND-1: the review report is now its own `e2e-review-report/v1` instance in the review-report family (§2, §3.2, #DINV-5, A-3); the "verbatim `review-report/v1`" claim is gone, and the record's D-2 report-half is refined by D-13. FND-2: the e2e plan is now an ordinary plan-new `plan/v1` homed at `docs/plans/`, frozen at its plan-PR merge, projected by `plangate project` verbatim (§2, §3.2, §3.3, #DINV-4/5, A-3/A-4/A-5); §7 Q2 is resolved and `design:e2e-verification-skills` §3.3 is aligned.
- **Advisories — applied.** FND-3: DINV-1 now carries well-formedness and DINV-2 the no-silent-drop-within-the-authored-set, citations corrected (FR-2/FR-4), R1/§3.3 crediting the coverage lens for completeness. FND-4: A-4 is split — index-absence + plan byte-stability tested, the design-doc successor-never-in-place posture explicitly deferred. FND-5: A-5 asserts static home/conformance; the runtime idempotency test lives in `design:e2e-verification-skills` A-5. FND-6: §3.4's not-governed item points at §3.3/§7 Q1. FND-7: `NR-1` added to `Serves`.
- **Regressions in changed spans**: none. The `e2e-review-report/v1` family instance and the plan-new `plan/v1` reuse are consistent across §2/§3.2/§4/#DINV-5/A-3; the split homes (`docs/e2e/` design+review, `docs/plans/` plan) are stated in §3.3/A-5.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-07-24 |
| 2 | approve-with-advisories | 2026-07-24 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 2 blocking · 5 advisory
- **Dispositions**: 7 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

none
