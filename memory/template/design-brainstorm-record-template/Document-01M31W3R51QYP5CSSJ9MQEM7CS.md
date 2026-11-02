# <Design effort name> — design-brainstorm record

| Field | Value |
|---|---|
| **In one line** | <the design effort distilled to one load-or-skip sentence — what a future orientation scan reads first> |
| **Relevant-when** | <load this record when… — designing or changing <the mechanism and its adjacents>; checking prior design rulings or refused shapes about <the area>> |
| **Basis** | <the admitted requirement basis, typed: `prd:<feature>` (merged ledger) or `corpus:<IDs>`> |
| **Authority** | <the stage-3 decision authority of record — this class's Owner-equivalent> |
| **Date(s)** | <YYYY-MM-DD, appended per resume or review> |
| **Status** | in-session *(→ `gated: <verdict>` at the gate; after gating, the record changes only through the introducing PR's review loop — rulings added or superseded with review anchors — and freezes entirely at merge)* |
| **Tracking** | — *(the introducing PR, once filed)* |
| **Feeds** | — *(set to `docs/tech-design/<slug>.md` at authoring; stays `—` for un-fed verdicts)* |
| **Prior records** | — *(frozen records consulted, either class: `brainstorm:<slug>` · `design-brainstorm:<slug>`)* |
| **Template version** | design-brainstorm-record/v1 |

## 0. How to use this template

*Guidance paragraphs look like this one — italic. Delete this section and
every guidance italic when instantiating. The contract this template carries
is decided in `docs/tech-design/design-brainstorm-record-model.md`; the
behavior that fills it is bound by `docs/prds/tech-design-brainstorm/`.
Appendices A–C are walked from this template, never copied into records.*

*The rules that matter:*

- ***The filename carries the class.*** *Records live in `docs/brainstorms/`
  as `<slug>-tech-design.md` (reopen: `<slug>-tech-design-2.md`) — the
  suffix keeps the shared home collision-free and the class visible in
  every listing.*
- ***The first two fields are the routing surface.*** *A later session's
  scan reads In-one-line + Relevant-when to decide whether to open this
  record — write them like a skill description.*
- ***Every section stays present.*** *Empty reads `none`.*
- ***Anchors are permanent.*** *`DQ/D/ALT/S/F/G/O/T/P-n` are dense from 1,
  never renumbered or reused; cited downstream as
  `design-brainstorm:<filename-stem>#D-3`.*
- ***Rulings are the authority's.*** *A `D-n` exists only from a live
  authority answer in the session (DINV-1); basis, corpus, and prior-record
  content enters only as `F-n` or Orientation notes (DINV-6). A changed
  ruling supersedes — both entries stay (DINV-5).*
- ***Every design question gets a panel-log line*** *— fired, skipped,
  forced, or waived, with the reason (DINV-7). A fired panel is backed by
  ≥ 2 scored `ALT-n`.*
- ***Requirement gaps are parked or bounced, never patched.*** *A discovered
  hole lands as a `P-n` with its basis anchors; a blocking one gates the
  session `needs-more-product` naming the `/prd-amend` route (DINV-8).*
- ***Append as you go; review adjusts; merge freezes.*** *Re-read before
  every write; until the introducing PR merges the operator review loop may add
  or supersede rulings (anchored `YYYY-MM-DD · review`); after merge the
  record is never edited — a reopened effort starts `<slug>-tech-design-2.md`
  citing this one (DINV-2).*

## Orientation

*Sources read before the first question (routing surfaces first), and every
overlap surfaced — live designs (from the index), prior records of either class.*

| Source | Why loaded |
|---|---|
| <path> | <matched Relevant-when / index Serves row> |

**Overlaps surfaced:** <live design or prior record · gist, or `none`>

## Design questions

*The open solution-space questions in scope, phrased precisely — the panel
log keys off these.*

- **DQ-1**: <the question>

## Panel log

*One line per `DQ-n`, whatever happened (DINV-7).*

- **DQ-1**: <opens/replaces | extends> — <fired | skipped | forced | waived> — <why>

## Decisions

*One block per ruling; all five fields mandatory (DINV-1); ≤ 6 lines each.*

### D-1 — <question gist>

- **Question**: <what was put to the authority>
- **Answer**: <the authority's ruling, gisted in their terms>
- **Reasoning**: <why — theirs, plus the recommendation context if adopted>
- **Anchor**: <YYYY-MM-DD · turn ref — or `· review`>
- **Status**: live *(or: superseded by D-<m> — <reason>)*

## Alternatives

*Every candidate shape — panel-born or hand-authored. Scores are named, never
vibes: principles by ID, incidents matched or `none checked <where>`.*

### ALT-1 — <name>

- **Shape**: <≤ 4 lines — what it is>
- **Trade-offs**: <where it wins, where it costs>
- **Principles**: <AP/RW honored / strained, by ID, one clause each>
- **Incidents**: <matched `I-n` and how, or: none checked <registry/date>>
- **Verdict**: <chosen — why | rejected — why | hybridized into ALT-<m>>
- **From**: <DQ-n · panel | hand-authored>

## Seam map

*The seams the direction touches — feeds the design's Scope and Serves.*

- **S-1**: <seam> — <opens | replaces | extends> — behind it: <what> — feeds: <the design's Scope/Serves line>

## Facts

- **F-1**: <fact> — source: <path / citation>

## Fog

*Typed by what it blocks: `blocking` sits on the chosen direction or a
design-template decision-shaped input (the §2 Decision, seam placement, the
Serves set); `non-blocking` rides the authored design as its own `Q-n`.*

- **G-1** (<blocking|non-blocking> — blocks: <direction / input / design Q-n>): <the gap>

## Out of scope

- **O-1**: <ruling> — <reason> (from D-<n>)

## Candidate glossary terms

- **T-1**: <term> — <definition> · avoid: <synonyms ruled out>

## Parked product gaps

*Requirement defects discovered mid-design — anchored, dispositioned, never
silently designed around (DINV-8).*

- **P-1**: <the gap> — anchors: <prd:<feature>/<ID>> — disposition: <bounced — see Gate | proceed-anyway per D-<n>>

## Gate

*Exactly one entry, ever (DINV-4). Verdict-conditional fields are mandatory.*

| Field | Value |
|---|---|
| **Verdict** | <ready-for-design · needs-more-product · needs-investigation · covered · abandoned> |
| **Date** | <YYYY-MM-DD> |
| **Authority** | <who ruled> |
| **Covering design** | <required when covered: the live design that decides it, else —> |
| **Coverage check** | <required when ready-for-design: `pass — core four covered, zero blocking fog`, else —> |
| **Amend route** | <required when needs-more-product: the `/prd-amend` target + the gap's `P-n` anchors, else —> |

---

## Appendix A — structure checklist *(walked at the gate and by review; delete from instantiated records)*

- [ ] Filename carries the class suffix: `<slug>-tech-design.md`
- [ ] Header complete: In one line, Relevant-when, Basis, Authority, Date(s), Status, Tracking, Feeds, Prior records, Template version — all filled or `—`
- [ ] Every section present; empty ones read `none`
- [ ] Every `DQ-n` has exactly one Panel-log line, whatever the action (DINV-7)
- [ ] A `fired` (or `forced`) panel line is backed by ≥ 2 `ALT-n` from that `DQ-n`, each with Principles and Incidents fields filled (an empty incidents check names *where checked*)
- [ ] Every `ALT-n` carries a Verdict
- [ ] Every `D-n` carries all five fields, including a live authority anchor (DINV-1)
- [ ] No `D-n` originates from basis/corpus/prior-record content (DINV-6)
- [ ] Superseded rulings keep both entries (DINV-5); nothing deleted
- [ ] Every `G-n` carries a type and a blocks-target
- [ ] Every `O-n` cites its `D-n`; every `P-n` carries basis anchors and a disposition (DINV-8)
- [ ] Exactly one Gate entry; verdict ∈ the five; verdict-conditional fields filled (DINV-4)
- [ ] Anchors dense from 1 per family; no gaps, no reuse
- [ ] Entries are gists + anchors, never transcript dumps (≤ 6 lines per `D-n`, ≤ 10 per `ALT-n`)

## Appendix B — coverage checklist *(walked before a `ready-for-design` verdict may be recorded; delete from instantiated records)*

*Targets pin to `tech-design-template/v1` — deliberately the narrow core (the
PRD's R6); a tech-design-template bump reviews this table in the same change.*

| tech-design-template/v1 target | Required record source | Covered? |
|---|---|---|
| §2 Decision — the direction | ≥ 1 live `D-n` choosing the shape, its `DQ-n` panel-logged | ☐ |
| §4 Alternatives considered | the `ALT-n` set — ≥ 2 entries where the panel fired, every entry verdicted | ☐ |
| header `Serves` + `Scope` | the `S-n` seam map + the `Basis` anchors, enumerated | ☐ |
| §3.2 Boundaries and non-goals | the `O-n` set | ☐ |

**Gate condition:** zero `blocking` fog remains (DINV-3). Any unchecked row ⇒
`needs-investigation` (or `needs-more-product` where the blocker is a `P-n`
gap), never `ready-for-design`.

## Appendix C — citation grammar *(the single definition; guides reference it; delete from instantiated records)*

Downstream documents cite record entries as
`design-brainstorm:<filename-stem>#<anchor>` (stem includes the suffix:
`foo-tech-design`):

- the authored design's **§2 Decision** cites its direction: `(design-brainstorm:<stem>#D-1)` — what the PRD's M2 counts
- every **§4 Alternatives row** cites its entry: `(design-brainstorm:<stem>#ALT-2)`
- **Scope/Serves** justifications may cite the seam map: `(design-brainstorm:<stem>#S-1)`
- a `needs-more-product` follow-up amendment cites the ruling in `Learned-from`: `operator-decision:design-brainstorm:<stem>#D-n`

Anchors resolve by grep, not URL.
