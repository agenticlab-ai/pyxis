# Integration bindings

Pyxis selects each external capability independently. Existing repositories
without `integrations` retain their GitHub/Carina configuration. Add this block
to `.pyxis/config.yaml` to use the local Forgejo installation for tasks and code:

```yaml
integrations:
  schema_version: pyxis-integrations/v1
  connections:
    local:
      provider: forgejo
      base_url: http://127.0.0.1:3000
      token_env: PYXIS_FORGEJO_TOKEN
  bindings:
    tasks:
      connection: local
      repository: operator/test-repo
    code_host:
      connection: local
      repository: operator/test-repo
  ci:
    mode: both
    runner_label: pyxis-linux
```

Export the token in the invoking process's environment. Store only its
environment variable name in configuration. `.pyxis/config.local.yaml` can
override connection leaves for one machine; shared configuration stays in Git.
Unknown integration keys are rejected. Forgejo base URLs include any port and
installation path. Public GitHub connections use `provider: github` and may
name their own `token_env`; an explicit connection is pinned to github.com.

| Binding | Pyxis uses it for |
| --- | --- |
| `tasks` | Plan projection, labels, blockers, claims, task pickup and closure |
| `code_host` | Git credentials, PRs, reviews, comments, check results, watch, wake and merge lane |
| `releases.source` | Publishing and pinning the project's canonical releases |
| `releases.distribution` | Publishing a verified distribution copy |
| `artifacts` | Downloading Pyxis binaries and payloads for install, execution and upgrade |

Every hosted binding has `connection` and `repository`. Use different
connections when credentials differ, even on the same server. For Carina tasks,
use `provider: carina` and `root: <operand-checkout>` instead. A relative Carina
root resolves from the configured repository. During init, omit it or use `.`;
runtime may use an external operand after initialization. CI belongs to the code
host: `local` requires Pyxis's verified local result, `actions` uses hosted
checks, and `both` requires both. The runner label selects an existing runner;
Pyxis does not register one automatically.

Optional release and artifact bindings fit under `bindings` as follows:

```yaml
    releases:
      source:
        connection: local
        repository: owner/project-releases
      distribution:
        connection: local
        repository: owner/project-distribution
    artifacts:
      connection: local
      repository: owner/pyxis-releases
```

These repository names are examples. The artifact repository must contain
actual Pyxis release manifests, payloads, binaries and checksum pins; selecting
a project's ordinary code repository does not make it a Pyxis artifact source.
`pyxis-release publish --target source|distribution` selects the publishing
role. The source repository must already contain the release tag pointing to
the packaged commit. Use the distribution binding for an asset repository
without that Git history. The [installed workflow notes](dist/forgejo-workflows/README.md) cover
artifact credentials and the initial bootstrap before configuration exists.

For an existing installed repository, run `pyxis converge --preview`, then
`pyxis converge`. New initialization accepts `pyxis init --integrations <file>`
alongside its ordinary install options. The file contains the `integrations:`
block above. Converge installs repository-scoped Git authentication; source
checkouts can configure it explicitly with
`pyxis credential --repo <checkout> configure` using a durable installed Pyxis
binary. Git fetches and pushes then use the selected code-host token without
putting it in remote URLs or Git configuration.

Run `pyxis watch --repo <checkout> --plan docs/plans/<feature>` or one
`pyxis exec implement-now -- --repo <checkout> --plan-ref <feature>/<version>`
sweep as usual.
Explicit bindings take precedence over legacy tracker flags. With an explicit
code-host binding, `--repo-slug` is optional. Operations whose hosted bindings
all select Forgejo do not invoke `gh` or require GitHub authentication. Omitted
release and artifact bindings retain their legacy defaults; configure those
bindings too when using Forgejo for distribution, installation and upgrade.

Tasks, claims, feedback cursors and proposal associations include the server
and repository. Enabling bindings refuses existing unscoped numeric claims or
legacy task branches until their work is finished or explicitly reconciled.
Wake references with explicit bindings must contain full URLs, such as
`pr:http://127.0.0.1:3000/operator/test-repo/pulls/7` or
`item:http://127.0.0.1:3000/operator/test-repo/issues/8`.

Forgejo dependencies use its native API. Issue types and parent/child links use
typed Pyxis metadata where Forgejo lacks matching fields. Native PR discussion,
review summaries and inline comments feed the same review-feedback loop.
Forgejo 16 may omit original-author provenance after an administrator remaps
imported content to a local user. Pyxis trusts the API's reported user identity;
it rejects exposed import markers and missing user IDs, but cannot distinguish
those remapped imports through that API.

## Fast installed feature E2E

With `PYXIS_FORGEJO_TOKEN` already exported, run from the Pyxis source checkout:

```sh
PYXIS_E2E_FORGEJO=1 \
  PYXIS_E2E_FORGEJO_URL=http://127.0.0.1:3000 \
  PYXIS_E2E_FORGEJO_REPO=operator/test-repo \
  go test ./internal/installer/e2e \
    -run '^TestLiveForgejoInstalledFeatureDevelopment$' -count=1 -v -timeout=5m
```

The opt-in test requires an existing nonempty repository with `main`, repository
write access, and token scopes `read:user`, `write:repository`, `write:issue`.
It uses the real Go/Git/Node toolchain and Pyxis's native worker confinement.
Ordinary test runs skip this live scenario.

The test builds a small, checksum-pinned release from current source, runs the
real `install.sh` and `pyxis init` in temporary directories, then develops a tiny
Go feature through the installed executor. Only the Codex implementation and
review processes are stubbed. Task projection, Git claims/worktrees, local Go
tests, exact-head Forgejo status, review comments and draft PR publication are
real. A second sweep must reuse that PR without another model call.

Each run owns a unique task, routing label and base/head/evidence refs. Cleanup
closes its task and unmerged PR, removes its label and refs, and checks that
pre-existing repository content is unchanged. Closed task/PR history remains.
The test never creates a repository or merges a PR. Installation starts with
Carina tasks, then switches to native Forgejo tasks through configuration and
`pyxis converge`, avoiding repeated repository-wide onboarding. HOME, shell
startup and XDG directories are temporary. Local CI keeps this test independent
of runner queues; Forgejo Actions and release publication have separate tests.

Review state has three independently configurable Git refs. Existing defaults
remain unchanged; an isolated run can set all three under its own namespace:

```yaml
executor:
  review:
    backend: codex
    evidence_ref: refs/pyxis/my-run/review
    process_evidence_ref: refs/pyxis/my-run/review-process
    publication_evidence_ref: refs/pyxis/my-run/review-publication
```

These name obligation, coordinator and publication state respectively. They
must be safe `refs/pyxis/*` refs. Configure them before starting work; changing
refs does not migrate existing review state.
