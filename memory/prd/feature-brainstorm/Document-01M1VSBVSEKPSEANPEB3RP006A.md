# Amendment 003 — question-legibility

| Field | Value |
|---|---|
| **Amends** | feature-brainstorm *(chain through 002)* |
| **Status** | Approved |
| **Owner** | @mdarabi |
| **Date** | 2026-07-30 |
| **Learned-from** | operator-decision:2026-07-30 live operator session — @mdarabi ruled the example-anchored, plain-register question form after reporting the cognitive load of answering session questions as the stage's blocker |
| **Blast radius** | tier: contract-changing; touches: FR-18 (new Must), EVAL-7 (new); reason: adds a Must interview-form row — the register a question is put to a human principal in becomes contract, completing FR-4's cheap-confirm intent against the R3 rubber-stamping risk; no existing row changes |
| **Tracking** | PR #854 — the companion guide + fixture change (the FR-18 form in the live authoring guide; the EVAL-7 `question-form` fixture); first graded eval run rides the next on-demand sweep (WR-5) |
| **Template version** | amendment-template/v2 |

## Operations

### op: add FR

Reason: FR-4's recommendation makes a turn a cheap confirm only if the question itself is cheap to parse — observed sessions present questions in the record's compressed, anchor-dense register, leaving the operator to dereference `F-n`/PR anchors and simulate each option's consequence in their head; the register of the conversation was never contracted, so it defaulted to the register of the record.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-18 | While interviewing a human decision principal (the human realization of FR-4 — FR-16), the skill shall present every question turn in the principal's register rather than the record's: the question phrased in plain language with every house term or corpus anchor glossed inline (never a bare `F-n`/`D-n`/document reference), grounded in one concrete example — looked up from the corpus, code, or prior records where one exists, marked hypothetical otherwise, or replaced by an explicit `no example: <reason>` line when no illustrative case exists — and carrying an enumerated option set in which every option states its outcome on that same example (outcomes pairwise distinct, the FR-4 recommendation marked among them), or one to two sample answers with their meaning where the question is open-form; the record keeps its own register (NFR-2) untouched. | a question the principal cannot cheaply parse is answered by laborious decoding or by rubber-stamping the recommendation (R3), hollowing the G3 experience the stage exists to protect; one example threaded through every option makes the options comparable — same scenario, divergent endings — and an interviewer that cannot instantiate its options on a concrete case does not yet understand its own question (operator ruling 2026-07-30) | Must | med | across the fixture corpus, 100% of human-principal question turns carry the form — plain-language question, example or typed skip, enumerated options each stating its outcome on the turn's example (or sample answers for open-form), recommendation marked; zero turns present a corpus anchor without its inline gloss; zero example-bearing turns where two options share an outcome (EVAL-7) | Eval | J1, G3 |

### op: add EVAL

Reason: the form is checkable — presence of the elements and anchor glossing mechanically, example fit and outcome divergence by judgment — and belongs beside EVAL-1's turn-shape thresholds it sharpens.

| ID | Behavior | Eval set / harness | Threshold | Negative examples |
|---|---|---|---|---|
| EVAL-7 | question turns legible in the principal's register — example-anchored comparable options, no bare anchors (FR-18) | fixture corpus, all human-principal interview turns (agent-principal fixtures exempt — FR-16) | 100% of turns carry the FR-18 form; zero unglossed corpus anchors; zero example-bearing turns with two options sharing an outcome; typed example-skips only where no illustrative case exists (judged at the report); example fidelity — every example presented as looked-up verifies against its named source at the report, and every invented example is marked hypothetical (a fabricated-as-real example is a fail) | a question quoting record IDs ("is the F-7 loop in scope?") with no gloss; options ("in scope / deferred / dropped") stating no outcomes; each option illustrated on a different example; a forced example bolted onto a pure naming question instead of a typed skip; a recommendation present but buried in prose with no option to confirm by label; an invented example presented as a real case, or a hypothetical left unmarked |
