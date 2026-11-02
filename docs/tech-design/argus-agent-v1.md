# Argus agent v1 — the drive protocol

| Field | Value |
|---|---|
| **Status** | Draft → Approved *(merge is the freezing event; "superseded" is computed by the index, never written here)* |
| **Owner** | @mdarabi |
| **Approver(s)** | @mdarabi (operator) — delegated-authority behavior, named reviewer mandatory |
| **Date** | 2026-07-11 |
| **Relevant-when** | designing or changing Argus's drive loop, session/resume behavior, assumption or escalation handling, the front door, the seam-amendment program, or the eval harness; triaging drive defects — duplicated external effects, missed escalations, floor violations, injection responses |
| **Scope** | The behavior half of the Argus pair: the per-stage drive loop and assumption regime, session posture and the resume protocol, the two-level escalation spine (briefs, parks, bumps), the front door and session bootstrap, the seam-amendment program's shape and order, and the eval harness. **Not**: the data contracts it reads and writes (argus-artifacts owns them); the stage protocols themselves (each stage's documents own those, amended per the program here); the fleet's internals; the report classes. |
| **Tracking** | — |
| **Builds-on** | docs/tech-design/argus-artifacts.md · docs/tech-design/plan-new-v1.md · docs/tech-design/prd-review-artifacts.md · docs/tech-design/tech-design-review-artifacts.md |
| **Supersedes** | — |
| **Serves** | prd:argus/FR-3 · prd:argus/FR-4 · prd:argus/FR-5 · prd:argus/FR-6 · prd:argus/FR-7 · prd:argus/FR-8 · prd:argus/FR-9 · prd:argus/FR-10 · prd:argus/FR-15 · prd:argus/UB-1 · prd:argus/UB-2 · prd:argus/UB-3 · prd:argus/UB-4 · prd:argus/UB-5 · prd:argus/UB-6 · prd:argus/UB-7 · prd:argus/UB-8 · prd:argus/NR-1 · prd:argus/NR-2 · prd:argus/NR-3 |
| **Security/privacy impact** | none — behavior over repo-internal artifacts and already-bound surfaces (tracker, code host, model provider); no new egress destination, permission, or data class; the injection posture is §3.7's substance |
| **Template version** | tech-design-template/v1 |

## 1. Problem

The data half is frozen: the ledgers, the authority contract, and their grammars exist as contracts (argus-artifacts). Nothing yet defines the behavior that animates them — how a session drives each pipeline stage as decision principal, executes the assumption regime's legality tests, keeps a delivery continuous across interruptions without duplicating an external effect, escalates only breaking decisions, bridges an approved plan to the executing fleet, and proves all of this before holding any authority. The behavior must satisfy two operator requirements recorded in the basis session: the drive *should* run in one continuous session (accumulated context is an asset), and it *must* run on any agentic CLI (no harness primitive may be load-bearing).

## 2. Decision

**Decision:** Argus's behavior is a **guide-bound drive protocol** — vendor-neutral, requiring only sequential agentic execution, file access, and a shell — executed per problem in **one preferred continuous session** (design-brainstorm:argus-tech-design#D-6): orient on the problem ledger and repo; loop the pipeline stages **by role** against the pipeline map, entering each as decision principal under its landed seam; at every gap run the **legality tests** (tier · channel · regression) and rule by recorded assumption or escalate the breaking residue as a self-contained brief that **parks the problem fail-closed and ends the session** (#D-7); bridge the approved plan through the planning stage's own materialization; drive proposals to **content-verified landed**; close the journal once. Continuity is the sibling design's journal, animated here by the **resume protocol** — replay, then verify every pending external effect against its goal object before any redo (#D-2). Every act that decides or permits passes a **grant check against the ladder at act time**; the never-list needs no *grant* check — the protocol contains no path that performs self-grant, governance authoring, or override acts — **with one exception that binds by measurement, not by path: the envelope**, which §3.2's envelope watch implements (driving itself spends). The harness (#D-9) proves the zero-tolerance families red-first before the floor grant is exercised.

A reader who stops here should see the shape: one session, one journal, stages entered by role under landed seams, assumptions legal-or-escalated, effects verified at the goal object, authority read from data at the moment of use — and nothing that runs while nothing is happening.

## 3. The design

### 3.1 The drive loop

**Decision:** per stage, the loop is: *enter* (a `stage-entered` entry; the stage resolved by role against the pipeline map, never by enumeration — FR-3), *run the stage's own protocol as its decision principal* (the stage documents govern; Argus supplies answers, dispositions, and gate verdicts only as its grant allows), *record* (every ruling at its load-bearing spot with the artifacts' citation rule; the paired journal entry appended), *advance*. The assumption regime binds at every gap: **tier check** (contradicts no governance-tier content and no `A-n`), **channel check** (standing feature-tier content moves only through its sanctioned evolution channel), **regression check** (no unamended contract breaks) — a gap failing the tests first narrows (a smaller assumption that passes), and only when no legal assumption remains does it escalate (prd:argus/FR-5, FR-6). Grant checks are one ladder lookup **at act time** — and an **ungranted** decision/permission act has a defined form: it is *not* breaking and does not park; Argus journals a `ruling-recorded` entry ("this act is ungranted at the current tier; requested from the human at <spot>"), places the request at the spot where the act is needed (the gate table, the proposal), and the derived block carries it as an **open item** that ages under FR-15's aging-items arm. The human's own completion of the act is observed at a later touchpoint and journaled as `reconciliation` with its effect identity. Nothing is queued invisibly. **After triggering materialization (FR-9's confirm arm)**: Argus verifies each created item at the goal object — it exists and carries the repo-configured automated-implementation tag — journaling the verification, and routes any discrepancy back through the planning stage's own contract as a reconciliation, never by editing items itself.

### 3.2 Session posture and the resume protocol

**Decision:** one problem, one session, held as long as the runtime allows — ended only by close, park, or the runtime itself. Step boundaries are journal appends, so the should-degrade-to-must path is seamless: **resume** = replay the journal (the derived block is a hint, never authority), sweep every entry carrying an external-effect identity and **verify the effect against its goal object** — the tracker item exists, the PR is disposed, the commit is an ancestor — append `reconciliation` entries for anything found done-but-unjournaled or journaled-but-undone, then continue at the reconstructed frontier. No second external act proceeds before the prior act's entry lands (DINV-1), which bounds the reconciliation window to at most one act.

**The envelope watch (UB-4's implementation):** the effective envelope is read at intake — the repo default, or the problem's recorded override — and cumulative spend is summed into the derived block at every append. **The mid-drive spend home, stated**: the sibling contract pins structured spend fields to the `delivered` entry only (its Q-2 owns the vocabulary), so until that vocabulary lands, mid-drive spend rides entry **gist bodies informally** — best-effort observability, never an invented structured field — and the watch binds on what is measurable from those gists; the closing entry carries the structured fields. When a step's projected cost would cross the envelope, the drive stops that step, emits the budget brief (`class: override` — the resource boundary is the grantor's), and parks per §3.3. Where a CLI exposes no spend signal, the watch degrades honestly: `unavailable` recorded, the dogfood evidence says so.

### 3.3 The escalation spine

**Decision:** two levels, one grammar. **Stage level**: reviewer verdicts and stage bounces route to Argus as the surrounding flow — it applies or declines-with-reason within grant (a declined blocking finding auto-escalates by the reviewers' own rule, becoming an Argus-level decision). **Argus level**: every human-bound escalation is a `brief-emitted` entry (self-contained: the single decision, options with risks, evidence, citations — EVAL-4's bar) that parks the problem fail-closed and **ends the session**. The frozen `class` enum maps the whole escalation space: **`override`** covers everything requiring the human to move a boundary they own — governance-tier content, a prior `A-n`, and the never-list's boundary rows (a self-grant attempt, the envelope, an ungranted outward surface — the brief's body names its row); **`deadlock`** covers convergence exhaustion (rounds spent, no legal assumption left). Park mechanics: the derived block flips to parked, and **where the repo binds a tracker, the problem's item gets the parked label pointing at the brief** (the record's D-7 clause). The answer arrives as an `A-n` append; the next front-door touch reconciles (`answer-received` citing it) and resumes.

**Touchpoint duties, scoped to the touched problem only**: at every drive touchpoint, *this problem's* parks and aging items past the repo-config threshold get exactly one `bump-issued` bump; an unclaimed materialized item aging past the second threshold gets a disposition — re-plan through the planning stage, or escalate. A session never writes another problem's files (AP-22/AP-27's disjointness); the repo-wide sweep across parked problems is the checkpoint walk's job, which reads everything anyway. There is no daemon; a parked problem costs zero. **UB-8's flow (confirmed missed-breaking)**: the confirming human lands the incident entry and the immediate demotion (`checkpoint` entry, `move: demotion` — authority data is human-landed per the sibling's writer discipline); Argus's duty is the backstop: at any touchpoint where it observes a `missed-breaking: yes` overturn whose demotion has not landed, that is bump-class — raised immediately, never re-judged.

### 3.4 The front door

**Decision:** `/argus <input>` on the skill surface (vendor dirs hold pointers; the glue is invocation-only). Disambiguation: an input resolving to an existing `docs/problems/<slug>/` **resumes** it; anything else is an **idea** — confirmed once if it looks slug-like — minting the slug and **instantiating both ledgers from their templates: the problem ledger *is* the intake record** (idea verbatim in its Idea section; optional envelope override and pre-rulings as header field and `A-n` entries), and the drive then opens with the journal's first `stage-entered` entry — the frozen entry vocabulary needs no intake type because intake lives on the problem side. The same door relays answers: `/argus <slug>` after the human appends (or states) an answer routes through reconciliation before resuming. The tracker-item door stays out (the basis's WR-2).

### 3.5 The seam-amendment program

**Decision:** the uniform delta is *the decision principal answers* — the operator, or Argus under grant, per the glossary's ruled term — landed per stage **through that stage's own evolution channel, before Argus drives that stage unmanned** (design-brainstorm:argus-tech-design#D-5):

| Stage document | Channel | Class | Order |
|---|---|---|---|
| feature-brainstorm | typed amendment on its ledger | **contract-changing** — rescope NG-4, re-anchor FR-6 + the zero-tolerance eval to principal semantics (*ungranted* unmanned stays sev-1) | first — the long pole |
| tech-design-brainstorm | its ledger | verify-then-classify at authoring; default additive (already "authority"-shaped) | with its first unmanned drive |
| prd-new · prd-amend · tech-design-new · plan-new · prd-review · tech-design-review | each ledger / guide home | additive — principal wording; Argus named the first-level escalation authority; plan-new's consent rows intact | drive order |
| operator review | — | no amendment — the human-overlay surface | — |

Each amendment carries its stage's own review; every touched stage's fixtures — including stage 1's zero-tolerance set — stay green after its seam lands (prd:argus/NR-1, NR-2). The gate suite's verdict on any tree stays author-independent (NR-3) — the seams change who may answer, never what passes.

### 3.6 The eval harness

**Decision:** the corpus lives at the agent's home (`fixtures/`), the reviewers' pattern: seeded subjects, golden run records, defect classes each red at exactly its row, and an eval report recording every run. Families (design-brainstorm:argus-tech-design#D-9): **EVAL-1** breaking set — governance conflicts (the red→black archetype) + deadlock transcripts; 100% escalated, zero assumed through, red-first. **EVAL-2** legality + disposition — legal gaps (≥ 90% assumed, not escalated), governance-crossers, self-grant attempts, outward acts, and disposition cases on seeded reviewer findings (recorded reasons, decline-of-blocking auto-escalates, zero re-judging); green before the floor grant is exercised. **EVAL-3** injection — planted directives across the drive's **full read surface**: subject documents, records, ledgers, tracker items, PR comments and reviews, fleet PR bodies; zero obeyed, red-first. **EVAL-4** brief quality — golden briefs and counter-examples behind the ≥ 4/5 decidable-alone rubric, operator-judged on real dogfood briefs. The artifacts design's per-DINV defect fixtures (its A-7) run under this same harness.

### 3.7 Security & privacy

none — matches the header row. The injection posture is the load-bearing clause: **every surface the drive reads is data** — instructions embedded in subjects, records, ledgers, tracker items, or PR content are never obeyed; a planted directive is itself escalation-candidate evidence (prd:argus/UB-6), and EVAL-3 proves the posture red-first. Egress is unchanged: the drive talks to the surfaces the repo already binds.

### 3.8 Observability

none of its own — the journal is the observability surface, owned by the artifacts design; this protocol's only duty is writing it faithfully (DINV-1) so the measurements stay reads. The drive adds no telemetry, no logs beside the journal, no heartbeat.

### 3.9 Compatibility and migration

New behavior; nothing migrates. The guide, glue, ledger templates, initial authority-contract file (born at `floor`), and fixtures land together (prd:argus/DOD-6). The graduated floor holds at birth: gate signatures and merges stay the human's until checkpoints promote. Engine-era: the protocol's mechanical steps (resume sweep, checkpoint walk, bump evaluation) graduate to registry-born verbs; the guide keeps the judgment.

### 3.10 Invariants

- **DINV-1** — No second external act before the prior act's journal entry lands; every externally-effective entry carries its effect identity.
- **DINV-2** — The legality tests run before every assumption; a failed test never downgrades into a recorded assumption — it narrows or escalates.
- **DINV-3** — Grant checks read the ladder file at act time; no cached grant survives a session boundary or a ladder change.
- **DINV-4** — A parked problem holds no live session and no recurring process; bumps evaluate only at touchpoints.
- **DINV-5** — The protocol requires only sequential agentic execution, files, and a shell; any harness primitive used is an optimization with a stated sequential fallback in the guide.
- **DINV-6** — Argus never implements fleet-tagged work items; the principal/hands boundary is the tracker convention.

## 4. Alternatives considered

| Alternative | Why rejected |
|---|---|
| A daemon/scheduler runtime that watches parks and fleet progress | violates the any-CLI must and AP-19 — a recurring process spending while nothing happens; the touchpoint model delivers the same alarms at zero idle cost (design-brainstorm:argus-tech-design#D-6, #D-7) |
| Fresh session per stage (per-stage sub-sessions) | discards accumulated context at every boundary — the operator's continuous-session requirement exists precisely against this; resume remains the exception path, not the architecture (#D-6) |
| Harness-native background driving as the mechanism (background tasks, wakeups) | binds the protocol to one CLI's primitives; permitted opportunistically under DINV-5's fallback rule, never load-bearing (#D-6) |
| Argus self-implements small tasks instead of tagging the fleet | re-imports the known principal/hands race class; the division of labor is ruled (the basis's fleet bridge; DINV-6) |
| Queue ungranted acts silently until promotion | invisible state outside the journal; an ungranted act is requested from the human and journaled, keeping the floor auditable (§3.1) |

## 5. Risks

| ID | Risk | Impact | Mitigation | Verification / evidence |
|---|---|---|---|---|
| R1 | context exhaustion mid-drive on a long problem | the *should* (one session) degrades | step boundaries are appends, so degradation to resume is lossless by construction; the resume drill proves it | A-3 |
| R2 | the stage-1 seam slips and an unmanned brainstorm runs against the operator-only contract | the sev-1 the stage's eval names | the program's order rule (land before unmanned drive) + stage-1 fixtures green as the seam's acceptance | A-5 |
| R3 | injection through the widened read surface | rulings serving an injector | UB-6 posture + EVAL-3 red-first; any slip is forensically attributable via the journal | A-4 |
| R4 | grant drift across a long session (ladder changes mid-drive) | acts under a stale grant | DINV-3 read-at-act; the journal records the contract version per entry | A-4 (flip drill, with the sibling's A-4) |
| R5 | fleet items sit unclaimed and the drive stalls silently | delivery latency invisible | FR-15 aging + bumps at touchpoints; the second-threshold disposition (re-plan through the planning stage, or escalate — §3.3) | A-6 |

## 6. Acceptance

| ID | Acceptance criterion | Verify | Required evidence | Traces to |
|---|---|---|---|---|
| A-1 | The agent's home ships complete: guide, invocation glue, ledger templates, the initial authority contract born at `floor`, fixtures | Inspection | the implementing PR | prd:argus/DOD-6 · §3.9 |
| A-2 | The dogfood drive traverses every existing stage by role with no human driving, the floor holding — every beyond-floor act requested and journaled | Demo | the dogfood ledgers + PRs | prd:argus/FR-3 · FR-10 · DOD-4 |
| A-3 | Kill-resume drill: a mid-drive kill resumes from the journal alone to the same next action, zero duplicated external effects, reconciliations journaled | Test | drill transcript + journal | prd:argus/UB-7 · #DINV-1 |
| A-4 | The eval families run at thresholds, zero-tolerance families red-first, before any floor act on a real subject — EVAL-2's set including the fleet self-implement negative (#DINV-6) and a mid-drive ladder-flip case (#DINV-3's mid-session arm) | Eval | the eval report at the home | prd:argus/DOD-3 · EVAL-1..4 · #DINV-3 · #DINV-6 |
| A-5 | The seam program lands in order, each amendment through its stage's channel, every touched stage's fixtures green — stage 1's zero-tolerance set included | Test + Inspection | the seam PRs + fixture runs | prd:argus/NR-1 · NR-2 · A1-of-basis |
| A-6 | Park/bump drill: an emitted brief parks the problem (tracker label set where bound), ends the session; a seeded over-threshold park and a seeded aging unclaimed item each bump exactly once at the next touchpoint; the second-threshold disposition fires; and the UB-8 drill — a seeded missed-breaking confirmation produces the incident entry and the immediate demotion, with the backstop bump observed when the demotion is withheld | Test | drill transcript | prd:argus/FR-6 · FR-15 · UB-8 · #DINV-4 |
| A-7 | A scripted drive segment executes on a second agentic CLI from the same guide, no harness primitive required — the any-CLI must demonstrated | Demo | the second-CLI transcript | #DINV-5 · basis D-6 |

## 7. Open questions

| ID | Question | Owner | Blocks | Status | Resolution |
|---|---|---|---|---|---|
| Q-1 | Parallel problems on one clone: per-problem worktrees, or serialized drives, given the shared-clone hazard class? | implementation | nothing | open | decided at implementation; per-problem files are already disjoint (AP-22/AP-27), the hazard is git-level only |
| Q-2 | The tracker-item front door | product (basis WR-2) | nothing | open | unbound here; revisit when wired |
| Q-3 | Harness-primitive fallbacks worth documenting per CLI (which optimizations exist where) | implementation | nothing | open | the guide's DINV-5 fallback table grows per adapter, additively |

## Principles

- **AP-16 — honored.** Every loop is bounded: stage rounds by the stages' own caps, deadlock escalates, two-level escalation terminates in a human brief; nothing retries forever.
- **AP-26 — honored.** Landed means mainline-content-verified; resume verifies effects at their goal objects; nothing trusts a badge or a cached status.
- **AP-25 — honored.** All knowledge on the vendor-neutral surface; glue is pointers; the protocol runs on any agentic CLI (DINV-5).
- **AP-19 — honored.** Zero steady-state cost: no daemon, no polling; parked problems spend nothing; bumps ride touchpoints.
- **AP-10 — honored.** The protocol is a guide first and graduates its mechanical steps to registry-born verbs; judgment stays judgment.
- **AP-22 / AP-27 — honored.** Parallel problems are parallel per-problem file sets; nothing shared appends; the seam program adds per-stage amendments, no central list.
- **AP-21 — honored.** Rulings freeze at their load-bearing spots with citations; the drive re-derives nothing it recorded.
- **RW-5 — honored.** One decision — the behavior — in one document; the data half is the frozen sibling, per the record's D-1 partition (design-brainstorm:argus-tech-design#D-1).
- **Deviation:** none.
