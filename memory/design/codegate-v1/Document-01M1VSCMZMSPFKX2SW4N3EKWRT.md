# codegate v1 — the verification harness — technical design

| Field | Value |
|---|---|
| **Status** | Approved *(2026-07-16 — operator approval in the introducing session; unmanned review: 2 rounds, 21/21 findings applied — docs/reviews/codegate-v1.md)* |
| **Owner** | @mdarabi |
| **Approver(s)** | @mdarabi (operator — named reviewer required: Security/privacy impact is yes) |
| **Date** | 2026-07-16 |
| **Relevant-when** | implementing or changing the verification harness — the codegate binary, the ladder/rungs and their checks, the verdict/finding schemas, the verification manifest / baseline / ecosystem defaults, incremental re-verification, or falsification mechanics; implementing the host-side VerificationGate adapter; triaging a wrong-block, missed-catch, parity, or stale-verdict defect; superseding the harness shape |
| **Scope** | The harness's concrete shape: the `internal/verify` core + `cmd/codegate` binary and its shipping, the subprocess invocation contract (preconditions, exit classes, verdict artifact, findings), the five v1 rungs and their check registry, verdict-as-state incremental re-verification, ephemeral-worktree falsification + data-table detection floors, and the manifest/baseline/ecosystem-defaults data model. **Not**: drive-side behavior (implement-now's contract); merge/landing (merge-lane); remote-gate enforcement wiring (the basis's WR-5); detection-assisted onboarding (WR-6); judgment/coverage/e2e rungs (WR-1..4); autonomy pricing. |
| **Tracking** | agenticlab-ai/pyxis#373 *(the introducing PR)* |
| **Builds-on** | implement-now (the host whose Worker consumes this contract) · repo-layout-and-system-footprint (tool-binary shipping/pinning conventions) · distributed-check-registration (the derived-registration pattern this instantiates anew) |
| **Supersedes** | — |
| **Serves** | prd:implementation-verification/FR-1 · prd:implementation-verification/FR-2 · prd:implementation-verification/FR-3 · prd:implementation-verification/FR-4 · prd:implementation-verification/FR-5 · prd:implementation-verification/FR-6 · prd:implementation-verification/FR-7 · prd:implementation-verification/FR-8 · prd:implementation-verification/FR-9 · prd:implementation-verification/FR-10 · prd:implementation-verification/FR-11 · prd:implementation-verification/FR-12 · prd:implementation-verification/FR-13 · prd:implementation-verification/FR-14 · prd:implementation-verification/FR-15 · prd:implementation-verification/FR-16 · prd:implementation-verification/FR-17 · prd:implementation-verification/FR-18 · prd:implementation-verification/FR-19 · prd:implementation-verification/UB-1 · prd:implementation-verification/UB-2 · prd:implementation-verification/UB-3 · prd:implementation-verification/UB-4 · prd:implementation-verification/UB-5 · prd:implementation-verification/UB-6 · prd:implementation-verification/UB-7 · prd:implementation-verification/UB-8 · prd:implementation-verification/UB-9 · prd:implementation-verification/NR-1 · prd:implementation-verification/NR-2 · prd:implementation-verification/NFR-1 · prd:implementation-verification/NFR-2 · prd:implementation-verification/NFR-3 · prd:implementation-verification/NFR-4 · prd:implementation-verification/NFR-5 · prd:implementation-verification/NFR-6 |
| **Security/privacy impact** | yes — the harness executes repo-supplied commands (untrusted input) inside worker workspaces where ambient credentials exist; mitigations are the env-strip/bound runner with platform-scoped write enforcement (DINV-5) and the no-mutation rule (DINV-7) · reviewer: @mdarabi (operator) |
| **Template version** | tech-design-template/v1 |

## 1. Problem

The contract exists; the shape does not. `prd:implementation-verification` (merged) defines what the harness must do — a latency-ordered deterministic ladder with anti-gaming checks, technology-neutral via declared verbs, honest on brownfield repos, portable unchanged to remote CI — but decides nothing about artifacts, seams, state, or schemas. Meanwhile the host it must serve became concrete mid-flight: implement-now's design merged with a Worker of five ports, a drive-to-green loop whose local gate is a **CI mirror** (its DINV-4), and no verification seam. Every week the shape stays undecided, the executor's Phase-B dogfood — which needs this harness as its local bar — slips, and the failure mode the basis names ("green that isn't green") stays open in the factory's hottest loop. The cheapest moment is now: the seam can be designed against the sibling's *frozen* reality instead of a moving draft, and the first implementation can land on ground that will not shift under it.

## 2. Decision

**Decision:** The verification harness ships as **codegate** — a sha256-pinned static Go binary (`cmd/codegate`) over an importable `internal/verify` core, shipped and acquired exactly like docgate (`design-brainstorm:implementation-verification-tech-design#D-1`, `#D-8`) — and hosts consume it through a **subprocess contract**: a clean workspace + diff base + mode in; exit class, one atomic revision-bound verdict artifact, and machine-parseable findings out (`#D-2`, `#D-3`). The prior verdict is the *only* incremental state (`#D-4`); falsification reverts in an ephemeral worktree, never the agent's tree (`#D-5`); repo binding is a closed-schema manifest on the pyxis-config surface with ecosystem defaults compiled into the binary (`#D-6`); checks self-register into codegate's own registry, DCR-style (`#D-7`).

Why this is safe: the identical pinned artifact runs in the local loop today and as the remote gate later (AP-11 — one gate implementation, both surfaces); the seam adds a thin port on the host's side without touching its frozen contract (their CI-mirror local gate coexists; nothing is redefined); and every stateful surface is either the revision-bound verdict (AP-13) or human-lane repo content — there is no hidden state to drift.

## 3. The design

### 3.1 The artifact and its core

`internal/verify/` holds everything real: ladder execution, the bounded verb runner, the check registry, the closed schemas (manifest, verdict, finding), the ecosystem defaults, and the falsification/test-delta engines. `cmd/codegate` is a thin main (the repo's mandated shape). Shipping follows repo-layout-and-system-footprint: goreleaser-style sha256-pinned static binaries per version; operand repos pin and fetch to the gitignored cache; this repo runs source. The future single engine binary absorbs `internal/verify` as a subcommand without reshaping (`#ALT-1`; `#ALT-3` rejected — a library alone cannot satisfy the headless CI dry-run, and M4's parity claim weakens to "identical library").

**Decision:** the operand acquisition config (repo-layout's closed-parse pin file) gains an **additive `codegate:` pin section** — version + sha256, the same shape as the existing `docgate:` section. This design declares that additive schema evolution (AP-6's additive line applied to a live design's closed schema — repo-layout's binding statements all stay true); the section ships with codegate's first release, and the pin path joins the machinery-protection set (§3.7) — a proposal diff that downgrades the pin is gate-gaming, not housekeeping.

**Runtime dependencies:** exactly one beyond the repo's own declared verbs — **system git**, with a minimum version checked at startup (below the floor ⇒ configuration-error; the floor is fixed at first release against the `diff`/`-M`/`apply --reverse`/`worktree` feature set §3.8–§3.9 consume). The observed git version is recorded in every verdict's bindings (§3.3 DINV-1): diff, rename-detection, and apply semantics are inputs to what a verdict means.

**Decision:** codegate carries two subcommands at v1 — `run` (the ladder) and `capture-baseline` (operator-run, §3.10) — plus the conventional `--help`/`version`. New subcommands arrive by amendment to the basis or a successor design, never silently.

### 3.2 Boundaries and non-goals

- **Not solved here** — drive-side behavior (launching/prompting agents, fix caps, escalation — implement-now's contract); landing (merge-lane); remote enforcement wiring, required-check registration, rollout (WR-5); detection-assisted onboarding and manifest drift checks (WR-6); coverage/traceability/judgment/mutation/e2e rungs (WR-1..4); autonomy pricing (the basis's NG-2).
- **Owned by a future design** — the remote-gate CI job and its enforcement semantics (WR-5, basis Q1); sub-rung incremental caching (a successor with its own state design — `#O-1`); ladder subsumption of required CI via custom check bindings (WR-7, revisited at its trigger).
- **Implementation must not infer** — that codegate may merge, approve, or write anything beyond its verdict and scratch (it may not — basis NG-3, DINV-6); that a skip is a pass (green-degraded is a distinct state — FR-17); that codegate replaces the host's CI-mirror local gate (it coexists; their DINV-4 stands — `#F-2`); that a verdict may be committed (DINV-2); that a missing manifest field may be guessed at runtime (declared or ecosystem-default, else absent — FR-5/FR-6); that a dirty tree is verifiable (it refuses — §3.6).
- **Must remain unchanged** — implement-now's drive-side contract rows and its five ports (the seam adds a port; it modifies none — basis NR-2); docgate and the `docs-gate` CI context; the repo's existing required checks (NR-1).

### 3.3 Invariants

- **DINV-1** — every verdict names exactly what it evaluated: the workspace head revision **of the asserted-clean tree** (§3.6), the resolved diff base, the manifest content hash, the baseline content hash, the codegate version, and the observed git version. A **certification read** (attaching evidence, judging a proposal) treats a verdict whose recorded revision differs from the object at hand as **absent**; the one sanctioned non-certification read is §3.8's incremental state-read, where the prior head sha is a diff anchor, never a certification claim (AP-13; basis FR-4).
- **DINV-2** — a verdict is never committed to the repo it certifies: committing it would change the revision it binds, self-invalidating; evidence attaches to proposals externally, by the host (`#D-3`).
- **DINV-3** — a pass-class overall state (green · green-degraded) never coexists with any failed or errored check result, **and never contains a `skip:not-reached` rung** — every rung in a pass-class verdict was executed or check-level-skipped with a recorded reason (fail-closed; basis FR-19, FR-17).
- **DINV-4** — every configured rung **and every registered check within it** appears in every verdict — pass, fail, `skip:<reason>`, `skip:not-reached` (rung level only), or error; silent absence is unrepresentable in the schema (basis FR-4/FR-6).
- **DINV-5** — every verb executes wall-clock-bounded with an environment stripped to the manifest's per-verb entries (ambient environment unreachable by construction), and with filesystem write scope **enforced where the platform provides enforcement** (Linux: kernel-level, §3.5) — elsewhere reduced to strip + redirection, a named residual (§3.5; basis FR-14).
- **DINV-6** — `run` mode writes nothing but the verdict artifact and gitignored scratch; machinery (manifest, baseline) is written only by explicit operator-run subcommands (`capture-baseline`), never by `run` (`#D-6`; basis FR-8's flag stays meaningful because the gate's own runtime cannot be the editor).
- **DINV-7** — verification never mutates the workspace under verification: falsification's revert executes in an ephemeral worktree, and a completed or killed run leaves the workspace tree byte-identical (`#D-5`; verified by A-3's tree-hash leg).
- **DINV-8** — incremental state is the prior verdict and nothing else: no prior verdict, or a mismatch in **any** DINV-1 binding other than the head revision (diff base, manifest hash, baseline hash, codegate version, git version), ⇒ a full ladder run (`#D-4`; basis FR-3).

### 3.4 Observability

- **Events/logs** — structured slog per run: `harness_run` (the basis's EVT-1 fields: run id, workspace id, host-supplied opaque invocation context, head sha, diff base, manifest/baseline hashes, mode, per-rung and per-check result+duration, verb runs, wall ms) and `finding_emitted` (EVT-2); the stdout summary prints state + leading findings + the artifact path.
- **Metrics** — per-run durations and verb counts (M3's incremental ratio computes from EVT-1 pairs by workspace id); catch-suite results (EVT-4) and dry-run parity (EVT-5) land in the harness's own CI.
- **Alerts** — none in v1: codegate is an invoked tool with no daemon; escalation on repeated failure is the host's contract (implement-now's caps/escalation).
- **Debugging surface** — the verdict artifact is self-describing (DINV-1/-4); full tool output is retained by reference in run scratch (findings carry `log_ref`). **Scratch retention:** the current run's scratch plus the prior verdict survive; older run scratch is reaped at the start of the next run (the AP-12 janitor, §3.6/§3.9 mechanics). Any wrong verdict is replayable: `codegate run` on the same clean inputs reproduces it byte-for-byte modulo timing fields (NFR-2).
- **Trace/correlation** — `run_id` plus the host-supplied `invocation_context` passthrough (opaque to codegate — FR-15's no-factory-metadata rule holds; the host, not codegate, knows what a task is).

### 3.5 Security & privacy

Matches the header row: **yes**. Surfaces: (1) **untrusted commands** — every verb is a repo-supplied command line; codegate executes them under DINV-5's runner. Environment: stripped to the manifest's per-verb entries — prevention by construction, exercised by A-6's ambient-var fixture. Time: per-verb wall-clock bound plus a termination grace, both recorded in the verdict schema (UB-2). Filesystem write scope, **by platform**: on **Linux** (the CI dry-run, fleet workers) the runner confines verb writes to the workspace + scratch via **Landlock** (kernel ≥5.13; self-contained — no external sandbox binary, NFR-3 holds); an out-of-scope write attempt fails the rung as **error (exit 3)** with a `violation`-classed finding naming the path. On **macOS** (the operator's attended dogfood) no self-contained enforcement exists: the posture is strip + cache/TMP redirection into scratch, and the write-scope guarantee is a **named residual** against the basis's ABUSE-4 — A-6's write-scope leg is pinned to the Linux CI matrix. (2) **Ambient credentials** — the strip-by-default posture makes exfiltration-by-verb require an explicit manifest env entry, which is human-lane content FR-8 flags when a proposal diff touches it. Manifest env entries may **pin values** as well as pass names (§3.10) — ecosystem defaults pin toolchain-cache variables (GOCACHE-class) into scratch, so verbs run warm without widening the strip; the write-scope rule treats scratch and pinned cache paths as in-scope. (3) **The gaming adversary** — the optimizing implementer (basis ABUSE-1..5): rung-3 checks + machinery protection are §3.7/§3.9; residuals stay as the basis states them (conditional-test detection → WR-3; bypass-wholesale closed at WR-5). No user/personal data; no network egress (NFR-2/NFR-5).

### 3.6 The invocation contract

**Decision:** one subprocess protocol, identical for every host (`#D-2`, `#D-3`):

```
codegate run --workspace <path> --diff-base <ref> [--incremental]
  → exit 0  pass-class (green | green-degraded — read the artifact)
  → exit 1  red (findings attached)
  → exit 2  configuration or usage error (I3 discipline: any invalid, ambiguous,
            or flag-like-positional invocation — or a dirty workspace — refuses
            before any rung executes — UB-7, UB-3, UB-5)
  → exit 3  rung error (infra fail-closed: spawn failure, timeout, revert-inapplicable,
            unparseable test output under a baseline, out-of-scope write — UB-1/2/4/9, §3.5)
```

**Precondition — the clean tree:** the workspace must be git-clean (codegate's own scratch excepted) at invocation; a dirty tree refuses with exit 2 and a configuration-error verdict. Every stateful mechanism below — DINV-1's revision binding, §3.8's diff anchor, §3.9's worktree — is sound *because* of this precondition; on a dirty tree the head sha under-identifies the evaluated state and the verdict would certify the wrong object. The host commits before invoking (the VerificationGate adapter's obligation, stated at integration); this is the derivation the recorded rulings force — binding a dirty tree would re-import exactly the in-place mutation gymnastics `#D-5` rejected.

The **verdict artifact** is one JSON document written atomically (temp + rename — UB-6's no-partial-verdict is structural) to the harness's fixed scratch location inside the workspace — the concrete path is a constant of the verdict schema, and the stdout summary prints it, so hosts never hardcode it — embedding per-rung **and per-check** results, the findings, and DINV-1's bindings. Rung entries carry their checks' individual results plus a rung aggregate; a rung with any check skipped (e.g. its verb unbound) aggregates as pass-with-skips and drives the overall state to green-degraded (FR-6/FR-17 decidable at check granularity). Rung 2 additionally records observed **baseline shrinkage** (baseline entries not reproduced by the run — the ratchet's observability hook, basis FR-10). **Emission rule:** when the workspace path resolves, configuration errors write the artifact *and* the stdout summary; when the defect is the workspace flag itself, the configuration-error verdict is **stdout-carried** (same schema, no artifact) — UB-7/UB-3/UB-5's "emitting only a configuration-error verdict" holds either way. **Exclusion ownership:** at first run in a workspace codegate ensures its scratch directory is excluded via `.git/info/exclude` (local metadata, not repo content — DINV-6 holds) and refuses with a configuration error if the verdict path is tracked; the verdict path also joins the machinery-protection set as belt-and-braces (DINV-2).

The **finding schema** is the joint feedback contract with the host's fixer brief: `check_id`, `class`, `rung`, `path`, `line` (where the class has one), `message` (one line), `log_ref` (full output by reference, never inline). stdout carries a bounded summary; the artifact is the truth and hosts must read it — the stale-verdict rule (DINV-1) requires the head sha anyway. At integration, implement-now's Worker consumes this through a thin consumer-side **VerificationGate** port whose sole adapter execs the pinned binary (`#S-2`); the integration lands via the queued additive amendment on prd:implement-now, the small superseding design on their cycle, and the corrective amendment on this basis's NR-2 (`#D-2`'s P-1 disposition, all three legs; this design changes nothing of theirs).

**Concurrency and residue:** one invocation per workspace, guarded by an **flock-held descriptor** on a scratch lock file — the kernel releases it on process death, so a killed holder never wedges the workspace (the file itself is inert; existence means nothing — the AP-12 in-flight predicate is the held lock, not the file). Interrupted-run residue is reaped at the next invocation's start: codegate prunes its own stale ephemeral-worktree registrations (identified by its run-id path namespace, §3.9) and drops older run scratch per §3.4's retention rule.

### 3.7 The ladder, the rungs, and the registry

Checks self-register into codegate's **own** registry — one self-contained file per check under `internal/verify/`, `init()`-registered, declaring its **rung**, its **verb dependencies**, its **author-independence class**, and its **input set** (§3.8); the ladder's composition is derived from the registry at startup, never hand-listed (`#D-7`; AP-27 — a new check is one new file plus zero shared-line edits, which is NFR-6's fit made structural). The v1 registry populates the basis's five rungs:

| Rung | Checks (registered units) | Verbs consumed |
|---|---|---|
| 0 — instant | format, lint (repo verbs) · **secret scan** · **forbidden-artifact hygiene** · **machinery protection** | `format-check`, `lint` |
| 1 — build | build/typecheck | `build` |
| 2 — tests | full suite; **no-new-failures** evaluation while a baseline is bound — failure identities parsed per §3.10; unparseable output with a baseline bound ⇒ rung error, never an exit-code fallback (UB-9), and a missing or malformed bound baseline ⇒ configuration error with rung 2 never running (UB-5 — no baseline-less fallback); observed shrinkage recorded (§3.6) | `test` |
| 3 — agent-code | **falsification** (§3.9) · **test-delta audit** (§3.9) | `test` (in the ephemeral worktree) |
| 4 — reality | exec-probe (where bound; absent ⇒ `skip:verb-absent`) | `exec-probe` |

The three verbless rung-0 checks, designed: **secret scan** — pattern rules over the **proposal diff's added lines** (diff-scoped: rung-0 latency owns NFR-1's 10 s p95, and pre-existing tree secrets are brownfield reality the baseline philosophy already covers; the scoped-scan lesson is paid for — `brainstorm:implementation-verification` orientation, the predecessor's gitleaks fixture); rules live in the ecosystem table with a manifest extension point (§3.10). **Forbidden-artifact hygiene** — diff-scoped rules: binary-content heuristics + size threshold + baked-absolute-path patterns, same data homes. **Machinery protection** — the diff touches a protected path: the set is **derived** — the manifest's own path plus its declared paths (baseline location), the acquisition pin path (§3.1), and the verdict/scratch path (§3.6) — never a hand-kept list.

Execution is latency-ordered with first-failure stop (FR-1): a failing rung returns its findings without executing later rungs; unreached rungs record `skip:not-reached` (DINV-4) — and can never appear in a pass-class verdict (DINV-3).

### 3.8 Incremental re-verification

**Decision:** the prior verdict is the only state (`#D-4`, `#ALT-8`). An `--incremental` invocation reads the workspace's prior verdict; it qualifies as state only when **every DINV-1 binding except the head revision matches** — resolved diff base, manifest hash, baseline hash, codegate version, git version — and it parses (DINV-8); otherwise the run is full. When it qualifies, the re-run set is: the previously-failed rung first (FR-3), **plus every rung not recorded pass** (unreached and errored rungs re-run — the record's rule, restored), **plus every rung whose input set changed** under `git diff <prior head sha>`. Only prior passes with unchanged inputs carry forward, marked `carried-from <run id>`.

**Input sets are registry data** (each check declares its own; §3.7): v1 defaults are conservative — the whole non-scratch tree for repo-verb checks (rungs 0–2, 4: a whole-repo command's inputs are honestly the tree) and the proposal diff's composition for rung 3. Under these defaults carried passes are rare, and that is the honest claim: **the primary fix-cycle economy is first-failure ordering** — most fix cycles end at a cheap early rung without paying for the suite — with carried passes the secondary optimization that data-level input refinement can grow later. M3 measures the composite.

### 3.9 Rung-3 mechanics

**Falsification** (basis FR-11/FR-18): applicable only when the proposal diff's non-test half is non-empty *and* test files were added or modified; otherwise the check records not-applicable and emits nothing. When applicable, codegate creates an **ephemeral worktree** at the workspace HEAD under a **run-id-unique scratch path** (stale registrations from dead runs are pruned at run start — §3.6), provisions it with the manifest's declared **state dirs** (node_modules-class directories linked or copied per ecosystem defaults — a bare checkout cannot run most stacks' tests), and runs a **two-leg protocol** under DINV-5's runner:

1. **Sanity leg** — the *unreverted* worktree runs the targeted tests (focused where the stack's runner supports it, whole-suite otherwise); they must pass — a failure here is an **environment or flake error** (rung error, exit 3), never a falsification verdict. This leg is what stops an unrunnable worktree from masquerading as "the tests bite."
2. **Revert leg** — the reverse of the non-test half is applied; the same tests run; tests that *pass* without the implementation are tautological ⇒ a falsification finding naming each. A revert that does not apply cleanly ⇒ rung error (UB-4).

The worktree is removed at run end; the agent's workspace is never touched (DINV-7; `#D-5` — in-place revert rejected as a workspace-corruption class the basis never priced). The two-leg cost is bounded by test focusing where available and is Phase-A spike ground (R4).

**Test-delta audit** (basis FR-12): structural, from git, with **floors as data**: deletions from diff file-status over the manifest's test globs; skip/exclusion markers and assertion patterns from per-stack entries in the ecosystem table (§3.10) — an added skip marker, or a modified test whose assertion count drops, produces a test-delta finding; git rename detection (`-M`) runs before any floor fires. Deepening detection (basis Q4) is table growth, not harness code — FR-16's zero-per-stack-code fit stays honest.

### 3.10 The data model

**The verification manifest** — one versioned, closed-schema file on the `pyxis-config/` surface (the executor-caps precedent, `#F-4`): declared stack(s), verb→command bindings, test globs, per-verb env entries (**names passed through or values pinned** — §3.5), per-verb time bounds, **state dirs** (§3.9), the baseline path. Unknown keys are errors (UB-3); a manifest absent ⇒ configuration-error, never a defaults-only run (FR-5).

**The baseline** — a committed file at the manifest-declared path: failure identities plus capture provenance (who, when, the verb-output hash). Writes are human-lane: `codegate capture-baseline` (operator-run) executes the test verb, writes the file, and exits — the operator commits it through an ordinary PR; `run` mode never writes it (DINV-6), and a proposal diff touching it is a rung-0 machinery finding (FR-8). Detection-assisted capture stays WR-6.

**Ecosystem defaults** — compiled into the binary as versioned Go data (`#D-6`): per-stack test globs, skip/assertion pattern tables and **secret/forbidden-artifact rules** (§3.7), **failure-identity parsers** for test output (§3.7 rung 2), state-dir defaults (§3.9), and cache-variable pins (§3.5). Manifest overrides win; the table versions with the codegate release — one *codegate* version input to determinism, with the observed git version recorded alongside in every verdict (`#D-6`; the runtime-defaults-file alternative rejected under `#D-6` — a second pinned artifact whose skew is the AP-11/I2 class).

### 3.11 Testing strategy

Seams binned per the template menu: the **ladder core, registry, schemas, incremental logic** — in-process (hermetic table-driven tests; golden verdicts; property tests for DINV-1/3/4 including the not-reached exclusion and per-check completeness). The **verb runner + rungs** — local-substitutable: **fixture repos** (Go plus two other stacks — FR-16's matrix) with real manifests and scripted verbs, exercising every UB row by failure injection (missing binary, hanging verb, garbage output, malformed manifest/baseline, dirty tree, kill-mid-run — including kill-during-rung-3, concurrent invocation, malformed invocation). The **catch suite** — the basis's three seed families (tautological test · silenced test in all three trigger classes · machinery edit including a pin-downgrade case) as permanent fixtures (M2), paired with the legitimate-diff fixtures (test-only, refactor-only, mixed) that gate wrong-blocks — rung-3 legs on all three stacks (R4). The **host seam** — the CLI contract is the test surface (fixtures drive `codegate run` exactly as the VerificationGate adapter will); the advisory GitHub Actions dry-run (one workflow, explicitly non-blocking) proves headless parity rung-for-rung on one commit (M4), which also exercises the Linux write-scope enforcement (A-6). No port is introduced inside codegate that only one adapter would implement — the subprocess boundary *is* the seam.

## 4. Alternatives considered

| Alternative | Why rejected |
|---|---|
| Verbs on the existing docgate binary (`design-brainstorm:implementation-verification-tech-design#ALT-2`) | identity/coupling: a harness bug forces a docgate release; rung checks join a documentation-check registry; the `docs-gate` context's meaning blurs |
| Engine-internal library only, no binary (`#ALT-3`) | FR-15's headless dry-run forces a cmd anyway; M4's "identical artifact" weakens to "identical library"; release coupling to the executor |
| In-process library port for the local host (`#ALT-5`) | two execution paths for one predicate — parity drift risk (AP-11); reachable later by supersession via the importable core if M3 shows spawn cost material |
| Ladder subsumes the host's CI-mirror gate now (`#ALT-6`) | reopens the basis's WR-7 deferral and contradicts implement-now's frozen DINV-4 — contract-changing amendments on both ledgers for a v1 that cannot even bind the docs-gate context yet |
| Dedicated incremental-state file (`#ALT-7`) | a second consistency domain (state⇄verdict divergence class) buying sub-rung granularity nothing in v1 measures |
| Verify dirty trees via working-tree content hashing (rejected under FND-1's derivation) | re-imports the in-place mutation gymnastics `#D-5` rejected (uncommitted state cannot be worktree'd cleanly); the clean-tree precondition is the shape the recorded rulings force |
| In-place revert for falsification (rejected under `#D-5`) | mutates the tree under verification — a workspace-corruption class UB-6 forbids and the basis never priced |
| Ecosystem defaults as a runtime data file (rejected under `#D-6`) | a second pinned artifact whose version skew re-creates the AP-11/I2 class; pattern changes are review-shaped and ship correctly with releases |
| Central explicit ladder constructor (rejected under `#D-7`) | re-creates AP-27's founding incident (shared-list collisions); fails NFR-6's fit by construction |
| External sandbox runtime (bwrap-class) for write-scope enforcement | an external runtime dependency colliding with NFR-3's self-contained executable; Landlock is in-kernel and self-contained — macOS carries the named residual instead (§3.5) |

## 5. Risks

| ID | Risk | Impact | Mitigation | Verification / evidence |
|---|---|---|---|---|
| R1 | Falsification wrong-blocks legitimate diffs and trains red-blindness | the gate gets ignored or demoted | the not-applicable disposition is structural (FR-18, §3.9); the sanity leg converts environment noise into rung errors, not findings; paired legitimate fixtures gate every catch fixture; the basis's §3.3 skip-demotion path is a human-lane manifest change | A-3's paired fixtures across three stacks; the basis's wrong-block guardrail during dogfood |
| R2 | Subprocess spawn + two-leg falsification cost per fix cycle proves material | the loop slows; hosts skip invocations | rung-0 latency budget (NFR-1, owned by A-12); first-failure ordering is the primary economy (§3.8); test focusing bounds the two-leg cost; `#ALT-5` reachable by supersession | EVT-1 timing data across the dogfood (M3) |
| R3 | The queued integration changes on implement-now's ledger slip | the harness ships with no host wired | the seam is one thin port + one adapter, confined to their local-gate slot (`#D-2`); the CLI contract is fully testable without them (fixtures drive it) | the amendment/supersession PRs at integration; A-9 runs regardless |
| R4 | Revert-based falsification is not cleanly implementable on some stack (state provisioning, focus support, revert semantics) | rung 3 degrades on that stack | Phase-A spike on the three fixture stacks (the basis's A2) before the rung's contract is declared stable; the two-leg protocol + state-dirs are the spike's subjects; UB-4 owns the inapplicable case fail-closed | the fixture matrix (A-1) includes rung-3 legs on all three stacks (A-2/A-3) |
| R5 | Assertion-pattern floors misfire on exotic test styles | noisy findings erode trust | floors are ecosystem-table data — tightened by data change, not release logic (§3.9); rename detection runs first; disputes ride the disposition guardrail | fixture set grows per disposition; table diffs are reviewed data PRs |
| R6 | The macOS write-scope residual (no kernel enforcement) is exploited by a malicious verb on the attended dogfood surface | out-of-workspace writes on the operator's machine | named residual (§3.5) against basis ABUSE-4: strip + redirection stand; manifest env/verb content is human-lane and FR-8-flagged; the enforced surface (Linux) is where unattended fleet work runs | the residual is recorded here and in A-6's platform pinning; revisited at WR-5 with the remote surface |

## 6. Acceptance

| ID | Acceptance criterion | Verify | Required evidence | Traces to |
|---|---|---|---|---|
| A-1 | Fixture repos of three stacks (Go + two others) bind manifests and run the full ladder with zero per-stack code in `internal/verify` | Test, Inspection | fixture-matrix CI run + inspection note | prd:implementation-verification/FR-16 · §3.10 |
| A-2 | The catch suite blocks all three seed families — tautology; silenced test via deletion, skip-marker, and assertion-removal; machinery edit including a pin-downgrade — permanently in CI, with rung-3 legs on all three stacks | Test | catch-suite CI run (EVT-4) | prd:implementation-verification/FR-8 · FR-11 · FR-12 · §3.7 · §3.9 |
| A-3 | The paired legitimate fixtures pass without findings — test-only (falsification not-applicable recorded), refactor-only, mixed — on all three stacks; after every rung-3 run the workspace tree hash is byte-identical | Test | fixture results + tree-hash assertion | prd:implementation-verification/FR-18 · FR-11 · #DINV-7 · §3.9 |
| A-4 | Verdict schema invariants hold as property tests — DINV-1 bindings present (incl. git version), DINV-3 state exclusion (incl. no `skip:not-reached` in pass-class), DINV-4 rung *and check* completeness; the revision-mismatch fixture is treated as verdict-absent on a certification read | Test | property-test suite | prd:implementation-verification/FR-4 · FR-17 · FR-19 · #DINV-1 · #DINV-3 · #DINV-4 |
| A-5 | A fixture fix-cycle re-verifies with strictly fewer verb runs than the initial full ladder; a changed manifest, baseline, diff base, or codegate version forces a full run; carried passes carry provenance; a prior `skip:not-reached` rung re-runs | Test | fixture results | prd:implementation-verification/FR-3 · §3.8 · #DINV-8 |
| A-6 | The bounded runner: an ambient env var is unobservable; a hanging verb terminates at bound + recorded grace; an out-of-workspace write fails the run with a violation finding **on the Linux enforcement matrix** (the macOS residual is named, §3.5) | Test | failure-injection results (Linux CI) | prd:implementation-verification/FR-14 · UB-1 · UB-2 · #DINV-5 |
| A-7 | Every malformed-invocation class (unknown mode, missing/ambiguous workspace or diff base, flag-like positional) and a dirty workspace refuse with exit 2 before any verb executes, emitting a configuration-error verdict (stdout-carried when the workspace flag is the defect) | Test | UB-7 + dirty-tree fixtures | prd:implementation-verification/UB-7 · §3.6 |
| A-8 | Kill-mid-run — including kill-during-rung-3 — leaves no verdict artifact, releases the lock (flock), and the subsequent run prunes stale worktree registrations and equals an uninterrupted run; a concurrent same-workspace invocation refuses busy | Test | UB-6/UB-8 fixtures incl. the rung-3 kill leg | prd:implementation-verification/UB-6 · UB-8 · #DINV-8 · §3.6 |
| A-9 | The advisory GitHub Actions dry-run produces a rung-for-rung identical verdict to a local run on the same commit | Demo | workflow run + comparison (EVT-5) | prd:implementation-verification/FR-15 · §3.1 |
| A-10 | A demonstration check lands as one new file with zero shared-line edits; the ladder derives it from the registry | Inspection | the demonstration diff | prd:implementation-verification/NFR-6 · §3.7 |
| A-11 | `codegate capture-baseline` produces a baseline the red-main fixture binds as no-new-failures; a baseline-shrink run records the shrink in the verdict; `run` mode performs zero git mutations and never writes manifest or baseline; the verdict path is excluded or the run refuses | Test | red-main + shrink + write-audit fixtures | prd:implementation-verification/FR-10 · #DINV-2 · #DINV-6 · §3.10 |
| A-12 | Rung 0's neutral checks: a seeded secret and a committed binary in the proposal diff each red rung 0 with class-named findings; rung-0 wall-time meets NFR-1's p95 ≤ 10 s on the dogfood repo | Test, Analysis | fixture results + EVT-1 rung-0 timings | prd:implementation-verification/FR-7 · NFR-1 · §3.7 |

## 7. Open questions

| ID | Question | Owner | Blocks | Status | Resolution |
|---|---|---|---|---|---|
| Q1 | The finding schema's final field freeze against the host's fixer-brief consumption (the joint contract's other half) | integration (this design + implement-now's adapter PR) | Phase-B integration | open | §3.6's field set is the v1 proposal; the adapter confirms it at integration, and any post-proposal field change lands as a successor design superseding §3.6's decision — the durable altitude (machine-parseable; id + location + message; logs by reference — basis FR-2) binds regardless |
| Q2 | Per-stack assertion/skip pattern depth beyond the v1 floor | implementation (ecosystem-table data) | nothing | open | grows by data PR per dogfood dispositions (§3.9, R5) |
| Q3 | The termination-grace constant's value | implementation | nothing | open | fixed in code at first release; recorded in the verdict schema (§3.6, UB-2) — the exit-code protocol itself is bound by §3.6 |

## Principles

- **AP-10 — honored.** Verification is code, not prompt discipline: every rung is deterministic, the ladder derives from a registry, floors are data.
- **AP-11 — honored.** One gate implementation runs on both surfaces (the pinned binary local and remote); the in-process alternative was rejected precisely for splitting the predicate.
- **AP-12 — honored.** Every throwaway resource ships with its liveness predicate and janitor: the lock is a kernel-released flock (the in-flight predicate is the held descriptor, never file existence); stale worktree registrations and old scratch are reaped at next-invocation start (§3.6, §3.4, §3.9).
- **AP-13 — honored.** A verdict certifies the exact revision set it evaluated (DINV-1, on an asserted-clean tree) and is treated as absent on any certification-read mismatch; it is never committed into what it certifies (DINV-2).
- **AP-5 / AP-6 — honored.** Structured events (EVT-1/2), closed schemas parsed closed (manifest, verdict, finding), additive schema evolution owned explicitly (§3.1's pin section), trace context passed through opaquely.
- **AP-19 — honored.** Zero model tokens in operation; zero cost at idle — codegate runs only when invoked.
- **AP-21 — honored.** Green's definition is frozen human-confirmed data (the manifest); runtime never guesses a binding.
- **AP-27 — honored.** The check registry is derived by self-registration; no author's diff touches a central list (NFR-6 structural).
- **AP-16 — honored.** Every verb execution is bounded (time, environment, platform-scoped writes) and fail-closed on the bound (UB-1/2); retry policy belongs to the host, not the gate.
- **AP-23 — honored.** Per-file check registration keeps parallel check authors collision-free — the same property the registry pattern was born from.
- **RW-4 — honored.** The core is hermetic and substrate-first: the whole ladder runs against fixture repos with no network, no agent, no real CI.
- **RW-5 — honored.** One decision — codegate's v1 shape; the subordinate choices (§3.6–§3.10) change only together with it, and independently-revisable successors (remote enforcement, sub-rung caching, subsumption) are named as future designs, not bundled.
- **Deviation:** none.
