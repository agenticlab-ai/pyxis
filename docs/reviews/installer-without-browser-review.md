# Review: installer system release — design-review report

| Field | Value |
|---|---|
| **In one line** | Approved after separating the inherited matrix timing target from correctness acceptance. |
| **Relevant-when** | reviewing the self-contained installer payload, manifest compatibility, or successor-design integrity |
| **Subject** | design:installer-without-browser-review · shape: present · supersedes: installer-v1 |
| **Moment** | supersession |
| **Reviewer** | tech-design-review · 2026-09-17 guide |
| **Date(s)** | 2026-09-17 |
| **Verdict** | approve |
| **Tracking** | prerequisite of https://github.com/agenticlab-ai/pyxis/pull/1199 |
| **Template version** | design-review-report/v1 |

## Round 1

Four independent lenses examined the linted design, its paired solution shape, the approved installer amendment, the frozen predecessor, and the governing corpus. Human-only §0 and the mechanical gate floor were excluded. The finding below survived independent adversarial screening. Contract-quality and upstream-alignment produced no additional surviving candidate. A feasibility candidate asking A-13 to repeat FR-31's payload-absence inspection was discarded because the traced requirement already binds that evidence and §3.3 already requires the deletions; ownership-class globs do not authorize forbidden payload content.

### FND-1 — Separate matrix correctness from duration

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: design: A-3 inherited installer-v1 §6's condition that the matrix end green “at the install budget,” while successor §3.12 declares timing advisory. Approved installer NFR-9 and DOD-14, and AP-29, require correctness independent of duration. The predecessor is evidence of the inherited condition, not the finding's object.
- **Acceptance condition**: require matrix correctness independently of duration and record NFR-9 measurements separately as advisory evidence.
- **Disposition**: applied — A-3 now states the independent correctness verdict and separate duration record.

## Round 2

Scope-locked review of the supplied changed spans closes FND-1. A-3 agrees with NFR-9, DOD-14, and AP-29. The author's additional A-13 clarification repeats FR-31's complete-operation and payload/fresh-install absence evidence, with historical-record and independent-tool boundaries intact. Adding FR-32 to the header makes its existing §3.1 source-resolution decision explicit in the declared relationship set. No changed-span regression survived any lens. The revised design passes lint.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-09-17 |
| 2 | approve | 2026-09-17 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 1 blocking · 0 advisory
- **Dispositions**: 1 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

none
