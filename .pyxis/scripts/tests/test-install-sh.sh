#!/usr/bin/env bash
# Hermetic test for install.sh — the one-liner machine installer. Exercises the
# file:// artifact-source leg (no gh, no network) and the fail-closed checksum
# refusal. bash-3.2-clean (macOS stock bash floor).
set -uo pipefail

ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"
INSTALL="$ROOT/install.sh"
[ -f "$INSTALL" ] || { echo "FAIL: install.sh not found at $INSTALL" >&2; exit 1; }
fail() { echo "FAIL: $*" >&2; exit 1; }
# The suite must not compute digests the way install.sh used to: that idiom
# yields an empty string when the tool is missing, and "" = "" compares equal.
digest() {
  if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1" | cut -d' ' -f1
  elif command -v shasum >/dev/null 2>&1; then shasum -a 256 "$1" | cut -d' ' -f1
  else fail "no sha256 tool on this host"; fi
}

# The current machine's platform in the release's asset naming.
OS="$(uname | tr 'A-Z' 'a-z')"
ARCH="$(uname -m)"
case "$ARCH" in x86_64|amd64) ARCH="amd64" ;; aarch64|arm64) ARCH="arm64" ;; esac
PLAT="$OS-$ARCH"
# Track whatever version install.sh pins, so a release bump doesn't break this.
VERSION="$(sed -n 's/^PINNED_VERSION="\(.*\)"/\1/p' "$INSTALL")"
[ -n "$VERSION" ] || fail "could not read PINNED_VERSION from install.sh"
NAME="pyxis-$VERSION-$PLAT"

TMP="$(mktemp -d "${TMPDIR:-/tmp}/test-install.XXXXXX")"
trap 'rm -rf "$TMP"' EXIT
SRC="$TMP/rel"
mkdir -p "$SRC"
printf '#!/bin/sh\nexit 2\n' > "$SRC/$NAME" # Legacy CLI without shell setup.
WANT="$(digest "$SRC/$NAME")"
[ -n "$WANT" ] || fail "could not digest the fixture"

# 1. Happy path: file:// source + correct sha installs the CLI verbatim and
#    records the release pin (Piece #2) for a flag-free `pyxis init`.
DEST="$TMP/bin"
XDG="$TMP/config"
PYXIS_ARTIFACT_SOURCE="file://$SRC" PYXIS_SHA256="$WANT" PYXIS_INSTALL_DIR="$DEST" XDG_CONFIG_HOME="$XDG" \
  sh "$INSTALL" >/dev/null 2>&1 || fail "happy path exited non-zero"
[ -x "$DEST/pyxis" ] || fail "pyxis not installed"
GOT="$(digest "$DEST/pyxis")"
[ "$GOT" = "$WANT" ] || fail "installed bytes differ from source"
# release.env carries the version + the manifest pin install.sh ships.
ENV="$XDG/pyxis/release.env"
[ -f "$ENV" ] || fail "release.env not recorded at $ENV"
grep -q "^PYXIS_SYSTEM_VERSION=$VERSION$" "$ENV" || fail "release.env missing the version pin"
grep -q "^PYXIS_MANIFEST_SHA256=[0-9a-f]\{64\}$" "$ENV" || fail "release.env missing a 64-hex manifest pin"

# 2. Fail-closed: a wrong pin refuses and installs nothing.
DEST2="$TMP/bin2"
if PYXIS_ARTIFACT_SOURCE="file://$SRC" \
   PYXIS_SHA256="deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef" \
   PYXIS_INSTALL_DIR="$DEST2" sh "$INSTALL" >/dev/null 2>&1; then
  fail "checksum-mismatch path succeeded (want refusal)"
fi
[ ! -e "$DEST2/pyxis" ] || fail "checksum-mismatch path installed a binary"

# 3. Fail-closed: an unknown version with no override pin refuses by name.
if PYXIS_VERSION="v9.9.9" PYXIS_ARTIFACT_SOURCE="file://$SRC" \
   PYXIS_INSTALL_DIR="$TMP/bin3" sh "$INSTALL" >/dev/null 2>&1; then
  fail "unknown-version path succeeded (want refusal)"
fi

# 4. An https mirror fetches anonymously — no credential option reaches curl.
#    Hermetic via a PATH-stubbed curl and a gh stub that records any call.
STUB="$TMP/stub"; mkdir -p "$STUB"
cat > "$STUB/curl" <<'EOF'
#!/bin/sh
echo "$*" >> "${CURL_LOG:?}"
[ -z "${CURL_FAIL:-}" ] || exit 22
out=""; prev=""
for a in "$@"; do [ "$prev" = "-o" ] && out="$a"; prev="$a"; done
[ -n "$out" ] && cp "${CURL_PAYLOAD:?}" "$out"
EOF
cat > "$STUB/gh" <<'EOF'
#!/bin/sh
echo "$*" >> "${GH_LOG:?}"
dir=""; prev=""
for a in "$@"; do [ "$prev" = "--dir" ] && dir="$a"; prev="$a"; done
[ -n "$dir" ] && cp "${CURL_PAYLOAD:?}" "$dir/${GH_NAME:?}"
EOF
chmod +x "$STUB/curl" "$STUB/gh"
: > "$TMP/curl.log"; : > "$TMP/gh.log"
PATH="$STUB:$PATH" CURL_LOG="$TMP/curl.log" GH_LOG="$TMP/gh.log" CURL_PAYLOAD="$SRC/$NAME" \
  PYXIS_ARTIFACT_SOURCE="https://mirror.example/rel" \
  PYXIS_SHA256="$WANT" PYXIS_INSTALL_DIR="$TMP/bin5" XDG_CONFIG_HOME="$TMP/config5" \
  sh "$INSTALL" >/dev/null 2>&1 || fail "mirror https path exited non-zero"
grep -q "https://mirror.example/rel/$NAME" "$TMP/curl.log" || fail "curl did not fetch the mirror's asset"
if grep -q -- "-u " "$TMP/curl.log"; then fail "curl got a -u credential"; fi

# 4a. The default: a new user with no credential and no gh login. The public
#     release asset rides curl, gh is never touched, and no channel is recorded
#     because the public default needs none.
: > "$TMP/curl.log"; : > "$TMP/gh.log"
CFG_P="$TMP/configP"
env -u PYXIS_ARTIFACT_SOURCE -u PYXIS_CHANNEL -u PYXIS_RELEASE_TOKEN_ENV -u PYXIS_RELEASE_REPOSITORY \
  PATH="$STUB:$PATH" CURL_LOG="$TMP/curl.log" GH_LOG="$TMP/gh.log" CURL_PAYLOAD="$SRC/$NAME" \
  PYXIS_SHA256="$WANT" PYXIS_INSTALL_DIR="$TMP/binP" XDG_CONFIG_HOME="$CFG_P" \
  sh "$INSTALL" >/dev/null 2>&1 || fail "public default install exited non-zero"
grep -q "https://github.com/agenticlab-ai/pyxis/releases/download/$VERSION/$NAME" "$TMP/curl.log" \
  || fail "the default did not fetch the public release asset: $(cat "$TMP/curl.log")"
[ ! -s "$TMP/gh.log" ] || fail "the default install called gh: $(cat "$TMP/gh.log")"
[ "$(digest "$TMP/binP/pyxis")" = "$WANT" ] || fail "the default install wrote the wrong bytes"
if grep -q "PYXIS_CHANNEL\|PYXIS_ARTIFACT_SOURCE\|PYXIS_RELEASE_" "$CFG_P/pyxis/release.env"; then
  fail "the public default recorded a source: $(cat "$CFG_P/pyxis/release.env")"
fi

# 4b. An unpublished release refuses by name and installs nothing.
: > "$TMP/curl.log"
out="$(env -u PYXIS_ARTIFACT_SOURCE -u PYXIS_RELEASE_TOKEN_ENV \
  PATH="$STUB:$PATH" CURL_FAIL=1 CURL_LOG="$TMP/curl.log" GH_LOG="$TMP/gh.log" CURL_PAYLOAD="$SRC/$NAME" \
  PYXIS_SHA256="$WANT" PYXIS_INSTALL_DIR="$TMP/binQ" XDG_CONFIG_HOME="$TMP/configQ" \
  sh "$INSTALL" 2>&1)"
code=$?
[ "$code" -eq 2 ] || fail "failed public fetch exited $code, want 2"
case "$out" in *"could not fetch https://github.com/agenticlab-ai/pyxis/releases/download/$VERSION/$NAME"*) ;; *) fail "failed public fetch is not named: $out" ;; esac
[ ! -e "$TMP/binQ/pyxis" ] || fail "a failed public fetch installed a binary"

# 4c. A private fork that names a token is the one case still reading through
#     gh, because a private asset's download URL 404s whatever the credential.
: > "$TMP/curl.log"; : > "$TMP/gh.log"
env -u PYXIS_ARTIFACT_SOURCE PATH="$STUB:$PATH" CURL_LOG="$TMP/curl.log" GH_LOG="$TMP/gh.log" CURL_PAYLOAD="$SRC/$NAME" GH_NAME="$NAME" \
  PYXIS_RELEASE_REPOSITORY="o/private" PYXIS_RELEASE_TOKEN_ENV="FORK_TOKEN" FORK_TOKEN="fork-secret" \
  PYXIS_SHA256="$WANT" PYXIS_INSTALL_DIR="$TMP/binR" XDG_CONFIG_HOME="$TMP/configR" \
  sh "$INSTALL" >/dev/null 2>&1 || fail "private-fork install exited non-zero"
grep -q "release download $VERSION --repo o/private --pattern $NAME" "$TMP/gh.log" || fail "the private fork did not read through gh"
[ ! -s "$TMP/curl.log" ] || fail "the private fork fetched over curl: $(cat "$TMP/curl.log")"
if grep -q "fork-secret" "$TMP/configR/pyxis/release.env"; then fail "the token value reached release.env"; fi
out="$(env -u PYXIS_ARTIFACT_SOURCE -u FORK_TOKEN PATH="$STUB:$PATH" CURL_LOG="$TMP/curl.log" GH_LOG="$TMP/gh.log" \
  PYXIS_RELEASE_REPOSITORY="o/private" PYXIS_RELEASE_TOKEN_ENV="FORK_TOKEN" \
  PYXIS_SHA256="$WANT" PYXIS_INSTALL_DIR="$TMP/binS" XDG_CONFIG_HOME="$TMP/configS" \
  sh "$INSTALL" 2>&1)" && fail "a named but empty token proceeded"
case "$out" in *"configured GitHub credential is missing"*) ;; *) fail "the missing token is not named: $out" ;; esac

# 5. Regression: an image carrying sha256sum but NOT shasum — slim Ubuntu,
#    where shasum arrives with perl. The old fallback was written
#    `shasum … | cut … || sha256sum … | cut …`, which binds || to the
#    pipeline, whose status is cut's; it never fired, so sum returned nothing
#    and intact bytes were refused as a checksum mismatch with a blank "got".
BOX="$TMP/box"; mkdir -p "$BOX"
for t in sh uname tr mktemp cp cut mkdir chmod mv rm sed grep head cat; do
  src="$(command -v "$t" 2>/dev/null)" && ln -sf "$src" "$BOX/$t"
done
if command -v sha256sum >/dev/null 2>&1; then
  ln -sf "$(command -v sha256sum)" "$BOX/sha256sum"
else
  printf '#!/bin/sh\nexec %s -a 256 "$1"\n' "$(command -v shasum)" > "$BOX/sha256sum"
  chmod +x "$BOX/sha256sum"
fi
[ ! -e "$BOX/shasum" ] || fail "the sandbox PATH must not carry shasum"
[ ! -e "$BOX/openssl" ] || fail "the sandbox PATH must not carry openssl"
DEST6="$TMP/bin6"
env -i PATH="$BOX" HOME="$TMP" TMPDIR="$TMP" \
  PYXIS_ARTIFACT_SOURCE="file://$SRC" PYXIS_SHA256="$WANT" \
  PYXIS_INSTALL_DIR="$DEST6" XDG_CONFIG_HOME="$TMP/config6" \
  sh "$INSTALL" >/dev/null 2>&1 || fail "an image with sha256sum but no shasum failed to install"
[ -x "$DEST6/pyxis" ] || fail "sha256sum-only image installed nothing"
[ "$(digest "$DEST6/pyxis")" = "$WANT" ] || fail "sha256sum-only image installed the wrong bytes"

# 6. No digest tool at all: a NAMED refusal, exit 2, nothing installed —
#    never a mismatch reported against an empty digest.
rm -f "$BOX/sha256sum"
DEST7="$TMP/bin7"
out="$(env -i PATH="$BOX" HOME="$TMP" TMPDIR="$TMP" \
  PYXIS_ARTIFACT_SOURCE="file://$SRC" PYXIS_SHA256="$WANT" \
  PYXIS_INSTALL_DIR="$DEST7" sh "$INSTALL" 2>&1)"
code=$?
[ "$code" -eq 2 ] || fail "no-digest-tool path exited $code, want 2"
case "$out" in *"no sha256 tool found"*) ;; *) fail "no-digest-tool refusal is not named: $out" ;; esac
case "$out" in *"mismatch"*) fail "no-digest-tool path reported a checksum mismatch: $out" ;; esac
[ ! -e "$DEST7/pyxis" ] || fail "no-digest-tool path installed a binary"

# 7. The channel record (prd:installer/FR-30): the installer writes the channel
#    ROOT beside the pin, so `pyxis init` needs no --artifact-source. It must
#    be a root — a recorded <root>/<tag> would strand the next upgrade on this
#    tag.
TREE="$TMP/tree/$VERSION"; mkdir -p "$TREE"; cp "$SRC/$NAME" "$TREE/"
channel_of() { sed -n 's/^PYXIS_CHANNEL=//p' "$1/pyxis/release.env"; }

# 7a. An explicit PYXIS_CHANNEL is what a flat source cannot supply: the
#     operator names the root, so upgrades compose against it even though this
#     fetch was not laid out by tag. It replaces the flat record rather than
#     joining it.
CFG_A="$TMP/cfgA"
PYXIS_CHANNEL="https://dist.example" PYXIS_ARTIFACT_SOURCE="file://$SRC" \
  PYXIS_SHA256="$WANT" PYXIS_INSTALL_DIR="$TMP/binA" XDG_CONFIG_HOME="$CFG_A" \
  sh "$INSTALL" >/dev/null 2>&1 || fail "explicit-channel install exited non-zero"
[ "$(channel_of "$CFG_A")" = "https://dist.example" ] || fail "explicit channel not recorded: $(channel_of "$CFG_A")"
[ -z "$(sed -n 's/^PYXIS_ARTIFACT_SOURCE=//p' "$CFG_A/pyxis/release.env")" ] || fail "an explicit channel left a flat source beside it"

# 7b. No explicit channel: the root is the source minus its trailing /<version>.
CFG_B="$TMP/cfgB"
env -u PYXIS_CHANNEL PYXIS_ARTIFACT_SOURCE="file://$TREE" \
  PYXIS_SHA256="$WANT" PYXIS_INSTALL_DIR="$TMP/binB" XDG_CONFIG_HOME="$CFG_B" \
  sh "$INSTALL" >/dev/null 2>&1 || fail "derived-channel install exited non-zero"
[ "$(channel_of "$CFG_B")" = "file://$TMP/tree" ] || fail "channel not derived from the source: $(channel_of "$CFG_B")"
case "$(channel_of "$CFG_B")" in
  *"$VERSION"*) fail "the recorded channel carries the version: $(channel_of "$CFG_B")" ;;
esac

# 7c. A flat mirror — an air-gapped copy, a one-release directory — has no
#     root to record, so the SOURCE is recorded instead. Recording nothing
#     would leave the offline adopter's next init reaching for the release
#     host, which on a locked-down network is the one place it cannot go.
CFG_C="$TMP/cfgC"
source_of() { sed -n 's/^PYXIS_ARTIFACT_SOURCE=//p' "$1/pyxis/release.env"; }
env -u PYXIS_CHANNEL PYXIS_ARTIFACT_SOURCE="file://$SRC" \
  PYXIS_SHA256="$WANT" PYXIS_INSTALL_DIR="$TMP/binC" XDG_CONFIG_HOME="$CFG_C" \
  sh "$INSTALL" >/dev/null 2>&1 || fail "flat-mirror install exited non-zero"
[ -z "$(channel_of "$CFG_C")" ] || fail "a flat mirror recorded a channel: $(channel_of "$CFG_C")"
[ "$(source_of "$CFG_C")" = "file://$SRC" ] || fail "flat mirror did not record its source: $(source_of "$CFG_C")"
grep -q "^PYXIS_SYSTEM_VERSION=$VERSION$" "$CFG_C/pyxis/release.env" || fail "flat-mirror install recorded no pin"

# 7d. The record describes where the bytes CAME FROM: a stale PYXIS_CHANNEL in
#     the operator's shell must not override a root we derived from the source
#     we actually fetched.
CFG_D="$TMP/cfgD"
PYXIS_CHANNEL="https://stale.example" PYXIS_ARTIFACT_SOURCE="file://$TREE" \
  PYXIS_SHA256="$WANT" PYXIS_INSTALL_DIR="$TMP/binD" XDG_CONFIG_HOME="$CFG_D" \
  sh "$INSTALL" >/dev/null 2>&1 || fail "derived-over-ambient install exited non-zero"
[ "$(channel_of "$CFG_D")" = "file://$TMP/tree" ] || fail "an ambient PYXIS_CHANNEL overrode the real source: $(channel_of "$CFG_D")"

# 7e. A tag-laid-out source records a root and NO flat source — one answer per
#     machine, never two that can disagree.
[ -z "$(source_of "$CFG_B")" ] || fail "a channelled install also recorded a flat source: $(source_of "$CFG_B")"

echo "test-install-sh: OK"
