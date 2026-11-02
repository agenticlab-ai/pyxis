# Source individually, or source startup.zsh to enable every harness.
source "${${(%):-%x}:A:h}/worktree.zsh"

function codex { _pyxis_startup codex _pyxis_codex_startup "$@"; }

_pyxis_codex_startup() {
  px_name=codex px_label=Codex px_command_reader=_pyxis_codex_commands
  local option
  local -A px_options
  for option in -V --remote --remote-auth-token-env; do px_options[$option]=pass; done
  for option in -c --config --enable --disable -m --model --local-provider -p --profile -s --sandbox -a --ask-for-approval; do px_options[$option]=value; done
  for option in --strict-config --oss --approve-for-me --dangerously-bypass-approvals-and-sandbox --dangerously-bypass-hook-trust --search --no-alt-screen; do px_options[$option]=flag; done
  for option in -C --cd; do px_options[$option]=target; done
  for option in -i --image --add-dir; do px_options[$option]=csv-paths; done
  _pyxis_startup_parse "$@" || return 1
  if [[ "${px_words[1]}" == resume ]]; then
    # A named session can move before launch. Picker/--last selection must
    # remain in the original directory, where Codex filters saved sessions.
    (( ${#px_positions} >= 2 && ${#px_positions} <= 3 )) || return 1
    [[ -n "${px_positions[2]}" ]] || return 1
    px_words=() # The session name and optional prompt are not subcommands.
    px_cwd_option=--cd # Override the saved cwd, including tui.resume_cwd=session.
  fi
  return 0
}

_pyxis_codex_commands() {
  local completion line
  local -i in_commands=0
  completion="$(command codex completion zsh 2>/dev/null)" || return 1
  for line in "${(@f)completion}"; do
    if [[ "$line" == '_codex_commands() {' ]]; then in_commands=1; continue; fi
    (( in_commands )) || continue
    [[ "$line" == '}' ]] && break
    [[ "$line" == \'*:* ]] || continue
    px_subcommands+=("${${line#\'}%%:*}")
  done
  (( ${#px_subcommands} ))
}
