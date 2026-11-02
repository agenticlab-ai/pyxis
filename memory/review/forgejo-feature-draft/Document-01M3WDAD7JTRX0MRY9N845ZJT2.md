# Review: forgejo-feature-draft PRD — review report

| Field | Value |
|---|---|
| **In one line** | Request changes: the draft does not yet make the pre-merge human gate, pre-credential origin check, or final memory-integrity gate blocking acceptance obligations. |
| **Relevant-when** | Auditing the first PRD review of the Forgejo feature-draft compatibility proposal or disposing its findings before a new contract review. |
| **Subject** | prd:forgejo-feature-draft/draft |
| **Moment** | draft |
| **Reviewer** | prd-review · `.pyxis/authoring/prd-review.md` at reviewed head |
| **Date(s)** | 2026-10-01 |
| **Verdict** | request-changes |
| **Tracking** | [draft PR #1341](https://github.com/agenticlab-ai/pyxis/pull/1341), open at reviewed head `514d550a311ccda44b082dbfd6592747ee872db9` |
| **Template version** | review-report/v1 |

**Review subject:** prd
**Draft digest:** sha256:0d925c7476ca194e3ef0e67a8eb07ac948673e24d6d0e5a0c1fdc1bce5456caf

## Round 1

Contract-quality, coherence, upstream-alignment, and completeness-critic were applied to the committed subject. Only findings that survived adversarial scrutiny appear below.

### FND-1 — Prevent an unapproved merge before contract records reach mainline

- **Severity**: blocking (CAL-4: a pre-merge gate is load-bearing for §8's human-lock claim).
- **Lens**: completeness-critic.
- **Evidence**: PRD FR-6 starts **“When a prepared draft is merged”** and only then verifies the review before publishing a tag; §6.3 R-2 says **“Configure protection and independently read exact-head human review before tag”**, but no Must fit criterion or blocking DoD row verifies the protection. — vs — `design:feature-draft-v1` §3.9 requires branch protection with the operator's code-owner review, new-push dismissal, and admins bound; `prd:feature-draft/FR-5` keeps draft content off mainline until its lock. A refused post-merge tag cannot remove already merged contract records.
- **Acceptance condition**: Add a Must acceptance obligation and blocking evidence that the selected Forgejo repository refuses a prepared contract merge before the configured human's authenticated review of that exact head, with stale approvals dismissed and privileged merges covered; retain the independent post-merge review and mainline verification. State an equivalent enforceable pre-merge mechanism if native protection cannot meet this requirement.
- **Disposition**: applied
  Application evidence: FR-5 and DOD-3 now require enforceable exact-head human approval before merge with stale/agent/privileged-bypass negatives; FR-6 remains independent. Working-draft repair authority and interpretation: problem A-5; P-1/P-5.

### FND-2 — Validate origin before any credential-bearing request

- **Severity**: blocking (CAL-4: credential disclosure across a hostile origin is load-bearing for §8's dangerous-row safety claim).
- **Lens**: completeness-critic.
- **Evidence**: PRD FR-1 promises remote matching **“before host mutation”** and UB-1 tests **“zero writes”**; NFR-1's canary tests only listed output locations. PRD ABUSE-3 itself names **“credential exposure”** and says the match precedes credentials, but DOD-5 does not require proof that a mismatched origin receives no credential-bearing request. — vs — `problem:forgejo-feature-draft#A-1` requires fail-closed origin and credential behavior, and `brainstorm:forgejo-feature-draft#D-3` carries leaked credentials as a refusal boundary. A read or Git authentication to the wrong host can disclose a token without a POST, push, or logged canary.
- **Acceptance condition**: Make pre-credential selected-host/repository validation an explicit requirement and blocking fixture: for mismatched or hostile remotes and redirects, no API or Git request carries the agent credential to the unbound destination, and no host write occurs. Keep the existing output-canary checks.
- **Disposition**: applied
  Application evidence: FR-1, UB-1, and DOD-5 now require pre-credential destination validation, hostile-redirect refusal, and zero credential-bearing requests/writes. Working-draft repair authority and interpretation: problem A-5; P-2/P-6/P-7.

### FND-3 — Bound the intermediate push exception and require final memory integrity

- **Severity**: blocking (CAL-2: the final-gate omission conflicts with the scoped operator ruling and the frozen integrity obligation).
- **Lens**: upstream-alignment.
- **Evidence**: PRD FR-7 says intermediate pushes run **“applicable source hooks”** and final preparation/lock require **“all readiness conditions”**; its fit criterion calls for success under **“remaining hook checks”**. DOD-1..DOD-5 have no explicit final source memory-integrity result. — vs — `problem:forgejo-feature-draft#A-4` authorizes only intermediate-document publication and expressly leaves strict final readiness **and memory integrity** to be resolved before lock; `prd:feature-draft/NR-4` preserves memory integrity for every feature. Committed `delivery:forgejo-feature-draft#E-7` distinguishes the two missing lossless ledger bindings from draft-readiness failures. A stage-aware implementation that skips that separate memory check at the final head could satisfy the stated FR-7 fit criterion.
- **Acceptance condition**: Specify which check is deferred for unfinished draft pushes, preserve all other applicable checks, and add blocking evidence that the final prepared head and lock pass the full source memory-integrity gate, including lossless ledger bindings, as well as current-digest draft readiness. A-4 must remain scoped to intermediate publication for this problem.
- **Disposition**: applied
  Application evidence: FR-7 and new DOD-6 name readiness-only intermediate deferral and complete final memory integrity, including lossless bindings. A-4 stays temporary and scoped. Working-draft repair authority and interpretation: problem A-5; P-3.

### FND-4 — Exercise every status form promised by FR-2

- **Severity**: advisory (CAL-3: FR-2 states a testable behavior, but its stated fit criterion leaves two status forms without an explicit case).
- **Lens**: contract-quality.
- **Evidence**: PRD FR-2 promises to derive **“open draft proposals, remote lock versions, and document-pipeline ledgers”**, while its fit criterion covers only **“drafting/locked states”** and missing or ambiguous proposals. — vs — `prd:feature-draft/FR-7` requires four contract states, including drafting version N+1 while N binds and still in the document pipeline; `design:feature-draft-v1` §3.1 derives those states from PR and lock history.
- **Acceptance condition**: Extend FR-2's test criterion to cover a reopened Forgejo draft with its previous lock still binding and an unmigrated document-pipeline feature, alongside the new-draft and locked states.
- **Disposition**: applied
  Application evidence: FR-2 now explicitly exercises all five requested status forms with no other-host CLI. Working-draft repair authority and interpretation: problem A-5; P-4.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-10-01 |

## Measurements

- **Rounds**: 1
- **Findings**: 3 blocking · 1 advisory
- **Dispositions**: 4 applied · 0 declined · 0 withheld-as-violation · 0 pending
- **Review phase/token/cost receipt**: unavailable; no phase allocation or billed cost inferred.
- **Human lock approval**: none observed or inferred from the open draft PR or operator A-4.
- **Protocol note**: A broad read-only search surfaced lines from the frozen sibling PRD's §0. Those lines were excluded from review reasoning and are neither cited nor judged here.

## Overlay corrections

none
