# Argus artifacts — the ledger classes, the authority contract, and their grammars

| Field | Value |
|---|---|
| **Status** | Draft → Approved *(merge is the freezing event; "superseded" is computed by the index, never written here)* |
| **Owner** | @mdarabi |
| **Approver(s)** | @mdarabi (operator) — factory-control data surface, named reviewer mandatory |
| **Date** | 2026-07-11 |
| **Relevant-when** | defining, parsing, or changing Argus's problem/delivery ledgers, the authority-contract data, their citation grammars, or the measurement mapping; triaging attribution, audit, staleness, or grant-check defects; authoring the templates or any future lint over these classes |
| **Scope** | The three data contracts the argus PRD requires: `problem-ledger/v1`, `delivery-ledger/v1`, `authority-contract/v1` — sections, entry types, attribution fields, freeze semantics, the citation grammars (`problem:` · `delivery:`), the EVT↔entry mapping, the checkpoint walk's evidence block, and the never-list's frozen home. **Not**: the drive protocol that writes them (argus-agent-v1 owns behavior); the per-stage seam amendments; the report classes (the reviewer designs own those). |
| **Tracking** | — |
| **Builds-on** | docs/tech-design/brainstorm-record-model.md · docs/tech-design/design-brainstorm-record-model.md · docs/tech-design/prd-review-artifacts.md |
| **Supersedes** | — |
| **Serves** | prd:argus/FR-1 · prd:argus/FR-2 · prd:argus/FR-4 · prd:argus/FR-8 · prd:argus/FR-11 · prd:argus/FR-12 · prd:argus/FR-13 · prd:argus/FR-14 · prd:argus/FR-15 · prd:argus/NFR-1 · prd:argus/NFR-2 |
| **Security/privacy impact** | none — repo-internal markdown data contracts; no new data classes beyond operator handles already in the repo, no network surface, no egress change |
| **Template version** | tech-design-template/v1 |

## 1. Problem

The merged argus contract binds artifacts that nothing yet defines. The drive needs a root intent artifact that accretes human answers (FR-1, FR-8), a journal that makes any interruption lossless (FR-2), authority as flippable data with a kill switch (FR-11), permanent typed attribution on every ruling (FR-12), exposed ages (FR-15), and measurements computable from artifacts alone (FR-13, the M-family) — all with freeze semantics that survive events arriving *after* a ledger closes, and with the never-list structurally out of reach of any data edit. Without bound contracts, every drive improvises formats: unparseable by future tooling, unauditable by humans, and unable to carry the evidence the graduation checkpoints consume.

## 2. Decision

**Decision:** Argus's data surface is **three versioned contracts and two citation grammars** (design-brainstorm:argus-tech-design#D-3, #D-4). Per problem, two sibling instances live at `docs/problems/<slug>/`: `problem.md` (`problem-ledger/v1`) — the idea verbatim plus an append-only-forever family of human authority entries `A-n` — and `delivery.md` (`delivery-ledger/v1`) — an append-only journal of typed, attributed entries `E-n` with a regenerated derived-state block, closing exactly once at content-verified delivery. At the agent's home, one `authority-contract/v1` file declares the **ordered tier ladder** — named bundles of action classes, one `current-tier` pointer, an append-only change history carrying checkpoint evidence — and the `revoked` below-floor tier is the kill switch. Citations run `problem:<slug>#A-n` and `delivery:<slug>#E-n`; **an assumption's load-bearing spot cites the journal entry that recorded it**, and post-close events reach a frozen ledger only by reverse citation. The never-list is frozen design content **here**, never data.

A reader who stops here should see the shape: intent and its answers in one append-only place, every act journaled with attribution, authority as one pointer on a declared ladder, and every measurement a read over these files — nothing emits, nothing else remembers.

## 3. The design

### 3.1 The problem ledger — `problem-ledger/v1`

**Decision:** `docs/problems/<slug>/problem.md` holds: a closed header of intake-time facts only (slug · principal identity · authority-contract version at intake · optional per-problem envelope override — **no mutable cell**: status lives on the delivery side, where the derived block owns it), the **Idea** section (the human's words, verbatim, never edited), and the **`A-n` family** — one dense, append-only sequence of human authority entries: pre-rulings recorded at intake, then every breaking-call answer as it arrives, each entry dated, attributed to its human author, carrying the authority-contract version current at recording, and (for answers) citing the brief it answers (`delivery:<slug>#E-k`). An `A-n` binds as a ruling-in-advance for everything downstream; the file is append-only forever with no exceptions — DINV-1 is absolute. The slug is kebab-case, minted at intake from the idea; a collision appends `-2`, `-3`.

### 3.2 The delivery ledger — `delivery-ledger/v1`

**Decision:** `docs/problems/<slug>/delivery.md` holds: a closed header (slug only — status is derived, per DINV-6), a **derived state block** (**status: `open | parked | delivered`** — the one closed enum, derived from the journal — plus stage position against the pipeline map, open items, park state with ages; regenerated on every append, never cited, never authoritative), and the **`E-n` journal** — append-only typed entries from a **closed type vocabulary**: `stage-entered` · `ruling-recorded` · `assumption-recorded` · `item-materialized` · `brief-emitted` · `answer-received` · `proposal-disposed` · `reconciliation` · `bump-issued` · `delivered`. Every entry carries: type, date, **principal identity + authority-contract version** (FR-12), a gist body, and — wherever the act touched anything external — the **external-effect identity** (tracker item id, PR number, commit) that lets a resuming session verify the effect against its goal object before any redo. Type-specific fields bind the PRD's typed event properties: a `brief-emitted` entry carries `class: override | deadlock`; the `delivered` entry carries the touch count, breaking count, and spend fields (adapter-honest, `unavailable` legal); an `answer-received` entry cites its `A-n`. The journal closes with exactly one `delivered` entry, recorded only after mainline-content verification, and the file freezes; post-close overturns never touch it — the overturning artifact carries `overturns delivery:<slug>#E-n`, discoverable by reverse citation (design-brainstorm:argus-tech-design#D-8).

### 3.3 The authority contract — `authority-contract/v1`

**Decision:** one versioned file at the agent's home declaring, as data: the **ladder** — an ordered table of named tiers, each a bundle of action classes; the **`current-tier`** pointer; and the **change history** — an append-only section of `checkpoint` entries, each recording date, **`move: promotion | demotion | revocation | none`** (one field, one closed enum), the resulting **`tier`** (EVT-5's binding — for `none`, the unchanged current tier), and the evidence refs behind the move (checkpoint walk blocks, incident anchors). The initial ladder, full vocabulary (design-brainstorm:argus-tech-design#D-5's binding):

| Tier (ordered) | Bundle (action classes) |
|---|---|
| `revoked` | ∅ — every act escalates; the kill switch |
| `floor` | dispose-advisory-finding |
| `tier-2` | + dispose-blocking-finding · sign-gate-verdict · trigger-materialization |
| `tier-3` | + rule-acceptance |
| `tier-4` | + approve-merge *(activates only after the operator changes org governance — the PRD's NG-1 boundary)* |

*(Placement note, per review: `trigger-materialization` joins tier-2 rather than a later tier because plan-new's own consent and idempotency contract already bounds the act — FR-9 defers to it — and materialized items are closable, so the act is reversible at feature scope; the PRD's §6.5 stage rows named the decision-class headlines only, and the bundles-as-data carry the full vocabulary by the record's D-5 binding.)*

**Action-class tokens, defined** (the closed vocabulary a grant check reads): `dispose-advisory-finding` — record a take-or-leave disposition on a reviewer advisory; `dispose-blocking-finding` — apply or decline-with-reason a blocking finding (a decline auto-escalates by the reviewers' own rule); `sign-gate-verdict` — sign a stage gate (problem/solution/record gates) as the principal; `trigger-materialization` — give the planning stage its go signal (consent stays plan-new's, FR-9); `rule-acceptance` — record the D-2-of-basis acceptance ruling on a delivered feature; `approve-merge` — approve a proposal for landing. **The default is fail-closed**: an act class appearing in no tier's bundle is not grantable by any data state — it escalates; adding a class to the vocabulary is a successor-design decision, never a quiet bundle edit. **The grant-checked line**: the ladder governs *decision and permission* acts (the classes above); acts inherent to driving — reading, orienting, authoring drafts, running a stage's own protocol as its principal — are the drive itself, bounded by the stage contracts and the never-list, not by grant rows.

A grant check is one lookup: is the act's class within the current tier's bundle? Bundles and ladder are data, amendable by PR — re-cutting a bundle is a visible, reviewed act. **The never-list is not in this file**: the structural refusals — amending this file or any grant, promoting a tier, **un-parking an escalation Argus itself raised**, authoring governance-tier content, overriding a human ruling, an incident lesson, or a breaking-call answer, exceeding the problem's effective envelope, crossing ungranted outward surfaces — are frozen content of *this design* and the agent's contract (prd:argus UB-1..UB-5), unreachable by any data edit. **Writer discipline**: Argus never writes this file — authority changes are human-versioned data changes (the basis's ruled posture): the checkpoint walk *prepares* an evidence block, and a human lands it by PR, promotion or none; a `checkpoint` entry's `move` may be `none`, so no-move checkpoints have a landing spot too.

### 3.4 Grammars, measurement mapping, and the checkpoint walk

**Decision:** two new citation namespaces join the house grammar set: `problem:<slug>#A-n` and `delivery:<slug>#E-n` — disjoint from every namespace in the glossary's grammar set, the owning surface for that enumeration (anchors resolve by grep). The binding rule: **a ruling's load-bearing spot cites its journal entry** — a PRD row, design decision, or record entry shaped by an Argus assumption carries `(delivery:<slug>#E-n)`, giving the FR-4 audit its two-way trace. The measurement mapping is a read, not an emission:

| PRD event | Is exactly |
|---|---|
| EVT-1 assumption_recorded | an `assumption-recorded` entry |
| EVT-2 brief_emitted / brief_answered | the `brief-emitted` entry · the `answer-received` entry citing its `A-n` |
| EVT-3 problem_delivered | the closing `delivered` entry (mainline-content-verified) |
| EVT-4 overturn_recorded | a reverse citation — `overturns delivery:<slug>#E-n` in the overturning artifact, carrying `channel: amendment | supersession | audit-annotation` and `missed-breaking: yes | no` |
| EVT-5 tier_changed | a change-history `checkpoint` entry in the authority contract — its `move`, resulting `tier`, and evidence refs (§3.3's one closed vocabulary) |

**The overturn duty and its backstop**: the `overturns …` citation is authored by whoever lands the overturning change — the human's amendment, supersession, or audit annotation carries it (the agent design's seam and audit conventions prompt for it). The backstop owner is the **checkpoint walk**: its sweep scans the window's landed amendments, supersessions, and annotations that touch Argus-attributed rulings and adds any citation-less overturns to the evidence block, so a forgotten citation delays measurement by at most one checkpoint, never loses it.

The **checkpoint walk** computes M1–M3 at each ten-problem checkpoint by reading closed ledgers and reverse citations, and its output becomes the checkpoint's **evidence block** in the authority contract's change history — the exact input FR-14's promotion rule consumes — **prepared by the walk's runner, landed by a human PR** (§3.3's writer discipline). In the bootstrap the walk is performed by the implementing session under operator direction; a registry-born verb takes it over in the engine era (AP-10's arc, per the distributed-check-registration pattern).

### 3.5 Invariants

- **DINV-1** — A problem ledger is append-only forever: no `A-n` is ever edited, renumbered, or removed; answers are permanent.
- **DINV-2** — A delivery journal is append-only and closes exactly once (one `delivered` entry); after close the file never changes — post-close events reach it only by reverse citation.
- **DINV-3** — Every `A-n` and `E-n` carries its principal identity and authority-contract version; an unattributed entry is invalid.
- **DINV-4** — The authority contract admits no state outside its declared ladder: `current-tier` always names a declared tier, and every change appends a history entry with evidence refs; a bare pointer flip with no history entry is invalid.
- **DINV-5** — No content of the authority-contract file can grant a never-class act; the never-list binds from frozen design and contract text, out of reach of data edits.
- **DINV-6** — Derived state blocks are regenerable projections: never cited, never authoritative; on any conflict the journal wins.

### 3.6 Observability

The artifacts are the observability surface — measurements ride them (the PRD's §5.7 posture) and nothing emits elsewhere. Correctness is guarded by the classes' structure checklists at review time and by the agent design's drills (resume, flip, walk); a mechanical lint over these classes is deliberately deferred to the engine era (§7 Q-1), the same staging every record class took.

### 3.7 Security & privacy

none — matches the header row. The contracts add repo-internal markdown holding operator handles already present in the repo; no new egress, network, or permission surface. Injection posture: ledger content is data to every reader — the agent contract's UB-6 binds the behavior side; nothing in these files is executable.

### 3.8 Compatibility and migration

New classes; nothing migrates. Template versions are pinned at instantiation (`problem-ledger/v1` · `delivery-ledger/v1` · `authority-contract/v1`); evolution is a template version bump decided by a successor design, never an in-place redefinition. The templates ship with the implementation at the house template home — **each carrying a structure-checklist appendix** (walked at instantiation, close, and review — the record-class precedent) — with their layout rows added to the operating manual in the same change.

**Mutability class — stated, because it is novel here**: these instances are **registry-style living documents** — the incident registry's posture, not the frozen-at-merge posture of designs and ledger entries. The files grow by appends across many PRs after their first merge; what freezes is each *entry* once merged (and the whole delivery journal at close). AP-7's supersession applies at entry grain: recorded content is never edited — a correction is a new entry citing what it corrects. **Docgate boundaries**: these classes take no fold, are not PRD-ledger entries (`check-single-amendment` does not apply), and `check-immutable` must not flag their appends — they live outside its governed set until the Q-1 lint gives them their own mechanical guard; until then the templates' checklists and review are the guard.

## 4. Alternatives considered

| Alternative | Why rejected |
|---|---|
| In-place snapshot state for the delivery ledger (design-brainstorm:argus-tech-design#ALT-1) | erases history — attribution needs a second mechanism; a crash between act and write repeats the act on resume (UB-7's hazard); I2's trust-the-status shape |
| Stateless drive — external state as the only truth (#ALT-3) | re-orientation cost every session; the basis binds ledger content existing (FR-2, FR-15, the EVT fields); **hybridized**: its observe-before-redo posture is the resume rule |
| Flat grant table for authority (#ALT-4) | the §6.5 ladder becomes implicit — promotions flip several rows, out-of-order flips are representable; **hybridized**: its action-class vocabulary fills the bundles |
| Stage×action grant matrix (#ALT-6) | the basis graduates by action class, not stage; combinatorial surface with no ruled consumer |
| A second event/telemetry surface for the EVT rows | two surfaces drift; the journal already carries every event as an attributed entry (design-brainstorm:argus-tech-design#D-8, O-2) |

## 5. Risks

| ID | Risk | Impact | Mitigation | Verification / evidence |
|---|---|---|---|---|
| R1 | hand-regenerated derived blocks drift from their journals | a resume trusts a stale block | DINV-6 (journal wins, block never authoritative); the agent design's resume drill replays the journal, not the block | kill-resume drill (A-3) |
| R2 | the authority-contract file is mis-edited (malformed tier, dangling pointer) | grant checks misfire | DINV-4 (no state outside the ladder, history mandatory); changes only by PR; closed parse when the engine reads it | flip drill (A-4); review |
| R3 | grammar collision with future document classes | ambiguous citations | `problem:` and `delivery:` are new namespaces, disjoint from every existing grammar token; anchors grep-resolvable | inspection against the glossary's grammar set |
| R4 | unbounded journal growth on long drives | illegible ledgers | entries are gists with citations — bodies reference artifacts, never duplicate them; the derived block keeps the current view one screen tall | NFR-2's legibility demo (A-2) |

## 6. Acceptance

| ID | Acceptance criterion | Verify | Required evidence | Traces to |
|---|---|---|---|---|
| A-1 | The three templates exist at the house template home, version-pinned, with layout rows in the operating manual | Inspection | the implementing PR | §3.8 |
| A-2 | A seeded problem instantiates both ledgers with every DINV-3 field present, and a fresh reader locates stage position, open items, and park state from the derived block alone | Demo | seeded fixture + read-through | prd:argus/FR-1 · FR-12 · NFR-2 |
| A-3 | Kill-resume drill: a session killed mid-drive is resumed by a fresh session from the journal alone, reaching the same next action with zero duplicated external effects | Test | drill transcript + journal diff | prd:argus/FR-2 · #DINV-2 · #DINV-6 |
| A-4 | Flip drill: moving `current-tier` changes grant outcomes with no prose change; `revoked` escalates every act; a bare pointer flip without a history entry is rejected at review | Test | drill transcript | prd:argus/FR-11 · #DINV-4 |
| A-5 | The checkpoint walk over closed fixtures — including one post-close overturn reverse-citation — produces an evidence block appended to the change history, computing M1–M3 | Demo | the walk's output block | prd:argus/FR-13 · FR-14 |
| A-6 | No edit to the authority-contract file can grant a never-class act (self-grant, tier self-promotion, un-parking its own escalation, governance authoring, human-ruling override, envelope breach, ungranted outward act) | Inspection | the file's schema vs the frozen never-list; the agent design's UB-1 eval | #DINV-5 · prd:argus/NFR-1 |
| A-7 | Per-DINV defect fixtures exist and each is red at exactly its row (edited `A-n`, second `delivered` entry, unattributed entry, out-of-ladder tier state, bare pointer flip, cited derived block) | Test | the fixture corpus + first-run report (with the agent design's harness) | #DINV-1..6 |

## 7. Open questions

| ID | Question | Owner | Blocks | Status | Resolution |
|---|---|---|---|---|---|
| Q-1 | When does a mechanical lint over these classes (closed parse, DINV checks) join the gate registry? | engine era | nothing | open | staged like every record class — on volume evidence, born into the check registry |
| Q-2 | The spend-field vocabulary on the `delivered` entry per CLI (adapter-honest capture) | implementation | nothing | open | decided at implementation; `unavailable` is always legal, invention never is |

## Principles

- **AP-27 — honored.** Order-bearing state is a ledger by design (journal, answers, change history); per-problem directories keep parallel problems file-disjoint; the one central aggregate (the derived block) is regenerated, never authored.
- **AP-21 — honored.** The ruled rollout ladder is judged once and frozen as data; every later move is one pointer with evidence, not a re-litigation.
- **AP-26 — honored.** `delivered` records only mainline-content-verified delivery; resume verifies external effects against goal objects; overturns are read from the goal side (reverse citations), never trusted status.
- **AP-10 — honored.** Policy and evidence are data; the checkpoint walk starts as a recorded hand computation and graduates to a registry-born verb.
- **AP-19 — honored.** Nothing recurs: appends do the work, parked problems cost zero, regeneration happens only at append time.
- **AP-6 — honored.** Contractual data: closed type vocabularies, closed headers, pinned template versions, additive evolution by version bump.
- **AP-5 — honored.** The journal is the event surface — every observable moment of a drive is a typed, attributed entry; measurements are reads over it, nothing emits elsewhere.
- **AP-7 — honored.** Supersession at entry grain: recorded content is never edited; corrections and overturns arrive as new entries or reverse citations (§3.8).
- **RW-5 — honored.** One decision — Argus's data surface — in one document; the behavior half is the sibling design, per the record's D-1 partition (design-brainstorm:argus-tech-design#D-1).
- **Deviation:** none.
