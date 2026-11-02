# Review: prd:skill-command-prefix/000 — review report

| Field | Value |
|---|---|
| **In one line** | Approved with three advisories applied in-session — FR-5 glossary timing vs prd-new T-n routing; FR-3 underspecified live-surface set; living-rollup inventory omitted from routing surfaces. |
| **Relevant-when** | auditing or consuming the review of prd:skill-command-prefix/000; asking why it was approved; tuning prd-review calibration against real findings |
| **Subject** | prd:skill-command-prefix/000 |
| **Moment** | new-base |
| **Reviewer** | prd-review · `.pyxis/authoring/prd-review.md` (run 2026-07-29) |
| **Date(s)** | 2026-07-29 |
| **Verdict** | approve |
| **Tracking** | agenticlab-ai/pyxis#808 — the subject's introducing PR |
| **Template version** | review-report/v1 |

## Round 1

### FND-1 — FR-5 ties glossary exclusively to the closing PR, conflicting with prd-new T-n routing

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: §5.1 FR-5 — "When the closing PR lands, the glossary shall distinguish…" — vs — `.pyxis/authoring/prd-new.md` ("Route `T-n` candidate glossary terms into `docs/wiki/glossary.md` … in the same introducing PR") and the subject's own Artifacts/`brainstorm:skill-command-prefix#T-1`. As written, FR-5's "when" clause makes an earlier correct T-n landing look like a contract miss, and M1(e) inherits the ambiguity.
- **Acceptance condition**: FR-5 (and §6.6) require the persona/skill split present by the M1 checkpoint, allowing the candidate on the PRD introducing PR and confirmation of `/pyxis-argus` on the closing PR.
- **Disposition**: applied — FR-5 and §6.6 rewritten for M1-by-when with explicit T-n-early / closing-confirm split.

### FND-2 — FR-3 says "live routing surfaces in scope" without naming the set

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: §5.1 FR-3 — "live routing surfaces in scope" — vs — `brainstorm:skill-command-prefix#D-2`, which enumerated pipeline map, help fixtures/tests, and editable wiki invocation pointers. "In scope" leaves the implementer to re-derive D-2; M1(b) repeated the same underspecification.
- **Acceptance condition**: FR-3 and M1(b) name the concrete surface set (at least D-2's enumeration).
- **Disposition**: applied — FR-3/M1/§4.2 enumerate the set; living rollup added per FND-3.

### FND-3 — Living rollup agent-surface inventory omitted from routing-surface coverage

- **Severity**: advisory (CAL-4)
- **Lens**: completeness-critic
- **Evidence**: §4.2/FR-3 name pipeline map, help fixtures, wiki pointers — vs — `docs/techspec.md` § Agent surface (the living rollup that inventories skills) and AGENTS.md rule 3 (living rollups stay current as shipped structure changes). A train that renames every skill but leaves the rollup on unprefixed names would pass FR-3 as written while teaching the old identity in the shipped inventory — the same class of miss as ecosystem-help's NR-1/living-rollup finding.
- **Acceptance condition**: FR-3 (and launch-readiness) include the living rollup's agent-surface inventory among surfaces that must cite only prefixed forms by M1.
- **Disposition**: applied — FR-3, §4.2, and §6.6 include the living rollup agent-surface inventory.

## Round 2

Scope-locked confirmation after applying FND-1..3 (changed spans: FR-3, FR-5, M1, §4.2, §6.6, §7.4). All three acceptance conditions are met; no regression found within the changed spans — FR-3 remains EARS-shaped with an Inspection fit criterion naming a closed surface set; FR-5 no longer contradicts prd-new T-n routing; M1(b) points at FR-3. No new findings on unchanged content (scope lock).

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | approve-with-advisories | 2026-07-29 |
| 2 | approve | 2026-07-29 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 0 blocking · 3 advisory
- **Dispositions**: 3 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

none
