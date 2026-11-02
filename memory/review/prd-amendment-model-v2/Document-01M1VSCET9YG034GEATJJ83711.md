# Review: prd-amendment-model-v2 — design-review report

| Field | Value |
|---|---|
| **In one line** | Request changes — the replay-state direction is coherent, but six blocking gaps remain in the Q-state oracle, activation audit, emitted remedies, supersession migration, incident carry-forward, and defect retention. |
| **Relevant-when** | auditing or consuming the review of design:prd-amendment-model-v2; asking why the draft is not yet approvable; tuning tech-design-review calibration |
| **Subject** | design:prd-amendment-model-v2 · shape: none · supersedes: prd-amendment-model · docgate-v1 |
| **Moment** | supersession |
| **Reviewer** | tech-design-review · guide 2026-07 (`.pyxis/authoring/tech-design-review.md`) |
| **Date(s)** | 2026-08-20 |
| **Verdict** | request-changes *(round 1; advisory-period reviewer, but the design itself remains operator-unapproved)* |
| **Tracking** | PR #1097 |
| **Template version** | design-review-report/v1 |

## Round 1

Four independent lenses reviewed the post-lint successor, both supersession
targets, the merged docgate contract, governing principles, and relevant
incidents. Adversarial survival merged the duplicate Q-state candidates and
dropped the entry-atomicity candidate: the top-level Decision already binds an
entry-wide transaction, so the missing dedicated fixture is verification
detail rather than a contradictory design. The feasibility-completeness lens
found no additional candidate after the same screening.

### FND-1 — Q answer-state congruence has no independent Markdown-side oracle

- **Severity**: blocking (CAL-1 · CAL-3 — the binding byte-inventory decision is internally contradictory and cannot be tested as written)
- **Lens**: coherence · contract-quality
- **Evidence**: design:§3.5 says the Markdown inventory derives Q answer state from serialized `effective.md`, while design:§3.4 and §3.7 make the typed boolean authoritative, forbid interpreting Status prose, and strip checkpoint metadata before emitting the projection — vs — prd:docgate/FR-42 permits arbitrary Status prose and forbids deriving routing state from it; prd:docgate/FR-43 puts the required boolean in fold state, checkpoint metadata, and the source map, not in effective Markdown.
- **Acceptance condition**: either add one canonical, non-prose Q-state representation to the Markdown face or narrow two-face congruence so Q answer state is verified by an independent carrier. A fixture with byte-identical rendered Q rows and opposite typed booleans must obtain the intended red/green verdict without reading fold state or interpreting Status.
- **Disposition**: pending — author revision required before approval

### FND-2 — The activation-golden audit has no coherent revision model

- **Severity**: blocking (CAL-3 — A-9 is load-bearing and untestable under the current revision rules)
- **Lens**: contract-quality
- **Evidence**: design:§3.8 requires one immutable activation revision and ledger fingerprints, a byte-comparison against independently reconstructed committed artifacts, ordinary amendments that never regenerate the golden, and repair proposals that delete exact mismatch multiplicity — vs — prd:docgate/M3 and DOD-7 require a congruent amendment with a new ledger fingerprint to stay green, an exact later repair to shrink debt, and any new mismatch to red.
- **Acceptance condition**: state separately which revision proves immutable activation provenance and which revision supplies the current mismatch inventory. A fixture must cover `f0 + m1` at activation, a congruent `f1 + m1` with no golden edit, a repair `f2` with exact `m1` deletion, and a new `m2` that reds.
- **Disposition**: pending — author revision required before approval

### FND-3 — Serialized-emitter defects have no executable remedy branch

- **Severity**: blocking (CAL-1 — the artifact-observation decision conflicts with its mandatory repair router)
- **Lens**: coherence
- **Evidence**: design:§3.5 inventories serialized bytes specifically so Markdown-annotation and JSON-emission bugs remain observable; design:§3.6 nevertheless sends every mismatch to §3.4, whose routes are author-owned ledger operations only — vs — prd:docgate/UB-6 requires every emitted remedy to pass the next validation layer.
- **Acceptance condition**: distinguish an invalid ledger transition from a successful transition whose serializer or source-map emitter disagrees. An injected emitter-only mismatch must name a generator/gate repair and regeneration, while ledger-caused defects continue to receive the immediately-before-state operation route.
- **Disposition**: pending — author revision required before approval

### FND-4 — Foundational supersession strands 25 live Builds-on edges

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: design:header supersedes both foundational targets and design:R6 accepts newly surfaced stale edges without closing them; the regenerated design index marks 25 live direct dependents `stale-builds-on` — vs — prd:contract-gate/FR-14 leaves every uncovered stale dependency edge blocking, and incident:I6 records that a stale-builds-on flag is a planning blocker whose legal repair is a successor or an exact governed compatibility disposition. The Serves side is clean: the successor carries every requirement served by docgate-v1 and adds the new amendment requirements, while prd-amendment-model served none, so this supersession creates no orphaned requirement and no stale-Serves repair obligation.
- **Acceptance condition**: regenerate the index and evaluate every newly stale `Builds-on` edge through the currency contract. Each dependency edge must be covered by a live dependent successor or one current, independently approved `currency-disposition/v1` record mapping its exact consumed bindings to this successor; zero uncovered dependency edge remains. No Serves-set migration is required.
- **Disposition**: pending — operator-visible migration decision required before approval

### FND-5 — The complete successor silently carries a downstream-invalidation model disproved by I9

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: design:Scope calls this the complete amendment model and design:§2 carries all predecessor downstream-invalidation rules forward; target design:prd-amendment-model §12 treats affected IDs and fingerprints as sufficient — vs — incident:I9 records that requirement retirement can make immutable delivered-plan traces unresolvable and requires plan-aware blast radius plus an explicit stale-trace or delivered-plan policy. The successor neither incorporates that lesson nor drops the target rule with a reason.
- **Acceptance condition**: explicitly disposition I9. Either design and test the plan-aware invalidation behavior for active and delivered frozen plans, or narrow the supersession claim and name an approved live design that owns the omitted behavior.
- **Disposition**: pending — author revision or upstream amendment required before approval

### FND-6 — Congruence defect output violates AP-28 while claiming no deviation

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: design:§3.10 emits failures only on stderr and `--defects-json`, and design:Principles says `Deviation: none` — vs — AP-28 requires a gate's signal-bearing defect report to be retained in git as an immutable append-only event record, while a clean pass materializes nothing.
- **Acceptance condition**: specify the read-only gate's closed dual-face defect record and the host-owned immutable git sink, with a red fixture retaining the checked revision and full mismatch key and a green fixture emitting no record; or record an explicit operator-approved AP-28 deviation and its rationale.
- **Disposition**: pending — author revision or operator ruling required before approval

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-08-20 |

## Escalation brief

none — no disposition has been declined; the six findings await author or
operator disposition in the draft PR.

## Measurements

- **Rounds**: 1
- **Findings**: 6 blocking · 0 advisory
- **Dispositions**: 0 applied · 0 declined · 0 withheld-as-violation · 6 pending

## Overlay corrections

none
