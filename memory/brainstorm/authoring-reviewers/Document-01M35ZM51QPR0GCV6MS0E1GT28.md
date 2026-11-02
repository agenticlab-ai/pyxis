# Authoring reviewers (prd-review · tech-design-review) — brainstorm record

| Field | Value |
|---|---|
| **In one line** | Two reviewer helpers for the authoring pipeline — prd-review and tech-design-review — that double-check authored documents and deliver valuable, complementary feedback without nit-picking or author↔reviewer ping-pong. |
| **Relevant-when** | brainstorming or contracting the document-reviewer skills, their feedback contract, or their place around the stage-2/stage-5 human gates; checking prior rulings on review loops, finding severity, or reviewer↔author interaction |
| **Idea** | "Two new pieces in our pipeline that act as helpers to the engineers. We want two reviewers: prd-review and tech-design-review that double check everything and provide useful feedback. They are not just nit picking or stuck in the loop of back and forth with the authoring agent but complement them and provide valuable feedback." (operator, 2026-07-10, verbatim) |
| **Operator** | @mdarabi |
| **Date(s)** | 2026-07-10 |
| **Status** | gated: ready-for-prd |
| **Tracking** | — |
| **Feeds** | docs/prds/prd-review/ |
| **Prior records** | — |
| **Template version** | brainstorm-record/v1 |

## Orientation

| Source | Why loaded |
|---|---|
| docs/product-vision.md | §4 stages 2/5 (the human reviews these helpers serve); §6 evidence packages + escalation quality; §10 design-share metric |
| docs/prds/feature-brainstorm/effective.md + tech-design-brainstorm/effective.md | the authoring pipeline being reviewed; M1/M2 metric patterns; the review-adjusts-merge-freezes rule |
| .pyxis/authoring/prd-new.md + tech-design-new.md | the authoring guides whose output is the review subject; §8 contract-readiness walk (the existing hand-checklist) |
| docs/prds/design-and-planning.md | PLAN-3 — the operator conversation used to review a design and apply confirmed feedback |
| docs/prds/docgate/effective.md | the mechanical floor — what a reviewer must never re-report |
| external prior-art survey 2026-07-09 | code-review's two-axis pattern (Standards vs Spec, parallel sub-agents, findings never merged/reranked, judgment-calls-not-violations, "skip anything tooling enforces"); adversarial-verify + completeness-critic workflow patterns |
| docs/architecture-principles.md | AP-16 (bounded loops, escalate), AP-21 (judge once, freeze), AP-10 (codify over prompt) |

**Overlaps surfaced:** the operator conversation is the human review channel, not a reviewer — it carries no judgment of its own. docgate/`lint-*` are the *mechanical floor* — structure, never substance. The PRD template's §8 walk is a hand-checklist the author self-applies. No agent reviewer exists; the ground is open.

## Framings

- **The pre-gate analyst** — each reviewer runs once after synthesis and produces a structured, severity-tagged findings **report** the operator consumes at their gate session; the author applies accepted findings in one revision pass. Thesis: *make the human gate cheap; the report is the artifact.*
- **The conversation collaborator** — the reviewer presents findings in the same operator conversation used for human review; agent findings and operator findings converge in one place, the operator arbitrates each, and accepted ones are applied to the source. Thesis: *one review channel, two reviewers; the operator is the arbiter — there is structurally no author↔reviewer channel to ping-pong on.*
- **The survival panel** — parallel reviewer lenses (contract quality · upstream alignment · feasibility/risk) generate candidate findings, then adversarial verification kills everything that doesn't survive independent scrutiny; only survivors surface, each with severity and evidence. Thesis: *fewer, harder findings — valuable by selection pressure.*

**Destination:** the *verdict-producing review protocol* (D-2) — self-contained under D-1's no-human assumption, escalation surfaced per D-3 — delivered as **two separate features** (D-4): this record feeds `prd-review`; the `tech-design-review` sibling runs its own session citing this record's shared rulings.

## Decisions

### D-1 — the world assumption

- **Question**: do the reviewers assume a human at the review surface?
- **Answer**: no — "consider the fully automated system where there is no human to co-annotate or review, so we will fully rely on this reviewer system." Amplified same session: "the human may or may not be reviewing the doc, so the reviewer system should be fully self-contained with no assumption of the human review's existence."
- **Reasoning**: the long-run posture (human involvement ends at feature-brainstorm) makes these reviewers what stands where stages 2/5's human review stood; the protocol must run to completion with zero human involvement in every case — human review, when present, is an optional overlay on the same data, never a dependency. Rules out the co-annotator framing as identity; any human-facing surface is a delivery mode, not the thing.
- **Anchor**: 2026-07-10 · t2–t3
- **Status**: live

### D-2 — the identity: the verdict-producing review protocol

- **Question**: what is the reviewer, in a world where nothing human arbitrates it?
- **Answer**: adopted as recommended — (1) findings as closable data: severity (blocking/advisory) · axis · evidence (document quote + upstream anchor) · a named acceptance condition, with the docgate floor (nothing tooling enforces is ever reported); (2) a total verdict set — approve · approve-with-advisories · request-changes · escalate — with blocking findings binding on the author (apply, or decline-with-recorded-reason which auto-escalates), re-review scope-locked to named defects + regressions, rounds hard-capped (2), every path terminating in approve* or escalate (an unanswered escalation parks the document fail-closed — recorded, alarmed, zero spend); (3) parallel-lens generation with adversarial survival underneath; (4) any human review is an optional overlay reading the same data — the verdict exists before and regardless.
- **Reasoning**: four independent loop-brakes (no conversational channel, closable findings, monotonic scope lock, round cap) make ping-pong structurally impossible; stage-6 verdict semantics (named defects, bounce-back) applied per-document at birth; the report doubles as the document's evidence package — what makes an unreviewed-by-human approval auditable (D-1's world).
- **Anchor**: 2026-07-10 · t5
- **Status**: live

### D-3 — escalation surfacing: the pipeline reacts, the reviewer only emits

- **Question**: how does anyone learn about a document parked by an escalate verdict?
- **Answer**: confirmed as recommended — the reviewer's output ends at data (verdict + decision brief); the surrounding machinery reacts: (1) the brief is minted as a **work item** addressed to the decision authority, carrying the GOV-5 operator-class routing signal, linking the parked document, un-parking it only on closure; (2) a **domain event** fires (AP-5), and notification transports (chat/email/digest/dashboard) key off events and signal-carrying items — an adapter concern, deliberately outside this feature; (3) **staleness is alarmed** (AP-16/OBS-4-class): an open escalation past threshold gets louder, never quieter. Bootstrap binding: a labeled issue + the verdict riding the review artifact in the PR; item-minting is the stage machinery's job (engine-era) or the session's closing convention (today).
- **Reasoning**: stuck things must be work items, not vibes — queryable, queued, closable; the reviewer's write perimeter stays clean; the fail-closed park is safe precisely because it cannot be quiet. Feeds §10's interventions taxonomy with the vision's self-classification.
- **Anchor**: 2026-07-10 · t6
- **Status**: live

### D-4 — two PRDs, one per reviewer

- **Question**: one feature (the protocol, binding two skills with per-class lens packs) or two features?
- **Answer**: two PRDs — prd-review and tech-design-review, each self-contained.
- **Reasoning**: operator's ruling (recommendation was one protocol-PRD; chosen: two) — the independent-evolution posture of D-6 in the brainstorm program, applied again: the two reviewers evolve freely, mirrored by pattern rather than coupled by artifact, drift bounded by per-feature evals. The uniformity concern (one verdict grammar for a future engine consumer) parks for the engine era: if a shared grammar becomes load-bearing, the engine's own design binds it as a port contract then, with richer context (AP-21). Consequence: this session narrows to prd-review; the sibling session starts fresh citing this record.
- **Anchor**: 2026-07-10 · t7
- **Status**: live

### D-5 — prd-review's lens pack

- **Question**: what does prd-review judge a PRD against?
- **Answer**: four lenses adopted — contract quality (EARS held, vagueness sweep, fit criteria genuinely verifiable, honest risk tiers, checkpoints-not-dates) · coherence (goal↔metric↔FR↔DoD chains unbroken, §8 verified not trusted, skips reasoned, priorities ranked) · upstream alignment (vision/principles/**incidents** fidelity — incident-memory matching explicit per the operator: does §6.3 ignore a known incident class, which lesson does this repeat; no silent conflict with other merged PRDs; record-fed basis fidelity — every live D-n carried, citations resolve, fog kept) · the completeness critic (missing failure modes, warranted abuse cases, the what-would-stage-6-bounce pre-check). Calibration as the anti-nitpick dial: coherence + alignment findings default blocking; contract-quality findings default advisory unless a Must row goes unverifiable; critic findings block only when load-bearing for §8. The docgate floor excluded throughout.
- **Reasoning**: the lenses are parallel generators feeding adversarial survival (D-2's discipline); calibration-as-data keeps severity tunable without prose edits.
- **Anchor**: 2026-07-10 · t8
- **Status**: live

### D-6 — the rest of the lens field, dispositioned

- **Question**: beyond the four lenses, what else — and where does each candidate land?
- **Answer**: folds — **incident memory into lens 3, explicitly** (operator-emphasized: upstream = vision · principles · incidents); duplication/overlap named inside lens 3; eval-quality (§5.9 adequacy) a named clause of lens 1; cost-consciousness a checklist item on lens 4. Refusals — a style/readability lens (the nit factory; template guidance already governs); a worth-it lens, refused on the binding boundary rule: **the reviewer verifies the contract about the decisions; it never re-judges the decisions themselves** (it may flag a missing record anchor, never a "wrong" ruling). Parks — red-team simulation (workflow-era, waiting room); feasibility (solution-space — belongs in tech-design-review's sibling pack, noted for that session).
- **Reasoning**: lens count stays at four (parallel generation stays cheap); the refusals encode the complement-not-compete constraint and the judge-once provenance; the boundary rule graduates to a requirement at synthesis.
- **Anchor**: 2026-07-10 · t9
- **Status**: live

### D-7 — invocation moments: prd-new and prd-amend both

- **Question**: when does prd-review run?
- **Answer**: at **both** authoring moments — (a) `prd-new`: on the draft 000-base after the fold, before optional operator review and the PR; (b) `prd-amend`: on the amendment after validate+fold+diff, before its PR — reviewing the **folded effect** (the diff, the affected IDs, blast-radius honesty declared-vs-computed, Learned-from evidence resolving) plus the resulting projection's chain integrity, scope-locked to the change by construction. Same protocol at both moments; the verdict + report ride the respective introducing PR as the evidence package; re-review per the protocol only. Engine-era: the stage runner invokes at the same two positions unchanged.
- **Reasoning**: operator widened the recommendation (draft-only, amendments parked) to both — amendments are where a frozen contract actually changes, so leaving them unreviewed guards the front door and leaves the side door open; the `diff` verb already produces exactly the reviewer's input shape.
- **Anchor**: 2026-07-10 · t10
- **Status**: live

### D-8 — the audience

- **Question**: whom does prd-review serve?
- **Answer**: P1 the authoring flow (prd-new/prd-amend sessions — the findings consumer, applying or declining); P2 the decision authority (verdicts + escalation briefs; optional overlay on the same data); P3 the downstream pipeline and auditors (the report as the document's evidence package — approvals reconstructable with no human having watched); anti-segment: authors wanting an auto-fixer — the reviewer never edits, never re-judges decisions, never judges unrecorded. Jobs J1–J5: defects caught pre-freeze at synthesis; folded-effect integrity at amendment; approve-as-recorded-gate in the no-human world; one pre-digested brief per conflict; audit-time why-approved from the report alone.
- **Reasoning**: authority-neutral per D-1; mirrors the proven segment pattern of the four-skill program.
- **Anchor**: 2026-07-10 · t11
- **Status**: live

### D-9 — the success bet

- **Question**: what does prd-review measurably win?
- **Answer**: the bet — a protocol-bound reviewer produces findings the author accepts and leaves nothing for the human overlay to catch. M1 (primary): blocking-finding acceptance rate ≥ 80%, median across the first ten reviews (valuable-not-nitpick, measurable — nits get declined). M2: human-overlay residual — defects found in the optional human review after an approve verdict — trending to median 0 by the checkpoint (the evidence that D-1's full reliance is earned). G3 (qualitative): the authority judges findings worth-having at gate sessions. Guardrails: zero scope-lock violations (sev-1 protocol breach); escalation rate held as a band (never-escalates = rubber stamp, always-escalates = useless — both alarm); cost per review a bounded constant (the round cap). Checkpoint: ten reviews across both invocation moments.
- **Reasoning**: each operator constraint becomes a number; M2 is what legitimizes removing the human, as evidence rather than hope.
- **Anchor**: 2026-07-10 · t12
- **Status**: live

### D-10 — non-goals and waiting room

- **Question**: what does prd-review refuse, and what waits?
- **Answer**: refuses — editing the document (application is the author's); re-judging decisions (verify the contract about the rulings, never the rulings); signing the human gates (approve is data beside them); docgate duplication; a style/readability lens; the tech-design sibling (own feature, D-4); tracker writes by the reviewer itself (D-3 layering); any class beyond PRDs. Waiting room — verdict-substitutes-the-gate (autonomy-policy era, the trust ladder's document analog); reviewer self-tuning from acceptance/decline telemetry (Optimizer era); a shared verdict grammar as an engine port contract (D-4's parked concern); the red-team simulation lens (D-6).
- **Reasoning**: the perimeter encodes complement-not-compete structurally; every waiting-room row names its revisit era.
- **Anchor**: 2026-07-10 · t13
- **Status**: live

### D-11 — appetite, with measurement capture in scope

- **Question**: what is this feature worth?
- **Answer**: small — guide + skill glue + the finding/report artifact contract (schema pinned by this feature's tech design) + eval fixtures (incl. the protocol's integrity negatives: scope-lock breach, round-cap, decline-escalation, rubber-stamp, nit-storm); two additive guide edits (prd-new and prd-amend, per D-7); ≤ 2 human review rounds per document; zero rollout burden. **Operator addition: the measurements ride the artifacts** — the report schema records per-finding disposition (applied / declined-with-reason — the author's pushback), round count, and verdict history; human corrections found in the optional overlay after an approve are captured back into the review report (the M2 residual datum), so M1/M2 and the guardrails are computable by inspection with no separate telemetry system. Excluded: engine/stage-runner reaction wiring, notification adapters, the tech-design-review sibling, docgate changes, any stage-1/3 surface.
- **Reasoning**: metrics without capture are fiction (the template's own rule); riding the artifacts keeps instrumentation zero-infrastructure in the bootstrap and schema-ready for the engine era.
- **Anchor**: 2026-07-10 · t14
- **Status**: live

## Facts

- **F-1**: operator comments and reviewer findings converge in the same conversation before confirmed edits are applied — another reviewer in that conversation is a channel capability question, not a new pipeline stage — source: PLAN-3 lineage
- **F-2**: the external survey's code-review pattern binds two rules directly relevant to the anti-nitpick constraint: findings are judgment calls (never "violations") and anything tooling already enforces is never reported — source: 2026-07-09 survey (code-review skill; two-axis review)

## Fog

none

## Out of scope

- **O-1**: a style/readability lens — the nit factory; template guidance already governs (from D-6)
- **O-2**: re-judging decisions — the reviewer verifies the contract about the rulings, never the rulings; missing anchors flag, "wrong ruling" never (from D-6)
- **O-3**: editing the document / auto-applying findings — application is the author's (from D-10)
- **O-4**: signing the human gates — approve is data beside them; substitution is a parked autonomy-policy question (from D-10)
- **O-5**: re-reporting the docgate floor (from D-5)
- **O-6**: reviewing any class beyond PRDs, incl. the tech-design sibling — its own feature (from D-4/D-10)
- **O-7**: tracker writes by the reviewer itself — the surrounding flow mints; the reviewer emits data (from D-3)

## Candidate glossary terms

- **T-1**: closable finding — a review finding that names its own acceptance condition at birth; whether a revision closes it is checked against that stated condition, never against fresh taste. The anti-ping-pong unit. · avoid: comment, nit, suggestion
- **T-2**: scope lock — the convergence rule that a re-review may examine only the previously named defects plus regressions in changed spans; new findings on unchanged text are protocol violations. · avoid: re-review scope (vague)

## Parked technical constraints

- **P-1**: the finding/report/brief artifact schemas, their home, and the lens-pack-as-data format (incl. the severity-calibration table and the measurement fields of D-11) — pinned by this feature's tech design, not here (from D-2/D-5/D-11)

## Gate

| Field | Value |
|---|---|
| **Verdict** | ready-for-prd |
| **Date** | 2026-07-10 |
| **Operator** | @mdarabi |
| **Covering source** | — |
| **Coverage check** | pass — all six inputs covered (§2.1←D-1/D-2 · §2.2←D-8 · §2.3←D-8 · §3.1←D-9 · §3.4←O-1..7 · appetite←D-11), zero blocking fog |
