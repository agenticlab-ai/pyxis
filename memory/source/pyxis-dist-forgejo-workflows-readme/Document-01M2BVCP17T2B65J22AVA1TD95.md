# Forgejo installed workflow

See [integration configuration](../../integrations.md) for connections, bindings,
and the GitHub-compatible defaults.

The code-host binding selects `.forgejo/workflows/docs-gate.yml`. Converge
renders this template using the verified release manifest's version and Linux
binary digests, the configured Git branch/remote, artifact source, and runner
label. The same release payload continues to support GitHub installations.

The repository must have an online Forgejo runner matching `integrations.ci.runner_label`
(default `pyxis-linux`). The runner image or project workflow must provide the
toolchains needed by the project's configured code checks. For example, a Go
project on the prepared Node image needs a Go setup step after checkout; use
`go-version-file: go.mod` and disable that action's cache when the runner cache
service is disabled. Pyxis downloads its own gate binaries separately.

Public artifact sources need no credential. Private
Forgejo artifact sources use the `PYXIS_ARTIFACT_TOKEN` Actions secret; a Basic
authentication distribution door uses `PYXIS_ARTIFACT_AUTH`. Configure only the
credential belonging to the selected artifact source. These secrets never enter
the generated workflow. The ordinary job token is not sent to another source.

The Pyxis artifact binding is independent of `bindings.releases.source` and
`bindings.releases.distribution`: those roles publish the project's releases.
`pyxis-release publish --target source|distribution`, `pins --target ...`,
`cut`, and `mirror` resolve those roles separately. Native Forgejo publication
verifies the complete local artifact set, uploads missing assets, verifies
existing bytes, and refuses clobbers. Mirroring also retargets the distribution
copy of `install.sh` to its provider, base URL, repository, and credential name.

Before a Go binary or repository configuration exists, a Forgejo copy of
`install.sh` accepts `PYXIS_RELEASE_PROVIDER=forgejo`,
`PYXIS_RELEASE_BASE_URL`, `PYXIS_RELEASE_REPOSITORY`, and optionally
`PYXIS_RELEASE_TOKEN_ENV` (default `PYXIS_FORGEJO_TOKEN`). Its embedded version
and platform SHA256 remain the trust anchor. A private copy of the script itself
must first be fetched through an authenticated connection to that same server.
The installer records the provider coordinates and token **name** in the
machine's `release.env`; later `init`, resolution, and `upgrade` use that source
without storing the credential value. Existing file/flat artifact-source
configuration retains precedence.

The repository bootstrap shim receives its source and verified platform pins
from the generated workflow. `PYXIS_ARTIFACT_TOKEN` authenticates that one
source, is passed to curl through stdin, and does not appear in command-line
arguments. A missing or mismatched checksum refuses installation.

The job runs the document and code gates for the exact checkout revision.
Forgejo 16 reports the pull-request context
`docs-gate / docs-gate (pull_request)`; branch protection must use that context.
Pyxis does not mirror repositories to run CI
on a different forge.
