# Implementation-verification harness — design-brainstorm record

| Field | Value |
|---|---|
| **In one line** | Design-space grill for the stage-8 verification harness (prd:implementation-verification): delivery shape, the host seam against the merged implement-now Worker, verdict/finding transport, incremental re-verification state, falsification mechanics, the manifest/baseline data model, and rung/check registration. |
| **Relevant-when** | designing or changing the verification harness's shape or home; its seam with implement-now's Worker (drive-to-green, local gate); verdict/finding schemas and their revision binding; incremental re-verification state; falsification/test-delta mechanics; the verification manifest / ecosystem defaults / baseline data model; rung registration — or checking prior design rulings and refused shapes for stage-8 verification |
| **Basis** | prd:implementation-verification (merged ledger, PR #356, 2026-07-16) |
| **Authority** | @mdarabi |
| **Date(s)** | 2026-07-16 |
| **Status** | gated: ready-for-design |
| **Tracking** | agenticlab-ai/pyxis#373 *(the introducing PR)* |
| **Feeds** | docs/tech-design/codegate-v1.md |
| **Prior records** | brainstorm:implementation-verification · design-brainstorm:implement-now-tech-design |
| **Template version** | design-brainstorm-record/v1 |

## Orientation

| Source | Why loaded |
|---|---|
| docs/prds/implementation-verification/effective.md | the basis — FR-1..19, UB-1..9, NR-1..2, NFR-1..6, WR-1..8; authored + folded this session, held verbatim |
| docs/tech-design/index.md (origin/main) | the live decision graph — nothing serves prd:implementation-verification yet; implement-now live serving its FR-1..FR-20; the Builds-on candidates (docgate-v1, distributed-check-registration, merge-lane-v1, repo-layout) |
| docs/tech-design/implement-now.md (live, merged this day) | the host's shape of record: Worker of five consumer-side ports (WorkTracker · Lease · Workspace · AgentRuntime · CodeHost) — **no verification-harness port**; drive-to-green = local gate + bounded CI self-heal feeding redacted+clamped CI logs to a fixer; **DINV-4 binds the Worker's local gate to "the same deterministic checks required by CI"** (AP-11, single-source predicates); DINV-3 verify-by-artifact |
| docs/brainstorms/implement-now-tech-design.md (frozen) | the sibling's rulings that constrain the seam: D-1 (port decomposition), D-3 (fresh-headless runtime), D-6 (caps as closed-schema policy on the pyxis-config surface — the precedent for the manifest's home) |
| docs/tech-design/repo-layout-and-system-footprint.md (live) | tool-binary conventions: sha256-pinned static per-version releases, gitignored fetch cache, this repo runs source, committed binaries rejected (DINV-2 no unpinned execution) |
| docs/tech-design/distributed-check-registration.md (live) + `internal/docgate/gate/` | the derived-registration precedent (per-file init() into a registry, no shared enumeration — AP-27) for NFR-6 |
| docs/architecture-principles.md + incident registry (via this session's earlier context pack) | AP-11 mirror · AP-13 revision binding · AP-21 judge-once · AP-27 derived aggregates · AP-5/6 data · AP-19 idle-free; incidents I2 (badge/base) · I3 (CLI misparse) |
| docs/wiki/glossary.md | verification manifest · ladder/rung · falsification check (landed with the basis PR); floor-clearing; in-session run |

**Overlaps surfaced:**

- `design:implement-now` (live) — owns the Worker's drive-to-green and defines its local gate as a **CI mirror** (DINV-4); has no harness seam. Constrains DQ-2; does not cover it — it decides the Worker's shape, not the harness's.
- `prd:implement-now` **merged** (FR-1..FR-20 frozen) — the basis's NR-2 fit ("integration lands as a draft-phase edit") is now stale → parked as **P-1**, disposition asked under DQ-2.
- `design:merge-lane-v1` (live) — the floor consumes "every required mechanical check green"; a remote-phase consumer (WR-5), untouched here.
- `design:distributed-check-registration` (live) — the registration pattern DQ-7 extends rather than reinvents.
- `design:repo-layout-and-system-footprint` (live) — the shipping/pinning conventions DQ-1's alternatives must honor.
- `design:prepush-ref-awareness` (live) — pre-push ref-range semantics; the desk/CI parity ground the remote phase will reuse (context for WR-5; out of this design's scope).

No live design covers the harness itself; the ground is open.

## Design questions

- **DQ-1**: What artifact is the harness — a second pinned tool binary shipped docgate-style, a verb-set on the existing docgate binary, or an engine-internal library with no standalone artifact — and where does it live in the repo layout?
- **DQ-2**: What is the host↔harness seam against the merged implement-now shape — a new consumer-side port on their Worker (requiring an amendment/superseding design on their ledger), a subprocess CLI contract with no port, or an in-process library call — and how do findings feed their fixer loop? (Carries P-1's disposition.)
- **DQ-3**: How do verdicts and findings travel — workspace artifact files, stdout payloads, or both — and where do the revision-binding fields and the stale-verdict consumer rule live?
- **DQ-4**: How does incremental re-verification keep state — persisted per-rung input hashes in the workspace, or stateless git-diff-driven recomputation per invocation — and what are the rung-restart rules?
- **DQ-5**: How are falsification and test-delta mechanically implemented — revert-in-place vs a second ephemeral worktree; and what structural form implements the delete/skip/assertion-removal floor?
- **DQ-6**: What is the data model of the verification manifest, the ecosystem defaults table, and the baseline — file homes on the pyxis-config surface, schema/version fields, failure-identity parsers as data, and the v1 operator-run baseline-capture procedure?
- **DQ-7**: How do rungs/checks register — extending the distributed-check-registration pattern into the new artifact — such that NFR-6's derived-aggregate fit holds?

## Panel log

- **DQ-1**: opens (a new executable/module boundary) — fired — the delivery shape is a new seam with at least three structurally distinct homes
- **DQ-2**: opens/replaces (the Worker↔harness interface; touches the sibling's frozen contract) — fired — the seam has genuinely orthogonal shapes (subprocess contract · in-process port · gate subsumption) with different integration mechanics on implement-now's ledger
- **DQ-3**: extends (AP-5/6 closed-schema payloads + workspace-artifact conventions) — skipped — the transport composes existing data discipline; no new seam class
- **DQ-4**: opens (a persistent incremental-state artifact in the workspace) — fired — two structurally distinct shapes exist: a dedicated state file vs the prior verdict as the only state
- **DQ-5**: extends (git plumbing + the bounded verb runner; detection floors as data tables) — skipped — mechanics compose existing seams; the one real choice (revert site) is a two-option ruling, not a new seam class
- **DQ-6**: extends (the pyxis-config closed-schema surface — the caps precedent — plus repo-layout data conventions) — skipped — homes and formats compose existing surfaces; no new seam class
- **DQ-7**: extends (the distributed-check-registration pattern into the new binary's own registry) — skipped — the pattern is live in-repo; the question is adoption, not shape

## Decisions

### D-1 — delivery shape: a second pinned tool binary over an importable core

- **Question**: DQ-1 — what artifact is the harness: a pinned tool binary (library core + thin cmd), verbs on docgate, or an engine-internal library only?
- **Answer**: **ALT-1** — `internal/verify/` core + a thin `cmd/` main, shipped docgate-style (sha256-pinned static per-version releases, gitignored fetch cache; this repo runs source); absorbable later as a subcommand of the single engine binary without reshaping.
- **Reasoning**: cleanest FR-15/M4 (the CI dry-run and future remote gate run the identical pinned artifact); zero docgate release coupling; the importable core keeps DQ-2's in-process option open. Cost accepted: one more bucket-2 acquisition surface. Recommendation adopted.
- **Anchor**: 2026-07-16 · t1
- **Status**: live

### D-2 — the host seam: subprocess contract behind a thin port; P-1 → proceed-anyway

- **Question**: DQ-2 — how does implement-now's Worker acquire the harness (subprocess contract · in-process port · gate subsumption), and how do findings feed its fixer loop? Carries P-1's disposition.
- **Answer**: **ALT-4** — the Worker execs the pinned binary (`run --workspace --diff-base [--incremental]` → exit code + verdict artifact + findings); at integration their Worker gains a thin consumer-side VerificationGate port whose sole adapter execs the binary; findings replace raw-log parsing in the fixer brief. **P-1 → proceed-anyway**: integration lands as (a) an additive amendment on prd:implement-now (invoke per fix cycle + before PR-raise; their CI-mirror local gate and DINV-4 untouched), (b) a small superseding design on their §3.7 cycle, (c) a corrective amendment on this basis's NR-2 stale fit — all queued at integration, Learned-from citing this ruling.
- **Reasoning**: the identical artifact runs locally and remotely (AP-11; M4 stays honest); release-decoupled; their hexagon intact. Spawn cost bounded by NFR-1/M3 — ALT-5's in-process path stays reachable by supersession via the importable core if M3 ever shows spawn cost material. Recommendation adopted.
- **Anchor**: 2026-07-16 · t2
- **Status**: live

### D-3 — transport: one atomic verdict artifact + stdout summary; never committed

- **Question**: DQ-3 — how do verdicts and findings travel: one artifact embedding findings, split files, or stdout-only?
- **Answer**: **one JSON verdict artifact**, atomically written (temp+rename) to a gitignored workspace path, embedding per-rung results, findings (logs by reference), and the revision bindings (head_sha · diff_base · manifest_hash · baseline_hash · harness version); stdout carries a compact summary + the artifact path; exit code is coarse class only (pass-class / red / rung-error / config-or-usage error) — the artifact is the truth and the host must read it. **Never committed**: committing the verdict would change the revision it certifies (self-invalidating) — a DINV candidate for the authored design; the host attaches evidence externally.
- **Reasoning**: one artifact = one consistency domain; UB-6's no-partial-verdict and FR-4's bindings fall out of the write discipline; matches the Worker's artifact-consumption shape. Recommendation adopted.
- **Anchor**: 2026-07-16 · t3
- **Status**: live

### D-4 — incremental state: the prior verdict is the only state

- **Question**: DQ-4 — dedicated incremental-state file, or the prior verdict as the only state?
- **Answer**: **ALT-8** — re-invocation reads the prior verdict: `git diff <prior head_sha>` + recorded rung results → the re-run set (previously-failed rung first per FR-3); carried passes marked `carried-from <run id>`; manifest/baseline hash mismatch or missing/invalid prior verdict ⇒ full run. No second artifact.
- **Reasoning**: UB-6 idempotence structural (no verdict ⇒ no state ⇒ full run); FR-3 invalidation is one hash comparison; one consistency domain kills the state⇄verdict divergence bug class; rung-level granularity is exactly FR-3's bar — sub-rung caching is a future supersession. Recommendation adopted.
- **Anchor**: 2026-07-16 · t4
- **Status**: live

### D-5 — rung-3 mechanics: ephemeral worktree revert; detection floors as ecosystem data

- **Question**: DQ-5 — where does falsification's revert run (throwaway worktree vs in-place), and what implements the delete/skip/assertion-removal floor?
- **Answer**: **ephemeral worktree** — a throwaway worktree at the same HEAD, reverse of the non-test half applied there, test verb run bounded + env-stripped (FR-14 applies), worktree removed; the agent's workspace is never mutated; UB-4 detected in the throwaway. **Floors as data**: deletions via diff file-status on test-glob paths; skip/exclusion markers and assertion patterns as per-stack ecosystem-table entries (assertion-count drop on a modified test ⇒ finding); git rename detection (-M) runs before any floor fires.
- **Reasoning**: the throwaway is the same isolation primitive the host's Workspace port trusts; in-place mutation would create a workspace-corruption class UB-6 forbids and the PRD never priced; floors-as-data keeps FR-16's zero-per-stack-code fit honest — Q4's depth grows by table rows. Recommendation adopted.
- **Anchor**: 2026-07-16 · t5
- **Status**: live

### D-6 — data model: config-surface manifest, committed baseline, compiled ecosystem defaults, operator-run capture

- **Question**: DQ-6 — where do the manifest, baseline, and ecosystem defaults live, and how is the v1 baseline captured?
- **Answer**: manifest = one versioned closed-schema file on the `pyxis-config/` surface (the caps precedent; verb bindings, declared stacks, test globs, per-verb env allowlist + time bounds, baseline path). Baseline = the manifest-declared committed repo path (human-lane; FR-8 flags it in proposal diffs; carries failure identities + capture provenance). Ecosystem defaults = **compiled into the binary as versioned Go data** (globs, D-5's marker/assertion patterns, failure-identity parsers); manifest overrides win; the table versions with the release. v1 capture = an operator-run `capture-baseline` subcommand — run mode never writes it; the operator commits via ordinary PR. Detection-assisted capture stays WR-6.
- **Reasoning**: every home is an existing surface with an existing discipline; compiled defaults keep NFR-2/NFR-3 structural and avoid a second pinned artifact whose version skew would be the AP-11/I2 class; a runtime defaults file's update-cadence win is illusory (pattern changes are review-shaped). Recommendation adopted.
- **Anchor**: 2026-07-16 · t6
- **Status**: live

### D-7 — registration: the DCR pattern in the harness's own registry

- **Question**: DQ-7 — per-file init() self-registration (DCR pattern, new registry instance) or a central explicit ladder constructor?
- **Answer**: **extend the DCR pattern** — each check is one self-contained file in `internal/verify/` registering via init() into the harness's own registry (never shared with docgate's); a check declares its rung, verb dependencies, and author-independence class; the ladder's composition is derived from the registry.
- **Reasoning**: NFR-6's fit becomes structural (new check = new file, zero shared-line edits — AP-27 by construction); the central-constructor shape re-creates AP-27's founding incident and would fail a Must fit by design. Recommendation adopted.
- **Anchor**: 2026-07-16 · t7
- **Status**: live

### D-8 — the tool's name: codegate

- **Question**: (authoring gap-fill, asked once at `/tech-design-new`) — what is the harness binary named: codegate, verify, or pyxis-verify?
- **Answer**: **codegate** — `cmd/codegate` over `internal/verify`; house-symmetric with docgate (docgate gates the docs, codegate gates the code); the capability keeps the PRD's name ("the verification harness"), while codegate is the tool's name.
- **Reasoning**: the naming grammar and shipping shape mirror docgate exactly; `codegate run` / `capture-baseline` / a future `codegate` CI context beside `docs-gate` all read cleanly. Recommendation adopted.
- **Anchor**: 2026-07-16 · t8 (authoring gap)
- **Status**: live

## Alternatives

### ALT-1 — a second pinned tool binary (library core + thin cmd)

- **Shape**: `internal/verify/` (the harness core: ladder, runner, checks, schemas) + a thin `cmd/` main, shipped exactly like docgate — sha256-pinned static per-version releases, gitignored fetch cache, this repo runs source. The engine can later absorb the same core as a subcommand of the single engine binary (the vision's end-state) without reshaping it.
- **Trade-offs**: cleanest FR-15/M4 story — the CI dry-run and the future remote gate run the *identical pinned artifact*; zero release coupling to docgate; costs one more bucket-2 acquisition surface (shim + pin + release lane).
- **Principles**: AP-11 (one gate implementation both surfaces) · AP-10 (codified, not prompted) · RW-4 (hermetic substrate-first core) · repo-layout DINV-2 (pinned execution) honored; strains the system-footprint count (a second tool binary).
- **Incidents**: I3 honored by design (UB-7 usage-refusal); the runtime-bundling lesson (I-series 2026-06-25) is the reason the shape is an un-bundled static Go binary — no runtime deps (NFR-3).
- **Verdict**: chosen — the identical-pinned-artifact story (FR-15/M4) plus release decoupling; the importable core preserves DQ-2 freedom (D-1)
- **From**: DQ-1 · panel

### ALT-2 — verbs on the existing docgate binary

- **Shape**: the harness lands as `docgate verify`/`docgate ladder` verbs — reusing docgate's release/pin/shim machinery and its check registry.
- **Trade-offs**: zero new acquisition surface (docgate is already pinned everywhere an operand repo exists); but couples doc-system tooling to code verification — a harness bug forces a docgate release, the `docs-gate` CI context's identity blurs, and rung checks would join a *documentation*-check registry (semantic mismatch with DCR's scope).
- **Principles**: repo-layout DINV-2 trivially honored; strains the hexagon's package-boundary discipline (one binary, two unrelated capabilities) and AP-27's registry cohesion (one aggregate serving two check domains).
- **Incidents**: none matched (registry checked 2026-07-16).
- **Verdict**: rejected — identity/coupling: a harness bug forcing a docgate release and rung checks joining a documentation-check registry (D-1)
- **From**: DQ-1 · panel

### ALT-3 — engine-internal library only, no standalone artifact

- **Shape**: `internal/verify/` linked into the executor (implement-now's binary); no cmd of its own — the Worker calls the harness in-process.
- **Trade-offs**: tightest local integration (zero subprocess overhead per fix cycle); but FR-15's headless CI dry-run needs *some* executable, forcing a thin cmd anyway; harness releases couple to executor releases; "the identical artifact runs remotely" (M4, D-4) weakens to "the identical library, differently hosted".
- **Principles**: honors the basis D-2's co-location maximally; strains FR-15/M4 (portable-unchanged) and D-9's reusability bet (this core *is* the future remote gate).
- **Incidents**: the green-merge ≠ working-artifact class (2026-06-25 lineage) — a library has no `--help` to exec-probe; the basis's own execution-reality rung argues for a runnable artifact.
- **Verdict**: rejected as primary — FR-15 forces a cmd anyway and M4 weakens to "identical library"; its in-process desire survives as a DQ-2 option via the importable core (D-1)
- **From**: DQ-1 · panel

### ALT-4 — subprocess CLI contract behind a thin consumer-side port

- **Shape**: the Worker execs the pinned binary — `run --workspace <path> --diff-base <sha> [--incremental]` → exit code (coarse class) + verdict artifact + findings; at integration implement-now's Worker gains a thin VerificationGate port whose sole adapter is that exec (the shape of their gh adapters). Findings (check id · location · message) replace raw-log parsing in the fixer brief.
- **Trade-offs**: identical artifact local and remote; release-decoupled; hexagon intact on both sides; costs ~one spawn per fix cycle, bounded by NFR-1's rung-0 budget and measured by M3.
- **Principles**: AP-11 (one gate implementation, both surfaces) · AP-13 (verdict binds the revision, checked at the port) · AP-5/6 (structured payloads) honored; their DINV-4 untouched.
- **Incidents**: I3 honored (UB-7 usage refusal at the CLI boundary); none further matched (registry checked 2026-07-16).
- **Verdict**: chosen — parity by one artifact, minimal integration surface (D-2)
- **From**: DQ-2 · panel

### ALT-5 — in-process library port (local), binary remote

- **Shape**: the Worker's port adapter imports `internal/verify` directly; the remote gate (later) runs the binary — two execution paths for one predicate.
- **Trade-offs**: fastest fix cycles (zero spawn); but local/remote parity (M4) becomes vulnerable to build/version skew — the drift AP-11 exists to prevent; executor binary couples to harness releases; their side needs the same amendment plus a build dependency edge.
- **Principles**: honors the basis D-2's co-location; strains AP-11 (two paths) and the reusability bet (D-9).
- **Incidents**: none matched (registry checked 2026-07-16); the parity-drift concern is I2-adjacent (true-of-the-wrong-object verdicts) but not a recorded incident.
- **Verdict**: rejected for now — reachable later by supersession via the importable core if M3 shows spawn cost material (D-2)
- **From**: DQ-2 · panel

### ALT-6 — subsumption: the harness becomes the Worker's only local gate (WR-7 pulled forward)

- **Shape**: the manifest binds every required CI check as a verb; the ladder subsumes the CI mirror; one predicate, one gate.
- **Trade-offs**: conceptually purest end-state (ladder-green ⇒ CI-green); but reopens the basis's WR-7 deferral without an amendment, contradicts implement-now's frozen DINV-4 (contract-changing amendment + design supersession on their ledger — the heaviest integration surface), and the docs-gate CI context has no verb binding yet, so day-one subsumption is a fiction.
- **Principles**: maximal AP-11 in the limit; violates the waiting-room discipline now (a WR row is a decided deferral).
- **Incidents**: none matched (registry checked 2026-07-16).
- **Verdict**: rejected — the WR-7 trigger ("at implement-now integration or the remote phase") may revisit it, with amendments, when the trigger fires (D-2)
- **From**: DQ-2 · panel

### ALT-7 — dedicated incremental-state file

- **Shape**: a separate gitignored state artifact (per-rung input hashes keyed by manifest hash + baseline hash + rung input sets) written after every run; re-invocation diffs current inputs against it to pick the re-run set.
- **Trade-offs**: state schema tuned exactly for incrementality (can carry finer-grained keys than the verdict needs); but a second artifact with its own atomicity, staleness, and cleanup story — UB-6's "no stale incremental state corrupts the next run" must be proven for two files, and state⇄verdict divergence becomes a possible bug class.
- **Principles**: AP-5/6 honored (another closed schema); strains the one-consistency-domain shape D-3 just chose.
- **Incidents**: none matched (registry checked 2026-07-16); the divergence risk is I2-adjacent in structure.
- **Verdict**: rejected — buys sub-rung granularity nothing in v1 measures, at the price of a second consistency domain (D-4)
- **From**: DQ-4 · panel

### ALT-8 — the prior verdict is the only state

- **Shape**: no second artifact — the previous verdict (D-3) already records head_sha, manifest hash, baseline hash, and per-rung results; re-invocation computes the affected set from `git diff <prior head_sha>` + the recorded rung results (previously-failed rung first, per FR-3), re-running any rung whose inputs changed or that was not pass; carried passes are marked with provenance (`carried-from <run id>`) in the new verdict. Any binding mismatch (manifest/baseline hash) or missing/invalid prior verdict ⇒ full run.
- **Trade-offs**: zero additional state machinery — UB-6 idempotence is structural (no verdict ⇒ no state ⇒ full run); invalidation on manifest/baseline change (FR-3) is one hash comparison; slightly coarser incrementality (rung-level granularity, keyed by git diff), which is exactly the granularity FR-3's fit demands and no finer.
- **Principles**: AP-13 (the state *is* the certified object — no second source of truth) · AP-21 (judge once, read the frozen record) honored; one consistency domain preserved.
- **Incidents**: none matched (registry checked 2026-07-16).
- **Verdict**: chosen — structural idempotence, one consistency domain, exactly FR-3's granularity (D-4)
- **From**: DQ-4 · panel

## Seam map

- **S-1**: the harness artifact — **opens** — behind it: `internal/verify/` core + thin `cmd/` main, sha256-pinned static releases per repo-layout conventions — feeds: the design's Scope ("the harness binary and core") + Serves prd:implementation-verification/FR-15, NFR-3 (D-1)
- **S-2**: the host↔harness invocation contract — **opens** — behind it: the subprocess CLI protocol (workspace + diff base + mode → exit code + verdict artifact + findings), consumed at integration by a thin VerificationGate port on implement-now's Worker — feeds: Scope ("the invocation contract") + Serves prd:implementation-verification/FR-1, FR-2, NR-2; integration mechanics per D-2 (D-2)
- **S-3**: the verdict artifact class — **opens** — behind it: one atomic, gitignored, revision-bound JSON artifact per run, doubling as the only incremental state — feeds: Serves FR-3, FR-4, UB-6 (D-3, D-4)
- **S-4**: the manifest/baseline data surfaces — **extends** — behind it: a closed-schema manifest on `pyxis-config/` (the caps precedent), a committed human-lane baseline at its declared path, ecosystem defaults compiled as versioned Go data, an operator-run `capture-baseline` subcommand — feeds: Serves FR-5, FR-6, FR-10; §5.4 of the basis (D-6)
- **S-5**: the check registry — **extends** (the DCR pattern, a new registry instance) — behind it: per-file init() self-registration of checks declaring rung, verb dependencies, author-independence class; ladder composition derived — feeds: Serves NFR-6; AP-27 (D-7)

## Facts

- **F-1**: implement-now's live design decomposes the Worker into five consumer-side ports — WorkTracker, Lease, Workspace, AgentRuntime, CodeHost — with no verification seam; drive-to-green feeds redacted+clamped failed-CI logs to a fixer — source: docs/tech-design/implement-now.md §2, §3.7
- **F-2**: implement-now DINV-4 binds the Worker's local gate to "the same deterministic checks required by CI" (AP-11, single-source predicates); its PRD is merged with FR-1..FR-20 frozen — source: implement-now.md DINV-4 · tech-design index
- **F-3**: tool binaries ship as sha256-pinned static per-version releases into a gitignored cache; operand repos pin, this repo runs source; committed binaries are rejected — source: repo-layout-and-system-footprint.md
- **F-4**: the executor's caps live as closed-schema policy blocks on the pyxis-config surface (per-repo override; unknown keys error) — the home precedent for the verification manifest — source: design-brainstorm:implement-now-tech-design#D-6
- **F-5**: the derived-registration precedent is live in-repo: docgate gate checks self-register per-file via init() into a registry, no shared enumeration (AP-27) — source: docs/tech-design/distributed-check-registration.md · internal/docgate/gate/
- **F-6**: prepush-ref-awareness (live, prd:docgate/FR-29) owns pre-push ref-range semantics — the parity ground the remote phase (WR-5) will reuse; issue #357 documents the working-tree-scope bug it fixes — source: tech-design index · agenticlab-ai/pyxis#357

## Fog

- **G-1** (non-blocking — blocks: integration-time execution, owned by implement-now's ledger): whether implement-now's side of the seam lands as a typed PRD amendment, a superseding design, or both — resolved in kind by D-2 (both: additive amendment + small supersession); the residual fog is only its timing and exact wording, executed at integration

## Out of scope

- **O-1**: sub-rung (per-test/per-package) incremental caching — a future supersession with its own state design; v1's granularity is rung-level, exactly FR-3's bar (from D-4)
- **O-2**: an in-process local execution path for the Worker — out until M3 evidence shows subprocess spawn cost material; reachable by supersession via the importable core (from D-2)

## Candidate glossary terms

- **T-1**: **VerificationGate port** — the thin consumer-side port on implement-now's Worker whose adapter execs the pinned harness binary · avoid: *local gate* (implement-now's CI-mirror predicate, DINV-4 — a different, coexisting thing), *harness port* (the port lives on the host's side of the seam)
- **T-2**: **carried pass** — a rung result copied into a new verdict from the prior one because its inputs are provably unchanged, marked `carried-from <run id>` · avoid: *cached* (implies a separate cache artifact; the prior verdict is the only state)
- **T-3**: **codegate** — the verification harness's tool name (`cmd/codegate`); the capability keeps the PRD's name, the binary carries this one — docgate's sibling · avoid: using *codegate* for the capability (the PRD/glossary say "the verification harness"), *harness* as the binary name (too generic on a PATH)

## Parked product gaps

- **P-1**: the basis's NR-2 fit criterion presumes implement-now's PRD is a draft ("integration lands as a draft-phase edit, not a post-merge amendment") — that PRD merged 2026-07-16 with FR-1..FR-20 frozen, and its design's DINV-4 defines the Worker's local gate as a CI mirror; integration now requires a typed amendment on prd:implement-now (and/or a superseding design there) — anchors: prd:implementation-verification/NR-2 · prd:implement-now/FR-8, FR-18 — disposition: **proceed-anyway per D-2** — additive amendment on prd:implement-now + small superseding design there + corrective amendment on this basis's NR-2, all queued at integration

## Gate

| Field | Value |
|---|---|
| **Verdict** | ready-for-design |
| **Date** | 2026-07-16 |
| **Authority** | @mdarabi |
| **Covering design** | — |
| **Coverage check** | pass — core four covered, zero blocking fog |
| **Amend route** | — |
