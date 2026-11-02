# Amendment 002 — rollup guard: capability diffs update the live docs

| Field | Value |
|---|---|
| **Amends** | docgate (chain: 000 + 001) |
| **Status** | Approved |
| **Owner** | @mdarabi |
| **Date** | 2026-07-07 |
| **Learned-from** | review-thread:pr-30-late-rollup-seeding · operator-decision:2026-07-07-enforce-rule-3 |
| **Blast radius** | tier: additive; touches: FR-10; reason: adds a gate verb — no existing unit changes |
| **Template version** | amendment-template/v1 |

## Operations

### op: add FR

Reason: AGENTS.md rule 3 (live rollups updated per capability PR) was
reviewer-enforced and failed four PRs running, caught by the operator at
#30. Presence of a rollup update is machine-checkable; truth of its
content stays with review — the standard structural/semantic tier split.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-10 | When invoked with a merge base, docgate shall fail a diff that touches capability paths (cmd/, internal/, .githooks/, .github/workflows/ — excluding *_test.go and testdata/) without touching docs/prd.md or docs/techspec.md. | rule 3 becomes code, not reviewer memory; a Last-verified bump is the auditable no-capability-change attestation | Must | low | temp-repo fixtures: capability diff without rollup diff → red naming the remedy; with a rollup touch → green; test-only diff → green | Test | J3, G1 |
