# Incident machinery — the coded writer, graduation, and the entry template

| Field | Value |
|---|---|
| **Status** | Approved (2026-07-08 — operator, via the introducing PR review) *(merge is the freezing event; "superseded" is computed by the index, never written here)* |
| **Owner** | @mdarabi |
| **Approver(s)** | @mdarabi (operator — governance-adjacent: the registry is append-only memory) |
| **Date** | 2026-07-08 |
| **Relevant-when** | implementing or changing `add-incident`/`graduate`; capturing an incident through the skill; changing the entry schema; asking why the registry writer may mutate a governed file |
| **Scope** | The Go shape of the incident writer verbs, the shared entry schema, the entry template, and the `/incident` skill surface — implementing `prd:incident-machinery`. **Not**: when to file an incident (a judgment point, NG-3 there), automated discovery/attribution (#53, engine-era), the lint's check semantics (FR-25, docgate-v2 lineage). |
| **Tracking** | issue #54 · PR #56 (requirements) |
| **Builds-on** | docs/tech-design/docgate-v1.md · docs/tech-design/docgate-v2.md |
| **Supersedes** | — |
| **Serves** | prd:incident-machinery/FR-1 · prd:incident-machinery/FR-2 · prd:incident-machinery/FR-3 · prd:incident-machinery/FR-4 · prd:incident-machinery/UB-1 · prd:incident-machinery/NR-1 |
| **Security/privacy impact** | none — hermetic repo-local writes to one tracked file, gated by the existing lint; no identity, secrets, or egress surface |
| **Template version** | tech-design-template/v1 |

## 1. Problem

INV-35 says the incident registry has one coded writer and code-allocated
IDs; reality is a hand-authored I1, hand-performed graduation, and a
prompt rule hoping nobody typos an ID (#54 C2). The registry is one entry
old — the writer must arrive before habits form, and before the lint
hardening (FR-25) raises the cost of hand-writing a conformant entry.
The PRD (`docs/prds/incident-machinery/`) fixes the contract; this design
decides where the writer lives, how it shares schema with the lint, and
how it coexists with docgate-v1's "gate verbs never mutate" invariant.

## 2. Decision

**Decision:** The writer is a **new verb family in the existing binary** —
`add-incident` and `graduate` — living in `internal/docgate/incidents`,
distinct from the gate verbs: gate verbs report, writer verbs write
**exactly one file** (the configured registry) and nothing else. One
entry schema (`internal/docgate/schema`, extending the existing registry
pattern) is consumed by three parties: the writer renders from it, the
FR-25 lint checks against it, and the version-pinned
`incident-entry-template/v1` documents it for humans — one definition,
no drift by construction. `add-incident` computes the next dense `I<n>`
from the parsed registry, renders the entry, and **self-checks**: it runs
the lint over the would-be result and refuses the write (byte-unchanged
registry) on any defect. `graduate` performs the one sanctioned mutation
— appending `Graduated: AP-<n>` to an entry's `Links:` line — under the
same self-check. The `/incident` skill is a thin authoring surface on the
vendor-neutral skills tree that gathers the four fields and tags, then
invokes the writer; it never writes the file itself.

## 3. The design

### 3.1 Mechanics

- **ID allocation** — parse the registry with the same entry regex the
  lint uses; next ID = max + 1 (dense by lint guarantee). Allocation and
  write are one process; cross-branch races surface as duplicate-ID lint
  reds at the second PR's gate (PRD UB-2) — the amendment-numbering
  pattern, reused.
- **Rendering** — the entry is rendered from the schema definition:
  `**I<n> (YYYY-MM-DD) [tag, tag]**` head plus the four `- **Field:**`
  lines, in order, ~6-line body preserved as given. The date is an
  argument (hermeticity: no clock read — docgate-v1's determinism
  posture; the skill passes today's date).
- **Self-check-then-write** — render → splice into an in-memory copy →
  run the FR-25 shape checks over it → on any defect, print it and exit 1
  with the file untouched; else write atomically (temp + rename).
- **Graduation** — locate the entry block, verify no existing
  `Graduated:` token, append `· Graduated: AP-<n>` to its `Links:` line;
  self-check; write. The append-only checker's sanctioned-growth rule
  (Links prefix-extension) is exactly what this produces.
- **The skill** — `/incident` lives beside the other authoring skills on
  the vendor-neutral surface (pointer shims reach it per AP-25);
  instructions: gather tags/fields, cite links (introducing design,
  implementing PRs — the AP-7 link contract), invoke `add-incident`,
  never hand-edit the registry; graduation goes through `graduate` with
  the operator named.

### 3.2 Boundaries and non-goals

- **Not solved here** — *whether* to file (judgment, PRD NG-3);
  discovery/attribution (#53); principle drafting at graduation (the
  registry→principle direction stays operator-fed — PRD Q2).
- **Owned by a future design** — the engine-era capture feeder (WR-1
  there): when traces exist, drafted entries arrive as proposals; the
  writer designed here is the thing those proposals call.
- **Implementation must not infer** — that writer verbs may join the
  `gate` umbrella (they must not: the umbrella stays report-only); that
  any file other than the configured registry may be written; or that
  refusal may leave a partial write (atomic or nothing).
- **A stated delta from docgate-v1#DINV-3** — v1 pinned "the only writes
  in the system are the two artifact files." docgate-v2 already extended
  the write set with the two index artifacts (generated projections,
  without superseding v1 — the house precedent for additive artifact
  growth). This design extends it differently in kind: the registry is a
  **source document**, not a projection. The constraint that survives,
  restated as DINV-1 below: *gate verbs never mutate; every write in the
  system is performed by `cmd` from a dedicated producer's output, to an
  enumerated path list* — now: two projection artifacts, two index
  artifacts, one registry. Both documents remain simultaneously true
  read as scope statements of their own verb families; if the operator
  reads DINV-3's letter as absolute, the honest alternative is
  superseding docgate-v1 — flagged for the review of this PR, decided
  there.

### 3.3 Invariants

- **DINV-1** — gate verbs never mutate (docgate-v1#DINV-3 carried
  forward); writer verbs write exactly the configured registry path,
  atomically, and only after their would-be output passes the lint's
  own checks.
- **DINV-2** — ID allocation happens only inside the writer: no flag,
  argument, or skill path lets a caller choose an ID.
- **DINV-3** — one schema definition serves writer, lint, and template;
  a schema change is one edit visible to all three (the template ships
  with updated goldens in the same PR, the amendment model's rule).
- **DINV-4** — refusal is total: on any self-check defect the registry
  is byte-identical to its pre-invocation state.

### 3.4 Observability

Writer outcomes use the existing defect stream (stderr +
`--defects-json`) for refusals; a successful write prints the allocated
ID on stdout (the skill reports it back to the author). No other
channel; the registry's own git history is the audit log.

### 3.5 Security & privacy

none — matches the header row: one tracked file written locally under
the same review/gate path as any commit; no identity, secret, network,
or egress surface. The writer cannot be invoked into writing elsewhere
(DINV-1's enumerated path).

## 4. Alternatives considered

| Alternative | Why rejected |
|---|---|
| a separate `pyxis-incident` binary | two pins, two schema copies, two drifts (AP-11); the lint and writer must share one schema and already share a home |
| writer as a skill-only discipline (prompt renders the entry) | the exact honor system INV-35 exists to end; ID allocation in a prompt is the first thing to typo |
| registry as YAML, rendered to markdown | the registry is read by grep and humans in place; a generated face adds the projection machinery for no consumer; entries are already closed-shape |
| letting `add-incident` also auto-commit | writes and commits are different authorities; the author reviews the diff like any change; the gate holds it to shape either way |
| putting writer verbs in the `gate` umbrella for convenience | report-only umbrella is a load-bearing property (hooks run it unattended); a mutating umbrella is how "check my tree" grows side effects |

## 5. Risks

| ID | Risk | Impact | Mitigation | Verification / evidence |
|---|---|---|---|---|
| R1 | schema triple-consumer coupling makes format evolution heavy | changing the entry shape touches writer+lint+template | that weight is the point (one definition); evolution is one schema edit + goldens, versioned via the template pin | DINV-3 fixture: a schema tweak flips all three or none |
| R2 | writer verbs normalize hand-capture away and the skill becomes a bottleneck in odd contexts (no Go toolchain) | capture skipped | hand-writing remains physically possible and lint-checked — the writer is the paved road, not a locked gate; the resolver's release-binary path removes the toolchain need | PRD guardrail measurement over the next five entries |
| R3 | atomic-write rename semantics differ on exotic filesystems | torn registry | temp-in-same-dir + rename is the stdlib-portable idiom; the self-check re-reads the written bytes as a final verify | write-crash fixture (kill between temp and rename leaves original) |

## 6. Acceptance

| ID | Acceptance criterion | Verify | Required evidence | Traces to |
|---|---|---|---|---|
| A-1 | on a fixture registry at I3, `add-incident` writes a lint-green I4; a second call writes I5; prior entries hash-identical | Test | fixture suite | prd:incident-machinery/FR-1 · NR-1 |
| A-2 | a write that would fail lint (empty tags, missing field) is refused with the lint's defect and a byte-identical registry | Test | refusal fixtures | prd:incident-machinery/UB-1 · DINV-4 |
| A-3 | `graduate` grows only the target `Links:` line (append-only check green); double-graduation and unknown-ID red | Test | fixtures | prd:incident-machinery/FR-2 |
| A-4 | writer, lint, and template consume one schema definition (import-graph + template-golden check) | Test | suite | prd:incident-machinery/FR-3 · DINV-3 |
| A-5 | the `/incident` skill exists on the vendor-neutral surface, reachable through the existing pointer shims, and instructs writer-only capture | Inspection | file listing in the implementing PR | prd:incident-machinery/FR-4 |
| A-6 | the `gate` umbrella performs zero writes with the writer verbs present (report-only preserved) | Test | fs-guard in suite | DINV-1 |

## 7. Open questions

| ID | Question | Owner | Blocks | Status | Resolution |
|---|---|---|---|---|---|
| Q1 | Should `add-incident` accept a tracker cross-reference argument written into `Links:` (PRD Q1)? | implementation | nothing — additive flag | open | lean yes at implementation; the link contract wants it |
| Q2 | Does the writer family eventually host other sanctioned mutations (e.g. wiki `Last reviewed` bumps), or stay incident-only? | operator | nothing | open | — |

## Principles

- **AP-10 — honored; the class's defining application.** ID allocation, rendering, and the sanctioned mutation move from prompt discipline to code; INV-35 stops being aspirational.
- **AP-11 / INV-30 — honored.** One schema, three consumers; one binary hosts lint and writer so the predicate and the producer cannot drift.
- **AP-13 — honored.** A capture "succeeded" only if the written registry passes the lint that gates it — verified artifact, not exit-code optimism.
- **AP-17 / INV-6 — honored.** Refusal is total and byte-safe (DINV-4); ambiguity never writes.
- **AP-20 — honored.** The write surface is one enumerated path (DINV-1); the blast radius of a writer bug is one file under version control.
- **AP-25 — honored.** The skill is vendor-neutral content; vendor dirs reach it by pointer.
- **INV-35 — honored; implemented.** Append-only, stable IDs, one coded writer, graduation as the one sanctioned mutation.
- **Deviation:** none — the docgate-v1#DINV-3 delta is handled by scope statement and explicit review flag (§3.2), not silent reinterpretation.
