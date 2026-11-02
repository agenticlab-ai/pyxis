# Review: e2e-verification-artifacts-v2 — design-review report

| Field | Value |
|---|---|
| **In one line** | approve-with-advisories at round 2, after request-changes at round 1 on three blocking findings — the checker was told both to reject a library-name-only slot and never to read a slot for meaning (an implementer could not build both), DINV-4's approval hold was asserted with no decidable rule behind it, and the supersession stranded a live merged dependent whose stale lineage edge the design never dispositioned — all seven findings applied, the checker/lens boundary now drawn at a structural shape floor whose threshold rides §7 Q4. |
| **Relevant-when** | auditing or consuming the review of the `e2e-verification-artifacts-v2` design; asking why it bounced; tuning tech-design-review calibration |
| **Subject** | design:e2e-verification-artifacts-v2 · shape: none · supersedes: e2e-verification-artifacts |
| **Moment** | supersession |
| **Reviewer** | tech-design-review · 2026-08-06 |
| **Date(s)** | 2026-08-06 |
| **Verdict** | approve-with-advisories *(round 2; the history below is authoritative)* |
| **Tracking** | — *(the subject's introducing PR)* |
| **Template version** | design-review-report/v1 |

## Round 1

*Generated through four blind fresh-context lenses over the subject and its declared supersession target, with the stage context pack as governing corpus. The `lint-tech-designs` floor is excluded — the header schema, `Serves` resolution and liveness, and the mandatory-by-name subsections all pass and are never findings here.*

### FND-1 — the checker is required to reject a library-name-only slot *and* never to read a slot for meaning; both cannot be built

- **Severity**: blocking (CAL-1 — a direct internal contradiction between two binding elements)
- **Lens**: coherence
- **Evidence**: `design:` §3.5's rule table — "a slot answered empty, by a placeholder, or **by a bare library or tool name with no mechanics** | malformed" — and `#DINV-2` repeating it, and A-2 seeding "a slot answered only by a library name" as a defect that must "fail at exactly its rule" — vs — `design:` `#DINV-8` "the checker of §3.5 decides presence and never sufficiency; **no rule it enforces requires reading a slot's answer for meaning**" and A-9 "no checker rule requires reading a slot answer for meaning — inspection of the rule set confirms every rule is decidable structurally". Deciding that a slot's text is "a bare library or tool name with no mechanics" is precisely a reading for meaning. An implementer satisfying A-2 fails A-9, and vice versa.
- **Acceptance condition**: pick one side and make every binding element agree. Either (a) narrow the checker rule to a structurally decidable proxy — e.g. a slot answer below a stated length, or matching a bare-token shape with no sentence structure — and hand "answers only by naming a library" to the buildability lens, adjusting `#DINV-2`, §3.5, A-2 and the §0 call; or (b) drop `#DINV-8`/A-9's absolute and state honestly that one checker rule applies a named heuristic. Whichever is chosen, §3.5's rule table, `#DINV-2`, `#DINV-8`, A-2 and A-9 must state the same boundary.
- **Disposition**: applied — option (a), with `#DINV-8` widened to "presence **and shape**". The record's `#D-3` gains a review-anchored refinement (`design-brainstorm:e2e-verification-tech-design-2#D-9`) narrowing its mechanical half to the shape floor; the authority's split is preserved, only its boundary corrected.

### FND-2 — DINV-4's approval hold is asserted with no rule that can test it

- **Severity**: blocking (CAL-3 escalated — a binding invariant goes untestable as stated)
- **Lens**: contract-quality
- **Evidence**: `design:` `#DINV-4` "a slot carrying a blocking open item leaves the document well-formed but **unapprovable**; the document cannot reach `approved` while one stands" and A-4 "a doc carrying a blocking open item in a slot is well-formed yet cannot reach `approved` — Verify: **Test**" — vs — §3.5's rule table records only the verdict "well-formed; blocks `approved`" and lists no rule pairing the header's `Status` value with a present blocking open item. The class is ungoverned by design (§3.7), so nothing outside the checker enforces the hold either. As written there is no artifact a test can assert against: the checker reports a state, and a document whose `Status` reads `approved` while a blocking open item stands violates DINV-4 with nothing detecting it.
- **Acceptance condition**: add the decidable rule to §3.5 — `Status: approved` together with a present blocking open item is **malformed** — and re-point A-4's Test at that rule, so the invariant has an enforcing surface rather than an assertion.
- **Disposition**: applied — §3.5 gains the `Status: approved` + blocking-open-item rule; `#DINV-4` and A-4 re-point at it.

### FND-3 — the supersession strands a live merged dependent and dispositions nothing

- **Severity**: blocking (CAL-2 — supersession integrity: a live dependent left stranded)
- **Lens**: upstream-alignment
- **Evidence**: `design:` `Supersedes: e2e-verification-artifacts`, and §3.8's boundaries name no downstream lineage consequence — vs — the regenerated index shows `implement-now-e2e-tasks` **live** with `builds_on` including `e2e-verification-artifacts` and now carrying `stale-builds-on`; `design:implement-now-e2e-tasks` is merged, so incident **I6**'s lesson applies directly — "a stale-builds-on index flag is a planning blocker in waiting, never decoration", and "the only legal lineage repair is whole-document supersession". The sibling `design:e2e-verification-skills-v2` §3.5 names the executor's *briefing-code* impact but neither design names the *lineage* stranding or says what becomes of the flag.
- **Acceptance condition**: name the stranding and its disposition in §3.8 — that superseding this design's target flags `implement-now-e2e-tasks` `stale-builds-on`, that re-pointing a merged design's `Builds-on` is illegal (I6), and that the flag therefore stands until that design is itself superseded under `prd:implement-now`. A named, owned consequence closes this; a silent one does not.
- **Disposition**: applied — §3.8 names the stranding, cites I6 and `check-immutable`, and assigns the disposition to `prd:implement-now`.

### FND-4 — the class defines seven sections against a requirement naming six contents, with no stated mapping

- **Severity**: advisory (CAL-2, downgraded — the substance satisfies FR-18; only the mapping is unstated, so this is a legibility gap rather than a contract deviation)
- **Lens**: upstream-alignment
- **Evidence**: `design:` §3.1 enumerates seven sections, splitting the per-scenario facts into `Scenario routes` and `Scenario build facts` — vs — `prd:e2e-verification/FR-18` requires a doc "containing scenarios, setup and teardown, a mock strategy, a harness design, a coverage map, and **a per-scenario route**" (six named contents), whose fit criterion is "the produced doc contains all six parts". All six are present, so the contract holds — but the design never says which section is FR-18's sixth part nor that the seventh is additive from `FR-21`, leaving a reader (and the checker's author) unable to confirm the six-part bar from this document.
- **Acceptance condition**: state the mapping in §3.1 — `Scenario routes` is FR-18's per-scenario route part; `Scenario build facts` is additive, carrying `FR-21`'s normalization — so the six-part requirement is visibly satisfied rather than inferred.
- **Disposition**: applied — §3.1 states the mapping onto `FR-18`'s six contents, with section 7 marked additive from `FR-21`.

### FND-5 — DINV-5's second half is vacuous under the new two-table structure

- **Severity**: advisory (CAL-1, downgraded — the invariant's live half still binds; the dead half misleads rather than breaks)
- **Lens**: coherence
- **Evidence**: `design:` `#DINV-5` "no required behavior the doc enumerates is absent from **both** the coverage map and the routes table" — vs — §3.3 keys the routes table by **scenario**, not by required behavior, so a required behavior can never appear in it. The clause is inherited verbatim from the predecessor's `#DINV-2`, where "the routes" was looser; under this design's named, scenario-keyed table the "both" test reduces to the coverage map alone.
- **Acceptance condition**: reword `#DINV-5` to the property that actually holds — every required behavior the doc enumerates appears in the coverage map carrying ≥ 1 scenario or an explicit routed disposition — dropping the routes-table half rather than carrying a test nothing can fail.
- **Disposition**: applied — `#DINV-5` reworded to the coverage-map property; the vacuous routes-table half dropped with a note.

### FND-6 — the class rests its integrity on a checker nothing is said to invoke

- **Severity**: advisory (CAL-4 — a completeness gap; not load-bearing for A-2, which tests the rules rather than their invocation)
- **Lens**: feasibility-completeness
- **Evidence**: `design:` §3.5 "the class ships a **checker** enforcing exactly the mechanical rules below… not a docgate verb and wired to no gate" and §3.10 "the class's authoring-time integrity surface is the checker of §3.5 plus review-time inspection" — vs — §3.8 routes behavior to `design:e2e-verification-skills-v2`, and that design's Scope explicitly excludes "the artifact schemas, slot vocabulary, **checker rules**, home or lifecycle", its §0 skip map sending structure "→ that class's checker". Neither document names what runs the checker, or when. A checker wired to no gate and invoked by nothing is decorative, and the integrity claim in §3.10 rests on it.
- **Acceptance condition**: name the invocation owner — either state in §3.5 that e2e-design runs the checker before emitting and e2e-review runs it over the subject (making it behavior the sibling design must carry, and saying so), or record it as an explicit §7 open question with an owner.
- **Disposition**: applied — §3.5 names e2e-design and e2e-review as callers; the sibling design carries the invocation (§3.4, `#DINV-12`, A-16).

## Round 2

*Scope-locked to the spans round 1's findings changed — §0 row 1 and the open-questions line, §3.1's FR-18 mapping, §3.5's rule table and its two new paragraphs, §3.8's stranding bullet, `#DINV-2`/`#DINV-4`/`#DINV-5`/`#DINV-8`, A-2/A-4/A-9, and the sibling design's §3.4/`#DINV-12`/A-16. The unchanged remainder is not re-reviewed.*

**FND-1 — closed.** `#DINV-8` now reads "presence **and shape**, never sufficiency", and §3.5's library-name rule is restated as an explicitly structural proxy that "decides the *shape* of an answer… and never whether the prose is right". A-2's fixture is re-described as a bare token run caught by the shape floor; A-9 now asserts structural decidability and adds a paired fixture (bare token run → malformed; short-but-structured → well-formed, left to the lens). An implementer can build both rows.

**FND-2 — closed.** §3.5 gains the decidable rule — `Status: approved` with any blocking open item present is **malformed** — and `#DINV-4` and A-4 both re-point at it, A-4 now exercising the fixture in both `Status` states. The invariant has an enforcing surface.

**FND-3 — closed.** §3.8 names the stranding, its illegality-of-repair (I6 · `check-immutable`), its owner (`prd:implement-now`), and its disposition (the flag stands until that design is itself superseded). The consequence is owned rather than silent.

**FND-4 — closed.** §3.1 states the one-to-one-plus-one mapping onto `FR-18`'s six contents explicitly.

**FND-5 — closed.** `#DINV-5` is reworded to the coverage-map property that can actually fail, with a parenthetical recording why the predecessor's second half was dropped.

**FND-6 — closed.** §3.5 names e2e-design and e2e-review as the checker's callers and routes the invocation to the sibling design, which now carries it as a `**Decision:**` in §3.4, `#DINV-12`, and A-16, with its Scope amended to admit the invocation while still excluding the rules.

### FND-7 — the new shape-floor rule referenced a floor the document never stated

- **Severity**: advisory (CAL-3 — a binding rule with a dangling reference; introduced by round 1's fix, so in scope)
- **Lens**: contract-quality
- **Evidence**: `design:` §3.5 (as revised) "a slot answered by a **bare token run** — no sentence structure, below the stated floor" and `#DINV-2` repeating "below the stated floor" — vs — no section of the document states that floor, so the rule's threshold resolved to nothing and A-9's fixtures had no bar to sit either side of.
- **Acceptance condition**: either state the floor, or bind the rule while routing its threshold to a named open question with an owner.
- **Disposition**: applied — both references now read "below the shape floor (§7 Q4)", and Q4 is added: the rule **binds**, its threshold is tuned at implementation against the A-2 and A-9 fixtures, deliberately not frozen as a number in an immutable document (the template's no-volatile-facts rule).

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-08-06 |
| 2 | approve-with-advisories | 2026-08-06 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 3 blocking · 4 advisory
- **Dispositions**: 7 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

none
