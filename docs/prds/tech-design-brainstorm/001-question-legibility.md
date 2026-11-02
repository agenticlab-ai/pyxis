# Amendment 001 — question-legibility

| Field | Value |
|---|---|
| **Amends** | tech-design-brainstorm *(first amendment on 000-base)* |
| **Status** | Approved |
| **Owner** | @mdarabi |
| **Date** | 2026-07-30 |
| **Learned-from** | operator-decision:2026-07-30 live operator session — @mdarabi ruled the example-anchored, plain-register question form after reporting the cognitive load of answering session questions as the stage's blocker |
| **Blast radius** | tier: contract-changing; touches: FR-17 (new Must), EVAL-7 (new); reason: adds a Must interview-form row — the register a question is put to the human authority in becomes contract, completing FR-3's cheap-confirm intent; worded for this discipline independently of the stage-1 sibling row (NG-3: no shared primitive); no existing row changes |
| **Tracking** | PR #855 — the companion guide + fixture change (the FR-17 form in the live authoring guide incl. the fired-panel presentation line; the EVAL-7 `question-form` fixture); first graded eval run rides the next on-demand sweep |
| **Template version** | amendment-template/v2 |

## Operations

### op: add FR

Reason: FR-3's recommendation makes a turn a cheap confirm only if the question itself is cheap to parse — design questions are the deepest in the pipeline, and turns presented in the record's compressed, anchor-dense register leave the authority dereferencing `DQ-n`/`ALT-n` anchors and simulating each shape's consequences in their head; the conversational register was never contracted, so it defaulted to the record's.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-17 | While interviewing the authority, the skill shall present every question turn in the authority's register rather than the record's: the question phrased in plain language with every house term or corpus anchor glossed inline (never a bare `F-n`/`DQ-n`/`ALT-n`/document reference), grounded in one concrete example — a real case looked up from the basis, code, or corpus where one exists, marked hypothetical or illustrative otherwise, or replaced by an explicit `no example: <reason>` line when no illustrative case exists; for a structural question (a seam, port, or document-class choice) the example is one concrete flow traced through each candidate shape — and carrying an enumerated option set (on a fired panel, the recorded alternatives) in which every option states its outcome on that same example (outcomes pairwise distinct, the FR-3 recommendation marked among them), or one to two sample answers with their meaning where the question is open-form; the record keeps its own register (NFR-2) untouched. | an authority who cannot cheaply parse a design question either stalls, delegates ("you decide") more than they mean to, or rubber-stamps the recommendation — eroding the provenance the stage exists to produce; one example traced through every candidate shape converts an architecture question into a comparable-outcomes question, and the same-example trace is exactly what makes panel alternatives judgeable rather than wallpaper (UB-7, G3) (operator ruling 2026-07-30) | Must | med | across the fixture corpus, 100% of authority-facing question turns carry the form — plain-language question, example or typed skip, enumerated options each stating its outcome on the turn's example (or sample answers for open-form), recommendation marked; zero turns present a corpus anchor without its inline gloss; zero example-bearing turns where two options share an outcome; on fired-panel fixtures, each presented alternative carries its outcome on the turn's example (EVAL-7) | Eval | J1, FR-3 |

### op: add EVAL

Reason: the form is checkable — element presence and anchor glossing mechanically, example fit, trace quality, and outcome divergence by judgment — and belongs beside EVAL-1's turn-shape thresholds it sharpens.

| ID | Behavior | Eval set / harness | Threshold | Negative examples |
|---|---|---|---|---|
| EVAL-7 | question turns legible in the authority's register — example-anchored comparable options, panel alternatives traced through the same example, no bare anchors (FR-17) | fixture corpus, all interview turns incl. fired-panel and extension fixtures | 100% of turns carry the FR-17 form; zero unglossed corpus anchors; zero example-bearing turns with two options sharing an outcome; fired panels present each alternative's outcome on the turn's example; typed example-skips only where no illustrative case exists (judged at the report); example fidelity — every example presented as looked-up verifies against its named source at the report, and every invented example is marked hypothetical or illustrative (a fabricated-as-real example is a fail) | "which shape should the reader port take — ALT-2 or ALT-3?" with no gloss and no trace; alternatives with trade-off adjectives ("simpler", "more flexible") but no outcomes; each alternative illustrated on a different scenario, defeating comparison; a forced example bolted onto a pure naming turn instead of a typed skip; an invented example presented as a real basis case, or a hypothetical left unmarked |
