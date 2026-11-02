# <Feature> — e2e design

| Field | Value |
|---|---|
| **In one line** | <the e2e coverage distilled to one load-or-skip sentence — what a future orientation scan reads first> |
| **Relevant-when** | <load this doc when… — verifying or changing <feature>'s end-to-end behavior; reviewing or re-authoring its e2e coverage> |
| **Basis** | <the feature join, typed: `prd:<feature>` — the coverage source> |
| **Governing design(s)** | <the live technical-design revision(s) the coverage was derived from, typed: `design:<slug>`, one or more, `·`-separated> |
| **Status** | draft *(→ approved at operator acceptance; frozen from that point — a materially changed feature is re-authored as a successor doc, never edited in place)* |
| **Template version** | e2e-design/v2 |

## 0. How to use this template

*Guidance paragraphs look like this one — italic. Delete this section and
every guidance italic when instantiating — the section set is closed, so a
retained `## 0.` or `## Appendix A` heading is itself a defect
(`section-unknown`). The contract this template carries is
`docs/tech-design/e2e-verification-artifacts-v2.md` §3.1–§3.5 (the class
and well-formedness invariants); the behavior that authors it is bound by
`docs/tech-design/e2e-verification-skills-v2.md`. The mechanical checker
is enforced by `internal/e2e/docclass` (`Parse` + `Check`) — every rule
there has a matching line in Appendix A.*

*The rules that matter:*

- ***This is a lightweight, ungoverned class*** — *not a technical design.
  It carries no `Serves`, no `Supersedes`, and no entry in the generated
  technical-design index; it is not `docgate`-folded or linted in v2
  (graduation is a named open question, `design:e2e-verification-artifacts-v2#7` Q1).
  Its only integrity surface is this checklist plus human review.*
- ***One doc per feature, frozen at approval.*** *Instantiate at
  `docs/e2e/<feature>/design.md`, alongside its review at
  `docs/e2e/<feature>/review.md` (the `e2e-review-report/v2` instance).
  A feature's plan is a separate, ordinary plan-new `plan/v1` at
  `docs/plans/<feature>/`. Once `Status` reads `approved`, the doc
  changes only by a successor file for a materially changed feature —
  never edited in place.*
- ***The section set is closed.*** *Exactly seven below, in this order.
  Every section stays present; an empty one reads a stated
  `none — <reason>`, never silent blankness.*
- ***The harness section is one of two forms.*** *Either a build form
  with a table of slots (if the harness is being designed new), or a
  reuse form citing an existing harness by path with only this feature's
  delta listed — never both, never neither. An empty slot, a placeholder,
  or a bare library name is malformed (§3.2).*
- ***Two scenario-keyed tables.*** *Sections 6 and 7 carry the scenario
  routes and build facts, both keyed identically by scenario ID. Both
  must list exactly the same scenario set the Scenarios section declares
  (§3.3).*

## Scenarios

*The end-to-end flows the tests will exercise. One row per scenario, each
carrying a stable ID — the Scenario routes and Scenario build facts
sections below reference these IDs. `none — <reason>` only for a feature
with no automatable e2e surface at all.*

| ID | Scenario | Flow |
|---|---|---|
| <S1> | <name> | <the end-to-end flow this scenario drives, one line> |

## Setup and teardown

*The fixture lifecycle — what is created before a scenario runs and torn
down after; what state, if any, crosses scenarios (and why that is safe).*

<the setup/teardown description, or `none — <reason>`>

## Mock strategy

*What is mocked, and with what — named per seam, not "mocks as needed."
An honest mock strategy is what the anti-gaming lens checks against;
naming nothing here for a scenario that in fact mocks something is exactly
what that lens exists to catch.*

<the mock strategy, or `none — <reason>`>

## Harness design

*The harness in one of two forms (§3.2):*

**(A) Build form** — if no existing harness fits, a table with one row per
element enumerated by `prd:e2e-verification/FR-19`. Each slot is answered
with the mechanics that element requires; a slot whose answer genuinely
cannot be settled carries a blocking open item instead (§3.4), written as
`open item — <what is unresolved and why>`. The literal `open item` must lead
the cell — that exact token is what holds the document short of approval, and
any other phrasing reads as an ordinary answer. `decision-authority-stand-in`
is the one slot that cannot take an open item (its own check refuses one).

| Slot | Answer |
|---|---|
| `driver` | <how scenarios start the system, and what they get back> |
| `world` | <what the run executes against, and who creates it> |
| `scenario-contract` | <what one scenario is as an artifact, and where it lives> |
| `seams` | <the named interfaces a fake attaches to> |
| `observation-surface` | <what a scenario is handed to assert on> |
| `lifecycle-and-isolation` | <setup, teardown, what is shared, what must run serially> |
| `determinism-controls` | <clock, randomness, model tier, retries — what assertions may claim> |
| `cost-controls` | <the per-run bound in tokens and session count; how it is enforced> |
| `failure-diagnostics` | <what a red run leaves behind> |
| `boundary` | <what the harness deliberately does not do> |
| `decision-authority-stand-in` | <the stand-in supplying human decisions; or a stated not-needed reason> |

**(B) Reuse form** — if a harness able to drive the feature's primary
external interface already exists, cite it by path and list only this
feature's additions:

Harness: `<path to the existing harness>` — cite an existing harness module,
test helper, or fixture.

Additions:

<this feature's fixtures, fakes, helpers — what is new, not re-deciding
the cited harness's mechanics>

## Coverage map

*Each required behavior of the feature mapped to the scenario(s) that
prove it. A behavior this doc cannot route — not runnable in CI, not
sensibly mockable, not cleanly manual — is surfaced as an explicit
`open item — <reason>` disposition, never dropped and never silently
merged into another row.*

| Required behavior | Scenario(s) / disposition |
|---|---|
| <the required behavior, gisted> | <one or more scenario IDs from above, or `open item — <reason>`> |

## Scenario routes

*Every scenario named above carries exactly one row here, and exactly one
route from the closed enum. A `manual-human` scenario names its Owner;
every other route's Owner column reads `—`.*

| Scenario | Route | Owner | Cost class | Cadence |
|---|---|---|---|---|
| <S1> | <run-for-real \| mocked \| manual-human> | <the named owner if manual-human, else —> | <deterministic \| bounded-model \| full-session> | <per-change \| scheduled \| checkpoint \| on-demand> |

## Scenario build facts

*Every scenario carries exactly one row here, restated in the harness
section's own terms. No two scenarios name the same owned artifact.*

| Scenario | Fixture | Seams exercised | Owned artifact |
|---|---|---|---|
| <S1> | <the fixture this scenario requires> | <the seam(s) it exercises, named in the harness section> | <the one artifact this scenario exclusively owns> |

---

## Appendix A — well-formedness checklist *(walked at authoring and by review; delete from instantiated docs; mirrors `internal/e2e/docclass`)*

- [ ] Header complete: In one line, Relevant-when, Basis, Governing design(s), Status, Template version — all filled, no unrecognized field
- [ ] `Basis` carries a `prd:<feature>` join
- [ ] `Status` is exactly `draft` or `approved`
- [ ] `Template version` reads exactly `e2e-design/v2` — the class pin
- [ ] All seven sections present, in this document: Scenarios, Setup and teardown, Mock strategy, Harness design, Coverage map, Scenario routes, Scenario build facts
- [ ] No eighth section, and no section declared twice — the set is closed, and this template's own `## 0.` and `## Appendix A` are deleted at instantiation
- [ ] Every section is non-empty, or reads a stated `none — <reason>` — never silently blank, and never a bare `—`, `N/A`, `TBD` or a `**none**` that states as little
- [ ] Harness section is in exactly one of the two forms — build form with all slots present, or reuse form citing a harness by path with no slot rows
- [ ] In build form, every slot carries a structured answer or a blocking open item — never empty, never a placeholder token, never a bare library/tool name
- [ ] Every Coverage map row carries ≥ 1 scenario reference **declared in the Scenarios section**, or an explicit `open item — <reason>` disposition with its reason stated
- [ ] Scenario routes and Scenario build facts tables both list exactly the same scenario set as the Scenarios section
- [ ] Every scenario carries exactly one route (run-for-real, mocked, or manual-human), one cost class (deterministic, bounded-model, or full-session), one cadence (per-change, scheduled, checkpoint, or on-demand), and one owned artifact
- [ ] No artifact is named as owned by two scenarios
- [ ] Every `manual-human` route names an Owner — never `—`, blank, or a placeholder like `TBD`
- [ ] If any slot carries a blocking open item, `Status` must read `draft`, never `approved`

*Out of scope for this checklist, by design (`design:e2e-verification-artifacts-v2#3.5`): whether the Coverage map is* complete *against the feature's full required-behavior set — that is the e2e review's coverage-lens judgment, checked at review time, not authoring time; whether a slot's answer is* sufficient *to build from — that is the buildability lens's judgment.*
