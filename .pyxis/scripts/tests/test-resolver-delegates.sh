#!/usr/bin/env bash
# test-resolver-delegates.sh — the T12 acceptance for the four per-tool resolver
# scripts (plan installer/001 T12; installer-v1 §3.10 "Shims"). Each resolver has
# collapsed to a locate-and-delegate shim into the one exec grammar; the
# fetch-verify-cache and per-tool config parse they once carried now live once in
# `pyxis exec` over internal/resolve (DINV-1/DINV-2), covered by the Go suite.
# This asserts, uniformly across all four: bash-3.2-clean (sh -n + stock bash
# -n), no fetch/parse logic remains (grep), and each delegates into the exec
# grammar. It then proves the docgate delegate actually REACHES its tool green on
# the clean source tree — the NR-1 parity proof through the delegate itself (not
# a direct `go run ./cmd/docgate`, which would bypass the shim under test).
set -u
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
SCRIPTS="$REPO_ROOT/.pyxis/scripts"
PASS=0; FAIL=0
ok()  { PASS=$((PASS+1)); printf 'ok   - %s\n' "$1"; }
bad() { FAIL=$((FAIL+1)); printf 'FAIL - %s\n' "$1"; }

# tool -> the exact `exec <tool> --` needle its delegate must carry.
for pair in \
  "docgate-resolve.sh:docgate" \
  "codegate-resolve.sh:codegate" \
  "code-gate-resolve.sh:code-gate" \
  "argus-stamp-resolve.sh:argus-stamp" \
  "feedback-capture-resolve.sh:feedback-capture"; do
  file="${pair%%:*}"; tool="${pair##*:}"
  RES="$SCRIPTS/$file"
  if [ ! -f "$RES" ]; then bad "$file missing"; continue; fi
  [ -x "$RES" ] && ok "$file is executable" || bad "$file is not executable"
  sh -n "$RES" && ok "$file parses (sh -n)" || bad "$file fails sh -n"
  /bin/bash -n "$RES" && ok "$file parses (stock bash 3.2 floor)" || bad "$file fails bash -n"
  grep -qE '\$\{[a-zA-Z_]+(\^\^|,,)\}|;&' "$RES" && bad "$file has bash-4-only syntax" || ok "$file: no bash-4-only syntax"
  grep -q "exec $tool --" "$RES" && ok "$file delegates into the exec grammar (pyxis exec $tool --)" || bad "$file does not delegate into the exec grammar"
  # The T12 point: no fetch/verify/section-parse logic survives in any shim.
  if grep -qE 'curl|sha256|shasum|releases/download|cfg_scoped' "$RES"; then
    bad "$file still carries fetch/parse logic (T12 retires it into pyxis exec)"
  else
    ok "$file holds no fetch/parse logic (locate-and-delegate only)"
  fi
  # Thin: a locate+delegate handful of lines.
  CODE_LINES=$(grep -cv '^[[:space:]]*#\|^[[:space:]]*$' "$RES")
  [ "$CODE_LINES" -le 8 ] && ok "$file stays thin ($CODE_LINES code lines)" || bad "$file has grown logic ($CODE_LINES code lines)"
done

# Reach proof: the docgate delegate runs the real gate on the clean source tree
# THROUGH the shim and exits 0 (the parity baseline for NR-1). docgate is the
# read-only, side-effect-free choice; the other three are exercised end-to-end by
# their own tests (a stamp is a real file write, a capture a real ledger write).
# The gate's own output is CAPTURED, not discarded: when this assertion goes red
# the reason is a precise, actionable docgate defect line (which file, which rule,
# which remedy), and throwing it away leaves only "did not reach green" — a
# verdict with no evidence, which sends the reader to reproduce the failure
# locally to learn what the gate already said. Incident I3's lesson, and the
# reason it recurred here.
if command -v go >/dev/null 2>&1; then
  if gate_out="$(bash "$SCRIPTS/docgate-resolve.sh" gate --merge-base "${PYXIS_CI_BASE:-origin/main}" 2>&1)"; then
    ok "docgate delegate reaches its tool through the exec grammar (gate exit 0)"
  else
    bad "docgate delegate did not reach docgate green on the clean tree"
    printf '%s\n' "$gate_out" | sed 's/^/#     /'
  fi
else
  printf 'skip - reach proof (no go toolchain)\n'
fi

echo "1..$((PASS+FAIL))"
[ "$FAIL" -eq 0 ] || exit 1
