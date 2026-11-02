# Review: implement-now-authenticated-review-obligations — design-review report

| Field | Value |
|---|---|
| **In one line** | Approve: exact-green handoff, authenticated artifacts, isolation, selected recovery, routing, activation, and predecessor coverage now form one closed provider contract. |
| **Relevant-when** | auditing or consuming the authenticated configured-review provider design; asking why it was approved or bounced; tuning tech-design-review calibration |
| **Subject** | design:implement-now-authenticated-review-obligations · shape: none · supersedes: implement-now-routed-execution |
| **Moment** | supersession |
| **Reviewer** | tech-design-review · 2026-08-06 guide |
| **Date(s)** | 2026-08-06 |
| **Verdict** | approve |
| **Tracking** | issue:1029 · PR #1034 |
| **Template version** | design-review-report/v1 |

## Round 1

### FND-1 — the successor did not carry every live predecessor requirement

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: design header `Serves` omitted live predecessor bindings and still included retired bindings — vs — `implement-now-routed-execution` header and the effective implement-now source map.
- **Acceptance condition**: carry every live predecessor binding, remove retired bindings, and make the successor proof cover the resulting complete set.
- **Disposition**: applied

### FND-2 — durable recovery had no consumer owner or live companion design

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: design §3.8 promised later review-only reconciliation — vs — the frozen state-reader-v4/wake-watch-v2 designs, which derived readiness only from implementation/proposal state.
- **Acceptance condition**: amend the consumer PRD and land superseding state-reader/wake designs owning exact readiness, bounded admission, exhaustion, and status before activation.
- **Disposition**: applied

### FND-3 — qualification authorization was repository- and wall-clock-fragile

- **Severity**: blocking (CAL-3)
- **Lens**: contract-quality
- **Evidence**: design routing did not close the repository/policy authority source — vs — `prd:implement-now/FR-45` and the observed compiled-slug/expiry failure class.
- **Acceptance condition**: source repository and policy validity from closed operator-authorized inputs with injected time; prove non-pyxis and future-clock fixtures.
- **Disposition**: applied

### FND-4 — mixed frozen/current backend preflight was ambiguous

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: design preflight did not define which backend wins after config changes — vs — immutable obligation backend identity and `prd:implement-now/FR-44`.
- **Acceptance condition**: preflight the exact union of applicable frozen and current backends without rerouting a persisted obligation; selected recovery must touch only its set.
- **Disposition**: applied

### FND-5 — artifact rejection did not distinguish unrelated history from forgery/replay

- **Severity**: blocking (CAL-1)
- **Lens**: feasibility-completeness
- **Evidence**: design verification could fail permanently on the first foreign/malformed comment — vs — `prd:implement-now/FR-41`, `UB-20`, and ordinary proposal-discussion behavior.
- **Acceptance condition**: define ordered candidate scanning that skips unrelated material, rejects attributable mismatch as untrusted, and accepts exactly one author+attempt-bound artifact.
- **Disposition**: applied

### FND-6 — activation relied on a superseded design's machinery

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: design deferred activation/fallback detail to its superseded target — vs — the supersession convention requiring a complete live replacement.
- **Acceptance condition**: restate the exact activation manifest, predicate, direct cutover, fallback, invalidation, and no-legacy reachability contract.
- **Disposition**: applied

### FND-7 — the stated workspace boundary did not prevent parent-clone mutation

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: design release ordering coexisted with reviewer launch in the operator clone — vs — isolation invariant and `prd:implement-now/FR-43`.
- **Acceptance condition**: require a disposable exact-revision workspace, separate review capacity, capability confinement, and byte-unchanged parent/implementation workspaces.
- **Disposition**: applied

### FND-8 — the activation acceptance criterion was not executable

- **Severity**: blocking (CAL-3)
- **Lens**: contract-quality
- **Evidence**: design A-1 named activation confidence without a closed object set or negative corpus — vs — the single-predicate activation requirement.
- **Acceptance condition**: define a hermetic exact-object manifest/predicate and fail missing, stale, unauthorized, or favorable-subset inputs.
- **Disposition**: applied

### FND-9 — legacy-path removal had no runnable proof

- **Severity**: blocking (CAL-3)
- **Lens**: contract-quality
- **Evidence**: design A-9 asserted replacement without a reachability check — vs — the prohibition on parallel legacy success paths.
- **Acceptance condition**: add compiler/link/call-graph/reachability evidence proving typed factories only and no old constructor, compatibility routing, worker hook, or generic event path.
- **Disposition**: applied

## Round 2

### FND-10 — a retired FR remained live in the successor header

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: design header served retired `prd:implement-now/FR-12` alongside live successor FR-47 — vs — effective source-map chain FR-12 → FR-39 → FR-47.
- **Acceptance condition**: remove FR-12 and regenerate a history-clean design index.
- **Disposition**: applied

### FND-11 — four live predecessor anchors were silently dropped

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: design header omitted `prd:implement-now/FR-27`, `EVAL-3`, and `prd:planner-driven-model-routing/FR-17`, `NR-8` — vs — the frozen predecessor header and live source maps.
- **Acceptance condition**: restore all four bindings and cover them through the successor conformance predicate.
- **Disposition**: applied

Scope-locked convergence found every finding closed. The final provider repairs handoff before review eligibility; processes only newly created/repaired obligations in normal runs and only closed selected IDs in recovery; preflights exact backend unions; routes against current role-qualified policy without compiled repository/time assumptions; launches in isolated exact-revision workspaces; accepts only broker-posted author+attempt proof; emits independent EVT-10/15/16 truth; and activates through one no-legacy predicate. History-aware lint and the full document gate are green.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-08-06 |
| 2 | approve | 2026-08-06 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 11 blocking · 0 advisory
- **Dispositions**: 11 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

- 2026-08-06 · operator `@mdarabi` (`review-thread:pyxis#1054`) — Removed the requirement that the review artifact's code-host author equal a separate preflighted reviewer principal. The design now authenticates completion with the host broker's exact native receipt plus the fresh post-worker `review_id`; same-author and different-author transport conform identically, and author identity is at most operational telemetry. A scope-locked re-check found no new finding because prewrite, replay, receipt substitution, revision mismatch, and process-exit-only success remain fail-closed. The historical FND-5 wording and Round-2 convergence sentence record the reviewed pre-correction state; this overlay supersedes only their author-comparison clause. Verdict remains `approve`.
- 2026-08-20 · operator `@mdarabi` (`review-thread:pyxis#1056`) — Reconciled this approved executor successor with merged implement-now amendments 005..007 and impl-review amendment 004. Impl-review now owns both the trusted `ProcessCoordinator` and sole `HostPersistenceAdapter`; implement-now adopts impl-review's process ID, mints and pre-validates a distinct protected proof after implementation release, passes only the coordinator's canonical `AggregateHandoff` to persistence, and appends the exact reciprocal EVT-17. Artifact schema/render/binding validation belongs only to impl-review; implement-now validates protected proof and terminal equality. Verified publication is an intermediate `pair_verified` state until the same canonical review terminal and EVT-15 land, so crash recovery cannot lose lifecycle/run significance. Native receipt and attempt proof remain distinct, code-host author remains non-authoritative, and no distinct reviewer principal is required. The historical FND-5 wording, Round-2 convergence sentence, and prior overlay's `review_id` shorthand record their reviewed moments; this overlay supersedes those clauses only. Verdict remains `approve`, with the new impl-review persistence subject reviewed separately.
