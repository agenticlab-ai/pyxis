# Argus ledger docgate lint — problem ledger

| Field | Value |
|---|---|
| **Slug** | argus-ledger-lint |
| **Principal** | @mdarabi (operator) — handed the idea over |
| **Authority-contract version** | authority-contract/v1 (tier: floor) |
| **Envelope override** | — (the repo default governs) |
| **Template version** | problem-ledger/v1 |

## Idea

The Argus ledger classes — problem-ledger, delivery-ledger, and authority-contract — have no mechanical lint yet; their design left it as open question Q-1. Two of their invariants can't be caught by a per-file structure check: an edited append-only entry (that needs git history to see) and a downstream document citing a ledger's regenerable derived-state block (that needs a cross-document scan). I want a docgate lint that catches these two — a history arm and a cross-document arm — so the ledger classes get the same gate protection every other document class already has.

## Authority entries

none — no pre-ruling or envelope override was recorded at intake; the idea above is the sole intake record. The `A-n` family opens on the first breaking-call answer, should one arise.
