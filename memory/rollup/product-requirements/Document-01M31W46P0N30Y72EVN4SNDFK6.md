# Pyxis — live product requirements (rollup)

| Field | Value |
|---|---|
| **In one line** | What pyxis actually does today — shipped capability only, aggregated by reference. |
| **Relevant-when** | asking "does the system do X *now*?"; scoping work against shipped reality rather than plans |
| **Authority** | rollup — the referenced projections/designs govern; on conflict they win |
| **Update rule** | when a change alters shipped capability, update the affected sections in the same PR (AGENTS.md rule 3 — convention since docgate amendment 017 retired FR-10's per-PR gate); freshness reads from `git log -- docs/prd.md` |
| **Seeded** | 2026-07-07 — late: the first capability shipped in #23 (2026-07-06); the gap was operator-caught and closed here |

## Shipped capability

- **read-to-rule §0 — the PRD's human-only routing section.** Requirements
  contract: `docs/prds/read-to-rule/effective.md` (projection, **as-of
  000**); design: `docs/tech-design/read-to-rule-v1.md`. Shipped: the
  `prd-template/v2` §0 contract (four blocks, honest-zero forms, cap +
  tier rule) with the capture-line convention as gate-table guidance; the
  additive `prd-new`/`prd-amend` §0 steps (synthesis; wholesale
  `op: supersede §0` at the computed contract-changing tier); docgate's
  v2 registration (supported set `{v1, v2}`, §0 wholesale-supersede
  exercised by the fold fixture test). Pending, deliberately: the prove
  (the first born-with-§0 base, PRD DOD-4), the reviewer ledgers' §0-skip
  adoption (PRD Q2), and the ten-gate checkpoint — capability today is
  the authoring surface, not yet the measured bet.

- **docgate v1 — the document gate and fold.** Requirements contract:
  `docs/prds/docgate/effective.md` (projection, **as-of 005**) — eight FRs
  (deterministic fold FR-1, defects-as-data FR-2, verify-generated FR-3,
  check-immutable FR-4, validate-amendments FR-5, diff FR-6, verify-rebase
  FR-7, lint-structure FR-8) plus FR-9 (001: commit-time guard); FR-10
  (002: rollup guard — a capability diff had to update these very rollups
  or attest otherwise) was **retired** by 017: rollup updates are AGENTS.md
  rule 3 convention, no longer gate-forced.
  FR-11..FR-15 (003: the v2 universal-lint contract) are **shipped**:
  lint-tech-designs, the generated tech-design index, wiki + rollup
  metadata lint, check-governing (the trust root's machine backing),
  and lint-incidents. FR-11 is superseded by FR-16 (004: requirement
  precedence, from incident I1) — **shipped**: a design citing a prd:
  ledger must anchor a real requirement row; stub anchors alone fail. FR-17 (005: design
  immutability — INV-41's CI half for the tech-design class) is
  **shipped**: check-immutable fences both frozen classes. Enforcement: `docs-gate` CI on every PR
  and push to main; `pre-commit`/`pre-push` hooks via `core.hooksPath`.

- **docgate review wave (#54) — fences, integrity checks, coverage, and the
  incident writer.** Requirements contract: `docs/prds/docgate/effective.md`
  (**as-of 011**) and `docs/prds/incident-machinery/effective.md`. Shipped:
  FR-18 (vision append-only + abstract-layer purity), FR-19 (placeholder-corpus
  byte-freeze), FR-20 (Serves hardening: corpus refs resolve, liveness at
  introduction, Approved status), FR-21 (`check-links`), FR-22 (system-map
  registry stability), FR-23 (gate-policy config keys need the operator
  trailer), FR-24 (envelope v2 with optional Tracking, validated by declared
  version), FR-25 (incident lint checks content, not silhouettes), FR-26
  (requirements-coverage face of the index) — and incident-machinery FR-1..4:
  `add-incident` (coded writer, dense IDs), `graduate` (the sanctioned Links
  growth), `incident-entry-template/v1`, the `/incident` skill.

- **plangate — plan fold and verify (the plan-new contract's artifact
  half).** Requirements contract: `docs/prds/plan-new/effective.md`
  (as-of 000); design: `docs/tech-design/plan-new-v1.md`. Shipped: the plan
  document class parses closed and gates structurally — sizing/quarantine/
  pattern (FR-4..FR-8, thresholds as config data per FR-6), dependency
  graph + overlap sequencing (FR-10/FR-11), charter completeness +
  durability lint (FR-12/FR-13), coverage with all-or-nothing fog
  (FR-15/FR-16), trace resolution (FR-2/UB-2), and fold-computed
  foundation/size stamps (FR-22's data half) — folding to a deterministic
  machine face with byte-exact verification (NFR-1). Projection is
  shipped: idempotent create→spec→link→wire→route-last onto GitHub in the
  executor dialect (FR-18/FR-19/FR-21/FR-23, UB-4/UB-5), plan-time stamps
  rendered (FR-22), machine-readable report printed + posted on the epic
  (FR-24), additive-only tracker verbs (FR-20/NR-2), duplicate races halt
  loudly, and projection refuses any plan not matching its merged form
  (FR-17's freeze half). The authoring surface is shipped: the `/plan-new`
  skill (`.pyxis/skills/plan-new` → `.pyxis/authoring/plan-new.md`, AP-25)
  carries input gating (FR-1), overlap surfacing (FR-3), the fog test's
  all-or-nothing bounce, and the checkpoint flow (plan PR review = FR-17's
  session half; merge = freeze), over `plan-template/v1`
  (`.pyxis/templates/plan-template.md`, config-pinned). **Not yet
  evidenced:** live executor dispatch (FR-21's M2) — awaits the first
  dogfood plan. No plan documents exist in the repo yet.

- **feature-brainstorm — stage-1 elicitation (the pipeline's front door).**
  Requirements contract: `docs/prds/feature-brainstorm/effective.md`
  (as-of 000); design: `docs/tech-design/brainstorm-record-model.md`.
  Shipped: the brainstorm-record contract (`brainstorm-record/v1` template —
  typed sections with `D/F/G/O/T/P-n` anchors, structure + coverage
  checklists, the `brainstorm:<slug>#D-n` citation grammar); the
  `/feature-brainstorm` skill (guide + glue: orientation over routing
  surfaces, overlap check, 2–3 framings, one-question-at-a-time grilling
  with recommendations, facts-by-lookup, live-operator-only decisions,
  typed fog, explicit gate, verdict-driven landing under
  `docs/brainstorms/`); `prd-new`'s additive gated-record input path
  (FR-13/NR-1); and the scripted-operator eval harness (8 behavioral
  fixtures + 4 seeded-defect records; first run: golden green with EVAL-2
  at zero, every defect record red at its intended checklist row).
  **Not yet evidenced:** the prove phase — DOD-4 (a real feature driven
  idea → record → synthesis) and DOD-5 (operator-judged gate) — and the
  remaining behavioral fixtures' first runs (on-demand per its WR-5). No
  real records exist under `docs/brainstorms/` yet.

- **tech-design-brainstorm — stage-3 elicitation (the design front door).**
  Requirements contract: `docs/prds/tech-design-brainstorm/effective.md`
  (as-of 000); design: `docs/tech-design/design-brainstorm-record-model.md`.
  Shipped: the design-brainstorm-record contract
  (`design-brainstorm-record/v1` — typed sections with the four stage-3
  organs: typed `Basis` header, `ALT-n` alternatives ledger with
  principle/incident scoring, the per-question panel log, the `S-n` seam
  map; structure + coverage checklists; the
  `design-brainstorm:<stem>#D-n` citation grammar); the
  `/tech-design-brainstorm` skill (basis admit/refuse, seam-ruled panel
  with force/waive audit, park-or-bounce for requirement holes,
  five-verdict gate incl. the two-PR needs-more-product shape);
  `tech-design-new`'s additive gated-record input path (NR-2); and the
  scripted-authority eval harness (12 behavioral fixtures + golden and six
  seeded-defect records; first run: golden green with EVAL-2 at zero,
  every defect red at its intended checklist row; full gate proven green
  with a sibling record present in `docs/brainstorms/` — NR-3 verified by
  execution). **Not yet evidenced:** the prove phase — DOD-4 (a real
  design through basis → record → authoring) and DOD-5 (authority-judged
  gate) — and the remaining fixtures' first runs. Zero stage-1 surface
  edits (NR-1 held). No real records of this class exist yet.

- **codegate — the verification harness (stage-8 local CI).** Requirements
  contract: `docs/prds/implementation-verification/effective.md`
  (projection, **as-of 001**); design: `docs/tech-design/codegate-v1.md`.
  Shipped: the `codegate run --workspace --diff-base [--incremental]`
  subprocess contract with its full refusal matrix and exit classes (FR-4,
  UB-3, UB-7); the closed manifest/verdict/finding schemas with
  DINV-1/DINV-3/DINV-4's invariants held as property tests (FR-4, FR-17,
  FR-19); the derived, self-registering check registry (NFR-6); rung 0
  (format, lint, secret-scan, forbidden-artifacts, machinery-protection —
  FR-7, FR-8, NFR-1), rung 1 (build — FR-9), rung 3 (falsification's
  two-leg ephemeral-worktree protocol — FR-11, FR-18; the test-delta
  audit's deletion/skip-marker/assertion-drop floors — FR-12), and rung 4
  (exec-probe — FR-13, `skip:verb-absent` when unbound — FR-6); incremental
  re-verification with the prior verdict as sole state (FR-3, DINV-8); the
  runtime-hardening leg — the bounded verb runner's write-scope confinement
  seam (Landlock on Linux, a named no-op residual elsewhere) plus the
  crash/kill-recovery janitors that reap a prior run's stale worktrees and
  run-scratch on the next invocation (UB-6, AP-12); and, landing with this
  entry, the release channel — sha256-pinned static
  binaries via the `codegate-release` tag workflow, the additive
  `codegate:` acquisition-pin section (the same closed
  `internal/docgate/config` surface as docgate's own pin; fails closed on a
  sha mismatch, DINV-2), and the advisory, explicitly non-blocking
  `codegate-dry-run` GitHub Actions job proving rung-for-rung headless
  parity with a local run (FR-15, NFR-3, EVT-5); and the multi-stack fixture
  matrix with its permanent catch suite — the identical stack-blind harness
  binding and adjudicating go, python, and node fixtures through ecosystem-table
  data alone (zero per-stack harness code, structurally asserted), with every
  adversarial seed family (a tautological test, a silenced test via
  deletion/skip-marker/assertion-removal, a config-pin downgrade) held blocked
  and EVT-4-recorded on all three stacks, and the paired legitimate diffs left
  untouched (FR-16, A-1/A-2/A-3); and rung 2 — the no-new-failures floor
  (FR-10, UB-9): the bound test verb's per-test failure identities compared to
  a manifest-declared baseline, reding on any failure the baseline does not
  carry and recording the ratchet's shrinkage, with `codegate capture-baseline`
  writing that baseline (the only write path; `run` never writes it, DINV-6) —
  a stack with no failure-identity parser skips the rung rather than guessing;
  and `codegate seed` — amendment 001's one-shot manifest authoring aid
  (FR-20): stack detection from the ecosystem defaults' marker set (the
  single source doctor's advisory and adopt read) writing the template
  manifest the operator owns — real default commands where the ecosystem
  ships them, named to-be-edited placeholders otherwise — refusing by name
  on an existing manifest (UB-10) and on no or cross-stack-ambiguous
  markers (UB-11), closing the loop both init-hardening truth surfaces
  (the welcome item, doctor's code-without-manifest advisory) route to;
  its success output names the one deliberate wall ahead — the
  manifest-introducing diff is itself FR-8 machinery that reds the gate
  and lands through the human lane.
  **Not yet shipped:** default-on Landlock write-scope enforcement —
  the confinement machinery ships but stays opt-in via `CODEGATE_LANDLOCK`,
  default-off pending the Linux-CI enforcement matrix (A-6). No host consumes the harness yet: wiring
  implement-now's Worker to it is a queued additive amendment on that
  PRD's own ledger (NR-2), tracked separately.

- **upgrade — the installed CLI's forward path (issue #1028).** `pyxis
  upgrade` brings the installed binary onto the release the channel pins,
  reusing `install.sh`'s pins block as the trust anchor rather than adding
  one: verify-before-write (DINV-1), atomic replacement of the running
  executable, the `release.env` pin hand-off on EVERY install (an explicit
  `--to` supplies its own manifest digest rather than skipping it) with pin
  reconciliation when the binary is already current, and `--preview` reporting the
  whole would-do set while performing none of it. **Scope is the binary by
  operator ruling** — an installed repository keeps its own pin and moves by
  converge; upgrade names that path and never edits it. This un-parks the
  binary half of the "upgrade UX" the installer PRD's appetite cut-line
  parked; the repository half stays parked.

- **installer — the atomic system release, the `pyxis` front-door, and
  converge.** Requirements contract: `docs/prds/installer/effective.md`
  (projection, **as-of 000**); design: `docs/tech-design/installer-v1.md`.
  Shipped: the **`pyxis` orchestrator** — one binary owning `init`,
  `converge`/`repair`, `doctor`, `exec`, and `version`, every verb parsed
  fail-closed (a flag-like positional, an unknown flag, or a missing `--`
  refuses before any action — UB-9/DINV-5) and spending zero model inference
  (NFR-6). The **exec grammar** `pyxis exec <tool> -- <argv…>` is the one
  path every installed-repo tool invocation flows through: resolve → verify →
  execute, re-hashing the cached binary against the pinned release manifest
  before EVERY execution, fetch-verifying atomically on a miss, refusing
  by name (tamper, offline, wrong-platform, missing-pin) with zero execution
  on any unverified or ambiguous path (FR-13/FR-17/FR-21/UB-2/UB-3/DINV-1);
  source mode falls through to `go run` where the source lives (DINV-8). The
  **atomic system release** — one tag builds every scan-derived binary per
  platform, the payload archive, and the
  build-derived release manifest (per-file hash + ownership class, per-binary
  per-platform sha256), published upload-or-verify with checksums; an operand
  pins exactly the system version + the manifest hash, the trust root
  (FR-1/FR-2/FR-22/NFR-2/DINV-2). **Converge** reconciles a repo against the
  pinned manifest per ownership class — engine-verbatim repaired byte-exact,
  generated regenerated, repo-owned validated-never-rewritten (violations
  named) — idempotently and kill-and-re-run convergent, with `--preview`
  enumerating the exact write/operation set and performing none of it, and
  the cache janitor shipping with the cache (FR-5/FR-6/FR-19/UB-5/DINV-3/
  DINV-4/AP-12/AP-22). **`pyxis init`** brings a brand-new repository onto the
  pinned release: admit-or-refuse the target (UB-1), the five knobs
  (flags/answers promptless — FR-4; interactively the same five are settled
  by four questions plus ONE repository question that names the install's
  destination — an existing empty repository, a new one to create, or a
  local-only install with no GitHub at all — deriving `wiring.repo` rather
  than asking it, and suppressing the GitHub-only questions on a local
  answer: FR-26/FR-25 (002). A blank or EOF answer to that question is no
  destination chosen, never a default one — UB-12), the committer
  identity (UB-10), the v2 config + the two-commit epoch, converge, seed the
  governance skeletons + operand manual + empty-but-valid registries
  (rollups correctly absent — NG-1), install review-runtime deps
  (offline-bundle or online), activate hooks, wire the tracker/CI per the
  knobs (carina store · the GitHub declared-operation table over gh · the
  installed-repo workflow), and end on a green gate with a welcome item
  plus the onboarding work items (#947) — the setup only the operator can
  complete (author the vision, author the principles, seed the code gate,
  finish any handed-off wiring), filed as tracker items in both lanes so
  the remaining human work lives where work lives, not in scrollback
  (FR-7..FR-12/FR-15/FR-16). The closing surface links and celebrates
  (#948): the welcome guide's URL and the filed items' refs render in a
  styled epilogue (TTY-only ANSI via `internal/termstyle`, `NO_COLOR`
  honored, words byte-identical when piped) that ends the screen on the
  send-off. **Doctor** is the read-only face — drift by
  ownership class, environment health, the epoch, and advisories, zero
  mutations (FR-14/UB-7). **GitHub wiring** is one declared-operation table
  executed through the gh CLI with preview, audit, permission hand-off, and
  bounded retry — the executed set never exceeds the declared set
  (FR-10/UB-4/ABUSE-3). The **source repo consumes the new path itself**: the
  per-tool resolver scripts collapsed to one-line exec-grammar delegates, the
  config flipped to schema v2 source mode, and the full source-repo gate and
  suite stay green through it (NR-1/NR-3). The **e2e matrix** proves the
  packaged deployment context on scratch repos, linux + darwin: the offline
  combo (network-denied, egress-scanned), drift/idempotency/kill legs,
  fresh-clone self-heal, the misparse suite, the removal test, the GitHub-mode
  wiring combos (transcript-faked), the source-vs-installed parity suite (zero
  verdict divergence — NR-2), the supply-chain tamper drills, and the recorded
  time budgets (per-invocation ≤100ms p95, install ≤5min — A-9/NFR-1/NFR-2).
  **Not yet done:** cutting the first live release tag (an operator act — the
  system-release + installer-e2e source workflows are staged at
  `.pyxis/dist/source-workflows/` pending the `workflow`-scope hand-off,
  #483); live GitHub Actions on a real scratch repo (the
  deferred remote leg — the hermetic transcript proves the wiring set). No
  operand has been installed from a published release yet; the matrix installs
  from a file:// fixture release.

- **ecosystem-help — the `/help` front door (post-install discovery).**
  Requirements contract: `docs/prds/ecosystem-help/effective.md`
  (projection, **as-of 000**); design: `docs/tech-design/ecosystem-help-v1.md`.
  Shipped: the **`/help` skill** on the vendor-neutral surface
  (`.pyxis/skills/help` → `.pyxis/authoring/help.md`, AP-25; invocable from
  every host CLI through the existing `.claude`/`.agents`/`.codex` skills
  symlinks, FR-5) — a stateless prompt-walk skill that derives all three
  answers live from the routing surfaces and holds no catalog of its own:
  the capability catalog grouped by pipeline position with each stage's
  built-status (bare `/help`, FR-1), a described problem routed to its entry
  command with the map's own reason (`/help <problem>`, FR-2), and a
  newcomer's first-steps path (`/help start`, FR-3/FR-6) — honest at every
  edge: an unbuilt stage is named with its status, never handed back as a
  command (UB-3); an unroutable ask gets an explicit non-entry + the map
  pointer (UB-2); a missing surface is named, never fabricated for (UB-1).
  The pipeline map gained its cross-cutting `/help` row; the derivation
  reads routing surfaces only, never a corpus body (NFR-1, #DINV-4). The
  **derivation-purity conformance test** (plan T2) mechanically proves FR-4/
  #DINV-1/#DINV-3: `.pyxis/scripts/tests/test-help-purity.sh` harvests the
  live capability names at run time and reds if any appears in help's own
  files, proves a newly-dropped skill surfaces with zero help-side edits,
  and reds on a planted write/cache instruction — each detector carrying a
  self-verifying red-run; a `payload_test` wrapper runs it under
  `go test`/code-gate so CI enforces it. **Not yet shipped:** the routing
  eval checklist with recorded runs (plan T3 — EVAL-1); the newcomer-proxy
  dogfood (PRD M1/DOD-4) awaits its exposure checkpoint.

- **e2e authoring line — e2e-design · e2e-review · e2e-plan (stage-10 e2e
  authoring).** Requirements contract:
  `docs/prds/e2e-verification/effective.md` (projection, **as-of 001**);
  design: `docs/tech-design/e2e-verification-artifacts-v2.md` ·
  `docs/tech-design/e2e-verification-skills-v2.md`. Shipped: three
  single-invocation skills carrying a fully-implemented feature to a
  reviewed e2e test-coverage design and an ordered plan. **e2e-design**
  authors the new `e2e-design/v2` doc — a seven-section class with a
  coverage map anchored to the feature's PRD requirement rows and topped up
  from its live (non-superseded) technical-design set and built code;
  scenarios bearing one of `run-for-real`/`mocked`/`manual-human` per route,
  cost class, and cadence; and a closed-slot harness section (the build form
  specifying mechanics for each of ten mechanical elements plus a conditional
  decision-authority stand-in, or the reuse form citing an existing harness);
  plus two scenario-keyed tables naming each scenario's fixtures, seams, and
  exclusively-owned artifact — no app-shape-specific skill code
  (FR-1..FR-6, FR-18..FR-23, NFR-1); it refuses and produces nothing on a
  missing PRD/design or a not-yet-implemented feature, naming the gap
  (UB-1, UB-2), and reads the live design *set* rather than refusing on
  mere plurality — the split-design case this very feature is (UB-2).
  **e2e-review** stands up as the fourth reviewer-program instance — three
  blind fresh-context lenses, coverage and anti-gaming (judging whether
  required behaviors are covered and scenarios are not hollow) and the new
  buildability lens (judging whether harness-slot answers are sufficient
  for a builder), held as data over the reused context-pack/sibling-session
  machinery, emitting an `e2e-review-report/v2` (its own review-report family
  instance, not a foreign schema forced) with `blocking`/`advisory` findings
  structurally bounded to two convergence rounds (FR-7, FR-8, FR-11, FR-24,
  NFR-4). **e2e-plan** turns an *approved* design doc into an ordinary
  plan-new `plan/v1` — one task per scenario, plus a harness task when the
  harness section is in build form (detected by reading the section, not by
  path lookup), plus a conventions task when the repository carries no
  scenario built against that harness, with the conventions task `blocked-by`
  the harness task and every scenario task `blocked-by` each conditional task
  emitted — reusing plan-new's pipeline and the executor's ready-frontier
  verbatim, with no new task type and zero edits to either; it refuses an
  unapproved doc, and refuses an approved one whose scenarios do not each
  name an exclusively-owned artifact, naming the offending scenarios and
  projecting zero tasks (FR-14..FR-17, FR-26, UB-4, UB-9, NR-1, NR-2).
  Distilled for a human reader at `docs/wiki/e2e-authoring-reference.md`
  (the design-doc shape, the three lenses, the plan handoff); the living
  techspec's own entry carries the package breakdown. Also shipped: the
  no-automatable-surface stop that produces zero documents and halts the line
  on a feature with nothing to automate (UB-7); the blocking-convergence loop
  that settles every finding before approval and gates advancement to
  e2e-plan, with the human review half running alongside it into the same
  loop (FR-9, FR-10, UB-5); and the seeded catch corpus measuring the three
  lenses' catch-rate, which measures and never gates (DOD-3,
  EVAL-1/EVAL-2/EVAL-3). **Not yet shipped:** no feature has been carried
  design → review → plan to an accepted plan (DOD-2, DOD-8, M1) — the
  dogfood is unrun and `docs/e2e/` holds no artifacts. It is an operator-run
  acceptance milestone, deliberately kept out of the plan so the fleet cannot
  claim it from the ready frontier.

The engine does not exist; requirements for it live in `docs/prds/`
(baseline + VNEXT) as *requirements, not capability*.
