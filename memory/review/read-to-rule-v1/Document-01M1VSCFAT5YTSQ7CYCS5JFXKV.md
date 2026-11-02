# Review: read-to-rule-v1 — design-review report

| Field | Value |
|---|---|
| **In one line** | Approve-with-advisories after two rounds: round 1 bounced the draft (9 blocking + 3 advisory across all four lenses — the §0 blast-tier ruling refused by docgate's computed tier, a capture-line form that could not carry its own prose's data, two Serves-claimed fallbacks stated nowhere, and an unhomed convention text); all twelve applied; round 2 confirmed closure with zero regressions, re-verifying the fixes against the fold's source. |
| **Relevant-when** | auditing or consuming the review of the read-to-rule-v1 design; asking why it was bounced; tuning tech-design-review calibration against real findings |
| **Subject** | design:read-to-rule-v1 · shape: none · supersedes: — |
| **Moment** | new-design |
| **Reviewer** | tech-design-review · guide as merged 2026-07-10 · calibration tech-design-review-calibration/v1 |
| **Date(s)** | 2026-07-14 |
| **Verdict** | approve-with-advisories |
| **Tracking** | the subject's introducing PR (this PR) |
| **Template version** | design-review-report/v1 |

*Advisory period: this verdict blocks nothing — the reviewer is pre-adoption; findings and verdict accumulate as measurement, and the operator approval + merge remain the stage-5 gate.*

## Round 1

### FND-1 — the §0 blast-tier ruling is refused by docgate's computed tier

- **Severity**: blocking (CAL-4 — load-bearing for acceptance: A-3/A-5 unsatisfiable as specified)
- **Lens**: feasibility-completeness (converged: upstream-alignment, coherence)
- **Evidence**: design: "A §0-only amendment declares blast radius `additive` … a §0 update riding a substantive amendment simply travels under that amendment's tier" (§3.6) — vs — the fold computes any section supersede as `contract-changing` and refuses declared-understates-computed on every fold (`internal/docgate/fold` tier computation + envelope validation; amendment model §7: declared may exceed, never understate).
- **Acceptance condition**: §3.6 declares the computed tier (contract-changing, informative scope in the reason), corrects the ride-along claim, accepts the review-bar consequence, names the carve-out as a future docgate-channel option; A-3/A-5 carry it.
- **Disposition**: applied.

### FND-2 — the pinned capture-line form cannot carry the sampled full-read time its prose mandates

- **Severity**: blocking (CAL-3 — a binding Decision form untestable as pinned)
- **Lens**: contract-quality (converged: coherence, feasibility, upstream-alignment)
- **Evidence**: design: "a sampled gate's full-read time rides the same line under its marker" — vs — the pinned marker `[· sampled: full read]` carrying no time slot; the block also spanned two lines against "one line per review event" in a gate-table cell, and M2's arithmetic differed by reading.
- **Acceptance condition**: the form gains `sampled: full read <M>m`; `<N>m` pinned as M2's only input; single-physical-line stated.
- **Disposition**: applied.

### FND-3 — the form's aggregate counts cannot populate the basis's per-item record

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: design: "`<a>` in-§0 / `<b>` outside-§0" + one unpartitioned citation list — vs — prd:read-to-rule §5.4 (items[{gist, routing}]), EVT-1/EVT-2: uncited items had no gist anywhere, citations no routing, sampled extras indistinguishable.
- **Acceptance condition**: per-item routing markers with citation-or-gist per item; sampled extras as `[out]` items; §5.4/EVT-1/EVT-2 reconstructable from the line alone.
- **Disposition**: applied.

### FND-4 — A-2 verifies a prd-new mechanic stated nowhere in §3

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: design: A-2's synthesis criterion — vs — no §3 subsection stating the synthesis mechanic; the header Scope's positive list omitting the prd-new/prd-amend steps Relevant-when claims.
- **Acceptance condition**: §3 states the synthesis mechanic (moment, source, DINV-1 anchoring); Scope carries the steps.
- **Disposition**: applied — §3.6 retitled "Authoring and amendment mechanics", opening with the synthesis Decision.

### FND-5 — the convention text has no named home, breaking the basis's NFR-2

- **Severity**: blocking (CAL-4 — NFR-2 is Serves-claimed and DoD-sourced)
- **Lens**: feasibility-completeness (converged: contract-quality)
- **Evidence**: design: A-4 "stated at the sign-off surfaces" naming no artifact; A-1's template enumeration omitting the capture line — vs — prd:read-to-rule NFR-2 (the format lives in template guidance, tunable by a template PR alone).
- **Acceptance condition**: the home named as prd-template's gate-table guidance; A-1/A-4 updated.
- **Disposition**: applied.

### FND-6 — Serves claims FR-9 and UB-2 but neither fallback is stated

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: design: Serves row + §3.7 containing neither fallback — vs — prd:read-to-rule DOD-3 requiring the stated convention to include the pre-§0 fallback and the stale-route fallback.
- **Acceptance condition**: §3.7 states both, covered by A-4.
- **Disposition**: applied.

### FND-7 — "a staged problem-only gate owes §0 nothing" narrows the basis's counting unit

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: design: the owes-nothing sentence — vs — the basis's pinned "§0 gate" definition (any review event signing a gate table on a §0-bearing document) and FR-7's "zero silent lineless gates" fit: the instruction produces silent lineless window members.
- **Acceptance condition**: restate — such a review is a §0 gate (line or recorded lineless miss); problem-space rows taken, solution-space rows deferred; no narrowing.
- **Disposition**: applied.

### FND-8 — the design mis-states the live enforcement landscape

- **Severity**: advisory (CAL-4)
- **Lens**: feasibility-completeness
- **Evidence**: design: "no lint enforces its presence or format" + R1 framing the resolution check as future — vs — the counterpart's live repo-wide `review-feedback:` resolution check already gate-enforcing citations in tracked files (sequencing consequence: cite only committed entries; the real residual: PR-review-side lines).
- **Acceptance condition**: the AP-10 deviation and R1 acknowledge the live check, its reach and asymmetry; §3.7 carries the sequencing note.
- **Disposition**: applied.

### FND-9 — the deferred introduction path is pre-decided inside a binding callout with falsified sizing

- **Severity**: advisory (CAL-4)
- **Lens**: feasibility-completeness (converged: contract-quality, upstream-alignment)
- **Evidence**: design: the callout binding the `add` op shape and "one-line docgate follow-up" — vs — the implemented `add` accepting table kinds only and the model's append-only next-fresh section anchors: the real path is a grammar-and-fold evolution; §3.2 assigns it to a future owner.
- **Acceptance condition**: the callout binds only synthesis + maintenance + the deferral; the sketch moves to non-binding prose naming the true class of the change.
- **Disposition**: applied.

### FND-10 — the cap's unit is not deterministically countable

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: design: "at most 40 rendered lines" — vs — rendering being width-dependent with no stable line count; conformance verdicts varying by inspector.
- **Acceptance condition**: a deterministic counting surface — markdown source lines of the section in `effective.md`.
- **Disposition**: applied.

### FND-11 — the ranked-call bound is stated twice, inconsistently, with the 1–2-call case undefined

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: design: "a table of 3–7 rows" (block description) vs "ranked calls at most 7" (the cap) vs honest-zero legitimizing 0 — a 1–2-call table conformant under two rules and nonconformant under the third.
- **Acceptance condition**: one binding bound (≤ 7; zero legal via honest-zero) with 3–7 as non-binding typical range.
- **Disposition**: applied.

### FND-12 — the pinned citation writes `#<anchor>` where the counterpart's grammar pins `#<entry-anchor>`

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: design: the format block's `#<anchor>` — vs — review-feedback-capture §3.5 (`#<entry-anchor>`; its schema carries a distinct `subject_anchor`) and the subject's own DINV-3: a wrong-referent citation evades the live check's entry grammar silently.
- **Acceptance condition**: the format block matches the counterpart's grammar verbatim.
- **Disposition**: applied.

## Round 2

*Scope-locked to the twelve named defects plus regressions within changed spans. All twelve closures confirmed; the fixes' factual claims re-verified against the implementation (the fold's section-supersede tier computation, the `add` op's accepted kinds, the counterpart's citation grammar); the residual-phrase sweep found no "additive"/"rendered lines"/"owes §0 nothing" residue in binding positions. Zero regressions.*

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-07-14 |
| 2 | approve-with-advisories | 2026-07-14 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 9 blocking · 3 advisory
- **Dispositions**: 12 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

none
