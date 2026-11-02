# Review: feature-draft amendment 002 — review report

| Field | Value |
|---|---|
| **In one line** | approve — UB-16 closes FND-1; no regressions were found within the round-2 changed spans. |
| **Relevant-when** | auditing amendment 002, implementing driver approval, or checking approval recovery after contract revisions |
| **Subject** | `prd:feature-draft/002` |
| **Moment** | amendment |
| **Reviewer** | prd-review · guide revision 1540484 (2026-09-25) · independent Codex session `/root` |
| **Date(s)** | 2026-10-05 |
| **Verdict** | approve |
| **Tracking** | Introducing PR not supplied; recorded source request: [issue #1438](https://github.com/agenticlab-ai/pyxis/issues/1438) |
| **Template version** | review-report/v1 |

## Round 1

Reviewed amendment `002-argus-driver-approval.md`, its folded effect, and `.pyxis/logs/tests/argus-driver-approval-contract-diff.txt` at repository revision `0dc427e45b89523ee4f3ca4c7238d3283b506fe3`. Folded fingerprint: `sha256:44ff1f2703d1f9a5f569bcf972007cd8f54778b4d66d13c7cfe85f1a3937f304`.

The host-reported validate-amendments, fold, and verify-generated passes were accepted as preconditions. The complete guide, canonical template, calibration, governing corpus, and artifact contract were read. Direct-source loading substituted for context-pack generation because no docgate binary was available. The actual mechanical floor was excluded; substantive cross-contract coherence was examined independently.

### FND-1 — Revision recovery still mandates operator review

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: New FR-57 requires the granted driver to “submit its exact-head approval through FR-59,” including fresh approval after changed content receives current independent reviews. Retained Must UB-13 (§5.2) requires “fresh preparation and operator review of the current request” after a draft revision. The amendment’s precedence statement addresses earlier summary prose, leaving this normative recovery requirement active. An implementer can satisfy both only by adding operator review before fresh Argus approval, contrary to the authorized driver lane established by FR-56/FR-57, the recorded request in issue #1438, and vision §4’s policy-selected execution lane.
- **Acceptance condition**: Supersede UB-13 so renewed GitHub requests use the applicable authorized approval route. Demonstrate H1→H2 invalidating the old approval, both independent H2 reviews completing, and fresh granted Argus approval succeeding without an additional Pyxis operator-review requirement. Preserve operator and local procedures, operator changes-request refusals, and native protection.
- **Disposition**: applied — author disposition recorded 2026-10-05: amendment 002 supersedes UB-13 with UB-16 and supplies focused race-test evidence. Closure confirmed in round 2.

## Round 2

Reviewed FND-1 and regressions within `.pyxis/logs/reviews/argus-approval-prd-r2.changed-spans.txt`, together with the corresponding folded UB-16 and its supersession of UB-13, at committed revision `3b543befd4812588f14e522873e9dfee65ee2575`. Folded fingerprint: `sha256:773eb83466cb5f6e4e252f0896b55594e5f9ff46c63baf1e3e63bf7d330e7188`. The host-reported native validate-amendments, verify-generated, and diff passes were accepted.

**FND-1 closure: confirmed.** UB-16 requires fresh preparation and review through the applicable authorized route: the configured operator or granted authenticated Argus driver on GitHub, and the retained operator procedure on explicit local-git. Its fit criterion requires both independent H2 reviews and readiness before fresh driver approval succeeds. It preserves stale-approval invalidation, local freshness and replay refusals, outstanding operator changes-request refusals, and native host protection.

The reported race-detector pass of `TestRevisedPreparedContractNeedsFreshDriverApproval` supplies focused fixture evidence: H1 approval does not authorize H2, fresh H2 driver approval succeeds without an operator decision, and the old digest refuses. Its review summary is fixture evidence, not a claim that model reviewers ran within the unit test. Existing readiness requirements remain in force.

The declared blast radius now includes UB-13 and renewed review without lowering the contract-changing tier. No regression was found within the supplied changed spans. No new findings.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-10-05 |
| 2 | approve | 2026-10-05 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 1 blocking · 0 advisory
- **Dispositions**: 1 applied · 0 declined · 0 withheld-as-violation · 0 pending
- **Unresolved blocking findings**: 0
- **Round-2 findings**: 0; FND-1 closed
- **Round-1 lens attribution**: four fresh agents, forked without history: `/root/contract_quality`, `/root/coherence`, `/root/upstream_alignment`, and `/root/completeness_critic`.
- **Round-1 adversarial survival**: FND-1 was generated by `/root/coherence` and independently challenged by `/root/upstream_alignment`; it survived.
- **Round-1 concurrency limitation**: four simultaneous lens agents exceeded the available slots. Finding-blind generation ran in batches of three and one.
- **Round-1 protocol limitation**: the coordinator’s initial unfiltered amendment read exposed the excluded human-only section. Its content was not reviewed, used, or cited; lens agents received filtered-read instructions.
- **Round-2 scope**: `/root` reviewed only FND-1 and regressions within the supplied changed spans. The unchanged corpus and human-only section were not reloaded or judged; no additional governing slice was consulted.
- **Round cap**: reached at round 2; the episode ends with approve.
- **Output boundary**: no source writes, contract changes, GitHub publications, or authority acts were performed. This report is returned for host persistence.

## Overlay corrections

none

## Author disposition

### Later design-review correction — 2026-10-05

Independent technical review FND-3 identified an unachievable final-observation-to-publication zero-write promise in FR-59's fit criterion. The author corrected this still-open amendment to require zero writes for movement observed by final preflight; subsequent movement can leave at most one exact old-head decision, must report staleness, and cannot authorize the changed head or policy. The corresponding failure-injection test passes for both interleavings. The design's scope-locked round 2 judges that changed binding as part of FND-3 closure. This is an attributed author correction, not a human overlay, a rewritten PRD verdict, or a third PRD-review round. The original rounds retain their exact reviewed revisions and fingerprints and do not certify this later FR-59 edit.

- **FND-1 — applied, 2026-10-05.** Amendment 002 now supersedes UB-13 with UB-16. Renewed GitHub requests use the applicable authorized operator or Argus route; explicit local-git keeps its operator procedure. UB-16 retains stale-request refusal, independent current reviews, operator changes-request refusal, and native host protection. `TestRevisedPreparedContractNeedsFreshDriverApproval` exercises H1-to-H2 refusal and a fresh H2 driver decision without an operator decision. The test passed under the race detector; its independent-review summary is fixture evidence, not a claim that model reviews ran inside the test. The original round-1 verdict remains unchanged pending scope-locked round 2.
