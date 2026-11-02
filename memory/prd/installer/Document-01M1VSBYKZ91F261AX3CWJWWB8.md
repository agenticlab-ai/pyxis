# Amendment 002 — one repository question

| Field | Value |
|---|---|
| **Amends** | installer *(chain through 001)* |
| **Status** | Approved |
| **Owner** | @mdarabi |
| **Date** | 2026-08-06 |
| **Learned-from** | operator-decision:`issue:1043` · review-thread:`issue:1043#adversarial-review-2026-08-06` |
| **Blast radius** | tier: contract-changing; touches: FR-23, FR-25, FR-26, UB-12, §0; reason: Must-row supersession merging the interactive elicitation into one destination question, plus the derived local-install contract and the absence rule the review's reproduction forced; §0's routed numbers block updated in the same set |
| **Template version** | amendment-template/v2 |

## Operations

### op: supersede FR-23

Reason: a real install refused after seven questions because `wiring.repo` and the coordinate exchange are one decision asked twice, four questions apart, whose glosses can contradict each other (`issue:1043`); merging them makes the offered choices the only valid ones and lets the wiring depth be derived rather than guessed in advance.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-26 | When `pyxis init` runs interactively, it shall elicit at most four knobs (tracker · CI mode · CLI surfaces · argus enablement), each with a recommended default, plus one repository question naming the install's destination — an existing empty repository the credential can write to, a new repository to create, or a local-only install with no GitHub — deriving the GitHub wiring depth from that answer instead of asking it, suppressing the GitHub-only questions where the answer declares no GitHub (FR-25), persisting every answer plus every path, pin, and branch value the engine consumes into the repo config, and asking nothing further. | one decision, one question: the destination determines the wiring depth, so asking them separately invites a contradiction the operator only discovers at the end; complete capture, no implicit paths | Must | med | post-init config contains the same five knob keys and the complete path/pin/branch key set, closed-schema valid; the interactive transcript asks exactly the repository question plus the applicable knobs — neither a `wiring.repo` enum question nor a separate coordinate exchange appears; a local-only answer leaves the tracker, CI-mode, and protection questions unasked | Test | G1, J1, F1 |

### op: add FR

Reason: a carina-tracked, locally-gated install never touches GitHub, yet the interview still asked it to choose a GitHub wiring depth and a branch-protection level — questions it cannot act on; the offline floor is a v1 acceptance target, so declining GitHub entirely must be an answer rather than an inference.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-25 | Where the operator's repository answer declares a local-only install, the system shall derive `tracker=carina`, `ci_mode=local`, and `wiring.protection=none`, declare no gh-transport operation, and ask no GitHub question. | the offline floor is v1 acceptance, not a fallback; an install that will never reach GitHub must not interrogate the operator about it | Must | med | for the derived knobs the declared operation table contains zero gh-transport rows and the run performs no GitHub call; the interactive transcript contains no tracker, CI-mode, or protection question | Test | G1, F1, FR-13 |

### op: add UB

Reason: an adversarial review reproduced a bare Enter — and a closed stdin — selecting the first listed repository and running the full account-facing wiring walk (routing labels, branch protection, welcome and onboarding issues) against a repository the operator never named; every other knob's default is a policy value, but this answer names an external destination, so the coordinate exchange's existing absence ruling must govern it too.

| ID | If… (trigger) | Then the system shall… | User sees | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|---|
| UB-12 | the repository question receives a blank answer or EOF — a bare Enter, a closed stdin, or a non-interactive pipe | treat it as no destination chosen, never as a default destination, and stop cleanly before any write, exactly as the conditional coordinate exchange's absence ruling does | the pre-write refusal naming the missing destination | Must | high | with stdin closed, init performs zero GitHub operations and zero writes and the target is byte-identical to its pre-run state; no repository is ever selected by default | Test | FR-26, FR-25 |

### op: supersede §0

Reason: this op set changes content §0 routes — the numbers block's five-asked-knobs claim and its conditional-coordinate-exchange clause are replaced by the merged destination question, and the block re-anchors from FR-23 to FR-26/FR-25; §0-only informative scope, replaced wholesale per the read-to-rule convention.


*The body is the contract; this section only routes attention — on any conflict the body governs; machines ignore it.*

**Judge these**

| # | The call | Could have gone | If you disagree |
|---|---|---|---|
| 1 | **Converge onto a release** — install, re-run/repair, and (later) upgrade and adopt are one manifest-driven code path | a copy-once scaffolder, converge built later | repair (M2) and the existing-repo follow-up become a second product; idempotency is untested until it hurts → §4.1, G2 |
| 2 | **A single system directory, manifest-marked ownership** — no separate `pyxis-config/` directory in v1 | the two-directory engine/repo-owned physical split the vision's §8 sketches | the vision's extension surface arrives later, and the implementation-verification PRD's manifest home needs re-ruling → §7.2 Q-1 |
| 3 | **Green with skeletons** — install seeds fill-me governing-doc skeletons and ends with gates passing | block install until the human authors vision/principles; or a new advisory gate state | installed repos can sit skeleton-governed indefinitely; the welcome item is the only authorship driver → FR-7, FR-15, R4 |
| 4 | **argus is opt-in, off by default** — the fifth knob | ship the AI Engineer enabled like every other skill | strangers get delegated decision authority by default → FR-16 |
| 5 | **Everything pipeline-serving ships** in the payload | a curated core subset first | payload breadth is parity surface: every shipped skill must behave identically in installed repos → §4.2, §3.3 |
| 6 | **Targets consume released binaries, never Go source** | clone-and-build installs | every adopter inherits a Go toolchain requirement and the module-path coupling → NG-2, FR-17 |
| 7 | **The offline floor is v1 acceptance** — carina + local hooks, zero network after fetch | GitHub-first, offline later | medium-independence stays unproven and the tracker-neutral claim is marketing → FR-13, M1 |

**Sanity-check the numbers** — install ends green at **≤ 5 min p95** on reference runners, dependency install included (NFR-1); **one** repository question naming the destination plus at most **four** asked knobs, the GitHub wiring depth derived from that answer and everything else defaulted-and-captured (FR-26), with a local-only answer leaving the GitHub questions unasked (FR-25); **100%** targets on the release-candidate e2e matrix for install success (M1) and drift repair (M2); **zero** telemetry egress (§5.4).

**The author's open questions** — one cross-contract obligation: the implementation-verification PRD homes its verification manifest in a directory this feature deliberately does not create (§7.2 Q-1). The remaining §7.2 rows are design-owned mechanism choices behind settled requirements.

**Skip the rest** — structure/ID/fold correctness → docgate; substance and coherence → the unmanned `/prd-review`; packaging, versioning-scheme, and wiring mechanisms → the technical design (§7.2); supply-chain hardening depth → the design's security section plus the §5.3 drills.
