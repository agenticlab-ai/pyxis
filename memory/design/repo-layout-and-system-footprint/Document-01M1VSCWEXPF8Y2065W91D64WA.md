# Repo layout and system footprint — the three buckets, `.pyxis/`, and the docgate release channel

| Field | Value |
|---|---|
| **Status** | Approved (2026-07-07 — operator, in doc-review) *(merge is the freezing event; "superseded" is computed by the index, never written here)* |
| **Owner** | @mdarabi |
| **Approver(s)** | @mdarabi (operator) |
| **Date** | 2026-07-07 |
| **Relevant-when** | adding any file the pyxis system (not this product) owns; wiring pyxis into a new operand repo; changing how docgate is acquired, versioned, or configured; touching install/uninstall or the config surface |
| **Scope** | Ownership taxonomy for every repo pyxis operates on (including this one): what belongs to the operand vs the system vs unavoidable root integration points; the `.pyxis/` footprint; the docgate release channel (source mode vs pinned release); the config surface (issue #4). **Not**: the engine's runtime architecture, adapter design, or docgate's verb semantics (docgate-v1). |
| **Tracking** | introducing PR (filed on approval) · issue #4 (pyxis-config surface) |
| **Builds-on** | docs/tech-design/tech-design-conventions.md · docs/tech-design/docgate-v1.md |
| **Supersedes** | — |
| **Serves** | doc:AGENTS.md (rule 7's "bootstrap-era binding … per-repo configurable via the pyxis-config surface") · design:docgate-v1#3.10 (wiring, generalized beyond self-host) |
| **Security/privacy impact** | binary acquisition is new surface: releases are sha256-pinned in config and verified at fetch — no unpinned execution (see §3.5) |
| **Template version** | tech-design-template/v1 |

## 1. Problem

Pyxis is self-hosting: this repo is both the product's source and the
factory's first operand. The two roles' files are interleaved — templates,
skills, scripts, hooks, vendored tools, and gate wiring sit beside the
product's own documents and code — so "what is pyxis's footprint?" has no
one-line answer, installing into a second repo ("my-product") has no
defined shape, and docgate exists only as in-repo source (`go run` works
solely here). Rule 7 already names today's fixed paths a bootstrap-era
binding awaiting the config surface.

## 2. Decision

**Decision:** Every pyxis-operated repo is partitioned into three buckets
with a removal test as the boundary oracle. Bucket 1 — **operand content**
(the repo's own documents, ledgers, rollups, and code): survives pyxis's
removal untouched. Bucket 2 — **system footprint**: everything pyxis
installs, consolidated under one directory, **`.pyxis/`** (config,
templates, authoring guides, skills, scripts, hooks, vendored tools, a
gitignored binary cache). Bucket 3 — **root integration points** vendors
dictate (`AGENTS.md`, `.claude/`, `.codex/`, `.github/workflows/`): kept,
but as pointers/thin callers into `.pyxis/` only — the repo root is
treated as just another vendor surface (AP-25 generalized). Docgate
reaches operands as a **versioned, sha256-pinned static release binary**
resolved by shim and CI identically; `source` mode (`go run`) is a config
mode reserved for this repo, where the source lives.

## 3. The design

### 3.1 The three buckets

| Bucket | Owner | Contents (today's paths) | On `pyxis remove` |
|---|---|---|---|
| 1 — operand content | the repo | `docs/prds/` ledgers+projections · `docs/tech-design/` · `docs/wiki/` · `docs/observability/` · vision/principles/domain/system docs · `docs/prd.md`+`docs/techspec.md` · product code (`cmd/`, `internal/`, `go.mod` — here only because the product *is* pyxis) | stays, complete and self-describing |
| 2 — system footprint | pyxis | `docs/templates/` · `docs/authoring/` · `agents/{skills,scripts}` · `.githooks/` · `vendor/{lavish-axi,mermaid}` · `scripts/setup-lavish.sh` · `scripts/tests/` (tooling's tests) · docs-gate workflow body · docgate binary | removed — capability leaves, content stays |
| 3 — root integration | vendors | `AGENTS.md` · `.claude/` · `.codex/` · `.github/workflows/docs-gate.yml` | pointers deleted; nothing dangles |

Content never links into `.pyxis/` by path: documents cite templates by
**version pin** (`prd-template/v1`), never location — the corpus stays
interpretable after removal.

### 3.2 Boundaries and non-goals

- **Not solved here** — engine architecture, adapter ports, `pyxis init`'s
  full UX (a future design owns the installer); docgate verb semantics.
- **Implementation must not infer** — that bucket-2 files may reference
  operand-specific facts (they must be operand-portable), or that any
  content file may hard-code a `.pyxis/` path.
- **Must remain unchanged** — ledger/projection semantics (docgate-v1);
  the placeholder corpus; AP-25's pointer-only rule for vendor dirs.

### 3.3 Invariants

- **DINV-1 (the removal test)** — deleting `.pyxis/` and the bucket-3
  pointers leaves bucket 1 complete, readable, and fingerprint-verifiable
  (re-fetching the pinned binary reproduces every fingerprint).
- **DINV-2** — no unpinned execution: shim and CI run only a binary whose
  sha256 matches the config pin, or `source` mode in the repo that carries
  the source.
- **DINV-3** — bucket-2 assets are operand-agnostic: installing the same
  system version into two repos yields byte-identical `.pyxis/` trees
  (config excepted).
- **DINV-4** — root files carry pointers and invocation glue only; all
  knowledge and capability live under `.pyxis/` (or `docs/` for content).

### 3.4 Observability

- **Install/upgrade** — the installer writes a manifest (`.pyxis/
  manifest.json`: system version, per-file hashes) so drift between
  installed assets and their release is detectable (`pyxis doctor`-style
  check; docgate v2 candidate).
- **Version handshake** — `docgate --version` is compared to the config
  pin in CI; mismatch is a named defect, not a silent difference.
- **Metrics/alerts** — none beyond existing gate verdicts (red CI is the
  alert), inherited from docgate-v1 §3.4.

### 3.5 Security & privacy

Binary acquisition is the one new surface (header row): releases are
fetched over TLS from the pinned release channel, verified against the
config's sha256 **before first execution**, cached in gitignored
`.pyxis/bin/`; a hash mismatch aborts fail-closed. No telemetry, no
secrets, no data handling beyond docgate-v1's (none).

### 3.6 `.pyxis/` layout and the config surface (issue #4)

```
.pyxis/
├─ config.yaml      # closed-parse: the full schema below
├─ templates/  authoring/  skills/  scripts/  hooks/  vendor/
├─ bin/             # gitignored fetch cache
└─ manifest.json    # installed-asset hashes (§3.4)
```

`config.yaml`, spelled out (closed-parse: unknown keys are errors, AP-6):

```yaml
system:
  version: "0.3.0"          # installed pyxis release; must match manifest.json
docgate:
  version: "v1.2.0"         # release tag, or "source" (legal only where the source lives)
  sha256:                   # per-platform checksums, verified before first run (DINV-2);
    darwin-arm64: "<hex>"   # omitted iff version == "source" — mutually exclusive,
    linux-amd64: "<hex>"    # one config fixture pins each direction
paths:                      # defaults = today's locations (rule 7's binding becomes data)
  prds: docs/prds
  tech_designs: docs/tech-design
  rollup_prd: docs/prd.md
  rollup_techspec: docs/techspec.md
  incidents: docs/observability/incidents.md
templates:                  # versions this install ships; the binary must support every pin
  prd: prd-template/v1
  amendment: amendment-template/v1
  tech_design: tech-design-template/v1
capability_paths:           # FR-10 rollup-guard trigger set, operand-tunable
  - cmd/                    # (my-product would list src/ instead; the test-only
  - internal/               #  exemption stays compiled into the binary)
  - .githooks/
  - .github/workflows/
```

Docgate reads paths from config instead of hard-coding `docs/prds`.
The binary validates each document's declared template version against
the set it supports; the installer keeps `.pyxis/templates/` matched to
the binary's supported set.

### 3.7 The docgate release channel

Docgate ships as goreleaser-style artifacts per version: static binaries
(darwin/linux, arm64/amd64) + checksums. Resolution order (shim and CI
identical, DINV-2): `.pyxis/bin/docgate` if hash matches the pin → fetch
the pinned release, verify, cache → in `source` mode only, `go run
./cmd/docgate`. This repo runs `source`; every other operand pins a
release. Upgrades are deliberate: bump pin → CI shows any verdict drift.

### 3.8 Migration (phased, each phase green)

1. **Config-with-current-defaults** — introduce `.pyxis/config.yaml` +
   manifest; docgate reads paths from it (defaults = today); no file moves.
2. **Consolidate bucket 2** — move templates/authoring/skills/scripts/
   hooks/vendor under `.pyxis/`; root and vendor dirs become
   pointers/symlinks; citation sweep; suites green through the links.
3. **Release channel** — first tagged docgate release + fetch-verify in
   shim/workflow; this repo flips to `source` mode explicitly.
4. **Installer** — `pyxis init` assembles `.pyxis/` + pointers in a fresh
   operand (future design; unblocks "my-product").

## 4. Alternatives considered

| Alternative | Why rejected |
|---|---|
| copy docgate source into operands | bucket confusion; N repos × source drift; operands forced onto Go toolchains |
| go install from module path | requires Go everywhere; version drift by toolchain; no sha pinning |
| binaries committed into git | repo bloat; undiffable; the cache-with-pin gives the same determinism without it |
| keep today's scattered layout, document it | "footprint" stays unanswerable; uninstall undefined; my-product install unspecifiable |
| one `pyxis/` visible dir (not dotted) | collides with product naming in operands; dot-dir matches `.github`/`.claude` precedent for tool surfaces |

## 5. Risks

| ID | Risk | Impact | Mitigation | Verification / evidence |
|---|---|---|---|---|
| R1 | migration breaks path citations | broken links, red gates | phased moves with citation sweeps; suites + gate green per phase (§3.8) | CI on each phase PR |
| R2 | release channel adds supply-chain surface | tampered binary | sha256 pin verified pre-exec, fail-closed (DINV-2, §3.5) | fetch-verify test |
| R3 | config drift vs binary expectations | confusing verdicts | version handshake defect (§3.4); closed-parse config (unknown keys error) | fixture |
| R4 | self-host `source` mode diverges from released behavior | verdicts differ across repos | releases cut from the same tested source; goldens run in both modes in this repo's CI | CI matrix at phase 3 |

## 6. Acceptance

| ID | Acceptance criterion | Verify | Required evidence | Traces to |
|---|---|---|---|---|
| A-1 | removal test passes: delete `.pyxis/` + pointers in a scratch clone → bucket 1 intact, docs readable, fingerprints reproducible with a fetched binary | Test | scripted check | DINV-1 |
| A-2 | no content file references a `.pyxis/` path (version pins only) | Test (grep gate) | CI check | §3.1, DINV-4 |
| A-3 | shim and CI resolve the identical binary (hash-verified) or both run source mode | Inspection | shim + workflow diff | DINV-2, §3.7 |
| A-4 | config parses closed; unknown keys are defects; paths default to today's | Test | config fixtures | §3.6 |
| A-5 | fresh-operand install (phase 4) yields a working gate with zero Go toolchain present | Demo | my-product walkthrough | §3.7 |

## 7. Open questions

| ID | Question | Owner | Blocks | Status | Resolution |
|---|---|---|---|---|---|
| Q1 | Release hosting: GitHub Releases on this repo vs a dedicated dist repo | operator | phase 3 | open | — |
| Q2 | Does `AGENTS.md` in operands stay hand-authored or become installer-generated from a bucket-2 template + operand config | operator | phase 4 | open | — |

## Principles

- **AP-1 / AP-25 — honored.** The operand's mediums and the repo's agent surface stay swappable; the root becomes pointers-only, generalizing AP-25 to the filesystem root.
- **AP-5 / AP-6 — honored.** Config and manifest are closed-parse, versioned, additive.
- **AP-10 — honored.** The removal test and pin verification are checks (A-1, A-2, DINV-2), not discipline.
- **AP-17 / INV-6 — honored.** Unpinned or hash-mismatched binaries fail closed.
- **RW-4 / RW-5 — honored.** Substrate (footprint, channel, config) is designed before the installer is built; this document precedes all moves.
- **Deviation:** none.
