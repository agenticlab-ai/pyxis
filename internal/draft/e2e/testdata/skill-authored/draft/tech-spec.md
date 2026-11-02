# Widget summary — tech spec

| Field | Value |
|---|---|
| **Status** | Draft — simulator direction gated ready-for-design; review and lock pending; evaluation only |
| **Owner** | alice — simulator architect |
| **Date** | 2026-09-27 |
| **Relevant-when** | implementing or reviewing the isolated widget-summary function and its count, validation, cancellation, or preservation behavior |
| **Basis** | draft:widget-summary/prd.md after problems.md#P-1 operator Ruling |
| **Tracking** | Scripted host PR1 on draft/widget-summary; no production PR |
| **Security/privacy impact** | none — no network, identity, secrets, egress, persistence, or production control surface |

## 0. Read to rule — the human reviewer's section (~2 min)

*The body is the contract; this section only routes attention — on any conflict the body governs; machines ignore it.*

**Judge these**

| # | The call | Could have gone | If you disagree |
|---|---|---|---|
| 1 | One read-only scan → D-1 | Two passes → D-1 Alternatives, ALT-2 | Revisit the simulator's chosen direction before changing D-1. |

**Sanity-check the numbers** — 10,000-entry maximum → D-1 Concrete layout; zero summary on every error → DINV-2; every acceptance case passes before proposal → D-1 Verification.

**The author's open questions** — none — D-1 Open questions is empty.

**Skip the rest** — types/control flow → D-1 Concrete layout and implementation checks; correctness → D-1 Acceptance; rationale → D-1 Alternatives. Draft review and lock remain pending.

## Decisions

### D-1 — Read-only single-scan widget summary

**Decision:** Implement the settled context-taking function with one standard-library loop that validates and counts each input entry without modifying caller-owned widgets. Keep counts local until the operation succeeds; return Summary{} on validation or cancellation errors. No port, dependency, telemetry, IO, persistence, CLI/UI, or process-crossing schema is added. Direction: (design-brainstorm:widget-summary-tech-design#D-1) in brainstorm/003.md; scope: (design-brainstorm:widget-summary-tech-design#S-1).

**Serves:** FR-1 · FR-2 · FR-3 · FR-4 · UB-1 · UB-2 · UB-3 · UB-4 · NR-1 · DOD-1 · DOD-2 · DOD-3 · DOD-4

#### Concrete layout

The implementation consists of package widgetsummary under internal/widgetsummary, with summary.go containing the types/function and summary_test.go containing the hermetic tables. These are implementation targets, not files created by this authoring fixture. Documentation lives with the Go API. All engineering layout is inside this serving decision; no separate solution-shape document exists.

Use these in-process Go shapes; they have no serialization or storage role:

```go
type State string

const (
	StateActive   State = "active"
	StateArchived State = "archived"
)

type Widget struct {
	ID    string
	State State
}

type Summary struct {
	Total    int
	Active   int
	Archived int
}

type ErrInvalidWidget struct{}

func (ErrInvalidWidget) Error() string { return "invalid widget" }

func Summarize(ctx context.Context, widgets []Widget) (Summary, error)
```

Callers pass a nonnil context, following the Go context package's documented convention; the function neither stores the context nor starts a goroutine. The zero State value and every value other than the two constants are invalid. An ID is valid exactly when ID != ""; no trimming, normalization, or uniqueness check is performed. ErrInvalidWidget is an error type, discoverable with errors.As; it is not an untyped message contract.

Control flow:

1. Read ctx.Err(); if nonnil, return Summary{} and that same error.
2. If len(widgets) > 10,000, return Summary{} and ErrInvalidWidget{}.
3. Initialize one local Summary. For each entry, check ctx.Err(), then validate the ID and state. A failed validation returns Summary{} and ErrInvalidWidget{} immediately.
4. For each valid entry, increment local Total and the matching local state counter. Never write through the input slice, sort entries, deduplicate IDs, or archive widgets.
5. Check ctx.Err() again before publishing the completed summary; a nonnil error returns Summary{} and that same error. Otherwise return the local summary and nil.

The first observed failing check ends computation: context checks precede the length check and each entry's validation, following the operator ruling at problems.md#P-2 and brainstorm:widget-summary#D-6. A cancellation observed at such a check wins over the subsequent invalid-input check. If that context check returns nil and validation fails, return the validation error immediately; later cancellation does not change an already returned error. Counts accumulated before a later failure are never returned. Uncancelled nil/empty slices pass through with Summary{} and nil. The loop uses O(n) work and O(1) auxiliary state; the input cap bounds every counter by 10,000. There is no second input traversal or copy in production.

#### Boundaries and invariants

This decision implements only the settled function, tests, and documentation. Existing caller presentation/storage responsibilities remain outside the function. NG-1, NG-2, NG-3, and NG-4 remain binding boundaries, adopted in design-brainstorm:widget-summary-tech-design#O-1.

- **DINV-1** — Slice length, order, IDs, and states remain unchanged on every return path.
- **DINV-2** — Every nonnil returned error is paired with Summary{}.
- **DINV-3** — Successful Total equals Active + Archived and the number of input entries; duplicate IDs contribute once per entry.
- **DINV-4** — The context is checked before processing each entry and before successful completion; cancellation returns the observed ctx.Err().

#### Observability

- Events/logs: none — runtime telemetry is excluded by the product scope.
- Metrics: none — acceptance evidence comes from the test suite, without production measurement.
- Alerts: none — this function creates no monitored service or worker.
- Debugging: the caller observes complete Summary/nil, typed ErrInvalidWidget/zero Summary, or context error/zero Summary; table tests distinguish these outcomes.
- Trace/correlation: caller-provided ctx passes directly into the operation; the function creates no trace record or correlation store.

#### Security and privacy

none — this fixture has no network, identity, permission, secret, egress, persistence, billing, or production-control surface. Caller-owned synthetic IDs stay in caller memory. The input cap and state/ID checks enforce the product domain; no authorization role or security adapter is introduced.

#### Failure behavior and recovery

| Failure | Returned outcome | Detection | Caller recovery |
|---|---|---|---|
| Too many entries, empty ID, or invalid state after a context check returned nil | Summary{}, ErrInvalidWidget{}; return immediately | errors.As to ErrInvalidWidget; UB tables | Supply valid bounded input. |
| Cancellation or deadline observed | Summary{}, exact ctx.Err() | Context-error table assertions | Retry with an uncancelled context if the caller still needs a summary. |

No failure mutates input or creates a side effect to undo. No retry loop is implemented inside the function.

#### Alternatives

| Shape | Verdict and reason | Provenance |
|---|---|---|
| One validation/count loop | Chosen by simulator architect; settled outcomes with one traversal and local counters. | design-brainstorm:widget-summary-tech-design#ALT-1 |
| Validate, then count in a second pass | Rejected by simulator architect; unnecessary repeated work. | design-brainstorm:widget-summary-tech-design#ALT-2 |

The architect explicitly waived the parallel alternatives panel for the pure in-process scope. The waiver and its reason remain in brainstorm/003.md's DQ-1 panel log.

#### Risks

| ID | Risk | Mitigation | Verification | Owner |
|---|---|---|---|---|
| R-1 | Deduplication or state conversion corrupts counts/input. | Direct read-only per-entry counting; DINV-1/DINV-3. | Duplicate and preservation table cases. | Fixture implementer |
| R-2 | Error returns leak accumulated counts. | Return the literal zero Summary on all error paths; DINV-2. | Invalid-last and mid-scan cancellation cases. | Fixture implementer |
| R-3 | Cancellation is checked only at entry. | Per-entry and final checks; DINV-4. | Deterministic cancellation during scanning. | Fixture implementer |

#### Verification and acceptance

All dependencies are in-process: standard-library context and the input values are tested directly. There is no remote medium, injected production port, test service, wall-clock wait, network call, or random input source. Tests compare input against a pre-call copy; copying is test-only.

Use hermetic table-driven tests for nil/empty, active-only, archived-only, mixed, duplicate same-state and opposite-state IDs, 10,000 valid entries, 10,001 entries, empty ID first/last, invalid state first/last, pre-cancelled context, expired context, and cancellation during scanning. For deterministic mid-scan cancellation, a test-only context wrapper forwards the Context methods and calls a real CancelFunc at a chosen Err-check count; the wrapper varies the existing Context input rather than creating a production port. An expired-context stub supplies DeadlineExceeded without consulting a wall clock. Each error case asserts Summary{} and error identity/type; every case asserts unchanged input.

| ID | Acceptance criterion | Verify | Required evidence | Traces to |
|---|---|---|---|---|
| A-1 | All valid/empty/state-count/duplicate/boundary cases produce the exact expected summary and nil error. | Test | Table-case outcomes | FR-1 · FR-2 · FR-3 · FR-4 · DOD-1 |
| A-2 | Over-limit, empty-ID, and invalid-state cases whose preceding context check returned nil return typed ErrInvalidWidget and Summary{} immediately, including invalid-last cases. | Test | Failure-table outcomes and errors.As assertions; cancellation not observed until after validation cannot replace the returned error | UB-1 · UB-2 · UB-3 · DOD-2 · DINV-2 |
| A-3 | Pre-cancelled, expired, and mid-scan cancellation observed at the specified checks returns that context error and Summary{}, before subsequent validation or successful return. | Test | Deterministic cancellation-case outcomes; pre-cancelled over-limit/empty-ID/invalid-state inputs and cancellation before an invalid last entry return the context error and zero summary | UB-4 · DOD-2 · DINV-4 |
| A-4 | Every acceptance case preserves the caller's complete input slice. | Test | Pre-call/post-call equality assertions | FR-1 · NR-1 · DOD-3 · DINV-1 |
| A-5 | Source and documentation preserve Go/stdlib-only, single-scan, pure local return-value behavior and every scope exclusion. | Inspection | Source/dependency inspection and API documentation | DOD-4 · NG-1 · NG-2 · NG-3 · NG-4 |

All table-driven acceptance cases pass before proposal. This authoring fixture has not implemented the function or executed that future suite; the table specifies the required completion evidence.

#### Principles

- AP-10 — honored: deterministic validation/counting lives in Go code with tests.
- AP-6 — honored within scope: the settled typed local function carries inputs/results; no process-crossing schema is introduced.
- AP-7 — honored: the draft product problem is ruled before its requirement sentence is changed; the earlier design attempt and its gate remain intact.
- RW-5 — honored: the concrete layout is contained in the serving technical decision before implementation.
- AP-5 — deviation: this pure function emits no runtime events or logs, following the live operator's explicit caller-only observability and no-telemetry scope. Returned errors/results and acceptance evidence expose success and failure; no external trace surface exists.

**Deviation:** AP-5 runtime event emission is intentionally omitted for the operator-authorized pure-function scope; no other deviation is declared.

#### Open questions

none — the product contradiction is ruled at problems.md#P-1 and the simulator's explicit ready-for-design gate selects this direction. Reviews, implementation evidence, and lock approval remain future workflow steps, not unresolved design decisions.
