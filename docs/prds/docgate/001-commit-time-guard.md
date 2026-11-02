# Amendment 001 — commit-time guard: generation holds without a remote

| Field | Value |
|---|---|
| **Amends** | docgate (chain: 000 only) |
| **Status** | Approved |
| **Owner** | @mdarabi |
| **Date** | 2026-07-07 |
| **Learned-from** | operator-decision:2026-07-07-no-remote-generation · review-thread:docgate-launch-qa |
| **Blast radius** | tier: additive; touches: FR-9; reason: adds a local wiring row — no existing unit changes |
| **Template version** | amendment-template/v1 |

## Operations

### op: add FR

Reason: enforcement boundaries (pre-push, CI required context) assume a
remote; with none, no gate moment ever fires and a stale projection can
persist. The projection is a materialized view whose freshness must not
depend on hosting topology: the commit is the only universal boundary, so
the gate gains a commit-time moment. The verbs involved need no merge
base, so this is pure wiring — no fold or verb semantics change.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-9 | When a commit is created in a clone with the hook path activated, the pre-commit hook shall run the merge-base-free gate verbs (lint-structure, validate-amendments, verify-generated, verify-rebase) and reject the commit on any defect. | generation must hold with no remote: the projection is committed fresh or not at all | Should | low | hook fixture: a stale projection rejects the commit naming the regenerate remedy; a fresh one passes; absent Go toolchain skips with the two-line notice | Test | G1, J1 |
