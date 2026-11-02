# Session log — review implement-now worker flow against the ideal local-CI-first loop

Started: 2026-08-31 17:23 · CLI: Claude Code · Stage or skill: chore (gap analysis, no code change)



## P-1 — build the local-first convergence loop without the PRD / design / plan path

- **Found:** 2026-08-31 17:40
- **Problem:** The repo rule says implementation starts only from an approved technical design, and the frozen implement-now contract pins review after remote CI as advisory. The requested workflow (local gate first, review-then-fix loop before any push, remote CI last) changes that order and makes review consumed.
- **Blocked:** Writing the code contradicts the frozen contract; I could not settle which rule wins.
- **Asked:** 2026-08-31 17:30 — laid out that the change needs PRD amendments plus a superseding design, and what the code gaps are.
- **Decided:** 2026-08-31 17:40 — the operator directed: implement the workflow directly in pyxis, without going through the PRD / tech-design / plan process.
- **How:** Built the loop in the executor code path; contracts left for the operator to reconcile later.
- **Refs:** internal/executor/worker, internal/executor/drivetogreen, cmd/implement-now; docs/prds/implement-now (FR-7..9, FR-40..49, DOD-32, UB-21); docs/tech-design/implement-now-authenticated-review-obligations.md

## P-2 — how far a task's session may keep answering review rounds

- **Found:** 2026-09-01 08:40
- **Problem:** Building task session continuity needed two bounds the code cannot pick on its own: what human activity on an open proposal wakes a feedback round, and how many rounds a task may spend before a human must step in.
- **Blocked:** Either choice materially changes the loop (a keyword opt-in versus any comment; a hard cap versus none), and the cost of guessing wrong is either an unresponsive engine or an unbounded one.
- **Asked:** 2026-09-01 08:45 — which activity counts as feedback, and what the per-proposal round bound should be.
- **Decided:** 2026-09-01 08:55 — any new human comment on any proposal surface, or a changes-requested review, wakes a round; approvals alone do not. Human-review rounds are unbounded (each is one human action, serviced once); automated (LLM) review remediation is bounded at three rounds.
- **How:** Feedback detection admits a round per new human activity with a durable cursor advanced at admission; `review.remediation_rounds` defaults to 3.
- **Refs:** internal/executor/dispatch/feedback.go, internal/executor/tasksession, internal/executor/config (remediation_rounds, session.continuity)
