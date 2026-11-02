# Read to rule (§0) — the section contract, its ops, and the federated capture line — technical design

| Field | Value |
|---|---|
| **Status** | Draft — design review pending *(→ Approved before merge; merge is the freezing event; "superseded" is computed by the index, never written)* |
| **Owner** | @mdarabi |
| **Approver(s)** | @mdarabi (operator) |
| **Date** | 2026-07-14 |
| **Relevant-when** | implementing or changing the §0 read-to-rule template contract, the prd-new/prd-amend §0 steps, the gate capture line, or its citations into the review-feedback ledger; triaging a §0 that fails conformance, a stale §0 route, or an unresolvable capture-line citation; superseding the §0 mechanics |
| **Scope** | The §0 section contract in prd-template's next version (placement, blocks, constants, tier rule, the capture-line convention as gate-table guidance), the prd-new/prd-amend §0 steps, the amendment-op semantics for §0, and the capture line's format, home, and federation. **Not**: any docgate/fold change, any review-feedback-ledger schema change, the reviewers' §0-skip adoption (their ledgers — prd:read-to-rule Q2), the pre-§0-ledger introduction path (deferred, §3.6), render affordances (prd:read-to-rule WR-3). |
| **Tracking** | filed with the introducing PR |
| **Builds-on** | prd-amendment-model · review-feedback-capture · tech-design-conventions |
| **Supersedes** | — |
| **Serves** | prd:read-to-rule/FR-1 · prd:read-to-rule/FR-2 · prd:read-to-rule/FR-3 · prd:read-to-rule/FR-4 · prd:read-to-rule/FR-5 · prd:read-to-rule/FR-6 · prd:read-to-rule/FR-7 · prd:read-to-rule/FR-8 · prd:read-to-rule/FR-9 · prd:read-to-rule/UB-1 · prd:read-to-rule/UB-2 · prd:read-to-rule/UB-3 · prd:read-to-rule/UB-4 · prd:read-to-rule/NR-1 · prd:read-to-rule/NR-2 · prd:read-to-rule/NR-3 · prd:read-to-rule/NFR-1 · prd:read-to-rule/NFR-2 · prd:read-to-rule/NFR-3 |
| **Security/privacy impact** | none — template guidance and a sign-off-line convention over repo-internal markdown; no new data class, egress, or surface (§3.5) |
| **Template version** | tech-design-template/v1 |

## 1. Problem

The merged read-to-rule PRD binds *what* §0 is — a human-only, informative, one-screen routing section, authored at synthesis, maintained by amendments, measured by a capture line — and parks the *how* in its Q1: the exact block and skip forms, the cap and bounds, tier applicability, the capture-line shape and home, and how typed ops address a section outside the ID namespaces. The design brainstorm resolved the two real questions (`design-brainstorm:read-to-rule-tech-design#D-1..D-2`); the authority's altitude ruling there is binding on this document's size: §0 is an ordinary section, so the design reuses existing mechanisms and states constants — it builds nothing.

## 2. Decision

**Decision:** §0 ships as **template guidance plus two existing mechanisms**: (1) the section contract lives entirely in prd-template's next version — placement before §1, four blocks with stated honest-zero forms, a hard cap, a tier rule, the ranking rule — tunable only by template PR (`design-brainstorm:read-to-rule-tech-design#D-2`); (2) amendments touch §0 exactly as they touch any unanchored prose — **wholesale section-supersede** under the amendment model's existing grammar, no sub-anchors, no fold or docgate change (`#D-2`, F-1/F-2 of the record); (3) the capture line is a **federated sign-off-surface line**: hand-recorded at each §0 gate, its pinned format citing the review-feedback ledger's entry anchors one-directionally, with graduation to ledger-native explicitly routed through that feature's own channel if the checkpoint proves the bet (`#D-1`, ALT-3).

The reasoning: every constant sits on a surface that already exists (the template, the op grammar, the gate tables, the citation-grammar family), so the implementation appetite stays exactly what the PRD priced — one template version (carrying both the §0 contract and the capture-line convention) and two additive guide edits.

## 3. The design

### 3.1 The §0 template contract

**Decision:** prd-template's next version defines §0 as follows — these are the constants FR-1/FR-2/FR-4/FR-5 and NFR-1 bind to:

- **Placement and heading** — the first numbered section, before §1: `## 0. Read to rule — the human reviewer's section (~N min)`, where `~N min` is the authored reading-budget hint.
- **The normativity notice** — the section opens with one italic line: *the body is the contract; this section only routes attention — on any conflict the body governs; machines ignore it.*
- **Four blocks, in order** — **Judge these** (the ranked calls: a table of up to 7 rows, each carrying the call · the alternative it displaced · what disagreeing means, ordered by irreversibility × blast radius; healthy instances typically carry 3–7, but the bound is the cap, and zero is legal via the honest-zero form); **Sanity-check the numbers**; **The author's open questions**; **Skip the rest** (the informed-skip map: each omitted content class with the mechanism that backstops it).
- **Honest zero** — an empty block reads `none — <one-line assertion>` (e.g. *none — the record parked no fog*); an absent block is nonconformant, an asserted-empty one is information.
- **The cap** — §0 fits one screen: at most 40 markdown source lines of the section as it stands in `effective.md`, heading through the skip map's last line — a count any two inspectors compute identically; ranked calls at most 7. Over-cap is nonconformant (the basis's UB-3).
- **Tier rule** — §0 is required at T2/T3; a T1 one-pager may skip it with the template's standard `Skipped: <reason>` line. A same-template document that skips §0 holds no §0 gates — its reviews are outside the capture convention (the basis's window definition).
- **Anchoring** — every routed item names its body anchor (§/row id); anchors resolve within the same document (the basis's FR-3).

### 3.2 Boundaries and non-goals

- **Not solved here** — the reviewers' §0-skip adoption (their ledgers, prd:read-to-rule Q2); the self-optimization consumers that might one day read the capture data.
- **Owned by a future design** — the pre-§0-ledger introduction path (§3.6's deferred line); the staleness lint (prd:read-to-rule WR-2); §0 render affordances (WR-3).
- **Implementation must not infer** — that §0 needs IDs, fold logic, or docgate awareness (it is prose to every tool); that the capture line implies a review-feedback schema change (it never does — citations are one-directional).
- **Must remain unchanged** — docgate behavior and verbs (the basis's NR-1; template-PR accompaniments excepted per its fit), the reviewers' surfaces (NR-2), §1's contract (NR-3), the review-feedback ledger's schema and files (its own NG-5, mirrored).

### 3.3 Invariants

- **DINV-1** — every live §0 item resolves to a live anchor in its own document; a §0 with a dead route is a defect in §0, never in the body.
- **DINV-2** — §0 carries no sub-anchors and is amended only wholesale at section granularity; no downstream artifact ever cites into §0.
- **DINV-3** — a capture-line citation names a stable review-feedback entry anchor (`review-feedback:<subject-stem>#<entry-anchor>`), never a working-branch ref (I2's lesson, the counterpart design's own rule).

### 3.4 Observability

none — no runtime surface. The convention's own observation channel is the capture line itself: minutes, in-§0/outside-§0 counts, and the sampled-read marker ride the sign-off records, hand-computable into M1/M2 exactly as the basis's §5.7 states (artifact-borne, no telemetry).

### 3.5 Security & privacy

none — template guidance and a sign-off-line convention over repo-internal markdown; no new data class, no egress, no identity or permission surface (agrees with the header row).

### 3.6 Authoring and amendment mechanics — §0 is an ordinary section

**Decision:** the synthesis flow writes §0 at base authoring — one additive prd-new step: §0 is composed from the document's own content, every routed item anchored per DINV-1 and §3.1's contract, before the fold and the review moments. Maintenance is the existing grammar, unchanged: an amendment whose ops change content §0 routes to carries `### op: supersede §0` with the complete replacement section body — the same wholesale-at-section-granularity rule every unanchored section already follows; the fold projects it with zero new logic. **Blast radius states the computed truth:** under the unchanged grammar any amendment carrying a §0 supersede computes `contract-changing` (a section supersede's computed tier; declared never understates computed), so the amendment declares `contract-changing` with the §0-only, informative-content scope named in the declared reason — the top-tier review bar this implies is accepted as the price of zero docgate change, and if practice shows it burdensome for pure §0 refreshes, an informative-section tier carve-out is a docgate-channel follow-up, deliberately not this feature's (the basis's NG-4). **Introduction into a pre-§0 ledger is deferred until first exercised** (the basis's NG-6 may-adopt posture).

*(For the future owner of that deferred path, non-binding: under today's implemented grammar this is a grammar-and-fold evolution through the amendment model's own channel — the implemented `add` accepts table payloads only, and section numbers are append-only next-fresh anchors, both of which a §0-before-§1 introduction violates — not a one-line code follow-up.)*

### 3.7 The capture line

**Decision:** one physical line per review event (the basis's "§0 gate" counting unit), recorded where the signature lives — the document's gate-table row during the draft phase, the approving review otherwise — in the pinned form, whose home is prd-template's gate-table guidance (the basis's NFR-2: tunable by a template PR alone; the two guides point at it, never restate it):

```
capture: <N>m [· sampled: full read <M>m] · items: [in|out] <entry-citation or gist> · … — or: items: none
```

- `<N>m` is the §0 pass plus any drill-ins — the only quantity M2's median consumes; `<M>m` is the sampled full read's own time (the baseline datum, outside M2).
- Every feedback item carries its routing marker (`[in]` / `[out]`) and either its `review-feedback:<subject-stem>#<entry-anchor>` citation (verbatim per that design's grammar — DINV-3) or, for an uncaptured item (e.g. a verbal ruling), a short gist — so the basis's per-item record (its §5.4 entity, EVT-1, EVT-2) reconstructs from the line alone. A sampled gate's extra findings are recorded as `[out]` items on that same line.
- Citations resolve against entries already in the tree — the counterpart's live resolution check gate-enforces every `review-feedback:` citation in tracked files, so a draft-phase line cites only committed entries; lines living in PR reviews sit outside that check's reach (R1's real residual).
- **Fallbacks, stated where the reviewer reads them:** a pre-§0 document's gates sign exactly as today — outside this convention entirely (FR-9); a §0 item whose route went stale falls back to the body (which governs), and the miss is recorded as an `[out]` item (UB-2).
- **Staged gates:** any review event that signs at least one gate table on a §0-bearing document is a §0 gate — it carries the line or stands as a recorded lineless miss (UB-4); at a problem-only review the reviewer takes §0's problem-space rows and defers solution-space rows to the later gate (deferred rows are not feedback items), so the template's read-nothing-below-§3 rule stands without narrowing the counting unit.

Graduation path, explicit: if the ten-gate checkpoint proves the bet, a later amendment through the review-feedback feature's own channel may absorb the line's fields as ledger-native signals; until then the line is the record.

## 4. Alternatives considered

| Alternative | Why rejected |
|---|---|
| Standalone sign-off line, no citations (`design-brainstorm:read-to-rule-tech-design#ALT-1`) | the same review event lands twice — once in the line, once in the review-feedback ledger — with no join; the classification never reaches a machine face (AP-11 strained) |
| Ledger-native capture in review-feedback (`#ALT-2`) | evolves another feature's frozen schema through a channel the basis's appetite excluded; per-gate minutes have no per-comment home; couples the template launch to their implementation timing (I1-class inversion) |
| Federated line (`#ALT-3`) | **chosen** — `#D-1` |
| Sub-anchored §0 rows (per-row IDs, targeted ops) | a new ID namespace and mandatory docgate schema/fold work for informative content — the complexity the authority's `#D-2` ruling refused; wholesale supersede already suffices |
| Fold-generated §0 refresh | unlawful — re-litigates the frozen operator ruling that §0 is authored, never computed (brainstorm:gate-attention#D-3) |

## 5. Risks

| ID | Risk | Impact | Mitigation | Verification / evidence |
|---|---|---|---|---|
| R1 | citation discipline decays on the surfaces the live check cannot reach — capture lines recorded in PR reviews (amendment gates) cite anchors that rot or never existed | the federation join quietly degrades toward ALT-1 on exactly those gates | DINV-3 (stable entry anchors only); draft-phase lines are already gate-enforced by the counterpart's live repo-wide resolution check (cite only committed entries — §3.7); extending reach to review-side lines is the counterpart's check pattern, adoptable if decay is observed | sampled-gate inspection of review-side citations during the checkpoint window |
| R2 | the fold's section handling surprises on the first real §0 maintenance op (anchor match or tier computation) | FR-6's op path errors or bounces at first use | §0 maintenance uses the same `supersede <section anchor>` path as any section, declaring the computed `contract-changing` tier (§3.6); verified on a fixture ledger before the guide edit ships | an implementation-time fixture: a toy ledger with a §0 base + one §0-supersede amendment declaring `contract-changing`, folding green with unmodified docgate |
| R3 | the constants (cap 40 / calls 7 / tier rule) prove mis-tuned in practice | §0 bloats or starves | all constants live in template guidance, retunable by one template PR (basis NFR-2); the ship rule's recalibration path names the mechanism | the checkpoint's M1 misses + G3 rubric verdicts |

## 6. Acceptance

| ID | Acceptance criterion | Verify | Required evidence | Traces to |
|---|---|---|---|---|
| A-1 | prd-template's next version carries the full §3.1 contract — placement, notice, four blocks with honest-zero forms, cap 40 source lines / calls ≤ 7, tier rule, ranking rule, budget hint — plus the §3.7 capture-line convention as gate-table guidance | Inspection | the template version diff | prd:read-to-rule/FR-1 · FR-2 · FR-4 · FR-5 · FR-7 · NFR-1 · NFR-2 |
| A-2 | prd-new's guide gains one additive step: synthesize §0 per §3.6's authoring mechanic (from the document's own content, every item anchored — DINV-1) | Inspection | the guide diff | prd:read-to-rule/FR-3 · §3.6 |
| A-3 | prd-amend's guide gains one additive step: ops touching routed content carry `op: supersede §0` with the full replacement body, the amendment declaring the computed `contract-changing` tier with the §0-only scope in the reason (DINV-2, §3.6) | Inspection | the guide diff | prd:read-to-rule/FR-6 · §3.6 |
| A-4 | the §3.7 convention — the pinned line form, per-item routing markers and citations, the `<M>m` sampled slot, both fallbacks (pre-§0, stale-route), the staged-gate rule, and the committed-entries sequencing note — lands verbatim-class in prd-template's gate-table guidance, with the guides pointing at it | Inspection | the template version diff (guidance section) | prd:read-to-rule/FR-7 · FR-8 · FR-9 · UB-2 · UB-4 · NFR-2 |
| A-5 | a fixture ledger with a §0 base and one §0-supersede amendment declaring `contract-changing` folds green with unmodified docgate | Test | the fixture run | prd:read-to-rule/NR-1 · #DINV-2 · R2 |
| A-6 | zero diffs under docgate behavior/verbs, the reviewer surfaces, and §1's template contract (template-PR accompaniments excepted) | Inspection | the implementation PR's diff scope | prd:read-to-rule/NR-1 · NR-2 · NR-3 |

## 7. Open questions

| ID | Question | Owner | Blocks | Status | Resolution |
|---|---|---|---|---|---|
| Q1 | none — the deferred paths (§3.6 introduction, the staleness lint, graduation) are boundaries with named owners, not open questions | — | — | — | — |

## Principles

- **AP-11 — honored.** One review event, one measurement line, one comment record — joined by citation instead of duplicated or merged.
- **AP-19 — honored.** The convention's recurring costs (the sampled full read, the hand-recorded line) are stated in the basis and carried here; nothing recurs silently.
- **AP-21 — honored.** The in-§0/outside-§0 classification and minutes freeze at signing; nothing is re-derived later.
- **AP-26 / I2 — honored.** Capture-line citations name stable entry anchors, never working-branch refs.
- **RW-5 — honored.** One independently-revisable decision: the §0 contract and its capture line; the reviewers' skip, the introduction path, and any ledger-native graduation are separate documents when their day comes.
- **Deviation: AP-10** — the capture line's presence and format stay convention, not code: no lint enforces them in this feature. Its citation field, by contrast, is already gate-enforced wherever the line lands in a tracked file — the counterpart's live repo-wide resolution check reds any unresolvable `review-feedback:` citation — so the deviation's true residual is presence/format everywhere and citations on PR-review-side lines (R1). Reason: the basis's appetite and solo-operator scale; the named upgrade paths are extending the counterpart's check pattern (R1) and the basis's WR-2 staleness lint.
