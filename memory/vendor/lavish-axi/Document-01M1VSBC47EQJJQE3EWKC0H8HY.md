# vendor/lavish-axi — pinned, reviewed lavish-axi

This is a **vendored, pinned copy** of [`lavish-axi`](https://www.npmjs.com/package/lavish-axi)
— the HTML review surface + inline-annotation channel the `doc-review` skill drives.
Pyxis runs it **only** via

```bash
node vendor/lavish-axi/dist/cli.mjs <args>
```

and **never** `npx -y lavish-axi`. `npx -y` fetches and executes remote code at
runtime — the operation every agentic-CLI sandbox blocks, and a supply-chain
surface. A pinned, reviewed, vendored copy run through the local `node` is plain
local code execution, which the sandbox allows (proven live on the strictest
sandbox, 2026-06-24, in harbormaster).

## Vendoring model — un-bundled snapshot + install-time deps

This model is inherited from harbormaster, where the alternative (esbuild-bundling
the CLI into one self-contained file) broke **three separate times**: an ESM bundle
of CJS deps threw `Dynamic require of "tty" is not supported` at runtime, the build
machine's `mktemp` path leaked into 632 committed module-registry keys, and the
bundle was non-reproducible across re-vendors. The durable lesson: **vendor a node
CLI un-bundled and install its deps at install time; never bundle it.**

Committed (the un-bundled snapshot + its pin):

- `dist/` — the **un-bundled** upstream `cli.mjs` (the entry the driver runs) + its
  client assets (`chrome-client.js`, `chrome.css`, `design/`).
- `LICENSE`, `package.json` — provenance + the embedded version.
- `package-lock.json` — pins the runtime deps so the install is reproducible.

**Not** committed:

- `node_modules/` — the runtime deps (`express`, `chokidar`, `axi-sdk-js`, `open`)
  are installed once by **`scripts/setup-lavish.sh`** (`npm ci --omit=dev`), pinned
  by the committed lockfile. Gitignored; appears only after that install (or
  `refresh.sh`'s verify step).
- The upstream tarball's multi-MB marketing GIF and its bundled skill copy (pyxis
  owns `.claude/skills/doc-review/`) — dropped to keep the vendored copy lean.

## Pin

The version is pinned by **`LAVISH_AXI_VERSION` in `scripts/versions.env`** — the
single tool-version source. The vendored `package.json` carries the same version;
`scripts/tests/test-lavish-api-contract.sh` **fails on a missing or mismatched
vendored copy** against that pin, and pins the `/api` wire contract the driver
depends on, so a version bump that moves the shapes is caught before it ships.

## Updating

Updating is **one reviewed version bump**:

1. Bump `LAVISH_AXI_VERSION` in `scripts/versions.env`.
2. `bash vendor/lavish-axi/refresh.sh` — re-vendors the pinned version from the
   registry, dropping the non-runtime files, and verifies the snapshot installs and
   runs in an isolated dir. Re-running is idempotent (full re-vendor each run).
3. **Review the `dist/` + `package-lock.json` diff content** — leaked absolute/home
   paths and non-determinism, not only runtime behavior ("it runs" ≠ "it's
   committable"). Then commit.
4. Re-run `scripts/tests/test-lavish-api-contract.sh` — it fails if the bump moved
   the `/api` shapes out from under the driver.
