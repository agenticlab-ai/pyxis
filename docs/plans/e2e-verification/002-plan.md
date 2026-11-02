# e2e authoring line — the harness becomes a buildable machine — plan 002

| Field | Value |
|---|---|
| **Feature** | e2e-verification |
| **PRD** | docs/prds/e2e-verification |
| **Design** | docs/tech-design/e2e-verification-artifacts-v2.md |
| **Plan version** | 002 |
| **Dialect** | harbormaster/fs-spec:v1 |
| **Epic title** | Make the e2e design doc carry the machine that proves the coverage — the e2e-design/v2 slot form and its checker, harness reuse by judgment, the decision-authority stand-in, declared per-scenario cost, a third review lens, and a plan that lands conventions before it fans out |
| **Status** | Draft — freezes at merge |
| **Template version** | plan-template/v1 |

## Tasks

### T1 — The e2e-design/v2 class and its well-formedness checker

| Field | Value |
|---|---|
| **Blocked by** | — |
| **Size band** | 250–300 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | e2e-design-template, internal/e2e/docclass |

**Goal:** an e2e design doc is mechanically well-formed against the v2 contract, or the checker names exactly which rule it broke.

**Current behavior:** the class is the predecessor version — six sections, its harness part one free-prose paragraph — and the checker enforces the section set, the coverage rows and the one-route rule. Nothing checks harness mechanics, per-scenario build facts, declared cost, or an approval hold.

**Desired behavior:** the class is the successor version, with the seven-section set; its harness section is in exactly one of two forms — every slot of a closed vocabulary answered, or a cited existing harness plus this feature's delta and no slot rows; two scenario-keyed tables list exactly the scenario set the scenarios section declares; and the checker decides presence and shape only. An empty, placeholder, or bare-token-run slot is malformed. A slot carrying a blocking open item leaves the doc well-formed as a draft and malformed the moment its status reads approved. A doc declaring the predecessor's version string is not this class.

**Scope:**
- the successor template carrying the section set, both harness forms, and the two scenario-keyed tables
- the checker's structural rules over sections, slots, tables, routes, cost class, cadence and owned artifacts
- the shape floor rejecting a slot answered by a bare token run, its threshold tuned against the fixtures
- the approval-hold rule pairing an approved status with a present blocking open item

**Out of scope:**
- judging whether a slot's answer suffices to build from — that is the buildability lens's, never a checker rule

**Acceptance criteria:**
- [ ] a golden successor-version doc passes the checker and each seeded defect fails at exactly its own rule
- [ ] a slot answered by a bare token run is malformed while a short but structured answer passes
- [ ] a doc carrying a blocking open item passes as a draft and fails once its status reads approved
- [ ] inspection of the rule inventory shows every rule decidable from structure, none reading an answer for meaning

**Key interfaces:**
- the successor template-version string, rejected when a doc declares the predecessor's
- the closed slot vocabulary, shared by template, checker, lens calibration and plan
- the checker's rule set and the defect identity each rule raises

**Traces:** prd:e2e-verification/FR-18 · prd:e2e-verification/FR-19 · prd:e2e-verification/FR-20 · prd:e2e-verification/FR-21 · prd:e2e-verification/FR-23 · prd:e2e-verification/UB-8 · design:e2e-verification-artifacts-v2#3.1 · design:e2e-verification-artifacts-v2#3.2 · design:e2e-verification-artifacts-v2#3.3 · design:e2e-verification-artifacts-v2#3.5

### T2 — The e2e review report admits a buildability finding

| Field | Value |
|---|---|
| **Blocked by** | — |
| **Size band** | 80–150 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | e2e-review-report-template, internal/e2e/report, internal/e2e/corpus |

**Goal:** a buildability finding can be recorded, validated and scored end to end.

**Current behavior:** the report schema's lens field admits two values and the catch-corpus run's case-family field admits two seeded families, so a buildability finding cannot be written down and a harness-hole case cannot be scored.

**Desired behavior:** the report is the successor schema — still a member of the review-report family, sharing its closable-finding and versioned-report grammar — whose lens field admits coverage, anti-gaming and buildability; the finding event and the corpus-run event admit the third lens and the harness-hole case family respectively.

**Scope:**
- the successor report schema and its template, admitting the third lens
- the finding event's lens field and the corpus-run event's case-family field
- a conformance check that the successor schema still validates against the family's shared grammar

**Out of scope:**
- the lens's own prompt and calibration rows — those ride the lens task

**Acceptance criteria:**
- [ ] a report carrying a buildability finding validates against the successor schema
- [ ] the same report validates against the review-report family's shared grammar
- [ ] a run emitting a buildability finding and a harness-hole corpus case records both without an unknown-value error

**Key interfaces:**
- the successor report-schema version string and its lens enumeration
- the finding event's lens field and the corpus-run event's case-family field

**Traces:** prd:e2e-verification/FR-24 · prd:e2e-verification/EVT-5 · prd:e2e-verification/EVT-6 · design:e2e-verification-artifacts-v2#3.6

### T3 — e2e-design picks the harness form by judging whether a fitting harness exists

| Field | Value |
|---|---|
| **Blocked by** | T1 |
| **Size band** | 150–250 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | e2e-design-skill, internal/e2e/design |

**Goal:** a feature whose interface an existing harness can already drive gets a citation and a delta; one with no such harness gets a full mechanical design.

**Current behavior:** the skill names a harness able to drive the feature's primary interface and argues why it fits — a library preference, with no mechanics and no notion of reusing one already in the repository.

**Desired behavior:** the skill reads the repository's existing test trees and judges for itself whether any can drive this feature's primary external interface. On a hit it authors the reuse form — citing that harness and listing only this feature's fixtures, fakes and helpers, re-deciding none of its mechanics. On a miss it authors the build form, answering every slot of the closed vocabulary. There is no registry to consult and no fixed location to test; existence is a judgment, and its output is a citation a reviewer can check.

**Scope:**
- the reuse-or-build judgment over the repository's existing test trees
- authoring the reuse form as a citation plus this feature's additions only
- authoring the build form with an answer under every slot

**Out of scope:**
- any declared harness registry or location convention — existence stays a judgment
- the per-scenario restatement in the harness's terms, which rides its own task

**Acceptance criteria:**
- [ ] on a fixture whose harness already exists the skill emits the reuse form with no slot rows and no mechanic re-decided
- [ ] on a fixture with no fitting harness the skill emits the build form with every slot answered
- [ ] across fixtures of at least two application shapes both hold, and inspection finds no per-shape branch in skill code

**Key interfaces:**
- the harness section's two forms and the condition selecting between them
- the app-shape-neutral rule keeping every shape choice in authored content rather than skill code

**Traces:** prd:e2e-verification/FR-19 · prd:e2e-verification/FR-20 · prd:e2e-verification/NFR-1 · design:e2e-verification-skills-v2#3.1

### T4 — e2e-design restates each scenario in the harness's terms and declares its cost

| Field | Value |
|---|---|
| **Blocked by** | T1, T3 |
| **Size band** | 120–200 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | e2e-design-skill, internal/e2e/design |

**Goal:** every scenario names what it needs to be built and what it costs to run, so a planner can assign it and a reviewer can price the suite.

**Current behavior:** scenarios are English prose carrying a route and, where manual, an owner. Nothing says which fixture a scenario needs, which seams it exercises, which artifact it owns, what class of spend it incurs, or how often it runs.

**Desired behavior:** once the harness section is settled, every scenario is restated in that harness's own vocabulary — the fixture it requires, the seam or seams it exercises, and the one artifact it exclusively owns — and carries exactly one cost class and one run cadence alongside its route. The harness states a per-run bound in tokens and session count, never money and never wall-clock, so one scheduled run's cost is derivable from the document alone.

**Scope:**
- restating every scenario with its fixture, its seams, and its one exclusively-owned artifact
- declaring one cost class and one cadence per scenario
- stating the harness's per-run bound in tokens and session count

**Out of scope:**
- converting the bound into money — that is derived on demand from the pinned price map, never written into the frozen document

**Acceptance criteria:**
- [ ] every authored scenario names its fixture, its seams and exactly one owned artifact, and no artifact is named twice
- [ ] every authored scenario carries exactly one cost class and one cadence
- [ ] a scenario naming a seam the harness section does not carry is raised as a defect
- [ ] one scheduled run's cost is derivable from an authored document alone

**Key interfaces:**
- the scenario build-facts row and its owned-artifact field, which the plan reads to assign work
- the cost-class and cadence enumerations, and the per-run bound's unit

**Traces:** prd:e2e-verification/FR-21 · prd:e2e-verification/FR-23 · prd:e2e-verification/NFR-5 · design:e2e-verification-skills-v2#3.1 · design:e2e-verification-artifacts-v2#3.3

### T5 — e2e-design specifies a decision-authority stand-in that probes rather than agrees

| Field | Value |
|---|---|
| **Blocked by** | T1, T3, T4 |
| **Size band** | 100–180 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | e2e-design-skill, internal/e2e/design |

**Goal:** a feature whose exercised flow needs human decisions can be tested without a human, by a stand-in that cannot manufacture a green.

**Current behavior:** nothing says what supplies the human decisions an agentic feature's flow requires; the one built example invented an answer bank with no contract behind it.

**Desired behavior:** where the flow under test requires human decisions, the harness section's conditional slot specifies a stand-in with all five properties — a scripted answer set consulted first, a persona fallback bound to a written character sheet, every fallback answer recorded as an improvisation with the assertions depending on it reported non-authoritative, at least one adversarial persona that stays silent or contradicts or refuses, and no sight of the run's assertions. Where the flow needs no human decision, the slot carries a stated not-needed reason. Specifying the stand-in is this work; building or running it is not.

**Scope:**
- authoring the conditional slot with all five properties where the flow needs human decisions
- authoring a stated not-needed reason where it does not
- the agentic fixture the specification is verified against

**Out of scope:**
- building or running the stand-in — that belongs to the deferred build follow-on

**Acceptance criteria:**
- [ ] on the agentic fixture the emitted slot specifies all five properties
- [ ] a stand-in specified as cooperative-only, as sighted of the assertions, or as improvising unrecorded is raised as a defect
- [ ] on a fixture whose flow needs no human decision the slot carries a stated not-needed reason

**Key interfaces:**
- the conditional stand-in slot and its five required properties
- the agentic fixture's shape as a feature whose exercised flow requires human decisions

**Traces:** prd:e2e-verification/FR-22 · design:e2e-verification-skills-v2#3.1 · design:e2e-verification-artifacts-v2#3.2

### T6 — e2e-design holds an unsettleable slot open and never emits a malformed doc

| Field | Value |
|---|---|
| **Blocked by** | T1, T3, T5 |
| **Size band** | 100–160 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | e2e-design-skill, internal/e2e/design |

**Goal:** a mechanical element that genuinely cannot be settled stops the document short of approval instead of being papered over.

**Current behavior:** nothing forbids answering a harness element with a library name, and nothing holds a document back when an element is unresolved.

**Desired behavior:** where no existing harness fits and a slot cannot be settled — no way to observe an outcome, no way to isolate a run from its neighbours — the skill records that slot as a blocking open item naming what is unresolved and why, and never lets a library or tool name stand in for missing mechanics. The skill runs the class checker over its own output before emitting and emits nothing malformed; the document is held short of approval until the element is settled or the scenarios depending on it are re-routed.

**Scope:**
- authoring an unsettleable slot as a blocking open item naming what is unresolved and why
- running the class checker over the authored document before emitting, and refusing to emit a malformed one
- the re-route path that lifts the hold when the dependent scenarios no longer owe the element

**Out of scope:**
- re-implementing or overriding any checker rule — the rules belong to the class

**Acceptance criteria:**
- [ ] the unsettleable-harness fixture yields a document carrying that slot as a blocking open item and does not reach approval
- [ ] zero emitted harness sections carry a slot below the class's shape floor
- [ ] a run whose authored output would be malformed emits no document and names the failing rule

**Key interfaces:**
- the blocking-open-item slot value and the approval hold it triggers
- the check-before-emit step and the rule identities it surfaces

**Traces:** prd:e2e-verification/UB-8 · design:e2e-verification-skills-v2#3.1 · design:e2e-verification-skills-v2#3.4

### T7 — The buildability lens reads blind and its checker duplicates are dropped afterwards

| Field | Value |
|---|---|
| **Blocked by** | T1, T2 |
| **Size band** | 150–250 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | e2e-review-skill lens set and calibration, internal/e2e/review |

**Goal:** a harness section nobody could build from is caught by review, without the operator dispositioning the same defect twice.

**Current behavior:** the review runs two lenses, neither of which judges whether the machine the document specifies could be built; a harness section carrying no mechanics passed review twice.

**Desired behavior:** a third lens judges buildability — whether every slot the harness owes is answered with mechanics a worker could build from, and whether each scenario is buildable against the harness as specified, its fixture obtainable and its seams carried. It reads the document blind, never handed the checker's verdict or another lens's findings. Findings that restate a defect the checker already raised on the same slot are dropped after the lens returns and logged as filtered, so the operator sees one clean list and over-reach stays measurable.

**Scope:**
- the buildability lens as data — its prompt and its calibration rows
- the blind read, with no verdict admitted into the shared context pack
- the post-return filter matching on checker-defect identity together with the slot it was raised on

**Out of scope:**
- narrowing the lens prompt to skip structure — that is the same leak wearing another hat

**Acceptance criteria:**
- [ ] inspection of the shared context pack's admitted kinds shows no verdict and no other lens's findings
- [ ] a lens finding restating a checker defect on the same slot is absent from the operator-visible report and present in the run log as filtered
- [ ] a near-miss finding on a different slot survives to the report
- [ ] adding the lens leaves the other lenses' logic untouched

**Key interfaces:**
- the lens set as data — one calibration row and one prompt per lens
- the filter's match key: checker-defect identity together with the slot

**Traces:** prd:e2e-verification/FR-24 · prd:e2e-verification/FR-7 · prd:e2e-verification/NFR-4 · design:e2e-verification-skills-v2#3.2

### T8 — e2e-review refuses a round on a malformed subject

| Field | Value |
|---|---|
| **Blocked by** | T1, T2, T7 |
| **Size band** | 80–140 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | e2e-review-skill, internal/e2e/review |

**Goal:** no review round is spent running lenses over a document that should never have been submitted.

**Current behavior:** the review runs its lenses over whatever it is handed; a structurally broken document consumes a whole round before anyone notices.

**Desired behavior:** the review runs the class checker over the subject at the start of a round. On a malformed subject it refuses the round and returns the checker's defects, running no lens and producing no lens output. On a well-formed subject the round proceeds exactly as before.

**Scope:**
- running the class checker over the subject at round start
- refusing the round and returning the checker's defects when the subject is malformed
- leaving the well-formed path unchanged

**Out of scope:**
- re-implementing any checker rule — the review consumes the class's verdict

**Acceptance criteria:**
- [ ] a malformed subject yields the checker's defects, zero lens output and no report round
- [ ] a well-formed subject proceeds through the lens set unchanged
- [ ] the refusal names the failing rules rather than a generic error

**Key interfaces:**
- the round-start check and its refusal, distinct from a lens finding
- the checker's defect list as the refusal's payload

**Traces:** prd:e2e-verification/FR-18 · design:e2e-verification-skills-v2#3.4

### T9 — e2e-plan lands conventions before it fans out and assigns exclusive artifact ownership

| Field | Value |
|---|---|
| **Blocked by** | T1, T4 |
| **Size band** | 180–280 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | e2e-plan-skill, internal/e2e/plan |

**Goal:** parallel scenario workers copy a landed shape and cannot collide, because each owns exactly one artifact nobody else owns.

**Current behavior:** the plan emits per-scenario tasks plus a harness task where none exists, so every scenario task enters the ready frontier at once against a package with no landed example, and nothing stops two of them editing the same file.

**Desired behavior:** the plan emits per-scenario tasks, a conventions task landing the harness's scenario contract and one worked scenario, and a harness task — the last two each only where the repository does not already supply them. Every scenario task carries a blocking dependency on whichever of the two were emitted and none where the repository supplies both, and names the one artifact it exclusively owns, taken from that scenario's restatement. No two scenario tasks in a plan name the same artifact.

**Scope:**
- emitting the conventions task, and omitting it where a scenario already exists against the harness
- emitting the harness task, and omitting it where a fitting harness already exists
- the blocking dependencies from each scenario task onto whichever tasks were emitted
- carrying each scenario's owned artifact through onto its task, uniquely

**Out of scope:**
- any change to the planner pipeline or the executor — the plan stays an ordinary one and the ordering comes free

**Acceptance criteria:**
- [ ] a fixture with no harness and no existing scenario yields both extra tasks, with every scenario task blocked on both
- [ ] a fixture supplying both yields neither extra task and scenario tasks carrying no such dependency
- [ ] every scenario task names exactly one owned artifact and no artifact is named twice
- [ ] the planner pipeline and the executor are unchanged by this task

**Key interfaces:**
- the conventions task's content: the harness's scenario contract plus one worked scenario
- the per-task owned-artifact field and its uniqueness rule across a plan

**Traces:** prd:e2e-verification/FR-25 · prd:e2e-verification/FR-26 · prd:e2e-verification/FR-14 · prd:e2e-verification/NR-2 · design:e2e-verification-skills-v2#3.3

### T10 — e2e-plan refuses a document whose scenarios name no owned artifact

| Field | Value |
|---|---|
| **Blocked by** | T9 |
| **Size band** | 60–120 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | e2e-plan-skill, internal/e2e/plan |

**Goal:** a plan never invents ownership the review never saw.

**Current behavior:** nothing checks that an approved document can supply exclusive ownership before the plan is cut.

**Desired behavior:** invoked on an approved document whose scenarios do not each name the one artifact they exclusively own, the plan refuses — naming every offending scenario and projecting zero tasks. It never assigns ownership itself: the fix belongs in the document, where review can see it.

**Scope:**
- the refusal on a document that cannot supply exclusive ownership, naming each offending scenario
- projecting zero tasks on that refusal

**Out of scope:**
- assigning or inferring ownership in the planner — the refusal is the whole behavior

**Acceptance criteria:**
- [ ] the un-normalized-document fixture yields a refusal naming each offending scenario
- [ ] that refusal projects zero tasks
- [ ] a document supplying ownership for every scenario is planned without a refusal

**Key interfaces:**
- the refusal's payload: the offending scenario identifiers
- the zero-projection guarantee on refusal

**Traces:** prd:e2e-verification/UB-9 · design:e2e-verification-skills-v2#3.3

### T11 — The harness-hole catch corpus and its recorded catch rate

| Field | Value |
|---|---|
| **Blocked by** | T7 |
| **Size band** | 200–300 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | e2e-review-skill catch-corpus fixtures, internal/e2e/corpus |

**Goal:** the third lens's bite is measured rather than assumed, on seeded cases that mirror the defects it exists to catch.

**Current behavior:** the corpus seeds coverage gaps and hollow scenarios only, so nothing measures whether a harness a worker could not build from is caught.

**Desired behavior:** the corpus gains a harness-hole family — an unanswered slot, a slot answered by a library name alone, a scenario naming a seam the harness does not carry, a scenario with no owned artifact, a cooperative-only stand-in, and a stand-in sighted of the assertions — alongside complete controls that must yield no finding. Its catch rate is recorded at the checkpoint as a measurement feeding the reviewer's advisory ramp, never a per-commit gate.

**Scope:**
- the seeded harness-hole cases and their complete controls
- recording the catch rate at the checkpoint as a measurement
- the corpus run's case-family value for the new family

**Out of scope:**
- gating any commit on the catch rate — model-driven verdicts measure, they never gate

**Acceptance criteria:**
- [ ] every seeded harness-hole case is flagged by the buildability lens
- [ ] the complete controls yield no buildability finding
- [ ] the checkpoint run records a catch rate for the new family
- [ ] no commit gate consumes that catch rate

**Key interfaces:**
- the harness-hole case family and its seeded members
- the recorded catch-rate output the advisory ramp reads

**Traces:** prd:e2e-verification/EVAL-3 · prd:e2e-verification/EVT-6 · prd:e2e-verification/DOD-9 · design:e2e-verification-skills-v2#3.2

### T12 — Update the living rollups and the authoring reference for the successor contract

| Field | Value |
|---|---|
| **Blocked by** | T6, T8, T10, T11 |
| **Size band** | 60–120 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | docs/techspec.md, docs/prd.md, docs/wiki/e2e-authoring-reference.md, docs/wiki/glossary.md |

**Goal:** the living documents describe the shipped contract rather than the one it replaced.

**Current behavior:** the rollups and the authoring reference describe the predecessor class — six sections, a harness named by library, two review lenses.

**Desired behavior:** they describe the successor contract: the section set, the harness slot form and its two variants, the two scenario-keyed tables, the third lens, the conventions task, and the exclusive-ownership rule. The glossary carries any term the successor contract introduces.

**Scope:**
- the rollup entries for the shipped successor capability
- the authoring reference's description of the class, the lens set and the plan shape
- the glossary terms the successor contract introduces

**Out of scope:**
- restating the technical designs — the reference routes to them, never duplicates them

**Acceptance criteria:**
- [ ] the rollups describe the successor class, the third lens and the conventions task
- [ ] the authoring reference carries no description of the predecessor's harness part
- [ ] every term the successor contract introduces resolves in the glossary

**Key interfaces:**
- the rollup entries naming the shipped capability
- the authoring reference's routing to the two live designs

**Traces:** prd:e2e-verification/DOD-7 · design:e2e-verification-artifacts-v2#3.1 · design:e2e-verification-skills-v2#3.2

## Coverage

| Requirement | Tasks |
|---|---|
| prd:e2e-verification/FR-18 | T1, T8 |
| prd:e2e-verification/FR-19 | T1, T3 |
| prd:e2e-verification/FR-20 | T1, T3 |
| prd:e2e-verification/FR-21 | T1, T4 |
| prd:e2e-verification/FR-22 | T5 |
| prd:e2e-verification/FR-23 | T1, T4 |
| prd:e2e-verification/FR-24 | T2, T7 |
| prd:e2e-verification/FR-25 | T9 |
| prd:e2e-verification/FR-26 | T9 |
| prd:e2e-verification/UB-8 | T1, T6 |
| prd:e2e-verification/UB-9 | T10 |
| prd:e2e-verification/NFR-5 | T4 |
| prd:e2e-verification/EVAL-3 | T11 |
| prd:e2e-verification/EVT-5 | T2 |
| prd:e2e-verification/EVT-6 | T2, T11 |
| prd:e2e-verification/DOD-9 | T11 |
| prd:e2e-verification/DOD-7 | T12 |
| prd:e2e-verification/FR-7 | T7 |
| prd:e2e-verification/FR-14 | T9 |
| prd:e2e-verification/NFR-1 | T3 |
| prd:e2e-verification/NFR-4 | T7 |
| prd:e2e-verification/NR-2 | T9 |

## Deferred

| Requirement | Reason |
|---|---|
| prd:e2e-verification/DOD-8 | The dogfood — carrying one real feature design to review to an accepted plan — is an operator-run acceptance milestone, not executor work: its bar is the operator accepting the result, which no worker can do on their behalf. Deliberately kept out of the plan so the fleet cannot claim it from the ready frontier. The operator runs it once T6, T8, T10 and T11 have landed; the accepted document, its review and its plan are its evidence. |
