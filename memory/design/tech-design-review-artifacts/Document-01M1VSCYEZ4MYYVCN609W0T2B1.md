# tech-design-review artifacts — the report, brief, and calibration contract — technical design

| Field | Value |
|---|---|
| **Status** | Approved — operator sign-off 2026-07-10 (doc-review session) *(merge is the freezing event)* |
| **Owner** | @mdarabi |
| **Approver(s)** | @mdarabi (operator) |
| **Date** | 2026-07-10 |
| **Relevant-when** | implementing or changing the tech-design-review skill's artifacts — the review report for design+shape subjects and supersessions, its home and lifecycle, or this class's calibration table; consuming a design-review report as an evidence package; superseding the design-review-report schema |
| **Scope** | The tech-design-review artifact contract: the `design-review-report/v1` schema and its anchors (incl. the supersession-integrity and pair-subject fields), the one-report-per-subject lifecycle and naming in the shared `docs/reviews/` home, the embedded brief, and this class's calibration-table format and home. **Not**: the reviewer's behavior (its PRD binds it), the prd-review class's schemas (its own frozen design), fold/projection tooling, the JSON-native graduation (parked with the shared trigger). |
| **Tracking** | filed with the introducing PR — authored recordless per the reviewer-artifacts session's two-designs ruling (design-brainstorm:reviewer-artifacts-tech-design#D-3): the direction was settled there; this document assembles its class binding |
| **Builds-on** | prd-review-artifacts |
| **Supersedes** | — |
| **Serves** | prd:tech-design-review/FR-5 · prd:tech-design-review/FR-6 · prd:tech-design-review/FR-7 · prd:tech-design-review/FR-11 · prd:tech-design-review/FR-14 |
| **Security/privacy impact** | none — repo-local markdown artifacts; no identity, secret, network, or egress surface |
| **Template version** | tech-design-template/v1 |

## 1. Problem

The tech-design-review PRD binds the same artifact obligations as its sibling — closable findings with calibration-row citations, a report riding the subject's PR with append-only verdict history, the brief, the measurement fields — and its Q1 deferred schemas and homes, leaning on the sibling's artifact design. The direction is already ruled: the reviewer-artifacts session paneled the source-format seam and the authority chose typed markdown, the shared `docs/reviews/` home, embedded briefs, skill-home calibration — and **two designs**, one per reviewer class (design-brainstorm:reviewer-artifacts-tech-design#D-1 · #D-2 · #D-3). What remains is this class's binding: subjects that are design+shape pairs or supersessions, the finding fields those subjects add, and the naming that keeps the shared home collision-free.

## 2. Decision

**Decision:** The tech-design-review artifacts follow the ruled direction — **typed markdown in the record-class pattern** (design-brainstorm:reviewer-artifacts-tech-design#D-1), instantiated for the design class as **`design-review-report/v1`**: one report per subject at **`docs/reviews/<design-slug>.md`** — the subject being the design+shape pair (or a superseding design with its targets), rounds and verdict history appending inside, the escalation brief embedded, the report riding the design's introducing PR — with this class's **own calibration table** (`tech-design-review-calibration/v1`) as versioned data at the tech-design-review skill's home (design-brainstorm:reviewer-artifacts-tech-design#D-2). The schema extends the sibling's shape with two class-specific structures: **pair-subject evidence** (findings may quote the design or its shape, the header records which halves the subject carried) and **supersession-integrity findings** (a finding class citing the successor's and targets' anchors, never disputing the targets' frozen content). JSON-native mechanization stays parked with the shared trigger.

The reasoning: the direction questions were grilled once at the right altitude and frozen; re-deciding them here would be the re-judging this whole program exists to prevent. The class binding is assembly — and the `<design-slug>.md` naming is collision-free against the sibling's `<feature>-<NNN>.md` by construction (design slugs never carry the dense-number suffix).

## 3. The design

### 3.1 The `design-review-report/v1` schema

The template lands in the templates directory (config `paths.templates_dir`), version-pinned; additive within v1, breaking change = v2 by supersession.

**Header (closed):** the routing pair — then **`Subject`** (typed: `design:<slug>`, with `shape: present | none` recording whether the pair's concrete half existed, and `supersedes: <target slugs> | —` for supersession subjects) · **`Moment`** (`new-design` \| `supersession`) · **`Reviewer`** (`tech-design-review` + guide version) · `Date(s)` · **`Verdict`** (live; history authoritative) · `Tracking` · `Template version`.

**Sections** mirror the sibling — `## Round N` (`FND-n`, ≤ 2 rounds structural) · `## Verdict history` · `## Escalation brief` (iff escalate) · `## Measurements` · `## Overlay corrections` — with the finding block extended for this class: evidence quotes name their half (`design:` or `shape:` span) so partition-honesty findings are legible, and a supersession-integrity finding carries **target anchors** (`<target-slug>#A-3`, `#DINV-1`) identifying what was stranded, dropped, or falsely contradicted — the targets' frozen content is evidence, never the finding's object (the PRD's NG-2 made schema). `FND-n` dense across rounds; cited externally as `review:<design-slug>#FND-2`.

**The structure checklist** rides the template: the sibling's rows plus — the Subject records shape presence and supersession targets; every supersession-integrity finding cites ≥ 1 target anchor; every evidence quote names its half on pair subjects.

### 3.2 Home, naming, and lifecycle

The shared **`docs/reviews/`** home (design-brainstorm:reviewer-artifacts-tech-design#D-2): design reviews named **`<design-slug>.md`** — unique because design slugs are globally unique in the class and never end in a dense-number suffix, so the two report families coexist flat and unambiguously. One report per subject: a supersession review lives in the *successor's* report (its authoring is the moment); a re-review is `## Round 2` in place. Same-PR travel, freeze-at-merge, post-approve overlay corrections as the only legal append, escalate-parks-open — all as ruled and as bound in the sibling design's §3.2, mirrored not re-decided.

### 3.3 The calibration table

`calibration.md` at the tech-design-review skill's home (the vendor-neutral skills surface; the footprint pointer is the root manual's) — `tech-design-review-calibration/v1`, the closed `| Row | Lens | Default severity | Escalation condition |` schema, seeded from its PRD's D-2 defaults (coherence and alignment blocking; contract-quality advisory unless a binding element goes untestable; feasibility/completeness blocking only when acceptance-load-bearing). Findings cite rows (`CAL-n`); contradiction is a checklist red; changes are data PRs. Deliberately its own table — the per-class dial both PRDs bound.

### 3.4 Observability

none — the artifacts have no runtime surface; measurements and verdict history are the in-place inspection surfaces, as in the sibling design.

### 3.5 Security & privacy

none — matches the header row; the PRD's abuse cases lean on the same append-only and row-citation structure this schema provides.

### 3.6 Boundaries and non-goals

- **Not solved here** — the reviewer's behavior (lenses, supersession checks — the PRD binds them); the prd-review class's schemas (frozen sibling design).
- **Owned by a future design** — the JSON-native graduation (shared trigger, sibling Q1); checklist mechanization (the provenance-class batch, sibling Q2).
- **Implementation must not infer** — that a supersession review may target the predecessor's content (targets are evidence — NG-2); that the two report families share a calibration table; that a shape-less subject blocks review (the pair degrades per the PRD).
- **Must remain unchanged** — the sibling's frozen artifact design; the subjects' bytes; docgate's verbs.

### 3.7 Invariants

- **DINV-1** — one report per design subject; supersession reviews live in the successor's report; rounds append, never fork files.
- **DINV-2** — verdict history append-only, one live verdict (mirrors the sibling's DINV-2).
- **DINV-3** — every `FND-n` five-elemented with a calibration-row citation; on pair subjects, evidence names its half.
- **DINV-4** — a supersession-integrity finding cites ≥ 1 target anchor and never disputes target content.
- **DINV-5** — post-approve appends are overlay corrections only; frozen rounds and history are never edited.
- **DINV-6** — the report travels in the same introducing PR as its subject design.

## 4. Alternatives considered

*The direction alternatives were paneled and verdicted in the feeding session — carried by citation, not re-argued: typed markdown chosen over JSON-first and the two-faces projection (design-brainstorm:reviewer-artifacts-tech-design#ALT-1..#ALT-3). Class-local alternatives:*

| Alternative | Why rejected |
|---|---|
| **Reuse the sibling's `review-report/v1` with a subject-type switch** | conditional fields for pair halves and supersession targets would make one checklist lie for both classes — the same conditional-section smell both record classes refused; two thin templates beat one forked one |
| **A separate report per shape** (design and shape reviewed as two artifacts) | the subject is the pair by the PRD's D-1; splitting re-creates the partition blind spot the co-review exists to close |
| **Supersession reviews as standalone files** (`<successor>-supersedes-<target>.md`) | the successor's authoring is the moment and its report the home (one report per subject); composite filenames re-introduce the collision surface the slug rule avoids |
| **A shared calibration table for both reviewers** | both PRDs bound per-class dials; a shared table couples exactly the evolution the two-PRD and two-design rulings decoupled |

## 5. Risks

| ID | Risk | Impact | Mitigation | Verification / evidence |
|---|---|---|---|---|
| R1 | schema drift between the two report families (the accepted cost of two designs) | consumers juggle dialects | mirrored-by-pattern with the shared direction frozen in the record; the engine-era graduation (shared trigger) is the named unification point if a uniform consumer emerges | the sibling PRDs' WR-3 watches this |
| R2 | supersession findings drift into target-content disputes despite the schema | relitigation by evidence-quote | DINV-4 requires target anchors and forbids content disputes; the reviewer PRD's EVAL-2 family holds it at sev-1 | defect fixture: a target-content dispute turns the checklist red |
| R3 | parse-by-convention until mechanization | silently malformed reports | the checklist + defect fixtures, as in the sibling; mechanization queued with the provenance-class batch | A-2 fixtures |

## 6. Acceptance

| ID | Acceptance criterion | Verify | Required evidence | Traces to |
|---|---|---|---|---|
| A-1 | the `design-review-report/v1` template exists: closed header (Subject with shape/supersedes fields, Moment), the five sections, the extended finding block, the checklist appendix | Inspection | template file in the implementation PR | prd:tech-design-review/FR-5 · FR-7 · §3.1 |
| A-2 | a golden report passes the checklist; each seeded defect class (the sibling's six + a half-less evidence quote on a pair subject + a supersession finding without target anchors + a target-content dispute) turns it red at its row | Test | golden + defect fixtures | #DINV-3 · #DINV-4 · §3.1 |
| A-3 | this class's calibration table exists at the skill home, versioned, seeded from the PRD's defaults; a data-only change flips a severity with no guide edit | Test | the table + a flip fixture | prd:tech-design-review/FR-6 · §3.3 |
| A-4 | the naming/lifecycle rules are implemented in the reviewer's guide: `<design-slug>.md`, supersession-in-successor, same-PR travel, round-append, overlay corrections | Inspection | guide text | prd:tech-design-review/FR-7 · FR-14 · §3.2 |
| A-5 | this document passes `docgate lint-tech-designs` and ships with the regenerated index | Test | gate output | conventions §3.2 |

## 7. Open questions

| ID | Question | Owner | Blocks | Status | Resolution |
|---|---|---|---|---|---|
| Q1 | The JSON-native graduation and checklist mechanization — shared with the sibling's Q1/Q2 (one batch for the provenance classes) | operator | nothing | open | |

## Principles

- **AP-6 — honored, staged.** Typed sections, closed header, dense anchors; the machine-native graduation parked with its named shared trigger.
- **AP-10 — honored, staged.** Checklist-enforced until the provenance-class mechanization batch.
- **AP-11 — honored.** One calibration table per class, cited by row — and the *direction* decided once in the feeding record, cited here rather than re-derived.
- **AP-13 — honored.** Verdict history + in-place measurements make review completion a verifiable artifact.
- **AP-21 — honored.** The record's rulings are frozen judgments this design reads verbatim — authored recordless *because* the judging already happened at the richest-context site.
- **AP-23 — honored.** Flat shared home, structurally collision-free naming; parallel reviews never contend.
- **RW-5 — honored.** One decision — this class's artifact binding; the direction, the sibling's schemas, and the reviewer's behavior each live in their own documents.
- **Deviation:** none.
