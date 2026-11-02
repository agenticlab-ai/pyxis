#!/usr/bin/env bash
# test-feedback-capture-shim.sh — the confirmed-feedback capture shim is the
# exec-grammar delegate (installer-v1 §3.10 "Shims"; plan installer/001 T12): it
# stays locate-and-delegate only (no capture logic — that lives in Go, reached
# through `pyxis exec`), bash-3.2-clean, and fails OPEN (never blocks the review)
# (docs/tech-design/review-feedback-capture.md §3.3; issue #235 acceptance).
set -u
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
SHIM="$REPO_ROOT/.pyxis/scripts/feedback-capture-resolve.sh"
PASS=0; FAIL=0
ok()  { PASS=$((PASS+1)); printf 'ok   - %s\n' "$1"; }
bad() { FAIL=$((FAIL+1)); printf 'FAIL - %s\n' "$1"; }

[ -f "$SHIM" ] || { bad "shim missing at $SHIM"; echo "1..$((PASS+FAIL))"; exit 1; }
[ -x "$SHIM" ] || bad "shim is not executable"

# Parses under both POSIX sh and stock (3.2-floor) bash.
sh -n "$SHIM" && ok "shim parses (sh -n)" || bad "shim fails sh -n"
/bin/bash -n "$SHIM" && ok "shim parses (stock bash)" || bad "shim fails bash -n"

# No bash-4-only syntax (macOS stock bash floor).
grep -qE '\$\{[a-zA-Z_]+(\^\^|,,)\}|;&' "$SHIM" && bad "shim has bash-4-only syntax" || ok "shim: no bash-4-only syntax"

# Locate-and-delegate only: it delegates to the writer through the one exec
# grammar (pyxis exec feedback-capture -- …) and never grows capture logic (no
# ledger rendering, no queue paths, no comment construction — all in Go).
grep -q 'exec feedback-capture --' "$SHIM" && ok "shim delegates into the exec grammar (pyxis exec feedback-capture --)" || bad "shim does not delegate into the exec grammar"
if grep -qE 'docs/feedback|feedback-queue|## |Subject-anchor|effective\.source\.json|curl|sha256' "$SHIM"; then
  bad "shim contains capture/fetch logic (ledger/queue/entry construction belongs in Go)"
else
  ok "shim holds no capture/fetch logic"
fi

# Thin: excluding comments/blanks, the shim is a handful of locate+delegate lines.
CODE_LINES=$(grep -cv '^[[:space:]]*#\|^[[:space:]]*$' "$SHIM")
[ "$CODE_LINES" -le 8 ] && ok "shim stays thin ($CODE_LINES code lines)" || bad "shim has grown logic ($CODE_LINES code lines)"

# Fails open: with no Go toolchain on PATH (a bare coreutils PATH), the shim must
# exit 0 (never block the review) rather than error out — the delegate needs go
# to run pyxis from source, so a go-absent guard must still return 0.
# /usr/bin:/bin supplies dirname/pwd but not go/feedback-capture.
if PATH="/usr/bin:/bin" "$SHIM" --author=@x --subject-anchor='d#1' </dev/null >/dev/null 2>&1; then
  ok "shim fails open when no writer is runnable (exit 0, review not blocked)"
else
  bad "shim blocked the review when no writer was runnable"
fi

echo "1..$((PASS+FAIL))"
[ "$FAIL" -eq 0 ] || exit 1
