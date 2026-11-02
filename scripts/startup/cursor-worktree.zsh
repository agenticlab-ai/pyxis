source "${${(%):-%x}:A:h}/worktree.zsh"

function agent { _pyxis_startup agent _pyxis_cursor_startup "$@"; }
function cursor-agent { _pyxis_startup cursor-agent _pyxis_cursor_startup "$@"; }
function cursor {
  if [[ "${1-}" == agent ]]; then
    _pyxis_startup cursor _pyxis_cursor_startup "$@"
  else
    _pyxis_startup cursor _pyxis_cursor_editor_startup "$@"
  fi
}

_pyxis_cursor_editor_startup() {
  px_name=cursor px_label='Cursor IDE'
  local option
  local -A px_options
  for option in -n --new-window -r --reuse-window -w --wait --disable-extensions --disable-gpu --disable-lcd-text --prof-startup --verbose --suppress-popups-on-startup; do px_options[$option]=flag; done
  for option in --locale --profile --log --disable-extension --inspect-extensions --inspect-brk-extensions --sync; do px_options[$option]=value; done
  for option in --user-data-dir --extensions-dir; do px_options[$option]=path; done
  if [[ "${1-}" == editor ]]; then
    px_launch=(editor)
    shift
  fi
  _pyxis_startup_parse "$@" || return 1
  # Only an implicit folder or one existing folder is an IDE worktree launch.
  # Files, multi-root workspaces, utilities, and unknown options stay native.
  (( ${#px_positions} > 1 )) && return 1
  [[ "${px_words[1]-}" == tunnel ]] && return 1
  if (( ${#px_positions} )); then
    [[ -d "${px_positions[1]}" ]] || return 1
    px_target="${px_positions[1]:A}"
    px_launch[${px_indices[1]}]=()
  fi
  px_words=()
  px_directory_arg=1
  return 0
}

_pyxis_cursor_startup() {
  px_name=cursor px_label=Cursor
  [[ "$px_cli" == cursor ]] && px_help_prefix=(agent)
  local option
  local -A px_options
  # Cursor stores history per workspace; moving a resume starts without its
  # conversation. Keep native resume routing until cross-workspace resume exists.
  for option in -v -p --print --output-format --stream-partial-output --resume --continue --list-models -w --worktree --worktree-base --skip-worktree-setup; do px_options[$option]=pass; done
  for option in --api-key -H --header -e --endpoint --mode --model --sandbox; do px_options[$option]=value; done
  for option in --plan -f --force --yolo --auto-review --approve-mcps --trust; do px_options[$option]=flag; done
  for option in --add-dir --plugin-dir; do px_options[$option]=path; done
  px_options[--workspace]=target
  _pyxis_startup_parse "$@" || return 1
  if [[ "${px_words[1]}" == agent ]]; then
    # The explicit agent command takes prompt words, not utility subcommands.
    px_words=()
  elif (( ${#px_words} )); then
    px_words=("${px_words[1]}")
  fi
  return 0
}
