# Amendment 014 — pre-push deletion awareness: a ref deletion carries no content

| Field | Value |
|---|---|
| **Amends** | docgate (chain: 000 → 014) |
| **Status** | Approved |
| **Date** | 2026-07-09 |
| **Owner** | @mdarabi |
| **Learned-from** | review-thread:issue-65 · incident:I2 |
| **Blast radius** | tier: additive; touches: FR-29; reason: adds hook-wiring behavior for a push class the gate misread; every content-push verdict is unchanged |
| **Tracking** | issue #65 |
| **Template version** | amendment-template/v2 |

## Operations

### op: add FR

Reason: deleting fully-absorbed branches — now routine, since the
merge-train rule prescribes delete-on-merge (incident I2) — trips the
push hook: a deletion push carries no content, but the shim runs the full
merge-base gate over the *checkout's* tree, and `check-immutable` reads
whatever that checkout lacks as mass deletion (issue #65, observed
2026-07-09: seven "ledger entry … is deleted" defects per branch-delete
attempt, forcing `--no-verify`). The hook protocol already says which
refs a push carries — an all-zeros local SHA is a deletion — so the
wiring gains a verb that reads it.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-29 | When invoked as `pre-push` by the push hook, docgate shall read the hook's ref lines from stdin and, when every pushed ref is a deletion (an all-zeros local SHA), pass without running content checks, saying so; when any non-deletion ref is pushed — or stdin carries no parseable ref lines — it shall run the merge-base gate exactly as today. | a ref deletion carries no content to gate; everything else fails closed to the full gate (issue #65) | Should | low | fixtures: a deletion-only stdin exits 0 with the skip notice; a mixed push runs the gate; a content push keeps today's verdicts; empty or malformed stdin runs the gate; the shim stays delegate-only and bash-3.2-clean | Test | G2, J3 |
