#!/usr/bin/env bash
# test-codegate-resolve.sh — the codegate resolver is the exec-grammar delegate
# (installer-v1 §3.10 "Shims"; plan installer/001 T12). The fetch-verify-cache
# and section-scoped config parse that once lived here now live once in `pyxis
# exec` over internal/resolve (DINV-1/DINV-2), covered by the Go suite. Here the
# resolver is a locate-and-delegate shim, bash-3.2-clean, holding no fetch/parse
# logic of its own — this asserts exactly that, and that the delegate reaches
# codegate through the exec grammar.
set -u
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
RES="$REPO_ROOT/.pyxis/scripts/codegate-resolve.sh"
PASS=0; FAIL=0
ok()  { PASS=$((PASS+1)); printf 'ok   - %s\n' "$1"; }
bad() { FAIL=$((FAIL+1)); printf 'FAIL - %s\n' "$1"; }

[ -f "$RES" ] || { bad "resolver missing at $RES"; echo "1..$((PASS+FAIL))"; exit 1; }
[ -x "$RES" ] || bad "resolver is not executable"

# Parses under both POSIX sh and stock (3.2-floor) bash.
sh -n "$RES" && ok "resolver parses (sh -n)" || bad "resolver fails sh -n"
/bin/bash -n "$RES" && ok "resolver parses (stock bash)" || bad "resolver fails bash -n"

# No bash-4-only syntax (macOS stock bash floor).
grep -qE '\$\{[a-zA-Z_]+(\^\^|,,)\}|;&' "$RES" && bad "resolver has bash-4-only syntax" || ok "resolver: no bash-4-only syntax"

# Delegates into the one exec grammar (pyxis exec codegate -- …) — the same
# acquisition-and-exec path every tool now speaks (T12).
grep -q 'exec codegate --' "$RES" && ok "resolver delegates into the exec grammar (pyxis exec codegate --)" || bad "resolver does not delegate into the exec grammar"

# Locate-and-delegate only: no fetch/parse/verify logic remains — that is the
# whole point of T12 (it, and the section-scoped parse the sibling once carried,
# now live in Go).
if grep -qE 'curl|sha256|shasum|releases/download|cfg_scoped|awk ' "$RES"; then
  bad "resolver still carries fetch/parse logic (T12 retires it into pyxis exec)"
else
  ok "resolver holds no fetch/parse logic (locate-and-delegate only)"
fi

# Thin: excluding comments/blanks, a locate+delegate handful of lines.
CODE_LINES=$(grep -cv '^[[:space:]]*#\|^[[:space:]]*$' "$RES")
[ "$CODE_LINES" -le 8 ] && ok "resolver stays thin ($CODE_LINES code lines)" || bad "resolver has grown logic ($CODE_LINES code lines)"

# Reaches codegate through the delegate (a harmless verb; the exec grammar's
# source-mode leg runs codegate from this repo). The NR-1 parity proof for this
# call site — through the delegate, not a direct go run.
if command -v go >/dev/null 2>&1; then
  if bash "$RES" version >/dev/null 2>&1; then ok "delegate reaches codegate through the exec grammar (version exit 0)"; else bad "delegate did not reach codegate"; fi
else
  printf 'skip - delegate reach (no go toolchain)\n'
fi

echo "1..$((PASS+FAIL))"
[ "$FAIL" -eq 0 ] || exit 1
