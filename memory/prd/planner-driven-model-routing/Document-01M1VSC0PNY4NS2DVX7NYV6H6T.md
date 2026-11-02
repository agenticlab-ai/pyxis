# Amendment 003 — append-first-routing-evidence

| Field | Value |
|---|---|
| **Amends** | planner-driven-model-routing *(chain through 002)* |
| **Status** | Approved |
| **Owner** | @mdarabi |
| **Date** | 2026-07-28 |
| **Learned-from** | operator-decision:2026-07-28-canonical-git-evidence-before-launch · build-result:docgate-source-map-missing-planner-routing-EVT-8-2026-07-28 · review-thread:planner-driven-model-routing-003-FND-2 |
| **Blast radius** | tier: contract-changing; touches: NFR-6, NFR-8, EVT-8, §5.7, §6.2, §6.4; reason: preserve offline deterministic route selection while requiring canonical git evidence append and exact-read before launch, coordinate the same admission rule across both consumer PRDs before this parent change lands, and repair the legacy-origin event's typed ledger provenance while normalizing its unchanged two-value enum notation |
| **Template version** | amendment-template/v2 |

## Operations

### op: supersede NFR-6

Reason: append-first admission requires durable canonical evidence before launch, while the current fit criterion forbids every outbound call in that interval and therefore contradicts FR-21's immutable-git evidence boundary.

| ID | Area | Requirement | Fit criterion | Verify |
|---|---|---|---|---|
| NFR-8 | Availability / cost | For identical assessment, policy version, selected harness, role, override, and qualification evidence, routing resolution shall produce the same result using zero model inference and zero network I/O; after every terminal result exists and before task claim or selected worker/reviewer invocation, the runtime shall durably append that terminal to the configured canonical git evidence store and exact-read it successfully. The append and exact-read are the only network operations permitted in that interval: no policy lookup, qualification lookup, harness probe, model call, or other outbound operation may occur. | repeated and reordered-input fixtures are byte-identical; the offline resolution suite records zero inference and zero network I/O through the terminal result; every healthy-path boundary trace contains a successful canonical append and exact-read before claim or invocation and no other network operation; a zero-evidence healthy path fails, and unavailable or unverifiable evidence produces zero claim and zero worker/reviewer invocation | Inspection + Test + Failure injection |

### op: supersede §5.7 Instrumentation — the tracking plan

Reason: amendment 001 introduced EVT-8 inside a wholesale section replacement, which rendered the row in the projection without creating typed unit provenance; this replacement removes that untracked copy so the following typed add can introduce it exactly once.

| ID | Event name | Fires when | Properties (typed) | Source | Feeds |
|---|---|---|---|---|---|
| EVT-1 | `execution_demand_frozen` | an `/implement-now` plan/v2 task passes plan validation | `demand_id:string`, `task_id:string`, `plan_version:string`, `assessment:closed object`, `assessment_evidence_refs:string[]`, `profile:closed object`, `policy_version:string`, `assurance_raise:boolean` | plan gate | M2, M5 |
| EVT-2 | `routing_resolved` | an implementation or review route resolves | `attempt_id:string`, `demand_id:string`, `task_id:string`, `proposal_id:string?`, `harness:string`, `role:implementation\|review`, `requested_profile:closed object`, `proposal_band:skip\|standard\|deep\|null`, `assurance_join_raise:boolean`, `native_profile_id:string`, `resolution:exact\|promoted`, `promoted_components:enum[]`, `policy_version:string` | runtime adapter | M2, M4, M5 |
| EVT-3 | `routing_resolution_failed` | routing stops before claim/invocation | `attempt_id:string`, `demand_id:string`, `task_id:string`, `proposal_id:string?`, `harness:string`, `role:implementation\|review`, `requested_profile:closed object`, `proposal_band:skip\|standard\|deep\|null`, `assurance_join_raise:boolean`, `policy_version:string`, `unmet_components:enum[]`, `reason:enum` | executor/reviewer | M2, M5 + health alert |
| EVT-4 | `routing_conformance_checked` | one required routing-domain cell finishes conformance | `harness:string`, `role:enum`, `semantic_profile_id:string`, `expected:resolved\|unsupported`, `result:pass\|fail`, `evidence_ref:string` | conformance harness | M3 |
| EVT-5 | `routed_attempt_completed` | implementation verification or independent review completes | `attempt_id:string`, `task_id:string`, `proposal_id:string?`, `outcome:verified\|failed\|blocked`, `usage:closed object`, `evidence_ref:string` | executor/reviewer | M4, M5 |
| EVT-6 | `execution_demand_checked` | every plan/v2 task presented for `/implement-now` reaches validation, before acceptance or rejection | `task_id:string`, `plan_version:string`, `result:pass\|fail`, `violations:enum[]` | plan gate | M1 |
| EVT-10 *(002 ← EVT-7)* | `routing_evidence_invalidated` | UB-10 or UB-14 identifies evidence produced under a defective policy/profile | `policy_version:string`, `native_profile_id:string`, `attempt_ids:string[]`, `review_process_ids:string[]`, `artifact_refs:string[]`, `disposition:enum{rerun_required,operator_remediation}` | execution/review evidence owner | health alert + UB-10/UB-14 audit |
| EVT-9 *(002)* | `review_process_completed` | the assurance-pass set for one exact proposal revision reaches an aggregate terminal, including a floor violation | `review_process_id:string`, `demand_id:string`, `task_id:string`, `proposal_id:string`, `reviewed_revision:string`, `planned_assurance:enum{ordinary,elevated,critical}`, `proposal_band:enum{skip,standard,deep}`, `effective_depth:enum{standard,deep}`, `required_passes:int`, `completed_passes:int`, `pass_attempt_ids:string[]`, `top_level_context_ids:string[]`, `pass_verdicts:enum[]`, `aggregate_verdict:enum`, `result:enum{satisfied,floor_violation,conflict_nonaccepting}`, `evidence_ref:string` | review orchestrator | M6, M5, §3.3 guardrail + health alert |

### op: add EVT

Reason: the legacy-origin event already governs FR-24, M2, M5, and DOD-15; adding it through the typed operation restores the missing source-map identity without changing its event name, payload meaning, source, or feeds, while expressing the existing two-value translation field in the parser-stable closed-enum notation.

| ID | Event name | Fires when | Properties (typed) | Source | Feeds |
|---|---|---|---|---|---|
| EVT-8 | `legacy_execution_demand_translated` | a frozen plan/v1 task is translated for `/implement-now` without rewrite | `demand_id:string`, `task_id:string`, `plan_version:string`, `rung_present:boolean`, `original_rung:string?`, `compatibility_version:string`, `translation:enum{explicit_rung,legacy_unassessed}`, `translated_profile:closed object`, `policy_version:string` | legacy translator | M2, M5 |

### op: supersede §6.2 Dependencies

Reason: append-first evidence admission governs both implementation and review routes, so the consumer contracts must adopt the same pre-invocation exception before this parent amendment lands.

| Dependency | Type | Needed by | Status | Owner |
|---|---|---|---|---|
| Typed amendments to the frozen plan-new, implement-now, and impl-review contracts | approval / document system | Phase A design handoff | required (`brainstorm:planner-driven-model-routing#P-1`); implement-now amendment 003 and impl-review amendment 003 must land before this parent amendment so both consumers require canonical route-evidence append/exact-read before claim or invocation; the impl-review contract continues to preserve merge-lane classification, retain impl-review's class-to-band projection, distinguish scope-locked remediation re-reviews from top-level assurance passes, prohibit a zero-review `skip` terminal, and carry FR-25/FR-26 evidence | @mdarabi + factory |
| Existing plan projection and tracker-adapter boundary | system | Phase A | available; known transport defects | plan/executor owners |
| Existing shared runtime port and supported harness registry | system | Phase B | available; native policies incomplete/inconsistent | runtime owners |
| Existing merge-lane proposal risk class and impl-review class-to-band projection | system | Phase B | available; ownership and output semantics protected by NR-9 | review/merge-lane owners |
| Qualified native model roster and cost metadata per harness/role | vendor / policy | Phase B and release | design-owned; evidence required by FR-12/EVAL-2 | @mdarabi + runtime owners |

### op: supersede §6.4 Release criteria

Reason: the performance/cost release lens must cite the live offline-resolution and append-first admission requirement rather than superseded NFR-6.

| Lens | Criterion | Evaluated by |
|---|---|---|
| Functionality | Every live Must FR and UB fit criterion passes; M1, M3, and M6 reach target over their complete denominators. | CI, eval, conformance matrix, EVT-9 process report |
| Usability | Operator explains exact, promoted, unsupported, and joined/aggregated review decisions from emitted evidence without reading adapter code. | DOD-15 audit walkthrough |
| Reliability | Zero §3.3 guardrail breaches across conformance, adversarial fixtures, dogfood, direct-cutover rehearsal, and kill-switch rehearsal; the post-defect disposition drill and universal-review process gate pass. | test/eval reports, DOD-13 decision report, DOD-14 drill record |
| Performance / cost | Resolution remains offline, deterministic, and inference-free; the sole permitted pre-launch network boundary is canonical evidence append/exact-read; every successful selection is the least-cost qualified satisfying profile. | NFR-1, NFR-5, NFR-8 suites |
| Supportability | Named failures identify harness, role, policy, task, unmet component, and any review-process violation; strongest-profile fallback and runbook are operator-approved. | diagnostic goldens, runbook review, DOD-12 |
