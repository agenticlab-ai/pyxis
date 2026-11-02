# demo v1 — an uncovered-Must fixture — technical design

| Field | Value |
|---|---|
| **Status** | Approved |
| **Owner** | @fixture |
| **Approver(s)** | @fixture |
| **Date** | 2026-07-17 |
| **Relevant-when** | a contract-gate inverse-coverage test fixture; never a real design |
| **Scope** | fixture only |
| **Tracking** | — |
| **Builds-on** | — |
| **Supersedes** | — |
| **Serves** | prd:demo/FR-2 |
| **Security/privacy impact** | none — a test fixture |
| **Template version** | tech-design-template/v1 |

## 1. Problem

A live serving design that serves prd:demo/FR-2 but not prd:demo/FR-1. FR-1 is a
live Must served by no live design, so the inverse-coverage check reuses the
coverage join and reds `incomplete`, naming FR-1.

## 3. The design

### Observability

none — a fixture.

### Security & privacy

none — a fixture.
