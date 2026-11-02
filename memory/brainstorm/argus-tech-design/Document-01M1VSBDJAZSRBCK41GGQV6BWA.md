# Argus (the AI Engineer) — design-brainstorm record

| Field | Value |
|---|---|
| **In one line** | Solution-space session for the merged argus contract: how the delegated-authority agent is actually built — design decomposition, drive continuity (delivery ledger + resume), the authority-contract data shape, the problem/delivery ledger document classes, the per-stage seam-amendment program, front door, brief/park/alarm mechanics, measurement fields, and the eval harness. |
| **Relevant-when** | designing or changing Argus's runtime, ledgers, authority-contract data, seam amendments, or eval harness; checking prior design rulings or refused shapes about the AI Engineer's mechanics |
| **Basis** | prd:argus (merged ledger — effective as of 000) |
| **Authority** | @mdarabi |
| **Date(s)** | 2026-07-10 · 2026-07-11 |
| **Status** | gated: ready-for-design |
| **Tracking** | — |
| **Feeds** | docs/tech-design/argus-artifacts.md · docs/tech-design/argus-agent-v1.md |
| **Prior records** | brainstorm:autonomous-developer (the stage-1 basis record — its parked technical constraints are this session's direct inputs) · design-brainstorm:reviewer-artifacts-tech-design (the stage-3 precedent) |
| **Template version** | design-brainstorm-record/v1 |

## Orientation

| Source | Why loaded |
|---|---|
| docs/prds/argus/effective.md | the basis: FR-1..15, NR-1..3, UB-1..8, EVAL-1..4, DoD, §4.2's seam-sequencing rule, §6.5's authority-graduation rollout |
| docs/brainstorms/autonomous-developer.md | the stage-1 record: **parked technical constraints** (runtime shape — session vs engine-hosted; fleet-seam adapter boundary) are this session's direct inputs; D-8 binds the bootstrap form (resumable session, files-as-truth), D-9 the home |
| docs/tech-design/plan-new-v1.md | the materialization contract FR-9 defers to (consent, idempotency, wire-before-route); the deterministic-core + judgment-skill precedent |
| docs/tech-design/prd-review-artifacts.md · tech-design-review-artifacts.md | artifact-contracts-as-their-own-design precedent; **calibration-as-data at the skill home** — the authority contract's closest cousin; report template classes + structure checklists |
| docs/tech-design/brainstorm-record-model.md · design-brainstorm-record-model.md | template-versioned session-artifact classes with citation grammars — the ledgers' precedent (typed sections, dense anchors, freeze semantics) |
| docs/tech-design/distributed-check-registration.md | any argus gate checks are born into the registry (stage-1 record F-8); no hand enumerations |
| docs/architecture-principles.md | AP-10 (codify over prompt), AP-16 (bounded loops), AP-19 (steady-state cost), AP-21 (judge once, freeze), AP-22 (raceable ops), AP-25 (vendor-neutral surface), AP-26 (goal-state by content), AP-27 (extension by addition) |
| docs/observability/incidents.md | I2 (badge-vs-mainline — already baked into FR-10/EVT-3); the injection classes behind UB-6/EVAL-3 |
| docs/wiki/glossary.md | the eight ruled Argus terms — this session must not drift from them |
| .pyxis/pipeline.md | the operational stage map the drive loop addresses **by role** (FR-3) — the projection the delivery ledger's stage-position field points into |

**Overlaps surfaced:** no live design serves prd:argus (new ledger). Adjacent deciders honored, not re-litigated: plan-new-v1 owns materialization (FR-9 already defers); the reviewer-artifacts designs own the report classes (briefs may *mirror* the embedded-brief shape, never bind it); solution-shape separation (separate-at-rest) applies if a design here carries a shape. None covers the agent or its artifacts — the ground is open.

## Design questions

- **DQ-1**: How many design documents carry the argus contract, and what partitions them?
- **DQ-2**: What is the drive's continuity mechanism — what exactly does the delivery ledger hold, and how does a fresh session resume a mid-flight problem without loss or duplication (FR-2, UB-7)?
- **DQ-3**: What is the authority contract's data shape — grants, tier states, checkpoint evidence — and how is enforcement bound to it in the bootstrap, before any engine exists (FR-11, FR-14, AP-10)?
- **DQ-4**: What document classes are the problem ledger and the delivery ledger — template-versioned classes like the records, or freer forms — and what citation grammar anchors agent rulings to them (FR-1, FR-8, FR-12)?
- **DQ-5**: What is the seam-amendment program — per stage: additive step or contract-changing amendment, in what landing order against the graduated floor (§4.2's sequencing rule, NR-2)?
- **DQ-6**: What is the front door concretely — the invocation, the session bootstrap, the agent's home on the vendor-neutral surface (D-9)?
- **DQ-7**: How do briefs, parks, and FR-15's visibility bump materialize — the brief's format in the delivery ledger and the tracker-surface convention?
- **DQ-8**: How do the EVT fields and M1–M3 computation materialize on the artifacts in the bootstrap (fields + hand-walk now, engine verb later)?
- **DQ-9**: What harness proves EVAL-1..4 — the fixture corpus shape at the agent's home, mirroring the reviewers' pattern?

## Panel log

- **DQ-1**: extends (the design-corpus partitioning convention) — skipped — a decomposition ruling with inline options; no structural shape to diverge on
- **DQ-2**: opens (the drive-continuity protocol + the delivery-ledger artifact class) — fired — three orthogonal shapes: snapshot-state, append-journal, stateless-observe
- **DQ-3**: opens (the authority-contract policy-data class) — fired — three orthogonal shapes: flat grant table, first-class tier ladder, stage×action matrix
- **DQ-4**: extends (the template-versioned session-artifact pattern — records and reports) — skipped — the basis's FR-1/FR-8/FR-12 field, accretion, attribution, and citability requirements leave no structurally distinct alternative to versioned template classes; the open sub-questions are parameters (homes, anchor families, grammar), presented as a package
- **DQ-5**: extends (each stage document's own amendment channel — the mechanism exists; the question is per-stage classification and order) — skipped — a program ruling, not a shape
- **DQ-6**: extends (the skill-surface pattern — glue + vendor-neutral guide + data at the skill home) — skipped — home and invocation parameters on the established surface; the authority added two shaping requirements (continuous session *should*; any-CLI *must*), recorded in D-6
- **DQ-7**: extends (the reviewers' embedded-brief shape, mirrored; FR-15's ruled touchpoint model) — skipped — format and convention parameters on ruled ground
- **DQ-8**: extends (journal-entry fields as the event surface; the checkpoint-evidence pattern of D-3's ladder history) — skipped — one genuine detail surfaced (post-close overturns vs the frozen ledger), resolved inside the package by citation, not by breaking freeze
- **DQ-9**: extends (the reviewers' fixture-corpus + eval-report pattern at the skill home) — skipped — family contents and thresholds are basis-bound (EVAL-1..4); the one widening is the injection corpus's surface list

## Decisions

### D-1 — two designs: behavior and data

- **Question**: how many design documents carry the argus contract, and what partitions them?
- **Answer**: **two** — `argus-agent-v1` (behavior: the drive protocol, continuity mechanics, the seam-amendment program's shape and ordering, the escalation spine, the front door, the eval harness) and `argus-artifacts` (data contracts: the problem-ledger and delivery-ledger template classes with their citation grammar, the authority-contract schema — grants, tiers, checkpoint evidence — and the EVT measurement fields).
- **Reasoning**: the partition line is what-freezes versus what-iterates — the artifacts are what other things cite and parse (template-versioned, freeze-hard); the agent iterates on dogfood evidence. The reviewer pair held exactly this way. The seam program is no third document: its amendments land on other features' ledgers, each reviewed there; the agent design carries only the program's shape.
- **Anchor**: 2026-07-11 · t2
- **Status**: live

### D-2 — continuity: the append-only journal, resuming by observation

- **Question**: DQ-2 — what is the drive's continuity mechanism?
- **Answer**: **ALT-2, absorbing ALT-3's posture as the resume rule** — the delivery ledger is an append-only journal of typed, timestamped, attributed entries, each carrying the identity of any external effect it caused; a derived current-state block regenerates on append; resume = replay, then **verify every pending external effect against its goal object before any redo** (observe first, append the reconciliation, then proceed). ALT-1 rejected.
- **Reasoning**: recommendation adopted — the house ledger grammar (AP-27); FR-12 attribution and FR-15 ages ride entries for free; duplication dies by observe-before-redo (AP-26 — I2's remedy); the derived block is bootstrap hand-discipline with a docgate-style verb later (AP-10 arc).
- **Anchor**: 2026-07-11 · t3
- **Status**: live

### D-3 — the authority contract: a first-class tier ladder; the never-list stays out of the data

- **Question**: DQ-3 — what is the authority contract's data shape, and how is enforcement bound in the bootstrap?
- **Answer**: **ALT-5 with ALT-4's action-class vocabulary as the bundles' contents** — one versioned file at the agent's home declaring the ordered tier ladder (each tier a named bundle of action classes), one `current-tier` pointer, an append-only tier-change history with evidence refs (EVT-5's home), and the `revoked` below-floor tier as the kill switch. **The never-list is not in this file** — D-3-of-the-basis's structural rows are frozen design content in `argus-artifacts`, unflippable by construction; the data file carries only the graduated ladder. Format: versioned markdown table (the calibration precedent), closed-parse when the engine reads it.
- **Reasoning**: recommendation adopted — §6.5's rollout table becomes the data itself (AP-21); integrity is structural (no representable state outside the ladder; promotions are single visible pointer moves); FR-11's flip-a-row fit and the revocation kill switch operate on exactly this file; ALT-6's flexibility is unconsumed by the basis.
- **Anchor**: 2026-07-11 · t4
- **Status**: live

### D-4 — the ledger classes: two versioned templates, sibling instances, journal-anchored citations

- **Question**: DQ-4 — the problem/delivery ledger document classes and citation grammar (parameter package).
- **Answer**: adopted — (1) two template classes, `problem-ledger/v1` and `delivery-ledger/v1`, contracts bound in `argus-artifacts`; (2) instances at `docs/problems/<slug>/` as siblings `problem.md` + `delivery.md`; (3) anchors: **`A-n`** human authority entries (pre-rulings, accreted answers) cited `problem:<slug>#A-n`; **`E-n`** typed journal entries cited `delivery:<slug>#E-n`, briefs as typed entries; **an assumption's load-bearing spot cites its journal entry** — the shaped artifact carries `(delivery:<slug>#E-n)`; (4) problem ledger append-only forever; delivery ledger append-only, closing at delivery (the EVT-3 entry) and freezing; the derived state block regenerates on append, never anchored or cited.
- **Reasoning**: the record/report grammar applied without invention; the one new move — artifact-cites-journal-entry — gives FR-4's audit its two-way trace mechanically.
- **Anchor**: 2026-07-11 · t5
- **Status**: live

### D-5 — the seam-amendment program: one contract-changing amendment, additive elsewhere, drive order

- **Question**: DQ-5 — per-stage classification and landing order for the principal seam.
- **Answer**: adopted — **feature-brainstorm**: contract-changing amendment (rescope NG-4; re-anchor FR-6 and the zero-tolerance eval to decision-principal semantics — *ungranted* unmanned stays the sev-1; fixtures re-anchored, never deleted). **tech-design-brainstorm**: verify-then-classify at authoring, defaulting additive (already "authority"-shaped). **prd-new / prd-amend / tech-design-new / plan-new / both reviewers**: additive steps (principal wording; Argus named first-level escalation authority — the reviewers' "surrounding flow"; plan-new's consent rows intact, floor tier requests the human). **doc-review**: no amendment (the human-overlay surface). **Order**: stage 1's amendment first, additive seams in drive order; §4.2's land-before-unmanned-drive holds per stage. **Binding surfaced**: the initial ladder bundles enumerate the full action-class vocabulary the drive exercises (including `trigger-materialization`), not only §6.5's headline four.
- **Reasoning**: honest per-contract classification — only stage 1 binds operator-only in FR/NG/EVAL rows; the vocabulary binding closes the gap between §6.5's headlines and FR-11's grant checks.
- **Anchor**: 2026-07-11 · t6
- **Status**: live

### D-6 — the front door and home, under two operator requirements: continuous session (should), any agentic CLI (must)

- **Question**: DQ-6 — front door, home, and session bootstrap.
- **Answer**: the package adopted (skill glue at the skills home, invocation-only; drive guide on the vendor-neutral authoring surface; authority contract + fixtures at the skill home; `/argus <idea>` verbatim intake with optional envelope/pre-rulings; `/argus <slug>` resumes; ledgers instantiated at bootstrap; answers as `A-n` appends reconciled by the journal; tracker door stays WR-2) — **shaped by two operator requirements**: (1) **single-session-preferred** — "argus should run in one session continuously to ensure the accumulated context would not get lost or rebuilt (not a must, but should)": the drive holds one continuous session through as much of a delivery as the runtime allows; the D-2 journal is the safety net making interruption, crash, and park lossless (the must under the should). Parks suspend the session by design — a parked problem holds no live session (FR-15's touchpoint model, zero idle spend). (2) **any agentic CLI** — "argus should be executed on any agentic cli": the drive protocol assumes only sequential agentic execution, file access, and a shell; all knowledge and logic live on the vendor-neutral surface, vendor dirs hold pointers only (AP-25); harness-specific conveniences (background tasks, wakeups) may be used opportunistically, never required.
- **Reasoning**: context accumulation is an asset the protocol should protect (rebuilding it costs tokens and fidelity); portability is the house posture made binding at design level — the journal, not any harness primitive, is the portable continuity mechanism; step boundaries = journal appends, so the should-degrade-to-must path is seamless.
- **Anchor**: 2026-07-11 · t7
- **Status**: live

### D-7 — briefs, parks, and the alarm

- **Question**: DQ-7 — the brief format, park mechanics, answer/wake path, and the FR-15 bump.
- **Answer**: adopted — (1) a brief is a typed `brief-emitted` journal entry mirroring the reviewers' embedded-brief shape (the single decision, options with risks, evidence, citations), self-contained per EVAL-4; (2) park flips the derived state block to `parked (brief E-n, since <date>)` and **the session ends** — no live session while parked; where the repo binds a tracker, the problem's item carries a parked label pointing at the brief; (3) the answer appends as `A-n`; the wake is the next front-door touch (`/argus <slug>`), and the journal reconciles (`answer-received` citing the `A-n`) before resuming; (4) at any drive touchpoint, parks and aging items past the repo-config threshold get exactly one visibility bump (FR-15).
- **Reasoning**: reuses the shipped brief grammar (mirrored, never bound — the reviewers' design owns theirs); park-suspends-session closes D-6's continuity posture against FR-15's zero-idle-cost model.
- **Anchor**: 2026-07-11 · t8
- **Status**: live

### D-8 — measurement: journal entries as events, reverse-citation for overturns, the checkpoint walk

- **Question**: DQ-8 — how EVT fields and M1–M3 computation materialize.
- **Answer**: adopted all — (1) **EVT rows are journal entries**, no second event surface: EVT-1 = `assumption-recorded`, EVT-2 = the `brief-emitted`/`answer-received` pair, EVT-3 = the closing `delivered` entry (content-verified), EVT-5 = the ladder's change history; (2) **post-close overturns cite in**: the overturning artifact carries `overturns delivery:<slug>#E-n` — overturn events are discoverable by reverse-citation into closed ledgers, and frozen files never change; (3) **M-computation = the checkpoint hand-walk** over closed ledgers, its output appended to the authority contract's change history as the checkpoint evidence block — exactly what FR-14's promotions consume; a registry-born verb takes the walk over in the engine era (AP-10); (4) **spend** rides the close entry, adapter-honest (`unavailable` where the CLI exposes nothing, never invented).
- **Reasoning**: one event surface means nothing drifts; freeze semantics stay intact via citation direction; evidence lands where promotion decisions read it.
- **Anchor**: 2026-07-11 · t9
- **Status**: live

### D-9 — the eval harness: the reviewers' pattern, injection surface widened

- **Question**: DQ-9 — the fixture corpus and eval families.
- **Answer**: adopted — the corpus at the skill home mirroring the reviewers (seeded subjects, golden run records, defect classes each red at its row; an eval report at the home). Families: **EVAL-1** breaking set — governance conflicts (red→black archetype) + deadlock transcripts, 100% escalated, red-first; **EVAL-2** legality + disposition — legal gaps assumed ≥ 90%, never-list refused 100%, disposition arms with decline-of-blocking auto-escalate and zero re-judging, green before the floor grant; **EVAL-3** injection corpus **widened to the drive's full read surface** — subjects, records, tracker items, PR comments/reviews, fleet PR bodies — zero obeyed; **EVAL-4** brief-quality goldens + counter-examples behind the ≥ 4/5 rubric, operator-judged on dogfood briefs. Runner: scripted first runs by the implementing agent; zero-tolerance families red-first per DOD-3.
- **Reasoning**: the proven harness shape; the one widening matches the drive's larger read perimeter — every surface Argus reads is an injection surface.
- **Anchor**: 2026-07-11 · t10
- **Status**: live

## Alternatives

### ALT-1 — the checkpoint ledger (in-place state snapshot)

- **Shape**: the delivery ledger is a live status document the session updates in place — stage-position field, open-items table, escalation states; resume = read the snapshot, continue from it.
- **Trade-offs**: current state readable in one look; but in-place mutation erases history (attribution and audit need a second mechanism), and "same next action" depends on the snapshot being written at every step boundary — a crash between an external act and its write means the act repeats on resume (UB-7's exact hazard).
- **Principles**: AP-21 strained (rulings should freeze, not be overwritten); AP-26 unaddressed (resume trusts the snapshot, never the goal object); AP-19 honored (cheap reads).
- **Incidents**: I2-class exposure — trusting recorded status over observed content.
- **Verdict**: rejected — I2's trust-the-status shape; cannot carry FR-12 attribution without growing a journal anyway (D-2)
- **From**: DQ-2 · panel

### ALT-2 — the append-only journal with a derived state block

- **Shape**: the delivery ledger is append-only typed entries (stage-entered, ruling-recorded, item-materialized, brief-emitted, answer-received, proposal-disposed …), each timestamped, attributed, and carrying the identity of any external effect (item id, PR number); a small derived current-state block is regenerated on append; resume = replay (or read the derived block), then **verify every pending external effect against its goal object before redoing it**.
- **Trade-offs**: history, attribution (FR-12), and ages (FR-15) ride the entries for free; duplication dies by verify-before-redo; matches the house ledger grammar. Cost: the derived block is regeneration discipline — hand-kept in bootstrap, a docgate-style verb later (the AP-10 arc).
- **Principles**: AP-27 (where order carries meaning, a ledger *is* the design); AP-21 honored (entries freeze); AP-26 honored (effect identity + goal-object verification); AP-19 honored (append-time work only).
- **Incidents**: I2 matched — resume observes the tracker/mainline instead of trusting recorded status.
- **Verdict**: **chosen** — with ALT-3's observe-before-redo absorbed as the resume rule (D-2)
- **From**: DQ-2 · panel

### ALT-3 — external state as truth (the stateless drive)

- **Shape**: no ledger state beyond a pointer; each session re-derives position by observing the pipeline artifacts themselves — which stage documents exist, which items are open, which proposals pend.
- **Trade-offs**: zero duplication by construction (pure observe-before-act); nothing to corrupt; but re-orientation is expensive every session, and the basis *binds* ledger content existing (FR-2's "from the ledger alone", FR-15's exposed ages, the EVT fields) — some state (a brief's age, a mid-stage frontier) has no artifact home elsewhere.
- **Principles**: AP-26 pure; AP-19 strained (recurring re-orientation cost); conflicts with the basis's FR-2/FR-15/EVT as the *sole* mechanism.
- **Incidents**: none checked beyond I2 (registry as of 2026-07-11) — the observe posture is I2's remedy generalized.
- **Verdict**: hybridized into ALT-2 — the verify-against-goal-object posture becomes the resume rule; rejected as sole mechanism (the basis binds ledger content existing) (D-2)
- **From**: DQ-2 · panel

### ALT-4 — the flat grant table

- **Shape**: one versioned file at the agent's home; rows = action classes (dispose-advisory, dispose-blocking, sign-gate-verdict, rule-acceptance, approve-merge, …), each `granted | escalate`; an append-only change-history section carries evidence refs; kill switch = a master row or setting every row to escalate.
- **Trade-offs**: maximally simple reads and flips; but the §6.5 ladder is implicit — a promotion flips several rows, and nothing structural prevents out-of-order flips (promote-slow becomes discipline, not shape).
- **Principles**: AP-10 honored (policy as data); AP-21 strained (the ruled ladder lives only in the PRD, re-derived at every change); D-7's promote-slow posture unprotected structurally.
- **Incidents**: none checked (registry as of 2026-07-11); gen-1's risk-tiered lanes are the cousin precedent.
- **Verdict**: hybridized into ALT-5 — its action-class vocabulary becomes the bundles' contents; rejected as the top-level shape (ladder integrity by discipline only) (D-3)
- **From**: DQ-3 · panel

### ALT-5 — the tier ladder, first-class

- **Shape**: the file declares the **ordered tier ladder** — each tier a named bundle of action classes (floor: advisory dispositions → +blocking dispositions & gate verdicts → +acceptance → +merge approval), plus one `current-tier` field and an append-only tier-change history (direction, evidence refs — EVT-5's home). Grant check = "is this action's class within the current tier's bundle?" Kill switch = a below-floor tier `revoked` (everything escalates). Bundles and ladder are themselves data, amendable by PR.
- **Trade-offs**: the PRD's §6.5 rollout table *is* the data — one pointer moves per change, integrity is structural (no state outside the ladder, no skipping without it being visible as pointer jumps); per-action exceptions require re-cutting bundles (a deliberate, visible act rather than a quiet flip).
- **Principles**: AP-21 honored (the ruled ladder frozen as data, judged once); AP-10 honored; AP-19 honored (a read is one lookup); D-7's promote-slow structurally enforced.
- **Incidents**: none checked (registry as of 2026-07-11); mirrors gen-1's risk-tier lanes shape.
- **Verdict**: **chosen** — with ALT-4's vocabulary inside the bundles; the never-list stays out of the data file (D-3)
- **From**: DQ-3 · panel

### ALT-6 — the stage×action matrix

- **Shape**: grants keyed by (stage, action class) — e.g., gate verdicts granted at stage 3 but not stage 1; the full combinatorial surface as data.
- **Trade-offs**: maximum flexibility; but the basis graduates by **action class**, not by stage (§6.5, D-7), so the flexibility is unconsumed; combinatorial rows make integrity and audit harder for no ruled need.
- **Principles**: AP-10 nominally honored; strains the basis's own graduation model; YAGNI against §6.5.
- **Incidents**: none checked (registry as of 2026-07-11).
- **Verdict**: rejected — the basis graduates by action class, not stage; the combinatorial surface is unconsumed (D-3)
- **From**: DQ-3 · panel

## Seam map

- **S-1**: the delivery-ledger document class (append-only journal + derived state block) — opens — behind it: per-problem drive state, FR-2/FR-15 ages, the EVT fields — feeds: argus-artifacts Scope
- **S-2**: the drive-continuity protocol (replay + verify-before-redo against goal objects) — opens — behind it: session resume, UB-7's no-loss/no-duplication — feeds: argus-agent-v1 Scope
- **S-3**: the authority-contract policy-data class (tier ladder + current-tier pointer + change history) — opens — behind it: FR-11 grant checks, FR-14 promotions/demotions, the revocation kill switch — feeds: argus-artifacts Scope
- **S-4**: the problem-ledger document class + the two-grammar citation surface (`problem:<slug>#A-n` · `delivery:<slug>#E-n`, artifacts citing journal entries) — opens — behind it: FR-1 intake, FR-8 accretion, FR-4's audit trace — feeds: argus-artifacts Scope
- **S-5**: the front door (`/argus` idea-or-slug invocation on the skill surface) — opens — behind it: FR-1 intake, resume, answer relay — feeds: argus-agent-v1 Scope
- **S-6**: the per-stage principal seam — extends (each stage document's contract, through its own amendment channel; classification and order per D-5) — behind it: FR-3's drive-by-role, NR-1/NR-2 — feeds: argus-agent-v1 Scope + the touched stage ledgers

## Facts

- **F-1**: the basis pins the bootstrap form already — a long-running agent session per problem, resuming via files-as-truth; the runtime-shape question left open by the stage-1 record is *session-mechanics*, not session-vs-engine — source: prd:argus FR-2/UB-7 + brainstorm:autonomous-developer#D-8, parked constraints
- **F-2**: the authority contract must be flippable data — FR-11's fit is "flipping a grant row in data changes the behavior with no prose change", and grant revocation is the tested kill switch (§6.5) — source: prd:argus FR-11, §6.5
- **F-3**: stage 1's seam amendment is contract-changing by the basis's own text (rescoping the operator-only binding), and each stage's seam must land before Argus drives that stage unmanned — source: prd:argus §4.2, NR-2, A1
- **F-4**: the calibration-as-data precedent lives at the reviewer skill homes (versioned table, severity rows cited by ID, changed only by PR) — the closest shipped cousin to the authority contract — source: the reviewer artifact designs + skill homes
- **F-5**: briefs are embedded in the delivery ledger per the basis's F2 flow; the reviewers' escalation brief (single decision, options with risks, evidence, citation) is the shipped format precedent — source: prd:argus §4.3 F2 + the reviewer protocol guides

## Fog

none *(session open)*

## Out of scope

- **O-1**: engine-hosted runtime — session-form only in this pair; the engine era supersedes (from D-6)
- **O-2**: any second event/telemetry surface — journal entries are the events; nothing else emits (from D-8)
- **O-3**: binding the reviewers' brief format — mirrored, never bound; their design owns it (from D-7)
- **O-4**: a doc-review seam amendment — the human-overlay surface is untouched (from D-5)
- **O-5**: the tracker-item front door — WR-2 of the basis, unbound here (from D-6)

## Candidate glossary terms

- **T-1**: drive touchpoint — any moment a live Argus session acts on a problem; where FR-15's bump thresholds evaluate · avoid: heartbeat (implies a daemon that does not exist)
- **T-2**: checkpoint walk — the bootstrap computation over closed ledgers that produces a checkpoint's evidence block on the authority contract · avoid: telemetry (nothing is emitted; everything is read from artifacts)

## Parked product gaps

none *(session open)*

## Gate

| Field | Value |
|---|---|
| **Verdict** | ready-for-design |
| **Date** | 2026-07-11 |
| **Authority** | @mdarabi |
| **Covering design** | — |
| **Coverage check** | pass — core four covered, zero blocking fog |
| **Amend route** | — |
