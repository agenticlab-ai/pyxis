# .pyxis/dist/operand-hooks — the installed-repo git-hook overlay

This directory is the payload's **overlay home** for the operand's git hooks
(installer-v1 §3.5 step 6; plan installer/001 T8). The files here ship inside
the payload archive at this path; they take effect in an operand only after
`pyxis init` installs them into the operand's hooks directory and points
`core.hooksPath` at it — the source repo's own desk hooks never run them.

## Why a separate variant exists (the finding-(b) gap)

The source repo's own desk hooks are **source-mode delegates**: they run
`go run ./cmd/pyxis exec <tool> -- …` through per-tool shim scripts, which need
the engine source and a Go toolchain. **An installed operand carries neither**
— it has no `cmd/pyxis`, so `go run ./cmd/pyxis` cannot work there, and those
source-mode desk hooks are not shipped into the operand payload at all.

The variants here instead call the pinned **`pyxis` binary directly** — it
reaches PATH through the release cache (`.pyxis/bin`, primed by the bootstrap
shim `pyxis-bootstrap.sh` and appended to PATH by the dev shell / CI). Every
`pyxis exec <tool> -- …` re-verifies the tool against the pinned release
manifest before running it, so the desk gate stays on the one no-unpinned-
execution floor (DINV-1) without any source in the tree.

## The mapping

`pyxis init` installs these files into the operand's hooks directory
(`.githooks`, the `core.hooksPath` value the doctor expects — installer-v1
§3.5 step 6, FR-9) **by name**, and sets `core.hooksPath .githooks`:

| Payload path | Operand hook | Delegates to |
|---|---|---|
| `.pyxis/dist/operand-hooks/pre-commit` | `.githooks/pre-commit` | `pyxis exec docgate -- gate` |
| `.pyxis/dist/operand-hooks/pre-push` | `.githooks/pre-push` | `pyxis exec docgate -- pre-push` (pyxis resolves the merge-base from the configured branch+remote at runtime — config-driven-portability-v1 §3.1) |
| `.pyxis/dist/operand-hooks/prepare-commit-msg` | `.githooks/prepare-commit-msg` | `pyxis exec argus-stamp -- <msg>` |

- The installed hooks are **executable** (mode 0755) and hold **no logic** —
  locate `pyxis` on PATH, delegate, propagate the tool's exit code.
- The class policy classes `.pyxis/dist/operand-hooks/**` **engine-verbatim**
  (it is under the `.pyxis/**` catch-all row) — converge repairs drift on the
  shipped variants byte-exact; the source repo's own `.pyxis/hooks/` delegates
  stay untouched (they are source-mode dev hooks).
- **This README is packer documentation, not overlay content** — it is never
  installed into an operand's `.githooks/`.

## Fail-closed posture

Each variant aborts the git operation with a named remedy when `pyxis` is not
on PATH — a commit or push must never slip past the gate (or a drive commit
land un-attributed) because the front door is missing. The remedy points at
the bootstrap shim and the dev shell.
