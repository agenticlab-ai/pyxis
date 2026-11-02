# Read to rule (§0) for technical designs — the reviewer-routing section for the design class — technical design

| Field | Value |
|---|---|
| **Status** | Draft → Approved *(merge requires Approved; merge is the freezing event; "superseded" is computed by the index, never written here)* |
| **Owner** | @mdarabi |
| **Approver(s)** | @mdarabi (operator — document-system surface) |
| **Date** | 2026-07-17 |
| **Relevant-when** | authoring or changing the §0 read-to-rule contract for technical designs, the tech-design-new §0 step, or the tech-design-template §0 section; triaging a design §0 that fails conformance or routes to a dead anchor; superseding the design-class §0 mechanics |
| **Scope** | The §0 section contract for the technical-design class in tech-design-template's next version (placement before §1, the four blocks sourced from the design body, honest-zero forms, the cap, the required-with-skip rule, anchoring) and the one additive tech-design-new §0-authoring step. **Not**: PRD §0 (settled by `read-to-rule-v1`); any docgate change (the design-shape and lint gates already tolerate a leading §0); the capture-line / review-efficiency measurement federation at design gates (deferred, §3.4); the docgate-v2 design-class gate (`tech-design-conventions` §3.4 owns it); UX-design conventions. |
| **Tracking** | filed with the introducing PR (adoption tracked by #411) |
| **Builds-on** | `read-to-rule-v1` · `tech-design-conventions` |
| **Supersedes** | — |
| **Serves** | — |
| **Security/privacy impact** | none — template guidance and an authoring convention over repo-internal markdown; no new data class, egress, or surface (§3.7) |
| **Template version** | tech-design-template/v1 |

## 0. Read to rule — the human reviewer's section (~3 min)

*The body is the contract; this section only routes attention — on any conflict the body governs; machines ignore it.*

**Judge these** *(each row: the call · the alternative it displaced · what disagreeing concretely means, with its anchor)*

| # | The call | Could have gone | If you disagree |
|---|---|---|---|
| 1 | Deliver WR-1 as a **focused design** now (operator pull-forward), not read-to-rule's planned own-PRD-and-brainstorm feature | the fuller feature process, after read-to-rule's checkpoint | you want the fuller process, or think WR-1 shouldn't be pulled forward yet → §1, Q3 |
| 2 | Extend §0 to designs via **Builds-on** `tech-design-conventions`, not a supersession | a v2 supersession folding §0 into its §3.3 section set | you think §0 changes the class contract enough to re-issue it → §2, §4 row 1 |
| 3 | Ship the **routing section only** — defer the capture-line / measurement federation to design gates | wire the measurement now | you want design-review efficiency measured in this change, not later → §3.4, Q2 |
| 4 | §0 **required for every design** (existing designs grandfathered) with an honest whole-section skip | a tier carve-out mirroring the PRD T1 skip | you'd rather small designs skip §0 by rule, not by honest assertion → §3.1, §4 row 4 |
| 5 | Leave §0 **un-gated** (template + guide only), zero docgate code | add a lint-tech-designs presence check now | you want missing-§0 to fail CI today → §3.2, Q1 |

**Sanity-check the numbers** — the one-screen cap (≤ 40 source lines) and ranked-calls cap (≤ 7) are inherited verbatim from `read-to-rule-v1` §3.1; R2 (§5) carries the risk they mis-tune for designs.

**The author's open questions** — Q1 (gate §0-presence at docgate v2?), Q2 (which design gate the future capture line rides), and Q3 (is the focused-design vehicle right, vs read-to-rule's planned feature?) — with named owners (§7).

**Skip the rest** — schema/header/section conformance → docgate (v2 design gate, `tech-design-conventions` §3.4); substance, coherence, reuse-claims → the unmanned `tech-design-review`; implementation defects → codegate + downstream gates; the alternatives and rationale in §4 → explanatory, non-binding.

## 1. Problem

A technical design is approved at a human gate — the introducing PR's operator review, the "one gate that matters" (`tech-design-conventions` §4, its rejected status-ladder alternative). But unlike a PRD — which now opens with §0 "Read to rule," a one-screen routing section for exactly this reviewer (`read-to-rule-v1`) — a design offers that reviewer nothing: to find the load-bearing calls, the displaced alternatives, and the low-confidence questions, they must read the whole narrative and reconstruct what matters. The operator named the asymmetry directly: PRDs and tech designs should both carry a human-reviewer routing section.

The moment is cheap, and cheaper than it was for PRDs. The §0 concept — the four blocks, the honest-zero forms, the one-screen cap, the anchored routes, the authored-never-computed rule — is already settled and frozen by `read-to-rule-v1`; extending it to the design class states constants against surfaces that already exist. And the design class is a *better* structural fit than the PRD: a design already carries explicit `**Decision:**` callouts and an `#ALT-n` ledger, so "the calls that could have gone the other way" are first-class elements to point at, not prose to be mined.

This work is read-to-rule's own waiting-room item **WR-1** — "the tech-design-class sibling … the same treatment for design documents" — which that PRD deliberately deferred to "its own feature … after this feature's checkpoint," recording *one class per feature* (the design class's judgment surface differs). The operator has pulled WR-1 forward, ahead of that checkpoint, and directed it now; and because the design-class §0 proves a **faithful, low-novelty mirror** of `read-to-rule-v1` §3.1 — same blocks, caps, honest-zero, anchoring, differing only in the design-sourced content and the no-fold simplification — a focused design is the proportionate vehicle, not the full own-PRD-and-brainstorm feature WR-1 imagined for a then-unexplored surface. WR-1 is a *waiting-room* anchor, which incident I1 and `lint-tech-designs` bar as a `Serves` target; this document names it as intent-lineage and carries `Serves: —` under the document-system-convention rule rather than serving it. The vehicle is the operator's to confirm at the gate (Q3).

## 2. Decision

**Decision:** The technical-design class gains **§0 "Read to rule"** as its first section, before §1, on the same terms `read-to-rule-v1` set for PRDs — four blocks (Judge these · Sanity-check the numbers · The author's open questions · Skip the rest), honest-zero forms, a one-screen cap, anchored routes — but **sourced from the design's own binding elements**: the ranked calls are the `**Decision:**` callouts paired with the `#ALT-n` they displaced, the numbers are the acceptance/DINV bounds, the open questions are §7, and the skip map names the design's downstream backstops. It ships as **template guidance plus one additive guide step** — the contract lives in tech-design-template's next version, and tech-design-new gains one step to author §0 last from the design's own content — and it **builds nothing** in docgate: `check-design-shape` and `lint-tech-designs` already ignore a leading §0 (they check header-closedness and mandatory-subsection presence, never an exact section set). §0 is **required for every design**, with an honest whole-section skip line for a design that genuinely routes nothing; the class has no PRD-style tiers, so there is no tier gate. The review-efficiency **capture line** `read-to-rule-v1` added at PRD gates is **deferred** for design gates (§3.4) — this decision delivers the routing section only.

The relationship is **Builds-on, not Supersedes**: §0 is additive. Prepending an informative, non-normative section falsifies no *binding* statement of `tech-design-conventions` — its §3.3 section enumeration is descriptive prose, and every binding element (A-1, A-7, A-8) stays simultaneously true. Two live designs never disagree; these two don't. Every constant already sits on an existing surface (the template, the guide, the design's binding-element grammar), so the appetite is exactly `read-to-rule-v1`'s: one template section and one guide step.

## 3. The design

### 3.1 The §0 template contract (for the design class)

**Decision:** tech-design-template's next version defines §0 as the design-class analog of `read-to-rule-v1` §3.1 — same shape, design-sourced content:

- **Placement and heading** — the first section, before §1: `## 0. Read to rule — the human reviewer's section (~N min)`, `~N min` the authored reading-budget hint. The template's existing `## 0. How to use this template` (instantiation scaffolding, deleted by the author) is **un-numbered** to `## How to use this template`, freeing the §0 slot — exactly the arrangement prd-template already uses.
- **The normativity notice** — the section opens with one italic line, verbatim: *the body is the contract; this section only routes attention — on any conflict the body governs; machines ignore it.*
- **Four blocks, in order** —
  - **Judge these** — the ranked calls: a table of up to 7 rows, each carrying the call · the alternative it displaced · what disagreeing means → its §/row anchor, ordered by irreversibility × blast radius. **Sourced from the design's `**Decision:**` callouts (§2 and load-bearing §3.x) paired with the `#ALT-n` each displaced** — the design's own first-class elements.
  - **Sanity-check the numbers** — the acceptance thresholds, DINV bounds, and risk tolerances worth a gut-check, each anchored — or the honest-zero form.
  - **The author's open questions** — the §7 `Q-n` rows still low-confidence and awaiting the reviewer, each anchored — or the honest-zero form.
  - **Skip the rest** — the informed-skip map: each omitted content class with the mechanism that backstops it (schema/structure → docgate; substance/coherence/reuse → the unmanned `tech-design-review`; implementation defects → codegate + downstream gates; rationale/alternatives → §4, explanatory).
- **Honest zero** — an empty block reads `none — <one-line assertion>`; an absent block is nonconformant, an asserted-empty one is information.
- **The cap** — §0 fits one screen: at most 40 markdown source lines from the heading through the skip map's last line, ranked calls at most 7. Inherited verbatim from `read-to-rule-v1` §3.1; retunable only by a template PR.
- **Required rule** — §0 is required for **every** design. A design that genuinely routes nothing carries the whole-section skip line `Skipped: <reason>`. The class has no tier system (`tech-design-conventions` rejected PRD-style tiered gates), so the PRD T1 carve-out has no analog: the escape is per-document honesty, not a tier. **Grandfathering** — designs merged before this template version are immutable and cannot gain a §0 (or a `Skipped:` line); they are exempt, exactly as `tech-design-conventions` §3.1 grandfathers pre-convention documents. The future docgate-v2 §0-presence gate (Q1), if built, inherits the exemption through the same declared-`Template version` mechanism: it checks §0 only on designs whose declared version is at or past the one that introduces §0.
- **Anchoring** — every routed item names a §/row id that resolves within the same document (DINV-1).

### 3.2 Boundaries and non-goals

- **Not solved here** — the docgate-v2 presence gate for §0 (this design requires none; §0-presence is a candidate for the v2 design gate, `tech-design-conventions` §3.4, if drift is observed — Q1); the review-efficiency capture line for design gates (§3.4).
- **Owned elsewhere** — PRD §0 (`read-to-rule-v1`); the design-class section-set and header contract (`tech-design-conventions` §3.1/§3.3 — this design **Builds-on** it additively, editing nothing).
- **Implementation must not infer** — that §0 needs IDs, index awareness, or docgate logic (it is prose to every tool); that adding §0 requires superseding `tech-design-conventions` (it is additive — Builds-on); that a design §0 is ever machine-consumed (DINV-3).
- **Must remain unchanged** — docgate behavior and verbs; the `tech-design-conventions` §3.3 section enumeration (untouched — §0 is prepended before §1, renumbering nothing); the generated design index; the §1–§7 + `## Principles` numbering.
- **Benign consequence, noted for completeness** — un-numbering the template's `## 0. How to use` repoints the *merge-time* evidence hint `template §0` in `tech-design-conventions` A-7 (whose guidance — "binding vs explanatory prose" — now sits in the un-numbered How-to-use section, while §0 is Read-to-rule). That acceptance row's binding claim stays true (the template still instructs authors), and `tech-design-conventions` is **frozen**, so nothing is edited: a frozen acceptance row records the state at *its* merge, and additive template evolution is exactly what that convention's §3.1 sanctions.

### 3.3 Authoring mechanic — §0 is authored, and simpler than the PRD case

**Decision:** tech-design-new gains one additive step: §0 is written **last**, after §1–§7 and Principles are stable, from the design's own content — each ranked call lifted from a `**Decision:**` callout and its displaced `#ALT-n`, each number from an acceptance/DINV bound, each open question from §7, every routed item anchored per DINV-1. §0 is never generated — it re-litigates nothing of `read-to-rule-v1`'s frozen ruling that §0 is authored, not folded.

Maintenance is the design class's existing evolution, and this is where the design case is **strictly simpler** than the PRD case: a design is immutable at merge, so §0 is refreshed only by a successor document (whole-document supersession) — there is no amendment op, no fold, no `op: supersede §0`, none of the amendment machinery `read-to-rule-v1` §3.6 needed for the PRD ledger. The design class's whole-document supersession already carries §0 forward or rewrites it, at zero additional cost.

### 3.4 The capture line — deferred for design gates (the measurement boundary)

**Decision:** This design ships the §0 **routing section** only; it does **not** extend `read-to-rule-v1`'s federated capture-line convention to design review gates. The capture line measures review efficiency by federating per-gate minutes and per-item routing into the review-feedback ledger — machinery `read-to-rule-v1` scoped to PRD gates and tied to the still-maturing review-feedback-capture feature. Design review has two candidate gates — the unmanned `tech-design-review` and the human approval — either of which could one day carry the line, but binding that measurement now would couple this template change to another feature's timing, the I1-class inversion `read-to-rule-v1` itself rejected as its ALT-2. The graduation path is explicit and symmetric with `read-to-rule-v1`'s own: when design-review efficiency measurement is wanted, a later change extends the capture-line convention to design gates through the review-feedback feature's channel (Q2). Until then, §0 for designs is a routing aid with no measurement obligation.

### 3.5 Invariants

- **DINV-1** — every live §0 item in a design resolves to a live anchor (§/row) in that same design; a dead route is a defect in §0, never in the body (which governs).
- **DINV-2** — §0 carries no sub-anchors and no downstream artifact cites into it; it is refreshed only by whole-document supersession (the class's evolution model), never amended in place.
- **DINV-3** — §0 is informative, never normative: on any conflict the body governs, and no machine consumer (the unmanned reviewers per their own adoption, the stage-6 compiler, the index, traceability, workers, stage flows) reads it.

### 3.6 Observability

none — no runtime surface. §0 is authoring guidance; its own health (a routed anchor that does not resolve) is an operator review concern now and a docgate-v2 resolution-check concern later, not a runtime one. With the capture line deferred (§3.4), this design adds no measurement channel.

### 3.7 Security & privacy

none — matches the header row: template guidance and an authoring convention over repo-internal markdown; no identity, secret, user-data, network-boundary, or model/provider-egress surface.

## 4. Alternatives considered

| Alternative | Why rejected |
|---|---|
| Supersede `tech-design-conventions` (v2) to fold §0 into its §3.3 section enumeration | heavier and RW-5-strained — re-issues the whole class contract to add one informative section, bundling the §0 decision with conventions that change on a different clock. §0 is additive (it falsifies no binding statement of conventions), so **Builds-on** is the honest relationship — exactly as `read-to-rule-v1` built on `tech-design-conventions` rather than superseding a governing doc |
| Extend `read-to-rule-v1`'s capture line to design gates now | couples this template change to review-feedback-capture's timing for a measurement the operator did not ask for (the ask was the routing section); `read-to-rule-v1` rejected the same coupling as its ALT-2; deferred with an explicit graduation path (§3.4) |
| Gate §0-presence in docgate (`lint-tech-designs`) now | the surface exists, but `read-to-rule-v1` deliberately left PRD §0 un-gated (its AP-10 deviation — presence/format stays convention); mirroring keeps the two classes symmetric and this change zero-code. §0-presence can join the docgate-v2 design gate later if missing-§0 drift is observed (Q1) |
| Tiered §0 (skip small designs, mirroring the PRD T1 carve-out) | the design class has no tier system to hang the rule on (`tech-design-conventions` rejected tiered gates); every design carries ≥ 1 `**Decision:**` callout worth routing, so required-for-all with an honest whole-section skip is both simpler and stricter than importing a tier gate designs otherwise lack |
| Fold-generated §0 for designs | unlawful — re-litigates `read-to-rule-v1`'s frozen ruling that §0 is authored, never computed; and designs have no fold at all, so there is nothing to generate from |

## 5. Risks

| ID | Risk | Impact | Mitigation | Verification / evidence |
|---|---|---|---|---|
| R1 | the four-block sourcing (Decision-callouts + `#ALT-n`) strains on a design whose load-bearing choices are not in explicit callouts | §0 mis-routes, or authoring stalls hunting for calls | the template step names the exact sources; tech-design-new discipline already requires load-bearing choices to be `**Decision:**` callouts (template guidance) — a design with none is a pre-existing defect §0 *surfaces*, not one it causes | operator review of the first §0-bearing designs |
| R2 | the caps (40 lines / 7 calls), inherited from the PRD case, mis-tune for designs that carry more independent calls | §0 bloats, or truncates genuinely top-tier calls | constants live in template guidance, retunable by one template PR; the ranking rule (irreversibility × blast radius) forces the 7 to be the *most* consequential, not the first 7 | first-designs review; retune if the 7-cap clips top-tier calls |
| R3 | §0 goes stale within a live design when a `Builds-on` foundation moves — a routed call still resolves but its framing is outdated | the reviewer routes to a call whose context shifted | designs are immutable at merge, so §0 cannot silently drift post-merge — only a superseding design rewrites it; pre-merge, operator review re-renders and re-parses §0's anchors against the frozen body (DINV-1) | the operator review re-render re-parses §0 anchors on every edit |
| R4 | un-gated §0 lets a design merge without one (the AP-10 residual, inherited) | routing-coverage gaps until docgate v2 | the tech-design-new step + operator review are the enforcement surface now — the same transitional posture the whole design class runs under until docgate v2 (`tech-design-conventions` R4); §0-presence is enumerated as a v2-gate candidate | design reviews catch a missing §0; v2 lint retroactively enforces if adopted |

## 6. Acceptance

| ID | Acceptance criterion | Verify | Required evidence | Traces to |
|---|---|---|---|---|
| A-1 | tech-design-template's next version carries §0 before §1 — placement, the verbatim notice, the four blocks with honest-zero forms, cap 40 source lines / calls ≤ 7, the ranking rule, the required-with-skip rule, the design-body sourcing — and its former `## 0. How to use` is un-numbered to free the slot | Inspection | the template diff | §3.1 |
| A-2 | tech-design-new gains one additive step: author §0 last from the design's own content, every routed item anchored (DINV-1) | Inspection | the guide diff | §3.3 |
| A-3 | this document self-conforms — it carries a conformant §0 (four blocks, honest-zero where empty, ≤ 40 source lines / ≤ 7 ranked calls, every route resolving in this document) | Test — the deterministic §0 source-line count (≤ 40) and ranked-call count (≤ 7) plus §/row anchor-resolution, the mechanical procedure R3 names (no docgate verb enforces it — AP-10) | the counts (19 source lines / 5 ranked calls) + the operator review anchor re-parse | §3.1 · DINV-1 |
| A-4 | zero docgate change — the new §0-bearing template and this design pass `lint-tech-designs` and `check-design-shape` unmodified (closed header, Observability + Security & privacy subsections present, legal `Serves`) | Test | the gate run on this PR | §2 · §3.2 |
| A-5 | the capture-line deferral and its graduation path are stated; no review-feedback or measurement obligation is created for design gates | Inspection | §3.4 | §3.4 |
| A-6 | the relationship to `tech-design-conventions` is Builds-on (additive), stated with the no-contradiction argument, and its §3.3 enumeration is not edited | Inspection | §2 · §4 row 1; the unchanged conventions doc | §2 |
| A-7 | the design names WR-1 as its requirement-lineage, states why `Serves` is `—` (WR-1 is a waiting-room anchor barred by I1) rather than silently claiming pure infrastructure, and records the focused-design vehicle for gate confirmation | Inspection | §1 · Principles (I1) · Q3 | §1 |

## 7. Open questions

| ID | Question | Owner | Blocks | Status | Resolution |
|---|---|---|---|---|---|
| Q1 | Should §0-presence be enforced at the docgate-v2 design gate, or stay convention like the PRD case? | operator | nothing — §0 ships un-gated either way | open | deferred — mirrors `read-to-rule-v1`'s un-gated AP-10 posture; a candidate for the v2 gate scope (`tech-design-conventions` §3.4) if missing-§0 drift appears |
| Q2 | When design-review efficiency measurement is wanted, does the capture line ride the human gate, the unmanned `tech-design-review` gate, or both? | operator | nothing — deferred | open | deferred to the graduation path (§3.4), through the review-feedback feature's channel |
| Q3 | read-to-rule scoped WR-1 as "its own feature" (own PRD + brainstorm); this delivers it as a focused design on the operator's pull-forward directive plus the faithful-mirror low novelty. Is the focused-design vehicle right, or is the fuller feature process wanted? | operator | nothing — proceeds as a focused design | resolved | operator confirmed the focused-design vehicle (2026-07-17); the fuller feature process was offered and declined. Formal approval rides the introducing PR. |

## Principles

- **AP-11 — honored.** One routing section, sourced from the design's existing binding elements; §0 duplicates no content — it points at the calls, numbers, and questions that already live in §2/§3/§6/§7.
- **AP-19 — honored.** Steady-state cost is zero tokens: §0 is authored once at design time and frozen at merge; no machine reads it, nothing recomputes it.
- **AP-21 — honored.** The ranked calls and their ordering freeze when the design is approved; nothing re-derives §0 later — a superseding design authors a fresh one.
- **AP-23 — honored.** §0 carries no sub-anchors and is refreshed only by whole-document supersession, so parallel design authors never collide on it — the class's collision-free evolution model.
- **AP-24 — honored.** §0 is derived from the design and governs nothing: it routes a human's attention and is explicitly non-normative (DINV-3); the body remains the sole authority.
- **RW-5 — honored.** One independently-revisable decision: the §0 routing section for the design class. The capture-line measurement (§3.4), the docgate-v2 enforcement (Q1), and the PRD §0 (`read-to-rule-v1`) are separate changes or separate documents.
- **I1 — honored.** This design realizes read-to-rule's waiting-room item WR-1, but WR-1 is a waiting-room anchor, which I1 and the lint bar as a `Serves` target; the document names WR-1 as intent-lineage (§1) and carries `Serves: —` under the document-system-convention exemption, never back-filling a requirement anchor.
- **Deviation: AP-10** — §0 presence and format stay convention, not code: no lint enforces them in this change. Reason: symmetry with the PRD case (`read-to-rule-v1`'s own AP-10 deviation) and a zero-code extension; the named upgrade path is the docgate-v2 design gate (Q1).
