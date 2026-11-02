# Argus ledger-lint — acceptance corpus manifest (plan T4 · design §3.10)

The per-invariant acceptance corpus for the two Argus ledger-lint arms — the
**history arm** (`check-ledger-history`, base-requiring) and the
**cross-document arm** (`check-ledger-citations`, base-free). Each fixture below
declares its **expected arm** and its **expected verdict** so the corpus is a
red-at-exactly-its-arm partition, green on every legal case.

The fixtures are constructed **inline in Go** (temp git repos), not as tracked
`.md` files — deliberately, so the corpus's own `delivery:<slug>#…` strings and
mutated ledgers never enter the live `git ls-files` scan and false-red the
repo's own gate (design §3.2 own-corpus guardrail; the pattern 190/191 set).
Every `delivery:` string in *this* document is shown in a code span for the same
reason. This file is the readable projection of the executable declaration in
`ledger_composition_test.go` (`TestLedgerCorpusRedAtExactlyItsArm`).

## Per-invariant defect fixtures — red at exactly one arm

| Fixture | Corruption | Expected arm | Expected verdict (`Op`) | Traces |
|---|---|---|---|---|
| edited merged `E-1` | a merged delivery-ledger entry's body is rewritten in the working tree | history arm | `history` — edit defect naming the ledger + `#E-1` | FR-1 · DINV-1 |
| removed merged `A-2` | a merged problem-ledger entry is deleted (or renumbered away) | history arm | `history` — removal defect naming the ledger + `#A-2` | FR-1 · DINV-1 |
| cited derived block | a tracked document cites `` `delivery:<slug>#<derived-field>` `` (a derived-state field) in bare prose | cross-document arm | `derived-citation` — DINV-6 defect naming the citing doc + target | FR-3 · DINV-6 |
| dangling in-scope anchor | a recognized citation `` `delivery:<slug>#E-999` `` resolves to neither a journal `E-n` nor a derived field | cross-document arm | `unresolvable-anchor` — distinct from the derived finding (DINV-5) | UB-2 · DINV-5 |
| malformed cited ledger | a resolvable target ledger cannot be delimited | cross-document arm | named parse defect, never a false pass | UB-3 |
| malformed working-tree ledger | an in-scope ledger's region cannot be delimited | history arm | named parse defect, never a false green | UB-3 |

## Legal (green) fixtures — silent at both arms

| Fixture | Change | Expected verdict | Traces |
|---|---|---|---|
| legal append `E-3` | a new, strictly-higher-numbered entry, with the derived-state region also changed | **green** — zero history-arm defects (the append is invisible; the derived region is never in the anchor map) | FR-2 · DINV-2 |
| legal journal citation | a bare-prose `` `delivery:<slug>#E-2` `` into a real journal entry | **green** — legal (FR-4) | FR-4 |
| code-span / fenced citation | a **resolvable**-slug reference shown inside a `` ` `` span or a fenced block | **green** — dropped at recognition stage 1 despite resolving | FR-9 · DINV-3 |
| metavariable / absent-ledger | `` `delivery:<slug>#E-n` `` or `` `delivery:no-such-slug#E-1` `` | **green** — not a citation (recognition stage 2) | FR-9 · DINV-3 |
| own citation-bearing corpus | this repo's own bare-prose `delivery:` citations (the PRD, design, record, reviews) all resolve to real journal `E-n` entries | **green** — verified by the live `docgate gate` run | FR-9 · A-6 |

## Composition (cross-arm) properties

| Property | Proof | Traces |
|---|---|---|
| every defect red at exactly its arm; legal cases green | `TestLedgerCorpusRedAtExactlyItsArm` — one corpus carrying all classes at once | A-2 |
| non-ledger-tree verdict byte-identical with/without the two arms | `TestLedgerArmsAdditiveOnNonLedgerTree` | A-5 · NR-1 |
| `check-immutable`'s governed set unchanged; zero of its defects on a legal append | `TestLedgerImmutableUntouchedByAppend` (governed set = `docs/prds` + `docs/tech-design`, disjoint from `docs/problems`) | A-5 · NR-2 |
| umbrella enumeration/dispatch/help carry no hand-edited line; both arms listed from descriptors | `TestUmbrellaNoHandEditedArmLine`, `TestLedgerArmsRegisteredFromDescriptors` | A-8 · FR-6 · NR-3 |
| both arms under the identical umbrella verb on desk + CI, differing only in merge-base source; history arm skipped base-free | `TestLedgerDeskCIUmbrellaParity`, `TestLedgerHistorySkippedBaseFreeUnderUmbrella` | A-7 · FR-8 · UB-1 |
