# Review: prototype tech spec — design-review report

| Field | Value |
|---|---|
| **In one line** | Approve: the revision closes FND-1 through FND-3 and introduces no surviving regression within the changed spans. |
| **Relevant-when** | Auditing closure of the prototype runtime integration findings at the round-2 contract digest. |
| **Subject** | design:prototype · shape: none · supersedes: — |
| **Moment** | draft |
| **Reviewer** | tech-design-review · repository guide at 709082562dedd718e57001195f5856b4eb621065 |
| **Date(s)** | 2026-10-10 |
| **Verdict** | approve |
| **Tracking** | https://github.com/agenticlab-ai/pyxis/pull/1547 |
| **Template version** | design-review-report/v1 |

**Review subject:** tech-spec
**Draft digest:** sha256:81a8b3621e83dd810cbe31b2c15171f6b8ecc3ea13346b25953920acb4e64dcb

## Round 1

[006.md](006.md) records `request-changes` for committed head `fed23f47431e65bad545a82ebfc9e57ed82ae79a`, with three blocking findings: FND-1 (runtime state and authentication preparation), FND-2 (effective catalog propagation), and FND-3 (process-tree quiescence before checkpointing). All three author dispositions are recorded as applied. The original five-element finding blocks and their calibration citations remain in that report, unchanged.

## Round 2

Reviewed only those findings' acceptance conditions and regressions within the computed delta from `fed23f47431e65bad545a82ebfc9e57ed82ae79a` to `709082562dedd718e57001195f5856b4eb621065`. The PRD blob is identical at both heads; this pass did not reopen unchanged requirements or read, cite, or judge PRD section 0. Applied contract-quality, coherence, upstream-alignment, and feasibility-completeness to the permitted scope, then challenged possible findings against the revised text and the actual dependency contracts. No new finding survived, and no violation-withheld candidate arose.

The following closure judgments rely on the committed revisions and source code at `709082562dedd718e57001195f5856b4eb621065`, not on the author's disposition assertions.

### Closure of FND-1

**Acceptance condition met.** D-5's new Runtime state passage requires preparation through the existing task-state helper before launch, a snapshot of the selected harness's login, and assignment of the prepared directory as the launch's runtime state directory. It assigns ownership to the verb; changed D-1 Remove deletes that state with the prototype. The revised Security and privacy passage accurately describes the snapshot and isolation.

Source check: `internal/executor/runtime/taskstate.go`, `PrepareTaskState`, creates a private root beside the worktree through `TaskStateRoot`, prepares its harness child, and calls `PrepareReviewAuthentication` for the built-in harnesses. `authentication.go` copies the selected login into that state. `access.go`, `resolveAccessPaths`, requires a separate absolute state directory; `environment.go`, `LaunchEnvironment`, redirects HOME and the adapter's login directories there. `internal/executor/worker/worker.go` already assigns the helper's returned directory to `LaunchSpec.RuntimeStateDir`. These are the preparation and launch seams the revision now names, and they are reusable without entering the worker cycle.

D-5 Verification now goes beyond a Launch-only stub: it requires preparing a fixture file-based login with no ambient API token and exercising the real shared launch-access and launch-environment functions to establish that both the directory and snapshot reach launch. This satisfies the finding's dependency-contract verification condition and AP-14.

### Closure of FND-2

**Acceptance condition met.** Changed D-2 requires one effective catalog snapshot, loaded once with the tracked configuration and machine-local overlay merged, for both route resolution and D-5 launch. Changed D-5 replaces bare registry construction with the existing composition-root constructor supplied that same snapshot; the constructor moves to a shared internal package with behavior preserved. Prototype-specific behavior remains in the shared host/runtime layers.

Source check: `internal/harnessconfig/config.go`, `Load`, merges the local overlay, whereas `LoadDefault` reads the tracked catalog alone. `cmd/implement-now/resolver.go`, `buildBackendForPurpose`, passes the supplied harness profiles through each built-in adapter's `WithProfiles` option. Thus sharing that existing construction function supplies the catalog that bare registry construction omitted. `internal/adapters/harness/cursor/routing.go`, `Catalog`, `ValidateProfile`, and `decodeLaunchCapture`, consult the adapter's stored profiles, providing a real seam for verifying an overlay-only model and effort.

D-2 adds an overlay-only profile case, and D-5 Verification requires that route resolution and the real adapter's catalog check agree on its model and effort. The revised refusal names inability to construct a runtime rather than merely lack of registry registration. These changed passages are consistent with the construction route and close the catalog mismatch without prototype-specific adapter code.

### Closure of FND-3

**Acceptance condition met.** D-5 Commit now authorizes checkpointing only after the runtime acknowledges that the process tree stopped, including an acknowledged kill. Its revised Failure passage expressly covers unproven termination from launch or await: “the verb stages nothing, commits nothing, and removes nothing”, preserving the worktree, branch, and runtime state and reporting the cause. The promise of committed partial work is conditional on acknowledged termination.

Source check: `internal/executor/port/agentruntime.go`, `ErrNotQuiescent` and `AgentRuntime.Await`, distinguish possible live writers from returns acknowledging termination. `internal/adapters/harness/codex/codex.go`, `Await`, can return the sentinel after a direct-child exit or failed cancellation kill; `codex/exec.go` can return it during failed launch observation. The revised condition preserves that port contract and AP-20 rather than treating child exit or a kill request as proof.

D-5 Verification now requires an acknowledged-kill timeout case and a not-quiescent runtime case with no staging, commit, or removal. The revised failure rule covers both launch and await, so the sentinel cannot authorize either checkpointing or teardown. No retry or recovery mechanism is required for closure.

### Changed-span regression assessment

The new purpose-handling passage identifies both additional shared-runtime changes required by the existing code: `access.go` currently rejects an unknown confined purpose, and `memory.go`, `AssembleBriefContext`, currently bypasses memory assembly for review rather than prototype. D-5 now explicitly admits prototype to implementation confinement and excludes its memory pack. All four built-in adapters invoke the shared brief assembler and select their review invocation by the review purpose; the proposed shared changes support the stated implementation-style prototype invocation without adding prototype-specific adapter logic. The revised AP-1 and AP-20 claims are consistent with the port boundary and conditional checkpointing.

This is approval of the revised contract and its specified verification, not a claim that the proposed feature or tests have already been implemented or executed. The two-round cap is reached with convergence. Subject and code checks used committed Git objects; no Git state was changed.

## Verdict history

| Round | Report | Verdict | Date |
|---|---|---|---|
| 1 | [006.md](006.md) | request-changes | 2026-10-10 |
| 2 | [008.md](008.md) | approve | 2026-10-10 |

## Escalation brief

none

## Measurements

- **Rounds**: 2; this episode is complete.
- **Findings**: round 2: 0 blocking · 0 advisory; episode: 3 blocking · 0 advisory, all three closed in round 2.
- **Dispositions**: episode: 3 applied, independently verified here · 0 declined · 0 withheld-as-violation · 0 pending.

## Overlay corrections

none
