# Incident machinery — coded writer, sanctioned mutations, and the entry template — PRD

| Field | Value |
|---|---|
| **In one line** | The incident registry's writer becomes code: ID allocation, entry capture, graduation, and the entry template — hand-written entries end. |
| **Relevant-when** | capturing an incident or graduating a lesson into a principle; planning, building, or reviewing the registry writer verbs; changing the registry's entry format; checking INV-35 conformance |
| **Status** | Problem & solution review — via the introducing PR's operator review · launch review pending, post-implementation |
| **Tier** | T1 |
| **Owner** | @mdarabi |
| **Approver(s)** | @mdarabi (operator) |
| **Contributors** | operator (product) · factory agents (implementation) |
| **Informed** | future incident authors (human and agent) |
| **Date** | 2026-07-08 created · 2026-07-08 last updated |
| **Appetite** | small: one review round; the writer and the lint share one schema; zero rollout/support burden (repo-internal tool) |
| **Target release** | with the review-wave implementation PR (#54) |
| **Tracking** | issue #54 (doc-system review corrective wave) |
| **Artifacts** | docs/observability/incidents.md (the registry) · issue #53 (automated discovery — deferred, engine-era) |
| **Template version** | prd-template/v1 |

## 1. Summary — the one-pager

- **Problem** — INV-35 requires the incident registry to have one coded writer with code-allocated IDs; in practice I1 was hand-authored, no writer exists, and graduation (the one sanctioned mutation) is a hand edit. Failure memory currently grows on the honor system.
- **For whom** — agents and the operator capturing incidents; design-stage consumers who grep the registry and must trust its shape.
- **Why now** — the registry is one entry old; the writer arrives before habits form, and the review wave (#54 C2) already funds the change.
- **Direction** — writer verbs in the existing doc-system tool (one binary, one predicate): `add-incident` allocates the next dense ID and writes a conformant entry; `graduate` performs the sanctioned `Links:` growth; a version-pinned entry template is the single schema both the writer and the lint consume; a vendor-neutral `/incident` skill drives capture through the writer.
- **Appetite** — as header.
- **Success** — 100% of registry mutations after the writer ships go through it: every new entry passes `lint-incidents` on first commit, zero hand-allocated IDs (acceptance-style; verified over the next five entries).
- **Guardrail** — no existing entry byte-changes except sanctioned `Links:` growth (the append-only check stays green through every writer operation).
- **Top risks** — writer/lint schema drift (mitigated: one schema, two consumers); concurrent ID allocation races (caught mechanically by density lint); the skill bypassed by habit (mitigated: prompt rule + lint red on malformed hand entries).

## 2. Problem space

Skipped: T1 — the problem is §1's; the registry exists with one hand-authored entry, and INV-35 already demands a coded writer. The evidence is the review finding (#54 C2).

## 3. Goals, metrics, and guardrails

### 3.1 Goals as falsifiable hypotheses

Skipped: T1 — success and guardrail are acceptance-style in §1; no instrumented metrics for a repo-internal writer.

### 3.2 Goals → signals → metrics

Skipped: T1 — see §1 Success/Guardrail; measured from git history and lint verdicts over the next five entries.

### 3.3 Guardrails and decision rule

Skipped: T1 — the guardrail is §1's (append-only invariance under writer operations); breach = incident entry + writer fix before further captures.

### 3.4 Non-goals and waiting room

| ID | Non-goal | Why refused |
|---|---|---|
| NG-1 | Automated incident discovery and causal attribution (watching signals, walking traces) | engine-era capability; requires the Ledger and traceability planes — deferred as issue #53 |
| NG-2 | Redesigning the entry format | the four-field, tagged, dense-ID shape is decided and practiced; this PRD codifies its writer, not a new shape |
| NG-3 | Gating *when* incidents must be filed | capture remains judgment (a JudgmentPoint); the machinery governs *how* an entry is written, never whether |

| ID | Waiting room | Revisit when / intended version |
|---|---|---|
| WR-1 | Discovery/attribution feeder proposing drafted entries from engine signals | when the engine's Ledger + traceability exist (issue #53) |
| WR-2 | Auto-linking the introducing design and implementing PRs from the trace | same trigger as WR-1 — the link contract is manual until traces exist |

## 4. Solution space

Skipped: T1 — direction in §1; mechanics belong to the technical design (the writer is a verb family in the existing doc-system tool).

## 5. System requirements

### 5.1 Functional requirements

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-1 | When invoked as `add-incident` with a date, a tag list, and the four field values, the writer shall append a conformant entry to the registry at its configured path under the next dense `I<n>` ID — allocation is code, never prompted — and shall refuse the write, changing nothing, if the resulting registry would fail `lint-incidents`. | INV-35's "one coded writer; ID allocation is code" becomes real | Must | med | on a fixture registry ending at I3, `add-incident` writes I4 passing lint; a second call allocates I5; a write that would fail lint (e.g. empty tags) is refused naming the lint defect and the registry is byte-unchanged | Test | §1 |
| FR-2 | When invoked as `graduate` with an incident ID and a principle ID, the writer shall append `Graduated: AP-<n>` to that entry's `Links:` line — the one sanctioned mutation — and shall fail if the entry does not exist or already carries a graduation pointer. | graduation stops being a hand edit to an append-only file | Must | low | graduating a fixture entry grows only its `Links:` line (append-only check green); a second graduation of the same entry red; an unknown ID red | Test | §1 |
| FR-3 | The incident-entry shape shall exist as a version-pinned template (`incident-entry-template/v1`) under the templates directory, from which the writer renders entries and against which `lint-incidents` checks — one schema definition, two consumers. | the vision's "each document kind with a template" reaches the incident class; writer/lint drift becomes structurally impossible | Must | low | the template file exists and is version-pinned; a writer-emitted entry and the template's example both pass lint; writer and lint consume one schema definition in code (inspection) | Test | §1 |
| FR-4 | Where an agent captures or graduates an incident, a vendor-neutral `/incident` authoring skill shall drive the operation through the writer verbs — an agent never hand-allocates an ID or hand-writes an entry. | the prompt-side half of INV-35, matching the other authoring skills | Should | low | the skill exists on the vendor-neutral surface with vendor pointer shims; its instructions invoke the writer verbs and forbid hand edits | Inspection | §1 |

**Non-regression (NR-n)**

| ID | Existing behavior protected | Why protected | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|
| NR-1 | Existing registry entries are byte-unchanged by any writer operation except sanctioned `Links:` growth | the registry is append-only memory; a writer that rewrites history is worse than no writer | med | across all writer-fixture runs, prior entries hash-identical; the registry append-only check green | Test | §1 |

### 5.2 Unwanted behavior and failure handling

| ID | If… (trigger) | Then the system shall… | User sees | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|---|
| UB-1 | If the registry at the configured path is missing or already fails lint | refuse the operation and write nothing | the lint defect naming file and reason | Must | low | missing-file and pre-corrupted-registry fixtures both refuse with named defects; nothing written | Test | FR-1 |
| UB-2 | If two branches allocate the same `I<n>` concurrently | the second to merge shall fail the registry lint (density/append) at its gate, naming the collision | a lint defect naming the duplicate ID | Must | low | a duplicate-ID fixture red at the gate; renumbering the newer entry passes — mirrors the amendment-number race | Test | FR-1 |

### 5.3 Permissions and abuse cases

Skipped: repo-local CLI, no roles, no new surface; the registry's own gate checks are the abuse control.

### 5.4 Data

Skipped: no state beyond the registry file, whose contract is FR-1..3 and the existing gate checks; no retention, privacy, or egress surface.

### 5.5 Interfaces and integrations

Skipped: T1 — the writer is a verb family inside the existing doc-system tool; its only interface is the registry file and the existing gate wiring.

### 5.6 Non-functional requirements

Skipped: inherits the doc-system tool's NFRs (single static binary, hermetic, repo Go rules) — see `prd:docgate/NFR-1..3`.

### 5.7 Instrumentation — the tracking plan

Skipped: no runtime telemetry; success is measured from git history and lint verdicts (§1).

### 5.8 Definition of Done

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-1 | All Must FRs pass their fit criteria | FR-1..3 | Test | CI fixture-suite run | yes |
| DOD-2 | No unwanted-behavior or non-regression row broken | UB-1..2, NR-1 | Test | suite result | yes |
| DOD-3 | The `/incident` skill exists on the vendor-neutral surface with pointer shims | FR-4 | Inspection | file listing in the implementing PR | yes |
| DOD-4 | Registry scaffolding and AGENTS.md name the writer as the sole entry path | FR-1, FR-4 | Inspection | doc diff in the implementing PR | yes |
| DOD-5 | Rollups updated in the same change (the capability guard is green, not attested around) | prd:docgate/FR-10 | Test | gate run | yes |

### 5.9 AI behavior and evals

Skipped: no model-driven behavior — the writer is deterministic code; capture judgment stays with the invoking human or agent (NG-3).

## 6. Delivery

Skipped: T1 — repo-internal tool behind the already-live gate; release criteria fold into §5.8 (blocking rows), reversibility is trivial (the writer is additive; hand capture remains physically possible and is caught by lint, not by lock-in).

## 7. Alignment

### 7.1 FAQ

Skipped: T1 — no customer surface; the one internal question ("why a writer for a one-entry registry?") is answered in §1 Why-now.

### 7.2 Open questions

| # | Question | Owner | Needed by | Status |
|---|---|---|---|---|
| Q1 | Should `add-incident` also take a tracker cross-reference (issue/PR) and write it into `Links:` in one step? | tech design | implementation | open |
| Q2 | Does graduation also draft the principle text, or is drafting entirely the operator's? (The registry→principle direction is operator-fed by rule 6.) | operator | nothing — graduation pointer works either way | open |

### 7.3 Decision log

| Date | Decision | Alternatives considered | Why | Who |
|---|---|---|---|---|
| 2026-07-08 | the writer lives in the existing doc-system tool as new verbs | a separate `incident` binary | one binary, one predicate (AP-11); the lint and the writer must share one schema, and they already share a home | @mdarabi (via review wave #54) |

### 7.4 Changelog

| Date | Change | Author |
|---|---|---|
| 2026-07-08 | 000-base drafted for review (review wave #54, finding C2) | factory agent for @mdarabi |

## 8. Contract readiness — the executability gate

| Check | Pass? | Notes |
|---|---|---|
| Problem basis accepted — §1 problem accepted (T1) | pass | INV-35 states the requirement; I1's hand authorship is the evidence |
| Zero solution-blocking TBDs or open questions (§7.2) | pass | Q1 blocks implementation detail only; Q2 blocks nothing |
| Every Must FR has a fit criterion and a verification method (§5.1) | pass | |
| Every requirement row carries a risk tier; high/critical rows have a declared safety plan | pass | no high/critical rows; writer refusal path (UB-1) is the safety behavior |
| Non-goals drawn (§3.4); non-regression rows named (§5.1) | pass | |
| Every primary job is covered by a requirement, a non-goal, or a waiting-room entry | pass | capture→FR-1/FR-4; graduation→FR-2; discovery→NG-1/WR-1 |
| Every key flow has functional, failure, and non-regression coverage | pass | the one flow (capture→lint→merge) is FR-1 + UB-1..2 + NR-1 |
| Every primary metric has instrumentation or an explicit measurement plan | pass | acceptance-style measurement plan in §1 (T1) |
| Every blocking DoD row traces to a requirement, guardrail, release criterion, or rubric (§5.8) | pass | |
| Risks and assumptions have owners (§6.3) | pass | §6 skipped at T1; §1 top risks carry their mitigations; owner is the implementing change |
| Human-judgment criteria name an owner and a rubric (§5.8) | pass | none present |
| DoD complete: every blocking row sourced and evidenced; every n/a explained (§5.8) | pass | no n/a rows |
