#!/bin/sh
# pyxis-bootstrap — the ONE shell first-mover (installer-v1 §3.4; plan T11).
# Bash is admissible here as the named bootstrap exception: this must run
# before any Go binary exists. It fetches the `pyxis` orchestrator and
# sha256-verifies it BEFORE any execution against THE DOCUMENTED PIN — the
# version + sha256 the install instructions themselves carry, an anchor
# deliberately independent of the release manifest (which pyxis is needed
# to parse). Delegate-only: verify → install into .pyxis/bin/ → exit; it
# EXECUTES NOTHING (the caller puts .pyxis/bin on PATH). bash-3.2-clean,
# POSIX-sh compatible; covered by scripts/tests/test-pyxis-bootstrap.sh.
#
# Pin inputs (from the install instructions — design §3.4):
#   PYXIS_VERSION             the system tag, e.g. v1.2.3   (or --version)
#   PYXIS_SHA256_<PLATFORM>   e.g. PYXIS_SHA256_DARWIN_ARM64=<hex>
#                             for this machine's platform   (or --sha256)
#   PYXIS_ARTIFACT_SOURCE     optional mirror base URL (FR-22); artifacts
#                             resolve flat at <base>/<name>. A mirror can
#                             serve bytes, never choose them (§3.9 leg 1).
#                             For a PRIVATE github.com base the plain download
#                             URL ignores credentials — an authenticated gh on
#                             PATH is the fallback fetch rung there. Default:
#                             this project's public GitHub release for VERSION.
#   PYXIS_ARTIFACT_TOKEN      optional access token for the selected Forgejo
#                             release source; passed to curl through stdin.
set -eu
VERSION="${PYXIS_VERSION:-}"; SHA=""
while [ $# -gt 0 ]; do
  case "$1" in
    --version) [ $# -ge 2 ] || { echo "pyxis-bootstrap: --version needs a value" >&2; exit 2; }; VERSION="$2"; shift 2 ;;
    --sha256)  [ $# -ge 2 ] || { echo "pyxis-bootstrap: --sha256 needs a value" >&2; exit 2; }; SHA="$2"; shift 2 ;;
    *) echo "pyxis-bootstrap: unknown argument $1 (only --version/--sha256; nothing was fetched)" >&2; exit 2 ;;
  esac
done
OS="$(uname | tr '[:upper:]' '[:lower:]')"; ARCH="$(uname -m)"
[ "$ARCH" = "x86_64" ] && ARCH="amd64"; [ "$ARCH" = "aarch64" ] && ARCH="arm64"
PLAT="$OS-$ARCH"
KEY="PYXIS_SHA256_$(printf '%s' "$PLAT" | tr 'a-z-' 'A-Z_')"
[ -n "$SHA" ] || SHA="$(eval "printf '%s' \"\${$KEY:-}\"")"
[ -n "$VERSION" ] || { echo "pyxis-bootstrap: no version pin — set PYXIS_VERSION or pass --version (the pin documented in the install instructions; installer-v1 §3.4)" >&2; exit 2; }
[ -n "$SHA" ] || { echo "pyxis-bootstrap: no sha256 pin for $PLAT — set $KEY or pass --sha256 (the pin documented in the install instructions; DINV-1, fail closed)" >&2; exit 2; }
ROOT="$(git rev-parse --show-toplevel 2>/dev/null)" || ROOT=""
[ -n "$ROOT" ] || ROOT="$(pwd)"
BIN_DIR="$ROOT/.pyxis/bin"; NAME="pyxis-$VERSION-$PLAT"; BIN="$BIN_DIR/$NAME"
# Select the digest tool ONCE. Written as `shasum … | cut … || sha256sum …`
# the fallback bound to the PIPELINE, whose status is cut's: on an image with
# no shasum (slim Ubuntu carries sha256sum but not the perl that ships it) the
# fallback never fired, sum returned nothing, and the empty digest was reported
# as a checksum mismatch against bytes that were intact.
if command -v sha256sum >/dev/null 2>&1; then sum() { sha256sum "$1" | cut -d' ' -f1; }
elif command -v shasum >/dev/null 2>&1; then sum() { shasum -a 256 "$1" | cut -d' ' -f1; }
elif command -v openssl >/dev/null 2>&1; then sum() { openssl dgst -sha256 "$1" | awk '{print $NF}'; }
else echo "pyxis-bootstrap: no sha256 tool found — need sha256sum, shasum, or openssl; refusing to install unverified bytes (DINV-1, fail closed)" >&2; exit 2; fi
# Warm cache: re-hash and skip the fetch when the cached binary still pins.
if [ ! -f "$BIN" ] || [ "$(sum "$BIN")" != "$SHA" ]; then
  BASE="${PYXIS_ARTIFACT_SOURCE:-https://github.com/agenticlab-ai/pyxis/releases/download/$VERSION}"
  URL="$BASE/$NAME"
  mkdir -p "$BIN_DIR"
  # The caller supplies the exact source and its credential; curl reads the
  # header on stdin so the token never appears in process arguments.
  case "${PYXIS_ARTIFACT_TOKEN:-}" in *[!A-Za-z0-9_.-]*) echo "pyxis-bootstrap: malformed artifact token" >&2; exit 2 ;; esac
  { [ -z "${PYXIS_ARTIFACT_TOKEN:-}" ] || printf 'header = "Authorization: token %s"\n' "$PYXIS_ARTIFACT_TOKEN"; } |
    curl --config - -fsSL -o "$BIN.tmp" "$URL" 2>/dev/null || rm -f "$BIN.tmp"
  # Private github release host: curl's plain download URL 404s regardless
  # of tokens, so fall back to an authenticated gh via the API asset route.
  if [ ! -f "$BIN.tmp" ] && command -v gh >/dev/null 2>&1; then
    case "$BASE" in
      https://github.com/*/*/releases/download/*)
        R="${BASE#https://github.com/}"
        TAG="${R#*/*/releases/download/}"
        gh release download "$TAG" --repo "${R%%/releases/download/*}" --pattern "$NAME" --output "$BIN.tmp" 2>/dev/null || rm -f "$BIN.tmp" ;;
    esac
  fi
  [ -f "$BIN.tmp" ] || { echo "pyxis-bootstrap: fetch failed: $URL (check the source and its PYXIS_ARTIFACT_TOKEN; private GitHub sources may use authenticated gh; DINV-1 — nothing was executed)" >&2; exit 2; }
  GOT="$(sum "$BIN.tmp")"
  [ "$GOT" = "$SHA" ] || { rm -f "$BIN.tmp"; echo "pyxis-bootstrap: sha256 mismatch for $NAME — expected $SHA, actual $GOT; refusing with zero execution (DINV-1, fail closed)" >&2; exit 2; }
  chmod +x "$BIN.tmp" && mv "$BIN.tmp" "$BIN"
fi
# The stable entry name beside the versioned binary (symlink, copy fallback).
ln -sf "$NAME" "$BIN_DIR/pyxis" 2>/dev/null || cp -f "$BIN" "$BIN_DIR/pyxis"
echo "pyxis-bootstrap: $NAME verified into .pyxis/bin (sha256 $SHA)"
