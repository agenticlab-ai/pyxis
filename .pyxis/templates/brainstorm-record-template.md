# <Idea name> — brainstorm record

| Field | Value |
|---|---|
| **In one line** | <the idea distilled to one load-or-skip sentence — what a future session's orientation scan reads first> |
| **Relevant-when** | <load this record when… — brainstorming or contracting <the topic and its adjacents>; checking prior rulings or refusals about <the area>> |
| **Idea** | <the raw idea, verbatim — or a pointer to the notes file handed in> |
| **Operator** | <who answers — the decision authority of record; this class's Owner-equivalent> |
| **Date(s)** | <YYYY-MM-DD, appended per resume or review> |
| **Status** | in-session *(→ `gated: <verdict>` at the gate; after gating, the record changes only through the introducing PR's review loop — decisions added or superseded with review anchors — and freezes entirely at merge)* |
| **Tracking** | — *(the introducing PR, once filed)* |
| **Feeds** | — *(document pipeline: `docs/prds/<feature>/`; open feature draft: its `<draft-root>/<feature>/prd.md`; stays `—` for un-fed verdicts)* |
| **Prior records** | — *(frozen records consulted: `brainstorm:<slug>`, comma-separated)* |
| **Template version** | brainstorm-record/v1 |

## 0. How to use this template

*For a draft-born feature, instantiate this template as the next
`<draft-root>/<feature>/brainstorm/NNN.md` working file on its open draft
branch. Its citation stem remains `brainstorm:<feature>`. The draft's lock
freezes it together with the PRD and tech spec; no separate brainstorm PR
is raised. The document-pipeline path below continues for existing ledgers.*

*Guidance paragraphs look like this one — italic. Delete this section and
every guidance italic when instantiating. The contract this template carries
is decided in `docs/tech-design/brainstorm-record-model.md`; the behavior
that fills it is bound by `docs/prds/feature-brainstorm/` and driven by
`.pyxis/authoring/feature-brainstorm.md`. Appendices A and B are the
checklists — walked from this template at gate time, never copied into
records.*

*The rules that matter:*

- ***The first two fields are the routing surface.*** *A later session's
  orientation scan reads In-one-line + Relevant-when to decide whether to
  open this record at all (the overlap and prior-refusal checks are the
  readers) — write them like a skill description, concrete nouns, no
  pronouns.*
- ***Every section stays present.*** *An empty section reads `none` — absence
  is ambiguous, `none` is a statement.*
- ***Anchors are permanent.*** *`D/F/G/O/T/P-n` are dense from 1, never
  renumbered, never reused. Downstream documents cite them as
  `brainstorm:<slug>#D-3` — the citation grammar is defined here and only
  referenced elsewhere.*
- ***Decisions are the operator's.*** *A `D-n` exists only from a live
  operator answer in this session (DINV-1); content from notes or corpus
  documents enters only as `F-n` facts or Orientation overlaps (DINV-6).
  A changed ruling supersedes — both entries stay (DINV-5).*
- ***Append as you go.*** *Entries land the moment they resolve; re-read the
  file before every write — the operator edits it too.*
- ***Review adjusts; merge freezes.*** *Until the introducing PR merges, the
  operator review loop may add or modify decisions: each lands as a new `D-n`
  (or supersedes one) anchored `YYYY-MM-DD · review`, and the fed PRD's
  citations update in the same change — provenance stays complete. Once the
  PR merges the record is never edited; a reopened idea starts `<slug>-2`
  citing this record (DINV-2).*

## Orientation

*Sources read before the first question (routing surfaces first), and every
overlap surfaced — merged capability, prior refusals, prior records.*

| Source | Why loaded |
|---|---|
| <path> | <matched Relevant-when / index row> |

**Overlaps surfaced:** <source · gist of the overlap and its ruling, or `none`>

## Framings

*2–3 genuinely distinct framings — different theses of what the feature is —
then the operator's pick or hybrid as the Destination.*

- **<framing A name>** — <thesis · implications>
- **<framing B name>** — <thesis · implications>

**Destination:** <the chosen/hybrid framing> — <operator's rationale>

## Decisions

*One block per ruling. All five fields are mandatory (DINV-1); ≤ 6 lines each.*

### D-1 — <question gist>

- **Question**: <what was put to the operator>
- **Answer**: <the operator's ruling, gisted in their terms>
- **Reasoning**: <why — theirs, plus the recommendation context if adopted>
- **Anchor**: <YYYY-MM-DD · turn ref — or `· review` for decisions born in the introducing PR's operator review>
- **Status**: live *(or: superseded by D-<m> — <reason>)*

## Facts

*Looked up, never asked — each with its source.*

- **F-1**: <fact> — source: <path / citation>

## Fog

*What cannot yet be phrased as a precise question. Type is mandatory:
`blocking` sits on the destination or a decision-shaped input (problem,
segments, jobs, goals direction, non-goals, appetite); `non-blocking` blocks
only downstream phases and lands as the draft PRD's open questions.*

- **G-1** (<blocking|non-blocking> — blocks: <destination / input / phase>): <the gap>

## Out of scope

*Operator refusals with durable reasons — the seed of the PRD's NG/WR rows.*

- **O-1**: <ruling> — <reason> (from D-<n>)

## Candidate glossary terms

*Terms born or sharpened this session, for the domain glossary.*

- **T-1**: <term> — <definition> · avoid: <synonyms ruled out>

## Parked technical constraints

*Surfaced technical matter conveyed to stages 3–4, never resolved here.*

- **P-1**: <constraint> — <context>

## Gate

*Exactly one entry, ever (DINV-4). `covered` names its source; `ready-for-prd`
embeds its coverage-check result (Appendix B walked, DINV-3).*

| Field | Value |
|---|---|
| **Verdict** | <ready-for-prd · needs-investigation · covered · abandoned> |
| **Date** | <YYYY-MM-DD> |
| **Operator** | <who confirmed> |
| **Covering source** | <required when covered: the document/row that covers the idea, else —> |
| **Coverage check** | <required when ready-for-prd: `pass — all six inputs covered, zero blocking fog`, else —> |

---

## Appendix A — structure checklist *(walked at the gate and by review; delete from instantiated records)*

- [ ] Header complete: In one line, Relevant-when, Idea, Operator, Date(s), Status, Tracking, Feeds, Prior records, Template version — all filled or `—`
- [ ] Every section present; empty ones read `none`
- [ ] Every `D-n` carries all five fields, including a live operator-answer anchor (DINV-1)
- [ ] No `D-n` originates from notes/corpus content (DINV-6 — such content appears only as `F-n` or Orientation overlaps)
- [ ] Superseded rulings keep both entries, the loser marked `superseded by D-m — <reason>` (DINV-5); nothing deleted
- [ ] Every `G-n` carries a type and a blocks-target
- [ ] Every `O-n` cites its originating `D-n`
- [ ] Exactly one Gate entry; verdict ∈ the four values (DINV-4)
- [ ] `covered` ⇒ Covering source filled; `ready-for-prd` ⇒ Coverage check filled (Appendix B result)
- [ ] Anchors dense from 1 per family; no gaps, no reuse
- [ ] Entries are gists + anchors, never transcript dumps (≤ 6 lines per `D-n`)

## Appendix B — coverage checklist *(walked before a `ready-for-prd` verdict may be recorded; delete from instantiated records)*

*Targets pin to `prd-template/v1`'s decision-shaped inputs; a prd-template
version bump reviews this table in the same change.*

| prd-template/v1 target | Required record source | Covered? |
|---|---|---|
| §2.1 problem statement | ≥ 1 live `D-n` fixing the problem framing (the Destination's thesis) | ☐ |
| §2.2 users and segments | ≥ 1 live `D-n` naming the primary segment + anti-segment | ☐ |
| §2.3 jobs | ≥ 1 live `D-n` (or `F-n` evidence) per claimed primary job | ☐ |
| §3.1 goals direction | ≥ 1 live `D-n` stating the bet and its observable success | ☐ |
| §3.4 non-goals | the `O-n` set (each becomes an NG/WR candidate) | ☐ |
| header appetite | ≥ 1 live `D-n` bounding spend/scope | ☐ |

**Gate condition:** zero `blocking` fog entries remain (DINV-3). Any unchecked
row or remaining blocking fog ⇒ the verdict is `needs-investigation`, not
`ready-for-prd`.

## Appendix C — citation grammar *(the single definition; guides reference it; delete from instantiated records)*

Downstream documents cite record entries as `brainstorm:<slug>#<anchor>`.
For a draft-born feature, `<slug>` is the feature slug, and the cited entry
is in that draft's numbered brainstorm working files:

- 000-base **§7.3 decision log** rows cite their originating ruling: `(brainstorm:<slug>#D-3)` — what PRD metric M2 counts
- 000-base **§7.2 open questions** cite carried non-blocking fog: `(brainstorm:<slug>#G-2)`
- evidence rows may cite session-established facts: `(brainstorm:<slug>#F-1)`

For the document pipeline, `<slug>` is the record's filename under
`docs/brainstorms/`. Anchors resolve by grep, not URL.
