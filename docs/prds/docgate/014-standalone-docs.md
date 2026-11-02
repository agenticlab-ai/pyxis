# Amendment 014 — one concern per PR: governed docs ship standalone, implementation stacks

| Field | Value |
|---|---|
| **Amends** | docgate (chain: 000 → 013) |
| **Status** | Approved |
| **Owner** | @mdarabi |
| **Date** | 2026-07-10 |
| **Learned-from** | review-thread:pr-82-bundled-amendment-design-impl · operator-decision:2026-07-10-standalone-doc-prs |
| **Blast radius** | tier: additive; touches: FR-30; reason: adds a gate verb — no existing unit changes |
| **Template version** | amendment-template/v1 |

## Operations

### op: add FR

Reason: FR-27 caps a PR at one PRD ledger entry, but nothing stops that entry riding with a technical design and implementation code — PR #82 bundled an amendment, a new design, and the impl into one change and passed the gate green. "One concern per PR" (AGENTS.md) for the governed doc classes is machine-checkable, the same reviewer-norm-to-gate move as FR-10 and FR-27, and it makes FR-16's PRD-precedes-design ordering real (the contract is approved on its own before code is built against it) rather than trivially satisfied in a bundle.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-30 | When invoked with a merge base, docgate shall fail a diff that touches a governed contract class — a PRD ledger entry (`NNN-<slug>.md`), an authored technical design (`docs/tech-design/*.md`, excluding the generated `index.*`), or the incident registry — if the same diff also touches implementation/capability paths or a second governed class; the generated projections (`effective.*`, `index.*`) and the live rollups (`docs/techspec.md`, `docs/prd.md`) are exempt, riding with their source or with the code respectively. | one concern per PR becomes code for the governed classes; the contract is reviewed on its own before code builds against it, so FR-16 precedence holds in fact and not only at lint time | Must | med | temp-repo fixtures: an amendment + a design → red (two classes); an amendment + code → red; a design + code → red; incident + code → red; a lone amendment with its regenerated `effective.*` → green; a lone design with its regenerated `index.*` → green; a pure code or rollup change → green | Test | J3, G1 |
