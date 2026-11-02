# vendor/mermaid — pinned mermaid browser bundle

This is a **vendored, pinned copy** of the upstream-built
[`mermaid`](https://www.npmjs.com/package/mermaid) browser bundle
(`dist/mermaid.min.js` from the npm tarball, verbatim). It exists for one
consumer: `.claude/scripts/render-doc.mjs` **inlines it into the rendered
HTML** when — and only when — the doc being rendered contains ` ```mermaid `
fences, so the diagrams draw in the doc-review surface.

Why inline, not fetch or `<script src>`:

- The rendered artifact must stay **standalone + offline** — it opens in any
  browser with no server and no network (the doc-review skill's degradation
  contract depends on this). A CDN tag would break that; a relative `src`
  breaks the moment the HTML is served or moved standalone.
- Render stays **byte-deterministic**: same doc + same pinned asset → same
  bytes. No fetch at render time, ever.

## Vendoring model — one upstream-built file, zero install

Unlike `vendor/lavish-axi` (a Node CLI with runtime deps, hence its
lockfile + install-time `npm ci` model), `mermaid.min.js` is a
**self-contained browser IIFE built by upstream** (esbuild; exposes
`globalThis.mermaid`). It has **zero runtime dependencies and no install
step** — the committed file is the whole runtime, the direct analogue of
lavish's committed `dist/`.

Committed:

- `mermaid.min.js` — the upstream `dist/mermaid.min.js`, verbatim.
- `LICENSE`, `package.json` — provenance + the embedded version
  (cross-checked against the pin by `scripts/tests/test-render-doc.mjs`).

Inline-safety invariants (checked by `refresh.sh` at re-vendor time, and by
the render test against the committed file):

- contains no `</script` sequence (would end the inline `<script>` element
  early; the renderer additionally guards by rewriting any future occurrence
  to `<\/script`, which is semantics-preserving in JS strings);
- contains no `<script` sequence (rules out the HTML script-data
  double-escape hazard around `<!--`);
- no build-machine paths (the harbormaster #851 lesson: review content, not
  just behavior).

Expected-and-guarded (do **not** "fix" at a bump): the bundle legitimately
contains `</body></html>` inside its own JS strings (embedded DOMPurify/html
templates). Left raw, the vendored lavish-axi's `injectLavishSdk()` — which
splices its sdk.js tag at the *first* `/<\/body\s*>/i` match in the served
file — would inject mid-bundle, detonating the script element and dumping
megabytes of JS as visible page text. The renderer's `inlineScriptSafe()`
neutralizes `</script`, `</body`, and `</html` to `<\/…` at inline time, and
the render test pins "exactly one `</body>` in the page".

## Pin

The version is pinned by **`MERMAID_VERSION` in `scripts/versions.env`** —
the single tool-version source. The vendored `package.json` carries the same
version; `scripts/tests/test-render-doc.mjs` fails on a missing or mismatched
vendored copy against that pin.

Current pin: **11.16.0** — sha256 of `mermaid.min.js`:
`74d7c46dabca328c2294733910a8aa1ed0c37451776e8d5295da38a2b758fb9b`

## Updating

One reviewed version bump:

1. Bump `MERMAID_VERSION` in `scripts/versions.env`.
2. `bash vendor/mermaid/refresh.sh` — re-vendors the pinned version from the
   registry tarball and re-checks the inline-safety invariants. Idempotent
   (full re-vendor each run). It prints the new sha256 — record it above.
3. **Review the diff content** (a 3.4 MB minified blob is unreviewable by
   eye — the invariant checks plus the sha256 are the review surface).
4. Re-run `node scripts/tests/test-render-doc.mjs` and re-render a doc with
   diagrams to confirm they still draw.
