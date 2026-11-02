The machine installer enables these wrappers automatically for zsh. The
verified `pyxis` binary carries the same files, installs them under
`${XDG_DATA_HOME:-$HOME/.local/share}/pyxis/startup/`, and adds one managed
source block to `${ZDOTDIR:-$HOME}/.zshrc`. Reinstalling updates that block
without duplicating it or changing the rest of the file. Existing rc symlinks
and permissions are preserved. Open a new terminal to activate the setup.

For an existing installation with this command, run `pyxis shell install`.
To use a source checkout directly, source the loader from `~/.zshrc`:

```zsh
source "/path/to/checkout/scripts/startup/startup.zsh"
```

Each harness has its own sourceable wrapper. Source one of these instead when
you only want that harness.

This directory is machine setup, outside the `.pyxis/` payload that `pyxis init`
copies into managed repositories.

Aliases that add flags should call the command name, for example
`alias claude='claude --effort high'`. `command claude` and absolute executable
paths bypass shell functions and therefore bypass these wrappers.

| File | Interactive commands |
| --- | --- |
| `codex-worktree.zsh` | `codex`, `codex "prompt"`, `codex resume <session-id-or-name>` |
| `claude-worktree.zsh` | `claude`, `claude "prompt"`, `claude --resume <session-id-or-name>`, `claude -r <session-id-or-name>` |
| `cursor-worktree.zsh` | `cursor`, `cursor .`, `cursor path/to/project` (IDE); `agent`, `cursor-agent`, `cursor agent` (CLI) |
| `opencode-worktree.zsh` | `opencode`, `opencode path/to/project --prompt "prompt"` |

`worktree.zsh` owns argument handling, repository discovery, and the Git
lifecycle. Each harness file defines its command entry points, option roles,
and command-discovery rules. Completion/help text is inspected as data; it is
never evaluated. Plain `cursor` opens the IDE with the selected worktree folder
passed explicitly. `cursor agent` and `cursor-agent` start the agent CLI.
IDE launches accept one existing folder and window options such as
`--new-window`, `--reuse-window`, and `--wait`. Files, multiple folders,
diff/merge, extension management, and remote commands pass through unchanged.
The native Cursor IDE shell command must be installed on `PATH`.

Fresh interactive sessions and explicit Codex/Claude resumes opt in when the
effective Git repository root contains a `.pyxis` directory: any repository
set up with Pyxis, with no further registration. Discovery works from subdirectories, Codex's
`--cd`, Cursor's `--workspace` directory, and OpenCode's positional project path.
Saved Cursor workspace names pass through when they are not directory paths.
There is no repository list or user-specific path in the wrappers.

Each repository names its own mainline. The launcher asks the installed
`pyxis` (`pyxis shell mainline <repository>`) for the `remote` and `branch`
set in that repository's `.pyxis/config.yaml`; a repository that sets neither
uses `origin` and `main`. Configuration `pyxis` refuses stops the launch
before any fetch. A source checkout sourced on a machine without `pyxis` on
`PATH` uses `origin` and `main`.

The shared launcher fetches that remote branch, fast-forwards the local branch
of the same name to that tip, creates a `chore/<harness>-...` branch under
`../.pyxis-worktrees/<repository-name>/<harness>-...`, and starts the CLI
there. If the original checkout is on the mainline branch, its working tree is
fast-forwarded too (`git pull --ff-only`). Other branches and their uncommitted
changes stay put. Relative file and additional-directory options keep referring
to their original paths. The shell stays in the new worktree after the CLI
exits. A failed fetch stops the launch. A local mainline branch that cannot be
fast-forwarded is left unchanged; the new worktree still uses the fetched tip.

Before launch, the shared launcher copies `.pyxis/config.local.yaml` and
`.vscode/` from the original checkout when those optional paths exist,
preserving contents and permissions. An absent path does not block startup.
Copying requires a real destination parent and an absent destination;
symlinks and existing paths are refused. A refusal or copy failure stops
startup and removes the newly created worktree and branch. Each new worktree
gets an independent snapshot. Reusing an existing worktree leaves its local
configuration and editor settings untouched.

Explicit Codex and Claude resumes use the same fresh mainline worktree and
receive the optional local config and `.vscode/` snapshots. They restore the conversation;
other uncommitted files stay in the original checkout. Inside an existing
linked worktree they reuse that tree. The wrapper supplies `--cd` so
Codex uses the selected tree even when the session saved another directory or
`tui.resume_cwd` is `"session"`. Claude uses its launch directory and can find
sessions across the repository's worktrees. Relative transcript paths passed to
Claude's `--resume` are made absolute before changing directory. Use the complete
session ID or its name.

Two native resume limitations prevent applying the same directory change:

- Cursor's `agent --resume <id>`, `cursor-agent --resume <id>`, and
  `cursor agent --resume <id>` stay in their original directory. Cursor stores
  history by workspace, and changing `--workspace` starts without that history;
  [Cursor support documents this limitation](https://forum.cursor.com/t/cli-agent-in-one-session-forgets-everything-when-switching-to-new-root/162955/4).
- OpenCode's `--session <id>` / `-s <id>` also pass through. OpenCode 1.17.20
  [routes session operations to the saved directory ahead of the launch directory](https://github.com/anomalyco/opencode/blob/v1.17.20/packages/opencode/src/server/routes/instance/httpapi/middleware/workspace-routing.ts).
  Creating a worktree would therefore leave session operations in the original
  checkout. Relocation needs session migration, which these wrappers do not perform.

Other launches in existing linked worktrees, resume pickers and continue/latest
selectors (including Codex's `--last`), attach commands, explicit native
worktree options, help/version checks, and nonterminal or noninteractive shell
invocations pass through. So do headless commands such as `codex exec`,
`claude -p`, `agent -p`, and `opencode run`, as well as utilities and servers.
Unknown options pass through unchanged for the installed CLI to interpret.

Pass-through preserves the CLI's own checks and exit status. For example,
`codex exec` normally rejects directories outside Git. Explicitly allow that
case with `codex exec --skip-git-repo-check "who you are?"`; the wrapper does
not add the flag automatically. CLI references describe the corresponding
[Claude Code](https://code.claude.com/docs/en/cli-reference),
[Cursor](https://cursor.com/docs/cli/reference/parameters), and
[OpenCode](https://opencode.ai/docs/cli/) modes.

Run the terminal integration tests from the repository root:

```sh
go test ./scripts/startup
```

Tests require Go, Git, zsh, and `script`. They exercise real Git repositories and
pseudo-terminals with CLI launch recorders and the `pyxis` binary built from
this checkout. An opt-in suite compares installed
CLI help/version output with and without the wrappers, verifies Codex's native
outside-Git rejection, and resumes fixture sessions in the installed Codex and
Claude TUIs. Codex's `/status` verifies the worktree with the saved-directory
preference enabled; Claude's `SessionStart` hook verifies the resumed session ID
and working directory. These native checks use isolated homes and offline
provider settings. Neither suite submits model prompts:

```sh
PYXIS_TEST_REAL_CODEX=1 PYXIS_TEST_REAL_HARNESSES=1 go test ./scripts/startup
```
