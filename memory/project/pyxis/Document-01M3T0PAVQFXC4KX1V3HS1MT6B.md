# Pyxis

> The compass that steers the fleet.

Pyxis is a software factory for software engineers: it orchestrates a fleet of
AI-agent workers to plan, build, gate, and ship software across a GitHub
organization. It is the clean-slate successor to **harbormaster** — same mission
(govern _how to build_, while the GitHub issue graph holds _what to build_),
rebuilt on a stronger design and a better toolchain.

The name is the mariner's **compass** — and the constellation _Pyxis_, formed
from a fragment of the ancient ship **Argo**: the instrument that keeps the whole
fleet on course.

## Install

Pyxis is distributed through Cloudflare R2. Obtain the distribution credential
from the operator, then install the `pyxis` CLI:

```bash
export PYXIS_ARTIFACT_AUTH="pyxis:<password>"
curl -fsSL https://pyxis.agenticlab.ai/install | bash
```

It downloads the pinned `pyxis` binary for your platform, verifies its checksum
before anything runs, installs it to `~/.local/bin`, and records the release pin
for the next step. Releases with `pyxis shell install` also enable zsh startup
automatically: a fresh terminal launches interactive Codex, Claude Code, Cursor,
and OpenCode sessions in a new worktree when the repository has a `.pyxis/`
directory. Headless and help/version commands retain their native behavior.
The installer preserves existing `.zshrc` content and avoids duplicate setup on
reinstall; [shell setup details](script/startup/README.md) cover custom paths
and checkout-based setup.

**Set up a repository.** Inside an empty directory or empty Git repository:

```bash
pyxis init
```

`init` collects the installation settings and Git destination, initializes Git
when needed, fetches and verifies the engine tools, and installs the gates,
hooks, and skills. It commits and verifies the initial tree, pushes it to the
configured destination, and sets the branch's upstream before completing setup.
The destination can be a local Git repository, GitHub, or Forgejo. Branch
protection and hosted welcome work follow the first push.

For a local installation, provide an existing bare repository path when prompted.
Promptless installation accepts `--remote-url <path-or-URL>` or the `remote_url`
answers-file field; an existing configured remote supplies the destination.
GitHub's chosen repository or the `code_host` integration binding supplies the
hosted destination. Preview includes the first push and performs no publication.

Installation refuses to overwrite an existing destination branch. If a push's
outcome is uncertain or a later setup step fails after publication, the error
names the failure and retains the installation. Fix the reported cause and run
`pyxis init` again to resume the same commits and destination. Successful
publication confirms the remote head; hosted CI reports its result separately.

Native implementation and review require an OS process boundary. On Linux,
install Bubblewrap (`sudo apt-get install bubblewrap` on Debian/Ubuntu).
AppArmor hosts may also require the shipped user-namespace profile:

```bash
sudo install -m 0644 .pyxis/security/bwrap.apparmor /etc/apparmor.d/pyxis-bwrap
sudo apparmor_parser --replace /etc/apparmor.d/pyxis-bwrap
pyxis doctor
```

`pyxis init` checks this support before setup and reports a remedy; `pyxis doctor`
marks it unhealthy until usable. macOS uses the system `sandbox-exec`.

### How it stays trustworthy

The R2 channel serves `install.sh` over authenticated HTTPS. Its version and
per-platform sha256 pins are the trust anchor: it verifies each byte
before installing and fails closed on any mismatch. Every artifact `pyxis` later
fetches is likewise checksum-verified against the release manifest before use —
the release host is never trusted to *choose* bytes, only to serve them. For
air-gapped or mirror installs, point at a local artifact directory with
`PYXIS_ARTIFACT_SOURCE` (a mirror never receives your token).

## Status

Pyxis is still in its product-definition and design phase, but its first code
has shipped: **docgate**, the doc-system gate. docgate performs the PRD-ledger
fold and runs the gate verbs that keep the documentation layer consistent.

It is enforced two ways: the **docs-gate** CI check on every pull request, and a
pre-push hook activated once per clone with
`go run ./cmd/hooks-setup`. This is the same installer the executor uses: one
active hook dispatcher preserves the parent-checkout guard and forwards to the
repository's `.githooks` definitions, including pre-push verification and commit
stamping. Re-run it once to migrate an older guard-only installation.

## Releases & distribution

Canonical releases live on **this repo's** GitHub Releases: one system tag
`vX.Y.Z` carries every engine binary (scan-derived, darwin/linux ×
arm64/amd64), the payload archive, the release
manifest, and `SHA256SUMS` (`docs/techspec.md` § Release channel).
**R2 is the only distribution channel**, serving the same verified artifact
set to consumers without requiring source-repository access.
The trust anchor is the config-pinned manifest hash, never the channel, so
the mirror can serve bytes but not choose them.

### Cutting a release

One command, run twice, with one human gate between:

```sh
# write access on the source repo; R2 credentials are required for run 2
export GH_TOKEN="$(gh auth token -u mdarabi)"

go run ./cmd/pyxis-release cut --version vX.Y.Z            # 1. publish + repin
#    → commit install.sh, open the PR, get it merged, git pull --ff-only
go run ./cmd/pyxis-release cut --version vX.Y.Z --mirror   # 2. finish
```

**Run 1** builds the scan-derived binary matrix, packs the artifact set into
`dist/`, verifies it against `SHA256SUMS` + the release manifest, checks that
the tag resolves to the exact commit the artifacts were packed from
(`--push-tag` creates it there when absent), publishes to this repo's
Releases upload-or-verify, and then **regenerates `install.sh`'s `BEGIN PINS`
block** from what it just published. It stops there.

**The gate** is the pin PR. The pins name the release manifest's hash, the
manifest names the source commit, and `install.sh` is committed — so a tag
can never carry the pins for its own artifacts, and the pin commit
necessarily lands *after* the tag, through review like any other change.

**Run 2** reuses the already-verified set, re-verifies the publish, confirms
the pins are current, and publishes to R2. It refuses
to mirror while `install.sh` is modified-but-uncommitted, because dist
consumers would then be served an anchor this repo's own front door lacks.

Run 2 wants the **same checkout**, with `dist/` still in place. Changed source
or toolchain inputs can produce different bytes that a published tag refuses.
From a fresh clone, skip `cut` and mirror the canonical set
directly with `mirror --tag vX.Y.Z`, which fetches it. `cut` says exactly
that if you hit it.

Every stage is upload-or-verify, so re-running verifies the published set
without changing it. Confirm the R2 publication summary and spot-check the
consumer path with the install command above.

The individual verbs (`build`, `pack`, `verify`, `publish`, `pins`, `mirror`)
remain available for partial re-runs — `go run ./cmd/pyxis-release` lists
them. Two worth knowing:

```sh
# regenerate the pins from a PUBLISHED release (no local build needed):
# downloads the five pinned artifacts, hashes THE BYTES, cross-checks them
# against that release's own SHA256SUMS. --check compares without writing.
go run ./cmd/pyxis-release pins --version vX.Y.Z [--check]

# mirror an already-published release on its own (what `cut --mirror` calls)
go run ./cmd/pyxis-release mirror --tag vX.Y.Z
```

`mirror` fetches the canonical set into a temp dir (`--out` reuses an
already-fetched one), verifies it, publishes it to R2 upload-or-verify, and
refreshes the bucket's root `install.sh` with the checkout's verbatim copy.
It refuses when the checkout's pins name a different tag. R2 configuration
is mandatory for `mirror` and `cut --mirror`; missing inputs fail before
publication. With `mirror --out`, no GitHub access is needed.

The `system-release` workflow that automates the tag-push half is staged at
`.pyxis/dist/source-workflows/system-release.yml` pending the
`workflow`-scope hand-off (#483); it runs the same `cut` verb. The mirror
half stays operator-driven by construction — it waits on the pin PR.

### The R2 channel (no GitHub account needed)

Consumers receive releases from **Cloudflare R2**
behind a Basic-auth Worker door at `https://dist.pyxis.agenticlab.ai`
(bucket public access off; the Worker is the only read path). The same
`mirror` run requires these inputs — locally or as repo Actions secrets/vars:

```sh
export R2_ENDPOINT="https://<account-id>.r2.cloudflarestorage.com"
export R2_BUCKET="pyxis-dist"
export R2_ACCESS_KEY_ID=…  R2_SECRET_ACCESS_KEY=…   # bucket-scoped API token
go run ./cmd/pyxis-release cut --version vX.Y.Z --mirror   # or: mirror --tag vX.Y.Z
```

`R2_BUCKET` names the existing Cloudflare bucket, which is independent of
any GitHub repository. Keep that bucket and its Worker in place.

Artifacts land under `<tag>/`; the source `install.sh` verbatim at the
bucket root (R2 consumers fetch via `PYXIS_ARTIFACT_SOURCE`, which bypasses
the script's gh path, so no repo retargeting applies). A consumer needs
only curl plus the door credential, which rides the environment — it never
enters a committed file.

**The front door.** `scripts/install_pyxis.sh` is published verbatim, public
and unauthenticated, at `https://pyxis.agenticlab.ai/install`, so what a new
user is given is a line rather than a file:

```sh
export PYXIS_ARTIFACT_AUTH="pyxis:<password>"   # the one thing to be given
curl -fsSL https://pyxis.agenticlab.ai/install | bash
pyxis init
```

Nothing else is passed, at install or afterwards. `pyxis init`, `pyxis
upgrade`, hooks, and an operand's CI all resolve through
`https://dist.pyxis.agenticlab.ai` — the channel the install recorded, or the
built-in one when nothing was recorded (prd:installer/FR-28) — so no consumer
ever types a channel, a source, or a repository. The credential stays in the
environment, which is the only thing a consumer holds.

`init` needs no channel flag: the installer records the channel it installed
from beside the release pin, and artifact resolution falls back to it — so a
doored install resolves to the door on its own (FR-30). The record holds the
channel ROOT, never a versioned directory, and `<root>/<version>` is composed
per fetch, so it keeps working across upgrades. The precedence, most specific
first: `--artifact-source` → the repo's `system.artifact_source` → the machine's
recorded channel → the built-in release host.

**Settings live in files, never in the environment.** A variable does not
survive a restart, a new shell, or a cron run, so no configuration value is
read from one: not the channel, not the artifact source, not the release pin.
An operator changes any of them with a flag, and the flag is written down —
`upgrade --channel` records the channel it resolved through, so it need be
given only once. Credentials are the sole exception, env-only in the opposite
direction: `PYXIS_ARTIFACT_AUTH` and `GH_TOKEN`/`GITHUB_TOKEN` must never reach
a file.

A machine installed from a source that is not laid out by tag — an air-gapped
copy, a one-release mirror — records that flat source instead of a root, good
for the release it names. The installer still takes its inputs from the
environment (`PYXIS_ARTIFACT_SOURCE`, `PYXIS_VERSION`, …): it runs before any
config exists, and writing them down is exactly what it is for.

Those public bytes carry no pins and name no version: they ask the channel
which release its root `install.sh` currently pins and hand off to that copy,
so the endpoint never goes stale between releases and nothing about a release
becomes public. `PYXIS_VERSION` + `PYXIS_SHA256` install a prior tag instead
(the kill-switch path); `PYXIS_DIST_HOST` retargets the channel, refusing any
plaintext base but loopback. Use the full `https://` URL — without a scheme
curl tries `http://` and, without `-L`, pipes the redirect page into the
shell. The equivalent against the door directly, no front door involved:

```sh
curl -fsSL -u "pyxis:<password>" https://dist.pyxis.agenticlab.ai/install.sh |
  PYXIS_ARTIFACT_SOURCE="https://dist.pyxis.agenticlab.ai/vX.Y.Z" \
  PYXIS_ARTIFACT_AUTH="pyxis:<password>" bash
PYXIS_ARTIFACT_AUTH="pyxis:<password>" pyxis init
```

(`--artifact-source https://dist.pyxis.agenticlab.ai/vX.Y.Z` still overrides,
and is what a mirror or an air-gapped copy wants; it is no longer needed just
to reach the door.)

Any shell or CI that later fetches (converge, upgrades, fresh-clone
self-heal) needs `PYXIS_ARTIFACT_AUTH` exported the same way. The door adds
confidentiality only — the config-pinned manifest hash still gates every
byte before first execution (DINV-2), so the bucket, like every mirror,
serves bytes but never chooses them.

### Agent-CLI surfaces

`pyxis init` decides which agent CLIs get pyxis's skills by probing `PATH`
once, so a machine with none installed records none — permanently. Installing
Codex or Cursor afterwards wires nothing until the repository is told:

```sh
pyxis surfaces                      # report: what is installed, what is wired
pyxis surfaces --detected           # wire what this machine has
pyxis surfaces --all                # wire all four, installed or not
pyxis surfaces --set codex,cursor   # wire exactly these
```

Bare it reports and performs nothing; on a terminal it offers the choice.
Selection is not limited to what is installed — a repository provisioned for a
team wires surfaces the provisioner's own machine has never seen. Supported:
`claude`, `codex`, `cursor`, `opencode`.

## Design & review tooling

Review substantial documents with the operator in Markdown or the PR, applying
confirmed edits to the source and recording approval in the owning artifact.
The automated PRD, technical-design, and e2e reviewers remain available through
their existing skills.

For an offline HTML plan preview, run:

```sh
go run ./cmd/plangate render docs/plans/<feature>
```

The viewer writes `.pyxis/scratch/plan-<feature>-<NNN>.html` by default and
opens as a standalone file in a browser. Pyxis installation and release
packaging use the verified system payload without a package-install step.

New releases use `release-manifest/v2`. Before repinning a repository with
older v1-only tooling, run `pyxis upgrade` or the current bootstrap installer
to upgrade the CLI; v1-only manifest readers cannot read v2 manifests.
