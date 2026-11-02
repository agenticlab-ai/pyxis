# Argus authority contract — agenticlab-ai/pyxis

| Field | Value |
|---|---|
| **Current tier** | tier-2 |
| **Template version** | authority-contract/v1 |

## The ladder

| Tier (ordered) | Bundle (action classes) |
|---|---|
| `revoked` | ∅ — every act escalates; the kill switch |
| `floor` | dispose-advisory-finding |
| `tier-2` | + dispose-blocking-finding · sign-gate-verdict · trigger-materialization |
| `tier-3` | + rule-acceptance |
| `tier-4` | + approve-merge *(activates only after the operator changes org governance)* |

## Change history

| Date | Move | Tier | Evidence |
|---|---|---|---|
| 2026-07-11 | none | floor | born at floor — the introducing PR (the argus home); prd:argus §6.5 stage 1 |
| 2026-07-11 | promotion | tier-2 | **operator-directed early promotion** (the operator is the grantor) on the first-drive dogfood evidence: `docs/problems/argus-ledger-lint/` — a clean floor-held drive (11 attributed journal entries, 9 recorded assumptions each legality-tested, zero missed-breaking, the gate correctly *requested* not self-signed). Ahead of FR-14's ten-problem checkpoint by explicit operator grant, not the ship rule. Grants `dispose-blocking-finding` · `sign-gate-verdict` · `trigger-materialization`; `rule-acceptance` (tier-3) and `approve-merge` (tier-4) remain withheld — the human keeps acceptance and the merge backstop (NG-1), and every breaking decision still escalates (D-3 never-list). Demotion stays instant on any confirmed missed-breaking (§3.3). |
