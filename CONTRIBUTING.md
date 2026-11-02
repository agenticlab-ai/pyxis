# Contributing to Pyxis

Thanks for helping. This guide gets you from a fresh clone to a pull request that
passes the gate. Pyxis develops itself with its own pipeline, so a few rules here
are stricter than you may be used to.

## Set up

You need [Go](https://go.dev/dl/) at the version in [`go.mod`](go.mod), Git, and
Node 20 and Python 3 for the multi-language fixture tests. On Linux, install
Bubblewrap and zsh too (`sudo apt-get install bubblewrap zsh` on Debian or
Ubuntu).

```bash
git clone https://github.com/<you>/pyxis.git
cd pyxis
go run ./cmd/hooks-setup   # once per clone: installs the pre-push and commit hooks
go build ./...
go test ./...
```

## Check your work

The pre-push hook and CI run the same gate. To run it yourself:

```bash
.pyxis/scripts/code-gate-resolve.sh run       # code gate: lint, tests, coverage, memory
.pyxis/scripts/docgate-resolve.sh pre-push    # document gate
```

Changed code needs tests. The gate requires 90% coverage for each file.

## Open a pull request

1. Fork the repository and work on a branch named `<type>/<slug>`, such as
   `fix/doctor-exit-code`.
2. Keep each pull request to one concern, with a
   [conventional-commit](https://www.conventionalcommits.org/) title.
3. Fill in the [description template](.pyxis/templates/pr-description-template.md).
   It starts with data for the reviewer, then What, Why, How, and Testing. Get the
   line count from the tool, and check the result before you open the pull request:

   ```bash
   go run ./cmd/pyxis pr-description lines
   go run ./cmd/pyxis pr-description check --body description.md
   ```

4. Recount after every push, because CI fails a description whose count no longer
   matches the diff.

A maintainer reviews and approves the exact head before it merges. CI on a
first-time contributor's pull request waits for a maintainer to start it.

## Rules worth knowing first

The full rules are in [AGENTS.md](AGENTS.md), which binds people and agents alike.

- **Go for everything.** Engine, tools, and tests are Go. Shell is allowed only as
  a thin shim where a shell is a hard requirement, and it must run on bash 3.2.
- **Few dependencies.** Prefer the standard library, and justify anything new.
- **Hexagonal structure.** Ports are interfaces on the consumer side, and adapters
  live under `internal/adapters/<port>/<medium>/`. Engine packages import no port
  adapter.
- **Documents have rules.** Requirement IDs are stable. The vision and principles
  documents are maintainer-owned. Merged designs and requirements change through
  a new document or a typed amendment, not an edit.
- **New features use a draft.** Larger features go through a reviewed requirements
  and design draft before code. Open an issue first so we can agree on the
  approach.

## Reporting bugs and ideas

Use the [issue templates](https://github.com/agenticlab-ai/pyxis/issues/new/choose).
Report security problems privately as described in [SECURITY.md](SECURITY.md).

Everyone taking part follows the [Code of Conduct](CODE_OF_CONDUCT.md). By
contributing, you agree that your contribution is licensed under the
[MIT License](LICENSE).
