# Gate attention — brainstorm record

| Field | Value |
|---|---|
| **In one line** | A presentation form for the human gate moments — capture most of the reviewer's valuable feedback at a fraction of full-document reading time, treating human attention as the pipeline's scarce resource. |
| **Relevant-when** | brainstorming or contracting how humans review authored documents at the gates (problem/solution/launch, amendment re-sign); optimizing operator review time, attention routing, briefs, or summaries; checking prior rulings on gate presentation or review-time trade-offs |
| **Idea** | "We are creating these rich documents, such as PRD, TechSpec, but still it takes a long time for an engineer or someone to read through and make sure the main things are clear. We should keep those documents as is — it's not a problem with the document. Can we come up with a way of presentation that is much simpler for a human reader to understand the main pieces of that document and where they should put their focus? As the number of agents in the world increases, human attention becomes the scarcity part of any process. Rather than reading the whole document, which might take a long time, is there a way we can optimize that review process — reducing the amount of time but still getting the most value out of the review? We do not expect them to provide a hundred percent accurate or valuable input — we are going to start that trade-off: by reducing their review time, can we capture at least eighty percent of their valuable feedback?" (operator, 2026-07-11, verbatim) · "We need to do the same improvements for the techspec later." (operator, 2026-07-11) |
| **Operator** | @mdarabi |
| **Date(s)** | 2026-07-11 (idea) · 2026-07-13 (session) |
| **Status** | gated: ready-for-prd |
| **Tracking** | — |
| **Feeds** | docs/prds/read-to-rule/ |
| **Prior records** | brainstorm:authoring-reviewers |
| **Template version** | brainstorm-record/v1 |

## Orientation

| Source | Why loaded |
|---|---|
| .pyxis/pipeline.md | the operational gate map — human gates are stages 2, 5, 9 plus contract authoring; advisory-period note: human approval + merge is what actually gates 2/5 today |
| docs/product-vision.md (targeted) | the governing layer this must fit — the autonomy ladder keeps human gates for non-low-risk classes well up the levels |
| docs/prds/README.md | the north star: the engineer touches a feature exactly twice (author the contract · accept the result); design-share-of-engineer-time is a tracked metric |
| docs/prds/prd-review/effective.md | Relevant-when match — P2's pain ("reviewing everything or trusting blindly are the only modes"); flow F4 optional overlay; WR-1; the DecisionBrief; the M1/M2 measurement pattern |
| docs/reviews/argus-000.md | a real review report — the machine floor a gate presentation could lean on instead of re-showing |
| docs/brainstorms/authoring-reviewers.md | the prior record feeding both reviewers — D-1 (no-human world; human review = optional overlay), D-3 (briefs as work items), D-4 (one class per feature), D-8 (pre-digested briefs for the authority); the rejected "pre-gate analyst" framing |
| docs/wiki/glossary.md | term check: "brief" is taken by prd-review's DecisionBrief (escalation payload); gates, appetite, routing surface, fold/projection vocabulary |

**Overlaps surfaced:**

- `prd-review` flow F4 (the optional human overlay) — renders the *machine's findings* onto the review surface; nothing compresses or routes the human's own read of the document. Adjacent, not covering.
- `prd-review` WR-1 (verdict substitutes the human gate, autonomy-policy era) — that is *removing* the gate; this idea *cheapens* the gate while it exists. Complementary; the O-4 boundary (approve is data beside the gates, never a signature) stays honored.
- `brainstorm:authoring-reviewers` framings — the rejected **pre-gate analyst** carried this exact thesis ("make the human gate cheap; the report is the artifact"); it was rejected as the *reviewer's identity* (D-2 chose the unmanned protocol), not refused as a feature. The ground is open.
- `brainstorm:authoring-reviewers#D-8` + the DecisionBrief (prd-review §5.4) — "one pre-digested brief at a time" exists for *escalation conflicts only*; generalizing pre-digestion to the whole gate moment is unclaimed. Term collision ("brief") to resolve.
- prd-template §1 one-pager + the three gate tables (problem §2.6 · solution §4.4 · launch §6.6) — authored in-document compression exists but is static, uniformly weighted, not interrogative, and amendment-blind.
- `/doc-review` (PLAN-3/BENCH-4 lineage) — the rendered annotate→apply surface: a natural delivery vehicle, carries no judgment or compression of its own.
- docgate fold/projection — the generated-derived-artifact precedent (`effective.md`): mechanical kin for any "review projection".

None covers the idea; proceeding.

## Framings

- **The gate brief — a generated review projection** — the feature is a new *derived artifact class*: at each human gate moment, a one-screen, attention-ranked, interrogative projection of the document — ratify/veto the judgment calls (decisions, refusals, appetite, numbers), answer the author's low-confidence questions, everything anchor-linked into the full doc, the not-surfaced remainder named with its machine backstop (informed skipping), dispositions recorded as data. Kin to `effective.md`: derived, never authored. Opens: delta briefs at amendments; the design-class sibling; brief-residual measurement (the M2 pattern inverted).
- **The attention budget — the gate becomes a driven session** — the feature is a *protocol*, not a document: each gate declares an attention appetite (minutes), and an agent walks the operator through the ranked decision surface live — one item at a time, recommendation attached (the brainstorm interview pattern applied to review), rulings recorded as they land. Presentation is whatever the session renders; the budget is the contract. Opens: measured attention-cost per feature; the budget as an autonomy-ladder dial (shrinks as trust grows).
- **The guided overlay — a render mode of the existing surface** — no new artifact: `/doc-review` renders the *full document* routed — machine-backstopped sections dimmed (docgate floor, reviewer-verified coherence), judgment sections highlighted (decision log, non-goals, appetite, targets), the reviewer's verdict inline. The human still reads the doc, but steered; smallest footprint, no new schema. Opens: incremental render work; weakest on "don't read the whole file".

**Destination:** the *human-only section* — the gate presentation is a section at the top of the PRD itself, "nothing new": no new artifact class, no session protocol, no render mode (D-1). The rejected containers' content — ranked judgment calls, interrogative items, informed skipping — becomes candidate content *for the section*, not a separate thing.

## Decisions

### D-1 — the framing: a human-only section inside the PRD

- **Question**: which framing — a generated gate-brief artifact (recommended), an attention-budget driven session, or a guided-overlay render mode?
- **Answer**: none of the three — "it should just be a section at the top of the PRD (like a human-only section), nothing new." The presentation lives inside the document itself.
- **Reasoning**: operator's — zero new surfaces; the document carries its own review instrument where every reader already is. Simpler than all three offered containers; their content survives as candidates for the section's design.
- **Anchor**: 2026-07-13 · t1
- **Status**: live

### D-2 — the section's genre: a review instrument, distinct from §1

- **Question**: what is the section's job relative to §1's one-pager — a distinct review instrument (read to rule), an upgraded §1 doing both jobs, or a pure focus map with no solicited answers?
- **Answer**: the review instrument, as recommended — §1 stays the summary ("what is this?"); the new section is interrogative and ranked ("what must I judge?"): judgment calls ranked by irreversibility × blast radius, the author's own low-confidence questions, the numbers to sanity-check, an informed-skip map naming what is machine-backstopped.
- **Reasoning**: a summary compresses uniformly — exactly what loses the decision points; read-to-understand and read-to-rule are different genres and merging them muddies both (recommendation adopted).
- **Anchor**: 2026-07-13 · t2
- **Status**: live

### D-3 — authorship and currency: authored at synthesis, maintained by amendments

- **Question**: who writes the section and what keeps it current — authored by prd-new and maintained by every amendment that touches a ranked call (recommended), fold-generated into the projection only, or a synthesis-time snapshot never maintained?
- **Answer**: authored + maintained, as recommended — prd-new synthesizes it with the 000-base; an amendment touching a ranked call updates the section in the same typed op set; the existing fold keeps `effective.md` current for free.
- **Reasoning**: the ranking is judgment (irreversibility × blast radius) — a deterministic fold can verify but never author it; a snapshot rots and misroutes the exact attention it exists to protect. Staleness guards are mechanical and parked (P-2); the reviewer's honesty clause belongs to prd-review's own evolution channel, not this feature.
- **Anchor**: 2026-07-13 · t3
- **Status**: live

### D-4 — "human-only" semantics: informative, never normative

- **Question**: what may downstream consumers do with the section — informative-only (binds nothing, body governs on conflict; recommended), semi-binding gate scope (the signature attests only the listed items), or normative contract text?
- **Answer**: informative only, as recommended — the section binds nothing and no downstream consumer may cite or obey it; on any conflict the body governs; the stage-6 compiler and traceability skip it; workers and stage skills treat it as not-there; the gate signature stays over the whole document.
- **Reasoning**: a routing error must never become a contract error — a wrong section can misdirect minutes, never obligations; keeps the "nothing new" promise (no new authority class, no duplicated contract statements). The wiki's descriptive-authority pattern, applied inside the document.
- **Anchor**: 2026-07-13 · t4
- **Status**: live

### D-5 — the bet: §0-capture ≥ 80% and median gate time ≤ 10 minutes, at ten gates

- **Question**: what makes "80% of the valuable feedback at reduced time" falsifiable?
- **Answer**: as recommended — at a ten-gate checkpoint: (1) §0-capture rate ≥ 80% — of gate feedback items that led to a change or ruling, the share routed by the section rather than found beyond it; (2) median gate-review time ≤ 10 minutes on an argus-scale PRD (30–45 min full read today). Every ~5th gate is a deliberate full read after the §0 pass, recording what extra surfaced — the residual's denominator. Capture is one line per gate on the sign-off: minutes + each item marked in-§0 / outside-§0. Measurements ride the artifacts; no telemetry.
- **Reasoning**: both halves of the operator's trade-off measured; misses name the extraction rule to fix; the sampled full read mirrors the sampled-audit move already ruled for Argus's acceptance; zero-infrastructure instrumentation per the standing D-11 pattern.
- **Anchor**: 2026-07-13 · t5
- **Status**: live

### D-6 — class scope: PRDs only; the design-class sibling is its own feature

- **Question**: PRDs only with the tech-design/techspec treatment as a follow-up feature citing this record, or both document classes in one feature now?
- **Answer**: PRDs only, as recommended — the template change is exactly one file (prd-template v1→v2); the tech-design sibling runs its own short session citing this record's shared rulings. The operator's standing intent ("we need to do the same improvements for the techspec later", 2026-07-11) lands as the waiting-room row, not in-scope work.
- **Reasoning**: the one-class-per-feature posture (authoring-reviewers#D-4) — the design class's judgment surface differs (seams, ALT ledgers, binding Decision callouts, DINV rows), so one selection rule spanning both goes vague; mirrored by pattern beats coupled by artifact.
- **Anchor**: 2026-07-13 · t6
- **Status**: live

### D-7 — the segment: the human reviewer only; machine reviews ignore the section

- **Question**: whom does the section serve — and refuse? (offered: gate-signer only with contributors as free riders, vs signer + contributor as designed-for)
- **Answer**: "this segment is only for human-reviewer. machine reviews should ignore it and focus on whole document." P1 = the human reviewer at the gates (operator today, any write-access human later). Machine reviewers — the unmanned review protocols included — ignore the section entirely and review the full document at full rigor.
- **Reasoning**: compression exists for the scarce resource (human attention); machine attention is cheap and stays exhaustive — the section must never become a machine shortcut. Extends D-4's consumer boundary to reviewers explicitly. Consequence: no substance reviewer watches §0's honesty, so the mechanical staleness guards (P-2) and D-5's capture metric are its only quality loops. The authoring flow writing it (D-3) is authorship, not consumption.
- **Anchor**: 2026-07-13 · t7
- **Status**: live

### D-8 — appetite: small, no retrofit

- **Question**: what is this feature worth?
- **Answer**: small, as recommended — prd-template v1→v2 (the section's contract: placement, the four blocks — ranked calls, numbers, author's questions, skip map — selection rule as template guidance); two additive guide edits (prd-new synthesizes, prd-amend maintains per D-3); the per-gate capture line riding the existing sign-offs (D-5); ≤ 2 human review rounds on this ledger. Excluded: docgate behavior changes (P-2 waits), reviewer changes (D-7 — prd-review's own channel), render/doc-review affordances, the design-class sibling (D-6), retrofit of merged ledgers — existing ledgers adopt the section at their next natural amendment; new 000-bases are born with it.
- **Reasoning**: the whole feature is one template bump + two additive guide steps; measurement rides existing artifacts (the standing zero-infrastructure pattern); retrofit is ~10 amendment PRs of pure section-authoring — deferred until the checkpoint proves the bet.
- **Anchor**: 2026-07-13 · t8
- **Status**: live

### D-9 — the name: "Read to rule", at §0

- **Question**: the section's name — the template heading and corpus term ("brief" is taken by the DecisionBrief; "summary" is wrong by D-2).
- **Answer**: "Read to rule", as recommended — heading `## 0. Read to rule — the human reviewer's section`; the §0 slot sits before the contract proper (free in instantiated documents, where the template's how-to-use §0 is deleted).
- **Reasoning**: names the genre and the reader's act in three words; contrasts self-evidently with §1's read-to-understand; honest about machines (they do nothing with it, D-7). Lands as glossary candidate T-1.
- **Anchor**: 2026-07-13 · t9
- **Status**: live

## Facts

- **F-1**: the human gates are pipeline stages 2, 5, and 9, plus contract authoring; during the reviewers' advisory period, human approval + merge is what actually gates stages 2 and 5 — source: .pyxis/pipeline.md
- **F-2**: pre-digestion for the authority already exists in exactly one shape — the DecisionBrief: "the single conflict, the options with risks, the evidence pre-digested, the one decision needed" — escalations only — source: docs/prds/prd-review/effective.md §5.4, FR-11
- **F-3**: the PRD's judgment surface is already structurally isolated by the template: decision log §7.3, non-goals/waiting room §3.4, alternatives §4.1, appetite (header), ship rule §3.3, risks §6.3, open questions §7.2 — plus an authored one-pager (§1) and three sign-off gates — source: docs/prds/prd-review/effective.md (prd-template/v1 instance)
- **F-4**: the doc-review loop is single-annotator, rendered, propose-and-confirm — a delivery surface with no judgment of its own — source: brainstorm:authoring-reviewers#F-1; .pyxis/skills/doc-review
- **F-5**: the north star bounds the engineer to two touches (author the contract · accept the result) and tracks design-share of engineer time — gate attention is the standing cost of the second touch — source: docs/prds/README.md (inherited north star)
- **F-6**: the M2 residual pattern (defects the optional human overlay finds after an approve verdict) is defined and rides the review artifacts — a residual-measurement precedent this idea's 80% trade-off could reuse — source: docs/prds/prd-review/effective.md §3.2, FR-14

## Fog

none

## Out of scope

- **O-1**: any normative status for the section — it binds nothing, is never cited by compiler/traceability/workers, and never redefines what the gate signature covers; on conflict the body governs (from D-4)
- **O-2**: any document class beyond PRDs — the tech-design/techspec treatment is its own follow-up feature citing this record (from D-6)
- **O-3**: machine consumption of the section in any form — reviewers routing or judging by it included; machine reviews ignore it and cover the whole document (from D-7)

## Candidate glossary terms

- **T-1**: read to rule (§0) — the PRD's human-only routing section: what the gate-signer must judge, ranked by irreversibility × blast radius, plus the author's open questions, the numbers, and the informed-skip map; informative never normative (on conflict the body governs); machines ignore it entirely · avoid: brief (the DecisionBrief is prd-review's escalation payload), summary (§1's job), review report (the unmanned reviewer's artifact)

## Parked technical constraints

- **P-1**: the generator's home (docgate verb vs authoring-skill step vs engine-era stage runner), the artifact schema, and the render/delivery vehicle (render-doc.mjs / the doc-review surface) — stage-3 matter, surfaced during orientation; narrowed by D-1/D-3: the "generator" is the authoring flow itself, so what remains for design is the section's template contract and any render affordances
- **P-2**: staleness guards for the section — mechanical checks (anchors resolve, cited IDs live; possibly "an op touching a ranked call must update the section" as a docgate lint) — stage-3/4 matter (from D-3)

## Gate

| Field | Value |
|---|---|
| **Verdict** | ready-for-prd |
| **Date** | 2026-07-13 |
| **Operator** | @mdarabi |
| **Covering source** | — |
| **Coverage check** | pass — all six inputs covered (§2.1←D-1/D-2 · §2.2←D-7 · §2.3←D-2/D-3/D-5 · §3.1←D-5 · §3.4←O-1..3 · appetite←D-8), zero blocking fog |
