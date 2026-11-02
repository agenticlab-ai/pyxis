# Maintenance jobs — tech spec

Status: draft for the operator's exact-head contract review. This file contains
the concrete layouts within their serving decisions. It grants no contract lock,
human approval, merge, or unattended installation on additional repositories.

## Principles

The design honors AP-1 (mediums behind consumer-owned ports), AP-5 (structured
observability), AP-6 (schema-versioned boundary data), AP-11 (reuse routing and
runtime predicates), AP-12 (accounted resource lifecycle), AP-13 (claims about
outcomes require evidence), AP-16 (persistent safety refusals are counted),
AP-20 (enforced workspace isolation), AP-21 (one recorded route per attempt),
AP-25 (vendor-neutral repository surfaces), AP-27
(extension by additive file units and derived registries), and RW-5 (approved
design before engine implementation). Go, closed configuration,
explicit cancellation, and fail-closed publication follow AGENTS.md. The
operator's requested `.pyxis/maintenance/logs.jsonl` is the explicit local-audit
path exception to the usual producer-log layout; detailed diagnostics remain
under ignored `.pyxis/logs/maintenance/`. Shared outcomes stay in Git. No
departure from the cited principles is proposed. The scheduler is deterministic; only the
configured external agent performs inference.

### D-1 — Core CLI and repository-owned configuration

**Decision:** Implement maintenance in the existing Go module and installed core CLI, with thin commands and a closed, versioned repository-owned jobs file.

**Serves:** FR-1 · FR-2 · FR-14 · FR-15 · UB-1 · NR-2 · NFR-1 · NFR-2

#### Concrete layout

The public command grammar is:

```text
pyxis maintenance install --repo <absolute-root> [--preview] [--replace-legacy-ci-daily]
pyxis maintenance run --repo <absolute-root> --job <id>
pyxis maintenance status --repo <absolute-root>
pyxis maintenance uninstall --repo <absolute-root> [--preview]
```

Flags are parsed fail-closed with the existing CLI argument guard. Unknown flags,
extra positionals, absent roots, or invalid IDs refuse before side effects.
`--repo` resolves to the canonical absolute Git worktree root; symlink aliases
produce the same installation identity. Each command accepts an explicit root,
independent of the installed binary's directory or the caller's working directory.

`cmd/pyxis/maintenance.go` supplies dispatch and composition as a self-registering
command file. Introduce one minimal root-command registry seam: a typed command
descriptor has its key, usage, and handler; dispatch consults it before the
unchanged legacy dispatch, and usage includes its sorted derived descriptors.
The registry rejects duplicate keys and collisions with legacy verbs. The new
maintenance file owns its descriptor; it adds no maintenance case or literal
usage line to `main.go`. A conformance fixture registers a second file unit and
proves dispatch/help discovery without central edits. This foundational seam
does not require migrating all existing verbs in this feature. Focused
`internal/maintenance` code parses policy, renders entries, derives daily keys,
and owns execution/publication. Cron, claim storage, and bounded historical CI
evidence interfaces remain consumer-owned. The evidence reader is a small
optional capability on the existing code-host adapter axis, not a second
code-host client or an agent credential grant. Agent execution reuses `executor/port.AgentRuntime`,
`CatalogProvider`, `LaunchBuilder`, and the native launch fidelity contract;
there is no second maintenance agent-launch port. Concrete integrations live on
their adapter axes under `internal/adapters/`; the command root imports and
wires the existing four harness adapters. Existing packages for Git refs,
repository resolution, logs, credentials, and process management are reused.
Issue-specific temporary leases are not repurposed into daily claims, and no
synthetic tracker task or worker session is required.

The schema is `pyxis-maintenance-jobs/v1`. The source block bootstraps the trusted
remote/branch binding from the operator's selected local repository. The executor
then reads policy at one fetched published commit and requires that published
source binding to agree; a disagreement refuses with a reinstall instruction.
The installer retains a closed, schema-versioned installation manifest in
machine-local runtime state scoped by the canonical repository-root hash. It
contains a generation ID, ready/installing/unavailable state, source binding,
derived host paths, and normalized cron/timezone fingerprints for installed
jobs. It contains no prompt or runner settings. `run` requires a ready manifest
and its recorded remote/branch binding; an absent or unavailable binding names
installation as the remedy. Manifest publication and execution use the D-2
transaction protocol; changing a file alone cannot expose new authority.
Changing the local source block without installation does not redirect execution.
The source block is still shared YAML policy, and this derived binding contains
no duplicate prompt or runner settings. Local source binding changes require an
explicit operator installation step. They do not cause a fallback to local
prompt content. A repository's operator
and configured Git remote remain the trust root; this is not protection against
a malicious operator or a malicious writable remote.

```yaml
schema_version: pyxis-maintenance-jobs/v1
source:
  remote: origin
  branch: main
jobs:
  - id: ci-performance
    enabled: true
    schedule:
      cron: "0 2 * * *"
      timezone: America/Los_Angeles
    timeout: 90m
    runner:
      harness: codex
      routing:
        capability: frontier
        deliberation: maximum
        context: standard
        assurance: ordinary
    delivery:
      mode: draft-pr
      branch: perf/ci-maintenance
    prompt: |
      Inspect and improve required Pyxis CI toward at most 10 minutes.
      The full initial prompt defined in D-6 replaces this abbreviated example.
```

Production structs have explicit YAML/JSON tags. Use the existing YAML dependency
with unknown-key rejection and document-level validation: exactly one document,
no duplicate keys, unique job IDs, no unresolved aliases, valid strings/enums,
positive durations, valid source/branch refs, and supported schema.
IDs use lowercase ASCII letters, digits, and hyphens, begin with a letter, and
are at most 64 characters. Job IDs are identity, not a label to change for another
attempt. Enabled draft-PR jobs cannot share a delivery branch within one file.
V1 supports only the once-per-day five-field form `<minute> <hour> * * *`,
with single decimal minute/hour values in their native ranges. Lists, ranges,
steps, non-daily day/month/weekday fields, macros, seconds, commands, and shell
snippets are refused. There is no startup-grace field or minute-eligibility
evaluator. `runner` contains only `harness` and `routing`: harness uses the
existing closed `entity.Harness` values claude/codex/cursor/opencode; routing
reuses `executiondemand.ProfileMinimum`'s capability, deliberation, context,
and assurance enums. All four dimensions are explicit and closed. Arbitrary
native argv, model overrides, executable expressions, and config snippets are
not accepted as maintenance policy.

Jobs YAML owns the maintenance prompt, schedule, timeout, harness/routing request,
and delivery policy. Existing `.pyxis/config.yaml` `harness-config` continues
to own qualified native model profiles; fetch it at the same published commit
as jobs YAML and parse it with `harnessconfig`. Do not apply local overlays or
read profiles from an ambient cwd. Catalog changes use their existing owner;
maintenance does not duplicate a model roster or routing ladder. Resolve the
job's semantic request using `nativerouting.Resolver` and its selected harness's
validated implementation-role catalog in normal mode. Retain the complete
selected profile and policy/qualification identity once; D-5 launches that
selection without reinterpretation.

The existing registry's default factories discover profiles from the ambient
repository. Add one typed configured-resolution path to that same registry so
the command root can supply profiles captured from the published commit.
Each existing adapter's registration owns its configured constructor using its
current `WithProfiles` option. Legacy `Resolve` retains its behavior; configured
resolution never falls back to ambient defaults. The registry owns one derived
harness set, not a second maintenance factory list or central harness switch.
Conformance proves an added registration can resolve supplied profiles without
a shared dispatch edit and all four existing adapters use the supplied catalog.

Absolute host binary/root locations and the tool search path are derived local
installation bindings, not a second shared-policy file. Runtime state contains
clones, caches, and installation bookkeeping; diagnostics use ignored logs.
The jobs file is repository-owned configuration: core installation, convergence,
and upgrades preserve it, and installation remains explicit rather than being
triggered for all Pyxis-enabled repositories.

Acceptance: two fixture repositories use one compiled CLI from a hostile cwd;
schema and argument negative cases cause zero cron/Git/agent mutations; all four
harnesses resolve through their existing catalogs; unsupported native routes
refuse before claiming; core ownership tests prove job configuration survives
upgrade/convergence. The CI sample's route targets the existing qualified
frontier/maximum provision rather than freezing a vendor model name.

#### Observability

Named parse errors include file/field and remediation, never entire environment
or credential files. CLI status distinguishes desired installation, actual owned
entries, latest shared attempted outcome, and absent/incomplete evidence.

#### Security and privacy

Configuration is trusted executable policy after normal repository publication.
Arguments are typed and validated before subprocess construction. Source/branch
and job IDs cannot inject options, ref syntax, newline, NUL, or cron command text.
Core upgrades do not overwrite reviewed job policy with shipped sample jobs.

### D-2 — Idempotent per-job native cron installation

**Decision:** Reconcile a repository-scoped set of direct per-job cron entries through a serialized Go installer, preserving every entry outside that set.

**Serves:** FR-3 · FR-4 · FR-5 · FR-14 · UB-4 · UB-5 · NR-1 · NFR-2

**Builds-on:** D-1

#### Concrete layout

The crontab adapter exposes read and install operations with context cancellation.
Read distinguishes an absent user crontab from permission/provider failure.
Rendering is pure and deterministic. Each owned entry is preceded by a marker
containing the schema marker, SHA-256 of the canonical local repository root,
and job ID. Ownership requires the complete generated marker/entry grammar;
malformed or duplicated ownership records refuse rather than taking over
unrelated text. Different repositories may use the same job ID without colliding.

For this host, a rendered command has this shape:

```text
# pyxis-maintenance/v1 repo=<root-hash> job=ci-performance
0 2 * * * PATH='<host-tool-search-path>' '/Users/operator/.local/bin/pyxis' maintenance run --repo '/Users/operator/agenticlab-ai/pyxis' --job 'ci-performance' >> '<repo>/.pyxis/logs/maintenance/cron.log' 2>&1
```

The binary is the stable installed core executable resolved from the installer,
not `go run`, an ephemeral build-cache executable, or a repository-local script.
The installer checks this location and the needed external tools. It captures the
operator's tool search path for the entry; generated PATH applies to that command,
not to other cron jobs. Authentication is supplied by the existing host credential
stores, not serialized into cron. Canonicalization/quoting handles spaces and
single quotes; cron percent escaping is a separate rendering step. Newline/NUL
values are rejected. Output redirection captures early invocation failures while
the executor owns detailed run logs.

Build desired entries for every enabled job before requesting a write. Replace
or remove only this repository's owned pairs, retaining all other bytes and
their ordering. Disabled/removed jobs are removed; a valid empty list removes
all owned entries. Missing/invalid YAML refuses; explicit uninstall is the
removal path without policy. Byte-identical output makes zero install calls.
Preview reports changes and makes zero external mutations.

Serialize installation and execution-authority transitions with a per-user
machine lock. Under that lock, read, compute, retain permission-restricted
crontab/manifest backups, and re-read to detect intervening change. Retry a
detected change a bounded number of times. Before writing cron, atomically mark
the manifest installing; install the proposed table, verify its owned entries,
then atomically publish the new ready generation. Release the lock only after
commit or rollback. An executor may fetch policy outside this lock, but must
reacquire it, prove the same ready generation and schedule binding, and hold it
through shared claim ownership proof before proceeding. A generation change
requires a fresh bounded read, never use of the earlier fetched authority.

On failure, restore and verify the previous crontab/manifest pair under the
lock, or retain unavailable state with backup evidence if verification is
uncertain. Uninstall uses the same protocol. A crashed installer can leave an
installing manifest: execution refuses and an explicit subsequent installation
reconciles it; no job takeover or retry is implied. A queued old invocation
cannot claim against intermediate or failed authority. An already proven claim
remains bound to its captured policy and need not be stopped by routine policy
reconciliation. Native crontab
installation is atomic as one table replacement but has no compare-and-swap
with a simultaneous external human writer; that limitation is explicit in R4.
Do not promise preservation against an undetectable external write race.

V1 requires a known native scheduler timezone matching each enabled job's IANA
timezone. The adapter identifies supported provider semantics and evaluates
retained environment directives in order at each proposed entry position,
including entries updated in place. It checks both the system timezone and the
effective scheduler timezone. A preceding `CRON_TZ=UTC` on a provider honoring
that directive refuses a Los Angeles job even on a Los Angeles host. Do not
treat `CRON_TZ` as portable to macOS or inject global timezone settings.
Require the provider's supported POSIX shell (`/bin/sh` in the initial profile),
no nonempty inherited shell-startup injection such as `ENV`/`BASH_ENV`, and no
unsupported/random startup delay directives. Interpret
`TZ`, `CRON_TZ`, and delay settings according to the identified provider rather
than guessing; unknown/mismatched semantics refuse before installation. Preserve
the original globals verbatim and name the conflicting directive in the error.
The host timezone supplies DST movement. There is no wake, catch-up, or foreign-
timezone workaround in this contract.

Acceptance: golden files cover install/update/removal/empty/no-op/preview,
ownership collisions, exact unrelated bytes, quoting, write failure, concurrent
Pyxis installers, unsupported timezone, inherited scheduler/shell/delay settings,
and transaction failure. A paused intermediate-manifest fixture fires an old
entry then fails the cron write: zero claims/launches use the failed generation.
A native integration smoke proves
direct invocation under a minimal cron environment.

#### Observability

Emit EVT-1 with repository identity, desired owned-entry count, changed flag,
outcome, and backup reference on failure. Status recomputes desired entries and
reports drift without rewriting crontab.

#### Security and privacy

No global SHELL/PATH/MAILTO/TZ change affects unrelated entries. Do not put tokens
in command text. A valid owned marker is required for deletion. Backups contain
existing crontab content and remain local with restrictive permissions.

### D-3 — Published policy and daily invocation identity

**Decision:** Read one published policy revision in runner-owned Git storage and derive the claim key from job ID plus the invocation's date in its configured timezone.

**Serves:** FR-6 · FR-7 · FR-8 · FR-9 · FR-15 · UB-2 · UB-4 · NFR-4

**Builds-on:** D-1 · D-2

#### Concrete layout

Capture the invocation's UTC start time before blocking work. Read and validate
a ready installation manifest, then fetch its trusted remote branch into a
private runner-owned control repository, outside the enrolled checkout and its
Git common directory. Resolve one exact commit and read jobs YAML plus the
existing harness catalog configuration from that commit. Use this same owned
storage for claim transport. Never fetch into the enrolled .git, mutate its
refs/FETCH_HEAD/config/index, switch its branch, or use local policy as fallback.
A local source edit cannot redirect the installed source binding.

Validate published policy and source binding before selecting a job. Disabled
or removed jobs emit a named skip; invalid IDs or malformed policy refuse.
For an enabled job, require its published normalized cron/timezone fingerprint
to equal the ready manifest's installed fingerprint before claiming. A changed
schedule requires explicit reinstall on each host, even when the old and new
times fall on the same date. Prompt, timeout, harness/routing, and disable
updates remain live published policy. Reprove the ready generation under the
D-2 lock immediately before shared ownership acquisition; a changed generation
requires a bounded fresh read.

Convert the captured start time into the published job's IANA timezone and
format YYYY-MM-DD. The identity is the configured repository remote plus job
ID plus that date. In refs/pyxis/maintenance/<job-id>, the record path is
occurrences/YYYY-MM-DD.json. Host, local root, launch hour, harness, routing,
prompt digest, and Git config revision do not enter the key. Date remains
immutable if fetching, claiming, or execution crosses midnight. All hosts using
the same published timezone calculate the same date for the same instant.

V1 schedules are once per day. Cron owns process timing; the executor does not
match a cron minute, apply startup grace, or implement catch-up. A manual run or
delayed same-date invocation uses the same key and can consume that date's
first attempt at any hour. Native cron's DST/missed-time dispatch semantics
remain provider-owned: two invocations on a repeated date cannot launch twice,
and Pyxis does not synthesize a missed invocation. A new date is independently
eligible, including after a failed or incomplete prior date. Different dates
are not a global active-run lock; the initial 02:00 daily job's 90-minute budget
fits its cadence. Unusual manual timing or future policy changes can overlap
different dates, so D-5 also guards publication against remote branch movement.

Resolve the selected harness's native route, installation/authentication, and
supported process confinement before consuming a daily claim. Use the existing
implementation-role catalog and normal native resolver; an unsupported request
or unusable dependency refuses with a named journal event and counter. Freeze
the selected profile, qualification/policy identity, exact config commit,
timezone/date, and deadline context for this attempt. No automatic harness
fallback or retry can change an already claimed date's authority.

Acceptance: injected-time fixtures cover starts at different hours on one date,
manual runs, UTC versus configured-timezone date boundaries, midnight crossed
during fetch, repeated DST invocations, and independently eligible next dates.
Dirty enrolled checkout snapshots include files/index/config/refs/FETCH_HEAD
before and after policy and claim operations. Disabled/removed/unmerged policy,
failed fetch, unsupported routing/authentication, and changed installed schedule
produce zero launches. A delayed invocation still uses its captured date rather
than a fresh post-fetch clock read.

#### Observability

D-8 logs the published revision, installation generation, selected route,
configured timezone, derived date, and skip/refusal reason, including failures
before claim. Persist closed typed local counters by repository/job/refusal cause
for safety/dependency failures. Count once per invocation; retain lifetime total,
first/last timestamps, and consecutive count; expose them in status. Reset the
consecutive count after a validated coordination decision while preserving
lifetime totals. Disabled/removed jobs and proved claim loss are ordinary skips,
not malfunction counters. Counter-write failure is logged and never grants
launch. No notification or retry service is introduced.

#### Security and privacy

One immutable published commit supplies joined maintenance intent and native
catalog policy. Local installation authority cannot be rewritten by fetched
instructions. Control repositories have private ownership journals and D-5's
resource lifecycle; losers clean only positively owned, quiescent resources.
Shared claims and the enrolled checkout are never reaped as scratch state.

### D-4 — Retained daily Git claims with positive ownership proof

**Decision:** Use the existing Git-ref CAS store for retained per-job date records; launch only with positively established ownership, without automatic recovery.

**Serves:** FR-6 · FR-7 · FR-11 · UB-2 · UB-3 · NFR-4

**Builds-on:** D-3

#### Concrete layout

The shared ref is refs/pyxis/maintenance/<job-id> in the configured repository
remote. Its path for a date is occurrences/YYYY-MM-DD.json. The adapter wraps
internal/gitrefstore and uses runner-owned control storage; it does not duplicate
Git transport/CAS logic or write the enrolled checkout's Git directory.

A closed versioned record contains job ID, configured-timezone date, timezone,
owner nonce, published config commit, selected harness/native profile and
policy/qualification identity, claimed timestamp, outcome enum, optional
finished timestamp/duration/exit code, bounded summary/cause, and optional
draft-PR URL/exact code head and safe evidence references. Shared references
use remote URLs or opaque artifact IDs, not private host paths. Records contain
no prompts, code diffs, full output, or credentials. Native route/config changes
do not participate in identity and cannot reset an already claimed date.

After preflight and before execution cloning/launching, mint an unpredictable
owner nonce with crypto/rand. CAS mutation inserts the claimed daily record
only when absent. Existing records are consumed dates and cause a no-op skip.
Commit/push use the exact prior ref OID as expected lease; an initially absent
ref requires an absence expectation. Only metadata uses this CAS mechanism;
normal code publication preserves ordinary pushes and hooks.

Launch requires an exact remote winning-record read proving this invocation's
nonce, job/date, captured config, and selected route. A CAS conflict rereads and
replays within the existing store bound. An uncertain push is not permission to
launch: obtain exact positive proof or refuse, retaining any claim that may
have reached the remote. Incompatible or malformed state fails closed.
Coordination is remote/job/date regardless of local root, host, or launch hour.

Only the same owner can transition its claimed date to succeeded, failed, or
timed-out, preserving immutable identity and nonce. Success requires a retained
final report and supervisor-verified artifacts. Verify publication against the
exact remote code head and draft PR rather than agent prose or exit zero.
A no-change success requires a retained honest report and observable unchanged
clone state. Missing/unproved evidence is failed with its cause. Timeout stays
timed-out even if useful work is retained. These outcomes describe execution
and evidence; a separate measured report field establishes whether CI reached
ten minutes. Bounded summary/cause and meaningful evidence references retain
shared diagnostic/outcome signal; D-8 holds local lifecycle detail.

A terminal metadata update failure is recorded locally and never erases the
claim or authorizes another attempt. A crash or unproved process-tree quiescence
can leave claimed/incomplete state with no publication. No TTL, deletion,
lease stealing, retry queue, or takeover is implemented. The next date is
independently claimable. Remote ref access/history retention remain operator
responsibilities; privileged external deletion/rewrite is outside deduplication
guarantees.

Acceptance: real temporary bare-remote clients compete at different hours of
one date; exactly one positively proved owner may launch and all others skip.
Manual runs, configuration/harness changes, failure, and interrupted startup
consume the same daily key; the next date is eligible. Missing credentials,
malformed records, uncertain pushes, and terminal-update failure exercise
fail-closed/no-retry paths while preserving enrolled Git state.

#### Observability

EVT-2 records won/lost/unproved, date, route, and config revision through D-8;
EVT-3 records terminal outcomes and retained evidence. Status reports an
unterminated claim as incomplete rather than inferring success from age.

#### Security and privacy

Only configured metadata refs are writable by claim operations. Validate nonce
and immutable fields before transition. Metadata-only hook scoping never
disables normal hooks on the supervisor's code branch. No cleanup deletes
claims or releases an attempted date.

### D-5 — Existing harness routing, isolated execution, and supervised delivery

**Decision:** Execute the configured existing harness in a private independent clone with one frozen native route; the Go supervisor owns verification, Git mutations, and draft-PR delivery.

**Serves:** FR-8 · FR-9 · FR-10 · FR-11 · FR-12 · FR-15 · UB-2 · UB-3 · UB-6 · NR-2 · NFR-3 · NFR-4

**Builds-on:** D-1 · D-3 · D-4

#### Concrete layout

After positive claim proof, create a private independent clone under local
runtime state keyed by repository/job/date/owner nonce. Every winning attempt
gets separate source files, .git, index, branch checkout, runtime state, and
caches, even when continuing an existing PR. Do not use the enrolled checkout
or a linked worktree sharing its Git common directory. The configured --repo
root identifies the installation and local audit/artifact destination, never
the harness Workdir. Before launch, mechanically prove that work/runtime/control
storage and Git metadata cannot overlap the enrolled checkout or another run's
writable resources. Supervisor operations preserve its files, branch, refs,
FETCH_HEAD, index, and Git configuration.

Use the existing runtime registry's configured-resolution seam (D-1) and the
existing Claude, Codex, Cursor, and OpenCode adapters. Maintenance depends on
the existing AgentRuntime AuthCheck/Launch/Await/Kill contract, not four new
launchers or a shell command. Resolve and validate the published catalog and
job ProfileMinimum with the existing native resolver and qualification policy.
Use LaunchBuilder and the adapter's actual native launch capture/fidelity
predicate to verify native model, effort, and controls. Record and use the
selected profile once; adapters cannot reinterpret the semantic request.
Validate route/authentication/process-access prerequisites before the D-4
claim. Unsupported routing or installation/authentication produces no consumed
daily claim. Once claiming succeeds, later preparation/launch errors consume it.

Construct port.LaunchSpec with the resolved native model/effort, the literal
maintenance brief, Purpose implementation, the independent Workdir, private
RuntimeStateDir, and configured HeadBranch. Session is nil: each date starts a
fresh harness conversation. Reuse runtime.AssembleBriefContext and its current
implementation delivery protocol: the agent edits/tests/reports, while the Go
supervisor alone mutates Git and publishes after process quiescence. Do not
fake a tracker task, reuse an issue-based lease, or select review purpose to
avoid implementation controls. No new maintenance-specific prompt protocol,
model roster, or native invocation builder is introduced.

The brief carries immutable job/date/timezone, config and catalog identities,
baseline, branch/PR, evidence paths, deadline, prior compact outcome, and the
private final-report path. Require a closed maintenance-report/v1 JSON artifact
there for success/no-change: job/date/config identity, changed/no-change/blocked
result, bounded summary, evidence references, validation results, proposed
title/description, and optional typed CI measurements/limits. Read it after
quiescence; it is evidence, not authority to change policy. RunResult.Incomplete
or a blocked/missing/invalid report is a named failed attempt, never exit-zero
success. This job-local artifact contract does not replace shared delivery
protocol or native prompt transport.
The supervisor supplies recent-CI and pending-PR evidence through a
maintenance-owned EvidenceReader port. Existing proposal/check readers and
authenticated code-host transport are reused. Historical successful-run timing
is a new optional adapter capability: current aggregate-check and failed-log
APIs do not already provide it. Add its implementation on the existing
code-host adapter axis, wired by the command root. No agent-side API client,
synthetic tracker task, or duplicated authentication implementation is needed.
The closed maintenance-ci-evidence/v1 result records the captured repository
and base, sample window, fetch time, required-check settings or their explicit
availability limit, and typed workflow/run/job/step identities, attempt,
commit/event, status/conclusion, source timestamps, and evidence URLs. Include
pending proposals and the exact dedicated-branch head through existing readers.
Bound the initial snapshot to D-6's twenty successful runs/fourteen days, with
non-success results separate. The reader has an injected deadline, at most 200
candidate workflow runs and 10 MiB of retained evidence; reaching any bound
returns an explicit cutoff/completeness limit rather than a complete-gate claim.
Group only positively matched commit/event/check evidence. Missing required
settings, timestamps, jobs, or steps remain named measurement limits. Retain
these data in run-owned evidence files and the brief without code-host secrets.
Model output, workflow text, and fetched evidence are data, not new authority.

Native CLI flags, prompt transport, feature settings, and output parsing remain
owned by existing harness adapters. The selected adapter's qualified unattended
contract plus the shared runtime's OS process confinement supplies the execution
boundary. The current shared confinement controls writes, not host credential
reads; private HOME and filtered environment alone do not establish the latter.
Add an optional host-owned ProcessAccessPolicy to the existing LaunchSpec and
CommandAccess path. The maintenance supervisor always supplies its restrictive
policy. It is not a jobs YAML field, model instruction, native sandbox flag, or
maintenance-specific process launcher. Existing launches without that policy
retain their current behavior. Extend the existing versioned delegated-launch
wire data and capability checks so descendants can only narrow both read and
write authority; a child cannot omit or widen its ancestor's policy.
Do not turn Codex approval/sandbox/effort flags into cross-harness
configuration, or inherit an interactive shell startup. The installed tool
search path resolves the selected CLI. Use shared environment sanitization and
selected-model authentication snapshotting in private runtime state (factoring
the existing review-named helper into a shared helper if needed without changing
review behavior). Project only the selected harness/provider's required model
authentication; a multi-provider store is not copied wholesale. Other provider
tokens, host settings, hooks, MCP configuration, and credential helpers are not
included. The supervisor obtains any required model credential from Keychain
before confinement and writes the selected private file-store projection.

Compile a positive filesystem read policy from the private source/runtime
roots, verified tool installation/dependency paths, and enumerated public OS
runtime/certificate/resolver files. Resolve and pin path ownership; never grant
all of a host home, credential directory, `/`, or a tool installation's parent
merely for convenience. Tool roots that overlap private host data refuse.
Darwin uses default-denied file reads with these explicit allows plus the
existing write controls. Linux builds a private mount view from the allowlist
instead of read-only binding the host root. Private temporary/device/process
views support the native CLI without exposing host process environments. Host
credential files, the enrolled Git configuration, control repositories,
supervisor authentication state, and other runs are outside the read grant.
Clone-local Git configuration remains readable but is supervisor-generated and
credential-free: remote URLs have no userinfo, and no extra authorization
headers, credential helpers, or includes reference host secrets. Keep any Git
transport credential solely in supervisor context/private storage outside the
harness read policy. Inherited descriptors, host SSH-agent/keychain/credential
broker IPC, and host-process inspection cannot bypass the restriction; only
the existing authenticated scoped launch broker may delegate an equal or
narrower process policy. Preserve allowed model network traffic and run-local
IPC. Unsupported OS enforcement or a tool/login requiring host credential
service access refuses before claim instead of silently widening access.

Before claim, use a control-owned minimal Git fixture and private model-auth
state to probe this restrictive policy through the same shared wrapping path
used by the selected adapter. Its registered prerequisite check is bounded and
performs no model inference; it does not create an execution checkout or run the
maintenance prompt. Probe absolute and symlinked reads of injected Git/code-host
secrets, credential-service access, and delegated-child policy widening, as well
as required source/tool reads and the selected model-auth snapshot. After claim,
bind and probe the exact execution clone/runtime paths before native launch;
failure at this later setup step consumes the date. A wrapper executable merely
existing or a sanitized environment is insufficient. Every four-adapter
Darwin/Linux conformance fixture must prove these negative and positive cases.

Model authentication is the only selected credential grant; host Git/code-host
credentials and mutable user settings remain outside the harness. Source edits
are allowed only in the private clone; its Git metadata,
the enrolled checkout, audit journal, and other attempts remain protected from
agent writes. Credential snapshots are never exported as diagnostic artifacts
and are removed only after positive process-tree quiescence.

Resolve the dedicated branch and base using supervisor-owned Git/code-host
operations. Continue an existing pending proposal's concern. An absent branch
starts at the captured published base. If a prior proposal's head is already an
ancestor of that base, advance the branch normally from the base. Incompatible
history or unexpected remote movement is a blocker, not permission to force
push. A ready pending PR can yield a useful no-change report.

After the harness is quiescent, capture changes and the retained report.
Configure normal code hooks through the existing Pyxis setup route, stage only
explicit paths, and run the required verification through the shared gate path
before publication. Go owns the commit and normal branch push; it never disables
hooks, weakens required checks, approves, or merges. Compare the exact remote
branch head with the captured expected head immediately before publication,
use normal non-forced code pushes, and retain work on divergence. Exact code-host
lookup creates/reuses the configured branch's draft PR; verify the exact pushed
head and PR after publication rather than trusting an agent's claim or an
eventually-consistent first read. An unchanged tree plus a retained honest report
can succeed without a commit/PR mutation. Agent-local Git writes/publication
are denied mechanically, not merely by prompt wording.

All blocking operations take contexts. Bound pre-claim network/authentication
work and setup separately; the configured job timeout covers the entire claimed
attempt, including clone preparation, launch, checks, publication, and evidence
capture. Use a bounded cleanup context after cancellation. Await/Kill must
positively prove the whole process tree stopped before export, Git publication,
or disposable workspace release; direct-parent exit is insufficient. Unproved
quiescence retains claimed/incomplete authority and work with no publication.
No repair loop, agent continuation, automatic retry, or harness fallback starts
another agent after that date has been consumed.

Preserve available tracked/untracked/binary changes and a patch on nonzero exit
or timeout. Explicit path staging for patch export is supervisor-owned and
occurs only after quiescence. Record report/patch absence honestly if failure
precedes useful work. D-8 correlates all artifacts/outcomes. No routine external
notification or unrelated PR comment is emitted.

Account for pre-claim control repositories, execution clones, caches, credential
state, and scratch artifacts with one versioned local ownership journal
(repository/job/date/nonce, process scope, lifecycle, and export references).
A successful quiescent workspace with verified exports can be explicitly
released. Failed/incomplete/unexported work is retained as operator-managed
partial-work evidence, not automatically expired. Ship the same-change Go
janitor: one bounded pass during install/run preparation consumes only owned
release-pending resources with verified exports and positive quiescence, audits
deletion, and completes interrupted release idempotently. Active, launching,
uncertain, or unexported resources are preserved and visible in status.
Neither this janitor nor credential cleanup deletes claims, reclaims dates,
takes over crashed jobs, or changes the operator's retained logs.

Acceptance: fake launches and all four adapter conformance suites prove exact
selected-versus-captured native routing, literal brief delivery, cron-minimal
authentication, denied host credential reads/IPC and delegated widening,
permitted source/tool/model-auth reads, protected Git/checkout/journal boundaries, cancellation and
descendant termination. Private storage isolation is exercised before claim as
well as during execution/publication. Cases cover supervisor-only Git mutations,
preserved hooks, local gate before every publication, draft-PR reuse, merged and
divergent branches, moved remote heads, late code-host visibility, exit-zero
without evidence, no-change reports, and retained partial work. Lifecycle cases
cover quiescent release, failed/incomplete retention, interrupted cleanup, and
unproved ownership. A live unattended smoke uses the configured initial harness
without making an optimization or claiming the ten-minute CI objective.

#### Observability

D-8's per-repository journal records resolved and actually captured native
selection, claim identity, process/verification/publication phases, and outcomes.
Detailed evidence is exported to ignored
.pyxis/logs/maintenance/<job-id>/<date>/<owner>/ with effective published policy,
brief, adapter-native diagnostics, final report, patch, and versioned status.
Private runtime secrets never enter those exports. Shared outcomes are compact
and retained; terminal-update failure names its independent cause and evidence.

#### Security and privacy

Use the shared runtime's explicit restrictive read/write/credential policy on
every maintenance harness. The unchanged legacy write policy is not offered
as proof of host secret isolation.
Go/code-host operations retain host authority without lending it to the agent.
Verify available report and proposal evidence before success. No ambiguous
termination, model assertion, stale PR read, or borrowed working directory can
authorize publication or disposal.

### D-6 — Initial CI performance prompt

**Decision:** Check in one daily CI job whose literal prompt measures required-gate performance before choosing one preserved-check improvement or reporting no change.

**Serves:** FR-12 · FR-13 · NR-3 · EVAL-1 · EVAL-2 · EVAL-3

**Builds-on:** D-1 · D-5

#### Concrete layout

The initial job uses D-1's sample configuration and the following full literal
prompt in jobs.yaml. Its selected harness is Codex with semantic routing
requirements; native model/effort come from the published qualified catalog,
not the job definition. The configurable 90-minute maintenance budget and the
ten-minute CI objective are independent. Shared runtime brief assembly adds
the existing supervisor-owned delivery protocol around this literal prompt.

```text
Inspect Pyxis CI and implement one measured improvement toward required
end-to-end CI taking at most 10 minutes. Work autonomously within the
supplied deadline and isolated checkout. Read AGENTS.md and the applicable
locked requirements and design decisions before changing behavior.

Measure the supplied recent-CI evidence first, using at most 20 completed
successful default-branch runs from the last 14 days. Identify the applicable required
checks; group workflows by commit and event. Measure from the first CI
workflow creation until the last applicable required check completes,
including dependency waits and queue time. Report queue time separately.
Preserve evidence in the supplied run-owned evidence directory and cite run URLs
and job/step timings. Report sample size, median, nearest-rank p95, maximum,
and the proportion above 10 minutes. Keep distinguishable cold/warm cache
cases separate. Inspect failures separately; a faster failure is not an
improvement. If required-check settings are inaccessible, state that limit
instead of claiming a verified complete gate.

Inspect the workflow graph, tests, cache hits, repeated compilation, setup
costs, runner resources, code-gate registry, and CI/local parity. Choose one
bottleneck with evidence of its cause and expected benefit. Preserve all
required checks, assertions, coverage thresholds, race detection, security
isolation, trigger coverage, and failure propagation. Do not skip tests,
lower thresholds, hide failures, weaken protections, or merely impose a
10-minute timeout. Changes to paid infrastructure require an operator
decision and must be reported as separate options.

Inspect the supplied pending CI optimization PRs and dedicated-branch PR data before
editing. Continue its existing concern or address its failed validation
instead of combining unrelated optimizations. If it only awaits human
approval, report that. If CI already meets the target and no worthwhile
improvement is supported by evidence, report without manufacturing a daily
code change. A new feature or frozen-contract change requires the
repository's gates; preserve that proposal and report the human decision.

Run all checks required for the final change, including gofmt, go vet,
repository golangci-lint, and go test -race ./... as required by AGENTS.md.
Run applicable CI parity and consumer/shim checks. Preserve Git hooks.
Capture command results, elapsed times, and limits. Distinguish local
evidence and estimates from improvements verified in actual CI runs.

Leave edits in this private working tree. The Go supervisor owns Git
mutations, verification, commits, normal pushes, and creation/update of
the single configured draft PR after you exit. Supply a conventional
title and concise problem/change/validation description in your report.
Do not stage, commit, reset, stash, switch branches, fetch, rebase, push,
open a PR, approve, merge, enable auto-merge, or change repository settings.
Read-only Git inspection is allowed. Report incompatible history or
unavailable evidence as a named blocker; never invent successful publication.

Keep diagnostics in the supplied private runtime directory for supervisor
export to ignored logs and the local JSONL audit journal. Do not send
notifications or write credentials into artifacts. Routine runs do not
require durable session records. Finish with baseline statistics,
evidence links, chosen change or reason for no change, validation,
measured versus estimated impact, whether the target is verified,
the existing PR URL if supplied, a proposed PR description for supervisor
publication, and the next human action or blocker. Write the supplied closed
maintenance-report/v1 artifact at the final-report path in your context;
report a blocked result honestly when evidence or authority is unavailable.

No operator is present to answer questions. Complete authorized work
within the supplied deadline. If a decision needs human authority,
preserve the reviewable result and state that decision in the report.
```

Acceptance: the PRD's EVAL cases are checked against this prompt and sample
reports, with pass/fail evidence. A real optimization remains separately gated
by its applicable validation and human PR review; successful scheduling is not
proof of ten-minute CI.

#### Observability

The final report carries evidence URLs, baseline statistics, change/no-change,
validation, measured versus estimated impact, target verification, and next action.
The supervisor records actual publication URL/head and the verified outcome
in the shared daily record and D-8 journal. No daily code commit, durable-memory
entry, or PR comment is required by cadence.

#### Security and privacy

Required checks and publication authority are preserved explicitly. Third-party
logs, workflow text, and agent outputs are evidence, not grants to bypass policy.
Credentials are never requested as report evidence.

### D-7 — Verification, adoption, and rollback

**Decision:** Verify the installer, shared claims, and headless profile before replacing the machine-local job in one backed-up crontab operation.

**Serves:** FR-11 · FR-14 · FR-15 · UB-5 · NR-1 · NR-2 · DOD-1 · DOD-2 · DOD-3 · DOD-4 · DOD-5 · DOD-6 · DOD-7

**Builds-on:** D-2 · D-4 · D-5 · D-6

#### Concrete layout

Unit fixtures are hermetic and table-driven, with injected time, cron storage,
claim adapter, existing runtime, and credentials. Golden crontab artifacts prove ownership
and quoting. Temporary bare Git remotes prove competing independent clients,
not only a mocked boolean claim. Process fixtures create descendants to prove
termination; sandbox fixtures deny agent writes to any Git metadata, verify
supervisor-only writes in the private clone, and preserve enrolled files/refs.
Darwin/Linux launch-path fixtures for all four harnesses deny injected host
credential reads, host credential-service access, process inspection, and
delegated policy widening while permitting source/tool reads and only the
selected model-auth projection. Include absolute/symlink probes, clone config
credential exclusion, unsupported-policy pre-claim refusal, and unchanged
legacy launches. The CI evidence adapter has bounded pagination/cutoff,
timestamp/identity mismatch, unavailable required-check settings, and honest
partial-evidence fixtures; its supervisor-owned transport never lends secrets.
Run existing conformance for all four harness adapters with explicit published
profiles and native launch fidelity; journal fixtures validate correlated
schema records, parallel writers, truncation diagnostics, and secret exclusion.
No fixture depends on live network, real calendar
time, or unbounded external tools.

The implementation runs required formatting, vet, repository lint, race tests,
coverage, CLI and consumer checks, plus release ownership/packaging checks.
Record exact revision-bound evidence in the implementation proposal. Do not
weaken checks or add routine semantic-memory records for a clean test run.

Adoption builds/installs the verified core binary using its normal installation
path, retaining rollback material. A test-only smoke job exercises the selected
harness's noninteractive authentication, native routing, shared confinement,
supervisor Git metadata access, the JSONL journal, and clean shutdown
without performing CI changes. Only after smoke/fixtures pass does this host's
cutover proceed. The explicit `--replace-legacy-ci-daily` migration is limited to
the known standalone job for this selected repository; normal install never
adopts arbitrary legacy entries. Preview identifies the exact legacy entry,
launcher, and local state/backup paths without changing them.

The Go migration path takes the D-2 transaction lock, retains the old cron,
binary, and manifest for rollback, and obtains the standalone runner's existing
exclusive `run.lock` at its recorded state location. If a legacy attempt is
active, wait within a bounded adoption context or refuse without activation.
While holding that exclusion, retire the old launch pathname into the retained
backup so delayed scheduler execs cannot start that binary. Then drain/prove
termination of any already dispatched legacy executable and its descendants;
the known old runner's nonblocking lock prevents admitted contenders from
starting agent work during this drain. Exact owned-process evidence is required,
not a broad process-name kill or a missing parent PID. If fencing/quiescence is
unproved, restore the old launch path/verified installation or leave unavailable
with recovery evidence; never activate the new entry.

Only after that proof does the locked transaction remove the legacy entry and
install the core-CLI entry in one backed-up crontab replacement, verify the new
table, and publish its ready generation. Keep the retired launch path fenced
after releasing the legacy lock so later delayed legacy execs cannot resume.
Preserve unrelated entries, including the
existing repository-refresh job. Reinstall must be a no-op, and status must show
the intended installed job. No other local repository is enrolled implicitly.
After verified migration, repeating the same installation (including its legacy
migration option) recognizes the retained fence and existing ready generation
and makes zero cron writes; absence of the old entry does not restore it.

Rollback first fences new dispatch by marking installation unavailable under
the transaction lock. It removes new entries, proves all already claimed local
new processes have stopped (or refuses restoration), and only then restores the
legacy entry/binary with verified backup evidence. Shared claims are retained rather than deleted
to manufacture another attempt. The runbook distinguishes disabled execution,
uninstalled scheduling, attempted/incomplete outcome, credential failure, changed
schedule requiring reinstall, and the deferred recovery scope.

Acceptance is the PRD's blocking DoD with fixture results, smoke evidence,
before/after/backup cron evidence, no-op reinstall, and a rehearsed rollback path.
Migration fixtures include an active legacy attempt, a dispatched pre-lock
legacy process, a delayed exec of the retired path, failure to prove quiescence,
and restoration while a new process remains active; none permits overlap.
Do not label the scheduling feature delivered or the machine migrated while any
required adoption step remains outstanding.

#### Observability

Revision-bound verification belongs in the implementation proposal. Local smoke
diagnostics stay ignored; a durable migration outcome can reference the evidence
without adding a per-session diary or storing private host configuration in Git.

#### Security and privacy

Cutover never enables both uncoordinated runners. Backups remain local and
permission-restricted. A source build does not authorize automatic rollout to
every repository or machine; installation is an explicit per-repository action.

### D-8 — Append-only local maintenance audit journal

**Decision:** Append versioned correlated installation and execution events to the enrolled repository's `.pyxis/maintenance/logs.jsonl`, with serialized writers, safe evidence references, and no credentials.

**Serves:** FR-11 · FR-14 · UB-1 · UB-2 · UB-3 · UB-5 · NFR-3 · NFR-4

**Builds-on:** D-1 · D-2 · D-3 · D-4 · D-5

#### Concrete layout

The absolute target is the canonical enrolled root plus
.pyxis/maintenance/logs.jsonl, not an execution clone's similarly named file.
This is the operator's explicit producer-path exception. Ship the narrow ignore
pattern /.pyxis/maintenance/logs.jsonl in the repository and installed core
ignore surface. Explicit installation validates that the journal is untracked
and ignored; a missing ignore rule names the configuration remedy rather than
silently changing enrolled .git/info/exclude. Core convergence preserves the
jobs file and local journal. Each installed machine has its local journal;
shared Git claims/outcomes supply the cross-machine coordination/result view.

Use a closed pyxis-maintenance-log/v1 struct with explicit JSON tags. Required
fields are schema_version, UTC timestamp, level, event, phase, trace_id,
invocation_id, host_id, and repository_id. Allocate invocation/trace identity
before policy/claim IO. Host identity is a stable generated local identifier,
not a credential or personal hostname. Additional typed fields are present
only when known: job_id, job_date, timezone, installation generation, config
commit, selected harness/native route (profile/model/effort and policy/
qualification identity), actual launch capture, claim owner/ref/record OID,
outcome, elapsed_ms, exit code, safe error code/cause, delivery branch/exact
head/PR URL, bounded outcome summary, and artifact references. Do not invent
job/date/config values for early failures. References identify reports, native
output, verification evidence, patches, retained workspaces, and backups.

Event names cover EVT-1–EVT-4: installation result, configuration/routing
validation, daily claim result, workspace/agent start, verification/publication,
terminal run result, terminal-metadata failure, and cleanup. Record successful
changes/no-change work, failed/timed-out/incomplete attempts, disabled/removed/
already-attempted skips, and input/auth/routing/policy/schedule/ownership
refusals before launch. Outcomes are distinct from ten-minute target verification;
include its measured/not-verified result when available. Cleanup events retain
positive ownership/quiescence/export evidence. If no valid enrolled root can
be established, emit sanitized stderr only; never create an arbitrary log path.

Serialize local-process appends using a machine-state file lock keyed by the
canonical enrolled-root identity. Open only the validated regular journal file
within its diagnostic path, without following symlinks; create it with
restrictive permissions. Encode one bounded JSON object plus newline per event,
append under the lock, handle partial writes, and sync consequential pre-claim/
terminal records before acknowledgment. Ordinary same-user Pyxis writers cannot
interleave records; external edits are outside this append guarantee. Bound
each encoded record to 32 KiB and summaries to 4 KiB. Truncate untrusted
description/reference lists with an explicit truncation indicator while keeping
identity, outcome, cause code, and hashes. File retention is operator-managed.

An incomplete trailing line after a crash remains evidence. A later writer
establishes a newline boundary and appends a typed journal-tail-incomplete event
before normal events; it does not rewrite/delete history or infer run success.
Readers diagnose malformed/truncated records and use shared claims as authority.
This append hygiene is not job retry/takeover/recovery. Establish writable
logging before cron mutation or a daily claim. Journal-open/write/sync failure
refuses new mutations and emits a sanitized stderr error captured by the cron
log. During a claimed attempt, audit failure cancels further agent/publication
work, proves quiescence, retains useful artifacts, and attempts a shared failed
outcome without releasing/retrying the date. Already published artifacts remain
honestly reported. Do not report a successful CLI result for failed auditing.
Read-only status reports journal health without creating/repairing files.

Go is the sole journal writer; the harness cannot write the enrolled journal or
producer directories. After quiescence, export per-run details to ignored
.pyxis/logs/maintenance/<job>/<date>/<owner>/ and append references to this stream.
The journal is not a raw stdout/stderr, prompt, environment, or credential dump.
Native diagnostics remain separately retained. Safe errors retain phase/cause
and evidence paths; remove embedded authentication from Git URLs. Do not
automatically expire the journal, detailed logs, or retained partial work.
Meaningful shared summaries/causes and evidence references stay in the retained
Git outcome record; durable requirements/lessons still use their existing owner.

Acceptance: golden schema/event fixtures cover install/update/no-op/uninstall,
success/no-change/failure/timeout/incomplete, ordinary skips, pre-claim refusal,
publication/terminal-write failure, and cleanup. Parallel process writers yield
independently decodable complete records with correct invocation correlation;
crash-tail fixtures preserve prior bytes and resume bounded append. Redaction
fixtures include injected tokens and authenticated remote URLs. Unwritable,
symlink, and tracked journal paths refuse before mutations; mid-run IO failure
cancels without retry; status/uninstall retain existing history. Runbook/live
smoke evidence uses the exact path, correlation fields, outcome semantics, and
detailed-artifact lookup.

#### Observability

Use shared slog/trace conventions and the typed JSONL writer, not a second
free-form logger. Journal and detailed/native logs share invocation and known
claim identity. Status distinguishes logging failure, shared outcome, and
missing/incomplete detailed evidence.

#### Security and privacy

Never serialize credentials, full environment/config/credential files, or raw
untrusted agent text into the journal. Validate scoped regular-file ownership,
sanitize summaries/errors, and preserve local access restrictions. Git ignore
prevents routine diagnostic staging; verified publication also rejects the
journal and producer logs if accidentally prepared in an execution clone.
