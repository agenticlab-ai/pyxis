# READY-TO-LAND ARTIFACT — copy VERBATIM to .github/workflows/merge-lane.yml.
#
# It is parked here because landing a workflow file requires a `workflow`-scope
# push, which the repo's machine identity (mdarabiassistant) deliberately lacks
# — landing it is an operator-ownable act (see issue #514 and
# docs/runbooks/merge-lane-activation.md). Until it lands, the same evaluation
# runs host-side via `lane sweep` (the design's stated sequential fallback).
#
# The merge lane's evaluation workflow (merge-lane-v1; plan T4's contracted
# wiring; #514). Classifies every proposal and records the lane verdict on it.
#
# TRUSTED CONTEXT (DINV-9): the event is `pull_request_target`, so this
# workflow definition, the lane binary, and the risk policy all come from the
# MAINLINE (the base repository's default branch) — never from the proposal's
# tree. The proposal contributes ONLY its (base, head) diff, fetched as git
# objects and consumed as data. No step checks out or executes proposal
# content; do not add one.
#
# SHADOW BY CONSTRUCTION: no --machine identity is passed, so the acting
# adapter holds no Machine and cannot act (DINV-5); independently, the acting
# state cannot resolve `enforcing` until the operator's recorded activation
# acts land (FR-9 — see docs/runbooks/merge-lane-activation.md). This job's
# only write is the append-only verdict-block comment the Recorder maintains.
name: merge-lane
on:
  pull_request_target:
    types: [opened, synchronize, reopened]

permissions:
  contents: read
  pull-requests: write
  checks: read

concurrency:
  group: merge-lane-${{ github.event.pull_request.number }}
  cancel-in-progress: true

jobs:
  evaluate:
    runs-on: ubuntu-latest
    steps:
      - name: Check out the trusted mainline
        uses: actions/checkout@v4
        with:
          # The BASE repository's default branch — the trusted context the
          # binary and policy load from (DINV-9). Never the PR's ref.
          ref: main
          fetch-depth: 0

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
          cache: true

      - name: Fetch the proposal's revisions as data
        run: |
          git fetch --no-tags origin "${{ github.event.pull_request.base.sha }}" "${{ github.event.pull_request.head.sha }}"

      - name: Evaluate and record the lane verdict
        env:
          GH_TOKEN: ${{ github.token }}
        run: |
          go run ./cmd/lane handle \
            --repo . \
            --gh-repo "${{ github.repository }}" \
            --pr "${{ github.event.pull_request.number }}" \
            --base "${{ github.event.pull_request.base.sha }}" \
            --head "${{ github.event.pull_request.head.sha }}" \
            --checks "gh-checks@${{ github.event.pull_request.head.sha }}"
