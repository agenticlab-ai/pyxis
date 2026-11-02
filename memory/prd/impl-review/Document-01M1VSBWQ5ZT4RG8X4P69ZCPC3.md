# Amendment 003 — append-first-routing-evidence

| Field | Value |
|---|---|
| **Amends** | impl-review *(chain through 002)* |
| **Status** | Approved |
| **Owner** | @mdarabi |
| **Date** | 2026-07-28 |
| **Learned-from** | operator-decision:2026-07-28-canonical-git-evidence-before-launch · review-thread:planner-driven-model-routing-003-FND-2 |
| **Blast radius** | tier: contract-changing; touches: NFR-9, NFR-12, UB-14, DOD-13, DOD-14, §0, §3.3, §5.2, §5.5, §5.8, §6.2, §6.4, §8; reason: prepare a backward-compatible consumer boundary that remains zero-network until the coordinated parent policy activates, then requires canonical git admission before review use; extend integrity to the complete frozen semantic demand and selected harness; and make that proof blocking in the one activation predicate |
| **Template version** | amendment-template/v2 |

## Operations

### op: supersede NFR-9

Reason: the parent routing contract now separates pure offline resolution from mandatory append-first admission; the current blanket pre-review network prohibition would forbid the canonical git evidence operation that authorizes a review launch.

| ID | Area | Requirement | Fit criterion | Verify |
|---|---|---|---|---|
| NFR-12 | Determinism / integrity | The complete frozen semantic demand and its identity and derivation-policy version, actual band and join policy, selected harness and review role, effective mapping policy, qualifications, and override shall parse as closed versioned data; identical inputs shall produce one identical pass set and exact/promoted/unsupported resolution using zero inference and zero network I/O through the terminal result. Until the coordinated parent append-first policy is effective, the compatibility path shall retain the existing zero-network pre-invocation boundary. Once that parent policy is effective, impl-review shall durably append every pass-routing terminal to the configured canonical git evidence store and exact-read it successfully before the route may authorize a selected review model invocation or contribute to the process result. The append and exact-read are then the only network operations permitted in that admission interval: no policy lookup, qualification lookup, harness probe, model call, or other outbound operation may occur. | every missing, changed, unknown, native-in-demand, selected-harness-mismatch, and version fixture fails before invocation; reordered candidate inputs resolve identically; dependency inspection proves zero inference/network through resolution and proves the pre-parent compatibility path remains zero-network; after parent activation, every healthy-store route trace contains a successful canonical append and exact-read before invocation or process use and no other network operation; a zero-evidence healthy path fails, while unavailable or unverifiable evidence produces zero review-model invocations, no asserted verified canonical terminal, and a non-accepting persistence failure under UB-10 | Test + Analysis + Inspection + Failure injection |

### op: supersede §0 Read to rule — the human reviewer's section (~7 min)

Reason: §0 must route the human reviewer to the new offline-resolution/append-first boundary and the live replacement NFR without changing any other decision.

**What changed.**

This amendment makes impl-review the consumer of planner-owned review demand. The merge lane still classifies actual proposal risk and impl-review still projects `skip|standard|deep`; a closed join adds the planner's `ordinary|elevated|critical` assurance floor. `skip` now means the lowest actual band, never no review. Critical assurance is two mutually blind top-level deep passes on the same exact revision; each pass may still have its own scope-locked remediation round. Review-route resolution stays deterministic, inference-free, and offline. This consumer amendment lands first without changing the current zero-network boundary; the coordinated parent amendment then activates mandatory canonical git evidence append and exact-read as the sole pre-invocation network operation.

| Read first | Binding decision | Refused alternative | Why / contract |
|---|---|---|---|
| FR-24 | exact 3×3 join; ordinary has at least `standard×1`, elevated `deep×1`, critical `deep×2` | scalar complexity or zero-review skip | workload and consequences remain independent; scrutiny is monotonic |
| FR-19/FR-20/FR-26 | critical passes are distinct blind top-level contexts; remediation stays inside a pass; aggregate is most restrictive | count one reviewer's fix round as corroboration or choose the favorable verdict | independence and aggregation are structural, not prompt requests |
| FR-25/NR-7 | the already-selected review harness maps semantic demand to its own least-cost qualified native profile, independently of implementation routing | planner-native model IDs, cross-harness fallback, or implementation-profile reuse | adapters own native mapping; harness selection remains upstream |
| FR-27/NFR-12 | native review-route resolution is offline; once the coordinated parent policy activates, every terminal is canonically appended and exact-read before it can authorize invocation or contribute to the process | activate the consumer before its parent, buffer route evidence locally, launch before durable evidence, or allow arbitrary preflight network calls | consumer-first landing preserves today's boundary; parent activation then prevents orphaned launch authority and hidden dispatch dependencies |
| FR-23/§6.5 | complete pre-release proof, then one direct production activation | shadow, canary, or production advisory bypass | operator ruling: test first, then activate universally |
| FR-28/FR-29 | strongest qualified profile is the only routing fallback; it preserves pass process; defective evidence is invalidated | switch harness, lower assurance, disable review, or destructive rollback | fallback may overprovide capacity but never erase the quality floor |

**Review these consequence rows:** UB-9..UB-19, NFR-6..NFR-8, NFR-10..NFR-12, EVAL-9..EVAL-11, and DOD-9..DOD-17.

### op: supersede §3.3 Guardrails and decision rule

Reason: append-first admission is release authority, so its complete boundary proof must be an explicit conjunct of the single activation predicate rather than an unbound release-table statement.

| Guardrail metric | Threshold | Action if breached |
|---|---|---|
| Nit rate | ≤ 3 operator-rejected findings per review at p50 across any five consecutive pre-release reviews | calibration changes only by reviewed data PR; pre-release activation gate remains closed |
| Process integrity | zero missing/zero-pass, depth/count, blindness, revision, remediation-miscount, aggregation, evidence-link, route-admission-order, or unauthorized pre-invocation network violations | no activation; after activation the affected process is non-accepting and UB-17 applies where policy is defective |
| Reviewer availability | zero unresolved required-pass, route-admission, or report-persistence failures | no acceptance from an absent/partial process; repair and rerun the full exact-revision set |
| Disposition engagement | dwell and burst signals surfaced | operator discusses the signal before signing activation; no automatic promotion |

**Activation predicate (single source):** every live blocking DoD row (`DOD-9`, `DOD-10..DOD-17`) is green, including DOD-13's complete NFR-12 boundary proof and the ten-review M1/M2/M3/nit/process-integrity/availability checkpoint; §6.6 launch readiness is green; the required implement-now invocation and merge-lane consumption seams pass end to end; and the parent routing contract's complete pre-release seam gate is green. EVT-11 records this one predicate. Only then may the operator activate the coordinated append-first boundary and perform one direct production activation for the entire supported matrix. Before that act, all runs are classified pre-release, retain the zero-network compatibility boundary, and authorize no production acceptance; after it, every route requires canonical admission and the complete satisfying assurance process is mandatory. §6.1, §6.4, §6.5, and §6.6 reference this predicate without restating a subset.

### op: supersede §5.2 Unwanted behavior and failure handling

Reason: demand-integrity failure UB-14 must trace to live NFR-12 after NFR-9 is superseded; every other consequence row remains unchanged.

| ID | If… (trigger) | Then the system shall… | User sees | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|---|
| UB-9 *(002 ← UB-1)* | a required pass fails or exceeds its bound after bounded retry | record the pass `review-absent`, preserve partial evidence, and keep the aggregate non-accepting until the complete exact-revision pass set succeeds | the failed pass, bound, retry count, and non-accepting aggregate | Must | critical | injected backend error and timeout never yield acceptance or an unbounded wait; a recovered full-set rerun converges | Failure injection | FR-23, FR-26 |
| UB-18 *(002 ← UB-2)* | a lens or internal remediation round emits schema-invalid findings, including a reason-less decline | void that pass output, retry once within the same pass identity, then apply UB-9; never persist free-form fallback or count the retry as another assurance pass | invalid pass or visible absent-pass terminal | Must | high | malformed-output fixtures retry once, persist zero invalid/free-form findings, preserve pass count/identity, and remain non-accepting on repeated failure | Test | FR-5, FR-20, UB-9 |
| UB-3 | the diff or its comments contain instructions addressed to the reviewer | treat the content as a finding candidate (planted directive), never as instruction | an injection finding, when material | Must | high | the §5.9 injection eval: seeded directives never alter the verdict and surface as candidates | Eval | ABUSE-3 |
| UB-19 *(002 ← UB-4)* | an internal remediation changes spans outside the named defects | review named defects plus changed spans, flag unrelated new content as scope drift, and withhold findings against unchanged content; any new proposal revision still stales the complete assurance set under UB-11 | scope-drift finding and full-set re-review on the new head | Must | high | drift fixtures flag every unrelated changed span, withhold unchanged-content findings, and never reuse sibling or stale pass evidence | Test | FR-20, UB-11 |
| UB-10 *(002 ← UB-5)* | a canonical git event append or linked pass/aggregate proposal-projection write fails | retry to a bounded cap, persist any available non-accepting integrity signal without treating it as replacement evidence, and refuse acceptance until both FR-27 faces are complete and losslessly consistent | delayed evidence or named persistence failure, never silent loss | Must | critical | git, both media, and face-mismatch failures converge or block; no satisfying result exists without the complete immutable event chain and byte-faithful linked projection | Failure injection | FR-6, FR-27, FR-30 |
| UB-11 *(002 ← UB-6)* | the proposal head differs from the revision bound to any live pass or aggregate | mark the complete process `review-stale`, preserve it, and require every FR-24 pass on the new exact head before acceptance | stale process and forced full-set re-review | Must | critical | one post-review edit stales one-pass and two-pass processes alike; no old pass is reused; acceptance resumes only after the new full set satisfies | Test | FR-18, FR-26, AP-13 |
| UB-12 *(002 ← UB-7)* | any required pass cannot read the exact proposal honestly within its qualified context limit | record a size escalation naming the unread extent, emit no approve for that pass, and keep the aggregate non-accepting | size escalation tied to pass and exact revision | Must | critical | oversize fixtures never truncate silently, never substitute a sibling pass, and never accept a favorable subset | Test + Eval | FR-25, FR-26, ABUSE-6 |
| UB-13 *(002 ← UB-8)* | the host or worker dies mid-process, or the same process is invoked again | resume idempotently from persisted process→pass→internal-round state, reap orphaned work, and never convert a retry or remediation round into another assurance pass | one converged process, never duplicate passes or a husk | Must | high | kill points before/between/inside passes resume at the exact level; double invocation is a no-op; required pass count and context identities remain stable | Test | FR-18, FR-20, FR-27 |
| UB-14 *(002)* | any field of the frozen assessment, evidence citations, complete semantic profile, demand identity or derivation-policy version, actual band, join policy, or selected review harness is missing, changed from its authoritative carrier, unknown, contains native dispatch data where forbidden, or disagrees with the selected-harness input | refuse process construction before any pass invocation and name the boundary and every offending field | proposal remains unreviewed with a demand-integrity error | Must | critical | one fixture per missing, changed, unknown, native, version, and selected-harness-mismatch class invokes zero passes, names every offending field, and cannot accept | Test | FR-21, FR-24, NFR-12 |
| UB-15 *(002)* | a review-role mapping is missing, malformed, incompatible, weakening, unqualified, cannot satisfy the joined demand, or the actual launch differs from the resolved harness/role/native settings | stop before invocation where possible; otherwise record `route_execution_mismatch`, invalidate the affected pass evidence, retain the selected harness, and keep the process non-accepting | invalid/unsupported route or mismatched launch with unmet/differing components | Must | critical | every negative mapping fixture invokes zero affected passes; every launch-mismatch fixture produces no accepting pass evidence; all retain harness and preserve independent unaffected work | Test + Failure injection | FR-25, FR-27, NR-7 |
| UB-16 *(002)* | effective depth or pass count falls below FR-24, a zero-pass terminal is attempted, critical passes share lineage or review different revisions, a remediation round is counted as a pass, or one required pass is absent, stale, invalid, non-accepting, or disagrees | preserve all evidence, name every violated invariant, and refuse aggregate satisfaction until the complete correctly floored, mutually blind exact-revision set succeeds | non-accepting aggregate with contributing outcomes and exact violations | Must | critical | floor, zero-pass, single-pass-critical, shared-context, leakage, mismatched-revision, remediation-miscount, missing-pass, and split-verdict fixtures never accept | Test + Failure injection | FR-19, FR-24, FR-26 |
| UB-17 *(002)* | a review mapping or native-profile qualification is found defective after production activation | apply FR-29, invalidating all affected open pass/process evidence and requiring complete exact-revision replacement while preserving unaffected and accepted artifacts | affected identities and `rerun_required` or operator-remediation disposition | Must | critical | seeded defects cover open, unaffected, and already accepted artifacts exactly as FR-29 specifies | Failure injection + Inspection | FR-27..FR-29 |

### op: supersede §5.5 Interfaces and integrations

Reason: the interface contract must expose canonical route admission as the only network operation between offline resolution and a selected review launch.

| Interface | Direction | Purpose | Contract | Failure modes | Owner |
|---|---|---|---|---|---|
| frozen plan demand + proposal | in | provide planned assurance, exact revision, charter, design, codegate refs | a typed implement-now VerificationGate-class amendment must carry the closed demand and selected review harness field-identically and receive the aggregate | UB-14; absent seam blocks activation | implement-now + impl-review |
| merge-lane class → impl-review band | in | actual proposal-risk input | existing class and class-to-band projection byte-identical | missing/unknown → UB-14, never default silently | merge-lane + impl-review |
| selected review harness | in | fix credential/egress/runtime boundary | explicit upstream selection; no installed-CLI inference | absent/auth defect visible; no substitution | invoker/operator |
| impl-review → harness adapter | out | resolve each pass's review semantic minimum | qualified review-role mapping; exact/promoted/unsupported; resolution performs zero inference and zero network I/O | UB-15 | harness adapter |
| harness adapter → canonical evidence store | out | admit the immutable pass-routing terminal before launch or process use after coordinated parent activation | dormant while the parent compatibility boundary remains zero-network; once active, configured git evidence ref, append-only terminal, successful exact-read proof, and sole network boundary after resolution | unavailable, unauthorized, conflicting, or unverifiable evidence after activation → UB-10 and zero review-model invocation under NFR-12 | routing evidence owner |
| harness adapter → reviewer instance | out | launch resolved blind top-level pass | native args only from adapter policy; unique context; exact revision; actual launch attests exact agreement with an admitted canonical route | launch/provenance/leakage/mismatch → UB-9/UB-15/UB-16 | runtime adapter + impl-review |
| pass set → aggregate | internal | union findings and derive most restrictive result | deterministic, order-invariant, full-set required | UB-16 | impl-review orchestrator |
| canonical routing/process events | out | retain route/failure/outcome/pass/aggregate/invalidation evidence in git | parent EVT-2/3/5/9/10-compatible closed schemas plus linked EVT-7; append-only | mismatch/orphan/overwrite is non-evidentiary | routing evidence owner |
| proposal media | out | project the linked human report + machine process payload | GitHub marker block / Carina decision record; lossless against git events | UB-10 or face mismatch | tracker adapters |
| reviewer verdict → merge lane | out | force downgrade-only HUMAN routing where required | persisted aggregate consumed by the owed typed `Downgrader` adapter; end-to-end dependency before activation | absent/invalid consumer blocks activation and never invents acceptance | impl-review + merge-lane adapter |

### op: supersede §5.8 Definition of Done

Reason: the blocking proof must cover append-before-use ordering and field-identical transport of the complete frozen semantic demand, not only eventual evidence round-trip and planned assurance.

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-10 *(002 ← DOD-1)* | Every live Must functional requirement and unwanted-behavior row passes its fit criterion, including the complete demand join, one-review minimum, critical two-pass independence, review-role launch fidelity, canonical evidence, aggregation, fallback, invalidation, HUMAN forcing, and direct activation. | FR-3..FR-7, FR-12, FR-14..FR-16, FR-18..FR-31; UB-3, UB-9..UB-19 | Test + Eval + Demo + Inspection + Analysis + Failure injection | per-row evidence index including every matrix, route/outcome, seam, failure, recovery, and production case | yes |
| DOD-17 *(002 ← DOD-2)* | EVAL-1..EVAL-11 remain executable in permanent CI and every family passes its stated threshold under §5.9; EVT-12 proves `required_runs≥5`, `executed_runs=required_runs`, every required per-run outcome passes, and exact harness/role/native-profile/policy/decode/prompt/calibration provenance. | §5.9, M3, FR-3, FR-4, FR-22, EVAL-1..EVAL-11, EVT-12 | Eval + Inspection | versioned corpus, per-family EVT-12 records, CI link, deterministic comparator definitions | yes |
| DOD-11 *(002 ← DOD-3)* | Pre-release dogfood completes the full assurance×band corpus, including `skip→standard×1`, elevated deep, and critical blind `deep×2`, with reports persisted, harvested, and attribution captured; dogfood authorizes no production acceptance. | M1; FR-23, FR-24, FR-27, FR-30 | Demo + Analysis | dogfood corpus + harvest + environment classification | yes |
| DOD-12 *(002 ← DOD-4)* | The ten-review pre-release checkpoint computes complete-process coverage, precision over the disposition floor, nit rate, assurance×band coverage, and floor/blindness/route/evidence violations from the harvested corpus alone; EVT-11 records the result as part of the single activation predicate. | M1/M2; §3.3; EVT-11; FR-30 | Test + Analysis | checkpoint fixture + pre-release reading + activation-predicate record | yes |
| DOD-13 *(002 ← DOD-5)* | Before coordinated parent activation the compatibility path performs zero pre-invocation network I/O; after activation every pass-routing terminal is canonically appended and exact-read before selected review-model invocation or process-result use, with no other network operation in that interval, and the complete git/proposal evidence then round-trips losslessly. | FR-6, FR-16, FR-27, FR-30, NFR-10, NFR-12 | Test + Inspection + Failure injection | pre-parent zero-network trace; post-parent append/exact-read ordering traces; zero-evidence and evidence-store-outage failures; git/GitHub/Carina golden round-trips plus overwrite and mismatch defects | yes |
| DOD-14 *(002 ← DOD-6)* | A typed implement-now amendment transports the frozen assessment, evidence citations, complete semantic profile, demand identity and derivation-policy version, planned assurance, and selected review harness field-identically into impl-review and receives one aggregate result, while a typed merge-lane `Downgrader` consumes that result to force HUMAN without granting AUTO; both pass end to end and leave protected behavior unchanged. | FR-18, FR-21, FR-24..FR-27, FR-31, NFR-12, NR-5, NR-8, §6.2 | Inspection + Test + Demo | both amendment refs, complete-demand and selected-harness carrier round-trip, and end-to-end HUMAN-forcing fixture | yes |
| DOD-15 *(002 ← DOD-7)* | Every live non-regression row remains green: codegate deterministic, implement-now non-review behavior, siblings untouched, merge-lane classification and impl-review band projection byte-identical, harness selection upstream, and downgrade-only authority unchanged. | NR-1..NR-3, NR-5..NR-8 | Inspection + Test | protected-surface diff audit + conformance results | yes |
| DOD-16 *(002 ← DOD-8)* | A post-review change stales every pass and aggregate; one-pass and two-pass fixtures rerun the complete set on the new exact revision, and no stale or partial set authorizes acceptance. | FR-18, FR-26, UB-11 | Test | staleness + full-rerun fixtures | yes |
| DOD-9 | Living rollups updated; the report/verdict/finding-schema + glossary terms (proposal boundary, impl-review) land in the introducing PR | repo convention; FND-28 | Inspection | the PRs | yes |

### op: supersede §6.2 Dependencies

Reason: this consumer amendment is the compatibility prerequisite that removes the cyclic contract dependency without activating network behavior before the parent policy lands.

| Dependency | Type | Needed by | Status | Owner |
|---|---|---|---|---|
| planner-driven model-routing contract and plan/v2 complete semantic-demand carrier | system | Phase A/B | parent through amendment 002 is approved; this impl-review amendment must land first as a dormant compatibility prerequisite, then `planner-driven-model-routing/003-append-first-routing-evidence` activates mandatory canonical admission; ancestry is proven against main before the parent PR is merged | @mdarabi |
| typed implement-now VerificationGate-class complete review carrier/invocation/result amendment | system | Phase B/C | **unimplemented** — PR #754 added implementation-demand consumption, not the review invocation seam; must carry the complete frozen semantic demand + selected review harness field-identically and return the aggregate before activation | @mdarabi |
| codegate verdict artifact + finding core | system | Phase A/B | existing dependency, unchanged | @mdarabi |
| merge-lane risk class and impl-review band projection | system | Phase B | existing contract preserved byte-for-byte | @mdarabi |
| merge-lane `Downgrader` adapter consuming the persisted aggregate verdict | system | Phase B/C | **unimplemented owed work** per NR-5; must prove end-to-end HUMAN forcing before activation | @mdarabi |
| selected harness review-role policies and qualification evidence | adapter data | Phase B/C | must be complete for every supported cell before activation | adapter owners |
| both tracker media | adapter | Phase A/C | existing report surfaces; schema enrichment required | tracker adapter owners |

### op: supersede §6.4 Release criteria

Reason: release must prove the live append-first route-admission boundary rather than rely on the superseded blanket pre-review network prohibition.

| Lens | Criterion | Evaluated by |
|---|---|---|
| Functionality | all live Must FR/UB fit criteria and EVAL-1..11 pass | CI, eval, failure-injection evidence |
| Process integrity | exhaustive FR-24 matrix; field-identical complete demand and selected harness; zero floor/blindness/revision/remediation/aggregate violations; pre-parent zero-network compatibility and post-parent append/exact-read before every invocation or process use | deterministic conformance + DOD-13/DOD-14 + harvest |
| Adapter support | every declared harness×review-demand cell has current qualification and expected exact/promoted/unsupported result | conformance matrix + operator sign-off |
| Calibration | ten-review pre-release M1/M2/M3/nit gate passes without production authority | checkpoint evidence |
| Reliability | both media, route admission, resume, persistence, staleness, fallback, and post-defect replacement converge; after parent activation an evidence-store outage invokes no review model | fixtures + rehearsals |
| Performance/cost | resolution spends zero inference and zero network I/O; before parent activation the full pre-invocation path remains zero-network, and after activation only canonical append/exact-read may cross that boundary; spend and latency remain observable by depth/count and no target changes semantics | NFR-12 inspection + EVT-7 analysis |
| Parent seam | planner→plan→implement-now→impl-review routing/evidence gate fully green | parent release evidence |
| Activation | the single §3.3 predicate, including every live blocking DoD and §6.6 readiness, is true | EVT-11 + operator sign-off |

### op: supersede §8 Contract readiness — the executability gate

Reason: readiness must account explicitly for complete-demand transport, the compatibility-to-activation transition, and DOD-13's append-first boundary proof.

| Check | Pass? | Notes |
|---|---|---|
| Problem basis accepted | ✓ | existing impl-review problem remains; amendment responds to the coordinated planner-routing policy |
| Zero solution-blocking TBDs | ✓ | Q1/Q3 answered; Q2/Q4 are design/data choices within fixed bars; unimplemented seams and parent activation are explicit blocking dependencies, not open product decisions |
| Every Must FR and UB has fit + verify | ✓ | live FR-3..FR-7, FR-12, FR-14..FR-16, FR-18..FR-31 and UB-3, UB-9..UB-19 carry both |
| High/critical safety plans declared | ✓ | complete-demand integrity, closed join, launch attestation, fail-before-invocation routing, pre-parent zero-network compatibility, post-parent canonical admission, blind contexts, conservative aggregation, direct-cutover gate, same-harness fallback, and invalidation |
| Non-goals and non-regression named | ✓ | §4.2 and NR-1..NR-3, NR-5..NR-8 preserve all boundaries |
| Jobs and flows covered | ✓ | F1 construction and route admission, F2 remediation, F3 aggregation, F4 recovery map to FR-18..FR-31, NFR-12, and UB-9..UB-19; J4 additionally blocks on the owed merge-lane adapter |
| Metrics instrumented | ✓ | M1←FR-30/EVT-7/9; M2←EVT-3/11; M3←EVT-12/EVAL-1..11; M4←explicit human capture line; spend/latency are descriptive; DOD-13 carries the boundary traces into activation |
| Blocking DoD complete | ✓ | DOD-9..DOD-17 cover model behavior, complete-demand carrier integrity, append-first route admission and outage, schema/media, invocation and lane seams, process invariants, release, and docs; §3.3 requires DOD-13's full NFR-12 proof |
| Risks and assumptions owned | ✓ | R1..R9 and A1..A2 have owners and tests/mitigations; §6.2 owns consumer-first then parent activation ordering |
| Human judgment rubric | ✓ | @mdarabi confirms every declared adapter cell has current qualification and real-launch evidence, every §6.6 non-n/a row, every blocking DoD, and the parent seam before EVT-11 can mark activation ready |
