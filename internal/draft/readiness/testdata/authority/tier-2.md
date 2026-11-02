# Argus authority contract

| Field | Value |
|---|---|
| **Current tier** | tier-2 |
| **Template version** | authority-contract/v1 |

## The ladder

| Tier (ordered) | Bundle (action classes) |
|---|---|
| `revoked` | ∅ — every act escalates |
| `floor` | dispose-advisory-finding |
| `tier-2` | + dispose-blocking-finding · sign-gate-verdict · trigger-materialization |
| `tier-3` | + rule-acceptance |
| `tier-4` | + approve-merge |

## Change history

| Date | Move | Tier | Evidence |
|---|---|---|---|
| 2026-01-01 | none | floor | born |
| 2026-01-02 | promotion | tier-2 | operator grant |
