# Review: implement-now amendment 008 — review report

| Field | Value |
|---|---|
| **In one line** | Approved with advisory: read boundary and frozen-citation impact clarified; semantic intake quality remains unmeasured. |
| **Relevant-when** | auditing direct issue admission, its selection boundary, or the amendment's compatibility impact |
| **Subject** | prd:implement-now/008 |
| **Moment** | amendment |
| **Reviewer** | prd-review · 2026-10-01 |
| **Date(s)** | 2026-10-01 |
| **Verdict** | approve-with-advisories |
| **Tracking** | — |
| **Template version** | review-report/v1 |

## Round 1

Four lenses used one docgate context pack with the informative §0 excluded. Independent screening retained the findings below. The candidate demanding separate planning-stage certification was screened out: the operator explicitly authorized an issue lane without per-issue draft/PRD/design/plan artifacts, and the design now states the limited claim it makes about those stages.

### FND-1 — Exact blockers contradict the absolute read prohibition

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: FR-51 requires exact reads of every native dependency, including outside the selection, while restricting reads to the issue/its obligations; UB-22 originally required zero unselected reads. Those statements conflict on an outside blocker. FR-51's stated purpose is to retain that blocker rather than drop it.
- **Acceptance condition**: Permit bounded read-only source-comment/native-dependency facts and shared selection metadata while prohibiting unrelated proposal/artifact lookup, repair, claim, or model execution.
- **Disposition**: applied — FR-51 and UB-22 now distinguish fact reads from execution authority.

### FND-2 — Retiring NR-1 has undeclared frozen consumers

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: The NR-1→NR-11 operation affects `docs/plans/implement-now/001-plan.md` T5/coverage and the Serves header in `implement-now-authenticated-review-obligations`; both cite NR-1. The original blast radius named only the amended rows. Incidents I6/I9 require the owning cross-artifact change and its legal evolution to be explicit; frozen artifacts cannot be rewritten in place.
- **Acceptance condition**: Declare these consumers and their legal successor/compatibility prerequisite; distinguish unchanged runtime fixtures from validity of an old plan fold under the new PRD.
- **Disposition**: applied — amendment blast radius and design §3.8 declare that impact and preserve the frozen consumers.

### FND-3 — Worker evaluations do not measure semantic intake

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: FR-50 introduces invoking-agent issue-to-charter and admit/refuse reasoning; existing EVAL-3 begins with a positive task charter. Compiler fixtures therefore do not establish that the invoking agent extracts intent or recognizes ambiguous/decomposable issues reliably.
- **Acceptance condition**: Add a calibrated semantic issue-intake corpus or explicitly bound the available evidence and defer that measurement without claiming it passed.
- **Disposition**: applied — design §3.8 records that compiler/provider fixtures do not measure this reasoning and retains the semantic corpus as a release-evaluation advisory. This implementation introduces no evaluation service.

## Round 2

Scope-locked review examined the named findings and revised spans only. All four lenses closed FND-1/FND-2 and accepted FND-3's explicit evidence limit. No changed-span regression survived independent screening. Semantic intake evaluation remains an advisory rather than claimed proof.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-10-01 |
| 2 | approve-with-advisories | 2026-10-01 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 2 blocking · 1 advisory
- **Dispositions**: 3 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

none
