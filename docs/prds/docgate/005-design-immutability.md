# Amendment 005 — design immutability: INV-41's CI half for the tech-design class

| Field | Value |
|---|---|
| **Amends** | docgate (chain: 000 + 001 + 002 + 003 + 004) |
| **Status** | Approved |
| **Date** | 2026-07-07 |
| **Owner** | @mdarabi |
| **Learned-from** | review-thread:issue-47 · operator-decision:conventions-3.4-promised-scope |
| **Blast radius** | tier: additive; touches: FR-17; reason: adds a fence for a second document class — FR-4 (the PRD fence) is untouched |
| **Template version** | amendment-template/v1 |

## Operations

### op: add FR

Reason: INV-41 requires design immutability at both CI and prompt level;
the conventions (§3.4) promised the CI half as docgate-v2 scope, but the
v2 design dropped it and amendment 003 never carried it — so approved
designs can be rewritten in place with nothing but review to catch it
(issue #47; same under-coverage family as incident I1). The fence
freezes on merge, so open-PR drafts stay editable; the generated index
artifacts are exempt — they are projections legitimately rewritten by
every design PR and guarded by verify-index, not by immutability. The
implementation also routes both fences through the config's paths
(check-immutable currently hardcodes docs/prds, ignoring the
issue-4 surface — a side-defect this closes).

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-17 | When invoked as `check-immutable` with a merge base, docgate shall fail any modify, delete, or rename of a technical-design document present at the merge base under the configured tech-designs path, excluding the generated index artifacts, with the remedy naming supersession. | INV-41's CI half for the design class; merge is the freezing event | Must | high | fixture: modify/delete/rename of a merged design each red naming supersession; a design new in the PR edits green; index.md modification green; the PRD fence unaffected | Test | G1, J3 |
