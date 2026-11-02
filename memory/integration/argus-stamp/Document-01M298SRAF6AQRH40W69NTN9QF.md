# Argus authorship producer — Claude Code wiring (a pointer, not a content copy)

Wires Claude Code's commit-authoring path to the canonical stamp
(`cmd/argus-stamp`), which holds the trailer logic and the `Argus-Drive` key
once (`internal/argus/stamp`, AP-25). This file names **no** trailer format and
**no** protected path — it is a pointer.

## Claude Code's commit path

Claude Code authors commits by running `git commit` through its Bash tool —
**standard git**. So its commit path is the repo's own git hook surface: the
`core.hooksPath` **prepare-commit-msg** hook (`.githooks/prepare-commit-msg`)
fires on every commit it authors, delegates to `argus-stamp`, and the stamp is
produced by construction — the model authored the *files*, the harness ran the
*commit*, so the trailer is out of the model's reach (DINV-3).

## Enabling

1. Activate the repo hook surface once per clone:

   ```
   go run ./cmd/hooks-setup
   ```

2. In the drive context (where the supervisor/harness lands a drive's work),
   export the active drive's slug so the producer keys the trailer on it:

   ```
   export ARGUS_DRIVE_SLUG=<the-active-drive-slug>
   ```

   With `$ARGUS_DRIVE_SLUG` unset, a commit is authored outside any drive and is
   left un-stamped (it derives as human).

The `argus-stamp` binary is run from source in this repo (the resolver falls
back to `go run ./cmd/argus-stamp`); a built binary on `PATH` is used when
present.
