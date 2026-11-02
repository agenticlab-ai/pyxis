# Review: prd:ecosystem-help/000 — review report

| Field | Value |
|---|---|
| **In one line** | Approved after one revision round — one blocking coherence defect (NR-1's fit criterion forbade the living-rollup updates the capability convention requires) and two advisories (FR-1 scope/fit mismatch; no unbuilt-stage honesty behavior), all applied. |
| **Relevant-when** | auditing or consuming the review of prd:ecosystem-help/000; asking why it was approved; tuning prd-review calibration against real findings |
| **Subject** | prd:ecosystem-help/000 |
| **Moment** | new-base |
| **Reviewer** | prd-review · `.pyxis/authoring/prd-review.md` (run 2026-07-23) |
| **Date(s)** | 2026-07-23 |
| **Verdict** | approve |
| **Tracking** | agenticlab-ai/pyxis#630 — the subject's introducing PR |
| **Template version** | review-report/v1 |

## Round 1

### FND-1 — NR-1's fit criterion contradicts the living-rollup convention

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: §5.1 NR-1 fit — "the implementation diff outside the new skill's own files is limited to vendor pointers and an additive pipeline-map row" — vs — the repo's shipped-capability convention (AGENTS.md rule 3: living rollups are "kept current as shipped structure changes") and the subject's own §6.6, which schedules doc updates at implementation landing. An implementation PR that ships the capability *must* update the living rollup(s); the fit criterion as written counts that required change as a violation, making DOD-5 unsatisfiable by any conforming implementation.
- **Acceptance condition**: NR-1's fit criterion enumerates the legitimately-touched surfaces, including the living-rollup updates the convention requires — or restates the bound as "no edits to existing skills/guides/map content beyond the additive row."
- **Disposition**: applied — fit criterion widened to "vendor pointers, an additive pipeline-map row, and the living-rollup updates the shipped-capability convention requires."

### FND-2 — FR-1 promises "every capability," its fit criterion measures only skills

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: §5.1 FR-1 — "every capability on the installed surface" — vs — its own fit criterion, "every skill present on the installed surface appears exactly once." "Capability" is wider than "skill" (the shipped map carries stages with non-skill mechanisms and unbuilt rows); whether those belong in the catalog is left to the implementer's guess — the ambiguity Appendix A's rules exist to prevent.
- **Acceptance condition**: the requirement's noun and its fit criterion name the same set — either scope the catalog to skills plus the map's stage rows with an explicit rule, or widen the fit criterion to match "capability."
- **Disposition**: applied — FR-1 now names the set precisely (every skill on the installed surface + every stage row of the shipped map, each with built-status) and the fit criterion measures exactly that.

### FND-3 — No stated behavior for the built-vs-unbuilt distinction the shipped map carries

- **Severity**: advisory (CAL-4)
- **Lens**: completeness-critic
- **Evidence**: §4.3/§5.2 — failure coverage stops at missing surfaces (UB-1) and unroutable problems (UB-2) — vs — the shipped pipeline map itself, which carries not-built stage rows by design. A status-blind derivation could catalog or route a newcomer to a stage with no invocable mechanism on their install — a dead end that fails M1's "first productive action" while every stated fit criterion passes (the map *is* a resolvable live source, so the §3.3 purity guardrail never fires).
- **Acceptance condition**: a requirement states that derived entries carry the map's built-status and that routing never recommends a command for an unbuilt stage (the honest alternative: name the stage and its status).
- **Disposition**: applied — FR-1 gained built-status carriage; UB-3 added (unbuilt stages presented with status, never as commands; nearest built entry offered); DOD-2 and EVAL-1 extended to cover it.

## Round 2

Scope-locked re-review of the three named defects and the changed spans (FR-1, NR-1, UB-3, DOD-2, EVAL-1, §7.4, §8 risk note). All three acceptance conditions are met by the revision; no regression found within the changed spans — the new UB-3 row is EARS-shaped with a verifiable fit criterion and lands in DOD-2's blocking aggregation, and the refold is clean (docgate gate green at `sha256:ecfaa1f…`). No new findings on unchanged content (scope lock).

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-07-23 |
| 2 | approve | 2026-07-23 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 1 blocking · 2 advisory
- **Dispositions**: 3 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

none
