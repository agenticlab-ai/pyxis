# Review: impl-review-host-persistence — design-review report

| Field | Value |
|---|---|
| **In one line** | Approve: the successor now has one trusted process-completion seam, one artifact predicate, implementable GitHub/Carina persistence, exact reciprocal terminals, and crash-safe lifecycle recovery. |
| **Relevant-when** | auditing or consuming the impl-review host-persistence successor; asking why its publisher, proof, receipt, Carina, or recovery boundaries were approved; tuning tech-design-review calibration |
| **Subject** | design:impl-review-host-persistence · shape: none · supersedes: impl-review-routed-assurance |
| **Moment** | supersession |
| **Reviewer** | tech-design-review · 2026-08-20 guide |
| **Date(s)** | 2026-08-20 |
| **Verdict** | approve |
| **Tracking** | PR #1056 |
| **Template version** | design-review-report/v1 |

## Round 1

### FND-1 — invalid handoffs had no total retry identity

- **Severity**: blocking (CAL-3)
- **Lens**: contract-quality
- **Evidence**: design §3.3 derived publication idempotency only from a completed aggregate — vs — `prd:impl-review/UB-22`, which requires zero, surplus, malformed, and mismatched handoffs to settle deterministically under retry.
- **Acceptance condition**: define one stable handoff identity and total idempotency formula across every closed handoff status; concurrent/restarted invalid calls must reuse one publication attempt and terminal with zero writes.
- **Disposition**: applied

### FND-2 — implement-now and impl-review both allocated the review process identity

- **Severity**: blocking (CAL-4)
- **Lens**: feasibility-completeness
- **Evidence**: design §3.3 accepted a host-created review attempt while the exact-incorporated predecessor owns `review_process_id` — vs — `design:impl-review-routed-assurance` §3.2 and `prd:impl-review/EVT-13`.
- **Acceptance condition**: make the trusted impl-review coordinator allocate/resume the process; the host adopts that exact ID as `review_attempt_id` and keeps proof/publication identities distinct.
- **Disposition**: applied

### FND-3 — the no-evidence-writer child could not satisfy the incorporated process contract

- **Severity**: blocking (CAL-4)
- **Lens**: feasibility-completeness
- **Evidence**: design §3.2 denied canonical evidence authority to every child but passed its output directly toward publication — vs — the incorporated route-outcome, pass-terminal, and process-terminal append/exact-read obligations in `design:impl-review-routed-assurance` §§3.2, 3.8.
- **Acceptance condition**: put a trusted `ProcessCoordinator` around the one-way child handoff; it must admit routes before launch and validate/persist returned process evidence before producing a canonical aggregate.
- **Disposition**: applied

### FND-4 — the reciprocal executor bypassed canonical process completion

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: design:implement-now-authenticated-review-obligations §3.6 sent raw reviewer output to persistence — vs — design:impl-review-host-persistence §3.2's `ProcessCoordinator.Complete → AggregateHandoff` boundary and the predecessor's DINV-10.
- **Acceptance condition**: decode only `ProcessHandoff`, call the trusted coordinator, and pass only its canonical `AggregateHandoff` to persistence; cover invalid handoff and process-completion crash boundaries.
- **Disposition**: applied

### FND-5 — Carina publication assumed capabilities no governing adapter exposed

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: design §3.3 claimed a store-generation receipt, exact-read, and create-if-absent — vs — `design:local-work-tracker-v2` §§3.2, 3.7, 3.14, which exclude ProposalService and expose only latest-wins decision namespaces unrelated to review.
- **Acceptance condition**: define an owning local ProposalService record slice with a closed review-record schema, native receipt, exact-read, create-if-absent, and conformance behavior; never widen or call WorkTracker's closed namespace.
- **Disposition**: applied

### FND-6 — the successor carried a stale e2e dependency

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: design header built on superseded `e2e-verification-skills` — vs — live `design:e2e-verification-skills-v2` and incident I6's supersession-repair rule.
- **Acceptance condition**: remove the obsolete edge and express the live e2e relationship in its correct, cycle-free direction.
- **Disposition**: applied

### FND-7 — ABUSE-8 was human-live but absent from generated provenance

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: design A-19 used `prd:impl-review/ABUSE-8`, which exists in `effective.md`, while the generated source map and therefore `Serves`/index omit it — vs — the source-map contract and incident I4.
- **Acceptance condition**: do not fabricate a machine edge or retroactively rewrite frozen history; disclose the historical mismatch and keep activation false until the live projection-congruence contract performs a lawful repair and the activation manifest proves the generated row.
- **Disposition**: applied

### FND-8 — artifact binding had two verifier authorities and conflicting classes

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: design §3.5 made impl-review the schema/binding owner while design:implement-now-authenticated-review-obligations §3.7 repeated schema/render/binding checks and assigned overlapping failure classes — vs — AP-11 and the required equal EVT-13↔EVT-17 result.
- **Acceptance condition**: make impl-review the only artifact schema/render/binding predicate; limit implement-now to pre-publication protected-proof and post-publication reciprocal-terminal checks, with disjoint failure ownership.
- **Disposition**: applied

### FND-9 — a verified publication pair could lose its lifecycle terminal forever

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: design:implement-now-authenticated-review-obligations §3.4 cleared the obligation from EVT-13↔EVT-17 before `review-terminal/v1` and EVT-15 — vs — the exactly-one lifecycle terminal and run-significance promises.
- **Acceptance condition**: introduce a discoverable `pair_verified` intermediate; recover the same review terminal and EVT-15 before backend preflight, attempt fencing, launch, or publication, including when backend authentication is unavailable.
- **Disposition**: applied

## Round 2

### FND-10 — the closed handoff enum omitted its required mismatch arm

- **Severity**: blocking (CAL-3)
- **Lens**: contract-quality
- **Evidence**: changed design §3.2 and A-14 required deterministic mismatched-handoff behavior while `HandoffStatus` exposed only completed, missing, surplus, and malformed.
- **Acceptance condition**: add `mismatched` to the closed status grammar and bind it into the stable diagnostic/idempotency and replay fixtures.
- **Disposition**: applied

### FND-11 — one ownership sentence still returned the aggregate from the child

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: changed design §3.2 called the outer execution's result `CompletedAggregate` — vs — its own repaired §2, `ProcessCoordinator.Complete` interface, and A-14.
- **Acceptance condition**: state consistently that the child returns only `ProcessHandoff` and only the trusted coordinator creates `AggregateHandoff`.
- **Disposition**: applied

Scope-locked convergence found every finding closed. Invalid handoffs now have total stable identities; impl-review alone allocates the process, canonicalizes its evidence, validates the artifact, publishes it, and emits EVT-13; implement-now pre-validates protected proof, appends the exact EVT-17, and repairs `pair_verified` lifecycle tails before any backend dependency. The GitHub path stays one-shot under ambiguous acknowledgement, the Carina path owns a separate immutable ProposalService record slice, the stale e2e edge is removed, and the known ABUSE-8 machine-provenance debt is activation-blocking rather than silently claimed or retroactively rewritten.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-08-20 |
| 2 | approve | 2026-08-20 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 11 blocking · 0 advisory
- **Dispositions**: 11 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

- 2026-08-20 · reviewer feedback (`review-thread:pyxis#1056`) — A complete corpus scan found a second historical source-missing row, `prd:e2e-verification/ABUSE-5`, beside the already disclosed `prd:impl-review/ABUSE-8`; both came through legacy §5.3 section supersedes. Design §3.9, R14, and A-20 now pin that exact two-row baseline and require an independently reconstructed zero-row corpus inventory after lawful repair, with a one-row-only repair explicitly red. The historical FND-7 title/evidence and Round-2 convergence sentence record the reviewed pre-correction state; this overlay supersedes their singular-debt wording only. Verdict remains `approve`.
