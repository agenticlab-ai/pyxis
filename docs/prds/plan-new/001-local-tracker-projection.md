# Amendment 001 — WR-3 out of the waiting room: project through the WorkTracker port's local (Carina) adapter

| Field | Value |
|---|---|
| **Amends** | plan-new *(chain: 000)* |
| **Status** | Approved |
| **Owner** | @mdarabi |
| **Date** | 2026-07-15 |
| **Learned-from** | dependency-change:local-work-tracker-adapter-shipped · operator-decision:plangate-self-host-2026-07-15 |
| **Blast radius** | tier: contract-changing; touches: FR-25, WR-3; reason: rescopes waiting-room WR-3 into a Should FR (FR-25) — materializing a scoped requirement from the waiting room is contract-changing per the computed tier |
| **Template version** | amendment-template/v2 |

## Operations

### op: rescope WR-3

Reason: WR-3's revisit trigger — "the local-work-tracker adapter ships; the projection then binds through the WorkTracker port" — is met: the local (Carina) WorkTracker adapter has shipped and is in dogfood use. The projection was deliberately specified port-shaped for exactly this rebinding (decision log 2026-07-09; §5.5 one-verb-surface), so binding the local adapter is the anticipated adapter work — and it is the operator's chosen next step to self-host plan projection on Carina and exercise it live.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-25 | When configured to target the local file medium, `plangate project` shall bind the plan projection — the epic, the tasks, the parent/child hierarchy, the dependency edges, the signals, and the frozen charters — through the WorkTracker port's local (Carina) adapter, selected by configuration, with the planner, the decomposition, and the plan schema unchanged. | WR-3's trigger is met (the Carina adapter shipped) and the projection was specified port-shaped for this rebinding (§5.5, decision log 2026-07-09); binding a second, radically different medium is what proves the projection's medium-independence rather than asserting it (AP-1) — and it lets pyxis dogfood its own tracker. | Should | med | `plangate project` against a local Carina store creates the same epic/task/hierarchy/edge/signal/charter structure the GitHub binding does and converges idempotently under re-runs (zero duplicates, zero mutation of pre-existing items); the planner, decomposition, and plan-schema code is byte-unchanged from the GitHub binding (diff inspection); the local adapter passes the same tracker behavior the GitHub adapter does | Test + Inspection | J5, FR-18, FR-19 |
