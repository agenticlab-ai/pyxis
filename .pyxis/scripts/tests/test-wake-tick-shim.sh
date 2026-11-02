#!/usr/bin/env bash
# test-wake-tick-shim.sh — the wake-tick shim stays delegate-only,
# bash-3.2-clean, and one dry idle pass runs end-to-end (wake-channel-v1
# §3.2; plan T8's shim; #515).
set -u
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
SHIM="$REPO_ROOT/.pyxis/scripts/wake-tick.sh"
PASS=0; FAIL=0
ok()  { PASS=$((PASS+1)); printf 'ok   - %s\n' "$1"; }
bad() { FAIL=$((FAIL+1)); printf 'FAIL - %s\n' "$1"; }

[ -f "$SHIM" ] && ok "shim exists" || { bad "shim missing"; echo "$FAIL failure(s)"; exit 1; }
sh -n "$SHIM" && ok "parses (sh -n)" || bad "fails sh -n"
/bin/bash -n "$SHIM" && ok "parses (stock bash)" || bad "fails bash -n"
grep -q 'exec go run ./cmd/wake-tick' "$SHIM" && ok "delegates to cmd/wake-tick" || bad "does not delegate"
CODE_LINES=$(grep -cv '^[[:space:]]*#\|^[[:space:]]*$' "$SHIM")
[ "$CODE_LINES" -le 6 ] && ok "stays a thin shim ($CODE_LINES code lines)" || bad "has grown logic ($CODE_LINES code lines)"
grep -qE '\$\{[a-zA-Z_]+(\^\^|,,)\}|;&' "$SHIM" && bad "bash-4-only syntax present" || ok "no bash-4-only syntax"

# One dry pass end-to-end against an EMPTY temp repo (no problems → an idle
# tick, zero probes, zero network) — hermetic even once real awaits exist.
# The later --repo overrides the shim's own; the shim stays delegate-only.
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
OUT="$(cd "$REPO_ROOT" && sh "$SHIM" --repo "$TMP" --dry-run 2>/dev/null)" || { bad "dry tick failed"; echo "$FAIL failure(s)"; exit 1; }
printf '%s' "$OUT" | grep -q '"dry_run":true' && ok "dry tick reports dry_run" || bad "dry tick output unexpected: $OUT"
printf '%s' "$OUT" | grep -q '"Probes":0' && ok "idle tick performs zero probes" || bad "idle tick probed: $OUT"

echo "$PASS pass, $FAIL fail"
[ "$FAIL" -eq 0 ]
