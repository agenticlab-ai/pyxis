# Init hardening — plan 001

| Field | Value |
|---|---|
| **Feature** | init-hardening |
| **PRD** | docs/prds/init-hardening |
| **Design** | docs/tech-design/init-hardening-v1.md |
| **Plan version** | 001 |
| **Dialect** | harbormaster/fs-spec:v1 |
| **Epic title** | Harden pyxis init — the checked front door |
| **Status** | Draft — freezes at merge |
| **Template version** | plan-template/v1 |

## Tasks

### T1 — Admit only empty targets and route everything else

| Field | Value |
|---|---|
| **Blocked by** | — |
| **Size band** | 150–280 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/preflight (new) · internal/installer/helpers.go · internal/installer/installer.go · cmd/pyxis/main.go |

**Goal:** The check-registry package exists with its outcome model, and its first check — content-based admission — refuses every non-empty target by name with the adoption route, before any write.

**Current behavior:** Admission tests git state only: a non-git directory full of files is admitted and overwritten; the non-empty refusal claims adoption is deferred and names no verb.

**Desired behavior:** A check registry (id · applicability · read-only probe · outcome pass · refuse · advise) drives init's front door. Admission admits exactly the empty directory and the metadata-only repository; a content-holding repository refuses naming `pyxis adopt`; a non-git target with content refuses with the runnable two-step (`git init -b <branch>`, then adopt); every refusal lists the conflicts, emits the refusal event, and leaves the target byte-identical.

**Scope:**
- The registry types and outcome model, with the report rendering init consumes
- The admission predicate (strict emptiness) and its conflict enumeration
- The routed refusal texts (adopt; the two-step) replacing the stale wording
- The refusal event (`init_refused`) with check, remedy, and route properties

**Out of scope:**
- Every other check (T2/T3); preview and doctor consumption (T9/T10)

**Acceptance criteria:**
- [ ] Across the admission fixtures (empty dir · empty repo · repo+content · non-git+content · dirty repo), exactly the two empty shapes proceed
- [ ] Both refusal classes name the adoption route; no refusal text claims adoption is unavailable
- [ ] Every admission refusal performs zero filesystem and zero remote mutation (byte-identical target)
- [ ] `init_refused` fires with typed check, remedy-present, and route-present properties on each refusal

**Key interfaces:**
- the check registry's `Check`/`Outcome` contracts; the admission predicate; the refusal renderer

**Traces:** prd:init-hardening/FR-1 · prd:init-hardening/FR-2 · prd:init-hardening/FR-16 · prd:installer/UB-11 · design:init-hardening-v1#3.1 · design:init-hardening-v1#DINV-5

### T2 — Front-load the toolchain and artifact-source checks

| Field | Value |
|---|---|
| **Blocked by** | T1 |
| **Size band** | 120–240 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/preflight · internal/installer/installer.go · internal/installer/tail.go |

**Goal:** The toolchain floors (git always; the knob-implied runtimes) and cold-cache artifact-source reachability are verified before the first question or write, retiring the mid-pipeline probes.

**Current behavior:** legacy package-runtime prerequisites are probed mid-pipeline and the artifact source at converge's fetch — both after mutations begin; git's version is never checked.

**Desired behavior:** Registry checks verify presence and floor version per implied executable (the floor set resolved from one release-owned source — design Q2 decided here) and probe the artifact source under the resolved credential when the cache is cold; each defect refuses pre-write with that check's remedy; the pass emits `init_checks_passed`; the legacy mid-pipeline probes are removed in favor of the suite (one home).

**Scope:**
- The toolchain checks (presence + floor), floors resolved from a single release-owned source
- The cold-cache reachability probe honoring the artifact-source override and credential
- Removal of the now-redundant mid-pipeline probes (the suite is the one evaluator)
- The `init_checks_passed` event with checks/duration properties

**Out of scope:**
- GitHub-mode checks (T3); the install-budget re-measurement (T11)

**Acceptance criteria:**
- [ ] Each seeded toolchain defect (absence; below-floor) refuses pre-write naming that check's remedy
- [ ] An unreachable artifact source on a cold cache refuses pre-write naming host and credential remedy; a warm verified cache skips the probe
- [ ] After the pass, no retired probe re-fires as a mid-pipeline refusal (the injection fixtures prove the ordering invariant)
- [ ] `init_checks_passed` fires exactly once per successful pass

**Key interfaces:**
- the floor-set resolution contract; the reachability probe seam; the check-pass event

**Traces:** prd:init-hardening/FR-3 · prd:init-hardening/FR-5 · prd:init-hardening/FR-9 · design:init-hardening-v1#3.1 · design:init-hardening-v1#DINV-1

### T3 — GitHub-mode checks: auth, coordinate, existence, scope advisories

| Field | Value |
|---|---|
| **Blocked by** | T1 |
| **Size band** | 140–280 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/preflight · internal/wire · internal/installer/helpers.go |

**Goal:** When the knobs imply any gh-transport operation, init verifies gh presence and an authenticated identity, resolves the coordinate from the configured remote or the promptless inputs, probes the coordinate's repository per wiring mode, and emits best-effort scope advisories — all read-only, all pre-write.

**Current behavior:** The only gh probe is a version call at the wiring stage, six stages in; auth is never verified; the coordinate comes only from the remote; scope gaps surface as late operation failures.

**Desired behavior:** Applicability derives from the declared operation table. Checks verify gh + login; the coordinate resolves from the configured remote name or a promptless flag/answers key (neither present promptless → named pre-write refusal); `use` probes existence + readability via the API, `create` treats absence as expected; knowable scope gaps emit advisories naming operation and remedy — never a refusal; the cross-knob coherence check refuses the wedged pair (`ci_mode` excluding hosted CI while protection requires the hosted check) with both remedies.

**Scope:**
- The gh presence/auth checks and the per-mode existence probe (transient-classified, bounded retries)
- Coordinate resolution: configured-remote parse + the promptless key + the promptless refusal
- Best-effort scope advisories (advisory-only by construction)
- The cross-knob coherence check with its first refusing pair

**Out of scope:**
- Interactive elicitation and remote materialization (T5)

**Acceptance criteria:**
- [ ] Logged-out gh, and `use` against a missing/inaccessible repository, each refuse pre-write with the remedy; carina-mode runs perform zero GitHub calls
- [ ] A promptless GitHub-mode run with neither remote nor coordinate refuses pre-write naming the flag and answers-file remedies
- [ ] Scope advisories never block: the token-matrix fixture yields warnings on knowable gaps and silence on unknowable ones, with the run proceeding
- [ ] The wedged knob pair refuses pre-write naming both remedies

**Key interfaces:**
- the operation-table applicability derivation; the coordinate resolution contract; the advisory outcome class

**Traces:** prd:init-hardening/FR-4 · prd:init-hardening/FR-7 · prd:init-hardening/FR-11 · design:init-hardening-v1#3.1 · design:init-hardening-v1#DINV-6 · design:init-hardening-v1#DINV-7

### T4 — The write journal and the rollback unwind

| Field | Value |
|---|---|
| **Blocked by** | — |
| **Size band** | 160–300 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/installer/installer.go · internal/installer/tail.go · internal/installer/helpers.go |

**Goal:** Every init mutation is journaled as it lands, and any stage failure after mutation begins unwinds the journal in reverse — the target restored to as-found, foreign entries preserved and named, remote creations reported never deleted.

**Current behavior:** A mid-pipeline failure strands the half-built tree; the retry refusal fires on init's own debris; recovery is manual deletion.

**Desired behavior:** One journaling write primitive carries every installer write (file · dir · config · git-init · remote-op); converge's contribution ingests its returned Report before the pipeline proceeds; failure (error, signal, closed input mid-apply) unwinds in reverse — files, emptied dirs, the metadata directory when init created it — preserving anything unjournaled and naming it; the created-remote case reports the cleanup command; the rollback event fires; a subsequent init proceeds past admission.

**Scope:**
- The journal types, the write primitive, and converge-Report ingestion
- The reverse unwind incl. the created-repo and foreign-entry semantics
- Signal/EOF interruption handling during apply
- The `init_rolled_back` event with failed-stage/restored properties

**Out of scope:**
- Journal persistence across uncatchable kills (design Q1 — the admission conflict list is the floor)

**Acceptance criteria:**
- [ ] Per-stage failure injection restores the target byte-identical to pre-run; the created-repo case returns the folder to empty
- [ ] A foreign file planted mid-apply survives the unwind and is named in the report
- [ ] A remotely created repository is never deleted; the report names its manual cleanup command
- [ ] After any rolled-back failure, a re-run proceeds past admission; `init_rolled_back` fired with typed properties

**Key interfaces:**
- the journal entry contract; the journaling write primitive; the unwind walk; the Report-ingestion seam

**Traces:** prd:init-hardening/FR-10 · prd:init-hardening/UB-1 · prd:init-hardening/UB-2 · prd:init-hardening/UB-3 · design:init-hardening-v1#3.6 · design:init-hardening-v1#DINV-3 · design:init-hardening-v1#DINV-4

### T5 — The interactive coordinate exchange and the create leg

| Field | Value |
|---|---|
| **Blocked by** | T3, T4 |
| **Size band** | 120–240 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/installer/helpers.go · internal/installer/tail.go · internal/wire |

**Goal:** GitHub mode with no configured remote elicits a shape-validated `owner/name` interactively, materializes the remote at apply (journaled), and `wiring repo=create` lands the repository from a bare folder.

**Current behavior:** With no remote, GitHub mode dies at the wiring stage; `create` cannot bootstrap because its coordinate comes from the remote it is meant to produce.

**Desired behavior:** The conditional exchange fires only interactively and only when no remote resolves (promptless behavior is T3's); input validates against the strict coordinate grammar (hostile input re-asks, never reaches argv); the check summary echoes the coordinate; the remote is configured at apply under the config-resolved name as a journaled write; `create` ensures the repository idempotently before dependent wiring; promptless zero-prompt parity holds.

**Scope:**
- The conditional interactive exchange with grammar validation and re-ask
- Remote materialization at apply (journaled, config-resolved name)
- The create leg ordered before dependent wiring ops, idempotent on already-exists
- The coordinate echo in the check summary

**Out of scope:**
- The five-knob question set's glosses (T7)

**Acceptance criteria:**
- [ ] Remote-present runs ask nothing; remote-absent interactive runs elicit, validate (hostile fixtures re-ask), and leave the remote configured post-apply
- [ ] The bare-folder create scenario ends green with repository, remote, and wiring landed (hermetic transport), the coordinate echoed pre-create
- [ ] An already-existing repository under `create` converges as the ensured end state, not an error
- [ ] The promptless e2e matrix passes unchanged (zero prompts)

**Key interfaces:**
- the coordinate grammar; the conditional-exchange gate in knob resolution; the remote-materialization step

**Traces:** prd:init-hardening/FR-6 · prd:init-hardening/FR-8 · prd:init-hardening/ABUSE-1 · prd:init-hardening/NR-1 · prd:installer/FR-23 · design:init-hardening-v1#3.7

### T6 — Wire `ci_mode` for real: knob-conditional workflow, advise-only migration

| Field | Value |
|---|---|
| **Blocked by** | — |
| **Size band** | 90–180 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | the class-policy source (payload authoring) · internal/converge · internal/converge/doctor.go |

**Goal:** The hosted docs-gate workflow materializes exactly when `ci_mode` includes hosted CI; an already-installed repo carrying it under an excluding knob gets a doctor advisory, never a deletion.

**Current behavior:** The workflow's class entry is unconditional — a `local`/carina install still ships and commits the hosted workflow; the knob changes nothing.

**Desired behavior:** The class-policy entry is knob-conditional; excluded means unlisted, and converge never acts on unlisted paths — no deletion machinery exists; fresh installs are born right; doctor reports the mismatch (workflow present + excluding knob) with both remedies; desk hooks install in every mode.

**Scope:**
- The knob-conditional class-policy entry and converge's conditional desired-set computation
- The doctor mismatch advisory with both remedies
- Hooks unconditional across all modes (unchanged, proven)

**Out of scope:**
- Any converge deletion behavior (design O-2 heritage — foreclosed)

**Acceptance criteria:**
- [ ] `local` installs ship no workflow file; `actions`/`both` ship it; hooks present and active in all modes
- [ ] On a fixture with the workflow present and `ci_mode=local`, doctor emits the advisory naming both remedies and performs zero mutations
- [ ] A second converge run on any mode mutates nothing (idempotent desired set)

**Key interfaces:**
- the conditional class-policy entry shape; the desired-set knob predicate; the doctor advisory row

**Traces:** prd:init-hardening/FR-11 · design:init-hardening-v1#3.8

### T7 — The answerable question surface

| Field | Value |
|---|---|
| **Blocked by** | — |
| **Size band** | 100–200 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/installer/helpers.go · cmd/pyxis/main.go |

**Goal:** Every interactive knob question renders its closed value set, a one-line plain-language gloss of what the answer changes, and its default (a derived default naming its basis); bad input re-asks bounded instead of killing the run.

**Current behavior:** Prompts are recycled flag-help strings; one prompt hides its legal values; one out-of-enum keystroke aborts the whole run discarding every answer; the tracker-derived CI default is unexplained.

**Desired behavior:** Each of the five knob questions carries value set + gloss + default; the tracker-derived CI-mode default names its basis; out-of-enum input re-asks up to 3 attempts then aborts listing every collected answer; a closed input stream takes the default without entering the loop; the five-knob boundary holds (no question beyond the knobs and the contracted conditional exchanges).

**Scope:**
- The question renderer (enum + gloss + default + derived-basis note) for the five knobs
- The bounded re-ask loop with the reported abort
- EOF-takes-default preserved through the new loop

**Out of scope:**
- The conditional coordinate exchange (T5); refusal texts (T1)

**Acceptance criteria:**
- [ ] Every asked question displays its value set, gloss, and default; the derived CI default names its basis
- [ ] Out-of-enum input re-asks without process exit; the third failure aborts listing every answer given
- [ ] EOF on any question takes the default (existing prompter suites pass unchanged)
- [ ] No question is asked outside the five knobs and the contracted conditional exchanges

**Key interfaces:**
- the question rendering contract; the re-ask loop; the gloss table

**Traces:** prd:init-hardening/FR-12 · prd:init-hardening/FR-13 · prd:installer/FR-23 · design:init-hardening-v1#3.1

### T8 — The truth surface: completion boundary and the manifest advisory

| Field | Value |
|---|---|
| **Blocked by** | — |
| **Size band** | 70–140 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/installer/tail.go · the welcome-item template (payload authoring) · internal/converge/doctor.go |

**Goal:** Install completion states truthfully what is gated (docs) and what is pending (the code gate, with its enabling seed command named); doctor advises when code exists without a verification manifest.

**Current behavior:** The welcome advertises the implement pipeline while the verify ladder refuses on every fresh operand; nothing names the manifest step; doctor is silent on the gap.

**Desired behavior:** The completion/welcome output names the active docs gates and the pending code gate with the seed command; doctor's advisory fires exactly when recognized code marker files exist without a manifest, naming the same command; manifest-present and code-free trees stay silent. The verb itself ships via the sibling implementation-verification amendment — this task only routes to it.

**Scope:**
- The completion/welcome boundary statement (knob-aware: desk/hosted per `ci_mode`)
- The doctor code-without-manifest advisory keyed on the ecosystem marker set
- Route text naming the seed command in both surfaces

**Out of scope:**
- The seed verb's implementation (prd:implementation-verification amendment 001 — a sibling build)

**Acceptance criteria:**
- [ ] Completion output names the active gates and the pending code gate with the seed command, matching the resolved knobs
- [ ] The code-without-manifest doctor fixture yields the advisory naming the seed command; manifest-present and code-free fixtures stay silent
- [ ] No completion text advertises a capability that refuses on the fresh operand without its caveat

**Key interfaces:**
- the welcome substitution variables; the doctor advisory row; the marker-set consumption (one detection source)

**Traces:** prd:init-hardening/FR-14 · prd:init-hardening/FR-15 · design:init-hardening-v1#3.9

### T9 — Preview reports the whole plan

| Field | Value |
|---|---|
| **Blocked by** | T1, T2, T3 |
| **Size band** | 90–180 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/installer/installer.go · cmd/pyxis/main.go |

**Goal:** `init --preview` runs the same check registry and reports every check outcome, the exact write set, and the declared wiring operation set — performing none of it, cold cache included.

**Current behavior:** Preview never plans the wiring leg (no coordinate, no operation table) and on a cold cache cannot enumerate the write set — blind to the likeliest failures.

**Desired behavior:** Preview executes the full registry (same code path as init — one home), renders every outcome including advisories, enumerates the would-write set and the declared wiring operations for the resolved knobs and coordinate, and performs zero mutations and zero mutating remote calls; the cold-cache leg reports what the manifest fetch would resolve rather than going blind.

**Scope:**
- Preview's registry execution and full outcome rendering
- The would-do enumeration: write set + declared wiring operation table
- The cold-cache reporting posture (read-only manifest resolution)

**Out of scope:**
- Converge's own preview contract beyond what init's report consumes

**Acceptance criteria:**
- [ ] Preview output enumerates every check outcome, the write set, and the declared wiring operations for the resolved inputs
- [ ] Preview performs zero filesystem writes and zero mutating remote calls (probe-only transcript), cold cache included
- [ ] Preview and a real run against the same fixture agree on the check outcomes and the declared operation set

**Key interfaces:**
- the preview report renderer; the shared registry entry point; the would-do enumerator

**Traces:** prd:installer/FR-24 · design:init-hardening-v1#3.1 · design:init-hardening-v1#DINV-2

### T10 — Doctor delegates its prerequisite rows to the suite

| Field | Value |
|---|---|
| **Blocked by** | T1, T2, T3 |
| **Size band** | 70–140 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/converge/doctor.go · internal/preflight |

**Goal:** Doctor's prerequisite section is produced by the check suite — one predicate home, doctor's report shape and zero-mutations contract unchanged.

**Current behavior:** Doctor carries its own copies of the prerequisite probes (Git presence, gh-when-required) beside the new suite — the two-evaluator drift shape.

**Desired behavior:** The prerequisite rows delegate to the suite's post-install applicability; the report shape, severities, and exit semantics are unchanged; a tree-wide sweep proves no probe is implemented twice.

**Scope:**
- The delegation seam (suite consumption in doctor's checks)
- Removal of doctor's duplicate probe implementations
- The duplication sweep proving the one-home property

**Out of scope:**
- Doctor's tree/drift checks (converge semantics — untouched)

**Acceptance criteria:**
- [ ] Doctor's prerequisite findings on the existing fixtures are unchanged in shape and severity
- [ ] The duplicate probe implementations are gone; the sweep finds each predicate implemented exactly once
- [ ] Doctor still performs zero mutations on every path

**Key interfaces:**
- the suite's post-install applicability mode; doctor's finding renderer

**Traces:** prd:init-hardening/FR-3 · design:init-hardening-v1#3.1 · design:init-hardening-v1#DINV-2

### T11 — The acceptance matrix: injection, rollback, parity, budget

| Field | Value |
|---|---|
| **Blocked by** | T1, T2, T3, T4, T5, T6, T7, T8, T9, T10 |
| **Size band** | 150–300 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/installer/e2e · internal/payload |

**Goal:** The composed proof: the full check-failure injection matrix, the per-stage rollback matrix, the three non-regression suites, credential containment on the probes, and the install-budget re-measurement — green together.

**Current behavior:** The pieces land per-task with local fixtures; no composed run proves the whole front door end to end or re-measures the budget with checks included.

**Desired behavior:** One e2e extension runs every check-defect and per-stage-failure fixture through real init flows (hermetic transports), asserts the event stream, extends the credential-containment tests to the probes, re-runs the promptless, empty-target, and onboarding NR suites unchanged, and records the install-budget measurement with the full check pass included, within the inherited bound.

**Scope:**
- The composed injection + rollback e2e matrix with event-stream assertions
- Containment extension over the probe calls
- The NR suites wired into the matrix run
- The install-budget measurement record

**Out of scope:**
- The unaided-install trial (a post-launch human observable — the PRD's DOD-6/§6.7)

**Acceptance criteria:**
- [ ] Every A-2 defect class and every A-3 stage-failure class passes in the composed matrix with the expected events
- [ ] The probe containment tests pass (gh→GitHub only; door credential→its host only, TLS)
- [ ] Promptless, empty-target, and onboarding suites pass unchanged
- [ ] The install-budget measurement, checks included, is recorded within the inherited bound

**Key interfaces:**
- the e2e matrix harness extensions; the containment test seams; the budget measurement record

**Traces:** prd:init-hardening/FR-9 · prd:init-hardening/FR-10 · prd:init-hardening/NR-1 · prd:init-hardening/NR-2 · prd:init-hardening/NR-3 · design:init-hardening-v1#DINV-1 · design:init-hardening-v1#DINV-3

## Coverage

| Requirement | Tasks |
|---|---|
| prd:init-hardening/FR-1 | T1 |
| prd:init-hardening/FR-2 | T1 |
| prd:init-hardening/FR-3 | T2, T10 |
| prd:init-hardening/FR-4 | T3 |
| prd:init-hardening/FR-5 | T2 |
| prd:init-hardening/FR-6 | T5 |
| prd:init-hardening/FR-7 | T3 |
| prd:init-hardening/FR-8 | T5 |
| prd:init-hardening/FR-9 | T2, T9, T11 |
| prd:init-hardening/FR-10 | T4, T11 |
| prd:init-hardening/FR-11 | T3, T6 |
| prd:init-hardening/FR-12 | T7 |
| prd:init-hardening/FR-13 | T7 |
| prd:init-hardening/FR-14 | T8 |
| prd:init-hardening/FR-15 | T8 |
| prd:init-hardening/FR-16 | T1 |
| prd:init-hardening/UB-1 | T4 |
| prd:init-hardening/UB-2 | T4 |
| prd:init-hardening/UB-3 | T4 |
| prd:init-hardening/NR-1 | T5, T11 |
| prd:init-hardening/NR-2 | T11 |
| prd:init-hardening/NR-3 | T11 |
| prd:init-hardening/ABUSE-1 | T5 |
| prd:init-hardening/NFR-1 | T11 |
| prd:init-hardening/NFR-2 | T11 |
| prd:init-hardening/NFR-3 | T11 |
| prd:init-hardening/EVT-1 | T2, T11 |
| prd:init-hardening/EVT-2 | T1, T11 |
| prd:init-hardening/EVT-3 | T4, T11 |
