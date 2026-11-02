# vendor/mermaid — pinned Mermaid browser bundle

This vendored copy of upstream's Mermaid browser bundle is consumed by the
Go plan viewer in `internal/plangate/render`. The viewer inlines it into
standalone HTML so plan diagrams work offline without a server or install step.
It has no Node.js runtime dependency.

The committed files are `mermaid.min.js` (upstream's self-contained browser
IIFE), `LICENSE`, and `package.json` (version provenance). The renderer
escapes closing script, body, and HTML tags before inlining the bundle.

## Pin

`MERMAID_VERSION` in `.pyxis/scripts/versions.env` is the version pin.
The plan-render tests check it against the vendored package and bundle.

Current pin: **11.16.0** — SHA-256 of `mermaid.min.js`:
`74d7c46dabca328c2294733910a8aa1ed0c37451776e8d5295da38a2b758fb9b`

## Updating

1. Bump `MERMAID_VERSION` in `.pyxis/scripts/versions.env`.
2. Run `bash .pyxis/vendor/mermaid/refresh.sh` to fetch the pinned upstream
   artifact and check inline safety. Its build-time tools are curl, tar, and Node.
3. Review the diff and record the new SHA-256 above.
4. Run `go test ./internal/plangate/render` and open a rendered plan to
   confirm the diagram draws.
