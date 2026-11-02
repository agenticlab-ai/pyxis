# Review: prd:e2e-verification/002 — review report

| Field | Value |
|---|---|
| **In one line** | request-changes (round 1) on four blocking findings — an unresolvable Learned-from reference, §8 reading ✓ on cells the amendment falsified, a Must row verified against an uncommitted fixture, and a blocking DoD row resting on unowned judgment — all four applied, along with the abuse row the stand-in surface warranted and the EARS slip → approve-with-advisories (round 2): two residual advisories are declined with one shared structural reason, that §5.1 and §7.2 carry amendment-op-anchor rows a section supersede cannot safely replace. |
| **Relevant-when** | auditing or consuming the review of e2e-verification amendment 002; asking why the harness-design amendment was bounced; tuning prd-review calibration against real findings |
| **Subject** | prd:e2e-verification/002 |
| **Moment** | amendment |
| **Reviewer** | prd-review · 2026-08-06 |
| **Date(s)** | 2026-08-06 (rounds 1 and 2) |
| **Verdict** | approve-with-advisories *(round 2; history below is authoritative)* |
| **Tracking** | — *(the subject's introducing PR, once filed)* |
| **Template version** | review-report/v1 |

## Round 1

*Generated through four adversarially-screened lenses over the docgate context pack (vision · principles · incidents · calibration) and the amendment's folded effect (`docgate fold --dry-run`, as of 002). The gate floor and §0 are excluded — nothing docgate enforces is reported, and the amendment's own §0 replacement is neither read nor judged. Blast radius declared (contract-changing) matches computed (contract-changing); the folded projection's chain integrity is intact — every superseded row lands in Superseded & retired with a fresh non-reused ID.*

### FND-1 — the amendment's `Learned-from` cites a session log that records no decision

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: the envelope declares `Learned-from: … · operator-decision:logs/2026-08-06-1717-e2e-design-gaps.md`, but that file's entire body reads `Nothing escalated this session.` — it records no operator decision, so a reader resolving the reference finds nothing that supports the `operator-decision` class. — vs — `.pyxis/templates/prd-amendment-template.md` §0: "**Learned-from** is typed and mandatory … each with a structured reference. 'Learned from review' does not parse; evidence is a reference, not a vibe."
- **Acceptance condition**: point the `operator-decision` reference at an artifact that actually records the ruling — a session-log entry carrying the decision, the introducing PR, or an issue — or drop the class and let the two `build-result` references (which do resolve, and which are the genuine evidence: the harness sections of the two built e2e design docs) carry the envelope alone.
- **Disposition**: applied — the `operator-decision` reference is removed; the envelope now rests on the two `build-result` references, which resolve and are the genuine evidence (the harness sections of the two built e2e design docs). No substitute operator-decision artifact was invented: the session log records no escalation, and writing one to satisfy the class would be the fabrication the typed vocabulary exists to prevent.

### FND-2 — §8 still reads ✓ on the high-risk safety-plan row, but 002 adds five high-risk rows it never covers

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: §8's row "Every requirement row carries a risk tier; high/critical rows have a declared safety plan" reads `✓ | … high rows (FR-2/4/8/13, NR none, UB-3) mitigated by the review lenses + catch corpus + human acceptance`. After 002 the live high-risk set is FR-2, FR-4, FR-19, FR-22, FR-24, FR-26, UB-3, UB-8 — so the cell names two rows that no longer exist (FR-8, FR-13) and omits **five** it must now cover (FR-19, FR-22, FR-24, FR-26, UB-8). The same section's job- and flow-traceability rows (`J1→FR-1/2/3/4`, `J2→FR-7/8/9/10`, `J3→FR-12/13/14/15`, `F1→FR-1/…`, `F2→…FR-8…`) likewise index superseded IDs and reach none of the nine new rows, while every cell still reads ✓. — vs — the coherence lens's standing rule that §8 is *verified, not trusted*; and incident I4's lesson that a coverage omission left unclosed pre-merge costs a supersession to repair afterwards (`I4`, contract-gate/traceability).
- **Acceptance condition**: supersede §8 so its high-risk cell enumerates the live high-risk set with each row's declared safety plan, and its job/flow traceability cells resolve to live IDs including the nine rows 002 adds — or, where a cell can no longer honestly read ✓, mark it ☐ with the gap named.
- **Disposition**: applied — `op: supersede §8` added. The high-risk cell now enumerates the live set (FR-2/4, FR-19, FR-22, FR-24, FR-26, UB-3, UB-8) with each row's declared safety plan; the job, flow, metric and DoD cells resolve to live IDs (FR-18/24/25/26, UB-7/8/9, ABUSE-5, EVT-5/6, DOD-8/9); the problem-basis row keeps its ☐.

### FND-3 — FR-22 (Must, high) is verified against "the agentic fixture", which no scope or phasing row commits to building

- **Severity**: blocking (CAL-3 — becomes blocking when the defect makes a Must row unverifiable)
- **Lens**: contract-quality
- **Evidence**: FR-22's fit criterion opens `on the agentic fixture and on the dogfood if it is agentic, the harness section specifies all five properties`. The definite article names a specific asset, but 002's own §4.2 replacement enumerates the committed corpus as `a catch-test corpus (seeded coverage gaps, hollow scenarios, and harness holes)` with no agentic fixture, and §6.1 Phase A commits only `the catch corpus; multi-app-shape fixtures`. The second clause (`if it is agentic`) is conditional on a dogfood-feature choice not yet made, so the row has no guaranteed execution path — the amendment demonstrably maintained the corpus list for the new lens and omitted this one asset. — vs — Appendix A: "every requirement carries a measurement that decides pass/fail. If no fit criterion can be found, the requirement is ambiguous or not yet understood."
- **Acceptance condition**: name the agentic fixture in §4.2's in-scope list and §6.1's Phase A scope, or rewrite FR-22's fit criterion to rest on an asset the contract commits to.
- **Disposition**: applied — the agentic fixture is now committed in §4.2's in-scope list and in §6.1's Phase A scope, and FR-22's fit criterion reads `on the agentic fixture committed in §4.2`, dropping the `if it is agentic` conditional that depended on an unmade dogfood choice.

### FND-4 — DOD-8 is blocking but its only evidence is an unowned human-judgment check, falsifying §8's human-judgment row

- **Severity**: blocking (CAL-4 — becomes blocking when the gap is load-bearing for §8)
- **Lens**: completeness-critic
- **Evidence**: DOD-8 (`Blocking? yes`) requires evidence of `a fresh-context readability check confirming no mechanical element had to be invented`, and FR-19's fit criterion carries the same clause (`a fresh reader can state how a scenario is started, isolated, and asserted on without reading code`) under a `Inspection, Test` verify. Neither names a judge or a rubric. §8's row "Human-judgment criteria name an owner and a rubric (§5.8)" still reads `✓ | no human-judgment DoD rows` — which 002 makes false. — vs — Appendix A: "*Human judgment* is admissible only with a named owner and a rubric — unowned judgment is a TBD in costume."
- **Acceptance condition**: give the fresh-reader check a named owner and a stated rubric (and reclassify the affected `Verify` cells as Human judgment), or replace it with the mechanical half already present — every FR-19 element carries a stated answer, checked by the well-formedness checker — and update §8's cell accordingly.
- **Disposition**: applied — the unowned clause is gone from both rows. FR-19's fit criterion now ends `the buildability lens returns no finding against the section` (a lens with its own proving eval, EVAL-3); DOD-8's criterion and evidence are wholly mechanical — the checker's clean result over the harness section, the lens's finding-free verdict, and the plan's per-task owned-artifact list. §8's human-judgment cell is restated to record that no human-judgment DoD row exists.

### FND-5 — FR-22 introduces a new adversary (the decision-authority stand-in) with no ABUSE row

- **Severity**: advisory (CAL-4)
- **Lens**: completeness-critic
- **Evidence**: FR-22's own rationale states the attack — `a stand-in that only agrees, or that can see what the test asserts, manufactures a green rather than proving one` — but §5.3's abuse table is untouched by 002 and its stated adversary remains only `the e2e-design skill optimizing to look complete`; no row covers a specified stand-in that is cooperative-only, sighted, or silently improvising. §5.3's own preamble calls the abuse table "the heart of this section". — vs — AP-14's test-double corollary: "a test double must satisfy the **real dependency's contract**, not the code-under-test's assumption about it — a stub that mirrors your own wrong guess makes a never-working feature test green."
- **Acceptance condition**: add an ABUSE row for the stand-in surface (cooperative-only persona · assertion-sighted stand-in · unrecorded improvisation), naming FR-22 and the buildability lens as mitigations.
- **Disposition**: applied — `op: supersede §5.3` added, introducing ABUSE-5 (a stand-in specified cooperative-only, assertion-sighted, or silently improvising, so the agentic scenario reaches its end state whatever the feature did), mitigated by FR-22 and the buildability lens, verified by the harness-hole catch corpus at M4.

### FND-6 — cross-cutting statements outside §8 still index rows 002 superseded

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: §5.1's Must-rank line reads `FR-1/2/4/8/13 are the irreducible bet … FR-3/7/9/10/12/14/15 are the mechanics` — after 002, FR-1 and FR-8 are superseded (joining FR-12/FR-13 from 001), and none of the nine new Must rows appears in either tier. §6.1's Phase A exit criterion cites `the catch-corpus catch-rate (DOD-3) recorded`, and DOD-3 is superseded by DOD-9. §5.3's ABUSE-1/2/3/4 mitigations each cite `FR-8`. — vs — the coherence lens's "priorities ranked" bar and Appendix A's supersession mechanics ("a superseded ID never returns").
- **Acceptance condition**: restate the Must-rank line over live IDs with the new Must rows placed in a tier, and repoint §6.1's and §5.3's citations at their live successors (DOD-9, FR-24).
- **Disposition**: applied in part — §5.3's four mitigation cells and §6.1's exit criterion now cite FR-24 and DOD-9; verification surfaced two further stale references, both closed: §7.2 Q2 (which still described the lens set as two dimensions named in FR-8) is superseded by Q7, and §6.3 is superseded so R1 cites FR-24 and R2 cites EVAL-1/2/3, with R6 added for the shallow-but-answered harness section. **§5.1's Must-rank line is declined**: the only sanctioned mechanism is a §5.1 section supersede, and §5.1 carries the FR and NR tables, whose prefixes are amendment-op anchors — replacing that section wholesale would freeze the fold's generated provenance markers as literal text and leave the unit map pointing into hand-authored content, breaking future row-level ops against it. The template's own rule that G/J/F/M/ABUSE/R rows (never FR/NR/UB/NFR/DOD/EVAL/EVT/Q) evolve by section supersede is exactly this boundary. The remedy costs more than the defect; the rank line is a narrative gloss and every row's own Priority cell remains correct.

### FND-7 — FR-21 opens with "Once", which is not an EARS keyword

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: FR-21 reads `Once the harness section is settled, each scenario shall be restated in that harness's own terms …`. `Once` is outside the six admitted patterns; the intended sense is event-driven (`When`) or state-driven (`While`). — vs — Appendix A's EARS table: "constrain every requirement sentence to one of these" (Ubiquitous · Event-driven *When* · State-driven *While* · Optional feature *Where* · Unwanted behavior *If* · Complex).
- **Acceptance condition**: reword FR-21's trigger to `When the harness section is settled, …` (or `While the harness section is settled, …`), leaving the response clause unchanged.
- **Disposition**: applied — FR-21 now opens `When the harness section is settled, …`, the event-driven EARS pattern; the response clause is unchanged.

## Round 2

*Scope-locked to the seven named defects plus regressions within the spans round 1's dispositions changed (the envelope, the FR-19/21/22 and DOD-8 payloads, and the §4.2 · §5.3 · §6.1 · §6.3 · §8 · Q2 ops). Unchanged content is not re-examined. All four blocking findings verified closed against their acceptance conditions; the folded projection recomputes clean (44 headings, section set identical to round 1; every superseded row in Superseded & retired with a fresh ID; declared blast radius still matches computed). Two regressions surfaced — both introduced by round 1's own fixes, both inside changed spans.*

### FND-8 — §8's readiness cells went stale against the rows round 1's fixes added

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: the §8 replacement written for FND-2 carried forward two cells unchanged — `Risks and assumptions have owners (§6.3) | ✓ | R1–R5, A1 owned` and `Zero solution-blocking TBDs or open questions (§7.2) | ✓ | Q1–Q3 and Q6 design-owned …` — while FND-6's fixes added R6 to §6.3 and superseded Q2 with Q7. Both cells still read ✓ over an enumeration that no longer matches the section they certify. — vs — the coherence lens's standing rule that §8 is *verified, not trusted* (the same rule FND-2 closed).
- **Acceptance condition**: restate both cells over the live sets — `R1–R6, A1` and the open-question set with Q7 in place of Q2.
- **Disposition**: applied — both §8 cells restated in the same op; the risk-owner cell reads `R1–R6, A1 owned` and the open-question cell names `Q1, Q3, Q6 and Q7 (which supersedes Q2)`.

### FND-9 — §7.2's closing note still enumerates Q2

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: §7.2 closes with `*None blocks the solution gate: Q1–Q3 and Q6 are design-owned with their product bars fixed in §5; Q4/Q5 are deferred by ruling to later phases.*` — naming Q2, which this amendment supersedes with Q7. — vs — Appendix A's supersession mechanics: "a superseded ID never returns."
- **Acceptance condition**: restate the note over the live question set, or record why the ledger's mechanism cannot reach it.
- **Disposition**: declined — the note is unanchored prose, so the only sanctioned mechanism is `supersede §7.2`; §7.2 carries the Q table, and `Q` is one of the amendment-op anchors (`FR NR UB NFR DOD EVAL EVT Q`), so replacing that section wholesale would freeze generated provenance markers as literal text and leave the unit map pointing into hand-authored content — breaking a future `answer Q1` or `supersede Q3`. This is the same structural boundary that declined FND-6's §5.1 half, and the template draws it explicitly: G/J/F/M/ABUSE/R rows evolve by section supersede, op-anchor rows never do. The substantive half of the defect is closed — Q2's own text (which described the lens set as two dimensions named in FR-8) is superseded by Q7 — leaving only a stale enumeration in a gloss. Both residuals are named here so a future amendment that reaches §5.1 or §7.2 for its own reasons can sweep them.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-08-06 |
| 2 | approve-with-advisories | 2026-08-06 |

## Escalation brief

none

## Measurements

- **Rounds**: 2 *(the cap)*
- **Findings**: 4 blocking · 5 advisory *(round 1: 4 blocking, 3 advisory; round 2: 2 advisory regressions in changed spans)*
- **Dispositions**: 8 applied (FND-6 in part) · 1 declined · 0 withheld-as-violation *(every blocking finding applied — nothing auto-escalates. The two declines are advisory and share one structural reason: FND-6's §5.1 Must-rank-line half and FND-9's §7.2 closing note both sit in sections carrying amendment-op-anchor rows, which a section supersede cannot safely replace.)*

## Overlay corrections

*Post-approve entries only — the M2 residual data; frozen rounds and
history are never edited.*

none
