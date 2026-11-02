# Review: memory-causal-admission-v1 — design-review report

| Field | Value |
|---|---|
| **In one line** | Approved: the shared history guard and retained causal/immutability checks support removing unrelated record-age refusals. |
| **Relevant-when** | reviewing rebase-stable memory admission and the evidence behind its design approval |
| **Subject** | design:memory-causal-admission-v1 · shape: present · supersedes: — |
| **Moment** | new-design |
| **Reviewer** | tech-design-review · repository guide inspected 2026-09-17 |
| **Date(s)** | 2026-09-17 |
| **Verdict** | approve |
| **Tracking** | Local introducing proposal; external PR not yet created |
| **Template version** | design-review-report/v1 |

## Round 1

Four lenses ran concurrently: independent contract-quality, coherence, and
upstream-alignment agents, with feasibility-completeness examined by the
coordinator against the existing resolver, packet, writer-admission seams,
and exact malicious packet fixtures. The subject's human-only §0 was
excluded. No blocking or advisory findings survived adversarial screening.

The review checked preservation of explicit relation chronology, rejection
of retroactive effects through actual honored edges, body-aware Document
trial projection, coverage of every writer, and immutable candidate bytes.
The design's A-4 already accounts for earlier successful independent writes
in a reconciliation batch; it does not promise batch atomicity.

The upstream lens also checked the added `prd:parallel-memory` requirement
joins before this verdict. Each referenced row has substantive design/shape
coverage. Design lint, the current authoring-tree `docgate gate`, and the
feature's `contract-doc-pr` check pass. These are design/contract findings;
the specified behavior has not been implemented or runtime-validated here.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | approve | 2026-09-17 |

## Escalation brief

none

## Measurements

- **Rounds**: 1
- **Findings**: 0 blocking · 0 advisory
- **Dispositions**: 0 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

none
