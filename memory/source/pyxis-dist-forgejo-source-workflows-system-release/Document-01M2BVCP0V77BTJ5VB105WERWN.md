# Source build/publication and independent distribution mirror. All release
# operations use the selected bindings through the typed pyxis-release API.
name: system-release
on:
  push:
    tags: ['v*']
  workflow_dispatch:
    inputs:
      tag:
        description: 'release tag to mirror after its pin commit has merged'
        required: true
jobs:
  release:
    if: forgejo.event_name == 'push'
    runs-on: {{runner_label}}
    env:
      {{release_token_env}}: ${{ secrets.PYXIS_RELEASE_TOKEN }}
      PYXIS_RELEASE_TAG: ${{ forgejo.ref_name }}
    steps:
      - uses: https://data.forgejo.org/actions/checkout@v4
      - uses: https://github.com/actions/setup-go@v5
        with:
          go-version-file: go.mod
      - name: cut release and regenerate installer pins
        run: go run ./cmd/pyxis-release cut --require-bindings --version "$PYXIS_RELEASE_TAG" --out dist
      - name: retain installer pins for a reviewed pin commit
        uses: https://data.forgejo.org/actions/upload-artifact@v3
        with:
          name: install-sh-pins
          path: install.sh
  # Dispatch after the pin commit has merged. Distribution defaults to R2;
  # an explicit releases.distribution binding selects a repository mirror.
  mirror:
    if: forgejo.event_name == 'workflow_dispatch'
    runs-on: {{runner_label}}
    env:
      {{release_token_env}}: ${{ secrets.PYXIS_RELEASE_TOKEN }}
      {{distribution_token_env}}: ${{ secrets.PYXIS_DISTRIBUTION_RELEASE_TOKEN }}
      PYXIS_RELEASE_TAG: ${{ inputs.tag }}
      R2_ENDPOINT: ${{ vars.R2_ENDPOINT }}
      R2_BUCKET: ${{ vars.R2_BUCKET }}
      R2_ACCESS_KEY_ID: ${{ secrets.R2_ACCESS_KEY_ID }}
      R2_SECRET_ACCESS_KEY: ${{ secrets.R2_SECRET_ACCESS_KEY }}
    steps:
      - uses: https://data.forgejo.org/actions/checkout@v4
        with:
          ref: '{{branch}}'
      - uses: https://github.com/actions/setup-go@v5
        with:
          go-version-file: go.mod
      - name: mirror verified release and installer
        run: go run ./cmd/pyxis-release mirror {{mirror_binding_flags}} --tag "$PYXIS_RELEASE_TAG"
