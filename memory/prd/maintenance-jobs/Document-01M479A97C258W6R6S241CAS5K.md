# Maintenance jobs — PRD

| Field | Value |
|---|---|
| **In one line** | Install daily repository-defined cron jobs through the core Pyxis CLI, reuse its harness routing, and coordinate one attempt per job/date across machines. |
| **Relevant-when** | Planning, building, or reviewing maintenance job installation, unattended agent launches, shared scheduled-run claims, or daily CI improvement proposals. |
| **Status** | Draft — problem confirmed by operator; contract review and lock pending |
| **Tier** | T3 — unattended authenticated execution, host scheduling, shared coordination state, and code-host publication |
| **Owner** | @mdarabi |
| **Approver(s)** | @mdarabi, via the draft PR's exact-head operator review |
| **Contributors** | @mdarabi, product decisions; Codex, contract authoring |
| **Informed** | Operators installing the same repository on additional machines |
| **Date** | 2026-10-05 created · 2026-10-05 last updated |
| **Appetite** | One contract and bounded implementation of the installer, single-job executor, shared claims, and first CI job; one operator contract review and implementation review. Excludes a scheduler daemon, retry service, and new model orchestration framework. |
| **Target release** | First core release satisfying the blocking Definition of Done |
| **Tracking** | https://github.com/agenticlab-ai/pyxis/pull/1443 |
| **Artifacts** | Operator decisions attributed in §7.3; review evidence in this feature draft |
| **Template version** | prd-template/v2 |
| **Design coverage** | requirements-first/v1 |

## 0. Read to rule — the human reviewer's section (~4 min)

Informative human routing only; the contract body governs. Exact-head approval
of this revised proposal remains pending.

### Judge these

| Call | Why it matters | Body anchor |
|---|---|---|
| Daily-only v1; one retained job/date attempt in the configured timezone | Manual runs consume the date too; failure/crash has no retry/takeover | §4.4, FR-6, FR-7, WR-1, WR-5 |
| Reuse Claude/Codex/Cursor/OpenCode and existing semantic routing | Jobs choose a harness and requirements; qualified catalogs own native settings | §4.2, FR-2, FR-10, FR-15 |
| Isolate policy/claim Git storage and every execution clone | Enrolled source and Git state remain outside the job's mutation boundary | FR-8, FR-9, NR-2 |
| Go owns verification and publication; humans approve and merge | Agent edits/tests/reports, one dedicated draft PR, no force-push | FR-12, UB-6, NR-3 |
| Local JSONL audit stream at the requested maintenance path | Pre-claim refusals and outcomes are inspectable; detailed evidence stays local | FR-11, §5.4, EVT-1–EVT-4 |

### Numbers and limits

- Initial job: daily 02:00 America/Los_Angeles; required CI target ≤10 minutes.
- Proposed maintenance deadline: 90 minutes, distinct from the CI target.
- Fifteen Must functional requirements; seven blocking DoD rows; three job evals.
- Zero duplicate attempts for a job/date, autonomous merges, or weakened checks.
- Native timezone compatibility is required; no Pyxis catch-up/distribution daemon.

### Open questions and informed skips

No solution-blocking questions remain (§7.2). Recovery, schedule distribution,
non-daily scheduling, and foreign-timezone support stay deferred (§3.4).
Schema/readiness checks backstop structure; reviews assess substance; harness,
coordination, confinement, journal, smoke, and rollback evidence backstop delivery.

## 1. Summary — the one-pager

The operator repeatedly asks an agent to measure Pyxis CI and improve its
runtime toward ten minutes. Repeating the request and maintaining a separate
machine-local setup make the work difficult to reproduce on another machine.
Independent installations can also attempt the same daily work twice.

Repository-defined jobs will be installed and executed by the core Pyxis CLI.
Each enabled daily job has its own cron entry. Its configuration selects an
existing harness and semantic routing requirements; Pyxis resolves the native
model and effort through its existing routing layer. A shared job/date claim
selects one machine per day in the job's timezone. The winner works in a private
checkout, and the Go supervisor creates or continues one draft PR for human
review. Failures consume the same date; the next date remains eligible. Local
installation and execution events are retained in
`.pyxis/maintenance/logs.jsonl`, including failures before claiming.

Success at adoption means unchanged installations perform zero crontab writes,
competing machines produce at most one launch for an occurrence, and a real
headless smoke run produces inspectable evidence. The ten-minute CI objective is
the job's ongoing target, not a promise that this scheduling feature achieves it
on its first run. The main risks are duplicated publication, unsafe unattended
access, and schedule or process failures that silently stop maintenance.

## 2. Problem space

### 2.1 Problem statement

The operator has recurring CI performance work but must repeat the request and
its execution setup. Settings held only on one machine are difficult to review
or reproduce. When several machines perform the same recurring work, duplicate
attempts waste agent work and can produce conflicting proposals.

### 2.2 Users and segments

| Segment | Role and context | Domain exp. | Tech exp. | Key goals | Pain today | Evidence |
|---|---|---|---|---|---|---|
| P1 (primary) | Repository operator assigning recurring CI work | journeyman | journeyman | Review scheduled behavior and approve useful proposals | Repeated prompting and machine-local configuration | Operator request, §7.3 |
| P2 | Operator installing the same repository on another host | journeyman | journeyman | Reproduce the installation without duplicate attempts | No shared attempted-occurrence record | Operator cross-machine requirement, §7.3 |
| Anti-segment | Users requiring automatic failover, catch-up, or exactly-once completion | — | — | Guaranteed completion after host failure | Those guarantees exceed the chosen v1 scope | WR-1, WR-3 |

### 2.3 Jobs and desired outcomes

| # | When… (situation) | I want to… (motivation) | So I can… (outcome) | Today's workaround | Evidence |
|---|---|---|---|---|---|
| J1 | Recurring maintenance settings change | Review and install the desired jobs | Control their behavior without repeating setup | A machine-local runner and configuration | Operator request |
| J2 | Several hosts are available at the scheduled time | Select one attempt for that occurrence | Avoid duplicate work and conflicting proposals | Local-only overlap protection | Operator Git-ref coordination ruling |
| J3 | CI needs investigation or a previous proposal needs attention | Obtain evidence and one reviewable proposal | Improve runtime while preserving required validation | Repeated interactive prompting | Operator initial CI request |
| J4 | An unattended attempt fails | Inspect the failure and continue at the next occurrence | Avoid an invisible permanent stall | Inspect local runner logs | Operator no-recovery ruling |

### 2.4 Evidence

| Claim | Source (research, data, tickets, interviews) | Confidence |
|---|---|---|
| CI performance investigation is recurring operator work | Direct operator statement: repeatedly prompting an AI agent to inspect and improve the Pyxis CI pipeline | High |
| Schedule and publication authority are settled | Operator selected 02:00 daily in America/Los_Angeles and one draft PR with human approval and merge | High |
| The installation must serve multiple repositories and machines | Operator requested the installed core CLI and shared Git-ref idempotency | High |
| A ten-minute target is achievable for the current pipeline | No proof established by this contract; the job must measure and report instead of assuming success | Low; owned by R3 |

### 2.5 The journey today

| Phase | User does | User feels / pain | Frontstage (what user sees) | Backstage (systems, people, processes) |
|---|---|---|---|---|
| Request | Repeats a CI investigation prompt | Repeated attention | Agent session | Local agent CLI and code-host credentials |
| Install | Configures one host | Setup is difficult to reproduce | Local config and cron entry | Host scheduler and standalone runner |
| Verify | Reads a report or proposal | Must establish what actually ran | Logs and draft PR | CI timestamps and validation results |
| Extend | Adds another host | Duplicate attempts become possible | Independent installations | No shared schedule-occurrence authority |

### 2.6 Opportunity and why now

The operator already requested daily unattended work and a shared repository
configuration. This capability replaces repeated setup with a reviewed job
definition and reproducible installation. Doing nothing preserves local-only
configuration and coordination; its cost is repeated operator attention and
potential duplicate agent work, not an invented monetary estimate.

**Decision:** Problem basis confirmed by the operator's direct request and
follow-up decisions; final contract approval remains the exact-head draft gate.

| Reviewer | Role | Date | Verdict |
|---|---|---|---|
| @mdarabi | Operator | 2026-10-05 | Problem confirmed in this conversation; review duration was not captured |

## 3. Goals, metrics, and guardrails

### 3.1 Goals as falsifiable hypotheses

| # | We believe that… | Will result in… | Measured by… |
|---|---|---|---|
| G1 | Reviewing and reconciling repository job definitions | Reproducible installation without configuration drift | M1 |
| G2 | Sharing attempted-occurrence authority | At most one agent attempt for the same scheduled work | M2 |
| G3 | Supplying measured CI work and bounded execution | Inspectable proposals and failures | M3 |

### 3.2 Goals → signals → metrics

| Metric | Goal | Signal | Baseline | Target (at checkpoint) | Feeds from |
|---|---|---|---|---|---|
| M1 | G1 | Reinstallation matches the desired entries without writes | Machine-local setup | Zero writes on unchanged installs and exact owned-entry reconciliation at adoption | EVT-1 |
| M2 (primary) | G2 | Launch count for competing hosts on one occurrence | No distributed deduplication | At most one launch in every coordination fixture at adoption | EVT-2 |
| M3 | G3 | Attempt has an inspectable outcome | Existing local-run evidence | Every ordinary success, failure, timeout, and skip fixture is diagnosable; headless smoke evidence retained at adoption | EVT-3 |

### 3.3 Guardrails and decision rule

| Guardrail metric | Threshold | Action if breached |
|---|---|---|
| Duplicate launches for an occurrence | Zero | Disable affected job and correct coordination before reinstalling |
| Changes to unrelated cron entries or enrolled source/Git state | Zero under quiescent external cron writers; only prescribed ignored journal/artifact paths receive local diagnostics | Restore backed-up cron state or disable the job; retain evidence |
| Autonomous approval, merge, or weaker required CI checks | Zero | Stop publication and return the proposal to human review |

**Ship rule:** All blocking DoD rows and guardrails pass before machine migration.
CI reaching ten minutes is measured by the maintenance job after adoption and
does not replace the scheduling capability's correctness gate.

### 3.4 Non-goals and waiting room

| ID | Non-goal | Why refused |
|---|---|---|
| NG-1 | Automated approval, merge, or repository protection changes | The operator reserves approval and merge |
| NG-2 | A shell implementation, nested Go module, or separate maintenance binary | The operator selected the installed Go core CLI |
| NG-3 | Guaranteed completion or retries of an attempted occurrence | V1 chooses one attempt and logged failures |
| NG-4 | Paid runner upgrades or weakened CI checks to meet the target | Performance work must preserve required validation and spending authority |

| ID | Waiting room | Revisit when / intended version |
|---|---|---|
| WR-1 | Crash takeover, automatic retry, and claim recovery | Operator requests recovery guarantees |
| WR-2 | Background distribution of changed cron schedules to every host | Manual per-host reconciliation becomes insufficient |
| WR-3 | Catch-up for sleep, downtime, and nonexistent DST times | Operator requests missed-occurrence handling |
| WR-4 | Scheduling a timezone different from the host's native cron timezone | A supported cross-timezone installation is requested |
| WR-5 | Multiple scheduled occurrences per job per day, weekly/monthly schedules, and minute-level occurrence identity | The operator requests schedules beyond daily-only v1 |

## 4. Solution space

### 4.1 Approach and alternatives considered

One repository job definition controls each job's prompt, agent settings,
schedule, and publication policy. Installation reconciles one native cron entry
per job. At execution, the published definition and shared attempted-occurrence
record decide whether a machine may proceed.

| Option | Sketch | Why not chosen |
|---|---|---|
| Chosen | Core CLI, per-job cron entries, and shared Git-ref claims | Operator selected this shape |
| Dispatcher | One frequent cron tick scans all jobs | Operator explicitly selected one entry per job |
| Local module | Repository-local Go maintenance executable | Operator selected the installed core binary for multiple repositories |
| Local-only claim | Prevent overlap on one machine | Does not satisfy cross-machine idempotency |
| Do nothing | Keep machine-local configuration | Does not provide centrally reviewed definitions |

### 4.2 Scope perimeter

In scope: reviewed YAML definitions; once-per-day schedules; explicit
installation and removal; single-job execution through the existing Claude,
Codex, Cursor, and OpenCode adapters and native routing; shared job/date claims
and ordinary terminal outcomes; isolated policy/coordination storage and private
execution clones; the local JSONL audit journal and retained diagnostics; the
initial daily CI job. Future work is listed in the waiting room. Notification
delivery and a new routing or harness-launch framework are outside v1.

### 4.3 Experience: flows and states

| Flow | User does | System responds | States covered | Coverage |
|---|---|---|---|---|
| F1 — install, J1 | Selects a repository and optionally previews reconciliation | Shows desired entries, then installs only changed owned entries | Empty list removes owned entries; invalid/missing file refuses; installation failure retains recovery evidence | FR-1–FR-5, UB-1, UB-5, NR-1 |
| F2 — execute, J2/J3 | Cron or a manual invocation calls one literal repository path and job ID | Validates published policy and route, derives the job's local date, claims it, and runs bounded isolated work | Disabled/removed jobs skip; already-attempted dates skip; schedule drift and dependency failure refuse launch | FR-6, FR-8–FR-13, FR-15, UB-2, UB-4, UB-6, NR-2, NR-3 |
| F3 — inspect or stop, J4 | Reads status/logs, disables a job, or uninstalls | Names the outcome and stops future eligible launches | Ordinary failure retains its attempt; hard crash may leave only a started record; next occurrence stays eligible | FR-7, FR-11, FR-14, UB-3, NR-1 |

The interface is a text CLI. Messages name the repository/job, outcome, and
actionable cause. There is no web surface or localization promise in v1;
timestamps include their timezone, and IDs are stable machine-readable tokens.

### 4.4 Scenarios

Two machines fire for the daily job. One wins the shared job/date claim; the
other logs a skip. The winner's selected harness investigates CI in its own
private checkout; the Go supervisor verifies and creates or continues the
dedicated branch's draft PR. A failed agent process retains evidence and consumes
that date; the following date is eligible. A merged prompt or routing edit is
read at the next invocation. A schedule edit additionally requires reinstalling
the affected hosts' cron entries; a published/installed mismatch refuses a claim.
A manual run at a different hour uses the same date and consumes that day's
attempt if it wins. Every invocation retains correlated events in the local
JSONL journal; cron, rather than a second minute-eligibility gate, controls timing.

| Reviewer | Role | Date | Verdict |
|---|---|---|---|
| @mdarabi | Operator | Pending | Exact-head solution/contract approval through this draft PR |

## 5. System requirements

### 5.1 Functional requirements

Priority is Must throughout; implementation cuts require a contract ruling.
The safety plan for authenticated and scheduling surfaces is §6.5.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-1 | When an operator selects a local Pyxis repository, the installed core CLI shall operate on that repository's maintenance jobs independently of the CLI's source checkout. | Multiple local repositories | Must | high | One installed binary operates on two fixture repositories without cross-repository writes | Test | G1, J1, F1 |
| FR-2 | The maintenance CLI shall obtain shared maintenance policy and prompts from `.pyxis/maintenance/jobs.yaml`. | One reviewed maintenance configuration surface | Must | high | Each schedule, prompt, harness selection, semantic routing request, timeout, and delivery setting comes from the selected file; native model catalogs remain owned by the existing harness configuration | Test | G1, J1, F1 |
| FR-3 | When jobs are installed, the maintenance CLI shall create one native cron entry per enabled job that directly calls the core CLI with a literal job ID and repository path. | Operator installation choice | Must | high | Golden crontabs contain exactly the enabled jobs' direct invocations | Test | G1, J1, F1 |
| FR-4 | When desired installation values change, the maintenance CLI shall reconcile only the selected repository's owned cron entries. | Preserve other work | Must | high | Added, updated, disabled, and removed jobs reconcile; unrelated and other-repository entries remain byte-identical under quiescent external writers | Test | G1, J1, F1 |
| FR-5 | When installed entries equal desired entries, the maintenance CLI shall leave crontab unwritten. | Idempotent installation | Must | med | Repeated identical install calls produce zero provider writes | Test | G1, J1, F1 |
| FR-6 | When machines contend for a daily job, the executor shall authorize at most one agent attempt per job ID and date through shared Git-ref coordination scoped to the configured repository remote. | Daily cross-machine idempotency | Must | critical | Starts at different hours on the same configured-timezone date contend for one key; timezone/date-boundary and competing-client fixtures launch at most one agent; unproved ownership launches zero | Test | G2, J2, F2 |
| FR-7 | After a job/date is claimed, the executor shall retain its attempted status while leaving later dates eligible. | Chosen no-recovery scope | Must | high | Success, failure, timeout, interrupted-start, and manual-run fixtures suppress that date and permit the next date; config or harness edits do not reset its key | Test | G2, J4, F3 |
| FR-8 | When cron or a manual invocation selects a job, the executor shall use that job's validated published policy from the configured remote branch without mutating the enrolled checkout's Git storage. | Central publication controls isolated execution | Must | high | Published prompt/disable/routing updates apply; unmerged policy supplies zero instructions; fetch and coordination fixtures preserve enrolled refs, FETCH_HEAD, index, source/untracked work, and configuration; only prescribed ignored diagnostics may be appended/exported there | Test | G1, J1, F2 |
| FR-9 | When a claimed job performs repository work, the executor shall provide that attempt a private independent checkout for edits, tests, and publication. | Preserve active and concurrent work | Must | high | Every winning attempt receives separate work and Git metadata; fixtures preserve enrolled source/Git state and other attempts' checkouts, allow only prescribed ignored diagnostic writes, and constrain the agent to its own workdir | Test | G3, J3, F2 |
| FR-10 | When a job launches its agent, the executor shall use the resolved native route through the existing unattended harness runtime and enforce the configured execution deadline. | Bounded configured execution | Must | high | Harness conformance verifies the selected model/effort and literal prompt; native settings remain adapter-owned; all four adapters terminate their process tree on cancellation or deadline | Test | G3, J3, F2 |
| FR-11 | When installation or job execution produces a lifecycle event or outcome, the CLI shall append inspectable structured evidence to the enrolled repository's `.pyxis/maintenance/logs.jsonl`. | Debugging and audit require one local stream | Must | high | Success/failure/timeout/skip/pre-claim-refusal fixtures produce versioned correlated records with available identity, config/route, phase/cause, timings, delivery and artifact references; concurrent writes do not interleave records; the journal is Git-ignored and credential-free | Test | G3, J4, F1, F3 |
| FR-12 | Where draft-PR delivery is configured, the Go supervisor shall verify and create or continue one draft PR from the attempt's private checkout on the configured dedicated branch for human approval and merge. | Existing harness protocol and operator publication authority | Must | critical | Agent Git writes/publication are denied; quiescent verified changes publish with normal hooks; existing-PR, moved-head and merged-branch fixtures obey branch policy; approval/merge stay human | Test + Demo | G3, J3, F2 |
| FR-13 | Where the initial Pyxis CI job is installed, the job shall run daily at 02:00 America/Los_Angeles with a prompt targeting required end-to-end CI of at most ten minutes while preserving required checks. | Original recurring work | Must | critical | Config inspection confirms cadence and target; EVAL-1 and EVAL-2 pass | Inspection + Eval | G3, J3, F2 |
| FR-14 | When an operator inspects or uninstalls maintenance jobs, the CLI shall report installation/outcome state or remove only the selected repository's managed entries. | Control and rollback | Must | high | Status performs zero writes; uninstall preserves unrelated and other-repository entries | Test | G1, J4, F3 |
| FR-15 | When a daily job selects a supported harness and semantic routing requirements, the executor shall resolve a qualified native route through Pyxis's existing routing and harness adapters. | Reuse all four supported harnesses | Must | high | Claude, Codex, Cursor, and OpenCode fixtures resolve from their published catalogs and launch through AgentRuntime; unsupported routes/authentication refuse before claiming; recorded native selection matches actual launch | Test | G3, J3, F2 |

| ID | Existing behavior protected | Why protected | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|
| NR-1 | Unrelated cron jobs and other repositories' installations | Host belongs to several workflows | high | Golden reconciliation and uninstall preserve their bytes | Test | F1, F3 |
| NR-2 | Enrolled source/untracked work/Git state, normal Git hooks, and existing Pyxis verbs | Maintenance shares the installation; local journal/artifact writes are owned diagnostic exceptions | high | Isolation/hook/CLI suites show zero regressions outside prescribed ignored diagnostics; job definitions survive core convergence and upgrades | Test | F2 |
| NR-3 | Required CI checks and human approval/merge | Runtime improvement cannot weaken the factory | critical | EVAL-1 and EVAL-2 reject proposals violating these constraints | Eval | F2 |

### 5.2 Unwanted behavior and failure handling

| ID | If… (trigger) | Then the system shall… | User sees | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|---|
| UB-1 | If configuration or CLI input is invalid | Refuse before installation or agent launch | Named field/input defect | Must | high | Unknown fields, duplicate IDs, schedules outside once-per-day v1, invalid refs/routing, and unsupported schemas cause zero cron/Git/agent mutations; local refusal diagnostics are allowed | Test | F1, FR-2 |
| UB-2 | If remote policy, authentication, or claim ownership cannot be established | Refuse agent execution | Named dependency or coordination failure | Must | critical | Transport failure and ambiguous CAS fixtures launch zero agents | Test | F2, FR-6 |
| UB-3 | If an ordinary agent attempt fails or times out | Retain partial evidence and record the outcome without retrying that occurrence | Failure/timeout and next eligible occurrence | Must | high | Failure injection preserves the claim, available patch, and terminal result; later occurrence is eligible | Test | F3, FR-7 |
| UB-4 | If native timezone support or the installed/published schedule binding is absent | Refuse installation or execution with the cause | Unsupported host timezone or schedule-drift/reinstall reason | Must | high | Mismatched/unknown timezone refuses install; changed published cron/timezone refuses before claiming; same-date delayed/manual launches use the same daily key without a minute/grace gate | Test | F1, F2, FR-3 |
| UB-5 | If cron reconciliation fails or detects concurrent change | Preserve backup evidence and report the failure without reporting successful installation | Recovery location and failure | Must | high | Provider failure/concurrent Pyxis installer fixtures preserve unrelated entries and identify recovery evidence | Test | F1, FR-4 |
| UB-6 | If the dedicated code branch cannot advance without rewriting incompatible history | Preserve a local proposal and report the blocker | Branch/PR requiring human attention | Must | high | Diverged branch fixtures perform zero force-pushes and retain available work | Test | F2, FR-12 |

### 5.3 Permissions and abuse cases

| Function / data | Operator | Installed job process | Unauthenticated |
|---|---|---|---|
| Publish job definitions | Through normal repository review | No grant to bypass policy review | None |
| Install cron entries | Explicit invocation | No implicit installation during job work | None |
| Claim occurrence and publish dedicated draft PR | Configured remote credentials | Scoped to configured job policy | None |
| Approve or merge | Existing human authority | No authority | None |

| ID | Actor and intent | Vector | Impact | Severity | Mitigated by | Verify |
|---|---|---|---|---|---|---|
| ABUSE-1 | Misconfigured job executes unintended command text | YAML or cron quoting | Unintended authenticated execution | high | UB-1, FR-3, FR-10 | Invalid-input and metacharacter fixtures |
| ABUSE-2 | Competing host repeats expensive work | Missing or lost shared claim | Duplicate proposals | high | FR-6, UB-2 | Concurrent and ambiguous-publication fixtures |
| ABUSE-3 | Agent expands its authority | Prompt asks to weaken checks, approve, or merge | Factory-control compromise | critical | FR-12, FR-13, NR-3 | EVAL-1, EVAL-2 and publication inspection |

### 5.4 Data

| Entity / field | Meaning | Source of truth | Retention | Integrity rules | Privacy class |
|---|---|---|---|---|---|
| Job definition | Shared desired behavior | Published jobs YAML | Repository history | Closed schema and reviewed publication | Repository visibility |
| Installation binding | Host binary/root/tool locations and owned entries | Managed crontab | Until reconciliation/uninstall | Repository-scoped ownership | Local |
| Occurrence claim/outcome | Attempt authority and compact result | Shared Git-ref state | Retained in v1 | Atomic claim, immutable ownership, no automatic deletion | Repository visibility |
| Local audit journal | Installation/execution lifecycle and outcomes | `.pyxis/maintenance/logs.jsonl` in each enrolled checkout | Append-only, operator-managed; Git-ignored | Versioned records, serialized writers, correlated invocations, no credentials | Local, potentially sensitive |
| Diagnostics and partial work | Detailed execution evidence | Ignored repository-local producer logs; local runtime clone/cache | Operator-managed local retention | No credential serialization by the runner; references from the journal | Local, potentially sensitive |

Migration replaces the existing standalone job with one core-CLI entry only
after verification, using a retained crontab backup and one replacement. The old
and new uncoordinated executors do not run in parallel during cutover. Rollback
removes new entries before restoring the old entry. Claims remain interpretable
even if the new binary is uninstalled. No new profiling or sensitive-data
collection is introduced; a DPIA is not triggered by the stated scope.

Egress: the selected harness/provider receives repository/prompt content under
the operator's existing provider account; Git receives compact coordination
metadata and proposed code; the code host receives the draft PR. No notification
service is added. Provider retention follows that account's terms, not a new
Pyxis retention promise. Local invocation evidence lets the operator inspect
the prompt and published revision without storing authentication secrets.

### 5.5 Interfaces and integrations

| Interface | Direction | Purpose | Contract (format, ranges, timing) | Failure modes | Owner |
|---|---|---|---|---|---|
| Native user cron | Both | Per-job host scheduling | One direct entry per enabled job; host timezone support checked | Missing scheduler, permission, concurrent edit | Host operator |
| Git remote | Both | Published policy and occurrence coordination | Shared configured namespace and positive atomic claim | Network/auth failure, contention, incompatible state | Repository operator |
| Existing harness runtime and native routing | Both | Bounded unattended work | Claude/Codex/Cursor/OpenCode; typed semantic routing to qualified native model/effort; deadline distinct from CI target | Unsupported route, authentication, nonzero exit, timeout, unproved quiescence | Host operator |
| Code host | Both | CI evidence and one draft proposal | Normal reviewed branch publication | Missing permissions, existing PR, divergent history | Repository operator |

### 5.6 Non-functional requirements

| ID | Area | Requirement | Fit criterion | Verify |
|---|---|---|---|---|
| NFR-1 | Maintainability | The maintenance capability shall ship as Go within the core Pyxis module. | No new logic-bearing shell artifact, nested module, or separate maintenance executable | Inspection |
| NFR-2 | Platforms | The maintenance capability shall support the core's Darwin and Linux architectures where compatible native cron is available. | Build and adapter fixtures pass for Darwin/Linux on arm64/amd64; unsupported scheduling is explicitly refused | Test |
| NFR-3 | Privacy | The runner shall exclude authentication values from its serialized configuration and invocation diagnostics. | Injected credential values are absent from runner-owned artifacts | Test |
| NFR-4 | Operability | The maintenance CLI shall distinguish executed, skipped, failed, timed-out, and incomplete claimed occurrences. | Status/outcome fixtures distinguish all states without treating incomplete claims as successes | Test |

### 5.7 Instrumentation — the tracking plan

| ID | Event name | Fires when | Properties (typed) | Source | Feeds |
|---|---|---|---|---|---|
| EVT-1 | maintenance_installation_result | Reconciliation finishes/refuses | repo: string, changed: bool, owned_entries: int, outcome: enum | Installer | M1 |
| EVT-2 | maintenance_claim_result | Daily claim is won, lost, or unproved | job_id: string, job_date: date, timezone: IANA string, invocation_id: string, owner: string, outcome: enum, config_revision: string | Executor | M2 |
| EVT-3 | maintenance_run_result | Invocation refuses/skips or attempt finishes | job_id: optional string, job_date: optional date, invocation_id: string, outcome: enum, phase: enum, cause: optional string, elapsed_ms: int, evidence_refs: string list | Executor | M3 |
| EVT-4 | maintenance_lifecycle | Configuration, routing, launch, publication, or cleanup advances/refuses | invocation_id: string, phase: enum, job_id/date: optional identity, config_revision: optional string, harness/native_route: optional typed selection, ownership/delivery/artifact_refs: optional typed evidence | Installer/executor | M3 |

### 5.8 Definition of Done

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-1 | All Must functional and unwanted-behavior rows satisfy their fit criteria | §5.1, §5.2 | Test + Demo | Applicable test results and headless smoke artifact | yes |
| DOD-2 | Existing cron, checkout, approval, and CI obligations remain protected | NR-1, NR-2, NR-3, §3.3 | Test + Eval | Regression results and guardrail inspection | yes |
| DOD-3 | Language, platform, privacy, and diagnostic requirements pass | NFR-1, NFR-2, NFR-3, NFR-4 | Test + Inspection | Build/quality evidence and artifact inspection | yes |
| DOD-4 | First job's prompt behaves within its measurement and publication policy | FR-13, EVAL-1, EVAL-2, EVAL-3 | Eval | Authored positive/negative prompt cases and reviewed reports | yes |
| DOD-5 | Host cutover and rollback preserve unrelated cron work | §5.4, §6.5, FR-14 | Test + Demo | Backup, before/after entries, idempotent reinstall, rollback fixture | yes |
| DOD-6 | Operator can inspect and control the installation | FR-11, FR-14, §6.6 | Demo | CLI usage/runbook and status demonstration | yes |
| DOD-7 | Every supported harness reuses qualified routing and isolated supervised execution | FR-8, FR-9, FR-10, FR-12, FR-15 | Test + Inspection | Four-adapter conformance, actual-selection fidelity, Git isolation, supervisor-publication and unsupported-route evidence | yes |

### 5.9 AI behavior and evals

These evals verify the CI job prompt and report policy, not a promise that every
model output is correct. Human review remains the publication backstop.

| ID | Behavior | Eval set / harness | Threshold | Negative examples |
|---|---|---|---|---|
| EVAL-1 | Preserve CI validation and human publication authority | Representative cached-build, test-balance, weakened-check, paid-runner, and auto-merge proposal cases | Zero policy-violating cases accepted by the named report rubric | Skip tests, lower coverage, hide failure, approve/merge, change paid tier |
| EVAL-2 | Measure the required CI gate honestly | Complete/incomplete required-check evidence, failed/cancelled runs, mixed commit/event samples, and local-only estimates | Every case labels evidence limits and rejects unverified target claims | Faster failed run counted as success; unrelated workflow added; local timing called CI proof |
| EVAL-3 | Continue one useful concern or make no artificial change | Pending draft PR, approval-only PR, already-fast pipeline, blocked measurement | Every case follows branch/PR policy and states no-change or blocker where warranted | Duplicate PR, unrelated concern, gratuitous daily edit |

The operator is the rubric owner. Acceptance checks the report's evidence,
preserved controls, publication state, and explicit limitations, with each
representative case recorded as pass/fail. An implementation may automate the
rubric without granting automated merge authority.

## 6. Delivery

### 6.1 Phasing and milestones

| Phase | Purpose | Scope | Exit criterion |
|---|---|---|---|
| Contract | Agree observable guarantees | This draft's PRD/spec and reviews | Operator approves and locks the exact prepared head |
| Verification | Prove installation and coordination before adoption | Hermetic fixtures and isolated smoke | Blocking DoD evidence passes |
| Adoption | Replace this machine's existing entry | Explicit host cutover | One effective entry, idempotent reinstall, inspectable outcome |

### 6.2 Dependencies

| Dependency | Type (team / system / vendor / approval) | Needed by | Status | Owner |
|---|---|---|---|---|
| Exact-head contract lock | Approval | Implementation | Pending | @mdarabi |
| Native cron and configured timezone | System | Installation | Verify per host | Host operator |
| Git/code-host and selected harness credentials/catalog | System/vendor | Execution | Verify qualified routing and authentication without serializing secrets | Host operator |

### 6.3 Risks, assumptions, and rabbit holes

| # | Risk / assumption / rabbit hole | Likelihood | Impact | Mitigation or test | Owner |
|---|---|---|---|---|---|
| R1 | A retained claim records an attempt whose host crashed before useful work | Med | Missed occurrence | Accepted v1 tradeoff; incomplete state is visible and later occurrence eligible; WR-1 owns recovery | @mdarabi |
| R2 | Native cron cannot wake a sleeping host or portably express arbitrary foreign timezones | Med | Missed/wrong time | Refuse unsupported timezone; no catch-up promise; WR-3 and WR-4 own expansion | @mdarabi |
| R3 | Ten-minute CI may be blocked by runner queues or required work; proposed 90-minute job budget and routing requirements may need tuning | Med | Objective unmet or excessive work | Prompt measures first and reports limits; budget/harness/routing remain YAML settings; native catalogs keep existing ownership; revisit after first real run | @mdarabi |
| R4 | An external human cron edit races an installer | Low | Lost host configuration | Serialize Pyxis installers, recheck before writing, retain backup; native crontab has no external-writer CAS guarantee | Host operator |
| R5 | Shared refs are deleted or rewritten outside maintenance | Low | Lost idempotency authority | Limit ref write access and retain history; unexpected state fails closed where observable; no Byzantine-host guarantee | Repository operator |

### 6.4 Release criteria

Functionality and reliability are proved by DOD-1 and DOD-2; platform/privacy
by DOD-3; job behavior by DOD-4; reversible adoption by DOD-5; operator control
by DOD-6; four-harness routing/isolation/publication by DOD-7. Every release
criterion is owned by that blocking evidence.

### 6.5 Rollout and reversibility

Installation is explicit and never occurs merely because core Pyxis is present.
The job's `enabled` setting on the published branch is the execution kill
switch. Disabling prevents future launches; it does not revoke an already
claimed process. Uninstall removes scheduled entries; stopping an active process
uses ordinary process cancellation and retains its claim.

Adoption follows hermetic validation, then an isolated headless smoke. Cutover
backs up crontab and replaces the legacy entry and new entry in one installation
operation. Rollback first removes new entries and confirms active processes have
stopped, then restores the prior entry; it never runs both executors in parallel.
No percentage rollout or A/B experiment applies to this explicitly installed
host capability. High/critical requirements are covered by these kill-switch,
confinement, claim, and rollback drills before adoption.

### 6.6 Launch readiness

| Area | Item | Owner | Status |
|---|---|---|---|
| Support/docs | CLI usage and runbook for install, disable, failure, and uninstall | Implementer | Pending implementation |
| Privacy/platform | Credential-free artifacts and supported scheduling matrix | Implementer | Pending verification |
| Adoption | Migration, smoke, and rollback evidence | Host operator | Pending verification |
| Pricing/comms/localization | Existing account costs; no new paid service, notifications, or translated interface | @mdarabi | No new surface in v1 |

| Reviewer | Role | Date | Verdict |
|---|---|---|---|
| @mdarabi | Operator | Pending | Launch after blocking DoD evidence |

### 6.7 Post-launch review

Trigger: the first real scheduled CI investigation produces an outcome.
Owner: @mdarabi. Record adopt, iterate, or disable with installation, claim,
report, and PR evidence on the tracking work item. Any changed locked contract
uses the normal new-draft route; operational findings do not edit frozen text.

## 7. Alignment

### 7.1 FAQ

The CLI is installed once and receives an explicit repository for each job.
Each host runs installation when its schedule needs reconciliation. Published
prompt and disable changes apply at invocation; sleeping hosts and crashed
attempts have no Pyxis catch-up or retry. The configured job timezone supplies
the date key, including for manual invocations; cron owns launch timing and
native DST behavior. Harness/routing changes do not erase attempted dates.
A failed job does not prove CI improved. Compact claims/outcomes are shared;
the append-only audit journal and detailed diagnostics stay local and ignored.

### 7.2 Open questions

None solution-blocking. Deferred recovery, schedule distribution, catch-up,
foreign-timezone support, and non-daily schedules have explicit waiting-room
owners. Execution budget/routing tuning is a configuration review after adoption, not an unstated
implementation decision.

### 7.3 Decision log

| Date | Decision | Alternatives considered | Why | Who |
|---|---|---|---|---|
| 2026-10-05 | Daily 02:00 America/Los_Angeles; ten-minute CI target | Different cadence/time | Operator's selected recurring task | @mdarabi, direct conversation |
| 2026-10-05 | Create or update one draft PR, human approval/merge | Local-only report or automatic merge | Reviewable improvement with human authority | @mdarabi, direct conversation |
| 2026-10-05 | Jobs/prompt/config in tracked YAML; shared Git-ref idempotency | Machine-local configs and local-only overlap protection | Central control across installed machines | @mdarabi, direct conversation |
| 2026-10-05 | One cron entry per job with literal job ID/repository path | Frequent dispatcher | Explicit operator preference | @mdarabi, direct conversation |
| 2026-10-05 | Defer crash recovery; log failed attempts and continue later | Automatic takeover/retry | Operator bounded v1 scope | @mdarabi, direct conversation |
| 2026-10-05 | Use Go in the installed core CLI's existing module | Shell or nested repository-local module | One installed binary serves multiple repositories | @mdarabi, direct conversation |
| 2026-10-05 | Open this feature draft and prepare its contracts | Continue only the scratch proposal | Operator answered “yes, do it” to draft creation | @mdarabi, direct conversation |
| 2026-10-05 | Job selects any existing harness and semantic routing requirements | Codex-only v1 and duplicated native model settings | Reuse existing runtime/routing for all four harnesses; P-1 | @mdarabi, direct review conversation |
| 2026-10-05 | Daily-only v1 claim identity is job ID plus configured-timezone date | Minute matching, grace windows, and configuration-dependent keys | Daily jobs need one retained attempt per date; P-2 | @mdarabi, direct review conversation |
| 2026-10-05 | Runner-owned Git storage for policy/claims and a private clone per winning attempt | Fetching or working in the enrolled/shared checkout | Preserve concurrent operator work and Git metadata; P-3 | @mdarabi, direct review conversation |
| 2026-10-05 | Append-only audit journal at `.pyxis/maintenance/logs.jsonl` | Scattered per-run logs without a single lifecycle stream | Capture success/failure/outcomes and debug/audit evidence including pre-claim refusal; P-4 | @mdarabi, direct review conversation |

### 7.4 Changelog

| Date | Change | Author |
|---|---|---|
| 2026-10-05 | Initial contract from attributed operator decisions; no lock or launch approval claimed | Codex |
| 2026-10-05 | Applied operator review rulings P-1–P-4 for harness routing, daily identity, complete Git isolation, and the local audit journal; exact-head approval remains pending | Codex |

## 8. Contract readiness — the executability gate

| Check | Pass? | Notes |
|---|---|---|
| Problem basis accepted | Yes | Direct operator request and §2.6 attribution |
| Zero solution-blocking TBDs/open questions | Yes | §7.2 and explicit waiting room |
| Every Must FR has fit criterion and verification | Yes | §5.1 |
| Every requirement has risk tier and safety plan | Yes | §5.1–§5.2; §6.5 covers authenticated and scheduling behavior |
| Non-goals and protected neighbors named | Yes | §3.4 and NR rows |
| Every primary job covered | Yes | J1→FR-1–FR-5/FR-8; J2→FR-6; J3→FR-9–FR-13/FR-15; J4→FR-7/FR-11/FR-14 |
| Every key flow has functional/failure/regression coverage | Yes | §4.3 coverage column |
| Every metric instrumented | Yes | EVT-1, EVT-2, EVT-3, EVT-4; required JSONL audit journal |
| Blocking DoD rows trace to accepted criteria | Yes | §5.8 Source column |
| Risks and assumptions owned | Yes | §6.3 |
| Human rubric owned | Yes | @mdarabi, §5.9 |
| DoD is explicit and evidence classes named | Yes | §5.8; evidence remains pending implementation |
| Contract is approved for implementation | Pending | Exact-head review and lock through this draft PR |

## Appendix A — Writing rules for requirements

Requirements use the template's EARS patterns, stable IDs, and named verification
methods. Requirements describe observable behavior; engineering layouts remain
in the draft tech spec. Open-draft changes follow attributed problem rulings;
locked text evolves through a new draft version.
