# e2e authoring artifacts — the `e2e-design/v2` doc class carries a buildable harness — technical design

| Field | Value |
|---|---|
| **Status** | Approved *(2026-08-07 — operator accepted, operator review waived; unmanned tech-design-review: approve-with-advisories at round 2, all seven findings applied. Merge is the freezing event; "superseded" is computed by the index, never written here.)* |
| **Owner** | @mdarabi |
| **Approver(s)** | @mdarabi (operator — a document class is a document-system surface; named reviewer mandatory) |
| **Date** | 2026-08-06 |
| **Relevant-when** | designing or changing the e2e authoring artifacts — the `e2e-design/v2` document class (its section set, the harness slot form, the two scenario-keyed tables, the per-feature home, freeze-at-approval), the well-formedness rules a checker enforces over it, or the reuse bindings for the e2e review report and plan; implementing a reader or writer against the e2e design-doc format; superseding the artifact contract |
| **Scope** | The three e2e artifacts' contract: the **`e2e-design/v2`** document class — its closed section set, the **closed harness slot form** (the mechanical elements `prd:e2e-verification/FR-19` enumerates, plus the conditional decision-authority stand-in slot, or the alternate cite-an-existing-harness form), the **two scenario-keyed tables** carrying each scenario's run facts and build facts, the blocking-open-item hold, the header, the per-feature home, and the freeze-at-approval lifecycle; the e2e **review report** as its own instance-schema in the review-report family (`e2e-review-report/v2`, three lenses); and the e2e **plan** as an ordinary plan-new `plan/v1`. **Not**: the three skills' behavior that produces and consumes these artifacts (`design:e2e-verification-skills-v2`); building or running the tests (`prd:e2e-verification/WR-1`); docgate governance of the class (staged behind the shared graduation trigger, §7 Q1). |
| **Tracking** | — *(the introducing PR)* |
| **Builds-on** | design-brainstorm-record-model (the lightweight-authored-class precedent) · prd-review-artifacts (the review-report family this instances) · plan-new-routed-demand (the plan class + pipeline the e2e plan reuses) |
| **Supersedes** | e2e-verification-artifacts |
| **Serves** | prd:e2e-verification/FR-2 · prd:e2e-verification/FR-4 · prd:e2e-verification/FR-5 · prd:e2e-verification/FR-11 · prd:e2e-verification/FR-14 · prd:e2e-verification/FR-15 · prd:e2e-verification/FR-18 · prd:e2e-verification/FR-19 · prd:e2e-verification/FR-20 · prd:e2e-verification/FR-21 · prd:e2e-verification/FR-22 · prd:e2e-verification/FR-23 · prd:e2e-verification/UB-8 · prd:e2e-verification/NFR-5 · prd:e2e-verification/NR-1 |
| **Security/privacy impact** | none — repo-local markdown documents; no identity, secret, user-data, network, or model/provider-egress surface (the authoring skills' egress is `design:e2e-verification-skills-v2`' concern) |
| **Template version** | tech-design-template/v1 |

## 0. Read to rule — the human reviewer's section (~4 min)

*The body is the contract; this section only routes attention — on any conflict the body governs; machines ignore it.*

**Judge these**

| # | The call | Could have gone | If you disagree |
|---|---|---|---|
| 1 | **The harness section is a closed slot form the checker validates for presence and shape, with the lens judging substance** (§3.2 · §3.5) | free prose judged wholly by the lens (`design-brainstorm:e2e-verification-tech-design-2#ALT-2`) | a library-name-only harness section can pass structurally again — the defect that produced amendment 002 → §3.2 · §3.5 · #DINV-2 · A-2 |
| 2 | **The class version bumps to `e2e-design/v2`** (§3.1) | keep `e2e-design/v1`, since no instance of it survives | one version string would name two incompatible contracts, and the predecessor defining v1 stays readable forever → §3.1 |
| 3 | **Every slot must be answered; an unsettleable one is a blocking open item, and `approved` alongside one is malformed** (§3.4 · §3.5) | let an unsettleable element be an ordinary open item, or leave the hold unenforced | the doc reaches approval with a hole in the machine every scenario depends on → §3.4 · #DINV-4 · `prd:e2e-verification/UB-8` |
| 4 | **Scenario facts live in two cross-checked tables, not one wide table or per-scenario prose** (§3.3) | one wide table (`#ALT` hand-authored) or fields under each scenario | either legibility or the no-two-scenarios-own-one-artifact rule gets harder to enforce → §3.3 · #DINV-3 |
| 5 | **The per-run cost bound is stated in tokens and session count; money is derived, never written** (§3.2) | a money figure in the doc | a frozen doc's dollar figure goes stale silently; wall-clock is non-hermetic (I5) → §3.2 · `design-brainstorm:e2e-verification-tech-design-2#D-7` |

**Sanity-check the numbers** — the class carries the section set §3.1 fixes; the harness form carries one slot per element `prd:e2e-verification/FR-19` enumerates **plus one conditional** stand-in slot (§3.2); **one** route, **one** cost class, **one** cadence and **exactly one** exclusively-owned artifact per scenario (#DINV-3); **zero** artifacts named twice (#DINV-3); the class carries **zero** design-index entries (#DINV-7).

**The author's open questions** — §7 Q1 (the trigger graduating the class to docgate governance) · §7 Q2 (whether the slot vocabulary is versioned independently of the class) · §7 Q4 (the shape floor's exact threshold — the rule binds, the number is tuned).

**Skip the rest** — structure/schema conformance → the class's own checker (§3.5), and docgate once the class graduates (§7 Q1); substance and reuse honesty → the unmanned `tech-design-review`; the three skills' behavior → `design:e2e-verification-skills-v2`; building or running the tests → the deferred build follow-on (`prd:e2e-verification/WR-1`).

## 1. Problem

The predecessor (`design:e2e-verification-artifacts`, frozen at merge) defined an `e2e-design/v1` class whose harness part was one free-prose paragraph naming a harness able to drive the feature's interface. Both documents authored under it answered that part with a library preference and a fit argument, and nothing else: no statement of what one scenario is as an artifact, what isolates one run from its neighbours, what controls the clock or the model tier, what a red run leaves behind, or what a run costs. The most reused and most load-bearing code in any e2e suite was therefore designed by whichever worker picked up the harness task, after review had closed and with no reviewer having seen it.

Amendment 002 to the merged PRD closes that: the design doc must now carry the **machine** that proves the coverage, not only the coverage. It supersedes five Must rows and adds nine more — mechanical harness elements, an existing-harness reuse rule, per-scenario normalization into fixture/seams/one-owned-artifact, a decision-authority stand-in, declared cost class and cadence, a blocking open item for an unsettleable element, and a refusal when scenarios name no owned artifact. The class as frozen can carry none of it: it has no slot to check, no scenario-level build facts, and no approval hold.

The predecessor cannot absorb this — a merged design is byte-immutable, and the only legal repair is whole-document supersession (incident I6). This document is that supersession, carrying forward the predecessor's still-valid decisions (the lightweight class, its per-feature home, the report-family instance, the plan reuse) and replacing what amendment 002 falsified. The stage-3 reopen grilled and gated the direction (`design-brainstorm:e2e-verification-tech-design-2`, ready-for-design), which this design assembles.

## 2. Decision

**Decision:** The e2e design doc remains a **lightweight authored document class** with its own per-feature home, frozen at operator approval, off the generated design index — and its version bumps to **`e2e-design/v2`**, whose harness section is a **closed slot form**: one slot per mechanical element the basis enumerates plus one conditional decision-authority stand-in slot, or, where a fitting harness already exists, an alternate form citing it and listing only this feature's delta. Each scenario additionally carries its run facts and its build facts in **two scenario-keyed tables** cross-checked to the same scenario set, one of which names the single artifact that scenario exclusively owns. **Presence is mechanical and substance is judgment**: a checker rejects an empty, placeholder, or bare-library-name slot and a table set that disagrees, while the review's buildability lens judges whether what is written could actually be built from (`design-brainstorm:e2e-verification-tech-design-2#D-3`, `#D-5`, `#D-6`, `#D-7`).

The review report stays its own instance in the review-report family — now **`e2e-review-report/v2`**, admitting the third (buildability) lens — and the e2e plan stays an ordinary plan-new `plan/v1`, homed, frozen and projected by plan-new's pipeline verbatim (`#D-1`).

The reasoning: the defect amendment 002 exists to close is *structural* — a section that names a library where mechanics belong. A structural defect is exactly what a hermetic parser catches reliably and a model-driven lens catches only on average, and incident I5's standing lesson is that model-driven verdicts measure rather than gate. Splitting the two — the checker owning that every slot is answered, the lens owning whether the answer is any good — puts each half where it is trustworthy, and it is the split the authority ruled explicitly: filling a slot is not passing it.

## 3. The design

### 3.1 The `e2e-design/v2` document class

**Decision:** one e2e design doc per feature is a markdown file instantiated from an `e2e-design/v2` template with a **closed section set** — every section present, an empty one reading a stated `none — <reason>` (`prd:e2e-verification/FR-18`). The header carries the routing pair (`In one line` · `Relevant-when`), a **`Basis`** row typing the feature join (`prd:<feature>` — the coverage source), the **governing-design revision(s)** the coverage was derived from (the live design *set*), and `Status` (`draft` → `approved`).

The sections, in canonical order:

1. **Scenarios** — the end-to-end flows the tests will exercise, each carrying a stable scenario ID the two tables key on.
2. **Setup / teardown** — the fixture lifecycle.
3. **Mock strategy** — what is faked, and with what.
4. **Harness design** — the slot form (§3.2). *Renamed from the predecessor's `Harness libraries`: the section no longer names a library, it specifies a machine.*
5. **Coverage map** — each required behavior of the feature → the scenario(s) proving it (`prd:e2e-verification/FR-2`); an unroutable behavior is surfaced as an explicit open item rather than dropped.
6. **Scenario routes** — the run-facts table (§3.3).
7. **Scenario build facts** — the build-facts table (§3.3).

Against `prd:e2e-verification/FR-18`'s six named contents the mapping is one-to-one plus one: sections 1, 2, 3, 4 and 5 are its scenarios, setup and teardown, mock strategy, harness design and coverage map; section 6 (**Scenario routes**) is its per-scenario route. Section 7 is **additive**, carrying `FR-21`'s normalization — a seventh section satisfies a six-part requirement, it does not dilute it.

**Decision:** the class's version string bumps to **`e2e-design/v2`** rather than reusing `v1`. Template versions are compatibility contracts validated against the version a document *declares* (`design:tech-design-conventions` §3.1), and the frozen predecessor defines `e2e-design/v1` as the six-section class with a prose harness part. That definition stays readable forever; reusing the string would make one version identifier name two incompatible contracts, and any checker validating "declared v1" would have to pick which one it meant. No instance of v1 survives to migrate (`design-brainstorm:e2e-verification-tech-design-2#D-2`), so the bump costs nothing and buys an unambiguous contract.

### 3.2 The harness section — a closed slot form

**Decision:** the harness section takes **exactly one of two forms**, and which one is present is itself checkable.

**Build form** — used where no fitting harness exists. A table with one row per slot, slot names drawn from a **closed vocabulary**, each row stating that slot's answer:

| Slot | What it answers |
|---|---|
| `driver` | how a scenario starts the system under test, and what it gets back |
| `world` | what the run executes against, and who creates it |
| `scenario-contract` | what one scenario is as an artifact, and where it lives |
| `seams` | the named interfaces a fake attaches to |
| `observation-surface` | what a scenario is handed to assert on |
| `lifecycle-and-isolation` | setup, teardown, what is shared, what must run serially |
| `determinism-controls` | clock, randomness, model tier, retries — and what an assertion may therefore claim |
| `cost-controls` | the per-run bound, and how it is enforced |
| `failure-diagnostics` | what a red run leaves behind |
| `boundary` | what the harness deliberately does not do |
| `decision-authority-stand-in` *(conditional)* | the stand-in supplying the human decisions the exercised flow requires, with the properties `prd:e2e-verification/FR-22` fixes |

The first ten are the elements `prd:e2e-verification/FR-19` enumerates — that requirement owns the set, and this document owns only the vocabulary naming them. The eleventh is conditional: a feature with no human decision inside the flow it exercises fills it with a stated not-needed reason; a feature that has one and leaves it blank is malformed. It sits here rather than in its own section because `FR-22` places the stand-in in the harness section, and because a stand-in is harness machinery — it is how the driver gets past a prompt.

**Reuse form** — used where a harness able to drive the feature's primary external interface already exists in the repository (`prd:e2e-verification/FR-20`). The section cites that harness by path and lists **only this feature's additions** — the fixtures, fakes and helpers it needs — carrying **no slot rows at all**. Restating or re-deciding a cited harness's mechanics is a review finding, not a legal fill: the mechanical design is paid once per app shape and reused thereafter, which is what keeps sibling features on one harness instead of forking it.

**Decision:** the per-run bound the `cost-controls` slot states is denominated in **tokens and session count** — never money, never wall-clock. A money view is *derived* on demand by applying the repository's pinned per-model price map, never written into the frozen document. A dollar figure in an immutable doc goes stale the next time a price moves, and a wall-clock bound is non-hermetic — the same code passes on a fast machine and fails on a slow one, which is precisely the category error incident I5 charged this repository for. A token-and-session bound stays true as prices move and is countable while the run executes, so the `cost-controls` slot's "how it is enforced" half has something real to enforce against (`design-brainstorm:e2e-verification-tech-design-2#D-7`).

Together with each scenario's declared cost class and cadence (§3.3), this is what makes the suite's price a reviewable property of the design rather than a discovery at build time (`prd:e2e-verification/NFR-5`): a reviewer derives one scheduled run's cost from the document alone by taking the scenarios whose cadence is `scheduled`, reading their cost classes, and checking the total against the stated bound.

### 3.3 The two scenario-keyed tables

**Decision:** each scenario's five declared facts live in **two tables, both keyed by scenario ID**, and the two must list **exactly the same scenario set** as the Scenarios section.

**Scenario routes** — how a scenario runs:

| Scenario | Route | Owner | Cost class | Cadence |
|---|---|---|---|---|
| *(ID)* | `run-for-real` · `mocked` · `manual-human` | the human-judgment owner when the route is `manual-human`, else `—` | `deterministic` · `bounded-model` · `full-session` | `per-change` · `scheduled` · `checkpoint` · `on-demand` |

**Scenario build facts** — how a scenario is built, restated in the harness's own terms (`prd:e2e-verification/FR-21`):

| Scenario | Fixture | Seams exercised | Owned artifact |
|---|---|---|---|
| *(ID)* | the fixture this scenario requires | the seam(s) it exercises, each named in the harness section | the one artifact it exclusively owns under the harness's scenario contract |

The split is deliberate. A single table carrying all of it would be eight columns wide — unreadable rendered, and awkward to edit. Its one genuine advantage over a split, that a scenario cannot appear in one place and be missing from the other, is recovered mechanically by the same-set rule rather than left to a reviewer's eye (`design-brainstorm:e2e-verification-tech-design-2#D-5`). Burying these facts under each scenario's prose was rejected for the opposite reason: the rule that matters most downstream — that no two scenarios name the same owned artifact — is a cross-row property, and cross-row properties are enforced over tables, not scattered blocks.

The owned artifact is the load-bearing column. It is what lets the plan assign parallel scenario tasks that cannot collide (`prd:e2e-verification/FR-26`), and a document whose scenarios do not each name one is what the planner refuses on (`UB-9`).

### 3.4 The blocking open item and the approval hold

**Decision:** where no existing harness fits and some slot's answer genuinely cannot be settled — no way to observe an outcome, no way to isolate a run from its neighbours — the slot carries an explicit **blocking open item** naming what is unresolved and why, and the document **cannot reach `approved`** while one stands (`prd:e2e-verification/UB-8`). The hold lifts one of two ways: the element is settled, or the scenarios depending on it are re-routed so it is no longer owed.

A blocking open item is a distinct fill, not a species of prose: it is what an honest author writes instead of a library name when the mechanics are not there. The class therefore admits it as a legal slot value and makes it consequential — a slot answered by a library or tool name is malformed (#DINV-2), a slot answered by a blocking open item is well-formed but holds approval (#DINV-4). Without the second, the first would simply push authors toward the papering-over the requirement forbids.

### 3.5 Well-formedness — what a checker owns, and what it does not

**Decision:** the class ships a **checker** enforcing exactly the structurally decidable rules below, and nothing requiring judgment. It is the class's own well-formedness surface, not a docgate verb and wired to no gate, until the class graduates (§7 Q1). **e2e-design runs it before emitting a doc and e2e-review runs it over the subject** — that invocation is behavior, carried by `design:e2e-verification-skills-v2`; the rules themselves are this document's.

| Rule | Verdict |
|---|---|
| every section of §3.1's set present, in canonical order, none repeated, none unrecognized | malformed |
| the harness section is in exactly one of the two forms of §3.2 (never both, never neither) | malformed |
| in build form, every slot of the closed vocabulary present; the conditional slot present with either an answer or a stated not-needed reason | malformed |
| a slot answered empty, or by a placeholder token | malformed |
| a slot answered by a **bare token run** — no sentence structure, below the shape floor (§7 Q4) — the structural proxy for a library or tool name standing in for mechanics | malformed |
| the two tables of §3.3 list exactly the scenario set the Scenarios section declares | malformed |
| every scenario carries exactly one route, one cost class, one cadence, and exactly one owned artifact | malformed |
| no artifact named as owned by two scenarios | malformed |
| a `manual-human` route names a human-judgment owner | malformed |
| every coverage-map entry carries ≥ 1 scenario or an explicit routed disposition | malformed |
| a slot carries a blocking open item | well-formed; blocks `approved` |
| the header's `Status` reads `approved` while any slot carries a blocking open item | malformed |

The bare-token-run rule is deliberately a **structural proxy, not a judgment**: it decides the *shape* of an answer — that prose was written where mechanics were owed — and never whether the prose is right. It catches the crudest form of the defect amendment 002 exists to close (a slot reading `testcontainers` and nothing else) at zero judgment cost. The richer form — a fluent paragraph that still specifies no machine — is beyond any structural rule and belongs to the buildability lens.

What the checker deliberately **cannot** see, and the review's buildability lens therefore owns: whether a slot's answer is *sufficient* — whether a worker could build the harness from it — and whether a scenario is buildable against the harness as specified. Presence and shape are hermetic; "good enough" is judgment, and the two must not be confused (I5).

The last rule is what gives the approval hold of §3.4 a surface to be tested against: without it, `#DINV-4` would assert a state nothing could detect.

### 3.6 The review report and the plan

**Decision:** neither artifact reuses a foreign schema verbatim; each honors the family or class it belongs to. This carries the predecessor's ruling forward, with one change.

- **Review report** — its own **`e2e-review-report/v2`** schema in the review-report family (subject `e2e-design:<feature>`, its own calibration table), mirroring how each reviewer class instances that family rather than reusing prd-review's PRD-typed `review-report/v1`, whose closed header admits only PRD subjects. The version bumps because the lens set the schema admits grows from two to three — coverage, anti-gaming, **buildability** (`prd:e2e-verification/FR-24`) — and a closed enum that cannot record a buildability finding cannot carry this feature's review. Findings carry `blocking`/`advisory` severity each dispositioned resolved-or-rejected; the report is persisted **alongside** the design doc and rides its introducing change (`prd:e2e-verification/FR-11`).
- **Plan** — the e2e plan **is an ordinary plan-new `plan/v1`**, homed at plan-new's `docs/plans/<feature>/`, folded by `plangate`, **frozen at its plan-PR merge** (satisfying `prd:e2e-verification/FR-15`'s freeze-at-approval) and projected idempotently by `plangate project`. No new plan schema and no edit to plan-new (`prd:e2e-verification/FR-14`, `NR-1`). Which tasks it carries and what edges they take is behavior (`design:e2e-verification-skills-v2`).

### 3.7 Home, naming, and lifecycle

**Decision:** a feature's e2e **design doc and its review** are co-located under a per-feature directory `docs/e2e/<feature>/` — `design.md` (the `e2e-design/v2` doc) and `review.md` (the `e2e-review-report/v2` instance, satisfying FR-11's "alongside"). The **plan is not here** — it is a plan-new `plan/v1` under plan-new's own home (§3.6), reusing that pipeline wholesale. The `docs/e2e/` path is the **bootstrap default**, config-surfaced like the other doc-system homes (AGENTS.md rule 7 · issue #4).

**Decision:** the e2e design doc is a **live draft through review and frozen at operator approval**; after approval it changes only by a **successor** document, never in place. Enforcement of that immutability and the successor-authoring path remain **deferred** (docgate governance, §7 Q1) — v1 of this class asserted the posture without a mechanism, and this version does not change that. The class is **not docgate-folded and not listed in the generated design index**; its authoring-time integrity rests on the checker of §3.5 for well-formedness, the review's lenses for substance, and operator approval.

### 3.8 Boundaries and non-goals

- **Not solved here** — the three skills' behavior that authors, reviews and plans these artifacts, including how an existing harness is discovered and how coverage is derived (`design:e2e-verification-skills-v2`).
- **Owned by a future design / feature** — building or running the tests, and any run-time check that a scenario genuinely bites (`prd:e2e-verification/WR-1`); the feature-evolution / e2e-test-delta capability; docgate governance of the class (§7 Q1).
- **Implementation must not infer** — that the class is docgate-governed (it is not, §3.7 · §7 Q1); that a document declaring `e2e-design/v1` satisfies this contract (it does not — the version bump is the whole point, §3.1); that the checker judges sufficiency (it judges presence only — §3.5); that a slot may be answered by naming a library (#DINV-2); that the review report reuses prd-review's `review-report/v1` (it is its own family instance — #DINV-6); that the plan is a new schema (it is an ordinary plan-new `plan/v1` — #DINV-6).
- **Must remain unchanged** — the technical-design class and its generated index; the review-report family's shared grammar; plan-new's plan class, pipeline and projection — this design **instances and reuses**, never edits, all three (`prd:e2e-verification/NR-1`).
- **Deliberately not carried forward** — any compatibility with `e2e-design/v1` instances, or a migration path for them. The authority ruled the existing e2e line removed outright (`design-brainstorm:e2e-verification-tech-design-2#D-2`), so this class owes its predecessor's instances nothing.
- **A stranded lineage edge, named and left standing** — `design:implement-now-e2e-tasks` is a **live, merged** design whose `Builds-on` names this document's supersession target, so retiring that target flags it `stale-builds-on` in the generated index. Re-pointing a merged design's lineage row is not legal — `check-immutable` refuses any edit to a merged design, and incident **I6** cost this repository a mid-repair reversal on exactly that attempt, leaving whole-document supersession as the only lawful repair. The flag therefore **stands until `implement-now-e2e-tasks` is itself superseded**, which belongs to `prd:implement-now`, not to this feature (`prd:e2e-verification/NR-2` forbids this feature editing there). Named here so it is a known, owned consequence rather than a surprise at that feature's next planning entry — I6's lesson is that such a flag is a planning blocker in waiting, never decoration.

### 3.9 Invariants

- **DINV-1** — an `e2e-design/v2` doc carries every section of §3.1's closed set (each present; empty reads `none — <reason>`) plus its header `Basis` join; a document missing, repeating, or adding a section is malformed (`prd:e2e-verification/FR-18`).
- **DINV-2** — the harness section is in exactly one of the two forms of §3.2. In build form every slot of the closed vocabulary is present and answered, the conditional stand-in slot carrying either an answer or a stated not-needed reason; a slot that is empty, a placeholder token, or a bare token run below the shape floor (§7 Q4) is malformed (`prd:e2e-verification/FR-19`, `FR-20`, `FR-22`, `UB-8`). Whether an answer that clears that floor actually specifies mechanics is the buildability lens's judgment, never the checker's (#DINV-8).
- **DINV-3** — the two scenario-keyed tables list exactly the scenario set the Scenarios section declares; every scenario carries exactly one route, one cost class, one cadence and exactly one exclusively-owned artifact, and no artifact is named as owned by two scenarios (`prd:e2e-verification/FR-4`, `FR-21`, `FR-23`, `FR-26`). A `manual-human` route names its human-judgment owner (`FR-5`).
- **DINV-4** — a slot carrying a blocking open item leaves the document well-formed but **unapprovable**: a document whose `Status` reads `approved` while any slot carries one is malformed, which is the rule that makes the hold detectable rather than merely asserted (`prd:e2e-verification/UB-8`, §3.5).
- **DINV-5** — every required behavior the doc enumerates appears in the coverage map carrying ≥ 1 scenario or an explicit routed disposition — the no-silent-drop property within the authored set (`prd:e2e-verification/FR-2`). *(The routes table is keyed by scenario, not behavior, so it is not a second place a behavior could hide; the predecessor's looser wording implied otherwise.)*
- **DINV-6** — the review report is an `e2e-review-report/v2` instance in the review-report family and the plan is an ordinary plan-new `plan/v1`; neither is a bespoke new schema nor a verbatim reuse of a foreign one (`prd:e2e-verification/FR-14`, `NR-1`). The plan artifact is byte-stable after its plan-new freeze (`FR-15`).
- **DINV-7** — the e2e design doc is not a technical design: it is absent from the generated design index and carries no `Serves`/`Supersedes` design-class semantics.
- **DINV-8** — the checker of §3.5 decides **presence and shape**, never sufficiency: every rule it enforces is decidable from a document's structure — which sections and slots exist, whether a slot's answer clears the stated shape floor, whether two tables agree, whether an enum value is admitted — and none requires judging whether an answer is *right* or *enough*. Sufficiency belongs to the buildability lens.

### 3.10 Observability

none — this is a document class with no runtime surface. The skills that author and consume it carry their own observability (`design:e2e-verification-skills-v2`, `prd:e2e-verification/§5.7`). The class's authoring-time integrity surface is the checker of §3.5 plus review-time inspection, until docgate governance graduates (§7 Q1).

### 3.11 Security & privacy

none — matches the header row: repo-local markdown documents; no identity, secret, user-data, network, or production surface. The model and provider egress belongs to the *authoring skills* (`design:e2e-verification-skills-v2`), not to this artifact contract.

## 4. Alternatives considered

| Alternative | Why rejected |
|---|---|
| **Free prose judged wholly by the lens** — rename the section, list the elements as template guidance, add no checker rule (`design-brainstorm:e2e-verification-tech-design-2#ALT-2`) | cheapest to build, and it leaves open exactly the failure mode amendment 002 exists to close: a library-name-only section passed review twice. It places a structural property behind a model-driven verdict, which incident I5 rules cannot gate — the corpus that would prove the lens catches it is a recorded measurement, never an enforcement |
| **An open slot set the author declares, with reasons for omissions** (`#ALT-3`) | bends gracefully to odd app shapes, but an author optimizing to look complete declares three slots, answers all three honestly and passes the checker clean; completeness returns to being a judgment call, and `prd:e2e-verification/FR-19`'s fit criterion ("a stated answer for each of the ten elements") stops being structurally guaranteed |
| **One wide scenario table** carrying route, owner, cost class, cadence, fixture, seams and owned artifact together | eight columns is unreadable rendered and awkward to edit; its only advantage over the split — no drift between two lists — is recovered mechanically by the same-set rule (§3.3) |
| **Per-scenario fields under each scenario's prose** | reads best narratively, but the rule that matters most downstream is cross-row (no two scenarios owning one artifact) and is far harder to enforce over scattered blocks than over a table |
| **A money-denominated per-run bound** | a frozen document's dollar figure goes stale the next time a price moves and cannot be corrected; tokens and session count stay true, and money is derivable from the repository's pinned price map when a reviewer wants it |
| **A wall-clock per-run bound** | non-hermetic — identical code passes on a fast machine and fails on a slow one. Incident I5 charged this repository a stranded PR for exactly that: a wall-clock assertion is a measurement, not a bound |
| **Keep the class at `e2e-design/v1`**, since no v1 instance survives | the frozen predecessor defines `e2e-design/v1` as the six-section prose-harness class and stays readable forever; reusing the string would make one version identifier name two incompatible contracts, and template versions are compatibility contracts validated against the declared version |
| **Reuse the technical-design class** for the e2e design doc *(carried forward from the predecessor's §4)* | the e2e doc is per-feature test coverage, not an engine-architecture decision; it mis-fits the class's semantics and floods the generated design index with a per-feature entry each run; I4 (a `Serves` omission costs a supersession) and I1 (`Serves` must cite committed FRs) bite a high-volume doc forced through the design-contract machinery |
| **A governed docgate kind** with its own fold, immutability and citation checks *(carried forward)* | strongest integrity, but the e2e design doc does not evolve by typed amendments — a changed feature re-runs the skill; it is not a ledger, so a fold machine is machinery it never uses. Governance can graduate later behind the shared trigger (§7 Q1) |
| **Reuse prd-review's `review-report/v1` verbatim** *(carried forward)* | that schema's closed header is typed to PRD subjects; the family pattern is one instance-schema per reviewer class, so the e2e report is its own instance — the family reused, not a foreign schema forced |
| **A bespoke e2e plan schema or home** *(carried forward)* | the plan is an ordinary plan-new `plan/v1`; reusing that class, its freeze-at-merge and its idempotent projection wholesale lets the executor consume it unchanged and edits plan-new not at all |

## 5. Risks

| ID | Risk | Impact | Mitigation | Verification / evidence |
|---|---|---|---|---|
| R1 | The slot form is filled shallowly — every slot answered, each answer a sentence that still leaves a worker guessing | a structurally green harness section nobody could build from; the amendment's bet fails while its checker passes | the checker owns presence only and says so (#DINV-8); substance is the buildability lens's job, with its own proving corpus (`prd:e2e-verification/EVAL-3`), and `prd:e2e-verification/DOD-8` requires that lens finding-free on the dogfood | A-2 (checker fixtures) + A-3 (the lens's corpus, owned by `design:e2e-verification-skills-v2`) |
| R2 | The closed slot vocabulary proves wrong for an app shape nobody anticipated — a shape needing a twelfth element, or for which one of the ten is meaningless | authors write `n/a` into slots to get past the checker, hollowing the form | the reuse form absorbs the common case (a shape's mechanics are designed once, then cited); a genuinely missing element is a basis change, since `prd:e2e-verification/FR-19` owns the set — not a slot the class quietly adds. §7 Q2 tracks whether the vocabulary needs its own version line | A-1; the multi-app-shape fixtures |
| R3 | The class is ungoverned (no docgate lint), so a malformed doc could reach review if the checker is not run | a doc missing a slot or a table row reaches the operator | the checker is a real code surface with fixtures (A-2), run at authoring and by the review; graduation to docgate governance is the named path (§7 Q1) | A-2 |
| R4 | The blocking open item becomes the easy exit — authors park elements as blocking rather than settling them | designs stall short of approval, or the hold gets waived habitually | a blocking open item must name *what* is unresolved and *why*, so it is reviewable rather than a checkbox; the hold lifts by re-routing the dependent scenarios, giving an honest alternative to stalling (§3.4) | A-4 |
| R5 | The two-table split drifts in a way the same-set rule does not catch — matching scenario sets but a build-facts row describing a different scenario than its ID says | a plan assigns the wrong owned artifact to a task | the same-set rule catches structural drift; semantic mismatch is a buildability-lens judgment (a scenario naming a seam the harness does not carry is a review finding, `prd:e2e-verification/FR-21`) | A-2 + the lens's corpus |
| R6 | The `e2e-review-report/v2` bump diverges from the review-report family rather than instancing it | family divergence; tooling reading the family breaks | the schema shares the family's closable-finding and versioned-report grammar; a conformance test validates it against that shared grammar | A-5 |

## 6. Acceptance

| ID | Acceptance criterion | Verify | Required evidence | Traces to |
|---|---|---|---|---|
| A-1 | the `e2e-design/v2` template exists, version-pinned, carrying §3.1's section set, §3.2's slot form in both variants, §3.3's two tables, and the header `Basis` + governing-design-revision rows | Inspection | template file in the introducing PR | `prd:e2e-verification/FR-18` · §3.1 · §3.2 |
| A-2 | a golden doc passes the checker, and each seeded defect fails at exactly its rule — a missing section; a harness section in both forms and in neither; a missing slot; a slot left empty; a slot answered by a placeholder; a slot answered by a bare token run (the library-name case, caught by the shape floor of §3.5); the two tables disagreeing on the scenario set; a scenario with two routes, no cost class, no cadence, or no owned artifact; one artifact owned twice; a `manual-human` route with no owner; a coverage-map entry with neither scenario nor disposition | Test | golden + defect fixtures with checker results | `prd:e2e-verification/FR-2` · `FR-4` · `FR-5` · `FR-19` · `FR-21` · `FR-23` · #DINV-1 · #DINV-2 · #DINV-3 · #DINV-5 |
| A-3 | across fixtures of at least two app shapes, a doc in build form carries an answer under every slot, and a doc whose harness already exists carries the reuse form — citation plus delta, with zero slot rows and no mechanic re-decided | Inspection, Test | multi-app-shape fixtures | `prd:e2e-verification/FR-19` · `FR-20` · §3.2 |
| A-4 | a doc carrying a blocking open item in a slot is well-formed while its `Status` is `draft`, and **malformed** the moment `Status` reads `approved`; removing the item or re-routing the dependent scenarios lifts the hold | Test | blocking-open-item fixture, in both `Status` states | `prd:e2e-verification/UB-8` · #DINV-4 · §3.5 |
| A-5 | the e2e review report validates against the `e2e-review-report/v2` family instance — sharing the closable-finding and versioned-report grammar, and admitting a buildability finding — and is persisted alongside the design doc; the e2e plan folds as a plan-new `plan/v1` with no new required field | Test, Inspection | conformance fixtures | `prd:e2e-verification/FR-11` · `FR-14` · `FR-24` · #DINV-6 |
| A-6 | the e2e design doc is absent from the generated design index; the plan artifact is byte-stable after its plan-new freeze | Inspection, Test | index output + plan-freeze fixture | `prd:e2e-verification/FR-15` · #DINV-6 · #DINV-7 |
| A-7 | a reviewer can derive one scheduled run's cost from the design doc alone — the scheduled-cadence scenarios, their cost classes, and the harness's stated token-and-session bound | Inspection, Analysis | the dogfood doc + a worked derivation | `prd:e2e-verification/NFR-5` · `FR-23` · §3.2 |
| A-8 | `docs/e2e/<feature>/` holds `design.md` + `review.md`; the plan is a plan-new `plan/v1` under plan-new's home, reachable from the design doc's header | Inspection | dogfood home layout | `prd:e2e-verification/FR-11` · §3.7 |
| A-9 | every checker rule is decidable from document structure alone — inspection of the rule inventory confirms none judges whether an answer is right or sufficient; the shape floor of §3.5 is exercised by two fixtures, a bare token run (malformed) and a short but structured answer (well-formed, left to the lens) | Inspection, Test | the checker's rule inventory + the two shape-floor fixtures | #DINV-8 · §3.5 |

## 7. Open questions

| ID | Question | Owner | Blocks | Status | Resolution |
|---|---|---|---|---|---|
| Q1 | The trigger that graduates `e2e-design/v2` from ungoverned to docgate-governed (a fold/lint/immutability check), mirroring the shared record/review-class graduation trigger | operator | nothing | open | ships ungoverned (integrity via the checker + review); graduation on demonstrated volume or abuse, behind the shared trigger |
| Q2 | Whether the harness slot vocabulary carries its own version line, so a future element can be added without bumping the whole class | design | nothing | open | v2 ties them together — the vocabulary is part of the class contract; if the basis adds an element and the coupling proves painful, a successor splits them |
| Q3 | Whether the reuse form should record *which* revision of the cited harness it was written against, so a later harness change is detectable | design | nothing | open | not required in v2 — the cited harness is a code path, not a versioned document; revisit if harness churn produces stale citations |
| Q4 | The exact shape floor a slot answer must clear to escape the bare-token-run rule (§3.5) | implementation | nothing | open | the rule is **binding**; its threshold is tuned at implementation against the defect fixtures of A-2 and A-9 — deliberately not frozen as a number here, since a document that cannot take corrections should not carry a tuning constant |

## Principles

- **AP-6 — honored.** The class is closed, contractual data: a closed section set, a closed slot vocabulary, typed enums for route, cost class and cadence, and a typed header `Basis` join. An unrecognized section or slot name is an error, not silent acceptance.
- **AP-7 — honored.** The class keeps its mutability posture — frozen at approval, evolving by successor, never edited in place — and this document evolves its predecessor the only legal way, by whole-document supersession (incident I6).
- **AP-10 — honored.** The mechanical half of integrity is codified in a checker rather than left to a checklist someone walks by hand; what cannot be codified is explicitly assigned elsewhere (#DINV-8) rather than pretended into code.
- **AP-11 — honored.** One slot vocabulary shared by the template, the checker, the lens calibration and the plan; one review-report family instanced rather than forked; one plan class and pipeline reused.
- **AP-13 / AP-14 — honored.** The class's teeth are on the artifact: a blocking open item holds approval mechanically (#DINV-4) rather than relying on a reviewer to remember. Sufficiency is *not* claimed as mechanically verified — it is named as judgment and routed to the lens.
- **AP-21 — honored.** The frozen doc is the judge-once artifact the plan and the later build read verbatim; re-judging is out of the artifact's contract.
- **AP-23 / AP-27 — honored.** One design doc per feature in a flat per-feature home, no shared append surface — so parallel e2e authors across features never contend. The rejected harness registry (`design-brainstorm:e2e-verification-tech-design-2#ALT-5`) would have introduced exactly such a surface.
- **AP-24 — honored.** The class derives from operator-approved rulings and governs nothing; nothing is regenerated *from* it.
- **AP-19 — honored.** The suite's own steady-state cost becomes a declared, reviewable property (§3.2, `prd:e2e-verification/NFR-5`) rather than a discovery at build time.
- **AP-25 — honored.** The class carries no app-shape-specific structure: the slot vocabulary is shape-neutral, and shape-specific choices are authored content filling the slots.
- **RW-5 — honored.** One decision — the e2e artifact contract; the skills' behavior that produces and consumes these artifacts is the separate, independently-revisable `design:e2e-verification-skills-v2`.
- **I5 — honored (incident lesson).** A model-driven verdict measures and never gates: the checker gates on structure, the lens measures substance, and no structural property is left behind a model's judgment. The rejected wall-clock cost bound is the same lesson applied to the artifact's own content.
- **Deviation:** none.
