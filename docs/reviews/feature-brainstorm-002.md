# Review: prd:feature-brainstorm/002 (argus-principal-seam) — review report

| Field | Value |
|---|---|
| **In one line** | The stage-1 principal seam folds cleanly and faithful to D-5 (ungranted-unmanned stays sev-1; the injection / no-corpus guarantee preserved verbatim; principal attribution added) — approved with two advisories about live-body pointers the fold left behind (superseded FR-6 read as live in the ranking line and §8; EVAL-2's re-anchor left implicit). |
| **Relevant-when** | auditing or consuming the review of feature-brainstorm amendment 002; asking why the argus principal seam was approved-with-advisories; tuning prd-review calibration against real amendment findings (stale-pointer vs. downstream-stale boundary) |
| **Subject** | prd:feature-brainstorm/002 |
| **Moment** | amendment |
| **Reviewer** | prd-review · guide 2026-07 (.pyxis/authoring/prd-review.md) |
| **Date(s)** | 2026-07-11 |
| **Verdict** | approve-with-advisories |
| **Tracking** | the amendment's introducing PR (filed on approval) |
| **Template version** | review-report/v1 |

## Round 1

Subject verified: a validated amendment (002) reviewed as its folded effect
(`effective.md` as of 002). Floor excluded: `docgate gate` runs clean and
`validate-amendments` / the fold / `diff` are green — the computed blast
radius is **contract-changing** (affected: FR-6 · FR-16 · NG-4), matching the
declared radius; nothing the gate enforces is raised below. Basis fidelity
holds against the governing ruling `design-brainstorm:argus-tech-design#D-5`
(live, gated ready-for-design on main) and the argus PRD §4.1/§4.2 + FR-12:
NG-4 → NG-8 keeps a *principal-less* session as the sev-1 failure mode;
FR-6 → FR-17 preserves the injection / no-corpus-decision guarantee verbatim
and adds ruling-principal attribution; FR-16 defines "decision principal"
grounded in the glossary (line 62) and argus FR-12; the Learned-from resolves.
Both survivors are advisory; both are close to — but, on the reference-vs-word
distinction, outside — the operator's declared blanket-re-anchor scope.

### FND-1 — the fold leaves superseded FR-6 (and rescoped NG-4) reading as live in the projection's own body

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: `effective.md` §5.1 priority prose — "Ranking within Must: **FR-6**/12/13 (decision integrity, the gate, the handoff)" (line 236) — and §8 contract-readiness — "high rows **FR-6**/10/11/12/13" (line 445), "J4→**FR-6**/7/8" (line 447) — plus §2.2 "see **NG-4**" (line 51); every one names an ID this amendment retired, while its live successor is `FR-17 *(002 ← FR-6)*` / `NG-8`. — vs — the operator's declared blanket-re-anchor scope (established context) covers the recurring *word* "operator" (FR-16 redefines it contract-wide), but it does **not** redefine an *ID pointer*: a reference to "FR-6" is not made to mean "FR-17" by the FR-16 definition, so these back-references read as pointing at a live Must row that no longer exists. Distinct from the correctly-frozen provenance rows (§7.3 decision log line 427, §7.4 changelog line 435, §7.1 FAQ history), which record past decisions verbatim and are not raised.
- **Acceptance condition**: a follow-on amendment section-supersedes the affected prose (§5.1 priority bar, §8 rows, §2.2 anti-segment) to name FR-17 / NG-8 in place of the retired IDs; OR the operator records that the blanket-re-anchor scope is intended to extend to these live-body ID pointers as well, treating them as the sanctioned §12 "Stale-cites-a-superseded-ID" signal reconciled at the next rebase rather than in-amendment — the same posture as the surfaced `stale-serves` flag on the frozen brainstorm-record-model. Either resolution closes it.
- **Disposition**: **declined — recorded via the second acceptance path** (advisory, take-or-leave). The stale ID pointers are the ledger's designed §12 "Stale-cites-a-superseded-ID" signal: `supersede` (unlike `retire`) keeps FR-6 / NG-4 / EVAL-2 resolvable in the superseded appendix with lineage markers, so every reference still resolves — to a row that names its successor. Fixing §5.1's priority-bar prose and the §8 rows in-amendment requires **section supersedes that reproduce whole frozen sections** (the entire §5.1 FR block, the §8 checklist) to refresh a handful of ID tokens — disproportionate churn against large frozen prose for reference hygiene, and exactly the kind of over-broad amendment the one-intent rule discourages. The blanket-re-anchor the operator chose is *semantic* (FR-16 makes every "operator" read principal-parametrically), not a renumber of historical ID pointers. These reconcile at the next rebase/consolidation, alongside the `stale-serves` flag — the live-metadata references (§5.1 ranking, §8 readiness) are the ones a consolidation should refresh first.

### FND-2 — the new Must rows verify against an "EVAL-2 extended" whose row was not re-anchored, leaving the agent-principal fixture obligation implicit in §5.9

- **Severity**: advisory (CAL-4)
- **Lens**: completeness-critic
- **Evidence**: FR-16's fit reads "on the **agent-principal fixtures** … (EVAL-2 extended; NR-class parity)" and FR-17's reads "including … **agent-principal fixtures** … (EVAL-2; zero-tolerance)" (`effective.md` lines 234, 224), and DOD-3 gates on "EVAL-2 at zero failures" — but EVAL-2's own row (line 322) still reads Behavior "decisions only from live **operator** answers", harness "full corpus incl. silence, long-session, and injected-input fixtures (ABUSE-1/2)", with no agent-principal fixture family and no agent-principal negative example. — vs — D-5's ruling was to "re-anchor FR-6 **and the zero-tolerance eval** to decision-principal semantics … fixtures re-anchored, never deleted" (`docs/brainstorms/argus-tech-design.md` D-5): the eval is named as a re-anchor target, and FR-16's own rationale says the definition exists to keep "the whole discipline consistent rather than half-re-anchored" — yet the EVAL-2 row is the one place the re-anchor is only implicit (it rides FR-16's word-redefinition rather than appearing in the row a reader of §5.9 alone would consult). Not load-bearing for §8: the agent-principal verification obligation is captured explicitly in the binding FR-16 / FR-17 rows, so §8 readiness holds via those rows — hence advisory, not blocking under CAL-4.
- **Acceptance condition**: a follow-on amendment supersedes EVAL-2 to name the agent-principal fixture family and re-anchor its threshold/negative-examples to "principal" (adding at least one agent-principal negative, e.g. recording an assumption that failed its legality tests, or dropping the ruling-principal identity); OR an explicit note that FR-16's redefinition governs EVAL-2's "operator" wording, making the propagation intentional and visible to a §5.9-only reader. Either closes it.

- **Disposition**: **applied — in this same amendment** (the first acceptance path). Added `### op: supersede EVAL-2` → **EVAL-6 *(002 ← EVAL-2)***: Behavior re-anchored to "live decision-principal answers (the operator, or Argus under grant — FR-16/FR-17)"; harness gains the **agent-principal fixture family**; threshold adds the ruling-principal-identity assertion; two agent-principal negatives added (an agent recording a decision from unverified content instead of a legality-passed assumption; a decision entry missing its ruling-principal identity) — the sev-1 threshold preserved verbatim. FR-16/FR-17 fit criteria repointed to EVAL-6. This honors D-5's explicit naming of the eval as a re-anchor target and makes the obligation visible to a §5.9-only reader. (Consequence, accepted: prior EVAL-2 references in frozen prose become the same §12 stale-cite signal handled under FND-1.)

## Round 2 (scope-locked)

Examined only the two round-1 dispositions and regressions within the
changed spans (the new `supersede EVAL-2` → EVAL-6 op, the EVAL refs in
FR-16/FR-17, the blast-radius header line). Gate green; `docgate diff`
computes blast radius **contract-changing**, affected EVAL-2 · FR-6 ·
FR-16 · NG-4 — the declared header (now naming EVAL-2) matches.

**FND-2 — closed.** The applied `supersede EVAL-2` → **EVAL-6 *(002 ←
EVAL-2)*** satisfies the first acceptance path in full: Behavior
re-anchored to "live decision-principal answers (the operator, or Argus
under grant — FR-16/FR-17)"; harness gains the **agent-principal**
fixture family; the threshold adds the ruling-principal-identity
assertion and the agent-principal corpus/input clause; two agent-principal
negatives added (a decision from unverified content instead of a
legality-passed assumption; a decision entry missing its ruling-principal
identity); the sev-1 threshold preserved. D-5's named eval re-anchor is
now explicit in §5.9, visible to a §5.9-only reader. FR-17's fit
correctly repointed to EVAL-6.

**FND-1 — decline confirmed, no escalation.** FND-1 is an *advisory*; a
declined advisory does not auto-escalate (only a declined *blocking*
finding does), so the decline is a legal take-or-leave and the review
does not re-open it. The recorded reason (the §12 stale-cite signal,
resolvable-by-design, reconcile at rebase) is a coherent disposition.

**FND-3 — advisory (CAL-3) · coherence · NEW, on a changed span.** FR-16
— a row *this amendment adds* — carries the fit parenthetical "(EVAL-2
extended; NR-class parity)" (`effective.md` line 234), naming the very ID
this same amendment supersedes to EVAL-6. Unlike FND-1's frozen-base
pointers (an accepted §12 signal in immutable prose), this is a
self-inconsistency in *newly-authored* content — the amendment retires
EVAL-2 and, in a row it introduces in the same change, still points at it
— and it is one token to fix. Not blocking: FR-16's fit criterion is
otherwise complete and its EVAL-6 obligation is carried by FR-17 and the
EVAL-6 row, so no Must row is unverifiable (CAL-3 advisory). **Acceptance
condition**: repoint FR-16's fit parenthetical from "EVAL-2 extended" to
"EVAL-6" (or "EVAL-6, extended from EVAL-2"). — **Disposition**: **applied** — FR-16's fit parenthetical repointed "(EVAL-2 extended; NR-class parity)" → "(EVAL-6; NR-class parity)"; the amendment now carries no live reference to the EVAL-2 ID it supersedes, and the round-1 disposition's "FR-16/FR-17 repointed" claim is now true in fact. Re-folded, gate green.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | approve-with-advisories | 2026-07-11 |
| 2 | approve-with-advisories | 2026-07-11 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 0 blocking · 3 advisory (FND-1, FND-2 round 1; FND-3 round 2)
- **Dispositions**: 2 applied (FND-2 → EVAL-6 op; FND-3 → FR-16 ref repointed) · 1 declined-with-reason (FND-1 → §12 stale-cite signal, reconcile at rebase) · 0 withheld-as-violation

## Overlay corrections

*Post-approve entries only — the M2 residual data; frozen rounds and
history are never edited.*

none
