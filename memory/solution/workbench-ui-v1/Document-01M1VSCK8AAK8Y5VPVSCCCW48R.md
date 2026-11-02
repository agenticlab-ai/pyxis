# Workbench UI v1 — solution shape

| Field | Value |
|---|---|
| **Design** | workbench-ui-v1 |
| **Status** | superseded-by-code |

**Artifact layout** — the UI's home in the managed footprint and its single write target, the gitignored render scratch (DINV-1, DINV-4, A-5):

```text
.pyxis/ui/
  server.mjs          # entry: node .pyxis/ui/server.mjs  (the documented launcher)
  registry.mjs        # the class registry — the ONLY home of class knowledge
  reader.mjs          # machine-faces-first corpus reader → DocEntry records
  search.mjs          # fingerprint-keyed in-memory index over DocEntry
  progress.mjs        # stage derivation over DocEntry + index face joins
  render/             # server-rendered page templates (vanilla ESM, no build step)
  fixtures/           # per-class goldens, hostile/degradation/scale fixtures
.lavish/              # (existing, gitignored) statefile + composed render scratch
```

**Class-registry entry** — one entry per document class; adding a class appends one entry and touches nothing else; `face` names the machine face when one exists, else the faceless header/anchor scan applies; `render` names the committed transform route (DINV-2, DINV-4, A-8):

```json
{
  "id": "tech-design",
  "root": "docs/tech-design",
  "face": { "kind": "index-json", "path": "docs/tech-design/index.json" },
  "anchorGrammars": ["DINV-n", "A-n", "R-n", "Q-n"],
  "citationGrammars": ["design:<slug>[#anchor]", "prd:<feature>/<ID>"],
  "render": { "via": "doc-renderer" },
  "progressRole": "design"
}
```

The faceless lane of the same schema — no machine face, the header/anchor scan carries the class (DINV-4, A-8):

```json
{
  "id": "incidents",
  "root": "docs/observability/incidents.md",
  "face": null,
  "anchorGrammars": ["I-n"],
  "citationGrammars": ["AP-n (Graduated)"],
  "render": { "via": "doc-renderer" },
  "progressRole": null
}
```

**DocEntry** — the reader's uniform output record; everything downstream (catalog, search, citations, progress) consumes DocEntry, never files; `fingerprint` is the cheap tree signature that keys the in-memory rebuild (DINV-5, A-1):

```json
{
  "class": "prd-ledger",
  "slug": "workbench-ui",
  "path": "docs/prds/workbench-ui/effective.md",
  "title": "Workbench UI (the reading room) — PRD",
  "status": "merged",
  "headings": [{ "text": "5.1 Functional requirements", "anchor": "#51-functional-requirements" }],
  "anchors": ["FR-1", "FR-3", "NFR-2"],
  "citations": [{ "raw": "brainstorm:workbench-ui#D-1", "resolved": "docs/brainstorms/workbench-ui.md#D-1" }],
  "mtime": 1760000000,
  "fingerprint": "sha256:…"
}
```

**Route table** — every route GET-only under the secret prefix; requests without the secret or with a Host outside the allowlist are refused before routing (DINV-3, A-6):

```text
GET /s/<secret>/                      → catalog (grouped by class)
GET /s/<secret>/d/<class>/<slug>      → rendered document (composed transform)
GET /s/<secret>/search?q=…            → results (ID-exact lane first)
GET /s/<secret>/progress              → per-feature stage rows
GET /s/<secret>/status                → debugging panel (root, counts, degradations, fingerprint)
GET /s/<secret>/assets/<file>         → local assets only; resolved inside render/ only,
                                        traversal (`..`, absolute) → 404 quadruple
Host allowlist: 127.0.0.1:<port> · localhost:<port>       any other Host → 403 quadruple
any non-GET → 405 quadruple           any secret miss → 404 (no probe oracle)
```

**Statefile** — written to the render scratch at launch, removed at exit; a second launch probes `pid` liveness plus a guarded ping before refusing; a dead statefile is replaced (DINV-1, A-5):

```json
{
  "schema": "workbench-ui-state/v1",
  "pid": 12345,
  "port": 51720,
  "startedAt": "2026-07-10T12:00:00Z",
  "secretHash": "sha256:…",
  "repoRoot": "/abs/path/to/repo",
  "idleExitSeconds": 7200
}
```

**Progress row** — the stage chain derived per feature slug from corpus signals only; `gates` lists pending human gates read from status ladders; beyond-corpus stages render as `unknown` (A-4):

```json
{
  "feature": "workbench-ui",
  "stages": {
    "brainstorm": { "state": "gated:ready-for-prd", "link": "docs/brainstorms/workbench-ui.md" },
    "prd":        { "state": "merged", "link": "docs/prds/workbench-ui/effective.md" },
    "design":     { "state": "draft", "link": "docs/tech-design/workbench-ui-v1.md" },
    "plan":       { "state": "absent" },
    "beyond":     { "state": "unknown" }
  },
  "gates": ["design: operator approval pending"],
  "reviews": [{ "subject": "prd:workbench-ui/000", "verdict": "approve-with-advisories" }]
}
```

**Entry invocation and fail-closed argument grammar** — the documented launcher line; any unknown flag or flag-like positional is a usage refusal before any action; bind or instance collision refuses with the quadruple (DINV-3, A-5):

```sh
node .pyxis/ui/server.mjs [--repo <path>] [--no-open]
# unknown flag / flag-like positional → usage + exit 2 (nothing served)
# occupied port / live sibling session → quadruple + exit 1 (no half-served state)
```
