# Amendment 003 — Contract-gate precondition: gate the feature contract before any plan output

| Field | Value |
|---|---|
| **Amends** | plan-new *(chain: 000 + 001 + 002)* |
| **Status** | Approved |
| **Owner** | @mdarabi |
| **Date** | 2026-07-17 |
| **Learned-from** | operator-decision:contract-gate-planning-entry-precondition-2026-07-17 |
| **Blast radius** | tier: additive; touches: FR-27; reason: adds one Must FR introducing the contract-gate planning-entry precondition; supersedes, retires, and rescopes nothing — plan-new's existing rows are byte-untouched (contract-gate NR-1) |
| **Template version** | amendment-template/v2 |

## Operations

### op: add FR

Reason: The mechanical stage-6 contract gate (`docgate contract-gate <feature>`) now exists — a read-only, feature-scoped verdict over a feature's assembled contract (contract-gate FR-1). Planning must not begin against a contract that gate reds: a plan drafted over an incomplete, stale, unresolved, or malformed contract inherits the gap. This precondition fires at planning entry — before plan-new drafts, freezes, or projects — and refuses on any non-clean verdict, surfacing the emitted defect report as the bounce (contract-gate design §3.10, FR-1, NR-1).

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-27 | Before drafting, freezing, or projecting a plan, the planner shall run `docgate contract-gate <feature>` over the feature's assembled contract and shall refuse to proceed on any non-clean verdict, surfacing the emitted defect report to the operator; only a clean verdict permits planning to begin. | plans derive from a mechanically-clean contract only — the interim stage-6 gate (NG-2) is now a real verb, so the planning-entry hard gate is enforced rather than assumed; a bounce yields zero plan output and the defect report is the bounce-back to the contract owner (contract-gate design §3.10) | Must | high | integration fixtures: a bouncing contract (a check reds) produces zero draft, freeze, or projected output and surfaces the defect report; a clean contract permits planning to proceed; the precondition runs before the plan-level coverage (FR-26) and fog (FR-16) checks | Test | J1, G1, NG-2 |
