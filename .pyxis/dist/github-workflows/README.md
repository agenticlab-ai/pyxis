# .pyxis/dist/github-workflows — the installed-repo workflow overlay

This directory is the payload's **overlay home** for hosted-CI workflow
files (installer-v1 §3.10). The files here ship inside the payload archive
at this path; they take effect in an operand only through the overlay
mapping below — the source repo's own CI never runs them.

## The mapping

Each workflow file here maps **per file, by name**, onto the operand's
workflow directory:

| Payload path | Operand path |
|---|---|
| `.pyxis/dist/github-workflows/docs-gate.yml` | `.github/workflows/docs-gate.yml` |

- The release packer stages the payload with this mapping declared, RENDERING
  the mapped file from this template with the **default** config
  (`config.Defaults()` → branch `main`, remote `origin`) and empty release pins.
  The manifest can't pin its own binaries' hashes into a file it describes, so
  this staged copy only satisfies the class walk; nothing runs it, and the
  bootstrap shim refuses its empty pins by name.
- The class policy's `.github/workflows/docs-gate.yml` row classes the mapped
  file **generated** (config-driven-portability-v1 §3.1, FR-4): a CI-host push
  trigger is a static literal the host evaluates with no config read, so
  branch/remote must be baked correct-for-the-repo. `converge` renders the
  operand's copy from this template with the repo config's branch+remote
  substituted, re-renders it on a config change, and `doctor` names a hand-edit
  as drift (DINV-4). This template — the overlay SOURCE, shipped
  engine-verbatim under `.pyxis/**` — is the single source; there is no second
  copy (AP-11). Its substitution points are `{{branch}}`, `{{remote}}`, and the
  bootstrap step's release pins and artifact source, which `converge` takes from
  the manifest it verified against `system.manifest_sha256`.
  **Template-author note:** the substitution grammar is `{{lower_snake_case}}`,
  and the renderer fails closed on any leftover `{{`. GitHub Actions'
  `${{ … }}` expression syntax collides with this: an added `${{ github.sha }}`
  (spaces/dots/uppercase) is not a valid token, so it would be left unresolved
  and the render would refuse. Keep this workflow to plain `$VAR` shell (as it
  is today — `$GITHUB_PATH`, `$GITHUB_STEP_SUMMARY`, `$PWD`); do not introduce
  `${{ … }}` here.
- `pyxis init` materializes the mapped file when the `ci_mode` knob is
  `actions` or `both` (installer-v1 §3.5 step 7); `local` mode installs no
  workflow.
- **This README is packer documentation, not overlay content** — it is
  never mapped into an operand's `.github/workflows/` (an unmapped file
  there would fail the release build's class walk, DINV-6).

## Why the variant exists

The source repo's `.github/workflows/docs-gate.yml` runs source-toolchain
steps (toolchain setup, the compile-and-check suite) that no installed
repository can execute — an operand has no engine source. The variant here
invokes the gate exclusively through the exec grammar
(`pyxis exec docgate -- gate --merge-base origin/main …`), so the binary is
hash-verified against the pinned release manifest before every execution
and desk hooks and CI can never drift (one umbrella verb, two callers).
Both variants exist before the first system-release tag, and the release
build refuses to publish while any payload workflow carries a
source-toolchain step (installer-v1 §3.10's release preconditions).

## The bootstrap dependency

The workflow's first step runs `.pyxis/scripts/pyxis-bootstrap.sh` — the
one shell first-mover, shipped by the system-release change (plan task T11;
installer-v1 §3.4). Its contract, which this workflow writes against:

- fetch-verify the `pyxis` binary against the pins rendered into the workflow's
  bootstrap step (sha256, fail-closed — a mismatch refuses with zero execution);
- install it into the gitignored `.pyxis/bin/` cache (the solution shape's
  cache-location contract) — on a warm cache it re-hashes and skips the
  fetch;
- bash-3.2-clean, delegate-only.

The workflow, not the shim, appends `.pyxis/bin` to `GITHUB_PATH`; every
later step then speaks `pyxis` from PATH.
