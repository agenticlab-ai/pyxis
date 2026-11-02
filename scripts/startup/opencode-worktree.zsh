source "${${(%):-%x}:A:h}/worktree.zsh"

function opencode { _pyxis_startup opencode _pyxis_opencode_startup "$@"; }

_pyxis_opencode_startup() {
  px_name=opencode px_label=OpenCode px_command_prefix=opencode
  local option
  local -A px_options
  # OpenCode routes resumed session operations to its saved directory, even
  # when launched elsewhere. A worktree alone cannot relocate that session.
  for option in -v -c --continue -s --session --fork; do px_options[$option]=pass; done
  for option in --log-level --port --hostname --mdns-domain -m --model --prompt --agent --replay-limit; do px_options[$option]=value; done
  for option in --print-logs --pure --mdns --auto --mini --no-replay; do px_options[$option]=boolean; done
  px_options[--cors]=values
  _pyxis_startup_parse "$@" || return 1
  (( ${#px_positions} > 1 )) && return 1
  if (( ${#px_positions} )); then
    # OpenCode's positional is a project directory, not an initial prompt.
    px_target="${px_positions[1]:A}"
    px_launch[${px_indices[1]}]=()
  fi
  return 0
}
