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

For anyone with a GitHub account and read access to this repository, Pyxis
installs in two steps.

**1. Authenticate the GitHub CLI.** Pyxis is served from this private repo over
your `gh` login, so [install `gh`](https://cli.github.com) and sign in:

```bash
gh auth login
```

**2. Install the `pyxis` CLI** with one line:

```bash
gh api repos/agenticlab-ai/pyxis/contents/install.sh \
  -H "Accept: application/vnd.github.raw" | bash
```

It downloads the pinned `pyxis` binary for your platform, verifies its checksum
before anything runs, installs it to `~/.local/bin`, and records the release pin
for the next step.

**Set up a repository.** Inside any repo you want Pyxis to manage:

```bash
pyxis init
```

`init` fetches and verifies every engine tool over your `gh` credentials, wires
the gates, hooks, and skills, and leaves a `doctor`-healthy operand — no flags
needed when the reported prerequisites are available.

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

`install.sh` is served over your authenticated `gh` session, so the version and
per-platform sha256 pins it carries are the trust anchor: it verifies each byte
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
`git config core.hooksPath .githooks`.

## Releases & distribution

Canonical releases live on **this repo's** GitHub Releases: one system tag
`vX.Y.Z` carries every engine binary (scan-derived, darwin/linux ×
arm64/amd64), the payload archive, the offline deps bundle, the release
manifest, and `SHA256SUMS` (`docs/techspec.md` § Release channel).
[`agenticlab-ai/pyxis-dist`](https://github.com/agenticlab-ai/pyxis-dist) is
the binaries-only mirror for consumers **without source read** — read access
there grants downloads, never code; its README is the consumer install guide.
The trust anchor is the config-pinned manifest hash, never the channel, so
the mirror can serve bytes but not choose them.

### Cutting a release

One command, run twice, with one human gate between:

```sh
# write access on both repos (the mirror leg needs pyxis-dist)
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
the pins are current, and mirrors to every distribution channel. It refuses
to mirror while `install.sh` is modified-but-uncommitted, because dist
consumers would then be served an anchor this repo's own front door lacks.

Run 2 wants the **same checkout**, with `dist/` still in place. A repack is
never byte-identical (the deps bundle wraps a real npm cache), so a published
tag refuses it — from a fresh clone, skip `cut` and mirror the canonical set
directly with `mirror --tag vX.Y.Z`, which fetches it. `cut` says exactly
that if you hit it.

Every stage is upload-or-verify, so re-running is free and never forks an
artifact set. Then confirm the mirror carries the full set —
`gh release view vX.Y.Z --repo agenticlab-ai/pyxis-dist` — and spot-check the
consumer path with the one-liner in pyxis-dist's README.

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
already-fetched one), verifies it, publishes it to pyxis-dist
upload-or-verify, and refreshes pyxis-dist's committed `install.sh` — the
checkout's copy with every repo reference retargeted, skipped when already
current. It refuses when the checkout's pins name a different tag, because
republishing stale pins would strand every dist consumer on the previous
release.

The `system-release` workflow that automates the tag-push half is staged at
`.pyxis/dist/source-workflows/system-release.yml` pending the
`workflow`-scope hand-off (#483); it runs the same `cut` verb. The mirror
half stays operator-driven by construction — it waits on the pin PR.

### The R2 channel (no GitHub account needed)

Beside the GitHub mirror, the same releases are served from **Cloudflare R2**
behind a Basic-auth Worker door at `https://dist.pyxis.agenticlab.ai`
(bucket public access off; the Worker is the only read path). The same
`mirror` run publishes it when the R2 environment is present — locally or
as repo Actions secrets/vars:

```sh
export R2_ENDPOINT="https://<account-id>.r2.cloudflarestorage.com"
export R2_BUCKET="pyxis-dist"
export R2_ACCESS_KEY_ID=…  R2_SECRET_ACCESS_KEY=…   # bucket-scoped API token
go run ./cmd/pyxis-release cut --version vX.Y.Z --mirror   # or: mirror --tag vX.Y.Z
```

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

Pyxis development is design-heavy, so the repo ships the **doc-review** loop
from day one: the `doc-review` skill (`.pyxis/skills/doc-review/`) renders a
markdown doc to a rich HTML surface (the vendored **lavish** editor), opens it
in your browser for inline annotation, and the agent applies each comment back
to the source by propose-and-confirm.

One-time setup (installs the vendored review server's runtime deps,
lockfile-pinned):

```bash
bash .pyxis/scripts/setup-lavish.sh
```

Codex discovers the repo skills through `.agents/skills` (with
`.codex/skills` retained as a compatibility pointer). In Codex, mention
`$doc-review`; in Claude Code, invoke `/doc-review`. The machinery keeps the
tool's own name — a pinned, vendored `lavish-axi` run via local `node` (never
`npx`), a self-host driver, and
contract tests that pin the `/api` wire shapes — documented in
`.pyxis/vendor/lavish-axi/README.md`, and every piece carries the lessons from
harbormaster's integration incidents. Tests:

```bash
bash .pyxis/scripts/tests/test-lavish-driver.sh
bash .pyxis/scripts/tests/test-lavish-api-contract.sh
```
