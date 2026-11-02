#!/usr/bin/env bash
# .pyxis/vendor/mermaid/refresh.sh — re-vendor the pinned mermaid browser bundle.
#
# mermaid.min.js is inlined by the Go plan viewer into standalone HTML,
# so diagrams work offline (no CDN, no fetch at render time). This
# script is the ONE reproducible way to update that vendored copy: bump
# MERMAID_VERSION in .pyxis/scripts/versions.env, run this, review the diff, commit.
#
# VENDORING MODEL — one upstream-built file, zero install. dist/mermaid.min.js in
# the npm tarball is a self-contained browser IIFE (esbuild; exposes
# globalThis.mermaid) with zero runtime deps, so there is
# no lockfile and no install-time step: the committed file IS the runtime.
#
# Committed artifacts: mermaid.min.js + LICENSE + package.json (provenance + the
# embedded version, pin-checked by internal/plangate/render's tests).
#
# INLINE-SAFETY INVARIANTS — the renderer embeds this file inside a <script>
# element, so a bundle that contains `</script` (ends the element early) or
# `<script` (the HTML script-data double-escape hazard) needs review before
# committing. This script fails on either; the renderer also guards `</script`
# at render time as belt-and-braces.
#
# Requires (build-time only, on the refreshing box): curl + tar + node.
#
# Usage: bash .pyxis/vendor/mermaid/refresh.sh              # uses the versions.env pin
#        MERMAID_VERSION=11.16.0 bash .pyxis/vendor/mermaid/refresh.sh
set -eu

HERE="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$HERE/../../.." && pwd)"   # vendor/<x>/ lives under .pyxis/ (footprint phase 2)

# Pin source: .pyxis/scripts/versions.env (the single tool-version source), overridable by
# an inline MERMAID_VERSION for a bump-and-review run.
if [ -z "${MERMAID_VERSION:-}" ]; then
  versions_env="$REPO_ROOT/.pyxis/scripts/versions.env"
  [ -f "$versions_env" ] || { echo "refresh: $versions_env not found" >&2; exit 2; }
  # shellcheck disable=SC1090
  . "$versions_env"
fi
VER="${MERMAID_VERSION:-}"
[ -n "$VER" ] || { echo "refresh: MERMAID_VERSION is empty (set it or .pyxis/scripts/versions.env)" >&2; exit 2; }

command -v curl >/dev/null 2>&1 || { echo "refresh: curl required (build-time only)" >&2; exit 2; }
command -v tar  >/dev/null 2>&1 || { echo "refresh: tar required (build-time only)" >&2; exit 2; }
command -v node >/dev/null 2>&1 || { echo "refresh: node required (build-time only, syntax + version checks)" >&2; exit 2; }

# sha256_hex — portable sha256 (Linux: sha256sum; macOS: shasum -a 256).
sha256_hex() {
  if command -v sha256sum >/dev/null 2>&1; then sha256sum | cut -d' ' -f1
  else shasum -a 256 | cut -d' ' -f1; fi
}

echo "refresh: vendoring mermaid@${VER} into $HERE (single upstream-built file)"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

tarball="https://registry.npmjs.org/mermaid/-/mermaid-${VER}.tgz"
curl -fsSL "$tarball" -o "$tmp/mermaid.tgz" || { echo "refresh: download failed: $tarball" >&2; exit 1; }
tar -xzf "$tmp/mermaid.tgz" -C "$tmp"

src="$tmp/package"
[ -f "$src/dist/mermaid.min.js" ] || { echo "refresh: tarball missing dist/mermaid.min.js" >&2; exit 1; }
[ -f "$src/package.json" ]        || { echo "refresh: tarball missing package.json" >&2; exit 1; }
[ -f "$src/LICENSE" ]             || { echo "refresh: tarball missing LICENSE" >&2; exit 1; }

# Inline-safety invariants — checked against the CANDIDATE before it replaces the
# committed copy (fail here, not after a broken render ships).
if grep -q '</script' "$src/dist/mermaid.min.js"; then
  echo "refresh: FAIL — bundle contains '</script' (would end the inline <script> early)." >&2
  echo "refresh: the renderer guards this, but a bundle that needs the guard needs review first." >&2
  exit 1
fi
if grep -q '<script' "$src/dist/mermaid.min.js"; then
  echo "refresh: FAIL — bundle contains '<script' (HTML script-data double-escape hazard)." >&2
  exit 1
fi
node --check "$src/dist/mermaid.min.js" || { echo "refresh: FAIL — bundle does not parse as JS" >&2; exit 1; }
grep -q 'globalThis\["mermaid"\]' "$src/dist/mermaid.min.js" \
  || echo "refresh: WARNING — no globalThis[\"mermaid\"] export found; verify the renderer's init still works" >&2

# Content-hygiene tripwire: no build-machine paths in a vendored artifact
# (the harbormaster #851 lesson — review diff CONTENT, not only runtime behavior).
if grep -nE '(/private/var|/var/folders|/Users/|/home/|/tmp/tmp\.)' "$src/dist/mermaid.min.js" >/dev/null 2>&1; then
  echo "refresh: WARNING — build-machine paths found in the bundle (review before committing):" >&2
  grep -nE '(/private/var|/var/folders|/Users/|/home/|/tmp/tmp\.)' "$src/dist/mermaid.min.js" | head -5 >&2
fi

# Replace the vendored artifacts (idempotent: full re-vendor each run), all verbatim.
cp "$src/dist/mermaid.min.js" "$HERE/mermaid.min.js"
cp "$src/LICENSE"             "$HERE/LICENSE"
cp "$src/package.json"        "$HERE/package.json"

got="$(node -e 'process.stdout.write(JSON.parse(require("fs").readFileSync(process.argv[1],"utf8")).version)' "$HERE/package.json" 2>/dev/null || true)"
if [ "$got" != "$VER" ]; then
  echo "refresh: WARNING vendored package.json version ($got) != requested ($VER)" >&2
fi

sha="$(sha256_hex < "$HERE/mermaid.min.js")"
echo "refresh: done — vendored mermaid@${got}."
echo "refresh: sha256(mermaid.min.js) = $sha  — record this in .pyxis/vendor/mermaid/README.md."
echo "refresh: review the diff, run go test ./internal/plangate/render, then commit."
