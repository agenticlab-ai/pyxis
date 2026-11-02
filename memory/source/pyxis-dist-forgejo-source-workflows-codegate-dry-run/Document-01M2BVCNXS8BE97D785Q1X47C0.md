# codegate advisory dry-run (codegate-v1 §3.11, A-9; prd:implementation-
# verification/FR-15, NFR-3): proves the harness produces a rung-for-rung
# identical verdict whether it runs headlessly here or by a local operator
# run on the same fixture content (EVT-5). Explicitly advisory:
# continue-on-error stays set at the job level, and this workflow is NEVER
# registered as a required check — required-check registration and any
# enforcement semantics stay parked (basis WR-5).
name: codegate-dry-run
on:
  pull_request:
    paths:
      - 'cmd/codegate/**'
      - 'internal/verify/**'
      - '.forgejo/workflows/codegate-dry-run.yml'
  push:
    branches: [{{branch}}]
    paths:
      - 'cmd/codegate/**'
      - 'internal/verify/**'
  workflow_dispatch:
jobs:
  dry-run:
    runs-on: {{runner_label}}
    continue-on-error: true
    steps:
      - uses: https://data.forgejo.org/actions/checkout@v4
      - uses: https://github.com/actions/setup-go@v5
        with: { go-version-file: go.mod }
      - name: headless parity run (EVT-5)
        run: go test -tags dryrunparity -run TestDryRunParity_MatchesLocalReference -v ./cmd/codegate/...
