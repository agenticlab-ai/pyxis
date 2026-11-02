# Feature draft reopen — what a reopened draft starts from and what its lock writes — design-brainstorm record

| Field | Value |
|---|---|
| **In one line** | A reopened feature draft starts from the locked version's stored text, and its lock writes only what changed against that version, with per-feature numbering continuing across versions — settled as an add-on design that builds on feature-draft-v1 without replacing it. |
| **Relevant-when** | designing or changing how a locked feature is reopened: what the new draft starts from, what its lock writes to memory against the binding version, how brainstorm, review, and problem numbers continue across versions, or how a retired requirement stays resolvable for an earlier version's plan; checking why feature-draft-v1 was narrowed rather than superseded |
| **Basis** | `prd:feature-draft` (merged ledger; FR-20, FR-21, FR-22, FR-43, FR-44, UB-4) |
| **Authority** | @mdarabi (operator — product and engineering) |
| **Date(s)** | 2026-09-24 · 2026-09-24 (review: D-3) |
| **Status** | gated: ready-for-design |
| **Tracking** | — |
| **Feeds** | `docs/tech-design/feature-draft-reopen.md` |
| **Prior records** | `design-brainstorm:feature-draft-tech-design` · `brainstorm:feature-draft` |
| **Template version** | design-brainstorm-record/v1 |

## Orientation

| Source | Why loaded |
|---|---|
| `docs/tech-design/index.md` | the designs whose Serves touch `prd:feature-draft`: `feature-draft-v1` and `contract-gate-v2`, both live |
| `docs/tech-design/feature-draft-v1.md` and `docs/solution-shapes/feature-draft-v1.md` | the live design that owns the draft lifecycle, the reopen included (§3.1, S-1 to S-3) |
| `docs/tech-design/contract-gate-v2.md` | builds on `feature-draft-v1` and serves `prd:feature-draft/FR-31` |
| `docs/tech-design/tech-design-conventions.md` | how a live design is narrowed or superseded |
| `docs/brainstorms/feature-draft-tech-design.md` | the predecessor record: the lifecycle (D-2), one compile at lock preparation (D-11), each file's exact text stored (D-13) |
| `docs/brainstorms/feature-draft.md` | the stage-1 record: a change reopens the feature as a new draft (D-5, superseded by D-25: a reopen starts from the locked PRD and tech spec) |
| `docs/wiki/glossary.md` | "locked version" and "reopen" |
| `docs/architecture-principles.md` | AP-7, AP-10, AP-17, AP-21, AP-22, RW-5 |
| `docs/observability/incidents.md` | I9: a requirement retired by a later change broke a frozen plan's traces |

**Overlaps surfaced:** `feature-draft-v1` owns the reopen but leaves undecided what a reopened draft starts from, what its lock writes against the locked version, and how its files are numbered (F-4) — so it does not cover this effort. `contract-gate-v2` builds on `feature-draft-v1` (F-2).

## Design questions

- **DQ-1**: Where is the reopen rule written down before planning: an add-on design that builds on `feature-draft-v1`, a successor that supersedes it, or the implementation plan's acceptance criteria?
- **DQ-2**: What does a reopened draft's working tree start from?
- **DQ-3**: What does the next version's lock preparation write to memory, relative to the binding version's records?
- **DQ-4**: How are a reopened draft's brainstorm records, review reports, and requirement problems numbered?
- **DQ-5**: What does lock preparation do when a reopened draft's PRD and tech-spec text equal the binding version's? *(raised by the design review, FND-3)*

## Panel log

- **DQ-1**: opens — fired — a new design document beside a live one is a document-boundary seam; three shapes authored in session under orthogonal constraints (one document per lifecycle, no cascade to other designs, no new document), presented as the question's options
- **DQ-2**: extends — skipped — the reopen command exists in `feature-draft-v1` (§3.1, S-2); this fixes only its starting content
- **DQ-3**: extends — skipped — the lock-preparation compile exists (§3.6, §3.7, S-3); this fixes its behavior against a binding version; two shapes hand-authored for the record
- **DQ-4**: extends — skipped — the per-feature record families exist (S-3); this fixes their numbering across versions
- **DQ-5**: extends — skipped — lock preparation exists; this fixes one refusal case; two shapes put to the operator in the design review

## Decisions

### D-1 — The reopen rule is an add-on design

- **Question**: Where should the reopen rule be written down before we plan: a small add-on design building on the tech spec, a successor replacing it (which forces replacing `contract-gate-v2` too), or the plan's acceptance criteria?
- **Answer**: A small add-on design that builds on `feature-draft-v1` without replacing it; one PR carries this record, the design, its review, and memory records; then the plan.
- **Reasoning**: Recommended and adopted. Superseding v1 cascades to `contract-gate-v2` (F-2); the plan is not a design document and never enters memory; the rule contradicts nothing in v1, so narrowing is allowed (F-3).
- **Anchor**: 2026-09-24 · turn 0 (the placement question, asked with its three options as this effort opened)
- **Status**: live

### D-2 — Seed from the stored text; write only differences; keep numbering

- **Question**: The rule stated as common to every option: a reopened draft starts from the locked version's stored text, and the next lock writes only what changed.
- **Answer**: As previewed with the chosen option: reopen seeds `drafts/<feature>/` from vN's stored text; the vN+1 lock writes only differences — a changed row supersedes its vN record, a removed row is retired, an unchanged row writes nothing; numbering continues from vN (v1 used 001–004, so v2 starts at 005).
- **Reasoning**: Memory refuses a second active record for a row unless it names the one it replaces (F-1); writing nothing for unchanged rows keeps the lock's change equal to what the approval request lists; a restarted number would land in the previous version's family (F-6).
- **Anchor**: 2026-09-24 · turn 0 (the chosen option's content)
- **Status**: live

### D-3 — A reopened draft with no change is refused

- **Question**: When a reopened draft's PRD and tech-spec text equal the binding version's, does lock preparation refuse, or lock an identical next version?
- **Answer**: Refuse: "no change to the PRD or tech spec — abandon the draft instead"; the binding version stays, and no new lock tag is written.
- **Reasoning**: Recommended and adopted. Under D-2 an unchanged unit writes nothing, so an identical lock would mint a version number with no contract change; abandoning keeps the reason as a Learning.
- **Anchor**: 2026-09-24 · review (the design review's FND-3, put to the operator)
- **Status**: live

## Alternatives

### ALT-1 — Successor tech spec

- **Shape**: `feature-draft-v2` supersedes v1 with the rule added; `contract-gate-v3` supersedes `contract-gate-v2` to re-point its Builds-on at the successor.
- **Trade-offs**: the whole lifecycle stays one live document; costs two full supersessions, each revoking and re-issuing its design's memory decisions, and two full documents for the operator to re-read.
- **Principles**: RW-5 honored (one lifecycle, one document); AP-7 honored (a frozen design evolves by supersession).
- **Incidents**: I9 matched in kind (a later change stranding a frozen artifact) but not triggered: no feature-draft plan exists yet — checked `docs/observability/incidents.md` I1–I12.
- **Verdict**: rejected — the cascade to `contract-gate-v2` (F-2) doubles the change for a rule that contradicts nothing in v1 (D-1).
- **From**: DQ-1 · panel

### ALT-2 — Add-on design building on v1

- **Shape**: a one-page design that builds on `feature-draft-v1` and serves the reopen rows; v1 stays live and unedited.
- **Trade-offs**: no cascade and a small review; two live designs describe the lifecycle, a narrowing the index shows.
- **Principles**: RW-5 strained (a lifecycle mechanic lives beside the lifecycle's document), honored in that the reopen rule revises independently; AP-7 honored (v1 stays frozen).
- **Incidents**: none matched — checked `docs/observability/incidents.md` I1–I12.
- **Verdict**: chosen — D-1.
- **From**: DQ-1 · panel

### ALT-3 — The rule in the plan's acceptance criteria

- **Shape**: no new design; the reopen task's acceptance criteria state the rule, approved in the plan PR.
- **Trade-offs**: zero proposals before the plan; a design choice lives in a plan, which is not a design document and never enters memory, so the rule would survive only in code.
- **Principles**: AP-10 strained (the rule would hold by code alone, with no design stating it); RW-5 strained (a design decision outside a design).
- **Incidents**: none matched — checked `docs/observability/incidents.md` I1–I12.
- **Verdict**: rejected — D-1.
- **From**: DQ-1 · panel

### ALT-4 — Differences only

- **Shape**: the vN+1 lock compares each compiled unit with the binding version's active record: a changed unit supersedes it, a removed unit is revoked, an unchanged unit writes nothing; the literal PRD and tech-spec Documents are superseded.
- **Trade-offs**: the lock writes exactly the change the approval request lists; a locked version is read as "memory as of its lock", not as one batch of records.
- **Principles**: AP-21 honored (the locked version downstream reads is what was judged); AP-22 honored (recompiling an unchanged draft writes nothing new).
- **Incidents**: I9 addressed together with v1's plan basis — see F-10, F-11.
- **Verdict**: chosen — D-2.
- **From**: DQ-3 · hand-authored

### ALT-5 — Full re-issue

- **Shape**: every unit of vN+1 is written anew, each superseding its vN record, unchanged units included.
- **Trade-offs**: each version is one complete batch of records; costs a supersession per unchanged row and a lock whose diff hides the real change among re-issued records.
- **Principles**: AP-21 honored; AP-22 strained (an unchanged draft still writes a full new batch).
- **Incidents**: none matched — checked `docs/observability/incidents.md` I1–I12.
- **Verdict**: rejected — D-2 chose differences only.
- **From**: DQ-3 · hand-authored

### ALT-6 — Refuse a no-change lock

- **Shape**: lock preparation refuses a reopened draft whose PRD and tech-spec text equal the binding version's, naming the abandon remedy.
- **Trade-offs**: every version number marks a contract change; a draft that changed only its reviews or brainstorm records ends by abandonment, not a lock.
- **Principles**: AP-17 honored (refuse rather than mint an empty version); AP-28 honored (the abandonment keeps the reason).
- **Incidents**: none matched — checked `docs/observability/incidents.md` I1–I12.
- **Verdict**: chosen — D-3.
- **From**: DQ-5 · hand-authored (design review)

### ALT-7 — Lock an identical version

- **Shape**: the reopened draft locks as the next version with the same contract; memory gains only its new history records.
- **Trade-offs**: any finished draft can lock; costs a version number with no contract change, and later plans based on it.
- **Principles**: AP-21 honored (what locks is what was judged); AP-22 honored (the lock writes no contract records).
- **Incidents**: none matched — checked `docs/observability/incidents.md` I1–I12.
- **Verdict**: rejected — D-3.
- **From**: DQ-5 · hand-authored (design review)

## Seam map

- **S-1**: the reopen command — extends — behind it: seeding the working files from the binding version's stored text — feeds: Serves `prd:feature-draft/FR-20`; Scope "what a reopened draft starts from"
- **S-2**: the lock-preparation compile — extends — behind it: writing only differences against the binding version's active records — feeds: Serves `prd:feature-draft/FR-22`; Scope "what the next lock writes"
- **S-3**: per-feature numbering of brainstorm records, review reports, and problems — extends — behind it: numbers continuing across versions — feeds: Scope "numbering across versions"

## Facts

- **F-1**: Memory admits a record in a family only if it names the family's active head, and the reconciler reports two active claims in one family as a blocking `memory-family-conflict` — source: `internal/memory/cli/cli.go` (admit); `internal/memory/reconcile/analyze.go`
- **F-2**: The contract gate's currency check fails a live serving design that builds on a superseded design; `contract-gate-v2` builds on `feature-draft-v1` and serves `prd:feature-draft/FR-31` — source: `internal/docgate/contractgate/reg_check_currency.go`; `design:contract-gate-v2` header
- **F-3**: A design may narrow a live design through Builds-on while both can be true at once; a contradiction requires supersession — source: `docs/tech-design/tech-design-conventions.md`
- **F-4**: `feature-draft-v1` fixes that a reopen opens `draft/<feature>-v<N+1>` from the mainline while vN binds and that the next lock replaces vN, but not what the new draft starts from, what its lock writes against vN, or how its files are numbered — source: `design:feature-draft-v1` §3.1; solution shape S-1, S-3
- **F-5**: Each locked contract file's exact text is stored as a literal Document, and the draft digest is recomputed from those texts — source: `prd:feature-draft/FR-3`; `design:feature-draft-v1` §3.7; solution shape S-1
- **F-6**: Brainstorm records, review reports, and requirement problems compile to per-feature numbered families (`brainstorm/<feature> · <NNN>`, `review/<feature> · <NNN>`, `prd/<feature> · P-<n>`) — source: solution shape S-3
- **F-7**: A record supersedes only a record of its own kind, and only a Revocation withdraws a record — source: `internal/memory/record/record.go`
- **F-8**: Requirement IDs are stable: never renumbered or reused; a change supersedes or appends — source: `AGENTS.md`, the document system, rule 4
- **F-9**: The document gate refuses a solution shape whose paired design is absent (`unpaired-shape`) — source: `internal/docgate/shapes/shapes.go`
- **F-10**: I9: a requirement retired by a later change left a frozen, fully delivered plan with unresolvable traces — source: `docs/observability/incidents.md` I9
- **F-11**: A draft-born plan names the lock it was generated from, and its traces resolve against that locked version's records — source: `design:feature-draft-v1` §3.11; solution shape S-10

## Fog

none

## Out of scope

- **O-1**: The approver-identity guard on `pyxis draft open`, the required readiness check passing on proposals that are not drafts, and counting a merged plan's unprojected tasks as unfinished — each implements a statement `feature-draft-v1` already makes (S-6; `prd:feature-draft/FR-43`) and rides the implementation plan's acceptance criteria (from D-1)
- **O-2**: Where a draft-born feature's concrete layouts live — rides the plan's stage-skill task; the document gate already refuses a separate shape without a paired design (F-9) (from D-1)
- **O-3**: Superseding `feature-draft-v1` or `contract-gate-v2` (from D-1)

## Candidate glossary terms

none

## Parked product gaps

none

## Gate

| Field | Value |
|---|---|
| **Verdict** | ready-for-design |
| **Date** | 2026-09-24 |
| **Authority** | @mdarabi |
| **Covering design** | — |
| **Coverage check** | pass — core four covered, zero blocking fog |
| **Amend route** | — |
