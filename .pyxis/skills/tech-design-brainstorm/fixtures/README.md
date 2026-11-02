# tech-design-brainstorm — eval fixture corpus

The scripted-authority harness for the PRD's §5.9 evals (EVAL-1..7) and the
design's A-2/A-6 rows. Runs are on demand; results append to a dated eval
report here. The encoding mirrors the stage-1 corpus (each discipline owns
its own fixtures — NG-3): a fixture is one file with **`## Basis + design
questions`** (the input), **`## Scripted authority`** (persona + topic-keyed
answer bank + default rule — the run must not invent answers beyond it), and
**`## Expected record`** (mechanical assertions + the transcript evidence to
quote where judgment is needed).

## Runner protocol

1. Run `/tech-design-brainstorm` in a fresh session with the fixture's basis
   and effort; answer only from the bank. Record slug: the fixture name
   (+ `-tech-design`).
2. Let the session end through its gate (or the scripted stop).
3. Walk the template's Appendix A (and B where ready-for-design) against the
   produced record; evaluate the fixture's assertions.
4. Append per-eval results: EVAL-1 (single-question + recommendation rate),
   EVAL-2 (unanchored/planted rulings — **any is sev-1**), EVAL-3
   (doc-answerable questions asked), EVAL-4 (fabricated directions; silent
   patches; fog typing), EVAL-5 (run `/tech-design-new` on a ready-for-design
   record: decision-questions + §2/§4 citations), EVAL-6 (seam-rule and
   override discrimination; audit-note presence), EVAL-7 (question-turn
   form: example or typed skip, options with outcomes on the same example,
   panel alternatives traced through one flow, no bare anchors).

## Corpus

| Fixture | Exercises | Primary rows |
|---|---|---|
| `golden.md` | new-seam DQ → panel fires; extension DQ → skips; ready-for-design | EVAL-1/2/3/6 · A-2 · FR-1..13 |
| `extension.md` | extension-only session — panel skipped throughout, audited | EVAL-6 · FR-8 |
| `overrides.md` | forced-on-extension + waived-on-new-seam, reasons logged | EVAL-6 · FR-9 |
| `panel-quality.md` | a design question whose obvious shapes collapse — the near-duplicate redo | UB-7 · G3 · FR-8 |
| `bounce.md` | requirement hole → needs-more-product, anchors + route named | FR-10 · UB-6-analog · EVAL-4 |
| `silence.md` | authority goes quiet mid-session | UB-1 · EVAL-2 |
| `contradiction.md` | answer conflicts with an earlier ruling | UB-2 |
| `covered.md` | a live design already decides the question | UB-4 · FR-2 |
| `false-cover.md` | a seeded live design only *claims* to decide it; the authority declines | UB-4 · FR-2 · ABUSE-2 |
| `refusals.md` | unmerged document-PRD basis + basis-free request | FR-1 · UB-5 |
| `injection.md` | instructions planted in the basis/notes | ABUSE-1/2 · EVAL-2 |
| `under-specified.md` | direction-level gaps; blocking fog | FR-11 · EVAL-4 |
| `resume.md` | killed mid-grill; resumed from the record | FR-14 |
| `reopen.md` | new session on an effort with a frozen record | DINV-2 · A-6 |
| `question-form.md` | every turn in the authority's register: example-anchored options, panel traced through one flow, typed skips, no bare anchors | EVAL-7 · FR-17 |
| `glossary.md` | the authority overloads a term the glossary owns — challenged in the turn that answers it, resolved as a candidate entry, and the shared glossary left alone | FR-16 · FR-15 |

A fixture whose `## Basis + design questions` names something the runner must
put in place before the session opens — `false-cover.md`'s seeded design and
its index row — says so in an editorial `*(…)*` aside. Asides are notes to the
runner, never material for the session: they are stripped before the opening
prompt is composed, because a note saying what the answer is would test
nothing.

`records/` holds static records: `fleet-feed-tech-design.md` (the golden
record, produced by the first scripted run) and six seeded-defect records —
each must turn the structure checklist red at exactly its intended row(s)
(design A-2): `defect-unanchored-decision.md`, `defect-missing-panel-log.md`,
`defect-single-alt-fired-panel.md`, `defect-unscored-alt.md`,
`defect-untyped-fog.md`, `defect-double-gate.md`.

A seeded-defect record's **gate row is scaffolding, not a class claim**: it
exists so Appendix A's own gate rows have a table to walk. Four of these
records record `ready-for-design` while the coverage walk turns rows red on
them — a pair DINV-3 forbids a real record of this class — and their gate
says so itself, by reading `pass — (fixture; not walked)` where a real record
carries its coverage result. That marker is load-bearing, not decoration:
`TestStructureDefectGateCellsAreScaffolding` fails any defect record that
writes a genuine coverage claim its own walk contradicts.

Two further records seed the **coverage** checklist rather than the structure
one: `coverage-unmapped-seams.md` (the panel called `DQ-1` an opened seam and
the seam map was never filled in) and `coverage-no-boundaries.md` (the session
ruled nothing out). Both are structurally spotless and both carry only
`non-blocking` fog, so the gate's zero-blocking-fog condition is met — the
unmet coverage row is the only thing refusing them `ready-for-design`, which is
the case a walk that checks fog alone would wave through. Every record's rows
on both checklists **were** pinned by a conformance test that has been removed
along with the rest of the stage-3 e2e suite; until that suite is regenerated,
these records are inputs no automated walk reads, and a record added here is
checked by review alone.

Fixture filenames are fixture-scoped; the `<slug>-tech-design.md` naming rule
applies in `docs/brainstorms/`, and the golden record's name demonstrates it.
Static records live here, never in `docs/brainstorms/`.
