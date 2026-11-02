# AGENTS.md — operating manual for agents working in this repository

*Instantiated once by `pyxis init` from `operand-agents-manual/v1`, then
**repo-owned**: the humans here extend it freely, `pyxis converge` never
rewrites it, and `pyxis doctor` advises when the shipped template has moved
on. Vendor entry files (`CLAUDE.md`, per-CLI configs) are pointers to this
file — knowledge lives here, never in a vendor surface.*

## This repository is factory-operated

This repository is managed by **pyxis**, pinned to one verified system
release. The knobs captured at install time:

- **Tracker:** `{{tracker}}` — where work items live and where the factory
  files escalations and welcome items.
- **CI mode:** `{{ci_mode}}` — where the docs gate runs: `local` (hooks
  only), `actions` (hosted CI), or `both`.
- **Default branch:** `{{branch}}` — the mainline every gate and merge-base
  computation keys on.
- **Delegated authority (argus):** `{{argus_enabled}}` — while `false`, no
  delegated decision authority exists in this repository: the argus skill
  refuses with an enablement pointer and no authority contract file exists;
  flipping the knob is a human act through the pyxis config.

Every engine tool runs through the one front door — `pyxis exec <tool> --
<argv…>` — which verifies the binary against the pinned release manifest
before every execution. **Never invoke an engine binary directly, fetch one
ad hoc, or build one from this repository**: an installed repository carries
no engine source, and nothing unpinned executes here.

## Durable knowledge — record it once where it belongs

Do not create mandatory per-session logs or "nothing happened" records.
Record decisions, constraints, and lessons that change future work once, in
the artifact they belong to: the relevant requirement, design, issue, pull
request, incident, or typed memory record. Keep the evidence needed to
understand the decision. Use the existing amendment and supersession rules
when the owning artifact is frozen.

A session with nothing durable to add creates no documentation. Operational
logs are optional, local, and ignored by Git. Never automatically import
session logs or transcripts into durable memory. Existing substantive logs
are historical evidence; they do not impose a logging obligation or govern
future work. Preserve an unresolved decision in its owning work item rather
than creating a separate session diary.

## The document system — rules that gate your edits

1. **Link direction is concrete → abstract, never the reverse.** PRDs and
   technical designs cite the vision and principles that govern them;
   `docs/product-vision.md` and `docs/architecture-principles.md` link to
   no concrete document and cite no requirement IDs.
2. **Mutability classes.** The vision, the principles, and incident entries
   are **append-only** (the gate enforces it; governing-document changes
   also need the `Governing-Change: operator` commit trailer — a human
   authorization an agent adds only on explicit instruction). Per-feature
   PRD ledger entries and technical designs are **immutable once merged** —
   merge is the freezing event. Frozen designs evolve by supersession (a new
   document citing the old); frozen PRDs evolve only by typed amendments.
3. **PRDs are ledgers.** A feature's PRD lives at `docs/prds/<feature>/` —
   `000-base.md` plus dense-numbered amendment entries, folded into the
   generated projection. **Never hand-edit `effective.md` or
   `effective.source.json`**; regenerate with
   `pyxis exec docgate -- fold docs/prds/<feature>`.
4. **Generated artifacts are never hand-edited.** The design index under
   `docs/tech-design/` regenerates via `pyxis exec docgate -- gen-index`;
   if a projection is wrong, fix the source and regenerate.
5. **Registries grow through coded writers.** Incidents append via
   `pyxis exec docgate -- add-incident` — never hand-written; entries are
   permanent once merged.
6. **Governing documents are the humans' to author.** The seeded skeletons
   under `docs/` await their owner; no automated author writes governing
   content — `pyxis doctor` advises while they stay unauthored.
7. **No living rollups before code ships.** `docs/prd.md` and
   `docs/techspec.md` do not exist until the first capability ships; do not
   create them early.

## Gates — what stands between an edit and a merge

- **Desk:** the git hooks are active from install (no manual step). The
  pre-commit hook runs the tree-local checks; the pre-push hook runs the
  full gate against `origin/{{branch}}` — both through the exec grammar.
- **CI:** per the `{{ci_mode}}` knob, the hosted `docs-gate` workflow runs
  the identical umbrella verb; desk and CI can never drift because both run
  one implementation.
- Run it yourself before proposing:
  `pyxis exec docgate -- gate --merge-base origin/{{branch}}`.
- A red gate names its defects and remedies; fix the source, never the
  gate's output. `pyxis doctor` reports drift, environment health, and
  advisories read-only; `pyxis converge` repairs engine-owned drift.

## Language policy

This manual sets no language policy for the repository's own code — the
repository's owners append theirs below. The factory's tooling arrives as
pinned release binaries through `pyxis`; agents never build or vendor
factory tools from within this repository.

## Default working rules

- **Work on the current branch.** Never create or switch branches unless
  asked; a proposal rides a `<type>/<slug>` feature branch onto
  `{{branch}}`.
- **Stage by explicit path — never add-all.** Parallel sessions share
  clones; add-all sweeps another session's work into your commit.
- **One concern per proposal**, conventional-commit title, and a body that
  names what closes.
- **Done is an artifact, not a claim.** Report success only on a verifiable
  artifact — the proposal exists, the gate is green — never on prose.
- **Ambiguity → escalate.** If a rule here and an instruction conflict, or
  intent is unclear, ask the operator with options; never guess a silent
  resolution.

## Extending this manual

This file is repo-owned: append the repository's own conventions below —
language policy, build commands, review rules. Keep the factory sections
above intact; they mirror gates that are mechanically enforced, so editing
their prose changes nothing but the truth of the description.
