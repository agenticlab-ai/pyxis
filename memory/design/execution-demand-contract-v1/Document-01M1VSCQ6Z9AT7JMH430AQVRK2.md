# Execution demand contract — technical design

| Field | Value |
|---|---|
| **Status** | Draft → Approved *(merge requires Approved; merge is the freezing event)* |
| **Owner** | @mdarabi |
| **Approver(s)** | @mdarabi (operator and security reviewer) |
| **Date** | 2026-07-28 |
| **Relevant-when** | designing or changing plan/v2 execution-demand assessment, derivation, validation, identity, tracker transport, or frozen plan/v1 compatibility; triaging a missing, changed, or irreproducible task demand; superseding the execution-demand contract |
| **Scope** | The vendor-neutral task-demand data contract from planner judgment through frozen plan and tracker reconstruction, including deterministic derivation, sealed transport, demand identity, and no-rewrite plan/v1 translation. **Not:** harness selection, native model/effort resolution, retained routing events, review-process orchestration, or production activation. |
| **Tracking** | — *(the introducing PR)* |
| **Builds-on** | docs/tech-design/local-work-tracker-v2.md · docs/tech-design/distributed-check-registration.md |
| **Supersedes** | docs/tech-design/routing-envelope-v1.md |
| **Serves** | prd:planner-driven-model-routing/FR-22 · prd:planner-driven-model-routing/FR-3 · prd:planner-driven-model-routing/FR-4 · prd:planner-driven-model-routing/FR-5 · prd:planner-driven-model-routing/FR-6 · prd:planner-driven-model-routing/FR-7 · prd:planner-driven-model-routing/FR-15 · prd:planner-driven-model-routing/FR-23 · prd:planner-driven-model-routing/FR-24 · prd:planner-driven-model-routing/NR-3 · prd:planner-driven-model-routing/NR-4 · prd:planner-driven-model-routing/UB-11 · prd:planner-driven-model-routing/UB-2 · prd:planner-driven-model-routing/UB-3 · prd:planner-driven-model-routing/UB-8 · prd:planner-driven-model-routing/ABUSE-2 · prd:planner-driven-model-routing/ABUSE-3 · prd:planner-driven-model-routing/NFR-2 · prd:planner-driven-model-routing/EVAL-1 · prd:planner-driven-model-routing/EVT-1 · prd:planner-driven-model-routing/EVT-6 · prd:plan-new/FR-7 · prd:plan-new/FR-17 · prd:plan-new/FR-21 · prd:plan-new/FR-28 · prd:plan-new/FR-29 · prd:plan-new/FR-30 · prd:plan-new/NR-4 · prd:plan-new/UB-7 · prd:plan-new/ABUSE-3 · prd:plan-new/NFR-7 · prd:plan-new/EVAL-2 · prd:implement-now/FR-21 · prd:implement-now/FR-22 · prd:implement-now/NR-5 · prd:implement-now/UB-9 · prd:implement-now/UB-11 · prd:implement-now/NFR-7 |
| **Security/privacy impact** | yes — this contract controls autonomous model capacity and review assurance and crosses repository/tracker trust boundaries; it carries citations and semantic classifications but no prompt, code, secret, native model, or raw launch argument. reviewer: @mdarabi. |
| **Template version** | tech-design-template/v1 |

## 0. Read to rule — the human reviewer's section (~5 min)

*The body is the contract; this section only routes attention — on any conflict the body governs; machines ignore it.*

**Judge these**

| # | The call | Could have gone | If you disagree |
|---|---|---|---|
| 1 | Plan/v2 freezes six cited measurements and four independently derived minima, never one score | a scalar routing envelope | you want downstream consumers to reconstruct lost dimensions → §3.1, #DINV-1 |
| 2 | Plangate alone derives and freezes the profile under immutable policy identity | planner arithmetic trusted at freeze | you accept two derivation truths or unchecked minima → §3.2, #DINV-2 |
| 3 | Plan/v1 is translated by one permanent table without rewriting source bytes | runtime re-assessment (`#ALT-3`) | you accept mutable post-freeze inference → §3.4, #DINV-5 |
| 4 | The tracker carries one canonical sealed capsule whose digest is checked against the frozen plan | tracker authority or reference-only transport | you accept inference or a second demand authority → §3.5, #DINV-7 |

**Sanity-check the numbers** — exactly six assessment axes and four semantic minima per plan/v2 task (#DINV-1); explicit v1 rungs accept only 1, 2, or 3 (#DINV-5); every executable task, including mechanical work, has one demand (#DINV-3).

**The author's open questions** — none — the gated brainstorm fixes every load-bearing contract value; storage syntax remains an implementation detail behind the canonical codec.

**Skip the rest** — schema/link grammar → docgate and plangate; semantic evidence quality → the operator checkpoint and EVAL-2; native routing/evidence/review/cutover → their sibling designs; rationale → §4.

## 1. Problem

The live scalar routing envelope reduces a task to an optional rung and allows the executor to turn that rung into a hardcoded native model. That shape cannot express the accepted workload and consequence measurements, cannot prove why an assurance floor was chosen, and makes native product choice leak into planning. Tracker projection then risks becoming an accidental second authority: a missing or edited marker can silently restore a cheap default.

The amended contracts instead require planners to preserve semantic evidence while harness adapters remain free to map that meaning to their own native model and effort. Frozen plan/v1 artifacts must still execute without mutation. This design replaces the scalar envelope with the one portable contract both paths can produce (`design-brainstorm:planner-driven-model-routing-tech-design-4#D-1`; `design-brainstorm:planner-driven-model-routing-tech-design-3#D-2`).

## 2. Decision

**Decision:** Every executable task resolves to one immutable `execution-demand/v2` origin: a plan/v2 origin freezes the six cited raw assessment axes plus the four minima produced by one versioned derivation policy, while a plan/v1 origin records the raw rung's tagged presence/value and the result of one permanent compatibility table. One canonical codec, evaluator, identifier function, and sealed tracker capsule preserve that origin without native model, effort, harness, or downstream inference.

The planner owns contextual judgment; the plan gate owns mechanical derivation and validation; tracker adapters only transport bytes; consumers verify against the exact frozen plan before claim or review. That boundary lets Codex and Cursor interpret the same demand differently without letting either revise it.

## 3. The design

### 3.1 Closed origin schema

**Decision:** `execution-demand/v2` is a tagged union with `plan_v2` and `plan_v1_compat` origins. All objects parse closed; unknown keys, values, versions, duplicate fields, and non-canonical encodings are defects.

The `plan_v2` member contains:

- repository, frozen-plan, and task identities plus the task source-content digest;
- `assessment`: `reasoning`, `coupling`, `novelty`, `verification`, `impact_scope`, and `architectural_role` using the PRD's closed vocabulary;
- a non-empty normalized set of resolvable evidence citations supporting those axes;
- `profile`: independently ordered `capability`, `deliberation`, `context`, and `assurance` minima;
- derivation-policy schema version, immutable policy identity and digest, and the identifiers of every applied floor and promotion;
- an optional planner assurance raise containing the raised value and non-empty evidence citations. A raise may only increase assurance.

The `plan_v1_compat` member contains the same repository/plan/task identity, the frozen source digest, a raw `rung_present` boolean, the raw value when present, compatibility-table version and digest, translation class, and translated four-part profile. It never fabricates plan/v2 assessment axes or citations.

- **DINV-1** — Every plan/v2 executable task carries exactly one accepted value for all six axes, a non-empty resolvable citation set, all four minima, and one derivation-policy identity; no scalar summary substitutes for any field.
- **DINV-2** — The frozen profile equals mechanical evaluation of the frozen assessment, applicable policy, and any monotone assurance raise byte-for-byte.
- **DINV-3** — Mechanical/provenance work is still model-backed and carries the complete contract; only an empty ready frontier has no demand.

### 3.2 One derivation and verification authority

**Decision:** A single pure Go evaluator consumes the closed assessment and immutable `execution-demand-policy/v1` data, takes the componentwise maximum of every applicable floor, applies named interaction promotions, and then applies a cited assurance raise. The planner authors only the assessment, citations, and optional assurance raise. Plangate alone calls the evaluator to materialize the four minima into the generated plan face and canonical capsule, then verifies those bytes before checkpoint and freeze. An authoring UI may preview evaluator output, but that preview is non-authoritative and never becomes a planner-authored profile accepted by the gate.

Policy identity is content-bound rather than an ambient alias. A frozen plan records the exact compatible policy version and digest; absence or mismatch refuses validation. The operator checkpoint separately judges whether each cited anchor substantively supports every axis and raise. The evaluator never uses a model, network, current harness catalog, task prose inference, or native model data.

- **DINV-4** — Reordering assessment fields, citations, floors, or promotions changes neither the canonical profile nor its identity.

### 3.3 Demand identity

`demand_id` is the lowercase digest of a domain tag plus canonical length-delimited fields:

`execution-demand-id/v1 || origin schema/version || repository identity || frozen-plan identity || task identity || source digest || origin payload || policy-or-compatibility identity || final profile`.

The plan/v1 origin payload includes rung presence separately from its value; the plan/v2 origin includes every raw axis, normalized citation, promotion, and raise. Tracker text, proposal text, timestamps, runtime attempts, and current policy aliases are excluded. Implementation retries, review passes, and remediation rounds reuse the demand ID and mint distinct attempt IDs in the evidence contract.

- **DINV-5** — Identical canonical source and policy bytes reproduce one demand ID; any identity-bearing source, policy, compatibility, or result change produces another.

### 3.4 Frozen plan/v1 compatibility

**Decision:** The immutable `plan-v1-rung/v1` table is:

| Frozen source | Capability | Deliberation | Context | Assurance | Class |
|---|---|---|---|---|---|
| explicit `rung=1` | economy | light | standard | ordinary | `explicit_rung` |
| explicit `rung=2` | standard | standard | large | ordinary | `explicit_rung` |
| explicit `rung=3` | frontier | maximum | extended | critical | `explicit_rung` |
| rung truly absent | frontier | maximum | extended | critical | `legacy_unassessed` |

The raw parser must distinguish an absent field from every explicit value. An explicit value outside `1..3`, including `0`, or an unavailable compatibility version fails before claim or invocation. Translation never writes the frozen plan and never re-assesses its charter (`design-brainstorm:planner-driven-model-routing-tech-design-3#D-2`).

- **DINV-6** — A plan/v1 source remains byte-identical before and after any number of translations or attempts.

### 3.5 Sealed tracker capsule

**Decision:** Projection writes one canonical `execution-demand-capsule/v1` marker into each supported tracker item's ordinary body before it can become routable. The capsule contains the complete canonical origin, exact plan/task identities and fingerprints, demand ID, codec version, and capsule digest. Both tracker adapters use the same codec and preserve the same logical bytes; they do not infer, merge, or default fields.

Readback accepts exactly one capsule. Before claim and again before review, the consumer exact-reads the authoritative frozen plan, reconstructs its demand through this design, and requires field and digest identity with the capsule. Missing, duplicate, malformed, truncated, copied-across-task, stale, or changed capsules refuse the affected item. Tracker state is a self-contained cache and transport, never the authority. Proposal state may carry only the capsule ID/digest needed to repeat that comparison after restart.

- **DINV-7** — No task claim or review process may begin unless one capsule is byte-equivalent to the demand reconstructed from the exact frozen plan.

### 3.6 Boundaries and non-goals

- **Not solved here** — selected-harness policy, native model/effort selection, route/outcome retention, assurance×band process, and activation.
- **Owned by sibling designs** — `native-routing-policy-v1`, `routing-evidence-v1`, `impl-review-routed-assurance`, and `implement-now-routed-execution`.
- **Implementation must not infer** — missing fields, plan/v1 axes, native availability, review count, or harness choice from charter prose, labels, installed CLIs, or current policy.
- **Must remain unchanged** — plan/v1 bytes, general authoring-stage routing, plan-new decomposition/projection shape, and non-planner tracker work.

### 3.7 Observability

- **Events/logs** — the evidence owner records a typed demand-origin event; validation emits structured accepted/refused diagnostics with task, schema, version, field, and rule, never prompt/code payload.
- **Metrics** — counts by origin class, policy/compatibility version, profile component, and refusal reason; no correctness threshold depends on cost.
- **Alerts** — repeated capsule-integrity or unavailable-policy defects alert the repository operator; isolated task defects remain in the run report.
- **Debugging surface** — `docgate`/plangate inspection renders the canonical origin, applied rules, expected profile, capsule digest, and exact mismatch without native routing data.
- **Trace/correlation** — demand ID joins plan, task, capsule, later attempts, reviews, and evidence events.

### 3.8 Security & privacy

The contract affects billing and autonomous capacity, crosses an untrusted tracker boundary, and sends its eventual context to a model provider. Closed parsing, exact-plan comparison, cited monotone derivation, and the prohibition on native/raw launch data prevent contributor text or adapter defaults from lowering capacity or injecting arguments (#DINV-2, #DINV-7). Payloads contain internal classifications and citation references, but never prompt bodies, code, credentials, provider secrets, or user data beyond repository-scoped identifiers. Authorization of policy and frozen plans remains with existing repository controls.

### 3.9 Failure and test seams

| Failure | System behavior | Detection | Recovery |
|---|---|---|---|
| invalid plan/v2 assessment, citation, profile, or raise | reject whole plan before checkpoint/freeze | evaluator + citation resolver + operator rubric | edit draft and re-present |
| unknown plan/v1 rung/version | refuse affected task before claim/review | tagged raw parser | operator supplies a new successor compatibility design; source stays frozen |
| capsule missing or mismatched | refuse affected task before claim | exact frozen-plan comparison | repair projection from the frozen authority; never edit demand in tracker |
| authoritative plan unavailable | block, not default | exact-read error | restore access and retry with the same source |

The evaluator and ID/codec are in-process pure tests; frozen-plan and tracker media use local-substitutable golden round trips; each production tracker adapter runs the same conformance suite. Property tests cover field ordering, max monotonicity, presence/value distinction, tamper classes, and repeated translation.

## 4. Alternatives considered

| Alternative | Why rejected |
|---|---|
| Translate known rungs and true absence through the fixed table (#ALT-4) | selected: preserves historical intent, treats only genuine ambiguity conservatively, and introduces no new inference (`design-brainstorm:planner-driven-model-routing-tech-design-3#ALT-4`) |
| Translate every plan/v1 task to maximum (#ALT-5) | throws away known frozen judgment and forces critical review on all legacy work |
| Re-assess legacy charter text at runtime (#ALT-6) | creates a second planner, mutable demand, and retry disagreement |
| Fold execution demand into the plan-new successor (`design-brainstorm:planner-driven-model-routing-tech-design-4#ALT-2`) | hides the shared implementation/review boundary inside one producer and creates divergent validation risk |
| Keep the optional scalar routing envelope | cannot preserve independent workload/consequence minima or harness-neutral evidence and leaves downstream inference necessary |
| Store only a plan reference in tracker | a worker cannot validate or resume from the item alone and truncation is not locally detectable |
| Treat tracker demand as authoritative | permits editable transport to override a frozen plan and creates two truths |

## 5. Risks

| ID | Risk | Impact | Mitigation | Verification / evidence |
|---|---|---|---|---|
| R1 | A policy update makes old demand irreproducible | retries route differently | content-bound policy identity; retain compatible policy data | old-policy golden corpus on a fresh clone |
| R2 | Presence parsing collapses `rung=0` into absence | invalid legacy work gets maximum translation instead of refusing | tagged raw parse before typed decode | absent/zero/unknown table tests |
| R3 | Tracker rendering alters capsule bytes | valid work refuses or corrupted work passes | one canonical codec and digest, media conformance | GitHub/Carina round-trip goldens |
| R4 | Citation existence is mistaken for substantive support | planner can minimize demand with irrelevant evidence | operator task-by-task rubric and EVAL-2 remain mandatory | persisted checkpoint rubric + adversarial eval |

## 6. Acceptance

| ID | Acceptance criterion | Verify | Required evidence | Traces to |
|---|---|---|---|---|
| A-1 | Every plan/v2 fixture, including mechanical work, freezes all six cited axes and four verified minima or freezes nothing. | Test + Eval | derivation/citation corpus and EVAL-2 report | `prd:plan-new/FR-28` · `prd:plan-new/FR-29` · #DINV-1 |
| A-2 | Plangate alone materializes the profile/capsule from planner-authored raw inputs; reordered inputs derive byte-identical profiles, previews carry no authority, and every tampered or lowered result fails before checkpoint/freeze. | Test + Inspection | exhaustive policy golden, sole-writer dependency audit, preview/tamper matrix | `prd:planner-driven-model-routing/FR-3` · `prd:planner-driven-model-routing/FR-4` · #DINV-2 |
| A-3 | The operator rubric rejects irrelevant, contradictory, or partial support and persists the accepted task-by-task evidence judgment. | Human judgment — @mdarabi, rubric: every axis and raise has substantive cited support | signed checkpoint rubric | `prd:plan-new/FR-30` · `prd:planner-driven-model-routing/FR-23` |
| A-4 | Rungs 1–3 and true absence translate exactly as §3.4; explicit 0/unknown/version-missing cases make zero claims/invocations; every source remains byte-identical. | Test + Inspection | compatibility report and byte-identity manifest | `prd:planner-driven-model-routing/FR-15` · `prd:implement-now/FR-22` · #DINV-6 |
| A-5 | Every supported tracker round-trips one capsule field-identically; every missing/duplicate/tampered/copied/stale class refuses before implementation claim and before direct or restarted review-process creation, producing zero reviewer/model launch. | Test | shared-codec media conformance plus implementation/review/restart refusal report | `prd:planner-driven-model-routing/FR-7` · `prd:implement-now/FR-21` · #DINV-7 |
| A-6 | Demand IDs reproduce across retries and roles while every identity-bearing source or policy mutation changes the ID. | Test | ID property/golden suite | `prd:planner-driven-model-routing/FR-24` · #DINV-5 |
| A-7 | Inspection finds no scalar demand, native model/effort, harness, provider context, raw launch argument, or runtime re-assessment in a plan/v2 origin. | Inspection | schema and dependency audit | `prd:plan-new/NFR-7` · `prd:implement-now/NFR-7` |

## 7. Open questions

None. Codec layout and storage paths may vary behind the binding closed schemas, identities, and conformance surface; changing any semantic field or compatibility mapping requires a successor design.

## Principles

- **AP-3 — honored.** The same semantic contract crosses planner, tracker, implementation, and review adapters.
- **AP-5 — honored.** Demand identity, refusals, and applied derivation rules are observable by construction.
- **AP-6 — honored.** Every cross-boundary object is a closed versioned schema; prose and labels never carry hidden routing semantics.
- **AP-10 — honored.** Derivation and compatibility are reviewed data evaluated mechanically.
- **AP-11 — honored.** One evaluator and one codec prevent competing predicates.
- **AP-13 — honored.** Exact frozen-plan evidence is checked before consequential execution.
- **AP-15 — honored.** Repeated projection, translation, and reconstruction converge on one demand.
- **AP-17 — honored.** Invalid or unavailable demand fails before claim or model invocation.
- **AP-20 — honored.** Planner judgment, mechanical validation, transport, and native resolution have separate owners.
- **AP-21 — honored.** Judgment happens once at planning; downstream stages verify and consume without re-judging.
- **AP-28 — honored.** The frozen plan remains the single canonical demand origin; tracker data is a verified projection.
- **RW-4 — honored.** Plan/v1 is consumed through explicit compatibility rather than rewritten.
- **RW-5 — honored.** This document owns only the independently revisable demand contract.
- **Deviation:** none.
