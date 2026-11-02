# Installer without the browser review runtime — the system release, the front-door orchestrator, and converge — technical design

| Field | Value |
|---|---|
| **Status** | Approved — operator approval, 2026-09-17 |
| **Owner** | @mdarabi |
| **Approver(s)** | @mdarabi (operator) — mandatory named reviewer: supply-chain acquisition and foreign-repo writes (a security surface) |
| **Date** | 2026-09-17 |
| **Relevant-when** | designing or changing the release unit, the release manifest or ownership classes, the `pyxis` binary and its exec grammar, installed-repo tool acquisition, converge/repair/doctor semantics, install-time seeding or GitHub wiring; implementing against any of these; superseding the installation/distribution shape |
| **Scope** | The installer PRD's concrete shape: the atomic system release (artifact set, hosting, manifest derivation, class policy), the `pyxis` front-door orchestrator (init/converge/repair/doctor/exec and the acquisition path), converge's ownership-class semantics, seeding, wiring, the config-surface binding, and the source-repo migration. **Not**: engine consolidation (tools as in-process subcommands — deferred, O-1 of the record), upgrade UX, existing-repo adoption, release channels/cohorts, uninstall (the basis's WR rows). |
| **Tracking** | https://github.com/agenticlab-ai/pyxis/pull/1199 — contract prerequisite draft |
| **Builds-on** | docs/tech-design/repo-layout-and-system-footprint.md · docs/tech-design/docgate-v1.md · docs/tech-design/codegate-v1.md · docs/tech-design/prepush-ref-awareness.md · docs/tech-design/local-work-tracker-v2.md |
| **Supersedes** | installer-v1 |
| **Serves** | prd:installer/FR-1 · prd:installer/FR-2 · prd:installer/FR-26 · prd:installer/FR-4 · prd:installer/FR-5 · prd:installer/FR-6 · prd:installer/FR-7 · prd:installer/FR-8 · prd:installer/FR-9 · prd:installer/FR-10 · prd:installer/FR-11 · prd:installer/FR-31 · prd:installer/FR-13 · prd:installer/FR-14 · prd:installer/FR-15 · prd:installer/FR-16 · prd:installer/FR-17 · prd:installer/FR-18 · prd:installer/FR-24 · prd:installer/FR-20 · prd:installer/FR-21 · prd:installer/FR-27 · prd:installer/FR-32 · prd:installer/UB-11 · prd:installer/UB-15 · prd:installer/UB-3 · prd:installer/UB-4 · prd:installer/UB-5 · prd:installer/UB-6 · prd:installer/UB-14 · prd:installer/UB-8 · prd:installer/UB-9 · prd:installer/UB-10 · prd:installer/NR-1 · prd:installer/NR-2 · prd:installer/NR-3 · prd:installer/NFR-9 · prd:installer/NFR-2 · prd:installer/NFR-8 · prd:installer/NFR-7 · prd:installer/NFR-5 · prd:installer/NFR-6 |
| **Security/privacy impact** | yes — binary acquisition and execution on user machines (supply chain), writes into repositories Pyxis does not own, and GitHub mutations under the user's ambient credential · reviewer: @mdarabi |
| **Template version** | tech-design-template/v1 |

## 0. Read to rule — the human reviewer's section (~3 min)

*The body is the contract; this section only routes attention — on any conflict the body governs; machines ignore it.*

**Judge these**

| # | The call | Could have gone | If you disagree |
|---|---|---|---|
| 1 | Remove the runtime, dependency stage, and new-release bundle (§3.3) | retain the browser runtime (#ALT-10) | veto the retirement and its implementation |
| 2 | Version the optional-hash manifest contract (§3.12) | silently relax v1 (#ALT-11) | require another explicit compatibility mechanism |

**Sanity-check the numbers** — no new performance bounds; inherited timings remain advisory under AP-29.

**The author's open questions** — no new questions; inherited §7 questions retain their owners.

**Skip the rest** — schema → docgate; supersession integrity → technical-design review; behavior → release and installer regression tests.

## 1. Problem

The installer and release tooling currently have a frozen obligation to install and ship the browser annotation runtime. The operator requested its removal, including its exclusive dependencies, skills, and prompts. Removing the implementation alone would contradict that obligation. This successor carries forward the installer's acquisition, ownership, convergence, seeding, wiring, and verification decisions while retiring the browser runtime under installer amendment 004. It also makes the manifest compatibility transition explicit: bundle-free releases use v2, and legacy v1 remains strict.

## 2. Decision

**Decision:** Pyxis ships as an **atomic system release** — one tag builds every engine binary for every supported platform from one commit, packs the payload without a browser-runtime dependency bundle, and emits a build-derived **release manifest** that pins every artifact by hash and ownership class; an operand's config pins exactly the system version and the manifest hash (`design-brainstorm:installer-tech-design#D-1`). The release is consumed through **`pyxis`, the front-door orchestrator**: one binary owning init, converge/repair, doctor, and the single acquisition-and-execution path — `pyxis exec <tool> -- <argv>` — behind which every skill, hook, and CI line in an installed repo invokes the engine (`#D-2`). Acquisition is **lazy and per-exec-verified**: every execution re-hashes the cached binary against the manifest before running it, cache misses fetch-verify atomically, and `pyxis converge` pre-warms the full tool set so the offline floor holds (`#D-3`). Install, re-run/repair, and (later) upgrade are one converge over ownership classes; the bootstrap shim that fetches `pyxis` itself is the one shell first-mover, minimized and drilled.

## 3. The design

### 3.1 The system release

**Decision:** One tag (`vX.Y.Z`) on the source repo triggers the release workflow: build every engine binary under the command tree — the set **derived by scan, never hand-kept** (AP-27) — for each supported platform; pack the payload archive (the system tree the manifest enumerates), with no browser-runtime dependency bundle (§3.3); generate the release manifest (per-file hash + ownership class, per-binary per-platform sha256, the system version); publish the complete set with checksums through the distribution door contracted in the installer PRD §5.5. Publishing is **upload-or-verify re-runnable** (AP-22): a re-run of a partially-published release completes it or verifies it byte-for-byte — never a second, different artifact set under the same tag.

The distribution door and source precedence follow installer FR-27, FR-29, FR-30, and FR-32: explicit flag, environment, repository configuration, then the recorded installation channel. Default-path artifact requests do not reach the source repository's release host. A missing or rejected door credential refuses by named environment variable and host, with no channel fallback. Preview and doctor report the effective source and whether it is default or overridden. The config-pinned **manifest hash remains the trust root** — a mirror can serve bytes, never choose them. These source rules carry forward amendment 003 rather than reopening the hosting decision.

A hotfix rebuilds and re-certifies the whole set — accepted cost of the atomic unit; the e2e matrix (§3.11) certifies exactly the set that ships, so no combination space exists (the incident-I2 class is structurally absent from distribution).

### 3.2 Ownership classes and the class policy

**Decision:** Every file a release installs carries one of three ownership classes — **engine-verbatim** (converge repairs byte-exact), **repo-owned** (converge validates against its closed schema, never rewrites), **generated** (converge regenerates) — assigned by an **authored class-policy file** committed in the source repo (the human input, AP-24); the release build walks the payload against it and derives the manifest (the aggregate, AP-27). A payload file the policy does not class **fails the release build** (DINV-6). The concrete schemas live in `docs/solution-shapes/installer-without-browser-review.md`.

### 3.3 The payload without a browser review runtime

**Decision:** Remove the vendored browser annotation runtime, its package manifests and lockfile, driver/setup/Markdown-rendering scripts, version pin, and dependency archive. Init performs no npm installation, and init, repair, doctor, and release preflight do not require Node.js or npm. No replacement review server or new dependency manager is introduced.

The payload continues to carry the independent Go plan viewer's Mermaid bundle and the generic feedback writer. Document review uses the conversation. Delete the doc-review skill entry and the former vendor README as the operator explicitly requires; land validated source-retirement evidence first, rather than retaining either path as a placeholder. The offline pre-fetch set consists of the platform binaries, release manifest, and payload archive; all fetched artifacts remain checksum-verified.

### 3.4 The orchestrator and the acquisition path

**Decision:** `pyxis` is the product face and the acquisition owner. Verbs: `init` (§3.5), `converge`/`repair` (one converge; repair is its name on a damaged tree), `doctor` (read-only), `exec` (the grammar below), plus `version`. Every installed-repo invocation of an engine tool is `pyxis exec <tool> -- <argv…>`:

- **Resolution**: tool → the release manifest (itself fetched into the gitignored cache and verified against the config pin before first use; the shape names its cache location) → expected sha256 for this platform → the gitignored binary cache. Cache hit: **re-hash before every execution**; mismatch or absence → fetch-verify (tmp file + atomic rename; override-aware) → exec. No path executes an unverified byte (DINV-1).
- **The grammar**: everything before `--` belongs to pyxis and parses fail-closed — a flag-like token in a positional slot, an unknown flag, or a missing `--` refuses with a usage error before any action (the incident-I3 discipline; basis UB-9). Everything after `--` passes argv-verbatim to the tool; pyxis never interprets it (DINV-5).
- **Source mode** (the source repo only, inherited rule): resolution falls through to running the tool from source; the same decision function, one mode bit — parity between installed and source repos is structural (basis NR-2), and the substrate's rule that source mode is legal only where the source lives is unchanged (DINV-8).
- **The first mover**: the bootstrap shim fetches and checksum-verifies `pyxis` itself against the pin **documented in the install instructions themselves** — the getting-started line carries the version and its sha256, an anchor deliberately independent of the release manifest (which pyxis is needed to parse) — before handing off; the one artifact no Go can gate (incident I1's bootstrap lesson) stays a minimal, pinned-HTTPS, bash-3.2-clean shim, covered by the tamper drill (basis ABUSE-6).
- **Cost**: the per-invocation overhead (config parse + one sha256 + execve) retains the **≤ 100 ms p95** measurement target on reference runners (AP-19). Report it separately from correctness; timing measurements are advisory under AP-29.
- **The cache's janitor ships with the cache** (AP-12): after a successful pre-warm, converge removes cached binaries and manifests whose version does not match the pinned system version — the resource type and its reaper arrive in the same change; doctor names any stale entry the janitor missed.

Consolidation later — tools as in-process subcommands — swaps exec for dispatch behind this same grammar with zero invocation-line churn (record O-1; the frozen sibling design's absorbs-without-reshaping anticipation).

### 3.5 Init: converge, seeding, and the end state

`pyxis init` runs one pipeline, every step idempotent and re-runnable (AP-22; kill-and-re-run converges — basis UB-5):

1. **Target check** — an empty target contains no content beyond repository metadata; where no git repository exists, init initializes one (basis FR-7). A non-empty or dirty target refuses before any write, names the conflicting content, and routes to the adoption path (UB-11). Unset committer identity is handled before any commit is attempted: a repo-local identity from the wizard's answers, or a named refusal (UB-10).
2. **Knobs** — the repository-destination question and at most four applicable knobs, interactive with recommended defaults or fully flag/answers-driven (FR-26/FR-4); derive wiring depth from the destination and suppress GitHub-only questions for a local-only destination; every answer, every path the engine consumes, the branch, and the pins land in the repo config (closed schema; §3.7).
3. **Converge** — materialize the payload per ownership class (§3.2); pre-warm the binary cache (§3.4).
4. **Seed** — instantiate the governance-skeleton templates (structure + fill-me guidance, zero invented content — basis NG-1) and the operand manual (the record's D-8: instantiated once from a shipped template with knob substitutions, **then repo-owned** — converge never rewrites it; doctor advises when the shipped template's version has moved); create the registries empty-but-valid (generated class where a tool is sole writer; repo-owned where the human appends); rollups stay correctly absent. **Where the argus knob is off, init instantiates no delegated-authority surface: the authority-contract instantiable is skipped and no authority contract file exists; the argus skill ships in the payload either way and reads the knob from the repo config at runtime, refusing with the named enablement pointer while off. Knob-on instantiates the authority contract at its floor tier (basis FR-16).**
5. **Hooks** — activate the hook path automatically (FR-9); the hook set itself is the payload's, unchanged (the push-hook ref handling stays the prior design's).
6. **Tracker + CI wiring** — carina: initialize the local store. GitHub: the declared operation set via the gh CLI (§3.6). Actions mode: install the workflow from the payload.
7. **Epoch** — write the epoch block: installed system version, the epoch commit, the manifest hash (record D-6). Write-only in v1: doctor reports it; the history-scoped consumers arrive with the adoption follow-up.
8. **Green proof + welcome** — run the gate; emit next-steps; create the welcome work item from its shipped template (tracker-wired modes), carrying the authorship pointers.

`pyxis doctor` is the read-only face of the same library: manifest drift by ownership class, environment health (prerequisites, hooks active, tracker reachable, CLI surfaces intact), the epoch, and unauthored-skeleton advisories — zero mutations. `--preview` on init/converge reports the exact write and external-operation set and performs none of it (FR-24).

### 3.6 GitHub wiring

**Decision (operator override of the REST recommendation, record D-5):** wiring shells out to the **gh CLI** — labels, workflow push, the chosen branch-protection level, the welcome item, optional repo creation. gh is a **GitHub-mode-only prerequisite** (absent → the named UB-14 remediation; the offline floor never touches it). The **declared-operation table** remains the single contract (AP-11): preview renders it, execution walks it, and the audit event records exactly what ran (EVT-4) — gh is transport. A permission gap completes every permitted step and emits the copy-paste hand-off (UB-4); transient API failures retry within a stated bound, then hand off the remainder.

### 3.7 The config surface and the pyxis-config binding

**Decision:** "The pyxis-config surface" is bound as a **concept** — the sanctioned, closed-schema, per-repo configuration surface (AP-8) — materialized in v1 as named files under the system tree: the repo config plus named policy files (the verification manifest, the class policy). This closes the basis's Q-1 with zero amendments: the sibling PRD's requirement row is directory-silent, the frozen sibling design's "surface" wording remains true, and the glossary term's parenthetical is updated in this introducing PR (record D-7). If the vision's per-repo extension directory ever materializes, relocation is a later feature's converge migration (record O-4).

The config schema (concrete form in the shape): the system pin pair, the captured paths (every directory the engine consumes, with defaults — a non-default value refuses with the named not-yet-configurable error, FR-18/UB-8, until the adoption follow-up delivers honoring), the branch, the knob answers, the artifact-source override, and the epoch block. Closed parse: unknown keys are errors (UB-6). **The captured path set is closed by construction: the schema is the authoritative list, every path key enforces the refuse-non-default posture, and a conformance check asserts the engine performs no configurable-path read that lacks a schema key — the enumeration audit feeds the schema before the first release, so the basis's zero-silent-ignore criterion is falsifiable against the schema itself.**

### 3.8 Observability

- **Events/logs** — structured slog from the pyxis verbs: the basis's EVT-1..EVT-5 (install completed, converge run with repaired-count and drift classes, doctor result, wiring audit, welcome emitted); local log + CI artifacts only.
- **Metrics** — none beyond the events; the basis's M1–M3 read from the matrix runs and the dogfood evidence.
- **Alerts** — none: a red gate or a red matrix is the alert (inherited posture).
- **Debugging surface** — `pyxis doctor` (drift by class, environment, epoch) and `--preview` (the would-be write/operation set).
- **Trace/correlation** — each run carries a run id on every event; **zero telemetry egress** (DINV-7; the platform loop is a later program).

### 3.9 Security & privacy

Matches the header row — this is the design's largest surface, three legs:

1. **Supply chain (inbound)** — every fetched artifact is checksum-verified against the config-rooted chain before any execution, on every path: the bootstrap verifies pyxis; pyxis verifies tools per-exec against the manifest; the manifest is verified against the config pin (DINV-1, DINV-2). The artifact-source override changes bytes' origin, never their acceptance; it is loudly surfaced in preview and doctor. The bootstrap shim is the one ungated first-mover: pinned-HTTPS, minimal, drilled with tamper fixtures (basis ABUSE-1/ABUSE-6, UB-3). Fetched binaries are cached gitignored; nothing is committed.
2. **Foreign-repo writes** — init writes only per the manifest and the seeding set; preview shows the exact set first (FR-24); repo-owned content is never rewritten (DINV-3); the removal test the substrate defined still passes in an installed repo (A-10).
3. **Credential use (outbound)** — GitHub mutations run under the user's ambient gh authentication, bounded by the declared-operation table and recorded in the audit event (ABUSE-3); no credential is stored; zero egress beyond artifact fetch and the declared wiring set (the basis's §5.4 posture).

### 3.10 Migration in the source repo

The source repo is not an installed repo (self-converge is the record's O-3-adjacent deferral, basis WR-3); it migrates only where the release machinery demands:

- **Config schema window** — the schema version bumps; one release **dual-reads** every per-tool pin section present in the live config (the docgate and codegate sections today) alongside the new system pin pair, with a doctor warning; the next refuses the legacy keys. This *narrows* the substrate design's spelled-out schema without contradiction: its binding rule — no unpinned execution, pins rooted in committed config, source mode only where source lives — remains true throughout; only the key layout under the closed, versioned schema evolves, exactly the drift its own risk row anticipated with the version handshake. The prior tool-release tags and artifacts stay hosted; their workflow is superseded by the system-release workflow.
- **Shims** — the per-tool resolver scripts collapse to one-line delegates into `pyxis exec` (two of them verify nothing today — the live gap this closes); the source repo keeps source mode, so its delegates fall through to source execution unchanged in behavior (NR-1).
- **The CI workflow** — the payload ships the **installed-repo variant** of the gate workflow: it invokes the gate through the exec grammar and carries no source-toolchain step (no Go setup, vet, or race suite — those belong to the source repo's own CI, which keeps its own workflow file). Both variants exist before the first system-release tag.
- **Release preconditions, asserted by the build** — the release workflow refuses to publish while any payload hook or workflow still references a retired resolver script or carries a source-toolchain step; converted shims and the installed workflow variant are preconditions of the first tag, checked mechanically (the green-merge-is-not-a-working-artifact lesson).
- Landing follows the stacked-train discipline (delete heads, verify mainline ancestry — incident I2) wherever the migration spans dependent changes.

### 3.11 Testing strategy

Seams binned per the template menu: the **converge/manifest/class core, config schema, epoch, and the operation table** — in-process (hermetic table-driven tests; golden manifests; property tests for DINV-3/DINV-4). The **acquisition path** — local-substitutable: a file:// artifact source serving fixture releases (tamper, truncation, wrong-platform, missing-pin fixtures — UB-15/UB-3; the override leg — FR-27). The **wiring leg** — remote-but-owned behind the operation table: a fake gh transcript asserts the executed set equals the declared set (ABUSE-3); live gh runs only in the matrix's GitHub-mode combos. The **e2e matrix** — the deployment context (AP-14): scratch repos, flag-driven, tracker × CI-mode × platform, the offline combo in a network-denied environment, kill/re-run, repair, fresh-clone self-heal, misparse fixtures; the parity suite runs identical fixtures in the source repo (source mode) and an installed repo and diffs verdicts (NR-2, the basis's §3.3 guardrail).

### 3.12 Versioned manifest compatibility and retirement proof

**Decision:** New releases use release-manifest/v2 and omit the dependency-bundle hash. Current readers support both v2 and legacy v1; v1 still requires a valid bundle hash. Verification checks any pinned legacy bundle and refuses unpinned extra artifacts. Packaging refuses stale dependency archives in the output directory. Old v1-only readers reject v2 explicitly; users upgrade through `pyxis upgrade` or the bootstrap installer before repinning. The exact wire object is carried in the paired solution shape.

The successor carries forward the predecessor's acquisition, ownership classes, seeding, wiring, config, diagnostics, security, and migration decisions. Only the browser-runtime obligations and the manifest-version transition are changed here. Later compatible designs and PRD amendments continue to govern their respective mechanisms. Inherited numeric performance budgets are measurements under AP-29, never correctness gates.

- **DINV-9** — installation, repair, diagnosis, and release packaging do not require the retired browser runtime, Node.js, npm, or its dependency archive.
- **DINV-10** — supporting bundle-free v2 manifests never relaxes v1's required bundle-hash validation or verification of a pinned artifact.

## 4. Alternatives considered

| Alternative | Why rejected |
|---|---|
| Per-tool release tags ×N + a certified-tuple record (`design-brainstorm:installer-tech-design#ALT-2`) | the combination space is real: every bump is an uncertified tuple or a full re-matrix; cross-tool schema skew becomes a live bug class; rebuilds the incident-I2 badge-summing shape at the distribution layer |
| Build-once, promote to a dist repo with channels-as-data (`#ALT-3`) | the cleanest staged-release runway, but a promote tool + second repo + two green surfaces is platform-loop substrate the basis's appetite excludes; the override + manifest-hash trust root keeps it reachable later |
| A tenth thin binary; shell resolvers keep runtime resolution (`#ALT-4`) | leaves the critical checksum floor split across shell copies — two in-tree resolvers already verify nothing, and the drift class is legislated today by comment, not code |
| Consolidate the engine into one binary now (`#ALT-5`) | the whole invocation surface churns and two frozen designs need superseding — none of it needed for the basis's targets; it smuggles a second architecture decision (the tool CLI topology) into this design against the one-decision rule, its cross-surface migration is the incident-I2 shape, and the sibling design's own rejection rationale (any tool's bug forces a shared release) applies at full scale |
| Verify at converge, exec direct (`#ALT-8`) | opens a tamper window between converge and doctor against the basis's critical "on any path" guardrail; fresh clones dead-end until converge |
| A standalone micro-resolver binary (`#ALT-9`) | a second front door and a resolver↔product skew class, against the frozen one-command face, for isolation the per-repo pin already provides |
| Hand-authored release manifest (record D-4 reasoning) | an authored aggregate is the drift pattern (AP-27); the build derives it from the authored class policy or the release fails |
| REST wiring with token discovery (the displaced recommendation) | operator override (record D-5): fleet consistency and gh's auth path outweigh the dependency-free wire; the operation table keeps preview/audit intact either way |
| Retain the browser runtime and dependency stage (#ALT-10) | conflicts with the operator's explicit removal decision and keeps the exclusive Node/npm requirements |
| Omit the hash while continuing to write release-manifest/v1 (#ALT-11) | silently changes a required field contract and makes old readers reject a supposedly unchanged schema |

## 5. Risks

| ID | Risk | Impact | Mitigation | Verification / evidence |
|---|---|---|---|---|
| R1 | The exec grammar churns after adoption — every surface speaks it | repo-wide line edits, the one hot contract (AP-23) | the grammar is deliberately minimal (`exec <tool> -- <argv>`); consolidation later reuses it unchanged (O-1); any change is a superseding design | grammar stated as a binding element (DINV-5); parity suite pins it |
| R2 | Hotfix blast radius — one fix re-releases everything | heavier release cadence | accepted at D-1; static Go builds are cheap; the matrix is the certification either way | release dry-run time tracked in CI |
| R3 | Payload surfaces remain secretly source-coupled and break only installed | the basis's top risk | the parity suite (NR-2) + the portability audit of every skill invocation line (basis P-8); the offline combo catches toolchain/network leaks | A-6; the matrix |
| R4 | gh version/output variance breaks wiring parsing | flaky GitHub-mode installs | a stated minimum gh version (Q-2); the operation table isolates gh behind one seam; transcripts fixture the expected surface | wiring fixtures + a live matrix combo |
| R6 | The first-mover shim is the one unverified execution | supply-chain foothold | minimal pinned-HTTPS shim; checksum set fetched before executing any fetched byte beyond itself; tamper drills (ABUSE-6) | drill fixtures in DOD-11's set |

## 6. Acceptance

| ID | Acceptance criterion | Verify | Required evidence | Traces to |
|---|---|---|---|---|
| A-1 | a release dry-run from one tag yields the complete artifact set — every scan-derived binary × platform, payload, manifest, checksums — and re-running publish converges (upload-or-verify) | Test | release-workflow dry-run record | prd:installer/FR-1 · §3.1 |
| A-2 | the bootstrap on clean darwin/linux runners fetch-verifies pyxis and hands off; tampered binary, payload, legacy pinned dependency-bundle, and bootstrap-script fixtures are all refused with zero execution | Test | drill records | prd:installer/FR-2 · UB-3 · §3.9 |
| A-3 | the full e2e matrix (tracker × CI mode × platform, offline included) passes every correctness assertion independently of duration; record NFR-9 duration measurements separately as advisory evidence | Test + advisory measurement | matrix run link and separate advisory duration record | prd:installer/FR-8 · FR-13 · NFR-9 · NFR-2 |
| A-4 | injected drift per ownership class is repaired byte-exact (engine-verbatim), regenerated (generated), or refused-by-name (repo-owned schema-invalid); a second converge run mutates nothing; a config carrying pin keys beyond the system pair — or legacy per-tool sections after the migration window — is refused by name | Test | repair + config fixture suite | prd:installer/FR-5 · FR-6 · #DINV-2 · #DINV-3 · #DINV-4 |
| A-5 | the offline combo completes init, converge, doctor, resolution, the hook gate, and a skill invocation with zero network operations post-fetch, in a network-denied environment — proving the zero-egress invariant | Test | network-denied run record + run-log egress scan | prd:installer/FR-13 · §3.3 · #DINV-7 |
| A-6 | the parity suite shows zero verdict divergence between the source repo (source mode) and an installed repo on identical fixtures | Test | parity suite result | prd:installer/NR-2 · §3.11 |
| A-7 | misparse fixtures (flag-like positional, unknown flag, missing `--`) on every pyxis verb refuse with zero writes and zero external operations | Test | misparse fixture suite | prd:installer/UB-9 · #DINV-5 |
| A-8 | the recorded wiring operation set equals the declared table in every GitHub-mode combo; a permission-limited run applies the permitted subset and emits the working hand-off | Test | wiring transcripts | prd:installer/FR-10 · UB-4 |
| A-9 | a fresh clone of an installed repo self-heals on first invocation online and refuses by name offline; record per-invocation overhead against the advisory ≤ 100 ms p95 target separately | Test + advisory measurement | clone fixtures + timing record | prd:installer/FR-17 · §3.4 |
| A-10 | the substrate's removal test passes in an installed repo: deleting the system tree and pointers leaves the operand's content complete and readable | Test | scripted removal check | §3.9 · the substrate's inherited invariant |
| A-11 | a payload file absent from the class policy fails the release build with the file named | Test | release-build fixture | #DINV-6 · §3.2 |
| A-12 | init in a bare directory and in a git-initialized empty repo both end green; unset committer identity yields a repo-local identity or the named refusal, never a failed commit | Test | entry-state fixtures | prd:installer/FR-7 · UB-10 |
| A-13 | init, repair, doctor, packaging, and the clean-room release entrypoint complete their FR-31 outcomes without Node.js or npm; inspect the payload and fresh installed tree for absence of the retired skill, Lavish vendor directory, exclusive scripts, and activation prompts; historical records and the independent feedback writer's stable source identity remain, and the independent viewer and feedback writer remain usable | Test | installer and release-entrypoint results, payload inventory, fresh installed-tree absence check, renderer and feedback regression results | prd:installer/FR-31 · #DINV-9 |
| A-14 | bundle-free manifests use v2; v1 without its hash refuses; legacy pinned bundle tampering and unexpected artifacts refuse | Test | schema round-trip and release-verification regression results | prd:installer/FR-1 · FR-17 · #DINV-10 |

## 7. Open questions

| ID | Question | Owner | Blocks | Status | Resolution |
|---|---|---|---|---|---|
| Q-1 | The interactive prompt wording, ordering, and recommended defaults for the destination question and applicable knobs | implementation | nothing — FR-26 fixes question scope; presentation is UX | open | — |
| Q-2 | The minimum gh version and the exact output/exit contract the wiring transcripts pin | implementation | wiring fixtures | open | — |
| Q-3 | The dual-read schema window's length (one release or two) before legacy pin keys refuse | operator, at the first system release | the source-repo migration's last step | open | — |
| Q-4 | Unsigned-binary posture: v1 ships unsigned static binaries (the curl path carries no quarantine); notarization/signing is platform-loop-era work | operator | nothing in v1 | open | — |

## Principles

- **AP-1 / AP-4 / AP-25 — honored.** The engine's mediums and the repo's agent surfaces stay swappable: the payload is operand-agnostic, vendor dirs stay pointers, and every CLI surface speaks the same exec grammar.
- **AP-6 / AP-8 — honored.** The manifest, config, class policy, and operation table are closed, versioned, compact contracts; the per-repo surface is the one sanctioned extension point, bound in §3.7.
- **AP-9 — honored (not precluded).** One coarse release stream now; channels/cohorts remain reachable behind the same trust root (O-2).
- **AP-10 / AP-11 — honored.** Resolution, verification, drift, and the operation table are single Go implementations; the shell resolvers and their comment-legislated parity die.
- **AP-13 / AP-14 / AP-26 — honored.** Only hash-verified artifacts execute; certification is the e2e matrix run on the exact shipped set; goal state is observed (gate exit, mainline ancestry at migration), never badge-summed.
- **AP-17 — honored.** Every ambiguous or failed verification path refuses closed with zero execution and zero partial trust.
- **AP-19 — strained, stated.** The per-invocation verify+exec hop is a deliberate ~tens-of-ms budget bought for the on-any-path floor (A-9); idle remains free.
- **AP-12 — honored.** The binary/manifest cache ships with its janitor in the same change: converge's pre-warm reaps entries off the pinned version; doctor names stragglers (§3.4).
- **AP-20 / AP-22 / AP-23 / AP-27 — honored.** Blast radii stay per-tool at runtime; converge, exec, and publish are idempotent and raceable — init-from-nothing is single-operator by contract (two concurrent inits on one empty directory sit outside the concurrency claim; the residual is a redundant, harmless git init); the build list and manifest are derived aggregates; the one hot grammar line is minimal and frozen by design.
- **AP-28 — honored by scoping.** Converge/doctor events are operational telemetry, re-derivable by re-running the verb on the same tree — the routine-projection exclusion, not the retained improvement-fuel set; the surfaces that teach here (matrix runs, drills, dogfood evidence) persist as CI artifacts and tracker records. Should the operator rule repair snapshots teach, a committed event-record path is an additive follow-up.
- **AP-24 — honored.** The class policy is the human-authored input; the manifest is derived from it; nothing regenerates governing inputs from installed reality.
- **RW-1 / RW-3 / RW-4 / RW-5 / RW-6 — honored.** Behaviors ported, not the shell mechanisms; the substrate precedes the installer; one decision here (the installation/distribution shape — its subordinate choices change together); the source-repo migration is dual-read and evidence-gated.
- **AP-7 / AP-29 — honored.** The frozen predecessor remains unchanged; this successor records the retirement. Inherited performance figures are advisory measurements, not blocking correctness conditions.
- **Deviation:** none.
