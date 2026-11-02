# system-release — the ATOMIC system release (installer-v1 §3.1; plan
# installer/001 T11): one tag vX.Y.Z builds every engine binary for every
# supported platform from one commit, packs the payload archive and the
# offline deps bundle, derives the release manifest from the authored class
# policy, and publishes the whole set with checksums to this repo's
# Releases. All logic lives in Go (cmd/pyxis-release → internal/release);
# this workflow only sequences it, per the docgate-release.yml conventions
# it supersedes for the system tag (that per-tool workflow keeps its own
# docgate/v* tags until the T12 migration retires it — deliberately
# untouched here).
#
# The shipped binary set is DERIVED BY SCAN, never hand-kept (AP-27): the
# rule lives in ONE place — internal/release's ShippedTools: every cmd/*
# directory ships EXCEPT pyxis-release, the build tool itself. Growing the
# set is `mkdir cmd/<tool>`; this file never changes for it.
#
# Publish is upload-or-verify re-runnable (AP-22): a re-run of a partially
# published tag completes it or byte-verifies it — an existing asset is
# downloaded and hash-compared, NEVER clobbered; a divergent asset refuses.
#
# Release preconditions (installer-v1 §3.10) are asserted inside `pack`,
# fail-closed and named: no staged payload file may reference a retired
# per-tool resolver script, the staged operand workflow must carry zero
# source-toolchain steps, the deps bundle must cohere with the pinned
# lockfile, and every staged file must class under .pyxis/class-policy.yaml
# (DINV-6). Until the T12 source-repo migration lands, the retired-resolver
# precondition correctly refuses this tree — the first real tag REQUIRES
# T12.
#
# Toolchain notes: the ubuntu runner ships node/npm (the deps bundle
# regenerates via `npm ci` against the pinned lockfile — risk R5; pack
# refuses by name if npm is absent) and gh (publish's transport, GH_TOKEN
# from the workflow token; contents:write covers release assets).
#
# The mirror step drives the whole distribution fan-out through the one
# `mirror` verb (installer-v1 record O-2's kept-open evolution, activated
# 2026-07-23): the binaries-only dist repo (agenticlab-ai/pyxis-dist; the
# DIST_PUBLISH_TOKEN secret is a fine-grained PAT scoped Contents:write on
# pyxis-dist alone, so the workflow token never crosses repos) AND the
# password-doored R2 bucket behind dist.pyxis.agenticlab.ai (S3 creds from
# the R2_* secrets/vars; the R2 leg activates only when they are present).
# Same upload-or-verify grammar everywhere; the dist tag drops --verify-tag
# (a mirror has no source tag — provenance rides the manifest's source
# commit, and every consumer re-verifies against the config-pinned
# manifest hash, DINV-2). mirror also refreshes each channel's install.sh
# front door and REFUSES when the checkout's install.sh pins a different
# tag — a release whose pins lag its artifacts would strand every dist
# consumer, so that is a release failure by design.
# WHY THE MIRROR IS A SEPARATE, MANUALLY DISPATCHED JOB — and cannot ride
# the tag push: mirror refuses when the checkout's install.sh pins a
# different tag, and at tag time it ALWAYS does. install.sh pins the release
# manifest's hash; the manifest names the source commit; install.sh is
# committed — so the pins for a tag's own artifacts are unknowable until
# after that tag exists, and the pin commit necessarily lands afterwards.
# The tag job therefore ends at "published + pins regenerated" and prints
# the remainder; a human merges the pin PR (the release's one reviewed
# decision) and then dispatches the mirror job below. Both jobs run the same
# `cut` verb an operator runs by hand — one runbook, not two.
name: system-release
on:
  push:
    tags: ['v*']
  workflow_dispatch:
    inputs:
      tag:
        description: 'release tag to mirror (after its pin PR has merged)'
        required: true
jobs:
  # The tag half: build → pack → verify → publish → regenerate the pins.
  release:
    if: github.event_name == 'push'
    runs-on: ubuntu-latest
    permissions: { contents: write }
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version-file: go.mod }
      - name: cut the release (build, pack, verify, publish, repin)
        run: go run ./cmd/pyxis-release cut --version "$GITHUB_REF_NAME" --out dist
        env: { GH_TOKEN: "${{ github.token }}" }
      - name: surface the regenerated pins for the pin PR
        run: |
          go run ./cmd/pyxis-release pins --version "$GITHUB_REF_NAME" --out dist
          echo '### install.sh pins for '"$GITHUB_REF_NAME" >> "$GITHUB_STEP_SUMMARY"
          echo 'Commit this block on main, then dispatch this workflow with tag='"$GITHUB_REF_NAME" >> "$GITHUB_STEP_SUMMARY"
          echo '```sh' >> "$GITHUB_STEP_SUMMARY"
          sed -n '/^# BEGIN PINS/,/^# END PINS/p' install.sh >> "$GITHUB_STEP_SUMMARY"
          echo '```' >> "$GITHUB_STEP_SUMMARY"
      - uses: actions/upload-artifact@v4
        with: { name: install-sh-pins, path: install.sh }

  # The distribution half: dispatched once the pin PR is on main, so the
  # checkout carries the anchor every channel is about to serve.
  mirror:
    if: github.event_name == 'workflow_dispatch'
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with: { ref: main }
      - uses: actions/setup-go@v5
        with: { go-version-file: go.mod }
      - name: mirror to the distribution channels (pyxis-dist + R2)
        run: go run ./cmd/pyxis-release mirror --tag "${{ inputs.tag }}"
        env:
          GH_TOKEN: "${{ secrets.DIST_PUBLISH_TOKEN }}"
          R2_ENDPOINT: "${{ vars.R2_ENDPOINT }}"
          R2_BUCKET: "${{ vars.R2_BUCKET }}"
          R2_ACCESS_KEY_ID: "${{ secrets.R2_ACCESS_KEY_ID }}"
          R2_SECRET_ACCESS_KEY: "${{ secrets.R2_SECRET_ACCESS_KEY }}"
