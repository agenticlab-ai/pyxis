# Review: prd:feature-brainstorm/003 (question-legibility) — review report

| Field | Value |
|---|---|
| **In one line** | The question-legibility Must folds cleanly and honestly — FR-18/EVAL-7 well-formed, the envelope declared above its computed tier with exact touches, NG-7's separate-evolution ruling honored — approved with two advisories: EVAL-7 grades no example-fidelity failure (a fabricated "looked-up" example or unmarked hypothetical is ungraded), and the guide + harness work the amendment specifies rides no Tracking row. |
| **Relevant-when** | auditing or consuming the review of feature-brainstorm amendment 003; asking why the question-legibility Must was approved-with-advisories; tuning prd-review calibration against real amendment findings (eval negative-example coverage of a requirement's own clauses; the optional Tracking row's keep-condition) |
| **Subject** | prd:feature-brainstorm/003 |
| **Moment** | amendment |
| **Reviewer** | prd-review · guide 2026-07 (.pyxis/authoring/prd-review.md) |
| **Date(s)** | 2026-07-30 |
| **Verdict** | approve-with-advisories |
| **Tracking** | the amendment's introducing PR (filed on approval) |
| **Template version** | review-report/v1 |

## Round 1

Subject verified: a validated amendment (003) reviewed as its folded effect
(`effective.md` as of 003, fingerprint `sha256:d255d042…` matching `docgate
diff`). Floor excluded: `docgate gate` exits 0; fold and diff are green.
The computed blast radius is **additive** (affected: EVAL-7 · FR-18) while
the declared tier is **contract-changing** — declaring above computed is the
envelope's one legal direction, the declared tier sets the review bar, and
here the declaration is the honest reading (the envelope's own tier prose
classes anything touching a Must row as contract-changing; the computation
under-tiers an `add` of a Must row); the declared touches list matches the
computed affected set exactly. The **Learned-from** resolves in the corpus's
established style for `operator-decision` references (a dated, attributed,
content-stated reference — the same form as docgate/001's
`2026-07-07-no-remote-generation` and init-hardening/001's session
reference), and the ruling session's log
(`logs/2026-07-30-1548-brainstorm-question-legibility.md`) rides this same
PR. Chain integrity holds: chain through 002, dense 000→003, exactly one new
ledger entry in the PR. Upstream sweep: no incident (I1–I6) is implicated or
repeated; NG-7's separate-evolution ruling is honored (the sibling
discipline lands in the tech-design-brainstorm ledger in its own change —
nothing from that ledger rides here); the record's own register is
explicitly left untouched (the FR-18 NFR-2 clause), so no R2
second-source-of-truth pressure; and the agent-principal exemption is
consistent with FR-16 (an agent principal's interview is the assumption
regime — plain-register glossing is a property of the human surface only).

Not re-raised, as the settled `review:feature-brainstorm-002#FND-1` class:
the frozen-base summary prose that does not name the new row — §5.1's
priority-bar ranking, §8's "FR-1..15" enumerations, DOD-1's source list —
is the recognized stale-pointer signal disposed reconcile-at-rebase there;
DoD coverage of FR-18 holds regardless through DOD-1's quantified criterion
("All Must FRs pass their fit criteria") and DOD-3's §5.9 sweep, which
EVAL-7 joins.

### FND-1 — EVAL-7 exercises every FR-18 clause except example fidelity: a fabricated "looked-up" example or an unmarked hypothetical is ungraded

- **Severity**: advisory (CAL-4)
- **Lens**: completeness-critic
- **Evidence**: FR-18 mandates the example be "looked up from the corpus,
  code, or prior records where one exists, marked hypothetical otherwise"
  (003, op: add FR), yet EVAL-7's threshold enumerates only form-presence,
  anchor-gloss, outcome-distinctness, and skip-legitimacy checks, and its
  five negative examples (bare-ID question · outcome-less options ·
  per-option different examples · forced example instead of a typed skip ·
  buried recommendation) exercise every FR-18 clause except this one — no
  negative or threshold clause grades an example presented as looked-up that
  the corpus does not contain, or an invented example not marked
  hypothetical. — vs — the ledger's own fabrication posture: EVAL-4's "zero
  fabricated resolutions", FR-5's lookup-with-source pattern (a question
  turn's example is fact-class content put before the principal), and
  ABUSE-1/2's premise that model-carried content steering rulings is the
  attack surface. A plausible invented example steers the principal's ruling
  on false premises — the highest-consequence FR-18 failure — and it is the
  one clause a grader cannot check from the transcript alone (FR-18 requires
  provenance of origin but no inline source), so left out of the graded
  negatives it is silently uncovered. Not load-bearing for §8: the binding
  obligation lives in FR-18's Must text, so readiness holds — advisory, not
  blocking, under CAL-4.
- **Acceptance condition**: while the introducing PR is open, extend the 003
  EVAL-7 row with an example-fidelity check — e.g. the threshold gains "zero
  looked-up-presented examples that do not resolve to the corpus; zero
  unmarked hypotheticals" and the negatives gain "a plausible 'looked-up'
  example the corpus does not contain" / "an invented example not marked
  hypothetical" — OR record in the row that example fidelity is deliberately
  assigned to the report-time judgment pass. Either closes it.
- **Disposition**: open — advisory, the author's to take or leave

### FND-2 — the amendment specifies guide + harness work its introducing PR does not land, and the envelope carries no Tracking row to name where that work lives

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: the envelope (amendment-template/v2) omits the optional
  **Tracking** row, while the introducing PR lands the contract only
  (amendment + projection + session log): the skill guide holds none of the
  FR-18 form today (no plain-register, gloss, or example-anchored discipline
  in `.pyxis/authoring/feature-brainstorm.md`) and the fixture harness
  grades no EVAL-7 property — and the feature's status is
  post-implementation with the skill in live use, so FR-18 merges as a Must
  the live skill does not yet meet, with the closing work unnamed. — vs —
  the template's own keep-condition: "**Tracking** … names the tracker for
  work this amendment specifies but does not itself land — an implementation
  to follow … delete the row if the introducing PR is the whole work" —
  which this PR does not satisfy; live precedent uses the row for exactly
  this contract-first shape (contract-gate/001 · docgate/015). Advisory, not
  blocking: the row is optional by template, and the obligation itself is
  held by DOD-1/DOD-3 and the pending launch-review checkpoint (§6.7) —
  what is missing is the visibility pointer, not the obligation.
- **Acceptance condition**: add the Tracking row naming where the guide +
  eval-harness work lands (issue or PR); OR land those edits in this same
  introducing PR; OR record that the pending launch-review checkpoint
  (§6.7, DOD-1/DOD-3) is deliberately the sole tracker for the gap. Any of
  the three closes it.
- **Disposition**: open — advisory, the author's to take or leave

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | approve-with-advisories | 2026-07-30 |

## Escalation brief

none

## Measurements

- **Rounds**: 1
- **Findings**: 0 blocking · 2 advisory
- **Dispositions**: 0 applied · 0 declined · 0 withheld-as-violation (both advisories open — take-or-leave)

## Overlay corrections

*Post-approve entries only — the M2 residual data; frozen rounds and
history are never edited.*

none
