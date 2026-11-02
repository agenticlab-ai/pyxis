# .pyxis/config.yaml — the pyxis config surface (repo-layout-and-system-footprint
# §3.6/§3.8; schema v2 per installer-v1 §3.7/§3.10). Closed-parse: unknown keys
# are defects. Paths default to these values when this file is absent; this repo
# states them explicitly. A machine may add `.pyxis/config.local.yaml`; Pyxis
# recursively overlays its keys on this tracked file, and git ignores it. Keep
# machine-specific preferences there rather than changing shared policy.
#
# Schema v2, source mode (the source-repo migration, plan installer/001 T12): the
# system pin PAIR replaces the per-tool pin sections (docgate:, codegate: — now
# gone), and `system.mode: source` runs every engine tool from this repo's own
# source through the one exec-grammar path (`pyxis exec <tool> -- …`), pyxis
# itself included. No per-tool sha256 pins here — source mode and release pins
# are mutually exclusive (DINV-2), and the source tree is authoritative (DINV-8).
schema_version: pyxis-config/v2
system:
  version: "0.1.0"
  # release | source. Source mode is legal only where the tool's source lives
  # (DINV-8) — this repo carries every operand's cmd/ tree, so it runs them all
  # from source. An installed repo instead pins a release: system.version +
  # system.manifest_sha256 (the trust root; the pair is the whole pin surface).
  mode: source
paths:
  prds: docs/prds
  tech_designs: docs/tech-design
  # The solution-shape corpus — distinct from and never nested under
  # tech_designs, either way (solution-shape-v1 §3.1, DINV-3).
  solution_shapes: docs/solution-shapes
  rollup_prd: docs/prd.md
  rollup_techspec: docs/techspec.md
  incidents: docs/observability/incidents.md
  plans: docs/plans
  # The instantiable-templates scan root (template integrity, FR-21).
  templates_dir: .pyxis/templates
# plangate's split thresholds (plan-new-v1 §3.10) — data, retuned by config
# PR on measured evidence (prd:plan-new/FR-6; seeds: the predecessor's
# baselines recorded in the plan-new PRD's evidence rows).
plan:
  max_acceptance_criteria: 4
  max_scope_bullets: 4
  max_est_logic_lines: 300
  max_task_share_pct: 50
templates:
  prd: prd-template/v2
  amendment: amendment-template/v2
  tech_design: tech-design-template/v1
  plan: plan-template/v1
precedence_grandfathered:
  - docgate-v2
grandfathered_designs:
  - prd-amendment-model
  - tech-design-conventions
# Pre-existing dangling citations that predate `check-citations` and live in
# artifacts INV-41 / the ledger add-only rule forbid editing — a genuine catch
# the check surfaced, but one that can only be corrected when the frozen
# artifact is next superseded/amended (fixing it in place fails check-immutable
# / check-ledger-history). Each `<file> | <token>` names the exact dangler; the
# comment gives the correct target so the successor authors it right. NEW
# dangling citations are never on this list, so the gate still bites for them.
grandfathered_citations:
  # frozen design (INV-41) — cites the stage-3 record but the anchor is D-5 of
  # the stage-1 record; correct: brainstorm:argus-boundary-enforcement#D-5.
  - docs/tech-design/argus-attribution-gate.md | design-brainstorm:argus-boundary-enforcement-tech-design#D-5
  # merged amendment entry (check-immutable) — a full path where the grammar
  # takes a slug; correct: design:argus-attribution-gate.
  - docs/prds/argus-boundary-enforcement/001-attribution-hardening.md | design:docs/tech-design/argus-attribution-gate.md
  # merged delivery-ledger E-entry (add-only) — Q2 is a PRD open question, not a
  # brainstorm anchor; correct: problem:argus-attribution-hardening#Q2.
  - docs/problems/argus-attribution-hardening/delivery.md | brainstorm:argus-attribution-hardening#Q2
capability_paths:
  - cmd/
  - internal/
  - .pyxis/
  - .github/workflows/
# Living map documents whose registry anchors are fenced against silent
# drops (check-maps, FR-22).
system_maps:
  - docs/domain-model.md
  - docs/system-design.md
# The implement-now engine's block (executor config surface, #476). docgate and
# carina skip it as a foreign consumer's block (#478); only the executor parser
# reads it. The executor's local gate is now the `code-gate:` block below, run
# in-process by the Worker BEFORE its first push (the local-first convergence
# loop) and again by the remote self-heal loop (code-gate-mirror.md, DINV-1) —
# it replaces the former hand-listed `executor.gate`, whose argv-only surface
# could not extend to gofmt or the race detector. The authenticated impl-review
# successor is intentionally not installed here: its production activation is
# a later, human-merged configuration change after the exact activation proof
# required by implement-now-authenticated-review-obligations §3.16 is green.
# When a `review:` block IS present, the Worker runs the local-first loop by
# default — review the exact local revision after the local gate, remediate on
# request-changes, publish only an accepted revision, then remote CI once:
#   review:
#     backend: claude          # optional; absent = inherit the implementation backend
#     remediation_rounds: 3    # automated review→remediate cycles before the task escalates
#     pre_publish: true        # false = review only post-green, as before
#   session:
#     continuity: true         # false = every launch is a fresh conversation (no
#                              # resume across fixer/remediation/heal/feedback rounds)
executor:
  # One readable log is reused for every run of the same feature. Relative
  # directories resolve from the repository root; *.log is gitignored.
  logging:
    directory: .pyxis/logs/implmentor
  # Model and effort are selected from each provider's shipped defaults. A
  # normal installation needs no routing keys, ACLs, or model table here.
  # To prefer a different logged-in provider, add only:
  # routing:
  #   default: claude
# The code-gate — the single-source, stack-scoped code quality gate
# (docs/tech-design/code-gate-mirror.md). This ordered list IS the required-CI
# code predicate: one `code-gate` verb runs it, and every surface (the executor
# gate, the pre-push hook, and required CI) invokes that same verb over this
# same list, so the local gate mirrors required CI by construction (DINV-1).
# Correctness checks only — environment-dependent cost, speed, and size
# measurements never gate (DINV-2/3). Deterministic per-file statement coverage
# is a source-code predicate, so every measured Go file must clear its floor.
# This remains distinct from and never runs the doc-gate/docgate (DINV-5).
# Closed-parse (AP-6). Each check is a
# block: `command` (required, run directly — no shell), `fail-on`
# (exit [default] | output), `class` (required [default] | advisory),
# `timeout-s` (optional), `env` (optional space-separated extra pass-throughs).
# `group` (optional, default: default) assigns independent execution groups.
# Groups run concurrently on every surface; checks within a group keep file
# order. Hosted jobs select whole groups with --group, never their own lists.
# This is the starter list for pyxis's own Go stack (the required-CI code
# checks). The executor's drive-to-green loop runs this list in-process; the
# pre-push hook and docs-gate.yml invoke the same list through code-gate.
code-gate:
  ci-parity:
    command: go run ./cmd/ci-parity
  memory:
    command: go run ./cmd/memory-gate
    env: PYXIS_CI_BASE PYXIS_CI_EVENT GITHUB_ACTIONS
    timeout-s: 600
  gofmt:
    command: gofmt -l .
    fail-on: output
  vet:
    command: go vet ./...
  lint:
    # Pin the tool here so local and hosted checks cannot select different
    # installed versions. The external tool stays outside the engine's go.mod.
    command: go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2 run ./...
    timeout-s: 600
  build:
    command: go build ./...
  process-boundary:
    group: tests
    command: go test -count=1 -run ^TestProcessAccess ./internal/executor/runtime
    env: PYXIS_REQUIRE_PROCESS_CONFINEMENT
  hook-shim:
    command: bash .pyxis/scripts/tests/test-docgate-hook.sh
    env: PYXIS_CI_BASE
  catch-suite:
    group: tests
    command: go test -v -run 'TestFixtureMatrix|TestCatchSuite|TestHarnessCarriesNoPerStackCode|TestStacksDeclaredOnlyInEcosystem' ./internal/verify/matrix/...
    timeout-s: 600
  test-race:
    group: tests
    command: go test -race -timeout 25m ./...
    env: PYXIS_REQUIRE_PROCESS_CONFINEMENT PYXIS_CI_BASE
    timeout-s: 1200
  test-coverage:
    command: go run ./cmd/coverage-gate -min 90
    env: PYXIS_REQUIRE_PROCESS_CONFINEMENT PYXIS_CI_BASE
    timeout-s: 1200
