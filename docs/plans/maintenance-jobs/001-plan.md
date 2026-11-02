# Daily repository maintenance jobs — plan 001

| Field | Value |
|---|---|
| **Feature** | maintenance-jobs |
| **PRD** | prd/maintenance-jobs |
| **Design** | design/maintenance-jobs |
| **Basis** | lock/maintenance-jobs/v1 |
| **Plan version** | 001 |
| **Dialect** | harbormaster/fs-spec:v2 |
| **Epic title** | Install and execute daily repository maintenance through the core CLI |
| **Status** | Draft — freezes at merge |
| **Template version** | plan-template/v2 |

## Tasks

### T1 — Discover core CLI commands through additive registration

| Field | Value |
|---|---|
| **Blocked by** | — |
| **Size band** | 120–220 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Requirements** | {"schema_version":"task-requirements/v1","implementation":{"assessment":{"reasoning":"complex","coupling":"composed","novelty":"adapt","verification":"composed","impact_scope":"system_wide","architectural_role":"system_core","citations":[{"id":"task:acceptance"},{"id":"design:maintenance-jobs#D-1"}]},"rationale":"Dispatch and help affect the whole installed CLI, although the new seam is additive and leaves legacy handlers intact."},"review":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"adapt","verification":"cross_boundary","impact_scope":"system_wide","architectural_role":"system_core","citations":[{"id":"task:acceptance"},{"id":"design:maintenance-jobs#D-1"}]},"rationale":"Review must prove discovery cannot shadow existing verbs or change refusal semantics across the core command surface."}} |
| **Pattern** | — |
| **Touches** | cmd/pyxis/main.go, cmd/pyxis/commands.go, cmd/pyxis/commands_test.go |

**Goal:** A new command file can supply a core CLI verb and its help entry without editing central dispatch again.

**Current behavior:** The core CLI enumerates verbs and help text in central dispatch.

**Desired behavior:** A typed descriptor registry runs before unchanged legacy dispatch, derives sorted help entries, and rejects duplicate or legacy-colliding keys.

**Scope:**
- Add the command descriptor and registration seam as the feature's prerequisite refactor.
- Preserve legacy dispatch, argument handling, exit codes, and help behavior.
- Prove discovery with a second independently registered fixture command.

**Out of scope:**
- Migrating existing verbs or adding maintenance behavior.

**Acceptance criteria:**
- [ ] A separately registered fixture verb dispatches and appears exactly once in sorted help without a central dispatch edit.
- [ ] Duplicate keys, empty descriptors, and legacy collisions fail registration with a named cause.
- [ ] Existing core verb and unknown-argument fixtures retain their observed behavior.

**Key interfaces:**
- Command descriptor key, usage, and handler contract; core dispatch and usage.

**Traces:** prd:maintenance-jobs/FR-1 · prd:maintenance-jobs/NR-2 · prd:maintenance-jobs/NFR-1 · design:maintenance-jobs#D-1

### T2 — Inspect closed daily job policy for an explicit repository

| Field | Value |
|---|---|
| **Blocked by** | T1 |
| **Size band** | 210–300 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Requirements** | {"schema_version":"task-requirements/v1","implementation":{"assessment":{"reasoning":"complex","coupling":"composed","novelty":"adapt","verification":"composed","impact_scope":"contained","architectural_role":"feature_local","citations":[{"id":"task:acceptance"},{"id":"design:maintenance-jobs#D-1"}]},"rationale":"Closed YAML, root canonicalization, and argument admission compose existing parsers into one inspectable feature-local behavior."},"review":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"adapt","verification":"composed","impact_scope":"multi_surface","architectural_role":"shared_substrate","citations":[{"id":"task:acceptance"},{"id":"design:maintenance-jobs#D-1"}]},"rationale":"Malformed policy must not reach authenticated or scheduling effects, and canonical identity must remain consistent across CLI consumers."}} |
| **Pattern** | — |
| **Touches** | cmd/pyxis/maintenance.go, internal/maintenance/policy.go, internal/maintenance/policy_test.go |

**Goal:** The installed CLI validates and inspects one repository's daily jobs independently of its own location or ambient working directory.

**Current behavior:** There is no core maintenance command or closed repository-owned maintenance policy parser.

**Desired behavior:** Maintenance command arguments and the versioned jobs document resolve an explicit canonical Git root and reject unsupported policy before cron, remote, or agent effects; status can show desired jobs and an uninstalled state.

**Scope:**
- Register maintenance dispatch and fail-closed install, run, status, and uninstall argument grammar; handlers remain unavailable until their behavior lands.
- Parse exactly one closed YAML document, validated source refs, unique IDs, enabled delivery-branch uniqueness, durations, and the four routing dimensions.
- Restrict cron syntax to one decimal minute and hour with daily wildcards; reject aliases, duplicate keys, extra documents, native argv, and command snippets.
- Resolve symlink aliases to one canonical root identity and expose desired-policy inspection.

**Out of scope:**
- Scheduler writes, published-policy fetches, native model catalogs, or installing entries.

**Acceptance criteria:**
- [ ] One compiled CLI inspects two fixture repositories from a hostile working directory without cross-repository writes; symlink aliases yield one root identity.
- [ ] Unknown fields, duplicate IDs or keys, unsupported schema, extra documents, aliases, invalid refs, invalid routing, and non-daily schedules produce named refusals and zero cron, remote, or agent mutations.
- [ ] Unknown flags, extra positionals, absent roots, invalid job IDs, and inappropriate verb flags refuse before effects.
- [ ] Valid empty and disabled-job documents inspect successfully; not-yet-implemented mutating handlers refuse explicitly.

**Key interfaces:**
- Jobs policy schema pyxis-maintenance-jobs/v1; source binding, Job, Schedule, Delivery, and ProfileMinimum; cliguard and canonical repository resolution.

**Traces:** prd:maintenance-jobs/FR-1 · prd:maintenance-jobs/FR-2 · prd:maintenance-jobs/FR-14 · prd:maintenance-jobs/FR-15 · prd:maintenance-jobs/UB-1 · prd:maintenance-jobs/NFR-1 · prd:maintenance-jobs/ABUSE-1 · design:maintenance-jobs#D-1

### T3 — Resolve explicit published profiles through the existing harness registry

| Field | Value |
|---|---|
| **Blocked by** | — |
| **Size band** | 170–270 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Requirements** | {"schema_version":"task-requirements/v1","implementation":{"assessment":{"reasoning":"complex","coupling":"cross_boundary","novelty":"adapt","verification":"composed","impact_scope":"multi_surface","architectural_role":"shared_substrate","citations":[{"id":"task:acceptance"},{"id":"design:maintenance-jobs#D-1"}]},"rationale":"The additive factory seam crosses the consumer registry and four adapters but reuses their existing catalog options."},"review":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"adapt","verification":"cross_boundary","impact_scope":"multi_surface","architectural_role":"shared_substrate","citations":[{"id":"task:acceptance"},{"id":"design:maintenance-jobs#D-1"}]},"rationale":"Review must detect ambient fallback or inconsistent registration that would change the route after policy capture."}} |
| **Pattern** | — |
| **Touches** | internal/executor/runtime/registry.go, internal/executor/runtime/registry_test.go, internal/adapters/harness/claude/claude.go, internal/adapters/harness/codex/codex.go, internal/adapters/harness/cursor/cursor.go, internal/adapters/harness/opencode/opencode.go |

**Goal:** Each registered harness can be constructed with supplied qualified profiles without reading an ambient repository.

**Current behavior:** Default registry factories discover harness profiles from the ambient repository even though adapters already accept WithProfiles.

**Desired behavior:** A typed configured-resolution path on the existing registry invokes each registration's configured constructor, uses its supplied catalog exclusively, and preserves legacy Resolve.

**Scope:**
- Add configured factory data to the existing derived registry rather than introducing another harness list.
- Register each of the four adapters' configured constructor using its existing profile option.
- Add registry-derived conformance for published catalog resolution and legacy behavior.

**Out of scope:**
- Changing the routing derivation policy, model roster, launch protocol, or default Resolve behavior.

**Acceptance criteria:**
- [ ] Claude, Codex, Cursor, and OpenCode resolve supplied implementation catalogs while conflicting ambient profiles are ignored.
- [ ] Missing or invalid supplied profiles refuse rather than falling back to defaults; existing Resolve fixtures remain unchanged.
- [ ] An additional fixture registration supports configured resolution without editing a central harness switch.

**Key interfaces:**
- Runtime Registry, configured constructor, WithProfiles, CatalogProvider, LaunchBuilder, and nativerouting.Resolver.

**Traces:** prd:maintenance-jobs/FR-10 · prd:maintenance-jobs/FR-15 · prd:maintenance-jobs/NR-2 · design:maintenance-jobs#D-1 · design:maintenance-jobs#D-5

### T4 — Append a credential-free correlated maintenance audit journal

| Field | Value |
|---|---|
| **Blocked by** | T2 |
| **Size band** | 240–300 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Requirements** | {"schema_version":"task-requirements/v1","implementation":{"assessment":{"reasoning":"difficult","coupling":"composed","novelty":"adapt","verification":"composed","impact_scope":"contained","architectural_role":"feature_local","citations":[{"id":"task:acceptance"},{"id":"design:maintenance-jobs#D-8"}]},"rationale":"Append locking, crash-tail preservation, bounds, and sync failures must agree before later consumers can admit effects."},"review":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"adapt","verification":"cross_boundary","impact_scope":"multi_surface","architectural_role":"shared_substrate","citations":[{"id":"task:acceptance"},{"id":"design:maintenance-jobs#D-8"}]},"rationale":"Audit corruption or leaked credentials affects every lifecycle path, so review must challenge process concurrency and unsafe file ownership."}} |
| **Pattern** | — |
| **Touches** | internal/maintenance/audit/, core-maintenance-ignore-surface, repository-maintenance-ignore-surface, internal/installer/helpers.go |

**Goal:** Maintenance lifecycle events append as bounded independently decodable records to the enrolled repository's ignored local journal.

**Current behavior:** Maintenance has no single versioned lifecycle stream or serialized audit admission.

**Desired behavior:** A typed JSONL writer allocates invocation and trace identity before IO, validates an ignored untracked regular journal, serializes appends, syncs consequential records, and preserves incomplete historical tails.

**Scope:**
- Implement the closed log record with optional known identity, route, claim, delivery, cause, timing, and artifact fields.
- Serialize same-user process writers by canonical repository identity; enforce restrictive permissions, non-symlink opening, partial-write handling, and consequential sync.
- Bound records to 32 KiB and summaries to 4 KiB, mark truncation, and sanitize credentials and authenticated URLs.
- Ship the narrow core and repository ignore rules; diagnose crash tails without rewriting history.

**Out of scope:**
- Raw output aggregation, journal expiration, job retry, or changing enrolled Git configuration.

**Acceptance criteria:**
- [ ] Parallel process writers produce complete correlated JSON records; early refusals retain unknown optional fields as absent and canary credentials never appear.
- [ ] Tracked, non-ignored, symlinked, unwritable, or unsyncable journal fixtures refuse new mutation admission and emit sanitized stderr.
- [ ] Oversized records retain identity, cause, outcome, and hashes with explicit truncation within the stated byte limits.
- [ ] A partial trailing line is preserved byte-for-byte, followed by a newline boundary and a typed tail diagnostic before subsequent events.

**Key interfaces:**
- pyxis-maintenance-log/v1, audit admission and append contract, shared slog/trace conventions, and canonical-root lock identity.

**Traces:** prd:maintenance-jobs/FR-11 · prd:maintenance-jobs/UB-1 · prd:maintenance-jobs/UB-2 · prd:maintenance-jobs/UB-5 · prd:maintenance-jobs/NFR-3 · prd:maintenance-jobs/NFR-4 · prd:maintenance-jobs/EVT-1 · prd:maintenance-jobs/EVT-2 · prd:maintenance-jobs/EVT-3 · prd:maintenance-jobs/EVT-4 · design:maintenance-jobs#D-8

### T5 — Render and reconcile only owned per-job cron entries

| Field | Value |
|---|---|
| **Blocked by** | T2 |
| **Size band** | 200–290 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Requirements** | {"schema_version":"task-requirements/v1","implementation":{"assessment":{"reasoning":"complex","coupling":"contained","novelty":"known","verification":"local","impact_scope":"contained","architectural_role":"feature_local","citations":[{"id":"task:acceptance"},{"id":"design:maintenance-jobs#D-2"}]},"rationale":"Rendering is pure but must compose ownership parsing, command quoting, and cron escaping without changing retained bytes."},"review":{"assessment":{"reasoning":"difficult","coupling":"composed","novelty":"adapt","verification":"composed","impact_scope":"multi_surface","architectural_role":"feature_local","citations":[{"id":"task:acceptance"},{"id":"design:maintenance-jobs#D-2"}]},"rationale":"Review must distinguish owned entries from arbitrary text and test cron command interpretation before installation can use the result."}} |
| **Pattern** | — |
| **Touches** | internal/maintenance/cron.go, internal/maintenance/cron_test.go, internal/maintenance/testdata/cron/ |

**Goal:** Desired daily jobs produce deterministic direct core-CLI entries while every unrelated crontab byte is preserved.

**Current behavior:** The standalone job has machine-local cron configuration and no repository-scoped reconciliation grammar.

**Desired behavior:** Pure rendering identifies complete owned marker-entry pairs, replaces only the selected root's set, and produces no change for identical desired entries.

**Scope:**
- Render one enabled-job entry with a literal job ID, canonical repository root, stable installed core executable, command-local tool search path, and early-failure redirection.
- Validate full ownership grammar and refuse malformed or duplicate owned records.
- Reconcile additions, in-place updates, disabled or removed jobs, and valid empty policy while retaining other entries and ordering.
- Implement POSIX argument quoting and separate cron percent escaping; reject newline and NUL inputs.

**Out of scope:**
- Native table writes, timezone-provider admission, or manifest authority.

**Acceptance criteria:**
- [ ] Golden install, update, removal, empty, and no-op fixtures produce exactly one direct invocation per enabled job and byte-identical unrelated and other-repository entries.
- [ ] Paths containing spaces, single quotes, and percent signs round-trip through the generated command and cron escaping.
- [ ] Malformed or duplicated ownership pairs refuse without producing a writable replacement table.
- [ ] An unstable build-cache executable, missing installed binary or tool, newline, or NUL produces a named admission defect.

**Key interfaces:**
- CronEntry ownership grammar, deterministic reconciliation result, installed executable binding, and tool-path binding.

**Traces:** prd:maintenance-jobs/FR-3 · prd:maintenance-jobs/FR-4 · prd:maintenance-jobs/FR-5 · prd:maintenance-jobs/NR-1 · prd:maintenance-jobs/ABUSE-1 · design:maintenance-jobs#D-2

### T6 — Admit compatible native cron semantics without changing globals

| Field | Value |
|---|---|
| **Blocked by** | T5 |
| **Size band** | 170–260 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Requirements** | {"schema_version":"task-requirements/v1","implementation":{"assessment":{"reasoning":"difficult","coupling":"composed","novelty":"adapt","verification":"composed","impact_scope":"contained","architectural_role":"feature_local","citations":[{"id":"task:acceptance"},{"id":"design:maintenance-jobs#D-2"}]},"rationale":"Provider-specific admission must evaluate retained environment in entry order instead of assuming portable timezone semantics."},"review":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"adapt","verification":"cross_boundary","impact_scope":"multi_surface","architectural_role":"shared_substrate","citations":[{"id":"task:acceptance"},{"id":"design:maintenance-jobs#D-2"}]},"rationale":"Incorrect native semantics can schedule authenticated work at the wrong time or inherit arbitrary startup behavior."}} |
| **Pattern** | — |
| **Touches** | internal/maintenance/cronport.go, internal/adapters/cron/native/ |

**Goal:** Installation accepts only a known native scheduler whose effective environment supports each configured daily timezone.

**Current behavior:** There is no maintenance-owned cron adapter or provider-specific scheduler admission.

**Desired behavior:** A consumer-owned cancellation-aware adapter distinguishes missing crontab from provider failure and validates system timezone and effective directives at each proposed entry position.

**Scope:**
- Provide typed read and install operations with supported Darwin and Linux provider identification.
- Interpret retained timezone, shell, shell-startup, and random-delay directives in order according to the identified provider.
- Require compatible IANA timezone and supported POSIX shell; reject unknown semantics and conflicting inherited directives.
- Keep global directives unchanged and expose actionable admission causes.

**Out of scope:**
- Foreign-timezone scheduling, catch-up, wake services, or adding global cron environment settings.

**Acceptance criteria:**
- [ ] An absent user table is distinguished from permission and provider failures; supported adapter fixtures honor context cancellation.
- [ ] Matching host timezone alone cannot admit a conflicting preceding effective scheduler timezone, including an in-place entry update.
- [ ] Unsupported shell, nonempty startup injection, random-delay settings, mismatched timezone, and unknown provider semantics refuse before writes.
- [ ] Supported Darwin/Linux fixtures leave all retained globals unchanged and delegate DST dispatch to native cron.

**Key interfaces:**
- Consumer cron port, native provider profile, ordered effective environment, and scheduler admission verdict.

**Traces:** prd:maintenance-jobs/FR-3 · prd:maintenance-jobs/FR-4 · prd:maintenance-jobs/UB-4 · prd:maintenance-jobs/NR-1 · prd:maintenance-jobs/NFR-2 · design:maintenance-jobs#D-2

### T7 — Publish serialized installation authority as a local manifest

| Field | Value |
|---|---|
| **Blocked by** | T2, T4 |
| **Size band** | 190–280 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Requirements** | {"schema_version":"task-requirements/v1","implementation":{"assessment":{"reasoning":"difficult","coupling":"composed","novelty":"adapt","verification":"composed","impact_scope":"contained","architectural_role":"feature_local","citations":[{"id":"task:acceptance"},{"id":"design:maintenance-jobs#D-1"}]},"rationale":"Crash-visible manifest state and serialized ready proofs must prevent partial local installation from becoming execution authority."},"review":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"adapt","verification":"cross_boundary","impact_scope":"multi_surface","architectural_role":"shared_substrate","citations":[{"id":"task:acceptance"},{"id":"design:maintenance-jobs#D-1"}]},"rationale":"Review must challenge generation races and backup restoration because later authenticated claims rely on this state."}} |
| **Pattern** | — |
| **Touches** | internal/maintenance/installation.go, internal/maintenance/installation_test.go |

**Goal:** A repository has one permission-restricted ready installation generation that executions can prove under a machine lock.

**Current behavior:** No derived manifest binds source authority, installed schedule fingerprints, and host executable locations.

**Desired behavior:** Closed machine-local state records ready, installing, or unavailable generations without copying prompts or runner policy; atomic transitions and verified backups share one per-user transaction lock.

**Scope:**
- Define the versioned manifest, source binding, normalized job schedule fingerprints, and canonical-root state identity.
- Implement per-user serialized mutation and execution-authority locking with cancellation.
- Publish atomic restricted manifests and backup references, distinguishing absent, installing, and unavailable state.
- Expose a ready-generation proof contract held through later claim ownership proof.

**Out of scope:**
- Writing cron, fetching policy, or retrying failed job dates.

**Acceptance criteria:**
- [ ] Concurrent process fixtures cannot publish interleaved authority transitions for the same user; cancellation releases owned locks.
- [ ] Manifest fixtures reject unknown keys and invalid states and contain no prompt, native catalog, or credential values.
- [ ] Interrupted publication never exposes a partial ready generation; absent, installing, and unavailable state names installation as the remedy.
- [ ] A ready proof reports exact generation, source binding, and schedule fingerprints and detects replacement under the transaction protocol.

**Key interfaces:**
- InstallationManifest, InstallationGeneration, transaction lock, atomic manifest transition, and ReadyBinding proof.

**Traces:** prd:maintenance-jobs/FR-1 · prd:maintenance-jobs/FR-8 · prd:maintenance-jobs/UB-4 · prd:maintenance-jobs/UB-5 · prd:maintenance-jobs/NFR-3 · design:maintenance-jobs#D-1 · design:maintenance-jobs#D-2 · design:maintenance-jobs#D-3

### T8 — Install and uninstall cron entries through a verified transaction

| Field | Value |
|---|---|
| **Blocked by** | T4, T5, T6, T7 |
| **Size band** | 240–300 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Requirements** | {"schema_version":"task-requirements/v1","implementation":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"adapt","verification":"cross_boundary","impact_scope":"multi_surface","architectural_role":"feature_local","citations":[{"id":"task:acceptance"},{"id":"design:maintenance-jobs#D-2"}]},"rationale":"A paired cron/manifest transaction crosses file state, scheduler IO, and audit admission with verified rollback."},"review":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"adapt","verification":"cross_boundary","impact_scope":"multi_surface","architectural_role":"shared_substrate","citations":[{"id":"task:acceptance"},{"id":"design:maintenance-jobs#D-2"}]},"rationale":"Review must prove no intermediate authority is usable and failure cannot overwrite another repository's installation."}} |
| **Pattern** | — |
| **Touches** | internal/maintenance/install.go, internal/maintenance/install_test.go, cmd/pyxis/maintenance_install.go |

**Goal:** Explicit install and uninstall commit a verified crontab and manifest pair while unchanged installation performs zero cron writes.

**Current behavior:** Policy rendering and manifest authority are separate and no core command commits their paired state.

**Desired behavior:** Under the machine lock, the CLI admits logging, computes all entries, rechecks the table with bounded retries, backs up state, marks installing, replaces cron, verifies it, and publishes ready or verified rollback.

**Scope:**
- Wire preview, install, and policy-independent uninstall with audit admission before mutations.
- Validate journal and host bindings, retain restricted backups, and detect intervening reads before the write.
- Commit the table and manifest protocol with bounded provider calls and verified failure recovery.
- Audit installation results and leave uncertain recovery unavailable with backup evidence.

**Out of scope:**
- Legacy-job migration or changing any unowned entry.

**Acceptance criteria:**
- [ ] Repeated identical installation produces zero cron provider writes; previews produce zero external mutations and describe the complete proposed write set.
- [ ] Disabled, removed, and empty jobs reconcile; uninstall works without jobs policy and preserves other repositories, unrelated bytes, and journal history.
- [ ] Injected write, verification, and concurrent-change failures retain backups and restore a verified prior pair or leave explicit unavailable state; none reports success.
- [ ] A paused installing generation followed by failed cron replacement admits zero claims or launches from an old queued invocation.

**Key interfaces:**
- Install, Uninstall, Preview, cron adapter, manifest transaction, audit admission, and EVT-1 result.

**Traces:** prd:maintenance-jobs/FR-3 · prd:maintenance-jobs/FR-4 · prd:maintenance-jobs/FR-5 · prd:maintenance-jobs/FR-11 · prd:maintenance-jobs/FR-14 · prd:maintenance-jobs/UB-5 · prd:maintenance-jobs/NR-1 · prd:maintenance-jobs/EVT-1 · design:maintenance-jobs#D-2 · design:maintenance-jobs#D-8

### T9 — Capture published policy and one immutable daily identity in private storage

| Field | Value |
|---|---|
| **Blocked by** | T2, T3, T4, T7 |
| **Size band** | 240–300 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Requirements** | {"schema_version":"task-requirements/v1","implementation":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"adapt","verification":"composed","impact_scope":"multi_surface","architectural_role":"feature_local","citations":[{"id":"task:acceptance"},{"id":"design:maintenance-jobs#D-3"}]},"rationale":"Source authority, exact Git reads, installed fingerprints, and injected time must agree in one immutable invocation context."},"review":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"adapt","verification":"cross_boundary","impact_scope":"multi_surface","architectural_role":"shared_substrate","citations":[{"id":"task:acceptance"},{"id":"design:maintenance-jobs#D-3"}]},"rationale":"Review must rule out stale generation, ambient policy, date recomputation, and any mutation of the enrolled Git common directory."}} |
| **Pattern** | — |
| **Touches** | internal/maintenance/published.go, internal/maintenance/published_test.go, internal/maintenance/resources.go |

**Goal:** An invocation reads one trusted published policy commit and keeps its original configured-timezone date without touching enrolled Git storage.

**Current behavior:** Maintenance has no private control repository, published-policy capture, or retained invocation-date context.

**Desired behavior:** Invocation time is captured before blocking work; a ready source binding fetches jobs and harness catalogs into positively owned control storage, validates one commit and schedule fingerprint, and freezes the route request and daily key.

**Scope:**
- Create an owned control repository outside enrolled and common Git storage, with an ownership journal from preparation onward.
- Read jobs and harness catalogs at one exact fetched commit without local overlays or ambient fallback.
- Validate published source binding and installed normalized schedule; skip removed or disabled jobs and refuse drift before claiming.
- Derive job/date identity from captured start time, retaining it across midnight and later reads; expose locked-generation reproof.

**Out of scope:**
- Minute eligibility, startup grace, local-policy fallback, claiming a date, or execution cloning.

**Acceptance criteria:**
- [ ] Dirty enrolled source, untracked files, branch, refs, index, FETCH_HEAD, and configuration snapshots remain identical after policy reads and failed fetches.
- [ ] Published prompt, timeout, route, and disable edits apply; unmerged edits supply no instructions and source or schedule drift refuses before claim.
- [ ] Different launch hours and manual same-date starts derive one key; timezone boundaries, midnight during fetch, and repeated DST invocations retain the captured date while the next date differs.
- [ ] Generation replacement requires a bounded fresh policy read; a stale fetched generation cannot authorize subsequent claim proof.

**Key interfaces:**
- PublishedPolicySnapshot, InvocationIdentity, configured ProfileMinimum and catalog capture, control-resource ownership journal, and ReadyBinding reproof.

**Traces:** prd:maintenance-jobs/FR-6 · prd:maintenance-jobs/FR-7 · prd:maintenance-jobs/FR-8 · prd:maintenance-jobs/FR-15 · prd:maintenance-jobs/UB-2 · prd:maintenance-jobs/UB-4 · prd:maintenance-jobs/NR-2 · design:maintenance-jobs#D-3

### T10 — Claim retained daily attempts with exact positive remote ownership proof

| Field | Value |
|---|---|
| **Blocked by** | T4, T7, T9 |
| **Size band** | 240–300 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Requirements** | {"schema_version":"task-requirements/v1","implementation":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"adapt","verification":"cross_boundary","impact_scope":"multi_surface","architectural_role":"feature_local","citations":[{"id":"task:acceptance"},{"id":"design:maintenance-jobs#D-4"}]},"rationale":"CAS reuse still requires exact remote proof, retained daily identity, and serialized installation authority across independent clients."},"review":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"adapt","verification":"cross_boundary","impact_scope":"system_wide","architectural_role":"shared_substrate","citations":[{"id":"task:acceptance"},{"id":"design:maintenance-jobs#D-4"}]},"rationale":"A mistaken ownership verdict can duplicate authenticated agent work on every installed host, making adversarial transport evidence necessary."}} |
| **Pattern** | — |
| **Touches** | internal/maintenance/claimport.go, internal/maintenance/claim.go, internal/adapters/claim/gitref/ |

**Goal:** Competing clients authorize at most one owner for a job's configured-timezone date and preserve that attempted date permanently.

**Current behavior:** Existing issue leases are temporary and there is no maintenance daily record on the shared remote.

**Desired behavior:** A consumer-owned claim adapter reuses gitrefstore from control storage, inserts a closed record by exact lease, and authorizes launch only after an exact winning-record read matches nonce, identity, policy, and route.

**Scope:**
- Define versioned daily records and immutable identity fields without prompts, diffs, secrets, or private host paths.
- Implement CAS insertion and bounded reread/replay using the existing store, holding the ready-generation proof through ownership confirmation.
- Use unpredictable owner nonces and positive proof after conflict or uncertain push; existing dates skip.
- Provide same-owner terminal transitions while preserving retained claim authority and independently eligible next dates.

**Out of scope:**
- TTL, deletion, lease takeover, automatic retry, or ordinary code-branch publication.

**Acceptance criteria:**
- [ ] Independent temporary bare-remote clients at different same-date hours prove exactly one owner; every loser skips and no unproved owner launches.
- [ ] Absent-ref leases, contention, uncertain pushes, malformed state, and mismatched immutable fields exercise fail-closed proof without enrolled Git mutations.
- [ ] Success, failure, timeout, interrupted startup, and configuration or harness changes preserve the consumed date; a new date can be claimed.
- [ ] Only the proved owner can publish a terminal transition; terminal-write failure retains the claim and cannot authorize another attempt.

**Key interfaces:**
- ClaimStore, daily record and ownership proof, crypto/rand nonce, gitrefstore ApplyResult and exact snapshot, and EVT-2.

**Traces:** prd:maintenance-jobs/FR-6 · prd:maintenance-jobs/FR-7 · prd:maintenance-jobs/FR-11 · prd:maintenance-jobs/UB-2 · prd:maintenance-jobs/UB-3 · prd:maintenance-jobs/ABUSE-2 · prd:maintenance-jobs/NFR-4 · prd:maintenance-jobs/EVT-2 · design:maintenance-jobs#D-4

### T11 — Carry an optional host-owned read and credential policy through shared runtime

| Field | Value |
|---|---|
| **Blocked by** | — |
| **Size band** | 220–300 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Requirements** | {"schema_version":"task-requirements/v1","implementation":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"first_of_kind","verification":"cross_boundary","impact_scope":"multi_surface","architectural_role":"shared_substrate","citations":[{"id":"task:acceptance"},{"id":"design:maintenance-jobs#D-5"}]},"rationale":"The optional read boundary extends a shared write policy while preserving legacy launch semantics and explicit path ownership."},"review":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"first_of_kind","verification":"cross_boundary","impact_scope":"system_wide","architectural_role":"system_core","citations":[{"id":"task:acceptance"},{"id":"design:maintenance-jobs#D-5"}]},"rationale":"A widened read or credential grant exposes host authority to all adapters; review must inspect both negative grants and legacy compatibility."}} |
| **Pattern** | — |
| **Touches** | internal/executor/port/agentruntime.go, internal/executor/runtime/access.go, internal/executor/runtime/access_other.go, internal/executor/runtime/access_test.go |

**Goal:** The existing launch boundary can carry and validate a restrictive host-owned process policy without changing launches that omit it.

**Current behavior:** Shared process confinement primarily restricts writes and LaunchSpec has no host-owned read or credential policy.

**Desired behavior:** An additive ProcessAccessPolicy is carried by LaunchSpec and CommandAccess, validates positive read grants and ownership, and refuses unsupported enforcement rather than widening them.

**Scope:**
- Define typed optional read, credential-service, descriptor, and process-inspection authority within the existing boundary.
- Resolve and pin allowed source, runtime, verified tool/dependency, and enumerated public OS paths; reject overbroad or private-data-overlapping grants.
- Preserve current write controls and legacy behavior when no restrictive policy is supplied.
- Expose policy compilation and enforcement capability refusal for registered launch prerequisites.

**Out of scope:**
- OS-specific policy execution, jobs YAML controls, new launchers, or changing qualified native flags.

**Acceptance criteria:**
- [ ] A restrictive LaunchSpec reaches CommandAccess with identical authority; ordinary launches without the new policy retain existing behavior.
- [ ] Root-wide, whole-home, credential-directory, unsafe tool-parent, symlink escape, and overlapping private-data grants are refused.
- [ ] Unsupported restrictive enforcement produces a named refusal instead of a legacy write-only fallback.
- [ ] Compiled policy permits only explicitly selected source, runtime, tool, and public OS resources while retaining protected Git and enrolled-write boundaries.

**Key interfaces:**
- ProcessAccessPolicy, LaunchSpec, CommandAccess, LaunchAccess, confined path ownership, and enforcement capability.

**Traces:** prd:maintenance-jobs/FR-9 · prd:maintenance-jobs/FR-10 · prd:maintenance-jobs/FR-15 · prd:maintenance-jobs/NR-2 · prd:maintenance-jobs/NFR-3 · design:maintenance-jobs#D-5 · design:maintenance-jobs#D-7

### T12 — Enforce restrictive Darwin process access through the existing wrapper

| Field | Value |
|---|---|
| **Blocked by** | T11 |
| **Size band** | 230–300 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Requirements** | {"schema_version":"task-requirements/v1","implementation":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"adapt","verification":"cross_boundary","impact_scope":"multi_surface","architectural_role":"shared_substrate","citations":[{"id":"task:acceptance"},{"id":"design:maintenance-jobs#D-5"}]},"rationale":"Native Darwin access rules must combine explicit reads, retained writes, credential IPC, and executable dependencies."},"review":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"first_of_kind","verification":"cross_boundary","impact_scope":"system_wide","architectural_role":"system_core","citations":[{"id":"task:acceptance"},{"id":"design:maintenance-jobs#D-5"}]},"rationale":"Negative native probes must establish denial without sacrificing permitted model authentication or silently restoring host access."}} |
| **Pattern** | — |
| **Touches** | internal/executor/runtime/access_darwin.go, internal/executor/runtime/access_darwin_test.go |

**Goal:** A restrictive Darwin launch reads only positively granted resources and cannot reach host credentials or process secrets.

**Current behavior:** The Darwin wrapper's legacy policy does not prove host credential-read isolation.

**Desired behavior:** The existing wrapper compiles default-denied file reads with explicit grants, retained write controls, selected model network access, scoped IPC, and denied host credential services and inspection.

**Scope:**
- Compile the restrictive host policy in the existing Darwin wrapper without changing legacy profiles.
- Deny absolute and symlinked host-secret reads, unrelated runtime/control reads, host credential IPC, and host-process inspection.
- Restrict inherited descriptors while allowing required native tools, selected model authentication, network traffic, and run-local IPC.
- Add positive and negative native process probes using injected secret fixtures.

**Out of scope:**
- Linux mount views, model inference, or broad allowances to make unsupported logins work.

**Acceptance criteria:**
- [ ] Native Darwin probes permit granted source, tool, public runtime, and selected private-auth reads and permitted model network or run-local IPC.
- [ ] Injected host Git/code-host secrets, other runs, control state, credential-service access, and process inspection remain denied through absolute and symlink aliases.
- [ ] Agent writes to protected Git metadata, enrolled state, and journal remain denied.
- [ ] Legacy launches keep their prior wrapper behavior; an unsupported restriction fails before model launch.

**Key interfaces:**
- Existing Darwin confinement wrapper and seatbelt profile derived from ProcessAccessPolicy.

**Traces:** prd:maintenance-jobs/FR-9 · prd:maintenance-jobs/FR-10 · prd:maintenance-jobs/FR-15 · prd:maintenance-jobs/NR-2 · prd:maintenance-jobs/NFR-2 · prd:maintenance-jobs/NFR-3 · design:maintenance-jobs#D-5 · design:maintenance-jobs#D-7

### T13 — Enforce restrictive Linux access with a private allowlisted mount view

| Field | Value |
|---|---|
| **Blocked by** | T11 |
| **Size band** | 230–300 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Requirements** | {"schema_version":"task-requirements/v1","implementation":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"adapt","verification":"cross_boundary","impact_scope":"multi_surface","architectural_role":"shared_substrate","citations":[{"id":"task:acceptance"},{"id":"design:maintenance-jobs#D-5"}]},"rationale":"A private mount/process view must satisfy real CLI dependencies without exposing the host's read-only root."},"review":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"first_of_kind","verification":"cross_boundary","impact_scope":"system_wide","architectural_role":"system_core","citations":[{"id":"task:acceptance"},{"id":"design:maintenance-jobs#D-5"}]},"rationale":"Review must test host-root, symlink, process, and IPC escape paths with native positive and negative probes."}} |
| **Pattern** | — |
| **Touches** | internal/executor/runtime/access_linux.go, internal/executor/runtime/access_linux_test.go |

**Goal:** A restrictive Linux launch sees an allowlisted private filesystem and process view instead of the host root.

**Current behavior:** The Linux write-confinement wrapper exposes broad host reads through a read-only root view.

**Desired behavior:** The same wrapper builds a private mount view from pinned allowed resources, private temporary, device, and process views, and restricted IPC/descriptors while preserving permitted model traffic.

**Scope:**
- Implement restrictive allowlist mount construction and private temporary, device, and process views in the existing wrapper.
- Exclude host root, credential files, supervisor/control state, enrolled Git settings, host credential brokers, and host-process environments.
- Retain explicit write protections, selected model authentication, verified executable dependencies, and run-local IPC.
- Refuse unavailable native enforcement and preserve legacy wrapper behavior without the optional policy.

**Out of scope:**
- Darwin policy or installing privileged host sandbox infrastructure automatically.

**Acceptance criteria:**
- [ ] Native Linux probes read only granted source, tool, public runtime, and selected private-auth resources; no read-only host-root bind exposes secrets.
- [ ] Absolute and symlinked canary-secret reads, host credential IPC, descriptors, and host-process inspection fail while permitted model traffic and local IPC work.
- [ ] Protected Git, enrolled checkout, journal, and other-run writes remain denied.
- [ ] Unavailable enforcement refuses a restrictive launch and ordinary launches retain their existing behavior.

**Key interfaces:**
- Existing Linux confinement wrapper, pinned allowlist mount view, and ProcessAccessPolicy.

**Traces:** prd:maintenance-jobs/FR-9 · prd:maintenance-jobs/FR-10 · prd:maintenance-jobs/FR-15 · prd:maintenance-jobs/NR-2 · prd:maintenance-jobs/NFR-2 · prd:maintenance-jobs/NFR-3 · design:maintenance-jobs#D-5 · design:maintenance-jobs#D-7

### T14 — Prevent delegated launches from omitting or widening ancestor authority

| Field | Value |
|---|---|
| **Blocked by** | T11, T12, T13 |
| **Size band** | 190–280 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Requirements** | {"schema_version":"task-requirements/v1","implementation":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"adapt","verification":"cross_boundary","impact_scope":"multi_surface","architectural_role":"shared_substrate","citations":[{"id":"task:acceptance"},{"id":"design:maintenance-jobs#D-5"}]},"rationale":"The launch wire and broker must preserve restrictive authority through descendants and existing legacy capability semantics."},"review":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"first_of_kind","verification":"cross_boundary","impact_scope":"system_wide","architectural_role":"system_core","citations":[{"id":"task:acceptance"},{"id":"design:maintenance-jobs#D-5"}]},"rationale":"Omission and widening are privileged escape paths, so review requires adversarial nested-process evidence on both platforms."}} |
| **Pattern** | — |
| **Touches** | internal/executor/runtime/launch_scope.go, internal/executor/runtime/launch_proxy.go, internal/executor/runtime/launch_broker.go, internal/executor/runtime/launch_nested_test.go |

**Goal:** Authenticated delegated children can only retain or narrow their parent's read, write, and credential authority.

**Current behavior:** Delegated launch wire data and narrowing checks carry the existing write-oriented boundary.

**Desired behavior:** Versioned launch paths and capability checks carry optional process policy and reject child omission or widening; the scoped broker remains the sole permitted delegation channel.

**Scope:**
- Extend existing closed delegated-launch data additively with the restrictive process policy.
- Check descendant grants against pinned ancestor authority, including reads, writes, IPC, credentials, and inspection.
- Carry enforcement through the existing broker and shared wrapping path rather than allowing a child to invoke an unconfined wrapper.
- Preserve legacy delegated launches and cancellation ownership.

**Out of scope:**
- A new broker, arbitrary provider IPC, or widening authority for nested tools.

**Acceptance criteria:**
- [ ] Same-authority and narrower children execute permitted fixture work through the existing authenticated broker.
- [ ] Omitted policy, added read roots, wider writes, credential-service access, and process-inspection grants are refused before child execution.
- [ ] Absolute and symlinked nested secret reads remain denied under both supported OS enforcement paths.
- [ ] Unknown wire keys or schema, forged capability, and stale grant fixtures refuse; legacy delegated-launch tests retain their behavior.

**Key interfaces:**
- launch capability, versioned launchPaths and launchRequest, narrowLaunchPaths, and the existing scoped launch broker.

**Traces:** prd:maintenance-jobs/FR-9 · prd:maintenance-jobs/FR-10 · prd:maintenance-jobs/FR-15 · prd:maintenance-jobs/NR-2 · prd:maintenance-jobs/NFR-3 · design:maintenance-jobs#D-5 · design:maintenance-jobs#D-7

### T15 — Project only selected model authentication into private runtime state

| Field | Value |
|---|---|
| **Blocked by** | T3 |
| **Size band** | 210–290 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Requirements** | {"schema_version":"task-requirements/v1","implementation":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"adapt","verification":"cross_boundary","impact_scope":"multi_surface","architectural_role":"shared_substrate","citations":[{"id":"task:acceptance"},{"id":"design:maintenance-jobs#D-5"}]},"rationale":"Provider-specific projection reuses host acquisition but must separate model authentication from every other host credential."},"review":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"adapt","verification":"cross_boundary","impact_scope":"system_wide","architectural_role":"system_core","citations":[{"id":"task:acceptance"},{"id":"design:maintenance-jobs#D-5"}]},"rationale":"Review must challenge multi-provider stores, mutable configuration inheritance, and cleanup before descendant termination."}} |
| **Pattern** | — |
| **Touches** | internal/executor/runtime/authentication.go, internal/executor/runtime/authentication_test.go, internal/executor/reviewinvoke/credentials_test.go |

**Goal:** A selected harness authenticates from a private model-only projection without inheriting mutable host settings or unrelated credentials.

**Current behavior:** Review authentication snapshots exist, but their review-specific helper and provider stores do not establish the maintenance restriction.

**Desired behavior:** A shared helper reuses current authentication acquisition, selects only the resolved provider, writes restricted private file-store state, and leaves supervisor Git/code-host credentials outside the harness grant.

**Scope:**
- Factor authentication projection into the shared runtime while preserving the existing review helper contract.
- Select required credentials for the resolved harness/provider instead of copying a multi-provider store wholesale.
- Snapshot required Keychain credentials before confinement; sanitize environment and exclude settings, hooks, MCP configuration, and helpers.
- Provide explicit unsupported-login refusal and secret lifecycle ownership.

**Out of scope:**
- Adding credentials to jobs policy, cron text, reports, or diagnostic exports.

**Acceptance criteria:**
- [ ] Four-harness fixtures authenticate using only selected private model state under a cron-style environment without shell startup.
- [ ] Other provider tokens, Git/code-host secrets, mutable host settings, hooks, MCP configuration, and credential helpers are absent from projections and serialized diagnostics.
- [ ] Keychain-backed supported credentials are acquired by the supervisor before confinement; a login requiring later host-service access refuses.
- [ ] Existing review-authentication fixtures retain their behavior, and credential cleanup requires positive process-tree quiescence.

**Key interfaces:**
- Shared model authentication preparation, existing PrepareReviewAuthentication compatibility contract, selected provider identity, and sanitized child environment.

**Traces:** prd:maintenance-jobs/FR-10 · prd:maintenance-jobs/FR-15 · prd:maintenance-jobs/UB-2 · prd:maintenance-jobs/NR-2 · prd:maintenance-jobs/NFR-3 · design:maintenance-jobs#D-5 · design:maintenance-jobs#D-7

### T16 — Prove the selected harness's prerequisites before consuming a date

| Field | Value |
|---|---|
| **Blocked by** | T3, T9, T12, T13, T14, T15 |
| **Size band** | 230–300 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Requirements** | {"schema_version":"task-requirements/v1","implementation":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"adapt","verification":"cross_boundary","impact_scope":"multi_surface","architectural_role":"feature_local","citations":[{"id":"task:acceptance"},{"id":"design:maintenance-jobs#D-1"}]},"rationale":"Preflight joins published catalogs, four adapters, authentication projection, and real OS enforcement without consuming shared authority."},"review":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"adapt","verification":"cross_boundary","impact_scope":"system_wide","architectural_role":"shared_substrate","citations":[{"id":"task:acceptance"},{"id":"design:maintenance-jobs#D-1"}]},"rationale":"Review must prove negative prerequisites precede claim and that probe wrapping is identical to the path later enforcing model execution."}} |
| **Pattern** | — |
| **Touches** | internal/maintenance/preflight.go, internal/maintenance/preflight_test.go, internal/executor/runtime/maintenance_conformance_test.go, cmd/pyxis/maintenance_runtime.go, internal/executor/runtime/registry.go |

**Goal:** Unsupported route, authentication, or process access refuses before daily claiming through the same shared path used by native launch.

**Current behavior:** The native adapters expose routing and authentication contracts but there is no maintenance joined pre-claim prerequisite proof.

**Desired behavior:** A bounded registry-derived preflight freezes the selected qualified route and proves native launch capture, private authentication, and restrictive access on a control-owned minimal fixture without inference.

**Scope:**
- Resolve normal-mode implementation requirements from the captured published catalog and retain complete selected profile and qualification identity.
- Use existing AuthCheck, LaunchBuilder, native capture/fidelity, and registered prerequisites without another harness factory list or launcher.
- Probe permitted source, tool, and model-authentication reads and denied secret reads, credential services, inspection, and delegated widening under shared confinement.
- Bound route, authentication, and probe preparation independently of the claimed-attempt deadline and audit refusal.

**Out of scope:**
- Model inference, an execution checkout before claim, fallback harnesses, or consuming an unsupported route's date.

**Acceptance criteria:**
- [ ] Each of the four adapters records one selection matching native model, effort, and controls under explicit published profiles.
- [ ] Darwin/Linux selected-adapter fixtures pass required read/authentication probes and deny absolute or symlinked host secrets, credential IPC, inspection, and child policy widening.
- [ ] Missing tools or credentials, unsupported route, failed confinement probe, and preflight timeout produce zero claim mutations and zero model launches.
- [ ] Conflicting ambient catalogs and interactive shell-startup settings cannot alter the selected route, sanitized environment, or prerequisite proof.

**Key interfaces:**
- Frozen native selection, CatalogProvider, LaunchBuilder, launch fidelity capture, AuthCheck, and shared confinement prerequisite proof.

**Traces:** prd:maintenance-jobs/FR-10 · prd:maintenance-jobs/FR-15 · prd:maintenance-jobs/UB-2 · prd:maintenance-jobs/NFR-2 · prd:maintenance-jobs/NFR-3 · prd:maintenance-jobs/DOD-7 · design:maintenance-jobs#D-1 · design:maintenance-jobs#D-3 · design:maintenance-jobs#D-5 · design:maintenance-jobs#D-7

### T17 — Read bounded historical CI timing through the existing code-host adapter

| Field | Value |
|---|---|
| **Blocked by** | T2 |
| **Size band** | 230–300 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Requirements** | {"schema_version":"task-requirements/v1","implementation":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"adapt","verification":"composed","impact_scope":"multi_surface","architectural_role":"feature_local","citations":[{"id":"task:acceptance"},{"id":"design:maintenance-jobs#D-1"}]},"rationale":"Historical timing adds a bounded optional reader while retaining native transport and separating measurement limitations from success evidence."},"review":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"adapt","verification":"cross_boundary","impact_scope":"multi_surface","architectural_role":"shared_substrate","citations":[{"id":"task:acceptance"},{"id":"design:maintenance-jobs#D-1"}]},"rationale":"Review must catch incorrect pagination, mismatched run identity, missing gate settings, or credentials reaching artifacts."}} |
| **Pattern** | — |
| **Touches** | internal/maintenance/evidence.go, internal/adapters/codehost/github/maintenance_ci.go, internal/adapters/codehost/github/maintenance_ci_test.go |

**Goal:** The supervisor obtains typed successful-run timing evidence through a bounded optional capability on the existing code-host axis.

**Current behavior:** Current check and failed-log readers do not provide successful historical workflow, job, and step timing.

**Desired behavior:** A maintenance-owned EvidenceReader uses the existing authenticated code-host transport to capture typed identities and timestamps, explicit availability limits, and bounded pagination without lending agent credentials.

**Scope:**
- Define the closed maintenance-ci-evidence/v1 result and optional consumer capability with repository/base, window, fetch time, settings limits, and workflow, run, job, and step evidence.
- Implement historical reads on the existing native code-host adapter and reuse its authentication, budget, and transport.
- Enforce injected deadline, at most 200 candidate workflow runs, and at most 10 MiB retained evidence; report cutoff and completeness limits.
- Validate repository, commit, event, attempts, and timestamps rather than combining unmatched or missing evidence.

**Out of scope:**
- An agent-side API client, a second code-host client, or treating existing aggregate checks as complete historical evidence.

**Acceptance criteria:**
- [ ] Hermetic transport fixtures produce typed successful workflow, job, and step timing with identities, timestamps, and source URLs.
- [ ] Pagination, deadline, candidate count, and byte cutoffs remain within the stated bounds and mark evidence incomplete.
- [ ] Missing required-check settings, inaccessible steps, identity mismatches, or invalid timestamps retain named limits rather than a complete-gate claim.
- [ ] Injected transport secrets never appear in the result, errors, or retained artifacts; unsupported adapter capability is reported explicitly.

**Key interfaces:**
- EvidenceReader, maintenance-ci-evidence/v1, optional historical capability, and existing code-host authenticated transport.

**Traces:** prd:maintenance-jobs/FR-12 · prd:maintenance-jobs/FR-13 · prd:maintenance-jobs/NR-3 · prd:maintenance-jobs/NFR-3 · prd:maintenance-jobs/EVAL-2 · design:maintenance-jobs#D-1 · design:maintenance-jobs#D-5 · design:maintenance-jobs#D-6 · design:maintenance-jobs#D-7

### T18 — Supply matched recent CI and pending-proposal evidence to one attempt

| Field | Value |
|---|---|
| **Blocked by** | T9, T17 |
| **Size band** | 190–280 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Requirements** | {"schema_version":"task-requirements/v1","implementation":{"assessment":{"reasoning":"difficult","coupling":"composed","novelty":"adapt","verification":"composed","impact_scope":"contained","architectural_role":"feature_local","citations":[{"id":"task:acceptance"},{"id":"design:maintenance-jobs#D-5"}]},"rationale":"Sample assembly must preserve identities and explicit limits while joining historical evidence and current proposal context."},"review":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"adapt","verification":"cross_boundary","impact_scope":"multi_surface","architectural_role":"feature_local","citations":[{"id":"task:acceptance"},{"id":"design:maintenance-jobs#D-5"}]},"rationale":"Review must challenge apparently successful but incomplete gates, mixed attempts, and stale dedicated-branch observations."}} |
| **Pattern** | — |
| **Touches** | internal/maintenance/evidence.go, internal/maintenance/evidence_snapshot.go, internal/maintenance/evidence_snapshot_test.go |

**Goal:** The attempt receives one bounded honest baseline and current proposal context before choosing a CI change.

**Current behavior:** A historical reader alone does not assemble the initial job's default-branch sample or pending dedicated-branch context.

**Desired behavior:** The supervisor retains at most twenty completed successful default-branch runs from fourteen days, separates non-success evidence, groups only matched commit, event, and check data, and captures pending proposals and the exact dedicated head.

**Scope:**
- Assemble the captured-repository/base sample and required-check settings or explicit accessibility limit.
- Retain source timestamps and queue/dependency evidence, distinguish attempts and cold/warm cases, and preserve failed or cancelled samples separately.
- Reuse existing proposal/check readers to supply pending optimization PRs and exact dedicated-branch state.
- Write bounded secret-free run-owned evidence for the brief without asserting a target from partial data.

**Out of scope:**
- Choosing an optimization, changing required checks, or creating tracker tasks.

**Acceptance criteria:**
- [ ] Complete and partial fixture samples respect twenty successful runs and fourteen days while retaining separate failed/cancelled evidence and reader cutoff limits.
- [ ] Mixed commit, event, attempt, and check fixtures cannot form a falsely complete gate or omit queue/dependency waits from available evidence.
- [ ] The snapshot identifies exact pending dedicated-branch PR/head and separate pending optimization proposals through existing readers.
- [ ] Unavailable settings or timestamps and empty samples remain named limitations in both retained evidence and assembled context.

**Key interfaces:**
- EvidenceSnapshot, matched gate sample, pending-proposal observation, dedicated-head binding, and EvidenceReader.

**Traces:** prd:maintenance-jobs/FR-12 · prd:maintenance-jobs/FR-13 · prd:maintenance-jobs/NR-3 · prd:maintenance-jobs/NFR-3 · prd:maintenance-jobs/EVAL-2 · prd:maintenance-jobs/EVAL-3 · design:maintenance-jobs#D-5 · design:maintenance-jobs#D-6

### T19 — Prepare a proved private independent execution clone for the winning owner

| Field | Value |
|---|---|
| **Blocked by** | T9, T10, T15, T16, T18 |
| **Size band** | 230–300 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Requirements** | {"schema_version":"task-requirements/v1","implementation":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"adapt","verification":"cross_boundary","impact_scope":"multi_surface","architectural_role":"feature_local","citations":[{"id":"task:acceptance"},{"id":"design:maintenance-jobs#D-5"}]},"rationale":"An independent clone requires compatible remote ancestry, credential-free settings, and exact resource/access proofs after claim."},"review":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"adapt","verification":"cross_boundary","impact_scope":"system_wide","architectural_role":"shared_substrate","citations":[{"id":"task:acceptance"},{"id":"design:maintenance-jobs#D-5"}]},"rationale":"Review must detect shared Git directories, supervisor credential exposure, or false resource ownership before any untrusted edit begins."}} |
| **Pattern** | — |
| **Touches** | internal/maintenance/workspace.go, internal/maintenance/workspace_test.go, internal/maintenance/resources.go |

**Goal:** A positively proved winner works in an independent clone whose source, Git metadata, runtime, and caches cannot overlap enrolled or other-run storage.

**Current behavior:** The maintenance control repository and claim exist, but no isolated execution checkout has been bound to the owner.

**Desired behavior:** After ownership proof, the supervisor creates and journals private resources, selects the compatible dedicated branch baseline, generates credential-free clone configuration, and proves the exact execution access boundary.

**Scope:**
- Create independent clone and distinct Git metadata, runtime state, caches, and authentication resources keyed to the winning identity.
- Prove non-overlap with enrolled Git common storage, control/supervisor credentials, and other attempts before launch.
- Handle absent dedicated branch and already-merged ancestor heads using normal ancestry; retain blockers for incompatible history.
- Bind restrictive reads/writes to the actual clone, ensure credential-free local Git settings, and repeat execution-path probes after claim.

**Out of scope:**
- Linked worktrees, agent Git mutations, branch publication, or automatic rebase of incompatible history.

**Acceptance criteria:**
- [ ] Winning-attempt fixtures have distinct work and Git metadata; enrolled and other-run source, untracked files, index, refs, FETCH_HEAD, and settings remain unchanged.
- [ ] Overlap, unsafe tool roots, clone configuration with userinfo, authorization headers, helpers, or secret includes refuses launch and consumes the already-claimed date.
- [ ] Absent branch and ancestor-of-base fixtures select the intended baseline; incompatible history retains a named local blocker without force operations.
- [ ] Actual-clone probes permit intended edits and source, tool, and model-authentication reads, deny protected Git writes and enrolled-checkout, journal, and control access, and bind all resources to a versioned ownership record.

**Key interfaces:**
- OwnedWorkspace, resource ownership journal, dedicated branch baseline, credential-free clone settings, and exact-clone ProcessAccessPolicy proof.

**Traces:** prd:maintenance-jobs/FR-8 · prd:maintenance-jobs/FR-9 · prd:maintenance-jobs/FR-12 · prd:maintenance-jobs/UB-2 · prd:maintenance-jobs/UB-6 · prd:maintenance-jobs/NR-2 · prd:maintenance-jobs/NFR-3 · prd:maintenance-jobs/DOD-7 · design:maintenance-jobs#D-5

### T20 — Run one native harness within the claimed attempt's entire deadline

| Field | Value |
|---|---|
| **Blocked by** | T10, T16, T19 |
| **Size band** | 240–300 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Requirements** | {"schema_version":"task-requirements/v1","implementation":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"adapt","verification":"cross_boundary","impact_scope":"multi_surface","architectural_role":"feature_local","citations":[{"id":"task:acceptance"},{"id":"design:maintenance-jobs#D-3"}]},"rationale":"The lifecycle joins authority, native launch fidelity, setup deadlines, and descendant termination without using the fleet's retry machinery."},"review":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"adapt","verification":"cross_boundary","impact_scope":"system_wide","architectural_role":"shared_substrate","citations":[{"id":"task:acceptance"},{"id":"design:maintenance-jobs#D-3"}]},"rationale":"Review must prove every branch after claim preserves one launch and that a parent exit cannot authorize publication or credential cleanup."}} |
| **Pattern** | — |
| **Touches** | internal/maintenance/run.go, internal/maintenance/run_test.go, cmd/pyxis/maintenance_run.go |

**Goal:** One proved daily owner launches its frozen native selection through AgentRuntime and bounds the whole claimed attempt without retry.

**Current behavior:** Preflight and the private clone are available but maintenance has no bounded single-attempt lifecycle.

**Desired behavior:** The supervisor reuses implementation brief assembly and native runtime methods, starts a fresh conversation, and carries one claimed-attempt deadline through setup, launch, verification, publication, and evidence work.

**Scope:**
- Wire run admission, policy capture, generation reproof, preflight, positive claim proof, and private workspace preparation in the required order.
- Construct an implementation-purpose LaunchSpec with frozen native model/effort, literal prompt, private runtime, dedicated branch, and nil Session without a fake tracker task.
- Supply immutable invocation, baseline, evidence, deadline, prior compact outcome, and final-report context through existing brief assembly.
- Use bounded cleanup after cancellation and positive whole-process-tree Await/Kill proof before export, publication, or disposal.

**Out of scope:**
- Recovery loops, continuation, fallback harnesses, another launch after a claimed date, or agent publication.

**Acceptance criteria:**
- [ ] Four-adapter fake-native fixtures receive the selected model/effort and literal prompt through existing implementation protocol, private Workdir/RuntimeStateDir, and a fresh nil session.
- [ ] Injected deadlines cover clone preparation and all subsequent claimed work; cancellation terminates descendants through the existing runtime contract.
- [ ] Direct-parent exit without proved tree quiescence preserves claimed/incomplete state and performs zero publication, exports requiring quiescence, or disposal.
- [ ] Every failure path preserves the consumed date and launches at most once; no synthetic tracker task, issue lease, interactive shell startup, or new launcher is used.

**Key interfaces:**
- AgentRuntime AuthCheck, Launch, Await, and Kill, LaunchSpec, AssembleBriefContext, claimed-attempt deadline, process-tree quiescence proof, and EVT-4 phases.

**Traces:** prd:maintenance-jobs/FR-6 · prd:maintenance-jobs/FR-7 · prd:maintenance-jobs/FR-9 · prd:maintenance-jobs/FR-10 · prd:maintenance-jobs/FR-15 · prd:maintenance-jobs/UB-3 · prd:maintenance-jobs/NFR-4 · prd:maintenance-jobs/EVT-4 · design:maintenance-jobs#D-3 · design:maintenance-jobs#D-4 · design:maintenance-jobs#D-5

### T21 — Validate final reports and retain useful quiescent partial work

| Field | Value |
|---|---|
| **Blocked by** | T4, T19, T20 |
| **Size band** | 230–300 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Requirements** | {"schema_version":"task-requirements/v1","implementation":{"assessment":{"reasoning":"difficult","coupling":"composed","novelty":"adapt","verification":"composed","impact_scope":"contained","architectural_role":"feature_local","citations":[{"id":"task:acceptance"},{"id":"design:maintenance-jobs#D-4"}]},"rationale":"Closed report validation and partial-work export must remain truthful across changed, no-change, blocked, and failed runs."},"review":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"adapt","verification":"cross_boundary","impact_scope":"multi_surface","architectural_role":"shared_substrate","citations":[{"id":"task:acceptance"},{"id":"design:maintenance-jobs#D-4"}]},"rationale":"Untrusted reports and paths can forge outcomes or expose secrets, so review must compare artifacts to actual quiescent clone state."}} |
| **Pattern** | — |
| **Touches** | internal/maintenance/report.go, internal/maintenance/artifacts.go, internal/maintenance/artifacts_test.go |

**Goal:** Only a valid retained report and proved workspace evidence can support success, while ordinary failures retain available work honestly.

**Current behavior:** Native process output is telemetry and there is no maintenance report validator or safe artifact export.

**Desired behavior:** After proved quiescence, bounded regular-file readers validate closed report identity and evidence, retain tracked, untracked, and binary changes and a patch, and export safe diagnostics without private authentication state.

**Scope:**
- Define and validate maintenance-report/v1 with changed, no-change, or blocked result, summary, evidence, validation, proposed PR text, and typed optional CI measurements.
- Reject unknown fields, identity mismatch, escaping report references, missing evidence, incomplete runtime results, and blocked success claims.
- Capture actual changed or unchanged clone state and available tracked, untracked, and binary work under explicit supervisor-owned paths.
- Export retained report, safe native diagnostics, verification evidence, patch, and status with audit correlation; exclude credential state and ignored operational logs from code payloads.

**Out of scope:**
- Trusting model prose or exit zero as success, publication, or disposing failed/unexported resources.

**Acceptance criteria:**
- [ ] Exit zero with missing, invalid, mismatched, blocked, or incomplete report is a named failed result; no-change requires a retained honest report and observable unchanged clone.
- [ ] Unknown keys, symlink/path escape, oversized artifact, and fabricated evidence fixtures cannot authorize success or expose supervisor files.
- [ ] Quiescent failure and timeout fixtures retain available tracked, untracked, and binary changes plus patch and safe report/output references, or explicitly report absence when preparation failed.
- [ ] Canary credentials, private authentication stores, journal, and producer logs are absent from exported code artifacts and serialized diagnostics.

**Key interfaces:**
- maintenance-report/v1, bounded scoped artifact readers, verified report outcome, supervisor path staging, and correlated export manifest.

**Traces:** prd:maintenance-jobs/FR-7 · prd:maintenance-jobs/FR-11 · prd:maintenance-jobs/FR-12 · prd:maintenance-jobs/UB-3 · prd:maintenance-jobs/NFR-3 · prd:maintenance-jobs/NFR-4 · prd:maintenance-jobs/EVT-3 · design:maintenance-jobs#D-4 · design:maintenance-jobs#D-5 · design:maintenance-jobs#D-8

### T22 — Verify and publish one dedicated draft PR under supervisor authority

| Field | Value |
|---|---|
| **Blocked by** | T18, T20, T21 |
| **Size band** | 240–300 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Requirements** | {"schema_version":"task-requirements/v1","implementation":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"adapt","verification":"cross_boundary","impact_scope":"multi_surface","architectural_role":"feature_local","citations":[{"id":"task:acceptance"},{"id":"design:maintenance-jobs#D-5"}]},"rationale":"Publication composes hooks, validation, expected remote ancestry, normal Git push, and exact proposal reads under one supervisor."},"review":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"adapt","verification":"cross_boundary","impact_scope":"system_wide","architectural_role":"system_core","citations":[{"id":"task:acceptance"},{"id":"design:maintenance-jobs#D-5"}]},"rationale":"A mistaken publication or authority decision bypasses factory controls, so review must test both remote movement and mechanically denied agent writes."}} |
| **Pattern** | — |
| **Touches** | internal/maintenance/delivery.go, internal/maintenance/delivery_test.go, cmd/pyxis/maintenance_delivery.go |

**Goal:** Quiescent verified changes advance the dedicated branch normally and create or continue one exact-head draft PR for human review.

**Current behavior:** The agent can leave edits and a report, but no maintenance supervisor owns gated Git publication and proposal verification.

**Desired behavior:** The supervisor installs normal hooks, stages explicit paths, runs the shared required gate, commits and non-forced pushes only from the compatible captured head, and verifies current remote code and draft proposal state.

**Scope:**
- Reuse supervisor Git, normal hook setup, shared verification, and existing code-host proposal readers/publication through command composition.
- Reject diagnostic or credential payloads and run required validation before every code publication.
- Compare the captured expected dedicated head immediately before normal push; retain changes on divergence and verify exact pushed head despite delayed code-host visibility.
- Create or reuse the configured draft PR with conventional title and report-backed description; support valid no-change without code or PR mutation.

**Out of scope:**
- Agent staging, commits, and publication, forced pushes, approval, merge, auto-merge, repository settings, or weakening validation.

**Acceptance criteria:**
- [ ] Agent Git writes and proposal publication are mechanically denied; only the Go supervisor stages explicit code paths, commits, and pushes after proved quiescence and a passing shared gate.
- [ ] Hook failure, failed verification, incompatible or moved remote head, and push rejection preserve a local proposal and perform zero force-pushes or successful-outcome claims.
- [ ] Existing draft, merged ancestor branch, approval-only pending proposal, and late-visibility fixtures obey dedicated-branch policy and prove the exact resulting remote head and draft PR.
- [ ] A verified unchanged clone and honest report performs zero commit, push, or PR mutations; no path approves, merges, enables auto-merge, or changes protections.

**Key interfaces:**
- Shared required verification path, supervisor Git operations, normal hook setup, existing code-host proposal capability, exact remote-head proof, and verified DeliveryResult.

**Traces:** prd:maintenance-jobs/FR-9 · prd:maintenance-jobs/FR-12 · prd:maintenance-jobs/UB-6 · prd:maintenance-jobs/NR-2 · prd:maintenance-jobs/NR-3 · prd:maintenance-jobs/ABUSE-3 · prd:maintenance-jobs/EVAL-3 · design:maintenance-jobs#D-5 · design:maintenance-jobs#D-6

### T23 — Record truthful terminal outcomes and stop on audit failure

| Field | Value |
|---|---|
| **Blocked by** | T8, T10, T20, T21, T22 |
| **Size band** | 220–300 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Requirements** | {"schema_version":"task-requirements/v1","implementation":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"adapt","verification":"cross_boundary","impact_scope":"multi_surface","architectural_role":"feature_local","citations":[{"id":"task:acceptance"},{"id":"design:maintenance-jobs#D-4"}]},"rationale":"Terminal truth requires joining publication and report proof with independent shared-state and local-audit failures."},"review":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"adapt","verification":"cross_boundary","impact_scope":"system_wide","architectural_role":"shared_substrate","citations":[{"id":"task:acceptance"},{"id":"design:maintenance-jobs#D-4"}]},"rationale":"Review must challenge success after auditing loss or partial publication and prove that metadata failures cannot release a consumed date."}} |
| **Pattern** | — |
| **Touches** | internal/maintenance/outcome.go, internal/maintenance/outcome_test.go, internal/maintenance/run.go, internal/maintenance/audit/ |

**Goal:** Every admitted invocation has a truthful correlated result, and auditing failure cannot permit further agent or publication work.

**Current behavior:** Separate launch, export, and delivery results do not yet close shared daily state or enforce mid-run journal failure.

**Desired behavior:** Verified outcomes update only the owner's retained claim; local terminal records distinguish success, failure, timeout, skip, refusal, and incomplete authority, while logging failures cancel work and retain evidence without retry.

**Scope:**
- Join report, actual launch, verification, delivery, timing, cause, and artifact observations into typed lifecycle and terminal events.
- Require evidence for success, keep timeout classified as timed-out, and retain incomplete authority when process-tree termination is unproved.
- Treat journal open, write, or sync failure before mutation as refusal; during a claim cancel further work, prove quiescence, retain useful artifacts, and attempt a shared failed result.
- Record terminal-metadata failure independently and never erase, release, or retry the date.

**Out of scope:**
- Inferring ten-minute target success from execution success or repairing failed daily authority.

**Acceptance criteria:**
- [ ] Success, no-change, failure, timeout, ordinary skip, pre-claim refusal, and incomplete fixtures yield distinct correlated journal/shared states with only available fields.
- [ ] A missing or unproved report/delivery cannot become success; a timeout remains timed-out even when useful artifacts were retained.
- [ ] Injected mid-run audit failure cancels further model/publication effects, attempts shared failure after quiescence, preserves already published facts, and never returns successful CLI status.
- [ ] Terminal CAS/update failure is diagnosed locally with safe evidence while the claim remains consumed; no second launch is authorized.

**Key interfaces:**
- Verified terminal outcome, same-owner ClaimStore transition, audit failure cancellation, EVT-2, EVT-3, and EVT-4, and bounded cleanup context.

**Traces:** prd:maintenance-jobs/FR-7 · prd:maintenance-jobs/FR-11 · prd:maintenance-jobs/FR-12 · prd:maintenance-jobs/UB-1 · prd:maintenance-jobs/UB-2 · prd:maintenance-jobs/UB-3 · prd:maintenance-jobs/NFR-3 · prd:maintenance-jobs/NFR-4 · prd:maintenance-jobs/EVT-2 · prd:maintenance-jobs/EVT-3 · prd:maintenance-jobs/EVT-4 · design:maintenance-jobs#D-4 · design:maintenance-jobs#D-5 · design:maintenance-jobs#D-8

### T24 — Inspect drift, shared outcomes, and persistent refusal counters without writes

| Field | Value |
|---|---|
| **Blocked by** | T2, T8, T23 |
| **Size band** | 220–300 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Requirements** | {"schema_version":"task-requirements/v1","implementation":{"assessment":{"reasoning":"complex","coupling":"composed","novelty":"adapt","verification":"composed","impact_scope":"contained","architectural_role":"feature_local","citations":[{"id":"task:acceptance"},{"id":"design:maintenance-jobs#D-1"}]},"rationale":"The status view composes existing observations and typed counters while keeping all inspection paths mutation-free."},"review":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"adapt","verification":"cross_boundary","impact_scope":"multi_surface","architectural_role":"feature_local","citations":[{"id":"task:acceptance"},{"id":"design:maintenance-jobs#D-1"}]},"rationale":"Review must detect counter double-counting, hidden status repairs, and missing evidence displayed as a successful attempted occurrence."}} |
| **Pattern** | — |
| **Touches** | internal/maintenance/status.go, internal/maintenance/counters.go, internal/maintenance/status_test.go, cmd/pyxis/maintenance.go |

**Goal:** Read-only status explains desired versus installed jobs, shared attempted outcomes, retained resources, journal health, and repeated safety/dependency refusals.

**Current behavior:** Status can inspect desired policy but cannot correlate installed entries, shared dates, local evidence, or persistent failure counts.

**Desired behavior:** The CLI distinguishes absent and drifting installation from executed, skipped, failed, timed-out, and incomplete dates, and exposes typed local counters without treating missing evidence as success.

**Scope:**
- Read actual owned cron entries, manifest state, latest shared record, journal health, and resource/evidence availability without repair.
- Persist one refusal-counter increment per invocation with lifetime/consecutive counts and first/last timestamps keyed by repository, job when known, and cause.
- Reset only consecutive malfunction count after a validated coordination decision; ordinary disabled, removed, and lost-claim skips are not malfunctions.
- Render actionable remedies for install state, schedule drift, authentication/routing refusal, log damage, and retained incomplete work.

**Out of scope:**
- Notifications, retries, journal-tail repair during status, or resetting attempted dates.

**Acceptance criteria:**
- [ ] Read-only status fixtures make zero local, cron, remote, or agent mutations and distinguish installation drift, no attempt, success, failure, timeout, and unterminated claim.
- [ ] Missing artifacts, malformed journal lines, and terminal-metadata failure remain explicit limits rather than inferred success.
- [ ] Repeated safety/dependency refusals increment once per invocation; ordinary skips do not increment, validated coordination resets consecutive count, and lifetime totals remain.
- [ ] Counter-write failure is audited without granting launch, and status reports preserved unexported or uncertain resources.

**Key interfaces:**
- MaintenanceStatus, closed refusal-counter state, desired/actual installation comparison, journal reader, latest shared outcome, and resource ownership view.

**Traces:** prd:maintenance-jobs/FR-11 · prd:maintenance-jobs/FR-14 · prd:maintenance-jobs/UB-2 · prd:maintenance-jobs/UB-4 · prd:maintenance-jobs/NFR-4 · prd:maintenance-jobs/DOD-6 · design:maintenance-jobs#D-1 · design:maintenance-jobs#D-3 · design:maintenance-jobs#D-4 · design:maintenance-jobs#D-5 · design:maintenance-jobs#D-8

### T25 — Release only exported quiescent resources through a bounded Go janitor

| Field | Value |
|---|---|
| **Blocked by** | T8, T9, T19, T21, T23, T24 |
| **Size band** | 220–290 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Requirements** | {"schema_version":"task-requirements/v1","implementation":{"assessment":{"reasoning":"difficult","coupling":"composed","novelty":"adapt","verification":"composed","impact_scope":"contained","architectural_role":"feature_local","citations":[{"id":"task:acceptance"},{"id":"design:maintenance-jobs#D-3"}]},"rationale":"Owned cleanup must distinguish released exported resources from partial work and interrupted transitions without becoming crash recovery."},"review":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"adapt","verification":"cross_boundary","impact_scope":"multi_surface","architectural_role":"shared_substrate","citations":[{"id":"task:acceptance"},{"id":"design:maintenance-jobs#D-3"}]},"rationale":"Review must prove cleanup cannot reap an active tree, borrowed directory, credentials still in use, or retained cross-machine authority."}} |
| **Pattern** | — |
| **Touches** | internal/maintenance/resources.go, internal/maintenance/janitor.go, internal/maintenance/janitor_test.go, internal/maintenance/install.go, internal/maintenance/run.go |

**Goal:** Explicitly releasable owned resources are cleaned idempotently while failed, active, uncertain, or unexported work remains inspectable.

**Current behavior:** Ownership journals retain control and execution resources but no maintenance cleanup transition safely completes interrupted release.

**Desired behavior:** A same-change bounded Go janitor runs during preparation, acts only on release-pending resources with verified exports and positive ownership/quiescence, and audits every deletion without touching shared authority or operator logs.

**Scope:**
- Define owned lifecycle and release eligibility for control repositories, clones, caches, credential state, and scratch artifacts.
- Require verified exports and positive whole-process-tree quiescence before disposable workspace or credential cleanup.
- Run one bounded preparation pass and complete interrupted releases idempotently.
- Retain partial work, unknown ownership, active or launching resources, and all claims and operator-managed logs.

**Out of scope:**
- Automatic expiry, crash recovery, takeover, releasing a job date, or deleting operator diagnostics.

**Acceptance criteria:**
- [ ] Quiescent exported release-pending fixtures are deleted only within positively proved owned roots and produce correlated cleanup evidence.
- [ ] Active, launching, failed, incomplete, uncertain, unexported, and foreign resources remain unchanged and visible in status.
- [ ] Interrupted cleanup resumes idempotently within the bounded pass; cancellation leaves a truthful retained lifecycle.
- [ ] Credential cleanup never precedes process-tree proof and no cleanup deletes shared claims, journal history, or detailed operator logs.

**Key interfaces:**
- ResourceOwnershipJournal, release eligibility proof, verified export reference, positive quiescence, and bounded Janitor preparation pass.

**Traces:** prd:maintenance-jobs/FR-9 · prd:maintenance-jobs/FR-11 · prd:maintenance-jobs/FR-14 · prd:maintenance-jobs/UB-3 · prd:maintenance-jobs/NFR-3 · prd:maintenance-jobs/NFR-4 · prd:maintenance-jobs/EVT-4 · design:maintenance-jobs#D-3 · design:maintenance-jobs#D-5 · design:maintenance-jobs#D-8

### T26 — Ship the daily CI job and evaluate its measurement and proposal policy

| Field | Value |
|---|---|
| **Blocked by** | T2, T18, T21, T22, T23 |
| **Size band** | 170–270 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Requirements** | {"schema_version":"task-requirements/v1","implementation":{"assessment":{"reasoning":"complex","coupling":"composed","novelty":"adapt","verification":"composed","impact_scope":"multi_surface","architectural_role":"feature_local","citations":[{"id":"task:acceptance"},{"id":"design:maintenance-jobs#D-1"}]},"rationale":"The initial job, representative report cases, and core payload ownership must preserve the locked prompt while avoiding default installation into other repositories."},"review":{"assessment":{"reasoning":"difficult","coupling":"composed","novelty":"adapt","verification":"composed","impact_scope":"multi_surface","architectural_role":"feature_local","citations":[{"id":"task:acceptance"},{"id":"design:maintenance-jobs#D-1"}]},"rationale":"Review must compare locked prompt bytes, challenge measurement and publication claims, and prove core upgrades preserve repository-owned jobs."}} |
| **Pattern** | — |
| **Touches** | repository-maintenance-jobs-resource, internal/maintenance/eval/, internal/maintenance/testdata/ci/, internal/release/stage.go, internal/release/stage_test.go, release-maintenance-ownership-policy |

**Goal:** The reviewed repository job schedules daily CI investigation with the locked prompt and yields rubric-checked honest reports rather than artificial daily edits.

**Current behavior:** The standalone prompt is machine-local and the core jobs resource and representative report evaluations are absent.

**Desired behavior:** The repository job uses 02:00 America/Los_Angeles, a 90-minute attempt budget, the contracted Codex semantic request, and dedicated draft delivery; the locked literal prompt is checked against all named eval cases.

**Scope:**
- Check in the complete D-6 prompt and D-1 initial settings without vendor model names; preserve repository ownership and exclude source-specific jobs and local journal from default release payloads.
- Author complete/incomplete timing, cache, failed/cancelled, mixed-identity, paid-runner, weakened-check, and authority proposal cases.
- Evaluate pending-PR continuation, approval-only, already-within-target, blocked evidence, and no-worthwhile-change cases.
- Retain per-case pass/fail report rubric evidence with baseline sample statistics, limits, measured versus estimated benefit, target verification, and next action.

**Out of scope:**
- Guaranteeing ten-minute CI, performing an optimization during fixture evaluation, or silently changing the locked prompt to follow adjacent workflow changes.

**Acceptance criteria:**
- [ ] The jobs document validates with exact contracted cadence, timezone, timeout, route dimensions, dedicated branch, and full D-6 prompt; core payload/convergence fixtures never ship default CI jobs or overwrite reviewed policy.
- [ ] EVAL-1 accepts zero cases that weaken required checks, thresholds, failure propagation, protections, spending authority, or human approval/merge.
- [ ] EVAL-2 reports sample size, median, nearest-rank p95, maximum, proportion over ten minutes, queue time, and cold/warm distinction when evidenced; incomplete, failed, cancelled, or local-only evidence cannot verify the target.
- [ ] EVAL-3 continues one concern or explicitly reports approval-only, no-change, or blocker without duplicate proposals or unrelated daily changes; every representative case has a recorded pass/fail.

**Key interfaces:**
- Initial jobs policy, literal D-6 maintenance prompt, maintenance-report/v1 measurement fields, and operator-owned EVAL-1, EVAL-2, and EVAL-3 rubric.

**Traces:** prd:maintenance-jobs/FR-2 · prd:maintenance-jobs/FR-12 · prd:maintenance-jobs/FR-13 · prd:maintenance-jobs/NR-3 · prd:maintenance-jobs/ABUSE-3 · prd:maintenance-jobs/EVAL-1 · prd:maintenance-jobs/EVAL-2 · prd:maintenance-jobs/EVAL-3 · prd:maintenance-jobs/DOD-4 · prd:maintenance-jobs/NR-2 · design:maintenance-jobs#D-1 · design:maintenance-jobs#D-6

### T27 — Fence and prove quiescence of the known standalone runner

| Field | Value |
|---|---|
| **Blocked by** | T7, T23, T25 |
| **Size band** | 220–300 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Requirements** | {"schema_version":"task-requirements/v1","implementation":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"adapt","verification":"cross_boundary","impact_scope":"multi_surface","architectural_role":"feature_local","citations":[{"id":"task:acceptance"},{"id":"design:maintenance-jobs#D-7"}]},"rationale":"Legacy exclusion joins executable-path fencing, existing lock semantics, backups, and attribution of previously dispatched descendants."},"review":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"adapt","verification":"cross_boundary","impact_scope":"system_wide","architectural_role":"shared_substrate","citations":[{"id":"task:acceptance"},{"id":"design:maintenance-jobs#D-7"}]},"rationale":"Review must rule out delayed dispatch and broad killing because old and new uncoordinated runners must never overlap."}} |
| **Pattern** | — |
| **Touches** | internal/maintenance/legacy.go, internal/maintenance/legacy_test.go |

**Goal:** Migration can exclude the known old runner and prevent delayed old dispatch from overlapping a new installation.

**Current behavior:** The standalone runner has an existing exclusive run lock but remains an uncoordinated executable dispatch path.

**Desired behavior:** A typed legacy fence captures the exact selected-repository launcher and recorded state, obtains its existing lock within a bound, retires the launch path, and proves exact owned-process-tree termination before new authority.

**Scope:**
- Inspect only the known standalone entry, launcher, and recorded lock/state for the selected repository; preview safe references.
- Obtain the existing exclusive runner lock or refuse a bounded active-run wait without activating new scheduling.
- Retire the launcher into restricted backup under exclusion and drain exactly attributed previously dispatched executable descendants.
- Restore verified prior fencing/installation or retain unavailable recovery evidence when proof fails; retain the fence against delayed exec.

**Out of scope:**
- Adopting arbitrary legacy entries, broad process-name killing, or new cron activation.

**Acceptance criteria:**
- [ ] An active old attempt is boundedly awaited or refused with zero new activation.
- [ ] A pre-lock dispatched process and a delayed exec of the retired path cannot admit old agent work after a proved fence.
- [ ] Missing exact process ownership or unproved descendant termination refuses migration and restores verified prior state or reports unavailable recovery evidence.
- [ ] Preview changes no launcher, lock, process, or cron state; repeated proof recognizes the retained fence without restoring an absent old launcher.

**Key interfaces:**
- Known LegacyInstallation, existing exclusive run lock, LegacyFence, retained launcher backup, and positive owned-process quiescence proof.

**Traces:** prd:maintenance-jobs/FR-14 · prd:maintenance-jobs/UB-5 · prd:maintenance-jobs/NR-1 · prd:maintenance-jobs/NR-2 · prd:maintenance-jobs/DOD-5 · design:maintenance-jobs#D-7

### T28 — Commit reversible legacy cutover through the installation transaction

| Field | Value |
|---|---|
| **Blocked by** | T8, T24, T25, T26, T27 |
| **Size band** | 220–300 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Requirements** | {"schema_version":"task-requirements/v1","implementation":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"adapt","verification":"cross_boundary","impact_scope":"multi_surface","architectural_role":"feature_local","citations":[{"id":"task:acceptance"},{"id":"design:maintenance-jobs#D-2"}]},"rationale":"Cutover combines scheduler, manifest, legacy executable exclusion, and rollback sequencing with no dual-active window."},"review":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"adapt","verification":"cross_boundary","impact_scope":"system_wide","architectural_role":"shared_substrate","citations":[{"id":"task:acceptance"},{"id":"design:maintenance-jobs#D-2"}]},"rationale":"Review must test every interrupted transition and new-process rollback race before the operator's host can adopt it."}} |
| **Pattern** | — |
| **Touches** | internal/maintenance/migration.go, internal/maintenance/migration_test.go, cmd/pyxis/maintenance_install.go |

**Goal:** An explicit selected-repository migration activates the core entry only after the old runner is fenced, and rollback cannot restore it while new work is active.

**Current behavior:** Ordinary install and standalone fencing are available but not joined into a reversible migration.

**Desired behavior:** The legacy replacement option holds transaction and legacy exclusion, backs up prior cron, binary, and manifest, replaces the old and new entries once, verifies ready authority, and retains idempotent rollback evidence.

**Scope:**
- Wire explicit legacy replacement into the existing install grammar; require completed verification and smoke evidence before host cutover.
- Join proved legacy fence and quiescence with one backed-up cron replacement and verified new ready generation.
- Recognize previously completed migration and retained fence on repeated install, including the migration option.
- Implement rollback that marks new authority unavailable, removes new dispatch, proves all already-claimed local new processes stopped, and only then restores verified legacy state.

**Out of scope:**
- Implicit enrollment of other repositories, running both executors, deleting daily claims, or changing unrelated refresh entries.

**Acceptance criteria:**
- [ ] Cutover activates exactly one effective entry after legacy proof, preserves unrelated and other-repository bytes, and retains restricted prior cron, binary, and manifest evidence.
- [ ] Failure at every transaction phase restores a verified safe pair or unavailable state; unproved old termination permits zero new activation.
- [ ] Repeating completed migration with or without its option makes zero cron writes and never resurrects the old launch path.
- [ ] Rollback removes and fences new dispatch first and refuses legacy restoration while a claimed new process tree is active or unproved; shared claims and audit history remain retained.

**Key interfaces:**
- Explicit legacy replacement option, InstallationManifest transaction, LegacyFence, verified adoption prerequisites, and ordered rollback contract.

**Traces:** prd:maintenance-jobs/FR-4 · prd:maintenance-jobs/FR-5 · prd:maintenance-jobs/FR-11 · prd:maintenance-jobs/FR-14 · prd:maintenance-jobs/UB-5 · prd:maintenance-jobs/NR-1 · prd:maintenance-jobs/NR-2 · prd:maintenance-jobs/DOD-5 · design:maintenance-jobs#D-2 · design:maintenance-jobs#D-7 · design:maintenance-jobs#D-8

### T29 — Prove the installed capability and document operator control

| Field | Value |
|---|---|
| **Blocked by** | T24, T25, T26, T28 |
| **Size band** | 180–280 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Requirements** | {"schema_version":"task-requirements/v1","implementation":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"adapt","verification":"cross_boundary","impact_scope":"multi_surface","architectural_role":"feature_local","citations":[{"id":"task:acceptance"},{"id":"design:maintenance-jobs#D-1"}]},"rationale":"Installed behavior combines release ownership, platform matrix, private authority, and regression checks beyond unit-level slices."},"review":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"adapt","verification":"cross_boundary","impact_scope":"system_wide","architectural_role":"shared_substrate","citations":[{"id":"task:acceptance"},{"id":"design:maintenance-jobs#D-1"}]},"rationale":"Review must map every blocking DoD to exact revision evidence and detect fixtures that accidentally rely on live state or omit a supported harness."}} |
| **Pattern** | — |
| **Touches** | internal/maintenance/e2e/, internal/installer/ownership_maintenance_test.go, internal/converge/maintenance_test.go, docs/runbooks/maintenance-jobs.md, internal/release/stage_test.go |

**Goal:** One installed core binary provides the contracted maintenance behavior across supported platforms and preserves repository job policy through core upgrades.

**Current behavior:** Individual slices do not yet establish installed two-repository behavior, release ownership, or an operator runbook.

**Desired behavior:** Hermetic installed-binary and adapter fixtures cover the full flow, platform matrix, convergence ownership, and each blocking DoD's fixture evidence; the runbook explains installation, disable, drift, failures, retained work, removal, and rollback. T30 supplies the live smoke and adoption evidence.

**Scope:**
- Add hermetic end-to-end fixtures with injected time, credentials, native tools, local bare remotes, and descendant processes; reuse each slice's conformance suites.
- Prove Darwin/Linux arm64/amd64 build and packaging ownership with one core CLI and no nested module or new logic-bearing shell.
- Prove core install, convergence, and upgrade preserve repository job definitions and ignored journal while legacy Pyxis verbs retain behavior.
- Document exact contracted resources and commands, audit/artifact correlation, native timezone limits, daily authority, kill switch, no-recovery scope, and first-real-outcome operator checkpoint.

**Out of scope:**
- Live model inference, real host cron activation, autonomous rubric ownership, or duplicating routine test runs as semantic memory.

**Acceptance criteria:**
- [ ] Two isolated fixture repositories use one compiled CLI under cron-style environments without cross-repository mutations, and the contracted full install, run, status, and uninstall flow passes.
- [ ] Supported architecture builds and platform enforcement/conformance fixtures pass; unsupported scheduler or process access refuses explicitly.
- [ ] Release, convergence, and upgrade fixtures preserve reviewed jobs and journal, omit local diagnostics from release payloads, and retain existing CLI/hook behavior.
- [ ] Revision-bound formatting, vet, repository lint, required race, coverage, and consumer checks, and each blocking DoD's hermetic evidence are captured; the runbook names operational states, T30's outstanding live smoke/adoption evidence, and first-outcome human review.

**Key interfaces:**
- Installed core maintenance CLI, release ownership and convergence contracts, four-adapter native conformance, blocking DoD evidence, and operator runbook.

**Traces:** prd:maintenance-jobs/FR-1 · prd:maintenance-jobs/FR-11 · prd:maintenance-jobs/FR-14 · prd:maintenance-jobs/FR-15 · prd:maintenance-jobs/NR-1 · prd:maintenance-jobs/NR-2 · prd:maintenance-jobs/NR-3 · prd:maintenance-jobs/NFR-1 · prd:maintenance-jobs/NFR-2 · prd:maintenance-jobs/NFR-3 · prd:maintenance-jobs/NFR-4 · prd:maintenance-jobs/DOD-1 · prd:maintenance-jobs/DOD-2 · prd:maintenance-jobs/DOD-3 · prd:maintenance-jobs/DOD-4 · prd:maintenance-jobs/DOD-5 · prd:maintenance-jobs/DOD-6 · prd:maintenance-jobs/DOD-7 · design:maintenance-jobs#D-1 · design:maintenance-jobs#D-7 · design:maintenance-jobs#D-8

### T30 — Smoke-test and adopt the verified daily CI job on the enrolled host

| Field | Value |
|---|---|
| **Blocked by** | T28, T29 |
| **Size band** | 40–100 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Requirements** | {"schema_version":"task-requirements/v1","implementation":{"assessment":{"reasoning":"complex","coupling":"composed","novelty":"known","verification":"composed","impact_scope":"contained","architectural_role":"feature_local","citations":[{"id":"task:acceptance"},{"id":"design:maintenance-jobs#D-6"}]},"rationale":"Host adoption invokes the already verified engine and smoke harness but must retain exact evidence before the machine's active schedule changes."},"review":{"assessment":{"reasoning":"difficult","coupling":"cross_boundary","novelty":"adapt","verification":"cross_boundary","impact_scope":"multi_surface","architectural_role":"shared_substrate","citations":[{"id":"task:acceptance"},{"id":"design:maintenance-jobs#D-6"}]},"rationale":"Review must distinguish fixture verification from real headless adoption and prove that rollback rehearsal cannot leave two uncoordinated executors active."}} |
| **Pattern** | — |
| **Touches** | internal/maintenance/e2e/smoke.go, enrolled-host-maintenance-installation, adoption-proposal-evidence |

**Goal:** The explicitly enrolled host replaces its standalone job only after a real headless smoke and retains inspectable cutover and rollback evidence.

**Current behavior:** The engine may be verified in fixtures while the original machine still runs its standalone daily entry.

**Desired behavior:** The verified core build follows normal installation, a test-only isolated job proves selected-harness headless access without CI optimization, and explicit transactional migration leaves one inspected 02:00 daily core entry.

**Scope:**
- Build/install the exact verified core revision through the normal installation route, retaining prior rollback material.
- Run a test-only published smoke job with the selected initial harness to prove native routing, private model authentication, shared confinement, supervised Git metadata access, journal correlation, and whole-tree shutdown without CI changes.
- Preview the exact known-host migration and invoke the verified fenced cutover only after all blocking evidence passes.
- Capture sanitized before, after, and backup references, no-op reinstall, read-only status, and rehearsed rollback evidence in the owning delivery proposal; leave only the approved daily entry active.

**Out of scope:**
- Installing other repositories or hosts, claiming ten-minute CI success, creating a real optimization during smoke, or human merge/approval of job-generated PRs.

**Acceptance criteria:**
- [ ] The live isolated smoke uses the verified revision and configured harness under a noninteractive cron-style environment, retains complete correlated evidence, and makes zero CI optimization changes.
- [ ] The selected host has exactly one intended core daily entry at 02:00 America/Los_Angeles with preserved unrelated refresh work and verified legacy fencing.
- [ ] Reinstallation, including legacy replacement, performs zero cron writes; status identifies intended installation and smoke evidence without repairing state.
- [ ] A rehearsed rollback proves removal and process-stop ordering with retained claims; private host configuration and credentials stay out of Git, and the owning proposal records completed adoption without inferring the CI target.

**Key interfaces:**
- Verified core installation, test-only smoke policy, native selected harness, explicit legacy migration, read-only status, and revision-bound adoption evidence.

**Traces:** prd:maintenance-jobs/FR-3 · prd:maintenance-jobs/FR-5 · prd:maintenance-jobs/FR-11 · prd:maintenance-jobs/FR-13 · prd:maintenance-jobs/FR-14 · prd:maintenance-jobs/FR-15 · prd:maintenance-jobs/NR-1 · prd:maintenance-jobs/NR-2 · prd:maintenance-jobs/DOD-1 · prd:maintenance-jobs/DOD-5 · prd:maintenance-jobs/DOD-6 · prd:maintenance-jobs/DOD-7 · design:maintenance-jobs#D-6 · design:maintenance-jobs#D-7 · design:maintenance-jobs#D-8

## Coverage

| Requirement | Tasks |
|---|---|
| prd:maintenance-jobs/FR-1 | T1, T2, T7, T29 |
| prd:maintenance-jobs/FR-2 | T2, T26 |
| prd:maintenance-jobs/FR-3 | T5, T6, T8, T30 |
| prd:maintenance-jobs/FR-4 | T5, T6, T8, T28 |
| prd:maintenance-jobs/FR-5 | T5, T8, T28, T30 |
| prd:maintenance-jobs/FR-6 | T9, T10, T20 |
| prd:maintenance-jobs/FR-7 | T9, T10, T20, T21, T23 |
| prd:maintenance-jobs/FR-8 | T7, T9, T19 |
| prd:maintenance-jobs/FR-9 | T11, T12, T13, T14, T19, T20, T22, T25 |
| prd:maintenance-jobs/FR-10 | T3, T11, T12, T13, T14, T15, T16, T20 |
| prd:maintenance-jobs/FR-11 | T4, T8, T10, T21, T23, T24, T25, T28, T29, T30 |
| prd:maintenance-jobs/FR-12 | T17, T18, T19, T21, T22, T23, T26 |
| prd:maintenance-jobs/FR-13 | T17, T18, T26, T30 |
| prd:maintenance-jobs/FR-14 | T2, T8, T24, T25, T27, T28, T29, T30 |
| prd:maintenance-jobs/FR-15 | T2, T3, T9, T11, T12, T13, T14, T15, T16, T20, T29, T30 |
| prd:maintenance-jobs/UB-1 | T2, T4, T23 |
| prd:maintenance-jobs/UB-2 | T4, T9, T10, T15, T16, T19, T23, T24 |
| prd:maintenance-jobs/UB-3 | T10, T20, T21, T23, T25 |
| prd:maintenance-jobs/UB-4 | T6, T7, T9, T24 |
| prd:maintenance-jobs/UB-5 | T4, T7, T8, T27, T28 |
| prd:maintenance-jobs/UB-6 | T19, T22 |
| prd:maintenance-jobs/NR-1 | T5, T6, T8, T27, T28, T29, T30 |
| prd:maintenance-jobs/NR-2 | T1, T3, T9, T11, T12, T13, T14, T15, T19, T22, T26, T27, T28, T29, T30 |
| prd:maintenance-jobs/NR-3 | T17, T18, T22, T26, T29 |
| prd:maintenance-jobs/NFR-1 | T1, T2, T29 |
| prd:maintenance-jobs/NFR-2 | T6, T12, T13, T16, T29 |
| prd:maintenance-jobs/NFR-3 | T4, T7, T11, T12, T13, T14, T15, T16, T17, T18, T19, T21, T23, T25, T29 |
| prd:maintenance-jobs/NFR-4 | T4, T10, T20, T21, T23, T24, T25, T29 |
| prd:maintenance-jobs/ABUSE-1 | T2, T5 |
| prd:maintenance-jobs/ABUSE-2 | T10 |
| prd:maintenance-jobs/ABUSE-3 | T22, T26 |
| prd:maintenance-jobs/DOD-1 | T29, T30 |
| prd:maintenance-jobs/DOD-2 | T29 |
| prd:maintenance-jobs/DOD-3 | T29 |
| prd:maintenance-jobs/DOD-4 | T26, T29 |
| prd:maintenance-jobs/DOD-5 | T27, T28, T29, T30 |
| prd:maintenance-jobs/DOD-6 | T24, T29, T30 |
| prd:maintenance-jobs/DOD-7 | T16, T19, T29, T30 |
| prd:maintenance-jobs/EVAL-1 | T26 |
| prd:maintenance-jobs/EVAL-2 | T17, T18, T26 |
| prd:maintenance-jobs/EVAL-3 | T18, T22, T26 |
| prd:maintenance-jobs/EVT-1 | T4, T8 |
| prd:maintenance-jobs/EVT-2 | T4, T10, T23 |
| prd:maintenance-jobs/EVT-3 | T4, T21, T23 |
| prd:maintenance-jobs/EVT-4 | T4, T20, T23, T25 |
