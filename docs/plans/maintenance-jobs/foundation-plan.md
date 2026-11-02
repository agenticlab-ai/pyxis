# Maintenance job foundation — retained-ticket plan

Status: foundation scope and dependency roadmap for direct issue execution.
Tracker snapshot: 2026-10-14. Owner and approver: @mdarabi.

This plan implements the operator's narrowed delivery: Go CRUD over the jobs
YAML, core CLI installation/execution, and monitoring/logging. It uses exactly
the 15 retained implementation issues under [epic #1472](https://github.com/agenticlab-ai/pyxis/issues/1472):
16 GitHub issues when the epic is included. Two implementation issues are
completed; thirteen remain open. Existing issue numbers are retained. No new
implementation issues are proposed.

The merged `001-plan.md` and its generated JSON remain historical. Their
30-task scope, dependency graph, and native task capsules do not authorize this
delivery. This document is the reviewable foundation replacement, not an
`NNN-plan.md` source that the executor can mistake for a frozen plan. The operator authorized the shipped direct-issue workflow for this scope on
2026-10-13. Each issue's exact admitted contract owns execution; this roadmap
is not an input to the full-plan watcher.

## Delivery boundary

Included behavior:

- Typed, closed-schema CRUD over `.pyxis/maintenance/jobs.yaml` in the existing
  root Go module, preserving unrelated jobs and repository independence.
- `pyxis maintenance install`, `run`, `status`, and `uninstall` with an explicit
  canonical absolute repository root; one cron entry per enabled job, directly
  calling the installed CLI with literal repository path and job ID.
- Existing Claude, Codex, Cursor, and OpenCode adapters and qualified semantic
  routing. Published job policy chooses the harness and routing requirements;
  the existing harness catalog owns native model and effort selection.
- Published policy read without changing the enrolled checkout, date plus job
  ID coordination scoped to its configured remote, and an independent private
  execution checkout for each winning attempt.
- Configured execution deadlines, cancellation through the existing runtime,
  structured local lifecycle/outcome logging, and read-only status inspection.
- The local audit stream at `.pyxis/maintenance/logs.jsonl`, with detailed
  diagnostics under ignored `.pyxis/logs/maintenance/`. Shared attempt outcomes
  remain in the coordination ref; credentials stay out of both streams.

Excluded from every task: a specific CI job or prompt, CI timing/evidence
adapters, CI optimization, supervisor PR publication, legacy runner migration
or host adoption, new shared sandbox/credential protocols, report/export
pipelines, a janitor, notifications, automatic merges, and crash recovery,
retry, takeover, catch-up, or non-daily scheduling. Optional delivery metadata
already supported by the jobs schema remains passive data. The runner must
refuse an enabled unsupported delivery mode before claiming or launching;
it must not silently ignore a requested delivery behavior.

## Existing issues and corrected dependencies

T1–T15 are navigation labels in this roadmap, not native plan task IDs. They do
not change IDs in frozen plan 001. Issue numbers remain the execution identity.
Dependency edges below become active after exact tracker read-back. Completed dependencies are satisfied
by their merged implementation; closed-as-not-planned tickets are never
treated as implemented prerequisites.

| Task | Existing issue | Foundation behavior | State at snapshot | Blocked by | Added production logic ceiling |
|---|---|---|---|---|---|
| T1 | [#1503](https://github.com/agenticlab-ai/pyxis/issues/1503) | YAML job CRUD | Completed, PR #1512 merged | — | Existing implementation |
| T2 | [#1515](https://github.com/agenticlab-ai/pyxis/issues/1515) | CLI admission and desired-policy inspection | Completed, PR #1517 merged | T1 | Existing implementation |
| T3 | [#1534](https://github.com/agenticlab-ai/pyxis/issues/1534) | Configured construction through the existing harness registry | Open | T2 | 300 |
| T4 | [#1476](https://github.com/agenticlab-ai/pyxis/issues/1476) | Correlated append-only JSONL audit writer | Open | T2 | 300 |
| T5 | [#1477](https://github.com/agenticlab-ai/pyxis/issues/1477) | Deterministic owned cron entry reconciliation | Open | T2 | 290 |
| T6 | [#1478](https://github.com/agenticlab-ai/pyxis/issues/1478) | Native cron and timezone admission | Open | T5 | 260 |
| T7 | [#1479](https://github.com/agenticlab-ai/pyxis/issues/1479) | Installed manifest and generation proof | Open | T4 | 280 |
| T8 | [#1480](https://github.com/agenticlab-ai/pyxis/issues/1480) | Idempotent install/uninstall transaction | Open | T4, T5, T6, T7 | 300 |
| T9 | [#1481](https://github.com/agenticlab-ai/pyxis/issues/1481) | Published policy and immutable daily identity | Open | T3, T4, T8 | 300 |
| T10 | [#1482](https://github.com/agenticlab-ai/pyxis/issues/1482) | One shared job/date attempt | Open | T9 | 300 |
| T11 | [#1491](https://github.com/agenticlab-ai/pyxis/issues/1491) | Private independent execution checkout | Open | T10 | 300 |
| T12 | [#1492](https://github.com/agenticlab-ai/pyxis/issues/1492) | One bounded native harness execution | Open | T3, T8, T11 | 300 |
| T13 | [#1495](https://github.com/agenticlab-ai/pyxis/issues/1495) | Truthful outcomes and audit failure handling | Open | T8, T10, T12 | 300 |
| T14 | [#1496](https://github.com/agenticlab-ai/pyxis/issues/1496) | Read-only drift, journal, and outcome status | Open | T8, T13 | 300 |
| T15 | [#1501](https://github.com/agenticlab-ai/pyxis/issues/1501) | Foundation integration proof and operator runbook | Open | T14 | 280 |

In particular, T11 no longer depends on #1487, #1488, or #1490; T12 no
longer depends on #1488; T13 no longer depends on #1493 or #1494; and T15
no longer depends on #1497, #1498, or #1500. Those exclusions remove new
credential projection, broad preflight/confinement expansion, CI evidence,
report/publication pipelines, janitor, CI job, and migration work. Existing
runtime authentication, routing, cancellation, and confinement still apply.
Unsupported existing capabilities refuse execution; they are not expanded by
these replacement tasks.

## Task charters

Each task delivers the stated behavior and its own focused verification. The
global delivery boundary applies in addition to each task's exclusions.

### T1 — Preserve typed YAML job CRUD (#1503, completed)

**Goal:** Create, read/list, update, and delete jobs without losing unrelated
definitions or crossing repository boundaries.

**Baseline and result:** PR #1512 supplies the closed versioned jobs codec and
root-bound store, using the already pinned YAML library, named errors, serialized
mutations, and atomic replacement. This task records a satisfied prerequisite;
it does not dispatch the storage implementation again.

**Acceptance:** Existing CRUD, malformed-policy, symlink, cancellation, and
concurrent-write coverage remains applicable when later tasks consume the API.
No second jobs parser or model is introduced.

**Interfaces:** `jobs.Document`, `jobs.Job`, `jobs.Store`, and its named error
contract. **Reuse:** `go.yaml.in/yaml/v3` and the merged storage implementation.

### T2 — Preserve CLI admission and desired-policy inspection (#1515, completed)

**Goal:** One installed CLI admits the maintenance grammar and inspects an
explicit repository's desired policy independently of ambient working directory.

**Baseline and result:** PR #1517 supplies additive command registration,
canonical repository admission, validated operands, and read-only desired status.
Install/run/uninstall currently return named unavailable refusals. T8, T12,
and T14 replace those refusals only for their respective completed behaviors.

**Acceptance:** Hostile-cwd/Git-environment and symlink-alias fixtures preserve
repository identity; existing verbs, sorted help, and argument refusals remain
unchanged. This completed task is not dispatched again.

**Interfaces:** Core command descriptors, maintenance argument admission,
`policy.Status`, and `jobs.Store.Load`. **Reuse:** `cliguard` and merged jobs APIs.

### T3 — Resolve supplied harness profiles (#1534)

**Goal:** Construct a registered harness exclusively from an explicit captured
typed catalog while preserving legacy registry behavior.

**Scope:** Use one registry-owned registration and derived harness set. Register
configured constructors in the four existing adapter packages through their
`WithProfiles` options. Reuse catalog validation and native routing. Existing
saved candidate work is not completion evidence until its required checks and
independent review pass.

**Acceptance:** All four live adapters use supplied profiles despite conflicting
ambient configuration; a separately registered fixture requires no central
harness switch edit. Missing profiles, unknown harnesses, constructor errors,
nil results, invalid catalogs, and unsupported requests refuse without ambient
fallback or launch. Role separation, cloning, and legacy resolution remain
covered.

**Interfaces:** `Register`, `Resolve`, configured resolution, `WithProfiles`,
`CatalogProvider`, and `nativerouting.Resolver`. **Excluded:** Actual job execution
or changes to native launch protocols and routing policy.

### T4 — Append a correlated local audit journal (#1476)

**Goal:** Append independently decodable, credential-free lifecycle records to
the canonical enrolled repository's ignored maintenance journal.

**Scope:** A closed `pyxis-maintenance-log/v1` writer uses shared `slog`/trace
conventions, per-repository process serialization, restrictive file ownership,
and consequential sync. Unknown job/date fields remain absent on early refusal.
Bound records to 32 KiB and summaries to 4 KiB with explicit truncation. Preserve
an incomplete historical tail and append a boundary/diagnostic before new events.

**Acceptance:** Concurrent writers yield complete records; credential canaries
and authenticated URL values are absent. Tracked, non-ignored, symlinked,
unwritable, and unsyncable targets refuse mutation admission. Tail and partial
write fixtures preserve historical bytes and report uncertainty.

**Interfaces:** Audit admission, append contract, invocation/trace identity,
and typed lifecycle event. **Reuse:** Existing lock, redaction, and trace patterns.
**Excluded:** Raw output aggregation, expiration, artifact export, or job recovery.

### T5 — Reconcile owned direct cron entries (#1477)

**Goal:** Produce exactly one deterministic direct CLI invocation per enabled
job while preserving all unowned crontab bytes.

**Scope:** Pure rendering/reconciliation handles additions, updates, disabled
or removed jobs, and an empty job list. Complete repository/job ownership pairs
are required; duplicate or malformed owned records refuse. Render the stable
installed binary, literal canonical root/job ID, command-local tool path, and
early-error redirection with argument quoting and separate cron percent escaping.

**Acceptance:** Golden no-op/install/update/removal fixtures preserve unrelated
and other-repository entries byte-for-byte. Spaces, single quotes, and percent
signs round-trip; newline, NUL, unstable executable bindings, and malformed
ownership yield named defects without a writable replacement.

**Interfaces:** Owned `CronEntry`, installed executable/tool binding, and pure
reconciliation result. **Excluded:** Native cron writes, probing, or global
environment changes. Cron's command interpretation requires no interactive shell.

### T6 — Admit compatible native cron semantics (#1478)

**Goal:** Accept installation only when the native scheduler supports the
configured daily timezone and effective entry environment.

**Scope:** A consumer-owned context-aware cron port reads/installs user tables
through supported Darwin/Linux provider semantics. Distinguish an absent table
from permission/provider failure. Interpret retained timezone, POSIX shell,
startup, and delay directives in entry order; leave global directives unchanged.

**Acceptance:** Cancellation bounds provider calls. A preceding conflicting
timezone directive refuses even when host timezone matches. Unsupported shells,
startup injection, random-delay settings, mismatched timezone, and unknown
semantics refuse before writes. DST dispatch remains native cron's responsibility.

**Interfaces:** Cron port, provider profile, ordered effective environment, and
admission verdict. **Excluded:** Foreign-timezone scheduling, wake services,
catch-up, or a Pyxis scheduler daemon.

### T7 — Prove installed configuration through a manifest (#1479)

**Goal:** Expose one ready installation generation whose source and schedule
binding can be verified before a daily claim.

**Scope:** Closed machine-local manifest data contains canonical repository
identity, source binding, derived executable/tool locations, job schedule
fingerprints, and ready/installing/unavailable state. Use atomic restricted
publication and the existing locking discipline. Keep prompts, catalogs, and
credentials in their owning stores.

**Acceptance:** Concurrent mutation cannot interleave generations; cancellation
releases owned locks. Malformed/unknown manifest fields refuse. Interrupted
publication cannot expose partial ready state. A proof identifies its generation
and detects replacement; absent/intermediate state names installation as remedy.

**Interfaces:** `InstallationManifest`, generation, transaction lock, and ready
binding proof. **Excluded:** Cron writes, policy fetch, retry queues, or job recovery.

### T8 — Commit idempotent installation and removal (#1480)

**Goal:** `install` and `uninstall` update only the selected repository's owned
cron/manifest pair, with zero cron writes for an unchanged installation.

**Scope:** Compose audit admission, rendering, native compatibility, and manifest
generation under the installation lock. Preview is read-only. Bound rechecks of
intervening table changes and provider calls; retain restricted prior-state
backups, verify installation, and publish ready only after verification. On a
write failure restore a verified prior pair or report unavailable state.

**Acceptance:** Identical reinstall writes zero times. Disabled/removed/empty
jobs reconcile; uninstall works without policy, preserving unrelated entries
and audit history. Faults at write/verify/concurrent-change boundaries never
report success or allow a queued invocation to claim intermediate authority.

**Interfaces:** Install/uninstall/preview, cron port, manifest transaction, and
installation audit result. **Excluded:** Legacy migration and adoption of arbitrary
entries. No guarantee is made against an undetectable external human write race.

### T9 — Read one published policy and daily identity (#1481)

**Goal:** Capture one published configuration revision and one job/date identity
without changing enrolled source or Git storage.

**Scope:** Reuse runner-owned Git/control storage and the installed source binding.
Read jobs and the existing harness catalog at one exact fetched commit, without
local overlays or fallback. Capture start time before blocking IO; derive the
configured-timezone date once and retain it across midnight. Reprove the ready
generation and installed schedule; validate the supported routing/dependency
boundary before claiming. Disabled/removed jobs skip; unsupported delivery
metadata refuses before claim.

**Acceptance:** Enrolled files, branch, index, refs, configuration, and FETCH_HEAD
remain unchanged after reads and failed fetches. Published prompt/timeout/route/
disable changes apply; unpublished policy cannot supply instructions. Source or
schedule drift refuses. Manual/delayed same-date starts share a key; DST and
midnight fixtures retain the captured date. Replacement generation requires a
bounded fresh read.

**Interfaces:** Published policy snapshot, invocation identity, ready binding,
captured catalog, and existing native resolver. **Excluded:** New authentication
or confinement mechanisms, minute eligibility, claims, or execution cloning.

### T10 — Coordinate one retained attempt across machines (#1482)

**Goal:** At most one positively proved owner may attempt a job on a configured-
timezone date; failure never makes that date eligible again.

**Scope:** Wrap existing `gitrefstore` CAS/transport in the maintenance claim
consumer port. Use remote/job ID/date identity, a random owner nonce, and a
closed compact record. Keep config/harness/host/hour out of the key. Hold ready
generation proof through exact winning-record confirmation; ambiguous transport
does not authorize launch. Permit only same-owner terminal updates.

**Acceptance:** Independent clients using a temporary bare remote elect one
owner and losers skip. Absent-ref leases, contention, ambiguous pushes, malformed
state, and identity mismatches launch no unproved owner. Config or harness edits,
failure, timeout, and interrupted startup consume the same date; a next-date
attempt remains eligible. Failed terminal updates retain the claim.

**Interfaces:** `ClaimStore`, immutable daily identity, ownership proof, and
existing `gitrefstore` snapshot/CAS result. **Excluded:** TTL, claim deletion,
takeover, retry, code publication, or a second Git CAS implementation.

### T11 — Prepare an independent private execution checkout (#1491)

**Goal:** A winning attempt edits its own source and Git metadata, preserving
the enrolled checkout and other attempts.

**Scope:** After positive claim proof, prepare a private independent clone at the
captured source revision with a per-attempt branch/runtime location. Check exact
ownership and non-overlap with enrolled/common/control Git storage and other
attempts. Reuse existing workspace/Git and runtime confinement APIs; clone config
does not embed transport credentials. Retain failed or uncertain work for
inspection rather than introducing automatic disposal.

**Acceptance:** Each fixture has distinct source, Git metadata, and runtime
state; enrolled and other-run snapshots remain unchanged. Overlap, unsafe
ownership, or credential-bearing local settings refuses launch and retains the
claimed date. Existing confinement constrains writes to the admitted workspace;
unsupported existing enforcement yields a named refusal.

**Interfaces:** Owned workspace, captured revision, execution branch, runtime
state, and existing process confinement. **Excluded:** Linked worktrees sharing
enrolled Git metadata, PR continuation, supervisor branch publication, new
credential isolation mechanisms, or automatic janitor work.

### T12 — Run one configured harness within its deadline (#1492)

**Goal:** `maintenance run` launches the selected existing harness once in the
proved private checkout and bounds that attempt through cancellation.

**Scope:** Wire admission, policy/routing/authentication checks, ready proof,
claim, workspace preparation, and existing runtime launch methods in order.
Create an implementation-purpose `LaunchSpec` with the literal prompt, frozen
native selection, private work/runtime roots, and nil session. The configured
job timeout covers all claimed setup and execution; cleanup has a separate bound.
Use existing runtime results and termination proof without a new report protocol.

**Acceptance:** Four-adapter fake-native fixtures receive the supplied prompt
and exact selected model/effort through existing protocol. Deadline/cancellation
fixtures cover preparation and descendants. Parent exit without proved process
tree quiescence remains incomplete and retains the workspace. Every path after
claim launches at most once, retains the date, and invokes no retry/fallback.

**Interfaces:** `AgentRuntime.AuthCheck/Launch/Await/Kill`, `LaunchSpec`, existing
brief assembly and launch-fidelity predicate. **Excluded:** Synthetic tracker
tasks, issue leases, interactive shell startup, PR delivery, code-host evidence,
new report/export pipelines, or new process launchers.

### T13 — Record truthful outcomes and logging failures (#1495)

**Goal:** Local and shared observations distinguish executed, skipped, refused,
failed, timed-out, and incomplete attempts without manufacturing success.

**Scope:** Join observed runtime result, claim, phase/cause, timing, exit status,
and retained workspace references. Succeeded execution requires an available,
complete runtime result and proved termination; it does not claim the prompt's
business objective or successful publication. Refusal before claim and ordinary
skip differ from a consumed failed attempt. Mid-run audit failure cancels further
work, uses bounded termination, and attempts a same-owner shared failure result.

**Acceptance:** Success/failure/timeout/skip/refusal/incomplete fixtures produce
distinct correlated results; exit zero alone cannot erase an incomplete result.
Pre-mutation audit failure refuses effects. Mid-run audit failure does not return
CLI success or release the date. Shared terminal-write failure is diagnosed
locally and cannot authorize another launch; absent observations remain absent.

**Interfaces:** Existing `RunResult`, typed terminal outcome, audit append, and
same-owner claim transition. **Excluded:** Report verification, PR facts, CI
target claims, terminal-state repair, recovery, or retry.

### T14 — Inspect drift and results without repair (#1496)

**Goal:** `maintenance status` explains desired versus installed state and known
execution outcomes while performing zero mutations.

**Scope:** Extend merged desired-policy inspection with owned cron entries,
manifest/source/schedule drift, journal health, shared attempt results, and
available retained-work references. Reuse atomic local-state/locking patterns
for a closed refusal-counter record updated by the installation/execution
paths, once per invocation and cause. Keep lifetime/consecutive counts and
first/last timestamps; status only reads them. Expose unavailable, malformed,
and missing observations explicitly.

**Acceptance:** Status fixtures perform zero file, cron, remote, or agent writes.
Absent/drifting installation, no attempt, success, failure, timeout, and incomplete
attempts differ. Missing evidence and malformed journal records remain visible.
Ordinary disabled/removed/lost-claim skips do not increment malfunction counts.
Validated coordination resets consecutive count while retaining lifetime
totals. Counter-write failure is audited and does not grant launch; reading
status never creates or repairs counter state.

**Interfaces:** Maintenance status, manifest/cron comparison, journal reader,
shared outcome reader, and closed refusal-counter state. **Excluded:** A general
metrics service, notifications, tail repair during status, retries, or resetting
attempted dates.

### T15 — Prove and document the foundation (#1501)

**Goal:** One compiled core binary demonstrates storage, installation, bounded
execution, and monitoring across two fixture repositories.

**Scope:** Reuse the preceding package/conformance suites in one hermetic
install/run/status/uninstall flow. Inject clock, cron, fake harness processes,
and temporary bare remotes. Cover supported Darwin/Linux arm64/amd64 builds
and core upgrade/convergence ownership. Document jobs CRUD API, CLI use,
published policy, daily deduplication, timezone limits, audit lookup, disable,
uninstall, retained work, and unsupported-delivery refusal.

**Acceptance:** One binary handles two fixture repositories without
cross-repository mutation; a generic test-only job launches through each
existing adapter and records its outcome. No-op reinstall, competing claims,
timeout/cancellation, drift, audit failure, and policy-independent uninstall
pass. Upgrade/convergence preserves jobs and journal; release payload omits
local logs. Applicable repository checks pass under current AGENTS.md without
the race detector. The runbook accurately states excluded capabilities.

**Interfaces:** Installed maintenance CLI, jobs API, existing harness conformance,
core ownership/release contracts, and operator runbook. **Excluded:** Live model
inference, actual host cron activation, real CI jobs, migration, rubric evaluation,
or describing inherited CI/adoption DoD rows as completed.

## Execution limits and reuse

- One selected ready issue at a time. Existing `parallel_width: 1` remains in
  effect. A dependent unit waits for its prerequisite to merge to main; no
  stacked implementation train is authorized by this proposal.
- Each implementation dispatch has a 60-minute worker bound and its table's
  production logic ceiling. Count added function logic excluding blank/comment/
  standalone-brace lines and type/import/data declarations; report total diff
  size separately. If the unit cannot fit, stop with a concrete scope proposal
  instead of implementing adjacent issues or opening more tickets.
- These worker bounds are distinct from a maintenance job's YAML timeout.
  Job runs use their validated configured deadline. No 90-minute default is
  introduced by this plan, and a CI runtime target is not a job deadline.
- Use the existing root Go module and installed core binary. Reuse the pinned
  YAML library, `cliguard`, jobs store, harness registry, `harnessconfig`, native
  resolver, `AgentRuntime`, `gitrefstore`, tracing/redaction, and file/workspace
  patterns. No new module, shell executable, scheduler daemon, model roster,
  routing ladder, CI client, or general-purpose framework is introduced.
- A new dependency requires a concrete necessity and review; none is planned.
  Unit fixtures are hermetic, with injected time and bounded IO. Required checks
  and configured independent review must pass before an implementation PR;
  exact-head human approval and merge remain separate.
- The recovered #1534 candidate is rebased onto current main and running its
  existing exact admitted contract. No repair cap was increased. A local
  candidate or open PR does not count as merged.

## Direct issue execution and preserved history

The operator narrowed delivery on 2026-10-06 and authorized the shipped direct
issue workflow on 2026-10-13, following the recommendation to complete these
existing issues without another feature-planning cycle. Epic #1472 retains the
foundation scope. The executor's `--inspect-issue` and `--issue` path owns exact
source admission, isolated work, bounded repair, publication, and independent
review. Required checks and human merge remain in force.

The original `001-plan.md`, its generated JSON, and
`lock/maintenance-jobs/v1` remain unchanged historical inputs. This roadmap
does not claim coverage of their excluded CI/PR requirements. A native plan
replacement would still require a correctly scoped locked contract and
fold-generated Markdown/JSON pair; that migration is not a prerequisite for
the authorized direct-issue route.

The exact old issue bodies, agent-owned capsules, and dependency edges are
retained in [the readmission record](issue-readmission-20261014.json). That
record also holds the bounded replacement source for each retained issue.
Preserve this record in shared Git before replacing an executable carrier.

For each remaining issue:

1. Verify the live body, old capsule author/content, and dependencies still
   match the captured source. Stop on another author's change.
2. Retire only the agent-owned old executable capsule, retaining a link to its
   exact Git archive. Replace the issue body with its foundation-only charter,
   remove the old plan-task marker, and install the corrected blocker edges.
   Keep the same issue number and exclude the issue from broad routing.
3. Read back the body, capsule retirement, and native blocker list. Inspect the
   exact issue with the shipped executor. Create its ignored input only when
   all actual implementation blockers are merged and closed.
4. Run that issue with the existing ambient/configured routing, repair bounds,
   local gate, and independent review. Reuse an existing admitted input exactly
   on recovery. The executor retains the immutable contract and outcomes in
   its shared Git state; do not hand-author a replacement native capsule.
5. Publish verified work for human review and merge. A dependent issue begins
   from main after its blockers land. Completed #1503/#1515 remain closed;
   excluded or superseded work is never treated as implemented.

Retain epic #1472 and link #1515/#1534 if absent. No new implementation issue,
full-plan watcher, old-plan re-projection, or plan JSON edit is part of this
execution route.
