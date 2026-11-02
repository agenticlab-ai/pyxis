#!/usr/bin/env bash
# test-argus-stamp-hook.sh — the Argus authorship producer's prepare-commit-msg
# shim stays delegate-only and bash-3.2-clean, its resolver is the exec-grammar
# delegate (installer-v1 §3.10 "Shims"; plan installer/001 T12), and an
# end-to-end commit through the hook carries the Argus-Drive trailer by
# construction while a commit outside a drive carries none
# (docs/tech-design/argus-attribution-hardening.md §3.1; A-1).
set -u
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
PASS=0; FAIL=0
ok()  { PASS=$((PASS+1)); printf 'ok   - %s\n' "$1"; }
bad() { FAIL=$((FAIL+1)); printf 'FAIL - %s\n' "$1"; }

HOOK="$REPO_ROOT/.githooks/prepare-commit-msg"
if [ ! -f "$HOOK" ]; then
  bad "prepare-commit-msg hook missing"
else
  sh -n "$HOOK" && ok "hook parses (sh -n)" || bad "hook fails sh -n"
  /bin/bash -n "$HOOK" && ok "hook parses (stock bash)" || bad "hook fails bash -n"
  grep -q "argus-stamp-resolve.sh" "$HOOK" && ok "hook delegates to the resolver" || bad "hook does not delegate to the resolver"
  CODE_LINES=$(grep -cv '^[[:space:]]*#\|^[[:space:]]*$' "$HOOK")
  [ "$CODE_LINES" -le 5 ] && ok "hook stays a thin shim ($CODE_LINES code lines)" || bad "hook has grown logic ($CODE_LINES code lines)"
  grep -qE '\$\{[a-zA-Z_]+(\^\^|,,)\}|;&' "$HOOK" && bad "hook has bash-4-only syntax" || ok "hook: no bash-4-only syntax"
fi

RES="$REPO_ROOT/.pyxis/scripts/argus-stamp-resolve.sh"
if [ ! -f "$RES" ]; then
  bad "resolver missing"
else
  sh -n "$RES" && ok "resolver parses (sh -n)" || bad "resolver fails sh -n"
  /bin/bash -n "$RES" && ok "resolver parses (stock bash)" || bad "resolver fails bash -n"
  # The exec-grammar delegate (T12): the stamper is reached through pyxis exec,
  # closing this resolver's old unpinned-PATH-binary DINV-2 gap.
  grep -q 'exec argus-stamp --' "$RES" && ok "resolver delegates into the exec grammar (pyxis exec argus-stamp --)" || bad "resolver does not delegate into the exec grammar"
  # Locate-and-delegate only: no fetch/verify logic remains (T12). It still
  # absolutizes the commit-message file (git hands it relative) and anchors to
  # this repo's tree — that is locate, not fetch.
  if grep -qE 'curl|sha256|shasum|releases/download' "$RES"; then bad "resolver carries fetch/verify logic (T12 retires it into pyxis exec)"; else ok "resolver holds no fetch/verify logic (locate-and-delegate only)"; fi
fi

# End-to-end: a commit authored through the hook carries the trailer keyed on
# the active drive's slug; a commit outside a drive carries none. Needs go.
# ARGUS_STAMP_WORKTREE pins the stamper to the sandbox worktree (the shim cds to
# THIS repo to source-run the stamper, so `git rev-parse --show-toplevel` alone
# would bind the wrong tree under the inherited GIT_DIR — a hermetic test hook,
# exactly the override cmd/argus-stamp documents for this case).
if command -v go >/dev/null 2>&1; then
  SB="$(mktemp -d)"
  ( cd "$SB" && git init -q -b main >/dev/null 2>&1 \
      && git config user.email t@e.x && git config user.name t \
      && mkdir -p .githooks \
      && printf '#!/bin/sh\nexport ARGUS_STAMP_WORKTREE="%s"\nexec "%s/.pyxis/scripts/argus-stamp-resolve.sh" "$@"\n' "$SB" "$REPO_ROOT" > .githooks/prepare-commit-msg \
      && chmod +x .githooks/prepare-commit-msg )
  printf 'x\n' > "$SB/f.txt"
  ( cd "$SB" && git -c core.hooksPath=.githooks add f.txt \
      && ARGUS_DRIVE_SLUG=widget git -c core.hooksPath=.githooks commit -q -m "feat: x" ) >/dev/null 2>&1
  BODY="$(cd "$SB" && git log -1 --format=%B 2>/dev/null)"
  case "$BODY" in
    *"Argus-Drive: widget"*) ok "drive commit stamped by construction (A-1)" ;;
    *) bad "drive commit not stamped: $BODY" ;;
  esac
  printf 'y\n' > "$SB/g.txt"
  ( cd "$SB" && git -c core.hooksPath=.githooks add g.txt \
      && git -c core.hooksPath=.githooks commit -q -m "chore: no drive" ) >/dev/null 2>&1
  BODY2="$(cd "$SB" && git log -1 --format=%B 2>/dev/null)"
  case "$BODY2" in
    *"Argus-Drive"*) bad "non-drive commit carried a trailer: $BODY2" ;;
    *) ok "commit outside a drive carries no trailer" ;;
  esac
  rm -rf "$SB"
else
  printf 'skip - end-to-end hook run (no go toolchain)\n'
fi

printf '\n%d passed, %d failed\n' "$PASS" "$FAIL"
[ "$FAIL" -eq 0 ]
