# demo b — a build-on-cycle fixture (b builds-on a) — technical design

| Field | Value |
|---|---|
| **Status** | Approved |
| **Owner** | @fixture |
| **Approver(s)** | @fixture |
| **Date** | 2026-07-17 |
| **Relevant-when** | a contract-gate compilability build-on-cycle fixture; never a real design |
| **Scope** | fixture only |
| **Tracking** | — |
| **Builds-on** | design:demo-a |
| **Supersedes** | — |
| **Serves** | prd:demo/FR-1 |
| **Security/privacy impact** | none — a test fixture |
| **Template version** | tech-design-template/v1 |

## 1. Problem

Design demo-b builds on demo-a, which builds on demo-b — a build-on cycle the
compilability check must detect.

## 3. The design

### Observability

none — a fixture.

### Security & privacy

none — a fixture.
