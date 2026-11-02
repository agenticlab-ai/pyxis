# other v1 — an unrelated design so the design dir exists — technical design

| Field | Value |
|---|---|
| **Status** | Approved |
| **Owner** | @fixture |
| **Approver(s)** | @fixture |
| **Date** | 2026-07-17 |
| **Relevant-when** | a contract-gate UB-5 test fixture; never a real design |
| **Scope** | fixture only |
| **Tracking** | — |
| **Builds-on** | — |
| **Supersedes** | — |
| **Serves** | corpus:SCAF-1 |
| **Security/privacy impact** | none — a test fixture |
| **Template version** | tech-design-template/v1 |

## 1. Problem

A live design that serves an unrelated corpus requirement, present only so the
tech-design directory exists (an unreadable design directory would be a
configuration-error, not UB-5). No live design serves the demo feature, so the
demo contract has zero live serving designs — every live Must (FR-1) is unserved
and the inverse-coverage check reds `incomplete` (UB-5), never vacuously clean.

## 3. The design

### Observability

none — a fixture.

### Security & privacy

none — a fixture.
