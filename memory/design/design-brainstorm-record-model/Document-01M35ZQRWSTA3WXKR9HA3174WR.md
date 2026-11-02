# Design-brainstorm-record model — technical design

| Field | Value |
|---|---|
| **Status** | Approved — operator sign-off 2026-07-10 (operator review session; naming ruling applied in-review) *(merge is the freezing event)* |
| **Owner** | @mdarabi |
| **Approver(s)** | @mdarabi (operator) |
| **Date** | 2026-07-10 |
| **Relevant-when** | implementing or changing the tech-design-brainstorm skill, its record template, or its checklists; authoring or consuming a design-brainstorm record; wiring `tech-design-new` to a gated record; deciding where a stage-3 session's state lives, freezes, or bounces; superseding the sibling record schema |
| **Scope** | The design-brainstorm-record contract: home and lifecycle (per gate verdict, incl. the needs-more-product bounce shape), the `design-brainstorm-record/v1` schema and its anchors (`D/F/ALT/S/G/O/T/P-n` + the panel log), the two checklists, the citation grammar, and `tech-design-new`'s consumption rules. **Not**: the session guide's interview prose, the panel's constraint prompts, the eval harness, the stage-1 record class or any stage-1 surface. |
| **Tracking** | filed with the introducing PR |
| **Builds-on** | brainstorm-record-model |
| **Supersedes** | — |
| **Serves** | prd:tech-design-brainstorm/FR-2 · prd:tech-design-brainstorm/FR-7 · prd:tech-design-brainstorm/FR-8 · prd:tech-design-brainstorm/FR-11 · prd:tech-design-brainstorm/FR-12 · prd:tech-design-brainstorm/FR-13 · prd:tech-design-brainstorm/FR-14 · prd:tech-design-brainstorm/NR-2 |
| **Security/privacy impact** | none — repo-local markdown artifacts; no identity, secret, network, or egress surface (artifact-integrity rules bound as invariants, §3.6) |
| **Template version** | tech-design-template/v1 |

## 1. Problem

The tech-design-brainstorm PRD binds *what* a stage-3 session must produce — authority-anchored rulings, scored alternatives with an audited panel call, typed fog, a five-verdict gate, a handoff `tech-design-new` can author from without re-interviewing — and left the artifact itself to this design: where the record lives (its Q1), what its sections and checklists are exactly (Q2, the parked #P-1), and how the distinctively stage-3 organs (the panel log, the alternatives ledger, the bounce evidence) are carried as structure rather than prose. Implementation cannot start without these; and the moment is again the cheapest one — zero records of this class exist, the stage-1 sibling's contract is proven and fresh, and every consumer convention (citation grammar, freeze-at-merge, coverage-checklist derivation) has one working precedent to inherit deliberately instead of re-derive.

## 2. Decision

**Decision:** A design-brainstorm record is **one markdown file per session in the shared `docs/brainstorms/` home, named `<slug>-tech-design.md`** — the class-bearing filename suffix (review ruling, 2026-07-10), the template version, and the routing pair distinguish it from its stage-1 sibling — instantiated from `design-brainstorm-record/v1`, whose typed sections extend the proven spine (routing-pair header, dense anchors, gate mechanics, freeze-at-merge) with four stage-3 organs: a **Basis** header row (the admitted requirement basis, replacing *Idea*), an **Alternatives ledger** (`ALT-n` — every candidate shape with trade-offs, principle citations, and matched incidents), a **Panel log** (one audited line per design question: seam call → fired/skipped/forced/waived → why), and a **Seam map** (`S-n` — the seams the direction opens or extends, feeding the design's Scope and Serves). Citations follow the sibling grammar as `design-brainstorm:<slug>#<anchor>`; the lifecycle is verdict-driven with one stage-3 addition — a **needs-more-product record commits on its own PR as the amendment's evidence** — and merge freezes uniformly.

The reasoning: sharing the home keeps **one provenance shelf** — stage-3 orientation must read stage-1 records anyway (the feeding PRD's record is its richest context), and the shipped stage-1 guide already scans `docs/brainstorms/` verbatim, so a shared home needs zero stage-1 edits (NR-1 there) while a sibling directory would split every future overlap scan across two paths for no discriminating power the template-version row doesn't already provide. The four organs are exactly the PRD's distinctive obligations made structural: FR-8/9's audit becomes a log line you can lint rather than prose you must trust, and the bounce's "name the gap with anchors" becomes a record that rides the amendment review.

## 3. The design

### 3.1 Home, naming, and lifecycle

Records live flat in `docs/brainstorms/` beside the stage-1 class — one file per session, named **`<slug>-tech-design.md`** where `<slug>` is the design effort (usually the feature slug). The class-bearing suffix keeps the shared home collision-free by construction — feature `foo` yields `foo.md` (stage 1) and `foo-tech-design.md` (stage 3) — and declares the class in every listing before a header is read; the header's `Template version: design-brainstorm-record/v1` and routing pair confirm it. Reopening appends an ordinal: `<slug>-tech-design-2.md`. A future volume split (per-class subdirectories) would be a one-time operator migration, deliberately not pre-built (the conventions' own R3 posture).

| Gate verdict | Commit path | Rationale |
|---|---|---|
| *(in-session)* | working-tree file, uncommitted; the resume state (FR-14) | not yet an outcome |
| **ready-for-design** | rides the authored design's introducing PR | reviewer sees design + provenance together (the stage-1 ABUSE-3 mitigation, inherited) |
| **needs-more-product** | committed on its own docs PR, referenced by the follow-up `/prd-amend`'s `Learned-from` | the record *is* the bounce's evidence — the gap, its anchors, and the authority's ruling, reviewable before the ledger changes |
| **needs-investigation** · **covered** | committed on its own docs PR | durable outcome; feeds future orientation |
| **abandoned** | not committed; discarded | nothing stands behind it |

**Decision:** merge freezes a record uniformly; a reopened design effort starts a fresh record citing the frozen one (the class rule, unchanged). The needs-more-product path deliberately produces *two* PRs (record, then amendment): the record freezes the evidence before the contract moves, and the amendment's `Learned-from: operator-decision:design-brainstorm:<slug>#D-n` gives docgate's evidence classes a resolvable reference — exactly the pattern amendment 001 just exercised against a stage-1 record.

### 3.2 The `design-brainstorm-record/v1` schema

The template lands in the templates directory (config `paths.templates_dir`), version-pinned `design-brainstorm-record/v1`; additive evolution within v1, breaking change = v2 by a superseding design.

**Header (closed):** `In one line` · `Relevant-when` — the routing pair, first — then **`Basis`** (the admitted requirement basis, typed like Serves: `prd:<feature>` or `corpus:<IDs>`; replaces the sibling's *Idea*) · **`Authority`** (the stage-3 decision authority of record — the Owner-equivalent, named per the PRD's D-7) · `Date(s)` · `Status` (`in-session` → `gated: <verdict>`; review-loop adjustments per the class rule) · `Tracking` · **`Feeds`** (the authored design's `docs/tech-design/<slug>.md`, set at authoring) · `Prior records` · `Template version`.

**Sections and entry contracts** (every section present; empty reads `none`):

| Section | Anchor | Entry fields |
|---|---|---|
| `## Orientation` | — | sources (path · why) · overlaps surfaced — live designs (from the index), prior records, both classes |
| `## Design questions` | `DQ-n` | the open solution-space questions in scope, each phrased precisely — the panel log keys off these |
| `## Panel log` | — | one line per `DQ-n`: seam call (`opens/replaces` \| `extends`) · action (`fired` \| `skipped` \| `forced` \| `waived`) · reason — mandatory, whatever the action (FR-8/9's audit made structural) |
| `## Decisions` | `D-n` | question · answer (the authority's ruling) · reasoning · anchor (date · turn / `· review`) · status (`live` \| `superseded by D-m — <reason>`) |
| `## Alternatives` | `ALT-n` | name · shape (≤ 4 lines) · trade-offs · principles (AP/RW honored / strained, by ID) · incidents (matched `I-n`, or `none checked <where>`) · verdict (chosen / rejected — why, or hybridized into `ALT-m`) · from (`DQ-n`, panel or hand-authored) |
| `## Seam map` | `S-n` | seam · disposition (`opens` \| `replaces` \| `extends`) · what sits behind it · feeds (the design's Scope/Serves line it will justify) |
| `## Facts` | `F-n` | fact · source |
| `## Fog` | `G-n` | gap · type (`blocking` \| `non-blocking`) · blocks (the direction / a design-template input / a downstream phase) |
| `## Out of scope` | `O-n` | ruling · reason · from (`D-n`) |
| `## Candidate glossary terms` | `T-n` | term · definition · avoid[] |
| `## Parked product gaps` | `P-n` | the requirement gap · its basis anchors (`prd:<feature>/<ID>`) · disposition (`bounced — see Gate` \| `proceed-anyway per D-n`) |
| `## Gate` | — | verdict (the five) · date · authority · covering design (required when `covered`) · coverage result (required when `ready-for-design`) · amend route + gap anchors (required when `needs-more-product`) |

`ALT-n` is deliberately not `A-n`: a gated record gets quoted inside design documents whose own `A-n` acceptance namespace already exists — one letter of disambiguation now beats every future collision. Blocking fog is defined against the coverage targets below (FR-11).

### 3.3 The two checklists

Both live in the template as appendices (the contract travels as one artifact); guides reference, never restate.

- **Structure checklist** (FR-7's fit criterion): the filename carries the class suffix (`<slug>-tech-design.md`); header complete (ten fields); sections present; every `D-n` five-fielded with a live authority anchor; every `DQ-n` has exactly one panel-log line; a `fired` panel yields ≥ 2 `ALT-n` from that `DQ-n`, each with principles and incidents fields filled; `G-n` typed; `O-n` and `P-n` cite their sources; one gate entry with verdict-conditional fields; anchors dense per family; gists not transcripts.
- **Coverage checklist** (FR-13's fit criterion; walked before `ready-for-design`) — pinned to `tech-design-template/v1` and deliberately narrow per the PRD's R6 (the design template's inputs resist full enumeration; the core four are the contract):

| tech-design-template/v1 target | Required record source |
|---|---|
| §2 Decision — the direction | ≥ 1 live `D-n` choosing the shape, its `DQ-n` panel-logged |
| §4 Alternatives considered | the `ALT-n` set — ≥ 2 entries where the panel fired, every entry verdicted |
| header `Serves` + `Scope` | the `S-n` seam map + the `Basis` anchors, enumerated |
| §3.2 Boundaries and non-goals | the `O-n` set |

— plus the gate condition: **zero `blocking` fog**. Any unmet row ⇒ the verdict is `needs-investigation` (or `needs-more-product` where the blocker is a `P-n` gap), never `ready-for-design`.

### 3.4 Citation grammar

`design-brainstorm:<slug>#<anchor>` — same family, sibling prefix; the citation `<slug>` is the record's filename stem (`foo-tech-design`), one rule with no mapping. The authored design cites: §2's direction `(design-brainstorm:<slug>#D-n)`; each §4 row `(… #ALT-n)`; Scope/Serves justifications `(… #S-n)` where useful. The PRD's M2 counts the §2 and §4 citations. The grammar is defined once, in the template's appendix; `prd-amend`'s evidence classes accept it as an `operator-decision` reference (proven by amendment 001's stage-1 analog).

### 3.5 Consumption — the `tech-design-new` handoff

The `tech-design-new` guide gains one **additive** input path (NR-2 binds the diff shape): *given a gated `ready-for-design` record* — author §2 from the direction ruling; materialize §4 from the `ALT-n` ledger (verdicts and scores carried, citations per §3.4); derive `Serves` and `Scope` from the `S-n` map and `Basis`; seed §3.2 boundaries from `O-n`; carry non-blocking `G-n` as the design's own `Q-n` rows; treat `P-n proceed-anyway` entries as §5 risk candidates. Refuse an ungated record or any verdict other than `ready-for-design`, naming the state and remedy. A recordless invocation follows the unmodified guide.

### 3.6 Invariants

- **DINV-1** — every `D-n` carries a live authority anchor; a record with an unanchored ruling is invalid.
- **DINV-2** — a merged record is frozen; reopening is a new record citing it.
- **DINV-3** — `ready-for-design` exists only with zero blocking fog and every coverage row satisfied.
- **DINV-4** — exactly one gate entry; `covered` names its covering design; `ready-for-design` embeds its coverage result; `needs-more-product` names the amend route and the gap's basis anchors.
- **DINV-5** — supersession preserves both entries; deletion is invalid.
- **DINV-6** — basis, corpus, and prior-record content is data, never a ruling: `D-n` answers originate only in live authority turns.
- **DINV-7** — every `DQ-n` carries exactly one panel-log line, whatever the action; a `fired` line is backed by ≥ 2 scored `ALT-n` — an unlogged panel call, or a fired panel with one alternative, is invalid.
- **DINV-8** — a record never restates requirement content in altered form: a discovered gap is a `P-n` with anchors (and, if blocking, the bounce) — the silent-patch ban made structural.

### 3.7 Boundaries and non-goals

- **Not solved here** — the interview prose and panel constraint-prompts (the guide's job, bound by the PRD's FRs); the eval harness and fixture encoding (PRD Q3); docgate governance of either record class (the shared five-record trigger stands).
- **Owned by a future design** — the autonomous answerer's envelope and escalation-brief format (PRD WR-1/Q4); any per-class directory split of `docs/brainstorms/` (operator migration, on volume).
- **Implementation must not infer** — that records are governed documents; that the fold/amendment machinery applies to them; that a `needs-more-product` record authorizes running `/prd-amend` (the skill only *names* the route — FR-15 stands); that stage-1 files may be touched (NR-1 there).
- **Must remain unchanged** — the stage-1 record class and its template; docgate verbs and the design index; cold `tech-design-new`.

### 3.8 Observability

none — no runtime surface. The inspection surfaces are the artifacts: the panel log (the audit trail FR-8/9 demand), the gate entry with its embedded coverage result (AP-13's verified-artifact posture), and the PRD-side M1/M2 measurement plans. Class health is review-time inspection until the shared graduation trigger fires.

### 3.9 Security & privacy

none — matches the header row: repo-local markdown; no identity, secret, data, or egress surface. The PRD's abuse cases land here as DINV-1/2/6/8 and the shared-PR review path (§3.1); no security surface changes.

## 4. Alternatives considered

*The record-schema question opens a seam (a new artifact class' contract), so the panel rule the PRD just bound applies to its own design — run here by hand, each alternative scored against principles and the incident registry (no matched incidents; the registry holds no document-system entries yet).*

| Alternative | Why rejected |
|---|---|
| **Sibling directory** (`docs/design-brainstorms/`) for the new class | splits the provenance shelf: stage-3 orientation reads stage-1 records (the feeding PRD's record is its context), so every scan doubles its paths; the shipped stage-1 guide scans `docs/brainstorms/` verbatim — a shared home costs zero edits. The collision a shared home invites is solved structurally by the class-bearing filename suffix instead. Challenged and re-argued at review; chosen: **shared home + `<slug>-tech-design.md`** (operator ruling, 2026-07-10) |
| **One universal record template** (stage discriminator + conditional sections) | re-litigates the PRD's D-2: conditional sections make both stages' checklists lie; already refused at the requirements level (prd:tech-design-brainstorm §7.3) |
| **Record embedded in the design doc** (a provenance appendix instead of a separate artifact) | couples the session's lifecycle to the design's (no needs-more-product/covered/investigation outcomes without a design PR); breaks resume (FR-14 needs a durable file before any design exists); pollutes the frozen design class with session state — strains AP-7's class rules |
| **`A-n` for alternatives** (mirror the design template's acceptance namespace) | guaranteed cross-document ambiguity the day a design quotes its record beside its own `A-n` rows; one letter (`ALT-n`) buys permanent disambiguation |
| **Panel audit as prose** (no structured log) | FR-8/9's fit criteria demand 100% audit presence — prose can't be linted at graduation and drifts under summarization; a keyed log line per `DQ-n` is the cheapest lintable form (AP-10's codify-over-prompt, staged) |
| **Full design-template coverage enumeration** (every §3.x concern a checklist row) | the template's situational subsections (failure modes, migration, perf) are legitimately absent from many designs — rows for them would force `n/a` theater; the PRD's R6 pre-authorized the narrow core (Decision · Alternatives · Serves/Scope · Boundaries) |

## 5. Risks

| ID | Risk | Impact | Mitigation | Verification / evidence |
|---|---|---|---|---|
| R1 | shared-home crowding: both classes plus verdict variety make `docs/brainstorms/` noisy at volume | orientation scans slow; routing pairs mis-skimmed | routing pair + template-version discriminate cheaply; a per-class split is a named future operator migration (§3.7); volume watched at the five-record checkpoints | directory size review; NFR-1 orientation bound holds on fixtures |
| R2 | the panel log becomes ritual — lines filled, judgment absent | audit theater | DINV-7 binds structure; EVAL-6's discrimination fixtures bind behavior; DOD-5's rubric has the authority affirm the log's accuracy at the gate | eval report + gate confirmations |
| R3 | `ALT-n` scores drift into vibes (principles cited without reasoning; "no incidents" unchecked) | scoring theater | the entry contract requires the incident field to name *where checked* when empty; structure checklist reds an unfilled principles/incidents field on fired-panel entries | checklist walks; A-2 defect fixtures |
| R4 | the two-PR bounce path feels heavy and gets skipped in favor of quiet workarounds | the silent-patch failure returns by another door | the bounce names its amendment anchors (cost already paid in the record); the PRD's sev-1 guardrail + EVAL-4 fixtures police the workaround | eval report; §3.3 guardrail audits |
| R5 | coverage-narrowing (R6's core four) misses a design-template input that mattered | thin designs pass the gate | the checklist pins its template version; a tech-design-template bump reviews the mapping in the same change; DOD-4's real-design demo is the empirical check | A-4 demo evidence; mapping review on bumps |

## 6. Acceptance

| ID | Acceptance criterion | Verify | Required evidence | Traces to |
|---|---|---|---|---|
| A-1 | the `design-brainstorm-record/v1` template exists in the templates directory: closed ten-field header (Basis · Authority rows included), all twelve sections with entry contracts, both checklist appendices, the citation-grammar appendix | Inspection | template file in the implementation PR | prd:tech-design-brainstorm/FR-7 · §3.2 |
| A-2 | a golden record instantiated from the template passes both checklists, and each seeded defect class (unanchored `D-n`; a `DQ-n` with no panel-log line; a fired panel with one `ALT-n`; an unscored `ALT-n`; untyped `G-n`; a second gate entry) turns the structure checklist red at exactly its row | Test | golden + defect fixtures with checklist results | prd:tech-design-brainstorm/FR-7 · #DINV-1 · #DINV-7 |
| A-3 | the `tech-design-new` guide gains the gated-record input path by an additive diff, referencing (never restating) the template's grammar and coverage checklist | Inspection | guide diff | prd:tech-design-brainstorm/NR-2 · §3.5 |
| A-4 | a design authored from the golden record carries `design-brainstorm:<slug>#D-n` on §2 and `#ALT-n` on 100% of §4 rows | Demo | the record + draft design pair | prd:tech-design-brainstorm/FR-13 · §3.4 |
| A-5 | the tech-design-brainstorm guide implements the verdict→commit-path table including the two-PR bounce shape (record as amendment evidence) and abandoned-record discard | Inspection | guide text | prd:tech-design-brainstorm/FR-12 · §3.1 |
| A-6 | a reopen fixture: a fresh record cites the frozen predecessor; orientation surfaces its rulings; none re-asked | Demo | fixture transcript + record pair | prd:tech-design-brainstorm/FR-2 · prd:tech-design-brainstorm/FR-14 · #DINV-2 |
| A-7 | this document passes `docgate lint-tech-designs` and ships with the regenerated index | Test | gate output in the introducing PR | conventions §3.2 |

## 7. Open questions

| ID | Question | Owner | Blocks | Status | Resolution |
|---|---|---|---|---|---|
| Q1 | Whether the panel-log line format gains a lintable micro-grammar at graduation (the shared five-record trigger), or stays checklist-walked | operator | nothing | open | |
| Q2 | Eval fixture corpus home and scripted-authority encoding (PRD Q3 — likely mirrors the stage-1 protocol; confirmed at the eval-harness phase) | implementation | the PRD's eval-harness phase | open | |
| Q3 | Whether `needs-more-product` records should carry a ready-to-paste amendment op sketch (lowering the bounce's cost further) or stay evidence-only | operator | nothing | open | leaning: evidence-only — the amendment is the authority's authored act, not the record's |

## Principles

- **AP-5 — honored.** The grammar extends the provenance chain to architecture: requirement → session ruling → design decision becomes greppable citation end to end.
- **AP-6 — honored.** Closed header, typed sections, dense anchors, scored-alternative entries with named fields: contractual data at the consumer's granularity, compact by rule.
- **AP-7 — honored by extension.** The five governed kinds are untouched; the sibling class adopts the same ungoverned-provenance posture and uniform merge-freeze as its stage-1 twin, mechanization staged behind the shared trigger.
- **AP-10 — honored, staged.** The audit (panel log) and scoring (ALT entry contract) are structure walked by checklist now, lintable at graduation — codified shape first, codified enforcement second.
- **AP-11 — honored.** One grammar, one coverage mapping, one panel-log form — defined in the template, referenced everywhere; version pins on both template axes.
- **AP-13 — honored.** Session completion, panel calls, and the bounce are verifiable artifacts (gate entry, log lines, anchored gaps), never prose claims.
- **AP-21 — honored.** The record is the judge-once artifact for the solution space; `tech-design-new` reads it verbatim and the PRD's M1 measures any re-judging.
- **AP-23 — honored.** One file per session in a flat shared namespace; parallel sessions and the two classes never contend on a shared surface.
- **AP-24 — honored.** Records derive from authority rulings and govern nothing; a discovered requirement gap routes to the ledger's own amendment machinery, never to regeneration.
- **RW-5 — honored.** One independently-revisable decision — the sibling record contract; interview prose, panel prompts, and harness live outside (§3.7).
- **Deviation:** none.
