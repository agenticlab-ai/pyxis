# Installer — plan 001

| Field | Value |
|---|---|
| **Feature** | installer |
| **PRD** | docs/prds/installer |
| **Design** | docs/tech-design/installer-v1.md |
| **Plan version** | 001 |
| **Dialect** | harbormaster/fs-spec:v1 |
| **Epic title** | Ship the Pyxis installer: the system release, the pyxis orchestrator, and converge |
| **Status** | Draft — freezes at merge |
| **Template version** | plan-template/v1 |

## Tasks

### T1 — Parse and derive the release manifest from the authored class policy

| Field | Value |
|---|---|
| **Blocked by** | — |
| **Size band** | 180–280 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/relmanifest/ (new) · the system tree's authored class-policy data file (new) |

**Goal:** the release-manifest and class-policy contracts exist as one closed-parse Go library, and a payload tree walks into a derived manifest or fails by name.

**Current behavior:** no manifest or class-policy schema exists anywhere; the only release metadata is a SHA256SUMS file for one tool.

**Desired behavior:** the library closed-parses `class-policy/v1` and `release-manifest/v1` (unknown keys are errors); a walk over a payload tree against the policy produces the manifest object (per-file sha256 + ownership class, per-binary per-platform hashes, system version, source commit); a payload file no policy glob matches fails the walk naming the file; the source repo's authored class-policy file lands with the library.

**Scope:**
- The two closed schemas with golden fixtures, additive-evolution-ready.
- The policy walk (first-match-wins globs, three ownership classes) and its unclassed-file refusal.
- The authored class-policy data file for the current payload tree.

**Out of scope:**
- The release workflow that calls the walk (T11); converge's consumption of the manifest (T5).

**Acceptance criteria:**
- [ ] Golden manifest fixtures round-trip byte-stable; unknown keys in either schema are named defects.
- [ ] A fixture payload with one unclassed file fails the walk naming exactly that file.
- [ ] The walk over the real payload tree with the authored policy classes every file (no unclassed remainder).

**Key interfaces:**
- `relmanifest.Manifest`, `relmanifest.ClassPolicy`, `relmanifest.Walk(payloadRoot, policy) (Manifest, error)`; ownership-class enum (engine-verbatim · repo-owned · generated).

**Traces:** prd:installer/FR-1 · design:installer-v1#3.2 · design:installer-v1#DINV-6

### T2 — Extend the repo config to schema v2 with the dual-read window

| Field | Value |
|---|---|
| **Blocked by** | — |
| **Size band** | 180–280 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/docgate/config/ (extend) · the system config file (schema fields only; the source repo's mode flip is T12) |

**Goal:** the one operand config surface speaks schema v2 — system pin pair, knobs, branch, captured paths with the closure rule, epoch, artifact-source override — while dual-reading today's per-tool pin sections with a deprecation surface.

**Current behavior:** the config parser reads per-tool pin sections (docgate, codegate), paths, and plan thresholds; no schema-version field, no knobs, no epoch, no system pins.

**Desired behavior:** schema v2 closed-parses the new blocks; legacy per-tool pin sections still parse during the window and surface a deprecation notice through the doctor seam; a config carrying pin keys beyond the system pair — or legacy sections once the window closes — refuses by name; every captured path key enforces refuse-non-default; a conformance test asserts no engine configurable-path read lacks a schema key.

**Scope:**
- The v2 schema blocks and their closed parse.
- The dual-read of legacy pin sections plus the deprecation surface.
- The refuse-non-default posture per path key and the path-closure conformance test.

**Out of scope:**
- Flipping the source repo's own config (T12); any consumer behavior change behind the unchanged defaults.

**Acceptance criteria:**
- [ ] v2 fixtures parse; unknown keys, extra pin keys, and post-window legacy sections each refuse by name.
- [ ] Every non-default path-key fixture refuses with the not-yet-configurable error; zero silent-ignore paths in the conformance sweep.
- [ ] Existing suites pass unchanged with the new schema present (defaults change no behavior).

**Key interfaces:**
- `config.SchemaVersion`, `config.System{Version, ManifestSHA256, ArtifactSource, Mode}`, `config.Knobs`, `config.Epoch`; the path-closure conformance test.

**Traces:** prd:installer/FR-3 · prd:installer/FR-18 · prd:installer/UB-6 · prd:installer/UB-8 · prd:installer/NR-3 · design:installer-v1#3.7 · design:installer-v1#3.10 · design:installer-v1#DINV-2

### T3 — The acquisition core: fetch-verify-cache with per-exec re-hash

| Field | Value |
|---|---|
| **Blocked by** | T1, T2 |
| **Size band** | 220–300 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/resolve/ (new) |

**Goal:** one Go implementation owns every step from config pin to verified execution: manifest fetch-verify-cache, binary fetch-verify-cache, per-exec re-hash, override, source-mode fall-through, and the cache janitor.

**Current behavior:** resolution lives in per-tool shell scripts; one verifies against a pin, two verify nothing; no manifest exists.

**Desired behavior:** resolution reads the config pin → verifies the cached manifest (fetching on miss) → resolves the tool's platform hash → re-hashes the cached binary before every execution → fetch-verifies on miss or mismatch (tmp + atomic rename, artifact-source-override-aware) → refuses by name when offline or tampered; source mode falls through to source execution only where the source tree exists; a pre-warm entry fetches the full tool set; the janitor removes cached entries whose version is off the pin.

**Scope:**
- The resolve → verify → exec decision function and its cache lifecycle (manifest + binaries + janitor).
- Tamper, truncation, wrong-platform, missing-pin, and offline-miss refusals, each named.
- The override and source-mode legs.

**Out of scope:**
- The `pyxis` verb surface that fronts this (T4); converge's orchestration of pre-warm (T5).

**Acceptance criteria:**
- [ ] Tampered manifest, tampered binary, and truncated-fetch fixtures each refuse with zero execution, naming expected vs actual.
- [ ] A cold cache self-heals online via a file:// fixture source; offline it refuses naming the pre-warm remedy; a second resolve of a warm tool performs no fetch.
- [ ] After a pin bump plus pre-warm, off-version cache entries are gone; the override fixture fetches from the alternate source under the same verification.

**Key interfaces:**
- `resolve.Tool(ctx, cfg, tool, platform) (execPath, error)`, `resolve.PreWarm(ctx, cfg)`, `resolve.Reap(cfg)`; the manifest-cache location contract from the solution shape.

**Traces:** prd:installer/FR-13 · prd:installer/FR-17 · prd:installer/FR-21 · prd:installer/FR-22 · prd:installer/UB-2 · prd:installer/UB-3 · prd:installer/NFR-3 · design:installer-v1#3.4 · design:installer-v1#DINV-1

### T4 — The pyxis binary: verbs, the exec grammar, and the fail-closed parser

| Field | Value |
|---|---|
| **Blocked by** | T3 |
| **Size band** | 150–250 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | cmd/pyxis/ (new) |

**Goal:** the `pyxis` thin main exists with the frozen verb surface, the I3-fail-closed argument discipline on every verb, and `pyxis exec <tool> -- <argv>` delegating through the acquisition core.

**Current behavior:** no `pyxis` binary; tools are invoked directly or through shell resolvers.

**Desired behavior:** the main dispatches the init, converge, repair, doctor, exec, and version verbs; a flag-like token in a positional slot, an unknown flag, or a missing `--` on exec refuses with a usage error before any write or external operation, via one shared guard; everything after `--` passes argv-verbatim to the resolved tool; exec's exit code is the tool's; the init, converge, and doctor verbs delegate to their libraries as those land.

**Scope:**
- The verb dispatch and the shared fail-closed parse guard.
- The exec grammar end to end over the acquisition core, source mode included.
- Verb stubs that refuse-with-remedy until their libraries land (no silent no-ops).

**Out of scope:**
- The behavior behind the init, converge, and doctor verbs (T5 through T8); any second entry binary.

**Acceptance criteria:**
- [ ] Misparse fixtures (flag-after-positional, unknown flag, missing `--`) on every verb refuse pre-action with zero writes and zero external operations.
- [ ] `pyxis exec` runs a fixture tool argv-verbatim (bytes after `--` untouched) and propagates its exit code; in source mode it falls through to source execution.
- [ ] The verbs spend zero model inference (structural: no inference call sites).

**Key interfaces:**
- The verb dispatch table; `parseArgs` (the shared guard); the exec grammar contract from the solution shape.

**Traces:** prd:installer/UB-9 · prd:installer/NFR-6 · design:installer-v1#3.4 · design:installer-v1#DINV-5

### T5 — The converge engine: ownership classes, idempotency, preview

| Field | Value |
|---|---|
| **Blocked by** | T1, T2, T3 |
| **Size band** | 220–300 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/converge/ (new) · cmd/pyxis/ (verb glue) |

**Goal:** one converge library reconciles a repo against the pinned manifest per ownership class, idempotently and raceably, with a preview mode that reports the exact write set and performs none of it.

**Current behavior:** no converge exists; the system tree arrives by being the source repo.

**Desired behavior:** engine-verbatim drift repairs byte-exact, generated files regenerate, repo-owned files validate against their closed schema and are never rewritten (violations named); a second run on a converged tree mutates nothing and reports convergence; an interrupted run re-runs to the identical end state; converge orchestrates the acquisition pre-warm and the cache janitor; `--preview` enumerates every write and external operation without performing any.

**Scope:**
- The per-class reconcile over the manifest with one drift predicate.
- Idempotency, kill-and-re-run convergence, and the preview write-set report.
- Pre-warm plus janitor orchestration.

**Out of scope:**
- Seeding and wiring (init's steps — T7/T8); doctor's read-only face (T6).

**Acceptance criteria:**
- [ ] Injected drift per class: engine-verbatim repaired byte-exact, generated regenerated, repo-owned schema-invalid refused by name with the file untouched.
- [ ] A second converge run produces zero file mutations; kill-at-random-step fixtures all converge on re-run to the identical tree.
- [ ] Preview output enumerates the exact write set of a subsequent real run; the preview run itself leaves the filesystem and remote state untouched.

**Key interfaces:**
- `converge.Run(ctx, cfg, manifest, opts) (Report, error)` with `opts.Preview`; the drift predicate shared with doctor; `converge.Report{Repaired, Regenerated, RefusedByName}`.

**Traces:** prd:installer/FR-5 · prd:installer/FR-6 · prd:installer/FR-19 · prd:installer/UB-5 · design:installer-v1#3.2 · design:installer-v1#DINV-3 · design:installer-v1#DINV-4

### T6 — Doctor: the read-only drift and health face

| Field | Value |
|---|---|
| **Blocked by** | T5 |
| **Size band** | 120–200 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/converge/ (doctor mode) · cmd/pyxis/ (verb glue) |

**Goal:** `pyxis doctor` reports manifest drift by ownership class and environment health with zero mutations.

**Current behavior:** no doctor verb exists anywhere in the toolset.

**Desired behavior:** doctor reads the same drift predicate as converge and names every divergence by class; reports environment health — Git present, gh present in GitHub mode, hooks active, tracker reachable, CLI surfaces intact; reports the epoch block, legacy-config deprecation notices, stale cache entries the janitor missed, and unauthored-skeleton advisories; a doctor run performs zero filesystem or remote mutations.

**Scope:**
- The read-only report over the shared drift predicate.
- The environment, epoch, deprecation, and advisory checks.

**Out of scope:**
- Any repair action (converge owns mutation); wiring health beyond reachability.

**Acceptance criteria:**
- [ ] Each injected drift class and each removed prerequisite is detected and named; a clean install reports healthy.
- [ ] The filesystem diff across a doctor run is empty and no external mutation is performed.
- [ ] Unauthored governance skeletons and legacy pin sections each surface as named advisories.

**Key interfaces:**
- `converge.Doctor(ctx, cfg) (Health, error)`; the advisory taxonomy (drift · prerequisite · epoch · deprecation · skeleton).

**Traces:** prd:installer/FR-14 · prd:installer/UB-7 · prd:installer/NFR-4 · design:installer-v1#3.5 · design:installer-v1#3.8

### T7 — Init core: target check, knobs, capture, epoch

| Field | Value |
|---|---|
| **Blocked by** | T2, T4, T5 |
| **Size band** | 200–300 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/installer/ (new) · cmd/pyxis/ (verb glue) |

**Goal:** `pyxis init` runs the pipeline's front half: admit or refuse the target, elicit the five knobs (or take flags/answers), converge the payload, and capture config plus epoch.

**Current behavior:** no init path exists; a fresh repo fails the gates for lack of structure, config, and hooks.

**Desired behavior:** an empty target (no tracked content) is admitted — git-initialized where no repository exists; a non-empty or dirty target refuses before any write, enumerating conflicts; unset committer identity yields a repo-local identity from the answers or a named refusal, never a failed commit; the five knobs elicit interactively with recommended defaults or resolve entirely from flags/an answers file with zero prompts; every answer, path, pin, and the branch land in the v2 config; the epoch block writes once (system version, epoch commit, manifest hash); converge materializes the payload.

**Scope:**
- Target admission (bare dir and git-initialized empty repo), refusals, and identity handling.
- Knob elicitation with full flag/answers parity.
- Config capture, the epoch write, and the converge invocation.

**Out of scope:**
- Seeding, dependencies, hooks, wiring, welcome (T8); GitHub operations (T10).

**Acceptance criteria:**
- [ ] Init from a bare directory and from a git-initialized empty repo both produce the converged tree with a complete v2 config and epoch block; the two entry states end byte-identical.
- [ ] Non-empty and dirty targets refuse before any write with the conflict set named; identity-unset fixtures produce a working repo-local identity or the named refusal with no failed-commit state.
- [ ] The full flag/answers path completes with zero prompts; asked knobs are exactly the five.

**Key interfaces:**
- `installer.Init(ctx, opts) (Report, error)`; `installer.Answers` (the five knobs + flags parity); the epoch write contract.

**Traces:** prd:installer/FR-3 · prd:installer/FR-4 · prd:installer/FR-7 · prd:installer/UB-1 · prd:installer/UB-10 · design:installer-v1#3.5 · design:installer-v1#3.7

### T8 — Init tail: seeding, dependencies, hooks, wiring, welcome, green proof

| Field | Value |
|---|---|
| **Blocked by** | T7, T9, T10 |
| **Size band** | 220–300 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/installer/ (extend) |

**Goal:** init ends green: skeletons and registries seeded from the self-contained payload, hooks active, tracker and CI wired per the knobs, the welcome path emitted, and the gate passing.

**Current behavior:** none of the end state exists; the manual steps (hook activation, store init) are documented one-time commands in the source repo only.

**Desired behavior:** the governance skeletons and the operand manual instantiate from the shipped templates (knob-substituted, repo-owned; zero invented governing content); registries create empty-but-valid; rollups stay absent; the argus knob off skips the authority-contract instantiable entirely; all runtime inputs come from the verified payload with no package-install step; hooks activate without manual steps and reject a violating commit; carina mode initializes the local store, GitHub mode walks the declared operation set, actions mode installs the payload workflow; the welcome item emits with authorship pointers; the gate exits green on the seeded tree.

**Scope:**
- Seeding orchestration (instantiables, registries, argus-conditional suppression).
- Verified-payload materialization and hook activation.
- Tracker/CI wiring dispatch and the welcome emission plus the final gate proof.

**Out of scope:**
- The GitHub operation implementations (T10); template content (T9).

**Acceptance criteria:**
- [ ] Post-init the gate exits green in every knob combination; seeded skeletons diff clean against their templates apart from knob substitutions.
- [ ] With argus off no authority contract exists and the skill's refusal path is reachable; with argus on the floor-tier surface is instantiated.
- [ ] The offline combo materializes the complete payload with zero network and no package install; hooks reject a gate-violating commit without any manual activation step; carina mode answers a store query post-init.

**Key interfaces:**
- The seeding orchestration over the instantiable set; the wiring dispatch (carina init · the operation table · workflow install); the green-proof invocation.

**Traces:** prd:installer/FR-7 · prd:installer/FR-8 · prd:installer/FR-9 · prd:installer/FR-11 · prd:installer/FR-12 · prd:installer/FR-15 · prd:installer/FR-16 · prd:installer/UB-7 · design:installer-v1#3.5 · design:installer-v1#3.3

### T9 — The shipped instantiables and the installed-repo workflow variant

| Field | Value |
|---|---|
| **Blocked by** | — |
| **Size band** | 150–280 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | the system template set (new instantiables) · the payload home of the installed workflow variant |

**Goal:** every artifact init instantiates ships in the payload: the four governance-doc skeletons, the operand agents manual, the welcome item template, and the installed-repo variant of the gate workflow.

**Current behavior:** no skeleton templates exist (a fresh repo fails the gates for their absence); the only gate workflow is the source repo's, which runs source-toolchain steps no installed repo can execute.

**Desired behavior:** the four skeletons carry template structure and fill-me guidance with zero invented governing content and pass the gate's structure checks when instantiated; the operand manual template carries the factory's operating rules with knob substitution points; the welcome template carries the authorship pointers and first-feature path; the installed-repo workflow variant invokes the gate through the exec grammar with zero source-toolchain steps.

**Scope:**
- The six template files, substitution-point conventions consistent with the existing instantiable set.
- The installed-repo workflow variant as payload content distinct from the source repo's own workflow.

**Out of scope:**
- The instantiation engine (T8); the class-policy rows classing these files (T1's authored data grows in the same change they land).

**Acceptance criteria:**
- [ ] A fixture instantiation of the four skeletons plus registries passes the gate's structure checks with zero content beyond structure and guidance.
- [ ] The installed workflow variant contains exec-grammar gate steps and zero source-toolchain steps (no Go setup, vet, or race suite).
- [ ] The operand manual and welcome templates render with every knob substitution resolved for both tracker modes.

**Key interfaces:**
- The instantiable naming set from the solution shape; the substitution-variable convention.

**Traces:** prd:installer/FR-7 · prd:installer/FR-15 · design:installer-v1#3.3 · design:installer-v1#3.10 · prd:installer/NG-1

### T10 — The wiring library: the operation table over the gh CLI

| Field | Value |
|---|---|
| **Blocked by** | T2 |
| **Size band** | 180–280 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/wire/ (new) |

**Goal:** GitHub wiring is one declared-operation table executed through the gh CLI, with preview, audit, permission hand-off, and transient retry.

**Current behavior:** no wiring exists; labels, protection, and workflows are hand-configured.

**Desired behavior:** the operation table (ensure-labels, push-workflow, set-protection, create-repo, welcome-item) is the single contract: preview renders it, execution walks exactly it via gh, the audit event records what ran; gh absent is a named GitHub-mode prerequisite failure; a missing permission completes every permitted operation and emits a copy-paste hand-off for the remainder, reporting partial; transient API failures retry within a stated bound then hand off; the executed set never exceeds the declared set.

**Scope:**
- The operation table, its knob conditions, and the gh transport behind it.
- Preview and audit from the same table; the hand-off and bounded-retry legs.

**Out of scope:**
- When operations run (init's dispatch — T8); non-GitHub trackers.

**Acceptance criteria:**
- [ ] Against a fake gh transcript, the executed operation set equals the declared table in every knob combination; the audit event records it.
- [ ] A permission-limited transcript yields the permitted subset applied plus a hand-off block that completes the remainder verbatim; a 429 transcript retries within the bound then hands off.
- [ ] With gh absent, GitHub-mode wiring refuses with the named prerequisite remedy and zero operations.

**Key interfaces:**
- `wire.Table(cfg) []Op`, `wire.Execute(ctx, ops) (Audit, error)`, `wire.Preview(ops)`; the Op row shape from the solution shape.

**Traces:** prd:installer/FR-10 · prd:installer/UB-4 · design:installer-v1#3.6 · prd:installer/ABUSE-3

### T11 — The system release: one tag to a complete verified artifact set

| Field | Value |
|---|---|
| **Blocked by** | T1 |
| **Size band** | 200–300 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | .github/workflows/ (new system-release workflow) · the system scripts home (bootstrap shim) · cmd/ (build-only manifest entry over T1) |

**Goal:** one version tag publishes the complete artifact set — every scan-derived binary per platform, the self-contained payload archive, the derived manifest, and checksums — re-runnably; and the bootstrap shim consumes it.

**Current behavior:** one tool has a tag-triggered release; no system payload, manifest, or bootstrap exists.

**Desired behavior:** the release workflow derives the binary list by command-tree scan, builds the platform matrix, packs the self-contained payload, derives the manifest via the policy walk, and publishes with checksums; publish is upload-or-verify (a re-run completes or byte-verifies, never forks); the build refuses to publish while any payload hook or workflow references a retired resolver, carries a source-toolchain step, or includes an undeclared file; the bootstrap shim fetch-verifies the pyxis binary against the pin documented in the install instructions, bash-3.2-clean, delegate-only, covered by a test.

**Scope:**
- The release workflow with the scan-derived matrix and the three-artifact packing.
- The release-precondition assertions and upload-or-verify publish.
- The bootstrap shim and its test.

**Out of scope:**
- Cutting the first live tag (an operator act after the plan's tasks land); channels or cohorts.

**Acceptance criteria:**
- [ ] A dry-run from a fixture tag yields the complete set (scan-derived binaries × platforms, self-contained payload, manifest, checksums) and a publish re-run converges upload-or-verify.
- [ ] Precondition fixtures (a retired-resolver reference, a source-toolchain step in the payload workflow, a bundle/lockfile mismatch) each refuse publish by name.
- [ ] The bootstrap shim on clean darwin/linux fixtures fetch-verifies pyxis against the documented pin and refuses a tampered binary with zero execution; the shim test passes on bash 3.2.

**Traces:** prd:installer/FR-1 · prd:installer/FR-2 · prd:installer/NFR-2 · prd:installer/NFR-5 · design:installer-v1#3.1 · design:installer-v1#3.10 · prd:installer/ABUSE-4

**Key interfaces:**
- The release workflow's job graph; the build-only manifest-derivation entry over `relmanifest.Walk`; the documented-pin convention the shim reads.

### T12 — Migrate the source repo: delegates, schema flip, parity preserved

| Field | Value |
|---|---|
| **Blocked by** | T2, T3, T4 |
| **Size band** | 100–200 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | the system scripts home (resolver scripts → delegates) · the system hooks (invocation lines) · the system config file (v2 flip, source mode) · the system skills (invocation-line sweep) |

**Goal:** the source repo consumes the new path itself: resolver scripts collapse to exec-grammar delegates, the config flips to schema v2 in source mode, and every source-repo suite stays green through the new path.

**Current behavior:** three per-tool resolver scripts carry their own fetch/parse logic (two verify nothing); hooks and skills invoke them directly; the config speaks the legacy per-tool schema.

**Desired behavior:** each resolver script is a one-line delegate into the exec grammar; hook shims and skill invocation lines speak the exec grammar; the config carries schema v2 with source mode, exercising the dual-read window's happy path; the full source-repo gate and test suite pass unchanged in verdict through the new invocation path.

**Scope:**
- The delegate collapse and the invocation-line sweep across hooks and skills.
- The source-repo config flip to v2 source mode.

**Out of scope:**
- Deleting the dual-read window (a later release closes it per the design's Q-3); any behavior change in the tools themselves.

**Acceptance criteria:**
- [ ] Every former resolver call site reaches its tool through the exec grammar; no script carries fetch or parse logic beyond locate-and-delegate.
- [ ] The full source-repo suite and gate are green with identical verdicts through the new path (the parity baseline for NR-1).
- [ ] Doctor on the source repo reports source mode and zero deprecation notices post-flip.

**Key interfaces:**
- The exec-grammar invocation convention for hooks and skills; the source-mode config contract.

**Traces:** prd:installer/NR-1 · prd:installer/NR-3 · prd:installer/NFR-5 · design:installer-v1#3.10

### T13 — The e2e harness: local combos, failure drills, repair proof

| Field | Value |
|---|---|
| **Blocked by** | T8 |
| **Size band** | 220–300 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/installer/e2e/ (new harness) · .github/workflows/ (matrix job wiring) |

**Goal:** the scratch-repo e2e harness proves the local half of the matrix: offline and carina combos, converge repair, idempotency, kill/re-run, fresh-clone self-heal, and the misparse suite.

**Current behavior:** no installer e2e exists; nothing exercises the packaged layout outside the source repo.

**Desired behavior:** the harness creates scratch repos, runs flag-driven installs from a fixture artifact source, and asserts: the offline combo (carina + local CI) completes every enumerated behavior in a network-denied environment including bundle-offline dependency install; injected drift per class repairs or refuses per the converge contract and a second run is a no-op; kill-at-random-step re-runs converge; a fresh clone of an installed repo self-heals online and refuses by name offline; misparse fixtures refuse on every verb; the removal test leaves operand content intact.

**Scope:**
- The scratch-repo harness and fixture artifact source.
- The network-denied offline leg; the repair, idempotency, and kill legs; the fresh-clone and misparse legs; the removal test.

**Out of scope:**
- GitHub-mode combos, wiring transcripts, parity, and budgets (T14).

**Acceptance criteria:**
- [ ] The offline combo passes in a network-denied environment with an egress scan showing zero operations post-fetch.
- [ ] The drift matrix (per-class injections × repair/refuse × second-run no-op) and the kill/re-run fixtures all converge as contracted.
- [ ] Fresh-clone, misparse, and removal-test legs pass on both linux and darwin runners.

**Key interfaces:**
- The harness's scratch-repo and fixture-source contracts; the drift-injection fixture set.

**Traces:** prd:installer/FR-8 · prd:installer/FR-13 · prd:installer/UB-5 · prd:installer/UB-9 · prd:installer/NFR-4 · design:installer-v1#3.11 · prd:installer/DOD-3

### T14 — The matrix tail: GitHub combos, parity, drills, budgets

| Field | Value |
|---|---|
| **Blocked by** | T8, T10, T11, T13 |
| **Size band** | 200–300 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/installer/e2e/ (extend) · .github/workflows/ (matrix jobs) |

**Goal:** the matrix completes: GitHub-mode combos through first-push green, the source-vs-installed parity suite, the supply-chain drills, and the time budgets.

**Current behavior:** the harness (T13) covers local combos only; no GitHub-mode, parity, drill, or budget evidence exists.

**Desired behavior:** GitHub-mode combos run the full wiring against a live scratch repo — labels, protection level, workflow — and the first push's required check completes green; the parity suite runs identical fixtures in the source repo (source mode) and an installed repo and diffs verdicts to zero divergence; the drill set (tampered binary, payload, manifest, bootstrap script; declared-operation audit) all refuse or verify as contracted; the install budget (≤ 5 min p95) and the per-invocation budget (≤ 100 ms p95) hold on reference runners, recorded.

**Scope:**
- The GitHub-mode legs (wiring + first-push) and the parity suite.
- The tamper-drill set and the two budget measurements.

**Out of scope:**
- New harness machinery beyond T13's contracts; non-GitHub hosts.

**Acceptance criteria:**
- [ ] Every GitHub-mode combo ends with the wiring audit equal to the declared table and the first push's required check green.
- [ ] The parity suite reports zero verdict divergence; the drill set refuses every tamper fixture with zero execution.
- [ ] Recorded timings hold ≤ 5 min p95 install and ≤ 100 ms p95 per invocation across the matrix runs.

**Key interfaces:**
- The parity fixture contract; the drill fixture set; the timing-record format the matrix emits.

**Traces:** prd:installer/FR-20 · prd:installer/FR-22 · prd:installer/UB-3 · prd:installer/UB-4 · prd:installer/NR-2 · prd:installer/NFR-1 · prd:installer/NFR-2 · prd:installer/NFR-3 · design:installer-v1#3.11 · prd:installer/DOD-5

## Coverage

| Requirement | Tasks |
|---|---|
| prd:installer/FR-1 | T1, T11 |
| prd:installer/FR-2 | T11, T14 |
| prd:installer/FR-3 | T2, T7 |
| prd:installer/FR-4 | T7 |
| prd:installer/FR-5 | T5 |
| prd:installer/FR-6 | T5, T13 |
| prd:installer/FR-7 | T7, T8, T9 |
| prd:installer/FR-8 | T8, T13 |
| prd:installer/FR-9 | T8 |
| prd:installer/FR-10 | T8, T10 |
| prd:installer/FR-11 | T8 |
| prd:installer/FR-12 | T8 |
| prd:installer/FR-13 | T3, T8, T13 |
| prd:installer/FR-14 | T6 |
| prd:installer/FR-15 | T8, T9 |
| prd:installer/FR-16 | T8 |
| prd:installer/FR-17 | T3 |
| prd:installer/FR-18 | T2 |
| prd:installer/FR-19 | T5, T10 |
| prd:installer/FR-20 | T14 |
| prd:installer/FR-21 | T3 |
| prd:installer/FR-22 | T3, T14 |
| prd:installer/UB-1 | T7 |
| prd:installer/UB-2 | T3, T13 |
| prd:installer/UB-3 | T3, T14 |
| prd:installer/UB-4 | T10, T14 |
| prd:installer/UB-5 | T5, T13 |
| prd:installer/UB-6 | T2 |
| prd:installer/UB-7 | T6, T8 |
| prd:installer/UB-8 | T2 |
| prd:installer/UB-9 | T4, T13 |
| prd:installer/UB-10 | T7 |
| prd:installer/NR-1 | T12 |
| prd:installer/NR-2 | T14 |
| prd:installer/NR-3 | T2, T12 |
| prd:installer/NFR-1 | T14 |
| prd:installer/NFR-2 | T11, T14 |
| prd:installer/NFR-3 | T3, T14 |
| prd:installer/NFR-4 | T6, T13 |
| prd:installer/NFR-5 | T11, T12 |
| prd:installer/NFR-6 | T4 |
