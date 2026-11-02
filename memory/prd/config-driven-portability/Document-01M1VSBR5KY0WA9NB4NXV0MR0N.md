# Config-driven portability — PRD

| Field | Value |
|---|---|
| **In one line** | The engine's per-repo parameterization: every target-facing path, surface pointer, and branch/remote resolves from the repo config so Pyxis runs against any repo's layout — phase 1 of existing-repo adoption. |
| **Relevant-when** | planning, building, or reviewing how the engine resolves doc paths, branch/remote, or CLI-surface pointers; triaging a gate that checks the wrong path or a branch mismatch in an installed repo; extending what the repo config parameterizes; scoping existing-repo adoption (installer WR-1) |
| **Status** | Draft — problem review pending |
| **Tier** | T2 |
| **Owner** | @mdarabi |
| **Approver(s)** | @mdarabi — via the introducing PR's operator review |
| **Contributors** | @mdarabi (product + engineering) |
| **Informed** | — |
| **Date** | 2026-07-22 created · 2026-07-22 last updated |
| **Appetite** | A focused single-PRD feature (`brainstorm:config-driven-portability#D-8`): human attention = the review gates; machine work ≈ a handful of worker passes (finish + generalize an existing closed schema); rollout burden ≈ zero — defaults are preserved, so no installed repo changes behavior until it sets a non-default value. |
| **Target release** | The phase-1 predecessor that unblocks phase-2 existing-repo adoption (installer WR-1); no calendar target. |
| **Tracking** | — *(the introducing PR, once filed)* |
| **Artifacts** | `brainstorm:config-driven-portability` (gated record); the 4-part hardcoding audit it summarizes (F-1…F-10) |
| **Template version** | prd-template/v2 |

## 0. Read to rule — the human reviewer's section (~4 min)

*The body is the contract; this section only routes attention — on any conflict the body governs; machines ignore it.*

**Judge these**

| # | The call | Could have gone | If you disagree |
|---|---|---|---|
| 1 | Requirements are **coherence invariants** ("no target-facing surface stays hardcoded"), not a checklist of the specific paths the audit found | enumerate the four doc paths + two maps as named rows | the acceptance shifts from a grep-provable property to a fixed list that rots as surfaces are added → §5.1 FR-1..FR-4, §3.2 M1 |
| 2 | Toolchain/stack selection is **out of phase-1** entirely | lift the stack alongside paths/branch now | a non-Go repo can't be honored until a later phase — but self-host can't witness non-Go anyway → §3.4 WR-1, `brainstorm:…#D-4` |
| 3 | The proof is **self-host on a non-default config**, not a foreign fixture repo | build a non-pyxis fixture the engine operates green | weaker "it's not just pyxis" evidence, traded for not dragging phase-2 onboarding forward → §3.2 M1, §5.8 DOD-1 |
| 4 | Non-Go surfaces (hooks, CI workflow) get config by **generate-at-converge**, not by reading config at runtime | teach the shell hooks to parse the repo config | config-interpretation logic grows into bash — the design must resolve the mechanism → §7.2 Q1, `brainstorm:…#P-1` |

**Sanity-check the numbers** — the appetite (~5–8 worker passes, header/§6.1) and the single guardrail (zero behavior change on a default config, §3.3) are the only quantities; both are acceptance-style, not instrumented.

**The author's open questions** — Q1 (§7.2): the config-delivery mechanism to non-Go surfaces is parked for the technical design, not decided here.

**Skip the rest** — structure/schema → docgate; substance & coherence → the unmanned prd-review; requirement-level defects → the tech design and downstream gates. §5.4/§5.5/§5.9 and §6.6 are asserted `Skipped` with reasons in place.

## 1. Summary — the one-pager

- **Problem** — The engine assumes it is running in the repository it was built in: its own doc-path layout, its `main`/`origin`, its `.claude`/`.codex` surface files are baked into code, hooks, and CI. It cannot run correctly anywhere else.
- **For whom** — The pyxis engine itself is the beneficiary (its own portability, dogfooded); the design audience is the phase-2 external adopter — a solo or 2–5-person team on their own infrastructure (`brainstorm:config-driven-portability#D-1`).
- **Why now** — Existing-repo adoption (installer WR-1) is blocked at the root by these self-assumptions, and a hardcoding audit shows the config schema already parameterizes most of the engine (F-1) — this is finishing a nearly-complete surface, not building one.
- **Direction** — Extend the repo config's closed schema so every *target-facing* surface (doc paths, branch/remote, CLI-surface pointers, the installed workflow's refs) resolves from it, with today's values as defaults; make the property coherent (no surface left behind) rather than a checklist.
- **Appetite** — A focused single-PRD feature; excludes toolchain, the collision residue, and all onboarding UX.
- **Success** — The pyxis repo runs green — every gate, the local `gate`, and `doctor` — under a deliberately non-default config (`brainstorm:…#D-5`).
- **Guardrail** — A default config changes no installed repo's behavior (byte-identical to today).
- **Top risks** — (1) a coherence gap: one surface still hardcoded while the rest honor config; (2) delivering config to the non-Go hooks/workflow (P-1); (3) scope creep into phase-2 onboarding.

## 2. Problem space

### 2.1 Problem statement

Pyxis's engine hardcodes "this repo" — its own doc paths, its `main`/`origin`, its `.claude`/`.codex` surface layout — so it can only run where it was developed (`brainstorm:config-driven-portability#D-7`). Any repository with a different layout would be dishonored or collided with the moment the engine ran there, which blocks existing-repo adoption (installer WR-1) at the root. The cost of doing nothing is that the factory's compounding value stays trapped in one repository and the product claim — "engineers simply install it" — remains unprovable for any repo that already has its own source and structure.

### 2.2 Users and segments

| Segment | Role and context | Domain exp. | Tech exp. | Key goals | Pain today | Evidence |
|---|---|---|---|---|---|---|
| **P1 (primary)** | The pyxis engine itself — the self-host proof is the "user"; phase-1 serves no new external end-user | master | master | run correctly against a declared layout, not an assumed one | every gate/verb/hook assumes the pyxis layout | `brainstorm:…#D-1` |
| P2 (design audience) | The phase-2 external adopter — solo / 2–5-person team on their own infra (installer P2) | novice–journeyman (of Pyxis) | journeyman–master | a config key exists for every surface their repo lays out differently | cannot adopt without engine changes | `brainstorm:…#D-1`, `prd:installer#D-2` |
| **Anti-segment** | Org-wide adoption sweeps, non-GitHub hosts (installer's anti-segment) **and the onboarding UX itself** — phase-2 | — | — | — | — | `brainstorm:…#D-1` |

### 2.3 Jobs and desired outcomes

| # | When… | I want to… | So I can… | Today's workaround | Evidence |
|---|---|---|---|---|---|
| J1 | my repo's paths / branch / remote / surfaces differ from Pyxis's own | declare them in the repo config and have every part of the engine obey | run the factory against my layout without editing engine code | none — the engine refuses non-default values fail-closed (installer WR-10) or ignores config and uses a constant | `brainstorm:…#D-2` |

### 2.4 Evidence

| Claim | Source | Confidence |
|---|---|---|
| The config is already a closed schema parameterizing most of the engine (paths, gate commands, system maps, capability paths, grandfathering) | `brainstorm:…#F-1` (the hardcoding audit of the config loader and surface) | High |
| `branch` is captured in config but honored nowhere; `main`/`origin` are hardcoded across code, hooks, and the installed workflow | `brainstorm:…#F-2` (audit) | High |
| A set of target-facing doc paths is hardcoded outside the schema while its siblings are config keys | `brainstorm:…#F-4` (audit) | High |
| The CLI-surface and governance-skeleton maps are hardcoded literals derivable from existing knobs + paths | `brainstorm:…#F-5` (audit) | High |
| A distinct set of surfaces is irreducibly single-owner and cannot be relocated by config (AGENTS.md, `core.hooksPath`, `.github/workflows/`, CLAUDE.md) | `brainstorm:…#F-8` (audit) | High |
| The Go module path and release host are not adopter concerns (binaries-not-source; `artifact_source` overrides host) | `brainstorm:…#F-7`, `prd:installer#FR-17` | High |

### 2.5 The journey today

*Skipped: phase-1 is an engine-internal parameterization with no end-user journey; the "flow" is the engine reading config, covered as behavior in §4.1 and §5.1.*

### 2.6 Opportunity and why now

- **Size** — Every existing repository that would adopt Pyxis passes through this gate; for an open-source factory, portability is the precondition of the entire adoption funnel (installer WR-1).
- **Alternatives users have** — Fork Pyxis and rewrite its assumptions (untenable); stay trapped in a fresh-init-only world; do nothing.
- **Why us / strategic fit** — The config schema, ownership classes, `doctor`, and `preview` already exist (F-1); this completes a surface Pyxis already committed to, realizing "no privileged substrate" (AP-1).
- **Why now** — Adoption is the stated next milestone and it is blocked here first; deferring keeps the compounding value trapped in one repo.
- **Cost of doing nothing** — Existing-repo adoption cannot begin; the product claim stays unproven.

**Decision:** Problem gate — do not proceed to §3–§4 until signed.

| Reviewer | Role | Date | Verdict |
|---|---|---|---|
| @mdarabi | operator | — | — *(via the introducing PR's operator review)* |

## 3. Goals, metrics, and guardrails

### 3.1 Goals as falsifiable hypotheses

| # | We believe that… | Will result in… | Measured by… |
|---|---|---|---|
| G1 | making every target-facing surface resolve from config | the engine runs correctly against a layout other than its own | M1 |
| G2 *(qualitative)* | the coherence property (no surface left behind) is real, not a checklist | "no privileged substrate" (AP-1) holds under inspection | grep/inspection for hardcoded target-facing literals returning zero |

### 3.2 Goals → signals → metrics

*Phase-1 is internal tooling; success is acceptance-style (rule §1), not instrumented telemetry.*

| Metric | Goal | Signal | Baseline | Target (at checkpoint) | Feeds from |
|---|---|---|---|---|---|
| M1 (primary) | G1 | the pyxis repo runs green under a non-default config | fails today (values are hardcoded) | all gates + local `gate` + `doctor` green under a config setting non-default doc paths, branch, remote, and surface pointers | EVT-1 (the self-host CI job) |

### 3.3 Guardrails and decision rule

| Guardrail metric | Threshold | Action if breached |
|---|---|---|
| Behavior under a **default** config | any observable difference from the pre-change engine | block — defaults must be preserved (byte-identical) |

**Ship rule (pre-registered):** M1 green (self-host on non-default config) AND the default-config guardrail clean → ship; the guardrail breached → block until defaults are restored.

### 3.4 Non-goals and waiting room

| ID | Non-goal | Why refused |
|---|---|---|
| NG-1 | Config-driving the engine's own system-tree paths | they live in Pyxis's own namespace and never collide with a target — config buys adoption nothing (`brainstorm:…#O-4`) |
| NG-2 | Changing the Go module path or hardcoded release host | adopters consume released binaries, never source (`prd:installer#FR-17`/NG-2); `artifact_source` already overrides the host (`brainstorm:…#O-3`) |

| ID | Waiting room | Revisit when / intended version |
|---|---|---|
| WR-1 | Toolchain/stack selection (a `stack` knob, generalized non-Go verb defaults) | a real non-Go adopter exists; the `executor.gate` override covers the interim (`brainstorm:…#O-1`, `#D-4`) |
| WR-2 | Merge/chaining policy for the irreducible single-owner surfaces (AGENTS.md, `core.hooksPath`, `.github/workflows/`, CLAUDE.md) | phase-2 onboarding (`brainstorm:…#O-2`) |
| WR-3 | Onboarding itself — non-empty-tree converge, the install-epoch/grandfathering shape (installer Q-4), any wizard/UX | phase-2 existing-repo adoption (installer WR-1/J4) (`brainstorm:…#O-5`) |

## 4. Solution space

### 4.1 Approach and alternatives considered

The chosen shape extends the existing closed repo-config schema so every target-facing surface resolves from it, with current values as defaults; consumers read the configured value instead of a constant; and the non-Go surfaces (shell hooks, the installed CI workflow) receive their values by generate-at-converge rather than by reading config at runtime. Coherence — no target-facing surface left hardcoded — is the acceptance bar, proven by self-hosting on a non-default config.

| Option | Sketch | Why not chosen |
|---|---|---|
| A (chosen) | extend the closed schema; consumers read config; non-Go surfaces generated at converge; self-host proof | — |
| B | per-surface hardcode-with-override flags | no coherent surface; each flag its own contract; the schema already exists (F-1) |
| C | introduce a separate `pyxis-config/` directory | installer D-7 settled one manifest-marked system tree; reopening it is out of scope (`prd:installer#D-7`) |
| Do nothing | — | existing-repo adoption stays blocked (§2.6) |

### 4.2 Scope perimeter

- **In (plan of record)** — the target-facing doc paths not yet in the schema; branch + remote across code, hooks, and the workflow's refs; the CLI-surface + governance-skeleton maps derived from config; the installed workflow's refs rendered from config; cross-field config validation; the self-host acceptance harness.
- **Future (believed next, not committed)** — toolchain/stack selection (WR-1); phase-2 onboarding + collision merge/chaining (WR-2, WR-3).
- **Out** — system-tree-internal paths (NG-1); module path / release host (NG-2).

### 4.3 Experience: flows and states

*Skipped: no user-facing flow or screen states — the observable behavior is "the engine resolves every target-facing surface from config," specified as requirements in §5.1 and failure handling in §5.2.*

### 4.4 Scenarios

- **Self-host on a non-default config.** A maintainer writes a repo config that relocates doc paths, renames the default branch, sets an explicit remote, and moves the surface pointers; runs the full gate + local `gate` + `doctor`; all green — proving no surface is left behind.
- **A relocated doc corpus.** A repo keeps its designs under a different root; the engine's gates read and write there because the path is a config key, not a constant.
- **A renamed default branch.** A repo whose default branch is not `main`; the merge-base checks, hooks, and the installed workflow all reference the configured branch.

**Decision:** Solution gate — this shape at this appetite is the bet. Zero TBDs may pass.

| Reviewer | Role | Date | Verdict |
|---|---|---|---|
| @mdarabi | operator | — | — *(via the introducing PR's operator review)* |

## 5. System requirements

### 5.1 Functional requirements

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-1 | The engine shall resolve every target-facing document path it reads or writes from the repo-config path schema. | a path the engine touches in the adopter's tree must be declarable (G1, F-4) | Must | high | inspection finds zero target-facing doc-path literals in engine code outside the config schema's defaults; self-host green with those paths relocated | Inspection + Test | G1, J1 |
| FR-2 | The engine shall resolve the default branch and the git remote from config wherever it references them, including generated non-Go surfaces. | `branch`/`remote` are assumed today (F-2, installer WR-9) | Must | high | zero `main`/`origin` literals remain in engine ref-resolution; self-host green with a renamed branch and an explicit remote | Inspection + Test | G1, J1 |
| FR-3 | The engine shall derive the CLI-surface pointers and governance-skeleton targets from the configured CLI-surface knob and path schema, rather than from a fixed literal map. | the maps are hardcoded but derivable (F-5) | Must | high | the maps are computed from config; self-host green with non-default surface pointers | Test | G1, J1 |
| FR-4 | When converge runs, the engine shall render the installed CI workflow's branch, remote, and path references from config. | the installed workflow bakes literals today (F-6); non-Go surface (P-1) | Must | high | the generated workflow contains no branch/remote/path literal absent from config | Test | G1, J1 |
| FR-5 | When it loads a configuration, the engine shall reject a cross-field-inconsistent configuration — a path escaping the repository root, a reference to an absent surface — with a named error and no partial application. | config coherence folds into the one job (D-2); extends the closed-schema contract (INV-32) | Must | high | each malformed-config fixture is refused by name; no partial write or resolution occurs | Test | G1, J1 |
| FR-6 | Where a value is absent from the configuration, the engine shall use the current built-in default, preserving today's behavior. | defaults-preserved is the reversibility guarantee (§3.3) | Must | medium | a config setting no non-default value produces behavior byte-identical to the pre-change engine | Test | G1 |

**Non-regression (NR-n)**

| ID | Existing behavior protected | Why protected | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|
| NR-1 | The pyxis repo's own gate, converge, doctor, and executor behavior on its default config | the engine must keep governing itself unchanged while gaining portability | high | the full existing test suite passes unchanged; the default-config self-host is byte-identical to today | Test | FR-6 |
| NR-2 | The closed-schema contract — unknown keys remain hard errors (INV-32) | schema growth must stay closed, not become permissive | high | an unknown-key config is still refused by name after the schema extension | Test | FR-5 |

### 5.2 Unwanted behavior and failure handling

| ID | If… | Then the system shall… | User sees | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|---|
| UB-1 | a configured path escapes the repository root | refuse the configuration and name the offending key | named schema error; nothing resolved or written | Must | high | N path-escape fixtures each refused by name; zero resolutions proceed | Test | FR-5 |
| UB-2 | a configured value is malformed or references an absent surface | fail closed, naming the key and the remedy | named error, nothing partial | Must | med | each malformed fixture named; no partial application | Test | FR-5 |
| UB-3 | a generated non-Go surface (hook, workflow) has drifted from the current config | detect and regenerate it on converge, or name the drift on `doctor` | drift named by `doctor`; converge repairs it | Should | med | a hand-edited generated surface is repaired by converge and named by doctor | Test | FR-4 |

### 5.3 Permissions and abuse cases

| ID | Actor and intent | Vector | Impact | Severity | Mitigated by | Verify |
|---|---|---|---|---|---|---|
| ABUSE-1 | an author aiming a gate away from the tree it should check | a config path pointing a gate at a location outside the repo, evading the check | a gate silently checks the wrong tree | med | FR-5, UB-1 | Test (path-escape refusal) |

### 5.4 Data

*Skipped: no new persistent state or entity — the change extends an existing config schema additively, and the config's dual-read/versioning already exists (installer §3.10). No migration: absent keys take defaults (FR-6).*

### 5.5 Interfaces and integrations

*Skipped: no new system boundary — the config surface is internal to the engine and already consumed by existing components (F-9).*

### 5.6 Non-functional requirements

| ID | Area | Requirement | Fit criterion | Verify |
|---|---|---|---|---|
| NFR-1 | Maintainability | Adding a target-facing surface to the schema shall be a single, uniform extension (a schema key + its default), not a bespoke override path. | a new key follows the existing `paths:` pattern; no per-surface flag mechanism is introduced | Inspection |
| NFR-2 | Operability | `doctor` shall report a configuration whose generated non-Go surfaces have drifted from the current config. | a drifted generated surface appears as a named `doctor` finding | Test |
| NFR-3 | Security | The engine shall keep configuration parsing closed — unknown keys and out-of-repo paths are refused, never honored. | UB-1/UB-2/NR-2 fixtures pass | Test |

### 5.7 Instrumentation — the tracking plan

| ID | Event name | Fires when | Properties (typed) | Source | Feeds |
|---|---|---|---|---|---|
| EVT-1 | selfhost_config_run | the self-host acceptance job runs the engine under the non-default config | `config_profile: string, green: bool` | CI | M1 |

### 5.8 Definition of Done

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-1 | The pyxis repo runs green under the non-default acceptance config (all gates + local `gate` + `doctor`) | M1, FR-1..FR-5 | Test | the self-host CI job link | yes |
| DOD-2 | Every Must FR passes its fit criterion | FR-1..FR-6 | Test + Inspection | CI run + the coherence inspection | yes |
| DOD-3 | No non-regression broken; default-config behavior byte-identical | NR-1, NR-2 | Test | suite result | yes |
| DOD-4 | Zero target-facing surface literals remain in engine code (coherence) | G2, FR-1..FR-3 | Inspection | the grep/inspection result | yes |
| DOD-5 | Malformed/path-escape configs are each refused by name | FR-5, UB-1, UB-2 | Test | fixture suite result | yes |

### 5.9 AI behavior and evals

*Skipped: no model-driven behavior — this is deterministic config resolution.*

## 6. Delivery

### 6.1 Phasing and milestones

| Phase | Purpose | Scope | Exit criterion |
|---|---|---|---|
| Single phase | finish + generalize the config surface | §4.2 "In" | DOD-1..DOD-5 all green (self-host on non-default config) |

### 6.2 Dependencies

| Dependency | Type | Needed by | Status | Owner |
|---|---|---|---|---|
| The existing closed config schema, `converge`, `doctor`, `preview` | system (already shipped, F-1) | single phase | confirmed | @mdarabi |

### 6.3 Risks, assumptions, and rabbit holes

| # | Risk / assumption / rabbit hole | Likelihood | Impact | Mitigation or test | Owner |
|---|---|---|---|---|---|
| R1 | A coherence gap — one surface still hardcoded while the rest honor config | M | H | the coherence inspection (DOD-4) + self-host proof (DOD-1) catch a left-behind surface | @mdarabi |
| R2 | Config delivery to the non-Go shell hooks / installed workflow balloons | M | M | generate-at-converge, not a config-reader in bash (P-1, §7.2 Q1); the design resolves the mechanism | @mdarabi |
| R3 | Scope creep into phase-2 (collision merge, onboarding) | M | M | the §3.4 waiting room draws the line; WR-2/WR-3 are explicit | @mdarabi |

### 6.4 Release criteria

| Lens | Criterion | Evaluated by |
|---|---|---|
| Functionality | Every Must FR passes its fit criterion | CI / test suite |
| Reliability | Default-config behavior byte-identical; malformed configs refused by name | NR + UB suites |
| Supportability | `doctor` reports config drift on generated surfaces | doctor test |

### 6.5 Rollout and reversibility

- **Flag** — none needed; safety is by construction — defaults preserve today's behavior (FR-6), so no installed repo changes until it sets a non-default value.
- **Kill switch** — a repo reverts to a default config; the engine behaves exactly as before.
- **Cutover/migration steps** — none; absent keys take defaults.

| Stage | Audience | Minimum evidence | Advance when | Roll back when |
|---|---|---|---|---|
| self-host | the pyxis repo (dogfood) | the non-default-config CI job green | DOD-1..DOD-5 green | any guardrail (§3.3) breach |

### 6.6 Launch readiness

*Skipped: internal engine tooling with no external launch, comms, pricing, or enablement surface.*

### 6.7 Post-launch review

- **Review trigger** — phase-2 existing-repo adoption (installer WR-1) begins consuming this surface. · **Owner** — @mdarabi
- **Verdict recorded** — adopt / iterate / kill, with the self-host + adoption evidence, on the header's Tracking ref (post-merge lifecycle lives in the tracker, never as edits to this frozen document).

## 7. Alignment

### 7.1 FAQ

**Customer**

- **Q:** Does this let me install Pyxis into my existing repo? — **A:** Not yet — this makes the *engine* able to honor a foreign layout; the actual onboarding (pointing it at a non-empty repo) is the phase-2 follow-up (WR-3).
- **Q:** Will adopting a new config break my repo? — **A:** No — every value defaults to today's behavior; nothing changes until you set a non-default value (FR-6).

**Internal**

- **Q:** Why not lift the toolchain now, so non-Go repos work? — **A:** Self-host can't witness non-Go (pyxis is Go), and `executor.gate` already lets a repo supply its commands; the generalization waits for a real non-Go adopter (WR-1, `brainstorm:…#D-4`).
- **Q:** How do the shell hooks and CI workflow — which can't read the repo config at runtime — honor config? — **A:** By generate-at-converge (P-1); the exact mechanism is the tech design's to settle (§7.2 Q1).

### 7.2 Open questions

| # | Question | Owner | Needed by | Status |
|---|---|---|---|---|
| Q1 | The config-delivery mechanism to non-Go surfaces (shell hooks, generated workflow): generate-at-converge with values baked in vs. any runtime read — and how drift is detected | @mdarabi | technical design (not the solution gate) | open — carried from `brainstorm:config-driven-portability#P-1` |

### 7.3 Decision log

| Date | Decision | Alternatives considered | Why | Who |
|---|---|---|---|---|
| 2026-07-22 | Portability-as-coherence thesis; finish the config schema; scope by what an adopter must vary | mechanical schema-completion; a full portability program; adoption-use-case-first | keeps the schema-completion coherent without over-scoping (`brainstorm:…#Destination`) | @mdarabi |
| 2026-07-22 | Engine is the beneficiary, the phase-2 adopter is the design audience | serve an external user now | phase-1 onboards no one; prove on ourselves, shape for the adopter (`brainstorm:…#D-1`) | @mdarabi |
| 2026-07-22 | One "declare → obey, or refuse by name" job; config validation folds in | split validation into a second job | makes the coherence property testable as one thing (`brainstorm:…#D-2`) | @mdarabi |
| 2026-07-22 | Proof = self-host on a non-default config, no foreign fixture | build a non-pyxis fixture | strong but bounded; no phase-2 pulled forward (`brainstorm:…#D-5`, supersedes `#D-3`) | @mdarabi |
| 2026-07-22 | Toolchain/stack selection deferred out of phase-1 | lift the stack now | speculative while pyxis is Go; `executor.gate` covers the interim (`brainstorm:…#D-4`) | @mdarabi |
| 2026-07-22 | Scope boundary: collision residue, module path/host, system-tree-internal, onboarding all out | absorb some now | config solves relocation; merge/chaining and onboarding are harder, separate problems (`brainstorm:…#D-6`) | @mdarabi |
| 2026-07-22 | Appetite: a focused single-PRD feature, smaller than the installer | program-scale | "finish + generalize" completes an existing schema (`brainstorm:…#D-8`) | @mdarabi |

### 7.4 Changelog

| Date | Change | Author |
|---|---|---|
| 2026-07-22 | 000-base authored from `brainstorm:config-driven-portability` (gated ready-for-prd) | @mdarabi |

## 8. Contract readiness — the executability gate

| Check | Pass? | Notes |
|---|---|---|
| Problem basis accepted — §2 problem gate signed (T2) | ☐ | via the introducing PR's operator review |
| Zero solution-blocking TBDs or open questions (§7.2) | ☑ | Q1 is a design question, not solution-blocking |
| Every Must FR has a fit criterion and a verification method (§5.1) | ☑ | FR-1..FR-6 |
| Every requirement row carries a risk tier; high/critical rows have a declared safety plan | ☑ | high-risk FRs guarded by defaults-preserved (FR-6/NR-1), self-host proof, and coherence inspection; §6.5 reversibility by construction |
| Non-goals drawn (§3.4); non-regression rows named (§5.1) | ☑ | NG-1..2, WR-1..3; NR-1..2 |
| Every primary job (§2.3) is covered by a requirement, a non-goal, or a waiting-room entry | ☑ | J1 → FR-1..FR-5 |
| Every key flow (§4.3) has functional, failure, and non-regression coverage | ☑ | §4.3 skipped (no UI); behavior in FR-1..FR-6, failures in UB-1..3, NR-1..2 |
| Every primary metric (§3.2) has instrumentation (§5.7) or an explicit measurement plan | ☑ | M1 ← EVT-1 (self-host CI job) |
| Every blocking DoD row traces to a requirement, guardrail, release criterion, or human-judgment rubric (§5.8) | ☑ | DOD-1..5 trace to M1/FR/NR/UB |
| Risks and assumptions have owners (§6.3) | ☑ | R1..R3 |
| Human-judgment criteria name an owner and a rubric (§5.8) | ☑ | none — all DoD rows are Test/Inspection |
| DoD complete: every blocking row sourced and evidenced; every n/a explained (§5.8) | ☑ | DOD-1..5 all blocking, sourced, evidenced |
