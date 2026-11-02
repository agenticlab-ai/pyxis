# Review: <design slug> — design-review report

| Field | Value |
|---|---|
| **In one line** | <the review's outcome distilled — verdict + the load-bearing findings, one sentence> |
| **Relevant-when** | <auditing or consuming the review of <design>; asking why it was approved/bounced; tuning tech-design-review calibration> |
| **Subject** | design:<slug> · shape: <present \| none> · supersedes: <target slug(s) \| —> |
| **Moment** | <new-design \| supersession> |
| **Reviewer** | tech-design-review · <guide version/date> |
| **Date(s)** | <YYYY-MM-DD, appended per round> |
| **Verdict** | <the live verdict — the history below is authoritative> |
| **Tracking** | <the subject's introducing PR, once filed> |
| **Template version** | design-review-report/v1 |

## 0. How to use this template

*Guidance italics — delete on instantiation. The contract is
`docs/tech-design/tech-design-review-artifacts.md`; the behavior is bound
by `docs/prds/tech-design-review/`. One report per design subject at
`docs/reviews/<design-slug>.md` — supersession reviews live in the
successor's report; re-reviews append `## Round 2` (two rounds maximum).
Same-PR travel, freeze-at-merge, overlay-corrections-only after approve.
On pair subjects every evidence quote names its half (`design:` or
`shape:`); a supersession-integrity finding cites ≥ 1 target anchor and
never disputes target content. Cited externally as
`review:<design-slug>#FND-2`.*

## Round 1

### FND-1 — <finding gist>

- **Severity**: <blocking | advisory> (CAL-<n>)
- **Lens**: <contract-quality | coherence | upstream-alignment | feasibility-completeness>
- **Evidence**: <design:/shape: quote (§/anchor)> — vs — <upstream anchor (PRD row · principle · incident · record ruling · target-design anchor for supersession findings)>
- **Acceptance condition**: <the change that closes this finding>
- **Disposition**: <applied | declined — <reason> | violation-withheld>

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | <approve · approve-with-advisories · request-changes · escalate> | <YYYY-MM-DD> |

## Escalation brief

*Present iff a verdict is `escalate`; otherwise `none`.*

- **Conflict**: <the single disputed question, citing FND-n + the decline reason>
- **Options**: <each with its risk>
- **Evidence**: <refs>
- **Decision needed**: <the one question for the authority>

## Measurements

- **Rounds**: <n>
- **Findings**: <b> blocking · <a> advisory
- **Dispositions**: <x> applied · <y> declined · <z> withheld-as-violation

## Overlay corrections

none

---

## Appendix — structure checklist *(walked at each verdict and by PR review; delete from instantiated reports)*

- [ ] Header complete — incl. Subject's `shape:` and `supersedes:` fields and the Moment
- [ ] One `## Round N` per recorded round; never more than two
- [ ] Every `FND-n` five-elemented — severity with a `CAL-n` citation, lens, evidence, acceptance condition, disposition
- [ ] On pair subjects, every evidence quote names its half (`design:` / `shape:`)
- [ ] Every supersession-integrity finding cites ≥ 1 target anchor — and none disputes target content
- [ ] Severity agrees with the cited calibration row; blocking findings disposed before a round-2/terminal verdict; declines carry reasons
- [ ] Verdict history append-only, one live verdict, header mirrors the last row
- [ ] The brief present iff escalate, citing its findings
- [ ] Measurements agree with the finding blocks; overlay corrections only after an approve
- [ ] `FND-n` dense across rounds; no reuse; nothing restates the `lint-tech-designs` floor
