# Review: <feature> — e2e-review report

| Field | Value |
|---|---|
| **In one line** | <the review's outcome distilled — verdict + the load-bearing findings, one sentence> |
| **Relevant-when** | <auditing or consuming the review of <feature>'s e2e design; asking why it was approved/bounced; tuning e2e-review calibration against real findings> |
| **Subject** | e2e-design:<feature> |
| **Moment** | review |
| **Reviewer** | e2e-review · <guide version/date> |
| **Date(s)** | <YYYY-MM-DD, appended per round> |
| **Verdict** | <the live verdict — the history below is authoritative> |
| **Tracking** | <the subject's introducing change, once filed> |
| **Template version** | e2e-review-report/v2 |

## 0. How to use this template

*Guidance italics — delete this section, and the Appendix below, on
instantiation: the section set is closed, so a retained `## 0.` or
`## Appendix` heading fails the parser at `unknown-section`. The contract is
`docs/tech-design/e2e-verification-artifacts-v2.md` §3.6 — this schema is its
own instance in the review-report family (sharing the closable-finding and
versioned-report grammar of `review-report/v1` and `design-review-report/v1`
rather than restating it), never a verbatim reuse of either sibling's
closed, foreign-typed header. The behavior that fills it is bound by
`design:e2e-verification-skills-v2` §3.2. One report per subject, forever: a
re-review appends a `## Round 2` here (never a new file; two rounds
maximum — the cap is structural). The report is co-located with the design
doc it reviews at `docs/e2e/<feature>/review.md` and rides the design
doc's introducing change (`prd:e2e-verification/FR-11`); after the design
doc's Status moves to `approved`, the only legal append is an
Overlay-corrections entry.

Findings carry five mandatory elements; severity must cite a calibration
row (`CAL-n`). `Lens` is `coverage`, `anti-gaming`, or `buildability`
(`prd:e2e-verification/FR-24`) — this instance's own customization of the
family's lens field, distinct from the sibling reviewers' lens sets.
`Disposition` is `open` (still converging), `resolved` (the design doc was
edited to close the finding), or `rejected — <reason>`
(`prd:e2e-verification/FR-10`) — this instance's own disposition
vocabulary, distinct from the sibling reviewers'
`applied`/`declined`/`violation-withheld`. No finding may stay `open` once
the live Verdict is `approve` or `approve-with-advisories`: every finding
is resolved or rejected before the design doc is approved
(`prd:e2e-verification/UB-5`, `design:e2e-verification-skills-v2#DINV-10`).

`Moment` is `review` — the design doc's only live v2 review moment. The
design doc's post-approval mutability posture is successor-only (never in
place), but the successor-authoring path is deferred
(`design:e2e-verification-artifacts-v2` §7 Q1), so no successor-review
moment exists yet to record; the field is extensible additively when that
path ships. `FND-n` anchors are dense across rounds, cited externally as
`review:<feature>#FND-3`.*

## Round 1

### FND-1 — <finding gist>

- **Severity**: <blocking | advisory> (CAL-<n>)
- **Lens**: <coverage | anti-gaming | buildability>
- **Evidence**: <design doc quote (§/coverage-map row/scenario/harness slot)> — vs — <upstream anchor (PRD requirement row · the live design set · the built code)>
- **Acceptance condition**: <the change that closes this finding>
- **Disposition**: <open | resolved | rejected — <reason>>

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | <approve · approve-with-advisories · request-changes · escalate> | <YYYY-MM-DD> |

## Escalation brief

*Present iff a verdict is `escalate`; otherwise `none`.*

- **Conflict**: <the single disputed question, citing FND-n and the rejection reason>
- **Options**: <each with its risk>
- **Evidence**: <refs — both positions, the spans, the upstream anchors>
- **Decision needed**: <the one question for the authority>

## Measurements

- **Rounds**: <n>
- **Findings**: <b> blocking · <a> advisory
- **Dispositions**: <x> resolved · <y> rejected

## Overlay corrections

*Post-approve entries only — the residual correction record; frozen rounds
and history are never edited.*

none

---

## Appendix — structure checklist *(walked at each verdict and by PR review; delete from instantiated reports)*

- [ ] Header complete: routing pair, Subject (`e2e-design:<feature>`), Moment (`review`), Reviewer, Date(s), Verdict, Tracking, Template version
- [ ] Subject and Reviewer are typed to this instance's own grammar — never a foreign sibling's (`Subject: prd:…`/`design:…` or `Reviewer: prd-review`/`tech-design-review` is a family-conformance defect, not an e2e report)
- [ ] One `## Round N` section per recorded round; never more than two
- [ ] Every `FND-n` carries all five elements — severity **with a `CAL-n` citation**, lens (`coverage` \| `anti-gaming` \| `buildability`), evidence, acceptance condition, disposition
- [ ] Severity agrees with the cited calibration row
- [ ] Disposition is `open`, `resolved`, or `rejected — <reason>`; a rejected disposition always carries a reason
- [ ] No finding stays `open` once the verdict is `approve` or `approve-with-advisories` — every finding is resolved or rejected before the design doc is approved (`prd:e2e-verification/UB-5`)
- [ ] Verdict history is append-only with exactly one live verdict (the header mirrors the last row)
- [ ] The escalation brief is present iff a verdict is `escalate`, and cites its findings
- [ ] Measurements agree with the finding blocks
- [ ] Overlay corrections appear only after an approve, attributed and dated
- [ ] `FND-n` anchors dense across rounds; no reuse
- [ ] No finding restates anything the `e2e-design/v2` well-formedness checker already enforces (`design:e2e-verification-artifacts-v2` §3.5) — a structural gap there is that checker's job, not any lens's
