# codegate multi-stack fixture matrix + permanent catch suite (design
# codegate-v1 §3.11, A-1/A-2/A-3; plan implementation-verification/001 T10).
# Keeps two properties tested, not claimed: technology-independence — the
# identical, stack-blind harness binds and adjudicates the go, python, and node
# fixtures — and the anti-gaming bar — every adversarial seed family (a
# tautological test, a silenced test via deletion/skip-marker/assertion-removal,
# and a config-pin downgrade) stays blocked. Each seed emits an EVT-4 record in
# this job's log.
#
# This job sets up all three toolchains explicitly so the python/node fixtures
# always run here. The same tests also run under docs-gate's `go test -race`
# (ubuntu-latest ships python3 + node), so the gate holds there regardless; a
# developer machine missing a toolchain skips that stack rather than failing.
# To gate merges on THIS job specifically, mark `catch-suite` a required status
# check in branch protection — a workflow cannot make itself required.
name: codegate-catch-suite
on:
  pull_request:
    paths:
      - 'cmd/codegate/**'
      - 'internal/verify/**'
      - '.forgejo/workflows/codegate-catch-suite.yml'
  push:
    branches: [{{branch}}]
    paths:
      - 'cmd/codegate/**'
      - 'internal/verify/**'
  workflow_dispatch:
jobs:
  catch-suite:
    runs-on: {{runner_label}}
    env:
      PYXIS_CI_EVENT: "${{ forgejo.event_name }}"
      PYXIS_CI_BASE: "${{ forgejo.event.pull_request.base.sha || forgejo.event.before }}"
    steps:
      - uses: https://data.forgejo.org/actions/checkout@v4
        with:
          fetch-depth: 0 # full history: the matrix builds real base→head commits
      - uses: https://github.com/actions/setup-go@v5
        with:
          go-version-file: go.mod
      - uses: https://github.com/actions/setup-python@v5
        with:
          python-version: '3.x'
      - uses: https://github.com/actions/setup-node@v4
        with:
          node-version: '20'
      - name: fixture matrix + catch suite (EVT-4)
        run: .pyxis/scripts/code-gate-resolve.sh run --no-cache --check catch-suite
