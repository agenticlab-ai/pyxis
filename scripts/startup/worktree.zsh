# Shared, sourceable zsh implementation. No harness or repository names live here.
# A classifier fills the dynamically scoped px_* invocation fields below.
# Returning nonzero from a classifier passes the original invocation through.
_pyxis_startup() {
  emulate -L zsh
  local px_cli="$1" classifier="$2"
  shift 2
  local -a px_original=("$@") px_launch=() px_words=() px_positions=() px_indices=()
  local -a px_subcommands=() px_help_prefix=()
  local px_target="$PWD" px_name="$px_cli" px_label="$px_cli" px_command_prefix='' px_cwd_option=''
  local -i px_directory_arg=0
  local px_command_reader=_pyxis_startup_help_commands arg
  if [[ ! -o interactive || ! -t 0 || ! -t 1 ]]; then
    command "$px_cli" "${px_original[@]}"
    return $?
  fi
  for arg in "$@"; do
    [[ "$arg" == -- ]] && break
    case "$arg" in
      -h|--help|--version)
        command "$px_cli" "${px_original[@]}"
        return $?
        ;;
    esac
  done
  if ! "$classifier" "$@"; then
    command "$px_cli" "${px_original[@]}"
    return $?
  fi
  local repo git_dir common_dir
  repo="$(command git -C "$px_target" rev-parse --show-toplevel 2>/dev/null)"
  if [[ $? -ne 0 || ! -d "$repo/.pyxis" ]]; then
    command "$px_cli" "${px_original[@]}"
    return $?
  fi
  repo="${repo:A}"
  git_dir="$(command git -C "$repo" rev-parse --absolute-git-dir)" || return $?
  common_dir="$(command git -C "$repo" rev-parse --path-format=absolute --git-common-dir)" || return $?
  if [[ "${git_dir:A}" != "${common_dir:A}" ]]; then
    if [[ -n "$px_cwd_option" ]] || (( px_directory_arg )); then
      _pyxis_startup_launch "$px_target"
    else
      command "$px_cli" "${px_original[@]}"
    fi
    return $?
  fi
  if (( ${#px_words} )); then
    "$px_command_reader" || {
      print -u2 -- "$px_cli: cannot inspect CLI commands; no worktree created."
      return 1
    }
    for arg in "${px_words[@]}"; do
      if (( ${px_subcommands[(Ie)$arg]} )); then
        command "$px_cli" "${px_original[@]}"
        return $?
      fi
    done
  fi
  whence -p "$px_cli" >/dev/null || { command "$px_cli" "${px_original[@]}"; return $?; }
  # The repository's configuration names its remote and default branch, and
  # pyxis reads it. Without an installed pyxis the engine's defaults apply.
  local remote=origin mainline=main resolved
  if whence -p pyxis >/dev/null; then
    resolved="$(command pyxis shell mainline "$repo")" && [[ "$resolved" == ?*$'\n'?* ]] || {
      print -u2 -- "$px_cli: cannot read the repository's remote and branch; no worktree created."
      return 1
    }
    remote="${resolved%%$'\n'*}" mainline="${resolved#*$'\n'}"
  fi
  # Fetch the remote mainline, then fast-forward the local branch to that tip.
  # Never switch the original checkout to another branch. A checked-out
  # mainline is updated with a fast-forward merge (git pull --ff-only); other
  # branches and their uncommitted work stay put. The new worktree still uses
  # the fetched tip.
  command git -C "$repo" fetch --no-tags "$remote" \
    "+refs/heads/${mainline}:refs/remotes/${remote}/${mainline}" || return $?
  local base parent wt branch stamp tail
  base="$(command git -C "$repo" rev-parse --verify "refs/remotes/${remote}/${mainline}^{commit}")" || return $?
  _pyxis_startup_fast_forward_main || return $?
  parent="${repo:h}/.pyxis-worktrees/${repo:t}"
  command mkdir -p "$parent" || return $?
  stamp="$(command date '+%Y%m%d-%H%M%S')" || return $?
  wt="$(command mktemp -d "$parent/${px_name}-${stamp}-XXXXXX")" || return $?
  branch="chore/${wt:t}"
  if ! command git -C "$repo" worktree add --no-track -b "$branch" "$wt" "$base"; then
    command rmdir "$wt" 2>/dev/null || true
    return 1
  fi
  # Preserve optional machine-local settings as independent copies.
  _pyxis_startup_copy_local .pyxis/config.local.yaml || return $?
  _pyxis_startup_copy_local .vscode || return $?
  tail="${${px_target:A}#${repo:A}}"
  if [[ -d "$wt$tail" ]]; then
    builtin cd "$wt$tail" || return $?
  else
    builtin cd "$wt" || return $?
  fi
  print -r -- "$px_label worktree: $wt"
  print -r -- "Branch: $branch"
  _pyxis_startup_launch "$PWD"
}

# Fast-forward refs/heads/$mainline to $base. A missing or diverged branch, a
# dirty checked-out one that cannot fast-forward, or one checked out in another
# worktree leaves the original checkout unchanged and still allows launch.
_pyxis_startup_fast_forward_main() {
  local head
  head="$(command git -C "$repo" symbolic-ref --quiet HEAD || true)"
  if [[ "$head" == "refs/heads/$mainline" ]]; then
    command git -C "$repo" merge --ff-only --quiet "$base" && return 0
    print -u2 -- "$px_cli: cannot fast-forward $mainline; leaving original checkout unchanged."
    return 0
  fi
  if ! command git -C "$repo" rev-parse --verify --quiet "refs/heads/$mainline" >/dev/null; then
    command git -C "$repo" branch --no-track "$mainline" "$base" || return $?
    return 0
  fi
  command git -C "$repo" merge-base --is-ancestor "refs/heads/$mainline" "$base" || return 0
  command git -C "$repo" branch -f "$mainline" "$base" && return 0
  print -u2 -- "$px_cli: cannot fast-forward $mainline; leaving original checkout unchanged."
  return 0
}

# Snapshot an optional machine-local path into the new worktree. A missing
# source is a no-op. The fetched tree controls the destination: never follow
# links or overwrite an existing path with machine-local settings.
_pyxis_startup_copy_local() {
  local rel="$1" src="$repo/$1" dest="$wt/$1" parent
  if [[ -f "$src" ]]; then
    parent="${dest:h}"
    if [[ -L "$parent" || ( -e "$parent" && ! -d "$parent" ) ||
          -e "$dest" || -L "$dest" ]] ||
       ! command mkdir -p "$parent" ||
       ! command cp -p "$src" "$dest"; then
      _pyxis_startup_abort_copy "$rel"
      return 1
    fi
  elif [[ -d "$src" ]]; then
    if [[ -e "$dest" || -L "$dest" ]] ||
       ! command cp -pRH "$src" "$dest"; then
      _pyxis_startup_abort_copy "$rel"
      return 1
    fi
  fi
  return 0
}

_pyxis_startup_abort_copy() {
  print -u2 -- "$px_cli: cannot copy $1 to $wt; harness not started."
  if command git -C "$repo" worktree remove --force "$wt"; then
    command git -C "$repo" branch -D "$branch" ||
      print -u2 -- "$px_cli: cannot remove unused branch $branch."
  else
    print -u2 -- "$px_cli: cannot remove unused worktree $wt; branch $branch retained."
  fi
}

# Supply the selected directory explicitly when the application requires it.
_pyxis_startup_launch() {
  if [[ -n "$px_cwd_option" ]]; then
    px_launch=("$px_cwd_option" "$1" "${px_launch[@]}")
  fi
  if (( px_directory_arg )); then
    px_launch+=("$1")
  fi
  command "$px_cli" "${px_launch[@]}"
}

# px_options maps harness flags to their arity/path role. Unknown or ambiguous
# syntax passes through unchanged, before any fetch or worktree creation.
_pyxis_startup_parse() {
  local arg key value kind item trimmed
  local -a values=() paths=()
  local -i inline=0 separated=0
  while (( $# )); do
    arg="$1"
    shift
    if (( separated )) || [[ "$arg" != -* || "$arg" == - ]]; then
      px_launch+=("$arg")
      px_positions+=("$arg")
      px_indices+=("${#px_launch}")
      (( separated )) || px_words+=("$arg")
      continue
    fi
    if [[ "$arg" == -- ]]; then
      separated=1
      px_launch+=(--)
      continue
    fi
    key="$arg" value='' inline=0
    if [[ "$arg" == --*=* ]]; then
      key="${arg%%=*}" value="${arg#*=}" inline=1
    elif [[ "$arg" == -[^-]?* ]]; then
      key="${arg[1,2]}" value="${arg[3,-1]}" inline=1
      value="${value#=}"
    fi
    kind="${px_options[$key]}"
    case "$kind" in
      ''|pass) return 1 ;;
      flag)
        (( inline )) && return 1
        px_launch+=("$arg")
        continue
        ;;
      boolean)
        if (( inline )) && [[ "$value" != true && "$value" != false ]]; then return 1; fi
        px_launch+=("$arg")
        continue
        ;;
    esac
    values=()
    if (( inline )); then
      values+=("$value")
    elif [[ "$kind" == optional ]]; then
      if (( $# )) && [[ "$1" != -* ]]; then
        values+=("$1")
        shift
      else
        px_launch+=("$arg")
        continue
      fi
    else
      (( $# )) || return 1
      [[ "$1" != -* ]] || return 1
      values+=("$1")
      shift
    fi
    case "$kind" in
      resume-path)
        # A missing/empty optional resume value opens a picker in some CLIs.
        # Such invocations must retain their original directory and arguments.
        [[ -n "${values[1]}" ]] || return 1
        ;;
      values|paths|json-paths)
        while (( $# )) && [[ "$1" != -* ]]; do values+=("$1"); shift; done
        ;;
    esac
    if [[ "$kind" == target ]]; then
      [[ -n "${values[1]}" ]] || return 1
      px_target="${values[1]:A}"
      continue
    fi
    paths=()
    for item in "${values[@]}"; do
      case "$kind" in
        resume-path) [[ -f "$item" ]] && item="${item:A}" ;;
        path|paths) item="${item:A}" ;;
        csv-paths)
          local -a entries=("${(@s:,:)item}")
          entries=("${(@)entries:A}")
          item="${(j:,:)entries}"
          ;;
        json-path|json-paths)
          trimmed="${item#"${item%%[![:space:]]*}"}"
          [[ "$trimmed" == '{'* || "$trimmed" == '['* ]] || item="${item:A}"
          ;;
      esac
      paths+=("$item")
    done
    px_launch+=("$key" "${paths[@]}")
  done
  return 0
}

# Inspect help as data, never evaluate it. Command entries have two-space
# indentation; wrapped descriptions are more deeply indented.
_pyxis_startup_help_commands() {
  local help line token alias_text
  local -i in_commands=0 found=0
  help="$(LC_ALL=C NO_COLOR=1 command "$px_cli" "${px_help_prefix[@]}" --help 2>/dev/null)" || return 1
  for line in "${(@f)help}"; do
    if [[ "$line" == 'Commands:' ]]; then in_commands=1; continue; fi
    (( in_commands )) || continue
    [[ "$line" == [[:alpha:]]* ]] && break
    [[ "$line" == '  '[![:space:]]* ]] || continue
    line="${line#'  '}"
    if [[ -n "$px_command_prefix" ]]; then
      [[ "$line" == "$px_command_prefix "* ]] || continue
      line="${line#"$px_command_prefix "}"
    fi
    token="${line%%[[:space:]]*}"
    [[ "$token" == [[:alpha:]]* ]] || continue
    px_subcommands+=("${(@s:|:)token}")
    if [[ "$line" == *'[aliases: '* ]]; then
      alias_text="${${line##*'[aliases: '}%%']'*}"
      alias_text="${alias_text//, /,}"
      px_subcommands+=("${(@s:,:)alias_text}")
    fi
    found=1
  done
  (( found ))
}
