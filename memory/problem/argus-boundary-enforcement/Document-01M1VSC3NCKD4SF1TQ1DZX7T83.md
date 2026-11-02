# Argus boundary enforcement — problem ledger

| Field | Value |
|---|---|
| **Slug** | argus-boundary-enforcement |
| **Principal** | @mdarabi (operator) — handed the idea over |
| **Authority-contract version** | authority-contract/v1 (tier: tier-2) |
| **Envelope override** | — (the repo default governs) |
| **Template version** | problem-ledger/v1 |

## Idea

Harden Argus's boundaries mechanically. Right now Argus's behavioral boundaries — the never-list, the grant checks, staying in scope, never writing its own authority contract — are prompt-deep: enforced only by the drive guide's instructions, backstopped downstream by the human merge and the docgate document gates. That violates AP-10 (codify over prompt). Build the mechanical enforcement so those boundaries become impossible to violate, not merely prompt-reliable, in three parts: (1) fail-closed write hooks on the agent surface that DENY a write to a protected path — the authority contract, any frozen/merged document, another problem's ledger, or a path outside the drive's declared scope — at the moment of the write; (2) WR-1, the deferred authority-contract structure arm: a docgate check enforcing the authority-contract invariants (DINV-3 attribution present, DINV-4 tier-in-ladder + no bare pointer flip, DINV-5 no grant of a never-class act) that the shipped argus-ledger-lint explicitly deferred; (3) an attribution gate: a CI check that fails a PR whose Argus-authored diff touches the authority contract, a governance doc, or out-of-declared-scope files.

## Authority entries

*No pre-ruling or envelope override was recorded at intake; the idea above is the sole intake record. The `A-n` family opened on the first breaking-call answer below.*

### A-1 — first-PR out-of-scope derivation → a slug-derived fixed-path grammar, never same-PR self-attribution

- **Date**: 2026-07-12
- **Author**: @mdarabi (operator)
- **Authority-contract version**: authority-contract/v1 (tier: tier-2)
- **Answers**: the `/tech-design-review` escalation of the attribution-gate design (review:argus-attribution-gate#FND-3), which reached the two-round disposition cap; no `brief-emitted` entry preceded it (the escalation was raised by the reviewer, not parked by a drive brief), so this entry answers the review escalation directly rather than a `delivery:` brief citation.
- **Ruling**: The attribution gate's **first-PR out-of-scope scope-derivation** must be a **fixed function of the drive's slug** — a slug-derived fixed-path grammar — **never** the paths a same-PR, author-controlled delivery-ledger entry self-attributes to the drive. Naming a broader slug may yield only more doc-subtree, never an arbitrary code path. This overrules the escalated §3.2.1(b) fallback (which admitted "the paths the current PR's own delivery-ledger entries attribute to the drive" — an author-controlled same-PR input over arbitrary paths, the live widen vector FND-3 named). The first-PR positive scope is the slug-derived fixed set (the problem's own `docs/problems/<slug>/**` ledgers, the stage-1 brainstorm record, the PRD ledger `docs/prds/<slug>/**`, its review report) ∪ a small fixed named shared-target allowlist the drive legitimately touches at that stage (e.g. the glossary, the design index if regenerated); every element is a fixed function of the slug or a fixed named allowlist entry, and nothing is derived from same-PR self-attribution. Any target the gate cannot place in that fixed set is out-of-scope (refuse). The direction is repo-internal and additive (a pure function of the slug + a static allowlist — no new grant or outward surface).
