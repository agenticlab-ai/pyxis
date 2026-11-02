# Amendment 001 — init-hardening alignment

| Field | Value |
|---|---|
| **Amends** | installer *(chain through 000)* |
| **Status** | Approved |
| **Owner** | @mdarabi |
| **Date** | 2026-07-28 |
| **Learned-from** | operator-decision:`brainstorm:init-hardening#D-7` · operator-decision:`brainstorm:init-hardening#D-8` · review-thread:`review:init-hardening-000#FND-1` |
| **Blast radius** | tier: contract-changing; touches: FR-3, FR-19, UB-1, §0; reason: Must-row supersessions aligning elicitation, preview, and admission with the merged `prd:init-hardening`; §0's routed numbers block updated in the same set |
| **Template version** | amendment-template/v2 |

## Operations

### op: supersede FR-3

Reason: `prd:init-hardening` FR-6 contracts a conditional GitHub-coordinate exchange, which FR-3's "no sixth question is asked" fit made a violation (`review:init-hardening-000#FND-1`); the question boundary now defers to that contract.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-23 | When `pyxis init` runs interactively, it shall elicit exactly the five knobs (tracker · CI mode · GitHub wiring depth · CLI surfaces · argus enablement), each with a recommended default, persist every answer plus every path, pin, and branch value the engine consumes into the repo config, and ask nothing further beyond the conditional exchanges `prd:init-hardening` contracts (its FR-6 GitHub-coordinate elicitation). | config-driven install; complete capture, no implicit paths — the question boundary defers to `prd:init-hardening` | Must | med | post-init config contains the five knob keys and the complete path/pin/branch key set, closed-schema valid; no question is asked outside the five knobs and the `prd:init-hardening`-contracted conditional exchanges | Test | G1, J1, F1 |

### op: supersede FR-19

Reason: the 2026-07-28 init-flow audit (`brainstorm:init-hardening#F-5`) found preview blind to the wiring leg and, on a cold cache, the write set; the front-loaded checks `prd:init-hardening` contracts give preview a complete report surface.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-24 | When init or converge runs with the preview flag, the system shall report every write and external operation it would perform — for init, including the front-loaded check outcomes and the declared wiring operation set `prd:init-hardening` contracts — and perform none of them. | judgment before mutation (SCAF-1 dry-run parity); the audit found preview blind to the likeliest failures | Must | med | preview output enumerates the exact write/operation set — init runs additionally naming each check outcome and the declared wiring operations; the filesystem and remote state are untouched | Test | J2, F3 |

### op: supersede UB-1

Reason: UB-1 predates the shipped adoption path — admission is content-based whether or not repository metadata exists, and the refusal must route to `pyxis adopt`/`apply` instead of claiming adoption is deferred (`brainstorm:init-hardening#D-7`, `#F-11`; `prd:init-hardening` FR-1/FR-2).

| ID | If… (trigger) | Then the system shall… | User sees | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|---|
| UB-11 | init targets a directory holding any content beyond the repository metadata directory itself — tracked, untracked, or with no repository at all | refuse before any write, enumerate the conflicts, and route to the adoption path (`pyxis adopt`/`apply`; a runnable remedy for a non-git target) per `prd:init-hardening` FR-1/FR-2 | a named refusal + the conflict list + the route | Must | high | zero writes across the admission fixtures (git and non-git content shapes); every refusal names `adopt`; no refusal claims adoption is unavailable | Test | FR-5, WR-1 |

### op: supersede §0

Reason: this op set changes content §0 routes — the numbers block's five-knob claim gains the `prd:init-hardening` conditional-exchange clause and re-anchors to FR-23; §0-only informative scope, replaced wholesale per the read-to-rule convention.


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

**Sanity-check the numbers** — install ends green at **≤ 5 min p95** on reference runners, dependency install included (NFR-1); exactly **five** asked knobs plus the single conditional GitHub-coordinate exchange `prd:init-hardening` contracts, everything else defaulted-and-captured (FR-23); **100%** targets on the release-candidate e2e matrix for install success (M1) and drift repair (M2); **zero** telemetry egress (§5.4).

**The author's open questions** — one cross-contract obligation: the implementation-verification PRD homes its verification manifest in a directory this feature deliberately does not create (§7.2 Q-1). The remaining §7.2 rows are design-owned mechanism choices behind settled requirements.

**Skip the rest** — structure/ID/fold correctness → docgate; substance and coherence → the unmanned `/prd-review`; packaging, versioning-scheme, and wiring mechanisms → the technical design (§7.2); supply-chain hardening depth → the design's security section plus the §5.3 drills.

