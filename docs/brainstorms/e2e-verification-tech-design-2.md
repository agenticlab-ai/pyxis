# E2E verification — the harness becomes a machine — design-brainstorm record

| Field | Value |
|---|---|
| **In one line** | Stage-3 reopen for the e2e authoring line under PRD amendment 002 — how the e2e design doc carries a buildable harness (ten mechanical elements, existing-harness reuse, the decision-authority stand-in), how scenarios normalize into exclusively-owned artifacts, how the suite's cost is declared, and how the whole delta lands against two frozen, stale-serves designs. |
| **Relevant-when** | designing or changing the e2e harness contract (the ten mechanical elements, citing an existing harness, the decision-authority stand-in), the per-scenario normalization and exclusive artifact ownership, the declared suite cost, the buildability lens, or the conventions task; checking prior design rulings about how the amendment-002 delta supersedes `design:e2e-verification-artifacts` and `design:e2e-verification-skills` |
| **Basis** | `prd:e2e-verification` (merged ledger — 000-base + amendments 001 and 002, folded; 002 frozen on main #1038) |
| **Authority** | @mdarabi (operator) |
| **Date(s)** | 2026-08-06 |
| **Status** | gated: ready-for-design — 2026-08-06, operator @mdarabi *(after gating, the record changes only through the introducing PR's review loop — rulings added or superseded with review anchors — and freezes entirely at merge)* |
| **Tracking** | — *(the introducing PR, once filed)* |
| **Feeds** | `docs/tech-design/e2e-verification-artifacts-v2.md` · `docs/tech-design/e2e-verification-skills-v2.md` |
| **Prior records** | `design-brainstorm:e2e-verification-tech-design` (stage-3, gated ready-for-design, frozen on main #678) · `brainstorm:e2e-verification` (stage-1, gated ready-for-prd) |
| **Template version** | design-brainstorm-record/v1 |

## Orientation

| Source | Why loaded |
|---|---|
| docs/prds/e2e-verification/effective.md | the basis — merged ledger folded through amendment 002; the FR/UB/NFR/ABUSE/EVAL contract this reopen must serve |
| docs/prds/e2e-verification/002-harness-designed-buildable-and-cost-bounded.md | the delta itself — the five superseded Must rows, the four added FRs, UB-8/UB-9, NFR-5, EVAL-3, DOD-8, ABUSE-5, R6, and each op's stated reason |
| docs/brainstorms/e2e-verification-tech-design.md | the frozen predecessor — every `D/ALT/S/F/O/P-n` ruling this reopen must not re-ask (the class choice, the reuse boundaries, the two-design partition) |
| docs/tech-design/e2e-verification-artifacts.md | live design — the `e2e-design/v1` class (six sections, header, home, freeze), the report family instance, the plan reuse; `Serves` now cites superseded IDs |
| docs/tech-design/e2e-verification-skills.md | live design — the three skills' behavior (coverage derivation, live-design-set selection, the two lenses, the conditional harness task); `Serves` now cites superseded IDs |
| docs/tech-design/index.md | both e2e designs flagged **`stale-serves`**; `implement-now-e2e-tasks` builds on both and is flagged `stale-builds-on` |
| .pyxis/templates/e2e-design-template.md | the instantiated class surface — §Harness libraries is one free-prose paragraph today |
| internal/e2e/docclass/ | the built well-formedness checker — the closed section set, the route/coverage row rules, the mechanical half of DINV-1/2/3 |
| docs/e2e/plan-new/design.md · docs/e2e/tech-design-brainstorm/design.md | the two built `e2e-design/v1` instances — amendment 002's `Learned-from` build-results, and the concrete cases every question in this session is exampled against |
| docs/observability/incidents.md | I1–I6 — the panel's incident matching; I4 and I6 bite the supersession question directly |
| docs/wiki/glossary.md | the seam definition driving the panel rule, and the one-adapter-vs-two guard |
| docs/architecture-principles.md | AP/RW registry — the panel's principle citations |

**Overlaps surfaced:**

- **Two live designs already serve this basis** — `design:e2e-verification-artifacts` (the `e2e-design/v1` class) and `design:e2e-verification-skills` (the three skills' behavior). Neither is a `covered` candidate: both are **frozen at merge and both are flagged `stale-serves`** because amendment 002 superseded five of the Must rows they cite (FR-1, FR-6, FR-8, FR-16, FR-17). The delta is new ground; the question is how it lands against them, not whether it is already decided.
- **`design:implement-now-e2e-tasks` builds on both** and is flagged `stale-builds-on` — a third document in the blast radius of any supersession here.
- **The frozen predecessor record settles the class, the homes, and the reuse boundaries** (D-1 lightweight `e2e-design/v1`, D-11 the two-design partition, D-3/D-5 the reviewer/planner reuse, D-13 the report family instance, D-14 the live design set). None of those is reopened; this session sits strictly on the amendment-002 delta.
- **The two built e2e design docs are the evidence trail**, not a competing decision: both answered the class's `Harness libraries` section with a library preference, which is the defect amendment 002 exists to close.

## Design questions

- **DQ-1**: How does the amendment-002 delta land against two frozen designs that both now carry stale `Serves` — one combined superseding design, two parallel supersessions mirroring the existing artifacts/behavior partition, or a supersession plus a new standalone harness-contract design?
- **DQ-2**: What structural form carries FR-19's ten mechanical elements inside the `e2e-design` document class, and where does the boundary fall between what `internal/e2e/docclass` can check mechanically and what the buildability lens must judge?
- **DQ-3**: How does e2e-design determine that a reusable harness already exists in the repo (FR-20) — and, by the same or a different mechanism, how does e2e-plan determine the repo already carries a scenario built against it (FR-25)?
- **DQ-4**: Where do FR-21's per-scenario normalization fields (fixture · seams · the one exclusively-owned artifact) live in the doc, and how does e2e-plan consume them into FR-26's per-task exclusive ownership and UB-9's refusal?
- **DQ-5**: Where is FR-22's decision-authority stand-in specified — an element of the harness section, its own conditional section, or content inside the mock strategy — and what makes its five properties checkable?
- **DQ-6**: Where are FR-23's cost class and cadence declared, and in what unit does the harness state its per-run bound so NFR-5's "derive the scheduled-run cost from the doc alone" is actually satisfiable?
- **DQ-7**: Does the buildability lens (FR-24) join purely as data — a third lens prompt, calibration rows, and a seeded corpus family — and what does its arrival oblige in the frozen `e2e-review-report/v1` schema and the EVT-5 lens enum?
- **DQ-8**: The two existing `e2e-design/v1` instances predate the harness contract and would fail it — are they re-authored under the new class, grandfathered as v1 instances, or does one of them become the dogfood the new bar is proven on?

## Panel log

- **DQ-1**: no seam (documentation-partition, not a system interface) — skipped — how many successor documents carry the delta, and how they divide it, is a packaging choice opening no interface, port, or class; the predecessor ruled the identical question a non-seam and skipped on that ground (`F-7`). Three shapes were hand-authored and put to the authority; the authority adopted the recommendation (D-1). No panel was forced or waived.
- **DQ-2**: replaces — fired — the `e2e-design` document class is a seam (a document class, per the glossary's own list), and FR-19 replaces its harness section wholesale rather than extending it; the shape chosen also relocates the boundary between the hermetic checker and the model-driven lens, so parallel alternatives under orthogonal constraints are warranted. ALT-1 · ALT-2 · ALT-3.
- **DQ-3**: opens — fired — one candidate answer stands up a **harness registry**, a new shared data artifact and lookup surface that both e2e-design and e2e-plan would read; a question whose candidate set opens a new class is on a seam whether or not that candidate wins. ALT-4 · ALT-5 · ALT-6.
- **DQ-4**: extends — skipped — where the per-scenario build facts sit is internal structure of the class whose shape DQ-2 already settled; it opens no interface, port, or class of its own. Three placements were hand-authored and put to the authority.
- **DQ-5**: extends — skipped — the decision-authority stand-in is placed *inside the harness section* by `FR-22`'s own wording, so where exactly it sits is internal structure of the form DQ-2 settled; no interface, port, or class is opened. Three placements were hand-authored and put to the authority.
- **DQ-6**: extends — skipped — both halves sit inside the form DQ-2 settled and open no interface, port, or class. The placement half was answered together with DQ-4 (the cost class and cadence join the runs-table, D-5); the unit half was put separately, with four units hand-authored (D-7).
- **DQ-7**: extends — skipped — every candidate keeps the reviewer program's context pack input-only over admitted kinds and adds no interface, port, or class; it is a wiring question over machinery that already exists (the pack, the lens set, the checker). Its schema half is moot under D-2: the report schema and the event enum are rebuilt carrying three lenses from the start, so no supersession question arises. Three wirings were hand-authored and put to the authority.
- **DQ-8**: no seam (a disposition of existing instances, not an interface) — skipped — whether the two authored `e2e-design/v1` instances are re-authored, grandfathered, or become the dogfood opens no interface, port, or class; the authority settled it directly by ruling the whole existing line removed (D-2), so no alternatives were warranted.

## Decisions

### D-1 — the delta lands as two parallel supersessions, keeping today's artifacts/behavior split

- **Question**: how does amendment 002's delta land against `design:e2e-verification-artifacts` and `design:e2e-verification-skills` — both frozen at merge, both now citing superseded requirement rows: one combined superseding design, two parallel supersessions mirroring the existing partition, or two supersessions plus a third standalone harness-contract design?
- **Answer**: two parallel supersessions — a successor to the artifacts design and a successor to the skills design, each replacing its predecessor whole and keeping the same division of labor (the document class in one, the three skills' behavior in the other).
- **Reasoning**: authority adopted the recommendation — it preserves `#D-11`'s artifacts/behavior partition and RW-5's one-independently-revisable-decision-per-document property, so a later change to the doc format does not re-freeze the behavior half; a single combined document would collapse two decisions onto one clock, and a third standalone harness document would add a shared per-app-shape artifact nothing in the current contract asks for. Whole-document supersession is in any case the only legal repair for a frozen design's stale lineage rows (`F-3`, incident I6).
- **Anchor**: 2026-08-06 · partition turn
- **Status**: live

### D-2 — the existing e2e material is being removed; no backward compatibility is owed

- **Question**: do the successor designs have to stay compatible with the built e2e authoring line — its document class instances, its checker, its skills, and the two e2e design docs already authored under the old six-section contract?
- **Answer**: no. The authority was not satisfied with the results and is removing the existing e2e designs, plans and implementation outright; the successors owe no backward compatibility, no migration path, and no grandfathering of the two authored instances.
- **Reasoning**: the authority's ruling, unprompted — the built line's output is what amendment 002 was written against, and its two produced docs are the evidence of its defect (`F-5`); rebuilding is judged cheaper than carrying it. This removes the migration constraint from every remaining question in this session and settles DQ-8 outright: neither authored instance is re-authored or grandfathered.
- **Anchor**: 2026-08-06 · removal turn
- **Status**: live

### D-3 — the harness section is a closed ten-element form; the checker owns presence, the lens owns substance

- **Question**: what structural form carries the ten mechanical elements, and who verifies they are answered — the hermetic checker, the buildability lens, or the author's own declaration?
- **Answer**: ALT-1 — a closed ten-element form the checker validates for presence (an empty, placeholder, or library-name-only slot is a mechanical defect raised at authoring time), **and** the buildability lens additionally judges what is written in each slot. The authority was explicit that both halves matter: filling the slot is not passing it.
- **Reasoning**: the authority's ruling — a filled-in form proves nothing if the content is hollow, so the lens must read the slots rather than only trust the checker's green. The split keeps the structural half hermetic (a parser verdict that never depends on a model's mood, per I5) while the substance half stays where judgment belongs. `D-2`'s removal ruling erases ALT-1's main cost: with the checker rebuilt from scratch and no instances to migrate, a versioned closed vocabulary is free.
- **Anchor**: 2026-08-06 · harness-form turn
- **Status**: superseded by D-9 — at design review, D-3's "a library-name-only slot is a *mechanical* defect" was found to contradict the same design's claim that the checker never reads an answer for meaning; refined by D-9, which keeps the split and narrows the mechanical half to a structural shape floor

### D-9 — the checker's half of the split is presence *and shape*; naming-a-library-only is caught by a shape floor, not by comprehension

- **Question**: D-3 put "an empty, placeholder, or library-name-only slot" on the checker's side of the split, but deciding that an answer *is* a library name standing in for mechanics means reading it for meaning — which the checker cannot do. Where does the line actually fall?
- **Answer**: the checker decides **presence and shape** — that a slot exists, is answered, is not a placeholder, and clears a stated structural floor (prose was written where mechanics were owed, rather than a bare token run). The crudest library-name case is caught there at zero judgment cost. Whether an answer that clears the floor genuinely specifies a machine is the buildability lens's, as D-3 always intended for substance.
- **Reasoning**: review finding (`review:e2e-verification-artifacts-v2#FND-1`, which rides the design PR) — D-3's split is right and unchanged; only its boundary was drawn one step too generously toward the checker, leaving the design asserting two rules an implementer could not both build. The refinement preserves the authority's ruling in full: filling a slot still is not passing it, and both halves still bite.
- **Anchor**: 2026-08-06 · review
- **Status**: live

### D-4 — existing-harness reuse is the skill's own judgment; no registry, no path rule

- **Question**: how does e2e-design determine that a harness able to drive this feature's primary interface already exists (`prd:e2e-verification/FR-20`), and how does e2e-plan determine the repo already carries a scenario built against it (`FR-25`) — the skill's judgment, a declared registry, or a path convention?
- **Answer**: ALT-4 — the skill reads the repo's existing test trees and judges for itself whether one can drive this feature's interface, citing it by path. No registry file, no fixed-path rule.
- **Reasoning**: authority adopted the recommendation — it adds no new shared surface, extends the standing ruling that app-shape choices are authored content rather than skill code (`design-brainstorm:e2e-verification-tech-design#D-9`), and works on any repository layout. The registry's mechanical certainty was judged not worth a shared file that must be kept true (AP-27 contention; the stale-shared-index failure mode of I6), and the path rule answers the wrong question — a directory existing is not a harness that fits.
- **Anchor**: 2026-08-06 · harness-reuse turn
- **Status**: live

### D-5 — the five new per-scenario facts land in two scenario-keyed tables, cross-checked for the same scenario set

- **Question**: where do `FR-21`'s build facts (fixture · seams · the one exclusively-owned artifact) and `FR-23`'s economics facts (cost class · cadence) sit — one wide table per scenario, two cohesive tables, or fields under each scenario's own description?
- **Answer**: two tables, both keyed by scenario. One carries how a scenario **runs** — route, human-judgment owner, cost class, cadence. The other carries how it is **built** — fixture, seams exercised, the one artifact it exclusively owns. The checker errors when the two tables do not list exactly the same scenario set.
- **Reasoning**: authority adopted the recommendation — each table stays readable and cohesive where a single eight-column table would not, and the split's only real weakness (a scenario present in one table and missing from the other) is closed mechanically by the same-set rule rather than left to review. It also keeps the existing route table's shape recognizable instead of widening it past legibility. Fields buried under each scenario's prose were rejected: the cross-scenario rule that matters most — no two scenarios owning the same artifact (`FR-26`, `UB-9`) — is far harder to enforce across scattered blocks.
- **Anchor**: 2026-08-06 · scenario-fields turn
- **Status**: live

### D-6 — the decision-authority stand-in is a conditional eleventh slot in the harness form

- **Question**: where is `FR-22`'s decision-authority stand-in specified — a conditional extra slot in the harness form, its own top-level doc section, or wherever it fits in existing prose?
- **Answer**: a conditional eleventh slot in the harness form, alongside `FR-19`'s ten. A feature with no human decision inside the flow it exercises fills it with a stated not-needed reason; a feature that has one and leaves it blank is a mechanical defect. The buildability lens then judges whether the five specified properties are real — in particular that the stand-in can refuse or contradict and cannot see what the run asserts.
- **Reasoning**: authority adopted the recommendation — it mirrors D-3's split exactly (the checker owns presence, the lens owns substance) and honors `FR-22`'s own placement, which puts the stand-in in the harness section. A top-level section was rejected as making a usually-empty concern as structurally prominent as Scenarios; free placement was rejected as reopening the hole D-3 closed. The slot is conditional-with-reason, so `FR-19`'s ten unconditional elements stand and the contract's stated count is untouched.
- **Anchor**: 2026-08-06 · stand-in turn
- **Status**: live

### D-7 — the per-run bound is stated in tokens and session count; money is a derived view

- **Question**: what unit does the harness's per-run cost bound take (`FR-23`, `NFR-5`) — tokens and session count, money, wall-clock, or sessions-and-turns?
- **Answer**: tokens and session count — e.g. "at most two full agent sessions and 400k output tokens per run". A money figure is **derived** when wanted, by applying the repository's pinned per-model price map, never written into the frozen doc.
- **Reasoning**: authority adopted the recommendation — a token-and-session bound stays true as prices move, is countable while the run executes so the harness's cost-control element can actually enforce it, and converts to money on demand through `.pyxis/vendor/litellm-prices/`, which the pipeline-measurement tool already reads. Money was rejected because a frozen doc's dollar figure silently goes stale; sessions-and-turns because one 30-turn session can cost ten times another, bounding shape rather than spend; and wall-clock because incident I5 already cost this repository a stranded PR on exactly that — a wall-clock assertion is non-hermetic, a measurement rather than a bound, and cannot be enforced consistently across machines.
- **Anchor**: 2026-08-06 · cost-unit turn
- **Status**: live

### D-8 — the buildability lens reads blind; checker-duplicate findings are filtered after the fact

- **Question**: is the buildability lens handed the well-formedness checker's verdict before it reads the harness section, or does it read blind — and what happens to findings the checker already raised?
- **Answer**: blind. The lens reads the doc with no sight of the checker's output, exactly as the reviewer program's other lenses read theirs. Findings that restate something the checker already raised are filtered out **after** the lens returns, so the operator sees one clean list rather than the same structural defect twice.
- **Reasoning**: authority adopted the recommendation — handing a lens another judge's verdict breaks the reviewer program's standing independence property (its context pack is input-only and admits no verdict or authoring-trail kind, `design:e2e-verification-skills#DINV-3`), and a lens told "structure is already green" is a lens with its scrutiny pre-lowered. Filtering downstream keeps blindness intact while removing the duplicate noise that a zero-tolerance false-red guardrail would otherwise make the operator disposition one by one.
- **Anchor**: 2026-08-06 · lens-wiring turn
- **Status**: live

## Alternatives

### ALT-1 — Closed ten-element table; the checker owns answered-ness, the lens owns sufficiency

- **Shape**: the harness section takes exactly one of two forms. **Build form** — a required table, one row per element, element names drawn from a **closed ten-name vocabulary**, each row stating its answer. **Reuse form** (`prd:e2e-verification/FR-20`) — a citation naming the existing harness plus a delta list of this feature's additions, and no element rows. `internal/e2e/docclass` branches on the form, requires all ten names in build form, and rejects an answer that is empty, a placeholder, or only a bare library or tool name. The buildability lens judges whether each answer *suffices*, never whether it is present.
- **Trade-offs**: wins — two of `EVAL-3`'s four seeded harness holes (an element unanswered; an element answered only by a library name) become hermetic parser verdicts that never depend on a model's mood, and the closed vocabulary becomes one shared word list the template, checker, lens calibration, scenarios and plan all speak. Costs — a fixed ten-row form on every from-scratch harness, and the vocabulary is a versioned contract: an eleventh element is a class bump.
- **Principles**: AP-6 honored — closed, contractual data; an unrecognized element name is an error, not silent acceptance. AP-10 honored — the mechanical half is codified in the checker, not left to a hand-walked checklist. AP-11 honored — one element vocabulary across template, checker, lens and plan.
- **Incidents**: I5 matched — its lesson is that model-driven verdicts measure and never gate; putting presence in a hermetic parser and sufficiency in the lens is that division applied. I1/I4 checked and do not bite: they govern a technical design's `Serves` completeness, and this class carries no `Serves` (`design:e2e-verification-artifacts#DINV-6`). Checked `docs/observability/incidents.md` 2026-08-06.
- **Verdict**: chosen — authority ruling 2026-08-06 (D-3), with the lens's reach widened: it judges the substance of every slot, not only the checker's structural green
- **From**: DQ-2 · panel

### ALT-2 — Free prose; the lens owns everything

- **Shape**: the harness section stays one prose block, its heading renamed from `Harness libraries` to `Harness design`, with the ten elements carried as template guidance the author is told to cover. `docclass` gains no harness rule beyond the section-non-empty check it already has. The buildability lens carries the ten elements in its prompt and calibration table and is the sole judge of whether each is answered.
- **Trade-offs**: wins — cheapest to build (a template edit and a lens prompt, no parser work), and the section reads like a design rather than a form, which is how both built docs already read. Costs — the amendment exists because a library-name-only section passed review twice; leaving presence to judgment leaves that failure mode open behind a better prompt, and the corpus that would prove the lens catches it is a recorded measurement, never a gate (I5).
- **Principles**: AP-6 strained — no closed structure, so nothing downstream reads the section as data. AP-10 strained — a mechanically checkable property is left to judgment. AP-19 honored — lowest build cost, no new code.
- **Incidents**: I5 matched **against** this shape — it places a structural property behind a model-driven verdict, the arrangement I5 rules cannot gate. Checked `docs/observability/incidents.md` 2026-08-06.
- **Verdict**: rejected — leaves presence to judgment, the exact failure mode amendment 002 exists to close (a library-name-only section passed review twice)
- **From**: DQ-2 · panel

### ALT-3 — Open element set; the author declares which elements the harness owes

- **Shape**: the harness section carries a table whose rows the author chooses — each an element name and its answer — plus a stated reason for any of the ten it is not answering. `docclass` checks that every declared row is answered and non-placeholder and that each omission carries a reason; it does not require the ten. The buildability lens judges both whether an omission's reason holds and whether each answer suffices.
- **Trade-offs**: wins — bends to app shapes the ten elements fit badly and absorbs the `FR-20` reuse case without a second form, reusing the explicit-`none — <reason>` posture the class already takes for empty sections. Costs — an author optimizing to look complete can declare three rows, answer all three honestly and pass the checker clean, so completeness moves entirely to the lens, which is the judgment call amendment 002 was written to stop; and `FR-19`'s fit criterion ("a stated answer for each of the ten elements") is not structurally guaranteed.
- **Principles**: AP-6 partly honored — structured rows, but an open vocabulary. AP-10 strained — the completeness half stays judgment. AP-24 honored — nothing is regenerated; the author's declaration governs.
- **Incidents**: I5 matched **partly against** — completeness rides a model verdict. Checked `docs/observability/incidents.md` 2026-08-06.
- **Verdict**: rejected — an author can declare three slots, answer all three honestly and pass clean, so completeness returns to being a judgment call
- **From**: DQ-2 · panel

### ALT-4 — The skill looks and judges

- **Shape**: e2e-design reads the repo's existing test trees, judges whether any of them can drive this feature's primary external interface, and cites it by path in the reuse form of the harness section. No new artifact, no registry, no path rule. e2e-plan makes the same judgment when deciding whether the repo already carries a scenario built against that harness (`prd:e2e-verification/FR-25`).
- **Trade-offs**: wins — no new surface, and it extends the predecessor's standing ruling that app-shape choices are authored content rather than skill code (`design-brainstorm:e2e-verification-tech-design#D-9`); works on any repo layout. Costs — the judgment is unauditable at the moment it is made, and two features can cite the same harness on different terms with nothing detecting the fork, which is the drift `FR-20` exists to prevent.
- **Principles**: AP-25 honored — app-neutral, no per-shape skill code. *One adapter = hypothetical seam, two = real* honored — no port or registry invented for a single use. AP-6 strained — the answer is prose, not data any other tool can read.
- **Incidents**: none matched — checked `docs/observability/incidents.md` 2026-08-06.
- **Verdict**: chosen — authority ruling 2026-08-06 (D-4)
- **From**: DQ-3 · panel

### ALT-5 — A harness registry

- **Shape**: a small declared file lists every e2e harness in the repo — its name, the app shape it drives, its path, and the document specifying its ten elements. e2e-design looks the feature's interface up and cites the hit (or designs fresh on a miss); e2e-plan reads the same file to decide whether it owes a harness task and a conventions task.
- **Trade-offs**: wins — "does a harness already exist?" becomes a lookup rather than a judgment, giving the same answer on every run; one lookup serves both skills; it is the only shape in which a fork between two features citing one harness is detectable. Costs — a new shared file someone must keep true, and a stale registry is worse than none; nothing in the current contract asks for it.
- **Principles**: AP-6 honored — closed, contractual data with a typed shape. AP-10 honored — a mechanical lookup replaces a judgment. AP-27 strained — one shared file every feature appends to is exactly the contention point that principle warns against.
- **Incidents**: I6 matched — a shared index trusted from the working tree is a planning blocker in waiting; a registry inherits that failure mode unless it is regenerated rather than hand-maintained. Checked `docs/observability/incidents.md` 2026-08-06.
- **Verdict**: rejected — a shared file every feature must keep true, whose staleness is a planning blocker in waiting (I6), for certainty the authority judged not worth the contention (AP-27)
- **From**: DQ-3 · panel

### ALT-6 — Path convention

- **Shape**: a harness is whatever lives at a fixed known path for the feature's area; existence is a path test, and the skill cites the hit or builds fresh on a miss.
- **Trade-offs**: wins — zero new surface, fully mechanical, trivially testable. Costs — it assumes one harness per area and a fixed repository layout, and it answers the wrong question: a path test says a directory exists, never that what lives there *can drive this feature's interface*, which is the condition `FR-20` actually sets. A CLI harness sitting in another area's directory is invisible to it, so the skill rebuilds what it should have reused.
- **Principles**: AP-10 honored — mechanical and hermetic. AP-25 strained — bakes a repository-layout assumption into skill behavior, which the app-neutrality requirement (`prd:e2e-verification/NFR-1`) rules against.
- **Incidents**: none matched — checked `docs/observability/incidents.md` 2026-08-06.
- **Verdict**: rejected — answers the wrong question: a directory existing is not a harness that can drive this feature's interface, and it bakes a repository layout into skill code against the app-neutrality requirement
- **From**: DQ-3 · panel

## Seam map

- **S-1**: the `e2e-design` class's harness section — **replaces** — behind it: a closed slot form — `FR-19`'s ten unconditional elements plus `FR-22`'s conditional decision-authority stand-in — validated for presence by `internal/e2e/docclass` (empty, placeholder, or bare-library-name is a defect) and judged for substance by the buildability lens; a reused harness takes the alternate citation-plus-delta form (`FR-20`) — feeds: the artifacts successor's §2 Decision and §3 class definition (from D-3, D-6)
- **S-2**: the per-scenario fact surface — **extends** — behind it: two scenario-keyed tables, one for how a scenario runs (route · human-judgment owner · cost class · cadence) and one for how it is built (fixture · seams · the one exclusively-owned artifact), with a checker rule that both list the same scenario set — feeds: the artifacts successor's §3 class definition and the skills successor's plan decomposition and `UB-9` refusal (from D-5)
- **S-3**: the reviewer program's lens set and context pack — **extends** — behind it: a third blind lens (buildability) over the same input-only pack, with checker-duplicate findings filtered after the lens returns rather than suppressed before it reads — feeds: the skills successor's §3 review behavior and `Serves` (from D-8)
- **S-4**: existing-harness reuse and the conventions-task condition — **extends** — behind it: the skill's own judgment over the repository's test trees, citing a fitting harness by path; no registry artifact and no path convention — feeds: the skills successor's §3 e2e-design and e2e-plan behavior (from D-4)
- **S-5**: the two frozen designs' lineage — **replaces** — behind it: two parallel supersessions, one per predecessor, preserving the artifacts/behavior partition — feeds: both successors' `Supersedes` rows and `Serves` sets (from D-1)

## Facts

- **F-1**: Amendment 002 supersedes five Must functional rows (FR-1→FR-18, FR-6→FR-19, FR-8→FR-24, FR-16→FR-25, FR-17→FR-26) and adds four more (FR-20, FR-21, FR-22, FR-23) plus UB-8, UB-9, NFR-5, EVAL-3, DOD-8, ABUSE-5 and R6 — source: `prd:e2e-verification` amendment 002 Blast radius
- **F-2**: Both live e2e designs cite superseded requirement IDs in `Serves` and are flagged **`stale-serves`** by the generated index; `design:implement-now-e2e-tasks` builds on both and is flagged `stale-builds-on` — source: docs/tech-design/index.md rows 33, 34, 45
- **F-3**: A merged technical design is byte-immutable under `check-immutable` (INV-41) with no lineage-row exemption, so **whole-document supersession is the only legal repair** for a frozen design's stale lineage rows — source: `I6` (2026-07-30), Lesson
- **F-4**: The `e2e-design/v1` class's harness section is one free-prose paragraph in the template, and `internal/e2e/docclass` checks the closed section set, the coverage rows and the one-route rule — it carries no harness-element rule at all — source: `.pyxis/templates/e2e-design-template.md` §Harness libraries · `internal/e2e/docclass/docclass.go`
- **F-5**: Both built `e2e-design/v1` instances answered the harness section with a library preference and a fit argument — `go test` + `os/exec` + a docclass parser, and Go's `testing` package driving the built binary — with no stated scenario contract, isolation rule, determinism control, or per-run bound — source: docs/e2e/plan-new/design.md §Harness libraries · docs/e2e/tech-design-brainstorm/design.md §Harness libraries
- **F-6**: The frozen predecessor already ruled the reuse boundaries this session must not re-litigate — the lightweight `e2e-design/v1` class (`#D-1`), the two-design artifacts/behavior partition (`#D-11`), the reviewer-program and plan-new reuses (`#D-3`, `#D-5`, `#D-13`), the live-design-set read (`#D-14`), and lenses-as-data (`design:e2e-verification-skills#DINV-3`) — source: `design-brainstorm:e2e-verification-tech-design`
- **F-7**: The predecessor ruled the one-vs-two design packaging question **not a seam** — "a packaging choice that opens no system interface/port/class" — and skipped its panel on that ground — source: `design-brainstorm:e2e-verification-tech-design` panel log, DQ-2
- **F-8**: The executor's e2e-task consumption (`prd:implement-now` amendment 002, 52 files under `internal/executor/e2eadmit` · `e2ebasis` · `e2egit` · `e2eorigin`) was built **against the contract this session replaces** — its `Learned-from` cites `design:e2e-verification-skills#3.3` and `design:e2e-verification-artifacts#3.1` as dependency-changes, and it briefs a worker by reading the e2e design doc, with fixtures of the six-section class at `internal/executor/e2ebasis/testdata/design.md` and `internal/executor/e2eorigin/testdata/design.md`. The **frontier walk itself is unchanged** — e2e tasks ride the same frontier as ordinary tasks — so `prd:e2e-verification/NR-2` is not falsified; what changes is the briefing surface downstream of it — source: `docs/prds/implement-now/002-consume-e2e-tasks.md` header · `internal/executor/e2eorigin/evaluate_test.go:305` · `internal/executor/dispatch/dispatch.go`

## Fog

none

## Out of scope

- **O-1**: migration, grandfathering, or re-authoring of the two existing `e2e-design/v1` instances, and any backward compatibility with the removed authoring line — the authority is removing the existing e2e designs, plans and implementation outright, so the successors owe the old contract nothing (from D-2)
- **O-2**: a harness registry, catalogue, or any other shared cross-feature artifact declaring which harnesses exist — existing-harness reuse is the skill's own judgment over the repository, with no declared lookup surface (from D-4)
- **O-3**: money-denominated cost figures written into the frozen design doc, and wall-clock per-run bounds — the bound is stated in tokens and session count; a money view is derived on demand from the repository's pinned price map (from D-7)
- **O-4**: pre-screening the buildability lens with the well-formedness checker's verdict — the lens reads blind and duplicates are filtered after it returns, never before (from D-8)

## Candidate glossary terms

none

## Parked product gaps

none

## Gate

| Field | Value |
|---|---|
| **Verdict** | ready-for-design |
| **Date** | 2026-08-06 |
| **Authority** | @mdarabi (operator) |
| **Covering design** | — |
| **Coverage check** | pass — core four covered (§2 direction: D-3 the closed harness slot form + D-1 the two-supersession shape, both panel-logged; §4 alternatives: ALT-1/2/3 from the DQ-2 fired panel and ALT-4/5/6 from the DQ-3 fired panel, all six verdicted; Serves+Scope: the S-1..S-5 seam map + Basis `prd:e2e-verification`; §3.2 boundaries: O-1..O-4) — zero blocking fog |
| **Amend route** | — |
