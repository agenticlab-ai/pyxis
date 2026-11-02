# Phase-1 cheap checks run first. Race and coverage start only after that
# barrier, matching local `code-gate run`. The lightweight docs-gate job
# preserves the existing required status and refuses success unless every
# check job succeeded. ci-parity enforces the shared grouping and the
# cheap-then-expensive graph.
name: docs-gate
on:
  pull_request:
  push:
    branches: [{{branch}}]
jobs:
  cheap:
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
          cache: false
      - name: restore Go and lint caches
        uses: https://github.com/actions/cache/restore@v4
        with:
          path: |
            ~/.cache/go-build
            ~/go/pkg/mod
            ~/.cache/golangci-lint
          key: go-build-cheap-${{ runner.os }}-${{ hashFiles('go.mod', '.pyxis/config.yaml', '.golangci.yml') }}-${{ forgejo.sha }}
          restore-keys: |
            go-build-cheap-${{ runner.os }}-${{ hashFiles('go.mod', '.pyxis/config.yaml', '.golangci.yml') }}-
            go-build-quality-${{ runner.os }}-${{ hashFiles('go.mod', '.pyxis/config.yaml', '.golangci.yml') }}-
            go-build-${{ runner.os }}-${{ hashFiles('go.mod', '.pyxis/config.yaml', '.golangci.yml') }}-
            go-build-cheap-${{ runner.os }}-
            go-build-quality-${{ runner.os }}-
            go-build-${{ runner.os }}-
      - name: install Linux test dependencies
        run: |
          sudo apt-get update -qq
          sudo apt-get install -y bubblewrap apparmor zsh
          sudo install -m 0644 .pyxis/security/bwrap.apparmor /etc/apparmor.d/pyxis-bwrap
          sudo apparmor_parser --replace /etc/apparmor.d/pyxis-bwrap
      - name: code-gate (phase 1)
        run: .pyxis/scripts/code-gate-resolve.sh run --no-cache --cheap
      - name: save Go and lint caches
        if: always() && forgejo.event_name == 'push' && forgejo.ref == 'refs/heads/{{branch}}'
        uses: https://github.com/actions/cache/save@v4
        with:
          path: |
            ~/.cache/go-build
            ~/go/pkg/mod
            ~/.cache/golangci-lint
          key: go-build-cheap-${{ runner.os }}-${{ hashFiles('go.mod', '.pyxis/config.yaml', '.golangci.yml') }}-${{ forgejo.sha }}
  quality:
    needs: [cheap]
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
          cache: false
      - name: restore Go and lint caches
        uses: https://github.com/actions/cache/restore@v4
        with:
          path: |
            ~/.cache/go-build
            ~/go/pkg/mod
            ~/.cache/golangci-lint
          key: go-build-quality-${{ runner.os }}-${{ hashFiles('go.mod', '.pyxis/config.yaml', '.golangci.yml') }}-${{ forgejo.sha }}
          # Fall back to the former shared cache while these job caches warm.
          restore-keys: |
            go-build-quality-${{ runner.os }}-${{ hashFiles('go.mod', '.pyxis/config.yaml', '.golangci.yml') }}-
            go-build-${{ runner.os }}-${{ hashFiles('go.mod', '.pyxis/config.yaml', '.golangci.yml') }}-
            go-build-quality-${{ runner.os }}-
            go-build-${{ runner.os }}-
      - name: install Linux test dependencies
        run: |
          sudo apt-get update -qq
          sudo apt-get install -y bubblewrap apparmor zsh
          sudo install -m 0644 .pyxis/security/bwrap.apparmor /etc/apparmor.d/pyxis-bwrap
          sudo apparmor_parser --replace /etc/apparmor.d/pyxis-bwrap
      - name: code-gate (coverage)
        run: .pyxis/scripts/code-gate-resolve.sh run --no-cache --group default --phase 2
      - name: docgate gate
        run: .pyxis/scripts/docgate-resolve.sh gate --merge-base "$PYXIS_CI_BASE" --defects-json -
      - name: coverage (report only)
        if: always()
        continue-on-error: true
        run: .pyxis/scripts/docgate-resolve.sh coverage >> "$GITHUB_STEP_SUMMARY"
      # Save independently on main so both jobs can publish their cache.
      - name: save Go and lint caches
        if: always() && forgejo.event_name == 'push' && forgejo.ref == 'refs/heads/{{branch}}'
        uses: https://github.com/actions/cache/save@v4
        with:
          path: |
            ~/.cache/go-build
            ~/go/pkg/mod
            ~/.cache/golangci-lint
          key: go-build-quality-${{ runner.os }}-${{ hashFiles('go.mod', '.pyxis/config.yaml', '.golangci.yml') }}-${{ forgejo.sha }}
  tests:
    needs: [cheap]
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
          cache: false
      - name: restore Go and lint caches
        uses: https://github.com/actions/cache/restore@v4
        with:
          path: |
            ~/.cache/go-build
            ~/go/pkg/mod
            ~/.cache/golangci-lint
          key: go-build-tests-${{ runner.os }}-${{ hashFiles('go.mod', '.pyxis/config.yaml', '.golangci.yml') }}-${{ forgejo.sha }}
          # Fall back to the former shared cache while these job caches warm.
          restore-keys: |
            go-build-tests-${{ runner.os }}-${{ hashFiles('go.mod', '.pyxis/config.yaml', '.golangci.yml') }}-
            go-build-${{ runner.os }}-${{ hashFiles('go.mod', '.pyxis/config.yaml', '.golangci.yml') }}-
            go-build-tests-${{ runner.os }}-
            go-build-${{ runner.os }}-
      - name: install Linux test dependencies
        run: |
          sudo apt-get update -qq
          sudo apt-get install -y bubblewrap apparmor zsh
          sudo install -m 0644 .pyxis/security/bwrap.apparmor /etc/apparmor.d/pyxis-bwrap
          sudo apparmor_parser --replace /etc/apparmor.d/pyxis-bwrap
      - name: code-gate (race)
        run: .pyxis/scripts/code-gate-resolve.sh run --no-cache --group tests --phase 2
      # Save independently on main so both jobs can publish their cache.
      - name: save Go and lint caches
        if: always() && forgejo.event_name == 'push' && forgejo.ref == 'refs/heads/{{branch}}'
        uses: https://github.com/actions/cache/save@v4
        with:
          path: |
            ~/.cache/go-build
            ~/go/pkg/mod
            ~/.cache/golangci-lint
          key: go-build-tests-${{ runner.os }}-${{ hashFiles('go.mod', '.pyxis/config.yaml', '.golangci.yml') }}-${{ forgejo.sha }}
  docs-gate:
    needs: [cheap, quality, tests]
    if: always()
    runs-on: {{runner_label}}
    timeout-minutes: 1
    env:
      PYXIS_CI_RESULTS: "${{ join(needs.*.result, ' ') }}"
      PYXIS_CI_EXPECTED: success success success
    steps:
      - name: require both check jobs to pass
        run: test "$PYXIS_CI_RESULTS" = "$PYXIS_CI_EXPECTED"
