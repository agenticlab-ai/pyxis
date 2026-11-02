# Review: inherited-learning-repair — design-review report

| Field | Value |
|---|---|
| **In one line** | Approve: the changed spans make verifier provenance and recurring work explicit while preserving fresh-target adoption and the accepted incident scope. |
| **Relevant-when** | Consuming the second-round tech-spec review for inherited-learning-repair at the exact draft digest below. |
| **Subject** | design:inherited-learning-repair · shape: none · supersedes: — |
| **Moment** | draft |
| **Reviewer** | tech-design-review · guide at d5b7964bb63c053022474435d23b996c6dfc2690 |
| **Date(s)** | 2026-10-05 |
| **Verdict** | approve |
| **Tracking** | Introducing draft PR pending first successful publication; delivery:inherited-learning-repair#E-2 records the bootstrap state. |
| **Template version** | design-review-report/v1 |

**Review subject:** tech-spec
**Draft digest:** sha256:48502b29de0881543036b34f06acede11abbacfd5a3a6f324c4417d0005f6daa

## Round 1

The retained report [002.md](002.md) approved the original reviewed contract at `23be656079001ac50540d7140df53d135e2e710a` with zero findings after four lens passes and independent scrutiny. Its digest remains unchanged. This paragraph records that prior verdict; it does not repeat the first-round review.

## Round 2

Reviewed only the tech-spec diff from `23be656079001ac50540d7140df53d135e2e710a` to `d5b7964bb63c053022474435d23b996c6dfc2690` and possible regressions within those changed spans. The PRD is byte-identical. `draft digest` confirmed the committed head and digest above.

D-2 now explicitly selects the strict existing resolver/exec path and rejects ambient PATH fallback after a pin-resolution failure. Inspection of `internal/resolve/resolve.go` confirms that the strict tool resolver supports verified release resolution and a typed source-mode result without that fallback. This clarification is consistent with the existing pinned-verifier decision and refusal behavior.

D-5 now states scan, polling, target-verification and model costs. A wait may reuse proof only for the identical target commit and verifier identity within the same invocation, after a fresh remote identity read; changed targets require revalidation, and failed remote reads yield `freshness_unknown`. The changed text preserves D-2's fresh-target adoption rule, supplies the recurring-work description required by AP-19, and leaves model work with the winning task under existing executor bounds. It introduces no new scheduler or abandoned-owner recovery.

The scoped contract-quality, coherence, upstream-alignment and feasibility-completeness passes produced no new finding. The first round's independent scrutiny remains retained in 002; no second-round candidate required survival screening. This approval covers the revised design contract, not completed implementation or acceptance evidence.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | approve | 2026-10-05 |
| 2 | approve | 2026-10-05 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 0 blocking · 0 advisory
- **Dispositions**: 0 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

none
