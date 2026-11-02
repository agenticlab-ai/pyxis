# Review: prototype tech spec — design-review report

| Field | Value |
|---|---|
| **In one line** | Request changes: the direct launch omits required runtime preparation, uses a registry catalog that excludes local overlays, and does not preserve the runtime's quiescence condition for committing. |
| **Relevant-when** | Auditing the technical review of the revised prototype contract or closing its runtime integration findings. |
| **Subject** | design:prototype · shape: none · supersedes: — |
| **Moment** | draft |
| **Reviewer** | tech-design-review · repository guide at fed23f47431e65bad545a82ebfc9e57ed82ae79a |
| **Date(s)** | 2026-10-10 |
| **Verdict** | request-changes |
| **Tracking** | https://github.com/agenticlab-ai/pyxis/pull/1547 |
| **Template version** | design-review-report/v1 |

**Review subject:** tech-spec
**Draft digest:** sha256:1a30cdc6d10fc60060ec11f0209355e4d56a74b83bfb9905c2fab51e9dfd46b8

## Round 1

Reviewed the whole tech spec from `fed23f47431e65bad545a82ebfc9e57ed82ae79a`, using the PRD at that commit as its requirement basis and excluding the PRD's human-only routing section. Read both brainstorm records and P-1 at the same commit. This is round 1 of a new episode; reports 001–004 remain untouched. Compared against `e18f4a0728d81855b329e5ff1ca12865d7b256ea`, concentrating on route resolution and the new launched build.

Ran contract-quality, coherence, upstream-alignment, and feasibility-completeness sequentially in this reviewer session, then attacked every candidate. The three findings below survived. The operator's direction and the brainstorm decisions stand; these findings concern the concrete layout's compatibility with the existing dependency contracts. None asks for implement-now, a spend meter, requirement-based routing, another file class, or a new readiness gate. Compiler, citation, and lint-floor checks are excluded from findings.

Checked source claims against the code at the reviewed head: draft capture and brainstorm anchor resolution, draft worktree creation, the closed configuration parsers and overlay loader, harness catalog loading, all four harness adapters, the shared brief assembler, process confinement, authentication preparation, and the launch/await port. `pyxis draft digest prototype` returned the exact head and digest above. The local branch is `draft/prototype`; the status command's remote lock-tag lookup could not complete because GitHub DNS resolution was unavailable.

### FND-1 — The direct launch lacks the runtime state and login preparation its adapters require

- **Severity**: blocking (CAL-4)
- **Lens**: feasibility-completeness
- **Evidence**: design: D-5, Launch, says the verb “builds the harness's runtime from the registry” and “calls its launch with the prototype worktree as the working directory” and the resolved model and effort; its Security and privacy section promises “the resolved harness's own login” and existing implementation confinement. D-2 performs only the adapter's authentication check. At the reviewed head, `internal/executor/runtime/access.go`, `resolveAccessPaths`, requires an existing, absolute `RuntimeStateDir` separate from the worktree; it has no default for that field. `internal/executor/runtime/environment.go`, `LaunchEnvironment`, redirects HOME and the harness login directories into isolated runtime state rather than inheriting the logged-in user's directories. The implementation host supplies both prerequisites through `PrepareTaskState` before `Launch` (`internal/executor/worker/worker.go`); that helper prepares directories and snapshots the selected login (`internal/executor/runtime/taskstate.go` and `authentication.go`). Neither registry construction nor `AuthCheck` performs that preparation. The proposed direct call therefore lacks a required confinement input, and merely adding an empty state directory still loses a file-based login that passed the host preflight. This is load-bearing for FR-6/DOD-8 and the claimed confinement, rather than an additional product capability (AP-14, AP-20).
- **Acceptance condition**: Specify the verb's preparation and ownership of isolated runtime state, including the existing selected-harness authentication preparation, its assignment to `LaunchSpec.RuntimeStateDir`, and its release with the prototype or after the run. Reuse the existing helpers without entering the worker cycle. Extend verification to exercise that host preparation and the real shared launch-access/environment seams with a fixture file-based login and no ambient API token; a stub implementing only `AgentRuntime.Launch` does not prove those prerequisites.
- **Disposition**: applied — D-5 gains a Runtime state passage: the verb prepares the isolated state and login snapshot with the runtime's existing task-state preparation, passes it as the launch's runtime state directory, and owns it; D-1's remove deletes it. D-5's verification adds a case through the shared launch-access and launch-environment functions with a fixture file-based login and no API token.

### FND-2 — The registry-built runtime does not receive the effective catalog used to resolve the route

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: design: D-2 makes the prototype block “legal in the tracked config and in the machine-local overlay” and resolves its profile from the harness's `harness-config` catalog. D-5 then constructs the runtime “from the registry”. At the reviewed head, each built-in registry factory calls `New()` without `WithProfiles`; `harnessconfig.ResolveProfiles` consequently calls `LoadDefault`, which reads only the tracked `.pyxis/config.yaml`. By contrast, `harnessconfig.Load` applies the machine-local overlay. The implementation composition root explicitly supplies merged profiles through each adapter's existing `WithProfiles` option (`cmd/implement-now/resolver.go`, `buildBackendForPurpose`); registry resolution alone does not do that. A valid Cursor implementation model introduced by the local catalog can resolve in D-2 but fail in D-5: `internal/adapters/harness/cursor/routing.go`, `decodeLaunchCapture`/`effortForModel`, recognizes models only from the adapter's stored profiles. This contradicts the two decisions' shared route and the FR-6 promise to launch the configured model.
- **Acceptance condition**: Specify one effective catalog snapshot for both route resolution and runtime construction/launch validation, including the local overlay, and how it reaches the existing adapter configuration seam. Keep prototype-specific behavior in the shared host/runtime layers. Add a hermetic verification case with a valid model/profile present only in the local catalog and exercise the actual adapter catalog or launch-decoding seam, demonstrating that resolution and launch use the same model and effort.
- **Disposition**: applied — D-2 names one effective catalog snapshot, tracked config merged with the local overlay, loaded once; D-5 constructs the runtime through the adapter's own constructor with that snapshot, by moving the existing constructor function to a shared package. Verification adds a profile present only in the local overlay, checked against the real adapter's catalog.

### FND-3 — Committing partial work must remain conditional on process-tree quiescence

- **Severity**: blocking (CAL-4)
- **Lens**: feasibility-completeness
- **Evidence**: design: D-5 says “After the process exits, the verb stages and commits everything in the worktree”; its failure passage covers launch errors, abnormal exits, and time-outs while retaining “the committed partial work”. The reused port explicitly distinguishes an error that acknowledges termination from `ErrNotQuiescent`: “Hosts must preserve the workspace without checkpointing or publishing it” (`internal/executor/port/agentruntime.go`). This distinction is observable in the current adapter, not hypothetical: Codex `Await` can return `ErrNotQuiescent` after its direct child exits when descendant termination cannot be acknowledged (`internal/adapters/harness/codex/codex.go`). A launch can also return that sentinel from failed process-start observation (`codex/exec.go`). A direct-child exit or a requested kill therefore does not authorize the promised commit. The new host must preserve the existing port's handoff condition to avoid checkpointing a tree with possible live writers (AP-20; FR-2 and UB-1).
- **Acceptance condition**: Narrow D-5's commit and failed-run promises to paths where the runtime acknowledges that the process tree has stopped. On `ErrNotQuiescent`, including a launch failure or failed termination, preserve the worktree without staging, committing, or removing it and report the cause. Add a stub-runtime case returning that sentinel and verify that no Git checkpoint or teardown occurs. No retry or new recovery mechanism is required.
- **Disposition**: applied — D-5 commits only where the runtime acknowledges the process tree stopped; on a not-quiescent report from launch or await it stages, commits, and removes nothing and reports the cause. Verification adds that stub case. No retry or recovery mechanism is added.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-10-10 |

## Escalation brief

none

## Measurements

- **Rounds**: 1
- **Findings**: 3 blocking · 0 advisory
- **Dispositions**: 3 applied · 0 declined · 0 withheld-as-violation · 0 pending

## Overlay corrections

none
