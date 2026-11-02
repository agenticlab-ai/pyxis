# e2e authoring artifacts — the `e2e-design/v1` doc class, and the report/plan reuse — technical design

| Field | Value |
|---|---|
| **Status** | Approved *(2026-07-24 — operator accepted, operator review waived; unmanned tech-design-review: approve-with-advisories. Merge is the freezing event; "superseded" is computed by the index.)* |
| **Owner** | @mdarabi |
| **Approver(s)** | @mdarabi (operator — a new document class is a document-system surface; named reviewer mandatory) |
| **Date** | 2026-07-24 |
| **Relevant-when** | designing or changing the e2e authoring artifacts — the `e2e-design/v1` document class (its sections, template, per-feature home, freeze-at-approval), or the reuse bindings for the e2e review report and plan; implementing a reader/writer against the e2e design-doc format; superseding the artifact contract |
| **Scope** | The three e2e artifacts' contract: the new **`e2e-design/v1`** document class (its closed section set, header, per-feature home, freeze-at-approval lifecycle); the e2e **review report** as its own instance-schema in the review-report family (`e2e-review-report/v1`); and the e2e **plan** as an ordinary plan-new `plan/v1`. **Not**: the three skills' behavior that produces/consumes these artifacts (`design:e2e-verification-skills`); building or running the tests (deferred — `prd:e2e-verification/WR-1`); docgate governance of the new class (staged behind the shared graduation trigger, §7 Q1). |
| **Tracking** | — *(the introducing PR)* |
| **Builds-on** | design-brainstorm-record-model (the lightweight-authored-class precedent) · prd-review-artifacts (the review-report family this instances) · plan-new-v1 (the plan class + pipeline the e2e plan reuses) |
| **Supersedes** | — |
| **Serves** | prd:e2e-verification/FR-1 · prd:e2e-verification/FR-2 · prd:e2e-verification/FR-4 · prd:e2e-verification/FR-5 · prd:e2e-verification/FR-11 · prd:e2e-verification/FR-14 · prd:e2e-verification/FR-15 · prd:e2e-verification/NR-1 |
| **Security/privacy impact** | none — repo-local markdown documents; no identity, secret, user-data, network, or model/provider-egress surface (the authoring skill's egress is `design:e2e-verification-skills`' concern) |
| **Template version** | tech-design-template/v1 |

## 0. Read to rule — the human reviewer's section (~4 min)

*The body is the contract; this section only routes attention — on any conflict the body governs; machines ignore it.*

**Judge these**

| # | The call | Could have gone | If you disagree |
|---|---|---|---|
| 1 | **The e2e design doc is a new lightweight class**, frozen at approval, off the design index (§2) | reuse the technical-design class (`design-brainstorm:e2e-verification-tech-design#ALT-1`) | you'd route a per-feature test-coverage doc through the design index + design-contract machinery → §2 · §4 |
| 2 | **Ungoverned by docgate in v1** — no fold/lint, integrity via the template checklist + review (§3.3) | a governed docgate kind now (`#ALT-3`) | you'd build fold/immutability machinery for an artifact that never amends → §3.3 · §4 · §7 Q1 |
| 3 | **The design doc + its review live in `docs/e2e/<feature>/`; the plan is an ordinary plan-new plan** in `docs/plans/` (§3.3) | one e2e folder holding all three | the plan would fork plan-new's home/freeze instead of reusing it → §3.2 · §3.3 |
| 4 | **The report is its own instance in the review-report *family*; the plan is a verbatim plan-new `plan/v1`** (§3.2) | reuse prd-review's PRD-typed `review-report/v1` verbatim (infeasible) | you'd force a fourth reviewer onto a schema whose header only admits PRDs → §3.2 · #DINV-5 |

**Sanity-check the numbers** — the `e2e-design/v1` doc has **six** required sections (§3.1); **one** route per scenario (#DINV-3); the class carries **zero** design-index entries (#DINV-6).

**The author's open questions** — §7 Q1 (the trigger to graduate the class to docgate governance).

**Skip the rest** — structure/schema conformance → docgate (once the class graduates, §7 Q1); substance/reuse honesty → the unmanned `tech-design-review`; the three skills' behavior → `design:e2e-verification-skills`; building or running the tests → the deferred build follow-on (`prd:e2e-verification/WR-1`).

## 1. Problem

The merged `e2e-verification` PRD requires three artifacts — an e2e design doc (six parts, including an explicit coverage map and a per-scenario route), a review report, and an ordered plan — but deliberately left their **document class, home, and lifecycle to design** (its Q1 · `brainstorm:e2e-verification#P-4`). Nothing yet decides what *kind* of documents these are: reuse an existing class, mint a lightweight new one, or stand up governed machinery. Decided wrong, the e2e design doc either misuses the technical-design class — flooding the generated design index with a per-feature entry every feature, and dragging along `Serves`/`Supersedes`/principles ceremony that does not fit test-coverage — or pays for amendment/governance machinery a non-ledger artifact never uses. The cheapest moment is now: no e2e artifact exists to migrate. The stage-3 brainstorm grilled and gated this direction (`design-brainstorm:e2e-verification-tech-design`, ready-for-design), which this design assembles.

## 2. Decision

**Decision:** The e2e design doc is a **new lightweight authored document class, `e2e-design/v1`** — its own template and per-feature home under `docs/e2e/<feature>/`, **frozen at operator approval**, not docgate-folded and not listed in the generated design index (the design-brainstorm-record / review-report precedent). The **review report is its own instance-schema in the review-report family — `e2e-review-report/v1`** (each reviewer class has its own report schema; the family shape is closable findings + a versioned report), and the **e2e plan is an ordinary plan-new `plan/v1`**, authored by e2e-plan and homed/frozen/projected by plan-new's pipeline verbatim (`design-brainstorm:e2e-verification-tech-design#D-1`, `#D-2` as refined at review by `#D-13`).

The reasoning: the e2e design doc is per-feature, high-volume, model-authored, and freeze-at-approval — exactly the shape of the design-brainstorm and review-report classes already added this way (`design:design-brainstorm-record-model` §3.1). Reusing the technical-design class mis-fits those semantics and pollutes the index (`#ALT-1`); a governed docgate kind pays for a fold/amendment machine a non-ledger artifact never uses (`#ALT-3`). The report and plan honor the family/pipeline they belong to — a fourth review-report instance and an ordinary plan — rather than forking prd-review's PRD-typed schema (infeasible: its header only admits PRD subjects) or inventing a bespoke plan.

## 3. The design

### 3.1 The `e2e-design/v1` document class

**Decision:** one e2e design doc per feature is a markdown file instantiated from an `e2e-design/v1` template, with a **closed section set** — every section present, an empty one reading a stated `none — <reason>` (`prd:e2e-verification/FR-1`). The header carries the routing pair (`In one line` · `Relevant-when`), a **`Basis`** row typing the feature join (`prd:<feature>` — the coverage source), the **governing-design revision(s)** the coverage was derived from (the live design *set* — §3.2 of the skills design), and `Status` (`draft` → `approved`).

The six required sections (`prd:e2e-verification/FR-1`):

1. **Scenarios** — the end-to-end flows the tests will exercise.
2. **Setup / teardown** — the fixture lifecycle.
3. **Mock strategy** — what is mocked, and with what.
4. **Harness libraries** — the named harness able to drive the feature's primary interface.
5. **Coverage map** — each required behavior of the feature → the scenario(s) proving it (`prd:e2e-verification/FR-2`); an unroutable behavior is surfaced as an explicit open item rather than dropped.
6. **Per-scenario route** — each scenario carries exactly one of `run-for-real` · `mocked` · `manual-human` (`prd:e2e-verification/FR-4`); a `manual-human` scenario names its human-judgment owner (`prd:e2e-verification/FR-5`).

The coverage map and the routes are the structure this class exists to make **checkable**: the reviewers (`design:e2e-verification-skills`) read them, but the class guarantees they are *present and well-formed*. Whether the map is *complete* against the feature's required-behavior set is the coverage lens's judgment (`design:e2e-verification-skills` §3.2), not something the class's own checklist can see.

### 3.2 The review report and the plan — a family instance and a reused plan

**Decision:** neither artifact reuses a foreign schema verbatim; each honors the family/class it belongs to.

- **Review report** — its own **`e2e-review-report/v1`** schema in the review-report family (subject `e2e-design:<feature>`, moment/lens fields for coverage · anti-gaming, its own calibration table), mirroring how `design-review-report/v1` and `impl-review-report/v1` are each their own instance of that family — **not** a verbatim reuse of prd-review's PRD-typed `review-report/v1`, whose closed header only admits PRD subjects. Findings carry `blocking`/`advisory` severity each dispositioned resolved-or-rejected; the report is persisted **alongside** the design doc (`docs/e2e/<feature>/review.md`) and rides its introducing change (`prd:e2e-verification/FR-11`). Emitting and converging the findings is behavior (`design:e2e-verification-skills`).
- **Plan** — the e2e plan **is an ordinary plan-new `plan/v1`**: e2e-plan authors it from the approved e2e design doc (a harness task — conditional per `prd:e2e-verification/FR-16` — plus per-scenario tasks with the native `blocked-by` edges), and it is then homed at plan-new's `docs/plans/<feature>/NNN-plan.md`, folded by `plangate`, **frozen at its plan-PR merge** (plan-new's approval-and-freeze event, satisfying `prd:e2e-verification/FR-15`'s freeze-at-approval), and projected idempotently by `plangate project`. No new plan schema and no edit to plan-new (`prd:e2e-verification/FR-14`, `NR-1`); the whole plan pipeline is reused. Declaring the tasks and edges is behavior (`design:e2e-verification-skills` §3.3).

### 3.3 Home, naming, and lifecycle

**Decision:** a feature's e2e **design doc and its review** are co-located under a per-feature directory **`docs/e2e/<feature>/`** — `design.md` (the `e2e-design/v1` doc) and `review.md` (the `e2e-review-report/v1` instance, satisfying FR-11's "alongside"). The **plan is not here** — it is a plan-new `plan/v1` at `docs/plans/<feature>/` (§3.2), reusing plan-new's home wholesale. The `docs/e2e/` path is the **bootstrap default**, config-surfaced like the other doc-system homes (AGENTS.md rule 7 · issue #4).

**Decision:** the e2e design doc is a **live draft through review and frozen at operator approval**; after approval, the intended mutability posture is that it changes only by a **successor** doc (a fresh file for a materially changed feature), never in place — the design-brainstorm/review-class freeze-at-merge posture (`design:design-brainstorm-record-model` §3.1). Enforcement of that immutability and the successor-authoring path are **deferred** (docgate governance, §7 Q1; re-authoring on feature change, `design-brainstorm:e2e-verification-tech-design#O-2`) — so v1 asserts the posture but does not ship a mechanism that rewrites-in-place. The class is **not docgate-folded and not listed in the generated design index**; its authoring-time integrity rests on the template's structure checklist for *well-formedness* (§3.5, `#DINV-1`), the *completeness* check being the e2e review's coverage lens, until the class graduates to docgate governance (§7 Q1).

### 3.4 Boundaries and non-goals

- **Not solved here** — the three skills' behavior that authors, reviews, and plans these artifacts (`design:e2e-verification-skills`); the exact per-behavior coverage derivation.
- **Owned by a future design / feature** — building or running the tests (`prd:e2e-verification/WR-1`); the feature-evolution / e2e-test-delta capability (a follow-on — `design-brainstorm:e2e-verification-tech-design#O-2`); docgate governance of the class (§7 Q1).
- **Implementation must not infer** — that the class is docgate-governed (it is not, in v1 — §3.3 · §7 Q1); that the review report reuses prd-review's `review-report/v1` (it is its own `e2e-review-report/v1` in the family — `#DINV-5`); that the plan is a new schema (it is an ordinary plan-new `plan/v1` — `#DINV-5`); that the e2e design doc is a technical design (it is a distinct lightweight class, absent from the design index — `#DINV-6`).
- **Must remain unchanged** — the technical-design class and its generated index; the review-report family schemas; plan-new's plan class, pipeline, and projection — this design **instances/reuses**, never edits, all three (`prd:e2e-verification/NR-1`).

### 3.5 Invariants

- **DINV-1** — an `e2e-design/v1` doc carries all six required sections (each present; empty reads `none — <reason>`) plus its header `Basis` join; every coverage-map entry carries ≥ 1 scenario or an explicit routed disposition, and every scenario exactly one route. A doc failing any of these is malformed (`prd:e2e-verification/FR-1`). *(Well-formedness; completeness-against-the-required-set is the coverage lens's job, `design:e2e-verification-skills` §3.2.)*
- **DINV-2** — no required behavior the doc *does* enumerate is left absent from **both** the coverage map and the routes — the no-silent-drop property within the authored set (`prd:e2e-verification/FR-2`).
- **DINV-3** — every scenario carries exactly one route (`run-for-real` · `mocked` · `manual-human`); a `manual-human` scenario names its human-judgment owner (`prd:e2e-verification/FR-4`, `FR-5`).
- **DINV-4** — the plan artifact is byte-stable after its plan-new freeze (`prd:e2e-verification/FR-15`); the e2e design doc's post-approval mutability posture (successor, never in place) is asserted but its enforcement is deferred (§3.3).
- **DINV-5** — the review report is an `e2e-review-report/v1` instance in the review-report family and the plan is an ordinary plan-new `plan/v1`; neither is a bespoke new schema nor a verbatim reuse of a foreign one (`prd:e2e-verification/FR-14`, `NR-1`).
- **DINV-6** — the e2e design doc is not a technical design: it is absent from the generated design index and carries no `Serves`/`Supersedes` design-class semantics.

### 3.6 Observability

none — this is a document class with no runtime surface (the skills that author and consume it carry their own observability in `design:e2e-verification-skills`, `prd:e2e-verification/§5.7`). The class's authoring-time integrity surface is the template's well-formedness checklist plus review-time inspection, until docgate governance graduates (§7 Q1).

### 3.7 Security & privacy

none — matches the header row: repo-local markdown documents; no identity, secret, user-data, network, or production surface. The model/provider egress belongs to the *authoring skill* (`design:e2e-verification-skills`, the same egress class as the existing authoring/review skills), not to this artifact contract.

## 4. Alternatives considered

| Alternative | Why rejected |
|---|---|
| **Reuse the technical-design class** for the e2e design doc (`design-brainstorm:e2e-verification-tech-design#ALT-1`) | the e2e doc is per-feature test-coverage, not an engine-architecture decision; it mis-fits the class's semantics (DINV / Supersedes lineage / principle citations) and floods the generated design index with a per-feature entry each run; I4 (a `Serves` omission costs a supersession) and I1 (`Serves` must cite committed FRs) bite a high-volume doc forced through the design-contract machinery |
| **A new governed docgate kind** (own fold / immutability / citation checks) (`#ALT-3`) | strongest integrity, but the e2e design doc does not evolve by typed amendments — a changed feature re-runs the skill; it is not a ledger — so a fold/amendment machine is machinery it never uses, over-built for the appetite; governance can graduate later behind the shared trigger (§7 Q1) |
| **Reuse prd-review's `review-report/v1` verbatim for the e2e review report** | that schema's closed header is typed to PRD subjects (`Subject: prd:…`, `Reviewer: prd-review`); the review-report *family* pattern is one instance-schema per reviewer class (`design-review-report/v1`, `impl-review-report/v1`), so the e2e report is its own `e2e-review-report/v1` instance — the family reused, not a foreign schema forced (`#DINV-5`) |
| **A bespoke e2e plan schema/home** | the plan is an ordinary plan-new `plan/v1` — reusing plan-new's class, freeze-at-merge, and idempotent `plangate project` wholesale lets the executor consume it unchanged and edits plan-new not at all (`#DINV-5`, `NR-1`) |

## 5. Risks

| ID | Risk | Impact | Mitigation | Verification / evidence |
|---|---|---|---|---|
| R1 | the class is ungoverned in v1 (no docgate lint), so a malformed e2e design doc could slip | a doc missing a section or a routeless scenario reaches review | the template's well-formedness checklist (`#DINV-1/2/3`) walked at authoring + the e2e review + operator approval; graduation to docgate governance is the named path (§7 Q1) | A-2 golden + defect fixtures |
| R2 | artifact homes are split across `docs/e2e/` and `docs/plans/`, confusing where a feature's e2e material lives | a reader hunts two trees | the split is deliberate and documented (§3.3): design + review co-located in `docs/e2e/<feature>/`, the plan reusing plan-new's `docs/plans/` home wholesale; the design doc's header names its own review path | A-5 |
| R3 | the `e2e-review-report/v1` schema drifts from the review-report family (restated rather than instanced) | family divergence; tooling that reads the family breaks | the schema is a family instance sharing the closable-finding + versioned-report shape (`#DINV-5`); a conformance test validates it against the family's shared grammar | A-3 |

## 6. Acceptance

| ID | Acceptance criterion | Verify | Required evidence | Traces to |
|---|---|---|---|---|
| A-1 | the `e2e-design/v1` template exists, version-pinned, with the six required sections + the header `Basis`/governing-design-revision(s) rows + the well-formedness checklist | Inspection | template file in the introducing PR | `prd:e2e-verification/FR-1` · §3.1 |
| A-2 | a golden e2e design doc passes the well-formedness checklist, and each seeded defect (a missing section; a coverage-map entry with no scenario/route; a scenario with two routes; a `manual-human` route with no owner) fails at exactly its rule | Test | golden + defect fixtures with checklist results | `prd:e2e-verification/FR-1` · `FR-4` · `FR-5` · `#DINV-1` · `#DINV-3` |
| A-3 | the e2e review report validates against the `e2e-review-report/v1` family instance (shares the closable-finding + versioned-report grammar) and is persisted alongside the design doc; the e2e plan folds as a plan-new `plan/v1` with no new required field | Test, Inspection | conformance fixtures | `prd:e2e-verification/FR-11` · `FR-14` · `#DINV-5` |
| A-4 | the e2e design doc is absent from the generated design index; the plan artifact is byte-stable after its plan-new freeze | Inspection, Test | index output + plan-freeze fixture | `prd:e2e-verification/FR-15` · `#DINV-4` · `#DINV-6` |
| A-5 | `docs/e2e/<feature>/` holds `design.md` + `review.md`; the plan is a plan-new `plan/v1` at `docs/plans/<feature>/` reachable from the design doc's header | Inspection | dogfood home layout | §3.3 · `prd:e2e-verification/FR-11` |

## 7. Open questions

| ID | Question | Owner | Blocks | Status | Resolution |
|---|---|---|---|---|---|
| Q1 | The trigger that graduates `e2e-design/v1` from ungoverned to docgate-governed (a fold/lint/immutability check), mirroring the shared record/review-class graduation trigger | operator | nothing | open | v1 ships ungoverned (integrity via the well-formedness checklist + review); graduation on demonstrated volume or abuse, behind the shared trigger |
| Q2 | Whether the frozen e2e plan is a full plan-new `plan/v1` or a thinner manifest | design | nothing | resolved (at review) | it **is** an ordinary plan-new `plan/v1` (§3.2) — homed/frozen/projected by plan-new's pipeline verbatim; no separate manifest |

## Principles

- **AP-7 — honored.** A new mutability class, frozen at approval and (posture) immutable-by-successor — exactly how the design-brainstorm and review-report classes were added; the artifact stays a decision-record whose evolution is a fresh document, never an in-place edit.
- **AP-6 — honored.** The class is closed, contractual data: a closed section set, a typed header `Basis` join, and a structured coverage map at the consumer's granularity.
- **AP-11 — honored.** Reuse over rebuild — the e2e review report instances the review-report *family* and the e2e plan is an ordinary plan-new `plan/v1`; one review-report grammar family and one plan class + pipeline, not an e2e fork.
- **AP-21 — honored.** The frozen e2e design doc is the judge-once coverage artifact the plan (and the later build) read verbatim; re-judging is out of the artifact's contract.
- **AP-23 / AP-27 — honored.** One design doc per feature in a flat per-feature home, no shared append surface — unlike the technical-design index — so parallel e2e authors across features never contend.
- **AP-24 — honored.** The class derives from operator-approved rulings and governs nothing; nothing is regenerated *from* it.
- **RW-5 — honored.** One decision — the e2e artifact contract; the skills' behavior that produces and consumes these artifacts is a separate, independently-revisable document (`design:e2e-verification-skills`).
- **Deviation:** none.
