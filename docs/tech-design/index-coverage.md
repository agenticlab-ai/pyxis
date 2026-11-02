# Index coverage — the requirements-coverage face: served, by what, and what remains

| Field | Value |
|---|---|
| **Status** | Approved (2026-07-08 — operator, via the introducing PR review) *(merge is the freezing event; "superseded" is computed by the index, never written here)* |
| **Owner** | @mdarabi |
| **Approver(s)** | @mdarabi (operator) |
| **Date** | 2026-07-08 |
| **Relevant-when** | asking which requirements have no design; changing `gen-index`/`verify-index` output; planning design-phase work against the corpus; consuming `index.json` programmatically |
| **Scope** | The additive coverage face of the tech-design index (FR-26): the requirement universe, the served/unserved computation, and both faces' rendering. **Not**: the core index graph (docgate-v2 §3.6, unchanged), `Serves` semantics (serves-hardening), corpus mutability (fence-completion). |
| **Tracking** | issue #54 · PR #56 (requirements) |
| **Builds-on** | docs/tech-design/docgate-v2.md · docs/tech-design/serves-hardening.md |
| **Supersedes** | — |
| **Serves** | prd:docgate/FR-26 |
| **Security/privacy impact** | none — deterministic generation over tracked files; no new surface |
| **Template version** | tech-design-template/v1 |

## 1. Problem

"Which requirements still have no design?" is the design phase's queue,
and answering it today is archaeology across 67 baseline features, 15
VNEXT requirements, and the ledger units (#54 C4). The index already
computes each design's `serves[]` and even the inverse map — but only
over anchors some design *already* cites: the unserved remainder, the
actually useful planning view, appears nowhere.

## 2. Decision

**Decision:** `gen-index` computes a **coverage face** and emits it into
both existing index artifacts, additively. The requirement universe is
the union of (a) every corpus ID extracted by the serves-hardening scan
(one extractor, two consumers — AP-11) and (b) every **live** unit of
every ledger's `effective.source.json`. Coverage joins that universe
against the existing `serves_inverse` restricted to **live** designs:
each requirement lists its serving designs; the remainder is emitted as
an explicit, sorted `unserved` list per family. `index.json` gains one
additive top-level key (`coverage`); `index.md` gains one generated
section (counts per family + the unserved lists). `verify-index`
byte-verifies it all exactly as before — the projection doctrine
unchanged. docgate-v2's §3.6 core shape is untouched: additive evolution
is the data doctrine (AP-6), and the schema_version field exists for
consumers to key on.

## 3. The design

### 3.1 Mechanics

- **Universe** — corpus IDs from the shared extractor (serves-hardening
  §3.1); ledger units filtered to `status: live` (superseded/retired
  units are not coverage targets — their successors are). Sorted
  family-then-numeric for determinism.
- **Join** — a requirement is *served* if ≥1 **live** design's `serves[]`
  names it (`corpus:<ID>` or `prd:<feature>/<unit>`); superseded designs
  never count (their successors carry the anchors forward or the
  requirement honestly returns to unserved).
- **Emission** — `index.json`: `coverage: {universe_size, served: {<anchor>:
  [slugs]}, unserved: {<family>: [ids]}}` under the existing
  schema_version discipline (additive). `index.md`: a `## Coverage`
  section — one summary table (family · total · served · unserved) and
  the unserved IDs verbatim, so the planning read is one glance.
- **Determinism** — sorted keys, no timestamps, same rules as the rest
  of the index; golden-pinned.

### 3.2 Boundaries and non-goals

- **Not solved here** — *quality* of coverage (a design serving FR-3
  badly still counts — semantic fit is review's, docgate-v2#DINV-2);
  DoD/EVT/other non-requirement units (coverage targets requirement
  rows: corpus IDs and live FR/UB/NFR/NR units — the planning question);
  prioritizing the unserved list (human judgment).
- **Owned by a future design** — the stage-6 Contract Compiler's richer
  join (binding blocks, verification graphs) — this face is deliberately
  its thin precursor, not its implementation.
- **Implementation must not infer** — that unserved means "work now"
  (some corpus areas are engine-era by intent); the face reports, the
  operator prioritizes.

### 3.3 Invariants

- **DINV-1** — coverage is derived only from committed inputs (corpus
  files, source maps, design headers): same-input ⇒ same-bytes, verified
  by the existing byte-compare.
- **DINV-2** — the core index keys of docgate-v2 §3.6 are byte-stable
  under this change for any corpus with no coverage-relevant delta: the
  face is additive, never a rewrite.
- **DINV-3** — only live designs serve, only live units are targets: the
  coverage face can never launder a supersession.

### 3.4 Observability

The rendered `## Coverage` section *is* the observability surface — the
class-health dashboard pattern docgate-v2 §3.4 established, extended to
the requirement corpus. Defects (verify-index mismatches) ride the
existing stream.

### 3.5 Security & privacy

none — matches the header row: deterministic generation over tracked
files already read by the index; nothing new crosses any boundary.

## 4. Alternatives considered

| Alternative | Why rejected |
|---|---|
| a separate `coverage` verb and artifact | a third projection with its own verify machinery for data the index already half-computes; AP-11 says widen the fence |
| coverage as a CI-only report (not committed) | consumers read the repo, not CI (docgate-v2's own rejection of uncommitted indexes) |
| counting superseded designs as serving | laundering: dead decisions would mask live gaps (DINV-3) |
| including DoD/EVT/G/J units in the universe | those are verification/derivation units, not requirement rows; noise in the planning view — revisit when the compiler consumes the face |

## 5. Risks

| ID | Risk | Impact | Mitigation | Verification / evidence |
|---|---|---|---|---|
| R1 | the unserved list is large and reads as a backlog indictment | planning noise | grouped per family with counts first; the §3.2 boundary states unserved ≠ work-now | rendered fixture review |
| R2 | corpus extractor drift between this face and serves-hardening's resolution | the two disagree about what exists | one extractor, imported by both (AP-11); a fixture asserts set-equality | shared-extractor test |
| R3 | index churn: every ledger amendment now touches index bytes | wider diffs on amendment PRs | already true for design PRs; the coverage delta is the *point* of the diff — reviewers see requirement coverage move | goldens |

## 6. Acceptance

| ID | Acceptance criterion | Verify | Required evidence | Traces to |
|---|---|---|---|---|
| A-1 | a fixture corpus + ledger + design set yields the expected served/unserved partition in both faces | Test | goldens | prd:docgate/FR-26 |
| A-2 | a hand-edited coverage section red under `verify-index`; regenerated green | Test | fixture | prd:docgate/FR-26 · DINV-1 |
| A-3 | superseding a design moves its solely-served anchors back to unserved | Test | supersession fixture | DINV-3 |
| A-4 | existing `index.json` consumers see the docgate-v2 §3.6 keys byte-unchanged on a coverage-neutral regeneration | Test | goldens diff | DINV-2 |

## 7. Open questions

| ID | Question | Owner | Blocks | Status | Resolution |
|---|---|---|---|---|---|
| Q1 | Should NR units join the universe (they protect behavior rather than demand it)? Included in v1 as requirement rows; drop by successor if noise | operator | nothing | open | included; revisit on real use |

## Principles

- **AP-6 — honored.** Additive, versioned schema evolution; the core shape survives byte-stable (DINV-2).
- **AP-10 / AP-13 — honored.** The planning view is generated and byte-verified, never hand-curated; "FR-3 is served" is a verifiable artifact.
- **AP-11 / INV-30 — honored.** One corpus extractor (shared with serves-hardening), one index generator, one verify.
- **AP-19 — honored.** Deterministic set arithmetic at generation time; zero tokens.
- **AP-24 — honored.** The face is derived from human-approved documents and governs nothing — it informs prioritization, never performs it.
- **Deviation:** none.
