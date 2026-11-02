# Forgejo source workflows

These eight active templates cover the source repository's GitHub workflow capabilities:
the required code/docs gate, codegate dry run, fixture
catch suite, retained docgate/codegate tag release channels, self-host acceptance,
the installer Linux/Darwin matrix, and system release publication/mirroring.
The historical `memory-reconciliation.yml` source remains in Git for its
captured memory lineage, but `pyxis-release workflows` excludes it from the
eight generated workflows. Memory is already verified by `docs-gate`.
If an earlier run already wrote `.forgejo/workflows/memory-reconciliation.yml`,
remove that generated file when adopting this change; the renderer preserves
existing workflow files.

After selecting a Forgejo `integrations.bindings.code_host`, an explicit
`integrations.bindings.releases.source`, and CI mode `actions` or `both`, run:

```console
go run ./cmd/pyxis-release workflows
```

This writes `.forgejo/workflows/*.yml` from the templates. It preserves existing
workflow files and refuses different content so the owner can review a
replacement. It never changes the source repository's host binding. Ordinary
installed repositories use the separate, checksum-pinned binary workflow under
`forgejo-workflows/`; they do not need these source build jobs.

The selected runner must support the source suite's toolchains and Linux process
confinement requirements. The source gate retains those checks and does not
grant Docker privileges or mount a Docker socket into jobs. A minimal Node
runner suitable for an installed repository is not sufficient for source tests
that require host AppArmor administration. Configure an appropriate source
runner label before enabling this source suite.

The source cache steps require the runner's cache service to be enabled and
reachable from its job environment; `setup-go` also uses that service by
default. Keep compression tools consistent across runners that share caches.
See [Forgejo's cache requirements](https://forgejo.org/docs/v16.0/user/actions/advanced-features/#cache).

The release artifact step intentionally uses `upload-artifact@v3`. Forgejo 16
supports v3 or Forgejo's patched v4 artifact actions; upstream GitHub v4 cannot
be substituted directly. See [Forgejo's artifact compatibility guidance](https://forgejo.org/docs/v16.0/user/actions/advanced-features/#artifacts).

Self-host and installer acceptance are manually dispatched supplements to the
required shared race suite, which already runs both test packages on each PR.
The supplements retain both operating systems as explicit jobs. Register a
Darwin runner separately and set the Actions repository variable
`PYXIS_DARWIN_RUNNER_LABEL` to its label; the configured `ci.runner_label` is the
Linux label. The Linux Docker runner does not supply Darwin coverage.

Store the source publishing credential as the Actions secret
`PYXIS_RELEASE_TOKEN`; the renderer maps it to the selected release connection's
`token_env`. Release targets remain independent of the code host. Forgejo
publishing uses the native API. A GitHub release target retains its `gh`
transport and therefore needs that CLI on the selected source runner.

The system release workflow saves regenerated `install.sh` pins as an Actions
artifact for a reviewed pin commit. Dispatch the mirror job after that commit
has merged. Distribution defaults to R2 when `releases.distribution` is absent;
provide `R2_ENDPOINT`, `R2_BUCKET`, `R2_ACCESS_KEY_ID`, and `R2_SECRET_ACCESS_KEY`.
An explicit `releases.distribution` selects a repository mirror. Put its
credential in `PYXIS_DISTRIBUTION_RELEASE_TOKEN` when that connection uses a
different `token_env`. If both targets intentionally share the same `token_env`,
`PYXIS_RELEASE_TOKEN` supplies both; use separate connection credential references
when the credentials differ. R2 can also accompany an explicit repository
mirror; any supplied R2 configuration must be complete. R2-only mirrors require
their source binding at execution time. Repository mirrors require both release
bindings: removing a required binding later refuses publication.
Release credential references must not use the workflow's reserved `R2_*` keys
or `PYXIS_RELEASE_TAG`.

Public action dependencies use explicit upstream URLs. Workflow syntax and
contexts follow the [Forgejo Actions reference](https://forgejo.org/docs/latest/user/actions/reference/).
