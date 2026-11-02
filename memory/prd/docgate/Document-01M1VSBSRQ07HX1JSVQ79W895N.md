# Amendment 009 — envelope v2: a Tracking hook, validated by declared version

| Field | Value |
|---|---|
| **Amends** | docgate (chain: 000 + 001 + 002 + 003 + 004 + 005 + 006 + 007 + 008) |
| **Status** | Approved |
| **Date** | 2026-07-08 |
| **Owner** | @mdarabi |
| **Learned-from** | review-thread:issue-54 · review-thread:issue-47 |
| **Blast radius** | tier: additive; touches: FR-24; reason: adds versioned-envelope support; v1 amendments stay valid forever |
| **Template version** | amendment-template/v1 |

## Operations

### op: add FR

Reason: amendment 005 merged a requirement deliberately decoupled from its
implementation, and the envelope had nowhere to name the tracker — the only
tracking artifact (issue #47) was closed by convention drift and confusion
followed (#54 A1's process half). The envelope gains an optional `Tracking`
row under a bumped template version; validation keys off each amendment's
**declared** version (the grandfathering pattern the design class already
uses), so merged v1 amendments never need retrofitting. Per the template's
own appendix rule, the `amendment-template/v2` bump ships with updated
docgate goldens in the same change.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-24 | When validating an amendment envelope, docgate shall validate against the schema of the envelope's declared template version from the supported set — `amendment-template/v1` (the seven base fields) and `amendment-template/v2` (v1 plus an optional `Tracking` row carrying an issue or PR reference) — and shall fail any envelope declaring a version outside the set or carrying fields unknown to its declared version. | an amendment whose implementation lands later needs a durable pointer to what tracks that work; version-keyed validation adds it without touching frozen history (#54 A1/C1) | Must | med | a v2 fixture with `Tracking` green; a v2 fixture with an unknown field red; a v1 fixture without `Tracking` green (grandfathered forever); a v1 fixture **with** `Tracking` red (unknown to v1); a declared v3 red; the real ledger green | Test | J1, J3 |
