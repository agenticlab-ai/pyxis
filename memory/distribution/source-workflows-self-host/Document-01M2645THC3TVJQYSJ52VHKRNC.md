# self-host — the config-driven-portability self-host acceptance job
# (config-driven-portability-v1 §3.8, the basis's M1/DOD-1; plan
# docs/plans/config-driven-portability/001 T6; prd:config-driven-portability/NR-1;
# #A-1, #A-2, #A-3). It is the CI half of A-1: it certifies, on every push/PR,
# that the pyxis engine runs green over its OWN corpus relocated under a
# NON-DEFAULT config — a target-facing literal missed anywhere fails here, on
# the real corpus, because a non-default config no longer lets a missed value
# coincidentally match its default (R1, DINV-1).
#
# What runs:
#   1. The self-host harness (internal/selfhost) under `go test -race`. This IS
#      the relocate-corpus-under-a-non-default-config proof: it archives the
#      real corpus into a throwaway tree, relocates it (doc corpora moved,
#      default branch renamed, explicit remote set, surface pointers moved), and
#      runs the FULL doc gate + doctor + the branch/remote runtime resolution,
#      asserting green. It also carries the coherence inspection (A-2) and the
#      default-config byte-identity assertions (A-3).
#   2. The repository's OWN local code-gate over the source checkout — the
#      "local gate" §3.8 names (gofmt · vet · build · race), the single-source
#      code predicate every surface shares (code-gate-mirror.md, DINV-1). The
#      Go harness proves the config-driven DOC-gate + doctor over the relocated
#      corpus in-process; this step adds the local CODE-gate over the checkout,
#      so the full "full gate + local gate + doctor" trio §3.8 demands is
#      covered end to end (the code paths are system-internal, NG-1, so they are
#      not varied by the relocation — this step certifies the relocated-corpus
#      checkout still builds and tests green).
#   3. `pyxis doctor` over the source checkout — the read-only drift/health face
#      (source mode: the drift diff is skipped by name, every environment and
#      config-driven check runs).
#
# The existing-(non-empty)-repository install e2e is DEFERRED to phase-2 (§3.8,
# §3.4) — it needs the non-empty-tree converge this phase does not build.
#
# Hermetic: no network, no gh, no release fetch — the config-driven resolution
# surface is exercised entirely over the repo's own real content.
#
# The suite also runs inside the source repo's docs-gate `go test -race ./...`
# (ubuntu only); this job isolates the self-host harness and the local
# code-gate/doctor so a self-host regression is legible on its own line, and
# adds the darwin runner so the packaged path is certified on both platforms.
name: self-host
on:
  pull_request:
  push:
    branches: [main]
jobs:
  self-host:
    strategy:
      fail-fast: false
      matrix:
        os: [ubuntu-latest, macos-latest]
    runs-on: ${{ matrix.os }}
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - name: install Linux process boundary
        if: runner.os == 'Linux'
        run: |
          sudo apt-get update -qq
          sudo apt-get install -y bubblewrap apparmor
          sudo install -m 0644 .pyxis/security/bwrap.apparmor /etc/apparmor.d/pyxis-bwrap
          sudo apparmor_parser --replace /etc/apparmor.d/pyxis-bwrap
      # A-1/A-2/A-3: the self-host harness relocates the real corpus under a
      # non-default config and runs the full doc gate + doctor + branch/remote
      # resolution green; it also carries the coherence inspection and the
      # default-config byte-identity assertions. -race is the standing floor;
      # -v surfaces the per-assertion A-1/A-2/A-3 log lines.
      - name: self-host acceptance (race)
        run: go test -race -v ./internal/selfhost/...
      # The local code-gate §3.8 names — the single-source code predicate
      # (gofmt · vet · build · race) over the source checkout.
      - name: local code-gate
        run: go run ./cmd/code-gate
      # `pyxis doctor` — the read-only drift/health/advisory face over the
      # source checkout (source mode; mutation-free, network-free). Activate the
      # hook gate first (the source repo's documented one-time step, AGENTS.md
      # rule 8) so doctor's hooks check passes on a fresh checkout — exactly the
      # remedy doctor names for an unconfigured core.hooksPath.
      - name: pyxis doctor
        run: |
          git config core.hooksPath .githooks
          go run ./cmd/pyxis doctor
