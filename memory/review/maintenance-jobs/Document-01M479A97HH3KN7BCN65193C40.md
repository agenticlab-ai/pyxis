# Review: maintenance-jobs — tech-spec re-review

| Field | Value |
|---|---|
| **In one line** | Approve: all eight findings close, with no surviving changed-span regression. |
| **Relevant-when** | Assessing the reviewed maintenance-jobs design at its exact contract digest. |
| **Subject** | design:maintenance-jobs · shape: none · supersedes: — |
| **Moment** | draft |
| **Reviewer** | tech-design-review · repository guide at e04314063c7577a2588085a1847d118eeda1d830 |
| **Date(s)** | 2026-10-05 |
| **Verdict** | approve |
| **Tracking** | https://github.com/agenticlab-ai/pyxis/pull/1443 |
| **Template version** | design-review-report/v1 |

**Review subject:** tech-spec
**Draft digest:** sha256:bd0874fdde891b5c1a32df486772fda61bd3eb10d2990f8a30857a6c0902fde4

Reviewed committed head e04314063c7577a2588085a1847d118eeda1d830. This is
pass 2 of the episode begun in reviews/002.md; the prior report preserves its
original digest, findings, and author dispositions.

## Round 2

Four parallel lenses examined the named defects and regressions within changed
tech-spec spans only. Independent adversarial screening confirmed each closure:

- FND-1: D-7 fences the legacy launch path under the old runner lock, drains
  dispatched processes, refuses unproved quiescence, and fences/drains new
  dispatch before inverse rollback.
- FND-2: D-1–D-3 use ready/installing/unavailable generation state and the shared
  transaction lock through claim proof, with verified rollback and a paused
  installation failure fixture.
- FND-3: D-2 evaluates provider-specific timezone, shell/startup and delay settings
  at each retained entry position and refuses incompatible environments.
- FND-4: Principles correctly separates AP-1 from AP-27; D-1 defines additive
  command registration with derived dispatch/help and a conformance fixture.
- FND-5: D-5 owns run resources, preserves retained/unexported work, and supplies
  a conservative audited janitor for positively releasable disposable resources.
- FND-6: D-3 defines repeated-refusal counters, reset rules, status and fixtures.
- FND-7: D-4 verifies report/publication artifacts, distinguishes the CI objective,
  and D-5 tests exit-zero with missing or unproved evidence.
- FND-8: D-1/D-3 require installed/published schedule fingerprints to agree before
  claiming and verify the close-shift counterexample without freezing live prompts.

No new finding survived calibration. No unchanged-content finding was delivered,
no human-only PRD routing section was consumed, and no separate solution-shape
was introduced. Approval is of the revised contract; the promised mechanisms
and acceptance fixtures still require implementation and execution evidence.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 (reviews/002.md) | request-changes | 2026-10-05 |
| 2 | approve | 2026-10-05 |

## Escalation brief

none

## Measurements

- **Rounds**: 2 in this episode
- **Findings**: 7 blocking in the prior report · 1 advisory · 0 new findings
- **Dispositions**: 8 applied in the prior report · 0 declined · 0 withheld-as-violation

## Overlay corrections

- 2026-10-05 · @mdarabi · Post-approval operator review requires all four
  existing harnesses with semantic routing, retained daily job/date keys,
  runner-owned policy/claim Git storage, private execution clones, and the
  local `.pyxis/maintenance/logs.jsonl` audit stream. The earlier design's
  minute/grace and Codex-only layouts are replaced in this open draft under
  attributed rulings P-1–P-4. This report's old head/digest and verdict remain
  historical evidence; a new review episode must assess the revised contract.
