# docs-gate — the required CI context. It runs BOTH gates over a change: the
# doc-gate (docgate — the PRD ledger contract, docs/prds/docgate/ FR-2..FR-8;
# docs/tech-design/docgate-v1.md §3.10) and the code-gate (the single-source
# code checks — docs/tech-design/code-gate-mirror.md §3.6, DINV-1). Desk parity:
# .githooks/pre-push runs the same two predicates (DINV-4).
name: docs-gate
on:
  pull_request:
  push:
    branches: [{{branch}}]
jobs:
  docs-gate:
    runs-on: {{runner_label}}
    env:
      PYXIS_REQUIRE_PROCESS_CONFINEMENT: "1"
      PYXIS_CI_EVENT: "${{ forgejo.event_name }}"
      PYXIS_CI_BASE: "${{ forgejo.event.pull_request.base.sha || forgejo.event.before }}"
    steps:
      - uses: https://data.forgejo.org/actions/checkout@v4
        with:
          fetch-depth: 0
      - uses: https://github.com/actions/setup-go@v5
        with:
          go-version-file: go.mod
          # Cache explicitly below, including the external pinned linter.
          cache: false
      # The compile cache. Every check below shells into the Go toolchain, and a
      # cold GOCACHE makes each one rebuild the world — including the
      # race-instrumented standard library, which `go test -race` cannot share
      # with the plain artifacts `vet`/`build` produce. Restore-only here; the
      # save is main-only (see the end of the job) so PRs read a shared warm
      # cache without 60 runs/day of per-PR entries evicting it. Measured cold,
      # this tree costs ≈100 s of CPU to compile and packs to ≈87 MB.
      - name: restore Go and lint caches
        uses: https://github.com/actions/cache/restore@v4
        with:
          path: |
            ~/.cache/go-build
            ~/go/pkg/mod
            ~/.cache/golangci-lint
          # forgejo.sha makes the key unique per commit (Actions cache entries are
          # immutable once written, so a fixed key would freeze at its first
          # write and never pick up new artifacts). The restore-keys prefixes are
          # what actually hit: nearest previous main build, toolchain-scoped.
          key: go-build-${{ runner.os }}-${{ hashFiles('go.mod', '.pyxis/config.yaml', '.golangci.yml') }}-${{ forgejo.sha }}
          restore-keys: |
            go-build-${{ runner.os }}-${{ hashFiles('go.mod', '.pyxis/config.yaml', '.golangci.yml') }}-
            go-build-${{ runner.os }}-
      - name: install Linux test dependencies
        run: |
          sudo apt-get update -qq
          sudo apt-get install -y bubblewrap apparmor zsh
          sudo install -m 0644 .pyxis/security/bwrap.apparmor /etc/apparmor.d/pyxis-bwrap
          sudo apparmor_parser --replace /etc/apparmor.d/pyxis-bwrap
      - name: code-gate (all shared checks, including CI parity and memory)
        run: .pyxis/scripts/code-gate-resolve.sh run --no-cache
      - name: docgate gate
        run: .pyxis/scripts/docgate-resolve.sh gate --merge-base "$PYXIS_CI_BASE" --defects-json -
      - name: coverage (report only)
        if: always()
        continue-on-error: true
        run: .pyxis/scripts/docgate-resolve.sh coverage >> "$GITHUB_STEP_SUMMARY"
      # Saved from main only. A cache written on the default branch is readable
      # by every branch and PR, while one written on a PR is visible only to that
      # PR — so main-only is what actually warms the fleet, and it keeps ~60
      # runs/day of PR entries from churning the repo's 10 GB cache budget and
      # evicting this one. always() so a red run still publishes its compile
      # artifacts: they are valid regardless of whether a check failed.
      - name: save Go and lint caches
        if: always() && forgejo.event_name == 'push' && forgejo.ref == 'refs/heads/{{branch}}'
        uses: https://github.com/actions/cache/save@v4
        with:
          path: |
            ~/.cache/go-build
            ~/go/pkg/mod
            ~/.cache/golangci-lint
          key: go-build-${{ runner.os }}-${{ hashFiles('go.mod', '.pyxis/config.yaml', '.golangci.yml') }}-${{ forgejo.sha }}
