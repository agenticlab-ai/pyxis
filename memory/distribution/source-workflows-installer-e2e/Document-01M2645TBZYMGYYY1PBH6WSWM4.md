# installer-e2e — the installer's scratch-repo end-to-end matrix
# (installer-v1 §3.11 "the e2e matrix — the deployment context (AP-14)"; plan
# installer/001 T13 + T14). The harness (internal/installer/e2e) builds the real
# pyxis + docgate binaries, then drives flag-driven installs from a file://
# fixture release through the shipped binary on scratch repos.
#
# The LOCAL half (T13): the offline combo (carina + local CI, network-denied
# with an egress scan), the drift matrix, kill/re-run, fresh-clone self-heal,
# the misparse suite, and the removal test.
#
# The MATRIX TAIL (T14, now landed — same package, same job): the GitHub-mode
# wiring combos (through an injected wire.Runner transcript fake — no live gh,
# no network), the source-vs-installed parity suite (the NR-2 reporoot-fix
# proof), the supply-chain tamper drills (binary · payload · deps bundle ·
# bootstrap script · the declared-operation audit), and the two recorded time
# budgets (per-invocation overhead ≤100ms p95 · install ≤5min — RECORDED to the
# log, asserted against generous ceilings; the real budget is enforced here on
# the reference runners). `-v` surfaces the recorded p95 / wall-time per runner.
#
# The matrix is the platform axis §3.11 names: linux AND darwin runners, so the
# packaged path is certified on both — no OS-specific assumption beyond what
# resolve.Platform() gives (T13 AC-3: fresh-clone, misparse, and removal-test
# legs pass on both; T14: the tamper/parity/budget legs record per runner). Live
# GitHub Actions is the deferred remote leg (installer-v1 §3.11); this matrix
# proves everything a hermetic harness can.
#
# The suite also runs inside the source repo's docs-gate `go test -race ./...`
# (ubuntu only); this job adds the darwin runner and isolates the e2e package
# so a matrix-leg failure is legible on its own line.
name: installer-e2e
on:
  pull_request:
  push:
    branches: [main]
jobs:
  e2e:
    strategy:
      fail-fast: false
      matrix:
        os: [ubuntu-latest, macos-latest]
    runs-on: ${{ matrix.os }}
    steps:
      - uses: actions/checkout@v4
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
      # The harness compiles pyxis + docgate itself (TestMain) and drives them
      # as subprocesses against a file:// fixture release — hermetic: no
      # network, no gh, carina/local tracker only (the GitHub-mode leg injects a
      # gh transcript fake through the wire.Runner seam). -race is the standing
      # floor; -v surfaces the T14 budget recordings in the log.
      - name: installer e2e matrix (race)
        run: go test -race -v ./internal/installer/e2e/...
