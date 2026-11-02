# <Feature> — prototype findings record

| Field | Value |
|---|---|
| **In one line** | <the probe and its main learning, for orientation scans> |
| **Relevant-when** | <load when authoring or reviewing the draft contract for this feature> |
| **Idea** | <the provider's words, or `problem:<slug>` for an Argus-driven idea> |
| **Prototype** | route: <configured or ambient route> · branch: `prototype/<feature>` · base: `<commit>` · head: `<commit>` |
| **Date** | <YYYY-MM-DD> |
| **Outcome** | <ran or did not run> |
| **Template version** | prototype-record/v1 |

## 0. How to use this template

*Instantiate this template as the next unused
`<draft-root>/<feature>/brainstorm/NNN.md` working file on the open draft
branch. Keep it with the draft's other numbered brainstorm files. Delete this
section and all guidance italics when instantiating. The draft compiler
captures the file as a brainstorm document; findings use the existing
`brainstorm:<feature>#L-n` citation grammar.*

*The record carries the provider's idea, what the separate run built and left
out, and verified findings. Do not copy prototype code into this record. Do
not include credentials, tokens, or unrelated private data from the run.*

*`L-n` anchors are dense across this feature's prototype records, never
renumbered or reused. A run that learned nothing uses `No findings: <why>` in
place of finding blocks. A completed run that did not reach a working state
has `Outcome: did not run` and at least one finding naming what stopped it.*

## What was built

<the smallest path that ran, or `none`>

## What was left out

<the behavior intentionally not attempted, or `none`>

## Findings

### L-1 — <short finding>

- **Kind**: technical challenge | product conflict
- **Tried**: <what the prototype did>
- **Observed**: <what happened, with the file, command, or output that supports it>
- **Answer**: open

*Answer is `open`, a citation to the PRD row, tech-spec decision, or ruled
problem that handles the finding, or `set aside — <reason>`. Examples of
answer citations are `prd:<feature>/FR-1`, `prd:<feature>/P-1`, and
`design:<feature>#D-1`. An `L-n` citation in another draft file is
`brainstorm:<feature>#L-n`.*

## Structure checklist

- [ ] Every header field is filled; the template version is `prototype-record/v1`.
- [ ] What was built and what was left out are stated, including `none` where appropriate.
- [ ] Each finding has a dense, never-reused `L-n`, one allowed kind, what was tried, and what was observed with supporting evidence.
- [ ] A completed run with no findings says `No findings:` and why; `did not run` names the obstacle in at least one finding.
- [ ] Each Answer is `open`, a resolvable contract citation, or `set aside — <reason>`.
- [ ] The record contains no credentials, tokens, or prototype source code.
