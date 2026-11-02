# e2e-review calibration — e2e-review-calibration/v1

The severity dial as data (`prd:e2e-verification/NFR-4`): adding or tuning a
lens touches no other lens, and tuning a lens touches no code. Findings cite
rows by id; a finding whose severity contradicts its cited row is a
structure-checklist red. Changes are ordinary data PRs.

**Every row carries exactly one severity, drawn from `blocking | advisory`, and
exactly one lens from the closed set `coverage | anti-gaming | buildability`.** That is not
presentation — it is the contract `internal/e2e/report` enforces: a finding's
severity must equal the severity its cited row maps to, so a row whose severity
is conditional ("advisory, becomes blocking when…") or absent is a severity no
finding could ever record. A failure class that is sometimes graver than its
sibling therefore gets its **own row** to cite (CAL-5, CAL-6), rather than a
promotion clause the grammar cannot express.

| Row | Lens | Default severity | Failure class |
|---|---|---|---|
| CAL-1 | coverage | blocking | a required behavior maps to no scenario at all (`prd:e2e-verification/ABUSE-2`) |
| CAL-2 | anti-gaming | blocking | a scenario's recorded mock strategy guts the very behavior its coverage-map entry claims to prove (`prd:e2e-verification/ABUSE-1`) |
| CAL-3 | coverage | advisory | a scenario is mapped to a behavior it does not plausibly exercise — over-mapping the coverage map's completeness claim does not rest on (`prd:e2e-verification/ABUSE-4`) |
| CAL-4 | anti-gaming | advisory | a scenario's route or mock choice is questionable but does not fully gut its mapped behavior (`prd:e2e-verification/ABUSE-3`) |
| CAL-5 | coverage | blocking | load-bearing over-mapping — the inflated scenario is what the coverage map's own completeness claim rests on, so the map reads complete only because of the inflation |
| CAL-6 | anti-gaming | blocking | the human review pass (`prd:e2e-verification/FR-9`) confirmed a scenario's route or mock does not genuinely exercise the behavior it is mapped to |
| CAL-7 | buildability | blocking | a harness slot's answer specifies no machine a worker could build from — a bare library or tool name, the slot's own question restated, mechanics deferred to a later decision, or an answer another slot's answer contradicts (`prd:e2e-verification/FR-19`, `design:e2e-verification-skills-v2#DINV-8`) |
| CAL-8 | buildability | blocking | a scenario is not buildable against the harness as specified — its fixture is obtainable by no means the document states, a seam it exercises is carried by no slot, or the artifact it claims to own exclusively is not real (`prd:e2e-verification/FR-21`) |
| CAL-9 | buildability | blocking | the decision-authority stand-in is specified without one of the five properties `FR-22` requires — cooperative-only, assertion-sighted, silently improvising, unscripted, or with no adversarial persona — so the stand-in as specified would manufacture a green (`prd:e2e-verification/ABUSE-5`) |
| CAL-10 | buildability | advisory | a harness or scenario answer is thinner than the work it implies — stated in outline where the rest of the section states mechanics — with no slot unanswered and no scenario's buildability actually at risk |

## Escalation is a verdict act, never a severity rewrite

A finding's severity **is** its cited row's severity — a lens picks the *row*,
not the severity. The conditions below move the round's **verdict**; they never
promote or demote a finding in place:

- A `blocking` finding rejected **without** a recorded reason is invalid (the
  report does not validate); rejected **with** a stated reason, the round
  escalates.
- The convergence round cap reached without every finding settled escalates,
  whatever the findings' severities are.

This mirrors the family's other machine-consumed dial
(`.pyxis/skills/impl-review/calibration.md`), where escalation is likewise a
verdict act and every row carries one flat severity.
