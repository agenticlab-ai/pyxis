# Review: read-to-rule 000-base — review report

| Field | Value |
|---|---|
| **In one line** | Approve-with-advisories after two rounds: round 1 bounced the base (10 blocking + 10 advisory across all four lenses — the reviewer-transition conflict with prd-review's merged contract, an unsatisfiable byte-zero docgate claim, an undefined checkpoint window, a tautological capture fit, four coherence chain-breaks); all twenty applied; round 2 confirmed closure, caught three regressions in changed spans plus one incomplete closure, all completed within the round. |
| **Relevant-when** | auditing or consuming the review of the read-to-rule base PRD; asking why it was bounced; tuning prd-review calibration against real findings |
| **Subject** | prd:read-to-rule/000 |
| **Moment** | new-base |
| **Reviewer** | prd-review · guide as merged 2026-07-10 · calibration prd-review-calibration/v1 |
| **Date(s)** | 2026-07-14 |
| **Verdict** | approve-with-advisories |
| **Tracking** | the subject's introducing PR (this PR) |
| **Template version** | review-report/v1 |

*Advisory period: this verdict blocks nothing — the reviewer is pre-adoption (prd:prd-review §6.5); findings and verdict accumulate as measurement, and the operator gate + merge remain the stage-2 gate.*

## Round 1

### FND-1 — G3's checkpoint moment is measured nowhere

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: "rubric at the prove and the checkpoint (DOD-4)" (§3.1 G3) — vs — DOD-4 being the single prove gate; §6.7 recording "M1, M2, and the guardrail data" with no rubric input.
- **Acceptance condition**: operationalize the checkpoint moment (rubric re-applied; §6.7 records the verdicts) or narrow G3's claim to the prove.
- **Disposition**: applied — G3 re-applies the DOD-4 rubric at each sampled gate (FR-8), verdicts feeding §6.7; round 2 found the §6.7 enumeration leg missing and it was completed within round 2.

### FND-2 — F2/F3 lack the non-regression leg §8 certifies

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: "F2→FR-6 / UB-2; F3→FR-8 / UB-4" (§8 flow row) — vs — the row's three-part criterion; F2's text leans on "the fold projects both", exactly NR-1's protected behavior, uncited.
- **Acceptance condition**: name F2's NR coverage (NR-1) and give F3 an NR citation or reasoned n/a; extend NR-1's Traces-to.
- **Disposition**: applied — F2→NR-1 with the fold rationale; F3 carries a reasoned n/a (the sampled read is the pre-§0 behavior itself, FR-9); NR-1 traces to F2.

### FND-3 — DOD-5 is sourced outside the document

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: DOD-5 Source "brainstorm:gate-attention#T-1" — vs — §5.8's aggregation universe (§5.1/5.2/5.9, §3.3, §6.4, §6.6) and §8's DoD-traceability row.
- **Acceptance condition**: re-source to an in-document unit, keeping the brainstorm anchor as provenance.
- **Disposition**: applied — Source = §6.6 Docs row (provenance: brainstorm:gate-attention#T-1).

### FND-4 — the header binds the launch gate to a PR that cannot host its evidence

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: "problem, solution, and launch gates via the introducing PR's operator review" (header Approver(s)) — vs — §6.4's criteria requiring Phase 1–3 evidence (landing diffs, corpus CI, the prove at a future base's gate); §6.6's own decision line drops the vehicle.
- **Acceptance condition**: scope the introducing-PR vehicle to problem + solution; state the launch gate's actual vehicle and timing.
- **Disposition**: applied — launch gate post-implementation, on §6.4 evidence at the prove, recorded in §6.6's table.

### FND-5 — thirteen Must rows with no within-class rank

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: every FR-1..9 and UB-1..4 Priority cell "Must" — vs — the template's rank-within-class rule ("unranked lists cut themselves").
- **Acceptance condition**: declare the rank or justify unrankability.
- **Disposition**: applied — within-Must rank = table order (§5.1 and §5.2), with the floor named honestly (FR-7..9 are the measurement's integrity; the cut line sits above them).

### FND-6 — FR-7's fit criterion cannot fail

- **Severity**: blocking (CAL-3 — the defect made a Must row unverifiable)
- **Lens**: contract-quality
- **Evidence**: "100% of checkpoint-counted gates carry the line (UB-4 voids lineless gates from the count)" (FR-7 fit) — vs — Appendix A's fit rule: the criterion quantifies over exactly the set from which every violation is excluded.
- **Acceptance condition**: quantify over all §0 gates signed in the window; a lineless gate is an observable, recorded miss.
- **Disposition**: applied — "every §0 gate in the window carries the line or stands as a recorded lineless miss — zero silent lineless gates; UB-4 voids lineless gates from metric arithmetic only."

### FND-7 — "the checkpoint window" is undefined while ten fit criteria quantify over it

- **Severity**: blocking (CAL-3 — multiple Must fits undecidable as written)
- **Lens**: contract-quality
- **Evidence**: "100% of §0 instances in the checkpoint window…" (FR-2 fit; same universe in FR-3..6, FR-8, UB-1, UB-3, NFR-1, NFR-3) — vs — no definition anywhere; whether lineless gates or pre-§0 documents are members was inference.
- **Acceptance condition**: define the window once — opening, closing, membership, including the lineless and pre-§0 cases.
- **Disposition**: applied — defined in the §5 subject-naming block (opens at the first §0 gate after Phase 2 exit; closes at the tenth capture-lined gate; lineless gates are members voided from metric arithmetic; pre-§0 gates outside per FR-9).

### FND-8 — day-one "machines ignore §0" conflicts with prd-review's merged contract

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: "untouched; they ignore §0 and review the whole document" (§5.5) + §3.3's sev-1 "any machine consuming §0" — vs — prd:prd-review FR-2/FR-3/§4.1 (its lenses cover the whole draft, excluding only the docgate floor; no §0-skip clause exists) while NG-5 forbids the repair surface and Q2 was untimed. This disputes no ruling — D-7 is the end-state; the defect was the unowned transition.
- **Acceptance condition**: scope §3.3's machine-consumption clause to consumers this feature binds until the reviewer ledgers adopt the skip; event-time Q2; recast the present-tense claims as the ruled end-state with lawful interim coverage.
- **Disposition**: applied — §3.3 scoped (reviewers join on Q2 adoption); §2.2/§5.5/§7.1 recast; Q2 event-timed (before the prove's synthesis, or immediately on the first §0-anchored reviewer finding).

### FND-9 — the byte-zero docgate claim is unsatisfiable under docgate's own template contract

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: "zero diff under the docgate code surfaces" (NR-1 fit) + "zero docgate or reviewer diffs" (DOD-6) — vs — prd:docgate §5.5 templates row ("template PRs ship updated goldens"), FR-8 (headers parse against declared template versions): the v1→v2 bump necessarily engages docgate-tree accompaniments. D-8 excluded *behavior* changes; the subject hardened that into byte-zero.
- **Acceptance condition**: narrow NR-1/DOD-6 to zero behavior/verb changes, explicitly permitting the golden/version-registration accompaniments a template PR owes docgate's contract.
- **Disposition**: applied — NR-1 fit, DOD-6, and NG-4 all narrowed and clarified.

### FND-10 — the glossary deliverable drops T-1's third avoid term

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: the working-tree glossary entry's avoid list ending at "*summary* (§1's job)" — vs — record T-1 ruling three terms: brief, summary, review report.
- **Acceptance condition**: carry "review report (the unmanned reviewer's artifact)" or record why dropped.
- **Disposition**: applied — the entry carries all three avoid terms.

### FND-11 — FR-5's fit never checks the skip map's completeness

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: "every named class carries its backstop" (FR-5 fit) — vs — the shall's core clause "each content class it omits"; a map silently dropping a class passed, and omission is ABUSE-1's vector.
- **Acceptance condition**: add a completeness check via the sampled full reads.
- **Disposition**: applied — "at each sampled full read (FR-8), zero content classes found covered by the body but neither routed in §0 nor named in the skip map."

### FND-12 — FR-8's sentence and its fit are different predicates

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: "approximately every fifth §0 gate" (FR-8) — vs — Appendix A's banned ambiguous adverbs; the fit's "≥ 1 per 5" floor left whole-window vs consecutive aggregation unpinned in FR-8 and §3.3.
- **Acceptance condition**: restate FR-8 in the criterion's own terms and pin the aggregation consistently.
- **Disposition**: applied — at-least-one-per-five-consecutive, pinned identically in FR-8's fit and §3.3.

### FND-13 — M1's "median" contradicts its own pooled signal — and the ruled basis

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: "≥ 80% median" (M1 target) — vs — the signal's pooled item-share and the record's D-5 (median attaches to gate time only); §3.3's ship rule and DOD-7 consume the number, and two honest computations could return opposite verdicts.
- **Acceptance condition**: pin M1's arithmetic once and use the same formula in §3.2, §3.3, and §1.
- **Disposition**: applied — pooled across all window feedback items, computed once; ship rule cites §3.2's formula.

### FND-14 — med risk tiers understate the template-wide surface

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: FR-1/FR-3/FR-4 Risk = med — vs — the template's tier definitions (high: broad blast radius; med requires existing-test coverage, absent here) and the subject's own ABUSE-1 rated high on the same surface; the capping rationale lived off-row.
- **Acceptance condition**: upgrade to high with the declared safety plan, or record the rationale on the rows.
- **Disposition**: applied — FR-1/FR-3/FR-4 high; §8's risk row names them with safety plan = the one-PR reversal (§6.5) + the normativity cap (NG-1).

### FND-15 — pronoun subjects in the EARS response slots

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: "…, it shall write §0…" (FR-3); "each content class it omits" (FR-5) — vs — Appendix A: the named system is always the subject; vague pronouns are struck on sight.
- **Acceptance condition**: name the subjects.
- **Disposition**: applied — "the synthesis flow shall write §0"; "each content class §0 omits".

### FND-16 — NFR-1 bundles two subjects behind a subjective predicate

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: "§0 shall fit one screen: the template shall state… and instances shall respect them" (NFR-1) — vs — Appendix A's singularity rule and subjective-language ban; the operative cap is Q1-deferred (a legal, owned TBD).
- **Acceptance condition**: split the obligations; demote "one screen" to rationale.
- **Disposition**: applied — NFR-1 is the template obligation; instance compliance delegated to UB-3 and the §3.3 bloat guardrail; "one screen" is rationale.

### FND-17 — the adoption path for existing ledgers has no mechanism, owner, or semantics

- **Severity**: advisory (CAL-4)
- **Lens**: completeness-critic
- **Evidence**: "existing ledgers adopt §0 at their next natural amendment" (NG-6) — vs — the amendment model's closed op vocabulary (no §0-prepend shape), the prd-amend step being maintenance-only (§4.2), no author for ranking over a historic ledger, and tension with FR-9's zero-new-obligations fit.
- **Acceptance condition**: Q1 gains the §0-introduction case; NG-6 states may-adopt, never an obligation.
- **Disposition**: applied — Q1 extended (op shape, authorship/ranking over historic ledgers); NG-6 restated with FR-9's guarantee.

### FND-18 — tier applicability of §0 is unstated

- **Severity**: advisory (CAL-4)
- **Lens**: completeness-critic
- **Evidence**: FR-1's four-block conformance with no tier rule — vs — the template's per-tier required-section lists; a mandatory four-block §0 on a T1 one-pager can cost more than the read it saves.
- **Acceptance condition**: the contract states the tier rule; Q1 holds the details.
- **Disposition**: applied — FR-1 (requirement + fit), §4.2 In, and DOD-1 carry the tier rule; Q1 the applicability details.

### FND-19 — ABUSE-1's mitigations fire on omission only, while its vector names downranking

- **Severity**: advisory (CAL-4)
- **Lens**: completeness-critic
- **Evidence**: ABUSE-1's Mitigated-by column (signature, sampling, recalibration, reviewer coverage — all omission-triggered) — vs — in-cap flooding/burial evading every listed control while the counters that touch it (FR-4's ordering rule, the DOD-4/G3 rubric, NFR-1's bound) sat unwired.
- **Acceptance condition**: wire the burial/flooding leg into the abuse row.
- **Disposition**: applied — ABUSE-1 covers burial and in-cap flooding, citing FR-4 conformance, NFR-1's bound, and the rubric at the prove and every sampled gate.

### FND-20 — the §0-gate counting unit is undefined; staged-gate reading unaddressed

- **Severity**: advisory (CAL-4)
- **Lens**: completeness-critic
- **Evidence**: FR-7's per-gate capture line and §6.7's "ten capture-lined §0 gates" — vs — template rule 15's collapsed single review signing three gate tables (one line or three — a 3x swing in the checkpoint trigger), and rule 2's read-nothing-below-§3 at a staged problem gate vs §0 ranking solution-space calls.
- **Acceptance condition**: define the counting unit; pin the per-event line shape and staged-gate reading in Q1.
- **Disposition**: applied — "a §0 gate" defined in the §5 subject-naming block (one review event, one line); Q1 covers the per-event format and staged-gate reading.

## Round 2

*Scope-locked to the twenty named defects plus regressions within changed spans. Nineteen closures confirmed against their acceptance conditions; FND-1's §6.7 leg was found missing (the G3 wiring was one-way) and completed within the round. Three regressions surfaced in the changed spans:*

### FND-21 — a residual present-tense reviewer claim contradicts the FND-8 recast

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: "docgate and the unmanned reviewer cover the full document regardless (they ignore §0 entirely)" (§7.1, customer FAQ) — vs — the recast interim state four lines below (reviewer coverage of §0 lawful until Q2 adoption): the same section asserted both states.
- **Acceptance condition**: qualify the customer answer to the end-state with the Q2 clause.
- **Disposition**: applied — "§0 earns no special weight from them, and once the reviewer ledgers adopt the skip (Q2) it sits outside their scope entirely."

### FND-22 — three old-cadence restatements survived the FND-12 rewrite

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: "approximately every fifth §0 gate" (§4.3 F3), "~every 5th gate sampled" (§5.5 contract cell), "Roughly every fifth gate" (§7.1) — vs — FR-8's pinned at-least-one-per-five-consecutive form.
- **Acceptance condition**: restate all three in FR-8's terms; the interface cell cites FR-8 rather than restating a looser rule.
- **Disposition**: applied — all three aligned; §5.5 cites FR-8.

### FND-23 — the changelog cites a review report absent from the tree

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: "FND-1..20 of docs/reviews/read-to-rule-000.md" (§7.4) — vs — docs/reviews/ holding no such file at review time.
- **Acceptance condition**: the report lands at the cited path on this introducing PR.
- **Disposition**: applied — this report.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-07-14 |
| 2 | approve-with-advisories | 2026-07-14 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 12 blocking · 11 advisory
- **Dispositions**: 23 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

none
