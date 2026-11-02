# demo a — a build-on-cycle fixture (a builds-on b) — technical design

| Field | Value |
|---|---|
| **Status** | Approved |
| **Owner** | @fixture |
| **Approver(s)** | @fixture |
| **Date** | 2026-07-17 |
| **Relevant-when** | a contract-gate compilability build-on-cycle fixture; never a real design |
| **Scope** | fixture only |
| **Tracking** | — |
| **Builds-on** | design:demo-b |
| **Supersedes** | — |
| **Serves** | prd:demo/FR-1 |
| **Security/privacy impact** | none — a test fixture |
| **Template version** | tech-design-template/v1 |

## 1. Problem

Design demo-a builds on demo-b, which builds on demo-a — a build-on cycle the
compilability check must detect (computeGraph flags stale-builds-on but not a
cycle).

## 3. The design

### Observability

none — a fixture.

### Security & privacy

none — a fixture.
