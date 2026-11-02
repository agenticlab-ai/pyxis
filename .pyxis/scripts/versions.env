# scripts/versions.env — the single source for pinned tool versions in pyxis.
#
# Format contract: plain KEY=value lines, no expansion, no quoting — consumers read
# this both by sourcing and by line-parsing, so keep every entry a literal.
#
# One item per line, one comment block per tool — concurrent additions by different
# workers must merge cleanly (append-list discipline).

# Mermaid is the standalone browser bundle inlined by the Go plan viewer.
# Refresh with .pyxis/vendor/mermaid/refresh.sh and run the plan-render tests.
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
