# Review: standalone source retirement — design-review report

| Field | Value |
|---|---|
| **In one line** | Approved after making multi-source issuance and exact retries executable without weakening mainline authority. |
| **Relevant-when** | reviewing exact retirement seals for ownerless skill entries and vendor READMEs |
| **Subject** | design:memory-standalone-source-retirement · shape: none · supersedes: — |
| **Moment** | new-design |
| **Reviewer** | tech-design-review · 2026-09-18 guide |
| **Date(s)** | 2026-09-18 |
| **Verdict** | approve |
| **Tracking** | prerequisite of https://github.com/agenticlab-ai/pyxis/pull/1199 |
| **Template version** | design-review-report/v1 |

## Round 1

Four independent lenses examined the linted design, the governing corpus, current writer and source-retirement checks, and the trusted-mainline boundary. Human-only §0 and the mechanical gate floor were excluded. The coherence candidate below survived independent adversarial screening. The other lenses produced no surviving finding.

### FND-1 — Make two-source issuance and retry executable

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: design: §2 and §3.1 required HEAD to equal mainline, clean source/memory state, and one receipt per command; §3.5 required both receipts on one branch. The first write dirtied memory, while committing it made HEAD topic-only. The next issuance and an immediate identical retry had no specified permitted path.
- **Acceptance condition**: specify a complete issuance sequence for both sources and exact retry handling, preserving refusal of unrelated dirty memory and topic-only source evidence; exercise the sequence through the production writer.
- **Disposition**: applied — the author added repeatable source arguments, whole-batch preflight at one clean mainline snapshot, and a no-write retry limited to the exact complete pending output of the same request. A-1 and A-2 cover the accepted sequence and refusal boundaries.

## Round 2

Scope-locked review of §2, §3.1, §3.5, A-1, and A-2 closes FND-1. All four lenses found no changed-span regression. Partial, altered, unrelated, or extended dirty batches remain ineligible; the rollout issues both seals in one batch. The revised design passes lint. Implementation and acceptance execution remain separate work.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-09-18 |
| 2 | approve | 2026-09-18 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 1 blocking · 0 advisory
- **Dispositions**: 1 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

none

### 2026-09-18 — system-root notation

The documentation gate required system-footprint paths to be expressed without embedding their physical root in a design. The author changed the two path descriptions in §1 and §2 to vendor-neutral root-relative names. Eligible files, authority, and acceptance remain identical; this is a mechanical path-notation correction, not a new review round.
