# Review: e2e-verification-skills — design-review report

| Field | Value |
|---|---|
| **In one line** | request-changes (round 1) on two blocking findings — e2e-plan's behavior on a no-automatable-surface (manual-only) feature is unspecified and as written yields the degenerate harness+scenario plan UB-6 forbids, and the governing-design rule refuses on "more than one live design" when the sanctioned artifacts+behavior split (this feature included) makes multiple live designs normal — plus one advisory (AP-28 improvement-fuel retention omitted). |
| **Relevant-when** | auditing or consuming the review of the `e2e-verification-skills` design; asking why it bounced; tuning tech-design-review calibration |
| **Subject** | design:e2e-verification-skills · shape: present · supersedes: — |
| **Moment** | new-design |
| **Reviewer** | tech-design-review · 2026-07-24 |
| **Date(s)** | 2026-07-24 |
| **Verdict** | approve-with-advisories *(round 2; history below is authoritative)* |
| **Tracking** | — *(the subject's introducing PR)* |
| **Template version** | design-review-report/v1 |

## Round 1

*Generated through four blind fresh-context lenses over the two paired designs and their shared solution-shape (`design-brainstorm:e2e-verification-tech-design`). The `lint-tech-designs` floor is excluded.*

### FND-1 — e2e-plan's behavior on a manual-only feature is unspecified and violates UB-6 as written

- **Severity**: blocking (CAL-2 — a served Must, UB-6, is not implemented for the e2e-plan half; the design contradicts it for the manual-only case)
- **Lens**: upstream-alignment
- **Evidence**: `design:` "e2e-plan decomposes an approved e2e design doc into a **harness task** plus **per-scenario tasks**" (§3.3) — no manual-only branch; A-5 traces "`FR-12 · FR-13 · FR-14 · UB-4 · #DINV-5 · #DINV-6`", **not UB-6**; #DINV-7 and §3.1 attribute the manual checklist to *e2e-design*, never to e2e-plan — vs — `prd:e2e-verification/UB-6`: "e2e-plan emits a manual checklist, **not a harness+scenario build plan** … zero degenerate harness/scenario tasks." A manual-only design fed to e2e-plan as specified produces exactly the harness-only degenerate plan the requirement forbids.
- **Acceptance condition**: specify e2e-plan's behavior on a manual-only / zero-scenario design in §3.3 — it emits a manual checklist (or an empty, reason-recorded result), producing no harness+scenario build plan — resolving which skill owns the checklist and how the reused plan schema represents (or is bypassed for) it; add/extend an acceptance row and a DINV covering the **e2e-plan** side of UB-6 (today only A-10's e2e-design side is covered), and add UB-6 to A-5's traces.

### FND-2 — the governing-design rule refuses on "more than one live design," but the split pattern makes that the normal case

- **Severity**: blocking (CAL-2 — PRD fidelity: the design reinterprets Q6's "live *set*" as single-design-or-refuse, and would refuse on the split-design dogfood feature)
- **Lens**: feasibility-completeness / upstream-alignment
- **Evidence**: `design:` "e2e-design reads the **live (non-superseded) technical design** … On ambiguity — **more than one live design**, or liveness indeterminable — it surfaces or refuses" (§3.1) and A-9 "given multiple or superseded designs it surfaces or refuses" — vs — `prd:e2e-verification/§7.2 Q6`: "pick the live (non-superseded) **set**, surface or refuse on ambiguity" (set, not singular), and the design index shows the sanctioned artifacts+behavior split routinely yields multiple live designs per feature (argus; this very feature: `e2e-verification-artifacts` + `e2e-verification-skills`). The dogfood is a real pyxis feature (M1), likely itself split-design — so this trips the acceptance run. (`shape:` `#D-7` carried the same "more than one live design → ambiguity" wording and needs the same review-anchored refinement.)
- **Acceptance condition**: restate selection as "read the live (non-superseded) **set** of designs serving the feature; ambiguity is a *superseded* design with no clear live successor, or liveness indeterminable — never merely `count(live) > 1`"; fix A-9 to expect the live-set read on a two-live-design feature and refusal only on the superseded/indeterminable case; make `design:e2e-verification-artifacts` §3.1's governing-design header field set-valued; refine record `#D-7` (review-anchored).

### FND-3 — AP-28 (improvement fuel retained in git) is silently omitted; the fuel-bearing events are framed as ephemeral slog

- **Severity**: advisory (CAL-2 — downgraded from default: partly honored — the review report is git-committed and DOD-3 requires a catch-corpus run record; the gap is the missing principle citation + unstated git-retention)
- **Lens**: upstream-alignment
- **Evidence**: `design:` §3.7 "`catch_corpus_run` …, `finding_disposition` …; **logged via `log/slog`**" and "M2/M3 … surfaced at the checkpoint" — with no statement that the catch-corpus run record and dispositions are retained as committed git event records; neither `## Principles` section cites AP-28 — vs — `AP-28`: "a failure disposition, a review verdict … kept as a committed artifact in the repository, not … a log that ages out … an immutable, append-only event record," whose named purpose is a defect/catch corpus accreting into the corpus the system improves from (the advisory→blocking ramp depends on exactly that accretion).
- **Acceptance condition**: cite AP-28 in the skills `## Principles`, and confirm in §3.7 that the catch-corpus run record and finding dispositions are retained as committed, append-only git event records (not only slog) — or deviate-with-reason.

## Round 2

*Terminal re-review, scope-locked to the round-1 findings and regressions in their changed spans. The `e2e-verification-skills` design was rewritten, and PRD amendment 001 landed the two contract changes the findings implicated.*

- **Blocking — both closed.** FND-1: the no-automatable-surface case is now handled by the amended `prd:e2e-verification/UB-7` — e2e-design produces zero e2e documents and signals the operator to stop the pipeline (§3.1, #DINV-7, A-10); e2e-plan is never reached, so the degenerate harness+scenario plan cannot arise. FND-2: e2e-design now reads the live design *set* and refuses only on a superseded-no-successor / indeterminable case (§3.1, #DINV-8, A-9); the record's D-7 is refined by D-14 and `design:e2e-verification-artifacts` §3.1's header field is set-valued.
- **Advisory — applied.** FND-3: AP-28 is cited in `## Principles`, and §3.7 states the review report and catch-corpus run record are retained as committed, append-only git records (not only slog).
- **Regressions in changed spans**: none. The `Serves` row now cites the amended `FR-16`/`FR-17`/`UB-7`; the harness task is conditional in §3.3/#DINV-5/A-5 consistently; the plan is an ordinary plan-new `plan/v1` matching `design:e2e-verification-artifacts` §3.2.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-07-24 |
| 2 | approve-with-advisories | 2026-07-24 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 2 blocking · 1 advisory
- **Dispositions**: 3 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

none
