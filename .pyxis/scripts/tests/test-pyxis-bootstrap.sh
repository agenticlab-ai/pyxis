#!/usr/bin/env bash
# test-pyxis-bootstrap.sh — the pyxis bootstrap shim (the ONE shell
# first-mover, installer-v1 §3.4; plan T11) stays delegate-only,
# bash-3.2-clean, and fail-closed: it fetch-verifies pyxis against the
# documented pin BEFORE anything executes, refuses a tamper naming expected
# vs actual, and execs nothing itself. Hermetic: file:// fixtures only.
# Runs standalone and via the Go harness (internal/release/shim_test.go).
set -u
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
SHIM="$REPO_ROOT/.pyxis/scripts/pyxis-bootstrap.sh"
PASS=0; FAIL=0
ok()  { PASS=$((PASS+1)); printf 'ok   - %s\n' "$1"; }
bad() { FAIL=$((FAIL+1)); printf 'FAIL - %s\n' "$1"; }

[ -f "$SHIM" ] || { bad "shim missing at $SHIM"; printf '\n%d passed, %d failed\n' "$PASS" "$FAIL"; exit 1; }

# Parses under both POSIX sh and stock (3.2-floor) bash.
sh -n "$SHIM" && ok "shim parses (sh -n)" || bad "shim fails sh -n"
/bin/bash -n "$SHIM" && ok "shim parses (stock bash)" || bad "shim fails bash -n"

# No bash-4-only syntax (macOS stock bash floor).
grep -qE '\$\{[a-zA-Z_]+(\^\^|,,)\}|;&' "$SHIM" && bad "shim has bash-4-only syntax" || ok "shim: no bash-4-only syntax"

# Thin, delegate-only: fetch + verify + install, no grown logic, and it
# EXECS NOTHING (unlike the retired per-tool resolvers, the bootstrap hands
# off by installing — the caller puts .pyxis/bin on PATH).
CODE_LINES=$(grep -cv '^[[:space:]]*#\|^[[:space:]]*$' "$SHIM")
# Two additional fetch-only instructions validate a token and pass its header
# on stdin. Authentication is needed before the verified Go binary exists.
[ "$CODE_LINES" -le 47 ] && ok "shim stays thin ($CODE_LINES code lines)" || bad "shim has grown logic ($CODE_LINES code lines)"
grep -v '^[[:space:]]*#' "$SHIM" | grep -qE '(^|[;&| ])exec ' && bad "shim execs something (delegate-only means install-only)" || ok "shim execs nothing"

# Fixture platform + a fixture artifact served over file://.
PLAT="$(uname | tr "[:upper:]" "[:lower:]")-$(uname -m | sed "s/x86_64/amd64/;s/aarch64/arm64/")"
PLAT_KEY="$(printf '%s' "$PLAT" | tr 'a-z-' 'A-Z_')"
VER="v9.9.9"
SB="$(mktemp -d "${TMPDIR:-/tmp}/pyxis-bootstrap-test.XXXXXXXX")"
SRC="$SB/src"; REPO="$SB/repo"
mkdir -p "$SRC" "$REPO"
printf 'fixture pyxis binary bytes\n' > "$SRC/pyxis-$VER-$PLAT"
if command -v shasum >/dev/null 2>&1; then
  WANT="$(shasum -a 256 "$SRC/pyxis-$VER-$PLAT" | cut -d' ' -f1)"
else
  WANT="$(sha256sum "$SRC/pyxis-$VER-$PLAT" | cut -d' ' -f1)"
fi

# Happy path (env pins): fetch-verify into .pyxis/bin + the stable entry.
if (cd "$REPO" && env PYXIS_VERSION="$VER" "PYXIS_SHA256_$PLAT_KEY=$WANT" PYXIS_ARTIFACT_SOURCE="file://$SRC" bash "$SHIM" >/dev/null 2>&1); then
  ok "shim fetch-verifies against the documented pin (env)"
else
  bad "shim failed a clean env-pinned fetch"
fi
[ -f "$REPO/.pyxis/bin/pyxis-$VER-$PLAT" ] && ok "versioned binary installed" || bad "versioned binary missing"
[ -e "$REPO/.pyxis/bin/pyxis" ] && ok "stable pyxis entry installed beside it" || bad "stable pyxis entry missing"
[ -x "$REPO/.pyxis/bin/pyxis-$VER-$PLAT" ] && ok "installed binary is executable" || bad "installed binary not executable"

# Warm cache: with the artifact source GONE, a re-run re-hashes and skips
# the fetch (the workflow README's warm-runner contract).
rm -f "$SRC/pyxis-$VER-$PLAT"
if (cd "$REPO" && env PYXIS_VERSION="$VER" "PYXIS_SHA256_$PLAT_KEY=$WANT" PYXIS_ARTIFACT_SOURCE="file://$SRC" bash "$SHIM" >/dev/null 2>&1); then
  ok "warm cache re-hashes and skips the fetch"
else
  bad "warm-cache re-run failed (it must not refetch)"
fi

# Args parity: --version/--sha256 replace the env pins.
REPO2="$SB/repo2"; mkdir -p "$REPO2"
printf 'fixture pyxis binary bytes\n' > "$SRC/pyxis-$VER-$PLAT"
if (cd "$REPO2" && env PYXIS_ARTIFACT_SOURCE="file://$SRC" bash "$SHIM" --version "$VER" --sha256 "$WANT" >/dev/null 2>&1); then
  ok "shim accepts --version/--sha256 args"
else
  bad "shim failed the args-pinned fetch"
fi

# Tamper refusal (DINV-1): a wrong pin refuses with zero installation,
# naming expected and actual.
REPO3="$SB/repo3"; mkdir -p "$REPO3"
BADPIN="0000000000000000000000000000000000000000000000000000000000000000"
ERR="$( (cd "$REPO3" && env PYXIS_VERSION="$VER" "PYXIS_SHA256_$PLAT_KEY=$BADPIN" PYXIS_ARTIFACT_SOURCE="file://$SRC" bash "$SHIM" 2>&1 >/dev/null) )" \
  && bad "shim accepted a tampered binary" || ok "shim refuses on pin mismatch"
[ ! -e "$REPO3/.pyxis/bin/pyxis-$VER-$PLAT" ] && ok "nothing installed on mismatch" || bad "tampered binary left installed"
printf '%s' "$ERR" | grep -q "$BADPIN" && printf '%s' "$ERR" | grep -q "$WANT" && ok "refusal names expected and actual" || bad "refusal does not name expected/actual"

# Missing pin refusal: no env, no args → fail closed, naming the pin inputs.
ERR="$( (cd "$REPO3" && env PYXIS_ARTIFACT_SOURCE="file://$SRC" bash "$SHIM" 2>&1 >/dev/null) )" \
  && bad "shim ran with no pin at all" || ok "shim refuses without a pin"
printf '%s' "$ERR" | grep -q "PYXIS_VERSION" && ok "missing-pin refusal names PYXIS_VERSION" || bad "missing-pin refusal unnamed"

# Unknown argument refusal (the I3 discipline at the shell edge).
(cd "$REPO3" && bash "$SHIM" --nope 2>/dev/null) && bad "shim accepted an unknown argument" || ok "shim refuses unknown arguments"

# The private-host gh rung (canonical source override): when curl cannot
# serve a github.com base, an authenticated gh fetches via the API asset
# route. Hermetic: PATH stubs — curl always fails, gh logs its argv and
# delivers the fixture to --output; the sha pin still gates acceptance.
REPO4="$SB/repo4"; STUB="$SB/stub"; mkdir -p "$REPO4" "$STUB"
printf '#!/bin/sh\nexit 22\n' > "$STUB/curl"
cat > "$STUB/gh" <<'EOF'
#!/bin/sh
echo "$*" >> "${STUB_LOG:?}"
out=""
while [ $# -gt 0 ]; do [ "$1" = "--output" ] && out="$2"; shift; done
[ -n "$out" ] && cp "${STUB_PAYLOAD:?}" "$out"
EOF
chmod +x "$STUB/curl" "$STUB/gh"
if (cd "$REPO4" && env PATH="$STUB:$PATH" STUB_LOG="$SB/gh.log" STUB_PAYLOAD="$SRC/pyxis-$VER-$PLAT" \
      PYXIS_VERSION="$VER" "PYXIS_SHA256_$PLAT_KEY=$WANT" \
      PYXIS_ARTIFACT_SOURCE="https://github.com/agenticlab-ai/pyxis/releases/download/$VER" \
      bash "$SHIM" >/dev/null 2>&1); then
  ok "gh rung fetch-verifies from a private github base"
else
  bad "gh rung failed against the stubbed private base"
fi
grep -q -- "release download $VER --repo agenticlab-ai/pyxis --pattern pyxis-$VER-$PLAT" "$SB/gh.log" 2>/dev/null \
  && ok "gh rung derives owner/repo + tag from the base URL" || bad "gh rung argv wrong"
# A non-github base must never reach gh (the rung is github-scoped).
REPO5="$SB/repo5"; mkdir -p "$REPO5"; : > "$SB/gh.log"
(cd "$REPO5" && env PATH="$STUB:$PATH" STUB_LOG="$SB/gh.log" STUB_PAYLOAD="$SRC/pyxis-$VER-$PLAT" \
  PYXIS_VERSION="$VER" "PYXIS_SHA256_$PLAT_KEY=$WANT" \
  PYXIS_ARTIFACT_SOURCE="https://mirror.example.com/pyxis" bash "$SHIM" >/dev/null 2>&1) \
  && bad "shim fetched from a dead non-github mirror" || ok "non-github mirror fails without a gh attempt"
[ -s "$SB/gh.log" ] && bad "gh rung fired for a non-github base" || ok "gh rung stays github-scoped"

# An https mirror fetches anonymously, with no credential option on curl.
# Hermetic via a PATH-stubbed curl that logs argv and delivers.
cat > "$STUB/curl" <<'EOF'
#!/bin/sh
echo "$*" >> "${CURL_LOG:?}"
out=""; prev=""
for a in "$@"; do [ "$prev" = "-o" ] && out="$a"; prev="$a"; done
[ -n "$out" ] && cp "${STUB_PAYLOAD:?}" "$out"
EOF
chmod +x "$STUB/curl"
REPO6="$SB/repo6"; mkdir -p "$REPO6"; : > "$SB/curl.log"
if (cd "$REPO6" && env PATH="$STUB:$PATH" CURL_LOG="$SB/curl.log" STUB_PAYLOAD="$SRC/pyxis-$VER-$PLAT" \
      PYXIS_VERSION="$VER" "PYXIS_SHA256_$PLAT_KEY=$WANT" \
      PYXIS_ARTIFACT_SOURCE="https://mirror.example/rel" bash "$SHIM" >/dev/null 2>&1); then
  ok "mirror fetch verifies via stubbed curl"
else
  bad "mirror fetch failed"
fi
grep -q -- "-u " "$SB/curl.log" && bad "curl got a -u credential" || ok "no -u on the mirror fetch"

# With NO PYXIS_ARTIFACT_SOURCE the shim reads the public GitHub release: an
# operand's CI runner and its hooks carry no artifact-source of their own, so
# this default is the one they get. It needs no credential and no gh.
REPO8="$SB/repo8"; mkdir -p "$REPO8"; : > "$SB/curl.log"; : > "$SB/gh.log"
(cd "$REPO8" && env -u PYXIS_ARTIFACT_SOURCE PATH="$STUB:$PATH" CURL_LOG="$SB/curl.log" STUB_LOG="$SB/gh.log" STUB_PAYLOAD="$SRC/pyxis-$VER-$PLAT" \
  PYXIS_VERSION="$VER" "PYXIS_SHA256_$PLAT_KEY=$WANT" \
  bash "$SHIM" >/dev/null 2>&1) || bad "default-channel fetch failed"
grep -q "https://github.com/agenticlab-ai/pyxis/releases/download/$VER/pyxis-$VER-$PLAT" "$SB/curl.log" \
  && ok "the default base is the public GitHub release" || bad "the default base is not the public GitHub release"
[ -s "$SB/gh.log" ] && bad "the default fetch called gh" || ok "the default fetch never calls gh"

rm -rf "$SB"
printf '\n%d passed, %d failed\n' "$PASS" "$FAIL"
[ "$FAIL" -eq 0 ]
