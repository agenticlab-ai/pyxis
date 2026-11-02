# Amendment 002 — authorized Argus driver approval on GitHub

| Field | Value |
|---|---|
| **Amends** | feature-draft *(chain through 001)* |
| **Status** | Approved |
| **Owner** | @mdarabi |
| **Date** | 2026-10-05 |
| **Learned-from** | operator-decision:https://github.com/agenticlab-ai/pyxis/issues/1438 |
| **Blast radius** | tier: contract-changing; touches: FR-48, FR-49, FR-50, UB-12, UB-13, NR-6, §0, §5.3; reason: permits authenticated delegated driver approval and renewed review while retaining exact-head review and hosted protection |
| **Template version** | amendment-template/v2 |

This is proposed merge-target content. The Approved envelope is required by
the existing fold and takes effect only when this source is reviewed and its
proposal lands. No prior exact-source approval or policy activation is asserted.

The operator asked that Argus own PR review and approval when it is the driver.
The source request is retained in issue 1438. This proposal enables that act
under the existing `approve-merge` class; it does not promote any repository's
authority tier, change org protection, or assert that a human reviewed this
source. Existing operator-only and local Git procedures remain the defaults.
The new requirements govern the GitHub delegated route where earlier summary
prose describes every lock as human-only. Other approval media are unchanged.

## Operations

### op: supersede FR-48

Reason: A driver that owns the feature must be able to perform its granted
approval act rather than waiting for another principal at every lock.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-56 | When a ready feature draft's lock is requested on GitHub, the system shall accept approval of the exact prepared head from the configured authenticated operator, or from the configured authenticated Argus driver where the pre-merge mainline authority contract grants approve-merge. Without that grant it shall request the operator. Other hosted and explicit local-git surfaces shall retain their existing approval requirements. | Makes the delegated driver accountable for its granted lock approval | Must | critical | Granted driver and operator fixtures lock; an ungranted driver, a forged principal label, and a candidate-only grant are refused; existing local Git fixtures retain their behavior. | Test + Inspection | G2, J2, F3 |

### op: supersede FR-49

Reason: The absolute refusal of Argus approval contradicts the operator's
driver instruction and the existing graduated action class.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-57 | Where Argus drives a GitHub feature draft and has the mainline approve-merge grant, the Argus workflow shall inspect the prepared contract, verify both independent current-digest reviews and required native checks, submit its exact-head approval through FR-59, and record the host review identity and policy commit before advancing. Without grant it shall request the operator; explicit local-git retains operator approval and landing. | Approval is a driver judgment after independent review, not an automatic consequence of green checks | Must | critical | The granted drive submits a typed decision with current review references; ungranted and local drives request the operator; changed contract content requires both independent reviews again and a fresh approval. | Test + Inspection | J1, J2, F1, F3 |

### op: supersede FR-50

Reason: Host-authenticated agent identity is distinct from operator identity;
GitHub forbids a PR author from using the native approving-review state.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-58 | Where GitHub is selected, the system shall authenticate an Argus approval from the host review's account identity and commit ID and a closed versioned driver decision binding repository, PR number, exact head, pre-merge policy commit, drive slug, prepared contract digest when applicable, and review evidence. It shall accept the same-account author's COMMENTED review only with that decision, and record the approval as Argus rather than human. The existing authenticated operator review and explicit local-git declared receipt routes shall remain supported. | Supports an honest agent decision without impersonation or a second account requirement | Must | critical | Same-account COMMENTED and distinct-account APPROVED driver decisions pass; ordinary comments, malformed payloads, wrong accounts, stale heads, policy commits, and digests refuse; old human lock tags and local receipts remain readable. | Test + Failure injection | G2, J2, F3 |

### op: add FR

Reason: The requested driver ownership covers implementation proposals as well
as contract locks, with one publication and observation mechanism.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-59 | When an authorized Argus driver approves an open ready GitHub PR, the system shall publish one review bound to the explicitly reviewed full head, read back its exact host review identity and bytes, and refuse to report it current if the PR or mainline moves. It shall read approval accounts and the grant from mainline without candidate or local overrides, preserve the operator's outstanding changes-requested review, and avoid automatic write retries. | The same driver procedure serves contract and implementation PRs while making publication races visible | Must | critical | Publication uses COMMENT for an author and APPROVE for a distinct reviewer; wrong identity, absent grant, operator changes request, and movement observed by final preflight cause zero writes; movement after that observation, including before submission, can leave at most one exact old-head decision and must report staleness and refuse lock authorization for the changed head or policy; publication transport failure is not retried. | Test + Failure injection | J1, J2, F3 |
| FR-60 | Where a GitHub lock is approved by Argus, the system shall retain an immutable versioned lock record identifying the agent account, host review ID, exact prepared head and contract digest, pre-merge policy commit, and review evidence, readable from a fresh clone; it shall not change required CI, code ownership, protected-branch review requirements, or merge bypass settings. | Approval attribution survives the authoring session while the host retains its own enforcement | Must | critical | A fresh clone renders the driver-approved locked contract with its v2 tag; human v1 tags keep their bytes and readability; candidate policy is refused; inspection confirms no protection or bypass mutation is introduced. | Test + Inspection | G2, G4, J2, F3 |

### op: supersede UB-12

Reason: Refusal applies to unauthorized agents, not a driver acting within its
explicit mainline grant.

| ID | If… (trigger) | Then the system shall… | User sees | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|---|
| UB-15 | If an agent attempts a lock approval without the configured host identity, mainline approve-merge grant, or exact-version driver decision | refuse the approval and leave the request pending for the authorized driver or operator | the missing authority or stale-binding refusal | Must | critical | Forged-account, missing-grant, malformed-decision, stale-head, stale-digest, and candidate-policy fixtures produce no lock tag. | Test | FR-56, FR-57, FR-58, F3 |

### op: supersede UB-13

Reason: Independent review FND-1 found that the old recovery path requires an
extra operator review even when Argus is the authorized GitHub approver.

| ID | If… (trigger) | Then the system shall… | User sees | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|---|
| UB-16 | If a draft changes after its approval request and before lock, or a local receipt's proposal generation, target, digest, or nonce no longer matches the current request | void the approval and require fresh preparation and review of the current request through the applicable authorized approval route: on GitHub the configured operator or granted authenticated Argus driver; on explicit local-git the retained operator procedure | the changed field, stale approval, and current prepare-and-review remedy | Must | critical | H1→H2 refuses H1 approval; both independent H2 reviews and readiness complete before fresh granted Argus approval succeeds without an additional Pyxis operator review; local generation, target, digest, nonce, and H1→H2→H1 mismatches still refuse before writes; outstanding operator changes requests and native host protection remain enforced. | Test + Inspection | FR-51, FR-52, FR-56, FR-57, FR-58, F3 |

### op: supersede NR-6

Reason: The intentional Argus exception changes who may approve, while
preserving host enforcement and the old approval surfaces.

| ID | Existing behavior protected | Why protected | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|
| NR-8 | Authenticated operator approval, exact-head and digest checks, memory integrity, required CI, code-owner protection, stale native-review dismissal, and admin enforcement retain their behavior; local-git remains explicit and never satisfies a hosted approval fallback | Driver delegation must not silently remove the checks that make a reviewed version eligible to land | critical | Existing human, readiness, memory, and local receipt suites pass; the only added approval path requires the mainline driver grant and account; no protection or admin-bypass write exists. | Test + Inspection | FR-56, FR-58, FR-60 |

### op: add DOD

Reason: Driver approval needs retained positive and refusal evidence at the
publication and fresh-clone boundaries.

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-18 | Granted same-account and distinct-account driver publication, exact-read confirmation, stale and ungranted refusals, operator changes-request preservation, and fresh-clone driver lock rendering pass; Argus guide and CLI expose the same procedure | FR-56 through FR-60, UB-15, NR-8 | Test + Inspection | release-candidate tests, reviewed guide and CLI, host conformance evidence | yes |

### op: add ABUSE

Reason: Candidate policy must not become the authority for its own approval;
the rendered case also needs an explicit typed ledger identity.

| ID | Actor and intent | Vector | Impact | Severity | Mitigated by | Verify |
|---|---|---|---|---|---|---|
| ABUSE-9 | A candidate widens its own approval grant | candidate or local policy overlay | self-authorization | high | FR-59, FR-60, UB-15 | mainline policy and pre-merge lock fixtures |

### op: supersede §0

Reason: Route attention to the deliberate delegation change; this informative
section still supplies no machine instructions.

*The body governs; this section only routes the human reviewer's attention.*

**Judge these**

| # | The call | Could have gone | If you disagree |
|---|---|---|---|
| 1 | Authorized Argus owns GitHub approval after independent review; the operator remains the fallback where the mainline grant is absent. | Require human approval even at the highest delegated tier. | FR-56, FR-57 and FR-59 change the driving principal at approval. |
| 2 | A same-account author records its decision in a host-authenticated COMMENTED review because GitHub prohibits author APPROVED reviews. | Require a separate GitHub reviewer account. | FR-58 and FR-60 state the exact authenticated boundary and honest agent attribution. |
| 3 | Mainline, not the candidate or local config, names the approver and grant; the operator's changes request remains a stop. | Let a proposed grant authorize itself. | FR-59, UB-15 and NR-8 preserve the policy boundary. |
| 4 | Existing host protection is preserved. A repository requiring a separate native approving review still requires that review. | Replace branch protection with a Pyxis-only check. | FR-60 does not authorize a protection change or bypass. |

**Sanity-check** — both independent current-digest contract reviews remain
mandatory, every approval names a full head, and every driver lock records the
host review and pre-merge policy commit. Existing rollout and local Git limits
remain; no repository is promoted by this amendment.

### op: supersede §5.3

Reason: State the authenticated driver exception alongside retained operator,
local Git, and abuse boundaries.

| Function / data | Role: operator | Role: agent session or Argus | Unauthenticated |
|---|---|---|---|
| Open or revise a feature draft | read/write | read/write, for the operator | none |
| Rule on a requirement problem | write | Argus under rule-acceptance grant only; other agents none | none |
| Approve a lock | hosted authenticated review; explicit local-git declared operator receipt and operator-invoked landing | GitHub Argus under the mainline approve-merge grant and configured authenticated account using FR-58; other agents none; local-git remains the operator procedure | hosted none; local actor identity remains unauthenticated |
| Change a locked version | only through reopen | only through reopen | none |
| Change approval policy or Argus's own grant | reviewed mainline operator change | none as an Argus act; a candidate cannot authorize itself | none |
| Change which features are draft-born | reviewed mainline change | none | none |
| Edit an existing frozen document | only its sanctioned evolution channel | only its sanctioned evolution channel | none |

| ID | Actor and intent | Vector | Impact | Severity | Mitigated by | Verify |
|---|---|---|---|---|---|---|
| ABUSE-1 | An agent seeks an easier requirement | silent requirement edit | product intent drifts | high | FR-8, FR-9, FR-18, UB-1 | unruled change appears in approval request |
| ABUSE-2 | An ungranted agent wants its work locked | self-declared driver approval | unread contract reaches execution | high | FR-56, FR-57, FR-58, UB-15 | ungranted and forged-host-account fixtures |
| ABUSE-3 | A session wants a quick lock | post-review contract revision | stale review certifies different content | high | FR-16, UB-2, UB-8 | post-review revision fixtures |
| ABUSE-4 | A candidate seeks weaker enforcement | same-proposal draft-born status | candidate selects its own gate | high | UB-6, FR-29, FR-54, NR-4 | same-proposal status fixtures |
| ABUSE-5 | A session skips reopen | in-place locked-record edit | unapproved contract changes | high | FR-19, UB-4 | memory integrity and reopen fixtures |
| ABUSE-6 | A session applies draft overrides to an existing document feature | wrong feature route | old document gates bypassed | med | FR-54, NR-7, NR-5 | existing-feature override fixtures |
| ABUSE-7 | A session starts before lock | open-draft planning or execution | unapproved contract acts as input | high | FR-5, FR-6 | open-draft planning fixtures |
| ABUSE-8 | An agent impersonates the operator | self-declared author label | forged approval | high | FR-58, FR-51, UB-13 | host-account and exact-version fixtures |
| ABUSE-9 | A candidate widens its own approval grant | candidate or local policy overlay | self-authorization | high | FR-59, FR-60, UB-15 | mainline policy and pre-merge lock fixtures |
