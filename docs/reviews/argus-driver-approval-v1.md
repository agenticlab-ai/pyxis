# Review: argus-driver-approval-v1 — design-review report

| Field | Value |
|---|---|
| **In one line** | approve — all three blocking findings closed; no regressions found within the round-2 scope. |
| **Relevant-when** | auditing this supersession’s requirement coverage, dependency compatibility, and exact-head GitHub publication guarantee |
| **Subject** | design:argus-driver-approval-v1 · shape: none · supersedes: feature-draft-local-git |
| **Moment** | supersession |
| **Reviewer** | tech-design-review · guide snapshot 3b543befd4812588f14e522873e9dfee65ee2575 |
| **Date(s)** | 2026-10-05 |
| **Verdict** | approve |
| **Tracking** | [issue #1438](https://github.com/agenticlab-ai/pyxis/issues/1438); introducing PR not supplied |
| **Template version** | design-review-report/v1 |

## Round 1

Reviewed `docs/tech-design/argus-driver-approval-v1.md` at committed revision `3b543befd4812588f14e522873e9dfee65ee2575`, against feature-draft amendment 002 and its folded effect, with the complete declared predecessor inspected. The full skill, vendor-neutral guide, report template, artifact contract, calibration, governing corpus, and served PRD basis were loaded. `docgate` was absent from PATH, so the guide’s direct-source context fallback was used. The supplied native lint pass and actual lint/source-gate implementation defined the excluded floor. Issue #1438’s recorded driver-ownership decision was accepted as the basis.

Four finding-blind lenses used fresh child contexts: three concurrently, then the fourth within the four-slot limit. Independent adversarial scrutiny followed. These were child contexts under one Codex root, not separate top-level harness sessions. An initial orchestration filter exposed the amendment’s human-only §0 replacement payload to the root and the first three contexts; those lens contexts were discarded and restarted with an operation-aware filter. All retained lenses and the survival screener excluded §0 before reading. The root’s exposure remains an independence limitation; this report uses the clean contexts’ findings.

Pre-existing reviewer outputs, implementation-author conversations, and author dispositions were not consulted. A concurrent session moved the shared worktree’s HEAD; substantive reads remained pinned to the requested commit. The reviewer performed no source/Git writes, publications, authority acts, contract edits, or permission changes. This returned report is for host persistence.

### FND-1 — Whole-document supersession loses retained live coverage

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: design:argus-driver-approval-v1’s opening “incorporates feature-draft-local-git's binding lifecycle, compile, readiness, round, reopen, memory, planning, and explicit local Git mechanisms unchanged,” but its header serves only FR-56–FR-60. Target design:feature-draft-local-git#A-1, #A-3, #A-24 and #DINV-8 retain obligations including FR-1 and FR-51. Whole-document retirement removes the target from the live `serves_inverse`; incorporation prose supplies no inherited coverage. The committed index and folded basis show 60 live Musts, eight live-served and 52 unserved: 50 were explicitly served by the retired target, while UB-15 and UB-16 are new replacement omissions. This conflicts with tech-design-conventions §2/§3.1, prd:contract-gate/FR-3 and FR-18, and incident I4.
- **Acceptance condition**: Restore explicit live coverage for the retained active requirements and applicable replacement UB/NR bindings in the successor or another live carrying design. Regenerate the index and demonstrate that the supersession introduces no inverse-coverage gaps; preserve the existing lifecycle and local Git contract.
- **Disposition**: pending — no author disposition supplied or inspected.

### FND-2 — Retained compatibility has no executable dependency discharge

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: design:argus-driver-approval-v1’s header both supersedes and builds on feature-draft-local-git while retaining its execution mechanics. The committed index consequently gives the successor and live feature-implementation-drive-local-git one stale Builds-on edge each, both targeting feature-draft-local-git. Target design:feature-draft-local-git#3.2, #3.16 and #3.17 supply the retained local bindings. Their substantive compatibility is preserved, but no valid currency disposition clears either edge. The verified consequence is loss of planning eligibility under prd:contract-gate/FR-14 and FR-18 and prd:plan-new/FR-27, contrary to prd:tech-design-review/FR-13’s no-stranded-dependent obligation and incident I6. The advisory index flag alone is not the finding.
- **Acceptance condition**: Remove or validly discharge the successor’s stale dependency, and clear the frozen dependent’s exact stale edge through a governed currency-disposition/v1 or sanctioned successor. Preserve the consumed local bindings and demonstrate restored currency/planning eligibility for the affected contracts.
- **Disposition**: pending — no author disposition supplied or inspected.

### FND-3 — The final observation cannot guarantee zero publication writes

- **Severity**: blocking (CAL-4 — load-bearing for A-3)
- **Lens**: feasibility-completeness
- **Evidence**: design:argus-driver-approval-v1#D-4 promises “A move before writing produces zero writes”; #A-3 requires “Pre-publication moves write nothing,” matching prd:feature-draft/FR-59’s fit criterion. The specified final observation and review publication are separate operations. GitHub’s [documented review-creation API](https://docs.github.com/en/rest/pulls/reviews#create-a-review-for-a-pull-request) allows a supplied review commit without an expected-current-head/base condition. Inference: a head or mainline move after the last observation but before submission can still leave one old-head review. Exact commit binding and post-publication staleness protect the new version, but do not satisfy the stated zero-write acceptance.
- **Acceptance condition**: Define zero writes for movement observed by final preflight, and explicitly handle subsequent movement with at most one exact old-head decision, a stale result, and lock refusal. Align A-3 and FR-59’s fit criterion through their sanctioned evolution channels, and add a conformance interleaving that moves head or mainline between final observation and submission; alternatively demonstrate a supported atomic host guard satisfying the existing promise.
- **Disposition**: pending — no author disposition supplied or inspected.

## Round 2

Reviewed committed revision `6f6b8150ca1a255df48afc40102662e8ce96ab70` and feature-draft folded fingerprint `sha256:350874d328a612233846cdff49b550fdc544f1a7af47e0f176a3185a680ba46c`. This was the one scope-locked reconsideration: FND-1, FND-2, FND-3 and regressions within `.pyxis/logs/reviews/argus-approval-design-r2.reduced-spans.txt`, including FR-59 and the two dependent successors. The authorized round-1 report and recorded author dispositions were read. The unchanged governing corpus and copied binding bodies were not semantically reloaded. Diff-prefixed human-only headings, routing bodies and adjacent §0 context were filtered before reading.

The four lenses resumed their retained clean contexts independently for closure and changed-span regression checks, within the available concurrency. A separate fresh child context adversarially checked the closure claims. No new scoped finding survived. The shared-harness topology and the root's disclosed round-1 exposure remain limitations; no excluded routing payload was exposed in round 2.

The round-1 findings and their original pending dispositions above remain historical records. The current author dispositions are retained below; the following are the reviewer's closure judgments.

| Finding | Current disposition | Closure judgment |
|---|---|---|
| FND-1 | applied — recorded author disposition, 2026-10-05 | **Closed.** The revised header explicitly serves retained active obligations and their live FR/UB/NR replacements, including UB-15 and UB-16. The committed source-map/index metadata join confirms **60 live Musts, 60 live-covered, zero unserved** at the stated fingerprint. The introduced inverse-coverage loss is repaired. |
| FND-2 | applied — recorded author disposition, 2026-10-05 | **Closed.** The primary successor drops its retired predecessor edge and retains the predecessor's other dependencies. `feature-implementation-drive-argus-approval` and `feature-implementation-drive-failure-argus-approval` provide the sanctioned dependent closure. All three successors are live with empty index flags; no live design depends on the three retired targets. Both dependent bodies from section 1 onward are byte-identical to their frozen predecessors at the round-1 revision, verified mechanically without semantic rereading. |
| FND-3 | applied — recorded author disposition, 2026-10-05 | **Closed.** D-4, A-3 and amendment 002's folded FR-59 fit criterion consistently reserve zero writes for movement observed by final preflight. Later head or mainline movement, including before submission, permits at most one exact old-head decision, requires a stale result and refuses authorization for changed head or policy. This is the achievable guarantee required by the original acceptance condition. |

The direct implementation-drive planning-entry artifact, `.pyxis/logs/reviews/argus-approval-implementation-currency.txt`, reports all five mechanical checks clean. Its evaluated successor fingerprints match the committed files. The separate feature-draft inverse coverage is clean; its two pre-existing currency defects on `contract-gate-v2` and `feature-draft-reopen`, targeting old `feature-draft-v1` dependencies, remain unchanged and outside this round's scope. This report does not claim that entire gate is clean.

The named `TestMovementBetweenFinalObservationAndSubmissionCannotAuthorizeNewVersion` was inspected as FND-3 acceptance evidence. It injects head and mainline changes between final observation and submission, checks one retained decision with the old head/policy and commit binding, requires the stale error, and rejects approval for the changed version. The race-detector pass and native fold, validate-amendments, verify-generated, gen-index and lint passes were supplied by the host; the reviewer did not rerun them. No atomic host guard is claimed. Closure judges the corrected current amendment and fold; it does not extend earlier PRD-review certification to this later correction or create an overlay or third PRD round.

**Round-2 verdict: approve.** All three original acceptance conditions are satisfied, with no unresolved scoped findings or advisories. The two-round cap is reached. The reviewer performed no source/Git writes, GitHub actions, authority acts, contract edits or permission changes; this complete returned report is for exact host persistence.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-10-05 |
| 2 | approve | 2026-10-05 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 3 blocking · 0 advisory, cumulative; all three closed
- **Dispositions**: 3 applied · 0 declined · 0 withheld-as-violation · 0 pending author disposition
- **Round-2 outcome**: 3 closures · 0 new findings · 0 unresolved findings
- **Lens coverage**: contract-quality, coherence, upstream-alignment, feasibility-completeness; fresh finding-blind lenses and adversarial survival in round 1, scoped independent reconsideration and adversarial closure scrutiny in round 2.

## Overlay corrections

none

## Author disposition

- **FND-1 — applied, 2026-10-05.** The successor explicitly carries the predecessor's retained live service bindings, maps its superseded FR/UB/NR references to the live replacements, and serves the new approval requirements. Native generation and lint pass; the direct feature-draft contract gate now reports inverse coverage clean.
- **FND-2 — applied, 2026-10-05.** Remove the successor's dependency on its retired predecessor and retain the predecessor's other dependencies. Add sanctioned successors for feature-implementation-drive-local-git and its sole live recovery dependent, changing their retired dependency edges while preserving all binding bodies verbatim. The generated index has no stale flags for the three successor designs. The direct feature-implementation-drive contract gate is clean after this dependency closure. The separate pre-existing feature-draft currency defects on contract-gate-v2 and feature-draft-reopen remain visible and unchanged; this proposal does not claim to clear them.
- **FND-3 — applied, 2026-10-05.** Revise D-4, A-3, and open amendment 002's FR-59 fit criterion to distinguish movement observed before publication from movement after final observation. The new race-detector interleaving test passes for both head and mainline changes between observation and submission: at most one old decision remains, a stale result is returned, and the changed version cannot use it. No atomic host guarantee or zero-write claim is fabricated.