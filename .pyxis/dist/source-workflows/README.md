# Source-repo workflow staging

The workflows here are source-repo workflows staged until the operator lands
them. They are staged because automated pushes lack the `workflow` scope (the
known #483 class). The source repo's release trigger, `system-release.yml`, has
already landed in `.github/workflows/`.

Not payload content: nothing here is staged into operand payloads (the
packer maps only `../github-workflows/`, per-file).

## installer-e2e.yml

Belongs at `.github/workflows/installer-e2e.yml` (the source repo's e2e matrix —
linux+darwin, runs the `internal/installer/e2e` harness). Staged here for the
same reason: automated pushes lack the `workflow` scope. The operator lands it
with `git mv .pyxis/dist/source-workflows/installer-e2e.yml .github/workflows/`.

## self-host.yml

Belongs at `.github/workflows/self-host.yml` (the source repo's config-driven-
portability self-host acceptance job — linux+darwin, runs the
`internal/selfhost` harness plus the local code-gate and `pyxis doctor` over the
checkout). It is A-1's CI half: it proves the engine runs green over its own
corpus relocated under a non-default config (config-driven-portability-v1 §3.8).
Staged here for the same reason: automated pushes lack the `workflow` scope. The
operator lands it with `git mv .pyxis/dist/source-workflows/self-host.yml .github/workflows/`.
