# Native model routing

New executable tasks carry `task-native-routing/v1`: an explicit harness,
native model, and native effort for implementation, another explicit choice
for review, and a task-specific rationale. No numeric rung or model tier is
authoring input.

Run `go run ./cmd/plangate routes` to inspect the choices published by the
installed harness adapters. Read the role on each entry: a supported
implementation pair is not automatically a supported review pair. Choose each
role independently based on the task's behavior, change surface, verification
work, and consequences of a missed defect. The native spec records the choice;
it does not ask execution to reassess the task or silently substitute a model.

Every task, including mechanical work, needs both routes. Deterministic tools
such as folds, projections, and gates run as code and do not need task routes.

`plan-template/v2` carries the native spec in each task's `Native routing` row.
Plangate rejects missing choices, unknown harnesses, unsupported model/effort
pairs, and legacy routing rows in a native plan. The review preview shows both
routes and the rationale. Review changes to these fields with the charter.

Frozen plan/v1 artifacts retain their original bytes and compatibility reader.
Their historical routing metadata is not the format for new work.

For a direct Carina task, fill `.pyxis/templates/native-task-template.json`
and run `carina create --native-task <file.json>`. The command validates the
charter and both catalog choices before creating an item with a complete native
spec. It generates the task identity and source fingerprint. Use the existing
`carina signal add <id> agent` action when the task is ready for agent pickup.
