# docgate defect catalog — every reason, its remedy (the runbook)

PRD §6.4: every defect names verb, file, op, reason, remedy — in human text
on stderr and as one JSON line on `--defects-json`. This catalog lists the
reason families; the remedy printed with each defect is authoritative.

| Verb | Reason (family) | Remedy |
|---|---|---|
| fold | document does not start with an H1 title | instantiate from prd-template/v1 |
| fold | Template version is not the pinned one | record the template version the document instantiates |
| fold | duplicate ID | IDs are never reused; pick the next free number |
| fold | row sits in a table of another kind | one table, one kind — move the row |
| fold | dangling target (supersede/retire/rescope/answer) | target an existing live unit |
| fold | target already superseded/retired | a unit is changed at most once; supersede the successor instead |
| fold | ID reuse in a payload | fresh IDs only; pick the next free number |
| fold | payload columns do not match the kind's schema | match prd-template/v1 column names and order exactly (UB-2) |
| fold | operation carries no payload row | payloads are complete and resolved |
| fold | unknown add target kind | name a receiving table kind (FR, NR, UB, NFR, DOD, EVAL, EVT, Q) |
| fold | rescope targets NG-n/WR-n only | use supersede for requirement rows |
| fold | answer payload missing a typed field | Answer, Evidence, Decided-by, Effects — all four |
| fold | section §x.y not found | target an existing numbered section |
| fold | empty ledger | a ledger starts at 000-base.md |
| validate-amendments | no envelope header table / missing or unknown envelope field | the envelope is closed: all seven fields, exact names |
| validate-amendments | Status is not Approved | an amendment merges only with Status: Approved |
| validate-amendments | unknown evidence class | one of the nine typed classes, `class:reference` |
| validate-amendments | Blast radius does not parse | `tier: <t>; touches: <IDs>; reason: <why>` |
| validate-amendments | declared blast radius understates computed | declared may exceed computed, never understate |
| validate-amendments | unknown operation verb / no operations | closed vocabulary: add, supersede, retire, answer, rescope |
| lint-structure | missing 000-base.md | a ledger starts at its base |
| lint-structure | filename is not NNN-<slug>.md | dense three-digit numbering, kebab-case slug |
| lint-structure | duplicate sequence / numbering gap | renumber onto the next free number; the sequence is dense |
| verify-generated | projection artifact missing | run `docgate fold` and commit the regenerated artifacts |
| verify-generated | committed artifact differs from the recomputed fold | never hand-edit generated artifacts: fix the op or the fold, refold, recommit |
| check-immutable | ledger entry modified/deleted/renamed vs merge base | revert; evolve the contract with a new amendment instead |
| verify-rebase | checkpoint not semantically identical to the preceding projection | a rebase is a no-op snapshot: regenerate it, changing nothing |
