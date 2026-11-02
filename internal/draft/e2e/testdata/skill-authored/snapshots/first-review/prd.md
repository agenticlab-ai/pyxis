# Widget summary — PRD

| Field | Value |
|---|---|
| **In one line** | A pure Go function returns exact total, active, and archived widget counts or an error with a zero summary. |
| **Relevant-when** | implementing or reviewing the isolated widget-summary function; triaging counting, validation, cancellation, or input-preservation defects |
| **Status** | Draft — simulated problem basis accepted; review and lock pending; evaluation only |
| **Tier** | T1 |
| **Owner** | alice — simulator principal |
| **Approver(s)** | alice — scripted fixture reviews; no production approval |
| **Contributors** | fixture authoring agent |
| **Informed** | parent fixture host |
| **Date** | 2026-09-27 created · 2026-09-27 last updated |
| **Appetite** | Scope budget: pure function, hermetic table-driven tests, documentation; Go/stdlib; human attention at scripted decision/review events; machine work limited to these artifacts; no added runtime or rollout burden. No numerical token/money budget supplied. |
| **Target release** | All table-driven acceptance cases pass before the fixture proposal. |
| **Tracking** | Scripted host PR1 on draft/widget-summary; no production PR |
| **Artifacts** | drafts/widget-summary/brainstorm/001.md |
| **Template version** | prd-template/v2 |
| **Design coverage** | requirements-first/v1 |

## 0. Read to rule — the human reviewer's section

Skipped: T1 uses §1 and acceptance criteria directly; this document holds no §0 gates.

## 1. Summary — the one-pager

A Go developer who already holds widgets in memory needs exact aggregate counts. The agreed job is a local function returning total, active, and archived counts. This is synthetic evaluation data, with no claim of market urgency, real-user research, or Pyxis product expansion. The authoring experiment follows docs/product-vision.md §4; generic widgets are outside that factory's product domain.

The function accepts at most 10,000 widgets, each with a nonempty ID and active or archived state. Duplicate IDs count individually. Invalid input returns typed ErrInvalidWidget and Summary{}. Cancellation throughout scanning returns ctx.Err() and Summary{}. Input remains unchanged. The selected product API is `Summarize(ctx context.Context, widgets []Widget) (Summary, error)`, with Widget{ID string, State enum(active,archived)} and Summary{Total,Active,Archived int}; API implementation choices remain for technical design.

Appetite is the function, hermetic table-driven tests, and documentation using Go/stdlib. Success is all acceptance cases passing before proposal. The guardrail is zero input changes after success, rejection, or cancellation. Risks are deduplicating entries, leaking partial counts, and checking cancellation only before scanning. Presentation and storage belong to the caller; NG-1..NG-4 bound the remaining scope. Sources: brainstorm:widget-summary#D-1 through brainstorm:widget-summary#D-5.

## 2. Problem space

### 2.1 Problem statement

Skipped: T1 problem and caller basis are recorded in §1 from live scripted answers.

### 2.2 Users and segments

Skipped: §1 names the local Go caller; interactive command/screen/storage users are the anti-segment per brainstorm:widget-summary#D-2.

### 2.3 Jobs and desired outcomes

Skipped: the sole job is exact aggregate counts for widgets in memory, recorded in §1 and brainstorm:widget-summary#D-2.

### 2.4 Evidence

Skipped: live scripted answers in the gated record are the only fixture evidence; no real-user findings are claimed.

### 2.5 The journey today

Skipped: no real user journey was supplied or observed.

### 2.6 Opportunity and why now

Skipped: T1 problem basis was accepted by alice at simulated stage-1 turn 6; no production problem gate or market opportunity is asserted.

## 3. Goals, metrics, and guardrails

### 3.1 Goals as falsifiable hypotheses

Skipped: T1 success is the acceptance checkpoint stated in §1 and DOD-1..DOD-4.

### 3.2 Goals → signals → metrics

Skipped: table-driven acceptance results measure T1 success; no runtime metric or product baseline is introduced.

### 3.3 Guardrails and decision rule

Skipped: §1 states input preservation and passing every acceptance case before proposal; NR-1 and DOD-1..DOD-4 verify that rule.

### 3.4 Non-goals and waiting room

| ID | Non-goal | Why refused |
|---|---|---|
| NG-1 | CLI, UI, or presentation | Caller owns presentation; in-process function chosen. D-1, D-2. |
| NG-2 | IO or persistence | Pure local computation bounds the scope. D-5. |
| NG-3 | Input mutation | Caller-owned data remains unchanged. D-5. |
| NG-4 | Telemetry, new ports, cross-process schemas, or new dependencies | Function/tests/docs and Go/stdlib bound delivery. D-5. |

Sources use the brainstorm:widget-summary stem. Waiting room: none; no follow-on capability is committed.

## 4. Solution space

### 4.1 Approach and alternatives considered

Skipped: §1 records the local-function direction; rejected framings and validation/cancellation alternatives are retained in §7.3.

### 4.2 Scope perimeter

Skipped: the bounded function/tests/docs scope and exclusions are recorded in §1 and §3.4.

### 4.3 Experience: flows and states

Skipped: in-process call/return is the sole flow; success, empty, invalid, and cancelled outcomes are specified in §5.1–§5.2; no screen or loading state exists.

### 4.4 Scenarios

Skipped: concrete acceptance examples are the fit criteria in §5.1–§5.2; fixture solution review is pending.

## 5. System requirements

### 5.1 Functional requirements

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-1 | When valid input completes without cancellation, the widget summarizer shall return its number of entries as Total without modifying any input widget. | Exact size | Must (rank 1) | low | Nil/empty returns 0; mixed active+archived returns 2; 10,000 valid entries return 10,000. Successful cases have nil error. | Test | §1; brainstorm:widget-summary#D-1 |
| FR-2 | When valid input completes without cancellation, the widget summarizer shall return its active-entry count as Active. | Exact active count | Must (rank 2) | low | Active-only, archived-only, and mixed cases return expected Active; nil/empty returns 0. | Test | §1; brainstorm:widget-summary#D-1 |
| FR-3 | When valid input completes without cancellation, the widget summarizer shall return its archived-entry count as Archived. | Exact archived count | Must (rank 3) | low | Active-only, archived-only, and mixed cases return expected Archived; nil/empty returns 0. | Test | §1; brainstorm:widget-summary#D-1 |
| FR-4 | When valid input contains duplicate IDs, the widget summarizer shall count every input entry individually. | Entries are not unique-ID records | Must (rank 4) | low | Two active x entries return {2,2,0}; active x plus archived x returns {2,1,1}. | Test | §1; brainstorm:widget-summary#D-3 |

| ID | Existing behavior protected | Why protected | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|
| NR-1 | Caller-owned slice length, order, IDs, and states remain unchanged. | Pure function; mutation excluded | low | Input equals a pre-call copy after success, invalid input, and cancellation cases. | Test | §1; brainstorm:widget-summary#D-5 |

### 5.2 Unwanted behavior and failure handling

| ID | If… (trigger) | Then the system shall… | User sees | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|---|
| UB-1 | If input has more than 10,000 entries | return typed ErrInvalidWidget with Summary{}. | Rejected input | Must (rank 5) | low | 10,001 valid entries yield the typed error and zero summary; 10,000 valid entries succeed. | Test | brainstorm:widget-summary#D-3 |
| UB-2 | If any ID is empty | return typed ErrInvalidWidget with Summary{}. | Rejected input | Must (rank 6) | low | Empty ID first or last yields the typed error and zero summary. | Test | brainstorm:widget-summary#D-3 |
| UB-3 | If any state is neither active nor archived | return typed ErrInvalidWidget with Summary{}. | Rejected input | Must (rank 7) | low | Unknown state first or last yields the typed error and zero summary. | Test | brainstorm:widget-summary#D-3 |
| UB-4 | If context cancellation occurs before or during the input scan | return ctx.Err() with Summary{}. | Cancelled operation | Must (rank 8) | low | Pre-cancelled, expired, and deterministically mid-scan cancelled contexts yield their context error and zero summary. | Test | brainstorm:widget-summary#D-4 |

### 5.3 Permissions and abuse cases

Skipped: this T1 local fixture adds no permission, identity, or remote access surface.

### 5.4 Data

Skipped: caller-owned memory is read and a transient result returned; no persistent state, migration, retention, or egress is introduced.

### 5.5 Interfaces and integrations

Skipped: no system boundary is crossed; the function itself is the product and its selected API/types are recorded in §1 from D-4.

### 5.6 Non-functional requirements

Skipped: Go/stdlib, purity, hermetic-table testing, and caller return-value observability are settled constraints in §1/§3.4, proved by DOD-4; no latency/allocation threshold was supplied.

### 5.7 Instrumentation — the tracking plan

Skipped: runtime telemetry is excluded; table-suite results measure acceptance and callers observe returned results/errors.

### 5.8 Definition of Done

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-1 | All functional acceptance cases pass before proposal. | FR-1..FR-4; §1 success | Test | Hermetic table-suite result listing case outcomes | yes |
| DOD-2 | All rejection and cancellation cases return the required error and zero summary. | UB-1..UB-4 | Test | Table-suite failure-case results, including deterministic cancellation during scanning | yes |
| DOD-3 | Input remains unchanged on success and error. | NR-1; §1 guardrail | Test | Pre-call/post-call equality assertions in the table suite | yes |
| DOD-4 | Scope constraints and caller-observable outcomes are preserved and documented. | §1 appetite/direction; NG-1..NG-4 | Inspection | Source/dependency inspection and function documentation: Go/stdlib, hermetic table tests, no excluded runtime surface, complete return/error semantics | yes |
| DOD-5 | Staged production rollout evidence does not apply. | §6.5; NG-1, NG-2 | Inspection | Local-function scope in §1 | n/a — isolated in-process fixture introduces no production rollout surface |

### 5.9 AI behavior and evals

Skipped: the widget function has deterministic behavior; authoring evaluation creates no model-driven product surface.

## 6. Delivery

### 6.1 Phasing and milestones

Skipped: one fixture checkpoint — all acceptance cases pass before proposal, as required in §1 and DOD-1..DOD-4.

### 6.2 Dependencies

Go/stdlib, hermetic table tests, purity, and caller-only observability pass to technical stages from the record's P-1/P-2 and D-5. No new dependency or port is authorized. Implementation mechanism remains for technical design.

### 6.3 Risks, assumptions, and rabbit holes

Skipped: §1 names count, partial-result, and cancellation risks; fixture implementer owns their verification under FR-4 and UB-2..UB-4. No real-user research assumption is made.

### 6.4 Release criteria

Skipped: T1 release criteria are the blocking DOD-1..DOD-4 rows.

### 6.5 Rollout and reversibility

Skipped: this isolated local function adds no IO, persisted state, or production exposure stage; no flag, migration, or kill switch applies.

### 6.6 Launch readiness

Skipped: this fixture authorizes authoring artifacts only, not a launch; documentation is required in DOD-4.

### 6.7 Post-launch review

Skipped: no production launch or post-launch exposure is authorized.

## 7. Alignment

### 7.1 FAQ

Skipped: the bounded caller contract and failure outcomes are explicit in §1 and §5.

### 7.2 Open questions

none — no product gap or carried fog. Technical mechanism and package/type spelling belong to design; product API and outcomes are already settled.

### 7.3 Decision log

| Date | Decision | Alternatives considered | Why | Who |
|---|---|---|---|---|
| 2026-09-27 | Pure in-process function; evaluation only (brainstorm:widget-summary#D-1) | CLI report; persisted dashboard | Bounded local count result | alice — simulator |
| 2026-09-27 | Local Go caller; caller owns presentation/storage (brainstorm:widget-summary#D-2) | Broader interactive user workflow | Exact aggregate-count job and explicit anti-segment | alice — simulator |
| 2026-09-27 | Limit 10,000; valid ID/state; duplicates count; invalid error with zero summary (brainstorm:widget-summary#D-3) | Skip invalid and deduplicate; unvalidated counts | Whole-input count semantics | alice — simulator |
| 2026-09-27 | Exact context API/types; cancellation error and zero summary (brainstorm:widget-summary#D-4) | Ignore cancellation; partial result | Distinguishable complete result | alice — simulator |
| 2026-09-27 | Function/tests/docs, Go/stdlib, pure boundary; all cases pass before proposal (brainstorm:widget-summary#D-5) | Expanded runtime delivery | Scope bounds the bet; acceptance decides success | alice — simulator |

### 7.4 Changelog

| Date | Change | Author |
|---|---|---|
| 2026-09-27 | Synthesized existing draft PRD from exact gated record; zero product questions. | Fixture authoring agent |

## 8. Contract readiness — the executability gate

Authoring check of synthetic input only; no production lock or approval.

| Check | Pass? | Notes |
|---|---|---|
| Problem basis accepted — §1 accepted at T1 | yes | Explicit simulated stage-1 turn 6, alice. |
| Zero solution-blocking TBDs/open questions | yes | §7.2 contains none. |
| Every Must FR has fit criterion and verification | yes | FR-1..FR-4 use exact outcomes and Test. |
| Requirement risks declared; dangerous rows have safety plan | yes | FR/UB/NR rows low; no dangerous surface or unclassified NFR rows. |
| Non-goals drawn; non-regression named | yes | NG-1..NG-4; NR-1. |
| Every primary job covered | yes | Sole count job in §1 covered by FR and UB rows. |
| Every key flow has functional/failure/non-regression coverage | yes | Call/return in §4.3 covered by FR, UB, NR-1. |
| Metrics have instrumentation or measurement plan | yes | T1 uses table-suite acceptance results; explained §3.2/§5.7 skips. |
| Every blocking DoD row traces to a source | yes | DOD-1..DOD-4 enumerate criteria sources. |
| Risks and assumptions have owners | yes | §6.3 names fixture implementer; no fabricated research assumption. |
| Human judgment has owner and rubric | yes | No Human judgment criterion; Test/Inspection only. |
| DoD complete; blocking evidence sourced; n/a explained | yes | DOD-1..DOD-4 evidence; DOD-5 explains inapplicability. |
