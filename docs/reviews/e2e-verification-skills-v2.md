# Review: e2e-verification-skills-v2 — design-review report

| Field | Value |
|---|---|
| **In one line** | approve-with-advisories at round 2, after request-changes at round 1 on two blocking findings — the supersession strands two live merged dependents with no lineage disposition named, and AP-23 (the parallel-authors principle whose text §3.3's whole decision implements) is silently omitted from Principles — plus four advisories (a half-specified checker rule, a stale §3.4 heading, an unnamed oracle for the never-a-library-name bar, and a duplicate filter whose stated matching may miss its own motivating case) — all six applied. |
| **Relevant-when** | auditing or consuming the review of the `e2e-verification-skills-v2` design; asking why it bounced; tuning tech-design-review calibration |
| **Subject** | design:e2e-verification-skills-v2 · shape: none · supersedes: e2e-verification-skills |
| **Moment** | supersession |
| **Reviewer** | tech-design-review · 2026-08-07 |
| **Date(s)** | 2026-08-07 |
| **Verdict** | approve-with-advisories *(round 2; the history below is authoritative)* |
| **Tracking** | — *(the subject's introducing PR)* |
| **Template version** | design-review-report/v1 |

## Round 1

*Generated through four blind fresh-context lenses over the subject and its declared supersession target, with the stage context pack as governing corpus. The `lint-tech-designs` floor is excluded — the header schema, `Serves` resolution and liveness, and the mandatory-by-name subsections all pass and are never findings here.*

### FND-1 — the supersession strands two live merged dependents and dispositions neither

- **Severity**: blocking (CAL-2 — supersession integrity: live dependents left stranded)
- **Lens**: upstream-alignment
- **Evidence**: `design:` `Supersedes: e2e-verification-skills`, and §3.5's boundaries name only the executor's *briefing-code* impact ("adapting the executor's e2e briefing to the new class… owned by `prd:implement-now`") — vs — the regenerated index shows **two** live designs whose `Builds-on` names this document's target: `impl-review-routed-assurance` (`builds_on: … e2e-verification-skills`), which carried **no flag before this supersession and now carries `stale-builds-on``, and `implement-now-e2e-tasks`, which gains a second stale edge. Both are merged, so incident **I6** applies — "a stale-builds-on index flag is a planning blocker in waiting, never decoration"; "the only legal lineage repair is whole-document supersession". The sibling `design:e2e-verification-artifacts-v2` §3.8 dispositions its own stranding; this design dispositions neither of its two, and the newly-stranded `impl-review-routed-assurance` is named nowhere in the change.
- **Acceptance condition**: name both stranded dependents in §3.5 with their disposition — that superseding this target flags them `stale-builds-on`, that re-pointing a merged design's lineage row is illegal (I6 · `check-immutable`), and that each flag stands until its own design is superseded under its owning feature (`prd:impl-review`, `prd:implement-now`), neither being this feature's to edit (`prd:e2e-verification/NR-1`, `NR-2`).
- **Disposition**: applied — §3.5 names both stranded dependents, cites I6 and `check-immutable`, and assigns each flag to its owning feature (`prd:impl-review`, `prd:implement-now`).

### FND-2 — AP-23 is silently omitted, though §3.3's decision implements its text almost clause for clause

- **Severity**: blocking (CAL-2 — principles honesty: a relevant principle silently omitted, not deviated-with-reason)
- **Lens**: upstream-alignment
- **Evidence**: `design:` §3.3 decides that every scenario task "names the one artifact it exclusively owns", that no two name the same one, that the conventions and harness tasks carry `blocked-by` edges ahead of the fan-out, and that the conventions task exists because otherwise "parallel workers each invent scaffolding nobody reuses and collide on shared files — structurally, not by bad luck" — vs — **AP-23**: "work decomposition **declares the files each task touches and sequences the overlaps**. Remove conflict magnets **structurally** — a recovery loop that repeatedly re-merges the same hot line is treating the symptom." The Principles section cites AP-10/21, AP-11, AP-13/14, AP-19, AP-5, AP-25, AP-27, AP-28, NR-1/NR-2, RW-5, I5 and I6 — and not AP-23, the one principle this design's most load-bearing new decision is a direct instance of. The template's rule is explicit: deviation-with-reason is legitimate, silent omission is not.
- **Acceptance condition**: cite AP-23 as honored in Principles, naming the three clauses §3.3 implements — declared per-task file ownership (`FR-26`), sequenced overlaps (the `blocked-by` edges), and the structural conflict-magnet removal (the conventions task landing a shape before fan-out).
- **Disposition**: applied — Principles gains an AP-23 bullet mapping its three clauses onto §3.3.

### FND-3 — DINV-12 mandates running the checker at round start but defines no consequence

- **Severity**: advisory (CAL-3 — a binding element half-specified; its run-it clause is testable, its outcome clause absent)
- **Lens**: contract-quality
- **Evidence**: `design:` `#DINV-12` "e2e-review runs it over the subject at the start of a round" and §3.4's Decision "so the operator never spends a round on a structural defect", with A-16 testing only that it runs — vs — nothing in the design says what e2e-review *does* when the checker returns malformed: whether it refuses the round, emits the defects as findings, or proceeds anyway. An implementer must guess, and the stated purpose (not spending a round on a structural defect) is unreachable without the branch. This reproduces one level up the "a checker nobody acts on is decorative" gap that the invocation clause was added to close.
- **Acceptance condition**: state the branch in §3.4 or §3.2 — e2e-review refuses the round on a malformed subject, returning the checker's defects rather than lens findings — and extend `#DINV-12` and A-16 to cover it.
- **Disposition**: applied — §3.4, `#DINV-12` and A-16 now carry the refuse-the-round branch on a malformed subject.

### FND-4 — §3.4's heading no longer describes half its content

- **Severity**: advisory (CAL-3 — legibility of a section carrying two `**Decision:**` callouts)
- **Lens**: contract-quality
- **Evidence**: `design:` §3.4 is titled "Skill separation and single invocation" and now carries two Decisions — the three-commands-one-invocation ruling, and the checker-invocation ruling ("the artifact class's well-formedness **checker is invoked by these skills**") — vs — the second has nothing to do with skill separation or invocation *count*, and the Scope's pointer sends a reader to "§3.4" for the checker invocation, where the heading suggests they are in the wrong place. Sections are append-only after merge, so a heading that mis-describes its content is permanent.
- **Acceptance condition**: retitle §3.4 to cover both rulings (e.g. "Skill separation, single invocation, and who runs the checker"), or move the checker ruling into its own appended `3.x` subsection with the Scope pointer updated. Either is legal pre-merge; neither is after.
- **Disposition**: applied — §3.4 retitled to cover both rulings.

### FND-5 — the never-a-library-name bar has no oracle named in this design

- **Severity**: advisory (CAL-1, downgraded — the property holds; only its test bar is unstated here)
- **Lens**: coherence
- **Evidence**: `design:` `#DINV-8` "never lets a library or tool name stand in for missing mechanics" and A-10 "**zero** harness sections are produced with a slot answered only by a library or tool name — Verify: **Test**" — vs — after the sibling review, the mechanical bar for that property is the artifacts design's **shape floor** (`design:e2e-verification-artifacts-v2` §3.5, threshold at its §7 Q4), with the richer judgment belonging to the buildability lens. This design names neither, so A-10's test has no stated oracle and reads as though the skill itself decides what counts as a library name.
- **Acceptance condition**: point `#DINV-8` and A-10 at the sibling's shape floor as the mechanical bar and at the buildability lens for the residue, so the row's Test has something to assert against.
- **Disposition**: applied — `#DINV-8` and A-10 point at the sibling shape floor for the mechanical bar and the buildability lens for the residue.

### FND-6 — the duplicate filter's stated matching may miss the very duplicate it exists to remove

- **Severity**: advisory (CAL-4 — a feasibility gap in a named mechanism; not load-bearing for A-5, which tests the filter's existence)
- **Lens**: feasibility-completeness
- **Evidence**: `design:` §7 Q4 "starts at exact defect identity (the conservative end, R3)" and R3's mitigation "the filter matches on the checker's own raised-defect identity, not on topic similarity" — vs — the motivating duplicate after the sibling review is a **rephrasing**, not a repeat: the checker raises `bare token run` (a shape verdict) while the buildability lens says "this slot names a library and specifies no machine". Those are the same defect under two vocabularies and would not match on exact defect identity, so the filter as specified lets through precisely the case §3.2 introduces it for. R3 anticipates over-reach; under-reach is unexamined.
- **Acceptance condition**: acknowledge under-reach alongside over-reach — either widen Q4's starting point to match a checker defect against the *slot* it was raised on (identity plus location, still not topic similarity), or record in R3 that duplicates surviving the filter are the accepted round-1 cost with the log as the evidence for widening.
- **Disposition**: applied — the match key widens to defect identity plus slot; under-reach named as R8; A-5 exercises both directions.

## Round 2

*Scope-locked to the spans round 1's findings changed — §0's open-questions line, §3.4's heading and its checker Decision, §3.5's stranding bullet, `#DINV-8` and `#DINV-12`, R3 and the new R8, A-5, A-10 and A-16, §7 Q4, and the Principles AP-23 bullet. The unchanged remainder is not re-reviewed.*

**FND-1 — closed.** §3.5 gains a stranding bullet naming **both** dependents — `impl-review-routed-assurance` (explicitly marked as carrying no flag before this change) and `implement-now-e2e-tasks` — with the illegality of re-pointing a merged lineage row (I6 · `check-immutable`), each flag's owning feature, and the statement that each stands until its own design is superseded.

**FND-2 — closed.** Principles gains an AP-23 bullet mapping the principle's three clauses onto §3.3: declared per-task file ownership, sequenced overlaps, and structural removal of the conflict magnet.

**FND-3 — closed.** §3.4's Decision now states the branch — e2e-review **refuses the round** on a malformed subject, returning the checker's defects and running no lens — and `#DINV-12` and A-16 both carry it, A-16 gaining a malformed-subject review fixture.

**FND-4 — closed.** §3.4 is retitled "Skill separation, single invocation, and who runs the checker"; the Scope's pointer resolves to a heading that describes the content.

**FND-5 — closed.** `#DINV-8` and A-10 now name the sibling's shape floor as the mechanical bar and the buildability lens as owner of the residue, so A-10's Test has a stated oracle.

**FND-6 — closed.** The match key widens to **checker-defect identity plus the slot it was raised on** — enough to collapse the predictable rephrasing, still an identity match rather than topic similarity. Under-reach is now a named risk (R8, deliberately placed beside its over-reach counterpart R3 rather than appended, so the pair reads together), and A-5's fixture exercises both a same-slot rephrasing and a different-slot near-miss.

*No new finding arose on the changed spans.*

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-08-07 |
| 2 | approve-with-advisories | 2026-08-07 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 2 blocking · 4 advisory
- **Dispositions**: 6 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

none
