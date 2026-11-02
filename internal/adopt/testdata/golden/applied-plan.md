# Adoption plan — applied

- **Schema:** adoption-plan/v1
- **Status:** applied
- **Epoch:** cdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcd

## Entries

- `.pyxis/config.yaml` — add
- `core.hooksPath` — chain
  the adopter's core.hooksPath is .husky; the engine chains its checks ahead of it and restores .husky on undo
- `AGENTS.md` — collide-manual
  ```
  - Pyxis: run pyxis converge before every commit.
  - Pyxis: see docs/tech-design/ for design records.
  ```
- `src/**` — never-touch
- `docs/tech-design/index.md` — generated
- `.pyxis/scripts/pyxis-bootstrap.sh` — engine-verbatim

## Hooks

- **Displaced prior hooks path:** .husky

## Stack

- **Detected:** go
- **Required gate actions:** executor.gate.build, executor.gate.test, executor.gate.lint

## Added paths (undo baseline)

- `.pyxis/config.yaml` — sha256:abababababababababababababababababababababababababababababababab
