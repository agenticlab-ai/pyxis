# Installed-repository gate. Pyxis renders the bootstrap pins from the verified
# release manifest; this workflow consumes binaries and never builds the engine.
name: docs-gate
on:
  pull_request:
  push:
    branches: [{{branch}}]
jobs:
  docs-gate:
    runs-on: {{runner_label}}
    env:
      PYXIS_VERSION: '{{version}}'
      PYXIS_SHA256_LINUX_ARM64: '{{linux_arm_checksum}}'
      PYXIS_SHA256_LINUX_AMD64: '{{linux_amd_checksum}}'
      PYXIS_ARTIFACT_SOURCE: '{{artifact_source}}'
      PYXIS_ARTIFACT_TOKEN: ${{ secrets.PYXIS_ARTIFACT_TOKEN }}
      PYXIS_ARTIFACT_AUTH: ${{ secrets.PYXIS_ARTIFACT_AUTH }}
      {{artifact_token_env}}: ${{ secrets.PYXIS_ARTIFACT_TOKEN }}
    steps:
      - uses: https://data.forgejo.org/actions/checkout@v4
        with:
          fetch-depth: 0
      - name: bootstrap pyxis
        run: bash .pyxis/scripts/pyxis-bootstrap.sh
      - name: docgate gate
        run: |
          git fetch {{remote}} {{branch}}
          .pyxis/bin/pyxis exec docgate -- gate --merge-base {{remote}}/{{branch}} --defects-json -
      - name: code gate
        run: .pyxis/bin/pyxis exec code-gate -- run
      - name: coverage (report only)
        if: always()
        continue-on-error: true
        run: .pyxis/bin/pyxis exec docgate -- coverage
