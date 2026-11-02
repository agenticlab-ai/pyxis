# Review: <subject identity> — review report

| Field | Value |
|---|---|
| **In one line** | <the review's outcome distilled — verdict + the load-bearing findings, one sentence> |
| **Relevant-when** | <auditing or consuming the review of <subject>; asking why it was approved/bounced; tuning prd-review calibration against real findings> |
| **Subject** | <typed: document path `prd:<feature>/000` or `prd:<feature>/<NNN>`; draft path `prd:<feature>/draft`> |
| **Moment** | <new-base \| amendment \| draft> |
| **Reviewer** | prd-review · <guide version/date> |
| **Date(s)** | <YYYY-MM-DD, appended per round> |
| **Verdict** | <the live verdict — the history below is authoritative> |
| **Tracking** | <the subject's introducing PR, once filed> |
| **Template version** | review-report/v1 |

**Review subject:** <draft path only: prd; remove this line on document path>
**Draft digest:** <draft path only: sha256: plus 64 lowercase hex from `pyxis draft digest`; remove this line on document path>

## 0. How to use this template

*Guidance italics — delete on instantiation. The contract is
`docs/tech-design/prd-review-artifacts.md`; the behavior that fills it is
bound by `docs/prds/prd-review/`. One report per subject per moment,
forever: re-reviews append a `## Round 2` here (never a new file; two
rounds maximum — the cap is structural). The report travels in the same
introducing PR as its subject and freezes with it; after an approve, the
only legal append is an Overlay-corrections entry. Findings carry five
mandatory elements; severity must cite a calibration row (`CAL-n`).
`FND-n` anchors are dense across rounds, cited externally as
`review:<feature>-<NNN>#FND-3`.*

*For an open feature draft, instantiate this report as the next
`<draft-root>/<feature>/reviews/NNN.md`. Replace the two standalone
draft-only lines above with `**Review subject:** prd` and the exact
`**Draft digest:** sha256:<hex>` printed for the committed PRD and tech
spec. A new contract digest gets a new numbered report; earlier reports
retain their evidence. Document-pipeline reports omit those lines.*

## Round 1

### FND-1 — <finding gist>

- **Severity**: <blocking | advisory> (CAL-<n>)
- **Lens**: <contract-quality | coherence | upstream-alignment | completeness-critic>
- **Evidence**: <subject quote (§/row)> — vs — <upstream anchor (template rule · principle · incident · record D-n · sibling PRD row)>
- **Acceptance condition**: <the change that closes this finding>
- **Disposition**: <applied | declined — <reason> | violation-withheld>

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | <approve · approve-with-advisories · request-changes · escalate> | <YYYY-MM-DD> |

## Escalation brief

*Present iff a verdict is `escalate`; otherwise `none`.*

- **Conflict**: <the single disputed question, citing FND-n and the decline reason>
- **Options**: <each with its risk>
- **Evidence**: <refs — both positions, the spans, the upstream anchors>
- **Decision needed**: <the one question for the authority>

## Measurements

- **Rounds**: <n>
- **Findings**: <b> blocking · <a> advisory
- **Dispositions**: <x> applied · <y> declined · <z> withheld-as-violation

## Overlay corrections

*Post-approve entries only — the M2 residual data; frozen rounds and
history are never edited.*

none

---

## Appendix — structure checklist *(walked at each verdict and by PR review; delete from instantiated reports)*

- [ ] Header complete: routing pair, Subject (typed), Moment, Reviewer, Date(s), Verdict, Tracking, Template version
- [ ] One `## Round N` section per recorded round; never more than two
- [ ] Every `FND-n` carries all five elements — severity **with a `CAL-n` citation**, lens, evidence (subject quote + upstream anchor), acceptance condition, disposition
- [ ] Severity agrees with the cited calibration row
- [ ] Every blocking finding has a disposition before a round-2 or terminal verdict; declined dispositions carry reasons
- [ ] Verdict history is append-only with exactly one live verdict (the header mirrors the last row)
- [ ] The escalation brief is present iff a verdict is `escalate`, and cites its findings
- [ ] Measurements agree with the finding blocks
- [ ] Overlay corrections appear only after an approve, attributed and dated
- [ ] `FND-n` anchors dense across rounds; no reuse
- [ ] No finding restates anything the docgate gate suite enforces
