# Amendment 012 — single amendment per PR: one ledger entry per change

| Field | Value |
|---|---|
| **Amends** | docgate (chain: 000 → 011) |
| **Status** | Approved |
| **Owner** | @mdarabi |
| **Date** | 2026-07-09 |
| **Learned-from** | review-thread:pr-56-bundled-docgate-and-incident-machinery · operator-decision:2026-07-09-one-amendment-per-pr |
| **Blast radius** | tier: additive; touches: FR-27; reason: adds a gate verb — no existing unit changes |
| **Template version** | amendment-template/v1 |

## Operations

### op: add FR

Reason: "one concern per PR" (AGENTS.md) and the amendment model's "one intent per amendment" were prose only — PR #56 bundled six docgate amendments and a whole new incident-machinery ledger into one change and no gate refused it; the presence of more than one added ledger entry is machine-checkable, the same reviewer-norm-to-gate move as FR-10.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-27 | When invoked with a merge base, docgate shall fail a diff that adds more than one PRD ledger entry — an added `NNN-<slug>.md` file one directory below the configured PRDs root — naming each added entry and the remedy; the generated projection, the flat placeholder corpus (NG-1), and modifications of existing entries (already fenced by FR-4) do not count. | one concern per PR becomes code for the ledger: one amendment, or one new base, per change keeps concurrent claims content-free and reviews honest | Must | low | temp-repo fixtures: a diff adding two entries (one ledger or two) → red naming both and the remedy; a diff adding exactly one amendment → green; a new PRD's lone `000-base.md` → green; a projection-only regeneration with no added entry → green; a flat placeholder is never counted | Test | J3, G1 |
