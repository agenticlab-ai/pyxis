# Review: forgejo-feature-draft — design-review report

| Field | Value |
|---|---|
| **In one line** | approve — both round 1 CAL-4 blockers are closed; no changed-span regression survives review. |
| **Relevant-when** | Reviewing the Forgejo draft’s installed lock recording and interrupted preparation recovery. |
| **Subject** | design:forgejo-feature-draft · shape: none · supersedes: — |
| **Moment** | draft |
| **Reviewer** | tech-design-review · draft-born guide read 2026-10-04 |
| **Date(s)** | 2026-10-04 |
| **Verdict** | approve |
| **Tracking** | https://github.com/agenticlab-ai/pyxis/pull/1365 |
| **Template version** | design-review-report/v1 |

**Review subject:** tech-spec  
**Draft digest:** sha256:5a0e19cec9228537f601f725e4823c5c79e1195398a236338632387830d29620

## Round 1

The actual round 1 report is `drafts/forgejo-feature-draft/reviews/010.md`, at head `1be3af0998cb6669f4a46e8180c02bf9b7de1271` and digest `sha256:7ca390f9bbbd47613d3b5f33081acdc904d59677316b2eec94dfe228f956ac10`. Its request-changes verdict and original finding text remain the round 1 record. The author marked both dispositions applied.

| Original finding | Round 2 disposition judgment |
|---|---|
| **FND-1, blocking (CAL-4):** no installed Forgejo lock-recorder entrypoint after merge. | **Closed.** D-6 specifies `pyxis exec draft-lock-record -- --before <B> --after <A>` in the bound installed checkout, validates full mainline IDs and repository credentials, preserves the GitHub workflow, and requires invocation after the human merge, repeat invocation, and fresh-clone verification of the exact annotated tag. |
| **FND-2, blocking (CAL-4):** no re-entry path after publishing prepared H while the author branch remains at R. | **Closed.** D-4 defines a typed preparation intent on the existing Git ref/CAS transport, explicitly extends initialization to retain R and H, and verifies identity, digest, heads, gates, and the same PR before the equality exception. It reuses H and reconciles body, ready state, and reviewer request with kill/restart/fresh-clone coverage. The closed `draft-event/v1` schema remains intact. |

## Round 2

I verified a clean tracked HEAD `a0216bcfb56468ca226bc15f7a7af7a1334cfa95` and merge base `eaac1a9d2d8358128fe83f2f42d7ed82bd1bbf2a`. The exact tech-spec snapshot hashes to `04515b0615af0438caf44d29e3bcf4e37b47ab41b554b03bc33c45aabfc0bfe1`; the supplied body-only PRD snapshot hashes to `efdc867fbe283afe8deac1acb33e20e437c61d586f3f1c5b237eb9a5c5a0f2c0`. The supplied native current-subject and digest receipts bind the combined digest above to this head.

The four adapted lenses found the changed designs deliberate and testable: D-4 names the intent fields, phases, refusal conditions, reconciliation steps, and interruption proofs; D-6 names the installed command, range refusals, and remote tag proof. Their sequence preserves the R lease and exact H, while uncertain reviewer dispatch remains pending until native evidence establishes its outcome. The existing `pyxis exec` grammar passes the flags through, `RecordRange` supplies the range-based core, and the required retained-parent support is identified as an implementation extension. These changes serve the existing FR-6/UB-6 and DOD-4/DOD-5 clauses without changing acceptance criteria or the binding P-9 operator ruling.

**New round 2 findings:** none. No tests, gates, or network probes were run. The earlier all11/attribution results apply to the previous published head only. The known ownership failure of emitted 010 is a mechanical gate result, not a substantive design finding. This design verdict does not constitute human #1392 merge approval or authorize a v2 lock.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-10-04 |
| 2 | approve | 2026-10-04 |

## Escalation brief

none

## Measurements

- **Rounds:** 2, across reports 010 and 011
- **Findings:** 2 original blocking · 0 original advisory · 0 new round 2 findings
- **Dispositions:** 2 applied and closed · 0 declined · 0 withheld-as-violation · 0 pending

## Overlay corrections

none