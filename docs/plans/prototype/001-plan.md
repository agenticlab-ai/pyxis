# Prototype an idea before its contract — plan 001

| Field | Value |
|---|---|
| **Feature** | prototype |
| **PRD** | prd/prototype |
| **Design** | design/prototype |
| **Basis** | lock/prototype/v1 |
| **Plan version** | 001 |
| **Dialect** | harbormaster/fs-spec:v2 |
| **Epic title** | Prototype an idea before its contract is written |
| **Status** | Draft — freezes at merge |
| **Template version** | plan-template/v2 |

## Tasks

### T1 — Create and remove a prototype worktree for an open draft

| Field | Value |
|---|---|
| **Blocked by** | — |
| **Size band** | 160–260 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Requirements** | {"schema_version":"task-requirements/v1","implementation":{"assessment":{"reasoning":"complex","coupling":"composed","novelty":"adapt","verification":"composed","impact_scope":"contained","architectural_role":"feature_local","citations":[{"id":"task:acceptance"},{"id":"design:prototype#D-1"}]},"rationale":"Creates and removes a worktree and branch with ordered refusals and cleanup on failure, following the existing draft verb; nothing that exists today changes."},"review":{"assessment":{"reasoning":"complex","coupling":"composed","novelty":"adapt","verification":"composed","impact_scope":"contained","architectural_role":"feature_local","citations":[{"id":"task:acceptance"},{"id":"prd:prototype/UB-1"}]},"rationale":"Review must confirm removal acts only on the exact prototype worktree and that a failed creation leaves nothing behind."}} |
| **Pattern** | — |
| **Touches** | cmd/pyxis/draft.go, cmd/pyxis/draft_prototype.go, cmd/pyxis/draft_prototype_test.go, internal/draft/lifecycle/prototype.go, internal/draft/lifecycle/prototype_test.go |

**Goal:** An operator or agent can give an open draft's feature a local prototype worktree on its own branch, and remove it again, with one draft verb.

**Current behavior:** No draft verb creates a worktree. Worktrees for a feature are made by hand or by the interactive launcher, with no tie to a draft.

**Desired behavior:** `pyxis draft prototype <feature> --idea <file>` creates the branch `prototype/<feature>` and its worktree from the fetched mainline, with no upstream, in the launcher's worktree directory, and prints the worktree path, branch, and base commit as `key: value` lines. `--remove` deletes the worktree, the runtime state directory beside it, and the branch. Neither form pushes, opens a pull request, or touches the draft branch or the invoking checkout.

**Scope:**
- Add the create form with its ordered checks: slug rule, idea file present and not empty, local draft branch present, mainline fetched, no existing prototype branch or directory.
- On a failed worktree creation or confirmation, remove whatever was created, exit non-zero, and print the cause without retrying.
- Add the remove form, acting only on a registered worktree checked out on exactly the prototype branch, with Git's own refusal for uncommitted changes.
- Resolve the worktree directory from the Git common directory so it is the same from the main checkout and from a linked worktree.

**Out of scope:**
- Route resolution and launching a build; those are T3 and T4.
- Any change to `draft open`, `draft round`, `draft prepare-lock`, or `draft abandon`.

**Acceptance criteria:**
- [ ] In a temporary repository with an open draft, the create form yields a worktree on `prototype/<feature>` whose base equals the fetched mainline commit and which has no upstream; the draft branch and the invoking checkout are unchanged; the three lines are printed.
- [ ] Each refusal (invalid slug, missing or empty idea file, missing draft branch, existing prototype branch, existing directory) exits non-zero before any branch or directory is created and names what was wrong.
- [ ] With worktree creation forced to fail, the command exits non-zero with the cause and leaves no prototype branch, directory, or worktree registration.
- [ ] The remove form deletes the worktree, the state directory, and the branch of a prototype whose changes are committed; refuses a worktree with uncommitted changes and names the files; and exits zero reporting nothing to remove when none exists.

**Key interfaces:**
- The `pyxis draft` verb set and its lifecycle operations
- `draft.branch_prefix`, the configured remote and mainline branch
- Git worktree registration, `--no-track`, the Git common directory

**Traces:** prd:prototype/FR-2 · prd:prototype/UB-1 · prd:prototype/NR-1 · design:prototype#D-1

### T2 — Admit the prototype config block and launch purpose on the shared surfaces

| Field | Value |
|---|---|
| **Blocked by** | — |
| **Size band** | 120–220 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Requirements** | {"schema_version":"task-requirements/v1","implementation":{"assessment":{"reasoning":"complex","coupling":"composed","novelty":"adapt","verification":"composed","impact_scope":"multi_surface","architectural_role":"shared_substrate","citations":[{"id":"task:scope"},{"id":"design:prototype#D-5"}]},"rationale":"Adds by addition to three shared surfaces: the closed config schema, the harness launch port with the shared runtime, and the runtime constructor both composition roots use. Every existing launch and config must behave identically."},"review":{"assessment":{"reasoning":"complex","coupling":"composed","novelty":"adapt","verification":"composed","impact_scope":"multi_surface","architectural_role":"shared_substrate","citations":[{"id":"task:acceptance"},{"id":"prd:prototype/NR-2"}]},"rationale":"Review must confirm that no existing configuration, launch purpose, or implement-now behavior changes, since every harness and every tool that reads config passes through these surfaces."}} |
| **Pattern** | — |
| **Touches** | internal/docgate/config/config.go, internal/docgate/config/prototype_test.go, internal/executor/port/agentruntime.go, internal/executor/runtime/brief.go, internal/executor/runtime/access.go, internal/executor/runtime/memory.go, internal/executor/runtime/brief_test.go, internal/executor/runtime/access_test.go, cmd/implement-now/resolver.go, internal/executor/harnessbuild/ |

**Goal:** The shared surfaces accept what a prototype build needs: a `prototype` config block, a prototype launch purpose, and one runtime constructor usable outside implement-now.

**Current behavior:** The closed config schema rejects a `prototype` block. The harness port knows two launch purposes, and the shared runtime rejects any other. The function that constructs a harness runtime with the effective catalog lives inside the implement-now composition root.

**Desired behavior:** The config schema admits an optional `prototype` block holding `harness` and `profile` together. The port has a `prototype` launch purpose: the shared brief assembler gives it its own protocol in place of the delivery protocol, launch access gives it the confinement of an implementation launch, and it receives no memory pack. The runtime constructor lives in a shared package and implement-now calls it there.

**Scope:**
- Parse the `prototype` block closed: both keys or absent; one key alone, an unknown harness name, and an unknown key are config defects.
- Add the `prototype` launch purpose and its brief protocol: non-interactive, isolated worktree on the named branch, nothing published, changes left in the working tree, no Git mutation, findings written to the scratch findings file.
- Admit the new purpose at the shared runtime's launch-access and memory-context switch points.
- Move the runtime constructor that passes the effective catalog to each adapter into a shared internal package, behavior unchanged, and call it from implement-now.

**Out of scope:**
- Resolving a route or launching a build; those are T3 and T4.
- Any prototype-specific code in a harness adapter.

**Acceptance criteria:**
- [ ] A config with a complete `prototype` block parses; one key alone, an unknown harness, and an unknown key under the block each produce a named config defect; a config without the block parses as before.
- [ ] For a launch spec with the `prototype` purpose, the assembled brief carries the charter followed by the prototype protocol and none of the delivery protocol's gate or proposal text, and launch access resolves with implementation confinement.
- [ ] The existing harness conformance suites, brief and access tests, and implement-now's suite pass unchanged with implement-now constructing its runtimes through the shared package.

**Key interfaces:**
- `LaunchPurpose`, `LaunchSpec`, the shared brief assembler, launch access
- The closed config schema and its defect reporting
- The harness catalog's profiles and each adapter's profiles option

**Traces:** prd:prototype/FR-6 · prd:prototype/NR-2 · design:prototype#D-2 · design:prototype#D-5

### T3 — Resolve the prototype's harness and model from config or the asking session

| Field | Value |
|---|---|
| **Blocked by** | T1, T2 |
| **Size band** | 120–200 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Requirements** | {"schema_version":"task-requirements/v1","implementation":{"assessment":{"reasoning":"complex","coupling":"composed","novelty":"adapt","verification":"composed","impact_scope":"contained","architectural_role":"feature_local","citations":[{"id":"task:acceptance"},{"id":"design:prototype#D-2"}]},"rationale":"Chooses a harness and catalog profile from the config block or the ambient flags against one effective catalog snapshot, with named refusals before anything is created."},"review":{"assessment":{"reasoning":"complex","coupling":"composed","novelty":"adapt","verification":"composed","impact_scope":"contained","architectural_role":"feature_local","citations":[{"id":"task:acceptance"},{"id":"prd:prototype/FR-6"}]},"rationale":"Review must confirm the resolution order, the lowest-cost rule, and that an unusable configured route is refused and never replaced by the ambient one."}} |
| **Pattern** | — |
| **Touches** | cmd/pyxis/draft_prototype.go, cmd/pyxis/draft_prototype_test.go, internal/draft/lifecycle/prototype_route.go, internal/draft/lifecycle/prototype_route_test.go |

**Goal:** The prototype verb decides, before it creates anything, which harness and model profile will build the prototype, and says so.

**Current behavior:** After T1 the verb creates a worktree and makes no choice of harness or model.

**Desired behavior:** The verb loads the effective harness catalog once, tracked config merged with the machine-local overlay. With a `prototype` block it takes that harness and profile; without one it takes `--ambient-backend` and the catalog entry whose model equals `--ambient-model`, the lowest cost rank when several share the model. It prints the route as `configured <harness> <profile>` or `ambient <harness> <profile>` with the other lines. A route that cannot be resolved or cannot run refuses before the worktree is created.

**Scope:**
- Add `--ambient-backend` and `--ambient-model` to the create form and resolve the route in the documented order from one catalog snapshot.
- Refuse, naming the `prototype` config block, when the block is absent and an ambient flag is missing or no catalog entry holds the ambient model.
- Refuse a configured profile the harness's catalog does not hold, a harness no runtime can be constructed for, and a harness that fails its authentication check, without falling back to the ambient route.
- Print the resolved route line and hand the route to the build step.

**Out of scope:**
- Launching the build; that is T4.
- Choosing a model level per request or metering spend; those wait in the contract's waiting room.

**Acceptance criteria:**
- [ ] With a valid `prototype` block the printed route is `configured <harness> <profile>`, including for a profile that exists only in the machine-local overlay.
- [ ] With no block and both ambient flags, the route is `ambient <harness> <profile>` for the catalog entry with that model, and the lowest cost rank is chosen when two entries share the model.
- [ ] Each refusal exits non-zero before any branch or directory exists and names its cause; a configured route that cannot run is not replaced by the ambient route.

**Key interfaces:**
- The `prototype` config block
- The effective harness catalog loader and its profile entries with model, effort, and cost rank
- The shared runtime constructor and the runtime's authentication check

**Traces:** prd:prototype/FR-6 · prd:prototype/UB-1 · design:prototype#D-2

### T4 — Launch the prototype build headless and commit what it leaves

| Field | Value |
|---|---|
| **Blocked by** | T2, T3 |
| **Size band** | 220–300 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Requirements** | {"schema_version":"task-requirements/v1","implementation":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"adapt","verification":"cross_boundary","impact_scope":"contained","architectural_role":"feature_local","citations":[{"id":"task:acceptance"},{"id":"design:prototype#D-5"}]},"rationale":"Owns a child process end to end across the harness runtime: prepared runtime state and login, a time bound with a kill, and a commit that must wait for the runtime to acknowledge the process tree stopped."},"review":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"adapt","verification":"cross_boundary","impact_scope":"contained","architectural_role":"feature_local","citations":[{"id":"task:acceptance"},{"id":"prd:prototype/UB-1"}]},"rationale":"Review must confirm that no path stages or commits while a writer may still be alive, that failures never switch route, and that the launch really receives its runtime state and login."}} |
| **Pattern** | — |
| **Touches** | cmd/pyxis/draft_prototype.go, cmd/pyxis/draft_prototype_test.go, internal/prototype/, internal/draft/lifecycle/prototype.go |

**Goal:** The prototype verb builds the prototype as one headless harness process on the resolved model, waits for it, and leaves its work committed on the prototype branch with a report.

**Current behavior:** After T3 the verb creates the worktree and prints a route, and nothing is built.

**Desired behavior:** The verb constructs the resolved harness's runtime with the catalog snapshot the route came from, prepares the isolated runtime state and login snapshot for the worktree, and launches under the `prototype` purpose with a brief made of the idea file, the build rules, and the protocol. It waits under the engine's default worker time bound. When the runtime acknowledges the process tree stopped, it commits everything in the worktree on the prototype branch and prints the model, effort, outcome, head commit, findings file path or `findings: missing`, and reported usage. A launch error, abnormal exit, or time-out exits non-zero with the cause and never switches route. When the runtime cannot prove the process tree stopped, the verb stages, commits, and removes nothing.

**Scope:**
- Construct the runtime through the shared constructor, prepare runtime state with the runtime's existing task-state preparation, and launch with the worktree, head branch, model, effort, and state directory.
- Assemble the brief from the idea file marked as data, the build rules, and the prototype protocol.
- Await under the default worker bound; on expiry kill the process and report `failed: timed out`.
- Commit and report only on an acknowledged stop; on a not-quiescent report leave the worktree, branch, and runtime state untouched and report that cause.

**Out of scope:**
- The implement-now worker cycle: no tracker item, claim, code gate, review, proposal, or task-session record.
- A spend meter, retries, or a fallback to another route.

**Acceptance criteria:**
- [ ] With a stub runtime, a configured route and an ambient route each launch once in the prototype worktree on the resolved model and effort, and after exit everything in the worktree is committed on the prototype branch and the report lines are printed.
- [ ] With a fixture harness that has a file-based login and no API token in the environment, the prepared state directory and login snapshot reach the launch through the shared launch-access and launch-environment functions, and a profile present only in the local overlay passes the real adapter's catalog check with the resolved model and effort.
- [ ] A launch error, an abnormal exit, and a time-out with an acknowledged kill each exit non-zero with the cause, keep the worktree, and start no second launch.
- [ ] When the stub runtime reports that the process tree could not be proved stopped, from launch or from await, no file is staged, no commit is created, and nothing is removed.

**Key interfaces:**
- `AgentRuntime` launch, await, and kill; the not-quiescent sentinel
- The runtime's task-state preparation and the launch's runtime state directory
- The engine's default per-worker wall-clock bound
- The scratch findings file in the prototype worktree

**Traces:** prd:prototype/FR-6 · prd:prototype/FR-4 · prd:prototype/UB-1 · design:prototype#D-5

### T5 — Ship the prototype guide, skill, findings template, and the guide passages that use findings

| Field | Value |
|---|---|
| **Blocked by** | T4 |
| **Size band** | 20–60 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Requirements** | {"schema_version":"task-requirements/v1","implementation":{"assessment":{"reasoning":"complex","coupling":"contained","novelty":"adapt","verification":"composed","impact_scope":"contained","architectural_role":"feature_local","citations":[{"id":"task:acceptance"},{"id":"design:prototype#D-4"}]},"rationale":"Writes one new guide, a skill pointer, a record template, and short passages in existing guides that apply only to a draft holding a prototype record, plus one fixture test."},"review":{"assessment":{"reasoning":"complex","coupling":"contained","novelty":"adapt","verification":"composed","impact_scope":"contained","architectural_role":"feature_local","citations":[{"id":"task:acceptance"},{"id":"prd:prototype/FR-5"}]},"rationale":"Review must confirm the passages change nothing for a draft without a prototype record and that the guide never has the asking session build or publish the prototype."}} |
| **Pattern** | — |
| **Touches** | prototype-guide (new) · the system skills tree's prototype home (new) · the prototype-record template (new) · feature-brainstorm-guide · prd-new-guide · tech-design-brainstorm-guide · tech-design-new-guide · prd-review-guide · tech-design-review-guide · argus-guide · pipeline-map · internal/draft/readiness/prototype_record_test.go · memory/authoring/ · memory/templates/ |

**Goal:** Someone handing Pyxis an idea can ask for a prototype through a skill, get a findings record in the draft, and have every finding answered before the lock.

**Current behavior:** No guide, skill, or template describes a prototype, and no authoring or review guide mentions prototype findings.

**Desired behavior:** A prototype guide and skill pointer tell the asking session to write the idea file, run the verb with its own harness and model as the ambient flags, read the report and findings, write the findings record, and release the prototype when the draft locks or is abandoned. A `prototype-record/v1` template defines the record and its `L-n` findings. The feature-brainstorm, PRD, design-brainstorm, tech-spec, and both review guides, the Argus drive guide, and the pipeline map carry the passages that answer findings, treat an `open` finding as blocking, run a prototype only when asked, and list the skill.

**Scope:**
- Write the prototype guide with its steps, Never list, and release passage, and the skill pointer holding invocation glue only.
- Write the prototype-record template with its header, the built and left-out sections, the finding block, and a structure checklist.
- Add the findings passages to the six stage guides and the Argus drive guide, and list the skill in the pipeline map.
- Add a test showing a draft that holds a prototype record compiles and resolves a citation to one of its findings with draft readiness unchanged.

**Out of scope:**
- Any change to the draft compiler, draft readiness, or the review reports' format.
- A mechanical lock check on unanswered findings; it waits in the contract's waiting room.

**Acceptance criteria:**
- [ ] The skill directory holds the pointer and the authoring directory holds the guide, whose command line passes `--idea`, `--ambient-backend`, and `--ambient-model`, and whose Never list forbids pushing the prototype branch, opening a pull request from it, and building the prototype in the asking session.
- [ ] A draft fixture holding a record instantiated from the template compiles, and a citation to its first finding in the brainstorm grammar resolves, with no change to readiness code.
- [ ] Each of the seven named guides carries its passage inside its draft-born section and the pipeline map lists the skill; the existing draft, readiness, and review suites pass unchanged.

**Key interfaces:**
- The vendor-neutral agent surface: authoring guides, skill pointers, templates
- The brainstorm citation grammar and the draft's numbered brainstorm working files
- The `Answer` line of a finding: `open`, a citation, or `set aside` with a reason

**Traces:** prd:prototype/FR-1 · prd:prototype/FR-3 · prd:prototype/FR-4 · prd:prototype/FR-5 · prd:prototype/NR-1 · prd:prototype/NR-2 · design:prototype#D-2 · design:prototype#D-3 · design:prototype#D-4

## Coverage

| Requirement | Tasks |
|---|---|
| prd:prototype/FR-1 | T5 |
| prd:prototype/FR-2 | T1 |
| prd:prototype/FR-3 | T5 |
| prd:prototype/FR-4 | T4, T5 |
| prd:prototype/FR-5 | T5 |
| prd:prototype/FR-6 | T2, T3, T4 |
| prd:prototype/UB-1 | T1, T3, T4 |
| prd:prototype/NR-1 | T1, T5 |
| prd:prototype/NR-2 | T2, T5 |
