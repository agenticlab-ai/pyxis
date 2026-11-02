# scripts/versions.env — the single source for pinned tool versions in pyxis.
#
# Format contract: plain KEY=value lines, no expansion, no quoting — consumers read
# this both by sourcing and by line-parsing (e.g. lavish-driver.sh reads it
# line-by-line), so keep every entry a literal.
#
# One item per line, one comment block per tool — concurrent additions by different
# workers must merge cleanly (append-list discipline).

# lavish-axi — the HTML review surface the doc-review skill drives. VENDORED + pinned and
# run via `node vendor/lavish-axi/dist/cli.mjs`, NEVER `npx` (no remote fetch+exec in
# the runtime path). Bump here, then re-vendor with `bash vendor/lavish-axi/refresh.sh`,
# review the dist/ + lockfile diff, and re-run scripts/tests/test-lavish-api-contract.sh
# (it fails on a pin↔package.json mismatch or an /api contract drift).
LAVISH_AXI_VERSION=0.1.31

# mermaid — the browser diagram renderer render-doc.mjs inlines into a rendered doc
# when (and only when) the doc contains ```mermaid fences. VENDORED + pinned as the
# single upstream-built file vendor/mermaid/mermaid.min.js (self-contained IIFE, zero
# runtime deps — nothing to npm-install), never fetched at render time: the rendered
# HTML must stay standalone + offline. Bump here, then re-vendor with
# `bash vendor/mermaid/refresh.sh`, review the diff, and re-run
# scripts/tests/test-render-doc.mjs (it fails on a pin↔package.json mismatch).
MERMAID_VERSION=11.16.0

# litellm-prices — the cross-provider per-model price + context-window map the
# pipeline-measurement tool's cost derivation reads (design:pipeline-measurement-v2#3.12),
# so pyxis never self-maintains a price table that would rot. VENDORED + pinned as the
# single upstream data file vendor/litellm-prices/model_prices_and_context_window.json
# (parsed with stdlib encoding/json — no code dependency), fetched from a tagged
# litellm release, never fetched at measurement time. Bump here, then re-vendor with
# `bash vendor/litellm-prices/refresh.sh`, review the diff (a price-table diff is
# reviewable by eye), and record the new version/commit sha/sha256 in
# vendor/litellm-prices/README.md.
LITELLM_PRICES_VERSION=1.93.0
