# Review: prd:docgate/018 — review report

| Field | Value |
|---|---|
| **In one line** | Approve, round 2 plus operator overlay — the contract fixes exact congruence equality and baseline adoption, excludes the §0 subtree while preserving merged-base inference, preserves attribution, and gives every lineage-debt family an executable repair. |
| **Relevant-when** | auditing or consuming the review of docgate amendment 018; asking why its congruence contract was revised; tuning prd-review calibration against a hard-gate adoption amendment |
| **Subject** | `prd:docgate/018` |
| **Moment** | amendment |
| **Reviewer** | prd-review · `.pyxis/authoring/prd-review.md` (2026-08-20 run) |
| **Date(s)** | 2026-08-20 |
| **Verdict** | approve |
| **Tracking** | PR #1093 |
| **Template version** | review-report/v1 |

## Round 1

### FND-1 — Determinism could satisfy the old metric while projection and source map disagreed

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: round-1 FR-32/FR-33 traced the new congruence behavior to G1, while the folded G1/M1 chain measured only deterministic byte-identical output — vs — product vision §1/§4 requires a machine-decidable definition of done and AP-13 requires success to be a verified artifact.
- **Acceptance condition**: add a congruence-specific goal and independently fed metric, update the ship rule to use them, and trace the new behavior to that chain.
- **Disposition**: applied — G3/M3 now measure accepted after-only mismatch independently through required-check history plus a committed corpus audit; §1, §3.3, §5.7, and §6.7 carry the checkpoint and ship rule.

### FND-2 — The hard gate had no coherent author-to-release path

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: round-1 FR-32 introduced `check-projection-congruence`, FR-33 named a two-entry repair, and DOD-7 required wiring, while the folded scope, F1, CI/pre-push interface, delivery phases, release criteria, rollout, launch readiness, and §8 still described only the old gate set — vs — the PRD's own J1/J3, F1, §5.5, §6, and §8 readiness chain.
- **Acceptance condition**: join the new behavior and repair into the effective scope, ordinary and repair flows, enforcement interfaces, delivery/release/rollout criteria, launch checklist, and readiness mappings.
- **Disposition**: applied — §1, §4.1–§4.4, §5.5, §6.1, §6.4–§6.7, and §8 now carry the complete contract/design/fixture/wire path and the two-proposal repair flow.

### FND-3 — “Worsened disagreement” was not a decidable gate predicate

- **Severity**: blocking (CAL-3)
- **Lens**: contract-quality
- **Evidence**: round-1 FR-33 grandfathered an “exact baseline multiset” but failed “new or worsened” disagreement without defining whether a changed value, field movement, or class exchange at constant count was new — vs — product vision §1/§4 stage 6 requires ambiguous contracts to bounce and AP-14 requires observable acceptance.
- **Acceptance condition**: define a canonical mismatch identity and multiset subtraction, with fixtures for unchanged, replaced, moved, increased, decreased, and fully repaired transitions.
- **Disposition**: applied — FR-34 defines the five-part canonical key and multiplicity rule; its fit criterion fixes every named transition's red/green result.

### FND-4 — The corpus-debt inventory had no completeness oracle

- **Severity**: blocking (CAL-3)
- **Lens**: contract-quality
- **Evidence**: round-1 FR-33 and DOD-7 required only a “known” inventory and audit output, so an empty or partial report could satisfy the literal evidence request — vs — AP-13/AP-14 require a verifiable artifact and observed acceptance, and AP-28 retains improvement signal in git.
- **Acceptance condition**: pin the complete merge-base inventory by reviewed golden or independently asserted exhaustive comparison, and make omission itself red.
- **Disposition**: applied — DOD-7 requires a committed sorted golden with merge-base fingerprint and full canonical records, independently reconstructed and byte-compared; omitted-record and hidden-mismatch fixtures must red.

### FND-5 — The two Must rows combined independently testable obligations

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: round-1 FR-32 combined entry attribution, mismatch domain, parser/§0 scope, and exact-revision isolation; FR-33 combined baseline subtraction, restatement behavior, and repair — vs — the PRD template writing rules require atomic, ID'd, measurable requirements.
- **Acceptance condition**: split independent observables into separate requirement rows or move already-protected behavior to a dedicated NR/UB row.
- **Disposition**: applied — FR-32..FR-37 separate mismatch detection, exact reads, baseline identity, checkpoint congruence, checkpoint-established targetability, and unit recognition; UB-4/UB-5 and NR-4 own their distinct failure/non-regression behavior.

### FND-6 — The named rebase repair could itself introduce lineage disagreement

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: round-1 FR-33 directed lineage-less debt through a semantically identical rebase, while FR-32 compared human and machine lineage but did not evaluate checkpoints or define permitted compaction — vs — prd-amendment-model §4/§6 requires two lineage faces and permits rebase compaction, and incident I6 requires every named remedy to be executable under its enforcement layer.
- **Acceptance condition**: define checkpoint congruence and permitted lineage re-anchoring on both faces, prove a checkpoint with annotated and lineage-less rows introduces no mismatch, prove the next typed operation green, and cite incident I6.
- **Disposition**: applied — Learned-from now cites I6; FR-35 checks checkpoint compaction with both faces re-anchored, FR-36 proves later targetability, and F2 plus DOD-7 require the full checkpoint→typed-operation proof.

## Round 2

Scope lock: only FND-1..FND-6, their revised spans, and regressions introduced inside those spans were re-reviewed. FND-1, FND-2, and FND-4 closed without further change; the completeness-critic found no survivor in the revised end-to-end matrix.

### FND-7 — Canonical mismatch values were still undefined

- **Severity**: blocking (CAL-3)
- **Lens**: contract-quality
- **Evidence**: revised FR-34 keyed the adoption multiset by “canonical value” while revised §6.1 still deferred canonicalization to design — vs — AP-14 requires observable acceptance and product vision §4 requires ambiguous contracts to bounce before design.
- **Acceptance condition**: define exact equality for projected cells, source-map strings, booleans, and absence; enumerate every permitted normalization and nearest disallowed variant; make design implement rather than decide that behavior.
- **Disposition**: applied — FR-38 fixes typed canonical JSON values, the exact ledger/source-map decoding boundary, forbidden normalizations, structural absence, and red/green fixtures; §6.1 now treats equality as fixed input.

### FND-8 — The repair promised ID operations outside the closed target grammar

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: revised FR-36 promised later typed operations for R, M, NG, WR, and ABUSE rows — vs — prd-amendment-model §3.2 and prd-template/v2 Appendix A reserve G/J/F/M/ABUSE/R for whole-section supersede and give Q, NG/WR, and the unit-target families distinct verbs.
- **Acceptance condition**: keep the operation vocabulary closed, enumerate an authorized repair for every prefix and relevant Q state, prove each end to end, and require the superseding design to reconcile its target grammar and section-reparse semantics.
- **Disposition**: applied — FR-39 enumerates rebase→unit-operation and direct section-repair branches, UB-6 owns prefix/state-correct diagnostics, F2/F3 expose both paths, and the next-design dependency names the grammar seam explicitly.

### FND-9 — Checkpoint lineage and later targetability were still combined

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: revised FR-36 required both complete checkpoint-origin state materialization and legal later operation targeting — vs — the prd-template/v2 singularity rule requires one independently falsifiable obligation per requirement row.
- **Acceptance condition**: keep checkpoint materialization in one requirement and move repair acceptance and illegal-operation diagnostics to their own rows.
- **Disposition**: applied — FR-36 now owns only complete checkpoint-origin fold state and source-map materialization; FR-39 and UB-6 separately own repair acceptance and refusal.

**Round-2 verdict:** approve. The final closure check found no surviving blocker, advisory, or revision-caused regression within the locked scope.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-08-20 |
| 2 | approve | 2026-08-20 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 7 blocking · 2 advisory
- **Dispositions**: 9 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

- 2026-08-20 · operator `@mdarabi` (PR #1093 review) — Measured FR-37's exact-header rule against origin/main and found that it would remove 132 live source-map records across 13 frozen PRDs, including the amendment's own UB rows, while all six reproduced phantom units were inside §0. The approved subject now preserves merged-base row-kind inference and excludes §0 by full section ancestry; its corpus fit proves zero non-§0 live record disappears. The same overlay closes the two adjacent ambiguities: F3 now requires the fold to write the repairing amendment's visible ID annotation and identical machine origin, and DOD-7/§5.7 pin the debt golden to one activation revision and fingerprint, forbid ordinary regeneration, permit exact repair-only deletion, and reserve regeneration for a dedicated reviewed gate-fix while unwired. The two recorded rounds and verdict history remain frozen; under the review protocol this post-approve correction replaces a prohibited round 3, and the verdict remains `approve`.
- 2026-08-20 · operator `@mdarabi` (PR #1093 follow-up) — Narrowed the first overlay's F3 lineage sentence: refresh and annotation apply only to a canonically changed row with no immediately-before live state or source-map lineage. Canonically unchanged rows retain exact projected bytes/state/lineage, preserving FR-34's byte-identical-restatement green path; changed lineage-bearing rows remain red under FR-32 and use the closed grammar's `retire`+fresh-`add` repair instead of self-healing. FR-37's 95/26/5 counts moved to explicitly dated as-of-review rationale, leaving zero lost non-§0 live records as the durable fit criterion. Verdict remains `approve`.
- 2026-08-20 · operator `@mdarabi` (PR #1093 follow-up) — Rejected the second overlay's `retire`+fresh-`add` replacement branch after folding both shapes: it erased `supersedes`/`superseded_by`, contradicted merged ABUSE/EVT supersession precedent, and could send a cited row to a structurally forbidden retirement. FR-39 now requires lineage-preserving `supersede <ID>` with a fresh same-prefix row for live replacements, explicitly expanding the superseding design's target grammar while retaining `answer` for open Q and `rescope` for NG/WR; `retire` means removal only. Independent unchanged-row preservation moved from FR-39 into atomic FR-40. The two recorded rounds and verdict history remain frozen; verdict remains `approve`.
- 2026-08-20 · operator `@mdarabi` (PR #1093 follow-up) — Closed two predicates the prior correction assumed: FR-41 now defines “registered/addressable prefix” by the addressable table-schema registry, not the broader lexical token registry, deliberately leaving real F1..F5 flow prose non-addressable and outside DOD-7; FR-42 defines Q answered state solely by a successful `answer` op in the replayed prefix, never by the free-text Status cell. FR-39/F2/F3 now consume those predicates, drop the impossible F and prose-resolved-Q section branches, and define retirement citations as resolvable references from live DOD `Source` or live `Traces to` cells. The two recorded rounds and verdict history remain frozen; verdict remains `approve`.
- 2026-08-20 · operator `@mdarabi` (PR #1093 follow-up) — Reproduced answer-state loss across a real rebase boundary: the establishing `answer` op falls outside the replay window, while Status is non-authoritative and the checkpoint/source map had no typed carrier. FR-42 now reads either a validated replay-base `question_answered: true` or a successful post-base answer op. Atomic FR-43 requires every checkpoint to carry, restore, and project a required per-Q boolean and extends `verify-rebase` to compare it; scanning compacted pre-checkpoint history is explicitly rejected. All delivery/readiness ranges now close through FR-43. The two recorded rounds and verdict history remain frozen; verdict remains `approve`.
