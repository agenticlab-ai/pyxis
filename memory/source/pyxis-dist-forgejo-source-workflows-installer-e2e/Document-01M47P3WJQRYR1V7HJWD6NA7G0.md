# Optional manual acceptance on both Linux and Darwin. The required shared
# test-coverage check runs these same suites on every PR; these jobs add an
# operator-selected Darwin runner without introducing separate merge gates.
# Set PYXIS_DARWIN_RUNNER_LABEL before dispatching.
name: installer-e2e
on:
  workflow_dispatch:
jobs:
  installer-e2e-linux:
    runs-on: {{runner_label}}
    steps:
      - uses: https://data.forgejo.org/actions/checkout@v4
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
      # The harness compiles pyxis + docgate itself (TestMain) and drives them
      # as subprocesses against a file:// fixture release — hermetic: no
      # network, no gh, carina/local tracker only (the GitHub-mode leg injects a
      # gh transcript fake through the wire.Runner seam).
      # -v surfaces the T14 budget recordings in the log.
      - name: installer e2e matrix
        run: go test -v ./internal/installer/e2e/...
  installer-e2e-darwin:
    runs-on: ${{ vars.PYXIS_DARWIN_RUNNER_LABEL }}
    steps:
      - uses: https://data.forgejo.org/actions/checkout@v4
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
      # The harness compiles pyxis + docgate itself (TestMain) and drives them
      # as subprocesses against a file:// fixture release — hermetic: no
      # network, no gh, carina/local tracker only (the GitHub-mode leg injects a
      # gh transcript fake through the wire.Runner seam).
      # -v surfaces the T14 budget recordings in the log.
      - name: installer e2e matrix
        run: go test -v ./internal/installer/e2e/...
