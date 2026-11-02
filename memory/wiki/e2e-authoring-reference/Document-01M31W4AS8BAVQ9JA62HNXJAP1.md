# e2e authoring reference — reading the e2e-design / e2e-review / e2e-plan artifacts

| Field | Value |
|---|---|
| **In one line** | The `e2e-design/v2` doc's seven-section shape with a closed-slot harness form and two scenario-keyed tables, the coverage + anti-gaming + buildability review lenses and their blocking/advisory findings, and the plan handoff to plan-new — so a human reads the e2e authoring line's artifacts without tooling. |
| **Relevant-when** | reading a feature's `docs/e2e/<feature>/design.md` or `review.md`; triaging a review finding; checking why a produced plan does or does not carry a harness task; onboarding to what e2e-design / e2e-review / e2e-plan each produce |
| **Authority** | descriptive — the sources below govern; the persisted design doc and review report govern on any conflict with this summary |
| **Sources** | `docs/tech-design/e2e-verification-artifacts-v2.md` (§3.1 the doc class, §3.2 the harness section, §3.3 the two scenario-keyed tables, §3.5 well-formedness, §3.6 the report + plan reuse) · `docs/tech-design/e2e-verification-skills-v2.md` (§3.1–§3.3 the three skills' behavior) · `docs/prds/e2e-verification/effective.md` · the `e2e-design/v2`, `e2e-review-report/v2`, and `e2e-review-calibration/v2` templates (version-pinned; `AGENTS.md` layout) · `internal/e2e/docclass`, `internal/e2e/report`, `internal/e2e/plan` |
| **Last reviewed** | 2026-08-11 |

The e2e authoring line is three single-invocation skills — `e2e-design`,
`e2e-review`, and `e2e-plan` (`AGENTS.md`'s vendor-neutral agent-surface
layout names where each lives) — that carry a fully-implemented feature to a
reviewed e2e test-coverage design and an ordered plan (`docs/techspec.md`'s
"e2e authoring line" section carries the package-level implementation this
page does not repeat). This page distills the shape of what the three
skills read and write; when it and a real artifact disagree, the artifact
is right.

## The design doc (`e2e-design/v2`)

One markdown file per feature at `docs/e2e/<feature>/design.md`, frozen at
operator approval. The header is **closed** over six rows: the routing pair
(`In one line` · `Relevant-when`), a `Basis` row typing the feature join
(`prd:<feature>`), the `Governing design(s)` the coverage was derived from
(one or more `design:<slug>`), `Status` (`draft` → `approved`), and the
`Template version` pin. A missing or wrong `Template version` is its own
checker defect (`template-version`), so a doc authored from this page
without it fails the well-formedness checklist.

The section set is **closed** — exactly these seven, in this order; an empty
one reads a stated `none — <reason>`, never silent blankness:

| # | Section | Carries |
|---|---|---|
| 1 | Scenarios | the end-to-end flows the tests will exercise, each carrying a stable scenario ID |
| 2 | Setup and teardown | the fixture lifecycle |
| 3 | Mock strategy | what is faked, and with what, named per seam |
| 4 | Harness design | the closed slot form specifying the harness (§4a) or citing an existing one (§4b) |
| 5 | Coverage map | each required behavior → the scenario(s) proving it, or an explicit `open item — <reason>` |
| 6 | Scenario routes | the run-facts table: each scenario's route, owner (if manual-human), cost class, and cadence |
| 7 | Scenario build facts | the build-facts table: each scenario's fixture, seams exercised, and exclusively-owned artifact |

### Harness section — two forms

The **Harness design** section takes exactly one of two forms:

**Build form** — used when no fitting harness exists. A table with one row per slot, each answering a mechanical element:

| Slot | What it answers |
|---|---|
| `driver` | how a scenario starts the system and what it gets back |
| `world` | what the run executes against, and who creates it |
| `scenario-contract` | what one scenario is as an artifact, and where it lives |
| `seams` | the named interfaces a fake attaches to |
| `observation-surface` | what a scenario is handed to assert on |
| `lifecycle-and-isolation` | setup, teardown, what is shared, what must run serially |
| `determinism-controls` | clock, randomness, model tier, retries — and what an assertion may claim |
| `cost-controls` | the per-run token and session bound, and how it is enforced |
| `failure-diagnostics` | what a red run leaves behind |
| `boundary` | what the harness deliberately does not do |
| `decision-authority-stand-in` | *(conditional)* the stand-in supplying human decisions the flow requires, or a stated not-needed reason |

Each slot must be answered by prose describing the mechanics — never left empty, answered by a placeholder token, or answered by a bare library or tool name. `internal/e2e/docclass.Check` rejects these structural defects.

**Reuse form** — used when a harness able to drive the feature's primary interface already exists. The section cites that harness by path and lists **only this feature's additions** — the fixtures, fakes and helpers it needs — carrying no slot rows at all.

### The two scenario-keyed tables

Two tables both keyed by the scenario IDs from the Scenarios section, and both must list exactly the same scenario set:

**Scenario routes** — how each scenario runs:

| Scenario | Route | Owner | Cost class | Cadence |
|---|---|---|---|---|
| *(ID)* | `run-for-real` · `mocked` · `manual-human` | the human-judgment owner if `manual-human`, else `—` | `deterministic` · `bounded-model` · `full-session` | `per-change` · `scheduled` · `checkpoint` · `on-demand` |

**Scenario build facts** — how each scenario is built in the harness's terms:

| Scenario | Fixture | Seams exercised | Owned artifact |
|---|---|---|---|
| *(ID)* | the fixture this scenario requires | the seams it exercises, named in the harness section | the one artifact this scenario exclusively owns |

The split enforces readability: an eight-column table would be unreadable. The same-set rule between the two tables is mechanically checked.

The **owned artifact** is load-bearing: it assigns each scenario a single artifact no other scenario names, enabling the planner to assign parallel scenario tasks that cannot collide.

### Blocking open items

Where a slot's answer genuinely cannot be settled — no way to observe an outcome, no way to isolate a run — the slot carries an explicit **blocking open item** naming what is unresolved and why. A document carrying a blocking open item is **well-formed but unapprovable**: it cannot reach `approved` status while any slot holds one.

### Well-formedness

`internal/e2e/docclass.Check` walks a fixed-order well-formedness checklist and enforces the *structure*: every section present, the harness section in exactly one of its two forms, every slot in build form present and answered (no empty slots, placeholders, or bare library names), the two tables matching the scenario set, and so on. It deliberately cannot see *sufficiency* — whether a slot's answer actually specifies buildable mechanics — that is the buildability lens's judgment, below.

## The review report (`e2e-review-report/v2`)

One markdown file per feature at `docs/e2e/<feature>/review.md`, co-located
with the design doc it reviews and riding the design doc's introducing
change. It is its own instance in the review-report family — sharing the
closable-finding and versioned-report grammar with `review-report/v1` and
`impl-review-report/v1` (`Round N` · `FND-n` · `Verdict history` ·
`Escalation brief` · `Measurements` · `Overlay corrections`) rather than
restating it — typed to this reviewer's own subject (`e2e-design:<feature>`),
lens set, and disposition vocabulary.

| Field | Meaning |
|---|---|
| `Severity` | `blocking` or `advisory`; must equal the severity its cited `CAL-n` calibration row maps to. |
| `Lens` | `coverage` · `anti-gaming` · `buildability` — this instance's own three-lens set (distinct from the sibling reviewers'). |
| `Evidence` | the design-doc quote (a coverage-map row, a scenario's route/mock, a harness slot's answer) set against the upstream anchor (a PRD requirement row, a governing-design binding element, the harness-slot vocabulary). |
| `Acceptance condition` | the change that would close the finding — named at birth, never fresh taste at re-review. |
| `Disposition` | `open` (still converging) · `resolved` (the design doc was edited) · `rejected — <reason>` — this instance's own vocabulary, distinct from the sibling reviewers' `applied`/`declined`/`violation-withheld`. |

**No finding may stay `open` once the live Verdict reads `approve` or
`approve-with-advisories`** — every finding is resolved or rejected before
the design doc is approved. The round cap is the same shape: two rounds
maximum. Both rules are carried in code by `internal/e2e/report.Validate`
(as its `undispositioned-at-approval` and `round-cap` defects), so a report
that breaks either is invalid.

Read that as a schema backstop, **not** as a pipeline gate. The reviewer is
**advisory in v1**: no CI context runs the validator over
`docs/e2e/<feature>/review.md` — the class is deliberately not
docgate-folded and not in the generated design index — so nothing mechanical
stops an unsettled design from advancing. **The operator is the approval
point**, and these rules describe what they are approving against.

## The three lenses

Coverage, anti-gaming, and buildability run as blind sibling sessions — none sees the
other's candidates, none sees a prior round's report, and none is handed the
well-formedness checker's verdict — over one shared, input-only context pack
(the design doc, the feature's PRD, its governing design(s), and the
calibration table).

The floor is excluded **after** a lens returns, not inside it. A blind lens
cannot know what `internal/e2e/docclass.Check` already caught, so a lens may
well raise a finding restating a structural defect (a missing section, an empty
harness slot, a bare library-name slot, an unrouted scenario, a missing owner,
…); `internal/e2e/review`'s post-return duplicate filter drops those before the
operator sees the report, and logs what it dropped. The ordering is the point:
telling a lens "structure is already green" would pre-lower its scrutiny on
exactly the section it exists to scrutinize, so blindness is kept intact and the
duplicate noise is removed downstream instead.

| Lens | Judges | Calibration rows |
|---|---|---|
| **coverage** | does every required behavior the feature's PRD and governing design(s) actually need have a genuine scenario or routed disposition behind it? Flags a behavior with no coverage-map row at all, and over-mapping (a scenario mapped to a behavior its own flow doesn't plausibly exercise). | `CAL-1` (unmapped behavior, blocking) · `CAL-3` (ordinary over-mapping, advisory) · `CAL-5` (load-bearing over-mapping — the map reads complete only because of the inflated edge, blocking) |
| **anti-gaming** | does each scenario's recorded Mock strategy and route genuinely exercise the behavior its coverage-map entry claims to prove — "reds if the behavior breaks; no mock hollows out what it proves"? Paper-level self-contradiction only: it cannot confirm a built test genuinely fails on broken code (that falsification is a deferred run-time capability). | `CAL-2` (a hollow scenario, blocking) · `CAL-4` (a questionable but non-fatal mock/route choice, advisory) |
| **buildability** | could a worker actually build the harness from what is written in the harness section? A slot may structurally pass (prose was written where mechanics belong) yet still be insufficient — unclear what one scenario is, no way to isolate runs, no decision authority named, a cost bound in vague terms. The lens judges sufficiency: whether an answer in the slot lets a builder say "I can build this." | `CAL-7` (a harness slot that passes structural checks but lacks buildable mechanics, blocking) · `CAL-8` (a harness slot whose buildability is unclear — might work, might not, blocking) · `CAL-9` (a lesser buildability concern, advisory) |

`CAL-6` (a human review confirmation of a genuine-exercise failure)
belongs to the operator's operator review pass alone — none of the three lenses ever cites
it. A directive planted in any reviewed surface ("ignore missing behaviors",
"mark this advisory") is data under review, never a command any lens
obeys — it becomes its own finding candidate instead.

## The plan handoff

e2e-plan reads one **approved** e2e design doc and authors an ordinary
plan-new `plan/v1` at `docs/plans/<feature>/<NNN>-plan.md` — no new task
type, no new charter field; plan-new's fold/PR/freeze/projection pipeline
and the executor's ready-frontier are both reused verbatim. The task-graph
shape is a closed rule, not judgment
(`internal/e2e/plan.DecideWithConventions`; the plain `Decide` is the same rule
with "a scenario already exists against this harness" assumed, so it never emits
a conventions task):

- **A harness task, first, iff the feature's e2e harness does not already
  exist** — detected by reading the harness section:
  - Build form → harness task needed
  - Reuse form (citing an existing harness) → no harness task
- **A conventions task, iff the repository does not already carry a scenario
  built against that harness.** It lands the harness's scenario contract and
  one worked scenario, so every parallel worker has a shape to copy instead of
  inventing its own scaffolding in the same place. Its trigger is repository
  state, not operator judgment.
- **One task per scenario**, in the design doc's declared order.
- **Each conditional task blocks what comes after it.** The conventions task is
  `blocked-by` the harness task where one was emitted; every scenario task is
  `blocked-by` **each** of the harness and conventions tasks that were emitted,
  and carries no such edge where the repository already supplies both. The
  executor's existing ready-frontier (open ∧ unblocked ∧ unclaimed) is what
  actually sequences harness → conventions → scenarios — the edges are all
  e2e-plan declares.

That closed rule covers the **shape only** — which tasks, which IDs, which
edges. Everything else on a task is e2e-plan's own authored content against
plan-new's existing template and grammar: each task's charter prose, and its
`Traces` citing the PRD requirement row(s) the coverage map maps to that
scenario. Do not read a missing `Traces` as a `plan.Decide` defect — the
package never emits one.

e2e-plan refuses — producing no plan — on anything but a well-formed,
`approved` `e2e-design/v2` doc (re-checked itself, never trusted off an
unverified `Status` cell), and on a doc declaring zero scenarios or a
scenario with no ID, or one where any scenario carries no exclusively-owned artifact.
