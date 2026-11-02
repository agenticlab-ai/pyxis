# Native model routing

Executable tasks carry `task-requirements/v1`: independent assessments and
mechanically derived capability, deliberation, context, and assurance profiles
for implementation and review, with a task-specific rationale for each role.
Planning records requirements; execution selects a harness, native model, and
native effort from the installed adapters' qualified catalogs.

Run `go run ./cmd/plangate routes` to inspect the choices published by the
installed harness adapters. Each catalog is role-specific: a supported
implementation choice is not automatically a supported review choice.
Execution uses each role's frozen requirements without reassessing the task.

Every task, including mechanical work, needs both assessments and profiles.
Deterministic tools such as folds, projections, and gates run as code and do
not need task routes.

`plan-template/v2` carries the contract in each task's `Requirements` row.
Plangate rejects missing assessments, unknown dimensions, and profiles that
do not match their derivation. The preview shows both roles and their rationale.
Review changes to these fields with the charter.

Historical plans retain their original bytes and remain readable for reference.
Their task carriers cannot be projected or executed; unfinished work must be
replanned with current requirements. Pending reviews require a persisted native
route or complete semantic demand. Routing evidence with retired event kinds
is refused; historical records are never rewritten into current evidence.

For a direct Carina task, fill `.pyxis/templates/native-task-template.json`
and run `carina create --native-task <file.json>`. The command validates the
charter and both role requirements before creating an item with a complete
native spec. It generates the task identity and source fingerprint. Use the
existing `carina signal add <id> agent` action when the task is ready for pickup.

## Deprecated legacy routing designs

Deprecated by operator instruction on 2026-09-25. These notices apply to the
numeric model-routing mechanism only. The linked design documents retain their
original bytes; their unrelated contracts remain in force.

| Historical design | Deprecated portion |
|---|---|
| [Routing envelope v1](../docs/tech-design/routing-envelope-v1.md) | Entire numeric routing envelope and model ladder. |
| [Execution demand contract v1](../docs/tech-design/execution-demand-contract-v1.md) | Plan/v1 compatibility origin, numeric translation table, and reconstruction path. |
| [Implement-now](../docs/tech-design/implement-now.md) | Numeric task routing, adapter tier maps, and scalar dispatch telemetry. |
| [Routed execution](../docs/tech-design/implement-now-routed-execution.md) | Numeric model selection and compatibility routing. |
| [Authenticated review obligations](../docs/tech-design/implement-now-authenticated-review-obligations.md) | Numeric routing fields and recovery through legacy demand translation. |
| [Routing evidence v1](../docs/tech-design/routing-evidence-v1.md) | Legacy translation event and its producer authority. |
| [Plan-new routed demand](../docs/tech-design/plan-new-routed-demand.md) | Legacy plan compatibility for executable projection. |
| [Local work tracker](../docs/tech-design/local-work-tracker.md) / [v2](../docs/tech-design/local-work-tracker-v2.md) | References to numeric capability sizing. |
| [Predecessor domain model](../docs/domain-model.md) | Historical Spec, AgentRuntime, and RoutingPolicy descriptions of numeric model routing. |
