#!/usr/bin/env bash
# .pyxis/vendor/litellm-prices/refresh.sh — re-vendor the pinned LiteLLM per-model
# price map.
#
# model_prices_and_context_window.json is LiteLLM's actively-maintained,
# cross-provider (Anthropic/OpenAI/…) per-model pricing + context-window table,
# consumed by the pipeline-measurement tool's cost derivation (design:
# pipeline-measurement-v2#3.12) so pyxis never self-maintains a price table that
# would rot. This script is the ONE reproducible way to update the vendored copy:
# bump LITELLM_PRICES_VERSION in .pyxis/scripts/versions.env, run this, review the
# diff, commit.
#
# VENDORING MODEL — one upstream data file, zero install. The file is plain JSON
# (parsed with stdlib encoding/json — vendored data, not a code dependency)
# fetched verbatim from a tagged release of github.com/BerriAI/litellm, so —
# like .pyxis/vendor/mermaid — there is no
# lockfile and no install-time step: the committed file IS the data.
#
# Pinning a release TAG (not `main`) means a deliberate, reviewed bump. Unlike an
# npm registry tarball, a git tag is not cryptographically immutable, so this
# script also resolves and prints the tag's commit sha for the committer to
# record in README.md — the reviewed, committed copy is the actual pin; this
# script is only how it gets refreshed.
#
# Committed artifacts: model_prices_and_context_window.json + LICENSE, verbatim.
#
# Requires (build-time only, on the refreshing box): curl + node (JSON shape
# validation).
#
# Usage: bash .pyxis/vendor/litellm-prices/refresh.sh           # uses the versions.env pin
#        LITELLM_PRICES_VERSION=1.94.0 bash .pyxis/vendor/litellm-prices/refresh.sh
set -eu

HERE="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$HERE/../../.." && pwd)"   # vendor/<x>/ lives under .pyxis/ (footprint phase 2)

# Pin source: .pyxis/scripts/versions.env (the single tool-version source), overridable by
# an inline LITELLM_PRICES_VERSION for a bump-and-review run.
if [ -z "${LITELLM_PRICES_VERSION:-}" ]; then
  versions_env="$REPO_ROOT/.pyxis/scripts/versions.env"
  [ -f "$versions_env" ] || { echo "refresh: $versions_env not found" >&2; exit 2; }
  # shellcheck disable=SC1090
  . "$versions_env"
fi
VER="${LITELLM_PRICES_VERSION:-}"
[ -n "$VER" ] || { echo "refresh: LITELLM_PRICES_VERSION is empty (set it or .pyxis/scripts/versions.env)" >&2; exit 2; }

command -v curl >/dev/null 2>&1 || { echo "refresh: curl required (build-time only)" >&2; exit 2; }
command -v node >/dev/null 2>&1 || { echo "refresh: node required (build-time only, JSON shape validation)" >&2; exit 2; }

# sha256_hex — portable sha256 (Linux: sha256sum; macOS: shasum -a 256).
sha256_hex() {
  if command -v sha256sum >/dev/null 2>&1; then sha256sum | cut -d' ' -f1
  else shasum -a 256 | cut -d' ' -f1; fi
}

echo "refresh: vendoring litellm@v${VER}'s price map into $HERE (single upstream data file)"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

raw_url="https://raw.githubusercontent.com/BerriAI/litellm/v${VER}/model_prices_and_context_window.json"
license_url="https://raw.githubusercontent.com/BerriAI/litellm/v${VER}/LICENSE"

curl -fsSL "$raw_url" -o "$tmp/model_prices_and_context_window.json" \
  || { echo "refresh: download failed: $raw_url" >&2; exit 1; }
curl -fsSL "$license_url" -o "$tmp/LICENSE" \
  || { echo "refresh: download failed: $license_url" >&2; exit 1; }

# Shape sanity, checked against the CANDIDATE before it replaces the committed
# copy (fail here, not after a broken price lookup ships) — catches a fetch
# that silently landed an HTML error page or an unrelated JSON blob.
cat > "$tmp/validate.mjs" << 'JSEOF'
import { readFileSync } from 'node:fs';
const path = process.argv[2];
const raw = readFileSync(path, 'utf8');
let j;
try { j = JSON.parse(raw); } catch (e) { console.error('refresh: FAIL — not valid JSON: ' + e.message); process.exit(1); }
if (typeof j !== 'object' || j === null || Array.isArray(j)) { console.error('refresh: FAIL — top level is not a JSON object'); process.exit(1); }
const keys = Object.keys(j);
if (keys.length < 1000) { console.error('refresh: FAIL — suspiciously few entries (' + keys.length + '), expected 1000+'); process.exit(1); }
if (!('sample_spec' in j)) { console.error('refresh: FAIL — missing the expected "sample_spec" schema-doc entry'); process.exit(1); }
const providers = new Set(Object.values(j).map(v => v && v.litellm_provider).filter(Boolean));
if (!providers.has('anthropic') || !providers.has('openai')) { console.error('refresh: FAIL — missing expected anthropic/openai provider entries'); process.exit(1); }
console.error('refresh: shape OK — ' + keys.length + ' entries, ' + providers.size + ' providers');
JSEOF
node "$tmp/validate.mjs" "$tmp/model_prices_and_context_window.json"

# Content-hygiene tripwire: no build-machine paths in a vendored artifact (the
# harbormaster #851 lesson — review diff CONTENT, not only runtime behavior).
if grep -nE '(/private/var|/var/folders|/Users/|/home/|/tmp/tmp\.)' "$tmp/model_prices_and_context_window.json" >/dev/null 2>&1; then
  echo "refresh: WARNING — build-machine paths found in the fetched file (review before committing):" >&2
  grep -nE '(/private/var|/var/folders|/Users/|/home/|/tmp/tmp\.)' "$tmp/model_prices_and_context_window.json" | head -5 >&2
fi

# Resolve the tag to a commit sha for provenance (belt-and-braces: unlike an npm
# registry tarball a git tag is not cryptographically immutable, so this is
# recorded but the REVIEWED, COMMITTED copy is the actual pin). Soft-fail: a
# rate-limited or unreachable API call never blocks the vendor, only the
# provenance note.
resolved_sha="$(curl -fsSL "https://api.github.com/repos/BerriAI/litellm/commits/v${VER}" 2>/dev/null \
  | node -e 'let d="";process.stdin.on("data",c=>d+=c);process.stdin.on("end",()=>{try{process.stdout.write(JSON.parse(d).sha||"")}catch(e){}})' \
  2>/dev/null || true)"
[ -n "$resolved_sha" ] || echo "refresh: WARNING could not resolve v${VER} to a commit sha (recording version only)" >&2

# Replace the vendored artifacts (idempotent: full re-vendor each run), verbatim.
cp "$tmp/model_prices_and_context_window.json" "$HERE/model_prices_and_context_window.json"
cp "$tmp/LICENSE"                              "$HERE/LICENSE"

sha="$(sha256_hex < "$HERE/model_prices_and_context_window.json")"
echo "refresh: done — vendored litellm's price map @ v${VER} (commit ${resolved_sha:-unknown})."
echo "refresh: sha256(model_prices_and_context_window.json) = $sha"
echo "refresh: record the version, commit sha, and sha256 above in .pyxis/vendor/litellm-prices/README.md."
echo "refresh: review the diff content, then commit."
