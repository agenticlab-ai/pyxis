# impl-review routed assurance — technical design

| Field | Value |
|---|---|
| **Status** | Draft → Approved *(merge requires Approved; merge is the freezing event)* |
| **Owner** | @mdarabi |
| **Approver(s)** | @mdarabi (operator and security reviewer) |
| **Date** | 2026-07-28 |
| **Relevant-when** | designing or changing impl-review's assurance×band join, review-role routing, top-level pass identity, mutually blind one/two-pass orchestration, shared review context packing, finding/report persistence, remediation lineage, staleness, aggregate verdict, or merge-lane downgrade signal; superseding impl-review |
| **Scope** | The impl-review successor: existing four-lens reviewer behavior and finding grammar, planned-assurance join, selected-harness review resolution, exact-revision pass/process identities, concurrently collected blind pass sets over a cacheable shared pack, conservative aggregation, canonical evidence projections, attribution, harvest, and downgrade-only aggregate consumption. **Not:** implementation worker lifecycle, demand derivation, native catalog semantics, git append mechanics, merge-lane classification, or production activation. |
| **Tracking** | — *(the introducing PR)* |
| **Builds-on** | docs/tech-design/execution-demand-contract-v1.md · docs/tech-design/native-routing-policy-v1.md · docs/tech-design/routing-evidence-v1.md · docs/tech-design/codegate-v1.md · docs/tech-design/prd-review-artifacts.md · docs/tech-design/merge-lane-v1.md · docs/tech-design/context-pack-v1.md · docs/tech-design/local-work-tracker-v2.md · docs/tech-design/e2e-verification-skills.md |
| **Supersedes** | docs/tech-design/impl-review-v1.md |
| **Serves** | prd:impl-review/FR-3 · prd:impl-review/FR-4 · prd:impl-review/FR-5 · prd:impl-review/FR-6 · prd:impl-review/FR-7 · prd:impl-review/FR-12 · prd:impl-review/FR-14 · prd:impl-review/FR-15 · prd:impl-review/FR-16 · prd:impl-review/FR-18 · prd:impl-review/FR-19 · prd:impl-review/FR-20 · prd:impl-review/FR-21 · prd:impl-review/FR-22 · prd:impl-review/FR-23 · prd:impl-review/FR-24 · prd:impl-review/FR-25 · prd:impl-review/FR-26 · prd:impl-review/FR-27 · prd:impl-review/FR-28 · prd:impl-review/FR-29 · prd:impl-review/FR-30 · prd:impl-review/FR-31 · prd:impl-review/NR-1 · prd:impl-review/NR-2 · prd:impl-review/NR-3 · prd:impl-review/NR-5 · prd:impl-review/NR-6 · prd:impl-review/NR-7 · prd:impl-review/NR-8 · prd:impl-review/UB-3 · prd:impl-review/UB-9 · prd:impl-review/UB-10 · prd:impl-review/UB-11 · prd:impl-review/UB-12 · prd:impl-review/UB-13 · prd:impl-review/UB-14 · prd:impl-review/UB-15 · prd:impl-review/UB-16 · prd:impl-review/UB-17 · prd:impl-review/UB-18 · prd:impl-review/UB-19 · prd:impl-review/ABUSE-3 · prd:impl-review/ABUSE-4 · prd:impl-review/ABUSE-5 · prd:impl-review/ABUSE-6 · prd:impl-review/ABUSE-7 · prd:impl-review/NFR-6 · prd:impl-review/NFR-7 · prd:impl-review/NFR-8 · prd:impl-review/NFR-10 · prd:impl-review/NFR-11 · prd:impl-review/NFR-12 · prd:impl-review/EVAL-1 · prd:impl-review/EVAL-2 · prd:impl-review/EVAL-3 · prd:impl-review/EVAL-4 · prd:impl-review/EVAL-5 · prd:impl-review/EVAL-6 · prd:impl-review/EVAL-7 · prd:impl-review/EVAL-8 · prd:impl-review/EVAL-9 · prd:impl-review/EVAL-10 · prd:impl-review/EVAL-11 · prd:impl-review/EVT-2 · prd:impl-review/EVT-3 · prd:impl-review/EVT-6 · prd:impl-review/EVT-7 · prd:impl-review/EVT-8 · prd:impl-review/EVT-9 · prd:impl-review/EVT-10 · prd:impl-review/EVT-11 · prd:impl-review/EVT-12 · prd:planner-driven-model-routing/FR-14 · prd:planner-driven-model-routing/FR-25 · prd:planner-driven-model-routing/FR-26 · prd:planner-driven-model-routing/NR-6 · prd:planner-driven-model-routing/NR-9 · prd:planner-driven-model-routing/UB-12 · prd:planner-driven-model-routing/UB-13 |
| **Security/privacy impact** | yes — fresh review model calls carry exact proposal diff, charter, binding requirements/design, and security context to the selected provider; process evidence affects acceptance and operator attribution. No worker session/output is shared and canonical payloads exclude prompt/code. reviewer: @mdarabi. |
| **Template version** | tech-design-template/v1 |

## 0. Read to rule — the human reviewer's section (~7 min)

*The body is the contract; this section only routes attention — on any conflict the body governs; machines ignore it.*

**Judge these**

| # | The call | Could have gone | If you disagree |
|---|---|---|---|
| 1 | Every proposal gets at least one model-backed pass; critical gets two mutually blind deep passes | zero-pass skip or remediation-as-pass | you want risk classification to erase the review floor → §3.1, #DINV-1 |
| 2 | Required top-level passes route first, then launch concurrently and collect all terminals behind one barrier | sequential fail-fast (`#ALT-2`) | you accept order-dependent evidence and omitted critical findings → §3.5, #DINV-7 |
| 3 | All passes receive one byte-identical shared base pack, then private pass/lens overlays | separately assembled packs or shared sessions | you reject provider prefix caching or accept judgment leakage → §3.4, #DINV-5 |
| 4 | Any head change stales the whole process and requires a fresh full set | reuse unchanged pass results | you accept partial certification of a changed tree → §3.7, #DINV-9 |
| 5 | Canonical git events govern; the one proposal report is a checked projection | proposal payload authority | you accept two evidence truths → §3.8, #DINV-10 |

**Sanity-check the numbers** — ordinary always ≥ standard×1; elevated always deep×1; critical always deep×2 (#DINV-1); every top-level pass contains four blind lens contexts (#DINV-6); no aggregate appears before every required pass terminal (#DINV-8).

**The author's open questions** — none — native profile inventories and context-size limits remain qualified policy data; all process semantics are closed here.

**Skip the rest** — finding calibration/eval content → live impl-review PRD data; demand/native/evidence mechanics → sibling designs; implementation lifecycle and activation → executor successor; rationale → §4.

## 1. Problem

The first impl-review design correctly established a machine-native finding/report grammar, one proposal-local rendered artifact, four fresh lenses, operator-attributed dispositions, and revision-bound review. Its binding runtime is now false in three consequential ways: a second reviewer was dormant rather than required, proposal payload was the evidence authority, and one configured reviewer envelope could bypass role-qualified demand. The amended contract makes at least one model-backed pass universal and requires two independent deep passes for critical assurance.

Simply running the old reviewer twice is insufficient. The system must identify an exact process, route both passes through the selected harness's review role, preserve mutual blindness, reuse common input for provider caching without sharing judgment, collect the complete set independent of order, and stale all evidence on a head change (`design-brainstorm:planner-driven-model-routing-tech-design-4#D-1`; `design-brainstorm:planner-driven-model-routing-tech-design-3#D-4`).

## 2. Decision

**Decision:** impl-review computes one exact-revision assurance process by joining frozen planned assurance with the unchanged merge-lane→impl-review band projection, resolves every required top-level pass independently through the selected harness's review-role catalog, durably admits all routes, then runs the passes concurrently as fresh sibling reviewer roots over one byte-identical shared context-pack prefix plus private overlays. It waits for the complete pass set, unions all surviving findings, and publishes the most restrictive aggregate; missing, stale, invalid, failed, or conflicting evidence is non-accepting.

Each top-level assurance pass carries forward the existing reviewer realization: four mutually blind fresh lens contexts, adversarial screening, calibration-cited findings, scope-locked remediation, operator-attributed dispositions, one rendered proposal artifact, and downgrade-only authority. Canonical git events replace the proposal payload as evidence authority; the proposal payload becomes a losslessly checked `impl-review-report/v2` projection.

## 3. The design

### 3.1 Assurance×band join

**Decision:** The merge lane remains the sole actual-risk classifier and impl-review retains its unchanged class→`skip|standard|deep` band projection. A pure `assurance-process-policy/v1` function joins that band with frozen planned assurance:

| Planned assurance | skip | standard | deep |
|---|---|---|---|
| ordinary | standard × 1 | standard × 1 | deep × 1 |
| elevated | deep × 1 | deep × 1 | deep × 1 |
| critical | deep × 2 | deep × 2 | deep × 2 |

`skip` is the lowest actual-risk band, never permission for zero review. Depth and count are independent; an internal scope-locked remediation re-review is not a top-level assurance pass.

- **DINV-1** — Every proposal requires one or two model-backed top-level passes exactly as this table; no input lowers another.
- **DINV-2** — Merge-lane risk classification and impl-review's band projection remain byte-identical; this join consumes their result and creates no second classifier.

### 3.2 Process, pass, attempt, and context identity

`review_subject_id` is the domain-separated digest of repository, demand/task/proposal identity, exact reviewed revision, authoritative lane-verdict identity and projected band, planned assurance, process-policy version/digest, and selected review harness. A process generation is CAS-allocated from canonical evidence for that subject: generation 1 for the first run, reuse of the same generation for any invocation while it has no terminal, and the next integer only after the prior generation has a terminal or invalidation requiring a full replacement. `review_process_id` is the digest of `review_subject_id` plus that generation and prior-process terminal reference. Thus double invocation resumes one in-flight process, while a recovered rerun after a terminal creates a linked new process.

The required pass set is derived before launch. Each `pass_id` adds the process ID and its stable one-based pass index; each actual route/launch has a distinct attempt ID from the evidence protocol and each pass has a fresh top-level context ID.

An internal remediation round derives from its pass ID, records its own reviewed revision and round index, and remains inside that pass lineage. It cannot be promoted into another pass identity.

- **DINV-3** — Required pass IDs are stable within one process generation, distinct from one another, and independent of scheduling order; a replacement generation derives fresh pass identities and links the prior terminal.
- **DINV-4** — Attempt, top-level pass, lens, and internal remediation identities are different types and never satisfy one another's cardinality.

### 3.3 Review-role routing and admission

The invocation supplies the selected review harness explicitly; impl-review never inherits the implementation harness implicitly and never selects a harness. For each pass, the joined execution profile keeps the planned capability/deliberation/context minima and applies the process depth/assurance minimum. `native-routing-policy-v1` resolves `role=review` independently.

Every pass gets its own route attempt and canonical `routing_resolved` proof. The orchestrator constructs and durably admits the complete pass set before launching any pass. If admission fails after earlier passes resolved, it appends a compensating blocked outcome for every already-resolved pass before recording the non-accepting process; the failed resolution gets no outcome. No pass launches until that compensation-or-complete-admission barrier is settled. Actual native launch is captured and must equal the route.

### 3.4 One cacheable base context, private overlays

**Decision:** A deterministic compiler builds one content-addressed base review pack for the exact process revision from:

- exact proposal diff/tree and codegate verdict;
- task charter and frozen execution demand;
- served PRD requirements and technical-design binding elements;
- required ABUSE rows and incident context at deep depth;
- versioned reviewer calibration and critic rules.

The compiler uses `context-pack-v1` ordering, normalization, budgets, and exact-source provenance. Every required pass receives byte-identical base bytes and records the same pack digest. The byte stream is ordered `base pack || private pass overlay || private lens overlay`, so all provider requests share the largest exact prefix while pass identity/instructions and lens instructions remain private suffixes.

The base contains no implementing-session state, prior/sibling pass identity, candidates, findings, verdicts, outputs, orchestration notes, mutable timestamps, or authoring trail. A pass overlay contains only process/pass identity and pass instruction. A lens overlay contains only that pass's lens identity/instruction. No output is written back into the base.

- **DINV-5** — All passes use identical base-pack bytes/digest, but distinct fresh top-level contexts and no sibling output lineage; caching shared context never shares judgment.

### 3.5 Concurrent pass and lens orchestration

**Decision:** The reusable reviewer runtime accepts a closed `reviewer-program/v1`: stable program ID/version, ordered lens set, screening/calibration identities, internal-round cap, scope-lock rule, verdict grammar, and context-pack projection. The impl-review program fixes four lenses—spec fidelity, design conformance, security/abuse, and correctness critic—while the existing e2e review program remains a compatible two-lens instance over the same runtime. Program conformance proves fresh contexts, lens-set cardinality, scope-locked convergence, and no cross-lens output for both instances; changing one program's lens set does not change the runtime or the other program.

Once all routes are durably admitted, one bounded owner launches required top-level passes concurrently and waits at a full-set barrier. It never cancels or hides a sibling because another pass is non-accepting. Each pass launches its program's mutually blind lens contexts over the base/pass prefix plus its private lens overlay, then adversarially screens candidates into calibrated findings and a pass verdict.

Host cancellation propagates to every child; the owner reaps all contexts and records a terminal for every admitted route. Results enter the aggregate only after revision/identity/schema/evidence validation.

- **DINV-6** — Each top-level pass contains the complete four-lens set; no lens sees another lens's or pass's output before its own terminal.
- **DINV-7** — Pass scheduling and completion order cannot change the finding union or aggregate verdict.

### 3.6 Findings, remediation, and pass terminal

The grammar-compatible finding core remains: path, line, message, log reference plus rule ID, lens, blocking/advisory severity, calibration row, acceptance, and attributed disposition. Screening removes codegate-mechanical defects and enforces compactness. A pass terminal is valid only after its routed attempt outcome is verified and records exact revision, pack digest, all context identities, candidate/killed/finding counts, internal rounds, verdict, usage, and provenance.

One scope-locked remediation re-review may revisit only an originating pass's surviving findings after a worker fix. It is feedback/disposition evidence attached to that pass lineage and never certifies the changed revision, changes the old pass verdict, or satisfies any top-level pass. Because the fix changes code/head, §3.7 immediately requires a new process generation with the full assurance-pass set; only that new generation can certify the new revision.

### 3.7 Barrier aggregation and staleness

**Decision:** After every required pass has a valid exact-revision terminal, one pure reducer unions findings by stable semantic identity, preserves distinct corroborating provenance, and chooses the most restrictive verdict. No favorable subset can accept. Missing, failed, blocked, invalid, stale, mismatched-revision, conflicting, or invalidated pass evidence makes the process non-accepting and names every contributor.

Any proposal head/code change makes every pass and prior aggregate non-current. The new revision creates a new review subject and generation, rebuilds the base pack from the exact tree, admits and runs the full required set with fresh top-level contexts, and retains all old events. A terminal failed/blocked process on an unchanged revision keeps the same subject but allocates the next linked generation; a duplicate invocation of an unterminated generation resumes it idempotently. A comment/disposition-only change does not alter the code revision but remains separately attributed.

- **DINV-8** — `satisfied` exists only behind a complete valid pass set; an absent or non-accepting pass can never be ignored.
- **DINV-9** — One code/head revision is certified only by one complete process built entirely from that exact revision.

### 3.8 Report projection, attribution, harvest, and downgrade

**Decision:** Canonical demand/route/outcome/pass/process events govern. One `impl-review-report/v2` artifact on the proposal's own medium is a pure rendered-plus-machine projection of the current canonical process, findings, invalidations, and event references. GitHub and Carina adapters round-trip the same schema; a mismatch is non-accepting. Old `impl-review-report/v1` artifacts remain historical and cannot satisfy this process.

Operator dispositions remain separate operator-authored medium acts whose native author establishes attribution; workers cannot write operator identity into the payload. Harvest enumerates the full proposal universe and reads canonical/current v2 projections to calculate coverage, precision, nit/miss, route/process integrity, cost, cache hits, and usage. Correctness gates never read rendered prose or cost targets.

The typed aggregate exposes proposal/revision/process identity, effective depth/count, complete pass/event references, finding union, and verdict. Implement-now consumes it as its review gate. The merge-lane `Downgrader` adapter consumes the same persisted aggregate and may force HUMAN; it can never grant AUTO or change the classifier.

- **DINV-10** — One canonical process event set governs; report, harvest, implement-now, and merge lane either match it exactly or reject it.

### 3.9 Boundaries and non-goals

- **Not solved here** — implementation claim/workspace/drive-to-green, native catalog entries, git CAS mechanics, production activation, or merge authority.
- **Owned elsewhere** — demand/policy/evidence protocols are dependencies; implement-now owns invocation and union cutover.
- **Implementation must not infer** — review harness from implementation, pass count from model name, pass identity from remediation, approval from a favorable subset, or attribution from payload strings.
- **Must remain unchanged** — codegate verdict/finding core, merge-lane classification and AUTO predicate, non-review executor behavior, and sibling review artifact classes.

### 3.10 Observability

- **Events/logs** — canonical pass/process/route/outcome/invalidation events plus structured orchestration logs for process/pass/context/pack IDs, barrier state, staleness, and projection checks.
- **Metrics** — complete-process coverage, precision/nit/miss, effective depth/count, route class/native profile, pass/internal rounds, candidate/kill/finding counts, wall time, tokens, cache hits, and process violations.
- **Alerts** — incomplete/reaped process, shared context/output leakage, stale acceptance attempt, launch mismatch, projection mismatch, invalidation, or zero-pass result alerts the operator.
- **Debugging surface** — a process inspector exact-reads canonical events and renders join inputs, pack digest/provenance, pass/lens tree, terminals, union, aggregate, staleness, and mismatches.
- **Trace/correlation** — demand→process→pass→attempt→context/lens→finding→aggregate→proposal/merge-lane references are explicit.

### 3.11 Security & privacy

The review base sends repository code/diff and internal requirements to the explicitly selected provider through an already authorized harness. It excludes credentials and all worker/sibling outputs; private overlays preserve structural blindness (#DINV-5). Closed routing and exact launch capture prevent a contributor from steering to arbitrary model arguments. Canonical events contain identities, verdicts, findings metadata, usage, and hashes but no prompt/code/model output. Operator dispositions derive identity from the medium author, blocking worker forgery. The aggregate is downgrade-only at merge lane and never conveys merge credentials.

### 3.12 Failure and test strategy

The join, identities, pack compiler, reducer, report renderer, and harvest are in-process deterministic tests. Context-pack and proposal media are local-substitutable goldens. Harness launch and code-host adapters use conformance suites. Model behavior runs decode-pinned EVAL-1..11 at the PRD's required repetition; structural routing/join/blindness/revision/aggregation cases are exhaustive.

| Failure | Behavior | Recovery |
|---|---|---|
| any pass route/evidence invalid before launch | launch none; block every earlier admitted route; persist non-accepting process | repair policy/evidence and allocate a linked replacement generation on the same exact revision |
| one pass/lens fails after launch | collect/reap all required siblings; aggregate non-accepting | rerun complete exact-revision process |
| head changes | stale all passes/aggregate immediately | new pack, process, routes, and full pass set |
| report write/mismatch | canonical process remains; acceptance blocked | regenerate projection from canonical events |
| qualification defect | invalidate affected processes, stop acceptance | full replacement pass set on exact revision or operator brief if accepted |

## 4. Alternatives considered

| Alternative | Why rejected |
|---|---|
| Concurrent blind complete-set collection over one cacheable base pack (#ALT-10) | selected: order-invariant complete evidence, lower wall time, and common-prefix caching without judgment sharing (`design-brainstorm:planner-driven-model-routing-tech-design-3#ALT-10`) |
| Sequential fail-fast passes (#ALT-11) | pass order determines available findings and critical work routinely omits the second required independent view |
| Count one reviewer's remediation round as pass two (#ALT-12) | same lineage is not independent corroboration and violates the closed process |
| Fold impl-review orchestration into implement-now (`design-brainstorm:planner-driven-model-routing-tech-design-4#ALT-2`) | erases the independently amended review product and makes review assurance inseparable from one executor lifecycle |
| Build each pass pack separately | duplicates expensive context loading and risks byte/provenance drift without improving independence |
| Share one reviewer session across passes | leaks conclusions and cannot prove distinct top-level contexts |
| Keep proposal payload as authority | unsupported/pre-proposal routes and cross-medium recovery remain incomplete; conflicts with canonical evidence |

## 5. Risks

| ID | Risk | Impact | Mitigation | Verification / evidence |
|---|---|---|---|---|
| R1 | Shared context pack leaks pass/lens output | correlated reviews masquerade as independent | immutable base, suffix-only private overlays, provenance checks | output-leakage and would-fail-if-shared fixtures |
| R2 | Concurrent passes amplify cost on already-red work | extra spend | critical complete-set evidence is required; bound owner and measure cache/tokens | cost/cache report, never correctness threshold |
| R3 | Aggregate reducer cherry-picks favorable evidence | defective proposal accepted | complete-set barrier and most-restrictive pure reducer | reversed-order/split-verdict/conflict corpus |
| R4 | Context limit truncates a required deep operand | false approve | qualified profile context, deterministic pack budget, refuse partial read | EVAL-8 truncation corpus |
| R5 | Proposal projection drifts from canonical events | operator sees wrong verdict | exact event references/digest and reject mismatch | both-media round-trip/mismatch suite |
| R6 | Existing consumers expect report/v1 | harvest or UI break | v2 tagged projection; historical v1 read-only; activation requires updated consumers | compatibility and unsupported-v1 acceptance tests |

## 6. Acceptance

| ID | Acceptance criterion | Verify | Required evidence | Traces to |
|---|---|---|---|---|
| A-1 | Exhaustive assurance×band fixtures produce exactly §3.1, including ordinary skip→standard×1, every elevated→deep×1, and every critical→deep×2; zero cells produce zero passes. | Test | join matrix | `prd:planner-driven-model-routing/FR-25` · `prd:impl-review/FR-24` · #DINV-1 |
| A-2 | Differing implementation/review mappings resolve and launch the selected harness's qualified review profile for every required cell; unsupported launches no pass. | Test + Eval | review-role conformance and EVAL-11 report | `prd:planner-driven-model-routing/FR-14` · `prd:impl-review/FR-25` |
| A-3 | Every required pass route is canonical and exact-read before any pass launches; a failure at every admission position launches none, appends blocked outcomes for all earlier resolved routes, and leaves zero terminal-less resolved attempts. | Test + Failure injection | prelaunch barrier/compensation trace | `prd:impl-review/FR-29` · #DINV-8 |
| A-4 | Critical processes create two distinct top-level contexts and attempts on one exact revision; each impl-review pass runs four blind lenses and receives no sibling/implementer/session output; the same generic runtime runs the e2e program's two-lens instance with unchanged scope-lock behavior. | Test + Eval | topology/provenance, reviewer-program compatibility, and EVAL-10 fixtures | `prd:impl-review/FR-20` · `prd:impl-review/FR-21` · #DINV-4 · #DINV-6 |
| A-5 | All pass/lens requests share byte-identical base-pack bytes/digest as their largest prefix, use private suffix overlays, and report provider cache hits when available. | Test + Analysis | byte-prefix golden and cache measurement | #DINV-5 |
| A-6 | Concurrent completion in every order yields the same full finding union and most-restrictive aggregate; missing/non-accepting/conflicting passes never satisfy. | Test + Failure injection | permutation/split-verdict corpus | `prd:planner-driven-model-routing/FR-26` · `prd:impl-review/FR-26` · #DINV-7 · #DINV-8 |
| A-7 | Duplicate invocation resumes one unterminated process generation; a terminal failure on the unchanged revision allocates a linked generation with fresh pass/attempt/context IDs; any code/head change creates a new subject/generation and full pass set, while remediation remains non-certifying. | Test | resume, terminal-rerun, one/two-pass staleness, and remediation fixtures | `prd:impl-review/FR-18` · `prd:impl-review/UB-9` · `prd:impl-review/UB-13` · #DINV-3 · #DINV-9 |
| A-8 | Findings remain calibration-cited and grammar-compatible with codegate; EVAL-1..11 meet their live PRD thresholds with complete provenance. | Eval + Inspection | eval records and grammar conformance | `prd:impl-review/FR-5` · `prd:impl-review/FR-12` · `prd:impl-review/NFR-8` |
| A-9 | GitHub and Carina each carry one self-contained v2 projection matching canonical events; operator attribution survives, harvest reads no prose, and every mismatch is non-accepting. | Test + Inspection | media/attribution/harvest conformance | `prd:impl-review/FR-6` · `prd:impl-review/FR-16` · `prd:impl-review/FR-27` · #DINV-10 |
| A-10 | The aggregate drives implement-now's review gate and can force merge-lane HUMAN through `Downgrader`, but can never grant AUTO or change risk classification. | Test + Demo | end-to-end carrier and downgrade fixture | `prd:impl-review/FR-31` · `prd:impl-review/NR-5` · #DINV-2 |
| A-11 | A seeded policy/profile defect invalidates every affected open pass/process, requires the complete replacement set on the exact revision, preserves unaffected work, and briefs accepted artifacts. | Failure injection + Inspection | invalidation drill | `prd:impl-review/UB-17` · `prd:planner-driven-model-routing/UB-14` |

## 7. Open questions

None. Calibration tables, native qualifications, context budgets, and current adapter inventories remain versioned live data; process shape, identity, blindness, barrier, and aggregation changes require a successor design.

## Principles

- **AP-3 — honored.** One typed aggregate and report contract serves executor, proposal media, harvest, and merge lane.
- **AP-5 — honored.** Pass topology, route, pack, findings, aggregate, staleness, usage, and cache behavior are observable.
- **AP-6 — honored.** Process, report, findings, routing, and projections parse closed.
- **AP-10 — honored.** Assurance join, calibration, and aggregation are versioned data plus pure evaluators.
- **AP-11 — honored.** Merge lane alone classifies risk; impl-review alone projects band and joins it with planned assurance.
- **AP-13 — honored.** Every pass and aggregate binds one exact proposal revision; any edit invalidates the set.
- **AP-15 — honored.** Full-set recovery and deterministic aggregation converge independent of completion order.
- **AP-17 — honored.** Missing route/evidence/pass/projection fails closed before invocation or acceptance.
- **AP-20 — honored.** Demand, native resolution, evidence, review process, executor, and merge authority remain separate.
- **AP-22 — honored.** Concurrent passes/lenses have one bounded owner, cancellation, reap, and terminal responsibility.
- **AP-28 — honored.** Canonical git events govern; proposal/report faces are projections.
- **AP-29 — honored.** Eval and process gates assert correctness; latency, tokens, and cache hits remain descriptive.
- **RW-5 — honored.** This successor owns one independently revisable impl-review assurance process.
- **Deviation:** none.
