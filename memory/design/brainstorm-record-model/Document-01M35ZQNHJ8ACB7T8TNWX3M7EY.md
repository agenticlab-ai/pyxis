# Brainstorm-record model — technical design

| Field | Value |
|---|---|
| **Status** | Approved — operator sign-off 2026-07-09 (operator review session) *(merge is the freezing event)* |
| **Owner** | @mdarabi |
| **Approver(s)** | @mdarabi (operator) |
| **Date** | 2026-07-09 |
| **Relevant-when** | implementing or changing the feature-brainstorm skill, its record template, or its checklists; authoring or consuming a brainstorm record; wiring `prd-new` to a gated record; deciding where a stage-1 session's state lives, freezes, or is reopened; superseding the record schema |
| **Scope** | The brainstorm-record contract: home and lifecycle (per gate verdict), the `brainstorm-record/v1` schema and its anchors, the two checklists (structure, coverage), the citation grammar, and `prd-new`'s consumption rules. **Not**: the session guide's interview prose, the eval-harness internals, the stage-3 sibling's record class, docgate graduation of this class. |
| **Tracking** | filed with the introducing PR |
| **Builds-on** | prd-amendment-model |
| **Supersedes** | — |
| **Serves** | prd:feature-brainstorm/FR-2 · prd:feature-brainstorm/FR-6 · prd:feature-brainstorm/FR-7 · prd:feature-brainstorm/FR-8 · prd:feature-brainstorm/FR-11 · prd:feature-brainstorm/FR-12 · prd:feature-brainstorm/FR-13 · prd:feature-brainstorm/FR-14 · prd:feature-brainstorm/NR-1 |
| **Security/privacy impact** | none — repo-local markdown artifacts; no identity, secret, network, or egress surface (artifact-integrity rules are bound as invariants, §3.5) |
| **Template version** | tech-design-template/v1 |

## 1. Problem

The feature-brainstorm PRD binds *what* a session must produce — operator-anchored decisions, typed fog, a gated verdict, a handoff `prd-new` can synthesize from without re-interviewing — but deliberately left the artifact itself undecided: where a record lives (its Q1), what exactly its sections and entries contain and how the checklists derive (Q2), what a `needs-investigation` verdict emits (Q3), and how a reopened idea relates to its old record (Q4). Implementation cannot start without these: the record is the skill's only durable output, the resume state (FR-14), the `prd-new` input (FR-13), and the provenance layer the traceability chain stands on (M2). Deciding the contract now — before the first real record exists — costs nothing; deciding it after even two records exist costs a migration. This is the same cheapest-moment argument the PRD-ledger and tech-design conventions each exploited at their own birth.

## 2. Decision

**Decision:** A brainstorm record is **one markdown file per session at `docs/brainstorms/<slug>.md`**, instantiated from a versioned template (`brainstorm-record/v1`) whose typed sections carry stable per-record anchors (`D-n`, `F-n`, `G-n`, `O-n`, `T-n`, `P-n`) cited downstream as `brainstorm:<slug>#D-n`. Its lifecycle is verdict-driven: a *ready-for-prd* record rides the 000-base's introducing PR; *needs-investigation* and *covered* records merge on their own PR; an *abandoned* record is discarded — and in every committed case **merge is the freezing event**: a frozen record is never edited, and a reopened idea starts a fresh record citing the frozen one. The record is a **provenance artifact, not a governed document kind**: freeze is enforced by prompt rules and PR review now, with docgate graduation staged behind schema stability.

The reasoning: the record must be operator-editable mid-session (the PRD fixed markdown for that reason), machine-consumable at handoff (AP-6), durable across context windows (FR-14), and immutable once anything downstream cites it (AP-5's chain is worthless if its upstream node can drift). One-file-per-session in a flat directory keeps parallel sessions collision-free (AP-23); verdict-driven commit paths keep the write perimeter exactly as narrow as the PRD demands (FR-15); and adopting the repo's uniform freeze rule — rather than inventing a record-specific one — means every author and reviewer already knows the lifecycle.

## 3. The design

### 3.1 Home, naming, and lifecycle

Records live flat at `docs/brainstorms/<slug>.md`. The slug is the candidate feature slug (lowercase kebab-case, permanent once committed); a reopened idea appends an ordinal — `<slug>-2`, `-3` — never reusing a frozen record's file.

| Gate verdict | Commit path | Rationale |
|---|---|---|
| *(in-session — no verdict yet)* | working-tree file, uncommitted; the resume state (FR-14) | session state is not yet an outcome; nothing to review |
| **ready-for-prd** | rides the fed 000-base's introducing PR | already decided by the PRD (§5.4); reviewer sees record + PRD together (ABUSE-3 mitigation) |
| **needs-investigation** | committed on its own docs PR | the fog *is* the session's outcome — durable, reviewable, and the seed of future investigation |
| **covered** | committed on its own docs PR | institutional memory: the pointer + ruling feed FR-2's prior-refusal/overlap surface for every later session |
| **abandoned** | not committed; the file is discarded | no verdict content stands behind it; committing it would put half-decisions the operator disowned into the corpus |

**Decision:** merge freezes a record uniformly — the generalization of the PRD's "frozen when the PRD it feeds merges" to the two self-committed verdicts. Post-merge, a record changes by no mechanism at all: no amendments, no supersession machinery — a reopened idea is a **new record** whose header cites the frozen predecessor (resolves PRD Q4, leaning as recorded there). Orientation (FR-2) must surface cited prior records and their rulings, which is what makes "zero re-asked questions" hold across reopenings without unfreezing anything.

**Decision:** a `needs-investigation` verdict emits **nothing beyond the record** (resolves PRD Q3, proposed for ratification here). The gate turn may *suggest* investigations conversationally, but the skill writes no work items, no issues, no investigation files — stage 1 precedes the artifact and the write perimeter is binding (FR-15). Routing fog into actual investigation work is a human act until a pipeline stage owns it.

### 3.2 The `brainstorm-record/v1` schema

The template lands at `.pyxis/templates/brainstorm-record-template.md` carrying version `brainstorm-record/v1` (recorded in the record's header at instantiation, never edited after — the same template-version contract every document class here uses). Evolution is additive within v1; a breaking section/field change is `brainstorm-record/v2` by a superseding design.

**Header table (closed):**

| Field | Content |
|---|---|
| **Idea** | the raw idea, verbatim (or a pointer to the notes file handed in) |
| **Operator** | who answers — the decision authority of record |
| **Date(s)** | session date(s), including resumes |
| **Status** | `in-session` → `gated: <verdict>` |
| **Feeds** | the PRD ledger path once fed (`docs/prds/<feature>/`), else `—` |
| **Prior records** | frozen records consulted (`brainstorm:<slug>`), else `—` |
| **Template version** | `brainstorm-record/v1` |

**Sections and entry contracts** (every section present; an empty one reads `none`):

| Section | Anchor | Entry fields |
|---|---|---|
| `## Orientation` | — | sources read (path · why loaded) · overlaps surfaced (source · gist) |
| `## Framings` | — | 2–3 framings (name · thesis · implications), closing with a `**Destination:**` callout — the chosen/hybrid framing + operator rationale |
| `## Decisions` | `D-n` | question · answer (the operator's ruling, gisted in their terms) · reasoning · anchor (date · turn ref) · status (`live` \| `superseded by D-m — <reason>`) |
| `## Facts` | `F-n` | fact · source (path / citation) — looked up, never asked (FR-5) |
| `## Fog` | `G-n` | gap · type (`blocking` \| `non-blocking`) · blocks (destination / the decision-shaped input / the downstream phase) |
| `## Out of scope` | `O-n` | ruling · reason · from (`D-n`) |
| `## Candidate glossary terms` | `T-n` | term · definition · avoid[] |
| `## Parked technical constraints` | `P-n` | constraint · context — conveyed to stages 3–4, never resolved here |
| `## Gate` | — | verdict (`ready-for-prd` \| `needs-investigation` \| `covered` \| `abandoned`) · date · operator · covering source (required when `covered`) · coverage-check result (required when `ready-for-prd`) |

Anchors are per-record, dense from 1, never renumbered or reused; within-record supersession keeps both entries (the PRD's UB-2 behavior, made structural).

### 3.3 Citation grammar — the provenance edge

**Decision:** downstream documents cite record entries with a typed reference, `brainstorm:<slug>#<anchor>` — the same shape as the design class's `prd:`/`design:`/`doc:` families:

- the 000-base's **§7.3 decision log** cites its originating ruling per row: `(brainstorm:<slug>#D-3)` — this is exactly what PRD metric M2 counts;
- the 000-base's **§7.2 open questions** cite carried-over fog: `(brainstorm:<slug>#G-2)`;
- evidence rows may cite session-established facts: `(brainstorm:<slug>#F-1)`.

The grammar is defined **once**, in the record template, and referenced by the `prd-new` guide — never restated there (AP-11). Resolution is by eye and grep until graduation (Q1); the index/compiler machinery does not learn this reference type in v1.

### 3.4 The two checklists

Both live **in the record template file** as appendices — co-located with the schema they check, one artifact carrying the whole contract. Guides reference them; nothing restates them.

- **Structure checklist** (implements FR-8's fit criterion; walked at the gate and by review): every section present or explicitly `none`; every `D-n` complete (all five fields — an unanchored decision is invalid, DINV-1); every `G-n` typed; exactly one gate entry, verdict-conditional fields present (DINV-4); supersessions preserve both entries (DINV-5).
- **Coverage checklist** (implements FR-13's fit criterion; walked before a `ready-for-prd` verdict may be recorded): a mapping table from the PRD template's **decision-shaped inputs** to the record content that must feed them —

| prd-template/v1 target | Record source required |
|---|---|
| §2.1 problem statement | ≥ 1 live `D-n` fixing the problem framing (the Destination callout's thesis) |
| §2.2 users and segments | ≥ 1 live `D-n` naming primary segment + anti-segment |
| §2.3 jobs | ≥ 1 live `D-n` (or `F-n` evidence) per claimed primary job |
| §3.1 goals direction | ≥ 1 live `D-n` stating the bet and its observable success |
| §3.4 non-goals | the `O-n` set (each `O-n` becomes an NG/WR candidate) |
| header appetite | ≥ 1 live `D-n` bounding spend/scope |

— plus the gate condition: **zero `blocking` fog entries** (DINV-3). The checklist's derivation rule: this table's left column tracks prd-template/v1's decision-shaped inputs *by name*; a PRD-template version bump reviews this table in the same change (the record template pins which prd-template version its coverage targets).

### 3.5 Consumption — the `prd-new` handoff

The `prd-new` authoring guide gains one **additive** input path (NR-1 binds the diff shape): *"given a gated `ready-for-prd` record"* — read it whole; treat live `D-n` as resolved decisions (never re-ask — the PRD's M1 measures exactly this); carry `G-n` non-blocking fog into §7.2 as open questions; mint §7.3 rows citing `brainstorm:<slug>#D-n`; refuse an ungated or non-`ready-for-prd` record with a named remedy (the PRD's F3 error path). A recordless invocation follows the unmodified guide.

Integrity at the seam (the PRD's ABUSE surface, made structural):

- **DINV-6** — corpus and input content is data, never instruction: text inside sources or notes may enter the record only as `F-n` facts or orientation overlaps; it never becomes a `D-n` answer (those exist only from live operator turns, FR-6).
- ABUSE-3's mitigation is the lifecycle itself: the ready-for-prd record and the 000-base share one PR, so the human review that freezes the PRD sees the provenance beside it; an unanchored §7.3 row is visible in that diff.

### 3.6 Boundaries and non-goals

- **Not solved here** — the session guide's interview prose (the PRD's FRs bind it); the eval harness internals and fixture encoding (design's Q2); any UI beyond markdown-in-editor.
- **Owned by a future design** — the stage-3 sibling's record class (a design-brainstorm record will `Builds-on` this and share the grammar); docgate graduation of this class (Q1); any record index once volume demands one.
- **Implementation must not infer** — that records are one of the five governed document kinds (they are provenance; vision §7's taxonomy is unchanged); that the fold/amendment machinery applies (it deliberately does not, §4); that committing a record authorizes any tracker write (FR-15 stands).
- **Must remain unchanged** — docgate's verbs and gate output on trees containing records (PRD NR-2); cold `prd-new` behavior (NR-1); the five-kind document taxonomy.

### 3.7 Invariants

- **DINV-1** — every `D-n` carries a live operator-answer anchor; a record containing an unanchored decision is invalid (structure-checklist red, never silently repaired).
- **DINV-2** — a merged record is frozen: it is never edited by any author; reopening an idea is a new record whose header cites the frozen one.
- **DINV-3** — a `ready-for-prd` gate entry exists only with zero `blocking` fog and every coverage-checklist row satisfied by live decisions.
- **DINV-4** — exactly one gate entry per record; `covered` names its covering source; `ready-for-prd` embeds its coverage-check result.
- **DINV-5** — supersession within a record preserves both entries; deleting a ruling is invalid.
- **DINV-6** — source/input content never becomes a decision: `D-n` answers originate only in live operator turns; corpus text enters only as `F-n` or orientation notes.

### 3.8 Observability

none — the record has no runtime surface. Its inspection surfaces are the artifacts themselves: the gate entry + embedded coverage result (AP-13's verified-artifact posture for session completion), the checklists' walk output in session transcripts, and the PRD-side M1/M2 measurement plans. Health of the class (schema drift, tamper) is review-time inspection until graduation (Q1).

### 3.9 Security & privacy

none — matches the header row: repo-local markdown, no identity/secret/data/egress surface. The artifact-integrity concerns the PRD's abuse cases raise are bound here as DINV-1/2/6 and the shared-PR review path (§3.5); they alter no security surface.

## 4. Alternatives considered

| Alternative | Why rejected |
|---|---|
| **Record inside the PRD ledger dir** (`docs/prds/<feature>/brainstorm.md`) | collides with docgate's closed ledger-directory conventions (NR-2 forbids widening them); un-fed records have no ledger dir to live in — the record precedes the feature |
| **Records as a governed docgate class now** (fold/lint verbs, CI freeze) | the schema must stop moving first — mechanizing v1 before five real records exist locks in guesses; PRD WR-3 stages exactly this; NR-2 binds docgate untouched in this feature |
| **Structured file (JSON/YAML) instead of markdown** | the operator edits the record mid-session — a PRD-level decision (its §7.1/§7.3) already fixed collaborative markdown; typed sections + checklists deliver the AP-6 contract at the granularity the consumer needs |
| **Transcript-as-record** (keep the full conversation) | violates the PRD's compactness bound (NFR-2) and buries rulings in noise; gists + anchors are the contract, the transcript stays in session logs |
| **One shared `brainstorms.md` file** | parallel sessions would collide on one hot file — AP-23 rejects the shape outright |
| **Amendment/fold machinery for records** (reuse the PRD ledger model) | granularity mismatch, same reasoning the tech-design class recorded: a session record accretes during drafting and then freezes whole; nothing changes piecemeal post-merge, so there is nothing for a fold to project |
| **Auto-emit investigation work items on `needs-investigation`** | breaches the write perimeter (FR-15) and stage 1's position before the artifact; suggestion-in-conversation costs nothing and keeps routing human (Q3 resolution) |

## 5. Risks

| ID | Risk | Impact | Mitigation | Verification / evidence |
|---|---|---|---|---|
| R1 | freeze is prompt-side + review only (no CI gate for this class) | a frozen record could be edited unnoticed, corrupting cited provenance | records change only via PRs (human review, git history is the audit trail); graduation to a docgate lint staged behind schema stability (Q1) | review discipline now; Q1's trigger names the mechanization moment |
| R2 | record slug diverges from the eventual feature slug chosen at synthesis | broken-looking provenance joins | the `Feeds` row records the binding at synthesis; A-4's demo checks the 000-base cites this record's actual slug | A-4 evidence |
| R3 | schema churn before mechanization | template/checklist drift across early records | additive-only within `brainstorm-record/v1`; breaking change = v2 via superseding design; records pin their instantiated version | template diffs reviewed; version row inspected per record |
| R4 | `docs/brainstorms/` accumulates covered/investigation records with no index | navigation cost grows | flat dir + Status-in-header keeps it greppable; an index is deliberately deferred until volume demands one (the conventions' own R3 posture) | directory size watched at reviews |
| R5 | citation-grammar or coverage-mapping drift between the template and the guides | broken M2 anchors; stale coverage targets | both are defined once in the template (AP-11); guides reference, never restate; the coverage table pins the prd-template version it targets and reviews on its bump | A-3 inspection; R5 grep at review |

## 6. Acceptance

| ID | Acceptance criterion | Verify | Required evidence | Traces to |
|---|---|---|---|---|
| A-1 | `.pyxis/templates/brainstorm-record-template.md` exists carrying `brainstorm-record/v1`: the closed header, all nine sections with entry contracts, and both checklist appendices | Inspection | template file in the introducing implementation PR | prd:feature-brainstorm/FR-8 · §3.2 · §3.4 |
| A-2 | a golden record instantiated from the template passes both checklists, and each seeded defect class (unanchored `D-n`, untyped `G-n`, second gate entry, deleted superseded ruling) turns the structure checklist red | Test | golden + defect fixtures with checklist results | prd:feature-brainstorm/FR-8 · #DINV-1 · #DINV-4 · #DINV-5 |
| A-3 | the `prd-new` guide gains the gated-record input path by an additive diff, referencing (never restating) the template's citation grammar and coverage checklist | Inspection | guide diff | prd:feature-brainstorm/NR-1 · §3.5 |
| A-4 | a 000-base synthesized from the golden record carries `brainstorm:<slug>#D-n` citations on 100% of its §7.3 rows and `#G-n` citations on carried fog | Demo | the demo record + draft 000-base pair | prd:feature-brainstorm/FR-13 · §3.3 |
| A-5 | the feature-brainstorm guide implements the verdict→commit-path table, including abandoned-record discard and the needs-investigation no-emission rule | Inspection | guide text | prd:feature-brainstorm/FR-12 · §3.1 |
| A-6 | a reopen fixture: a fresh `<slug>-2` record cites the frozen predecessor, and orientation surfaces its rulings so none is re-asked | Demo | reopen fixture transcript + record pair | prd:feature-brainstorm/FR-2 · prd:feature-brainstorm/FR-14 · #DINV-2 |
| A-7 | this document passes `docgate lint-tech-designs` and ships with the regenerated index (both faces) | Test | gate output in the introducing PR | conventions §3.2 |

## 7. Open questions

| ID | Question | Owner | Blocks | Status | Resolution |
|---|---|---|---|---|---|
| Q1 | When does the record class graduate to docgate governance (a structure-lint verb + CI freeze)? Trigger proposed: checklists stable across the first five records (the PRD's WR-3) | operator | nothing | open | |
| Q2 | Eval fixture corpus home and the scripted-operator encoding for the harness | implementation | the PRD's eval-harness phase, not this design | open | |
| Q3 | Cross-machine resume of an in-session (uncommitted) record — WIP-commit convention or out of scope? | operator | nothing | open | leaning: out of scope until a real need; a session resumes on the machine that holds it |

## Principles

- **AP-5 — honored.** The citation grammar (`brainstorm:<slug>#D-n`) is the missing upstream edge of the traceability chain: raw idea → ruling → PRD row becomes greppable citation, not archaeology.
- **AP-6 — honored.** Closed header, typed sections, dense anchors, gists-not-transcripts: the record is contractual data at the granularity its consumers need, and compact by rule.
- **AP-7 — honored by extension.** The five governed kinds are untouched; records adopt the repo's uniform merge-is-freezing rule as a *sixth, ungoverned provenance class* — prompt/review-enforced now, with mechanization staged (Q1), exactly the transitional posture the amendment model itself ran under.
- **AP-10 — honored, staged.** The checklists are data in the template (copy-walk, not re-derivation); their codification into a docgate verb is deliberately deferred behind schema stability rather than skipped — the same born-with-the-tool discipline the design index followed.
- **AP-11 — honored.** One citation grammar and one coverage mapping, defined in the template, referenced everywhere else; the coverage table pins the prd-template version it targets.
- **AP-13 — honored.** Session completion is a verifiable artifact — the gate entry embedding its coverage-check result — never a claim in prose.
- **AP-21 — honored.** The record *is* the judge-once artifact: rulings made at the richest-context site, frozen as data, read verbatim downstream; `prd-new` never re-judges (M1 measures the breach).
- **AP-23 — honored.** One file per session, flat permanent slugs, append-favoring sections: parallel sessions and parallel authors never contend on a shared surface.
- **AP-24 — honored.** Records derive from operator rulings and govern nothing; no governing document is ever regenerated from them.
- **RW-5 — honored.** One independently-revisable decision — the record contract; the interview prose, harness, and sibling record classes are explicitly outside (§3.6).
- **Deviation:** none.
