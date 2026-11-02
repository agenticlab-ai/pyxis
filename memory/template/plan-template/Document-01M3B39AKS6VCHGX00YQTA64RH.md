# <Feature title> — plan <NNN>

*Guidance paragraphs look like this — italic. Delete every one when
instantiating: `plangate fold` parses a CLOSED grammar and guidance text is
a defect, not decoration. The authoritative grammar is
`internal/plangate/plan/parse.go` + the golden fixtures; this skeleton
mirrors it. The plan is the frozen stage-7 artifact (plan-new-v1 §3.1):
file `<paths.plans>/<feature>/<NNN>-plan.md`, dense-numbered from 001;
approval and freeze are the plan PR's human merge; issues are its
projection, never its source.*

| Field | Value |
|---|---|
| **Feature** | <slug — must equal the PRD ledger's directory name> |
| **PRD** | docs/prds/<slug> |
| **Design** | docs/tech-design/<slug or design-slug>.md |
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
| **Native routing** | {"schema_version":"task-native-routing/v1","implementation":{"harness":"<harness>","model":"<native model>","effort":"<native effort>"},"review":{"harness":"<harness>","model":"<native model>","effort":"<native effort>"},"rationale":"<why these choices suit this task>"} |
| **Pattern** | — |
| **Touches** | <plan-internal surface list — used for overlap sequencing, never rendered to workers; paths belong HERE and only here> |

*Field notes (delete): `Blocked by` lists task IDs (`T2, T3`) — every
ordering constraint is an explicit edge; the frontier (open ∧ unblocked ∧
unclaimed) is what executors consume. `Size band` estimates added logic
lines; thresholds live in `.pyxis/config.yaml` `plan:` — exceeding one
means split, or record `Size accepted: <why unsplittable>`. `Native routing`
is required on every task, including mechanical tasks. Run `plangate routes`
to read supported native pairs for each harness and role; choose implementation
and review independently and explain the choices. Do not invent a model or
effort, use a numeric rung, or substitute a tier label.*

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
