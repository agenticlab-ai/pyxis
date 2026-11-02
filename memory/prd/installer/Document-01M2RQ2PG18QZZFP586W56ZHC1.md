# Amendment 004 — retire the browser review runtime

| Field | Value |
|---|---|
| **Amends** | installer (chain through 003) |
| **Status** | Approved |
| **Owner** | @mdarabi |
| **Date** | 2026-09-17 |
| **Learned-from** | operator-decision:2026-09-17-remove-lavish · review-thread:https://github.com/agenticlab-ai/pyxis/pull/1199#discussion_r4041050495 |
| **Blast radius** | tier: contract-changing; touches: FR-12, FR-28, UB-2, UB-7, NFR-1, DOD-4, DOD-9; reason: retire the mandatory browser runtime and its exclusive dependencies, preserving artifact verification and the independent review tools; section replacements include informative human-only §0 maintenance |
| **Template version** | amendment-template/v2 |

## Operations

### op: retire FR-12

Reason: The operator requested removal of Lavish and all of its exclusive dependencies. Document review continues through the conversation and requires no browser annotation runtime.



### op: add FR

Reason: Prevent the retired runtime from returning as an implicit install, doctor, or release prerequisite.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-31 | When Pyxis is installed, repaired, diagnosed, or released, the system shall perform those operations without a browser document-review runtime, Node.js, npm, or a review-runtime dependency archive, and shall omit the retired runtime, its skill entry, vendor directory, scripts, and activation prompts from the shipped payload. | removing the feature also removes its exclusive operational dependencies | Must | med | with Node.js and npm absent, init and doctor complete, repair restores the expected owned files, and release packaging produces verified binaries, payload, manifest, and checksums; the clean-room entrypoint separately passes preflight; payload and fresh-install inspection finds no doc-review skill entry, lavish-axi vendor directory, or Lavish activation scripts/prompts; historical records and stable feedback-source identifiers are not activation prompts; new manifests omit the runtime dependency archive; standalone feedback capture writes its expected record and Go plan rendering emits its expected HTML | Test | G1, G2, G3, F1, F2, F3, F4 |

### op: supersede FR-28

Reason: The default artifact-source boundary remains unchanged; its fit criterion must enumerate the artifact set after dependency installation is retired.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-32 | Where no artifact-source override is configured, the system shall resolve every fetched release artifact from the distribution door declared in §5.5, and shall issue no artifact request to the source repository's release host. | consumers must be servable without an account on, or a paid seat in, the organisation that publishes the release | Must | med | with no override configured, every resolved manifest, binary, and payload URL carries the door's host; a network transcript of a default-path install contains zero requests to the release host | Test | G3, F1, F3 |

### op: supersede UB-7

Reason: Git remains an installer prerequisite; the retired browser runtime is not a prerequisite or a doctor failure.

| ID | If… (trigger) | Then the system shall… | User sees | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|---|
| UB-14 | a required installation prerequisite, including git, is missing | name the missing prerequisite and its remediation, claim no success, and leave doctor red; absence of Node.js or npm alone is not a failure | a named prerequisite failure when a live prerequisite is absent | Must | med | removing git produces its named remediation and a doctor finding; removing only Node.js and npm permits init and doctor to complete | Test | FR-31, FR-14, F2 |

### op: supersede UB-2

Reason: Re-runnability remains required for artifact fetch failures; dependency installation no longer exists.

| ID | If… (trigger) | Then the system shall… | User sees | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|---|
| UB-15 | the network fails mid-artifact-fetch | trust no partial artifact, leave a re-runnable state, and converge to the same end state on re-run | a named failure; re-run completes | Must | high | after an injected mid-flight fetch failure, re-run ends green with no manual cleanup | Test | FR-6, F1, F2 |

### op: supersede NFR-1

Reason: Remove the retired stage and preserve the numerical targets as advisory measurements under AP-29, as required by review:installer-004#FND-4.

| ID | Area | Requirement | Fit criterion | Verify |
|---|---|---|---|---|
| NFR-9 | Performance | Init duration shall be recorded on reference CI runners against the existing advisory install targets; timing shall not determine a correctness verdict. | record p95 and p99 across release-candidate matrix runs against targets of p95 ≤ 5 min and p99 ≤ 8 min; results are advisory and cannot fail the release gate | Measurement |

### op: supersede DOD-9

Reason: The blocking acceptance row must include new live requirements without retaining a fixed numeric range.

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-13 | All live Must FRs pass their fit criteria | every live Must FR in §5.1, excluding superseded and retired rows | Test | CI run link | yes |

### op: supersede DOD-4

Reason: Preserve the blocking matrix correctness criterion and separate the NFR-9 measurements from its verdict, under AP-29 and review:installer-004#FND-4.

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-14 | The release-candidate e2e matrix (tracker × CI mode × platform, offline combo included) is 100% green; NFR-9 measurements are recorded separately and never determine this verdict | M1, M2, NFR-9, NFR-2 | Test + advisory measurement | matrix run link and advisory duration record | yes — matrix correctness only |

### op: supersede §4.1

Reason: review:installer-004#FND-3 — remove the runtime from the live payload description so it agrees with FR-31.

Pyxis becomes a **versioned, released product**, and installing becomes **converging a repository onto a pinned release**. A release ships every engine binary per supported platform, the payload (the system tree: skills, templates, authoring guides, scripts, hooks, and independently consumed viewer assets), and a **release manifest** enumerating every installed file with a hash and an **ownership class** — engine-verbatim (converge repairs it byte-exact), repo-owned (converge validates, never rewrites), generated (converge regenerates). One curl-able bootstrap line fetches the pinned `pyxis` binary, verifies it, and hands off to `pyxis init`: five knobs asked (or flags), answers persisted into the repo config together with every path and pin the engine consumes, the tree converged, governance skeletons and registries seeded, tracker/CI/hooks/CLI surfaces wired — ending green, with a welcome path into authoring the vision and the first feature. Re-running any of it is the same converge: idempotent on a clean repo, repairing on a damaged one. `pyxis doctor` is the read-only face of the same manifest.

| Option | Sketch | Why not chosen |
|---|---|---|
| A (chosen) | versioned release + manifest converge; the wizard is the front end | — |
| B | copy-once scaffolder now, converge/manifest later | rebuilds itself the day upgrade or adoption arrives; idempotency and repair untested until then (`brainstorm:installer` Framings) |
| C | portability-program-first: un-hardcode everything, ship the installer as its acceptance harness afterwards | slowest route to any usable install; the same items land as this feature's acceptance criteria instead (`brainstorm:installer` Framings) |
| Do nothing | — | the factory stays trapped in its own repo (§2.6) |

### op: supersede §2.4

Reason: Remove retired-runtime assumptions from this section while preserving the remaining contract.

| Claim | Source | Confidence |
|---|---|---|
| Of the engine binaries under `cmd/`, only docgate has release machinery (tag-triggered matrix + checksums); no installer, Makefile, or goreleaser exists | `brainstorm:installer#F-1` (repo scan) | High |
| A fetch-verify-cache resolver seam already exists for docgate, with a source-mode fallback and a hardcoded release URL | `brainstorm:installer#F-2` (the docgate resolver shim; CI workflow) | High |
| Targets consuming released binaries makes the module-path import surface a non-blocker; the true un-hardcode list is short (branch refs, label default, one trailer constant) | `brainstorm:installer#F-3` (repo grep) | High |
| A fresh empty repo fails docgate absent the governing/system doc set; rollups are validly absent pre-first-ship | `brainstorm:installer#F-4` (gate checks; repo config) | High |
| Tracker selection is flag-only today; `carina init` is the only installer-adjacent verb; no doctor verb exists anywhere | `brainstorm:installer#F-5` (cmd argument-parsing scan) | High |
| `pyxis-config/` is contracted (vision §8; the implementation-verification PRD's manifest home) but absent on disk | `brainstorm:installer#F-6` | High |
| The cross-CLI skill surface is symlink-solved (vendor dirs are pointers into the system tree, AP-25) | `brainstorm:installer#F-7` | High |
| The installable engine surface in the system tree is ~56 policy/template/skill/script files plus vendor trees; hooks activate via a manual one-time git config step | `brainstorm:installer#F-8` | High |

### op: supersede §4.2

Reason: Remove retired-runtime assumptions from this section while preserving the remaining contract.

- **In (plan of record)** — the release artifact set (binaries per platform + payload + manifest + checksums) and its pipeline; the bootstrap line; `pyxis init` with the five knobs (tracker gh|carina · CI mode local|actions|both · GitHub wiring depth incl. protection level · CLI surfaces auto|explicit · argus on|off), full flag parity, and complete config capture (paths, pins, branch); converge/repair with ownership classes; seeding (skeletons, registries, taxonomy dirs, hooks activation, `carina init`, workflow push, labels/protection); pinned-binary resolution in installed repos; runtime-free document review (FR-31); the offline floor; `pyxis doctor`; the welcome path; the preview mode; the e2e matrix.
- **Future (believed next, not committed)** — existing-repo adoption, upgrade UX, self-converge of the pyxis repo, uninstall, org sweep, other hosts, App wiring, the platform loop, non-default branches, and path honoring (WR-1..WR-10).
- **Out** — governing-content generation, source distribution, converge merge intelligence (mirrors §3.4 NG rows).

### op: supersede §4.3

Reason: Remove retired-runtime assumptions from this section while preserving the remaining contract.

*A terminal product: "states" are the CLI's reporting states. Accessibility: n/a — terminal tooling, no UI pages/processes in scope. Localization: n/a — developer-facing, English-only v1.*

**Flow F1 — fresh install, GitHub mode (serves J1)**

| Step | User does | System responds | States covered | Design |
|---|---|---|---|---|
| 1 | runs the bootstrap line in an empty directory | fetches the pinned binary, verifies checksum, hands off to init | ideal · error (checksum, network, platform) | tech design |
| 2 | answers the five knobs (or none: flags) | records answers; reports the converge plan | ideal · error (dirty dir, prereqs missing) | tech design |
| 3 | waits ≤ 5 min | converges payload, seeds docs/registries, activates hooks, wires labels/protection/workflow, folds and gates | ideal · partial (permission hand-off) · error | tech design |
| 4 | reads the end report | green confirmation + welcome next-steps + welcome work item | ideal | tech design |

**Flow F2 — offline install, carina mode (serves J1; proves FR-13)** — identical shape with zero network after artifact fetch: tracker = local store via `carina init`, CI = hooks only, no GitHub calls of any kind.

**Flow F3 — repair (serves J2)** — `pyxis init` (or the repair verb) on a damaged install: reports the drift set by ownership class, repairs engine-verbatim and generated files, refuses to touch repo-owned content, ends green or names what it cannot repair.

**Flow F4 — doctor** — read-only: manifest drift + environment health (required prerequisites present, hooks active, tracker reachable, CLI surfaces intact, skeletons still unauthored as an advisory), zero mutations.

- **Empty** — bootstrap into an empty dir is the ideal path; where no git repository exists init initializes one, and on a repo with no commits it creates the initial structure and first commit-able tree.
- **Partial** — a permission-limited run (e.g. no admin for protection) completes every permitted step and emits a copy-paste hand-off for the remainder (UB-4).
- **Error** — every refusal names the cause and the remediation; no partial write is left behind un-reported (UB-1, UB-5).

### op: supersede §4.4

Reason: Remove retired-runtime assumptions from this section while preserving the remaining contract.

**The operator's next project.** A new product idea deserves its own repo. One bootstrap line, five answers (GitHub tracker, both CI modes, auto CLI detection, argus on), under five minutes: labels and protection set, docs-gate workflow live, hooks active, welcome issue open. The first `/feature-brainstorm` runs that afternoon; nothing was copied from the pyxis repo.

**The offline adopter.** An engineer on a locked-down network fetches the release artifact once, runs init with carina + local-CI flags, argus off. No GitHub account is ever touched; the gates run in hooks; work items live in the local store. The repo is exactly as governed as the operator's.

**Death mid-install.** The network dies during artifact fetch. The tree holds a half-converged state; the user re-runs init; converge reports what exists, completes the remainder, ends green — no cleanup archaeology (UB-15, UB-5).

**The hand-damaged repo.** Someone "cleans up" a hook shim and edits a shipped template. Doctor names both drifts; repair restores them byte-exact from the manifest; the repo's own config and docs are untouched (J2, FR-5).

**Decision:** Solution gate — zero TBDs may pass.

| Reviewer | Role | Date | Verdict |
|---|---|---|---|
| @mdarabi | operator (via the introducing PR's review) | — | — |

### op: supersede §5.4

Reason: Remove retired-runtime assumptions from this section while preserving the remaining contract.

| Entity / field | Meaning | Source of truth | Retention | Integrity rules | Privacy class |
|---|---|---|---|---|---|
| repo config | the five knob answers + every captured path/pin/branch | the installed repo's git | repo lifetime | closed schema; unknown keys fail (UB-6) | internal |
| release manifest (installed copy) | the pinned release's file inventory: hash + ownership class per file | the release artifact; copied into the repo at converge | until next converge | converge/doctor read it; never hand-edited | public (ships in the release) |
| seeded skeletons + registries | the governed-doc structure the gates require | the installed repo's git | repo lifetime | content is the human's; structure from templates (NG-1) | internal |
| local work-item store | carina's files-as-truth store, when chosen | the installed repo / its store root | repo lifetime | carina's own invariants | internal |
| machine release record | the installed release pin and the channel it was installed from (FR-30) | the installing machine's user configuration directory — the one installer write outside the target repo | until the next install or upgrade rewrites it | written only by install and upgrade; outranked by a flag, the environment, and the repository's configured source (FR-30); its loss costs only flag-free resolution, never correctness | internal |
| door credential | the artifact-source credential (FR-29) | the operator's environment, per process | never retained — read per fetch, never written to disk, repo config, or the machine record | attached host-locked over TLS to the artifact-source host alone; never logged and never echoed by preview or doctor | secret |

- **Migration** — none: v1 targets empty repositories (UB-1, WR-1).
- **Parallel-run / cutover** — n/a (nothing pre-exists in the target).
- **Rollback** — every write inside the target is in its git; un-merging or reverting the install commit(s) restores the prior state. The two artifacts outside it are both disposable: the bootstrap binary cache, and the machine release record, which a reinstall or upgrade rewrites and whose deletion only returns `init` and `upgrade` to needing their pin and channel passed explicitly.
- **Decommission** — WR-4 (uninstall); until then, removal is documented as git revert + cache delete + machine-record delete.
- **Privacy check** — no personal data is created or read. One secret is handled: the door credential, read from the environment per fetch and never persisted, logged, or echoed (§5.3, FR-29). No DPIA trigger.
- **Egress** — **zero telemetry** (WR-8: keys reserved, nothing sent). Outbound traffic is exactly: release-artifact fetch (in), carrying the door credential to the artifact-source host alone (FR-29); and — GitHub mode only — the declared wiring operations (out). The offline combo performs none of the above after fetch (FR-13).

### op: supersede §5.5

Reason: Remove retired-runtime assumptions from this section while preserving the remaining contract.

| Interface | Direction | Purpose | Contract | Failure modes | Owner |
|---|---|---|---|---|---|
| release artifact host — the distribution door `https://dist.pyxis.agenticlab.ai` by default; the source repository's Releases remain the canonical publish target and a supported override | in | fetch binaries + payload | versioned URLs + checksums (FR-1, FR-17); the effective source is reportable (FR-27); the door credential is supplied by the environment and host-locked over TLS (FR-29) | unreachable → named failure, re-runnable (UB-15); tamper → refuse (UB-3); credential absent, malformed, or rejected → named refusal with no fallback (UB-13) | this feature |
| GitHub API (labels, protection, workflow, welcome item) | out | the wiring knob | the declared operation set (FR-10) | missing permission → hand-off (UB-4); absent credential → skip/offline | this feature |
| git (hooks path, initial structure) | both | hook activation + the installed tree | hooks enforce the same checks as the source repo (FR-9) | missing git → UB-14 | this feature |
| agent CLI surfaces (claude/codex/cursor/opencode dirs) | out | skill discovery in installed repos | pointers into the payload (FR-11, AP-25) | absent CLI → auto-detect skips; explicit choice materializes regardless | this feature |

### op: supersede §6.2

Reason: Remove retired-runtime assumptions from this section while preserving the remaining contract.

| Dependency | Type | Needed by | Status | Owner |
|---|---|---|---|---|
| the distribution door (object storage behind a credentialed front door) | vendor | default artifact resolution | available; the release already mirrors the full artifact set per tag | this feature |
| GitHub Releases (artifact hosting) | vendor | Release machinery; the canonical publish target and the override channel | available (docgate precedent) | this feature |
| the door credential on consumer machines and in operand CI | user prerequisite | every default-path fetch | user-provided; issued with the install instructions (UB-13) | user |
| darwin + linux CI runners for the matrix | system | Init end-to-end | available; darwin runner cost noted (R3) | this feature |
| GitHub credential for wiring (optional) | user prerequisite | GitHub-mode installs | user-provided (UB-4) | user |
| parked technical constraints handed to the design | design input | design gate | `brainstorm:installer#P-1..#P-8` | technical design |

### op: supersede R5

Reason: Replace the retired dependency-installation risk with the manifest migration risk using explicit row lineage; a whole-section replacement does not update this existing row's source-map identity.

| # | Risk / assumption / rabbit hole | Likelihood | Impact | Mitigation or test | Owner |
|---|---|---|---|---|---|
| R8 | older v1-only manifest readers reject the new bundle-free release schema | M | M | upgrade the CLI before repinning; regression tests preserve strict legacy v1 validation and new v2 parsing | design |

### op: supersede §6.5

Reason: Remove retired-runtime assumptions from this section while preserving the remaining contract.

- **Flag** — none: installation is invocation-gated (nothing happens to anyone who does not run it).
- **Kill switch** — pull or deprecate the release; the bootstrap fetches only pinned, checksummed versions, and a prior pin remains installable (FR-17). An installed repo is unaffected until it chooses to converge to a newer pin.
- **Experiment** — none.
- **Cutover/migration steps** — upgrade the CLI with `pyxis upgrade` or the current bootstrap installer before repinning a repository with v1-only tooling to the new release-manifest schema. Existing v1 manifests remain readable and hash-verified by the upgraded CLI.

| Stage | Audience | Minimum evidence | Advance when | Roll back when |
|---|---|---|---|---|
| RC | CI only | the full e2e matrix + drills | every live blocking DoD row green | any §3.3 breach |
| Dogfood | operator's next real repo | M3's checkpoint evidence | DOD-12 recorded | any §3.3 breach |
| Public | README + getting-started announce | dogfood held; docs walked (DOD-7) | operator go | supply-chain guardrail breach |

### op: supersede §6.6

Reason: Remove retired-runtime assumptions from this section while preserving the remaining contract.

| Area | Item | Owner | Status |
|---|---|---|---|
| Docs | getting-started for a fresh repo (bootstrap → five knobs → first feature) | @mdarabi | ☐ |
| Support | remediation reference: the UB refusals + doctor findings, as user-facing text | @mdarabi | ☐ |
| Platform | supported platforms + prerequisites note (git + committer identity; no Node.js or npm requirement) | @mdarabi | ☐ |
| Legal & privacy | license and notice files ride the release artifacts; the zero-egress posture stated in docs | @mdarabi | ☐ |
| Marketing & comms | README install section replaces the develop-in-repo framing | @mdarabi | ☐ |
| Enablement · Pricing · Localization | n/a — open-source tooling, no commercial packaging, English-only v1 | — | n/a |

**Decision:** Launch gate — §6.4 green, docs walked, rollback (pin the prior release) documented.

| Reviewer | Role | Date | Verdict |
|---|---|---|---|
| @mdarabi | operator (via the introducing PR's review) | — | — |

### op: supersede §7.1

Reason: Remove retired-runtime assumptions from this section while preserving the remaining contract.

**Customer** *(an adopter)*

- **Q:** Do I need Go installed? — **A:** No. You receive released, checksummed binaries; no source, no toolchain (NG-2).
- **Q:** Does it phone home? — **A:** No. Zero telemetry; outbound traffic is the artifact fetch and — only if you chose GitHub wiring — the operations it declared to you (§5.4).
- **Q:** Can I use it without GitHub? — **A:** Yes — the offline floor is a first-class configuration: local tracker, local hooks, zero network after fetch (FR-13).
- **Q:** What exactly will it do to my GitHub account? — **A:** Only the declared wiring set — labels, the CI workflow, the protection level you chose, a welcome issue — shown in preview before it runs, recorded after (FR-10, FR-19, EVT-4).
- **Q:** I already have a repo — can I adopt it? — **A:** Not yet; v1 refuses non-empty targets. Adoption is the designed-for follow-up (UB-1, WR-1).
- **Q:** How do I upgrade later? — **A:** The structure is ready (pins + converge); the polished upgrade experience arrives with the first release that changes installed files (WR-2).
- **Q:** Is the AI Engineer active after install? — **A:** Only if you turned its knob on; off is the default, and off means no delegated authority exists in the repo (FR-16).

**Internal**

- **Q:** Why one system directory instead of the vision's `pyxis-config/` split? — **A:** Operator ruling: ownership lives in the manifest, not in directory topology; the vision's surface is future ground, and the one cross-contract consequence is owned in Q-1 (`brainstorm:installer#D-7`).
- **Q:** Why does install end green with unauthored governance docs? — **A:** The alternatives were blocking install on human authorship or new gate states; the welcome item plus doctor's advisory carry the duty instead (`brainstorm:installer#D-8`, R4).
- **Q:** Why ship everything rather than a core subset? — **A:** The idea's own bar — "all the skills, all the tools" — with the one consent moment carved out for argus (`brainstorm:installer#D-11`).
- **Q:** Top three ways this fails? — **A:** The §1 obituaries: converge balloon (NG-3 + valve), source-coupled payload surfaces (NR-2 + P-8 audit), skeleton rot (FR-15 + FR-14 advisory).

- **Q:** Is a browser review server required? — **A:** No. Review documents with the operator in the conversation. The standalone feedback writer and Go plan viewer remain available (FR-31).

### op: supersede §8

Reason: Remove retired-runtime assumptions from this section while preserving the remaining contract.

| Check | Pass? | Notes |
|---|---|---|
| Problem basis accepted — §2 problem gate signed (T3) | ☐ | pending the introducing PR's review |
| Zero solution-blocking TBDs or open questions (§7.2) | ☑ | Q-2..Q-4 are design-owned; Q-1 and Q-5 are carried cross-contract obligations with named owners, and no requirement in this contract depends on either being answered — the CI credential carriage that would have depended on Q-5 is deferred to a later amendment |
| Every Must FR has a fit criterion and a verification method (§5.1) | ☑ | |
| Every requirement row carries a risk tier; high/critical rows have a declared safety plan (§5.1, §6.5) | ☑ | checksum-refusal floor (UB-3/NFR-8), preview-before-mutation (FR-19), declared-operation bound (FR-10), fail-closed credential refusal (FR-29/UB-13/NFR-7), invocation-gated rollout + prior-pin rollback (§6.5) |
| Non-goals drawn (§3.4); non-regression rows named (§5.1) | ☑ | NG-1..3; NR-1..3 |
| Every primary job (§2.3) covered by a requirement, non-goal, or waiting-room entry | ☑ | J1 → the live installation requirements in §5.1, including FR-31 and FR-32; J2 → FR-5, FR-6, FR-14, FR-19, FR-31 |
| Every key flow (§4.3) has functional, failure, and non-regression coverage (§5.1–5.2) | ☑ | F1 → the live install requirements and failure rows in §5.1–§5.2, including FR-31, FR-32, UB-14, and UB-15; F2 → FR-13, FR-29, FR-31, FR-32/UB-10, UB-13, UB-14, UB-15; F3 → FR-5, FR-6, FR-19, FR-31/UB-5, UB-8, UB-9; F4 → FR-14, FR-27, FR-31; NR-1..NR-3 apply flow-wide |
| Every primary metric (§3.2) has instrumentation (§5.7) or a measurement plan | ☑ | M1 → EVT-1/3; M2 → EVT-2; M3 → measurement plan: the DOD-12 demo record (EVT-1 certifies the install precondition) |
| Every blocking DoD row traces to a requirement, guardrail, release criterion, or rubric (§5.8) | ☑ | DOD-13 and DOD-10 name the live Must and UB sets, so rows added by amendment are inside them by construction |
| Risks and assumptions have owners (§6.3) | ☑ | the live rows in §6.3 |
| Human-judgment criteria name an owner and a rubric (§5.8) | ☑ | DOD-7: @mdarabi, verbatim-walkthrough rubric |
| DoD complete: every blocking row sourced and evidenced; every n/a explained (§5.8) | ☑ | DOD-12 non-blocking with its post-release reason stated; the release and RC gates name the live blocking set rather than a numeric range |

### op: supersede §0

Reason: Refresh the informative human-only routing section for the affected contract; it binds nothing.

*The body is the contract; this section only routes attention — on any conflict the body governs; machines ignore it.*

**Judge these**

| # | The call | Could have gone | If you disagree |
|---|---|---|---|
| 1 | Retire the mandatory browser runtime and its dependencies (§5.1, FR-31) | retain the runtime and its install stage | reject this amendment and the removal implementation |
| 2 | Keep artifact verification while introducing a versioned bundle-free release (§6.5) | preserve the old bundle as a permanent artifact | require a different compatibility transition before approval |

**Sanity-check the numbers** — NFR-9 preserves the existing numerical targets as advisory measurements; DOD-14 blocks on matrix correctness only, under AP-29.

**The author's open questions** — no new product decision is open; existing §7.2 questions retain their owners.

**Skip the rest** — fold/schema integrity → docgate; contract consistency → PRD review; implementation behavior → the release and installer regression suites.
