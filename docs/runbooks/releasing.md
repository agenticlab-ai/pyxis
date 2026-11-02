# Releasing Pyxis

Releases live on this repository's
[GitHub Releases](https://github.com/agenticlab-ai/pyxis/releases). One tag,
`vX.Y.Z`, carries every engine binary (darwin and linux, arm64 and amd64), the
payload archive, the release manifest, and `SHA256SUMS`. New users download them
anonymously over HTTPS.

## Cut a release

1. **Push the tag** from the commit you want to release:

   ```sh
   git tag vX.Y.Z <commit>
   git push origin vX.Y.Z
   ```

   The `system-release` workflow builds, packs, and verifies the artifact set,
   publishes it, and regenerates the pins in `install.sh`. It shows the new pin
   block in the run summary and uploads the file as an artifact.

2. **Merge the pins.** Open a pull request that changes `install.sh` to the
   regenerated `BEGIN PINS` block. Until it merges, installs and upgrades stay on
   the previous release.

3. **Check it.** Run the install command from the [README](../../README.md), then
   `pyxis version`.

The pins can't ride the tag. They name the manifest's hash, the manifest names the
source commit, and `install.sh` is committed, so the pin commit always lands after
the tag.

## Run it by hand

The workflow calls the same verb a maintainer can run, with write access to the
repository:

```sh
go run ./cmd/pyxis-release cut --version vX.Y.Z --push-tag
```

`cut` builds, packs, verifies, checks that the tag resolves to the packed commit,
publishes, and rewrites the pins, then stops so you can open the pin pull request.
Each stage is upload-or-verify: re-running completes a partial publish or
byte-checks a finished one, and a divergent asset refuses.

The individual verbs (`build`, `pack`, `verify`, `publish`, `pins`) are there for
partial re-runs. `go run ./cmd/pyxis-release` lists them. One worth knowing:

```sh
# Regenerate pins from a published release; --check compares without writing.
go run ./cmd/pyxis-release pins --version vX.Y.Z [--check]
```

## How the trust works

The pins in `install.sh` are the trust anchor. The installer checks the binary
against them before running anything, and every later artifact is checked against
the release manifest they name. The download host can serve bytes but can't choose
them.

## Mirrors and air-gapped installs

- Point the installer at a local or mirrored artifact directory with
  `PYXIS_ARTIFACT_SOURCE`.
- `pyxis-release mirror` and `cut --mirror` publish only to an explicit
  `releases.distribution` binding, such as a Forgejo instance. Without one they
  refuse.
- `PYXIS_VERSION` with `PYXIS_SHA256` installs an earlier release.
