# Document classes — the map of the pyxis document system

| Field | Value |
|---|---|
| **In one line** | Every document class in this repo — template, evolution model, current-state view, gate, and mutability — one table plus a paragraph each. |
| **Relevant-when** | orienting in the document system; deciding where a new document belongs; checking a class's mutability before editing; onboarding an agent or human to the doc conventions |
| **Authority** | descriptive — the sources below govern; on conflict they win |
| **Sources** | `AGENTS.md` (layout table + rules 1–8) · `docs/architecture-principles.md` (AP-7) · `docs/tech-design/prd-amendment-model.md` · `docs/tech-design/tech-design-conventions.md` · the system templates (version pins; surface in `AGENTS.md`'s layout table) |
| **Last reviewed** | 2026-07-10 |

## The map

| Class | Template | Evolution | Current-state view | Gate |
|---|---|---|---|---|
| Governing (vision · principles · incidents) | — (human-authored) | append-only, operator-fed | the doc itself | `check-governing` (operator trailer; principles line-frozen) + `lint-incidents` — CI + hooks; rule 2 is the prompt half |
| PRD (per-feature) | `prd-template` v1 + `prd-amendment-template` v1 | ledger/projection: immutable base + typed amendments, deterministically folded | `effective.md` + `effective.source.json` (generated) | `docgate` v1 (live: `docs-gate` CI + pre-push) |
| Technical design | `tech-design-template` v1 | successor-declared, whole-document supersession (`Supersedes` retires; `Builds-on` never changes liveness) | the generated index (`index.md` + `index.json`, live); `system-design.md` stays the narrative map | `lint-tech-designs` + `verify-index` + `check-immutable` (FR-17) — CI + hooks |
| UX design | none *(name reserved by the `docs/tech-design/` rename)* | undecided | undecided | undecided |
| System maps (`domain-model.md` · `system-design.md`) | — | living references, updated as decisions land | they are the view | none |
| Living rollups (`docs/prd.md` · `docs/techspec.md`) | — | updated as shipped capability changes (convention; FR-10's per-PR gate retired by docgate 017) | — | forbidden to exist before code (rule 3) |
| Templates (the system templates directory — `AGENTS.md` layout) | they are the templates | instruments — revised directly via PR, version-pinned per instance | — | pin↔instance checks arrive with `docgate` |
| Wiki (`docs/wiki/`, this class) | header convention (see `README.md`) | freely editable via PR | the entries themselves + the index | none — non-authoritative by rule |

## Governing layer

`product-vision.md`, `architecture-principles.md` (AP-1..24, RW-1..6), and the
incident registry. Human-authored inputs, append-only from the beginning;
never derived, regenerated, or "reconciled" from the governed system (AP-24).
They link to **no** concrete document — links point concrete → abstract only
(rule 1), which is why citations into this layer never go stale.

## PRDs — the requirements contract

The most machined class. A feature's PRD is instantiated from
`prd-template/v1` (tiers T1–T3, hard problem → solution →
launch gates, EARS requirements with fit criteria, DoD, contract-readiness
checklist), merges as an immutable ledger base (`000-base.md`), and then
evolves only by **typed amendments** (`prd-amendment-template.md`) folded by a
deterministic tool into the generated projection — the only thing consumers
read. LLMs author ledger entries; only code writes the projection; CI
byte-verifies. Defined in `docs/tech-design/prd-amendment-model.md`.

Note: today's `docs/prds/*.md` are flat **placeholder** inventories (the
harbormaster parity baseline + VNEXT blocks) — requirement *sources*, not
instances of the template, and exempt from its conventions by operator ruling.
Real per-feature PRDs arrive via pipeline stage 1 and live as ledger
directories.

## Technical designs

One decision per document (RW-5) — one *independently-revisable question* —
at `docs/tech-design/<slug>.md`, instantiated from
`tech-design-template/v1` per
`docs/tech-design/tech-design-conventions.md`. Immutable once merged; a
changed decision is a **new** document declaring `Supersedes` (the old doc is
never edited; liveness is computed), and contradiction *requires*
supersession — two live designs never disagree. The header carries typed
`Serves` anchors (the requirement→design join for stage-6 compilation),
`Approver(s)`, and the `Security/privacy impact` trigger; Observability and
Security & privacy subsections are mandatory by name (with `none — <reason>`
escapes); binding elements (`**Decision:**` callouts, `DINV-n` invariants,
`A-n` acceptance rows) bind implementation — everything else explains. The
generated index (the class's projection: a live decision graph with
`stale-serves` / `stale-builds-on` flags) and the CI gate are live
(`docgate` v2, shipped 2026-07-07; design immutability FR-17 shipped in
#52).

## UX designs

Reserved (`docs/ux-design/` — the reason `docs/design/` was renamed). Its
natural anchor is the PRD's §4.3 flows (F-n); its conventions (template,
fidelity, design-file linking, eval-heavy acceptance) are deliberately
deferred to the first UI-bearing feature.

## System maps

`docs/domain-model.md` (entity catalog, INV-1.. invariant registry, ports map,
open modeling questions) and `docs/system-design.md` (component names, planes,
data flows, open-decision queue D1..). Living *references*: individual design
docs make the decisions; the maps are updated to reflect them. They are
navigational truth, not decision records — the rationale trail lives in the
designs they point at.

## Living rollups

`docs/prd.md` and `docs/techspec.md` — the live "what the system does/is"
layer, **seeded 2026-07-07** when the first capability (docgate) shipped,
and updated as shipped capability changes (rule 3 — convention since
docgate amendment 017 retired the FR-10 rollup guard; freshness reads
from git history). A capability
claimed with no code behind it is exactly the drift the sentinel (VNEXT-4)
exists to catch.

## Templates

The system templates directory (`AGENTS.md` layout) — instantiable skeletons ("copy, fill, delete the guidance
italics"; the guidance *is* the authoring prompt). Templates are
**instruments**, not rule-2 decision records: they are revised directly via
normal PRs, and every instantiated document records the template version it
was born under, so template evolution never invalidates existing instances.
The templates are also the authoritative source of the ID-namespace and
payload schemas the amendment fold parses (`docgate`'s registries derive from
them).

## Wiki — this class

Distilled, descriptive knowledge: orientation, conventions explainers, maps
like this one. Flat entries with routing headers; freely editable via PR;
pointer-heavy rather than copy-heavy; **never authoritative** — on any
conflict with a governed document, the wiki is wrong by definition. Rules and
index: `docs/wiki/README.md`.

## Known gaps (honest, dated)

As of 2026-07-08: `docgate` v1 **and** v2 are live — the fold, the gate
verb set (enumerated by `docgate` usage and `docs/techspec.md`), the
generated tech-design index, and the `docs-gate` CI context, with desk
shims activated per clone (`git config core.hooksPath .githooks`).
Design immutability (FR-17) shipped in #52. The #54 review wave
(amendments 006–011: vision append-only, corpus freeze, link/registry
integrity, envelope v2, incident writer, index coverage) is in flight as
a stacked PR train — treat those capabilities as pending until merged.
ux-design is a reserved name only; this wiki's index is hand-maintained
until a generated one exists. The document-authoring skills (`/prd-new`,
`/prd-amend`, `/tech-design-new`) live on the vendor-neutral agent surface
(AP-25; `AGENTS.md`'s layout table names it) — docgate validates
their output. These cells are the most drift-prone statements in the repo
— if you are reading this long after `Last reviewed`, verify against the
sources.
