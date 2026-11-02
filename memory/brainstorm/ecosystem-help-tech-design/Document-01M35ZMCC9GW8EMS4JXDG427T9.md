# Ecosystem help (the /help front door) — design-brainstorm record

| Field | Value |
|---|---|
| **In one line** | Design exploration for the stateless `/help` skill: how the catalog derives from live surfaces within the token budget, how routing and first-steps behave, and where the installer's welcome pointer lands. |
| **Relevant-when** | designing or changing the help skill's derivation mechanics, catalog rendering, problem routing, or first-steps entry; checking prior design rulings or refused shapes about ecosystem discovery surfaces |
| **Basis** | `prd:ecosystem-help` (merged ledger, PR #630) |
| **Authority** | @mdarabi |
| **Date(s)** | 2026-07-23 |
| **Status** | gated: ready-for-design *(after gating, the record changes only through the introducing PR's review loop — rulings added or superseded with review anchors — and freezes entirely at merge)* |
| **Tracking** | agenticlab-ai/pyxis#636 (the introducing PR) |
| **Feeds** | `docs/tech-design/ecosystem-help-v1.md` |
| **Prior records** | brainstorm:ecosystem-help |
| **Template version** | design-brainstorm-record/v1 |

## Orientation

| Source | Why loaded |
|---|---|
| `docs/prds/ecosystem-help/effective.md` | the basis — FR-1..6, UB-1..3, NG-1..5, NFR-1, Q1 |
| `docs/tech-design/index.md` | Serves scan — no live design serves `prd:ecosystem-help`; adjacents identified |
| `docs/tech-design/installer-v1.md` | the welcome-path mechanism (§3.5 step 9: welcome work item from a shipped template; gh transport §3.6) — the FR-15/Q1 seam counterparty |
| `docs/tech-design/config-driven-portability-v1.md` (index row + glossary term) | target-facing surfaces resolve from repo config — governs how help locates the pipeline map and vendor pointers in a foreign repo |
| `docs/tech-design/repo-layout-and-system-footprint.md` (via check-content-paths encounter) | the system tree and content-purity rules the skill's authoring guide must respect |
| `docs/architecture-principles.md` | AP-6 (token consciousness), AP-10/AP-21 (codify vs judge), AP-25 (vendor neutrality), AP-27 (derived aggregates) |
| `docs/observability/incidents.md` | I1–I3 checked; I3 (fail-open CLI misparse) noted for any new verb; no direct match to a prompt-only skill |
| `docs/wiki/glossary.md` | established vocabulary; the landed "onboarding" reservation |
| `.pyxis/pipeline.md` | derivation surface #1 — the stage map with built-status and entry rules |
| `.pyxis/skills/*/SKILL.md` frontmatter (sampled) | derivation surface #2 — every skill ships `name` + `description` YAML frontmatter, already written as when-to-use routing text |
| `brainstorm:ecosystem-help` (frozen stage-1 record) | prior rulings D-1..D-7; F-5 (installer seam); P-1/P-2 constraints carried in |

**Overlaps surfaced:** no live design serves `prd:ecosystem-help` — the space is open. Adjacent, not covering: `installer-v1` owns the welcome work item's creation (help owes only the pointable entry — the seam is a content contract, not shared machinery); `config-driven-portability-v1` owns how target-facing paths resolve (help must read through it, not around it); the wiki's `reading room` (workbench-ui) remains the chartered home of corpus browsing, per the basis's NG-2.

## Design questions

- **DQ-1**: What is the derivation mechanism — how does `/help` turn the live surfaces (pipeline map, skill frontmatter, authoring-guide headers, wiki routing headers) into the catalog at invocation, within NFR-1's routing-surfaces-only budget?
- **DQ-2**: Where does the first-steps entry live so it is both conversationally reachable ("where do I start?") and directly invokable by the installer's welcome pointer (FR-6 / installer FR-15)?
- **DQ-3**: How does routing (FR-2/UB-2/UB-3) decide — prompt judgment over the derived catalog, or a codified entry-rules table the skill reads?
- **DQ-4**: What proves FR-4's derivation purity mechanically — how does the add-a-skill-changes-nothing-in-help fit criterion become a test?

## Panel log

- **DQ-1**: opens a seam (the derivation contract between help and every routing surface) — fired — the mechanism decides whether surfaces stay as-is or gain structure; ≥ 2 orthogonal shapes exist (pure prompt-walk vs codified enumerator)
- **DQ-2**: extends the existing skill-invocation seam (a skill argument, like every existing skill) — skipped — no new interface; the installer already points at named skills
- **DQ-3**: extends DQ-1's chosen surface-reading seam — skipped — routing consumes the same derived catalog plus the map's own entry section; judgment stays in the one prompt site (AP-21), no new boundary
- **DQ-4**: extends the existing scripts-test seam (`.pyxis/scripts/tests/` harness pattern) — skipped — a conformance test over existing surfaces, no new interface

## Decisions

### D-1 — Derivation mechanism: prompt-walk (DQ-1)

- **Question**: How does `/help` derive the catalog — ALT-1 pure prompt-walk over the raw surfaces, or ALT-2 a codified enumerator verb?
- **Answer**: ALT-1 — the authoring guide drives in-session reads of the pipeline map and every skill's frontmatter; no new binary, no generated artifact. FR-4's purity proof lands as a text-level conformance test on the existing scripts-test harness (no per-capability literals in help's own files), answering DQ-4 within the same ruling.
- **Reasoning**: Authority adopted the recommendation — ALT-2 is contractually barred by the basis appetite's explicit enumeration-API exclusion (F-5); the prompt-walk's costs have in-scope mitigations (F-1: frontmatter is already routing data; F-2: the map is one bounded read); proven fragility routes to a future amendment, not a design-time override.
- **Anchor**: 2026-07-23 · turn 2
- **Status**: live

### D-2 — The welcome-pointer invocation: `/help start` (DQ-2)

- **Question**: What invocation does the installer's welcome work item carry to land directly on the first-steps path?
- **Answer**: `/help start` — an argument on the one skill: bare `/help` gives the catalog; `start` lands on first-steps directly (FR-6, no interactive navigation). The installer's shipped welcome template carries the literal string; S-2 stays a content contract, no shared machinery.
- **Reasoning**: Authority confirmed the recommendation — a separate alias/welcome skill would reopen the two-door problem stage-1 D-7 closed; `start` over `first-steps` (newcomer-typeable) and over `welcome` (the installer's vocabulary, kept distinct per glossary T-1's reservation).
- **Anchor**: 2026-07-23 · turn 3
- **Status**: live

## Alternatives

### ALT-1 — Pure prompt-walk (the skill reads surfaces raw)

- **Shape**: the authoring guide instructs the session to read `.pyxis/pipeline.md` whole plus every `SKILL.md` frontmatter block (and wiki headers on demand), then compose catalog/route/first-steps answers in-session. No new code, no generated artifact.
- **Trade-offs**: wins — zero machinery, ships as one guide + skill glue, surfaces stay untouched; costs — the read set is N file opens per invocation (N = skill count), budget discipline is prompt-enforced only (AP-10 strain), and FR-1's "appears exactly once" fit rides on model diligence.
- **Principles**: AP-6 honored-at-risk (routing surfaces only, but per-invocation re-reads); AP-10 strained (the enumeration rule lives in a prompt); AP-25 honored; AP-19 honored (fires only on invocation).
- **Incidents**: none checked matching (registry read 2026-07-23; I1–I3 are gate/CLI classes, no prompt-walk failure recorded yet).
- **Verdict**: chosen — the appetite bars ALT-2's machinery; costs mitigated by F-1/F-2, purity testable without a verb (D-1)
- **From**: DQ-1 · panel

### ALT-2 — Codified enumerator verb (a small Go reader renders the catalog skeleton)

- **Shape**: a read-only verb (e.g. `pyxis help-index` or a docgate-style reader) that scans the map's stage table + every skill's frontmatter and emits one compact, deterministic catalog block (names, one-liners, built-status); the skill invokes it and layers routing/first-steps judgment on top.
- **Trade-offs**: wins — enumeration is code (AP-10), "exactly once + status carried" is a unit test not a hope, one bounded read per invocation (AP-6), and FR-4's purity test falls out free; costs — a new binary surface to ship with the payload, and the appetite explicitly excluded "an enumeration API" — this shape must argue it is a *verb*, not an API, or it violates the basis.
- **Principles**: AP-10/AP-6/AP-27 honored (derived aggregate, no authored center); AP-19 honored; strains the basis's appetite exclusion (prd:ecosystem-help header) — the deciding question.
- **Incidents**: I3 matched (a new CLI verb must fail closed on misparse — guard + tests inherited from the docgate/plangate lesson).
- **Verdict**: rejected — barred by the basis appetite's enumeration-API exclusion (F-5); revisitable only through a `/prd-amend` reopening the appetite (D-1)
- **From**: DQ-1 · panel

## Seam map

- **S-1**: the derivation contract (help ↔ every routing surface) — opens — behind it: how catalog truth is read (raw prompt-walk or codified enumerator) — feeds: the design's §2 Decision and Serves (FR-1, FR-4, NFR-1)
- **S-2**: the welcome-pointer content contract (installer FR-15 → help's first-steps entry) — extends — behind it: the invocation string the welcome template carries — feeds: Serves (FR-6) and the design's §3.2 boundary with installer-v1

## Facts

- **F-1**: Every shipped skill carries YAML frontmatter (`name`, `description`) written as when-to-use routing text — the catalog's per-skill one-liner already exists as data — source: `.pyxis/skills/*/SKILL.md` (sampled feature-brainstorm, prd-new, and plan-new).
- **F-2**: The pipeline map ships in the distributable surface and carries per-stage skill/mechanism, human gates, and built-status legend (✅ ◐ ⏸ ⛔) plus a "where a problem enters" section — routing's ground truth — source: `.pyxis/pipeline.md`.
- **F-3**: The installer's welcome work item is created from a shipped template at `pyxis init` step 9 (gh transport, tracker-wired modes only) — the pointer target must be a stable invocation string that template can carry — source: `docs/tech-design/installer-v1.md` §3.5/§3.6.
- **F-4**: Target-facing paths (doc paths, vendor pointers, root convention files) must resolve from repo config, not hardcoding — help reads foreign repos through the same binding — source: `config-driven-portability-v1` (index row; glossary "target-facing surface").
- **F-5**: The basis's appetite excludes "an enumeration API or generated index over the derivation surfaces"; fragility findings route to the surface as amendments — source: `prd:ecosystem-help` header (brainstorm:ecosystem-help#D-6).

## Fog

none

## Out of scope

- **O-1**: No enumerator verb, generated index, or any new binary/artifact surface for derivation — the appetite bars it; fragility routes to a `/prd-amend` (from D-1)
- **O-2**: The welcome work item's creation, timing, and transport stay installer-v1's — help owes only the stable `/help start` entry the template carries (from D-2)

## Candidate glossary terms

none

## Parked product gaps

none

## Gate

| Field | Value |
|---|---|
| **Verdict** | ready-for-design |
| **Date** | 2026-07-23 |
| **Authority** | @mdarabi |
| **Covering design** | — |
| **Coverage check** | pass — core four covered (§2 Decision: D-1/D-2, DQ-1 panel-logged · §4 Alternatives: ALT-1 chosen/ALT-2 rejected, both scored · Serves+Scope: S-1/S-2 + basis anchors · §3.2 boundaries: O-1/O-2), zero blocking fog |
| **Amend route** | — |
