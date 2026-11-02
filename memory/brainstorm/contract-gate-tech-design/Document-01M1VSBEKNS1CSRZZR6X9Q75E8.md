# Contract gate — mechanical — design-brainstorm record

| Field | Value |
|---|---|
| **In one line** | Design-space grill for the mechanical stage-6 contract gate (prd:contract-gate): its tool home (docgate-hosted vs a separate binary), the check-input data path (fold enrichment), the defect-report transport, the verdict schema's reserved semantic fields, and how its checks register + wire into plan-new. |
| **Relevant-when** | designing or changing the contract gate's shape or home; its relationship to docgate's registry/fold/machine-faces; the per-requirement field enrichment of the PRD source map; the defect-report transport/storage; the verdict schema's LLM-reserved fields; the plan-new planning-entry precondition — or checking prior design rulings/refused shapes for stage-6 contract validation |
| **Basis** | prd:contract-gate (merged ledger, PR #418, 2026-07-17) |
| **Authority** | @mdarabi |
| **Date(s)** | 2026-07-17 |
| **Status** | gated: ready-for-design |
| **Tracking** | — |
| **Feeds** | docs/tech-design/contract-gate-v1.md |
| **Prior records** | brainstorm:contract-compiler-2 · brainstorm:contract-compiler · design-brainstorm:implementation-verification-tech-design |
| **Template version** | design-brainstorm-record/v1 |

## Orientation

| Source | Why loaded |
|---|---|
| docs/prds/contract-gate/effective.md | the basis — FR-1..13, UB-1..6, NR-1..3, NFR-1..6, NG-1..5, WR-1..3; held verbatim |
| docs/tech-design/index.md | the live decision graph — **nothing Serves prd:contract-gate** (open ground); the Builds-on candidates (docgate-v1/v2, distributed-check-registration, plan-new-v1) |
| docs/brainstorms/implementation-verification-tech-design.md (frozen) | the sibling gate's design rulings — D-1 (delivery shape: separate binary *because code ≠ docs domain*), D-3 (verdict artifact never committed), D-7 (DCR registration in its own registry), D-8 (codegate naming) |
| docs/tech-design/distributed-check-registration.md (live) | the DCR pattern — per-file init() self-registration into a per-gate registry, assembled by verb (AP-27); §3.8 "distribute what grows per feature"; docgate + plangate each carry one |
| internal/docgate/ledger/ledger.go · internal/docgate/fold/fold.go | the fold internals — `Unit.Cells` (every requirement row already tokenized) + `Table.Cols`; `UnitState` (the machine face) exposes lineage only |
| internal/docgate/gate/reg_check_citations.go · internal/docgate/techdesigns/index.go | check-citations (link resolution — already a docgate check); serves_inverse + GenCoverage + stale-serves/stale-builds-on (coverage + currency substrate — already docgate machinery) |
| docs/architecture-principles.md (via the earlier study) + incident registry | AP-5/6 data · AP-10 reuse-not-rebuild · AP-11 single-source predicate · AP-13 verdict-binds-what-it-certifies · AP-21 judge-once · AP-27 derived-registration; the #149 derived-artifact-staleness family |
| docs/wiki/glossary.md | contract gate (landed with the basis) · source map · Serves · the design class |

**Overlaps surfaced:** `design:distributed-check-registration` (live) — the registration pattern DQ-5 extends, not reinvents. `design:implementation-verification-tech-design` (frozen, the sibling) — its DQ-1 chose a *separate* binary because code-verification is a different domain; contract-gate's checks are the *same* (doc-system) domain, so its precedent argues the opposite way here (F-1). No live design covers contract validation — open ground.

## Design questions

- **DQ-1**: What is contract-gate's tool home — a verb/mode on the existing **docgate** binary (composing docgate's own checks + machine faces), or a separate pinned **`cmd/contractgate`** binary in the docgate/plangate/codegate family — given its five checks are doc-system checks over docgate's own machine faces?
- **DQ-2**: Where do the structural + inverse-coverage checks get their per-requirement inputs (fit-criterion presence, declared verification method, Must/priority) — enrich the fold's machine face (the single parser), or have the gate parse `effective.md` itself? (Carries the PRD R1 budget audit.)
- **DQ-3**: How does the single defect report travel and persist — one atomic structured artifact to a gitignored path the host reads (the codegate-verdict precedent), never git-committed, versus a git-committed report — given a committed report self-stales the moment the contract is fixed (#149 family)?
- **DQ-4**: Are the verdict's semantic-verdict fields (reserved for the LLM follow-up #419) specified in the v1 closed schema now (present-but-not-evaluated), or deferred entirely to the follow-up?
- **DQ-5**: How do contract-gate's checks register and compose into a feature-scoped verdict (extending the DCR pattern, per DQ-1's home), and how is the planning-entry precondition wired into plan-new's frozen ledger (the NR-1 amendment)?

## Panel log

- **DQ-1**: opens (a new tool-home / module boundary) — **fired** — ≥ 2 structurally distinct homes with different reuse/identity/coupling profiles (docgate-hosted verb vs separate pinned binary)
- **DQ-2**: extends (the fold's existing machine-face schema — the single ledger parser) — **skipped** — the only alternative (a second parser in the gate) is ruled out by the single-parser discipline, not a genuine orthogonal seam; the audit (F-3) shows the data is already tokenized, so this is a projection add
- **DQ-3**: opens (a new committed-artifact class — the git-retained defect report) — **fired** — the authority's ruling revealed two structurally distinct artifact classes (gitignored-ephemeral vs git-committed immutable event record) with different retention/learning profiles; panel fired retroactively, resolved by D-3
- **DQ-4**: extends (AP-5/6 additive closed-schema evolution) — **skipped** — schema-field reservation composes existing data discipline; no new seam
- **DQ-5**: extends (the DCR pattern into docgate's registry; the plan-new precondition as a typed amendment the PRD's NR-1 already anticipates) — **skipped** — registration extends the live pattern; the amendment is an integration mechanic, not a new seam

## Decisions

### D-1 — tool home: docgate-hosted (a contract-gate verb composing docgate's checks)

- **Question**: DQ-1 — a docgate verb/mode reusing docgate's checks + machine faces, or a separate `cmd/contractgate` binary?
- **Answer**: **ALT-1** — "go with ALT-1 (reuse docgate)." Contract-gate is a feature-scoped, verdict-producing mode on docgate: a `docgate contract-gate <feature>` verb composing the five checks (reusing check-citations, GenCoverage, the index's stale flags + new structural/compilability checks self-registered into docgate's registry) into the verdict + one defect report; plan-new invokes it at planning entry, a docgate CI check runs the advisory continuous surface.
- **Reasoning**: the checks *are* docgate's machinery (F-2); a separate binary would duplicate them (the §4.1-C failure) or import docgate's internals anyway. The sibling's separate-binary ruling was domain-driven (code ≠ docs, F-1); contract-gate is same-domain. Cost accepted: docgate gains a verdict-producing feature-scoped mode — additive, not a conflict. Recommendation adopted.
- **Anchor**: 2026-07-17 · t1
- **Status**: live

### D-2 — check-input data path: enrich the fold's machine face (single parser); R1 de-risked

- **Question**: DQ-2 — do the structural + inverse-coverage checks read per-requirement fields from an enriched fold machine face, or a second parser in the gate?
- **Answer**: **enrich the fold**. Add typed per-requirement fields (priority, has-fit-criterion, verify-method) to the source map, extracted by the fold from the already-parsed row cells by header-column name; the gate reads the machine face and never re-parses `effective.md`.
- **Reasoning**: single-parser discipline — the fold is the sole ledger parser (AP-11; a second parser drifts, the §4.1-C failure D-1 refused). The audit de-risks the PRD's R1: the fold *already* tokenizes every requirement row into `Unit.Cells` (F-3), so this is a **projection add** (additive schema, AP-5/6, landing on docgate's fold) + a header-named column read — not the new table-parsing R1 feared. Exact field set + per-table column mapping = design detail. Recommendation adopted.
- **Anchor**: 2026-07-17 · t2
- **Status**: live

### D-3 — the defect report is git-retained improvement fuel, an immutable event record

- **Question**: DQ-3 — does the defect report persist gitignored (host reads, never committed), or git-committed?
- **Answer**: **git-committed / retained**. "any generated output that can help the system improve must be retained and stored in git." A contract-gate bounce (a PRD/techspec issue) is improvement fuel; it is stored in git durably for the learning loops, not gitignored.
- **Reasoning**: reconciles the staleness concern I raised (#149) — the report is an **event record** (moment-bound: "at run T, contract C had defects D"), the review-report/incident family, **not a live projection**, so it does not stale (a fixed contract yields a *new* record; the old stays as history). Resolves the PRD's P-3 in its explicitly-flagged "retention-as-history" branch. The gate stays read-only (FR-11): the gate **emits**, the **host retains-to-git** — no gate mutation. Broader operator principle stated (**"any improvement-fuel output → git-retained"**) — cross-cutting, a candidate AP, surfaced for graduation, not patched here.
- **Anchor**: 2026-07-17 · t3
- **Status**: live

### D-4 — verdict schema: reserve the extension points, not the full semantic shape

- **Question**: DQ-4 — specify the semantic-verdict fields now, reserve just the slot, or defer entirely?
- **Answer**: **reserve the extension points**. The v1 closed + versioned verdict schema declares the verdict-class enum open to the future semantic classes (`ambiguous`, `contradictory`) and a known-but-not-evaluated `semantic` slot; v1 fully specifies its mechanical fields; the LLM follow-up (#419) defines the semantic slot's internals additively.
- **Reasoning**: honors FR-7 (fields reserved) + AP-5/6 (additive, no v1 break — the reserved slot is a *known* key, so it never trips unknown-key rejection) while **not guessing** the LLM verdict's internal shape before #419 designs it (reserving the full shape risks specifying it wrong). Recommendation adopted.
- **Anchor**: 2026-07-17 · t4
- **Status**: live

### D-5 — registration into docgate's registry (contract-scope marker) + plan-new precondition (additive amendment)

- **Question**: DQ-5 — how do the checks register and compose, and how is the planning-entry precondition wired into plan-new's frozen ledger?
- **Answer**: agreed. **Registration**: the two new checks (structural, compilability) self-register per-file into docgate's **existing** registry (DCR, F-4); each contract-relevant descriptor carries a `contract-scope` membership marker; the `docgate contract-gate` verb assembles the marked subset, runs it feature-scoped, aggregates into the verdict + report. **plan-new precondition**: an additive typed amendment (`add` op) on plan-new's frozen ledger — plan-new calls `docgate contract-gate <feature>` before drafting and refuses on any non-clean verdict.
- **Reasoning**: extends the DCR pattern (AP-27 — new check = new file + a membership flag, no shared-line edit) and the amendment model (NR-1 anticipated it). Contract-gate's feature-level checks run *before* plan-new's plan-level coverage (FR-26) + fog (FR-16) — complementary levels, no contradiction, no requirement hole to bounce (unlike the sibling's stale-fit P-1). Recommendation adopted.
- **Anchor**: 2026-07-17 · t5
- **Status**: live

## Alternatives

### ALT-1 — docgate-hosted: a contract-gate verb/mode composing docgate's checks

- **Shape**: contract-gate is a `docgate contract-gate <feature>` verb (feature-scoped) that composes the five checks — reusing check-citations (link), GenCoverage (coverage), the index's stale flags (currency), plus new structural + compilability checks self-registered into docgate's registry — and produces the feature-scoped verdict + one defect report. plan-new invokes it at planning entry; a docgate CI check runs the advisory continuous surface. The fold, source-map reader, registry, and machine faces are reused, not rebuilt.
- **Trade-offs**: maximal reuse (the checks *are* doc checks; zero duplication of the fold/readers — the PRD's §4.1-C bet); the checks live where doc checks belong. Costs: docgate gains a **feature-scoped, verdict-producing mode** distinct from its tree-wide defect-stream shape; a contract-gate bug can force a docgate release (coupling); docgate's identity stretches to "doc-PR gate *and* stage-6 contract gate" — but both are doc-system gates, so the stretch is small.
- **Principles**: AP-10 (reuse not rebuild) · AP-11 (one predicate, both triggers) · AP-27 (checks self-register) · AP-5/6 (structured verdict) honored; strains docgate's single-identity slightly (two gate roles, same domain).
- **Incidents**: the sibling's DQ-1 lesson (separate binary for a separate *domain*) — here the domain is the *same* (docs), so it argues *for* hosting (F-1); none further matched (registry checked 2026-07-17).
- **Verdict**: chosen — the checks are docgate's own machinery; a separate binary would duplicate the fold/readers or import docgate's internals anyway (D-1)
- **From**: DQ-1 · panel

### ALT-2 — a separate pinned `cmd/contractgate` binary (the codegate sibling shape)

- **Shape**: `internal/contractgate/` core + a thin `cmd/contractgate` main, shipped docgate-style (sha256-pinned static releases), its own check registry, reading the machine faces docgate produces. House-symmetric: docgate/plangate/codegate/**contractgate**.
- **Trade-offs**: clean identity/trigger separation (contract-gate is a pipeline *stage*, not the doc-PR gate); release-decoupled from docgate; the house-symmetry reads cleanly. But: it must re-read docgate's machine faces (fine) yet the *checks* (check-citations, GenCoverage) live inside docgate — a separate binary either **duplicates** them (drift — the exact §4.1-C failure) or imports docgate's `internal/` (coupling without the co-location benefit); one more pinned-binary acquisition surface.
- **Principles**: repo-layout pinning + house symmetry honored; **strains AP-10** (rebuild vs reuse — duplicates docgate's readers/checks) and the PRD §4.1-C reuse bet.
- **Incidents**: the lavish-bundling/duplication lessons (2026-06-25 lineage) argue against a second copy of shared machinery; none directly matched (registry checked 2026-07-17).
- **Verdict**: rejected — the clean-identity win is real but bought by duplicating or coupling-to docgate's check machinery, which the PRD explicitly refused (D-1)
- **From**: DQ-1 · panel

*(A plangate-hosted variant — contract-gate as a plangate verb because it fires at planning entry — was considered and set aside within ALT-1: contract-gate's checks are doc-domain, over the PRD/design machine faces, not plan-domain; the firing moment is who-invokes-when, not where the checks belong.)*

### ALT-3 — gitignored ephemeral defect artifact (the codegate-verdict mirror)

- **Shape**: one atomic structured report to a gitignored path (temp+rename), the host reads it, never git-committed; the durable signal is the EVT-2 defect events flowing to telemetry.
- **Trade-offs**: no staleness or clutter in git; regenerates cheaply; mirrors codegate's never-committed verdict. But the **report itself is not retained** as improvement fuel — only aggregate EVT-2 events survive, losing the full structured bounce record the learning loops could mine.
- **Principles**: codegate D-3 (don't commit a derivable) honored; **strains** the improvement-loop retention the authority wants (the fuel is discarded).
- **Incidents**: #149 derived-artifact-staleness cited as motivation — but that family is about *live projections*, and misapplies to an event record (see ALT-4).
- **Verdict**: rejected — discards improvement fuel; its motivating staleness concern misframes an event record as a live projection (D-3)
- **From**: DQ-3 · panel

### ALT-4 — git-committed immutable event record (the review-report/incident family)

- **Shape**: one structured dual-face report per bounce, git-committed as an **immutable, append-only event record** (dense-numbered or timestamped per bounce), the review-report/incident family; the gate **emits** read-only (FR-11), the **host retains-to-git**.
- **Trade-offs**: the full bounce record is retained as durable improvement fuel for the learning loops; immutable history, never overwritten. Costs: a commit mechanism per trigger (a design detail — G-1); more committed artifacts over time, but bounded (a bounce is the exception, not every gate run).
- **Principles**: the authority's improvement-fuel-retention principle (candidate AP) · AP-5/6 (structured) honored; FR-11 preserved (gate emits, host commits). The #149 staleness family does **not** apply — an event record is moment-bound history, not a live projection.
- **Incidents**: none matched — the #149 family is live-projection drift, not event records (registry checked 2026-07-17).
- **Verdict**: chosen — improvement-fuel retention (the authority's principle); the event-record framing sidesteps the staleness a live-projection derivable would suffer (D-3)
- **From**: DQ-3 · panel

## Seam map

- **S-1**: the contract-gate tool home — **opens** — behind it: a `docgate contract-gate <feature>` verb/mode composing docgate's registry + machine faces into a feature-scoped verdict + defect report (ALT-1/D-1) — feeds: the design's Scope ("a contract-gate mode on docgate") + Serves prd:contract-gate/FR-1, FR-10, NFR-4
- **S-2**: the fold's PRD machine face (source map) — **extends** — behind it: additive per-requirement typed fields (priority · has-fit-criterion · verify-method) projected by the fold from the already-parsed row cells by header-column name; a docgate fold/schema change (additive) — feeds: Serves prd:contract-gate/FR-3, FR-5 (D-2)
- **S-3**: the defect-report artifact class — **opens** — behind it: one structured dual-face report per bounce, git-committed as an immutable append-only event record (the review-report/incident family; dense-numbered or timestamped per bounce), emitted read-only by the gate and retained-to-git by the host — feeds: Serves prd:contract-gate/FR-8; §5.4 of the basis (D-3)
- **S-4**: the check-registry membership — **extends** — behind it: the two new checks self-register into docgate's registry (DCR); a `contract-scope` marker on the descriptor; the verb assembles the marked subset feature-scoped — feeds: Serves prd:contract-gate/FR-2..FR-6, NFR-5; AP-27 (D-5)
- **S-5**: the plan-new planning-entry precondition — **extends** — behind it: an additive typed amendment on plan-new's frozen ledger (`add` op: the precondition FR); plan-new calls the gate before drafting, refuses on non-clean — feeds: Serves prd:contract-gate/FR-1, NR-1 (D-5)

## Facts

- **F-1**: the sibling gate's design (design-brainstorm:implementation-verification-tech-design#D-1) chose a *separate* binary (codegate) explicitly because code-verification is a **different domain** from docs (rung checks would join a documentation-check registry — a semantic mismatch); contract-gate's five checks are doc-system checks, so the same reasoning argues the opposite way — source: design-brainstorm:implementation-verification-tech-design#D-1 · #ALT-2
- **F-2**: three of the five checks are already docgate machinery — **link resolution IS check-citations** (a live docgate check), **inverse coverage IS GenCoverage** (a live docgate on-read computation), **currency IS the index's stale-serves/stale-builds-on** flags — source: internal/docgate/gate/reg_check_citations.go · internal/docgate/techdesigns/index.go
- **F-3**: the fold **already tokenizes every requirement row** into `ledger.Unit.Cells` (with the header columns in `Table.Cols`); the machine face `fold.UnitState` exposes only lineage fields (status/introduced_by/supersedes/superseded_by/source_location/current_location). So DQ-2's per-requirement enrichment is *projecting already-parsed cells as typed fields*, not new parsing — the PRD R1 budget risk is materially smaller than feared — source: internal/docgate/ledger/ledger.go:31 · internal/docgate/fold/fold.go:18
- **F-4**: the DCR pattern — each check self-registers per-file (init()) into a per-gate registry, assembled deterministically by verb; docgate and plangate each carry one; §3.8's audit rule is "distribute what grows per feature, single-source what is pinned" — source: docs/tech-design/distributed-check-registration.md §2, §3.8
- **F-5**: nothing in the design index Serves prd:contract-gate — open ground, no covering or conflicting live design — source: docs/tech-design/index.md (grep 2026-07-17)
- **F-6**: the tool family is docgate (docs) · plangate (plans) · codegate (code); a separate contract binary would be the fourth — source: repo cmd/ · design-brainstorm:implementation-verification-tech-design#D-8

## Fog

- **G-1** (non-blocking — blocks: the authored design's commit-mechanism detail): who commits the emitted report and when, preserving the gate's read-only FR-11. The continuous contract-doc-PR surface has a natural commit home (the report rides the PR, review-report-like); the planning-entry bounce (plan-new, pre-approval, no PR) needs a commit home the design must name — direction is set (git-retain, D-3), only the mechanism is open.

## Out of scope

- **O-1**: the **semantic-verdict internals** — the shape of the `semantic` slot, the ambiguous/contradictory checks, the analytical outputs — are out of this design; the LLM follow-up (#419) designs them additively into the reserved schema slot (from D-4). *(The basis's NG-1 no-success-artifact and NG-2 LLM-layer boundaries are inherited, not reopened here.)*

## Candidate glossary terms

none *(pending — a docgate-hosted vs separate-binary ruling may coin the tool name)*

## Parked product gaps

none

## Gate

| Field | Value |
|---|---|
| **Verdict** | ready-for-design |
| **Date** | 2026-07-17 |
| **Authority** | @mdarabi |
| **Covering design** | — |
| **Coverage check** | pass — core four covered (D-1..D-5 direction · ALT-1..4 · S-1..S-5 Serves/Scope · O-1 boundaries), zero blocking fog |
| **Amend route** | — |
