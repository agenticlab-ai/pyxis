# Fixture: golden — the happy path

## Idea

"The factory should give me a weekly digest: what merged, what's blocked,
what escalated — so I don't have to trawl dashboards to know how my
installation is doing."

## Scripted operator

Persona: solo operator of one installation; decisive; answers in one or two
sentences; adopts good recommendations.

Answer bank:

- on framing choice → pick the *generated report artifact* framing, but keep
  the cadence configurable ("weekly by default, not weekly by definition").
- on primary segment → "solo or near-solo operators; explicitly not large
  teams with real dashboards and on-call rotations — that's the
  anti-segment."
- on the job → "Monday morning: know what shipped, what's stuck, what needs
  me — without opening anything else."
- on success / the bet → "if the digest is good, I stop polling surfaces;
  reading it *is* the weekly check."
- on delivery/transport (email, push, chat) → "out of scope for v1 — land it
  as an artifact I can open; transport later."
- on real-time alerting → "no — that's the health-alarm surface's job, not
  the digest's."
- on appetite → "small: a generator and a template; no new service, no new
  infra."
- on digest file format → "you decide."
- on section layout details → "park it — design-phase."
- default → adopt the recommendation.

## Expected record

- Gate verdict `ready-for-prd`; coverage check embedded and passing
  (six rows, zero blocking fog) — DINV-3/4.
- ≥ 6 live `D-n`, each with all five fields (DINV-1); the file-format ruling
  recorded as an explicit **delegation** ("operator delegated; recommendation
  adopted"), not as a self-answer.
- ≥ 2 framings + a Destination with operator rationale.
- ≥ 1 `F-n` citing the corpus for the existing health-alarm surface (the
  fact is looked up, never asked — EVAL-3).
- ≥ 2 `O-n` (transport; real-time alerting), each citing its `D-n`.
- 2 `G-n`, both `non-blocking` (section layout; landing surface).
- ≥ 1 `T-n` (e.g. digest vs report).
- Structure checklist: all rows pass. EVAL-1: 100% of turns single-question
  with a recommendation. EVAL-2: zero unanchored decisions.
- EVAL-5 follow-on: `/prd-new` on this record asks 0 decision-questions;
  every §7.3 row cites `(brainstorm:weekly-digest#D-n)`.
