# <Problem title> — problem ledger

| Field | Value |
|---|---|
| **Slug** | <kebab-case, minted at intake from the idea; a collision appends `-2`, `-3`> |
| **Principal** | <the human problem provider — who handed the idea over> |
| **Authority-contract version** | <the authority contract's version current at intake> |
| **Envelope override** | <per-problem grant override when provided, else `—` (the repo default governs)> |
| **Template version** | problem-ledger/v1 |

*Guidance paragraphs look like this — italic. Delete every guidance italic when instantiating. The contract this template carries is `docs/tech-design/argus-artifacts.md` §3.1; the behavior that writes it is bound by `docs/tech-design/argus-agent-v1.md`. The header holds intake-time facts only — it has **no mutable cell**: status lives on the delivery ledger's derived block. This file is **append-only forever** (DINV-1 of the artifacts design): no `A-n` is ever edited, renumbered, or removed.*

## Idea

*The human's words, verbatim — never edited, never paraphrased. This is the root every downstream ruling traces to.*

<the idea, exactly as handed over>

## Authority entries

*One dense, append-only `A-n` family: pre-rulings recorded at intake, then every breaking-call answer as it arrives. Each entry is dated, attributed to its human author, and carries the authority-contract version current at recording; an answer cites the brief it answers (`delivery:<slug>#E-k`). An `A-n` binds as a ruling-in-advance for everything downstream; downstream documents cite it as `problem:<slug>#A-n`.*

### A-1 — <gist>

- **Date**: <YYYY-MM-DD>
- **Author**: <the human>
- **Authority-contract version**: <version at recording>
- **Answers**: <`delivery:<slug>#E-k` for a brief answer; `—` for an intake pre-ruling>
- **Ruling**: <the human's words, gisted faithfully — or verbatim where load-bearing>

---

## Appendix — structure checklist *(walked at instantiation and by review; delete from instantiated ledgers)*

- [ ] Header complete: Slug, Principal, Authority-contract version, Envelope override, Template version — all filled or `—`
- [ ] The header has no status or other mutable cell
- [ ] Idea present, verbatim, unedited
- [ ] `A-n` dense from 1; no gaps, no reuse, no edits to existing entries
- [ ] Every `A-n` carries date, author, authority-contract version, and its Answers field (`—` or a `delivery:` citation)
- [ ] Every answer-class `A-n` cites the `brief-emitted` entry it answers
