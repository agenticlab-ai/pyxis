# help — routing eval corpus

The scenario harness for the PRD's §5.9 eval (`EVAL-1`) and the design's
`A-2` acceptance row (`ecosystem-help-v1`). It proves the one judgment call
in the skill — routing a described problem to its entry (`FR-2`) — and the
honesty edges around it: an unroutable ask refused with the map pointer
(`UB-2`), an unbuilt stage named with its status and never handed back as a
command (`UB-3`), and a missing derivation surface named rather than
fabricated for (`UB-1`).

Runs are **on demand** (the skill ships no runtime; the eval is a scripted
walk of the guide's §3 routing procedure against fixed scenarios). Results
append to a dated `eval-report-*.md` in this directory.

## What the routing bar is (the pass condition)

Derived from the design's `A-2` and plan task T3's acceptance:

- **100% correct entry naming** on the **canonical set** — every scenario is
  routed to the entry the map's "Where a problem enters" rules, its stage
  table, or its cross-cutting list actually names.
- **Zero fabricated commands or stages** on **any** set — the skill never
  invents a `/command` or a stage absent from the surfaces read.
- **Every unbuilt-stage row** states the stage's status (its glyph + the
  legend's meaning) and names the nearest built entry — **never** a command
  for the unbuilt stage itself (`UB-3`).
- **The degraded scenario** shows the answer naming the absent surface
  explicitly and deriving zero entries from it (`UB-1`).

Any fabricated command/stage is a **hard fail** (the routing-honesty
invariant `#DINV-2` the whole skill exists to hold — the analogue of the
sibling brainstorm evals' "any unanchored decision is sev-1").

## The scenario sets (`routing-eval.md`)

| Set | Exercises | Rows |
|---|---|---|
| **Canonical** | one ask per entry class the map/stage-table/cross-cutting/tooling names → its correct entry (`FR-2`, `A-2`) | every entry class |
| **Negative** | a non-pyxis ask → honest "not an entry point" + the map pointer (`UB-2`) | off-domain asks |
| **Unbuilt-stage** | an ask landing on a `⛔`/`◐` stage → status stated, nearest built entry, no command (`UB-3`) | the not-built / partial stages |
| **Degraded** | the catalog with a derivation surface removed → the absence named, nothing fabricated (`UB-1`) | missing wiki · frontmatter-less skill |

## Runner protocol

1. **Recompute the ground truth live.** Read `.pyxis/pipeline.md` (stage
   table + status legend + "Where a problem enters" + cross-cutting) and
   every `.pyxis/skills/*/SKILL.md` frontmatter — the same read set §1 of the
   guide names. The expected-entry column of `routing-eval.md` is asserted
   against *this* live read, never a memory of it: a fixture row whose
   expected entry no longer matches the surface is a fixture bug, caught here.
2. **Walk the guide's §3 (route) / §4 (first steps) procedure** against each
   scenario's ask, naming the entry (or refusal, or unbuilt-status answer)
   it yields.
3. **Score** each row against its expected outcome; a fabricated command or
   stage fails the whole run regardless of the rest.
4. **Append** the per-row results and the two summary counts (canonical
   correct-naming rate; fabrication count across all sets) plus the degraded
   transcript to a dated `eval-report-*.md`.

When the map's stages or the skills tree change, rows may need adding or an
expected entry may shift — update `routing-eval.md` in the same PR and record
a fresh run. The corpus tracks the surface; it is never frozen against it.
