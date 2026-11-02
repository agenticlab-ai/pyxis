#!/usr/bin/env bash
# test-maintenance-sweep-shim.sh — the maintenance-sweep scheduler shim stays
# delegate-only, bash-3.2-clean, and its header carries the scheduler binding and
# the steady-state cost statement; one hermetic dry pass runs end-to-end and
# mutates nothing; and the sweep package holds no write-capable client
# (maintenance-sweep-v1 §3.9, DINV-1; A-8, A-9; plan T6's shim + proofs).
set -u
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
SHIM="$REPO_ROOT/.pyxis/scripts/maintenance-sweep.sh"
PKG="$REPO_ROOT/internal/sweep"
PASS=0; FAIL=0
ok()   { PASS=$((PASS+1)); printf 'ok   - %s\n' "$1"; }
bad()  { FAIL=$((FAIL+1)); printf 'FAIL - %s\n' "$1"; }
skip() { printf 'skip - %s\n' "$1"; }

# --- the shim exists and parses under both the POSIX and stock-bash floors ---
[ -f "$SHIM" ] || { bad "shim missing at $SHIM"; echo "$FAIL failure(s)"; exit 1; }
[ -x "$SHIM" ] && ok "shim is executable" || bad "shim is not executable"
sh -n "$SHIM" && ok "parses (sh -n)" || bad "fails sh -n"
/bin/bash -n "$SHIM" && ok "parses (stock bash)" || bad "fails bash -n"
grep -qE '\$\{[a-zA-Z_]+(\^\^|,,)\}|;&' "$SHIM" && bad "bash-4-only syntax present" || ok "no bash-4-only syntax (bash-3.2 floor)"

# --- delegate-only: it hands off to the Go binary and grows no sweep logic ---
grep -q 'exec go run ./cmd/maintenance-sweep' "$SHIM" && ok "delegates to cmd/maintenance-sweep" || bad "does not delegate to the binary"
# Inspect only the CODE lines (the header names detectors/policy descriptively) —
# the shim must carry none of the sweep's own decision logic.
if grep -v '^[[:space:]]*#' "$SHIM" | grep -qiE 'waiver|detector|finding|schema_version|staleness|import graph|jq|json'; then
  bad "shim contains sweep logic (detectors/policy/report belong in Go)"
else
  ok "shim holds no sweep logic"
fi
CODE_LINES=$(grep -cv '^[[:space:]]*#\|^[[:space:]]*$' "$SHIM")
[ "$CODE_LINES" -le 6 ] && ok "stays a thin shim ($CODE_LINES code lines)" || bad "has grown logic ($CODE_LINES code lines)"

# --- the header carries the scheduler binding and the cost statement (A-9) ---
grep -qi 'cron' "$SHIM" && grep -qi 'launchd' "$SHIM" && ok "header documents the scheduler binding (cron + launchd)" || bad "header lacks the scheduler binding"
grep -qi 'one bounded pass' "$SHIM" && grep -qiE 'nothing resident|zero between' "$SHIM" && ok "header states the steady-state cost (one bounded pass, nothing resident)" || bad "header lacks the cost statement"

# --- import audit (DINV-1): the sweep package holds no write-capable client ---
# Non-test sources may READ the tree and exec read-only git/go, but must open no
# network client and make no filesystem-write call — the read-only contract made
# structural. (Test sources build hermetic fixtures and are exempt.)
NONTEST=$(find "$PKG" -maxdepth 1 -name '*.go' ! -name '*_test.go')
if grep -nE 'net/http|net/rpc|"net"|golang\.org/x/net|ioutil\.WriteFile|os\.(Create|WriteFile|Remove|RemoveAll|Mkdir|MkdirAll|Rename|Chmod|Truncate|OpenFile|Symlink|Link)\b' $NONTEST; then
  bad "sweep package imports/uses a write-capable or network client (DINV-1 breach)"
else
  ok "import audit: sweep package holds no write-capable or network client (DINV-1)"
fi

# --- hermetic dry pass + mutation audit (A-8): one end-to-end pass over a temp
# repo emits the machine face and mutates nothing (needs the Go toolchain). ---
if ! command -v go >/dev/null 2>&1; then
  skip "hermetic dry pass (no Go toolchain on PATH)"
else
  TMP="$(mktemp -d)"
  trap 'rm -rf "$TMP"' EXIT
  # A minimal, self-contained module (one main under cmd/, no deps) so the pass
  # runs clean and offline — the sweep's own reachability shape in miniature.
  mkdir -p "$TMP/cmd/tool"
  printf 'module drytest\n\ngo 1.26\n' > "$TMP/go.mod"
  printf 'package main\n\nfunc main() {}\n' > "$TMP/cmd/tool/main.go"
  ( cd "$TMP" && git init -q && git add -A && \
    GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@t GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@t \
    git commit -q -m init ) || { bad "could not build the temp fixture repo"; }
  BEFORE_STATUS="$(cd "$TMP" && git status --porcelain)"
  BEFORE_HEAD="$(cd "$TMP" && git rev-parse HEAD)"
  # The trailing --repo overrides the shim's own; the shim stays delegate-only.
  OUT="$(cd "$REPO_ROOT" && sh "$SHIM" --repo "$TMP" 2>/dev/null)"; RC=$?
  printf '%s' "$OUT" | grep -q 'maintenance-report/v1' \
    && ok "dry pass emits a maintenance-report/v1 machine face" \
    || bad "dry pass emitted no report machine face (rc=$RC): $OUT"
  [ "$RC" -eq 0 ] && ok "dry pass over a clean fixture exits 0" || bad "dry pass exit was $RC, not 0 (clean fixture): $OUT"
  AFTER_STATUS="$(cd "$TMP" && git status --porcelain)"
  AFTER_HEAD="$(cd "$TMP" && git rev-parse HEAD)"
  [ -z "$AFTER_STATUS" ] && [ "$BEFORE_STATUS" = "$AFTER_STATUS" ] \
    && ok "mutation audit: the swept tree is byte-clean before and after" \
    || bad "the sweep mutated the working tree: [$AFTER_STATUS]"
  [ "$BEFORE_HEAD" = "$AFTER_HEAD" ] && ok "mutation audit: HEAD is unchanged" || bad "the sweep moved HEAD"
fi

echo "$PASS pass, $FAIL fail"
[ "$FAIL" -eq 0 ]
