# Planner-driven model routing — reopened design-brainstorm record

| Field | Value |
|---|---|
| **In one line** | Resume the amended routing design at the review, legacy-translation, retained-evidence, and direct-cutover seams after removal of the nonexistent `no_inference` executor mode. |
| **Relevant-when** | designing or changing the execution-demand schema or policy, plan/v1 translation, harness-native implementation/review resolution, review assurance joining, routing evidence, or direct-cutover mechanics; checking the amended-basis rulings and alternatives that follow the frozen first design-brainstorm record |
| **Basis** | prd:planner-driven-model-routing (merged ledger as of amendment 001, PR #737, merge commit `20f2227dc5ae0a68fee14e955d4f8fe4462e15a7`, 2026-07-26) |
| **Authority** | @mdarabi |
| **Date(s)** | 2026-07-26 |
| **Status** | gated: needs-more-product |
| **Tracking** | — |
| **Feeds** | — |
| **Prior records** | brainstorm:planner-driven-model-routing · design-brainstorm:planner-driven-model-routing-tech-design · design-brainstorm:implement-now-tech-design · design-brainstorm:impl-review-tech-design |
| **Template version** | design-brainstorm-record/v1 |

## Orientation

| Source | Why loaded |
|---|---|
| docs/prds/planner-driven-model-routing/effective.md | The amended merged basis: every plan/v2 `/implement-now` task is model-backed; FR-22/FR-23 replace the conditional assessment rows; FR-24 adds typed plan/v1 origins and unambiguous demand/attempt joins; NG-8 rejects a deterministic executor recipe. |
| docs/brainstorms/planner-driven-model-routing-tech-design.md | The frozen predecessor: D-1..D-4 chose the partition, derivation policy, native catalogs, and sealed transport; D-5 removed `no_inference`; D-6 bounced the contradiction; old DQ-5 alternatives and DQ-6..DQ-8 are the reopened frontier. |
| docs/brainstorms/planner-driven-model-routing.md | The stage-1 source of product direction and vocabulary; its planner-owned semantic-demand and adapter-owned native-resolution boundaries remain governing context through the merged PRD. |
| docs/tech-design/index.md | The live-design routing surface; no live design Serves the amended PRD, while the predecessor contracts below remain the concrete overlaps. |
| docs/tech-design/routing-envelope-v1.md | The optional scalar rung envelope directly contradicted by universal plan/v2 execution demand and therefore a supersession target. |
| docs/tech-design/plan-new-v1.md | The frozen plan/v1 contract and projection seam that plan/v2 extends without rewriting v1. |
| docs/tech-design/implement-now.md | The current one-AgentRuntime-launch Worker and `TierMap(rung)` design; selector and executor continuity remain, but the rung-to-native decision must be superseded. |
| docs/tech-design/impl-review-v1.md | The fresh sibling, revision binding, report persistence, and merge-lane consumption that review routing must extend without weakening independence. |
| docs/tech-design/merge-lane-v1.md | The sole actual-proposal classifier; review routing consumes its `skip|standard|deep` band and may only raise planned assurance monotonically. |
| docs/architecture-principles.md | AP-1/AP-3 adapter ownership, AP-6 contracts, AP-10/AP-11 single mechanical rules, AP-13/AP-15/AP-17 exact fail-closed evidence, AP-20 narrow blast, AP-21 judge once, AP-24 governed policy, AP-28 retained fuel, AP-29 gate/measurement separation, and RW-4/RW-5/RW-6. |
| docs/observability/incidents.md | I1–I5 checked: requirements-before-design, mainline ancestry, closed consequential CLI parsing, inverse Serves coverage, and one hermetic gate predicate constrain the remaining seams. |
| docs/wiki/glossary.md | Reuses `execution demand`, `execution profile`, `assurance floor`, and `harness adapter`; no new term is needed yet. |

**Overlaps surfaced:**

- Amendment 001 closes predecessor P-1/P-2: the live PRD now retires FR-2/NR-5, adds NG-8/NR-8, and requires the full assessment/profile on every plan/v2 task entering `/implement-now`.
- The predecessor is frozen and remains provenance, not a source of new rulings. Its live D-1..D-4 directions still align with the amended basis; D-5 is now effective product contract; DQ-5 is unblocked and DQ-6..DQ-8 remain intentionally unanswered.
- No live design Serves `prd:planner-driven-model-routing`. `routing-envelope-v1` and `implement-now` contain binding decisions this feature makes false; `plan-new-v1`, `impl-review-v1`, and `merge-lane-v1` retain still-valid boundaries that the new design set must build on or extend.
- The amended PRD still declares typed amendments to the `plan-new`, `implement-now`, and `impl-review` ledgers as a Phase-A design-handoff dependency; none has landed yet. This does not prevent solution exploration, but it blocks a final `Serves` set and ready-for-design handoff.

## Design questions

- **DQ-1**: With every executable task now carrying a full profile, which seam should own the planned-assurance × proposal-band join, legal review skip, review-harness inheritance/override, independent `role=review` resolution, and evidence identity without duplicating the retained routing-evidence store?
- **DQ-2**: Where should plan/v1 compatibility translation live, what exact historical-rung table and `legacy_unassessed` output should it produce, and how should its stable `demand_id` prove source immutability across implementation, retry, and review attempts?
- **DQ-3**: What owns the immutable routing decision/outcome event class, its `demand_id`/`attempt_id`/proposal joins, committed storage, invalidation, and late-defect remediation brief?
- **DQ-4**: What exact landing and direct-production cutover sequence removes the hardcoded pin and incompatible rung ladders only after complete conformance, dogfood, fallback rehearsal, and DOD-9 are green?

## Panel log

- **DQ-1**: opens/replaces — fired — three panels independently optimized for minimal integration, seam purity/reuse, and retry/orphan evidence integrity; their useful constraints converge in ALT-4 while differing on whether the join belongs beside `implreview/band`, in a proof-carrying transition chain, or behind an append-first admission reducer.

## Decisions

### D-1 — Every routed implementation receives at least one review round

**Question:** DQ-1 — May the lowest consequence level complete with a zero-round review skip, or is there a universal review floor whose process grows with risk?

**Answer:** Every implementation proposal receives at least one model-backed review round. A zero-round `skip` is not a valid completed-review disposition for planner-driven routing. The lowest risk level receives the minimum review process of one round; greater risk requires monotonically more review process and/or rounds. The exact higher-risk mapping is product policy and remains unresolved until the governing PRD is amended.

**Reasoning:** Review is a universal quality floor. Consequence classification may strengthen that floor, but may not remove it.

**Authority:** 2026-07-26 · “let's keep at least one round of reviews for even the lowest \"risk\" level task; (this is a floor for reviewing. more risk means more process/reviews)”

**Status:** live amendment direction — not effective until `prd:planner-driven-model-routing` is amended and merged

## Alternatives

### ALT-1 — extend the existing impl-review band seam

- **Shape**: Keep `implreview/band.Select` as the lane-verdict projection and add pure `band.ApplyFloor(actual, planned_assurance) → ReviewDecision`. Change the review factory to carry `{harness_id, runtime}`; skip bypasses harness/runtime/model work, while invoke resolves `role=review` and writes through the future DQ-3 evidence port.
- **Trade-offs**: Smallest migration and no new coordinator, ticket, or store. It makes the band package own two distinct policies—actual-diff projection and planned-demand joining—and any direct/orphan caller must depend on that implementation-specific package.
- **Principles**: AP-10/AP-11/AP-20/AP-21 strong through one pure adjacent predicate; AP-1/AP-3/AP-6/AP-28 preserved through typed bindings and the single evidence owner; RW-5 strained because two independently revisable rules share a package.
- **Incidents**: I1 basis is now merged but G-1 remains; I2 exact-binds demand/head; I3 closes band/assurance/override input; I4 requires complete Serves; I5 requires one hermetic `ApplyFloor` corpus.
- **Verdict**: rejected by D-1 — it carries forward a zero-round skip path.
- **From**: DQ-1 · minimal-integration panel

### ALT-2 — proof-carrying stateless review-admission chain

- **Shape**: A thin host sequences narrow typed transitions: unchanged merge-lane classification and `band.Select`; pure `reviewdemand.Join`; review-harness binding (`override`, else recorded implementation harness); shared native resolver with `role=review`; ephemeral `ReviewLaunchPermit`; fresh-sibling invocation; and the single DQ-3 evidence writer. Intermediate values carry identities/digests but are never durable stores.
- **Trade-offs**: Strongest seam purity, reuse, and unrepresentable invalid launch ordering without ALT-17's durable ticket subsystem. It adds more typed plumbing and changes the current factory to preserve harness identity.
- **Principles**: AP-1/AP-3/AP-5/AP-6/AP-10/AP-11/AP-13/AP-15/AP-17/AP-20/AP-21/AP-22/AP-24/AP-28/AP-30 and RW-4/RW-5 strongly honored.
- **Incidents**: I1 constrained by G-1; I2 binds exact head/content; I3 closes tagged transitions; I4 requires complete FR/UB/NR coverage; I5 fixes one join and transition corpus across desk/CI.
- **Verdict**: candidate only after the product amendment — its proof-carrying separation remains useful, but its zero-round skip arm is rejected by D-1.
- **From**: DQ-1 · seam-purity/reusability panel

### ALT-3 — event-first review admission

- **Shape**: A pure admission reducer consumes verified demand, exact proposal snapshot, head-bound lane verdict, authorized review-backend config, and prior routing events. The single DQ-3 journal appends a content-addressed review decision before side effects; invoke mints a fresh `attempt_id`, while legal skip writes a revision-bound zero-round terminal with no native route attempt.
- **Trade-offs**: Strongest replay, retry/orphan recovery, and crash-boundary behavior with no second ticket store. Every launch now depends on a successful git append/CAS protocol and the reducer risks becoming a broad coordinator if its inputs are not split into narrow transition owners.
- **Principles**: AP-5/AP-6/AP-10/AP-11/AP-13/AP-15/AP-17/AP-21/AP-22/AP-23/AP-28 and RW-4/RW-5 strongly honored; AP-20 requires the reducer remain thin.
- **Incidents**: I1 constrained by G-1; I2 binds exact head, not status; I3 closed-parses events/override; I4 requires union Serves; I5 uses one reducer predicate locally and in CI.
- **Verdict**: candidate only after the product amendment — its evidence-integrity properties remain useful, but any zero-round terminal path is rejected by D-1.
- **From**: DQ-1 · retry/orphan/evidence-integrity panel

### ALT-4 — proof-carrying chain with one append-first admission boundary

- **Shape**: Adopt ALT-2's narrow stateless owners and add one DQ-3-owned append boundary before launch. `reviewdemand.Join` alone computes the nine-cell monotone table; review-harness binding alone applies explicit override else recorded implementation harness; the shared resolver alone maps `{fixed harness, role=review, joined profile}`; an ephemeral permit exists only after exact demand/head/lane identities and the decision reference are durably appended. Skip writes a head-bound zero-round impl-review projection referencing that decision and creates no native `attempt_id`.
- **Trade-offs**: Preserves seam purity, independent review capacity, retry/orphan replay, and one retained store; invalid order is structurally difficult. It costs several small closed types, an append-before-side-effect dependency, a report-schema change for zero-round skip, and a review factory that carries harness identity.
- **Principles**: AP-1/AP-3/AP-5/AP-6/AP-10/AP-11/AP-13/AP-15/AP-17/AP-20/AP-21/AP-22/AP-24/AP-28/AP-30 and RW-4/RW-5 strongest; RW-6 proof is deferred to DQ-4.
- **Incidents**: I1 makes G-1 a handoff blocker; I2 fixes exact head/demand ancestry; I3 closes every transition and override; I4 requires complete per-design plus union Serves; I5 fixes one join/resolution/admission corpus across desk, CI, dogfood, and cutover.
- **Verdict**: superseded as the recommendation by D-1 — retain its proof-carrying and append-before-launch properties, remove the zero-round skip, and resume comparison only after product policy defines the review floors.
- **From**: DQ-1 · panel synthesis

## Seam map

none

## Facts

- **F-1**: Amendment 001 is merged on `origin/main` at PR #737 / commit `20f2227dc5ae0a68fee14e955d4f8fe4462e15a7`; the effective projection is as-of 001 with fingerprint `sha256:1af9c46352bcddd356eede1c2daf6267d5e6ba31d34aa20997e4e1f2f6149708`. — source: docs/prds/planner-driven-model-routing/effective.md; git mainline
- **F-2**: The amended contract has no plan/v2 `no_inference` arm: FR-22/FR-23 require complete assessment and support evidence for every `/implement-now` task, NG-8 refuses deterministic executor recipes, and NR-8 separately preserves empty-frontier zero-model behavior. — source: prd:planner-driven-model-routing/FR-22, FR-23, NG-8, NR-8
- **F-3**: The frozen predecessor selected declarative compiler-derived demand (D-2/ALT-9), role-qualified native catalogs (D-3/ALT-11), and sealed tracker transport verified against the frozen plan (D-4/ALT-14); those decisions remain consistent after deleting only the unused deterministic arm. — source: design-brainstorm:planner-driven-model-routing-tech-design#D-2..D-4
- **F-4**: Review skip remains a valid review disposition only for the mechanically joined `ordinary × skip` cell; it is not a task execution mode. Every implementation task is model-backed, while a skipped proposal review performs no reviewer model round and still needs durable revision-bound evidence. — source: prd:planner-driven-model-routing/FR-13, FR-14, NR-8; design-brainstorm:planner-driven-model-routing-tech-design#F-38
- **F-5**: FR-24 now requires every attempt to link by `demand_id` to one plan/v2 freeze or typed plan/v1 translation and by `attempt_id` to one resolution terminal, including explicit-rung, absent-rung, retry, and review fixtures. — source: prd:planner-driven-model-routing/FR-24, EVT-1..EVT-3, EVT-8
- **F-6**: Current review orchestration inherits the resolved implementation harness unless an explicit review backend overrides it, but launches with no model/effort; model selection must occur before the fresh sibling starts and must not inherit implementation native capacity. — source: design-brainstorm:planner-driven-model-routing-tech-design#F-6, #F-12, #F-34, #F-35
- **F-7**: The `plan-new`, `implement-now`, and `impl-review` PRD ledgers contain no planner-driven-model-routing amendments even though the basis names them as required before Phase-A design handoff. — source: prd:planner-driven-model-routing §6.2; docs/prds/{plan-new,implement-now,impl-review}/
- **F-8**: The current `impl-review-report/v1` validator requires at least one round and the current `reviewinvoke` always launches a fresh reviewer, but FR-13 permits the joined `ordinary × skip` cell to complete with zero reviewer model rounds. This is a binding contradiction, not merely stale dependency currency: the review-integration design must supersede `impl-review-v1` while carrying its still-valid independence, exact-revision, and persistence guarantees forward. — source: internal/implreview/report/validate.go; internal/executor/reviewinvoke/reviewinvoke.go; prd:planner-driven-model-routing/FR-13; docs/tech-design/impl-review-v1.md
- **F-9**: D-1 establishes at least one model-backed review round at the lowest risk level and monotonically stronger review process as risk rises. This conflicts with the merged `ordinary -> skip` allowance; the exact higher-tier floors remain undecided. — source: D-1; prd:planner-driven-model-routing/FR-13, NR-2

## Fog

- **G-1** (blocking — blocks: header `Serves` and ready-for-design handoff): Required typed amendments to the `plan-new`, `implement-now`, and `impl-review` PRD ledgers have not landed; the remaining design questions can be explored against the complete parent contract, but the final design set cannot cite absent sibling requirements.

## Out of scope

none — predecessor boundaries remain orientation data until a live ruling confirms the current direction

## Candidate glossary terms

none — existing glossary vocabulary is sufficient

## Parked product gaps

### P-1 — Minimum review floor contradicts the merged routing contract

**Basis:** `prd:planner-driven-model-routing/FR-13`, `prd:planner-driven-model-routing/NR-2`

The merged contract permits `ordinary -> skip` and preserves `skip` as a merge-lane meaning. D-1 instead requires at least one review round for every implementation and monotonically stronger process/review floors as risk rises. The exact tier-to-process mapping is product semantics and cannot be invented in technical design.

**Disposition:** bounced — the PRD must be amended before architecture exploration resumes; see Gate.

## Gate

| Field | Value |
|---|---|
| **Verdict** | needs-more-product |
| **Date** | 2026-07-26 |
| **Authority** | @mdarabi |
| **Covering design** | — |
| **Coverage check** | — |
| **Amend route** | `/prd-amend planner-driven-model-routing` — resolve P-1 against `prd:planner-driven-model-routing/FR-13` and `prd:planner-driven-model-routing/NR-2`: remove zero-round review as a valid terminal outcome, require at least one review round at the lowest risk level, and define monotonically stronger review process/round floors for greater risk. |
