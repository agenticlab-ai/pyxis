# Daily memory debt ratchet. This is deliberately separate from strict feature
# closeout: the repository-wide legacy baseline permits only already-recorded
# debt, while `memory closeout --feature ... --check` continues to require that
# feature's exact fixed point before any source retirement.
name: memory-reconciliation

on:
  pull_request:
  push:
    branches: [{{branch}}]
  schedule:
    - cron: '23 09 * * *'

permissions:
  contents: read

jobs:
  debt-ratchet:
    runs-on: {{runner_label}}
    timeout-minutes: 10
    env:
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
      # The initial v1 baseline was squash-merged with intervening main changes.
      # Fetch its original immutable snapshot only while that old main is the
      # trusted base. Replay still has to prove the exact trusted file and hashes.
      - name: fetch original baseline inputs for the initial v1 transition
        if: forgejo.event.pull_request.base.sha == '10561e22b0ad2ebceb84ac8c5b8913588b0096cc' || forgejo.event.before == '10561e22b0ad2ebceb84ac8c5b8913588b0096cc'
        run: git fetch --no-tags origin 3ed3ffb1cf6e4a593bf9990178bb10afe88dce02
      - name: shared memory integrity and debt ratchet (offline)
        run: .pyxis/scripts/code-gate-resolve.sh run --no-cache --check memory
