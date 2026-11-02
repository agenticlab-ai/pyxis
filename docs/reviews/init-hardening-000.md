# Review: prd:init-hardening/000 — review report

| Field | Value |
|---|---|
| **In one line** | Approve after one revision round — five blocking findings (a silent conflict with the installer's no-sixth-question fit, an under-specified rollback hazard, unranked priorities, two basis-fidelity gaps) all applied; three advisories applied. |
| **Relevant-when** | auditing or consuming the review of the init-hardening 000-base; asking why it was approved; tuning prd-review calibration against real findings |
| **Subject** | `prd:init-hardening/000` |
| **Moment** | new-base |
| **Reviewer** | prd-review · `.pyxis/authoring/prd-review.md` (run inline by the authoring session, bootstrap-era) |
| **Date(s)** | 2026-07-28 |
| **Verdict** | approve |
| **Tracking** | — *(the subject's introducing PR, once filed)* |
| **Template version** | review-report/v1 |

## Round 1

### FND-1 — Silent conflict with the installer's no-sixth-question fit

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: subject FR-6 — "where none exists interactively, the system shall elicit a shape-validated `owner/name`" — vs — `prd:installer` FR-3's fit criterion: "post-init config contains the five knob keys …; **no sixth question is asked**". The subject's §6.2 alignment-amendment row named preview/admission/`ci_mode` but not FR-3, leaving the conflict silent.
- **Acceptance condition**: the §6.2 dependency row names the installer's FR-3 fit among the rows the alignment amendments must touch.
- **Disposition**: applied — §6.2 row now reads "its five-knob/no-sixth-question fit (its FR-3), preview (its FR-19), admission (its UB-1), and `ci_mode` rows".

### FND-2 — Rollback semantics unstated for foreign files appearing mid-apply

- **Severity**: blocking (CAL-4 — load-bearing for §8's safety-plan row: a restore that deletes what init did not write is the feature's own defect class)
- **Lens**: completeness-critic
- **Evidence**: subject FR-10 — "restore the target to its pre-run state — removing every write it performed" — with no statement about entries that appeared during the run; a byte-identical-to-pre-run fit could pass by deleting a user's mid-run file.
- **Acceptance condition**: FR-10 states that restore removes exactly init's enumerated writes, preserves foreign entries, and names them; the fit criterion exercises a planted foreign file.
- **Disposition**: applied — FR-10 statement, rationale, and fit all amended.

### FND-3 — Priority classes unranked

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: subject §5.1 carries 14 Must rows with no intra-class ranking — vs — the template's §5.1 rule: "rank within each class, because cuts arrive mid-build and unranked lists cut themselves."
- **Acceptance condition**: an explicit ranking rule for the requirement tables.
- **Disposition**: applied — "Within each priority class, rows rank in table order (FR-1 highest); cuts consume a class from the bottom up."

### FND-4 — Parked constraints P-2/P-4 not conveyed to the design surface

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment (basis fidelity)
- **Evidence**: the record's P-2 (floor-probe pattern) and P-4 (`MarkerFiles` groundwork) appear nowhere in the subject — vs — the prd-new guide: "`P-n` constraints pass to the technical design via §6.2/§7.2".
- **Acceptance condition**: the subject routes every record P-row to the design.
- **Disposition**: applied — §7.2 closing note routes `#P-1`..`#P-7`, naming P-2/P-4 as questionless design inputs.

### FND-5 — EVT-3 feeds a non-metric

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: subject §5.7 EVT-3's Feeds cell read "M2, §3.3" — vs — the template's tracking-plan contract: "each event names the metric it feeds"; §3.3 is not a §3.2 metric.
- **Acceptance condition**: the Feeds cell resolves to metrics only.
- **Disposition**: applied — EVT-3 feeds M2.

### FND-6 — FR-3's toolchain set under-enumerated

- **Severity**: advisory (CAL-3 — the fit was weakly decidable, not undecidable)
- **Lens**: contract-quality
- **Evidence**: subject FR-3 — "the runtimes the install consumes" — an open set; the fit's "each seeded toolchain defect" presupposed an enumeration stated nowhere.
- **Acceptance condition**: the subject binds the set to the resolved knobs plus the release's documented floor set, and the fit exercises absence and below-floor per executable.
- **Disposition**: applied — FR-3 statement and fit rewritten.

### FND-7 — Closed-input (EOF) semantics unstated beside the new re-ask loop

- **Severity**: advisory (CAL-4)
- **Lens**: completeness-critic
- **Evidence**: subject FR-13 introduced bounded re-ask with no statement of the existing EOF-takes-default behavior; a re-ask loop naively reading a closed stream would spin or abort where today's contract defaults.
- **Acceptance condition**: FR-13's fit pins the EOF path.
- **Disposition**: applied — "a closed input stream takes the default without entering the re-ask loop (today's EOF semantics preserved)".

### FND-8 — Remote repository creation from one unechoed answer

- **Severity**: advisory (CAL-4)
- **Lens**: completeness-critic
- **Evidence**: subject FR-8 creates a real repository from a single interactive answer with the confirmation ceremony removed by design (`#D-7`); a typo'd owner lands a repo in the wrong place with no prior visibility.
- **Acceptance condition**: the pre-run check summary echoes the resolved coordinate before any creation — visibility without ceremony, consistent with `#D-7`.
- **Disposition**: applied — FR-8's fit criterion.

## Round 2

Scope-locked re-examination of the eight amended spans (§6.2 dependency row · FR-10 · the §5.1 ranking line · the §7.2 routing note · EVT-3 · FR-3 · FR-13 · FR-8) against their acceptance conditions, plus regression check within the changed spans. All eight conditions met; the refold is clean; no regressions introduced; no new findings within scope. Unchanged content not re-examined (protocol).

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-07-28 |
| 2 | approve | 2026-07-28 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 5 blocking · 3 advisory
- **Dispositions**: 8 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

none
