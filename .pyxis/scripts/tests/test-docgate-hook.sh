#!/usr/bin/env bash
# test-docgate-hook.sh — every docgate hook shim stays delegate-only and
# bash-3.2-clean (docgate-v1.md A-4; pre-commit added by amendment 001/FR-9).
set -u
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
PASS=0; FAIL=0
ok()  { PASS=$((PASS+1)); printf 'ok   - %s\n' "$1"; }
bad() { FAIL=$((FAIL+1)); printf 'FAIL - %s\n' "$1"; }

for name in pre-push pre-commit; do
  HOOK="$REPO_ROOT/.githooks/$name"
  if [ ! -f "$HOOK" ]; then bad "$name missing"; continue; fi
  sh -n "$HOOK" && ok "$name parses (sh -n)" || bad "$name fails sh -n"
  /bin/bash -n "$HOOK" && ok "$name parses (stock bash)" || bad "$name fails bash -n"
  case "$name" in
    pre-push) VERB="pre-push" ;;
    *) VERB="gate" ;;
  esac
  grep -q "docgate-resolve.sh\" $VERB" "$HOOK" && ok "$name delegates to the resolver ($VERB)" || bad "$name does not delegate to the resolver"
  CODE_LINES=$(grep -cv '^[[:space:]]*#\|^[[:space:]]*$' "$HOOK")
  [ "$CODE_LINES" -le 5 ] && ok "$name stays a thin shim ($CODE_LINES code lines)" || bad "$name has grown logic ($CODE_LINES code lines)"
  grep -qE '\$\{[a-zA-Z_]+(\^\^|,,)\}|;&' "$HOOK" && bad "$name has bash-4-only syntax" || ok "$name: no bash-4-only syntax"
done
# The pre-push also runs the code-gate predicate (code-gate-mirror.md §3.6): a
# human push is gated by the same required-CI code checks the executor and CI run.
grep -q 'code-gate-resolve.sh" run' "$REPO_ROOT/.githooks/pre-push" && ok "pre-push also delegates to code-gate (run)" || bad "pre-push does not delegate to code-gate"
# A deletion-only push passes without content checks (FR-29).
ZERO=0000000000000000000000000000000000000000
if printf '(delete) %s refs/heads/x %s\n' "$ZERO" "$ZERO" | (cd "$REPO_ROOT" && go run ./cmd/docgate pre-push --merge-base "${PYXIS_CI_BASE:-origin/main}" >/dev/null 2>&1); then
  ok "pre-push skips a deletion-only push (FR-29)"
else
  bad "pre-push gated a deletion-only push"
fi
# The commit-time hook must not require a merge base (FR-9: works with no remote).
grep -v "^#" "$REPO_ROOT/.githooks/pre-commit" | grep -q "merge-base" && bad "pre-commit invocation references a merge base" || ok "pre-commit invocation is merge-base-free"
# Resolver (post-T12): the exec-grammar delegate. The fetch-verify-cache and
# DINV-2 fail-closed logic that once lived here now lives once in `pyxis exec`
# over internal/resolve (covered by the Go suite); here the resolver is a
# locate-and-delegate shim, and this asserts exactly that (installer-v1 §3.10;
# plan installer/001 T12 — "no script carries fetch or parse logic").
RES="$REPO_ROOT/.pyxis/scripts/docgate-resolve.sh"
sh -n "$RES" && ok "resolver parses (sh -n)" || bad "resolver fails sh -n"
grep -q 'exec docgate --' "$RES" && ok "resolver delegates into the exec grammar (pyxis exec docgate --)" || bad "resolver does not delegate into the exec grammar"
if grep -qE 'curl|sha256|shasum|releases/download|cfg_scoped|awk ' "$RES"; then bad "resolver still carries fetch/parse logic (T12 retires it into pyxis exec)"; else ok "resolver holds no fetch/parse logic (locate-and-delegate only)"; fi
# Reaches docgate through the delegate on the clean source tree (the NR-1 parity
# proof for this call site — through the delegate, not a direct go run).
if gate_out="$(bash "$RES" gate --merge-base "${PYXIS_CI_BASE:-origin/main}" 2>&1)"; then
  ok "delegate reaches docgate through the exec grammar (gate exit 0)"
else
  bad "delegate did not reach docgate green on the clean tree"
  printf '%s\n' "$gate_out" | sed 's/^/#     /'
fi

printf '\n%d passed, %d failed\n' "$PASS" "$FAIL"
[ "$FAIL" -eq 0 ]
