name: codegate-release
on:
  push:
    tags: ['codegate/v*']
jobs:
  release:
    runs-on: {{runner_label}}
    env:
      {{release_token_env}}: ${{ secrets.PYXIS_RELEASE_TOKEN }}
      PYXIS_RELEASE_TAG: ${{ forgejo.ref_name }}
    steps:
      - uses: https://data.forgejo.org/actions/checkout@v4
      - uses: https://github.com/actions/setup-go@v5
        with:
          go-version-file: go.mod
      - name: build and publish verified legacy release
        run: go run ./cmd/pyxis-release legacy --require-bindings --tool codegate --tag "$PYXIS_RELEASE_TAG" --out dist
