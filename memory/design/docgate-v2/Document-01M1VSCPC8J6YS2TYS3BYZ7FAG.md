# docgate v2 — universal document lint and the design index

| Field | Value |
|---|---|
| **Status** | Approved (2026-07-07 — operator, in doc-review) *(merge is the freezing event; "superseded" is computed by the index, never written here)* |
| **Owner** | @mdarabi |
| **Approver(s)** | @mdarabi (operator) |
| **Date** | 2026-07-07 |
| **Relevant-when** | adding or changing a v2 verb (design/wiki/rollup/governing/incident checks, index generation); changing any document class's header schema; touching the design graph or its flags |
| **Scope** | The v2 checks: design-class lint + generated index (the scope frozen in tech-design-conventions §3.4), and the four classes issue #37 found unguarded — wiki headers, rollup headers, governing-doc immutability, incident registry shape. **Not**: semantic judgment (contradiction, quality — review + Contract Compiler), PRD content quality (§8/EARS — VNEXT-1), the installer (#36). |
| **Tracking** | issue #37 · introducing PR (filed on approval) |
| **Builds-on** | docs/tech-design/docgate-v1.md · docs/tech-design/tech-design-conventions.md · docs/tech-design/repo-layout-and-system-footprint.md |
| **Supersedes** | — |
| **Serves** | prd:docgate/WR-1 · design:tech-design-conventions#3.4 · doc:docs/wiki/README.md · doc:AGENTS.md |
| **Security/privacy impact** | none new — same hermetic binary, no network in the gate path; the governing-doc check *adds* protection for the trust root (see §3.5) |
| **Template version** | tech-design-template/v1 |

## 1. Problem

v1 guards the PRD ledger machine-tight, but five governed classes carry
their rules as prompt text only: tech designs (closed header schema,
mandatory subsections, supersession graph — all specified, none checked),
wiki entries (routing headers), the live rollups (their own metadata),
the governing docs (rule 2 — the repo's strongest rule with zero machine
backing), and the incident registry (append-only, shaped entries). The
conventions also promise a **generated design index** — the class's
projection — which cannot exist before its generator. Issue #37 holds
the operator audit.

## 2. Decision

**Decision:** v2 extends the same binary with five class linters and one
generator, all composed into the existing `gate` umbrella (DINV-4
unchanged — desk and CI identical): `lint-tech-designs` (header schema, typed
`Serves` grammar, mandatory-by-name §Observability + §Security & privacy
with the none+reason escape and the security header↔subsection pair,
slug rules, reference resolution); `gen-index` + `verify-index` (the
design graph as `index.md` + `index.json` — generated, byte-verified,
sole writer docgate, exactly the projection pattern of v1's fold);
`lint-wiki` and `lint-rollups` (routing/metadata header presence and
shape); `check-governing` (any diff touching the vision or principles is
a defect unless the commit carries the `Governing-Change: operator`
trailer — attestation, not authorization: the machine checks presence,
review checks truth); `lint-incidents` (append-only entries, dense `I…`
IDs, four named fields). Class rules live in `internal/docgate/schema`
as registries, like v1's table schemas. Semantic judgment stays human
everywhere.

## 3. The design

### 3.1 New verbs and their homes

| Verb | Checks | Needs merge base | Package |
|---|---|---|---|
| `lint-tech-designs` | closed header fields; `Serves` ref grammar (`prd:` `corpus:` `design:[#anchor]` `doc:` or `—`); refs resolve (targets exist; `prd:` anchors against the ledger's source map); mandatory-by-name subsections + security pair; slug `[a-z0-9-]+`; `Template version` pin | no | `internal/docgate/techdesigns` |
| `gen-index` / `verify-index` | **the tech-design index** — graph: liveness computed from `Supersedes` (acyclic, resolving, ≤1 live successor); lineage incl. transitive; `Serves` inverse map; `stale-serves` / `stale-builds-on` flags; emits `index.md` + `index.json` deterministically | no | `internal/docgate/techdesigns` |
| `lint-wiki` | header rows present (In one line, Relevant-when, Authority, Sources, Last reviewed); date parses; Authority row contains "descriptive" | no | `internal/docgate/gate` |
| `lint-rollups` | header rows present (In one line, Relevant-when, Authority, Update rule, Seeded) on both rollups | no | `internal/docgate/gate` |
| `check-governing` | `git diff --name-status <base>` touching `docs/product-vision.md` or `docs/architecture-principles.md` → defect unless HEAD commits carry `Governing-Change: operator`; principles additionally append-only (no deleted/modified existing lines) | yes | `internal/docgate/gate` |
| `lint-incidents` | entries append-only vs merge base; IDs `I<n>` dense; each entry carries the four named fields | yes (append check) / no (shape) | `internal/docgate/gate` |

The umbrella runs all merge-base-free verbs everywhere (pre-commit
included); `check-governing` and the incidents append check join the
merge-base path (pre-push, CI) beside `check-immutable` and
`check-rollups`.

### 3.2 Boundaries and non-goals

- **Not checked, deliberately** — contradiction between designs (semantic;
  supersession discipline + review own it); prose quality; PRD §8
  readiness (Contract Compiler); whether an operator trailer is *true*
  (review's job — the trailer is attestation, like declared blast radius).
- **Owned elsewhere** — installer and manifest drift (#36, phase 4); the
  **ux-design class**, when it exists, arrives as its own linter + index
  under fresh class-named verbs — v2 claims nothing generic;
  wiki *content* freshness (the `Last reviewed` value is checked for
  shape, not honesty).
- **Must remain unchanged** — v1 verbs and semantics; the placeholder
  corpus exemption; fold purity (index generation lives beside, not
  inside, the fold).

### 3.3 Invariants

- **DINV-1** — the index is generated-only: sole writer `docgate
  gen-index`; `verify-index` byte-rejects hand edits (the v1 projection
  invariant, applied to the design class).
- **DINV-2** — no semantic checks: every v2 defect is decidable from
  bytes + git history; anything requiring judgment is out of scope by
  construction.
- **DINV-3** — liveness is computed, never written: no document carries
  its own "superseded" state; only the index says so, derived from
  `Supersedes` edges.
- **DINV-4** — the governing check fails closed on absence of the
  trailer, never on its content: attestation-not-authorization.

### 3.4 Observability

Unchanged from v1 (defects on stderr + `--defects-json`; red required
check is the alert), plus: the generated `index.md` doubles as the
class's health dashboard — flags (`stale-serves`, `stale-builds-on`,
unresolved refs) are visible in the rendered index, not only as defects.

### 3.5 Security & privacy

none new — matches the header row: same hermetic binary, no network, no
secrets. Net gain: the trust root (vision/principles) moves from
prompt-protected to machine-guarded (`check-governing`), closing the
inversion issue #37 named — the strongest rules had the weakest backing.

### 3.6 Tech-design index shape (pinned by goldens)

`index.json`: `{schema_version, generated_by, tool_version, designs:
{<slug>: {status: live|superseded, superseded_by, current_successor,
builds_on[], serves[], flags[]}}, serves_inverse: {<anchor>: [slugs]}}`.
Emitted at `docs/tech-design/index.{json,md}` — class-scoped by location. `index.md`: one table (slug · status · serves · builds-on · flags),
banner-marked GENERATED. Both deterministic: sorted keys, no timestamps.

## 4. Alternatives considered

| Alternative | Why rejected |
|---|---|
| separate v2 binary | two pins, two drifts; AP-11 — one predicate |
| CODEOWNERS for governing docs | platform-specific (bucket-3 logic), invisible at desk; the trailer check runs identically at all rungs |
| index as CI artifact (not committed) | consumers (agents, compiler) read the repo, not CI; committed + verified matches the projection doctrine |
| checking design contradiction textually | semantic; false confidence is worse than honest absence (DINV-2) |

## 5. Risks

| ID | Risk | Impact | Mitigation | Verification / evidence |
|---|---|---|---|---|
| R1 | header lint red-flags the four existing frozen designs | unfixable defects on immutable docs | grandfather clause: docs merged before v2's gate carry `grandfathered` flag in the index, lint-exempt by merge date; conventions already anticipate grandfathering | fixture over the real corpus green at introduction |
| R2 | governing trailer friction for the operator | annoyed operator | one trailer line, named in the defect remedy; only fires on two paths | defect message review |
| R3 | index churn on every design PR | noisy diffs | deterministic emission; index regenerates only when design files change | goldens |

## 6. Acceptance

| ID | Acceptance criterion | Verify | Required evidence | Traces to |
|---|---|---|---|---|
| A-1 | one red fixture per defect class across all five linters | Test | fixture manifest | §3.1 |
| A-2 | the real repo (four frozen designs incl. this one, wiki, rollups, incidents, governing docs) gates green at introduction | Test | CI run | R1 |
| A-3 | index: hand-edit → red; regenerate → green; liveness/flags match a fixture graph incl. a supersession chain | Test | goldens | DINV-1/3 |
| A-4 | governing: untrailed diff red both paths; trailed diff green; principles line-deletion red even with trailer | Test | temp-repo fixtures | §3.1, DINV-4 |
| A-5 | `Serves` refs of every live design resolve (this document's own refs included) | Test | lint run | §3.1 |

## 7. Open questions

| ID | Question | Owner | Blocks | Status | Resolution |
|---|---|---|---|---|---|
| Q1 | Range shorthand (`FR-1..FR-8`) in `Serves`: v1's design deferred it here — accept in the ref grammar, or require enumeration? | operator | `lint-tech-designs` ref grammar | open | — |

## Principles

- **AP-10 — honored.** Five prompt-side rules become checks; the audit's inversion (sacred rules, zero backing) closes.
- **AP-11 / AP-13 / AP-14 — honored.** One binary, one umbrella; green = verified bytes; goldens execute the contract, the real corpus is fixture zero again.
- **AP-17 — honored.** Fail closed everywhere; the governing check's only escape is an explicit, visible attestation.
- **AP-21 — honored.** Liveness judged once by the generator and frozen as data in the index; consumers read, never re-derive.
- **AP-24 — honored.** The index (governance data) is generated from the governed documents, never hand-built.
- **Deviation:** none.
