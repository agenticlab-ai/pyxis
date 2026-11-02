# DINV defect fixtures — red at exactly one check

The artifacts design's A-7: one seeded artifact per data-contract
invariant, each failing at exactly its check and no other. Described here
rather than instantiated as live files, so the corpus itself stays
gate-clean; the runner constructs each in scratch and confirms the single
expected failure.

**The 2026-07-11 red-first discriminator walk (recorded in
`../eval-report.md`) split these by *which kind of check* catches them** —
a per-file structure walk cannot see an edit (needs history) or a
cross-document citation (needs a repo scan). The honest classification:

## Structure-walk fixtures — single-row, static (6)

Caught by the class's own structure-checklist appendix on the file alone:

| Fixture | Class | Seeded defect | Red at |
|---|---|---|---|
| status-in-header | problem-ledger/v1 | a `Status` cell added to the header | "no status or other mutable cell" |
| unattributed-entry | delivery-ledger/v1 | an `E-3` missing principal + version | "principal + authority-contract version" |
| second-delivered | delivery-ledger/v1 | two `delivered` entries | "exactly one `delivered` entry" *(also trips "no post-close append" — a second `delivered` **is** a post-close append; the pair is one defect, two honest guards)* |
| out-of-ladder-tier | authority-contract/v1 | `current-tier: tier-9` **plus** a history row naming `tier-9` (so only the ladder-membership row fires, not the tier-consistency row) | "Current tier names a declared tier" |
| bare-pointer-flip | authority-contract/v1 | `current-tier: tier-2`, latest history row still `floor`, no new row | "latest history row's tier equals Current tier" |
| never-token-in-bundle | authority-contract/v1 | a bundle lists a never-class token (e.g. `approve-own-grant`) | "every bundle token is a defined action-class token" *(renamed from `never-grant`: DINV-5's guarantee is **structural** — never-acts are simply absent from the vocabulary, so an attempt fails the defined-tokens row; there is no separate "no never-grant" scan, and none is needed)* |

## History / cross-document fixtures — for the Q-1 lint (2)

**Not** per-file structure rows — a single snapshot cannot detect them.
Recorded here so the future `docs/tech-design/argus-artifacts.md` Q-1 lint
gives each its proper check:

| Fixture | Class | Seeded defect | Caught by |
|---|---|---|---|
| edited-A-entry | problem-ledger/v1 (DINV-1) | an existing `A-2` body rewritten | **history** — compare against the prior committed version (the append-only / `check-immutable` arm); invisible in one snapshot |
| cited-derived-block | delivery-ledger/v1 (DINV-6) | a downstream doc cites the regenerable derived block | **cross-document scan** — the "never cited" property lives across files, not in the ledger's own checklist |

## Runner protocol

- **Structure-walk fixtures**: construct each in scratch, walk its class's
  checklist appendix — each must fail at exactly its named row and no
  other. That single-row discrimination is the test (a fixture red at two
  unrelated rows means the checklist or the seed is wrong — the
  `out-of-ladder-tier` seed carries its matching history row for exactly
  this reason).
- **History / cross-doc fixtures**: deferred to the Q-1 lint; until it
  exists, DINV-1's no-edit and DINV-6's never-cited are guarded by the
  mutability rule and review, not by a mechanical walk — stated so no
  implementer assumes the structure checklist covers them.
