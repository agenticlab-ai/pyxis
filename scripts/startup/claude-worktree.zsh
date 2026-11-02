source "${${(%):-%x}:A:h}/worktree.zsh"

function claude { _pyxis_startup claude _pyxis_claude_startup "$@"; }

_pyxis_claude_startup() {
  px_name=claude px_label='Claude Code'
  # Documented entry points that some installations hide from --help.
  px_subcommands=(daemon remote-control self-hosted-runner)
  local option
  local -A px_options
  for option in -v -p --print --output-format --input-format --max-budget-usd --no-session-persistence --bg --background --cloud --environment -c --continue --from-pr --teleport --fork-session --session-id -w --worktree --tmux; do px_options[$option]=pass; done
  for option in -r --resume; do px_options[$option]=resume-path; done
  for option in --agent --agents --advisor --append-system-prompt --autocompact --effort --fallback-model --model -n --name --permission-mode --setting-sources --system-prompt --system-prompt-snapshot; do px_options[$option]=value; done
  for option in --allow-dangerously-skip-permissions --ax-screen-reader --bare --brief --chrome --dangerously-skip-permissions --disable-slash-commands --exclude-dynamic-system-prompt-sections --ide --no-chrome --restricted --safe-mode --strict-mcp-config --verbose; do px_options[$option]=flag; done
  for option in --allowedTools --allowed-tools --disallowedTools --disallowed-tools --betas --file --tools; do px_options[$option]=values; done
  for option in --debug-file --plugin-dir --system-prompt-file --append-system-prompt-file; do px_options[$option]=path; done
  for option in -d --debug --prompt-suggestions; do px_options[$option]=optional; done
  px_options[--add-dir]=paths
  px_options[--mcp-config]=json-paths
  px_options[--settings]=json-path
  _pyxis_startup_parse "$@" || return 1
  (( ${#px_words} )) && px_words=("${px_words[1]}")
  return 0
}
