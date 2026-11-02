# Review: argus-problem-capture-attribution — design-review report

| Field | Value |
|---|---|
| **In one line** | Approve — the active-slug problem-capture extension preserves the existing attribution boundary and has testable acceptance. |
| **Relevant-when** | auditing this supersession, its first-PR scope change, or the independence and limits of this review |
| **Subject** | design:argus-problem-capture-attribution · shape: none · supersedes: argus-draft-attribution |
| **Moment** | supersession |
| **Reviewer** | tech-design-review · vendor-neutral guide at `ba9c53d4` |
| **Date(s)** | 2026-10-05 |
| **Verdict** | approve |
| **Tracking** | Pyxis issue #1439; introducing PR not supplied |
| **Template version** | design-review-report/v1 |

## Round 1

No findings survived independent adversarial scrutiny.

The added `memory/problem/<slug>/` grammar is a fixed function of the active slug, rather than an author-supplied scope expansion. §3.1 expressly retains other-slug, code, authority and governance refusals and names traversal and prefix-lookalike negative coverage. A-5 makes native captures of both canonical ledgers verifiable through the normal first-PR attribution path.

The supersession is substantive: the successor replaces the predecessor’s exhaustive first-PR grammar. The predecessor’s §3.2–§3.7, DINV-1..4, risks, A-1..4 and principle commitments remain carried forward. Its binding content was used as integrity evidence, never as a finding target. The exact-revision index identifies the successor as live with no flags and shows no live design depending on the retired target.

The governing folded PRD and source map identify FR-9, FR-10, FR-13 and FR-14 as live; FR-12 is superseded. The successor preserves the three authorship states, positive Argus precedence, protected-path refusals and credential-free Git derivation. Its shared-account assurance matches live FR-14. The narrow path extension introduces no new identity, credential, network service or authority grant.

### Review provenance and execution

The private host receipt was read before lens execution:

- **Reviewer root:** `01a10b75-a73b-7891-8817-5bb1f78789e0`
- **Implementing root:** `01a10a7f-0f19-79a0-ab5b-77554d761c59`
- **Context:** `implreview-ctx:57916385e6711abd563ed0a8812acfcee07ef6131e49904073980e8e437fc400`
- **Freshness:** `CheckFreshInstance: pass`
- **Reviewed revision:** `ba9c53d497ae7d95e4022bf1e2aeb130b830bb80`

Four fresh, finding-blind child contexts ran under this reviewer root with `fork_turns: none`: `contract_quality`, `coherence`, `upstream` and `feasibility`. The first three ran concurrently; feasibility started when a slot became available. No generation lens received another lens’s findings. The reviewer root independently screened the provisional candidates after generation. The receipt attests root freshness; separate native-thread receipts for the children were not supplied.

The complete required corpus was loaded through exact-revision `git show` reads: guides, artifact contract, template, calibration, vision, principles, incidents, successor, predecessor, reviewer PRD, governing folded PRD and source map, and index evidence. Truncated aggregate output was recovered with bounded reads before completion. No prior review outputs or author conversations were consulted.

An initial root-only template read exposed excluded template guidance. It was reread with filtering before synthesis. No child inherited that context or was exposed to excluded text, so no lens required replacement. Human routing was neither used nor judged; amendment routing payloads were not read.

### Evidence limits

Native lint, index generation and diff-check success were supplied host observations. The installed v0.1.5 refusal, candidate acceptance, negative-path refusals, race-test results and failing-before/passing-after regression were also supplied observations, not executions by this reviewer.

The installed/candidate comparison concerns AEC source `4a541b1135d194f9dbbca435d6c83677d481d6b9`, preserved in AEC PR #6, E-62. This review assesses design substance; it does not independently certify implementation behavior, installed-release availability or merge authority. No source, Git, GitHub, authority, record or report mutation occurred.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | approve | 2026-10-05 |

## Escalation brief

none

## Measurements

- **Rounds:** 1
- **Findings:** 0 blocking · 0 advisory
- **Dispositions:** 0 applied · 0 declined · 0 withheld-as-violation
- **Adversarial survival:** 2 provisional candidates · 2 killed · 0 surviving
- **Lens contexts:** 4 fresh children · 0 replacements
- **Calibration:** tech-design-review-calibration/v1, CAL-1..5
- **Additional rounds:** none; prerequisite completion and measurement clarification remained within Round 1

| Lens | Provisional candidates | Killed after independent scrutiny | Findings |
|---|---:|---:|---:|
| contract-quality | 0 | 0 | 0 |
| coherence | 2 | 2 | 0 |
| upstream-alignment | 0 | 0 | 0 |
| feasibility-completeness | 0 | 0 | 0 |

## Overlay corrections

none