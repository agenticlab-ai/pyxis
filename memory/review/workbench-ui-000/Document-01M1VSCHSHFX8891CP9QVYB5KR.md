# Review: workbench-ui 000-base — review report

| Field | Value |
|---|---|
| **In one line** | Approve-with-advisories: the contract is executable as drafted; five advisories applied — a served-surface abuse case (browser-origin probing), a launch-collision failure row, two fit/wording tightenings, and a class-extensibility maintainability requirement. |
| **Relevant-when** | auditing or consuming the review of the workbench-ui base PRD; asking why it was approved; tuning prd-review calibration against real findings |
| **Subject** | prd:workbench-ui/000 |
| **Moment** | new-base |
| **Reviewer** | prd-review · guide as merged 2026-07-10 |
| **Date(s)** | 2026-07-10 |
| **Verdict** | approve-with-advisories |
| **Tracking** | the subject's introducing PR (this PR) |
| **Template version** | review-report/v1 |

## Round 1

### FND-1 — the served surface's most realistic abuse vector is absent

- **Severity**: advisory (CAL-4)
- **Lens**: completeness-critic
- **Evidence**: §5.3 covers a local process (ABUSE-1) and hostile document content (ABUSE-2), but not a hostile *web page in the local browser* issuing requests at the loopback server (cross-origin fetch / DNS rebinding) — the standard local-dev-server vector — vs — §5.3's own charter ("invert the use cases") and the vendored review runtime's precedent of session-scoped URLs.
- **Acceptance condition**: an ABUSE row naming the browser-origin vector with a mitigation requirement (session secret and/or Host validation) wired into the security NFR's fit criterion.
- **Disposition**: applied — ABUSE-3 added; NFR-2 strengthened to refuse requests without the session secret or with an unexpected Host.

### FND-2 — launch collision has no failure row

- **Severity**: advisory (CAL-4)
- **Lens**: completeness-critic
- **Evidence**: §5.2 covers wrong-repo, missing deps, and unparseable docs, but not the entry failure "port cannot be bound / another session already serving" — vs — the template's §5.2 charter, whose enumeration explicitly includes "conflict".
- **Acceptance condition**: a UB row: bind/instance collision yields a named defect (path/op/reason/remedy) and no half-served state.
- **Disposition**: applied — UB-4 added.

### FND-3 — FR-2's fit criterion leaves its own bar undecided

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: FR-2 fit: "byte- or DOM-level equivalence per class" — an unresolved either/or inside a Must row's pass/fail bar — vs — Appendix A's banned-vagueness class (open alternatives are loopholes) and the fit-criterion rule (the measurement decides pass/fail).
- **Acceptance condition**: one decided bar: equivalence at the level each class's golden fixtures pin, recorded per class in the fixtures themselves.
- **Disposition**: applied — fit criterion reworded to pin the bar to the per-class golden fixtures.

### FND-4 — NFR-1's requirement sentence hides its numbers in the fit column

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: NFR-1 statement says "interactive at repo scale … within human-immediate budgets" (subjective-adjacent wording) while the numbers live only in the fit criterion — vs — Appendix A: subjective language struck on sight; the system is the named subject stating measurable response.
- **Acceptance condition**: the requirement sentence states the budgets it is held to; the fit criterion measures them.
- **Disposition**: applied — NFR-1 restated with the launch and response budgets in the requirement text.

### FND-5 — class extensibility is unstated on a surface every future class will touch

- **Severity**: advisory (CAL-4)
- **Lens**: completeness-critic
- **Evidence**: FR-1 serves "the corpus grouped by class" but nothing states the cost of the *next* class — the reading room could become the central edit-point every new document class must modify — vs — the principles registry's extension-is-by-addition rule (a shared append point is a design defect) and the recurring-paths-without-stated-costs check in this lens's charter.
- **Acceptance condition**: a maintainability requirement: admitting a new document class does not modify the surfaces serving existing classes beyond registering the new one, proven by a class-onboarding fixture.
- **Disposition**: applied — NFR-4 (maintainability) added with a synthetic-class conformance fixture as its fit criterion.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | approve-with-advisories | 2026-07-10 |

## Escalation brief

none

## Measurements

- **Rounds**: 1
- **Findings**: 0 blocking · 5 advisory
- **Dispositions**: 5 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

none
