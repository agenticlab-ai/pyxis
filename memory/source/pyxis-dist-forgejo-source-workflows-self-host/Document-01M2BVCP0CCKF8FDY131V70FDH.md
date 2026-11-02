# Optional manual acceptance on both Linux and Darwin. The required shared
# test-race check runs these same suites on every PR; these jobs add an
# operator-selected Darwin runner without introducing separate merge gates.
# Set PYXIS_DARWIN_RUNNER_LABEL before dispatching.
name: self-host
on:
  workflow_dispatch:
jobs:
  self-host-linux:
    runs-on: {{runner_label}}
    steps:
      - uses: https://data.forgejo.org/actions/checkout@v4
        with:
          fetch-depth: 0
      - uses: https://github.com/actions/setup-go@v5
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
          go run ./cmd/hooks-setup
          go run ./cmd/pyxis doctor
  self-host-darwin:
    runs-on: ${{ vars.PYXIS_DARWIN_RUNNER_LABEL }}
    steps:
      - uses: https://data.forgejo.org/actions/checkout@v4
        with:
          fetch-depth: 0
      - uses: https://github.com/actions/setup-go@v5
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
          go run ./cmd/hooks-setup
          go run ./cmd/pyxis doctor
