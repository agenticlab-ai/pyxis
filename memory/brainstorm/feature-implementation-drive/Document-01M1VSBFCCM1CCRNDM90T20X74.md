# Feature implementation drive — brainstorm record

| Field | Value |
|---|---|
| **In one line** | A feature-scoped implementation drive that advances a dependency DAG through human-reviewed task PRs on a remote feature branch, verifies the integrated feature end to end, and lets execution be owned locally or handed to a remote engine. |
| **Relevant-when** | brainstorming or contracting end-to-end implementation of one feature across implement-now sweeps, task-PR review and ordered landing, feature-branch integration, terminal e2e, or exclusive local-versus-remote execution ownership; checking the boundary with implement-now, e2e-verification, the merge lane, or fleet execution |
| **Idea** | "in future, we are planning to run multiple arugs agent on the same repo at the same time, and as part of the development pipeline each of them will run implement-now after the planning and task creations" · "i want each argus owns the implementation end to end on the same worktree (given we need to run e2e testing at the end of the implementation). Also we should consider the world in which we run the implement-now engine on a separate machine (without knowing about features, etc) and its only job to implement whatever issue is being tagged." · "for each individual feature, create a feature-branch (remote), add all the prs to that branch and test against that branch. this approach enables both one implement-now per feature or engine execution on a remote server given the remote branch is availabe" · "i want the human to have the ability to review every single pr and provide feedback if needed" · "one implement-now <feature-name> that ensures all the implementation happens on the same machine that triggered that skill (implement-now) and general exeuction engine on remote machine if there is no local implementation kicked off. so the human can decide on implementing the tasks locally on their machine, or let the remote engine implement and tst" (operator, 2026-07-25, verbatim, five turns) |
| **Operator** | @mdarabi |
| **Date(s)** | 2026-07-25 |
| **Status** | gated: ready-for-prd |
| **Tracking** | — |
| **Feeds** | docs/prds/feature-implementation-drive/ |
| **Prior records** | brainstorm:implement-now · brainstorm:e2e-verification · brainstorm:implementation-verification · brainstorm:argus-unattended-drive |
| **Template version** | brainstorm-record/v1 |

## Orientation

| Source | Why loaded |
|---|---|
| `docs/product-vision.md` (§1, §4, §6) | governing outcome and pipeline: stage 8 produces one reviewed proposal per task; stage 10 verifies the merged feature; the engineer defines and accepts rather than operating machinery |
| `docs/prds/README.md` (PLAN-4..7, IMPL-1..10, GATE-4..7) | requirement routing: dependency-ordered plans, kickoff routing, completion monitoring, cross-machine leases/workspaces, feedback rounds, and merge/verification precedents |
| `docs/domain-model.md` (§3.2–3.4, §5) | entity and lifecycle ground: Feature→WorkItem DAG, Worker/Lease/Workspace, Proposal/FeedbackRound, feature-level E2EVerification, and kind-blind Node |
| `docs/wiki/glossary.md` | established terms: in-session run, fleet execution, umbrella policy, proposal boundary, and e2e coverage map |
| `docs/brainstorms/implement-now.md` | frozen executor rulings: same worker core local/remote, tracker-mediated claims, ready-frontier DAG sweeps, genuinely-green task PRs, and never merge |
| `docs/prds/implement-now/effective.md` + `cmd/implement-now/` + `internal/executor/` | binding and as-built executor surface: configurable routing label and base, remote task-claim CAS, isolated worktrees, one sweep, and driver-owned landing/re-invocation |
| `docs/brainstorms/e2e-verification.md` + `docs/prds/e2e-verification/effective.md` | adjacent stage-10 ruling: once per feature after all tasks are implemented/reviewed; e2e authoring ships first and the implement-now build/run adaptation is an explicit follow-on |
| `docs/brainstorms/argus-unattended-drive.md` | adjacent unattended-driver ground: tracker convention is the fleet interface, normalized wake/resume, human path preserved, and fleet internals remain outside feature orchestration |

**Overlaps surfaced:**

- `brainstorm:implement-now` / `prd:implement-now` — **partially covered, deliberately driver-owned**: the executor already claims and builds a label-selected ready frontier from a configurable remote base, produces green task PRs, and never merges. It does not own a feature lifecycle, task-PR human-feedback/landing loop, terminal e2e, or exclusive feature-level local-versus-remote ownership.
- `brainstorm:e2e-verification#D-4/#D-10/#D-14` / `prd:e2e-verification` — **explicit follow-on, not duplicate**: terminal once-per-feature e2e and DAG-ordered e2e-test tasks are fixed, while building/running the e2e line and adapting implement-now are deferred. This idea supplies the integration surface and execution-drive seam that follow-on needs; it must not redesign the e2e authoring artifacts.
- `docs/domain-model.md` E2EVerification + PLAN-5 — **pattern ancestor, not implementation contract**: a terminal verification step blocked by feature work already exists conceptually, but no feature-branch integration/ownership mechanism is specified.
- `brainstorm:argus-unattended-drive` — **coexisting driver precedent**: unattended materialization and wake/resume are covered; feature-scoped execution ownership and local-versus-remote arbitration are not.
- Existing task-level git-ref claims prevent duplicate issue execution across machines, but they do **not** guarantee that every task in one feature—including tasks becoming ready later—stays on the machine that initiated a local feature drive.

## Framings

- **A — Feature integration drive.** The feature is the missing stage-8→10 orchestrator: create one remote integration branch, repeatedly run the unchanged executor against the feature's label/base, place every task PR before a human, land approved PRs in DAG order, and run terminal e2e on the integrated SHA before one final PR to main. Local and remote execution are deployment choices behind the drive. Thesis: *the durable product object is the feature's integration-and-verification lifecycle; execution location is policy.*
- **B — Execution reservation and handoff.** The feature is a feature-level ownership protocol above task leases: `implement-now <feature>` atomically reserves all present and future tasks for the invoking machine, while an unreserved feature is eligible for a remote engine; pause, expiry, takeover, and explicit handoff preserve work. Feature branches and terminal e2e are the proving workflow for that ownership. Thesis: *the unsolved risk is split-brain execution; integration is the context in which ownership matters.*
- **C — Feature delivery workspace.** The feature is a remote, committed workspace for one deliverable: a feature branch plus scoped task frontier, review queue, exact-SHA e2e checkpoint, and final promotion to main. Any number of feature-blind workers may supply task PRs, while the human controls admission and acceptance. Thesis: *the stable abstraction is not an Argus run or a machine lease but a reviewable feature delivery surface that survives processes and machines.*

**Destination:** **A with B mandatory, grounded in C's durable state** — the feature is the end-to-end **feature integration drive**: one remote feature branch receives DAG-ordered, human-reviewed task PRs; the integrated exact SHA is terminally e2e-verified before one final proposal to main. Exclusive local-versus-remote execution reservation and handoff is mandatory behavior inside that lifecycle, while the feature delivery workspace is the durable state that survives processes and machines. — Operator accepted the recommendation (2026-07-25 · framing turn).

## Decisions

### D-1 — destination: integration drive with mandatory execution ownership

- **Question**: is the feature fundamentally the integration drive (A), execution reservation/handoff (B), or durable delivery workspace (C)?
- **Answer**: **A with B as a mandatory capability**, grounded in C: own the complete feature integration/review/e2e lifecycle; guarantee local-versus-remote execution ownership within it; persist progress in the remote feature branch and associated durable state.
- **Reasoning**: the operator accepted the recommendation. The user-visible job is one integrated, reviewed, e2e-verified feature; execution ownership is essential behavior but too narrow to define the whole feature, and the workspace is its durable substrate rather than its purpose.
- **Anchor**: 2026-07-25 · framing turn
- **Status**: live

### D-2 — segments: human feature owner first, autonomous driver second

- **Question**: who is primary and secondary, and which work should deliberately remain outside the feature drive?
- **Answer**: **Primary:** a human engineer executing and reviewing one planned feature end to end, choosing local or remote execution. **Secondary:** an autonomous driver such as Argus using the same contract unattended. **Anti-segment:** ad hoc standalone issues outside a frozen feature DAG; they remain in the generic executor pool without feature-branch or terminal feature-e2e requirements.
- **Reasoning**: operator confirmed the recommended boundary. It preserves one feature-drive contract across attended and unattended use without burdening generic issue execution with feature lifecycle machinery.
- **Anchor**: 2026-07-25 · segments turn
- **Status**: live

### D-3 — primary job: reviewed DAG to one verified feature proposal

- **Question**: what primary job does the human hire the feature drive to perform?
- **Answer**: “I have an approved, dependency-ordered feature plan. Execute it locally or remotely, present every task PR for my review, advance only after approved changes land, assemble the whole feature off main, prove the integrated revision through e2e, and give me one final PR to accept.”
- **Reasoning**: operator approved the recommended job. It binds human feedback, DAG progression, integration isolation, and terminal verification into one outcome rather than separate optional workflows.
- **Anchor**: 2026-07-25 · jobs turn
- **Status**: live

### D-4 — bet and proof: equivalent local and remote feature drives

- **Question**: what observable result proves the feature works, and must v1 demonstrate both local and remote ownership modes?
- **Answer**: prove both modes on a real six-task DAG with parallel roots and dependencies: local ownership keeps every task local; each task yields a human-reviewable feature-branch PR with feedback repair; blocker merges unlock only valid dependents; a seeded terminal-e2e failure yields a repair PR; only the passing integrated SHA reaches one final PR to main. Repeat remotely on an otherwise equivalent unclaimed feature.
- **Reasoning**: operator accepted the full recommended proof. Demonstrating both paths makes execution-location equivalence and exclusive ownership measured behavior, while the seeded failure proves the drive owns repair and re-verification rather than only the happy path.
- **Anchor**: 2026-07-25 · goals turn
- **Status**: live

### D-5 — v1 refusals: human merge authority and bounded orchestration

- **Question**: what must v1 explicitly refuse?
- **Answer**: no automatic task-PR merge; no task PR directly to main; no shared worker worktree; no redesign of e2e authoring; no generic standalone-issue orchestration; no automatic stale-local takeover; no automatic final-feature-PR merge.
- **Reasoning**: operator confirmed the recommended set. The feature coordinates human-reviewed integration and verification without silently expanding merge/failover authority or absorbing adjacent executor and e2e contracts.
- **Anchor**: 2026-07-25 · non-goals turn
- **Status**: live

### D-6 — appetite: one complete medium-neutral feature-drive slice

- **Question**: how large is v1?
- **Answer**: one repository and one feature per drive: resolve the frozen DAG/routing scope/remote feature branch; reserve local or remote ownership; sweep after reviewed merges; resume feedback on the same PR; execute terminal e2e at an exact SHA; create repair work until green; raise but never merge the final PR; prove both modes. Contracts stay medium-neutral with GitHub/Git first.
- **Reasoning**: operator confirmed the recommendation. The slice is large enough to prove the complete job and both execution locations, while deferring multi-repo coordination, automatic takeover/merge, and fleet-wide scheduling policy.
- **Anchor**: 2026-07-25 · appetite turn
- **Status**: live

### D-7 — ownership handoff: explicit, non-preemptive, checkpointed

- **Question**: what happens when one execution domain owns a feature and the human requests the other domain?
- **Answer**: never preempt automatically. Report the current owner and refuse concurrent start; an explicit handoff waits for every active task attempt to finish or checkpoint, transfers feature ownership, and only then permits future tasks and feedback repairs in the new domain. The rule is symmetric local↔remote.
- **Reasoning**: operator accepted the recommendation. Non-preemptive handoff preserves the same-machine guarantee and prevents split-brain execution while still allowing deliberate movement without discarding resumable work.
- **Anchor**: 2026-07-25 · ownership-handoff turn
- **Status**: live

### D-8 — kickoff routing: explicit local or remote assignment, never a race

- **Question**: is initial execution ownership explicitly selected, or does an unowned feature automatically enter the remote pool after a timeout?
- **Answer**: select it explicitly at kickoff: `implement-now <feature>` reserves and runs locally; the remote form assigns remote ownership. Until a choice is durably recorded, the feature remains unowned and no executor may claim its tasks.
- **Reasoning**: operator accepted the recommendation. Explicit assignment gives the human a real, auditable choice and avoids arbitrary grace periods where local and remote race to establish ownership.
- **Anchor**: 2026-07-25 · kickoff-routing turn
- **Status**: live

### D-9 — review feedback: same proposal, still blocking until verified landing

- **Question**: how does human review feedback affect task completion and DAG progression?
- **Answer**: open/request-changes/approved-but-unmerged all remain incomplete and block dependents. The current feature owner addresses feedback on the same task branch and PR; a new head invalidates stale evidence per repo policy. Only verified landing on the feature branch advances tracker state. Out-of-charter work becomes a new dependency-linked task.
- **Reasoning**: operator confirmed the recommendation. Review is part of completion, not an annotation after it; same-PR repair avoids duplicate proposals, and explicit scope expansion keeps the frozen DAG honest.
- **Anchor**: 2026-07-25 · review-feedback turn
- **Status**: live

### D-10 — e2e repair: new traceable work, full re-verification

- **Question**: how does a terminal e2e failure become implementation work?
- **Answer**: record the failure against the exact tested feature SHA; create a feature-scoped repair task carrying evidence and links to implicated original work; implement it through a new human-reviewed PR to the feature branch; preserve merged history; rerun the full e2e suite on the new SHA, invalidating prior evidence after every repair merge.
- **Reasoning**: operator accepted the recommendation. New repair work makes integrated defects explicit and auditable without rewriting historical PRs; full rerun proves the corrected deliverable rather than only the failing scenario.
- **Anchor**: 2026-07-25 · e2e-repair turn
- **Status**: live

### D-11 — verification target: exact candidate merge with current main

- **Question**: what must terminal e2e test when main may advance during feature implementation?
- **Answer**: test a disposable candidate merge of the current feature head with the current target-main head, without mutating either branch. Bind evidence to feature head + main head + candidate-merge SHA; either input changing makes evidence stale and forces a rerun before the final PR is eligible.
- **Reasoning**: operator accepted the recommendation. Testing the candidate merge catches concurrent-main integration failures while preserving clean branch history and making the exact verified deliverable reproducible.
- **Anchor**: 2026-07-25 · verification-target turn
- **Status**: live

### D-12 — e2e completion: every required route has SHA-bound accepted evidence

- **Question**: what happens when a required e2e scenario is manual-human, unavailable, skipped, or inconclusive?
- **Answer**: the final PR stays blocked until every required scenario has an accepted outcome: automated routes pass on the candidate merge; manual-human routes carry explicit human attestation bound to that candidate SHA; unavailable, skipped, and inconclusive routes remain blocking and produce a decision brief.
- **Reasoning**: operator accepted the recommendation. This consumes the existing run-for-real/mocked/manual-human contract without weakening “verified feature” by treating inability to run as green.
- **Anchor**: 2026-07-25 · e2e-completion turn
- **Status**: live

### D-13 — completion: verified delivery on main, not final-PR creation

- **Question**: when is the feature drive complete and ownership released?
- **Answer**: final-PR creation is not completion. Ownership persists through final review; feedback repairs the feature branch and reruns candidate-merge e2e. After human merge, verify the delivered revision is an ancestor of main, then mark delivered, release ownership/routing, and clean disposable state. Feature-branch deletion follows retention policy only after verification.
- **Reasoning**: operator accepted the recommendation. Mainline ancestry is the goal-object proof that the verified feature actually landed; retaining ownership until then prevents abandoned feedback and premature cleanup.
- **Anchor**: 2026-07-25 · completion turn
- **Status**: live

### D-14 — reporting: action surfaces plus minimal recovery diagnostics

- **Question**: does v1 need a unified feature status surface, and where should progress appear?
- **Answer**: no new dashboard. Surface task review/checks in task PRs, dependencies in the tracker, attended progress in the local command, and remote/action-required events through the existing notification mechanism. Provide only a minimal feature-status diagnostic for inspection and recovery; durable internal state remains required for correctness.
- **Reasoning**: operator accepted the revised recommendation after challenging the need. Information belongs where the human acts; a comprehensive status view has no established user problem and is deferred until evidence justifies it.
- **Anchor**: 2026-07-25 · reporting turn
- **Status**: live

### D-15 — review waits: durable machine affinity, no live process

- **Question**: must a local feature-drive process stay alive while waiting for human review?
- **Answer**: no. Checkpoint durable state, report awaiting-review, and exit cleanly; resume by re-invocation or a local wake after feedback/merge. Local ownership means every implementation attempt runs on the same registered machine until explicit handoff, not that one terminal process lives for the feature's duration.
- **Reasoning**: operator accepted the recommendation. Durable affinity preserves the locality guarantee while keeping human review waits at zero live-session cost and surviving process restarts.
- **Anchor**: 2026-07-25 · review-wait turn
- **Status**: live

### D-16 — execution locality versus e2e placement

- **Question**: does feature execution ownership also constrain where terminal e2e scenarios run?
- **Answer**: implementation affinity is strict: implementation and feedback-repair workers stay in the selected domain. E2e is separately routable: the owner coordinates it, declared scenarios may use specialized runners/CI, evidence identifies the runner and candidate SHA, and absent configuration it defaults to the owning machine/domain.
- **Reasoning**: operator confirmed the recommendation. This keeps “implement locally” honest while allowing e2e to require browsers, devices, credentials, or infrastructure the implementation machine cannot provide.
- **Anchor**: 2026-07-25 · e2e-placement turn
- **Status**: live

### D-17 — cancellation: stop and preserve, never destroy implicitly

- **Question**: what happens when the human cancels or abandons an active feature drive?
- **Answer**: cancellation is explicit, waits for active workers to finish/checkpoint, releases ownership, and removes executable routing. It preserves branches, commits, PRs, task history, and e2e evidence. Resume versus permanent abandonment is another explicit choice; destructive cleanup requires later human confirmation.
- **Reasoning**: operator accepted the recommendation. Cancellation must prevent continued execution without turning a stop request into accidental loss of recoverable work.
- **Anchor**: 2026-07-25 · cancellation turn
- **Status**: live

### D-18 — boundary: feature-aware drive, feature-blind executor

- **Question**: how much must the remote implementation engine know about features?
- **Answer**: nothing beyond an ordinary execution assignment. The drive/scheduler resolves feature identity into routing scope, target base, and owner; the executor receives those parameters and ready tasks. Workers remain claim-one-task, isolated-worktree, implement/test/propose units; lifecycle, review waits, e2e, and delivery stay in the drive.
- **Reasoning**: operator approved the recommendation. Keeping feature semantics above the worker preserves one reusable executor across local, remote, and future fleet deployments.
- **Anchor**: 2026-07-25 · executor-boundary turn
- **Status**: live

## Facts

- **F-1**: The governing pipeline separates per-task implementation/review (stages 8–9) from feature-level e2e verification (stage 10), and defines the factory's output as merged, verified software. — source: `docs/product-vision.md` §1, §4
- **F-2**: The current executor is one frontier sweep: open routing-tagged tasks are narrowed by blocker state and remote task claims, workers run concurrently, green proposals open, and the driver owns landing and re-invocation. — source: `internal/executor/ghtracker/tracker.go`; `internal/executor/dispatch/dispatch.go`; `cmd/implement-now/main.go`
- **F-3**: `implement-now` already accepts a configurable `--label` and `--base`; the base is wired both to fresh worktree creation and to the proposal target. — source: `cmd/implement-now/run.go`; `internal/executor/workspace/workspace.go`; `internal/executor/ghcodehost/codehost.go`
- **F-4**: Task claims are cross-machine git-ref compare-and-swap leases keyed by issue number, so overlapping executors cannot implement one issue twice; they do not reserve future tasks of a feature for one execution domain. — source: `internal/executor/lease/lease.go`; inspection of `port.Task` and dispatcher claim flow
- **F-5**: The frozen e2e brainstorm places e2e once per feature after all implementation tasks are built and reviewed; its current PRD explicitly defers building/running the tests through implement-now to a follow-on. — source: `brainstorm:e2e-verification#D-4/#D-14`; `docs/prds/e2e-verification/effective.md` WR-1
- **F-6**: The domain model already has the durable ingredients—Feature, WorkItem DAG, ChangeProposal, FeedbackRound, E2EVerification, kind-blind Node—but not their feature-drive orchestration or ownership state. — source: `docs/domain-model.md` §3.2–3.4
- **F-7**: GitHub task PRs merged into a non-default feature branch may not close their linked issues through default-branch closing semantics; the feature driver must verify landing and advance tracker state explicitly rather than assume a merged badge closes the blocker. — source: GitHub-medium behavior noted for design validation; exact adapter behavior remains to be verified in the technical stage

## Fog

none

## Out of scope

- **O-1**: Automatic merge of task PRs — every task proposal remains a human review-and-merge boundary. (from D-5)
- **O-2**: Task proposals targeting main — task integration occurs only on the feature branch; only the final verified feature proposal targets main. (from D-5)
- **O-3**: A literal shared worker worktree — workers remain isolated; committed feature-branch state is the integration surface. (from D-5)
- **O-4**: Redesigning the e2e authoring line — consume its reviewed design and ordered-plan contract. (from D-5)
- **O-5**: Orchestrating generic standalone issues — they remain in the ordinary executor pool. (from D-2, D-5)
- **O-6**: Automatic takeover of stale local ownership — recovery or remote handoff requires an explicit human act. (from D-5)
- **O-7**: Automatic merge of the final feature proposal — the human remains the acceptance and merge authority. (from D-5)
- **O-8**: Multi-repository features and cross-repository DAGs — v1 drives one feature in one repository. (from D-6)
- **O-9**: Sophisticated fleet-wide fairness and capacity scheduling — the remote binding may use existing execution policy; global optimization is separate fleet work. (from D-6)
- **O-10**: A unified feature dashboard — v1 uses native action surfaces plus a minimal recovery diagnostic. (from D-14)

## Candidate glossary terms

- **T-1**: **Feature integration drive** — the durable lifecycle that routes one frozen feature DAG through owned execution, human-reviewed task integration on a feature branch, candidate-merge e2e, repair, and verified delivery to main. Avoid: “implement-now run” (one executor sweep, not the lifecycle), “feature workspace” (only its durable substrate).
- **T-2**: **Feature execution ownership** — the exclusive, durable assignment of all implementation and feedback-repair attempts for one feature to a local registered machine or remote execution domain until explicit handoff/release. Avoid: “task claim” (locks one current issue only), “process lock” (ownership survives process exit).
- **T-3**: **Candidate merge** — a disposable, non-mutating combination of an exact feature head with an exact target-main head, whose resulting identity is the subject of terminal e2e evidence. Avoid: “feature SHA” (omits the mainline input), “merge preview” (does not imply evidence binding).

## Parked technical constraints

- **P-1**: A locally initiated feature run needs a durable feature-level reservation above current task claims if it must guarantee that all future-unblocked tasks execute on that machine; exact ref/schema, heartbeat, expiry, and handoff mechanics belong to technical design.
- **P-2**: The current merge lane assumes `main`; intra-feature PR admission must either remain human-driven or gain a separately designed target-aware lane. The final feature→main PR can use the existing mainline lane.
- **P-3**: E2e evidence must bind to an exact feature-branch SHA and become stale on any head change; where the runner executes is adapter/policy, not feature semantics.
- **P-4**: Candidate-merge construction must be deterministic and non-mutating, and its evidence schema must carry both input heads plus the resulting tree/commit identity; exact ref mechanics belong to technical design.

## Gate

| Field | Value |
|---|---|
| **Verdict** | ready-for-prd |
| **Date** | 2026-07-25 |
| **Operator** | @mdarabi |
| **Covering source** | — |
| **Coverage check** | pass — all six inputs covered (problem: D-1; segments: D-2; jobs: D-3; goals: D-4; non-goals: O-1..O-10; appetite: D-6), zero blocking fog |
