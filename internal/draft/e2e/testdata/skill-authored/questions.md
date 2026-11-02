# Scripted operator transcript — widget-summary

Only answers received live from the parent fixture host establish rulings. The supplied answer bank is not itself an answer event.

## Turn 1 — framing

**Where we are:** the source scan found no widget-summary capability or prior refusal; Pyxis's product vision describes a software factory, so this generic widget idea is outside its product domain and is being evaluated as a fixture only.

**The question:** which feature should “Add a widget summary” mean in this evaluation?

**Example (hypothetical):** a Go caller has two widgets, one active and one archived, and needs a summary.

- **A. A pure Go function (recommended)** — the caller receives counts `{Total:2, Active:1, Archived:1}` from an in-process call; this keeps the fixture bounded and independently testable.
- **B. A CLI report** — the caller runs a command that prints those counts, introducing command input and output behavior.
- **C. A persisted dashboard** — the caller stores the widgets and reads a screen showing those counts, introducing storage and UI behavior.

Answer received live: A, pure in-process Go function. On two widgets active+archived, return Summary{Total:2, Active:1, Archived:1}. This is synthetic evaluation only, not a real Pyxis feature expansion.

## Turn 2 — users and job

**Where we are:** the fixture returns counts through an in-process Go call; the intended caller and job still need a ruling.

**The question:** should the primary user be a Go developer who already has widgets in memory and needs aggregate counts, with people needing a command, a screen, or stored data outside this feature?

**Example (hypothetical):** a Go developer has one active and one archived widget in a slice.

- **A. Local developer and counts only (recommended)** — that developer calls the function and receives 2 total, 1 active, 1 archived; a person wanting a screen has no supported workflow here. This matches the selected local function boundary.
- **B. Broader interactive user** — the same slice must be presented through a user workflow beyond the function; the developer also gets a command or screen to expose the counts.

Answer received live: A. Primary user is a Go developer with widgets already in memory; job is exact aggregate counts. Only the local caller owns any presentation or storage.

## Turn 3 — domain and validation

**Where we are:** the local caller and aggregate-count job are settled; input validity determines whether those counts can be trusted.

**The question:** should the function accept at most 10,000 widgets, require a nonempty ID and active or archived state on each, count duplicates, and reject any invalid input with a typed invalid-widget error and a zero summary?

**Example (hypothetical):** input contains `{ID:"x", State:active}`, the same widget again, and `{ID:"", State:archived}`.

- **A. Reject the whole invalid input; count duplicates on valid input (recommended)** — this example returns a typed ErrInvalidWidget and Summary{}; removing the empty-ID row returns `{Total:2, Active:2, Archived:0}`. A complete rejection prevents partial counts from masquerading as a full result.
- **B. Skip invalid rows and deduplicate** — this example returns successful `{Total:1, Active:1, Archived:0}` after ignoring the invalid row and merging duplicates.
- **C. Count every row without validation** — this example returns successful `{Total:3, Active:2, Archived:1}` despite the empty ID.

Answer received live: A. Limit 10,000 widgets; nonempty ID; active/archived only. Duplicates counted individually. Any invalid input returns ErrInvalidWidget and Summary{}, no partial counts.

## Turn 4 — API and cancellation

**Where we are:** validity and count semantics are settled; the caller needs a cancellable API.

**The question:** should the API be `Summarize(ctx context.Context, widgets []Widget) (Summary, error)`, where Widget has ID string and active/archived State, Summary has Total/Active/Archived int, and cancellation throughout the input scan returns ctx.Err() with a zero summary?

**Example (hypothetical):** the caller cancels its context while a 10,000-widget valid slice is being scanned.

- **A. Context-aware whole result (recommended)** — the scan honors cancellation and returns ctx.Err() plus Summary{}; the caller can distinguish cancellation from a valid completed count.
- **B. Ignore cancellation** — the function finishes counting and returns the complete 10,000-widget summary despite the cancellation.
- **C. Return a partial count on cancellation** — the function returns ctx.Err() and whatever subset count was accumulated before cancellation.

Answer received live: A. Use exact API/types given, check cancellation throughout scan and return ctx.Err()+Summary{}, not a partial result.

## Turn 5 — scope, success, and appetite

**Where we are:** the API, exact counts, validation, and cancellation are settled; the final scope and success checkpoint must bound the work.

**The question:** should this T1 fixture be limited to the pure function, hermetic table-driven unit tests, and documentation, using Go and the standard library, with all acceptance cases passing before proposal?

**Example (hypothetical):** the counts and rejection behavior pass every table-driven acceptance case, but someone proposes adding telemetry, a new port, or a widget-storage layer before presenting the change.

- **A. Bounded function delivery (recommended)** — the proposal can proceed after function/tests/docs and all acceptance cases pass; those additions are excluded, input stays unchanged, and only the local caller observes results or errors. This gives the bet an observable checkpoint without growing the fixture.
- **B. Expanded runtime delivery** — the proposal waits for the extra telemetry, port, or storage work even though the count cases already pass.

Answer received live: A bounded delivery. Only function, hermetic table tests, documentation, Go+stdlib; checkpoint all acceptance cases pass before proposal. No input mutation, IO, persistence, CLI/UI, telemetry, new port, cross-process schema, or dependency. Caller observes returned result/error. No remaining product decision gap.

## Turn 6 — explicit stage-1 gate

**Where we are:** five live decisions cover problem framing, caller and anti-segment, job, success, exclusions, and scope appetite. No fog remains. The template structure checklist passes up to the gate entry; the six coverage rows pass: problem D-1; users D-2; job D-2; goals D-5; non-goals O-1/O-2; appetite D-5. The gate entry will be recorded only after your explicit verdict.

**The question:** which verdict ends this stage-1 fixture session?

**Example (hypothetical):** the prepared record contains the bounded local-function decisions just confirmed, ready to feed requirements authoring.

- **A. Ready for PRD (recommended)** — retain this complete record as the fixture's requirements input and permit PRD synthesis; all six inputs are covered with zero blocking fog.
- **B. Needs investigation** — retain the record, pause requirements authoring, and name the unresolved decision that requires investigation.
- **C. Abandoned** — stop this idea and remove its working record.

Answer received live: A, ready-for-prd. Simulator principal alice, 2026-09-27. This is no actual Pyxis contract gate. PRD synthesis from this exact record is authorized fixture activity; no new product questions when the bank is settled.

## Stage-1 checklist result

Appendix A passes: complete header; all sections present; five-field live-answer decisions; zero decisions from answer-bank notes; no supersession; no untyped fog; both exclusions cite decisions; exactly one explicit gate; ready-for-prd coverage result present; dense anchors; concise entries. Appendix B passes all six inputs as listed in turn 6 and in the Gate row.

## PRD synthesis

Decision questions: 0. All product choices are taken from live stage-1 answers. Implementation details remain for the design stage. No production approval is represented by this evaluation.
