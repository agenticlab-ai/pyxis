# <Problem title> — delivery ledger

| Field | Value |
|---|---|
| **Slug** | <the problem's slug — matches the sibling problem ledger> |
| **Template version** | delivery-ledger/v1 |

*Guidance paragraphs look like this — italic. Delete every guidance italic when instantiating. The contract is `docs/tech-design/argus-artifacts.md` §3.2; the behavior that writes it is `docs/tech-design/argus-agent-v1.md`. The header holds the slug only — status is derived (DINV-6). The journal is **append-only**; it closes with exactly one `delivered` entry (recorded only after mainline-content verification) and the file freezes; post-close events reach it only by reverse citation (`overturns delivery:<slug>#E-n` in the overturning artifact).*

## Derived state

*Regenerated on every append — never cited, never authoritative; on any conflict the journal wins (DINV-6).*

| Field | Value |
|---|---|
| **Status** | open · parked · delivered |
| **Stage position** | <the pipeline-map stage the drive is in> |
| **Open items** | <requests awaiting the human, unclaimed materialized items, in-flight proposals — each with its age> |
| **Park state** | <`—`, or `parked (brief E-n, since <date>)`> |
| **Cumulative spend** | <best-effort sum from entry gists; `unavailable` where the CLI exposes nothing> |

## Journal

*Append-only `E-n` entries from the closed type vocabulary: `stage-entered` · `ruling-recorded` · `assumption-recorded` · `item-materialized` · `brief-emitted` · `answer-received` · `proposal-disposed` · `reconciliation` · `bump-issued` · `delivered`. Every entry carries type, date, principal identity + authority-contract version, a gist body, and — wherever the act touched anything external — the external-effect identity (tracker item id, PR number, commit) for verify-before-redo. Type-specific fields: `brief-emitted` carries `class: override | deadlock`; `answer-received` cites its `A-n`; `delivered` carries the touch count, breaking count, and spend fields (adapter-honest; `unavailable` legal). Downstream documents cite entries as `delivery:<slug>#E-n`; an assumption's load-bearing spot cites the `assumption-recorded` entry that recorded it.*

### E-1 — <type> — <gist>

- **Date**: <YYYY-MM-DD>
- **Principal**: <Argus | the human> · **Authority-contract version**: <version at the act>
- **External effect**: <identity, or `—`>
- **Body**: <the gist; citations, never duplicated content>

---

## Appendix — structure checklist *(walked at instantiation, close, and review; delete from instantiated ledgers)*

- [ ] Header holds the slug only — no status cell
- [ ] Derived state present with all five fields; regenerated at the last append
- [ ] `E-n` dense from 1; no gaps, no reuse, no edits to existing entries
- [ ] Every entry carries type (from the closed vocabulary), date, principal + authority-contract version
- [ ] Every externally-effective entry carries its effect identity
- [ ] `brief-emitted` entries carry `class`; `answer-received` entries cite their `A-n`
- [ ] At close: exactly one `delivered` entry, with touch count, breaking count, spend fields, and its mainline-content verification evidence
- [ ] After close: no appends — post-close events arrive only as reverse citations elsewhere
