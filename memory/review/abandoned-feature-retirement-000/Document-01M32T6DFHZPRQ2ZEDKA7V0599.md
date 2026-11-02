# Review: prd:abandoned-feature-retirement/000 — review report

| Field | Value |
|---|---|
| **In one line** | Approved after adding complete-surface proof, owning-drive terminal-state proof, exact unrelated-test preservation, retained refusal evidence, and measurable repair and work bounds. |
| **Relevant-when** | auditing or revising the initial abandoned-feature-retirement requirement contract; asking why it was bounced; tuning prd-review calibration against destructive lifecycle work |
| **Subject** | prd:abandoned-feature-retirement/000 |
| **Moment** | new-base |
| **Reviewer** | prd-review · repository guide inspected 2026-09-21 |
| **Date(s)** | 2026-09-21 · 2026-09-21 |
| **Verdict** | approve |
| **Tracking** | Local introducing proposal; external PR not yet created |
| **Template version** | review-report/v1 |

## Round 1

Four lenses ran concurrently after the folded draft passed `docgate gate`:
contract quality, coherence, upstream alignment, and completeness. The
human-only §0 was excluded. Duplicate candidates concerning unrelated E2E
behavior were combined after adversarial screening. Seven blocking findings
and one advisory survived.

### FND-1 — Prove the retirement selection covers the complete active surface

- **Severity**: blocking (CAL-4)
- **Lens**: completeness-critic
- **Evidence**: subject DOD-6 requires the E2E pilot to be absent, but FR-3, FR-4, FR-9, and FR-10 validate only identities the operator selected and the protected inventory. No Must row detects an eligible active feature source, code unit, entry point, or living reference omitted from the selector set. The template's §5.8 rule forbids a DoD row from inventing a criterion absent from requirements.
- **Acceptance condition**: Add a Must requirement and refusal case for a deterministic complete active-surface inventory derived from typed ownership and capability references, including exclusive identities selected for deletion and shared identities routed through ordinary reconciliation. Make the pilot DoD and human rubric require zero unexplained residual feature surface.
- **Disposition**: applied — FR-14 derives the complete typed active-surface inventory, UB-9 refuses omissions and unresolved identities, and DOD-6/DOD-7 require zero residual surface in the pilot evidence and operator rubric.

### FND-2 — Require the owning feature drive to be quiesced and terminal

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: subject FR-2 and UB-2 require abandoned product state but do not require proof that attempts are quiesced and execution ownership or routing is released. `prd:feature-implementation-drive/FR-25`, FR-28, FR-29, and UB-11 permit abandonment only after cancellation and preserve work through quiescence.
- **Acceptance condition**: Make owning lifecycle state an explicit input and require observed `cancelled → explicitly abandoned terminal` proof, zero active attempts, and released ownership and routing before authority publication. Add refusal fixtures for active, cancellation-pending, cancelled-but-resumable, stale, and malformed states; define an explicit equivalent transition for legacy features with no drive state.
- **Disposition**: applied — F1, FR-2, UB-2, the lifecycle interface and dependency, and DOD-1 now require cancellation, quiescence, released routing, explicit abandonment, and a legacy-equivalent terminal transition.

### FND-3 — Preserve unrelated E2E tests against weakening and measure their behavior

- **Severity**: blocking (CAL-3)
- **Lens**: contract-quality
- **Evidence**: subject G2 promises unrelated behavior remains unchanged, while M2 measures protected bytes and unrelated-test presence, EVT-1 exposes only `unrelated_missing_count`, and FR-10/NR-4 accept tests that remain present and green. A cleanup can rewrite or weaken an unrelated test while all stated criteria pass, and the M1–M3 post-launch review has no direct execution metric.
- **Acceptance condition**: Bind the exact pre-cleanup unrelated-E2E inventory and its byte, mode, and digest identity; reject mutation outside the separately reviewed ordinary-change path; add a typed execution/regression metric and event fields, connect them to G2 and the post-launch review, and require both exact preservation and passing behavior in FR-10, NR-4, and DOD-6.
- **Disposition**: applied — M2/M4, FR-10, NR-4, EVT-1, DOD-6, and post-launch review bind exact unrelated-test identity and execution of the unchanged inventory on candidate and fresh clone.

### FND-4 — Align the shared-artifact goal with ordinary reconciled changes

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: subject G2 says shared artifacts remain unchanged; FR-9 permits present changed or unselected identities through ordinary reconciliation, §4.4 expects changed shared units to use that path, and §3.3/M3 prohibit shared identity removal rather than all shared changes.
- **Acceptance condition**: Narrow G2 to preserving shared identities from retirement while allowing separately justified ordinary changes, or prohibit shared changes and update FR-9, §4.4, guardrails, and metrics consistently.
- **Disposition**: applied — G2 now protects shared identities from retirement while FR-9 and §4.4 retain separately justified ordinary reconciliation for shared changes.

### FND-5 — Supply non-regression coverage for authority publication and restoration

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: subject §8 claims F1 through F4 each have functional, failure, and non-regression coverage. NR-1 and NR-2 trace only to F1; NR-3 and NR-4 trace only to F3. No NR row traces to F2 authority publication or F4 restored-generation retirement.
- **Acceptance condition**: Add explicit NR coverage for F2 and F4, or explain why non-regression is inapplicable to either flow, then make §8's status and note match the resulting coverage.
- **Disposition**: applied — NR-1 through NR-3 now trace the protected behaviors through F2, and NR-1/NR-3 cover restored-generation flow F4.

### FND-6 — Make refusal remedies mechanically correct

- **Severity**: blocking (CAL-3)
- **Lens**: contract-quality
- **Evidence**: subject FR-12 requires an exact refusal remedy, but its fit criterion checks only that required fields are present. NFR-5 substitutes the subjective phrase “actionable remedy.” Neither criterion proves that the emitted remedy resolves the named predicate.
- **Acceptance condition**: For every closed refusal code, specify a deterministic repair transition or explicit irreparable outcome. Require repair-and-retry evidence showing the emitted repair removes the named refusal without weakening policy, and require human and machine outputs to identify the same transition.
- **Disposition**: applied — FR-12 and NFR-5 require a deterministic repair transition or explicit irreparable result, human/machine agreement, and repair-and-retry proof without policy weakening.

### FND-7 — Retain retirement refusal signal in Git

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: subject EVT-1 sources results from CLI/gate output and test evidence, §5.4 defines no retained failure-result entity, and DOD-5 requires only an output corpus. AP-28 requires defect reports and failure dispositions that carry learning signal to become immutable append-only Git event records; routine clean checks may remain artifact-free.
- **Acceptance condition**: Add a requirement, data-retention row, and blocking DoD coverage for bounded immutable Git event records for non-clean retirement results carrying failure or judgment signal, linked to the action, pinned inputs, selector or authority identity, refusal, and remedy. Preserve zero-artifact behavior for exact clean rechecks.
- **Disposition**: applied — FR-15, the retained failure entity, EVT-2, and DOD-5 require a bounded immutable Git event for non-clean signal and preserve zero-artifact clean rechecks.

### FND-8 — Replace the tautological repository-work bound

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: subject NFR-7 promises batched repository reads and no per-record Git subprocess growth, but its fit criterion asks only for “bounded snapshot reads for the pilot inventory.” Every finite completed run is bounded, including one subprocess per record.
- **Acceptance condition**: State a numeric or asymptotic operation bound as a function of pinned snapshots and inventory size, covering repository reads and Git subprocesses, and verify it across at least two inventory sizes.
- **Disposition**: applied — NFR-7 now bounds Git launches by pinned snapshots independently of inventory size and object work linearly in snapshots plus identities, verified at two inventory sizes.

## Round 2

The re-review was scope-locked to the spans cited by FND-1 through FND-8.
The completeness lens confirmed that FR-14, UB-9, DOD-6, and DOD-7 close
the omitted-surface path. The other three lenses independently confirmed
their assigned dispositions: terminal drive and AP-28 retention; shared-unit
and flow coherence; unrelated-test integrity, remedy correctness, and the
operation bound. No changed-span regression survived screening.

The refolded subject passes `docgate gate`. That proves mechanical validity;
the operator's document review and approval remain the introducing proposal's
human gate.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-09-21 |
| 2 | approve | 2026-09-21 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 7 blocking · 1 advisory
- **Dispositions**: 8 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

none
