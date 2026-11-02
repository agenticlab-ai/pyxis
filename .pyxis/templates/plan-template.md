# <Feature title> — plan <NNN>

*Guidance paragraphs look like this — italic. Delete every one when
instantiating: `plangate fold` parses a CLOSED grammar and guidance text is
a defect, not decoration. The authoritative grammar is
`internal/plangate/plan/parse.go` + the golden fixtures; this skeleton
mirrors it. The plan is the frozen stage-7 artifact (plan-new-v1 §3.1):
file `<paths.plans>/<Feature-or-Intake>/<NNN>-plan.md`, dense-numbered from 001;
approval and freeze are the plan PR's human merge; issues are its
projection, never its source. A feature plan has Feature, PRD, and Design;
add Basis only for a draft-born feature. A non-feature plan has Intake instead
and deletes all four feature-only rows.*

| Field | Value |
|---|---|
| **Feature** | <slug — must equal the PRD ledger's directory name> |
| **PRD** | docs/prds/<slug> (document pipeline) or prd/<slug> (draft-born) |
| **Design** | docs/tech-design/<design-slug>.md (document pipeline) or design/<slug> (draft-born) |
| **Basis** | lock/<slug>/v<N> — draft-born feature only; delete this row for a document-pipeline plan |
| **Intake** | <non-feature plan slug — use this row instead of Feature, PRD, Design, and Basis> |
| **Plan version** | <NNN — must equal the filename number> |
| **Dialect** | harbormaster/fs-spec:v2 |
| **Epic title** | <the feature's tracker face — one imperative title> |
| **Status** | Draft — freezes at merge |
| **Template version** | plan-template/v2 |

## Tasks

*One `### T<n> — <imperative title>` per task, dense from T1 — IDs are
permanent (the plan-task-id is the tracker idempotency key; never renumber
after the plan PR opens). Each task is a tracer bullet: ONE behavior,
sliced vertically, independently verifiable. Wide refactors are the one
exception — declare them with `Pattern` as expand → migrate-batch(es) →
contract. Provenance-only work (vendoring, generated artifacts,
moves/renames) is a `Mechanical: yes` task: band 0, a `**Provenance:**`
list instead of Current/Desired/Key interfaces, never mixed into behavior
work.*

### T1 — <imperative title>

| Field | Value |
|---|---|
| **Blocked by** | — |
| **Size band** | <lo>–<hi> |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Requirements** | {"schema_version":"task-requirements/v1","implementation":{"assessment":{"reasoning":"<enum>","coupling":"<enum>","novelty":"<enum>","verification":"<enum>","impact_scope":"<enum>","architectural_role":"<enum>","citations":[{"id":"task:acceptance"}]},"rationale":"<why these values fit implementation>"},"review":{"assessment":{"reasoning":"<enum>","coupling":"<enum>","novelty":"<enum>","verification":"<enum>","impact_scope":"<enum>","architectural_role":"<enum>","citations":[{"id":"task:goal"}]},"rationale":"<why these values fit review>"}} |
| **Pattern** | — |
| **Touches** | <plan-internal surface list — used for overlap sequencing, never rendered to workers; paths belong HERE and only here> |

*Field notes (delete): `Blocked by` lists task IDs (`T2, T3`) — every
ordering constraint is an explicit edge; the frontier (open ∧ unblocked ∧
unclaimed) is what executors consume. `Size band` estimates added logic
lines; thresholds live in `.pyxis/config.yaml` `plan:` — exceeding one
means split, or record `Size accepted: <why unsplittable>`. `Requirements`
is required on every task, including mechanical tasks. Run `plangate dimensions`
to read allowed enum values and the derivation policy. Assess implementation
and review independently and explain each assessment. Citations name charter
sections (`task:goal`, `task:scope`, `task:acceptance`) or declared Traces.
Fold derives capability, deliberation, context, and assurance with policy
provenance; any supplied profile must match exactly. Harness, model, and effort
selection belongs to execution. Do not supply rungs or tiers.*

**Goal:** <the one behavior this task delivers, end to end>

**Current behavior:** <what the system does today at this seam — the worker's delta baseline>

**Desired behavior:** <what it does once this task is done, edge cases named>

**Scope:**
- <the included work, stated behaviorally — never file paths (those go in Touches)>

**Out of scope:**
- <what this task must not change, even though adjacent — `none — <reason>` is legal>

**Acceptance criteria:**
- [ ] <independently decidable: running X yields Y — the vagueness blacklist is enforced>

**Key interfaces:**
- <durable anchors: `TypeName`, function contracts, config shapes to find or change — never paths or line numbers>

**Traces:** <`prd:<slug>/FR-n` · `design:<slug>#3.n` · `corpus:FAMILY-n` — every ref must resolve>

## Coverage

*The completion criterion: every live Must FR/UB of the governing PRD maps
to ≥1 task; every task appears in ≥1 row; Should/Could rows are covered or
explicitly Deferred. An unmappable requirement is fog — the whole contract
bounces (all or nothing), never a vague task.*

| Requirement | Tasks |
|---|---|
| prd:<slug>/FR-1 | T1 |

## Deferred

*Optional. Non-Must rows deliberately not planned, each with a reason —
recorded decisions, never silence. Delete the section if empty.*

| Requirement | Reason |
|---|---|
| prd:<slug>/FR-9 | <why deferred, and to where> |
