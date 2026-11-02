# Review: impl-review amendment 004 — review report

| Field | Value |
|---|---|
| **In one line** | Approve: the amendment preserves the live dependency chain and closes aggregate ownership, durable receipt evidence, bounded recovery, and authenticated capability preflight. |
| **Relevant-when** | auditing or consuming impl-review amendment 004; implementing the host persistence adapter, completed-report handoff, or native-receipt verification seam |
| **Subject** | `prd:impl-review/004` |
| **Moment** | amendment |
| **Reviewer** | prd-review · 2026-08-19 |
| **Date(s)** | 2026-08-19 (rounds 1–2) |
| **Verdict** | approve |
| **Tracking** | — *(introducing PR not filed)* |
| **Template version** | review-report/v1 |

## Round 1

### FND-1 — retiring FR-6 strands the merged sibling and frozen downstream chain

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: amendment 004 supersedes FR-6 with FR-32 and also retires UB-10, DOD-10, and DOD-13 — vs — `prd:implement-now/FR-41`, DOD-28, and Q-7 explicitly require authority to perform `prd:impl-review/FR-6`; live impl-review designs and the delivered plan also cite FR-6, repeating incident I9's stale-trace failure class.
- **Acceptance condition**: Keep FR-6 and the existing UB/DoD IDs live; add performer, receipt, security, and recovery requirements as refinements, and update amendment blast radius/readiness without claiming dependent-chain repair.
- **Disposition**: applied

### FND-2 — a pass-level reviewer child cannot produce the process aggregate

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: FR-34 and §5.5 make a “reviewer instance” return one completed aggregate to persistence — vs — live FR-19/FR-24/FR-26 launch one or two mutually blind pass contexts and assign aggregation to the orchestrator only after all pass terminals exist; `prd:implement-now/FR-41` expects one outer review-process execution to return the completed report.
- **Acceptance condition**: Define pass children as returning pass terminals only to the impl-review orchestrator; define the outer isolated impl-review execution as returning exactly one completed process aggregate to the host persistence adapter; carry distinct pass-attempt and publication-attempt identities through data, interfaces, failures, and evidence.
- **Disposition**: applied

### FND-3 — an unreceipted aggregate can still be recorded and counted as satisfying

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: §5.4 says the corpus excludes unreceipted evidence and §8 claims M1 is fed by the exact publication receipt — vs — EVT-9 may emit `result=satisfied`, FR-30 harvests it, and no live event or closed field records a verified publication receipt or prevents that aggregate from counting.
- **Acceptance condition**: Add a closed publication-terminal event bound to process/attempt/proposal/revision and the exact receipt; require M1/FR-30 acceptance to join a satisfying aggregate to one verified publication terminal; cover missing, duplicate, and failed terminals in blocking evidence.
- **Disposition**: applied

### FND-4 — exactly-once proof omits host crash and restart at the publication boundary

- **Severity**: blocking (CAL-3)
- **Lens**: contract-quality
- **Evidence**: FR-33, UB-21, and DOD-18 cover concurrency, response loss, retry, and exact-read but not a host crash before write, after the remote write but before receipt persistence, or after receipt persistence but before caller acknowledgement — vs — `prd:implement-now` R10 and AP-22 require restart-safe convergence.
- **Acceptance condition**: Require durable host-side handoff/idempotency/receipt state outside child write surfaces and failure-injection at every publication kill point; recovery must converge to one artifact and the same exact-readable receipt or a named non-accepting terminal.
- **Disposition**: applied

### FND-5 — the bounded publication retry has no verifiable bound

- **Severity**: blocking (CAL-3)
- **Lens**: contract-quality
- **Evidence**: UB-21 says “retry to a bounded cap” while its fit criterion permits any implementation that eventually converges or remains blocked — vs — the PRD template's Must-row rule that bounds resolve through an explicit value or closed policy and a pass/fail terminal.
- **Acceptance condition**: Bind publication recovery to a closed, versioned persistence policy carrying maximum attempts and an overall deadline; require one named exhausted terminal and failure injection proving no operation exceeds the policy.
- **Disposition**: applied

### FND-6 — capability preflight does not prove the adapter is authenticated

- **Severity**: blocking (CAL-4)
- **Lens**: completeness-critic
- **Evidence**: UB-20 preflights performer authority and interface capabilities but not current authenticated read/write access, while §6.2 merely leaves “capability preflight” to design — vs — `prd:implement-now/FR-42` requires auth/setup failure before avoidable task spend and the amendment's §8 claims the seam is executable.
- **Acceptance condition**: Require a side-effect-free authenticated read/write readiness preflight before reviewer launch, distinguish auth/infrastructure failure from missing capability, and add expired/missing credential fixtures that spend no reviewer work and write no artifact.
- **Disposition**: applied

## Round 2

Scope lock: only the six corrected spans from round 1 and regressions introduced by those corrections were reviewed. No new scope was admitted.

### FND-1 closure — dependency identity remains live

FR-6, UB-10, DOD-10, and DOD-13 remain live. FR-32..FR-37, UB-20..UB-23, EVT-13, and DOD-18 refine the existing contract additively, so the merged implement-now contract and frozen downstream artifacts retain valid citations. The amendment no longer claims a dependent-chain rewrite.

### FND-2 closure — pass, orchestrator, and outer-process outputs are distinct

FR-34, §5.4, §5.5, and §7.1 now state that blind pass contexts return only pass terminals to the impl-review orchestrator, FR-26 forms the aggregate, and only the outer isolated impl-review execution hands one completed aggregate to the host persistence adapter. The handoff is bound to separate review-process and publication-attempt identities.

### FND-3 closure — receipt evidence is durable and required for counting

FR-35 and new EVT-13 define a closed publication terminal carrying the exact native receipt and its process/attempt/proposal/revision binding. §3.2, §5.4, §5.5, §6.4, and §8 now require every counted satisfying EVT-9 to join exactly one verified EVT-13 and its exact-readable artifact; failed, duplicate, missing, or unreceipted evidence cannot satisfy M1 or activation.

### FND-4 closure — crash/restart boundaries are contractual

UB-23 requires durable host-side handoff, idempotency, and receipt state outside implementation and reviewer write surfaces. Its fit criterion and DOD-18 inject failure before write, after remote write before receipt persistence/return, and after receipt persistence before acknowledgement, requiring convergence to one artifact and receipt or one named non-accepting failure.

### FND-5 closure — recovery is closed and bounded

UB-21 requires a closed versioned persistence policy containing maximum attempts and an overall deadline, names the exhausted infrastructure terminal, and requires proof that no path exceeds the policy. DOD-18 carries policy exhaustion into blocking evidence.

### FND-6 closure — authentication fails before reviewer spend

UB-20 and the proposal-medium interface require a side-effect-free preflight proving current authenticated read/write readiness. Missing, expired, and unauthorized credential fixtures launch no reviewer work, perform no write, and surface a distinct infrastructure/authentication failure.

Regression scan: the corrected spans preserve FR-16 operator-disposition attribution while making only completed-artifact transport authorship non-authoritative; they retain live FR-6 publication ownership, keep implement-now a binding/receipt consumer rather than a publisher, and add no distinct-principal requirement. No new blocking contradiction was found.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-08-19 |
| 2 | approve | 2026-08-19 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 6 blocking · 0 advisory
- **Dispositions**: 6 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

- **2026-08-20 · operator review — cross-contract publication terminal join.** The approved review found that impl-review EVT-13 and implement-now EVT-15 represented the same publication terminal without a declared join grammar. Applied in the open amendment: FR-35 now defines four required same-grammar/same-value pairs plus identical presence and value for the two optional artifact/receipt pairs; EVT-13 carries `review_obligation_id`; §5.4, §5.5, §6.2, §6.4, §7.1, §8, and DOD-18 carry the join into data, interfaces, release evidence, and readiness. `publication_attempt_id` remains a distinct impl-review-owned child identity and is explicitly forbidden from aliasing the stable review obligation. The correction changes no round-2 disposition or verdict.
- **2026-08-20 · operator review — reciprocal contract and receipt/proof separation.** Follow-up review found the first correction still assumed a reciprocal obligation that merged implement-now amendment 005 does not contain and incorrectly mapped impl-review's native medium receipt onto EVT-15's undefined `artifact_proof_ref`. Applied in the open amendment: FR-35 now defines the desired seven-pair host contract generically; EVT-13 and §5.4 keep `native_receipt` as the exact medium locator and add a distinct non-secret `attempt_proof_ref`; UB-20 and DOD-18 prove amendment 005 insufficient and block hosted launch until a sibling amendment defines the reciprocal terminal, required identity pairs, verified evidence fields, and receipt/proof semantics. §0, §3.2, §5.5, §6.2, §6.4, §7.1, and §8 no longer claim current EVT-15 conformance. The correction changes no round-2 disposition or verdict.
