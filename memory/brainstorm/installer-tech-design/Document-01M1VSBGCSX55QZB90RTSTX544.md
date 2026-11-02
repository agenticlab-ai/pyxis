# installer — design-brainstorm record

| Field | Value |
|---|---|
| **In one line** | The solution shape for the installer PRD: how Pyxis becomes a versioned release (unit, hosting, manifest derivation), what binary carries init/converge/doctor, how installed repos acquire and execute engine tools without source, and how wiring/seeding/epoch mechanics bind — on top of the live repo-layout substrate. |
| **Relevant-when** | designing or changing the release unit/versioning, the `pyxis` binary, converge/manifest mechanics, installed-repo tool acquisition, install-time GitHub wiring, seeding, or the install-epoch surface; checking prior design rulings or refused shapes about distribution and installation |
| **Basis** | prd:installer (merged ledger, as of 000, fingerprint sha256:970ca43f…) |
| **Authority** | @mdarabi (operator) |
| **Date(s)** | 2026-07-19 |
| **Status** | gated: ready-for-design |
| **Tracking** | — |
| **Feeds** | docs/tech-design/installer-v1.md |
| **Prior records** | brainstorm:installer |
| **Template version** | design-brainstorm-record/v1 |

## Orientation

| Source | Why loaded |
|---|---|
| `docs/prds/installer/effective.md` | the admitted basis — 22 FRs/10 UBs incl. converge/ownership classes (FR-5), pinned no-source resolution (FR-17/FR-21/FR-22), offline floor (FR-13), five knobs (FR-3), epoch-adjacent WR-1/WR-10, Q-1..Q-4 design-owned questions |
| `docs/tech-design/index.md` | Serves routing — surfaced the overlap set below |
| `docs/tech-design/repo-layout-and-system-footprint.md` (live) | **the substrate design**: three buckets, `.pyxis/` consolidation, DINV-2 no-unpinned-execution, the config schema (per-tool pins, paths), §3.4's manifest.json sketch, §3.7's docgate release channel; §3.8 phase 4 defers `pyxis init` to *this* design; its Q1 (hosting) and Q2 (operand AGENTS.md) are open and inherited here |
| `docs/tech-design/docgate-v1.md` §3.10 + `codegate-v1.md` | acquisition precedents: one-binary-umbrella-verb shape; codegate ships "exactly like docgate" and pre-plans "the future single engine binary absorbs internal/verify as a subcommand without reshaping" (#ALT-1); the verification manifest binds to "the `pyxis-config/` **surface** (the executor-caps precedent)" |
| `docs/prds/implementation-verification/effective.md` FR-5 | Q-1 input: the FR row itself names **no directory** — only closed schema + ecosystem defaults; the directory phrasing lives in glossary/design prose |
| `docs/tech-design/prepush-ref-awareness.md` (live) | the hook seam is already Go-owned (`pre-push` verb, delegate-only shims) — installer wiring extends it, never re-shapes it |
| `docs/architecture-principles.md` + `docs/observability/incidents.md` | the scoring base: AP-9 (staged releases), AP-27 (aggregates derived, never authored), AP-24, AP-22, AP-17, AP-13/AP-26, RW-4 ("the factory-in-a-box is the first shippable artifact"); incidents I1–I3 |
| `docs/tech-design/local-work-tracker-v2.md` (header) + `.pyxis/config.yaml` | carina is one of the nine cmd binaries — acquisition uniform with the rest; config reality: per-tool pin sections (docgate, codegate) both `source`-mode here |
| `docs/brainstorms/installer.md` (frozen, prior record) | stage-1 rulings D-1..D-11 — carried, never re-asked |

**Overlaps surfaced:**
- `repo-layout-and-system-footprint` — **not covered; the explicit predecessor**: it built the substrate (buckets, pins, config, channel) and named the installer its future-design phase 4. This design builds on it; nothing here re-litigates the buckets or DINV-2.
- `codegate-v1` #ALT-1 — the single-engine-binary future is *anticipated* by a frozen sibling; DQ-2 must stay compatible with "absorbs without reshaping".
- `prd:implementation-verification` FR-5 / glossary "verification manifest" — the Q-1 surface; the FR row is directory-silent → the materialization is definable here (DQ-7), with a glossary-term touch, no contract-changing amendment.
- `prepush-ref-awareness` — hooks are Go-verb-owned already; the installer only activates them (basis FR-9) and inherits the base-ref posture (basis WR-9).

## Design questions

- **DQ-1**: What is the release unit and versioning scheme — one system release (a single tag pinning all engine binaries + the payload + the release manifest under one system version) or per-binary tags (today's `docgate/v*` pattern, ×9) — and where are artifacts hosted (repo-layout Q1)?
- **DQ-2**: Which binary carries init/converge/doctor — a new thin `pyxis` binary alongside the nine, the start of engine consolidation (tools absorbed as subcommands now), or an installer-only tool — under codegate-v1's absorbs-without-reshaping anticipation?
- **DQ-3**: How are the payload and the release manifest produced and shipped — tarball composition (including the offline dependency bundle F-7 forces), and the manifest generated at release build from authored ownership-class policy (AP-27/AP-24 split)?
- **DQ-4**: How do installed repos acquire and execute engine tools (the docgate-resolve.sh pattern's successor): pyxis-owned acquisition-and-exec, per-tool resolvers, or converge-time installation into the cache with thin exec shims?
- **DQ-5**: What mechanism performs GitHub wiring (basis Q-3) — a Go stdlib REST client with ambient-token discovery, or a `gh` CLI subprocess dependency — and how is the declared-operation audit (EVT-4) produced?
- **DQ-6**: What does the install-epoch surface record in v1 (basis Q-4) — and what deliberately consumes it now versus at WR-1?
- **DQ-7**: How is "the pyxis-config surface" materialized in v1 (basis Q-1) — the definitional binding that closes the FR-5 home question without touching that PRD's contract?
- **DQ-8**: Is the operand's AGENTS.md hand-authored or installer-generated (repo-layout Q2) — and which ownership class does it land in?
- **DQ-9**: Where do the seeded skeletons come from — new shipped templates for the governing/system docs, registries created empty-valid, the welcome item's content?

## Panel log

- **DQ-1**: replaces (the per-binary release seam becomes the system-release seam) — fired — a new platform contract every installed repo trusts; orthogonal shapes exist
- **DQ-2**: opens (the engine-binary seam the codenamed single-engine future needs) — fired — placement decides reshaping cost for every later tool
- **DQ-3**: extends (DQ-1's release seam; mechanism follows AP-27/AP-24) — skipped — composition and derivation are principle-forced once DQ-1 picks the unit
- **DQ-4**: replaces (the resolver seam: one bash shim per tool → the installed-repo acquisition contract) — fired — the pattern multiplies ×9 if wrong
- **DQ-5**: extends (the code-host adapter class already speaks REST/gh; a mechanism choice inside it) — skipped — mechanism choice inside an existing adapter class; ruled in D-5
- **DQ-6**: extends (the closed config schema gains keys; consumers deferred by the basis's WR-1/WR-10) — skipped — v1 is write-only capture
- **DQ-7**: extends (AP-8's sanctioned surface, already precedented by executor-caps/codegate) — skipped — a definitional binding, not a new seam
- **DQ-8**: extends (the templates seam gains one instantiable) — skipped — ownership-class assignment is the only real call
- **DQ-9**: extends (the templates seam; registries already have empty-valid shapes) — skipped — mechanism follows the basis's FR-7/NG-1

## Decisions

### D-1 — DQ-1: the atomic train — one system release

- **Question**: the release unit and versioning scheme, and artifact hosting (repo-layout Q1)?
- **Answer**: **ALT-1** — one system tag (`vX.Y.Z`) builds all engine binaries × platforms from one commit, packs the payload (system tree + review runtime + the offline deps bundle) and emits the release manifest + checksums; operand config pins `system.version` + manifest sha256 (per-binary hashes live in the manifest); the source repo takes one repo-wide `source` mode; per-tool pin sections retire via a dual-read closed-schema window; `docgate/v*` artifacts stay hosted, the workflow is superseded. Hosting: GitHub Releases on the pyxis repo; the FR-22 mirror story is a base-URL override with the config-pinned manifest hash as the trust root. Two invariants ride the pick: the built-binary list is derived by `cmd/` scan (AP-27), and publish is re-runnable upload-or-verify (AP-22).
- **Reasoning**: one module + one commit means cross-tool skew cannot exist — ALT-2's tuple certification rebuilds incident I2 at the distribution layer, and ALT-3's promote/dist-repo machinery is platform-loop substrate the appetite excludes, kept reachable later by the override + trust-root shape. Operator confirmed the recommendation.
- **Anchor**: 2026-07-19 · t1
- **Status**: live

### D-2 — DQ-2: the front-door orchestrator behind the one-command face

- **Question**: which binary carries init/converge/doctor, and what stands between skills/hooks and the nine tools in installed repos?
- **Answer**: **ALT-6** — `cmd/pyxis` owns init/converge/repair/doctor and the single Go acquisition path: `pyxis exec <tool> -- <argv>` resolves per-binary hashes from the D-1 release manifest, checksum-verifies, and execs argv-verbatim; the shell resolvers collapse into one tested `internal/resolve`; the nine tools keep their binaries and blast radii; source mode (this repo only) falls through to `go run`. UB-9 posture: mandatory `--`, flag-like tokens before it refuse pre-action.
- **Reasoning**: the only shape putting the §3.3-critical checksum-on-every-path floor into one tested Go implementation (F-9's live shell drift dies); pre-pays codegate-v1's absorb-later (a future in-process dispatch swap behind a stable grammar, zero line churn); both panels converged on it blind (F-10). ALT-4 leaves the floor in shipped shell; ALT-5 exceeds the appetite and smuggles the nine-tool topology into this design. Operator confirmed the recommendation.
- **Anchor**: 2026-07-19 · t2
- **Status**: live

### D-3 — DQ-4: lazy per-exec-verified acquisition with converge pre-warm

- **Question**: the acquisition mechanics under D-2's orchestrator — when are binaries fetched and when verified?
- **Answer**: **ALT-7** — every `pyxis exec` re-hashes the cached binary before running it (UB-3's per-exec floor; §3.3's "on any path" literal); cache miss → fetch-verify with tmp+atomic-rename, FR-22-override-aware; `pyxis converge` pre-warms every tool (the offline floor's mechanism, FR-13); a fresh clone self-heals on first invocation, an offline cold clone refuses by name pointing at converge. The bootstrap shim remains the one bash-with-logic file — it fetch-verifies pyxis itself (I1: a gate cannot gate its own arrival), minimal and drilled (ABUSE-4).
- **Reasoning**: ALT-8 trades away per-exec verification (tamper runs until doctor — AP-13/AP-17 strained against the critical guardrail as written) and dead-ends fresh clones; ALT-9's tenth artifact + second front door fights the frozen one-command face for isolation the per-repo pin already provides. Steady-state cost (~tens of ms/invocation) stated per AP-19. Operator confirmed the recommendation.
- **Anchor**: 2026-07-19 · t3
- **Status**: live

### D-4 — DQ-3: payload + deps bundle; manifest derived from authored class policy

- **Question**: payload composition, the offline dependency story, and how the release manifest is produced?
- **Answer**: the release ships one payload archive (the system tree) plus a separate platform-neutral dependency bundle (the npm cache for the pinned lockfile — `npm ci --offline` at init makes FR-13's offline init real). The release manifest is **generated at release build** by walking the payload against an **authored ownership-class policy file** committed in the source repo; a build-time conformance check fails the release if any payload file lacks a class. Operand config keys stay per D-1: `system.version` + manifest sha256.
- **Reasoning**: AP-24 (the class policy is the human-authored input) / AP-27 (the manifest aggregate is derived, never authored) split cleanly; a separate deps bundle keeps online installs light while making offline first-class. Operator confirmed the recommendation.
- **Anchor**: 2026-07-19 · t4
- **Status**: live

### D-5 — DQ-5: GitHub wiring rides the gh CLI

- **Question**: the wiring mechanism (basis Q-3) — Go stdlib REST vs a `gh` CLI subprocess?
- **Answer**: **gh CLI** — `internal/wire` shells out to `gh` for the declared operation set (labels, workflow push, protection, welcome item, repo-create). gh becomes a **GitHub-mode-only prerequisite** (UB-7 class: absent gh → named remediation; the offline/carina floor is untouched). The declared-operation table remains the single source driving FR-19's preview and EVT-4's audit — gh is transport, the table is the contract (AP-11); UB-4's permission hand-off and the transient-retry-then-hand-off live around the gh calls.
- **Reasoning**: operator's call, displacing the REST recommendation — consistency with the fleet's existing gh-speaking adapters and gh's battle-tested auth path outweigh the added mode-scoped prerequisite; no HTTP client to maintain.
- **Anchor**: 2026-07-19 · t5
- **Status**: live

### D-6 — DQ-6: write-only install epoch

- **Question**: what does the install-epoch surface record in v1 (basis Q-4), and who consumes it?
- **Answer**: at init, converge writes an `epoch` block into the repo config — installed system version, the epoch commit (the initial commit init creates; later, WR-1's adoption commit), and the manifest sha converged to. v1 consumers: doctor reports it; nothing else reads it. The history-scoped check semantics ("enforce from install forward", the generalized grandfather baseline) are WR-1's design.
- **Reasoning**: the anchor must exist on day one for every v1-installed repo or WR-1 adoption loses its mechanical floor; write-only keeps this design inside the appetite. Operator confirmed the recommendation.
- **Anchor**: 2026-07-19 · t6
- **Status**: live

### D-7 — DQ-7: "the pyxis-config surface" bound as a concept, materialized under the system tree

- **Question**: how is "the pyxis-config surface" materialized in v1 — the Q-1 home question?
- **Answer**: the surface is **defined**, not relocated: "the pyxis-config surface" := AP-8's sanctioned, closed-schema, per-repo configuration surface, materialized in v1 as named files under the system tree (the repo config + named policy files: the verification manifest, the ownership-class policy). No amendment to implementation-verification (its FR-5 row is directory-silent — F-3); the glossary parenthetical updates in this design's introducing PR; codegate-v1 stands unsuperseded (its "surface" wording remains true); the installer PRD's Q-1 closes with this ruling. If the vision §8 directory ever materializes, the surface relocates by converge migration — a later feature.
- **Reasoning**: the cheapest binding that makes every existing contract true simultaneously; a directory was never the contract, the closed schema was. Operator confirmed the recommendation.
- **Anchor**: 2026-07-19 · t7
- **Status**: live

### D-8 — DQ-8: operand AGENTS.md — template-instantiated once, then repo-owned

- **Question**: is the operand's AGENTS.md hand-authored or installer-generated (repo-layout Q2), and in which ownership class does it land?
- **Answer**: **installer-generated once** from a new shipped instantiable (the factory's operating rules for agents in a target repo: doc-system rules, gate expectations, language-policy pointer; knob substitutions like tracker choice and argus state), then **repo-owned** — the human extends it freely, converge never rewrites it, doctor advises when the shipped template's version has moved. The root stays pointer-thin per repo-layout DINV-4; CLI-surface pointers materialize per the knob.
- **Reasoning**: day-one agents need rules (hand-authored leaves them ruleless), and permanent engine ownership would forbid the operand's own agent policy (fights AP-8); instantiate-then-own is the only class assignment satisfying both. Closes repo-layout's Q2. Operator confirmed the recommendation.
- **Anchor**: 2026-07-19 · t8
- **Status**: live

### D-9 — DQ-9: shipped-skeleton seeding

- **Question**: where do the seeded artifacts come from?
- **Answer**: the four governing/system doc skeletons become **new shipped instantiables** in the payload template set (structure + fill-me guidance, zero invented content — basis NG-1); init instantiates them as repo-owned files. Registries are created empty-but-valid by init code — generated class where a tool is the sole writer (the tech-design index), repo-owned where the human appends (incidents). The welcome work item's body is a shipped template substituted with knob answers and authorship pointers. No new document classes are invented.
- **Reasoning**: guidance-bearing skeletons are what make a stranger's authorship path real (G4); class assignments follow who writes next. Operator confirmed the recommendation.
- **Anchor**: 2026-07-19 · t9
- **Status**: live

## Alternatives

### ALT-1 — The atomic train (one system tag, build-at-release)

- **Shape**: one tag (`vX.Y.Z`) on the pyxis repo builds all nine binaries × four platforms from that commit, packs the payload archive (system tree + vendored review runtime + offline deps bundle) and emits a schema-versioned release manifest (system version; per-file hash + ownership class; per-binary per-platform sha256) + SHA256SUMS. Operand config pins two keys: `system.version` + manifest sha256; the source repo takes one repo-wide `source` mode. Per-tool config sections retire via a closed-parse schema flip (one dual-read release with doctor warning, then refusal); `docgate/v*` artifacts stay hosted, the workflow is superseded. Hosting: GitHub Releases on the pyxis repo; FR-22 = a base-URL override, safe under any mirror because the config-pinned manifest hash is the trust root.
- **Trade-offs**: wins — e2e certifies exactly what ships (no combination space; M1/M2 map 1:1 to a tag); upgrade = one pin bump; cross-tool skew impossible by construction (one commit, one go.mod); config compacts ~40 hash lines → 2 keys. Costs — a hotfix rebuilds the world (full RC matrix per patch; unchanged tools get new, not byte-identical, binaries); per-tool semver signal dies; migration needs the dual-read window run RW-6-style with the source repo green throughout.
- **Principles**: AP-13/AP-26 honored (the PASS certifies the assembled release, never a sum of per-tool badges); AP-6 honored (manifest = the contract; compact pin); AP-11 honored (one resolver predicate ×9); AP-17 honored (manifest-hash root fails closed); AP-22 conditional (publish must be upload-or-verify re-runnable); AP-27 conditional (build list stays `cmd/`-scan-derived, never hand-kept); AP-9 strained (channels key on one coarse stream — not precluded); RW-4/RW-6 honored.
- **Incidents**: I2 — the per-object-statuses-summing-false class is eliminated structurally (no per-tool statuses exist); I1 — the first system-release pipeline cannot be certified by a prior system release; operator review + the proven docgate/v0.1.0 machinery backstop release #1.
- **Verdict**: chosen — D-1; the scan-derived build list and re-runnable publish ride as design invariants
- **From**: DQ-1 · panel

### ALT-2 — Ten channels (per-tool tags + payload tag + certification manifest)

- **Shape**: `<tool>/v*` tags ×9 + `payload/v*`, one tag-prefix-derived workflow; operand config pins nine `{version, sha256×4}` blocks + payload; FR-1's manifest becomes a published *certification record* of one tested tuple; the bootstrap resolves the latest certified tuple then fetches per-channel. Hosting: pyxis repo.
- **Trade-offs**: wins — surgical hotfix (one binary rebuilds, eight stay byte-identical); per-tool semver lives; zero migration. Costs — the combination space is real: every bump creates an uncertified tuple (full matrix per bump, or operands drift uncertified); cross-tool schema skew becomes a live bug class demanding versioned interchange contracts; ~40-line pin matrix; converge must survive mid-skew windows.
- **Principles**: AP-27 honored (tool #10 = pure addition); AP-11 strained (ten pin-verify twins unless one parameterized implementation); AP-13/AP-26 strained hard (a tool's green certifies that tool only — the tuple record is a mandatory countermeasure); AP-22 stretched (converge mid-skew); RW-4 strained (skew contracts = substrate easiest to skip under hotfix pressure).
- **Incidents**: I2 matched adversely — ten true per-channel statuses summing to a false system belief IS the I2 class at the distribution layer unless tuple-pinning is mandatory; I3 glancing (ten-pin resolution multiplies misparse surface → UB-9 must cover resolution verbs).
- **Verdict**: rejected — D-1: rebuilds the I2 class at the distribution layer to buy independence a single-module build cannot need
- **From**: DQ-1 · panel

### ALT-3 — Build-once, promote (tool builds + a dist-repo assembly release)

- **Shape**: two tiers — engineering tags `<tool>/v*` on the pyxis repo (build events), and a system release as an *assembly*: a Go promote tool selects built artifacts, re-hashes them, runs the full matrix against exactly that tuple, then publishes the frozen combination (manifest-of-versions + artifacts) to a dedicated `pyxis-dist` repo with `channels/{stable,beta}.json` as committed data. Operand pins `system.version` + manifest hash.
- **Trade-offs**: wins — hotfix minimal AND certified (eight binaries byte-identical to previously certified); channels-as-data is the cleanest AP-9 runway; mirroring = fork the dist repo. Costs — the most machinery (promote tool, second repo, two green surfaces as standing obligations the basis's guardrails never name); indirect provenance (dist → manifest → tag → commit); promote is new supply-chain surface that must re-hash everything or it launders stale artifacts.
- **Principles**: AP-26 honored most literally (promote re-hashes and re-tests the actual artifacts); AP-13/AP-24/AP-6/AP-17 honored; AP-9 honored strongest (channels/cohorts as reviewable data); RW-4 strained hardest — the largest substrate bill, paid before the installer exists; AP-11 strained (resolution spans two repos + the promote tool).
- **Incidents**: I2 matched as the central countermeasure (the promote gate exists precisely against badge-summing); I1 — the first promote has no prior certified system; operator review backstops release #1.
- **Verdict**: rejected for v1 — D-1: platform-loop substrate beyond the appetite; the override + manifest-hash trust root keeps this evolution open for the WR-8 era
- **From**: DQ-1 · panel

### ALT-4 — Tenth thin binary (peer tool; consolidation stays future)

- **Shape**: `cmd/pyxis` = a tenth thin main (init/converge/repair/doctor/version, I3 guard) over new packages `internal/converge` (manifest/ownership/drift core; doctor = its read-only mode), `internal/installer` (knobs, seeding, capture), `internal/wire` (GitHub ops + EVT-4), `internal/relmanifest`; the nine tools, shell resolvers, hooks, and skill lines untouched.
- **Trade-offs**: wins — smallest blast radius now (purely additive; NR-1 trivially green; inside the appetite); absorb-later compatible (cmd/pyxis is the natural umbrella host). Costs — the **critical checksum floor stays split**: Go for bootstrap/converge but the shipped shell resolvers keep runtime resolution ×N (FR-22's override lands in awk against NFR-5); a third copy of the misparse guard unless extracted.
- **Principles**: AP-22, AP-20, AP-27, RW-4, RW-5, AP-19 honored; **AP-11 strained** (resolution predicate stays N shell copies); **AP-10 strained** (resolution logic remains shell).
- **Incidents**: I3 matched — adds a tenth stdlib-flag main to I3's class (guard must ship shared, not a fourth copy); leaves the resolvers' "must parse the same way" comment hazard standing.
- **Verdict**: rejected — D-2: leaves the critical checksum floor split across shipped shell copies (AP-10/AP-11 strained; F-9's drift class survives)
- **From**: DQ-2 · panel

### ALT-5 — Single engine binary now (consolidation in this feature)

- **Shape**: `pyxis` becomes THE engine binary today — `pyxis docgate|plangate|codegate|carina|… <verb>` via self-registered/build-generated dispatch; nine mains kept as deprecated shims; installed repos receive only `pyxis`; per-tool pin sections collapse into `system:` by superseding repo-layout §3.6/§3.7 + codegate-v1's pin binding.
- **Trade-offs**: wins — FR-17/FR-22/UB-3 reduce to one artifact and one Go path; UB-9 genuinely one predicate; the offline floor trivial; codegate-v1's anticipation fulfilled early. Costs — blast radius = the entire invocation surface of this repo (hooks, resolvers, workflows, SKILL/AGENTS lines) + supersession of two frozen designs, none of it needed for M1/M2 — exactly what the appetite parks; codegate-v1's own verbs-on-docgate rejection rationale (any tool's bug forces a shared release) now applies ×9 by construction.
- **Principles**: AP-11/AP-10 honored strongly; **AP-20 strained** (widest release blast radius); **AP-27/AP-23 strained** (the verb dispatch is a shared append surface unless generation is mandated); **RW-5 strained** (a second architecture decision smuggled into this design); **RW-4 strained** (reshapes proven substrate mid-program).
- **Incidents**: I3 matched (one parser, cleanest win); **I2 matched** — the invocation-line migration is a cross-surface stacked train, I2's exact failure shape.
- **Verdict**: rejected — D-2: outside the appetite; a second architecture decision (nine-tool CLI topology) smuggled into this design, plus two frozen-design supersessions and an I2-shaped migration
- **From**: DQ-2 · panel

### ALT-6 — Front-door orchestrator (`pyxis exec` acquires and execs the tools)

- **Shape**: `cmd/pyxis` owns init/converge/repair/doctor AND the single Go acquisition path: `pyxis exec <tool> -- <argv…>` resolves the pin (under D-1: per-binary hashes from the config-pinned release manifest), cache → fetch-verify → refuse (FR-22-aware), then execs argv-verbatim; tools keep their binaries; the shell resolvers collapse into one tested `internal/resolve`; payload hooks/workflows/skills invoke everything through the exec grammar; `source` mode (this repo only) falls through to `go run`.
- **Trade-offs**: wins — the critical checksum-before-exec floor becomes ONE tested Go implementation (the live unpinned-sibling-resolver gap and the #485 "must parse the same way" drift class die); NFR-5 true of the shipped surface; absorb-later pre-paid (absorption = in-process dispatch behind an already-stable grammar, zero line churn); per-tool release blast radii preserved. Costs — payload invocation-grammar churn now + one extra execve per invocation (stated, bounded); the grammar becomes the one hot line every surface depends on.
- **Principles**: AP-11 honored (one resolution predicate, finally); AP-10 honored (thrice-copied awk → tested Go); AP-22, AP-20, RW-1, RW-5, AP-13, AP-17 honored; **AP-19 mildly strained** (a stated per-invocation exec hop); **AP-23 mildly strained** (grammar stability is the point and the risk).
- **Incidents**: I3 matched twice — pyxis's own verbs carry the fail-closed guard, and the pass-through boundary is designed against I3's exact mode: mandatory `--`, flag-like tokens refused before it, argv-verbatim after.
- **Verdict**: chosen — D-2; the exec grammar's stability becomes a named design obligation (AP-23)
- **From**: DQ-2 · panel

### ALT-7 — One owner at runtime: lazy `pyxis exec` with converge pre-warm

- **Shape**: DQ-4's mechanics for ALT-6's shape: per-exec re-hash of the cached binary (UB-3's strict floor), fetch-verify (tmp+rename, override-aware) on miss, converge pre-warms all tools (FR-13), lazy self-heal on fresh clones; the bootstrap shim remains the one bash-with-logic file, fetch-verifying pyxis itself (I1: a gate cannot gate its own arrival — minimized and drilled, ABUSE-4).
- **Trade-offs**: wins — fresh clone self-heals on first invocation (offline cold clone = named refusal pointing at converge); CI in installed repos runs the identical path as the desk; pin bumps take effect atomically next invocation; NR-2 parity structural (same Go decision code, only the mode differs). Costs — ~tens of ms per invocation (config parse + sha256 + execve), a forever-held budget; the product binary is the hot-path single point of failure (contained: a bad release arrives only by deliberate pin bump; rollback = repin).
- **Principles**: AP-11, AP-10, AP-13, AP-17, AP-14, AP-4, AP-25, RW-3 honored; AP-19 strained-and-stated.
- **Incidents**: I3 (argv boundary as ALT-6); I1 (the minimized ungated first-mover).
- **Verdict**: chosen — D-3 (folds ALT-8's converge pre-warm in as the offline-floor mechanism)
- **From**: DQ-4 · panel

### ALT-8 — Converge-time materialization: cache once, exec direct

- **Shape**: acquisition only in init/converge — all binaries fetch-verified into the cache; shims regenerated with the exact versioned filename baked in (generated ownership class); runtime = direct exec, zero resolution; doctor names drift; source-mode converge `go build`s the same layout.
- **Trade-offs**: wins — fastest steady state (sub-ms), cleanest post-converge offline floor, pin-bump staleness fails loud on the hook path (missing-file refusal, never a silent wrong version). Costs — **the per-exec verification gap**: a binary tampered after converge executes unverified until doctor — against §3.3's "on any path" critical guardrail and UB-3/FR-17 as written; fresh clones fail-by-name until converge runs (a stranger papercut, G4); CI needs a converge step per job; source devs lose `go run` freshness (a new stale-build class); the skill path needs a stable name → a silent old-version window between bump and re-converge.
- **Principles**: AP-19 honored best; AP-10/AP-22/AP-4/RW-3 honored; **AP-13 strained** (verification historical, not asserted at the trusting run); **AP-17 strained** (tamper window fails open until doctor); **AP-11 strained** (the hash-matches-pin predicate runs at a different time than the execution it certifies).
- **Incidents**: I3 inversely (argument-free generated shims; loud missing-file refusals) but the stable-name skill path re-admits a silent-stale variant; I1 unchanged.
- **Verdict**: rejected — D-3: write-time-only verification opens a tamper window the §3.3 guardrail forbids as written; fresh clones dead-end until converge
- **From**: DQ-4 · panel

### ALT-9 — `pyxis-resolve`: a standalone micro-resolver binary

- **Shape**: a dedicated stdlib-only ~2 MB `pyxis-resolve <tool> -- <args>` binary (own slow cadence) is what every line calls; the product `pyxis` is itself a resolved client (no self-resolution paradox); the same `internal/resolve` library compiles into pyxis for converge/doctor/preview; one bootstrap shim fetch-verifies pyxis-resolve alone.
- **Trade-offs**: wins — failure isolation: the gate hot path depends on the smallest, slowest-moving trusted base; the ungated first-mover artifact is minimal and auditable (the strongest I1 answer). Costs — a tenth released artifact and a **second front door** (guides invoke a resolver, not the product — weaker fit with the frozen one-command face); resolver↔product version skew is a new class needing a doctor check; the bash count is not reduced vs ALT-7.
- **Principles**: AP-11 honored at the library level, mildly strained by two frontends; AP-20 honored (isolation); AP-10/AP-13/AP-17/AP-4/AP-19/RW-3 as ALT-7.
- **Incidents**: I3 (same boundary rule); I1 (best-in-class answer).
- **Verdict**: rejected — D-3: a tenth artifact and a second front door against the frozen one-command face; isolation already provided by the per-repo pin
- **From**: DQ-4 · panel

## Seam map

- **S-1**: the system-release seam (tag → artifact set → release manifest) — replaces (the per-binary channel) — behind it: the release workflow, the manifest generator, checksums, the deps bundle — feeds: Scope "release machinery"; Serves prd:installer/FR-1, FR-2, FR-17, FR-22
- **S-2**: the orchestration seam (`pyxis exec <tool> -- <argv>`) — opens (the installed-repo invocation grammar every skill/hook/CI line speaks; the future engine-consolidation host) — behind it: `internal/resolve` (pin→manifest→cache→fetch-verify→exec), the source-mode fall-through — feeds: Scope "converge core + acquisition"; Serves prd:installer/FR-11, FR-17, FR-21, FR-22, NR-2
- **S-3**: the acquisition seam (pin → manifest → cache → per-exec verify → exec; bootstrap = the one shell first-mover) — replaces (the per-tool bash resolvers and their drift class) — behind it: `internal/resolve`, the gitignored binary cache, the bootstrap shim — feeds: Scope "acquisition"; Serves prd:installer/FR-2, FR-13, FR-17, FR-21, FR-22, UB-3
- **S-4**: the per-repo config/policy surface (the pyxis-config surface, bound by D-7) — extends (AP-8's sanctioned surface: repo config + named policy files under the system tree; epoch block per D-6) — behind it: the closed schemas, the ownership-class policy, the verification manifest's home — feeds: Scope "config capture"; Serves prd:installer/FR-3, FR-18, Q-1
- **S-5**: the seeding/templates seam — extends (the payload template set gains the four governing-doc skeletons, the operand AGENTS.md instantiable, the welcome template) — behind it: init's instantiation + empty-valid registry creation — feeds: Scope "seeding"; Serves prd:installer/FR-7, FR-15, NG-1

## Facts

- **F-1**: repo-layout-and-system-footprint (live) already decides: three buckets with the removal test, `.pyxis/` consolidation, DINV-2 (no unpinned execution; source mode legal only where source lives), the closed config schema with per-tool pin sections, a manifest.json sketch (system version + per-file hashes), and the docgate release channel (static binaries + checksums, fetch-verify-cache). Its §3.8 phase 4 names `pyxis init` a future design — this one. — source: docs/tech-design/repo-layout-and-system-footprint.md
- **F-2**: codegate-v1 (live) ships codegate "exactly like docgate" and pre-plans "the future single engine binary absorbs `internal/verify` as a subcommand without reshaping" (its ALT-1); it binds the verification manifest to "the `pyxis-config/` surface (the executor-caps precedent)". — source: docs/tech-design/codegate-v1.md §2, §3.10
- **F-3**: prd:implementation-verification FR-5's normative row requires a closed-schema repo manifest + ecosystem defaults and names **no directory**; the `pyxis-config/` wording lives in the glossary term and design prose only. — source: docs/prds/implementation-verification/effective.md FR-5; docs/wiki/glossary.md
- **F-4**: `.pyxis/config.yaml` on the merged tip carries per-tool pin sections (docgate, codegate — both `source` here), the paths block, and plan thresholds; repo-layout §3.6's schema adds `system.version` and per-platform sha256 pins for release mode. — source: .pyxis/config.yaml; repo-layout §3.6
- **F-5**: nine binaries under `cmd/` on the merged tip; docgate alone has release machinery (`docgate-release.yml`, tag `docgate/v*`, darwin/linux × arm64/amd64 + SHA256SUMS). — source: cmd/ listing; .github/workflows/docgate-release.yml
- **F-6**: the pyxis incident registry holds I1 (Serves discipline: a design must serve ≥1 committed FR), I2/AP-26 (a merged badge certifies the base, verify by mainline ancestry), I3 (consequential CLIs fail closed on misparse — already carried as basis UB-9). — source: docs/observability/incidents.md
- **F-7**: the basis forces an offline dependency bundle: FR-13 requires init itself to complete zero-network post-fetch in the offline combo while FR-12 installs the review-surface deps — so the release artifact set must carry the dependency payload (an offline-installable archive), not rely on the npm registry at init time. — source: prd:installer FR-12/FR-13
- **F-8**: the hook seam is already Go-owned: prepush-ref-awareness moved ref handling into a `pre-push` verb behind delegate-only shims; the installer activates hooks (basis FR-9) without reshaping them. — source: docs/tech-design/prepush-ref-awareness.md
- **F-9**: a live DINV-2 gap exists in-tree today: `argus-stamp-resolve.sh` and `feedback-capture-resolve.sh` run unpinned PATH binaries with zero sha verification, and `docgate-resolve.sh`'s #485 parse fix legislates by comment that "a future codegate-resolve.sh must parse the same way" — the bash-clone drift class is not hypothetical. — source: .pyxis/scripts/ (DQ-4 panel read, 2026-07-19)
- **F-10**: the DQ-2 and DQ-4 panels, run blind to each other under different constraint framings, independently converged on the same shape — `pyxis exec <tool> -- <argv>` as the single acquisition-and-execution owner. — source: panel outputs, this session

## Fog

none

## Out of scope

- **O-1**: Engine consolidation (tools as in-process `pyxis` subcommands) — deferred; D-2's exec grammar makes the future swap zero-churn, and codegate-v1's absorbs-without-reshaping anticipation stands. (from D-2)
- **O-2**: Release channels/cohorts, a dist repo, promote machinery (ALT-3's territory) — platform-loop era; D-1's override + manifest-hash trust root keeps the path open. (from D-1)
- **O-3**: Write-time-only binary verification (ALT-8's direct-exec) — rejected; the per-exec re-hash floor is binding. (from D-3)
- **O-4**: A literal `pyxis-config/` directory — if the vision §8 surface ever materializes as a directory, relocation is a later feature's converge migration. (from D-7)

## Candidate glossary terms

- **T-1**: **system release** — the atomic release unit (D-1): one tag shipping every engine binary, the payload, the deps bundle, the release manifest, and checksums under one system version. · avoid: *per-tool release* (the retired channel shape)
- **T-2**: **exec grammar** — the stable installed-repo invocation line `pyxis exec <tool> -- <argv>` (D-2): everything before `--` is pyxis's (fail-closed, UB-9), everything after passes verbatim. · avoid: *wrapper* (implies interpretation; the grammar forbids it)
- **T-3**: **first-mover shim** — the one bash artifact no Go can gate: the bootstrap that fetch-verifies pyxis itself (D-3, I1); minimal, pinned-URL, drilled. · avoid: *installer script* (the installer is Go; this only fetches it)
- **T-4**: **class policy** — the authored file mapping payload paths to ownership classes (D-4); the human input (AP-24) the release build derives the manifest from (AP-27). · avoid: bare *policy* (overloaded)

## Parked product gaps

none

## Gate

| Field | Value |
|---|---|
| **Verdict** | ready-for-design |
| **Date** | 2026-07-19 |
| **Authority** | @mdarabi (operator) |
| **Covering design** | — |
| **Coverage check** | pass — core four covered (§2 Decision → D-1/D-2/D-3, panels logged · §4 → ALT-1..9 verdicted · Serves+Scope → S-1..S-5 + basis anchors · §3.2 → O-1..4), zero blocking fog |
| **Amend route** | — |
