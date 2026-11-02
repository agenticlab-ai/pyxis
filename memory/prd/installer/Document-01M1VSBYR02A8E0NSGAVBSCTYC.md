# Amendment 003 — the distribution door is the default channel

| Field | Value |
|---|---|
| **Amends** | installer *(chain through 002)* |
| **Status** | Approved |
| **Owner** | @mdarabi |
| **Date** | 2026-08-19 |
| **Learned-from** | operator-decision:`logs/2026-08-19-1523-pyxis-init-404-diagnosis.md#P-1` |
| **Blast radius** | tier: contract-changing; touches: FR-22, FR-27, FR-28, FR-29, FR-30, UB-13, ABUSE-1, ABUSE-4, ABUSE-5, ABUSE-6, NFR-3, NFR-8, DOD-5, DOD-8, DOD-11, DOD-12, §6.1, §6.5, DOD-1, DOD-3, DOD-9, DOD-10, NFR-4, NFR-7, Q-5, §5.3, §5.4, §5.5, §6.2, §6.3, §7.2, §8, §0; reason: Must-row supersession moving the default artifact source off the source repository's release host onto the distribution door, plus the fail-closed credential rule, the channel hand-off, and the hosted-CI credential carriage the flag-free contract requires; §0's judgment table gains the call in the same op set, and the readiness, permissions, risk, and open-question sections are re-stated to match (`review:installer-003#FND-1..#FND-5` and the human overlay's P1/P2 corrections) |
| **Template version** | amendment-template/v2 |

## Operations

### op: supersede FR-22

Reason: an install whose consumer holds no read access to the source repository refused at preflight with a 404, because the default artifact source is a private repository access to which costs one paid seat per consumer; with the default moving to the distribution door, "the override" is no longer the only source a human cannot see at a glance, so the effective source itself must be reportable.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-27 | Where artifacts are resolved, the system shall fetch every artifact from the effective artifact source — the distribution door by default, or a configured override — under the same manifest-hash verification, and `preview` and `doctor` shall report that effective source and whether it is the default or an override. | a mirror can serve bytes but never choose them, so the channel may move freely; what may not move is the operator's ability to see which channel is answering | Must | high | default-source and override-configured fixtures both fetch from the effective source and both refuse checksum mismatches; preview and doctor name the effective source and label it default or override | Test | G3, F3, F4 |

### op: add FR

Reason: the default artifact source is a decision the engine carries in itself, and the one it carried named a repository whose read access is billed per person — the channel the release already mirrors to serves every audience at no seat cost.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-28 | Where no artifact-source override is configured, the system shall resolve every artifact from the distribution door declared in §5.5, and shall issue no artifact request to the source repository's release host. | consumers must be servable without an account on, or a paid seat in, the organisation that publishes the release | Must | med | with no override configured, every resolved artifact URL carries the door's host across manifest, binary, payload, and dependency fetches; a network transcript of a default-path install contains zero requests to the release host | Test | G3, F1, F3 |
| FR-29 | Where the effective artifact source requires a door credential and none is present or the source rejects the one presented, the system shall refuse with zero execution, naming the credential's environment variable and the refusing host, and shall not fall back to another channel. | a silent fallback would restore exactly the failure this amendment removes — a refusal from a host the operator never chose, whose remedy they cannot guess | Must | high | absent, malformed, and rejected-credential fixtures each refuse before any artifact byte is trusted; each refusal names the environment variable and the host; no fixture issues a request to a second channel | Test | G3, F3, UB-3 |
| FR-30 | When the CLI is installed, the installer shall record the channel it installed from alongside the release pin, and every artifact-source resolution shall follow one total precedence — an explicit flag, then the environment, then the repository's configured artifact source, then the recorded channel — so that `init` and `upgrade` require no channel or artifact-source flag from the operator. | the pin already travels this way, and a channel the consumer must retype is a channel they will get wrong; the order runs from the most specific to this invocation to the most general, with the environment above repository configuration because the shims already treat it as the redirect for a mirror or an air-gapped copy, and because the pinned manifest hash makes substituting a source safe by construction | Must | med | after a door install, `pyxis init` and `pyxis upgrade` run with no channel or artifact-source flag and resolve to the door; and each adjacent-pair conflict fixture — flag vs environment, environment vs repository configuration, repository configuration vs record — resolves to the higher precedent, as does every non-adjacent pair | Test | G1, J1, F1 |

### op: add UB

Reason: the credential is the one input the seat-free channel cannot supply for the operator, so its absence is a first-class unhappy path rather than a transport error.

| ID | If… (trigger) | Then the system shall… | User sees | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|---|
| UB-13 | the door credential is absent, malformed, or rejected by the artifact source | refuse with zero execution, name the credential's environment variable and the refusing host, and exit non-zero without attempting another channel | a named door refusal carrying the environment variable and the host | Must | high | absent, malformed, and 401/403 fixtures each refuse with zero execution and zero cross-channel requests; the refusal text names both the variable and the host | Test | FR-29, FR-27, UB-3 |

### op: supersede §5.5 Interfaces and integrations

Reason: the artifact host row named the code host as the source of bytes; the door is now the default and carries a credential contract and a refusal mode the row did not describe.

| Interface | Direction | Purpose | Contract | Failure modes | Owner |
|---|---|---|---|---|---|
| release artifact host — the distribution door `https://dist.pyxis.agenticlab.ai` by default; the source repository's Releases remain the canonical publish target and a supported override | in | fetch binaries + payload | versioned URLs + checksums (FR-1, FR-17); the effective source is reportable (FR-27); the door credential is supplied by the environment and host-locked over TLS (FR-29) | unreachable → named failure, re-runnable (UB-2); tamper → refuse (UB-3); credential absent, malformed, or rejected → named refusal with no fallback (UB-13) | this feature |
| GitHub API (labels, protection, workflow, welcome item) | out | the wiring knob | the declared operation set (FR-10) | missing permission → hand-off (UB-4); absent credential → skip/offline | this feature |
| git (hooks path, initial structure) | both | hook activation + the installed tree | hooks enforce the same checks as the source repo (FR-9) | missing git → UB-7 | this feature |
| agent CLI surfaces (claude/codex/cursor/opencode dirs) | out | skill discovery in installed repos | pointers into the payload (FR-11, AP-25) | absent CLI → auto-detect skips; explicit choice materializes regardless | this feature |
| node runtime + pinned lockfile | in | the review-surface hard dependency (FR-12) | lockfile-pinned install, un-bundled vendor | absent node → UB-7 | this feature |

### op: supersede §6.2 Dependencies

Reason: the default resolution path now depends on the door, and the door credential becomes a user prerequisite the base did not carry.

| Dependency | Type | Needed by | Status | Owner |
|---|---|---|---|---|
| the distribution door (object storage behind a credentialed front door) | vendor | default artifact resolution | available; the release already mirrors the full artifact set per tag | this feature |
| GitHub Releases (artifact hosting) | vendor | Release machinery; the canonical publish target and the override channel | available (docgate precedent) | this feature |
| the door credential on consumer machines and in operand CI | user prerequisite | every default-path fetch | user-provided; issued with the install instructions (UB-13) | user |
| darwin + linux CI runners for the matrix | system | Init end-to-end | available; darwin runner cost noted (R3) | this feature |
| node runtime on target machines | user prerequisite | Init end-to-end | documented prerequisite (UB-7) | user |
| GitHub credential for wiring (optional) | user prerequisite | GitHub-mode installs | user-provided (UB-4) | user |
| parked technical constraints handed to the design | design input | design gate | `brainstorm:installer#P-1..#P-8` | technical design |

### op: supersede §7.2 Open questions

Reason: `review:installer-003` overlay — the CI credential carriage this amendment first contracted could not be satisfied until a sibling contract is amended, so it is deferred and the obligation is recorded here instead; the section's closing prose could no longer call Q-1 the only cross-contract row.

| # | Question | Owner | Needed by | Status |
|---|---|---|---|---|
| Q-1 | The verification manifest's home: the implementation-verification PRD contracts it into `pyxis-config/`, which v1 deliberately does not create — re-home it into the system tree by typed amendment there, or hold FR-5-of-that-PRD for a later `pyxis-config/` feature? | operator (an amendment to `docs/prds/implementation-verification/`) | before the verification manifest ships in a payload release | open — `brainstorm:installer#G-1` |
| Q-2 | The release/versioning scheme: one system version pinning binaries + payload together, or per-binary tags (today's docgate pattern) — and the manifest's exact schema | technical design | design gate | open — `brainstorm:installer#P-4` |
| Q-3 | The GitHub wiring mechanism: gh CLI as a prerequisite vs direct REST with the ambient token — and how the declared-operation audit is produced | technical design | design gate | open |
| Q-4 | The install-epoch baseline shape (generalized grandfathering) that keeps WR-1 adoption structurally cheap without building it now | technical design | design gate | open — `brainstorm:installer#P-5` |

*Q-2..Q-4 are design-owned mechanism choices behind settled requirements. Q-1 and Q-5 are cross-contract obligations this PRD carries but does not decide; neither is solution-blocking — no requirement in this contract depends on either being answered first.*

### op: add Q

Reason: `review:installer-003` overlay — the section supersession renders the row but registers no typed unit, so the source map carried no Q-5 for citations or invalidation to resolve; the typed op restores it.

| # | Question | Owner | Needed by | Status |
|---|---|---|---|---|
| Q-5 | The generated CI workflow's carriage of the release pin and the door credential (opened by amendment 003): `prd:config-driven-portability` FR-4 scopes that file's rendered content to branch, remote, and path references, and its UB-3 has converge regenerate any hand-edit, so that contract must be amended — and its substitution grammar extended past `{{lower_snake_case}}` — before any installer requirement can contract CI credential carriage | @mdarabi | before an installer amendment contracts CI credential carriage | open — a carried cross-contract obligation; blocks nothing in this contract |

### op: supersede NFR-4

Reason: `review:installer-003` overlay — the operability fit criterion enumerated UB-1..UB-10, so every unhappy path added by amendment (UB-11, UB-12, and now UB-13's credential refusal) sat outside the row that requires refusals to name their remedy.

| ID | Area | Requirement | Fit criterion | Verify |
|---|---|---|---|---|
| NFR-7 | Operability | Every refusal and failure shall name its cause and remediation; doctor shall classify every failure state named in §5.2. | every fixture in §5.2's live UB set — not a fixed range — produces output containing its named remediation, and doctor classifies every §5.2 state | Test |

### op: supersede DOD-3

Reason: `review:installer-003` overlay — the blocking DoD row sourced UB-1..UB-10, so a release could have passed it without ever exercising an absent, malformed, or rejected door credential.

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-10 | All UB rows handled as specified | every live UB row (§5.2; superseded rows excluded) | Test | failure-injection suite | yes |

### op: supersede DOD-1

Reason: `review:installer-003#FND-3` — the row's source range ended at an ID this amendment supersedes, so the blocking DoD named a retired anchor and excluded every Must row added since the base.

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-9 | All Must FRs pass their fit criteria | every live Must FR (FR-1..FR-30; superseded rows excluded) | Test | CI run link | yes |

### op: supersede §5.3 Permissions and abuse cases

Reason: `review:installer-003#FND-2` — the access paragraph told the reader that the offline path needs no credential at all, which the default channel falsifies: the local-only install's zero-network floor begins *after* a fetch that now requires the door credential.

*Access: init runs under the invoking user's identity. Artifact resolution on the default channel requires the door credential from the environment on every install shape — the local-only one included, whose zero-network floor (FR-13) begins after that fetch (FR-28, FR-29). GitHub operations use their ambient credential for the declared operation set only; no GitHub credential is needed on the offline path. The credential itself is read from the environment per fetch and is never written to disk, never placed in repo config or the machine record, and never echoed by preview, doctor, or any log. Beyond the repo's own files the installer stores exactly one thing: the machine release record — the pin and the channel it came from (FR-30, §5.4).*

| Function / data | User with repo admin | User with write-only | No GitHub credential |
|---|---|---|---|
| converge files, seed, hooks, carina, CLI surfaces | full | full | full (offline path) |
| labels + workflow push | full | full | none (skipped or hand-off) |
| branch protection | full | hand-off block (UB-4) | none |

| ID | Actor and intent | Vector | Impact | Severity | Mitigated by | Verify |
|---|---|---|---|---|---|---|
| ABUSE-5 | an attacker serving a spoofed artifact (poisoned mirror, MITM, typo-squatted override) | the artifact-source override or a compromised fetch path | arbitrary code on the user's machine | critical | FR-2/FR-17/UB-3 (checksum before any execution); the effective source — default or override — is loudly reported in preview and doctor (FR-27) | drill: spoofed-source fixtures are refused on every fetch path |
| ABUSE-2 | a malicious PR to an installed repo edits engine-verbatim files to smuggle behavior into future agent runs | installed repos trust their installed system tree | altered gates/skills act with the repo's authority | high | FR-14 (doctor names the drift by class) + FR-5 (repair restores byte-exact); manifest hashes make tampering detectable | drill: a tampered shipped file is detected and repaired |
| ABUSE-3 | the installer itself over-reaching on the granted GitHub credential | any API call beyond the declared wiring set | unwanted account mutation; trust collapse for an open-source installer | high | FR-10's declared-operation bound + EVT-4's audit record; preview (FR-19) shows the operation set before it runs | Test: the recorded operation set equals the declared set in every matrix combo |
| ABUSE-6 | an attacker serving a malicious bootstrap script (distinct from a tampered binary — the shell executes fetched bytes before any binary verification can run) | bootstrap URL substitution or interception ahead of the checksum step | arbitrary shell on the user's machine | critical | the documented bootstrap line is a pinned HTTPS URL; the shim fetches the checksum set before executing any fetched byte beyond itself; the getting-started docs state the HTTPS floor and the pinned-URL pattern | drill: a tampered bootstrap-script fixture is refused; DOD-11's drill set includes it |

### op: supersede ABUSE-1

Reason: `review:installer-003` overlay — the §5.3 restatement changed ABUSE-1's mitigation cell while keeping its ID, so the source map still attributed the changed row to the base and neither the affected-unit set nor downstream invalidation saw the amendment. A changed row needs a fresh ID and a typed op, the same rule Q-5 and R7 taught for new ones.

| ID | Actor and intent | Vector | Impact | Severity | Mitigated by | Verify |
|---|---|---|---|---|---|---|
| ABUSE-5 | an attacker serving a spoofed artifact (poisoned mirror, MITM, typo-squatted override) | the artifact-source override or a compromised fetch path | arbitrary code on the user's machine | critical | FR-2/FR-17/UB-3 (checksum before any execution); the effective source — default or override — is loudly reported in preview and doctor (FR-27) | drill: spoofed-source fixtures are refused on every fetch path |

### op: supersede ABUSE-4

Reason: `review:installer-003` overlay — the row's verify cell names the DoD row that carries its drill, and this amendment supersedes that row; re-anchoring closes the last dangling executable reference the ABUSE-1 supersession opened.

| ID | Actor and intent | Vector | Impact | Severity | Mitigated by | Verify |
|---|---|---|---|---|---|---|
| ABUSE-6 | an attacker serving a malicious bootstrap script (distinct from a tampered binary — the shell executes fetched bytes before any binary verification can run) | bootstrap URL substitution or interception ahead of the checksum step | arbitrary shell on the user's machine | critical | the documented bootstrap line is a pinned HTTPS URL; the shim fetches the checksum set before executing any fetched byte beyond itself; the getting-started docs state the HTTPS floor and the pinned-URL pattern | drill: a tampered bootstrap-script fixture is refused; DOD-11's drill set includes it |

### op: supersede NFR-3

Reason: `review:installer-003` overlay — the fit criterion names the drill set that proves it, and one member of that set is now the superseded ABUSE-1; re-anchoring keeps the criterion runnable against rows that exist.

| ID | Area | Requirement | Fit criterion | Verify |
|---|---|---|---|---|
| NFR-8 | Security | The system shall execute no fetched artifact that has not passed checksum verification, on any code path. | the UB-3/ABUSE-5 drill set shows zero unverified executions | Test |

### op: supersede DOD-5

Reason: `review:installer-003` overlay — the row's source enumerates the drills that discharge it, and ABUSE-1 among them is superseded; the door's credential refusal belongs in the same supply-chain evidence set.

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-11 | Supply-chain posture proven: checksum refusal on every fetch path; declared-operation audit clean; the door credential refused closed | UB-3, UB-9, UB-13, ABUSE-2, ABUSE-3, ABUSE-5, ABUSE-6, NFR-8 | Test + drill | drill records | yes |

### op: supersede DOD-8

Reason: `review:installer-003` overlay — the row's own blocking cell named the release condition as the numeric range DOD-1..7, which now points at three superseded rows and omits all three replacements, DOD-11's door-credential refusal evidence among them; a live set replaces the range so no future supersession can silently drop a blocking row out of the release gate.

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-12 | Dogfood: the next real project repo is created through the installer and a feature passes the pipeline there | M3, G1 | Demo | the repo + its pipeline artifacts | no — post-release checkpoint (§6.7); the release ships on every live blocking DoD row |

### op: supersede §6.1 Phasing

Reason: `review:installer-003` overlay — the dogfood phase's exit criterion names the DoD row this amendment supersedes.

| Phase | Purpose | Scope | Exit criterion |
|---|---|---|---|
| Release machinery | make "a Pyxis release" exist | FR-1, FR-2, FR-17 (artifact set, bootstrap, pinned resolution) | a tagged release's artifact set fetches, verifies, and runs on all platforms |
| Converge core | the manifest semantics | FR-5, FR-6, FR-19, doctor (FR-14) | offline-fixture converge/repair/preview/doctor all green |
| Init end-to-end | the product face | FR-3, FR-4, FR-7..FR-13, FR-15, FR-16 | the full e2e matrix green at the install budget (DOD-4) |
| Dogfood | prove it on a real repo | M3 | DOD-12 recorded on Tracking |

### op: supersede §6.5 Rollout and reversibility

Reason: `review:installer-003` overlay — the release-candidate stage advanced on DOD-1..6, a range that now names three superseded rows and omits every replacement, so a release could have shipped without the door-credential refusal ever being proven.

- **Flag** — none: installation is invocation-gated (nothing happens to anyone who does not run it).
- **Kill switch** — pull or deprecate the release; the bootstrap fetches only pinned, checksummed versions, and a prior pin remains installable (FR-17). An installed repo is unaffected until it chooses to converge to a newer pin.
- **Experiment** — none.
- **Cutover/migration steps** — none (empty-repo scope).

| Stage | Audience | Minimum evidence | Advance when | Roll back when |
|---|---|---|---|---|
| RC | CI only | the full e2e matrix + drills | every live blocking DoD row green | any §3.3 breach |
| Dogfood | operator's next real repo | M3's checkpoint evidence | DOD-12 recorded | any §3.3 breach |
| Public | README + getting-started announce | dogfood held; docs walked (DOD-7) | operator go | supply-chain guardrail breach |

### op: supersede §5.4 Data

Reason: `review:installer-003` overlay — FR-30 stores a record outside the target repository, which the section's rollback bullet denied, and FR-29 reads a credential the privacy check said did not exist.

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
- **Egress** — **zero telemetry** (WR-8: keys reserved, nothing sent). Outbound traffic is exactly: release-artifact fetch (in), carrying the door credential to the artifact-source host alone (FR-29); dependency install from the pinned lockfile (in); and — GitHub mode only — the declared wiring operations (out). The offline combo performs none of the above after fetch (FR-13).

### op: supersede §6.3 Risks, assumptions, and rabbit holes

Reason: `review:installer-003#FND-5` — the default-channel flip buys seat-free distribution by giving up per-identity revocation, and that cost appeared nowhere in the contract.

| # | Risk / assumption / rabbit hole | Likelihood | Impact | Mitigation or test | Owner |
|---|---|---|---|---|---|
| R1 | converge semantics balloon (merge cleverness, conflict UX) | M | H | NG-3 + the appetite valve: ownership classes only; repo-owned is never rewritten | design |
| R2 | a payload surface is source-coupled and breaks only in installed repos (resolver fallbacks, path assumptions, toolchain leaks) | H | H | the parity suite (NR-2) + the portability audit of every skill invocation line (`brainstorm:installer#P-8`); the offline combo catches network/toolchain leaks | design |
| R3 | darwin CI cost/flakiness undermines the matrix | M | M | full matrix on linux; darwin as a smoke tier per release candidate | design |
| R4 | installed repos rot with skeleton governance never authored | M | M | FR-15's welcome item + FR-14's advisory finding keep it visible; accepted residual — NG-1 forbids stronger coercion | product |
| R5 | dependency install at init time varies with registry health (the lockfile pins versions, not availability) | M | M | UB-2 re-runnability; the offline combo documents the pre-fetched alternative | design |
| R6 | the five knobs prove insufficient for a real adopter (a sixth decision emerges) | M | M | config keys are reserved ahead of knobs (WR-8, WR-9); a new knob is an amendment, not a redesign | product |

### op: add R

Reason: `review:installer-003` overlay — the risk belongs to the amendment as a typed unit; introducing it only inside the §6.3 restatement would leave it with no source-map lineage, the same defect the overlay caught on Q-5.

| # | Risk / assumption / rabbit hole | Likelihood | Impact | Mitigation or test | Owner |
|---|---|---|---|---|---|
| R7 | the default channel's credential has no per-consumer revocation — withdrawing it from one consumer withdraws it from all — and every rotation must reach every desk, hook, and CI environment that fetches | M | M | accepted trade for seat-free distribution; per-consumer credentials at the door are a reachable evolution needing no contract change, and FR-27's effective-source reporting plus UB-13's named refusal make a stale credential diagnosable rather than mysterious | product |

### op: supersede §8 Contract readiness — the executability gate

Reason: `review:installer-003#FND-3` — the flow-coverage row still mapped F4 onto FR-22, the requirement this amendment supersedes, and no row accounted for the five Must rows and the failure mode added here.

| Check | Pass? | Notes |
|---|---|---|
| Problem basis accepted — §2 problem gate signed (T3) | ☐ | pending the introducing PR's review |
| Zero solution-blocking TBDs or open questions (§7.2) | ☑ | Q-2..Q-4 are design-owned; Q-1 and Q-5 are carried cross-contract obligations with named owners, and no requirement in this contract depends on either being answered — the CI credential carriage that would have depended on Q-5 is deferred to a later amendment |
| Every Must FR has a fit criterion and a verification method (§5.1) | ☑ | |
| Every requirement row carries a risk tier; high/critical rows have a declared safety plan (§5.1, §6.5) | ☑ | checksum-refusal floor (UB-3/NFR-8), preview-before-mutation (FR-19), declared-operation bound (FR-10), fail-closed credential refusal (FR-29/UB-13/NFR-7), invocation-gated rollout + prior-pin rollback (§6.5) |
| Non-goals drawn (§3.4); non-regression rows named (§5.1) | ☑ | NG-1..3; NR-1..3 |
| Every primary job (§2.3) covered by a requirement, non-goal, or waiting-room entry | ☑ | J1 → FR-3..FR-13, FR-15..FR-16, FR-20, FR-30; J2 → FR-5, FR-6, FR-14, FR-19 |
| Every key flow (§4.3) has functional, failure, and non-regression coverage (§5.1–5.2) | ☑ | F1 → FR-2..FR-12, FR-20, FR-28..FR-30/UB-1..UB-7, UB-9, UB-10, UB-13; F2 → FR-13, FR-28, FR-29/UB-2, UB-7, UB-10, UB-13; F3 → FR-5, FR-6, FR-19/UB-5, UB-8, UB-9; F4 → FR-14, FR-27; NR-1..NR-3 apply flow-wide |
| Every primary metric (§3.2) has instrumentation (§5.7) or a measurement plan | ☑ | M1 → EVT-1/3; M2 → EVT-2; M3 → measurement plan: the DOD-12 demo record (EVT-1 certifies the install precondition) |
| Every blocking DoD row traces to a requirement, guardrail, release criterion, or rubric (§5.8) | ☑ | DOD-9 and DOD-10 name the live Must and UB sets, so rows added by amendment are inside them by construction |
| Risks and assumptions have owners (§6.3) | ☑ | R1..R7 |
| Human-judgment criteria name an owner and a rubric (§5.8) | ☑ | DOD-7: @mdarabi, verbatim-walkthrough rubric |
| DoD complete: every blocking row sourced and evidenced; every n/a explained (§5.8) | ☑ | DOD-12 non-blocking with its post-release reason stated; the release and RC gates name the live blocking set rather than a numeric range |

### op: supersede §0 Read to rule — the human reviewer's section (~6 min)

Reason: §0-only informative maintenance at the computed contract-changing tier — the default channel is a call a signing reviewer must see, and the routed numbers paragraph now has a credential prerequisite behind it.

*The body is the contract; this section only routes attention — on any conflict the body governs; machines ignore it.*

**Judge these**

| # | The call | Could have gone | If you disagree |
|---|---|---|---|
| 1 | **Converge onto a release** — install, re-run/repair, and (later) upgrade and adopt are one manifest-driven code path | a copy-once scaffolder, converge built later | repair (M2) and the existing-repo follow-up become a second product; idempotency is untested until it hurts → §4.1, G2 |
| 2 | **A single system directory, manifest-marked ownership** — no separate `pyxis-config/` directory in v1 | the two-directory engine/repo-owned physical split the vision's §8 sketches | the vision's extension surface arrives later, and the implementation-verification PRD's manifest home needs re-ruling → §7.2 Q-1 |
| 3 | **Green with skeletons** — install seeds fill-me governing-doc skeletons and ends with gates passing | block install until the human authors vision/principles; or a new advisory gate state | installed repos can sit skeleton-governed indefinitely; the welcome item is the only authorship driver → FR-7, FR-15, R4 |
| 4 | **argus is opt-in, off by default** — the fifth knob | ship the AI Engineer enabled like every other skill | strangers get delegated decision authority by default → FR-16 |
| 5 | **Everything pipeline-serving ships** in the payload | a curated core subset first | payload breadth is parity surface: every shipped skill must behave identically in installed repos → §4.2, §3.3 |
| 6 | **Targets consume released binaries, never Go source** | clone-and-build installs | every adopter inherits a Go toolchain requirement and the module-path coupling → NG-2, FR-17 |
| 7 | **The offline floor is v1 acceptance** — carina + local hooks, zero network after fetch | GitHub-first, offline later | medium-independence stays unproven and the tracker-neutral claim is marketing → FR-13, M1 |
| 8 | **The distribution door is the default channel** — consumers resolve artifacts from the door unless they override, and a missing credential refuses rather than falling back | keep the source repository's Releases as the default and grant each consumer read access there | per-consumer access control moves from repository grants to door credentials, and every consumer environment — desk, hook, and CI — must carry one → FR-28, FR-29, FR-30, §5.5 |

**Sanity-check the numbers** — install ends green at **≤ 5 min p95** on reference runners, dependency install included (NFR-1); **one** repository question naming the destination plus at most **four** asked knobs, the GitHub wiring depth derived from that answer and everything else defaulted-and-captured (FR-26), with a local-only answer leaving the GitHub questions unasked (FR-25); **zero** channel or artifact-source flags on `init` and `upgrade` after any install, and **one** environment credential as the whole consumer prerequisite for the default channel (FR-30, FR-29); **100%** targets on the release-candidate e2e matrix for install success (M1) and drift repair (M2); **zero** telemetry egress (§5.4).

**The author's open questions** — two cross-contract obligations, neither blocking a row here. The implementation-verification PRD homes its verification manifest in a directory this feature deliberately does not create (§7.2 Q-1). `prd:config-driven-portability` must be amended, and its substitution grammar widened, before any installer requirement can contract the hosted CI's credential carriage (§7.2 Q-5) — which is why that carriage is deferred rather than contracted here. The remaining §7.2 rows are design-owned mechanism choices behind settled requirements.

**Skip the rest** — structure/ID/fold correctness → docgate; substance and coherence → the unmanned `/prd-review`; packaging, versioning-scheme, and wiring mechanisms → the technical design (§7.2); supply-chain hardening depth → the design's security section plus the §5.3 drills.
