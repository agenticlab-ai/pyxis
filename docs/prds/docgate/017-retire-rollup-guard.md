# Amendment 017 — retire the rollup guard: rollup updates return to convention

| Field | Value |
|---|---|
| **Amends** | docgate (chain: 000 → 016) |
| **Status** | Approved |
| **Date** | 2026-07-10 |
| **Owner** | @mdarabi |
| **Learned-from** | review-thread:pr-121-techspec-conflicts · operator-decision:2026-07-10-retire-rollup-guard |
| **Blast radius** | tier: contract-changing; touches: FR-10; reason: retires a Must row — capability diffs stop being forced to touch a rollup |
| **Template version** | amendment-template/v2 |

## Operations

### op: retire FR-10

Reason: FR-10 made every capability PR touch a live rollup, and the cheapest
compliant edit — rewriting the single `Last verified` line of
`docs/techspec.md` — became the standing attest, so any two in-flight PRs
collided on that line (12 of the 20 most recently merged PRs touched the
file; the conflict recurred through PR #121, operator-observed 2026-07-10).
The freshness the guard bought does not repay a built-in textual conflict
between every pair of concurrent PRs. Rollup updates return to AGENTS.md
rule 3 as convention — a PR that changes shipped structure updates the
affected rollup sections — and freshness reads from git history; the
`Last verified` attest row leaves with the guard's enforcement half.
If drift reappears, the revisit path is per-PR fragment files folded on
main — the ledger's own write model — not a restored shared-line attest.
