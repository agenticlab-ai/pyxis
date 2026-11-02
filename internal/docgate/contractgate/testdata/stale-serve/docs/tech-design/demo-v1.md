# demo v1 — a stale-serve fixture — technical design

| Field | Value |
|---|---|
| **Status** | Approved |
| **Owner** | @fixture |
| **Approver(s)** | @fixture |
| **Date** | 2026-07-17 |
| **Relevant-when** | a contract-gate currency test fixture; never a real design |
| **Scope** | fixture only |
| **Tracking** | — |
| **Builds-on** | — |
| **Supersedes** | — |
| **Serves** | prd:demo/FR-1 · prd:demo/FR-2 |
| **Security/privacy impact** | none — a test fixture |
| **Template version** | tech-design-template/v1 |

## 1. Problem

A live serving design that serves prd:demo/FR-1 (live) and prd:demo/FR-2 (which
has since been superseded by FR-3). The currency check reads the shared
stale-serves computation and reds `stale`, naming this design and the
superseded anchor prd:demo/FR-2. FR-1 stays served so inverse-coverage is clean.

## 3. The design

### Observability

none — a fixture.

### Security & privacy

none — a fixture.
