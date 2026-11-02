# Review: read-to-rule-tech-design-v1 — design-review report

| Field | Value |
|---|---|
| **In one line** | Approve — a faithful, well-anchored design extending §0 to the tech-design class; five findings (2 blocking · 3 advisory) all applied, the load-bearing one surfacing that the design realizes read-to-rule's deferred WR-1 and had to name it + justify the focused-design vehicle (operator to confirm at the gate, Q3). |
| **Relevant-when** | auditing or consuming the review of `read-to-rule-tech-design-v1`; asking why it was approved; tuning tech-design-review calibration |
| **Subject** | design:read-to-rule-tech-design-v1 · shape: none · supersedes: — |
| **Moment** | new-design |
| **Reviewer** | tech-design-review · guide 2026-07-17 (four lenses, fresh-context, over the stage context pack) |
| **Date(s)** | 2026-07-17 |
| **Verdict** | approve |
| **Tracking** | the introducing PR (branch `docs/read-to-rule-tech-design`), once filed |
| **Template version** | design-review-report/v1 |

## Round 1

### FND-1 — §2's no-contradiction proof cites "its DINVs" as a binding element `tech-design-conventions` does not possess

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: design:§2 "every binding element (A-1, A-7, A-8, its DINVs) stays simultaneously true" — vs — `tech-design-conventions` §3.3 prescribes `DINV-n` as a *per-document* construct for design instances and carries **none of its own** (its binding set is A-1..A-8 + Principles, "Deviation: none"). The enumeration names a class of binding element the cited doc lacks — a factual defect in the design's central Builds-on argument.
- **Acceptance condition**: drop "its DINVs" from the §2 enumeration (leaving A-1/A-7/A-8, which exist and stay true), or replace it with an element `tech-design-conventions` actually carries.
- **Disposition**: applied — "its DINVs" removed from §2. (Blast radius is limited — the self-refutation confirmed the Builds-on conclusion holds over the real A-rows and an empty DINV set is vacuously true — but the mis-citation sits in the load-bearing argument, so it is recorded at CAL-1's coherence default and fixed, not waved through.)

### FND-2 — A-3 verifies a mechanical self-conformance claim by "Inspection"

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: design:§6 A-3 `Verify: Inspection` / evidence "this document's §0" for a claim whose clauses are all deterministic counts (four blocks, ≤ 40 source lines, ≤ 7 ranked calls, every route resolving) — vs — the design's own R3 asserting the procedure is mechanical (the source checker re-parses §0 anchors), and the sibling `read-to-rule-v1` A-5 realizing the isomorphic self-conformance as `Verify: Test`. CAL-3→blocking does not fire (A-3 is testable, not untestable), so advisory.
- **Acceptance condition**: relabel A-3 to `Verify: Test` and name a runnable evidence artifact (the source-line / ranked-call count + the §0 anchor re-parse), matching the rigor the paired `read-to-rule-v1` A-5 applies.
- **Disposition**: applied — A-3 relabelled to `Test` with the mechanical count (19 lines / 5 calls) + anchor re-parse as evidence; the AP-10 un-gated caveat kept.

### FND-3 — §0 "required for every design" is silent on grandfathering the existing designs that can never gain one

- **Severity**: advisory (CAL-4)
- **Lens**: feasibility-completeness
- **Evidence**: design:§3.1 "§0 is required for **every** design … the escape is per-document honesty, not a tier" — vs — 45 of 46 designs under `docs/tech-design/` carry no §0 and are immutable (they can gain neither §0 nor a `Skipped:` line); the sibling `tech-design-conventions` §3.1 explicitly wrote this carve-out ("documents merged before this convention … cannot gain the new rows"). The unqualified "every" leaves the existing corpus ambiguously non-conformant and hands the future docgate-v2 §0-gate (Q1) no grandfathering handoff. Advisory: no A-row asserts the rule and nothing is gated today.
- **Acceptance condition**: add a grandfathering clause (designs merged before this template version are exempt; the v2 gate inherits the exemption via the declared `Template version`), mirroring `tech-design-conventions` §3.1.
- **Disposition**: applied — grandfathering clause added to §3.1 with the declared-version handoff to the Q1 v2 gate; §0 row 4 annotated "existing designs grandfathered".

### FND-4 — un-numbering `## 0. How to use` repoints a live cross-reference in `tech-design-conventions` A-7

- **Severity**: advisory (CAL-4)
- **Lens**: feasibility-completeness
- **Evidence**: design:§3.1 "the template's existing `## 0. How to use` … is un-numbered … freeing the §0 slot" and §3.2 "Must remain unchanged — the conventions §3.3 section enumeration (untouched)" — vs — `tech-design-conventions` A-7 cites `template §0` as evidence for "binding vs explanatory prose is distinguished," and that guidance now sits in the un-numbered How-to-use section while §0 = Read-to-rule. The completeness sweep's "does un-numbering break a cross-reference" probe: the design's unchanged-list missed this.
- **Acceptance condition**: note the anchor drift in §3.2, framed correctly against the read source's true state.
- **Disposition**: applied (reduced remedy) — a "benign consequence" note added to §3.2. **The lens's proposed remedy was corrected**: it read `tech-design-conventions`'s stale "Draft v0.1" Status line and proposed editing that doc as an unmerged draft, but the doc is **merged/frozen on origin/main** — so nothing is edited. The note records that A-7's binding claim stays true, and that a frozen acceptance row referencing merge-time template state under additive template evolution (sanctioned by conventions §3.1) is expected, not a defect.

### FND-5 — `Serves: —` is dishonest: the design realizes read-to-rule's deferred WR-1 without naming it

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: design:header `Serves: —` (claiming document-system infrastructure that "implements no product anchor") — vs — `prd:read-to-rule` carries committed WR-1 "the tech-design-class sibling — the same treatment for design documents" (effective.md:146), which this design *is*; `tech-design-conventions` §3.1 restricts `—` to work that "implements no product anchor"; incident I1 governs exactly this WR-anchor / under-anchoring case and is uncited. The design silently claims pure infrastructure while realizing a named PRD requirement.
- **Acceptance condition**: name WR-1 as the requirement-lineage, state why `Serves` is `—` rather than silently claiming infrastructure, and cite I1 in Principles — OR argue explicitly why WR-1 is not implemented here.
- **Disposition**: applied (corrected remedy). **The lens's literal fix — `Serves: prd:read-to-rule/WR-1` — is invalid**: WR-1 is a *waiting-room* anchor, which I1 and `lint-tech-designs` bar as a `Serves` target (it would red the gate). Applied the honest core instead: §1 now names WR-1 as intent-lineage, records read-to-rule's explicit "one class per feature / its own feature … after this feature's checkpoint" deferral, justifies the focused-design vehicle (operator pull-forward + the faithful low-novelty mirror), and states why `Serves: —` is the only gate-legal, honest classification for this document-system-convention work; Principles gains an I1 entry; A-7 and Q3 make the WR-1 lineage and the vehicle choice verifiable and operator-confirmable. The vehicle question is surfaced to the operator at the gate (§0 call 1 / Q3).

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | approve | 2026-07-17 |

## Escalation brief

none

## Measurements

- **Rounds**: 1
- **Findings**: 2 blocking · 3 advisory
- **Dispositions**: 5 applied · 0 declined · 0 withheld-as-violation
- **Notes**: two lens remedies were corrected before applying (FND-4 read a stale Status line and assumed an editable draft; FND-5's literal `Serves: WR-1` would fail the lint) — the honest core of each was applied. Advisory period: this verdict blocks nothing; findings accumulate toward the ten-review calibration checkpoint. All findings applied in-round → terminal approve, re-gate green.

## Overlay corrections

none
