# Pyxis — live technical spec (rollup)

| Field | Value |
|---|---|
| **In one line** | How the shipped system is actually built — components, wiring, and surfaces, aggregated by reference. |
| **Relevant-when** | changing shipped code; asking where a capability physically lives; onboarding to the repo's tooling |
| **Authority** | rollup — the referenced designs govern; on conflict they win |
| **Update rule** | when a change alters shipped structure, update the affected sections in the same PR (AGENTS.md rule 3 — convention since amendment 017 retired FR-10's per-PR gate); freshness reads from `git log -- docs/techspec.md` |
| **Seeded** | 2026-07-07 — late: first code merged in #23; operator-caught, closed here |

## Components

- **feature-drive dogfood** (design:
  `docs/tech-design/feature-implementation-drive.md`):
  `cmd/feature-drive dogfood --workspace <empty-directory>` composes the
  feature lifecycle's release proof over two disposable Git repositories,
  one local-owned and one remote-owned. Each journey composes the production
  `Kickoff` ownership/routing boundary, reviewed `RecordLanding` predicate,
  `GitCandidateBuilder`/`GitCandidatePublisher`, and `VerifyDelivery` proof;
  materializes six task revisions; lands a seeded e2e repair into a distinct
  candidate; updates the discovered disposable target branch to that exact
  accepted candidate; and emits `feature-drive-dogfood/v1` JSON carrying the
  task heads, both candidates, delivery identity, and event journal.
  `DogfoodJournal.Validate` checks the exact `UB-1..UB-11` closed set and
  derives race/restart counts plus M1–M4 from unique authorized event evidence;
  unknown, missing, open, duplicated, skeletal, or caller-forged proof fails
  closed. The command refuses a non-empty owner repository rather than
  rewriting prior evidence.

- **docgate** (design: `docs/tech-design/docgate-v1.md`): `cmd/docgate`
  (thin main; exit 0/1/2; `--defects-json`) over `internal/docgate/`
  {`schema`, `ledger`, `fold` (pure — DINV-1), `gate`}; stdlib-only,
  Go 1.26; goldens + red fixtures under `internal/docgate/fold/testdata/`
  (manifest + defect catalog beside them). The **review-feedback ledger**
  (design: `docs/tech-design/review-feedback-capture.md` §3.1) is a second,
  path-distinct storage class in `internal/docgate/feedback` (pure fold →
  `effective.source.json` per subject under `docs/feedback/<subject>/`);
  the `fold`/`verify-generated` verbs and the `gate` umbrella dispatch it by
  path (`gate.IsFeedbackDir`), byte-verifying each subject's projection.
  Entries cite as `review-feedback:<subject>#<NNN>` (grep-resolvable); the
  base-free, registry-born `check-review-feedback-resolution`
  (`docs/tech-design/review-feedback-capture.md` §3.5) reds any such citation —
  bare or riding an amendment's `Learned-from: review-thread` free ref slot —
  that names no ledger entry, so the amendment model itself stays untouched.
  Its outbound sibling, the base-free `check-review-feedback-crosslinks`
  (same §3.5, FR-8), reds any **one-directional** cross-link an entry carries
  (`review:<report>#FND-n` to a reviewer M2 residual · `delivery:<slug>#E-n`
  to an Argus disposition — parsed into the projection's `cross_links`) that
  fails to resolve to its target; the read-only arm never writes the cited
  `review-report/v1` / Argus ledgers, so they are provably unmodified (NG-5).

- **plangate** (design: `docs/tech-design/plan-new-v1.md`): `cmd/plangate`
  (docgate's shape: thin main, verb dispatch, `--defects-json`, exit
  0/1/2) over five packages. `internal/plangate/plan` — the closed plan
  grammar (`NNN-plan.md` under the configured plans root), the fold gate
  suite (sizing thresholds from config, mechanical quarantine,
  expand–contract shape, cycle/overlap/coverage checks, all-or-nothing
  fog, charter durability lint, DINV-8 bucket purity), fold →
  `NNN-plan.json` (`plan/v1`, deterministic bytes), byte-exact `verify`.
  `internal/plangate/tracker` — the projection's one narrow seam (§3.2:
  additive write verbs only, exact bounded listings) + the gh-CLI GitHub
  adapter. `internal/plangate/dialect` — the harbormaster manifest
  (fs-spec:v1 spec comments, plan-task/epic markers, rig: stamps,
  trigger-string neutralization, operator-surface routing, [Epic]/[T<n>]
  title prefixes + native issue types as manifest data) + the contract
  test replicating the executor's spec-validity gate (A-6/DINV-9).
  `internal/plangate/project` — the stateless convergent projection
  (§3.7: probe by marker over exact listings; create → spec → link →
  wire → route-last; duplicate markers halt loudly, never auto-deleted;
  `--dry-run`/`--no-routing` staged modes; DINV-1 merged-plan check via
  `git show origin/main`; `plan-projection-report/v1` printed and posted
  on the epic) and the read-only `report` verb.
  `internal/plangate/render` (design: `docs/tech-design/plan-viewer-v1.md`)
  — the read-only `render` verb: folds the named (default highest)
  plan version in memory and, when green and its committed face is current,
  composes one self-contained `html/template` page (header region
  — feature, version, dialect, status, totals — a dependency-graph region,
  an anchored task-card region, the coverage matrix, the flags panel, and a
  fingerprint footer, inline-styled, auto-escaped) into
  `.pyxis/scratch/plan-<feature>-<NNN>.html` (or `--out`); fold-red, a stale/missing
  face, a bad `--version`, a missing directory, or a missing mermaid bundle
  each refuse with the named quadruple and zero writes (DINV-1/DINV-2). The
  graph region (`graph.go`) draws a mermaid `graph TD` from the folded edge
  set — each task one node (`T<n> — <title>`), each `blocked_by` one edge,
  the three foundation levels as distinct `classDef` styles (DINV-4) — drawn
  offline by the pinned `.pyxis/vendor/mermaid/mermaid.min.js` inlined at
  render time (byte-identical to the doc renderer's copy; its embedded
  element-closers are neutralized), keeping the page free of external
  references (DINV-3). The task-card region (`cards.go`) renders one anchored
  `id="T<n>"` card per task carrying its complete §4.5 charter — prose and
  list fields, traces, plan-task-id, and a badge row (kind, size band,
  foundation, blocked-by, routing, size/overlap acceptances, pattern) — every
  value auto-escaped and `Touches` (plan-internal) never surfaced (A-3/A-6/R2).
  The coverage region (`coverage.go`) renders the folded coverage matrix as one
  table — covered rows first (each requirement's serving task IDs as in-page
  `href="#T<n>"` links to their cards), then deferred rows carrying a distinct
  marking and their reason so a non-Must requirement is never silently dropped
  (A-4/FR-4/FR-15). The flags region (`flags.go`) aggregates exactly the folded
  model's recorded size/overlap acceptances in task order — each with its
  task-linked card and reason — or an explicit empty state when none is
  recorded (A-5/FR-5). Reuses `internal/docgate/schema` +
  `internal/docgate/config` (AP-11). Hermetic suites: plan fixtures +
  golden, dialect gate replica, projection interruption ladder / route-last
  checker / duplicate-race / freeze-refusal against a contract-faithful fake
  tracker, and the viewer's header/footer / graph node-edge-multiplicity /
  distinct-foundation-styles / task-card charter-completeness / schema-drift
  partition / anchor-uniqueness / coverage row-link-resolution / deferred-row
  distinctness / flags exact-membership / self-containment / escaping /
  refusal-matrix / filesystem-audit fixtures, and the proof pass — a
  byte-deterministic end-to-end golden over a maximal fixture plan
  (`testdata/maximal-plan.golden.html`, rendered with a stub engine so it
  stays legible and survives a mermaid re-vendor), the asset + write audits
  standing over the whole page, and the real `docs/plans/plan-viewer` plan
  rendered self-contained from its own committed bytes.

## Config surface (phase 1 of the layout design)

- **`.pyxis/config.yaml`** — closed-parse YAML subset read by
  `internal/docgate/config` (unknown keys are defects; absent file =
  defaults, which equal today's paths): paths (prds, tech-designs,
  solution-shapes, rollups, incidents, plans, work-items), template pins, the
  `capability_paths` trigger set (operand-tunable; check-standalone-docs'
  FR-30 fence — its FR-10 consumer was retired by 017), and the closed `plan:`
  sizing-threshold block plangate's fold enforces (plan-new-v1 §3.10).
  **Schema v2** (`schema_version: pyxis-config/v2`; installer-v1 §3.7, the
  installer's "pyxis-config surface" binding) added the `system:` block (the
  pin pair `version` + `manifest_sha256` — the trust root, DINV-2 — plus the
  `artifact_source` FR-22 override and `mode: release|source`), the five
  install `knobs:` (tracker · ci_mode · wiring · cli_surfaces · argus, captured
  at init — FR-3), and the `epoch:` block (written once by init). The paths
  block enforces the refuse-non-default posture (a non-default value is the
  named not-yet-configurable refusal until the adoption follow-up — FR-18/
  UB-8), and a conformance check asserts no engine configurable-path read lacks
  a schema key (§3.7). During the migration window the parser dual-reads the
  legacy per-tool pin sections (docgate/codegate) with a deprecation surface
  through doctor; the source repo is now v2 `mode: source`.

## Release channel (phase 3 of the layout design)

- **`docgate-release` workflow** — tag `docgate/vX.Y.Z` → static binaries
  (darwin/linux × arm64/amd64, CGO off) + `SHA256SUMS` as a GitHub
  Release. **`docgate-resolve.sh`** is now a one-line **exec-grammar delegate**
  (installer-v1 §3.10, T12): the fetch-verify-cache logic that once lived here
  lives once in `pyxis exec` over `internal/resolve` — the shim only locates
  the root and delegates (`go run ./cmd/pyxis exec docgate -- "$@"` in the
  source repo's source mode). The `codegate-resolve.sh`,
  `argus-stamp-resolve.sh`, and `feedback-capture-resolve.sh` shims collapsed
  the same way; the three per-tool workflows keep their own `<tool>/v*` tags
  until a later release retires them. This is the migration the installer's
  atomic **system release** supersedes at the aggregate: one `vX.Y.Z` tag now
  publishes every scan-derived binary + the payload + the
  derived manifest (see the installer section above; the source workflow is
  staged at `.pyxis/dist/source-workflows/system-release.yml` pending the
  operator's `workflow`-scope hand-off, #483).
- **`codegate-release` workflow** (codegate-v1 §3.1) — the identical shape,
  one tag namespace over: `codegate/vX.Y.Z` → static binaries + `SHA256SUMS`.
  **`codegate-resolve.sh`** is codegate's own resolution point, section-scoped
  against the shared config surface exactly like docgate's (an earlier
  positional read of a shared multi-tool config was issue #485's lesson —
  every resolver on this surface parses section-scoped since). See "codegate
  — the verification harness" below for the additive `codegate:` pin section
  and the advisory dry-run workflow this release channel feeds.
- **The R2 distribution door `https://dist.pyxis.agenticlab.ai`** — the only
  distribution channel (operator direction 2026-09-11), carrying the canonical
  source release's artifact set on Cloudflare R2 (S3-compatible), fronted by a
  Basic-auth Worker (bucket public access off; the Worker is the only read
  path): the no-GitHub-account channel. Layout: `<tag>/<artifact>` plus the
  source `install.sh` VERBATIM at the bucket root (R2 consumers fetch via
  `PYXIS_ARTIFACT_SOURCE`, bypassing the script's gh path — no repo
  retargeting). Published by `pyxis-release mirror`, which requires
  `R2_ENDPOINT`/`R2_BUCKET` + the env-only credential pair
  `R2_ACCESS_KEY_ID`/`R2_SECRET_ACCESS_KEY`; absent or incomplete configuration
  refuses before publication. No GitHub distribution repository or
  cross-repository token is required: upload-or-verify per object with
  the same never-clobber and closed-set-per-tag refusals, install.sh as the
  one rolling object, cache-control split immutable-vs-5-minutes, SigV4
  signed in stdlib (`internal/release/r2.go`; the fake bucket verifies
  signatures by independent reconstruction). Consumers authenticate with
  `PYXIS_ARTIFACT_AUTH="user:password"` — env-only, never committed config
  — which `internal/resolve` attaches host-locked over TLS only and the
  shims pass as `curl -u`. Auth governs access, never acceptance: pins
  still gate every byte (DINV-2).
- **Artifact-source precedence** (`internal/resolve.EffectiveSource`,
  prd:installer/FR-30) — one order, most specific to this invocation first:
  the repository's `system.artifact_source` (where an `--artifact-source` flag
  is captured verbatim), then the channel the installer recorded, then the
  **built-in channel — the distribution door** (`resolve.DefaultChannelRoot`,
  prd:installer/FR-28). The source repository's Releases remain the canonical
  publish target and a supported override, never the default: a machine that
  recorded nothing — an operand CI runner, a binary copied between machines, a
  record written by a pyxis too old to carry a channel — would otherwise be
  sent to a private repository, and answered 404 with a remedy naming
  authentication when the wall was authorization. **Every rank is a file.** No configuration value is
  read from the environment — not the artifact source, not the channel, not the
  release pin — because a variable does not survive a restart, a new shell, or
  a cron run; an operator changes a setting with a flag and the flag is written
  down (operator ruling, 2026-08-20, taken without a PRD amendment: FR-30's
  wording still names an environment rank). Credentials are the sole exception,
  env-only in the opposite direction — they must never reach a file. The
  installer keeps its own environment inputs, which run before any config
  exists and are what populate the record. The first two are
  FLAT bases (`<base>/<name>`, so a mirror serving one release needs no tag
  layout); the recorded channel is a ROOT and composes `<root>/<version>` per
  fetch, which is why the record cannot strand the next upgrade on the tag it
  was written for. `internal/releaseenv` is the single reader of the
  installer's `$XDG_CONFIG_HOME/pyxis/release.env` record (pin pair +
  `PYXIS_CHANNEL`); the door credential is never recorded, staying env-only in
  `PYXIS_ARTIFACT_AUTH`. Consequence: after a doored install, `pyxis init`
  takes no channel or artifact-source flag, and the artifact-source preflight
  names the door rather than a GitHub host the operator never chose.
- **The public front door `https://pyxis.agenticlab.ai/install`** — the
  channel's getting-started line (installer PRD FR-2's "one curl-able
  bootstrap line"), serving `scripts/install_pyxis.sh` verbatim,
  unauthenticated, to `curl -fsSL … | bash`. It is pinless and
  version-agnostic by construction: it fetches the doored root `install.sh`
  with the consumer's `PYXIS_ARTIFACT_AUTH`, reads that copy's
  `PINNED_VERSION`, and hands off with `PYXIS_ARTIFACT_SOURCE` set to the
  matching `<tag>/` tree — so the endpoint tracks every mirror run without
  republishing, and no version or checksum leaves the door. Refusals are
  named and fail-closed (absent credential, a 401/403 door refusal, a body
  with no `PINNED_VERSION`, a tag that is not a tag); the TLS floor
  (ABUSE-4) is held in the bytes themselves — a plaintext `PYXIS_DIST_HOST`
  is refused except to loopback, which is how the offline suite serves a
  fake channel. The whole body is one function invoked on its last line, so
  a download truncated in flight runs nothing. `PYXIS_VERSION` +
  `PYXIS_SHA256` reach a prior tag's tree (the kill-switch path, FR-17).
  Covered by `internal/release/frontdoor_test.go`, which drives the real
  script against a loopback fake door.

## docgate v2 (universal lint + the tech-design index)

- **`internal/docgate/techdesigns`** — design-class checks (FR-11: closed
  header schema, typed Serves with enumerated refs only, mandatory
  Observability/Security pair, grandfather set + the check-scoped precedence grandfather (FR-16/I1:
  docgate-v2 exempt from the precedence clause only) from config) and the
  index generator (FR-12: `docs/tech-design/index.{md,json}` — liveness
  computed from Supersedes edges, byte-verified by `verify-index`).
- **Gate additions** (`gate_v2.go`): `lint-wiki`, `lint-rollups` (FR-13),
  `check-governing` (FR-14: the `Governing-Change: operator` trailer;
  principles append-only even with it), `lint-incidents` (FR-15: append-only is **entry-scoped** — committed
  `**I<n>**` blocks immutable, Links lines may grow for graduation,
  scaffolding editable; the seeded placeholder is replaceable by I1) —
  all in the umbrella; the merge-base ones ride pre-push/CI.

## docgate v3 (the #54 review wave)

- **`internal/docgate/gate/gate_v3.go`** — `LintGoverning` (FR-18 purity:
  no relative links / baseline requirement-IDs in the governing docs),
  `CheckConfigPolicy` (FR-23: `grandfathered_designs` /
  `precedence_grandfathered` / `capability_paths` changes need the
  `Governing-Change: operator` trailer), `CheckMaps` (FR-22: `INV-<n>`/`D<n>`
  row anchors in the configured system maps persist vs merge base),
  `CheckLinks` (FR-21: relative links across `docs/`, `AGENTS.md`, and the
  templates dir resolve). `CheckGoverning` now line-freezes **both**
  governing docs even with the trailer (FR-18); `CheckImmutable` gained the
  flat-corpus fence (FR-19, README exempt). `LintTechDesigns` (FR-20) takes
  a merge base: corpus refs resolve against the shared extractor
  (`CorpusIDs`), designs added since the base may not serve
  superseded/retired units, Status must record Approved.
- **`internal/docgate/incidents`** — the coded writer (incident-machinery
  FR-1/2): `add-incident` (dense ID allocation, lint self-check, atomic
  write, refusal leaves the registry byte-identical) and `graduate` (the
  sanctioned `Links:` growth). Shared shape predicate:
  `gate.LintIncidentBytes` (FR-25: kebab-tag content, `- **Field:**`-anchored
  lines, configured path). Entry schema pinned by
  `incident-entry-template/v1`; authoring surface `/incident`
  (`.pyxis/skills/incident` → `.pyxis/authoring/incident.md`).
- **Envelope v2** (FR-24) — `amendment-template/v2` adds an optional
  `Tracking` row; `ledger`/`fold` validate against the **declared** version
  (v1 amendments grandfathered forever; unknown versions red). Config pin
  bumped (`templates.amendment`).
- **Requirements coverage** (FR-26 → FR-28, coverage-on-read) — the coverage
  join (corpus IDs + live ledger units vs live designs' `serves[]`) is computed
  **on read** by the `coverage` verb (human table, `--json` machine face) and
  committed nowhere; the index (`schema_version` 2) carries only the design
  graph, so adding or amending a PRD never touches `docs/tech-design/`. CI
  publishes the view to the job summary. `splitRefs` strips backticks/prose
  from grandfathered free-text `Builds-on` rows (the garbled-edge fix, #54 E3).
- **Config keys added** — `paths.templates_dir` (default `.pyxis/templates`)
  and `system_maps` (default the two map docs); closed parse as ever.

## codegate — the verification harness

- **`internal/verify/`** (design: `docs/tech-design/codegate-v1.md`; PRD:
  `docs/prds/implementation-verification/`; plan:
  `docs/plans/implementation-verification/001-plan.md`) — the latency-ordered,
  five-rung deterministic verification ladder behind `cmd/codegate` (a thin
  main; the repo's mandated shape), stdlib-only Go. Split into focused
  packages: **`schema`** (the closed manifest/verdict/finding types,
  `DurationMS`'s millisecond wire format, the atomic temp+rename verdict
  write, `ValidateVerdictInvariants`/`ValidateCompleteness` enforcing
  DINV-3/DINV-4); **`registry`** (per-file `init()` self-registration —
  `Register(*Check)` — deriving the ladder from whichever checks are
  actually registered, never a hand-listed set, AP-27); **`ecosystem`**
  (compiled-in per-stack defaults: verb commands, test globs, secret/
  artifact/skip/assertion pattern tables, failure-identity parsers,
  state-dir and cache-pin defaults, manifest-override resolution);
  **`invoke`** (the `run` subcommand's contract: flag parse, the full
  refusal matrix at exit 2, clean-tree assertion, the git-version floor
  `2.5.0`, the six-binding resolution, the flock workspace lock);
  **`runner`** (the bounded verb executor: env stripped to the manifest's
  per-verb entries, wall-clock bound + termination grace, UB-1/UB-2 spawn-
  failure/timeout classing, and the write-scope confinement seam — a verb's
  writes are confined to the workspace + toolchain scratch by Landlock on
  Linux (an out-of-scope write is `ClassViolation`), a named no-op residual
  elsewhere; enforcement is opt-in via `CODEGATE_LANDLOCK`, default-off
  pending the Linux-CI matrix, and a confinement-setup fault refuses
  fail-closed rather than run the verb unconfined); **`diffset`** (the proposal diff extracted
  once — added lines, file status, rename detection — shared by every
  diff-based check); **`ladder`** (`Run`/`RunIncremental`: latency-ordered
  execution with first-failure stop, `skip:not-reached` for every rung a
  stop leaves behind, the green/green-degraded/red/error state derivation,
  T8's prior-verdict-as-sole-state incremental replanning); **`worktree`**
  (the ephemeral falsification worktree's lifecycle, plus
  `PruneStaleWorktrees` — the AP-12 janitor reaping a prior run's leftover
  worktrees, the crash/kill-recovery counterpart to the run-scratch reaping
  `invoke.PruneRunScratch` does at the next run's start); **`baseline`** (the
  no-new-failures file format and evaluation — `Parse`/`Load`/`Capture`/
  `Evaluate`/`WriteAtomicToFile` — driving rung 2's `no-new-failures` check
  (read side) and `capture-baseline` (the sole write side, DINV-6));
  **`checks`** (ten registered check units — see the rung table).

  **The CLI contract** — `codegate run --workspace <path> --diff-base <ref>
  [--incremental]`, `codegate capture-baseline --workspace <path>`
  (runs the test verb once and writes the manifest-declared baseline — the
  only path that writes a baseline, DINV-6), and `codegate seed
  [--workspace <path>]` (**`internal/verify/seed`**; workspace defaults to
  the current directory — the shape the welcome item and doctor's
  code-without-manifest advisory route to). Seed is
  prd:implementation-verification/FR-20's one-shot authoring aid
  (amendment 001): detects the stack from the ecosystem defaults' marker
  set (the single detection source doctor and adopt read) and writes the
  template `.pyxis/codegate.json` — verb commands from the ecosystem
  defaults where real defaults exist (go), named to-be-edited placeholders
  otherwise (node · python; a placeholder run unedited spawn-fails by
  name, never a silent green); refuses by name on an existing manifest
  (UB-10, operator-owned, byte-untouched) and on no or cross-stack
  ambiguous markers (UB-11, the hand-authoring remedy — never a guess),
  emitting plain stderr refusals, never a verdict artifact (it verifies
  nothing). One deliberate wall sits right after seeding, and seed's
  success output names it: the manifest is itself rung-0 machinery
  (FR-8), so the diff introducing it reds the gate by design and lands
  through the human lane — `run` passes once the diff base carries it.
  Plus `--help`/`--version`; new
  subcommands arrive only by design amendment (codegate-v1 §3.1). Exit
  classes: `0` pass-class (`green` · `green-degraded`), `1` `red`, `2`
  configuration/usage error (refuses before any rung executes — a dirty
  tree, a malformed manifest, an ambiguous invocation), `3` rung error
  (spawn failure, timeout, an out-of-scope write). The verdict is one
  atomically-written JSON document at the workspace's fixed
  `.codegate/verdict.json`; the manifest binds at the fixed
  `.pyxis/codegate.json` (distinct from the pyxis-wide `.pyxis/config.yaml`
  config surface below). Both paths are protected by the rung-0
  machinery-protection check rather than a preventive `.gitignore` — a
  diff that touches either reds rung 0 (DINV-2 belt-and-braces).

  **The ladder:**

  | Rung | Checks | Status |
  |---|---|---|
  | 0 — instant | `format`, `lint` (repo verbs) · `secret-scan` · `forbidden-artifacts` · `machinery-protection` | shipped |
  | 1 — build | `build` | shipped |
  | 2 — tests | `no-new-failures` (the bound test verb's per-test failure identities vs. a manifest baseline; skips when the stack has no failure-identity parser) | shipped |
  | 3 — agent-code | `falsification` (two-leg revert protocol in an ephemeral worktree) · `test-delta` (deletion/skip-marker/assertion-drop floors) | shipped |
  | 4 — reality | `exec-probe` (`skip:verb-absent` when unbound) | shipped |

  Finding classes emitted today: `machinery` (machinery-protection),
  `secret` (secret-scan), `forbidden-binary`/`forbidden-size`/
  `forbidden-path` (forbidden-artifacts), `new-failure` (no-new-failures),
  `tautological-test` (falsification),
  `test-renamed-out`/`test-deleted`/`skip-marker-added`/
  `assertion-count-drop` (test-delta), and `no-checks-executed` (the
  ladder's own fail-closed floor when every check skips — e.g. an unbound
  stack). Every finding carries `check_id`, `class`, `rung`, `path`/`line`
  where the class has one, a one-line `message`, and `log_ref` (full verb
  output by reference under run scratch, never inline).

  **Release channel and portability** (codegate-v1 §3.1, §3.11) — ships
  exactly like docgate (see Release channel above): `codegate-release` tags
  → static sha256-pinned binaries; `codegate-resolve.sh` is the resolution
  point (source mode in this repo; a pinned release elsewhere,
  fetch-verify-cache, fail-closed on a sha mismatch). The operand
  acquisition config (`.pyxis/config.yaml`, parsed by
  `internal/docgate/config`) carries an additive `codegate:` pin section —
  version + sha256, the same closed-parse shape as `docgate:`, mutually
  exclusive per DINV-2. The advisory, explicitly non-blocking
  `codegate-dry-run` workflow proves the portability claim: it runs
  `codegate run` headlessly against a freshly built copy of the
  `fixture-go` fixture and diffs the resulting verdict RUNG-FOR-RUNG (and
  check-for-check) against a reference verdict captured by an actual local
  run (`internal/verify/testdata/dryrun-parity/local-verdict.json`),
  logging the comparison as a structured `EVT-5` record
  (`cmd/codegate/dryrun_parity_test.go`, built only under the
  `dryrunparity` tag so it never joins the default `go test ./...` the
  docs-gate context runs — NR-1). `continue-on-error` is set at the job
  level; this workflow is never a required check (remote-gate enforcement
  stays parked, basis WR-5).

  **Multi-stack matrix and the catch suite** (codegate-v1 §3.11, A-1/A-2/A-3;
  FR-16) — `internal/verify/matrix` drives the real ladder against fixture
  repos of three distinct stacks (`internal/verify/testdata/fixture-{go,
  python,node}`; the non-go ones use scripted verbs over the stdlib
  toolchains, and `fixture-node` a gitignored `node_modules/` state dir that
  the rung-3 worktree must be provisioned with). The **matrix** proves the
  identical, stack-blind harness binds and adjudicates all three to the
  expected verdict on the clean and legitimate (test-only/refactor-only/mixed)
  diffs; the **catch suite** proves every adversarial seed — a tautological
  test, a silenced test (deletion, skip-marker, assertion-removal), and a
  config-pin downgrade — stays blocked with the expected finding class,
  emitting a per-seed `EVT-4` record. Both assert the workspace's tracked tree
  is byte-identical after every run (DINV-7); an inspection test asserts no
  harness package names a per-stack literal — the three stacks differ only in
  ecosystem-table data and their manifests, never in `internal/verify` code.
  Runs both under `docs-gate`'s suite and the dedicated `codegate-catch-suite`
  workflow (which sets up all three toolchains); a stack whose toolchain is
  absent skips rather than fails.

  **Troubleshooting:**
  - *Verdict state `error`, rung `0` skip-reason `configuration-error`* —
    the invocation itself was refused (exit 2) before any rung ran: check
    for a dirty workspace tree (`.codegate/` scratch is the only
    exemption), an unparseable `.pyxis/codegate.json`, an unresolved
    `--diff-base`, or a git below the `2.5.0` floor.
  - *A check's state is `skip:verb-absent`* — the manifest (and ecosystem
    defaults for the declared stack) bind no command for that check's verb;
    expected, and drives `green-degraded` rather than a failure — bind the
    verb in `.pyxis/codegate.json` to make the rung run for real.
  - *Rung `2`'s `no-new-failures` is `skip:parser-absent`* — the stack has no
    failure-identity parser (only `go` ships one today; python/node are bound
    for the other rungs but carry no parser), so per-test identities cannot be
    computed and the ratchet cannot be evaluated. Expected; drives
    `green-degraded`. A baseline declared for such a stack is instead a rung
    error (a claimed evaluation the stack cannot perform). Add the stack's
    parser (ecosystem `failureIdentityParsers`) to make rung 2 run for real.
  - *Rung `2` reds with `new-failure` findings* — the test verb produced
    failure identities absent from the bound baseline (or, with no baseline
    declared, any failure at all is new). Re-run `codegate capture-baseline`
    to re-snapshot the accepted set, or fix the regression. Unparseable test
    output with a baseline bound is a rung error, never a silent zero (UB-9).
  - *`--incremental` silently ran the full ladder* — a stderr notice, not
    an error: the prior verdict didn't qualify (a changed manifest,
    baseline, diff base, or codegate version, or none exists yet) —
    DINV-8's designed fallback.
  - *A finding's `log_ref` file is missing* — run scratch retains only the
    current run plus the prior verdict; older run scratch is reaped at the
    next invocation's start (§3.4's AP-12 janitor).

## impl-review (the machine-payload proposal reviewer)

- **`internal/implreview/`** (design: `docs/tech-design/impl-review-v1.md`;
  PRD: `docs/prds/impl-review/`; plan: `docs/plans/impl-review/001-plan.md`) —
  the unmanned proposal-class reviewer that runs once at a proposal's
  ladder-green boundary (before PR-raise) over the diff + task charter +
  codegate verdict, persisting closable schema'd findings and a downgrade-only
  verdict on the proposal itself; advisory until a ten-review adoption
  checkpoint. Stdlib-only Go, split into focused packages:
  - **`report`** — the closed, versioned `impl-review-report/v1` payload
    (`Finding`, `Disposition`, `Round`, `Report`, the `EscalationBrief`) and
    its `Validate`/`ValidateUnique`/`ValidateAppend`; parses **closed**
    (unknown key = error, `Unmarshal`). The finding's overlapping keys
    (`path`/`line`/`message`/`log_ref`) are byte-compatible with codegate's
    `Finding` grammar (a reflection test in `grammar_compat_test.go`, **not** a
    Go import — DINV-6/NR-1); the review-only fields are `rule_id`/`lens`/
    `severity`/`cal_row`/`acceptance`/`disposition`. One payload per
    proposal-review (DINV-1); each round binds its own `reviewed_revision`
    (DINV-7); every finding cites a `cal_row` its `severity` must match
    (DINV-3); a decline carries a mandatory `reason`.
  - **`verdict`** — the live **downgrade-only** verdict set (`approve` ·
    `approve-with-advisories` · `request-changes` · `escalate`) made
    structural: the closed `Authority` enum has no AUTO-granting member, so
    "the verdict grants AUTO" is unrepresentable (FR-7). `Compute` derives the
    verdict from the latest round's findings + dispositions (only an
    operator-attributed accept/reject resolves a blocker away); `SetEnvelope`
    stamps it onto the report; `staleness.go` treats a verdict whose head ≠ the
    latest round's `reviewed_revision` as **absent** (`review-stale`, UB-6);
    `signal.go` emits a self-contained `DowngradeSignal` a merge lane could read
    (emit only — consuming it into `internal/lane` is the owed lane amendment,
    design §3.6 Q4). Imports no `internal/lane` code (a local adapter proves
    shape-compatibility).
  - **`persist`** + its GitHub adapter — the two-method persistence port
    (`Persist(payload) → ref`, `Read(ref) → payload`); the GitHub adapter
    writes **one** bot-owned PR comment carrying the human rendering plus a
    fenced ` ```impl-review ` machine block, round-trips the payload
    byte-identically (incl. an oversize leg), renders the human face as a pure
    function of the payload (DINV-2), and on a write failure retries to a
    bounded cap then records a review-absence via an alternate signal — never
    split, mirrored, or lost (DINV-9/UB-5).
  - **`pack`** — the one shared read-only review context pack, assembled once
    over `context-pack-v1`'s admitted kinds (the proposal at its reviewed
    revision + codegate verdict as the frozen subject, the charter + governing
    design bindings as governing-peers, the calibration table as
    stage-calibration); admits **no** authoring-trail kind (no prior findings —
    the answer-key exclusion, DINV-5).
  - **`runtime`** — the sibling-session orchestration: the reviewer is a fresh
    top-level session, a **sibling** of the implementing worker (never a
    child), with the four lens subagents its own children over the one pack,
    each blind to the others; records a fresh-instance `context_id`+provenance
    that fails the check if it resolved to the implementing session (FR-2/
    NFR-3/DINV-5).
  - **`lens`** + **`calibration`** — the four lens instructions loaded as
    reviewed data, and the closed-parse `impl-review-calibration/v1` table
    (`| Row | Lens | Default severity | Escalation condition |`) every finding
    cites; a data-only row change flips a severity with no instruction edit
    (FR-12). **`floor`** reads the ladder-enforced defect classes **read-only**
    from codegate (`internal/verify`) to exclude any finding that restates the
    mechanical floor (FR-4/NG-4). **`band`** reads the proposal's risk class
    from the merge lane (no second risk model), applies skip/standard/deep, adds
    the ABUSE rows + incident registry only in the deep band, carries the
    dormant (off-by-default) second-instance verify, and records a
    size-escalation rather than approve over a truncated diff (FR-9/FR-10/UB-7).
  - **`protocol`** — the two-round dispute protocol: a blocking finding is
    disputed only with a reason, round 2 is scope-locked to named defects +
    changed spans (a finding on unchanged content is withheld-as-violation),
    round 3 is refused, every blocking finding is disposed before raise, and an
    applied-but-unverified finding holds the verdict at request-changes
    (FR-8/FR-15/UB-2/UB-4). **`charter`** is the invocation validation:
    charter-less ⇒ refuse; a materially-defective charter ⇒ escalate an
    upstream-defect class and withhold findings against the defective span
    (FR-14). **`lifecycle`** is the idempotent resume from persisted round
    state, the in-flight predicate + `absent-timeout` (the reap contract), and
    the recorded `review-absent` on a bounded failure (UB-1/UB-8).
  - **`harvest`** — the attribution-bound measurement harvest: an operator
    disposition counts only when its medium author **is** the operator (never a
    payload-writable field, so a forged "operator accepted" is dropped —
    FR-16/DINV-4); a full-proposal-universe enumeration reconstructs coverage
    (a never-reviewed proposal counted against it), precision over the
    disposition floor, the nit rate, and the miss corpus **from payload fields
    alone** (DINV-8). **`checkpoint`** is the advisory ramp (verdicts gate
    nothing until a recorded, per-repo, default-off operator promotion) and the
    ten-review checkpoint computation, surfacing a disposition-engagement flag
    (FR-11).
  - **`eval`** — the permanent eval suite (per-lens catches incl. deep-band
    legs, floor-exclusion + injection-invariance by a deterministic comparator,
    nit goldens, truncation, a payload-compactness bound), run N≥5×/release
    caught-every-run, with the band token/latency budgets as versioned data
    beside the calibration table (DOD-2).
- **The seam** (`cmd/implement-now`) — the reviewer invocation lands as one
  additive amendment of the VerificationGate class at the worker's ladder-green
  exit (the invocation slot only, drive-side rows untouched — NR-2); the host
  janitor reaps an orphaned review through the lifecycle in-flight predicate.
- **Executor access** — the Go supervisor stages and commits implementation,
  repair, and remediation edits before revision-based checks, and refuses to
  publish a dirty tree. Harness processes may inspect Git but cannot write its
  metadata. All four native adapters use an outer filesystem boundary:
  `sandbox-exec` on macOS and `bwrap` on Linux; launch fails when the boundary
  is unavailable. Ubuntu hosts with AppArmor user-namespace restrictions need a
  profile permitting `bwrap` to create those namespaces. CI loads the scoped
  profile in `.pyxis/security/bwrap.apparmor` and probes the boundary before the full gate.
  The profile ships with the installed payload. `pyxis init` probes native
  execution support and reports a remedy before setup; `pyxis doctor` reports
  unavailable support as an unhealthy prerequisite. Documentation setup can
  continue, while native launches fail closed. Supervisor-triggered verification
  commands and e2e admission tests use the same Git protection, with private
  scratch and toolchain caches. Unconfined cached verdicts cannot satisfy these
  runs, and supervisor Git mutations do not execute repository hooks.
  Implementation processes can write their task tree and native
  runtime state; review processes can write only their runtime state. Native
  state lives outside the repository, persists across task attempts, and is
  removed with the worktree. Older conversations restart observably when they
  first enter isolated storage. The environment excludes host Git and code-host
  overrides while retaining standard uppercase and lowercase proxy settings.
  Feedback disagreements are reported in terminal output for the supervisor.
  This enforces filesystem writes, not a per-tool allowlist or full
  read, network, credential, or IPC isolation.
- **Agent surface** — `.pyxis/skills/impl-review/` (`SKILL.md` invocation glue
  → `.pyxis/authoring/impl-review.md` the review protocol; `calibration.md`,
  the four `lenses/*.md` instructions, and `fixtures/` the eval corpus + budgets
  + provenance) under AP-25's vendor-neutral surface.

## maintenance-sweep (the read-only rot detector)

- **`cmd/maintenance-sweep/`** + **`internal/sweep/`** (design:
  `docs/tech-design/maintenance-sweep-v1.md`; shape:
  `docs/solution-shapes/maintenance-sweep-v1.md`; PRD: `docs/prds/maintenance-sweep/`;
  plan: `docs/plans/maintenance-sweep/001-plan.md`) — a thin, **verb-less** binary
  over one focused stdlib-only package: one invocation is **one bounded, read-only
  pass**. It pins the evaluated state (HEAD + a `dirty` annotation on a non-clean
  tree), loads the closed-parse policy, runs the three detectors in fixed order
  each isolated, emits **one `maintenance-report/v1`** machine face on stdout and
  the human face on stderr (explicit "0 findings" per clean detector — never
  silence), and exits by the pinned ranking: **0** clean, **1** findings, **2**
  fault. The invocation is fail-closed (an unknown flag or a flag-like positional
  refuses with usage + exit 2 before any detector runs — DINV-3, incident I3); the
  pass writes nothing beyond its two faces (**DINV-1** — the package holds no
  network client and makes no filesystem-write call, audited by the shim test).
  - **`policy`** — the single closed-parse policy file (`maintenance-policy/v1`,
    homed at `docs/policy/maintenance-sweep.yaml` via the additive
    `paths.maintenance_policy` config key): the staleness bound (default 14 days)
    and the two waiver classes (packages, plans). Every waiver entry is
    **reason-required** — a reasonless entry, an unknown key, or a wrong schema
    version refuses to load with a named defect (UB-2/DINV-3); an absent file is
    the legal empty policy with the default bound.
  - **`report`** — the deterministic report/finding schema: fixed detector order,
    lexicographic subjects, the four contracted finding fields
    (`detector`/`subject`/`evidence`/`remedy`), applied waivers surfaced with
    their reasons (FR-10/DINV-5), in-band per-detector faults (a fault never
    masquerades as clean — DINV-4), and **no wall-clock field** anywhere; marshal
    is byte-identical for equal input (DINV-2).
  - **the three detectors** (each a pure function of the evaluated state + policy)
    — **`unwired`** flags every first-party non-main package reachable from no
    `cmd/` binary (the import-graph union), the #514/#515 dead-feature class;
    **`touches`** reads each frozen plan's machine face and reports a task's
    path-like Touches token that names no file or directory in the tree, phrased
    as an **advisory** reconciliation signal (never a breach — FR-9), plan-waivable
    for in-flight plans; **`staleness`** reports a living rollup whose newest
    commit trails the newest `capability_paths` commit beyond the bound, with the
    age delta as evidence.
- **Scheduler shim** — `.pyxis/scripts/maintenance-sweep.sh` (test:
  `.pyxis/scripts/tests/test-maintenance-sweep-shim.sh`) — a delegate-only,
  bash-3.2-clean shim binding the sweep to the host scheduler exactly like the
  wake tick's (pattern reuse, never cohabitation — #ALT-3); its header documents
  the cron/launchd binding and the steady-state cost (one bounded pass per
  invocation, nothing resident between — AP-19/NFR-2). The operator installs the
  schedule; the repo never does.
- **The seeded policy** — `docs/policy/maintenance-sweep.yaml` carries the first
  live triage: 19 package waivers (14 impl-review skill-runtime/schema packages —
  a skill, not a binary, by design — and 5 test-harness/fixture/library packages)
  and 9 plan waivers (delivered-descriptive, relocated, or in-flight frozen
  plans), each reason-carrying. The live sweep on mainline emits exactly one
  verified-true finding — `internal/lane/state`, a merge-lane plan-T5 organ
  reachable from no binary (`cmd/lane`'s `state` verb still resolves through
  `internal/lane/acting`) — deliberately left un-waived for the operator to wire
  or waive.

## installer — the system release, the pyxis front-door, and converge

- **`cmd/pyxis`** (design: `docs/tech-design/installer-v1.md`; PRD:
  `docs/prds/installer/`; plan: `docs/plans/installer/001-plan.md`) — the
  front-door orchestrator, a thin main (exit 0 clean, 1 named refusals, 2
  usage/environment; no model inference — NFR-6) dispatching `init`,
  `converge`/`repair`, `doctor`, `exec`, and `version`. Every verb parses
  fail-closed through **`internal/cliguard`** (a flag-like positional, an
  unknown flag, or — on `exec` — a missing `--` refuses before any action; I3/
  DINV-5); everything after `--` passes argv-verbatim to the resolved tool. On
  unix a successful `exec` execve's the verified binary (never returns); a
  non-execve fallback relays the child's exit code (`exec_unix.go` /
  `exec_other.go`). Root discovery is the one shared predicate
  **`internal/reporoot`** (nearest `.pyxis/config.yaml` ancestor, then the git
  working-tree root) — so the orchestrator and every tool it execs agree on a
  source-less operand's root (the go.mod-walk defect T13 caught). The init
  report's closing surface (#948) links rather than describes: the welcome
  guide's URL rides the wire audit's `Refs` (created or probe-derived) into
  `Report.T8.Welcome.Ref`, the epilogue lists the filed onboarding items by
  short ref and ends on the send-off line, and rendering goes through
  **`internal/termstyle`** — ANSI only when the writer is a character
  device with `NO_COLOR` unset, styling that WRAPS but never changes the
  words, so pipes, tests, and machine consumers keep seeing byte-identical
  plain text.
- **`internal/upgrade`** (issue #1028) — the installed CLI's forward path,
  binary scope: discover the release the channel pins, fetch this platform's
  binary, verify it, replace the running executable. The trust anchor is
  `install.sh`'s `BEGIN PINS` block, read from the same places a fresh
  install reads it — the **GitHub leg** (`contents/install.sh` on the
  DEFAULT BRANCH names the release and carries its digests — never
  `?ref=<tag>`, since a tag cannot carry the pins for its own artifacts: the
  pins name the manifest hash, which names the source commit, so the pin
  commit necessarily lands after the tag, and reading at a tag would name the
  PREVIOUS release and downgrade a current machine — with the release asset
  serving the bytes, all over the operator's own `gh`) or the **channel leg** (`<root>/install.sh` + `<root>/<tag>/<binary>`,
  Basic credential from `PYXIS_ARTIFACT_AUTH`, environment-only). So the host
  chooses which version is current, never which bytes are acceptable: nothing
  is written until the fetched binary hashes to the pinned value (DINV-1).
  Moving BACKWARDS is never implicit: a pinned release older than the
  installed one (a channel rollback) refuses unless `--allow-downgrade` says
  so, and an unorderable pair fails closed the same way. Ordering is
  prerelease-aware — `v1.2.4-rc1` outranks `v1.2.3`, and a final release
  outranks its own prereleases — because reading a prerelease as its
  zero-padded core let a genuinely older release install over it.
  `--to vX.Y.Z` installs an explicit release in either direction and REQUIRES
  its `--sha256` — the same pairing `install.sh` requires of `PYXIS_VERSION`
  + `PYXIS_SHA256`, since the channel's pins describe only the release they
  pin — plus `--manifest-sha256` whenever the target is not, byte for byte,
  that pinned release. The discovered manifest describes the release whose
  BINARY the channel pins, so it is borrowed only when the supplied
  `--sha256` IS the channel's digest for this platform; naming the same
  version with different bytes refuses, because that manifest cannot
  describe them. The pin hand-off is unconditional: every install leaves
  `pyxis init` resolving what was just installed, so a target with no
  recordable manifest digest is refused rather than silently leaving the
  previous release pinned. Given all three the channel is never consulted, so an
  unreachable or malformed `install.sh` cannot block installing a release the
  operator has already pinned by hand. An up-to-date run is not a no-op: it
  reconciles the pin, which is what makes "re-run upgrade" a real remedy
  after a run that swapped the binary and then failed to record it. The
  pins block resolves each key's EFFECTIVE final assignment and validates
  after, so a later block that blanks a digest can never inherit an earlier
  release's; a missing or malformed `MANIFEST_SHA256` refuses before any
  fetch rather than half-upgrading (binary replaced, older pin left for
  `pyxis init`), and a source build — one reporting version `source`, whose
  executable under `go run` is a throwaway — refuses with installation
  guidance instead of "succeeding" against a temp file. A channel is
  **https-only, credential or not**: it supplies both the pins this verb
  trusts and the binary it installs, so plain http would let an intermediary
  choose the bytes AND the digest they are checked against. The client also
  refuses to FOLLOW a redirect that leaves TLS (Go copies `Authorization`
  over same-host hops regardless of scheme) or — while a credential is in
  play — one that changes origin.
  The install is atomic — staged beside the target so the rename cannot cross
  filesystems, then renamed over the running binary (safe on unix: the live
  process keeps its inode) — and BOTH write destinations (the install
  directory and the pin's home) are probed BEFORE any fetch, so neither an
  unwritable one costs a download nor strands the install half-done. A
  failure after the swap discloses that the binary was already replaced,
  rather than reporting only that the command failed. `release.env` is
  rewritten exactly as `install.sh` writes it, so `pyxis init` stays flagless.
  An installed REPOSITORY is out of scope by ruling: it keeps its own pin and
  moves by converge, and the verb names that path rather than editing it.
- **`internal/relmanifest`** — the closed release and policy schemas (`release-manifest/v2`:
  per-file sha256 + ownership class, per-binary per-platform hashes, system
  version, source commit, optional dependency-bundle hash; `class-policy/v1`:
  glob → class, first match wins), with legacy `release-manifest/v1` parsing
  that still requires a valid dependency-bundle hash,
  the ownership-class enum (engine-verbatim · repo-owned · generated), and
  `Walk(payloadRoot, policy)` deriving the manifest from a payload tree — an
  unclassed file fails the walk by name (DINV-6, A-11). The authored
  `.pyxis/class-policy.yaml` is the human input (AP-24); the manifest is the
  derived aggregate (AP-27).
- **`internal/resolve`** — the acquisition core, the ONE checksum floor
  (DINV-1). `Tool(ctx,cfg,tool,platform)` resolves config pin → verified
  manifest (fetched into the gitignored `.pyxis/bin/` cache, verified against
  the config pin before first use) → the platform's expected sha256 →
  re-hash-before-every-exec of the cached binary → fetch-verify (tmp + atomic
  rename, artifact-source-override-aware) on a miss/mismatch; `PreWarm` fetches
  the full tool set, `Reap` is the janitor removing off-version cache entries.
  The refusal taxonomy is the package's contract, each typed and named, none
  ever accompanying a usable path: `TamperError` (expected-vs-actual),
  `OfflineError` (the pre-warm remedy), `MissingPinError`, `WrongPlatformError`,
  `SourceTreeAbsentError`, and `SourceMode` (the DINV-8 fall-through). Transport
  is the `Fetcher` seam (`HTTPFetcher` + `FileFetcher` behind `ByScheme`; tests
  inject counting/scripted fetchers). Which credential may travel is decided by
  the HOST that will answer, never by the precedence rank that chose it: against
  a github.com base — a github-hosted `artifact_source` such as the source repo — a
  token (env, else `gh auth token`) upgrades it to the authenticated API
  asset route private repos require, scoped per-request so a third-party
  mirror never sees the token. A NON-github artifact source with
  `PYXIS_ARTIFACT_AUTH="user:password"` set (the R2 door) instead gets the
  Basic-credential fetcher — host-locked to the source, TLS-only (plain
  http refuses by name), 401/403 surfaced as the named door refusal.
- **`internal/converge`** — the ownership-class reconcile engine.
  `Run(ctx,cfg,opts)` repairs engine-verbatim byte-exact, regenerates
  generated, validates-never-rewrites repo-owned (per-file `RefusedByName`,
  DINV-3), orchestrates the pre-warm + janitor, and is idempotent + kill-and-
  re-run convergent (per-file atomic writes; the reap scan tolerates abandoned
  `.tmp-` fetches); `opts.Preview` enumerates the exact write/fetch set and
  performs none of it (FR-19). `Doctor(ctx,cfg)` is the read-only face over the
  SAME drift predicate — the closed `CheckClass` taxonomy (drift · prerequisite
  · hooks · tracker · surfaces · epoch · deprecation · cache · skeleton ·
  verification) with finding/advisory severities (advisories never red the
  run), zero mutations. `verification` advises when recognized code marker
  files exist with no `.pyxis/codegate.json` manifest, naming the codegate
  seed command (`pyxis exec codegate -- seed`) — silent once a manifest
  exists or when no recognized code is present (prd:init-hardening/FR-15).
  Emits EVT-2 (`converge`) / EVT-3 (`doctor`) via slog.
- **`internal/installer`** — `init`'s pipeline (`Init(ctx,opts) → Report`): an
  ordered stage list — the front-loaded **`internal/preflight`** check
  registry (admission, content-based and `pyxis adopt`-routed, DINV-5; the
  Git toolchain floor, UB-7/FR-3; cold-cache artifact-source
  reachability, FR-5 — every miss refuses pre-write by name, `init_refused`;
  a clean pass fires `init_checks_passed`; init-hardening/001 T1/T2), knob
  resolution (the five; flags/answers promptless — FR-4 — or, interactively,
  **`internal/installer/repochoice.go`'s one repository question** opening the
  interview: `internal/wire`'s `ListEmptyRepositories` offers the empty,
  unarchived, writable repositories the ambient credential can see (the
  GraphQL query carries `ownerAffiliations` explicitly — its default drops
  every org-owned repo — plus `pageInfo` so a truncated 100-repo window is
  never reported as "you have none"), then "create a new one" and "keep it
  local — no GitHub at all". The answer DERIVES `wiring.repo` and, for a
  local install, `tracker=carina · ci_mode=local · protection=none`, which
  declares zero gh-transport ops, so the GitHub questions are not asked at
  all (FR-26/FR-25, amendment 002). The menu carries **no default**: unlike
  every other knob, whose default is a policy value, this one names an
  external destination, so a blank answer re-asks and EOF resolves to no
  destination and stops pre-write (UB-12) — a defaulted menu would wire
  labels, protection, and issues into a repository nobody chose. A listing
  failure is logged and still offers create/local rather than removing the
  local option from the operator least able to reach GitHub; an unresolvable
  typed name is ambiguous by construction (404 covers both "absent" and
  "private and invisible to this credential"), so it is put to the operator
  naming the active gh account rather than silently created), the UB-10
  identity probe (ambient → answers → prompt,
  resolved BEFORE any mutation), the v2 config + operand `.gitignore` writes,
  converge, then the T8 tail (seed the four governance skeletons + operand
  `AGENTS.md` knob-substituted + empty-but-valid registries — rollups absent;
  hooks into `.githooks` + `core.hooksPath`; tracker/CI
  wiring; the real docgate green proof; the welcome item), and the two-commit
  epoch (commit #1 the converged tree, commit #2 the config's epoch block
  citing #1 — the honest self-reference resolution). Fires EVT-1
  (`install_completed`) only after the whole pipeline succeeds; all writes
  staged by explicit path (never add-all). Seams: `Options.Fetcher`
  (artifact transport) and
  `Options.WireRunner` (the github-mode gh boundary — the e2e matrix injects a
  transcript fake here; production defaults to `wire.GHRunner{}`).
- **`internal/wire`** — GitHub wiring as ONE declared-operation table (AP-11).
  `Table(cfg)` derives the declared set from the captured knobs (create-repo ·
  ensure-labels · push-workflow · set-protection · welcome-item ·
  onboarding-items, each with its
  knob condition and transport — `gh` or converge-carried). The
  onboarding-items row (#947) is declared last: it ensures the
  installer-authored standing set — author the vision, author the
  principles, seed the code gate — plus one finish-the-wiring item derived
  from the walk's own hand-offs (the same rendered block the terminal
  shows, preserved where it cannot scroll away), each idempotent via a
  per-item body marker (`OnboardingMarker`, the welcome item's AP-22
  convention) with created-issue URLs collected on the audit row's `Refs`.
  The carina lane files the identical set into the local store beside the
  welcome item (`installer.createCarinaOnboarding`). `Preview` renders
  it, `Execute` walks exactly it through the `Runner` seam (`GHRunner` shells
  the user's ambient gh; tests inject a transcript), and the `Audit` (EVT-4)
  records what ran — the executed set can never exceed the declared set
  (ABUSE-3). Failure posture per UB-4: gh absent → the named GitHub-mode
  prereq refusal before any op (`GHAbsentError`); a permission gap completes
  the permitted subset + emits a copy-paste hand-off (reports partial);
  transient failures (classified from gh stderr as data) retry within a bound
  then hand off. `RequiredCheckContext` (`docs-gate`) is pinned against the
  shipped installed-workflow job id by a consistency test.
- **`internal/payload`** — the instantiable naming set + substitution engine:
  `SkeletonTargets` (the four governance skeletons → their operand doc paths),
  the operand `AGENTS.md` manual, the welcome item, and the installed-repo
  workflow variant (`InstalledWorkflow` `.pyxis/dist/github-workflows/
  docs-gate.yml` → `InstalledWorkflowTarget` `.github/workflows/docs-gate.yml`);
  `Substitute` fills knob variables fail-closed (an unresolved variable is an
  error). Governance skeletons carry structure + fill-me guidance only, zero
  invented governing content (NG-1).
- **`internal/release`** + **`cmd/pyxis-release`** — the build-only release
  machinery (never shipped in the payload; `ShippedTools` scans `cmd/*` EXCEPT
  `pyxis-release`, AP-27). `Pack` stages the payload, asserts the release
  preconditions (no retired-resolver reference, the installed workflow
  toolchain-free, every staged file classed —
  fail-closed and named), refuses an out dir already holding another
  release's artifacts (`SHA256SUMS` covers the whole dir, so packing over a
  leftover set would publish it under the new tag), writes the deterministic
  payload archive, copies the scan-derived binary matrix in, derives the
  manifest via the class-policy walk, and finishes with `SHA256SUMS`. `Verify`
  re-hashes the set against the sums + manifest and asserts every artifact
  belongs to the release under verification; `Publish` is upload-or-verify
  re-runnable via gh on the canonical source repository (never clobbers,
  a divergent asset refuses). `Mirror` fetches the canonical set (unless
  `--out` carries it), verifies its checksums and manifest, and publishes
  only to R2 (`internal/release/r2.go`: the same closed set under
  `<tag>/` plus the verbatim install.sh at the bucket root, SigV4-signed,
  upload-or-verify with the same refusals). R2 configuration is required;
  a prefetched set needs no GitHub runner. It refuses before any remote
  operation when the checkout's `install.sh` pins a different tag (the
  consumer-stranding hazard as a precondition). `Build` compiles the
  scan-derived matrix (`-trimpath`, `-s -w -X main.version=<tag>`,
  `CGO_ENABLED=0`) — the one place those flags live — then runs the
  host-platform front-door binary and refuses the set unless `pyxis version`
  reports the built tag (the #868 drift as a named refusal; skipped only on
  hosts outside the matrix). `Pins` DERIVES `install.sh`'s `BEGIN PINS` block (the
  consumer trust anchor: the manifest hash plus the front-door binary's hash
  per platform) from a verified packed set, or — with no `--out` — from the
  published release's own bytes, hashed on download and cross-checked against
  that release's `SHA256SUMS`; the spliced result is re-read through the same
  parser `Mirror`'s precondition uses, so an anchor that would strand
  consumers cannot be written. `Cut` composes the runbook: reuse-or-build the
  set, pack, verify, assert the tag resolves to the packed source commit
  (annotated tags dereferenced; `--push-tag` creates it there), publish
  canonically, regenerate the pins — and, with `--mirror`, publish to R2,
  refusing missing R2 configuration before any stage and refusing while `install.sh` is
  uncommitted. That refusal is structural, not stylistic: the pins name the
  manifest hash, the manifest names the source commit, so a tag can never
  carry the pins for its own artifacts and the pin commit necessarily lands
  after the tag, through a reviewed PR. The verbs:
  `pyxis-release tools|build|pack|verify|publish|pins|mirror|cut`.

## The bootstrap shim (the one shell first-mover)

- **`.pyxis/scripts/pyxis-bootstrap.sh`** — the single admissible shell
  first-mover (it must run before any Go binary exists; installer-v1 §3.4). It
  fetch-verifies `pyxis` itself against THE DOCUMENTED PIN (the version + sha256
  the install instructions carry, deliberately independent of the release
  manifest pyxis is needed to parse), warm-cache-aware, then installs into
  `.pyxis/bin/` and EXECS NOTHING (delegate-only; the caller puts `.pyxis/bin`
  on PATH). A pin mismatch refuses naming expected-vs-actual with zero
  installation (DINV-1, ABUSE-4); when curl cannot serve a github.com base
  (a private source repository ignores tokens on plain download URLs) it
  falls back to an authenticated `gh release download`, github-scoped; the
  `PYXIS_ARTIFACT_AUTH` door credential rides `curl -u` when set (and only
  then); bash-3.2-clean, POSIX-sh compatible, ≤45 code
  lines, covered by `.pyxis/scripts/tests/test-pyxis-bootstrap.sh` and the Go
  harness (`internal/release/shim_test.go`).

## The installer e2e matrix

- **`internal/installer/e2e`** — the scratch-repo end-to-end harness
  (installer-v1 §3.11). `TestMain` builds real `pyxis` + `docgate` once
  (`!testing.Short()`), a `file://` fixture release serves them under their
  artifact names + a real-instantiable payload + the derived manifest, and
  every leg drives the SHIPPED binary as a subprocess (the one in-process
  exception is the offline egress scan and the GitHub-mode wiring assertion). The
  **local half (T13)**: the offline combo (network-denied, a counting
  `denyFetcher` IS the egress scan + a dead-proxy subprocess tripwire), the
  drift matrix (repair/regenerate/refuse × second-run no-op), kill/re-run,
  fresh-clone self-heal (online re-fetch · offline named refusal), the misparse
  suite (every verb, exit 2, snapshotted zero-mutation), and the removal test.
  The **matrix tail (T14)**: `github_test.go` (github-mode combos through the
  injected `wire.Runner` transcript — the executed wiring set equals
  `wire.Table`'s declared set, and the installed workflow's job id matches the
  set-protection required-check context — FR-20/A-8), `parity_test.go`
  (source-vs-installed verdict parity, zero divergence — the NR-2 reporoot-fix
  proof), `tamper_test.go` (the binary/payload/deps-bundle/bootstrap drills +
  the declared-operation audit — DOD-5/A-2), and `budget_test.go` (the recorded
  per-invocation ≤100ms-p95 and install ≤5min budgets — A-9/NFR-1, asserted
  against generous ceilings; the real enforcement is the reference-runner CI).
  `findings_test.go` keeps the reporoot-fix regression proof. Runs inside
  `docs-gate`'s `go test -race ./...` (ubuntu) and the staged
  `installer-e2e.yml` matrix (linux + darwin).

## Wiring

- **CI**: `.github/workflows/docs-gate.yml` and the Forgejo source template
  run the shared list from `.pyxis/config.yaml` in the same cheap-then-expensive
  order as local `code-gate run`. Each check's optional `group` field defines
  its execution group: `tests` for process boundary, catch suite, and race
  tests; `default` for all other checks, including newly configured checks
  without a group. Local `code-gate run`, the pre-push hook, and the
  in-process executor run phases in order (default 1; `test-race` and
  `test-coverage` are phase 2), groups concurrently inside a phase, and
  checks sequentially within each group. A required red cancels sibling
  groups and skips later phases. Reports stay in configuration order.
  `code-gate run --cheap` is the phase-1 discovery set. Hosted CI runs that
  set first, then `--group default --phase 2` and `--group tests --phase 2`
  only after it succeeds. `ci-parity` rejects workflow-specific check lists,
  sequential group invocations, missing or repeated group-phase slices, and
  later-phase jobs that do not wait for every earlier required check, and
  verifies that the jobs cover every required check and the document gate
  exactly once. Standalone diagnostic workflows may rerun one registered
  required check with `--check`; those invocations cannot certify a group
  or the complete gate. A lightweight final `docs-gate` job requires
  every check job to succeed, including after failure, cancellation, or skipping.
  The quality job also runs `docgate gate --merge-base "$PYXIS_CI_BASE"`,
  using the platform's trusted event base (incl.
  check-immutable fencing PRD ledgers **and** tech designs via config paths —
  FR-4/FR-17, index artifacts exempt; check-single-amendment, FR-27 — at most
  one added ledger entry (`NNN-<slug>.md`) per PR; and check-standalone-docs,
  FR-30 — a governed doc (PRD ledger / tech design / incident) ships without
  code or a second governed class; check-rollups, FR-10, was retired by
  amendment 017). Each check job maintains a separate Go cache on main. The
  installer's own **`installer-e2e`** matrix
  (linux + darwin) runs the harness isolated (staged at
  `.pyxis/dist/source-workflows/installer-e2e.yml` pending the operator's
  `workflow`-scope hand-off, #483). Required-context status:
  operator-controlled in branch protection.
- **Desk**: `.pyxis/hooks/pre-commit` (merge-base-free verbs) and
  `.pyxis/hooks/pre-push` — delegate-only shims; `.githooks` is a symlink
  to `.pyxis/hooks`, so activation stays `git config core.hooksPath .githooks`.
  The push shim delegates to `pre-push` (FR-29): the verb reads the hook's
  stdin ref lines and passes deletion-only pushes without content checks
  (design: `docs/tech-design/prepush-ref-awareness.md`); anything else runs
  the same merge-base gate.

## Agent surface (AP-25)

- Phase 2 complete: bucket 2 lives under `.pyxis/` — `skills/ scripts/
  templates/ authoring/ hooks/ vendor/`; root integration points
  (`.agents/skills`, `.codex/skills`, `.claude/skills`, `.claude/scripts`,
  `.githooks`) are symlinks into it. Human document review uses source
  Markdown and PR feedback; the Go plan viewer inlines the pinned Mermaid
  bundle for standalone HTML previews. Installation and packaging consume only
  artifacts declared by the release manifest. New releases use
  `release-manifest/v2`; existing v1 manifests remain readable under their
  original integrity rules. Run `pyxis upgrade` or the current bootstrap
  installer before repinning a repository with v1-only tooling to a v2
  release; old v1-only manifest readers reject v2.
- **plan-new** — `.pyxis/skills/plan-new` (glue) →
  `.pyxis/authoring/plan-new.md` (the guide: input gating, tracer-bullet
  decomposition, fog's all-or-nothing bounce, the plangate fold loop, the
  plan-PR checkpoint, staged projection) + `.pyxis/templates/
  plan-template.md` (`plan-template/v1`, pinned in config `templates.plan`;
  the grammar's authoritative bytes stay `internal/plangate/plan/parse.go`
  + goldens).
- **feature-brainstorm** — `.pyxis/skills/feature-brainstorm` (glue +
  `fixtures/`: the scripted-operator eval corpus, runner protocol, static
  golden/defect records, eval reports) → `.pyxis/authoring/
  feature-brainstorm.md` (the guide: orient → overlap check → framings →
  grill → typed fog → gate → verdict-driven landing; resume/reopen; the
  Never list) + `.pyxis/templates/brainstorm-record-template.md`
  (`brainstorm-record/v1`: nine typed sections, checklist Appendices A/B,
  citation grammar Appendix C — the single definitions `prd-new`'s
  gated-record path references). Records land under `docs/brainstorms/`
  (none yet). Design: `docs/tech-design/brainstorm-record-model.md`.
- **tech-design-brainstorm** — `.pyxis/skills/tech-design-brainstorm`
  (glue + `fixtures/`: scripted-authority corpus incl. seam-rule/override/
  bounce/injection fixtures, static golden/defect records, eval reports) →
  `.pyxis/authoring/tech-design-brainstorm.md` (the guide: basis gate →
  orient → DQ framing → grill → seam-ruled panel with audit log → typed
  fog → park-or-bounce → five-verdict gate → landing incl. the two-PR
  needs-more-product shape; resume/reopen; the Never list) +
  `.pyxis/templates/design-brainstorm-record-template.md`
  (`design-brainstorm-record/v1`: twelve typed sections with the four
  stage-3 organs — Basis, ALT-n ledger, panel log, S-n seam map —
  checklist Appendices A/B, citation grammar Appendix C referenced by
  `tech-design-new`'s gated-record path). Records land under
  `docs/brainstorms/` as `<slug>-tech-design.md` (none yet). Design:
  `docs/tech-design/design-brainstorm-record-model.md`.
- **read-to-rule (§0)** — `prd-template/v2`
  (`.pyxis/templates/prd-template.md`, pinned in config `templates.prd`;
  docgate's supported parse set is `{prd-template/v1, prd-template/v2}` —
  registered in `internal/docgate/schema`, accepted at the base parse in
  `internal/docgate/ledger`): the §0 human-only routing section (four
  blocks with honest-zero forms, cap 40 source lines / ≤ 7 ranked calls,
  tier rule — required T2/T3, T1 skippable) plus the capture-line
  convention as gate-table guidance (per-item `[in|out]` routing,
  `review-feedback:` entry-anchor citations, the sampled-read slot, both
  fallbacks, the staged-gate rule). The `prd-new`/`prd-amend` guides carry
  the synthesis step and the wholesale `op: supersede §0` maintenance step
  (declared at the computed contract-changing tier). Exercised by
  `TestPRDTemplateV2AndSectionZero` (`internal/docgate/fold`). PRD:
  `docs/prds/read-to-rule/`; design: `docs/tech-design/read-to-rule-v1.md`.
- **ecosystem-help (`/help`)** — `.pyxis/skills/help` (glue: the frontmatter
  `name`/`description` routing card + the one-line pointer at the guide) →
  `.pyxis/authoring/help.md` (the guide: classify the ask; read the live
  surfaces — the pipeline map whole, every `.pyxis/skills/*/SKILL.md`
  frontmatter, wiki routing headers on demand; compose the catalog / route /
  first-steps answer; the degraded walk and the Never list). Pure prompt-walk
  — no binary, no generated artifact, no state: derivation logic lives only
  in the guide, and the purity invariant (#DINV-1) is that no concrete skill
  or stage name appears in either help file. Invocation reaches every host
  CLI through the existing `.pyxis/skills` symlinks (no per-skill pointer).
  The pipeline map carries the cross-cutting `/help` row. Its mechanical
  purity proof (the scripts-test conformance test) and the routing eval
  fixtures are the separately-tracked plan tasks T2/T3. Design:
  `docs/tech-design/ecosystem-help-v1.md`; PRD: `docs/prds/ecosystem-help/`.
