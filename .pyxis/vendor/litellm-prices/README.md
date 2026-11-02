# vendor/litellm-prices — pinned LiteLLM per-model price map

This is a **vendored, pinned copy** of
[`model_prices_and_context_window.json`](https://github.com/BerriAI/litellm/blob/main/model_prices_and_context_window.json)
from [`litellm`](https://github.com/BerriAI/litellm) — the de-facto,
actively-maintained, cross-provider (Anthropic/OpenAI/…) per-model pricing +
context-window table. It exists for one consumer: the **pipeline-measurement**
tool's cost derivation (`docs/tech-design/pipeline-measurement-v2.md` §3.12),
which derives a phase's dollar cost from ground-truth token counts times this
table, instead of pyxis self-maintaining a price table that would rot (`#D-7`,
rejected alternative in §4).

Why vendored + pinned, not fetched at runtime:

- **No new egress** — the measurement tool reads only harness accounting and
  git (DINV-1); a runtime price fetch would add network I/O and an
  availability dependency neither the design nor the tool's offline posture
  wants.
- **Reproducibility** — every run record stamps the price-snapshot version it
  used (DINV-6), so a historical cost reproduces exactly. That only holds if
  the snapshot is a pinned, committed file, not a moving upstream target.
- **Supply-chain surface** — parsed with stdlib `encoding/json` as vendored
  *data*, never a code dependency (no new package, no transitive deps).

## Vendoring model — one upstream data file, zero install

Like `vendor/mermaid`, this is a single
upstream-provided file with **zero runtime dependencies and no install
step** — the committed JSON file is the whole artifact; there is no lockfile
and nothing to `npm ci`.

Committed:

- `model_prices_and_context_window.json` — the upstream file, verbatim, fetched
  from a **tagged release** (never `main`) of `github.com/BerriAI/litellm`.
- `LICENSE` — litellm's root license (MIT; the repo's `enterprise/` carve-out
  does not apply to this file, which lives at the repo root), verbatim,
  fetched from the same tag.

## Pin

The version is pinned by **`LITELLM_PRICES_VERSION` in
`.pyxis/scripts/versions.env`** — the single tool-version source, alongside
the `MERMAID_VERSION` pin.

Unlike an npm registry tarball (content-addressed by publish, in practice
immutable), a GitHub git tag is not cryptographically immutable — the
reviewed, **committed** copy is the actual pin; `refresh.sh` is only how it
gets refreshed. The resolved commit sha is recorded below at each bump for
provenance.

Current pin: **1.93.0** (tag `v1.93.0`, commit
`052b5a2169d8d3082e1d66e69f200a72b0c1e274`) — sha256 of
`model_prices_and_context_window.json`:
`dd16488fd139eaab72ea32cf87efdb48109f44dc9308c6b993a456740a113912`

## Updating

One reviewed version bump:

1. Bump `LITELLM_PRICES_VERSION` in `.pyxis/scripts/versions.env`.
2. `bash .pyxis/vendor/litellm-prices/refresh.sh` — re-vendors the pinned tag
   from `raw.githubusercontent.com`, checks the fetched JSON's shape (parses,
   has enough entries, carries the `sample_spec` schema-doc key, covers
   `anthropic` + `openai`), and prints the new sha256 + resolved commit sha.
   Idempotent (full re-vendor each run).
3. **Review the diff content** — a price-table diff is reviewable by eye
   (unlike the minified Mermaid bundle): scan for
   unexpected per-model price swings, not just added/removed models.
4. Record the new version, commit sha, and sha256 in the **Pin** section
   above, then commit.
