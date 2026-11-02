# Workbench UI v1 — solution shape

| Field | Value |
|---|---|
| **Design** | workbench-ui-v1 |
| **Status** | authoritative |

**Artifact layout** — the Go binary, its internal packages, and its single write target, the gitignored scratch directory (DINV-1, DINV-4, A-5):

```text
cmd/workbench-ui/main.go       # thin released entry point
internal/workbenchui/
  registry.go                  # the ONLY home of class knowledge
  reader.go                    # machine-faces-first corpus reader → DocEntry records
  search.go                    # fingerprint-keyed in-memory index over DocEntry
  progress.go                  # stage derivation over DocEntry + index face joins
  server.go                    # loopback lifecycle, guards, GET-only routes
  templates/                   # embedded html/template files and inline assets
  testdata/                    # per-class goldens, hostile/degradation/scale fixtures
.pyxis/scratch/                # gitignored statefile + regenerable scratch
```

**Class-registry entry** — one entry per document class; adding a class appends one entry and touches nothing else; `Face` names the machine face when one exists, else the faceless header/anchor scan applies; `Presentation` names the registered read-only presentation (DINV-2, DINV-4, A-8):

```go
Class{
    ID:               "tech-design",
    Root:             "docs/tech-design",
    Face:             Face{Kind: "index-json", Path: "docs/tech-design/index.json"},
    AnchorGrammars:   []string{"DINV-n", "A-n", "R-n", "Q-n"},
    CitationGrammars: []string{"design:<slug>[#anchor]", "prd:<feature>/<ID>"},
    Presentation:     "safe-markdown",
    ProgressRole:     "design",
}
```

The faceless lane of the same schema — no machine face, the header/anchor scan carries the class (DINV-4, A-8):

```go
Class{
    ID:               "incidents",
    Root:             "docs/observability/incidents.md",
    AnchorGrammars:   []string{"I-n"},
    CitationGrammars: []string{"AP-n (Graduated)"},
    Presentation:     "safe-markdown",
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
GET /s/<secret>/d/<class>/<slug>      → document through its registered presentation
GET /s/<secret>/search?q=…            → results (ID-exact lane first)
GET /s/<secret>/progress              → per-feature stage rows
GET /s/<secret>/status                → debugging panel (root, counts, degradations, fingerprint)
Host allowlist: 127.0.0.1:<port> · localhost:<port>       any other Host → 403 quadruple
any non-GET → 405 quadruple           any secret miss → 404 (no probe oracle)
```

**Statefile** — written to scratch at launch, removed at exit; a second launch probes `pid` liveness plus a guarded ping before refusing; a dead statefile is replaced (DINV-1, A-5):

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
pyxis exec workbench-ui -- [--repo <path>] [--no-open]
# unknown flag / flag-like positional → usage + exit 2 (nothing served)
# occupied port / live sibling session → quadruple + exit 1 (no half-served state)
```
