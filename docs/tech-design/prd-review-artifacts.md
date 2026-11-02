# prd-review artifacts — the report, brief, and calibration contract — technical design

| Field | Value |
|---|---|
| **Status** | Approved — operator sign-off 2026-07-10 (operator review session) *(merge is the freezing event)* |
| **Owner** | @mdarabi |
| **Approver(s)** | @mdarabi (operator) |
| **Date** | 2026-07-10 |
| **Relevant-when** | implementing or changing the prd-review skill's artifacts — the review report (findings, dispositions, verdict history, embedded brief, measurements), its home and lifecycle, or the calibration table; consuming a report as an evidence package; superseding the review-report schema |
| **Scope** | The prd-review artifact contract: the `review-report/v1` schema and its anchors, the one-report-per-subject-per-moment lifecycle and `docs/reviews/` home, the embedded escalation brief, and the calibration-table format and home. **Not**: the reviewer's behavior (the PRD binds it), the tech-design-review class's schemas (its own design, per the session's two-designs ruling), fold/projection tooling, the JSON-native graduation (parked). |
| **Tracking** | filed with the introducing PR |
| **Builds-on** | brainstorm-record-model |
| **Supersedes** | — |
| **Serves** | prd:prd-review/FR-5 · prd:prd-review/FR-6 · prd:prd-review/FR-7 · prd:prd-review/FR-11 · prd:prd-review/FR-14 |
| **Security/privacy impact** | none — repo-local markdown artifacts; no identity, secret, network, or egress surface |
| **Template version** | tech-design-template/v1 |

## 1. Problem

The prd-review PRD binds *what* its artifacts must carry — closable findings with calibration-row citations (FR-5/6), a report riding the subject's PR as its evidence package with append-only verdict history (FR-7), an embedded-or-emitted decision brief (FR-11), and measurement fields down to per-finding dispositions and post-approve overlay corrections (FR-14) — and its Q1 deferred the schemas, formats, and homes to this design. Implementation cannot start without them, and the direction was just grilled and gated in a real stage-3 session (the record this design is fed from), whose panel weighed the source-format seam explicitly.

## 2. Decision

**Decision:** The prd-review artifacts are **typed markdown in the record-class pattern** (design-brainstorm:reviewer-artifacts-tech-design#D-1): one **review report** per subject per moment at **`docs/reviews/<feature>-<NNN>.md`** — rounds, per-finding dispositions, and the verdict history appending *inside* the one artifact, the escalation brief **embedded** as its terminal section, the report riding the subject's introducing PR — and a **calibration table** as versioned, reviewable markdown data at the prd-review skill's own home on the vendor-neutral agent surface — `calibration.md` beside the skill's glue (design-brainstorm:reviewer-artifacts-tech-design#D-2). The schema ships as a versioned template (`review-report/v1`) with a structure checklist, mirroring the proven record classes; mechanization to JSON-native is deliberately parked for the engine era, when a mechanical consumer of findings and measurements exists.

The reasoning, stated once: the bootstrap's human surface is *now* — the authority reads these reports in PRs during the overlay era — and typed markdown is readable in a diff where JSON is hostile; the pattern (template + checklist + staged mechanization) shipped twice this week and survived real sessions; and one-artifact-per-review keeps FR-7's append-only history a document lifecycle instead of a file sprawl.

## 3. The design

### 3.1 The `review-report/v1` schema

The template lands in the templates directory (config `paths.templates_dir`), version-pinned `review-report/v1`; additive evolution within v1, breaking change = v2 by a superseding design.

**Header (closed):** `In one line` · `Relevant-when` — the routing pair — then **`Subject`** (the reviewed artifact, typed: `prd:<feature>/000` or `prd:<feature>/<NNN>` for an amendment) · **`Moment`** (`new-base` \| `amendment`) · **`Reviewer`** (`prd-review` + guide version) · `Date(s)` · **`Verdict`** (the *live* verdict — history below is authoritative) · `Tracking` · `Template version`.

**Sections and entry contracts** (every section present; empty reads `none`):

| Section | Anchor | Entry fields |
|---|---|---|
| `## Round N` *(one per round, max two — the cap is structural)* | `FND-n` | one block per finding: severity (+ the calibration row cited) · lens · evidence (subject quote + upstream anchor) · acceptance condition · **disposition** (`applied` \| `declined — <reason>` \| `violation-withheld`) — five elements mandatory, the PRD's FR-5 made structure |
| `## Verdict history` | — | append-only table: round · verdict · date — exactly one live verdict (the header mirrors the last row) |
| `## Escalation brief` | — | present iff a verdict is `escalate`: the single conflict (citing `FND-n`) · options with risks · evidence refs · the one decision needed — the payload the surrounding flow mints from |
| `## Measurements` | — | round count · findings by severity · dispositions summary — the M1 numerators/denominators, computable in place |
| `## Overlay corrections` | — | post-approve entries only: correction · found-by · date — the M2 residual data, appending without touching frozen history |

`FND-n` anchors are per-report, dense, never reused across rounds (round 2 findings continue the sequence); cited externally as `review:<feature>-<NNN>#FND-3`.

**The structure checklist** rides the template as its appendix (walked at verdict time and by PR review; the enforcement surface until mechanization): header complete; one `Round` section per recorded round, ≤ 2; every `FND-n` five-elemented with a calibration-row citation; dispositions present for every blocking finding before a round-2 or terminal verdict; verdict history append-only with one live row; the brief present iff escalate; measurements consistent with the finding blocks; overlay corrections only after an approve.

### 3.2 Home, naming, and lifecycle

Reports live flat in **`docs/reviews/`**, named by the subject's identity: `<feature>-000.md` for a base review, `<feature>-<NNN>.md` for an amendment review — unique by ledger construction, no collision rule needed. One report per subject per moment, forever: a re-review is a new `## Round 2` inside the same artifact, never a new file. The report is created by the review, travels in the **same introducing PR as its subject**, and freezes with it at merge — post-merge, the only legal append is an `## Overlay corrections` entry (the M2 datum), attributed and dated, which the next PR touching that feature carries. An `escalate` verdict leaves the report (and its PR) parked open — the report *is* the brief's home; the surrounding flow mints the work item from it per the escalation convention.

### 3.3 The calibration table

`calibration.md` at the prd-review skill's home (the vendor-neutral skills surface; the exact footprint pointer is the root manual's, per repo-layout A-2) — versioned (`prd-review-calibration/v1` header row), a closed markdown table: `| Row | Lens | Default severity | Escalation condition |`. Findings cite rows by id (`CAL-3`); a finding whose severity contradicts its cited row is a structure-checklist red. Changes are ordinary data PRs — the PRD's tunable-dial contract (FR-6) with the defanging guard living in review (its ABUSE-2).

### 3.4 Observability

none — the artifacts have no runtime surface; their inspection surfaces are themselves: the measurements section (M1 in place), the overlay-corrections section (M2 in place), and the verdict history (AP-13's verified-artifact posture for review completion).

### 3.5 Security & privacy

none — matches the header row: repo-local markdown; the PRD's abuse cases (planted content, table defanging, post-verdict laundering) are behavioral and data-review concerns bound there; this schema contributes the append-only and row-citation structure they lean on.

### 3.6 Boundaries and non-goals

- **Not solved here** — the reviewer's behavior (lenses, survival, loop mechanics — the PRD binds them; the guide implements them); the sibling class's schemas (its own design, per design-brainstorm:reviewer-artifacts-tech-design#D-3).
- **Owned by a future design** — the JSON-native graduation when a mechanical consumer exists (the record's G-1); checklist mechanization (with the record-lint graduation batch).
- **Implementation must not infer** — that reports are governed documents (they are provenance-class, like records); that a report may be regenerated or rewritten (append-only past each verdict); that the brief ever lives as a sibling file.
- **Must remain unchanged** — docgate's verbs and gate output on trees containing reports; the subjects' bytes; the record classes and their templates.

### 3.7 Invariants

- **DINV-1** — one report per subject per moment; rounds append inside it; a second file for the same subject+moment is invalid.
- **DINV-2** — the verdict history is append-only with exactly one live verdict; a rewritten history row is invalid.
- **DINV-3** — every `FND-n` carries all five elements including its calibration-row citation; severity contradicting the cited row is invalid.
- **DINV-4** — the escalation brief exists iff a verdict is `escalate`, embedded in the report, citing its findings.
- **DINV-5** — post-approve appends are overlay corrections only, attributed and dated; frozen rounds and history are never edited.
- **DINV-6** — the report travels in the same introducing PR as its subject.

## 4. Alternatives considered

*The source-format seam was paneled in the feeding session; the ledger carries verbatim, verdicts included.*

| Alternative | Why rejected |
|---|---|
| **JSON-first, the defects-as-data pattern** (design-brainstorm:reviewer-artifacts-tech-design#ALT-2) | the bootstrap's human surface pays: raw JSON in PR diffs during the exact era the authority reviews PRs; rejected **for now** — it is the engine-era graduation shape, parked as this design's Q1 |
| **Two-faces projection — JSON source + committed markdown face** (…#ALT-3) | the fold tool must precede the artifact (born-with-the-tool), front-loading engine tooling into a bootstrap skill |
| **Brief as a sibling file** (…#D-2, considered and refused in-session) | splits the evidence package; the brief is the report's terminal payload and the minting flow copies from it |
| **One report file per round** | shatters FR-7's append-only verdict history across files; the round cap is structural inside one artifact |

## 5. Risks

| ID | Risk | Impact | Mitigation | Verification / evidence |
|---|---|---|---|---|
| R1 | parse-by-convention until mechanization — a malformed report goes unnoticed | measurement fields silently wrong | the structure checklist walked at verdict time + PR review; defect fixtures make each invalid class red; mechanization queued with the record-lint batch (Q2) | A-2's defect fixtures |
| R2 | multi-round reports sprawl past the PRD's compactness bound | unreadable evidence packages | the PRD's NFR-2 bound (findings ≤ 10 lines, reports ≤ 200) enforced by the checklist; the two-round cap bounds totals structurally | fixture line counts |
| R3 | post-merge overlay corrections tempt edits to frozen content | laundered history (the PRD's ABUSE-3) | DINV-5 confines appends to the corrections section; the carrying PR shows the diff | checklist row + PR review |

## 6. Acceptance

| ID | Acceptance criterion | Verify | Required evidence | Traces to |
|---|---|---|---|---|
| A-1 | the `review-report/v1` template exists in the templates directory: closed header, the five sections with entry contracts, the structure checklist appendix | Inspection | template file in the implementation PR | prd:prd-review/FR-5 · FR-7 · §3.1 |
| A-2 | a golden report instantiated from the template passes the checklist, and each seeded defect class (missing calibration citation · missing acceptance condition · second live verdict · rewritten history row · brief-without-escalate · third round) turns it red at exactly its row | Test | golden + defect fixtures with results | #DINV-1..5 · §3.1 |
| A-3 | the calibration table exists at the skill home, versioned, closed-schema; a data-only change alters finding severity with no guide edit | Test | the table + a severity-flip fixture | prd:prd-review/FR-6 · §3.3 |
| A-4 | the naming/lifecycle rules are implemented in the reviewer's guide: subject-identity naming, same-PR travel, round-append, post-approve corrections | Inspection | guide text | prd:prd-review/FR-7 · FR-14 · §3.2 |
| A-5 | this document passes `docgate lint-tech-designs` and ships with the regenerated index | Test | gate output in the introducing PR | conventions §3.2 |

## 7. Open questions

| ID | Question | Owner | Blocks | Status | Resolution |
|---|---|---|---|---|---|
| Q1 | The JSON-native graduation — trigger and shape when a mechanical consumer of findings/measurements exists (the record's G-1) | operator | nothing | open | |
| Q2 | Whether report-checklist mechanization joins the record-lint graduation batch (one docgate amendment for all provenance classes) or ships separately | operator | nothing | open | |

## Principles

- **AP-6 — honored, staged.** Typed sections with dense anchors and closed entry contracts are contractual-lite now; the JSON graduation is parked with a named trigger, not skipped.
- **AP-10 — honored, staged.** The checklist is the enforcement surface until mechanization — the record classes' exact posture, third application.
- **AP-11 — honored.** One calibration table per class, cited by row from every finding — the dial has one home and one grammar.
- **AP-13 — honored.** Review completion is a verifiable artifact: the append-only verdict history plus in-place measurements, never a claim.
- **AP-21 — honored.** Dispositions and verdicts are frozen data downstream consumers read verbatim; nothing re-judges a recorded round.
- **AP-23 — honored.** One flat file per subject+moment, unique by ledger construction; parallel reviews never contend.
- **RW-5 — honored.** One decision — the prd-review artifact contract; the sibling's schemas and the reviewer's behavior live elsewhere (§3.6).
- **Deviation:** none.
